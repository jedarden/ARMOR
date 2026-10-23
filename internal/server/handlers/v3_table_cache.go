package handlers

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// v3TableCacheMetrics are the cumulative counters for the v3 block-table
// cache. They are read with atomics; Snapshot is safe alongside any load.
type v3TableCacheMetrics struct {
	Hits       uint64 // lookups served from a cached decoded table
	Misses     uint64 // lookups that had to fetch and decode the trailer
	Coalesced  uint64 // waiters that rode an in-flight load instead of starting one
	Loads      uint64 // trailer fetches issued to the backend (one per miss generation)
	LoadBytes  uint64 // trailer bytes downloaded across those fetches
	Evictions  uint64 // entries dropped by the entry/byte bounds
}

// v3TableKeyParts is the stored-object generation a block table belongs to.
// The cache key is derived from all of these, not from the plaintext ETag
// alone: a same-plaintext re-encryption or a same-ETag overwrite changes at
// least one of them, and serving a table decoded from one stored object
// against a different stored object would fail block authentication at best.
// VersionID is the strongest B2 signal; StoredSize and the second-granularity
// LastModified keep the key honest against backends (and test doubles) whose
// ETag does not change when the stored bytes do.
type v3TableKeyParts struct {
	VersionID    string
	ETag         string
	StoredSize   int64
	LastModified time.Time
}

// key renders the cache key: tenant/prefix-addressed stored key plus every
// generation signal. Unset fields simply render empty; the tuple as a whole
// is what must match.
func (p v3TableKeyParts) key(bucket, storedKey string) string {
	return bucket + "/" + storedKey + "@" + p.VersionID + "|" + p.ETag + "|" +
		fmt.Sprint(p.StoredSize) + "|" + fmt.Sprint(p.LastModified.Unix())
}

// generationFromInfo extracts the cache-key generation from an authoritative
// stored-object HEAD.
func generationFromInfo(info *backend.ObjectInfo) v3TableKeyParts {
	if info == nil {
		return v3TableKeyParts{}
	}
	return v3TableKeyParts{
		VersionID:    info.VersionID,
		ETag:         info.ETag,
		StoredSize:   info.StoredSize,
		LastModified: info.LastModified,
	}
}

// v3TableCacheEntry is one decoded block table plus its byte cost.
type v3TableCacheEntry struct {
	table     *crypto.BlockTable
	bytes     int64
	expiresAt time.Time
}

// v3TableLoad is one in-flight trailer fetch. Concurrent misses for the same
// key wait on done and share the result instead of each fetching the trailer.
type v3TableLoad struct {
	done  chan struct{}
	table *crypto.BlockTable
	err   error
}

// v3BlockTableCache caches decoded v3 trailer block tables, bounded by entry
// count, TTL, and total cached bytes. Loads are coalesced per key. Entries are
// keyed by stored-object generation, so an overwrite, re-encryption, or
// migration simply misses and repopulates — no explicit invalidation and no
// chance of serving a table against ciphertext it does not describe.
type v3BlockTableCache struct {
	mu         sync.Mutex
	entries    map[string]*v3TableCacheEntry
	inflight   map[string]*v3TableLoad
	bytes      int64
	maxEntries int
	maxBytes   int64
	ttl        time.Duration
	hits       atomic.Uint64
	misses     atomic.Uint64
	coalesced  atomic.Uint64
	loads      atomic.Uint64
	loadBytes  atomic.Uint64
	evictions  atomic.Uint64
}

// defaultV3TableCacheMaxBytes bounds the decoded-table cache by bytes
// independently of the entry-count bound (a 5GiB object at 64KiB blocks
// carries a ~2.9MiB table, so a small entry bound alone would not bound
// memory). Overridable in tests via newV3BlockTableCache.
const defaultV3TableCacheMaxBytes = 64 << 20

func newV3BlockTableCache(maxEntries int, ttlSeconds int, maxBytes int64) *v3BlockTableCache {
	if maxEntries <= 0 {
		maxEntries = 1000
	}
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	if maxBytes <= 0 {
		maxBytes = defaultV3TableCacheMaxBytes
	}
	return &v3BlockTableCache{
		entries:    make(map[string]*v3TableCacheEntry),
		inflight:   make(map[string]*v3TableLoad),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		ttl:        time.Duration(ttlSeconds) * time.Second,
	}
}

// get returns a live cached table, if any.
func (c *v3BlockTableCache) get(key string) (*crypto.BlockTable, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.table, true
}

