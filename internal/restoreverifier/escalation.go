package restoreverifier

// escalation.go implements ADR-004 §5: storm-proof failure escalation.
//
// On each distinct verification failure the verifier files exactly one bead
// carrying the object key, bucket, deployment, writer version from provenance
// (where available), and both-path evidence. Staleness (no verified restore
// within the freshness window) escalates the same way, once per window.
//
// Storm-proofing (the explicit anti-pattern is the 2026-07 NEEDLE retry-storms):
//   - A persisted dedupe set means a failing object never files more than one
//     bead across scheduler ticks (or process restarts). Escalation is one bead
//     per distinct failure, never per tick.
//   - Staleness escalates once per freshness window, never per tick.
//   - The filer never retries. A failed `bf create` records nothing, so the
//     next tick may make one further attempt — bounded by the schedule cadence,
//     never an unbounded loop. No counter/attempt beads are ever filed.
//
// The bead-filer is an interface so unit tests exercise the dedupe and
// staleness-window logic against a fake without invoking the bf CLI or touching
// the live beads store.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// FailureClass categorizes a verification failure for dedupe and reporting.
// It is the "failure class" component of the dedupe key
// (bucket + object key + path + failure class).
type FailureClass string

const (
	// FailureRestoreError is a path that could not produce plaintext at all
	// (GET/decrypt/range error). The broken path is recorded separately.
	FailureRestoreError FailureClass = "restore_error"
	// FailureChecksumError means both paths agreed but disagreed with the
	// expected plaintext SHA-256 — the stored data is wrong.
	FailureChecksumError FailureClass = "checksum_error"
	// FailureAssertionError means the restore completed and matched checksums
	// but failed an application-level assertion (e.g. SQLite integrity_check).
	FailureAssertionError FailureClass = "assertion_error"
	// FailureConflict means the two paths produced different plaintext — the
	// fault is localized to ARMOR's serving path vs. the stored data.
	FailureConflict FailureClass = "dual_path_conflict"
)

// classFor maps a verification status to a failure class. A passing or pending
// status has no failure class; callers only invoke this on non-pass results.
func classFor(s VerificationStatus) FailureClass {
	switch s {
	case StatusRestoreError:
		return FailureRestoreError
	case StatusChecksumError:
		return FailureChecksumError
	case StatusAssertionError:
		return FailureAssertionError
	case StatusConflict:
		return FailureConflict
	default:
		// Any other non-pass status (e.g. an unknown/future status) is treated
		// as a generic restore error so it still escalates exactly once.
		return FailureRestoreError
	}
}

// dedupeKey is the storm-proof identity of a distinct active failure. The same
// object failing the same way on the same path collapses to one key (one bead);
// a different failure mode, or a different path, on the same object is a
// separate key (a separate bead) — because they are different things to fix.
type dedupeKey struct {
	Bucket       string
	ObjectKey    string
	Path         VerificationPath
	FailureClass FailureClass
}

func (k dedupeKey) String() string {
	return fmt.Sprintf("%s\x1f%s\x1f%s\x1f%s", k.Bucket, k.ObjectKey, k.Path, k.FailureClass)
}

// Provenance carries writer-version provenance for an object, captured "where
// available". The restore-verifier does not currently walk the provenance chain
// (`.armor/chain-head/<writer>`), so the writer version comes from the object's
// ARMOR envelope metadata; WriterID is populated when chain lookup is wired.
type Provenance struct {
	// EnvelopeVersion is x-amz-meta-armor-version from object metadata (the
	// ARMOR envelope format version that wrote the object).
	EnvelopeVersion string
	// WriterID identifies the writing ARMOR instance, from the provenance
	// chain when available. Empty until chain lookup is integrated.
	WriterID string
}

// BeadPayload is the evidence bundle an escalation bead carries, per ADR-004 §5:
// object key, bucket, deployment, writer version from provenance, and both-path
// evidence.
type BeadPayload struct {
	Kind         BeadKind
	Bucket       string
	ObjectKey    string
	Path         VerificationPath
	FailureClass FailureClass
	ArtifactType ArtifactType
	Deployment   string

	// Provenance / writer version, where available.
	EnvelopeVersion string
	WriterID        string

	// Both-path evidence.
	ExpectedSHA256 string
	ARMORSHA256    string
	DirectSHA256   string
	Error          string
	ARMORLatency   time.Duration
	DirectLatency  time.Duration

	// Staleness evidence (Kind == BeadStaleness only).
	LastVerifiedRestore time.Time
	FreshnessWindow     time.Duration

	// Detected is when the escalation was generated.
	Detected time.Time
}

