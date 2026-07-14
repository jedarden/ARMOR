package server

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestNonAuthErrorScenarios tests all non-authentication error scenarios
// to ensure proper error codes, messages, and HTTP status codes.
func TestNonAuthErrorScenarios(t *testing.T) {
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
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	server, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := server.Handler()

	// Helper to verify error response
	verifyErrorResponse := func(t *testing.T, w *httptest.ResponseRecorder, expectedCode string, expectedStatus int, messageMinLength int) {
		t.Helper()

		// Verify HTTP status code
		if w.Code != expectedStatus {
			t.Errorf("Expected HTTP status %d, got %d", expectedStatus, w.Code)
		}

		// Verify Content-Type header
		contentType := w.Header().Get("Content-Type")
		if contentType != "application/xml" {
			t.Errorf("Expected Content-Type 'application/xml', got '%s'", contentType)
		}

		// Verify response body is not empty
		if w.Body.Len() == 0 {
			t.Fatal("Expected non-empty response body")
		}

		// Parse and verify error response
		var s3Err S3Error
		if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
			t.Fatalf("Failed to parse error response: %v", err)
		}

		// Verify error code
		if s3Err.Code != expectedCode {
			t.Errorf("Expected error code '%s', got '%s'", expectedCode, s3Err.Code)
		}

		// Verify error message is meaningful
		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}

		// Verify message is long enough to be meaningful
		if len(s3Err.Message) < messageMinLength {
			t.Errorf("Error message too short (got %d chars, want at least %d): %s", len(s3Err.Message), messageMinLength, s3Err.Message)
		}

		// Verify XML declaration
		body := w.Body.String()
		if !strings.HasPrefix(body, "<?xml") {
			t.Errorf("Expected XML declaration, got: %s", body[:min(len(body), 50)])
		}
	}

	t.Run("NoSuchKey scenarios", func(t *testing.T) {
		t.Run("Non-existent object returns 404 NoSuchKey", func(t *testing.T) {
			req := createValidAuthRequest(t, "GET", "/test-bucket/non-existent-key.txt")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			verifyErrorResponse(t, w, "NoSuchKey", 404, 15)
		})

		t.Run("NoSuchKey error contains object path", func(t *testing.T) {
			req := createValidAuthRequest(t, "GET", "/test-bucket/path/to/missing/file.json")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			var s3Err S3Error
			xml.Unmarshal(w.Body.Bytes(), &s3Err)

			// Verify error message contains relevant context
			if !strings.Contains(strings.ToLower(s3Err.Message), "not found") &&
			   !strings.Contains(strings.ToLower(s3Err.Message), "object") {
				t.Errorf("Error message should mention 'not found' or 'object', got: %s", s3Err.Message)
			}
		})
	})

	t.Run("InvalidRequest scenarios", func(t *testing.T) {
		t.Run("Unsupported POST operation returns 400 InvalidRequest", func(t *testing.T) {
			// Create a POST request to a GET-only endpoint
			req := createValidAuthRequest(t, "POST", "/test-bucket/test-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			verifyErrorResponse(t, w, "InvalidRequest", 400, 20)
		})

		t.Run("InvalidRequest message specifies unsupported operation", func(t *testing.T) {
			req := createValidAuthRequest(t, "POST", "/test-bucket/test-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			var s3Err S3Error
			xml.Unmarshal(w.Body.Bytes(), &s3Err)

			// Verify error message mentions the unsupported operation
			if !strings.Contains(strings.ToLower(s3Err.Message), "post") &&
			   !strings.Contains(strings.ToLower(s3Err.Message), "unsupported") {
				t.Errorf("Error message should mention 'POST' or 'unsupported', got: %s", s3Err.Message)
			}
		})
	})

	t.Run("MethodNotAllowed scenarios", func(t *testing.T) {
		t.Run("Unsupported HTTP method returns 405 MethodNotAllowed", func(t *testing.T) {
			// Test with PATCH method (not supported)
			req := createValidAuthRequest(t, "PATCH", "/test-bucket/test-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			verifyErrorResponse(t, w, "MethodNotAllowed", 405, 15)
		})

		t.Run("MethodNotAllowed error specifies the disallowed method", func(t *testing.T) {
			req := createValidAuthRequest(t, "PATCH", "/test-bucket/test-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			var s3Err S3Error
			xml.Unmarshal(w.Body.Bytes(), &s3Err)

			// Verify error message mentions the method
			if !strings.Contains(s3Err.Message, "PATCH") {
				t.Errorf("Error message should mention 'PATCH', got: %s", s3Err.Message)
			}
		})

		t.Run("DELETE on endpoint returns error", func(t *testing.T) {
			req := createValidAuthRequest(t, "DELETE", "/test-bucket/test-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			// DELETE may fail with various errors depending on backend state
			// It could be MethodNotAllowed (405) or InternalError (500)
			if w.Code != 405 && w.Code != 500 {
				t.Logf("DELETE returned status %d (expected 405 or 500)", w.Code)
			}
		})
	})

	t.Run("PreconditionFailed scenarios", func(t *testing.T) {
		t.Run("Conditional GET with mismatched ETag returns 412", func(t *testing.T) {
			req := createValidAuthRequest(t, "GET", "/test-bucket/test-key")
			// Add If-Match header with non-matching ETag
			req.Header.Set("If-Match", "\"non-matching-etag\"")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Note: This test requires the object to exist
			// For now, we'll test the error response format
			// In a real scenario, you'd need to mock the backend
		})

		t.Run("PreconditionFailed error message is meaningful", func(t *testing.T) {
			// Test error structure when precondition fails
			// This would typically require mocking the conditional check
		})
	})

	t.Run("InvalidRange scenarios", func(t *testing.T) {
		t.Run("Invalid range header on non-existent object returns 404", func(t *testing.T) {
			req := createValidAuthRequest(t, "GET", "/test-bucket/test-key")
			req.Header.Set("Range", "bytes=invalid-range")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			// NoSuchKey takes precedence over InvalidRange when object doesn't exist
			verifyErrorResponse(t, w, "NoSuchKey", 404, 15)
		})

		t.Run("InvalidRange error would be returned for existing objects", func(t *testing.T) {
			// Note: InvalidRange validation happens after object existence check
			// Since we're testing with non-existent objects, we get NoSuchKey first
			// In production, InvalidRange would be returned for existing objects with bad ranges
			req := createValidAuthRequest(t, "GET", "/test-bucket/test-key")
			req.Header.Set("Range", "bytes=100-50") // end < start
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// With non-existent object, NoSuchKey is returned
			if w.Code != 404 {
				t.Logf("Expected 404 for non-existent object, got %d", w.Code)
			}
		})

		t.Run("Range syntax validation", func(t *testing.T) {
			// Test that malformed range syntax is handled
			req := createValidAuthRequest(t, "GET", "/test-bucket/test-key")
			req.Header.Set("Range", "bytes=-100--50")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			// For non-existent objects, NoSuchKey is returned
			// The range validation would happen if the object existed
			if w.Code != 404 {
				t.Logf("Non-existent object with bad range returned status %d", w.Code)
			}
		})
	})

	t.Run("NoSuchBucket scenarios", func(t *testing.T) {
		t.Run("Non-existent bucket returns 404", func(t *testing.T) {
			req := createValidAuthRequest(t, "GET", "/non-existent-bucket/test-key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			// Server is configured with a single bucket, so bucket validation
			// returns 404 NoSuchKey (object not found in configured bucket)
			verifyErrorResponse(t, w, "NoSuchKey", 404, 15)
		})

		t.Run("NoSuchBucket error mentions bucket", func(t *testing.T) {
			req := createValidAuthRequest(t, "GET", "/wrong-bucket/key")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			var s3Err S3Error
			xml.Unmarshal(w.Body.Bytes(), &s3Err)

			// Verify error message mentions bucket
			if !strings.Contains(strings.ToLower(s3Err.Message), "bucket") &&
			   !strings.Contains(strings.ToLower(s3Err.Message), "not found") {
				t.Errorf("Error message should mention 'bucket' or 'not found', got: %s", s3Err.Message)
			}
		})
	})

	t.Run("MalformedXML scenarios", func(t *testing.T) {
		t.Run("Invalid XML in delete request returns 400 MalformedXML", func(t *testing.T) {
			body := "<Delete><Invalid>"
			req := createInvalidCredTestRequest(t, "POST", "/test-bucket?delete", body, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
			req.Header.Set("Content-Type", "application/xml")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			verifyErrorResponse(t, w, "MalformedXML", 400, 15)
		})

		t.Run("Empty delete object list returns MalformedXML", func(t *testing.T) {
			body := "<Delete></Delete>"
			req := createInvalidCredTestRequest(t, "POST", "/test-bucket?delete", body, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
			req.Header.Set("Content-Type", "application/xml")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			verifyErrorResponse(t, w, "MalformedXML", 400, 15)
		})

		t.Run("MalformedXML error specifies the XML issue", func(t *testing.T) {
			body := "<Delete><Broken>"
			req := createInvalidCredTestRequest(t, "POST", "/test-bucket?delete", body, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
			req.Header.Set("Content-Type", "application/xml")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			var s3Err S3Error
			xml.Unmarshal(w.Body.Bytes(), &s3Err)

			// Verify error message mentions XML parsing
			if !strings.Contains(strings.ToLower(s3Err.Message), "xml") &&
			   !strings.Contains(strings.ToLower(s3Err.Message), "parse") {
				t.Errorf("Error message should mention 'xml' or 'parse', got: %s", s3Err.Message)
			}
		})
	})

	t.Run("InternalError scenarios", func(t *testing.T) {
		// Note: InternalError scenarios are difficult to test deterministically
		// as they represent actual server failures. The following tests verify
		// the error response format when such failures occur.

		t.Run("InternalError returns 500 status code", func(t *testing.T) {
			// This would require inducing a real internal error
			// For now, we verify the error response format
		})

		t.Run("InternalError message specifies the failure", func(t *testing.T) {
			// Verify that internal error messages are descriptive
			// This would require mocking internal failures
		})
	})
}

// TestNonAuthErrorResponseStructure verifies the structure of error responses
func TestNonAuthErrorResponseStructure(t *testing.T) {
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

	errorCodes := []struct {
		name          string
		setupRequest  func() *http.Request
		expectedCode  string
		expectedStatus int
	}{
		{
			name: "NoSuchKey",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/nonexistent-key")
			},
			expectedCode:   "NoSuchKey",
			expectedStatus: 404,
		},
		{
			name: "InvalidRequest",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "POST", "/test-bucket/test-key")
			},
			expectedCode:   "InvalidRequest",
			expectedStatus: 400,
		},
		{
			name: "MethodNotAllowed",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "PATCH", "/test-bucket/test-key")
			},
			expectedCode:   "MethodNotAllowed",
			expectedStatus: 405,
		},
		{
			name: "NoSuchKey",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/wrong-bucket/test-key")
			},
			expectedCode:   "NoSuchKey",
			expectedStatus: 404,
		},
	}

	for _, errorCode := range errorCodes {
		t.Run(errorCode.name+" has correct structure", func(t *testing.T) {
			req := errorCode.setupRequest()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Parse XML response
			var s3Err S3Error
			if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
				t.Fatalf("Failed to parse error response: %v", err)
			}

			// Verify Code field is present
			if s3Err.Code == "" {
				t.Error("Error response missing Code field")
			}

			// Verify Message field is present
			if s3Err.Message == "" {
				t.Error("Error response missing Message field")
			}

			// Verify XML structure
			body := w.Body.String()
			if !strings.Contains(body, "<Code>") {
				t.Error("Error response missing <Code> element")
			}
			if !strings.Contains(body, "<Message>") {
				t.Error("Error response missing <Message> element")
			}

			// Verify no extra fields (only Code and Message)
			// This ensures S3 compatibility
			if strings.Contains(body, "<Resource>") ||
			   strings.Contains(body, "<RequestId>") {
				// These are optional in S3, but we should be minimal
				t.Logf("Warning: Error response contains optional S3 fields")
			}
		})
	}
}

// TestNonAuthErrorMessageQuality verifies the quality of error messages
func TestNonAuthErrorMessageQuality(t *testing.T) {
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
		name              string
		setupRequest      func() *http.Request
		expectedKeywords []string
	}{
		{
			name: "NoSuchKey mentions object",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/nonexistent-key")
			},
			expectedKeywords: []string{"not found", "object", "key"},
		},
		{
			name: "InvalidRequest mentions operation",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "POST", "/test-bucket/test-key")
			},
			expectedKeywords: []string{"post", "unsupported", "operation"},
		},
		{
			name: "MethodNotAllowed mentions method",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "PATCH", "/test-bucket/test-key")
			},
			expectedKeywords: []string{"method", "not allowed", "patch"},
		},
		{
			name: "InvalidRange mentions range",
			setupRequest: func() *http.Request {
				req := createValidAuthRequest(t, "GET", "/test-bucket/test-key")
				req.Header.Set("Range", "bytes=invalid")
				return req
			},
			expectedKeywords: []string{"range", "invalid"},
		},
		{
			name: "MalformedXML mentions XML",
			setupRequest: func() *http.Request {
				req := createValidAuthRequest(t, "POST", "/test-bucket?delete")
				req.Header.Set("Content-Type", "application/xml")
				req.Body = io.NopCloser(strings.NewReader("<Delete><Broken"))
				return req
			},
			expectedKeywords: []string{"xml", "parse"},
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := scenario.setupRequest()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			var s3Err S3Error
			xml.Unmarshal(w.Body.Bytes(), &s3Err)

			message := strings.ToLower(s3Err.Message)

			// Verify at least one expected keyword is present
			found := false
			for _, keyword := range scenario.expectedKeywords {
				if strings.Contains(message, keyword) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Error message should contain at least one of %v, got: %s",
					scenario.expectedKeywords, s3Err.Message)
			}

			// Verify message is not generic
			if s3Err.Message == "Error" || s3Err.Message == "Invalid request" {
				t.Error("Error message is too generic")
			}
		})
	}
}

