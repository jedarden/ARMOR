package backend

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/metrics"
)

// newTestLRU builds an LRU whose entry size is len(value) — the byte bound
// then reads directly as "total cached bytes".
func newTestLRU(maxEntries int, maxBytes int64, ttl time.Duration) *lruCache[string] {
	return newLRUCache[string]("test", maxEntries, maxBytes, ttl,
		func(v string) int64 { return int64(len(v)) }, nil)
}

func TestLRU_EvictionOrderIsRecency(t *testing.T) {
	c := newTestLRU(3, 0, time.Minute)

	c.set("a", "aa")
	c.set("b", "bb")
	c.set("c", "cc")

	// Touch "a": recency is now [a, c, b] MRU→LRU, so b is least recently
	// used despite being newer than a by insert order.
	if _, ok := c.get("a", nil); !ok {
		t.Fatal("expected hit on a")
	}

	// Inserting a fourth entry must evict b, the least recently used —
	// not a, the oldest insert. The former evictOldest scans evicted by
	// earliest expiry, which here coincides with oldest insert (a).
	c.set("d", "dd")

	for _, key := range []string{"a", "c", "d"} {
		if _, ok := c.get(key, nil); !ok {
			t.Errorf("expected %s to survive eviction", key)
		}
	}
	if _, ok := c.get("b", nil); ok {
		t.Error("expected b (least recently used) to be evicted")
	}
	if n, _ := c.size(); n != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", n)
	}
}

func TestLRU_ByteBoundEvictsLeastRecentlyUsed(t *testing.T) {
	// Budget: 10 bytes; sizes: a=4, b=4, c=4.
	c := newTestLRU(100, 10, time.Minute)

	c.set("a", "aaaa")
	c.set("b", "bbbb")
	if _, b := c.size(); b != 8 {
		t.Fatalf("expected 8 bytes cached, got %d", b)
	}

	// Touch a so b becomes the least recently used.
	c.get("a", nil)
	c.set("c", "cccc")

	if _, b := c.size(); b > 10 {
		t.Errorf("byte bound violated: %d bytes cached", b)
	}
	if _, ok := c.get("b", nil); ok {
		t.Error("expected b (LRU) to be evicted by the byte bound")
	}
	if _, ok := c.get("a", nil); !ok {
		t.Error("expected a to survive the byte-bound eviction")
	}
}

func TestLRU_OversizedEntryRefused(t *testing.T) {
	c := newTestLRU(10, 4, time.Minute)

	c.set("a", "aaaa")
	c.set("big", "bbbbbbbbb") // 9 bytes > the whole 4-byte budget

	if n, _ := c.size(); n != 1 {
		t.Errorf("expected refused insert to leave 1 entry, got %d", n)
	}
	if _, ok := c.get("big", nil); ok {
		t.Error("an entry larger than the whole budget must not be cached")
	}
	if _, ok := c.get("a", nil); !ok {
		t.Error("the refusal must not evict existing entries")
	}
	if got := c.stats().Evictions; got != 1 {
		t.Errorf("expected the refusal to count as one eviction, got %d", got)
	}
}

func TestLRU_ReplaceAdjustsBytesWithoutEvicting(t *testing.T) {
	c := newTestLRU(10, 10, time.Minute)

	c.set("a", "aaaa")
	c.set("b", "bb")
	c.set("a", "a") // shrink in place: 4+2 -> 1+2 = 3 bytes total

	if _, b := c.size(); b != 3 {
		t.Errorf("expected replace to recompute bytes to 3, got %d", b)
	}
	if n, _ := c.size(); n != 2 {
		t.Errorf("expected both entries to survive, got %d", n)
	}
	if _, ok := c.get("b", nil); !ok {
		t.Error("shrinking an entry must not evict its neighbors")
	}

	// Growing past the bound evicts the OTHER entry, never the one being
	// replaced.
	c.set("a", "aaaaaaaaa")
	if _, ok := c.get("b", nil); ok {
		t.Error("expected b to be evicted once a grew past the budget")
	}
	if _, ok := c.get("a", nil); !ok {
		t.Error("the replaced entry itself must survive its own resize")
	}
}

