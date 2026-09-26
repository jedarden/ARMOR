package server

// The rollback half of the ARMOR_PREFIX cutover: runbook
// docs/runbooks/prefix-cutover-and-legacy-state.md §7, "Rollback — a data
// move, not an env flip", executed against one real store the way the
// forward cutover suite (prefix_cutover_mixed_era_test.go) executes §1–§6.
// Nothing here re-pins the forward legs it depends on; they are setup.
//
// Two scenarios, both ending on the deployment that un-armed:
//
//   - Inside the pause window (armed, nothing written through it, nothing
//     moved): un-arming restores service to the untouched root state — the
//     runbook's "the root objects never moved out of serving position" —
//     and the composed tree the armed deployment left behind (nothing at
//     all, with no ops enqueued) is ignored rather than ingested. The
//     resumed deployment records into the ROOT manifest tree again.
//
//   - After post-cutover writes: the reverse data move. Every object now
//     stored under the prefix is copied back to its root name — the
//     post-cutover writes AND the forward-moved pre-cutover objects,
//     whose root originals §4 step 8 already retired, so a literal
//     "post-cutover objects only" reading of §7.1 would strand the legacy
//     era on the root namespace it is being rolled back to. Both sidecar
//     flavors rename back with the mirrored three-piece rule: the ADR-016
//     geometry sidecar to the root name with its ciphertext reference
//     rewritten to the root name, the ADR-003 HMAC sidecar to the
//     empty-prefix hash name ("the empty-prefix invocation of the §4
//     recipe reproduces the legacy names"). The prefix is unset last, one
//     step, the coupling §7 imposes; validation (§6 mirrored) runs before
//     the prefixed copies are retired.
//
// The manifest assertions are §7's headline property, pinned from both
// sides: the root manifest tree is never touched and the rolled-back index
// loads exactly it — the abandoned era's composed deltas stay in the
// bucket as dead history and are never ingested, yet the objects they
// describe still serve, because reads are metadata-driven and the index is
// an optimization. The first post-rollback write lands in the root tree.
//
// As in the forward suite, all traffic goes through the authenticated S3
// mux with a real SigV4 client, and the bucket-side moves work through the
// raw FSBackend the way an operator's bucket-side tooling would.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/jedarden/armor/internal/backend"
)

// rollbackPrefix is the ARMOR_PREFIX the scenarios arm and then un-arm.
const rollbackPrefix = "roll/"

// rollbackFixture is one shared filesystem store plus the servers and the
// client handles the rollback scenarios drive it with. The t/cur split and
// every cleanup rule are the forward suite's (see mixedEraFixture): servers,
// writers and httptest listeners register on the PARENT test so a deployment
// booted in one phase stays alive until the sequence ends; assertions use
// the current subtest's handle.
type rollbackFixture struct {
	t   *testing.T // parent test: cleanups
	cur *testing.T // current subtest: assertions

	root   string
	bucket string

	// store is the operator's bucket-side handle — what the §4 and §7
	// data moves work through, never a client path.
	store *backend.FSBackend

	// want holds the exact plaintext of every client key, so the
	// byte-for-byte assertions read as data.
	want map[string][]byte

	// eraCensus is the store-wide hash census taken at the end of the
	// legacy era, before anything prefixed exists — the baseline the
	// rollback scenarios prove came through untouched.
	eraCensus map[string]string
}

func newRollbackFixture(t *testing.T) *rollbackFixture {
	t.Helper()

	root := t.TempDir()
	store, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("bucket-side store handle: %v", err)
	}
	return &rollbackFixture{
		t:      t,
		cur:    t,
		root:   root,
		bucket: "shared-bucket",
		store:  store,
		want:   make(map[string][]byte),
	}
}

// at points the fixture's assertion handle at the calling subtest.
func (f *rollbackFixture) at(t *testing.T) {
	f.cur = t
	f.cur.Helper()
}

// serverAt boots a full deployment over the shared store with the given
// ADR-001 tenant prefix; "" is the un-prefixed (legacy or rolled-back)
// deployment. Cleanups land on the parent test, per mixedEraFixture.
func (f *rollbackFixture) serverAt(tenantPrefix, writerID string) *Server {
	f.t.Helper()
	srv := newManifestTenantServer(f.t, f.root, tenantPrefix, writerID, "")
	f.t.Cleanup(srv.StopManifestCompactor)
	return srv
}

