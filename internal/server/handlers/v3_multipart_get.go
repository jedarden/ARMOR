package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// handleV3MultipartGet handles full GET requests for v3 multipart objects.
// It streams parts in order, verifying HMACs and decrypting each block.
func (h *Handlers) handleV3MultipartGet(w http.ResponseWriter, r *http.Request, bucket, key, prefixedKey string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time) {
	ctx := r.Context()

	// Load the v3 sidecar (with caching)
	sidecar, err := h.loadV3MultipartSidecar(ctx, bucket, key, prefixedKey, armorMeta.ETag)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load v3 sidecar: %v", err), 500)
		return
	}

	// Set response headers for streaming
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))

	// Stream parts in order
	for partIdx := 0; partIdx < sidecar.PartCount(); partIdx++ {
		part := sidecar.Sidecar.Parts[partIdx]

		// Fetch this part's ciphertext from B2
		partCiphertext, err := h.fetchPartCiphertext(ctx, bucket, prefixedKey, sidecar, partIdx)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch part %d: %v", part.N, err), 500)
			return
		}

		// Decrypt and verify this part
		partPlaintext, err := h.decryptV3Part(ctx, partCiphertext, sidecar, partIdx, decryptor, armorMeta.IV, armorMeta.BlockSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to decrypt part %d: %v", part.N, err), 500)
			return
		}

		// Write this part's plaintext to the response
		if _, err := w.Write(partPlaintext); err != nil {
			// Client disconnect
			return
		}
	}
}

// loadV3MultipartSidecar loads the v3 multipart sidecar with caching.
//
// The sidecar is named sha256 of the CLIENT key, because CompleteMultipartUpload
// calls SaveHMACTableV3 with the client key while the assembled ciphertext is
// stored under the prefixed key. Both forms are therefore required: key names
// the sidecar, prefixedKey addresses the ciphertext and keys the cache.
func (h *Handlers) loadV3MultipartSidecar(ctx context.Context, bucket, key, prefixedKey, etag string) (*backend.MultipartSidecarEntry, error) {
	// Check cache first
	if cached := h.multipartSidecarCache.Get(bucket, prefixedKey, etag); cached != nil {
		return cached, nil
	}

	// LoadHMACTableV3 derives the sidecar key from the object key itself.
	// Passing an already-derived sidecar key would hash it a second time.
	manager := h.getMultipartManager(bucket)
	sidecar, err := manager.LoadHMACTableV3(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load v3 sidecar from B2: %w", err)
	}

	// Build and cache the entry
	if err := h.multipartSidecarCache.Set(bucket, prefixedKey, etag, sidecar); err != nil {
		return nil, fmt.Errorf("failed to cache sidecar: %w", err)
	}

	return h.multipartSidecarCache.Get(bucket, prefixedKey, etag), nil
}

// fetchPartCiphertext fetches the ciphertext for a specific part of a v3 multipart object.
// The parts are concatenated in the final object, so we need to calculate the offset.
func (h *Handlers) fetchPartCiphertext(ctx context.Context, bucket, prefixedKey string, sidecar *backend.MultipartSidecarEntry, partIdx int) ([]byte, error) {
	if partIdx < 0 || partIdx >= sidecar.PartCount() {
		return nil, fmt.Errorf("invalid part index: %d", partIdx)
	}

	part := sidecar.Sidecar.Parts[partIdx]

	// Calculate the byte offset of this part in the concatenated ciphertext
	// For v3 multipart, B2 simply concatenates all parts
	var partOffset uint32
	for i := 0; i < partIdx; i++ {
		prevPart := sidecar.Sidecar.Parts[i]
		partOffset += uint32(prevPart.CiphertextLen)
	}

	// Fetch this part's range from B2
	partCiphertext, err := h.backend.GetRange(ctx, bucket, prefixedKey, int64(partOffset), part.CiphertextLen)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch part %d range: %w", part.N, err)
	}
	defer partCiphertext.Close()

	ciphertext, err := io.ReadAll(partCiphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to read part %d ciphertext: %w", part.N, err)
	}

	return ciphertext, nil
}

