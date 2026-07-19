package restoreverifier

// escalation_test.go exercises ADR-004 §5 storm-proof escalation logic against a
// fake BeadFiler. The bf/br CLI is never invoked here (only a dedicated test
// shells out to a throwaway script in TestBFCLIFiler), so these tests prove the
// dedupe and staleness-window invariants quickly and hermetically.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// recordingFiler is a BeadFiler that records every payload it is asked to file.
// A non-nil err makes the next (and subsequent) File call fail, so tests can
// prove a failed filing leaves the dedupe key unrecorded (one bounded
// re-attempt on the next scheduler tick, never a retry loop).
type recordingFiler struct {
	mu    sync.Mutex
	filed []BeadPayload
	err   error // when non-nil, File returns it instead of recording
}

func (f *recordingFiler) File(_ context.Context, p BeadPayload) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return "", f.err
	}
	f.filed = append(f.filed, p)
	return "bf-" + p.ObjectKey, nil
}

func (f *recordingFiler) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.filed)
}

// vr builds a VerificationResult for escalation tests. Bucket is fixed so the
// dedupe key only varies by key/path/status (the components under test).
func vr(key string, status VerificationStatus, path VerificationPath) VerificationResult {
	return VerificationResult{
		Bucket: "test-bucket",
		Key:    key,
		Status: status,
		Path:   path,
	}
}

// fileFailure asserts the escalator files a bead for this failure.
func fileFailure(t *testing.T, e *Escalator, ctx context.Context, r VerificationResult, p Provenance) {
	t.Helper()
	id, err := e.EscalateFailure(ctx, r, p)
	if err != nil {
		t.Fatalf("expected a filed bead, got error: %v", err)
	}
	if id == "" {
		t.Fatal("expected a bead id, got empty (deduped or disabled)")
	}
}

// dedupFailure asserts the escalator does NOT file (already active for this key).
func dedupFailure(t *testing.T, e *Escalator, ctx context.Context, r VerificationResult, p Provenance) {
	t.Helper()
	id, err := e.EscalateFailure(ctx, r, p)
	if err != nil {
		t.Fatalf("dedupe check returned unexpected error: %v", err)
	}
	if id != "" {
		t.Fatalf("expected dedupe (no filing), got bead id %q", id)
	}
}

// fileStale asserts the escalator files a staleness bead for this bucket/window.
func fileStale(t *testing.T, e *Escalator, bucket string, lastSuccess time.Time) {
	t.Helper()
	id, err := e.EscalateStaleness(context.Background(), bucket, lastSuccess)
	if err != nil {
		t.Fatalf("expected a filed staleness bead, got error: %v", err)
	}
	if id == "" {
		t.Fatal("expected a staleness bead id, got empty (not stale / deduped / disabled)")
	}
}

// dedupStale asserts the escalator does NOT file a staleness bead.
func dedupStale(t *testing.T, e *Escalator, bucket string, lastSuccess time.Time) {
	t.Helper()
	id, err := e.EscalateStaleness(context.Background(), bucket, lastSuccess)
	if err != nil {
		t.Fatalf("dedupe staleness check returned unexpected error: %v", err)
	}
	if id != "" {
		t.Fatalf("expected no staleness filing, got bead id %q", id)
	}
}

// ---------------------------------------------------------------------------
// classFor
// ---------------------------------------------------------------------------

func TestClassFor(t *testing.T) {
	cases := []struct {
		status VerificationStatus
		class  FailureClass
	}{
		{StatusRestoreError, FailureRestoreError},
		{StatusChecksumError, FailureChecksumError},
		{StatusAssertionError, FailureAssertionError},
		{StatusConflict, FailureConflict},
		// Passing/pending/unknown statuses have no real class; they collapse to a
		// generic restore error so a future status still escalates exactly once.
		{StatusPass, FailureRestoreError},
		{StatusPending, FailureRestoreError},
		{StatusUnknown, FailureRestoreError},
		{VerificationStatus("bogus-future-status"), FailureRestoreError},
	}
	for _, tc := range cases {
		if got := classFor(tc.status); got != tc.class {
			t.Errorf("classFor(%q) = %q, want %q", tc.status, got, tc.class)
		}
	}
}

