package handlers

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/metrics"
)

// enrichCiphertextOverhead is the envelope bytes the fake LIST reports on top
// of the plaintext size — the encrypted size a real B2 ListObjectsV2 returns.
const enrichCiphertextOverhead int64 = 100

// enrichListBackend is a minimal backend for the ListObjectsV2 enrichment
// tests. It mirrors the production B2 semantics that matter here: List
// reports encrypted (ciphertext) sizes and stored ETags per row — real B2 has
// no plaintext size without a per-key metadata call — while Head resolves the
// plaintext size from ARMOR metadata. The countable, failable Head is the
// instrumentation the fan-out and retry assertions read.
type enrichListBackend struct {
	backend.Backend // nil: any method these tests do not stub panics loudly

	// storedKeys are the keys List reports, already in stored space
	// (ARMOR_PREFIX applied). plaintext maps each stored key to the size
	// HEAD resolves it to.
	storedKeys []string
	plaintext  map[string]int64

	failAll          bool
	failFirstAttempt bool // each key's first HEAD fails, the retry succeeds

	mu            sync.Mutex
	headCalls     int
	headAttempts  map[string]int
	requestedKeys []string

	inFlight    atomic.Int32
	maxInFlight atomic.Int32
}

func newEnrichListBackend(keys []string, plaintext int64) *enrichListBackend {
	b := &enrichListBackend{
		storedKeys:   keys,
		plaintext:    make(map[string]int64, len(keys)),
		headAttempts: make(map[string]int, len(keys)),
	}
	for _, k := range keys {
		b.plaintext[k] = plaintext
	}
	return b
}

func (b *enrichListBackend) armorMeta(key string, plaintextSize int64) map[string]string {
	return (&backend.ARMORMetadata{
		Version:       2,
		BlockSize:     65536,
		PlaintextSize: plaintextSize,
		ContentType:   "application/octet-stream",
		ETag:          "etag-" + key,
		IV:            make([]byte, 16),
		WrappedDEK:    []byte("test-wrapped-dek"), // ParseARMORMetadata requires key material
	}).ToMetadata()
}

func (b *enrichListBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	objects := make([]backend.ObjectInfo, 0, len(b.storedKeys))
	for _, k := range b.storedKeys {
		if prefix != "" && !strings.HasPrefix(k, prefix) {
			continue
		}
		pt := b.plaintext[k]
		objects = append(objects, backend.ObjectInfo{
			Key:        k,
			Size:       pt + enrichCiphertextOverhead,
			StoredSize: pt + enrichCiphertextOverhead,
			ETag:       "stored-" + k,
		})
	}
	return &backend.ListResult{Objects: objects}, nil
}

func (b *enrichListBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	b.inFlight.Add(1)
	defer b.inFlight.Add(-1)
	for {
		cur := b.maxInFlight.Load()
		n := b.inFlight.Load()
		if n <= cur || b.maxInFlight.CompareAndSwap(cur, n) {
			break
		}
	}

	b.mu.Lock()
	b.headCalls++
	b.headAttempts[key]++
	attempt := b.headAttempts[key]
	b.requestedKeys = append(b.requestedKeys, key)
	b.mu.Unlock()

	// Small sleep so concurrent HEADs actually overlap under the cap.
	time.Sleep(2 * time.Millisecond)

	if b.failAll || (b.failFirstAttempt && attempt == 1) {
		return nil, fmt.Errorf("head failed for %s", key)
	}
	pt, ok := b.plaintext[key]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return &backend.ObjectInfo{
		Key:              key,
		Size:             pt,
		StoredSize:       pt + enrichCiphertextOverhead,
		ETag:             "stored-" + key,
		IsARMOREncrypted: true,
		Metadata:         b.armorMeta(key, pt),
	}, nil
}

func (b *enrichListBackend) headCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.headCalls
}

