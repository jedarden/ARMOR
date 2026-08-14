# Core Error Message Structure for Verification Failures

## Overview

This document defines the standard error message format for decompression verification failures in ARMOR. All verification error reporting must conform to this structure to ensure consistency across the restore verification pipeline (ADR-004).

## Core Standard Format

The core error message structure is defined as a JSON schema with three required fields that form the foundation for all verification error reporting:

```json
{
  "offset": "<int64>",
  "expected": "<byte[]>",
  "actual": "<byte[]>"
}
```

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `offset` | `int64` | Position in the decompressed data where the mismatch occurred (0-indexed byte offset) |
| `expected` | `byte[]` | The expected byte(s) at the error offset |
| `actual` | `byte[]` | The actual byte(s) received at the error offset |

### Field Specifications

#### offset (int64)
- **Purpose**: Identifies the precise byte position where verification failed
- **Calculation**: Start byte-by-byte comparison from position 0; the first position `i` where `decompressed[i] != expected[i]` is the offset
- **Special Values**:
  - `-1`: Content is identical (no error)
  - `-2`: Length mismatch error (total sizes differ)
  - `< -2`: Reserved for future error types
- **Examples**:
  ```
  decompressed: [0x01, 0x02, 0x00, 0x04] (4 bytes)
  expected:      [0x01, 0x02, 0x03, 0x04]
  offset:       2 (third byte differs: 0x00 vs 0x03)
  ```

#### expected (byte[])
- **Purpose**: The reference byte(s) that should be present at the error offset
- **Format**: 
  - Single-byte errors: Contains exactly 1 byte
  - Multi-byte context errors: Contains [ContextBefore + 1 + ContextAfter] bytes centered on the error offset
  - Length mismatch: Contains full expected data for size comparison
  - Out of range: nil (empty)
- **Examples**:
  ```
  Single-byte error:    [0x03]
  With context (8 bytes): [0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08]
  ```

#### actual (byte[])
- **Purpose**: The actual byte(s) found in the decompressed content at the error offset
- **Format**: Mirrors the `expected` field structure
- **Examples**:
  ```
  Single-byte error:    [0x00] (corruption: null byte overwrite)
  Bit-flip error:      [0x54] (0x55 expected, 1 bit differs)
  With context:         [0x01, 0x02, 0x00, 0x04, 0x05, 0x06, 0x07, 0x08]
  ```

## Data Type Definitions

### Byte Representation
- **Type**: `byte[]` (array of bytes)
- **Serialization**: Hexadecimal string or base64 encoding
- **Constraints**: 
  - Minimum length: 0 (empty array)
  - Maximum length: Unbounded (constrained by available memory)
  - Each byte: 0x00-0xFF (unsigned 8-bit integer)

### Integer Representation
- **Type**: `int64` (64-bit signed integer)
- **Range**: -9,223,372,036,854,775,808 to 9,223,372,036,854,775,807
- **Special Values**: Reserved negative range for error classifications

## Standard Structure Schema

```go
// CoreVerificationError defines the minimum required fields for all verification errors.
type CoreVerificationError struct {
    // Offset is the byte position where the first difference occurs.
    // Special values: -1 (no error), -2 (length mismatch), <-2 (reserved)
    Offset int64 `json:"offset"`

    // Expected is the byte(s) that should be present at the error offset.
    // Single-byte errors: 1 byte. Context errors: [ContextBefore + 1 + ContextAfter] bytes.
    Expected []byte `json:"expected"`

    // Actual is the byte(s) found in the decompressed content at the error offset.
    // Mirrors Expected structure.
    Actual []byte `json:"actual"`
}
```

## Extended Structure (Optional Fields)

The core structure may be extended with optional context fields for detailed diagnostics:

```json
{
  "offset": "<int64>",
  "expected": "<byte[]>",
  "actual": "<byte[]>",
  "context_bytes": "<int>",           // Number of surrounding bytes included
  "context_before": "<int>",           // Actual bytes before error offset
  "context_after": "<int>",            // Actual bytes after error offset
  "expected_length": "<int>",          // Total length of expected data
  "actual_length": "<int>",            // Total length of actual data
  "error_type": "<string>",            // Error classification: "byte_mismatch", "length_mismatch", "out_of_range"
  "severity": "<string>"               // Severity: "critical", "high", "medium", "low"
}
```

## Error Classification

### Byte Mismatch Error
```json
{
  "offset": 512,
  "expected": [0x03],
  "actual": [0x00],
  "error_type": "byte_mismatch",
  "severity": "high"
}
```
**Interpretation**: Null byte overwrite at position 512

