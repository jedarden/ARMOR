package server

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// Helper function for safe string length checking (local to avoid conflicts)
func safeMin(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestBasicErrorResponseStructure tests the fundamental structure of all error responses
// to ensure they have proper Code and Message fields with correct HTTP status codes.
//
// This test file provides foundational validation of error response format and status codes,
// serving as a baseline for more detailed error message content testing in other files.
//
// Bead: bf-5wwu1c
// Created: 2026-07-15

// assertErrorResponseStructure verifies that an error response has the correct structure
func assertErrorResponseStructure(t *testing.T, w *httptest.ResponseRecorder, expectedCode string, expectedStatus int) {
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
		t.Fatalf("Failed to parse error response XML: %v\nResponse body: %s", err, w.Body.String())
	}

	// Verify Code field is present and not empty
	if s3Err.Code == "" {
		t.Error("Error response missing Code field or Code is empty")
	}

	// Verify Code matches expected value
	if s3Err.Code != expectedCode {
		t.Errorf("Expected error code '%s', got '%s'", expectedCode, s3Err.Code)
	}

	// Verify Message field is present and not empty
	if s3Err.Message == "" {
		t.Error("Error response missing Message field or Message is empty")
	}

	// Verify XML structure
	body := w.Body.String()
	if !strings.Contains(body, "<Code>") {
		t.Error("Error response missing <Code> element")
	}
	if !strings.Contains(body, "<Message>") {
		t.Error("Error response missing <Message> element")
	}

	// Verify XML declaration
	if !strings.HasPrefix(body, "<?xml") {
		t.Errorf("Expected XML declaration, got: %s", body[:safeMin(len(body), 50)])
	}
}

// TestAllErrorResponseCodesHaveCorrectStructure tests that all error codes return properly structured responses
func TestAllErrorResponseCodesHaveCorrectStructure(t *testing.T) {
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

	// Test cases for each error code type
	testCases := []struct {
		name          string
		setupRequest  func() *http.Request
		expectedCode  string
		expectedStatus int
		description   string
	}{
		// 400 Bad Request errors
		{
			name: "InvalidRequest - unsupported operation",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "POST", "/test-bucket/test-key")
			},
			expectedCode:   "InvalidRequest",
			expectedStatus: 400,
			description:   "Invalid request parameters should return 400 Bad Request",
		},
		{
			name: "MalformedXML - invalid XML in delete request",
			setupRequest: func() *http.Request {
				return createInvalidCredTestRequest(t, "POST", "/test-bucket?delete", "<Delete><Invalid>", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
			},
			expectedCode:   "MalformedXML",
			expectedStatus: 400,
			description:   "Malformed XML should return 400 Bad Request",
		},

		// 403 Forbidden errors
		{
			name: "AccessDenied - .armor/ namespace access",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/.armor/restricted-file")
			},
			expectedCode:   "AccessDenied",
			expectedStatus: 403,
			description:   "Access to .armor/ namespace should return 403 Forbidden",
		},

		// 404 Not Found errors
		{
			name: "NoSuchKey - non-existent object",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/nonexistent-key.txt")
			},
			expectedCode:   "NoSuchKey",
			expectedStatus: 404,
			description:   "Object not found should return 404 Not Found",
		},
		{
			name: "NoSuchBucket - non-existent bucket",
			setupRequest: func() *http.Request {
				// Use HEAD bucket operation to trigger NoSuchBucket
				req := createValidAuthRequest(t, "HEAD", "/wrong-bucket")
				return req
			},
			expectedCode:   "NoSuchBucket",
			expectedStatus: 404,
			description:   "Bucket not found should return 404 Not Found",
		},

		// 405 Method Not Allowed errors
		{
			name: "MethodNotAllowed - unsupported HTTP method",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "PATCH", "/test-bucket/test-key")
			},
			expectedCode:   "MethodNotAllowed",
			expectedStatus: 405,
			description:   "Unsupported method should return 405 Method Not Allowed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.setupRequest()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			t.Logf("Testing: %s", tc.description)
			assertErrorResponseStructure(t, w, tc.expectedCode, tc.expectedStatus)
		})
	}
}

