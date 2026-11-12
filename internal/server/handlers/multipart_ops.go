package handlers

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

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

// defaultUploadPartMemoryBudget is the aggregate in-memory v3 UploadPart
// plaintext budget when the config does not set one: eight pooled 64 MiB
// parts, the iad-ci workload the budget was sized against (armor-cb2b0731).
const defaultUploadPartMemoryBudget = 512 << 20

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
	if err := h.backend.AbortMultipartUpload(ctx, bucket, h.applyPrefix(key), uploadID); err != nil {
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
	result, err := h.backend.ListParts(ctx, bucket, h.applyPrefix(key), uploadID)
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

	// The backend returns stored-space keys and its contract predates the
	// marker and max-uploads fields, so apply those page boundaries here while
	// keeping the backend namespace private. This preserves the existing prefix
	// forwarding without changing ListObjectsV2 semantics.
	//
	// Compose the configured prefix onto the key-marker only when the client
	// actually sent one — composing onto the empty string would fabricate a
	// cursor at the bare ARMOR_PREFIX, the same failure shape ListObjectsV2
	// guards against for StartAfter (armor-56ecc6b7). S3 semantics for the
	// pair: without upload-id-marker only keys lexicographically GREATER than
	// the key-marker are listed, so uploads whose key equals the marker are
	// excluded; with upload-id-marker, uploads for the marked key return only
	// when their upload ID is lexicographically greater (armor-4e5b7a10).
	keyMarker := ""
	if rawKeyMarker := r.URL.Query().Get("key-marker"); rawKeyMarker != "" {
		keyMarker = h.applyPrefix(rawKeyMarker)
	}
	uploadIDMarker := r.URL.Query().Get("upload-id-marker")
	filtered := result.Uploads[:0]
	for _, upload := range result.Uploads {
		if keyMarker != "" {
			if upload.Key < keyMarker {
				continue
			}
			if upload.Key == keyMarker && (uploadIDMarker == "" || upload.UploadID <= uploadIDMarker) {
				continue
			}
		}
		filtered = append(filtered, upload)
	}
	result.Uploads = filtered
	maxUploads := 1000
	if raw := r.URL.Query().Get("max-uploads"); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil && parsed > 0 {
			maxUploads = parsed
		}
	}
	if len(result.Uploads) > maxUploads {
		last := result.Uploads[maxUploads-1]
		result.Uploads = result.Uploads[:maxUploads]
		result.IsTruncated = true
		result.NextKeyMarker = last.Key
		result.NextUploadIDMarker = last.UploadID
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