### Length Mismatch Error
```json
{
  "offset": -2,
  "expected": [0x01, 0x02, ...],  // full expected data
  "actual": [0x01, 0x02, ...],    // full actual data
  "expected_length": 1024,
  "actual_length": 997,
  "error_type": "length_mismatch",
  "severity": "critical"
}
```
**Interpretation**: Decompressed output is 27 bytes too short

### Bit-Flip Error
```json
{
  "offset": 1024,
  "expected": [0x55],
  "actual": [0x54],
  "error_type": "byte_mismatch",
  "severity": "medium"
}
```
**Interpretation**: Single bit difference at position 1024 (01010101 vs 01010100)

## Implementation Requirements

### Code Integration
All verification functions must return results that include the three required fields:

```go
func VerifyDecompression(decompressed, expected []byte) *VerificationResult {
    result := &VerificationResult{}
    
    // Check if lengths match
    if len(decompressed) != len(expected) {
        result.Offset = -2  // Special code for length mismatch
        result.Expected = expected
        result.Actual = decompressed
        result.Passed = false
        return result
    }
    
    // Find the first mismatching byte
    for i := range expected {
        if decompressed[i] != expected[i] {
            result.Offset = int64(i)
            result.Expected = []byte{expected[i]}
            result.Actual = []byte{decompressed[i]}
            result.Passed = false
            return result
        }
    }
    
    // All bytes match
    result.Offset = -1
    result.Passed = true
    return result
}
```

### Error Message Format
Human-readable error messages must follow this format:

```
verification failed: byte mismatch at offset {offset} (expected 0x{Expected}, got 0x{Actual})
```

Example:
```
verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00)
```

### Serialization Format
For machine-readable output (JSON), the structure must serialize as:

```json
{
  "offset": 512,
  "expected": "0x03",
  "actual": "0x00"
}
```

Or with context:
```json
{
  "offset": 512,
  "expected": "0x01 0x02 0x03 0x04",
  "actual": "0x01 0x02 0x00 0x04",
  "context_bytes": 1,
  "context_before": 2,
  "context_after": 1
}
```

## Integration Points

### Restore Verifier
The `restoreverifier` package uses this structure in `VerificationResult`:

```go
type VerificationResult struct {
    Key          string
    Bucket       string
    Status       VerificationStatus
    Error        string  // Formatted from CoreVerificationError
    ByteOffset   int64   // Maps to offset
    ExpectedSHA256 string
    ActualSHA256   string
}
```

### HTTP Handlers
Error responses to clients must include the core fields:

```go
w.Header().Set("Content-Type", "application/json")
json.NewEncoder(w)._encode(map[string]interface{}{
    "error": "verification failed",
    "offset": result.ByteOffset,
    "expected": fmt.Sprintf("0x%02X", expectedBytes),
    "actual": fmt.Sprintf("0x%02X", actualBytes),
})
```

### Escalation Beads
Verification failures escalated as beads must include the core structure in bead metadata for correlation and debugging.

## Testing Requirements

All verification error reporting must include tests that validate:

1. **Core Field Presence**: All errors include offset, expected, actual
2. **Offset Accuracy**: Offset correctly identifies first mismatch
3. **Byte Accuracy**: Expected/Actual bytes match the mismatch position
4. **Special Values**: Reserved offset values (-1, -2) used correctly
5. **Serialization**: JSON serialization preserves field values

Example test:
```go
func TestCoreErrorStructure(t *testing.T) {
    decompressed := []byte{0x01, 0x02, 0x00, 0x04}
    expected := []byte{0x01, 0x02, 0x03, 0x04}
    
    result := VerifyDecompression(decompressed, expected)
    
    // Validate required fields
    if result.Offset != 2 {
        t.Errorf("Expected offset 2, got %d", result.Offset)
    }
    if len(result.Expected) != 1 || result.Expected[0] != 0x03 {
        t.Errorf("Expected [0x03], got %v", result.Expected)
    }
    if len(result.Actual) != 1 || result.Actual[0] != 0x00 {
        t.Errorf("Expected [0x00], got %v", result.Actual)
    }
}
```

## Version History

- **v1.0** (2026-08-14): Initial core structure definition with three required fields
- See `internal/crypto/verify_decompress.go` for implementation reference

## References

- ADR-004: Restore Verification Pipeline
- `internal/crypto/verify_decompress.go`: Core verification implementation
- `internal/restoreverifier/verifier.go`: Restore verifier integration
- Test files: `internal/crypto/crypto_decompress_test.go`