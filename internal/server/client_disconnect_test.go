package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/metrics"
)

const clientDisconnectTestAccessKey = "TESTACCESSKEY"

type closedClientResponseWriter struct {
	*httptest.ResponseRecorder
	writes int
}

func (w *closedClientResponseWriter) Write([]byte) (int, error) {
	w.writes++
	return 0, net.ErrClosed
}

func newClientDisconnectTestServer(t *testing.T) (*Server, *strings.Builder, string) {
	t.Helper()
	secretKey := strings.Repeat("s", 32)
	cfg := &config.Config{
		Backend:   "filesystem",
		FSPath:    t.TempDir(),
		Bucket:    "test-bucket",
		B2Region:  "us-east-005",
		MEK:       make([]byte, 32),
		BlockSize: 65536,
		Credentials: map[string]*config.Credential{
			clientDisconnectTestAccessKey: {
				AccessKey: clientDisconnectTestAccessKey,
				SecretKey: secretKey,
			},
		},
	}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("create ARMOR test server: %v", err)
	}
	s.metrics = metrics.NewMetrics()
	s.requestTracker = &metrics.RequestTracker{}
	var logOutput strings.Builder
	s.logger.SetOutput(&logOutput)
	return s, &logOutput, secretKey
}

func clientDisconnectTestRequest(t *testing.T, secretKey string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://armor.test/test-bucket/object", nil)
	signRequest(t, req, nil, clientDisconnectTestAccessKey, secretKey, time.Now().UTC())
	return req
}

func clientDisconnectCompletionLog(t *testing.T, output string) map[string]any {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("decode access log: %v\n%s", err, output)
		}
		if entry["msg"] == "request completed" {
			fields, ok := entry["Fields"].(map[string]any)
			if !ok {
				t.Fatalf("request completed log has no fields: %s", line)
			}
			fields["level"] = entry["level"]
			return fields
		}
	}
	t.Fatalf("no request completed access log: %s", output)
	return nil
}

func assertClientDisconnectResult(t *testing.T, s *Server, logOutput string) {
	t.Helper()
	entry := clientDisconnectCompletionLog(t, logOutput)
	if got := entry["level"]; got != "WARN" {
		t.Errorf("access log level = %v, want WARN", got)
	}
	if got := entry["status"]; got != float64(499) {
		t.Errorf("access log status = %v, want 499", got)
	}
	if got := entry["outcome"]; got != "client_disconnect" {
		t.Errorf("access log outcome = %v, want client_disconnect", got)
	}
	if got := s.metrics.ClientDisconnectsTotal.Get("GetObject"); got != 1 {
		t.Errorf("client disconnect count = %d, want 1", got)
	}
	exposition := s.metrics.PrometheusFormat()
	if !strings.Contains(exposition, "armor_client_disconnects_total{operation=\"GetObject\"} 1") {
		t.Errorf("client disconnect sample missing from metrics exposition")
	}
	if strings.Contains(exposition, "armor_errors_total{") {
		t.Errorf("client disconnect was exported as an S3 error")
	}
}

func TestClientDisconnectResponseWriterClosed(t *testing.T) {
	s, logs, secretKey := newClientDisconnectTestServer(t)
	handler := s.Handler()
	body := []byte("encrypted object served by the real GET handler")
	put := httptest.NewRequest(http.MethodPut, "http://armor.test/test-bucket/object", strings.NewReader(string(body)))
	signRequest(t, put, body, clientDisconnectTestAccessKey, secretKey, time.Now().UTC())
	seed := httptest.NewRecorder()
	handler.ServeHTTP(seed, put)
	if seed.Code != http.StatusOK {
		t.Fatalf("seed PUT status = %d, want 200; body: %s", seed.Code, seed.Body.String())
	}
	logs.Reset()

	req := clientDisconnectTestRequest(t, secretKey)
	writer := &closedClientResponseWriter{ResponseRecorder: httptest.NewRecorder()}
	handler.ServeHTTP(writer, req)
	if writer.writes == 0 {
		t.Fatal("GET handler did not write the response body")
	}

	assertClientDisconnectResult(t, s, logs.String())
}

