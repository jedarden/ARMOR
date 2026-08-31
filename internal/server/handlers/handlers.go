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
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
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
}

// ManifestRecorder records successful S3 write operations in the manifest
// index for fast in-memory lookup and enqueues them for async B2 persistence
// as delta files. Implementations must be safe for concurrent use.
type ManifestRecorder interface {
	// RecordPut records a successful PutObject or CompleteMultipartUpload.
	// chainEntry is the provenance chain entry to embed in the delta line.
	// May be nil if provenance is disabled for this key.
	RecordPut(bucket, key string, size int64, sha256Hex string, iv, wrappedDEK []byte, mekFingerprint string, blockSize int, contentType, etag string, chainEntry *manifest.ChainEntry, ciphertextSize int64)
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

	// multipartLocks serializes per-upload state updates. ADR-005 removes the
	// sequential-only rejection, so parts of one upload may now arrive
	// concurrently. The multipart state object (.armor/multipart/<id>.state)
	// is updated by a read-modify-write in every UploadPart/Complete; without
	// per-upload serialization a later writer would drop earlier parts'
	// HMAC/size entries. Cross-upload parallelism is unaffected — one mutex
	// per uploadID. The zero value is a usable sync.Map.
	multipartLocks sync.Map // uploadID -> *sync.Mutex

	// multipartSidecarCache caches v3 multipart sidecar data
	multipartSidecarCache *backend.MultipartSidecarCache
}

