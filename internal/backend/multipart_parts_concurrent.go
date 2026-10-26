package backend

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

const multipartPartReadConcurrency = 16

// ListPartsV3 lists all v3 part records with bounded concurrent reads.
func (m *MultipartStateManager) ListPartsV3(ctx context.Context, uploadID string) (map[int]*PartDataV3, error) {
	return m.listPartsV3Concurrent(ctx, uploadID)
}

// listPartsV3Concurrent loads the durable per-part records with bounded
// concurrency. The existing LoadPartV3 method remains the source of truth for
// decoding and error handling, so missing or malformed records retain the
// existing skip semantics.
func (m *MultipartStateManager) listPartsV3Concurrent(ctx context.Context, uploadID string) (map[int]*PartDataV3, error) {
	prefix := m.internalPrefix() + "multipart/" + uploadID + "/part-"
	result, err := m.backend.ListRaw(ctx, m.bucket, prefix, "", "", 1000)
	if err != nil {
		return nil, fmt.Errorf("failed to list parts: %w", err)
	}

	partNumbers := make([]int, 0, len(result.Objects))
	appendPartNumbers := func(objects []ObjectInfo) {
		for _, object := range objects {
			if !strings.HasPrefix(object.Key, prefix) || !strings.HasSuffix(object.Key, ".json") {
				continue
			}
			partNumber, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(object.Key, prefix), ".json"))
			if err != nil || partNumber <= 0 {
				continue
			}
			partNumbers = append(partNumbers, partNumber)
		}
	}
	appendPartNumbers(result.Objects)

	// Preserve the existing partial-result behavior if a later page cannot be
	// listed: the first page is still eligible for loading, just as it was in
	// the serial implementation.
	for result.IsTruncated {
		result, err = m.backend.ListRaw(ctx, m.bucket, prefix, "", result.NextToken, 1000)
		if err != nil {
			break
		}
		appendPartNumbers(result.Objects)
	}

	parts := make(map[int]*PartDataV3, len(partNumbers))
	if len(partNumbers) == 0 {
		return parts, nil
	}

	workerCount := multipartPartReadConcurrency
	if len(partNumbers) < workerCount {
		workerCount = len(partNumbers)
	}

	type loadedPart struct {
		partNumber int
		part       *PartDataV3
	}
	jobs := make(chan int, len(partNumbers))
	loaded := make(chan loadedPart, len(partNumbers))
	var workers sync.WaitGroup
	workers.Add(workerCount)

	for _, partNumber := range partNumbers {
		jobs <- partNumber
	}
	close(jobs)

	for i := 0; i < workerCount; i++ {
		go func() {
			defer workers.Done()
			for partNumber := range jobs {
				part, loadErr := m.LoadPartV3(ctx, uploadID, partNumber)
				if loadErr != nil {
					part = nil
				}
				loaded <- loadedPart{partNumber: partNumber, part: part}
			}
		}()
	}

	workers.Wait()
	close(loaded)
	for item := range loaded {
		if item.part != nil {
			parts[item.partNumber] = item.part
		}
	}

	return parts, nil
}
