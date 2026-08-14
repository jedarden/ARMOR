# Decompression Verification Helper API Design

## Overview

This document describes the design and usage of the decompression correctness verification helpers provided in `internal/crypto/verify_decompress.go`. These helpers enable byte-for-byte verification that decompressed content matches the original expected data, with detailed diagnostics for mismatches.

## Purpose

When encrypted objects are stored and later retrieved, they must be:
1. **Decrypted** using the per-object DEK and HMAC table
2. **Decompressed** (if compression was applied during upload)
3. **Verified** to ensure the decompressed output matches the original plaintext byte-for-byte

The verification helpers address step 3, providing confidence that the decompression process hasn't introduced corruption or data loss.

## Core Types

### `VerificationResult`

Represents the outcome of a decompression verification check.

```go
type VerificationResult struct {
    Passed      bool   // true if decompressed content matches expected
    Message     string // human-readable result message
    ByteOffset  int64  // -1 if passed, -2 for length mismatch, otherwise first difference offset
    ContextSize int    // number of bytes to show around the mismatch (default: 16)
}
```

**Status Codes:**
- `-1`: Verification passed (no differences)
- `-2`: Length mismatch (decompressed and expected have different sizes)
- `>= 0`: Byte offset where the first difference occurs

**Methods:**
- `Passed() bool`: Returns true if verification passed
- `String() string`: Returns the formatted message
- `GetMismatchDetail(decompressed, expected []byte) *BytesMismatch`: Extracts detailed mismatch information

### `BytesMismatch`

Provides detailed diagnostics about byte-level mismatches.

```go
type BytesMismatch struct {
    Offset        int64  // byte offset where mismatch occurs
    ExpectedByte  byte   // expected byte value at offset
    ActualByte    byte   // actual byte value at offset
    ExpectedHex   string // hexadecimal representation of expected byte
    ActualHex     string // hexadecimal representation of actual byte
    ExpectedCtx   string // context around expected mismatch (hexdump)
    ActualCtx     string // context around actual mismatch (hexdump)
    ContextBefore int    // number of bytes before mismatch in context
    ContextAfter  int    // number of bytes after mismatch in context
}
```

### `ByteStats`

Provides statistics about byte differences for pattern analysis.

```go
type ByteStats struct {
    TotalBytes      int           // total bytes compared
    MismatchCount   int           // number of mismatching bytes
    MismatchOffsets []int64       // offsets of all mismatches
    MismatchMap     map[byte]int  // frequency distribution of mismatching byte values
}
```

## Core Functions

### Full Object Verification

```go
func VerifyDecompression(decompressed, expected []byte) *VerificationResult
```

**Purpose:** Verify that a complete decompressed object matches the original expected data.

**Parameters:**
- `decompressed`: The output from the decompression pipeline
- `expected`: The original plaintext data that was compressed before encryption

**Returns:** `VerificationResult` with pass/fail status and diagnostic info

**Usage Pattern:**
```go
// After retrieving and decrypting an object
decompressed, err := crypto.Decompress(decryptedData)
if err != nil {
    return fmt.Errorf("decompression failed: %w", err)
}

// Verify against the original plaintext (e.g., from B2 metadata or a known-good copy)
result := crypto.VerifyDecompression(decompressed, originalPlaintext)
if !result.Passed() {
    return fmt.Errorf("decompression verification failed: %s", result)
}
```

### Range Request Verification

```go
func VerifyRangeDecompression(decompressed, expected []byte, rangeStart int64) *VerificationResult
```

**Purpose:** Verify that a decompressed range matches the expected range from the original object.

**Parameters:**
- `decompressed`: Decompressed range data
- `expected`: Expected range data (original plaintext slice for the range)
- `rangeStart`: The absolute byte offset where the range starts in the full object

**Returns:** `VerificationResult` with pass/fail status and diagnostic info (including absolute offsets)

