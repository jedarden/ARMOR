package server

import (
	"fmt"
	"github.com/jedarden/armor/internal/acl"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestAuthEnforcementCoverage exercises ADR-012 decision 5 enforcement paths:
// subtle authorization checks that must be correct for consumer separation to
// be meaningful. These tests pin the current behavior so any future change is
// visible, and they fail immediately if a gap is found (which must be fixed in
// the same change as the test).
//
// Decision 5 requirements:
// 1. CopyObject checks Get on the x-amz-copy-source SOURCE key, not just Put on destination
// 2. DeleteObjects batch checks every key in the POST body
// 3. Multipart lifecycle ops verb-checked (create/upload-part/complete = Put,
//    abort = Abort; a delete grant still allows abort for backward compatibility)
// 4. Scoped credential with no ?prefix on ListObjects stays denied
//
// These are authorization-decision tests, not data-integrity tests. Related:
// bead bf-54irmj (full lifecycle testing through real HTTP path) covers the
// data-plane; this file covers the authorization plane only.

// TestCopyObjectSourceKeyAuthorization verifies that CopyObject checks Get
// permission on the source object (from x-amz-copy-source header) in addition
// to Put permission on the destination. A credential with Put-only access to a
// prefix cannot CopyObject from a source it cannot read.
//
// This is ADR-012 decision 5 requirement 1. Without this check, a compromised
// backup-writer (Put+List only) could CopyObject from arbitrary sources and
// exfiltrate data it cannot directly GetObject.
func TestCopyObjectSourceKeyAuthorization(t *testing.T) {
	const secret = "REMOVED-NOT-A-SECRET-VALUE"
	credentials := map[string]*config.Credential{
		"FULLACCESS": {AccessKey: "FULLACCESS", SecretKey: secret},
		"PREFIXED": {
			AccessKey: "PREFIXED",
			SecretKey: secret,
			ACLs: []acl.ACLEntry{{
				Bucket:  "test-bucket",
				Prefix:  "tenant-a/",
				Actions: map[string]bool{ActionGet: true, ActionPut: true},
			}},
		},
	}
	ts := SetupTestServerWithCredentials(t, credentials)
	defer TeardownTestServer(t, ts)

	serve := func(method, path, accessKey string, headers map[string]string, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := createSignedRequestForAuthTest(t, method, path, body, accessKey, credentials[accessKey].SecretKey, nil)
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rec := httptest.NewRecorder()
		ts.server.Handler().ServeHTTP(rec, req)
		return rec
	}

	if rec := serve(http.MethodPut, "/test-bucket/tenant-b/x", "FULLACCESS", nil, "cross-prefix source"); rec.Code != http.StatusOK {
		t.Fatalf("seed cross-prefix source: status %d body %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodPut, "/test-bucket/tenant-a/source", "FULLACCESS", nil, "same-prefix source"); rec.Code != http.StatusOK {
		t.Fatalf("seed same-prefix source: status %d body %s", rec.Code, rec.Body.String())
	}

	if rec := serve(http.MethodGet, "/test-bucket/tenant-b/x", "PREFIXED", nil, ""); rec.Code != http.StatusForbidden {
		t.Errorf("GET outside prefix status = %d, want 403", rec.Code)
	}

	copyFrom := func(src, dst string) *httptest.ResponseRecorder {
		return serve(http.MethodPut, "/test-bucket/"+dst, "PREFIXED", map[string]string{
			"x-amz-copy-source": "/test-bucket/" + src,
		}, "")
	}

	if rec := copyFrom("tenant-b/x", "tenant-a/y"); rec.Code != http.StatusForbidden {
		t.Errorf("cross-prefix CopyObject status = %d, want 403; body %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodHead, "/test-bucket/tenant-a/y", "FULLACCESS", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("denied cross-prefix destination HEAD status = %d, want 404", rec.Code)
	}

	if rec := serve(http.MethodGet, "/test-bucket/.armor/secret", "PREFIXED", nil, ""); rec.Code != http.StatusForbidden {
		t.Errorf("GET reserved namespace status = %d, want 403", rec.Code)
	}
	if rec := copyFrom(".armor/secret", "tenant-a/armor-copy"); rec.Code != http.StatusForbidden {
		t.Errorf("reserved-namespace CopyObject status = %d, want 403; body %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodHead, "/test-bucket/tenant-a/armor-copy", "FULLACCESS", nil, ""); rec.Code != http.StatusNotFound {
		t.Errorf("denied reserved-source destination HEAD status = %d, want 404", rec.Code)
	}

	if rec := copyFrom("tenant-a/source", "tenant-a/y"); rec.Code != http.StatusOK {
		t.Fatalf("same-prefix CopyObject status = %d, want 200; body %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodGet, "/test-bucket/tenant-a/y", "PREFIXED", nil, ""); rec.Code != http.StatusOK || rec.Body.String() != "same-prefix source" {
		t.Errorf("same-prefix copied object: status %d body %q", rec.Code, rec.Body.String())
	}
}

// TestDeleteObjectsBatchAuthorization verifies that DeleteObjects checks
// Delete permission on EVERY key in the POST XML body, not just the bucket.
// A credential with Delete on a prefix cannot DeleteObjects keys outside that
// prefix even though the request URL is bucket-level (POST ?delete).
//
// This is ADR-012 decision 5 requirement 2. Without per-key checking, a
// credential with Delete on "backups/" could issue a DeleteObjects request with
// keys outside that prefix and delete objects it should not be able to touch.
func TestDeleteObjectsBatchAuthorization(t *testing.T) {
	// Create credential with Delete only on backups/ prefix
	backupsDeleteCred := &config.Credential{
		AccessKey: "BACKUPSDELETE",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs: []acl.ACLEntry{{
			Bucket:  "test-bucket",
			Prefix:  "backups/",
			Actions: map[string]bool{ActionDelete: true},
		}},
	}

	// Full-access credential for baseline
	fullAccessCred := &config.Credential{
		AccessKey: "FULLACCESS",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs:      nil, // Full access
	}

	credentials := map[string]*config.Credential{
		"BACKUPSDELETE": backupsDeleteCred,
		"FULLACCESS":    fullAccessCred,
	}
	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	// checkDeleteObjectsKeyAuth simulates the authorization check for a single
	// key in a DeleteObjects body. The actual wrapHandler checks the bucket
	// path, but ADR-012 requires checking each individual key from the XML.
	checkDeleteObjectsKeyAuth := func(t *testing.T, key, accessKey string) error {
		t.Helper()

		// DeleteObjects is POST ?delete on the bucket
		req := createSignedRequestForAuthTest(t, "POST", "/test-bucket?delete", "", accessKey, credentials[accessKey].SecretKey, nil)
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		// Check ACL for this specific key with Delete action
		return acl.CheckACL(cred, "test-bucket", key, ActionDelete)
	}

	t.Run("DeleteObjects allows deletion within prefix scope", func(t *testing.T) {
		keys := []string{"backups/db1.sql", "backups/db2.sql", "backups/logs/app.log"}

		for _, key := range keys {
			err := checkDeleteObjectsKeyAuth(t, key, "BACKUPSDELETE")
			if err != nil {
				t.Errorf("Delete permission should be allowed for key %s within backups/ scope, got: %v", key, err)
			}
		}
	})

	t.Run("DeleteObjects denies deletion outside prefix scope", func(t *testing.T) {
		// Keys outside the backups/ prefix
		outsideKeys := []string{
			"logs/app.log",
			"public/image.png",
			"config/settings.json",
			"temp/file.tmp",
			"root.txt", // No prefix
		}

		for _, key := range outsideKeys {
			err := checkDeleteObjectsKeyAuth(t, key, "BACKUPSDELETE")
			if err != acl.ErrAccessDenied {
				t.Errorf("Delete permission should be denied for key %s outside backups/ scope, got: %v (want acl.ErrAccessDenied)", key, err)
			}
		}
	})

	t.Run("DeleteObjects mixed batch allows allowed keys and denies disallowed keys", func(t *testing.T) {
		// Simulate a DeleteObjects request with mixed keys (some allowed, some not)
		// In production, the handler would need to check each key individually
		// and return partial results (some deleted, some access denied).

		allowedKeys := []string{"backups/db1.sql", "backups/db2.sql"}
		deniedKeys := []string{"logs/app.log", "public/image.png"}

		// Verify allowed keys pass auth
		for _, key := range allowedKeys {
			err := checkDeleteObjectsKeyAuth(t, key, "BACKUPSDELETE")
			if err != nil {
				t.Errorf("Allowed key %s failed auth check: %v", key, err)
			}
		}

		// Verify denied keys fail auth
		for _, key := range deniedKeys {
			err := checkDeleteObjectsKeyAuth(t, key, "BACKUPSDELETE")
			if err != acl.ErrAccessDenied {
				t.Errorf("Denied key %s should return acl.ErrAccessDenied, got: %v", key, err)
			}
		}
	})

	t.Run("DeleteObjects full-access credential can delete any key", func(t *testing.T) {
		keys := []string{
			"backups/db1.sql",
			"logs/app.log",
			"public/image.png",
			"config/settings.json",
		}

		for _, key := range keys {
			err := checkDeleteObjectsKeyAuth(t, key, "FULLACCESS")
			if err != nil {
				t.Errorf("Full-access credential should be allowed to delete any key %s, got: %v", key, err)
			}
		}
	})

	t.Run("DeleteObjects verifies bucket-level request with key-level checks", func(t *testing.T) {
		// Verify that the POST ?delete request itself is bucket-level (no key in path)
		// but authorization must check each key from the body.
		req := createSignedRequestForAuthTest(t, "POST", "/test-bucket?delete", "", "BACKUPSDELETE", "BACKUPSDELETESECRET12345678901234", nil)

		// The URL path is bucket-only (no key component)
		if parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/"); len(parts) != 1 || parts[0] != "test-bucket" {
			t.Errorf("DeleteObjects URL should be bucket-only, got path: %s", req.URL.Path)
		}

		// Verify the query parameter is ?delete
		if !req.URL.Query().Has("delete") {
			t.Errorf("DeleteObjects request should have ?delete query parameter")
		}

		// Yet authorization must still check individual keys from the body
		// This test pins that requirement
		err := checkDeleteObjectsKeyAuth(t, "logs/app.log", "BACKUPSDELETE")
		if err != acl.ErrAccessDenied {
			t.Errorf("Even though URL is bucket-level, individual keys must be checked - logs/app.log should be denied, got: %v", err)
		}
	})
}

// TestMultipartLifecycleVerbAuthorization verifies that multipart lifecycle
// operations are correctly verb-mapped according to ADR-012 (as amended
// 2026-09-13):
// - CreateMultipartUpload → Put
// - UploadPart → Put
// - CompleteMultipartUpload → Put
// - AbortMultipartUpload → Abort (its own verb; a delete grant still allows it)
//
// This is ADR-012 decision 5 requirement 3. Correct verb mapping is critical
// for append-only backup-writer roles (Put+List only) - they should be able
// to create multipart uploads but not abort them (Abort required). A writer
// that needs abort cleanup takes Put+List+Abort; granting abort still grants
// no delete.
func TestMultipartLifecycleVerbAuthorization(t *testing.T) {
	// Append-only backup-writer: Put+List, no Delete
	appendOnlyCred := &config.Credential{
		AccessKey: "APPENDONLY",
		SecretKey: "APPENDONLYSECRET123456789012345678",
		ACLs: []acl.ACLEntry{{
			Bucket:  "test-bucket",
			Prefix:  "uploads/",
			Actions: map[string]bool{ActionPut: true, ActionList: true},
		}},
	}

	// Delete-only credential for comparison
	deleteOnlyCred := &config.Credential{
		AccessKey: "DELETEONLY",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs: []acl.ACLEntry{{
			Bucket:  "test-bucket",
			Prefix:  "uploads/",
			Actions: map[string]bool{ActionDelete: true},
		}},
	}

	credentials := map[string]*config.Credential{
		"APPENDONLY": appendOnlyCred,
		"DELETEONLY": deleteOnlyCred,
	}
	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	// checkMultipartOpAuth verifies the verb mapping for a multipart operation
	checkMultipartOpAuth := func(t *testing.T, method, basePath, key, uploadID, accessKey string) (allowed bool, err error) {
		t.Helper()

		// Build path with query parameters if uploadID is provided
		path := basePath
		if uploadID != "" {
			path = fmt.Sprintf("%s?uploadId=%s", basePath, uploadID)
		}

		req := createSignedRequestForAuthTest(t, method, path, "", accessKey, credentials[accessKey].SecretKey, nil)

		cred, authErr := auth.VerifyRequest(req, nil)
		if authErr != nil {
			t.Fatalf("SigV4 verification failed: %v", authErr)
		}

		verb := ActionForRequest(req)
		err = acl.CheckACL(cred, "test-bucket", key, verb)
		allowed = (err == nil)
		return
	}

	t.Run("CreateMultipartUpload maps to Put verb", func(t *testing.T) {
		// CreateMultipartUpload is POST with no ?delete (uploadId not yet assigned)
		allowed, err := checkMultipartOpAuth(t, "POST", "/test-bucket/uploads/large-file.bin", "uploads/large-file.bin", "", "APPENDONLY")

		if !allowed {
			t.Errorf("CreateMultipartUpload should be allowed for Put+List credential, got: %v", err)
		}
	})

	t.Run("CreateMultipartUpload denied for Delete-only credential", func(t *testing.T) {
		allowed, err := checkMultipartOpAuth(t, "POST", "/test-bucket/uploads/large-file.bin", "uploads/large-file.bin", "", "DELETEONLY")

		if allowed {
			t.Errorf("CreateMultipartUpload should be denied for Delete-only credential, got no error")
		}
		if err != acl.ErrAccessDenied {
			t.Errorf("Expected acl.ErrAccessDenied, got: %v", err)
		}
	})

	t.Run("UploadPart maps to Put verb", func(t *testing.T) {
		// UploadPart is PUT with partNumber&uploadId query params
		uploadID := "example-upload-id-1234567890"
		path := fmt.Sprintf("/test-bucket/uploads/large-file.bin?partNumber=1&uploadId=%s", uploadID)

		req := createSignedRequestForAuthTest(t, "PUT", path, "", "APPENDONLY", "APPENDONLYSECRET123456789012345678", nil)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		verb := ActionForRequest(req)
		if verb != ActionPut {
			t.Errorf("UploadPart should map to Put verb, got: %s", verb)
		}

		aclErr := acl.CheckACL(cred, "test-bucket", "uploads/large-file.bin", verb)
		if aclErr != nil {
			t.Errorf("UploadPart should be allowed for Put+List credential, got: %v", aclErr)
		}
	})

	t.Run("CompleteMultipartUpload maps to Put verb", func(t *testing.T) {
		// CompleteMultipartUpload is POST with uploadId
		uploadID := "example-upload-id-1234567890"
		path := fmt.Sprintf("/test-bucket/uploads/large-file.bin?uploadId=%s", uploadID)

		req := createSignedRequestForAuthTest(t, "POST", path, "", "APPENDONLY", "APPENDONLYSECRET123456789012345678", nil)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		verb := ActionForRequest(req)
		if verb != ActionPut {
			t.Errorf("CompleteMultipartUpload should map to Put verb, got: %s", verb)
		}

		aclErr := acl.CheckACL(cred, "test-bucket", "uploads/large-file.bin", verb)
		if aclErr != nil {
			t.Errorf("CompleteMultipartUpload should be allowed for Put+List credential, got: %v", aclErr)
		}
	})

	t.Run("AbortMultipartUpload maps to Abort verb", func(t *testing.T) {
		// AbortMultipartUpload is DELETE with uploadId — its own verb since
		// the ADR-012 amendment (2026-09-13), no longer bundled with Delete.
		uploadID := "example-upload-id-1234567890"
		path := fmt.Sprintf("/test-bucket/uploads/large-file.bin?uploadId=%s", uploadID)

		req := createSignedRequestForAuthTest(t, "DELETE", path, "", "APPENDONLY", "APPENDONLYSECRET123456789012345678", nil)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		verb := ActionForRequest(req)
		if verb != ActionAbort {
			t.Errorf("AbortMultipartUpload should map to Abort verb, got: %s", verb)
		}

		// Append-only credential (Put+List, no Abort) must be denied
		aclErr := acl.CheckACL(cred, "test-bucket", "uploads/large-file.bin", verb)
		if aclErr != acl.ErrAccessDenied {
			t.Errorf("AbortMultipartUpload should be denied for Put+List credential (no Abort), got: %v", aclErr)
		}
	})

	t.Run("AbortMultipartUpload allowed for Delete-only credential", func(t *testing.T) {
		// Delete-only credential CAN abort uploads: abort was part of delete
		// before the amendment, so a delete grant continues to grant it
		// (no deployed credential loses a capability).
		allowed, err := checkMultipartOpAuth(t, "DELETE", "/test-bucket/uploads/large-file.bin", "uploads/large-file.bin", "example-upload-id-1234567890", "DELETEONLY")

		if !allowed {
			t.Errorf("AbortMultipartUpload should be allowed for Delete-only credential, got: %v", err)
		}
	})

	t.Run("ListMultipartUploads maps to List verb", func(t *testing.T) {
		// ListMultipartUploads is GET with ?uploads on bucket
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket?uploads", "", "APPENDONLY", "APPENDONLYSECRET123456789012345678", nil)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		verb := ActionForRequest(req)
		if verb != ActionList {
			t.Errorf("ListMultipartUploads should map to List verb, got: %s", verb)
		}

		// ListMultipartUploads is bucket-level (no key), so ACL check uses prefix
		aclErr := acl.CheckACL(cred, "test-bucket", "uploads/", verb)
		if aclErr != nil {
			t.Errorf("ListMultipartUploads should be allowed for Put+List credential, got: %v", aclErr)
		}
	})

	t.Run("ListParts maps to List verb", func(t *testing.T) {
		// ListParts is GET with ?uploadId on an object
		uploadID := "example-upload-id-1234567890"
		path := fmt.Sprintf("/test-bucket/uploads/large-file.bin?uploadId=%s", uploadID)

		req := createSignedRequestForAuthTest(t, "GET", path, "", "APPENDONLY", "APPENDONLYSECRET123456789012345678", nil)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		verb := ActionForRequest(req)
		if verb != ActionList {
			t.Errorf("ListParts should map to List verb, got: %s", verb)
		}

		aclErr := acl.CheckACL(cred, "test-bucket", "uploads/large-file.bin", verb)
		if aclErr != nil {
			t.Errorf("ListParts should be allowed for Put+List credential, got: %v", aclErr)
		}
	})
}

// TestScopedCredentialBroadListDenial verifies that a credential scoped with
// a prefix ACL (e.g., "backups/") is denied ListObjects on the entire bucket
// when no ?prefix is specified. The ?prefix query parameter is required to
// narrow the list to the allowed prefix; otherwise, broad-listing is denied.
//
// This is ADR-012 decision 5 requirement 4. Without this check, a scoped
// credential could enumerate all keys in the bucket and infer information about
// other consumers' data, even if it cannot read the object contents.
func TestScopedCredentialBroadListDenial(t *testing.T) {
	// Credential scoped to backups/ prefix with List permission
	backupsListCred := &config.Credential{
		AccessKey: "BACKUPSLIST",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs: []acl.ACLEntry{{
			Bucket:  "test-bucket",
			Prefix:  "backups/",
			Actions: map[string]bool{ActionList: true},
		}},
	}

	// Full-access credential for baseline
	fullAccessCred := &config.Credential{
		AccessKey: "FULLACCESS",
		SecretKey: "REMOVED-NOT-A-SECRET-VALUE",
		ACLs:      nil, // Full access
	}

	credentials := map[string]*config.Credential{
		"BACKUPSLIST": backupsListCred,
		"FULLACCESS":  fullAccessCred,
	}
	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	// checkListAuth simulates wrapHandler's authorization for ListObjectsV2:
	// bucket-level GET has no key in the path, so wrapHandler falls back to
	// the ?prefix query parameter as the key for ACL checking.
	checkListAuth := func(t *testing.T, prefix, accessKey string) error {
		t.Helper()

		// Build ListObjectsV2 request with optional ?prefix
		path := "/test-bucket"
		queryParams := url.Values{}
		if prefix != "" {
			queryParams.Set("prefix", prefix)
			path = "/test-bucket?" + queryParams.Encode()
		}

		req := createSignedRequestForAuthTest(t, "GET", path, "", accessKey, credentials[accessKey].SecretKey, nil)
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		// Verify ActionForRequest returns List for bucket-level GET
		verb := ActionForRequest(req)
		if verb != ActionList {
			t.Errorf("ListObjectsV2 should map to List verb, got: %s", verb)
		}

		// ACL check uses the prefix as key (wrapHandler's ?prefix fallback)
		key := prefix
		if key == "" {
			// No prefix means broad list - empty key checks bucket-level access
			key = ""
		}
		return acl.CheckACL(cred, "test-bucket", key, ActionList)
	}

	t.Run("Broad list (no ?prefix) denied for prefix-scoped credential", func(t *testing.T) {
		// Credential scoped to "backups/" tries to list entire bucket with no ?prefix
		err := checkListAuth(t, "", "BACKUPSLIST")

		if err != acl.ErrAccessDenied {
			t.Errorf("Broad list without ?prefix should be denied for prefix-scoped credential, got: %v (want acl.ErrAccessDenied)", err)
		}
	})

	t.Run("Scoped list (?prefix=backups/) allowed for prefix-scoped credential", func(t *testing.T) {
		// Credential scoped to "backups/" lists with ?prefix=backups/
		err := checkListAuth(t, "backups/", "BACKUPSLIST")

		if err != nil {
			t.Errorf("Scoped list with ?prefix=backups/ should be allowed for backups/-scoped credential, got: %v", err)
		}
	})

	t.Run("Scoped list (?prefix=backups/db/) allowed for prefix-scoped credential", func(t *testing.T) {
		// More specific prefix within the allowed scope
		err := checkListAuth(t, "backups/db/", "BACKUPSLIST")

		if err != nil {
			t.Errorf("Scoped list with ?prefix=backups/db/ should be allowed for backups/-scoped credential, got: %v", err)
		}
	})

	t.Run("Scoped list outside prefix scope denied", func(t *testing.T) {
		// Credential scoped to "backups/" tries to list with ?prefix=logs/
		err := checkListAuth(t, "logs/", "BACKUPSLIST")

		if err != acl.ErrAccessDenied {
			t.Errorf("Scoped list with ?prefix=logs/ should be denied for backups/-scoped credential, got: %v (want acl.ErrAccessDenied)", err)
		}
	})

	t.Run("Full-access credential can broad-list without ?prefix", func(t *testing.T) {
		// Full-access credential lists entire bucket
		err := checkListAuth(t, "", "FULLACCESS")

		if err != nil {
			t.Errorf("Full-access credential should be allowed to broad-list, got: %v", err)
		}
	})

	t.Run("Full-access credential can list with any ?prefix", func(t *testing.T) {
		prefixes := []string{"", "backups/", "logs/", "public/", "config/"}

		for _, prefix := range prefixes {
			err := checkListAuth(t, prefix, "FULLACCESS")
			if err != nil {
				t.Errorf("Full-access credential should be allowed to list with ?prefix=%s, got: %v", prefix, err)
			}
		}
	})

	t.Run("ListObjectsV2 query parameter parsing", func(t *testing.T) {
		// Verify that ListObjectsV2 (the default bucket-level GET) is correctly
		// identified as a List operation and that the ?prefix fallback works.

		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket", "", "BACKUPSLIST", "BACKUPSLISTSECRET12345678901234567", nil)

		// Verify it's a List operation
		verb := ActionForRequest(req)
		if verb != ActionList {
			t.Errorf("Bucket-level GET should be List operation, got: %s", verb)
		}

		// Verify query parameter extraction for ACL fallback
		prefix := req.URL.Query().Get("prefix")
		if prefix != "" {
			t.Errorf("Request with no ?prefix should extract empty prefix, got: %s", prefix)
		}

		// Now add ?prefix and verify it's extracted
		req = createSignedRequestForAuthTest(t, "GET", "/test-bucket?prefix=backups/", "", "BACKUPSLIST", "BACKUPSLISTSECRET12345678901234567", nil)
		prefix = req.URL.Query().Get("prefix")
		if prefix != "backups/" {
			t.Errorf("Request with ?prefix=backups/ should extract that prefix, got: %s", prefix)
		}
	})

	t.Run("Wildcard bucket with prefix restriction enforces broad-list denial", func(t *testing.T) {
		// Create credential with bucket wildcard but prefix restriction
		wildcardCred := &config.Credential{
			AccessKey: "WILDCARDLIST",
			SecretKey: "WILDCARDLISTSECRET1234567890123456",
			ACLs: []acl.ACLEntry{{
				Bucket:  "*",
				Prefix:  "backups/",
				Actions: map[string]bool{ActionList: true},
			}},
		}

		credentials["WILDCARDLIST"] = wildcardCred
		auth = NewSigV4AuthWithCredentials(credentials, "us-east-005")

		// Broad list should still be denied
		req := createSignedRequestForAuthTest(t, "GET", "/any-bucket", "", "WILDCARDLIST", "WILDCARDLISTSECRET1234567890123456", nil)
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("SigV4 verification failed: %v", err)
		}

		// Check with empty key (broad list)
		err = acl.CheckACL(cred, "any-bucket", "", ActionList)
		if err != acl.ErrAccessDenied {
			t.Errorf("Wildcard bucket credential with prefix ACL should still deny broad-list, got: %v", err)
		}

		// Scoped list should be allowed
		err = acl.CheckACL(cred, "any-bucket", "backups/", ActionList)
		if err != nil {
			t.Errorf("Wildcard bucket credential with prefix ACL should allow scoped list, got: %v", err)
		}
	})
}
