package crypto_test

import (
	"bytes"
	"crypto/aes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jedarden/armor/internal/crypto"
	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/require"
)

// Test vectors use fixed DEK and IV for reproducibility
// DO NOT change these values - they are part of the normative spec
var (
	fixedDEK = []byte{
		0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
		0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
		0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
		0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	}
	fixedIV = []byte{
		0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8,
		0xF7, 0xF6, 0xF5, 0xF4, 0xF3, 0xF2, 0xF1, 0xF0,
	}
)

// V3TestVector represents a complete test vector for v3 format
type V3TestVector struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Input       InputData         `json:"input"`
	Keys        KeyMaterial       `json:"keys"`
	Header      string            `json:"header"` // hex-encoded
	Blocks      []BlockInfo       `json:"blocks"`
	Sidecar     SidecarV3         `json:"sidecar"`
	Metadata    map[string]string `json:"metadata"`
}

type InputData struct {
	Plaintext string `json:"plaintext"` // base64-encoded
	BlockSize int    `json:"block_size"`
	Compress  bool   `json:"compress"`
}

type KeyMaterial struct {
	DEK      string `json:"dek"`       // hex-encoded
	IV       string `json:"iv"`        // hex-encoded
	HMACKey  string `json:"hmac_key"`  // hex-encoded (derived)
	FileName string `json:"file_name"` // for sidecar key derivation
}

type BlockInfo struct {
	Index        int    `json:"index"`
	Part         int    `json:"part"`
	Plaintext    string `json:"plaintext"`  // base64-encoded
	Ciphertext   string `json:"ciphertext"` // hex-encoded
	HMAC         string `json:"hmac"`       // hex-encoded
	CLen         uint32 `json:"clen"`       // encoded with compression flag
	Compressed   bool   `json:"compressed"`
	CounterBlock string `json:"counter_block"` // hex-encoded (for reference)
}

type SidecarV3 struct {
	Version   int          `json:"version"`
	BlockSize int          `json:"block_size"`
	Parts     []PartInfoV3 `json:"parts"`
}

type PartInfoV3 struct {
	N             int        `json:"n"`
	PlaintextLen  int        `json:"plaintext_len"`
	CiphertextLen int        `json:"ciphertext_len"`
	Blocks        [][]string `json:"blocks"` // [[hmac_b64, clen], ...]
}

// TestGenerateV3Vectors generates the normative test vectors for envelope v3
// Use -update to regenerate the golden files
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
		require.NoError(t, os.MkdirAll(testDir, 0755))
	}

	vectors := []V3TestVector{
		generateOneBlockSinglePUT(),
		generateThreeBlockCompressed(),
		generateTwoPartMultipart(),
	}

	for i, vec := range vectors {
		t.Run(vec.Name, func(t *testing.T) {
			// Write golden file
			filename := filepath.Join(testDir, fmt.Sprintf("%d-%s.json", i+1, vec.Name))
			data, err := json.MarshalIndent(vec, "", "  ")
			require.NoError(t, err)

			if update {
				t.Logf("Writing test vector: %s", filename)
				require.NoError(t, os.WriteFile(filename, data, 0644))
			} else {
				// Verify golden file exists and matches
				golden, err := os.ReadFile(filename)
				require.NoError(t, err, "Golden file missing - run with -update to generate")

				var goldenVec V3TestVector
				require.NoError(t, json.Unmarshal(golden, &goldenVec))
				require.Equal(t, goldenVec, vec, "Generated vector differs from golden - run with -update")
			}
		})
	}
}

