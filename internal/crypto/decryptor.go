package crypto

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

// Decryptor handles AES-256-CTR decryption with per-block HMAC verification.
type Decryptor struct {
	dek       []byte
	hmacKey   []byte
	iv        []byte
	blockSize int
	version   uint8
	block     cipher.Block
}

// NewDecryptor creates a new decryptor with Version2 (fixed counter derivation) for security.
// Version1 is only available via NewDecryptorWithVersion for legacy data.
func NewDecryptor(dek, iv []byte, blockSize int) (*Decryptor, error) {
	return NewDecryptorWithVersion(dek, iv, blockSize, Version2)
}

// NewDecryptorWithVersion creates a new decryptor with the specified version.
// The version must match the version used during encryption.
func NewDecryptorWithVersion(dek, iv []byte, blockSize int, version uint8) (*Decryptor, error) {
	if len(dek) != 32 {
		return nil, fmt.Errorf("DEK must be 32 bytes")
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("IV must be 16 bytes")
	}

	if version != Version1 && version != Version2 && version != Version3 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	hmacKey, err := DeriveHMACKey(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to derive HMAC key: %w", err)
	}

	return &Decryptor{
		dek:       dek,
		hmacKey:   hmacKey,
		iv:        iv,
		blockSize: blockSize,
		version:   version,
		block:     block,
	}, nil
}

// Decrypt decrypts encrypted data after verifying HMACs.
func (d *Decryptor) Decrypt(encrypted []byte, hmacTable []byte) ([]byte, error) {
	blockCount := len(encrypted) / d.blockSize
	if len(encrypted)%d.blockSize != 0 {
		blockCount++
	}

	// Verify HMAC table size
	if len(hmacTable) < blockCount*HMACSize {
		return nil, fmt.Errorf("HMAC table too short: got %d, need %d", len(hmacTable), blockCount*HMACSize)
	}

	plaintext := make([]byte, len(encrypted))

	for i := 0; i < blockCount; i++ {
		start := i * d.blockSize
		end := start + d.blockSize
		if end > len(encrypted) {
			end = len(encrypted)
		}

		encryptedBlock := encrypted[start:end]

		// Verify HMAC first
		expectedHMAC := hmacTable[i*HMACSize : (i+1)*HMACSize]
		if err := d.verifyBlockHMAC(encryptedBlock, uint32(i), expectedHMAC); err != nil {
			return nil, fmt.Errorf("block %d: %w", i, err)
		}

		// Decrypt the block
		ctr := d.makeCounter(uint32(i))
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(plaintext[start:end], encryptedBlock)
	}

	return plaintext, nil
}

