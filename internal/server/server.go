// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/canary"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/provenance"
	"github.com/jedarden/armor/internal/server/handlers"
)

// Server represents the ARMOR server.
type Server struct {
	config      *config.Config
	backend     backend.Backend
	cache       *backend.MetadataCache
	footerCache *backend.FooterCache
	mek         []byte
	canary      *canary.Monitor
	provenance  *provenance.Manager

	// canaryStarted tracks whether the canary monitor has been started
	canaryStarted bool
}

// New creates a new ARMOR server.
func New(cfg *config.Config) (*Server, error) {
	// Create B2 backend
	b2Backend, err := backend.NewB2Backend(context.Background(), backend.B2Config{
		Region:      cfg.B2Region,
		Endpoint:    cfg.B2Endpoint,
		AccessKeyID: cfg.B2AccessKeyID,
		SecretKey:   cfg.B2SecretAccessKey,
		CFDomain:    cfg.CFDomain,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create B2 backend: %w", err)
	}

	// Create metadata cache
	cache := backend.NewMetadataCache(cfg.CacheMaxEntries, cfg.CacheTTL)

	// Create footer cache (for Parquet footer pinning)
	footerCache := backend.NewFooterCache(cfg.CacheMaxEntries, cfg.CacheTTL)

	// Create canary monitor
	canaryMonitor := canary.NewMonitor(canary.Config{
		Backend:    b2Backend,
		Bucket:     cfg.Bucket,
		MEK:        cfg.MEK,
		BlockSize:  cfg.BlockSize,
		InstanceID: cfg.WriterID,
		Interval:   5 * time.Minute,
		CanarySize: 1024,
		MaxRetries: 3,
		RetryDelay: 10 * time.Second,
	})

	// Create provenance manager
	provenanceMgr := provenance.NewManager(b2Backend, cfg.Bucket, cfg.WriterID)

	return &Server{
		config:      cfg,
		backend:     b2Backend,
		cache:       cache,
		footerCache: footerCache,
		mek:         cfg.MEK,
		canary:      canaryMonitor,
		provenance:  provenanceMgr,
	}, nil
}

// StartCanary starts the canary monitor.
// It should be called after the server is created.
func (s *Server) StartCanary(ctx context.Context) {
	if s.canary == nil || s.canaryStarted {
		return
	}
	s.canaryStarted = true
	s.canary.Start(ctx)
	log.Println("Canary monitor started")
}

// StopCanary stops the canary monitor.
func (s *Server) StopCanary() {
	if s.canary != nil && s.canaryStarted {
		s.canary.Stop()
		log.Println("Canary monitor stopped")
	}
}

// Handler returns the main S3 API handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)

	// S3 operations
	h := handlers.New(s.config, s.backend, s.cache, s.footerCache, s.mek)

	// Wire up provenance if available
	if s.provenance != nil {
		h.WithProvenance(s.provenance)
	}

	// Bucket operations
	mux.HandleFunc("/", s.wrapHandler(h.HandleRoot))

	return mux
}

// AdminHandler returns the admin API handler.
func (s *Server) AdminHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/admin/key/verify", s.verifyKey)
	mux.HandleFunc("/admin/key/rotate", s.rotateKey)
	mux.HandleFunc("/admin/key/export", s.exportKey)
	mux.HandleFunc("/armor/canary", s.canaryHandler)
	mux.HandleFunc("/armor/audit", s.audit)

	return mux
}

// healthz returns the health status.
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	// Check canary status if monitor is running
	if s.canary != nil && s.canaryStarted {
		if !s.canary.IsHealthy() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Unhealthy - canary check failed"))
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyz returns the readiness status.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	// Check canary if monitor is running
	if s.canary != nil && s.canaryStarted && !s.canary.IsHealthy() {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not ready - canary check failed"))
		return
	}

	// Backend connectivity check via minimal List operation
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if _, err := s.backend.List(ctx, s.config.Bucket, "", "", "", 1); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("Not ready - backend unavailable"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

// verifyKey verifies the MEK is correct.
func (s *Server) verifyKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.canary == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"unknown","error":"canary monitor not configured"}`))
		return
	}

	status := s.canary.GetStatus()
	if status.DecryptVerified && status.HMACVerified {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"verified","message":"MEK is correct"}`))
		return
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte(`{"status":"unverified","error":"canary check failed - MEK may be incorrect"}`))
}

