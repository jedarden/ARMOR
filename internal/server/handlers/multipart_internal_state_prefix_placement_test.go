package handlers_test

// CHARACTERIZATION of a known ADR-001 deviation — not an endorsement.
//
// ADR-001's "Internal Namespaces" addendum says every internal writer resolves
// `.armor/` beneath `<ARMOR_PREFIX>.armor/`, with the provenance chain as the
// only documented exception. The manifest writer composes (2347a039); the
// multipart state writer and the HMAC sidecar writers do not: they build
// literal `.armor/...` keys and the backend never applies the prefix, so on a
// prefixed deployment this state lands at the bucket root. Consequences: a B2
// key scoped to namePrefix <tenant>/ is denied these writes, and sidecar
// sha256(client key) names can collide across tenants sharing a bucket.
//
// Fixing it is not a test-only change — sidecars are persistent read state, so
// the read side needs a dual-location fallback before the write side moves.
// That work is tracked on armor-01f79985; these tests pin today's placement so
// the fix must consciously flip them, and so a regression to some third
// location fails loudly in the meantime.

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

const placementTestPrefix = "tenant-a/"
const placementTestBucket = "test-bucket"

// assertNoComposedInternalWrite fails the test if any backend Put addressed a
// key beneath the ADR-001 composed internal namespace
// <bucket>/<ARMOR_PREFIX>.armor/ — the location the addendum requires and the
// writers tracked on armor-01f79985 do not use yet.
func assertNoComposedInternalWrite(t *testing.T, putKeys []string) {
	t.Helper()
	for _, k := range putKeys {
		if strings.Contains(k, placementTestPrefix+".armor/") {
			t.Errorf("internal state was composed under the tenant namespace: %s", k)
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

// hmacSidecarKey names the sidecar the way both writers do: sha256 of the
// CLIENT key, at the bucket-root .armor/hmac/ namespace.
func hmacSidecarKey(bucket, key string) string {
	sum := sha256.Sum256([]byte(key))
	return bucket + "/.armor/hmac/" + fmt.Sprintf("%x", sum)
}

// TestV2MultipartInternalStatePlacementUnderPrefix pins where the v2 multipart
// flow writes its upload state and HMAC sidecar while ARMOR_PREFIX is set: at
// the bucket-root .armor/ namespace (the deviation), never at the composed
// tenant location, while the assembled object itself IS composed (covered by
// TestMultipartAppliesArmorPrefix).
func TestV2MultipartInternalStatePlacementUnderPrefix(t *testing.T) {
	_, rb, h := recordingTestSetupWithPrefix(t, placementTestPrefix)
	bucket, key := placementTestBucket, "backups/base/data.tar"

	const block = 65536
	part1 := make([]byte, 5*1024*1024) // aligned, pins P
	for i := range part1 {
		part1[i] = byte(i % 251)
	}
	part2 := make([]byte, 3*block) // short final part
	for i := range part2 {
		part2[i] = byte(255 - i%251)
	}

	uploadID := initiateMultipart(t, h, bucket, key)
	e1 := uploadPart(t, h, bucket, key, uploadID, 1, part1)
	e2 := uploadPart(t, h, bucket, key, uploadID, 2, part2)
	completeMultipart(t, h, bucket, key, uploadID, []string{e1, e2})

	putKeys := putKeysSnapshot(t, rb)

	// Upload state: .armor/multipart/<id>.state at the bucket root.
	stateKey := bucket + "/.armor/multipart/" + uploadID + ".state"
	found := false
	for _, k := range putKeys {
		if k == stateKey {
			found = true
		}
	}
	if !found {
		t.Errorf("v2 upload state was never Put at the bucket-root key %s; puts were:\n\t%s",
			stateKey, strings.Join(putKeys, "\n\t"))
	}

	// HMAC sidecar: .armor/hmac/<sha256(client key)> at the bucket root, and
	// it must survive CompleteMultipartUpload's DeleteState cleanup — it is
	// the read path's verification table for the object's lifetime.
	sidecarKey := hmacSidecarKey(bucket, key)
	found = false
	for _, k := range putKeys {
		if k == sidecarKey {
			found = true
		}
	}
	if !found {
		t.Errorf("v2 HMAC sidecar was never Put at the bucket-root key %s", sidecarKey)
	}
	rb.mu.Lock()
	_, sidecarLive := rb.objects[sidecarKey]
	rb.mu.Unlock()
	if !sidecarLive {
		t.Errorf("v2 HMAC sidecar missing at %s after completion — DeleteState removed read state", sidecarKey)
	}

	// The deviation, stated both ways: nothing under the composed internal
	// namespace, and the composed state key specifically never written.
	assertNoComposedInternalWrite(t, putKeys)
	composedStateKey := bucket + "/" + placementTestPrefix + ".armor/multipart/" + uploadID + ".state"
	for _, k := range putKeys {
		if k == composedStateKey {
			t.Errorf("v2 upload state was written at the COMPOSED key %s — characterization out of date; see armor-01f79985", k)
		}
	}
}

// TestV3MultipartInternalStatePlacementUnderPrefix is the v3 sibling: the
// meta.json + part-<n>.json state objects and the HMAC sidecar must sit at the
// bucket-root .armor/ namespace under a prefix (the deviation), matching what
// both v3 read paths load.
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
	metaKey := bucket + "/.armor/multipart/" + uploadID + "/meta.json"
	part1Key := bucket + "/.armor/multipart/" + uploadID + "/part-1.json"
	for _, want := range []string{metaKey, part1Key} {
		found := false
		for _, k := range putKeys {
			if k == want {
				found = true
			}
		}
		if !found {
			t.Errorf("v3 upload state was never Put at the bucket-root key %s", want)
		}
	}

	// HMAC sidecar: same bucket-root naming rule as v2, and it survives.
	sidecarKey := hmacSidecarKey(bucket, key)
	found := false
	for _, k := range putKeys {
		if k == sidecarKey {
			found = true
		}
	}
	if !found {
		t.Errorf("v3 HMAC sidecar was never Put at the bucket-root key %s", sidecarKey)
	}
	rb.mu.Lock()
	_, sidecarLive := rb.objects[sidecarKey]
	rb.mu.Unlock()
	if !sidecarLive {
		t.Errorf("v3 HMAC sidecar missing at %s after completion — DeleteState removed read state", sidecarKey)
	}

	assertNoComposedInternalWrite(t, putKeys)
}
