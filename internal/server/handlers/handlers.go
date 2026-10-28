// Package handlers implements S3 operation handlers for ARMOR.
package handlers

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/sync/errgroup"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/metrics"
	"github.com/jedarden/armor/internal/provenance"
	"github.com/jedarden/armor/internal/replication"
	"github.com/jedarden/armor/internal/server/middleware"
	"github.com/jedarden/armor/internal/server/s3errors"
)

// ProvenanceRecorder records uploads in the provenance chain.
type ProvenanceRecorder interface {
	RecordUpload(ctx context.Context, objectKey, plaintextSHA256, operation string) error
	ShouldRecord(key string) bool
	// CreateChainEntry creates a chain entry for embedding in a manifest delta.
	// It atomically increments the sequence number and computes the chain hash,
	// but does not write the entry to B2. Returns (nil, nil) if the key should
	// not have provenance recorded (e.g., internal objects).
	CreateChainEntry(ctx context.Context, objectKey, plaintextSHA256, operation string) (*provenance.ChainEntryData, error)
}

// ManifestEntry holds the decryption metadata for a tracked object as exposed
// to handlers. It mirrors manifest.Entry but is defined here to avoid an
// import cycle between the handlers and manifest packages.
type ManifestEntry struct {
	PlaintextSize  int64
	ContentType    string
	ETag           string
	LastModified   time.Time
	IV             []byte
	WrappedDEK     []byte
	BlockSize      int
	CiphertextSize int64 // v3 single-PUT readers need this to locate trailer block table
	// UserMetadata carries the client-supplied x-amz-meta-* keys so the
	// HeadObject index fast path can serve them for single-PUT objects,
	// which have no per-key manifest object to enrich from.
	UserMetadata map[string]string
}

// ManifestRecorder records successful S3 write operations in the manifest
// index for fast in-memory lookup and enqueues them for async B2 persistence
// as delta files. Implementations must be safe for concurrent use.
type ManifestRecorder interface {
	// RecordPut records a successful PutObject or CompleteMultipartUpload.
	// chainEntry is the provenance chain entry to embed in the delta line.
	// May be nil if provenance is disabled for this key. userMetadata is the
	// client-supplied x-amz-meta-* subset (ARMOR's reserved namespace
	// excluded) that HeadObject's index fast path serves back.
	RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, mekFingerprint string, blockSize int, contentType, etag string, chainEntry *manifest.ChainEntry, ciphertextSize int64, userMetadata map[string]string)
	// RecordDelete records a successful DeleteObject.
	RecordDelete(bucket, key string)
	// Lookup returns manifest metadata for bucket/key, or (nil, false) if not
	// tracked. Used to serve HeadObject and ListObjectVersions from memory
	// without a B2 round-trip.
	Lookup(bucket, key string) (*ManifestEntry, bool)
}

// Handlers contains all S3 operation handlers.
type Handlers struct {
	config           *config.Config
	backend          backend.Backend
	secondaryBackend backend.Backend // Secondary backend for async replication (ADR-006); nil when unset (no-op)
	cache            *backend.MetadataCache
	footerCache      *backend.FooterCache
	listCache        *backend.ListCache
	keyManager       *keymanager.KeyManager
	provenance       ProvenanceRecorder
	manifest         ManifestRecorder
	metrics          *metrics.Metrics
	replicationQueue replication.Enqueuer
	logger           *logging.Logger // Structured logger for S3 error events

	// multipartLocks protects multipart state updates. Legacy v2 parts share one
	// read-modify-write state object and therefore lock by upload ID. V3 stores
	// each part in an independent part-<n>.json object, so it locks only duplicate
	// requests for the same part number; distinct v3 parts remain concurrent.
	// Registry entries stay alive while a part holds or waits for its lock.
	multipartLocksMu   sync.Mutex
	multipartLocks     map[string]*multipartLockEntry
	multipartLastSweep time.Time

	// multipartSidecarCache caches v3 multipart sidecar data
	multipartSidecarCache *backend.MultipartSidecarCache

	// v3Tables caches decoded v3 trailer block tables, keyed by stored-object
	// generation, with in-flight load coalescing and entry/byte bounds.
	v3Tables *v3BlockTableCache

	// smallFetchBufs bounds the number of small single-PUT objects whose whole
	// ciphertext is buffered at once (armor-98864f12). Each slot covers one
	// in-flight buffered GET from fetch to completed response, so aggregate
	// buffered ciphertext never exceeds cap × SmallObjectFetchBytes. A request
	// that finds the cap full takes the streaming path instead of waiting.
	smallFetchBufs chan struct{}

	// partBudget bounds the aggregate plaintext bytes concurrent v3
	// UploadPart requests hold in memory (armor-cb2b0731). Initialized
	// lazily so every construction path — including zero-valued configs in
	// tests — gets the same default.
	partBudgetOnce sync.Once
	partBudget     *partMemoryBudget
}

const (
	multipartLockIdleTTL       = 24 * time.Hour
	multipartLockSweepInterval = time.Hour
)

type multipartLockEntry struct {
	mu       sync.Mutex
	refs     int // Holders and waiters; protected by multipartLocksMu.
	lastUsed time.Time
	retired  bool
}

// multipartLock registers a waiter before taking the per-key mutex. Cleanup
// cannot replace that mutex while another request is still using it.
func (h *Handlers) multipartLock(lockKey string) func() {
	now := time.Now()
	h.multipartLocksMu.Lock()
	if h.multipartLocks == nil {
		h.multipartLocks = make(map[string]*multipartLockEntry)
	}
	if now.Sub(h.multipartLastSweep) >= multipartLockSweepInterval {
		h.sweepMultipartLocksLocked(now)
		h.multipartLastSweep = now
	}
	entry := h.multipartLocks[lockKey]
	if entry == nil {
		entry = &multipartLockEntry{}
		h.multipartLocks[lockKey] = entry
	}
	entry.refs++
	h.multipartLocksMu.Unlock()

	entry.mu.Lock()
	return func() {
		entry.mu.Unlock()
		h.multipartLocksMu.Lock()
		entry.refs--
		entry.lastUsed = time.Now()
		if entry.refs == 0 && entry.retired {
			delete(h.multipartLocks, lockKey)
		}
		h.multipartLocksMu.Unlock()
	}
}

// cleanupMultipartLocks retires both the v2 upload lock and all v3 part
// locks. Active holders and waiters keep their mutex until the last release.
func (h *Handlers) cleanupMultipartLocks(uploadID string) {
	h.multipartLocksMu.Lock()
	defer h.multipartLocksMu.Unlock()
	for key, entry := range h.multipartLocks {
		if key != uploadID && !strings.HasPrefix(key, uploadID+"/part/") {
			continue
		}
		if entry.refs == 0 {
			delete(h.multipartLocks, key)
		} else {
			entry.retired = true
		}
	}
}

// sweepMultipartLocksLocked reclaims idle locks from abandoned uploads. It is
// run on the next multipart lock acquisition after each sweep interval.
func (h *Handlers) sweepMultipartLocksLocked(now time.Time) {
	for key, entry := range h.multipartLocks {
		if entry.refs == 0 && now.Sub(entry.lastUsed) >= multipartLockIdleTTL {
			delete(h.multipartLocks, key)
		}
	}
}

// getMultipartManager returns the multipart state manager for a given bucket.
// The manager is created on first use per bucket and resolves the internal
// .armor/ namespace beneath the ADR-001 tenant prefix when one is configured,
// so multipart state and HMAC sidecars land inside the tenant's namespace —
// a B2 key scoped to namePrefix <tenant>/ is denied bucket-root writes.
func (h *Handlers) getMultipartManager(bucket string) *backend.MultipartStateManager {
	return backend.NewMultipartStateManager(h.backend, bucket).WithKeyPrefix(h.config.Prefix)
}

// New creates a new Handlers instance.
func New(cfg *config.Config, be backend.Backend, cache *backend.MetadataCache, footerCache *backend.FooterCache, km *keymanager.KeyManager, listCache *backend.ListCache) *Handlers {
	// The small-fetch buffer cap rides the backend's own ranged-request
	// budget: one buffered small fetch is one ranged request, so the same
	// limit that bounds in-flight backend requests bounds live buffers. A
	// zero-valued config (direct Handlers construction in tests) falls back
	// to the same default the backend applies for ReadBackendLimit.
	bufCap := cfg.ReadBackendLimit
	if bufCap < 1 {
		bufCap = 64
	}
	return &Handlers{
		config:                cfg,
		backend:               be,
		cache:                 cache,
		footerCache:           footerCache,
		listCache:             listCache,
		keyManager:            km,
		provenance:            nil,
		manifest:              nil,
		multipartSidecarCache: backend.NewMultipartSidecarCache(cfg.CacheMaxEntries, cfg.CacheTTL),
		v3Tables:              newV3BlockTableCache(cfg.CacheMaxEntries, cfg.CacheTTL, 0),
		smallFetchBufs:        make(chan struct{}, bufCap),
	}
}

// WithProvenance adds provenance support to handlers.
func (h *Handlers) WithProvenance(p ProvenanceRecorder) {
	h.provenance = p
}

// defaultUploadPartMemoryBudget is the aggregate in-memory v3 UploadPart
// plaintext budget when the config does not set one: eight pooled 64 MiB
// parts, the iad-ci workload the budget was sized against (armor-cb2b0731).
const defaultUploadPartMemoryBudget = 512 << 20

// partMemoryBudget returns the server's aggregate v3 part memory budget,
// creating it on first use from the configured total (zero-valued configs,
// including direct Handlers construction in tests, take the default).
func (h *Handlers) partMemoryBudget() *partMemoryBudget {
	h.partBudgetOnce.Do(func() {
		total := int64(defaultUploadPartMemoryBudget)
		if h.config != nil && h.config.UploadPartMemoryBudget > 0 {
			total = h.config.UploadPartMemoryBudget
		}
		h.partBudget = newPartMemoryBudget(total)
	})
	return h.partBudget
}

// recordProvenanceUpload records a per-object provenance entry after the
// object is durable. A failed append leaves an audit gap; the S3 write still
// succeeds, while a warning and counter make the gap observable.
func (h *Handlers) recordProvenanceUpload(ctx context.Context, key, plaintextSHA256, operation string) {
	if err := h.provenance.RecordUpload(ctx, key, plaintextSHA256, operation); err != nil {
		if h.logger != nil {
			h.logger.WithFields(map[string]interface{}{
				"event":     "provenance_record_failure",
				"operation": operation,
				"key":       key,
				"error":     err.Error(),
			}).Warn("provenance RecordUpload failed; audit chain gap for this key")
		} else {
			log.Printf("WARN provenance RecordUpload failed (operation=%s key=%q error=%v): audit chain gap", operation, key, err)
		}
		if h.metrics != nil {
			h.metrics.IncProvenanceRecordFailures()
		}
	}
}

// WithManifest wires a ManifestRecorder into the handlers so that successful
// PutObject and DeleteObject calls update the in-memory index and enqueue
// delta ops for async B2 persistence.
func (h *Handlers) WithManifest(m ManifestRecorder) {
	h.manifest = m
}

// WithSecondaryBackend wires an optional secondary backend into the handlers
// for async replication (ADR-006). When ARMOR_SECONDARY_BACKEND_TYPE is unset
// the server passes nil and this method is never called, leaving
// secondaryBackend nil and replication a complete no-op — no handler touches
// the secondary backend unless a non-nil one is wired in. The backend is
// constructed once by the server from configuration and injected here, the
// same pattern used for the primary backend in New.
func (h *Handlers) WithSecondaryBackend(be backend.Backend) {
	h.secondaryBackend = be
}

// WithMetrics wires the metrics instance into the handlers.
func (h *Handlers) WithMetrics(m *metrics.Metrics) {
	h.metrics = m
	// The sidecar cache is built in New, before a metrics instance is
	// available; wire its LRU counters here so its lookups and evictions
	// publish alongside the server-owned caches' (armor-a4e5356d).
	if h.multipartSidecarCache != nil {
		h.multipartSidecarCache.WithMetrics(m)
	}
}

// WithReplicationQueue wires the replication queue into the handlers.
// When ARMOR_SECONDARY_BACKEND is configured, the server passes a non-nil
// queue and this method is called, leaving replicationQueue ready to enqueue
// tasks after successful PutObject operations. When unconfigured, the server
// passes nil and this method is never called, leaving replicationQueue nil and
// replication a complete no-op.
func (h *Handlers) WithReplicationQueue(q replication.Enqueuer) {
	h.replicationQueue = q
}

// WithLogger wires the structured logger into the handlers.
func (h *Handlers) WithLogger(logger *logging.Logger) {
	h.logger = logger
}

// HandleRoot routes S3 operations based on the request.
func (h *Handlers) HandleRoot(w http.ResponseWriter, r *http.Request) {
	// Parse bucket and key from the URL path. Routing below keys off the name
	// the client actually sent (an empty one means a root ListBuckets), while
	// the handlers receive the bucket ARMOR serves the request from so that a
	// legacy name in the URL reaches the objects that now live under
	// config.Bucket. See config.ResolveBucket.
	//
	// The two must not be conflated: ResolveBucket maps an empty name onto
	// config.Bucket, so a root ListBuckets would otherwise look like a
	// bucket-level request for the configured bucket and list its objects
	// instead of the bucket list.
	parsed := config.ParseBucketKey(r.URL.Path)
	named := parsed.Bucket
	key := parsed.Key
	bucket := h.config.ResolveBucket(named)

	// Protect the .armor/ reserved namespace
	// Client operations targeting keys with this prefix return 403 AccessDenied
	if strings.HasPrefix(key, ".armor/") {
		h.writeError(w, r, "AccessDenied", "Access to .armor/ reserved namespace is denied", 403)
		return
	}

	// Route based on method and path
	switch r.Method {
	case http.MethodGet:
		// Handle ListMultipartUploads (GET ?uploads on bucket, no key)
		if r.URL.Query().Has("uploads") && key == "" && named != "" {
			h.ListMultipartUploads(w, r, bucket)
			return
		}
		// Handle ListObjectVersions (GET ?versions on bucket)
		if r.URL.Query().Has("versions") && key == "" && named != "" {
			h.ListObjectVersions(w, r, bucket)
			return
		}
		// Handle GetBucketLifecycleConfiguration (GET ?lifecycle on bucket)
		if r.URL.Query().Has("lifecycle") && key == "" && named != "" {
			h.GetBucketLifecycleConfiguration(w, r, bucket)
			return
		}
		// Handle GetObjectLockConfiguration (GET ?object-lock on bucket)
		if r.URL.Query().Has("object-lock") && key == "" && named != "" {
			h.GetObjectLockConfiguration(w, r, bucket)
			return
		}
		// Handle GetObjectRetention (GET ?retention on object)
		if r.URL.Query().Has("retention") && key != "" {
			h.GetObjectRetention(w, r, bucket, key)
			return
		}
		// Handle GetObjectLegalHold (GET ?legal-hold on object)
		if r.URL.Query().Has("legal-hold") && key != "" {
			h.GetObjectLegalHold(w, r, bucket, key)
			return
		}
		// Handle ListParts (GET ?uploadId on object)
		if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" && key != "" {
			h.ListParts(w, r, bucket, key, uploadID)
			return
		}
		// Regular Get operations
		if key != "" {
			h.GetObject(w, r, bucket, key)
		} else if named != "" {
			q := r.URL.Query()
			switch {
			case q.Has("location"):
				h.GetBucketLocation(w, r, bucket)
			case q.Has("versioning"):
				h.GetBucketVersioning(w, r, bucket)
			default:
				// All other bucket-level GETs are list operations.
				// HeadBucket is only for HTTP HEAD method (handled in MethodHead case).
				h.ListObjectsV2(w, r, bucket)
			}
		} else {
			h.ListBuckets(w, r)
		}
	case http.MethodPut:
		// Handle PutBucketLifecycleConfiguration (PUT ?lifecycle on bucket)
		if r.URL.Query().Has("lifecycle") && key == "" && named != "" {
			h.PutBucketLifecycleConfiguration(w, r, bucket)
			return
		}
		// Handle PutObjectLockConfiguration (PUT ?object-lock on bucket)
		if r.URL.Query().Has("object-lock") && key == "" && named != "" {
			h.PutObjectLockConfiguration(w, r, bucket)
			return
		}
		// Handle PutObjectRetention (PUT ?retention on object)
		if r.URL.Query().Has("retention") && key != "" {
			h.PutObjectRetention(w, r, bucket, key)
			return
		}
		// Handle PutObjectLegalHold (PUT ?legal-hold on object)
		if r.URL.Query().Has("legal-hold") && key != "" {
			h.PutObjectLegalHold(w, r, bucket, key)
			return
		}
		if key != "" {
			// Handle UploadPart (PUT ?partNumber&uploadId on object) — standard
			// S3 clients PUT parts; routing this to PutObject silently stored
			// each part as the whole object and left the B2 multipart upload
			// empty, so CompleteMultipartUpload always failed with InvalidPart.
			if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" && r.URL.Query().Get("partNumber") != "" {
				h.UploadPart(w, r, bucket, key, uploadID)
				return
			}
			// Check for CopyObject (has x-amz-copy-source header)
			if r.Header.Get("x-amz-copy-source") != "" {
				h.CopyObject(w, r, bucket, key)
			} else {
				h.PutObject(w, r, bucket, key)
			}
		} else if named != "" {
			h.CreateBucket(w, r, bucket)
		}
	case http.MethodHead:
		if key != "" {
			h.HeadObject(w, r, bucket, key)
		} else if named != "" {
			h.HeadBucket(w, r, bucket)
		}
	case http.MethodDelete:
		// Handle DeleteBucketLifecycleConfiguration (DELETE ?lifecycle on bucket)
		if r.URL.Query().Has("lifecycle") && key == "" && named != "" {
			h.DeleteBucketLifecycleConfiguration(w, r, bucket)
			return
		}
		// Handle AbortMultipartUpload (DELETE ?uploadId on object)
		if uploadID := r.URL.Query().Get("uploadId"); uploadID != "" && key != "" {
			h.AbortMultipartUpload(w, r, bucket, key, uploadID)
			return
		}
		if key != "" {
			h.DeleteObject(w, r, bucket, key)
		} else if named != "" {
			h.DeleteBucket(w, r, bucket)
		}
	case http.MethodPost:
		// Handle multipart upload operations
		uploadID := r.URL.Query().Get("uploadId")
		if r.URL.Query().Has("uploads") {
			h.CreateMultipartUpload(w, r, bucket, key)
		} else if uploadID != "" {
			if r.URL.Query().Get("partNumber") != "" {
				h.UploadPart(w, r, bucket, key, uploadID)
			} else {
				h.CompleteMultipartUpload(w, r, bucket, key, uploadID)
			}
		} else if r.URL.Query().Has("delete") {
			// DeleteObjects (bulk delete) - uses POST with ?delete query param
			h.DeleteObjects(w, r, bucket)
		} else {
			h.writeError(w, r, "InvalidRequest", "Unsupported POST operation", 400)
		}
	default:
		h.writeError(w, r, "MethodNotAllowed", fmt.Sprintf("Method %s not allowed", r.Method), 405)
	}
}

// userMetadata returns client-supplied metadata while keeping ARMOR's
// reserved metadata namespace private to the proxy.
func userMetadata(r *http.Request) map[string]string {
	metadata := make(map[string]string)
	for key, values := range r.Header {
		lowerKey := strings.ToLower(key)
		if !strings.HasPrefix(lowerKey, "x-amz-meta-") ||
			strings.HasPrefix(lowerKey, "x-amz-meta-armor-") || len(values) == 0 {
			continue
		}
		metadata[lowerKey] = strings.Join(values, ",")
	}
	return metadata
}

func mergeUserMetadata(metadata map[string]string, r *http.Request) {
	for key, value := range userMetadata(r) {
		metadata[key] = value
	}
}

// clientMetadata returns the client-supplied entries of a stored metadata
// map (lowercased x-amz-meta-* keys), keeping ARMOR's reserved
// x-amz-meta-armor-* namespace private to the proxy — the map-shaped twin of
// userMetadata's request-shaped filter.
func clientMetadata(metadata map[string]string) map[string]string {
	filtered := make(map[string]string, len(metadata))
	for key, value := range metadata {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "x-amz-meta-") && !strings.HasPrefix(lowerKey, "x-amz-meta-armor-") {
			filtered[lowerKey] = value
		}
	}
	return filtered
}

func writeUserMetadataHeaders(w http.ResponseWriter, metadata map[string]string) {
	for key, value := range metadata {
		lowerKey := strings.ToLower(key)
		if strings.HasPrefix(lowerKey, "x-amz-meta-") && !strings.HasPrefix(lowerKey, "x-amz-meta-armor-") {
			w.Header().Set(lowerKey, value)
		}
	}
}