**Usage Pattern:**
```go
// For HTTP range requests (e.g., "Range: bytes=1024-2047")
rangeStart := int64(1024)
rangeEnd := int64(2047)

// Retrieve and decrypt the range
decryptedRange, err := retrieveAndDecryptRange(objectID, rangeStart, rangeEnd)
if err != nil {
    return err
}

decompressedRange, err := crypto.Decompress(decryptedRange)
if err != nil {
    return fmt.Errorf("range decompression failed: %w", err)
}

// Extract the expected range from the original
expectedRange := originalPlaintext[rangeStart : rangeEnd+1]

// Verify
result := crypto.VerifyRangeDecompression(decompressedRange, expectedRange, rangeStart)
if !result.Passed() {
    return fmt.Errorf("range verification failed at offset %d: %s", 
        result.ByteOffset, result)
}
```

### Contextual Verification

```go
func VerifyDecompressionWithContext(decompressed, expected []byte, context string) *VerificationResult
```

**Purpose:** Perform verification with additional context for logging/debugging.

**Usage Pattern:**
```go
result := crypto.VerifyDecompressionWithContext(
    decompressed, 
    originalPlaintext,
    fmt.Sprintf("object=%s, version=%d", objectID, version),
)
if !result.Passed() {
    log.Error("Verification failed", "detail", result.Message)
    return result
}
```

### Advanced Analysis

```go
func AnalyzeByteDifferences(decompressed, expected []byte) *ByteStats
```

**Purpose:** Perform detailed analysis of byte-level differences for pattern detection and corruption diagnosis.

**Returns:** `ByteStats` with mismatch count, offsets, and frequency distribution

**Usage Pattern:**
```go
result := crypto.VerifyDecompression(decompressed, expected)
if !result.Passed() {
    // Analyze the corruption pattern
    stats := crypto.AnalyzeByteDifferences(decompressed, expected)
    
    log.Error("Decompression corruption detected",
        "mismatches", stats.MismatchCount,
        "percentage", float64(stats.MismatchCount)/float64(stats.TotalBytes)*100.0,
        "first_mismatch", result.ByteOffset,
    )
    
    // Check for patterns (e.g., all mismatches are the same byte value)
    topMismatches := stats.TopMismatches(5)
    for _, m := range topMismatches {
        log.Info("Common mismatching byte",
            "byte", m.Byte,
            "hex", fmt.Sprintf("0x%02X", m.Byte),
            "count", m.Count)
    }
}
```

## Integration with Existing GET and Range Helpers

### Full Object Download Integration

When performing a full object GET request:

```go
func GetAndVerifyObject(objectID string) ([]byte, error) {
    // 1. Retrieve encrypted object from B2
    encryptedData, err := b2backend.Get(objectID)
    if err != nil {
        return nil, err
    }
    
    // 2. Parse envelope header
    envelope, err := crypto.ParseEnvelope(encryptedData)
    if err != nil {
        return nil, err
    }
    
    // 3. Retrieve DEK from OpenBao
    dek, err := openbao.GetDEK(envelope.DEKID)
    if err != nil {
        return nil, err
    }
    
    // 4. Decrypt the object
    decryptor, err := crypto.NewDecryptor(dek, envelope.IV, envelope.BlockSize)
    if err != nil {
        return nil, err
    }
    
    decryptedData, err := decryptor.Decrypt(encryptedData, envelope.HMACTable)
    if err != nil {
        return nil, err
    }
    
    // 5. Decompress
    decompressed, err := crypto.Decompress(decryptedData)
    if err != nil {
        return nil, err
    }
    
    // 6. Verify (NEW STEP)
    // For this example, assume we store the original plaintext hash or a reference copy
    expectedData, err := getExpectedData(objectID)
    if err != nil {
        return nil, err
    }
    
    result := crypto.VerifyDecompression(decompressed, expectedData)
    if !result.Passed() {
        return nil, fmt.Errorf("decompression verification failed: %s", result)
    }
    
    return decompressed, nil
}
```

### Range Request Integration

When handling HTTP range requests:

