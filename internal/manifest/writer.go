package manifest

import (
	"context"
	"sync"
	"time"
)

// Uploader uploads a manifest delta file directly to B2 (free ingress, no Cloudflare).
// key is the full B2 object key; data is the JSONL content.
type Uploader func(ctx context.Context, key string, data []byte) error

// DefaultOpsBufferSize is the default buffered channel capacity for Writer.
const DefaultOpsBufferSize = 4096

// Writer batches manifest operations and asynchronously persists them as
// delta files in B2. Each flush produces one delta-NNNNNNNNNN.jsonl object
// whose zero-padded filename enables lexicographic discovery on startup.
// There is no head pointer file — discovery uses ListObjects at startup.
//
// The writer is a performance aid: losing a delta (crash before flush) is
// acceptable because B2 object headers remain the authoritative source of
// truth, and startup replay catches up from whatever deltas reached B2.
type Writer struct {
	idx      *Index
	prefix   string
	writerID string
	upload   Uploader
	opsCh    chan Op
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
	// onFlush is called after each successful B2 delta upload, e.g. to notify
	// a Compactor that a new delta file exists. May be nil.
	onFlush func()
	// lastFlush records the timestamp of the most recent successful delta upload.
	// It is updated only when upload() succeeds. A zero time indicates no
	// successful flush has occurred since startup.
	lastFlush time.Time
}

// NewWriter creates a Writer backed by idx. bufSize controls the ops channel
// capacity; pass 0 to use DefaultOpsBufferSize. Call Start to launch the
// background flush goroutine.
func NewWriter(idx *Index, prefix, writerID string, upload Uploader, bufSize int) *Writer {
	if bufSize <= 0 {
		bufSize = DefaultOpsBufferSize
	}
	return &Writer{
		idx:      idx,
		prefix:   prefix,
		writerID: writerID,
		upload:   upload,
		opsCh:    make(chan Op, bufSize),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the background flush goroutine. Call it once after NewWriter.
// ctx cancellation stops the goroutine (same effect as Stop).
func (w *Writer) Start(ctx context.Context) {
	go w.run(ctx)
}

// Stop signals the goroutine to stop, flushes any remaining ops to B2, and
// waits for the goroutine to exit. Safe to call multiple times (idempotent).
func (w *Writer) Stop() {
	w.once.Do(func() { close(w.stop) })
	<-w.done
}

// SetOnFlush registers a callback that is called after each successful B2 delta
// upload. Use this to notify a Compactor that a new delta file has been created.
// Must be called before Start.
func (w *Writer) SetOnFlush(fn func()) {
	w.onFlush = fn
}

// EnqueuePut enqueues a "put" operation for async delta persistence.
// Non-blocking: ops are silently dropped when the channel is full (manifest
// is a cache — B2 headers remain authoritative for any missed entry).
func (w *Writer) EnqueuePut(bucket, objectKey string, entry *Entry) {
	select {
	case w.opsCh <- Op{Operation: "put", Key: bucket + "/" + objectKey, Entry: entry, Ts: time.Now().UTC()}:
	default:
	}
}

// EnqueueDelete enqueues a "del" operation. Non-blocking; see EnqueuePut.
func (w *Writer) EnqueueDelete(bucket, objectKey string) {
	select {
	case w.opsCh <- Op{Operation: "del", Key: bucket + "/" + objectKey, Ts: time.Now().UTC()}:
	default:
	}
}

// run is the background goroutine. It blocks on the ops channel, drains any
// immediately buffered ops into a single batch, then uploads one delta file.
// On stop or context cancellation it drains all remaining ops and flushes once.
func (w *Writer) run(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case op := <-w.opsCh:
			w.flush(w.drainInto(op))
		case <-w.stop:
			w.drainAndFlush()
			return
		case <-ctx.Done():
			w.drainAndFlush()
			return
		}
	}
}

// drainInto returns a batch starting with first, plus any additional ops
// already buffered in the channel (non-blocking drain).
func (w *Writer) drainInto(first Op) []Op {
	batch := []Op{first}
	for {
		select {
		case op := <-w.opsCh:
			batch = append(batch, op)
		default:
			return batch
		}
	}
}

// drainAndFlush drains all remaining ops from the channel and flushes them
// as a single delta file before the goroutine exits.
func (w *Writer) drainAndFlush() {
	var pending []Op
	for {
		select {
		case op := <-w.opsCh:
			pending = append(pending, op)
		default:
			w.flush(pending)
			return
		}
	}
}

// flush serialises ops to JSONL, increments the index sequence counter, and
// uploads the delta file directly to B2. The sequence is zero-padded to 10
// digits so lexicographic filename sort equals numeric sort on startup.
// Upload errors are silently tolerated — the manifest is a cache.
func (w *Writer) flush(ops []Op) {
	if len(ops) == 0 {
		return
	}
	data, err := MarshalDelta(ops)
	if err != nil {
		return
	}
	seq := w.idx.IncSeq()
	key := DeltaKey(w.prefix, w.writerID, seq)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := w.upload(ctx, key, data); err == nil {
		w.lastFlush = time.Now().UTC()
		if w.onFlush != nil {
			w.onFlush()
		}
	}
}

// LastFlush returns the timestamp of the most recent successful delta upload.
// A zero time indicates no successful flush has occurred since startup.
func (w *Writer) LastFlush() time.Time {
	return w.lastFlush
}