// multipartLock returns the mutex serializing state updates for one upload id.
func (h *Handlers) multipartLock(uploadID string) *sync.Mutex {
	v, _ := h.multipartLocks.LoadOrStore(uploadID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// getMultipartManager returns the multipart state manager for a given bucket.
// The manager is created on first use per bucket.
func (h *Handlers) getMultipartManager(bucket string) *backend.MultipartStateManager {
	return backend.NewMultipartStateManager(h.backend, bucket)
}

// New creates a new Handlers instance.
func New(cfg *config.Config, be backend.Backend, cache *backend.MetadataCache, footerCache *backend.FooterCache, km *keymanager.KeyManager, listCache *backend.ListCache) *Handlers {
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
	}
}

// WithProvenance adds provenance support to handlers.
func (h *Handlers) WithProvenance(p ProvenanceRecorder) {
	h.provenance = p
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
	path := r.URL.Path

	// Parse bucket and key from path
	// Path format: /bucket/key or /bucket
	path = strings.TrimPrefix(path, "/")
	parts := strings.SplitN(path, "/", 2)

	bucket := ""
	key := ""
	if len(parts) > 0 && parts[0] != "" {
		bucket = parts[0]
	}
	if len(parts) > 1 {
		key = parts[1]
		// URL decode the key (DuckDB httpfs encodes special chars like = as %3D)
		if decoded, err := url.PathUnescape(key); err == nil {
			key = decoded
		}
	}

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
		if r.URL.Query().Has("uploads") && key == "" && bucket != "" {
			h.ListMultipartUploads(w, r, bucket)
			return
		}
		// Handle ListObjectVersions (GET ?versions on bucket)
		if r.URL.Query().Has("versions") && key == "" && bucket != "" {
			h.ListObjectVersions(w, r, bucket)
			return
		}
		// Handle GetBucketLifecycleConfiguration (GET ?lifecycle on bucket)
		if r.URL.Query().Has("lifecycle") && key == "" && bucket != "" {
			h.GetBucketLifecycleConfiguration(w, r, bucket)
			return
		}
		// Handle GetObjectLockConfiguration (GET ?object-lock on bucket)
		if r.URL.Query().Has("object-lock") && key == "" && bucket != "" {
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
		} else if bucket != "" {
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
		if r.URL.Query().Has("lifecycle") && key == "" && bucket != "" {
			h.PutBucketLifecycleConfiguration(w, r, bucket)
			return
		}
		// Handle PutObjectLockConfiguration (PUT ?object-lock on bucket)
		if r.URL.Query().Has("object-lock") && key == "" && bucket != "" {
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
		} else if bucket != "" {
			h.CreateBucket(w, r, bucket)
		}
	case http.MethodHead:
		if key != "" {
			h.HeadObject(w, r, bucket, key)
		} else if bucket != "" {
			h.HeadBucket(w, r, bucket)
		}
	case http.MethodDelete:
		// Handle DeleteBucketLifecycleConfiguration (DELETE ?lifecycle on bucket)
		if r.URL.Query().Has("lifecycle") && key == "" && bucket != "" {
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
		} else if bucket != "" {
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

// PutObject handles S3 PutObject with encryption.
// For small files (<10MB), it buffers in memory.
// For larger files, it uses streaming encryption via temp files to avoid
// loading the entire file into memory.
func (h *Handlers) PutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Check Content-Length header
	contentLength := r.ContentLength
	streamingThreshold := int64(10 * 1024 * 1024) // 10MB threshold

	// Use streaming for large files or unknown size, but NOT when compression is enabled
	// Compression requires buffering the entire file anyway (to compress), so disable streaming
	// Also disable streaming when compress rules are configured (may require evaluation)
	useStreaming := !h.config.Compress && !h.config.CompressRules.HasRules() && (contentLength < 0 || contentLength > streamingThreshold)

	if useStreaming {
		h.putObjectStreaming(ctx, w, r, bucket, key)
		return
	}

	// Small file: buffer in memory
	plaintext, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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
		h.writeError(w, r, "InternalError", "Invalid wrapped DEK format from WrapDEKWithFingerprint", 500)
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

	// Extract content-type for compression rules (before compression decision)
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	contentType = config.ExtractContentType(contentType)

	// Get compression override header (highest priority)
	compressOverride := r.Header.Get("x-amz-meta-armor-compress")

	// Compress based on rules (ADR-007: compress-before-encrypt)
	// Rules support: suffix matching (.jsonl), content-type matching (application/json),
	// wildcard (*), and per-request override (x-amz-meta-armor-compress header).
	// ARMOR_COMPRESS=true is an alias for "*=zstd".
	var compressed bool
	var compressionType crypto.CompressionType
	dataToEncrypt := plaintext

	// Evaluate compression decision (rules + override)
	shouldCompress, err := config.EvaluateCompression(key, contentType, h.config.CompressRules, compressOverride)
	if err != nil {
		h.writeError(w, r, "InvalidArgument", fmt.Sprintf("Invalid compression override: %v", err), 400)
		return
	}

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

	// Upload to B2 with prefix applied
	prefixedKey := h.applyPrefix(key)
	if err := h.backend.Put(ctx, bucket, prefixedKey, bytes.NewReader(envelope), int64(len(envelope)), meta); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to upload: %v", err), 500)
		return
	}

	// Record in manifest for fast metadata lookup (async B2 persistence)
	// When manifest is enabled, provenance is embedded in delta lines.
	// When manifest is disabled, provenance uses per-object entries.
	var chainEntry *manifest.ChainEntry
	if h.manifest != nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		entryData, err := h.provenance.CreateChainEntry(ctx, key, plaintextSHAHex, "put")
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
		h.manifest.RecordPut(bucket, key, plaintextSize, hex.EncodeToString(plaintextSHA[:]), iv, wrappedDEK, mekFingerprint, h.config.BlockSize, contentType, etag, chainEntry, ciphertextSize)
	}

	// Record provenance (fallback when manifest is disabled)
	if h.manifest == nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		_ = h.provenance.RecordUpload(ctx, key, plaintextSHAHex, "put")
	}

	// Invalidate list cache entries covering this key's directory
	if h.listCache != nil {
		h.listCache.InvalidatePrefix(bucket, path.Dir(key)+"/")
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
				h.replicationQueue.Enqueue(bucket, key)
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
func (h *Handlers) putObjectStreaming(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket, key string) {
	// Phase 1: Stream to temp file and compute SHA-256
	tmpFile, err := os.CreateTemp("", "armor-upload-*.tmp")
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create temp file: %v", err), 500)
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up on exit

	// Compute SHA-256 while copying to temp file
	plaintextHash := sha256.New()
	teeReader := io.TeeReader(r.Body, plaintextHash)

	plaintextSize, err := io.Copy(tmpFile, teeReader)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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
		h.writeError(w, r, "InternalError", "Invalid wrapped DEK format from WrapDEKWithFingerprint", 500)
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

	// Phase 3: Stream encrypt via io.Pipe
	pr, pw := io.Pipe()

	// Start encryption goroutine
	encErr := make(chan error, 1)
	go func() {
		defer pw.Close()
		defer close(encErr)

		// Write header
		if _, err := pw.Write(headerBytes); err != nil {
			encErr <- fmt.Errorf("failed to write header: %w", err)
			return
		}

		if envelopeVersion == crypto.Version3 {
			// Stream encrypt with v3 and produce trailer block table
			encryptor, err := crypto.NewEncryptorWithVersion(dek, iv, h.config.BlockSize, crypto.Version3)
			if err != nil {
				encErr <- fmt.Errorf("failed to create v3 encryptor: %w", err)
				return
			}

			blockTable, err := encryptor.EncryptV3Stream(tmpFile, pw, plaintextSize, false) // Compression off per compress-rules bead
			if err != nil {
				encErr <- fmt.Errorf("v3 encryption failed: %w", err)
				return
			}

			// Write trailer block table
			trailerTable, err := blockTable.Encode()
			if err != nil {
				encErr <- fmt.Errorf("failed to encode block table: %w", err)
				return
			}

			if _, err := pw.Write(trailerTable); err != nil {
				encErr <- fmt.Errorf("failed to write trailer block table: %w", err)
				return
			}
		} else {
			// Stream encrypt with v2 (inline HMAC table)
			encryptor, err := crypto.NewEncryptor(dek, iv, h.config.BlockSize)
			if err != nil {
				encErr <- fmt.Errorf("failed to create encryptor: %w", err)
				return
			}

			hmacTable, err := encryptor.EncryptStream(tmpFile, pw, plaintextSize)
			if err != nil {
				encErr <- fmt.Errorf("encryption failed: %w", err)
				return
			}

			// Write HMAC table
			if _, err := pw.Write(hmacTable); err != nil {
				encErr <- fmt.Errorf("failed to write HMAC table: %w", err)
				return
			}
		}

		encErr <- nil
	}()

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

	// Upload to B2 with prefix applied using streaming reader
	prefixedKey := h.applyPrefix(key)
	if err := h.backend.Put(ctx, bucket, prefixedKey, pr, envelopeSize, meta); err != nil {
		tmpFile.Close()
		// Check if there was an encryption error
		select {
		case encErrVal := <-encErr:
			if encErrVal != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Encryption error: %v", encErrVal), 500)
				return
			}
		default:
		}
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to upload: %v", err), 500)
		return
	}

	// Close temp file
	tmpFile.Close()

	// Check for encryption errors
	if encErrVal := <-encErr; encErrVal != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Encryption error: %v", encErrVal), 500)
		return
	}

	// Record in manifest for fast metadata lookup (async B2 persistence)
	// When manifest is enabled, provenance is embedded in delta lines.
	// When manifest is disabled, provenance uses per-object entries.
	var chainEntry *manifest.ChainEntry
	if h.manifest != nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		entryData, err := h.provenance.CreateChainEntry(ctx, key, plaintextSHAHex, "put-streaming")
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
		h.manifest.RecordPut(bucket, key, plaintextSize, hex.EncodeToString(plaintextSHA[:]), iv, wrappedDEK, mekFingerprint, h.config.BlockSize, contentType, etag, chainEntry, ciphertextSize)
	}

	// Record provenance (fallback when manifest is disabled)
	if h.manifest == nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		_ = h.provenance.RecordUpload(ctx, key, plaintextSHAHex, "put-streaming")
	}

	// Invalidate list cache entries covering this key's directory
	if h.listCache != nil {
		h.listCache.InvalidatePrefix(bucket, path.Dir(key)+"/")
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
				h.replicationQueue.Enqueue(bucket, key)
				if h.metrics != nil {
					h.metrics.IncReplicationEnqueued("put-streaming")
				}
			} else {
				log.Printf("replication queue is nil, skipping enqueue for %s/%s", bucket, key)
			}
		}()
	}
}

