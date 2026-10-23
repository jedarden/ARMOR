package handlers

// Generation-keyed v3 block-table cache and single-authoritative-HEAD read
// reuse (armor-c3199f1c). The unit tests pin the cache contract — stored-
// generation keying, entry/byte/TTL bounds, in-flight load coalescing, and
// the empty-generation bypass — and the handler-level tests pin that repeated
// same-version ranges stop fetching the trailer while an overwrite (even a
// same-plaintext re-encryption that keeps the ETag) forces a refetch.

import (
	"context"
	"errors"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// headCountingBackend records HEAD calls and serves a canned ObjectInfo, for
// asserting that freshness verification and size resolution share one
// authoritative HEAD.
type headCountingBackend struct {
	*backend.NilBackend

	mu    sync.Mutex
	heads int
	info  *backend.ObjectInfo
}

func (b *headCountingBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	b.mu.Lock()
	b.heads++
	info := b.info
	b.mu.Unlock()
	return info, nil
}

func (b *headCountingBackend) headCalls() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.heads
}

// genA returns the stored generation for obj as an authoritative HEAD would
// report it, tagged with the given version/etag so tests can simulate
// overwrites and re-encryptions.
func genA(obj *v3SinglePUTObject, versionID, etag string, lastModified time.Time) v3TableKeyParts {
	return v3TableKeyParts{
		VersionID:    versionID,
		ETag:         etag,
		StoredSize:   int64(len(obj.stored)),
		LastModified: lastModified,
	}
}

