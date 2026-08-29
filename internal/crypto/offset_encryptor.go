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

// OffsetEncryptor handles encryption starting at an arbitrary byte offset.
// This is used for non-uniform multipart parts (ADR-011) where each part
// may start at a non-block-aligned position.
type OffsetEncryptor struct {
	dek       []byte
	hmacKey   []byte
	iv        []byte
	blockSize int
	version   uint8
	block     cipher.Block
}

// NewOffsetEncryptor creates a new offset encryptor.
func NewOffsetEncryptor(dek, iv []byte, blockSize int, version uint8) (*OffsetEncryptor, error) {
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

	hmacKey, err := DeriveHMACKey(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to derive HMAC key: %w", err)
	}

	return &OffsetEncryptor{
		dek:       dek,
		hmacKey:   hmacKey,
		iv:        iv,
		blockSize: blockSize,
		version:   version,
		block:     block,
	}, nil
}

// EncryptFromOffset encrypts data starting at a specific byte offset.
// Returns encrypted data and HMACs for each full/partial block.
// The offset may be non-block-aligned.
func (e *OffsetEncryptor) EncryptFromOffset(plaintext []byte, byteOffset int64) (encrypted []byte, hmacTable []byte, err error) {
	if len(plaintext) == 0 {
		return []byte{}, []byte{}, nil
	}

	// Calculate which block we start in
	startBlock := byteOffset / int64(e.blockSize)
	offsetInBlock := byteOffset % int64(e.blockSize)

	// Calculate total bytes in this part
	totalBytes := int64(len(plaintext))

	// Calculate how many blocks this part touches (including partial blocks)
	totalBlocks := (offsetInBlock + totalBytes + int64(e.blockSize) - 1) / int64(e.blockSize)

	// HMAC table size: one HMAC slot per block touched (including boundary blocks)
	// Boundary blocks will have placeholder HMACs (zeros) that get replaced during completion
	hmacTableSize := totalBlocks * HMACSize

	encrypted = make([]byte, len(plaintext))
	hmacTable = make([]byte, hmacTableSize)

	plaintextIdx := 0
	hmacIdx := 0
	blockIdx := startBlock

	// Handle partial first block if offset is not block-aligned
	if offsetInBlock > 0 {
		firstBlockEnd := min(int64(e.blockSize)-offsetInBlock, int64(len(plaintext)))
		firstBlockData := plaintext[:firstBlockEnd]

		// Encrypt the partial first block
		ctr := e.makeCounterForOffset(uint32(startBlock), offsetInBlock)
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encrypted[:firstBlockEnd], firstBlockData)

		// Leave placeholder HMAC (zeros) for this boundary block - will be computed at completion
		if firstBlockEnd == int64(len(plaintext)) {
			// Entire part is within a single partial block - return empty HMAC table
			return encrypted, []byte{}, nil
		}

		plaintextIdx = int(firstBlockEnd)
		startBlock++
		blockIdx++
		// Skip HMAC slot for the boundary block (leave as zeros)
		hmacIdx++
	}

	// Encrypt remaining blocks
	for blockIndex := startBlock; plaintextIdx < len(plaintext); blockIndex++ {
		blockData := plaintext[plaintextIdx:]
		blockEnd := plaintextIdx + e.blockSize
		if blockEnd > len(plaintext) {
			// Partial last block - leave placeholder HMAC (zeros)
			blockEnd = len(plaintext)
			// Encrypt the partial last block
			ctr := e.makeCounter(uint32(blockIndex))
			stream := cipher.NewCTR(e.block, ctr)
			stream.XORKeyStream(encrypted[plaintextIdx:blockEnd], blockData)
			// Leave HMAC slot as zeros for this boundary block
			plaintextIdx = blockEnd
			hmacIdx++
			break
		}
		blockData = plaintext[plaintextIdx:blockEnd]

		// Encrypt this full block
		ctr := e.makeCounter(uint32(blockIndex))
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encrypted[plaintextIdx:blockEnd], blockData)

		// Compute HMAC for this full block
		encryptedBlock := encrypted[plaintextIdx:blockEnd]
		hmacValue := e.computeBlockHMAC(encryptedBlock, uint32(blockIndex))
		copy(hmacTable[hmacIdx*HMACSize:], hmacValue)
		hmacIdx++

		plaintextIdx = blockEnd
	}

	return encrypted, hmacTable, nil
}

