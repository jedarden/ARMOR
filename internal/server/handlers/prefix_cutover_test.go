package handlers_test

// ADR-001 "Internal Namespaces" cutover, at the object level.
//
// A deployment that has been writing at the bucket root arms ARMOR_PREFIX.
// The prefix rewrites where ARMOR stores everything — client objects and
// internal state alike — and it is applied unconditionally in both
// directions: after arming, a client key resolves to <prefix><key> and no
// read path probes the unprefixed location. The pre-prefix bytes are still
// in the bucket and still decryptable, so the cutover is safe exactly as
// long as the operator performs the data move the runbook
// (docs/runbooks/prefix-cutover-and-legacy-state.md) prescribes: a
// bucket-side, byte-preserving relocation of every pre-prefix object to its
// prefixed name — client objects included, plus the HMAC sidecars of
// multipart objects, whose names hash the prefix in (ADR-003 addendum).
//
// These tests walk that sequence against one shared mock backend per test:
// write pre-prefix, arm the prefix, observe the pre-move gap, perform the
// move, and verify both eras of objects read, list, and write correctly
// through the prefixed deployment.

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// prefixCutoverPrefix is the ARMOR_PREFIX the cutover arms.
const prefixCutoverPrefix = "tenant/"

// prefixCutoverEra is one shared store plus the MEK every deployment on it
// uses, so objects written before the cutover decrypt after it. The store is
// a recordingBackend because its multipart completion actually assembles the
// object into the key-value view; the plain mockBackend stub does not.
type prefixCutoverEra struct {
	t      *testing.T
	mb     *recordingBackend
	km     *keymanager.KeyManager
	bucket string
}

func newPrefixCutoverEra(t *testing.T) *prefixCutoverEra {
	t.Helper()

	mb := newRecordingBackend()
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	return &prefixCutoverEra{t: t, mb: mb, km: km, bucket: "test-bucket"}
}

// handlersAt returns a deployment on the shared store with the given prefix —
// the same store and MEK across the cutover, which is what makes the two
// eras one deployment's history rather than two unrelated ones.
func (e *prefixCutoverEra) handlersAt(prefix string) *handlers.Handlers {
	e.t.Helper()

	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		Prefix:        normalizeTestPrefix(prefix),
	}
	return handlers.New(cfg, e.mb, backend.NewMetadataCache(1000, 300), backend.NewFooterCache(1000, 300), e.km, nil)
}

func (e *prefixCutoverEra) put(h *handlers.Handlers, key string, body []byte) {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodPut, "/"+e.bucket+"/"+key, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		e.t.Fatalf("PUT %s: status %d: %s", key, w.Code, w.Body.String())
	}
}

func (e *prefixCutoverEra) get(h *handlers.Handlers, key string) (int, []byte) {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/"+e.bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	return w.Code, w.Body.Bytes()
}

// object returns the raw stored bytes at a bucket-relative key, whatever
// namespace they sit in — the operator's bucket-side view, not a client's.
func (e *prefixCutoverEra) object(key string) ([]byte, bool) {
	e.t.Helper()

	e.mb.mu.Lock()
	defer e.mb.mu.Unlock()
	data, ok := e.mb.objects[e.bucket+"/"+key]
	return data, ok
}

func (e *prefixCutoverEra) storedKeys() []string {
	e.t.Helper()

	e.mb.mu.Lock()
	defer e.mb.mu.Unlock()
	keys := make([]string, 0, len(e.mb.objects))
	for k := range e.mb.objects {
		keys = append(keys, k)
	}
	return keys
}

// deleteStored removes one stored object directly — the root-copy retirement
// step, done bucket-side exactly as the runbook prescribes.
func (e *prefixCutoverEra) deleteStored(key string) {
	e.t.Helper()

	if err := e.mb.Delete(context.Background(), e.bucket, key); err != nil {
		e.t.Fatalf("bucket-side delete of %s: %v", key, err)
	}
}

