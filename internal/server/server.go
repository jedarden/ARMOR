// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jedarden/armor/internal/b2keys"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/canary"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/dashboard"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/presign"
	"github.com/jedarden/armor/internal/provenance"
	"github.com/jedarden/armor/internal/replication"
	"github.com/jedarden/armor/internal/server/handlers"
	"github.com/jedarden/armor/internal/server/middleware"
)

// loggerWriter adapts the structured logging.Logger to the io.Writer interface
// expected by the standard library's log.Logger. This allows the manifest
// compactor (which uses standard library logging) to write to the structured
// logger configured in the server.
type loggerWriter struct {
	logger *logging.Logger
}

// Write implements io.Writer by forwarding the byte array to the structured
// logger's Warn method (compaction errors are warnings, not fatal errors).
func (w *loggerWriter) Write(p []byte) (n int, err error) {
	w.logger.Warn(string(p))
	return len(p), nil
}

// Server represents the ARMOR server.
type Server struct {
	config            *config.Config
	backend           backend.Backend
	secondaryBackend  backend.Backend // Secondary backend for replication (ADR-006)
	cache             *backend.MetadataCache
	footerCache       *backend.FooterCache
	listCache         *backend.ListCache
	keyManager        *keymanager.KeyManager
	canary            *canary.Monitor
	provenance        *provenance.Manager
	presigner         *presign.Signer
	b2keys            *b2keys.Client // B2 native API key management
	dashboard         *dashboard.Dashboard
	manifest          *manifest.Index     // in-memory metadata index (nil when disabled)
	manifestWriter    *manifest.Writer    // async delta writer (nil when disabled)
	manifestCompactor *manifest.Compactor // background compaction goroutine (nil when disabled)
	replicationQueue  *replication.ReplicationQueue // async replication worker (nil when secondary backend not configured)

	// canaryStarted tracks whether the canary monitor has been started
	canaryStarted bool

	// canaryDisabled skips the canary check in /readyz when true
	canaryDisabled bool

	// Metrics and request tracking
	metrics        *metrics.Metrics
	requestTracker *metrics.RequestTracker
	logger         *logging.Logger
}

