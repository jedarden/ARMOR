package restoreverifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

// B2 does not persist CreateMultipartUpload S3 metadata onto the finished
// large file (armor-86a90341), so every multipart-completed object on B2
// heads with an EMPTY metadata map and both restore paths used to bail at
// ParseARMORMetadata with "object is not ARMOR-encrypted" before touching any
// crypto — the 10-GiB transcript-archive class of backups was silently
// unverifiable. The ADR-016 manifest sidecar CompleteMultipartUpload writes
// beside the object is the surviving source of the ARMOR parameters, and it is
// what the server's own GET path already prefers (handlers.go, ADR-016). These
// tests prove the verifier resolves parameters from the manifest the same way,
// for v2 and v3 multipart layouts, through both restore paths and through the
// verifyObject/verifyBucket flows; that the declared per-part digest becomes
// enforced (not just dual-path agreement) once the manifest metadata is
// resolved; and that a store with neither metadata nor a manifest still fails
// exactly as before.

const manifestTestPrefix = "forgejo/"

// manifestObjectKeyFor returns the ADR-016 manifest object name for a stored
// key — the stored (prefixed) key with the manifest suffix, no client-key
// stripping: CompleteMultipartUpload names it manifestKeyFor(key) =
// applyPrefix(key) + ".armor-manifest".
func manifestObjectKeyFor(storedKey string) string {
	return storedKey + manifestObjectSuffix
}

// manifestTestSetup builds one v3 or v2 multipart object plus its ADR-016
// manifest, served by a fakeBackend whose object heads with the EMPTY metadata
// map B2 actually returns for a finished large file. manifestHeaders selects
// whether the manifest object's own headers carry the metadata (the normal
// server write) or are empty and only the manifest JSON body does (the
// fallback readManifest also supports).
func manifestTestSetup(t *testing.T, version int, manifestHeaders bool) (*fakeBackend, *Verifier, string, []byte, map[string]string) {
	t.Helper()

	const (
		bucket    = "test-bucket"
		blockSize = 4096
	)
	// The key ends in .sqlite so inferArtifactType (driven by the key, before
	// any metadata is resolved) selects the SQLite assertion, which the
	// valid.sqlite fixture satisfies.
	storedKey := manifestTestPrefix + "repos/jedarden/proj/archive.sqlite"
	clientKey := strings.TrimPrefix(storedKey, manifestTestPrefix)
	mek := bytes.Repeat([]byte{0xA5}, 32)
	plaintext := fixture(t, "valid.sqlite")

	var ciphertext, sidecarJSON []byte
	var meta map[string]string
	switch version {
	case 3:
		ciphertext, sidecarJSON, meta = armorEncryptV3Multipart(t, mek, blockSize, plaintext)
	case 2:
		ciphertext, sidecarJSON, meta = armorEncryptMultipart(t, mek, blockSize, clientKey, plaintext)
	default:
		t.Fatalf("unsupported fixture version %d", version)
	}

	manifestMeta := meta
	if !manifestHeaders {
		// The manifest object's own headers arrive empty; only the JSON body
		// carries the map.
		manifestMeta = nil
	}
	manifestJSON, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: storedKey,
		UploadID:         "upload-1",
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		Metadata:         meta,
	})
	if err != nil {
		t.Fatalf("marshal manifest body: %v", err)
	}

	fb := &fakeBackend{
		ciphertext: ciphertext,
		plaintext:  plaintext,
		// The defect under test: the finished large file heads with no
		// x-amz-meta-* keys at all.
		info: &backend.ObjectInfo{
			Key:      storedKey,
			Size:     int64(len(plaintext)),
			Metadata: map[string]string{},
		},
		sidecars: map[string][]byte{sidecarKeyFor(clientKey): sidecarJSON},
		direct: map[string]fakeDirectObject{
			manifestObjectKeyFor(storedKey): {
				info: &backend.ObjectInfo{Key: manifestObjectKeyFor(storedKey), Metadata: manifestMeta},
				body: manifestJSON,
			},
		},
	}

	v := New(fb, mek, nil, blockSize, nil, Config{
		Buckets: []BucketConfig{{Bucket: bucket, Prefix: manifestTestPrefix, Enabled: true}},
	})
	return fb, v, storedKey, plaintext, meta
}

