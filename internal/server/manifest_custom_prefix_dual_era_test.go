package server

// The dual-era regression for ARMOR_MANIFEST_PREFIX containment (ADR-001
// "Internal Namespaces", manifest paragraph): one bucket that lived through
// the pre-prefix era — client objects at the root, manifest deltas, an
// ADR-003 HMAC sidecar and an ADR-016 geometry sidecar under the root
// .armor/ tree — and then armed ARMOR_PREFIX together with a CUSTOM
// ARMOR_MANIFEST_PREFIX.
//
// The other prefix suites each hold one of the two variables at its default:
// the forward cutover suite (prefix_cutover_mixed_era_test.go) walks the
// whole runbook with ARMOR_MANIFEST_PREFIX unset, and the confinement suite
// (manifest_prefix_isolation_test.go, TestManifestPrefixConfinement)
// exercises a custom value on a store with no legacy era beside it. A real
// deployment can meet both conditions at once — the relocation value exists
// exactly for a tenant that needs its manifest history out of the default
// tree, which is a history that pre-dates the arming — and that combination
// is what this suite pins, end to end through the authenticated S3 mux:
//
//   arming       the armed instance boots with an EMPTY index; the root-era
//                deltas are not ingested and the legacy objects do not
//                resolve, while their bytes stay at the bucket root
//   placement    era-B data lands under the tenant prefix, HMAC sidecars
//                stay in the COMPOSED reserved namespace (tenant/.armor/hmac/,
//                not the relocated tree — the custom value relocates the
//                manifest tree only), and deltas land solely in the custom
//                composed location; the default composed location
//                (tenant/.armor/manifest/) never comes into existence
//   containment  every era-B write, internal state included, lands inside
//                the tenant namespace; the custom tree holds nothing but
//                manifest deltas
//   hiding       the root-era state stays byte-identical, stays un-ingested
//                across a restart, and neither internal location surfaces in
//                a client listing; the reserved namespace is refused to
//                clients at both locations and the custom tree's stored
//                names are not reachable as client keys
//   isolation    a same-tenant instance configured with the DEFAULT manifest
//                prefix does not adopt the relocated history — the manifest
//                location is part of the deployment's identity, so booting
//                without the custom value forks history rather than mixing
//                it
//
// The custom value deliberately stays INSIDE the reserved namespace
// (.armor/manifest-v2 under the tenant): that is the relocation a real
// tenant runs (both runbooks prescribe leaving the variable unset, and a
// value outside .armor/ would give up the reserved-namespace listing filter
// — a different, undocumented contract this suite must not pin). The census
// and payload helpers are the forward cutover suite's.

import (
	"bytes"
	"context"
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

const (
	// customManifestTenant is the ARMOR_PREFIX era B arms.
	customManifestTenant = "tenant/"
	// customManifestRelocation is the custom ARMOR_MANIFEST_PREFIX era B
	// runs with: relative to the prefix, inside the reserved namespace, so
	// the composed manifest location is tenant/.armor/manifest-v2.
	customManifestRelocation = ".armor/manifest-v2"
)

// customManifestFixture is one shared filesystem store plus the servers and
// client handles the suite drives it with. The t/cur split and every cleanup
// rule are the forward suite's (see mixedEraFixture): servers, writers and
// httptest listeners register on the PARENT test so a deployment booted in
// one phase stays alive until the sequence ends; assertions use the current
// subtest's handle.
type customManifestFixture struct {
	t   *testing.T // parent test: cleanups
	cur *testing.T // current subtest: assertions

	root   string
	bucket string

	// store is the operator's bucket-side handle — read-only here, used to
	// confirm what the deployments physically left behind.
	store *backend.FSBackend

	legacy *Server // era A: no ARMOR_PREFIX, default root manifest tree
	armed  *Server // era B: customManifestTenant + customManifestRelocation
	sib    *Server // same tenant, DEFAULT manifest prefix (isolation leg)
	again  *Server // era B restarted (persistence leg)

	legacyAPI *s3.Client
	armedAPI  *s3.Client

	// before is the store census taken at the end of era A. Everything
	// outside the tenant namespace must stay byte-identical through the
	// whole sequence.
	before map[string]string
}

func newCustomManifestFixture(t *testing.T) *customManifestFixture {
	t.Helper()

	root := t.TempDir()
	store, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("bucket-side store handle: %v", err)
	}
	f := &customManifestFixture{
		t:      t,
		cur:    t,
		root:   root,
		bucket: "shared-bucket", // newManifestTenantServer hardcodes it
		store:  store,
	}

	f.legacy = f.serverAt("", "custom-era-a", "")
	f.legacyAPI = f.apiFor(f.legacy)
	return f
}