// apiFor exposes one deployment through a real HTTP server and returns a
// SigV4 client signed with the credentials the harness configures.
func (f *rollbackFixture) apiFor(srv *Server) *s3.Client {
	f.cur.Helper()
	ts := httptest.NewServer(srv.Handler())
	f.t.Cleanup(ts.Close)
	return newListingS3Client(f.cur, ts.URL, "test-access-key", "test-secret-key")
}

// --- S3 operations -------------------------------------------------------

// get returns (status, body); non-2xx responses surface their status with a
// nil body instead of failing, so gate assertions can expect 404/500.
func (f *rollbackFixture) get(client *s3.Client, key string) (int, []byte) {
	f.cur.Helper()
	out, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		status, ok := f.statusOf(err)
		if !ok {
			f.cur.Fatalf("GET %s: unrecognized error: %v", key, err)
		}
		return status, nil
	}
	defer out.Body.Close()
	body, err := io.ReadAll(out.Body)
	if err != nil {
		f.cur.Fatalf("GET %s: read body: %v", key, err)
	}
	return http.StatusOK, body
}

// put returns the status of a PutObject, same contract as get.
func (f *rollbackFixture) put(client *s3.Client, key string, body []byte) int {
	f.cur.Helper()
	_, err := client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	if err == nil {
		return http.StatusOK
	}
	status, ok := f.statusOf(err)
	if !ok {
		f.cur.Fatalf("PUT %s: unrecognized error: %v", key, err)
	}
	return status
}

func (f *rollbackFixture) statusOf(err error) (int, bool) {
	f.cur.Helper()
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode(), true
	}
	return 0, false
}

func (f *rollbackFixture) createUpload(client *s3.Client, key string) string {
	f.cur.Helper()
	out, err := client.CreateMultipartUpload(context.Background(), &s3.CreateMultipartUploadInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		f.cur.Fatalf("CreateMultipartUpload %s: %v", key, err)
	}
	return aws.ToString(out.UploadId)
}

func (f *rollbackFixture) uploadPart(client *s3.Client, key, uploadID string, partNumber int32, body []byte) string {
	f.cur.Helper()
	out, err := client.UploadPart(context.Background(), &s3.UploadPartInput{
		Bucket:     aws.String(f.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(partNumber),
		Body:       bytes.NewReader(body),
	})
	if err != nil {
		f.cur.Fatalf("UploadPart %d of %s: %v", partNumber, key, err)
	}
	return strings.Trim(aws.ToString(out.ETag), `"`)
}

func (f *rollbackFixture) complete(client *s3.Client, key, uploadID string, etags []string) int {
	f.cur.Helper()
	parts := make([]types.CompletedPart, len(etags))
	for i, etag := range etags {
		parts[i] = types.CompletedPart{
			PartNumber: aws.Int32(int32(i + 1)),
			ETag:       aws.String(etag),
		}
	}
	_, err := client.CompleteMultipartUpload(context.Background(), &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(f.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: parts,
		},
	})
	if err == nil {
		return http.StatusOK
	}
	status, ok := f.statusOf(err)
	if !ok {
		f.cur.Fatalf("CompleteMultipartUpload %s: unrecognized error: %v", key, err)
	}
	return status
}

// listKeys returns the client-visible keys a ListObjectsV2 through the
// deployment reports — raw handler output, with the real backend
// internal-namespace filter applied.
func (f *rollbackFixture) listKeys(client *s3.Client) []string {
	return f.listKeysScoped(client, "")
}

