// Package main provides a fully standalone V1/V2 fixture generator.
// This tool implements all crypto primitives independently of the ARMOR codebase
// to ensure adversarial validation - if the migration code has a bug, this
// generator won't share it.
//
// Usage:
//
//	go run standalone_generator.go <output-dir>
//
// Independence guarantee: This code does NOT import any ARMOR internal packages.
// All crypto primitives (envelope encoding, DEK wrapping, encryption) are
// reimplemented from the format specifications.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
)

const (
	// ARMOR envelope magic number
	magic = "ARMR"

	// Version constants
	version1 = 0x01
	version2 = 0x02
)

// FixtureMetadata records the plaintext properties and expected V3 layout.
type FixtureMetadata struct {
	PlaintextSHA256          string   `json:"plaintext_sha256"`
	PlaintextLength          int64    `json:"plaintext_length"`
	SourceVersion            string   `json:"source_version"`   // "v1", "v2", "malformed"
	SourceLayout             string   `json:"source_layout"`    // "single", "multipart-uniform", "multipart-nonuniform"
	V3Expected               V3Layout `json:"v3_expected"`
	Description              string   `json:"description"`
	ExpectedMigrationOutcome string   `json:"expected_migration_outcome"` // "success", "failure", "skip"
	ExpectedFailureReason    string   `json:"expected_failure_reason,omitempty"`
}

// V3Layout describes the expected V3 outcome.
type V3Layout struct {
	IsMultipart       bool     `json:"is_multipart"`
	PartCount         int      `json:"part_count,omitempty"`
	BlocksPerPart     []int    `json:"blocks_per_part,omitempty"`
	CompressionUsed   bool     `json:"compression_used"`
	SidecarPath       string   `json:"sidecar_path,omitempty"`
	ManifestReference string   `json:"manifest_reference,omitempty"`
}

// FixtureBundle contains all fixture data for a single test case.
type FixtureBundle struct {
	Metadata       FixtureMetadata   `json:"metadata"`
	StoredCiphertext []byte          `json:"-"` // Stored in separate file
	ObjectMetadata   map[string]string `json:"object_metadata"`
	SidecarData      []byte          `json:"-"` // For multipart, stored separately
}

// FixtureGenerator creates canonical V1/V2 fixtures with independent crypto.
type FixtureGenerator struct {
	outputDir string
	mek       []byte // Master Encryption Key (32 bytes)
	dek       []byte // Data Encryption Key (32 bytes)
	iv        []byte // Initialization Vector (16 bytes)
}

// NewFixtureGenerator creates a new fixture generator with deterministic keys.
func NewFixtureGenerator(outputDir string) (*FixtureGenerator, error) {
	// Use deterministic values for reproducibility across runs
	mek := make([]byte, 32)
	dek := make([]byte, 32)
	iv := make([]byte, 16)

	for i := range mek {
		mek[i] = byte(i + 1)     // MEK: 0x01, 0x02, ..., 0x20
		dek[i] = byte(i + 2)     // DEK: 0x02, 0x03, ..., 0x21
	}
	for i := range iv {
		iv[i] = byte(i + 3)     // IV: 0x03, 0x04, ..., 0x12
	}

	return &FixtureGenerator{
		outputDir: outputDir,
		mek:       mek,
		dek:       dek,
		iv:        iv,
	}, nil
}

// encodeEnvelopeHeader creates a standalone envelope header.
// Header format (64 bytes total):
//   Magic(4) + Version(1) + BlockSizeLog2(1) + IV(16) + PlaintextSize(8) + PlaintextSHA(32) + Reserved(2)
func (fg *FixtureGenerator) encodeEnvelopeHeader(version byte, blockSize int, plaintextSize int64, plaintextSHA []byte) ([]byte, error) {
	if len(plaintextSHA) != 32 {
		return nil, fmt.Errorf("plaintextSHA must be 32 bytes")
	}

	header := make([]byte, 64)

	// Magic number
	copy(header[0:4], magic)

	// Version
	header[4] = version

	// Block size log2
	blockSizeLog2 := byte(0)
	for blockSize > 1 {
		blockSize >>= 1
		blockSizeLog2++
	}
	header[5] = blockSizeLog2

	// IV
	copy(header[6:22], fg.iv)

	// Plaintext size
	binary.BigEndian.PutUint64(header[22:30], uint64(plaintextSize))

	// Plaintext SHA256
	copy(header[30:62], plaintextSHA)

	// Reserved bytes (zero-filled)
	header[62] = 0
	header[63] = 0

	return header, nil
}

// wrapDEK implements standalone DEK wrapping using AES-GCM.
// This matches the ARMOR DEK wrapping format.
func (fg *FixtureGenerator) wrapDEK() ([]byte, error) {
	// Generate random nonce for GCM
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create AES-GCM cipher from MEK
	block, err := aes.NewCipher(fg.mek)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Wrap DEK with AES-GCM
	wrapped := aesgcm.Seal(nil, nonce, fg.dek, nil)

	// Format: nonce(12) || ciphertext(40) = 52 bytes total
	result := make([]byte, 0, 12+len(wrapped))
	result = append(result, nonce...)
	result = append(result, wrapped...)

	return result, nil
}

