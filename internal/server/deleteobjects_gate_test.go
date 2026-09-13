package server

// DeleteObjects (POST ?delete) is the only S3 operation whose keys arrive in
// the request body. The middleware gate authorizes from the URL alone, so it
// has no key to check and used to run CheckACL against the empty key — which
// no prefix-scoped ACL can ever match, so the whole batch was rejected with
// 403 before the handler's per-key enforcement could run.
//
// queue-api's litestream replica holds exactly that credential shape
// (delete on one prefix) and its L0 retention issues POST ?delete batches, so
// expired L0 objects were never removed from the replica. These tests pin the
// contract that the gate stays out of the way for this one sub-operation and
// that the per-key decision is what the client sees.

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/config"
)

// deleteResult mirrors the S3 DeleteObjects response document.
type deleteResult struct {
	XMLName xml.Name        `xml:"DeleteResult"`
	Deleted []deletedObject `xml:"Deleted"`
	Errors  []deleteError   `xml:"Error"`
}

type deletedObject struct {
	Key string `xml:"Key"`
}

type deleteError struct {
	Key  string `xml:"Key"`
	Code string `xml:"Code"`
}

// setupDeleteObjectsTestServer starts a real server on a throwaway filesystem
// backend, authenticated by exactly the credentials passed in. The shared
// SetupTestServer helpers predate the filesystem backend's required BasePath
// and cannot start a server any more.
func setupDeleteObjectsTestServer(t *testing.T, credentials map[string]*config.Credential) *TestServer {
	t.Helper()

	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i % 256)
	}

	cfg := &config.Config{
		Backend:     "filesystem",
		FSPath:      t.TempDir(),
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         mek,
		BlockSize:   65536,
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create test server: %v", err)
	}

	testServer := httptest.NewServer(srv.Handler())

	return &TestServer{
		server:     srv,
		testServer: testServer,
		config:     cfg,
		baseURL:    testServer.URL,
	}
}

func postDeleteObjects(t *testing.T, ts *TestServer, accessKey, secretKey, xmlBody string) (*http.Response, deleteResult) {
	t.Helper()

	resp := MakeAuthenticatedRequest(t, ts, "POST", "/test-bucket?delete", []byte(xmlBody),
		accessKey, secretKey)
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	var result deleteResult
	if err := xml.Unmarshal(raw, &result); err != nil {
		t.Fatalf("failed to unmarshal DeleteResult from %q: %v", raw, err)
	}
	return resp, result
}

// TestDeleteObjects_BulkRequestReachesPerKeyACL verifies that a prefix-scoped
// credential's bulk delete is not rejected by the middleware gate, and that
// the handler deletes only the keys inside the credential's prefix.
func TestDeleteObjects_BulkRequestReachesPerKeyACL(t *testing.T) {
	// The real-world shape: a replication credential that may delete inside
	// its own prefix and nothing else.
	creds := map[string]*config.Credential{
		"PREFIXDELETE": {
			AccessKey: "PREFIXDELETE",
			SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
			ACLs: []acl.ACLEntry{
				{
					Bucket: "test-bucket",
					Prefix: "queue-api/",
					Actions: map[string]bool{
						acl.ActionGet:    true,
						acl.ActionPut:    true,
						acl.ActionDelete: true,
						acl.ActionList:   true,
					},
				},
			},
		},
	}

	ts := setupDeleteObjectsTestServer(t, creds)
	defer TeardownTestServer(t, ts)

	body := `<Delete>` +
		`<Object><Key>queue-api/queue.db/0000/0000000000000001-0000000000000002.ltx</Key></Object>` +
		`<Object><Key>other-app/queue.db/0000/0000000000000001-0000000000000002.ltx</Key></Object>` +
		`</Delete>`

	resp, result := postDeleteObjects(t, ts, "PREFIXDELETE", "PREFIXDELETESECRET1234567890123456", body)

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("bulk delete denied before per-key enforcement: the middleware gate checked the empty URL key")
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for a batch with per-key outcomes, got %d", resp.StatusCode)
	}

	if len(result.Deleted) != 1 || result.Deleted[0].Key != "queue-api/queue.db/0000/0000000000000001-0000000000000002.ltx" {
		t.Errorf("expected the in-prefix key to be deleted, got %+v", result.Deleted)
	}
	if len(result.Errors) != 1 || result.Errors[0].Key != "other-app/queue.db/0000/0000000000000001-0000000000000002.ltx" ||
		result.Errors[0].Code != "AccessDenied" {
		t.Errorf("expected the out-of-prefix key to be reported as AccessDenied, got %+v", result.Errors)
	}
}

// TestDeleteObjects_BulkRequestOutsideScopeDeletesNothing verifies that
// skipping the gate does not widen access: a credential with no matching
// prefix gets every key reported as AccessDenied and nothing deleted.
func TestDeleteObjects_BulkRequestOutsideScopeDeletesNothing(t *testing.T) {
	creds := map[string]*config.Credential{
		"UNRELATED": {
			AccessKey: "UNRELATED",
			SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
			ACLs: []acl.ACLEntry{
				{
					Bucket: "test-bucket",
					Prefix: "someone-else/",
					Actions: map[string]bool{
						acl.ActionGet:  true,
						acl.ActionPut:  true,
						acl.ActionList: true,
					},
				},
			},
		},
	}

	ts := setupDeleteObjectsTestServer(t, creds)
	defer TeardownTestServer(t, ts)

	body := `<Delete>` +
		`<Object><Key>queue-api/queue.db/0000/0000000000000001-0000000000000002.ltx</Key></Object>` +
		`</Delete>`

	resp, result := postDeleteObjects(t, ts, "UNRELATED", "UNRELATEDSECRET123456789012345678", body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with per-key errors, got %d", resp.StatusCode)
	}
	if len(result.Deleted) != 0 {
		t.Errorf("expected no deleted keys for an out-of-scope credential, got %+v", result.Deleted)
	}
	if len(result.Errors) != 1 || result.Errors[0].Code != "AccessDenied" {
		t.Errorf("expected the key to be reported as AccessDenied, got %+v", result.Errors)
	}
}
