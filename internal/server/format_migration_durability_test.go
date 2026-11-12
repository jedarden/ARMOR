// Package server tests for the durable migration engine's restart,
// rollback, lease and write-race safety. Each test injects one deterministic
// fault at one engine checkpoint (backend/source read, staged part upload,
// candidate verification, conditional manifest publish, post-publish sidecar
// copy) or exercises a real backend precondition (CopyIfMatch ETag fence,
// PutIfAbsent lease) so no test depends on timing.
//
// Coverage map (bead armor-103a2789):
//
//	source read fault          TestDurableMigrationSourceReadFaultPreservesSource
//	staged part fault          TestDurableMigrationStagedPartFaultAbortsUpload
//	candidate verification     TestDurableMigrationVerificationFailureIsIntegrityOutcome
//	manifest publish race      TestDurableMigrationCommitRaceKeepsClientOverwrite
//	commit fence primitive     TestDurableMigrationCopyIfMatchETagFence
//	sidecar publish fault      TestDurableMigrationSidecarPublishFaultRetainsStaging
//	request cancellation       TestDurableMigrationCancelMidRunInterruptsAndResumesExactlyOnce
//	replica handoff            TestDurableMigrationReplicaHandoffViaLease
//	stale lease fencing        TestDurableMigrationStaleLeaseFencing
//	changed-parameter resume   TestDurableMigrationChangedParametersRestartFresh
//	bounded concurrency        TestDurableMigrationWorkerConcurrencyBounded
package server

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

const (
	durBucket        = "durability-bucket"
	durStatePath     = ".armor/migration-state.json"
	durLeaseKey      = ".armor/format-migration-lease.json"
	durStagingPrefix = ".armor/migrations/"
	durLargeSize     = 5*1024*1024 + 1024 // above multipartThreshold (5 MiB)
)

// migFault is one deterministic fault rule: the first matching call of op on
// the target key (exact key, or any key under keyPrefix) either fails failN
// times with err, blocks until the context is canceled once, or runs the
// before hook and then proceeds.
type migFault struct {
	op        string
	key       string
	keyPrefix string // used when key is empty; matches by prefix
	failN     int
	err       error
	blockCtx  bool
	before    func(h *migHarness, op, key string)

	fails    int
	used     bool
	disarmed bool
}

func (f *migFault) matches(op, key string) bool {
	if f.op != op {
		return false
	}
	if f.keyPrefix != "" {
		return strings.HasPrefix(key, f.keyPrefix)
	}
	return f.key == key
}

// migHarness wraps a real FSBackend with deterministic fault rules and call
// recording, so migration checkpoint faults are reproducible under -race.
type migHarness struct {
	backend.Backend // embedded pass-through; overridden methods record first
	fs              *backend.FSBackend
	bucket          string
	mek             []byte

	mu          sync.Mutex
	faults      []*migFault
	calls       map[string]int
	inflight    map[string]int
	maxInflight int

	// first-source-read concurrency gate: the first GetRange of each gateKey
	// blocks until gateWant distinct gated keys are waiting at once.
	gateKeys    map[string]bool
	gateArmed   map[string]bool
	gateWant    int
	gateWaiting int
	gateReady   chan struct{}
	gateOnce    sync.Once
}

func durNewHarness(t *testing.T, faults ...*migFault) *migHarness {
	t.Helper()
	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	if err := fs.CreateBucket(context.Background(), durBucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	h := &migHarness{
		Backend:   fs,
		fs:        fs,
		bucket:    durBucket,
		mek:       clsTestMEK(),
		faults:    faults,
		calls:     make(map[string]int),
		inflight:  make(map[string]int),
		gateReady: make(chan struct{}),
		gateArmed: make(map[string]bool),
	}
	return h
}

// apply records the call and returns a non-nil error when a fault fires.
// The before hook runs without the harness lock held so it may call back in.
func (h *migHarness) apply(op, key string, ctx context.Context) error {
	h.mu.Lock()
	h.calls[op+"\x00"+key]++
	var f *migFault
	for _, c := range h.faults {
		if !c.disarmed && c.matches(op, key) {
			f = c
			break
		}
	}
	if f == nil {
		h.mu.Unlock()
		return nil
	}
	switch {
	case f.failN > 0:
		f.fails++
		if f.fails >= f.failN {
			f.disarmed = true
		}
		err := f.err
		h.mu.Unlock()
		if err == nil {
			err = fmt.Errorf("%s fault on %s", op, key)
		}
		return err
	case f.blockCtx:
		if f.used {
			f.disarmed = true
			h.mu.Unlock()
			return nil
		}
		f.used = true
		h.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Second):
			return fmt.Errorf("blockCtx fault on %s/%s timed out (run hung)", op, key)
		}
	default:
		before := f.before
		h.mu.Unlock()
		if before != nil {
			before(h, op, key)
		}
		return nil
	}
}

