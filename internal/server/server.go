// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/canary"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/presign"
	"github.com/jedarden/armor/internal/provenance"
	"github.com/jedarden/armor/internal/server/handlers"
)

// Server represents the ARMOR server.
type Server struct {
	config      *config.Config
	backend     backend.Backend
	cache       *backend.MetadataCache
	footerCache *backend.FooterCache
	keyManager  *keymanager.KeyManager
	canary      *canary.Monitor
	provenance  *provenance.Manager
	presigner   *presign.Signer

	// canaryStarted tracks whether the canary monitor has been started
	canaryStarted bool

	// Metrics and request tracking
	metrics       *metrics.Metrics
	requestTracker *metrics.RequestTracker
	logger        *logging.Logger
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

	// Create key manager
	// Convert config.KeyRoutes to keymanager.Route format
	var routes []keymanager.Route
	for _, r := range cfg.KeyRoutes {
		routes = append(routes, keymanager.Route{
			Prefix:  r.Prefix,
			KeyName: r.KeyName,
		})
	}
	keyMgr, err := keymanager.New(cfg.MEK, cfg.NamedKeys, routes)
	if err != nil {
		return nil, fmt.Errorf("failed to create key manager: %w", err)
	}

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

	// Create logger
	logger := logging.New("armor")

	// Create presign signer
	presigner := presign.NewSigner(cfg.PresignSecret, cfg.PresignBaseURL)

	return &Server{
		config:         cfg,
		backend:        b2Backend,
		cache:          cache,
		footerCache:    footerCache,
		keyManager:     keyMgr,
		canary:         canaryMonitor,
		provenance:     provenanceMgr,
		presigner:      presigner,
		metrics:        metrics.DefaultMetrics,
		requestTracker: metrics.DefaultRequestTracker,
		logger:         logger,
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
	s.logger.Info("Canary monitor started")
}

// StopCanary stops the canary monitor.
func (s *Server) StopCanary() {
	if s.canary != nil && s.canaryStarted {
		s.canary.Stop()
		s.logger.Info("Canary monitor stopped")
	}
}

// Handler returns the main S3 API handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Health endpoints
	mux.HandleFunc("/healthz", s.healthz)
	mux.HandleFunc("/readyz", s.readyz)

	// Share endpoint for pre-signed URLs (public, no auth required)
	mux.HandleFunc("/share/", s.handleShare)

	// S3 operations
	h := handlers.New(s.config, s.backend, s.cache, s.footerCache, s.keyManager)

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
	mux.HandleFunc("/admin/presign", s.handlePresign)
	mux.HandleFunc("/metrics", s.metrics.Handler())

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

	// Create key rotator with the default key
	defaultKey := s.keyManager.DefaultKey()
	rotator := NewKeyRotator(s.backend, s.config.Bucket, defaultKey.MEK, newMEK)

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
		if err := s.keyManager.UpdateDefaultKey(newMEK); err != nil {
			s.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("failed to update key manager after rotation")
		}
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

	// Export the default MEK as hex-encoded string
	defaultKey := s.keyManager.DefaultKey()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"mek":   hex.EncodeToString(defaultKey.MEK),
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
		start := time.Now()

		// Track in-flight request
		s.requestTracker.Start()
		defer s.requestTracker.End()

		// Log request
		s.logger.WithFields(map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Debug("incoming request")

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
			cred, err := s.verifyAuthAndGetCredential(r)
			if err != nil {
				s.writeError(w, "AccessDenied", "Invalid credentials", 403)
				s.metrics.IncRequestsTotal("auth", 403)
				return
			}
			// Check ACL for the request
			bucket, key := s.extractBucketAndKey(r)
			if err := CheckACL(cred, bucket, key); err != nil {
				s.writeError(w, "AccessDenied", "Access Denied", 403)
				s.metrics.IncRequestsTotal("acl", 403)
				return
			}
		}

		// Use a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		h(rw, r)

		// Record metrics
		duration := time.Since(start)
		s.metrics.IncRequestsTotal(r.Method, rw.statusCode)
		s.metrics.RecordRequestDuration(r.Method, duration)

		// Log completed request
		s.logger.WithFields(map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      rw.statusCode,
			"duration_ms": duration.Milliseconds(),
		}).Info("request completed")
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code.
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// isPublicPath checks if a path is public (no auth required).
func (s *Server) isPublicPath(path string) bool {
	return path == "/healthz" || path == "/readyz"
}