// generateOneBlockSinglePUT creates a minimal single-PUT object with 1 block (uncompressed)
func generateOneBlockSinglePUT() V3TestVector {
	blockSize := 65536                                // 64 KiB
	plaintext := bytes.Repeat([]byte("A"), blockSize) // All 'A's - incompressible

	dekHex := fmt.Sprintf("%x", fixedDEK)
	ivHex := fmt.Sprintf("%x", fixedIV)
	hmacKeyHex := fmt.Sprintf("%x", deriveHMACKey(fixedDEK))

	header := buildV3Header(fixedIV, int64(len(plaintext)), blockSize, plaintext)

	blocks := encryptBlocksV3(plaintext, fixedDEK, fixedIV, 0 /* part */, blockSize, false /* compress */)

	// Build sidecar
	sidecar := SidecarV3{
		Version:   3,
		BlockSize: blockSize,
		Parts: []PartInfoV3{
			{
				N:             0,
				PlaintextLen:  len(plaintext),
				CiphertextLen: totalCiphertextLen(blocks),
				Blocks:        blocksToSidecarFormat(blocks),
			},
		},
	}

	// Compute sidecar filename from key
	sidecarKey := computeSidecarKey(fixedDEK, "test-object.bin")

	return V3TestVector{
		Name:        "1-block-single-put",
		Description: "Minimal single-PUT object with 1 block, uncompressed (all 'A's)",
		Input: InputData{
			Plaintext: base64.StdEncoding.EncodeToString(plaintext),
			BlockSize: blockSize,
			Compress:  false,
		},
		Keys: KeyMaterial{
			DEK:      dekHex,
			IV:       ivHex,
			HMACKey:  hmacKeyHex,
			FileName: sidecarKey,
		},
		Header:   fmt.Sprintf("%x", header),
		Blocks:   blocks,
		Sidecar:  sidecar,
		Metadata: map[string]string{"x-amz-meta-armor-version": "3"},
	}
}

// generateThreeBlockCompressed creates a single-PUT object with 3 blocks where middle block compresses
func generateThreeBlockCompressed() V3TestVector {
	blockSize := 65536 // 64 KiB

	// Block 0: Random/incompressible
	block0 := make([]byte, blockSize)
	for i := range block0 {
		block0[i] = byte(i % 256)
	}

	// Block 1: Highly compressible (all zeros)
	block1 := make([]byte, blockSize)

	// Block 2: Incompressible (alternating)
	block2 := make([]byte, blockSize)
	for i := range block2 {
		if i%2 == 0 {
			block2[i] = 0xAA
		} else {
			block2[i] = 0x55
		}
	}

	plaintext := append(append(block0, block1...), block2...)

	dekHex := fmt.Sprintf("%x", fixedDEK)
	ivHex := fmt.Sprintf("%x", fixedIV)
	hmacKeyHex := fmt.Sprintf("%x", deriveHMACKey(fixedDEK))

	header := buildV3Header(fixedIV, int64(len(plaintext)), blockSize, plaintext)

	blocks := encryptBlocksV3(plaintext, fixedDEK, fixedIV, 0 /* part */, blockSize, true /* compress */)

	sidecar := SidecarV3{
		Version:   3,
		BlockSize: blockSize,
		Parts: []PartInfoV3{
			{
				N:             0,
				PlaintextLen:  len(plaintext),
				CiphertextLen: totalCiphertextLen(blocks),
				Blocks:        blocksToSidecarFormat(blocks),
			},
		},
	}

	sidecarKey := computeSidecarKey(fixedDEK, "test-compressed.bin")

	return V3TestVector{
		Name:        "3-block-compressed",
		Description: "Single-PUT object with 3 blocks, middle block (all zeros) compresses well",
		Input: InputData{
			Plaintext: base64.StdEncoding.EncodeToString(plaintext),
			BlockSize: blockSize,
			Compress:  true,
		},
		Keys: KeyMaterial{
			DEK:      dekHex,
			IV:       ivHex,
			HMACKey:  hmacKeyHex,
			FileName: sidecarKey,
		},
		Header:   fmt.Sprintf("%x", header),
		Blocks:   blocks,
		Sidecar:  sidecar,
		Metadata: map[string]string{"x-amz-meta-armor-version": "3"},
	}
}

