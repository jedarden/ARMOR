package crypto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// V3TestVector represents a single v3 test case.
type V3TestVector struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	DEK          string            `json:"dek"`          // Base64-encoded 32-byte DEK
	IV           string            `json:"iv"`           // Base64-encoded 16-byte IV
	Part         uint16            `json:"part"`
	BlockSize    int               `json:"block_size"`
	Plaintext    string            `json:"plaintext"`    // Base64-encoded plaintext
	Header       string            `json:"header"`       // Base64-encoded 64-byte header
	Ciphertext   string            `json:"ciphertext"`   // Base64-encoded ciphertext
	HMAC         string            `json:"hmac"`         // Base64-encoded 32-byte HMAC
	Blocks       []V3BlockEntry    `json:"blocks"`       // Block table entries
	Sidecar      *V3Sidecar        `json:"sidecar,omitempty"` // For multipart tests
}

// V3BlockEntry represents a single block's table entry.
type V3BlockEntry struct {
	HMAC string `json:"hmac"` // Base64-encoded 32-byte HMAC
	CLen uint32 `json:"clen"`  // Ciphertext length with compression flag
}

// V3Sidecar represents the v3 sidecar format.
type V3Sidecar struct {
	Version   int           `json:"version"`
	BlockSize int           `json:"block_size"`
	Parts     []V3PartEntry `json:"parts"`
}

// V3PartEntry represents a part in the sidecar.
type V3PartEntry struct {
	N              uint16        `json:"n"`               // Part number
	PlaintextLen   int64         `json:"plaintext_len"`   // Total plaintext size
	CiphertextLen  int64         `json:"ciphertext_len"`  // Total ciphertext size
	Blocks         []V3BlockEntry `json:"blocks"`         // Array of [hmac, clen] pairs
}

// TestGenerateV3Vectors generates v3 test vectors.
// Run with: go test ./internal/crypto -run TestGenerateV3Vectors -update
func TestGenerateV3Vectors(t *testing.T) {
	update := false
	for _, arg := range os.Args {
		if arg == "-update" {
			update = true
			break
		}
	}

	testDir := "testdata/v3"
	if update {
		require.NoError(t, os.MkdirAll(testDir, 0755), "Failed to create testdata directory")
	}

	vectors := []V3TestVector{
		generate1BlockSinglePUT(),
		generate3BlockCompressed(),
		generate2PartMultipart(),
	}

	for i := range vectors {
		vec := &vectors[i]
		t.Run(vec.Name, func(t *testing.T) {
			filename := filepath.Join(testDir, fmt.Sprintf("%s.json", vec.Name))

			if update {
				// Write the vector
				data, err := json.MarshalIndent(vec, "", "  ")
				require.NoError(t, err, "Failed to marshal vector")

				err = os.WriteFile(filename, data, 0644)
				require.NoError(t, err, "Failed to write vector file")

				t.Logf("Generated test vector: %s", filename)
			} else {
				// Verify the vector exists and is valid
				data, err := os.ReadFile(filename)
				require.NoError(t, err, "Failed to read vector file (run with -update to generate)")

				var loaded V3TestVector
				err = json.Unmarshal(data, &loaded)
				require.NoError(t, err, "Failed to unmarshal vector")

				// Compare individual fields instead of struct equality
				assert.Equal(t, vec.Name, loaded.Name, "Name should match")
				assert.Equal(t, vec.Description, loaded.Description, "Description should match")
				assert.Equal(t, vec.DEK, loaded.DEK, "DEK should match")
				assert.Equal(t, vec.IV, loaded.IV, "IV should match")
				assert.Equal(t, vec.Part, loaded.Part, "Part should match")
				assert.Equal(t, vec.BlockSize, loaded.BlockSize, "BlockSize should match")
				assert.Equal(t, vec.Plaintext, loaded.Plaintext, "Plaintext should match")
				assert.Equal(t, vec.Header, loaded.Header, "Header should match")
				assert.Equal(t, vec.Ciphertext, loaded.Ciphertext, "Ciphertext should match")
				assert.Equal(t, vec.HMAC, loaded.HMAC, "HMAC should match")
				assert.Equal(t, len(vec.Blocks), len(loaded.Blocks), "Block count should match")

				// Verify the vector works: encrypt and decrypt should match
				verifyTestVector(t, &loaded)
			}
		})
	}
}

