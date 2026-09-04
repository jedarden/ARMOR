// Package canonical holds the legacy crypto-backed V1/V2 fixture generator.
// It calls internal/crypto instead of reimplementing the envelope format, so
// unlike ../standalone_generator.go it is not an independent oracle. It lives
// in its own directory because Go allows one package per directory and both
// programs declare the same symbols; keeping them together broke the package
// for every ./... build and lint run.
package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/jedarden/armor/internal/crypto"
)

// FixtureMetadata records the plaintext properties and expected V3 layout.
type FixtureMetadata struct {
	PlaintextSHA256        string   `json:"plaintext_sha256"`
	PlaintextLength        int64    `json:"plaintext_length"`
	SourceVersion          string   `json:"source_version"`    // "v1", "v2", "malformed"
	SourceLayout           string   `json:"source_layout"`     // "single", "multipart-uniform", "multipart-nonuniform", "multipart-variable-final"
	V3Expected             V3Layout `json:"v3_expected"`
	Description            string   `json:"description"`
	KnownBugs              []string `json:"known_bugs,omitempty"`
	ExpectedMigrationOutcome string `json:"expected_migration_outcome"` // "success", "failure", "skip"
	ExpectedFailureReason  string   `json:"expected_failure_reason,omitempty"`
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
	Metadata        FixtureMetadata   `json:"metadata"`
	StoredCiphertext []byte          `json:"-"` // Stored in separate file
	ObjectMetadata   map[string]string `json:"object_metadata"`
	SidecarData      []byte          `json:"-"` // For multipart, stored separately
}

// FixtureGenerator creates canonical fixtures.
type FixtureGenerator struct {
	outputDir string
	mek       []byte
	dek       []byte
	iv        []byte
}

// NewFixtureGenerator creates a new fixture generator.
func NewFixtureGenerator(outputDir string) (*FixtureGenerator, error) {
	mek := make([]byte, 32)
	dek := make([]byte, 32)
	iv := make([]byte, 16)

	// Use deterministic values for reproducibility
	for i := range mek {
		mek[i] = byte(i + 1)
		dek[i] = byte(i + 2)
	}
	for i := range iv {
		iv[i] = byte(i + 3)
	}

	return &FixtureGenerator{
		outputDir: outputDir,
		mek:       mek,
		dek:       dek,
		iv:        iv,
	}, nil
}

