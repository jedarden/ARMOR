package handlers_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/provenance"
	"github.com/jedarden/armor/internal/server/handlers"
)

type failedProvenanceCall struct {
	key, digest, operation string
}

type failingProvenanceRecorder struct {
	calls []failedProvenanceCall
}

func (f *failingProvenanceRecorder) RecordUpload(_ context.Context, key, digest, operation string) error {
	f.calls = append(f.calls, failedProvenanceCall{key, digest, operation})
	return errors.New("injected chain write failure")
}

func (f *failingProvenanceRecorder) ShouldRecord(string) bool { return true }

func (f *failingProvenanceRecorder) CreateChainEntry(context.Context, string, string, string) (*provenance.ChainEntryData, error) {
	return nil, nil
}

func TestRecordUploadFailureAfterDurableWrite(t *testing.T) {
	const bucket = "test-bucket"
	payload := []byte("provenance failure payload")
	digest := sha256.Sum256(payload)
	wantDigest := hex.EncodeToString(digest[:])

	for _, tc := range []struct {
		name, key, operation string
		run                  func(*testing.T, *handlers.Handlers)
	}{
		{"buffered PUT", "buffered-key", "put", func(t *testing.T, h *handlers.Handlers) {
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/buffered-key", bytes.NewReader(payload))
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if w.Code != http.StatusOK {
				t.Fatalf("PUT status = %d: %s", w.Code, w.Body.String())
			}
		}},
		{"streaming PUT", "streaming-key", "put-streaming", func(t *testing.T, h *handlers.Handlers) {
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/streaming-key", bytes.NewReader(payload))
			req.ContentLength = -1
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if w.Code != http.StatusOK || w.Header().Get("X-Armor-Streaming") != "true" {
				t.Fatalf("streaming PUT response = %d, streaming=%q: %s", w.Code, w.Header().Get("X-Armor-Streaming"), w.Body.String())
			}
		}},
		{"server-side COPY", "copy-key", "copy", func(t *testing.T, h *handlers.Handlers) {
			req := httptest.NewRequest(http.MethodPut, "/test-bucket/copy-key", nil)
			req.Header.Set("x-amz-copy-source", "/test-bucket/source-key")
			w := httptest.NewRecorder()
			h.HandleRoot(w, req)
			if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "CopyObjectResult") {
				t.Fatalf("COPY response = %d: %s", w.Code, w.Body.String())
			}
		}},
		{"multipart completion", "multipart-key", "multipart", func(t *testing.T, h *handlers.Handlers) {
			uploadID := initiateMultipart(t, h, bucket, "multipart-key")
			etag := uploadPart(t, h, bucket, "multipart-key", uploadID, 1, payload)
			completeMultipart(t, h, bucket, "multipart-key", uploadID, []string{etag})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, be, h := recordingTestSetup(t)
			if tc.operation == "copy" {
				req := httptest.NewRequest(http.MethodPut, "/test-bucket/source-key", bytes.NewReader(payload))
				w := httptest.NewRecorder()
				h.HandleRoot(w, req)
				if w.Code != http.StatusOK {
					t.Fatalf("source PUT status = %d: %s", w.Code, w.Body.String())
				}
			}

			recorder := &failingProvenanceRecorder{}
			m := metrics.NewMetrics()
			var logs strings.Builder
			logger := logging.New("armor-test")
			logger.SetOutput(&logs)
			h.WithProvenance(recorder)
			h.WithMetrics(m)
			h.WithLogger(logger)

			tc.run(t, h)
			if len(recorder.calls) != 1 || recorder.calls[0].key != tc.key || recorder.calls[0].operation != tc.operation {
				t.Fatalf("RecordUpload calls = %+v", recorder.calls)
			}
			if tc.operation == "multipart" {
				if len(recorder.calls[0].digest) != sha256.Size*2 {
					t.Fatalf("multipart digest = %q, want SHA-256 hex", recorder.calls[0].digest)
				}
			} else if recorder.calls[0].digest != wantDigest {
				t.Fatalf("RecordUpload digest = %q, want %q", recorder.calls[0].digest, wantDigest)
			}
			if got := m.ProvenanceRecordFailures.Value(); got != 1 {
				t.Fatalf("failure counter = %d, want 1", got)
			}
			if !strings.Contains(m.PrometheusFormat(), "armor_provenance_record_failures_total 1\n") {
				t.Fatal("Prometheus exposition omitted the provenance failure")
			}
			var entry logging.Entry
			if err := json.Unmarshal([]byte(strings.TrimSpace(logs.String())), &entry); err != nil {
				t.Fatalf("parse warning %q: %v", logs.String(), err)
			}
			if entry.Level != "WARN" || entry.Fields["key"] != tc.key ||
				entry.Fields["error"] != "injected chain write failure" || entry.Fields["operation"] != tc.operation {
				t.Fatalf("warning = %+v", entry)
			}
			if _, err := be.Head(context.Background(), bucket, tc.key); err != nil {
				t.Fatalf("durable object missing after provenance failure: %v", err)
			}
		})
	}
}
