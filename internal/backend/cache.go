package backend

import (
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// Byte backstops for the entry-count bounds. ARMOR_CACHE_MAX_ENTRIES /
// ARMOR_LIST_CACHE_MAX_ENTRIES bound entries; these bounds total bytes so a
// workload of unusually large entries cannot grow the cache past its memory
// budget even while under the entry count (armor-a4e5356d). Overrides are
// for tests; production always uses the defaults.
const (
	defaultMetadataCacheMaxBytes = 64 << 20  // entries are ~0.5-1 KiB each
	defaultListCacheMaxBytes     = 128 << 20 // pages of up to 1000 keys each
)

// CacheEntry represents a cached metadata entry.
type CacheEntry struct {
	Metadata *ARMORMetadata
	// Version binds the entry to one stored-object generation — the backend's
	// content identity (ETag) observed when the entry was written. A versioned
	// Get only serves the entry while the stored generation still matches, so
	// an overwrite (even one that keeps the plaintext ETag, e.g. a
	// re-encryption of the same content) cannot serve stale sizes.
	Version string
}

// metadataEntryBytes estimates one cached entry's memory cost: the metadata
// struct's scalar fields plus every variable-length member and the version
// string. An estimate, not an exact accounting — it must merely grow with
// the entry's real footprint so the byte bound tracks memory pressure.
func metadataEntryBytes(e *CacheEntry) int64 {
	const scalarBytes = int64(160) // struct fields, pointers, map/slice headers
	if e == nil {
		return scalarBytes
	}
	n := scalarBytes + int64(len(e.Version)) + 32 // entry struct + key reference
	if e.Metadata == nil {
		return n
	}
	m := e.Metadata
	n += int64(len(m.ContentType) + len(m.MEKFingerprint) + len(m.PlaintextSHA) +
		len(m.ETag) + len(m.KeyID) + len(m.CiphertextRef) + len(m.CompletedAt))
	n += int64(len(m.IV) + len(m.WrappedDEK))
	return n
}

// MetadataCache is an LRU cache for object metadata, bounded by entries
// (ARMOR_CACHE_MAX_ENTRIES) and total bytes, via the shared lruCache. Every
// method is safe on a nil receiver: tests (and some companion binaries)
// build handler stacks without a metadata cache, and the retirement/warm
// calls on the PUT, HEAD, GET and LIST paths must be no-ops there rather
// than panics.
type MetadataCache struct {
	lru  *lruCache[*CacheEntry]
	sink *metrics.Metrics // mirrors hits/misses into the legacy metadata_cache_* pair
}

// NewMetadataCache creates a new metadata cache. maxBytes, when given,
// overrides the total-bytes backstop; 0 or negative selects the default.
func NewMetadataCache(maxEntries int, ttlSeconds int, maxBytes ...int64) *MetadataCache {
	budget := int64(defaultMetadataCacheMaxBytes)
	if len(maxBytes) > 0 && maxBytes[0] > 0 {
		budget = maxBytes[0]
	}
	return &MetadataCache{
		lru: newLRUCache[*CacheEntry](cacheNameMetadata, maxEntries, budget,
			time.Duration(ttlSeconds)*time.Second, metadataEntryBytes, nil),
	}
}

// WithMetrics wires the metrics sink: the shared LRU counters publish as
// armor_cache_lookups_total / armor_cache_evictions_total, and this cache
// additionally mirrors its hits/misses into armor_metadata_cache_hits_total
// and armor_metadata_cache_misses_total (the pair the dashboard displays).
func (c *MetadataCache) WithMetrics(m *metrics.Metrics) *MetadataCache {
	if c == nil {
		return c
	}
	c.sink = m
	c.lru.setMetricsSink(m)
	return c
}

// Stats snapshots the cache's cumulative hit/miss/eviction counters.
func (c *MetadataCache) Stats() CacheStats {
	if c == nil {
		return CacheStats{}
	}
	return c.lru.stats()
}

// cacheKey generates a cache key from bucket and object key.
func cacheKey(bucket, key string) string {
	return bucket + "/" + key
}

// Get retrieves cached metadata. Only unversioned entries (written by Set)
// are served; version-bound entries require GetVersioned with the generation
// the caller observed.
func (c *MetadataCache) Get(bucket, key string) (*ARMORMetadata, bool) {
	return c.getVersion(bucket, key, "")
}

// GetVersioned retrieves cached metadata bound to a stored-object generation.
// The entry is served only while version equals the one SetVersioned recorded,
// so a same-key overwrite misses instead of serving the previous object's
// plaintext size.
func (c *MetadataCache) GetVersioned(bucket, key, version string) (*ARMORMetadata, bool) {
	return c.getVersion(bucket, key, version)
}

func (c *MetadataCache) getVersion(bucket, key, version string) (*ARMORMetadata, bool) {
	if c == nil {
		return nil, false
	}
	entry, ok := c.lru.get(cacheKey(bucket, key), func(e *CacheEntry) bool {
		return e.Version == version
	})
	if c.sink != nil {
		if ok {
			c.sink.IncCacheHits()
		} else {
			c.sink.IncCacheMisses()
		}
	}
	if !ok {
		return nil, false
	}
	return entry.Metadata, true
}

// Set stores metadata in the cache (unversioned: served to any Get until it
// expires or is deleted).
func (c *MetadataCache) Set(bucket, key string, meta *ARMORMetadata) {
	c.setVersion(bucket, key, "", meta)
}

// SetVersioned stores metadata bound to a stored-object generation. Callers
// pass the backend's content identity for the stored object they just
// observed (its ETag), never a plaintext-only value: a re-encryption of the
// same plaintext must not collide with the previous generation.
func (c *MetadataCache) SetVersioned(bucket, key, version string, meta *ARMORMetadata) {
	c.setVersion(bucket, key, version, meta)
}

func (c *MetadataCache) setVersion(bucket, key, version string, meta *ARMORMetadata) {
	if c == nil {
		return
	}
	c.lru.set(cacheKey(bucket, key), &CacheEntry{
		Metadata: meta,
		Version:  version,
	})
}

// Delete removes an entry from the cache.
func (c *MetadataCache) Delete(bucket, key string) {
	if c == nil {
		return
	}
	c.lru.remove(cacheKey(bucket, key))
}

// Clear removes all entries from the cache.
func (c *MetadataCache) Clear() {
	if c == nil {
		return
	}
	c.lru.clear()
}

// Size returns the number of entries in the cache.
func (c *MetadataCache) Size() int {
	if c == nil {
		return 0
	}
	n, _ := c.lru.size()
	return n
}

// ListCacheEntry is a cached list result.
type ListCacheEntry struct {
	Result *ListResult
	Prefix string
}

// listEntryBytes estimates one cached page's memory cost: every listed
// object's key and variable-length fields plus the common prefixes and the
// continuation token. An estimate, not an exact accounting — see
// metadataEntryBytes.
func listEntryBytes(e *ListCacheEntry) int64 {
	const scalarBytes = int64(96) // entry struct, result struct, slice headers
	if e == nil {
		return scalarBytes
	}
	n := scalarBytes + int64(len(e.Prefix))
	if e.Result == nil {
		return n
	}
	n += int64(len(e.Result.NextToken))
	for _, o := range e.Result.Objects {
		n += 128 + int64(len(o.Key)+len(o.ContentType)+len(o.ETag)+len(o.VersionID))
		for k, v := range o.Metadata {
			n += int64(len(k) + len(v))
		}
	}
	for _, p := range e.Result.CommonPrefixes {
		n += int64(len(p)) + 8
	}
	return n
}

// ListCache caches ListObjectsV2 results keyed by query parameters, bounded
// by entries and total bytes via the shared lruCache.
type ListCache struct {
	lru *lruCache[*ListCacheEntry]
}

// NewListCache creates a new list cache. Returns nil when ttlSeconds <= 0
// (disabled). maxBytes, when given, overrides the total-bytes backstop.
func NewListCache(maxEntries int, ttlSeconds int, maxBytes ...int64) *ListCache {
	if ttlSeconds <= 0 {
		return nil
	}
	budget := int64(defaultListCacheMaxBytes)
	if len(maxBytes) > 0 && maxBytes[0] > 0 {
		budget = maxBytes[0]
	}
	return &ListCache{
		lru: newLRUCache[*ListCacheEntry](cacheNameList, maxEntries, budget,
			time.Duration(ttlSeconds)*time.Second, listEntryBytes, nil),
	}
}

// WithMetrics wires the metrics sink for the shared LRU counters.
func (c *ListCache) WithMetrics(m *metrics.Metrics) *ListCache {
	if c == nil {
		return c
	}
	c.lru.setMetricsSink(m)
	return c
}

// Stats snapshots the cache's cumulative hit/miss/eviction counters.
func (c *ListCache) Stats() CacheStats {
	if c == nil {
		return CacheStats{}
	}
	return c.lru.stats()
}

func listCacheKey(bucket, prefix, delimiter string, maxKeys int, continuationToken string) string {
	return bucket + "|" + prefix + "|" + delimiter + "|" + strconv.Itoa(maxKeys) + "|" + continuationToken
}

// Get retrieves a cached list result.
func (c *ListCache) Get(bucket, prefix, delimiter string, maxKeys int, continuationToken string) (*ListResult, bool) {
	entry, ok := c.lru.get(listCacheKey(bucket, prefix, delimiter, maxKeys, continuationToken), nil)
	if !ok {
		return nil, false
	}
	return entry.Result, true
}

// Set stores a list result in the cache.
func (c *ListCache) Set(bucket, prefix, delimiter string, maxKeys int, continuationToken string, result *ListResult) {
	c.lru.set(listCacheKey(bucket, prefix, delimiter, maxKeys, continuationToken), &ListCacheEntry{
		Result: result,
		Prefix: prefix,
	})
}

// InvalidateForWrite deletes cached list results whose queried prefix covers
// the written object key. Prefixes are recorded on entries because the cache
// key also contains delimiter and pagination parameters.
func (c *ListCache) InvalidateForWrite(bucket, key string) {
	bucketPrefix := bucket + "|"
	c.lru.removeWhere(func(cacheKey string, entry *ListCacheEntry) bool {
		return strings.HasPrefix(cacheKey, bucketPrefix) && strings.HasPrefix(key, entry.Prefix)
	})
}

// InvalidatePrefix deletes all cache entries for the given bucket+prefix combination.
func (c *ListCache) InvalidatePrefix(bucket, prefix string) {
	needle := bucket + "|" + prefix
	c.lru.removeWhere(func(k string, _ *ListCacheEntry) bool {
		return strings.HasPrefix(k, needle)
	})
}

// Clear removes all entries from the list cache.
func (c *ListCache) Clear() {
	c.lru.clear()
}

// Size returns the number of entries in the list cache.
func (c *ListCache) Size() int {
	n, _ := c.lru.size()
	return n
}