// DecryptRange decrypts a specific range of blocks after verifying HMACs.
// plaintextStart and plaintextEnd are byte offsets in the plaintext.
// The encrypted slice contains only the blocks needed for the range, starting from blockStart.
// The hmacTable contains HMAC entries for blocks blockStart to blockEnd (if !hmacTableIsFull)
// or for all blocks in the object (if hmacTableIsFull, as with multipart sidecar).
func (d *Decryptor) DecryptRange(encrypted []byte, hmacTable []byte, plaintextStart, plaintextEnd int64, totalPlaintextSize int64, hmacTableIsFull bool) ([]byte, error) {
	blockStart := int(plaintextStart / int64(d.blockSize))
	blockEnd := int(plaintextEnd / int64(d.blockSize))

	// Clamp blockEnd to valid range
	maxBlocks := ComputeBlockCount(totalPlaintextSize, d.blockSize)
	if blockEnd >= int(maxBlocks) {
		blockEnd = int(maxBlocks) - 1
	}

	// Number of blocks in the encrypted slice
	numBlocks := blockEnd - blockStart + 1

	// Verify HMACs for all blocks in range
	for relIdx := 0; relIdx < numBlocks; relIdx++ {
		absBlockIdx := blockStart + relIdx

		encStart := relIdx * d.blockSize
		encEnd := encStart + d.blockSize
		if encEnd > len(encrypted) {
			encEnd = len(encrypted)
		}

		encryptedBlock := encrypted[encStart:encEnd]

		// For full HMAC table (multipart sidecar), use absolute block index.
		// For partial HMAC table (single-PUT range), use relative index.
		var hmacOffset int
		if hmacTableIsFull {
			hmacOffset = absBlockIdx * HMACSize
		} else {
			hmacOffset = relIdx * HMACSize
		}
		if hmacOffset+HMACSize > len(hmacTable) {
			return nil, fmt.Errorf("HMAC table too short for block %d", absBlockIdx)
		}
		expectedHMAC := hmacTable[hmacOffset : hmacOffset+HMACSize]

		if err := d.verifyBlockHMAC(encryptedBlock, uint32(absBlockIdx), expectedHMAC); err != nil {
			return nil, fmt.Errorf("block %d: %w", absBlockIdx, err)
		}
	}

	// Decrypt the blocks
	plaintext := make([]byte, plaintextEnd-plaintextStart+1)
	outputOffset := 0

	for relIdx := 0; relIdx < numBlocks; relIdx++ {
		absBlockIdx := blockStart + relIdx

		encStart := relIdx * d.blockSize
		encEnd := encStart + d.blockSize
		if encEnd > len(encrypted) {
			encEnd = len(encrypted)
		}

		encryptedBlock := encrypted[encStart:encEnd]

		// Decrypt block
		decryptedBlock := make([]byte, len(encryptedBlock))
		ctr := d.makeCounter(uint32(absBlockIdx))
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(decryptedBlock, encryptedBlock)

		// Calculate which portion of this block we need
		blockPlaintextStart := int64(absBlockIdx * d.blockSize)
		blockPlaintextEnd := blockPlaintextStart + int64(len(decryptedBlock))

		// Find overlap with requested range
		rangeStart := max(plaintextStart, blockPlaintextStart)
		rangeEnd := min(plaintextEnd+1, blockPlaintextEnd)

		if rangeStart < rangeEnd {
			// Copy the relevant portion
			srcOffset := rangeStart - blockPlaintextStart
			copyLen := rangeEnd - rangeStart
			copy(plaintext[outputOffset:outputOffset+int(copyLen)], decryptedBlock[srcOffset:srcOffset+copyLen])
			outputOffset += int(copyLen)
		}
	}

	return plaintext, nil
}

// DecryptStream decrypts data as it streams from the reader.
// The hmacTable must be provided (fetched separately).
// Returns the plaintext size actually decrypted.
func (d *Decryptor) DecryptStream(ciphertext io.Reader, plaintext io.Writer, hmacTable []byte, totalBlocks int) error {
	encryptedBuf := make([]byte, d.blockSize)
	decryptedBuf := make([]byte, d.blockSize)

	for blockIndex := 0; blockIndex < totalBlocks; blockIndex++ {
		// Read encrypted block
		n, err := io.ReadFull(ciphertext, encryptedBuf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read error at block %d: %w", blockIndex, err)
		}
		if n == 0 {
			break
		}

		// Verify HMAC
		hmacOffset := blockIndex * HMACSize
		if hmacOffset+HMACSize > len(hmacTable) {
			return fmt.Errorf("HMAC table too short at block %d", blockIndex)
		}
		expectedHMAC := hmacTable[hmacOffset : hmacOffset+HMACSize]
		if err := d.verifyBlockHMAC(encryptedBuf[:n], uint32(blockIndex), expectedHMAC); err != nil {
			return fmt.Errorf("block %d: %w", blockIndex, err)
		}

		// Decrypt
		ctr := d.makeCounter(uint32(blockIndex))
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(decryptedBuf[:n], encryptedBuf[:n])

		// Write plaintext
		if _, err := plaintext.Write(decryptedBuf[:n]); err != nil {
			return fmt.Errorf("write error at block %d: %w", blockIndex, err)
		}
	}

	return nil
}

