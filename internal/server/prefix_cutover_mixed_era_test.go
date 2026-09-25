package server

// The single regression suite for the ARMOR_PREFIX cutover over ONE
// mixed-era bucket: a real store that has lived through the exact sequence
// docs/runbooks/prefix-cutover-and-legacy-state.md prescribes, holding
// pre-prefix objects at the bucket root alongside pre-prefix internal state
// (.armor/manifest deltas, an ADR-003 HMAC sidecar, an ADR-016 part-geometry
// sidecar, in-flight multipart upload state), then post-prefix objects and
// internal state under <prefix>.
//
// Where the other prefix suites each pin one mechanism in isolation — the
// handler-level cutover walk on a mock backend
// (internal/server/handlers/prefix_cutover_test.go), the manifest index rules
// (manifest_prefix_isolation_test.go, manifest_preprefix_coexistence_test.go),
// the dual-location listing filter (listing_internal_namespace_test.go), the
// multipart state placement (handlers/multipart_internal_state_prefix_placement_test.go)
// — this suite walks the whole runbook against one fixture and asserts every
// verification class the runbook's §6 demands of a cutover, in one place:
//
//	reads            both eras byte-for-byte through the armed deployment
//	writes           post-cutover writes land only under the tenant prefix
//	listings         both eras once, prefix-transparent, internal state of
//	                 BOTH locations filtered by the real backend predicate
//	finalization     post-cutover multipart completion writes both sidecars
//	                 at the composed names; a pre-cutover in-flight upload
//	                 cannot finalize at all (runbook gate 4 — no root
//	                 fallback for upload state)
//	hiding           legacy root .armor/ state never surfaces and is never
//	                 ingested (the armed manifest index starts and stays
//	                 empty of it); the root trees stay byte-identical
//	isolation        a second tenant of the same bucket sees neither era
//
// All traffic goes through the authenticated S3 mux with a real SigV4
// client, so the assertions cover the same request path a cutover operator
// validates with.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/jedarden/armor/internal/backend"
)

// mixedEraPrefix is the ARMOR_PREFIX the cutover arms; the bucket-side move
// and every composed-location assertion key off it.
const mixedEraPrefix = "tenant/"

// mixedEraFixture is one shared filesystem store plus the servers and the
// client handles the suite drives it with. newManifestTenantServer hardcodes
// the bucket name and credentials, so the fixture inherits them.
//
// Two test handles, deliberately: t is the parent test, which owns every
// cleanup — servers, manifest writers, httptest listeners — so a deployment
// booted in one phase stays alive (its writer flushing) until the whole
// sequence ends; cur is the subtest currently running, which owns the
// assertions, because a t.Fatalf on the parent from inside a subtest turns a
// clean failure into a runtime.Goexit. Every subtest begins with f.at(t).
type mixedEraFixture struct {
	t   *testing.T // parent test: cleanups
	cur *testing.T // current subtest: assertions

	root   string
	bucket string

	// store is the operator's bucket-side handle — the thing the runbook's
	// §4 data move works through, never a client path.
	store *backend.FSBackend

	legacy *Server // the pre-prefix deployment (era A)
	armed  *Server // the same store, prefix armed (era B)

	legacyAPI *s3.Client // SigV4 clients for each deployment
	armedAPI  *s3.Client

	// before is the store census taken at the end of era A: every file's
	// SHA-256, keyed by store-relative slash path. Root-state immutability
	// assertions compare later censuses against it.
	before map[string]string

	// the in-flight era-A upload: open at cutover, never completable after
	inflightUploadID string
	inflightETag     string

	// want holds the exact plaintext of every client key by era, so the
	// byte-for-byte assertions read as data, not as inline literals.
	want map[string][]byte
}

func newMixedEraFixture(t *testing.T) *mixedEraFixture {
	t.Helper()

	root := t.TempDir()
	store, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("bucket-side store handle: %v", err)
	}
	f := &mixedEraFixture{
		t:      t,
		cur:    t,
		root:   root,
		bucket: "shared-bucket",
		store:  store,
		want:   make(map[string][]byte),
	}

	f.legacy = f.serverAt("", "era-legacy")
	f.legacyAPI = f.apiFor(f.legacy)
	return f
}

// at points the fixture's assertion handle at the calling subtest.
func (f *mixedEraFixture) at(t *testing.T) {
	f.cur = t
	f.cur.Helper()
}

// serverAt boots a full deployment over the shared store with the given
// ADR-001 tenant prefix. Cleanups land on the PARENT test — a writer stopped
// when its booting subtest ends would silently drop every op enqueued later,
// and the sequence depends on the armed era's deltas flushing last.
func (f *mixedEraFixture) serverAt(tenantPrefix, writerID string) *Server {
	f.t.Helper()
	srv := newManifestTenantServer(f.t, f.root, tenantPrefix, writerID, "")
	f.t.Cleanup(srv.StopManifestCompactor)
	return srv
}

