package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Encryptor handles AES-256-CTR encryption with per-block HMAC.
type Encryptor struct {
	dek       []byte
	hmacKey   []byte
	iv        []byte
	blockSize int
	version   uint8
	block     cipher.Block
}

// NewEncryptor creates a new encryptor.
// Defaults to Version2 (fixed counter derivation) for security.
// Version1 is only available via build tag for legacy testing.
func NewEncryptor(dek, iv []byte, blockSize int) (*Encryptor, error) {
	return NewEncryptorWithVersion(dek, iv, blockSize, Version2)
}

// NewEncryptorV2 creates a Version2 encryptor with fixed counter derivation.
// Use this for all new objects to prevent keystream reuse.
func NewEncryptorV2(dek, iv []byte, blockSize int) (*Encryptor, error) {
	return NewEncryptorWithVersion(dek, iv, blockSize, Version2)
}

// NewEncryptorWithVersion creates a new encryptor with the specified version.
func NewEncryptorWithVersion(dek, iv []byte, blockSize int, version uint8) (*Encryptor, error) {
	if len(dek) != 32 {
		return nil, fmt.Errorf("DEK must be 32 bytes")
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("IV must be 16 bytes")
	}

	if version != Version1 && version != Version2 && version != Version3 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	// Version3 enforces blockSize <= 1 MiB
	if version == Version3 && blockSize > V3MaxBlockSize {
		return nil, fmt.Errorf("Version3 block size must be <= 1 MiB, got %d", blockSize)
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	return &Encryptor{
		dek:       dek,
		hmacKey:   DeriveHMACKey(dek),
		iv:        iv,
		blockSize: blockSize,
		version:   version,
		block:     block,
	}, nil
}

// Encrypt encrypts plaintext data and returns the encrypted blocks and HMAC table.
func (e *Encryptor) Encrypt(plaintext []byte) (encrypted []byte, hmacTable []byte, err error) {
	blockCount := ComputeBlockCount(int64(len(plaintext)), e.blockSize)

	// Check Version 2 counter space won't overflow
	if e.version == Version2 {
		if err := e.checkCounterSpace(blockCount); err != nil {
			return nil, nil, err
		}
	}

	// Allocate output buffers
	encrypted = make([]byte, blockCount*uint32(e.blockSize))
	hmacTable = make([]byte, blockCount*HMACSize)

	// Encrypt each block with its own counter
	for i := uint32(0); i < blockCount; i++ {
		start := int(i) * e.blockSize
		end := start + e.blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		blockData := plaintext[start:end]
		encryptedBlock := encrypted[start:end]

		// Create CTR stream starting at counter = block index
		ctr := e.makeCounter(i)
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encryptedBlock, blockData)

		// Compute HMAC for this block
		hmacValue := e.computeBlockHMAC(encryptedBlock, i)
		copy(hmacTable[int(i)*HMACSize:], hmacValue)
	}

	// Trim encrypted buffer to actual size
	encrypted = encrypted[:len(plaintext)]

	return encrypted, hmacTable, nil
}

// EncryptStream encrypts plaintext and writes to the provided writer.
// Returns the HMAC table after all data is written.
func (e *Encryptor) EncryptStream(plaintext io.Reader, ciphertext io.Writer, plaintextSize int64) ([]byte, error) {
	blockCount := ComputeBlockCount(plaintextSize, e.blockSize)

	// Check Version 2 counter space won't overflow
	if e.version == Version2 {
		if err := e.checkCounterSpace(blockCount); err != nil {
			return nil, err
		}
	}

	hmacTable := make([]byte, blockCount*HMACSize)

	buf := make([]byte, e.blockSize)
	encryptedBuf := make([]byte, e.blockSize)
	totalWritten := int64(0)

	for blockIndex := uint32(0); blockIndex < blockCount; blockIndex++ {
		// Read a block
		n, err := io.ReadFull(plaintext, buf)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("read error: %w", err)
		}
		if n == 0 {
			break
		}

		// Encrypt the block
		ctr := e.makeCounter(blockIndex)
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encryptedBuf[:n], buf[:n])

		// Compute HMAC
		hmacValue := e.computeBlockHMAC(encryptedBuf[:n], blockIndex)
		copy(hmacTable[int(blockIndex)*HMACSize:], hmacValue)

		// Write encrypted block
		written, err := ciphertext.Write(encryptedBuf[:n])
		if err != nil {
			return nil, fmt.Errorf("write error: %w", err)
		}
		totalWritten += int64(written)
	}

	return hmacTable, nil
}

