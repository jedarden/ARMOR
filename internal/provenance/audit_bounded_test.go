package provenance

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"bytes"
)

// recordUploads records count uploads for one writer and marks the stored
// objects ARMOR-encrypted so the untracked cross-reference can see them.
func recordUploads(t *testing.T, mb *mockBackend, writerID string, count int) *Manager {
	t.Helper()
	ctx := context.Background()
	m := NewManager(mb, "test-bucket", writerID)
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("content "+writerID)))
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("data/%s/file%d.txt", writerID, i)
		if err := m.RecordUpload(ctx, key, plaintextSHA, "put"); err != nil {
			t.Fatalf("%s RecordUpload failed: %v", writerID, err)
		}
		mb.metadata["test-bucket/"+key] = map[string]string{"x-amz-meta-armor-version": "1"}
	}
	return m
}

// TestAuditWithOptionsWriterScoped checks that a writer-scoped audit walks
// and reports only the requested chain and skips the untracked
// cross-reference by default (a single chain cannot vouch for the bucket).
func TestAuditWithOptionsWriterScoped(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	recordUploads(t, mb, "writer-1", 3)
	recordUploads(t, mb, "writer-2", 2)

	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.AuditWithOptions(ctx, AuditOptions{Writer: "writer-1"})
	if err != nil {
		t.Fatalf("AuditWithOptions failed: %v", err)
	}

	if result.Status != "valid" {
		t.Errorf("audit status = %s, want valid (errors: %v)", result.Status, result.Errors)
	}
	if len(result.Writers) != 1 || result.Writers[0].WriterID != "writer-1" {
		t.Fatalf("expected only writer-1, got %+v", result.Writers)
	}
	if result.Writers[0].EntriesVerified != 3 || !result.Writers[0].Valid {
		t.Errorf("writer-1 audit = %+v, want 3 verified and valid", result.Writers[0])
	}
	if result.TotalEntries != 3 {
		t.Errorf("total entries = %d, want 3", result.TotalEntries)
	}
	if result.TotalObjects != 0 {
		t.Errorf("total objects = %d, want 0 (cross-reference skipped)", result.TotalObjects)
	}
}

// TestAuditWithOptionsUnknownWriter checks that scoping to a writer with no
// chain head reports an incomplete audit instead of an empty valid one.
func TestAuditWithOptionsUnknownWriter(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	recordUploads(t, mb, "writer-1", 3)

	auditor := NewAuditor(mb, "test-bucket")
	result, err := auditor.AuditWithOptions(ctx, AuditOptions{Writer: "no-such-writer"})
	if err != nil {
		t.Fatalf("AuditWithOptions failed: %v", err)
	}

	if result.Status != "incomplete" {
		t.Errorf("audit status = %s, want incomplete", result.Status)
	}
	if len(result.Writers) != 1 || result.Writers[0].WriterID != "no-such-writer" || result.Writers[0].Valid {
		t.Fatalf("expected an invalid no-such-writer result, got %+v", result.Writers)
	}
	if result.Writers[0].Error == "" {
		t.Error("expected an error explaining the missing chain head")
	}
}

// TestAuditWithOptionsUntrackedFlag checks that IncludeUntracked overrides
// the default in both directions: explicitly off for an otherwise unbounded
// audit, and explicitly on for a writer-scoped one.
func TestAuditWithOptionsUntrackedFlag(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	recordUploads(t, mb, "writer-1", 3)

	// An ARMOR-encrypted object no chain tracks.
	mb.objects["test-bucket/data/untracked.txt"] = []byte("content")
	mb.metadata["test-bucket/data/untracked.txt"] = map[string]string{"x-amz-meta-armor-version": "1"}

	auditor := NewAuditor(mb, "test-bucket")

	skip := false
	result, err := auditor.AuditWithOptions(ctx, AuditOptions{IncludeUntracked: &skip})
	if err != nil {
		t.Fatalf("AuditWithOptions failed: %v", err)
	}
	if result.Status != "valid" {
		t.Errorf("status = %s, want valid with the cross-reference skipped (errors: %v)", result.Status, result.Errors)
	}
	if result.TotalObjects != 0 || len(result.UntrackedObjects) != 0 {
		t.Errorf("cross-reference ran despite untracked=false: %+v", result)
	}

	include := true
	scoped, err := auditor.AuditWithOptions(ctx, AuditOptions{Writer: "writer-1", IncludeUntracked: &include})
	if err != nil {
		t.Fatalf("AuditWithOptions failed: %v", err)
	}
	if scoped.Status != "invalid" {
		t.Errorf("status = %s, want invalid with the cross-reference forced on", scoped.Status)
	}
	if len(scoped.UntrackedObjects) != 1 || scoped.UntrackedObjects[0] != "data/untracked.txt" {
		t.Errorf("untracked objects = %v, want [data/untracked.txt]", scoped.UntrackedObjects)
	}
}

