package server

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestMalformedSignatureRejection tests that requests with malformed signatures
// are rejected with 403 Forbidden and meaningful error messages.
func TestMalformedSignatureRejection(t *testing.T) {
	// Create test credentials
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil, // Full access
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32), // 32-byte MEK is required
		BlockSize:   65536,            // Default block size
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := server.Handler()

	t.Run("Garbage signature string returns 403 Forbidden", func(t *testing.T) {
		tests := []struct {
			name      string
			signature string
		}{
			{
				name:      "non-hex signature",
				signature: "this-is-not-a-valid-hex-signature-at-all!!!",
			},
			{
				name:      "too short signature",
				signature: "abc123",
			},
			{
				name:      "empty signature",
				signature: "",
			},
			{
				name:      "random characters",
				signature: "!@#$%^&*()_+-=[]{}|;':,.<>?/",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := createMalformedSigRequest(t, "GET", "/test-bucket/test-key", "TESTACCESSKEY", tt.signature)

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Check response
				if w.Code != 403 {
					t.Errorf("Expected 403 Forbidden, got %d", w.Code)
				}

				// Parse error response
				var s3Err S3Error
				if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
					t.Errorf("Failed to parse error response: %v", err)
				}

				// Verify error code
				// Empty signatures return IncompleteSignature, others return SignatureDoesNotMatch
				if tt.signature == "" && s3Err.Code != "IncompleteSignature" {
					t.Errorf("Expected error code 'IncompleteSignature' for empty signature, got '%s'", s3Err.Code)
				} else if tt.signature != "" && s3Err.Code != "SignatureDoesNotMatch" {
					t.Errorf("Expected error code 'SignatureDoesNotMatch', got '%s'", s3Err.Code)
				}

				// Verify error message is meaningful
				if s3Err.Message == "" {
					t.Error("Expected meaningful error message, got empty string")
				}
			})
		}
	})

	t.Run("Invalid signature format returns 403 Forbidden", func(t *testing.T) {
		tests := []struct {
			name           string
			authHeader     string
			expectedErrorCode string
		}{
			{
				name:           "missing algorithm prefix",
				authHeader:     "Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123",
				expectedErrorCode: "InvalidAlgorithm",
			},
			{
				name:           "wrong algorithm",
				authHeader:     "AWS4-HMAC-SHA1 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123",
				expectedErrorCode: "InvalidAlgorithm",
			},
			{
				name:           "missing signature component",
				authHeader:     "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host",
				expectedErrorCode: "IncompleteSignature",
			},
			{
				name:           "missing signed headers",
				authHeader:     "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, Signature=abc123",
				expectedErrorCode: "IncompleteSignature",
			},
			{
				name:           "missing credential",
				authHeader:     "AWS4-HMAC-SHA256 SignedHeaders=host, Signature=abc123",
				expectedErrorCode: "IncompleteSignature",
			},
			{
				name:           "malformed credential - insufficient parts",
				authHeader:     "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524, SignedHeaders=host, Signature=abc123",
				expectedErrorCode: "InvalidCredential",
			},
			{
				name:           "empty signature value",
				authHeader:     "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=",
				expectedErrorCode: "IncompleteSignature",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", tt.authHeader)
				req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Check response
				if w.Code != 403 {
					t.Errorf("Expected 403 Forbidden, got %d", w.Code)
				}

				// Parse error response
				var s3Err S3Error
				if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
					t.Errorf("Failed to parse error response: %v", err)
				}

				// Verify error code matches expected
				if s3Err.Code != tt.expectedErrorCode {
					t.Errorf("Expected error code '%s', got '%s'", tt.expectedErrorCode, s3Err.Code)
				}

				// Verify error message is meaningful
				if s3Err.Message == "" {
					t.Error("Expected meaningful error message, got empty string")
				}
			})
		}
	})

	t.Run("Partial signature (missing components) returns 403 Forbidden", func(t *testing.T) {
		tests := []struct {
			name           string
			authHeader     string
			expectedErrorCode string
		}{
			{
				name:           "only algorithm",
				authHeader:     "AWS4-HMAC-SHA256",
				expectedErrorCode: "InvalidAlgorithm",
			},
			{
				name:           "algorithm and credential only",
				authHeader:     "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request",
				expectedErrorCode: "IncompleteSignature",
			},
			{
				name:           "missing signature in valid format",
				authHeader:     "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date",
				expectedErrorCode: "IncompleteSignature",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", tt.authHeader)
				req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Check response
				if w.Code != 403 {
					t.Errorf("Expected 403 Forbidden, got %d", w.Code)
				}

				// Parse error response
				var s3Err S3Error
				if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
					t.Errorf("Failed to parse error response: %v", err)
				}

				// Verify error code
				if s3Err.Code != tt.expectedErrorCode {
					t.Errorf("Expected error code '%s', got '%s'", tt.expectedErrorCode, s3Err.Code)
				}

				// Verify error message is meaningful
				if s3Err.Message == "" {
					t.Error("Expected meaningful error message, got empty string")
				}
			})
		}
	})

	t.Run("Error responses include meaningful error messages", func(t *testing.T) {
		tests := []struct {
			name       string
			authHeader string
		}{
			{
				name:       "invalid algorithm message",
				authHeader: "AWS3-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123",
			},
			{
				name:       "incomplete signature message",
				authHeader: "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request",
			},
			{
				name:       "invalid credential message",
				authHeader: "AWS4-HMAC-SHA256 Credential=invalid/credential/format, SignedHeaders=host, Signature=abc123",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", tt.authHeader)
				req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Parse error response
				var s3Err S3Error
				if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
					t.Errorf("Failed to parse error response: %v", err)
				}

				// Verify error message is meaningful (not empty)
				if s3Err.Message == "" {
					t.Error("Expected meaningful error message, got empty string")
				}

				// Verify error message describes the problem
				// Error messages should be meaningful and non-empty
				messageLower := strings.ToLower(s3Err.Message)
				if s3Err.Message == "" || len(s3Err.Message) < 10 {
					t.Errorf("Error message should be meaningful, got: %s", s3Err.Message)
				}
				// Check that it contains at least one relevant term
				if !strings.Contains(messageLower, "authentication") &&
				   !strings.Contains(messageLower, "signature") &&
				   !strings.Contains(messageLower, "credential") &&
				   !strings.Contains(messageLower, "algorithm") &&
				   !strings.Contains(messageLower, "header") &&
				   !strings.Contains(messageLower, "aws4") {
					t.Errorf("Error message should describe the authentication problem, got: %s", s3Err.Message)
				}
			})
		}
	})

	t.Run("Rejection happens quickly (no long timeouts)", func(t *testing.T) {
		tests := []struct {
			name       string
			authHeader string
		}{
			{
				name:       "malformed algorithm",
				authHeader: "INVALID-ALGORITHM Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=abc123",
			},
			{
				name:       "garbage signature",
				authHeader: "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=!@#$%^&*()",
			},
			{
				name:       "incomplete auth header",
				authHeader: "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", tt.authHeader)
				req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
				req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

				w := httptest.NewRecorder()
				start := time.Now()
				handler.ServeHTTP(w, req)
				elapsed := time.Since(start)

				// Check response is 403
				if w.Code != 403 {
					t.Errorf("Expected 403 Forbidden, got %d", w.Code)
				}

				// Verify rejection was quick (less than 50ms for a local test)
				// This ensures no unnecessary delays or timeouts
				if elapsed > 50*time.Millisecond {
					t.Errorf("Rejection took too long: %v (expected < 50ms)", elapsed)
				}
			})
		}
	})

	t.Run("Valid signature is still accepted", func(t *testing.T) {
		req := createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Should NOT get 403 for valid auth
		// (Might get 404 or other error, but not auth-related)
		if w.Code == 403 {
			var s3Err S3Error
			xml.Unmarshal(w.Body.Bytes(), &s3Err)
			t.Errorf("Valid credentials were rejected with 403: code=%s message=%s", s3Err.Code, s3Err.Message)
		}
	})
}

// createMalformedSigRequest creates a request with a malformed signature
func createMalformedSigRequest(t *testing.T, method, path, accessKey, signature string) *http.Request {
	t.Helper()

	now := time.Now().UTC()
	amzDate := formatAmzDate(now)
	credentialScope := formatAmzDate(now)[:8] + "/us-east-005/s3/aws4_request"

	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", sha256Sum([]byte{}))

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}

	authHeader := "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + credentialScope +
		", SignedHeaders=" + strings.Join(signedHeaders, ";") +
		", Signature=" + signature

	req.Header.Set("Authorization", authHeader)

	return req
}