// EncryptWithStartingCounter encrypts plaintext data with a starting counter offset.
// This is used for multipart uploads where each part continues the CTR stream
// from where the previous part left off.
func (e *Encryptor) EncryptWithStartingCounter(plaintext []byte, startBlockIndex uint32) (encrypted []byte, hmacTable []byte, err error) {
	blockCount := ComputeBlockCount(int64(len(plaintext)), e.blockSize)

	// Check Version 2 counter space won't overflow
	if e.version == Version2 {
		if err := e.checkCounterSpaceWithStart(blockCount, startBlockIndex); err != nil {
			return nil, nil, err
		}
	}

	// Allocate output buffers
	encrypted = make([]byte, blockCount*uint32(e.blockSize))
	hmacTable = make([]byte, blockCount*HMACSize)

	// Encrypt each block with its own counter (starting from startBlockIndex)
	for i := uint32(0); i < blockCount; i++ {
		start := int(i) * e.blockSize
		end := start + e.blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		blockData := plaintext[start:end]
		encryptedBlock := encrypted[start:end]

		// Create CTR stream starting at counter = startBlockIndex + i
		ctr := e.makeCounter(startBlockIndex + i)
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encryptedBlock, blockData)

		// Compute HMAC for this block
		hmacValue := e.computeBlockHMAC(encryptedBlock, startBlockIndex+i)
		copy(hmacTable[int(i)*HMACSize:], hmacValue)
	}

	// Trim encrypted buffer to actual size
	encrypted = encrypted[:len(plaintext)]

	return encrypted, hmacTable, nil
}

// makeCounter creates a 16-byte counter value from the IV and block index.
// Counter = IV[0:12] || uint32(counter_value) in big-endian
//
// Version1 (legacy, vulnerable): counter_value = blockIndex
//   - BUG: Only increments by 1 per block, but each block uses blockSize/16 AES blocks
//   - This causes keystream reuse between adjacent blocks (two-time pad)
//
// Version2 (fixed): counter_value = blockIndex * (blockSize / 16)
//   - Strides by the number of AES blocks per ARMOR block
//   - Ensures no counter reuse across blocks
func (e *Encryptor) makeCounter(blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], e.iv[0:12])

	var counterValue uint32
	if e.version == Version2 {
		// Version2: stride by number of AES blocks per ARMOR block
		// This prevents counter reuse
		aesBlocksPerArmorBlock := uint32(e.blockSize / 16)
		counterValue = blockIndex * aesBlocksPerArmorBlock
	} else {
		// Version1: legacy (buggy) derivation for backward compatibility
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
	return counter
}

// checkCounterSpace validates that the block count won't overflow the Version 2
// counter space. Version 2 stores blockIndex * (blockSize / 16) in a uint32, so
// the maximum block index is 2^32 / (blockSize / 16). At 64 KiB blocks, this is
// 2^20 blocks = 64 GiB.
func (e *Encryptor) checkCounterSpace(blockCount uint32) error {
	const maxCounterValue = 1 << 32
	aesBlocksPerArmorBlock := uint64(e.blockSize / 16)
	finalCounterValue := uint64(blockCount) * aesBlocksPerArmorBlock
	if finalCounterValue >= maxCounterValue {
		return fmt.Errorf("object exceeds the Version 2 counter space; envelope v3 removes this limit")
	}
	return nil
}