// TestV3RangeReusesTableAcrossRequests drives two range reads of the same
// stored generation through one Handlers: the first fetches the trailer block
// table, the second must be served from the cache with no trailer round trip.
// A third read under a new generation (an overwrite that kept the plaintext
// ETag but produced a new stored object) must refetch — never serve the
// previous object's table.
func TestV3RangeReusesTableAcrossRequests(t *testing.T) {
	obj := newV3SinglePUTObject(t, 8, 1024, 0)
	beb := newRangeBackend(obj.stored)
	h := &Handlers{backend: beb, v3Tables: newV3BlockTableCache(100, 300, 1<<20)}

	decryptor, err := crypto.NewDecryptor(obj.dek, obj.iv, obj.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	armorMeta := &backend.ARMORMetadata{
		BlockSize:   obj.blockSize,
		IV:          obj.iv,
		ETag:        "etag-v3-table-reuse",
		ContentType: "application/octet-stream",
	}

	ciphertextSize := int64(len(obj.stored))
	gen := genA(obj, "version-1", armorMeta.ETag, time.Now())

	doRange := func(g v3TableKeyParts, start, end int64) string {
		t.Helper()
		r := httptest.NewRequest("GET", "/bucket/key", nil)
		w := httptest.NewRecorder()
		h.handleV3RangeRequest(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, int64(len(obj.plaintext)), ciphertextSize, time.Now(), start, end, g)
		if w.Code != 206 {
			t.Fatalf("range status = %d, body %s", w.Code, w.Body.String())
		}
		return w.Body.String()
	}

	wantSpan := string(obj.plaintext[2*1024 : 5*1024])
	if got := doRange(gen, 2*1024, 5*1024-1); got != wantSpan {
		t.Fatalf("first range plaintext mismatch")
	}
	callsAfterFirst := len(beb.recordedCalls())
	if callsAfterFirst != 2 { // trailer + span
		t.Fatalf("first range backend calls = %d (%+v), want 2 (trailer + span)", callsAfterFirst, beb.recordedCalls())
	}

	// Same generation: the trailer must come from the cache — one call total.
	if got := doRange(gen, 2*1024, 5*1024-1); got != wantSpan {
		t.Fatalf("second range plaintext mismatch")
	}
	if got := len(beb.recordedCalls()); got != callsAfterFirst+1 {
		t.Fatalf("second range added %d backend calls (%+v), want exactly the span read — trailer must be cached", got-callsAfterFirst, beb.recordedCalls())
	}

	// New stored generation, same plaintext ETag (a same-plaintext
	// re-encryption keeps the ETag): the key must miss and refetch the
	// trailer, never serve the old object's table.
	reenc := genA(obj, "version-1", armorMeta.ETag, gen.LastModified.Add(2*time.Second))
	if got := doRange(reenc, 2*1024, 5*1024-1); got != wantSpan {
		t.Fatalf("post-re-encryption range plaintext mismatch")
	}
	if got := len(beb.recordedCalls()); got != callsAfterFirst+3 {
		t.Fatalf("re-encrypted generation backend calls = %d, want 2 more after the cached read (trailer + span)", got)
	}

	m := h.v3Tables.metrics()
	if m.Hits != 1 || m.Loads != 2 {
		t.Fatalf("cache metrics = %+v, want 1 hit (second read) and 2 loads (first read + re-encryption)", m)
	}
}

// TestV3TableCacheKeyIncludesEveryGenerationSignal pins the key contract: two
// generations that agree on the plaintext ETag but differ in any stored-object
// signal (VersionID, StoredSize, LastModified) are distinct entries.
func TestV3TableCacheKeyIncludesEveryGenerationSignal(t *testing.T) {
	base := v3TableKeyParts{VersionID: "v1", ETag: "e", StoredSize: 10, LastModified: time.Unix(100, 0)}
	variants := []v3TableKeyParts{
		{VersionID: "v2", ETag: "e", StoredSize: 10, LastModified: base.LastModified},
		{VersionID: "v1", ETag: "other", StoredSize: 10, LastModified: base.LastModified},
		{VersionID: "v1", ETag: "e", StoredSize: 11, LastModified: base.LastModified},
		{VersionID: "v1", ETag: "e", StoredSize: 10, LastModified: base.LastModified.Add(time.Second)},
	}
	seen := map[string]string{base.key("b", "k"): "base"}
	for i, v := range variants {
		k := v.key("b", "k")
		if prev, dup := seen[k]; dup {
			t.Fatalf("variant %d collides with %s: key %q", i, prev, k)
		}
		seen[k] = fmt.Sprintf("variant-%d", i)
	}
}

// TestV3TableCacheCoalescesConcurrentMisses fires concurrent loads of one
// generation and pins that exactly one trailer fetch reaches the backend.
func TestV3TableCacheCoalescesConcurrentMisses(t *testing.T) {
	obj := newV3SinglePUTObject(t, 4, 1024, 0)
	beb := newRangeBackend(obj.stored)
	h := &Handlers{backend: beb, v3Tables: newV3BlockTableCache(100, 300, 1<<20)}

	gen := genA(obj, "version-1", "etag-coalesce", time.Now())
	const workers = 8
	start := make(chan struct{})
	errs := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			table, err := h.loadV3BlockTable(context.Background(), "bucket", "prefixed/key", gen, int64(len(obj.stored)), 4)
			if err == nil && table == nil {
				err = fmt.Errorf("loadV3BlockTable returned nil table without error")
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent load: %v", err)
		}
	}

	if calls := len(beb.recordedCalls()); calls != 1 {
		t.Fatalf("trailer fetches = %d (%+v), want 1 — concurrent misses must share one load", calls, beb.recordedCalls())
	}
	m := h.v3Tables.metrics()
	if m.Loads != 1 {
		t.Fatalf("loads = %d, want 1", m.Loads)
	}
	if m.LoadBytes == 0 {
		t.Fatalf("load bytes = 0, want the trailer byte count")
	}
	if size, bytes := h.v3Tables.size(); size != 1 || bytes == 0 {
		t.Fatalf("cache size = (%d entries, %d bytes), want (1, >0)", size, bytes)
	}
}

// TestV3TableCacheBoundsBytesAndEntries pins both bounds: an entry larger than
// the whole cache is never stored, and the entry-count bound evicts even when
// bytes would still fit.
func TestV3TableCacheBoundsBytesAndEntries(t *testing.T) {
	obj := newV3SinglePUTObject(t, 4, 1024, 0)
	entryBytes := int64(4) * crypto.BlockTableEntrySize

	// An entry bigger than the whole cache is dropped, not stored.
	tooSmall := newV3BlockTableCache(100, 300, entryBytes-1)
	tooSmall.storeLocked("k", obj.table)
	if size, _ := tooSmall.size(); size != 0 {
		t.Fatalf("oversized entry stored; size = %d", size)
	}
	if tooSmall.metrics().Evictions == 0 {
		t.Fatalf("oversized store did not count an eviction")
	}

	// Entry-count bound: a second distinct key evicts the first.
	c := newV3BlockTableCache(1, 300, 1<<20)
	c.storeLocked("k1", obj.table)
	c.storeLocked("k2", obj.table)
	if size, bytes := c.size(); size != 1 || bytes > entryBytes {
		t.Fatalf("entry bound failed: size = %d, bytes = %d, want 1 entry at %d bytes", size, bytes, entryBytes)
	}

	// Byte bound: entries whose combined bytes exceed maxBytes evict down.
	c2 := newV3BlockTableCache(100, 300, entryBytes+entryBytes/2)
	c2.storeLocked("k1", obj.table)
	c2.storeLocked("k2", obj.table)
	if size, bytes := c2.size(); size != 1 || bytes != entryBytes {
		t.Fatalf("byte bound failed: size = %d, bytes = %d, want 1 entry at %d bytes", size, bytes, entryBytes)
	}
}