func TestLRU_ReplacedEntryIsMostRecent(t *testing.T) {
	c := newTestLRU(2, 0, time.Minute)

	c.set("a", "aa")
	c.set("b", "bb")
	c.set("a", "aa") // replace moves a to the front; b is now LRU
	c.set("c", "cc")

	if _, ok := c.get("b", nil); ok {
		t.Error("expected b to be evicted after a's replacement refreshed it")
	}
	if _, ok := c.get("a", nil); !ok {
		t.Error("expected a to survive")
	}
}

func TestLRU_ExpiryReapsAndCounts(t *testing.T) {
	c := newTestLRU(10, 0, time.Minute)
	c.ttl = time.Nanosecond
	c.set("a", "aa")
	time.Sleep(2 * time.Millisecond)

	if _, ok := c.get("a", nil); ok {
		t.Error("expected expired entry to miss")
	}
	if n, b := c.size(); n != 0 || b != 0 {
		t.Errorf("expected expired entry to be reaped on lookup, got %d entries / %d bytes", n, b)
	}
	s := c.stats()
	if s.Misses != 1 || s.Hits != 0 {
		t.Errorf("expected 1 miss / 0 hits, got %+v", s)
	}
	if s.Evictions != 1 {
		t.Errorf("expected the expired reap to count as an eviction, got %d", s.Evictions)
	}
}

func TestLRU_ValidationMissLeavesEntry(t *testing.T) {
	c := newTestLRU(10, 0, time.Minute)
	c.set("a", "aa")

	// A validation failure is a miss, but the entry stays: a later caller
	// holding the matching version/ETag can still be served.
	if _, ok := c.get("a", func(v string) bool { return false }); ok {
		t.Error("expected validation failure to miss")
	}
	if n, _ := c.size(); n != 1 {
		t.Errorf("expected entry to survive a validation miss, got %d entries", n)
	}
	if _, ok := c.get("a", func(v string) bool { return v == "aa" }); !ok {
		t.Error("expected entry to serve a later matching lookup")
	}
	s := c.stats()
	if s.Misses != 1 || s.Hits != 1 {
		t.Errorf("expected 1 miss / 1 hit, got %+v", s)
	}
}

func TestLRU_RemoveWhereDoesNotCountEviction(t *testing.T) {
	c := newTestLRU(10, 0, time.Minute)
	c.set("a", "aa")
	c.set("b", "bb")

	removed := c.removeWhere(func(k string, _ string) bool { return strings.HasPrefix(k, "a") })
	if removed != 1 {
		t.Fatalf("expected 1 removal, got %d", removed)
	}
	if _, ok := c.get("b", nil); !ok {
		t.Error("non-matching entry must survive removeWhere")
	}
	if got := c.stats().Evictions; got != 0 {
		t.Errorf("invalidation is not eviction; expected 0 evictions, got %d", got)
	}
}

func TestLRU_ClearAndCounters(t *testing.T) {
	c := newTestLRU(2, 0, time.Minute)
	c.set("a", "aa")
	c.set("b", "bb")
	c.set("c", "cc") // evicts a
	c.get("a", nil)  // miss
	c.clear()

	if n, b := c.size(); n != 0 || b != 0 {
		t.Errorf("expected clear to empty the cache, got %d entries / %d bytes", n, b)
	}
	s := c.stats()
	if s.Evictions != 1 {
		t.Errorf("expected cumulative evictions to survive clear, got %d", s.Evictions)
	}
	if s.Hits != 0 || s.Misses != 1 {
		t.Errorf("expected counters to survive clear, got %+v", s)
	}
}

func TestLRU_MetricsSinkPublishes(t *testing.T) {
	m := metrics.NewMetrics()
	c := newLRUCache[string]("metadata", 1, 0, time.Minute,
		func(v string) int64 { return int64(len(v)) }, m)

	c.set("a", "aa")
	c.get("a", nil)  // hit
	c.get("x", nil)  // miss
	c.set("b", "bb") // evicts a (maxEntries 1)

	dump := m.PrometheusFormat()
	for _, series := range []struct {
		line, want string
	}{
		{`armor_cache_lookups_total{cache="metadata",outcome="hit"}`, "1"},
		{`armor_cache_lookups_total{cache="metadata",outcome="miss"}`, "1"},
		{`armor_cache_evictions_total{cache="metadata"}`, "1"},
	} {
		if !strings.Contains(dump, series.line+" "+series.want+"\n") {
			t.Errorf("missing series %s %s in Prometheus output", series.line, series.want)
		}
	}
}