// checkCounterSpaceWithStart validates that the final block index (start + count)
// won't overflow the Version 2 counter space. Used for multipart uploads where
// each part continues from a previous part's counter.
func (e *Encryptor) checkCounterSpaceWithStart(blockCount uint32, startBlockIndex uint32) error {
	const maxCounterValue = 1 << 32
	aesBlocksPerArmorBlock := uint64(e.blockSize / 16)
	finalBlockIndex := uint64(startBlockIndex) + uint64(blockCount)
	finalCounterValue := finalBlockIndex * aesBlocksPerArmorBlock
	if finalCounterValue >= maxCounterValue {
		return fmt.Errorf("object exceeds the Version 2 counter space; envelope v3 removes this limit")
	}
	return nil
}

// computeBlockHMAC computes HMAC-SHA256 for an encrypted block.
func (e *Encryptor) computeBlockHMAC(encryptedBlock []byte, blockIndex uint32) []byte {
	mac := hmac.New(sha256.New, e.hmacKey)

	// Include block index in HMAC to prevent block reordering
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, blockIndex)
	mac.Write(indexBytes)

	mac.Write(encryptedBlock)
	return mac.Sum(nil)
}

// BlockSize returns the block size.
func (e *Encryptor) BlockSize() int {
	return e.blockSize
}

// EncryptAndCompress encrypts plaintext with optional compression.
// If compress is true, plaintext is compressed with zstd before encryption.
// Opportunistic pass-through: if compression doesn't shrink the data, original is used.
//
// Returns:
// - encrypted: The encrypted (and optionally compressed) data
// - hmacTable: The HMAC table for integrity verification
// - wasCompressed: true if data was compressed, false otherwise
// - compressionType: The type of compression used (or CompressionNone if not compressed)
// - plaintextSize: The size of the plaintext BEFORE compression (for envelope header)
// - plaintextSHA: SHA-256 of the plaintext BEFORE compression (for envelope header)
// - err: Error if encryption/compression fails
func (e *Encryptor) EncryptAndCompress(plaintext []byte, compress bool) (encrypted []byte, hmacTable []byte, wasCompressed bool, compressionType CompressionType, plaintextSize int64, plaintextSHA [32]byte, err error) {
	// Compute SHA-256 of original plaintext (before compression)
	plaintextSHA = ComputePlaintextSHA256(plaintext)
	plaintextSize = int64(len(plaintext))

	// Compress if requested
	dataToEncrypt := plaintext
	if compress {
		compressedData, compressed, compType, compErr := Compress(plaintext)
		if compErr != nil {
			return nil, nil, false, CompressionNone, 0, plaintextSHA, fmt.Errorf("compression failed: %w", compErr)
		}
		wasCompressed = compressed
		compressionType = compType
		if compressed {
			dataToEncrypt = compressedData
		}
	}

	// Encrypt the (possibly compressed) data
	encrypted, hmacTable, err = e.Encrypt(dataToEncrypt)
	if err != nil {
		return nil, nil, false, CompressionNone, 0, plaintextSHA, fmt.Errorf("encryption failed: %w", err)
	}

	return encrypted, hmacTable, wasCompressed, compressionType, plaintextSize, plaintextSHA, nil
}

