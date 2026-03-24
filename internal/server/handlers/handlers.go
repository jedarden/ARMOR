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
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
)

// Handlers contains all S3 operation handlers.
type Handlers struct {
	config      *config.Config
	backend     backend.Backend
	cache       *backend.MetadataCache
	footerCache *backend.FooterCache
	mek         []byte
}

// New creates a new Handlers instance.
func New(cfg *config.Config, be backend.Backend, cache *backend.MetadataCache, footerCache *backend.FooterCache, mek []byte) *Handlers {
	return &Handlers{
		config:      cfg,
		backend:     be,
		cache:       cache,
		footerCache: footerCache,
		mek:         mek,
	}
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
		if key != "" {
			h.DeleteObject(w, r, bucket, key)
		} else if bucket != "" {
			// Check for delete marker query
			if r.URL.Query().Get("delete") != "" {
				h.DeleteObjects(w, r, bucket)
			} else {
				h.DeleteBucket(w, r, bucket)
			}
		}
	case http.MethodPost:
		// Handle multipart upload operations
		uploads := r.URL.Query().Get("uploads")
		uploadID := r.URL.Query().Get("uploadId")
		if uploads != "" {
			h.CreateMultipartUpload(w, r, bucket, key)
		} else if uploadID != "" {
			if r.URL.Query().Get("partNumber") != "" {
				h.UploadPart(w, r, bucket, key, uploadID)
			} else {
				h.CompleteMultipartUpload(w, r, bucket, key, uploadID)
			}
		} else {
			h.writeError(w, "InvalidRequest", "Unsupported POST operation", 400)
		}
	default:
		h.writeError(w, "MethodNotAllowed", fmt.Sprintf("Method %s not allowed", r.Method), 405)
	}
}

// PutObject handles S3 PutObject with encryption.
func (h *Handlers) PutObject(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Read plaintext body
	plaintext, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500)
		return
	}

	plaintextSize := int64(len(plaintext))

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
	wrappedDEK, err := crypto.WrapDEK(h.mek, dek)
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
	}).ToMetadata()

	// Upload to B2
	if err := h.backend.Put(ctx, bucket, key, bytes.NewReader(envelope), int64(len(envelope)), meta); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to upload: %v", err), 500)
		return
	}

	// Return ETag
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, etag))
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

	// Unwrap DEK
	dek, err := crypto.UnwrapDEK(h.mek, armorMeta.WrappedDEK)
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

	w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	w.Header().Set("Content-Type", info.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, info.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))

	if info.IsARMOREncrypted {
		if am, ok := backend.ParseARMORMetadata(info.Metadata); ok {
			w.Header().Set("Content-Length", strconv.FormatInt(am.PlaintextSize, 10))
			w.Header().Set("Content-Type", am.ContentType)
			w.Header().Set("ETag", fmt.Sprintf(`"%s"`, am.ETag))
		}
	}

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

		// Unwrap DEK with current MEK
		dek, err := crypto.UnwrapDEK(h.mek, armorMeta.WrappedDEK)
		if err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Failed to unwrap DEK: %v", err), 500)
			return
		}

		// Re-wrap DEK with current MEK (handles key rotation case)
		wrappedDEK, err := crypto.WrapDEK(h.mek, dek)
		if err != nil {
			h.writeError(w, "InternalError", fmt.Sprintf("Failed to re-wrap DEK: %v", err), 500)
			return
		}

		// Build new metadata with re-wrapped DEK
		newMeta := (&backend.ARMORMetadata{
			Version:       armorMeta.Version,
			BlockSize:     armorMeta.BlockSize,
			PlaintextSize: armorMeta.PlaintextSize,
			ContentType:   armorMeta.ContentType,
			IV:            armorMeta.IV,
			WrappedDEK:    wrappedDEK,
			PlaintextSHA:  armorMeta.PlaintextSHA,
			ETag:          armorMeta.ETag,
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

	for _, cp := range result.CommonPrefixes {
		resp.CommonPrefixes = append(resp.CommonPrefixes, cp)
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

// Stub implementations for other operations

func (h *Handlers) HeadBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	// TODO: Implement
	w.WriteHeader(http.StatusOK)
}

func (h *Handlers) CreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	// TODO: Implement
	h.writeError(w, "NotImplemented", "CreateBucket not implemented", 501)
}

func (h *Handlers) DeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	// TODO: Implement
	h.writeError(w, "NotImplemented", "DeleteBucket not implemented", 501)
}

func (h *Handlers) DeleteObjects(w http.ResponseWriter, r *http.Request, bucket string) {
	// TODO: Implement
	h.writeError(w, "NotImplemented", "DeleteObjects not implemented", 501)
}

func (h *Handlers) ListBuckets(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement
	h.writeError(w, "NotImplemented", "ListBuckets not implemented", 501)
}

func (h *Handlers) CreateMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key string) {
	// TODO: Implement
	h.writeError(w, "NotImplemented", "CreateMultipartUpload not implemented", 501)
}

func (h *Handlers) UploadPart(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	// TODO: Implement
	h.writeError(w, "NotImplemented", "UploadPart not implemented", 501)
}

func (h *Handlers) CompleteMultipartUpload(w http.ResponseWriter, r *http.Request, bucket, key, uploadID string) {
	// TODO: Implement
	h.writeError(w, "NotImplemented", "CompleteMultipartUpload not implemented", 501)
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