// VerifyHMACs verifies all HMACs in the table without decrypting.
func (d *Decryptor) VerifyHMACs(encrypted []byte, hmacTable []byte) error {
	blockCount := len(encrypted) / d.blockSize
	if len(encrypted)%d.blockSize != 0 {
		blockCount++
	}

	if len(hmacTable) < blockCount*HMACSize {
		return fmt.Errorf("HMAC table too short")
	}

	for i := 0; i < blockCount; i++ {
		start := i * d.blockSize
		end := start + d.blockSize
		if end > len(encrypted) {
			end = len(encrypted)
		}

		encryptedBlock := encrypted[start:end]
		expectedHMAC := hmacTable[i*HMACSize : (i+1)*HMACSize]

		if err := d.verifyBlockHMAC(encryptedBlock, uint32(i), expectedHMAC); err != nil {
			return fmt.Errorf("block %d: %w", i, err)
		}
	}

	return nil
}

// makeCounter creates a 16-byte counter value from the IV and block index.
// Counter = IV[0:12] || uint32(counter_value) in big-endian
//
// Version1 (legacy, vulnerable): counter_value = blockIndex
// Version2 (fixed): counter_value = blockIndex * (blockSize / 16)
//
// The version must match the encryption version.
func (d *Decryptor) makeCounter(blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], d.iv[0:12])

	var counterValue uint32
	if d.version == Version2 {
		// Version2: stride by number of AES blocks per ARMOR block
		aesBlocksPerArmorBlock := uint32(d.blockSize / 16)
		counterValue = blockIndex * aesBlocksPerArmorBlock
	} else {
		// Version1: legacy (buggy) derivation for backward compatibility
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
	return counter
}

// verifyBlockHMAC verifies the HMAC for a single encrypted block.
func (d *Decryptor) verifyBlockHMAC(encryptedBlock []byte, blockIndex uint32, expected []byte) error {
	mac := hmac.New(sha256.New, d.hmacKey)

	// Include block index in HMAC
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, blockIndex)
	mac.Write(indexBytes)

	mac.Write(encryptedBlock)
	computed := mac.Sum(nil)

	if !hmac.Equal(computed, expected) {
		return ErrHMACMismatch
	}

	return nil
}

// BlockSize returns the block size.
func (d *Decryptor) BlockSize() int {
	return d.blockSize
}

// DecryptWithBlockDecompression decrypts encrypted data with per-block decompression.
// Uses the block table to determine which blocks are compressed and decompresses them
// after decryption. Handles mixed compressed/uncompressed blocks in the same object.
//
// Returns:
// - plaintext: The decompressed plaintext data
// - err: Error if decryption/decompression fails
func (d *Decryptor) DecryptWithBlockDecompression(encrypted []byte, blockTable *BlockTable) ([]byte, error) {
	blockCount := blockTable.EntryCount()
	plaintext := make([]byte, 0)
	encryptedOffset := 0

	for i := 0; i < blockCount; i++ {
		entry := blockTable.Entries[i]
		blockLength := entry.RawLength()

		// Validate block boundaries
		if encryptedOffset+int(blockLength) > len(encrypted) {
			return nil, fmt.Errorf("block %d exceeds encrypted data bounds", i)
		}

		encryptedBlock := encrypted[encryptedOffset : encryptedOffset+int(blockLength)]

		// Verify HMAC first
		if err := d.verifyBlockHMAC(encryptedBlock, uint32(i), entry.HMAC[:]); err != nil {
			return nil, fmt.Errorf("block %d HMAC verification failed: %w", i, err)
		}

		// Decrypt the block
		ctr := d.makeCounter(uint32(i))
		stream := cipher.NewCTR(d.block, ctr)
		decryptedBlock := make([]byte, len(encryptedBlock))
		stream.XORKeyStream(decryptedBlock, encryptedBlock)

		// Decompress if the compression flag is set
		var plaintextBlock []byte
		if entry.IsCompressed() {
			var err error
			plaintextBlock, err = DecompressBlock(decryptedBlock, true)
			if err != nil {
				return nil, fmt.Errorf("block %d decompression failed: %w", i, err)
			}
		} else {
			plaintextBlock = decryptedBlock
		}

		// Append to plaintext
		plaintext = append(plaintext, plaintextBlock...)
		encryptedOffset += int(blockLength)
	}

	return plaintext, nil
}

