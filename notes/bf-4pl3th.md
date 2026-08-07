# Bead bf-4pl3th: Add Decompress function to crypto package

## Status: Already Implemented

The `Decompress` function already exists in `/home/coding/ARMOR/internal/crypto/decryptor.go` (lines 306-331).

## Implementation Details

### Function Signature
```go
func Decompress(compressed []byte) ([]byte, error)
```

### Features
- Uses zstd algorithm via `github.com/klauspost/compress/zstd`
- Returns data unchanged if not compressed (no zstd magic bytes)
- Returns error for corrupted or invalid compressed data
- Handles empty and short data gracefully

### Magic Number Detection
- Checks for zstd magic bytes: `0x28B52FFD` (zstd frame identifier)
- Returns original data unchanged if magic bytes not present

### Test Coverage
All tests pass:
- `TestDecompress` - Tests zstd compressed, uncompressed, empty, and short data
- `TestDecompressRoundtrip` - Tests compression/decompression roundtrip with various data sizes (small, medium, large)
- `TestEncryptDecryptCompressRoundtrip` - Tests full encryption→decryption→decompression pipeline

## Acceptance Criteria Met
✅ crypto.Decompress function exists and accepts []byte
✅ Returns decompressed []byte and error
✅ Handles zstd algorithm correctly
✅ Unit tests pass for decompression logic