// GetObject handles S3 GetObject with decryption and range support.
func (h *Handlers) GetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Apply prefix for backend operations
	prefixedKey := h.applyPrefix(key)

	// ADR-016: Try manifest first for multipart objects
	var info *backend.ObjectInfo
	manifestBody, manifestMeta, err := h.readManifest(ctx, bucket, key)
	if err == nil && manifestBody != nil {
		// Manifest exists - use it as the source of truth for metadata
		// Verify ciphertext freshness if we have a completion timestamp
		if completedAt := manifestMeta["x-amz-meta-armor-completed-at"]; completedAt != "" {
			if verifyErr := h.verifyCiphertextFreshness(ctx, bucket, manifestBody.CiphertextObject, completedAt); verifyErr != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Stale manifest: %v", verifyErr), 500)
				return
			}
		}

		// Parse ARMOR metadata from manifest
		armorMeta, ok := backend.ParseARMORMetadata(manifestMeta)
		if !ok {
			h.writeError(w, r, "InternalError", "Failed to parse ARMOR metadata from manifest", 500)
			return
		}

		// Get ciphertext object info for size check
		ciphertextInfo, ciphertextErr := h.backend.Head(ctx, bucket, manifestBody.CiphertextObject)
		if ciphertextErr != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to head ciphertext object: %v", ciphertextErr), 500)
			return
		}

		// Set info from manifest metadata
		info = &backend.ObjectInfo{
			Key:              manifestBody.CiphertextObject,
			Size:             armorMeta.PlaintextSize,
			ContentType:      armorMeta.ContentType,
			ETag:             armorMeta.ETag,
			LastModified:     ciphertextInfo.LastModified,
			Metadata:         manifestMeta,
			IsARMOREncrypted: true,
		}

		// Override prefixedKey to point to ciphertext object
		prefixedKey = manifestBody.CiphertextObject
	} else {
		// Legacy path: no manifest, get metadata from object itself
		info, err = h.backend.Head(ctx, bucket, prefixedKey)
	}

	if err != nil && info == nil {
		h.writeError(w, r, "NoSuchKey", fmt.Sprintf("Object not found: %v", err), 404)
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
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object: %v", err), 500)
			return
		}
		defer body.Close()

		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.Header().Set("Content-Type", info.ContentType)
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, info.ETag))
		w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
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
	var decryptor *crypto.Decryptor
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
		headerReader, err := h.backend.GetRange(ctx, bucket, prefixedKey, 0, crypto.HeaderSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read envelope header: %v", err), 500)
			return
		}
		defer headerReader.Close()

		headerBuf := make([]byte, crypto.HeaderSize)
		if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
			return
		}

		header, err := crypto.DecodeHeader(headerBuf)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
			return
		}

		// Create decryptor with the version from the envelope header
		decryptor, err = crypto.NewDecryptorWithVersion(dek, header.IV[:], header.BlockSize(), header.Version)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create decryptor: %v", err), 500)
			return
		}

		// For v3 single-PUT objects, we need the ciphertext size to locate the trailer block table
		// Store it in a local variable for use in the streaming path
		if header.Version == crypto.Version3 {
			armorMeta.CiphertextSize = info.Size
		}
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
			h.handleRangeRequest(w, r, bucket, key, decryptor, armorMeta, plaintextSize, info.LastModified, isMultipart)
		}
		return
	}

	// Full object download - dispatch based on version and multipart status
	if isMultipart && armorMeta.Version == 3 {
		prefixedKey := h.applyPrefix(key)
		h.handleV3MultipartGet(w, r, bucket, key, prefixedKey, decryptor, armorMeta, plaintextSize, info.LastModified)
	} else {
		h.handleFullObjectStream(w, r, bucket, key, decryptor, armorMeta, plaintextSize, info.LastModified, isMultipart, multipartPartSize, isNonUniform, cumulativePartSizes)
	}
}

