// cmd_verify_multipart.go carries the multipart halves of 'armor verify':
// ADR-016 manifest metadata resolution for objects whose B2 head carries no
// x-amz-meta-* keys, and quick/full verification of multipart-completed
// objects whose ciphertext is headerless and whose per-block HMACs live in a
// JSON sidecar (ADR-003). The decrypt walk mirrors the restore-verifier's
// decryptV3Multipart (armor-86a90341) through the same exported backend view
// (NewMultipartSidecarEntry) the server's GET path uses.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/crypto"
)

// verifyManifestSuffix completes a stored object key to the name of the
// ADR-016 manifest sidecar CompleteMultipartUpload writes beside every
// multipart-completed object. Must match the server's manifestSuffix
// (internal/server/handlers) and the restore-verifier's manifestObjectSuffix;
// the manifest is stored under the prefixed data key, so no client-key
// stripping applies to it.
const verifyManifestSuffix = ".armor-manifest"

// emptyPlaintextSHA256Hex is the SHA-256 of the empty string — the placeholder
// digest legacy ADR-003 multipart uploads declared before per-part digests
// existed (bf-1v2ehf). "No digest declared", never something to enforce.
var emptyPlaintextSHA256Hex = func() string {
	sum := sha256.Sum256(nil)
	return hex.EncodeToString(sum[:])
}()

// resolveVerifyMetadata returns the ARMOR metadata map an object should be
// verified against: its own head metadata when that carries ARMOR key
// material, otherwise the ADR-016 manifest sidecar's copy. B2 never persists
// CreateMultipartUpload metadata onto the finished large file, so on B2 every
// multipart-completed object heads with an empty map and the manifest is the
// only place the wrapped DEK, IV and sizes still exist (armor-86a90341; the
// server's GET prefers the manifest for the same reason). ok is false when
// neither source yields a wrapped DEK: the object is genuinely not
// ARMOR-encrypted, or is a pre-ADR-016 multipart object nothing on B2
// describes any more.
func resolveVerifyMetadata(ctx context.Context, b2 backend.Backend, bucket, key string, headMeta map[string]string) (map[string]string, bool) {
	if _, ok := backend.ParseARMORMetadata(headMeta); ok {
		return headMeta, true
	}
	if meta, ok := loadManifestMetadataForVerify(ctx, b2, bucket, key); ok {
		if _, ok := backend.ParseARMORMetadata(meta); ok {
			return meta, true
		}
	}
	return headMeta, false
}

// loadManifestMetadataForVerify reads the ADR-016 manifest sidecar for a
// stored object key and returns the ARMOR metadata map it carries. The
// manifest object is written by an ordinary single-PUT, so — unlike the
// finished multipart large file — its metadata persists: the manifest
// object's own x-amz-meta-* headers carry the full map (what the server's
// readManifest parses), and CompleteMultipartUpload embeds the same map in
// the manifest JSON body as the fallback. ok is false when there is no
// manifest, or neither source yields ARMOR key material.
func loadManifestMetadataForVerify(ctx context.Context, b2 backend.Backend, bucket, key string) (map[string]string, bool) {
	body, info, err := b2.GetDirect(ctx, bucket, key+verifyManifestSuffix)
	if err != nil {
		return nil, false
	}
	defer body.Close()

	if info != nil {
		if _, ok := backend.ParseARMORMetadata(info.Metadata); ok {
			return info.Metadata, true
		}
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, false
	}
	var manifest backend.ManifestBody
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, false
	}
	if _, ok := backend.ParseARMORMetadata(manifest.Metadata); ok {
		return manifest.Metadata, true
	}
	return nil, false
}

// hasArmorMetadataHeader reports whether a metadata map carries any
// x-amz-meta-armor-* keys at all — the tell between "not an ARMOR object"
// (no ARMOR headers anywhere → keying ERROR) and "an ARMOR object whose
// stored metadata is damaged" (headers present but the wrapped DEK will not
// parse → CORRUPTED, e.g. TestVerifyCorruptedDEK: a truncated wrapped DEK
// fails base64 inside ParseARMORMetadata, which surfaces as ok=false).
func hasArmorMetadataHeader(meta map[string]string) bool {
	for k := range meta {
		if strings.HasPrefix(k, "x-amz-meta-armor-") {
			return true
		}
	}
	return false
}

