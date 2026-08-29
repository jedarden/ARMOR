package backend

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// MultipartSidecarCache is a cache for v3 multipart sidecar data.
// It stores parsed sidecar information with prefix sums for efficient range mapping.
// Cache entries are invalidated when the object's ETag changes (object overwritten).
type MultipartSidecarCache struct {
	mu         sync.RWMutex
	entries    map[string]*MultipartSidecarEntry
	maxEntries int
	ttl        time.Duration
}

// MultipartSidecarEntry represents a cached v3 multipart sidecar.
type MultipartSidecarEntry struct {
	Sidecar      *HMACTableSidecarV3
	ETag         string                // ETag of the object when cached
	PartPrefixSums []int64            // Cumulative plaintext lengths per part: [0, len(part1), len(part1)+len(part2), ...]
	BlockPrefixSums [][]uint32        // Per-part cumulative ciphertext lengths per block
	ExpiresAt    time.Time
}

// NewMultipartSidecarCache creates a new v3 multipart sidecar cache.
func NewMultipartSidecarCache(maxEntries int, ttlSeconds int) *MultipartSidecarCache {
	return &MultipartSidecarCache{
		entries:    make(map[string]*MultipartSidecarEntry),
		maxEntries: maxEntries,
		ttl:        time.Duration(ttlSeconds) * time.Second,
	}
}

// Get retrieves cached sidecar data for an object.
// Returns nil if not cached, expired, or ETag doesn't match (object was overwritten).
func (c *MultipartSidecarCache) Get(bucket, key, etag string) *MultipartSidecarEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[cacheKey(bucket, key)]
	if !ok {
		return nil
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		return nil
	}

	// Check ETag match for cache invalidation on overwrite
	if entry.ETag != etag {
		return nil
	}

	return entry
}

// Set stores sidecar data in the cache.
func (c *MultipartSidecarCache) Set(bucket, key, etag string, sidecar *HMACTableSidecarV3) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Build prefix sums
	partPrefixSums, blockPrefixSums, err := buildPrefixSums(sidecar)
	if err != nil {
		return fmt.Errorf("failed to build prefix sums: %w", err)
	}

	// Evict if at capacity
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	c.entries[cacheKey(bucket, key)] = &MultipartSidecarEntry{
		Sidecar:        sidecar,
		ETag:           etag,
		PartPrefixSums: partPrefixSums,
		BlockPrefixSums: blockPrefixSums,
		ExpiresAt:      time.Now().Add(c.ttl),
	}

	return nil
}

// Delete removes an entry from the cache.
func (c *MultipartSidecarCache) Delete(bucket, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, cacheKey(bucket, key))
}

// Clear removes all entries from the cache.
func (c *MultipartSidecarCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*MultipartSidecarEntry)
}