// handleFullObjectStream handles full object downloads with pipelined stream decryption.
// This uses io.Pipe to decrypt blocks as they stream from Cloudflare, reducing
// time-to-first-byte and memory usage compared to buffering the entire envelope.
func (h *Handlers) handleFullObjectStream(w http.ResponseWriter, r *http.Request, bucket, key string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time, isMultipart bool, multipartPartSize int64, isNonUniform bool, cumulativePartSizes map[int]int64) {
	ctx := r.Context()

	// Apply prefix for backend operations
	prefixedKey := h.applyPrefix(key)

	blockSize := armorMeta.BlockSize
	blockCount := int(crypto.ComputeBlockCount(plaintextSize, blockSize))

	var hmacTable []byte
	var dataBody io.ReadCloser
	var streamSize int64
	var header *crypto.EnvelopeHeader

	if isMultipart {
		// Multipart object: HMAC table is in sidecar, no embedded header
		manager := backend.NewMultipartStateManager(h.backend, bucket)
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

		// Read header to determine version
		headerBuf := make([]byte, crypto.HeaderSize)
		headerReader, err := h.backend.GetRange(ctx, bucket, prefixedKey, 0, crypto.HeaderSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
			return
		}
		if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
			headerReader.Close()
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header bytes: %v", err), 500)
			return
		}
		headerReader.Close()

		header, err = crypto.DecodeHeader(headerBuf)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
			return
		}

		if header.Version == crypto.Version3 {
			// v3: fetch trailer block table and use prefix sums for block offsets
			if armorMeta.CiphertextSize == 0 {
				h.writeError(w, r, "InternalError", "v3 object missing ciphertext size", 500)
				return
			}

			blockTable, err := h.readV3BlockTable(ctx, bucket, prefixedKey, armorMeta.CiphertextSize, blockCount)
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

			// 3. Read and discard the 64-byte header (single-PUT only)
			headerBuf := make([]byte, crypto.HeaderSize)
			if _, err := io.ReadFull(dataBody, headerBuf); err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
				return
			}

			// Parse header to get plaintext SHA for verification
			header, err = crypto.DecodeHeader(headerBuf)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
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
	useCombinedDigest := isMultipart && multipartPartSize > 0 && multipartPartSize%int64(blockSize) == 0
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

		// Update digest
		if accumulator != nil {
			isLastPart := partNumber == partNumbers[len(partNumbers)-1]
			// Write the decrypted part as if it were blocks
			// The accumulator expects block-aligned writes, so we need to handle this
			// For non-uniform parts, we just write directly to wholeHash instead
			wholeHash.Write(decryptedPart)
			_ = isLastPart // Used in non-accumulator path
		} else {
			wholeHash.Write(decryptedPart)
		}

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
func (h *Handlers) handleRangeRequest(w http.ResponseWriter, r *http.Request, bucket, key string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time, isMultipart bool) {
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
		manager := backend.NewMultipartStateManager(h.backend, bucket)
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
		// Single-PUT object: check version for HMAC/block table format
		// Read header to determine version
		headerBuf := make([]byte, crypto.HeaderSize)
		headerReader, err := h.backend.GetRange(ctx, bucket, prefixedKey, 0, crypto.HeaderSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
			return
		}
		if _, err := io.ReadFull(headerReader, headerBuf); err != nil {
			headerReader.Close()
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header bytes: %v", err), 500)
			return
		}
		headerReader.Close()

		header, err := crypto.DecodeHeader(headerBuf)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
			return
		}

		if header.Version == crypto.Version3 {
			// v3: use dedicated v3 range handler
			if armorMeta.CiphertextSize == 0 {
				h.writeError(w, r, "InternalError", "v3 object missing ciphertext size for range request", 500)
				return
			}
			h.handleV3RangeRequest(w, r, bucket, key, prefixedKey, decryptor, armorMeta, plaintextSize, armorMeta.CiphertextSize, lastModified, start, end)
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

	// Fast path: serve from the in-memory manifest index when available,
	// avoiding a B2 HeadObject round-trip entirely.
	if h.manifest != nil {
		if entry, ok := h.manifest.Lookup(bucket, key); ok {
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

	// Manifest miss or disabled: fall back to a B2 HeadObject call.
	prefixedKey := h.applyPrefix(key)
	info, err := h.backend.Head(ctx, bucket, prefixedKey)
	if err != nil {
		h.writeError(w, r, "NoSuchKey", "Object not found", 404)
		return
	}

	// Determine ETag and content info based on encryption status
	var etag string
	var contentLength int64
	var contentType string
	if info.IsARMOREncrypted {
		if am, ok := backend.ParseARMORMetadata(info.Metadata); ok {
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
		h.listCache.InvalidatePrefix(bucket, path.Dir(key)+"/")
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
			h.writeError(w, r, "InternalError", "Invalid wrapped DEK format from WrapDEKWithFingerprint", 500)
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
			_ = h.provenance.RecordUpload(ctx, dstKey, armorMeta.PlaintextSHA, "copy")
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

// enrichPlaintextSizes replaces encrypted on-disk sizes with plaintext sizes for
// ARMOR-encrypted objects. B2's ListObjectsV2 response includes the encrypted
// object size; we resolve the plaintext size using the metadata cache (if warm)
// or a parallel HeadObject call per object (on cache miss). Each errgroup goroutine
// writes to a distinct slice index so no mutex is needed.
func (h *Handlers) enrichPlaintextSizes(ctx context.Context, bucket string, result *backend.ListResult) {
	eg, egCtx := errgroup.WithContext(ctx)
	for i := range result.Objects {
		i := i
		key := result.Objects[i].Key
		if am, ok := h.cache.Get(bucket, key); ok {
			if am != nil && am.PlaintextSize > 0 {
				result.Objects[i].Size = am.PlaintextSize
			}
			continue
		}
		eg.Go(func() error {
			prefixedKey := h.applyPrefix(key)
			info, err := h.backend.Head(egCtx, bucket, prefixedKey)
			if err != nil {
				return nil // non-fatal: keep encrypted size rather than failing the listing
			}
			if info.Size > 0 {
				result.Objects[i].Size = info.Size
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
		if cached, ok := h.listCache.Get(bucket, backendPrefix, delimiter, maxKeys, contToken); ok {
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
			h.listCache.Set(bucket, backendPrefix, delimiter, maxKeys, contToken, result)
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
		Name:                  bucket,
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
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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

	// Remove from manifest using original (unprefixed) keys
	if h.manifest != nil {
		for _, key := range originalKeys {
			h.manifest.RecordDelete(bucket, key)
		}
	}

	// Invalidate cache for deleted objects using original (unprefixed) keys
	for _, key := range originalKeys {
		h.cache.Delete(bucket, key)
		h.footerCache.Delete(bucket, key)
	}

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

	// If not quiet mode, include successfully deleted keys and access denied errors
	if !deleteReq.Quiet {
		for _, key := range allowedKeys {
			result.Deleted = append(result.Deleted, DeletedObject{Key: key})
		}
		// Add access denied errors for keys that failed ACL checks
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
		h.writeError(w, r, "InternalError", "Invalid wrapped DEK format from WrapDEKWithFingerprint", 500)
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
	manager := backend.NewMultipartStateManager(h.backend, bucket)

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
		Bucket:   bucket,
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
// Per-upload state updates are serialized by multipartLock: the state object is
// read-modify-written here, so without per-upload serialization a concurrent
// writer would drop an earlier part's HMAC/size entry. The lock is held across
// the backend upload too — this keeps state updates atomic. It serializes the
// parts of ONE upload (different uploads still proceed in parallel).
func (h *Handlers) UploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Serialize state updates for this one upload id.
	mu := h.multipartLock(uploadID)
	mu.Lock()
	defer mu.Unlock()

	// Parse part number
	partNumberStr := r.URL.Query().Get("partNumber")
	if partNumberStr == "" {
		h.writeError(w, r, "InvalidRequest", "Missing partNumber", 400)
		return
	}
	partNumber, err := strconv.ParseInt(partNumberStr, 10, 32)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		h.writeError(w, r, "InvalidRequest", "Invalid partNumber", 400)
		return
	}

	// Load multipart state/metadata
	// Try v3 format first (meta.json), fall back to v2 format (.state file)
	manager := backend.NewMultipartStateManager(h.backend, bucket)

	// Try v3 format
	metadata, errV3 := manager.LoadMetadataV3(ctx, uploadID)
	var state *backend.MultipartState
	var formatVersion int

	if errV3 == nil && metadata != nil {
		// V3 format loaded successfully
		formatVersion = 3
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

	// If a prior contradiction poisoned this upload (ADR-005 rule 4), every
	// further part fails the same way — the client must abort and retry.
	if state.Poisoned {
		h.writeError(w, r, "InvalidPart",
			fmt.Sprintf("This multipart upload has been invalidated and cannot be completed: %s. Abort and retry the upload from the beginning.", state.PoisonReason), 400)
		return
	}

	// Read plaintext part
	plaintext, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}
	plaintextSize := int64(len(plaintext))

	// Idempotent retry of an already-uploaded part number (ADR-005 rule 5).
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

	// V3 format: encrypt each part independently using (part n, block b) counter
	// V3 encrypts each part as an independent stream starting from block 0
	// No ordering constraints, no size pinning, no cumulative offsets
	encrypted, blockHMACsRaw, err = crypto.EncryptPartV3(dek, state.IV, uint16(partNumber), state.BlockSize, plaintext)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to encrypt part: %v", err), 500)
		return
	}

	// Upload encrypted part to B2. Prefixed for the same reason as
	// CreateMultipartUpload above — the part must be addressed by the same
	// storage key the upload was created under.
	etag, err := h.backend.UploadPart(ctx, bucket, h.applyPrefix(key), uploadID, int32(partNumber), bytes.NewReader(encrypted), int64(len(encrypted)))
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to upload part: %v", err), 500)
		return
	}

	// Split concatenated HMACs into individual HMACs for storage.
	// EncryptWithStartingCounter returns HMACs as a flat byte slice.
	blockCount := len(blockHMACsRaw) / crypto.HMACSize
	blockHMACs := make([][]byte, blockCount)
	for i := 0; i < blockCount; i++ {
		blockHMACs[i] = blockHMACsRaw[i*crypto.HMACSize : (i+1)*crypto.HMACSize]
	}

	// Compute plaintext SHA-256 for this part
	partSHA := sha256.Sum256(plaintext)
	partSHAHex := hex.EncodeToString(partSHA[:])

	if formatVersion == 3 {
		// V3 format: save part data to a separate part-<n>.json file
		// This allows concurrent UploadPart operations to proceed without touching the same file
		partData := &backend.PartDataV3{
			PartNumber:       int(partNumber),
			PlaintextLen:     plaintextSize,
			CiphertextLen:    int64(len(encrypted)),
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
// ADR-005 rule-4 contradiction error. It tells the client the upload is dead
// and must be retried — the invariant that no corrupt object is ever stored.
const multipartRetryMessage = "This upload has been invalidated; abort it and retry the multipart upload from the beginning."

// multipartMinPartSize is B2's minimum part size for multi-part objects
// (ADR-005 rule 1). Enforced at Complete for any upload with more than one
// part.
const multipartMinPartSize = int64(5 * 1024 * 1024)

// poisonUpload marks the multipart state as permanently failed (ADR-005 rule 4)
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
	manifestKey := h.applyPrefix(key) + ".armor-manifest"

	// Build manifest body (optional, for debugging)
	manifestBody := &backend.ManifestBody{
		CiphertextObject: ciphertextRef,
		UploadID:        uploadID,
		CompletedAt:     time.Now().UTC().Format(time.RFC3339),
		Metadata:        meta,
	}

	manifestJSON, err := json.Marshal(manifestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest body: %w", err)
	}

	// Add ciphertext reference to metadata if not already present
	if meta["x-amz-meta-armor-ciphertext-ref"] == "" {
		meta["x-amz-meta-armor-ciphertext-ref"] = ciphertextRef
	}

	// Add completion timestamp to metadata
	if meta["x-amz-meta-armor-completed-at"] == "" {
		meta["x-amz-meta-armor-completed-at"] = manifestBody.CompletedAt
	}

	// Write manifest object with metadata
	manifestMeta := map[string]string{
		"Content-Type": "application/x-armor-manifest+json",
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
	manifestKey := h.applyPrefix(key) + ".armor-manifest"

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
			Metadata:        info.Metadata,
		}
	}

	return &manifestBody, info.Metadata, nil
}

// verifyCiphertextFreshness checks if the ciphertext object referenced by the manifest
// is still fresh (hasn't been overwritten by a newer upload).
func (h *Handlers) verifyCiphertextFreshness(ctx context.Context, bucket, ciphertextRef string, completedAt string) error {
	// Parse completion timestamp
	createdAt, err := time.Parse(time.RFC3339, completedAt)
	if err != nil {
		// If we can't parse the timestamp, skip verification
		return nil
	}

	// Get ciphertext object info
	info, err := h.backend.Head(ctx, bucket, ciphertextRef)
	if err != nil {
		return fmt.Errorf("failed to head ciphertext object: %w", err)
	}

	// Check if ciphertext was created after the manifest completion
	// Truncate to seconds since B2 timestamps have second precision
	ciphertextTime := info.LastModified.UTC().Truncate(time.Second)
	manifestTime := createdAt.Truncate(time.Second)

	if ciphertextTime.Before(manifestTime) {
		return fmt.Errorf("ciphertext object is stale (created %v, manifest completed %v)", ciphertextTime, manifestTime)
	}

	return nil
}

// CompleteMultipartUpload handles S3 CompleteMultipartUpload.
// It assembles the parts in B2 and stores the HMAC table as a sidecar.
func (h *Handlers) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state/metadata
	// Try v3 format first (meta.json), fall back to v2 format (.state file)
	manager := backend.NewMultipartStateManager(h.backend, bucket)

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

	// ADR-005 rule 4: if a prior UploadPart contradicted the uniform-part-size
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
		// sizes are intentionally unconstrained.  Preserve part 1's size only
		// for legacy metadata consumers.
		if partOne, ok := state.PartSizes[1]; ok {
			state.PartSize = partOne
		}
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
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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

	// Validate that all parts in the complete request exist
	for _, p := range completeReq.Parts {
		if _, ok := state.PartSizes[p.PartNumber]; !ok {
			h.writeError(w, r, "InvalidPart",
				fmt.Sprintf("Part %d was not uploaded and cannot be completed. %s", p.PartNumber, multipartRetryMessage), 400)
			return
		}
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
	// Parts arrive out of order (ADR-005), so the per-part digests cannot be
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

	// Build ARMOR metadata and update via CopyObject
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

	// Add multipart flag to indicate HMAC table is external
	meta["x-amz-meta-armor-multipart"] = "true"

	// ADR-016: Write manifest object instead of using CopyObject
	// This avoids the 5GB limit and race condition window of CopyObject
	ciphertextRef := h.applyPrefix(key)
	if err := h.writeManifest(ctx, bucket, key, meta, uploadID, etag, ciphertextRef); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to write manifest after completion: %v", err), 500)
		return
	}

	// Clean up multipart state
	manager.DeleteState(ctx, uploadID)

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
		h.manifest.RecordPut(bucket, key, totalPlaintextSize, plaintextSHAHex, state.IV, state.WrappedDEK, state.MEKFingerprint, state.BlockSize, state.ContentType, etag, chainEntry, totalCiphertextSize)
	}

	// Record provenance (fallback when manifest is disabled)
	if h.manifest == nil && h.provenance != nil && h.provenance.ShouldRecord(key) {
		_ = h.provenance.RecordUpload(ctx, key, plaintextSHAHex, "multipart")
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

	result := CompleteMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: fmt.Sprintf("/%s/%s", bucket, key),
		Bucket:   bucket,
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
				h.replicationQueue.Enqueue(bucket, key)
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
	manager := backend.NewMultipartStateManager(h.backend, bucket)

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

	w.WriteHeader(http.StatusNoContent)
}

// ListParts handles S3 ListParts operation.
// It forwards to B2 and adjusts part sizes to plaintext sizes.
func (h *Handlers) ListParts(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state/metadata to get plaintext sizes
	// Try v3 format first (meta.json), fall back to v2 format (.state file)
	manager := backend.NewMultipartStateManager(h.backend, bucket)

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
		Bucket:               bucket,
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
		Bucket:             bucket,
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
		Name:                bucket,
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
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
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
	// Log structured S3 error event before writing response
	if h.logger != nil {
		h.logS3Error(r, code, message, statusCode)
	}

	// Extract request ID from middleware context
	requestID := middleware.GetRequestID(r.Context())

	// Build resource path from request URL
	resource := r.URL.Path

	// Set headers
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(statusCode)

	// Build XML response
	var codeBuf, msgBuf, ridBuf, resBuf bytes.Buffer
	xml.EscapeText(&codeBuf, []byte(code))
	xml.EscapeText(&msgBuf, []byte(message))
	xml.EscapeText(&ridBuf, []byte(requestID))
	xml.EscapeText(&resBuf, []byte(resource))

	if requestID != "" {
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n  <RequestId>%s</RequestId>\n  <Resource>%s</Resource>\n</Error>",
			codeBuf.String(), msgBuf.String(), ridBuf.String(), resBuf.String())
	} else {
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+"\n<Error>\n  <Code>%s</Code>\n  <Message>%s</Message>\n  <Resource>%s</Resource>\n</Error>",
			codeBuf.String(), msgBuf.String(), resBuf.String())
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

	// Increment error counter (ADR-008)
	if h.metrics != nil {
		h.metrics.IncErrors(code, operation)
	}
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

// readV3BlockTable fetches the trailer block table for a v3 object.
// The trailer block table is located at ciphertext_length - 36*blockCount.
// Returns a decoded BlockTable.
func (h *Handlers) readV3BlockTable(ctx context.Context, bucket, key string, ciphertextSize int64, blockCount int) (*crypto.BlockTable, error) {
	// Calculate trailer offset and size
	// Trailer is at the end: [header][blocks][trailer table]
	// Trailer size = blockCount * BlockTableEntrySize (36 bytes per entry)
	trailerSize := int64(blockCount) * crypto.BlockTableEntrySize
	trailerOffset := ciphertextSize - trailerSize

	if trailerOffset < 0 {
		return nil, fmt.Errorf("invalid v3 trailer offset: ciphertextSize %d, trailerSize %d", ciphertextSize, trailerSize)
	}

	// Fetch the trailer block table
	trailerBody, err := h.backend.GetRange(ctx, bucket, key, trailerOffset, trailerSize)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch v3 trailer block table: %w", err)
	}
	defer trailerBody.Close()

	trailerData, err := io.ReadAll(trailerBody)
	if err != nil {
		return nil, fmt.Errorf("failed to read v3 trailer block table: %w", err)
	}

	// Decode the block table
	// We need the block size for validation - use default for now (will be validated against entry data)
	blockSize := 65536 // Default block size, validated per-entry
	table, err := crypto.DecodeBlockTable(trailerData, blockSize, uint32(blockCount))
	if err != nil {
		return nil, fmt.Errorf("failed to decode v3 block table: %w", err)
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
