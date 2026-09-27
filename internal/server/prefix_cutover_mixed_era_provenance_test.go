package server

// The provenance leg of the mixed-era ARMOR_PREFIX coverage — runbook §6
// check 6, the one verification class of docs/runbooks/prefix-cutover-and-
// legacy-state.md that TestPrefixCutoverMixedEraBucketRegression does not
// drive (that suite covers reads, writes, listings, finalization, and the
// manifest split). Two real deployments live through the cutover over one
// store: era A with no prefix, era B with the tenant prefix armed. By the
// audit the bucket holds root-era internal state (.armor/manifest deltas and
// a chain head from era A) beside prefixed internal state (era B's composed
// deltas) and era B's own chain head, which the ADR-001 exception keeps at
// the bucket root.
//
// The audit the armed deployment's /armor/audit handler runs must resolve
// each internal namespace by its own rule:
//
//	chain records   BOTH eras' heads at the bucket root — the namespace
//	               that never composes onto ARMOR_PREFIX, so the audit
//	               discovers both writers (compose the listing and every
//	               prefixed deployment's audit would come up empty)
//	deltas          era B's entries verified end-to-end from the COMPOSED
//	               tree only; era A's root deltas are nobody's history now
//	               (ingesting them is the ADR-001 cross-tenant
//	               contamination), so its writer surfaces as the honest
//	               unverifiable-history gap, not a verified chain
//	objects         the untracked cross-reference lists the tenant scope
//	               only — era A's ciphertext sitting at the bucket root is
//	               out of namespace and never flagged untracked
//	clients         none of it is client-visible: the root and composed
//	               internal trees stay hidden from listings and GETs
//
// The unit-level namespace rules this builds on are pinned in
// internal/provenance (manifest_delta_walk_prefix_test.go for the composed
// delta walk); this suite pins them through the real server wiring — the
// handlers' chain-entry embedding, the manifest writer's dual placement, and
// the auditor the admin endpoint constructs.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/provenance"
)