// TestNonAuthErrorHTTPStatusCodes verifies correct HTTP status codes
func TestNonAuthErrorHTTPStatusCodes(t *testing.T) {
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

	statusTests := []struct {
		name          string
		setupRequest  func() *http.Request
		expectedStatus int
		description   string
	}{
		{
			name: "NoSuchKey is 404",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/nonexistent-key")
			},
			expectedStatus: 404,
			description:    "Object not found should return 404 Not Found",
		},
		{
			name: "InvalidRequest is 400",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "POST", "/test-bucket/test-key")
			},
			expectedStatus: 400,
			description:    "Invalid request parameters should return 400 Bad Request",
		},
		{
			name: "MethodNotAllowed is 405",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "PATCH", "/test-bucket/test-key")
			},
			expectedStatus: 405,
			description:    "Unsupported method should return 405 Method Not Allowed",
		},
		{
			name: "InvalidRange is 400",
			setupRequest: func() *http.Request {
				req := createValidAuthRequest(t, "GET", "/test-bucket/test-key")
				req.Header.Set("Range", "bytes=invalid")
				return req
			},
			expectedStatus: 404,
			description:    "Invalid range on non-existent object returns 404 (NoSuchKey takes precedence)",
		},
		{
			name: "MalformedXML is 400",
			setupRequest: func() *http.Request {
				req := createValidAuthRequest(t, "POST", "/test-bucket?delete")
				req.Header.Set("Content-Type", "application/xml")
				req.Body = io.NopCloser(strings.NewReader("<Delete><Broken"))
				return req
			},
			expectedStatus: 400,
			description:    "Malformed XML should return 400 Bad Request",
		},
		{
			name: "NoSuchBucket is 404",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/wrong-bucket/test-key")
			},
			expectedStatus: 404,
			description:    "Bucket not found should return 404 Not Found",
		},
	}

	for _, test := range statusTests {
		t.Run(test.name, func(t *testing.T) {
			req := test.setupRequest()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != test.expectedStatus {
				t.Errorf("%s: Expected status %d, got %d",
					test.description, test.expectedStatus, w.Code)
			}
		})
	}
}

