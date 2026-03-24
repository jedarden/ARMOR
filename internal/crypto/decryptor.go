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

// Decryptor handles AES-256-CTR decryption with per-block HMAC verification.
type Decryptor struct {
	dek       []byte
	hmacKey   []byte
	iv        []byte
	blockSize int
	block     cipher.Block
}

// NewDecryptor creates a new decryptor.
func NewDecryptor(dek, iv []byte, blockSize int) (*Decryptor, error) {
	if len(dek) != 32 {
		return nil, fmt.Errorf("DEK must be 32 bytes")
	}
	if len(iv) != 16 {
		return nil, fmt.Errorf("IV must be 16 bytes")
	}

	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	return &Decryptor{
		dek:       dek,
		hmacKey:   DeriveHMACKey(dek),
		iv:        iv,
		blockSize: blockSize,
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
func (d *Decryptor) DecryptRange(encrypted []byte, hmacTable []byte, plaintextStart, plaintextEnd int64, totalPlaintextSize int64) ([]byte, error) {
	blockStart := int(plaintextStart / int64(d.blockSize))
	blockEnd := int(plaintextEnd / int64(d.blockSize))

	// Clamp blockEnd to valid range
	maxBlocks := ComputeBlockCount(totalPlaintextSize, d.blockSize)
	if blockEnd >= int(maxBlocks) {
		blockEnd = int(maxBlocks) - 1
	}

	// Verify HMACs for all blocks in range
	for i := blockStart; i <= blockEnd; i++ {
		encStart := i * d.blockSize
		encEnd := encStart + d.blockSize
		if encEnd > len(encrypted) {
			encEnd = len(encrypted)
		}

		encryptedBlock := encrypted[encStart:encEnd]

		hmacOffset := i * HMACSize
		if hmacOffset+HMACSize > len(hmacTable) {
			return nil, fmt.Errorf("HMAC table too short for block %d", i)
		}
		expectedHMAC := hmacTable[hmacOffset : hmacOffset+HMACSize]

		if err := d.verifyBlockHMAC(encryptedBlock, uint32(i), expectedHMAC); err != nil {
			return nil, fmt.Errorf("block %d: %w", i, err)
		}
	}

	// Decrypt the blocks
	plaintext := make([]byte, plaintextEnd-plaintextStart+1)
	outputOffset := 0

	for i := blockStart; i <= blockEnd; i++ {
		encStart := i * d.blockSize
		encEnd := encStart + d.blockSize
		if encEnd > len(encrypted) {
			encEnd = len(encrypted)
		}

		encryptedBlock := encrypted[encStart:encEnd]

		// Decrypt block
		decryptedBlock := make([]byte, len(encryptedBlock))
		ctr := d.makeCounter(uint32(i))
		stream := cipher.NewCTR(d.block, ctr)
		stream.XORKeyStream(decryptedBlock, encryptedBlock)

		// Calculate which portion of this block we need
		blockPlaintextStart := int64(i * d.blockSize)
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
func (d *Decryptor) makeCounter(blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], d.iv[0:12])
	binary.BigEndian.PutUint32(counter[12:16], blockIndex)
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