// enrichStore is a small in-memory backend for the end-to-end PUT → HEAD →
// LIST warm test. Head and List report the same stored-ETag generation for a
// key — the consistency the version-gated cache needs — and List reports
// ciphertext sizes like real B2.
type enrichStore struct {
	backend.Backend // nil: any method these tests do not stub panics loudly

	mu        sync.Mutex
	objects   map[string][]byte
	meta      map[string]map[string]string
	headCount int
}

func newEnrichStore() *enrichStore {
	return &enrichStore{
		objects: make(map[string][]byte),
		meta:    make(map[string]map[string]string),
	}
}

// headCallsViaStore counts Head round-trips the handler stack issued.
func (s *enrichStore) headCallsViaStore() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headCount
}

func (s *enrichStore) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[bucket+"/"+key] = data
	s.meta[bucket+"/"+key] = meta
	return nil
}

func (s *enrichStore) objectInfo(bucket, key string, meta map[string]string, data []byte) *backend.ObjectInfo {
	info := &backend.ObjectInfo{
		Key:        key,
		Size:       int64(len(data)),
		StoredSize: int64(len(data)),
		Metadata:   meta,
		ETag:       "stored-" + bucket + "/" + key,
	}
	if am, ok := backend.ParseARMORMetadata(meta); ok && am != nil {
		info.IsARMOREncrypted = true
		info.Size = am.PlaintextSize
		info.ContentType = am.ContentType
	}
	return info
}

func (s *enrichStore) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.headCount++
	k := bucket + "/" + key
	data, ok := s.objects[k]
	if !ok {
		return nil, fmt.Errorf("object not found: %s", key)
	}
	return s.objectInfo(bucket, key, s.meta[k], data), nil
}

func (s *enrichStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := bucket + "/" + key
	data, ok := s.objects[k]
	if !ok {
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), s.objectInfo(bucket, key, s.meta[k], data), nil
}

func (s *enrichStore) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	body, _, err := s.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return body, err
}

func (s *enrichStore) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	s.mu.Lock()
	k := bucket + "/" + key
	data, ok := s.objects[k]
	if !ok {
		s.mu.Unlock()
		return nil, nil, fmt.Errorf("object not found: %s", key)
	}
	all := append([]byte(nil), data...)
	s.mu.Unlock()
	if offset > int64(len(all)) {
		offset = int64(len(all))
	}
	end := offset + length
	if end > int64(len(all)) || end < offset {
		end = int64(len(all))
	}
	return io.NopCloser(bytes.NewReader(all[offset:end])), map[string]string{}, nil
}

func (s *enrichStore) Delete(ctx context.Context, bucket, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, bucket+"/"+key)
	delete(s.meta, bucket+"/"+key)
	return nil
}

func (s *enrichStore) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, k := range keys {
		if err := s.Delete(ctx, bucket, k); err != nil {
			return err
		}
	}
	return nil
}

func (s *enrichStore) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	objects := make([]backend.ObjectInfo, 0, len(s.objects))
	for k, data := range s.objects {
		key := k[len(bucket)+1:]
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		objects = append(objects, backend.ObjectInfo{
			Key:        key,
			Size:       int64(len(data)), // ciphertext size, like real B2
			StoredSize: int64(len(data)),
			ETag:       "stored-" + k,
		})
	}
	return &backend.ListResult{Objects: objects}, nil
}

func (s *enrichStore) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	return s.List(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

// enrichManifest is a minimal ManifestRecorder double for these tests; it
// never persists anywhere.
type enrichManifest struct {
	mu      sync.Mutex
	entries map[string]*ManifestEntry
}

func (m *enrichManifest) RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, mekFingerprint string, blockSize int, contentType, etag string, chainEntry *manifest.ChainEntry, ciphertextSize int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[bucket+"/"+key] = &ManifestEntry{
		PlaintextSize:  size,
		ContentType:    contentType,
		ETag:           etag,
		BlockSize:      blockSize,
		CiphertextSize: ciphertextSize,
	}
}