// EncryptWithBlockCompression encrypts plaintext with per-block zstd compression.
// Each block is compressed independently before encryption; if compression doesn't
// reduce size, the raw block is used. This allows mixed compressed/uncompressed blocks.
//
// Returns:
// - encrypted: Variable-length encrypted data (blocks may be different sizes)
// - blockTable: Block table with compression flags set appropriately
// - err: Error if encryption/compression fails
func (e *Encryptor) EncryptWithBlockCompression(plaintext []byte) (encrypted []byte, blockTable *BlockTable, err error) {
	blockCount := ComputeBlockCount(int64(len(plaintext)), e.blockSize)

	// Check Version 2 counter space won't overflow
	if e.version == Version2 {
		if err := e.checkCounterSpace(blockCount); err != nil {
			return nil, nil, err
		}
	}

	// Create block table to track compressed blocks
	blockTable = NewBlockTable(e.blockSize, int(blockCount))

	// Pre-allocate encrypted buffer with estimated size (will be trimmed)
	estimatedSize := len(plaintext)
	encrypted = make([]byte, 0, estimatedSize)

	// Encrypt each block independently with compression
	for i := uint32(0); i < blockCount; i++ {
		start := int(i) * e.blockSize
		end := start + e.blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		plaintextBlock := plaintext[start:end]

		// Compress the block with opportunistic pass-through
		compressedBlock, wasCompressed, _, err := CompressBlock(plaintextBlock)
		if err != nil {
			return nil, nil, fmt.Errorf("block %d compression failed: %w", i, err)
		}

		// Determine what to encrypt (compressed or original)
		dataToEncrypt := compressedBlock
		if !wasCompressed {
			dataToEncrypt = plaintextBlock
		}

		// Encrypt the (possibly compressed) block
		encryptedBlock := make([]byte, len(dataToEncrypt))
		ctr := e.makeCounter(i)
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encryptedBlock, dataToEncrypt)

		// Compute HMAC for encrypted block
		hmacValue := e.computeBlockHMAC(encryptedBlock, i)

		// Create block table entry with compression flag
		entry := NewBlockTableEntry(hmacValue, uint32(len(encryptedBlock)), wasCompressed)
		if err := blockTable.AddEntry(entry); err != nil {
			return nil, nil, fmt.Errorf("block %d table entry failed: %w", i, err)
		}

		// Append encrypted block to output
		encrypted = append(encrypted, encryptedBlock...)
	}

	return encrypted, blockTable, nil
}

// EncryptStreamWithCompress encrypts a stream with optional compression.
// NOTE: This function requires the caller to handle SHA-256 computation separately
// for compressed data, as the stream is consumed during compression.
// For most use cases, buffer the data and use EncryptAndCompress instead.
func (e *Encryptor) EncryptStreamWithCompress(plaintext io.Reader, ciphertext io.Writer, compress bool, plaintextSHA [32]byte, plaintextSize int64) (hmacTable []byte, wasCompressed bool, compressionType CompressionType, err error) {
	if !compress {
		// No compression - encrypt directly
		hmacTable, err := e.EncryptStream(plaintext, ciphertext, plaintextSize)
		return hmacTable, false, CompressionNone, err
	}

	// For compressed data, we need to buffer, compress, then encrypt
	// This is a limitation of the streaming approach with compression
	compressedData, compressed, compType, _, compErr := CompressStream(plaintext)
	if compErr != nil {
		return nil, false, CompressionNone, fmt.Errorf("compression failed: %w", compErr)
	}

	wasCompressed = compressed
	compressionType = compType

	// Encrypt the (possibly compressed) data
	encrypted, encHmacTable, encErr := e.Encrypt(compressedData)
	if encErr != nil {
		return nil, false, CompressionNone, fmt.Errorf("encryption failed: %w", encErr)
	}

	// Write to the ciphertext writer
	if _, err := ciphertext.Write(encrypted); err != nil {
		return nil, false, CompressionNone, fmt.Errorf("failed to write encrypted data: %w", err)
	}

	// Write HMAC table
	if _, err := ciphertext.Write(encHmacTable); err != nil {
		return nil, false, CompressionNone, fmt.Errorf("failed to write HMAC table: %w", err)
	}

	return encHmacTable, wasCompressed, compressionType, nil
}

// NewEncryptorWithCounter creates a new encryptor with a specific starting counter.
// This is used for multipart uploads where each part needs to continue the CTR stream
// from where the previous part left off.
// Defaults to Version2 for security.
func NewEncryptorWithCounter(dek, iv []byte, blockSize int, startBlockIndex uint32) (*Encryptor, error) {
	return NewEncryptorWithCounterAndVersion(dek, iv, blockSize, startBlockIndex, Version2)
}

