package provenance

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
)

// TestRecordUploadTimings verifies that RecordUpload fills in the lock-wait
// and write durations when the caller attaches a Timings to the context
// (armor-69dd394b), and leaves a skipped key untouched.
func TestRecordUploadTimings(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("timings content")))

	timings := &Timings{}
	if err := m.RecordUpload(WithTimings(ctx, timings), "data/file1.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload failed: %v", err)
	}

	if timings.LockWait <= 0 {
		t.Errorf("LockWait = %v, want > 0 (lock acquisition always takes some time)", timings.LockWait)
	}
	if timings.Write <= 0 {
		t.Errorf("Write = %v, want > 0 (entry+head writes take some time)", timings.Write)
	}

	// A skipped key must not touch the timings at all
	skipped := &Timings{}
	if err := m.RecordUpload(WithTimings(ctx, skipped), ".armor/internal", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload of internal object failed: %v", err)
	}
	if skipped.LockWait != 0 || skipped.Write != 0 {
		t.Errorf("skipped key must leave timings zero, got %+v", skipped)
	}
}

// TestCreateChainEntryTimings verifies the same split on the manifest path,
// where the locked section covers the head read and in-memory head update.
func TestCreateChainEntryTimings(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("timings manifest content")))

	timings := &Timings{}
	entry, err := m.CreateChainEntry(WithTimings(ctx, timings), "data/file1.txt", plaintextSHA, "put")
	if err != nil {
		t.Fatalf("CreateChainEntry failed: %v", err)
	}
	if entry == nil {
		t.Fatal("CreateChainEntry returned nil entry for a recordable key")
	}

	if timings.LockWait <= 0 {
		t.Errorf("LockWait = %v, want > 0", timings.LockWait)
	}
	if timings.Write <= 0 {
		t.Errorf("Write = %v, want > 0", timings.Write)
	}
}

// TestTimingsOptional verifies that a nil collector in the context (the
// common case — no instrumentation attached) is a no-op.
func TestTimingsOptional(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("no timings")))

	if err := m.RecordUpload(ctx, "data/file1.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload without timings failed: %v", err)
	}
	if _, err := m.CreateChainEntry(ctx, "data/file2.txt", plaintextSHA, "put"); err != nil {
		t.Fatalf("CreateChainEntry without timings failed: %v", err)
	}
}
