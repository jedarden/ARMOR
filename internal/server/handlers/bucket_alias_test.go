// Package handlers_test tests S3 operation handlers.
package handlers_test

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/server/handlers"
)

// aliasTestConfig returns a config consolidated onto the unified bucket with
// legacy-bucket still accepted as an alias of it. This is the ADR-001 cutover
// state: the objects already live under the unified bucket and the clients
// have not been repointed yet.
func aliasTestConfig(t *testing.T) (*config.Config, *mockBackend, *handlers.Handlers) {
	t.Helper()

	cfg, mb, cache, footerCache, km := testSetup(t)
	cfg.Bucket = "unified-bucket"
	cfg.BucketAliases = []string{"legacy-bucket"}
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)
	return cfg, mb, h
}

// putObject stores body at the given URL and fails the test on a non-200.
func putObject(t *testing.T, h *handlers.Handlers, url string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, url, bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT %s: status %d, want 200: %s", url, w.Code, w.Body.String())
	}
	return w
}

// TestHandleRootBucketAliasServesFromConfiguredBucket is the table-driven check
// over both addressing styles and all three bucket-name classes. The assertion
// in every case is which backend bucket the request actually landed in: an
// alias must reach config.Bucket, the configured name must be unchanged, and an
// unknown name must be left alone rather than silently remapped.
func TestHandleRootBucketAliasServesFromConfiguredBucket(t *testing.T) {
	tests := []struct {
		name string
		url  string
		// wantBackendKey is the key the object must be stored under in the
		// backend, bucket included. An alias and the configured name both land
		// in config.Bucket; an unknown name, and the first path segment of a
		// virtual-host-style URL (whose bucket is in the Host and is therefore
		// not a bucket ARMOR recognises), land under the name they carry.
		wantBackendKey string
	}{
		{
			name:           "path-style alias",
			url:            "/legacy-bucket/data/obj.bin",
			wantBackendKey: "unified-bucket/data/obj.bin",
		},
		{
			name:           "path-style configured bucket",
			url:            "/unified-bucket/data/obj.bin",
			wantBackendKey: "unified-bucket/data/obj.bin",
		},
		{
			name:           "path-style unknown bucket",
			url:            "/unknown-bucket/data/obj.bin",
			wantBackendKey: "unknown-bucket/data/obj.bin",
		},
		{
			// Virtual-host-style puts the bucket in the host, leaving "/data" as
			// the first path segment. ARMOR only recognises path-style addressing
			// and deliberately ignores the Host header, so the bucket here is
			// "data" and the host — whatever it says — is not consulted. A host
			// naming an alias therefore cannot reach the alias either, which is
			// the point of these three rows.
			name:           "virtual-host-style alias host",
			url:            "http://legacy-bucket.s3.example.com/data/obj.bin",
			wantBackendKey: "data/obj.bin",
		},
		{
			name:           "virtual-host-style configured host",
			url:            "http://unified-bucket.s3.example.com/data/obj.bin",
			wantBackendKey: "data/obj.bin",
		},
		{
			name:           "virtual-host-style unknown host",
			url:            "http://unknown-bucket.s3.example.com/data/obj.bin",
			wantBackendKey: "data/obj.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, mb, h := aliasTestConfig(t)
			body := []byte("payload for " + tt.name)
			putObject(t, h, tt.url, body)

			if _, ok := mb.objects[tt.wantBackendKey]; !ok {
				t.Errorf("object stored outside %q; backend holds:", tt.wantBackendKey)
				for k := range mb.objects {
					t.Errorf("  %q", k)
				}
			}

			// Reading through the unified bucket returns the stored body exactly
			// when the write addressed the unified bucket.
			wantReadOK := tt.wantBackendKey == "unified-bucket/data/obj.bin"
			req := httptest.NewRequest(http.MethodGet, "/unified-bucket/data/obj.bin", nil)
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if wantReadOK {
				if w.Code != http.StatusOK {
					t.Fatalf("GET via configured bucket: status %d, want 200: %s", w.Code, w.Body.String())
				}
				if !bytes.Equal(w.Body.Bytes(), body) {
					t.Errorf("GET via configured bucket: body %q, want %q", w.Body.String(), string(body))
				}
				return
			}
			if w.Code == http.StatusOK {
				t.Errorf("GET via configured bucket: status %d, want not found — an unknown bucket must not reach the unified bucket", w.Code)
			}
		})
	}
}

