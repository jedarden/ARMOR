// Package handlers implements S3 operation handlers for ARMOR.
package handlers

import (
	"bytes"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/keymanager"
)

// ProvenanceRecorder records uploads in the provenance chain.
type ProvenanceRecorder interface {
	RecordUpload(ctx context.Context, objectKey, plaintextSHA256, operation string) error
	ShouldRecord(key string) bool
}

// Handlers contains all S3 operation handlers.
type Handlers struct {
	config      *config.Config
	backend     backend.Backend
	cache       *backend.MetadataCache
	footerCache *backend.FooterCache
	keyManager  *keymanager.KeyManager
	provenance  ProvenanceRecorder
}

// New creates a new Handlers instance.
func New(cfg *config.Config, be backend.Backend, cache *backend.MetadataCache, footerCache *backend.FooterCache, km *keymanager.KeyManager) *Handlers {
	return &Handlers{
		config:      cfg,
		backend:     be,
		cache:       cache,
		footerCache: footerCache,
		keyManager:  km,
		provenance: nil,
	}
}

// WithProvenance adds provenance support to handlers.
func (h *Handlers) WithProvenance(p ProvenanceRecorder) {
	h.provenance = p
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
			// Check for query parameters
			if r.URL.Query().Get("list-type") != "" || r.URL.Query().Get("prefix") != "" {
				h.ListObjectsV2(w, r, bucket)
			} else {
				h.HeadBucket(w, r, bucket)
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
			h.writeError(w, "InvalidRequest", "Unsupported POST operation", 400)
		}
	default:
		h.writeError(w, "MethodNotAllowed", fmt.Sprintf("Method %s not allowed", r.Method), 405)
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

	// Use streaming for large files or unknown size
	useStreaming := contentLength < 0 || contentLength > streamingThreshold

	if useStreaming {
		h.putObjectStreaming(ctx, w, r, bucket, key)
		return
	}

	// Small file: buffer in memory
	plaintext, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	plaintextSize := int64(len(plaintext))

	// Get the appropriate MEK for this object key
	mek, keyID, err := h.keyManager.GetMEK(key)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get encryption key: %v", err), 500)
		return
	}

	// Generate DEK and IV
	dek, err := crypto.GenerateDEK()
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to generate DEK: %v", err), 500)
		return
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to generate IV: %v", err), 500)
		return
	}

	// Wrap DEK with MEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to wrap DEK: %v", err), 500)
		return
	}

	// Compute plaintext SHA-256
	plaintextSHA := crypto.ComputePlaintextSHA256(plaintext)

	// Create envelope header
	header, err := crypto.NewEnvelopeHeader(iv, plaintextSize, h.config.BlockSize, plaintextSHA)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create header: %v", err), 500)
		return
	}

	headerBytes, err := header.Encode()
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to encode header: %v", err), 500)
		return
	}

	// Encrypt data
	encryptor, err := crypto.NewEncryptor(dek, iv, h.config.BlockSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create encryptor: %v", err), 500)
		return
	}

	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to encrypt: %v", err), 500)
		return
	}

	// Build envelope: header + encrypted blocks + HMAC table
	envelopeSize := int64(len(headerBytes)) + int64(len(encrypted)) + int64(len(hmacTable))
	envelope := make([]byte, 0, envelopeSize)
	envelope = append(envelope, headerBytes...)
	envelope = append(envelope, encrypted...)
	envelope = append(envelope, hmacTable...)

	// Compute plaintext ETag (MD5)
	etag := backend.ComputeETag(plaintext)

	// Build metadata
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	meta := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     h.config.BlockSize,
		PlaintextSize: plaintextSize,
		ContentType:   contentType,
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
		ETag:          etag,
		KeyID:         keyID,
	}).ToMetadata()

	// Upload to B2
	if err := h.backend.Put(ctx, bucket, key, bytes.NewReader(envelope), int64(len(envelope)), meta); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to upload: %v", err), 500)
		return
	}

	// Record provenance
	if h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		_ = h.provenance.RecordUpload(ctx, key, plaintextSHAHex, "put")
	}

	// Return ETag
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.WriteHeader(http.StatusOK)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create temp file: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	// Get the computed SHA-256
	var plaintextSHA [32]byte
	copy(plaintextSHA[:], plaintextHash.Sum(nil))

	// Seek back to beginning of temp file for reading
	if _, err := tmpFile.Seek(0, 0); err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to seek temp file: %v", err), 500)
		return
	}

	// Phase 2: Get encryption keys
	mek, keyID, err := h.keyManager.GetMEK(key)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get encryption key: %v", err), 500)
		return
	}

	dek, err := crypto.GenerateDEK()
	if err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to generate DEK: %v", err), 500)
		return
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to generate IV: %v", err), 500)
		return
	}

	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to wrap DEK: %v", err), 500)
		return
	}

	// Create envelope header
	header, err := crypto.NewEnvelopeHeader(iv, plaintextSize, h.config.BlockSize, plaintextSHA)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create header: %v", err), 500)
		return
	}

	headerBytes, err := header.Encode()
	if err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to encode header: %v", err), 500)
		return
	}

	// Create encryptor
	encryptor, err := crypto.NewEncryptor(dek, iv, h.config.BlockSize)
	if err != nil {
		tmpFile.Close()
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create encryptor: %v", err), 500)
		return
	}

	// Calculate envelope size
	blockCount := crypto.ComputeBlockCount(plaintextSize, h.config.BlockSize)
	hmacTableSize := int64(blockCount) * crypto.HMACSize
	envelopeSize := int64(len(headerBytes)) + plaintextSize + hmacTableSize

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

		// Stream encrypt the plaintext
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

		encErr <- nil
	}()

	// Build metadata
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Compute ETag while we have the temp file
	// We need to read the file again for MD5, but we already have SHA-256
	// Use SHA-256 truncated to 16 bytes as ETag for streaming (non-standard but works)
	etag := hex.EncodeToString(plaintextSHA[:16])

	meta := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     h.config.BlockSize,
		PlaintextSize: plaintextSize,
		ContentType:   contentType,
		IV:            iv,
		WrappedDEK:    wrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
		ETag:          etag,
		KeyID:         keyID,
	}).ToMetadata()

	// Upload to B2 using streaming reader
	if err := h.backend.Put(ctx, bucket, key, pr, envelopeSize, meta); err != nil {
		tmpFile.Close()
		// Check if there was an encryption error
		select {
		case encErrVal := <-encErr:
			if encErrVal != nil {
				h.writeError(w, "InternalError", fmt.Sprintf("Encryption error: %v", encErrVal), 500)
				return
			}
		default:
		}
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to upload: %v", err), 500)
		return
	}

	// Close temp file
	tmpFile.Close()

	// Check for encryption errors
	if encErrVal := <-encErr; encErrVal != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Encryption error: %v", encErrVal), 500)
		return
	}

	// Record provenance
	if h.provenance != nil && h.provenance.ShouldRecord(key) {
		plaintextSHAHex := hex.EncodeToString(plaintextSHA[:])
		_ = h.provenance.RecordUpload(ctx, key, plaintextSHAHex, "put-streaming")
	}

	// Return ETag
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
	w.Header().Set("X-Armor-Streaming", "true")
	w.WriteHeader(http.StatusOK)
}

