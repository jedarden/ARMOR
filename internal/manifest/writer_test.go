package manifest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// captureUploader records delta uploads for assertions.
type captureUploader struct {
	mu      sync.Mutex
	uploads map[string][]byte
}

func newCaptureUploader() *captureUploader {
	return &captureUploader{uploads: make(map[string][]byte)}
}

func (u *captureUploader) Upload(_ context.Context, key string, data []byte) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	cp := make([]byte, len(data))
	copy(cp, data)
	u.uploads[key] = cp
	return nil
}

func (u *captureUploader) Count() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.uploads)
}

func (u *captureUploader) Keys() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	out := make([]string, 0, len(u.uploads))
	for k := range u.uploads {
		out = append(out, k)
	}
	return out
}

func (u *captureUploader) Content(key string) []byte {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.uploads[key]
}

func (u *captureUploader) TotalOps() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	total := 0
	for _, data := range u.uploads {
		dec := json.NewDecoder(bytes.NewReader(data))
		for dec.More() {
			var op Op
			if err := dec.Decode(&op); err != nil {
				break
			}
			total++
		}
	}
	return total
}

func sampleEntry(n int) *Entry {
	return &Entry{
		PlaintextSize:   int64(n) * 1024,
		PlaintextSHA256: fmt.Sprintf("sha256-%d", n),
		IV:              []byte{byte(n), 0, 0},
		WrappedDEK:      []byte{byte(n), 1, 2},
		BlockSize:       65536,
		ContentType:     "application/octet-stream",
		ETag:            fmt.Sprintf("etag-%d", n),
		LastModified:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func TestWriter_SinglePutFlushedOnStop(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 64)
	w.Start(context.Background())

	w.EnqueuePut("bucket", "path/file.txt", sampleEntry(1))
	w.Stop()

	if up.Count() != 1 {
		t.Fatalf("expected 1 delta upload, got %d", up.Count())
	}

	keys := up.Keys()
	if !strings.Contains(keys[0], "delta-0000000001.jsonl") {
		t.Errorf("expected zero-padded delta key, got %q", keys[0])
	}
	if !strings.HasPrefix(keys[0], ".armor/manifest/writer-1/") {
		t.Errorf("unexpected prefix in key %q", keys[0])
	}
}

func TestWriter_DeleteOp(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 64)
	w.Start(context.Background())

	w.EnqueueDelete("bucket", "path/file.txt")
	w.Stop()

	if up.Count() != 1 {
		t.Fatalf("expected 1 delta upload, got %d", up.Count())
	}
	data := up.Content(up.Keys()[0])
	var op Op
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&op); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	if op.Operation != "del" {
		t.Errorf("expected del op, got %q", op.Operation)
	}
	if op.Key != "bucket/path/file.txt" {
		t.Errorf("expected key %q, got %q", "bucket/path/file.txt", op.Key)
	}
	if op.Entry != nil {
		t.Error("del op should have nil entry")
	}
}

func TestWriter_PutOpContents(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 64)
	w.Start(context.Background())

	entry := sampleEntry(42)
	w.EnqueuePut("mybucket", "dir/obj.parquet", entry)
	w.Stop()

	data := up.Content(up.Keys()[0])
	var op Op
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&op); err != nil {
		t.Fatalf("decode delta: %v", err)
	}
	if op.Operation != "put" {
		t.Errorf("expected put, got %q", op.Operation)
	}
	if op.Key != "mybucket/dir/obj.parquet" {
		t.Errorf("unexpected key %q", op.Key)
	}
	if op.Entry == nil {
		t.Fatal("put op must have entry")
	}
	if op.Entry.PlaintextSize != entry.PlaintextSize {
		t.Errorf("size mismatch: got %d", op.Entry.PlaintextSize)
	}
	if op.Entry.ETag != entry.ETag {
		t.Errorf("etag mismatch: got %q", op.Entry.ETag)
	}
}

func TestWriter_MultipleOpsBatched(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 128)
	w.Start(context.Background())

	const n = 20
	for i := 0; i < n; i++ {
		w.EnqueuePut("b", fmt.Sprintf("k%d", i), sampleEntry(i))
	}
	w.Stop()

	if up.Count() == 0 {
		t.Fatal("expected at least one delta")
	}
	if up.TotalOps() != n {
		t.Errorf("expected %d total ops across all deltas, got %d", n, up.TotalOps())
	}
}

func TestWriter_SequenceIncrementsPerFlush(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	// bufSize=1 forces the goroutine to flush after each op (no batching)
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 1)
	w.Start(context.Background())

	for i := 0; i < 3; i++ {
		w.EnqueuePut("b", fmt.Sprintf("k%d", i), sampleEntry(i))
		// Small sleep so the goroutine can drain between enqueues.
		time.Sleep(20 * time.Millisecond)
	}
	w.Stop()

	if idx.Seq() < 1 {
		t.Errorf("expected seq >= 1, got %d", idx.Seq())
	}
	if up.Count() == 0 {
		t.Error("expected at least one delta")
	}
}

func TestWriter_PaddedFilename(t *testing.T) {
	idx := New()
	idx.SetSeq(999999999) // seq will become 1000000000 on next IncSeq
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 64)
	w.Start(context.Background())

	w.EnqueuePut("b", "k", sampleEntry(1))
	w.Stop()

	keys := up.Keys()
	if len(keys) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(keys))
	}
	want := ".armor/manifest/writer-1/delta-1000000000.jsonl"
	if keys[0] != want {
		t.Errorf("expected %q, got %q", want, keys[0])
	}
}

func TestWriter_NoUploadsOnEmptyStop(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 64)
	w.Start(context.Background())
	w.Stop()
	if up.Count() != 0 {
		t.Errorf("expected 0 uploads on empty stop, got %d", up.Count())
	}
}

func TestWriter_StopIsIdempotent(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 64)
	w.Start(context.Background())
	w.Stop()
	w.Stop() // must not panic or deadlock
}

func TestWriter_ContextCancellationStops(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	ctx, cancel := context.WithCancel(context.Background())
	w := NewWriter(idx, ".armor/manifest", "writer-1", up.Upload, 64)
	w.Start(ctx)

	w.EnqueuePut("b", "k", sampleEntry(1))
	cancel() // trigger stop via context
	// Wait for done by calling Stop (which will just wait on <-done)
	w.Stop()

	// At least the op should have been flushed
	if up.TotalOps() != 1 {
		t.Errorf("expected 1 op flushed, got %d", up.TotalOps())
	}
}

func TestWriter_WriterIDInKey(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, ".armor/manifest", "my-cluster-node-3", up.Upload, 64)
	w.Start(context.Background())

	w.EnqueuePut("b", "k", sampleEntry(1))
	w.Stop()

	for _, k := range up.Keys() {
		if !strings.Contains(k, "my-cluster-node-3") {
			t.Errorf("expected writer ID in key, got %q", k)
		}
	}
}

func TestWriter_CustomPrefix(t *testing.T) {
	idx := New()
	up := newCaptureUploader()
	w := NewWriter(idx, "custom/prefix", "w1", up.Upload, 64)
	w.Start(context.Background())

	w.EnqueuePut("b", "k", sampleEntry(1))
	w.Stop()

	for _, k := range up.Keys() {
		if !strings.HasPrefix(k, "custom/prefix/") {
			t.Errorf("expected custom prefix in key, got %q", k)
		}
	}
}