// NewEncryptorWithCounterAndVersion creates a new encryptor with a specific starting counter and version.
func NewEncryptorWithCounterAndVersion(dek, iv []byte, blockSize int, startBlockIndex uint32, version uint8) (*Encryptor, error) {
	enc, err := NewEncryptorWithVersion(dek, iv, blockSize, version)
	if err != nil {
		return nil, err
	}
	// The encryptor itself doesn't store state - each block uses its own counter
	// So we just need the encryptor with the correct IV
	return enc, nil
}

// EncryptV3 encrypts plaintext data using Version3 semantics and produces a trailer block table.
// This is the primary method for single-PUT v3 objects.
//
// Version3 format: header || blocks || trailer_block_table
// - Each block is encrypted with v3 counter construction (part=0 for single-PUT)
// - Trailer block table contains [HMAC (32 bytes), clen (4 bytes)] for each block
// - The high bit of clen indicates compression (currently always false for single-PUT)
//
// Parameters:
//   - plaintext: Plaintext data to encrypt
//   - compress: Whether to compress the data (currently unused pending compress-rules bead)
//
// Returns:
//   - encrypted: The encrypted blocks (concatenated)
//   - blockTable: The trailer block table with HMAC and length for each block
//   - err: Error if encryption fails
func (e *Encryptor) EncryptV3(plaintext []byte, compress bool) (encrypted []byte, blockTable *BlockTable, err error) {
	if e.version != Version3 {
		return nil, nil, fmt.Errorf("EncryptV3 requires Version3 encryptor, got %d", e.version)
	}

	blockCount := ComputeBlockCount(int64(len(plaintext)), e.blockSize)

	// Create block table to track blocks
	blockTable = NewBlockTable(e.blockSize, int(blockCount))

	// Pre-allocate encrypted buffer with exact size (same as plaintext for AES-CTR)
	encrypted = make([]byte, 0, len(plaintext))

	// For single-PUT v3, part number is 0
	part := uint16(0)

	// Encrypt each block independently
	for i := uint32(0); i < blockCount; i++ {
		start := int(i) * e.blockSize
		end := start + e.blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		plaintextBlock := plaintext[start:end]

		// Encrypt the block using v3 counter construction
		encryptedBlock, blockHMAC, err := EncryptBlockV3(e.dek, e.iv, part, i, plaintextBlock, e.blockSize)
		if err != nil {
			return nil, nil, fmt.Errorf("block %d encryption failed: %w", i, err)
		}

		// Create block table entry (compression flag is false for now)
		var hmacArray [32]byte
		copy(hmacArray[:], blockHMAC)
		entry := NewBlockTableEntry(hmacArray, uint32(len(encryptedBlock)), false)
		if err := blockTable.AddEntry(entry); err != nil {
			return nil, nil, fmt.Errorf("block %d table entry failed: %w", i, err)
		}

		// Append encrypted block to output
		encrypted = append(encrypted, encryptedBlock...)
	}

	return encrypted, blockTable, nil
}

