# VerificationError to Error Message Mapping

## Overview

This document describes how the `VerificationError` type in ARMOR's crypto package maps to the standardized error message format defined in `error-message-structure.md`. This mapping is critical for the restore verification pipeline (ADR-004), ensuring that all verification errors are consistently formatted and include required diagnostic information.

## Type Structure vs Message Format

### VerificationError Type Definition

The `VerificationError` type is defined in `internal/crypto/verify_decompress.go`:

```go
type VerificationError struct {
    // Offset is the byte position where the first difference occurs
    Offset int64

    // Expected is the byte(s) that should be present at the error offset
    Expected []byte

    // Actual is the byte(s) found in the decompressed content
    Actual []byte

    // ContextBytes specifies the number of surrounding bytes included
    ContextBytes int

    // ContextBefore is the actual number of bytes before the error offset
    ContextBefore int

    // ContextAfter is the actual number of bytes after the error offset
    ContextAfter int

    // ExpectedLength is the total length of the expected data
    ExpectedLength int

    // ActualLength is the total length of the decompressed data
    ActualLength int
}
```

### Core Message Format

The standardized error message format requires three core fields:

```json
{
  "offset": "<int64>",
  "expected": "<byte[]>",
  "actual": "<byte[]>"
}
```

## Field-by-Field Mapping

### Required Fields Mapping

| Message Field | VerificationError Field | Mapping Logic |
|--------------|-------------------------|---------------|
| `offset` | `Offset` | Direct 1:1 mapping. Special values: -1 (no error), -2 (length mismatch) |
| `expected` | `Expected` | Direct byte array mapping. May contain 1 byte or context bytes |
| `actual` | `Actual` | Direct byte array mapping. Mirrors expected structure |

### Optional Context Fields Mapping

| Message Field | VerificationError Field(s) | Mapping Logic |
|--------------|---------------------------|---------------|
| `surrounding_bytes.before` | Not directly stored | Computed from data slice at runtime |
| `surrounding_bytes.after` | Not directly stored | Computed from data slice at runtime |
| `context_bytes` | `ContextBytes` | Direct mapping: requested context window size |
| `context_before` | `ContextBefore` | Direct mapping: actual bytes available before offset |
| `context_after` | `ContextAfter` | Direct mapping: actual bytes available after offset |
| `expected_length` | `ExpectedLength` | Direct mapping: total expected data size |
| `actual_length` | `ActualLength` | Direct mapping: total decompressed data size |

## Conversion Logic

### Human-Readable Message Format

The `VerificationError.Error()` method implements the conversion to human-readable format:

```go
func (ve *VerificationError) Error() string {
    // Handle nil case
    if ve == nil {
        return "verification failed: nil error"
    }

    // Length mismatch (special offset code)
    if ve.Offset == -2 {
        return fmt.Sprintf("verification failed: length mismatch (got %d bytes, expected %d bytes)",
            ve.ActualLength, ve.ExpectedLength)
    }

    // Out of range offset
    if ve.Offset < 0 {
        return fmt.Sprintf("verification failed: invalid offset %d", ve.Offset)
    }

    // Build expected/actual representation
    expectedStr := formatBytes(ve.Expected)
    actualStr := formatBytes(ve.Actual)

    // Byte mismatch error
    msg := fmt.Sprintf("verification failed: byte mismatch at offset %d (expected %s, got %s)",
        ve.Offset, expectedStr, actualStr)

    // Add context information if available
    if ve.ContextBytes > 0 {
        msg += fmt.Sprintf(" [%d bytes context: %d before, %d after]",
            ve.ContextBytes, ve.ContextBefore, ve.ContextAfter)
    }

    return msg
}
```

### JSON Serialization Format

For machine-readable output, the VerificationError maps to JSON as follows:

**Minimal mapping (core fields only):**
```json
{
  "offset": 512,
  "expected": "0x03",
  "actual": "0x00"
}
```

**With context fields:**
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

## Error Type-Specific Mapping

### 1. Length Mismatch Error

**Condition:** `Offset == -2`

**VerificationError State:**
```go
VerificationError{
    Offset: -2,
    ExpectedLength: 1024,
    ActualLength: 997,
    Expected: <full expected data>,
    Actual: <full actual data>
}
```

**Message Format:**
```
"verification failed: length mismatch (got 997 bytes, expected 1024 bytes)"
```

**JSON Output:**
```json
{
  "offset": -2,
  "expected_length": 1024,
  "actual_length": 997,
  "error_type": "length_mismatch",
  "severity": "critical"
}
```

### 2. Byte Mismatch Error (Single Byte)

**Condition:** `Offset >= 0 && len(Expected) == 1`

**VerificationError State:**
```go
VerificationError{
    Offset: 512,
    Expected: []byte{0x03},
    Actual: []byte{0x00},
    ContextBytes: 0
}
```

