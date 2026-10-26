package backend

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type delayedMultipartPartsBackend struct {
	Backend
	objects []ObjectInfo
	parts   map[int][]byte
	delay   time.Duration
	active  atomic.Int32
	maximum atomic.Int32
}

func (b *delayedMultipartPartsBackend) ListRaw(context.Context, string, string, string, string, int) (*ListResult, error) {
	return &ListResult{Objects: b.objects}, nil
}

func (b *delayedMultipartPartsBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	current := b.active.Add(1)
	for {
		previous := b.maximum.Load()
		if current <= previous || b.maximum.CompareAndSwap(previous, current) {
			break
		}
	}
	defer b.active.Add(-1)

	timer := time.NewTimer(b.delay)
	select {
	case <-ctx.Done():
		timer.Stop()
		return nil, nil, ctx.Err()
	case <-timer.C:
	}

	partNumber, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(key, ".armor/multipart/test-upload/part-"), ".json"))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid test part key %q: %w", key, err)
	}
	data, ok := b.parts[partNumber]
	if !ok {
		return nil, nil, fmt.Errorf("part %d is missing", partNumber)
	}
	return io.NopCloser(strings.NewReader(string(data))), &ObjectInfo{Key: key}, nil
}

func TestListPartsV3LoadsPartRecordsConcurrently(t *testing.T) {
	const partCount = 200
	objects := make([]ObjectInfo, 0, partCount)
	parts := make(map[int][]byte, partCount)
	for partNumber := 1; partNumber <= partCount; partNumber++ {
		key := fmt.Sprintf(".armor/multipart/test-upload/part-%d.json", partNumber)
		objects = append(objects, ObjectInfo{Key: key})
		parts[partNumber] = []byte(fmt.Sprintf("{\"part_number\":%d}", partNumber))
	}

	backend := &delayedMultipartPartsBackend{
		Backend: NewNilBackend(),
		objects: objects,
		parts:   parts,
		delay:   100 * time.Millisecond,
	}
	manager := NewMultipartStateManager(backend, "test-bucket")

	started := time.Now()
	loaded, err := manager.ListPartsV3(context.Background(), "test-upload")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("ListPartsV3() error = %v", err)
	}
	if len(loaded) != partCount {
		t.Fatalf("ListPartsV3() loaded %d parts, want %d", len(loaded), partCount)
	}
	serialBaseline := time.Duration(partCount) * backend.delay
	if serialBaseline < 20*time.Second {
		t.Fatalf("test setup serial baseline = %s, want at least 20s", serialBaseline)
	}
	if maximum := backend.maximum.Load(); maximum < 2 {
		t.Fatalf("part reads were not concurrent: maximum active reads = %d", maximum)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("part loading took %s, want <= 2s at 100ms RTT (serial baseline is >= 20s)", elapsed)
	}
}

func TestListPartsV3SkipsMissingOrMalformedPartRecords(t *testing.T) {
	objects := []ObjectInfo{
		{Key: ".armor/multipart/test-upload/part-1.json"},
		{Key: ".armor/multipart/test-upload/part-2.json"},
		{Key: ".armor/multipart/test-upload/part-3.json"},
	}
	backend := &delayedMultipartPartsBackend{
		Backend: NewNilBackend(),
		objects: objects,
		parts: map[int][]byte{
			1: []byte("{\"part_number\":1}"),
			2: []byte("not-json"),
		},
	}

	manager := NewMultipartStateManager(backend, "test-bucket")
	loaded, err := manager.ListPartsV3(context.Background(), "test-upload")
	if err != nil {
		t.Fatalf("ListPartsV3() error = %v", err)
	}
	if len(loaded) != 1 || loaded[1] == nil {
		t.Fatalf("ListPartsV3() = %#v, want only valid part 1", loaded)
	}
}
