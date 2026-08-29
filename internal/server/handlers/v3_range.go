// Package handlers provides v3 range request support
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/jedarden/armor/internal/crypto"
)

// handleV3RangeRequest handles range requests for v3 objects.
// It fetches the trailer block table, decrypts only the needed blocks,
// and returns the requested range.
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

	// Build the plaintext by decrypting each needed block and slicing
	var plaintextBuilder []byte
	for blockIndex := startBlock; blockIndex <= endBlock; blockIndex++ {
		if blockIndex >= blockTable.EntryCount() {
			break
		}

		entry := blockTable.Entries[blockIndex]
		isCompressed := entry.IsCompressed()
		ciphertextLen := entry.RawLength()

		// Calculate offset for this block using prefix sums
		var blockOffset uint32
		for i := 0; i < blockIndex; i++ {
			blockOffset += blockTable.Entries[i].RawLength()
		}
		blockOffsetInFile := crypto.HeaderSize + int64(blockOffset)

		// Fetch this specific block
		encryptedBody, err := h.backend.GetRange(ctx, bucket, prefixedKey, blockOffsetInFile, int64(ciphertextLen))
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch v3 block %d: %v", blockIndex, err), 500)
			return
		}

		encryptedBuf, err := io.ReadAll(encryptedBody)
		encryptedBody.Close()
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read v3 block %d: %v", blockIndex, err), 500)
			return
		}

		// Verify HMAC using v3 format
		hmacKey := decryptor.HMACKey()
		expectedHMAC := entry.HMAC[:]
		mac := hmac.New(sha256.New, hmacKey)
		partBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(partBytes, 0) // part=0 for single-PUT
		mac.Write(partBytes)
		blockBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(blockBytes, uint32(blockIndex))
		mac.Write(blockBytes)
		mac.Write(encryptedBuf)
		computed := mac.Sum(nil)

		if !hmac.Equal(computed, expectedHMAC) {
			h.writeError(w, r, "InternalError", fmt.Sprintf("v3 block %d: HMAC verification failed", blockIndex), 500)
			return
		}

		// Decrypt block using v3 counter construction
		decrypted, err := crypto.DecryptBlockV3(decryptor.DEK(), armorMeta.IV, 0, uint32(blockIndex), encryptedBuf, expectedHMAC, blockSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("v3 block %d decryption failed: %v", blockIndex, err), 500)
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
		plaintext = plaintextBuilder[startOffsetInBlock : endOffsetInBlock+1]
	} else {
		// Range spans multiple blocks
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
