package backend

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// defaultSidecarCacheMaxBytes is the total-bytes backstop for the sidecar
// cache (armor-a4e5356d), matching the v3 block-table cache's byte bound: a
// 5GiB object at 64KiB blocks carries a sidecar in the MiB range, so an
// entry bound alone would not bound memory.
const defaultSidecarCacheMaxBytes = 64 << 20

// MultipartSidecarCache is a cache for v3 multipart sidecar data.
// It stores parsed sidecar information with prefix sums for efficient range mapping.
// Cache entries are invalidated when the object's ETag changes (object overwritten).
// Bounded by entries and total bytes via the shared lruCache.
type MultipartSidecarCache struct {
	lru *lruCache[*MultipartSidecarEntry]
}

// MultipartSidecarEntry represents a cached v3 multipart sidecar.
type MultipartSidecarEntry struct {
	Sidecar         *HMACTableSidecarV3
	ETag            string  // ETag of the object when cached
	PartPrefixSums  []int64 // Cumulative plaintext lengths per part: [0, len(part1), len(part1)+len(part2), ...]
	BlockPrefixSums [][]int64
	// Per-part cumulative ciphertext lengths per block. int64, not uint32:
	// B2 allows parts up to 5GiB, so per-part ciphertext can exceed 2^32.
	// Cumulative ciphertext offsets per part in the concatenated stored
	// object: [0, len(part1), len(part1)+len(part2), ...], one entry per
	// part plus the end-exclusive total. Built with checked addition — a
	// sidecar whose offsets overflow int64 is rejected at load instead of
	// silently addressing the wrong bytes (armor-817d9d92 class).
	CiphertextPrefixSums []int64
}

// sidecarEntryBytes estimates one cached entry's memory cost: the prefix-sum
// slices (8 bytes per element), the sidecar's own parts and base64 block
// entries (the cache holds the only live reference between requests), and
// per-slice overhead. An estimate, not an exact accounting — see
// metadataEntryBytes.
func sidecarEntryBytes(e *MultipartSidecarEntry) int64 {
	n := int64(96 + 24*2) // entry struct + the three slice headers
	n += int64(len(e.PartPrefixSums)+len(e.CiphertextPrefixSums)) * 8
	for _, blocks := range e.BlockPrefixSums {
		n += 24 + int64(len(blocks))*8
	}
	if e.Sidecar == nil {
		return n
	}
	for _, part := range e.Sidecar.Parts {
		n += 64
		for _, block := range part.Blocks {
			for _, s := range block {
				n += int64(len(s))
			}
			n += 32 // slice header + two string headers
		}
	}
	return n
}

// NewMultipartSidecarCache creates a new v3 multipart sidecar cache.
// maxBytes, when given, overrides the total-bytes backstop; 0 or negative
// selects the default.
func NewMultipartSidecarCache(maxEntries int, ttlSeconds int, maxBytes ...int64) *MultipartSidecarCache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	budget := int64(defaultSidecarCacheMaxBytes)
	if len(maxBytes) > 0 && maxBytes[0] > 0 {
		budget = maxBytes[0]
	}
	return &MultipartSidecarCache{
		lru: newLRUCache[*MultipartSidecarEntry](cacheNameSidecar, maxEntries, budget,
			time.Duration(ttlSeconds)*time.Second, sidecarEntryBytes, nil),
	}
}

// WithMetrics wires the metrics sink for the shared LRU counters.
func (c *MultipartSidecarCache) WithMetrics(m *metrics.Metrics) *MultipartSidecarCache {
	if c == nil {
		return c
	}
	c.lru.setMetricsSink(m)
	return c
}

// Stats snapshots the cache's cumulative hit/miss/eviction counters.
func (c *MultipartSidecarCache) Stats() CacheStats {
	if c == nil {
		return CacheStats{}
	}
	return c.lru.stats()
}

// Get retrieves cached sidecar data for an object.
// Returns nil if not cached, expired, or ETag doesn't match (object was overwritten).
func (c *MultipartSidecarCache) Get(bucket, key, etag string) *MultipartSidecarEntry {
	entry, ok := c.lru.get(cacheKey(bucket, key), func(e *MultipartSidecarEntry) bool {
		// ETag coherence: the object may have been overwritten since this
		// entry was written.
		return e.ETag == etag
	})
	if !ok {
		return nil
	}
	return entry
}

// Set stores sidecar data in the cache.
func (c *MultipartSidecarCache) Set(bucket, key, etag string, sidecar *HMACTableSidecarV3) error {
	// Build prefix sums
	partPrefixSums, blockPrefixSums, ciphertextPrefixSums, err := buildPrefixSums(sidecar)
	if err != nil {
		return fmt.Errorf("failed to build prefix sums: %w", err)
	}

	c.lru.set(cacheKey(bucket, key), &MultipartSidecarEntry{
		Sidecar:              sidecar,
		ETag:                 etag,
		PartPrefixSums:       partPrefixSums,
		BlockPrefixSums:      blockPrefixSums,
		CiphertextPrefixSums: ciphertextPrefixSums,
	})

	return nil
}

