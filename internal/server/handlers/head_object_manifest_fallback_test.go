package handlers_test

// Coverage for armor-c62898b8: HeadObject's fallback (manifest index absent —
// disabled at startup, or lost on a degraded restart) must consult the
// manifest object the way GetObject does. ADR-016 multipart objects carry
// their metadata only on the <key>.armor-manifest object — the assembled
// ciphertext has none — so a bare ciphertext Head used to serve a zero-value
// ETag and Content-Type, and aws-cli's If-Match multipart download 412'd on
// every part.

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/server/handlers"
)

const (
	headFallbackBucket = "test-bucket"
	headFallbackKey    = "head-fallback/multipart-object.bin"
	headFallbackPart   = 5 * 1024 * 1024 // ADR-015 minimum uniform part size
	headFallbackCType  = "application/x-parquet"
)

// assemblingMultipartBackend extends the package mock backend with real
// UploadPart storage and ciphertext assembly, mirroring what B2 does at
// CompleteMultipartUpload — the plain mock's stubs discard parts and store no
// object, leaving nothing for a HEAD or GET to head.
type assemblingMultipartBackend struct {
	*mockBackend
	parts map[string]map[int][]byte
}

func newAssemblingMultipartBackend() *assemblingMultipartBackend {
	return &assemblingMultipartBackend{
		mockBackend: newMockBackend(),
		parts:       map[string]map[int][]byte{},
	}
}

func (m *assemblingMultipartBackend) UploadPart(_ context.Context, _, _, uploadID string, partNumber int32, body io.Reader, _ int64) (string, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.parts[uploadID] == nil {
		m.parts[uploadID] = map[int][]byte{}
	}
	m.parts[uploadID][int(partNumber)] = data
	return fmt.Sprintf("etag-%d", partNumber), nil
}

func (m *assemblingMultipartBackend) CompleteMultipartUpload(_ context.Context, bucket, key, uploadID string, parts []backend.CompletedPart) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	stored, ok := m.parts[uploadID]
	if !ok {
		return "", fmt.Errorf("unknown upload id: %s", uploadID)
	}
	ordered := make([]backend.CompletedPart, len(parts))
	copy(ordered, parts)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].PartNumber < ordered[j].PartNumber })
	var buf bytes.Buffer
	for _, p := range ordered {
		buf.Write(stored[int(p.PartNumber)])
	}
	// The assembled ciphertext sits at the logical key with no ARMOR metadata
	// (the manifest object carries the metadata), and predates the manifest's
	// completedAt — the ADR-016 ordering the freshness gate expects.
	m.objects[bucket+"/"+key] = buf.Bytes()
	m.modified[bucket+"/"+key] = time.Now().Add(-time.Second)
	delete(m.parts, uploadID)
	return fmt.Sprintf("final-etag-%d", buf.Len()), nil
}

// countingAssemblingBackend counts ciphertext Head calls so a test can tell a
// fallback-path HEAD (which heads the ciphertext) from an index hit (which
// does not).
type countingAssemblingBackend struct {
	*assemblingMultipartBackend
	headCalls atomic.Int64
}

func (c *countingAssemblingBackend) Head(ctx context.Context, bucket, key string) (*backend.ObjectInfo, error) {
	c.headCalls.Add(1)
	return c.assemblingMultipartBackend.Head(ctx, bucket, key)
}