**Message Format:**
```
"verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00)"
```

**JSON Output:**
```json
{
  "offset": 512,
  "expected": "0x03",
  "actual": "0x00",
  "error_type": "byte_mismatch",
  "severity": "high"
}
```

### 3. Byte Mismatch Error (With Context)

**Condition:** `Offset >= 0 && ContextBytes > 0`

**VerificationError State:**
```go
VerificationError{
    Offset: 512,
    Expected: []byte{0x01, 0x02, 0x03, 0x04},
    Actual: []byte{0x01, 0x02, 0x00, 0x04},
    ContextBytes: 1,
    ContextBefore: 2,
    ContextAfter: 1
}
```

**Message Format:**
```
"verification failed: byte mismatch at offset 512 (expected 4-byte context, got 4-byte context) [1 bytes context: 2 before, 1 after]"
```

**JSON Output:**
```json
{
  "offset": 512,
  "expected": "0x01 0x02 0x03 0x04",
  "actual": "0x01 0x02 0x00 0x04",
  "context_bytes": 1,
  "context_before": 2,
  "context_after": 1,
  "error_type": "byte_mismatch",
  "severity": "medium"
}
```

### 4. No Error (Success Case)

**Condition:** `Offset == -1`

**VerificationError State:**
```go
VerificationError{
    Offset: -1,
    Expected: nil,
    Actual: nil
}
```

**Note:** This case is typically represented by `VerifyResult.Passed = true` rather than by an error object.

## Special Offset Value Semantics

The `Offset` field uses special negative values to encode error conditions:

| Offset Value | Meaning | Message Format |
|-------------|---------|----------------|
| `-1` | No error (content matches) | Not used in error messages (success case) |
| `-2` | Length mismatch | "length mismatch (got X bytes, expected Y bytes)" |
| `< -2` | Reserved for future error types | "invalid offset X" |

## Context Field Calculation Logic

When context is requested (`ContextBytes > 0`), the actual context included is calculated as follows:

```go
// Calculate actual context before error offset
ContextBefore = min(ContextBytes, Offset)

// Calculate actual context after error offset
remainingBytes = len(data) - Offset - 1
ContextAfter = min(ContextBytes, remainingBytes)

// Total context size
totalContextSize = ContextBefore + 1 + ContextAfter
```

**Examples:**

| Scenario | ContextBytes | Offset | Data Length | ContextBefore | ContextAfter | Total |
|----------|-------------|--------|-------------|---------------|--------------|-------|
| Near start | 16 | 5 | 1024 | 5 | 16 | 22 |
| Near end | 16 | 1020 | 1024 | 16 | 3 | 20 |
| Middle | 16 | 512 | 1024 | 16 | 16 | 33 |
| No context | 0 | 512 | 1024 | 0 | 0 | 1 |

## Edge Cases and Special Handling

### 1. Nil Expected/Actual Arrays

**Condition:** `len(Expected) == 0 || len(Actual) == 0`

**Handling:**
- In `Error()`: Returns `"<nil>"` string representation
- In JSON serialization: Returns empty array `[]`
- Indicates out-of-range offset or missing data

### 2. Offset Exceeds Data Length

**Condition:** `Offset >= len(Expected) || Offset >= len(Actual)`

**Handling:**
- Treated as "out of range" error
- Returns `"verification failed: invalid offset X"`
- Prevents panic from array index out of bounds

### 3. Empty Data Slices

**Condition:** `len(Expected) == 0 && len(Actual) == 0`

**Handling:**
- Considered a match (both empty)
- Offset set to -1 (success)
- Returns `"verified: 0 bytes match exactly"`

### 4. Context Window Larger Than Data

**Condition:** `ContextBytes > len(data)`

**Handling:**
- ContextBefore and ContextAfter are capped at available data
- Example: 100 bytes context requested for 50-byte data
  - ContextBefore = Offset (if near start)
  - ContextAfter = remaining bytes (if near end)

## Integration with VerifyResult

The `VerificationError` is embedded in the `VerifyResult` type:

```go
type VerifyResult struct {
    Pass       bool               // true if verification succeeded
    Diagnostic string            // human-readable message
    Error      *VerificationError // structured error details
}
```

**Conversion Flow:**
1. Verification function detects mismatch
2. Creates `VerificationError` with offset and bytes
3. Wraps in `VerifyResult` with `Pass = false`
4. Sets `Diagnostic` using `VerificationError.Error()` method
5. Returns complete result to caller

**Usage Pattern:**
```go
result := crypto.VerifyDecompression(decompressed, expected)
if !result.Pass {
    // result.Error contains structured details
    // result.Diagnostic contains human-readable message
    log.Printf("Verification failed: %s", result.Diagnostic)

    // Programmatic access to error details
    if result.Error.IsLengthMismatch() {
        log.Printf("Length error: got %d, expected %d",
            result.Error.ActualLength, result.Error.ExpectedLength)
    }
}
```