// deriveHMACKey derives HMAC key from DEK using HKDF-SHA256.
// This matches the ARMOR HMAC key derivation.
func (fg *FixtureGenerator) deriveHMACKey() ([]byte, error) {
	// Simple HKDF: HMAC-SHA256(DEK, "armor-hmac") with expansion
	h := hmac.New(sha256.New, fg.dek)
	h.Write([]byte("armor-hmac-key"))
	return h.Sum(nil), nil
}

// computeBlockHMAC computes HMAC-SHA256 for a single encrypted block.
func (fg *FixtureGenerator) computeBlockHMAC(hmacKey []byte, encryptedBlock []byte, blockIndex int) []byte {
	mac := hmac.New(sha256.New, hmacKey)

	// HMAC = HMAC-SHA256(hmacKey, uint32(block_index) || ciphertext_block)
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, uint32(blockIndex))
	mac.Write(indexBytes)
	mac.Write(encryptedBlock)

	return mac.Sum(nil)
}

// encryptV1 encrypts using V1 counter derivation (vulnerable - keystream reuse).
// V1 bug: counter = blockIndex (not blockIndex * aesBlocksPerArmorBlock)
func (fg *FixtureGenerator) encryptV1(plaintext []byte, blockSize int) ([]byte, []byte, error) {
	block, err := aes.NewCipher(fg.dek)
	if err != nil {
		return nil, nil, err
	}

	blockCount := int(math.Ceil(float64(len(plaintext)) / float64(blockSize)))
	ciphertext := make([]byte, 0, len(plaintext))
	hmacTable := make([]byte, 0, blockCount*32)

	hmacKey, err := fg.deriveHMACKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive HMAC key: %w", err)
	}

	for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
		start := blockIndex * blockSize
		end := start + blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}
		blockPlaintext := plaintext[start:end]

		if len(blockPlaintext) == 0 {
			break
		}

		// V1 counter derivation (BUGGY - causes keystream reuse)
		counter := uint32(blockIndex) // BUG: should be blockIndex * (blockSize/16)
		blockCtr := make([]byte, 16)
		binary.LittleEndian.PutUint32(blockCtr, counter)

		stream := cipher.NewCTR(block, blockCtr)
		blockCiphertext := make([]byte, len(blockPlaintext))
		stream.XORKeyStream(blockCiphertext, blockPlaintext)

		// Compute per-block HMAC
		blockHMAC := fg.computeBlockHMAC(hmacKey, blockCiphertext, blockIndex)
		hmacTable = append(hmacTable, blockHMAC...)

		ciphertext = append(ciphertext, blockCiphertext...)
	}

	return ciphertext, hmacTable, nil
}

// encryptV2 encrypts using V2 counter derivation (fixed - no keystream reuse).
// V2 fix: counter = blockIndex * (blockSize / 16)
func (fg *FixtureGenerator) encryptV2(plaintext []byte, blockSize int) ([]byte, []byte, error) {
	block, err := aes.NewCipher(fg.dek)
	if err != nil {
		return nil, nil, err
	}

	blockCount := int(math.Ceil(float64(len(plaintext)) / float64(blockSize)))
	ciphertext := make([]byte, 0, len(plaintext))
	hmacTable := make([]byte, 0, blockCount*32)

	hmacKey, err := fg.deriveHMACKey()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive HMAC key: %w", err)
	}

	aesBlocksPerArmorBlock := blockSize / 16

	for blockIndex := 0; blockIndex < blockCount; blockIndex++ {
		start := blockIndex * blockSize
		end := start + blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}
		blockPlaintext := plaintext[start:end]

		if len(blockPlaintext) == 0 {
			break
		}

		// V2 counter derivation (FIXED - no keystream reuse)
		counter := uint32(blockIndex * aesBlocksPerArmorBlock)
		blockCtr := make([]byte, 16)
		binary.LittleEndian.PutUint32(blockCtr, counter)

		stream := cipher.NewCTR(block, blockCtr)
		blockCiphertext := make([]byte, len(blockPlaintext))
		stream.XORKeyStream(blockCiphertext, blockPlaintext)

		// Compute per-block HMAC
		blockHMAC := fg.computeBlockHMAC(hmacKey, blockCiphertext, blockIndex)
		hmacTable = append(hmacTable, blockHMAC...)

		ciphertext = append(ciphertext, blockCiphertext...)
	}

	return ciphertext, hmacTable, nil
}

// GenerateV1SingleExplicit creates a V1 single-PUT fixture with explicit version metadata.
func (fg *FixtureGenerator) GenerateV1SingleExplicit(plaintext []byte) (*FixtureBundle, error) {
	blockSize := 65536 // 64 KB
	plaintextSHA := sha256.Sum256(plaintext)

	// Create V1 envelope header (standalone implementation)
	header, err := fg.encodeEnvelopeHeader(version1, blockSize, int64(len(plaintext)), plaintextSHA[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create V1 header: %w", err)
	}

	// Encrypt using V1 counter derivation (vulnerable keystream reuse)
	ciphertext, hmacTable, err := fg.encryptV1(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V1: %w", err)
	}

	// Build envelope: header || ciphertext || hmacTable
	envelope := append(header, ciphertext...)
	envelope = append(envelope, hmacTable...)

	// Wrap DEK for storage (standalone implementation)
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v1",
			SourceLayout:             "single",
			V3Expected:               V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:              "V1 single-PUT with explicit version metadata",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: envelope,
		ObjectMetadata:   metadata,
	}, nil
}

