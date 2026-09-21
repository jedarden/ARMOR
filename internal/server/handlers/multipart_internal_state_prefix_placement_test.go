package handlers_test

// Placement contract for ARMOR's internal multipart state under ARMOR_PREFIX.
//
// ADR-001's "Internal Namespaces" addendum says every internal writer resolves
// `.armor/` beneath `<ARMOR_PREFIX>.armor/`, with the provenance chain as the
// only documented exception. These tests pin the placement armor-01f79985
// landed: multipart upload state and HMAC sidecars compose the prefix, and a
// regression to the pre-2026-09-20 bucket-root placement (or to any third
// location) fails loudly here.
//
// The read side is dual-location by design (ADR-003 sidecar addendum):
// sidecars written before the composition — by a prefixed deployment that
// predates the fix, or a bucket that gained its prefix later — still sit at
// the bucket root, and every loader probes the composed location first, then
// the root. TestV2SidecarRootFallbackUnderPrefix pins that fallback; without
// it, moving the write side alone would 500 every multipart GET on such a
// deployment.

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
)

const placementTestPrefix = "tenant-a/"
const placementTestBucket = "test-bucket"

// assertNoRootInternalWrite fails the test if any backend Put addressed a key
// in the bucket-root .armor/ namespace — the pre-2026-09-20 placement. With a
// prefix in force, every internal write must resolve beneath
// <bucket>/<ARMOR_PREFIX>.armor/ (ADR-001 "Internal Namespaces").
func assertNoRootInternalWrite(t *testing.T, bucket string, putKeys []string) {
	t.Helper()
	root := bucket + "/.armor/"
	for _, k := range putKeys {
		if strings.HasPrefix(k, root) {
			t.Errorf("internal state was written at the bucket-root namespace: %s", k)
		}
	}
}

// putKeysSnapshot returns a copy of every key passed to backend.Put so far.
func putKeysSnapshot(t *testing.T, rb *recordingBackend) []string {
	t.Helper()
	rb.rmu.Lock()
	defer rb.rmu.Unlock()
	return append([]string(nil), rb.putKeys...)
}

// storedKey renders the backend-storage key for an internal object the way
// the manager computes it: <bucket>/<prefix-resolved internal path>.
func storedKey(bucket, internalPath string) string {
	return bucket + "/" + internalPath
}

// plaintextFixture builds the two-part v2 upload body the placement tests use:
// an aligned first part (block multiple) and a short final part.
func plaintextFixture() (part1, part2 []byte) {
	const block = 65536
	part1 = make([]byte, 5*1024*1024) // aligned, pins P
	for i := range part1 {
		part1[i] = byte(i % 251)
	}
	part2 = make([]byte, 3*block) // short final part
	for i := range part2 {
		part2[i] = byte(255 - i%251)
	}
	return part1, part2
}