// generate1BlockSinglePUT generates a minimal single-PUT test vector with 1 block.
func generate1BlockSinglePUT() V3TestVector {
	// Fixed DEK and IV for reproducibility
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 1)
	}

	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 10)
	}

	part := uint16(0) // Single-PUT
	blockSize := 64 * 1024

	// Small plaintext that fits in one block
	plaintext := []byte("Hello, ARMOR v3! This is a test plaintext for the 1-block single-PUT vector.")

	// Create header
	plaintextSHA := ComputePlaintextSHA256(plaintext)
	header, err := NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, Version3)
	if err != nil {
		panic(fmt.Sprintf("Failed to create header: %v", err))
	}
	headerBytes, err := header.Encode()
	if err != nil {
		panic(fmt.Sprintf("Failed to encode header: %v", err))
	}

	// Encrypt the block
	ciphertext, hmacValue, err := EncryptBlockV3(dek, iv, part, 0, plaintext, blockSize)
	if err != nil {
		panic(fmt.Sprintf("Failed to encrypt block: %v", err))
	}

	return V3TestVector{
		Name:        "1-block-single-put",
		Description: "Minimal single-PUT object (1 block, uncompressed)",
		DEK:         base64.StdEncoding.EncodeToString(dek),
		IV:          base64.StdEncoding.EncodeToString(iv),
		Part:        part,
		BlockSize:   blockSize,
		Plaintext:   base64.StdEncoding.EncodeToString(plaintext),
		Header:      base64.StdEncoding.EncodeToString(headerBytes),
		Ciphertext:  base64.StdEncoding.EncodeToString(ciphertext),
		HMAC:        base64.StdEncoding.EncodeToString(hmacValue),
		Blocks: []V3BlockEntry{
			{
				HMAC: base64.StdEncoding.EncodeToString(hmacValue),
				CLen: uint32(len(ciphertext)), // No compression flag
			},
		},
	}
}

// generate3BlockCompressed generates a 3-block single-PUT with middle block compressible.
func generate3BlockCompressed() V3TestVector {
	// Fixed DEK and IV
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 2)
	}

	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 20)
	}

	part := uint16(0)
	blockSize := 64 * 1024

	// Create 3 blocks of plaintext
	// Block 0: Pseudo-random data (incompressible) - deterministic pattern
	block0 := make([]byte, blockSize)
	for i := range block0 {
		block0[i] = byte((i * 7 + 13) % 256)
	}

	// Block 1: Repetitive data (compressible)
	block1 := make([]byte, blockSize)
	for i := range block1 {
		block1[i] = byte(i % 256) // Pattern that compresses well
	}

	// Block 2: Mixed data (somewhat compressible)
	block2 := make([]byte, blockSize/2) // Partial block
	for i := range block2 {
		block2[i] = byte((i * 3 + 7) % 256)
	}

	plaintext := append(append(block0, block1...), block2...)

	// Create header
	plaintextSHA := ComputePlaintextSHA256(plaintext)
	header, err := NewEnvelopeHeaderWithVersion(iv, int64(len(plaintext)), blockSize, plaintextSHA, Version3)
	if err != nil { panic(fmt.Sprintf("Failed to create header: %v", err)) }
	headerBytes, err := header.Encode()
	if err != nil { panic(fmt.Sprintf("Failed to encode header: %v", err)) }

	// Encrypt each block
	var fullCiphertext []byte
	blocks := make([]V3BlockEntry, 0)

	blockCount := ComputeBlockCount(int64(len(plaintext)), blockSize)
	for blockIdx := uint32(0); blockIdx < blockCount; blockIdx++ {
		start := int(blockIdx) * blockSize
		end := start + blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		blockPlaintext := plaintext[start:end]
		blockCiphertext, blockHMAC, err := EncryptBlockV3(dek, iv, part, blockIdx, blockPlaintext, blockSize)
		if err != nil { panic(fmt.Sprintf("Failed to encrypt block %d: %v", blockIdx, err)) }

		fullCiphertext = append(fullCiphertext, blockCiphertext...)

		blocks = append(blocks, V3BlockEntry{
			HMAC: base64.StdEncoding.EncodeToString(blockHMAC),
			CLen: uint32(len(blockCiphertext)), // No compression in this test
		})
	}

	// Use HMAC of first block for the top-level HMAC field
	var firstBlockHMAC []byte
	if len(blocks) > 0 {
		firstBlockHMAC, _ = base64.StdEncoding.DecodeString(blocks[0].HMAC)
		if err != nil { panic(fmt.Sprintf("Failed to decode HMAC: %v", err)) }
	}

	return V3TestVector{
		Name:        "3-block-compressed",
		Description: "Single-PUT object (3 blocks, middle block compressible)",
		DEK:         base64.StdEncoding.EncodeToString(dek),
		IV:          base64.StdEncoding.EncodeToString(iv),
		Part:        part,
		BlockSize:   blockSize,
		Plaintext:   base64.StdEncoding.EncodeToString(plaintext),
		Header:      base64.StdEncoding.EncodeToString(headerBytes),
		Ciphertext:  base64.StdEncoding.EncodeToString(fullCiphertext),
		HMAC:        base64.StdEncoding.EncodeToString(firstBlockHMAC),
		Blocks:      blocks,
	}
}