// ---------------------------------------------------------------------------
// Dedupe: one bead per distinct active failure, never per tick.
// ---------------------------------------------------------------------------

// TestEscalator_DedupesSameFailureAcrossTicks is the core storm-proofing claim:
// the same distinct failure filed on every scheduler tick produces exactly one
// bead (the 2026-07 NEEDLE retry-storm anti-pattern must not recur).
func TestEscalator_DedupesSameFailureAcrossTicks(t *testing.T) {
	f := &recordingFiler{}
	e := NewEscalator(EscalatorConfig{Filer: f})
	ctx := context.Background()
	r := vr("obj-1", StatusRestoreError, PathARMOR)

	// First tick files.
	fileFailure(t, e, ctx, r, Provenance{})
	// Every subsequent tick is deduped — no re-filing, no counter/attempt bead.
	for tick := 0; tick < 50; tick++ {
		dedupFailure(t, e, ctx, r, Provenance{})
	}
	if got := f.count(); got != 1 {
		t.Fatalf("storm-proofing failed: %d beads filed across ticks, want 1", got)
	}
}

// TestEscalator_DistinctFailuresFileSeparately proves the dedupe key
// (bucket + object key + path + failure class) keeps genuinely distinct
// breakages separate — they are different things to fix, each gets its own bead.
func TestEscalator_DistinctFailuresFileSeparately(t *testing.T) {
	f := &recordingFiler{}
	e := NewEscalator(EscalatorConfig{Filer: f})
	ctx := context.Background()

	// Different object key → distinct.
	fileFailure(t, e, ctx, vr("obj-A", StatusRestoreError, PathARMOR), Provenance{})
	// Same object, different failure class (restore error vs dual-path conflict).
	fileFailure(t, e, ctx, vr("obj-A", StatusConflict, PathDirect), Provenance{})
	// Same object+class, different path (ARMOR path vs direct path).
	fileFailure(t, e, ctx, vr("obj-A", StatusRestoreError, PathDirect), Provenance{})
	// Repeating any of the three is deduped.
	dedupFailure(t, e, ctx, vr("obj-A", StatusRestoreError, PathARMOR), Provenance{})
	dedupFailure(t, e, ctx, vr("obj-A", StatusConflict, PathDirect), Provenance{})
	dedupFailure(t, e, ctx, vr("obj-A", StatusRestoreError, PathDirect), Provenance{})

	if got, want := f.count(), 3; got != want {
		t.Fatalf("distinct failures filed %d beads, want %d", got, want)
	}
}

// TestEscalator_PersistenceSurvivesRestart proves the dedupe set is persisted:
// after a process restart (a fresh Escalator over the same state file) a still-
// active failure is NOT re-filed. This is the cross-restart storm-proofing that
// in-memory dedupe alone cannot provide.
func TestEscalator_PersistenceSurvivesRestart(t *testing.T) {
	state := filepath.Join(t.TempDir(), "escalation-state.json")
	f := &recordingFiler{}
	r := vr("obj-1", StatusRestoreError, PathARMOR)
	ctx := context.Background()

	e1 := NewEscalator(EscalatorConfig{Filer: f, Deployment: "d", StatePath: state})
	fileFailure(t, e1, ctx, r, Provenance{})
	if got := f.count(); got != 1 {
		t.Fatalf("want 1 bead before restart, got %d", got)
	}

	// Simulate a restart: brand-new Escalator loads the persisted dedupe set.
	e2 := NewEscalator(EscalatorConfig{Filer: f, Deployment: "d", StatePath: state})
	dedupFailure(t, e2, ctx, r, Provenance{})
	if got := f.count(); got != 1 {
		t.Fatalf("dedupe set must survive restart; want still 1 bead, got %d", got)
	}
}