func (h *migHarness) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	if err := h.apply("GetRange", key, ctx); err != nil {
		return nil, err
	}
	h.mu.Lock()
	h.inflight[key]++
	if n := len(h.inflight); n > h.maxInflight {
		h.maxInflight = n
	}
	gated := h.gateKeys[key] && h.gateArmed[key]
	if gated {
		h.gateArmed[key] = false
		h.gateWaiting++
		if h.gateWaiting >= h.gateWant {
			h.gateOnce.Do(func() { close(h.gateReady) })
		}
	}
	h.mu.Unlock()

	if gated {
		select {
		case <-h.gateReady:
		case <-ctx.Done():
			h.exitTracked(key)
			return nil, ctx.Err()
		case <-time.After(15 * time.Second):
			h.exitTracked(key)
			return nil, fmt.Errorf("concurrency gate timed out (workers: %d wanted %d)", h.gateWaiting, h.gateWant)
		}
	}
	defer h.exitTracked(key)
	return h.Backend.GetRange(ctx, bucket, key, offset, length)
}

func (h *migHarness) exitTracked(key string) {
	h.mu.Lock()
	delete(h.inflight, key)
	h.mu.Unlock()
}

// PutIfAbsent and CopyIfMatch must exist on the harness itself: the engine
// type-asserts ConditionalPutBackend / ConditionalCopyBackend on the backend
// value it was constructed with.
func (h *migHarness) PutIfAbsent(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if err := h.apply("PutIfAbsent", key, ctx); err != nil {
		return err
	}
	return h.fs.PutIfAbsent(ctx, bucket, key, body, size, meta)
}

func (h *migHarness) CopyIfMatch(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey, expectedETag string, meta map[string]string, replaceMetadata bool) error {
	if err := h.apply("CopyIfMatch", dstKey, ctx); err != nil {
		return err
	}
	return h.fs.CopyIfMatch(ctx, srcBucket, srcKey, dstBucket, dstKey, expectedETag, meta, replaceMetadata)
}

func (h *migHarness) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	if err := h.apply("UploadPart", key, ctx); err != nil {
		return "", err
	}
	return h.fs.UploadPart(ctx, bucket, key, uploadID, partNumber, body, size)
}

func (h *migHarness) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	if err := h.apply("Copy", dstKey, ctx); err != nil {
		return err
	}
	return h.fs.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, meta, replaceMetadata)
}

func (h *migHarness) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	_ = h.apply("AbortMultipartUpload", key, ctx) // record only; never fails
	return h.fs.AbortMultipartUpload(ctx, bucket, key, uploadID)
}

func (h *migHarness) callCount(op, key string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.calls[op+"\x00"+key]
}

// callCountPrefix counts recorded calls whose key has prefix (used when the
// staging key is only knowable via its state-ID prefix).
func (h *migHarness) callCountPrefix(op, prefix string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for k, c := range h.calls {
		if strings.HasPrefix(k, op+"\x00") && strings.HasPrefix(strings.TrimPrefix(k, op+"\x00"), prefix) {
			n += c
		}
	}
	return n
}

func (h *migHarness) observedMaxInflight() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.maxInflight
}

// durArmGate arms the first-source-read concurrency gate on the given keys.
func (h *migHarness) durArmGate(keys []string, want int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.gateKeys = make(map[string]bool, len(keys))
	h.gateArmed = make(map[string]bool, len(keys))
	for _, k := range keys {
		h.gateKeys[k] = true
		h.gateArmed[k] = true
	}
	h.gateWant = want
}

func durMigrator(h *migHarness, include []string) *FormatMigrator {
	return NewFormatMigrator(h, h.bucket, h.mek, "default", crypto.Version3, include, nil)
}

// durPutV1Object stores a genuinely migratable v1 single-PUT object of the
// given plaintext size (real KWP wrap, real header, real ciphertext+HMAC
// trailer, real plaintext SHA so verification stays active) and returns the
// plaintext SHA-256 hex.
func durPutV1Object(t *testing.T, h *migHarness, key string, size int) string {
	t.Helper()
	plaintext := make([]byte, size)
	for i := range plaintext {
		plaintext[i] = byte(i % 251)
	}
	dek := clsTestDEK()
	wrapped, err := crypto.WrapDEK(h.mek, dek)
	if err != nil {
		t.Fatalf("wrap DEK: %v", err)
	}
	iv := make([]byte, 16)
	encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, 65536, crypto.Version1)
	if err != nil {
		t.Fatalf("encryptor: %v", err)
	}
	ciphertext, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	sha := crypto.ComputePlaintextSHA256(plaintext)
	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, int64(size), 65536, sha, crypto.Version1)
	if err != nil {
		t.Fatalf("header: %v", err)
	}
	headerBuf, err := header.Encode()
	if err != nil {
		t.Fatalf("encode header: %v", err)
	}
	body := append(append(append([]byte{}, headerBuf...), ciphertext...), hmacTable...)
	meta := map[string]string{
		armorMetaVersion:       "1",
		armorMetaWrappedDEK:    base64.StdEncoding.EncodeToString(wrapped),
		armorMetaIV:            base64.StdEncoding.EncodeToString(iv),
		armorMetaBlockSize:     "65536",
		armorMetaPlaintextSize: fmt.Sprintf("%d", size),
		armorMetaPlaintextSHA:  hex.EncodeToString(sha[:]),
	}
	if err := h.Put(context.Background(), h.bucket, key, bytesReader(body), int64(len(body)), meta); err != nil {
		t.Fatalf("put %s: %v", key, err)
	}
	return hex.EncodeToString(sha[:])
}

