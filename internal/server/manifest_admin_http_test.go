// Package server provides end-to-end tests for the /admin/manifest repair
// endpoints, driven through the real admin mux and its bearer-token gate.
//
// The objects under repair are written straight into the filesystem backend:
// a manifest whose completedAt predates the ciphertext's LastModified is the
// stale condition the ADR-016 freshness gate rejects on every read, and these
// tests verify the operator's HTTP way out of it.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
)

const (
	manifestAdminBucketName = "manifest-repair"
	manifestAdminTestToken  = "manifest-repair-admin-token"
	manifestAdminObjectKey  = "stale/segment"
)

// manifestAdminStatus mirrors the repair-relevant subset of
// handlers.ManifestStatus as it appears in the JSON response.
type manifestAdminStatus struct {
	Key                 string `json:"key"`
	ManifestKey         string `json:"manifest_key"`
	CiphertextObject    string `json:"ciphertext_object,omitempty"`
	CompletedAt         string `json:"completed_at,omitempty"`
	OriginalCompletedAt string `json:"original_completed_at,omitempty"`
	RepairedAt          string `json:"repaired_at,omitempty"`
	CiphertextModified  string `json:"ciphertext_modified,omitempty"`
	FreshnessChecked    bool   `json:"freshness_checked"`
	Fresh               bool   `json:"fresh"`
	Quarantined         bool   `json:"quarantined"`
	QuarantineReason    string `json:"quarantine_reason,omitempty"`
	VerifyError         string `json:"verify_error,omitempty"`
}

type manifestAdminResponse struct {
	Status   string              `json:"status"`
	Error    string              `json:"error"`
	Manifest manifestAdminStatus `json:"manifest"`
	// RawBody carries the body of responses that are not JSON — the plain
	// text the bearer-token gate (401) and the method guards (405) answer
	// with, both here and across the rest of the admin API.
	RawBody string `json:"-"`
}

// manifestAdminCaller issues requests against a running admin mux.
type manifestAdminCaller struct {
	url    string
	client *http.Client
}

