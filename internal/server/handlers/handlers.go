// Package handlers implements S3 operation handlers for ARMOR.
package handlers

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/crypto"
)

// Handlers contains all S3 operation handlers.
type Handlers struct {
	config  *config.Config
	backend backend.Backend
	cache   *backend.MetadataCache
	mek     []byte
}

// New creates a new Handlers instance.
func New(cfg *config.Config, be backend.Backend, cache *backend.MetadataCache, mek []byte) *Handlers {
	return &Handlers{
		config:  cfg,
		backend: be,
		cache:   cache,
		mek:     mek,
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
			h.PutObject(w, r, bucket, key)
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

	// Full object download
	// Calculate encrypted size and fetch envelope
	encSize := crypto.FullObjectSize(plaintextSize, armorMeta.BlockSize)

	// Fetch entire envelope
	body, err := h.backend.GetRange(ctx, bucket, key, 0, encSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to get object: %v", err), 500)
		return
	}
	defer body.Close()

	// Read envelope
	envelope, err := io.ReadAll(body)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read envelope: %v", err), 500)
		return
	}

	// Parse header
	header, err := crypto.DecodeHeader(envelope)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
		return
	}

	// Extract encrypted blocks and HMAC table
	dataStart := crypto.HeaderSize
	dataEnd := dataStart + int(plaintextSize)
	hmacStart := int(header.HMACTableOffset())
	hmacEnd := hmacStart + int(header.BlockCount())*crypto.HMACSize

	if hmacEnd > len(envelope) {
		h.writeError(w, "InternalError", "Envelope too short for HMAC table", 500)
		return
	}

	encryptedBlocks := envelope[dataStart:dataEnd]
	hmacTable := envelope[hmacStart:hmacEnd]

	// Decrypt
	plaintext, err := decryptor.Decrypt(encryptedBlocks, hmacTable)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to decrypt: %v", err), 500)
		return
	}

	// Verify plaintext SHA
	if err := header.VerifyPlaintextSHA(plaintext); err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Integrity check failed: %v", err), 500)
		return
	}

	// Set response headers
	w.Header().Set("Content-Length", strconv.FormatInt(plaintextSize, 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.WriteHeader(http.StatusOK)
	w.Write(plaintext)
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

	// Translate range to encrypted blocks
	translation, err := crypto.TranslateRange(start, end, plaintextSize, armorMeta.BlockSize, crypto.HeaderSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to translate range: %v", err), 500)
		return
	}

	// Fetch encrypted blocks and HMAC table in parallel
	// For now, fetch sequentially (parallel fetch is an optimization)
	encryptedBody, err := h.backend.GetRange(ctx, bucket, key, translation.DataOffset, translation.DataLength)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to fetch encrypted blocks: %v", err), 500)
		return
	}
	defer encryptedBody.Close()

	encrypted, err := io.ReadAll(encryptedBody)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read encrypted blocks: %v", err), 500)
		return
	}

	hmacBody, err := h.backend.GetRange(ctx, bucket, key, translation.HMACOffset, translation.HMACLength)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to fetch HMAC table: %v", err), 500)
		return
	}
	defer hmacBody.Close()

	hmacTable, err := io.ReadAll(hmacBody)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to read HMAC table: %v", err), 500)
		return
	}

	// Decrypt range
	plaintext, err := decryptor.DecryptRange(encrypted, hmacTable, start, end, plaintextSize)
	if err != nil {
		h.writeError(w, "InternalError", fmt.Sprintf("Failed to decrypt range: %v", err), 500)
		return
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
