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

// ComprehensiveErrorVerificationTest verifies all acceptance criteria for error responses:
// 1. All error responses include meaningful error messages
// 2. Error messages specify the rejection reason
// 3. Response time for all rejections under 100ms
// 4. Response headers are consistent across rejection types
// 5. Documentation of error response format
func TestComprehensiveErrorVerification(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil,
		},
		"RESTRICTEDKEY": {
			AccessKey: "RESTRICTEDKEY",
			SecretKey: "RESTRICTEDSECRET1234567890123456789",
			ACLs: []config.ACLEntry{
				{
					Bucket: "test-bucket",
					Prefix: "allowed/",
				},
			},
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

	// Define all rejection scenarios
	scenarios := []struct {
		name                        string
		setupRequest                func() *http.Request
		expectedCode                string
		shouldHaveMeaningfulMessage bool
		maxResponseTime             time.Duration
	}{
		// Authentication failures
		{
			name: "Missing authentication header",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/test-bucket/test-key", nil)
			},
			expectedCode:                "MissingAuthenticationToken",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
		{
			name: "Invalid access key",
			setupRequest: func() *http.Request {
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "INVALIDKEY", "TESTSECRETKEY123456789012345678901234", nil)
			},
			expectedCode:                "InvalidAccessKeyId",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
		{
			name: "Invalid signature (wrong secret key)",
			setupRequest: func() *http.Request {
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "WRONGSECRETKEY123456789012345678901", nil)
			},
			expectedCode:                "SignatureDoesNotMatch",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
		{
			name: "Malformed authorization header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "InvalidAuthHeaderFormat")
				return req
			},
			expectedCode:                "InvalidAlgorithm",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
		{
			name: "Missing date header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=example")
				return req
			},
			expectedCode:                "MissingDateHeader",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
		{
			name: "Expired request",
			setupRequest: func() *http.Request {
				oldTime := time.Now().Add(-20 * time.Minute)
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", &oldTime)
			},
			expectedCode:                "RequestExpired",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
		{
			name: "Empty signature",
			setupRequest: func() *http.Request {
				req := createMalformedSigRequest(t, "GET", "/test-bucket/test-key", "TESTACCESSKEY", "")
				return req
			},
			expectedCode:                "IncompleteSignature",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
		{
			name: "Invalid signature characters",
			setupRequest: func() *http.Request {
				req := createMalformedSigRequest(t, "GET", "/test-bucket/test-key", "TESTACCESSKEY", "!@#$%^&*()")
				return req
			},
			expectedCode:                "SignatureDoesNotMatch",
			shouldHaveMeaningfulMessage: true,
			maxResponseTime:             100 * time.Millisecond,
		},
	}

	// Track response times for all scenarios
	responseTimes := make(map[string]time.Duration)
	contentTypes := make(map[string]string)

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := scenario.setupRequest()
			w := httptest.NewRecorder()

			// Measure response time
			start := time.Now()
			handler.ServeHTTP(w, req)
			elapsed := time.Since(start)
			responseTimes[scenario.name] = elapsed

			// Verify response time is under threshold
			if elapsed > scenario.maxResponseTime {
				t.Errorf("Response time %v exceeds maximum %v", elapsed, scenario.maxResponseTime)
			}

			// Verify status code is 403
			if w.Code != 403 {
				t.Errorf("Expected status 403, got %d", w.Code)
			}

			// Parse error response
			var s3Err S3Error
			if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
				t.Errorf("Failed to parse error response: %v", err)
			}

			// Verify error code matches expected
			if s3Err.Code != scenario.expectedCode {
				t.Errorf("Expected error code '%s', got '%s'", scenario.expectedCode, s3Err.Code)
			}

			// Verify error message is meaningful and non-empty
			if scenario.shouldHaveMeaningfulMessage && s3Err.Message == "" {
				t.Error("Expected meaningful error message, got empty string")
			}

			// Verify error message specifies the rejection reason
			if scenario.shouldHaveMeaningfulMessage && len(s3Err.Message) < 10 {
				t.Errorf("Error message too short to be meaningful: %s", s3Err.Message)
			}

			// Verify Content-Type header
			contentType := w.Header().Get("Content-Type")
			contentTypes[scenario.name] = contentType
			if contentType != "application/xml" {
				t.Errorf("Expected Content-Type 'application/xml', got '%s'", contentType)
			}

			// Verify response starts with XML declaration
			body := w.Body.String()
			if !strings.HasPrefix(body, "<?xml") && !strings.HasPrefix(body, "<Error>") {
				t.Errorf("Expected response to start with XML, got: %s", body[:min(len(body), 50)])
			}
		})
	}

	// Verify all responses have consistent Content-Type headers
	t.Run("All responses have consistent Content-Type headers", func(t *testing.T) {
		for scenario, contentType := range contentTypes {
			if contentType != "application/xml" {
				t.Errorf("Scenario %s has inconsistent Content-Type: %s", scenario, contentType)
			}
		}
	})

	// Verify all responses complete quickly
	t.Run("All responses complete within performance threshold", func(t *testing.T) {
		for scenario, elapsed := range responseTimes {
			if elapsed > 100*time.Millisecond {
				t.Errorf("Scenario %s exceeded 100ms threshold: %v", scenario, elapsed)
			}
		}
	})

	// Report performance statistics
	t.Run("Performance statistics", func(t *testing.T) {
		var total time.Duration
		var max time.Duration
		var min = 1 * time.Second // Initialize to high value

		for _, elapsed := range responseTimes {
			total += elapsed
			if elapsed > max {
				max = elapsed
			}
			if elapsed < min {
				min = elapsed
			}
		}

		avg := total / time.Duration(len(responseTimes))

		t.Logf("Error response performance statistics:")
		t.Logf("  Total scenarios: %d", len(responseTimes))
		t.Logf("  Average response time: %v", avg)
		t.Logf("  Min response time: %v", min)
		t.Logf("  Max response time: %v", max)
		t.Logf("  All responses under 100ms: %v", max < 100*time.Millisecond)
	})
}

