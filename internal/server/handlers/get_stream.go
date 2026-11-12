package handlers

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// cancelingReadCloser keeps the parallel fetch context alive while the caller
// consumes the returned pipeline, then releases it with the reader's normal
// Close. Canceling before return would abort the B2 read-ahead reader itself.
type cancelingReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelingReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.cancel()
	return err
}

// handleFullObjectStream handles full object downloads with pipelined stream decryption.
// This uses io.Pipe to decrypt blocks as they stream from Cloudflare, reducing
// time-to-first-byte and memory usage compared to buffering the entire envelope.
// header is the request-local envelope header GetObject already read for
// single-PUT objects (nil for multipart objects, which have none); the stream
// path reuses it instead of re-reading the same 64 bytes from the backend.
// storedGen is the verified stored-object generation for cache keying.
// singleFetch is the whole-ciphertext buffer GetObject took for a small
// single-PUT object (armor-98864f12); nil when the object streams. When set,
// the table and data regions are sliced out of it locally instead of fetched.
func (h *Handlers) handleFullObjectStream(w http.ResponseWriter, r *http.Request, bucket, key string, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, plaintextSize int64, lastModified time.Time, isMultipart bool, multipartPartSize int64, isNonUniform bool, cumulativePartSizes map[int]int64, header *crypto.EnvelopeHeader, storedGen v3TableKeyParts, singleFetch []byte) {
	ctx := r.Context()
	if plaintextSize == 0 {
		// Empty v3 objects have a header and an empty trailer, so there is no
		// data or block table range to fetch. Return the authenticated empty
		// representation directly.
		w.Header().Set("Content-Length", "0")
		w.Header().Set("Content-Type", armorMeta.ContentType)
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
		return
	}

	// Apply prefix for backend operations
	prefixedKey := h.applyPrefix(key)

	blockSize := armorMeta.BlockSize
	blockCount := int(crypto.ComputeBlockCount(plaintextSize, blockSize))

	var hmacTable []byte
	var dataBody io.ReadCloser
	var streamSize int64

	if isMultipart {
		// Multipart object: HMAC table is in sidecar, no embedded header
		manager := h.getMultipartManager(bucket)
		sidecar, err := manager.LoadHMACTable(ctx, key)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to load HMAC table from sidecar: %v", err), 500)
			return
		}

		// Concatenate all block HMACs from sidecar
		for _, hmac := range sidecar.BlockHMACs {
			hmacTable = append(hmacTable, hmac...)
		}

		// Data starts at offset 0 (no header), read only the encrypted data
		streamSize = plaintextSize
		dataBody, err = h.backend.GetRange(ctx, bucket, prefixedKey, 0, streamSize)
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object stream: %v", err), 500)
			return
		}
		defer dataBody.Close()
	} else {
		// Single-PUT object: dispatch on version for HMAC/block table format
		// v1/v2: inline HMAC table at HeaderSize + plaintextSize
		// v3: trailer block table at ciphertext_length - 36*blockCount
		//
		// Range and compressed paths decode the envelope header in GetObject;
		// an uncached uncompressed v3 full GET deliberately leaves it nil so the
		// header can arrive as the prefix of the data stream.

		if singleFetch != nil {
			if header == nil {
				h.writeError(w, r, "InternalError", "missing envelope header for buffered single-PUT stream", 500)
				return
			}
			// Small single-PUT object: the whole ciphertext is already local
			// (armor-98864f12). Slice the table and data regions out of the
			// buffer instead of two more round trips. Every layout invariant
			// is checked against the buffer bounds here, so a table that
			// disagrees with the fetched length, a shifted layout, or a
			// wrong-sized envelope fails the request before any plaintext
			// exists — the same failure the streaming path would reach on
			// its own reads, one round trip earlier.
			var dataEnd int64
			if header.Version == crypto.Version3 {
				blockTable, err := decodeV3BlockTableFromBuffer(singleFetch, armorMeta.CiphertextSize, blockCount, armorMeta.Compressed, header.BlockSize())
				if err != nil {
					h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read v3 block table: %v", err), 500)
					return
				}
				// Whole-object compression produces a table for the compressed
				// payload, which can have fewer blocks than the original
				// plaintext declared by the envelope header.
				blockCount = blockTable.EntryCount()
				if blockCount > 0 {
					hmacTable = encodeV3BlockTableForTransport(blockTable)
				}
				h.cacheV3DecodedTable(bucket, prefixedKey, storedGen, blockTable)
				h.cacheV3Header(bucket, prefixedKey, storedGen, header)
				// blockCount == 0 (an empty envelope): the transport encoder
				// refuses an empty table by design, and the read loop below
				// runs zero iterations either way, so no table is transported.
				dataEnd = int64(crypto.HeaderSize) + blockTable.TotalCiphertextLength64()
			} else {
				hmacTableOffset := crypto.HeaderSize + plaintextSize
				hmacTableSize := int64(blockCount) * crypto.HMACSize
				if hmacTableOffset < crypto.HeaderSize || hmacTableOffset+hmacTableSize > int64(len(singleFetch)) {
					h.writeError(w, r, "InternalError", "stored object shorter than its inline HMAC table", 500)
					return
				}
				hmacTable = singleFetch[hmacTableOffset : hmacTableOffset+hmacTableSize]
				dataEnd = hmacTableOffset
			}
			dataBody = io.NopCloser(bytes.NewReader(singleFetch[crypto.HeaderSize:dataEnd]))
			defer dataBody.Close()
		} else if armorMeta.Version == crypto.Version3 {
			// v3: fetch the trailer block table and the header+data stream in
			// parallel. Uncompressed full GETs decode the header directly from
			// that stream; compressed v3 retains the bounded whole-envelope
			// fallback because its trailer length is not derivable up front.
			if armorMeta.CiphertextSize == 0 {
				h.writeError(w, r, "InternalError", "v3 object missing ciphertext size", 500)
				return
			}

			var (
				blockTable *crypto.BlockTable
				err        error
			)
			if armorMeta.Compressed {
				if header == nil {
					h.writeError(w, r, "InternalError", "missing envelope header for compressed v3 stream", 500)
					return
				}
				blockTable, err = h.loadV3CompressedBlockTable(ctx, bucket, prefixedKey, armorMeta.CiphertextSize, blockCount, header.BlockSize())
			} else {
				if header == nil {
					dataBody, blockTable, header, err = h.loadV3FullStream(ctx, bucket, prefixedKey, storedGen, armorMeta.CiphertextSize, blockCount, armorMeta)
				} else {
					blockTable, err = h.loadV3BlockTable(ctx, bucket, prefixedKey, storedGen, armorMeta.CiphertextSize, blockCount)
				}
			}
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read v3 block table: %v", err), 500)
				return
			}
			blockCount = blockTable.EntryCount()

			if dataBody == nil {
				// Header was already read before dispatch (compressed or a
				// direct helper call), so open the exact header+data span and
				// discard only the duplicate header bytes.
				streamSize = int64(crypto.HeaderSize) + int64(blockTable.TotalCiphertextLength())
				dataBody, err = h.backend.GetRange(ctx, bucket, prefixedKey, 0, streamSize)
				if err != nil {
					h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object stream: %v", err), 500)
					return
				}
				defer dataBody.Close()

				discardBuf := make([]byte, crypto.HeaderSize)
				if _, err := io.ReadFull(dataBody, discardBuf); err != nil {
					h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to discard header: %v", err), 500)
					return
				}
			}

			// Encode block table for transport to decryption goroutine
			hmacTable = encodeV3BlockTableForTransport(blockTable)
		} else {
			// v1/v2: inline HMAC table at end of file
			hmacTableOffset := crypto.HeaderSize + plaintextSize
			hmacTableSize := int64(blockCount) * crypto.HMACSize
			dataSize := plaintextSize

			// 1. Prefetch HMAC table (small range read)
			hmacBody, err := h.backend.GetRange(ctx, bucket, prefixedKey, hmacTableOffset, hmacTableSize)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to prefetch HMAC table: %v", err), 500)
				return
			}
			hmacTable, err = io.ReadAll(hmacBody)
			hmacBody.Close()
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read HMAC table: %v", err), 500)
				return
			}

			// 2. Start streaming data from Cloudflare (header + encrypted blocks, stop before HMAC)
			streamSize = crypto.HeaderSize + dataSize
			dataBody, err = h.backend.GetRange(ctx, bucket, prefixedKey, 0, streamSize)
			if err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object stream: %v", err), 500)
				return
			}
			defer dataBody.Close()

			// 3. Read and discard the 64-byte header (single-PUT only).
			// The header itself was already decoded from GetObject's
			// request-local read (armor-c3199f1c); the stream re-decode is
			// gone, only the discard remains.
			discard := make([]byte, crypto.HeaderSize)
			if _, err := io.ReadFull(dataBody, discard); err != nil {
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to read header: %v", err), 500)
				return
			}
		}
	}

	// 4. Set response headers before streaming
	// Note: Content-Length is not set for compressed objects because decompression
	// changes the size dynamically. HTTP will use chunked transfer encoding.
	if !armorMeta.Compressed {
		w.Header().Set("Content-Length", strconv.FormatInt(plaintextSize, 10))
	}
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
	w.Header().Set("X-Armor-Stream", "pipelined")
	if armorMeta.Compressed {
		w.Header().Set("X-Armor-Decompressed", "true")
	}
	w.WriteHeader(http.StatusOK)

	// 5. Stream decrypt using io.Pipe
	pr, pw := io.Pipe()

	// Determine the digest form this object declares so the streaming integrity
	// check below compares like-for-like. Single-PUT objects declare the plain
	// SHA-256 of the whole plaintext (in the envelope header). Multipart objects
	// written since bf-1v2ehf declare the combined per-part digest and carry a
	// block-aligned uniform part size P (x-amz-meta-armor-part-size); reproduce
	// it incrementally with MultipartDigestAccumulator. Legacy multipart objects
	// (no P) carry the empty-string placeholder, which is not a real digest and
	// is skipped below rather than enforced.
	//
	// A non-uniform (ADR-011) object's real upload-part boundaries don't line
	// up with P — MultipartDigestAccumulator's fixed-P chunking would combine
	// the wrong byte ranges — so it's excluded here; decryptNonUniformParts
	// reproduces the combined digest itself, from the real part boundaries it
	// already iterates, straight into wholeHash.
	useCombinedDigest := isMultipart && !isNonUniform && multipartPartSize > 0 && multipartPartSize%int64(blockSize) == 0
	var accumulator *backend.MultipartDigestAccumulator
	if useCombinedDigest {
		accumulator = backend.NewMultipartDigestAccumulator(multipartPartSize, blockSize)
	}

	// Start decryption goroutine
	go func() {
		defer pw.Close()

		wholeHash := sha256.New() // plain whole-object digest (single-PUT / legacy multipart)

		// ADR-011: Non-uniform multipart objects need offset-aware decryption.
		// Other objects retain the v3 single-PUT and v1/v2 block readers.
		if isNonUniform && len(cumulativePartSizes) > 0 {
			if err := h.decryptNonUniformParts(dataBody, pw, plaintextSize, blockSize, decryptor, armorMeta, cumulativePartSizes, hmacTable, accumulator, wholeHash); err != nil {
				pw.CloseWithError(err)
				return
			}
		} else if v3BlockTable, isV3 := decodeV3BlockTableFromTransport(hmacTable); isV3 {
			// v3 single-PUT: read blocks using the trailer's prefix sums.
			// One cryptor serves the whole object: the AES cipher, HKDF HMAC
			// key, and Reset-reusable HMAC state are derived once instead of
			// per block, each block's HMAC is verified exactly once (by the
			// cryptor, before any plaintext is released), and the read and
			// plaintext buffers are caller-owned and reused across blocks
			// (armor-61a35059).
			v3Cryptor, cryptorErr := decryptor.V3BlockCryptor(blockSize)
			if cryptorErr != nil {
				pw.CloseWithError(fmt.Errorf("v3 block cryptor: %w", cryptorErr))
				return
			}
			var encryptedBuf, plainBuf []byte
			for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
				if blockIndex >= v3BlockTable.EntryCount() {
					pw.CloseWithError(fmt.Errorf("block index %d out of range (v3 table has %d entries)", blockIndex, v3BlockTable.EntryCount()))
					return
				}

				entry := v3BlockTable.Entries[blockIndex]
				if needed := int(entry.RawLength()); cap(encryptedBuf) < needed {
					encryptedBuf = make([]byte, needed)
					plainBuf = make([]byte, needed)
				}
				encryptedBuf = encryptedBuf[:entry.RawLength()]
				n, err := io.ReadFull(dataBody, encryptedBuf)
				if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
					pw.CloseWithError(fmt.Errorf("read error at v3 block %d: %w", blockIndex, err))
					return
				}
				if n == 0 {
					break
				}
				encryptedBuf = encryptedBuf[:n]

				// DecryptBlockInto verifies the block's (part, block)-bound
				// HMAC first and releases plaintext only on a match, so no
				// separate verification pass runs here.
				decrypted := plainBuf[:len(encryptedBuf)]
				if err := v3Cryptor.DecryptBlockInto(decrypted, 0, uint32(blockIndex), encryptedBuf, entry.HMAC[:]); err != nil {
					pw.CloseWithError(fmt.Errorf("v3 block %d authentication/decryption failed: %w", blockIndex, err))
					return
				}
				if entry.IsCompressed() {
					decrypted, err = crypto.DecompressBlock(decrypted, true)
					if err != nil {
						pw.CloseWithError(fmt.Errorf("v3 block %d decompression failed: %w", blockIndex, err))
						return
					}
				}

				if accumulator != nil {
					accumulator.WriteBlock(decrypted, blockIndex == blockCount-1)
				} else if !armorMeta.Compressed {
					wholeHash.Write(decrypted)
				}
				if _, err := pw.Write(decrypted); err != nil {
					pw.CloseWithError(fmt.Errorf("write error at v3 block %d: %w", blockIndex, err))
					return
				}
			}
		} else {
			// v1/v2: optimized block-by-block decryption with inline HMAC table
			encryptedBuf := make([]byte, blockSize)
			for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
				// Calculate actual block size (last block may be smaller)
				remaining := plaintextSize - int64(blockIndex)*int64(blockSize)
				actualBlockSize := int(min64(int64(blockSize), remaining))

				// Read encrypted block
				encryptedBuf = encryptedBuf[:actualBlockSize]
				n, err := io.ReadFull(dataBody, encryptedBuf)
				if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
					pw.CloseWithError(fmt.Errorf("read error at block %d: %w", blockIndex, err))
					return
				}
				if n == 0 {
					break
				}
				encryptedBuf = encryptedBuf[:n]

				// Verify HMAC
				hmacOffset := blockIndex * crypto.HMACSize
				if hmacOffset+crypto.HMACSize > len(hmacTable) {
					pw.CloseWithError(fmt.Errorf("HMAC table too short at block %d", blockIndex))
					return
				}
				expectedHMAC := hmacTable[hmacOffset : hmacOffset+crypto.HMACSize]

				// ADR-011: For non-uniform parts, skip HMAC verification for placeholder (zero) HMACs
				// These are boundary blocks that span multiple parts and couldn't have HMACs
				// computed during upload. The decryption is still correct because each byte
				// was encrypted with the proper CTR counter.
				if !isPlaceholderHMAC(expectedHMAC) {
					mac := hmac.New(sha256.New, decryptor.HMACKey())
					indexBytes := make([]byte, 4)
					binary.BigEndian.PutUint32(indexBytes, uint32(blockIndex))
					mac.Write(indexBytes)
					mac.Write(encryptedBuf)
					computed := mac.Sum(nil)

					if !hmac.Equal(computed, expectedHMAC) {
						pw.CloseWithError(fmt.Errorf("block %d: HMAC verification failed", blockIndex))
						return
					}
				}

				// Decrypt block (need to use CTR stream)
				decrypted := make([]byte, n)
				ctr := makeCounter(armorMeta.IV, uint32(blockIndex), armorMeta.Version, blockSize)
				stream := cipher.NewCTR(decryptor.CipherBlock(), ctr)
				stream.XORKeyStream(decrypted, encryptedBuf)

				// Update the plaintext digest for end-of-stream verification. For
				// multipart objects with a known part size, fold the block into the
				// per-part accumulator; otherwise hash the whole plaintext.
				isLastBlock := blockIndex == blockCount-1
				if accumulator != nil {
					accumulator.WriteBlock(decrypted, isLastBlock)
				} else {
					wholeHash.Write(decrypted)
				}

				// Write plaintext to pipe
				if _, err := pw.Write(decrypted); err != nil {
					pw.CloseWithError(fmt.Errorf("write error at block %d: %w", blockIndex, err))
					return
				}
			}
		}

		// Verify the plaintext digest declared for this object.
		var computedSHA []byte
		if accumulator != nil {
			computedSHA, _ = hex.DecodeString(accumulator.Sum())
		} else {
			computedSHA = wholeHash.Sum(nil)
		}
		var expectedSHA []byte
		enforce := true
		if isMultipart {
			// Multipart objects carry the digest in metadata. Legacy uploads
			// (pre bf-1v2ehf) carry the empty-string placeholder, which is not a
			// real checksum — never enforce it.
			if backend.IsPlaceholderPlaintextSHA(armorMeta.PlaintextSHA) {
				enforce = false
			}
			expectedSHA, _ = hex.DecodeString(armorMeta.PlaintextSHA)
		} else {
			// For single-PUT objects, use the digest from the envelope header.
			expectedSHA = header.PlaintextSHA[:]
		}
		if enforce && !armorMeta.Compressed && len(expectedSHA) > 0 && !bytes.Equal(computedSHA, expectedSHA) {
			pw.CloseWithError(fmt.Errorf("plaintext SHA-256 mismatch"))
			return
		}
	}()

	// 6. Stream plaintext to client
	// If the object is compressed, decompress it before writing to response
	if armorMeta.Compressed {
		decompressedHash := sha256.New()
		var decompressor io.ReadCloser
		var err error

		// Select decompression method based on compression type
		switch armorMeta.CompressionType {
		case backend.CompressionGzip:
			// Create gzip decoder for streaming decompression
			decompressor, err = gzip.NewReader(pr)
			if err != nil {
				// Decompression setup failed - close pipe and return error
				pr.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create gzip decompression decoder: %v", err), 500)
				return
			}
		case backend.CompressionZlib:
			// Create zlib decoder for streaming decompression
			decompressor, err = zlib.NewReader(pr)
			if err != nil {
				// Decompression setup failed - close pipe and return error
				pr.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create zlib decompression decoder: %v", err), 500)
				return
			}
		case backend.CompressionZstd:
			// Create zstd decoder for streaming decompression
			zstdDecoder, err := zstd.NewReader(pr)
			if err != nil {
				// Decompression setup failed - close pipe and return error
				pr.Close()
				h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create zstd decompression decoder: %v", err), 500)
				return
			}
			// Wrap in zstdReadCloser to implement io.ReadCloser (zstd.Decoder.Close() doesn't return error)
			decompressor = &zstdReadCloser{Decoder: zstdDecoder}
		default:
			// Unknown compression type - close pipe and return error
			pr.Close()
			h.writeError(w, r, "InternalError", fmt.Sprintf("Unknown compression type: %s", armorMeta.CompressionType), 500)
			return
		}
		defer decompressor.Close()

		// Stream decompressed data to client
		_, err = io.Copy(io.MultiWriter(w, decompressedHash), decompressor)
		if err != nil {
			// Decompression failed during streaming
			// Log the error with context for debugging
			// Since headers are already sent, we can't change HTTP status,
			// but we log meaningful error information
			log.Printf("GET %s/%s: decompression streaming error: %v (compression type: %s)",
				bucket, key, err, armorMeta.CompressionType)

			// Classify the error to determine root cause
			errMsg := err.Error()
			if strings.Contains(errMsg, "unexpected EOF") || strings.Contains(errMsg, "EOF") {
				log.Printf("GET %s/%s: likely cause: truncated or corrupted compressed data",
					bucket, key)
			} else if strings.Contains(errMsg, "corrupt") || strings.Contains(errMsg, "invalid") {
				log.Printf("GET %s/%s: likely cause: data corruption in storage",
					bucket, key)
			} else {
				log.Printf("GET %s/%s: likely cause: decompression infrastructure error",
					bucket, key)
			}
			// Connection will be closed; client receives partial response
			return
		}
		if header != nil && !bytes.Equal(decompressedHash.Sum(nil), header.PlaintextSHA[:]) {
			// The response headers are already committed by the streaming
			// contract, so the client may have received bytes. Still record the
			// authenticated-format failure rather than silently accepting a
			// decompressed body whose declared plaintext digest disagrees.
			log.Printf("GET %s/%s: compressed plaintext SHA-256 mismatch", bucket, key)
		}
	} else {
		// Stream plaintext directly to client
		_, err := io.Copy(w, pr)
		if err != nil {
			log.Printf("GET %s/%s: plaintext streaming error: %v", bucket, key, err)
			return
		}
	}
}