// completeMultipartThroughHandlers drives CreateMultipartUpload, two uniform
// 5 MiB UploadParts, and CompleteMultipartUpload through HandleRoot — the
// flow that writes the manifest object carrying the completion metadata — and
// returns the completion ETag.
func completeMultipartThroughHandlers(t *testing.T, h *handlers.Handlers) string {
	t.Helper()

	createReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploads", headFallbackBucket, headFallbackKey), strings.NewReader(
		`<CreateMultipartUploadRequest xmlns="http://s3.amazonaws.com/doc/2006-03-01/"></CreateMultipartUploadRequest>`))
	createReq.Header.Set("Content-Type", headFallbackCType)
	createW := httptest.NewRecorder()
	h.HandleRoot(createW, createReq)
	if createW.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload failed: status %d, body %s", createW.Code, createW.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.NewDecoder(createW.Body).Decode(&created); err != nil {
		t.Fatalf("parse CreateMultipartUpload result: %v", err)
	}

	etags := make([]string, 2)
	for n := 1; n <= 2; n++ {
		part := make([]byte, headFallbackPart)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/%s/%s?partNumber=%d&uploadId=%s", headFallbackBucket, headFallbackKey, n, created.UploadID), bytes.NewReader(part))
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("UploadPart %d failed: status %d, body %s", n, w.Code, w.Body.String())
		}
		etags[n-1] = w.Header().Get("ETag")
	}

	completeReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploadId=%s", headFallbackBucket, headFallbackKey, created.UploadID), strings.NewReader(fmt.Sprintf(
		`<CompleteMultipartUpload xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
			<Part><PartNumber>1</PartNumber><ETag>%s</ETag></Part>
			<Part><PartNumber>2</PartNumber><ETag>%s</ETag></Part>
		</CompleteMultipartUpload>`, etags[0], etags[1])))
	completeW := httptest.NewRecorder()
	h.HandleRoot(completeW, completeReq)
	if completeW.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload failed: status %d, body %s", completeW.Code, completeW.Body.String())
	}
	var done struct {
		ETag string `xml:"ETag"`
	}
	if err := xml.NewDecoder(completeW.Body).Decode(&done); err != nil {
		t.Fatalf("parse CompleteMultipartUpload result: %v", err)
	}
	if done.ETag == "" {
		t.Fatal("completion returned an empty ETag")
	}
	return done.ETag
}

// wantMultipartHeadHeaders asserts the HEAD response a manifest-backed object
// must produce: the completion ETag (never the zero value), the recorded
// content type, and the plaintext size.
func wantMultipartHeadHeaders(t *testing.T, w *httptest.ResponseRecorder, completionETag string) {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("ETag"); got != fmt.Sprintf(`"%s"`, completionETag) {
		t.Errorf("ETag: got %s, want %q", got, completionETag)
	}
	if got := w.Header().Get("Content-Type"); got != headFallbackCType {
		t.Errorf("Content-Type: got %q, want %q", got, headFallbackCType)
	}
	if got := w.Header().Get("Content-Length"); got != fmt.Sprint(2*headFallbackPart) {
		t.Errorf("Content-Length: got %s, want %d", got, int64(2*headFallbackPart))
	}
}

