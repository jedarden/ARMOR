package backend

import (
	"strconv"
	"strings"
	"sync"
	"time"
)

// CacheEntry represents a cached metadata entry.
type CacheEntry struct {
	Metadata   *ARMORMetadata
	ExpiresAt  time.Time
}

// MetadataCache is an LRU cache for object metadata.
type MetadataCache struct {
	mu         sync.RWMutex
	entries    map[string]*CacheEntry
	maxEntries int
	ttl        time.Duration
}

// NewMetadataCache creates a new metadata cache.
func NewMetadataCache(maxEntries int, ttlSeconds int) *MetadataCache {
	return &MetadataCache{
		entries:    make(map[string]*CacheEntry),
		maxEntries: maxEntries,
		ttl:        time.Duration(ttlSeconds) * time.Second,
	}
}

// cacheKey generates a cache key from bucket and object key.
func cacheKey(bucket, key string) string {
	return bucket + "/" + key
}

// Get retrieves cached metadata.
func (c *MetadataCache) Get(bucket, key string) (*ARMORMetadata, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[cacheKey(bucket, key)]
	if !ok {
		return nil, false
	}

	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Metadata, true
}

// Set stores metadata in the cache.
func (c *MetadataCache) Set(bucket, key string, meta *ARMORMetadata) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Evict if at capacity
	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	c.entries[cacheKey(bucket, key)] = &CacheEntry{
		Metadata:  meta,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes an entry from the cache.
func (c *MetadataCache) Delete(bucket, key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, cacheKey(bucket, key))
}

// Clear removes all entries from the cache.
func (c *MetadataCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*CacheEntry)
}

// Size returns the number of entries in the cache.
func (c *MetadataCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// evictOldest removes the oldest entry (simple eviction strategy).
// For a proper LRU, we'd track access order, but this is sufficient for now.
func (c *MetadataCache) evictOldest() {
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

// ListCacheEntry is a cached list result.
type ListCacheEntry struct {
	Result    *ListResult
	ExpiresAt time.Time
}

// ListCache caches ListObjectsV2 results keyed by query parameters.
type ListCache struct {
	mu         sync.RWMutex
	entries    map[string]*ListCacheEntry
	maxEntries int
	ttl        time.Duration
}

// NewListCache creates a new list cache. Returns nil when ttlSeconds <= 0 (disabled).
func NewListCache(maxEntries int, ttlSeconds int) *ListCache {
	if ttlSeconds <= 0 {
		return nil
	}
	return &ListCache{
		entries:    make(map[string]*ListCacheEntry),
		maxEntries: maxEntries,
		ttl:        time.Duration(ttlSeconds) * time.Second,
	}
}

func listCacheKey(bucket, prefix, delimiter string, maxKeys int, continuationToken string) string {
	return bucket + "|" + prefix + "|" + delimiter + "|" + strconv.Itoa(maxKeys) + "|" + continuationToken
}

// Get retrieves a cached list result.
func (c *ListCache) Get(bucket, prefix, delimiter string, maxKeys int, continuationToken string) (*ListResult, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.entries[listCacheKey(bucket, prefix, delimiter, maxKeys, continuationToken)]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.ExpiresAt) {
		return nil, false
	}
	return entry.Result, true
}

// Set stores a list result in the cache.
func (c *ListCache) Set(bucket, prefix, delimiter string, maxKeys int, continuationToken string, result *ListResult) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.entries) >= c.maxEntries {
		c.evictOldest()
	}

	c.entries[listCacheKey(bucket, prefix, delimiter, maxKeys, continuationToken)] = &ListCacheEntry{
		Result:    result,
		ExpiresAt: time.Now().Add(c.ttl),
	}
}

// InvalidatePrefix deletes all cache entries for the given bucket+prefix combination.
func (c *ListCache) InvalidatePrefix(bucket, prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	needle := bucket + "|" + prefix
	for k := range c.entries {
		if strings.HasPrefix(k, needle) {
			delete(c.entries, k)
		}
	}
}

// Clear removes all entries from the list cache.
func (c *ListCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*ListCacheEntry)
}

func (c *ListCache) evictOldest() {
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