// verifyClientKey maps a stored key to the client key the multipart HMAC
// sidecar is named by. It returns the UNprefixed client key itself — the
// manager's Load hashes it into <prefix>.armor/hmac/<sha256(prefix+key)> —
// because CompleteMultipartUpload saves the sidecar before applyPrefix while
// the assembled ciphertext is stored under the prefixed key (armor-1b272971 —
// the same defect the server read path and the restore-verifier fixed). The
// managers here are built WithKeyPrefix(b2PrefixFlag) so the composed
// location is probed first, with the pre-2026-09-20 bucket-root sidecar as
// fallback (ADR-003 sidecar addendum).
func verifyClientKey(storedKey string) string {
	return strings.TrimPrefix(storedKey, b2PrefixFlag)
}

// digestDeclaredForVerify reports whether a declared plaintext digest should
// be enforced: absent, or the legacy empty-string placeholder (bf-1v2ehf),
// means "no digest declared".
func digestDeclaredForVerify(expected string) bool {
	return expected != "" && expected != emptyPlaintextSHA256Hex
}

// plaintextDigestForVerify returns the plaintext digest comparable to an
// object's declared x-amz-meta-armor-plaintext-sha256: plain SHA-256 of the
// whole plaintext normally; the combined per-part digest
// (backend.ComputeMultipartDigest) when the metadata declares a uniform part
// size, which is the form CompleteMultipartUpload stores for multipart
// objects (ADR-015). Mirrors the restore-verifier's
// plaintextDigestForMetadata.
func plaintextDigestForVerify(plaintext []byte, meta map[string]string) string {
	if ps := meta["x-amz-meta-armor-part-size"]; ps != "" {
		var partSize int64
		if _, err := fmt.Sscanf(ps, "%d", &partSize); err == nil && partSize > 0 {
			return backend.ComputeMultipartDigest(plaintext, partSize)
		}
	}
	sum := sha256.Sum256(plaintext)
	return hex.EncodeToString(sum[:])
}