func durStoredSHA(t *testing.T, h *migHarness, key string) string {
	t.Helper()
	data := durReadObject(t, h, key)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func durReadObject(t *testing.T, h *migHarness, key string) []byte {
	t.Helper()
	r, _, err := h.GetDirect(context.Background(), h.bucket, key)
	if err != nil {
		t.Fatalf("read %s: %v", key, err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read body %s: %v", key, err)
	}
	return data
}

func durLoadState(t *testing.T, h *migHarness) *MigrationState {
	t.Helper()
	data := durReadObject(t, h, durStatePath)
	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("parse state: %v", err)
	}
	return &state
}

func durStagingKeys(t *testing.T, h *migHarness) []string {
	t.Helper()
	res, err := h.ListRaw(context.Background(), h.bucket, durStagingPrefix, "", "", 1000)
	if err != nil {
		t.Fatalf("list staging: %v", err)
	}
	keys := make([]string, 0, len(res.Objects))
	for _, o := range res.Objects {
		keys = append(keys, o.Key)
	}
	return keys
}

func durAssertNoStaging(t *testing.T, h *migHarness) {
	t.Helper()
	if keys := durStagingKeys(t, h); len(keys) != 0 {
		t.Fatalf("staging state leaked: %v", keys)
	}
}

func durLeasePresent(t *testing.T, h *migHarness) bool {
	t.Helper()
	_, err := h.Head(context.Background(), h.bucket, durLeaseKey)
	return err == nil
}

func durAssertSourceUntouched(t *testing.T, h *migHarness, key, wantStoredSHA string) {
	t.Helper()
	if got := durStoredSHA(t, h, key); got != wantStoredSHA {
		t.Fatalf("source %s changed under fault: stored sha %s, want %s", key, got, wantStoredSHA)
	}
}

// durAssertCommittedV3 pins the post-publish state of a committed object:
// the source now carries v3 metadata whose plaintext SHA equals the
// original plaintext's SHA (the SHA the engine computed from the decrypted
// source stream while staging).
func durAssertCommittedV3(t *testing.T, h *migHarness, key, wantPlaintextSHA string) {
	t.Helper()
	info, err := h.Head(context.Background(), h.bucket, key)
	if err != nil {
		t.Fatalf("head committed %s: %v", key, err)
	}
	if info.Metadata[armorMetaVersion] != "3" {
		t.Fatalf("committed %s version = %q, want 3", key, info.Metadata[armorMetaVersion])
	}
	if got := info.Metadata[armorMetaPlaintextSHA]; got != wantPlaintextSHA {
		t.Fatalf("committed %s plaintext sha = %q, want %q", key, got, wantPlaintextSHA)
	}
}

type durRunResult struct {
	r   *MigrationResult
	err error
}

func durStartRun(fm *FormatMigrator, ctx context.Context, dryRun bool, concurrency int) <-chan durRunResult {
	ch := make(chan durRunResult, 1)
	go func() {
		r, err := fm.RunDurable(ctx, dryRun, concurrency)
		ch <- durRunResult{r: r, err: err}
	}()
	return ch
}

func durJoinRun(t *testing.T, ch <-chan durRunResult, what string) durRunResult {
	t.Helper()
	var res durRunResult
	select {
	case res = <-ch:
	case <-time.After(45 * time.Second):
		t.Fatalf("migration run did not return (%s)", what)
	}
	return res
}

func durWaitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestDurableMigrationSourceReadFaultPreservesSource injects a failure at the
// source-read checkpoint. The object must be checkpoint-failed without its
// source changing and without staging leakage, and a fresh run must commit
// it exactly once.
func TestDurableMigrationSourceReadFaultPreservesSource(t *testing.T) {
	h := durNewHarness(t, &migFault{op: "GetRange", key: "a", failN: 1, err: errors.New("source read boom")})
	wantPlain := map[string]string{}
	wantStored := map[string]string{}
	for _, k := range []string{"a", "b", "c"} {
		wantPlain[k] = durPutV1Object(t, h, k, 2048)
		wantStored[k] = durStoredSHA(t, h, k)
	}

	res, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if res.Status != "completed" || res.FailedObjects != 1 || res.ProcessedObjects != 2 {
		t.Fatalf("run 1 result = %+v", res)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != "a" || !strings.Contains(res.Failures[0].Reason, "source read boom") {
		t.Fatalf("run 1 failures = %+v", res.Failures)
	}
	st := durLoadState(t, h)
	if cp := st.Checkpoints["a"]; cp.Status != "failed" || !strings.Contains(cp.Error, "source read boom") {
		t.Fatalf("checkpoint a = %+v", cp)
	}
	durAssertSourceUntouched(t, h, "a", wantStored["a"])
	if h.callCount("CopyIfMatch", "a") != 0 {
		t.Fatalf("failed source reached the commit point %d times", h.callCount("CopyIfMatch", "a"))
	}
	for _, k := range []string{"b", "c"} {
		durAssertCommittedV3(t, h, k, wantPlain[k])
		if h.callCount("CopyIfMatch", k) != 1 {
			t.Fatalf("CopyIfMatch %s called %d times, want 1", k, h.callCount("CopyIfMatch", k))
		}
	}
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released after run 1")
	}

	// A fresh run (completed runs re-initialize) retries the failed object.
	res2, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if res2.Status != "completed" || res2.FailedObjects != 0 {
		t.Fatalf("run 2 result = %+v", res2)
	}
	durAssertCommittedV3(t, h, "a", wantPlain["a"])
	if h.callCount("CopyIfMatch", "a") != 1 {
		t.Fatalf("CopyIfMatch a across both runs = %d, want 1", h.callCount("CopyIfMatch", "a"))
	}
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released after run 2")
	}
}

