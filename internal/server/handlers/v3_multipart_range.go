package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
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
	sidecar, err := h.loadV3MultipartSidecar(ctx, bucket, prefixedKey, armorMeta.ETag)
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

	// Build the plaintext by fetching and decrypting only the needed blocks
	var plaintextBuilder []byte

	for _, blockReq := range rangeInfo.BlockRequests {
		// Fetch this block's ciphertext
		blockCiphertext, err := h.fetchV3MultipartBlock(ctx, bucket, prefixedKey, sidecar, blockReq.PartIdx, blockReq.BlockIdx)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch block (part %d, block %d): %v", blockReq.PartIdx, blockReq.BlockIdx, err), 500)
			return
		}

		// Verify HMAC
		partNum := sidecar.Sidecar.Parts[blockReq.PartIdx].N
		if err := verifyV3BlockHMAC(blockCiphertext, blockReq.ExpectedHMAC, decryptor.HMACKey(), partNum, uint32(blockReq.BlockIdx)); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("HMAC verification failed (part %d, block %d): %v", blockReq.PartIdx, blockReq.BlockIdx, err), 500)
			return
		}

		// Decrypt
		decryptedBlock, err := crypto.DecryptBlockV3(decryptor.DEK(), armorMeta.IV, partNum, uint32(blockReq.BlockIdx), blockCiphertext, blockReq.ExpectedHMAC, blockSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Decryption failed (part %d, block %d): %v", blockReq.PartIdx, blockReq.BlockIdx, err), 500)
			return
		}

		// Decompress if needed
		isCompressed, err := sidecar.IsBlockCompressed(blockReq.PartIdx, blockReq.BlockIdx)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to check compression (part %d, block %d): %v", blockReq.PartIdx, blockReq.BlockIdx, err), 500)
			return
		}

		if isCompressed {
			decryptedBlock, err = crypto.DecompressBlock(decryptedBlock, true)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Decompression failed (part %d, block %d): %v", blockReq.PartIdx, blockReq.BlockIdx, err), 500)
				return
			}
		}

		// Append to builder
		plaintextBuilder = append(plaintextBuilder, decryptedBlock...)
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
	BlockRequests     []V3BlockRequest // Blocks to fetch (in order)
	FirstBlockOffset  int64             // Offset within the first block's plaintext where the range starts
}

// V3BlockRequest describes a single block to fetch for a range request.
type V3BlockRequest struct {
	PartIdx       int    // Part index (0-based)
	BlockIdx      int    // Block index within part (0-based)
	ExpectedHMAC  []byte // HMAC for this block
}

// mapV3MultipartRange maps a plaintext byte range to the parts and blocks needed.
func (h *Handlers) mapV3MultipartRange(sidecar *backend.MultipartSidecarEntry, rangeStart, rangeEnd int64, blockSize int) (*V3MultipartRangeInfo, error) {
	if rangeStart < 0 || rangeEnd < rangeStart || rangeEnd >= sidecar.TotalPlaintextSize() {
		return nil, fmt.Errorf("invalid range: %d-%d (size: %d)", rangeStart, rangeEnd, sidecar.TotalPlaintextSize())
	}

	var blockRequests []V3BlockRequest
	firstBlockOffset := int64(-1)

	// Process each part that intersects the range
	for partIdx := 0; partIdx < sidecar.PartCount(); partIdx++ {
		part := sidecar.Parts[partIdx]

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

		// Add block requests for this part
		for blockIdx := startBlock; blockIdx <= endBlock; blockIdx++ {
			expectedHMAC, err := sidecar.GetBlockHMAC(partIdx, blockIdx)
			if err != nil {
				return nil, fmt.Errorf("failed to get HMAC for part %d block %d: %w", partIdx, blockIdx, err)
			}

			blockRequests = append(blockRequests, V3BlockRequest{
				PartIdx:      partIdx,
				BlockIdx:     blockIdx,
				ExpectedHMAC: expectedHMAC,
			})

			// Track offset within first block
			if firstBlockOffset < 0 {
				blockOffsetInPart := int64(blockIdx) * int64(blockSize)
				firstBlockOffset = overlapStart - partStart - blockOffsetInPart
			}
		}
	}

	if len(blockRequests) == 0 {
		return nil, fmt.Errorf("no blocks found for range %d-%d", rangeStart, rangeEnd)
	}

	return &V3MultipartRangeInfo{
		BlockRequests:    blockRequests,
		FirstBlockOffset: firstBlockOffset,
	}, nil
}

// fetchV3MultipartBlock fetches a single block from a v3 multipart object.
// The block is identified by its part index and block index within that part.
func (h *Handlers) fetchV3MultipartBlock(ctx context.Context, bucket, prefixedKey string, sidecar *backend.MultipartSidecarEntry, partIdx, blockIdx int) ([]byte, error) {
	if partIdx < 0 || partIdx >= sidecar.PartCount() {
		return nil, fmt.Errorf("invalid part index: %d", partIdx)
	}

	part := sidecar.Parts[partIdx]
	if blockIdx < 0 || blockIdx >= len(part.Blocks) {
		return nil, fmt.Errorf("invalid block index: %d for part %d", blockIdx, partIdx)
	}

	// Calculate the byte offset of this block within the concatenated ciphertext
	// First, sum all previous parts' ciphertext lengths
	var partOffset uint32
	for i := 0; i < partIdx; i++ {
		prevPart := sidecar.Parts[i]
		partOffset += uint32(prevPart.CiphertextLen)
	}

	// Then add the offset of this block within its part
	blockOffset, err := sidecar.GetCiphertextOffset(partIdx, blockIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block offset: %w", err)
	}

	totalOffset := uint64(partOffset) + uint64(blockOffset)

	// Get the block length
	blockLen, err := sidecar.GetBlockLength(partIdx, blockIdx)
	if err != nil {
		return nil, fmt.Errorf("failed to get block length: %w", err)
	}

	// Fetch this block from B2
	blockBody, err := h.backend.GetRange(ctx, bucket, prefixedKey, int64(totalOffset), int64(blockLen))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch block (part %d, block %d): %w", partIdx, blockIdx, err)
	}
	defer blockBody.Close()

	blockCiphertext, err := io.ReadAll(blockBody)
	if err != nil {
		return nil, fmt.Errorf("failed to read block ciphertext: %w", err)
	}

	return blockCiphertext, nil
}
