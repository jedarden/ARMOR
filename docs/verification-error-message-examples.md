# Verification Error Message Examples

## Overview

This document provides concrete error message examples for typical verification failure scenarios in ARMOR. Each example demonstrates both required and optional fields with explanatory comments.

These examples serve as:
- **Reference documentation** for understanding error message structure
- **Test fixtures** for verification error handling
- **Integration templates** for implementing error reporting

## Core Structure Reminder

All verification errors include three required fields:

```json
{
  "offset": "<int64>",      // Required: Byte position of first difference
  "expected": "<byte[]>",   // Required: Expected byte(s) at offset
  "actual": "<byte[]>"      // Required: Actual byte(s) found
}
```

Optional fields may be added to provide additional diagnostic context.

---

## Example 1: Single Byte Mismatch

### Scenario

A single byte at position 512 has been corrupted. Expected byte `0x03` was replaced with `0x00` (null byte overwrite).

### Minimal Message (Core Fields Only)

```json
{
  "offset": 512,
  "expected": "0x03",
  "actual": "0x00"
}
```

**Interpretation**: At byte position 512, the decompressed data contains `0x00` but should contain `0x03`.

### Complete Message (With Optional Context)

```json
{
  "offset": 512,
  "expected": "0x03",
  "actual": "0x00",
  "error_type": "byte_mismatch",
  "severity": "high",
  "surrounding_bytes": {
    "before": "0x01 0x02 0xFF 0xFF",
    "after": "0x04 0x05 0x06 0x07"
  },
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "databases/primary/snapshot-2026-08-14.db"
  },
  "checksum_type": "byte_compare"
}
```

**Explanatory Comments**:
- **`offset: 512`**: The corruption occurred exactly 512 bytes from the start
- **`expected: "0x03"` vs `actual: "0x00"`**: Single byte corruption - common pattern of null byte overwrites
- **`error_type: "byte_mismatch"`**: Standard single-byte corruption classification
- **`severity: "high"`**: Position 512 is in the header/metadata region (offset < 256 = high severity)
- **`surrounding_bytes.before`**: Shows pattern `0xFF 0xFF` before the error - may indicate buffer boundary issues
- **`surrounding_bytes.after`**: Normal data sequence after the error
- **`object_identifier`**: Correlates error with specific storage object for investigation
- **`checksum_type: "byte_compare"`**: Verification used direct byte comparison (not hash-based)

### Human-Readable Format

```
verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00)
```

### Go Code to Generate This Error

```go
// Single byte mismatch detected at offset 512
verr := &crypto.VerificationError{
    Offset:        512,
    Expected:      []byte{0x03},
    Actual:        []byte{0x00},
    ContextBytes:  4,  // Request 4 bytes context
    ContextBefore: 4,  // 4 bytes available before offset
    ContextAfter:  4,  // 4 bytes available after offset
    ExpectedLength: 1024,
    ActualLength:  1024,
}

// Convert to message
message := verr.Error()
// Output: "verification failed: byte mismatch at offset 512 (expected 0x03, got 0x00) [4 bytes context: 4 before, 4 after]"
```

---

## Example 2: Multi-Byte Mismatch

### Scenario

A corruption event affects a contiguous run of 3 bytes starting at offset 2048. This could indicate a storage system bug that corrupted an entire block.

### Minimal Message (Core Fields Only)

```json
{
  "offset": 2048,
  "expected": "0xAA 0xBB 0xCC",
  "actual": "0x00 0x00 0x00"
}
```

**Interpretation**: Starting at byte 2048, three consecutive bytes are corrupted to null values.

### Complete Message (With Optional Context)

```json
{
  "offset": 2048,
  "expected": "0xAA 0xBB 0xCC",
  "actual": "0x00 0x00 0x00",
  "error_type": "byte_mismatch",
  "severity": "medium",
  "surrounding_bytes": {
    "before": "0x01 0x02 0x03 0x04 0x05 0x06 0x07 0x08",
    "after": "0xDD 0xEE 0xFF 0x11 0x22 0x33 0x44 0x55"
  },
  "context_bytes": 8,
  "context_before": 8,
  "context_after": 8,
  "expected_length": 4096,
  "actual_length": 4096,
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "logs/app/access-2026-08-14.log.gz"
  },
  "checksum_type": "byte_compare"
}
```

