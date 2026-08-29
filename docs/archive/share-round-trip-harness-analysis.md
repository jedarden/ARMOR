# Share Round-Trip Test Harness Analysis

## Overview

The share round-trip test harness validates the complete data flow for the share endpoint: **compress → encrypt → store → GET → decrypt → decompress → original**.

**Important Clarification:** The share endpoint (`/share/<token>`) only handles **GET operations**. The task description mentioned PUT/DELETE, but those methods are not supported on the share endpoint - the handler explicitly returns HTTP 405 "Method not allowed" for non-GET requests (see `internal/server/server.go:971-974`).

## Current Implementation

### Test Files
- **`internal/server/share_decompress_test.go`** - Comprehensive GET testing (1,330+ lines)
- **`internal/server/share_decompress_corrupted_test.go`** - Corrupted data handling (253 lines)

### Test Coverage

The current harness tests GET operations across these dimensions:

1. **Compression States**
   - Compressed objects (zstd) → verify decompression
   - Uncompressed objects → verify passthrough
   - Legacy objects (no compression flag) → verify backward compatibility

2. **Object Sizes**
   - Empty objects (0 bytes)
   - Single-byte objects
   - Small objects (< 4 bytes with compression flag)
   - Medium objects (50KB - 512KB)
   - Large objects (multi-block encryption)

3. **Data Types**
   - Text data (ASCII, multilingual UTF-8)
   - Binary data (random bytes)
   - Structured data (JSON, log lines)
   - Repetitive data (high compression ratio)
   - Edge cases (zstd magic byte prefixes)

4. **Error Handling**
   - Corrupted compressed data (various corruption patterns)
   - Truncated streams
   - Invalid tokens (expired, bad signature)
   - Range requests on compressed objects (416 error)

5. **Range Requests**
   - Uncompressed objects → 206 Partial Content
   - Compressed objects → 416 Range Not Satisfiable (compression breaks seeking)

## Data Structures & Patterns

### Test Harness Components

```go
// Test setup helpers
setupTestEnvironment(t)        → Creates temp dir with cleanup
loadTestConfig(t, tmpDir)      → Loads test config with env vars

// Encryption pipeline helpers
encryptTestData(t, srv, data, compressed) → Returns (encrypted, hmacTable, armorMeta)
storeTestObject(t, backend, ctx, bucket, key, encryptedData, hmacTable, armorMeta) → Stores full envelope

// Token generation
generateTestToken(t, srv, bucket, key, expiration) → Returns presigned token

// Data preparation
compressData(data) → zstd compression
generateRandomData(size) → Random bytes

// HTTP simulation
httptest.NewRequest("GET", "/share/"+token, nil) → Creates request
httptest.NewRecorder() → Captures response
srv.handleShare(w, req) → Executes handler
```

### Round-Trip Test Pattern

```go
// 1. Prepare data
originalData := []byte("test data")
if compressed {
    dataToEncrypt = compressData(originalData)
} else {
    dataToEncrypt = originalData
}

// 2. Encrypt
encryptedData, hmacTable, armorMeta := encryptTestData(t, srv, dataToEncrypt, compressed)

// 3. Store
storeTestObject(t, srv.backend, ctx, "test-bucket", "test-key", encryptedData, hmacTable, armorMeta)

// 4. Generate token
token := generateTestToken(t, srv, "test-bucket", "test-key", time.Hour)

// 5. GET request
req := httptest.NewRequest("GET", "/share/"+token, nil)
w := httptest.NewRecorder()
srv.handleShare(w, req)

// 6. Verify round-trip
resp := w.Result()
retrievedData, _ := io.ReadAll(resp.Body)
assert bytes.Equal(retrievedData, originalData) // Complete round-trip
```

## Extension Points for Additional GET Testing

### 1. Table-Driven Test Expansion

**Location:** `TestShareGET_CompressionBehavior` (line 179-391)

Add new test cases to the existing table:

```go
{
    name:              "custom_scenario",
    data:              []byte("..."),
    compressed:        true,
    wantDecompression: true,
    description:       "description of what this tests",
},
```

**Examples of scenarios to add:**
- Specific content types (JSON, XML, binary formats)
- Unicode edge cases (invalid UTF-8, mixed encodings)
- Large objects (> 1MB)
- Objects at encryption block boundaries