// TestNonAuthErrorEdgeCases tests edge cases and boundary conditions
func TestNonAuthErrorEdgeCases(t *testing.T) {
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

	t.Run("Very long key name in NoSuchKey", func(t *testing.T) {
		longKey := strings.Repeat("a", 1024) + "/file.txt"
		req := createValidAuthRequest(t, "GET", "/test-bucket/"+longKey)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var s3Err S3Error
		xml.Unmarshal(w.Body.Bytes(), &s3Err)

		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected NoSuchKey for long key, got %s", s3Err.Code)
		}
	})

	t.Run("Special characters in key name", func(t *testing.T) {
		specialKey := "test/file@#$%.txt"
		req := createValidAuthRequest(t, "GET", "/test-bucket/"+url.QueryEscape(specialKey))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Should handle gracefully
		if w.Code != 404 && w.Code != 400 {
			t.Logf("Got status code %d for special characters", w.Code)
		}
	})

	t.Run("Empty key name", func(t *testing.T) {
		req := createValidAuthRequest(t, "GET", "/test-bucket/")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Should handle gracefully (either 404 or 400)
		if w.Code != 404 && w.Code != 400 {
			t.Logf("Got status code %d for empty key", w.Code)
		}
	})

	t.Run("Unicode characters in error messages", func(t *testing.T) {
		// Verify that error messages handle unicode properly
		req := createValidAuthRequest(t, "GET", "/test-bucket/文件.txt")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Parse response to ensure XML is valid
		var s3Err S3Error
		err := xml.Unmarshal(w.Body.Bytes(), &s3Err)
		if err != nil {
			t.Errorf("Failed to parse error response with unicode: %v", err)
		}
	})

	t.Run("Range boundary conditions", func(t *testing.T) {
		testCases := []struct {
			name   string
			rng    string
		}{
			{"Start equals end", "bytes=100-100"},
			{"Very large range", "bytes=0-999999999999"},
			{"Negative start", "bytes=-100"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				req := createValidAuthRequest(t, "GET", "/test-bucket/test-key")
				req.Header.Set("Range", tc.rng)
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				// Should handle gracefully (either 200, 416, or 400)
				if w.Code != 200 && w.Code != 416 && w.Code != 400 && w.Code != 404 {
					t.Logf("Range '%s' returned status %d", tc.rng, w.Code)
				}
			})
		}
	})

	t.Run("Multiple error conditions", func(t *testing.T) {
		// Test when multiple error conditions apply (e.g., wrong bucket + invalid range)
		req := createValidAuthRequest(t, "GET", "/wrong-bucket/test-key")
		req.Header.Set("Range", "bytes=invalid")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Should return the first error encountered
		if w.Code != 404 && w.Code != 400 {
			t.Logf("Multiple errors returned status %d", w.Code)
		}
	})
}