// PutObject handles S3 PutObject with encryption.
// For small files (<10MB), it buffers in memory.
// For larger files, it uses streaming encryption via temp files to avoid
// loading the entire file into memory.
func (h *Handlers) PutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()
	// explicitCondition records the client's own If-None-Match: * intent,
	// separate from the create-only decision below: an append-only credential
	// has create-only forced on every write, and ADR-017 translates its
	// conflicts differently depending on whether the client asked for the
	// conditional check.
	explicitCondition, validCondition := putObjectCreateOnly(r)
	if !validCondition {
		h.writeError(w, r, "InvalidArgument", "Conditional uploads support only If-None-Match: *", http.StatusBadRequest)
		return
	}

	appendOnly, err := appendOnlyPut(ctx, bucket, key)
	if err != nil {
		h.writeError(w, r, "AccessDenied", "Access Denied", http.StatusForbidden)
		return
	}
	createOnly := explicitCondition || appendOnly
	if createOnly && h.conditionalObjectExists(ctx, bucket, key) {
		h.recordConditionalPut("early_412")
		if appendOnly {
			h.writeAppendOnlyPutError(w, r, backend.ErrPreconditionFailed, explicitCondition)
		} else {
			h.writePutObjectError(w, r, backend.ErrPreconditionFailed)
		}
		return
	}

	// Check Content-Length header
	contentLength := r.ContentLength
	streamingThreshold := int64(10 * 1024 * 1024) // 10MB threshold

	// Resolve the per-object compression decision (x-amz-meta-armor-compress
	// override > compress rules > global Compress flag) BEFORE choosing the
	// upload path. Keying streaming off the mere presence of compression
	// configuration forced every large or unknown-length body — including
	// objects no rule would compress — through io.ReadAll, and the streaming
	// path never evaluated the override header (silently ignoring compress
	// requests and never rejecting invalid ones). Only an object that will
	// actually be whole-object compressed needs the buffered path.
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	contentType = config.ExtractContentType(contentType)
	compressOverride := r.Header.Get("x-amz-meta-armor-compress")
	shouldCompress, err := config.EvaluateCompression(key, contentType, h.config.CompressRules, compressOverride, h.config.Compress)
	if err != nil {
		h.writeError(w, r, "InvalidArgument", fmt.Sprintf("Invalid compression override: %v", err), 400)
		return
	}

	// Use streaming for large files or unknown size unless THIS object will be
	// compressed: whole-object compression requires the buffer, and the
	// streaming path stores objects uncompressed.
	useStreaming := !shouldCompress && (contentLength < 0 || contentLength > streamingThreshold)

	if useStreaming {
		h.putObjectStreaming(ctx, w, r, bucket, key, createOnly, appendOnly, explicitCondition)
		return
	}

	// Small file: buffer in memory
	plaintext, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	plaintextSize := int64(len(plaintext))

	// Get the appropriate MEK for this object key
	mek, keyID, err := h.keyManager.GetMEK(key)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get encryption key: %v", err), 500)
		return
	}

	// Generate DEK and IV
	dek, err := crypto.GenerateDEK()
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to generate DEK: %v", err), 500)
		return
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to generate IV: %v", err), 500)
		return
	}

	// Wrap DEK with MEK and encode fingerprint in v2 format
	wrappedDEKStr, err := crypto.WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to wrap DEK: %v", err), 500)
		return
	}

	// Parse v2 format to extract fingerprint and wrapped DEK bytes
	// Format: v2:<fp16>:<base64>
	parts := strings.SplitN(wrappedDEKStr, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" {
		h.writeError(w, r, "InternalError", "wrapped DEK format validation failed", 500)
		return
	}
	mekFingerprint := parts[1]
	wrappedDEK, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode wrapped DEK: %v", err), 500)
		return
	}

	// Compute plaintext SHA-256 BEFORE compression (ADR-007)
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	// contentType and the compression decision (shouldCompress) were resolved
	// above, before the buffered-vs-streaming choice; reuse them here.
	// Compress based on that decision (ADR-007: compress-before-encrypt).
	// Rules support: suffix matching (.jsonl), content-type matching (application/json),
	// wildcard (*), and per-request override (x-amz-meta-armor-compress header).
	// ARMOR_COMPRESS=true is an alias for "*=zstd".
	var compressed bool
	var compressionType crypto.CompressionType
	dataToEncrypt := plaintext

	if shouldCompress {
		compressedData, wasCompressed, compType, err := crypto.Compress(plaintext)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Compression failed: %v", err), 500)
			return
		}
		compressed = wasCompressed
		compressionType = compType
		if compressed {
			dataToEncrypt = compressedData
		}
	}

	// Create envelope header with ORIGINAL plaintext size (before compression)
	// Use v3 format when Config.FormatWriteVersion is 3
	envelopeVersion := crypto.Version2
	if h.config.FormatWriteVersion == 3 {
		envelopeVersion = crypto.Version3
	}

	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, plaintextSize, h.config.BlockSize, plaintextSHA, uint8(envelopeVersion))
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create header: %v", err), 500)
		return
	}

	// Set compression flag in envelope header (ADR-007)
	if compressed {
		switch compressionType {
		case crypto.CompressionZstd:
			header.SetCompressionFlag(crypto.CompressionFlagZstd)
		default:
			header.SetCompressionFlag(crypto.CompressionFlagNone)
		}
	}

	headerBytes, err := header.Encode()
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encode header: %v", err), 500)
		return
	}

	// Build envelope based on version
	var envelope []byte
	var envelopeSize int64

	if envelopeVersion == crypto.Version3 {
		// v3 format: header || blocks || trailer block table
		encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, h.config.BlockSize, crypto.Version3)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create encryptor: %v", err), 500)
			return
		}

		// Encrypt with v3 and produce trailer block table
		encrypted, blockTable, err := encryptor.EncryptV3(dataToEncrypt, false) // Compression off per compress-rules bead
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encrypt v3: %v", err), 500)
			return
		}

		// Encode trailer block table
		trailerTable, err := blockTable.Encode()
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encode block table: %v", err), 500)
			return
		}

		// Build v3 envelope: header + encrypted blocks + trailer block table
		envelopeSize = int64(len(headerBytes)) + int64(len(encrypted)) + int64(len(trailerTable))
		envelope = make([]byte, 0, envelopeSize)
		envelope = append(envelope, headerBytes...)
		envelope = append(envelope, encrypted...)
		envelope = append(envelope, trailerTable...)
	} else {
		// v2 format: header || blocks || inline HMAC table
		encryptor, err := crypto.NewEncryptor(dek, iv, h.config.BlockSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create encryptor: %v", err), 500)
			return
		}

		encrypted, hmacTable, err := encryptor.Encrypt(dataToEncrypt)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encrypt: %v", err), 500)
			return
		}

		// Build v2 envelope: header + encrypted blocks + HMAC table
		envelopeSize = int64(len(headerBytes)) + int64(len(encrypted)) + int64(len(hmacTable))
		envelope = make([]byte, 0, envelopeSize)
		envelope = append(envelope, headerBytes...)
		envelope = append(envelope, encrypted...)
		envelope = append(envelope, hmacTable...)
	}

	// Compute plaintext ETag (MD5) on ORIGINAL plaintext
	etag := backend.ComputeETag(plaintext)

	// Build metadata with version matching envelope format
	// contentType was already extracted above for compression rules
	metaVersion := 2
	if envelopeVersion == crypto.Version3 {
		metaVersion = 3
	}

	meta := (&backend.ARMORMetadata{
		Version:         metaVersion,
		BlockSize:       h.config.BlockSize,
		PlaintextSize:   plaintextSize,
		ContentType:     contentType,
		IV:              iv,
		WrappedDEK:      wrappedDEK,
		MEKFingerprint:  mekFingerprint,
		PlaintextSHA:    hex.EncodeToString(plaintextSHA[:]),
		ETag:            etag,
		KeyID:           keyID,
		Compressed:      compressed,
		CompressionType: backend.CompressionType(compressionType),
	}).ToMetadata()
	mergeUserMetadata(meta, r)

	// Upload to B2 with prefix applied
	prefixedKey := h.applyPrefix(key)
	backendPutStart := time.Now()
	if err := h.storePutObject(ctx, bucket, prefixedKey, bytes.NewReader(envelope), int64(len(envelope)), meta, createOnly); err != nil {
		if appendOnly {
			h.writeAppendOnlyPutError(w, r, err, explicitCondition)
		} else {
			h.writePutObjectError(w, r, err)
		}
		return
	}
	backendPutDuration := time.Since(backendPutStart)

	// The overwrite retired the cached generation (if any). Backend.Put does
	// not return the new stored ETag, so there is nothing sound to cache here;
	// the next HEAD or GET repopulates it.
	h.cache.Delete(bucket, key)

	// PUT latency instrumentation (armor-69dd394b): when the request takes the
	// provenance-recording path below, collect the provenance lock-wait/write
	// split and publish it — with the backend PUT duration — to the request
	// context for the request-completed log (backend_put_ms,
	// provenance_lock_wait_ms, provenance_write_ms) and the armor_put_*_ms
	// histograms. Absent otherwise, so the fields mark exactly the PUTs the
	// provenance path contributed to.
	recordProvenance := h.provenance != nil && h.provenance.ShouldRecord(key)
	var provTimings *provenance.Timings
	if recordProvenance {
		provTimings = &provenance.Timings{}
	}

	// Record in manifest for fast metadata lookup (async B2 persistence)
	// When manifest is enabled, provenance is embedded in delta lines.
	// When manifest is disabled, provenance uses per-object entries.
	var chainEntry *manifest.ChainEntry
	if h.manifest != nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		entryData, err := h.provenance.CreateChainEntry(provenance.WithTimings(ctx, provTimings), key, plaintextSHAHex, "put")
		if err == nil && entryData != nil {
			chainEntry = &manifest.ChainEntry{
				Sequence:      entryData.Sequence,
				ChainHash:     entryData.ChainHash,
				PrevChainHash: entryData.PrevChainHash,
			}
		}
		// If CreateChainEntry fails, we still record the manifest entry
		// without provenance data — this is acceptable as a degradation mode.
	}
	if h.manifest != nil {
		// For v3 objects, pass CiphertextSize for trailer block table location
		var ciphertextSize int64
		if envelopeVersion == crypto.Version3 {
			ciphertextSize = int64(len(envelope))
		}
		h.manifest.RecordPut(bucket, key, plaintextSize, hex.EncodeToString(plaintextSHA[:]), iv, wrappedDEK, mekFingerprint, h.config.BlockSize, contentType, etag, chainEntry, ciphertextSize, userMetadata(r))
	}

	// Record provenance (fallback when manifest is disabled)
	if h.manifest == nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		h.recordProvenanceUpload(provenance.WithTimings(ctx, provTimings), key, plaintextSHAHex, "put")
	}

	// Publish the PUT latency split to the request context (the in-place
	// mutation logCompletedRequest reads, same pattern as writeError) and
	// record the histograms.
	if recordProvenance {
		*r = *r.WithContext(middleware.WithPutLatency(r.Context(), &middleware.PutLatency{
			BackendPutMs:         backendPutDuration.Milliseconds(),
			ProvenanceLockWaitMs: provTimings.LockWait.Milliseconds(),
			ProvenanceWriteMs:    provTimings.Write.Milliseconds(),
		}))
		if h.metrics != nil {
			h.metrics.RecordPutLatency("put", backendPutDuration, provTimings.LockWait, provTimings.Write)
		}
	}

	// Invalidate list cache entries covering this key.
	if h.listCache != nil {
		h.listCache.InvalidateForWrite(bucket, key)
	}

	// Return ETag
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.WriteHeader(http.StatusOK)

	// Enqueue replication task to secondary backend if configured (non-blocking)
	// This runs in a goroutine after the client receives the success response
	if h.replicationQueue != nil {
		go func() {
			// Recover from panics to prevent goroutine crashes
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC in replication enqueue (put %s/%s): %v", bucket, key, r)
				}
			}()

			// Enqueue the replication task
			// Note: Enqueue() is non-blocking and does not return errors
			// Dropped items are tracked via the replication_dropped_total metric
			if h.replicationQueue != nil {
				h.replicationQueue.Enqueue(bucket, prefixedKey)
				if h.metrics != nil {
					h.metrics.IncReplicationEnqueued("put")
				}
			} else {
				log.Printf("replication queue is nil, skipping enqueue for %s/%s", bucket, key)
			}
		}()
	}
}

// putObjectStreaming handles large file uploads with streaming encryption.
// It uses a temp file to avoid loading the entire plaintext into memory.
// The process is:
// 1. Stream request body to temp file while computing SHA-256
// 2. Create envelope header with the computed SHA-256
// 3. Stream from temp file through encryption to B2 via io.Pipe
// 4. Clean up temp file
//
// explicitCondition is the client's own If-None-Match: * intent (ADR-017):
// it only shapes conflict translation for append-only credentials, never the
// create-only decision, which createOnly already carries.
func (h *Handlers) putObjectStreaming(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket, key string, createOnly, appendOnly, explicitCondition bool) {
	// Phase 1: Stream to temp file and compute SHA-256
	tmpFile, err := os.CreateTemp("", "armor-upload-*.tmp")
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create temp file: %v", err), 500)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up on exit
	defer tmpFile.Close()

	// Compute SHA-256 while copying to temp file
	plaintextHash := sha256.New()
	teeReader := io.TeeReader(r.Body, plaintextHash)

	plaintextSize, err := io.Copy(tmpFile, teeReader)
	if err != nil {
		tmpFile.Close()
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}
	if r.ContentLength >= 0 && plaintextSize != r.ContentLength {
		tmpFile.Close()
		h.writeError(w, r, "InvalidRequest", "Request body is shorter than Content-Length", 400)
		return
	}
	if err := ctx.Err(); err != nil {
		tmpFile.Close()
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Upload canceled: %v", err), 500, err)
		return
	}

	// Get the computed SHA-256
	var plaintextSHA [32]byte
	copy(plaintextSHA[:], plaintextHash.Sum(nil))

	// Seek back to beginning of temp file for reading
	if _, err := tmpFile.Seek(0, 0); err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to seek temp file: %v", err), 500)
		return
	}

	// Phase 2: Get encryption keys
	mek, keyID, err := h.keyManager.GetMEK(key)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get encryption key: %v", err), 500)
		return
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to generate DEK: %v", err), 500)
		return
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to generate IV: %v", err), 500)
		return
	}

	// Wrap DEK with MEK and encode fingerprint in v2 format
	wrappedDEKStr, err := crypto.WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to wrap DEK: %v", err), 500)
		return
	}

	// Parse v2 format to extract fingerprint and wrapped DEK bytes
	// Format: v2:<fp16>:<base64>
	parts := strings.SplitN(wrappedDEKStr, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", "wrapped DEK format validation failed", 500)
		return
	}
	mekFingerprint := parts[1]
	wrappedDEK, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode wrapped DEK: %v", err), 500)
		return
	}

	// Create envelope header with version based on FormatWriteVersion
	envelopeVersion := crypto.Version2
	if h.config.FormatWriteVersion == 3 {
		envelopeVersion = crypto.Version3
	}

	header, err := crypto.NewEnvelopeHeaderWithVersion(iv, plaintextSize, h.config.BlockSize, plaintextSHA, uint8(envelopeVersion))
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create header: %v", err), 500)
		return
	}

	headerBytes, err := header.Encode()
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encode header: %v", err), 500)
		return
	}

	// Calculate envelope size based on version
	blockCount := crypto.ComputeBlockCount(plaintextSize, h.config.BlockSize)
	var envelopeSize int64

	if envelopeVersion == crypto.Version3 {
		// v3: header + blocks + trailer block table (36 bytes per block)
		trailerTableSize := int64(blockCount) * crypto.BlockTableEntrySize
		envelopeSize = int64(len(headerBytes)) + plaintextSize + trailerTableSize
	} else {
		// v2: header + blocks + inline HMAC table (32 bytes per block)
		hmacTableSize := int64(blockCount) * crypto.HMACSize
		envelopeSize = int64(len(headerBytes)) + plaintextSize + hmacTableSize
	}

	// Encrypt lazily as the backend reads. The seekable source lets the SDK
	// provide Content-Length and replay exactly the same bytes on retries,
	// without writing a second full-object ciphertext spool.
	encrypted, err := crypto.NewReplayableEnvelopeReader(tmpFile, headerBytes, dek, iv, h.config.BlockSize, plaintextSize, uint8(envelopeVersion))
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to prepare encryption: %v", err), 500)
		return
	}

	// Build metadata with version matching envelope format
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Compute ETag while we have the temp file
	// We need to read the file again for MD5, but we already have SHA-256
	// Use SHA-256 truncated to 16 bytes as ETag for streaming (non-standard but works)
	etag := hex.EncodeToString(plaintextSHA[:16])

	metaVersion := 2
	if envelopeVersion == crypto.Version3 {
		metaVersion = 3
	}

	meta := (&backend.ARMORMetadata{
		Version:       metaVersion,
		BlockSize:     h.config.BlockSize,
		PlaintextSize: plaintextSize,
		ContentType:   contentType,
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
		ETag:          etag,
		KeyID:         keyID,
	}).ToMetadata()
	mergeUserMetadata(meta, r)

	// Upload with prefix applied. Encryption starts with the backend read.
	prefixedKey := h.applyPrefix(key)
	backendPutStart := time.Now()
	if err := h.storePutObject(ctx, bucket, prefixedKey, encrypted, envelopeSize, meta, createOnly); err != nil {
		tmpFile.Close()
		if appendOnly {
			h.writeAppendOnlyPutError(w, r, err, explicitCondition)
		} else {
			h.writePutObjectError(w, r, err)
		}
		return
	}
	if pos, seekErr := encrypted.Seek(0, io.SeekCurrent); seekErr != nil || pos != envelopeSize {
		h.writeError(w, r, "InternalError", "Backend did not consume the complete encrypted object", 500)
		return
	}
	if err := ctx.Err(); err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Upload canceled: %v", err), 500, err)
		return
	}
	backendPutDuration := time.Since(backendPutStart)

	// The overwrite retired the cached generation (if any). Backend.Put does
	// not return the new stored ETag, so there is nothing sound to cache here;
	// the next HEAD or GET repopulates it.
	h.cache.Delete(bucket, key)

	// Close temp file
	tmpFile.Close()

	// PUT latency instrumentation (armor-69dd394b): same split as the buffered
	// path — see the comment there. This backend PUT leg includes encryption
	// as the backend consumes the replayable body.
	recordProvenance := h.provenance != nil && h.provenance.ShouldRecord(key)
	var provTimings *provenance.Timings
	if recordProvenance {
		provTimings = &provenance.Timings{}
	}

	// Record in manifest for fast metadata lookup (async B2 persistence)
	// When manifest is enabled, provenance is embedded in delta lines.
	// When manifest is disabled, provenance uses per-object entries.
	var chainEntry *manifest.ChainEntry
	if h.manifest != nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		entryData, err := h.provenance.CreateChainEntry(provenance.WithTimings(ctx, provTimings), key, plaintextSHAHex, "put-streaming")
		if err == nil && entryData != nil {
			chainEntry = &manifest.ChainEntry{
				Sequence:      entryData.Sequence,
				ChainHash:     entryData.ChainHash,
				PrevChainHash: entryData.PrevChainHash,
			}
		}
	}
	if h.manifest != nil {
		// For v3 objects, pass CiphertextSize for trailer block table location
		var ciphertextSize int64
		if envelopeVersion == crypto.Version3 {
			ciphertextSize = envelopeSize
		}
		h.manifest.RecordPut(bucket, key, plaintextSize, hex.EncodeToString(plaintextSHA[:]), iv, wrappedDEK, mekFingerprint, h.config.BlockSize, contentType, etag, chainEntry, ciphertextSize, userMetadata(r))
	}

	// Record provenance (fallback when manifest is disabled)
	if h.manifest == nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		h.recordProvenanceUpload(provenance.WithTimings(ctx, provTimings), key, plaintextSHAHex, "put-streaming")
	}

	// Publish the PUT latency split to the request context (the in-place
	// mutation logCompletedRequest reads, same pattern as writeError) and
	// record the histograms.
	if recordProvenance {
		*r = *r.WithContext(middleware.WithPutLatency(r.Context(), &middleware.PutLatency{
			BackendPutMs:         backendPutDuration.Milliseconds(),
			ProvenanceLockWaitMs: provTimings.LockWait.Milliseconds(),
			ProvenanceWriteMs:    provTimings.Write.Milliseconds(),
		}))
		if h.metrics != nil {
			h.metrics.RecordPutLatency("put-streaming", backendPutDuration, provTimings.LockWait, provTimings.Write)
		}
	}

	// Invalidate list cache entries covering this key.
	if h.listCache != nil {
		h.listCache.InvalidateForWrite(bucket, key)
	}

	// Return ETag
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.Header().Set("X-Armor-Streaming", "true")
	w.WriteHeader(http.StatusOK)

	// Enqueue replication task to secondary backend if configured (non-blocking)
	// This runs in a goroutine after the client receives the success response
	if h.replicationQueue != nil {
		go func() {
			// Recover from panics to prevent goroutine crashes
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC in replication enqueue (put-streaming %s/%s): %v", bucket, key, r)
				}
			}()

			// Enqueue the replication task
			// Note: Enqueue() is non-blocking and does not return errors
			// Dropped items are tracked via the replication_dropped_total metric
			if h.replicationQueue != nil {
				h.replicationQueue.Enqueue(bucket, prefixedKey)
				if h.metrics != nil {
					h.metrics.IncReplicationEnqueued("put-streaming")
				}
			} else {
				log.Printf("replication queue is nil, skipping enqueue for %s/%s", bucket, key)
			}
		}()
	}
}

// backendFailureResponse maps a backend Head/Get failure to the S3 error
// code, message, and HTTP status the client should see. Only a definitive
// not-found is a 404: backup and sync clients (rclone, litestream, barman)
// treat 404 as "object absent" and may re-upload, skip a restore, or delete a
// local copy, so a backend timeout or an upstream 5xx must surface as a
// retryable 503 (or a 500 when the failure cannot be classified) instead of
// masquerading as absence. The upstream failure text is kept in the message,
// which the structured s3_error log carries with it.
func backendFailureResponse(err error) (code, message string, status int) {
	switch backend.ClassifyBackendError(err) {
	case backend.BackendErrorNotFound:
		return "NoSuchKey", fmt.Sprintf("Object not found: %v", err), http.StatusNotFound
	case backend.BackendErrorTimeout:
		return "RequestTimeout", fmt.Sprintf("Backend request timed out: %v", err), http.StatusServiceUnavailable
	case backend.BackendErrorUnavailable:
		return "ServiceUnavailable", fmt.Sprintf("Backend unavailable: %v", err), http.StatusServiceUnavailable
	default:
		return "InternalError", fmt.Sprintf("Failed to read object: %v", err), http.StatusInternalServerError
	}
}

