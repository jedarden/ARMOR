// Package server tests for GET /admin/key/ring, driven through the real
// admin mux and its bearer-token gate.
//
// The endpoint's default census is in-memory: it reads the MEK fingerprint
// each manifest entry records at PUT time instead of HEADing the backend.
// These tests pin the two properties that behavior exists for — the response
// completes promptly while writes land concurrently, and it never issues a
// single backend call — plus the attribution rules and the authoritative
// ?census=head walk (armor-5023fc92).
package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
)

const keyRingTestBucket = "key-ring-test"

// bytes32 returns a 32-byte MEK with every byte set to b.
func bytes32(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

// slowHeadBackend wraps mockRotationBackend, charging a delay per Head and
// counting the calls. The count is what discriminates the censuses: the
// manifest census must issue zero Heads, the head census exactly one per
// manifest entry regardless of how many keys are configured.
type slowHeadBackend struct {
	*mockRotationBackend
	headDelay time.Duration
	headCalls atomic.Int64
}

func (b *slowHeadBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	b.headCalls.Add(1)
	time.Sleep(b.headDelay)
	return b.mockRotationBackend.Head(ctx, bucket, key)
}

// keyRingResponse mirrors the /admin/key/ring body.
type keyRingResponse struct {
	Keys map[string]struct {
		ActiveFingerprint    string         `json:"active_fp"`
		RingFingerprints     []string       `json:"ring_fps"`
		ObjectsByFingerprint map[string]int `json:"objects_by_fp"`
	} `json:"keys"`
	Census                    string         `json:"census"`
	ManifestObjects           int            `json:"manifest_objects"`
	HeadFailures              int            `json:"head_failures"`
	UnattributedByFingerprint map[string]int `json:"unattributed_objects_by_fp"`
	Approximate               bool           `json:"approximate"`
	Note                      string         `json:"note"`
	RawBody                   string         `json:"-"`
}

// keyRingFixture holds a server seeded with objects under the active MEK, a
// ring MEK, a legacy (v1, fingerprintless) entry, a fingerprint no key
// holds, and a manifest entry whose object is gone from the backend.
type keyRingFixture struct {
	srv        *httptest.Server
	idx        *manifest.Index
	backend    *slowHeadBackend
	activeFP   string
	oldFP      string
	retiredFP  string
	activeObjs int
	oldObjs    int
}

// keyRingSeededEntries is the number of manifest entries startKeyRingServer
// seeds: 7 tracked objects plus the vanished entry.
const keyRingSeededEntries = 8

// startKeyRingServer seeds the fixture. Objects are written through
// putARMORObject so the stored metadata and the manifest entry carry the
// same fingerprint, exactly as the production PUT path records them.
func startKeyRingServer(t *testing.T) *keyRingFixture {
	t.Helper()

	activeMEK := bytes32(0xa1)
	oldMEK := bytes32(0xb2)
	retiredMEK := bytes32(0xc3)

	mock := newMockRotationBackend()
	slow := &slowHeadBackend{mockRotationBackend: mock, headDelay: 100 * time.Millisecond}
	idx := manifest.New()

	f := &keyRingFixture{
		idx:       idx,
		backend:   slow,
		activeFP:  crypto.MEKFingerprint(activeMEK),
		oldFP:     crypto.MEKFingerprint(oldMEK),
		retiredFP: crypto.MEKFingerprint(retiredMEK),
	}

	for i := 0; i < 2; i++ {
		putARMORObject(t, mock, keyRingTestBucket, fmt.Sprintf("active-%d.txt", i), []byte("data"), activeMEK, idx)
	}
	f.activeObjs = 2
	for i := 0; i < 3; i++ {
		putARMORObject(t, mock, keyRingTestBucket, fmt.Sprintf("old-%d.txt", i), []byte("data"), oldMEK, idx)
	}
	f.oldObjs = 3

	// A v1-format object: plain-base64 wrapped DEK, no version header, so
	// neither the stored metadata nor the manifest entry carries a
	// fingerprint. It must count under "legacy".
	putLegacyObject(t, mock, keyRingTestBucket, "legacy.txt", []byte("data"), activeMEK, idx)

	// An object whose fingerprint no longer belongs to any key or ring —
	// the state right after a fingerprint is retired post-rotation.
	putARMORObject(t, mock, keyRingTestBucket, "retired.txt", []byte("data"), retiredMEK, idx)

	// A manifest entry whose object is gone from the backend: the
	// transient-Head-failure shape that used to undercount silently.
	idx.Put(keyRingTestBucket, "vanished.txt", &manifest.Entry{
		MEKFingerprint: f.activeFP,
		LastModified:   time.Now(),
	})

	cfg := &config.Config{
		Bucket:     keyRingTestBucket,
		BlockSize:  65536,
		MEK:        activeMEK,
		AdminToken: "key-ring-admin-token",
		// The old MEK is retired into the default key's ring, the state a
		// mid-rotation deployment is in. KeyRings values are concatenated
		// raw 32-byte MEKs; the hex decoding happens in config parsing.
		KeyRings: map[string][]byte{"default": append([]byte(nil), oldMEK...)},
	}
	armorServer, err := NewWithBackend(cfg, slow)
	if err != nil {
		t.Fatalf("create ARMOR server: %v", err)
	}
	// NewWithBackend leaves the manifest nil on purpose (the S3 data plane
	// does not need it); the ring census is precisely the admin feature
	// that does.
	armorServer.manifest = idx

	f.srv = httptest.NewServer(armorServer.AdminHandler())
	t.Cleanup(f.srv.Close)
	return f
}

// putLegacyObject writes a v1-format object: the wrapped DEK is plain
// base64 with no version header, and the manifest entry records an empty
// MEKFingerprint — the pre-v2 shape still readable through the ring.
func putLegacyObject(t *testing.T, mock *mockRotationBackend, bucket, key string, plaintext, mek []byte, idx *manifest.Index) {
	t.Helper()

	dek, err := crypto.GenerateDEK()
	if err != nil {
		t.Fatalf("GenerateDEK: %v", err)
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		t.Fatalf("GenerateIV: %v", err)
	}
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		t.Fatalf("WrapDEK: %v", err)
	}

	meta := map[string]string{
		"x-amz-meta-armor-block-size":       "65536",
		"x-amz-meta-armor-plaintext-size":   fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-content-type":     "application/octet-stream",
		"x-amz-meta-armor-iv":               base64.StdEncoding.EncodeToString(iv),
		"x-amz-meta-armor-wrapped-dek":      base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-plaintext-sha256": "test-sha256",
		"x-amz-meta-armor-etag":             "test-etag",
	}

	var buf bytes.Buffer
	buf.Write(plaintext)
	if err := mock.Put(context.Background(), bucket, key, &buf, int64(len(plaintext)), meta); err != nil {
		t.Fatalf("Put: %v", err)
	}

	am, ok := backend.ParseARMORMetadata(meta)
	if !ok {
		t.Fatal("failed to parse legacy ARMOR metadata")
	}
	if am.MEKFingerprint != "" {
		t.Fatal("legacy metadata must not carry a fingerprint")
	}
	idx.Put(bucket, key, &manifest.Entry{
		PlaintextSize:  am.PlaintextSize,
		ContentType:    am.ContentType,
		ETag:           am.ETag,
		IV:             am.IV,
		WrappedDEK:     am.WrappedDEK,
		MEKFingerprint: am.MEKFingerprint,
		BlockSize:      am.BlockSize,
		LastModified:   time.Now(),
	})
}

