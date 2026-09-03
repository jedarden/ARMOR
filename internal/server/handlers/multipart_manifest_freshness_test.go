package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	handlers "github.com/jedarden/armor/internal/server/handlers"
)

// TestGetObjectManifestCiphertextFreshnessOrdering covers both timestamp
// orderings of the ADR-016 ciphertext freshness gate on the manifest read path.
//
// A multipart completion assembles the ciphertext first and writes the manifest
// afterwards, so a manifest whose completedAt postdates the ciphertext's
// LastModified is the normal ordering and must be served (regression: this used
// to 500 on every such object, as seen in production on ord-devimprint where a
// 60s assembly gap permanently broke GetObject for a litestream segment). Only
// a ciphertext NEWER than the manifest — an overwrite by a later same-key
// upload between the two manifest writes — leaves the manifest stale.
//
// The manifest is written directly against a ciphertext produced by a real PUT,
// which exercises the full GetObject manifest path (readManifest → freshness
// gate → decrypt) without needing the multipart machinery.
func TestGetObjectManifestCiphertextFreshnessOrdering(t *testing.T) {
	cfg, mb, cache, footerCache, km := testSetup(t)
	h := handlers.New(cfg, mb, cache, footerCache, km, nil)

	bucket, key := "test-bucket", "freshness-key"
	plaintext := []byte("multipart manifest freshness regression plaintext")

	// Store a valid encrypted ciphertext object with a real PUT.
	putReq := httptest.NewRequest(http.MethodPut, "/"+bucket+"/"+key, bytes.NewReader(plaintext))
	putReq.Header.Set("Content-Type", "text/plain")
	putW := httptest.NewRecorder()
	h.HandleRoot(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT failed: status %d, body %s", putW.Code, putW.Body.String())
	}

	// The manifest carries the same ARMOR metadata as the ciphertext object,
	// plus the ADR-016 manifest fields.
	mb.mu.Lock()
	armorMeta := make(map[string]string, len(mb.meta[bucket+"/"+key]))
	for k, v := range mb.meta[bucket+"/"+key] {
		armorMeta[k] = v
	}
	mb.mu.Unlock()
	if armorMeta["x-amz-meta-armor-version"] == "" {
		t.Fatal("stored ciphertext object carries no ARMOR metadata")
	}

	writeManifest := func(t *testing.T, completedAt time.Time) {
		t.Helper()
		meta := make(map[string]string, len(armorMeta)+2)
		for k, v := range armorMeta {
			meta[k] = v
		}
		meta["x-amz-meta-armor-completed-at"] = completedAt.UTC().Format(time.RFC3339)
		meta["x-amz-meta-armor-ciphertext-ref"] = key
		meta["Content-Type"] = "application/x-armor-manifest+json"

		manifestBody, err := json.Marshal(backend.ManifestBody{
			CiphertextObject: key,
			UploadID:         "upload-freshness-test",
			CompletedAt:      completedAt.UTC().Format(time.RFC3339),
			Metadata:         meta,
		})
		if err != nil {
			t.Fatalf("failed to marshal manifest body: %v", err)
		}

		if err := mb.Put(context.Background(), bucket, key+".armor-manifest", bytes.NewReader(manifestBody), int64(len(manifestBody)), meta); err != nil {
			t.Fatalf("failed to write manifest: %v", err)
		}
	}

	getObject := func() (int, string, []byte) {
		req := httptest.NewRequest(http.MethodGet, "/"+bucket+"/"+key, nil)
		w := httptest.NewRecorder()
		h.HandleRoot(w, req)
		return w.Code, w.Body.String(), w.Body.Bytes()
	}

	t.Run("ciphertext predating manifest completion is served", func(t *testing.T) {
		// mockBackend.Head stamps LastModified at GET time, so a completedAt in
		// the near future reproduces the production ordering: the ciphertext
		// was assembled before the manifest was completed.
		writeManifest(t, time.Now().Add(2*time.Minute))

		code, body, got := getObject()
		if code != http.StatusOK {
			t.Fatalf("GetObject returned %d for ciphertext older than manifest: %s", code, body)
		}
		if !bytes.Equal(got, plaintext) {
			t.Errorf("GetObject content mismatch: got %q, want %q", string(got), string(plaintext))
		}
	})

	t.Run("ciphertext newer than manifest completion is rejected as stale", func(t *testing.T) {
		// completedAt in the past puts the ciphertext's LastModified (GET time)
		// after manifest completion: the overwrite ordering. The manifest no
		// longer describes the object at the ciphertext ref, so the read must
		// fail rather than serve data the manifest cannot vouch for.
		writeManifest(t, time.Now().Add(-2*time.Minute))

		code, body, _ := getObject()
		if code != http.StatusInternalServerError {
			t.Fatalf("GetObject returned %d for overwritten ciphertext, want 500: %s", code, body)
		}
		if !strings.Contains(body, "Stale manifest") {
			t.Errorf("GetObject error %q does not mention the stale manifest", body)
		}
	})
}