// moveStoredObject relocates one stored object byte-for-byte, metadata
// included — the bucket-side move the runbook prescribes. The source is left
// in place: root copies are retired only after validation succeeds.
func (e *prefixCutoverEra) moveStoredObject(from, to string) {
	e.t.Helper()

	if err := e.mb.Copy(context.Background(), e.bucket, from, e.bucket, to, nil, false); err != nil {
		e.t.Fatalf("bucket-side move %s -> %s: %v", from, to, err)
	}
}

// moveLegacySidecar renames a pre-prefix multipart HMAC sidecar to the name
// the prefixed deployment looks up. The name hashes the prefix in, so the
// move is a rename to a recomputed key — the root name is removed, leaving
// nothing for the root fallback to resolve the moved object through.
func (e *prefixCutoverEra) moveLegacySidecar(clientKey string) {
	e.t.Helper()

	root := backend.GetSidecarKey("", clientKey)
	e.moveStoredObject(root, backend.GetSidecarKey(prefixCutoverPrefix, clientKey))
	e.deleteStored(root)
}

// listKeys returns the client-visible keys a ListObjectsV2 through h reports.
// The real backends hide the reserved .armor/ namespace from listings before
// the handler ever sees a key (internal/backend's isInternalKey); the mock
// performs no such filtering, and neither backend hides the ADR-016
// <key>.armor-manifest multipart sidecars, so this applies the same
// predicates to the handler output to assert what clients actually get.
func (e *prefixCutoverEra) listKeys(h *handlers.Handlers) []string {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/"+e.bucket+"?list-type=2", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		e.t.Fatalf("ListObjectsV2: status %d: %s", w.Code, w.Body.String())
	}
	var result struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		e.t.Fatalf("ListObjectsV2: parse XML: %v (body %s)", err, w.Body.String())
	}
	keys := make([]string, 0, len(result.Contents))
	for _, c := range result.Contents {
		k := c.Key
		internal := strings.HasPrefix(k, ".armor/") ||
			strings.HasPrefix(k, prefixCutoverPrefix+".armor/") ||
			strings.HasSuffix(k, ".armor-manifest")
		if internal {
			continue
		}
		keys = append(keys, k)
	}
	return keys
}

// prefixCutoverParts builds the two-part upload body the cutover tests use:
// a uniform 5 MiB part (the ADR-015 minimum) and a short final part, the
// shape a real multipart object has.
func prefixCutoverParts() (part1, part2 []byte) {
	const block = 65536
	part1 = make([]byte, 5*1024*1024)
	for i := range part1 {
		part1[i] = byte(i % 251)
	}
	part2 = make([]byte, 3*block)
	for i := range part2 {
		part2[i] = byte(255 - i%251)
	}
	return part1, part2
}

// TestPrefixCutoverLegacyObjectReadsAfterDataMove covers a pre-prefix
// single-part object across the cutover: unreachable between arming the
// prefix and moving the data, byte-identical afterwards, and still served
// once the root copy is retired — the prefixed copy is the serving one.
func TestPrefixCutoverLegacyObjectReadsAfterDataMove(t *testing.T) {
	era := newPrefixCutoverEra(t)
	const key = "reports/q3.csv"
	body := bytes.Repeat([]byte("pre-prefix era payload \x00\x01\x02."), 4096) // crosses block boundaries

	era.put(era.handlersAt(""), key, body)
	if _, ok := era.object(key); !ok {
		t.Fatalf("pre-prefix object was not stored at the bucket root key %s", key)
	}

	// Cutover: the same store, now served with the prefix armed.
	prefixed := era.handlersAt(prefixCutoverPrefix)

	// Between arming and moving, the object is unreachable by its client
	// name — the prefix rewrites the lookup and nothing probes the legacy
	// location. This gap is why arming the prefix and moving the data are
	// one rollout, not two.
	if code, _ := era.get(prefixed, key); code != http.StatusNotFound {
		t.Fatalf("pre-move GET returned %d, want 404 — a prefixed deployment must not silently resolve the legacy location", code)
	}
	if _, ok := era.object(key); !ok {
		t.Fatal("legacy object vanished from the bucket root before any move was made")
	}

	era.moveStoredObject(key, prefixCutoverPrefix+key)

	code, got := era.get(prefixed, key)
	if code != http.StatusOK {
		t.Fatalf("post-move GET failed: status %d: %s", code, got)
	}
	if !bytes.Equal(got, body) {
		t.Fatalf("post-move round-trip mismatch: got %d bytes, want %d", len(got), len(body))
	}

	// Retire the root copy; the prefixed copy keeps serving the object.
	era.deleteStored(key)
	if code, got = era.get(prefixed, key); code != http.StatusOK || !bytes.Equal(got, body) {
		t.Fatalf("GET after root-copy retirement: status %d, %d bytes — the prefixed copy must be the serving one", code, len(got))
	}
}