**Explanatory Comments**:
- **`offset: 2048`**: First byte of the corrupted run - all subsequent bytes in the run differ from expected
- **`expected: "0xAA 0xBB 0xCC"` vs `actual: "0x00 0x00 0x00"`**: Multi-byte null overwrite pattern - suggests storage block corruption
- **`error_type: "byte_mismatch"`**: Multi-byte corruption is still classified as byte mismatch (offset points to first difference)
- **`severity: "medium"`**: Offset 2048 is in the data region (offset >= 256 = medium severity)
- **`surrounding_bytes`**: 8 bytes before/after show normal data - confirms corruption is localized to the 3-byte run
- **`context_bytes: 8`**: Context window of 8 bytes was requested to capture surrounding pattern
- **`context_before: 8` vs `context_after: 8`**: Full context available on both sides of the error
- **`expected_length: 4096` vs `actual_length: 4096`**: Lengths match - this is not a truncation error

### Human-Readable Format

```
verification failed: byte mismatch at offset 2048 (expected 0xAA 0xBB 0xCC, got 0x00 0x00 0x00) [8 bytes context: 8 before, 8 after]
```

### Go Code to Generate This Error

```go
// Multi-byte mismatch with context
contextSize := 16
before := min(contextSize/2, 2048)  // 8 bytes before
after := min(contextSize/2, 4096-2048-3)  // 8 bytes after

verr := &crypto.VerificationError{
    Offset:        2048,
    Expected:      []byte{0xAA, 0xBB, 0xCC},
    Actual:        []byte{0x00, 0x00, 0x00},
    ContextBytes:  contextSize,
    ContextBefore: before,
    ContextAfter:  after,
    ExpectedLength: 4096,
    ActualLength:  4096,
}

message := verr.Error()
// Output includes context information showing the corruption is localized
```

---

## Example 3: Offset Mismatch in Range Request

### Scenario

During an HTTP range request for bytes 4096-8191, a corruption is detected at absolute offset 4608 (512 bytes into the requested range). The error must correlate the absolute offset with the range context.

### Minimal Message (Core Fields Only)

```json
{
  "offset": 4608,
  "expected": "0x77",
  "actual": "0x88"
}
```

**Interpretation**: At absolute byte position 4608 in the object, the data contains `0x88` instead of `0x77`.

### Complete Message (With Range Context)

```json
{
  "offset": 4608,
  "expected": "0x77",
  "actual": "0x88",
  "error_type": "byte_mismatch",
  "severity": "medium",
  "range_info": {
    "start": 4096,
    "end": 8191,
    "length": 4096
  },
  "surrounding_bytes": {
    "before": "0x11 0x22 0x33 0x44 0x55 0x66 0x12 0x34",
    "after": "0x99 0xAA 0xBB 0xCC 0xDD 0xEE 0xFF 0x00"
  },
  "context_bytes": 8,
  "context_before": 8,
  "context_after": 8,
  "expected_length": 4096,
  "actual_length": 4096,
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "videos/recording-2026-08-14.mp4"
  },
  "checksum_type": "byte_compare"
}
```

**Explanatory Comments**:
- **`offset: 4608`**: Absolute byte position in the full object (not relative to range)
- **`range_info.start: 4096` vs `range_info.end: 8191`**: The HTTP range request was for `Range: bytes=4096-8191`
- **`range_info.length: 4096`**: Total requested range size (8192 - 4096 = 4096 bytes)
- **Correlation**: The error at absolute offset 4608 is 512 bytes into the range (4608 - 4096 = 512)
- **`expected: "0x77"` vs `actual: "0x88"`**: Single bit difference (0x77 = 0111 0111, 0x88 = 1000 1000) - suggests bit-flip corruption
- **`surrounding_bytes`**: Context is relative to the absolute offset, not the range start
- **`checksum_type: "byte_compare"`**: Range requests use byte-by-byte verification (hash-based not feasible for partial data)

### HTTP Request Context

```
GET /videos/recording-2026-08-14.mp4 HTTP/1.1
Host: armor-backups.s3.amazonaws.com
Range: bytes=4096-8191
```

The error indicates that byte 512 of the requested range (absolute offset 4608) is corrupted.

### Human-Readable Format