// quickVerifyMultipartObject verifies a multipart object's DEK and HMAC
// sidecar without decrypting the body: the envelope-header check of quick
// mode has no equivalent here (multipart ciphertext is headerless), so the
// structural stand-ins are that the sidecar loads, parses and accounts for
// exactly the stored ciphertext bytes.
func quickVerifyMultipartObject(ctx context.Context, b2 backend.Backend, ks *verifyKeySource, bucket, key string, info *backend.ObjectInfo, meta map[string]string, startTime time.Time) ObjectVerificationResult {
	result := ObjectVerificationResult{
		Bucket:    bucket,
		Key:       key,
		SizeBytes: info.Size,
		ModTime:   info.LastModified,
	}

	armorMeta, ok := backend.ParseARMORMetadata(meta)
	if !ok {
		result.Status = "ERROR"
		result.Error = "Failed to parse ARMOR metadata for multipart object"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	// DEK first: unwrapping proves the key material and the (possibly
	// fingerprinted) wrapped DEK agree.
	dek, ok := unwrapObjectDEK(ks, meta, &result, startTime)
	if !ok {
		return result
	}
	defer zeroBytes(dek)

	switch armorMeta.Version {
	case 3:
		sidecar, err := backend.NewMultipartStateManager(b2, bucket).WithKeyPrefix(b2PrefixFlag).LoadHMACTableV3(ctx, verifyClientKey(key))
		if err != nil {
			result.Status = "ERROR"
			result.Error = fmt.Sprintf("Failed to load v3 multipart HMAC sidecar: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
		// Parsing the sidecar is itself the structural check: a block entry
		// that does not parse is an error, never a silent skip.
		if _, err := backend.NewMultipartSidecarEntry(sidecar); err != nil {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("v3 multipart sidecar does not parse: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
		var stored int64
		for _, part := range sidecar.Parts {
			stored += part.CiphertextLen
		}
		if stored != info.Size {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("v3 multipart sidecar accounts for %d ciphertext bytes, object stores %d", stored, info.Size)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	default:
		// Loading the flat v1/v2 sidecar doubles as its structural check: a
		// missing or unparseable sidecar fails here; the payload itself is
		// only consumed by the full walk.
		if _, err := backend.NewMultipartStateManager(b2, bucket).WithKeyPrefix(b2PrefixFlag).LoadHMACTable(ctx, verifyClientKey(key)); err != nil {
			result.Status = "ERROR"
			result.Error = fmt.Sprintf("Failed to load multipart HMAC sidecar: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
		// v1/v2 multipart ciphertext is CTR output: its length equals the
		// declared plaintext size byte for byte.
		if armorMeta.PlaintextSize != info.Size {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("multipart object stores %d bytes but metadata declares plaintext size %d", info.Size, armorMeta.PlaintextSize)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	}

	result.Status = "OK"
	result.Details = "Multipart DEK and HMAC sidecar verified successfully"
	result.Duration = time.Since(startTime).Seconds()
	return result
}

// fullVerifyMultipartObject performs complete HMAC + digest verification of a
// multipart-completed object: it reads the whole stored ciphertext (raw
// concatenated part ciphertext, no envelope header), loads the per-block HMAC
// sidecar, decrypts and HMAC-verifies every block, and compares the combined
// per-part digest when the metadata declares one.
func fullVerifyMultipartObject(ctx context.Context, b2 backend.Backend, ks *verifyKeySource, bucket, key string, info *backend.ObjectInfo, meta map[string]string, startTime time.Time) ObjectVerificationResult {
	result := ObjectVerificationResult{
		Bucket:    bucket,
		Key:       key,
		SizeBytes: info.Size,
		ModTime:   info.LastModified,
	}

	armorMeta, ok := backend.ParseARMORMetadata(meta)
	if !ok {
		result.Status = "ERROR"
		result.Error = "Failed to parse ARMOR metadata for multipart object"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	if len(armorMeta.IV) == 0 {
		result.Status = "ERROR"
		result.Error = "Multipart object missing IV metadata"
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	dek, ok := unwrapObjectDEK(ks, meta, &result, startTime)
	if !ok {
		return result
	}
	defer zeroBytes(dek)

	// Read the whole raw ciphertext. Multipart bodies carry no envelope
	// header; CTR keeps v1/v2 ciphertext at plaintext size, and v3 stores
	// whatever the per-block compression produced — the sidecar's part
	// CiphertextLen sum, checked against info.Size during the walk below.
	objectReader, _, err := b2.Get(ctx, bucket, key)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read object: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}
	defer objectReader.Close()
	ciphertext, err := io.ReadAll(objectReader)
	if err != nil {
		result.Status = "ERROR"
		result.Error = fmt.Sprintf("Failed to read object data: %v", err)
		result.Duration = time.Since(startTime).Seconds()
		return result
	}

	var plaintext []byte
	switch armorMeta.Version {
	case 3:
		sidecar, err := backend.NewMultipartStateManager(b2, bucket).WithKeyPrefix(b2PrefixFlag).LoadHMACTableV3(ctx, verifyClientKey(key))
		if err != nil {
			result.Status = "ERROR"
			result.Error = fmt.Sprintf("Failed to load v3 multipart HMAC sidecar: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
		plaintext, err = decryptV3MultipartForVerify(ciphertext, sidecar, dek, armorMeta.IV, armorMeta.BlockSize)
		if err != nil {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("v3 multipart verification failed: %v", err)
			result.Details = "Object data corruption detected - HMAC mismatch, sidecar mismatch or decompression error"
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	default:
		sidecar, err := backend.NewMultipartStateManager(b2, bucket).WithKeyPrefix(b2PrefixFlag).LoadHMACTable(ctx, verifyClientKey(key))
		if err != nil {
			result.Status = "ERROR"
			result.Error = fmt.Sprintf("Failed to load multipart HMAC sidecar: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
		// Flatten the sidecar's per-block HMACs into the contiguous table the
		// Decryptor consumes; Decrypt verifies each block's HMAC.
		hmacTable := make([]byte, 0, len(sidecar.BlockHMACs)*crypto.HMACSize)
		for _, h := range sidecar.BlockHMACs {
			hmacTable = append(hmacTable, h...)
		}
		decryptor, err := crypto.NewDecryptorWithVersion(dek, armorMeta.IV, armorMeta.BlockSize, uint8(armorMeta.Version))
		if err != nil {
			result.Status = "ERROR"
			result.Error = fmt.Sprintf("Failed to create decryptor: %v", err)
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
		plaintext, err = decryptor.Decrypt(ciphertext, hmacTable)
		if err != nil {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("Decryption failed: %v", err)
			result.Details = "Object data corruption detected - HMAC mismatch"
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	}

	// Verify the declared combined per-part digest (skipped when the metadata
	// declares none — the legacy placeholder).
	expectedSHA := meta["x-amz-meta-armor-plaintext-sha256"]
	if digestDeclaredForVerify(expectedSHA) {
		computedSHA := plaintextDigestForVerify(plaintext, meta)
		if expectedSHA != computedSHA {
			result.Status = "CORRUPTED"
			result.Error = fmt.Sprintf("SHA-256 mismatch: expected=%s, got=%s", expectedSHA, computedSHA)
			result.Details = "Plaintext checksum verification failed"
			result.Duration = time.Since(startTime).Seconds()
			return result
		}
	}

	result.Status = "OK"
	result.Details = "Full HMAC + digest verification passed (multipart)"
	result.Duration = time.Since(startTime).Seconds()
	return result
}

// decryptV3MultipartForVerify decrypts a whole v3 multipart object from its
// concatenated part ciphertext, part by part and block by block — the same
// per-part walk the server's handleV3MultipartGet and the restore-verifier's
// decryptV3Multipart perform. The per-block HMACs and CTR counters are bound
// to the (part number, block index) pair, so a flat whole-object table
// cannot verify anything past the first part (armor-86a90341). Ciphertext
// for part N starts at the sum of the preceding parts' CiphertextLen — B2
// stores the parts concatenated.
func decryptV3MultipartForVerify(ciphertext []byte, sidecar *backend.HMACTableSidecarV3, dek, iv []byte, blockSize int) ([]byte, error) {
	entry, err := backend.NewMultipartSidecarEntry(sidecar)
	if err != nil {
		return nil, fmt.Errorf("v3 sidecar does not parse: %w", err)
	}

	var plaintext []byte
	offset := int64(0)
	for partIdx, part := range sidecar.Parts {
		if offset+part.CiphertextLen > int64(len(ciphertext)) {
			return nil, fmt.Errorf("part %d extends beyond the stored ciphertext (need %d bytes at offset %d, have %d)",
				part.N, part.CiphertextLen, offset, len(ciphertext))
		}
		partPlaintext, err := decryptV3MultipartPartForVerify(ciphertext[offset:offset+part.CiphertextLen], entry, partIdx, dek, iv, blockSize)
		if err != nil {
			return nil, err
		}
		plaintext = append(plaintext, partPlaintext...)
		offset += part.CiphertextLen
	}
	if offset != int64(len(ciphertext)) {
		return nil, fmt.Errorf("stored ciphertext has %d bytes outside any declared part", int64(len(ciphertext))-offset)
	}
	return plaintext, nil
}

// decryptV3MultipartPartForVerify verifies and decrypts one part's
// ciphertext. Each block's HMAC is bound to (part.N, block index within the
// part); the compression flag lives in the high bit of the sidecar's clen
// field and DecompressBlock's zstd mode follows the server's decryptV3Part.
func decryptV3MultipartPartForVerify(partCiphertext []byte, entry *backend.MultipartSidecarEntry, partIdx int, dek, iv []byte, blockSize int) ([]byte, error) {
	part := entry.Sidecar.Parts[partIdx]

	var plaintext []byte
	offset := int64(0)
	for blockIdx := 0; blockIdx < len(part.Blocks); blockIdx++ {
		blockLen, err := entry.GetBlockLength(partIdx, blockIdx)
		if err != nil {
			return nil, fmt.Errorf("part %d block %d: %w", part.N, blockIdx, err)
		}
		if offset+int64(blockLen) > int64(len(partCiphertext)) {
			return nil, fmt.Errorf("part %d block %d extends beyond the part ciphertext", part.N, blockIdx)
		}
		blockCiphertext := partCiphertext[offset : offset+int64(blockLen)]
		offset += int64(blockLen)

		expectedHMAC, err := entry.GetBlockHMAC(partIdx, blockIdx)
		if err != nil {
			return nil, fmt.Errorf("part %d block %d: %w", part.N, blockIdx, err)
		}

		// DecryptBlockV3 verifies the (part, block)-bound HMAC before touching
		// the counter, so a mismatch aborts here as a verification failure.
		blockPlaintext, err := crypto.DecryptBlockV3(dek, iv, uint16(part.N), uint32(blockIdx), blockCiphertext, expectedHMAC, blockSize)
		if err != nil {
			return nil, fmt.Errorf("part %d block %d: %w", part.N, blockIdx, err)
		}

		compressed, err := entry.IsBlockCompressed(partIdx, blockIdx)
		if err != nil {
			return nil, fmt.Errorf("part %d block %d: %w", part.N, blockIdx, err)
		}
		if compressed {
			blockPlaintext, err = crypto.DecompressBlock(blockPlaintext, true)
			if err != nil {
				return nil, fmt.Errorf("part %d block %d: decompression failed: %w", part.N, blockIdx, err)
			}
		}

		plaintext = append(plaintext, blockPlaintext...)
	}
	if offset != int64(len(partCiphertext)) {
		return nil, fmt.Errorf("part %d ciphertext has %d bytes outside any declared block", part.N, int64(len(partCiphertext))-offset)
	}
	return plaintext, nil
}