func (f *customManifestFixture) at(t *testing.T) {
	f.cur = t
	f.cur.Helper()
}

// serverAt boots a deployment over the shared store; manifestPrefix "" means
// ARMOR_MANIFEST_PREFIX unset (the default tree), any other value is the
// custom relocation. Cleanups land on the PARENT test, for the same reason
// as the forward suite: a writer stopped when its booting subtest ends would
// silently drop every op enqueued later.
func (f *customManifestFixture) serverAt(tenantPrefix, writerID, manifestPrefix string) *Server {
	f.t.Helper()
	srv := newManifestTenantServer(f.t, f.root, tenantPrefix, writerID, manifestPrefix)
	f.t.Cleanup(srv.StopManifestCompactor)
	return srv
}

func (f *customManifestFixture) apiFor(srv *Server) *s3.Client {
	f.cur.Helper()
	ts := httptest.NewServer(srv.Handler())
	f.t.Cleanup(ts.Close)
	return newListingS3Client(f.cur, ts.URL, "test-access-key", "test-secret-key")
}

// --- S3 operations -------------------------------------------------------

// get returns (status, body); non-2xx responses surface their status with a
// nil body so the gate assertions can expect 403/404.
func (f *customManifestFixture) get(client *s3.Client, key string) (int, []byte) {
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
func (f *customManifestFixture) put(client *s3.Client, key string, body []byte) int {
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
func (f *customManifestFixture) statusOf(err error) (int, bool) {
	f.cur.Helper()
	var re *smithyhttp.ResponseError
	if errors.As(err, &re) {
		return re.HTTPStatusCode(), true
	}
	return 0, false
}

// completeTwoParts runs the two-part multipart shape the forward suite uses
// — a non-final part above the 5 MiB ADR-015 minimum and a short final part —
// to completion. Completion is what makes the deployment write both sidecar
// flavors (the ADR-003 HMAC sidecar and the ADR-016 geometry sidecar), so
// both eras complete one multipart object each.
func (f *customManifestFixture) completeTwoParts(client *s3.Client, key string, salt byte) []byte {
	f.cur.Helper()
	p1, p2, plaintext := mixedEraMultipartParts(salt)

	up, err := client.CreateMultipartUpload(context.Background(), &s3.CreateMultipartUploadInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		f.cur.Fatalf("CreateMultipartUpload %s: %v", key, err)
	}
	parts := make([]types.CompletedPart, 0, 2)
	for i, body := range [][]byte{p1, p2} {
		part, err := client.UploadPart(context.Background(), &s3.UploadPartInput{
			Bucket:     aws.String(f.bucket),
			Key:        aws.String(key),
			UploadId:   up.UploadId,
			PartNumber: aws.Int32(int32(i + 1)),
			Body:       bytes.NewReader(body),
		})
		if err != nil {
			f.cur.Fatalf("UploadPart %d of %s: %v", i+1, key, err)
		}
		parts = append(parts, types.CompletedPart{
			PartNumber: aws.Int32(int32(i + 1)),
			ETag:       part.ETag,
		})
	}
	_, err = client.CompleteMultipartUpload(context.Background(), &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(f.bucket),
		Key:             aws.String(key),
		UploadId:        up.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		f.cur.Fatalf("CompleteMultipartUpload %s: %v", key, err)
	}
	return plaintext
}

// listKeys returns the client-visible keys and common prefixes a
// ListObjectsV2 through the deployment reports — raw handler output with the
// real backend internal-namespace filter applied. No test-side predicate
// mirrors it: the filter itself is under test.
func (f *customManifestFixture) listKeys(client *s3.Client, prefix, delimiter string) ([]string, []string) {
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

// --- store-side (operator) observations ----------------------------------

// storedExists reports whether the store holds an object at a bucket-side
// key. The FS backend surfaces a missing object as a raw os.ErrNotExist
// chain, not a typed backend error, so classify on that.
func (f *customManifestFixture) storedExists(key string) bool {
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

// assertByteFor compares a client key's read through client against the
// expected plaintext recorded in want.
func (f *customManifestFixture) assertBytesFor(client *s3.Client, want map[string][]byte, key string) {
	f.cur.Helper()
	wantBody, ok := want[key]
	if !ok {
		f.cur.Fatalf("assertBytesFor: no expected plaintext recorded for %s", key)
	}
	code, got := f.get(client, key)
	if code != http.StatusOK {
		f.cur.Fatalf("GET %s: status %d, want 200", key, code)
	}
	if !bytes.Equal(got, wantBody) {
		f.cur.Fatalf("GET %s: byte mismatch: got %d bytes, want %d", key, len(got), len(wantBody))
	}
}

// assertStoreDirEmpty fails when any file exists under storeDir, the
// bucket-directory-relative path of a location that must not come into
// existence (a missing directory is the ordinary cold-start case).
func (f *customManifestFixture) assertStoreDirEmpty(storeDir, what string) {
	f.cur.Helper()
	full := filepath.Join(f.root, f.bucket, filepath.FromSlash(storeDir))
	_, err := os.Stat(full)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		f.cur.Fatalf("stat %s: %v", full, err)
	}
	err = filepath.WalkDir(full, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			f.cur.Errorf("%s: unexpected file %s — %s must not come into existence", what, path, storeDir)
		}
		return nil
	})
	if err != nil {
		f.cur.Fatalf("walk %s: %v", full, err)
	}
}

// TestManifestCustomPrefixDualEraBucketRegression is the suite. The subtests
// run in order and share one fixture, because the scenario is a sequence:
// era A leaves its state behind, era B arms over it, and each phase's
// assertions depend on the phases before it.
func TestManifestCustomPrefixDualEraBucketRegression(t *testing.T) {
	f := newCustomManifestFixture(t)

	// Era-A data: a single-part object and a completed multipart object
	// (whose stored state spans ciphertext + HMAC sidecar + geometry
	// sidecar), all at the bucket root; plus the root manifest tree the
	// era-A writer flushes.
	const (
		eraALegacy = "reports/q3.csv"
		eraAMulti  = "backups/base.tar"
		eraBNew    = "reports/q4.csv"
		eraBMulti  = "backups/incremental.tar"
	)
	want := make(map[string][]byte)

	t.Run("era A writes objects and root internal state", func(t *testing.T) {
		f.at(t)

		body := mixedEraPattern(3*65536+17, 0x10)
		if code := f.put(f.legacyAPI, eraALegacy, body); code != http.StatusOK {
			t.Fatalf("era-A PUT %s: status %d", eraALegacy, code)
		}
		want[eraALegacy] = body

		want[eraAMulti] = f.completeTwoParts(f.legacyAPI, eraAMulti, 0x20)

		// Both sidecar flavors at the ROOT names: the ADR-016 geometry
		// sidecar beside the object, the ADR-003 HMAC sidecar under the
		// root .armor/hmac/ tree — exactly what a pre-prefix era leaves.
		for _, key := range []string{
			eraAMulti,
			eraAMulti + ".armor-manifest",
			backend.GetSidecarKey("", eraAMulti),
		} {
			if !f.storedExists(key) {
				t.Fatalf("era-A multipart completion left nothing at bucket-root key %s", key)
			}
		}
		f.assertBytesFor(f.legacyAPI, want, eraAMulti)

		// Flush era A's manifest op: the delta lands at the root
		// .armor/manifest/ tree.
		flushTenantManifest(t, f.legacy)
		assertDirHasFiles(t, filepath.Join(f.root, f.bucket, ".armor", "manifest"),
			"bucket-root .armor/manifest (pre-prefix manifest deltas)")

		// The "before" census: every file in the store, hashed.
		f.before = mixedEraCensus(t, f.root)
	})

	t.Run("arming a custom manifest prefix boots on an empty custom tree", func(t *testing.T) {
		f.at(t)

		// newManifestTenantServer asserts the composed config on every
		// boot: ARMOR_MANIFEST_PREFIX must have composed onto the tenant
		// prefix, yielding tenant/.armor/manifest-v2.
		f.armed = f.serverAt(customManifestTenant, "custom-era-b", customManifestRelocation)
		f.armedAPI = f.apiFor(f.armed)

		// The index must come up EMPTY: the root deltas sit in the bucket
		// and must not be ingested (ADR-001: ingesting them is
		// cross-tenant contamination; keys are client-visible) — and the
		// custom value changes nothing about that rule, because the
		// custom tree does not exist yet.
		if got := f.armed.manifest.Len(); got != 0 {
			t.Errorf("armed instance loaded %d manifest entries from a store holding only root-era deltas, want 0; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{eraALegacy, eraAMulti} {
			if _, ok := f.armed.manifest.Get(f.bucket, key); ok {
				t.Errorf("legacy key %s resolved through the armed manifest index — root-era state was ingested", key)
			}
		}

		// Between arming and any data move the legacy objects do not exist
		// through ARMOR — the prefix rewrote the lookup and nothing probes
		// the unprefixed location, custom manifest tree or not.
		for _, key := range []string{eraALegacy, eraAMulti} {
			if code, _ := f.get(f.armedAPI, key); code != http.StatusNotFound {
				t.Errorf("pre-move GET of %s returned %d, want 404 — a prefixed deployment must not silently resolve the legacy location", key, code)
			}
		}

		// Nothing vanished bucket-side, and arming itself wrote nothing:
		// the custom tree comes into existence with the first delta, not
		// with the deployment.
		for _, key := range []string{eraALegacy, eraAMulti} {
			if !f.storedExists(key) {
				t.Errorf("legacy object %s vanished from the bucket root before any move was made", key)
			}
		}
		f.assertStoreDirEmpty(customManifestTenant+".armor/manifest-v2",
			"the custom composed manifest location")
	})

	t.Run("era B writes land in the tenant namespace and the custom tree", func(t *testing.T) {
		f.at(t)

		body := mixedEraPattern(100*1024, 0x40)
		if code := f.put(f.armedAPI, eraBNew, body); code != http.StatusOK {
			t.Fatalf("era-B PUT %s: status %d", eraBNew, code)
		}
		want[eraBNew] = body

		if f.storedExists(eraBNew) {
			t.Errorf("era-B object stored at the bare root key %s — prefix not applied on write", eraBNew)
		}
		if !f.storedExists(customManifestTenant + eraBNew) {
			t.Errorf("era-B object missing at %s", customManifestTenant+eraBNew)
		}

		// The multipart completion must write both sidecar flavors at the
		// COMPOSED names — and, the containment property this suite exists
		// for, the HMAC sidecar stays in the COMPOSED RESERVED namespace
		// (tenant/.armor/hmac/, named by sha256(prefix+key)): the custom
		// ARMOR_MANIFEST_PREFIX relocates the manifest tree ONLY, never
		// the rest of the reserved namespace, and never the root.
		want[eraBMulti] = f.completeTwoParts(f.armedAPI, eraBMulti, 0x50)

		composedHMAC := backend.GetSidecarKey(customManifestTenant, eraBMulti)
		rootHMAC := backend.GetSidecarKey("", eraBMulti)
		if !f.storedExists(composedHMAC) {
			t.Errorf("era-B completion left no HMAC sidecar at the composed name %s", composedHMAC)
		}
		for _, key := range []string{
			rootHMAC,
			eraBMulti,
			eraBMulti + ".armor-manifest",
		} {
			if f.storedExists(key) {
				t.Errorf("era-B completion wrote state at the legacy root name %s", key)
			}
		}
		if !f.storedExists(customManifestTenant + eraBMulti + ".armor-manifest") {
			t.Errorf("era-B completion left no geometry sidecar at %s", customManifestTenant+eraBMulti+".armor-manifest")
		}

		// Flush era B's ops: the deltas must appear ONLY under the custom
		// composed location. The default composed location — what this
		// same deployment would have used with ARMOR_MANIFEST_PREFIX
		// unset — must never come into existence: the custom value
		// REPLACES the default, it does not add a second manifest tree.
		flushTenantManifest(t, f.armed)
		assertDirHasFiles(t, filepath.Join(f.root, f.bucket, "tenant", ".armor", "manifest-v2"),
			"custom composed manifest deltas for era B")
		f.assertStoreDirEmpty(customManifestTenant+".armor/manifest",
			"the default composed manifest location")

		// And the custom tree holds nothing but manifest deltas — no
		// sidecar, no canary, no data may have been dragged into the
		// relocated location by the custom value.
		relocationDir := filepath.Join(f.root, f.bucket, "tenant", ".armor", "manifest-v2")
		err := filepath.WalkDir(relocationDir, func(path string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				return nil
			}
			// The FS backend keeps each object's metadata in a
			// <key>.metadata companion file (on B2 it is object metadata,
			// not a second object), so a delta's companion is part of the
			// delta, not a foreign file.
			name := d.Name()
			isDelta := strings.HasPrefix(name, "delta-") && strings.HasSuffix(name, ".jsonl")
			isDeltaMeta := strings.HasPrefix(name, "delta-") && strings.HasSuffix(name, ".jsonl.metadata")
			if !isDelta && !isDeltaMeta {
				t.Errorf("custom manifest tree holds a non-delta file: %s", path)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", relocationDir, err)
		}

		f.assertBytesFor(f.armedAPI, want, eraBNew)
		f.assertBytesFor(f.armedAPI, want, eraBMulti)

		// Containment, store-wide: every file era B added sits inside the
		// tenant namespace — with ONE documented exception (ADR-001
		// "Provenance chain — a known exception"): the chain records stay
		// at the bucket-root .armor/ tree even on a prefixed deployment,
		// which is why manifest containment matters enough to relocate
		// within the namespace in the first place. Era A's root state is
		// asserted byte-identical in the next subtest.
		after := mixedEraCensus(t, f.root)
		tenantRel := f.bucket + "/" + customManifestTenant
		rootChain := f.bucket + "/.armor/chain"
		for rel := range after {
			if _, ok := f.before[rel]; ok {
				continue
			}
			if strings.HasPrefix(rel, tenantRel) || strings.HasPrefix(rel, rootChain) {
				continue
			}
			t.Errorf("era-B write landed outside the tenant namespace and is not a chain record: %s", rel)
		}
	})

	t.Run("restart loads the custom tree; a default-configured sibling adopts nothing", func(t *testing.T) {
		f.at(t)

		// A restarted era-B instance loads exactly its own two deltas from
		// the custom tree — persistence of the relocation — and still zero
		// ingestion of the root tree sitting beside it.
		f.again = f.serverAt(customManifestTenant, "custom-era-b-2", customManifestRelocation)
		if got := f.again.manifest.Len(); got != 2 {
			t.Errorf("restarted era-B instance loaded %d manifest entries, want exactly its own 2; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{eraBNew, eraBMulti} {
			if _, ok := f.again.manifest.Get(f.bucket, key); !ok {
				t.Errorf("restarted era-B instance did not load %s from the custom tree", key)
			}
		}
		for _, key := range []string{eraALegacy, eraAMulti} {
			if _, ok := f.again.manifest.Get(f.bucket, key); ok {
				t.Errorf("restarted era-B instance resolved legacy key %s — root deltas were ingested", key)
			}
		}

		// The isolation leg: the same tenant configured with the DEFAULT
		// manifest prefix must not adopt the relocated history. Its own
		// composed tree does not exist, so its index comes up empty even
		// though era B's objects sit inside its namespace — the manifest
		// location is part of the deployment's identity, and silently
		// ingesting the relocated tree would be exactly the cross-config
		// mixing the relocation rule exists to prevent.
		f.sib = f.serverAt(customManifestTenant, "custom-era-sib", "")
		if got := f.sib.manifest.Len(); got != 0 {
			t.Errorf("default-prefix sibling loaded %d manifest entries from the relocated tree, want 0; store holds:%s",
				got, listStoreKeys(t, f.root))
		}
		for _, key := range []string{eraBNew, eraBMulti, eraALegacy, eraAMulti} {
			if _, ok := f.sib.manifest.Get(f.bucket, key); ok {
				t.Errorf("default-prefix sibling resolved %s — it must load neither the relocated tree nor the root one", key)
			}
		}

		// Every byte era A left at the root is still byte-identical: the
		// whole store outside the tenant namespace, root objects, sidecars,
		// deltas and multipart staging included.
		after := mixedEraCensus(t, f.root)
		mixedEraAssertUnchanged(t, f.before, after, func(rel string) bool {
			return !strings.HasPrefix(rel, f.bucket+"/"+customManifestTenant)
		}, "era-A root state")
	})

	t.Run("listings strip the prefix and hide both internal locations", func(t *testing.T) {
		f.at(t)

		// Full listing through the armed deployment: exactly era B's client
		// keys, none carrying the prefix, none from either internal
		// location. A prefixed listing is scoped to the tenant namespace,
		// so era A's root-era objects — still sitting at the bucket root,
		// pinned by the census — do NOT surface either: arming hides the
		// legacy era from listings exactly as it hides it from reads. The
		// root-era tree (.armor/manifest, .armor/hmac) and the era-B
		// reserved namespace (tenant/.armor/hmac,
		// tenant/.armor/manifest-v2) are all in the store and all under the
		// same both-locations predicate. The era-B ADR-016 geometry sidecar
		// DOES appear: backends hide the .armor/ namespace but not the
		// <key>.armor-manifest sidecars beside the objects (the same
		// documented visibility the forward suite pins).
		wantKeys := map[string]bool{
			eraBNew:                       true,
			eraBMulti:                     true,
			eraBMulti + ".armor-manifest": true,
		}
		keys, prefixes := f.listKeys(f.armedAPI, "", "")
		if len(keys) != len(wantKeys) {
			t.Errorf("listing returned %d keys, want %d: %v", len(keys), len(wantKeys), keys)
		}
		for _, k := range keys {
			if !wantKeys[k] {
				t.Errorf("unexpected key in listing: %s", k)
			}
			if strings.Contains(k, ".armor/") {
				t.Errorf("internal namespace leaked into a client listing: %s", k)
			}
			if strings.HasPrefix(k, customManifestTenant) {
				t.Errorf("client-visible key still carries the ARMOR_PREFIX: %s", k)
			}
		}
		if len(prefixes) != 0 {
			t.Errorf("undelimited listing produced common prefixes: %v", prefixes)
		}

		// Delimited listing: the pruned internal directories must not
		// resurface as common prefixes — neither the root-era .armor/ tree
		// nor the composed one with its custom subtree.
		_, delimPrefixes := f.listKeys(f.armedAPI, "", "/")
		got := append([]string{}, delimPrefixes...)
		sort.Strings(got)
		if want := []string{"backups/", "reports/"}; !equalStrings(got, want) {
			t.Errorf("delimiter common prefixes = %v, want %v", got, want)
		}

		// Asking for the reserved namespace by any of its names returns
		// nothing: the client form (root and composed), and the stored
		// prefix itself, which is not part of the client key space.
		for _, p := range []string{
			".armor/",
			customManifestTenant + ".armor/",
			customManifestTenant + ".armor/manifest-v2/",
			customManifestTenant,
		} {
			internal, _ := f.listKeys(f.armedAPI, p, "")
			if len(internal) != 0 {
				t.Errorf("prefix=%q listing returned %d objects, want 0: %v", p, len(internal), internal)
			}
		}

		// Clients are refused the reserved namespace outright — the bare
		// .armor/ form, read and write. The root-era delta is named here by
		// its concrete stored name (era-A writer, first delta) so the
		// refusal is pinned against an object that really exists.
		if code, _ := f.get(f.armedAPI, ".armor/manifest/custom-era-a/delta-0000000001.jsonl"); code != http.StatusForbidden {
			t.Errorf("GET of a root internal object returned %d, want 403", code)
		}
		if code := f.put(f.armedAPI, ".armor/evil", []byte("x")); code != http.StatusForbidden {
			t.Errorf("PUT into .armor/ returned %d, want 403", code)
		}
		// The composed custom tree's STORED name is not in the client key
		// space either: a client key of that shape would store at
		// tenant/tenant/.armor/manifest-v2/…, so the internal object cannot
		// be reached by its stored name — 404, not the delta's bytes.
		if code, _ := f.get(f.armedAPI, customManifestTenant+".armor/manifest-v2/custom-era-b/delta-0000000001.jsonl"); code != http.StatusNotFound {
			t.Errorf("GET of the stored custom-tree internal name returned %d, want 404 — internal state must not be reachable by its stored name", code)
		}
	})
}