```
verification failed: byte mismatch at offset 4608 (expected 0x77, got 0x88) [range: 4096-8191, 8 bytes context: 8 before, 8 after]
```

### Go Code to Generate This Error

```go
// Range request verification
rangeStart := int64(4096)
rangeEnd := int64(8191)
rangeLength := rangeEnd - rangeStart + 1  // 4096 bytes

// Error detected at absolute offset 4608 (512 bytes into the range)
absoluteOffset := int64(4608)

verr := &crypto.VerificationError{
    Offset:        absoluteOffset,
    Expected:      []byte{0x77},
    Actual:        []byte{0x88},
    ContextBytes:  16,
    ContextBefore: 8,
    ContextAfter:  8,
    ExpectedLength: int(rangeLength),
    ActualLength:  int(rangeLength),
}

// For range requests, add range_info context
// (Implementation would wrap this in a structure that includes range_info)
message := verr.Error()
```

---

## Example 4: Entire Object Mismatch (Length Mismatch)

### Scenario

The decompressed output is 27 bytes shorter than expected. This indicates truncation during decompression or storage corruption.

### Minimal Message (Core Fields Only)

```json
{
  "offset": -2,
  "expected": "0x01 0x02 0x03 ...",
  "actual": "0x01 0x02 0x03 ..."
}
```

**Interpretation**: The special offset value `-2` indicates a length mismatch, not a byte mismatch. The `expected_length` and `actual_length` fields show the size difference.

### Complete Message (With Optional Context)

```json
{
  "offset": -2,
  "expected": "0x01 0x02 0x03 0x04 0x05 0x06 0x07 0x08",
  "actual": "0x01 0x02 0x03 0x04 0x05 0x06 0x07",
  "error_type": "length_mismatch",
  "severity": "critical",
  "expected_length": 1024,
  "actual_length": 997,
  "surrounding_bytes": {
    "before": "0x01 0x02 0x03 0x04 0x05 0x06 0x07",
    "after": []
  },
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "documents/contract-2026-08-14.pdf"
  },
  "checksum_type": "sha256"
}
```