// TestHandleRootUnknownBucketIsNotAliasMapped pins the failure mode for a
// bucket ARMOR knows nothing about: it must still be a distinct bucket, so a
// client pointing at a typo or a decommissioned name sees an error rather than
// another tenant's objects.
func TestHandleRootUnknownBucketIsNotAliasMapped(t *testing.T) {
	_, _, h := aliasTestConfig(t)

	putObject(t, h, "/unified-bucket/data/obj.bin", []byte("unified payload"))

	// A HEAD against the unknown bucket must not be satisfied by the unified
	// bucket's contents.
	req := httptest.NewRequest(http.MethodHead, "/unknown-bucket/data/obj.bin", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("HEAD unknown-bucket/data/obj.bin: status %d, want not found", w.Code)
	}

	// And the unknown bucket must not be listed as if it were the unified one.
	// The prefix scopes the assertion to the key the object was written under,
	// which is what makes a non-empty result a genuine leak rather than noise.
	req = httptest.NewRequest(http.MethodGet, "/unknown-bucket?list-type=2&prefix=data/", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list unknown-bucket: status %d, want 200: %s", w.Code, w.Body.String())
	}
	var listing struct {
		Name     string `xml:"Name"`
		Contents []struct {
			Key string `xml:"Key"`
		} `xml:"Contents"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatalf("decode list response: %v\n%s", err, w.Body.String())
	}
	if listing.Name != "unknown-bucket" {
		t.Errorf("list Name = %q, want the name the client asked for", listing.Name)
	}
	if len(listing.Contents) != 0 {
		t.Errorf("list of unknown-bucket returned %d objects, want 0", len(listing.Contents))
	}

	// Control for the assertion above: the same listing against the configured
	// bucket does find the object, so the empty result is isolation and not a
	// broken listing.
	req = httptest.NewRequest(http.MethodGet, "/unified-bucket?list-type=2&prefix=data/", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("list unified-bucket: status %d, want 200: %s", w.Code, w.Body.String())
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &listing); err != nil {
		t.Fatalf("decode unified-bucket list response: %v\n%s", err, w.Body.String())
	}
	if len(listing.Contents) != 1 {
		t.Errorf("list of unified-bucket returned %d objects, want 1", len(listing.Contents))
	}
}

// TestListResponsesEchoNamedBucket checks that the bucket echoed back in a
// response body is the one the client addressed, not the one the objects live
// in. S3 clients compare it against their request and discard a response that
// names a different bucket, so echoing the alias would make a correct listing
// look like a failure to a client that has not been repointed yet.
func TestListResponsesEchoNamedBucket(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "list via alias",
			url:  "/legacy-bucket",
			want: "legacy-bucket",
		},
		{
			name: "list via configured bucket",
			url:  "/unified-bucket",
			want: "unified-bucket",
		},
		{
			name: "list versions via alias",
			url:  "/legacy-bucket?versions",
			want: "legacy-bucket",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, h := aliasTestConfig(t)
			putObject(t, h, "/legacy-bucket/data/obj.bin", []byte("payload"))

			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("GET %s: status %d, want 200: %s", tt.url, w.Code, w.Body.String())
			}

			var listing struct {
				Name string `xml:"Name"`
			}
			if err := xml.Unmarshal(w.Body.Bytes(), &listing); err != nil {
				t.Fatalf("decode response: %v\n%s", err, w.Body.String())
			}
			if listing.Name != tt.want {
				t.Errorf("GET %s echoed Name %q, want %q", tt.url, listing.Name, tt.want)
			}
		})
	}
}

// TestListBucketsReturnsConfiguredBucket checks that aliasing does not make the
// legacy names appear as buckets of their own: the consolidation happened, so
// only the configured bucket exists.
func TestListBucketsReturnsConfiguredBucket(t *testing.T) {
	_, _, h := aliasTestConfig(t)

	putObject(t, h, "/legacy-bucket/data/obj.bin", []byte("payload"))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ListBuckets: status %d, want 200: %s", w.Code, w.Body.String())
	}

	var result struct {
		Buckets []struct {
			Name string `xml:"Name"`
		} `xml:"Buckets>Bucket"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode ListBuckets response: %v\n%s", err, w.Body.String())
	}

	var names []string
	for _, b := range result.Buckets {
		names = append(names, b.Name)
		if b.Name == "legacy-bucket" {
			t.Errorf("ListBuckets returned the alias %q; aliases are not buckets", b.Name)
		}
	}
	if len(names) != 1 || names[0] != "unified-bucket" {
		t.Errorf("ListBuckets returned %v, want [unified-bucket]", names)
	}
}

