package srvtest

// Provider-outage restore drill: the committed, repeatable form of Route A in
// docs/disaster-recovery.md ("B2 Account or Bucket Gone: Provider-Outage
// Recovery"). A mixed corpus — single-PUT objects and multipart objects in
// both envelope generations — is written through the real authenticated mux,
// the replication queue drains completely, the primary is destroyed, and a
// fresh ARMOR server stood up on the SECONDARY alone with nothing but the
// escrowed MEK must serve every single-PUT object byte-identically with
// integrity verified against the metadata that replicated alongside it.
//
// The drill also pins the two honest boundaries the runbook documents rather
// than hiding them:
//
//   - Multipart objects are NOT recoverable from the mirror as plaintext.
//     Their ciphertext replicates (they stay listed and HEAD-able), but their
//     ARMOR metadata does not: CreateMultipartUpload stores no metadata on
//     the data object (ADR-016 keeps it in the <stored-key>.armor-manifest
//     sidecar), and the queue replicates only the data key, so the manifest,
//     the .armor/hmac/ sidecar, and the multipart marker are all absent from
//     the mirror. On a promoted replica the read path's manifest lookup
//     misses, the legacy probe sees an object with no ARMOR metadata, and the
//     non-ARMOR passthrough serves the raw ciphertext bytes with a 200. The
//     drill pins the property that matters — the original plaintext is never
//     served, and the body is exactly the replicated ciphertext — asserted
//     here so a semantics change that silently narrows (or widens) the
//     mirror's recoverable set trips a test. Note this is observably
//     different from docs/disaster-recovery.md Route A step 3's "Multipart
//     objects fail to read": the failure mode is an opaque 200 passthrough,
//     not a read error; the doc correction is recorded on this bead for the
//     operator runbook (armor-b2235418) to pick up.
//   - The async loss window (ADR-006 decision #6) is real: a write acked
//     while the secondary is unreachable is legitimately absent at failover.
//     The drained case is what this drill asserts; the window itself has its
//     own test below.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/server"
)

// Restore-drill credentials: material issued to the recovery deployment
// AFTER the outage. They are deliberately distinct constants from
// TestAccessKey/TestSecretKey so the restore leg provably carries no auth
// material from the failed deployment — the MEK is the only shared secret.
const (
	restoreAccessKey = "armor-outage-restore-access-key"  // gitleaks:allow
	restoreSecretKey = "armor-outage-restore-signing-key" // gitleaks:allow
)

// outageObject is one corpus member: the client key, the exact plaintext
// written pre-outage (multipart members carry the part concatenation), the
// pre-outage ETag, and whether it was stored via CompleteMultipartUpload.
type outageObject struct {
	key       string
	plaintext []byte
	etag      string
	multipart bool
}

// seedOutageCorpus writes a mixed corpus through the real write paths:
// three single-PUT objects (a one-block object, a multi-block incompressible
// one, a highly compressible one) and two multipart objects (ADR-015
// uniform parts, and varied parts whose non-block-aligned part 1 forces
// ADR-011 non-uniform mode in v2 / the contract-free v3 layout).
func seedOutageCorpus(t *testing.T, h *Harness) []outageObject {
	t.Helper()

	tiny := []byte("provider-outage drill: tiny single-PUT object, one partial block")
	random := RandomBytes(t, 200*1024+37)
	compressible := bytes.Repeat([]byte("compressible-drill-pattern."), 11*1024)

	objs := []outageObject{
		{key: "drill/tiny.txt", plaintext: tiny},
		{key: "drill/random-200k.bin", plaintext: random},
		{key: "drill/compressible.txt", plaintext: compressible},
	}
	for i := range objs {
		objs[i].etag = h.PutObject(h.Bucket, objs[i].key, objs[i].plaintext)
	}

	uniformParts := [][]byte{
		RandomBytes(t, minMultipartPart),
		RandomBytes(t, 128*1024+9),
	}
	variedParts := [][]byte{
		RandomBytes(t, minMultipartPart+37),
		RandomBytes(t, 3*1024*1024+5*1024),
		RandomBytes(t, 1024+3),
	}
	for _, mp := range []struct {
		key   string
		parts [][]byte
	}{
		{"drill/uniform-multipart.bin", uniformParts},
		{"drill/varied-multipart.bin", variedParts},
	} {
		etag := h.MultipartUpload(h.Bucket, mp.key, mp.parts...)
		objs = append(objs, outageObject{
			key:       mp.key,
			plaintext: bytes.Join(mp.parts, nil),
			etag:      etag,
			multipart: true,
		})
	}
	return objs
}