// TestV2MultipartInternalStatePlacementUnderPrefix pins where the v2 multipart
// flow writes its upload state and HMAC sidecar while ARMOR_PREFIX is set:
// beneath the composed tenant namespace <prefix>.armor/ — never at the
// bucket-root .armor/ the pre-armor-01f79985 writers used — while the
// assembled object itself is composed under the plain prefix (covered by
// TestMultipartAppliesArmorPrefix).
func TestV2MultipartInternalStatePlacementUnderPrefix(t *testing.T) {
	_, rb, h := recordingTestSetupWithPrefix(t, placementTestPrefix)
	bucket, key := placementTestBucket, "backups/base/data.tar"

	part1, part2 := plaintextFixture()

	uploadID := initiateMultipart(t, h, bucket, key)
	e1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
	e2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
	completeMultipart(t, h, bucket, key, uploadID, []string{e1, e2})

	putKeys := putKeysSnapshot(t, rb)

	// Upload state: <prefix>.armor/multipart/<id>.state.
	stateKey := storedKey(bucket, placementTestPrefix+".armor/multipart/"+uploadID+".state")
	found := false
	for _, k := range putKeys {
		if k == stateKey {
			found = true
		}
	}
	if !found {
		t.Errorf("v2 upload state was never Put at the composed key %s; puts were:\n\t%s",
			stateKey, strings.Join(putKeys, "\n\t"))
	}

	// HMAC sidecar: <prefix>.armor/hmac/<sha256(prefix+client key)>, and it
	// must survive CompleteMultipartUpload's DeleteState cleanup — it is the
	// read path's verification table for the object's lifetime.
	sidecarKey := storedKey(bucket, backend.GetSidecarKey(placementTestPrefix, key))
	found = false
	for _, k := range putKeys {
		if k == sidecarKey {
			found = true
		}
	}
	if !found {
		t.Errorf("v2 HMAC sidecar was never Put at the composed key %s", sidecarKey)
	}
	rb.mu.Lock()
	_, sidecarLive := rb.objects[sidecarKey]
	rb.mu.Unlock()
	if !sidecarLive {
		t.Errorf("v2 HMAC sidecar missing at %s after completion — DeleteState removed read state", sidecarKey)
	}

	// The old deviation, stated both ways: nothing internal at the bucket
	// root, and the root state key specifically never written.
	assertNoRootInternalWrite(t, bucket, putKeys)
	rootStateKey := storedKey(bucket, ".armor/multipart/"+uploadID+".state")
	for _, k := range putKeys {
		if k == rootStateKey {
			t.Errorf("v2 upload state was written at the bucket-ROOT key %s — placement regressed past armor-01f79985", k)
		}
	}
}

// TestV3MultipartInternalStatePlacementUnderPrefix is the v3 sibling: the
// meta.json + part-<n>.json state objects and the HMAC sidecar must sit
// beneath <prefix>.armor/ under a prefix, matching what the v3 read paths
// load.
func TestV3MultipartInternalStatePlacementUnderPrefix(t *testing.T) {
	const mib = 1024 * 1024
	cfg, rb, h := recordingTestSetupWithPrefix(t, placementTestPrefix)
	cfg.FormatWriteVersion = 3
	bucket, key := placementTestBucket, "backups/base/data.tar"

	parts, _ := v3MultipartFixture([]int{5*mib + 512, 1})

	uploadID := initiateMultipart(t, h, bucket, key)
	etags := uploadV3PartsConcurrently(t, h, bucket, key, uploadID, parts, 4)
	completeMultipart(t, h, bucket, key, uploadID, etags)

	putKeys := putKeysSnapshot(t, rb)

	// v3 upload state: meta.json at initiate, part-<n>.json per part.
	metaKey := storedKey(bucket, placementTestPrefix+".armor/multipart/"+uploadID+"/meta.json")
	part1Key := storedKey(bucket, placementTestPrefix+".armor/multipart/"+uploadID+"/part-1.json")
	for _, want := range []string{metaKey, part1Key} {
		found := false
		for _, k := range putKeys {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("v3 upload state was never Put at the composed key %s", want)
		}
	}

	// HMAC sidecar: same composed naming rule as v2, and it survives.
	sidecarKey := storedKey(bucket, backend.GetSidecarKey(placementTestPrefix, key))
	found := false
	for _, k := range putKeys {
		if k == sidecarKey {
			found = true
		}
	}
	if !found {
		t.Errorf("v3 HMAC sidecar was never Put at the composed key %s", sidecarKey)
	}
	rb.mu.Lock()
	_, sidecarLive := rb.objects[sidecarKey]
	rb.mu.Unlock()
	if !sidecarLive {
		t.Errorf("v3 HMAC sidecar missing at %s after completion — DeleteState removed read state", sidecarKey)
	}

	assertNoRootInternalWrite(t, bucket, putKeys)
}