// listKeysScoped is listKeys with a listing prefix.
func (f *rollbackFixture) listKeysScoped(client *s3.Client, prefix string) []string {
	f.cur.Helper()
	in := &s3.ListObjectsV2Input{Bucket: aws.String(f.bucket)}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	out, err := client.ListObjectsV2(context.Background(), in)
	if err != nil {
		f.cur.Fatalf("ListObjectsV2(prefix=%q): %v", prefix, err)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, o := range out.Contents {
		keys = append(keys, aws.ToString(o.Key))
	}
	return keys
}

// --- bucket-side (operator) operations ----------------------------------

// copyStored is the §4 bulk copy run in reverse in §7: byte-preserving,
// metadata verbatim. Sources are left in place; retirement is explicit.
func (f *rollbackFixture) copyStored(from, to string) {
	f.cur.Helper()
	if err := f.store.Copy(context.Background(), f.bucket, from, f.bucket, to, nil, false); err != nil {
		f.cur.Fatalf("bucket-side copy %s -> %s: %v", from, to, err)
	}
}

// deleteStored retires a copy once the rolled-back namespace validates (§7.4).
func (f *rollbackFixture) deleteStored(key string) {
	f.cur.Helper()
	if err := f.store.Delete(context.Background(), f.bucket, key); err != nil {
		f.cur.Fatalf("bucket-side delete %s: %v", key, err)
	}
}

// storedExists reports whether the store holds an object at a bucket-side key.
// The FS backend surfaces a missing object as a raw os.ErrNotExist chain, so
// classify on that.
func (f *rollbackFixture) storedExists(key string) bool {
	f.cur.Helper()
	_, err := f.store.Head(context.Background(), f.bucket, key)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	f.cur.Fatalf("Head %s: %v", key, err)
	return false
}

// storedKeys enumerates every key in the bucket, internal state included —
// the operator's bucket-side inventory. The one empty key the FS backend can
// surface for an in-flight part's Key-less staging sidecar is skipped; these
// scenarios open no uploads, so any other empty key is a failure.
func (f *rollbackFixture) storedKeys() []string {
	f.cur.Helper()
	res, err := f.store.ListRaw(context.Background(), f.bucket, "", "", "", 10000)
	if err != nil {
		f.cur.Fatalf("ListRaw: %v", err)
	}
	if res.IsTruncated {
		f.cur.Fatal("ListRaw truncated at 10000 keys; raise the page")
	}
	keys := make([]string, 0, len(res.Objects))
	for _, o := range res.Objects {
		if o.Key == "" {
			continue
		}
		keys = append(keys, o.Key)
	}
	return keys
}

// --- sidecar naming and the mirrored rewrite -----------------------------

// sidecarNames names the two sidecar flavors of a multipart object at both
// the root and composed locations: [rootHMAC, composedHMAC, rootGeometry,
// composedGeometry]. The HMAC name hashes the prefix in; the geometry
// sidecar is a prefix-prepend rename beside the object.
func (f *rollbackFixture) sidecarNames(clientKey string) (string, string, string, string) {
	return backend.GetSidecarKey("", clientKey),
		backend.GetSidecarKey(rollbackPrefix, clientKey),
		clientKey + ".armor-manifest",
		rollbackPrefix + clientKey + ".armor-manifest"
}

// rewriteGeometryRef rewrites an ADR-016 geometry sidecar's ciphertext
// reference, asserting it currently names wantCurrent — the §4.5 content
// rewrite and its §7 mirror, one helper for both directions. sidecarKey is
// the stored name the sidecar sits at WHEN it is rewritten: the composed
// name during the forward move, the root name after the rename-back. The
// sidecar's ciphertext_object field (and its x-amz-meta-armor-ciphertext-ref
// metadata) records the ciphertext location as of its last write; until it
// names the copy the reading deployment will resolve, reads silently serve
// the stale location and die with its retirement.
func (f *rollbackFixture) rewriteGeometryRef(sidecarKey, wantCurrent, newRef string) {
	f.cur.Helper()
	rc, info, err := f.store.Get(context.Background(), f.bucket, sidecarKey)
	if err != nil {
		f.cur.Fatalf("bucket-side read of geometry sidecar %s: %v", sidecarKey, err)
	}
	body, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		f.cur.Fatalf("read geometry sidecar %s: %v", sidecarKey, err)
	}
	var manifest backend.ManifestBody
	if err := json.Unmarshal(body, &manifest); err != nil {
		f.cur.Fatalf("parse geometry sidecar %s: %v", sidecarKey, err)
	}
	if manifest.CiphertextObject != wantCurrent {
		f.cur.Fatalf("geometry sidecar %s names ciphertext %q, want %q",
			sidecarKey, manifest.CiphertextObject, wantCurrent)
	}
	manifest.CiphertextObject = newRef
	updated, err := json.Marshal(manifest)
	if err != nil {
		f.cur.Fatalf("re-marshal geometry sidecar %s: %v", sidecarKey, err)
	}
	meta := map[string]string{}
	for k, v := range info.Metadata {
		meta[k] = v
	}
	meta["x-amz-meta-armor-ciphertext-ref"] = manifest.CiphertextObject
	if err := f.store.Put(context.Background(), f.bucket, sidecarKey, bytes.NewReader(updated), int64(len(updated)), meta); err != nil {
		f.cur.Fatalf("bucket-side rewrite of geometry sidecar %s: %v", sidecarKey, err)
	}
}