// Delete removes an entry from the cache.
func (c *MultipartSidecarCache) Delete(bucket, key string) {
	c.lru.remove(cacheKey(bucket, key))
}

// Clear removes all entries from the cache.
func (c *MultipartSidecarCache) Clear() {
	c.lru.clear()
}

// Size returns the number of entries in the cache.
func (c *MultipartSidecarCache) Size() int {
	n, _ := c.lru.size()
	return n
}

// buildPrefixSums builds cumulative plaintext lengths per part, cumulative
// ciphertext lengths per block for each part, and cumulative ciphertext
// offsets per part in the concatenated stored object.
// Returns:
// - partPrefixSums: [0, len(part1), len(part1)+len(part2), ...] for offset→part mapping
// - blockPrefixSums: per-part array of cumulative ciphertext lengths per block
// - ciphertextPrefixSums: [0, off(part2), off(part3), ..., total] addressing the stored object
func buildPrefixSums(sidecar *HMACTableSidecarV3) ([]int64, [][]int64, []int64, error) {
	if len(sidecar.Parts) == 0 {
		return nil, nil, nil, fmt.Errorf("no parts in sidecar")
	}

	// Build cumulative plaintext lengths per part
	partPrefixSums := make([]int64, len(sidecar.Parts)+1)
	partPrefixSums[0] = 0
	for i, part := range sidecar.Parts {
		partPrefixSums[i+1] = partPrefixSums[i] + part.PlaintextLen
	}

	// Build cumulative ciphertext offsets per part with checked int64
	// addition: a wrap or a negative length would address the wrong bytes
	// of the stored object, so the sidecar is rejected outright.
	ciphertextPrefixSums := make([]int64, len(sidecar.Parts)+1)
	ciphertextPrefixSums[0] = 0
	for i, part := range sidecar.Parts {
		if part.CiphertextLen < 0 {
			return nil, nil, nil, fmt.Errorf("negative ciphertext length %d for part %d", part.CiphertextLen, part.N)
		}
		next := ciphertextPrefixSums[i] + part.CiphertextLen
		if next < ciphertextPrefixSums[i] {
			return nil, nil, nil, fmt.Errorf("cumulative ciphertext offset overflows int64 at part %d", part.N)
		}
		ciphertextPrefixSums[i+1] = next
	}

	// Build cumulative ciphertext lengths per block for each part
	blockPrefixSums := make([][]int64, len(sidecar.Parts))
	for partIdx, part := range sidecar.Parts {
		if len(part.Blocks) == 0 {
			if part.CiphertextLen != 0 {
				return nil, nil, nil, fmt.Errorf("part %d declares %d ciphertext bytes but has no block entries", part.N, part.CiphertextLen)
			}
			continue
		}

		prefixSums := make([]int64, len(part.Blocks)+1)
		prefixSums[0] = 0

		for blockIdx, blockData := range part.Blocks {
			// blockData is [hmac_base64, clen]
			if len(blockData) < 2 {
				return nil, nil, nil, fmt.Errorf("invalid block data at part %d block %d", part.N, blockIdx)
			}

			// Decode clen (may have compression flag in high bit)
			clen, err := parseBlockLength(blockData[1])
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to parse block length at part %d block %d: %w", part.N, blockIdx, err)
			}

			prefixSums[blockIdx+1] = prefixSums[blockIdx] + int64(clen)
		}

		// The block entries describe the part's full ciphertext layout: if
		// they disagree with the declared part length, the stored bytes a
		// block range addresses are not the bytes the HMACs cover.
		if prefixSums[len(prefixSums)-1] != part.CiphertextLen {
			return nil, nil, nil, fmt.Errorf("part %d block lengths sum to %d, sidecar declares %d", part.N, prefixSums[len(prefixSums)-1], part.CiphertextLen)
		}

		blockPrefixSums[partIdx] = prefixSums
	}

	return partPrefixSums, blockPrefixSums, ciphertextPrefixSums, nil
}

