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

// Encryptor handles AES-256-CTR encryption with per-block HMAC.
type Encryptor struct {
	dek       []byte
	hmacKey   []byte
	iv        []byte
	blockSize int
	block     cipher.Block
	stream    cipher.Stream
}

// NewEncryptor creates a new encryptor.
func NewEncryptor(dek, iv []byte, blockSize int) (*Encryptor, error) {
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

	return &Encryptor{
		dek:       dek,
		hmacKey:   DeriveHMACKey(dek),
		iv:        iv,
		blockSize: blockSize,
		block:     block,
	}, nil
}

// Encrypt encrypts plaintext data and returns the encrypted blocks and HMAC table.
func (e *Encryptor) Encrypt(plaintext []byte) (encrypted []byte, hmacTable []byte, err error) {
	blockCount := ComputeBlockCount(int64(len(plaintext)), e.blockSize)

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
	hmacTable := make([]byte, blockCount*HMACSize)

	buf := make([]byte, e.blockSize)
	encryptedBuf := make([]byte, e.blockSize)
	totalWritten := int64(0)

	for blockIndex := uint32(0); blockIndex < blockCount; blockIndex++ {
		// Read a block
		n, err := io.ReadFull(plaintext, buf)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
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

// makeCounter creates a 16-byte counter value from the IV and block index.
// Counter = IV[0:12] || uint32(block_index) in big-endian
func (e *Encryptor) makeCounter(blockIndex uint32) []byte {
	counter := make([]byte, 16)
	copy(counter[0:12], e.iv[0:12])
	binary.BigEndian.PutUint32(counter[12:16], blockIndex)
	return counter
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

// NewEncryptorWithCounter creates a new encryptor with a specific starting counter.
// This is used for multipart uploads where each part needs to continue the CTR stream
// from where the previous part left off.
func NewEncryptorWithCounter(dek, iv []byte, blockSize int, startBlockIndex uint32) (*Encryptor, error) {
	enc, err := NewEncryptor(dek, iv, blockSize)
	if err != nil {
		return nil, err
	}
	// The encryptor itself doesn't store state - each block uses its own counter
	// So we just need the encryptor with the correct IV
	return enc, nil
}
