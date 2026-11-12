// Package handlers implements S3 operation handlers for ARMOR.
package handlers

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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

// reservedNamespaceDeniedMsg is the refusal every client-facing operation
// returns for keys under ARMOR's reserved .armor/ namespace. DeleteObjects
// reuses it for the per-key <Error> entries of a bulk request, since its
// keys travel in the request body where the HandleRoot guard cannot see
// them.
const reservedNamespaceDeniedMsg = "Access to .armor/ reserved namespace is denied"

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
		h.writeError(w, r, "AccessDenied", reservedNamespaceDeniedMsg, 403)
		return
	}

	// Object Lock is outside ARMOR's supported S3 contract. Reject its request
	// headers and subresources before any handler can silently accept them or
	// pass their XML through to the backend.
	if h.rejectUnsupportedObjectLock(w, r) {
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
		// Handle GetObjectTagging (GET ?tagging on object)
		if r.URL.Query().Has("tagging") && key != "" {
			h.GetObjectTagging(w, r, bucket, key)
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
		// Handle PutObjectTagging (PUT ?tagging on object). This must be
		// routed before PutObject: the tagging body is XML, not object
		// data, and the generic PUT would store it as the object's bytes.
		if r.URL.Query().Has("tagging") && key != "" {
			h.PutObjectTagging(w, r, bucket, key)
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
		// Handle DeleteObjectTagging (DELETE ?tagging on object). This
		// must be routed before DeleteObject, which would delete the
		// object itself.
		if r.URL.Query().Has("tagging") && key != "" {
			h.DeleteObjectTagging(w, r, bucket, key)
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
func decodeV3BlockTableFromBuffer(buf []byte, ciphertextSize int64, blockCount int, compressed bool, blockSize int) (*crypto.BlockTable, error) {
	if compressed {
		if int64(len(buf)) != ciphertextSize {
			return nil, fmt.Errorf("fetched ciphertext length %d does not match stored length %d", len(buf), ciphertextSize)
		}
		table, err := crypto.DecodeV3CompressedBlockTable(buf, blockSize, uint32(blockCount))
		if err != nil {
			return nil, fmt.Errorf("failed to decode compressed v3 block table: %w", err)
		}
		return table, nil
	}

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
