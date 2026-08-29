# Range Request Simulation Helpers

This guide demonstrates how to use ARMOR's range request simulation helpers for testing HTTP range requests on objects.

## Overview

The range request simulation helpers in `internal/crypto/range_testutil.go` provide utilities for:

- Parsing HTTP Range headers (e.g., `bytes=0-1023`, `bytes=512-`, `bytes=-500`)
- Extracting byte ranges from test data
- Simulating complete range requests with proper headers
- Working with both compressed and uncompressed objects
- Generating common range specifications for comprehensive testing

## Basic Usage

### 1. Parse a Range Header

```go
import "github.com/jedarden/armor/internal/crypto"

// Parse a standard range request
spec, err := crypto.ParseRangeSpec("bytes=0-1023", 2048)
if err != nil {
    // Handle error
}
// spec.Start = 0, spec.End = 1023

// Parse an open-ended range (from offset to end)
spec, err := crypto.ParseRangeSpec("bytes=512-", 2048)
if err != nil {
    // Handle error
}
// spec.Start = 512, spec.End = -1 (indicates open-ended)

// Parse a suffix range (last N bytes)
spec, err := crypto.ParseRangeSpec("bytes=-500", 2048)
if err != nil {
    // Handle error
}
// spec.Start = 1548, spec.End = 2047
```

### 2. Extract Range Data

```go
data := []byte("Hello, World! This is test data for range requests.")

// Create a range spec
spec := &crypto.RangeSpec{Start: 0, End: 12}

// Extract the range
partialData, err := crypto.ExtractRange(data, spec)
if err != nil {
    // Handle error
}
// partialData = []byte("Hello, World!")
```

### 3. Simulate Range Requests

```go
// Create test data
testData := []byte("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ")

// Create a range simulator
simulator := crypto.NewRangeSimulator(testData, false, 65536)

// Simulate a range request
result, err := simulator.SimulateRangeRequest("bytes=0-9")
if err != nil {
    // Handle error
}

// Access the result
fmt.Printf("Data: %s\n", string(result.Data))           // "0123456789"
fmt.Printf("Content-Range: %s\n", result.ContentRange)  // "bytes 0-9/36"
fmt.Printf("Content-Length: %d\n", result.ContentLength) // 10
fmt.Printf("Total Size: %d\n", result.TotalSize)        // 36

// Verify the result matches expected data
err = result.Verify([]byte("0123456789"))
if err != nil {
    // Verification failed
}
```

### 4. Generate Common Range Specifications

```go
dataSize := int64(len(testData))

// Get common range specs for testing
specs := crypto.CommonRangeSpecs(dataSize)

// Test each spec
for _, rangeHeader := range specs {
    result, err := simulator.SimulateRangeRequest(rangeHeader)
    if err != nil {
        t.Errorf("Range %s failed: %v", rangeHeader, err)
    }
    // Validate result...
}
```

### 5. Parse Content-Range Headers

```go
// Parse a Content-Range response header
start, end, total, err := crypto.ParseContentRange("bytes 0-1023/2048")
if err != nil {
    // Handle error
}
// start = 0, end = 1023, total = 2048
```

## Range Specification Formats

The helpers support the following HTTP Range header formats:

| Format | Example | Description |
|--------|---------|-------------|
| Standard range | `bytes=0-1023` | Bytes from position 0 to 1023 (inclusive) |
| Open-ended | `bytes=512-` | From position 512 to end of file |
| Suffix range | `bytes=-500` | Last 500 bytes of file |
| Single byte | `bytes=0-0` | Just the first byte |
| Middle range | `bytes=100-200` | Bytes from position 100 to 200 |

## Testing with Compressed Objects

```go
// Simulate range requests on compressed data
compressedData := []byte{0x28, 0xB5, 0x2F, 0xFD, 0x01, 0x00, 0x00, 0x00}
compressedData = append(compressedData, []byte("payload...")...)

// Create simulator with compressed flag
simulator := crypto.NewRangeSimulator(compressedData, true, 65536)

// Range requests work the same way
result, err := simulator.SimulateRangeRequest("bytes=0-7")
if err != nil {
    // Handle error
}
// simulator.compressed == true
```

## Integration with ARMOR Backend

These helpers work seamlessly with ARMOR's backend mock for testing:

```go
import "github.com/jedarden/armor/internal/backend"

// Create test data and encrypt it
plaintext := []byte("Test data for range requests")
encrypted, hmacTable, err := encryptor.Encrypt(plaintext)

// Store in mock backend
mockBackend.Put(ctx, "test-bucket", "test-key", encrypted, ...)

// Test range requests
for _, rangeHeader := range crypto.CommonRangeSpecs(int64(len(plaintext))) {
    spec, _ := crypto.ParseRangeSpec(rangeHeader, int64(len(plaintext)))
    start, end := spec.ResolveRange(int64(len(plaintext)))

    // Use ARMOR's range translation
    translation, err := crypto.TranslateRange(
        start, end, int64(len(plaintext)),
        blockSize, crypto.HeaderSize,
    )

    // Fetch range from backend
    data, err := mockBackend.GetRange(ctx, "test-bucket", "test-key",
        translation.DataOffset, translation.DataLength)

    // Verify decryption of range...
}
```