// GetObject handles S3 GetObject with decryption and range support.
func (h *Handlers) GetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Apply prefix for backend operations
	prefixedKey := h.applyPrefix(key)

	// ADR-016: Try manifest first for multipart objects
	var info *backend.ObjectInfo
	var err error
	// ciphertextInfo holds the authoritative stored-ciphertext HEAD on the
	// manifest path; on the legacy path info itself is the stored HEAD.
	var ciphertextInfo *backend.ObjectInfo
	var headErr error

	// The manifest probe decides which object is served, but for every
	// single-PUT object it is a miss that used to cost one full sequential
	// round trip before the legacy HEAD could even start (~74 ms per read;
	// armor-50a36688). Start the legacy HEAD concurrently and decide exactly
	// as the sequential path did: an existing manifest wins and the HEAD
	// result is discarded, a manifest miss consumes it, and a non-404
	// manifest failure still fails the request — never a fallback to the
	// legacy object. The overlap trades one extra backend request on the
	// manifest path for one fewer sequential round trip on the legacy path;
	// which object is served is unchanged (armor-420c15a5). legacyKey is
	// captured before the goroutine starts because the manifest path below
	// reassigns prefixedKey to the ciphertext object. The channel is
	// buffered so the probe goroutine can always deliver and exit, and the
	// request context — cancelled by net/http when this handler returns or
	// the client disconnects — bounds its lifetime.
	type legacyHeadResult struct {
		info *backend.ObjectInfo
		err  error
	}
	legacyKey := prefixedKey
	legacyHeadCh := make(chan legacyHeadResult, 1)
	go func() {
		i, herr := h.backend.Head(ctx, bucket, legacyKey)
		legacyHeadCh <- legacyHeadResult{info: i, err: herr}
	}()
	manifestBody, manifestMeta, manifestErr := h.readManifest(ctx, bucket, key)
	if manifestErr != nil && !backend.IsNotFound(manifestErr) {
		// The manifest read failed for a reason other than "no manifest was
		// ever written" — a timeout or upstream 5xx. Falling back to the
		// legacy path here would serve raw ciphertext for a multipart object
		// or report an existing object absent, so surface the backend
		// failure with the same classification the legacy head gets.
		code, _, status := backendFailureResponse(manifestErr)
		h.writeBackendError(w, r, code, fmt.Sprintf("Failed to read manifest: %v", manifestErr), status, manifestErr)
		return
	}
	if manifestErr == nil && manifestBody != nil {
		// Manifest exists - use it as the source of truth for metadata

		// Operator quarantine gate (see manifest_repair.go). A quarantined
		// manifest is a definitive verdict on this object, so the read fails
		// with a non-retryable 403 instead of the retryable 500 a stale
		// manifest produces — clients that retry 5xx forever (litestream
		// compaction, restore verification) would otherwise hammer a dead
		// object indefinitely. Checked before the freshness gate so a
		// quarantined-and-stale manifest still gets the definitive verdict.
		if reason, quarantined := manifestQuarantineState(manifestMeta); quarantined {
			h.writeError(w, r, "AccessDenied", fmt.Sprintf("object is quarantined and cannot be read; operator action is required: %s", reason), http.StatusForbidden)
			return
		}

		// One authoritative ciphertext HEAD serves both the ADR-016 freshness
		// gate and the stored size/LastModified below (armor-c3199f1c).
		// Previously the freshness check and the size check each headed the
		// same ciphertext object. The check is still performed on every
		// request — nothing is cached or skipped here.
		ciphertextInfo, headErr = h.headCiphertextForFreshness(ctx, bucket, manifestBody.CiphertextObject, manifestMeta[metaCompletedAt])
		if headErr != nil {
			var freshErr *ciphertextFreshnessError
			if errors.As(headErr, &freshErr) {
				// Retryable by design: an in-progress ADR-016 overwrite
				// resolves itself once the new manifest lands. When it never
				// resolves, the operator repairs or quarantines via the admin
				// endpoints named in the message — the ones operators grep
				// for during exactly this outage.
				h.writeError(w, r, "InternalError", fmt.Sprintf("manifest verification failed; operator repair or quarantine is required: %v", freshErr.err), 500)
			} else {
				h.writeBackendError(w, r, "InternalError", fmt.Sprintf("Failed to head ciphertext object: %v", headErr), 500, headErr)
			}
			return
		}

		// Parse ARMOR metadata from manifest
		armorMeta, ok := backend.ParseARMORMetadata(manifestMeta)
		if !ok {
			h.writeError(w, r, "InternalError", "Failed to parse ARMOR metadata from manifest", 500)
			return
		}

		// Set info from manifest metadata
		info = &backend.ObjectInfo{
			Key:              manifestBody.CiphertextObject,
			Size:             armorMeta.PlaintextSize,
			StoredSize:       ciphertextInfo.StoredSize,
			ContentType:      armorMeta.ContentType,
			ETag:             armorMeta.ETag,
			LastModified:     ciphertextInfo.LastModified,
			Metadata:         manifestMeta,
			IsARMOREncrypted: true,
		}
		// Warm the metadata cache for ListObjectsV2 size resolution, bound to
		// the stored ciphertext generation the freshness HEAD just observed.
		h.cache.SetVersioned(bucket, key, ciphertextInfo.ETag, armorMeta)

		// Override prefixedKey to point to ciphertext object
		prefixedKey = manifestBody.CiphertextObject
	} else {
		// Legacy path: no manifest, so the overlapped HEAD's result IS the
		// decision input — wait for it here instead of issuing a second,
		// sequential HEAD. This wait is why the overlap wins: the HEAD has
		// been in flight since before the manifest probe started.
		res := <-legacyHeadCh
		info, err = res.info, res.err
		if err == nil && info != nil && info.IsARMOREncrypted {
			// Warm the metadata cache for ListObjectsV2 size resolution
			// (the manifest path above warmed it from its own HEAD),
			// bound to this stored generation (info.ETag).
			if am, ok := backend.ParseARMORMetadata(info.Metadata); ok && am != nil {
				h.cache.SetVersioned(bucket, key, info.ETag, am)
			}
		}
	}

	if err != nil && info == nil {
		code, message, status := backendFailureResponse(err)
		h.writeBackendError(w, r, code, message, status, err)
		return
	}

	if !info.IsARMOREncrypted {
		// Check conditional request headers for non-ARMOR objects
		if status := checkConditionalRequest(r, info.ETag, info.LastModified); status != 0 {
			if status == http.StatusNotModified {
				w.Header().Set("ETag", fmt.Sprintf(`"%s"`, info.ETag))
				w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
				w.WriteHeader(status)
			} else {
				h.writeError(w, r, "PreconditionFailed", "Precondition failed", status)
			}
			return
		}

		// Passthrough for non-ARMOR objects
		body, _, err := h.backend.Get(ctx, bucket, prefixedKey)
		if err != nil {
			h.writeBackendError(w, r, "InternalError", fmt.Sprintf("Failed to get object: %v", err), 500, err)
			return
		}
		defer body.Close()

		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.Header().Set("Content-Type", info.ContentType)
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, info.ETag))
		w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
		writeUserMetadataHeaders(w, info.Metadata)
		w.WriteHeader(http.StatusOK)
		_, err = io.Copy(w, body)
		if err != nil {
			log.Printf("GET %s/%s: non-ARMOR streaming error: %v", bucket, key, err)
		}
		return
	}

	// Parse ARMOR metadata
	armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		h.writeError(w, r, "InternalError", "Failed to parse ARMOR metadata", 500)
		return
	}

	// Unwrap DEK using fingerprint with ring fallback
	// Build a lookup function for the keymanager to find MEK by fingerprint
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		return h.keyManager.GetMEKByFingerprint(keyID, fingerprint)
	}

	// Build a legacy fallback that tries the active key then ring keys
	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		// Try active key first
		mek, err := h.keyManager.GetMEKByID(armorMeta.KeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to get active key: %w", err)
		}
		dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
		if err == nil {
			return dek, nil
		}

		// Try ring keys in order
		ring := h.keyManager.Ring(armorMeta.KeyID)
		for _, ringEntry := range ring {
			dek, err := crypto.UnwrapDEK(ringEntry.MEK, wrappedDEK)
			if err == nil {
				return dek, nil
			}
		}

		return nil, fmt.Errorf("no key in active or ring can unwrap DEK")
	}

	wrappedDEKStr := base64.StdEncoding.EncodeToString(armorMeta.WrappedDEK)
	if armorMeta.MEKFingerprint != "" {
		// Already have fingerprint from metadata, build v2 format
		wrappedDEKStr = fmt.Sprintf("v2:%s:%s", armorMeta.MEKFingerprint, wrappedDEKStr)
	}

	dek, usedFingerprint, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		if err == crypto.ErrFingerprintNotFound {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to unwrap DEK: MEK fingerprint %s not found in active or ring keys", armorMeta.MEKFingerprint), 500)
		} else {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to unwrap DEK: %v", err), 500)
		}
		return
	}
	_ = usedFingerprint // Currently unused, but available for logging/auditing

	plaintextSize := armorMeta.PlaintextSize

	// Check if this is a multipart object (HMAC table in sidecar, no embedded header)
	isMultipart := info.Metadata["x-amz-meta-armor-multipart"] == "true"

	// Determine the version and create decryptor
	// For single-PUT objects: read envelope header to get version
	// For multipart objects: trust the metadata version (no envelope header exists)
	// envelopeHeader is the request-local decoded header read here once; the
	// downstream full/range helpers reuse it instead of re-reading the same 64
	// bytes from the backend (armor-c3199f1c). It is nil for multipart objects,
	// which have no envelope header.
	var decryptor *crypto.Decryptor
	var envelopeHeader *crypto.EnvelopeHeader
	// singleFetchEnvelope holds the whole-ciphertext buffer taken by the
	// small-object single fetch below; nil when the request streams. It is
	// consumed by handleFullObjectStream and never by the range handlers.
	var singleFetchEnvelope []byte
	if isMultipart {
		// Multipart objects have no envelope header - trust metadata version
		decryptor, err = crypto.NewDecryptorWithVersion(dek, armorMeta.IV, armorMeta.BlockSize, uint8(armorMeta.Version))
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create decryptor: %v", err), 500)
			return
		}
	} else {
		// Single-PUT objects: read envelope header to get the actual version
		prefixedKey := h.applyPrefix(key)

		// Small single-PUT objects (armor-98864f12): the HEAD already reported
		// the stored ciphertext length, so at or below the configured threshold
		// the whole envelope — header, data blocks, and the v1/v2 inline HMAC
		// table or v3 trailer block table — arrives in ONE ranged request and
		// is parsed locally, replacing two further sequential round trips with
		// extra bytes. Range requests deliberately keep the streaming reads:
		// the range handlers fetch their own spans, so a whole-object fetch
		// would buy them nothing. The slot is held until this handler returns
		// — i.e. until the streamed response is complete — so at most
		// ReadBackendLimit buffers of at most SmallObjectFetchBytes are live
		// at once; a request that finds the cap full streams instead.
		var singleFetch []byte
		if threshold := h.config.SmallObjectFetchBytes; threshold > 0 &&
			!isMultipart && r.Header.Get("Range") == "" {
			if storedSize := info.StoredSize; storedSize >= crypto.HeaderSize && storedSize <= threshold {
				select {
				case h.smallFetchBufs <- struct{}{}:
					defer func() { <-h.smallFetchBufs }()
					body, ferr := h.backend.GetRange(ctx, bucket, prefixedKey, 0, storedSize)
					if ferr != nil {
						h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read small object: %v", ferr), 500)
						return
					}
					singleFetch, ferr = readSmallFetchBody(body, storedSize)
					body.Close()
					if ferr != nil {
						h.writeError(w, r, "InternalError", fmt.Sprintf("Small object read did not match the stored length: %v", ferr), 500)
						return
					}
				default:
					// Buffer cap exhausted: stream rather than queue behind
					// other small fetches.
				}
			}
		}

		var headerBuf []byte
		if singleFetch != nil {
			// The single fetch already covers the header; no separate read.
			headerBuf = singleFetch[:crypto.HeaderSize]
		} else {
			headerReader, err := h.backend.GetRange(ctx, bucket, prefixedKey, 0, crypto.HeaderSize)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read envelope header: %v", err), 500)
				return
			}
			defer headerReader.Close()

			headerBuf = make([]byte, crypto.HeaderSize)
			if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
				return
			}
		}

		header, err := crypto.DecodeHeader(headerBuf)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
			return
		}
		envelopeHeader = header

		// Create decryptor with the version from the envelope header
		decryptor, err = crypto.NewDecryptorWithVersion(dek, header.IV[:], header.BlockSize(), header.Version)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create decryptor: %v", err), 500)
			return
		}

		// For v3 single-PUT objects, we need the ciphertext size to locate the trailer block table
		// Store it in a local variable for use in the streaming path
		if header.Version == crypto.Version3 {
			armorMeta.CiphertextSize = info.StoredSize
			if armorMeta.CiphertextSize == 0 {
				h.writeError(w, r, "InternalError", "v3 object missing stored ciphertext size", 500)
				return
			}
		}
		singleFetchEnvelope = singleFetch
	}

	// Check conditional request headers
	if status := checkConditionalRequest(r, armorMeta.ETag, info.LastModified); status != 0 {
		if status == http.StatusNotModified {
			// 304 Not Modified - set headers but no body
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
			w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
			w.WriteHeader(status)
		} else {
			// 412 Precondition Failed
			h.writeError(w, r, "PreconditionFailed", "Precondition failed", status)
		}
		return
	}

	// Serve the client-supplied metadata on every ARMOR GET (full and ranged
	// alike); the dispatchers below write their own standard headers but no
	// metadata, and S3 clients expect x-amz-meta-* on GetObject responses.
	// info.Metadata is the authoritative source on both paths: the manifest
	// path set it to the manifest object's headers, the legacy path to the
	// stored ciphertext's.
	writeUserMetadataHeaders(w, info.Metadata)

	// Uniform part size P for multipart objects written since bf-1v2ehf
	// (x-amz-meta-armor-part-size). The full-object stream path needs it to
	// reproduce the stored combined per-part digest; 0 means the object is either
	// single-PUT or a legacy multipart upload that carries only the empty-string
	// placeholder digest (unverifiable on read — left to the restore verifier).
	multipartPartSize := int64(0)
	if isMultipart {
		if ps, err := strconv.ParseInt(info.Metadata["x-amz-meta-armor-part-size"], 10, 64); err == nil {
			multipartPartSize = ps
		}
	}

	// ADR-011: Check if this is a non-uniform multipart object
	isNonUniform := info.Metadata["x-amz-meta-armor-non-uniform"] == "true"
	var cumulativePartSizes map[int]int64
	if isNonUniform {
		// Parse cumulative part sizes from metadata
		if cumulativeJSON, ok := info.Metadata["x-amz-meta-armor-cumulative-sizes"]; ok {
			if err := json.Unmarshal([]byte(cumulativeJSON), &cumulativePartSizes); err != nil {
				log.Printf("Warning: failed to parse cumulative part sizes for %s/%s: %v", bucket, key, err)
				// Continue anyway - will fall back to block-aligned decryption
				isNonUniform = false
			}
		} else {
			log.Printf("Warning: non-uniform flag set but no cumulative sizes for %s/%s", bucket, key)
			isNonUniform = false
		}
	}

	// Stored-object generation for generation-keyed read caches (v3 block
	// tables): the manifest path's authoritative ciphertext HEAD, or on the
	// legacy path the object HEAD itself. Derived fresh per request — never
	// from a previous request's view — so an overwrite, re-encryption, or
	// migration misses the cache instead of serving a mismatched table.
	storedGenSource := info
	if ciphertextInfo != nil {
		storedGenSource = ciphertextInfo
	}
	storedGen := generationFromInfo(storedGenSource)

	// Check for range request
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		// Fail-closed: range requests over compressed objects are not supported
		// Compression destroys fixed-offset seeking (zstd/gzip/zlib are variable-length encodings),
		// so byte ranges into compressed ciphertext would return corrupt data.
		if armorMeta.Compressed {
			h.writeError(w, r, "InvalidRange", "Range reads unsupported on compressed objects", 416)
			return
		}

		// Dispatch to appropriate range handler based on version and multipart status
		if isMultipart && armorMeta.Version == 3 {
			prefixedKey := h.applyPrefix(key)
			start, end, err := parseRangeHeader(rangeHeader, plaintextSize)
			if err != nil {
				h.writeError(w, r, "InvalidRange", fmt.Sprintf("Invalid range: %v", err), 416)
				return
			}
			h.handleV3MultipartRangeRequest(w, r, bucket, key, prefixedKey, decryptor, armorMeta, plaintextSize, info.LastModified, start, end)
		} else {
			h.handleRangeRequest(w, r, bucket, key, decryptor, armorMeta, plaintextSize, info.LastModified, isMultipart, envelopeHeader, storedGen)
		}
		return
	}

	// Full object download - dispatch based on version and multipart status
	if isMultipart && armorMeta.Version == 3 {
		prefixedKey := h.applyPrefix(key)
		h.handleV3MultipartGet(w, r, bucket, key, prefixedKey, decryptor, armorMeta, plaintextSize, info.LastModified)
	} else {
		h.handleFullObjectStream(w, r, bucket, key, decryptor, armorMeta, plaintextSize, info.LastModified, isMultipart, multipartPartSize, isNonUniform, cumulativePartSizes, envelopeHeader, storedGen, singleFetchEnvelope)
	}
}

// readSmallFetchBody reads exactly want bytes from a whole-object fetch,
// failing short and over-long bodies alike (armor-98864f12). The buffered
// small-fetch path sizes its request from the HEAD's stored length, so a body
// that does not match it exactly means the object changed under the request
// or the backend ignored the range — either way the bytes cannot be trusted
// against the envelope layout the HEAD implied, and the request fails before
// any plaintext exists. The real backends enforce the same length on every
// ranged response (internal/backend readRangeBody); this re-check also covers
// test doubles and any future backend that does not.
func readSmallFetchBody(body io.Reader, want int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, want+1))
	if err != nil {
		return nil, fmt.Errorf("read %d bytes: %w", want, err)
	}
	if int64(len(data)) != want {
		return nil, fmt.Errorf("got %d bytes, want %d", len(data), want)
	}
	return data, nil
}

// handleFullObjectStream handles full object downloads with pipelined stream decryption.
// This uses io.Pipe to decrypt blocks as they stream from Cloudflare, reducing
// time-to-first-byte and memory usage compared to buffering the entire envelope.
// header is the request-local envelope header GetObject already read for
// single-PUT objects (nil for multipart objects, which have none); the stream
// path reuses it instead of re-reading the same 64 bytes from the backend.
// storedGen is the verified stored-object generation for cache keying.
// singleFetch is the whole-ciphertext buffer GetObject took for a small
// single-PUT object (armor-98864f12); nil when the object streams. When set,
// the table and data regions are sliced out of it locally instead of fetched.
func (h *Handlers) handleFullObjectStream(w http.ResponseWriter, r *http.Request, bucket, key string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time, isMultipart bool, multipartPartSize int64, isNonUniform bool, cumulativePartSizes map[int]int64, header *crypto.EnvelopeHeader, storedGen v3TableKeyParts, singleFetch []byte) {
	ctx := r.Context()

	// Apply prefix for backend operations
	prefixedKey := h.applyPrefix(key)

	blockSize := armorMeta.BlockSize
	blockCount := int(crypto.ComputeBlockCount(plaintextSize, blockSize))

	var hmacTable []byte
	var dataBody io.ReadCloser
	var streamSize int64

	if isMultipart {
		// Multipart object: HMAC table is in sidecar, no embedded header
		manager := h.getMultipartManager(bucket)
		sidecar, err := manager.LoadHMACTable(ctx, key)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load HMAC table from sidecar: %v", err), 500)
			return
		}

		// Concatenate all block HMACs from sidecar
		for _, hmac := range sidecar.BlockHMACs {
			hmacTable = append(hmacTable, hmac...)
		}

		// Data starts at offset 0 (no header), read only the encrypted data
		streamSize = plaintextSize
		dataBody, err = h.backend.GetRange(ctx, bucket, prefixedKey, 0, streamSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object stream: %v", err), 500)
			return
		}
		defer dataBody.Close()
	} else {
		// Single-PUT object: dispatch on version for HMAC/block table format
		// v1/v2: inline HMAC table at HeaderSize + plaintextSize
		// v3: trailer block table at ciphertext_length - 36*blockCount
		//
		// The envelope header was already read and decoded once in GetObject
		// (request-local reuse, armor-c3199f1c); no second header read here.

		if header == nil {
			// Defensive: the single-PUT dispatch always provides the header.
			h.writeError(w, r, "InternalError", "missing envelope header for single-PUT stream", 500)
			return
		}

		if singleFetch != nil {
			// Small single-PUT object: the whole ciphertext is already local
			// (armor-98864f12). Slice the table and data regions out of the
			// buffer instead of two more round trips. Every layout invariant
			// is checked against the buffer bounds here, so a table that
			// disagrees with the fetched length, a shifted layout, or a
			// wrong-sized envelope fails the request before any plaintext
			// exists — the same failure the streaming path would reach on
			// its own reads, one round trip earlier.
			var dataEnd int64
			if header.Version == crypto.Version3 {
				blockTable, err := decodeV3BlockTableFromBuffer(singleFetch, armorMeta.CiphertextSize, blockCount)
				if err != nil {
					h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read v3 block table: %v", err), 500)
					return
				}
				if blockCount > 0 {
					hmacTable = encodeV3BlockTableForTransport(blockTable)
				}
				// blockCount == 0 (an empty envelope): the transport encoder
				// refuses an empty table by design, and the read loop below
				// runs zero iterations either way, so no table is transported.
				dataEnd = int64(crypto.HeaderSize) + blockTable.TotalCiphertextLength64()
			} else {
				hmacTableOffset := crypto.HeaderSize + plaintextSize
				hmacTableSize := int64(blockCount) * crypto.HMACSize
				if hmacTableOffset < crypto.HeaderSize || hmacTableOffset+hmacTableSize > int64(len(singleFetch)) {
					h.writeError(w, r, "InternalError", "stored object shorter than its inline HMAC table", 500)
					return
				}
				hmacTable = singleFetch[hmacTableOffset : hmacTableOffset+hmacTableSize]
				dataEnd = hmacTableOffset
			}
			dataBody = io.NopCloser(bytes.NewReader(singleFetch[crypto.HeaderSize:dataEnd]))
			defer dataBody.Close()
		} else if header.Version == crypto.Version3 {
			// v3: fetch trailer block table and use prefix sums for block offsets
			if armorMeta.CiphertextSize == 0 {
				h.writeError(w, r, "InternalError", "v3 object missing ciphertext size", 500)
				return
			}

			blockTable, err := h.loadV3BlockTable(ctx, bucket, prefixedKey, storedGen, armorMeta.CiphertextSize, blockCount)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read v3 block table: %v", err), 500)
				return
			}

			// Stream data from the start (header + blocks)
			streamSize = int64(crypto.HeaderSize) + int64(blockTable.TotalCiphertextLength())
			dataBody, err = h.backend.GetRange(ctx, bucket, prefixedKey, 0, streamSize)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object stream: %v", err), 500)
				return
			}
			defer dataBody.Close()

			// Read and discard the 64-byte header
			discardBuf := make([]byte, crypto.HeaderSize)
			if _, err := io.ReadFull(dataBody, discardBuf); err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to discard header: %v", err), 500)
				return
			}

			// Encode block table for transport to decryption goroutine
			hmacTable = encodeV3BlockTableForTransport(blockTable)
		} else {
			// v1/v2: inline HMAC table at end of file
			hmacTableOffset := crypto.HeaderSize + plaintextSize
			hmacTableSize := int64(blockCount) * crypto.HMACSize
			dataSize := plaintextSize

			// 1. Prefetch HMAC table (small range read)
			hmacBody, err := h.backend.GetRange(ctx, bucket, prefixedKey, hmacTableOffset, hmacTableSize)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to prefetch HMAC table: %v", err), 500)
				return
			}
			hmacTable, err = io.ReadAll(hmacBody)
			hmacBody.Close()
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read HMAC table: %v", err), 500)
				return
			}

			// 2. Start streaming data from Cloudflare (header + encrypted blocks, stop before HMAC)
			streamSize = crypto.HeaderSize + dataSize
			dataBody, err = h.backend.GetRange(ctx, bucket, prefixedKey, 0, streamSize)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object stream: %v", err), 500)
				return
			}
			defer dataBody.Close()

			// 3. Read and discard the 64-byte header (single-PUT only).
			// The header itself was already decoded from GetObject's
			// request-local read (armor-c3199f1c); the stream re-decode is
			// gone, only the discard remains.
			discard := make([]byte, crypto.HeaderSize)
			if _, err := io.ReadFull(dataBody, discard); err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
				return
			}
		}
	}

	// 4. Set response headers before streaming
	// Note: Content-Length is not set for compressed objects because decompression
	// changes the size dynamically. HTTP will use chunked transfer encoding.
	if !armorMeta.Compressed {
		w.Header().Set("Content-Length", strconv.FormatInt(plaintextSize, 10))
	}
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("X-Armor-Stream", "pipelined")
	if armorMeta.Compressed {
		w.Header().Set("X-Armor-Decompressed", "true")
	}
	w.WriteHeader(http.StatusOK)

	// 5. Stream decrypt using io.Pipe
	pr, pw := io.Pipe()

	// Determine the digest form this object declares so the streaming integrity
	// check below compares like-for-like. Single-PUT objects declare the plain
	// SHA-256 of the whole plaintext (in the envelope header). Multipart objects
	// written since bf-1v2ehf declare the combined per-part digest and carry a
	// block-aligned uniform part size P (x-amz-meta-armor-part-size); reproduce
	// it incrementally with MultipartDigestAccumulator. Legacy multipart objects
	// (no P) carry the empty-string placeholder, which is not a real digest and
	// is skipped below rather than enforced.
	//
	// A non-uniform (ADR-011) object's real upload-part boundaries don't line
	// up with P — MultipartDigestAccumulator's fixed-P chunking would combine
	// the wrong byte ranges — so it's excluded here; decryptNonUniformParts
	// reproduces the combined digest itself, from the real part boundaries it
	// already iterates, straight into wholeHash.
	useCombinedDigest := isMultipart && !isNonUniform && multipartPartSize > 0 && multipartPartSize%int64(blockSize) == 0
	var accumulator *backend.MultipartDigestAccumulator
	if useCombinedDigest {
		accumulator = backend.NewMultipartDigestAccumulator(multipartPartSize, blockSize)
	}

	// Start decryption goroutine
	go func() {
		defer pw.Close()

		wholeHash := sha256.New() // plain whole-object digest (single-PUT / legacy multipart)

		// ADR-011: Non-uniform multipart objects need offset-aware decryption.
		// Other objects retain the v3 single-PUT and v1/v2 block readers.
		if isNonUniform && len(cumulativePartSizes) > 0 {
			if err := h.decryptNonUniformParts(dataBody, pw, plaintextSize, blockSize, decryptor, armorMeta, cumulativePartSizes, hmacTable, accumulator, wholeHash); err != nil {
				pw.CloseWithError(err)
				return
			}
		} else if v3BlockTable, isV3 := decodeV3BlockTableFromTransport(hmacTable); isV3 {
			// v3 single-PUT: read blocks using the trailer's prefix sums.
			for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
				if blockIndex >= v3BlockTable.EntryCount() {
					pw.CloseWithError(fmt.Errorf("block index %d out of range (v3 table has %d entries)", blockIndex, v3BlockTable.EntryCount()))
					return
				}

				entry := v3BlockTable.Entries[blockIndex]
				encryptedBuf := make([]byte, entry.RawLength())
				n, err := io.ReadFull(dataBody, encryptedBuf)
				if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
					pw.CloseWithError(fmt.Errorf("read error at v3 block %d: %w", blockIndex, err))
					return
				}
				if n == 0 {
					break
				}
				encryptedBuf = encryptedBuf[:n]

				expectedHMAC := entry.HMAC[:]
				mac := hmac.New(sha256.New, decryptor.HMACKey())
				partBytes := make([]byte, 2)
				binary.BigEndian.PutUint16(partBytes, 0) // part=0 for single-PUT
				mac.Write(partBytes)
				blockBytes := make([]byte, 4)
				binary.BigEndian.PutUint32(blockBytes, uint32(blockIndex))
				mac.Write(blockBytes)
				mac.Write(encryptedBuf)
				if !hmac.Equal(mac.Sum(nil), expectedHMAC) {
					pw.CloseWithError(fmt.Errorf("v3 block %d: HMAC verification failed", blockIndex))
					return
				}

				decrypted, err := crypto.DecryptBlockV3(decryptor.DEK(), armorMeta.IV, 0, uint32(blockIndex), encryptedBuf, expectedHMAC, blockSize)
				if err != nil {
					pw.CloseWithError(fmt.Errorf("v3 block %d decryption failed: %w", blockIndex, err))
					return
				}
				if entry.IsCompressed() {
					decrypted, err = crypto.DecompressBlock(decrypted, true)
					if err != nil {
						pw.CloseWithError(fmt.Errorf("v3 block %d decompression failed: %w", blockIndex, err))
						return
					}
				}

				if accumulator != nil {
					accumulator.WriteBlock(decrypted, blockIndex == blockCount-1)
				} else {
					wholeHash.Write(decrypted)
				}
				if _, err := pw.Write(decrypted); err != nil {
					pw.CloseWithError(fmt.Errorf("write error at v3 block %d: %w", blockIndex, err))
					return
				}
			}
		} else {
			// v1/v2: optimized block-by-block decryption with inline HMAC table
			encryptedBuf := make([]byte, blockSize)
			for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
				// Calculate actual block size (last block may be smaller)
				remaining := plaintextSize - int64(blockIndex)*int64(blockSize)
				actualBlockSize := int(min64(int64(blockSize), remaining))

				// Read encrypted block
				encryptedBuf = encryptedBuf[:actualBlockSize]
				n, err := io.ReadFull(dataBody, encryptedBuf)
				if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
					pw.CloseWithError(fmt.Errorf("read error at block %d: %w", blockIndex, err))
					return
				}
				if n == 0 {
					break
				}
				encryptedBuf = encryptedBuf[:n]

				// Verify HMAC
				hmacOffset := blockIndex * crypto.HMACSize
				if hmacOffset+crypto.HMACSize > len(hmacTable) {
					pw.CloseWithError(fmt.Errorf("HMAC table too short at block %d", blockIndex))
					return
				}
				expectedHMAC := hmacTable[hmacOffset : hmacOffset+crypto.HMACSize]

				// ADR-011: For non-uniform parts, skip HMAC verification for placeholder (zero) HMACs
				// These are boundary blocks that span multiple parts and couldn't have HMACs
				// computed during upload. The decryption is still correct because each byte
				// was encrypted with the proper CTR counter.
				if !isPlaceholderHMAC(expectedHMAC) {
					mac := hmac.New(sha256.New, decryptor.HMACKey())
					indexBytes := make([]byte, 4)
					binary.BigEndian.PutUint32(indexBytes, uint32(blockIndex))
					mac.Write(indexBytes)
					mac.Write(encryptedBuf)
					computed := mac.Sum(nil)

					if !hmac.Equal(computed, expectedHMAC) {
						pw.CloseWithError(fmt.Errorf("block %d: HMAC verification failed", blockIndex))
						return
					}
				}

				// Decrypt block (need to use CTR stream)
				decrypted := make([]byte, n)
				ctr := makeCounter(armorMeta.IV, uint32(blockIndex), armorMeta.Version, blockSize)
				stream := cipher.NewCTR(decryptor.CipherBlock(), ctr)
				stream.XORKeyStream(decrypted, encryptedBuf)

				// Update the plaintext digest for end-of-stream verification. For
				// multipart objects with a known part size, fold the block into the
				// per-part accumulator; otherwise hash the whole plaintext.
				isLastBlock := blockIndex == blockCount-1
				if accumulator != nil {
					accumulator.WriteBlock(decrypted, isLastBlock)
				} else {
					wholeHash.Write(decrypted)
				}

				// Write plaintext to pipe
				if _, err := pw.Write(decrypted); err != nil {
					pw.CloseWithError(fmt.Errorf("write error at block %d: %w", blockIndex, err))
					return
				}
			}
		}

		// Verify the plaintext digest declared for this object.
		var computedSHA []byte
		if accumulator != nil {
			computedSHA, _ = hex.DecodeString(accumulator.Sum())
		} else {
			computedSHA = wholeHash.Sum(nil)
		}
		var expectedSHA []byte
		enforce := true
		if isMultipart {
			// Multipart objects carry the digest in metadata. Legacy uploads
			// (pre bf-1v2ehf) carry the empty-string placeholder, which is not a
			// real checksum — never enforce it.
			if backend.IsPlaceholderPlaintextSHA(armorMeta.PlaintextSHA) {
				enforce = false
			}
			expectedSHA, _ = hex.DecodeString(armorMeta.PlaintextSHA)
		} else {
			// For single-PUT objects, use the digest from the envelope header.
			expectedSHA = header.PlaintextSHA[:]
		}
		if enforce && len(expectedSHA) > 0 && !bytes.Equal(computedSHA, expectedSHA) {
			pw.CloseWithError(fmt.Errorf("plaintext SHA-256 mismatch"))
			return
		}
	}()

	// 6. Stream plaintext to client
	// If the object is compressed, decompress it before writing to response
	if armorMeta.Compressed {
		var decompressor io.ReadCloser
		var err error

		// Select decompression method based on compression type
		switch armorMeta.CompressionType {
		case backend.CompressionGzip:
			// Create gzip decoder for streaming decompression
			decompressor, err = gzip.NewReader(pr)
			if err != nil {
				// Decompression setup failed - close pipe and return error
				pr.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create gzip decompression decoder: %v", err), 500)
				return
			}
		case backend.CompressionZlib:
			// Create zlib decoder for streaming decompression
			decompressor, err = zlib.NewReader(pr)
			if err != nil {
				// Decompression setup failed - close pipe and return error
				pr.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create zlib decompression decoder: %v", err), 500)
				return
			}
		case backend.CompressionZstd:
			// Create zstd decoder for streaming decompression
			zstdDecoder, err := zstd.NewReader(pr)
			if err != nil {
				// Decompression setup failed - close pipe and return error
				pr.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create zstd decompression decoder: %v", err), 500)
				return
			}
			// Wrap in zstdReadCloser to implement io.ReadCloser (zstd.Decoder.Close() doesn't return error)
			decompressor = &zstdReadCloser{Decoder: zstdDecoder}
		default:
			// Unknown compression type - close pipe and return error
			pr.Close()
			h.writeError(w, r, "InternalError", fmt.Sprintf("Unknown compression type: %s", armorMeta.CompressionType), 500)
			return
		}
		defer decompressor.Close()

		// Stream decompressed data to client
		_, err = io.Copy(w, decompressor)
		if err != nil {
			// Decompression failed during streaming
			// Log the error with context for debugging
			// Since headers are already sent, we can't change HTTP status,
			// but we log meaningful error information
			log.Printf("GET %s/%s: decompression streaming error: %v (compression type: %s)",
				bucket, key, err, armorMeta.CompressionType)

			// Classify the error to determine root cause
			errMsg := err.Error()
			if strings.Contains(errMsg, "unexpected EOF") || strings.Contains(errMsg, "EOF") {
				log.Printf("GET %s/%s: likely cause: truncated or corrupted compressed data",
					bucket, key)
			} else if strings.Contains(errMsg, "corrupt") || strings.Contains(errMsg, "invalid") {
				log.Printf("GET %s/%s: likely cause: data corruption in storage",
					bucket, key)
			} else {
				log.Printf("GET %s/%s: likely cause: decompression infrastructure error",
					bucket, key)
			}
			// Connection will be closed; client receives partial response
			return
		}
	} else {
		// Stream plaintext directly to client
		_, err := io.Copy(w, pr)
		if err != nil {
			log.Printf("GET %s/%s: plaintext streaming error: %v", bucket, key, err)
			return
		}
	}
}