// TestEscalator_ClearObjectPersistsAfterRecovery proves a passing object clears
// its active-failure keys AND persists that clearance, so a genuine regression
// after recovery files a fresh bead rather than being deduped away.
func TestEscalator_ClearObjectPersistsAfterRecovery(t *testing.T) {
	state := filepath.Join(t.TempDir(), "escalation-state.json")
	f := &recordingFiler{}
	r := vr("obj-1", StatusRestoreError, PathARMOR)
	ctx := context.Background()

	e1 := NewEscalator(EscalatorConfig{Filer: f, Deployment: "d", StatePath: state})
	fileFailure(t, e1, ctx, r, Provenance{})
	e1.ClearObject("test-bucket", "obj-1")

	// After a restart, the cleared object must file fresh again (regression).
	e2 := NewEscalator(EscalatorConfig{Filer: f, Deployment: "d", StatePath: state})
	fileFailure(t, e2, ctx, r, Provenance{})
	if got := f.count(); got != 2 {
		t.Fatalf("want 2 beads (initial + post-regression), got %d", got)
	}
}

// TestEscalator_FailedFilingNotRecorded proves the filer never retries within a
// tick. A failed filing leaves the dedupe key unrecorded, so the NEXT scheduler
// tick (bounded by cadence) may make one further attempt — never an unbounded
// retry loop.
func TestEscalator_FailedFilingNotRecorded(t *testing.T) {
	f := &recordingFiler{err: errors.New("bf CLI is down")}
	e := NewEscalator(EscalatorConfig{Filer: f})
	r := vr("obj-1", StatusRestoreError, PathARMOR)
	ctx := context.Background()

	// First attempt fails: no key recorded.
	if _, err := e.EscalateFailure(ctx, r, Provenance{}); err == nil {
		t.Fatal("expected an error when the filer fails")
	}
	if got := f.count(); got != 0 {
		t.Fatalf("nothing should be recorded on a failed filing, got %d", got)
	}

	// The filer recovers. The next attempt files (key was left unrecorded), and
	// this is the only re-attempt path — rate-limited by the scheduler cadence.
	f.err = nil
	fileFailure(t, e, ctx, r, Provenance{})

	// Now it is deduped: exactly one active bead per distinct failure.
	dedupFailure(t, e, ctx, r, Provenance{})
	if got := f.count(); got != 1 {
		t.Fatalf("want exactly 1 bead after one failure then success, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Staleness: one bead per freshness window, never per tick.
// ---------------------------------------------------------------------------

// TestEscalator_StalenessOncePerWindow proves staleness escalation is deduped
// per freshness window. A bucket with no verified restore in the window files
// once; further ticks in the same window do not re-file; the next window files
// again. It also confirms two buckets each get their own per-window bead.
func TestEscalator_StalenessOncePerWindow(t *testing.T) {
	f := &recordingFiler{}
	clock := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	const window = time.Hour
	e := NewEscalator(EscalatorConfig{
		Filer:           f,
		FreshnessWindow: window,
		Now:             func() time.Time { return clock },
	})

	// never verified (zero LastSuccess) → stale on the first tick.
	fileStale(t, e, "bucket-A", time.Time{})
	// Same window, same bucket: must not re-file on every tick.
	for tick := 0; tick < 10; tick++ {
		dedupStale(t, e, "bucket-A", time.Time{})
	}
	// A second bucket in the same window gets its own one bead.
	fileStale(t, e, "bucket-B", time.Time{})

	// Advance beyond the window: a new window escalates again, once.
	clock = clock.Add(2 * window)
	fileStale(t, e, "bucket-A", time.Time{})
	dedupStale(t, e, "bucket-A", time.Time{})

	if got := f.count(); got != 3 {
		t.Fatalf("want 3 staleness beads (A, B, A-next-window), got %d", got)
	}
}

// TestEscalator_StalenessNotStaleSkips proves a bucket with a verified restore
// inside the window does not escalate, and that a zero window disables
// staleness escalation entirely.
func TestEscalator_StalenessNotStaleSkips(t *testing.T) {
	f := &recordingFiler{}
	now := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	e := NewEscalator(EscalatorConfig{
		Filer:           f,
		FreshnessWindow: time.Hour,
		Now:             func() time.Time { return now },
	})

	// A verified restore 5 minutes ago is well within the 1h window: not stale.
	dedupStale(t, e, "bucket-A", now.Add(-5*time.Minute))
	// A verified restore exactly at the window edge is still within it (strict >).
	dedupStale(t, e, "bucket-A", now.Add(-time.Hour))

	// A zero/negative window disables staleness escalation entirely.
	off := NewEscalator(EscalatorConfig{Filer: f, FreshnessWindow: 0})
	dedupStale(t, off, "bucket-A", time.Time{})
	if got := f.count(); got != 0 {
		t.Fatalf("want 0 staleness beads when fresh/disabled, got %d", got)
	}
}

// TestEscalator_StalenessPersistenceAcrossRestart proves the per-window staleness
// timestamp is persisted, so a restart mid-window does not re-file.
func TestEscalator_StalenessPersistenceAcrossRestart(t *testing.T) {
	state := filepath.Join(t.TempDir(), "escalation-state.json")
	f := &recordingFiler{}
	clock := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	cfg := EscalatorConfig{
		Filer:           f,
		FreshnessWindow: time.Hour,
		StatePath:       state,
		Now:             func() time.Time { return clock },
	}

	e1 := NewEscalator(cfg)
	fileStale(t, e1, "bucket-A", time.Time{})

	// Restart within the same window: persisted timestamp prevents re-filing.
	e2 := NewEscalator(cfg)
	dedupStale(t, e2, "bucket-A", time.Time{})
	if got := f.count(); got != 1 {
		t.Fatalf("staleness must not re-file after restart within window, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// Nil safety / no-op filer
// ---------------------------------------------------------------------------

func TestEscalator_NilSafe(t *testing.T) {
	var e *Escalator
	ctx := context.Background()
	if id, err := e.EscalateFailure(ctx, vr("k", StatusRestoreError, PathARMOR), Provenance{}); err != nil || id != "" {
		t.Fatalf("nil Escalator EscalateFailure = (%q,%v), want (\"\",nil)", id, err)
	}
	if id, err := e.EscalateStaleness(ctx, "b", time.Time{}); err != nil || id != "" {
		t.Fatalf("nil Escalator EscalateStaleness = (%q,%v), want (\"\",nil)", id, err)
	}
	e.ClearObject("b", "k") // must not panic
}

func TestNoopFiler(t *testing.T) {
	id, err := NoopFiler().File(context.Background(), BeadPayload{Kind: BeadFailure})
	if err != nil || id != "" {
		t.Fatalf("noop filer must discard, got id=%q err=%v", id, err)
	}
}

// ---------------------------------------------------------------------------
// BeadPayload title/body formatting (envelope of the evidence bead)
// ---------------------------------------------------------------------------

func TestBeadPayload_FailureTitleAndBody(t *testing.T) {
	// A pathological object key must not blow the beads schema's title CHECK.
	longKey := strings.Repeat("k", 8000)
	p := BeadPayload{
		Kind:             BeadFailure,
		Bucket:           "bkt",
		ObjectKey:        longKey,
		Path:             PathARMOR,
		FailureClass:     FailureRestoreError,
		Deployment:       "ord-devimprint",
		ArtifactType:     ArtifactSQLite,
		EnvelopeVersion:  "3",
		WriterID:         "",
		ExpectedSHA256:   "abc",
		ARMORSHA256:      "def",
		DirectSHA256:     "ghi",
		Error:            "ARMOR path failed: boom",
		Detected:         time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
	}
	title := p.Title()
	if len(title) > 500 {
		t.Fatalf("title length %d exceeds the beads schema 500-char limit", len(title))
	}
	if !strings.Contains(title, "restore_error") || !strings.Contains(title, "armor") {
		t.Fatalf("title %q must name the failure class and path", title)
	}

	body := p.Body()
	for _, want := range []string{
		"Restore verification failure",
		"bkt",
		"sqlite",
		"Provenance / writer version",
		"3", // envelope version
		"Both-path evidence",
		"abc", "def", "ghi", // the three SHA-256 digests
		"ARMOR path failed: boom",
		"One bead per distinct failure",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("failure body missing %q", want)
		}
	}
}

func TestBeadPayload_StalenessTitleAndBody(t *testing.T) {
	p := BeadPayload{
		Kind:                BeadStaleness,
		Bucket:              "bkt",
		Deployment:          "iad-ci",
		FreshnessWindow:     time.Hour,
		LastVerifiedRestore: time.Time{}, // never
		Detected:            time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC),
	}
	if !strings.Contains(p.Title(), "RV stale") {
		t.Fatalf("staleness title %q must be prefixed RV stale", p.Title())
	}
	body := p.Body()
	for _, want := range []string{
		"Stale restore verification",
		"bkt",
		"iad-ci",
		"never (no verified restore since verifier start)",
		"once per window (never per scheduler tick)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("staleness body missing %q", want)
		}
	}
}

// ---------------------------------------------------------------------------
// Verifier→Escalator wiring (the single place failures reach the escalator)
// ---------------------------------------------------------------------------

// TestVerifier_EscalateResultWiring proves the verifier's sole escalation call
// site honors the contract: a non-pass result files exactly one bead carrying
// the object's provenance; repeat failures dedupe; a pass clears the object so
// a later regression files fresh.
func TestVerifier_EscalateResultWiring(t *testing.T) {
	f := &recordingFiler{}
	v := New(nil, nil, 0, nil, Config{
		Escalator: NewEscalator(EscalatorConfig{Filer: f, Deployment: "d"}),
	})
	ctx := context.Background()
	// vr() hardcodes Bucket="test-bucket"; the obj must match so that
	// ClearObject(obj.Bucket,...) and EscalateFailure(result.Bucket,...) hit the
	// same dedupe keys — exactly as production, where verifyObject copies
	// obj.Bucket into the result.
	obj := ObjectSample{
		Bucket: "test-bucket",
		Key:    "k",
		Metadata: map[string]string{
			"x-amz-meta-armor-version": "3",
		},
	}

	// A failure files exactly one bead, with provenance propagated.
	v.escalateResult(ctx, obj, vr("k", StatusRestoreError, PathARMOR))
	if got := f.count(); got != 1 {
		t.Fatalf("want 1 bead after first failure, got %d", got)
	}
	if f.filed[0].EnvelopeVersion != "3" {
		t.Fatalf("provenance envelope version not propagated: got %q want 3", f.filed[0].EnvelopeVersion)
	}
	// A repeat failure of the same distinct kind is deduped at the verifier too.
	v.escalateResult(ctx, obj, vr("k", StatusRestoreError, PathARMOR))
	if got := f.count(); got != 1 {
		t.Fatalf("want still 1 (deduped) after repeat failure, got %d", got)
	}

	// A pass clears the object's active-failure keys, re-arming it.
	v.escalateResult(ctx, obj, vr("k", StatusPass, PathDualMatch))
	// The same failure now files fresh (post-recovery regression).
	v.escalateResult(ctx, obj, vr("k", StatusRestoreError, PathARMOR))
	if got := f.count(); got != 2 {
		t.Fatalf("want 2 beads after pass+regression, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// BFCLIFiler: arg construction against a fake "br" script (no live beads store)
// ---------------------------------------------------------------------------

// fakeBRScript writes a throwaway br-compatible shell script that records its
// argv to RV_FAKE_ARGS, then succeeds, fails, or hangs based on RV_FAKE_MODE.
// This exercises BFCLIFiler's exec + arg construction without touching the real
// bf CLI or the live beads store.
func fakeBRScript(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-br.sh")
	body := "#!/bin/sh\n" +
		"# Fake br for BFCLIFiler tests. Records argv and emulates ok/fail/timeout.\n" +
		"if [ -n \"$RV_FAKE_ARGS\" ]; then\n" +
		"  for a in \"$@\"; do printf '%s\\n' \"$a\" >> \"$RV_FAKE_ARGS\"; done\n" +
		"fi\n" +
		"case \"${RV_FAKE_MODE:-ok}\" in\n" +
		"  fail)    printf 'rejected: bad bead' >&2; exit 1 ;;\n" +
		"  timeout) sleep 5 ;;\n" +
		"esac\n" +
		"printf 'bf-fake-42\\n'\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake-br script: %v", err)
	}
	return path
}

func TestBFCLIFiler_BuildsCorrectArgs(t *testing.T) {
	script := fakeBRScript(t)
	argsFile := filepath.Join(t.TempDir(), "args.txt")
	t.Setenv("RV_FAKE_ARGS", argsFile)
	t.Setenv("RV_FAKE_MODE", "ok")

	filer := &BFCLIFiler{
		Binary:    script,
		Workspace: "/var/lib/restore-verifier/.beads",
		Label:     "restore-verifier",
		Priority:  1,
	}
	id, err := filer.File(context.Background(), BeadPayload{
		Kind:         BeadFailure,
		Bucket:       "bkt",
		ObjectKey:    "backups/obj-1",
		Path:         PathARMOR,
		FailureClass: FailureRestoreError,
	})
	if err != nil {
		t.Fatalf("File returned unexpected error: %v", err)
	}
	if id != "bf-fake-42" {
		t.Fatalf("File id = %q, want bf-fake-42", id)
	}

	args, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatalf("read recorded args: %v", err)
	}
	s := string(args)
	// The fixed argv layout: create --title <t> --type bug --priority 1
	// --description <body> -w <workspace> --label restore-verifier
	for _, want := range []string{
		"create\n",
		"--title\n",
		"--type\nbug\n",
		"--priority\n1\n",
		"--description\n",
		"-w\n/var/lib/restore-verifier/.beads\n",
		"--label\nrestore-verifier\n",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("bf argv missing %q\n--- recorded argv ---\n%s", want, s)
		}
	}
	// The body (passed as --description) must carry the evidence.
	if !strings.Contains(s, "Restore verification failure") || !strings.Contains(s, "backups/obj-1") {
		t.Errorf("bf argv did not carry the bead body/evidence\n--- recorded argv ---\n%s", s)
	}
}

func TestBFCLIFiler_PropagatesFailureAndTimeout(t *testing.T) {
	script := fakeBRScript(t)
	t.Setenv("RV_FAKE_ARGS", filepath.Join(t.TempDir(), "args.txt"))

	t.Run("nonzero_exit_returns_error", func(t *testing.T) {
		t.Setenv("RV_FAKE_MODE", "fail")
		filer := &BFCLIFiler{Binary: script, ExecTimeout: 5 * time.Second}
		_, err := filer.File(context.Background(), BeadPayload{
			Kind: BeadFailure, Bucket: "b", ObjectKey: "o",
			Path: PathARMOR, FailureClass: FailureRestoreError,
		})
		if err == nil {
			t.Fatal("expected an error when the bf CLI exits non-zero")
		}
		if !strings.Contains(err.Error(), "br create failed") {
			t.Fatalf("error must wrap the CLI failure, got: %v", err)
		}
	})

	t.Run("timeout_bounds_a_single_call", func(t *testing.T) {
		t.Setenv("RV_FAKE_MODE", "timeout")
		filer := &BFCLIFiler{Binary: script, ExecTimeout: 100 * time.Millisecond}
		start := time.Now()
		_, err := filer.File(context.Background(), BeadPayload{
			Kind: BeadFailure, Bucket: "b", ObjectKey: "o",
			Path: PathARMOR, FailureClass: FailureRestoreError,
		})
		elapsed := time.Since(start)
		if err == nil {
			t.Fatal("expected a timeout error when the bf CLI hangs")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Fatalf("error must mention the timeout, got: %v", err)
		}
		if elapsed > 3*time.Second {
			t.Fatalf("ExecTimeout did not bound the call: elapsed %v", elapsed)
		}
	})
}

func TestClampPriority(t *testing.T) {
	cases := []struct{ in, want int }{
		{-5, 0}, {0, 0}, {1, 1}, {2, 2}, {3, 3}, {4, 4}, {9, 4},
	}
	for _, tc := range cases {
		if got := clampPriority(tc.in); got != tc.want {
			t.Errorf("clampPriority(%d) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