// destroyPrimary simulates the provider outage itself: every object the
// primary backend holds — data, manifests, HMAC sidecars, multipart state —
// is deleted and the deletion verified. The restore server never references
// the primary at all, so this is belt-and-braces: any code path that somehow
// reached the primary would find nothing there.
func destroyPrimary(t *testing.T, h *Harness) {
	t.Helper()
	ctx := context.Background()
	res, err := h.Primary.ListRaw(ctx, h.Bucket, "", "", "", 0)
	if err != nil {
		t.Fatalf("primary ListRaw during outage simulation: %v", err)
	}
	for _, obj := range res.Objects {
		if err := h.Primary.Delete(ctx, h.Bucket, obj.Key); err != nil {
			t.Fatalf("primary Delete %q during outage simulation: %v", obj.Key, err)
		}
	}
	left, err := h.Primary.ListRaw(ctx, h.Bucket, "", "", "", 0)
	if err != nil {
		t.Fatalf("primary ListRaw verification: %v", err)
	}
	if len(left.Objects) != 0 {
		t.Fatalf("primary still holds %d objects after the outage simulation", len(left.Objects))
	}
}

// newRestoreHandler stands up the recovery deployment the way Route A step 3
// prescribes: a fresh ARMOR server pointed at the secondary backend,
// configured with the escrowed MEK and the matching ARMOR_PREFIX, issued
// brand-new credentials, wired to no primary and no replication queue. The
// MEK is the only value that crosses from the failed deployment.
func newRestoreHandler(t *testing.T, h *Harness, formatVersion int) http.Handler {
	t.Helper()
	cfg := &config.Config{
		BlockSize:          64 * 1024,
		MEK:                h.MEK,
		B2Region:           TestRegion,
		Prefix:             h.Prefix,
		FormatWriteVersion: formatVersion,
		Credentials: map[string]*config.Credential{
			restoreAccessKey: {AccessKey: restoreAccessKey, SecretKey: restoreSecretKey},
		},
		CacheMaxEntries: 1000,
		CacheTTL:        300,
	}
	srv, err := server.NewWithBackend(cfg, h.Secondary.Backend)
	if err != nil {
		t.Fatalf("restore server: %v", err)
	}
	return srv.Handler()
}