// TestHTTPStatusCodesForErrorTypes verifies that all error types return correct HTTP status codes
func TestHTTPStatusCodesForErrorTypes(t *testing.T) {
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

	// Map of HTTP status codes to error codes that should use them
	statusCodeTests := map[int][]struct {
		name         string
		setupRequest func() *http.Request
		expectedCode string
	}{
		400: {
			{
				name: "InvalidRequest",
				setupRequest: func() *http.Request {
					return createValidAuthRequest(t, "POST", "/test-bucket/test-key")
				},
				expectedCode: "InvalidRequest",
			},
			{
				name: "MalformedXML",
				setupRequest: func() *http.Request {
					return createInvalidCredTestRequest(t, "POST", "/test-bucket?delete", "<Delete><Invalid>", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
				},
				expectedCode: "MalformedXML",
			},
		},
		403: {
			{
				name: "AccessDenied",
				setupRequest: func() *http.Request {
					return createValidAuthRequest(t, "GET", "/test-bucket/.armor/restricted")
				},
				expectedCode: "AccessDenied",
			},
		},
		404: {
			{
				name: "NoSuchKey",
				setupRequest: func() *http.Request {
					return createValidAuthRequest(t, "GET", "/test-bucket/nonexistent-key")
				},
				expectedCode: "NoSuchKey",
			},
			{
				name: "NoSuchBucket",
				setupRequest: func() *http.Request {
					return createValidAuthRequest(t, "HEAD", "/wrong-bucket")
				},
				expectedCode: "NoSuchBucket",
			},
		},
		405: {
			{
				name: "MethodNotAllowed",
				setupRequest: func() *http.Request {
					return createValidAuthRequest(t, "PATCH", "/test-bucket/key")
				},
				expectedCode: "MethodNotAllowed",
			},
		},
	}

	for expectedStatus, testCases := range statusCodeTests {
		t.Run(http.StatusText(expectedStatus), func(t *testing.T) {
			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					req := tc.setupRequest()
					w := httptest.NewRecorder()
					handler.ServeHTTP(w, req)

					if w.Code != expectedStatus {
						t.Errorf("Expected status %d (%s), got %d (%s)",
							expectedStatus, http.StatusText(expectedStatus),
							w.Code, http.StatusText(w.Code))
					}

					// Verify the error code matches expectations
					var s3Err S3Error
					if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
						t.Fatalf("Failed to parse error response: %v", err)
					}

					if s3Err.Code != tc.expectedCode {
						t.Errorf("Expected error code '%s', got '%s'", tc.expectedCode, s3Err.Code)
					}
				})
			}
		})
	}
}

// TestErrorResponsesHaveRequiredFields tests that all error responses contain required Code and Message fields
func TestErrorResponsesHaveRequiredFields(t *testing.T) {
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

	errorScenarios := []struct {
		name         string
		setupRequest func() *http.Request
	}{
		{
			name: "NoSuchKey error",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/missing-key")
			},
		},
		{
			name: "InvalidRequest error",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "POST", "/test-bucket/key")
			},
		},
		{
			name: "MethodNotAllowed error",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "PATCH", "/test-bucket/key")
			},
		},
		{
			name: "AccessDenied error",
			setupRequest: func() *http.Request {
				return createValidAuthRequest(t, "GET", "/test-bucket/.armor/file")
			},
		},
	}

	for _, scenario := range errorScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := scenario.setupRequest()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			// Parse the XML response
			var s3Err S3Error
			if err := xml.Unmarshal(w.Body.Bytes(), &s3Err); err != nil {
				t.Fatalf("Failed to parse error response XML: %v", err)
			}

			// Verify Code field exists and is not empty
			if s3Err.Code == "" {
				t.Error("Error response is missing Code field or Code is empty")
			}

			// Verify Message field exists and is not empty
			if s3Err.Message == "" {
				t.Error("Error response is missing Message field or Message is empty")
			}

			// Verify both fields are present in the raw XML
			body := w.Body.String()
			if !strings.Contains(body, "<Code>") {
				t.Error("XML response missing <Code> element")
			}
			if !strings.Contains(body, "<Message>") {
				t.Error("XML response missing <Message> element")
			}
		})
	}
}

// TestErrorResponseXMLStructure tests that error responses are well-formed XML
func TestErrorResponseXMLStructure(t *testing.T) {
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

	t.Run("XML declaration is present", func(t *testing.T) {
		req := createValidAuthRequest(t, "GET", "/test-bucket/missing-key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		body := w.Body.String()
		if !strings.HasPrefix(body, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
			t.Errorf("Expected XML declaration, got: %s", body[:safeMin(len(body), 50)])
		}
	})

	t.Run("XML has Error root element", func(t *testing.T) {
		req := createValidAuthRequest(t, "GET", "/test-bucket/missing-key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		body := w.Body.String()
		if !strings.Contains(body, "<Error>") {
			t.Error("Expected <Error> root element")
		}
		if !strings.Contains(body, "</Error>") {
			t.Error("Expected closing </Error> tag")
		}
	})

	t.Run("XML is parseable", func(t *testing.T) {
		req := createValidAuthRequest(t, "POST", "/test-bucket/key")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		var s3Err S3Error
		err := xml.Unmarshal(w.Body.Bytes(), &s3Err)
		if err != nil {
			t.Errorf("Failed to parse error response XML: %v", err)
		}
	})
}