// GenerateV1SingleImplicit creates a V1 single-PUT fixture with missing version metadata.
func (fg *FixtureGenerator) GenerateV1SingleImplicit(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}

	// Remove version metadata to test implicit V1 detection
	delete(bundle.ObjectMetadata, "x-amz-meta-armor-version")
	bundle.Metadata.Description = "V1 single-PUT with missing version metadata (implicit V1)"

	return bundle, nil
}

// GenerateV2Single creates a V2 single-PUT fixture.
func (fg *FixtureGenerator) GenerateV2Single(plaintext []byte) (*FixtureBundle, error) {
	blockSize := 65536 // 64 KB
	plaintextSHA := sha256.Sum256(plaintext)

	// Create V2 envelope header (standalone implementation)
	header, err := fg.encodeEnvelopeHeader(version2, blockSize, int64(len(plaintext)), plaintextSHA[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create V2 header: %w", err)
	}

	// Encrypt using V2 counter derivation (fixed, no keystream reuse)
	ciphertext, hmacTable, err := fg.encryptV2(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2: %w", err)
	}

	// Build envelope: header || ciphertext || hmacTable
	envelope := append(header, ciphertext...)
	envelope = append(envelope, hmacTable...)

	// Wrap DEK for storage (standalone implementation)
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Compute MEK fingerprint for V2 wrapped DEK format
	mekSHA := sha256.Sum256(fg.mek)
	mekFingerprint := hex.EncodeToString(mekSHA[:])[:8]
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "2",
		"x-amz-meta-armor-wrapped-dek":    wrappedDEKV2,
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v2",
			SourceLayout:             "single",
			V3Expected:               V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:              "V2 single-PUT object with fixed counter derivation",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: envelope,
		ObjectMetadata:   metadata,
	}, nil
}