// generate2PartMultipart generates a 2-part multipart test vector.
func generate2PartMultipart() V3TestVector {
	// Fixed DEK and IV
	dek := make([]byte, 32)
	for i := range dek {
		dek[i] = byte(i + 3)
	}

	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i + 30)
	}

	blockSize := 64 * 1024

	// Part 1: 1 full block (64 KiB)
	part1Plaintext := make([]byte, blockSize)
	for i := range part1Plaintext {
		part1Plaintext[i] = byte(i % 256)
	}

	// Part 2: 2 blocks (one full, one partial)
	part2Block0 := make([]byte, blockSize)
	for i := range part2Block0 {
		part2Block0[i] = byte((i + 100) % 256)
	}
	part2Block1 := make([]byte, blockSize/2) // Partial block
	for i := range part2Block1 {
		part2Block1[i] = byte((i + 200) % 256)
	}
	part2Plaintext := append(part2Block0, part2Block1...)

	// Create sidecar
	sidecar := &V3Sidecar{
		Version:   3,
		BlockSize: blockSize,
		Parts: []V3PartEntry{
			{
				N:             1,
				PlaintextLen:  int64(len(part1Plaintext)),
				CiphertextLen: 0, // Will be computed
				Blocks:        []V3BlockEntry{},
			},
			{
				N:             2,
				PlaintextLen:  int64(len(part2Plaintext)),
				CiphertextLen: 0, // Will be computed
				Blocks:        []V3BlockEntry{},
			},
		},
	}

	// Encrypt Part 1
	partNum := uint16(1)
	part1Blocks := ComputeBlockCount(int64(len(part1Plaintext)), blockSize)
	var part1Ciphertext []byte

	for blockIdx := uint32(0); blockIdx < part1Blocks; blockIdx++ {
		start := int(blockIdx) * blockSize
		end := start + blockSize
		if end > len(part1Plaintext) {
			end = len(part1Plaintext)
		}

		blockPlaintext := part1Plaintext[start:end]
		blockCiphertext, blockHMAC, err := EncryptBlockV3(dek, iv, partNum, blockIdx, blockPlaintext, blockSize)
		if err != nil { panic(fmt.Sprintf("Failed to encrypt part 1 block %d: %v", blockIdx, err)) }

		part1Ciphertext = append(part1Ciphertext, blockCiphertext...)

		sidecar.Parts[0].Blocks = append(sidecar.Parts[0].Blocks, V3BlockEntry{
			HMAC: base64.StdEncoding.EncodeToString(blockHMAC),
			CLen: uint32(len(blockCiphertext)),
		})
	}
	sidecar.Parts[0].CiphertextLen = int64(len(part1Ciphertext))

	// Encrypt Part 2
	partNum = uint16(2)
	part2Blocks := ComputeBlockCount(int64(len(part2Plaintext)), blockSize)
	var part2Ciphertext []byte

	for blockIdx := uint32(0); blockIdx < part2Blocks; blockIdx++ {
		start := int(blockIdx) * blockSize
		end := start + blockSize
		if end > len(part2Plaintext) {
			end = len(part2Plaintext)
		}

		blockPlaintext := part2Plaintext[start:end]
		blockCiphertext, blockHMAC, err := EncryptBlockV3(dek, iv, partNum, blockIdx, blockPlaintext, blockSize)
		if err != nil { panic(fmt.Sprintf("Failed to encrypt part 2 block %d: %v", blockIdx, err)) }

		part2Ciphertext = append(part2Ciphertext, blockCiphertext...)

		sidecar.Parts[1].Blocks = append(sidecar.Parts[1].Blocks, V3BlockEntry{
			HMAC: base64.StdEncoding.EncodeToString(blockHMAC),
			CLen: uint32(len(blockCiphertext)),
		})
	}
	sidecar.Parts[1].CiphertextLen = int64(len(part2Ciphertext))

	// For the test vector, we'll use Part 1's first block as the main ciphertext/HMAC
	var firstPartHMAC []byte
	if len(sidecar.Parts[0].Blocks) > 0 {
		firstPartHMAC, _ = base64.StdEncoding.DecodeString(sidecar.Parts[0].Blocks[0].HMAC)
	}

	// Create a combined plaintext for the header
	fullPlaintext := append(part1Plaintext, part2Plaintext...)
	plaintextSHA := ComputePlaintextSHA256(fullPlaintext)
	header, err := NewEnvelopeHeaderWithVersion(iv, int64(len(fullPlaintext)), blockSize, plaintextSHA, Version3)
	if err != nil { panic(fmt.Sprintf("Failed to create header: %v", err)) }
	headerBytes, err := header.Encode()
	if err != nil { panic(fmt.Sprintf("Failed to encode header: %v", err)) }

	return V3TestVector{
		Name:        "2-part-multipart",
		Description: "Multipart object (2 parts, different sizes)",
		DEK:         base64.StdEncoding.EncodeToString(dek),
		IV:          base64.StdEncoding.EncodeToString(iv),
		Part:        1, // First part
		BlockSize:   blockSize,
		Plaintext:   base64.StdEncoding.EncodeToString(fullPlaintext),
		Header:      base64.StdEncoding.EncodeToString(headerBytes),
		Ciphertext:  base64.StdEncoding.EncodeToString(part1Ciphertext),
		HMAC:        base64.StdEncoding.EncodeToString(firstPartHMAC),
		Blocks:      sidecar.Parts[0].Blocks,
		Sidecar:     sidecar,
	}
}