// decryptNonUniformParts handles decryption for non-uniform multipart uploads (ADR-011).
// It decrypts part-by-part using offset-aware decryption, which is necessary because
// parts may start at arbitrary byte offsets (not just block boundaries).
func (h *Handlers) decryptNonUniformParts(dataBody io.ReadCloser, pw *io.PipeWriter, plaintextSize int64, blockSize int, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, cumulativePartSizes map[int]int64, hmacTable []byte, accumulator *backend.MultipartDigestAccumulator, wholeHash hash.Hash) error {
	// Unwrap DEK using fingerprint with ring fallback
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		return h.keyManager.GetMEKByFingerprint(keyID, fingerprint)
	}

	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		mek, err := h.keyManager.GetMEKByID(armorMeta.KeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to get active key: %w", err)
		}
		dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
		if err == nil {
			return dek, nil
		}

		ring := h.keyManager.Ring(armorMeta.KeyID)
		for _, ringEntry := range ring {
			dek, err := crypto.UnwrapDEK(ringEntry.MEK, wrappedDEK)
			if err == nil {
				return dek, nil
			}
		}

		return nil, fmt.Errorf("no key in active or ring can unwrap DEK")
	}

	wrappedDEKStr := base64.StdEncoding.EncodeToString(armorMeta.WrappedDEK)
	if armorMeta.MEKFingerprint != "" {
		wrappedDEKStr = fmt.Sprintf("v2:%s:%s", armorMeta.MEKFingerprint, wrappedDEKStr)
	}

	dek, _, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		return fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Create offset decryptor for arbitrary byte offset decryption
	offsetDecryptor, err := crypto.NewOffsetDecryptor(
		dek,
		armorMeta.IV,
		blockSize,
		uint8(armorMeta.Version),
	)
	if err != nil {
		return fmt.Errorf("failed to create offset decryptor: %w", err)
	}

	// Sort part numbers
	partNumbers := make([]int, 0, len(cumulativePartSizes))
	for pn := range cumulativePartSizes {
		partNumbers = append(partNumbers, pn)
	}
	for i := 0; i < len(partNumbers); i++ {
		for j := i + 1; j < len(partNumbers); j++ {
			if partNumbers[i] > partNumbers[j] {
				partNumbers[i], partNumbers[j] = partNumbers[j], partNumbers[i]
			}
		}
	}

	// Read and decrypt each part
	for _, partNumber := range partNumbers {
		if partNumber == 0 {
			continue // Part 0 doesn't exist (parts are 1-indexed)
		}

		partOffset := cumulativePartSizes[partNumber]
		var partSize int64

		// Calculate part size from next part's offset or total size
		if nextOffset, ok := cumulativePartSizes[partNumber+1]; ok {
			partSize = nextOffset - partOffset
		} else {
			// Last part - use remaining size
			partSize = plaintextSize - partOffset
		}

		if partSize <= 0 {
			continue
		}

		// Read encrypted part data
		encryptedPart := make([]byte, partSize)
		n, err := io.ReadFull(dataBody, encryptedPart)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read error for part %d: %w", partNumber, err)
		}
		if n == 0 {
			break
		}
		encryptedPart = encryptedPart[:n]

		// Decrypt this part using offset-aware decryption
		decryptedPart, err := offsetDecryptor.DecryptFromOffset(encryptedPart, partOffset)
		if err != nil {
			return fmt.Errorf("decrypt error for part %d at offset %d: %w", partNumber, partOffset, err)
		}

		// Update digest. CompleteMultipartUpload stores the combined per-part
		// digest (backend.CombinePartPlaintextSHAs): each upload part's plaintext
		// SHA-256, fed as raw digest bytes into one SHA-256 in ascending part
		// order. MultipartDigestAccumulator reproduces that only for the ADR-015
		// uniform case (chunking at fixed P boundaries); a genuinely non-uniform
		// object's real part boundaries don't line up with P, so reproduce the
		// combined digest directly from the actual parts this loop already
		// iterates in order: hash each decrypted part and feed that digest into
		// wholeHash, which callers.go then reads via wholeHash.Sum(nil) for
		// non-uniform objects instead of the accumulator.
		partDigest := sha256.Sum256(decryptedPart)
		wholeHash.Write(partDigest[:])

		// Write decrypted plaintext to pipe
		if _, err := pw.Write(decryptedPart); err != nil {
			return fmt.Errorf("write error for part %d: %w", partNumber, err)
		}
	}

	return nil
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

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// zstdReadCloser wraps zstd.Decoder to implement io.ReadCloser
// zstd.Decoder.Close() doesn't return error, so we adapt it
type zstdReadCloser struct {
	Decoder *zstd.Decoder
}

func (z *zstdReadCloser) Read(p []byte) (n int, err error) {
	return z.Decoder.Read(p)
}

func (z *zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}

// handleRangeRequest handles range read requests.
// handleRangeRequest handles v1/v2 range requests and dispatches v3
// single-PUT ranges to handleV3RangeRequest. header is the request-local
// envelope header GetObject already read (nil for multipart objects);
// storedGen is the verified stored-object generation for cache keying
// (armor-c3199f1c).
func (h *Handlers) handleRangeRequest(w http.ResponseWriter, r *http.Request, bucket, key string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time, isMultipart bool, header *crypto.EnvelopeHeader, storedGen v3TableKeyParts) {
	ctx := r.Context()

	// Apply prefix for backend operations
	prefixedKey := h.applyPrefix(key)

	// Parse range header (bytes=start-end)
	rangeHeader := r.Header.Get("Range")
	start, end, err := parseRangeHeader(rangeHeader, plaintextSize)
	if err != nil {
		h.writeError(w, r, "InvalidRange", fmt.Sprintf("Invalid range: %v", err), 400)
		return
	}

	// Check if this is a Parquet footer request and we have it cached
	// DuckDB reads footer in two steps: last 8 bytes, then footer body
	// Footer is at the end of the file: [footer_metadata][footer_length (4B)][PAR1 (4B)]
	if h.footerCache != nil && end >= plaintextSize-8 {
		// This range includes the end of the file - could be a footer read
		if footer, ok := h.footerCache.Get(bucket, key, armorMeta.ETag); ok {
			// We have a cached footer, serve from cache
			footerStart := plaintextSize - int64(len(footer))
			if start >= footerStart {
				// Request is entirely within the cached footer
				offset := start - footerStart
				footerEnd := offset + (end - start + 1)
				if footerEnd <= int64(len(footer)) {
					plaintext := footer[offset:footerEnd]
					w.Header().Set("Content-Length", strconv.FormatInt(int64(len(plaintext)), 10))
					w.Header().Set("Content-Type", armorMeta.ContentType)
					w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
					w.Header().Set("Accept-Ranges", "bytes")
					w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, plaintextSize))
					w.Header().Set("X-Armor-Footer-Cache", "HIT")
					w.WriteHeader(http.StatusPartialContent)
					w.Write(plaintext)
					return
				}
			}
		}
	}

	var encrypted, hmacTable []byte
	var startBlockIndex int

	if isMultipart {
		// Multipart object: load HMAC table from sidecar, no embedded header
		manager := h.getMultipartManager(bucket)
		sidecar, err := manager.LoadHMACTable(ctx, key)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load HMAC table from sidecar: %v", err), 500)
			return
		}

		// Concatenate all block HMACs from sidecar
		for _, hmac := range sidecar.BlockHMACs {
			hmacTable = append(hmacTable, hmac...)
		}

		// Calculate block range for this request
		blockSize := armorMeta.BlockSize
		startBlockIndex = int(start / int64(blockSize))
		endBlockIndex := int(end / int64(blockSize))

		// Clamp to valid range
		blockCount := int(crypto.ComputeBlockCount(plaintextSize, blockSize))
		if endBlockIndex >= blockCount {
			endBlockIndex = blockCount - 1
		}

		// Calculate encrypted data range (no header offset for multipart)
		dataOffset := int64(startBlockIndex * blockSize)
		lastBlockEnd := int64((endBlockIndex + 1) * blockSize)
		if lastBlockEnd > plaintextSize {
			lastBlockEnd = plaintextSize
		}
		dataLength := lastBlockEnd - dataOffset

		// Fetch encrypted blocks
		encryptedBody, err := h.backend.GetRange(ctx, bucket, prefixedKey, dataOffset, dataLength)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("failed to fetch encrypted blocks: %v", err), 500)
			return
		}
		defer encryptedBody.Close()

		encrypted, err = io.ReadAll(encryptedBody)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("failed to read encrypted blocks: %v", err), 500)
			return
		}
	} else {
		// Single-PUT object: check version for HMAC/block table format.
		// The envelope header was already read and decoded once in GetObject
		// (request-local reuse, armor-c3199f1c); no second header read here.
		if header == nil {
			// Defensive: the single-PUT dispatch always provides the header.
			h.writeError(w, r, "InternalError", "missing envelope header for single-PUT range", 500)
			return
		}

		if header.Version == crypto.Version3 {
			// v3: use dedicated v3 range handler
			if armorMeta.CiphertextSize == 0 {
				h.writeError(w, r, "InternalError", "v3 object missing ciphertext size for range request", 500)
				return
			}
			h.handleV3RangeRequest(w, r, bucket, key, prefixedKey, decryptor, armorMeta, plaintextSize, armorMeta.CiphertextSize, lastModified, start, end, storedGen)
			return
		}

		// v1/v2: inline HMAC table at end of file
		// Translate range to encrypted blocks
		translation, err := crypto.TranslateRange(start, end, plaintextSize, armorMeta.BlockSize, crypto.HeaderSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to translate range: %v", err), 500)
			return
		}

		// Fetch encrypted blocks and HMAC table in parallel using errgroup.
		// This cuts range-read latency nearly in half for cache misses.
		g, gctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			encryptedBody, err := h.backend.GetRange(gctx, bucket, prefixedKey, translation.DataOffset, translation.DataLength)
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
			hmacBody, err := h.backend.GetRange(gctx, bucket, prefixedKey, translation.HMACOffset, translation.HMACLength)
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
			h.writeError(w, r, "InternalError", err.Error(), 500)
			return
		}
	}

	// Decrypt range (DecryptRange internally calculates block indices from plaintext offsets)
	// For multipart objects, hmacTableIsFull=true (all HMACs from sidecar).
	// For single-PUT objects, hmacTableIsFull=false (only range HMACs from embedded table).
	plaintext, err := decryptor.DecryptRange(encrypted, hmacTable, start, end, plaintextSize, isMultipart)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decrypt range: %v", err), 500)
		return
	}

	// Cache Parquet footer if this looks like a footer read
	// Footer is detected by: 1) Reading near end of file, 2) Data ends with "PAR1" magic
	if h.footerCache != nil && end >= plaintextSize-8 && len(plaintext) >= 8 {
		if backend.IsParquetFile(plaintext[len(plaintext)-4:]) {
			// This is a Parquet file, try to cache the full footer
			// If we just read the last 8 bytes, cache it for footer length detection
			// If we read more, it might be the full footer
			h.cacheParquetFooter(ctx, bucket, key, armorMeta, plaintext, plaintextSize)
		}
	}

	// Set response headers
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(plaintext)), 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, plaintextSize))
	w.WriteHeader(http.StatusPartialContent)
	w.Write(plaintext)
}

// cacheParquetFooter caches Parquet footer data for faster subsequent reads.
func (h *Handlers) cacheParquetFooter(ctx context.Context, bucket, key string, armorMeta *backend.ARMORMetadata, plaintext []byte, plaintextSize int64) {
	// If we got the last 8 bytes, we can determine footer length
	if len(plaintext) == 8 {
		// This is the footer length read - cache a small marker
		// The actual footer will be cached on the next read
		return
	}

	// Check if we have the full footer by verifying PAR1 magic at end
	if len(plaintext) >= 8 && backend.IsParquetFile(plaintext[len(plaintext)-4:]) {
		// We have at least part of the footer
		// Parse footer length from the last 8 bytes if available
		footerRange, err := backend.GetParquetFooterRange(plaintext[len(plaintext)-8:], plaintextSize)
		if err != nil {
			return
		}

		// Check if we have the complete footer
		footerStart := plaintextSize - int64(footerRange.Length) - 8
		requestStart := plaintextSize - int64(len(plaintext))

		if requestStart <= footerStart {
			// We have the complete footer, extract and cache it
			footerOffset := footerStart - requestStart
			if footerOffset >= 0 && int(footerOffset)+footerRange.Length+8 <= len(plaintext) {
				footer := plaintext[footerOffset : int(footerOffset)+footerRange.Length+8]
				h.footerCache.Set(bucket, key, armorMeta.ETag, footer)
			}
		}
	}
}

// checkConditionalRequest evaluates conditional headers and returns the appropriate
// response status. Returns 0 if the request should proceed normally.
// Supports: If-Match, If-None-Match, If-Modified-Since, If-Unmodified-Since
func checkConditionalRequest(r *http.Request, etag string, lastModified time.Time) int {
	ifMatch := r.Header.Get("If-Match")
	ifNoneMatch := r.Header.Get("If-None-Match")
	ifModifiedSince := r.Header.Get("If-Modified-Since")
	ifUnmodifiedSince := r.Header.Get("If-Unmodified-Since")

	// Normalize ETag (remove quotes if present for comparison)
	normalizedETag := strings.Trim(etag, `"`)

	// If-Match: Return 412 Precondition Failed if ETag doesn't match
	if ifMatch != "" {
		// If-Match can be "*" (match any) or a comma-separated list of ETags
		if ifMatch == "*" {
			// Match any existing resource - proceed
		} else {
			// Parse comma-separated ETags
			etags := strings.Split(ifMatch, ",")
			matched := false
			for _, e := range etags {
				// Trim space first, then quotes (order matters for " value" case)
				e = strings.Trim(strings.TrimSpace(e), `"`)
				if e == normalizedETag {
					matched = true
					break
				}
			}
			if !matched {
				return http.StatusPreconditionFailed
			}
		}
	}

	// If-Unmodified-Since: Return 412 Precondition Failed if modified since date
	if ifUnmodifiedSince != "" {
		if t, err := http.ParseTime(ifUnmodifiedSince); err == nil {
			if lastModified.After(t) {
				return http.StatusPreconditionFailed
			}
		}
	}

	// If-None-Match: Return 304 Not Modified if ETag matches
	if ifNoneMatch != "" {
		// If-None-Match can be "*" (match any) or a comma-separated list of ETags
		if ifNoneMatch == "*" {
			return http.StatusNotModified
		}
		// Parse comma-separated ETags
		etags := strings.Split(ifNoneMatch, ",")
		for _, e := range etags {
			// Trim space first, then quotes (order matters for " value" case)
			e = strings.Trim(strings.TrimSpace(e), `"`)
			if e == normalizedETag {
				return http.StatusNotModified
			}
		}
	}

	// If-Modified-Since: Return 304 Not Modified if not modified since date
	// Only applies if If-None-Match is not present (per RFC 7232)
	if ifModifiedSince != "" && ifNoneMatch == "" {
		if t, err := http.ParseTime(ifModifiedSince); err == nil {
			// Use >= comparison per RFC 7232
			if !lastModified.After(t) {
				return http.StatusNotModified
			}
		}
	}

	return 0
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

	// Clamp end per RFC 7233: a range ending beyond the file size is satisfiable
	// as long as start is within bounds. DuckDB httpfs requests aligned blocks
	// (e.g. bytes=0-131071) regardless of actual file size.
	if end >= totalSize {
		end = totalSize - 1
	}

	if start < 0 || start >= totalSize || end < start {
		return 0, 0, fmt.Errorf("range out of bounds")
	}

	return start, end, nil
}

// HeadObject handles S3 HeadObject.
func (h *Handlers) HeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()
	prefixedKey := h.applyPrefix(key)

	// Fast path: serve from the in-memory manifest index when available,
	// avoiding a B2 HeadObject round-trip for entries that carry user metadata.
	// Older index entries predate that field and fall back to the stored object
	// (or multipart manifest) so existing objects keep their S3 headers.
	if h.manifest != nil {
		if entry, ok := h.manifest.Lookup(bucket, key); ok {
			if entry.UserMetadata != nil {
				writeUserMetadataHeaders(w, entry.UserMetadata)
			} else if _, manifestMetadata, err := h.readManifest(ctx, bucket, key); err == nil {
				writeUserMetadataHeaders(w, manifestMetadata)
			} else if backend.IsNotFound(err) {
				// Legacy single-PUT entries have no per-key manifest. Their
				// stored ciphertext still carries the client headers.
				info, headErr := h.backend.Head(ctx, bucket, prefixedKey)
				if headErr != nil {
					code, _, status := backendFailureResponse(headErr)
					h.writeBackendError(w, r, code, fmt.Sprintf("Failed to read object metadata: %v", headErr), status, headErr)
					return
				}
				writeUserMetadataHeaders(w, info.Metadata)
			} else {
				code, _, status := backendFailureResponse(err)
				h.writeBackendError(w, r, code, fmt.Sprintf("Failed to read manifest metadata: %v", err), status, err)
				return
			}
			if status := checkConditionalRequest(r, entry.ETag, entry.LastModified); status != 0 {
				if status == http.StatusNotModified {
					w.Header().Set("ETag", fmt.Sprintf(`"%s"`, entry.ETag))
					w.Header().Set("Last-Modified", entry.LastModified.UTC().Format(http.TimeFormat))
					w.WriteHeader(status)
				} else {
					h.writeError(w, r, "PreconditionFailed", "Precondition failed", status)
				}
				return
			}
			w.Header().Set("Content-Length", strconv.FormatInt(entry.PlaintextSize, 10))
			w.Header().Set("Content-Type", entry.ContentType)
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, entry.ETag))
			w.Header().Set("Accept-Ranges", "bytes")
			w.Header().Set("Last-Modified", entry.LastModified.UTC().Format(http.TimeFormat))
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Manifest miss or disabled: mirror GetObject and consult the manifest
	// object before treating the key as legacy. Multipart ciphertext carries
	// no stamped metadata (ADR-016 moved metadata truth into the manifest
	// object), so a bare ciphertext Head here would serve a zero-value
	// ETag/Content-Type and the ciphertext size — exactly what made aws-cli's
	// If-Match multipart download 412 on every part. A backend failure other
	// than "no manifest was ever written" surfaces with the same
	// classification GetObject uses; falling back would misreport a multipart
	// object as a passthrough object.
	manifestBody, manifestMeta, manifestErr := h.readManifest(ctx, bucket, key)
	if manifestErr != nil && !backend.IsNotFound(manifestErr) {
		code, _, status := backendFailureResponse(manifestErr)
		h.writeBackendError(w, r, code, fmt.Sprintf("Failed to read manifest: %v", manifestErr), status, manifestErr)
		return
	}
	if manifestErr == nil && manifestBody != nil {
		h.headObjectFromManifest(w, r, bucket, manifestBody, manifestMeta)
		return
	}

	// No manifest: legacy single-PUT or passthrough object.
	info, err := h.backend.Head(ctx, bucket, prefixedKey)
	if err != nil {
		code, message, status := backendFailureResponse(err)
		h.writeBackendError(w, r, code, message, status, err)
		return
	}

	// Determine ETag and content info based on encryption status
	var etag string
	var contentLength int64
	var contentType string
	if info.IsARMOREncrypted {
		if am, ok := backend.ParseARMORMetadata(info.Metadata); ok {
			// Warm the metadata cache for ListObjectsV2 size resolution,
			// bound to this stored generation (info.ETag).
			h.cache.SetVersioned(bucket, key, info.ETag, am)
			etag = am.ETag
			contentLength = am.PlaintextSize
			contentType = am.ContentType
		} else {
			etag = info.ETag
			contentLength = info.Size
			contentType = info.ContentType
		}
	} else {
		etag = info.ETag
		contentLength = info.Size
		contentType = info.ContentType
	}

	// Check conditional request headers
	if status := checkConditionalRequest(r, etag, info.LastModified); status != 0 {
		if status == http.StatusNotModified {
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
			w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
			w.WriteHeader(status)
		} else {
			h.writeError(w, r, "PreconditionFailed", "Precondition failed", status)
		}
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	writeUserMetadataHeaders(w, info.Metadata)

	w.WriteHeader(http.StatusOK)
}

// headObjectFromManifest serves HEAD metadata from a manifest object, applying
// the same quarantine and ciphertext-freshness gates as the GetObject read
// path so HEAD and GET agree on what a manifest describes (a HEAD that 200s
// where GET 500s would send conditional-request clients into 412 loops). The
// one authoritative ciphertext HEAD doubles as the Last-Modified source, the
// same trade GetObject makes (armor-c3199f1c).
func (h *Handlers) headObjectFromManifest(w http.ResponseWriter, r *http.Request, bucket string, manifestBody *backend.ManifestBody, manifestMeta map[string]string) {
	ctx := r.Context()

	if reason, quarantined := manifestQuarantineState(manifestMeta); quarantined {
		h.writeError(w, r, "AccessDenied", fmt.Sprintf("object is quarantined and cannot be read; operator action is required: %s", reason), http.StatusForbidden)
		return
	}

	ciphertextInfo, headErr := h.headCiphertextForFreshness(ctx, bucket, manifestBody.CiphertextObject, manifestMeta[metaCompletedAt])
	if headErr != nil {
		var freshErr *ciphertextFreshnessError
		if errors.As(headErr, &freshErr) {
			// Retryable by design; see GetObject's manifest path.
			h.writeError(w, r, "InternalError", fmt.Sprintf("manifest verification failed; operator repair or quarantine is required: %v", freshErr.err), 500)
		} else {
			h.writeBackendError(w, r, "InternalError", fmt.Sprintf("Failed to head ciphertext object: %v", headErr), 500, headErr)
		}
		return
	}

	armorMeta, ok := backend.ParseARMORMetadata(manifestMeta)
	if !ok {
		h.writeError(w, r, "InternalError", "Failed to parse ARMOR metadata from manifest", 500)
		return
	}

	// Check conditional request headers
	if status := checkConditionalRequest(r, armorMeta.ETag, ciphertextInfo.LastModified); status != 0 {
		if status == http.StatusNotModified {
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
			w.Header().Set("Last-Modified", ciphertextInfo.LastModified.UTC().Format(http.TimeFormat))
			w.WriteHeader(status)
		} else {
			h.writeError(w, r, "PreconditionFailed", "Precondition failed", status)
		}
		return
	}

	w.Header().Set("Content-Length", strconv.FormatInt(armorMeta.PlaintextSize, 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", ciphertextInfo.LastModified.UTC().Format(http.TimeFormat))
	writeUserMetadataHeaders(w, manifestMeta)

	w.WriteHeader(http.StatusOK)
}

// DeleteObject handles S3 DeleteObject.
func (h *Handlers) DeleteObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	prefixedKey := h.applyPrefix(key)
	if err := h.backend.Delete(ctx, bucket, prefixedKey); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to delete: %v", err), 500)
		return
	}

	// Remove from manifest
	if h.manifest != nil {
		h.manifest.RecordDelete(bucket, key)
	}

	// Invalidate metadata cache and list cache
	h.cache.Delete(bucket, key)
	if h.listCache != nil {
		h.listCache.InvalidateForWrite(bucket, key)
	}

	w.WriteHeader(http.StatusNoContent)
}

