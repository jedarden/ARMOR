package manifest

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

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
		return fmt.Errorf("manifest discovery list: %w", err)
	}
	if len(allKeys) == 0 {
		return nil // first startup — nothing to load
	}

	// Step 2: Group by writer shard.
	writerKeys := groupByWriter(prefix, allKeys)

	// Step 3+4+5: Load each shard, merge into idx.
	for writerID, keys := range writerKeys {
		workerIdx := New()
		maxSeq, err := loadWriterShard(ctx, workerIdx, prefix, writerID, keys, fetch)
		if err != nil {
			return fmt.Errorf("load writer shard %q: %w", writerID, err)
		}

		// Merge this writer's entries into the unified index.
		idx.Merge(workerIdx.All())

		// Set the seq counter only from the current writer's shard so that new
		// delta files written by this instance have unique sequence numbers.
		if writerID == currentWriter && maxSeq > idx.Seq() {
			idx.SetSeq(maxSeq)
		}
	}

	return nil
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
		switch {
		case k == snapshotKey:
			hasSnapshot = true
		default:
			if _, ok := DeltaSeqFromKey(prefix, writerID, k); ok {
				deltaKeys = append(deltaKeys, k)
			}
		}
	}

	// Sort delta keys lexicographically — zero-padded sequence numbers make
	// lexicographic order identical to numeric order.
	sort.Strings(deltaKeys)

	// Load snapshot first so delta replay builds on top of it.
	if hasSnapshot {
		data, ferr := fetch(ctx, snapshotKey)
		if ferr != nil {
			return 0, fmt.Errorf("fetch snapshot: %w", ferr)
		}
		if uerr := workerIdx.UnmarshalSnapshot(data); uerr != nil {
			return 0, fmt.Errorf("parse snapshot: %w", uerr)
		}
	}

	// Replay each delta in sequence order.
	for _, dk := range deltaKeys {
		seq, ok := DeltaSeqFromKey(prefix, writerID, dk)
		if !ok {
			continue
		}
		data, ferr := fetch(ctx, dk)
		if ferr != nil {
			return maxSeq, fmt.Errorf("fetch delta %q: %w", dk, ferr)
		}
		if uerr := workerIdx.UnmarshalDelta(data); uerr != nil {
			return maxSeq, fmt.Errorf("parse delta %q: %w", dk, uerr)
		}
		if seq > maxSeq {
			maxSeq = seq
		}
	}

	return maxSeq, nil
}
