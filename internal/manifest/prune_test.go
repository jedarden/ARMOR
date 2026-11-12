package manifest

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"
)

type recordingPruner struct {
	batches [][]string
	err     error
}

func (r *recordingPruner) prune(_ context.Context, keys []string) error {
	batch := slices.Clone(keys)
	r.batches = append(r.batches, batch)
	return r.err
}

func stamps(keys ...string) []ObjectStamp {
	base := time.Date(2026, 10, 31, 4, 0, 0, 0, time.UTC)
	out := make([]ObjectStamp, len(keys))
	for i, k := range keys {
		out[i] = ObjectStamp{Key: k, LastModified: base}
	}
	return out
}

func TestPruneStaleShardsDeletesOnlyStaleForeignObjects(t *testing.T) {
	cutoff := time.Date(2026, 10, 31, 4, 0, 0, 0, time.UTC)
	oneLister := func(_ context.Context, prefix, token string) ([]ObjectStamp, string, error) {
		if token != "" {
			return nil, "", nil
		}
		if prefix != "m/" {
			t.Errorf("list prefix %q, want %q", prefix, "m/")
		}
		return stamps(
			"m/writer-alive/delta-0000000001.jsonl", // current shard: never pruned
			"m/writer-alive/snapshot.json.gz",       // current shard: never pruned
			"m/writer-fresh/delta-0000000002.jsonl", // foreign but newer than cutoff: kept
		), "", nil
	}
	// The dead shard's objects are stamped before the cutoff; the fresh
	// foreign object after it. The cutoff comparison is strict, so equal
	// timestamps survive.
	listed, _, err := oneLister(context.Background(), "m/", "")
	if err != nil {
		t.Fatalf("discovery: %v", err)
	}
	listed = append(listed,
		ObjectStamp{Key: "m/writer-dead/delta-0000000007.jsonl", LastModified: cutoff.Add(-time.Minute)},
		ObjectStamp{Key: "m/writer-dead/snapshot.json.gz", LastModified: cutoff.Add(-2 * time.Minute)},
	)

	p := &recordingPruner{}
	n, err := PruneStaleShards(context.Background(), "m", "writer-alive", cutoff,
		func(ctx context.Context, prefix, token string) ([]ObjectStamp, string, error) {
			return listed, "", nil
		}, p.prune)
	if err != nil {
		t.Fatalf("PruneStaleShards: %v", err)
	}
	if n != 2 {
		t.Fatalf("pruned %d objects, want 2", n)
	}
	if len(p.batches) != 1 {
		t.Fatalf("%d pruner batches, want 1", len(p.batches))
	}
	want := []string{"m/writer-dead/delta-0000000007.jsonl", "m/writer-dead/snapshot.json.gz"}
	if !slices.Equal(p.batches[0], want) {
		t.Errorf("pruned keys %v, want %v", p.batches[0], want)
	}
}

func TestPruneStaleShardsSkipsMalformedKeys(t *testing.T) {
	cutoff := time.Now().Add(-time.Hour)
	listed := stamps(
		"m/stray.jsonl",             // no shard segment: the loader skips it too
		"m//delta-0000000001.jsonl", // empty writer ID: skipped
	)
	p := &recordingPruner{}
	n, err := PruneStaleShards(context.Background(), "m", "writer-alive", cutoff,
		func(context.Context, string, string) ([]ObjectStamp, string, error) {
			return listed, "", nil
		}, p.prune)
	if err != nil {
		t.Fatalf("PruneStaleShards: %v", err)
	}
	if n != 0 || len(p.batches) != 0 {
		t.Errorf("pruned %d objects in %d batches, want none: %v", n, len(p.batches), p.batches)
	}
}

func TestPruneStaleShardsChunksBatches(t *testing.T) {
	cutoff := time.Now().Add(-time.Hour)
	const total = pruneBatchSize + 50
	listed := make([]ObjectStamp, 0, total)
	for i := 0; i < total; i++ {
		listed = append(listed, ObjectStamp{
			Key:          fmt.Sprintf("m/writer-dead/delta-%010d.jsonl", i),
			LastModified: cutoff.Add(-time.Minute),
		})
	}
	p := &recordingPruner{}
	n, err := PruneStaleShards(context.Background(), "m", "writer-alive", cutoff,
		func(context.Context, string, string) ([]ObjectStamp, string, error) {
			return listed, "", nil
		}, p.prune)
	if err != nil {
		t.Fatalf("PruneStaleShards: %v", err)
	}
	if n != total {
		t.Errorf("pruned %d objects, want %d", n, total)
	}
	if len(p.batches) != 2 || len(p.batches[0]) != pruneBatchSize || len(p.batches[1]) != 50 {
		t.Errorf("batches %d (%d, %d), want 2 (%d, 50)",
			len(p.batches), len(p.batches[0]), len(p.batches[1]), pruneBatchSize)
	}
}

func TestPruneStaleShardsPaginationAndErrors(t *testing.T) {
	cutoff := time.Now().Add(-time.Hour)
	t.Run("list error", func(t *testing.T) {
		p := &recordingPruner{}
		_, err := PruneStaleShards(context.Background(), "m", "w", cutoff,
			func(context.Context, string, string) ([]ObjectStamp, string, error) {
				return nil, "", errors.New("backend down")
			}, p.prune)
		if err == nil || len(p.batches) != 0 {
			t.Errorf("want list error and no deletes, got err=%v batches=%d", err, len(p.batches))
		}
	})
	t.Run("pruner error reports progress", func(t *testing.T) {
		p := &recordingPruner{err: errors.New("delete failed")}
		listed := stamps("m/writer-dead/delta-0000000001.jsonl", "m/writer-dead/delta-0000000002.jsonl")
		for i := range listed {
			listed[i].LastModified = cutoff.Add(-time.Minute)
		}
		n, err := PruneStaleShards(context.Background(), "m", "writer-alive", cutoff,
			func(context.Context, string, string) ([]ObjectStamp, string, error) {
				return listed, "", nil
			}, p.prune)
		if err == nil {
			t.Fatal("want pruner error propagated")
		}
		if n != 0 {
			t.Errorf("reported %d durable deletions behind a failing first batch, want 0", n)
		}
	})
	t.Run("pagination walks every page", func(t *testing.T) {
		p := &recordingPruner{}
		stale := func(keys ...string) []ObjectStamp {
			out := stamps(keys...)
			for i := range out {
				out[i].LastModified = cutoff.Add(-time.Minute)
			}
			return out
		}
		pages := [][]ObjectStamp{
			stale("m/writer-dead/a.jsonl"),
			stale("m/writer-dead/b.jsonl"),
			nil, // empty final page with an empty token still ends the walk
		}
		calls := 0
		n, err := PruneStaleShards(context.Background(), "m", "writer-alive", cutoff,
			func(context.Context, string, string) ([]ObjectStamp, string, error) {
				page := pages[calls]
				token := ""
				if calls < len(pages)-1 {
					token = fmt.Sprintf("token-%d", calls)
				}
				calls++
				return page, token, nil
			}, p.prune)
		if err != nil {
			t.Fatalf("PruneStaleShards: %v", err)
		}
		if calls != len(pages) {
			t.Errorf("discovery listed %d pages, want %d", calls, len(pages))
		}
		if n != 2 {
			t.Errorf("pruned %d objects, want 2", n)
		}
	})
}