// CopyObject handles S3 CopyObject with DEK re-wrapping for ARMOR-encrypted objects.
// This supports:
// - Renaming files (same bucket, different key)
// - Copying files (potentially different bucket)
// - Key rotation (re-wraps DEK with current MEK)
func (h *Handlers) CopyObject(w http.ResponseWriter, r *http.Request, dstBucket, dstKey string) {
	ctx := r.Context()

	// Parse copy source header
	copySource := r.Header.Get("x-amz-copy-source")
	if copySource == "" {
		h.writeError(w, r, "InvalidRequest", "Missing x-amz-copy-source header", 400)
		return
	}

	// Parse source bucket and key
	// Format: /bucket/key or bucket/key
	srcBucket, srcKey := parseCopySource(copySource)
	if srcBucket == "" || srcKey == "" {
		h.writeError(w, r, "InvalidCopySource", "Invalid copy source format", 400)
		return
	}

	// A legacy bucket name in the copy source names the same objects as
	// config.Bucket once the tenant has been consolidated into it, so it
	// resolves the same way the URL bucket does. Without this the copy fails
	// with NoSuchKey against a bucket that no longer holds anything.
	srcBucket = h.config.ResolveBucket(srcBucket)

	// Apply prefix for backend operations
	srcPrefixedKey := h.applyPrefix(srcKey)
	dstPrefixedKey := h.applyPrefix(dstKey)

	// Get source object metadata
	srcInfo, err := h.backend.Head(ctx, srcBucket, srcPrefixedKey)
	if err != nil {
		h.writeError(w, r, "NoSuchKey", fmt.Sprintf("Source object not found: %v", err), 404)
		return
	}

	// Check metadata directive
	metadataDirective := r.Header.Get("x-amz-metadata-directive")
	replaceMetadata := metadataDirective == "REPLACE"

	// Build response XML structure
	type CopyObjectResult struct {
		XMLName      xml.Name `xml:"CopyObjectResult"`
		LastModified string   `xml:"LastModified"`
		ETag         string   `xml:"ETag"`
	}

	// Handle ARMOR-encrypted objects
	if srcInfo.IsARMOREncrypted {
		// Parse ARMOR metadata
		armorMeta, ok := backend.ParseARMORMetadata(srcInfo.Metadata)
		if !ok {
			h.writeError(w, r, "InternalError", "Failed to parse ARMOR metadata", 500)
			return
		}

		// Get the source MEK using the key ID from metadata
		if _, err := h.keyManager.GetMEKByID(armorMeta.KeyID); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get source decryption key: %v", err), 500)
			return
		}

		// Unwrap DEK with source MEK using fingerprint with ring fallback
		lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
			return h.keyManager.GetMEKByFingerprint(keyID, fingerprint)
		}

		legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
			mek, err := h.keyManager.GetMEKByID(armorMeta.KeyID)
			if err != nil {
				return nil, fmt.Errorf("failed to get active key: %w", err)
			}
			dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
			if err == nil {
				return dek, nil
			}

			ring := h.keyManager.Ring(armorMeta.KeyID)
			for _, ringEntry := range ring {
				dek, err := crypto.UnwrapDEK(ringEntry.MEK, wrappedDEK)
				if err == nil {
					return dek, nil
				}
			}

			return nil, fmt.Errorf("no key in active or ring can unwrap DEK")
		}

		wrappedDEKStr := base64.StdEncoding.EncodeToString(armorMeta.WrappedDEK)
		if armorMeta.MEKFingerprint != "" {
			wrappedDEKStr = fmt.Sprintf("v2:%s:%s", armorMeta.MEKFingerprint, wrappedDEKStr)
		}

		dek, _, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to unwrap DEK: %v", err), 500)
			return
		}

		// Get the destination MEK for the target key
		dstMEK, dstKeyID, err := h.keyManager.GetMEK(dstKey)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get destination encryption key: %v", err), 500)
			return
		}

		// Re-wrap DEK with destination MEK (handles key rotation and cross-prefix copy)
		// Wrap with destination MEK and encode fingerprint in v2 format
		wrappedDEKStr, err = crypto.WrapDEKWithFingerprint(dstMEK, dek)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to re-wrap DEK: %v", err), 500)
			return
		}

		// Parse v2 format to extract fingerprint and wrapped DEK bytes
		parts := strings.SplitN(wrappedDEKStr, ":", 3)
		if len(parts) != 3 || parts[0] != "v2" {
			h.writeError(w, r, "InternalError", "wrapped DEK format validation failed", 500)
			return
		}
		dstMekFingerprint := parts[1]
		wrappedDEK, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode wrapped DEK: %v", err), 500)
			return
		}

		// Build new metadata with re-wrapped DEK and destination key ID
		newMeta := (&backend.ARMORMetadata{
			Version:        armorMeta.Version,
			BlockSize:      armorMeta.BlockSize,
			PlaintextSize:  armorMeta.PlaintextSize,
			ContentType:    armorMeta.ContentType,
			IV:             armorMeta.IV,
			WrappedDEK:     wrappedDEK,
			MEKFingerprint: dstMekFingerprint,
			PlaintextSHA:   armorMeta.PlaintextSHA,
			ETag:           armorMeta.ETag,
			KeyID:          dstKeyID,
		}).ToMetadata()

		// Handle REPLACE directive - copy custom metadata headers from request
		if replaceMetadata {
			// Check for new content-type in request
			if ct := r.Header.Get("Content-Type"); ct != "" {
				newMeta["x-amz-meta-armor-content-type"] = ct
			}
			// Copy any additional custom headers from request
			for k, v := range r.Header {
				if strings.HasPrefix(k, "X-Amz-Meta-") && !strings.HasPrefix(k, "X-Amz-Meta-Armor-") {
					newMeta[strings.ToLower(k)] = v[0]
				}
			}
		}

		// Perform server-side copy with updated metadata
		if err := h.backend.Copy(ctx, srcBucket, srcPrefixedKey, dstBucket, dstPrefixedKey, newMeta, true); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Copy failed: %v", err), 500)
			return
		}

		// Record provenance for the copy
		if h.provenance != nil && h.provenance.ShouldRecord(dstKey) {
			h.recordProvenanceUpload(ctx, dstKey, armorMeta.PlaintextSHA, "copy")
		}

		// Invalidate cache for destination
		h.cache.Delete(dstBucket, dstKey)
		h.footerCache.Delete(dstBucket, dstKey)

		// Return success response
		result := CopyObjectResult{
			LastModified: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
			ETag:         fmt.Sprintf(`"%s"`, armorMeta.ETag),
		}

		output, err := xml.Marshal(result)
		if err != nil {
			h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
			return
		}

		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
		w.Write(output)
		return
	}

	// Non-ARMOR object - pass through copy
	var meta map[string]string
	if replaceMetadata {
		meta = make(map[string]string)
		if ct := r.Header.Get("Content-Type"); ct != "" {
			meta["Content-Type"] = ct
		}
		for k, v := range r.Header {
			if strings.HasPrefix(k, "X-Amz-Meta-") {
				meta[strings.ToLower(k)] = v[0]
			}
		}
	}

	if err := h.backend.Copy(ctx, srcBucket, srcPrefixedKey, dstBucket, dstPrefixedKey, meta, replaceMetadata); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Copy failed: %v", err), 500)
		return
	}

	// Get the destination object info for ETag
	dstInfo, err := h.backend.Head(ctx, dstBucket, dstPrefixedKey)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to get destination info", 500)
		return
	}

	// Return success response
	result := CopyObjectResult{
		LastModified: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		ETag:         fmt.Sprintf(`"%s"`, dstInfo.ETag),
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// parseCopySource parses the x-amz-copy-source header value.
// Supports formats: /bucket/key or bucket/key
func parseCopySource(copySource string) (bucket, key string) {
	// Remove leading slash if present
	copySource = strings.TrimPrefix(copySource, "/")

	// URL decode the key portion (keys may contain special characters)
	if idx := strings.Index(copySource, "/"); idx != -1 {
		bucket = copySource[:idx]
		key = copySource[idx+1:]
		// URL decode the key
		if decoded, err := url.QueryUnescape(key); err == nil {
			key = decoded
		}
	}

	return bucket, key
}

// defaultListHeadFanout bounds the per-page HEAD fan-out when the handler was
// built from a Config literal that did not go through config.Load (tests).
const defaultListHeadFanout = 16

// enrichPlaintextSizes replaces encrypted on-disk sizes with plaintext sizes for
// ARMOR-encrypted objects. B2's ListObjectsV2 response includes the encrypted
// object size; the plaintext size is resolved from the in-memory manifest index
// first, then from the version-gated metadata cache, and only on a double miss
// from a bounded fan-out of backend HeadObject calls (ARMOR_LIST_HEAD_FANOUT,
// retried once on failure). A HEAD that still fails leaves the size unset (0)
// with a WARN — never the ciphertext size, which would silently corrupt
// clients that compute byte offsets from LIST sizes (DuckDB/httpfs).
//
// backend.List returns stored-space keys (ARMOR_PREFIX applied), so the
// manifest and cache lookups use the client-space key while the backend HEAD
// reuses the listed stored key as-is — applying the prefix again would 404
// every HEAD on a prefixed deployment. Each errgroup goroutine writes to a
// distinct slice index so no mutex is needed.
func (h *Handlers) enrichPlaintextSizes(ctx context.Context, bucket string, result *backend.ListResult) {
	fanout := defaultListHeadFanout
	if h.config != nil && h.config.ListHeadFanout >= 1 {
		fanout = h.config.ListHeadFanout
	}
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(fanout)
	for i := range result.Objects {
		i := i
		storedKey := result.Objects[i].Key
		clientKey := h.stripPrefix(storedKey)

		if h.manifest != nil {
			if entry, ok := h.manifest.Lookup(bucket, clientKey); ok && entry != nil {
				result.Objects[i].Size = entry.PlaintextSize
				if h.metrics != nil {
					h.metrics.IncListHead("index")
				}
				continue
			}
		}
		// The listed ETag is the stored generation this row describes; a
		// version-matched cache entry therefore describes the same object and
		// its plaintext size is authoritative. A miss after an overwrite is
		// the freshness rule working, not a negative cache.
		if am, ok := h.cache.GetVersioned(bucket, clientKey, result.Objects[i].ETag); ok && am != nil {
			result.Objects[i].Size = am.PlaintextSize
			if h.metrics != nil {
				h.metrics.IncListHead("cache")
			}
			continue
		}
		eg.Go(func() error {
			var info *backend.ObjectInfo
			var err error
			for attempt := 0; attempt < 2; attempt++ {
				info, err = h.backend.Head(egCtx, bucket, storedKey)
				if err == nil || egCtx.Err() != nil {
					break
				}
			}
			if err != nil {
				log.Printf("WARN LIST %s/%s: plaintext-size HEAD failed after retry; omitting size rather than reporting the ciphertext size: %v", bucket, storedKey, err)
				result.Objects[i].Size = 0
				if h.metrics != nil {
					h.metrics.IncListHead("error")
				}
				return nil
			}
			if info.IsARMOREncrypted {
				if am, ok := backend.ParseARMORMetadata(info.Metadata); ok && am != nil {
					result.Objects[i].Size = am.PlaintextSize
					h.cache.SetVersioned(bucket, clientKey, info.ETag, am)
				} else {
					// Encrypted but unparseable: the size cannot be resolved.
					log.Printf("WARN LIST %s/%s: ARMOR metadata unparseable; omitting size rather than reporting the ciphertext size", bucket, storedKey)
					result.Objects[i].Size = 0
					if h.metrics != nil {
						h.metrics.IncListHead("error")
					}
					return nil
				}
			}
			if h.metrics != nil {
				h.metrics.IncListHead("head")
			}
			return nil
		})
	}
	eg.Wait() //nolint:errcheck
}

// ListObjectsV2 handles S3 ListObjectsV2.
func (h *Handlers) ListObjectsV2(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	contToken := r.URL.Query().Get("continuation-token")
	maxKeys := 1000
	if mk := r.URL.Query().Get("max-keys"); mk != "" {
		if v, err := strconv.Atoi(mk); err == nil && v > 0 {
			maxKeys = v
		}
	}

	// Apply the configured prefix to the prefix parameter for backend operations.
	// When ARMOR_PREFIX is set, the backend stores all keys with the prefix prepended,
	// but clients don't know about it. We need to prepend the prefix to the client's
	// requested prefix so the backend finds the right objects.
	backendPrefix := h.applyPrefix(prefix)

	var result *backend.ListResult
	if h.listCache != nil {
		if cached, ok := h.listCache.Get(bucket, prefix, delimiter, maxKeys, contToken); ok {
			result = cached
		}
	}
	if result == nil {
		var err error
		result, err = h.backend.List(ctx, bucket, backendPrefix, delimiter, contToken, maxKeys)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list: %v", err), 500)
			return
		}
		// Resolve plaintext sizes for ARMOR-encrypted objects before caching.
		// B2 ListObjectsV2 returns encrypted (on-disk) sizes; clients like DuckDB
		// use these sizes to compute byte-range offsets, so they must reflect the
		// plaintext length.
		h.enrichPlaintextSizes(ctx, bucket, result)
		if h.listCache != nil {
			h.listCache.Set(bucket, prefix, delimiter, maxKeys, contToken, result)
		}
	}

	// Build XML response
	type Contents struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	}

	// S3 spec: each CommonPrefixes entry is a separate wrapper element with one
	// Prefix child. Using []string with "Outer>Inner" produces an empty
	// <CommonPrefixes/> when the slice is nil, which the AWS SDK v1 parses as a
	// slice with one nil element — causing nil-pointer crashes in clients like
	// litestream. Using a dedicated struct + omitempty avoids this.
	type CommonPrefix struct {
		Prefix string `xml:"Prefix"`
	}

	type ListBucketResult struct {
		XMLName               xml.Name `xml:"ListBucketResult"`
		Xmlns                 string   `xml:"xmlns,attr"`
		Name                  string   `xml:"Name"`
		Prefix                string   `xml:"Prefix"`
		Delimiter             string   `xml:"Delimiter,omitempty"`
		MaxKeys               int      `xml:"MaxKeys"`
		IsTruncated           bool     `xml:"IsTruncated"`
		NextContinuationToken string   `xml:"NextContinuationToken,omitempty"`
		Contents              []Contents
		CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
	}

	resp := ListBucketResult{
		Xmlns:                 "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:                  h.namedBucket(r),
		Prefix:                prefix,
		Delimiter:             delimiter,
		MaxKeys:               maxKeys,
		IsTruncated:           result.IsTruncated,
		NextContinuationToken: result.NextToken,
	}

	for _, obj := range result.Objects {
		// Strip the configured prefix from object keys before returning to client.
		// Clients don't know about the prefix, so we need to remove it from the keys.
		strippedKey := h.stripPrefix(obj.Key)
		resp.Contents = append(resp.Contents, Contents{
			Key:          strippedKey,
			LastModified: obj.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
			ETag:         fmt.Sprintf(`"%s"`, obj.ETag),
			Size:         obj.Size,
			StorageClass: "STANDARD",
		})
	}

	// Sort common prefixes lexicographically per S3 spec
	sort.Strings(result.CommonPrefixes)

	for _, p := range result.CommonPrefixes {
		// Strip the configured prefix from common prefixes before returning to client.
		// Common prefixes are used for directory-like listings with delimiters.
		strippedPrefix := h.stripPrefixFromCommonPrefix(p)
		resp.CommonPrefixes = append(resp.CommonPrefixes, CommonPrefix{Prefix: strippedPrefix})
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// Stub implementations for other operations

// HeadBucket handles S3 HeadBucket.
func (h *Handlers) HeadBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.HeadBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "NoSuchBucket", fmt.Sprintf("Bucket not found: %v", err), 404)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketLocation handles S3 GetBucketLocation (GET ?location).
// Returns a static LocationConstraint so MinIO/Forgejo clients can determine the region
// without requiring region-aware routing.
func (h *Handlers) GetBucketLocation(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.HeadBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "NoSuchBucket", fmt.Sprintf("Bucket not found: %v", err), 404)
		return
	}

	type locationConstraintResponse struct {
		XMLName  xml.Name `xml:"LocationConstraint"`
		XMLNS    string   `xml:"xmlns,attr"`
		Location string   `xml:",chardata"`
	}

	resp := locationConstraintResponse{
		XMLNS:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: "",
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, xml.Header); err != nil {
		return
	}
	_ = xml.NewEncoder(w).Encode(resp)
}

// GetBucketVersioning handles S3 GetBucketVersioning (GET ?versioning).
// Returns an empty VersioningConfiguration (versioning is not enabled on ARMOR buckets).
func (h *Handlers) GetBucketVersioning(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.HeadBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "NoSuchBucket", fmt.Sprintf("Bucket not found: %v", err), 404)
		return
	}

	type versioningConfigurationResponse struct {
		XMLName xml.Name `xml:"VersioningConfiguration"`
		XMLNS   string   `xml:"xmlns,attr"`
	}

	resp := versioningConfigurationResponse{
		XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/",
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, xml.Header); err != nil {
		return
	}
	_ = xml.NewEncoder(w).Encode(resp)
}