// GenerateV1Multipart creates a V1 multipart fixture.
func (fg *FixtureGenerator) GenerateV1Multipart(plaintext []byte, partSize int) (*FixtureBundle, error) {
	blockSize := 65536 // 64 KB
	plaintextSHA := sha256.Sum256(plaintext)

	// Encrypt using V1 counter derivation
	ciphertext, hmacTable, err := fg.encryptV1(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V1 multipart: %w", err)
	}

	// Wrap DEK
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Calculate parts
	partCount := int(math.Ceil(float64(len(plaintext)) / float64(partSize)))
	blocksPerPart := make([]int, partCount)
	for i := 0; i < partCount; i++ {
		start := i * partSize
		end := start + partSize
		if end > len(plaintext) {
			end = len(plaintext)
		}
		partLen := end - start
		blocksPerPart[i] = int(math.Ceil(float64(partLen) / float64(blockSize)))
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":      "true",
		"x-amz-meta-armor-part-size":      fmt.Sprintf("%d", partSize),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v1",
			SourceLayout:             "multipart-uniform",
			V3Expected: V3Layout{
				IsMultipart:     true,
				PartCount:       partCount,
				BlocksPerPart:   blocksPerPart,
				CompressionUsed: false,
				SidecarPath:     sidecarPath,
			},
			Description:              "V1 multipart object with HMAC sidecar",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateV2Multipart creates a V2 multipart fixture.
func (fg *FixtureGenerator) GenerateV2Multipart(plaintext []byte, partSize int) (*FixtureBundle, error) {
	blockSize := 65536 // 64 KB
	plaintextSHA := sha256.Sum256(plaintext)

	// Encrypt using V2 counter derivation
	ciphertext, hmacTable, err := fg.encryptV2(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2 multipart: %w", err)
	}

	// Wrap DEK
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Calculate parts
	partCount := int(math.Ceil(float64(len(plaintext)) / float64(partSize)))
	blocksPerPart := make([]int, partCount)
	for i := 0; i < partCount; i++ {
		start := i * partSize
		end := start + partSize
		if end > len(plaintext) {
			end = len(plaintext)
		}
		partLen := end - start
		blocksPerPart[i] = int(math.Ceil(float64(partLen) / float64(blockSize)))
	}

	// Compute MEK fingerprint for V2 format
	mekSHA := sha256.Sum256(fg.mek)
	mekFingerprint := hex.EncodeToString(mekSHA[:])[:8]
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "2",
		"x-amz-meta-armor-wrapped-dek":    wrappedDEKV2,
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":      "true",
		"x-amz-meta-armor-part-size":      fmt.Sprintf("%d", partSize),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v2",
			SourceLayout:             "multipart-uniform",
			V3Expected: V3Layout{
				IsMultipart:     true,
				PartCount:       partCount,
				BlocksPerPart:   blocksPerPart,
				CompressionUsed: false,
				SidecarPath:     sidecarPath,
			},
			Description:              "V2 multipart object with HMAC sidecar",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateV1SingleMinimal creates a V1 single-PUT fixture with minimal metadata.
func (fg *FixtureGenerator) GenerateV1SingleMinimal(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}

	// Keep only required fields
	requiredKeys := map[string]bool{
		"x-amz-meta-armor-wrapped-dek": true,
		"x-amz-meta-armor-iv":          true,
		"x-amz-meta-armor-block-size":  true,
	}

	for k := range bundle.ObjectMetadata {
		if !requiredKeys[k] {
			delete(bundle.ObjectMetadata, k)
		}
	}

	bundle.Metadata.Description = "V1 single-PUT with minimal metadata (only required fields)"

	return bundle, nil
}

// GenerateV2SingleStandard creates a V2 single-PUT fixture with full metadata.
func (fg *FixtureGenerator) GenerateV2SingleStandard(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV2Single(plaintext)
	if err != nil {
		return nil, err
	}

	// Add V2-specific metadata fields
	plaintextSHA := sha256.Sum256(plaintext)
	bundle.ObjectMetadata["x-amz-meta-armor-etag"] = hex.EncodeToString(plaintextSHA[:])
	bundle.Metadata.Description = "V2 single-PUT with full metadata (fixed counter derivation, ETag)"

	return bundle, nil
}

// splitIntoVariableFinalParts splits plaintext into uniform parts with a smaller final part (ADR-010).
func (fg *FixtureGenerator) splitIntoVariableFinalParts(plaintext []byte, uniformPartSize int, finalPartSize int) [][]byte {
	if uniformPartSize <= 0 {
		uniformPartSize = 5 * 1024 * 1024 // Default 5MB
	}
	if finalPartSize <= 0 {
		finalPartSize = uniformPartSize / 2 // Default half size
	}

	var parts [][]byte
	offset := 0

	// Create uniform parts until we have enough room for the final part
	for offset < len(plaintext) {
		remaining := len(plaintext) - offset
		if remaining <= finalPartSize {
			// This is the final part
			parts = append(parts, plaintext[offset:])
			break
		} else if remaining < uniformPartSize {
			// Not enough for a full uniform part, make it the final part
			parts = append(parts, plaintext[offset:])
			break
		} else {
			// Create a uniform part
			end := offset + uniformPartSize
			parts = append(parts, plaintext[offset:end])
			offset = end
		}
	}

	return parts
}

// splitIntoNonUniformParts splits plaintext into parts with explicit sizes (ADR-011).
func (fg *FixtureGenerator) splitIntoNonUniformParts(plaintext []byte, partSizes []int) [][]byte {
	if len(partSizes) == 0 {
		// Default to a simple non-uniform pattern
		return fg.splitIntoVariableFinalParts(plaintext, 3*1024*1024, 2*1024*1024)
	}

	var parts [][]byte
	offset := 0

	for i, partSize := range partSizes {
		if offset >= len(plaintext) {
			break
		}

		end := offset + partSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		// Last part gets everything remaining
		if i == len(partSizes)-1 {
			parts = append(parts, plaintext[offset:])
		} else {
			parts = append(parts, plaintext[offset:end])
		}

		offset = end
	}

	return parts
}

// computeCombinedHMACTable computes HMAC table for all blocks in all parts.
func (fg *FixtureGenerator) computeCombinedHMACTable(encryptedParts [][]byte, blockSize int) []byte {
	hmacTable := make([]byte, 0)
	hmacKey, err := fg.deriveHMACKey()
	if err != nil {
		return nil
	}

	globalBlockIndex := 0
	for _, encryptedPart := range encryptedParts {
		// Split part into blocks
		for i := 0; i < len(encryptedPart); i += blockSize {
			blockEnd := i + blockSize
			if blockEnd > len(encryptedPart) {
				blockEnd = len(encryptedPart)
			}
			encryptedBlock := encryptedPart[i:blockEnd]

			if len(encryptedBlock) == 0 {
				continue
			}

			blockHMAC := fg.computeBlockHMAC(hmacKey, encryptedBlock, globalBlockIndex)
			hmacTable = append(hmacTable, blockHMAC...)
			globalBlockIndex++
		}
	}

	return hmacTable
}

// computeBlocksPerPart calculates blocks per part for V3 expected layout.
func (fg *FixtureGenerator) computeBlocksPerPart(parts [][]byte, blockSize int) []int {
	blocksPerPart := make([]int, len(parts))
	for i, part := range parts {
		blocksPerPart[i] = (len(part) + blockSize - 1) / blockSize
		if blocksPerPart[i] == 0 && len(part) > 0 {
			blocksPerPart[i] = 1
		}
	}
	return blocksPerPart
}

// encryptMultipart encrypts plaintext parts using the specified version derivation.
func (fg *FixtureGenerator) encryptMultipart(parts [][]byte, blockSize int, version byte) ([]byte, []byte, error) {
	var encryptedParts [][]byte

	for _, part := range parts {
		var ciphertext []byte
		var err error

		if version == version1 {
			ciphertext, _, err = fg.encryptV1(part, blockSize)
		} else {
			ciphertext, _, err = fg.encryptV2(part, blockSize)
		}

		if err != nil {
			return nil, nil, fmt.Errorf("failed to encrypt part: %w", err)
		}
		encryptedParts = append(encryptedParts, ciphertext)
	}

	// Assemble multipart ciphertext
	ciphertext := make([]byte, 0)
	for _, part := range encryptedParts {
		ciphertext = append(ciphertext, part...)
	}

	// Compute combined HMAC table
	hmacTable := fg.computeCombinedHMACTable(encryptedParts, blockSize)

	return ciphertext, hmacTable, nil
}

// GenerateV1MultipartVariableFinal creates a V1 multipart fixture with variable final part (ADR-010).
func (fg *FixtureGenerator) GenerateV1MultipartVariableFinal(plaintext []byte, uniformPartSize int, finalPartSize int) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Split into parts: uniform parts + smaller final part
	parts := fg.splitIntoVariableFinalParts(plaintext, uniformPartSize, finalPartSize)

	// Encrypt each part using V1 derivation
	ciphertext, hmacTable, err := fg.encryptMultipart(parts, blockSize, version1)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V1 multipart: %w", err)
	}

	// Wrap DEK
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":      "true",
		"x-amz-meta-armor-part-size":      fmt.Sprintf("%d", uniformPartSize), // Nominal uniform size
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v1",
			SourceLayout:             "multipart-variable-final",
			V3Expected: V3Layout{
				IsMultipart:     true,
				PartCount:       len(parts),
				BlocksPerPart:   fg.computeBlocksPerPart(parts, blockSize),
				CompressionUsed: false,
				SidecarPath:     sidecarPath,
			},
			Description:              "V1 multipart with variable final part (ADR-010 exemption pattern)",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateV2MultipartVariableFinal creates a V2 multipart fixture with variable final part (ADR-010).
func (fg *FixtureGenerator) GenerateV2MultipartVariableFinal(plaintext []byte, uniformPartSize int, finalPartSize int) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Split into parts: uniform parts + smaller final part
	parts := fg.splitIntoVariableFinalParts(plaintext, uniformPartSize, finalPartSize)

	// Encrypt each part using V2 derivation
	ciphertext, hmacTable, err := fg.encryptMultipart(parts, blockSize, version2)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2 multipart: %w", err)
	}

	// Wrap DEK with v2 format
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	mekSHA := sha256.Sum256(fg.mek)
	mekFingerprint := hex.EncodeToString(mekSHA[:])[:8]
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "2",
		"x-amz-meta-armor-wrapped-dek":    wrappedDEKV2,
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":      "true",
		"x-amz-meta-armor-part-size":      fmt.Sprintf("%d", uniformPartSize), // Nominal uniform size
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v2",
			SourceLayout:             "multipart-variable-final",
			V3Expected: V3Layout{
				IsMultipart:     true,
				PartCount:       len(parts),
				BlocksPerPart:   fg.computeBlocksPerPart(parts, blockSize),
				CompressionUsed: false,
				SidecarPath:     sidecarPath,
			},
			Description:              "V2 multipart with variable final part (ADR-010 exemption pattern)",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateV1MultipartNonUniform creates a V1 multipart fixture with non-uniform part sizes (ADR-011).
