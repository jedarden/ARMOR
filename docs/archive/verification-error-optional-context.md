# Optional Context Fields for Verification Errors

## Overview

This document defines the optional context fields for verification error messages. These fields provide additional debugging and diagnostic information beyond the core required fields (offset, expected, actual) defined in `error-message-structure.md`.

Optional fields are **not required** for basic error reporting but **must be included** when the relevant context is available during verification. They help operators and automated systems understand the full circumstances of a verification failure.

## Core Structure Reminder

All verification errors start with the three required fields:

```json
{
  "offset": "<int64>",
  "expected": "<byte[]>",
  "actual": "<byte[]>"
}
```

Optional fields are added to this core structure as needed.

## Optional Context Fields

### 1. surrounding_bytes

**Type**: `object`  
**Required**: No (but recommended when available)

Bytes before and after the mismatch location to provide context for debugging. This helps identify corruption patterns (e.g., repeated null bytes, bit-flip regions).

#### Structure

```json
{
  "surrounding_bytes": {
    "before": "<byte[]>",    // Bytes immediately preceding the error offset
    "after": "<byte[]>"      // Bytes immediately following the error offset
  }
}
```

#### When to Include

- **Always include** when verification operates on a buffer containing more than just the mismatch location
- **Include** when the verification function has access to the full decompressed buffer
- **Omit** only when operating on streaming data without historical context

#### Guidelines

- **Recommended window size**: 4-16 bytes before/after for single-byte errors
- **Balanced context**: Equal bytes before and after when possible
- **Full buffer for length errors**: When offset is -2 (length mismatch), `before` contains the full actual data and `after` is empty

#### Examples

**Single-byte error with context:**

```json
{
  "offset": 512,
  "expected": [0x03],
  "actual": [0x00],
  "surrounding_bytes": {
    "before": [0x01, 0x02, 0xFF, 0xFF],
    "after": [0x04, 0x05, 0x06, 0x07]
  }
}
```

**Length mismatch with surrounding context:**

```json
{
  "offset": -2,
  "expected": [0x01, 0x02, 0x03, 0x04],
  "actual": [0x01, 0x02, 0x03],
  "surrounding_bytes": {
    "before": [0x01, 0x02, 0x03],
    "after": []
  }
}
```

**Pattern identification (repeated null overwrites):**

```json
{
  "offset": 1024,
  "expected": [0x55],
  "actual": [0x00],
  "surrounding_bytes": {
    "before": [0x00, 0x00, 0x00, 0x00],
    "after": [0x00, 0x00, 0x56, 0x57]
  }
}
```

### 2. range_info

**Type**: `object`  
**Required**: No

Details about the range request when the verification occurred during a partial read or chunked verification. This field is critical for correlating verification failures with specific HTTP range requests.

#### Structure

```json
{
  "range_info": {
    "start": "<int64>",     // First byte position of the range
    "end": "<int64>",       // Last byte position of the range (inclusive)
    "length": "<int64>"     // Total bytes in the range
  }
}
```

#### When to Include

- **Always include** when verification occurs within a range request context
- **Include** when the restore verifier processes objects in chunks
- **Include** for partial reads that don't cover the entire object

#### Guidelines

- **Zero-based indexing**: `start` is 0-indexed from the beginning of the object
- **Inclusive end**: `end` is the last byte position (not exclusive)
- **Consistent with HTTP Range**: Values should match the HTTP `Range: bytes=start-end` header

#### Examples

**Single-range chunk verification:**

```json
{
  "offset": 1024,
  "expected": [0xAB],
  "actual": [0xCD],
  "range_info": {
    "start": 0,
    "end": 4095,
    "length": 4096
  }
}
```

**Multi-chunk verification (failure in second chunk):**

```json
{
  "offset": 512,
  "expected": [0x03],
  "actual": [0x00],
  "range_info": {
    "start": 4096,
    "end": 8191,
    "length": 4096
  }
}
```

**Range request correlation:**

```
HTTP Request: Range: bytes=4096-8191
Error occurs at absolute offset: 4608 (512 bytes into the range)
range_info.start: 4096
range_info.end: 8191
offset: 4608
```

### 3. object_identifier

**Type**: `object`  
**Required**: No (but strongly recommended)

Identifies which object was being verified. This is essential for correlation with storage systems and logs.

#### Structure

```json
{
  "object_identifier": {
    "bucket": "<string>",    // Storage bucket name
    "key": "<string>",        // Object key/path
    "version_id": "<string>"  // Optional: Object version (if versioning enabled)
  }
}
```

#### When to Include

- **Always include** when verification is performed on a storage object (S3, GCS, etc.)
- **Include** when the restore verifier processes keys from a queue or manifest
- **Omit** only for verification tests on synthetic data

#### Guidelines

- **bucket**: Use the bucket name exactly as stored (no escaping)
- **key**: Preserve the full key path including prefixes
- **version_id**: Include only when the storage system uses versioning

