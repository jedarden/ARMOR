package handlers_test

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	handlers "github.com/jedarden/armor/internal/server/handlers"
)

// The tests in this file pin the client-visible S3 contract of a completed
// multipart upload under ARMOR_PREFIX. They are the handler-level twins of
// the armor-integration failures tracked in armor-9a58cd90: each of those
// reached main once without a test below the real-B2 gate, so a regression
// could only be seen on the nightly.

const contractPrefix = "integration-tests/"

// contractSetup builds handlers over a real filesystem backend (its Head
// reports plaintext Size and physical StoredSize separately, the distinction
// the manifest passthrough depends on) with v3 writes and a key prefix.
func contractSetup(t *testing.T) (*handlers.Handlers, *backend.FSBackend, string) {
	t.Helper()
	cfg, _, cache, footerCache, km := testSetupWithPrefix(t, contractPrefix)
	cfg.FormatWriteVersion = 3

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	bucket := "test-bucket"
	if err := fs.CreateBucket(t.Context(), bucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	return handlers.New(cfg, fs, cache, footerCache, km, nil), fs, bucket
}

func contractCreateUpload(t *testing.T, h *handlers.Handlers, bucket, key string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/"+bucket+"/"+key+"?uploads=", nil)
	req.Header.Set("x-amz-meta-test-name", "contract")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CreateMultipartUpload: status %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &created); err != nil || created.UploadID == "" {
		t.Fatalf("CreateMultipartUpload response: %v: %s", err, w.Body.String())
	}
	return created.UploadID
}

func contractComplete(t *testing.T, h *handlers.Handlers, bucket, key, uploadID string, etags []string) string {
	t.Helper()
	var body bytes.Buffer
	body.WriteString("<CompleteMultipartUpload>")
	for i, etag := range etags {
		fmt.Fprintf(&body, "<Part><PartNumber>%d</PartNumber><ETag>%s</ETag></Part>", i+1, etag)
	}
	body.WriteString("</CompleteMultipartUpload>")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%s?uploadId=%s", bucket, key, uploadID), &body)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CompleteMultipartUpload: status %d: %s", w.Code, w.Body.String())
	}
	var completed struct {
		ETag string `xml:"ETag"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &completed); err != nil || completed.ETag == "" {
		t.Fatalf("CompleteMultipartUpload response: %v: %s", err, w.Body.String())
	}
	return strings.Trim(completed.ETag, `"`)
}

func contractDo(h *handlers.Handlers, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.HandleRoot(w, httptest.NewRequest(method, path, nil))
	return w
}

func contractRandom(t *testing.T, n int) []byte {
	t.Helper()
	data := make([]byte, n)
	if _, err := rand.Read(data); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	return data
}

// compositeETag is the S3 multipart convention: md5(md5(p1)||…||md5(pN))-N
// over the plaintext parts the client uploaded.
func compositeETag(parts ...[]byte) string {
	outer := md5.New()
	for _, part := range parts {
		digest := md5.Sum(part)
		outer.Write(digest[:])
	}
	return fmt.Sprintf("%x-%d", outer.Sum(nil), len(parts))
}

func armorHeaders(header http.Header) []string {
	var found []string
	for name := range header {
		if strings.HasPrefix(strings.ToLower(name), "x-amz-meta-armor-") {
			found = append(found, strings.ToLower(name))
		}
	}
	return found
}

var contractCryptoKeys = []string{
	"x-amz-meta-armor-iv",
	"x-amz-meta-armor-wrapped-dek",
	"x-amz-meta-armor-key-id",
}