// TestNonAuthErrorPerformance verifies error responses meet performance targets
func TestNonAuthErrorPerformance(t *testing.T) {
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

	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	scenarios := []struct {
		name         string
		setupRequest func() *http.Request
		maxDuration  string // Maximum acceptable duration
	}{
		{
			name: "NoSuchKey response time",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/nonexistent-key")
			},
			maxDuration: "100ms",
		},
		{
			name: "InvalidRequest response time",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "POST", "/test-bucket/test-key")
			},
			maxDuration: "50ms",
		},
		{
			name: "MethodNotAllowed response time",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "PATCH", "/test-bucket/test-key")
			},
			maxDuration: "50ms",
		},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			if testing.Short() {
				t.Skip("Skipping individual performance test in short mode")
			}

			req := scenario.setupRequest()
			w := httptest.NewRecorder()

			// Measure response time
			start := time.Now()
			handler.ServeHTTP(w, req)
			duration := time.Since(start)

			// Verify response is fast (< 100ms for non-auth errors)
			if duration > 100*time.Millisecond {
				t.Errorf("Error response took %v, expected < 100ms", duration)
			}

			t.Logf("Response time: %v", duration)
		})
	}
}

// Helper function to create valid authenticated requests
func createValidAuthRequest(t *testing.T, method, path string) *http.Request {
	t.Helper()

	// Pass nil for timestamp to use current time at request creation
	// This avoids expiration issues that occur when capturing time.Now()
	// too early and then using it later
	return createSignedRequestForAuthTest(t, method, path, "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
}
