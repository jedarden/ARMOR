// Package handlers provides v3 range request support
package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// handleV3RangeRequest handles range requests for v3 objects.
// It fetches the trailer block table (through the generation-keyed decoded
// table cache, armor-c3199f1c), maps the requested plaintext range to blocks
// once, and fetches the whole needed ciphertext span with a single ranged
// request: single-PUT blocks are contiguous in the stored object, so the
// backend's bounded pipelined reader parallelizes the origin round trips
// instead of the handler paying one synchronous round trip per 64KiB block
// (armor-5694713b; same shape as the multipart runs in armor-817d9d92).
// storedGen is the verified stored-object generation this request's
// authoritative HEAD produced; the table cache is keyed on it so an overwrite
// or re-encryption can never be served a foreign table.
func (h *Handlers) handleV3RangeRequest(w http.ResponseWriter, r *http.Request, bucket, key, prefixedKey string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize, ciphertextSize int64, lastModified time.Time, start, end int64, storedGen v3TableKeyParts) {
	ctx := r.Context()

	// Parquet footer fast path (armor-c3199f1c): serve tail ranges from the
	// decrypted-footer cache, the same way the generic range path does. A hit
	// needs no block table and no backend round trip at all.
	if h.serveFooterCacheHit(w, r, bucket, key, armorMeta, plaintextSize, start, end) {
		return
	}

	blockSize := armorMeta.BlockSize
	blockCount := int(crypto.ComputeBlockCount(plaintextSize, blockSize))

	// Fetch the trailer block table (cached by stored generation)
	blockTable, err := h.loadV3BlockTable(ctx, bucket, prefixedKey, storedGen, ciphertextSize, blockCount)
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

	// Decrypt and emit one block at a time (armor-31fe90c8): each block is
	// authenticated by DecryptBlockV3 (which verifies the block's HMAC before
	// releasing any plaintext; no separate HMAC pass) and written to the
	// client as soon as it is verified, so in-process memory stays at one
	// block's ciphertext plus one block's plaintext however long the range
	// is. The writer declares Content-Length up front; a failure after the
	// first block aborts the connection against that declared length instead
	// of returning a truncated 206 as success.
	sw := h.newRangeStreamWriter(w, r, bucket, key, armorMeta, plaintextSize, start, end, lastModified)
	for blockIndex := startBlock; blockIndex <= endBlock; blockIndex++ {
		entry := blockTable.Entries[blockIndex]
		isCompressed := entry.IsCompressed()
		ciphertextLen := entry.RawLength()

		blockCiphertext := make([]byte, ciphertextLen)
		if _, err := io.ReadFull(spanBody, blockCiphertext); err != nil {
			sw.abort(fmt.Sprintf("Failed to read v3 block %d from fetched span: %v", blockIndex, err))
			return
		}

		// Decrypt block using v3 counter construction (verifies the block's
		// HMAC first)
		decrypted, err := crypto.DecryptBlockV3(decryptor.DEK(), armorMeta.IV, 0, uint32(blockIndex), blockCiphertext, entry.HMAC[:], blockSize)
		if err != nil {
			sw.abort(fmt.Sprintf("v3 block %d authentication/decryption failed: %v", blockIndex, err))
			return
		}

		// Decompress if flagged
		if isCompressed {
			decrypted, err = crypto.DecompressBlock(decrypted, true)
			if err != nil {
				sw.abort(fmt.Sprintf("v3 block %d decompression failed: %v", blockIndex, err))
				return
			}
		}

		if err := sw.emitBlock(int64(blockIndex)*int64(blockSize), decrypted); err != nil {
			log.Printf("GET %s/%s: v3 range stream write failed: %v", bucket, key, err)
			return
		}
	}
	// Verifies the emitted length matches the declared range and warms the
	// Parquet footer cache from the retained tail when this read covered it.
	sw.finish()
}