// GenerateV1SingleExplicit creates a V1 single-PUT fixture with explicit version metadata.
func (fg *FixtureGenerator) GenerateV1SingleExplicit(plaintext []byte) (*FixtureBundle, error) {
	blockSize := 65536 // 64 KB
	plaintextSHA := sha256.Sum256(plaintext)

	// Create V1 envelope header
	header, err := crypto.NewEnvelopeHeaderWithVersion(fg.iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version1)
	if err != nil {
		return nil, fmt.Errorf("failed to create V1 header: %w", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	// Encrypt using V1 counter derivation (vulnerable keystream reuse)
	ciphertext, hmacTable, err := fg.encryptV1(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V1: %w", err)
	}

	// Build envelope: header || ciphertext || hmacTable
	envelope := append(headerBuf, ciphertext...)
	envelope = append(envelope, hmacTable...)

	// Wrap DEK for storage
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:         hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:         int64(len(plaintext)),
			SourceVersion:           "v1",
			SourceLayout:            "single",
			V3Expected:             V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:             "V1 single-PUT with explicit version metadata",
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

// GenerateV1SingleMinimal creates a V1 single-PUT fixture with minimal metadata.
func (fg *FixtureGenerator) GenerateV1SingleMinimal(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV1SingleExplicit(plaintext)
	if err != nil {
		return nil, err
	}

	// Keep only required fields
	requiredKeys := map[string]bool{
		"x-amz-meta-armor-wrapped-dek":     true,
		"x-amz-meta-armor-iv":              true,
		"x-amz-meta-armor-block-size":      true,
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
	blockSize := 65536 // 64 KB
	plaintextSHA := sha256.Sum256(plaintext)

	// Create V2 envelope header
	header, err := crypto.NewEnvelopeHeaderWithVersion(fg.iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version2)
	if err != nil {
		return nil, fmt.Errorf("failed to create V2 header: %w", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	// Encrypt using V2 counter derivation (fixed, no keystream reuse)
	ciphertext, hmacTable, err := fg.encryptV2(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2: %w", err)
	}

	// Build envelope: header || ciphertext || hmacTable
	envelope := append(headerBuf, ciphertext...)
	envelope = append(envelope, hmacTable...)

	// Wrap DEK for storage with v2 fingerprint format
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	mekFingerprint := crypto.MEKFingerprint(fg.mek)
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "2",
		"x-amz-meta-armor-wrapped-dek":     wrappedDEKV2,
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-etag":            hex.EncodeToString(plaintextSHA[:]), // Simplified ETag
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:         hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:         int64(len(plaintext)),
			SourceVersion:           "v2",
			SourceLayout:            "single",
			V3Expected:             V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:             "V2 single-PUT with full metadata (fixed counter derivation)",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: envelope,
		ObjectMetadata:   metadata,
	}, nil
}

// GenerateV1MultipartUniform creates a V1 multipart fixture with uniform part sizes.
func (fg *FixtureGenerator) GenerateV1MultipartUniform(plaintext []byte, partSize int) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Split into uniform parts
	parts := fg.splitIntoUniformParts(plaintext, partSize)

	// Encrypt each part using V1 derivation (vulnerable, but each part starts fresh)
	encryptedParts := make([][]byte, len(parts))
	for i, part := range parts {
		ciphertext, _, err := fg.encryptV1(part, blockSize)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt V1 part %d: %w", i, err)
		}
		encryptedParts[i] = ciphertext
	}

	// Assemble multipart ciphertext (concatenation of encrypted parts)
	ciphertext := make([]byte, 0)
	for _, part := range encryptedParts {
		ciphertext = append(ciphertext, part...)
	}

	// Compute combined HMAC table (for all blocks across all parts)
	hmacTable := fg.computeCombinedHMACTable(encryptedParts, blockSize)

	// Wrap DEK
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":       "true",
		"x-amz-meta-armor-part-size":       fmt.Sprintf("%d", partSize),
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:         hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:         int64(len(plaintext)),
			SourceVersion:           "v1",
			SourceLayout:            "multipart-uniform",
			V3Expected: V3Layout{
				IsMultipart:   true,
				PartCount:     len(parts),
				BlocksPerPart: fg.computeBlocksPerPart(parts, blockSize),
				SidecarPath:   sidecarPath,
			},
			Description:             "V1 multipart with uniform part sizes (legacy ADR-005 contract)",
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

// GenerateContradictoryVersionLayout creates a fixture with contradictory version/layout.
func (fg *FixtureGenerator) GenerateContradictoryVersionLayout(plaintext []byte) (*FixtureBundle, error) {
	// Create V2 ciphertext (fixed counter)
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	ciphertext, hmacTable, err := fg.encryptV2(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2: %w", err)
	}

	// Create V1 header (claims vulnerable derivation)
	header, err := crypto.NewEnvelopeHeaderWithVersion(fg.iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version1)
	if err != nil {
		return nil, fmt.Errorf("failed to create V1 header: %w", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	envelope := append(headerBuf, ciphertext...)
	envelope = append(envelope, hmacTable...)

	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:         hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:         int64(len(plaintext)),
			SourceVersion:           "contradictory",
			SourceLayout:            "single",
			V3Expected:             V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:             "Contradictory: version says V1, layout uses V2 counter derivation",
			ExpectedMigrationOutcome: "failure",
			ExpectedFailureReason:  "version/header mismatch (V1 header with V2 ciphertext)",
		},
		StoredCiphertext: envelope,
		ObjectMetadata:   metadata,
	}, nil
}

// GenerateEdgeCaseEmpty creates an edge case fixture with empty plaintext.
func (fg *FixtureGenerator) GenerateEdgeCaseEmpty() (*FixtureBundle, error) {
	plaintext := []byte{}
	return fg.GenerateV1SingleExplicit(plaintext)
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

// GenerateV2MultipartUniform creates a V2 multipart fixture with uniform part sizes.
func (fg *FixtureGenerator) GenerateV2MultipartUniform(plaintext []byte, partSize int) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Split into uniform parts
	parts := fg.splitIntoUniformParts(plaintext, partSize)

	// Encrypt each part using V2 derivation (fixed counter)
	encryptedParts := make([][]byte, len(parts))
	for i, part := range parts {
		ciphertext, _, err := fg.encryptV2(part, blockSize)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt V2 part %d: %w", i, err)
		}
		encryptedParts[i] = ciphertext
	}

	// Assemble multipart ciphertext (concatenation of encrypted parts)
	ciphertext := make([]byte, 0)
	for _, part := range encryptedParts {
		ciphertext = append(ciphertext, part...)
	}

	// Compute combined HMAC table (for all blocks across all parts)
	hmacTable := fg.computeCombinedHMACTable(encryptedParts, blockSize)

	// Wrap DEK with v2 fingerprint format
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	mekFingerprint := crypto.MEKFingerprint(fg.mek)
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "2",
		"x-amz-meta-armor-wrapped-dek":     wrappedDEKV2,
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-etag":            hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":       "true",
		"x-amz-meta-armor-part-size":       fmt.Sprintf("%d", partSize),
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256: hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength: int64(len(plaintext)),
			SourceVersion:   "v2",
			SourceLayout:    "multipart-uniform",
			V3Expected: V3Layout{
				IsMultipart:   true,
				PartCount:     len(parts),
				BlocksPerPart: fg.computeBlocksPerPart(parts, blockSize),
				SidecarPath:   sidecarPath,
			},
			Description:             "V2 multipart with uniform part sizes (fixed counter derivation)",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateV1MultipartVariableFinal creates a V1 multipart fixture with variable final part (ADR-010).
func (fg *FixtureGenerator) GenerateV1MultipartVariableFinal(plaintext []byte, uniformPartSize int, finalPartSize int) (*FixtureBundle, error) {
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Split into parts: uniform parts + smaller final part
	parts := fg.splitIntoVariableFinalParts(plaintext, uniformPartSize, finalPartSize)

	// Encrypt each part using V1 derivation
	encryptedParts := make([][]byte, len(parts))
	for i, part := range parts {
		ciphertext, _, err := fg.encryptV1(part, blockSize)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt V1 part %d: %w", i, err)
		}
		encryptedParts[i] = ciphertext
	}

	// Assemble multipart ciphertext
	ciphertext := make([]byte, 0)
	for _, part := range encryptedParts {
		ciphertext = append(ciphertext, part...)
	}

	// Compute combined HMAC table
	hmacTable := fg.computeCombinedHMACTable(encryptedParts, blockSize)

	// Wrap DEK
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":       "true",
		"x-amz-meta-armor-part-size":       fmt.Sprintf("%d", uniformPartSize), // Nominal uniform size
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256: hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength: int64(len(plaintext)),
			SourceVersion:   "v1",
			SourceLayout:    "multipart-variable-final",
			V3Expected: V3Layout{
				IsMultipart:   true,
				PartCount:     len(parts),
				BlocksPerPart: fg.computeBlocksPerPart(parts, blockSize),
				SidecarPath:   sidecarPath,
			},
			Description:             "V1 multipart with variable final part (ADR-010 exemption pattern)",
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
	encryptedParts := make([][]byte, len(parts))
	for i, part := range parts {
		ciphertext, _, err := fg.encryptV2(part, blockSize)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt V2 part %d: %w", i, err)
		}
		encryptedParts[i] = ciphertext
	}

	// Assemble multipart ciphertext
	ciphertext := make([]byte, 0)
	for _, part := range encryptedParts {
		ciphertext = append(ciphertext, part...)
	}

	// Compute combined HMAC table
	hmacTable := fg.computeCombinedHMACTable(encryptedParts, blockSize)

	// Wrap DEK with v2 format
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	mekFingerprint := crypto.MEKFingerprint(fg.mek)
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "2",
		"x-amz-meta-armor-wrapped-dek":     wrappedDEKV2,
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-etag":            hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":       "true",
		"x-amz-meta-armor-part-size":       fmt.Sprintf("%d", uniformPartSize), // Nominal uniform size
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256: hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength: int64(len(plaintext)),
			SourceVersion:   "v2",
			SourceLayout:    "multipart-variable-final",
			V3Expected: V3Layout{
				IsMultipart:   true,
				PartCount:     len(parts),
				BlocksPerPart: fg.computeBlocksPerPart(parts, blockSize),
				SidecarPath:   sidecarPath,
			},
			Description:             "V2 multipart with variable final part (ADR-010 exemption pattern)",
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
	encryptedParts := make([][]byte, len(parts))
	for i, part := range parts {
		ciphertext, _, err := fg.encryptV1(part, blockSize)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt V1 part %d: %w", i, err)
		}
		encryptedParts[i] = ciphertext
	}

	// Assemble multipart ciphertext
	ciphertext := make([]byte, 0)
	for _, part := range encryptedParts {
		ciphertext = append(ciphertext, part...)
	}

	// Compute combined HMAC table
	hmacTable := fg.computeCombinedHMACTable(encryptedParts, blockSize)

	// Wrap DEK
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "1",
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":       "true",
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256: hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength: int64(len(plaintext)),
			SourceVersion:   "v1",
			SourceLayout:    "multipart-nonuniform",
			V3Expected: V3Layout{
				IsMultipart:   true,
				PartCount:     len(parts),
				BlocksPerPart: fg.computeBlocksPerPart(parts, blockSize),
				SidecarPath:   sidecarPath,
			},
			Description:             "V1 multipart with non-uniform part sizes (ADR-011 pattern)",
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
	encryptedParts := make([][]byte, len(parts))
	for i, part := range parts {
		ciphertext, _, err := fg.encryptV2(part, blockSize)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt V2 part %d: %w", i, err)
		}
		encryptedParts[i] = ciphertext
	}

	// Assemble multipart ciphertext
	ciphertext := make([]byte, 0)
	for _, part := range encryptedParts {
		ciphertext = append(ciphertext, part...)
	}

	// Compute combined HMAC table
	hmacTable := fg.computeCombinedHMACTable(encryptedParts, blockSize)

	// Wrap DEK with v2 format
	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	mekFingerprint := crypto.MEKFingerprint(fg.mek)
	wrappedDEKV2 := fmt.Sprintf("v2:%s:%s", mekFingerprint, base64.StdEncoding.EncodeToString(wrappedDEK))

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "2",
		"x-amz-meta-armor-wrapped-dek":     wrappedDEKV2,
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-etag":            hex.EncodeToString(plaintextSHA[:]),
		"x-amz-meta-armor-multipart":       "true",
	}

	// Compute sidecar path
	keySHA := sha256.Sum256([]byte("test-object-key"))
	sidecarPath := fmt.Sprintf(".armor/hmac/%x", keySHA)

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256: hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength: int64(len(plaintext)),
			SourceVersion:   "v2",
			SourceLayout:    "multipart-nonuniform",
			V3Expected: V3Layout{
				IsMultipart:   true,
				PartCount:     len(parts),
				BlocksPerPart: fg.computeBlocksPerPart(parts, blockSize),
				SidecarPath:   sidecarPath,
			},
			Description:             "V2 multipart with non-uniform part sizes (ADR-011 pattern)",
			ExpectedMigrationOutcome: "success",
		},
		StoredCiphertext: ciphertext,
		ObjectMetadata:   metadata,
		SidecarData:      hmacTable,
	}, nil
}