// CreateBucket handles S3 CreateBucket.
func (h *Handlers) CreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.CreateBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create bucket: %v", err), 500)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// DeleteBucket handles S3 DeleteBucket.
func (h *Handlers) DeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.DeleteBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to delete bucket: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteObjects handles S3 DeleteObjects (bulk delete).
// The request body contains XML with a list of objects to delete.
// Per-key ACL enforcement is performed for each key in the request body.
func (h *Handlers) DeleteObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	// Retrieve credential from context for per-key ACL checks.
	// The credential is stored in wrapHandler after successful authentication.
	// For public endpoints or when auth is disabled, cred may be nil.
	cred := acl.CredentialFromContext(ctx)

	// Parse the DeleteObjects request XML
	type Object struct {
		Key string `xml:"Key"`
	}

	type DeleteRequest struct {
		XMLName xml.Name `xml:"Delete"`
		Objects []Object `xml:"Object"`
		Quiet   bool     `xml:"Quiet"`
	}

	var deleteReq DeleteRequest

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := xml.Unmarshal(body, &deleteReq); err != nil {
		h.writeError(w, r, "MalformedXML", fmt.Sprintf("Failed to parse XML: %v", err), 400)
		return
	}

	if len(deleteReq.Objects) == 0 {
		h.writeError(w, r, "MalformedXML", "No objects specified for deletion", 400)
		return
	}

	// Extract original keys from request and prepare prefixed keys for backend
	originalKeys := make([]string, len(deleteReq.Objects))
	for i, obj := range deleteReq.Objects {
		originalKeys[i] = obj.Key
	}

	// Perform per-key ACL enforcement.
	// CheckACL requires bucket, key, and verb. For DeleteObjects, the verb is "delete".
	// If credential is nil (no auth), allow all deletions (backward compatibility).
	var allowedKeys []string
	var deniedKeys []string

	if cred == nil {
		// No authentication - allow all deletions (backward compatible)
		allowedKeys = originalKeys
	} else {
		// Check ACL for each key individually
		for _, key := range originalKeys {
			// Import CheckACL from server package
			if err := acl.CheckACL(cred, bucket, key, "delete"); err != nil {
				deniedKeys = append(deniedKeys, key)
			} else {
				allowedKeys = append(allowedKeys, key)
			}
		}
	}

	// Prepare prefixed keys for backend operations
	prefixedKeys := make([]string, len(allowedKeys))
	for i, key := range allowedKeys {
		prefixedKeys[i] = h.applyPrefix(key)
	}

	// Perform bulk delete with prefixed keys (only allowed keys)
	if len(allowedKeys) > 0 {
		if err := h.backend.DeleteObjects(ctx, bucket, prefixedKeys); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("DeleteObjects failed: %v", err), 500)
			return
		}
	}

	// Remove from manifest using original (unprefixed) allowed keys
	if h.manifest != nil {
		for _, key := range allowedKeys {
			h.manifest.RecordDelete(bucket, key)
		}
	}

	// Invalidate cache for deleted objects using original (unprefixed) allowed keys
	for _, key := range allowedKeys {
		h.cache.Delete(bucket, key)
		h.footerCache.Delete(bucket, key)
	}

	// Denied keys must not be tombstoned: RecordDelete drops the manifest
	// entry and persists a "del" delta, so sweeping originalKeys here would
	// hide still-existing objects from manifest lookups even though the ACL
	// refused to delete them. Only allowedKeys were removed, above.

	// Build response XML
	type DeletedObject struct {
		Key string `xml:"Key"`
	}

	type DeleteResult struct {
		XMLName xml.Name        `xml:"DeleteResult"`
		Xmlns   string          `xml:"xmlns,attr"`
		Deleted []DeletedObject `xml:"Deleted"`
		Error   []struct {
			Key     string `xml:"Key"`
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		} `xml:"Error"`
	}

	result := DeleteResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
	}

	// Per-key errors are always reported: S3 quiet mode omits only the
	// successful Deleted entries, so suppressing AccessDenied here would make
	// a restricted credential's failed batch look fully successful.
	for _, key := range deniedKeys {
		result.Error = append(result.Error, struct {
			Key     string `xml:"Key"`
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		}{
			Key:     key,
			Code:    "AccessDenied",
			Message: "Access Denied",
		})
	}

	// Quiet mode omits the successfully deleted keys.
	if !deleteReq.Quiet {
		for _, key := range allowedKeys {
			result.Deleted = append(result.Deleted, DeletedObject{Key: key})
		}
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// ListBuckets handles S3 ListBuckets.
func (h *Handlers) ListBuckets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	buckets, err := h.backend.ListBuckets(ctx)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list buckets: %v", err), 500)
		return
	}

	// Build XML response
	type Bucket struct {
		Name         string `xml:"Name"`
		CreationDate string `xml:"CreationDate"`
	}

	type ListAllMyBucketsResult struct {
		XMLName xml.Name `xml:"ListAllMyBucketsResult"`
		Xmlns   string   `xml:"xmlns,attr"`
		Owner   struct {
			ID          string `xml:"ID"`
			DisplayName string `xml:"DisplayName"`
		} `xml:"Owner"`
		Buckets struct {
			Bucket []Bucket `xml:"Bucket"`
		} `xml:"Buckets"`
	}

	result := ListAllMyBucketsResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
	}
	result.Owner.ID = "armor"
	result.Owner.DisplayName = "ARMOR"

	for _, b := range buckets {
		result.Buckets.Bucket = append(result.Buckets.Bucket, Bucket{
			Name:         b.Name,
			CreationDate: b.CreationDate.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// CreateMultipartUpload handles S3 CreateMultipartUpload with ARMOR encryption.
// It generates a DEK and IV, wraps the DEK, and stores the state in B2.
func (h *Handlers) CreateMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Reject multipart uploads when compression is enabled (ADR-007)
	// Check both legacy Compress flag and CompressRules
	if h.config.Compress || h.config.CompressRules.HasRules() {
		h.writeError(w, r, "InvalidArgument", "Compression is not supported for multipart uploads. Use single-PUT uploads for compressed objects or disable compression (ARMOR_COMPRESS=false or ARMOR_COMPRESS_RULES empty).", 400)
		return
	}

	// Get the appropriate MEK for this object key
	mek, keyID, err := h.keyManager.GetMEK(key)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get encryption key: %v", err), 500)
		return
	}

	// Generate DEK and IV for this upload
	dek, err := crypto.GenerateDEK()
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to generate DEK: %v", err), 500)
		return
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to generate IV: %v", err), 500)
		return
	}

	// Wrap DEK with MEK and encode fingerprint in v2 format
	wrappedDEKStr, err := crypto.WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to wrap DEK: %v", err), 500)
		return
	}

	// Parse v2 format to extract fingerprint and wrapped DEK bytes
	// Format: v2:<fp16>:<base64>
	parts := strings.SplitN(wrappedDEKStr, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" {
		h.writeError(w, r, "InternalError", "wrapped DEK format validation failed", 500)
		return
	}
	mekFingerprint := parts[1]
	wrappedDEK, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode wrapped DEK: %v", err), 500)
		return
	}

	// Get content type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Initiate multipart upload with B2 (no ARMOR metadata yet - that comes on completion).
	//
	// ARMOR_PREFIX must be applied here, exactly as every single-object handler
	// does (ADR-001: the proxy prepends the prefix on the way to storage and
	// strips it from responses, so it stays transparent to clients). This whole
	// path used to pass the RAW key while the read path applied the prefix, so on
	// any deployment with a non-empty prefix every multipart object was written
	// to an unprefixed key and was then unreachable: reads 404'd against
	// <prefix>/<key> while the bytes sat at <key>. Confirmed live on
	// ord-devimprint, where a Postgres base backup's data.tar (33.4MB) landed at
	// postgres/cnpg/... while its single-PUT backup.info correctly landed at
	// commitgraph/postgres/cnpg/..., making the backup unrestorable.
	uploadID, err := h.backend.CreateMultipartUpload(ctx, bucket, h.applyPrefix(key), nil)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create multipart upload: %v", err), 500)
		return
	}

	// Save multipart state to B2
	// The format version is fixed at CreateMultipartUpload time based on the server's
	// ARMOR_FORMAT_VERSION configuration and does not change during the upload's lifetime.
	formatVersion := h.config.FormatWriteVersion
	manager := h.getMultipartManager(bucket)

	if formatVersion == 3 {
		// V3 format: save metadata to .armor/multipart/<id>/meta.json
		// No per-part state yet — that's written by each UploadPart
		metadata := &backend.MultipartMetadataV3{
			UploadID:       uploadID,
			Bucket:         bucket,
			Key:            key,
			IV:             iv,
			WrappedDEK:     wrappedDEK,
			MEKFingerprint: mekFingerprint,
			BlockSize:      h.config.BlockSize,
			Created:        time.Now(),
			ContentType:    contentType,
			KeyID:          keyID,
			Metadata:       userMetadata(r),
			FormatVersion:  3,
		}
		if err := manager.SaveMetadataV3(ctx, metadata); err != nil {
			// Try to abort the upload on metadata save failure
			h.backend.AbortMultipartUpload(ctx, bucket, key, uploadID)
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to save multipart metadata: %v", err), 500)
			return
		}
	} else {
		// V2 format: save state to .armor/multipart/<id>.state (legacy)
		state := &backend.MultipartState{
			UploadID:       uploadID,
			Bucket:         bucket,
			Key:            key,
			IV:             iv,
			WrappedDEK:     wrappedDEK,
			MEKFingerprint: mekFingerprint,
			BlockSize:      h.config.BlockSize,
			Created:        time.Now(),
			ContentType:    contentType,
			KeyID:          keyID,
			Metadata:       userMetadata(r),
			PartHMACs:      make(map[int]string),
			PartSizes:      make(map[int]int64),
			FormatVersion:  2,
		}
		if err := manager.SaveState(ctx, state); err != nil {
			// Try to abort the upload on state save failure
			h.backend.AbortMultipartUpload(ctx, bucket, key, uploadID)
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to save multipart state: %v", err), 500)
			return
		}
	}

	// Build XML response
	type InitiateMultipartUploadResult struct {
		XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
		Xmlns    string   `xml:"xmlns,attr"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		UploadID string   `xml:"UploadId"`
	}

	result := InitiateMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   h.namedBucket(r),
		Key:      key,
		UploadID: uploadID,
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// UploadPart handles S3 UploadPart with encryption.
//
// Format version 3 (the default) encrypts each part independently using the
// (part number, block) counter: part N, block B uses counter (N<<16 | B).
// This imposes no ordering constraints — parts can be uploaded in any order,
// with any size (subject only to S3's 5 MiB minimum part size), and with any
// concurrency. No part-1-first requirement, no uniform part size, no block
// alignment beyond each part's own internal block structure.
//
// V2 multipart objects remain readable: the NonUniformParts read path handles
// both the original sequential-only layout (ADR-003) and the ADR-011
// cumulative-offset layout.
//
// Legacy v2 per-upload state updates are serialized by multipartLock: the state
// object is read-modify-written here, so without per-upload serialization a
// concurrent writer would drop an earlier part's HMAC/size entry. V3 uses an
// independent part-<n>.json object per part, so distinct part numbers must not
// share this lock. Serializing them leaves concurrent clients' request bodies
// unread while a slow B2 UploadPart runs, eventually resetting those idle
// connections. Duplicate requests for the same v3 part remain serialized.
func (h *Handlers) UploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Parse part number
	partNumberStr := r.URL.Query().Get("partNumber")
	if partNumberStr == "" {
		h.writeError(w, r, "InvalidRequest", "Missing part number", 400)
		return
	}
	partNumber, err := strconv.ParseInt(partNumberStr, 10, 32)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		h.writeError(w, r, "InvalidRequest", "Invalid part number", 400)
		return
	}

	// Load multipart state/metadata
	// Try v3 format first (meta.json), fall back to v2 format (.state file)
	manager := h.getMultipartManager(bucket)

	// Try v3 format
	metadata, errV3 := manager.LoadMetadataV3(ctx, uploadID)
	var state *backend.MultipartState
	var formatVersion int

	if errV3 == nil && metadata != nil {
		// V3 format loaded successfully
		formatVersion = 3
		// V3 part metadata is independent, so only serialize an idempotent retry
		// racing the same part number. Different parts must reach B2 concurrently.
		defer h.multipartLock(fmt.Sprintf("%s/part/%d", uploadID, partNumber))()
		// Convert v3 metadata to a minimal state structure for the existing logic
		// The state is used read-only for most of the function
		state = &backend.MultipartState{
			UploadID:          metadata.UploadID,
			Bucket:            metadata.Bucket,
			Key:               metadata.Key,
			IV:                metadata.IV,
			WrappedDEK:        metadata.WrappedDEK,
			MEKFingerprint:    metadata.MEKFingerprint,
			BlockSize:         metadata.BlockSize,
			ContentType:       metadata.ContentType,
			KeyID:             metadata.KeyID,
			Metadata:          metadata.Metadata,
			PartSize:          metadata.PartSize,
			NonUniformParts:   metadata.NonUniformParts,
			Poisoned:          metadata.Poisoned,
			PoisonReason:      metadata.PoisonReason,
			FormatVersion:     3,
			PartSizes:         make(map[int]int64),  // Will be loaded from part files as needed
			PartHMACs:         make(map[int]string), // Will be loaded from part files as needed
			PartPlaintextSHAs: make(map[int]string), // Will be loaded from part files as needed
		}
	} else {
		// Fall back to v2 format
		// V2 has one shared read-modify-write state object, so the lock must cover
		// the state load, backend upload, and state save.
		defer h.multipartLock(uploadID)()
		state, err = manager.LoadState(ctx, uploadID)
		if err != nil {
			h.writeError(w, r, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
			return
		}
		formatVersion = state.FormatVersion
		if formatVersion == 0 {
			formatVersion = 2 // Old state files without format version are v2
		}
	}

	// Verify bucket and key match
	if state.Bucket != bucket || state.Key != key {
		h.writeError(w, r, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
		return
	}

	// If a prior contradiction poisoned this upload (ADR-015 rule 4), every
	// further part fails the same way — the client must abort and retry.
	if state.Poisoned {
		h.writeError(w, r, "InvalidPart",
			fmt.Sprintf("This multipart upload has been invalidated and cannot be completed: %s. Abort and retry the upload from the beginning.", state.PoisonReason), 400)
		return
	}

	// ADR-015 (amended 2026-07-19): P is pinned ONLY from part number 1, for
	// formatVersion != 3 uploads (v3 imposes no ordering/size contract — see the
	// EncryptPartV3 branch below). A part numbered >1 that arrives before part 1
	// has pinned P cannot have its CTR offset computed and must not be allowed to
	// pin P — under default aws-cli concurrency the short final part (fewest
	// bytes) finishes FIRST and used to pin P too small, so the first full-size
	// part contradicted it and every default-concurrency upload failed. Defer
	// such a part with S3's retryable 503 SlowDown and store NOTHING: no state
	// mutation, no backend upload, no PartSizes entry. Standard clients (aws
	// cli, SDKs, rclone, litestream) retry SlowDown transparently with backoff;
	// part 1 arrives and pins P, and the deferred part then succeeds at its
	// correct (N-1)*P/BlockSize offset. The body is left unread so a deferred
	// (potentially large) part costs no server memory.
	if formatVersion != 3 && state.PartSize == 0 && partNumber != 1 {
		w.Header().Set("Retry-After", "1")
		h.writeError(w, r, "SlowDown",
			"Part 1 for this multipart upload has not been received yet, so the uniform part size is not established. Retry this part after uploading part 1.",
			http.StatusServiceUnavailable)
		return
	}

	// V3 reads the body once into a bounded, seekable plaintext source: a
	// pooled Content-Length-sized buffer admitted against the server's
	// aggregate part memory budget, or a temp-file spool for unknown or
	// over-budget lengths. The replayable part reader then encrypts lazily
	// under the backend's read, so encryption overlaps the backend transfer
	// and a backend retry replays byte-identical ciphertext (armor-cb2b0731).
	// V2 keeps the buffered read: its offset-based encryption consumes the
	// whole plaintext at once.
	var plaintext []byte
	var partSource *uploadPartSource
	var releasePartBody func()
	var partSHA [sha256.Size]byte
	var plaintextSize int64
	if formatVersion == 3 {
		partSource, err = readUploadPartBodySeekable(ctx, r.Body, r.ContentLength, h.partMemoryBudget())
		if err != nil {
			h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
			return
		}
		defer partSource.release()
		plaintextSize = partSource.size
		partSHA = partSource.digest
	} else {
		plaintext, partSHA, releasePartBody, err = readUploadPartBody(r.Body, r.ContentLength)
		if err != nil {
			h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
			return
		}
		defer func() {
			if releasePartBody != nil {
				releasePartBody()
			}
		}()
		plaintextSize = int64(len(plaintext))
	}

	// ADR-011: Detect non-uniform part pattern (e.g., Barman's chunk_size + N*512).
	// Only applies to formatVersion != 3 — v3 parts are independently encrypted
	// and never need offset-based decryption.
	if formatVersion != 3 {
		shouldEnableNonUniform := false
		if partNumber == 1 && plaintextSize%int64(state.BlockSize) != 0 {
			// Part 1 is non-block-aligned - enable non-uniform mode
			shouldEnableNonUniform = true
		} else if partNumber > 1 && state.PartSize > 0 && plaintextSize != state.PartSize && plaintextSize%int64(state.BlockSize) != 0 {
			// Subsequent part differs from P and is non-block-aligned - enable non-uniform mode
			shouldEnableNonUniform = true
		}

		if shouldEnableNonUniform && !state.NonUniformParts {
			state.NonUniformParts = true
			state.CumulativePartSizes = make(map[int]int64)
			// Initialize from existing parts
			cumulative := int64(0)
			for i := 1; i < int(partNumber); i++ {
				if sz, exists := state.PartSizes[i]; exists {
					state.CumulativePartSizes[i] = cumulative
					cumulative += sz
				}
			}
		}

		// For Part 1 in non-uniform mode, set its offset to 0
		if partNumber == 1 && state.NonUniformParts && state.CumulativePartSizes != nil {
			if _, exists := state.CumulativePartSizes[1]; !exists {
				state.CumulativePartSizes[1] = 0
			}
		}

		// ADR-015 rule 1 / U8: block alignment is required only where it does work —
		// of a part that another part is placed after. The offset formula
		// (N-1)*P/BlockSize needs P on a block boundary, and a non-aligned regular
		// part would misalign every subsequent part's HMAC index.
		//
		// Exempt:
		//   - pinningP: part 1, which always starts at block 0 whatever its size.
		//   - presumedFinal: a part smaller than an already-pinned P. Nothing is
		//     placed after it; its partial trailing block is ordinary.
		// (0 bytes is trivially aligned but is rejected below — it cannot pin P.)
		pinningP := state.PartSize == 0
		presumedFinal := state.PartSize != 0 && plaintextSize < state.PartSize
		if plaintextSize > 0 && !pinningP && !presumedFinal && !state.NonUniformParts && plaintextSize%int64(state.BlockSize) != 0 {
			h.writeError(w, r, "InvalidPartSize",
				fmt.Sprintf("Part size %d is not a multiple of the block size (%d bytes). ARMOR's uniform-part-size contract (ADR-015) requires block-aligned parts. Use a part size that's a multiple of %d (e.g., 5,242,880 for 5MiB, 16,777,216 for 16MiB).", plaintextSize, state.BlockSize, state.BlockSize), 400)
			return
		}
	}

	// Idempotent retry of an already-uploaded part number (ADR-015 rule 5).
	if existingSize, exists := state.PartSizes[int(partNumber)]; exists {
		if existingSize != plaintextSize {
			// A retry with a different size contradicts the contract — poison.
			reason := fmt.Sprintf("part %d was already uploaded with size %d but re-uploaded with size %d", partNumber, existingSize, plaintextSize)
			h.poisonUpload(ctx, manager, state, reason, formatVersion)
			h.writeError(w, r, "InvalidPart",
				fmt.Sprintf("Part %d was already uploaded with size %d but re-uploaded with size %d. %s", partNumber, existingSize, plaintextSize, multipartRetryMessage), 400)
			return
		}
		// Same-size retry: idempotent. Re-encrypt at the same (N-1)*P offset —
		// CTR is deterministic, so the ciphertext is identical — and re-upload
		// (overwrite). Skip the contract checks below; this part already passed
		// them when first uploaded. Fall through to the shared encrypt/upload path.
	}

	// P is the pinned uniform part size for formatVersion != 3 uploads. ADR-015
	// (amended 2026-07-19): pinned ONLY from part number 1 — the SlowDown guard
	// above already returned any part >1 that arrived before part 1, so reaching
	// here with P==0 implies partNumber == 1. Part 1 of a well-formed multipart
	// upload is always a full-size part (the short final is by definition the
	// highest-numbered part), so pinning P from part 1 makes the uniform size
	// correct by construction.
	var P int64
	if formatVersion != 3 {
		P = state.PartSize
		if P == 0 {
			if plaintextSize == 0 {
				h.writeError(w, r, "InvalidPartSize", "Cannot establish the uniform part size from an empty first part. Upload a non-empty first part.", 400)
				return
			}
			P = plaintextSize
			state.PartSize = P
		}

		// Contradiction detection against the pinned P (ADR-015 rule 4, defense-in-
		// depth). Skip for same-size retries and for non-uniform uploads.
		if _, exists := state.PartSizes[int(partNumber)]; !exists && !state.NonUniformParts {
			switch {
			case plaintextSize > P:
				reason := fmt.Sprintf("part %d size %d exceeds the uniform part size %d pinned by part 1", partNumber, plaintextSize, P)
				h.poisonUpload(ctx, manager, state, reason, formatVersion)
				h.writeError(w, r, "InvalidPartSize",
					fmt.Sprintf("Part %d size %d is larger than the uniform part size %d established for this upload by part 1. %s", partNumber, plaintextSize, P, multipartRetryMessage), 400)
				return
			case plaintextSize < P:
				// Presumed final part. At most one part may differ from P (and it must
				// be the highest-numbered); a second short part is a contradiction.
				for _, sz := range state.PartSizes {
					if sz < P {
						reason := fmt.Sprintf("multiple short (presumed-final) parts seen (sizes %d and %d); the uniform-part-size contract allows at most one short final part of size %d", sz, plaintextSize, P)
						h.poisonUpload(ctx, manager, state, reason, formatVersion)
						h.writeError(w, r, "InvalidPartSize",
							fmt.Sprintf("Part %d size %d contradicts an earlier short final part; only one short final part is allowed. %s", partNumber, plaintextSize, multipartRetryMessage), 400)
						return
					}
				}
			}
		}
	}

	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		return h.keyManager.GetMEKByFingerprint(keyID, fingerprint)
	}

	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		mek, err := h.keyManager.GetMEKByID(state.KeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to get active key: %w", err)
		}
		dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
		if err == nil {
			return dek, nil
		}

		ring := h.keyManager.Ring(state.KeyID)
		for _, ringEntry := range ring {
			dek, err := crypto.UnwrapDEK(ringEntry.MEK, wrappedDEK)
			if err == nil {
				return dek, nil
			}
		}

		return nil, fmt.Errorf("no key in active or ring can unwrap DEK")
	}

	wrappedDEKStr := base64.StdEncoding.EncodeToString(state.WrappedDEK)
	if state.MEKFingerprint != "" {
		wrappedDEKStr = fmt.Sprintf("v2:%s:%s", state.MEKFingerprint, wrappedDEKStr)
	}

	dek, _, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to unwrap DEK: %v", err), 500)
		return
	}

	var encrypted []byte
	var blockHMACsRaw []byte
	var blockHMACs [][]byte
	var etag string
	var partCiphertextLen int64

	if formatVersion == 3 {
		// V3 encrypts each part as an independent stream starting from block 0
		// with the (part, block) counter — no ordering constraints, no size
		// pinning, no cumulative offsets. The replayable reader encrypts each
		// block as the backend consumes it, recording its HMAC, so encryption
		// overlaps the backend transfer and a backend retry rewinds and
		// replays byte-identical ciphertext from the same plaintext source.
		partReader, perr := crypto.NewReplayablePartReader(partSource.reader, dek, state.IV, state.BlockSize, plaintextSize, uint16(partNumber))
		if perr != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encrypt part with v3: %v", perr), 500)
			return
		}
		partCiphertextLen = partReader.Len()
		var uerr error
		etag, uerr = h.backend.UploadPart(ctx, bucket, h.applyPrefix(key), uploadID, int32(partNumber), partReader, partCiphertextLen)
		if uerr != nil {
			h.writeBackendError(w, r, "InternalError", fmt.Sprintf("Failed to upload part: %v", uerr), 500, uerr)
			return
		}
		if pos, seekErr := partReader.Seek(0, io.SeekCurrent); seekErr != nil || pos != partCiphertextLen {
			h.writeError(w, r, "InternalError", "Backend did not consume the complete encrypted part", 500)
			return
		}
		if cerr := ctx.Err(); cerr != nil {
			h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Upload canceled: %v", cerr), 500, cerr)
			return
		}
		// Collect the per-block HMACs recorded during the transfer, backfilling
		// from the plaintext source any block the backend never reached. The
		// source stays alive until the handler returns, so this re-read is safe.
		blockHMACs, perr = partReader.PartBlockHMACs()
		if perr != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to collect part block HMACs: %v", perr), 500)
			return
		}
	} else if state.NonUniformParts {
		// ADR-011: Use cumulative offsets for non-uniform parts (e.g., Barman's chunk_size + N*512)
		// Calculate cumulative offset for this part
		cumulativeOffset := int64(0)
		for i := 1; i < int(partNumber); i++ {
			if sz, exists := state.PartSizes[i]; exists {
				cumulativeOffset += sz
			} else {
				// A previous part is missing - defer with SlowDown
				w.Header().Set("Retry-After", "1")
				h.writeError(w, r, "SlowDown",
					fmt.Sprintf("Part %d has not been received yet, so the cumulative offset for part %d cannot be computed. Retry this part after uploading part %d.", i, partNumber, i),
					http.StatusServiceUnavailable)
				return
			}
		}

		// Store cumulative offset for this part
		if state.CumulativePartSizes == nil {
			state.CumulativePartSizes = make(map[int]int64)
		}
		state.CumulativePartSizes[int(partNumber)] = cumulativeOffset

		// Create offset encryptor
		offsetEncryptor, err := crypto.NewOffsetEncryptor(dek, state.IV, state.BlockSize, crypto.Version2)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create offset encryptor: %v", err), 500)
			return
		}

		// Encrypt with cumulative byte offset
		encrypted, blockHMACsRaw, err = offsetEncryptor.EncryptFromOffset(plaintext, cumulativeOffset)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encrypt with offset: %v", err), 500)
			return
		}
	} else {
		// Legacy uniform part size mode (ADR-015): CTR starting block is a
		// function of part NUMBER and the uniform part size P alone (rule 2):
		// part N starts at block (N-1)*P/BlockSize. Because P is block-aligned,
		// this is an exact block boundary regardless of arrival order.
		startBlockIndex := uint32(((partNumber - 1) * P) / int64(state.BlockSize))

		encryptor, err := crypto.NewEncryptor(dek, state.IV, state.BlockSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create encryptor: %v", err), 500)
			return
		}

		// Encrypt the part with the correct starting counter. CTR is deterministic,
		// so an idempotent retry (same N, same size) produces byte-identical ciphertext.
		encrypted, blockHMACsRaw, err = encryptor.EncryptWithStartingCounter(plaintext, startBlockIndex)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encrypt: %v", err), 500)
			return
		}
	}

	// In v2, the ciphertext is separate, so the plaintext buffer can be
	// returned before waiting for the backend. V3's plaintext source stays
	// alive until the handler returns — the part reader and the HMAC
	// backfill read from it.
	if formatVersion != 3 {
		releasePartBody()
		releasePartBody = nil
	}
	plaintext = nil

	// Upload encrypted part to B2 (v2; v3 uploaded from its replayable
	// reader above). Prefixed for the same reason as CreateMultipartUpload
	// above — the part must be addressed by the same storage key the upload
	// was created under.
	if formatVersion != 3 {
		etag, err = h.backend.UploadPart(ctx, bucket, h.applyPrefix(key), uploadID, int32(partNumber), bytes.NewReader(encrypted), int64(len(encrypted)))
		if err != nil {
			h.writeBackendError(w, r, "InternalError", fmt.Sprintf("Failed to upload part: %v", err), 500, err)
			return
		}

		// Split concatenated HMACs into individual HMACs for storage.
		// EncryptWithStartingCounter returns HMACs as a flat byte slice.
		blockCount := len(blockHMACsRaw) / crypto.HMACSize
		blockHMACs = make([][]byte, blockCount)
		for i := 0; i < blockCount; i++ {
			blockHMACs[i] = blockHMACsRaw[i*crypto.HMACSize : (i+1)*crypto.HMACSize]
		}
	}

	// The plaintext SHA-256 was computed while reading, before in-place v3
	// encryption changed the input buffer.
	partSHAHex := hex.EncodeToString(partSHA[:])

	if formatVersion == 3 {
		// V3 format: save part data to a separate part-<n>.json file
		// This allows concurrent UploadPart operations to proceed without touching the same file
		partData := &backend.PartDataV3{
			PartNumber:       int(partNumber),
			PlaintextLen:     plaintextSize,
			CiphertextLen:    partCiphertextLen,
			BlockHMACsBase64: backend.EncodeHMACToBase64(blockHMACs),
			PlaintextSHAHex:  partSHAHex,
		}

		if err := manager.SavePartV3(ctx, uploadID, partData); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to save part data: %v", err), 500)
			return
		}

		// V3 format: no uniform size pinning, no part-1-first requirement
		// Parts can be uploaded in any order with any size (subject only to S3 limits)
	} else {
		// V2 format: update the shared state object (legacy)
		state.PartHMACs[int(partNumber)] = backend.EncodeHMACToBase64(blockHMACs)
		state.PartSizes[int(partNumber)] = plaintextSize
		if state.PartPlaintextSHAs == nil {
			state.PartPlaintextSHAs = make(map[int]string)
		}
		state.PartPlaintextSHAs[int(partNumber)] = partSHAHex

		if err := manager.SaveState(ctx, state); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to update multipart state: %v", err), 500)
			return
		}
	}

	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

// multipartRetryMessage is the user-facing instruction appended to every
// ADR-015 rule-4 contradiction error. It tells the client the upload is dead
// and must be retried — the invariant that no corrupt object is ever stored.
const multipartRetryMessage = "This upload has been invalidated; abort it and retry the multipart upload from the beginning."

// multipartMinPartSize is B2's minimum part size for multi-part objects
// (ADR-015 rule 1). Enforced at Complete for any upload with more than one
// part.
const multipartMinPartSize = int64(5 * 1024 * 1024)

// poisonUpload marks the multipart state as permanently failed (ADR-015 rule 4)
// and persists that state so the failure survives to CompleteMultipartUpload.
// A best-effort save: if it fails we still return the 400 to the client, and
// the contradiction is re-caught at Complete by the uniformity validation.
func (h *Handlers) poisonUpload(ctx context.Context, manager *backend.MultipartStateManager, state *backend.MultipartState, reason string, formatVersion int) {
	state.Poisoned = true
	state.PoisonReason = reason

	if formatVersion == 3 {
		// V3 format: update metadata instead of state
		metadata, err := manager.LoadMetadataV3(ctx, state.UploadID)
		if err == nil {
			metadata.Poisoned = true
			metadata.PoisonReason = reason
			_ = manager.SaveMetadataV3(ctx, metadata)
		}
	} else {
		// V2 format: save state
		_ = manager.SaveState(ctx, state)
	}
}

// writeManifest writes the manifest object for a completed multipart upload.
// The manifest contains all ARMOR metadata and references the ciphertext object.
// This is used instead of CopyObject to avoid the 5GB limit and race conditions.
func (h *Handlers) writeManifest(ctx context.Context, bucket, key string, meta map[string]string, uploadID, etag string, ciphertextRef string) error {
	// Build manifest key: <prefixed-key>.armor-manifest
	manifestKey := h.manifestKeyFor(key)

	// Build manifest body (optional, for debugging)
	manifestBody := &backend.ManifestBody{
		CiphertextObject: ciphertextRef,
		UploadID:         uploadID,
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		Metadata:         meta,
	}

	manifestJSON, err := json.Marshal(manifestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest body: %w", err)
	}

	// Add ciphertext reference to metadata if not already present
	if meta[metaCiphertextRef] == "" {
		meta[metaCiphertextRef] = ciphertextRef
	}

	// Add completion timestamp to metadata
	if meta[metaCompletedAt] == "" {
		meta[metaCompletedAt] = manifestBody.CompletedAt
	}

	// Write manifest object with metadata
	manifestMeta := map[string]string{
		"Content-Type": manifestContentType,
	}
	for k, v := range meta {
		manifestMeta[k] = v
	}

	err = h.backend.Put(ctx, bucket, manifestKey, bytes.NewReader(manifestJSON), int64(len(manifestJSON)), manifestMeta)
	if err != nil {
		return fmt.Errorf("manifest write failed: %w", err)
	}

	return nil
}

// readManifest reads the manifest object for a key.
// Returns (manifestBody, metadata, error).
func (h *Handlers) readManifest(ctx context.Context, bucket, key string) (*backend.ManifestBody, map[string]string, error) {
	// Try to read manifest: <prefixed-key>.armor-manifest
	manifestKey := h.manifestKeyFor(key)

	body, info, err := h.backend.Get(ctx, bucket, manifestKey)
	if err != nil {
		// Manifest not found is not an error - caller will fall back to legacy
		return nil, nil, err
	}
	defer body.Close()

	// Read manifest body
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read manifest body: %w", err)
	}

	// Parse manifest JSON
	var manifestBody backend.ManifestBody
	if err := json.Unmarshal(bodyBytes, &manifestBody); err != nil {
		// If JSON parsing fails, we still have metadata from headers
		// Create a minimal manifest body
		manifestBody = backend.ManifestBody{
			CiphertextObject: info.Metadata["x-amz-meta-armor-ciphertext-ref"],
			Metadata:         info.Metadata,
		}
	}

	return &manifestBody, info.Metadata, nil
}

// verifyCiphertextFreshness checks that the ciphertext object referenced by the
// manifest is still the one the manifest was written for (not overwritten by a
// newer same-key upload).
//
// CompleteMultipartUpload assembles the ciphertext object first and writes the
// manifest afterwards, so a ciphertext LastModified BEFORE the manifest's
// completedAt is the normal ordering and must be served. A ciphertext
// LastModified AFTER completedAt means the ciphertext object was replaced after
// this manifest was finalized, leaving the manifest pointing at ciphertext it
// cannot describe — that manifest is stale and the read must be rejected.
// (This assumes ARMOR and the backend clock stay roughly aligned; the ADR-016
// overwrite window is seconds wide, so sub-second skew does not flip the
// comparison.)
// ciphertextFreshnessError marks a failed ADR-016 freshness verdict so the
// combined head-and-verify path can keep the freshness and head-failure error
// classes distinct.
type ciphertextFreshnessError struct{ err error }

func (e *ciphertextFreshnessError) Error() string { return e.err.Error() }
func (e *ciphertextFreshnessError) Unwrap() error { return e.err }

// headCiphertextForFreshness performs the one authoritative ciphertext HEAD a
// read needs: it verifies ADR-016 ciphertext freshness when completedAt is
// parseable and returns the stored ObjectInfo for the caller's size and
// generation needs (armor-c3199f1c). Freshness is evaluated on every call —
// nothing about the verdict is cached.
func (h *Handlers) headCiphertextForFreshness(ctx context.Context, bucket, ciphertextRef, completedAt string) (*backend.ObjectInfo, error) {
	// Parse completion timestamp
	createdAt, err := time.Parse(time.RFC3339, completedAt)
	if err != nil {
		// If we can't parse the timestamp, skip verification; the HEAD is
		// still required for the stored size below.
		createdAt = time.Time{}
	}

	// Get ciphertext object info
	info, err := h.backend.Head(ctx, bucket, ciphertextRef)
	if err != nil {
		return nil, fmt.Errorf("failed to head ciphertext object: %w", err)
	}

	if !createdAt.IsZero() {
		// Truncate to seconds since B2 timestamps have second precision
		ciphertextTime := info.LastModified.UTC().Truncate(time.Second)
		manifestTime := createdAt.Truncate(time.Second)

		if ciphertextTime.After(manifestTime) {
			return info, &ciphertextFreshnessError{fmt.Errorf("ciphertext object was overwritten after manifest completion (ciphertext %v, manifest completed %v)", ciphertextTime, manifestTime)}
		}
	}

	return info, nil
}

// verifyCiphertextFreshness checks that the ciphertext object referenced by the
// manifest is still the one the manifest was written for (not overwritten by a
// newer same-key upload). Callers that also need the stored size or the
// generation of the ciphertext object should call headCiphertextForFreshness
// directly and keep the single HEAD.
func (h *Handlers) verifyCiphertextFreshness(ctx context.Context, bucket, ciphertextRef string, completedAt string) error {
	_, err := h.headCiphertextForFreshness(ctx, bucket, ciphertextRef, completedAt)
	var freshErr *ciphertextFreshnessError
	if errors.As(err, &freshErr) {
		return freshErr.err
	}
	return err
}

// CompleteMultipartUpload handles S3 CompleteMultipartUpload.
// It assembles the parts in B2 and stores the HMAC table as a sidecar.
func (h *Handlers) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state/metadata
	// Try v3 format first (meta.json), fall back to v2 format (.state file)
	manager := h.getMultipartManager(bucket)

	// Try v3 format
	metadata, errV3 := manager.LoadMetadataV3(ctx, uploadID)
	var state *backend.MultipartState
	var err error
	var formatVersion int

	if errV3 == nil && metadata != nil {
		// V3 format loaded successfully
		formatVersion = 3
		// For v3, we'll load part data on-demand below
		// Create a minimal state structure for compatibility
		state = &backend.MultipartState{
			UploadID:          metadata.UploadID,
			Bucket:            metadata.Bucket,
			Key:               metadata.Key,
			IV:                metadata.IV,
			WrappedDEK:        metadata.WrappedDEK,
			MEKFingerprint:    metadata.MEKFingerprint,
			BlockSize:         metadata.BlockSize,
			ContentType:       metadata.ContentType,
			KeyID:             metadata.KeyID,
			Metadata:          metadata.Metadata,
			PartSize:          metadata.PartSize,
			NonUniformParts:   metadata.NonUniformParts,
			Poisoned:          metadata.Poisoned,
			PoisonReason:      metadata.PoisonReason,
			FormatVersion:     3,
			PartSizes:         make(map[int]int64),
			PartHMACs:         make(map[int]string),
			PartPlaintextSHAs: make(map[int]string),
		}
	} else {
		// Fall back to v2 format
		state, err = manager.LoadState(ctx, uploadID)
		if err != nil {
			h.writeError(w, r, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
			return
		}
		formatVersion = state.FormatVersion
		if formatVersion == 0 {
			formatVersion = 2 // Old state files without format version are v2
		}
	}

	// Verify bucket and key match
	if state.Bucket != bucket || state.Key != key {
		h.writeError(w, r, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
		return
	}

	// ADR-015 rule 4: if a prior UploadPart contradicted the uniform-part-size
	// contract, the upload id was poisoned and persisted. Fail clearly here,
	// before assembling or storing anything — the client must abort and retry.
	if state.Poisoned {
		h.writeError(w, r, "InvalidPart",
			fmt.Sprintf("This multipart upload has been invalidated and cannot be completed: %s. %s", state.PoisonReason, multipartRetryMessage), 400)
		return
	}

	// For v3 format, load part data from individual part-<n>.json files
	// This populates state.PartSizes and state.PartHMACs for the rest of the function
	if formatVersion == 3 {
		parts, err := manager.ListPartsV3(ctx, uploadID)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list parts: %v", err), 500)
			return
		}

		if len(parts) == 0 {
			h.writeError(w, r, "InvalidPart", "No parts have been uploaded for this multipart upload", 400)
			return
		}

		// Populate state maps from loaded part data
		for partNum, partData := range parts {
			state.PartSizes[partNum] = partData.PlaintextLen
			state.PartHMACs[partNum] = partData.BlockHMACsBase64
			if state.PartPlaintextSHAs == nil {
				state.PartPlaintextSHAs = make(map[int]string)
			}
			state.PartPlaintextSHAs[partNum] = partData.PlaintextSHAHex
		}

		// Version 3 encrypts every part in its own counter namespace, so part
		// sizes are intentionally unconstrained. Preserve part 1's size only
		// for legacy metadata consumers; mark the temporary state non-uniform so
		// the v2 uniform-size validation below does not reject valid v3 uploads.
		if partOne, ok := state.PartSizes[1]; ok {
			state.PartSize = partOne
		}
		state.NonUniformParts = true
	}

	// Parse the CompleteMultipartUpload request XML
	type Part struct {
		PartNumber int    `xml:"PartNumber"`
		ETag       string `xml:"ETag"`
	}

	type CompleteMultipartUploadReq struct {
		XMLName xml.Name `xml:"CompleteMultipartUpload"`
		Parts   []Part   `xml:"Part"`
	}

	var completeReq CompleteMultipartUploadReq
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := xml.Unmarshal(body, &completeReq); err != nil {
		h.writeError(w, r, "MalformedXML", fmt.Sprintf("Failed to parse XML: %v", err), 400)
		return
	}

	if len(completeReq.Parts) == 0 {
		h.writeError(w, r, "InvalidRequest", "No parts specified", 400)
		return
	}

	// CRITICAL: Sort parts by PartNumber before assembling HMACs and computing size
	// Clients may send CompleteMultipartUpload with parts in arbitrary order, but:
	// 1. B2 assembles parts in PartNumber order (Part 1, then Part 2, etc.)
	// 2. The HMAC sidecar must match: position N contains HMAC for block N
	// Without this sort, HMAC verification fails for out-of-order part lists.
	// See bf-2sq7gf: production litestream sends parts out of order, causing
	// "block 256: HMAC verification failed" because HMAC table was assembled in wrong order.
	sort.Slice(completeReq.Parts, func(i, j int) bool {
		return completeReq.Parts[i].PartNumber < completeReq.Parts[j].PartNumber
	})

	// ADR-015 rule 3 (Complete-time contract validation): every part except the
	// highest-numbered one must have size exactly P, the uniform part size pinned
	// from part 1 (ADR-015, amended 2026-07-19). The highest-numbered part is
	// allowed to be the short final part (< P). This is the authoritative gate —
	// UploadPart already rejects contradictions as they arrive (and poisons the
	// upload), but this backstop catches a violating state that reached Complete
	// anyway (e.g. a best-effort poison save that dropped). It runs before any
	// assembly/storage so a violating object is never stored.
	P := state.PartSize
	if P == 0 {
		h.writeError(w, r, "InvalidPart",
			fmt.Sprintf("Part 1 was never uploaded for this multipart upload, so the uniform part size is unknown (every other part would have been deferred with SlowDown until part 1 arrived). Upload part 1 and retry. %s", multipartRetryMessage), 400)
		return
	}
	highestPartNumber := completeReq.Parts[len(completeReq.Parts)-1].PartNumber
	for _, p := range completeReq.Parts {
		size, ok := state.PartSizes[p.PartNumber]
		if !ok {
			h.writeError(w, r, "InvalidPart",
				fmt.Sprintf("Part %d was not uploaded and cannot be completed. %s", p.PartNumber, multipartRetryMessage), 400)
			return
		}
		// ADR-011: Non-uniform parts are exempt from uniform-part-size validation.
		// When state.NonUniformParts is true, parts may vary in size.
		if !state.NonUniformParts {
			// The highest-numbered part may be the short final part (< P); every other
			// part must match P exactly.
			if p.PartNumber != highestPartNumber && size != P {
				h.writeError(w, r, "InvalidPartSize",
					fmt.Sprintf("Part %d has size %d but the uniform part size for this upload is %d (only the final part may differ). The uniform-part-size contract (ADR-015) was violated. %s", p.PartNumber, size, P, multipartRetryMessage), 400)
				return
			}
		}
	}
	// ADR-015 rule 1: a multi-part object's regular part size P must meet B2's
	// 5 MiB minimum. (A single-part upload — only the highest part — is the last
	// part and is exempt, matching B2.) Not enforced at UploadPart pin time so
	// the short-final-part-arriving-first case can still be detected and poisoned
	// by rule 4 instead of being rejected outright at the first part.
	if len(completeReq.Parts) > 1 && P < multipartMinPartSize {
		h.writeError(w, r, "InvalidPartSize",
			fmt.Sprintf("Uniform part size %d is smaller than the 5 MiB minimum for a multipart object (ADR-015). Use a part size of at least %d bytes. %s", P, multipartMinPartSize, multipartRetryMessage), 400)
		return
	}
	// Backstop for the single-part alignment exemption. A non-block-aligned P is
	// valid ONLY when the upload has exactly one part: part 1 starts at block 0,
	// so its size is unconstrained, but with a second part the
	// (N-1)*P/BlockSize offsets stop landing on block boundaries and the
	// assembled object would decrypt to garbage. UploadPart already rejects and
	// poisons a part >1 on such an upload; this catches a violating state that
	// reached Complete anyway (e.g. a best-effort poison save that dropped),
	// before anything is assembled or stored.
	// ADR-011: Non-uniform parts are exempt from block alignment validation.
	if !state.NonUniformParts && len(completeReq.Parts) > 1 && P%int64(state.BlockSize) != 0 {
		h.writeError(w, r, "InvalidPartSize",
			fmt.Sprintf("Uniform part size %d is not a multiple of the block size (%d bytes), which is only valid for a single-part upload, but this upload has %d parts. %s", P, state.BlockSize, len(completeReq.Parts), multipartRetryMessage), 400)
		return
	}
	// The v3 multipart layout carries its part boundaries in its sidecar, so
	// its temporary non-uniform marker is only for bypassing the legacy v2
	// validation above. Do not persist v2 cumulative-offset metadata for it.
	if formatVersion == 3 {
		state.NonUniformParts = false
	}

	// Convert to backend.CompletedPart
	parts := make([]backend.CompletedPart, len(completeReq.Parts))
	for i, p := range completeReq.Parts {
		parts[i] = backend.CompletedPart{
			PartNumber: int32(p.PartNumber),
			ETag:       p.ETag,
		}
	}

	// Calculate total plaintext size and expected ciphertext size
	// For encrypted multipart uploads, B2 assembles just the encrypted part data
	// (same size as plaintext). The HMAC table is stored separately as a sidecar
	// file at .armor/hmac/<sha256(key)> and is NOT included in the assembled object.
	var totalPlaintextSize int64
	var totalCiphertextSize int64
	for _, p := range completeReq.Parts {
		if size, ok := state.PartSizes[p.PartNumber]; ok {
			totalPlaintextSize += size
			// Each part's encrypted data is the same size as its plaintext
			// (HMACs are stored separately in the sidecar, not in the assembled object)
			totalCiphertextSize += size
		}
	}

	// Assemble all block HMACs in order (now that parts are sorted)
	// For non-uniform parts, track absolute block indices to handle boundary blocks correctly
	allBlockHMACsMap := make(map[uint32][]byte)
	cumulativeOffset := int64(0)
	for _, p := range completeReq.Parts {
		partSize, exists := state.PartSizes[p.PartNumber]
		if !exists {
			continue
		}

		if hmacsBase64, ok := state.PartHMACs[p.PartNumber]; ok {
			hmacs, err := backend.DecodeHMACFromBase64(hmacsBase64)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode HMACs for part %d: %v", p.PartNumber, err), 500)
				return
			}

			// Calculate starting block index for this part
			startBlockIndex := uint32(cumulativeOffset / int64(state.BlockSize))

			// Map each HMAC to its absolute block index
			// Include placeholder HMACs (zeros) so the final array has no gaps
			for i, hmac := range hmacs {
				absBlockIndex := startBlockIndex + uint32(i)
				allBlockHMACsMap[absBlockIndex] = hmac
			}
		}

		cumulativeOffset += partSize
	}

	// Convert map to sorted array
	totalBlocks := uint32((cumulativeOffset + int64(state.BlockSize) - 1) / int64(state.BlockSize))
	allBlockHMACs := make([][]byte, totalBlocks)
	for blockIdx := uint32(0); blockIdx < totalBlocks; blockIdx++ {
		allBlockHMACs[blockIdx] = allBlockHMACsMap[blockIdx]
	}

	// Check Version 2 counter space won't overflow before completing the upload
	// Version 2 stores blockIndex * (blockSize / 16) in a uint32, which wraps at
	// 2^32 / (blockSize / 16) blocks. At 64 KiB blocks, this is 2^20 blocks = 64 GiB.
	// All new multipart uploads use Version 2 (see line 2861), so we check unconditionally.
	const maxCounterValue = 1 << 32
	aesBlocksPerArmorBlock := uint64(state.BlockSize / 16)
	finalCounterValue := uint64(totalBlocks) * aesBlocksPerArmorBlock
	if finalCounterValue >= maxCounterValue {
		h.writeError(w, r, "InternalError", "object exceeds the Version 2 counter space; envelope v3 removes this limit", 500)
		return
	}

	// Complete the multipart upload in B2, under the same prefixed storage key
	// the upload was created and its parts uploaded under.
	etag, err := h.backend.CompleteMultipartUpload(ctx, bucket, h.applyPrefix(key), uploadID, parts)
	if err != nil {
		// A long object-store completion can outlive the originating HTTP
		// request. In that ambiguous-success case the retry receives
		// NoSuchUpload because B2 already consumed the upload id. Resume the
		// ARMOR metadata/sidecar finalization only when the newly assembled raw
		// object has the exact expected ciphertext size and was created after
		// this upload began. This rejects a stale same-key object instead of
		// blessing it as the result of the timed-out upload.
		if !backend.IsNoSuchUpload(err) {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to complete multipart upload: %v", err), 500)
			return
		}
		info, headErr := h.backend.Head(ctx, bucket, h.applyPrefix(key))
		// B2 ObjectInfo timestamps have whole-second precision, while the
		// multipart state Created timestamp includes nanoseconds. Truncate the
		// upload timestamp before comparing so a valid completion in the same
		// second is not rejected as stale.
		createdAt := state.Created.UTC().Truncate(time.Second)
		if headErr != nil || info == nil || info.Size != totalCiphertextSize ||
			(!state.Created.IsZero() && info.LastModified.UTC().Before(createdAt)) {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to recover ambiguous multipart completion: complete=%v head=%v", err, headErr), 500)
			return
		}
		etag = info.ETag
	}

	// The completed object is now visible to list operations. Invalidate all
	// cached queries whose prefixes cover this key, including bare prefixes.
	if h.listCache != nil {
		h.listCache.InvalidateForWrite(bucket, key)
	}

	// ADR-011: Non-uniform parts - leave placeholder HMACs (zeros) for boundary blocks.
	// These will be skipped during decryption. The offset encryptor already creates
	// placeholder HMACs for partial blocks, so we don't need to backfill them.
	// The decryption code handles non-uniform parts by using offset-aware decryption.

	// Store HMAC table as sidecar with version
	// For v3 format, build the per-part sidecar structure from the loaded part data
	if formatVersion == 3 {
		// Build v3 sidecar parts from the loaded part data
		v3Parts := make([]backend.HMACPartV3, 0, len(completeReq.Parts))

		for _, p := range completeReq.Parts {
			partSize, exists := state.PartSizes[p.PartNumber]
			if !exists {
				continue
			}

			// Get HMACs for this part
			hmacsBase64, ok := state.PartHMACs[p.PartNumber]
			if !ok {
				continue
			}

			hmacs, err := backend.DecodeHMACFromBase64(hmacsBase64)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode HMACs for part %d: %v", p.PartNumber, err), 500)
				return
			}

			// Build blocks array: [hmac_b64, clen] for each block
			blocks := make([][]string, len(hmacs))
			for i, hmac := range hmacs {
				// Calculate ciphertext length for this block
				// For multipart uploads, each block's ciphertext length equals plaintext length
				// except possibly the last block of the part
				blockIndex := i
				var blockCiphertextLen int

				// Calculate block plaintext size
				blockStart := int64(blockIndex) * int64(state.BlockSize)
				blockEnd := blockStart + int64(state.BlockSize)
				if blockEnd > partSize {
					blockEnd = partSize
				}
				blockPlaintextLen := blockEnd - blockStart
				blockCiphertextLen = int(blockPlaintextLen)

				lengthBytes := make([]byte, 4)
				binary.BigEndian.PutUint32(lengthBytes, uint32(blockCiphertextLen))
				blocks[i] = []string{
					base64.StdEncoding.EncodeToString(hmac),
					base64.StdEncoding.EncodeToString(lengthBytes),
				}
			}

			v3Parts = append(v3Parts, backend.HMACPartV3{
				N:             p.PartNumber,
				PlaintextLen:  partSize,
				CiphertextLen: partSize, // For multipart without compression, ciphertext equals plaintext
				Blocks:        blocks,
			})
		}

		// Save v3 sidecar with gzip compression
		if err := manager.SaveHMACTableV3(ctx, key, state.BlockSize, v3Parts); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to save v3 HMAC table: %v", err), 500)
			return
		}
	} else {
		// v2 format: save as binary sidecar
		if err := manager.SaveHMACTable(ctx, key, allBlockHMACs, state.BlockSize, 2); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to save HMAC table: %v", err), 500)
			return
		}
	}

	// Compute the real whole-object plaintext SHA-256 by combining the per-part
	// digests accumulated during UploadPart, in ascending part-number order.
	// Parts arrive out of order (ADR-015), so the per-part digests cannot be
	// streamed into one hash during upload; this combination is the
	// order-sensitive step and is exactly what a verifier reproduces from the
	// decrypted plaintext split at the uniform part-size P boundaries
	// (backend.ComputeMultipartDigest). Replaces the empty-string placeholder
	// (bf-1v2ehf / ADR-003 residual gap).
	partNumbers := make([]int, len(completeReq.Parts))
	for i, p := range completeReq.Parts {
		partNumbers[i] = p.PartNumber
	}
	plaintextSHAHex, err := backend.CombinePartPlaintextSHAs(state.PartPlaintextSHAs, partNumbers)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to assemble plaintext SHA-256: %v", err), 500)
		return
	}

	// Build ARMOR metadata for manifest object (ADR-016)
	// Use version 3 for v3 format, version 2 for v2 format
	armorVersion := 2
	if formatVersion == 3 {
		armorVersion = 3
	}
	meta := (&backend.ARMORMetadata{
		Version:        armorVersion,
		BlockSize:      state.BlockSize,
		PlaintextSize:  totalPlaintextSize,
		ContentType:    state.ContentType,
		IV:             state.IV,
		WrappedDEK:     state.WrappedDEK,
		MEKFingerprint: state.MEKFingerprint,
		PlaintextSHA:   plaintextSHAHex,
		ETag:           etag,
		KeyID:          state.KeyID,
	}).ToMetadata()
	for key, value := range state.Metadata {
		meta[key] = value
	}

	// Add multipart flag to indicate HMAC table is external
	meta["x-amz-meta-armor-multipart"] = "true"

	// Record the uniform part size P so a verifier (or any future audit) can
	// recompute the whole-object digest from the decrypted plaintext by splitting
	// at P boundaries (backend.ComputeMultipartDigest). Without it the digest is
	// not independently reproducible.
	meta["x-amz-meta-armor-part-size"] = strconv.FormatInt(P, 10)

	// ADR-011: Record non-uniform parts flag and cumulative part sizes
	// so GET can use OffsetDecryptor with correct per-part offsets
	if state.NonUniformParts {
		meta["x-amz-meta-armor-non-uniform"] = "true"

		// Store cumulative part sizes as JSON: map[partNumber]offset
		// This allows GET to reconstruct which parts contribute to each block
		if state.CumulativePartSizes != nil && len(state.CumulativePartSizes) > 0 {
			// Convert to JSON for storage
			cumulativeJSON, err := json.Marshal(state.CumulativePartSizes)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to marshal cumulative part sizes: %v", err), 500)
				return
			}
			meta["x-amz-meta-armor-cumulative-sizes"] = string(cumulativeJSON)
		}
	}

	// ADR-016: Write manifest object instead of using CopyObject
	// This avoids the 5GB limit and race condition window of CopyObject
	ciphertextRef := h.applyPrefix(key)
	if err := h.writeManifest(ctx, bucket, key, meta, uploadID, etag, ciphertextRef); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to write manifest after completion: %v", err), 500)
		return
	}

	// Clean up multipart state
	manager.DeleteState(ctx, uploadID)
	h.cleanupMultipartLocks(uploadID)

	// Record in manifest for fast metadata lookup (async B2 persistence)
	// When manifest is enabled, provenance is embedded in delta lines.
	// When manifest is disabled, provenance uses per-object entries.
	var chainEntry *manifest.ChainEntry
	if h.manifest != nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		entryData, err := h.provenance.CreateChainEntry(ctx, key, plaintextSHAHex, "multipart")
		if err == nil && entryData != nil {
			chainEntry = &manifest.ChainEntry{
				Sequence:      entryData.Sequence,
				ChainHash:     entryData.ChainHash,
				PrevChainHash: entryData.PrevChainHash,
			}
		}
	}
	if h.manifest != nil {
		h.manifest.RecordPut(bucket, key, totalPlaintextSize, plaintextSHAHex, state.IV, state.WrappedDEK, state.MEKFingerprint, state.BlockSize, state.ContentType, etag, chainEntry, totalCiphertextSize, clientMetadata(state.Metadata))
	}

	// Completion installed a new stored generation at this key; retire any
	// cached entry for the previous one (see the PUT paths above).
	h.cache.Delete(bucket, key)
	// A LIST may have filled its page cache after the early invalidation above
	// but before writeManifest and RecordPut completed. Retire those pages
	// again so a successful completion is visible on the next LIST. Include
	// the manifest sibling for clients listing that exact prefix.
	if h.listCache != nil {
		h.listCache.InvalidateForWrite(bucket, key)
		h.listCache.InvalidateForWrite(bucket, key+manifestSuffix)
	}

	// Record provenance (fallback when manifest is disabled)
	if h.manifest == nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		h.recordProvenanceUpload(ctx, key, plaintextSHAHex, "multipart")
	}

	// Build XML response
	type CompleteMultipartUploadResult struct {
		XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
		Xmlns    string   `xml:"xmlns,attr"`
		Location string   `xml:"Location"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		ETag     string   `xml:"ETag"`
	}

	named := h.namedBucket(r)
	result := CompleteMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: fmt.Sprintf("/%s/%s", named, key),
		Bucket:   named,
		Key:      key,
		ETag:     etag,
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)

	// Enqueue replication task to secondary backend if configured (non-blocking)
	// This runs in a goroutine after the client receives the success response
	if h.replicationQueue != nil {
		go func() {
			// Recover from panics to prevent goroutine crashes
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC in replication enqueue (completemultipart %s/%s): %v", bucket, key, r)
				}
			}()

			// Enqueue the replication task
			// Note: Enqueue() is non-blocking and does not return errors
			// Dropped items are tracked via the replication_dropped_total metric
			if h.replicationQueue != nil {
				h.replicationQueue.Enqueue(bucket, h.applyPrefix(key))
				if h.metrics != nil {
					h.metrics.IncReplicationEnqueued("completemultipart")
				}
			} else {
				log.Printf("replication queue is nil, skipping enqueue for %s/%s", bucket, key)
			}
		}()
	}
}