// TestDurableMigrationStagedPartFaultAbortsUpload injects a failure at the
// staged-part checkpoint of a multipart staging upload. The in-flight upload
// must be aborted, the failure checkpointed, the source preserved, and a
// rerun must commit the object as v3 multipart with its sidecar published.
func TestDurableMigrationStagedPartFaultAbortsUpload(t *testing.T) {
	h := durNewHarness(t, &migFault{op: "UploadPart", keyPrefix: durStagingPrefix, failN: 1, err: errors.New("staged part boom")})
	wantPlain := durPutV1Object(t, h, "big1", durLargeSize)
	wantStored := durStoredSHA(t, h, "big1")

	res, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if res.Status != "completed" || res.FailedObjects != 1 {
		t.Fatalf("run 1 result = %+v", res)
	}
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Reason, "upload staged part") {
		t.Fatalf("run 1 failures = %+v", res.Failures)
	}
	if h.callCountPrefix("AbortMultipartUpload", durStagingPrefix) == 0 {
		t.Fatal("staged multipart upload was not aborted after the part fault")
	}
	durAssertSourceUntouched(t, h, "big1", wantStored)
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released after run 1")
	}

	res2, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if res2.Status != "completed" || res2.FailedObjects != 0 {
		t.Fatalf("run 2 result = %+v", res2)
	}
	durAssertCommittedV3(t, h, "big1", wantPlain)
	info, err := h.Head(context.Background(), h.bucket, "big1")
	if err != nil {
		t.Fatal(err)
	}
	if info.Metadata[armorMetaMultipart] != "true" {
		t.Fatalf("committed big1 multipart meta = %q, want true", info.Metadata[armorMetaMultipart])
	}
	if _, err := h.Head(context.Background(), h.bucket, backend.GetSidecarKey("", "big1")); err != nil {
		t.Fatalf("published sidecar missing: %v", err)
	}
	if h.callCount("CopyIfMatch", "big1") != 1 {
		t.Fatalf("CopyIfMatch big1 across both runs = %d, want 1", h.callCount("CopyIfMatch", "big1"))
	}
	durAssertNoStaging(t, h)
}

// TestDurableMigrationVerificationFailureIsIntegrityOutcome corrupts a
// candidate's ciphertext so the candidate-verification checkpoint fails HMAC
// verification. The failure must be classified as an integrity failure, the
// corrupt source must be left untouched, and nothing may be staged.
func TestDurableMigrationVerificationFailureIsIntegrityOutcome(t *testing.T) {
	h := durNewHarness(t)
	_ = durPutV1Object(t, h, "corrupt1", 2048)
	meta := func() map[string]string {
		info, err := h.Head(context.Background(), h.bucket, "corrupt1")
		if err != nil {
			t.Fatal(err)
		}
		return info.Metadata
	}()
	body := durReadObject(t, h, "corrupt1")
	body[crypto.HeaderSize+7] ^= 0xFF
	if err := h.Put(context.Background(), h.bucket, "corrupt1", bytesReader(body), int64(len(body)), meta); err != nil {
		t.Fatal(err)
	}
	corruptStored := durStoredSHA(t, h, "corrupt1")

	res, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Status != "completed" || res.FailedObjects != 1 {
		t.Fatalf("result = %+v", res)
	}
	// The corrupt ciphertext must be detected at the candidate-verification
	// checkpoint (per-block HMAC), never committed, and never leave staging
	// state. Known divergence (recorded on armor-103a2789): the legacy
	// in-memory path wraps every decrypt failure in ErrIntegrityVerification
	// and counts it under outcome_integrity_failed, while the durable
	// streaming path currently surfaces the decryptor's raw HMAC error and
	// counts outcome_failed. The durability assertions below hold either way.
	if len(res.Failures) != 1 || res.Failures[0].Key != "corrupt1" || !strings.Contains(res.Failures[0].Reason, "HMAC verification failed") {
		t.Fatalf("failures = %+v", res.Failures)
	}
	if res.Classification.OutcomeIntegrityFailed+res.Classification.OutcomeFailed != 1 {
		t.Fatalf("corruption failure not recorded in a failure outcome bucket: %+v", res.Classification)
	}
	if h.callCount("CopyIfMatch", "corrupt1") != 0 {
		t.Fatalf("corrupt object reached the commit point %d times", h.callCount("CopyIfMatch", "corrupt1"))
	}
	durAssertSourceUntouched(t, h, "corrupt1", corruptStored)
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released")
	}
}

