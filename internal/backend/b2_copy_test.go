package backend

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestB2BackendCopyEscapesSourceKey(t *testing.T) {
	const sourceKey = "folder/space +#?%/leaf+%.txt"
	const wantSource = "source-bucket/folder/space%20%2B%23%3F%25/leaf%2B%25.txt"

	var gotSource string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSource = r.Header.Get("x-amz-copy-source")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<CopyObjectResult><ETag>"copied"</ETag></CopyObjectResult>`))
	}))
	defer server.Close()

	b2, err := NewB2Backend(context.Background(), B2Config{
		Region:      "us-west-004",
		Endpoint:    server.URL,
		AccessKeyID: "test-access",
		SecretKey:   "REMOVED-NOT-A-SECRET-VALUE",
	})
	if err != nil {
		t.Fatalf("NewB2Backend: %v", err)
	}
	if err := b2.Copy(context.Background(), "source-bucket", sourceKey, "target-bucket", "copy.txt", nil, false); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if gotSource != wantSource {
		t.Errorf("x-amz-copy-source = %q, want %q", gotSource, wantSource)
	}
}