// TestPrefixCutoverMultipartObjectAndSidecarAcrossCutover covers a
// pre-prefix multipart object, whose stored state spans three pieces: the
// assembled ciphertext, the ADR-016 <stored-key>.armor-manifest part-geometry
// sidecar, and the ADR-003 HMAC sidecar. The move must take all three — the
// manifest sidecar under its prefixed name (a plain prefix-prepend, no
// rehash), the HMAC sidecar renamed to the composed location's name, which
// hashes the prefix in — after which the relocated object reads
// byte-identically, part verification included.
func TestPrefixCutoverMultipartObjectAndSidecarAcrossCutover(t *testing.T) {
	era := newPrefixCutoverEra(t)
	const key = "backups/base.tar"
	part1, part2 := prefixCutoverParts()
	plaintext := append(append([]byte{}, part1...), part2...)

	legacy := era.handlersAt("")
	uploadID := initiateMultipart(t, legacy, era.bucket, key)
	e1 := uploadPart(t, legacy, era.bucket, key, uploadID, 1, part1)
	e2 := uploadPart(t, legacy, era.bucket, key, uploadID, 2, part2)
	completeMultipart(t, legacy, era.bucket, key, uploadID, []string{e1, e2})

	rootSidecar := backend.GetSidecarKey("", key)
	if _, ok := era.object(rootSidecar); !ok {
		t.Fatalf("pre-prefix completion left no HMAC sidecar at the root name %s", rootSidecar)
	}
	if _, ok := era.object(key + ".armor-manifest"); !ok {
		t.Fatalf("pre-prefix completion left no ADR-016 manifest sidecar at %s", key+".armor-manifest")
	}

	prefixed := era.handlersAt(prefixCutoverPrefix)

	// The data move takes all three pieces. Moving the ciphertext without
	// its manifest sidecar is not enough: the read path resolves part
	// geometry at <stored-key>.armor-manifest, and with it missing the
	// relocated object decrypts to wrong bytes (verified, not assumed).
	era.moveStoredObject(key, prefixCutoverPrefix+key)
	era.moveStoredObject(key+".armor-manifest", prefixCutoverPrefix+key+".armor-manifest")
	era.deleteStored(key + ".armor-manifest") // the sidecar move is a rename
	era.moveLegacySidecar(key)

	composedSidecar := backend.GetSidecarKey(prefixCutoverPrefix, key)
	if _, ok := era.object(composedSidecar); !ok {
		t.Fatalf("HMAC sidecar not found at the composed name %s after the move", composedSidecar)
	}
	if _, ok := era.object(rootSidecar); ok {
		t.Fatalf("HMAC sidecar still sitting at the root name %s after a rename", rootSidecar)
	}
	if _, ok := era.object(key + ".armor-manifest"); ok {
		t.Fatalf("ADR-016 manifest sidecar still at the root name %s after the move", key+".armor-manifest")
	}

	code, got := era.get(prefixed, key)
	if code != http.StatusOK {
		t.Fatalf("GET of relocated multipart object failed: status %d: %s", code, got)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("relocated multipart object round-trip mismatch: got %d bytes, want %d", len(got), len(plaintext))
	}
}