```go
func GetObjectRange(ctx context.Context, objectID string, start, end int64) ([]byte, error) {
    // Get object metadata (size, encryption info)
    meta, err := b2backend.GetMetadata(objectID)
    if err != nil {
        return nil, err
    }
    
    // 1. Translate plaintext range to encrypted ranges
    translation, err := crypto.TranslateRange(start, end, meta.Size, meta.BlockSize, crypto.HeaderSize)
    if err != nil {
        return nil, err
    }
    
    // 2. Retrieve the encrypted data range and corresponding HMAC entries
    encryptedRange, err := b2backend.GetRange(objectID, translation.DataOffset, translation.DataLength)
    if err != nil {
        return nil, err
    }
    
    hmacRange, err := b2backend.GetRange(objectID, translation.HMACOffset, translation.HMACLength)
    if err != nil {
        return nil, err
    }
    
    // 3. Retrieve DEK
    dek, err := openbao.GetDEK(meta.DEKID)
    if err != nil {
        return nil, err
    }
    
    // 4. Decrypt the range
    decryptor, err := crypto.NewDecryptor(dek, meta.IV, meta.BlockSize)
    if err != nil {
        return nil, err
    }
    
    decryptedRange, err := decryptor.DecryptRange(encryptedRange, hmacRange, translation.BlockStart)
    if err != nil {
        return nil, err
    }
    
    // 5. Decompress the range
    decompressedRange, err := crypto.Decompress(decryptedRange)
    if err != nil {
        return nil, err
    }
    
    // 6. Verify the range (NEW STEP)
    // For verification, we need the expected range from the original plaintext
    // This could be stored as a separate reference or obtained from a cache
    expectedRange, err := getExpectedRange(objectID, start, end)
    if err != nil {
        return nil, err
    }
    
    result := crypto.VerifyRangeDecompression(decompressedRange, expectedRange, start)
    if !result.Passed() {
        return nil, fmt.Errorf("range verification failed at offset %d: %s", 
            result.ByteOffset, result)
    }
    
    return decompressedRange, nil
}
```

## Error Message Format

### Length Mismatch
```
length mismatch: got 1024 bytes, expected 2048 bytes
```

### Byte Mismatch (Full Object)
```
byte mismatch at offset 512: expected 0x48 (72), got 0x47 (71)
  expected context: 0a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2021222324252627
  actual context:   0a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2021222324252626
                                                ^^
```

### Byte Mismatch (Range)
```
range byte mismatch at absolute offset 1536 (relative offset 512 within range 1024-2047): expected 0x48 (72), got 0x47 (71)
  expected context: 0a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2021222324252627
  actual context:   0a0b0c0d0e0f101112131415161718191a1b1c1d1e1f2021222324252626
                                                ^^
```

### Pattern Analysis Output
```go
stats := crypto.AnalyzeByteDifferences(decompressed, expected)
fmt.Println(stats.Summary()) // "512/2048 bytes mismatch (25.00%)"

topMismatches := stats.TopMismatches(3)
// Output:
// Byte 0x00 (0): 256 occurrences
// Byte 0xFF (255): 128 occurrences
// Byte 0x47 (71): 64 occurrences
```

## Testing Strategy

### Unit Tests

```go
func TestVerifyDecompression_Success(t *testing.T) {
    original := []byte("Hello, World!")
    compressed := compressData(original)
    decompressed, _ := crypto.Decompress(compressed)
    
    result := crypto.VerifyDecompression(decompressed, original)
    
    if !result.Passed() {
        t.Errorf("Expected verification to pass, got: %s", result)
    }
    if result.ByteOffset != -1 {
        t.Errorf("Expected offset -1, got %d", result.ByteOffset)
    }
}

func TestVerifyDecompression_ByteMismatch(t *testing.T) {
    original := []byte("Hello, World!")
    corrupted := []byte("HellX, World!") // Single byte corruption at offset 4
    
    result := crypto.VerifyDecompression(corrupted, original)
    
    if result.Passed() {
        t.Error("Expected verification to fail")
    }
    if result.ByteOffset != 4 {
        t.Errorf("Expected offset 4, got %d", result.ByteOffset)
    }
    
    // Check detailed mismatch info
    mismatch := result.GetMismatchDetail(corrupted, original)
    if mismatch.ExpectedByte != 'o' || mismatch.ActualByte != 'X' {
        t.Errorf("Mismatch details incorrect")
    }
}

func TestVerifyRangeDecompression(t *testing.T) {
    fullObject := []byte("0123456789ABCDEFGHIJ")
    rangeStart := int64(5)
    rangeEnd := int64(14)
    
    // Simulate range retrieval
    decompressedRange := fullObject[rangeStart : rangeEnd+1]
    expectedRange := fullObject[rangeStart : rangeEnd+1]
    
    result := crypto.VerifyRangeDecompression(decompressedRange, expectedRange, rangeStart)
    
    if !result.Passed() {
        t.Errorf("Range verification failed: %s", result)
    }
}
```

