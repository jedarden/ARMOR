// Package handlers provides v3 range request support
package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// handleV3RangeRequest handles range requests for v3 objects.
// It fetches the trailer block table, maps the requested plaintext range to
// blocks once, and fetches the whole needed ciphertext span with a single
// ranged request: single-PUT blocks are contiguous in the stored object, so
// the backend's bounded pipelined reader parallelizes the origin round trips
// instead of the handler paying one synchronous round trip per 64KiB block
// (armor-5694713b; same shape as the multipart runs in armor-817d9d92).
func (h *Handlers) handleV3RangeRequest(w http.ResponseWriter, r *http.Request, bucket, key, prefixedKey string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize, ciphertextSize int64, lastModified time.Time, start, end int64) {
	ctx := r.Context()
	blockSize := armorMeta.BlockSize
	blockCount := int(crypto.ComputeBlockCount(plaintextSize, blockSize))

	// Fetch the trailer block table
	blockTable, err := h.readV3BlockTable(ctx, bucket, prefixedKey, ciphertextSize, blockCount)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read v3 block table for range: %v", err), 500)
		return
	}

	// Calculate which blocks are needed for this range
	startBlock := int(start / int64(blockSize))
	endBlock := int(end / int64(blockSize))
	if endBlock >= blockCount {
		endBlock = blockCount - 1
	}
	if startBlock >= blockTable.EntryCount() {
		h.writeError(w, r, "InternalError", fmt.Sprintf("v3 range start block %d beyond block table (%d entries)", startBlock, blockTable.EntryCount()), 500)
		return
	}
	if endBlock >= blockTable.EntryCount() {
		endBlock = blockTable.EntryCount() - 1
	}

	// The needed blocks are contiguous in the stored object: one bounded
	// 64-bit span covers them. The table's offsets are summed once at decode
	// time, so no per-block O(n) resumming (and no uint32 wrap above 4GiB).
	spanStart, spanEnd, ok := blockTable.CiphertextSpan(startBlock, endBlock)
	if !ok {
		h.writeError(w, r, "InternalError", fmt.Sprintf("v3 range blocks %d-%d outside block table (%d entries)", startBlock, endBlock, blockTable.EntryCount()), 500)
		return
	}
	spanStart += crypto.HeaderSize
	spanEnd += crypto.HeaderSize
	if spanEnd > ciphertextSize {
		h.writeError(w, r, "InternalError", fmt.Sprintf("v3 block table spans past ciphertext end: %d > %d", spanEnd, ciphertextSize), 500)
		return
	}

	// Fetch the whole span; the backend pipelines the origin ranged reads
	// within its own bounded window.
	spanBody, err := h.backend.GetRange(ctx, bucket, prefixedKey, spanStart, spanEnd-spanStart)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch v3 blocks %d-%d: %v", startBlock, endBlock, err), 500)
		return
	}
	defer spanBody.Close()

	// Build the plaintext by slicing each block's ciphertext off the span and
	// decrypting it. DecryptBlockV3 authenticates each original ARMOR block
	// exactly once before any plaintext is released; the handler performs no
	// separate HMAC pass.
	var plaintextBuilder []byte
	for blockIndex := startBlock; blockIndex <= endBlock; blockIndex++ {
		entry := blockTable.Entries[blockIndex]
		isCompressed := entry.IsCompressed()
		ciphertextLen := entry.RawLength()

		blockCiphertext := make([]byte, ciphertextLen)
		if _, err := io.ReadFull(spanBody, blockCiphertext); err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read v3 block %d from fetched span: %v", blockIndex, err), 500)
			return
		}

		// Decrypt block using v3 counter construction (verifies the block's
		// HMAC first)
		decrypted, err := crypto.DecryptBlockV3(decryptor.DEK(), armorMeta.IV, 0, uint32(blockIndex), blockCiphertext, entry.HMAC[:], blockSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("v3 block %d authentication/decryption failed: %v", blockIndex, err), 500)
			return
		}

		// Decompress if flagged
		if isCompressed {
			decrypted, err = crypto.DecompressBlock(decrypted, true)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("v3 block %d decompression failed: %v", blockIndex, err), 500)
				return
			}
		}

		// Append to builder
		plaintextBuilder = append(plaintextBuilder, decrypted...)
	}

	// Slice to the requested range
	startOffsetInBlock := start % int64(blockSize)
	endOffsetInBlock := end % int64(blockSize)
	var plaintext []byte
	if startBlock == endBlock {
		// Range is within a single block
		if int64(len(plaintextBuilder)) <= endOffsetInBlock {
			h.writeError(w, r, "InternalError", fmt.Sprintf("v3 block %d decrypted short: %d bytes", startBlock, len(plaintextBuilder)), 500)
			return
		}
		plaintext = plaintextBuilder[startOffsetInBlock : endOffsetInBlock+1]
	} else {
		// Range spans multiple blocks
		if int64(len(plaintextBuilder)) <= startOffsetInBlock {
			h.writeError(w, r, "InternalError", fmt.Sprintf("v3 blocks %d-%d decrypted short: %d bytes", startBlock, endBlock, len(plaintextBuilder)), 500)
			return
		}
		plaintext = plaintextBuilder[startOffsetInBlock:]
		expectedLen := end - start + 1
		if int64(len(plaintext)) > expectedLen {
			plaintext = plaintext[:expectedLen]
		}
	}

	// Set response headers
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(plaintext)), 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, plaintextSize))
	w.WriteHeader(http.StatusPartialContent)
	w.Write(plaintext)
}
