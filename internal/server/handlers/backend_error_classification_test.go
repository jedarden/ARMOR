package handlers_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/smithy-go"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/server/handlers"
)

type headFailureBackend struct {
	*mockBackend
	headErr error
}

func (b *headFailureBackend) Head(context.Context, string, string) (*backend.ObjectInfo, error) {
	return nil, b.headErr
}

type manifestFailureBackend struct {
	*mockBackend
	manifestErr error
}

func (b *manifestFailureBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if strings.HasSuffix(key, ".armor-manifest") {
		return nil, nil, b.manifestErr
	}
	return b.mockBackend.Get(ctx, bucket, key)
}

func TestGetObjectClassifiesLegacyHeadFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		headErr    error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "timeout is retryable",
			headErr:    context.DeadlineExceeded,
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "RequestTimeout",
		},
		{
			name:       "not found remains absent",
			headErr:    &smithy.GenericAPIError{Code: "NotFound", Message: "missing"},
			wantStatus: http.StatusNotFound,
			wantCode:   "NoSuchKey",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, mb, cache, footerCache, km := testSetup(t)
			h := handlers.New(cfg, &headFailureBackend{mockBackend: mb, headErr: tc.headErr}, cache, footerCache, km, nil)
			m := metrics.NewMetrics()
			h.WithMetrics(m)
			var logs strings.Builder
			logger := logging.New("armor-test")
			logger.SetOutput(&logs)
			h.WithLogger(logger)

			w := httptest.NewRecorder()
			h.HandleRoot(w, httptest.NewRequest(http.MethodGet, "/test-bucket/object", nil))

			if w.Code != tc.wantStatus || !strings.Contains(w.Body.String(), "<Code>"+tc.wantCode+"</Code>") {
				t.Fatalf("response = %d %q, want %d/%s", w.Code, w.Body.String(), tc.wantStatus, tc.wantCode)
			}
			if !strings.Contains(logs.String(), `"status":`+strconv.Itoa(tc.wantStatus)) {
				t.Fatalf("structured log = %q, want backend response status %d", logs.String(), tc.wantStatus)
			}
		})
	}
}

func TestHeadObjectClassifiesBackendFailures(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, &headFailureBackend{mockBackend: mb, headErr: context.DeadlineExceeded}, cache, footerCache, km, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, httptest.NewRequest(http.MethodHead, "/test-bucket/object", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("HEAD status = %d, want 503", w.Code)
	}
}

func TestGetObjectClassifiesManifestLoadFailure(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, &manifestFailureBackend{mockBackend: mb, manifestErr: errors.Join(context.DeadlineExceeded, errors.New("manifest load"))}, cache, footerCache, km, nil)
	w := httptest.NewRecorder()
	h.HandleRoot(w, httptest.NewRequest(http.MethodGet, "/test-bucket/object", nil))
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "<Code>RequestTimeout</Code>") {
		t.Fatalf("manifest failure response = %d %q, want RequestTimeout/503", w.Code, w.Body.String())
	}
}