// Size returns the number of entries in the cache.
func (c *MultipartSidecarCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *MultipartSidecarCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for k, v := range c.entries {
		if oldestKey == "" || v.ExpiresAt.Before(oldestTime) {
			oldestKey = k
			oldestTime = v.ExpiresAt
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
	}
}

// buildPrefixSums builds cumulative plaintext lengths per part and
// cumulative ciphertext lengths per block for each part.
// Returns:
// - partPrefixSums: [0, len(part1), len(part1)+len(part2), ...] for offset→part mapping
// - blockPrefixSums: per-part array of cumulative ciphertext lengths per block
func buildPrefixSums(sidecar *HMACTableSidecarV3) ([]int64, [][]uint32, error) {
	if len(sidecar.Parts) == 0 {
		return nil, nil, fmt.Errorf("no parts in sidecar")
	}

	// Build cumulative plaintext lengths per part
	partPrefixSums := make([]int64, len(sidecar.Parts)+1)
	partPrefixSums[0] = 0
	for i, part := range sidecar.Parts {
		partPrefixSums[i+1] = partPrefixSums[i] + part.PlaintextLen
	}

	// Build cumulative ciphertext lengths per block for each part
	blockPrefixSums := make([][]uint32, len(sidecar.Parts))
	for partIdx, part := range sidecar.Parts {
		if len(part.Blocks) == 0 {
			continue
		}

		prefixSums := make([]uint32, len(part.Blocks)+1)
		prefixSums[0] = 0

		for blockIdx, blockData := range part.Blocks {
			// blockData is [hmac_base64, clen]
			if len(blockData) < 2 {
				return nil, nil, fmt.Errorf("invalid block data at part %d block %d", part.N, blockIdx)
			}

			// Decode clen (may have compression flag in high bit)
			clen, err := parseBlockLength(blockData[1])
			if err != nil {
				return nil, nil, fmt.Errorf("failed to parse block length at part %d block %d: %w", part.N, blockIdx, err)
			}

			prefixSums[blockIdx+1] = prefixSums[blockIdx] + clen
		}

		blockPrefixSums[partIdx] = prefixSums
	}

	return partPrefixSums, blockPrefixSums, nil
}

// parseBlockLength parses the ciphertext length from a block entry.
// The length may have the compression flag in the high bit.
func parseBlockLength(clenStr string) (uint32, error) {
	// clen is stored as a base64 string of the 4-byte length
	clenBytes, err := base64.StdEncoding.DecodeString(clenStr)
	if err != nil {
		return 0, fmt.Errorf("failed to decode block length: %w", err)
	}
	if len(clenBytes) < 4 {
		return 0, fmt.Errorf("block length too short: %d bytes", len(clenBytes))
	}

	// Parse as big-endian uint32
	clen := binary.BigEndian.Uint32(clenBytes)
	// Mask out the compression flag (high bit) to get the actual length
	return clen & 0x7FFFFFFF, nil
}

// MapOffsetToPart maps a plaintext byte offset to a part number.
// Uses binary search on the cumulative plaintext lengths.
func (e *MultipartSidecarEntry) MapOffsetToPart(offset int64) (int, int64, error) {
	if offset < 0 {
		return 0, 0, fmt.Errorf("negative offset: %d", offset)
	}

	// Binary search for the part containing this offset
	// partPrefixSums[i] is the start offset of part i (1-indexed)
	// partPrefixSums[i+1] is the end offset (exclusive)
	left, right := 1, len(e.PartPrefixSums)-1
	for left <= right {
		mid := (left + right) / 2
		if offset >= e.PartPrefixSums[mid] && (mid == len(e.PartPrefixSums)-1 || offset < e.PartPrefixSums[mid+1]) {
			partIdx := mid - 1 // Convert to 0-indexed
			partStart := e.PartPrefixSums[mid]
			return partIdx, partStart, nil
		}
		if offset < e.PartPrefixSums[mid] {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	return 0, 0, fmt.Errorf("offset %d out of range (total size: %d)", offset, e.PartPrefixSums[len(e.PartPrefixSums)-1])
}

// MapOffsetToBlock maps a plaintext offset within a part to a block number and offset within that block.
// Returns (blockNumber, offsetWithinBlock, error).
func (e *MultipartSidecarEntry) MapOffsetToBlock(partIdx int, offsetInPart int64, blockSize int) (int, int64, error) {
	if partIdx < 0 || partIdx >= len(e.Sidecar.Parts) {
		return 0, 0, fmt.Errorf("invalid part index: %d", partIdx)
	}

	part := e.Sidecar.Parts[partIdx]
	if offsetInPart < 0 || offsetInPart >= part.PlaintextLen {
		return 0, 0, fmt.Errorf("offset in part %d out of range: %d (part size: %d)", partIdx, offsetInPart, part.PlaintextLen)
	}

	blockNum := int(offsetInPart / int64(blockSize))
	offsetInBlock := offsetInPart % int64(blockSize)

	if blockNum >= len(part.Blocks) {
		return 0, 0, fmt.Errorf("block number %d out of range for part %d (total blocks: %d)", blockNum, partIdx, len(part.Blocks))
	}

	return blockNum, offsetInBlock, nil
}

// GetCiphertextOffset returns the ciphertext byte offset for a specific block within a part.
// Uses the cached prefix sums to compute the offset efficiently.
func (e *MultipartSidecarEntry) GetCiphertextOffset(partIdx int, blockIdx int) (uint32, error) {
	if partIdx < 0 || partIdx >= len(e.BlockPrefixSums) {
		return 0, fmt.Errorf("invalid part index: %d", partIdx)
	}

	prefixSums := e.BlockPrefixSums[partIdx]
	if blockIdx < 0 || blockIdx >= len(prefixSums)-1 {
		return 0, fmt.Errorf("invalid block index: %d for part %d", blockIdx, partIdx)
	}

	return prefixSums[blockIdx], nil
}

// GetBlockLength returns the ciphertext length for a specific block.
func (e *MultipartSidecarEntry) GetBlockLength(partIdx int, blockIdx int) (uint32, error) {
	if partIdx < 0 || partIdx >= len(e.Sidecar.Parts) {
		return 0, fmt.Errorf("invalid part index: %d", partIdx)
	}

	part := e.Sidecar.Parts[partIdx]
	if blockIdx < 0 || blockIdx >= len(part.Blocks) {
		return 0, fmt.Errorf("invalid block index: %d for part %d", blockIdx, partIdx)
	}

	blockData := part.Blocks[blockIdx]
	if len(blockData) < 2 {
		return 0, fmt.Errorf("invalid block data at part %d block %d", part.N, blockIdx)
	}

	return parseBlockLength(blockData[1])
}

// GetBlockHMAC returns the HMAC for a specific block.
func (e *MultipartSidecarEntry) GetBlockHMAC(partIdx int, blockIdx int) ([]byte, error) {
	if partIdx < 0 || partIdx >= len(e.Sidecar.Parts) {
		return nil, fmt.Errorf("invalid part index: %d", partIdx)
	}

	part := e.Sidecar.Parts[partIdx]
	if blockIdx < 0 || blockIdx >= len(part.Blocks) {
		return nil, fmt.Errorf("invalid block index: %d for part %d", blockIdx, partIdx)
	}

	blockData := part.Blocks[blockIdx]
	if len(blockData) < 1 {
		return nil, fmt.Errorf("invalid block HMAC at part %d block %d", part.N, blockIdx)
	}

	hmacBytes, err := base64.StdEncoding.DecodeString(blockData[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode block HMAC: %w", err)
	}

	return hmacBytes, nil
}

// TotalPlaintextSize returns the total plaintext size across all parts.
func (e *MultipartSidecarEntry) TotalPlaintextSize() int64 {
	if len(e.PartPrefixSums) == 0 {
		return 0
	}
	return e.PartPrefixSums[len(e.PartPrefixSums)-1]
}

// PartCount returns the number of parts in the sidecar.
func (e *MultipartSidecarEntry) PartCount() int {
	return len(e.Sidecar.Parts)
}

// BlockCount returns the total number of blocks across all parts.
func (e *MultipartSidecarEntry) BlockCount() int {
	total := 0
	for _, part := range e.Sidecar.Parts {
		total += len(part.Blocks)
	}
	return total
}

// IsBlockCompressed reports whether a block is zstd-compressed.
func (e *MultipartSidecarEntry) IsBlockCompressed(partIdx int, blockIdx int) (bool, error) {
	if partIdx < 0 || partIdx >= len(e.Sidecar.Parts) {
		return false, fmt.Errorf("invalid part index: %d", partIdx)
	}

	part := e.Sidecar.Parts[partIdx]
	if blockIdx < 0 || blockIdx >= len(part.Blocks) {
		return false, fmt.Errorf("invalid block index: %d for part %d", blockIdx, partIdx)
	}

	blockData := part.Blocks[blockIdx]
	if len(blockData) < 2 {
		return false, fmt.Errorf("invalid block data at part %d block %d", part.N, blockIdx)
	}

	clenBytes, err := base64.StdEncoding.DecodeString(blockData[1])
	if err != nil {
		return false, fmt.Errorf("failed to decode block length: %w", err)
	}
	if len(clenBytes) < 4 {
		return false, fmt.Errorf("block length too short: %d bytes", len(clenBytes))
	}

	clen := binary.BigEndian.Uint32(clenBytes)
	// High bit indicates compression
	return (clen & 0x80000000) != 0, nil
}

// GetSidecarKey returns the B2 key for the sidecar object.
func GetSidecarKey(key string) string {
	keyHash := sha256.Sum256([]byte(key))
	return fmt.Sprintf(".armor/hmac/%x", keyHash)
}