// New creates a new ARMOR server.
func New(cfg *config.Config) (*Server, error) {
	// Create logger early for use in backend initialization
	logger := logging.New("armor")

	// Create B2 backend
	b2Backend, err := backend.NewB2Backend(context.Background(), backend.B2Config{
		Region:          cfg.B2Region,
		Endpoint:        cfg.B2Endpoint,
		AccessKeyID:     cfg.B2AccessKeyID,
		SecretKey:       cfg.B2SecretAccessKey,
		CFDomain:        cfg.CFDomain,
		ReadConcurrency: cfg.ReadConcurrency,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create B2 backend: %w", err)
	}

	// Create secondary backend if configured (ADR-006)
	// Config package has already validated ARMOR_SECONDARY_BACKEND_TYPE and
	// ARMOR_SECONDARY_BACKEND_PATH (when Type=filesystem). When Type is empty,
	// secondaryBackend remains nil and replication is a complete no-op.
	var secondaryBackend backend.Backend
	if cfg.SecondaryBackendType != "" {
		// Build BackendConfig from validated config fields
		secondaryCfg := backend.BackendConfig{
			Type: cfg.SecondaryBackendType,
			Path: cfg.SecondaryBackendPath,
		}

		// Initialize backend based on type using the proper BackendConfig-based initializers
		switch secondaryCfg.Type {
		case "filesystem":
			secondaryBackend, err = backend.InitFilesystemBackend(secondaryCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize filesystem secondary backend: %w", err)
			}
			logger.WithFields(map[string]interface{}{
				"type": secondaryCfg.Type,
				"path": secondaryCfg.Path,
			}).Info("secondary backend initialized")
		case "b2":
			secondaryBackend, err = backend.InitB2Backend(context.Background(), secondaryCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to initialize B2 secondary backend: %w", err)
			}
			logger.WithFields(map[string]interface{}{
				"type":   secondaryCfg.Type,
				"bucket": secondaryCfg.Bucket,
			}).Info("secondary backend initialized")
		default:
			return nil, fmt.Errorf("unsupported secondary backend type: %s", secondaryCfg.Type)
		}
	}

	// Create metadata cache
	cache := backend.NewMetadataCache(cfg.CacheMaxEntries, cfg.CacheTTL)

	// Create footer cache (for Parquet footer pinning)
	footerCache := backend.NewFooterCache(cfg.CacheMaxEntries, cfg.CacheTTL)

	// Create list cache (nil when TTL is 0, meaning disabled)
	var listCache *backend.ListCache
	if cfg.ListCacheTTL > 0 {
		listCache = backend.NewListCache(cfg.ListCacheMaxEntries, cfg.ListCacheTTL)
	}

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

	// Create provenance manager
	provenanceMgr := provenance.NewManager(b2Backend, cfg.Bucket, cfg.WriterID)

	// Create presign signer
	presigner := presign.NewSigner(cfg.PresignSecret, cfg.PresignBaseURL)

	// Create B2 keys client (for B2 native API key management)
	// Uses the same B2 credentials as the S3 backend
	var b2keysClient *b2keys.Client
	if cfg.Bucket != "" {
		// Bucket-scoped configuration: application key is restricted to a single bucket
		// and cannot call account-level B2 API endpoints (like b2_authorize_account).
		// Key management is unavailable in this mode, which is expected and intentional.
		logger.Info("B2 key management disabled (application key is bucket-scoped)")
	} else {
		// No bucket restriction: attempt to create B2 keys client for account-level operations
		// Extract account ID from the key ID (B2 key IDs are in format accountIdKeyId)
		// The account ID is the first 12 characters
		accountID := cfg.B2AccessKeyID
		if len(accountID) > 12 {
			accountID = accountID[:12]
		}
		b2keysClient, err = b2keys.NewClient(context.Background(), accountID, cfg.B2SecretAccessKey)
		if err != nil {
			logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Warn("Failed to create B2 keys client - key management disabled")
		}
	}

	// Create dashboard with optional authentication
	dash := dashboard.NewWithAuth(b2Backend, cfg.Bucket, metrics.DefaultMetrics,
		cfg.DashboardUser, cfg.DashboardPass, cfg.DashboardToken)

	// Load manifest index from B2 (startup load).
	// The manifest is a performance optimisation — errors are logged as warnings
	// and do not prevent startup.
	var manifestIdx *manifest.Index
	if cfg.ManifestEnabled {
		manifestIdx = manifest.New()
		// ListRaw bypasses the .armor/ filter so manifest keys are discoverable.
		lister := func(ctx context.Context, prefix, token string) ([]string, string, error) {
			result, lerr := b2Backend.ListRaw(ctx, cfg.Bucket, prefix, "", token, 1000)
			if lerr != nil {
				return nil, "", lerr
			}
			keys := make([]string, len(result.Objects))
			for i, obj := range result.Objects {
				keys[i] = obj.Key
			}
			return keys, result.NextToken, nil
		}
		// Fetch snapshot and delta blobs via Cloudflare for free egress and edge
		// caching. Fall back to direct B2 if no Cloudflare domain is configured.
		fetcher := func(ctx context.Context, key string) ([]byte, error) {
			var body io.ReadCloser
			var ferr error
			if cfg.CFDomain != "" {
				body, _, ferr = b2Backend.Get(ctx, cfg.Bucket, key)
			} else {
				body, _, ferr = b2Backend.GetDirect(ctx, cfg.Bucket, key)
			}
			if ferr != nil {
				return nil, ferr
			}
			defer body.Close()
			return io.ReadAll(body)
		}
		loadStart := time.Now()
		if loadErr := manifest.Load(context.Background(), manifestIdx, cfg.ManifestPrefix, cfg.WriterID, lister, fetcher); loadErr != nil {
			logger.WithFields(map[string]interface{}{
				"error": loadErr.Error(),
			}).Warn("manifest startup load failed — continuing with empty manifest index")
			manifestIdx = manifest.New() // reset to empty on error
		} else {
			logger.WithFields(map[string]interface{}{
				"entries":  manifestIdx.Len(),
				"seq":      manifestIdx.Seq(),
				"duration": time.Since(loadStart).String(),
			}).Info("manifest index loaded")
		}
	}

	// Create and start the manifest writer (async delta persistence to B2).
	// Uploads go direct to B2 (free ingress); no Cloudflare path needed here.
	var manifestWriter *manifest.Writer
	var manifestCompactor *manifest.Compactor
	if manifestIdx != nil {
		uploader := func(ctx context.Context, key string, data []byte) error {
			return b2Backend.Put(ctx, cfg.Bucket, key, bytes.NewReader(data), int64(len(data)), nil)
		}
		manifestWriter = manifest.NewWriter(manifestIdx, cfg.ManifestPrefix, cfg.WriterID, uploader, 0)

		// Create compactor that lists and batch-deletes via the B2 backend.
		listerForCompactor := func(ctx context.Context, prefix, token string) ([]string, string, error) {
			result, lerr := b2Backend.ListRaw(ctx, cfg.Bucket, prefix, "", token, 1000)
			if lerr != nil {
				return nil, "", lerr
			}
			keys := make([]string, len(result.Objects))
			for i, obj := range result.Objects {
				keys[i] = obj.Key
			}
			return keys, result.NextToken, nil
		}
		deleter := func(ctx context.Context, keys []string) error {
			return b2Backend.DeleteObjects(ctx, cfg.Bucket, keys)
		}
		compactionInterval := time.Duration(cfg.ManifestCompactionInterval) * time.Second

		// Create a logger writer adapter that bridges the structured logger to
		// the standard library logger interface expected by the compactor.
		loggerWriter := &loggerWriter{logger: logger}
		compactorLogger := log.New(loggerWriter, "[manifest-compactor] ", log.LstdFlags|log.Lmsgprefix)

		manifestCompactor = manifest.NewCompactor(
			manifestIdx,
			cfg.ManifestPrefix,
			cfg.WriterID,
			uploader,
			listerForCompactor,
			deleter,
			compactionInterval,
			cfg.ManifestCompactionThreshold,
			compactorLogger,
			metrics.DefaultMetrics,
		)

		// Wire writer → compactor: after each delta flush, notify compactor.
		manifestWriter.SetOnFlush(manifestCompactor.NotifyDelta)

		manifestWriter.Start(context.Background())
		manifestCompactor.Start(context.Background())
		logger.Info("manifest writer and compactor started")
	}

	// Create and start the replication queue if secondary backend is configured (ADR-006)
	var replicationQueue *replication.ReplicationQueue
	if secondaryBackend != nil {
		replicationMetrics := replication.NewMetrics()
		// Use the same loggerWriter adapter for the replication queue
		replicationLoggerWriter := &loggerWriter{logger: logger}
		replicationLogger := log.New(replicationLoggerWriter, "[replication] ", log.LstdFlags|log.Lmsgprefix)
		replicationQueue = replication.NewReplicationQueue(
			replicationMetrics,
			b2Backend,
			secondaryBackend,
			0, // Use default buffer size
			replicationLogger,
		)
		logger.Info("replication queue initialized")
	}

	// Create canary monitor (after replicationQueue is available)
	canaryMonitor := canary.NewMonitor(canary.Config{
		Backend:           b2Backend,
		SecondaryBackend:  secondaryBackend,
		ReplicationQueue:  replicationQueue,
		Bucket:            cfg.Bucket,
		MEK:               cfg.MEK,
		BlockSize:         cfg.BlockSize,
		InstanceID:        cfg.WriterID,
		Interval:          5 * time.Minute,
		CanarySize:        1024,
		MaxRetries:        3,
		RetryDelay:        10 * time.Second,
		MultipartInterval: 1 * time.Hour,
		MultipartSize:     10*1024*1024 + 512*1024, // 10.5 MiB — two 5.25 MiB parts (B2 requires non-final parts >= 5 MiB)
		SecondaryInterval: 5 * time.Minute,
	})

	return &Server{
		config:            cfg,
		backend:           b2Backend,
		secondaryBackend:  secondaryBackend,
		cache:             cache,
		footerCache:       footerCache,
		listCache:         listCache,
		keyManager:        keyMgr,
		canary:            canaryMonitor,
		canaryDisabled:    cfg.CanaryDisabled,
		provenance:        provenanceMgr,
		presigner:         presigner,
		b2keys:            b2keysClient,
		dashboard:         dash,
		manifest:          manifestIdx,
		manifestWriter:    manifestWriter,
		manifestCompactor: manifestCompactor,
		replicationQueue:  replicationQueue,
		metrics:           metrics.DefaultMetrics,
		requestTracker:    metrics.DefaultRequestTracker,
		logger:            logger,
	}, nil
}

// manifestRecorder implements handlers.ManifestRecorder. It updates the
// in-memory index synchronously (so subsequent HeadObject calls see the new
// state immediately) and enqueues the op for async B2 delta persistence.
type manifestRecorder struct {
	idx    *manifest.Index
	writer *manifest.Writer
}

func (m *manifestRecorder) RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, blockSize int, contentType, etag string) {
	entry := &manifest.Entry{
		PlaintextSize:   size,
		PlaintextSHA256: sha256Hex,
		IV:              iv,
		WrappedDEK:      wrappedDEK,
		BlockSize:       blockSize,
		ContentType:     contentType,
		ETag:            etag,
		LastModified:    time.Now().UTC(),
	}
	m.idx.Put(bucket, key, entry)
	m.writer.EnqueuePut(bucket, key, entry)
}