// GetObject handles S3 GetObject with decryption and range support.
func (h *Handlers) GetObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Get metadata first
	info, err := h.backend.Head(ctx, bucket, key)
	if err != nil {
		h.writeError(w, "NoSuchKey", fmt.Sprintf("Object not found: %v", err), 404)
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
				h.writeError(w, "PreconditionFailed", "Precondition failed", status)
			}
			return
		}

		// Passthrough for non-ARMOR objects
		body, _, err := h.backend.Get(ctx, bucket, key)
		if err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Failed to get object: %v", err), 500)
			return
		}
		defer body.Close()

		w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
		w.Header().Set("Content-Type", info.ContentType)
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, info.ETag))
		w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, body)
		return
	}

	// Parse ARMOR metadata
	armorMeta, ok := backend.ParseARMORMetadata(info.Metadata)
	if !ok {
		h.writeError(w, "InternalError", "Failed to parse ARMOR metadata", 500)
		return
	}

	// Get the MEK for this object using the key ID from metadata
	mek, err := h.keyManager.GetMEKByID(armorMeta.KeyID)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get decryption key: %v", err), 500)
		return
	}

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(mek, armorMeta.WrappedDEK)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to unwrap DEK: %v", err), 500)
		return
	}

	// Create decryptor
	decryptor, err := crypto.NewDecryptor(dek, armorMeta.IV, armorMeta.BlockSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create decryptor: %v", err), 500)
		return
	}

	plaintextSize := armorMeta.PlaintextSize

	// Check conditional request headers
	if status := checkConditionalRequest(r, armorMeta.ETag, info.LastModified); status != 0 {
		if status == http.StatusNotModified {
			// 304 Not Modified - set headers but no body
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
			w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
			w.WriteHeader(status)
		} else {
			// 412 Precondition Failed
			h.writeError(w, "PreconditionFailed", "Precondition failed", status)
		}
		return
	}

	// Check for range request
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		h.handleRangeRequest(w, r, bucket, key, decryptor, armorMeta, plaintextSize)
		return
	}

	// Full object download with pipelined stream decryption
	h.handleFullObjectStream(w, r, bucket, key, decryptor, armorMeta, plaintextSize)
}

