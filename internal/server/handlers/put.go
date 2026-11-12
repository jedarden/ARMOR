package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
	"github.com/jedarden/armor/internal/provenance"
	"github.com/jedarden/armor/internal/server/middleware"
)

type putEncryptionContext struct {
	dek            []byte
	iv             []byte
	wrappedDEK     []byte
	mekFingerprint string
	keyID          string
	version        uint8
	headerBytes    []byte
}

type putSetupError string

func (e putSetupError) Error() string { return string(e) }

func (h *Handlers) preparePutEncryption(key string) (*putEncryptionContext, error) {
	mek, keyID, err := h.keyManager.GetMEK(key)
	if err != nil {
		return nil, putSetupError(fmt.Sprintf("Failed to get encryption key: %v", err))
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		return nil, putSetupError(fmt.Sprintf("Failed to generate DEK: %v", err))
	}
	iv, err := crypto.GenerateIV()
	if err != nil {
		return nil, putSetupError(fmt.Sprintf("Failed to generate IV: %v", err))
	}

	wrappedDEKStr, err := crypto.WrapDEKWithFingerprint(mek, dek)
	if err != nil {
		return nil, putSetupError(fmt.Sprintf("Failed to wrap DEK: %v", err))
	}
	parts := strings.SplitN(wrappedDEKStr, ":", 3)
	if len(parts) != 3 || parts[0] != "v2" {
		return nil, putSetupError("wrapped DEK format validation failed")
	}
	wrappedDEK, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, putSetupError(fmt.Sprintf("Failed to decode wrapped DEK: %v", err))
	}

	return &putEncryptionContext{
		dek:            dek,
		iv:             iv,
		wrappedDEK:     wrappedDEK,
		mekFingerprint: parts[1],
		keyID:          keyID,
	}, nil
}

func (h *Handlers) setPutEnvelopeHeader(envelope *putEncryptionContext, plaintextSize int64, plaintextSHA [32]byte, compressed bool, compressionType crypto.CompressionType) error {
	version := crypto.Version2
	if h.config.FormatWriteVersion == 3 {
		version = crypto.Version3
	}
	header, err := crypto.NewEnvelopeHeaderWithVersion(envelope.iv, plaintextSize, h.config.BlockSize, plaintextSHA, uint8(version))
	if err != nil {
		return putSetupError(fmt.Sprintf("Failed to create header: %v", err))
	}
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
		return putSetupError(fmt.Sprintf("Failed to encode header: %v", err))
	}

	envelope.version = uint8(version)
	envelope.headerBytes = headerBytes
	return nil
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

	// Parse and validate the tag set (x-amz-tagging) before any body work:
	// the streaming path spools the whole body before it would otherwise
	// notice an invalid header, and an invalid tag set must fail the PUT
	// without storing the object. taggingEncoded == "" (header absent)
	// writes an untagged object.
	taggingEncoded, err := taggingHeaderEncoded(r)
	if err != nil {
		h.writeError(w, r, "InvalidArgument", fmt.Sprintf("Invalid x-amz-tagging header: %v", err), 400)
		return
	}

	// Resolve the per-object compression decision (x-amz-meta-armor-compress
	// override > compress rules > global Compress flag) BEFORE choosing the
	// upload path, so an invalid override fails the PUT on both paths with
	// the same InvalidArgument error (the streaming path cannot validate the
	// header once the body is spooled). The decision applies only on the
	// buffered path; it never influences path choice — see useStreaming.
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

	// Use streaming for large files or unknown size — ALWAYS, even when the
	// object carries a compression decision. Whole-object compression requires
	// the entire body in memory, so it is bounded to the buffered path (a
	// declared Content-Length at or under streamingThreshold; net/http caps
	// body reads at the declared length). Letting shouldCompress defeat this
	// check made a single client-supplied header a memory-DoS route: any
	// authenticated PUT with x-amz-meta-armor-compress: true pulled a
	// multi-GiB body through io.ReadAll. Bodies above the threshold — or
	// without a length — stream and store UNCOMPRESSED (the streaming path
	// cannot compress; this is the same fallback an incompressible payload
	// already gets). Deliberate routing change, documented in
	// docs/configuration.md: compressed GET round-trips for whole-object
	// compressed envelopes are independently broken (the reader derives block
	// layout from the original plaintext size), so large bodies never belonged
	// on the buffered compression path.
	useStreaming := contentLength < 0 || contentLength > streamingThreshold

	if useStreaming {
		h.putObjectStreaming(ctx, w, r, bucket, key, createOnly, appendOnly, explicitCondition, taggingEncoded)
		return
	}

	// Small file: buffer in memory
	plaintext, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	plaintextSize := int64(len(plaintext))

	putEncryption, err := h.preparePutEncryption(key)
	if err != nil {
		h.writeError(w, r, "InternalError", err.Error(), 500)
		return
	}
	dek := putEncryption.dek
	iv := putEncryption.iv
	wrappedDEK := putEncryption.wrappedDEK
	mekFingerprint := putEncryption.mekFingerprint
	keyID := putEncryption.keyID

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

	if err := h.setPutEnvelopeHeader(putEncryption, plaintextSize, plaintextSHA, compressed, compressionType); err != nil {
		h.writeError(w, r, "InternalError", err.Error(), 500)
		return
	}
	envelopeVersion := putEncryption.version
	headerBytes := putEncryption.headerBytes

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
	setStoredTagging(meta, taggingEncoded)

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
// create-only decision, which createOnly already carries. taggingEncoded is
// the request's validated tag set ("" writes an untagged object), parsed in
// PutObject before the body is spooled.
func (h *Handlers) putObjectStreaming(ctx context.Context, w http.ResponseWriter, r *http.Request, bucket, key string, createOnly, appendOnly, explicitCondition bool, taggingEncoded string) {
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

	putEncryption, err := h.preparePutEncryption(key)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", err.Error(), 500)
		return
	}
	dek := putEncryption.dek
	iv := putEncryption.iv
	wrappedDEK := putEncryption.wrappedDEK
	mekFingerprint := putEncryption.mekFingerprint
	keyID := putEncryption.keyID

	if err := h.setPutEnvelopeHeader(putEncryption, plaintextSize, plaintextSHA, false, crypto.CompressionNone); err != nil {
		tmpFile.Close()
		h.writeError(w, r, "InternalError", err.Error(), 500)
		return
	}
	envelopeVersion := putEncryption.version
	headerBytes := putEncryption.headerBytes

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
	setStoredTagging(meta, taggingEncoded)

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
