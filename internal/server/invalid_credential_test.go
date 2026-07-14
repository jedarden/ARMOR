package server

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestInvalidCredentialRejection tests that invalid credentials are properly rejected
// with appropriate error responses at the HTTP layer.
func TestInvalidCredentialRejection(t *testing.T) {
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

	t.Run("Invalid AWS credentials return 403 Forbidden", func(t *testing.T) {
		// Create a request with an invalid access key
		req := createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "INVALIDKEY", "TESTSECRETKEY123456789012345678901234", nil)

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
		if s3Err.Code != "InvalidAccessKeyId" {
			t.Errorf("Expected error code 'InvalidAccessKeyId', got '%s'", s3Err.Code)
		}

		// Verify error message is meaningful
		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Malformed signatures return 403 Forbidden", func(t *testing.T) {
		// Create a request with a wrong secret key (invalid signature)
		req := createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "WRONGSECRETKEY123456789012345678901", nil)

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
		if s3Err.Code != "SignatureDoesNotMatch" {
			t.Errorf("Expected error code 'SignatureDoesNotMatch', got '%s'", s3Err.Code)
		}

		// Verify error message is meaningful
		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Missing authentication headers return 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		// No Authorization header

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
		if s3Err.Code != "MissingAuthenticationToken" {
			t.Errorf("Expected error code 'MissingAuthenticationToken', got '%s'", s3Err.Code)
		}

		// Verify error message is meaningful
		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Malformed authorization header returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		req.Header.Set("Authorization", "InvalidAuthHeaderFormat")

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

		// Verify error code indicates a problem with the auth header
		if s3Err.Code == "" {
			t.Error("Expected error code, got empty string")
		}
	})

	t.Run("Missing authentication headers on POST return 403 Forbidden", func(t *testing.T) {
		postBody := []byte("test content for upload")
		req := httptest.NewRequest("POST", "/test-bucket/test-key", bytes.NewReader(postBody))
		// No Authorization header

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
		if s3Err.Code != "MissingAuthenticationToken" {
			t.Errorf("Expected error code 'MissingAuthenticationToken', got '%s'", s3Err.Code)
		}

		// Verify error message is meaningful
		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Missing date header returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=example")
		// No X-Amz-Date header

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
		if s3Err.Code != "MissingDateHeader" {
			t.Errorf("Expected error code 'MissingDateHeader', got '%s'", s3Err.Code)
		}
	})

	t.Run("Expired request returns 403 Forbidden", func(t *testing.T) {
		oldTime := time.Now().Add(-20 * time.Minute)
		req := createSignedRequestWithTime(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", &oldTime)

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
		if s3Err.Code != "RequestExpired" {
			t.Errorf("Expected error code 'RequestExpired', got '%s'", s3Err.Code)
		}

		// Verify error message is meaningful
		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Rejection happens quickly", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		// No Authorization header

		w := httptest.NewRecorder()
		start := time.Now()
		handler.ServeHTTP(w, req)
		elapsed := time.Since(start)

		// Check response
		if w.Code != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", w.Code)
		}

		// Verify rejection was quick (less than 100ms for a local test)
		if elapsed > 100*time.Millisecond {
			t.Errorf("Rejection took too long: %v (expected < 100ms)", elapsed)
		}
	})

	t.Run("Valid authentication still works", func(t *testing.T) {
		req := createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Should NOT get 403 for valid auth
		// (Might get 404 or other error, but not auth-related)
		if w.Code == 403 {
			body := w.Body.String()
			t.Errorf("Valid credentials were rejected with 403: %s", body)
		}
	})
}

// createSignedRequestWithTime is an alias for createInvalidCredTestRequest for compatibility
func createSignedRequestWithTime(t *testing.T, method, path, body, accessKey, secretKey string, timestamp *time.Time) *http.Request {
	return createInvalidCredTestRequest(t, method, path, body, accessKey, secretKey, timestamp)
}

// Helper function to create a signed request for invalid credential tests
func createInvalidCredTestRequest(t *testing.T, method, path, body, accessKey, secretKey string, timestamp *time.Time) *http.Request {
	t.Helper()

	now := time.Now().UTC()
	if timestamp != nil {
		now = *timestamp
	}

	amzDate := formatAmzDate(now)
	credentialScope := formatAmzDate(now)[:8] + "/us-east-005/s3/aws4_request"

	var bodyBytes []byte
	if body != "" {
		bodyBytes = []byte(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", sha256Sum(bodyBytes))

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}

	canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, bodyBytes)
	stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)
	signingKey := deriveSigningKey(secretKey, amzDate[:8], "us-east-005")
	signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

	authHeader := "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + credentialScope +
		", SignedHeaders=" + strings.Join(signedHeaders, ";") +
		", Signature=" + signature

	req.Header.Set("Authorization", authHeader)

	return req
}
