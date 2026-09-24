package provenance

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/jedarden/armor/internal/backend"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// failingEntryBackend fails injected chain writes on top of a mockBackend.
// failSeq > 0 fails the entry put for that sequence number; failHead fails
// every chain-head put. Both are atomic so a batch's parallel writes can read
// them while the test adjusts recovery.
type failingEntryBackend struct {
	*mockBackend
	failSeq  atomic.Int64
	failHead atomic.Bool
}

func newFailingEntryBackend() *failingEntryBackend {
	return &failingEntryBackend{mockBackend: newMockBackend()}
}

func (f *failingEntryBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if strings.HasPrefix(key, ChainHeadPrefix) {
		if f.failHead.Load() {
			return fmt.Errorf("injected chain head write failure")
		}
		return f.mockBackend.Put(ctx, bucket, key, body, size, meta)
	}
	if strings.HasPrefix(key, ChainPrefix) && strings.HasSuffix(key, ".json") {
		relative := key[len(ChainPrefix):]
		if idx := strings.LastIndex(relative, "/"); idx >= 0 {
			if seq, err := strconv.ParseInt(relative[idx+1:][:len(relative[idx+1:])-len(".json")], 10, 64); err == nil {
				if f.failSeq.Load() == seq {
					return fmt.Errorf("injected chain entry write failure at seq %d", seq)
				}
			}
		}
	}
	return f.mockBackend.Put(ctx, bucket, key, body, size, meta)
}

// countedBackend counts chain entry and head puts to prove batching actually
// happens: before group commit, N uploads produced N head puts.
type countedBackend struct {
	*mockBackend
	entryPuts atomic.Int64
	headPuts  atomic.Int64
}

func newCountedBackend() *countedBackend {
	return &countedBackend{mockBackend: newMockBackend()}
}

func (c *countedBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if strings.HasPrefix(key, ChainHeadPrefix) {
		c.headPuts.Add(1)
	} else if strings.HasPrefix(key, ChainPrefix) {
		c.entryPuts.Add(1)
	}
	return c.mockBackend.Put(ctx, bucket, key, body, size, meta)
}

// gatedBackend blocks the first chain-entry put that arrives after the gate
// is armed until the gate is closed, letting a test hold the appender
// mid-cycle while it queues more appends. entered is signaled (once per put)
// when a put enters the gate. Everything else is forwarded to the wrapped
// backend.
type gatedBackend struct {
	backend.Backend
	armed   atomic.Bool
	gate    chan struct{}
	entered chan struct{}
}

func (g *gatedBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	if strings.HasPrefix(key, ChainPrefix) && g.armed.Load() {
		select {
		case g.entered <- struct{}{}:
		default:
		}
		<-g.gate
	}
	return g.Backend.Put(ctx, bucket, key, body, size, meta)
}