// TestAuditWithOptionsMaxEntries checks that an entry budget smaller than
// the chain truncates the walk with an incomplete status and the progress
// verified so far, and that a budget larger than the chain changes nothing
// except skipping the untracked cross-reference.
func TestAuditWithOptionsMaxEntries(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	recordUploads(t, mb, "writer-1", 5)

	auditor := NewAuditor(mb, "test-bucket")

	truncated, err := auditor.AuditWithOptions(ctx, AuditOptions{MaxEntries: 3})
	if err != nil {
		t.Fatalf("AuditWithOptions failed: %v", err)
	}
	if truncated.Status != "incomplete" {
		t.Errorf("status = %s, want incomplete after budget exhaustion", truncated.Status)
	}
	if len(truncated.Writers) != 1 {
		t.Fatalf("expected 1 writer, got %+v", truncated.Writers)
	}
	w := truncated.Writers[0]
	if w.Valid {
		t.Error("truncated writer reported valid; the chain was not fully verified")
	}
	if w.EntriesVerified != 3 || truncated.TotalEntries != 3 {
		t.Errorf("entries verified = %d (total %d), want 3 — the budget bounds the progress", w.EntriesVerified, truncated.TotalEntries)
	}
	if !strings.Contains(w.Error, "budget") {
		t.Errorf("writer error = %q, want it to name the exhausted entry budget", w.Error)
	}
	if truncated.TotalObjects != 0 {
		t.Errorf("total objects = %d, want 0 (cross-reference skipped for a bounded walk)", truncated.TotalObjects)
	}

	generous, err := auditor.AuditWithOptions(ctx, AuditOptions{MaxEntries: 10})
	if err != nil {
		t.Fatalf("AuditWithOptions failed: %v", err)
	}
	if generous.Status != "valid" {
		t.Errorf("status = %s, want valid when the budget covers the chain (errors: %v)", generous.Status, generous.Errors)
	}
	if generous.TotalEntries != 5 {
		t.Errorf("total entries = %d, want 5", generous.TotalEntries)
	}
}

// TestWalkDeltaEntriesLimitedStopsAtWindow checks that a budgeted delta walk
// reads files newest-first and stops once it has covered the verification
// window, instead of reading every delta file.
func TestWalkDeltaEntriesLimitedStopsAtWindow(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	auditor := NewAuditor(mb, "test-bucket")
	writerID := "test-writer"

	// Three delta files carrying chain sequences 1-2, 3-4, and 5-6.
	for fileNum := 0; fileNum < 3; fileNum++ {
		var lines []string
		for i := 0; i < 2; i++ {
			seq := int64(fileNum*2 + i + 1)
			op := struct {
				Operation string          `json:"op"`
				Key       string          `json:"key"`
				Chain     *ChainEntryData `json:"chain,omitempty"`
			}{
				Operation: "put",
				Key:       fmt.Sprintf("bucket/file%d.txt", seq),
				Chain: &ChainEntryData{
					Sequence:      seq,
					ChainHash:     strings.Repeat(fmt.Sprintf("%d", seq), sha256.Size*2/len(fmt.Sprintf("%d", seq))),
					PrevChainHash: InitialChainHash,
				},
			}
			line, err := json.Marshal(op)
			if err != nil {
				t.Fatalf("marshal delta op: %v", err)
			}
			lines = append(lines, string(line))
		}
		data := []byte(strings.Join(lines, "\n") + "\n")
		deltaKey := fmt.Sprintf(".armor/manifest/%s/delta-%010d.jsonl", writerID, fileNum+1)
		if err := mb.Put(ctx, "test-bucket", deltaKey, bytes.NewReader(data), int64(len(data)), map[string]string{"Content-Type": "application/jsonl"}); err != nil {
			t.Fatalf("put delta file: %v", err)
		}
	}

	// A two-entry window over a head at sequence 6 needs only the newest file.
	entries, _, err := auditor.walkDeltaEntriesLimited(ctx, writerID, 1, 6, 2)
	if err != nil {
		t.Fatalf("walkDeltaEntriesLimited failed: %v", err)
	}
	if len(entries) != 2 || entries[5] == nil || entries[6] == nil {
		t.Fatalf("expected only sequences 5 and 6, got %+v", entries)
	}

	// An unbounded walk still reads every file.
	all, _, err := auditor.walkDeltaEntriesLimited(ctx, writerID, 1, 6, -1)
	if err != nil {
		t.Fatalf("walkDeltaEntriesLimited failed: %v", err)
	}
	if len(all) != 6 {
		t.Fatalf("expected all 6 entries from an unbounded walk, got %d", len(all))
	}

	// A spent budget reads nothing.
	none, _, err := auditor.walkDeltaEntriesLimited(ctx, writerID, 1, 6, 0)
	if err != nil {
		t.Fatalf("walkDeltaEntriesLimited failed: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no entries from a spent budget, got %d", len(none))
	}
}