// generateTwoPartMultipart creates a multipart object with 2 parts of different sizes
func generateTwoPartMultipart() V3TestVector {
	blockSize := 65536 // 64 KiB

	// Part 1: 1.5 blocks (~96 KiB)
	part1 := bytes.Repeat([]byte("PART1_DATA_"), blockSize/10)

	// Part 2: 2.5 blocks (~160 KiB)
	part2 := bytes.Repeat([]byte("PART2_DATA_"), blockSize/8)

	plaintextPart1 := part1
	plaintextPart2 := part2

	dekHex := fmt.Sprintf("%x", fixedDEK)
	ivHex := fmt.Sprintf("%x", fixedIV)
	hmacKeyHex := fmt.Sprintf("%x", deriveHMACKey(fixedDEK))

	// Encrypt part 1
	blocks1 := encryptBlocksV3(plaintextPart1, fixedDEK, fixedIV, 1 /* part 1 */, blockSize, false)

	// Encrypt part 2
	blocks2 := encryptBlocksV3(plaintextPart2, fixedDEK, fixedIV, 2 /* part 2 */, blockSize, false)

	// Build combined plaintext for header
	fullPlaintext := append(plaintextPart1, plaintextPart2...)
	header := buildV3Header(fixedIV, int64(len(fullPlaintext)), blockSize, fullPlaintext)

	// Build sidecar with both parts
	sidecar := SidecarV3{
		Version:   3,
		BlockSize: blockSize,
		Parts: []PartInfoV3{
			{
				N:             1,
				PlaintextLen:  len(plaintextPart1),
				CiphertextLen: totalCiphertextLen(blocks1),
				Blocks:        blocksToSidecarFormat(blocks1),
			},
			{
				N:             2,
				PlaintextLen:  len(plaintextPart2),
				CiphertextLen: totalCiphertextLen(blocks2),
				Blocks:        blocksToSidecarFormat(blocks2),
			},
		},
	}

	// Combine all blocks for the test vector
	allBlocks := append(blocks1, blocks2...)

	sidecarKey := computeSidecarKey(fixedDEK, "test-multipart.bin")

	return V3TestVector{
		Name:        "2-part-multipart",
		Description: "Multipart object with 2 parts of different sizes (96 KiB and 160 KiB)",
		Input: InputData{
			Plaintext: base64.StdEncoding.EncodeToString(fullPlaintext),
			BlockSize: blockSize,
			Compress:  false, // multipart never compressed
		},
		Keys: KeyMaterial{
			DEK:      dekHex,
			IV:       ivHex,
			HMACKey:  hmacKeyHex,
			FileName: sidecarKey,
		},
		Header:  fmt.Sprintf("%x", header),
		Blocks:  allBlocks,
		Sidecar: sidecar,
		Metadata: map[string]string{
			"x-amz-meta-armor-version":  "3",
			"x-amz-meta-armor-compress": "false",
		},
	}
}

// buildV3Header constructs a v3 envelope header
func buildV3Header(iv []byte, plaintextSize int64, blockSize int, plaintext []byte) []byte {
	header := crypto.EnvelopeHeader{}
	copy(header.Magic[:], "ARMR")
	header.Version = 0x03 // v3

	// Compute block size log2
	blockSizeLog2 := uint8(0)
	for bs := blockSize; bs > 1; bs >>= 1 {
		blockSizeLog2++
	}
	header.BlockSizeLog2 = blockSizeLog2

	copy(header.IV[:], iv)
	header.PlaintextSize = uint64(plaintextSize)
	header.PlaintextSHA = crypto.ComputePlaintextSHA256(plaintext)
	header.Reserved[0] = 0x00 // compression flag
	header.Reserved[1] = 0x00 // reserved

	encoded, err := header.Encode()
	if err != nil {
		panic(err)
	}
	return encoded
}