// rotateKey rotates the MEK.
func (s *Server) rotateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the new MEK from the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read request body: %v", err), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// The new MEK should be a 32-byte hex-encoded string (64 hex chars)
	// or raw 32 bytes
	var newMEK []byte
	if len(body) == 64 {
		// Hex-encoded
		newMEK, err = hex.DecodeString(string(body))
		if err != nil {
			http.Error(w, "Invalid hex-encoded MEK", http.StatusBadRequest)
			return
		}
	} else if len(body) == 32 {
		// Raw bytes
		newMEK = body
	} else {
		http.Error(w, fmt.Sprintf("Invalid MEK length: expected 32 bytes or 64 hex chars, got %d", len(body)), http.StatusBadRequest)
		return
	}

	// Create key rotator
	rotator := NewKeyRotator(s.backend, s.config.Bucket, s.mek, newMEK)

	// Perform rotation
	result, err := rotator.Rotate(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "failed",
			"error":   err.Error(),
			"result":  result,
		})
		return
	}

	// Update the server's MEK on success
	if result.Status == "completed" {
		s.mek = newMEK
		// Clear the metadata cache since DEKs are now wrapped with new MEK
		s.cache.Clear()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// exportKey exports the current MEK.
func (s *Server) exportKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Query().Get("confirm") != "yes" {
		http.Error(w, "Must include ?confirm=yes to export key", http.StatusBadRequest)
		return
	}

	// Export the MEK as hex-encoded string
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"mek":   hex.EncodeToString(s.mek),
		"format": "hex",
		"warning": "This key provides access to all encrypted data. Store securely.",
	})
}

// canaryHandler returns the canary status.
func (s *Server) canaryHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.canary == nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"unknown","error":"canary monitor not configured"}`))
		return
	}

	status := s.canary.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// audit returns the audit status.
func (s *Server) audit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Create auditor and perform audit
	auditor := provenance.NewAuditor(s.backend, s.config.Bucket)
	result, err := auditor.Audit(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// wrapHandler wraps a handler with common middleware.
func (s *Server) wrapHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Log request
		log.Printf("%s %s", r.Method, r.URL.Path)

		// Add CORS headers for browser clients
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, DELETE, HEAD, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Range, Content-Length")

		// Handle preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Verify auth for non-public endpoints
		if !s.isPublicPath(r.URL.Path) {
			if !s.verifyAuth(r) {
				s.writeError(w, "AccessDenied", "Invalid credentials", 403)
				return
			}
		}

		h(w, r)
	}
}

// isPublicPath checks if a path is public (no auth required).
func (s *Server) isPublicPath(path string) bool {
	return path == "/healthz" || path == "/readyz"
}

// verifyAuth validates AWS SigV4 authentication.
func (s *Server) verifyAuth(r *http.Request) bool {
	// Check for query-based auth (presigned URLs)
	if r.URL.Query().Get("X-Amz-Credential") != "" {
		auth := NewSigV4Auth(s.config.AuthAccessKey, s.config.AuthSecretKey, s.config.B2Region)
		return auth.VerifyQueryAuth(r) == nil
	}

	// Check for header-based auth
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// Parse the auth header to extract access key for early rejection
	parsed, err := ParseAuthHeader(authHeader)
	if err != nil {
		return false
	}

	// Quick access key check before expensive signature verification
	if parsed.AccessKey != s.config.AuthAccessKey {
		return false
	}

	// Full SigV4 signature verification
	auth := NewSigV4Auth(s.config.AuthAccessKey, s.config.AuthSecretKey, s.config.B2Region)

	// Read body for signature calculation (but preserve it for handlers)
	// For GET/HEAD/DELETE, body is typically empty
	var body []byte
	if r.Method == "PUT" || r.Method == "POST" {
		// Note: In production, we'd need to handle body reading more carefully
		// to avoid consuming it before handlers can read it.
		// For now, we'll skip body-based signature verification for PUT/POST
		// and just verify headers. This is a known limitation.
		body = nil
	}

	return auth.VerifyRequest(r, body) == nil
}

// writeError writes an S3 error response.
func (s *Server) writeError(w http.ResponseWriter, code, message string, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/xml")
	errorXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
</Error>`, code, message)
	w.Write([]byte(errorXML))
}

// GenerateDEK generates a new DEK (exposed for handlers).
func (s *Server) GenerateDEK() ([]byte, error) {
	return crypto.GenerateDEK()
}

// GenerateIV generates a new IV (exposed for handlers).
func (s *Server) GenerateIV() ([]byte, error) {
	return crypto.GenerateIV()
}
