package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/manifest"
)

// /readyz surfaces a startup manifest load that fell back to an empty index
// after retries, without changing readiness by itself (armor-f1791426).
func TestReadyzReportsManifestDegradation(t *testing.T) {
	s := &Server{manifestSys: resolvedManifestSystem(manifestState{
		status:         manifestDegraded,
		idx:            manifest.New(),
		degradedReason: "manifest discovery list: cloudflare returned status 522",
	})}
	rec := httptest.NewRecorder()
	s.readyz(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	var body struct {
		Ready            bool   `json:"ready"`
		ManifestDegraded bool   `json:"manifest_degraded"`
		Reason           string `json:"reason"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode readyz body: %v", err)
	}
	if !body.ManifestDegraded {
		t.Fatalf("manifest_degraded not set: %s", rec.Body.String())
	}
	if !strings.Contains(body.Reason, "empty index") {
		t.Fatalf("reason %q does not mention the empty-index fallback", body.Reason)
	}

	// Without degradation the flag stays false.
	clean := &Server{}
	rec2 := httptest.NewRecorder()
	clean.readyz(rec2, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	var body2 struct {
		ManifestDegraded bool `json:"manifest_degraded"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &body2); err != nil {
		t.Fatalf("decode clean readyz body: %v", err)
	}
	if body2.ManifestDegraded {
		t.Fatal("manifest_degraded set on a clean server")
	}
}