// TestCopyObjectResolvesAliasSource checks that a copy whose source names a
// legacy bucket still finds the object after it moved into the unified bucket.
// The copy source arrives in a header rather than the URL, so it is parsed
// separately from the request path and has to be resolved on its own.
func TestCopyObjectResolvesAliasSource(t *testing.T) {
	_, mb, h := aliasTestConfig(t)

	body := []byte("copied payload")
	putObject(t, h, "/legacy-bucket/data/src.bin", body)

	req := httptest.NewRequest(http.MethodPut, "/legacy-bucket/data/dst.bin", nil)
	req.Header.Set("x-amz-copy-source", "/legacy-bucket/data/src.bin")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("copy with alias source: status %d, want 200: %s", w.Code, w.Body.String())
	}

	// The copy must have been served from, and written into, the unified bucket.
	if _, ok := mb.objects["unified-bucket/data/dst.bin"]; !ok {
		t.Fatalf("unified-bucket/data/dst.bin missing from backend; the alias source did not resolve")
	}

	get := httptest.NewRequest(http.MethodGet, "/legacy-bucket/data/dst.bin", nil)
	w = httptest.NewRecorder()
	h.HandleRoot(w, get)
	if w.Code != http.StatusOK {
		t.Fatalf("GET copied object: status %d, want 200: %s", w.Code, w.Body.String())
	}
	if !bytes.Equal(w.Body.Bytes(), body) {
		t.Errorf("copied object body %q, want %q", w.Body.String(), string(body))
	}
}

// TestCopyObjectUnknownSourceBucketFails checks that an unknown bucket in the
// copy source is still an error rather than a silent read of the unified
// bucket.
func TestCopyObjectUnknownSourceBucketFails(t *testing.T) {
	_, _, h := aliasTestConfig(t)

	putObject(t, h, "/legacy-bucket/data/src.bin", []byte("payload"))

	req := httptest.NewRequest(http.MethodPut, "/legacy-bucket/data/dst.bin", nil)
	req.Header.Set("x-amz-copy-source", "/unknown-bucket/data/src.bin")
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code == http.StatusOK {
		t.Error("copy from unknown-bucket succeeded, want failure — an unknown source must not resolve to the unified bucket")
	}
}

// TestBucketAliasCannotWidenAccess is the authorization half of the feature.
// Aliasing is applied before the ACL check so that credentials written against
// the configured bucket keep working for clients still sending the legacy
// name, and the same rewrite is what keeps it from being a grant: a credential
// scoped to a bucket an alias does not name gains nothing.
func TestBucketAliasCannotWidenAccess(t *testing.T) {
	cfg := &config.Config{
		Bucket:        "unified-bucket",
		BucketAliases: []string{"legacy-bucket"},
	}

	// A credential written before the consolidation: it names the configured
	// bucket and a single prefix inside it.
	tenantCred := &config.Credential{
		AccessKey: "tenant",
		SecretKey: "secret",
		ACLs: []acl.ACLEntry{
			{Bucket: "unified-bucket", Prefix: "data/"},
		},
	}
	// A credential scoped to the legacy bucket name. No alias maps onto the
	// bucket it names, so it must stay empty.
	legacyCred := &config.Credential{
		AccessKey: "legacy",
		SecretKey: "secret",
		ACLs: []acl.ACLEntry{
			{Bucket: "legacy-bucket", Prefix: ""},
		},
	}

	tests := []struct {
		name    string
		cred    *config.Credential
		urlPath string
		verb    string
		want    error // nil means allowed
	}{
		{
			// The whole point of the feature: an existing ACL string keeps
			// matching a client that still sends the legacy name.
			name:    "alias allowed for credential naming configured bucket",
			cred:    tenantCred,
			urlPath: "/legacy-bucket/data/obj.bin",
			verb:    acl.ActionGet,
			want:    nil,
		},
		{
			name:    "configured bucket still allowed",
			cred:    tenantCred,
			urlPath: "/unified-bucket/data/obj.bin",
			verb:    acl.ActionGet,
			want:    nil,
		},
		{
			// Prefix scoping survives the rewrite.
			name:    "alias outside allowed prefix denied",
			cred:    tenantCred,
			urlPath: "/legacy-bucket/other/obj.bin",
			verb:    acl.ActionGet,
			want:    acl.ErrAccessDenied,
		},
		{
			// An unknown bucket must not be rewritten into the configured one,
			// or any credential for it would become a credential for everything.
			name:    "unknown bucket not mapped onto configured bucket",
			cred:    tenantCred,
			urlPath: "/unknown-bucket/data/obj.bin",
			verb:    acl.ActionGet,
			want:    acl.ErrAccessDenied,
		},
		{
			// A name that merely contains the configured bucket as a substring
			// is a different bucket, not an alias of it.
			name:    "name containing configured bucket is not an alias",
			cred:    tenantCred,
			urlPath: "/unified-bucket-backup/data/obj.bin",
			verb:    acl.ActionGet,
			want:    acl.ErrAccessDenied,
		},
		{
			// The property that makes the rewrite safe: authorization only ever
			// sees the resolved name, so a credential scoped to the legacy name
			// grants nothing at all once the alias exists.
			name:    "credential scoped to legacy name grants nothing",
			cred:    legacyCred,
			urlPath: "/legacy-bucket/data/obj.bin",
			verb:    acl.ActionGet,
			want:    acl.ErrAccessDenied,
		},
		{
			name:    "credential scoped to legacy name gains nothing on configured bucket",
			cred:    legacyCred,
			urlPath: "/unified-bucket/data/obj.bin",
			verb:    acl.ActionGet,
			want:    acl.ErrAccessDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			named, key, _ := strings.Cut(strings.TrimPrefix(tt.urlPath, "/"), "/")
			// This is the server's authorization seam: the bucket is resolved
			// before CheckACL sees it, exactly as extractBucketAndKey does.
			got := acl.CheckACL(tt.cred, cfg.ResolveBucket(named), key, tt.verb)
			if got != tt.want {
				t.Errorf("CheckACL(%q) = %v, want %v", tt.urlPath, got, tt.want)
			}
		})
	}
}