// --- validation helpers --------------------------------------------------

// assertStoredBytes round-trips a client key through the deployment and
// compares byte-for-byte — §7.4's "every key round-trips".
func (f *rollbackFixture) assertStoredBytes(client *s3.Client, key string) {
	f.cur.Helper()
	want, ok := f.want[key]
	if !ok {
		f.cur.Fatalf("assertStoredBytes: no expected plaintext recorded for %s", key)
	}
	code, got := f.get(client, key)
	if code != http.StatusOK {
		f.cur.Fatalf("GET %s: status %d, want 200", key, code)
	}
	if !bytes.Equal(got, want) {
		f.cur.Fatalf("GET %s: byte mismatch: got %d bytes, want %d", key, len(got), len(want))
	}
}

// writeLegacyEra produces the pre-prefix state both scenarios roll back
// from: one single-part object, one completed multipart object (ciphertext
// + geometry sidecar + HMAC sidecar at the root names), and the flushed
// root manifest deltas. It records the root-era census on the fixture.
func (f *rollbackFixture) writeLegacyEra(t *testing.T, legacy *Server, legacyAPI *s3.Client, legacySingle, legacyMulti string) {
	t.Helper()

	body := mixedEraPattern(3*65536+17, 0x70)
	if code := f.put(legacyAPI, legacySingle, body); code != http.StatusOK {
		t.Fatalf("legacy PUT %s: status %d", legacySingle, code)
	}
	f.want[legacySingle] = body

	p1, p2, plaintext := mixedEraMultipartParts(0x71)
	uploadID := f.createUpload(legacyAPI, legacyMulti)
	e1 := f.uploadPart(legacyAPI, legacyMulti, uploadID, 1, p1)
	e2 := f.uploadPart(legacyAPI, legacyMulti, uploadID, 2, p2)
	if code := f.complete(legacyAPI, legacyMulti, uploadID, []string{e1, e2}); code != http.StatusOK {
		t.Fatalf("legacy complete %s: status %d", legacyMulti, code)
	}
	f.want[legacyMulti] = plaintext

	// The completed multipart object's three pieces sit at the ROOT names.
	rootHMAC, _, rootGeometry, _ := f.sidecarNames(legacyMulti)
	for _, key := range []string{legacyMulti, rootGeometry, rootHMAC} {
		if !f.storedExists(key) {
			t.Fatalf("era-A multipart completion left nothing at bucket-root key %s", key)
		}
	}

	// The era-A deltas land at the root .armor/manifest/ tree — the index a
	// rollback must find exactly where it left it.
	flushTenantManifest(t, legacy)
	assertDirHasFiles(t, filepath.Join(f.root, f.bucket, ".armor", "manifest"),
		"bucket-root .armor/manifest (pre-prefix manifest deltas)")

	f.eraCensus = mixedEraCensus(t, f.root)
}