// assertManifestResolutionLookups pins the two naming schemes the manifest
// fallback depends on: the manifest object is addressed by the STORED key (the
// server wrote it under applyPrefix(key)), while the HMAC sidecar is still
// fetched by the CLIENT key (armor-1b272971).
func assertManifestResolutionLookups(t *testing.T, fb *fakeBackend, storedKey, clientKey string, wantPathVisits int) {
	t.Helper()

	wantManifest := manifestObjectKeyFor(storedKey)
	wantSidecar := sidecarKeyFor(clientKey)
	// The sidecar load probes the ADR-003 addendum composed location before
	// the bucket-root fallback, so both names are expected GetDirect keys.
	wantComposedSidecar := backend.GetSidecarKey(manifestTestPrefix, clientKey)
	visits := 0
	for _, got := range fb.sidecarLookups {
		switch got {
		case wantManifest:
			visits++
		case wantSidecar, wantComposedSidecar:
		default:
			t.Fatalf("unexpected GetDirect key %q (want only %q, %q and %q)", got, wantManifest, wantSidecar, wantComposedSidecar)
		}
	}
	if visits != wantPathVisits {
		t.Fatalf("manifest looked up %d times, want %d (one per restore path): %v",
			visits, wantPathVisits, fb.sidecarLookups)
	}
}

// TestRestorePaths_V3Multipart_EmptyObjectMetadata_ManifestFallback is the
// armor-86a90341 regression test: a v3 multipart object whose B2 metadata is
// empty (and which lives on a prefixed bucket, so manifest and sidecar name by
// DIFFERENT keys) restores through both paths via the manifest sidecar.
func TestRestorePaths_V3Multipart_EmptyObjectMetadata_ManifestFallback(t *testing.T) {
	for _, tc := range []struct {
		name            string
		manifestHeaders bool
	}{
		{name: "manifest_headers", manifestHeaders: true},
		{name: "manifest_body_only", manifestHeaders: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fb, v, storedKey, plaintext, _ := manifestTestSetup(t, 3, tc.manifestHeaders)
			ctx := context.Background()

			got, err := v.restoreViaARMOR(ctx, "test-bucket", storedKey)
			if err != nil {
				t.Fatalf("restoreViaARMOR: %v", err)
			}
			if !bytes.Equal(got, plaintext) {
				t.Fatalf("restoreViaARMOR returned %d bytes differing from the %d-byte plaintext", len(got), len(plaintext))
			}

			direct, err := v.restoreViaDirectDecrypt(ctx, "test-bucket", storedKey)
			if err != nil {
				t.Fatalf("restoreViaDirectDecrypt: %v", err)
			}
			if !bytes.Equal(direct, plaintext) {
				t.Fatalf("restoreViaDirectDecrypt returned %d bytes differing from the plaintext", len(direct))
			}

			assertManifestResolutionLookups(t, fb, storedKey, strings.TrimPrefix(storedKey, manifestTestPrefix), 2)
		})
	}
}