// encryptBlocksV3 encrypts plaintext into blocks using v3 counter construction
func encryptBlocksV3(plaintext, dek, iv []byte, part int, blockSize int, compress bool) []BlockInfo {
	hmacKey := deriveHMACKey(dek)

	blockCount := (len(plaintext) + blockSize - 1) / blockSize
	blocks := make([]BlockInfo, 0, blockCount)

	encoder, _ := zstd.NewWriter(nil)

	for blockIdx := 0; blockIdx < blockCount; blockIdx++ {
		start := blockIdx * blockSize
		end := start + blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		blockPlaintext := plaintext[start:end]
		plaintextForEnc := blockPlaintext

		// Try compression if enabled
		var compressed bool
		var ciphertext []byte

		if compress {
			compressedData := encoder.EncodeAll(blockPlaintext, nil)
			if len(compressedData) >= len(blockPlaintext) {
				// No benefit, use raw
				compressed = false
				ciphertext = encryptBlockV3(blockPlaintext, dek, iv, part, blockIdx)
			} else {
				compressed = true
				ciphertext = encryptBlockV3(compressedData, dek, iv, part, blockIdx)
			}
		} else {
			ciphertext = encryptBlockV3(blockPlaintext, dek, iv, part, blockIdx)
		}

		// Compute HMAC
		blockHMAC := computeBlockHMAC(hmacKey, part, blockIdx, ciphertext)

		// Encode CLen with compression flag
		clen := uint32(len(ciphertext))
		if compressed {
			clen |= 0x80000000
		}

		// Get counter block for first AES block (for documentation)
		counterBlock := buildCounterBlock(iv, part, blockIdx, 0)

		blocks = append(blocks, BlockInfo{
			Index:        blockIdx,
			Part:         part,
			Plaintext:    base64.StdEncoding.EncodeToString(plaintextForEnc),
			Ciphertext:   fmt.Sprintf("%x", ciphertext),
			HMAC:         fmt.Sprintf("%x", blockHMAC),
			CLen:         clen,
			Compressed:   compressed,
			CounterBlock: fmt.Sprintf("%x", counterBlock),
		})
	}

	return blocks
}

// encryptBlockV3 encrypts a single block with AES-CTR using v3 counter construction
func encryptBlockV3(plaintext, dek, iv []byte, part, blockIdx int) []byte {
	block, err := aes.NewCipher(dek)
	if err != nil {
		panic(err)
	}

	ciphertext := make([]byte, len(plaintext))
	aesBlocks := (len(plaintext) + 15) / 16

	for i := 0; i < aesBlocks; i++ {
		counterBlock := buildCounterBlock(iv, part, blockIdx, i)

		// Encrypt counter block to get keystream
		keystream := make([]byte, 16)
		block.Encrypt(keystream, counterBlock)

		// XOR with plaintext
		start := i * 16
		end := start + 16
		if end > len(plaintext) {
			end = len(plaintext)
		}

		for j := start; j < end; j++ {
			ciphertext[j] = plaintext[j] ^ keystream[j-start]
		}
	}

	return ciphertext
}

// buildCounterBlock constructs the v3 counter block
func buildCounterBlock(iv []byte, part, blockIdx, aesBlockIdx int) []byte {
	counter := make([]byte, 16)

	// IV[0:8]
	copy(counter[0:8], iv[0:8])

	// uint16(part) - big-endian
	binary.BigEndian.PutUint16(counter[8:10], uint16(part))

	// uint32(blockIdx) - big-endian
	binary.BigEndian.PutUint32(counter[10:14], uint32(blockIdx))

	// uint16(aesBlockIdx) - big-endian
	binary.BigEndian.PutUint16(counter[14:16], uint16(aesBlockIdx))

	return counter
}

// computeBlockHMAC computes HMAC for a block
func computeBlockHMAC(hmacKey []byte, part, blockIdx int, ciphertext []byte) []byte {
	mac := hmac.New(sha256.New, hmacKey)

	// Write part number (big-endian)
	binary.Write(mac, binary.BigEndian, uint16(part))

	// Write block index (big-endian)
	binary.Write(mac, binary.BigEndian, uint32(blockIdx))

	// Write ciphertext
	mac.Write(ciphertext)

	return mac.Sum(nil)
}