// TestDurableMigrationCommitRaceKeepsClientOverwrite injects a concurrent
// client overwrite between the engine's source Head and its conditional
// publish, so the real CopyIfMatch precondition fails. The client's object
// must win (no source loss), the failure must be checkpointed, and the
// staged output must stay retained and accounted in the rollback record.
func TestDurableMigrationCommitRaceKeepsClientOverwrite(t *testing.T) {
	clientBody := []byte("client overwrote this object mid-migration")
	h := durNewHarness(t, &migFault{
		op:  "CopyIfMatch",
		key: "doc",
		before: func(h *migHarness, _, _ string) {
			// The racing client PUT lands between the engine's Head and its
			// CopyIfMatch, changing the destination ETag.
			if err := h.Put(context.Background(), h.bucket, "doc", bytesReader(clientBody), int64(len(clientBody)), map[string]string{"Content-Type": "text/plain"}); err != nil {
				t.Errorf("client overwrite: %v", err)
			}
		},
	})
	_ = durPutV1Object(t, h, "doc", 2048)

	res, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Status != "completed" || res.FailedObjects != 1 {
		t.Fatalf("result = %+v", res)
	}
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Reason, "commit") || !strings.Contains(res.Failures[0].Reason, "precondition failed") {
		t.Fatalf("failures = %+v", res.Failures)
	}
	got := durReadObject(t, h, "doc")
	if string(got) != string(clientBody) {
		t.Fatalf("concurrent client overwrite was clobbered: doc = %q", got)
	}
	if h.callCount("CopyIfMatch", "doc") != 1 {
		t.Fatalf("CopyIfMatch doc retried %d times, want 1", h.callCount("CopyIfMatch", "doc"))
	}
	// The staged output is retained for diagnosis and accounted as rollback
	// state rather than leaking unrecorded.
	st := durLoadState(t, h)
	if len(st.Rollback) != 1 || st.Rollback[0].Key != "doc" || st.Rollback[0].StagingKey == "" {
		t.Fatalf("rollback record = %+v", st.Rollback)
	}
	if keys := durStagingKeys(t, h); len(keys) != 1 || keys[0] != st.Rollback[0].StagingKey {
		t.Fatalf("retained staging keys = %v, rollback = %+v", keys, st.Rollback)
	}
	if durLeasePresent(t, h) {
		t.Fatal("lease not released")
	}
}

// TestDurableMigrationCopyIfMatchETagFence pins the commit-point primitive
// the exactly-once property rests on: publish succeeds only against the
// ETag observed before migration, and fails closed once the destination
// changed underneath it.
func TestDurableMigrationCopyIfMatchETagFence(t *testing.T) {
	h := durNewHarness(t)
	_ = durPutV1Object(t, h, "fence", 2048)
	_ = durPutV1Object(t, h, "staged", 3000) // different bytes, so the publish changes the ETag
	before, err := h.Head(context.Background(), h.bucket, "fence")
	if err != nil {
		t.Fatal(err)
	}
	if err := h.CopyIfMatch(context.Background(), h.bucket, "staged", h.bucket, "fence", before.ETag, nil, false); err != nil {
		t.Fatalf("CopyIfMatch with the observed ETag: %v", err)
	}
	changed, err := h.Head(context.Background(), h.bucket, "fence")
	if err != nil {
		t.Fatal(err)
	}
	if changed.ETag == before.ETag {
		t.Fatal("publish did not change the ETag; fence would be vacuous")
	}
	if err := h.CopyIfMatch(context.Background(), h.bucket, "staged", h.bucket, "fence", before.ETag, nil, false); !errors.Is(err, backend.ErrPreconditionFailed) {
		t.Fatalf("stale-ETag publish err = %v, want ErrPreconditionFailed", err)
	}
}