### Integration Tests

```go
func TestFullObjectVerificationIntegration(t *testing.T) {
    // Setup: Create and store an object
    original := generateTestPayload(1024 * 1024) // 1MB
    
    // Compress, encrypt, and store
    encrypted, err := encryptAndCompressObject(original)
    if err != nil {
        t.Fatalf("Setup failed: %v", err)
    }
    
    // Retrieve, decrypt, and decompress
    retrieved, err := retrieveAndDecryptObject(encrypted)
    if err != nil {
        t.Fatalf("Retrieval failed: %v", err)
    }
    
    // Verify
    result := crypto.VerifyDecompression(retrieved, original)
    if !result.Passed() {
        t.Errorf("Full object verification failed: %s", result)
    }
}
```

## Performance Considerations

### Memory Usage

- **Verification is memory-efficient:** It operates on byte slices without allocating large intermediate structures
- **Context size is configurable:** The default 16-byte context uses minimal memory (32 hex-encoded bytes per mismatch)

### Computational Cost

- **Byte-for-byte comparison is O(n):** Linear in the size of the data
- **First mismatch detection:** Short-circuits on the first difference, avoiding full scans for most corruption cases
- **Statistical analysis:** `AnalyzeByteDifferences` performs a full scan and is O(n), but is only used for diagnostics

### Recommendations

1. **Always verify in production:** The computational cost is negligible compared to the security benefit
2. **Use `VerifyDecompression` for most cases:** It's optimized for fast pass/fail detection
3. **Use `AnalyzeByteDifferences` only for diagnostics:** It's more expensive but provides pattern analysis
4. **Adjust `ContextSize` if needed:** For very large objects, a smaller context (8 bytes) reduces log volume

## Security Considerations

### Tamper Detection

The verification helpers serve as a **defense in depth** measure:

1. **Primary:** HMAC-based integrity verification (performed during decryption)
2. **Secondary:** Byte-for-byte decompression verification (these helpers)

If decompression is buggy or compromised, these helpers will detect the mismatch even if HMAC verification passes.

### Error Handling

All verification functions are **safe to use with malformed input**:
- `nil` inputs are handled gracefully
- Length mismatches are reported clearly
- No panics on corrupt data

### Auditing

Verification failures should be:
1. **Logged** with full diagnostic details (use `VerifyDecompressionWithContext`)
2. **Alerted** for investigation (potential data corruption or attack)
3. **Tracked** in metrics (monitor verification failure rate)

## Future Enhancements

### Potential Additions

1. **Streaming verification:** Verify data as it's decompressed (for very large objects)
2. **Parallel verification:** Compare multiple ranges concurrently
3. **Checksum-based verification:** Support for BLAKE3/SHA256-based pre-computed checksums
4. **Differential analysis:** Tools for comparing multiple corrupted versions

### API Stability

The current API is **stable and production-ready**. Future enhancements will be additive and won't break existing usage.

## Summary

The decompression verification helper API provides:

- ✅ **Clear pass/fail status** via `VerificationResult.Passed`
- ✅ **Detailed diagnostics** via `BytesMismatch` with byte offsets and context
- ✅ **Full object verification** via `VerifyDecompression`
- ✅ **Range request verification** via `VerifyRangeDecompression`
- ✅ **Integration points** with existing GET and range helpers
- ✅ **Pattern analysis** via `AnalyzeByteDifferences`
- ✅ **Comprehensive error messages** with hex dumps and context
- ✅ **Production-ready** with safe error handling and no panics

The implementation is complete and ready for integration into the ARMOR retrieval pipeline.
