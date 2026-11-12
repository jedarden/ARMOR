package handlers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// handleV3MultipartRangeRequest handles range requests for v3 multipart objects.
// It maps plaintext offsets to parts and blocks, fetches only what's needed,
// and returns the requested plaintext range.
func (h *Handlers) handleV3MultipartRangeRequest(w http.ResponseWriter, r *http.Request, bucket, key, prefixedKey string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time, rangeStart, rangeEnd int64) {
	ctx := r.Context()

	// Parquet footer fast path (armor-c3199f1c): serve tail ranges from the
	// decrypted-footer cache, the same way the generic range path does. A hit
	// needs no sidecar and no backend round trip at all.
	if h.serveFooterCacheHit(w, r, bucket, key, armorMeta, plaintextSize, rangeStart, rangeEnd) {
		return
	}

	blockSize := armorMeta.BlockSize

	// Load the v3 sidecar (with caching)
	sidecar, err := h.loadV3MultipartSidecar(ctx, bucket, key, prefixedKey, armorMeta.ETag)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load v3 sidecar: %v", err), 500)
		return
	}

	// Map the plaintext range to parts and blocks
	rangeInfo, err := h.mapV3MultipartRange(sidecar, rangeStart, rangeEnd, blockSize)
	if err != nil {
		h.writeError(w, r, "InvalidRange", fmt.Sprintf("Invalid range: %v", err), 416)
		return
	}

	// One cryptor serves the whole range: the AES cipher, HKDF HMAC key, and
	// Reset-reusable HMAC state are derived once instead of per block, and
	// the ciphertext and plaintext buffers are caller-owned and reused
	// across blocks (armor-61a35059).
	v3Cryptor, err := decryptor.V3BlockCryptor(blockSize)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to build v3 block cryptor for range: %v", err), 500)
		return
	}

	// Fetch and decrypt only the needed blocks, emitting each one as it is
	// verified (armor-31fe90c8). Contiguous runs of blocks are fetched with
	// one ranged request per run: the backend's pipelined reader then
	// parallelizes the round trips, which keeps a large range (hundreds of
	// blocks) within the client's read timeout. One request per block does
	// not — see armor-817d9d92. The writer declares Content-Length up front;
	// a failure after the first block aborts the connection against that
	// declared length instead of returning a truncated 206 as success.
	sw := h.newRangeStreamWriter(w, r, bucket, key, armorMeta, plaintextSize, rangeStart, rangeEnd, lastModified)

	var ciphertextBuf, plainBuf []byte
	for _, run := range rangeInfo.BlockRuns {
		partNum := sidecar.Sidecar.Parts[run.PartIdx].N

		runBody, err := h.fetchV3MultipartBlockRun(ctx, bucket, prefixedKey, sidecar, run)
		if err != nil {
			sw.abort(fmt.Sprintf("Failed to fetch blocks (part %d, blocks %d-%d): %v", run.PartIdx, run.StartBlock, run.EndBlock, err))
			return
		}

		for blockIdx := run.StartBlock; blockIdx <= run.EndBlock; blockIdx++ {
			// Slice this block's ciphertext off the run's stream
			blockLen, err := sidecar.GetBlockLength(run.PartIdx, blockIdx)
			if err != nil {
				runBody.Close()
				sw.abort(fmt.Sprintf("Failed to get block length (part %d, block %d): %v", run.PartIdx, blockIdx, err))
				return
			}
			if needed := int(blockLen); cap(ciphertextBuf) < needed {
				ciphertextBuf = make([]byte, needed)
				plainBuf = make([]byte, needed)
			}
			blockCiphertext := ciphertextBuf[:blockLen]
			if _, err := io.ReadFull(runBody, blockCiphertext); err != nil {
				runBody.Close()
				sw.abort(fmt.Sprintf("Failed to read block ciphertext (part %d, block %d): %v", run.PartIdx, blockIdx, err))
				return
			}

			expectedHMAC, err := sidecar.GetBlockHMAC(run.PartIdx, blockIdx)
			if err != nil {
				runBody.Close()
				sw.abort(fmt.Sprintf("Failed to get HMAC (part %d, block %d): %v", run.PartIdx, blockIdx, err))
				return
			}

			// DecryptBlockInto verifies the block's HMAC as part of
			// decryption and releases the plaintext only on a match, so no
			// separate verification pass runs here (the single-PUT range
			// path relies on the same guarantee).
			decryptedBlock := plainBuf[:blockLen]
			if err := v3Cryptor.DecryptBlockInto(decryptedBlock, uint16(partNum), uint32(blockIdx), blockCiphertext, expectedHMAC); err != nil {
				runBody.Close()
				sw.abort(fmt.Sprintf("Decryption failed (part %d, block %d): %v", run.PartIdx, blockIdx, err))
				return
			}

			// Decompress if needed
			isCompressed, err := sidecar.IsBlockCompressed(run.PartIdx, blockIdx)
			if err != nil {
				runBody.Close()
				sw.abort(fmt.Sprintf("Failed to check compression (part %d, block %d): %v", run.PartIdx, blockIdx, err))
				return
			}

			if isCompressed {
				decryptedBlock, err = crypto.DecompressBlock(decryptedBlock, true)
				if err != nil {
					runBody.Close()
					sw.abort(fmt.Sprintf("Decompression failed (part %d, block %d): %v", run.PartIdx, blockIdx, err))
					return
				}
			}

			blockPlainOffset := sidecar.PartPrefixSums[run.PartIdx] + int64(blockIdx)*int64(blockSize)
			if err := sw.emitBlock(blockPlainOffset, decryptedBlock); err != nil {
				runBody.Close()
				log.Printf("GET %s/%s: v3 multipart range stream write failed: %v", bucket, key, err)
				return
			}
		}

		runBody.Close()
	}

	// Verifies the emitted length matches the declared range and warms the
	// Parquet footer cache from the retained tail when this read covered it.
	sw.finish()
}