#### Examples

**Standard S3 object:**

```json
{
  "offset": 512,
  "expected": [0x03],
  "actual": [0x00],
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "databases/primary/snapshot-2026-08-14.db"
  }
}
```

**Versioned object:**

```json
{
  "offset": 2048,
  "expected": [0xAA],
  "actual": [0xBB],
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "databases/primary/snapshot-2026-08-14.db",
    "version_id": "abcd1234efgh5678"
  }
}
```

### 4. checksum_type

**Type**: `string`  
**Required**: No (but recommended for multi-algorithm systems)

Identifies the verification algorithm or checksum type used. This field is important when verification supports multiple algorithms (e.g., SHA256 vs. CRC32).

#### Structure

```json
{
  "checksum_type": "<string>"
}
```

#### Supported Values

| Value | Description | Use Case |
|-------|-------------|----------|
| `"sha256"` | SHA-256 hash verification | Full-object integrity |
| `"crc32"` | CRC32 checksum | Fast integrity checks |
| `"byte_compare"` | Direct byte comparison | Range request verification |
| `"adler32"` | Adler-32 checksum | zlib compression streams |
| `"custom"` | Custom algorithm | Domain-specific verification |

#### When to Include

- **Always include** when the verification system supports multiple checksum types
- **Include** when different algorithms are used for different object types
- **Include** for backward compatibility when algorithm choices may change

#### Guidelines

- **Explicit is better**: Use the exact algorithm name, not generic terms
- **Case-sensitive**: Use lowercase (e.g., `"sha256"`, not `"SHA256"`)
- **Future-proof**: New algorithm types should be added to this document

#### Examples

**SHA-256 verification:**

```json
{
  "offset": -1,
  "expected": [0x00],
  "actual": [0x00],
  "checksum_type": "sha256"
}
```

**Range request byte comparison:**

```json
{
  "offset": 1024,
  "expected": [0x55],
  "actual": [0x54],
  "checksum_type": "byte_compare",
  "range_info": {
    "start": 0,
    "end": 4095,
    "length": 4096
  }
}
```

## Integration with Core Message Structure

### Field Combination Rules

1. **Core fields are mandatory**: All verification errors must include `offset`, `expected`, and `actual`
2. **Optional fields are additive**: Any subset of optional fields may be included
3. **No dependencies**: Optional fields do not depend on each other (except `range_info` should be consistent with any `surrounding_bytes`)
4. **Extensibility**: New optional fields may be added without breaking existing consumers

### Complete Example (All Optional Fields)

```json
{
  "offset": 512,
  "expected": [0x03],
  "actual": [0x00],
  "surrounding_bytes": {
    "before": [0x01, 0x02, 0xFF, 0xFF],
    "after": [0x04, 0x05, 0x06, 0x07]
  },
  "range_info": {
    "start": 0,
    "end": 4095,
    "length": 4096
  },
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "databases/primary/snapshot-2026-08-14.db"
  },
  "checksum_type": "byte_compare"
}
```

### Minimal Example (Core Only)

```json
{
  "offset": 512,
  "expected": [0x03],
  "actual": [0x00]
}
```

### Selective Inclusion Examples

**With surrounding_bytes only:**

```json
{
  "offset": 1024,
  "expected": [0x55],
  "actual": [0x54],
  "surrounding_bytes": {
    "before": [0x00, 0x00, 0x00, 0x00],
    "after": [0x00, 0x00, 0x56, 0x57]
  }
}
```

**With object_identifier only:**

```json
{
  "offset": -2,
  "expected": [0x01, 0x02, 0x03, 0x04],
  "actual": [0x01, 0x02, 0x03],
  "object_identifier": {
    "bucket": "armor-backups",
    "key": "databases/primary/snapshot-2026-08-14.db"
  }
}
```

**With range_info + checksum_type:**

```json
{
  "offset": 2048,
  "expected": [0xAA],
  "actual": [0xBB],
  "range_info": {
    "start": 0,
    "end": 8191,
    "length": 8192
  },
  "checksum_type": "byte_compare"
}
```

## Go Type Definition

