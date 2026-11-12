package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

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
	// The authoritative HEAD's stored length is needed by both v3 full-read
	// paths: single-PUT uses it to locate the trailer before the data stream's
	// header arrives, and multipart uses it to start ciphertext and sidecar
	// reads concurrently.
	armorMeta.CiphertextSize = info.StoredSize

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

	// Stored-object generation for the v3 table/header cache. Compute it before
	// the header decision so range reads can reuse a cached header and full
	// reads can attach the streamed header to the table loaded below.
	storedGenSource := info
	if ciphertextInfo != nil {
		storedGenSource = ciphertextInfo
	}
	storedGen := generationFromInfo(storedGenSource)

	// Check if this is a multipart object (HMAC table in sidecar, no embedded header)
	isMultipart := info.Metadata["x-amz-meta-armor-multipart"] == "true"
	partNumber := int64(0)
	if rawPartNumber := r.URL.Query().Get("partNumber"); rawPartNumber != "" {
		parsedPartNumber, parseErr := strconv.ParseInt(rawPartNumber, 10, 64)
		if parseErr != nil || parsedPartNumber < 1 || parsedPartNumber > 10000 {
			h.writeError(w, r, "InvalidPart", "part number must be between 1 and 10000", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if !isMultipart {
			h.writeError(w, r, "InvalidPart", "part number is only valid for multipart objects", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if r.Header.Get("Range") != "" {
			h.writeError(w, r, "InvalidRequest", "part number cannot be combined with Range", http.StatusBadRequest)
			return
		}
		partNumber = parsedPartNumber
	}

	if partNumber != 0 {
		var start, end int64
		var ok bool
		if armorMeta.Version == crypto.Version3 {
			// V3 multipart uploads allow independently-sized parts and therefore
			// do not have a uniform part-size metadata value. The sidecar is the
			// authoritative source for their part boundaries.
			sidecar, loadErr := h.loadV3MultipartSidecar(ctx, bucket, key, prefixedKey, armorMeta.ETag)
			if loadErr != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load v3 sidecar: %v", loadErr), 500)
				return
			}
			start, end, ok = v3MultipartPartRange(sidecar, partNumber)
		} else {
			start, end, ok = multipartPartRange(info.Metadata, partNumber, plaintextSize)
		}
		if !ok {
			h.writeError(w, r, "InvalidPart", fmt.Sprintf("multipart part %d does not exist", partNumber), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		r.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	}

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

		// Uncached, uncompressed v3 full GETs locate the trailer from the
		// authoritative stored length and decode the header from the data stream
		// itself. Range reads retain the pre-dispatch header read, except when
		// the matching table cache entry already carries the decoded header.
		streamHeaderFromData := singleFetch == nil && r.Header.Get("Range") == "" &&
			armorMeta.Version == crypto.Version3 && !armorMeta.Compressed
		if streamHeaderFromData {
			decryptor, err = crypto.NewDecryptorWithVersion(dek, armorMeta.IV, armorMeta.BlockSize, uint8(armorMeta.Version))
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create decryptor: %v", err), 500)
				return
			}
		} else {
			var header *crypto.EnvelopeHeader
			if singleFetch != nil {
				// The single fetch already covers the header; no separate read.
				header, err = crypto.DecodeHeader(singleFetch[:crypto.HeaderSize])
			} else if r.Header.Get("Range") != "" && armorMeta.Version == crypto.Version3 {
				// A full v3 GET stores this header beside its table, allowing
				// repeated ranges to skip the 64-byte backend read.
				header, _ = h.cachedV3Header(bucket, prefixedKey, storedGen)
				if header == nil {
					headerReader, readErr := h.backend.GetRange(ctx, bucket, prefixedKey, 0, crypto.HeaderSize)
					if readErr != nil {
						h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read envelope header: %v", readErr), 500)
						return
					}
					defer headerReader.Close()
					headerBuf := make([]byte, crypto.HeaderSize)
					if _, readErr := io.ReadFull(headerReader, headerBuf); readErr != nil {
						h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", readErr), 500)
						return
					}
					header, err = crypto.DecodeHeader(headerBuf)
				}
			} else {
				headerReader, readErr := h.backend.GetRange(ctx, bucket, prefixedKey, 0, crypto.HeaderSize)
				if readErr != nil {
					h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read envelope header: %v", readErr), 500)
					return
				}
				defer headerReader.Close()
				headerBuf := make([]byte, crypto.HeaderSize)
				if _, readErr := io.ReadFull(headerReader, headerBuf); readErr != nil {
					h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", readErr), 500)
					return
				}
				header, err = crypto.DecodeHeader(headerBuf)
			}
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decode header: %v", err), 500)
				return
			}
			envelopeHeader = header
			if header.Version == crypto.Version3 {
				h.cacheV3Header(bucket, prefixedKey, storedGen, header)
			}

			// Create decryptor with the version from the envelope header.
			decryptor, err = crypto.OpenDecryptor(header, dek)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create decryptor: %v", err), 500)
				return
			}
		}

		if armorMeta.Version == crypto.Version3 && armorMeta.CiphertextSize == 0 {
			h.writeError(w, r, "InternalError", "v3 object missing stored ciphertext size", 500)
			return
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

// validateEnvelopeHeader checks that the header carried by a streamed
// ciphertext agrees with the authenticated object metadata selected by the
// manifest/HEAD path. The stream is not exposed to the client until this
// check and the v3 table load both succeed.
func validateEnvelopeHeader(header *crypto.EnvelopeHeader, armorMeta *backend.ARMORMetadata) error {
	if header == nil || armorMeta == nil {
		return fmt.Errorf("missing envelope metadata")
	}
	if int(header.Version) != armorMeta.Version {
		return fmt.Errorf("header version %d does not match metadata version %d", header.Version, armorMeta.Version)
	}
	if armorMeta.BlockSize > 0 && header.BlockSize() != armorMeta.BlockSize {
		return fmt.Errorf("header block size %d does not match metadata block size %d", header.BlockSize(), armorMeta.BlockSize)
	}
	if len(armorMeta.IV) == len(header.IV) && !bytes.Equal(header.IV[:], armorMeta.IV) {
		return fmt.Errorf("header IV does not match object metadata")
	}
	if armorMeta.PlaintextSize >= 0 && int64(header.PlaintextSize) != armorMeta.PlaintextSize {
		return fmt.Errorf("header plaintext size %d does not match metadata size %d", header.PlaintextSize, armorMeta.PlaintextSize)
	}
	if armorMeta.PlaintextSHA != "" && !strings.EqualFold(hex.EncodeToString(header.PlaintextSHA[:]), armorMeta.PlaintextSHA) {
		return fmt.Errorf("header plaintext digest does not match object metadata")
	}
	return nil
}

// loadV3FullStream starts the uncached v3 trailer and header+data reads at the
// same time. The stored size from HEAD makes the trailer window known before
// the data stream's 64-byte header is decoded: data length is stored size less
// 36 bytes per expected block. Both results are validated before the caller
// can send response headers or release plaintext.
func (h *Handlers) loadV3FullStream(ctx context.Context, bucket, key string, gen v3TableKeyParts, ciphertextSize int64, blockCount int, armorMeta *backend.ARMORMetadata) (io.ReadCloser, *crypto.BlockTable, *crypto.EnvelopeHeader, error) {
	trailerSize := int64(blockCount) * crypto.BlockTableEntrySize
	dataSize := ciphertextSize - trailerSize
	if blockCount <= 0 || dataSize < crypto.HeaderSize {
		return nil, nil, nil, fmt.Errorf("invalid v3 full-read layout: ciphertext=%d blocks=%d", ciphertextSize, blockCount)
	}

	parallelCtx, cancel := context.WithCancel(ctx)
	type tableResult struct {
		table *crypto.BlockTable
		err   error
	}
	type dataResult struct {
		body io.ReadCloser
		err  error
	}
	tableCh := make(chan tableResult, 1)
	dataCh := make(chan dataResult, 1)
	go func() {
		table, err := h.loadV3BlockTable(parallelCtx, bucket, key, gen, ciphertextSize, blockCount)
		tableCh <- tableResult{table: table, err: err}
	}()
	go func() {
		body, err := h.backend.GetRange(parallelCtx, bucket, key, 0, dataSize)
		dataCh <- dataResult{body: body, err: err}
	}()

	tableRes := <-tableCh
	dataRes := <-dataCh
	if tableRes.err != nil {
		cancel()
		if dataRes.body != nil {
			dataRes.body.Close()
		}
		return nil, nil, nil, tableRes.err
	}
	if dataRes.err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("failed to get object stream: %w", dataRes.err)
	}
	if tableRes.table == nil {
		cancel()
		dataRes.body.Close()
		return nil, nil, nil, fmt.Errorf("v3 block table load returned nil table")
	}
	expectedCiphertext := dataSize - crypto.HeaderSize
	if tableRes.table.TotalCiphertextLength64() != expectedCiphertext {
		cancel()
		dataRes.body.Close()
		return nil, nil, nil, fmt.Errorf("v3 block table total ciphertext length %d does not match data span %d", tableRes.table.TotalCiphertextLength64(), expectedCiphertext)
	}

	headerBuf := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(dataRes.body, headerBuf); err != nil {
		cancel()
		dataRes.body.Close()
		return nil, nil, nil, fmt.Errorf("failed to read streamed envelope header: %w", err)
	}
	header, err := crypto.DecodeHeader(headerBuf)
	if err != nil {
		cancel()
		dataRes.body.Close()
		return nil, nil, nil, fmt.Errorf("failed to decode streamed envelope header: %w", err)
	}
	if err := validateEnvelopeHeader(header, armorMeta); err != nil {
		cancel()
		dataRes.body.Close()
		return nil, nil, nil, err
	}
	h.cacheV3Header(bucket, key, gen, header)
	return &cancelingReadCloser{ReadCloser: dataRes.body, cancel: cancel}, tableRes.table, header, nil
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
		h.writeError(w, r, "InvalidRange", fmt.Sprintf("Invalid range: %v", err), http.StatusRequestedRangeNotSatisfiable)
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

	var hmacTable []byte
	var hmacBody io.ReadCloser
	var startBlockIndex int
	// dataBody streams the encrypted bytes of exactly the blocks the range
	// needs; the plaintext is decrypted and emitted block by block rather
	// than buffered (armor-31fe90c8).
	var dataBody io.ReadCloser
	var streamBlockEnd int
	var streamHMACIsFull bool

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

		// Fetch encrypted blocks as a stream; the decrypt loop below reads
		// exactly this many bytes, one block at a time.
		dataBody, err = h.backend.GetRange(ctx, bucket, prefixedKey, dataOffset, dataLength)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("failed to fetch encrypted blocks: %v", err), 500)
			return
		}
		defer dataBody.Close()
		streamBlockEnd = endBlockIndex
		// The sidecar table covers the whole object: absolute block indices.
		streamHMACIsFull = true
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
			h.cacheV3Header(bucket, prefixedKey, storedGen, header)
			return
		}

		// v1/v2: inline HMAC table at end of file
		// Translate range to encrypted blocks
		translation, err := crypto.TranslateRange(start, end, plaintextSize, armorMeta.BlockSize, crypto.HeaderSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to translate range: %v", err), 500)
			return
		}

		// Open the encrypted-block stream and fetch the HMAC table in
		// parallel using errgroup. This cuts range-read latency nearly in
		// half for cache misses. The data body is returned as a reader and
		// consumed block by block below — never buffered whole.
		g, gctx := errgroup.WithContext(ctx)

		g.Go(func() error {
			body, err := h.backend.GetRange(gctx, bucket, prefixedKey, translation.DataOffset, translation.DataLength)
			if err != nil {
				return fmt.Errorf("failed to fetch encrypted blocks: %w", err)
			}
			dataBody = body
			return nil
		})

		g.Go(func() error {
			body, err := h.backend.GetRange(gctx, bucket, prefixedKey, translation.HMACOffset, translation.HMACLength)
			if err != nil {
				return fmt.Errorf("failed to fetch HMAC table: %w", err)
			}
			// Keep the HMAC table as a bounded reader. The range decrypt loop
			// consumes one entry per block; buffering every entry made a
			// bytes=0- request retain an object-sized side table.
			hmacBody = body
			return nil
		})

		if err := g.Wait(); err != nil {
			if dataBody != nil {
				dataBody.Close()
			}
			if hmacBody != nil {
				hmacBody.Close()
			}
			h.writeError(w, r, "InternalError", err.Error(), 500)
			return
		}
		defer dataBody.Close()
		defer hmacBody.Close()
		streamBlockEnd = translation.BlockEnd
		// The inline table reader yields only this range's blocks in relative
		// order, one HMAC entry at a time.
		streamHMACIsFull = false
	}

	// Decrypt and emit one block at a time (armor-31fe90c8). Each block is
	// HMAC-verified before its plaintext is released; the writer declares
	// Content-Length up front, and a failure after the first block aborts the
	// connection against that declared length instead of returning a
	// truncated 206 as success.
	sw := h.newRangeStreamWriter(w, r, bucket, key, armorMeta, plaintextSize, start, end, lastModified)
	if err := streamV1V2RangeBlocks(sw, dataBody, hmacTable, hmacBody, streamBlockEnd, streamHMACIsFull, decryptor, armorMeta, plaintextSize); err != nil {
		sw.abort(err.Error())
		return
	}
	sw.finish()
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

