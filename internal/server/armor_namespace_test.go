package server

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestArmorNamespaceProtection verifies that the .armor/ reserved namespace
// is protected and returns 403 AccessDenied for all operations.
func TestArmorNamespaceProtection(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil,
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := server.Handler()

	// Define all operations that should be blocked for .armor/ namespace
	testCases := []struct {
		name           string
		method         string
		url            string
		body           string
		expectedStatus int
	}{
		{
			name:           "GET .armor/chain/writer/file",
			method:         "GET",
			url:            "/test-bucket/.armor/chain/writer/file",
			expectedStatus: 403,
		},
		{
			name:           "PUT .armor/manifest/writer/delta",
			method:         "PUT",
			url:            "/test-bucket/.armor/manifest/writer/delta",
			body:           "test content",
			expectedStatus: 403,
		},
		{
			name:           "DELETE .armor/hmac/sha256",
			method:         "DELETE",
			url:            "/test-bucket/.armor/hmac/abc123",
			expectedStatus: 403,
		},
		{
			name:           "HEAD .armor/canary/object",
			method:         "HEAD",
			url:            "/test-bucket/.armor/canary/test",
			expectedStatus: 403,
		},
		{
			name:           "POST multipart upload to .armor/",
			method:         "POST",
			url:            "/test-bucket/.armor/testfile?uploads",
			expectedStatus: 403,
		},
		{
			name:           "GET .armor/rotation-state.json",
			method:         "GET",
			url:            "/test-bucket/.armor/rotation-state.json",
			expectedStatus: 403,
		},
		{
			name:           "PUT .armor/multipart/upload.state",
			method:         "PUT",
			url:            "/test-bucket/.armor/multipart/upload123.state",
			body:           "state data",
			expectedStatus: 403,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Verify status code is 403
			if w.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, w.Code)
			}

			// Verify response contains AccessDenied error
			body := w.Body.String()
			if !strings.Contains(body, "AccessDenied") {
				t.Errorf("Response should contain AccessDenied error, got: %s", body)
			}

			// Verify response mentions .armor/ namespace
			if !strings.Contains(body, ".armor/") {
				t.Errorf("Response should mention .armor/ namespace, got: %s", body)
			}

			// Verify Content-Type header
			contentType := w.Header().Get("Content-Type")
			if contentType != "application/xml" {
				t.Errorf("Expected Content-Type 'application/xml', got '%s'", contentType)
			}
		})
	}
}

// TestArmorNamespaceProtectionWithAuth verifies that even valid authentication
// cannot access the .armor/ reserved namespace.
func TestArmorNamespaceProtectionWithAuth(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil, // Full access credentials
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := server.Handler()

	// Create a properly authenticated request for .armor/ resource
	req := createInvalidCredTestRequest(t, "GET", "/test-bucket/.armor/protected/file", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should still be rejected with 403
	if w.Code != 403 {
		t.Errorf("Expected status 403 for authenticated request to .armor/ namespace, got %d", w.Code)
	}

	// Verify error message mentions reserved namespace
	body := w.Body.String()
	if !strings.Contains(body, "reserved namespace") {
		t.Errorf("Response should mention reserved namespace, got: %s", body)
	}
}

// TestNonArmorNamespaceAllowed verifies that regular keys are not affected
// by the .armor/ protection (e.g., "myarmor-file.txt" should work).
func TestNonArmorNamespaceAllowed(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil,
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := server.Handler()

	// These should NOT be blocked by .armor/ protection
	testCases := []struct {
		name string
		url  string
	}{
		{
			name: "File starting with 'armor' but not in .armor/ directory",
			url:  "/test-bucket/armor-file.txt",
		},
		{
			name: "File with 'armor' in name but not in .armor/ directory",
			url:  "/test-bucket/my-armor-backup.zip",
		},
		{
			name: "File in directory that contains 'armor' but not .armor/",
			url:  "/test-bucket/files/armor/data.txt",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create authenticated request
			req := createInvalidCredTestRequest(t, "GET", tc.url, "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Should NOT get 403 AccessDenied for .armor/ namespace
			// (We expect 404 or other error, but NOT 403 for reserved namespace)
			if w.Code == 403 {
				body := w.Body.String()
				if strings.Contains(body, "reserved namespace") {
					t.Errorf("Request to %s should not be rejected as .armor/ reserved namespace", tc.url)
				}
			}
		})
	}
}