// handleFullObjectStream handles full object downloads with pipelined stream decryption.
// This uses io.Pipe to decrypt blocks as they stream from Cloudflare, reducing
// time-to-first-byte and memory usage compared to buffering the entire envelope.
func (h *Handlers) handleFullObjectStream(w http.ResponseWriter, r *http.Request, bucket, key string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64) {
	ctx := r.Context()

	blockSize := armorMeta.BlockSize
	blockCount := int(crypto.ComputeBlockCount(plaintextSize, blockSize))

	// Calculate offsets
	hmacTableOffset := crypto.HeaderSize + plaintextSize
	hmacTableSize := int64(blockCount) * crypto.HMACSize
	dataSize := plaintextSize

	// 1. Prefetch HMAC table (small range read)
	hmacBody, err := h.backend.GetRange(ctx, bucket, key, hmacTableOffset, hmacTableSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to prefetch HMAC table: %v", err), 500)
		return
	}
	hmacTable, err := io.ReadAll(hmacBody)
	hmacBody.Close()
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read HMAC table: %v", err), 500)
		return
	}

	// 2. Start streaming data from Cloudflare (header + encrypted blocks, stop before HMAC)
	streamSize := crypto.HeaderSize + dataSize
	dataBody, err := h.backend.GetRange(ctx, bucket, key, 0, streamSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get object stream: %v", err), 500)
		return
	}
	defer dataBody.Close()

	// 3. Read and discard the 64-byte header
	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(dataBody, headerBuf); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
		return
	}

	// Parse header to get plaintext SHA for verification
	header, err := crypto.DecodeHeader(headerBuf)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
		return
	}

	// 4. Set response headers before streaming
	w.Header().Set("Content-Length", strconv.FormatInt(plaintextSize, 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("X-Armor-Stream", "pipelined")
	w.WriteHeader(http.StatusOK)

	// 5. Stream decrypt using io.Pipe
	pr, pw := io.Pipe()

	// Start decryption goroutine
	go func() {
		defer pw.Close()

		plaintextHash := sha256.New()
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

			// Decrypt block (need to use CTR stream)
			decrypted := make([]byte, n)
			ctr := makeCounter(armorMeta.IV, uint32(blockIndex))
			stream := cipher.NewCTR(decryptor.CipherBlock(), ctr)
			stream.XORKeyStream(decrypted, encryptedBuf)

			// Update plaintext hash for verification
			plaintextHash.Write(decrypted)

			// Write plaintext to pipe
			if _, err := pw.Write(decrypted); err != nil {
				pw.CloseWithError(fmt.Errorf("write error at block %d: %w", blockIndex, err))
				return
			}
		}

		// Verify plaintext SHA-256
		computedSHA := plaintextHash.Sum(nil)
		if !bytes.Equal(computedSHA, header.PlaintextSHA[:]) {
			pw.CloseWithError(fmt.Errorf("plaintext SHA-256 mismatch"))
			return
		}
	}()

	// Stream plaintext to client
	io.Copy(w, pr)
}