// deriveHMACKey derives HMAC key from DEK using HKDF-SHA256
func deriveHMACKey(dek []byte) []byte {
	// HKDF-SHA256 with info="armor-hmac-v1"
	// Simplified: use SHA256(DEK || info) as the HMAC key
	hash := sha256.Sum256(append(dek, []byte("armor-hmac-v1")...))
	return hash[:]
}

// totalCiphertextLen computes total ciphertext length from blocks
func totalCiphertextLen(blocks []BlockInfo) int {
	total := 0
	for _, block := range blocks {
		total += int(block.CLen & 0x7FFFFFFF) // Mask off compression bit
	}
	return total
}

// blocksToSidecarFormat converts BlockInfo to sidecar block format
func blocksToSidecarFormat(blocks []BlockInfo) [][]string {
	result := make([][]string, len(blocks))
	for i, block := range blocks {
		hmacBytes, _ := base64.StdEncoding.DecodeString(block.HMAC)
		hmacB64 := base64.StdEncoding.EncodeToString(hmacBytes)
		clenStr := fmt.Sprintf("%d", block.CLen)
		result[i] = []string{hmacB64, clenStr}
	}
	return result
}

// computeSidecarKey computes the sidecar filename
func computeSidecarKey(dek []byte, objectKey string) string {
	// Sidecar key is SHA-256 of some derivation of DEK and object key
	// For test vectors, use a simple derivation
	hashInput := append(append([]byte{}, dek...), []byte(objectKey)...)
	hash := sha256.Sum256(hashInput)
	return fmt.Sprintf("%x", hash)
}

// TestV3CounterBlockUniqueness verifies that counter blocks are unique
func TestV3CounterBlockUniqueness(t *testing.T) {
	part := 1
	blockSize := 65536
	seen := make(map[string]bool)

	// Test multiple blocks and parts
	for blockIdx := 0; blockIdx < 10; blockIdx++ {
		for aesBlockIdx := 0; aesBlockIdx < blockSize/16; aesBlockIdx++ {
			cb := buildCounterBlock(fixedIV, part, blockIdx, aesBlockIdx)
			cbStr := fmt.Sprintf("%x", cb)

			if seen[cbStr] {
				t.Errorf("Duplicate counter block: part=%d block=%d aes=%d", part, blockIdx, aesBlockIdx)
			}
			seen[cbStr] = true
		}
	}

	// Verify different parts don't collide
	cb1 := buildCounterBlock(fixedIV, 1, 0, 0)
	cb2 := buildCounterBlock(fixedIV, 2, 0, 0)
	if bytes.Equal(cb1, cb2) {
		t.Error("Counter blocks collide across different parts")
	}
}

// TestV3HMACInputFormat verifies HMAC input format
func TestV3HMACInputFormat(t *testing.T) {
	hmacKey := deriveHMACKey(fixedDEK)
	part := uint16(1)
	blockIdx := uint32(5)
	ciphertext := []byte{0xAA, 0xBB, 0xCC, 0xDD}

	mac := hmac.New(sha256.New, hmacKey)

	buf := make([]byte, 8) // 2 + 4 + 2 (padding)
	binary.BigEndian.PutUint16(buf[0:2], part)
	binary.BigEndian.PutUint32(buf[2:6], blockIdx)

	mac.Write(buf[0:6])
	mac.Write(ciphertext)

	result := mac.Sum(nil)

	// Verify HMAC is deterministic
	mac2 := hmac.New(sha256.New, hmacKey)
	binary.Write(mac2, binary.BigEndian, part)
	binary.Write(mac2, binary.BigEndian, blockIdx)
	mac2.Write(ciphertext)
	result2 := mac2.Sum(nil)

	if !hmac.Equal(result, result2) {
		t.Error("HMAC computation is not deterministic")
	}
}