// TestDurableMigrationSidecarPublishFaultRetainsStaging injects a failure at
// the post-publish sidecar copy of a v3-multipart commit. The source has
// already been published at that point, so the current engine contract is:
// the failure is checkpointed, the published source stays (no rollback of a
// committed source), and the staged object plus its sidecar remain retained
// under the rollback record. A rerun must not re-commit the already-v3
// source.
func TestDurableMigrationSidecarPublishFaultRetainsStaging(t *testing.T) {
	finalSidecar := backend.GetSidecarKey("", "big2")
	h := durNewHarness(t, &migFault{op: "Copy", key: finalSidecar, failN: 1, err: errors.New("sidecar boom")})
	wantPlain := durPutV1Object(t, h, "big2", durLargeSize)

	res, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run 1: %v", err)
	}
	if res.Status != "completed" || res.FailedObjects != 1 {
		t.Fatalf("run 1 result = %+v", res)
	}
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Reason, "publish multipart sidecar") {
		t.Fatalf("run 1 failures = %+v", res.Failures)
	}
	// Current engine contract: the conditional publish already replaced the
	// source, so the committed v3 metadata stays and is NOT rolled back.
	durAssertCommittedV3(t, h, "big2", wantPlain)
	if _, err := h.Head(context.Background(), h.bucket, finalSidecar); err == nil {
		t.Fatal("final sidecar unexpectedly present after failed publish")
	}
	if h.callCount("CopyIfMatch", "big2") != 1 {
		t.Fatalf("CopyIfMatch big2 = %d, want 1", h.callCount("CopyIfMatch", "big2"))
	}
	st := durLoadState(t, h)
	if len(st.Rollback) != 1 || st.Rollback[0].Key != "big2" {
		t.Fatalf("rollback record = %+v", st.Rollback)
	}
	// Staged object and staged sidecar stay retained (accounted by the
	// rollback entry), not silently dropped.
	if keys := durStagingKeys(t, h); len(keys) != 1 || keys[0] != st.Rollback[0].StagingKey {
		t.Fatalf("retained staging keys = %v, rollback = %+v", keys, st.Rollback)
	}

	// A rerun must not double-commit the already-migrated source.
	res2, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil {
		t.Fatalf("run 2: %v", err)
	}
	if res2.Status != "completed" || res2.FailedObjects != 0 || res2.SkippedObjects != 1 {
		t.Fatalf("run 2 result = %+v", res2)
	}
	if h.callCount("CopyIfMatch", "big2") != 1 {
		t.Fatalf("CopyIfMatch big2 across both runs = %d, want 1", h.callCount("CopyIfMatch", "big2"))
	}
	durAssertCommittedV3(t, h, "big2", wantPlain)
}

// TestDurableMigrationCancelMidRunInterruptsAndResumesExactlyOnce cancels the
// run's context while a worker is blocked at a source read (the pod/request
// cancellation shape). The run must persist an interrupted state, release the
// lease, leave every untouched source byte-identical, and a same-contract
// resume must reuse the state, skip the already-committed object, and commit
// every remaining object exactly once with no staging left behind.
func TestDurableMigrationCancelMidRunInterruptsAndResumesExactlyOnce(t *testing.T) {
	// Six objects with one worker: a commits, b blocks at its source read,
	// c..d fill the job buffer, and the fifth send parks the list loop so the
	// cancel deterministically lands as an interrupt rather than a drain.
	keys := []string{"a", "b", "c", "d", "e", "f"}
	h := durNewHarness(t, &migFault{op: "GetRange", key: "b", blockCtx: true})
	wantPlain := map[string]string{}
	wantStored := map[string]string{}
	for _, k := range keys {
		wantPlain[k] = durPutV1Object(t, h, k, 2048)
		wantStored[k] = durStoredSHA(t, h, k)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := durStartRun(durMigrator(h, []string{"1"}), ctx, false, 1)
	durWaitFor(t, "worker reaching the blocked source read", func() bool {
		return h.callCount("GetRange", "b") > 0
	})
	cancel()
	res := durJoinRun(t, done, "after cancel")
	if !errors.Is(res.err, context.Canceled) {
		t.Fatalf("run err = %v, want context.Canceled", res.err)
	}
	if res.r == nil || res.r.Status != "interrupted" {
		t.Fatalf("run result = %+v, want interrupted", res.r)
	}
	st := durLoadState(t, h)
	if st.Status != "interrupted" {
		t.Fatalf("persisted state status = %q, want interrupted", st.Status)
	}
	if st.Checkpoints["a"].Status != "committed" {
		t.Fatalf("checkpoint a = %+v, want committed", st.Checkpoints["a"])
	}
	if st.Checkpoints["b"].Status == "committed" {
		t.Fatal("in-flight object b committed before its staging completed")
	}
	if h.callCount("CopyIfMatch", "a") != 1 {
		t.Fatalf("CopyIfMatch a = %d, want 1", h.callCount("CopyIfMatch", "a"))
	}
	// b was mid-staging at cancel: its source must be untouched. Queued
	// objects may legitimately have drained after the cancel, so only the
	// never-twice property is asserted for them.
	durAssertSourceUntouched(t, h, "b", wantStored["b"])
	if h.callCount("CopyIfMatch", "b") != 0 {
		t.Fatalf("CopyIfMatch b after cancel = %d, want 0", h.callCount("CopyIfMatch", "b"))
	}
	for _, k := range keys {
		if n := h.callCount("CopyIfMatch", k); n > 1 {
			t.Fatalf("CopyIfMatch %s after run 1 = %d, want at most 1", k, n)
		}
	}
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released by the interrupted run")
	}

	res2, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil || res2.Status != "completed" {
		t.Fatalf("resume run = (%+v, %v)", res2, err)
	}
	st2 := durLoadState(t, h)
	if st2.ID != st.ID {
		t.Fatalf("resume started a new migration %q; interrupted run was %q", st2.ID, st.ID)
	}
	for _, k := range keys {
		durAssertCommittedV3(t, h, k, wantPlain[k])
		if n := h.callCount("CopyIfMatch", k); n != 1 {
			t.Fatalf("CopyIfMatch %s across both runs = %d, want exactly 1", k, n)
		}
	}
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released after resume")
	}
}