// makeCounter creates a 16-byte counter value from the IV and block index.
func makeCounter(iv []byte, blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], iv[0:12])
	binary.BigEndian.PutUint32(counter[12:16], blockIndex)
	return counter
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// handleRangeRequest handles range read requests.
func (h *Handlers) handleRangeRequest(w http.ResponseWriter, r *http.Request, bucket, key string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64) {
	ctx := r.Context()

	// Parse range header (bytes=start-end)
	rangeHeader := r.Header.Get("Range")
	start, end, err := parseRangeHeader(rangeHeader, plaintextSize)
	if err != nil {
		h.writeError(w, "InvalidRange", fmt.Sprintf("Invalid range: %v", err), 400)
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

	// Translate range to encrypted blocks
	translation, err := crypto.TranslateRange(start, end, plaintextSize, armorMeta.BlockSize, crypto.HeaderSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to translate range: %v", err), 500)
		return
	}

	// Fetch encrypted blocks and HMAC table in parallel using errgroup.
	// This cuts range-read latency nearly in half for cache misses.
	var encrypted, hmacTable []byte

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		encryptedBody, err := h.backend.GetRange(gctx, bucket, key, translation.DataOffset, translation.DataLength)
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
		hmacBody, err := h.backend.GetRange(gctx, bucket, key, translation.HMACOffset, translation.HMACLength)
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
		h.writeError(w, "InternalError", err.Error(), 500)
		return
	}

	// Decrypt range
	plaintext, err := decryptor.DecryptRange(encrypted, hmacTable, start, end, plaintextSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to decrypt range: %v", err), 500)
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

	if start < 0 || start >= totalSize || end < start || end >= totalSize {
		return 0, 0, fmt.Errorf("range out of bounds")
	}

	return start, end, nil
}

// HeadObject handles S3 HeadObject.
func (h *Handlers) HeadObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	info, err := h.backend.Head(ctx, bucket, key)
	if err != nil {
		h.writeError(w, "NoSuchKey", "Object not found", 404)
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
			h.writeError(w, "PreconditionFailed", "Precondition failed", status)
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

	if err := h.backend.Delete(ctx, bucket, key); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to delete: %v", err), 500)
		return
	}

	// Invalidate cache
	h.cache.Delete(bucket, key)

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
		h.writeError(w, "InvalidRequest", "Missing x-amz-copy-source header", 400)
		return
	}

	// Parse source bucket and key
	// Format: /bucket/key or bucket/key
	srcBucket, srcKey := parseCopySource(copySource)
	if srcBucket == "" || srcKey == "" {
		h.writeError(w, "InvalidCopySource", "Invalid copy source format", 400)
		return
	}

	// Get source object metadata
	srcInfo, err := h.backend.Head(ctx, srcBucket, srcKey)
	if err != nil {
		h.writeError(w, "NoSuchKey", fmt.Sprintf("Source object not found: %v", err), 404)
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
			h.writeError(w, "InternalError", "Failed to parse ARMOR metadata", 500)
			return
		}

		// Get the source MEK using the key ID from metadata
		srcMEK, err := h.keyManager.GetMEKByID(armorMeta.KeyID)
		if err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Failed to get source decryption key: %v", err), 500)
			return
		}

		// Unwrap DEK with source MEK
		dek, err := crypto.UnwrapDEK(srcMEK, armorMeta.WrappedDEK)
		if err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Failed to unwrap DEK: %v", err), 500)
			return
		}

		// Get the destination MEK for the target key
		dstMEK, dstKeyID, err := h.keyManager.GetMEK(dstKey)
		if err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Failed to get destination encryption key: %v", err), 500)
			return
		}

		// Re-wrap DEK with destination MEK (handles key rotation and cross-prefix copy)
		wrappedDEK, err := crypto.WrapDEK(dstMEK, dek)
		if err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Failed to re-wrap DEK: %v", err), 500)
			return
		}

		// Build new metadata with re-wrapped DEK and destination key ID
		newMeta := (&backend.ARMORMetadata{
			Version:       armorMeta.Version,
			BlockSize:     armorMeta.BlockSize,
			PlaintextSize: armorMeta.PlaintextSize,
			ContentType:   armorMeta.ContentType,
			IV:            armorMeta.IV,
			WrappedDEK:    wrappedDEK,
			PlaintextSHA:  armorMeta.PlaintextSHA,
			ETag:          armorMeta.ETag,
			KeyID:         dstKeyID,
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
		if err := h.backend.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, newMeta, true); err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Copy failed: %v", err), 500)
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
			LastModified: time.Now().UTC().Format(http.TimeFormat),
			ETag:         fmt.Sprintf(`"%s"`, armorMeta.ETag),
		}

		output, err := xml.Marshal(result)
		if err != nil {
			h.writeError(w, "InternalError", "Failed to marshal response", 500)
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

	if err := h.backend.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey, meta, replaceMetadata); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Copy failed: %v", err), 500)
		return
	}

	// Get the destination object info for ETag
	dstInfo, err := h.backend.Head(ctx, dstBucket, dstKey)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to get destination info", 500)
		return
	}

	// Return success response
	result := CopyObjectResult{
		LastModified: time.Now().UTC().Format(http.TimeFormat),
		ETag:         fmt.Sprintf(`"%s"`, dstInfo.ETag),
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
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

	result, err := h.backend.List(ctx, bucket, prefix, delimiter, contToken, maxKeys)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to list: %v", err), 500)
		return
	}

	// Build XML response
	type Contents struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
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
		CommonPrefixes        []string `xml:"CommonPrefixes>Prefix,omitempty"`
	}

	resp := ListBucketResult{
		Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:        bucket,
		Prefix:      prefix,
		Delimiter:   delimiter,
		MaxKeys:     maxKeys,
		IsTruncated: result.IsTruncated,
		NextContinuationToken: result.NextToken,
	}

	for _, obj := range result.Objects {
		resp.Contents = append(resp.Contents, Contents{
			Key:          obj.Key,
			LastModified: obj.LastModified.UTC().Format(http.TimeFormat),
			ETag:         fmt.Sprintf(`"%s"`, obj.ETag),
			Size:         obj.Size,
			StorageClass: "STANDARD",
		})
	}

	resp.CommonPrefixes = append(resp.CommonPrefixes, result.CommonPrefixes...)

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
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
		h.writeError(w, "NoSuchBucket", fmt.Sprintf("Bucket not found: %v", err), 404)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreateBucket handles S3 CreateBucket.