// HMACKey returns the HMAC key derived from the DEK.
func (d *Decryptor) HMACKey() []byte {
	return d.hmacKey
}

// CipherBlock returns the underlying AES cipher block.
func (d *Decryptor) CipherBlock() cipher.Block {
	return d.block
}

// IV returns the initialization vector.
func (d *Decryptor) IV() []byte {
	return d.iv
}

// DetectCompressionType detects the compression type from the data magic bytes.
// Returns "zstd", "gzip", "zlib", or "" (uncompressed).
func DetectCompressionType(data []byte) string {
	// Handle nil or empty data
	if len(data) < 2 {
		return ""
	}

	// Check for gzip magic bytes: 0x1F 0x8B
	if len(data) >= 2 && data[0] == 0x1F && data[1] == 0x8B {
		return "gzip"
	}

	// Check for zlib magic bytes: 0x78 (and second byte 0x01, 0x5E, 0x9C, 0xDA)
	if len(data) >= 2 && data[0] == 0x78 {
		second := data[1]
		if second == 0x01 || second == 0x5E || second == 0x9C || second == 0xDA {
			return "zlib"
		}
	}

	// Check for zstd magic bytes: 0x28 0xB5 0x2F 0xFD
	if len(data) >= 4 && data[0] == 0x28 && data[1] == 0xB5 &&
		data[2] == 0x2F && data[3] == 0xFD {
		return "zstd"
	}

	return ""
}

// IsCompressed detects if the decrypted plaintext is compressed.
// Checks for zstd, gzip, and zlib magic bytes.
// Returns true if the data appears to be compressed.
func IsCompressed(plaintext []byte) bool {
	return DetectCompressionType(plaintext) != ""
}

// Decompress decompresses zstd-compressed data.
// Returns the decompressed data or an error if decompression fails.
// If the data is not compressed (no zstd magic), returns the data unchanged.
func Decompress(compressed []byte) ([]byte, error) {
	// Handle nil data
	if compressed == nil {
		return nil, &DecompressionError{
			Err:        fmt.Errorf("nil data provided"),
			ErrType:    ErrTypeClient,
			Cause:      "nil_data",
		}
	}

	// Handle empty data or data too short to be compressed
	if len(compressed) < 4 {
		return compressed, nil
	}

	// Check for zstd magic bytes before attempting decompression
	if !IsCompressed(compressed) {
		return compressed, nil
	}

	decoder, err := zstd.NewReader(nil)
	if err != nil {
		return nil, &DecompressionError{
			Err:        fmt.Errorf("failed to create zstd decoder: %w", err),
			ErrType:    ErrTypeServer,
			Cause:      "decoder_init_failed",
		}
	}
	defer decoder.Close()

	decompressed, err := decoder.DecodeAll(compressed, nil)
	if err != nil {
		// Classify the error based on its content
		errType := classifyDecompressionError(err)
		return nil, &DecompressionError{
			Err:        err,
			ErrType:    errType,
			Cause:      determineErrorCause(err),
		}
	}

	return decompressed, nil
}

