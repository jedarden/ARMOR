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

// BoundaryBlock represents a block that spans two parts.
type BoundaryBlock struct {
	PartNumber      int      // The part number this block belongs to
	BlockIndex      uint32   // Absolute block index in the object
	ByteOffset      int64    // Byte offset where this block starts (in the encrypted object)
	PartStartOffset int64    // Byte offset of this part's data within the block
	PartLength      int64    // Length of this part's data within the block
}

// BoundaryHMACComputer computes HMACs for boundary blocks that span parts.
type BoundaryHMACComputer struct {
	hmacKey   []byte
	blockSize int
}

// NewBoundaryHMACComputer creates a new computer for boundary block HMACs.
func NewBoundaryHMACComputer(dek []byte, blockSize int) (*BoundaryHMACComputer, error) {
	if len(dek) != 32 {
		return nil, fmt.Errorf("DEK must be 32 bytes")
	}

	return &BoundaryHMACComputer{
		hmacKey:   DeriveHMACKey(dek),
		blockSize: blockSize,
	}, nil
}

// ComputeBoundaryHMACs computes HMACs for all boundary blocks in a multipart upload.
// It reads each part's contribution to boundary blocks, concatenates them, and computes HMACs.
//
// Parameters:
//   - objectReader: Reader for the completed encrypted object
//   - partSizes: Map of part number -> plaintext size for each part
//   - blockSize: The encryption block size
//
// Returns: map[blockIndex]HMAC for all boundary blocks
func (b *BoundaryHMACComputer) ComputeBoundaryHMACs(objectReader io.ReaderAt, partSizes map[int]int64, blockSize int) (map[uint32][]byte, error) {
	boundaryHMACs := make(map[uint32][]byte)

	// Find all segments that contribute to boundary blocks
	segmentsByBlock := b.findBoundarySegments(partSizes, blockSize)

	// For each block that has multiple segments (i.e., a boundary block), compute its HMAC
	for blockIdx, segments := range segmentsByBlock {
		// Skip blocks that are not boundary blocks (only one segment)
		if len(segments) <= 1 {
			continue
		}

		// Sort segments by BlockOffset to ensure correct order
		// Use simple insertion sort (small number of segments)
		for i := 0; i < len(segments); i++ {
			for j := i + 1; j < len(segments); j++ {
				if segments[i].BlockOffset > segments[j].BlockOffset {
					segments[i], segments[j] = segments[j], segments[i]
				}
			}
		}

		// Reassemble the full block by concatenating all segments
		fullBlock := make([]byte, 0, blockSize)
		for _, seg := range segments {
			segData := make([]byte, seg.Length)
			n, err := objectReader.ReadAt(segData, seg.ByteOffset)
			if err != nil && err != io.EOF {
				return nil, fmt.Errorf("failed to read segment for block %d (part %d): %w", blockIdx, seg.PartNumber, err)
			}
			if n < len(segData) {
				return nil, fmt.Errorf("short read for segment in block %d (part %d): expected %d, got %d", blockIdx, seg.PartNumber, len(segData), n)
			}
			fullBlock = append(fullBlock, segData...)
		}

		// Compute HMAC for the reassembled full block
		hmacValue := b.computeBlockHMAC(fullBlock, blockIdx)
		boundaryHMACs[blockIdx] = hmacValue
	}

	return boundaryHMACs, nil
}

// BoundarySegment represents one part's contribution to a boundary block.
type BoundarySegment struct {
	PartNumber  int     // Part number
	ByteOffset  int64   // Offset in the encrypted object where this part's contribution starts
	Length      int64   // Length of this part's contribution
	BlockOffset int64   // Offset of this data within the block (0-based)
}

// findBoundarySegments identifies all segments that contribute to boundary blocks.
// Returns a map of block index -> list of segments that make up that block.
func (b *BoundaryHMACComputer) findBoundarySegments(partSizes map[int]int64, blockSize int) map[uint32][]BoundarySegment {
	segments := make(map[uint32][]BoundarySegment)

	// Sort part numbers
	partNumbers := make([]int, 0, len(partSizes))
	for pn := range partSizes {
		partNumbers = append(partNumbers, pn)
	}
	// Simple insertion sort (small number of parts)
	for i := 0; i < len(partNumbers); i++ {
		for j := i + 1; j < len(partNumbers); j++ {
			if partNumbers[i] > partNumbers[j] {
				partNumbers[i], partNumbers[j] = partNumbers[j], partNumbers[i]
			}
		}
	}

	// Track cumulative offset and identify boundary segments
	cumulativeOffset := int64(0)

	for _, partNumber := range partNumbers {
		partSize := partSizes[partNumber]
		if partSize == 0 {
			continue
		}

		partStartOffset := cumulativeOffset
		partEndOffset := cumulativeOffset + partSize

		// Find all blocks this part touches
		startBlock := partStartOffset / int64(blockSize)
		endBlock := (partEndOffset - 1) / int64(blockSize)

		// If this part spans multiple blocks, identify each block it contributes to
		for blockIdx := startBlock; blockIdx <= endBlock; blockIdx++ {
			blockStart := blockIdx * int64(blockSize)
			blockEnd := blockStart + int64(blockSize)

			// Calculate this part's contribution to this block
			segStartInBlock := max(partStartOffset, blockStart) - blockStart
			segEndInBlock := min(partEndOffset, blockEnd) - blockStart
			segLength := segEndInBlock - segStartInBlock

			if segLength > 0 {
				// This part contributes to this block
				seg := BoundarySegment{
					PartNumber:  partNumber,
					ByteOffset: blockStart + segStartInBlock, // Absolute offset in encrypted object
					Length:     segLength,
					BlockOffset: segStartInBlock,
				}
				segments[uint32(blockIdx)] = append(segments[uint32(blockIdx)], seg)
			}
		}

		cumulativeOffset = partEndOffset
	}

	return segments
}

// computeBlockHMAC computes HMAC for a single block.
func (b *BoundaryHMACComputer) computeBlockHMAC(encryptedBlock []byte, blockIndex uint32) []byte {
	mac := hmac.New(sha256.New, b.hmacKey)

	// Include block index in HMAC to prevent block reordering
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, blockIndex)
	mac.Write(indexBytes)

	mac.Write(encryptedBlock)
	return mac.Sum(nil)
}

// DecryptBoundaryBlock decrypts a boundary block for verification.
// This is used during testing to verify boundary blocks are correctly encrypted.
func DecryptBoundaryBlock(dek, iv, encryptedBlock []byte, blockIndex uint32, blockSize int, version uint8) ([]byte, error) {
	block, err := aes.NewCipher(dek)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	decrypted := make([]byte, len(encryptedBlock))

	// Create counter for this block
	counter := make([]byte, 16)
	copy(counter[0:12], iv[0:12])

	var counterValue uint32
	if version == Version2 {
		aesBlocksPerArmorBlock := uint32(blockSize / 16)
		counterValue = blockIndex * aesBlocksPerArmorBlock
	} else {
		counterValue = blockIndex
	}

	binary.BigEndian.PutUint32(counter[12:16], counterValue)
	stream := cipher.NewCTR(block, counter)
	stream.XORKeyStream(decrypted, encryptedBlock)

	return decrypted, nil
}