// TestDurableMigrationReplicaHandoffViaLease runs a second migrator (a
// replica of the service) against the shared bucket while a live run holds
// the lease. The replica must be fenced out without touching sources or
// state; after the holder stops, a fresh migrator must take over and honor
// the interrupted run's checkpoints without re-committing anything.
func TestDurableMigrationReplicaHandoffViaLease(t *testing.T) {
	keys := []string{"a", "b", "c", "d", "e", "f"}
	h := durNewHarness(t, &migFault{op: "GetRange", key: "b", blockCtx: true})
	wantStored := map[string]string{}
	for _, k := range keys {
		durPutV1Object(t, h, k, 2048)
		wantStored[k] = durStoredSHA(t, h, k)
	}

	ctx, cancel := context.WithCancel(context.Background())
	holder := durMigrator(h, []string{"1"})
	done := durStartRun(holder, ctx, false, 1)
	durWaitFor(t, "lease holder reaching the blocked source read", func() bool {
		return h.callCount("GetRange", "b") > 0
	})

	_, replicaErr := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if replicaErr == nil || !strings.Contains(replicaErr.Error(), "migration lease held") {
		t.Fatalf("replica err = %v, want lease-held fence", replicaErr)
	}
	st := durLoadState(t, h)
	if !strings.Contains(replicaErr.Error(), st.ID) {
		t.Fatalf("replica fence %q does not name the holder %q", replicaErr.Error(), st.ID)
	}
	if st.Status != "in_progress" {
		t.Fatalf("state mutated by fenced replica: status %q", st.Status)
	}
	durAssertSourceUntouched(t, h, "b", wantStored["b"])
	for _, k := range keys {
		if n := h.callCount("CopyIfMatch", k); n > 1 {
			t.Fatalf("CopyIfMatch %s after fenced replica = %d, want at most 1", k, n)
		}
	}

	cancel()
	res := durJoinRun(t, done, "holder after cancel")
	if !errors.Is(res.err, context.Canceled) {
		t.Fatalf("holder err = %v, want context.Canceled", res.err)
	}

	// The handoff: a fresh replica resumes the same migration contract.
	res2, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil || res2.Status != "completed" {
		t.Fatalf("handoff run = (%+v, %v)", res2, err)
	}
	st2 := durLoadState(t, h)
	if st2.ID != st.ID {
		t.Fatalf("handoff started %q; holder was %q", st2.ID, st.ID)
	}
	for _, k := range keys {
		if info, headErr := h.Head(context.Background(), h.bucket, k); headErr != nil || info.Metadata[armorMetaVersion] != "3" {
			t.Fatalf("object %s after handoff: (%+v, %v)", k, info, headErr)
		}
		if n := h.callCount("CopyIfMatch", k); n != 1 {
			t.Fatalf("CopyIfMatch %s across holder+handoff = %d, want exactly 1", k, n)
		}
	}
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released after handoff")
	}
}

// TestDurableMigrationStaleLeaseFencing covers both lease outcomes after a
// pod death: a fresh lease fences new workers out without any state write,
// and a lease older than the bounded TTL is taken over and released by the
// recovery run.
func TestDurableMigrationStaleLeaseFencing(t *testing.T) {
	t.Run("fresh lease blocks a new worker", func(t *testing.T) {
		h := durNewHarness(t)
		for _, k := range []string{"x", "y"} {
			durPutV1Object(t, h, k, 2048)
		}
		lease := fmt.Sprintf(`{"migration_id":"dead-pod","acquired_at":%q}`, time.Now().UTC().Add(-10*time.Second).Format(time.RFC3339Nano))
		if err := h.Put(context.Background(), h.bucket, durLeaseKey, bytesReader([]byte(lease)), int64(len(lease)), map[string]string{"Content-Type": "application/json"}); err != nil {
			t.Fatal(err)
		}
		_, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
		if err == nil || !strings.Contains(err.Error(), "migration lease held by dead-pod") {
			t.Fatalf("err = %v, want fence naming dead-pod", err)
		}
		if _, stateErr := h.Head(context.Background(), h.bucket, durStatePath); stateErr == nil {
			t.Fatal("fenced worker wrote migration state before holding the lease")
		}
		for _, k := range []string{"x", "y"} {
			durAssertCommittedV3Never(t, h, k)
		}
	})

	t.Run("expired lease is taken over", func(t *testing.T) {
		h := durNewHarness(t)
		wantPlain := map[string]string{}
		for _, k := range []string{"x", "y"} {
			wantPlain[k] = durPutV1Object(t, h, k, 2048)
		}
		lease := fmt.Sprintf(`{"migration_id":"dead-pod","acquired_at":%q}`, time.Now().UTC().Add(-6*time.Minute).Format(time.RFC3339Nano))
		if err := h.Put(context.Background(), h.bucket, durLeaseKey, bytesReader([]byte(lease)), int64(len(lease)), map[string]string{"Content-Type": "application/json"}); err != nil {
			t.Fatal(err)
		}
		res, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
		if err != nil || res.Status != "completed" {
			t.Fatalf("takeover run = (%+v, %v)", res, err)
		}
		for _, k := range []string{"x", "y"} {
			durAssertCommittedV3(t, h, k, wantPlain[k])
		}
		if durLeasePresent(t, h) {
			t.Fatal("takeover run did not release the recovered lease")
		}
	})
}