func (m *manifestRecorder) RecordDelete(bucket, key string) {
	m.idx.Delete(bucket, key)
	m.writer.EnqueueDelete(bucket, key)
}

func (m *manifestRecorder) Lookup(bucket, key string) (*handlers.ManifestEntry, bool) {
	entry, ok := m.idx.Get(bucket, key)
	if !ok {
		return nil, false
	}
	return &handlers.ManifestEntry{
		PlaintextSize: entry.PlaintextSize,
		ContentType:   entry.ContentType,
		ETag:          entry.ETag,
		LastModified:  entry.LastModified,
		IV:            entry.IV,
		WrappedDEK:    entry.WrappedDEK,
		BlockSize:     entry.BlockSize,
	}, true
}

// StopManifestWriter flushes any pending manifest ops and stops the async writer.
func (s *Server) StopManifestWriter() {
	if s.manifestWriter != nil {
		s.manifestWriter.Stop()
		s.logger.Info("manifest writer stopped")
	}
}

// StopManifestCompactor stops the background compaction goroutine.
func (s *Server) StopManifestCompactor() {
	if s.manifestCompactor != nil {
		s.manifestCompactor.Stop()
		s.logger.Info("manifest compactor stopped")
	}
}

// StartCanary starts the canary monitor.
// It should be called after the server is created.
// When ARMOR_CANARY_DISABLED=true, the monitor is not started and /readyz
// always returns 200 regardless of B2 reachability.
func (s *Server) StartCanary(ctx context.Context) {
	if s.canary == nil || s.canaryStarted {
		return
	}
	if s.canaryDisabled {
		s.logger.Info("Canary monitor disabled (ARMOR_CANARY_DISABLED=true)")
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

// StartReplicationQueue starts the replication queue worker goroutine (ADR-006).
// It should be called after the server is created if a secondary backend is configured.
func (s *Server) StartReplicationQueue(ctx context.Context) {
	if s.replicationQueue != nil {
		s.replicationQueue.Start(ctx)
		s.logger.Info("Replication queue started")
	}
}

// StopReplicationQueue stops the replication queue worker goroutine (ADR-006).
// It drains the queue before shutdown, with a timeout for graceful shutdown.
func (s *Server) StopReplicationQueue() {
	if s.replicationQueue != nil {
		s.replicationQueue.Stop()
		s.logger.Info("Replication queue stopped")
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
	h := handlers.New(s.config, s.backend, s.cache, s.footerCache, s.keyManager, s.listCache)

	// Wire up provenance if available
	if s.provenance != nil {
		h.WithProvenance(s.provenance)
	}

	// Wire up manifest recorder if enabled
	if s.manifest != nil && s.manifestWriter != nil {
		h.WithManifest(&manifestRecorder{idx: s.manifest, writer: s.manifestWriter})
	}

	// Wire up secondary backend for async replication if configured (ADR-006).
	// When unset, secondaryBackend is nil and replication is a no-op.
	if s.secondaryBackend != nil {
		h.WithSecondaryBackend(s.secondaryBackend)
	}

	// Wire up replication queue if configured (ADR-006).
	// When unset, replicationQueue is nil and replication is a no-op.
	if s.replicationQueue != nil {
		h.WithReplicationQueue(s.replicationQueue)
	}

	// Bucket operations
	mux.HandleFunc("/", s.wrapHandler(h.HandleRoot))

	// Apply request ID middleware to all S3 responses
	return middleware.RequestID(mux)
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
	mux.HandleFunc("/admin/b2/keys", s.handleB2Keys)       // GET=List, POST=Create
	mux.HandleFunc("/admin/b2/keys/", s.handleB2KeyDelete) // DELETE=Delete by ID
	mux.HandleFunc("/metrics", s.metrics.Handler())

	// Dashboard routes (authenticated via DashboardUser/Pass or DashboardToken)
	if s.dashboard != nil {
		mux.HandleFunc("/dashboard", s.dashboard.HandlerWithAuth())
		mux.HandleFunc("/dashboard/", s.dashboard.HandlerWithAuth()) // For prefix navigation
		mux.HandleFunc("/dashboard/object", s.dashboard.ObjectDetailHandlerWithAuth())
		mux.HandleFunc("/dashboard/metrics", s.dashboard.MetricsHandlerWithAuth())
		mux.HandleFunc("/dashboard/encryption-stats", s.dashboard.EncryptionStatsHandlerWithAuth())
		mux.HandleFunc("/dashboard/api/list", s.dashboard.ListAPIHandlerWithAuth())

		// Key rotation proxy handler (authenticated).
		// The dashboard proxies rotation to the admin API over loopback; it must
		// present the admin token now that /admin/key/rotate is token-gated.
		adminClient := &http.Client{
			Timeout: 30 * time.Minute, // Key rotation can take a long time
		}
		adminURL := "http://" + s.config.AdminListen + "/admin/key/rotate"
		mux.HandleFunc("/dashboard/admin/key/rotate", s.dashboard.KeyRotateHandlerWithAuth(adminClient, adminURL, s.config.AdminToken))
		mux.HandleFunc("/dashboard/admin/key/status", s.dashboard.KeyRotateStatusHandlerWithAuth())
	}

	// Gate every non-public admin route behind ARMOR_ADMIN_TOKEN and audit-log
	// each call. Public probe/scrape/dashboard paths are passed through.
	return s.adminAuthMiddleware(mux)
}

// healthz returns the health status.
// This is the liveness probe — it only checks that the process is alive.
// Backend connectivity is checked by /readyz (the readiness probe).
func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// readyz returns the readiness status.
// When the canary monitor is running and not disabled, its in-memory health
// state is the sole signal — no backend call is made. When the canary is
// disabled (ARMOR_CANARY_DISABLED=true), /readyz always returns 200 and the
// liveness probe (/healthz) is the sole health guard. When the canary is not
// configured, the manifest writer's last flush is used as the health signal.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	// When canary is explicitly disabled, skip all backend checks.
	if s.canaryDisabled {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
		return
	}

	// Canary monitor is authoritative when running.
	if s.canary != nil && s.canaryStarted {
		if !s.canary.IsHealthy() {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Not ready - canary check failed"))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ready"))
		return
	}

	// Fallback: manifest writer's last successful delta flush.
	// A flush within the last 60 seconds indicates the service is healthy.
	const flushThreshold = 60 * time.Second
	if s.manifestWriter != nil {
		lastFlush := s.manifestWriter.LastFlush()
		if !lastFlush.IsZero() && time.Since(lastFlush) < flushThreshold {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Ready"))
			return
		}
		// No recent flush — report unhealthy
		w.WriteHeader(http.StatusServiceUnavailable)
		if lastFlush.IsZero() {
			w.Write([]byte("Not ready - manifest writer has never flushed"))
		} else {
			fmt.Fprintf(w, "Not ready - manifest writer last flush %v ago (threshold %v)", time.Since(lastFlush).Round(time.Second), flushThreshold)
		}
		return
	}

	// Neither canary nor manifest writer available — report unhealthy.
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte("Not ready - no health signal available"))
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

	// Select the key to rotate. The default key remains the backward-compatible
	// choice when no key-id is supplied; named keys are rotated independently.
	keyID := strings.TrimSpace(r.URL.Query().Get("key-id"))
	key, err := s.keyManager.GetKeyByID(keyID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Unknown key ID %q: %v", keyID, err), http.StatusBadRequest)
		return
	}
	keyID = key.Name

	// Pass the manifest index so the rotator can use cached metadata where
	// possible. Per-key filtering still verifies the authoritative object
	// metadata before selecting an object.
	oldMEK := key.MEK

	// Compute MEK hashes for provenance tracking
	oldMEKHashBytes := sha256.Sum256(oldMEK)
	newMEKHashBytes := sha256.Sum256(newMEK)
	oldMEKHash := hex.EncodeToString(oldMEKHashBytes[:8])
	newMEKHash := hex.EncodeToString(newMEKHashBytes[:8])

	// Generate rotation ID for provenance tracking
	rotationID := fmt.Sprintf("%s-%s-%d", oldMEKHash, newMEKHash, time.Now().Unix())

	// Record key rotation start in provenance chain
	if err := s.provenance.RecordKeyEvent(r.Context(), "key-rotate-start", provenance.KeyEventOpts{
		OldMEKHash: oldMEKHash,
		NewMEKHash: newMEKHash,
		RotationID: rotationID,
	}); err != nil {
		s.logger.WithFields(map[string]interface{}{
			"error":       err.Error(),
			"rotation_id": rotationID,
		}).Warn("failed to record key rotation start event in provenance chain")
	}

	rotator := NewKeyRotatorForKey(s.backend, s.config.Bucket, keyID, oldMEK, newMEK, s.manifest)

	// Perform rotation
	result, err := rotator.Rotate(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "failed",
			"error":  err.Error(),
			"result": result,
		})
		return
	}

	// Update the server's MEK on success
	if result.Status == "completed" {
		if err := s.keyManager.UpdateKey(keyID, newMEK); err != nil {
			s.logger.WithFields(map[string]interface{}{
				"error": err.Error(),
			}).Error("failed to update key manager after rotation")
		}
		// Clear the metadata cache since DEKs are now wrapped with new MEK
		s.cache.Clear()

		// Record key rotation complete in provenance chain
		rotationResult := &provenance.KeyRotationResult{
			TotalObjects:     result.TotalObjects,
			ProcessedObjects: result.ProcessedObjects,
			SkippedObjects:   result.SkippedObjects,
			Exceptions:       result.Exceptions,
			DurationSec:      result.Duration.Seconds(),
			Status:           result.Status,
		}
		if err := s.provenance.RecordKeyEvent(r.Context(), "key-rotate-complete", provenance.KeyEventOpts{
			OldMEKHash:     oldMEKHash,
			NewMEKHash:     newMEKHash,
			RotationID:     rotationID,
			RotationResult: rotationResult,
		}); err != nil {
			s.logger.WithFields(map[string]interface{}{
				"error":       err.Error(),
				"rotation_id": rotationID,
			}).Warn("failed to record key rotation complete event in provenance chain")
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
}

// exportKey exports the current MEK along with B2 credentials and configuration
// for self-contained break-glass recovery.
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

	// Compute MEK hash for provenance tracking
	exportedMEKHashBytes := sha256.Sum256(defaultKey.MEK)
	exportedMEKHash := hex.EncodeToString(exportedMEKHashBytes[:8])

	// Record key export in provenance chain
	if err := s.provenance.RecordKeyEvent(r.Context(), "key-export", provenance.KeyEventOpts{
		ExportedMEKHash: exportedMEKHash,
	}); err != nil {
		s.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Warn("failed to record key export event in provenance chain")
	}

	// Export complete escrow package: MEK + B2 credentials + B2 config
	// This ensures recovery is self-contained without relying on K8s ConfigMaps
	escrowPackage := map[string]interface{}{
		"mek": hex.EncodeToString(defaultKey.MEK),
		"b2": map[string]string{
			"region":       s.config.B2Region,
			"endpoint":     s.config.B2Endpoint,
			"access_key":   s.config.B2AccessKeyID,
			"secret_key":   s.config.B2SecretAccessKey,
			"bucket":       s.config.Bucket,
		},
		"format":  "hex",
		"warning": "This package provides access to all encrypted data. Store securely.",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(escrowPackage)
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

	w.Header().Set("Content-Type", "application/json")

	// Create auditor and perform audit
	auditor := provenance.NewAuditorWithPrefix(s.backend, s.config.Bucket, s.config.Prefix)
	result, err := auditor.Audit(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"error":  err.Error(),
		})
		return
	}

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
				if authErr, ok := err.(*AuthError); ok {
					s.writeError(w, r, authErr.Code, authErr.Message, 403)
				} else {
					s.writeError(w, r, "AccessDenied", "Invalid credentials", 403)
				}
				s.metrics.IncRequestsTotal("auth", 403)
				return
			}
			// Check ACL for the request. For list operations the URL path has no
			// key component, so fall back to the ?prefix query param so that ACL
			// prefix restrictions are enforced correctly against the listed prefix.
			bucket, key := s.extractBucketAndKey(r)
			if key == "" {
				key = r.URL.Query().Get("prefix")
			}
			if err := CheckACL(cred, bucket, key, ActionForRequest(r)); err != nil {
				s.writeError(w, r, "AccessDenied", "Access Denied", 403)
				s.metrics.IncRequestsTotal("acl", 403)
				return
			}
		}

		// Decode aws-chunked body when MinIO streaming signature is used.
		// The seed signature in the Authorization header already authenticated the
		// request; we only need to strip the chunk framing to recover the payload.
		if r.Header.Get("X-Amz-Content-Sha256") == "STREAMING-AWS4-HMAC-SHA256-PAYLOAD" {
			if decoded := r.Header.Get("X-Amz-Decoded-Content-Length"); decoded != "" {
				if n, err := strconv.ParseInt(decoded, 10, 64); err == nil {
					r.ContentLength = n
				}
			}
			r.Body = newAWSChunkedReader(r.Body)
		}

		// Use a response writer wrapper to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		h(rw, r)

		// Record metrics
		duration := time.Since(start)
		s.metrics.IncRequestsTotal(r.Method, rw.statusCode)
		s.metrics.RecordRequestDuration(r.Method, duration)

		// Log completed request
		fields := map[string]interface{}{
			"method":      r.Method,
			"path":        r.URL.Path,
			"status":      rw.statusCode,
			"duration_ms": duration.Milliseconds(),
		}
		if rng := r.Header.Get("Range"); rng != "" {
			fields["range"] = rng
		}
		s.logger.WithFields(fields).Info("request completed")
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

	// Pass nil body — buildCanonicalRequest uses x-amz-content-sha256 header
	// when present (which covers all AWS SDK/CLI requests), falling back to
	// sha256(body) for requests without that header.
	return auth.VerifyRequest(r, nil)
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
		// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
		if decoded, err := url.PathUnescape(key); err == nil {
			key = decoded
		}
	}

	// Use configured bucket if empty
	if bucket == "" {
		bucket = s.config.Bucket
	}

	return bucket, key
}

