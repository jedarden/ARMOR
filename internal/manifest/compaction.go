package manifest

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Deleter batch-deletes B2 objects by key. Used to remove obsolete delta files
// after a snapshot has been uploaded. Keys are full B2 object keys.
type Deleter func(ctx context.Context, keys []string) error

// Compactor manages periodic compaction of manifest delta files into a single
// snapshot. It runs in a background goroutine and triggers compaction when:
//   - The timer fires (interval-based), or
//   - The delta count since the last compaction reaches the threshold.
//
// Compaction steps:
//  1. Note current idx.Seq() as the compaction point.
//  2. Serialize the in-memory index to snapshot.json.gz and upload to B2,
//     overwriting any existing snapshot.
//  3. List all delta files for this writer with seq <= compaction point.
//  4. Batch-delete those delta files.
//
// Compaction is best-effort: errors are tolerated because the manifest is a
// performance cache. B2 object headers remain the authoritative source of truth.
// In-flight writes continue creating new delta files during compaction; those
// new deltas (seq > compaction point) survive and are replayed on the next startup.
type Compactor struct {
	idx      *Index
	prefix   string
	writerID string
	upload   Uploader
	list     Lister
	del      Deleter

	interval  time.Duration
	threshold int // delta count that triggers early compaction (0 = disabled)

	// triggerCh receives a signal when the threshold is reached.
	// Buffered capacity 1 so a racing NotifyDelta doesn't block.
	triggerCh chan struct{}

	stop chan struct{}
	done chan struct{}
	once sync.Once

	mu          sync.Mutex
	deltasSince int // deltas written since last compaction
}

// NewCompactor creates a Compactor. Call Start to launch the background goroutine.
// interval controls how often compaction runs automatically; threshold is the
// delta count that triggers an early compaction (0 disables threshold-based
// compaction).
func NewCompactor(
	idx *Index,
	prefix, writerID string,
	upload Uploader,
	list Lister,
	del Deleter,
	interval time.Duration,
	threshold int,
) *Compactor {
	return &Compactor{
		idx:       idx,
		prefix:    prefix,
		writerID:  writerID,
		upload:    upload,
		list:      list,
		del:       del,
		interval:  interval,
		threshold: threshold,
		triggerCh: make(chan struct{}, 1),
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start launches the background compaction goroutine.
func (c *Compactor) Start(ctx context.Context) {
	go c.run(ctx)
}

// Stop signals the compaction goroutine to exit and waits for it to finish.
// Safe to call multiple times (idempotent).
func (c *Compactor) Stop() {
	c.once.Do(func() { close(c.stop) })
	<-c.done
}

// NotifyDelta is called by the manifest Writer after each successful delta flush.
// It increments the internal counter and signals an early compaction if the
// threshold is reached.
func (c *Compactor) NotifyDelta() {
	c.mu.Lock()
	c.deltasSince++
	trigger := c.threshold > 0 && c.deltasSince >= c.threshold
	c.mu.Unlock()
	if trigger {
		select {
		case c.triggerCh <- struct{}{}:
		default:
		}
	}
}

// run is the background goroutine. It blocks until the timer fires, a threshold
// signal arrives, or stop/context cancellation is requested.
func (c *Compactor) run(ctx context.Context) {
	defer close(c.done)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.compact(ctx)
		case <-c.triggerCh:
			c.compact(ctx)
			ticker.Reset(c.interval) // restart timer after threshold-triggered compaction
		case <-c.stop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// compact runs one compaction pass and resets the delta counter regardless of
// whether the compaction succeeded (so we don't spin on repeated errors).
func (c *Compactor) compact(ctx context.Context) {
	_ = c.doCompact(ctx) // errors are silently tolerated
	c.mu.Lock()
	c.deltasSince = 0
	c.mu.Unlock()
}

// doCompact performs the core compaction logic and returns any error.
// Exported only for tests; callers should use compact (which swallows errors).
func (c *Compactor) doCompact(ctx context.Context) error {
	// Capture the compaction point before any B2 I/O.
	compactionSeq := c.idx.Seq()
	if compactionSeq == 0 {
		return nil // nothing has been written yet
	}

	// 1. Serialize the current in-memory index to a gzip snapshot.
	data, err := c.idx.MarshalSnapshot()
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	// 2. Upload snapshot, overwriting any previous snapshot.json.gz.
	snapshotKey := SnapshotKey(c.prefix, c.writerID)
	uploadCtx, uploadCancel := context.WithTimeout(ctx, 2*time.Minute)
	defer uploadCancel()
	if err := c.upload(uploadCtx, snapshotKey, data); err != nil {
		return fmt.Errorf("upload snapshot: %w", err)
	}

	// 3. List all objects under this writer's B2 prefix.
	writerPfx := WriterPrefix(c.prefix, c.writerID)
	allKeys, err := listAllKeys(ctx, writerPfx, c.list)
	if err != nil {
		return fmt.Errorf("list delta files: %w", err)
	}

	// 4. Collect delta keys with seq <= compaction point.
	var toDelete []string
	for _, key := range allKeys {
		seq, ok := DeltaSeqFromKey(c.prefix, c.writerID, key)
		if ok && seq <= compactionSeq {
			toDelete = append(toDelete, key)
		}
	}

	if len(toDelete) == 0 {
		return nil
	}

	// 5. Batch-delete in chunks of 1000 (B2 DeleteObjects limit).
	const batchSize = 1000
	for i := 0; i < len(toDelete); i += batchSize {
		end := i + batchSize
		if end > len(toDelete) {
			end = len(toDelete)
		}
		delCtx, delCancel := context.WithTimeout(ctx, 2*time.Minute)
		err := c.del(delCtx, toDelete[i:end])
		delCancel()
		if err != nil {
			return fmt.Errorf("batch delete deltas [%d:%d]: %w", i, end, err)
		}
	}

	return nil
}