// TestParseBucketAliases parses the env value into the alias list, including the
// entries that must be dropped.
func TestParseBucketAliases(t *testing.T) {
	tests := []struct {
		name string
		list string
		want []string
	}{
		{name: "empty", list: "", want: nil},
		{name: "whitespace only", list: "  ", want: nil},
		{name: "single", list: "legacy-bucket", want: []string{"legacy-bucket"}},
		{name: "multiple", list: "legacy-bucket, older-name", want: []string{"legacy-bucket", "older-name"}},
		{name: "empty entries dropped", list: "legacy-bucket,,older-name,", want: []string{"legacy-bucket", "older-name"}},
		{name: "duplicates dropped", list: "legacy-bucket,legacy-bucket", want: []string{"legacy-bucket"}},
		{
			// The configured bucket is already served from itself; recording it
			// as an alias would claim a consolidation that did not happen.
			name: "configured bucket dropped",
			list: "unified-bucket,legacy-bucket",
			want: []string{"legacy-bucket"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ParseBucketAliases(tt.list, "unified-bucket")
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("ParseBucketAliases(%q) = %v, want %v", tt.list, got, tt.want)
			}
		})
	}
}

// TestResolveBucket is the table over the name classes ResolveBucket maps.
func TestResolveBucket(t *testing.T) {
	cfg := &config.Config{
		Bucket:        "unified-bucket",
		BucketAliases: []string{"legacy-bucket"},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "empty means the configured bucket", input: "", want: "unified-bucket"},
		{name: "configured bucket unchanged", input: "unified-bucket", want: "unified-bucket"},
		{name: "alias maps to configured bucket", input: "legacy-bucket", want: "unified-bucket"},
		{name: "unknown passes through", input: "unknown-bucket", want: "unknown-bucket"},
		{name: "alias is matched exactly", input: "legacy-bucket-backup", want: "legacy-bucket-backup"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.ResolveBucket(tt.input); got != tt.want {
				t.Errorf("ResolveBucket(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestParseBucketKey pins the path-style split, including the percent-encoded
// keys DuckDB httpfs sends.
func TestParseBucketKey(t *testing.T) {
	tests := []struct {
		name       string
		urlPath    string
		wantBucket string
		wantKey    string
	}{
		{name: "root", urlPath: "/", wantBucket: "", wantKey: ""},
		{name: "bucket only", urlPath: "/unified-bucket", wantBucket: "unified-bucket", wantKey: ""},
		{name: "bucket and key", urlPath: "/unified-bucket/a/b.bin", wantBucket: "unified-bucket", wantKey: "a/b.bin"},
		{
			name:       "encoded key decoded",
			urlPath:    "/unified-bucket/a%3Db.bin",
			wantBucket: "unified-bucket",
			wantKey:    "a=b.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := config.ParseBucketKey(tt.urlPath)
			if got.Bucket != tt.wantBucket || got.Key != tt.wantKey {
				t.Errorf("ParseBucketKey(%q) = {%q %q}, want {%q %q}",
					tt.urlPath, got.Bucket, got.Key, tt.wantBucket, tt.wantKey)
			}
		})
	}
}