// restoreDo issues a request against a restore deployment under the recovery
// credentials — the failed deployment's credentials never appear here.
func restoreDo(t *testing.T, handler http.Handler, method, target string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	SignS3Request(req, body, restoreAccessKey, restoreSecretKey, TestRegion)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// TestProviderOutageRestoreFromSecondary is the Route A drill proper. For
// both envelope generations: seed the corpus, drain replication completely,
// destroy the primary, then serve every object from a fresh deployment on
// the secondary alone with the MEK as the only carried-over secret.
func TestProviderOutageRestoreFromSecondary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
	}{
		{"v2-envelopes", 0},
		{"v3-envelopes", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewWithConfig(t, HarnessConfig{FormatWriteVersion: tc.version})
			objs := seedOutageCorpus(t, h)

			// Pre-outage sanity: every corpus member round-trips on the
			// primary deployment, multipart included — the limitations
			// asserted below are mirror limitations, not corpus defects.
			for _, o := range objs {
				rec := h.Do(http.MethodGet, "/"+h.Bucket+"/"+o.key, nil)
				if rec.Code != http.StatusOK {
					t.Fatalf("pre-outage GET %s: status %d: %s", o.key, rec.Code, rec.Body.String())
				}
				if !bytes.Equal(rec.Body.Bytes(), o.plaintext) {
					t.Fatalf("pre-outage GET %s: body mismatch (%d vs %d bytes)", o.key, rec.Body.Len(), len(o.plaintext))
				}
			}

			// Drain completely before the outage — this drill asserts the
			// drained case; the in-flight window has its own test.
			stored := make([]string, len(objs))
			for i, o := range objs {
				stored[i] = h.StoredKey(o.key)
			}
			h.WaitUntilEnqueued(int64(len(objs)), WaitTimeout)
			h.WaitUntilReplicated(stored, WaitTimeout)
			h.AssertLandingSet(stored)
			h.AssertQueueDrained(int64(len(objs)))

			// Failover: replication stops, then the provider takes everything.
			h.cancel()
			h.Queue.Stop()
			destroyPrimary(t, h)

			handler := newRestoreHandler(t, h, tc.version)
			for _, o := range objs {
				if o.multipart {
					assertMultipartMirrorLimitation(t, handler, h, o)
					continue
				}
				assertSinglePutRestored(t, handler, h, o)
			}
			assertRestoreListing(t, handler, h, objs)

			// A key that never existed must 404 through the restore path.
			rec := restoreDo(t, handler, http.MethodGet, "/"+h.Bucket+"/drill/never-existed.bin", nil)
			if rec.Code != http.StatusNotFound {
				t.Errorf("restore GET of a never-existing key: status %d, want 404", rec.Code)
			}
		})
	}
}

// assertSinglePutRestored proves full recovery of one single-PUT member:
// 200, byte-identical plaintext, the pre-outage ETag, and a plaintext digest
// match against the x-amz-meta-armor-plaintext-sha256 that replicated with
// the object (runbook step 4's validation, not just an in-memory comparison).
func assertSinglePutRestored(t *testing.T, handler http.Handler, h *Harness, o outageObject) {
	t.Helper()
	rec := restoreDo(t, handler, http.MethodGet, "/"+h.Bucket+"/"+o.key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore GET %s: status %d: %s", o.key, rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), o.plaintext) {
		t.Fatalf("restore GET %s: plaintext mismatch: got %d bytes, want %d", o.key, rec.Body.Len(), len(o.plaintext))
	}
	if got := strings.Trim(rec.Header().Get("ETag"), `"`); got != o.etag {
		t.Errorf("restore GET %s: ETag %q, want the pre-outage ETag %q", o.key, got, o.etag)
	}

	info, err := h.Secondary.Backend.Head(context.Background(), h.Bucket, h.StoredKey(o.key))
	if err != nil {
		t.Fatalf("secondary Head %s: %v", o.key, err)
	}
	want := info.Metadata["x-amz-meta-armor-plaintext-sha256"]
	sum := sha256.Sum256(rec.Body.Bytes())
	if got := hex.EncodeToString(sum[:]); got != want {
		t.Errorf("restore GET %s: plaintext sha256 %s, want the replicated metadata digest %s", o.key, got, want)
	}
}

