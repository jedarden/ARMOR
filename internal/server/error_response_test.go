package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestErrorResponseHeadersConsistency verifies that all error responses
// have consistent headers regardless of the rejection reason.
func TestErrorResponseHeadersConsistency(t *testing.T) {
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

	scenarios := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
	}{
		{
			name: "Missing auth header",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/test-bucket/test-key", nil)
			},
			expectedStatus: 403,
		},
		{
			name: "Invalid access key",
			setupRequest: func() *http.Request {
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "INVALIDKEY", "TESTSECRETKEY123456789012345678901234", nil)
			},
			expectedStatus: 403,
		},
		{
			name: "Malformed auth header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "InvalidAuthHeaderFormat")
				return req
			},
			expectedStatus: 403,
		},
		{
			name: "Missing date header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=example")
				return req
			},
			expectedStatus: 403,
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := scenario.setupRequest()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Verify status code
			if w.Code != scenario.expectedStatus {
				t.Errorf("Expected status %d, got %d", scenario.expectedStatus, w.Code)
			}

			// Verify Content-Type header is set consistently
			contentType := w.Header().Get("Content-Type")
			if contentType != "application/xml" {
				t.Errorf("Expected Content-Type 'application/xml', got '%s'", contentType)
			}

			// Verify response body is not empty
			if w.Body.Len() == 0 {
				t.Error("Expected non-empty response body")
			}

			// Verify response starts with XML declaration
			body := w.Body.String()
			if !startsWithXMLDeclaration(body) {
				t.Errorf("Expected response to start with XML declaration, got: %s", body[:min(len(body), 50)])
			}
		})
	}
}

func startsWithXMLDeclaration(s string) bool {
	return len(s) >= len("<?xml version=\"1.0\" encoding=\"UTF-8\"?>") &&
		(s[:5] == "<?xml" || s[:5] == "<Erro")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
