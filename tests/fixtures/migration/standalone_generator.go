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

	fmt.Println("Generating V1/V2 ARMOR fixtures (fully standalone implementation)...")

	// Test plaintexts
	shortPlaintext := []byte("ARMOR migration test data - V1/V2 to V3 fixture")
	mediumPlaintext := make([]byte, 256*1024) // 256 KB
	for i := range mediumPlaintext {
		mediumPlaintext[i] = byte(i % 256)
	}

	// Generate V1 single-PUT fixtures
	v1Explicit, err := gen.GenerateV1SingleExplicit(shortPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 explicit: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1-single-explicit-short", v1Explicit); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 explicit: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Generated v1-single-explicit-short")

	v1Implicit, err := gen.GenerateV1SingleImplicit(shortPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 implicit: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1-single-implicit-short", v1Implicit); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 implicit: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Generated v1-single-implicit-short")

	// Generate V2 single-PUT fixtures
	v2Single, err := gen.GenerateV2Single(shortPlaintext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 single: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2-single-short", v2Single); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 single: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Generated v2-single-short")

	// Generate V1 multipart fixtures (5MB parts)
	v1Multipart, err := gen.GenerateV1Multipart(mediumPlaintext, 5*1024*1024)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 multipart: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1-multipart-uniform", v1Multipart); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 multipart: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Generated v1-multipart-uniform")

	// Generate V2 multipart fixtures (5MB parts)
	v2Multipart, err := gen.GenerateV2Multipart(mediumPlaintext, 5*1024*1024)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 multipart: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2-multipart-uniform", v2Multipart); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 multipart: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Generated v2-multipart-uniform")

	fmt.Printf("\n✓ Fixture generation complete!\n")
	fmt.Printf("Output directory: %s\n", outputDir)
	fmt.Printf("\nFixtures generated:\n")
	fmt.Printf("  - v1-single-explicit-short: V1 single-PUT with explicit version metadata\n")
	fmt.Printf("  - v1-single-implicit-short: V1 single-PUT with missing version metadata\n")
	fmt.Printf("  - v2-single-short:          V2 single-PUT object\n")
	fmt.Printf("  - v1-multipart-uniform:      V1 multipart (256 KB, 1 part)\n")
	fmt.Printf("  - v2-multipart-uniform:      V2 multipart (256 KB, 1 part)\n")
	fmt.Printf("\nKey independence guarantee:\n")
	fmt.Printf("  - No imports from ARMOR internal packages\n")
	fmt.Printf("  - Standalone envelope header encoding\n")
	fmt.Printf("  - Standalone DEK wrapping (AES-GCM)\n")
	fmt.Printf("  - Standalone HMAC key derivation\n")
	fmt.Printf("  - Standalone V1/V2 counter derivation\n")
	fmt.Printf("  - If migration code has a bug, this generator will catch it\n")
}