// multipartPartRange maps an S3 GetObject partNumber to the plaintext span
// represented by that part. Current multipart objects record a uniform part
// size; non-uniform legacy objects carry cumulative part sizes instead.
func multipartPartRange(metadata map[string]string, partNumber, plaintextSize int64) (start, end int64, ok bool) {
	if raw := metadata["x-amz-meta-armor-cumulative-sizes"]; raw != "" {
		var cumulative map[int]int64
		if err := json.Unmarshal([]byte(raw), &cumulative); err == nil {
			endExclusive, exists := cumulative[int(partNumber)]
			if !exists {
				return 0, 0, false
			}
			if partNumber > 1 {
				var previous bool
				start, previous = cumulative[int(partNumber-1)]
				if !previous {
					return 0, 0, false
				}
			}
			if start < 0 || endExclusive <= start || endExclusive > plaintextSize {
				return 0, 0, false
			}
			return start, endExclusive - 1, true
		}
	}

	partSize, err := strconv.ParseInt(metadata["x-amz-meta-armor-part-size"], 10, 64)
	if err != nil || partSize <= 0 {
		return 0, 0, false
	}
	start = (partNumber - 1) * partSize
	if start < 0 || start >= plaintextSize {
		return 0, 0, false
	}
	end = start + partSize - 1
	if end >= plaintextSize {
		end = plaintextSize - 1
	}
	return start, end, true
}