// apiFor exposes one deployment through a real HTTP server and returns a
// SigV4 client signed with the credentials the deployment was configured
// with (newManifestTenantServer's hardcoded pair).
func (f *mixedEraFixture) apiFor(srv *Server) *s3.Client {
	f.cur.Helper()
	ts := httptest.NewServer(srv.Handler())
	f.t.Cleanup(ts.Close)
	return newListingS3Client(f.cur, ts.URL, "test-access-key", "test-secret-key")
}

// --- S3 operations -------------------------------------------------------

// get returns (status, body); non-2xx responses surface their status with a
// nil body instead of failing, so the gate assertions can expect 404/403.
func (f *mixedEraFixture) get(client *s3.Client, key string) (int, []byte) {
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
func (f *mixedEraFixture) put(client *s3.Client, key string, body []byte) int {
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

// statusOf maps an SDK error to an HTTP status, reporting false for anything
// it does not recognize so callers fail loudly rather than assert on guesses.
func (f *mixedEraFixture) statusOf(err error) (int, bool) {
	f.cur.Helper()
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode(), true
	}
	return 0, false
}

func (f *mixedEraFixture) createUpload(client *s3.Client, key string) string {
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

func (f *mixedEraFixture) uploadPart(client *s3.Client, key, uploadID string, partNumber int32, body []byte) string {
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

// complete returns the status of CompleteMultipartUpload; the drain-gate leg
// expects a 404 and must be able to see it.
func (f *mixedEraFixture) complete(client *s3.Client, key, uploadID string, etags []string) int {
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

// listKeys returns the client-visible keys and common prefixes a
// ListObjectsV2 through the deployment reports — raw handler output, with
// the real backend internal-namespace filter applied. No test-side predicate
// mirrors it: the filter itself is under test.
func (f *mixedEraFixture) listKeys(client *s3.Client, prefix, delimiter string) ([]string, []string) {
	f.cur.Helper()
	in := &s3.ListObjectsV2Input{Bucket: aws.String(f.bucket)}
	if prefix != "" {
		in.Prefix = aws.String(prefix)
	}
	if delimiter != "" {
		in.Delimiter = aws.String(delimiter)
	}
	out, err := client.ListObjectsV2(context.Background(), in)
	if err != nil {
		f.cur.Fatalf("ListObjectsV2(prefix=%q, delimiter=%q): %v", prefix, delimiter, err)
	}
	keys := make([]string, 0, len(out.Contents))
	for _, o := range out.Contents {
		keys = append(keys, aws.ToString(o.Key))
	}
	prefixes := make([]string, 0, len(out.CommonPrefixes))
	for _, cp := range out.CommonPrefixes {
		prefixes = append(prefixes, aws.ToString(cp.Prefix))
	}
	return keys, prefixes
}

// listVersionKeys is ListObjectVersions' view of the same store.
func (f *mixedEraFixture) listVersionKeys(client *s3.Client) []string {
	f.cur.Helper()
	out, err := client.ListObjectVersions(context.Background(), &s3.ListObjectVersionsInput{
		Bucket: aws.String(f.bucket),
	})
	if err != nil {
		f.cur.Fatalf("ListObjectVersions: %v", err)
	}
	keys := make([]string, 0, len(out.Versions))
	for _, v := range out.Versions {
		keys = append(keys, aws.ToString(v.Key))
	}
	return keys
}

// --- bucket-side (operator) operations ----------------------------------

// copyStored is the runbook §4 bulk copy: byte-preserving, metadata verbatim.
// The root original is left in place — retirement happens only after reads
// validate (§6.7).
func (f *mixedEraFixture) copyStored(from, to string) {
	f.cur.Helper()
	if err := f.store.Copy(context.Background(), f.bucket, from, f.bucket, to, nil, false); err != nil {
		f.cur.Fatalf("bucket-side copy %s -> %s: %v", from, to, err)
	}
}

// deleteStored is the runbook §4 step 8: retire a root original.
func (f *mixedEraFixture) deleteStored(key string) {
	f.cur.Helper()
	if err := f.store.Delete(context.Background(), f.bucket, key); err != nil {
		f.cur.Fatalf("bucket-side delete %s: %v", key, err)
	}
}

// rewriteGeometrySidecarRef rewrites a moved ADR-016 geometry sidecar's
// ciphertext reference to the prefixed name — bucket-side, the way an
// operator would. The sidecar's ciphertext_object field (and its
// x-amz-meta-armor-ciphertext-ref metadata) records the ciphertext location
// AS OF COMPLETION TIME (handlers applyPrefix at Complete), so a pre-cutover
// sidecar names the root copy. The prefix-prepend rename in §4.5 moves the
// sidecar's NAME but not that field; until it is rewritten the moved object
// is silently served from its root original, and retirement 500s it — the
// failure mode the retirement subtest pins.
func (f *mixedEraFixture) rewriteGeometrySidecarRef(clientKey string) {
	f.cur.Helper()
	key := mixedEraPrefix + clientKey + ".armor-manifest"
	rc, info, err := f.store.Get(context.Background(), f.bucket, key)
	if err != nil {
		f.cur.Fatalf("bucket-side read of geometry sidecar %s: %v", key, err)
	}
	body, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		f.cur.Fatalf("read geometry sidecar %s: %v", key, err)
	}
	var manifest backend.ManifestBody
	if err := json.Unmarshal(body, &manifest); err != nil {
		f.cur.Fatalf("parse geometry sidecar %s: %v", key, err)
	}
	if manifest.CiphertextObject != clientKey {
		f.cur.Fatalf("geometry sidecar %s names ciphertext %q, want the pre-cutover root name %q",
			key, manifest.CiphertextObject, clientKey)
	}
	manifest.CiphertextObject = mixedEraPrefix + clientKey
	updated, err := json.Marshal(manifest)
	if err != nil {
		f.cur.Fatalf("re-marshal geometry sidecar %s: %v", key, err)
	}
	meta := map[string]string{}
	for k, v := range info.Metadata {
		meta[k] = v
	}
	meta["x-amz-meta-armor-ciphertext-ref"] = manifest.CiphertextObject
	if err := f.store.Put(context.Background(), f.bucket, key, bytes.NewReader(updated), int64(len(updated)), meta); err != nil {
		f.cur.Fatalf("bucket-side rewrite of geometry sidecar %s: %v", key, err)
	}
}

// storedExists reports whether the store holds an object at a bucket-side key.
// The FS backend surfaces a missing object as a raw os.ErrNotExist chain, not
// a typed backend error, so classify on that.
func (f *mixedEraFixture) storedExists(key string) bool {
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
// the operator's bucket-side inventory (runbook §3.2). The one empty key the
// FS backend can surface for an in-flight part's Key-less .metadata sidecar
// is skipped: it is a staging artifact, not an object anyone stored.
func (f *mixedEraFixture) storedKeys() []string {
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

// --- census and data helpers ---------------------------------------------

// census hashes every file in the store, keyed by slash-relative path. The
// runbook's before/after evidence rows, in executable form.
func mixedEraCensus(t *testing.T, root string) map[string]string {
	t.Helper()
	digests := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(data)
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digests[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	if err != nil {
		t.Fatalf("census walk of %s: %v", root, err)
	}
	return digests
}

// assertUnchanged fails when any census entry matching keep is missing from
// after or holds different bytes. The matched count is asserted non-zero so
// a wrong filter cannot make the assertion pass vacuously.
func mixedEraAssertUnchanged(t *testing.T, before, after map[string]string, keep func(string) bool, what string) {
	t.Helper()
	matched := 0
	for rel, want := range before {
		if !keep(rel) {
			continue
		}
		matched++
		got, ok := after[rel]
		if !ok {
			t.Errorf("%s: %s vanished from the store", what, rel)
			continue
		}
		if got != want {
			t.Errorf("%s: %s changed during the sequence", what, rel)
		}
	}
	if matched == 0 {
		t.Fatalf("%s: census subset matched nothing — wrong filter, assertion would pass vacuously", what)
	}
}

// assertStoredBytes round-trips a client key through the deployment and
// compares byte-for-byte — runbook §6.1, "every legacy key, byte-for-byte".
func (f *mixedEraFixture) assertStoredBytes(client *s3.Client, key string) {
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

// mixedEraPattern is deterministic filler distinct per offset, sized past
// block boundaries the way real payloads are.
func mixedEraPattern(n int, salt byte) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i%251) ^ salt
	}
	return b
}

// mixedEraMultipartParts is the two-part shape of a real multipart object: a
// non-final part above the 5 MiB ADR-015 minimum and a short final part.
func mixedEraMultipartParts(salt byte) (part1, part2, plaintext []byte) {
	part1 = mixedEraPattern(5*1024*1024+512, salt)
	part2 = mixedEraPattern(1, salt)
	plaintext = append(append([]byte{}, part1...), part2...)
	return part1, part2, plaintext
}

// mixedEraHMACSidecars names the two sidecar flavors a completed multipart
// object has, at both the root and composed locations, for placement
// assertions: [rootHMAC, composedHMAC, rootGeometry, composedGeometry].
func (f *mixedEraFixture) hmacSidecars(clientKey string) (string, string, string, string) {
	rootHMAC := backend.GetSidecarKey("", clientKey)
	composedHMAC := backend.GetSidecarKey(mixedEraPrefix, clientKey)
	return rootHMAC, composedHMAC,
		clientKey + ".armor-manifest",
		mixedEraPrefix + clientKey + ".armor-manifest"
}

// TestPrefixCutoverMixedEraBucketRegression is the suite. The subtests run
// in order and share one fixture, because the runbook is a sequence: each
// phase's assertions depend on the phases before it.
func TestPrefixCutoverMixedEraBucketRegression(t *testing.T) {
	f := newMixedEraFixture(t)

	// Era-A data: a single-part object, a completed multipart object (whose
	// stored state spans ciphertext + geometry sidecar + HMAC sidecar), and
	// one upload left in flight. Both client-visible eras and both internal
	// namespaces end up in one bucket by the end.
	const (
		legacySingle = "reports/q3.csv"
		legacyMulti  = "backups/base.tar"
		inflightKey  = "backups/inflight.tar"
		newSingle    = "reports/q4.csv"
		newMulti     = "backups/incremental.tar"
	)

	t.Run("legacy era writes objects and root internal state", func(t *testing.T) {
		f.at(t)

		// Single-part write.
		body := mixedEraPattern(3*65536+17, 0x10)
		if code := f.put(f.legacyAPI, legacySingle, body); code != http.StatusOK {
			t.Fatalf("legacy PUT %s: status %d", legacySingle, code)
		}
		f.want[legacySingle] = body

		// Completed multipart write (v3 is the config default this helper
		// stack runs with).
		p1, p2, plaintext := mixedEraMultipartParts(0x20)
		uploadID := f.createUpload(f.legacyAPI, legacyMulti)
		e1 := f.uploadPart(f.legacyAPI, legacyMulti, uploadID, 1, p1)
		e2 := f.uploadPart(f.legacyAPI, legacyMulti, uploadID, 2, p2)
		if code := f.complete(f.legacyAPI, legacyMulti, uploadID, []string{e1, e2}); code != http.StatusOK {
			t.Fatalf("legacy complete %s: status %d", legacyMulti, code)
		}
		f.want[legacyMulti] = plaintext

		// The completed multipart object's three pieces sit at the ROOT
		// locations: ciphertext at the bare key, the ADR-016 geometry
		// sidecar beside it, and the ADR-003 HMAC sidecar under the root
		// .armor/hmac/ name.
		rootHMAC, _, rootGeometry, _ := f.hmacSidecars(legacyMulti)
		for _, key := range []string{legacyMulti, rootGeometry, rootHMAC} {
			if !f.storedExists(key) {
				t.Fatalf("era-A multipart completion left nothing at bucket-root key %s", key)
			}
		}

		// An upload left in flight: its ARMOR-level state and the backend's
		// part staging both live under the root .armor/ namespace.
		inflightPart := mixedEraPattern(64*1024, 0x30)
		f.inflightUploadID = f.createUpload(f.legacyAPI, inflightKey)
		f.inflightETag = f.uploadPart(f.legacyAPI, inflightKey, f.inflightUploadID, 1, inflightPart)

		// Flush era A's manifest ops: the delta lands at the root
		// .armor/manifest/ tree, exactly what a pre-prefix era leaves behind.
		flushTenantManifest(t, f.legacy)
		assertDirHasFiles(t, filepath.Join(f.root, f.bucket, ".armor", "manifest"),
			"bucket-root .armor/manifest (pre-prefix manifest deltas)")

		// The "before" census: every file in the store, hashed.
		f.before = mixedEraCensus(t, f.root)
	})

	t.Run("arming the prefix hides legacy objects and un-ingested root deltas", func(t *testing.T) {
		f.at(t)

		// The armed deployment on the same store: composed manifest location
		// (asserted by the helper), and an index that must come up EMPTY —
		// the root deltas sit right there in the bucket and must not be
		// ingested (ADR-001: ingesting them is cross-tenant contamination;
		// keys are client-visible).
		f.armed = f.serverAt(mixedEraPrefix, "era-armed")
		f.armedAPI = f.apiFor(f.armed)

		if got := f.armed.manifest.Len(); got != 0 {
			t.Errorf("armed instance loaded %d manifest entries from a store holding only root-era deltas, want 0; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{legacySingle, legacyMulti} {
			if _, ok := f.armed.manifest.Get(f.bucket, key); ok {
				t.Errorf("legacy key %s resolved through the armed manifest index — root-era state was ingested", key)
			}
		}

		// Runbook gate 1: between arming and the data move the legacy
		// objects do not exist through ARMOR — the prefix rewrote the
		// lookup and nothing probes the unprefixed location.
		for _, key := range []string{legacySingle, legacyMulti} {
			if code, _ := f.get(f.armedAPI, key); code != http.StatusNotFound {
				t.Errorf("pre-move GET of %s returned %d, want 404 — a prefixed deployment must not silently resolve the legacy location", key, code)
			}
		}

		// Nothing vanished bucket-side: the bytes are still at the root.
		for _, key := range []string{legacySingle, legacyMulti} {
			if !f.storedExists(key) {
				t.Errorf("legacy object %s vanished from the bucket root before any move was made", key)
			}
		}
	})

	t.Run("data move takes ciphertext, geometry sidecar, and renamed HMAC sidecar", func(t *testing.T) {
		f.at(t)

		// The multipart move, one piece at a time, so the runbook's gate 2
		// is pinned rather than assumed: with the ciphertext moved but its
		// ADR-016 geometry sidecar still only at the root name, the read
		// must NOT produce the correct plaintext — there is no error-free
		// path that re-derives part geometry the sidecar carries.
		f.copyStored(legacyMulti, mixedEraPrefix+legacyMulti)
		if code, got := f.get(f.armedAPI, legacyMulti); code == http.StatusOK && bytes.Equal(got, f.want[legacyMulti]) {
			t.Fatalf("multipart GET returned correct bytes with the geometry sidecar still unmoved — the three-piece rule is not being exercised")
		}

		// Complete the three-piece move: geometry sidecar is a prefix-prepend
		// rename; the HMAC sidecar's name hashes the prefix in, so it moves
		// to a RECOMPUTED name. Renames: the destination is copied, the root
		// original retired.
		_, _, rootGeometry, _ := f.hmacSidecars(legacyMulti)
		rootHMAC, composedHMAC, _, _ := f.hmacSidecars(legacyMulti)
		f.copyStored(rootGeometry, mixedEraPrefix+rootGeometry)
		f.deleteStored(rootGeometry)
		f.copyStored(rootHMAC, composedHMAC)
		f.deleteStored(rootHMAC)

		if !f.storedExists(composedHMAC) {
			t.Fatalf("HMAC sidecar not found at the composed name %s after the rename", composedHMAC)
		}
		if f.storedExists(rootHMAC) {
			t.Fatalf("HMAC sidecar still sitting at the root name %s after a rename", rootHMAC)
		}

		// Both era-A objects now read byte-for-byte through the armed
		// deployment — the multipart one through its relocated sidecar pair.
		f.assertStoredBytes(f.armedAPI, legacyMulti)

		f.copyStored(legacySingle, mixedEraPrefix+legacySingle)
		f.assertStoredBytes(f.armedAPI, legacySingle)
	})

	t.Run("post-cutover writes land only under the tenant prefix", func(t *testing.T) {
		f.at(t)

		body := mixedEraPattern(100*1024, 0x40)
		if code := f.put(f.armedAPI, newSingle, body); code != http.StatusOK {
			t.Fatalf("post-cutover PUT %s: status %d", newSingle, code)
		}
		f.want[newSingle] = body

		if f.storedExists(newSingle) {
			t.Errorf("post-cutover object stored at the bare root key %s — prefix not applied on write", newSingle)
		}
		if !f.storedExists(mixedEraPrefix + newSingle) {
			t.Errorf("post-cutover object missing at %s", mixedEraPrefix+newSingle)
		}
		f.assertStoredBytes(f.armedAPI, newSingle)

		// The bucket still legitimately holds the era-A ROOT ORIGINALS at
		// this point — the runbook retires them only after validation — so
		// the store-wide nothing-unprefixed walk belongs to the retirement
		// phase, not here.
	})

	t.Run("multipart finalization writes sidecars at the composed names", func(t *testing.T) {
		f.at(t)

		p1, p2, plaintext := mixedEraMultipartParts(0x50)
		uploadID := f.createUpload(f.armedAPI, newMulti)
		e1 := f.uploadPart(f.armedAPI, newMulti, uploadID, 1, p1)
		e2 := f.uploadPart(f.armedAPI, newMulti, uploadID, 2, p2)
		if code := f.complete(f.armedAPI, newMulti, uploadID, []string{e1, e2}); code != http.StatusOK {
			t.Fatalf("post-cutover complete %s: status %d", newMulti, code)
		}
		f.want[newMulti] = plaintext

		// Both sidecar flavors at the COMPOSED names, and neither at a root
		// name: the geometry sidecar prefixed beside the object, the HMAC
		// sidecar named by sha256(prefix + client key) under the composed
		// .armor/hmac/ tree (runbook §6.3).
		rootHMAC, composedHMAC, rootGeometry, composedGeometry := f.hmacSidecars(newMulti)
		for _, key := range []string{composedHMAC, composedGeometry} {
			if !f.storedExists(key) {
				t.Errorf("post-cutover finalization left no sidecar at the composed name %s", key)
			}
		}
		for _, key := range []string{rootHMAC, rootGeometry} {
			if f.storedExists(key) {
				t.Errorf("post-cutover finalization wrote a sidecar at the legacy root name %s", key)
			}
		}
		f.assertStoredBytes(f.armedAPI, newMulti)
	})

	t.Run("pre-cutover upload cannot finalize post-cutover", func(t *testing.T) {
		f.at(t)

		// Runbook gate 4 / §3.3: upload state resolves beneath the CURRENT
		// prefix with no root fallback, so the era-A upload opened before
		// the cutover is NoSuchUpload to the armed deployment — drain or
		// abort before arming, exactly as the runbook says.
		code := f.complete(f.armedAPI, inflightKey, f.inflightUploadID, []string{f.inflightETag})
		if code != http.StatusNotFound {
			t.Fatalf("completing a pre-cutover upload through the armed deployment returned %d, want 404 NoSuchUpload", code)
		}

		// The failed attempt left the root-era state byte-identical: the
		// ARMOR-level state files and the backend's part staging.
		after := mixedEraCensus(t, f.root)
		inflightPrefix := func(rel string) bool {
			for _, p := range []string{
				filepath.ToSlash(filepath.Join(f.bucket, ".armor", "multipart", f.inflightUploadID)) + "/",
				filepath.ToSlash(filepath.Join(f.bucket, ".armor", "multipart", "backups", "inflight.tar")) + "/",
			} {
				if strings.HasPrefix(rel, p) {
					return true
				}
			}
			return false
		}
		mixedEraAssertUnchanged(t, f.before, after, inflightPrefix,
			"pre-cutover in-flight upload state")
	})

	t.Run("listings show both eras and hide internal state at both locations", func(t *testing.T) {
		f.at(t)

		// Full listing: every client key of BOTH eras exactly once, none
		// carrying the prefix, none from either internal location. The
		// filtering is the real backend predicate — the root .armor/ trees
		// (manifest deltas, HMAC sidecars, in-flight upload state) are in
		// the store and must not surface. The two ADR-016 geometry sidecars
		// DO appear: backends hide the .armor/ namespace but not the
		// <key>.armor-manifest sidecars beside the objects (the same
		// documented visibility the handler-level cutover suite filters
		// test-side), so they are pinned here as client-visible.
		want := map[string]bool{
			legacySingle:                    true,
			legacyMulti:                     true,
			legacyMulti + ".armor-manifest": true,
			newSingle:                       true,
			newMulti:                        true,
			newMulti + ".armor-manifest":    true,
		}
		keys, prefixes := f.listKeys(f.armedAPI, "", "")
		if len(keys) != len(want) {
			t.Errorf("listing returned %d keys, want %d: %v", len(keys), len(want), keys)
		}
		for _, k := range keys {
			if !want[k] {
				t.Errorf("unexpected key in listing: %s", k)
			}
			if strings.Contains(k, ".armor/") {
				t.Errorf("internal namespace leaked into a client listing: %s", k)
			}
			if strings.HasPrefix(k, mixedEraPrefix) {
				t.Errorf("client-visible key still carries the ARMOR_PREFIX: %s", k)
			}
		}
		if len(prefixes) != 0 {
			t.Errorf("undelimited listing produced common prefixes: %v", prefixes)
		}

		// Delimited listing: pruned internal directories must not resurface
		// as common prefixes — the root-era .armor/ tree and the composed
		// one both prune.
		_, delimPrefixes := f.listKeys(f.armedAPI, "", "/")
		got := append([]string{}, delimPrefixes...)
		sort.Strings(got)
		if want := []string{"backups/", "reports/"}; !equalStrings(got, want) {
			t.Errorf("delimiter common prefixes = %v, want %v", got, want)
		}

		// Asking for the reserved namespace by either name — the client
		// form (which the handler maps onto the stored composed location)
		// or the literal stored form — returns nothing.
		for _, p := range []string{".armor/", mixedEraPrefix + ".armor/"} {
			internal, _ := f.listKeys(f.armedAPI, p, "")
			if len(internal) != 0 {
				t.Errorf("prefix=%q listing returned %d objects, want 0: %v", p, len(internal), internal)
			}
		}

		// The stored prefix itself is not part of the client key space: a
		// client listing scoped to "tenant/" finds nothing, because keys
		// named that way would be stored at tenant/tenant/...
		scoped, _ := f.listKeys(f.armedAPI, mixedEraPrefix, "")
		if len(scoped) != 0 {
			t.Errorf("prefix=%q listing returned %d objects, want 0: %v", mixedEraPrefix, len(scoped), scoped)
		}

		// Versioned listings inherit the same filter.
		versionKeys := f.listVersionKeys(f.armedAPI)
		if len(versionKeys) != len(want) {
			t.Errorf("version listing returned %d keys, want %d: %v", len(versionKeys), len(want), versionKeys)
		}
		for _, k := range versionKeys {
			if !want[k] || strings.Contains(k, ".armor/") {
				t.Errorf("unexpected or internal key in version listing: %s", k)
			}
		}

		// Clients are refused the reserved namespace outright — the bare
		// .armor/ form, read and write. The composed form tenant/.armor/…
		// is NOT the namespace as a client key: it is legal client data
		// that stores OUTSIDE the reserved namespace, at
		// tenant/tenant/.armor/…, so the stored internal name is simply
		// not in the client key space — a GET of it cannot reach the
		// internal object (404, not the delta's bytes).
		if code, _ := f.get(f.armedAPI, ".armor/manifest/era-legacy/delta-0000000001.jsonl"); code != http.StatusForbidden {
			t.Errorf("GET of a root internal object returned %d, want 403", code)
		}
		if code := f.put(f.armedAPI, ".armor/evil", []byte("x")); code != http.StatusForbidden {
			t.Errorf("PUT into .armor/ returned %d, want 403", code)
		}
		if code, _ := f.get(f.armedAPI, mixedEraPrefix+".armor/manifest/era-armed/delta-0000000001.jsonl"); code != http.StatusNotFound {
			t.Errorf("GET of the stored composed internal name returned %d, want 404 — internal state must not be reachable by its stored name", code)
		}
	})

	t.Run("root manifest stays put and un-ingested while deltas land composed", func(t *testing.T) {
		f.at(t)

		// Flush the armed deployment's own ops (the two post-cutover
		// writes): its deltas must appear under the composed location.
		flushTenantManifest(t, f.armed)
		composedWriter := filepath.Join(f.root, f.bucket, strings.SplitN(mixedEraPrefix, "/", 2)[0],
			".armor", "manifest", "era-armed")
		assertDirHasFiles(t, composedWriter, "composed manifest deltas for era-armed")

		// The in-memory index holds exactly the armed deployment's own keys;
		// the legacy keys stay unresolved through it even though both read
		// fine — the manifest is an optimization, not the source of truth,
		// and the root deltas are nobody's index now.
		for _, key := range []string{newSingle, newMulti} {
			if _, ok := f.armed.manifest.Get(f.bucket, key); !ok {
				t.Errorf("armed index missing its own post-cutover key %s", key)
			}
		}
		for _, key := range []string{legacySingle, legacyMulti} {
			if _, ok := f.armed.manifest.Get(f.bucket, key); ok {
				t.Errorf("legacy key %s entered the armed manifest index", key)
			}
		}

		// A restarted armed instance loads exactly its own two deltas —
		// persistence of the composed tree, and still zero ingestion of the
		// root tree sitting beside it.
		again := f.serverAt(mixedEraPrefix, "era-armed-2")
		if got := again.manifest.Len(); got != 2 {
			t.Errorf("restarted armed instance loaded %d manifest entries, want exactly its own 2; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{newSingle, newMulti} {
			if _, ok := again.manifest.Get(f.bucket, key); !ok {
				t.Errorf("restarted armed instance did not load %s from the composed deltas", key)
			}
		}
		for _, key := range []string{legacySingle, legacyMulti} {
			if _, ok := again.manifest.Get(f.bucket, key); ok {
				t.Errorf("restarted armed instance resolved legacy key %s — root deltas were ingested", key)
			}
		}

		// The root manifest tree is byte-identical to the era-A census: the
		// runbook's gate 3 (do not move, do not disturb) and §6.5's count
		// check, in one assertion.
		after := mixedEraCensus(t, f.root)
		rootManifest := filepath.ToSlash(filepath.Join(f.bucket, ".armor", "manifest")) + "/"
		mixedEraAssertUnchanged(t, f.before, after, func(rel string) bool {
			return strings.HasPrefix(rel, rootManifest)
		}, "root manifest tree")
	})

	t.Run("retiring root copies keeps both eras serving", func(t *testing.T) {
		f.at(t)

		// Runbook §4 step 8 + §6.7: after validation, the root originals of
		// everything MOVED are deleted; the root .armor/ trees stay. Every
		// key must keep reading byte-for-byte from the prefixed copies.
		f.deleteStored(legacySingle)
		f.deleteStored(legacyMulti)

		// The single-part object serves from its prefixed copy.
		f.assertStoredBytes(f.armedAPI, legacySingle)

		// The multipart object does NOT — not yet. Its moved geometry
		// sidecar still names the retired root ciphertext (completion
		// records applyPrefix at Complete time; the move renamed the
		// sidecar but not that field), and the read path heads the named
		// object verbatim. The staleness was INVISIBLE during §6
		// validation because the read succeeded — from the root original.
		// This retryable 500 is exactly what a cutover that skips the
		// sidecar content rewrite ships to its clients; pin it.
		if code, _ := f.get(f.armedAPI, legacyMulti); code != http.StatusInternalServerError {
			t.Fatalf("GET of the moved multipart object with a stale sidecar ciphertext ref returned %d, want 500 (the pinned stale-manifest failure)", code)
		}

		// The corrected §4.5 recipe: rewrite the sidecar's ciphertext
		// reference to the prefixed name. The moved object now serves from
		// its prefixed copy, byte-for-byte.
		f.rewriteGeometrySidecarRef(legacyMulti)
		f.assertStoredBytes(f.armedAPI, legacyMulti)

		for _, key := range []string{legacySingle, legacyMulti, newSingle, newMulti} {
			f.assertStoredBytes(f.armedAPI, key)
		}
		// The two geometry sidecars remain client-visible beside their
		// objects (pinned in the listings subtest above).
		keys, _ := f.listKeys(f.armedAPI, "", "")
		if len(keys) != 6 {
			t.Errorf("listing after root retirement returned %d keys, want 6: %v", len(keys), keys)
		}

		// The store-wide "after" row of the cutover evidence (§6.7): every
		// remaining key outside the deliberately-staying root .armor/ trees
		// lives under the tenant prefix.
		for _, k := range f.storedKeys() {
			if k == "" {
				// FS-backend part-staging artifact: an in-flight part's
				// .metadata carries no Key field, so a raw list surfaces one
				// empty key. Never client-visible (the backend prunes
				// .armor/ before the handler sees anything).
				continue
			}
			if scoped := strings.TrimPrefix(k, mixedEraPrefix); strings.HasPrefix(scoped, ".armor/") {
				continue // root internal state deliberately stays (gate 3)
			}
			if !strings.HasPrefix(k, mixedEraPrefix) {
				t.Errorf("store holds a non-prefixed key after retirement: %s", k)
			}
		}
	})

	t.Run("a second tenant of the same bucket sees neither era", func(t *testing.T) {
		f.at(t)

		// The ADR-001 shared-bucket scenario, over the post-cutover store:
		// q/ boots with an empty index (p/'s composed deltas are not its
		// history), sees none of p/'s objects of either era, and its writes
		// land inside its own namespace even on a client-key collision.
		q := f.serverAt("q/", "era-q")
		qAPI := f.apiFor(q)

		if got := q.manifest.Len(); got != 0 {
			t.Errorf("q/ instance loaded %d manifest entries from p/'s deltas, want 0", got)
		}
		for _, key := range []string{legacySingle, legacyMulti, newSingle, newMulti} {
			if code, _ := f.get(qAPI, key); code != http.StatusNotFound {
				t.Errorf("q/ GET of p/'s %s returned %d, want 404", key, code)
			}
		}
		keys, _ := f.listKeys(qAPI, "", "")
		if len(keys) != 0 {
			t.Errorf("q/ listing returned %v, want empty", keys)
		}

		// A colliding write is confined: q/'s reports/q3.csv lands at
		// q/reports/q3.csv only, and p/'s object is untouched.
		qBody := mixedEraPattern(4096, 0x60)
		if code := f.put(qAPI, legacySingle, qBody); code != http.StatusOK {
			t.Fatalf("q/ PUT %s: status %d", legacySingle, code)
		}
		if !f.storedExists("q/" + legacySingle) {
			t.Errorf("q/ object missing at q/%s", legacySingle)
		}
		if code, got := f.get(qAPI, legacySingle); code != http.StatusOK || !bytes.Equal(got, qBody) {
			t.Errorf("q/ read of its own object: status %d, %d bytes", code, len(got))
		}
		f.assertStoredBytes(f.armedAPI, legacySingle) // p/ still reads its own bytes

		// And p/'s listing did not grow q/'s key. Six keys: p/'s four
		// objects plus its two client-visible geometry sidecars.
		keys, _ = f.listKeys(f.armedAPI, "", "")
		if len(keys) != 6 {
			t.Errorf("p/ listing after q/'s write returned %d keys, want 6: %v", len(keys), keys)
		}
	})
}

// equalStrings compares two string slices element-wise, order included.
func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