// EncryptV3Stream encrypts plaintext from a reader using Version3 semantics and produces a trailer block table.
// This is the streaming version for large single-PUT v3 objects.
//
// Parameters:
//   - plaintext: Reader for plaintext data
//   - ciphertext: Writer for encrypted data
//   - plaintextSize: Total size of plaintext (for block count calculation)
//   - compress: Whether to compress the data (currently unused pending compress-rules bead)
//
// Returns:
//   - blockTable: The trailer block table with HMAC and length for each block
//   - err: Error if encryption fails
func (e *Encryptor) EncryptV3Stream(plaintext io.Reader, ciphertext io.Writer, plaintextSize int64, compress bool) (blockTable *BlockTable, err error) {
	if e.version != Version3 {
		return nil, fmt.Errorf("EncryptV3Stream requires Version3 encryptor, got %d", e.version)
	}

	blockCount := ComputeBlockCount(plaintextSize, e.blockSize)

	// Create block table to track blocks
	blockTable = NewBlockTable(e.blockSize, int(blockCount))

	// For single-PUT v3, part number is 0
	part := uint16(0)

	buf := make([]byte, e.blockSize)
	totalWritten := int64(0)

	for blockIndex := uint32(0); blockIndex < blockCount; blockIndex++ {
		// Read a block
		n, err := io.ReadFull(plaintext, buf)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("read error at block %d: %w", blockIndex, err)
		}
		if n == 0 {
			break
		}

		plaintextBlock := buf[:n]

		// Encrypt the block using v3 counter construction
		encryptedBlock, blockHMAC, err := EncryptBlockV3(e.dek, e.iv, part, blockIndex, plaintextBlock, e.blockSize)
		if err != nil {
			return nil, fmt.Errorf("block %d encryption failed: %w", blockIndex, err)
		}

		// Create block table entry (compression flag is false for now)
		var hmacArray [32]byte
		copy(hmacArray[:], blockHMAC)
		entry := NewBlockTableEntry(hmacArray, uint32(len(encryptedBlock)), false)
		if err := blockTable.AddEntry(entry); err != nil {
			return nil, fmt.Errorf("block %d table entry failed: %w", blockIndex, err)
		}

		// Write encrypted block
		written, err := ciphertext.Write(encryptedBlock)
		if err != nil {
			return nil, fmt.Errorf("write error at block %d: %w", blockIndex, err)
		}
		totalWritten += int64(written)
	}

	return blockTable, nil
}

// EncryptPartV3 encrypts a single part of a multipart upload using Version3 semantics.
//
// Each part is encrypted as an independent stream starting from block 0 within its
// part namespace, using the v3 counter construction: IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock).
//
// This allows parts to be uploaded in any order with no size or alignment constraints.
//
// Parameters:
//   - dek: 32-byte data encryption key
//   - iv: 16-byte initialization vector
//   - partNumber: Part number (1..10000)
//   - blockSize: ARMOR block size (must be <= V3MaxBlockSize)
//   - plaintext: Part plaintext data
//
// Returns:
//   - encrypted: Concatenated encrypted blocks
//   - hmacs: Concatenated HMACs for each block (32 bytes per block)
//   - err: Error if encryption fails
func EncryptPartV3(dek, iv []byte, partNumber uint16, blockSize int, plaintext []byte) (encrypted []byte, hmacs []byte, err error) {
	if len(dek) != 32 {
		return nil, nil, fmt.Errorf("DEK must be 32 bytes")
	}
	if len(iv) != 16 {
		return nil, nil, fmt.Errorf("IV must be 16 bytes")
	}
	if blockSize > V3MaxBlockSize {
		return nil, nil, fmt.Errorf("block size %d exceeds Version3 maximum %d", blockSize, V3MaxBlockSize)
	}

	blockCount := ComputeBlockCount(int64(len(plaintext)), blockSize)

	// Pre-allocate encrypted buffer with exact size (same as plaintext for AES-CTR)
	encrypted = make([]byte, 0, len(plaintext))

	// Pre-allocate HMAC buffer (32 bytes per block)
	hmacs = make([]byte, 0, blockCount*HMACSize)

	// Encrypt each block independently
	for blockIndex := uint32(0); blockIndex < blockCount; blockIndex++ {
		start := int(blockIndex) * blockSize
		end := start + blockSize
		if end > len(plaintext) {
			end = len(plaintext)
		}

		plaintextBlock := plaintext[start:end]

		// Encrypt the block using v3 counter construction
		encryptedBlock, blockHMAC, err := EncryptBlockV3(dek, iv, partNumber, blockIndex, plaintextBlock, blockSize)
		if err != nil {
			return nil, nil, fmt.Errorf("block %d encryption failed: %w", blockIndex, err)
		}

		// Append encrypted block and HMAC
		encrypted = append(encrypted, encryptedBlock...)
		hmacs = append(hmacs, blockHMAC...)
	}

	return encrypted, hmacs, nil
}