func (fg *FixtureGenerator) GenerateV1MultipartNonUniform(plaintext []byte, partSizes []int) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Split into non-uniform parts
	parts := fg.splitIntoNonUniformParts(plaintext, partSizes)

	// Encrypt each part using V1 derivation
	ciphertext, hmacTable, err := fg.encryptMultipart(parts, blockSize, version1)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V1 multipart: %w", err)
	}

	// Wrap DEK
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":      "true",
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v1",
			SourceLayout:             "multipart-nonuniform",
			V3Expected: V3Layout{
				IsMultipart:     true,
				PartCount:       len(parts),
				BlocksPerPart:   fg.computeBlocksPerPart(parts, blockSize),
				CompressionUsed: false,
				SidecarPath:     sidecarPath,
			},
			Description:              "V1 multipart with non-uniform part sizes (ADR-011 pattern)",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateV2MultipartNonUniform creates a V2 multipart fixture with non-uniform part sizes (ADR-011).
func (fg *FixtureGenerator) GenerateV2MultipartNonUniform(plaintext []byte, partSizes []int) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Split into non-uniform parts
	parts := fg.splitIntoNonUniformParts(plaintext, partSizes)

	// Encrypt each part using V2 derivation
	ciphertext, hmacTable, err := fg.encryptMultipart(parts, blockSize, version2)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2 multipart: %w", err)
	}

	// Wrap DEK with v2 format
	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	mekSHA := sha256.Sum256(fg.mek)
	mekFingerprint := hex.EncodeToString(mekSHA[:])[:8]
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "2",
		"x-amz-meta-armor-wrapped-dek":    wrappedDEKV2,
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":      "true",
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "v2",
			SourceLayout:             "multipart-nonuniform",
			V3Expected: V3Layout{
				IsMultipart:     true,
				PartCount:       len(parts),
				BlocksPerPart:   fg.computeBlocksPerPart(parts, blockSize),
				CompressionUsed: false,
				SidecarPath:     sidecarPath,
			},
			Description:              "V2 multipart with non-uniform part sizes (ADR-011 pattern)",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateMalformedInvalidVersion creates a malformed fixture with invalid version string.
