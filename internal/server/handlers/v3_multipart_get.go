package handlers

import (
	"context"
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
// Integrity and termination: the request's v3 block cryptor verifies each
// block's HMAC against the sidecar before any plaintext is released, and
// Content-Length is declared from the sidecar up front. A backend error or
// HMAC mismatch after the headers are sent therefore truncates the body
// against the declared length — the client observes a broken transfer
// instead of a clean-EOF truncated success, and no XML error body is
// appended to partial plaintext.
func (h *Handlers) handleV3MultipartGet(w http.ResponseWriter, r *http.Request, bucket, key, prefixedKey string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time) {
	ctx := r.Context()

	var sidecar *backend.MultipartSidecarEntry
	var body io.ReadCloser
	var err error
	if armorMeta.CiphertextSize > 0 {
		// The authoritative ciphertext HEAD already supplied the stored size.
		// Start the sidecar and whole-ciphertext reads together so the sidecar
		// no longer adds a full round trip before the first data byte.
		parallelCtx, cancel := context.WithCancel(ctx)
		// Keep this child context alive until the returned range reader is
		// closed; canceling it immediately after GetRange returns would abort
		// the backend's read-ahead pipeline before the stream consumes it.
		type sidecarResult struct {
			sidecar *backend.MultipartSidecarEntry
			err     error
		}
		type bodyResult struct {
			body io.ReadCloser
			err  error
		}
		sidecarCh := make(chan sidecarResult, 1)
		bodyCh := make(chan bodyResult, 1)
		go func() {
			loaded, loadErr := h.loadV3MultipartSidecar(parallelCtx, bucket, key, prefixedKey, armorMeta.ETag)
			sidecarCh <- sidecarResult{sidecar: loaded, err: loadErr}
		}()
		go func() {
			loaded, loadErr := h.backend.GetRange(parallelCtx, bucket, prefixedKey, 0, armorMeta.CiphertextSize)
			bodyCh <- bodyResult{body: loaded, err: loadErr}
		}()
		sidecarRes := <-sidecarCh
		bodyRes := <-bodyCh
		if sidecarRes.err != nil {
			cancel()
			if bodyRes.body != nil {
				bodyRes.body.Close()
			}
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load v3 sidecar: %v", sidecarRes.err), 500)
			return
		}
		if bodyRes.err != nil {
			cancel()
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch v3 multipart object: %v", bodyRes.err), 500)
			return
		}
		sidecar, body = sidecarRes.sidecar, &cancelingReadCloser{ReadCloser: bodyRes.body, cancel: cancel}
	} else {
		// Direct unit-test callers may omit CiphertextSize. Keep a sidecar-first
		// fallback for that helper contract; production GetObject always sets it
		// from the authoritative HEAD.
		sidecar, err = h.loadV3MultipartSidecar(ctx, bucket, key, prefixedKey, armorMeta.ETag)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load v3 sidecar: %v", err), 500)
			return
		}
		totalCiphertext, totalErr := sidecar.TotalCiphertextSize()
		if totalErr != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Invalid v3 sidecar: %v", totalErr), 500)
			return
		}
		body, err = h.backend.GetRange(ctx, bucket, prefixedKey, 0, totalCiphertext)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to fetch v3 multipart object: %v", err), 500)
			return
		}
	}

	// The sidecar is authoritative for part boundaries. It must describe the
	// same stored object length that HEAD selected before any response byte is
	// released; otherwise a stale sidecar could pair with a different body.
	totalCiphertext, err := sidecar.TotalCiphertextSize()
	if err != nil {
		body.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("Invalid v3 sidecar: %v", err), 500)
		return
	}
	if armorMeta.CiphertextSize > 0 && totalCiphertext != armorMeta.CiphertextSize {
		body.Close()
		h.writeError(w, r, "InternalError", fmt.Sprintf("v3 sidecar ciphertext size %d does not match stored size %d", totalCiphertext, armorMeta.CiphertextSize), 500)
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

	// One cryptor and one pair of caller-owned buffers serve the whole
	// (part, block) walk: key material and the reusable HMAC state are
	// derived once, each block's HMAC is verified exactly once by the
	// cryptor, and the ciphertext and plaintext buffers are reused across
	// blocks (armor-61a35059).
	cryptor, err := decryptor.V3BlockCryptor(blockSize)
	if err != nil {
		return fmt.Errorf("v3 block cryptor: %w", err)
	}

	var ciphertextBuf, plaintextBuf []byte
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

			if needed := int(blockLen); cap(ciphertextBuf) < needed {
				ciphertextBuf = make([]byte, needed)
				plaintextBuf = make([]byte, needed)
			}
			blockCiphertext := ciphertextBuf[:blockLen]
			if _, err := io.ReadFull(body, blockCiphertext); err != nil {
				return fmt.Errorf("after %d of %d plaintext bytes: read ciphertext for part %d block %d: %w", emitted, plaintextSize, part.N, blockIdx, err)
			}

			// DecryptBlockInto verifies the block HMAC first and decrypts
			// only on a match, so unverified plaintext never reaches out.
			// The counter namespace is (part, block-within-part).
			plaintext := plaintextBuf[:blockLen]
			if err := cryptor.DecryptBlockInto(plaintext, uint16(part.N), uint32(blockIdx), blockCiphertext, expectedHMAC); err != nil {
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