// DecompressGzip decompresses gzip-compressed data.
// Returns the decompressed data or an error if decompression fails.
func DecompressGzip(compressed []byte) ([]byte, error) {
	// Handle nil data
	if compressed == nil {
		return nil, &DecompressionError{
			Err:        fmt.Errorf("nil data provided"),
			ErrType:    ErrTypeClient,
			Cause:      "nil_data",
		}
	}

	// Handle empty data or data too short to be compressed
	if len(compressed) < 2 {
		return compressed, nil
	}

	// Check for gzip magic bytes
	if compressed[0] != 0x1F || compressed[1] != 0x8B {
		return compressed, nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, &DecompressionError{
			Err:        fmt.Errorf("failed to create gzip reader: %w", err),
			ErrType:    classifyDecompressionError(err),
			Cause:      "decoder_init_failed",
		}
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, &DecompressionError{
			Err:        err,
			ErrType:    classifyDecompressionError(err),
			Cause:      determineErrorCause(err),
		}
	}

	return decompressed, nil
}

// DecompressZlib decompresses zlib-compressed data.
// Returns the decompressed data or an error if decompression fails.
func DecompressZlib(compressed []byte) ([]byte, error) {
	// Handle nil data
	if compressed == nil {
		return nil, &DecompressionError{
			Err:        fmt.Errorf("nil data provided"),
			ErrType:    ErrTypeClient,
			Cause:      "nil_data",
		}
	}

	// Handle empty data or data too short to be compressed
	if len(compressed) < 2 {
		return compressed, nil
	}

	// Check for zlib magic bytes: 0x78 followed by valid second byte
	if compressed[0] != 0x78 {
		return compressed, nil
	}
	second := compressed[1]
	if second != 0x01 && second != 0x5E && second != 0x9C && second != 0xDA {
		return compressed, nil
	}

	reader := flate.NewReader(bytes.NewReader(compressed))
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, &DecompressionError{
			Err:        err,
			ErrType:    classifyDecompressionError(err),
			Cause:      determineErrorCause(err),
		}
	}

	return decompressed, nil
}

// DecompressionError wraps decompression errors with classification.
type DecompressionError struct {
	Err     error
	ErrType ErrorType
	Cause   string
}

func (e *DecompressionError) Error() string {
	return e.Err.Error()
}

func (e *DecompressionError) Unwrap() error {
	return e.Err
}

// ErrorType classifies decompression errors.
type ErrorType int

const (
	// ErrTypeClient indicates client-side data integrity issues (400 Bad Request)
	ErrTypeClient ErrorType = iota
	// ErrTypeServer indicates server-side infrastructure issues (500 Internal Server Error)
	ErrTypeServer
)

// classifyDecompressionError determines if an error is client-side (data integrity) or server-side (infrastructure).
func classifyDecompressionError(err error) ErrorType {
	if err == nil {
		return ErrTypeServer
	}

	errMsg := err.Error()

	// Client-side errors: data integrity issues in stored data
	//
	// Truncated/incomplete data
	if strings.Contains(errMsg, "unexpected EOF") || strings.Contains(errMsg, "EOF") ||
		strings.Contains(errMsg, "truncated") || strings.Contains(errMsg, "incomplete") {
		return ErrTypeClient
	}

	// Invalid format / corrupt data
	if strings.Contains(errMsg, "magic") || strings.Contains(errMsg, "corrupt") ||
		strings.Contains(errMsg, "invalid input") || strings.Contains(errMsg, "reserved block") ||
		strings.Contains(errMsg, "invalid header") || strings.Contains(errMsg, "invalid format") {
		return ErrTypeClient
	}

	// Size violations
	if strings.Contains(errMsg, "size too big") || strings.Contains(errMsg, "size exceeded") ||
		strings.Contains(errMsg, "window size") || strings.Contains(errMsg, "size limit") {
		return ErrTypeClient
	}

	// Checksum/digest errors indicate data corruption
	if strings.Contains(errMsg, "checksum") || strings.Contains(errMsg, "digest") ||
		strings.Contains(errMsg, "crc") || strings.Contains(errMsg, "validation") ||
		strings.Contains(errMsg, "hash mismatch") {
		return ErrTypeClient
	}

	// Dictionary/encoding errors suggest data integrity issues
	if strings.Contains(errMsg, "dictionary") || strings.Contains(errMsg, "encoding") ||
		strings.Contains(errMsg, "illegal state") {
		return ErrTypeClient
	}

	// Block/frame errors indicate corruption
	if strings.Contains(errMsg, "block") || strings.Contains(errMsg, "frame") {
		return ErrTypeClient
	}

	// Decompression-specific errors that indicate data corruption
	if strings.Contains(errMsg, "decompression") || strings.Contains(errMsg, "corrupted stream") ||
		strings.Contains(errMsg, "data error") {
		return ErrTypeClient
	}

	// Default to server-side for infrastructure issues
	return ErrTypeServer
}

