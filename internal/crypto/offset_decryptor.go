package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
)

// OffsetDecryptor handles decryption starting at an arbitrary byte offset.
// This is used for non-uniform multipart parts (ADR-011).
type OffsetDecryptor struct {
	dek       []byte
	hmacKey   []byte
	iv        []byte
	blockSize int
	version   uint8
	block     cipher.Block
}

// NewOffsetDecryptor creates a new offset decryptor.
func NewOffsetDecryptor(dek, iv []byte, blockSize int, version uint8) (*OffsetDecryptor, error) {
	if len(dek) != 32 {
		return nil, fmt.Errorf("DEK must be 32 bytes")
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("IV must be 16 bytes")
	}

	if version != Version1 && version != Version2 {
		return nil, fmt.Errorf("unsupported version: %d", version)
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	return &OffsetDecryptor{
		dek:       dek,
		hmacKey:   DeriveHMACKey(dek),
		iv:        iv,
		blockSize: blockSize,
		version:   version,
		block:     block,
	}, nil
}

// DecryptFromOffset decrypts data starting at a specific byte offset.
func (d *OffsetDecryptor) DecryptFromOffset(encrypted []byte, byteOffset int64) (plaintext []byte, err error) {
	if len(encrypted) == 0 {
		return []byte{}, nil
	}

	startBlock := byteOffset / int64(d.blockSize)
	offsetInBlock := byteOffset % int64(d.blockSize)

	plaintext = make([]byte, len(encrypted))
	plaintextIdx := 0
	encryptedIdx := 0

	// Handle partial first block if offset is not block-aligned
	if offsetInBlock > 0 {
		firstBlockEnd := min(int64(d.blockSize)-offsetInBlock, int64(len(encrypted)))
		firstBlockData := encrypted[:firstBlockEnd]

		// Decrypt the partial first block
		ctr := d.makeCounterForOffset(uint32(startBlock), offsetInBlock)
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(plaintext[:firstBlockEnd], firstBlockData)

		plaintextIdx = int(firstBlockEnd)
		encryptedIdx = int(firstBlockEnd)
		startBlock++
	}

	// Decrypt remaining full blocks
	for encryptedIdx < len(encrypted) {
		blockEnd := encryptedIdx + d.blockSize
		if blockEnd > len(encrypted) {
			blockEnd = len(encrypted)
		}

		encryptedBlock := encrypted[encryptedIdx:blockEnd]

		// Decrypt this block
		ctr := d.makeCounter(uint32(startBlock))
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(plaintext[plaintextIdx:blockEnd], encryptedBlock)

		plaintextIdx = blockEnd - encryptedIdx
		encryptedIdx = blockEnd
		startBlock++
	}

	return plaintext, nil
}

// DecryptAtOffset decrypts a specific range starting at an arbitrary byte offset.
// This is used for range requests on non-uniform multipart objects.
func (d *OffsetDecryptor) DecryptAtOffset(encrypted []byte, byteOffset int64, length int64) ([]byte, error) {
	if len(encrypted) == 0 {
		return []byte{}, nil
	}

	if length > int64(len(encrypted)) {
		length = int64(len(encrypted))
	}

	startBlock := byteOffset / int64(d.blockSize)
	offsetInBlock := byteOffset % int64(d.blockSize)

	plaintext := make([]byte, length)
	plaintextIdx := 0
	encryptedIdx := 0

	// Handle partial first block if offset is not block-aligned
	if offsetInBlock > 0 {
		firstBlockEnd := min(int64(d.blockSize)-offsetInBlock, length)
		firstBlockData := encrypted[:firstBlockEnd]

		// Decrypt the partial first block
		ctr := d.makeCounterForOffset(uint32(startBlock), offsetInBlock)
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(plaintext[:firstBlockEnd], firstBlockData)

		plaintextIdx = int(firstBlockEnd)
		encryptedIdx = int(firstBlockEnd)
		startBlock++
	}

	// Decrypt remaining blocks
	for plaintextIdx < len(plaintext) && encryptedIdx < len(encrypted) {
		blockEnd := encryptedIdx + d.blockSize
		if blockEnd > len(encrypted) {
			blockEnd = len(encrypted)
		}
		if plaintextIdx+(blockEnd-encryptedIdx) > len(plaintext) {
			blockEnd = encryptedIdx + (len(plaintext) - plaintextIdx)
		}

		encryptedBlock := encrypted[encryptedIdx:blockEnd]

		// Decrypt this block
		ctr := d.makeCounter(uint32(startBlock))
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(plaintext[plaintextIdx:], encryptedBlock)

		plaintextIdx += blockEnd - encryptedIdx
		encryptedIdx = blockEnd
		startBlock++
	}

	return plaintext, nil
}

// DecryptStreamFromOffset decrypts a stream starting at a specific byte offset.
func (d *OffsetDecryptor) DecryptStreamFromOffset(ciphertext io.Reader, plaintext io.Writer, byteOffset int64, totalSize int64) error {
	startBlock := byteOffset / int64(d.blockSize)
	offsetInBlock := byteOffset % int64(d.blockSize)

	buf := make([]byte, d.blockSize)
	decryptedBuf := make([]byte, d.blockSize)
	totalRead := int64(0)
	blockIndex := startBlock

	// Handle partial first block if offset is not block-aligned
	if offsetInBlock > 0 {
		firstPartialSize := min(int64(d.blockSize)-offsetInBlock, totalSize)
		firstBuf := make([]byte, firstPartialSize)
		n, err := io.ReadFull(ciphertext, firstBuf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read error in partial first block: %w", err)
		}
		if n == 0 && firstPartialSize > 0 {
			return fmt.Errorf("unexpected EOF in partial first block")
		}

		// Decrypt partial first block
		ctr := d.makeCounterForOffset(uint32(startBlock), offsetInBlock)
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(decryptedBuf[:n], firstBuf[:n])

		// Write decrypted data
		if _, err := plaintext.Write(decryptedBuf[:n]); err != nil {
			return fmt.Errorf("write error: %w", err)
		}

		totalRead += int64(n)
		startBlock++
		blockIndex = startBlock
	}

	// Decrypt remaining blocks
	remaining := totalSize - totalRead
	for remaining > 0 {
		n, err := io.ReadFull(ciphertext, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return fmt.Errorf("read error: %w", err)
		}
		if n == 0 {
			break
		}

		// Decrypt this block
		ctr := d.makeCounter(uint32(blockIndex))
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(decryptedBuf[:n], buf[:n])

		// Write decrypted block
		if _, err := plaintext.Write(decryptedBuf[:n]); err != nil {
			return fmt.Errorf("write error: %w", err)
		}

		totalRead += int64(n)
		remaining -= int64(n)
		blockIndex++
	}

	return nil
}

// makeCounterForOffset creates a counter for decryption starting at a specific byte offset within a block.
func (d *OffsetDecryptor) makeCounterForOffset(blockIndex uint32, offsetInBlock int64) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], d.iv[0:12])

	var counterValue uint32
	if d.version == Version2 {
		// Version2: stride by number of AES blocks per ARMOR block
		aesBlocksPerArmorBlock := uint32(d.blockSize / 16)
		baseCounter := blockIndex * aesBlocksPerArmorBlock
		// Add the offset within the block (in AES blocks)
		aesBlocksOffset := uint32(offsetInBlock / 16)
		counterValue = baseCounter + aesBlocksOffset
	} else {
		// Version1: legacy derivation
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
	return counter
}

// makeCounter creates a 16-byte counter value from the IV and block index.
func (d *OffsetDecryptor) makeCounter(blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], d.iv[0:12])

	var counterValue uint32
	if d.version == Version2 {
		aesBlocksPerArmorBlock := uint32(d.blockSize / 16)
		counterValue = blockIndex * aesBlocksPerArmorBlock
	} else {
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
	return counter
}

// VerifyBlockHMAC verifies the HMAC for a single encrypted block.
func (d *OffsetDecryptor) VerifyBlockHMAC(encryptedBlock []byte, blockIndex uint32, expected []byte) error {
	mac := hmac.New(sha256.New, d.hmacKey)

	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, blockIndex)
	mac.Write(indexBytes)

	mac.Write(encryptedBlock)
	actual := mac.Sum(nil)

	if !hmac.Equal(expected, actual) {
		return fmt.Errorf("HMAC verification failed for block %d", blockIndex)
	}

	return nil
}