// TestV3TableCacheTTLExpiry expires an entry by rewinding its deadline (no
// wall-clock sleep) and pins that the next lookup misses and refetches.
func TestV3TableCacheTTLExpiry(t *testing.T) {
	obj := newV3SinglePUTObject(t, 4, 1024, 0)
	beb := newRangeBackend(obj.stored)
	h := &Handlers{backend: beb, v3Tables: newV3BlockTableCache(100, 300, 1<<20)}

	gen := genA(obj, "version-1", "etag-ttl", time.Now())
	ctx := context.Background()
	if _, err := h.loadV3BlockTable(ctx, "bucket", "prefixed/key", gen, int64(len(obj.stored)), 4); err != nil {
		t.Fatalf("first load: %v", err)
	}
	h.v3Tables.mu.Lock()
	for _, e := range h.v3Tables.entries {
		e.expiresAt = time.Now().Add(-time.Second)
	}
	h.v3Tables.mu.Unlock()

	if _, err := h.loadV3BlockTable(ctx, "bucket", "prefixed/key", gen, int64(len(obj.stored)), 4); err != nil {
		t.Fatalf("post-expiry load: %v", err)
	}
	if calls := len(beb.recordedCalls()); calls != 2 {
		t.Fatalf("trailer fetches = %d, want 2 — an expired entry must miss", calls)
	}
}

// TestV3TableCacheEmptyGenerationBypasses pins that a generation with no
// signals at all never populates the cache (an empty key would collapse
// distinct stored objects onto one entry) but reads still succeed.
func TestV3TableCacheEmptyGenerationBypasses(t *testing.T) {
	obj := newV3SinglePUTObject(t, 4, 1024, 0)
	beb := newRangeBackend(obj.stored)
	h := &Handlers{backend: beb, v3Tables: newV3BlockTableCache(100, 300, 1<<20)}

	ctx := context.Background()
	for i := 0; i < 2; i++ {
		table, err := h.loadV3BlockTable(ctx, "bucket", "prefixed/key", v3TableKeyParts{}, int64(len(obj.stored)), 4)
		if err != nil || table == nil {
			t.Fatalf("bypass load %d: table=%v err=%v", i, table, err)
		}
	}
	if calls := len(beb.recordedCalls()); calls != 2 {
		t.Fatalf("trailer fetches = %d, want one per call — empty generation must bypass the cache", calls)
	}
	if size, _ := h.v3Tables.size(); size != 0 {
		t.Fatalf("empty generation cached an entry; size = %d", size)
	}
	if m := h.v3Tables.metrics(); m.Hits != 0 {
		t.Fatalf("bypass path recorded hits: %+v", m)
	}
}