// NewMultipartSidecarEntry builds a MultipartSidecarEntry for an already
// loaded v3 sidecar, without going through the cache. It is the entry point
// for readers that verify or decrypt whole objects themselves (the
// restore-verifier, the offline verifier): it yields the same parsed view —
// prefix sums plus the per-block accessors handleV3MultipartGet uses — that
// cache.Set builds for the server's own GET path.
//
// A sidecar whose block entries do not parse is an error here, never a
// partial table: skipping a malformed entry silently would decrypt to a
// truncated plaintext with no signal (armor-86a90341).
func NewMultipartSidecarEntry(sidecar *HMACTableSidecarV3) (*MultipartSidecarEntry, error) {
	partPrefixSums, blockPrefixSums, ciphertextPrefixSums, err := buildPrefixSums(sidecar)
	if err != nil {
		return nil, err
	}
	return &MultipartSidecarEntry{
		Sidecar:              sidecar,
		PartPrefixSums:       partPrefixSums,
		BlockPrefixSums:      blockPrefixSums,
		CiphertextPrefixSums: ciphertextPrefixSums,
	}, nil
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
func (e *MultipartSidecarEntry) GetCiphertextOffset(partIdx int, blockIdx int) (int64, error) {
	if partIdx < 0 || partIdx >= len(e.BlockPrefixSums) {
		return 0, fmt.Errorf("invalid part index: %d", partIdx)
	}

	prefixSums := e.BlockPrefixSums[partIdx]
	if blockIdx < 0 || blockIdx >= len(prefixSums)-1 {
		return 0, fmt.Errorf("invalid block index: %d for part %d", blockIdx, partIdx)
	}

	return prefixSums[blockIdx], nil
}

// GetPartCiphertextOffset returns the byte offset of a part's ciphertext in
// the concatenated stored object. The sums were built once at sidecar load
// with checked int64 addition, so readers never re-accumulate per-part
// lengths (the repeated accumulation armor-817d9d92 corrected) and an
// out-of-range part is an error rather than a wrapped address. partIdx may
// be PartCount() to read the end-exclusive total.
func (e *MultipartSidecarEntry) GetPartCiphertextOffset(partIdx int) (int64, error) {
	if partIdx < 0 || partIdx >= len(e.CiphertextPrefixSums) {
		return 0, fmt.Errorf("invalid part index: %d (sidecar has %d parts)", partIdx, e.PartCount())
	}
	return e.CiphertextPrefixSums[partIdx], nil
}

// TotalCiphertextSize returns the total ciphertext size across all parts,
// as declared by the sidecar's checked per-part cumulative offsets.
func (e *MultipartSidecarEntry) TotalCiphertextSize() (int64, error) {
	return e.GetPartCiphertextOffset(e.PartCount())
}

// GetCiphertextSpan returns the half-open ciphertext byte span [start, end)
// within a part covered by the inclusive block range startBlock..endBlock.
// Blocks within a part are contiguous in the stored ciphertext, so a span can
// be fetched with a single ranged request. The offsets are relative to the
// start of the part; add the part's offset in the concatenated object to
// address the stored object.
func (e *MultipartSidecarEntry) GetCiphertextSpan(partIdx, startBlock, endBlock int) (int64, int64, error) {
	if partIdx < 0 || partIdx >= len(e.BlockPrefixSums) {
		return 0, 0, fmt.Errorf("invalid part index: %d", partIdx)
	}

	prefixSums := e.BlockPrefixSums[partIdx]
	if startBlock < 0 || endBlock < startBlock || endBlock >= len(prefixSums)-1 {
		return 0, 0, fmt.Errorf("invalid block range: %d-%d for part %d (%d blocks)", startBlock, endBlock, partIdx, len(prefixSums)-1)
	}

	return prefixSums[startBlock], prefixSums[endBlock+1], nil
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

// GetSidecarKey returns the B2 key for a multipart HMAC sidecar in its
// ADR-003 addendum location: <keyPrefix>.armor/hmac/<sha256(keyPrefix+key)>.
// key is the CLIENT key (the name the sidecar is written under at
// CompleteMultipartUpload), and keyPrefix must be normalized exactly as
// config.normalizePrefix produces — empty, or ending in exactly one slash.
// An empty prefix yields the bucket-root .armor/hmac/<sha256(key)> location
// every ARMOR before 2026-09-20 wrote, so unprefixed deployments are
// byte-for-byte unchanged.
//
// The hash input includes the prefix on purpose. Sidecars are named by the
// client key, and two tenants sharing one bucket can hold the same client
// key; hashing the bare key let their sidecars clobber each other (a
// wrong-tenant read then fails HMAC verification closed, but one tenant's
// table is still destroyed by the other's write). Hashing keyPrefix+key
// gives every tenant a distinct sidecar name.
func GetSidecarKey(keyPrefix, key string) string {
	keyHash := sha256.Sum256([]byte(keyPrefix + key))
	return keyPrefix + internalNamespace + "hmac/" + hex.EncodeToString(keyHash[:])
}

// SidecarLocations returns every B2 key a multipart HMAC sidecar for key may
// live at, in the order readers must probe them: the composed location first,
// then the pre-2026-09-20 bucket-root location. The root fallback stays live
// while a prefix is set because prefixed deployments accumulated root
// sidecars before the composed location existed, and a bucket that gained its
// prefix later still holds its pre-prefix sidecars there — the same
// both-branches rule isInternalKey applies to listings. Without a prefix the
// two locations coincide and exactly one is returned.
func SidecarLocations(keyPrefix, key string) []string {
	composed := GetSidecarKey(keyPrefix, key)
	root := GetSidecarKey("", key)
	if composed == root {
		return []string{composed}
	}
	return []string{composed, root}
}
