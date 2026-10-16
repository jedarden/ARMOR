package handlers_test

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/server/handlers"
)

// A pipe represents the client upload stream: it sends a prefix, then closes
// before the declared body length. The typed read error must survive the
// handler's formatted error message and be recorded as a client outcome.
func TestClientDisconnectMidBodyPUTAndUploadPart(t *testing.T) {
	for _, tc := range []struct {
		name      string
		operation string
		multipart bool
	}{
		{name: "PutObject", operation: "PutObject"},
		{name: "UploadPart", operation: "UploadPart", multipart: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, mb, cache, footerCache, km := testSetup(t)
			h := handlers.New(cfg, mb, cache, footerCache, km, nil)
			m := metrics.NewMetrics()
			h.WithMetrics(m)
			var logs strings.Builder
			logger := logging.New("armor-test")
			logger.SetOutput(&logs)
			h.WithLogger(logger)

			target := "/test-bucket/aborted"
			if tc.multipart {
				create := httptest.NewRequest(http.MethodPost, target+"?uploads", nil)
				created := httptest.NewRecorder()
				h.HandleRoot(created, create)
				if created.Code != http.StatusOK {
					t.Fatalf("create upload: status %d: %s", created.Code, created.Body.String())
				}
				var result struct {
					UploadID string `xml:"UploadId"`
				}
				if err := xml.Unmarshal(created.Body.Bytes(), &result); err != nil || result.UploadID == "" {
					t.Fatalf("decode upload ID: %v", err)
				}
				target += "?partNumber=1&uploadId=" + result.UploadID
			}

			reader, writer := io.Pipe()
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				_, _ = writer.Write([]byte("partial body"))
				_ = writer.CloseWithError(io.ErrUnexpectedEOF)
			}()
			req := httptest.NewRequest(http.MethodPut, target, reader)
			req.ContentLength = 100
			response := httptest.NewRecorder()
			h.HandleRoot(response, req)
			<-finished

			if response.Code != 499 {
				t.Fatalf("client close status = %d, want 499: %s", response.Code, response.Body.String())
			}
			if got := m.ClientDisconnectsTotal.Get(tc.operation); got != 1 {
				t.Errorf("disconnects for %s = %d, want 1", tc.operation, got)
			}
			if got := m.ErrorsTotal.Get("InternalError:" + tc.operation); got != 0 {
				t.Errorf("InternalError count for %s = %d, want 0", tc.operation, got)
			}
			if got := logs.String(); !strings.Contains(got, `"outcome":"client_disconnect"`) ||
				!strings.Contains(got, `"status":499`) || !strings.Contains(got, `"level":"WARN"`) {
				t.Errorf("disconnect log lacks WARN outcome and 499: %s", got)
			}
			if want := `armor_client_disconnects_total{operation="` + tc.operation + `"} 1`; !strings.Contains(m.PrometheusFormat(), want) {
				t.Errorf("Prometheus exposition missing %s", want)
			}
		})
	}
}

type deadlineHeadBackend struct{ *mockBackend }

func (b *deadlineHeadBackend) Head(context.Context, string, string) (*backend.ObjectInfo, error) {
	return nil, context.DeadlineExceeded
}

func TestBackendDeadlineGetObjectIsRequestTimeout(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, &deadlineHeadBackend{mb}, cache, footerCache, km, nil)
	m := metrics.NewMetrics()
	h.WithMetrics(m)
	req := httptest.NewRequest(http.MethodGet, "/test-bucket/object", nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, req)
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "<Code>RequestTimeout</Code>") {
		t.Errorf("deadline response = %d %q, want RequestTimeout/503", w.Code, w.Body.String())
	}
	if got := m.ErrorsTotal.Get("RequestTimeout:GetObject"); got != 1 {
		t.Errorf("RequestTimeout count = %d, want 1", got)
	}
	if got := m.ClientDisconnectsTotal.Get("GetObject"); got != 0 {
		t.Errorf("client disconnect count = %d, want 0", got)
	}
}