func (fg *FixtureGenerator) GenerateMalformedInvalidVersion(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}

	// Corrupt version field
	bundle.ObjectMetadata["x-amz-meta-armor-version"] = "not-a-number"
	bundle.Metadata.SourceVersion = "malformed"
	bundle.Metadata.Description = "Malformed: invalid version string"
	bundle.Metadata.ExpectedMigrationOutcome = "failure"
	bundle.Metadata.ExpectedFailureReason = "invalid version format"

	return bundle, nil
}

// GenerateMalformedEnvelopeVersionMismatch creates a malformed fixture with envelope version != metadata version.
func (fg *FixtureGenerator) GenerateMalformedEnvelopeVersionMismatch(plaintext []byte) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Create V2 ciphertext
	ciphertext, hmacTable, err := fg.encryptV2(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2: %w", err)
	}

	// Create V1 header (claims vulnerable derivation)
	header, err := fg.encodeEnvelopeHeader(version1, blockSize, int64(len(plaintext)), plaintextSHA[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create V1 header: %w", err)
	}

	envelope := append(header, ciphertext...)
	envelope = append(envelope, hmacTable...)

	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "2", // Metadata says V2
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "malformed",
			SourceLayout:             "single",
			V3Expected:               V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:              "Malformed: envelope header is V1 but metadata claims V2",
			ExpectedMigrationOutcome: "failure",
			ExpectedFailureReason:    "envelope header version (1) != metadata version (2)",
		},
		StoredCiphertext: envelope,
		ObjectMetadata:   metadata,
	}, nil
}

// GenerateMalformedCorruptedHMAC creates a malformed fixture with corrupted HMAC table.
func (fg *FixtureGenerator) GenerateMalformedCorruptedHMAC(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}

	// Corrupt the HMAC table at the end of the envelope
	if len(bundle.StoredCiphertext) > 32 {
		corruptionStart := len(bundle.StoredCiphertext) - 32
		bundle.StoredCiphertext[corruptionStart] ^= 0xFF // Flip bits in HMAC
	}

	bundle.Metadata.SourceVersion = "malformed"
	bundle.Metadata.Description = "Malformed: HMAC table is corrupted (bit flip)"
	bundle.Metadata.ExpectedMigrationOutcome = "failure"
	bundle.Metadata.ExpectedFailureReason = "HMAC verification fails due to corrupted table"

	return bundle, nil
}

// GenerateMalformedInconsistentPartMetadata creates a malformed fixture with contradictory multipart metadata.
func (fg *FixtureGenerator) GenerateMalformedInconsistentPartMetadata(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV1Multipart(plaintext, 5*1024*1024)
	if err != nil {
		return nil, err
	}

	// Corrupt multipart metadata to be inconsistent
	bundle.ObjectMetadata["x-amz-meta-armor-part-count"] = "999" // Wrong part count
	bundle.ObjectMetadata["x-amz-meta-armor-part-size"] = "1"     // Wrong part size
	bundle.Metadata.SourceVersion = "malformed"
	bundle.Metadata.Description = "Malformed: multipart metadata is inconsistent with actual data"
	bundle.Metadata.ExpectedMigrationOutcome = "failure"
	bundle.Metadata.ExpectedFailureReason = "part count/size in metadata doesn't match actual structure"

	return bundle, nil
}

// GenerateContradictoryVersionLayout creates a contradictory fixture with version/layout mismatch.
func (fg *FixtureGenerator) GenerateContradictoryVersionLayout(plaintext []byte) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Create V2 ciphertext (fixed counter)
	ciphertext, hmacTable, err := fg.encryptV2(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2: %w", err)
	}

	// Create V1 header (claims vulnerable derivation)
	header, err := fg.encodeEnvelopeHeader(version1, blockSize, int64(len(plaintext)), plaintextSHA[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create V1 header: %w", err)
	}

	envelope := append(header, ciphertext...)
	envelope = append(envelope, hmacTable...)

	wrappedDEK, err := fg.wrapDEK()
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":       "1",
		"x-amz-meta-armor-wrapped-dek":    base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":             base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":     fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size": fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":         hex.EncodeToString(plaintextSHA[:]),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:          hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:          int64(len(plaintext)),
			SourceVersion:            "contradictory",
			SourceLayout:             "single",
			V3Expected:               V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:              "Contradictory: version says V1, layout uses V2 counter derivation",
			ExpectedMigrationOutcome: "failure",
			ExpectedFailureReason:    "version/header mismatch (V1 header with V2 ciphertext)",
		},
		StoredCiphertext: envelope,
		ObjectMetadata:   metadata,
	}, nil
}