// verifyTestVector verifies that a test vector works correctly.
func verifyTestVector(t *testing.T, vec *V3TestVector) {
	// Decode DEK and IV
	dek, err := base64.StdEncoding.DecodeString(vec.DEK)
	require.NoError(t, err, "Failed to decode DEK")
	require.Len(t, dek, 32, "DEK must be 32 bytes")

	iv, err := base64.StdEncoding.DecodeString(vec.IV)
	require.NoError(t, err, "Failed to decode IV")
	require.Len(t, iv, 16, "IV must be 16 bytes")

	// Decode plaintext
	plaintext, err := base64.StdEncoding.DecodeString(vec.Plaintext)
	require.NoError(t, err, "Failed to decode plaintext")

	// Decode expected ciphertext
	expectedCiphertext, err := base64.StdEncoding.DecodeString(vec.Ciphertext)
	require.NoError(t, err, "Failed to decode ciphertext")

	// Verify the vector has blocks and decrypt using block table
	if len(vec.Blocks) > 0 {
		// Build block table from vector
		blockTable := NewBlockTable(vec.BlockSize, len(vec.Blocks))
		for _, blockEntry := range vec.Blocks {
			hmacBytes, err := base64.StdEncoding.DecodeString(blockEntry.HMAC)
			require.NoError(t, err, "Failed to decode block HMAC")
			var hmacArray [32]byte
			copy(hmacArray[:], hmacBytes)

			entry := &BlockTableEntry{
				HMAC:             hmacArray,
				CiphertextLength: blockEntry.CLen,
			}
			err = blockTable.AddEntry(entry)
			require.NoError(t, err, "Failed to add block entry")
		}

		// Reconstruct full ciphertext from block table
		ciphertext := make([]byte, 0, blockTable.TotalCiphertextLength())
		for blockIdx := 0; blockIdx < blockTable.EntryCount(); blockIdx++ {
			start := blockIdx * vec.BlockSize
			end := start + vec.BlockSize
			if end > len(expectedCiphertext) {
				end = len(expectedCiphertext)
			}
			ciphertext = append(ciphertext, expectedCiphertext[start:end]...)
		}

		// Decrypt using v3 semantics
		decryptor, err := NewDecryptorWithVersion(dek, iv, vec.BlockSize, Version3)
		require.NoError(t, err, "Failed to create decryptor")

		decrypted, err := decryptor.DecryptV3(ciphertext, vec.Part, blockTable)
		require.NoError(t, err, "Failed to decrypt v3 blocks")

		// Verify decryption round-trip
		assert.Equal(t, plaintext, decrypted, "Decrypted plaintext should match original")
	} else {
		// Single block verification
		// Decode expected HMAC
		expectedHMAC, err := base64.StdEncoding.DecodeString(vec.HMAC)
		require.NoError(t, err, "Failed to decode HMAC")
		require.Len(t, expectedHMAC, 32, "HMAC must be 32 bytes")

		// Encrypt the plaintext
		ciphertext, hmacValue, err := EncryptBlockV3(dek, iv, vec.Part, 0, plaintext, vec.BlockSize)
		require.NoError(t, err, "Failed to encrypt")

		// Verify ciphertext matches
		assert.Equal(t, expectedCiphertext, ciphertext, "Ciphertext should match expected")

		// Verify HMAC matches
		assert.Equal(t, expectedHMAC, hmacValue, "HMAC should match expected")

		// Decrypt the ciphertext
		decrypted, err := DecryptBlockV3(dek, iv, vec.Part, 0, ciphertext, expectedHMAC, vec.BlockSize)
		require.NoError(t, err, "Failed to decrypt")

		// Verify decryption round-trip
		assert.Equal(t, plaintext, decrypted, "Decrypted plaintext should match original")
	}
}