func (h *Handlers) CreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.CreateBucket(ctx, bucket); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create bucket: %v", err), 500)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// DeleteBucket handles S3 DeleteBucket.
func (h *Handlers) DeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.DeleteBucket(ctx, bucket); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to delete bucket: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DeleteObjects handles S3 DeleteObjects (bulk delete).
// The request body contains XML with a list of objects to delete.
func (h *Handlers) DeleteObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	if err := xml.Unmarshal(body, &deleteReq); err != nil {
		h.writeError(w, "MalformedXML", fmt.Sprintf("Failed to parse XML: %v", err), 400)
		return
	}

	if len(deleteReq.Objects) == 0 {
		h.writeError(w, "MalformedXML", "No objects specified for deletion", 400)
		return
	}

	// Extract keys
	keys := make([]string, len(deleteReq.Objects))
	for i, obj := range deleteReq.Objects {
		keys[i] = obj.Key
	}

	// Perform bulk delete
	if err := h.backend.DeleteObjects(ctx, bucket, keys); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("DeleteObjects failed: %v", err), 500)
		return
	}

	// Invalidate cache for deleted objects
	for _, key := range keys {
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

	// If not quiet mode, include all deleted keys
	if !deleteReq.Quiet {
		for _, key := range keys {
			result.Deleted = append(result.Deleted, DeletedObject{Key: key})
		}
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to list buckets: %v", err), 500)
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
			CreationDate: b.CreationDate.UTC().Format(time.RFC3339),
		})
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
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

	// Get the appropriate MEK for this object key
	mek, keyID, err := h.keyManager.GetMEK(key)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get encryption key: %v", err), 500)
		return
	}

	// Generate DEK and IV for this upload
	dek, err := crypto.GenerateDEK()
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to generate DEK: %v", err), 500)
		return
	}

	iv, err := crypto.GenerateIV()
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to generate IV: %v", err), 500)
		return
	}

	// Wrap DEK with MEK
	wrappedDEK, err := crypto.WrapDEK(mek, dek)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to wrap DEK: %v", err), 500)
		return
	}

	// Get content type
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Initiate multipart upload with B2 (no ARMOR metadata yet - that comes on completion)
	uploadID, err := h.backend.CreateMultipartUpload(ctx, bucket, key, nil)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create multipart upload: %v", err), 500)
		return
	}

	// Save multipart state to B2
	state := &backend.MultipartState{
		UploadID:       uploadID,
		Bucket:         bucket,
		Key:            key,
		IV:             iv,
		WrappedDEK:     wrappedDEK,
		BlockSize:      h.config.BlockSize,
		Created:        time.Now(),
		ContentType:    contentType,
		KeyID:          keyID,
		EncryptedBytes: 0,
		PartHMACs:      make(map[int]string),
		PartSizes:      make(map[int]int64),
	}

	manager := backend.NewMultipartStateManager(h.backend, bucket)
	if err := manager.SaveState(ctx, state); err != nil {
		// Try to abort the upload on state save failure
		h.backend.AbortMultipartUpload(ctx, bucket, key, uploadID)
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to save multipart state: %v", err), 500)
		return
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
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// UploadPart handles S3 UploadPart with encryption.
// Each part is encrypted with a CTR counter offset based on cumulative encrypted bytes.
func (h *Handlers) UploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Parse part number
	partNumberStr := r.URL.Query().Get("partNumber")
	if partNumberStr == "" {
		h.writeError(w, "InvalidRequest", "Missing partNumber", 400)
		return
	}
	partNumber, err := strconv.ParseInt(partNumberStr, 10, 32)
	if err != nil || partNumber < 1 || partNumber > 10000 {
		h.writeError(w, "InvalidRequest", "Invalid partNumber", 400)
		return
	}

	// Load multipart state
	manager := backend.NewMultipartStateManager(h.backend, bucket)
	state, err := manager.LoadState(ctx, uploadID)
	if err != nil {
		h.writeError(w, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
		return
	}

	// Verify bucket and key match
	if state.Bucket != bucket || state.Key != key {
		h.writeError(w, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
		return
	}

	// Read plaintext part
	plaintext, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	plaintextSize := int64(len(plaintext))

	// Get the MEK for this upload using the key ID from state
	mek, err := h.keyManager.GetMEKByID(state.KeyID)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get decryption key: %v", err), 500)
		return
	}

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(mek, state.WrappedDEK)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to unwrap DEK: %v", err), 500)
		return
	}

	// Calculate starting block index for CTR counter
	// Each block is blockSize bytes, so counter = encryptedBytes / blockSize
	startBlockIndex := uint32(state.EncryptedBytes / int64(state.BlockSize))

	// Create encryptor with the part's starting counter
	encryptor, err := crypto.NewEncryptorWithCounter(dek, state.IV, state.BlockSize, startBlockIndex)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to create encryptor: %v", err), 500)
		return
	}

	// Encrypt the part
	encrypted, hmacTable, err := encryptor.Encrypt(plaintext)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to encrypt: %v", err), 500)
		return
	}

	// Derive HMAC key
	hmacKey := crypto.DeriveHMACKey(dek)

	// Compute block HMACs
	blockHMACs, err := backend.ComputeBlockHMACs(encrypted, state.BlockSize, hmacKey, startBlockIndex)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to compute HMACs: %v", err), 500)
		return
	}

	// Upload encrypted part to B2
	etag, err := h.backend.UploadPart(ctx, bucket, key, uploadID, int32(partNumber), bytes.NewReader(encrypted), int64(len(encrypted)))
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to upload part: %v", err), 500)
		return
	}

	// Update state with cumulative bytes and part HMACs
	state.EncryptedBytes += int64(len(encrypted))
	state.PartHMACs[int(partNumber)] = backend.EncodeHMACToBase64(blockHMACs)
	state.PartSizes[int(partNumber)] = plaintextSize

	if err := manager.SaveState(ctx, state); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to update multipart state: %v", err), 500)
		return
	}

	// Return ETag - suppress unused variable warning
	_ = hmacTable

	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusOK)
}

