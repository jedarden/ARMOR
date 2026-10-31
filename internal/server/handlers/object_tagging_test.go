package handlers_test

import (
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/server/handlers"
)

// newTaggingTestHandlers builds a handler set on the shared mock backend for
// the object-tagging tests.
func newTaggingTestHandlers(t *testing.T) (*handlers.Handlers, *mockBackend) {
	t.Helper()
	mek := make([]byte, 32)
	if _, err := rand.Read(mek); err != nil {
		t.Fatalf("failed to generate MEK: %v", err)
	}
	cfg := &config.Config{
		BlockSize:     65536,
		AuthAccessKey: "test-access-key",
		AuthSecretKey: "REMOVED-NOT-A-SECRET-VALUE",
	}
	mb := newMockBackend()
	cache := backend.NewMetadataCache(1000, 300)
	footerCache := backend.NewFooterCache(1000, 300)
	km, err := keymanager.New(mek, nil, nil)
	if err != nil {
		t.Fatalf("failed to create key manager: %v", err)
	}
	return handlers.New(cfg, mb, cache, footerCache, km, nil), mb
}

// putTestObject stores body at bucket/key through the real PutObject path.
func putTestObject(t *testing.T, h *handlers.Handlers, bucket, key, body string, hdr map[string]string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PutObject %s/%s: status %d, body %s", bucket, key, w.Code, w.Body.String())
	}
}