// GenerateMalformedEnvelopeVersionMismatch creates a malformed fixture with envelope version != metadata version.
func (fg *FixtureGenerator) GenerateMalformedEnvelopeVersionMismatch(plaintext []byte) (*FixtureBundle, error) {
	// Create V1 metadata but V2 envelope (or vice versa)
	blockSize := 65536
	plaintextSHA := sha256.Sum256(plaintext)

	// Create V2 ciphertext
	ciphertext, hmacTable, err := fg.encryptV2(plaintext, blockSize)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt V2: %w", err)
	}

	// Create V1 header (claims vulnerable derivation)
	header, err := crypto.NewEnvelopeHeaderWithVersion(fg.iv, int64(len(plaintext)), blockSize, plaintextSHA, crypto.Version1)
	if err != nil {
		return nil, fmt.Errorf("failed to create V1 header: %w", err)
	}

	headerBuf, err := header.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode header: %w", err)
	}

	envelope := append(headerBuf, ciphertext...)
	envelope = append(envelope, hmacTable...)

	wrappedDEK, err := crypto.WrapDEK(fg.mek, fg.dek)
	if err != nil {
		return nil, fmt.Errorf("failed to wrap DEK: %w", err)
	}

	metadata := map[string]string{
		"x-amz-meta-armor-version":        "2", // Metadata says V2
		"x-amz-meta-armor-wrapped-dek":     base64.StdEncoding.EncodeToString(wrappedDEK),
		"x-amz-meta-armor-iv":              base64.StdEncoding.EncodeToString(fg.iv),
		"x-amz-meta-armor-block-size":      fmt.Sprintf("%d", blockSize),
		"x-amz-meta-armor-plaintext-size":  fmt.Sprintf("%d", len(plaintext)),
		"x-amz-meta-armor-sha256":          hex.EncodeToString(plaintextSHA[:]),
	}

	return &FixtureBundle{
		Metadata: FixtureMetadata{
			PlaintextSHA256:         hex.EncodeToString(plaintextSHA[:]),
			PlaintextLength:         int64(len(plaintext)),
			SourceVersion:           "malformed",
			SourceLayout:            "single",
			V3Expected:             V3Layout{IsMultipart: false, CompressionUsed: false},
			Description:             "Malformed: envelope header is V1 but metadata claims V2",
			ExpectedMigrationOutcome: "failure",
			ExpectedFailureReason:  "envelope header version (1) != metadata version (2)",
		},
		StoredCiphertext: envelope,
		ObjectMetadata:   metadata,
	}, nil
}