// decryptNonUniformParts handles decryption for non-uniform multipart uploads (ADR-011).
// It decrypts part-by-part using offset-aware decryption, which is necessary because
// parts may start at arbitrary byte offsets (not just block boundaries).
func (h *Handlers) decryptNonUniformParts(dataBody io.ReadCloser, pw *io.PipeWriter, plaintextSize int64, blockSize int, decryptor *crypto.Decryptor, armorMeta *backend.ARMORMetadata, cumulativePartSizes map[int]int64, hmacTable []byte, accumulator *backend.MultipartDigestAccumulator, wholeHash hash.Hash) error {
	// Unwrap DEK using fingerprint with ring fallback
	lookupMEK := func(keyID, fingerprint string) ([]byte, bool) {
		return h.keyManager.GetMEKByFingerprint(keyID, fingerprint)
	}

	legacyFallback := func(wrappedDEK []byte) ([]byte, error) {
		mek, err := h.keyManager.GetMEKByID(armorMeta.KeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to get active key: %w", err)
		}
		dek, err := crypto.UnwrapDEK(mek, wrappedDEK)
		if err == nil {
			return dek, nil
		}

		ring := h.keyManager.Ring(armorMeta.KeyID)
		for _, ringEntry := range ring {
			dek, err := crypto.UnwrapDEK(ringEntry.MEK, wrappedDEK)
			if err == nil {
				return dek, nil
			}
		}

		return nil, fmt.Errorf("no key in active or ring can unwrap DEK")
	}

	wrappedDEKStr := base64.StdEncoding.EncodeToString(armorMeta.WrappedDEK)
	if armorMeta.MEKFingerprint != "" {
		wrappedDEKStr = fmt.Sprintf("v2:%s:%s", armorMeta.MEKFingerprint, wrappedDEKStr)
	}

	dek, _, err := crypto.UnwrapDEKByFingerprint(wrappedDEKStr, lookupMEK, legacyFallback)
	if err != nil {
		return fmt.Errorf("failed to unwrap DEK: %w", err)
	}

	// Create offset decryptor for arbitrary byte offset decryption
	offsetDecryptor, err := crypto.NewOffsetDecryptor(
		dek,
		armorMeta.IV,
		blockSize,
		uint8(armorMeta.Version),
	)
	if err != nil {
		return fmt.Errorf("failed to create offset decryptor: %w", err)
	}

	// Sort part numbers
	partNumbers := make([]int, 0, len(cumulativePartSizes))
	for pn := range cumulativePartSizes {
		partNumbers = append(partNumbers, pn)
	}
	for i := 0; i < len(partNumbers); i++ {
		for j := i + 1; j < len(partNumbers); j++ {
			if partNumbers[i] > partNumbers[j] {
				partNumbers[i], partNumbers[j] = partNumbers[j], partNumbers[i]
			}
		}
	}

	// Read and decrypt each part
	for _, partNumber := range partNumbers {
		if partNumber == 0 {
			continue // Part 0 doesn't exist (parts are 1-indexed)
		}

		partOffset := cumulativePartSizes[partNumber]
		var partSize int64

		// Calculate part size from next part's offset or total size
		if nextOffset, ok := cumulativePartSizes[partNumber+1]; ok {
			partSize = nextOffset - partOffset
		} else {
			// Last part - use remaining size
			partSize = plaintextSize - partOffset
		}

		if partSize <= 0 {
			continue
		}

		// Read encrypted part data
		encryptedPart := make([]byte, partSize)
		n, err := io.ReadFull(dataBody, encryptedPart)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read error for part %d: %w", partNumber, err)
		}
		if n == 0 {
			break
		}
		encryptedPart = encryptedPart[:n]

		// Decrypt this part using offset-aware decryption
		decryptedPart, err := offsetDecryptor.DecryptFromOffset(encryptedPart, partOffset)
		if err != nil {
			return fmt.Errorf("decrypt error for part %d at offset %d: %w", partNumber, partOffset, err)
		}

		// Update digest. CompleteMultipartUpload stores the combined per-part
		// digest (backend.CombinePartPlaintextSHAs): each upload part's plaintext
		// SHA-256, fed as raw digest bytes into one SHA-256 in ascending part
		// order. MultipartDigestAccumulator reproduces that only for the ADR-015
		// uniform case (chunking at fixed P boundaries); a genuinely non-uniform
		// object's real part boundaries don't line up with P, so reproduce the
		// combined digest directly from the actual parts this loop already
		// iterates in order: hash each decrypted part and feed that digest into
		// wholeHash, which callers.go then reads via wholeHash.Sum(nil) for
		// non-uniform objects instead of the accumulator.
		partDigest := sha256.Sum256(decryptedPart)
		wholeHash.Write(partDigest[:])

		// Write decrypted plaintext to pipe
		if _, err := pw.Write(decryptedPart); err != nil {
			return fmt.Errorf("write error for part %d: %w", partNumber, err)
		}
	}

	return nil
}

// makeCounter creates a 16-byte counter value from the IV and block index.
// Version 1 (legacy, vulnerable): counter_value = blockIndex
// Version 2 (fixed): counter_value = blockIndex * (blockSize / 16)
//
// The version must match the encryption version to decrypt correctly.
func makeCounter(iv []byte, blockIndex uint32, version int, blockSize int) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], iv[0:12])

	var counterValue uint32
	if version == 2 {
		// Version2: stride by number of AES blocks per ARMOR block
		aesBlocksPerArmorBlock := uint32(blockSize / 16)
		counterValue = blockIndex * aesBlocksPerArmorBlock
	} else {
		// Version1: legacy (buggy) derivation for backward compatibility
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
	return counter
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// zstdReadCloser wraps zstd.Decoder to implement io.ReadCloser
// zstd.Decoder.Close() doesn't return error, so we adapt it
type zstdReadCloser struct {
	Decoder *zstd.Decoder
}

func (z *zstdReadCloser) Read(p []byte) (n int, err error) {
	return z.Decoder.Read(p)
}

func (z *zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}