// EncryptStreamFromOffset encrypts a stream starting at a specific byte offset.
func (e *OffsetEncryptor) EncryptStreamFromOffset(plaintext io.Reader, ciphertext io.Writer, byteOffset int64, totalSize int64) ([]byte, error) {
	startBlock := byteOffset / int64(e.blockSize)
	offsetInBlock := byteOffset % int64(e.blockSize)

	totalBlocks := ComputeBlockCount(offsetInBlock+totalSize, e.blockSize)
	hmacTable := make([]byte, totalBlocks*HMACSize)

	buf := make([]byte, e.blockSize)
	encryptedBuf := make([]byte, e.blockSize)
	totalWritten := int64(0)
	blockIndex := startBlock

	// Handle partial first block if offset is not block-aligned
	if offsetInBlock > 0 {
		firstPartialSize := min(int64(e.blockSize)-offsetInBlock, totalSize)
		firstBuf := make([]byte, firstPartialSize)
		n, err := io.ReadFull(plaintext, firstBuf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("read error in partial first block: %w", err)
		}
		if n == 0 && firstPartialSize > 0 {
			return nil, fmt.Errorf("unexpected EOF in partial first block")
		}

		// Encrypt partial first block
		ctr := e.makeCounterForOffset(uint32(startBlock), offsetInBlock)
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encryptedBuf[:n], firstBuf[:n])

		// Compute HMAC (will be recomputed at completion)
		hmacValue := e.computeBlockHMACForOffset(encryptedBuf[:n], uint32(startBlock), offsetInBlock, int64(n))
		copy(hmacTable[0*HMACSize:], hmacValue)

		// Write encrypted data
		written, err := ciphertext.Write(encryptedBuf[:n])
		if err != nil {
			return nil, fmt.Errorf("write error: %w", err)
		}
		totalWritten += int64(written)

		startBlock++
		blockIndex = startBlock
	}

	// Encrypt remaining blocks
	remaining := totalSize - totalWritten
	for remaining > 0 {
		n, err := io.ReadFull(plaintext, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("read error: %w", err)
		}
		if n == 0 {
			break
		}

		// Encrypt this block
		ctr := e.makeCounter(uint32(blockIndex))
		stream := cipher.NewCTR(e.block, ctr)
		stream.XORKeyStream(encryptedBuf[:n], buf[:n])

		// Compute HMAC
		hmacValue := e.computeBlockHMAC(encryptedBuf[:n], uint32(blockIndex))
		hmacOffset := blockIndex * HMACSize
		if int(hmacOffset)+HMACSize <= len(hmacTable) {
			copy(hmacTable[hmacOffset:], hmacValue)
		}

		// Write encrypted block
		written, err := ciphertext.Write(encryptedBuf[:n])
		if err != nil {
			return nil, fmt.Errorf("write error: %w", err)
		}
		totalWritten += int64(written)
		remaining -= int64(written)
		blockIndex++
	}

	return hmacTable, nil
}

// makeCounterForOffset creates a counter for encryption starting at a specific byte offset within a block.
func (e *OffsetEncryptor) makeCounterForOffset(blockIndex uint32, offsetInBlock int64) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], e.iv[0:12])

	var counterValue uint32
	if e.version == Version2 {
		// Version2: stride by number of AES blocks per ARMOR block
		aesBlocksPerArmorBlock := uint32(e.blockSize / 16)
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
// This is the same as the encryptor's makeCounter.
func (e *OffsetEncryptor) makeCounter(blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], e.iv[0:12])

	var counterValue uint32
	if e.version == Version2 {
		aesBlocksPerArmorBlock := uint32(e.blockSize / 16)
		counterValue = blockIndex * aesBlocksPerArmorBlock
	} else {
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
	return counter
}

// computeBlockHMACForOffset computes HMAC for a block that starts at a specific byte offset.
// This is a placeholder HMAC that will be recomputed at completion.
func (e *OffsetEncryptor) computeBlockHMACForOffset(encryptedBlock []byte, blockIndex uint32, offsetInBlock int64, dataLength int64) []byte {
	mac := hmac.New(sha256.New, e.hmacKey)

	// Include block index and offset in HMAC
	indexBytes := make([]byte, 8)
	binary.BigEndian.PutUint32(indexBytes[0:4], blockIndex)
	binary.BigEndian.PutUint32(indexBytes[4:8], uint32(offsetInBlock))
	mac.Write(indexBytes)

	mac.Write(encryptedBlock)
	return mac.Sum(nil)
}

// computeBlockHMAC computes HMAC for a full block.
func (e *OffsetEncryptor) computeBlockHMAC(encryptedBlock []byte, blockIndex uint32) []byte {
	mac := hmac.New(sha256.New, e.hmacKey)

	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, blockIndex)
	mac.Write(indexBytes)

	mac.Write(encryptedBlock)
	return mac.Sum(nil)
}