```go
// OptionalContextFields provides additional debugging context for verification errors.
// All fields are optional and should be populated only when the relevant context is available.
type OptionalContextFields struct {
    // SurroundingBytes provides context around the error location.
    // Helps identify corruption patterns (e.g., repeated null bytes).
    SurroundingBytes *SurroundingBytes `json:"surrounding_bytes,omitempty"`

    // RangeInfo describes the range request context.
    // Critical for correlating failures with specific HTTP range requests.
    RangeInfo *RangeInfo `json:"range_info,omitempty"`

    // ObjectIdentifier identifies which storage object was being verified.
    // Essential for correlation with storage systems and logs.
    ObjectIdentifier *ObjectIdentifier `json:"object_identifier,omitempty"`

    // ChecksumType identifies the verification algorithm used.
    // Important when multiple checksum types are supported.
    ChecksumType string `json:"checksum_type,omitempty"`
}

// SurroundingBytes contains bytes before and after the error offset.
type SurroundingBytes struct {
    Before []byte `json:"before"` // Bytes immediately preceding the error
    After  []byte `json:"after"`  // Bytes immediately following the error
}

// RangeInfo describes the range request context.
type RangeInfo struct {
    Start  int64 `json:"start"`  // First byte position of the range (0-indexed)
    End    int64 `json:"end"`    // Last byte position of the range (inclusive)
    Length int64 `json:"length"` // Total bytes in the range
}

// ObjectIdentifier identifies a storage object.
type ObjectIdentifier struct {
    Bucket    string `json:"bucket"`              // Storage bucket name
    Key       string `json:"key"`                 // Object key/path
    VersionID string `json:"version_id,omitempty"` // Object version (if versioning enabled)
}

// FullVerificationError combines core and optional fields.
type FullVerificationError struct {
    Offset    int64                `json:"offset"`              // Required: error offset
    Expected  []byte               `json:"expected"`            // Required: expected bytes
    Actual    []byte               `json:"actual"`              // Required: actual bytes
    Context   *OptionalContextFields `json:"context,omitempty"` // Optional: additional context
}
```

## Implementation Guidelines

### When to Populate Each Field

| Field | Populate When... | Example Scenario |
|-------|----------------|------------------|
| `surrounding_bytes.before` | Buffer available before offset | Decompressing a chunk in memory |
| `surrounding_bytes.after` | Buffer available after offset | Non-streaming verification |
| `range_info` | Range request active | Verifying an HTTP range response |
| `object_identifier` | Storage object being verified | Restore verifier processing S3 objects |
| `checksum_type` | Multiple algorithms supported | System supports both SHA256 and CRC32 |

### Performance Considerations

- **surrounding_bytes**: Avoid copying large buffers; use references/slices when possible
- **range_info**: Minimal overhead (three int64 fields)
- **object_identifier**: Strings are small; always include when available
- **checksum_type**: Single string field; negligible overhead

### Backward Compatibility

- **Consumers must ignore unknown optional fields**: New fields may be added in the future
- **Core fields are version-stable**: `offset`, `expected`, `actual` will not change
- **Serialization format**: JSON field names are stable; Go struct tags use `omitempty`

## Testing Requirements

Tests for optional context fields must verify:

1. **Field presence**: Optional fields are included when context is available
2. **Field absence**: Optional fields are omitted when context is unavailable
3. **Data correctness**: Field values match the verification context
4. **Serialization**: JSON serialization preserves optional field data
5. **Deserialization**: Consumers can parse messages with any subset of optional fields

### Example Test

```go
func TestOptionalContextFields(t *testing.T) {
    decompressed := []byte{0x01, 0x02, 0x00, 0x04}
    expected := []byte{0x01, 0x02, 0x03, 0x04}
    
    result := VerifyDecompressionWithContext(
        decompressed,
        expected,
        &OptionalContextFields{
            SurroundingBytes: &SurroundingBytes{
                Before: []byte{0xFF, 0xFF},
                After:  []byte{0x05, 0x06},
            },
            ObjectIdentifier: &ObjectIdentifier{
                Bucket: "test-bucket",
                Key:    "test-key",
            },
        },
    )
    
    // Verify core fields
    if result.Offset != 2 {
        t.Errorf("Expected offset 2, got %d", result.Offset)
    }
    
    // Verify optional fields
    if result.Context == nil {
        t.Fatal("Expected context to be populated")
    }
    if result.Context.SurroundingBytes == nil {
        t.Error("Expected surrounding_bytes to be populated")
    }
    if result.Context.ObjectIdentifier == nil {
        t.Error("Expected object_identifier to be populated")
    }
    
    // Verify JSON serialization
    data, err := json.Marshal(result)
    if err != nil {
        t.Fatalf("Failed to marshal: %v", err)
    }
    
    var parsed FullVerificationError
    if err := json.Unmarshal(data, &parsed); err != nil {
        t.Fatalf("Failed to unmarshal: %v", err)
    }
    
    // Verify deserialized optional fields
    if parsed.Context.SurroundingBytes.Before[0] != 0xFF {
        t.Error("Surrounding bytes not preserved")
    }
    if parsed.Context.ObjectIdentifier.Bucket != "test-bucket" {
        t.Error("Object identifier not preserved")
    }
}
```

## Version History

- **v1.0** (2026-08-14): Initial optional context fields definition
  - surrounding_bytes
  - range_info
  - object_identifier
  - checksum_type

## References

- Core error message structure: `error-message-structure.md`
- ADR-004: Restore Verification Pipeline
- `internal/crypto/verify_decompress.go`: Core verification implementation
- `internal/restoreverifier/verifier.go`: Restore verifier integration