// TestCompletedMultipartContractUnderPrefix walks one two-part v3 upload and
// checks everything a client can observe about the result.
func TestCompletedMultipartContractUnderPrefix(t *testing.T) {
	h, fs, bucket := contractSetup(t)
	key := "contract/object.bin"
	objectPath := "/" + bucket + "/" + key
	manifestPath := objectPath + ".armor-manifest"

	part1 := contractRandom(t, 5*1024*1024)
	part2 := contractRandom(t, 1024)
	plaintext := append(append([]byte{}, part1...), part2...)
	wantETag := compositeETag(part1, part2)

	uploadID := contractCreateUpload(t, h, bucket, key)
	etags := []string{
		uploadPart(t, h, bucket, key, uploadID, 1, part1),
		uploadPart(t, h, bucket, key, uploadID, 2, part2),
	}
	completeETag := contractComplete(t, h, bucket, key, uploadID, etags)

	t.Run("ETag is the composite of the plaintext part MD5s", func(t *testing.T) {
		// A composite over the stored ciphertext parts (what the backend
		// returns) verifies nothing for the client, and a whole-object MD5
		// with a -N suffix is not what any S3 server produces.
		if completeETag != wantETag {
			t.Errorf("CompleteMultipartUpload ETag = %s, want %s", completeETag, wantETag)
		}
		for _, method := range []string{http.MethodHead, http.MethodGet} {
			w := contractDo(h, method, objectPath)
			if w.Code != http.StatusOK {
				t.Fatalf("%s object: status %d: %s", method, w.Code, w.Body.String())
			}
			if got := w.Header().Get("ETag"); got != `"`+wantETag+`"` {
				t.Errorf("%s object ETag = %s, want quoted %s", method, got, wantETag)
			}
		}
	})

	t.Run("object reads carry client metadata and no ARMOR metadata", func(t *testing.T) {
		for _, method := range []string{http.MethodHead, http.MethodGet} {
			w := contractDo(h, method, objectPath)
			if w.Code != http.StatusOK {
				t.Fatalf("%s object: status %d: %s", method, w.Code, w.Body.String())
			}
			if got := w.Header().Get("x-amz-meta-test-name"); got != "contract" {
				t.Errorf("%s object x-amz-meta-test-name = %q, want the client's value", method, got)
			}
			if leaked := armorHeaders(w.Header()); len(leaked) != 0 {
				t.Errorf("%s object exposes ARMOR metadata %v; a multipart object must read like a single-PUT one", method, leaked)
			}
			if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(plaintext)) {
				t.Errorf("%s object Content-Length = %s, want plaintext size %d", method, got, len(plaintext))
			}
			if method == http.MethodGet && !bytes.Equal(w.Body.Bytes(), plaintext) {
				t.Errorf("GET object returned %d bytes that differ from the %d uploaded", w.Body.Len(), len(plaintext))
			}
		}
	})

	t.Run("manifest HEAD and GET agree and describe the object", func(t *testing.T) {
		head := contractDo(h, http.MethodHead, manifestPath)
		get := contractDo(h, http.MethodGet, manifestPath)
		if head.Code != http.StatusOK || get.Code != http.StatusOK {
			// GET used to 500: the decrypt path ran on plaintext JSON.
			t.Fatalf("manifest HEAD %d / GET %d, want 200 / 200: %s", head.Code, get.Code, get.Body.String())
		}

		// Content-Length must be the bytes served. It used to be the
		// described object's plaintext size, so every client read failed
		// with a short body.
		if got := get.Header().Get("Content-Length"); got != strconv.Itoa(get.Body.Len()) {
			t.Errorf("manifest GET Content-Length = %s, body is %d bytes", got, get.Body.Len())
		}
		servedMD5 := md5.Sum(get.Body.Bytes())
		wantManifestETag := `"` + hex.EncodeToString(servedMD5[:]) + `"`
		for name, w := range map[string]*httptest.ResponseRecorder{"HEAD": head, "GET": get} {
			if got := w.Header().Get("ETag"); got != wantManifestETag {
				t.Errorf("manifest %s ETag = %s, want the MD5 of the served body %s", name, got, wantManifestETag)
			}
			if got := w.Header().Get("Content-Length"); got != strconv.Itoa(get.Body.Len()) {
				t.Errorf("manifest %s Content-Length = %s, want %d", name, got, get.Body.Len())
			}
			if got := w.Header().Get("Content-Type"); got != "application/x-armor-manifest+json" {
				t.Errorf("manifest %s Content-Type = %q", name, got)
			}
			for header, want := range map[string]string{
				"x-amz-meta-armor-version":   "3",
				"x-amz-meta-armor-multipart": "true",
			} {
				if got := w.Header().Get(header); got != want {
					t.Errorf("manifest %s %s = %q, want %q", name, header, got, want)
				}
			}
			for _, header := range []string{"x-amz-meta-armor-ciphertext-ref", "x-amz-meta-armor-completed-at"} {
				if w.Header().Get(header) == "" {
					t.Errorf("manifest %s is missing %s", name, header)
				}
			}
			for _, header := range contractCryptoKeys {
				if w.Header().Get(header) != "" {
					t.Errorf("manifest %s exposes %s", name, header)
				}
			}
		}

		var served backend.ManifestBody
		if err := json.Unmarshal(get.Body.Bytes(), &served); err != nil {
			t.Fatalf("manifest GET body is not manifest JSON: %v: %s", err, get.Body.String())
		}
		if served.CiphertextObject != contractPrefix+key {
			t.Errorf("manifest ciphertext_object = %q, want the stored key %q", served.CiphertextObject, contractPrefix+key)
		}
		if served.UploadID != uploadID || served.CompletedAt == "" {
			t.Errorf("manifest upload_id = %q (want %q), completed_at = %q", served.UploadID, uploadID, served.CompletedAt)
		}
		if served.Metadata["x-amz-meta-armor-version"] != "3" {
			t.Errorf("manifest body metadata lacks armor-version: %v", served.Metadata)
		}
		for _, k := range contractCryptoKeys {
			if _, leaked := served.Metadata[k]; leaked {
				t.Errorf("manifest GET body exposes %s", k)
			}
		}
		if bytes.Contains(get.Body.Bytes(), []byte("wrapped-dek")) {
			t.Error("manifest GET body mentions the wrapped DEK")
		}
	})

	t.Run("stored manifest body keeps the key material for recovery", func(t *testing.T) {
		// The restore-verifier and `armor verify-multipart` fall back to the
		// body's metadata when the manifest object's headers are gone. The
		// redaction is on the S3 surface only.
		body, _, err := fs.Get(t.Context(), bucket, contractPrefix+key+".armor-manifest")
		if err != nil {
			t.Fatalf("backend Get manifest: %v", err)
		}
		defer body.Close()
		var stored backend.ManifestBody
		if err := json.NewDecoder(body).Decode(&stored); err != nil {
			t.Fatalf("stored manifest body: %v", err)
		}
		if _, ok := backend.ParseARMORMetadata(stored.Metadata); !ok {
			t.Errorf("stored manifest body no longer parses as ARMOR metadata; keys: %d", len(stored.Metadata))
		}
	})

	t.Run("a non-manifest object at a manifest key is refused, not leaked", func(t *testing.T) {
		raw := []byte("not json at all")
		if err := fs.Put(t.Context(), bucket, contractPrefix+"stray.armor-manifest", bytes.NewReader(raw), int64(len(raw)), map[string]string{"Content-Type": "text/plain"}); err != nil {
			t.Fatalf("backend Put: %v", err)
		}
		w := contractDo(h, http.MethodGet, "/"+bucket+"/stray.armor-manifest")
		if w.Code != http.StatusInternalServerError || bytes.Contains(w.Body.Bytes(), raw) {
			t.Errorf("GET of an unparseable manifest key: status %d, body %q; want 500 without the stored bytes", w.Code, w.Body.String())
		}
	})

	t.Run("a plain PUT over the multipart object is served, not rejected", func(t *testing.T) {
		// The stale manifest still sits beside the new single-PUT object;
		// reads used to 500 (SidecarCacheInvalidation in armor-integration).
		replacement := contractRandom(t, 4096)
		put := httptest.NewRecorder()
		h.HandleRoot(put, httptest.NewRequest(http.MethodPut, objectPath, bytes.NewReader(replacement)))
		if put.Code != http.StatusOK {
			t.Fatalf("overwriting PUT: status %d: %s", put.Code, put.Body.String())
		}
		sum := md5.Sum(replacement)
		wantPutETag := `"` + hex.EncodeToString(sum[:]) + `"`
		for _, method := range []string{http.MethodHead, http.MethodGet} {
			w := contractDo(h, method, objectPath)
			if w.Code != http.StatusOK {
				t.Fatalf("%s after overwrite: status %d: %s", method, w.Code, w.Body.String())
			}
			if got := w.Header().Get("ETag"); got != wantPutETag {
				t.Errorf("%s after overwrite ETag = %s, want %s", method, got, wantPutETag)
			}
			if method == http.MethodGet && !bytes.Equal(w.Body.Bytes(), replacement) {
				t.Error("GET after overwrite did not return the replacement bytes")
			}
		}
	})
}