### 2. New Test Functions

**Pattern:** Follow the existing function signature pattern:

```go
func TestShareGET_YourScenario(t *testing.T) {
    tmpDir, cleanup := setupTestEnvironment(t)
    defer cleanup()

    fsBackend, _ := backend.NewFSBackend(backend.FSConfig{BasePath: tmpDir})
    cfg := loadTestConfig(t, tmpDir)
    srv, _ := NewWithBackend(cfg, fsBackend)
    srv.presigner = presign.NewSigner(cfg.PresignSecret, "")

    // ... test-specific setup ...

    token := generateTestToken(t, srv, "test-bucket", "test-key", time.Hour)
    req := httptest.NewRequest("GET", "/share/"+token, nil)
    w := httptest.NewRecorder()
    srv.handleShare(w, req)

    resp := w.Result()
    // ... assertions ...
}
```

**Examples of new test functions:**
- `TestShareGET_ContentDispositionHeader` → Verify Content-Disposition handling
- `TestShareGET_TokenExpirationEdgeCases` → Test token expiration boundary conditions
- `TestShareGET_ConcurrentRequests` → Test concurrent access to same object
- `TestShareGET_MemoryEfficiency` → Test large object streaming

### 3. Error Case Expansion

**Location:** `share_decompress_corrupted_test.go`

Add new corruption scenarios to the table in `TestShareGET_CorruptedCompressedData`:

```go
{
    name: "corruption_pattern_name",
    corruptFunc: func(data []byte) []byte {
        // Return corrupted version of data
        corrupted := make([]byte, len(data))
        copy(corrupted, data)
        // Apply specific corruption pattern
        corrupted[someIndex] ^= 0xFF
        return corrupted
    },
    description: "what this corruption pattern tests",
},
```

### 4. Header and Metadata Testing

The current harness tests basic GET operations but could be extended to test:

- Content-Type header propagation
- Content-Disposition filename encoding
- ETag and cache headers
- Custom metadata passthrough
- CORS headers

### 5. Performance and Stress Testing

Add tests for:
- Large object streaming (avoid loading full response in memory)
- Memory usage during decompression
- Decompression speed benchmarks
- Concurrent request handling

## Implementation Notes

### Key Functions

**Handler Entry Point:**
- `handleShare(w, r)` - Main handler, verifies token and routes to full/range handlers
- `handleShareFullObject(...)` - Full object download with decompression
- `handleShareRangeRequest(...)` - Range request handling (416 for compressed objects)

**Encryption/Decryption:**
- `crypto.NewEncryptor(dek, iv, blockSize)` - Creates encryptor
- `crypto.NewDecryptor(dek, iv, blockSize)` - Creates decryptor
- `decryptor.DecryptBlock(block, hmac)` - Decrypts single block
- `crypto.Decompress(data)` - Decompresses zstd data

**Backend Operations:**
- `backend.Put(ctx, bucket, key, body, size, meta)` - Store object
- `backend.Get(ctx, bucket, key)` - Retrieve object
- `backend.GetRange(ctx, bucket, key, offset, size)` - Range read
- `backend.Head(ctx, bucket, key)` - Get metadata

### Important Constraints

1. **Compression destroys range reads:** Range requests on compressed objects return 416 (not satisfiable) because zstd is variable-length encoding.
2. **Metadata-driven decompression:** The `Compressed` flag in metadata controls decompression - content sniffing is NOT used (critical for backward compatibility).
3. **Legacy object compatibility:** Objects without compression flag must bypass decompression entirely, even if they start with zstd magic bytes.
4. **Block-based encryption:** Objects are encrypted in 64KB blocks, each with HMAC. Decryptor must reassemble blocks.

## Summary

The current round-trip test harness is comprehensive for GET operations and provides a solid foundation for expansion. The main extension points are:

1. **Add test cases to existing table-driven tests** (easiest)
2. **Create new test functions following the established pattern**
3. **Add corruption/error scenarios** to corrupted data tests
4. **Test additional HTTP behaviors** (headers, metadata, edge cases)

The harness uses filesystem backend for test isolation, `httptest` for HTTP simulation, and manual encryption pipeline setup to test the complete round-trip flow end-to-end.