func (m *enrichManifest) RecordDelete(bucket, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, bucket+"/"+key)
}

func (m *enrichManifest) Lookup(bucket, key string) (*ManifestEntry, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[bucket+"/"+key]
	return e, ok
}

// metricOutcome reads one armor_list_head_total outcome from the Prometheus
// exposition.
func metricOutcome(t *testing.T, m *metrics.Metrics, outcome string) int64 {
	t.Helper()
	for _, line := range strings.Split(m.PrometheusFormat(), "\n") {
		if strings.HasPrefix(line, fmt.Sprintf("armor_list_head_total{outcome=%q} ", outcome)) {
			var v int64
			if _, err := fmt.Sscanf(line, fmt.Sprintf("armor_list_head_total{outcome=%q} %%d", outcome), &v); err == nil {
				return v
			}
		}
	}
	return 0
}

// TestListEnrichResolvesFromManifestIndex proves a page whose keys are all in
// the manifest index issues zero backend HEADs and reports plaintext sizes.
func TestListEnrichResolvesFromManifestIndex(t *testing.T) {
	cfg := &config.Config{
		BlockSize:      65536,
		AuthAccessKey:  "test-access-key",
		AuthSecretKey:  "REMOVED-NOT-A-SECRET-VALUE",
		ListHeadFanout: 4,
	}
	be := newEnrichListBackend([]string{"a", "b", "c"}, 1234)
	cache := backend.NewMetadataCache(1000, 300)
	h := New(cfg, be, cache, backend.NewFooterCache(1000, 300), nil, nil)
	h.WithMetrics(metrics.NewMetrics())

	rec := &enrichManifest{entries: make(map[string]*ManifestEntry)}
	for _, k := range be.storedKeys {
		rec.entries["test-bucket/"+k] = &ManifestEntry{PlaintextSize: be.plaintext[k]}
	}
	h.WithManifest(rec)

	result := &backend.ListResult{Objects: []backend.ObjectInfo{
		{Key: "a", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-a"},
		{Key: "b", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-b"},
		{Key: "c", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-c"},
	}}
	h.enrichPlaintextSizes(context.Background(), "test-bucket", result)

	if n := be.headCallCount(); n != 0 {
		t.Fatalf("expected 0 backend HEADs for index-covered keys, got %d", n)
	}
	for _, obj := range result.Objects {
		if obj.Size != 1234 {
			t.Errorf("key %s: expected plaintext size 1234, got %d", obj.Key, obj.Size)
		}
	}
	if got := metricOutcome(t, h.metrics, "index"); got != 3 {
		t.Errorf("expected index outcome = 3, got %d", got)
	}
}

// TestListEnrichCapsConcurrentHeads proves a 1000-key page with no index or
// cache coverage issues exactly one HEAD per key, never more than the
// configured fan-out at once, and resolves every size to the plaintext value.
func TestListEnrichCapsConcurrentHeads(t *testing.T) {
	const fanout = 4
	const keys = 1000
	cfg := &config.Config{
		BlockSize:      65536,
		AuthAccessKey:  "test-access-key",
		AuthSecretKey:  "REMOVED-NOT-A-SECRET-VALUE",
		ListHeadFanout: fanout,
	}
	names := make([]string, keys)
	for i := range names {
		names[i] = fmt.Sprintf("obj-%04d", i)
	}
	be := newEnrichListBackend(names, 5000)
	h := New(cfg, be, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), nil, nil)
	h.WithMetrics(metrics.NewMetrics())

	objects := make([]backend.ObjectInfo, keys)
	for i, k := range names {
		objects[i] = backend.ObjectInfo{Key: k, Size: 5000 + enrichCiphertextOverhead, ETag: "stored-" + k}
	}
	result := &backend.ListResult{Objects: objects}
	h.enrichPlaintextSizes(context.Background(), "test-bucket", result)

	if got := be.maxInFlight.Load(); got > fanout {
		t.Fatalf("HEAD fan-out exceeded the cap: %d concurrent HEADs > %d", got, fanout)
	}
	if n := be.headCallCount(); n != keys {
		t.Fatalf("expected exactly %d HEADs (one per key), got %d", keys, n)
	}
	for _, obj := range result.Objects {
		if obj.Size != 5000 {
			t.Errorf("key %s: expected plaintext size 5000, got %d", obj.Key, obj.Size)
		}
	}
	if got := metricOutcome(t, h.metrics, "head"); got != keys {
		t.Errorf("expected head outcome = %d, got %d", keys, got)
	}
	if got := metricOutcome(t, h.metrics, "error"); got != 0 {
		t.Errorf("expected error outcome = 0, got %d", got)
	}
}

// TestListEnrichHeadFailureNeverReportsCiphertextSize proves a HEAD that fails
// even after its retry leaves the size unset (0) instead of silently reporting
// the ciphertext size — the failure mode that corrupts DuckDB/httpfs byte
// offset arithmetic.
func TestListEnrichHeadFailureNeverReportsCiphertextSize(t *testing.T) {
	cfg := &config.Config{
		BlockSize:      65536,
		AuthAccessKey:  "test-access-key",
		AuthSecretKey:  "REMOVED-NOT-A-SECRET-VALUE",
		ListHeadFanout: 2,
	}
	be := newEnrichListBackend([]string{"x", "y"}, 1234)
	be.failAll = true
	h := New(cfg, be, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), nil, nil)
	h.WithMetrics(metrics.NewMetrics())

	result := &backend.ListResult{Objects: []backend.ObjectInfo{
		{Key: "x", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-x"},
		{Key: "y", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-y"},
	}}
	h.enrichPlaintextSizes(context.Background(), "test-bucket", result)

	for _, obj := range result.Objects {
		if obj.Size != 0 {
			t.Errorf("key %s: expected omitted size 0 after HEAD failure, got %d (ciphertext size would be %d)", obj.Key, obj.Size, 1234+enrichCiphertextOverhead)
		}
	}
	if got := metricOutcome(t, h.metrics, "error"); got != 2 {
		t.Errorf("expected error outcome = 2, got %d", got)
	}
	if got := metricOutcome(t, h.metrics, "head"); got != 0 {
		t.Errorf("expected head outcome = 0, got %d", got)
	}
}

// TestListEnrichRetriesOnceThenResolves proves a transient HEAD failure is
// retried exactly once per key and the retry's result is used.
func TestListEnrichRetriesOnceThenResolves(t *testing.T) {
	cfg := &config.Config{
		BlockSize:      65536,
		AuthAccessKey:  "test-access-key",
		AuthSecretKey:  "REMOVED-NOT-A-SECRET-VALUE",
		ListHeadFanout: 4,
	}
	be := newEnrichListBackend([]string{"r1", "r2", "r3", "r4"}, 777)
	be.failFirstAttempt = true
	h := New(cfg, be, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), nil, nil)
	h.WithMetrics(metrics.NewMetrics())

	objects := make([]backend.ObjectInfo, 0, 4)
	for _, k := range be.storedKeys {
		objects = append(objects, backend.ObjectInfo{Key: k, Size: 777 + enrichCiphertextOverhead, ETag: "stored-" + k})
	}
	result := &backend.ListResult{Objects: objects}
	h.enrichPlaintextSizes(context.Background(), "test-bucket", result)

	if n := be.headCallCount(); n != 8 {
		t.Fatalf("expected 8 HEADs (one retry per key), got %d", n)
	}
	for _, obj := range result.Objects {
		if obj.Size != 777 {
			t.Errorf("key %s: expected plaintext size 777 from the retry, got %d", obj.Key, obj.Size)
		}
	}
	if got := metricOutcome(t, h.metrics, "error"); got != 0 {
		t.Errorf("expected error outcome = 0 after successful retries, got %d", got)
	}
}

// TestListEnrichUsesVersionedCache proves a version-matched cache entry
// resolves the size with zero HEADs, and a stale-version entry (same key,
// different stored generation) misses and costs a HEAD instead of serving the
// previous object's size.
func TestListEnrichUsesVersionedCache(t *testing.T) {
	cfg := &config.Config{
		BlockSize:      65536,
		AuthAccessKey:  "test-access-key",
		AuthSecretKey:  "REMOVED-NOT-A-SECRET-VALUE",
		ListHeadFanout: 4,
	}
	be := newEnrichListBackend([]string{"fresh", "stale"}, 1234)
	cache := backend.NewMetadataCache(1000, 300)
	h := New(cfg, be, cache, backend.NewFooterCache(1000, 300), nil, nil)
	h.WithMetrics(metrics.NewMetrics())

	freshAM := &backend.ARMORMetadata{Version: 2, PlaintextSize: 1234, ETag: "etag-fresh"}
	cache.SetVersioned("test-bucket", "fresh", "stored-fresh", freshAM)
	cache.SetVersioned("test-bucket", "stale", "stored-STALE-GENERATION", &backend.ARMORMetadata{Version: 2, PlaintextSize: 999})

	result := &backend.ListResult{Objects: []backend.ObjectInfo{
		{Key: "fresh", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-fresh"},
		{Key: "stale", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-stale"},
	}}
	h.enrichPlaintextSizes(context.Background(), "test-bucket", result)

	byKey := map[string]int64{}
	for _, obj := range result.Objects {
		byKey[obj.Key] = obj.Size
	}
	if byKey["fresh"] != 1234 {
		t.Errorf("fresh: expected size 1234 from version-matched cache, got %d", byKey["fresh"])
	}
	if byKey["stale"] != 1234 {
		t.Errorf("stale: expected size 1234 from HEAD after version mismatch, got %d", byKey["stale"])
	}
	if n := be.headCallCount(); n != 1 {
		t.Fatalf("expected exactly 1 HEAD (the stale-generation key), got %d", n)
	}
	if got := metricOutcome(t, h.metrics, "cache"); got != 1 {
		t.Errorf("expected cache outcome = 1, got %d", got)
	}
	if got := metricOutcome(t, h.metrics, "head"); got != 1 {
		t.Errorf("expected head outcome = 1, got %d", got)
	}
}

// TestListEnrichUnderPrefixHeadsStoredKey proves the enrichment resolves
// manifest keys in client space while HEADing the listed stored key as-is.
// Applying the configured prefix a second time would 404 every HEAD on a
// prefixed deployment and silently report ciphertext sizes.
func TestListEnrichUnderPrefixHeadsStoredKey(t *testing.T) {
	const prefix = "tenant/"
	cfg := &config.Config{
		BlockSize:      65536,
		AuthAccessKey:  "test-access-key",
		AuthSecretKey:  "REMOVED-NOT-A-SECRET-VALUE",
		Prefix:         prefix,
		ListHeadFanout: 4,
	}
	be := newEnrichListBackend([]string{prefix + "a", prefix + "b"}, 1234)

	// Part 1: manifest coverage (client-space keys) means zero HEADs.
	h := New(cfg, be, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), nil, nil)
	rec := &enrichManifest{entries: make(map[string]*ManifestEntry)}
	rec.entries["test-bucket/a"] = &ManifestEntry{PlaintextSize: 1234}
	rec.entries["test-bucket/b"] = &ManifestEntry{PlaintextSize: 1234}
	h.WithManifest(rec)

	result := &backend.ListResult{Objects: []backend.ObjectInfo{
		{Key: prefix + "a", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-" + prefix + "a"},
		{Key: prefix + "b", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-" + prefix + "b"},
	}}
	h.enrichPlaintextSizes(context.Background(), "test-bucket", result)
	if n := be.headCallCount(); n != 0 {
		t.Fatalf("prefixed deployment with index coverage: expected 0 HEADs, got %d", n)
	}
	for _, obj := range result.Objects {
		if obj.Size != 1234 {
			t.Errorf("key %s: expected plaintext size 1234 from index, got %d", obj.Key, obj.Size)
		}
	}

	// Part 2: no manifest coverage — HEAD must use the stored key, not a
	// double-prefixed one, and still resolve the plaintext size.
	h2 := New(cfg, be, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), nil, nil)
	result2 := &backend.ListResult{Objects: []backend.ObjectInfo{
		{Key: prefix + "a", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-" + prefix + "a"},
		{Key: prefix + "b", Size: 1234 + enrichCiphertextOverhead, ETag: "stored-" + prefix + "b"},
	}}
	h2.enrichPlaintextSizes(context.Background(), "test-bucket", result2)

	for _, k := range be.requestedKeys {
		if strings.HasPrefix(k, prefix+prefix) {
			t.Errorf("HEAD was called with double-prefixed key %q", k)
		}
		if k != prefix+"a" && k != prefix+"b" {
			t.Errorf("HEAD was called with unexpected key %q", k)
		}
	}
	if n := be.headCallCount(); n != 2 {
		t.Fatalf("expected 2 HEADs (one per uncovered key), got %d", n)
	}
	for _, obj := range result2.Objects {
		if obj.Size != 1234 {
			t.Errorf("key %s: expected plaintext size 1234 from HEAD, got %d", obj.Key, obj.Size)
		}
	}
}

// TestHeadObjectWarmsCacheForListEnrichment runs end to end: a PUT followed by
// a HeadObject warms the version-gated metadata cache, and a subsequent LIST
// resolves the size from that cache without issuing a single backend HEAD.
func TestHeadObjectWarmsCacheForListEnrichment(t *testing.T) {
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("keymanager.New: %v", err)
	}
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}
	be := newEnrichStore()
	cache := backend.NewMetadataCache(1000, 300)
	h := New(cfg, be, cache, backend.NewFooterCache(1000, 300), km, nil)
	h.WithMetrics(metrics.NewMetrics())

	// A stale cached generation is left beside the key before the PUT; the
	// PUT must retire it.
	cache.SetVersioned("test-bucket", "warm-me", "pre-put-generation", &backend.ARMORMetadata{Version: 2, PlaintextSize: 1})

	plaintext := []byte("warm the cache for listing")
	req := httptest.NewRequest(http.MethodPut, "/test-bucket/warm-me", bytes.NewReader(plaintext))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT failed: %d %s", w.Code, w.Body.String())
	}

	if cache.Size() != 0 {
		t.Fatalf("expected PUT to retire the cached previous generation, %d entries remain", cache.Size())
	}

	// HeadObject warms the cache bound to the stored generation.
	req = httptest.NewRequest(http.MethodHead, "/test-bucket/warm-me", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("HEAD failed: %d %s", w.Code, w.Body.String())
	}
	if n := be.headCallsViaStore(); n != 1 {
		t.Fatalf("expected 1 backend HEAD from HeadObject, got %d", n)
	}
	headCallsAfterWarm := be.headCallsViaStore()

	// LIST now resolves the size from the cache: no additional backend HEAD,
	// and the response reports the plaintext size, not the envelope size.
	req = httptest.NewRequest(http.MethodGet, "/test-bucket?list-type=2", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("LIST failed: %d %s", w.Code, w.Body.String())
	}
	if be.headCallsViaStore() != headCallsAfterWarm {
		t.Errorf("expected no backend HEADs during LIST (cache hit), got %d extra", be.headCallsViaStore()-headCallsAfterWarm)
	}
	if !strings.Contains(w.Body.String(), fmt.Sprintf("<Size>%d</Size>", len(plaintext))) {
		t.Errorf("expected LIST to report plaintext size %d, got: %s", len(plaintext), w.Body.String())
	}
	if got := metricOutcome(t, h.metrics, "cache"); got < 1 {
		t.Errorf("expected at least one cache outcome, got %d", got)
	}
}