// verifyAuthAndGetCredential validates AWS SigV4 authentication and returns the credential.
func (s *Server) verifyAuthAndGetCredential(r *http.Request) (*config.Credential, error) {
	// Create auth with all credentials
	auth := NewSigV4AuthWithCredentials(s.config.Credentials, s.config.B2Region)

	// Check for query-based auth (presigned URLs)
	if r.URL.Query().Get("X-Amz-Credential") != "" {
		return auth.VerifyQueryAuth(r)
	}

	// Check for header-based auth
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, ErrMissingAuthHeader
	}

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

	return auth.VerifyRequest(r, body)
}

// extractBucketAndKey extracts bucket and key from the request URL.
func (s *Server) extractBucketAndKey(r *http.Request) (bucket, key string) {
	path := r.URL.Path
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// Check for virtual-hosted-style (bucket in host)
	// For path-style: /bucket/key
	parts := strings.SplitN(path, "/", 2)
	if len(parts) >= 1 {
		bucket = parts[0]
	}
	if len(parts) >= 2 {
		key = parts[1]
	}

	// Use configured bucket if empty
	if bucket == "" {
		bucket = s.config.Bucket
	}

	return bucket, key
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

// WaitForInFlightRequests waits for all in-flight requests to complete.
func (s *Server) WaitForInFlightRequests() {
	s.requestTracker.Wait()
}

// InFlightRequestCount returns the current number of in-flight requests.
func (s *Server) InFlightRequestCount() int64 {
	return s.requestTracker.Count()
}

// handlePresign generates a pre-signed URL for sharing encrypted files.
// POST /admin/presign
// Body: {"bucket": "my-bucket", "key": "path/to/file.parquet", "expires_in": "1h", "content_disposition": "attachment; filename=\"file.parquet\""}
func (s *Server) handlePresign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify auth
	cred, err := s.verifyAuthAndGetCredential(r)
	if err != nil {
		s.writeError(w, "AccessDenied", "Invalid credentials", 403)
		return
	}

	// Parse request body
	var req struct {
		Bucket             string `json:"bucket"`
		Key                string `json:"key"`
		ExpiresIn          string `json:"expires_in"`
		ContentDisposition string `json:"content_disposition"`
		Range              string `json:"range"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Use configured bucket if not specified
	bucket := req.Bucket
	if bucket == "" {
		bucket = s.config.Bucket
	}

	// Validate required fields
	if req.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}

	// Check ACL for the request
	if err := CheckACL(cred, bucket, req.Key); err != nil {
		s.writeError(w, "AccessDenied", "Access Denied", 403)
		return
	}

	// Parse expiration (default 1 hour)
	expiration := presign.DefaultExpiration
	if req.ExpiresIn != "" {
		expiration, err = presign.ParseExpiration(req.ExpiresIn)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid expires_in: %v", err), http.StatusBadRequest)
			return
		}
	}

	// Build options
	var opts []presign.Option
	if req.ContentDisposition != "" {
		opts = append(opts, presign.WithContentDisposition(req.ContentDisposition))
	}
	if req.Range != "" {
		opts = append(opts, presign.WithRange(req.Range))
	}

	// Generate pre-signed URL
	shareURL, err := s.presigner.GenerateURL(bucket, req.Key, expiration, opts...)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate URL: %v", err), http.StatusInternalServerError)
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"url":       shareURL,
		"expires_in": presign.FormatExpiration(expiration),
		"expires_at": time.Now().Add(expiration).UTC().Format(time.RFC3339),
	})
}

// handleShare serves decrypted content from a pre-signed URL token.
// GET /share/<token>
func (s *Server) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract token from path
	tokenStr := presign.ParseTokenFromPath(r.URL.Path)
	if tokenStr == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Verify token
	token, err := s.presigner.VerifyToken(tokenStr)
	if err != nil {
		if errors.Is(err, presign.ErrExpiredToken) {
			http.Error(w, "Link expired", http.StatusGone)
			return
		}
		if errors.Is(err, presign.ErrInvalidSignature) {
			http.Error(w, "Invalid link", http.StatusForbidden)
			return
		}
		http.Error(w, "Invalid token", http.StatusBadRequest)
		return
	}

	// Get object metadata
	ctx := r.Context()
	info, err := s.backend.Head(ctx, token.Bucket, token.Key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Object not found: %v", err), http.StatusNotFound)
		return
	}

	// Check if object is ARMOR-encrypted
	if !info.IsARMOREncrypted {
		// Serve non-ARMOR objects directly (passthrough)
		body, _, err := s.backend.Get(ctx, token.Bucket, token.Key)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get object: %v", err), http.StatusInternalServerError)
			return
		}
		defer body.Close()

		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size))
		w.Header().Set("Content-Type", info.ContentType)
		if token.ContentDisposition != "" {
			w.Header().Set("Content-Disposition", token.ContentDisposition)
		}
		w.WriteHeader(http.StatusOK)
		io.Copy(w, body)
		return
	}

	// Parse ARMOR metadata
	armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		http.Error(w, "Failed to parse object metadata", http.StatusInternalServerError)
		return
	}

	// Get the MEK for this object
	mek, err := s.keyManager.GetMEKByID(armorMeta.KeyID)
	if err != nil {
		http.Error(w, "Failed to get decryption key", http.StatusInternalServerError)
		return
	}

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(mek, armorMeta.WrappedDEK)
	if err != nil {
		http.Error(w, "Failed to unwrap DEK", http.StatusInternalServerError)
		return
	}

	// Create decryptor
	decryptor, err := crypto.NewDecryptor(dek, armorMeta.IV, armorMeta.BlockSize)
	if err != nil {
		http.Error(w, "Failed to create decryptor", http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Length", fmt.Sprintf("%d", armorMeta.PlaintextSize))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("Accept-Ranges", "bytes")
	if token.ContentDisposition != "" {
		w.Header().Set("Content-Disposition", token.ContentDisposition)
	}

	// Handle range request if specified in token or header
	rangeHeader := token.Range
	if rangeHeader == "" {
		rangeHeader = r.Header.Get("Range")
	}

	if rangeHeader != "" {
		s.handleShareRangeRequest(w, r, token, decryptor, armorMeta, rangeHeader)
		return
	}

	// Full object download
	s.handleShareFullObject(w, r, token, decryptor, armorMeta)
}

// handleShareFullObject handles full object downloads for share endpoint.
func (s *Server) handleShareFullObject(w http.ResponseWriter, r *http.Request, token *presign.Token, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata) {
	ctx := r.Context()

	blockSize := armorMeta.BlockSize
	blockCount := int(crypto.ComputeBlockCount(armorMeta.PlaintextSize, blockSize))
	plaintextSize := armorMeta.PlaintextSize

	// Calculate offsets
	hmacTableOffset := crypto.HeaderSize + plaintextSize
	hmacTableSize := int64(blockCount) * crypto.HMACSize

	// 1. Prefetch HMAC table
	hmacBody, err := s.backend.GetRange(ctx, token.Bucket, token.Key, hmacTableOffset, hmacTableSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch HMAC table: %v", err), http.StatusInternalServerError)
		return
	}
	hmacTable, err := io.ReadAll(hmacBody)
	hmacBody.Close()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read HMAC table: %v", err), http.StatusInternalServerError)
		return
	}

	// 2. Stream data from Cloudflare
	streamSize := crypto.HeaderSize + plaintextSize
	dataBody, err := s.backend.GetRange(ctx, token.Bucket, token.Key, 0, streamSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get object stream: %v", err), http.StatusInternalServerError)
		return
	}
	defer dataBody.Close()

	// 3. Read and discard header
	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(dataBody, headerBuf); err != nil {
		http.Error(w, fmt.Sprintf("Failed to read header: %v", err), http.StatusInternalServerError)
		return
	}

	// 4. Write status before streaming
	w.WriteHeader(http.StatusOK)

	// 5. Stream decrypt
	encryptedBuf := make([]byte, blockSize)
	for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
		remaining := plaintextSize - int64(blockIndex)*int64(blockSize)
		actualBlockSize := int(min64(int64(blockSize), remaining))

		encryptedBuf = encryptedBuf[:actualBlockSize]
		n, err := io.ReadFull(dataBody, encryptedBuf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return
		}
		if n == 0 {
			break
		}
		encryptedBuf = encryptedBuf[:n]

		// Verify HMAC
		hmacOffset := blockIndex * crypto.HMACSize
		if hmacOffset+crypto.HMACSize > len(hmacTable) {
			return
		}
		expectedHMAC := hmacTable[hmacOffset : hmacOffset+crypto.HMACSize]

		mac := hmac.New(sha256.New, decryptor.HMACKey())
		indexBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(indexBytes, uint32(blockIndex))
		mac.Write(indexBytes)
		mac.Write(encryptedBuf)
		computed := mac.Sum(nil)

		if !hmac.Equal(computed, expectedHMAC) {
			return
		}

		// Decrypt
		decrypted := make([]byte, n)
		ctr := makeCounter(armorMeta.IV, uint32(blockIndex))
		stream := cipher.NewCTR(decryptor.CipherBlock(), ctr)
		stream.XORKeyStream(decrypted, encryptedBuf)

		// Write to client
		w.Write(decrypted)
	}
}

// handleShareRangeRequest handles range requests for share endpoint.
func (s *Server) handleShareRangeRequest(w http.ResponseWriter, r *http.Request, token *presign.Token, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, rangeHeader string) {
	ctx := r.Context()
	plaintextSize := armorMeta.PlaintextSize

	// Parse range header
	start, end, err := parseRangeHeader(rangeHeader, plaintextSize)
	if err != nil {
		http.Error(w, "Invalid range", http.StatusBadRequest)
		return
	}

	// Translate range to encrypted blocks
	translation, err := crypto.TranslateRange(start, end, plaintextSize, armorMeta.BlockSize, crypto.HeaderSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to translate range: %v", err), http.StatusInternalServerError)
		return
	}

	// Fetch encrypted blocks and HMAC table in parallel
	var encrypted, hmacTable []byte

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		encryptedBody, err := s.backend.GetRange(gctx, token.Bucket, token.Key, translation.DataOffset, translation.DataLength)
		if err != nil {
			return fmt.Errorf("failed to fetch encrypted blocks: %w", err)
		}
		defer encryptedBody.Close()

		encrypted, err = io.ReadAll(encryptedBody)
		if err != nil {
			return fmt.Errorf("failed to read encrypted blocks: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		hmacBody, err := s.backend.GetRange(gctx, token.Bucket, token.Key, translation.HMACOffset, translation.HMACLength)
		if err != nil {
			return fmt.Errorf("failed to fetch HMAC table: %w", err)
		}
		defer hmacBody.Close()

		hmacTable, err = io.ReadAll(hmacBody)
		if err != nil {
			return fmt.Errorf("failed to read HMAC table: %w", err)
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Decrypt range
	plaintext, err := decryptor.DecryptRange(encrypted, hmacTable, start, end, plaintextSize)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decrypt range: %v", err), http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(plaintext)))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, plaintextSize))
	w.WriteHeader(http.StatusPartialContent)
	w.Write(plaintext)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// makeCounter creates a 16-byte counter value from the IV and block index.
func makeCounter(iv []byte, blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], iv[0:12])
	binary.BigEndian.PutUint32(counter[12:16], blockIndex)
	return counter
}

// parseRangeHeader parses a Range header like "bytes=0-1023".
func parseRangeHeader(header string, totalSize int64) (start, end int64, err error) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	rangeSpec := strings.TrimPrefix(header, "bytes=")

	if strings.Contains(rangeSpec, ",") {
		return 0, 0, fmt.Errorf("multiple ranges not supported")
	}

	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid range format")
	}

	if parts[0] == "" {
		// Suffix range: -500 means last 500 bytes
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		start = totalSize - suffix
		end = totalSize - 1
	} else {
		start, err = strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return 0, 0, err
		}
		if parts[1] == "" {
			end = totalSize - 1
		} else {
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return 0, 0, err
			}
		}
	}

	if start < 0 || start >= totalSize || end < start || end >= totalSize {
		return 0, 0, fmt.Errorf("range out of bounds")
	}

	return start, end, nil
}
