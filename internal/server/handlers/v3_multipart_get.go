package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// handleV3MultipartGet handles full GET requests for v3 multipart objects.
//
// The parts of a v3 multipart object are contiguous in the stored ciphertext,
// so the whole object is fetched with ONE ranged request and decrypted block
// by block as the bytes arrive. The backend's bounded pipelined range reader
// (ARMOR_READ_CONCURRENCY window over ARMOR_READ_FETCH_SIZE sub-requests)
// provides the read-ahead — across part boundaries too, since storage has no
// part boundaries — while in-process memory stays at one ciphertext block plus
// one plaintext block, never a whole part. Time-to-first-byte is the first
// fetched chunk, not the first full part (armor-272299ca).
//
// Integrity and termination: DecryptBlockV3 verifies each block's HMAC against
// the sidecar before any plaintext is released, and Content-Length is declared
// from the sidecar up front. A backend error or HMAC mismatch after the
// headers are sent therefore truncates the body against the declared length —
// the client observes a broken transfer instead of a clean-EOF truncated
// success, and no XML error body is appended to partial plaintext.
func (h *Handlers) handleV3MultipartGet(w http.ResponseWriter, r *http.Request, bucket, key, prefixedKey string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time) {
	ctx := r.Context()

	// Load the v3 sidecar (with caching)
	sidecar, err := h.loadV3MultipartSidecar(ctx, bucket, key, prefixedKey, armorMeta.ETag)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load v3 sidecar: %v", err), 500)
		return
	}

	// Fetch the whole concatenated ciphertext. The part offsets in the sidecar
	// were summed once with checked int64 addition at sidecar load, so the
	// address is correct past 4GiB (armor-817d9d92) and a sidecar whose
	// offsets overflow is rejected before any byte is served.
	totalCiphertext, err := sidecar.TotalCiphertextSize()
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Invalid v3 sidecar: %v", err), 500)
		return
	}
	body, err := h.backend.GetRange(ctx, bucket, prefixedKey, 0, totalCiphertext)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch v3 multipart object: %v", err), 500)
		return
	}

	// Set response headers for streaming. Declaring Content-Length is what
	// makes a later failure observable: net/http closes the connection on a
	// short body instead of letting it read as a truncated success.
	w.Header().Set("Content-Length", strconv.FormatInt(plaintextSize, 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("X-Armor-Stream", "pipelined")
	w.WriteHeader(http.StatusOK)

	pr, pw := io.Pipe()
	// Unblocks the producer parked on a pw.Write if io.Copy stops early —
	// a client disconnect must not strand the streaming goroutine (and with
	// it the backend reader) forever.
	defer pr.Close()

	go func() {
		// The streaming goroutine is the only reader of body, so it owns the
		// close — including on the cancellation path, where it must observe
		// the read failure before the resources go away.
		defer body.Close()
		if err := streamV3MultipartBlocks(pw, body, sidecar, decryptor, armorMeta.IV, armorMeta.BlockSize, plaintextSize); err != nil {
			log.Printf("GET %s/%s: v3 multipart stream failed: %v", bucket, key, err)
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	if _, err := io.Copy(w, pr); err != nil {
		// Headers are already sent: the declared-but-unmet Content-Length is
		// the client-observable failure. Log the cause for the operator.
		log.Printf("GET %s/%s: v3 multipart plaintext streaming error: %v", bucket, key, err)
	}
}

// streamV3MultipartBlocks walks the sidecar's (part, block) sequence in order,
// reading each block's ciphertext from body, verifying its HMAC and decrypting
// it with the part-scoped v3 counter namespace, and writing the plaintext to
// out. Blocks are emitted only after verification; a read, verification, or
// write failure is returned with the plaintext position it happened at.
func streamV3MultipartBlocks(out io.Writer, body io.Reader, sidecar *backend.MultipartSidecarEntry, decryptor *crypto.Decryptor, iv []byte, blockSize int, plaintextSize int64) error {
	var emitted int64

	for partIdx := 0; partIdx < sidecar.PartCount(); partIdx++ {
		part := sidecar.Sidecar.Parts[partIdx]

		for blockIdx := 0; blockIdx < len(part.Blocks); blockIdx++ {
			blockLen, err := sidecar.GetBlockLength(partIdx, blockIdx)
			if err != nil {
				return fmt.Errorf("after %d of %d plaintext bytes: get block length (part %d, block %d): %w", emitted, plaintextSize, part.N, blockIdx, err)
			}
			expectedHMAC, err := sidecar.GetBlockHMAC(partIdx, blockIdx)
			if err != nil {
				return fmt.Errorf("after %d of %d plaintext bytes: get block HMAC (part %d, block %d): %w", emitted, plaintextSize, part.N, blockIdx, err)
			}
			isCompressed, err := sidecar.IsBlockCompressed(partIdx, blockIdx)
			if err != nil {
				return fmt.Errorf("after %d of %d plaintext bytes: get compression flag (part %d, block %d): %w", emitted, plaintextSize, part.N, blockIdx, err)
			}

			blockCiphertext := make([]byte, blockLen)
			if _, err := io.ReadFull(body, blockCiphertext); err != nil {
				return fmt.Errorf("after %d of %d plaintext bytes: read ciphertext for part %d block %d: %w", emitted, plaintextSize, part.N, blockIdx, err)
			}

			// DecryptBlockV3 verifies the block HMAC first and decrypts only
			// on a match, so unverified plaintext never reaches out. The
			// counter namespace is (part, block-within-part).
			plaintext, err := crypto.DecryptBlockV3(decryptor.DEK(), iv, uint16(part.N), uint32(blockIdx), blockCiphertext, expectedHMAC, blockSize)
			if err != nil {
				return fmt.Errorf("after %d of %d plaintext bytes: part %d block %d: %w", emitted, plaintextSize, part.N, blockIdx, err)
			}

			if isCompressed {
				plaintext, err = crypto.DecompressBlock(plaintext, true)
				if err != nil {
					return fmt.Errorf("after %d of %d plaintext bytes: decompress part %d block %d: %w", emitted, plaintextSize, part.N, blockIdx, err)
				}
			}

			if _, err := out.Write(plaintext); err != nil {
				return fmt.Errorf("after %d of %d plaintext bytes: emit part %d block %d: %w", emitted, plaintextSize, part.N, blockIdx, err)
			}
			emitted += int64(len(plaintext))
		}
	}

	if emitted != plaintextSize {
		return fmt.Errorf("streamed %d plaintext bytes, sidecar declares %d", emitted, plaintextSize)
	}
	return nil
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
