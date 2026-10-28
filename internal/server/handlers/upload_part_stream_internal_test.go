package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"os"
	"testing"
	"time"
)

// zeroReader yields n zero bytes without a backing allocation, so a
// over-pooled-length routing probe costs a spool write but no 64 MiB slice.
type zeroReader struct{ remaining int64 }

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.remaining <= 0 {
		return 0, io.EOF
	}
	n := int64(len(p))
	if n > z.remaining {
		n = z.remaining
	}
	for i := int64(0); i < n; i++ {
		p[i] = 0
	}
	z.remaining -= n
	return int(n), nil
}

func assertSpoolDirEmpty(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("part spool left %d files in %s", len(entries), dir)
	}
}

// TestReadUploadPartBodySeekableRoutesPoolAndSpool pins the routing decision
// and the source contract: known lengths within the pooled part size come
// back as a pooled *bytes.Reader admitted against the budget; unknown and
// over-pooled lengths spill to a temp file that release removes. A body
// shorter than its declared Content-Length fails and leaves nothing behind.
func TestReadUploadPartBodySeekableRoutesPoolAndSpool(t *testing.T) {
	ctx := context.Background()
	spoolDir := t.TempDir()
	t.Setenv("TMPDIR", spoolDir)
	budget := newPartMemoryBudget(1 << 20)

	data := bytes.Repeat([]byte{0xA5}, 1000)
	wantDigest := sha256.Sum256(data)

	// Known length within the pooled size: pooled buffer source.
	src, err := readUploadPartBodySeekable(ctx, bytes.NewReader(data), int64(len(data)), budget)
	if err != nil {
		t.Fatal(err)
	}
	if _, pooled := src.reader.(*bytes.Reader); !pooled {
		t.Fatalf("known in-pool length routed to %T, want *bytes.Reader", src.reader)
	}
	if src.size != int64(len(data)) || src.digest != wantDigest {
		t.Fatalf("pooled source size/digest mismatch: size=%d digest=%x", src.size, src.digest)
	}
	got, err := io.ReadAll(src.reader)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("pooled source readback: err=%v equal=%v", err, bytes.Equal(got, data))
	}
	src.release()
	// The budget must be fully restored by release.
	restore, err := budget.acquire(ctx, 1<<20)
	if err != nil {
		t.Fatalf("budget not restored after release: %v", err)
	}
	restore()
	assertSpoolDirEmpty(t, spoolDir)

	// Unknown length: temp-file spool.
	src, err = readUploadPartBodySeekable(ctx, bytes.NewReader(data), -1, budget)
	if err != nil {
		t.Fatal(err)
	}
	if _, spilled := src.reader.(*os.File); !spilled {
		t.Fatalf("unknown length routed to %T, want *os.File", src.reader)
	}
	if src.size != int64(len(data)) || src.digest != wantDigest {
		t.Fatalf("spilled source size/digest mismatch: size=%d digest=%x", src.size, src.digest)
	}
	got, err = io.ReadAll(src.reader)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("spilled source readback: err=%v equal=%v", err, bytes.Equal(got, data))
	}
	// The spool exists until release removes it.
	if entries, _ := os.ReadDir(spoolDir); len(entries) != 1 {
		t.Fatalf("spool dir holds %d files before release, want 1", len(entries))
	}
	src.release()
	assertSpoolDirEmpty(t, spoolDir)

	// Over-pooled known length: spool, without a 64 MiB backing allocation.
	src, err = readUploadPartBodySeekable(ctx, &zeroReader{remaining: pooledUploadPartSize + 1}, pooledUploadPartSize+1, budget)
	if err != nil {
		t.Fatal(err)
	}
	if _, spilled := src.reader.(*os.File); !spilled {
		t.Fatalf("over-pooled length routed to %T, want *os.File", src.reader)
	}
	if src.size != pooledUploadPartSize+1 {
		t.Fatalf("over-pooled size=%d, want %d", src.size, pooledUploadPartSize+1)
	}
	src.release()
	assertSpoolDirEmpty(t, spoolDir)

	// Body shorter than Content-Length: error, no leftover spool.
	if _, err := readUploadPartBodySeekable(ctx, bytes.NewReader(data[:5]), int64(len(data)), budget); err == nil {
		t.Fatal("short body accepted")
	}
	assertSpoolDirEmpty(t, spoolDir)
}

// TestPartMemoryBudgetAdmitsOversizedAloneAndCancels covers the budget's
// edge semantics: a part larger than the whole budget is admitted alone
// (never deadlocking), a waiter canceled while blocked returns an error
// without taking bytes, and a release wakes every waiter — including ones
// smaller than the released amount.
func TestPartMemoryBudgetAdmitsOversizedAloneAndCancels(t *testing.T) {
	ctx := context.Background()
	b := newPartMemoryBudget(100)

	oversized, err := b.acquire(ctx, 150)
	if err != nil {
		t.Fatalf("oversized acquire: %v", err)
	}

	waitCtx, cancel := context.WithCancel(ctx)
	acquired := make(chan error, 1)
	go func() {
		_, aerr := b.acquire(waitCtx, 10)
		acquired <- aerr
	}()
	select {
	case err := <-acquired:
		t.Fatalf("acquire completed while budget fully held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-acquired:
		if err == nil {
			t.Fatal("canceled acquire reported success")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("canceled acquire never returned")
	}

	oversized()
	restored, err := b.acquire(ctx, 100)
	if err != nil {
		t.Fatalf("budget not restored after oversized release: %v", err)
	}
	restored()

	// Wake-all: two waiters that cannot fit while 6 of 10 bytes are held
	// both proceed after the release.
	b2 := newPartMemoryBudget(10)
	held, _ := b2.acquire(ctx, 6)
	small := make(chan error, 1)
	large := make(chan error, 1)
	go func() {
		r, aerr := b2.acquire(ctx, 5)
		if aerr == nil {
			r()
		}
		small <- aerr
	}()
	go func() {
		r, aerr := b2.acquire(ctx, 5)
		if aerr == nil {
			r()
		}
		large <- aerr
	}()
	select {
	case err := <-small:
		t.Fatalf("small acquire completed while budget held: %v", err)
	case err := <-large:
		t.Fatalf("large acquire completed while budget held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	held()
	for _, ch := range []chan error{small, large} {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("waiter not woken by release: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("waiter never woken by release")
		}
	}
}