// AbortMultipartUpload handles S3 AbortMultipartUpload.
// It deletes the multipart state and forwards the abort to B2.
func (h *Handlers) AbortMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state/metadata to verify it exists
	// Try v3 format first (meta.json), fall back to v2 format (.state file)
	manager := h.getMultipartManager(bucket)

	// Try v3 format
	metadata, errV3 := manager.LoadMetadataV3(ctx, uploadID)
	if errV3 != nil {
		// Fall back to v2 format
		state, err := manager.LoadState(ctx, uploadID)
		if err != nil {
			h.writeError(w, r, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
			return
		}

		// Verify bucket and key match
		if state.Bucket != bucket || state.Key != key {
			h.writeError(w, r, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
			return
		}
	} else {
		// Verify bucket and key match for v3
		if metadata.Bucket != bucket || metadata.Key != key {
			h.writeError(w, r, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
			return
		}
	}

	// Forward abort to B2
	if err := h.backend.AbortMultipartUpload(ctx, bucket, key, uploadID); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to abort multipart upload: %v", err), 500)
		return
	}

	// Clean up multipart state (handles both v2 and v3 formats)
	manager.DeleteState(ctx, uploadID)
	h.cleanupMultipartLocks(uploadID)

	w.WriteHeader(http.StatusNoContent)
}

// ListParts handles S3 ListParts operation.
// It forwards to B2 and adjusts part sizes to plaintext sizes.
func (h *Handlers) ListParts(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state/metadata to get plaintext sizes
	// Try v3 format first (meta.json), fall back to v2 format (.state file)
	manager := h.getMultipartManager(bucket)

	// Try v3 format
	metadata, errV3 := manager.LoadMetadataV3(ctx, uploadID)
	var state *backend.MultipartState
	var err error
	var formatVersion int

	if errV3 == nil && metadata != nil {
		// V3 format loaded successfully
		formatVersion = 3
		// For v3, load part data on-demand below
		state = &backend.MultipartState{
			UploadID:  metadata.UploadID,
			Bucket:    metadata.Bucket,
			Key:       metadata.Key,
			PartSizes: make(map[int]int64),
		}
	} else {
		// Fall back to v2 format
		state, err = manager.LoadState(ctx, uploadID)
		if err != nil {
			h.writeError(w, r, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
			return
		}
		formatVersion = state.FormatVersion
		if formatVersion == 0 {
			formatVersion = 2 // Old state files without format version are v2
		}
	}

	// Verify bucket and key match
	if state.Bucket != bucket || state.Key != key {
		h.writeError(w, r, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
		return
	}

	// Forward to B2
	result, err := h.backend.ListParts(ctx, bucket, key, uploadID)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list parts: %v", err), 500)
		return
	}

	// For v3, load part data if needed
	if formatVersion == 3 {
		parts, err := manager.ListPartsV3(ctx, uploadID)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list parts: %v", err), 500)
			return
		}

		// Populate state.PartSizes from loaded part data
		for partNum, partData := range parts {
			state.PartSizes[partNum] = partData.PlaintextLen
		}
	}

	// Build XML response with plaintext sizes
	type Part struct {
		PartNumber   int    `xml:"PartNumber"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		LastModified string `xml:"LastModified"`
	}

	type ListPartsResult struct {
		XMLName              xml.Name `xml:"ListPartsResult"`
		Xmlns                string   `xml:"xmlns,attr"`
		Bucket               string   `xml:"Bucket"`
		Key                  string   `xml:"Key"`
		UploadID             string   `xml:"UploadId"`
		StorageClass         string   `xml:"StorageClass"`
		PartNumberMarker     int      `xml:"PartNumberMarker"`
		NextPartNumberMarker int      `xml:"NextPartNumberMarker"`
		MaxParts             int      `xml:"MaxParts"`
		IsTruncated          bool     `xml:"IsTruncated"`
		Parts                []Part   `xml:"Part"`
	}

	resp := ListPartsResult{
		Xmlns:                "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:               h.namedBucket(r),
		Key:                  key,
		UploadID:             uploadID,
		StorageClass:         "STANDARD",
		PartNumberMarker:     result.NextPartNumberMarker,
		NextPartNumberMarker: result.NextPartNumberMarker,
		IsTruncated:          result.IsTruncated,
	}

	for _, part := range result.Parts {
		// Use plaintext size from state if available, otherwise use reported size
		plaintextSize := part.Size
		if size, ok := state.PartSizes[int(part.PartNumber)]; ok {
			plaintextSize = size
		}

		resp.Parts = append(resp.Parts, Part{
			PartNumber:   int(part.PartNumber),
			ETag:         part.ETag,
			Size:         plaintextSize,
			LastModified: part.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// ListMultipartUploads handles S3 ListMultipartUploads operation.
// It forwards directly to B2 (passthrough operation).
//
// Prefix handling (when ARMOR_PREFIX is configured):
// 1. Client sends unprefixed keys in request (prefix, key-marker)
// 2. Handler prepends prefix before calling backend: backendPrefix = applyPrefix(prefix)
// 3. Backend returns results with prefixed keys
// 4. Handler strips prefix from upload keys and next-key marker before returning to client
//
// This pattern matches ListObjectsV2 and ListObjectVersions exactly.
func (h *Handlers) ListMultipartUploads(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	prefix := r.URL.Query().Get("prefix")

	// Apply the configured prefix to the prefix parameter for backend operations.
	// When ARMOR_PREFIX is set, the backend stores all keys with the prefix prepended,
	// but clients don't know about it. We need to prepend the prefix to the client's
	// requested prefix so the backend finds the right uploads.
	backendPrefix := h.applyPrefix(prefix)

	result, err := h.backend.ListMultipartUploads(ctx, bucket, backendPrefix)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list multipart uploads: %v", err), 500)
		return
	}

	// Build XML response
	type Upload struct {
		Key          string `xml:"Key"`
		UploadID     string `xml:"UploadId"`
		Initiator    string `xml:"Initiator>ID"`
		Owner        string `xml:"Owner>ID"`
		StorageClass string `xml:"StorageClass"`
		Initiated    string `xml:"Initiated"`
	}

	type ListMultipartUploadsResult struct {
		XMLName            xml.Name `xml:"ListMultipartUploadsResult"`
		Xmlns              string   `xml:"xmlns,attr"`
		Bucket             string   `xml:"Bucket"`
		KeyMarker          string   `xml:"KeyMarker"`
		UploadIDMarker     string   `xml:"UploadIdMarker"`
		NextKeyMarker      string   `xml:"NextKeyMarker"`
		NextUploadIDMarker string   `xml:"NextUploadIdMarker"`
		MaxUploads         int      `xml:"MaxUploads"`
		IsTruncated        bool     `xml:"IsTruncated"`
		Uploads            []Upload `xml:"Upload"`
	}

	resp := ListMultipartUploadsResult{
		Xmlns:              "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:             h.namedBucket(r),
		KeyMarker:          r.URL.Query().Get("key-marker"),
		UploadIDMarker:     r.URL.Query().Get("upload-id-marker"),
		NextKeyMarker:      h.stripPrefix(result.NextKeyMarker),
		NextUploadIDMarker: result.NextUploadIDMarker,
		IsTruncated:        result.IsTruncated,
	}

	for _, upload := range result.Uploads {
		// Strip the configured prefix from upload keys before returning to client.
		// Clients don't know about the prefix, so we remove it from the keys.
		resp.Uploads = append(resp.Uploads, Upload{
			Key:          h.stripPrefix(upload.Key),
			UploadID:     upload.UploadID,
			Initiator:    "armor",
			Owner:        "armor",
			StorageClass: "STANDARD",
			Initiated:    upload.Initiated.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// ListObjectVersions handles S3 ListObjectVersions operation.
// It lists all versions of objects in a bucket. For ARMOR-encrypted objects,
// it retrieves per-version metadata to provide plaintext sizes.
//
// Prefix handling (when ARMOR_PREFIX is configured):
// 1. Client sends unprefixed keys in request (prefix, keyMarker)
// 2. Handler prepends prefix before calling backend: backendPrefix = applyPrefix(prefix)
// 3. Backend returns results with prefixed keys
// 4. Handler strips prefix from version keys and common prefixes before returning to client
//
// This pattern matches ListObjectsV2 exactly, ensuring consistent behavior across all list operations.
func (h *Handlers) ListObjectVersions(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	keyMarker := r.URL.Query().Get("key-marker")
	versionIDMarker := r.URL.Query().Get("version-id-marker")
	maxKeys := 1000
	if mk := r.URL.Query().Get("max-keys"); mk != "" {
		if v, err := strconv.Atoi(mk); err == nil && v > 0 {
			maxKeys = v
		}
	}

	// Apply the configured prefix to the prefix parameter for backend operations.
	// When ARMOR_PREFIX is set, the backend stores all keys with the prefix prepended,
	// but clients don't know about it. We need to prepend the prefix to the client's
	// requested prefix so the backend finds the right objects.
	backendPrefix := h.applyPrefix(prefix)
	backendKeyMarker := h.applyPrefix(keyMarker)

	result, err := h.backend.ListObjectVersions(ctx, bucket, backendPrefix, delimiter, backendKeyMarker, versionIDMarker, maxKeys)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list object versions: %v", err), 500)
		return
	}

	// Build XML response
	type Version struct {
		Key            string `xml:"Key"`
		VersionID      string `xml:"VersionId"`
		IsLatest       bool   `xml:"IsLatest"`
		IsDeleteMarker bool   `xml:"IsDeleteMarker,omitempty"`
		LastModified   string `xml:"LastModified"`
		ETag           string `xml:"ETag,omitempty"`
		Size           int64  `xml:"Size,omitempty"`
		StorageClass   string `xml:"StorageClass,omitempty"`
	}

	type ListVersionsResult struct {
		XMLName             xml.Name  `xml:"ListVersionsResult"`
		Xmlns               string    `xml:"xmlns,attr"`
		Name                string    `xml:"Name"`
		Prefix              string    `xml:"Prefix"`
		Delimiter           string    `xml:"Delimiter,omitempty"`
		MaxKeys             int       `xml:"MaxKeys"`
		IsTruncated         bool      `xml:"IsTruncated"`
		KeyMarker           string    `xml:"KeyMarker"`
		VersionIDMarker     string    `xml:"VersionIdMarker"`
		NextKeyMarker       string    `xml:"NextKeyMarker"`
		NextVersionIDMarker string    `xml:"NextVersionIdMarker"`
		Versions            []Version `xml:"Version"`
		CommonPrefixes      []string  `xml:"CommonPrefixes>Prefix"`
	}

	resp := ListVersionsResult{
		Xmlns:               "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:                h.namedBucket(r),
		Prefix:              prefix,
		Delimiter:           delimiter,
		MaxKeys:             maxKeys,
		IsTruncated:         result.IsTruncated,
		KeyMarker:           keyMarker,
		VersionIDMarker:     versionIDMarker,
		NextKeyMarker:       h.stripPrefix(result.NextKeyMarker),
		NextVersionIDMarker: result.NextVersionIDMarker,
	}

	// Process versions and retrieve per-version metadata for ARMOR objects
	for _, version := range result.Versions {
		// Strip the configured prefix from version keys before returning to client.
		// Clients don't know about the prefix, so we need to remove it from the keys.
		strippedKey := h.stripPrefix(version.Key)
		v := Version{
			Key:            strippedKey,
			VersionID:      version.VersionID,
			IsLatest:       version.IsLatest,
			IsDeleteMarker: version.IsDeleteMarker,
			LastModified:   version.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
		}

		if !version.IsDeleteMarker {
			// For the latest version, try the manifest index first to avoid a
			// per-version HeadObject B2 API call. The manifest tracks current
			// state only, so this optimisation applies only to IsLatest.
			manifestHit := false
			if h.manifest != nil && version.IsLatest {
				if entry, ok := h.manifest.Lookup(bucket, version.Key); ok {
					v.Size = entry.PlaintextSize
					v.ETag = fmt.Sprintf(`"%s"`, entry.ETag)
					v.StorageClass = "STANDARD"
					manifestHit = true
				}
			}
			if !manifestHit {
				// Fall back to a per-version HeadObject call (non-latest versions
				// or manifest miss).
				if info, err := h.backend.HeadVersion(ctx, bucket, version.Key, version.VersionID); err == nil {
					if am, ok := backend.ParseARMORMetadata(info.Metadata); ok {
						v.Size = am.PlaintextSize
						v.ETag = fmt.Sprintf(`"%s"`, am.ETag)
						v.StorageClass = "STANDARD"
					} else {
						v.Size = version.Size
						v.ETag = fmt.Sprintf(`"%s"`, version.ETag)
						v.StorageClass = "STANDARD"
					}
				} else {
					v.Size = version.Size
					v.ETag = fmt.Sprintf(`"%s"`, version.ETag)
					v.StorageClass = "STANDARD"
				}
			}
		}

		resp.Versions = append(resp.Versions, v)
	}

	// Sort common prefixes lexicographically per S3 spec
	sort.Strings(result.CommonPrefixes)

	// Process common prefixes
	for _, p := range result.CommonPrefixes {
		// Strip the configured prefix from common prefixes before returning to client.
		// Common prefixes are used for directory-like listings with delimiters.
		strippedPrefix := h.stripPrefixFromCommonPrefix(p)
		resp.CommonPrefixes = append(resp.CommonPrefixes, strippedPrefix)
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// GetBucketLifecycleConfiguration handles GET ?lifecycle on a bucket.
// This is a passthrough operation - lifecycle configuration is not encrypted.
func (h *Handlers) GetBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	config, err := h.backend.GetBucketLifecycleConfiguration(ctx, bucket)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get lifecycle configuration: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(config)
}

// PutBucketLifecycleConfiguration handles PUT ?lifecycle on a bucket.
// This is a passthrough operation - lifecycle configuration is not encrypted.
func (h *Handlers) PutBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	// Read the lifecycle configuration XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutBucketLifecycleConfiguration(ctx, bucket, body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put lifecycle configuration: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBucketLifecycleConfiguration handles DELETE ?lifecycle on a bucket.
// This is a passthrough operation - lifecycle configuration is not encrypted.
func (h *Handlers) DeleteBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.DeleteBucketLifecycleConfiguration(ctx, bucket); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to delete lifecycle configuration: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetObjectLockConfiguration handles GET ?object-lock on a bucket.
// This is a passthrough operation - object lock configuration is not encrypted.
func (h *Handlers) GetObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	config, err := h.backend.GetObjectLockConfiguration(ctx, bucket)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object lock configuration: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write(config)
}

// PutObjectLockConfiguration handles PUT ?object-lock on a bucket.
// This is a passthrough operation - object lock configuration is not encrypted.
func (h *Handlers) PutObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	// Read the object lock configuration XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutObjectLockConfiguration(ctx, bucket, body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put object lock configuration: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectRetention handles GET ?retention on an object.
// This is a passthrough operation - retention settings are not encrypted.
func (h *Handlers) GetObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	retention, err := h.backend.GetObjectRetention(ctx, bucket, key)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object retention: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write(retention)
}

// PutObjectRetention handles PUT ?retention on an object.
// This is a passthrough operation - retention settings are not encrypted.
func (h *Handlers) PutObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Read the retention XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutObjectRetention(ctx, bucket, key, body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put object retention: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectLegalHold handles GET ?legal-hold on an object.
// This is a passthrough operation - legal hold status is not encrypted.
func (h *Handlers) GetObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	legalHold, err := h.backend.GetObjectLegalHold(ctx, bucket, key)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object legal hold: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write(legalHold)
}

// PutObjectLegalHold handles PUT ?legal-hold on an object.
// This is a passthrough operation - legal hold status is not encrypted.
func (h *Handlers) PutObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Read the legal hold XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutObjectLegalHold(ctx, bucket, key, body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put object legal hold: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// applyPrefix adds the configured prefix to a key for backend operations.
// If no prefix is configured, returns the key unchanged.
func (h *Handlers) applyPrefix(key string) string {
	if h.config.Prefix == "" {
		return key
	}
	return h.config.Prefix + key
}

// stripPrefix removes the configured prefix from a key for client responses.
// If no prefix is configured or the key doesn't start with the prefix, returns the key unchanged.
func (h *Handlers) stripPrefix(key string) string {
	if h.config.Prefix == "" {
		return key
	}
	if strings.HasPrefix(key, h.config.Prefix) {
		return strings.TrimPrefix(key, h.config.Prefix)
	}
	return key
}

// stripPrefixFromCommonPrefix removes the configured prefix from a common prefix string for client responses.
// Handles the case where the common prefix is a directory path ending with /.
func (h *Handlers) stripPrefixFromCommonPrefix(commonPrefix string) string {
	if h.config.Prefix == "" {
		return commonPrefix
	}
	if strings.HasPrefix(commonPrefix, h.config.Prefix) {
		return strings.TrimPrefix(commonPrefix, h.config.Prefix)
	}
	return commonPrefix
}

// writeError writes an S3 error response with request ID and resource.
// The request ID is extracted from the request context and included in the XML body.
// The resource is the request path. This function also logs a structured s3_error event.
func (h *Handlers) writeError(w http.ResponseWriter, r *http.Request, code, message string, statusCode int) {
	h.writeErrorCause(w, r, code, message, statusCode, nil)
}

// writeBackendError keeps backend deadline causes without treating upstream
// connection errors as evidence that the downstream client disconnected.
func (h *Handlers) writeBackendError(w http.ResponseWriter, r *http.Request, code, message string, statusCode int, cause error) {
	if errors.Is(cause, context.DeadlineExceeded) {
		h.writeErrorCause(w, r, code, message, statusCode, cause)
		return
	}
	h.writeError(w, r, code, message, statusCode)
}

// writeErrorCause retains the typed body-read error so connection failures
// can be distinguished from backend and application errors.
func (h *Handlers) writeErrorCause(w http.ResponseWriter, r *http.Request, code, message string, statusCode int, cause error) {
	switch middleware.ClassifyRequestFailure(r.Context(), cause) {
	case middleware.RequestFailureDeadline:
		code, message, statusCode = "RequestTimeout", "ARMOR request deadline exceeded", http.StatusServiceUnavailable
	case middleware.RequestFailureClientDisconnect:
		code, statusCode = "ClientDisconnect", 499
		*r = *r.WithContext(middleware.MarkClientDisconnect(r.Context()))
	}
	// A body whose digest disagrees with the signed x-amz-content-sha256 is a
	// client error, not an internal one: fail it with 400 before the object is
	// finalised (armor-328896ea).
	var payloadMismatch *ContentSHA256MismatchError
	if errors.As(cause, &payloadMismatch) {
		code, message, statusCode = ContentSHA256MismatchCode, payloadMismatch.Error(), http.StatusBadRequest
	}
	// A chunk signature that fails the chained streaming verification is a
	// client error too: fail the upload with 403 at the first bad chunk
	// (armor-3b18e5bc).
	var chunkMismatch *ChunkSignatureMismatchError
	if errors.As(cause, &chunkMismatch) {
		code, message, statusCode = ChunkSignatureMismatchCode, chunkMismatch.Error(), http.StatusForbidden
	}
	*r = *r.WithContext(middleware.SetErrorCode(r.Context(), code))

	// Extract request ID from middleware context
	requestID := middleware.GetRequestID(r.Context())
	errorEntry := s3errors.Resolve(code, s3errors.Request{
		Method:    r.Method,
		Path:      r.URL.Path,
		RequestID: requestID,
		Status:    statusCode,
	}, message)

	// Build resource path from request URL
	resource := r.URL.Path

	// Set headers
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(statusCode)

	// Build XML response
	var codeBuf, msgBuf, ridBuf, resBuf bytes.Buffer
	xml.EscapeText(&codeBuf, []byte(code))
	xml.EscapeText(&msgBuf, []byte(errorEntry.ClientMessage))
	xml.EscapeText(&ridBuf, []byte(requestID))
	xml.EscapeText(&resBuf, []byte(resource))

	var writeErr error
	if requestID != "" {
		_, writeErr = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n  <RequestId>%s</RequestId>\n  <Resource>%s</Resource>\n</Error>",
			codeBuf.String(), msgBuf.String(), ridBuf.String(), resBuf.String())
	} else {
		_, writeErr = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n  <Resource>%s</Resource>\n</Error>",
			codeBuf.String(), msgBuf.String(), resBuf.String())
	}
	if middleware.ClassifyRequestFailure(r.Context(), writeErr) == middleware.RequestFailureClientDisconnect {
		code, statusCode = "ClientDisconnect", 499
		*r = *r.WithContext(middleware.MarkClientDisconnect(r.Context()))
		*r = *r.WithContext(middleware.SetErrorCode(r.Context(), code))
	}
	// Count after writing so a failed writer cannot leave both a server error
	// and a client disconnect in the two counters.
	if h.metrics != nil {
		if middleware.IsClientDisconnect(r.Context()) {
			h.metrics.IncClientDisconnects(s3OperationForRequest(r))
		} else {
			h.metrics.IncErrors(code, s3OperationForRequest(r))
		}
	}
	if h.logger != nil {
		h.logS3Error(r, code, errorEntry.LogDetail, statusCode)
	}
}

// logS3Error emits a structured log line for S3 errors with request context.
func (h *Handlers) logS3Error(r *http.Request, code, message string, statusCode int) {
	// Extract request ID from middleware context
	requestID := middleware.GetRequestID(r.Context())

	// Extract operation (verb) from request
	operation := acl.ActionForRequest(r)

	// Extract bucket and key from request URL
	bucket, key := h.extractBucketAndKey(r)

	// Extract access_key_id from credential context
	var accessKeyID string
	if cred := h.credentialFromContext(r.Context()); cred != nil {
		accessKeyID = cred.AccessKey
	}

	// Build log fields
	fields := map[string]interface{}{
		"event":      "s3_error",
		"error_code": code,
		"operation":  operation,
		"bucket":     bucket,
		"key":        key,
		"request_id": requestID,
		"status":     statusCode,
		"message":    message,
	}
	if code == "ClientDisconnect" {
		fields["outcome"] = "client_disconnect"
	}

	// Add access_key_id only when present (no empty fields)
	if accessKeyID != "" {
		fields["access_key_id"] = accessKeyID
	}

	// Log at appropriate level: WARN for 4xx, ERROR for 5xx
	if statusCode >= 400 && statusCode < 500 {
		h.logger.WithFields(fields).Warn("S3 operation failed")
	} else if statusCode >= 500 {
		h.logger.WithFields(fields).Error("S3 operation failed")
	} else {
		// Unexpected: non-error status codes should not use error logging path
		h.logger.WithFields(fields).Info("S3 operation error logged with non-error status")
	}
}

// namedBucket returns the bucket exactly as the client named it, before alias
// resolution. Response bodies echo this value: S3 clients compare the bucket in
// a response against the one they addressed, so replying with config.Bucket to
// a request sent to a legacy alias makes the client discard a response that was
// in fact correct. Handlers keep using the resolved name for backend calls.
func (h *Handlers) namedBucket(r *http.Request) string {
	named := config.ParseBucketKey(r.URL.Path).Bucket
	if named == "" {
		// A request that named no bucket is served from the configured bucket.
		// None of the handlers echoing a bucket name are reachable that way, but
		// ResolveBucket keeps the fallback consistent if one ever is.
		return h.config.ResolveBucket(named)
	}
	return named
}

// extractBucketAndKey extracts bucket and key from the request URL.
func (h *Handlers) extractBucketAndKey(r *http.Request) (bucket, key string) {
	path := r.URL.Path
	// Remove leading slash
	path = strings.TrimPrefix(path, "/")

	// For path-style: /bucket/key
	parts := strings.SplitN(path, "/", 2)
	if len(parts) >= 1 && parts[0] != "" {
		bucket = parts[0]
	}
	if len(parts) >= 2 {
		key = parts[1]
		// URL decode the key
		if decoded, err := url.PathUnescape(key); err == nil {
			key = decoded
		}
	}

	// Use configured bucket if empty
	if bucket == "" && h.config != nil {
		bucket = h.config.Bucket
	}

	return bucket, key
}

// credentialFromContext retrieves the credential from the request context.
// Returns nil if no credential is present (e.g., for public endpoints).
func (h *Handlers) credentialFromContext(ctx context.Context) *config.Credential {
	if cred, ok := acl.CredentialFromContext(ctx).(*config.Credential); ok {
		return cred
	}
	return nil
}

// isPlaceholderHMAC checks if an HMAC is a placeholder (all zeros).
// Placeholder HMACs are used for boundary blocks that will be computed during CompleteMultipartUpload.
func isPlaceholderHMAC(hmac []byte) bool {
	if len(hmac) == 0 {
		return false
	}
	for _, b := range hmac {
		if b != 0 {
			return false
		}
	}
	return true
}

// v3BlockTableTrailerWindow returns the trailer offset and size for a v3
// object's block table: the table is the last 36*blockCount bytes of the
// stored ciphertext.
func v3BlockTableTrailerWindow(ciphertextSize int64, blockCount int) (offset, size int64, err error) {
	size = int64(blockCount) * crypto.BlockTableEntrySize
	offset = ciphertextSize - size
	if offset < 0 {
		return 0, 0, fmt.Errorf("invalid v3 trailer offset: ciphertextSize %d, trailerSize %d", ciphertextSize, size)
	}
	return offset, size, nil
}

// decodeV3BlockTableFromBuffer decodes the trailer block table out of an
// already-fetched whole-ciphertext buffer (armor-98864f12) instead of a
// separate trailer round trip. buf must be exactly the stored ciphertext the
// request's HEAD reported: the trailer window is cut from its end, and the
// decoded table must claim a data region that ends exactly where the trailer
// begins. Any disagreement — a truncated or over-long buffer, a table whose
// prefix sums do not fit the object — fails here, before the decryption loop
// starts and before any plaintext exists.
func decodeV3BlockTableFromBuffer(buf []byte, ciphertextSize int64, blockCount int) (*crypto.BlockTable, error) {
	trailerOffset, trailerSize, err := v3BlockTableTrailerWindow(ciphertextSize, blockCount)
	if err != nil {
		return nil, err
	}
	if trailerOffset < crypto.HeaderSize || trailerOffset+trailerSize != int64(len(buf)) {
		return nil, fmt.Errorf("v3 trailer window [%d,+%d] does not match the fetched ciphertext (%d bytes)", trailerOffset, trailerSize, len(buf))
	}
	// The block size is only used for per-entry validation here; each entry
	// carries its own length (same convention as fetchV3BlockTable).
	const defaultBlockSize = 65536
	if blockCount == 0 {
		// A zero-block envelope (empty object) has an empty trailer window;
		// DecodeBlockTable rejects empty input by design, so build the empty
		// table directly — the data region is empty either way.
		return crypto.NewBlockTable(defaultBlockSize, 0), nil
	}
	table, err := crypto.DecodeBlockTable(buf[trailerOffset:], defaultBlockSize, uint32(blockCount))
	if err != nil {
		return nil, fmt.Errorf("failed to decode v3 block table: %w", err)
	}
	dataEnd := int64(crypto.HeaderSize) + table.TotalCiphertextLength64()
	if dataEnd != trailerOffset {
		return nil, fmt.Errorf("v3 block table total ciphertext length (%d) does not fit the fetched object: data region ends at %d, trailer begins at %d", table.TotalCiphertextLength64(), dataEnd, trailerOffset)
	}
	return table, nil
}

// encodeV3BlockTableForTransport encodes a BlockTable for transport to the decryption goroutine.
// The transport format is the serialized block table bytes.
func encodeV3BlockTableForTransport(table *crypto.BlockTable) []byte {
	encoded, err := table.Encode()
	if err != nil {
		// This should never fail if the table was successfully decoded
		panic(fmt.Sprintf("failed to encode v3 block table for transport: %v", err))
	}
	return encoded
}

// decodeV3BlockTableFromTransport decodes a BlockTable from the transport format.
// Returns (table, true) if the data is a v3 block table, (nil, false) otherwise.
func decodeV3BlockTableFromTransport(data []byte) (*crypto.BlockTable, bool) {
	// Try to decode as a v3 block table
	// We need to determine block count from the data length
	entryCount := len(data) / crypto.BlockTableEntrySize
	if entryCount == 0 || len(data)%crypto.BlockTableEntrySize != 0 {
		// Not a valid block table format
		return nil, false
	}

	// Try to decode - use default block size for validation (will be checked per-entry)
	blockSize := 65536
	table, err := crypto.DecodeBlockTable(data, blockSize, uint32(entryCount))
	if err != nil {
		// Not a valid block table
		return nil, false
	}

	return table, true
}