// getTagging drives GET ?tagging and decodes the response.
func getTagging(t *testing.T, h *handlers.Handlers, bucket, key string) (int, []tagPair) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key+"?tagging", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		return w.Code, nil
	}
	var resp struct {
		XMLName xml.Name  `xml:"Tagging"`
		TagSet  []tagPair `xml:"TagSet>Tag"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("GetObjectTagging body is not valid Tagging XML: %v\nbody: %s", err, w.Body.String())
	}
	return w.Code, resp.TagSet
}

type tagPair struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

func assertTagSet(t *testing.T, got []tagPair, want []tagPair) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("tag set = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tag %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestObjectTagging_HeaderRoundTrip pins the x-amz-tagging header on
// PutObject: the tag set survives and GetObjectTagging returns it as valid
// Tagging XML in the order the client sent.
func TestObjectTagging_HeaderRoundTrip(t *testing.T) {
	h, _ := newTaggingTestHandlers(t)
	putTestObject(t, h, "bkt", "tagged.txt", "payload", map[string]string{
		"x-amz-tagging": "Environment=Test&Project=ARMOR&Owner=Test%20Suite",
	})

	code, tags := getTagging(t, h, "bkt", "tagged.txt")
	if code != http.StatusOK {
		t.Fatalf("GetObjectTagging: status %d", code)
	}
	assertTagSet(t, tags, []tagPair{
		{Key: "Environment", Value: "Test"},
		{Key: "Project", Value: "ARMOR"},
		{Key: "Owner", Value: "Test Suite"},
	})

	// An untagged object reports an empty set, not an error.
	putTestObject(t, h, "bkt", "untagged.txt", "payload", nil)
	code, tags = getTagging(t, h, "bkt", "untagged.txt")
	if code != http.StatusOK {
		t.Fatalf("GetObjectTagging on untagged object: status %d", code)
	}
	assertTagSet(t, tags, nil)
}

// TestObjectTagging_SubresourceRoundTrip pins the ?tagging subresource
// APIs on an existing object, and that they never touch the object payload:
// before the dedicated routes existed, PUT ?tagging stored the XML body as
// object data and DELETE ?tagging deleted the object outright.
func TestObjectTagging_SubresourceRoundTrip(t *testing.T) {
	h, _ := newTaggingTestHandlers(t)
	putTestObject(t, h, "bkt", "obj.txt", "original-body", nil)

	// PUT ?tagging replaces the whole set.
	body := `<?xml version="1.0" encoding="UTF-8"?>
<Tagging xmlns="http://s3.amazonaws.com/doc/2006-03-01/"><TagSet>
<Tag><Key>Stage</Key><Value>prod</Value></Tag>
<Tag><Key>Owner</Key><Value>platform</Value></Tag>
</TagSet></Tagging>`
	req := httptest.NewRequest(http.MethodPut, "/bkt/obj.txt?tagging", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PutObjectTagging: status %d, body %s", w.Code, w.Body.String())
	}
	_, tags := getTagging(t, h, "bkt", "obj.txt")
	assertTagSet(t, tags, []tagPair{{Key: "Stage", Value: "prod"}, {Key: "Owner", Value: "platform"}})

	// The object's bytes and readability are untouched by the tagging writes.
	req = httptest.NewRequest(http.MethodGet, "/bkt/obj.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetObject after tagging writes: status %d", w.Code)
	}
	got, err := io.ReadAll(w.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(got) != "original-body" {
		t.Fatalf("object body mutated by tagging writes: %q", got)
	}

	// DELETE ?tagging clears the set and keeps the object.
	req = httptest.NewRequest(http.MethodDelete, "/bkt/obj.txt?tagging", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("DeleteObjectTagging: status %d", w.Code)
	}
	_, tags = getTagging(t, h, "bkt", "obj.txt")
	assertTagSet(t, tags, nil)

	req = httptest.NewRequest(http.MethodGet, "/bkt/obj.txt", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetObject after DeleteObjectTagging: status %d — the tagging DELETE must not delete the object", w.Code)
	}
}

// TestObjectTagging_MissingObject pins the 404 contract for every tagging
// operation against a key that does not exist.
func TestObjectTagging_MissingObject(t *testing.T) {
	h, _ := newTaggingTestHandlers(t)

	for _, tc := range []struct {
		name   string
		method string
	}{
		{"get", http.MethodGet},
		{"put", http.MethodPut},
		{"delete", http.MethodDelete},
	} {
		var body io.Reader
		if tc.method == http.MethodPut {
			body = strings.NewReader(`<Tagging><TagSet><Tag><Key>k</Key><Value>v</Value></Tag></TagSet></Tagging>`)
		}
		req := httptest.NewRequest(tc.method, "/bkt/absent.txt?tagging", body)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s ?tagging on missing object: status %d, want 404", tc.method, w.Code)
		}
		if !strings.Contains(w.Body.String(), "NoSuchKey") {
			t.Errorf("%s ?tagging on missing object: error code not NoSuchKey: %s", tc.method, w.Body.String())
		}
	}
}

// TestObjectTagging_RejectsInvalidTagSets pins the validation shared by the
// header and XML paths: query-string form, tag count, key/value length, and
// duplicate keys.
func TestObjectTagging_RejectsInvalidTagSets(t *testing.T) {
	h, _ := newTaggingTestHandlers(t)
	putTestObject(t, h, "bkt", "obj.txt", "payload", nil)

	tooMany := make([]string, 0, 11)
	for i := 0; i < 11; i++ {
		tooMany = append(tooMany, fmt.Sprintf("k%d=v", i))
	}

	for i, tc := range []struct {
		name   string
		header string
	}{
		{"not key=value pairs", "justakey"},
		{"bad percent-encoding", "k=%zz"},
		{"empty key", "=v"},
		{"duplicate keys", "a=1&a=2"},
		{"too many tags", strings.Join(tooMany, "&")},
		{"key too long", strings.Repeat("k", 129) + "=v"},
		{"value too long", "k=" + strings.Repeat("v", 257)},
	} {
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/bkt/invalid-%d.txt", i), strings.NewReader("x"))
		req.Header.Set("x-amz-tagging", tc.header)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		if w.Code != http.StatusBadRequest {
			t.Errorf("%s: PutObject status %d, want 400 (body %s)", tc.name, w.Code, w.Body.String())
		}
	}

	// The PUT ?tagging body must be Tagging XML.
	req := httptest.NewRequest(http.MethodPut, "/bkt/obj.txt?tagging", strings.NewReader("this is not xml"))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "MalformedXML") {
		t.Errorf("malformed tagging XML: status %d body %s, want 400 MalformedXML", w.Code, w.Body.String())
	}

	// A structurally valid body still runs the tag-set validation.
	dupBody := `<Tagging><TagSet><Tag><Key>a</Key><Value>1</Value></Tag><Tag><Key>a</Key><Value>2</Value></Tag></TagSet></Tagging>`
	req = httptest.NewRequest(http.MethodPut, "/bkt/obj.txt?tagging", strings.NewReader(dupBody))
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("duplicate tag keys via XML: status %d, want 400", w.Code)
	}
}

// TestCopyObject_TaggingDirective pins the copy semantics: the default COPY
// carries the source's tag set to the destination (which the metadata
// rebuild in the ARMOR-encrypted copy path used to drop), REPLACE applies
// the request's x-amz-tagging set, and REPLACE without the header clears it.
func TestCopyObject_TaggingDirective(t *testing.T) {
	h, _ := newTaggingTestHandlers(t)
	putTestObject(t, h, "bkt", "src.txt", "payload", map[string]string{
		"x-amz-tagging": "Environment=Production&Project=ARMOR",
	})

	doCopy := func(dst string, hdr map[string]string) int {
		req := httptest.NewRequest(http.MethodPut, "/bkt/"+dst, nil)
		req.Header.Set("x-amz-copy-source", "/bkt/src.txt")
		for k, v := range hdr {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		return w.Code
	}

	// Default directive (COPY): the destination inherits the source tags.
	if code := doCopy("copy-default.txt", nil); code != http.StatusOK {
		t.Fatalf("CopyObject (default COPY): status %d", code)
	}
	_, tags := getTagging(t, h, "bkt", "copy-default.txt")
	assertTagSet(t, tags, []tagPair{
		{Key: "Environment", Value: "Production"},
		{Key: "Project", Value: "ARMOR"},
	})

	// Explicit COPY with a stray x-amz-tagging header: the header is ignored.
	if code := doCopy("copy-explicit.txt", map[string]string{
		"x-amz-tagging":           "Ignored=yes",
		"x-amz-tagging-directive": "COPY",
	}); code != http.StatusOK {
		t.Fatalf("CopyObject (COPY): status %d", code)
	}
	_, tags = getTagging(t, h, "bkt", "copy-explicit.txt")
	assertTagSet(t, tags, []tagPair{
		{Key: "Environment", Value: "Production"},
		{Key: "Project", Value: "ARMOR"},
	})

	// REPLACE applies the request's tag set.
	if code := doCopy("copy-replace.txt", map[string]string{
		"x-amz-tagging":           "Environment=Test&Project=Copied",
		"x-amz-tagging-directive": "REPLACE",
	}); code != http.StatusOK {
		t.Fatalf("CopyObject (REPLACE): status %d", code)
	}
	_, tags = getTagging(t, h, "bkt", "copy-replace.txt")
	assertTagSet(t, tags, []tagPair{
		{Key: "Environment", Value: "Test"},
		{Key: "Project", Value: "Copied"},
	})

	// REPLACE without a tag header writes an untagged destination.
	putTestObject(t, h, "bkt", "other-src.txt", "payload", map[string]string{
		"x-amz-tagging": "A=1",
	})
	req := httptest.NewRequest(http.MethodPut, "/bkt/copy-cleared.txt", nil)
	req.Header.Set("x-amz-copy-source", "/bkt/other-src.txt")
	req.Header.Set("x-amz-tagging-directive", "REPLACE")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("CopyObject (REPLACE, no header): status %d", w.Code)
	}
	_, tags = getTagging(t, h, "bkt", "copy-cleared.txt")
	assertTagSet(t, tags, nil)

	// An unknown directive value is rejected before any backend work.
	req = httptest.NewRequest(http.MethodPut, "/bkt/copy-bad.txt", nil)
	req.Header.Set("x-amz-copy-source", "/bkt/src.txt")
	req.Header.Set("x-amz-tagging-directive", "MERGE")
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("CopyObject (bad directive): status %d, want 400", w.Code)
	}
}