func sha256Hex(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

// TestGroupCommitBatchesConcurrentUploads proves the throughput property the
// change exists for: appends queued while a cycle is in flight commit in one
// batch with a single head write, instead of one entry+head write pair each.
func TestGroupCommitBatchesConcurrentUploads(t *testing.T) {
	ctx := context.Background()
	cb := newCountedBackend()
	gb := &gatedBackend{Backend: cb, gate: make(chan struct{}), entered: make(chan struct{}, 64)}
	m := NewManager(gb, "test-bucket", "test-writer")

	const batched = 32
	total := batched + 1

	gb.armed.Store(true)
	errCh := make(chan error, total)
	go func() {
		errCh <- m.RecordUpload(ctx, "data/first.txt", sha256Hex("first"), "put")
	}()

	// Wait until the appender is mid-cycle, blocked inside the gated entry
	// put, then queue the rest behind it.
	select {
	case <-gb.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("first entry put never reached the gate")
	}
	gb.armed.Store(false)
	for i := 0; i < batched; i++ {
		go func(i int) {
			errCh <- m.RecordUpload(ctx, fmt.Sprintf("data/file-%d.txt", i), sha256Hex(fmt.Sprintf("file-%d", i)), "put")
		}(i)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if len(m.appendQueue) == batched {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if len(m.appendQueue) != batched {
		t.Fatalf("queue never filled: len = %d, want %d", len(m.appendQueue), batched)
	}
	close(gb.gate)

	for i := 0; i < total; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("RecordUpload %d failed: %v", i, err)
		}
	}

	// One singleton cycle plus one 32-append batch: 2 head writes for 33
	// uploads. The pre-group-commit path wrote 33.
	if got := cb.headPuts.Load(); got != 2 {
		t.Errorf("head puts = %d, want 2 (one per batch, not one per upload)", got)
	}
	if got := cb.entryPuts.Load(); got != int64(total) {
		t.Errorf("entry puts = %d, want %d", got, total)
	}

	result, err := NewAuditor(cb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "valid" || len(result.Writers) != 1 {
		t.Fatalf("batched chain is invalid: %+v", result)
	}
	if result.Writers[0].HeadSequence != int64(total) || result.Writers[0].EntriesVerified != total {
		t.Fatalf("batched chain lost entries: %+v", result.Writers[0])
	}
}

// TestGroupCommitMidBatchFailureFailsWholeBatch drives commitBatch directly so
// the batch shape is deterministic: an entry write failing partway through the
// batch must fail every waiter in that batch and leave the chain head — both
// the durable object and the in-memory sequence allocator — at the pre-batch
// position. A later successful batch then reuses the sequence numbers,
// overwrites the orphans, and the chain audits valid and gap-free.
func TestGroupCommitMidBatchFailureFailsWholeBatch(t *testing.T) {
	ctx := context.Background()
	fb := newFailingEntryBackend()
	m := NewManager(fb, "test-bucket", "test-writer")

	// Warm the head cache at sequence 1 so the failure path below cannot be
	// confused with a cold-start head load.
	if err := m.RecordUpload(ctx, "data/warm.txt", sha256Hex("warm"), "put"); err != nil {
		t.Fatalf("warm-up RecordUpload failed: %v", err)
	}

	const batchLen = 5
	batch := make([]*pendingAppend, batchLen)
	for i := range batch {
		batch[i] = &pendingAppend{
			kind:            appendUpload,
			objectKey:       fmt.Sprintf("data/batch-%d.txt", i),
			plaintextSHA256: sha256Hex(fmt.Sprintf("batch-%d", i)),
			operation:       "put",
			writeCtx:        context.Background(),
			done:            make(chan error, 1),
			enqueuedAt:      time.Now(),
		}
	}

	// Sequence numbers 2..6; the middle write (seq 4 of 6... i.e. the third
	// item, seq 4) fails, so writes both before and after it succeed.
	fb.failSeq.Store(4)
	m.commitBatch(batch)

	for i, item := range batch {
		select {
		case err := <-item.done:
			if err == nil {
				t.Errorf("waiter %d succeeded; every waiter in a failed batch must get the error", i)
			}
		default:
			t.Errorf("waiter %d was not released", i)
		}
	}

	// Durable head unchanged.
	head, err := m.loadHead(ctx)
	if err != nil {
		t.Fatalf("loadHead failed: %v", err)
	}
	if head.Sequence != 1 {
		t.Errorf("durable head sequence = %d, want 1 (head must not cover a failed batch)", head.Sequence)
	}

	// In-memory allocator unchanged, so the sequences are reused next batch.
	m.mu.RLock()
	cached := m.head
	m.mu.RUnlock()
	if cached == nil || cached.Sequence != 1 {
		t.Errorf("cached head sequence = %+v, want sequence 1", cached)
	}

	// Recover: same append set succeeds once the injected failure is lifted,
	// reusing sequences 2..6 and overwriting any orphaned entries.
	fb.failSeq.Store(0)
	m.commitBatch(batch)
	for i, item := range batch {
		if err := <-item.done; err != nil {
			t.Errorf("recovery waiter %d failed: %v", i, err)
		}
	}

	result, err := NewAuditor(fb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "valid" {
		t.Fatalf("chain after recovery is not valid: %+v", result)
	}
	if result.Writers[0].HeadSequence != batchLen+1 || result.Writers[0].EntriesVerified != batchLen+1 {
		t.Fatalf("chain after recovery lost entries: %+v", result.Writers[0])
	}
	if len(result.Gaps) != 0 {
		t.Fatalf("chain after recovery has gaps: %+v", result.Gaps)
	}
}

// TestGroupCommitHeadWriteFailureFailsWholeBatch covers the other half of the
// batch failure surface: entries durable but the covering head write failing
// must still fail every waiter and leave the durable head at the pre-batch
// position.
func TestGroupCommitHeadWriteFailureFailsWholeBatch(t *testing.T) {
	ctx := context.Background()
	fb := newFailingEntryBackend()
	m := NewManager(fb, "test-bucket", "test-writer")

	if err := m.RecordUpload(ctx, "data/warm.txt", sha256Hex("warm"), "put"); err != nil {
		t.Fatalf("warm-up RecordUpload failed: %v", err)
	}

	batch := make([]*pendingAppend, 3)
	for i := range batch {
		batch[i] = &pendingAppend{
			kind:            appendUpload,
			objectKey:       fmt.Sprintf("data/batch-%d.txt", i),
			plaintextSHA256: sha256Hex(fmt.Sprintf("batch-%d", i)),
			operation:       "put",
			writeCtx:        context.Background(),
			done:            make(chan error, 1),
			enqueuedAt:      time.Now(),
		}
	}

	fb.failHead.Store(true)
	m.commitBatch(batch)
	for i, item := range batch {
		if err := <-item.done; err == nil {
			t.Errorf("waiter %d succeeded despite head write failure", i)
		}
	}

	head, err := m.loadHead(ctx)
	if err != nil {
		t.Fatalf("loadHead failed: %v", err)
	}
	if head.Sequence != 1 {
		t.Errorf("durable head sequence = %d, want 1", head.Sequence)
	}

	fb.failHead.Store(false)
	m.commitBatch(batch)
	for i, item := range batch {
		if err := <-item.done; err != nil {
			t.Errorf("recovery waiter %d failed: %v", i, err)
		}
	}

	result, err := NewAuditor(fb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "valid" || result.Writers[0].HeadSequence != 4 {
		t.Fatalf("chain after recovery is not valid at sequence 4: %+v", result)
	}
}

// TestGroupCommitConcurrentUploadsAndKeyEvents verifies through the real
// appender loop that mixed upload and key-event appends interleave into one
// linear, gap-free, verifiable chain.
func TestGroupCommitConcurrentUploadsAndKeyEvents(t *testing.T) {
	ctx := context.Background()
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	const uploads = 64
	var wg sync.WaitGroup
	errCh := make(chan error, uploads+2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.RecordKeyEvent(ctx, "key-rotate-start", KeyEventOpts{
			OldMEKHash: "aaaaaaaaaaaaaaaa",
			NewMEKHash: "bbbbbbbbbbbbbbbb",
			RotationID: "rotation-1",
		}); err != nil {
			errCh <- err
		}
	}()

	for i := 0; i < uploads; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("data/file-%d.txt", i)
			if err := m.RecordUpload(ctx, key, sha256Hex(key), "put"); err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := m.RecordKeyEvent(ctx, "key-rotate-complete", KeyEventOpts{
			OldMEKHash: "aaaaaaaaaaaaaaaa",
			NewMEKHash: "bbbbbbbbbbbbbbbb",
			RotationID: "rotation-1",
			RotationResult: &KeyRotationResult{
				TotalObjects:     uploads,
				ProcessedObjects: uploads,
				Status:           "complete",
			},
		}); err != nil {
			errCh <- err
		}
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent append failed: %v", err)
	}

	result, err := NewAuditor(mb, "test-bucket").Audit(ctx)
	if err != nil {
		t.Fatalf("Audit failed: %v", err)
	}
	if result.Status != "valid" || len(result.Writers) != 1 {
		t.Fatalf("mixed chain is invalid: %+v", result)
	}
	writer := result.Writers[0]
	if writer.HeadSequence != uploads+2 || writer.EntriesVerified != uploads {
		t.Fatalf("mixed chain lost entries: %+v", writer)
	}
	if writer.KeyEvents != 2 {
		t.Fatalf("mixed chain key events = %d, want 2: %+v", writer.KeyEvents, writer)
	}
	if len(result.Gaps) != 0 {
		t.Fatalf("mixed chain has gaps: %+v", result.Gaps)
	}
}

// TestGroupCommitTimingsSplitUnderBatch verifies the armor-69dd394b timing
// split still reaches every waiter of a batch: lock wait covers the queue
// time, write covers the batch cycle, for every item in the batch.
func TestGroupCommitTimingsSplitUnderBatch(t *testing.T) {
	mb := newMockBackend()
	m := NewManager(mb, "test-bucket", "test-writer")

	const batchLen = 8
	timings := make([]*Timings, batchLen)
	batch := make([]*pendingAppend, batchLen)
	for i := 0; i < batchLen; i++ {
		timings[i] = &Timings{}
		batch[i] = &pendingAppend{
			kind:            appendUpload,
			objectKey:       fmt.Sprintf("data/timed-%d.txt", i),
			plaintextSHA256: sha256Hex(fmt.Sprintf("timed-%d", i)),
			operation:       "put",
			writeCtx:        context.Background(),
			done:            make(chan error, 1),
			enqueuedAt:      time.Now().Add(-time.Millisecond),
			timed:           timings[i],
		}
	}

	m.commitBatch(batch)

	for i, item := range batch {
		if err := <-item.done; err != nil {
			t.Fatalf("batch item %d failed: %v", i, err)
		}
		if timings[i].LockWait <= 0 {
			t.Errorf("waiter %d LockWait = %v, want > 0", i, timings[i].LockWait)
		}
		if timings[i].Write <= 0 {
			t.Errorf("waiter %d Write = %v, want > 0", i, timings[i].Write)
		}
	}
}