func TestPrefixCutoverMixedEraProvenanceNamespaceResolution(t *testing.T) {
	const (
		writerLegacy = "prov-era-legacy"
		writerArmed  = "prov-era-armed"

		legacyA = "reports/q3-a.csv"
		legacyB = "reports/q3-b.csv"
		newA    = "reports/q4-a.csv"
		newB    = "reports/q4-b.csv"
	)

	root := t.TempDir()
	store, err := backend.NewFSBackend(backend.FSConfig{BasePath: root})
	if err != nil {
		t.Fatalf("bucket-side store handle: %v", err)
	}
	const bucket = "shared-bucket"

	// storedExists is the bucket-side existence probe (operator view, never
	// a client path). The FS backend surfaces a missing object as a raw
	// os.ErrNotExist chain, not a typed backend error.
	storedExists := func(key string) bool {
		t.Helper()
		_, err := store.Head(context.Background(), bucket, key)
		if err == nil {
			return true
		}
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		t.Fatalf("Head %s: %v", key, err)
		return false
	}

	// serverFor boots one full deployment over the shared store and exposes
	// it through a real HTTP server with a SigV4 client, the same shape the
	// mixed-era fixture uses.
	serverFor := func(tenantPrefix, writerID string) (*Server, *s3.Client) {
		t.Helper()
		srv := newManifestTenantServer(t, root, tenantPrefix, writerID, "")
		t.Cleanup(srv.StopManifestCompactor)
		ts := httptest.NewServer(srv.Handler())
		t.Cleanup(ts.Close)
		return srv, newListingS3Client(t, ts.URL, "test-access-key", "test-secret-key")
	}

	put := func(client *s3.Client, key string, body []byte) {
		t.Helper()
		if _, err := client.PutObject(context.Background(), &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(body),
		}); err != nil {
			t.Fatalf("PUT %s: %v", key, err)
		}
	}

	// getStatus issues a GET and maps an SDK error to its HTTP status,
	// failing on anything it does not recognize so assertions never pass on
	// a guessed status.
	getStatus := func(client *s3.Client, key string) int {
		t.Helper()
		out, err := client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})
		if err == nil {
			out.Body.Close()
			return http.StatusOK
		}
		var re *smithyhttp.ResponseError
		if !errors.As(err, &re) {
			t.Fatalf("GET %s: unrecognized error: %v", key, err)
		}
		return re.HTTPStatusCode()
	}

	// --- era A: the pre-prefix deployment ------------------------------

	legacy, legacyAPI := serverFor("", writerLegacy)
	put(legacyAPI, legacyA, mixedEraPattern(8192, 0x11))
	put(legacyAPI, legacyB, mixedEraPattern(8192, 0x12))
	flushTenantManifest(t, legacy)

	// Era A leaves its internal state at the bucket root: manifest deltas
	// under .armor/manifest/ and — the ADR-001 exception, written by the
	// manifest writer after each delta upload — a chain head under
	// .armor/chain-head/. Nothing exists under the tenant prefix yet.
	assertDirHasFiles(t, filepath.Join(root, bucket, ".armor", "manifest", writerLegacy),
		"era-A root manifest deltas")
	if !storedExists(".armor/chain-head/" + writerLegacy) {
		t.Fatalf("era-A chain head missing at the bucket root; store holds:%s",
			listStoreKeys(t, root))
	}
	if _, err := os.Stat(filepath.Join(root, bucket, strings.SplitN(mixedEraPrefix, "/", 2)[0])); !os.IsNotExist(err) {
		t.Fatalf("store already holds a %q tree before any prefixed deployment booted (stat err: %v)",
			mixedEraPrefix, err)
	}

	// rootInternal matches era A's frozen internal state: its delta tree and
	// its own chain head. Era B's head at the root is legitimate new state
	// and must not be part of the immutability set.
	rootInternal := func(rel string) bool {
		return strings.HasPrefix(rel, bucket+"/.armor/manifest/") ||
			rel == bucket+"/.armor/chain-head/"+writerLegacy
	}
	afterEraA := mixedEraCensus(t, root)

	// --- era B: the same store with the prefix armed --------------------

	armed, armedAPI := serverFor(mixedEraPrefix, writerArmed)
	put(armedAPI, newA, mixedEraPattern(8192, 0x21))
	put(armedAPI, newB, mixedEraPattern(8192, 0x22))
	flushTenantManifest(t, armed)

	// The dual placement: era B's chain head lands at the bucket ROOT (the
	// namespace that never composes), while its manifest deltas land under
	// the composed tenant tree — never at the root manifest location.
	if !storedExists(".armor/chain-head/" + writerArmed) {
		t.Fatalf("era-B chain head missing at the bucket root — the chain namespace must not compose onto the prefix; store holds:%s",
			listStoreKeys(t, root))
	}
	assertDirHasFiles(t,
		filepath.Join(root, bucket, strings.SplitN(mixedEraPrefix, "/", 2)[0], ".armor", "manifest", writerArmed),
		"era-B composed manifest deltas")
	if _, err := os.Stat(filepath.Join(root, bucket, ".armor", "manifest", writerArmed)); !os.IsNotExist(err) {
		t.Errorf("era-B wrote manifest deltas at the root .armor/manifest/ tree (stat err: %v)", err)
	}

	// Era A's root internal state is untouched by the armed era (runbook
	// gate 3): same files, same bytes.
	mixedEraAssertUnchanged(t, afterEraA, mixedEraCensus(t, root), rootInternal,
		"era-A root internal state after arming")

	// --- the audit through the armed deployment -------------------------

	rec := httptest.NewRecorder()
	armed.audit(rec, httptest.NewRequest(http.MethodGet, "/armor/audit", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("audit returned %d: %s", rec.Code, rec.Body.String())
	}
	var result provenance.AuditResult
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}

	// Chain records resolve at the bucket root, so the audit discovers BOTH
	// eras' writers. A listing composed onto the prefix would find neither
	// head — every prefixed deployment's audit would come up empty.
	if len(result.Writers) != 2 {
		t.Fatalf("audit discovered %d writers, want exactly the two eras' (%s, %s): %+v",
			len(result.Writers), writerLegacy, writerArmed, result.Writers)
	}
	rows := make(map[string]provenance.WriterAudit)
	for _, w := range result.Writers {
		rows[w.WriterID] = w
	}
	rowArmed, rowLegacy := rows[writerArmed], rows[writerLegacy]

	// Era B verifies end-to-end from its composed deltas: both entries
	// walked, the chain linked back to genesis, no error.
	if !rowArmed.Valid {
		t.Errorf("era-B writer chain not verified: %+v", rowArmed)
	}
	if rowArmed.HeadSequence != 2 || rowArmed.EntriesVerified != 2 {
		t.Errorf("era-B writer audited head %d / verified %d, want 2/2: %+v",
			rowArmed.HeadSequence, rowArmed.EntriesVerified, rowArmed)
	}
	if rowArmed.Error != "" {
		t.Errorf("era-B writer audit error: %s", rowArmed.Error)
	}

	// Era A's head was found and parsed (namespace: root) but its history
	// cannot be verified through the armed audit: its entries live in root
	// deltas, which are nobody's index now. The honest outcome is zero
	// entries verified and a reported gap — the runbook's "root deltas stay
	// and are never ingested" showing up in the audit, not corruption.
	if rowLegacy.Valid {
		t.Errorf("era-A writer chain verified through the armed audit — root-era deltas were ingested: %+v", rowLegacy)
	}
	if rowLegacy.HeadSequence != 2 {
		t.Errorf("era-A writer head sequence = %d, want 2 (head found and parsed at the root)", rowLegacy.HeadSequence)
	}
	if rowLegacy.EntriesVerified != 0 {
		t.Errorf("era-A writer verified %d entries, want 0 (composed walk only)", rowLegacy.EntriesVerified)
	}
	if len(result.Gaps) != 1 || result.Gaps[0].WriterID != writerLegacy {
		t.Errorf("audit reported gaps %+v, want exactly one for %s", result.Gaps, writerLegacy)
	}
	if result.Status != "invalid" {
		t.Errorf("overall audit status = %q, want \"invalid\" (the unverifiable era-A history), errors: %v",
			result.Status, result.Errors)
	}

	// The untracked cross-reference resolves the OBJECT namespace as the
	// tenant scope: era B's two objects counted and tracked, era A's
	// ciphertext at the bucket root out of namespace and never flagged, the
	// composed internal tree skipped.
	if result.TotalObjects != 2 {
		t.Errorf("audit counted %d objects, want 2 (the tenant scope only): %+v", result.TotalObjects, result)
	}
	if len(result.UntrackedObjects) != 0 {
		t.Errorf("audit flagged untracked objects in the mixed-era bucket: %v", result.UntrackedObjects)
	}
	if result.TotalEntries != 2 {
		t.Errorf("audit verified %d total entries, want 2 (era B's only): %+v", result.TotalEntries, result)
	}

	// --- none of it is client-visible -----------------------------------

	out, err := armedAPI.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		t.Fatalf("ListObjectsV2: %v", err)
	}
	got := make(map[string]bool)
	for _, o := range out.Contents {
		got[aws.ToString(o.Key)] = true
	}
	want := map[string]bool{newA: true, newB: true}
	if len(got) != len(want) {
		t.Errorf("armed listing returned %v, want exactly its own era's keys %v", got, want)
	}
	for k := range got {
		if !want[k] {
			t.Errorf("unexpected key in armed listing: %s", k)
		}
		if strings.Contains(k, ".armor/") {
			t.Errorf("internal namespace leaked into a client listing: %s", k)
		}
	}

	// The root chain records exist bucket-side (asserted above) but a
	// client cannot reach the reserved namespace: the bare form is refused
	// outright.
	if code := getStatus(armedAPI, ".armor/chain-head/"+writerArmed); code != http.StatusForbidden {
		t.Errorf("GET of a chain-head object by its client-form name returned %d, want 403", code)
	}

	// The audit is read-only: era A's root internal state is byte-identical
	// to its pre-audit census.
	mixedEraAssertUnchanged(t, afterEraA, mixedEraCensus(t, root), rootInternal,
		"era-A root internal state after the audit")
}
