package backend

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// Cache names used as the `cache` label value on armor_cache_lookups_total
// and armor_cache_evictions_total. One set of spellings, shared by the four
// bounded caches, so the label space stays closed.
const (
	cacheNameMetadata = "metadata"
	cacheNameFooter   = "footer"
	cacheNameList     = "list"
	cacheNameSidecar  = "multipart_sidecar"
)

// CacheStats is a point-in-time snapshot of one bounded cache's cumulative
// counters. The counters themselves are atomics, so a snapshot is safe to
// take alongside any load.
type CacheStats struct {
	Hits      uint64 // lookups served from a live, matching entry
	Misses    uint64 // lookups that found no usable entry
	Evictions uint64 // entries dropped by the entry/byte bounds
}

// lruEntry is one cached value plus its intrusive recency-list links.
// head.next is the most recently used entry, tail.prev the least.
type lruEntry[V any] struct {
	key        string
	value      V
	bytes      int64
	expiresAt  time.Time
	prev, next *lruEntry[V]
}

// lruCache is a string-keyed LRU cache bounded by both entry count and the
// sum of entry byte sizes, with TTL expiry. It replaces the four copy-pasted
// map-plus-evictOldest caches (metadata, list, footer, multipart sidecar):
// those scanned the whole map under a write lock to evict one entry — O(n)
// per insert at capacity — and evicted by earliest expiry, not recency,
// despite their docstrings saying LRU (armor-a4e5356d).
//
// Eviction is O(1): the recency list's tail is dropped until both bounds
// hold. Expiry is lazy — a lookup that finds an expired entry reaps it (and
// counts it as an eviction); unvisited expired entries keep costing bytes
// until capacity pressure or a matching lookup removes them, the standard
// Redis-style trade so no insert pays an O(n) expired sweep.
//
// Every method counts hits/misses/evictions into atomic counters and, when a
// metrics sink is wired, into armor_cache_lookups_total{cache,outcome} and
// armor_cache_evictions_total{cache}.
type lruCache[V any] struct {
	mu      sync.Mutex
	entries map[string]*lruEntry[V]
	head    *lruEntry[V] // sentinel; head.next is the MRU entry
	tail    *lruEntry[V] // sentinel; tail.prev is the LRU entry (first evicted)

	bytes      int64
	maxEntries int
	maxBytes   int64 // <= 0 disables the byte bound
	ttl        time.Duration
	sizeOf     func(V) int64 // estimates one entry's memory cost

	hits      atomic.Uint64
	misses    atomic.Uint64
	evictions atomic.Uint64

	sink *metrics.Metrics // nil disables metric publication
	name string           // cache label on published metrics
}

// newLRUCache builds a bounded LRU. maxEntries < 1 is clamped to 1: a cache
// configured to hold nothing would thrash (every insert evicting itself),
// and holding one entry is strictly more useful. maxBytes <= 0 disables the
// byte bound. sizeOf may be nil, in which case every entry costs one byte
// and the byte bound degenerates to a second entry bound.
func newLRUCache[V any](name string, maxEntries int, maxBytes int64, ttl time.Duration, sizeOf func(V) int64, sink *metrics.Metrics) *lruCache[V] {
	if maxEntries < 1 {
		maxEntries = 1
	}
	if sizeOf == nil {
		sizeOf = func(V) int64 { return 1 }
	}
	c := &lruCache[V]{
		entries:    make(map[string]*lruEntry[V]),
		maxEntries: maxEntries,
		maxBytes:   maxBytes,
		ttl:        ttl,
		sizeOf:     sizeOf,
		sink:       sink,
		name:       name,
	}
	c.head = &lruEntry[V]{}
	c.tail = &lruEntry[V]{}
	c.head.next = c.tail
	c.tail.prev = c.head
	return c
}

// setMetricsSink wires (or rewires) the metrics sink. Used by the caches'
// WithMetrics chainables, which exist because handlers build some caches
// before their metrics instance is available.
func (c *lruCache[V]) setMetricsSink(sink *metrics.Metrics) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sink = sink
}

// get returns the entry for key when it exists, is unexpired, and passes
// valid (valid == nil accepts any entry). A found entry that fails valid is
// a miss but is left in place: the version/ETag mismatches that fail
// validation mean the caller observed a different generation, not that the
// entry is unusable for a later caller holding the generation it was stored
// under. Only a hit refreshes recency.
func (c *lruCache[V]) get(key string, valid func(V) bool) (V, bool) {
	var zero V
	c.mu.Lock()
	e, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		c.countMiss()
		return zero, false
	}
	if time.Now().After(e.expiresAt) {
		c.dropLocked(e)
		c.mu.Unlock()
		c.countEviction()
		c.countMiss()
		return zero, false
	}
	if valid != nil && !valid(e.value) {
		c.mu.Unlock()
		c.countMiss()
		return zero, false
	}
	c.moveToFrontLocked(e)
	c.mu.Unlock()
	c.countHit()
	return e.value, true
}