// CompleteMultipartUpload handles S3 CompleteMultipartUpload.
// It assembles the parts in B2 and stores the HMAC table as a sidecar.
func (h *Handlers) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state
	manager := backend.NewMultipartStateManager(h.backend, bucket)
	state, err := manager.LoadState(ctx, uploadID)
	if err != nil {
		h.writeError(w, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
		return
	}

	// Verify bucket and key match
	if state.Bucket != bucket || state.Key != key {
		h.writeError(w, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
		return
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	if err := xml.Unmarshal(body, &completeReq); err != nil {
		h.writeError(w, "MalformedXML", fmt.Sprintf("Failed to parse XML: %v", err), 400)
		return
	}

	if len(completeReq.Parts) == 0 {
		h.writeError(w, "InvalidRequest", "No parts specified", 400)
		return
	}

	// Convert to backend.CompletedPart
	parts := make([]backend.CompletedPart, len(completeReq.Parts))
	for i, p := range completeReq.Parts {
		parts[i] = backend.CompletedPart{
			PartNumber: int32(p.PartNumber),
			ETag:       p.ETag,
		}
	}

	// Calculate total plaintext size
	var totalPlaintextSize int64
	for _, p := range completeReq.Parts {
		if size, ok := state.PartSizes[p.PartNumber]; ok {
			totalPlaintextSize += size
		}
	}

	// Assemble all block HMACs in order
	var allBlockHMACs [][]byte
	for _, p := range completeReq.Parts {
		if hmacsBase64, ok := state.PartHMACs[p.PartNumber]; ok {
			hmacs, err := backend.DecodeHMACFromBase64(hmacsBase64)
			if err != nil {
				h.writeError(w, "InternalError", fmt.Sprintf("Failed to decode HMACs for part %d: %v", p.PartNumber, err), 500)
				return
			}
			allBlockHMACs = append(allBlockHMACs, hmacs...)
		}
	}

	// Complete the multipart upload in B2
	etag, err := h.backend.CompleteMultipartUpload(ctx, bucket, key, uploadID, parts)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to complete multipart upload: %v", err), 500)
		return
	}

	// Store HMAC table as sidecar
	if err := manager.SaveHMACTable(ctx, key, allBlockHMACs, state.BlockSize); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to save HMAC table: %v", err), 500)
		return
	}

	// Compute plaintext SHA-256 (we don't have the full plaintext, so use a placeholder)
	// In a full implementation, we'd track SHA during upload
	plaintextSHA := sha256.Sum256([]byte{})

	// Build ARMOR metadata and update via CopyObject
	meta := (&backend.ARMORMetadata{
		Version:       1,
		BlockSize:     state.BlockSize,
		PlaintextSize: totalPlaintextSize,
		ContentType:   state.ContentType,
		IV:            state.IV,
		WrappedDEK:    state.WrappedDEK,
		PlaintextSHA:  hex.EncodeToString(plaintextSHA[:]),
		ETag:          etag,
		KeyID:         state.KeyID,
	}).ToMetadata()

	// Add multipart flag to indicate HMAC table is external
	meta["x-amz-meta-armor-multipart"] = "true"

	// Update metadata via CopyObject
	if err := h.backend.Copy(ctx, bucket, key, bucket, key, meta, true); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to update metadata: %v", err), 500)
		return
	}

	// Clean up multipart state
	manager.DeleteState(ctx, uploadID)

	// Record provenance for the multipart upload
	if h.provenance != nil && h.provenance.ShouldRecord(key) {
		_ = h.provenance.RecordUpload(ctx, key, hex.EncodeToString(plaintextSHA[:]), "multipart")
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
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// AbortMultipartUpload handles S3 AbortMultipartUpload.
// It deletes the multipart state and forwards the abort to B2.
func (h *Handlers) AbortMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state to verify it exists
	manager := backend.NewMultipartStateManager(h.backend, bucket)
	state, err := manager.LoadState(ctx, uploadID)
	if err != nil {
		h.writeError(w, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
		return
	}

	// Verify bucket and key match
	if state.Bucket != bucket || state.Key != key {
		h.writeError(w, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
		return
	}

	// Forward abort to B2
	if err := h.backend.AbortMultipartUpload(ctx, bucket, key, uploadID); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to abort multipart upload: %v", err), 500)
		return
	}

	// Clean up multipart state
	manager.DeleteState(ctx, uploadID)

	w.WriteHeader(http.StatusNoContent)
}

// ListParts handles S3 ListParts operation.
// It forwards to B2 and adjusts part sizes to plaintext sizes.
func (h *Handlers) ListParts(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	ctx := r.Context()

	// Load multipart state to get plaintext sizes
	manager := backend.NewMultipartStateManager(h.backend, bucket)
	state, err := manager.LoadState(ctx, uploadID)
	if err != nil {
		h.writeError(w, "NoSuchUpload", fmt.Sprintf("Multipart upload not found: %v", err), 404)
		return
	}

	// Verify bucket and key match
	if state.Bucket != bucket || state.Key != key {
		h.writeError(w, "NoSuchUpload", "Multipart upload does not match bucket/key", 404)
		return
	}

	// Forward to B2
	result, err := h.backend.ListParts(ctx, bucket, key, uploadID)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to list parts: %v", err), 500)
		return
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
			LastModified: part.LastModified.UTC().Format(http.TimeFormat),
		})
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// ListMultipartUploads handles S3 ListMultipartUploads operation.
// It forwards directly to B2 (passthrough operation).
func (h *Handlers) ListMultipartUploads(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	result, err := h.backend.ListMultipartUploads(ctx, bucket)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to list multipart uploads: %v", err), 500)
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
		NextKeyMarker:      result.NextKeyMarker,
		NextUploadIDMarker: result.NextUploadIDMarker,
		IsTruncated:        result.IsTruncated,
	}

	for _, upload := range result.Uploads {
		resp.Uploads = append(resp.Uploads, Upload{
			Key:          upload.Key,
			UploadID:     upload.UploadID,
			Initiator:    "armor",
			Owner:        "armor",
			StorageClass: "STANDARD",
			Initiated:    upload.Initiated.UTC().Format(time.RFC3339),
		})
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// ListObjectVersions handles S3 ListObjectVersions operation.
// It lists all versions of objects in a bucket, For ARMOR-encrypted objects,
// it retrieves per-version metadata to provide plaintext sizes.
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

	result, err := h.backend.ListObjectVersions(ctx, bucket, prefix, delimiter, keyMarker, versionIDMarker, maxKeys)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to list object versions: %v", err), 500)
		return
	}

	// Build XML response
	type Version struct {
		Key          string `xml:"Key"`
		VersionID    string `xml:"VersionId"`
		IsLatest     bool   `xml:"IsLatest"`
		IsDeleteMarker bool `xml:"IsDeleteMarker,omitempty"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag,omitempty"`
		Size         int64  `xml:"Size,omitempty"`
		StorageClass string `xml:"StorageClass,omitempty"`
	}

	type ListVersionsResult struct {
		XMLName               xml.Name `xml:"ListVersionsResult"`
		Xmlns                 string   `xml:"xmlns,attr"`
		Name                  string   `xml:"Name"`
		Prefix                string   `xml:"Prefix"`
		Delimiter             string   `xml:"Delimiter,omitempty"`
		MaxKeys               int      `xml:"MaxKeys"`
		IsTruncated           bool     `xml:"IsTruncated"`
		KeyMarker             string   `xml:"KeyMarker"`
		VersionIDMarker       string   `xml:"VersionIdMarker"`
		NextKeyMarker         string   `xml:"NextKeyMarker"`
		NextVersionIDMarker   string   `xml:"NextVersionIdMarker"`
		Versions              []Version `xml:"Version"`
		CommonPrefixes       []string `xml:"CommonPrefixes>Prefix"`
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
		NextKeyMarker:       result.NextKeyMarker,
		NextVersionIDMarker: result.NextVersionIDMarker,
	}

	// Process versions and retrieve per-version metadata for ARMOR objects
	for _, version := range result.Versions {
		v := Version{
			Key:            version.Key,
			VersionID:      version.VersionID,
			IsLatest:       version.IsLatest,
			IsDeleteMarker: version.IsDeleteMarker,
			LastModified:   version.LastModified.UTC().Format(http.TimeFormat),
		}

		if !version.IsDeleteMarker {
		// Try to get ARMOR metadata for this specific version
		// This requires a HeadObject call per version, get plaintext size
		if info, err := h.backend.HeadVersion(ctx, bucket, version.Key, version.VersionID); err == nil {
			// Check if it version is ARMOR-encrypted
			if am, ok := backend.ParseARMORMetadata(info.Metadata); ok {
				// Use plaintext size and ARMOR ETag
				v.Size = am.PlaintextSize
				v.ETag = fmt.Sprintf(`"%s"`, am.ETag)
				v.StorageClass = "STANDARD"
			} else {
				// Non-ARMOR object, use raw size
				v.Size = version.Size
				v.ETag = fmt.Sprintf(`"%s"`, version.ETag)
				v.StorageClass = "STANDARD"
			}
		} else {
			// Fallback to raw size if we can't get version metadata
			v.Size = version.Size
			v.ETag = fmt.Sprintf(`"%s"`, version.ETag)
			v.StorageClass = "STANDARD"
		}
	}

		resp.Versions = append(resp.Versions, v)
	}

	// Process common prefixes
	resp.CommonPrefixes = append(resp.CommonPrefixes, result.CommonPrefixes...)

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, "InternalError", "Failed to marshal response", 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get lifecycle configuration: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	if err := h.backend.PutBucketLifecycleConfiguration(ctx, bucket, body); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to put lifecycle configuration: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBucketLifecycleConfiguration handles DELETE ?lifecycle on a bucket.
// This is a passthrough operation - lifecycle configuration is not encrypted.
func (h *Handlers) DeleteBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.DeleteBucketLifecycleConfiguration(ctx, bucket); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to delete lifecycle configuration: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get object lock configuration: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	if err := h.backend.PutObjectLockConfiguration(ctx, bucket, body); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to put object lock configuration: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get object retention: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	if err := h.backend.PutObjectRetention(ctx, bucket, key, body); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to put object retention: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get object legal hold: %v", err), 500)
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
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	if err := h.backend.PutObjectLegalHold(ctx, bucket, key, body); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to put object legal hold: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// writeError writes an S3 error response.
func (h *Handlers) writeError(w http.ResponseWriter, code, message string, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/xml")
	errorXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
</Error>`, code, message)
	w.Write([]byte(errorXML))
}
