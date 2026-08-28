package server

import (
	"bytes"
	"encoding/xml"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/server/middleware"
)

// TestS3ErrorLoggingInvalidPartSize verifies that InvalidPartSize errors
// emit structured log lines with all required fields: event, error_code,
// operation, bucket, key, access_key_id, request_id, status, and message.
// The log level should be WARN for 4xx errors.
func TestS3ErrorLoggingInvalidPartSize(t *testing.T) {
	// Create a test logger that captures log output
	var logBuf strings.Builder
	testLogger := logging.New("armor-test")
	testLogger.SetOutput(&logBuf)

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
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	// Create server with test logger
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	s.logger = testLogger

	// Create a multipart upload
	bucket, key := "test-bucket", "test-multipart.dat"
	uploadID := "test-upload-id"

	// Create UploadPart request with an empty first part (triggers InvalidPartSize)
	// This error occurs when part 1 is empty, which prevents establishing uniform part size
	body := bytes.NewReader([]byte{}) // Empty part

	req := httptest.NewRequest("PUT", "/"+bucket+"/"+key+"?uploadId="+uploadID+"&partNumber=1", body)
	req.Header.Set("Content-Type", "application/octet-stream")
	// Sign the request
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20260828/us-east-005/s3/aws4_request, SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date, Signature=test")

	// We need to actually set up the multipart state file for this to work
	// For now, let's test the server.go writeError path directly which is simpler
	// and exercises the same logging code

	t.Run("server writeError logs all fields", func(t *testing.T) {
		logBuf.Reset()

		req := httptest.NewRequest("PUT", "/test-bucket/test-key", nil)
		// Add request ID to context (simulating middleware)
		req = req.WithContext(middleware.WithRequestID(req.Context(), "test-request-id-123"))
		req = req.WithContext(middleware.WithExtendedID(req.Context(), "test-ext-id-456"))
		// Add credential to context
		req = req.WithContext(WithCredential(req.Context(), credentials["TESTACCESSKEY"]))

		w := httptest.NewRecorder()

		// Trigger an error using server.writeError
		s.writeError(w, req, "InvalidPartSize", "Part size 100 is not a multiple of the block size (65536 bytes)", 400)

		// Verify response was written
		if w.Code != 400 {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		// Check XML response
		var errResp struct {
			Code      string `xml:"Code"`
			Message   string `xml:"Message"`
			RequestID string `xml:"RequestId"`
		}
		if err := xml.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
			t.Fatalf("failed to unmarshal error response: %v", err)
		}

		if errResp.Code != "InvalidPartSize" {
			t.Errorf("expected error code InvalidPartSize, got %s", errResp.Code)
		}

		// Verify log output contains all required fields
		logOutput := logBuf.String()

		requiredFields := []string{
			`"event":"s3_error"`,
			`"error_code":"InvalidPartSize"`,
			`"operation":"put"`,
			`"bucket":"test-bucket"`,
			`"key":"test-key"`,
			`"request_id":"test-request-id-123"`,
			`"status":400`,
			`"access_key_id":"TESTACCESSKEY"`,
		}

		for _, field := range requiredFields {
			if !strings.Contains(logOutput, field) {
				t.Errorf("log output missing required field %q\nGot: %s", field, logOutput)
			}
		}

		// Verify it's logged at WARN level (4xx errors)
		if !strings.Contains(logOutput, `"level":"WARN"`) {
			t.Errorf("4xx error should log at WARN level, got: %s", logOutput)
		}

		// Verify the message is present (may be truncated)
		if !strings.Contains(logOutput, "Part size") || !strings.Contains(logOutput, "block size") {
			t.Errorf("log output missing error message, got: %s", logOutput)
		}

		// Verify no secret material is in the log
		secretKey := "TESTSECRETKEY123456789012345678901234"
		if strings.Contains(logOutput, secretKey) {
			t.Errorf("log output contains secret key (SECURITY VIOLATION): %s", logOutput)
		}
	})

	t.Run("server writeError with 5xx logs at ERROR level", func(t *testing.T) {
		logBuf.Reset()

		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		req = req.WithContext(middleware.WithRequestID(req.Context(), "test-request-id-789"))
		req = req.WithContext(middleware.WithExtendedID(req.Context(), "test-ext-id-012"))
		req = req.WithContext(WithCredential(req.Context(), credentials["TESTACCESSKEY"]))

		w := httptest.NewRecorder()

		// Trigger a 500 error
		s.writeError(w, req, "InternalError", "Failed to process request", 500)

		logOutput := logBuf.String()

		// Verify it's logged at ERROR level (5xx errors)
		if !strings.Contains(logOutput, `"level":"ERROR"`) {
			t.Errorf("5xx error should log at ERROR level, got: %s", logOutput)
		}

		// Verify event is still s3_error
		if !strings.Contains(logOutput, `"event":"s3_error"`) {
			t.Errorf("5xx error should have event=s3_error, got: %s", logOutput)
		}
	})

	t.Run("server writeError without credential omits access_key_id", func(t *testing.T) {
		logBuf.Reset()

		req := httptest.NewRequest("GET", "/public-bucket/public-key", nil)
		req = req.WithContext(middleware.WithRequestID(req.Context(), "public-request-id"))
		req = req.WithContext(middleware.WithExtendedID(req.Context(), "public-ext-id"))
		// No credential in context

		w := httptest.NewRecorder()

		s.writeError(w, req, "AccessDenied", "Missing credentials", 403)

		logOutput := logBuf.String()

		// Verify access_key_id is NOT present
		if strings.Contains(logOutput, `"access_key_id":`) {
			t.Errorf("log without credential should not contain access_key_id field, got: %s", logOutput)
		}

		// But other fields should still be present
		requiredFields := []string{
			`"event":"s3_error"`,
			`"error_code":"AccessDenied"`,
			`"request_id":"public-request-id"`,
		}

		for _, field := range requiredFields {
			if !strings.Contains(logOutput, field) {
				t.Errorf("log output missing required field %q\nGot: %s", field, logOutput)
			}
		}
	})
}