// call performs an admin request. An empty token omits the Authorization
// header entirely, which is how the token-gate test reaches the mux.
func (c manifestAdminCaller) call(t *testing.T, method, path, query, token string) (*http.Response, manifestAdminResponse) {
	t.Helper()

	req, err := http.NewRequest(method, c.url+path, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.URL.RawQuery = query
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("%s %s failed: %v", method, path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	var parsed manifestAdminResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		// Plain-text failure bodies are legitimate admin responses (see
		// RawBody); the status-code assertions on them still hold.
		parsed.RawBody = string(body)
	}
	return resp, parsed
}

// startManifestAdminServer runs the production admin mux against a filesystem
// backend pre-seeded with one ARMOR object whose manifest is stale. The
// manifest is written first with a completion timestamp two minutes in the
// past and the ciphertext afterwards, so the ciphertext's LastModified
// postdates the manifest's completedAt — the overwrite ordering the freshness
// gate rejects with a retryable 500 on every read.
func startManifestAdminServer(t *testing.T) manifestAdminCaller {
	t.Helper()

	fsBackend, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("create filesystem backend: %v", err)
	}
	if err := fsBackend.CreateBucket(context.Background(), manifestAdminBucketName); err != nil {
		t.Fatalf("create test bucket: %v", err)
	}

	staleCompletedAt := time.Now().Add(-2 * time.Minute).UTC().Truncate(time.Second).Format(time.RFC3339)

	manifestMeta := map[string]string{
		"Content-Type":                    "application/x-armor-manifest+json",
		"x-amz-meta-armor-version":        "3",
		"x-amz-meta-armor-completed-at":   staleCompletedAt,
		"x-amz-meta-armor-ciphertext-ref": manifestAdminObjectKey,
	}
	manifestJSON, err := json.Marshal(backend.ManifestBody{
		CiphertextObject: manifestAdminObjectKey,
		UploadID:         "upload-admin-repair-test",
		CompletedAt:      staleCompletedAt,
		Metadata:         manifestMeta,
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := fsBackend.Put(context.Background(), manifestAdminBucketName, manifestAdminObjectKey+".armor-manifest", bytes.NewReader(manifestJSON), int64(len(manifestJSON)), manifestMeta); err != nil {
		t.Fatalf("seed manifest object: %v", err)
	}
	// Written after the manifest: the stale ordering. The FS backend stamps
	// LastModified at write time, so this is strictly newer than the manifest's
	// completedAt.
	const ciphertext = "ciphertext"
	if err := fsBackend.Put(context.Background(), manifestAdminBucketName, manifestAdminObjectKey, strings.NewReader(ciphertext), int64(len(ciphertext)), map[string]string{"Content-Type": "application/octet-stream"}); err != nil {
		t.Fatalf("seed ciphertext object: %v", err)
	}

	cfg := &config.Config{
		Bucket:     manifestAdminBucketName,
		BlockSize:  65536,
		MEK:        bytes.Repeat([]byte{0x4d}, 32),
		AdminToken: manifestAdminTestToken,
	}
	armorServer, err := NewWithBackend(cfg, fsBackend)
	if err != nil {
		t.Fatalf("create ARMOR server: %v", err)
	}

	adminServer := httptest.NewServer(armorServer.AdminHandler())
	t.Cleanup(adminServer.Close)

	return manifestAdminCaller{url: adminServer.URL, client: adminServer.Client()}
}

// TestManifestRepairEndpointRestampsStaleManifest walks the full operator flow
// over HTTP: inspect a stale manifest, repair it, and see the gate pass.
func TestManifestRepairEndpointRestampsStaleManifest(t *testing.T) {
	caller := startManifestAdminServer(t)

	// Inspect: the gate is checked and fails, which is exactly the state a
	// GetObject retry loop is stuck in right now.
	resp, got := caller.call(t, http.MethodGet, "/admin/manifest", "key="+manifestAdminObjectKey, manifestAdminTestToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inspect: status = %d (error: %s)", resp.StatusCode, got.Error)
	}
	if !got.Manifest.FreshnessChecked || got.Manifest.Fresh {
		t.Fatalf("inspect reports checked=%v fresh=%v, want checked=true fresh=false (verify error %q)", got.Manifest.FreshnessChecked, got.Manifest.Fresh, got.Manifest.VerifyError)
	}

	// Repair: re-stamp completedAt to the ciphertext's LastModified.
	resp, got = caller.call(t, http.MethodPost, "/admin/manifest/repair", "key="+manifestAdminObjectKey, manifestAdminTestToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("repair: status = %d (error: %s)", resp.StatusCode, got.Error)
	}
	if !got.Manifest.Fresh {
		t.Errorf("repaired manifest reports Fresh=false (verify error: %q)", got.Manifest.VerifyError)
	}
	if got.Manifest.CompletedAt != got.Manifest.CiphertextModified {
		t.Errorf("re-stamped completedAt %q does not match ciphertext LastModified %q", got.Manifest.CompletedAt, got.Manifest.CiphertextModified)
	}
	if got.Manifest.OriginalCompletedAt == "" || got.Manifest.RepairedAt == "" {
		t.Errorf("repair response missing provenance: original=%q repaired=%q", got.Manifest.OriginalCompletedAt, got.Manifest.RepairedAt)
	}
	if got.Manifest.Quarantined {
		t.Error("repair response reports the manifest quarantined")
	}

	// A follow-up inspect sees the repaired, fresh state.
	resp, got = caller.call(t, http.MethodGet, "/admin/manifest", "key="+manifestAdminObjectKey, manifestAdminTestToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inspect after repair: status = %d (error: %s)", resp.StatusCode, got.Error)
	}
	if !got.Manifest.Fresh {
		t.Errorf("inspect after repair reports Fresh=false (verify error: %q)", got.Manifest.VerifyError)
	}
	if got.Manifest.CompletedAt != got.Manifest.CiphertextModified {
		t.Errorf("inspect after repair: completedAt %q != ciphertext LastModified %q", got.Manifest.CompletedAt, got.Manifest.CiphertextModified)
	}
}

// TestManifestQuarantineEndpointLifecycle verifies quarantine and release over
// HTTP, including the reason round-tripping back out of the inspect response.
func TestManifestQuarantineEndpointLifecycle(t *testing.T) {
	caller := startManifestAdminServer(t)

	resp, got := caller.call(t, http.MethodPost, "/admin/manifest/quarantine", "key="+manifestAdminObjectKey+"&reason=ciphertext+under+investigation", manifestAdminTestToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("quarantine: status = %d (error: %s)", resp.StatusCode, got.Error)
	}
	if !got.Manifest.Quarantined || got.Manifest.QuarantineReason != "ciphertext under investigation" {
		t.Fatalf("quarantine response: quarantined=%v reason=%q", got.Manifest.Quarantined, got.Manifest.QuarantineReason)
	}

	resp, got = caller.call(t, http.MethodGet, "/admin/manifest", "key="+manifestAdminObjectKey, manifestAdminTestToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("inspect quarantined: status = %d (error: %s)", resp.StatusCode, got.Error)
	}
	if !got.Manifest.Quarantined || got.Manifest.QuarantineReason != "ciphertext under investigation" {
		t.Errorf("inspect does not reflect quarantine: quarantined=%v reason=%q", got.Manifest.Quarantined, got.Manifest.QuarantineReason)
	}

	resp, got = caller.call(t, http.MethodPost, "/admin/manifest/release", "key="+manifestAdminObjectKey, manifestAdminTestToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("release: status = %d (error: %s)", resp.StatusCode, got.Error)
	}
	if got.Manifest.Quarantined {
		t.Error("release response still reports quarantined")
	}

	// Release is idempotent, so an operator script can release unconditionally.
	resp, got = caller.call(t, http.MethodPost, "/admin/manifest/release", "key="+manifestAdminObjectKey, manifestAdminTestToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("second release: status = %d (error: %s)", resp.StatusCode, got.Error)
	}
}

// TestManifestAdminEndpointFailures covers the fail-closed shapes: missing key
// parameter, an object with no manifest, wrong methods, an unknown bucket, and
// the admin token gate.
func TestManifestAdminEndpointFailures(t *testing.T) {
	caller := startManifestAdminServer(t)

	t.Run("missing key parameter", func(t *testing.T) {
		resp, got := caller.call(t, http.MethodPost, "/admin/manifest/repair", "", manifestAdminTestToken)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("repair without key: status = %d, want 400 (error: %s)", resp.StatusCode, got.Error)
		}
	})

	t.Run("object with no manifest", func(t *testing.T) {
		resp, got := caller.call(t, http.MethodGet, "/admin/manifest", "key=no-such-object", manifestAdminTestToken)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("inspect of unmanifested object: status = %d, want 404 (error: %s)", resp.StatusCode, got.Error)
		}
	})

	t.Run("wrong method", func(t *testing.T) {
		resp, _ := caller.call(t, http.MethodGet, "/admin/manifest/repair", "key="+manifestAdminObjectKey, manifestAdminTestToken)
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET on repair: status = %d, want 405", resp.StatusCode)
		}
	})

	t.Run("unknown bucket", func(t *testing.T) {
		resp, got := caller.call(t, http.MethodGet, "/admin/manifest", "key="+manifestAdminObjectKey+"&bucket=other-bucket", manifestAdminTestToken)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("inspect with unknown bucket: status = %d, want 404 (error: %s)", resp.StatusCode, got.Error)
		}
	})

	t.Run("missing admin token", func(t *testing.T) {
		// The bearer-token gate answers a missing token with 401 — the same
		// contract as every other gated /admin route. (403 is reserved for
		// the fail-closed case where no admin token is configured at all.)
		resp, _ := caller.call(t, http.MethodGet, "/admin/manifest", "key="+manifestAdminObjectKey, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("unauthenticated inspect: status = %d, want 401", resp.StatusCode)
		}
	})
}