// V3MultipartRangeInfo describes how to fetch a plaintext range from a v3 multipart object.
type V3MultipartRangeInfo struct {
	BlockRuns        []V3BlockRun // Contiguous runs of blocks to fetch, in order
	FirstBlockOffset int64        // Offset within the first block's plaintext where the range starts
}

// V3BlockRun describes a contiguous run of blocks within one part. The
// ciphertext of consecutive blocks is contiguous in the stored object, so a
// run is fetched with a single ranged request instead of one request per
// block.
type V3BlockRun struct {
	PartIdx    int // Part index (0-based)
	StartBlock int // First block index within the part (0-based, inclusive)
	EndBlock   int // Last block index within the part (0-based, inclusive)
}

// mapV3MultipartRange maps a plaintext byte range to the parts and blocks needed.
func (h *Handlers) mapV3MultipartRange(sidecar *backend.MultipartSidecarEntry, rangeStart, rangeEnd int64, blockSize int) (*V3MultipartRangeInfo, error) {
	if rangeStart < 0 || rangeEnd < rangeStart || rangeEnd >= sidecar.TotalPlaintextSize() {
		return nil, fmt.Errorf("invalid range: %d-%d (size: %d)", rangeStart, rangeEnd, sidecar.TotalPlaintextSize())
	}

	var blockRuns []V3BlockRun
	firstBlockOffset := int64(-1)

	// Process each part that intersects the range
	for partIdx := 0; partIdx < sidecar.PartCount(); partIdx++ {
		part := sidecar.Sidecar.Parts[partIdx]

		// Get the byte range of this part in the overall plaintext
		partStart := sidecar.PartPrefixSums[partIdx]
		partEnd := sidecar.PartPrefixSums[partIdx+1]

		// Check if this part intersects the requested range
		if rangeEnd < partStart || rangeStart >= partEnd {
			continue // No overlap
		}

		// Calculate the overlap range within this part
		overlapStart := rangeStart
		if overlapStart < partStart {
			overlapStart = partStart
		}
		overlapEnd := rangeEnd
		if overlapEnd >= partEnd {
			overlapEnd = partEnd - 1
		}

		// Map the overlap to blocks within this part
		offsetInPart := overlapStart - partStart
		startBlock := int(offsetInPart / int64(blockSize))
		endBlock := int((overlapEnd - partStart) / int64(blockSize))

		// Clamp to valid block range
		if startBlock < 0 {
			startBlock = 0
		}
		if endBlock >= len(part.Blocks) {
			endBlock = len(part.Blocks) - 1
		}

		// Track offset within first block
		if firstBlockOffset < 0 {
			blockOffsetInPart := int64(startBlock) * int64(blockSize)
			firstBlockOffset = overlapStart - partStart - blockOffsetInPart
		}

		// Merge this part's blocks into a single contiguous run: blocks are
		// generated in ascending order and parts are processed in order, so a
		// run only ever extends forward or starts fresh.
		if n := len(blockRuns); n > 0 && blockRuns[n-1].PartIdx == partIdx && blockRuns[n-1].EndBlock == startBlock-1 {
			blockRuns[n-1].EndBlock = endBlock
		} else {
			blockRuns = append(blockRuns, V3BlockRun{
				PartIdx:    partIdx,
				StartBlock: startBlock,
				EndBlock:   endBlock,
			})
		}
	}

	if len(blockRuns) == 0 {
		return nil, fmt.Errorf("no blocks found for range %d-%d", rangeStart, rangeEnd)
	}

	return &V3MultipartRangeInfo{
		BlockRuns:        blockRuns,
		FirstBlockOffset: firstBlockOffset,
	}, nil
}

