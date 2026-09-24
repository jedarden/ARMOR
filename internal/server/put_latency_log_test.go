package server

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/server/middleware"
)

// TestLogCompletedRequestPutLatencyFields verifies the armor-69dd394b
// request-completed log fields: backend_put_ms, provenance_lock_wait_ms and
// provenance_write_ms appear when the PUT handlers published the split, and
// are absent otherwise (no provenance path taken).
func TestLogCompletedRequestPutLatencyFields(t *testing.T) {
	var logBuf strings.Builder
	testLogger := logging.New("armor-test")
	testLogger.SetOutput(&logBuf)

	s := &Server{
		logger: testLogger,
	}

	req := httptest.NewRequest("PUT", "/testbucket/testkey", nil)

	t.Run("fields present when split published", func(t *testing.T) {
		logBuf.Reset()
		req := req.WithContext(middleware.WithPutLatency(req.Context(), &middleware.PutLatency{
			BackendPutMs:         800,
			ProvenanceLockWaitMs: 12,
			ProvenanceWriteMs:    45,
		}))

		s.logCompletedRequest(req, time.Now(), 200, "allow", "test-key-id", "put", "testbucket/testkey", 0)

		logOutput := logBuf.String()
		for _, field := range []string{
			`"backend_put_ms":800`,
			`"provenance_lock_wait_ms":12`,
			`"provenance_write_ms":45`,
		} {
			if !strings.Contains(logOutput, field) {
				t.Errorf("log output missing required field %q\nGot: %s", field, logOutput)
			}
		}
	})

	t.Run("fields absent without split", func(t *testing.T) {
		logBuf.Reset()

		s.logCompletedRequest(req, time.Now(), 200, "allow", "test-key-id", "put", "testbucket/testkey", 0)

		logOutput := logBuf.String()
		for _, field := range []string{`"backend_put_ms"`, `"provenance_lock_wait_ms"`, `"provenance_write_ms"`} {
			if strings.Contains(logOutput, field) {
				t.Errorf("log output should not contain %q without a published split\nGot: %s", field, logOutput)
			}
		}
	})
}