// BeadKind distinguishes failure escalations from staleness escalations so the
// filer and tests can title/format them differently.
type BeadKind string

const (
	BeadFailure  BeadKind = "failure"
	BeadStaleness BeadKind = "staleness"
)

// Title returns the bead title (<=500 chars, enforced by the beads schema).
func (p BeadPayload) Title() string {
	switch p.Kind {
	case BeadStaleness:
		last := "never"
		if !p.LastVerifiedRestore.IsZero() {
			last = p.LastVerifiedRestore.UTC().Format(time.RFC3339)
		}
		return fmt.Sprintf("RV stale: no verified restore for %s within %s (last: %s)",
			p.Bucket, durHuman(p.FreshnessWindow), last)
	default:
		// The object key is the only unbounded component (S3 keys may be up to
		// 1024 chars). Cap *it* rather than the whole string, so the failure
		// class and path — the parts a human triages by — always survive within
		// the beads schema's 500-char title CHECK.
		const maxTitle = 480
		prefix := fmt.Sprintf("RV escalate: %s on %s/", p.FailureClass, p.Bucket)
		suffix := fmt.Sprintf(" via %s path", p.Path)
		maxKey := maxTitle - len(prefix) - len(suffix) - len("…")
		if maxKey < 16 {
			maxKey = 16
		}
		key := p.ObjectKey
		if len(key) > maxKey {
			key = key[:maxKey] + "…"
		}
		return prefix + key + suffix
	}
}

// Body returns the markdown bead body with full both-path evidence.
func (p BeadPayload) Body() string {
	var b strings.Builder
	if p.Kind == BeadStaleness {
		fmt.Fprintf(&b, "## Stale restore verification\n\n")
		fmt.Fprintf(&b, "- **Bucket:** %s\n", p.Bucket)
		fmt.Fprintf(&b, "- **Deployment:** %s\n", orNA(p.Deployment))
		last := "never (no verified restore since verifier start)"
		if !p.LastVerifiedRestore.IsZero() {
			last = p.LastVerifiedRestore.UTC().Format(time.RFC3339)
		}
		fmt.Fprintf(&b, "- **Last verified restore:** %s\n", last)
		fmt.Fprintf(&b, "- **Freshness window:** %s\n", durHuman(p.FreshnessWindow))
		fmt.Fprintf(&b, "- **Detected:** %s\n\n", p.Detected.UTC().Format(time.RFC3339))
		b.WriteString("No verified restore has completed within the freshness window. " +
			"Escalated once per window (never per scheduler tick) per ADR-004 §5.\n")
		return b.String()
	}

	fmt.Fprintf(&b, "## Restore verification failure\n\n")
	fmt.Fprintf(&b, "- **Bucket:** %s\n", p.Bucket)
	fmt.Fprintf(&b, "- **Object key:** %s\n", p.ObjectKey)
	fmt.Fprintf(&b, "- **Path:** %s\n", p.Path)
	fmt.Fprintf(&b, "- **Failure class:** %s\n", p.FailureClass)
	fmt.Fprintf(&b, "- **Artifact type:** %s\n", p.ArtifactType)
	fmt.Fprintf(&b, "- **Deployment:** %s\n", orNA(p.Deployment))
	fmt.Fprintf(&b, "- **Detected:** %s\n\n", p.Detected.UTC().Format(time.RFC3339))

	b.WriteString("## Provenance / writer version\n")
	fmt.Fprintf(&b, "- **Envelope version:** %s\n", orNA(p.EnvelopeVersion))
	fmt.Fprintf(&b, "- **Writer ID:** %s\n\n", orNA2(p.WriterID, "provenance chain not wired into verifier"))

	b.WriteString("## Both-path evidence\n")
	fmt.Fprintf(&b, "- **Expected SHA-256:** %s\n", orNA2(p.ExpectedSHA256, "(none declared in metadata)"))
	fmt.Fprintf(&b, "- **ARMOR path SHA-256:** %s\n", orNA2(p.ARMORSHA256, "(path did not complete)"))
	fmt.Fprintf(&b, "- **Direct path SHA-256:** %s\n", orNA2(p.DirectSHA256, "(path did not complete)"))
	fmt.Fprintf(&b, "- **ARMOR path latency:** %s\n", durHuman(p.ARMORLatency))
	fmt.Fprintf(&b, "- **Direct path latency:** %s\n\n", durHuman(p.DirectLatency))

	b.WriteString("## Error\n")
	if p.Error != "" {
		b.WriteString("```\n")
		b.WriteString(truncate(p.Error, 4096))
		b.WriteString("\n```\n\n")
	} else {
		b.WriteString("(no error string)\n\n")
	}
	b.WriteString("Filed by restore-verifier escalation (ADR-004 §5). One bead per distinct " +
		"failure — dedupe key = bucket + object key + path + failure class; storm-proof across " +
		"scheduler ticks. This is an escalation, not a retry: do not file counter/attempt beads " +
		"or loop in response.\n")
	return b.String()
}

