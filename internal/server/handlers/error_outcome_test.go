package handlers

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
)

type closedErrorWriter struct{ *httptest.ResponseRecorder }

func (w *closedErrorWriter) Write([]byte) (int, error) { return 0, net.ErrClosed }

var _ http.ResponseWriter = (*closedErrorWriter)(nil)

func TestClientDisconnectWhileWritingError(t *testing.T) {
	m := metrics.NewMetrics()
	h := New(&config.Config{}, nil, nil, nil, nil, nil)
	h.WithMetrics(m)
	var logs strings.Builder
	logger := logging.New("armor-test")
	logger.SetOutput(&logs)
	h.WithLogger(logger)
	req := httptest.NewRequest("GET", "/bucket/key", nil)
	w := &closedErrorWriter{httptest.NewRecorder()}
	h.writeError(w, req, "InternalError", "backend failed", 500)
	if got := m.ClientDisconnectsTotal.Get("GetObject"); got != 1 {
		t.Errorf("client disconnects = %d, want 1", got)
	}
	if got := m.ErrorsTotal.Get("InternalError:GetObject"); got != 0 {
		t.Errorf("InternalError count = %d, want 0", got)
	}
	if got := logs.String(); !strings.Contains(got, `"outcome":"client_disconnect"`) || !strings.Contains(got, `"status":499`) || !strings.Contains(got, `"level":"WARN"`) {
		t.Errorf("writer disconnect log lacks WARN outcome and 499: %s", got)
	}
}

func TestClientDisconnectErrorCauses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		cause  error
		cancel bool
	}{
		{name: "request context canceled", cancel: true},
		{name: "unexpected EOF", cause: io.ErrUnexpectedEOF},
		{name: "connection reset", cause: &net.OpError{Err: syscall.ECONNRESET}},
		{name: "closed connection", cause: net.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := metrics.NewMetrics()
			h := New(&config.Config{}, nil, nil, nil, nil, nil)
			h.WithMetrics(m)
			req := httptest.NewRequest("PUT", "/bucket/key", nil)
			if tc.cancel {
				ctx, cancel := context.WithCancel(req.Context())
				cancel()
				req = req.WithContext(ctx)
			}
			response := httptest.NewRecorder()
			h.writeErrorCause(response, req, "InternalError", "failed", 500, tc.cause)
			if response.Code != 499 {
				t.Errorf("status = %d, want 499", response.Code)
			}
			if got := m.ClientDisconnectsTotal.Get("PutObject"); got != 1 {
				t.Errorf("client disconnects = %d, want 1", got)
			}
			if got := m.ErrorsTotal.Get("InternalError:PutObject"); got != 0 {
				t.Errorf("InternalError count = %d, want 0", got)
			}
		})
	}
}

func TestRequestDeadlineRemainsServerError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cause error
	}{
		{name: "request deadline"},
		{name: "backend deadline", cause: errors.Join(context.DeadlineExceeded, errors.New("ranged read"))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := metrics.NewMetrics()
			h := New(&config.Config{}, nil, nil, nil, nil, nil)
			h.WithMetrics(m)
			req := httptest.NewRequest("GET", "/bucket/key", nil)
			if tc.cause == nil {
				ctx, cancel := context.WithDeadline(req.Context(), time.Now().Add(-time.Second))
				defer cancel()
				req = req.WithContext(ctx)
			}
			response := httptest.NewRecorder()
			h.writeBackendError(response, req, "InternalError", "failed", 500, tc.cause)
			if response.Code != 503 {
				t.Errorf("status = %d, want 503", response.Code)
			}
			if got := m.ErrorsTotal.Get("RequestTimeout:GetObject"); got != 1 {
				t.Errorf("RequestTimeout count = %d, want 1", got)
			}
			if got := m.ClientDisconnectsTotal.Get("GetObject"); got != 0 {
				t.Errorf("client disconnects = %d, want 0", got)
			}
		})
	}
}