// GenerateMalformedInconsistentPartMetadata creates a malformed fixture with contradictory multipart metadata.
func (fg *FixtureGenerator) GenerateMalformedInconsistentPartMetadata(plaintext []byte) (*FixtureBundle, error) {
	bundle, err := fg.GenerateV1MultipartUniform(plaintext, 5*1024*1024)
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

// encryptV1 encrypts using V1 counter derivation (vulnerable).
func (fg *FixtureGenerator) encryptV1(plaintext []byte, blockSize int) ([]byte, []byte, error) {
	block, err := aes.NewCipher(fg.dek)
	if err != nil {
		return nil, nil, err
	}

	// V1 counter derivation: just blockIndex (VULNERABLE - causes keystream reuse)
	blockCount := int(math.Ceil(float64(len(plaintext)) / float64(blockSize)))
	ciphertext := make([]byte, 0)
	hmacTable := make([]byte, 0)

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

		// V1 counter derivation: just blockIndex (VULNERABLE)
		blockCtr := make([]byte, 16)
		copy(blockCtr[0:12], fg.iv[0:12])
		binary.BigEndian.PutUint32(blockCtr[12:16], uint32(blockIndex))

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

// encryptV2 encrypts using V2 counter derivation (fixed).
func (fg *FixtureGenerator) encryptV2(plaintext []byte, blockSize int) ([]byte, []byte, error) {
	block, err := aes.NewCipher(fg.dek)
	if err != nil {
		return nil, nil, err
	}

	// V2 counter derivation: blockIndex * (blockSize / 16) with proper stride
	// Fixed: ensures no keystream reuse between blocks
	blockCount := int(math.Ceil(float64(len(plaintext)) / float64(blockSize)))
	ciphertext := make([]byte, 0)
	hmacTable := make([]byte, 0)

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

		// V2 counter derivation (fixed)
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

// deriveHMACKey derives the HMAC key from the DEK using HKDF-SHA256.
func (fg *FixtureGenerator) deriveHMACKey() ([]byte, error) {
	return crypto.DeriveHMACKey(fg.dek)
}

// computeBlockHMAC computes HMAC-SHA256 for a single encrypted block.
func (fg *FixtureGenerator) computeBlockHMAC(hmacKey []byte, encryptedBlock []byte, blockIndex int) []byte {
	// HMAC = HMAC-SHA256(hmacKey, uint32(blockIndex) || ciphertext_block)
	mac := hmac.New(sha256.New, hmacKey)

	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, uint32(blockIndex))
	mac.Write(indexBytes)
	mac.Write(encryptedBlock)

	return mac.Sum(nil)
}

// splitIntoUniformParts splits plaintext into uniform-sized parts.
func (fg *FixtureGenerator) splitIntoUniformParts(plaintext []byte, partSize int) [][]byte {
	if partSize <= 0 {
		partSize = 5 * 1024 * 1024 // Default 5MB
	}

	var parts [][]byte
	for i := 0; i < len(plaintext); i += partSize {
		end := i + partSize
		if end > len(plaintext) {
			end = len(plaintext)
		}
		parts = append(parts, plaintext[i:end])
	}

	return parts
}

// splitIntoVariableFinalParts splits plaintext into uniform parts with a smaller final part.
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

// splitIntoNonUniformParts splits plaintext into parts with explicit sizes.
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

// WriteFixture writes a fixture bundle to disk.
func (fg *FixtureGenerator) WriteFixture(name string, bundle *FixtureBundle) error {
	fixtureDir := filepath.Join(fg.outputDir, name)
	if err := os.MkdirAll(fixtureDir, 0755); err != nil {
		return fmt.Errorf("failed to create fixture directory: %w", err)
	}

	// Write metadata
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

	// Write object metadata
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
		os.Exit(1)
	}

	outputDir := os.Args[1]
	gen, err := NewFixtureGenerator(outputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create generator: %v\n", err)
		os.Exit(1)
	}

	// Generate test plaintext
	testPlaintext := []byte("ARMOR migration test data - V1/V2 to V3 fixture with sufficient length to test multiple blocks")

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
	v1Multipart, err := gen.GenerateV1MultipartUniform(testPlaintext, 5*1024*1024) // 5MB parts
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 multipart: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_multipart/uniform_parts", v1Multipart); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 multipart: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_multipart/uniform_parts")

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

	// Generate V2 multipart fixtures
	fmt.Println("Generating V2 multipart fixtures...")
	v2Multipart, err := gen.GenerateV2MultipartUniform(testPlaintext, 5*1024*1024) // 5MB parts
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 multipart: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2_multipart/uniform_parts", v2Multipart); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 multipart: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v2_multipart/uniform_parts")

	// Generate ADR-010 variable-final-part fixtures
	fmt.Println("Generating ADR-010 variable-final-part fixtures...")
	v1VariableFinal, err := gen.GenerateV1MultipartVariableFinal(testPlaintext, 3*1024*1024, 2*1024*1024) // 3MB uniform, 2MB final
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 variable final: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_multipart/variable_final_part", v1VariableFinal); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 variable final: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_multipart/variable_final_part")

	v2VariableFinal, err := gen.GenerateV2MultipartVariableFinal(testPlaintext, 3*1024*1024, 2*1024*1024) // 3MB uniform, 2MB final
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 variable final: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2_multipart/variable_final_part", v2VariableFinal); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 variable final: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v2_multipart/variable_final_part")

	// Generate ADR-011 non-uniform multipart fixtures
	fmt.Println("Generating ADR-011 non-uniform multipart fixtures...")
	// Non-uniform part sizes: 1MB, 2MB, 3MB, remainder
	nonUniformSizes := []int{1024 * 1024, 2 * 1024 * 1024, 3 * 1024 * 1024}
	v1NonUniform, err := gen.GenerateV1MultipartNonUniform(testPlaintext, nonUniformSizes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V1 non-uniform: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v1_multipart/non_uniform_parts", v1NonUniform); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V1 non-uniform: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v1_multipart/non_uniform_parts")

	v2NonUniform, err := gen.GenerateV2MultipartNonUniform(testPlaintext, nonUniformSizes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate V2 non-uniform: %v\n", err)
		os.Exit(1)
	}
	if err := gen.WriteFixture("v2_multipart/non_uniform_parts", v2NonUniform); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write V2 non-uniform: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("  ✓ v2_multipart/non_uniform_parts")

	// Generate additional malformed fixtures
	fmt.Println("Generating additional malformed fixtures...")
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

	fmt.Println("\n✓ Fixture generation complete!")
	fmt.Printf("Output directory: %s\n", outputDir)
}