// BeadFiler files an escalation bead. The interface lets unit tests exercise
// the Escalator's dedupe/staleness logic against a fake without invoking the
// bf CLI or touching the live beads store.
//
// File must be idempotent from the caller's perspective only in that the
// Escalator guarantees it is called at most once per active dedupe key; the
// filer itself performs no dedupe. File must not retry — a failed filing is
// reported as an error and the dedupe key is left unrecorded so the next
// scheduler tick may make one further bounded attempt.
type BeadFiler interface {
	File(ctx context.Context, payload BeadPayload) (beadID string, err error)
}

// noopFiler discards every filing. It is the default when escalation is
// configured off, keeping the verifier's behavior unchanged.
type noopFiler struct{}

func (noopFiler) File(context.Context, BeadPayload) (string, error) { return "", nil }

// Escalator files exactly one bead per distinct active failure and one
// staleness bead per freshness window per bucket. It is storm-proof: a
// persisted dedupe set means a failing object never files more than one bead
// across scheduler ticks or process restarts.
//
// The zero-value Escalator is not usable; construct with NewEscalator.
type Escalator struct {
	filer     BeadFiler
	deployment string
	window    time.Duration

	mu   sync.Mutex
	path string // persistence file; "" = in-memory only
	now  func() time.Time

	// failures is the persisted set of dedupe keys that already have an active
	// escalation bead. A key is present iff a bead has been filed for that
	// distinct failure and the object has not since passed (which clears it).
	failures map[string]bool
	// staleness is the persisted per-bucket last-escalation timestamp. A
	// staleness bead is filed at most once per freshness window.
	staleness map[string]time.Time
}

// EscalatorConfig configures an Escalator.
type EscalatorConfig struct {
	Filer           BeadFiler // required; use NoopFiler() to disable filing
	Deployment      string    // deployment name included in every bead body
	FreshnessWindow time.Duration
	StatePath       string // persistence file; "" = in-memory only (still storm-proof within a process)
	Now             func() time.Time
}

