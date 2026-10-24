// Package crypto provides compression utilities for ARMOR.
package crypto

import (
	"fmt"
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// zstd codecs are expensive to build (the encoder allocates its window and
// level tables; the decoder its own frame state), and every compressed block
// in the ARMOR hot path goes through Compress/DecompressBlock/Decompress.
// Pool both and reuse them across calls instead of constructing per call.
// Pooled codecs are never closed: Close retires a codec permanently, and a
// closed codec returned to the pool would poison every later borrower.
var (
	zstdEncoderPool = sync.Pool{
		New: func() any {
			encoder, err := zstd.NewWriter(nil)
			if err != nil {
				panic(fmt.Sprintf("crypto: zstd encoder construction failed: %v", err))
			}
			return encoder
		},
	}
	zstdDecoderPool = sync.Pool{
		New: func() any {
			decoder, err := zstd.NewReader(nil)
			if err != nil {
				panic(fmt.Sprintf("crypto: zstd decoder construction failed: %v", err))
			}
			return decoder
		},
	}
)

// CompressionType specifies the compression algorithm used.
type CompressionType string

const (
	CompressionNone CompressionType = ""
	CompressionZstd CompressionType = "zstd"
)

// Compress compresses data using zstd with opportunistic pass-through.
// If the compressed size is >= the original size, returns the original data.
// This ensures we don't waste CPU on data that doesn't compress well (e.g., Parquet).
//
// Returns:
// - compressedData: The compressed data (or original if compression didn't help)
// - compressed: true if the data was compressed, false if original was returned
// - compressionType: The type of compression used (or CompressionNone if not compressed)
func Compress(plaintext []byte) ([]byte, bool, CompressionType, error) {
	if len(plaintext) == 0 {
		return plaintext, false, CompressionNone, nil
	}

	// Compress the data with a pooled encoder; the dst is sized to the
	// plaintext so EncodeAll never regrows it for a compressible block.
	encoder := zstdEncoderPool.Get().(*zstd.Encoder)
	compressed := encoder.EncodeAll(plaintext, make([]byte, 0, len(plaintext)))
	zstdEncoderPool.Put(encoder)

	// Opportunistic pass-through: if compression didn't help, use original
	if len(compressed) >= len(plaintext) {
		return plaintext, false, CompressionNone, nil
	}

	return compressed, true, CompressionZstd, nil
}

// CompressStream compresses data from a reader using zstd with opportunistic pass-through.
// Reads all data from r, compresses it, and returns the compressed (or original) data.
// If the compressed size is >= the original size, returns the original data.
//
// Returns:
// - compressedData: The compressed data (or original if compression didn't help)
// - compressed: true if the data was compressed, false if original was returned
// - compressionType: The type of compression used (or CompressionNone if not compressed)
// - originalSize: The size of the original uncompressed data
func CompressStream(r io.Reader) ([]byte, bool, CompressionType, int64, error) {
	// Read all data from reader
	original, err := io.ReadAll(r)
	if err != nil {
		return nil, false, CompressionNone, 0, fmt.Errorf("failed to read data: %w", err)
	}

	originalSize := int64(len(original))
	if originalSize == 0 {
		return original, false, CompressionNone, originalSize, nil
	}

	compressed, wasCompressed, compressionType, err := Compress(original)
	if err != nil {
		return nil, false, CompressionNone, originalSize, err
	}

	return compressed, wasCompressed, compressionType, originalSize, nil
}

// DecompressData decompresses zstd-compressed data (full in-memory operation).
// This is a convenience wrapper around the existing Decompress function.
func DecompressData(compressed []byte) ([]byte, error) {
	return Decompress(compressed)
}

// DecompressStream creates a reader that decompresses zstd-compressed data on the fly.
// Returns a ReadCloser that must be closed when done.
func DecompressStream(r io.Reader) (io.ReadCloser, error) {
	decoder, err := zstd.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd decoder: %w", err)
	}

	// Wrap the decoder to add a proper Close method
	return &readCloserWrapper{decoder}, nil
}

// readCloserWrapper wraps zstd.Decoder to provide io.ReadCloser interface.
type readCloserWrapper struct {
	*zstd.Decoder
}

// Close implements io.ReadCloser by closing the underlying decoder.
func (w *readCloserWrapper) Close() error {
	w.Decoder.Close()
	return nil
}

// CompressBlock compresses a single plaintext block with opportunistic pass-through.
// This is a convenience wrapper around Compress for per-block compression operations.
// Returns:
// - compressedData: The compressed data (or original if compression didn't help)
// - compressed: true if the data was compressed, false if original was returned
// - compressionType: The type of compression used (or CompressionNone if not compressed)
func CompressBlock(plaintextBlock []byte) ([]byte, bool, CompressionType, error) {
	return Compress(plaintextBlock)
}

// DecompressBlock decompresses a single decrypted block based on the compression flag.
// If isCompressed is false, returns the data unchanged.
// If isCompressed is true, attempts zstd decompression.
// Returns:
// - decompressedData: The decompressed data (or original if not compressed)
// - err: Error if decompression fails when isCompressed is true
func DecompressBlock(decryptedBlock []byte, isCompressed bool) ([]byte, error) {
	if !isCompressed {
		return decryptedBlock, nil
	}

	// Attempt decompression with a pooled decoder
	decoder := zstdDecoderPool.Get().(*zstd.Decoder)
	decompressed, err := decoder.DecodeAll(decryptedBlock, nil)
	if err != nil {
		zstdDecoderPool.Put(decoder)
		return nil, fmt.Errorf("block decompression failed: %w", err)
	}
	zstdDecoderPool.Put(decoder)

	return decompressed, nil
}