## Error Handling

The helpers provide clear error messages for invalid range specifications:

```go
// Invalid format
_, err := crypto.ParseRangeSpec("invalid", 100)
// Error: "invalid range format: must start with 'bytes='"

// Multiple ranges (not supported)
_, err := crypto.ParseRangeSpec("bytes=0-1023,2048-3071", 4096)
// Error: "multiple ranges not supported"

// Start beyond file size
_, err := crypto.ParseRangeSpec("bytes=2000-2999", 1000)
// Error: "start offset 2000 exceeds file size 1000"

// End before start
_, err := crypto.ParseRangeSpec("bytes=500-400", 1000)
// Error: "end 400 is before start 500"
```

## Testing Best Practices

1. **Test Multiple Range Types**: Always test standard, open-ended, and suffix ranges
2. **Edge Cases**: Include single-byte ranges and boundary conditions
3. **Error Cases**: Verify proper error handling for invalid ranges
4. **Large Files**: Test with files larger than block sizes
5. **Compressed Data**: Test range requests on both compressed and uncompressed data

## Example Test Suite

```go
func TestObjectRangeRequests(t *testing.T) {
    testData := []byte("Test data for range request simulation")
    simulator := crypto.NewRangeSimulator(testData, false, 65536)

    tests := []struct {
        name         string
        rangeHeader  string
        expectedData []byte
        expectError  bool
    }{
        {"first 10 bytes", "bytes=0-9", []byte("Test data "), false},
        {"middle range", "bytes=5-14", []byte("data for ra"), false},
        {"open-ended", "bytes=20-", []byte("nge simulation"), false},
        {"suffix range", "bytes=-10", []byte("simulation"), false},
        {"invalid range", "bytes=invalid", nil, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := simulator.SimulateRangeRequest(tt.rangeHeader)

            if tt.expectError {
                if err == nil {
                    t.Errorf("expected error but got none")
                }
                return
            }

            if err != nil {
                t.Fatalf("unexpected error: %v", err)
            }

            if !bytes.Equal(result.Data, tt.expectedData) {
                t.Errorf("data mismatch: got %s, want %s",
                    string(result.Data), string(tt.expectedData))
            }

            // Verify Content-Length matches
            if result.ContentLength != int64(len(tt.expectedData)) {
                t.Errorf("Content-Length mismatch: got %d, want %d",
                    result.ContentLength, len(tt.expectedData))
            }
        })
    }
}
```

## API Reference

### Types

- **`RangeSpec`**: Represents a parsed HTTP Range specification
  - `Start int64`: Start byte offset (inclusive)
  - `End int64`: End byte offset (inclusive), or -1 for open-ended ranges

- **`RangeSimulator`**: Simulates range requests on test data
  - `data []byte`: The test data
  - `totalSize int64`: Total size of the data
  - `compressed bool`: Whether data is compressed
  - `blockSize int`: Block size for encrypted objects

- **`RangeResult`**: Result of a range request simulation
  - `Spec *RangeSpec`: The range specification used
  - `Data []byte`: The extracted partial content
  - `ContentRange string`: HTTP Content-Range header value
  - `ContentLength int64`: HTTP Content-Length header value
  - `TotalSize int64`: Total size of the original data

### Functions

- **`ParseRangeSpec(header string, totalSize int64) (*RangeSpec, error)`**: Parse HTTP Range header
- **`ExtractRange(data []byte, spec *RangeSpec) ([]byte, error)`**: Extract byte range from data
- **`NewRangeSimulator(data []byte, compressed bool, blockSize int) *RangeSimulator`**: Create simulator
- **`(*RangeSimulator) SimulateRangeRequest(rangeHeader string) (*RangeResult, error)`**: Simulate range request
- **`CommonRangeSpecs(dataSize int64) []string`**: Generate common range specifications
- **`ParseContentRange(header string) (start, end, total int64, err error)`**: Parse Content-Range header

### Methods

- **`(*RangeSpec) String() string`**: Convert range spec to string format
- **`(*RangeSpec) ContentRange(totalSize int64) string`**: Generate Content-Range header
- **`(*RangeSpec) Length(totalSize int64) int64`**: Calculate range length in bytes
- **`(*RangeSpec) ResolveRange(totalSize int64) (start, end int64)`**: Resolve open-ended ranges
- **`(*RangeResult) Verify(expectedData []byte) error`**: Verify result matches expected data

## Conclusion

These range request simulation helpers provide a comprehensive toolkit for testing HTTP range request functionality in ARMOR. They handle all common range specification formats, work with both compressed and uncompressed data, and integrate seamlessly with ARMOR's existing testing infrastructure.