**Explanatory Comments**:
- **`offset: -2`**: Special value indicating length mismatch (not a byte position)
- **`expected_length: 1024` vs `actual_length: 997`**: Decompressed output is 27 bytes too short
- **`error_type: "length_mismatch"`**: Classification for size mismatch errors
- **`severity: "critical"`**: Length mismatches are critical - data is truncated or extended
- **`expected`**: Full expected data (or truncated representation for display)
- **`actual`**: Full actual data (27 bytes shorter than expected)
- **`surrounding_bytes.before`**: Contains the full actual data (since it's shorter)
- **`surrounding_bytes.after`**: Empty array (no bytes after the truncated end)
- **`checksum_type: "sha256"`**: Hash-based verification detected the length mismatch

### Human-Readable Format

```
verification failed: length mismatch (got 997 bytes, expected 1024 bytes)
```

### Go Code to Generate This Error

```go
// Length mismatch detected
verr := &crypto.VerificationError{
    Offset:         -2,  // Special code for length mismatch
    ExpectedLength: 1024,
    ActualLength:   997,
    Expected:       expectedData,     // Full expected data
    Actual:         decompressedData, // Full actual data (truncated)
    ContextBytes:   0,                // No context for length errors
    ContextBefore:  0,
    ContextAfter:   0,
}

message := verr.Error()
// Output: "verification failed: length mismatch (got 997 bytes, expected 1024 bytes)"
```

---

## Example 5: Bit-Flip Error (Single Bit Difference)

### Scenario

A single bit has flipped at position 1024. Expected `0x55` (binary: 01010101) but received `0x54` (binary: 01010100). This pattern is typical of memory or storage bit-flip errors.

### Minimal Message (Core Fields Only)

```json
{
  "offset": 1024,
  "expected": "0x55",
  "actual": "0x54"
}
```

**Interpretation**: At byte 1024, only the least significant bit differs (01010101 vs 01010100).

### Complete Message (With Pattern Analysis)

```json
{
  "offset": 1024,
  "expected": "0x55",
  "actual": "0x54",
  "error_type": "byte_mismatch",
  "severity": "medium",
  "surrounding_bytes": {
    "before": "0x55 0x55 0x55 0x55 0x55 0x55 0x55 0x55",
    "after": "0x55 0x55 0x56 0x57 0x58 0x59 0x5A 0x5B"
  },
  "context_bytes": 8,
  "context_before": 8,
  "context_after": 8,
  "expected_length": 2048,
  "actual_length": 2048,
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "images/photo-2026-08-14.cr2"
  },
  "checksum_type": "byte_compare"
}
```

**Explanatory Comments**:
- **`offset: 1024`**: Bit-flip occurs in the data region (offset >= 256 = medium severity)
- **`expected: "0x55"` vs `actual: "0x54"`**: Hamming distance of 1 bit - classic bit-flip signature
  - `0x55` = `0101 0101` (binary)
  - `0x54` = `0101 0100` (binary)
  - Only the least significant bit differs (bit 0)
- **`surrounding_bytes.before`**: Repeated `0x55` pattern suggests structured data or padding
- **`surrounding_bytes.after`**: Incrementing byte sequence (`0x56 0x57 0x58...`) confirms normal data resume
- **Pattern analysis**: Single-bit errors are characteristic of:
  - DRAM/correctable ECC errors
  - Storage media bit-rot
  - Transmission errors (if data traveled over network)

### Human-Readable Format

```
verification failed: byte mismatch at offset 1024 (expected 0x55, got 0x54) [1-bit difference]
```

### Binary Representation

```
Position: 1024
Expected: 0x55 = 0101 0101
Actual:   0x54 = 0101 0100
                      ^^^^^^
                      1-bit difference (LSB)
```

---

## Example 6: Pattern Corruption (Repeated Null Bytes)

### Scenario

A region of repeated null bytes (`0x00`) has overwritten valid data starting at offset 1536. This could indicate buffer overflows, uninitialized memory, or zero-filled padding errors.

### Minimal Message (Core Fields Only)

```json
{
  "offset": 1536,
  "expected": "0xAB 0xCD 0xEF",
  "actual": "0x00 0x00 0x00"
}
```

**Interpretation**: Bytes 1536-1538 should contain data but are null.

### Complete Message (With Pattern Context)

```json
{
  "offset": 1536,
  "expected": "0xAB 0xCD 0xEF",
  "actual": "0x00 0x00 0x00",
  "error_type": "byte_mismatch",
  "severity": "medium",
  "surrounding_bytes": {
    "before": "0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00",
    "after": "0x00 0x00 0x00 0x00 0x12 0x34 0x56 0x78"
  },
  "context_bytes": 16,
  "context_before": 8,
  "context_after": 8,
  "expected_length": 3072,
  "actual_length": 3072,
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "structs/user-profile-2026-08-14.bin"
  },
  "checksum_type": "byte_compare"
}
```

**Explanatory Comments**:
- **`offset: 1536`**: First non-null byte in what should be data
- **`surrounding_bytes.before`**: 8 consecutive null bytes before the error - pattern started earlier
- **`surrounding_bytes.after`**: 4 more null bytes after the error, then valid data resumes at `0x12`
- **Pattern analysis**: Suggests:
  - **Buffer overflow**: Code wrote 12 null bytes (before + error bytes) into this region
  - **Uninitialized memory**: Region was never initialized and zero-filled
  - **Padding corruption**: Structure padding or alignment bytes were incorrectly zeroed
- **Investigation clue**: The run of null bytes is 12 bytes long (8 before + 3 error + 1 after before valid data)

### Human-Readable Format

```
verification failed: byte mismatch at offset 1536 (expected 0xAB 0xCD 0xEF, got 0x00 0x00 0x00) [12-byte null run detected]
```

---

## Example 7: Header Corruption (High Severity)

### Scenario

Critical metadata in the first 256 bytes is corrupted. This is high-severity because header corruption often makes the entire object unusable.

### Minimal Message (Core Fields Only)

```json
{
  "offset": 64,
  "expected": "0x89 0x50 0x4E 0x47",
  "actual": "0x89 0x50 0x4E 0x47"
}
```

**Interpretation**: Actually this shows a match - here's the real error:

```json
{
  "offset": 16,
  "expected": "0x0D 0x0A 0x1A 0x0A",
  "actual": "0x00 0x00 0x1A 0x0A"
}
```

**Interpretation**: PNG signature bytes at offset 16 are corrupted - the file header is damaged.

### Complete Message (High Severity)

```json
{
  "offset": 16,
  "expected": "0x0D 0x0A 0x1A 0x0A",
  "actual": "0x00 0x00 0x1A 0x0A",
  "error_type": "byte_mismatch",
  "severity": "high",
  "surrounding_bytes": {
    "before": "0x89 0x50 0x4E 0x47 0x0D 0x0A 0x1A 0x0A",
    "after": "0x00 0x00 0x00 0x49 0x48 0x44 0x52"
  },
  "context_bytes": 8,
  "context_before": 8,
  "context_after": 8,
  "expected_length": 102400,
  "actual_length": 102400,
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "images/screenshot-2026-08-14.png"
  },
  "checksum_type": "byte_compare"
}
```

**Explanatory Comments**:
- **`offset: 16`**: Error is within the first 256 bytes → high severity
- **`expected: "0x0D 0x0A 0x1A 0x0A"`**: PNG signature bytes (standard magic number)
- **`actual: "0x00 0x00 0x1A 0x0A"`**: First two bytes corrupted to null - signature damaged
- **`severity: "high"`**: Header corruption often renders entire file unreadable
- **`surrounding_bytes.before`**: Shows correct PNG magic number start (`0x89 0x50 0x4E 0x47` = "PNG")
- **`surrounding_bytes.after`**: Shows IHDR chunk start (`0x00 0x00 0x00 0x49 0x48 0x44 0x52` = "IHDR")
- **Impact**: This PNG file may fail to open in image viewers due to damaged signature

### Human-Readable Format

```
verification failed: byte mismatch at offset 16 (expected 0x0D 0x0A 0x1A 0x0A, got 0x00 0x00 0x1A 0x0A) [HEADER CORRUPTION - file signature damaged]
```

---

## Example 8: Out of Range Offset

### Scenario

Verification attempts to access an offset beyond the data bounds. This indicates a programming error or corrupted length field.

### Minimal Message (Core Fields Only)

```json
{
  "offset": -3,
  "expected": [],
  "actual": []
}
```

**Interpretation**: Special offset value `-3` indicates an out-of-range error (reserved for future use, currently treated as invalid offset).

### Complete Message (Error Details)

```json
{
  "offset": -3,
  "expected": [],
  "actual": [],
  "error_type": "out_of_range",
  "severity": "unknown",
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "unknown/corrupted-file.bin"
  },
  "checksum_type": "byte_compare"
}
```

**Explanatory Comments**:
- **`offset: -3`**: Reserved value indicating out-of-range or invalid offset
- **`expected: []` vs `actual: []`**: Empty arrays - no byte data available
- **`error_type: "out_of_range"`**: Offset was beyond data bounds or negative
- **`severity: "unknown"`**: Reserved offsets have unknown severity until defined
- **Possible causes**:
  - Corrupted length field in header
  - Integer overflow during offset calculation
  - Verification code bug
  - Mismatch between expected and actual data sizes

### Human-Readable Format

```
verification failed: invalid offset -3 (out of range or unknown error code)
```

---

## Summary Table: Error Scenarios

| Scenario | Offset | Expected | Actual | Severity | Pattern |
|----------|--------|----------|--------|----------|---------|
| Single byte mismatch | 512 | `0x03` | `0x00` | high | Null byte overwrite |
| Multi-byte mismatch | 2048 | `0xAA 0xBB 0xCC` | `0x00 0x00 0x00` | medium | Block corruption |
| Range request offset | 4608 | `0x77` | `0x88` | medium | Bit-flip in range |
| Length mismatch | -2 | 1024 bytes | 997 bytes | critical | Truncation |
| Single bit-flip | 1024 | `0x55` | `0x54` | medium | 1-bit difference |
| Pattern corruption | 1536 | `0xAB 0xCD 0xEF` | `0x00 0x00 0x00` | medium | Null run |
| Header corruption | 16 | `0x0D 0x0A 0x1A 0x0A` | `0x00 0x00 0x1A 0x0A` | high | Signature damage |
| Out of range | -3 | (empty) | (empty) | unknown | Invalid offset |

---

## Usage as Test Fixtures

These examples can be used directly in tests:

```go
func TestVerificationErrorExamples(t *testing.T) {
    // Example 1: Single byte mismatch
    t.Run("single_byte_mismatch", func(t *testing.T) {
        decompressed := []byte{0x01, 0x02, 0x00, 0x04}
        expected := []byte{0x01, 0x02, 0x03, 0x04}

        result := crypto.VerifyDecompression(decompressed, expected)

        if result.Offset != 2 {
            t.Errorf("Expected offset 2, got %d", result.Offset)
        }
        if !bytes.Equal(result.Expected, []byte{0x03}) {
            t.Errorf("Expected [0x03], got %v", result.Expected)
        }
        if !bytes.Equal(result.Actual, []byte{0x00}) {
            t.Errorf("Expected [0x00], got %v", result.Actual)
        }
    })

    // Example 4: Length mismatch
    t.Run("length_mismatch", func(t *testing.T) {
        decompressed := []byte{0x01, 0x02, 0x03}
        expected := []byte{0x01, 0x02, 0x03, 0x04}

        result := crypto.VerifyDecompression(decompressed, expected)

        if result.Offset != -2 {
            t.Errorf("Expected offset -2 (length mismatch), got %d", result.Offset)
        }
        if result.ExpectedLength != 4 {
            t.Errorf("Expected length 4, got %d", result.ExpectedLength)
        }
        if result.ActualLength != 3 {
            t.Errorf("Expected actual length 3, got %d", result.ActualLength)
        }
    })

    // Example 5: Bit-flip
    t.Run("bit_flip", func(t *testing.T) {
        decompressed := []byte{0x54}  // 01010100
        expected := []byte{0x55}      // 01010101

        result := crypto.VerifyDecompression(decompressed, expected)

        if result.Offset != 0 {
            t.Errorf("Expected offset 0, got %d", result.Offset)
        }
        // Verify single-bit difference
        expectedBits := expected[0]
        actualBits := decompressed[0]
        xor := expectedBits ^ actualBits
        if xor != 1 {  // Only 1 bit should differ
            t.Errorf("Expected 1-bit difference, got Hamming distance %d", bits.OnesCount8(xor))
        }
    })
}
```

---

## JSON Schema Validation

All examples conform to this JSON schema:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["offset", "expected", "actual"],
  "properties": {
    "offset": {
      "type": "integer",
      "description": "Byte position of first difference (-2 = length mismatch, -1 = no error)"
    },
    "expected": {
      "type": "string",
      "description": "Expected byte(s) at offset (hexadecimal string)"
    },
    "actual": {
      "type": "string",
      "description": "Actual byte(s) found at offset (hexadecimal string)"
    },
    "error_type": {
      "type": "string",
      "enum": ["byte_mismatch", "length_mismatch", "out_of_range"],
      "description": "Error classification"
    },
    "severity": {
      "type": "string",
      "enum": ["critical", "high", "medium", "low", "unknown"],
      "description": "Error severity level"
    },
    "surrounding_bytes": {
      "type": "object",
      "properties": {
        "before": {"type": "string"},
        "after": {"type": "string"}
      }
    },
    "range_info": {
      "type": "object",
      "properties": {
        "start": {"type": "integer"},
        "end": {"type": "integer"},
        "length": {"type": "integer"}
      }
    },
    "object_identifier": {
      "type": "object",
      "properties": {
        "bucket": {"type": "string"},
        "key": {"type": "string"},
        "version_id": {"type": "string"}
      }
    },
    "checksum_type": {
      "type": "string",
      "enum": ["sha256", "crc32", "byte_compare", "adler32", "custom"]
    }
  }
}
```

---

## Version History

- **v1.0** (2026-08-14): Initial error message examples
  - Single byte mismatch
  - Multi-byte mismatch
  - Range request offset mismatch
  - Length mismatch (entire object)
  - Bit-flip error
  - Pattern corruption
  - Header corruption
  - Out of range offset

## References

- Core error message structure: `error-message-structure.md`
- Optional context fields: `verification-error-optional-context.md`
- Error-to-message mapping: `verification-error-to-message-mapping.md`
- ADR-004: Restore Verification Pipeline
- `internal/crypto/verify_decompress.go`: Verification implementation
- `internal/restoreverifier/verifier.go`: Restore verifier integration