// NewEscalator constructs an Escalator, loading any persisted dedupe state from
// StatePath. On load error the escalator starts empty rather than failing — a
// lost dedupe set can at worst cause one extra bead per currently-active
// failure, which is acceptable and self-corrects.
func NewEscalator(cfg EscalatorConfig) *Escalator {
	filer := cfg.Filer
	if filer == nil {
		filer = noopFiler{}
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	e := &Escalator{
		filer:      filer,
		deployment: cfg.Deployment,
		window:     cfg.FreshnessWindow,
		path:       cfg.StatePath,
		now:        now,
		failures:   make(map[string]bool),
		staleness:  make(map[string]time.Time),
	}
	if cfg.StatePath != "" {
		e.load()
	}
	return e
}

// NoopFiler returns a filer that discards every filing, for configurations that
// want escalation tracking (dedupe/staleness state) without filing beads.
func NoopFiler() BeadFiler { return noopFiler{} }

// EscalateFailure files a bead for a distinct active failure if one has not
// already been filed for this dedupe key, and records the key in the persisted
// set. It returns the filed bead ID ("" if deduped or escalation disabled).
//
// The result must be a non-pass result; caller responsibility. The object
// sample's metadata supplies the envelope version for provenance.
func (e *Escalator) EscalateFailure(ctx context.Context, r VerificationResult, prov Provenance) (string, error) {
	if e == nil {
		return "", nil
	}
	key := dedupeKey{
		Bucket:       r.Bucket,
		ObjectKey:    r.Key,
		Path:         r.Path,
		FailureClass: classFor(r.Status),
	}

	e.mu.Lock()
	if e.failures[key.String()] {
		e.mu.Unlock()
		return "", nil // already escalated this active failure
	}
	e.mu.Unlock()

	payload := e.failurePayload(r, prov)
	id, err := e.filer.File(ctx, payload)
	if err != nil {
		// Do NOT record the key: the next tick may make one further bounded
		// attempt. This is the only re-attempt path and it is rate-limited by
		// the schedule cadence — never a retry loop.
		return "", fmt.Errorf("escalation file failed for %s: %w", key, err)
	}

	e.mu.Lock()
	e.failures[key.String()] = true
	e.persistLocked()
	e.mu.Unlock()
	return id, nil
}

// ClearObject removes every dedupe key for an object that has since passed
// verification, so a genuine regression after recovery files a fresh bead.
// This makes escalation track *active* failures: exactly one bead per active
// breakage, never an unbounded accumulation.
func (e *Escalator) ClearObject(bucket, objectKey string) {
	if e == nil {
		return
	}
	prefix := fmt.Sprintf("%s\x1f%s\x1f", bucket, objectKey)
	e.mu.Lock()
	defer e.mu.Unlock()
	changed := false
	for k := range e.failures {
		if strings.HasPrefix(k, prefix) {
			delete(e.failures, k)
			changed = true
		}
	}
	if changed {
		e.persistLocked()
	}
}

// EscalateStaleness files one staleness bead per freshness window when a bucket
// has no verified restore within the window (lastSuccess zero, or older than the
// window). It is deduped per window, never per tick. Returns the filed bead ID
// ("" if not stale this window, or escalation disabled).
func (e *Escalator) EscalateStaleness(ctx context.Context, bucket string, lastSuccess time.Time) (string, error) {
	if e == nil || e.window <= 0 {
		return "", nil
	}
	now := e.now()
	stale := lastSuccess.IsZero() || now.Sub(lastSuccess) > e.window
	if !stale {
		return "", nil
	}

	e.mu.Lock()
	last := e.staleness[bucket]
	if !last.IsZero() && now.Sub(last) < e.window {
		e.mu.Unlock()
		return "", nil // already escalated in this window
	}
	e.mu.Unlock()

	payload := BeadPayload{
		Kind:                BeadStaleness,
		Bucket:              bucket,
		Deployment:          e.deployment,
		LastVerifiedRestore: lastSuccess,
		FreshnessWindow:     e.window,
		Detected:            now,
	}
	id, err := e.filer.File(ctx, payload)
	if err != nil {
		return "", fmt.Errorf("staleness escalation file failed for %s: %w", bucket, err)
	}

	e.mu.Lock()
	e.staleness[bucket] = now
	e.persistLocked()
	e.mu.Unlock()
	return id, nil
}

// failurePayload builds the BeadPayload for a verification failure.
func (e *Escalator) failurePayload(r VerificationResult, prov Provenance) BeadPayload {
	return BeadPayload{
		Kind:            BeadFailure,
		Bucket:          r.Bucket,
		ObjectKey:       r.Key,
		Path:            r.Path,
		FailureClass:    classFor(r.Status),
		ArtifactType:    r.ArtifactType,
		Deployment:      e.deployment,
		EnvelopeVersion: prov.EnvelopeVersion,
		WriterID:        prov.WriterID,
		ExpectedSHA256:  r.ExpectedSHA256,
		ARMORSHA256:     r.ARMORSHA256,
		DirectSHA256:    r.DirectSHA256,
		Error:           r.Error,
		ARMORLatency:    r.ARMORPathLatency,
		DirectLatency:   r.DirectPathLatency,
		Detected:        e.now(),
	}
}

// escalationState is the persisted on-disk representation of the dedupe set.
type escalationState struct {
	Failures  map[string]bool    `json:"failures"`
	Staleness map[string]string  `json:"staleness"` // bucket -> RFC3339
}

// load reads the persisted dedupe state. A missing file is not an error (fresh
// start); a malformed file logs and starts empty rather than blocking escalation.
func (e *Escalator) load() {
	data, err := os.ReadFile(e.path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "restore-verifier: escalation state load (%s): %v; starting empty\n", e.path, err)
		}
		return
	}
	var s escalationState
	if err := json.Unmarshal(data, &s); err != nil {
		fmt.Fprintf(os.Stderr, "restore-verifier: escalation state parse (%s): %v; starting empty\n", e.path, err)
		return
	}
	if s.Failures != nil {
		e.failures = s.Failures
	}
	for bucket, ts := range s.Staleness {
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			e.staleness[bucket] = t
		}
	}
}