// TestPrefixCutoverRollbackInsidePauseWindow is §7's first bullet: the
// prefix is armed, nothing has been written through it and nothing has been
// moved, and the operator un-arms. Service returns to the root state with
// no data move at all, and the resumed deployment records into the root
// manifest tree again.
func TestPrefixCutoverRollbackInsidePauseWindow(t *testing.T) {
	f := newRollbackFixture(t)

	const (
		legacySingle = "ledger/accounts.csv"
		legacyMulti  = "archives/nightly-0.tar"
		resumedKey   = "ledger/after-rollback.csv"
	)

	legacy := f.serverAt("", "rb-legacy")
	legacyAPI := f.apiFor(legacy)

	t.Run("the legacy era writes root state", func(t *testing.T) {
		f.at(t)
		f.writeLegacyEra(t, legacy, legacyAPI, legacySingle, legacyMulti)
	})

	t.Run("arming the prefix hides the legacy era", func(t *testing.T) {
		f.at(t)

		armed := f.serverAt(rollbackPrefix, "rb-armed")
		armedAPI := f.apiFor(armed)

		// The armed index comes up empty (the root deltas are not its
		// history) and the legacy keys 404 — the gap that triggers the
		// rollback decision.
		if got := armed.manifest.Len(); got != 0 {
			t.Errorf("armed instance loaded %d manifest entries from a root-era store, want 0; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{legacySingle, legacyMulti} {
			if code, _ := f.get(armedAPI, key); code != http.StatusNotFound {
				t.Errorf("armed GET of %s returned %d, want 404", key, code)
			}
		}
	})

	t.Run("un-arming restores the legacy era in place", func(t *testing.T) {
		f.at(t)

		// §7, pause-window bullet: unset ARMOR_PREFIX, restart. No data
		// move — the root objects never left serving position.
		rb := f.serverAt("", "rb-restored")
		rbAPI := f.apiFor(rb)

		// Every legacy key serves byte-for-byte again.
		for _, key := range []string{legacySingle, legacyMulti} {
			f.assertStoredBytes(rbAPI, key)
		}

		// The index is the root manifest, whole: both era-A entries
		// resolve, and nothing from the composed namespace was ingested.
		if got := rb.manifest.Len(); got != 2 {
			t.Errorf("rolled-back index loaded %d entries, want the era-A 2; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{legacySingle, legacyMulti} {
			if _, ok := rb.manifest.Get(f.bucket, key); !ok {
				t.Errorf("rolled-back index lost era-A key %s", key)
			}
		}

		// Listings show one era: the two client keys plus the multipart
		// object's client-visible geometry sidecar, nothing from either
		// internal namespace, nothing carrying the prefix.
		want := map[string]bool{
			legacySingle:                    true,
			legacyMulti:                     true,
			legacyMulti + ".armor-manifest": true,
		}
		keys := f.listKeys(rbAPI)
		if len(keys) != len(want) {
			t.Errorf("listing returned %d keys, want %d: %v", len(keys), len(want), keys)
		}
		for _, k := range keys {
			if !want[k] {
				t.Errorf("unexpected key in listing: %s", k)
			}
			if strings.Contains(k, ".armor/") || strings.HasPrefix(k, rollbackPrefix) {
				t.Errorf("listing leaked internal or prefixed state: %s", k)
			}
		}

		// The armed deployment's pause-window footprint on the store is
		// nil: every pre-arming byte is exactly where it was, and the
		// composed tree holds nothing to ignore.
		after := mixedEraCensus(t, f.root)
		mixedEraAssertUnchanged(t, f.eraCensus, after, func(string) bool { return true },
			"store during the armed pause")
		for rel := range after {
			if strings.HasPrefix(rel, f.bucket+"/"+rollbackPrefix) {
				t.Errorf("the armed deployment left %s under the composed namespace", rel)
			}
		}
	})

	t.Run("the resumed deployment records into the root manifest again", func(t *testing.T) {
		f.at(t)

		rb := f.serverAt("", "rb-resumed")
		rbAPI := f.apiFor(rb)

		body := mixedEraPattern(4096, 0x72)
		if code := f.put(rbAPI, resumedKey, body); code != http.StatusOK {
			t.Fatalf("resumed PUT %s: status %d", resumedKey, code)
		}
		f.want[resumedKey] = body
		if !f.storedExists(resumedKey) {
			t.Errorf("resumed write missing at the bare root key %s", resumedKey)
		}

		// Its delta flushes into the ROOT manifest tree, beside era A's.
		flushTenantManifest(t, rb)
		assertDirHasFiles(t, filepath.Join(f.root, f.bucket, ".armor", "manifest", "rb-resumed"),
			"root manifest deltas for the resumed deployment")

		// A restart loads the root tree: both era-A keys and the resumed
		// one, still nothing composed.
		again := f.serverAt("", "rb-resumed-2")
		if got := again.manifest.Len(); got != 3 {
			t.Errorf("restarted index loaded %d entries, want 3; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		if _, ok := again.manifest.Get(f.bucket, resumedKey); !ok {
			t.Errorf("restarted index missing the resumed key %s", resumedKey)
		}
	})
}

// TestPrefixCutoverRollbackAfterPostCutoverWrites is §7's second bullet:
// the cutover completed (data moved, roots retired per §4 step 8) and
// post-cutover writes happened, so the rollback is the reverse data move.
func TestPrefixCutoverRollbackAfterPostCutoverWrites(t *testing.T) {
	f := newRollbackFixture(t)

	const (
		legacySingle = "ledger/accounts.csv"
		legacyMulti  = "archives/nightly-0.tar"
		newSingle    = "ledger/q2.csv"
		newMulti     = "archives/weekly-1.tar"
	)
	allClientKeys := []string{legacySingle, legacyMulti, newSingle, newMulti}

	legacy := f.serverAt("", "rb-legacy")
	legacyAPI := f.apiFor(legacy)

	t.Run("the cutover completes and the post-cutover era writes", func(t *testing.T) {
		f.at(t)

		// Era A, then §4 in full: arm, move the three multipart pieces,
		// rewrite the geometry sidecar's reference forward, retire the
		// root originals (§4 step 8).
		f.writeLegacyEra(t, legacy, legacyAPI, legacySingle, legacyMulti)

		armed := f.serverAt(rollbackPrefix, "rb-armed")
		armedAPI := f.apiFor(armed)

		rootHMAC, composedHMAC, rootGeometry, _ := f.sidecarNames(legacyMulti)
		f.copyStored(legacyMulti, rollbackPrefix+legacyMulti)
		f.copyStored(rootGeometry, rollbackPrefix+rootGeometry)
		f.deleteStored(rootGeometry)
		f.copyStored(rootHMAC, composedHMAC)
		f.deleteStored(rootHMAC)
		// §4.5: the moved sidecar still names the pre-cutover root copy;
		// point it at the prefixed one.
		f.rewriteGeometryRef(rollbackPrefix+rootGeometry, legacyMulti, rollbackPrefix+legacyMulti)
		f.copyStored(legacySingle, rollbackPrefix+legacySingle)
		f.deleteStored(legacySingle)
		f.deleteStored(legacyMulti)
		for _, key := range []string{legacySingle, legacyMulti} {
			f.assertStoredBytes(armedAPI, key)
		}

		// Post-cutover writes: born under the prefix, sidecars at the
		// composed names, deltas flushed into the composed manifest tree.
		body := mixedEraPattern(100*1024, 0x73)
		if code := f.put(armedAPI, newSingle, body); code != http.StatusOK {
			t.Fatalf("post-cutover PUT %s: status %d", newSingle, code)
		}
		f.want[newSingle] = body

		p1, p2, plaintext := mixedEraMultipartParts(0x74)
		uploadID := f.createUpload(armedAPI, newMulti)
		e1 := f.uploadPart(armedAPI, newMulti, uploadID, 1, p1)
		e2 := f.uploadPart(armedAPI, newMulti, uploadID, 2, p2)
		if code := f.complete(armedAPI, newMulti, uploadID, []string{e1, e2}); code != http.StatusOK {
			t.Fatalf("post-cutover complete %s: status %d", newMulti, code)
		}
		f.want[newMulti] = plaintext

		_, composedHMACNew, _, composedGeometryNew := f.sidecarNames(newMulti)
		for _, key := range []string{composedHMACNew, composedGeometryNew} {
			if !f.storedExists(key) {
				t.Errorf("post-cutover finalization left no sidecar at the composed name %s", key)
			}
		}

		// §7's coupling gate, forward direction: the final delta flushes
		// BEFORE the prefix is unset.
		flushTenantManifest(t, armed)
		assertDirHasFiles(t, filepath.Join(f.root, f.bucket, rollbackPrefix, ".armor", "manifest", "rb-armed"),
			"composed manifest deltas for the armed deployment")

		for _, key := range allClientKeys {
			f.assertStoredBytes(armedAPI, key)
		}
	})

	t.Run("the reverse move copies everything back and renames the sidecars", func(t *testing.T) {
		f.at(t)

		// §7.1 mirrored, and read as "every object now stored under the
		// prefix": the post-cutover writes AND the forward-moved legacy
		// objects, whose roots §4 step 8 retired — a literal
		// post-cutover-writes-only move would strand the legacy era on
		// the namespace being rolled back to.
		for _, key := range allClientKeys {
			f.copyStored(rollbackPrefix+key, key)
		}

		// §7.2: the sidecars rename back. Geometry sidecars: copy to the
		// root name, retire the composed one, rewrite the ciphertext
		// reference to the root name (the mirrored §4.5). HMAC sidecars:
		// copy to the empty-prefix hash name, retire the composed one.
		for _, key := range []string{legacyMulti, newMulti} {
			rootHMAC, composedHMAC, rootGeometry, composedGeometry := f.sidecarNames(key)
			f.copyStored(composedGeometry, rootGeometry)
			f.deleteStored(composedGeometry)
			f.rewriteGeometryRef(rootGeometry, rollbackPrefix+key, key)
			f.copyStored(composedHMAC, rootHMAC)
			f.deleteStored(composedHMAC)

			if !f.storedExists(rootHMAC) || !f.storedExists(rootGeometry) {
				t.Errorf("sidecars of %s missing at the root names after the rename-back", key)
			}
			if f.storedExists(composedHMAC) || f.storedExists(composedGeometry) {
				t.Errorf("sidecars of %s still at the composed names after a rename-back", key)
			}
		}
	})

	t.Run("the rolled-back deployment serves every key from the root namespace", func(t *testing.T) {
		f.at(t)

		// §7.3: unset ARMOR_PREFIX and restart — one step.
		rb := f.serverAt("", "rb-restored")
		rbAPI := f.apiFor(rb)

		// §7.4 mirrored: every key of both eras round-trips. The
		// post-cutover keys resolve with NO index entry — reads are
		// metadata-driven, the index an optimization.
		for _, key := range allClientKeys {
			f.assertStoredBytes(rbAPI, key)
		}

		// The manifest does not merge on rollback: the index is exactly
		// the root tree era A left behind. The composed deltas sit beside
		// it describing two of the four serving keys — dead history, and
		// provably nobody's index.
		if got := rb.manifest.Len(); got != 2 {
			t.Errorf("rolled-back index loaded %d entries, want exactly the era-A 2 (no composed merge); store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{legacySingle, legacyMulti} {
			if _, ok := rb.manifest.Get(f.bucket, key); !ok {
				t.Errorf("rolled-back index lost era-A key %s", key)
			}
		}
		for _, key := range []string{newSingle, newMulti} {
			if _, ok := rb.manifest.Get(f.bucket, key); ok {
				t.Errorf("rolled-back index resolved %s from the abandoned composed deltas", key)
			}
		}

		// §7.4: listings show one era again. Pre-retirement this is a
		// subset claim: the six root keys of both eras are all present,
		// and neither internal namespace surfaces. The prefixed copies
		// are ALSO client keys at this point — through the un-prefixed
		// deployment a stored key is a client key verbatim — and they
		// disappear with their retirement in the next phase.
		want := []string{
			legacySingle,
			legacyMulti,
			legacyMulti + ".armor-manifest",
			newSingle,
			newMulti,
			newMulti + ".armor-manifest",
		}
		got := f.listKeys(rbAPI)
		present := make(map[string]bool, len(got))
		for _, k := range got {
			present[k] = true
			if strings.Contains(k, ".armor/") {
				t.Errorf("listing leaked internal state: %s", k)
			}
		}
		for _, k := range want {
			if !present[k] {
				t.Errorf("listing missing the root key %s; got %v", k, got)
			}
		}
	})

	t.Run("retiring the prefixed copies leaves one era and the dead composed tree", func(t *testing.T) {
		f.at(t)

		// §7.4, last step: retire the prefixed copies, now that the root
		// namespace validates. The renamed-back sidecars are already
		// gone from the composed names; what remains prefixed is the
		// four ciphertexts and the abandoned composed manifest tree.
		for _, key := range allClientKeys {
			f.deleteStored(rollbackPrefix + key)
		}

		// Reads cannot have been quietly resolving to the prefixed
		// copies: every key still round-trips byte-for-byte.
		rb := f.serverAt("", "rb-restored-2")
		rbAPI := f.apiFor(rb)
		for _, key := range allClientKeys {
			f.assertStoredBytes(rbAPI, key)
		}

		// Listings show one era, exactly: the four client keys plus the
		// two client-visible geometry sidecars, nothing internal, nothing
		// prefixed — and a listing scoped to the retired prefix finds
		// nothing at all.
		want := map[string]bool{
			legacySingle:                    true,
			legacyMulti:                     true,
			legacyMulti + ".armor-manifest": true,
			newSingle:                       true,
			newMulti:                        true,
			newMulti + ".armor-manifest":    true,
		}
		keys := f.listKeys(rbAPI)
		if len(keys) != len(want) {
			t.Errorf("listing after retirement returned %d keys, want %d: %v", len(keys), len(want), keys)
		}
		for _, k := range keys {
			if !want[k] {
				t.Errorf("unexpected key in listing: %s", k)
			}
			if strings.Contains(k, ".armor/") || strings.HasPrefix(k, rollbackPrefix) {
				t.Errorf("listing leaked internal or prefixed state: %s", k)
			}
		}
		scoped := f.listKeysScoped(rbAPI, rollbackPrefix)
		if len(scoped) != 0 {
			t.Errorf("prefix=%q listing returned %d objects, want 0: %v", rollbackPrefix, len(scoped), scoped)
		}

		// The store-wide "after" row, mirrored from §6.7: nothing remains
		// under the prefix except the abandoned composed manifest tree,
		// and the root manifest tree is byte-identical to the era census —
		// never moved, never merged.
		for _, k := range f.storedKeys() {
			if !strings.HasPrefix(k, rollbackPrefix) {
				continue
			}
			if !strings.HasPrefix(k, rollbackPrefix+".armor/manifest/") {
				t.Errorf("prefixed copy survived retirement: %s", k)
			}
		}
		rootManifest := f.bucket + "/.armor/manifest/"
		after := mixedEraCensus(t, f.root)
		matched := 0
		for rel, wantDigest := range f.eraCensus {
			if !strings.HasPrefix(rel, rootManifest) {
				continue
			}
			matched++
			if got, ok := after[rel]; !ok || got != wantDigest {
				t.Errorf("root manifest tree changed across the rollback: %s", rel)
			}
		}
		if matched == 0 {
			t.Fatalf("root manifest census matched nothing — wrong filter, assertion would pass vacuously")
		}
	})

	t.Run("the resumed deployment records into the root manifest again", func(t *testing.T) {
		f.at(t)

		rb := f.serverAt("", "rb-resumed")
		rbAPI := f.apiFor(rb)

		// The first post-rollback write lands at the root name and, once
		// flushed, in the ROOT manifest tree — the abandoned composed
		// tree gets no second life.
		body := mixedEraPattern(2048, 0x75)
		if code := f.put(rbAPI, newSingle, body); code != http.StatusOK {
			t.Fatalf("resumed PUT %s: status %d", newSingle, code)
		}
		f.want[newSingle] = body
		flushTenantManifest(t, rb)
		assertDirHasFiles(t, filepath.Join(f.root, f.bucket, ".armor", "manifest", "rb-resumed"),
			"root manifest deltas for the resumed deployment")

		again := f.serverAt("", "rb-resumed-2")
		if got := again.manifest.Len(); got != 3 {
			t.Errorf("restarted index loaded %d entries, want 3 (era A's 2 plus the resumed write); store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		if _, ok := again.manifest.Get(f.bucket, newSingle); !ok {
			t.Errorf("restarted index missing the resumed key %s", newSingle)
		}
		if _, ok := again.manifest.Get(f.bucket, newMulti); ok {
			t.Errorf("restarted index resolved %s — the composed delta was ingested after all", newMulti)
		}
	})
}