// assertMultipartMirrorLimitation pins the documented unrecoverable set: a
// multipart object's ciphertext replicates, so the key stays listed and
// HEAD-able from the mirror, but its ARMOR metadata does not — the manifest
// sidecar, the .armor/hmac/ sidecar, and the multipart marker are all
// primary-only. The GET that a promoted replica serves for such an object
// takes the non-ARMOR passthrough (manifest miss → no ARMOR metadata →
// opaque blob) and returns the raw ciphertext bytes with a 200. The two
// properties asserted are the ones the drill exists to guarantee: the
// original plaintext is never served, and the body is exactly the replicated
// ciphertext. See the file header for the divergence this pins against
// docs/disaster-recovery.md Route A step 3.
func assertMultipartMirrorLimitation(t *testing.T, handler http.Handler, h *Harness, o outageObject) {
	t.Helper()
	hrec := restoreDo(t, handler, http.MethodHead, "/"+h.Bucket+"/"+o.key, nil)
	if hrec.Code != http.StatusOK {
		t.Fatalf("restore HEAD %s: status %d, want 200 (multipart ciphertext replicates)", o.key, hrec.Code)
	}
	if got := hrec.Header().Get("Content-Length"); got != strconv.FormatInt(int64(len(o.plaintext)), 10) {
		t.Errorf("restore HEAD %s: Content-Length %s, want the replicated ciphertext size %d (CTR keeps ciphertext the size of the plaintext)", o.key, got, len(o.plaintext))
	}
	if got := strings.Trim(hrec.Header().Get("ETag"), `"`); got != o.etag {
		t.Errorf("restore HEAD %s: ETag %q, want the pre-outage ETag %q", o.key, got, o.etag)
	}

	// The mirror must never hand back the original plaintext for an object
	// whose decrypt material (manifest, HMAC sidecar) never replicated.
	grec := restoreDo(t, handler, http.MethodGet, "/"+h.Bucket+"/"+o.key, nil)
	if bytes.Equal(grec.Body.Bytes(), o.plaintext) {
		t.Fatalf("restore GET %s served the original plaintext — the mirror recovered a multipart object whose decrypt material never replicated", o.key)
	}

	// What it serves instead is the replicated ciphertext itself: with the
	// manifest absent the object carries no ARMOR metadata, so the read falls
	// to the non-ARMOR passthrough and streams the stored bytes opaquely
	// (200, not the read error the runbook describes — see file header).
	body, _, err := h.Secondary.Backend.Get(context.Background(), h.Bucket, h.StoredKey(o.key))
	if err != nil {
		t.Fatalf("secondary Get %s: %v", o.key, err)
	}
	raw, err := io.ReadAll(body)
	body.Close()
	if err != nil {
		t.Fatalf("secondary read %s: %v", o.key, err)
	}
	if grec.Code != http.StatusOK {
		t.Errorf("restore GET %s: status %d, want the passthrough 200 pinned by this drill (a different failure mode means the mirror contract changed)", o.key, grec.Code)
	}
	if !bytes.Equal(grec.Body.Bytes(), raw) {
		t.Errorf("restore GET %s: body is not the replicated ciphertext verbatim (got %d bytes, mirror holds %d)", o.key, grec.Body.Len(), len(raw))
	}
}

// assertRestoreListing proves object enumeration survives the outage: the
// restore listing returns exactly the corpus members — every client object
// present, and no leaked internal state or manifest sidecars.
func assertRestoreListing(t *testing.T, handler http.Handler, h *Harness, objs []outageObject) {
	t.Helper()
	rec := restoreDo(t, handler, http.MethodGet, "/"+h.Bucket+"?list-type=2", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore ListObjectsV2: status %d: %s", rec.Code, rec.Body.String())
	}
	var lr struct {
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &lr); err != nil {
		t.Fatalf("restore ListObjectsV2: parse: %v", err)
	}
	got := make(map[string]bool, len(lr.Contents))
	for _, c := range lr.Contents {
		got[c.Key] = true
	}
	for _, o := range objs {
		if !got[o.key] {
			t.Errorf("restore listing is missing %q", o.key)
		}
	}
	for k := range got {
		if strings.HasSuffix(k, ".armor-manifest") || IsInternalKey(k) {
			t.Errorf("restore listing exposed internal ARMOR state %q", k)
		}
	}
	if len(got) != len(objs) {
		t.Errorf("restore listing returned %d keys, want exactly the %d corpus members", len(got), len(objs))
	}
}

