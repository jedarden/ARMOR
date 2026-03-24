package crypto

import (
	"fmt"
)

// RangeTranslation represents the translation of a plaintext range to encrypted ranges.
type RangeTranslation struct {
	// Plaintext range requested by the client
	PlaintextStart int64
	PlaintextEnd   int64

	// Block range that contains the plaintext range
	BlockStart int
	BlockEnd   int

	// Encrypted data range (offset and length within the B2 object)
	DataOffset int64
	DataLength int64

	// HMAC table range for the blocks
	HMACOffset int64
	HMACLength int64
}

// TranslateRange translates a plaintext byte range to the encrypted byte ranges needed.
// plaintextStart and plaintextEnd are inclusive byte offsets in the plaintext.
// totalPlaintextSize is the total size of the plaintext file.
func TranslateRange(plaintextStart, plaintextEnd, totalPlaintextSize int64, blockSize int, headerSize int) (*RangeTranslation, error) {
	if plaintextStart < 0 {
		return nil, fmt.Errorf("plaintext start cannot be negative")
	}
	if plaintextEnd >= totalPlaintextSize {
		plaintextEnd = totalPlaintextSize - 1
	}
	if plaintextStart > plaintextEnd {
		return nil, fmt.Errorf("plaintext start > end")
	}

	blockCount := int(ComputeBlockCount(totalPlaintextSize, blockSize))

	// Calculate block range
	blockStart := int(plaintextStart / int64(blockSize))
	blockEnd := int(plaintextEnd / int64(blockSize))

	// Clamp blockEnd to valid range
	if blockEnd >= blockCount {
		blockEnd = blockCount - 1
	}

	// Calculate encrypted data range
	// Encrypted blocks start at: headerSize + (blockStart * blockSize)
	dataOffset := int64(headerSize) + int64(blockStart*blockSize)

	// Data length: from dataOffset to end of last needed block
	// Last block might be partial, so we calculate based on actual data
	lastBlockDataEnd := int64(headerSize) + int64((blockEnd+1)*blockSize)
	totalEncryptedDataSize := int64(headerSize) + totalPlaintextSize
	if lastBlockDataEnd > totalEncryptedDataSize {
		lastBlockDataEnd = totalEncryptedDataSize
	}
	dataLength := lastBlockDataEnd - dataOffset

	// Calculate HMAC table range
	hmacTableOffset := int64(headerSize) + int64(blockCount*blockSize)
	hmacOffset := hmacTableOffset + int64(blockStart*HMACSize)
	hmacLength := int64((blockEnd-blockStart+1) * HMACSize)

	return &RangeTranslation{
		PlaintextStart: plaintextStart,
		PlaintextEnd:   plaintextEnd,
		BlockStart:     blockStart,
		BlockEnd:       blockEnd,
		DataOffset:     dataOffset,
		DataLength:     dataLength,
		HMACOffset:     hmacOffset,
		HMACLength:     hmacLength,
	}, nil
}

// TranslateFullObject returns the range translation for a full object download.
func TranslateFullObject(totalPlaintextSize int64, blockSize int, headerSize int) *RangeTranslation {
	blockCount := int(ComputeBlockCount(totalPlaintextSize, blockSize))

	return &RangeTranslation{
		PlaintextStart: 0,
		PlaintextEnd:   totalPlaintextSize - 1,
		BlockStart:     0,
		BlockEnd:       blockCount - 1,
		DataOffset:     int64(headerSize),
		DataLength:     totalPlaintextSize,
		HMACOffset:     int64(headerSize) + int64(blockCount*blockSize),
		HMACLength:     int64(blockCount * HMACSize),
	}
}

// FullObjectSize returns the total size of the encrypted object in B2.
func FullObjectSize(plaintextSize int64, blockSize int) int64 {
	blockCount := ComputeBlockCount(plaintextSize, blockSize)
	return int64(HeaderSize) + plaintextSize + int64(blockCount*HMACSize)
}