## Severity Classification

The `VerificationError.MismatchSeverity()` method maps error characteristics to severity levels:

| Method | Condition | Severity | Rationale |
|--------|-----------|----------|-----------|
| `IsLengthMismatch()` | `Offset == -2` | "critical" | Data truncation or extension |
| `IsByteMismatch()` | `Offset >= 0` | Based on offset | Header corruption = high, data area = medium |
| `IsOutOfRange()` | `Offset < -2` | "unknown" | Reserved error codes |

**Severity Logic:**
```go
func (ve *VerificationError) MismatchSeverity() string {
    switch {
    case ve.Offset == -2:
        return "critical"  // Length mismatch
    case ve.Offset < -2:
        return "unknown"   // Reserved codes
    case ve.Offset < 256:
        return "high"      // Header/metadata corruption
    default:
        return "medium"    // Data area corruption
    }
}
```

## Implementation Examples

### Example 1: Creating VerificationError from mismatch

```go
// Detect mismatch at offset 512
offset := int64(512)
decompressed := []byte{0x01, 0x02, 0x00, 0x04}
expected := []byte{0x01, 0x02, 0x03, 0x04}

// Create error with single byte context
verr := &VerificationError{
    Offset:        offset,
    Expected:      []byte{expected[offset]},
    Actual:        []byte{decompressed[offset]},
    ContextBytes:  0,  // No surrounding context
    ContextBefore: 0,
    ContextAfter:  0,
    ExpectedLength: len(expected),
    ActualLength:  len(decompressed),
}

// Convert to message
message := verr.Error()
// Output: "verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00)"
```

### Example 2: Creating VerificationError with context

```go
// Create error with 16-byte context
contextSize := 16
before := min(contextSize, offset)
after := min(contextSize, len(expected)-offset-1)

verr := &VerificationError{
    Offset:        offset,
    Expected:      expected[offset-before : offset+after+1],
    Actual:        decompressed[offset-before : offset+after+1],
    ContextBytes:  contextSize,
    ContextBefore: before,
    ContextAfter:  after,
    ExpectedLength: len(expected),
    ActualLength:  len(decompressed),
}

// Convert to message
message := verr.Error()
// Output: "verification failed: byte mismatch at offset 512 (expected 33-byte context, got 33-byte context) [16 bytes context: 16 before, 16 after]"
```

### Example 3: Length mismatch error

```go
verr := &VerificationError{
    Offset:         -2,
    ExpectedLength: 1024,
    ActualLength:   997,
    Expected:       expected,  // Full expected data
    Actual:        decompressed,  // Full actual data
}

// Convert to message
message := verr.Error()
// Output: "verification failed: length mismatch (got 997 bytes, expected 1024 bytes)"
```

## Testing Requirements

Tests for VerificationError mapping must validate:

1. **Core field accuracy**: Offset, Expected, Actual match the mismatch location
2. **Special offset values**: -1, -2, and other negative values handled correctly
3. **Context calculation**: ContextBefore/ContextAfter match data boundaries
4. **Message format**: Output string matches expected format for each error type
5. **JSON serialization**: Structured output includes all required fields
6. **Edge cases**: Nil arrays, offset out of range, empty data handled gracefully

**Example test:**
```go
func TestVerificationErrorMapping(t *testing.T) {
    // Test byte mismatch
    verr := &VerificationError{
        Offset:        2,
        Expected:     []byte{0x03},
        Actual:       []byte{0x00},
        ContextBytes: 0,
        ExpectedLength: 4,
        ActualLength: 4,
    }

    msg := verr.Error()
    expected := "verification failed: byte mismatch at offset 2 (expected 0x03, got 0x00)"
    if msg != expected {
        t.Errorf("Message mismatch:\n got: %s\nwant: %s", msg, expected)
    }

    // Test length mismatch
    verr.Length = &VerificationError{
        Offset:         -2,
        ExpectedLength: 1024,
        ActualLength:   997,
    }

    msg = verr.Error()
    expected = "verification failed: length mismatch (got 997 bytes, expected 1024 bytes)"
    if msg != expected {
        t.Errorf("Length mismatch message error:\n got: %s\nwant: %s", msg, expected)
    }
}
```

## References

- **Core type definition**: `internal/crypto/verify_decompress.go`
- **Message format specification**: `docs/error-message-structure.md`
- **Optional context fields**: `docs/verification-error-optional-context.md`
- **Verification pipeline**: `internal/restoreverifier/verifier.go`
- **ADR-004**: Restore Verification Pipeline design document

## Version History

- **v1.0** (2026-08-14): Initial mapping documentation
- Covers VerificationError type to message format conversion
- Documents all field mappings and conversion logic
- Includes examples and testing requirements