// TestProviderOutageRestoreDetectsSecondaryCorruption proves the restore leg
// verifies integrity rather than just decrypting: one ciphertext byte flipped
// on the mirror (silent bit-rot on the standby) must never yield the original
// plaintext. The read path checks every block's HMAC and the whole-object
// digest recorded in the replicated metadata, so a corrupted mirror object
// fails loudly mid-stream instead of serving wrong bytes.
func TestProviderOutageRestoreDetectsSecondaryCorruption(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
	}{
		{"v2-envelopes", 0},
		{"v3-envelopes", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := NewWithConfig(t, HarnessConfig{FormatWriteVersion: tc.version})
			const key = "drill/tampered.bin"
			plaintext := RandomBytes(t, 200*1024+37)
			h.PutObject(h.Bucket, key, plaintext)
			stored := h.StoredKey(key)
			h.WaitUntilEnqueued(1, WaitTimeout)
			h.WaitUntilReplicated([]string{stored}, WaitTimeout)

			h.cancel()
			h.Queue.Stop()
			destroyPrimary(t, h)

			// Flip one ciphertext byte on the mirror, preserving metadata so
			// the restore path attempts a genuine decrypt that must fail
			// verification.
			ctx := context.Background()
			body, info, err := h.Secondary.Backend.Get(ctx, h.Bucket, stored)
			if err != nil {
				t.Fatalf("secondary Get: %v", err)
			}
			data, err := io.ReadAll(body)
			body.Close()
			if err != nil {
				t.Fatalf("secondary read: %v", err)
			}
			data[crypto.HeaderSize+64*1024] ^= 0xFF // first byte of block 2's ciphertext
			if err := h.Secondary.Backend.Put(ctx, h.Bucket, stored, bytes.NewReader(data), int64(len(data)), info.Metadata); err != nil {
				t.Fatalf("secondary re-Put of tampered bytes: %v", err)
			}

			handler := newRestoreHandler(t, h, tc.version)
			rec := restoreDo(t, handler, http.MethodGet, "/"+h.Bucket+"/"+key, nil)
			if bytes.Equal(rec.Body.Bytes(), plaintext) {
				t.Fatalf("restore served the full original plaintext from corrupted mirror ciphertext — HMAC/digest verification did not protect the read")
			}
			t.Logf("tampered mirror GET: status %d, %d body bytes, original plaintext not served", rec.Code, rec.Body.Len())
		})
	}
}

// TestProviderOutageRestoreAsyncWindowLoss pins the honest boundary of the
// drill (ADR-006 decision #6): replication is asynchronous after the client
// ack, so a write acknowledged while the secondary is unreachable is
// legitimately absent from the mirror if the provider dies inside that
// window. The restore deployment 404s it — an in-flight window loss
// (retries recorded, nothing dropped), not a queue drop — and once the
// secondary recovers, the queue catches up and the same restore path serves
// the object byte-identically.
func TestProviderOutageRestoreAsyncWindowLoss(t *testing.T) {
	h := New(t)
	h.Secondary.SetPutError(errors.New("injected secondary outage: provider-outage drill window"))

	const key = "drill/in-flight.bin"
	plaintext := RandomBytes(t, 128*1024+3)
	h.PutObject(h.Bucket, key, plaintext)
	h.WaitUntilEnqueued(1, WaitTimeout)
	pollFor(t, WaitTimeout, "queue to record retries against the faulted secondary", func() bool {
		return h.QueueMetrics.RetriesTotal.Load() > 0
	})

	// Failover inside the window: the mirror does not have the object...
	handler := newRestoreHandler(t, h, 0)
	rec := restoreDo(t, handler, http.MethodGet, "/"+h.Bucket+"/"+key, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("restore GET during the async window: status %d, want 404", rec.Code)
	}
	if got := h.QueueMetrics.DroppedTotal.Load(); got != 0 {
		t.Errorf("DroppedTotal = %d during the async window, want 0 (the loss is the window, not a drop)", got)
	}

	// ...and secondary recovery closes the window: the queue catches up and a
	// restore started afterwards serves the object byte-identically.
	h.Secondary.SetPutError(nil)
	h.WaitUntilReplicated([]string{h.StoredKey(key)}, WaitTimeout)
	handler = newRestoreHandler(t, h, 0)
	rec = restoreDo(t, handler, http.MethodGet, "/"+h.Bucket+"/"+key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("restore GET after the window closed: status %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(rec.Body.Bytes(), plaintext) {
		t.Fatalf("restore GET after the window closed: plaintext mismatch (%d vs %d bytes)", rec.Body.Len(), len(plaintext))
	}
}