// TestS3ErrorLoggingNoSecretMaterial verifies that secret material
// (secret keys, MEK, wrapped DEKs) never appears in S3 error logs.
func TestS3ErrorLoggingNoSecretMaterial(t *testing.T) {
	var logBuf strings.Builder
	testLogger := logging.New("armor-test")
	testLogger.SetOutput(&logBuf)

	credentials := map[string]*config.Credential{
		"SECRETKEY": {
			AccessKey: "SECRETKEY",
			SecretKey: "super-secret-value-that-must-not-appear-in-logs",
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

	s, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}
	s.logger = testLogger

	req := httptest.NewRequest("PUT", "/test-bucket/sensitive-key", bytes.NewReader([]byte("test data")))
	req = req.WithContext(middleware.WithRequestID(req.Context(), "secret-test-id"))
	req = req.WithContext(middleware.WithExtendedID(req.Context(), "secret-ext-id"))
	req = req.WithContext(WithCredential(req.Context(), credentials["SECRETKEY"]))

	w := httptest.NewRecorder()

	s.writeError(w, req, "AccessDenied", "Unauthorized", 403)

	logOutput := logBuf.String()

	// Verify secret key is NOT in the log
	if strings.Contains(logOutput, "super-secret-value-that-must-not-appear-in-logs") {
		t.Errorf("SECURITY VIOLATION: log contains secret key value: %s", logOutput)
	}

	// Verify access_key_id is present (this is the identifier, not the secret)
	if !strings.Contains(logOutput, `"access_key_id":"SECRETKEY"`) {
		t.Errorf("log should contain access_key_id identifier, got: %s", logOutput)
	}
}