// TestV2SidecarRootFallbackUnderPrefix pins the read half of the ADR-003
// sidecar addendum: a sidecar that exists ONLY at the pre-2026-09-20
// bucket-root location must still be found. Prefixed deployments accumulated
// root sidecars before the composed location existed (ord-devimprint,
// ARMOR_PREFIX=commitgraph/); a loader without the dual-location fallback
// 500s every multipart GET for exactly those objects.
func TestV2SidecarRootFallbackUnderPrefix(t *testing.T) {
	_, rb, h := recordingTestSetupWithPrefix(t, placementTestPrefix)
	bucket, key := placementTestBucket, "backups/base/data.tar"

	part1, part2 := plaintextFixture()
	plaintext := append(append([]byte{}, part1...), part2...)

	uploadID := initiateMultipart(t, h, bucket, key)
	e1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
	e2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
	completeMultipart(t, h, bucket, key, uploadID, []string{e1, e2})

	// Rewind time: move the sidecar from the composed location to the
	// bucket-root location a pre-fix deployment would have written it to,
	// and leave nothing at the composed key.
	composed := storedKey(bucket, backend.GetSidecarKey(placementTestPrefix, key))
	root := storedKey(bucket, backend.GetSidecarKey("", key))
	rb.mu.Lock()
	sidecarBytes, ok := rb.objects[composed]
	if !ok {
		rb.mu.Unlock()
		t.Fatalf("composed sidecar %s missing after completion; puts were:\n\t%s",
			composed, strings.Join(putKeysSnapshot(t, rb), "\n\t"))
	}
	rb.objects[root] = sidecarBytes
	delete(rb.objects, composed)
	rb.mu.Unlock()

	// The GET must succeed through the root fallback and return the exact
	// plaintext — the end-to-end property the orphaned-sidecar failure mode
	// destroys.
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET with root-only sidecar failed: status %d: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), plaintext) {
		t.Fatalf("root-fallback round-trip mismatch: got %d bytes, want %d", w.Body.Len(), len(plaintext))
	}
}

// TestSidecarLocationsOrder pins the probe order itself: composed location
// first, bucket-root fallback second, and exactly one location when no prefix
// is set (unprefixed deployments are byte-for-byte unchanged).
func TestSidecarLocationsOrder(t *testing.T) {
	composed := backend.GetSidecarKey(placementTestPrefix, "k")
	root := backend.GetSidecarKey("", "k")

	if !strings.HasPrefix(composed, placementTestPrefix+".armor/hmac/") {
		t.Errorf("composed sidecar key %q is not beneath <prefix>.armor/hmac/", composed)
	}
	if composed == root {
		t.Fatalf("composed and root sidecar keys must differ under a prefix: %s", composed)
	}

	// The name hashes prefix+key, so two tenants sharing a bucket (same
	// client key, different prefixes) get distinct sidecars — the bare-key
	// hash let them clobber each other's tables.
	otherTenant := backend.GetSidecarKey("tenant-b/", "k")
	if otherTenant == composed {
		t.Errorf("sidecar names collide across tenants: %s", composed)
	}

	locs := backend.SidecarLocations(placementTestPrefix, "k")
	if len(locs) != 2 || locs[0] != composed || locs[1] != root {
		t.Errorf("SidecarLocations under prefix = %v, want [%s %s]", locs, composed, root)
	}

	// Without a prefix the two coincide and exactly one location is probed.
	rootOnly := backend.SidecarLocations("", "k")
	if len(rootOnly) != 1 || rootOnly[0] != root {
		t.Errorf("SidecarLocations without prefix = %v, want [%s]", rootOnly, root)
	}
}

// TestSidecarKeyDeterministic guards the name computation against accidental
// input drift: the same (prefix, key) pair must always hash to the same name,
// and the hash input is prefix+key — not the key alone.
func TestSidecarKeyDeterministic(t *testing.T) {
	a := backend.GetSidecarKey(placementTestPrefix, "backups/base/data.tar")
	b := backend.GetSidecarKey(placementTestPrefix, "backups/base/data.tar")
	if a != b {
		t.Errorf("sidecar key not deterministic: %s vs %s", a, b)
	}
	legacy := backend.GetSidecarKey("", "backups/base/data.tar")
	if a == legacy {
		t.Errorf("prefixed sidecar name equals the legacy root name; prefix not hashed into it")
	}
	if strings.Contains(a, "//") {
		t.Errorf("sidecar key %q contains a double slash — prefix normalization invariant broken", a)
	}
}
