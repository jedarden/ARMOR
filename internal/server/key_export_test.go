package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/provenance"
)

func TestExportKeyIncludesRingAndDoesNotLogKeyMaterial(t *testing.T) {
	activeMEK := bytes.Repeat([]byte{0x11}, 32)
	ringMEKs := [][]byte{
		bytes.Repeat([]byte{0x22}, 32),
		bytes.Repeat([]byte{0x33}, 32),
	}
	ringBytes := append(append([]byte(nil), ringMEKs[0]...), ringMEKs[1]...)

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	cfg := &config.Config{
		Bucket:            "export-test-bucket",
		MEK:               activeMEK,
		KeyRings:          map[string][]byte{"default": ringBytes},
		AdminToken:        "export-admin-token",
		B2Region:          "test-region",
		B2Endpoint:        "https://b2.example.test",
		B2AccessKeyID:     "export-access-key",
		B2SecretAccessKey: "export-secret-key",
	}
	srv, err := NewWithBackend(cfg, fs)
	if err != nil {
		t.Fatalf("NewWithBackend: %v", err)
	}
	srv.provenance = provenance.NewManager(fs, cfg.Bucket, "export-test-writer")
	var logs strings.Builder
	srv.logger.SetOutput(&logs)

	httpServer := httptest.NewServer(srv.AdminHandler())
	t.Cleanup(httpServer.Close)
	noConfirmReq, err := http.NewRequest(http.MethodGet, httpServer.URL+"/admin/key/export", nil)
	if err != nil {
		t.Fatalf("NewRequest without confirmation: %v", err)
	}
	noConfirmReq.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	noConfirmResp, err := httpServer.Client().Do(noConfirmReq)
	if err != nil {
		t.Fatalf("GET /admin/key/export without confirmation: %v", err)
	}
	if noConfirmResp.StatusCode != http.StatusBadRequest {
		t.Errorf("status without confirm=yes = %d, want 400", noConfirmResp.StatusCode)
	}
	noConfirmResp.Body.Close()

	req, err := http.NewRequest(http.MethodGet, httpServer.URL+"/admin/key/export?confirm=yes", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+cfg.AdminToken)
	resp, err := httpServer.Client().Do(req)
	if err != nil {
		t.Fatalf("GET /admin/key/export: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}

	var got struct {
		MEK                  string `json:"mek"`
		ActiveKeyFingerprint string `json:"active_key_fingerprint"`
		RingKeys             []struct {
			Fingerprint string `json:"fingerprint"`
			MEK         string `json:"mek"`
		} `json:"ring_keys"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.MEK != hex.EncodeToString(activeMEK) {
		t.Error("export response has the wrong active MEK")
	}
	if got.ActiveKeyFingerprint != crypto.MEKFingerprint(activeMEK) {
		t.Errorf("active_key_fingerprint = %q, want %q", got.ActiveKeyFingerprint, crypto.MEKFingerprint(activeMEK))
	}
	if len(got.RingKeys) != len(ringMEKs) {
		t.Fatalf("ring_keys length = %d, want %d", len(got.RingKeys), len(ringMEKs))
	}
	for i, wantMEK := range ringMEKs {
		if got.RingKeys[i].MEK != hex.EncodeToString(wantMEK) {
			t.Errorf("ring_keys[%d].mek does not match the configured ring key", i)
		}
		if got.RingKeys[i].Fingerprint != crypto.MEKFingerprint(wantMEK) {
			t.Errorf("ring_keys[%d].fingerprint = %q, want %q", i, got.RingKeys[i].Fingerprint, crypto.MEKFingerprint(wantMEK))
		}
		decoded, err := hex.DecodeString(got.RingKeys[i].MEK)
		if err != nil {
			t.Fatalf("decode ring_keys[%d].mek: %v", i, err)
		}
		if got.RingKeys[i].Fingerprint != crypto.MEKFingerprint(decoded) {
			t.Errorf("ring_keys[%d] fingerprint does not match crypto.MEKFingerprint of its mek", i)
		}
	}

	for _, secret := range []string{
		hex.EncodeToString(activeMEK),
		hex.EncodeToString(ringMEKs[0]),
		hex.EncodeToString(ringMEKs[1]),
		cfg.B2SecretAccessKey,
		cfg.AdminToken,
	} {
		if strings.Contains(logs.String(), secret) {
			t.Errorf("log output contains secret material")
		}
	}

	chainBody, _, err := fs.GetDirect(context.Background(), cfg.Bucket, provenance.ChainPrefix+"export-test-writer/1.json")
	if err != nil {
		t.Fatalf("load key-export provenance record: %v", err)
	}
	defer chainBody.Close()
	var event provenance.KeyEvent
	if err := json.NewDecoder(chainBody).Decode(&event); err != nil {
		t.Fatalf("decode key-export provenance record: %v", err)
	}
	if event.EventType != "key-export" {
		t.Errorf("provenance event type = %q, want key-export", event.EventType)
	}
	if event.ExportedMEKHash != crypto.MEKFingerprint(activeMEK) {
		t.Errorf("provenance exported_mek_hash = %q, want %q", event.ExportedMEKHash, crypto.MEKFingerprint(activeMEK))
	}
}
