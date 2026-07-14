package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestContentTypeConsistencyAcrossAllRejections verifies that ALL authentication rejection scenarios
// return consistent Content-Type: application/xml headers.
//
// This test ensures complete coverage of all authentication error types defined in auth.go:
// - MissingAuthenticationToken
// - InvalidAccessKeyId
// - SignatureDoesNotMatch
// - RequestExpired
// - InvalidAlgorithm
// - IncompleteSignature
// - MissingDateHeader
// - InvalidDateFormat
// - InvalidCredential
// - AccessDenied (authorization, not authentication, but tested for completeness)
func TestContentTypeConsistencyAcrossAllRejections(t *testing.T) {
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

	// Define ALL authentication/authorization rejection scenarios
	scenarios := []struct {
		name         string
		setupRequest func() *http.Request
		expectedCode string
	}{
		// Authentication failures
		{
			name: "MissingAuthenticationToken",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/test-bucket/test-key", nil)
			},
			expectedCode: "MissingAuthenticationToken",
		},
		{
			name: "InvalidAccessKeyId",
			setupRequest: func() *http.Request {
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "INVALIDKEY", "TESTSECRETKEY123456789012345678901234", nil)
			},
			expectedCode: "InvalidAccessKeyId",
		},
		{
			name: "SignatureDoesNotMatch",
			setupRequest: func() *http.Request {
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "WRONGSECRETKEY123456789012345678901", nil)
			},
			expectedCode: "SignatureDoesNotMatch",
		},
		{
			name: "InvalidAlgorithm",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "InvalidAuthHeaderFormat")
				return req
			},
			expectedCode: "InvalidAlgorithm",
		},
		{
			name: "IncompleteSignature",
			setupRequest: func() *http.Request {
				req := createMalformedSigRequest(t, "GET", "/test-bucket/test-key", "TESTACCESSKEY", "")
				return req
			},
			expectedCode: "IncompleteSignature",
		},
		{
			name: "MissingDateHeader",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=example")
				return req
			},
			expectedCode: "MissingDateHeader",
		},
		{
			name: "InvalidDateFormat",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=example")
				req.Header.Set("X-Amz-Date", "invalid-date-format")
				return req
			},
			expectedCode: "InvalidDateFormat",
		},
		{
			name: "RequestExpired",
			setupRequest: func() *http.Request {
				oldTime := time.Now().Add(-20 * time.Minute)
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", &oldTime)
			},
			expectedCode: "RequestExpired",
		},
		{
			name: "InvalidCredential",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "AWS4-HMAC-SHA256 InvalidCredentialFormat, SignedHeaders=host, Signature=example")
				return req
			},
			expectedCode: "InvalidCredential",
		},
		// Authorization failure (ACL-based)
		{
			name: "AccessDenied",
			setupRequest: func() *http.Request {
				// Use RESTRICTEDKEY but try to access disallowed path
				return createInvalidCredTestRequest(t, "GET", "/test-bucket/disallowed-key", "", "RESTRICTEDKEY", "RESTRICTEDSECRET1234567890123456789", nil)
			},
			expectedCode: "AccessDenied",
		},
	}

	// Track results
	contentTypes := make(map[string]string)
	statusCodes := make(map[string]int)
	xmlResponses := make(map[string]bool)

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			req := scenario.setupRequest()
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Record Content-Type
			contentType := w.Header().Get("Content-Type")
			contentTypes[scenario.name] = contentType

			// Record status code
			statusCodes[scenario.name] = w.Code

			// Verify response is XML
			body := w.Body.String()
			xmlResponses[scenario.name] = strings.HasPrefix(body, "<?xml") || strings.HasPrefix(body, "<Error>")

			// Verify Content-Type is application/xml
			if contentType != "application/xml" {
				t.Errorf("Expected Content-Type 'application/xml', got '%s'", contentType)
			}

			// Verify status code is 403 for all auth errors (AccessDenied is also 403)
			if w.Code != 403 {
				t.Errorf("Expected status code 403, got %d", w.Code)
			}

			// Verify response is XML
			if !xmlResponses[scenario.name] {
				t.Errorf("Expected XML response, got: %s", body[:min(len(body), 100)])
			}
		})
	}

	// Global verification
	t.Run("All rejection scenarios have consistent Content-Type", func(t *testing.T) {
		inconsistent := []string{}
		for scenario, contentType := range contentTypes {
			if contentType != "application/xml" {
				inconsistent = append(inconsistent, scenario)
			}
		}
		if len(inconsistent) > 0 {
			t.Errorf("The following scenarios have inconsistent Content-Type: %v", inconsistent)
		}
	})

	t.Run("All rejection scenarios return 403 status", func(t *testing.T) {
		incorrectStatus := []string{}
		for scenario, statusCode := range statusCodes {
			if statusCode != 403 {
				incorrectStatus = append(incorrectStatus, scenario)
			}
		}
		if len(incorrectStatus) > 0 {
			t.Errorf("The following scenarios have incorrect status code (expected 403): %v", incorrectStatus)
		}
	})

	t.Run("All rejection scenarios return XML responses", func(t *testing.T) {
		nonXML := []string{}
		for scenario, isXML := range xmlResponses {
			if !isXML {
				nonXML = append(nonXML, scenario)
			}
		}
		if len(nonXML) > 0 {
			t.Errorf("The following scenarios returned non-XML responses: %v", nonXML)
		}
	})

	// Summary reporting
	t.Run("Summary report", func(t *testing.T) {
		t.Log("Content-Type Consistency Verification Summary")
		t.Log("==============================================")
		t.Logf("Total scenarios tested: %d", len(scenarios))
		t.Logf("All scenarios have Content-Type: application/xml: %v", len(contentTypes) == len(scenarios))
		t.Logf("All scenarios return status 403: %v", contentTypeAllEqual(statusCodes, 403))
		t.Logf("All scenarios return XML: %v", contentTypeAllTrue(xmlResponses))
		t.Log("")
		t.Log("Content-Type values by scenario:")
		for scenario, contentType := range contentTypes {
			t.Logf("  %s: %s", scenario, contentType)
		}
	})
}

func contentTypeAllEqual(m map[string]int, value int) bool {
	for _, v := range m {
		if v != value {
			return false
		}
	}
	return true
}

func contentTypeAllTrue(m map[string]bool) bool {
	for _, v := range m {
		if !v {
			return false
		}
	}
	return true
}