// getRing calls GET /admin/key/ring with the given query string.
func (f *keyRingFixture) getRing(t *testing.T, query string) (*http.Response, keyRingResponse) {
	t.Helper()

	url := f.srv.URL + "/admin/key/ring"
	if query != "" {
		url += "?" + query
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer key-ring-admin-token")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /admin/key/ring: %v", err)
	}
	defer resp.Body.Close()

	body := readAllBody(t, resp)
	var got keyRingResponse
	got.RawBody = string(body)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode response (status %d, body %q): %v", resp.StatusCode, got.RawBody, err)
	}
	return resp, got
}

func readAllBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return body
}

// TestKeyRingPromptUnderConcurrentWrites is the armor-5023fc92 regression:
// the endpoint must answer within a bounded time while manifest entries land
// concurrently, and the default census must not touch the backend at all.
// The pre-fix handler HEADed every entry (here 100 ms apiece) once per key
// and sent nothing until the walk finished — 8 entries would mean ≥800 ms of
// silent, headerless connection time and 8 backend calls; the in-memory
// census is instant and issues none.
func TestKeyRingPromptUnderConcurrentWrites(t *testing.T) {
	f := startKeyRingServer(t)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		extra := &manifest.Entry{LastModified: time.Now()}
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			extra.MEKFingerprint = f.activeFP
			f.idx.Put(keyRingTestBucket, fmt.Sprintf("concurrent-%d.txt", i), extra)
			time.Sleep(time.Millisecond)
		}
	}()
	defer close(stop)

	start := time.Now()
	resp, got := f.getRing(t, "")
	elapsed := time.Since(start)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", resp.StatusCode, got.RawBody)
	}
	if elapsed > 5*time.Second {
		t.Errorf("GET took %v; the endpoint must answer within a bounded time even while writes land", elapsed)
	}
	if calls := f.backend.headCalls.Load(); calls != 0 {
		t.Errorf("default census issued %d backend Heads; it must build the histogram from the manifest alone", calls)
	}
	if got.Census != "manifest" {
		t.Errorf("census = %q, want %q", got.Census, "manifest")
	}
	if got.ManifestObjects < keyRingSeededEntries {
		t.Errorf("manifest_objects = %d, want >= %d (the seeded entries)", got.ManifestObjects, keyRingSeededEntries)
	}
	// The concurrent writer keeps adding active-fingerprint entries, so the
	// seeded count is a floor, not an exact count.
	if n := got.Keys["default"].ObjectsByFingerprint[f.activeFP]; n < f.activeObjs {
		t.Errorf("objects_by_fp[%s] = %d, want >= %d", f.activeFP, n, f.activeObjs)
	}
}