// fetchV3BlockTable downloads and decodes the trailer block table. This is
// the exact body of the former uncached readV3BlockTable, factored out so the
// cache and the fallback path share one implementation.
func (h *Handlers) fetchV3BlockTable(ctx context.Context, bucket, key string, trailerSize, trailerOffset int64, blockCount int) (*crypto.BlockTable, error) {
	trailerBody, err := h.backend.GetRange(ctx, bucket, key, trailerOffset, trailerSize)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch v3 trailer block table: %w", err)
	}
	defer trailerBody.Close()

	trailerData, err := io.ReadAll(trailerBody)
	if err != nil {
		return nil, fmt.Errorf("failed to read v3 trailer block table: %w", err)
	}
	if h.v3Tables != nil {
		h.v3Tables.loads.Add(1)
		h.v3Tables.loadBytes.Add(uint64(len(trailerData)))
	}

	// Decode the block table. The block size is only used for per-entry
	// validation here; each entry carries its own length.
	const defaultBlockSize = 65536
	table, err := crypto.DecodeBlockTable(trailerData, defaultBlockSize, uint32(blockCount))
	if err != nil {
		return nil, fmt.Errorf("failed to decode v3 block table: %w", err)
	}
	return table, nil
}

// loadV3BlockTable returns the decoded block table for a v3 stored object,
// going through the generation-keyed cache. gen must come from the
// authoritative HEAD the current request already performed (GetObject's
// freshness head, or the legacy-path head) — never from a previous request's
// view of the object. An empty-generation key would collapse distinct stored
// objects onto one entry, so it bypasses the cache and fetches directly.
func (h *Handlers) loadV3BlockTable(ctx context.Context, bucket, storedKey string, gen v3TableKeyParts, ciphertextSize int64, blockCount int) (*crypto.BlockTable, error) {
	trailerOffset, trailerSize, err := v3BlockTableTrailerWindow(ciphertextSize, blockCount)
	if err != nil {
		return nil, err
	}

	cache := h.v3Tables
	if cache == nil || blockCount <= 0 {
		return h.fetchV3BlockTable(ctx, bucket, storedKey, trailerSize, trailerOffset, blockCount)
	}
	key := gen.key(bucket, storedKey)
	if gen == (v3TableKeyParts{}) {
		// No generation signals at all: refuse to cache, but still serve.
		return h.fetchV3BlockTable(ctx, bucket, storedKey, trailerSize, trailerOffset, blockCount)
	}

	if table, ok := cache.get(key); ok {
		cache.hits.Add(1)
		return table, nil
	}
	cache.misses.Add(1)

	c := cache
	c.mu.Lock()
	// Re-check under the lock: a racing request may have landed the entry.
	if entry, ok := c.entries[key]; ok && time.Now().Before(entry.expiresAt) {
		c.mu.Unlock()
		c.hits.Add(1)
		return entry.table, nil
	}
	if load, ok := c.inflight[key]; ok {
		// Another request is already fetching this generation's trailer.
		c.mu.Unlock()
		c.coalesced.Add(1)
		<-load.done
		return load.table, load.err
	}
	load := &v3TableLoad{done: make(chan struct{})}
	c.inflight[key] = load
	c.mu.Unlock()

	table, err := h.fetchV3BlockTable(ctx, bucket, storedKey, trailerSize, trailerOffset, blockCount)

	c.mu.Lock()
	delete(c.inflight, key)
	if err == nil {
		load.table = table
		c.storeLocked(key, table)
	}
	c.mu.Unlock()
	close(load.done)
	return table, err
}

// storeLocked inserts an entry and enforces the entry/byte bounds.
// c.mu must be held.
func (c *v3BlockTableCache) storeLocked(key string, table *crypto.BlockTable) {
	entryBytes := int64(table.EntryCount()) * crypto.BlockTableEntrySize
	if entryBytes > c.maxBytes {
		// A single table larger than the whole cache must not evict
		// everything else for one entry that will be dropped next store.
		c.evictions.Add(1)
		return
	}
	c.entries[key] = &v3TableCacheEntry{
		table:     table,
		bytes:     entryBytes,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.bytes += entryBytes
	c.evictLocked()
}

// evictLocked drops expired entries first, then oldest-expiry entries, until
// both bounds hold. c.mu must be held.
func (c *v3BlockTableCache) evictLocked() {
	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, key)
			c.bytes -= entry.bytes
			c.evictions.Add(1)
		}
	}
	for (len(c.entries) > c.maxEntries || c.bytes > c.maxBytes) && len(c.entries) > 0 {
		var oldestKey string
		var oldest time.Time
		for k, v := range c.entries {
			if oldestKey == "" || v.expiresAt.Before(oldest) {
				oldestKey = k
				oldest = v.expiresAt
			}
		}
		c.bytes -= c.entries[oldestKey].bytes
		delete(c.entries, oldestKey)
		c.evictions.Add(1)
	}
}

// metrics returns a snapshot of the cumulative counters.
func (c *v3BlockTableCache) metrics() v3TableCacheMetrics {
	return v3TableCacheMetrics{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Coalesced: c.coalesced.Load(),
		Loads:     c.loads.Load(),
		LoadBytes: c.loadBytes.Load(),
		Evictions: c.evictions.Load(),
	}
}

// size returns the number of live entries and cached bytes.
func (c *v3BlockTableCache) size() (int, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries), c.bytes
}