// TestHeadObjectFallbackServesManifestAfterIndexLoss verifies that HEAD keeps
// serving the completion metadata when the in-memory index lost the entry
// (restart before deltas persisted, degraded load): the fallback must read the
// manifest object instead of heading the bare ciphertext.
func TestHeadObjectFallbackServesManifestAfterIndexLoss(t *testing.T) {
	cfg, _, cache, footerCache, km := testSetup(t)
	ab := newAssemblingMultipartBackend()
	hcb := &countingAssemblingBackend{assemblingMultipartBackend: ab}
	rec := newMockManifestRecorder()
	h := handlers.New(cfg, hcb, cache, footerCache, km, nil)
	h.WithManifest(rec)

	completionETag := completeMultipartThroughHandlers(t, h)

	// Drop the index entry so HEAD takes the fallback path.
	rec.RecordDelete(headFallbackBucket, headFallbackKey)
	hcb.headCalls.Store(0)

	req := httptest.NewRequest(http.MethodHead, "/"+headFallbackBucket+"/"+headFallbackKey, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	wantMultipartHeadHeaders(t, w, completionETag)
	// The answer came from the fallback's ciphertext head, not the index.
	if n := hcb.headCalls.Load(); n == 0 {
		t.Errorf("expected the fallback path to head the ciphertext object, got %d head calls", n)
	}
}

// TestHeadObjectManifestDisabledServesManifestMetadata verifies HEAD on a
// server with the manifest index disabled (ARMOR_MANIFEST_ENABLED=false — the
// compatibility-harness shape): every multipart HEAD takes the fallback, and
// it must still serve the manifest's completion metadata.
func TestHeadObjectManifestDisabledServesManifestMetadata(t *testing.T) {
	cfg, _, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, newAssemblingMultipartBackend(), cache, footerCache, km, nil) // no WithManifest

	completionETag := completeMultipartThroughHandlers(t, h)

	req := httptest.NewRequest(http.MethodHead, "/"+headFallbackBucket+"/"+headFallbackKey, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	wantMultipartHeadHeaders(t, w, completionETag)
}

// TestHeadObjectFallbackConditionalRequests verifies conditional requests on
// the manifest fallback: aws-cli's multipart download HEADs the object and
// replays the served ETag as If-Match on every ranged GET, so HEAD and GET
// must agree on the ETag — a HEAD serving "" turned into a 412 on every part.
func TestHeadObjectFallbackConditionalRequests(t *testing.T) {
	cfg, _, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, newAssemblingMultipartBackend(), cache, footerCache, km, nil) // no WithManifest

	completionETag := completeMultipartThroughHandlers(t, h)

	head := func(ifMatch string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodHead, "/"+headFallbackBucket+"/"+headFallbackKey, nil)
		if ifMatch != "" {
			req.Header.Set("If-Match", ifMatch)
		}
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		return w
	}

	// A stale If-Match — what a client that trusted the old empty-ETag HEAD
	// would send — must fail with 412, and the real ETag must pass.
	if w := head(`"stale-etag-from-broken-head"`); w.Code != http.StatusPreconditionFailed {
		t.Errorf("If-Match with wrong etag: got %d, want 412", w.Code)
	}
	wantMultipartHeadHeaders(t, head(fmt.Sprintf(`"%s"`, completionETag)), completionETag)

	// If-None-Match matching the served ETag is a 304 carrying the ETag.
	req := httptest.NewRequest(http.MethodHead, "/"+headFallbackBucket+"/"+headFallbackKey, nil)
	req.Header.Set("If-None-Match", fmt.Sprintf(`"%s"`, completionETag))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match with matching etag: got %d, want 304", w.Code)
	}
	if got := w.Header().Get("ETag"); got != fmt.Sprintf(`"%s"`, completionETag) {
		t.Errorf("304 ETag: got %s, want %q", got, completionETag)
	}

	// GET parity: the same If-Match the HEAD endorsed must succeed on the
	// read path (a ranged GET, the shape s3transfer sends).
	getReq := httptest.NewRequest(http.MethodGet, "/"+headFallbackBucket+"/"+headFallbackKey, nil)
	getReq.Header.Set("If-Match", fmt.Sprintf(`"%s"`, completionETag))
	getReq.Header.Set("Range", "bytes=0-1023")
	getW := httptest.NewRecorder()
	h.HandleRoot(getW, getReq)
	if getW.Code != http.StatusPartialContent {
		t.Fatalf("ranged GET with If-Match endorsed by HEAD: got %d, want 206, body %s", getW.Code, getW.Body.String())
	}
	getBad := httptest.NewRecorder()
	getReqBad := httptest.NewRequest(http.MethodGet, "/"+headFallbackBucket+"/"+headFallbackKey, nil)
	getReqBad.Header.Set("If-Match", `"stale-etag-from-broken-head"`)
	h.HandleRoot(getBad, getReqBad)
	if getBad.Code != http.StatusPreconditionFailed {
		t.Errorf("ranged GET with wrong If-Match: got %d, want 412", getBad.Code)
	}
}

// TestHeadObjectManifestDisabledSinglePUTObject verifies the legacy fallback
// still serves single-PUT objects when the manifest index is disabled — the
// added manifest probe must not disturb the no-manifest path.
func TestHeadObjectManifestDisabledSinglePUTObject(t *testing.T) {
	cfg, _, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, newAssemblingMultipartBackend(), cache, footerCache, km, nil) // no WithManifest

	putReq := httptest.NewRequest(http.MethodPut, "/"+headFallbackBucket+"/single-put.bin", bytes.NewReader(bytes.Repeat([]byte("x"), 4096)))
	putReq.Header.Set("Content-Type", "text/plain")
	putW := httptest.NewRecorder()
	h.HandleRoot(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body %s", putW.Code, putW.Body.String())
	}
	putETag := putW.Header().Get("ETag")
	if putETag == "" {
		t.Fatal("PUT response carried no ETag")
	}

	req := httptest.NewRequest(http.MethodHead, "/"+headFallbackBucket+"/single-put.bin", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d, body %s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("ETag"); got != putETag {
		t.Errorf("ETag: got %s, want PUT response ETag %s", got, putETag)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain" {
		t.Errorf("Content-Type: got %q, want %q", got, "text/plain")
	}
}
