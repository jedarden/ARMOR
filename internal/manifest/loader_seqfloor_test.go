package manifest

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"testing"
)

// mapLister serves keys from an in-memory store (single page).
func mapLister(store map[string][]byte) Lister {
	return func(ctx context.Context, prefix, token string) ([]string, string, error) {
		if token != "" {
			return nil, "", nil
		}
		var keys []string
		for k := range store {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys, "", nil
	}
}

// mapFetcher serves bytes from an in-memory store.
func mapFetcher(store map[string][]byte) Fetcher {
	return func(ctx context.Context, key string) ([]byte, error) {
		if data, ok := store[key]; ok {
			return data, nil
		}
		return nil, errors.New("no such key: " + key)
	}
}

// TestRecoverSeqFloorTakesMaxPersistedDeltaSequence pins the floor contract:
// only the current writer's delta keys count, the snapshot and foreign
// writers' shards are ignored, and the maximum sequence wins.
func TestRecoverSeqFloorTakesMaxPersistedDeltaSequence(t *testing.T) {
	const prefix = ".armor/manifest"
	const writer = "w1"
	store := map[string][]byte{
		DeltaKey(prefix, writer, 1): []byte("{}"),
		DeltaKey(prefix, writer, 3): []byte("{}"),
		DeltaKey(prefix, writer, 7): []byte("{}"),
		SnapshotKey(prefix, writer): []byte("{}"),
		// A foreign writer's higher sequence must not lift this writer's floor.
		DeltaKey(prefix, "other-writer", 99): []byte("{}"),
	}

	floor, err := RecoverSeqFloor(context.Background(), prefix, writer, mapLister(store))
	if err != nil {
		t.Fatalf("RecoverSeqFloor: %v", err)
	}
	if floor != 7 {
		t.Fatalf("floor = %d, want 7 (max persisted sequence of %q)", floor, writer)
	}
}

// TestRecoverSeqFloorEmptyShardIsZero: a writer with no persisted shard
// recovers floor 0 — the first-ever-delta case, not an error.
func TestRecoverSeqFloorEmptyShardIsZero(t *testing.T) {
	floor, err := RecoverSeqFloor(context.Background(), ".armor/manifest", "w1", mapLister(map[string][]byte{}))
	if err != nil {
		t.Fatalf("RecoverSeqFloor on empty shard: %v", err)
	}
	if floor != 0 {
		t.Fatalf("floor = %d, want 0", floor)
	}
}

// TestRecoverSeqFloorPropagatesListError: when the discovery list itself
// fails there is no floor, and the caller must hear about it rather than
// publish from sequence 1.
func TestRecoverSeqFloorPropagatesListError(t *testing.T) {
	listErr := errors.New("list blew up")
	_, err := RecoverSeqFloor(context.Background(), ".armor/manifest", "w1", func(ctx context.Context, prefix, token string) ([]string, string, error) {
		return nil, "", listErr
	})
	if err == nil {
		t.Fatal("RecoverSeqFloor swallowed the list error")
	}
}

// TestSeqFloorRecoveryCycleIsTheOverwriteFix is the manifest-level recovery
// half of the cold-load overwrite fix (armor-b1745d47): after a failed load
// the writer resumes above the recovered floor, and the next successful load
// replays every delta — pre-failure and recovery-era alike — into an index
// whose sequence continues uniquely.
func TestSeqFloorRecoveryCycleIsTheOverwriteFix(t *testing.T) {
	const prefix = ".armor/manifest"
	const writer = "w1"
	ctx := context.Background()

	// Era 1: a healthy writer persisted deltas 1..3.
	store := map[string][]byte{}
	seed := func(seq uint64, key string, size int64) {
		data, err := MarshalDelta([]Op{{
			Operation: "put",
			Key:       key,
			Entry:     &Entry{PlaintextSize: size, BlockSize: 65536},
		}})
		if err != nil {
			t.Fatalf("MarshalDelta: %v", err)
		}
		store[DeltaKey(prefix, writer, seq)] = data
	}
	seed(1, "b/obj-1", 11)
	seed(2, "b/obj-2", 22)
	seed(3, "b/obj-3", 33)

	// Cold load fails; the floor is recovered from key names alone.
	floor, err := RecoverSeqFloor(ctx, prefix, writer, mapLister(store))
	if err != nil {
		t.Fatalf("RecoverSeqFloor: %v", err)
	}
	if floor != 3 {
		t.Fatalf("floor = %d, want 3", floor)
	}

	// The degraded writer continues above the floor, never at 1.
	degraded := New()
	degraded.SetSeq(floor)
	next := degraded.IncSeq()
	if next != 4 {
		t.Fatalf("first post-recovery sequence = %d, want 4", next)
	}
	seed(next, "b/obj-4", 44)

	// Next startup: the load succeeds and must reconstruct every entry and
	// continue the sequence without reusing a number.
	reloaded := New()
	if err := Load(ctx, reloaded, prefix, writer, mapLister(store), mapFetcher(store)); err != nil {
		t.Fatalf("Load after recovery: %v", err)
	}
	for i := 1; i <= 4; i++ {
		key := "b/obj-" + strconv.Itoa(i)
		entry, ok := reloaded.Get("b", "obj-"+strconv.Itoa(i))
		if !ok {
			t.Fatalf("reloaded index missing %q", key)
		}
		if want := int64(i) * 11; entry.PlaintextSize != want {
			t.Fatalf("reloaded %q PlaintextSize = %d, want %d", key, entry.PlaintextSize, want)
		}
	}
	if got := reloaded.Seq(); got != 4 {
		t.Fatalf("reloaded seq = %d, want 4", got)
	}
	if next := reloaded.IncSeq(); next != 5 {
		t.Fatalf("first post-reload sequence = %d, want 5", next)
	}
}