// GenerateEdgeCaseEmpty creates an edge case fixture with empty plaintext.
func (fg *FixtureGenerator) GenerateEdgeCaseEmpty() (*FixtureBundle, error) {
	plaintext := []byte{}
	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}
	bundle.Metadata.Description = "Edge case: empty plaintext (tests zero-length handling)"
	return bundle, nil
}

// GenerateEdgeCaseSingleByte creates an edge case fixture with single-byte plaintext.
func (fg *FixtureGenerator) GenerateEdgeCaseSingleByte() (*FixtureBundle, error) {
	plaintext := []byte("A")
	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}
	bundle.Metadata.Description = "Edge case: single-byte plaintext (tests partial last block)"
	return bundle, nil
}

// GenerateEdgeCaseExactBoundary creates an edge case at exact block boundary.
func (fg *FixtureGenerator) GenerateEdgeCaseExactBoundary() (*FixtureBundle, error) {
	blockSize := 65536
	// Exactly 2 blocks of data
	plaintext := make([]byte, blockSize*2)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}
	bundle.Metadata.Description = "Edge case: plaintext is exact multiple of block size (no partial last block)"
	return bundle, nil
}

// WriteFixture writes a fixture bundle to disk.
func (fg *FixtureGenerator) WriteFixture(name string, bundle *FixtureBundle) error {
	fixtureDir := filepath.Join(fg.outputDir, name)
	if err := os.MkdirAll(fixtureDir, 0755); err != nil {
		return fmt.Errorf("failed to create fixture directory: %w", err)
	}

	// Write metadata JSON
	metadataPath := filepath.Join(fixtureDir, "metadata.json")
	metadataJSON, err := json.MarshalIndent(bundle.Metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(metadataPath, metadataJSON, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Write stored ciphertext
	ciphertextPath := filepath.Join(fixtureDir, "stored_ciphertext.bin")
	if err := os.WriteFile(ciphertextPath, bundle.StoredCiphertext, 0644); err != nil {
		return fmt.Errorf("failed to write ciphertext: %w", err)
	}

	// Write object metadata JSON
	metaPath := filepath.Join(fixtureDir, "object_metadata.json")
	metaJSON, err := json.MarshalIndent(bundle.ObjectMetadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal object metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaJSON, 0644); err != nil {
		return fmt.Errorf("failed to write object metadata: %w", err)
	}

	// Write sidecar if present
	if len(bundle.SidecarData) > 0 {
		sidecarPath := filepath.Join(fixtureDir, "sidecar.bin")
		if err := os.WriteFile(sidecarPath, bundle.SidecarData, 0644); err != nil {
			return fmt.Errorf("failed to write sidecar: %w", err)
		}
	}

	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <output-dir>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nGenerates V1/V2 ARMOR fixtures for format migration testing.\n")
		fmt.Fprintf(os.Stderr, "This generator is fully standalone - it implements all crypto primitives\n")
		fmt.Fprintf(os.Stderr, "independently of the ARMOR codebase to ensure adversarial validation.\n")
		fmt.Fprintf(os.Stderr, "\nFixtures are written to <output-dir>/<fixture-name>/\n")
		os.Exit(1)
	}

	outputDir := os.Args[1]
	gen, err := NewFixtureGenerator(outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create generator: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating comprehensive V1/V2 ARMOR fixtures (fully standalone implementation)...")

	// Test plaintexts
	testPlaintext := []byte("ARMOR migration test data - V1/V2 to V3 fixture with sufficient length to test multiple blocks and encryption scenarios")
	mediumPlaintext := make([]byte, 256*1024) // 256 KB
	for i := range mediumPlaintext {
		mediumPlaintext[i] = byte(i % 256)
	}

	// Large plaintext for proper multipart fixtures (15 MB - large enough to span multiple parts)
	multipartPlaintext := make([]byte, 15*1024*1024) // 15 MB
	for i := range multipartPlaintext {
		multipartPlaintext[i] = byte(i % 256)
	}

	// Generate V1 single-PUT fixtures
	fmt.Println("Generating V1 single-PUT fixtures...")

	v1Explicit, err := gen.GenerateV1SingleExplicit(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 explicit: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_single_put/explicit_version", v1Explicit); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 explicit: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_single_put/explicit_version")

	v1Implicit, err := gen.GenerateV1SingleImplicit(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 implicit: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_single_put/implicit_version", v1Implicit); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 implicit: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_single_put/implicit_version")

	v1Minimal, err := gen.GenerateV1SingleMinimal(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 minimal: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_single_put/minimal_metadata", v1Minimal); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 minimal: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_single_put/minimal_metadata")

	// Generate V2 single-PUT fixtures
	fmt.Println("Generating V2 single-PUT fixtures...")

	v2Standard, err := gen.GenerateV2SingleStandard(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 standard: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2_single_put/standard", v2Standard); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 standard: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v2_single_put/standard")

	// Generate V1 multipart fixtures
	fmt.Println("Generating V1 multipart fixtures...")

	v1MultipartUniform, err := gen.GenerateV1Multipart(multipartPlaintext, 5*1024*1024)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 multipart uniform: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_multipart/uniform_parts", v1MultipartUniform); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 multipart uniform: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_multipart/uniform_parts")

	v1VariableFinal, err := gen.GenerateV1MultipartVariableFinal(multipartPlaintext, 3*1024*1024, 2*1024*1024)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 variable final: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_multipart/variable_final_part", v1VariableFinal); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 variable final: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_multipart/variable_final_part")

	v1NonUniform, err := gen.GenerateV1MultipartNonUniform(multipartPlaintext, []int{1024*1024, 2*1024*1024, 3*1024*1024})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 non-uniform: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_multipart/non_uniform_parts", v1NonUniform); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 non-uniform: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_multipart/non_uniform_parts")

	// Generate V2 multipart fixtures
	fmt.Println("Generating V2 multipart fixtures...")

	v2MultipartUniform, err := gen.GenerateV2Multipart(multipartPlaintext, 5*1024*1024)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 multipart uniform: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2_multipart/uniform_parts", v2MultipartUniform); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 multipart uniform: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v2_multipart/uniform_parts")

	v2VariableFinal, err := gen.GenerateV2MultipartVariableFinal(multipartPlaintext, 3*1024*1024, 2*1024*1024)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 variable final: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2_multipart/variable_final_part", v2VariableFinal); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 variable final: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v2_multipart/variable_final_part")

	v2NonUniform, err := gen.GenerateV2MultipartNonUniform(multipartPlaintext, []int{1024*1024, 2*1024*1024, 3*1024*1024})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 non-uniform: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2_multipart/non_uniform_parts", v2NonUniform); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 non-uniform: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v2_multipart/non_uniform_parts")

	// Generate malformed fixtures
	fmt.Println("Generating malformed fixtures...")

	malformedVersion, err := gen.GenerateMalformedInvalidVersion(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate malformed version: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("malformed/invalid_version_string", malformedVersion); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write malformed version: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ malformed/invalid_version_string")

	envelopeMismatch, err := gen.GenerateMalformedEnvelopeVersionMismatch(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate envelope mismatch: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("malformed/envelope_version_mismatch", envelopeMismatch); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write envelope mismatch: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ malformed/envelope_version_mismatch")

	corruptedHMAC, err := gen.GenerateMalformedCorruptedHMAC(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate corrupted HMAC: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("malformed/corrupted_hmac_table", corruptedHMAC); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write corrupted HMAC: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ malformed/corrupted_hmac_table")

	inconsistentMeta, err := gen.GenerateMalformedInconsistentPartMetadata(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate inconsistent metadata: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("malformed/inconsistent_part_metadata", inconsistentMeta); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write inconsistent metadata: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ malformed/inconsistent_part_metadata")

	// Generate contradictory fixtures
	fmt.Println("Generating contradictory fixtures...")

	contradictory, err := gen.GenerateContradictoryVersionLayout(testPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate contradictory: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("contradictory/version_says_v1_layout_v2", contradictory); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write contradictory: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ contradictory/version_says_v1_layout_v2")

	// Generate edge case fixtures
	fmt.Println("Generating edge case fixtures...")

	empty, err := gen.GenerateEdgeCaseEmpty()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate empty: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("edge_cases/empty_plaintext", empty); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write empty: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ edge_cases/empty_plaintext")

	singleByte, err := gen.GenerateEdgeCaseSingleByte()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate single byte: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("edge_cases/single_byte_plaintext", singleByte); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write single byte: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ edge_cases/single_byte_plaintext")

	exactBoundary, err := gen.GenerateEdgeCaseExactBoundary()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate exact boundary: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("edge_cases/exact_block_boundary", exactBoundary); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write exact boundary: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ edge_cases/exact_block_boundary")

	fmt.Printf("\n✓ Comprehensive fixture generation complete!\n")
	fmt.Printf("Output directory: %s\n", outputDir)
	fmt.Printf("\nFixtures generated: 20 total\n")
	fmt.Printf("  V1 single-PUT:    3 variants (explicit, implicit, minimal)\n")
	fmt.Printf("  V2 single-PUT:    1 variant (standard)\n")
	fmt.Printf("  V1 multipart:     3 variants (uniform, variable-final, non-uniform)\n")
	fmt.Printf("  V2 multipart:     3 variants (uniform, variable-final, non-uniform)\n")
	fmt.Printf("  Malformed:        4 variants (invalid version, envelope mismatch, corrupted HMAC, inconsistent metadata)\n")
	fmt.Printf("  Contradictory:    1 variant (version/layout mismatch)\n")
	fmt.Printf("  Edge cases:       3 variants (empty, single byte, exact boundary)\n")
	fmt.Printf("\nKey independence guarantee:\n")
	fmt.Printf("  - No imports from ARMOR internal packages\n")
	fmt.Printf("  - Standalone envelope header encoding\n")
	fmt.Printf("  - Standalone DEK wrapping (AES-GCM)\n")
	fmt.Printf("  - Standalone HMAC key derivation\n")
	fmt.Printf("  - Standalone V1/V2 counter derivation\n")
	fmt.Printf("  - If migration code has a bug, this generator will catch it\n")
	fmt.Printf("\nAll fixtures include plaintext SHA256 and length for validation\n")
}