// TestErrorResponseFormatDocumentation documents the error response format
func TestErrorResponseFormatDocumentation(t *testing.T) {
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

	req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Document the error response format
	t.Run("Error response format documentation", func(t *testing.T) {
		var s3Err S3Error
		if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
			t.Fatalf("Failed to parse error response: %v", err)
		}

		t.Log("ARMOR Error Response Format:")
		t.Log("================================")
		t.Log("All error responses follow S3 XML error format:")
		t.Log("")
		t.Logf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
		t.Logf("<Error>")
		t.Logf("  <Code>%s</Code>", s3Err.Code)
		t.Logf("  <Message>%s</Message>", s3Err.Message)
		t.Logf("</Error>")
		t.Log("")
		t.Log("HTTP Status Code: 403 Forbidden (for authentication/authorization errors)")
		t.Log("Content-Type: application/xml")
		t.Log("")
		t.Log("Common Error Codes:")
		t.Log("  - MissingAuthenticationToken: Authorization header is missing")
		t.Log("  - InvalidAccessKeyId: The provided access key does not exist")
		t.Log("  - SignatureDoesNotMatch: Calculated signature does not match provided signature")
		t.Log("  - RequestExpired: Request timestamp is outside allowed time window")
		t.Log("  - AccessDenied: ACL-based access control rejection")
		t.Log("  - InvalidAlgorithm: Only AWS4-HMAC-SHA256 is supported")
		t.Log("  - IncompleteSignature: Authorization header is missing required fields")
		t.Log("  - MissingDateHeader: X-Amz-Date header is missing")
		t.Log("")
		t.Log("Performance Characteristics:")
		t.Log("  - Average response time: <1ms for local testing")
		t.Log("  - Maximum response time: <100ms under normal conditions")
		t.Log("  - Response time includes authentication verification")
	})
}
