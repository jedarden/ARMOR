package manifest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"golang.org/x/sync/errgroup"
)

// manifestLoadConcurrency bounds the number of snapshot and delta bodies that
// a cold load fetches at once. The replay remains ordered below, so this only
// overlaps I/O and does not change manifest semantics.
const manifestLoadConcurrency = 16

// Lister enumerates manifest object keys under a given prefix (paginated).
// prefix is the full B2 object-key prefix (e.g. ".armor/manifest/").
// token is the continuation token from a previous call; pass "" for the first page.
// Returns the keys found, the next continuation token (or "" when done), and any error.
type Lister func(ctx context.Context, prefix, token string) (keys []string, nextToken string, err error)

// Fetcher returns the full byte content of the manifest object at key.
// Used to download snapshot.json.gz and delta-*.jsonl files.
type Fetcher func(ctx context.Context, key string) ([]byte, error)

// Load performs the manifest startup load sequence:
//  1. Paginates through all manifest objects under prefix+"/" using list
//     (one direct-B2 Class C call per page — not through Cloudflare).
//  2. Groups discovered keys by writer shard.
//  3. For each writer shard, loads snapshot.json.gz (if present) then replays
//     all delta-*.jsonl files in lexicographic (= numeric by sequence) order.
//  4. Merges all writers' entries into idx using last-write-wins by LastModified.
//  5. Sets idx.Seq to the maximum delta sequence number found for currentWriter,
//     so the first delta written after startup gets a unique sequence number.
//
// If no manifest objects exist yet (first startup ever), idx is left empty and
// nil is returned. The manifest is a performance optimisation — callers should
// log and ignore non-nil errors rather than failing startup.
//
// list and fetch are provided as functions so the manifest package has no
// dependency on the backend package; tests can supply lightweight fakes.
func Load(
	ctx context.Context,
	idx *Index,
	prefix string,
	currentWriter string,
	list Lister,
	fetch Fetcher,
) error {
	// Step 1: Discover all manifest objects under "{prefix}/".
	allKeys, err := listAllKeys(ctx, prefix+"/", list)
	if err != nil {
		// A store that does not exist yet holds nothing to discover: the
		// filesystem backend fails the bucket walk with ENOENT where B2
		// simply lists an empty bucket. That is the documented first-startup
		// case, not a load failure — and crucially it is decided here, on
		// the discovery error only, so a delta that vanishes between list
		// and fetch (ENOENT on a fetch) still fails the load and lands in
		// the caller's sequence-floor fallback.
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("manifest discovery list: %w", err)
	}
	if len(allKeys) == 0 {
		return nil // first startup — nothing to load
	}

	// Step 2: Group by writer shard.
	writerKeys := groupByWriter(prefix, allKeys)

	// Step 3: Load all shards concurrently, with one shared fetch bound. Each
	// shard replays into a private index so completion order cannot affect the
	// final merge.
	writerIDs := make([]string, 0, len(writerKeys))
	for writerID := range writerKeys {
		writerIDs = append(writerIDs, writerID)
	}
	sort.Strings(writerIDs)
	type shardResult struct {
		idx    *Index
		maxSeq uint64
	}
	results := make([]shardResult, len(writerIDs))
	fetchSlots := make(chan struct{}, manifestLoadConcurrency)
	boundedFetch := func(fetchCtx context.Context, key string) ([]byte, error) {
		select {
		case fetchSlots <- struct{}{}:
		case <-fetchCtx.Done():
			return nil, fetchCtx.Err()
		}
		defer func() { <-fetchSlots }()
		return fetch(fetchCtx, key)
	}

	g, loadCtx := errgroup.WithContext(ctx)
	for i, writerID := range writerIDs {
		i, writerID := i, writerID
		g.Go(func() error {
			workerIdx := New()
			maxSeq, err := loadWriterShard(loadCtx, workerIdx, prefix, writerID, writerKeys[writerID], boundedFetch)
			if err != nil {
				return fmt.Errorf("load writer shard %q: %w", writerID, err)
			}
			results[i] = shardResult{idx: workerIdx, maxSeq: maxSeq}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Step 4+5: Merge shards in stable writer order and set the current
	// writer's sequence floor only from that writer's replay.
	for i, writerID := range writerIDs {
		result := results[i]
		idx.Merge(result.idx.All())
		if writerID == currentWriter && result.maxSeq > idx.Seq() {
			idx.SetSeq(result.maxSeq)
		}
	}

	return nil
}

// RecoverSeqFloor lists only the current writer's shard and returns the
// highest persisted delta sequence number (0 when the shard holds no deltas).
// It is the cold-load failure fallback: delta key names alone encode the
// sequence, so a successful list recovers a safe high-water mark for the
// writer without fetching a single delta body. A returned error means no
// floor could be established and the caller must not publish deltas —
// restarting the sequence at 1 would overwrite persisted
// delta-0000000001.jsonl and friends (plan §9.6).
func RecoverSeqFloor(ctx context.Context, prefix, writerID string, list Lister) (uint64, error) {
	allKeys, err := listAllKeys(ctx, WriterPrefix(prefix, writerID), list)
	if err != nil {
		// Same empty-store classification as Load: no store means no
		// persisted delta anywhere, so 0 is a safe floor and the writer may
		// publish — this is the first-startup case, not an unsafe recovery.
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, fmt.Errorf("seq-floor discovery list: %w", err)
	}
	var floor uint64
	for _, key := range allKeys {
		if seq, ok := DeltaSeqFromKey(prefix, writerID, key); ok && seq > floor {
			floor = seq
		}
	}
	return floor, nil
}

// listAllKeys pages through all object keys under prefix using list.
func listAllKeys(ctx context.Context, prefix string, list Lister) ([]string, error) {
	var all []string
	token := ""
	for {
		batch, next, err := list(ctx, prefix, token)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if next == "" {
			break
		}
		token = next
	}
	return all, nil
}

// groupByWriter partitions object keys by writer shard ID.
// Keys are expected to have the form "{prefix}/{writerID}/...".
func groupByWriter(prefix string, keys []string) map[string][]string {
	base := prefix + "/"
	result := make(map[string][]string)
	for _, key := range keys {
		rel := strings.TrimPrefix(key, base)
		sep := strings.Index(rel, "/")
		if sep <= 0 {
			continue // malformed or empty writer ID — skip
		}
		writerID := rel[:sep]
		result[writerID] = append(result[writerID], key)
	}
	return result
}

// loadWriterShard loads snapshot + deltas for one writer shard into workerIdx.
// Returns the maximum delta sequence number found (0 if no deltas).
func loadWriterShard(
	ctx context.Context,
	workerIdx *Index,
	prefix, writerID string,
	keys []string,
	fetch Fetcher,
) (maxSeq uint64, err error) {
	snapshotKey := SnapshotKey(prefix, writerID)

	var hasSnapshot bool
	var deltaKeys []string

	for _, k := range keys {
		if k == snapshotKey {
			hasSnapshot = true
		} else if _, ok := DeltaSeqFromKey(prefix, writerID, k); ok {
			deltaKeys = append(deltaKeys, k)
		}
	}

	// Sort delta keys lexicographically — zero-padded sequence numbers make
	// lexicographic order identical to numeric order.
	sort.Strings(deltaKeys)

	// Fetch the complete shard concurrently, but retain the snapshot-then-delta
	// order in fetchKeys so replay below is deterministic regardless of fetch
	// completion order. No body is applied until every fetch succeeds, which
	// keeps the destination index unchanged on a cold-load fetch failure.
	fetchKeys := make([]string, 0, len(deltaKeys)+1)
	if hasSnapshot {
		fetchKeys = append(fetchKeys, snapshotKey)
	}
	fetchKeys = append(fetchKeys, deltaKeys...)
	fetched, ferr := fetchManifestObjects(ctx, fetchKeys, fetch)
	if ferr != nil {
		return maxSeq, formatManifestFetchError(fetchKeys, snapshotKey, ferr)
	}

	// Replay the snapshot first so delta records build on top of it.
	fetchOffset := 0
	if hasSnapshot {
		if uerr := workerIdx.UnmarshalSnapshot(fetched[fetchOffset]); uerr != nil {
			return 0, fmt.Errorf("parse snapshot: %w", uerr)
		}
		fetchOffset++
	}

	// Replay each delta in sequence order, not completion order.
	for i, dk := range deltaKeys {
		seq, ok := DeltaSeqFromKey(prefix, writerID, dk)
		if !ok {
			continue
		}
		if uerr := workerIdx.UnmarshalDelta(fetched[fetchOffset+i]); uerr != nil {
			return maxSeq, fmt.Errorf("parse delta %q: %w", dk, uerr)
		}
		if seq > maxSeq {
			maxSeq = seq
		}
	}

	return maxSeq, nil
}

type manifestFetchFailure struct {
	index int
	err   error
}

func (e *manifestFetchFailure) Error() string {
	return e.err.Error()
}

func (e *manifestFetchFailure) Unwrap() error {
	return e.err
}

// fetchManifestObjects fetches keys into slots matching their input order.
// The workers only populate slots; callers are responsible for replaying those
// slots in order after all fetches finish.
func fetchManifestObjects(ctx context.Context, keys []string, fetch Fetcher) ([][]byte, error) {
	fetched := make([][]byte, len(keys))
	if len(keys) == 0 {
		return fetched, nil
	}

	workerCount := manifestLoadConcurrency
	if len(keys) < workerCount {
		workerCount = len(keys)
	}

	jobs := make(chan int)
	g, fetchCtx := errgroup.WithContext(ctx)

	// Dispatch through the same cancellation-aware group so an early fetch
	// failure stops work that has not started yet.
	g.Go(func() error {
		defer close(jobs)
		for i := range keys {
			select {
			case jobs <- i:
			case <-fetchCtx.Done():
				return fetchCtx.Err()
			}
		}
		return nil
	})

	for worker := 0; worker < workerCount; worker++ {
		g.Go(func() error {
			for {
				select {
				case <-fetchCtx.Done():
					return fetchCtx.Err()
				case i, ok := <-jobs:
					if !ok {
						return nil
					}
					data, err := fetch(fetchCtx, keys[i])
					if err != nil {
						return &manifestFetchFailure{index: i, err: err}
					}
					fetched[i] = data
				}
			}
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return fetched, nil
}

func formatManifestFetchError(keys []string, snapshotKey string, err error) error {
	var failure *manifestFetchFailure
	if errors.As(err, &failure) && failure.index >= 0 && failure.index < len(keys) {
		key := keys[failure.index]
		if key == snapshotKey {
			return fmt.Errorf("fetch snapshot: %w", failure.err)
		}
		return fmt.Errorf("fetch delta %q: %w", key, failure.err)
	}
	return fmt.Errorf("fetch manifest objects: %w", err)
}