// set stores value under key, replacing any existing entry, and enforces
// both bounds. A single value whose size exceeds the whole byte budget is
// refused outright — inserting it would evict every other entry only to be
// dropped again on the next insert; the refusal is counted as an eviction.
func (c *lruCache[V]) set(key string, value V) {
	entryBytes := c.sizeOf(value)
	if c.maxBytes > 0 && entryBytes > c.maxBytes {
		c.countEviction()
		return
	}

	c.mu.Lock()
	now := time.Now()
	if e, ok := c.entries[key]; ok {
		c.bytes -= e.bytes
		e.value = value
		e.bytes = entryBytes
		e.expiresAt = now.Add(c.ttl)
		c.moveToFrontLocked(e)
	} else {
		e := &lruEntry[V]{
			key:       key,
			value:     value,
			bytes:     entryBytes,
			expiresAt: now.Add(c.ttl),
		}
		c.entries[key] = e
		c.pushFrontLocked(e)
	}
	c.bytes += entryBytes
	c.evictLocked()
	c.mu.Unlock()
}

// remove deletes one entry by key.
func (c *lruCache[V]) remove(key string) {
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		c.dropLocked(e)
	}
	c.mu.Unlock()
}

// removeWhere deletes every entry whose key or value matches pred and
// returns how many were removed. Removal by invalidation is not eviction —
// the entries were correct but deliberately dropped — so it does not touch
// the eviction counter.
func (c *lruCache[V]) removeWhere(pred func(key string, value V) bool) int {
	c.mu.Lock()
	removed := 0
	for k, e := range c.entries {
		if pred(k, e.value) {
			c.dropLocked(e)
			removed++
		}
	}
	c.mu.Unlock()
	return removed
}

// clear drops every entry. Counters survive: they are cumulative.
func (c *lruCache[V]) clear() {
	c.mu.Lock()
	c.entries = make(map[string]*lruEntry[V])
	c.head.next = c.tail
	c.tail.prev = c.head
	c.bytes = 0
	c.mu.Unlock()
}

// size returns the entry count and total entry bytes.
func (c *lruCache[V]) size() (int, int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries), c.bytes
}

// stats snapshots the cumulative counters.
func (c *lruCache[V]) stats() CacheStats {
	return CacheStats{
		Hits:      c.hits.Load(),
		Misses:    c.misses.Load(),
		Evictions: c.evictions.Load(),
	}
}

// pushFrontLocked links e as the most recently used entry. c.mu must be held.
func (c *lruCache[V]) pushFrontLocked(e *lruEntry[V]) {
	e.prev = c.head
	e.next = c.head.next
	c.head.next.prev = e
	c.head.next = e
}

// moveToFrontLocked unlinks e and relinks it at the front. c.mu must be held.
func (c *lruCache[V]) moveToFrontLocked(e *lruEntry[V]) {
	if c.head.next == e {
		return
	}
	e.prev.next = e.next
	e.next.prev = e.prev
	c.pushFrontLocked(e)
}

// dropLocked unlinks e from the recency list and map and repays its bytes.
// c.mu must be held.
func (c *lruCache[V]) dropLocked(e *lruEntry[V]) {
	e.prev.next = e.next
	e.next.prev = e.prev
	delete(c.entries, e.key)
	c.bytes -= e.bytes
}

// evictLocked drops least-recently-used entries until both bounds hold.
// c.mu must be held. The just-inserted entry sits at the front, so it can
// only be evicted here when it is itself the LRU entry — impossible after
// the oversized-entry refusal unless it alone exceeds maxEntries.
func (c *lruCache[V]) evictLocked() {
	for len(c.entries) > c.maxEntries || (c.maxBytes > 0 && c.bytes > c.maxBytes) {
		e := c.tail.prev
		if e == c.head {
			return
		}
		c.dropLocked(e)
		c.evictions.Add(1)
		if c.sink != nil {
			c.sink.IncCacheEviction(c.name)
		}
	}
}

func (c *lruCache[V]) countHit() {
	c.hits.Add(1)
	if c.sink != nil {
		c.sink.IncCacheLookup(c.name, "hit")
	}
}

func (c *lruCache[V]) countMiss() {
	c.misses.Add(1)
	if c.sink != nil {
		c.sink.IncCacheLookup(c.name, "miss")
	}
}

func (c *lruCache[V]) countEviction() {
	c.evictions.Add(1)
	if c.sink != nil {
		c.sink.IncCacheEviction(c.name)
	}
}