// determineErrorCause provides a human-readable cause for the error.
func determineErrorCause(err error) string {
	if err == nil {
		return "unknown"
	}

	errMsg := err.Error()

	// Truncated data
	if strings.Contains(errMsg, "unexpected EOF") || strings.Contains(errMsg, "EOF") ||
		strings.Contains(errMsg, "truncated") || strings.Contains(errMsg, "incomplete") {
		return "truncated_data"
	}

	// Invalid format
	if strings.Contains(errMsg, "magic") || strings.Contains(errMsg, "invalid header") ||
		strings.Contains(errMsg, "invalid format") {
		return "invalid_format"
	}

	// Corrupt data
	if strings.Contains(errMsg, "corrupt") || strings.Contains(errMsg, "corrupted stream") ||
		strings.Contains(errMsg, "data error") {
		return "corrupted_data"
	}

	// Reserved block type
	if strings.Contains(errMsg, "reserved block") {
		return "invalid_format"
	}

	// Size issues
	if strings.Contains(errMsg, "size too big") || strings.Contains(errMsg, "size exceeded") ||
		strings.Contains(errMsg, "size limit") {
		return "size_violation"
	}

	if strings.Contains(errMsg, "window size") {
		return "size_violation"
	}

	// Checksum/digest errors
	if strings.Contains(errMsg, "checksum") || strings.Contains(errMsg, "digest") ||
		strings.Contains(errMsg, "crc") || strings.Contains(errMsg, "validation") ||
		strings.Contains(errMsg, "hash mismatch") {
		return "checksum_mismatch"
	}

	// Dictionary errors
	if strings.Contains(errMsg, "dictionary") {
		return "dictionary_error"
	}

	// Encoding errors
	if strings.Contains(errMsg, "encoding") || strings.Contains(errMsg, "illegal state") {
		return "encoding_error"
	}

	// Block/frame errors
	if strings.Contains(errMsg, "block") {
		return "block_error"
	}

	if strings.Contains(errMsg, "frame") {
		return "frame_error"
	}

	// Header errors
	if strings.Contains(errMsg, "header") {
		return "header_error"
	}

	// Decompression-specific errors
	if strings.Contains(errMsg, "decompression") {
		return "decompression_failed"
	}

	// Default
	return "decompression_error"
}

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// DecryptV3 decrypts version 3 encrypted data with per-(part, block) HMAC verification and optional block decompression.
// Version 3 uses (part, block) counter construction and per-(part, block) HMACs.
//
// Parameters:
//   - encrypted: Concatenated ciphertext for all blocks in the part
//   - part: Part number (0 for single-PUT, 1..N for multipart)
//   - blockTable: Block table with HMACs and compression flags
//
// Returns:
//   - plaintext: Decrypted and decompressed plaintext
//   - err: Error if decryption, HMAC verification, or decompression fails
func (d *Decryptor) DecryptV3(encrypted []byte, part uint16, blockTable *BlockTable) ([]byte, error) {
	if d.version != Version3 {
		return nil, fmt.Errorf("DecryptV3 requires version 3, got version %d", d.version)
	}

	blockCount := blockTable.EntryCount()
	plaintext := make([]byte, 0)
	encryptedOffset := 0

	for blockIdx := 0; blockIdx < blockCount; blockIdx++ {
		entry := blockTable.Entries[blockIdx]
		blockLength := entry.RawLength()

		// Validate block boundaries
		if encryptedOffset+int(blockLength) > len(encrypted) {
			return nil, fmt.Errorf("block %d exceeds encrypted data bounds", blockIdx)
		}

		encryptedBlock := encrypted[encryptedOffset : encryptedOffset+int(blockLength)]

		// Verify HMAC with v3 (part, block) semantics
		if err := d.verifyV3BlockHMAC(encryptedBlock, part, uint32(blockIdx), entry.HMAC[:]); err != nil {
			return nil, fmt.Errorf("block %d HMAC verification failed: %w", blockIdx, err)
		}

		// Decrypt the block using v3 (part, block, aesBlock) counter
		decryptedBlock, err := d.decryptBlockV3(encryptedBlock, part, uint32(blockIdx))
		if err != nil {
			return nil, fmt.Errorf("block %d decryption failed: %w", blockIdx, err)
		}

		// Decompress if the compression flag is set
		var plaintextBlock []byte
		if entry.IsCompressed() {
			var err error
			plaintextBlock, err = DecompressBlock(decryptedBlock, true)
			if err != nil {
				return nil, fmt.Errorf("block %d decompression failed: %w", blockIdx, err)
			}
		} else {
			plaintextBlock = decryptedBlock
		}

		// Append to plaintext
		plaintext = append(plaintext, plaintextBlock...)
		encryptedOffset += int(blockLength)
	}

	return plaintext, nil
}