// TestPrefixCutoverPostPrefixWritesAndBothEraListings covers the post-prefix
// era on a store that has been through the move: new writes land only under
// the prefix (objects, multipart state, and sidecars), listings stay
// prefix-transparent across both eras of client keys, and the reserved
// namespace stays closed to clients.
func TestPrefixCutoverPostPrefixWritesAndBothEraListings(t *testing.T) {
	era := newPrefixCutoverEra(t)
	legacyKeys := []string{"reports/q3.csv", "archive/2025.dump"}

	legacy := era.handlersAt("")
	for _, k := range legacyKeys {
		era.put(legacy, k, []byte("legacy payload of "+k))
	}

	prefixed := era.handlersAt(prefixCutoverPrefix)

	// The data move, then root-copy retirement.
	for _, k := range legacyKeys {
		era.moveStoredObject(k, prefixCutoverPrefix+k)
		era.deleteStored(k)
	}

	// A post-cutover single-part write must land only at the prefixed name.
	const newKey = "reports/q4.csv"
	newBody := bytes.Repeat([]byte{0xA5}, 100*1024)
	era.put(prefixed, newKey, newBody)
	if _, ok := era.object(newKey); ok {
		t.Errorf("post-cutover object stored at the bare root key %s — prefix not applied on write", newKey)
	}
	if _, ok := era.object(prefixCutoverPrefix + newKey); !ok {
		t.Errorf("post-cutover object missing at %s", prefixCutoverPrefix+newKey)
	}
	if code, got := era.get(prefixed, newKey); code != http.StatusOK || !bytes.Equal(got, newBody) {
		t.Fatalf("post-cutover PUT round-trip: status %d, %d bytes", code, len(got))
	}

	// A post-cutover multipart write: sidecar and state land in the
	// prefixed namespace with the object.
	const mpKey = "backups/incremental.tar"
	p1, p2 := prefixCutoverParts()
	uploadID := initiateMultipart(t, prefixed, era.bucket, mpKey)
	pe1 := uploadPart(t, prefixed, era.bucket, mpKey, uploadID, 1, p1)
	pe2 := uploadPart(t, prefixed, era.bucket, mpKey, uploadID, 2, p2)
	completeMultipart(t, prefixed, era.bucket, mpKey, uploadID, []string{pe1, pe2})

	composedSidecar := backend.GetSidecarKey(prefixCutoverPrefix, mpKey)
	if _, ok := era.object(composedSidecar); !ok {
		t.Errorf("post-cutover completion left no sidecar at the composed location %s", composedSidecar)
	}
	wantMP := append(append([]byte{}, p1...), p2...)
	if code, got := era.get(prefixed, mpKey); code != http.StatusOK || !bytes.Equal(got, wantMP) {
		t.Fatalf("post-cutover multipart round-trip: status %d, %d bytes", code, len(got))
	}

	// After the move and retirement, nothing but prefixed keys remains.
	for _, k := range era.storedKeys() {
		if !strings.HasPrefix(k, era.bucket+"/"+prefixCutoverPrefix) {
			t.Errorf("store holds a non-prefixed key after the cutover: %s", k)
		}
	}

	// Listings are prefix-transparent across both eras: every client key
	// exactly once, no tenant prefix, no internal state.
	listed := era.listKeys(prefixed)
	wantListed := map[string]bool{
		"reports/q3.csv":    true,
		"archive/2025.dump": true,
		newKey:              true,
		mpKey:               true,
	}
	if len(listed) != len(wantListed) {
		t.Errorf("listing returned %d keys, want %d: %v", len(listed), len(wantListed), listed)
	}
	for _, k := range listed {
		if !wantListed[k] {
			t.Errorf("unexpected key in listing: %s", k)
		}
		if strings.Contains(k, ".armor/") {
			t.Errorf("internal namespace leaked into a client listing: %s", k)
		}
		if strings.HasPrefix(k, prefixCutoverPrefix) {
			t.Errorf("client-visible key still carries the ARMOR_PREFIX: %s", k)
		}
	}

	// The reserved namespace is refused no matter what the prefix is.
	req := httptest.NewRequest(http.MethodPut, "/"+era.bucket+"/.armor/evil", bytes.NewReader([]byte("x")))
	w := httptest.NewRecorder()
	prefixed.HandleRoot(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("PUT into .armor/ returned %d, want 403", w.Code)
	}
}
