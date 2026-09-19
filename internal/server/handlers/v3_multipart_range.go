package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// handleV3MultipartRangeRequest handles range requests for v3 multipart objects.
// It maps plaintext offsets to parts and blocks, fetches only what's needed,
// and returns the requested plaintext range.
func (h *Handlers) handleV3MultipartRangeRequest(w http.ResponseWriter, r *http.Request, bucket, key, prefixedKey string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time, rangeStart, rangeEnd int64) {
	ctx := r.Context()
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

	// Build the plaintext by fetching and decrypting only the needed blocks.
	// Contiguous runs of blocks are fetched with one ranged request per run:
	// the backend's pipelined reader then parallelizes the round trips, which
	// keeps a large range (hundreds of blocks) within the client's read
	// timeout. One request per block does not — see armor-817d9d92.
	var plaintextBuilder []byte

	for _, run := range rangeInfo.BlockRuns {
		partNum := sidecar.Sidecar.Parts[run.PartIdx].N

		runBody, err := h.fetchV3MultipartBlockRun(ctx, bucket, prefixedKey, sidecar, run)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch blocks (part %d, blocks %d-%d): %v", run.PartIdx, run.StartBlock, run.EndBlock, err), 500)
			return
		}

		for blockIdx := run.StartBlock; blockIdx <= run.EndBlock; blockIdx++ {
			// Slice this block's ciphertext off the run's stream
			blockLen, err := sidecar.GetBlockLength(run.PartIdx, blockIdx)
			if err != nil {
				runBody.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get block length (part %d, block %d): %v", run.PartIdx, blockIdx, err), 500)
				return
			}
			blockCiphertext := make([]byte, blockLen)
			if _, err := io.ReadFull(runBody, blockCiphertext); err != nil {
				runBody.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read block ciphertext (part %d, block %d): %v", run.PartIdx, blockIdx, err), 500)
				return
			}

			// Verify HMAC
			expectedHMAC, err := sidecar.GetBlockHMAC(run.PartIdx, blockIdx)
			if err != nil {
				runBody.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get HMAC (part %d, block %d): %v", run.PartIdx, blockIdx, err), 500)
				return
			}
			if err := verifyV3BlockHMAC(blockCiphertext, expectedHMAC, decryptor.HMACKey(), partNum, uint32(blockIdx)); err != nil {
				runBody.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("HMAC verification failed (part %d, block %d): %v", run.PartIdx, blockIdx, err), 500)
				return
			}

			// Decrypt
			decryptedBlock, err := crypto.DecryptBlockV3(decryptor.DEK(), armorMeta.IV, uint16(partNum), uint32(blockIdx), blockCiphertext, expectedHMAC, blockSize)
			if err != nil {
				runBody.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Decryption failed (part %d, block %d): %v", run.PartIdx, blockIdx, err), 500)
				return
			}

			// Decompress if needed
			isCompressed, err := sidecar.IsBlockCompressed(run.PartIdx, blockIdx)
			if err != nil {
				runBody.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to check compression (part %d, block %d): %v", run.PartIdx, blockIdx, err), 500)
				return
			}

			if isCompressed {
				decryptedBlock, err = crypto.DecompressBlock(decryptedBlock, true)
				if err != nil {
					runBody.Close()
					h.writeError(w, r, "InternalError", fmt.Sprintf("Decompression failed (part %d, block %d): %v", run.PartIdx, blockIdx, err), 500)
					return
				}
			}

			// Append to builder
			plaintextBuilder = append(plaintextBuilder, decryptedBlock...)
		}

		runBody.Close()
	}

	// Slice to the exact requested range
	resultStart := rangeInfo.FirstBlockOffset
	resultEnd := resultStart + (rangeEnd - rangeStart + 1)
	if resultEnd > int64(len(plaintextBuilder)) {
		resultEnd = int64(len(plaintextBuilder))
	}
	result := plaintextBuilder[resultStart:resultEnd]

	// Set response headers
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(result)), 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", rangeStart, rangeEnd, plaintextSize))
	w.WriteHeader(http.StatusPartialContent)
	w.Write(result)
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

// fetchV3MultipartBlockRun fetches the ciphertext of a contiguous run of
// blocks within one part with a single ranged request. The returned reader
// yields the run's ciphertext in block order; the caller reads exactly
// GetBlockLength(part, block) bytes per block and must Close the reader.
func (h *Handlers) fetchV3MultipartBlockRun(ctx context.Context, bucket, prefixedKey string, sidecar *backend.MultipartSidecarEntry, run V3BlockRun) (io.ReadCloser, error) {
	if run.PartIdx < 0 || run.PartIdx >= sidecar.PartCount() {
		return nil, fmt.Errorf("invalid part index: %d", run.PartIdx)
	}

	// Sum the previous parts' ciphertext lengths in int64: the concatenated
	// ciphertext of a large multipart object exceeds 4GiB, which silently
	// wraps a uint32 accumulator and addresses the wrong bytes.
	var partOffset int64
	for i := 0; i < run.PartIdx; i++ {
		partOffset += sidecar.Sidecar.Parts[i].CiphertextLen
	}

	// The run's ciphertext span within the part
	spanStart, spanEnd, err := sidecar.GetCiphertextSpan(run.PartIdx, run.StartBlock, run.EndBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to get block span: %w", err)
	}

	return h.backend.GetRange(ctx, bucket, prefixedKey, partOffset+spanStart, spanEnd-spanStart)
}