// v3MultipartPartRange returns the plaintext span for an S3 GetObject
// partNumber request. V3 parts may have different sizes, so the sidecar's
// prefix sums—not the uniform-part metadata used by legacy v2 objects—are
// authoritative.
func v3MultipartPartRange(sidecar *backend.MultipartSidecarEntry, partNumber int64) (start, end int64, ok bool) {
	if sidecar == nil || sidecar.Sidecar == nil || partNumber < 1 || partNumber > 10000 {
		return 0, 0, false
	}
	for partIdx, part := range sidecar.Sidecar.Parts {
		if int64(part.N) != partNumber || partIdx+1 >= len(sidecar.PartPrefixSums) {
			continue
		}
		start = sidecar.PartPrefixSums[partIdx]
		endExclusive := sidecar.PartPrefixSums[partIdx+1]
		if start < 0 || endExclusive <= start || endExclusive > sidecar.TotalPlaintextSize() {
			return 0, 0, false
		}
		return start, endExclusive - 1, true
	}
	return 0, 0, false
}

// fetchV3MultipartBlockRun fetches the ciphertext of a contiguous run of
// blocks within one part with a single ranged request. The returned reader
// yields the run's ciphertext in block order; the caller reads exactly
// GetBlockLength(part, block) bytes per block and must Close the reader.
func (h *Handlers) fetchV3MultipartBlockRun(ctx context.Context, bucket, prefixedKey string, sidecar *backend.MultipartSidecarEntry, run V3BlockRun) (io.ReadCloser, error) {
	if run.PartIdx < 0 || run.PartIdx >= sidecar.PartCount() {
		return nil, fmt.Errorf("invalid part index: %d", run.PartIdx)
	}

	// The part's offset in the concatenated object comes from the sidecar's
	// precomputed, overflow-checked cumulative ciphertext sums — int64, since
	// a large multipart object's total exceeds 4GiB and a wrapping
	// accumulator addresses the wrong bytes (armor-817d9d92).
	partOffset, err := sidecar.GetPartCiphertextOffset(run.PartIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to get part offset: %w", err)
	}

	// The run's ciphertext span within the part
	spanStart, spanEnd, err := sidecar.GetCiphertextSpan(run.PartIdx, run.StartBlock, run.EndBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to get block span: %w", err)
	}

	return h.backend.GetRange(ctx, bucket, prefixedKey, partOffset+spanStart, spanEnd-spanStart)
}
