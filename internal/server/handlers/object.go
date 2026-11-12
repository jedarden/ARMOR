package handlers

import (
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

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

	// CopyObject reads the source object on behalf of the caller, so the
	// source needs its own Get authorization in addition to the destination's
	// Put check in the server wrapper. Keep direct handler callers (which have
	// no authenticated context) compatible with the legacy handler tests; every
	// client-facing request reaches this point with a credential in context.
	if strings.HasPrefix(srcKey, ".armor/") {
		h.writeError(w, r, "AccessDenied", "Access to .armor/ reserved namespace is denied", http.StatusForbidden)
		return
	}
	if cred := acl.CredentialFromContext(ctx); cred != nil {
		if err := acl.CheckACL(cred, srcBucket, srcKey, acl.ActionGet); err != nil {
			h.writeError(w, r, "AccessDenied", fmt.Sprintf("Access denied: GET access to /%s/%s", srcBucket, srcKey), http.StatusForbidden)
			return
		}
	}
	if srcBucket == dstBucket && srcKey == dstKey {
		h.writeError(w, r, "InvalidRequest", "Cannot copy an object onto itself", http.StatusBadRequest)
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

	// Resolve the tagging directive (x-amz-tagging-directive) once, before
	// any backend work: COPY (the default) carries the source's tag set to
	// the destination, REPLACE takes the request's x-amz-tagging set. An
	// invalid directive or tag header fails the copy with InvalidArgument.
	replaceTags, replaceTagsEncoded, err := copyTaggingDirective(r)
	if err != nil {
		h.writeError(w, r, "InvalidArgument", fmt.Sprintf("Invalid tagging directive: %v", err), 400)
		return
	}

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
		} else {
			// The ARMOR metadata map is rebuilt above for the re-wrapped DEK;
			// explicitly preserve client metadata for the default COPY directive.
			for k, v := range srcInfo.Metadata {
				lower := strings.ToLower(k)
				if strings.HasPrefix(lower, "x-amz-meta-") && !strings.HasPrefix(lower, "x-amz-meta-armor-") {
					newMeta[lower] = v
				}
			}
		}

		// Carry the tag set across the re-wrap: newMeta was rebuilt from the
		// parsed ARMOR fields, so without this the destination would silently
		// drop the source's tags even under the COPY directive.
		applyCopyTagging(newMeta, srcInfo.Metadata, replaceTags, replaceTagsEncoded)

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
			lower := strings.ToLower(k)
			if strings.HasPrefix(lower, "x-amz-meta-") && !strings.HasPrefix(lower, "x-amz-meta-armor-") {
				meta[strings.ToLower(k)] = v[0]
			}
		}
		applyCopyTagging(meta, srcInfo.Metadata, replaceTags, replaceTagsEncoded)
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

// maxDeleteObjectsKeys is the S3 limit on the number of objects a single
// DeleteObjects request may name; a request above it is rejected with 400
// MalformedXML before any key is evaluated or removed.
const maxDeleteObjectsKeys = 1000

// maxDeleteObjectsBodyBytes bounds the DeleteObjects request body before it
// is parsed. A legitimate body names at most maxDeleteObjectsKeys keys, each
// at most 1024 bytes of UTF-8 plus XML markup and entity escaping, so this
// bound never rejects a valid request while capping what one request can
// make the server buffer.
const maxDeleteObjectsBodyBytes = 8 << 20 // 8 MiB

// DeleteObjects handles S3 DeleteObjects (bulk delete).
// The request body contains XML with a list of objects to delete.
// Per-key ACL enforcement is performed for each key in the request body,
// and keys under the reserved .armor/ namespace are refused per key —
// POST ?delete carries its keys in the body, so the HandleRoot namespace
// guard (which only sees the URL key, empty here) cannot protect them.
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

	// Bound the body before parsing: a DeleteObjects body is a bounded list
	// of keys, so an oversized one cannot be a legitimate request and must
	// not be buffered whole.
	body, err := io.ReadAll(io.LimitReader(r.Body, maxDeleteObjectsBodyBytes+1))
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}
	if int64(len(body)) > maxDeleteObjectsBodyBytes {
		h.writeError(w, r, "MalformedXML", fmt.Sprintf("Delete request body exceeds %d bytes", maxDeleteObjectsBodyBytes), 400)
		return
	}

	if err := xml.Unmarshal(body, &deleteReq); err != nil {
		h.writeError(w, r, "MalformedXML", fmt.Sprintf("Failed to parse XML: %v", err), 400)
		return
	}

	// S3 caps a DeleteObjects request at 1000 keys; a longer one is rejected
	// outright, before any key is evaluated or removed.
	if len(deleteReq.Objects) > maxDeleteObjectsKeys {
		h.writeError(w, r, "MalformedXML", fmt.Sprintf("Delete request contains more than %d keys", maxDeleteObjectsKeys), 400)
		return
	}

	// S3 accepts an empty batch and returns an empty DeleteResult. Continue
	// through the ordinary response path so quiet mode and the XML shape stay
	// identical to a successful non-empty request.

	// Extract original keys from request and prepare prefixed keys for backend
	originalKeys := make([]string, len(deleteReq.Objects))
	for i, obj := range deleteReq.Objects {
		originalKeys[i] = obj.Key
	}

	// Perform per-key ACL enforcement.
	// CheckACL requires bucket, key, and verb. For DeleteObjects, the verb is "delete".
	// If credential is nil (no auth), allow all deletions (backward compatibility).
	var allowedKeys []string
	type deniedKey struct {
		key     string
		message string
	}
	var deniedKeys []deniedKey

	for _, key := range originalKeys {
		// The reserved .armor/ namespace is unreachable through every
		// single-key operation (HandleRoot refuses it before dispatch);
		// a bulk request must not become the bypass. Refuse the key per
		// key — no delete, no tombstone — whatever the credential's ACL
		// grants, and even when auth is disabled.
		if strings.HasPrefix(key, ".armor/") {
			deniedKeys = append(deniedKeys, deniedKey{key: key, message: reservedNamespaceDeniedMsg})
			continue
		}
		if cred == nil {
			// No authentication - allow all deletions (backward compatible)
			allowedKeys = append(allowedKeys, key)
			continue
		}
		// Check ACL for each key individually
		if err := acl.CheckACL(cred, bucket, key, "delete"); err != nil {
			deniedKeys = append(deniedKeys, deniedKey{key: key, message: "Access Denied"})
		} else {
			allowedKeys = append(allowedKeys, key)
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
	// a restricted credential's failed batch look fully successful. Reserved
	// namespace refusals share the entries, in request order.
	for _, denied := range deniedKeys {
		result.Error = append(result.Error, struct {
			Key     string `xml:"Key"`
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		}{
			Key:     denied.key,
			Code:    "AccessDenied",
			Message: denied.message,
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
