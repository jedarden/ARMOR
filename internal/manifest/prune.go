package manifest

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// pruneBatchSize caps one pruner call. B2's S3 DeleteObjects accepts at most
// 1000 keys per request, and the backend does not chunk on the caller's
// behalf.
const pruneBatchSize = 1000

// ObjectStamp is one manifest object observed during a stale-shard scan,
// carrying the object-store timestamp it was last written at.
type ObjectStamp struct {
	Key          string
	LastModified time.Time
}

// StaleLister paginates manifest objects under a prefix together with their
// timestamps. token "" starts the listing; a "" nextToken ends it.
type StaleLister func(ctx context.Context, prefix, token string) (stamps []ObjectStamp, nextToken string, err error)

// Pruner deletes one batch of manifest objects. Batches are independent, so
// a failed batch leaves the rest of the outcome well-defined; an error means
// that batch may be partially applied.
type Pruner func(ctx context.Context, keys []string) error

// PruneStaleShards deletes manifest objects that belong to writer shards
// other than currentWriter and were last modified before cutoff. It returns
// the number of objects handed to the pruner.
//
// Every ARMOR writer persists under its own shard named by ARMOR_WRITER_ID —
// the pod name in Kubernetes — so each pod replacement abandons a shard no
// compactor will ever fold and every cold load re-fetches. Left unpruned the
// corpus grows one shard per restart until the load overruns its budget and
// the pod serves with an empty index (iad-ci: 8,244 objects across 71 shards,
// >480 s, empty-index fallback on every start).
//
// Safety contract for callers:
//
//   - Prune only after a fully successful Load of the same prefix. The load
//     merged every discovered object into the live index, so a pruned object
//     loses no entry; a load that fell back to the empty index must not
//     prune, or it would discard the only compacted copy of entries the
//     empty index does not hold.
//   - cutoff must predate the load's discovery walk (callers subtract a
//     clock-skew margin from the load start). Objects written after the load
//     began keep newer timestamps and survive.
//   - The current writer's shard is never touched, including its snapshot.
//
// B2 object headers remain the authoritative source of truth for every
// entry, so even an unsafe prune degrades reads to the HEAD fall-through —
// it cannot serve wrong data. Deletion is chunked into pruneBatchSize-key
// batches and stops at the first failing batch.
func PruneStaleShards(ctx context.Context, prefix, currentWriter string, cutoff time.Time, list StaleLister, pruner Pruner) (int, error) {
	var stale []string
	token := ""
	for {
		stamps, next, err := list(ctx, prefix+"/", token)
		if err != nil {
			return 0, fmt.Errorf("manifest prune discovery list: %w", err)
		}
		for _, s := range stamps {
			shard := shardOfObject(prefix, s.Key)
			if shard == "" {
				continue // malformed or shardless: the loader skips it, so do not delete it
			}
			if shard != currentWriter && s.LastModified.Before(cutoff) {
				stale = append(stale, s.Key)
			}
		}
		if next == "" {
			break
		}
		token = next
	}
	deleted := 0
	for start := 0; start < len(stale); start += pruneBatchSize {
		end := start + pruneBatchSize
		if end > len(stale) {
			end = len(stale)
		}
		batch := stale[start:end]
		if err := pruner(ctx, batch); err != nil {
			return deleted, fmt.Errorf("manifest prune delete (batch of %d, %d deleted so far): %w", len(batch), deleted, err)
		}
		deleted += len(batch)
	}
	return deleted, nil
}

// shardOfObject returns the writer shard a manifest object key belongs to,
// or "" when the key has no shard segment. Mirrors the classification
// groupByWriter applies, so a key the loader would skip is never pruned.
func shardOfObject(prefix, key string) string {
	rel := strings.TrimPrefix(key, prefix+"/")
	sep := strings.Index(rel, "/")
	if sep <= 0 {
		return ""
	}
	return rel[:sep]
}
