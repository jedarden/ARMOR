package server

// Admin endpoints giving operators a repair path for manifests rejected by the
// ADR-016 ciphertext freshness gate. The semantics live in
// internal/server/handlers/manifest_repair.go; this file is the HTTP surface:
//
//	GET  /admin/manifest?key=<key>[&bucket=<b>]            state of the manifest
//	POST /admin/manifest/repair?key=<key>[&bucket=<b>]     re-stamp completedAt
//	POST /admin/manifest/quarantine?key=<key>&reason=<r>   mark quarantined
//	POST /admin/manifest/release?key=<key>[&bucket=<b>]    lift quarantine
//
// key is the logical object key as clients read it, not the manifest key.
// Like /admin/format/migrate, these routes sit behind the admin token gate
// (admin_auth.go) and return JSON.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jedarden/armor/internal/server/handlers"
)

// ErrMissingKey marks a request that named no object key. It is a client error
// (400), not a server one, so it gets its own sentinel instead of falling into
// writeManifestAdminError's 500 default.
var ErrMissingKey = errors.New("key parameter is required")

// manifestAdminHandlers builds a Handlers wired the same way as the S3 mux.
// The admin mux has no Handlers instance of its own — Handler() constructs one
// for the S3 surface — so these endpoints build the identical wiring on
// demand, the same per-request pattern migrateFormat uses for
// NewFormatMigrator.
func (s *Server) manifestAdminHandlers() *handlers.Handlers {
	return handlers.New(s.config, s.backend, s.cache, s.footerCache, s.keyManager, s.listCache)
}

// manifestAdminBucket resolves the bucket an admin request addresses: the
// explicit bucket parameter when given, else the server's configured bucket.
func (s *Server) manifestAdminBucket(r *http.Request) string {
	if bucket := r.URL.Query().Get("bucket"); bucket != "" {
		return bucket
	}
	return s.config.Bucket
}

// manifestAdminKey extracts and validates the object key parameter shared by
// all four endpoints.
func manifestAdminKey(r *http.Request) (string, error) {
	key := r.URL.Query().Get("key")
	if key == "" {
		return "", ErrMissingKey
	}
	return key, nil
}

func writeManifestAdminResult(w http.ResponseWriter, status *handlers.ManifestStatus) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"manifest": status,
	})
}

func writeManifestAdminError(w http.ResponseWriter, err error) {
	statusCode := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrMissingKey):
		statusCode = http.StatusBadRequest
	case errors.Is(err, handlers.ErrManifestNotFound):
		statusCode = http.StatusNotFound
	case errors.Is(err, handlers.ErrNoCiphertextRef),
		errors.Is(err, handlers.ErrNoCompletedAt),
		errors.Is(err, handlers.ErrNoCiphertextModified):
		statusCode = http.StatusConflict
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "error",
		"error":  err.Error(),
	})
}

// handleManifestInspect reports the repair-relevant state of a manifest:
// which ciphertext it references, the completion timestamp the freshness gate
// compares against, whether the gate currently passes, and quarantine state.
func (s *Server) handleManifestInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key, err := manifestAdminKey(r)
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	status, err := s.manifestAdminHandlers().InspectManifest(r.Context(), s.manifestAdminBucket(r), key)
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	writeManifestAdminResult(w, status)
}

// handleManifestRepair re-stamps a manifest's completedAt to its ciphertext's
// LastModified, declaring the ciphertext canonical and letting reads resume.
func (s *Server) handleManifestRepair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key, err := manifestAdminKey(r)
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	status, err := s.manifestAdminHandlers().RepairManifest(r.Context(), s.manifestAdminBucket(r), key)
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	writeManifestAdminResult(w, status)
}

// handleManifestQuarantine marks a manifest quarantined so readers get a
// definitive non-retryable error instead of a retryable 500.
func (s *Server) handleManifestQuarantine(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key, err := manifestAdminKey(r)
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	status, err := s.manifestAdminHandlers().QuarantineManifest(r.Context(), s.manifestAdminBucket(r), key, r.URL.Query().Get("reason"))
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	writeManifestAdminResult(w, status)
}

// handleManifestRelease lifts a quarantine, restoring whatever readability the
// manifest otherwise has. Idempotent.
func (s *Server) handleManifestRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	key, err := manifestAdminKey(r)
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	status, err := s.manifestAdminHandlers().ReleaseManifest(r.Context(), s.manifestAdminBucket(r), key)
	if err != nil {
		writeManifestAdminError(w, err)
		return
	}
	writeManifestAdminResult(w, status)
}
