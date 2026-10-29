package manifest

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoadFetchesConcurrentlyAndReplaysInOrder(t *testing.T) {
	const deltaCount = manifestLoadConcurrency + 8

	keys := make([]string, 0, deltaCount)
	objects := make(map[string][]byte, deltaCount)
	for seq := uint64(1); seq <= deltaCount; seq++ {
		key := DeltaKey(".armor/test", "writer-a", seq)
		data, err := MarshalDelta([]Op{{
			Operation: "put",
			Key:       "bucket/shared",
			Entry:     &Entry{PlaintextSize: int64(seq)},
			Ts:        time.Unix(int64(seq), 0).UTC(),
		}})
		if err != nil {
			t.Fatalf("MarshalDelta(%d): %v", seq, err)
		}
		keys = append(keys, key)
		objects[key] = data
	}

	var inFlight atomic.Int64
	var maxInFlight atomic.Int64
	var completionMu sync.Mutex
	completionOrder := make([]uint64, 0, deltaCount)
	firstStarted := make(chan struct{})
	secondCompleted := make(chan struct{})
	var firstOnce sync.Once
	var secondOnce sync.Once
	releaseFirst := make(chan struct{})
	var releaseFirstOnce sync.Once
	release := func() { releaseFirstOnce.Do(func() { close(releaseFirst) }) }
	defer release()

	fetch := func(ctx context.Context, key string) ([]byte, error) {
		current := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			previous := maxInFlight.Load()
			if current <= previous || maxInFlight.CompareAndSwap(previous, current) {
				break
			}
		}

		seq, ok := DeltaSeqFromKey(".armor/test", "writer-a", key)
		if !ok {
			return nil, fmt.Errorf("unexpected manifest key %q", key)
		}
		if seq == 1 {
			firstOnce.Do(func() { close(firstStarted) })
			select {
			case <-releaseFirst:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		completionMu.Lock()
		completionOrder = append(completionOrder, seq)
		completionMu.Unlock()
		if seq == 2 {
			secondOnce.Do(func() { close(secondCompleted) })
		}
		return objects[key], nil
	}

	list := func(context.Context, string, string) ([]string, string, error) {
		listed := append([]string(nil), keys...)
		sort.Strings(listed)
		return listed, "", nil
	}
	idx := New()
	loadDone := make(chan error, 1)
	go func() {
		loadDone <- Load(context.Background(), idx, ".armor/test", "writer-a", list, fetch)
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first manifest fetch did not start")
	}
	select {
	case <-secondCompleted:
	case <-time.After(time.Second):
		t.Fatal("second manifest fetch did not complete while first was blocked")
	}
	if got := maxInFlight.Load(); got <= 1 {
		t.Fatalf("manifest fetches were not concurrent: max in flight = %d", got)
	}
	release()

	select {
	case err := <-loadDone:
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Load did not finish after fetches were released")
	}

	if got := maxInFlight.Load(); got > manifestLoadConcurrency {
		t.Fatalf("max in-flight fetches = %d, want <= %d", got, manifestLoadConcurrency)
	}
	completionMu.Lock()
	if len(completionOrder) == 0 || completionOrder[0] == 1 {
		t.Fatalf("fetch completion was not out of order: %v", completionOrder)
	}
	completionMu.Unlock()

	entry, ok := idx.Get("bucket", "shared")
	if !ok {
		t.Fatal("replayed entry missing")
	}
	if entry.PlaintextSize != deltaCount {
		t.Fatalf("ordered replay lost the final delta: size = %d, want %d", entry.PlaintextSize, deltaCount)
	}
	if idx.Seq() != deltaCount {
		t.Fatalf("current-writer sequence = %d, want %d", idx.Seq(), deltaCount)
	}
}

func TestLoadFetchErrorCancelsOutstandingFetches(t *testing.T) {
	const deltaCount = manifestLoadConcurrency + 2
	keys, _ := testDeltaObjects(t, deltaCount)
	sentinel := errors.New("fetch failed")
	var started atomic.Int64
	var canceled atomic.Int64
	startedTwice := make(chan struct{})
	var startedTwiceOnce sync.Once
	releaseFailure := make(chan struct{})

	fetch := func(ctx context.Context, key string) ([]byte, error) {
		count := started.Add(1)
		if count >= 2 {
			startedTwiceOnce.Do(func() { close(startedTwice) })
		}
		seq, ok := DeltaSeqFromKey(".armor/test", "writer-a", key)
		if !ok {
			return nil, fmt.Errorf("unexpected manifest key %q", key)
		}
		if seq == 1 {
			select {
			case <-releaseFailure:
				return nil, sentinel
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		<-ctx.Done()
		canceled.Add(1)
		return nil, ctx.Err()
	}

	idx := New()
	loadDone := make(chan error, 1)
	go func() {
		loadDone <- Load(context.Background(), idx, ".armor/test", "writer-a", testList(keys), fetch)
	}()
	select {
	case <-startedTwice:
	case <-time.After(time.Second):
		close(releaseFailure)
		t.Fatal("fetch failure test did not start overlapping fetches")
	}
	close(releaseFailure)

	select {
	case err := <-loadDone:
		if !errors.Is(err, sentinel) {
			t.Fatalf("Load error = %v, want sentinel fetch error", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Load did not return promptly after a fetch failure")
	}
	if canceled.Load() == 0 {
		t.Fatal("fetch failure did not cancel an outstanding fetch")
	}
	if idx.Len() != 0 {
		t.Fatalf("failed load partially applied %d entries", idx.Len())
	}
}

func TestLoadCancellationCancelsFetches(t *testing.T) {
	keys, _ := testDeltaObjects(t, manifestLoadConcurrency+2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var started atomic.Int64
	var canceled atomic.Int64
	startedTwice := make(chan struct{})
	var startedTwiceOnce sync.Once
	fetch := func(ctx context.Context, key string) ([]byte, error) {
		if started.Add(1) >= 2 {
			startedTwiceOnce.Do(func() { close(startedTwice) })
		}
		<-ctx.Done()
		canceled.Add(1)
		return nil, ctx.Err()
	}

	loadDone := make(chan error, 1)
	go func() {
		loadDone <- Load(ctx, New(), ".armor/test", "writer-a", testList(keys), fetch)
	}()
	select {
	case <-startedTwice:
		cancel()
	case <-time.After(time.Second):
		cancel()
		t.Fatal("cancellation test did not start overlapping fetches")
	}

	select {
	case err := <-loadDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Load error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Load did not return promptly after cancellation")
	}
	if canceled.Load() == 0 {
		t.Fatal("caller cancellation did not reach an outstanding fetch")
	}
}

func testDeltaObjects(t *testing.T, count int) ([]string, map[string][]byte) {
	t.Helper()
	keys := make([]string, 0, count)
	objects := make(map[string][]byte, count)
	for seq := uint64(1); seq <= uint64(count); seq++ {
		key := DeltaKey(".armor/test", "writer-a", seq)
		data, err := MarshalDelta([]Op{{
			Operation: "put",
			Key:       "bucket/object",
			Entry:     &Entry{PlaintextSize: int64(seq)},
		}})
		if err != nil {
			t.Fatalf("MarshalDelta(%d): %v", seq, err)
		}
		keys = append(keys, key)
		objects[key] = data
	}
	return keys, objects
}

func testList(keys []string) Lister {
	return func(context.Context, string, string) ([]string, string, error) {
		listed := append([]string(nil), keys...)
		sort.Strings(listed)
		return listed, "", nil
	}
}