// writeError writes an S3 error response with request ID.
// The request ID is extracted from the request context and included in both
// the response headers (x-amz-request-id, x-amz-id-2) and the XML body (<RequestId>).
func (s *Server) writeError(w http.ResponseWriter, r *http.Request, code, message string, statusCode int) {
	// Extract request IDs from context
	requestID := middleware.GetRequestID(r.Context())
	extendedID := middleware.GetExtendedID(r.Context())

	// Set headers
	w.Header().Set("Content-Type", "application/xml")
	if requestID != "" {
		w.Header().Set("x-amz-request-id", requestID)
	}
	if extendedID != "" {
		w.Header().Set("x-amz-id-2", extendedID)
	}
	w.WriteHeader(statusCode)

	// Build XML response
	var codeBuf, msgBuf, ridBuf bytes.Buffer
	xml.EscapeText(&codeBuf, []byte(code))
	xml.EscapeText(&msgBuf, []byte(message))
	xml.EscapeText(&ridBuf, []byte(requestID))

	if requestID != "" {
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n  <RequestId>%s</RequestId>\n</Error>",
			codeBuf.String(), msgBuf.String(), ridBuf.String())
	} else {
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n</Error>",
			codeBuf.String(), msgBuf.String())
	}
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
		// Return specific authentication error code and message
		if authErr, ok := err.(*AuthError); ok {
			s.writeError(w, r, authErr.Code, authErr.Message, 403)
		} else {
			s.writeError(w, r, "AccessDenied", "Invalid credentials", 403)
		}
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

	// Check ACL for the request. Presigning mints a download (GET) URL, so the
	// action verb under test is "get" — the caller must be permitted to read.
	if err := CheckACL(cred, bucket, req.Key, ActionGet); err != nil {
		s.writeError(w, r, "AccessDenied", "Access Denied", 403)
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
		"url":        shareURL,
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

	// Detect and log compression status
	s.logger.WithFields(map[string]interface{}{
		"bucket":     token.Bucket,
		"key":        token.Key,
		"compressed": armorMeta.Compressed,
	}).Debug("share/range request: compression status detected")

	// Set response headers (Content-Length will be set by handleShareFullObject after decompression)
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

	// Track whether compression has been checked from first block
	checkedCompression := false

	// Early return for empty objects (plaintextSize = 0)
	// This prevents returning the envelope header instead of an empty body
	if plaintextSize == 0 {
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
		return
	}

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

	// 4. Collect all decrypted blocks into a buffer for decompression
	allDecrypted := make([]byte, 0, plaintextSize)
	encryptedBuf := make([]byte, blockSize)
	for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
		remaining := plaintextSize - int64(blockIndex)*int64(blockSize)
		actualBlockSize := int(min64(int64(blockSize), remaining))

		encryptedBuf = encryptedBuf[:actualBlockSize]
		n, err := io.ReadFull(dataBody, encryptedBuf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			s.logger.WithFields(map[string]interface{}{
				"bucket": token.Bucket,
				"key":    token.Key,
				"error":  err.Error(),
			}).Error("share full object: failed to read encrypted block")
			http.Error(w, fmt.Sprintf("Failed to read encrypted block: %v", err), http.StatusInternalServerError)
			return
		}
		if n == 0 {
			break
		}
		encryptedBuf = encryptedBuf[:n]

		// Verify HMAC
		hmacOffset := blockIndex * crypto.HMACSize
		if hmacOffset+crypto.HMACSize > len(hmacTable) {
			s.logger.WithFields(map[string]interface{}{
				"bucket":           token.Bucket,
				"key":              token.Key,
				"hmac_offset":      hmacOffset,
				"hmac_table_size":  len(hmacTable),
			}).Error("share full object: HMAC table bounds check failed")
			http.Error(w, "HMAC table bounds check failed", http.StatusInternalServerError)
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
			s.logger.WithFields(map[string]interface{}{
				"bucket":     token.Bucket,
				"key":        token.Key,
				"blockIndex": blockIndex,
			}).Warn("share full object: HMAC verification failed - data may be corrupted or tampered")
			http.Error(w, "HMAC verification failed - data integrity check failed", http.StatusBadRequest)
			return
		}

		// Decrypt
		decrypted := make([]byte, n)
		ctr := makeCounter(armorMeta.IV, uint32(blockIndex), armorMeta.Version, blockSize)
		stream := cipher.NewCTR(decryptor.CipherBlock(), ctr)
		stream.XORKeyStream(decrypted, encryptedBuf)

		// Append to buffer
		allDecrypted = append(allDecrypted, decrypted...)

		// Check compression from first decrypted block (contains zstd magic if compressed)
		if blockIndex == 0 && !checkedCompression {
			isCompressed := crypto.IsCompressed(decrypted)
			s.logger.WithFields(map[string]interface{}{
				"bucket":              token.Bucket,
				"key":                 token.Key,
				"compressed_meta":     armorMeta.Compressed,
				"compressed_detected": isCompressed,
			}).Debug("share full object: compression status detected (post-decrypt first block)")
			checkedCompression = true
		}
	}

	// 5. Decompress if the data is compressed
	finalData := allDecrypted
	if armorMeta.Compressed {
		s.logger.WithFields(map[string]interface{}{
			"bucket": token.Bucket,
			"key":    token.Key,
		}).Debug("share full object: decompressing zstd data")

		decompressed, err := crypto.Decompress(allDecrypted)
		if err != nil {
			// Classify the error to determine appropriate HTTP status code
			var decompErr *crypto.DecompressionError
			if errors.As(err, &decompErr) {
				// Log the corruption with metadata
				s.logger.WithFields(map[string]interface{}{
					"bucket":         token.Bucket,
					"key":            token.Key,
					"corruption_type": decompErr.Cause,
					"error_type":     "client",
				}).Warn("share full object: corrupted compressed data detected")

				// Client-side data integrity issue: 400 Bad Request
				http.Error(w, fmt.Sprintf("Failed to decompress data: %v (corruption type: %s)", err, decompErr.Cause), http.StatusBadRequest)
				return
			}

			// Server-side infrastructure issue: 500 Internal Server Error
			s.logger.WithFields(map[string]interface{}{
				"bucket": token.Bucket,
				"key":    token.Key,
				"error":  err.Error(),
			}).Error("share full object: decompression infrastructure error")

			http.Error(w, fmt.Sprintf("Failed to decompress data: %v", err), http.StatusInternalServerError)
			return
		}
		finalData = decompressed
	}

	// 6. Set Content-Length and write status
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(finalData)))
	w.WriteHeader(http.StatusOK)
	w.Write(finalData)
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

	// Fail-closed: range requests over compressed objects are not supported
	// Compression destroys fixed-offset seeking (zstd is variable-length encoding),
	// so byte ranges into compressed ciphertext would return corrupt data.
	if armorMeta.Compressed {
		s.logger.WithFields(map[string]interface{}{
			"bucket":     token.Bucket,
			"key":        token.Key,
			"range":      rangeHeader,
			"compressed": armorMeta.Compressed,
		}).Warn("share/range request rejected: range reads unsupported on compressed objects")
		http.Error(w, "Range reads unsupported on compressed objects", http.StatusRequestedRangeNotSatisfiable)
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
	plaintext, err := decryptor.DecryptRange(encrypted, hmacTable, start, end, plaintextSize, false)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to decrypt range: %v", err), http.StatusInternalServerError)
		return
	}

	// Detect compression from decrypted plaintext
	isCompressed := crypto.IsCompressed(plaintext)

	// Detect and log compression status
	s.logger.WithFields(map[string]interface{}{
		"bucket":            token.Bucket,
		"key":               token.Key,
		"range":             rangeHeader,
		"compressed_meta":   armorMeta.Compressed,
		"compressed_detected": isCompressed,
	}).Debug("share/range request: compression status detected (post-decrypt)")

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
// Version 1 (legacy, vulnerable): counter_value = blockIndex
// Version 2 (fixed): counter_value = blockIndex * (blockSize / 16)
//
// The version must match the encryption version to decrypt correctly.
func makeCounter(iv []byte, blockIndex uint32, version int, blockSize int) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], iv[0:12])

	var counterValue uint32
	if version == 2 {
		// Version2: stride by number of AES blocks per ARMOR block
		aesBlocksPerArmorBlock := uint32(blockSize / 16)
		counterValue = blockIndex * aesBlocksPerArmorBlock
	} else {
		// Version1: legacy (buggy) derivation for backward compatibility
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
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

// handleB2Keys handles B2 application key management.
// GET: List keys
// POST: Create a new key
func (s *Server) handleB2Keys(w http.ResponseWriter, r *http.Request) {
	if s.b2keys == nil {
		http.Error(w, `{"error":"B2 key management not available - check B2 credentials"}`, http.StatusServiceUnavailable)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.listB2Keys(w, r)
	case http.MethodPost:
		s.createB2Key(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listB2Keys lists B2 application keys.
func (s *Server) listB2Keys(w http.ResponseWriter, r *http.Request) {
	count := 100
	if c := r.URL.Query().Get("count"); c != "" {
		if parsed, err := strconv.Atoi(c); err == nil && parsed > 0 {
			count = parsed
		}
	}
	cursor := r.URL.Query().Get("cursor")

	result, err := s.b2keys.ListKeys(r.Context(), count, cursor)
	if err != nil {
		s.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
		}).Error("Failed to list B2 keys")
		http.Error(w, fmt.Sprintf(`{"error":"Failed to list keys: %v"}`, err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// createB2Key creates a new B2 application key.
func (s *Server) createB2Key(w http.ResponseWriter, r *http.Request) {
	var req b2keys.CreateKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"Invalid request body: %v"}`, err), http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, `{"error":"name is required"}`, http.StatusBadRequest)
		return
	}

	if len(req.Capabilities) == 0 {
		http.Error(w, `{"error":"capabilities is required"}`, http.StatusBadRequest)
		return
	}

	key, err := s.b2keys.CreateKey(r.Context(), &req)
	if err != nil {
		s.logger.WithFields(map[string]interface{}{
			"error": err.Error(),
			"name":  req.Name,
		}).Error("Failed to create B2 key")
		http.Error(w, fmt.Sprintf(`{"error":"Failed to create key: %v"}`, err), http.StatusInternalServerError)
		return
	}

	s.logger.WithFields(map[string]interface{}{
		"key_id": key.ID,
		"name":   key.Name,
	}).Info("Created B2 application key")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(key)
}

// handleB2KeyDelete handles DELETE /admin/b2/keys/{id}.
func (s *Server) handleB2KeyDelete(w http.ResponseWriter, r *http.Request) {
	if s.b2keys == nil {
		http.Error(w, `{"error":"B2 key management not available - check B2 credentials"}`, http.StatusServiceUnavailable)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract key ID from path: /admin/b2/keys/{id}
	keyID := strings.TrimPrefix(r.URL.Path, "/admin/b2/keys/")
	if keyID == "" || keyID == r.URL.Path {
		http.Error(w, `{"error":"key ID is required"}`, http.StatusBadRequest)
		return
	}

	err := s.b2keys.DeleteKey(r.Context(), keyID)
	if err != nil {
		if errors.Is(err, b2keys.ErrKeyNotFound) {
			http.Error(w, `{"error":"key not found"}`, http.StatusNotFound)
			return
		}
		s.logger.WithFields(map[string]interface{}{
			"error":  err.Error(),
			"key_id": keyID,
		}).Error("Failed to delete B2 key")
		http.Error(w, fmt.Sprintf(`{"error":"Failed to delete key: %v"}`, err), http.StatusInternalServerError)
		return
	}

	s.logger.WithFields(map[string]interface{}{
		"key_id": keyID,
	}).Info("Deleted B2 application key")

	w.WriteHeader(http.StatusNoContent)
}