// decryptV3Part decrypts and verifies a single v3 multipart part.
// It splits the part into blocks, verifies each block's HMAC, decrypts with v3 counter construction,
// and decompresses if needed.
func (h *Handlers) decryptV3Part(ctx context.Context, partCiphertext []byte, sidecar *backend.MultipartSidecarEntry, partIdx int, decryptor *crypto.Decryptor, iv []byte, blockSize int) ([]byte, error) {
	if partIdx < 0 || partIdx >= sidecar.PartCount() {
		return nil, fmt.Errorf("invalid part index: %d", partIdx)
	}

	part := sidecar.Sidecar.Parts[partIdx]
	partNum := part.N // 1-based part number for v3 counter construction

	// Split the part into blocks and process each one
	var plaintextBuilder []byte
	ciphertextOffset := 0

	for blockIdx := 0; blockIdx < len(part.Blocks); blockIdx++ {
		// Get the block length from the sidecar
		blockLen, err := sidecar.GetBlockLength(partIdx, blockIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to get block %d length: %w", blockIdx, err)
		}

		// Extract this block from the ciphertext
		if ciphertextOffset+int(blockLen) > len(partCiphertext) {
			return nil, fmt.Errorf("block %d extends beyond part ciphertext", blockIdx)
		}
		blockCiphertext := partCiphertext[ciphertextOffset : ciphertextOffset+int(blockLen)]
		ciphertextOffset += int(blockLen)

		// Verify HMAC for this block
		expectedHMAC, err := sidecar.GetBlockHMAC(partIdx, blockIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to get block %d HMAC: %w", blockIdx, err)
		}

		if err := verifyV3BlockHMAC(blockCiphertext, expectedHMAC, decryptor.HMACKey(), partNum, uint32(blockIdx)); err != nil {
			return nil, fmt.Errorf("block %d HMAC verification failed: %w", blockIdx, err)
		}

		// Decrypt this block using v3 counter construction
		decryptedBlock, err := crypto.DecryptBlockV3(decryptor.DEK(), iv, uint16(partNum), uint32(blockIdx), blockCiphertext, expectedHMAC, blockSize)
		if err != nil {
			return nil, fmt.Errorf("block %d decryption failed: %w", blockIdx, err)
		}

		// Decompress if flagged
		isCompressed, err := sidecar.IsBlockCompressed(partIdx, blockIdx)
		if err != nil {
			return nil, fmt.Errorf("failed to check compression flag for block %d: %w", blockIdx, err)
		}

		if isCompressed {
			decryptedBlock, err = crypto.DecompressBlock(decryptedBlock, true)
			if err != nil {
				return nil, fmt.Errorf("block %d decompression failed: %w", blockIdx, err)
			}
		}

		// Append to plaintext
		plaintextBuilder = append(plaintextBuilder, decryptedBlock...)
	}

	return plaintextBuilder, nil
}

// verifyV3BlockHMAC verifies the HMAC for a v3 block.
// V3 HMAC format: HMAC-SHA256(hmacKey, uint16(part) ‖ uint32(blockIdx) ‖ ciphertext)
func verifyV3BlockHMAC(blockCiphertext, expectedHMAC, hmacKey []byte, partNum int, blockIdx uint32) error {
	mac := hmac.New(sha256.New, hmacKey)

	// Write part number (big-endian uint16)
	partBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(partBytes, uint16(partNum))
	mac.Write(partBytes)

	// Write block index (big-endian uint32)
	blockBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(blockBytes, blockIdx)
	mac.Write(blockBytes)

	// Write ciphertext
	mac.Write(blockCiphertext)

	computed := mac.Sum(nil)

	if !hmac.Equal(computed, expectedHMAC) {
		return fmt.Errorf("HMAC mismatch for part %d block %d", partNum, blockIdx)
	}

	return nil
}

// getV3MultipartContentLength returns the total plaintext size for a v3 multipart object.
func (h *Handlers) getV3MultipartContentLength(sidecar *backend.MultipartSidecarEntry) int64 {
	return sidecar.TotalPlaintextSize()
}