// persistLocked writes the dedupe state atomically. Caller holds e.mu.
func (e *Escalator) persistLocked() {
	if e.path == "" {
		return
	}
	s := escalationState{
		Failures:  e.failures,
		Staleness: make(map[string]string, len(e.staleness)),
	}
	for bucket, t := range e.staleness {
		s.Staleness[bucket] = t.UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	dir := filepath.Dir(e.path)
	tmp, err := os.CreateTemp(dir, ".escalation-state-*.json")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op on the successful rename path
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return
	}
	if err := tmp.Close(); err != nil {
		return
	}
	// Atomic replace: a crash between write and rename leaves the temp file,
	// never a half-written state file. os.Rename overwrites the destination.
	if err := os.Rename(tmpName, e.path); err != nil {
		return
	}
}

// BFCLIFiler files escalation beads by shelling out to the bf CLI (the `br`
// binary, i.e. bead-forge). It performs no dedupe and never retries — one exec
// per call, bounded by execTimeout.
type BFCLIFiler struct {
	// Binary is the path to the br CLI. Defaults to "br" (resolved via PATH).
	Binary string
	// Workspace is the -w flag value (beads workspace dir). Empty uses the
	// CLI's default (cwd .beads/).
	Workspace string
	// BeadType is the --type value. Defaults to "bug" (verification failures
	// and staleness are defects requiring attention).
	BeadType string
	// Label is an optional --label applied to every escalation bead.
	Label string
	// Priority is the --priority value (0=Critical..4=Backlog). Defaults to 1
	// (High) — restore unavailability is high-severity.
	Priority int
	// ExecTimeout bounds a single bf create call so a hung CLI cannot stall
	// escalation. Defaults to 10s.
	ExecTimeout time.Duration
}

// File runs `br create` and returns the printed bead ID.
func (f *BFCLIFiler) File(ctx context.Context, p BeadPayload) (string, error) {
	binary := f.Binary
	if binary == "" {
		binary = "br"
	}
	beadType := f.BeadType
	if beadType == "" {
		beadType = "bug"
	}
	timeout := f.ExecTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := []string{
		"create",
		"--title", p.Title(),
		"--type", beadType,
		"--priority", fmt.Sprintf("%d", clampPriority(f.Priority)),
		"--description", p.Body(),
	}
	if f.Workspace != "" {
		args = append(args, "-w", f.Workspace)
	}
	if f.Label != "" {
		args = append(args, "--label", f.Label)
	}

	cmd := exec.CommandContext(callCtx, binary, args...)
	// WaitDelay makes Output() give up promptly once the context deadline fires.
	// CommandContext alone sends SIGKILL only to the immediate process; a child
	// that inherits the stdout pipe (e.g. a shell wrapper, or a hung bf child)
	// can keep Output blocked on that pipe indefinitely. WaitDeadline starts the
	// moment the process is killed and forces pipe closure, so a single call is
	// truly bounded by ~ExecTimeout + WaitDelay — the storm-proof "never hangs"
	// guarantee.
	cmd.WaitDelay = timeout
	out, err := cmd.Output()
	if err != nil {
		stderr := ""
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		if callCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("br create timed out after %s: %s", timeout, stderr)
		}
		return "", fmt.Errorf("br create failed: %w: %s", err, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}

// clampPriority maps any int into the beads schema's allowed range [0,4].
func clampPriority(p int) int {
	switch {
	case p <= 0:
		return 0
	case p >= 4:
		return 4
	default:
		return p
	}
}

// --- small formatting helpers ---

func durHuman(d time.Duration) string {
	if d <= 0 {
		return "n/a"
	}
	return d.Round(time.Second).String()
}

func orNA(s string) string {
	if s == "" {
		return "n/a"
	}
	return s
}

func orNA2(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + " …[truncated]"
}