// verifyV3BlockHMAC verifies the HMAC for a v3 encrypted block using (part, block) semantics.
func (d *Decryptor) verifyV3BlockHMAC(encryptedBlock []byte, part uint16, block uint32, expected []byte) error {
	mac := hmac.New(sha256.New, d.hmacKey)

	// Include part number (big-endian)
	partBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(partBytes, part)
	mac.Write(partBytes)

	// Include block index (big-endian)
	blockBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(blockBytes, block)
	mac.Write(blockBytes)

	// Include ciphertext
	mac.Write(encryptedBlock)

	computed := mac.Sum(nil)

	if !hmac.Equal(computed, expected) {
		return ErrHMACMismatch
	}

	return nil
}

// decryptBlockV3 decrypts a single block using v3 (part, block, aesBlock) counter semantics.
func (d *Decryptor) decryptBlockV3(encryptedBlock []byte, part uint16, block uint32) ([]byte, error) {
	decryptedBlock := make([]byte, len(encryptedBlock))

	// Decrypt each 16-byte AES block within the ARMOR block
	numAESBlocks := (len(encryptedBlock) + 15) / 16
	for aesBlockIdx := 0; aesBlockIdx < numAESBlocks; aesBlockIdx++ {
		// Create v3 counter for this AES block
		counter := make([]byte, 16)
		copy(counter[0:8], d.iv[0:8])                  // IV[0:8]
		binary.BigEndian.PutUint16(counter[8:10], part) // uint16(part)
		binary.BigEndian.PutUint32(counter[10:14], block) // uint32(block)
		binary.BigEndian.PutUint16(counter[14:16], uint16(aesBlockIdx)) // uint16(aesBlock)

		stream := cipher.NewCTR(d.block, counter)

		// Decrypt this AES block's worth of data
		start := aesBlockIdx * 16
		end := start + 16
		if end > len(encryptedBlock) {
			end = len(encryptedBlock)
		}
		if end > start {
			stream.XORKeyStream(decryptedBlock[start:end], encryptedBlock[start:end])
		}
	}

	return decryptedBlock, nil
}

// DEK returns the data encryption key.
func (d *Decryptor) DEK() []byte {
	return d.dek
}