func TestClientDisconnectPUTCountedOnce(t *testing.T) {
	s, logs, secretKey := newClientDisconnectTestServer(t)
	fullBody := []byte("client announced this whole body but sent only a prefix")
	req := httptest.NewRequest(http.MethodPut, "http://armor.test/test-bucket/object", strings.NewReader(string(fullBody)))
	signRequest(t, req, fullBody, clientDisconnectTestAccessKey, secretKey, time.Now().UTC())
	reader, writer := io.Pipe()
	req.Body = reader
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = writer.Write(fullBody[:8])
		_ = writer.CloseWithError(io.ErrUnexpectedEOF)
	}()
	response := httptest.NewRecorder()
	s.Handler().ServeHTTP(response, req)
	<-done
	if response.Code != 499 {
		t.Fatalf("client close status = %d, want 499: %s", response.Code, response.Body.String())
	}
	entry := clientDisconnectCompletionLog(t, logs.String())
	if entry["status"] != float64(499) || entry["outcome"] != "client_disconnect" || entry["level"] != "WARN" {
		t.Errorf("client close access log = %v", entry)
	}
	if got := s.metrics.ClientDisconnectsTotal.Get("PutObject"); got != 1 {
		t.Errorf("PutObject client disconnect count = %d, want 1", got)
	}
	if got := s.metrics.ErrorsTotal.Get("InternalError:PutObject"); got != 0 {
		t.Errorf("InternalError PutObject count = %d, want 0", got)
	}
}

func TestClientDisconnectRequestCanceled(t *testing.T) {
	s, logs, secretKey := newClientDisconnectTestServer(t)
	req := clientDisconnectTestRequest(t, secretKey)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	cancel()

	s.wrapHandler(func(_ http.ResponseWriter, r *http.Request) {
		if !errors.Is(r.Context().Err(), context.Canceled) {
			t.Errorf("request context error = %v, want canceled", r.Context().Err())
		}
	})(httptest.NewRecorder(), req)

	assertClientDisconnectResult(t, s, logs.String())
}

func TestClientDisconnectRequestDeadline(t *testing.T) {
	s, logs, secretKey := newClientDisconnectTestServer(t)
	req := clientDisconnectTestRequest(t, secretKey)
	ctx, cancel := context.WithDeadline(req.Context(), time.Now().Add(-time.Second))
	defer cancel()
	req = req.WithContext(ctx)
	writer := httptest.NewRecorder()

	s.wrapHandler(func(w http.ResponseWriter, r *http.Request) {
		if !errors.Is(r.Context().Err(), context.DeadlineExceeded) {
			t.Errorf("request context error = %v, want deadline exceeded", r.Context().Err())
		}
		// A handler may return on its deadline without writing a response.
		// The wrapper must still emit the S3 timeout response when possible.
	})(writer, req)

	if writer.Code != http.StatusServiceUnavailable || !strings.Contains(writer.Body.String(), "<Code>RequestTimeout</Code>") {
		t.Errorf("wire response = %d %q, want RequestTimeout/503", writer.Code, writer.Body.String())
	}
	entry := clientDisconnectCompletionLog(t, logs.String())
	if got := entry["status"]; got != float64(http.StatusServiceUnavailable) {
		t.Errorf("access log status = %v, want 503", got)
	}
	if got := entry["error_code"]; got != "RequestTimeout" {
		t.Errorf("access log error_code = %v, want RequestTimeout", got)
	}
	if _, present := entry["outcome"]; present {
		t.Errorf("deadline was classified as client disconnect: %v", entry["outcome"])
	}
	if got := s.metrics.ErrorsTotal.Get("RequestTimeout:GetObject"); got != 1 {
		t.Errorf("RequestTimeout S3 error count = %d, want 1", got)
	}
	if !strings.Contains(s.metrics.PrometheusFormat(), `armor_errors_total{code="RequestTimeout",operation="GetObject"} 1`) {
		t.Errorf("RequestTimeout sample missing from metrics exposition")
	}
	if got := s.metrics.ClientDisconnectsTotal.Get("GetObject"); got != 0 {
		t.Errorf("deadline client disconnect count = %d, want 0", got)
	}
}
