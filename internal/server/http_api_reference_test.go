package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestHTTPAPIReferenceCoversRegisteredRoutes keeps the human-facing API
// reference from silently losing a route when the server mux changes. The
// route markers are intentionally in the document as comments: they make the
// check independent of prose wording while keeping the rendered reference
// readable.
func TestHTTPAPIReferenceCoversRegisteredRoutes(t *testing.T) {
	serverSource, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	doc, err := os.ReadFile("../../docs/http-api-reference.md")
	if err != nil {
		t.Fatalf("read HTTP API reference: %v", err)
	}

	routeRE := regexp.MustCompile(`mux\.HandleFunc\("([^"]+)"`)
	seen := make(map[string]bool)
	for _, match := range routeRE.FindAllSubmatch(serverSource, -1) {
		seen[string(match[1])] = true
	}
	if len(seen) == 0 {
		t.Fatal("found no mux registrations in server.go")
	}

	routes := make([]string, 0, len(seen))
	for route := range seen {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		marker := "<!-- endpoint: " + route + " -->"
		if !strings.Contains(string(doc), marker) {
			t.Errorf("registered route %q is missing from docs/http-api-reference.md (expected marker %q)", route, marker)
		}
	}
}

// TestAdminRouteAuthCoverage keeps the admin listener's auth boundary tied to
// the route inventory. Every route registered by AdminHandler must either be
// in the explicit public exemption list or require the admin bearer token;
// dashboard routes are the only other exception because they carry their own
// configured auth. The fixed exemption list also makes a code/docs change
// deliberate when a future public admin route is added.
func TestAdminRouteAuthCoverage(t *testing.T) {
	serverSource, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}

	const adminHandlerStart = "func (s *Server) AdminHandler() http.Handler {"
	start := strings.Index(string(serverSource), adminHandlerStart)
	if start < 0 {
		t.Fatalf("could not find AdminHandler in server.go")
	}
	adminSource := string(serverSource)[start:]
	end := strings.Index(adminSource, "\n// healthz returns")
	if end < 0 {
		t.Fatalf("could not find end of AdminHandler in server.go")
	}
	adminSource = adminSource[:end]

	routeRE := regexp.MustCompile(`mux\.HandleFunc\("([^"]+)"`)
	registered := make(map[string]bool)
	for _, match := range routeRE.FindAllStringSubmatch(adminSource, -1) {
		registered[match[1]] = true
	}
	if len(registered) == 0 {
		t.Fatal("found no admin mux registrations in AdminHandler")
	}

	public := map[string]bool{
		"/healthz":      true,
		"/readyz":       true,
		"/version":      true,
		"/armor/canary": true,
		"/metrics":      true,
	}
	for path := range public {
		if !adminPublicExact[path] {
			t.Errorf("public admin route %q is missing from adminPublicExact", path)
		}
	}
	for path := range adminPublicExact {
		if !public[path] {
			t.Errorf("adminPublicExact contains undocumented test exemption %q", path)
		}
	}

	s := newAdminAuthServerWithDashboard(t, "sekrit", "admin", "pw", "", nil)
	h := s.adminAuthMiddleware(stubOK())
	for path := range registered {
		isDashboardRoute := path == "/dashboard" || strings.HasPrefix(path, "/dashboard/")
		isPublic := public[path] || isDashboardRoute
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		want := http.StatusUnauthorized
		if isPublic {
			want = http.StatusOK
		}
		if rec.Code != want {
			t.Errorf("admin route %s without token: got %d, want %d", path, rec.Code, want)
		}
	}
}

// TestHTTPAPIReferenceListsEveryS3Operation pins the query-dispatched S3
// operations that share the catch-all "/" registration. A mux-path inventory
// alone cannot see these operations because they are selected by method and
// query parameters inside handlers.HandleRoot.
func TestHTTPAPIReferenceListsEveryS3Operation(t *testing.T) {
	doc, err := os.ReadFile("../../docs/http-api-reference.md")
	if err != nil {
		t.Fatalf("read HTTP API reference: %v", err)
	}
	reference := string(doc)

	operations := []string{
		"ListBuckets", "ListObjectsV2", "HeadBucket", "CreateBucket", "DeleteBucket",
		"GetObject", "HeadObject", "PutObject", "DeleteObject", "CopyObject", "DeleteObjects",
		"GetBucketLocation", "GetBucketVersioning", "GetBucketLifecycleConfiguration",
		"PutBucketLifecycleConfiguration", "DeleteBucketLifecycleConfiguration",
		"GetObjectLockConfiguration", "PutObjectLockConfiguration", "GetObjectRetention",
		"PutObjectRetention", "GetObjectLegalHold", "PutObjectLegalHold",
		"CreateMultipartUpload", "UploadPart", "CompleteMultipartUpload", "AbortMultipartUpload",
		"ListParts", "ListMultipartUploads", "ListObjectVersions",
	}
	for _, operation := range operations {
		if !strings.Contains(reference, "`"+operation+"`") {
			t.Errorf("S3 operation %s is missing from docs/http-api-reference.md", operation)
		}
	}
}
