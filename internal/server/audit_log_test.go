package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/logging"
)

// TestAuditLogFields verifies that the audit logging captures all required
// ADR-012 fields: access_key_id, verb, key, and authz_result.
func TestAuditLogFields(t *testing.T) {
	// Create a test logger that captures log output
	var logBuf strings.Builder
	testLogger := logging.New("armor-test")
	testLogger.SetOutput(&logBuf)

	// Create a minimal server for testing
	s := &Server{
		logger: testLogger,
	}

	// Create a test request
	req := httptest.NewRequest("GET", "/testbucket/testkey", nil)
	req.Header.Set("Range", "bytes=0-1023")

	// Test allowed request with full identity
	t.Run("allowed request with identity", func(t *testing.T) {
		logBuf.Reset()
		start := time.Now()

		s.logCompletedRequest(req, start, 200, "allow", "test-key-id", "get", "testbucket/testkey")

		logOutput := logBuf.String()

		// Verify all required fields are present (JSON format uses colons)
		requiredFields := []string{
			`"method":"GET"`,
			`"path":"/testbucket/testkey"`,
			`"status":200`,
			`"authz_result":"allow"`,
			`"access_key_id":"test-key-id"`,
			`"verb":"get"`,
			`"key":"testbucket/testkey"`,
			`"range":"bytes=0-1023"`,
			`"duration_ms":`,
		}

		for _, field := range requiredFields {
			if !strings.Contains(logOutput, field) {
				t.Errorf("log output missing required field %q\nGot: %s", field, logOutput)
			}
		}

		// Verify it's logged at Info level (not Warn)
		if !strings.Contains(logOutput, `"level":"INFO"`) {
			t.Errorf("allowed request should log at info level, got: %s", logOutput)
		}
	})

	// Test denied auth request
	t.Run("denied auth request", func(t *testing.T) {
		logBuf.Reset()
		start := time.Now()

		s.logCompletedRequest(req, start, 403, "deny-auth", "", "", "")

		logOutput := logBuf.String()

		// Verify denial is logged with authz_result but no identity fields
		if !strings.Contains(logOutput, `"authz_result":"deny-auth"`) {
			t.Errorf("deny-auth log should contain authz_result=deny-auth, got: %s", logOutput)
		}

		// Verify identity fields are absent for failed auth
		if strings.Contains(logOutput, `"access_key_id":`) {
			t.Errorf("deny-auth log should not contain access_key_id, got: %s", logOutput)
		}

		// Verify it's logged at Warn level
		if !strings.Contains(logOutput, `"level":"WARN"`) {
			t.Errorf("denied request should log at warn level, got: %s", logOutput)
		}
	})

	// Test denied ACL request
	t.Run("denied ACL request", func(t *testing.T) {
		logBuf.Reset()
		start := time.Now()

		s.logCompletedRequest(req, start, 403, "deny-acl", "test-key-id", "get", "restricted/key")

		logOutput := logBuf.String()

		// Verify denial is logged with full identity
		requiredFields := []string{
			`"authz_result":"deny-acl"`,
			`"access_key_id":"test-key-id"`,
			`"verb":"get"`,
			`"key":"restricted/key"`,
		}

		for _, field := range requiredFields {
			if !strings.Contains(logOutput, field) {
				t.Errorf("deny-acl log missing field %q\nGot: %s", field, logOutput)
			}
		}

		// Verify it's logged at Warn level
		if !strings.Contains(logOutput, `"level":"WARN"`) {
			t.Errorf("denied request should log at warn level, got: %s", logOutput)
		}
	})

	// Test request without range header
	t.Run("request without range", func(t *testing.T) {
		reqNoRange := httptest.NewRequest("PUT", "/testbucket/anotherkey", nil)
		logBuf.Reset()
		start := time.Now()

		s.logCompletedRequest(reqNoRange, start, 200, "allow", "test-key-id", "put", "testbucket/anotherkey")

		logOutput := logBuf.String()

		// Verify range is NOT present
		if strings.Contains(logOutput, `"range":`) {
			t.Errorf("log without range header should not contain range field, got: %s", logOutput)
		}

		// Verify other fields are present
		requiredFields := []string{
			`"method":"PUT"`,
			`"verb":"put"`,
			`"key":"testbucket/anotherkey"`,
		}

		for _, field := range requiredFields {
			if !strings.Contains(logOutput, field) {
				t.Errorf("log output missing required field %q\nGot: %s", field, logOutput)
			}
		}
	})
}

// TestAuditLogPublicPaths verifies that public paths (healthz, readyz) log
// with minimal fields and no authz_result (no identity is established).
func TestAuditLogPublicPaths(t *testing.T) {
	var logBuf strings.Builder
	testLogger := logging.New("armor-test")
	testLogger.SetOutput(&logBuf)

	s := &Server{
		logger: testLogger,
	}

	req := httptest.NewRequest("GET", "/healthz", nil)
	logBuf.Reset()
	start := time.Now()

	s.logCompletedRequest(req, start, 200, "", "", "", "")

	logOutput := logBuf.String()

	// Public paths should log method/path/status/duration but no identity
	if !strings.Contains(logOutput, `"method":"GET"`) {
		t.Errorf("healthz log should contain method, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"path":"/healthz"`) {
		t.Errorf("healthz log should contain path, got: %s", logOutput)
	}
	if !strings.Contains(logOutput, `"status":200`) {
		t.Errorf("healthz log should contain status, got: %s", logOutput)
	}

	// Should NOT contain identity fields (no auth for public paths)
	identityFields := []string{"access_key_id", "verb", "key"}
	for _, field := range identityFields {
		if strings.Contains(logOutput, `"`+field+`":`) {
			t.Errorf("public path log should not contain %s field, got: %s", field, logOutput)
		}
	}
}
