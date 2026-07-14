package server

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

func createTestRequest(method, path, accessKey, secretKey string, requestTime *time.Time) *http.Request {
	req := httptest.NewRequest(method, path, nil)

	if requestTime == nil {
		requestTime = &[]time.Time{time.Now().UTC()}[0]
	}

	amzDate := requestTime.Format("20060102T150405Z")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

	if accessKey != "" && secretKey != "" {
		// Create a simple auth header for testing
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=dummy123",
			accessKey, amzDate[:8])
		req.Header.Set("Authorization", authHeader)
	}

	return req
}

func createMalformedSigRequestForDoc(method, path, accessKey, signature string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	amzDate := time.Now().UTC().Format("20060102T150405Z")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/20130524/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=%s",
		accessKey, signature)
	req.Header.Set("Authorization", authHeader)

	return req
}

// TestAuthRejectionHeadersDocumentation generates documentation for authentication rejection headers
func TestAuthRejectionHeadersDocumentation(t *testing.T) {
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

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := srv.Handler()

	// Define all authentication rejection scenarios
	scenarios := []struct {
		name         string
		setupRequest func() *http.Request
		expectedCode string
	}{
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
				return createTestRequest("GET", "/test-bucket/test-key", "INVALIDKEY", "TESTSECRETKEY123456789012345678901234", nil)
			},
			expectedCode: "InvalidAccessKeyId",
		},
		{
			name: "SignatureDoesNotMatch",
			setupRequest: func() *http.Request {
				return createTestRequest("GET", "/test-bucket/test-key", "TESTACCESSKEY", "WRONGSECRETKEY123456789012345678901", nil)
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
			name: "MissingDateHeader",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
				req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=example")
				return req
			},
			expectedCode: "MissingDateHeader",
		},
		{
			name: "RequestExpired",
			setupRequest: func() *http.Request {
				oldTime := time.Now().Add(-20 * time.Minute)
				return createTestRequest("GET", "/test-bucket/test-key", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", &oldTime)
			},
			expectedCode: "RequestExpired",
		},
		{
			name: "IncompleteSignature",
			setupRequest: func() *http.Request {
				return createMalformedSigRequestForDoc("GET", "/test-bucket/test-key", "TESTACCESSKEY", "")
			},
			expectedCode: "IncompleteSignature",
		},
	}

	t.Log("# ARMOR Authentication Rejection Response Headers")
	t.Log("")
	t.Log("This document documents the exact response headers returned for all authentication rejection scenarios.")
	t.Log("")

	for _, scenario := range scenarios {
		req := scenario.setupRequest()
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Parse error response
		var s3Err S3Error
		xml.Unmarshal(w.Body.Bytes(), &s3Err)

		t.Logf("## %s", scenario.name)
		t.Log("")

		// Document HTTP status code
		t.Logf("**HTTP Status Code:** `%d`", w.Code)
		t.Log("")

		// Document response headers
		t.Log("**Response Headers:**")
		t.Log("```")
		for key, values := range w.Header() {
			for _, value := range values {
				t.Logf("%s: %s", key, value)
			}
		}
		t.Log("```")
		t.Log("")

		// Document response body (formatted)
		t.Log("**Response Body:**")
		t.Log("```xml")
		body := w.Body.String()
		if strings.HasPrefix(body, "<?xml") {
			t.Log(body)
		} else {
			t.Logf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>")
			t.Logf("%s", body)
		}
		t.Log("```")
		t.Log("")

		// Document error details
		t.Log("**Error Details:**")
		t.Logf("- **Error Code:** `%s`", s3Err.Code)
		t.Logf("- **Error Message:** %s", s3Err.Message)
		t.Log("")

		t.Log("---")
		t.Log("")
	}
}