// TestKeyRingManifestCensusAttribution pins the fast path's attribution
// rules: ring membership decides the key, empty fingerprints count as
// "legacy" on the default key, and fingerprints matching no configured key
// are reported unattributed rather than dropped.
func TestKeyRingManifestCensusAttribution(t *testing.T) {
	f := startKeyRingServer(t)

	resp, got := f.getRing(t, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	def := got.Keys["default"]
	if def.ActiveFingerprint != f.activeFP {
		t.Errorf("active_fp = %q, want %q", def.ActiveFingerprint, f.activeFP)
	}
	if len(def.RingFingerprints) != 1 || def.RingFingerprints[0] != f.oldFP {
		t.Errorf("ring_fps = %v, want [%s]", def.RingFingerprints, f.oldFP)
	}

	// The vanished entry carries the active fingerprint in the manifest, so
	// the manifest census counts it alongside the two seeded objects.
	if n := def.ObjectsByFingerprint[f.activeFP]; n != f.activeObjs+1 {
		t.Errorf("objects_by_fp[active] = %d, want %d (seeded + the vanished entry)", n, f.activeObjs+1)
	}
	if n := def.ObjectsByFingerprint[f.oldFP]; n != f.oldObjs {
		t.Errorf("objects_by_fp[ring] = %d, want %d", n, f.oldObjs)
	}
	if n := def.ObjectsByFingerprint["legacy"]; n != 1 {
		t.Errorf("objects_by_fp[legacy] = %d, want 1", n)
	}
	// The retired fingerprint matches no configured key, so it must not
	// pretend to belong to the default key's histogram.
	if n, ok := def.ObjectsByFingerprint[f.retiredFP]; ok {
		t.Errorf("objects_by_fp[%s] = %d; a fingerprint no key holds must stay out of the key's histogram", f.retiredFP, n)
	}
	if got.UnattributedByFingerprint[f.retiredFP] != 1 {
		t.Errorf("unattributed_objects_by_fp[%s] = %d, want 1", f.retiredFP, got.UnattributedByFingerprint[f.retiredFP])
	}
	if got.ManifestObjects != keyRingSeededEntries {
		t.Errorf("manifest_objects = %d, want %d", got.ManifestObjects, keyRingSeededEntries)
	}
	// No entry may silently vanish: histogram + unattributed must reconcile
	// against the manifest total exactly.
	total := 0
	for _, n := range def.ObjectsByFingerprint {
		total += n
	}
	for _, n := range got.UnattributedByFingerprint {
		total += n
	}
	if total != got.ManifestObjects {
		t.Errorf("histogram total %d != manifest_objects %d; no entry may silently vanish", total, got.ManifestObjects)
	}
	if got.HeadFailures != 0 {
		t.Errorf("head_failures = %d on the manifest census, want 0", got.HeadFailures)
	}
}

// TestKeyRingHeadCensusSingleWalk pins the authoritative ?census=head path:
// one Head per manifest entry (not one per key), counts from live object
// metadata keyed by the metadata key ID, and a visible head_failures counter
// where the old implementation silently skipped.
func TestKeyRingHeadCensusSingleWalk(t *testing.T) {
	f := startKeyRingServer(t)

	resp, got := f.getRing(t, "census=head")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Exactly one Head per manifest entry. The pre-fix code walked the
	// bucket once per configured key; any per-key re-walk pushes this over.
	if calls := f.backend.headCalls.Load(); calls != keyRingSeededEntries {
		t.Errorf("head census issued %d Heads, want exactly %d (one per manifest entry)", calls, keyRingSeededEntries)
	}
	if got.Census != "head" {
		t.Errorf("census = %q, want %q", got.Census, "head")
	}
	if got.ManifestObjects != keyRingSeededEntries {
		t.Errorf("manifest_objects = %d, want %d", got.ManifestObjects, keyRingSeededEntries)
	}
	if got.HeadFailures != 1 {
		t.Errorf("head_failures = %d, want 1 (the vanished object)", got.HeadFailures)
	}

	def := got.Keys["default"]
	if n := def.ObjectsByFingerprint[f.activeFP]; n != f.activeObjs {
		t.Errorf("objects_by_fp[active] = %d, want %d", n, f.activeObjs)
	}
	if n := def.ObjectsByFingerprint[f.oldFP]; n != f.oldObjs {
		t.Errorf("objects_by_fp[ring] = %d, want %d", n, f.oldObjs)
	}
	// In head mode every object is attributed by its own live metadata: the
	// retired object still carries its fingerprint, the legacy object reads
	// as "legacy", and empty key IDs fall to the default key.
	if n, ok := def.ObjectsByFingerprint[f.retiredFP]; !ok || n != 1 {
		t.Errorf("objects_by_fp[%s] = %d (present %v), want 1 — head census reads live metadata", f.retiredFP, n, ok)
	}
	if n := def.ObjectsByFingerprint["legacy"]; n != 1 {
		t.Errorf("objects_by_fp[legacy] = %d, want 1", n)
	}
}

// TestKeyRingManifestDisabled preserves the pre-existing contract when the
// manifest is off: 200, per-key shape intact, empty histograms, and the
// "approximate" note.
func TestKeyRingManifestDisabled(t *testing.T) {
	cfg := &config.Config{
		Bucket:     keyRingTestBucket,
		BlockSize:  65536,
		MEK:        bytes32(0xa1),
		AdminToken: "key-ring-admin-token",
	}
	armorServer, err := NewWithBackend(cfg, newMockRotationBackend())
	if err != nil {
		t.Fatalf("create ARMOR server: %v", err)
	}
	srv := httptest.NewServer(armorServer.AdminHandler())
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL+"/admin/key/ring", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer key-ring-admin-token")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	body := readAllBody(t, resp)
	var got keyRingResponse
	got.RawBody = string(body)
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if !got.Approximate || got.Note == "" {
		t.Errorf("approximate = %v, note = %q; the manifest-disabled note must survive", got.Approximate, got.Note)
	}
	if def, ok := got.Keys["default"]; !ok || def.ObjectsByFingerprint == nil || len(def.ObjectsByFingerprint) != 0 {
		t.Errorf("keys[default] = %+v; want present with an empty objects_by_fp", def)
	}
}
