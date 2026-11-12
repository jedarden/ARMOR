package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
	"github.com/jedarden/armor/internal/manifest"
)

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
		if len(state.CumulativePartSizes) > 0 {
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
