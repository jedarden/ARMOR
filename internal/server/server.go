// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/canary"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
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

	return &Server{
		config:      cfg,
		backend:     b2Backend,
		cache:       cache,
		footerCache: footerCache,
		mek:         cfg.MEK,
		canary:      canaryMonitor,
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

	// TODO: Implement key rotation
	http.Error(w, "Not implemented", http.StatusNotImplemented)
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

	// TODO: Implement secure key export
	http.Error(w, "Not implemented", http.StatusNotImplemented)
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

	// TODO: Implement provenance chain audit
	http.Error(w, "Not implemented", http.StatusNotImplemented)
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
	// For now, use simple access key validation
	// TODO: Implement full AWS SigV4 signature verification
	auth := r.Header.Get("Authorization")
	if auth == "" {
		// Check query string auth
		accessKey := r.URL.Query().Get("AWSAccessKeyId")
		if accessKey == "" {
			return false
		}
		return accessKey == s.config.AuthAccessKey
	}

	// Parse Authorization header
	// Format: AWS4-HMAC-SHA256 Credential=accessKey/...
	if strings.HasPrefix(auth, "AWS4-HMAC-SHA256") {
		parts := strings.Split(auth, " ")
		if len(parts) < 2 {
			return false
		}
		credPart := parts[1]
		if strings.HasPrefix(credPart, "Credential=") {
			cred := strings.TrimPrefix(credPart, "Credential=")
			credParts := strings.Split(cred, "/")
			if len(credParts) > 0 {
				return credParts[0] == s.config.AuthAccessKey
			}
		}
	}

	return false
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