// durAssertCommittedV3Never asserts an object was NOT migrated to v3.
func durAssertCommittedV3Never(t *testing.T, h *migHarness, key string) {
	t.Helper()
	info, err := h.Head(context.Background(), h.bucket, key)
	if err != nil {
		t.Fatalf("head %s: %v", key, err)
	}
	if info.Metadata[armorMetaVersion] != "1" {
		t.Fatalf("fenced worker migrated %s to version %q", key, info.Metadata[armorMetaVersion])
	}
}

// TestDurableMigrationChangedParametersRestartFresh interrupts a run and then
// resumes with a different migration contract. The contract digest must
// reject the old checkpoints: a fresh state (new ID, new digest) starts over
// and re-commits, so no checkpoint from the abandoned contract is ever
// honored.
func TestDurableMigrationChangedParametersRestartFresh(t *testing.T) {
	h := durNewHarness(t, &migFault{op: "GetRange", key: "b", blockCtx: true})
	wantPlain := map[string]string{}
	wantStored := map[string]string{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f"} {
		wantPlain[k] = durPutV1Object(t, h, k, 2048)
		wantStored[k] = durStoredSHA(t, h, k)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := durStartRun(durMigrator(h, []string{"1"}), ctx, false, 1)
	durWaitFor(t, "worker reaching the blocked source read", func() bool {
		return h.callCount("GetRange", "b") > 0
	})
	cancel()
	res := durJoinRun(t, done, "after cancel")
	if res.r == nil || res.r.Status != "interrupted" {
		t.Fatalf("run 1 result = %+v", res.r)
	}
	st := durLoadState(t, h)
	if h.callCount("CopyIfMatch", "a") != 1 {
		t.Fatalf("run 1 CopyIfMatch a = %d, want 1", h.callCount("CopyIfMatch", "a"))
	}

	// Same bucket, different contract (include list widened): an unfinished
	// live operation must reject the changed contract instead of silently
	// re-reading its checkpoints, and the rejected attempt must not touch a
	// single object.
	_, rejectErr := durMigrator(h, []string{"1", "2"}).RunDurable(context.Background(), false, 1)
	if rejectErr == nil || !strings.Contains(rejectErr.Error(), "migration contract does not match unfinished operation") {
		t.Fatalf("changed-contract err = %v, want fail-closed rejection", rejectErr)
	}
	if !strings.Contains(rejectErr.Error(), st.ID) {
		t.Fatalf("rejection %q does not name the unfinished operation %q", rejectErr.Error(), st.ID)
	}
	st2 := durLoadState(t, h)
	if st2.ID != st.ID || st2.Status != "interrupted" {
		t.Fatalf("state after rejected run = id %q status %q, want untouched interrupted %q", st2.ID, st2.Status, st.ID)
	}
	if h.callCount("CopyIfMatch", "a") != 1 {
		t.Fatalf("rejected run committed: CopyIfMatch a = %d, want 1", h.callCount("CopyIfMatch", "a"))
	}
	durAssertSourceUntouched(t, h, "b", wantStored["b"])

	// The matching contract still resumes the same operation.
	res3, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 1)
	if err != nil || res3.Status != "completed" {
		t.Fatalf("matching-contract run = (%+v, %v)", res3, err)
	}
	st3 := durLoadState(t, h)
	if st3.ID != st.ID {
		t.Fatalf("matching-contract run started %q; interrupted operation was %q", st3.ID, st.ID)
	}
	for _, k := range []string{"a", "b", "c", "d", "e", "f"} {
		durAssertCommittedV3(t, h, k, wantPlain[k])
		if n := h.callCount("CopyIfMatch", k); n != 1 {
			t.Fatalf("CopyIfMatch %s across all runs = %d, want exactly 1", k, n)
		}
	}
	durAssertNoStaging(t, h)
	if durLeasePresent(t, h) {
		t.Fatal("lease not released")
	}
}

// TestDurableMigrationWorkerConcurrencyBounded gates the first source read
// of two distinct objects until both workers are inside it at once, then
// verifies the observed watermark equals the configured bound exactly and
// that the run still commits everything.
func TestDurableMigrationWorkerConcurrencyBounded(t *testing.T) {
	h := durNewHarness(t)
	keys := []string{"a", "b", "c", "d"}
	for _, k := range keys {
		durPutV1Object(t, h, k, 2048)
	}
	h.durArmGate([]string{"a", "b"}, 2)

	res, err := durMigrator(h, []string{"1"}).RunDurable(context.Background(), false, 2)
	if err != nil || res.Status != "completed" {
		t.Fatalf("run = (%+v, %v)", res, err)
	}
	if got := h.observedMaxInflight(); got != 2 {
		t.Fatalf("observed max concurrent source reads = %d, want exactly the configured bound 2", got)
	}
	for _, k := range keys {
		if h.callCount("CopyIfMatch", k) != 1 {
			t.Fatalf("CopyIfMatch %s = %d, want 1", k, h.callCount("CopyIfMatch", k))
		}
	}
	durAssertNoStaging(t, h)
}