func TestLRU_NilSizeFuncDefaultsToOneByte(t *testing.T) {
	c := newLRUCache[string]("test", 10, 2, time.Minute, nil, nil)
	c.set("a", "whatever")
	c.set("b", "whatever")
	c.set("c", "whatever")

	if n, b := c.size(); n != 2 || b != 2 {
		t.Errorf("expected nil sizeOf to cost 1 byte per entry (2/2), got %d entries / %d bytes", n, b)
	}
}

// The wrapper-level bound tests below pin that each public cache actually
// wires a byte budget and LRU order, not just the shared core.

func TestFooterCache_ByteBound(t *testing.T) {
	c := NewFooterCache(1000, 60, 1024)

	c.Set("b", "k1", "e1", make([]byte, 600))
	c.Set("b", "k2", "e2", make([]byte, 600)) // 1200 > 1024: k1 (LRU) goes

	if got := c.Size(); got != 1 {
		t.Fatalf("expected byte bound to hold one footer, got %d entries", got)
	}
	if _, ok := c.Get("b", "k1", "e1"); ok {
		t.Error("expected k1 to be evicted by the byte bound")
	}
	if _, ok := c.Get("b", "k2", "e2"); !ok {
		t.Error("expected k2 to survive")
	}

	// A single footer larger than the whole budget is refused.
	c.Set("b", "k3", "e3", make([]byte, 4096))
	if _, ok := c.Get("b", "k3", "e3"); ok {
		t.Error("an oversized footer must not be cached")
	}
	if got := c.Size(); got != 1 {
		t.Errorf("oversized footer must not evict k2; size = %d", got)
	}
}

func TestMetadataCache_LRUEvictionOrder(t *testing.T) {
	c := NewMetadataCache(2, 60)
	meta := func(size int64) *ARMORMetadata { return &ARMORMetadata{PlaintextSize: size} }

	c.Set("b", "k1", meta(1))
	c.Set("b", "k2", meta(2))
	if _, ok := c.Get("b", "k1"); !ok {
		t.Fatal("expected hit on k1")
	}
	c.Set("b", "k3", meta(3))

	if _, ok := c.Get("b", "k2"); ok {
		t.Error("expected k2 (LRU) to be evicted, not k1")
	}
	if _, ok := c.Get("b", "k1"); !ok {
		t.Error("expected the recently used k1 to survive")
	}
}

func TestMultipartSidecarCache_ByteBound(t *testing.T) {
	// clenB64 renders one block's ciphertext length the way real sidecars
	// store it: a base64 big-endian uint32. buildPrefixSums validates that
	// the per-part block lengths sum to the part's declared ciphertext
	// length, so the fixture must be internally consistent.
	clenB64 := func(n uint32) string {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, n)
		return base64.StdEncoding.EncodeToString(b)
	}
	sidecar := func(nParts, blocksPerPart int) *HMACTableSidecarV3 {
		s := &HMACTableSidecarV3{Parts: make([]HMACPartV3, nParts)}
		for i := range s.Parts {
			s.Parts[i].N = i + 1
			s.Parts[i].PlaintextLen = 100
			s.Parts[i].CiphertextLen = int64(50 * blocksPerPart)
			s.Parts[i].Blocks = make([][]string, blocksPerPart)
			for j := range s.Parts[i].Blocks {
				s.Parts[i].Blocks[j] = []string{
					strings.Repeat("h", 44), clenB64(50),
				}
			}
		}
		return s
	}

	// Tiny budget: the first sidecar (many base64 block strings) fills it;
	// the second must evict the first.
	c := NewMultipartSidecarCache(1000, 60, 4096)
	if err := c.Set("b", "k1", "e1", sidecar(4, 8)); err != nil {
		t.Fatalf("Set k1: %v", err)
	}
	if err := c.Set("b", "k2", "e2", sidecar(4, 8)); err != nil {
		t.Fatalf("Set k2: %v", err)
	}

	if got := c.Size(); got != 1 {
		t.Fatalf("expected byte bound to hold one sidecar, got %d entries", got)
	}
	if c.Get("b", "k1", "e1") != nil {
		t.Error("expected k1 to be evicted by the byte bound")
	}
	if c.Get("b", "k2", "e2") == nil {
		t.Error("expected k2 to survive")
	}
}
