package server

import (
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