// TestHeadCiphertextForFreshnessOneHeadPerRequest pins the combined
// freshness-and-size HEAD: one backend HEAD per call, the stored info returned
// to the caller, staleness still reported as a freshness error, and an
// unparseable completion timestamp still performing the HEAD for the size.
func TestHeadCiphertextForFreshnessOneHeadPerRequest(t *testing.T) {
	completedAt := time.Now().Add(-time.Minute)

	t.Run("fresh_object_single_head", func(t *testing.T) {
		be := &headCountingBackend{NilBackend: backend.NewNilBackend(), info: &backend.ObjectInfo{
			Key: "cipher", StoredSize: 164, LastModified: completedAt.Add(-30 * time.Second),
		}}
		h := &Handlers{backend: be}
		info, err := h.headCiphertextForFreshness(context.Background(), "bucket", "cipher", completedAt.Format(time.RFC3339))
		if err != nil {
			t.Fatalf("fresh object: %v", err)
		}
		if info == nil || info.StoredSize != 164 {
			t.Fatalf("info = %+v, want the stored HEAD result", info)
		}
		if be.headCalls() != 1 {
			t.Fatalf("HEADs = %d, want 1", be.headCalls())
		}
	})

	t.Run("stale_object_returns_freshness_error_and_info", func(t *testing.T) {
		be := &headCountingBackend{NilBackend: backend.NewNilBackend(), info: &backend.ObjectInfo{
			Key: "cipher", StoredSize: 164, LastModified: time.Now(),
		}}
		h := &Handlers{backend: be}
		info, err := h.headCiphertextForFreshness(context.Background(), "bucket", "cipher", completedAt.Format(time.RFC3339))
		var freshErr *ciphertextFreshnessError
		if !errors.As(err, &freshErr) {
			t.Fatalf("err = %v, want *ciphertextFreshnessError", err)
		}
		if !strings.Contains(err.Error(), "overwritten after manifest completion") {
			t.Fatalf("freshness message changed: %v", err)
		}
		if info == nil {
			t.Fatalf("info = nil, want the HEAD result alongside the verdict")
		}
	})

	t.Run("verify_wrapper_preserves_verdict", func(t *testing.T) {
		be := &headCountingBackend{NilBackend: backend.NewNilBackend(), info: &backend.ObjectInfo{
			Key: "cipher", StoredSize: 164, LastModified: time.Now(),
		}}
		h := &Handlers{backend: be}
		err := h.verifyCiphertextFreshness(context.Background(), "bucket", "cipher", completedAt.Format(time.RFC3339))
		if err == nil {
			t.Fatal("stale object verified clean")
		}
		var freshErr *ciphertextFreshnessError
		if errors.As(err, &freshErr) {
			t.Fatalf("verifyCiphertextFreshness must report the plain verdict, not the marker type")
		}
	})

	t.Run("unparseable_timestamp_still_heads", func(t *testing.T) {
		be := &headCountingBackend{NilBackend: backend.NewNilBackend(), info: &backend.ObjectInfo{
			Key: "cipher", StoredSize: 164, LastModified: time.Now(),
		}}
		h := &Handlers{backend: be}
		info, err := h.headCiphertextForFreshness(context.Background(), "bucket", "cipher", "")
		if err != nil {
			t.Fatalf("unparseable completedAt: %v", err)
		}
		if info == nil {
			t.Fatal("info = nil; the HEAD must still serve the stored size")
		}
		if be.headCalls() != 1 {
			t.Fatalf("HEADs = %d, want 1", be.headCalls())
		}
	})
}

// TestV3MultipartRangeFooterCacheHit pins that the multipart v3 dispatch
// serves a tail range from the decrypted-footer cache with zero backend calls
// — the generic range path's fast path, now extended to the multipart layout.
func TestV3MultipartRangeFooterCacheHit(t *testing.T) {
	fx := newV3RangeFixture(t, []int{2, 2}, 128)
	beb := newRangeBackend(fx.ciphertext)

	h := &Handlers{backend: beb, multipartSidecarCache: fx.cache, footerCache: backend.NewFooterCache(10, 60)}
	h.footerCache.Set("bucket", "key", v3RangeETag, []byte("PAR1-FOOTER-BYTES"))

	decryptor, err := crypto.NewDecryptor(fx.dek, fx.iv, fx.blockSize)
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	armorMeta := &backend.ARMORMetadata{
		BlockSize:   fx.blockSize,
		IV:          fx.iv,
		ETag:        v3RangeETag,
		ContentType: "application/octet-stream",
	}

	plaintextSize := int64(len(fx.plaintext))
	start := plaintextSize - int64(len("PAR1-FOOTER-BYTES"))
	r := httptest.NewRequest("GET", "/bucket/key", nil)
	w := httptest.NewRecorder()
	h.handleV3MultipartRangeRequest(w, r, "bucket", "key", "prefixed/key", decryptor, armorMeta, plaintextSize, time.Now(), start, plaintextSize-1)

	if w.Code != 206 {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Armor-Footer-Cache"); got != "HIT" {
		t.Fatalf("X-Armor-Footer-Cache = %q, want HIT", got)
	}
	if got := w.Body.String(); got != "PAR1-FOOTER-BYTES" {
		t.Fatalf("footer body = %q", got)
	}
	if calls := len(beb.recordedCalls()); calls != 0 {
		t.Fatalf("footer hit made %d backend calls (%+v), want 0", calls, beb.recordedCalls())
	}
}