// TestRestorePaths_V2Multipart_EmptyObjectMetadata_ManifestFallback covers the
// v1/v2 multipart layout through the same fallback: readMultipartCiphertext
// must take the IV from the manifest metadata and the HMAC table from the v2
// JSON sidecar.
func TestRestorePaths_V2Multipart_EmptyObjectMetadata_ManifestFallback(t *testing.T) {
	fb, v, storedKey, plaintext, _ := manifestTestSetup(t, 2, true)
	ctx := context.Background()

	got, err := v.restoreViaARMOR(ctx, "test-bucket", storedKey)
	if err != nil {
		t.Fatalf("restoreViaARMOR: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("restoreViaARMOR returned %d bytes differing from the plaintext", len(got))
	}

	direct, err := v.restoreViaDirectDecrypt(ctx, "test-bucket", storedKey)
	if err != nil {
		t.Fatalf("restoreViaDirectDecrypt: %v", err)
	}
	if !bytes.Equal(direct, plaintext) {
		t.Fatalf("restoreViaDirectDecrypt returned %d bytes differing from the plaintext", len(direct))
	}

	assertManifestResolutionLookups(t, fb, storedKey, strings.TrimPrefix(storedKey, manifestTestPrefix), 2)
}

// TestRestorePaths_NoManifestNoMetadata_StillRejected pins the pre-existing
// behavior for objects nothing on the store describes: with neither object
// metadata nor a manifest, both paths must still report the object as not
// ARMOR-encrypted rather than inventing parameters.
func TestRestorePaths_NoManifestNoMetadata_StillRejected(t *testing.T) {
	const blockSize = 4096
	storedKey := manifestTestPrefix + "repos/jedarden/proj/plain.bin"
	mek := bytes.Repeat([]byte{0xA5}, 32)

	fb := &fakeBackend{
		info: &backend.ObjectInfo{Key: storedKey, Metadata: map[string]string{}},
	}
	v := New(fb, mek, nil, blockSize, nil, Config{
		Buckets: []BucketConfig{{Bucket: "test-bucket", Prefix: manifestTestPrefix, Enabled: true}},
	})
	ctx := context.Background()

	if _, err := v.restoreViaARMOR(ctx, "test-bucket", storedKey); err == nil || !strings.Contains(err.Error(), "object is not ARMOR-encrypted") {
		t.Fatalf("restoreViaARMOR error = %v, want not-ARMOR-encrypted", err)
	}
	if _, err := v.restoreViaDirectDecrypt(ctx, "test-bucket", storedKey); err == nil || !strings.Contains(err.Error(), "object is not ARMOR-encrypted") {
		t.Fatalf("restoreViaDirectDecrypt error = %v, want not-ARMOR-encrypted", err)
	}
}

// TestVerifyObject_ManifestMetadata_EnforcesDeclaredDigest proves the second
// half of the fix: once the sample's metadata is filled from the manifest, the
// declared combined per-part digest is actually ENFORCED (a matching digest
// passes, a mismatching one is a checksum_error) in both dual and DR-drill
// modes — instead of being invisible because backend.List returns no metadata.
func TestVerifyObject_ManifestMetadata_EnforcesDeclaredDigest(t *testing.T) {
	for _, mode := range []Mode{ModeDual, ModeDRDrill} {
		t.Run(string(mode), func(t *testing.T) {
			fb, v, storedKey, plaintext, meta := manifestTestSetup(t, 3, true)
			ctx := context.Background()

			// Declare the digest the way CompleteMultipartUpload does for a
			// uniform part size P: the combined per-part digest a verifier
			// reproduces by splitting the plaintext at P boundaries.
			declared := backend.ComputeMultipartDigest(plaintext, int64(4096))
			meta["x-amz-meta-armor-plaintext-sha256"] = declared
			manifestJSON, err := json.Marshal(backend.ManifestBody{Metadata: meta})
			if err != nil {
				t.Fatalf("marshal manifest body: %v", err)
			}
			fb.direct[manifestObjectKeyFor(storedKey)] = fakeDirectObject{
				info: &backend.ObjectInfo{Key: manifestObjectKeyFor(storedKey), Metadata: meta},
				body: manifestJSON,
			}

			// obj.Metadata is nil exactly as backend.List returns it (S3
			// ListObjectsV2 carries no user metadata).
			obj := v.withManifestMetadata(ctx, ObjectSample{Key: storedKey, Bucket: "test-bucket", ArtifactType: ArtifactGeneric})
			if obj.Metadata["x-amz-meta-armor-plaintext-sha256"] != declared {
				t.Fatalf("withManifestMetadata did not fill the declared digest: %q", obj.Metadata["x-amz-meta-armor-plaintext-sha256"])
			}

			result := v.verifyObject(ctx, obj, mode)
			if result.Status != StatusPass {
				t.Fatalf("status %q error %q, want pass with the declared digest enforced", result.Status, result.Error)
			}
			if result.ExpectedSHA256 != declared {
				t.Fatalf("ExpectedSHA256 = %q, want the declared digest %q", result.ExpectedSHA256, declared)
			}

			// A declared digest that does not match the restored plaintext must
			// fail the verification — the enforcement the empty listing used to
			// silently skip.
			obj.Metadata["x-amz-meta-armor-plaintext-sha256"] = strings.Repeat("ab", 32)
			result = v.verifyObject(ctx, obj, mode)
			if result.Status != StatusChecksumError {
				t.Fatalf("status %q error %q, want checksum_error for a wrong declared digest", result.Status, result.Error)
			}
		})
	}
}

// TestVerifyBucket_SkipsManifestSidecarsAndVerifiesDataObject drives the full
// bucket loop over a listing that contains the ADR-016 manifest sidecar next
// to its data object — the manifest with a NEWER LastModified, as it is
// written after the ciphertext. The manifest must not be sampled (it is
// internal bookkeeping: verifying it would decrypt manifest JSON as if it were
// ciphertext), and the data object must pass through the manifest-metadata
// fallback even though the listing carries no metadata for it.
func TestVerifyBucket_SkipsManifestSidecarsAndVerifiesDataObject(t *testing.T) {
	fb, v, storedKey, _, meta := manifestTestSetup(t, 3, true)
	ctx := context.Background()

	now := time.Now()
	fb.listObjects = []backend.ObjectInfo{
		{Key: storedKey, Size: 1234, LastModified: now},
		// The manifest is internal bookkeeping that lives OUTSIDE the .armor/
		// namespace and sorts as the most recent object.
		{Key: manifestObjectKeyFor(storedKey), Size: int64(len(meta)) + 128, LastModified: now.Add(time.Second)},
	}

	state := &BucketState{Bucket: "test-bucket"}
	v.verifyBucket(ctx, "test-bucket", state, ModeDual)

	if state.TotalObjects != 1 {
		t.Fatalf("verified %d objects, want exactly 1 (the manifest sidecar must not be sampled)", state.TotalObjects)
	}
	if len(state.RecentResults) != 1 {
		t.Fatalf("%d recent results, want 1", len(state.RecentResults))
	}
	result := state.RecentResults[0]
	if result.Key != storedKey {
		t.Fatalf("verified %q, want the data object %q", result.Key, storedKey)
	}
	if result.Status != StatusPass || result.Path != PathDualMatch {
		t.Fatalf("status %q path %q error %q, want pass/dual_match", result.Status, result.Path, result.Error)
	}
	if result.ExpectedSHA256 == "" {
		t.Fatalf("declared digest empty: the manifest metadata must have been resolved before verification")
	}
}

// TestIsManifestObject pins the suffix predicate the sampling filters rely on:
// only the exact .armor-manifest suffix counts, and an object that merely
// contains the text is a data object.
func TestIsManifestObject(t *testing.T) {
	testCases := []struct {
		key  string
		want bool
	}{
		{"forgejo/a.tar.gz" + manifestObjectSuffix, true},
		{"a.tar.gz" + manifestObjectSuffix, true},
		{"forgejo/armor-manifest/backup.tar.gz", false},
		{"forgejo/a.tar.gz", false},
		{".armor/hmac/abcd", false},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.key), func(t *testing.T) {
			if got := isManifestObject(tc.key); got != tc.want {
				t.Fatalf("isManifestObject(%q) = %v, want %v", tc.key, got, tc.want)
			}
		})
	}
}
