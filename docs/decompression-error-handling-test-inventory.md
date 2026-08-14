# Decompression and Error-Handling Test Inventory

## Overview
This document inventories all decompression, error-handling, and edge-case tests added for ARMOR's compressed object handling. These tests validate the complete lifecycle of compressed objects from storage through retrieval and error scenarios.

## Test Files

### 1. `internal/server/share_decompress_test.go`
**Purpose:** Main test suite for compressed object GET operations via share endpoints  
**Dependencies:** 
- `internal/server` package
- `internal/backend` for filesystem backend
- `internal/presign` for token generation
- `github.com/klauspost/compress/zstd` for compression

### 2. `internal/server/share_decompress_corrupted_test.go`
**Purpose:** Tests for corrupted compressed data error handling  
**Dependencies:** Same as share_decompress_test.go

### 3. `internal/server/handlers/handlers_test.go`
**Purpose:** Handler-level tests (may contain some edge case tests)  
**Note:** This file is part of the broader handler test suite

---

## Test Functions by Category

### A. Core Decompression Tests

#### 1. `TestShareGET_CompressedObject` (share_decompress_test.go:23)
- **Purpose:** Validates that compressed objects are properly decompressed during GET
- **Coverage:** 
  - Compresses test data using zstd
  - Encrypts compressed data
  - Stores in backend
  - Retrieves via share endpoint
  - Verifies decompressed data matches original
- **Data:** Simple text string ("Hello, ARMOR! This is compressed test data.")
- **Expected Behavior:** HTTP 200, decompressed content equals original

#### 2. `TestShareGET_UncompressedObject` (share_decompress_test.go:116)
- **Purpose:** Ensures uncompressed objects bypass decompression logic
- **Coverage:**
  - Stores data without compression
  - Verifies data is served unchanged
  - Confirms decompression is skipped
- **Data:** Text string ("Hello, ARMOR! This is uncompressed test data.")
- **Expected Behavior:** HTTP 200, content matches original exactly

#### 3. `TestShareGET_LegacyObject` (share_decompress_test.go:393)
- **Purpose:** Tests backward compatibility with objects created before compression feature
- **Coverage:**
  - Objects without `Compressed` metadata flag
  - Ensures they're served without decompression attempts
- **Data:** "Hello, ARMOR! This is legacy test data."
- **Expected Behavior:** HTTP 200, legacy objects work unchanged

---

### B. Comprehensive Compression Behavior Tests

#### 4. `TestShareGET_CompressionBehavior` (share_decompress_test.go:177)
- **Purpose:** Table-driven test covering all compression scenarios
- **Test Cases (13 total):**
  1. **uncompressed_small_text** - Small text without compression
  2. **uncompressed_binary_data** - 50KB binary without compression
  3. **compressed_repeating_text** - Highly compressible repeating text
  4. **compressed_binary_data** - 100KB random binary data
  5. **compressed_highly_repetitive_large** - ~250KB highly repetitive data
  6. **compressed_structured_json** - Repeated JSON records (~21KB)
  7. **compressed_repeated_log_lines** - Timestamped log entries (~250KB)
  8. **compressed_unicode_multilingual** - UTF-8 with emoji, CJK, Cyrillic, Greek (~150KB)
  9. **legacy_object_no_compression_flag** - Legacy object without flag
  10. **legacy_object_zstd_magic_prefix_no_flag** - Legacy object with zstd magic bytes but no flag (critical for backward compatibility)
  11. **legacy_object_multiblock_no_flag** - Legacy object spanning multiple 64KB encryption blocks (~136KB)
- **Coverage Areas:**
  - Various data patterns (text, binary, JSON, logs)
  - Multiple sizes (<64KB, ~64KB, >64KB, >100KB, >250KB)
  - Unicode/multilingual content
  - Legacy backward compatibility
  - Multi-block encryption boundary cases
- **Expected Behavior:** Each case validates correct decompression or bypass based on metadata

---

### C. Size and Boundary Edge Cases

#### 5. `TestShareGET_EmptyObject` (share_decompress_test.go:841)
- **Purpose:** Tests 0-byte (empty) objects
- **Coverage:**
  - Empty plaintext (0 bytes)
  - Verifies PlaintextSize metadata is 0
  - Ensures no panic or error on empty data
- **Data:** Empty byte array `[]byte("")`
- **Expected Behavior:** HTTP 200, 0-byte response body

#### 6. `TestShareGET_SingleByteObject` (share_decompress_test.go:912)
- **Purpose:** Tests single-byte objects with various byte values
- **Test Cases (4):**
  1. **single_byte_zero** - Value 0x00
  2. **single_byte_a** - ASCII 'a'
  3. **single_byte_ff** - Value 0xFF
  4. **single_byte_space** - Space character
- **Coverage:**
  - Single-byte plaintext size
  - Different byte values (0x00, 0x20, 0x61, 0xFF)
  - Verifies exact byte preservation
- **Expected Behavior:** HTTP 200, exact single byte returned

#### 7. `TestShareGET_SmallObjectsCompressedFlag` (share_decompress_test.go:1018)
- **Purpose:** Tests objects < 4 bytes with Compressed=true flag
- **Test Cases (4):**
  1. **empty_compressed_flag** - 0 bytes
  2. **one_byte_compressed_flag** - 1 byte
  3. **two_bytes_compressed_flag** - 2 bytes
  4. **three_bytes_compressed_flag** - 3 bytes
- **Coverage:**
  - Exercises Decompress len<4 early-return path
  - Verifies data is returned unchanged for <4-byte compressed objects
  - Tests Compressed=true flag edge cases
- **Expected Behavior:** HTTP 200, data returned unchanged (early return)

#### 8. `TestSmallCompressedObjectGet` (share_decompress_test.go:1138)
- **Purpose:** Specific test for 3-byte compressed object
- **Coverage:**
  - Exercises crypto/decryptor.go:310-312 Decompress early-return
  - Verifies no panic or error from early-return path
  - Tests exact byte preservation
- **Data:** 3-byte "ABC"
- **Expected Behavior:** HTTP 200, 3 bytes unchanged, confirms early-return works

---

### D. Corruption and Error Handling Tests

#### 9. `TestShareGET_CorruptedCompressedData` (share_decompress_corrupted_test.go:17)
- **Purpose:** Validates graceful error handling for corrupted compressed data
- **Test Cases (4 corruption types):**
  1. **corrupted_content_after_magic** - Magic bytes intact, frame content corrupted
  2. **truncated_stream** - Valid magic bytes but stream cut off mid-frame
  3. **only_magic_bytes** - Only zstd magic bytes (0x28 0xB5 0x2F 0xFD), no frame content
  4. **partial_frame** - Magic bytes + partial frame (8 bytes)
- **Coverage:**
  - No panic - server handles errors gracefully
  - Returns HTTP 500 InternalServerError
  - Response body contains "Failed to decompress data" message
  - Error message mentions "zstd" or "decompression failed"
- **Expected Behavior:** HTTP 500 with appropriate error message, no server crash

---

### E. Advanced Compression Tests

#### 10. `TestShareGET_RoundTrip` (share_decompress_test.go:456)
- **Purpose:** Validates complete round-trip: compress → encrypt → store → GET → decompress
- **Test Cases (3 sizes):**
  1. **small_text** - Small text
  2. **medium_binary** - 100KB binary
  3. **large_binary** - 512KB binary
- **Coverage:**
  - Full pipeline validation
  - Compression ratio verification
  - Data integrity after complete cycle
- **Expected Behavior:** Original data retrieved exactly after round-trip

#### 11. `TestShareGET_CompressionDetectionFromFirstBlock` (share_decompress_test.go:547)
- **Purpose:** Tests that compression detection works from first decrypted block
- **Coverage:**
  - Verifies zstd magic bytes (0x28 0xB5 0x2F 0xFD) are detected
  - Confirms `crypto.IsCompressed()` helper function
  - Tests detection → decompression flow
- **Data:** Repeating pattern "Repeat data pattern for compression..."
- **Expected Behavior:** Magic bytes detected, successful decompression

#### 12. `TestShareGET_RangeRequestWithCompression` (share_decompress_test.go:610)
- **Purpose:** Validates range requests work correctly with compressed data
- **Coverage:**
  - Requests first 10KB of 200KB compressed object
  - Verifies HTTP 206 Partial Content or 200 OK
- **Data:** 200KB random binary data
- **Expected Behavior:** Partial content status or full content with correct range

---

## Shared Test Fixtures and Helpers

### Helper Functions (share_decompress_test.go)

#### `setupTestEnvironment(t)` (Line 662)
- **Purpose:** Creates temporary directory for test file storage
- **Returns:** tmpDir path and cleanup function
- **Cleanup:** Removes temp directory automatically

#### `loadTestConfig(t, tmpDir)` (Line 678)
- **Purpose:** Loads test configuration with required environment variables
- **Env Vars Set:**
  - ARMOR_B2_REGION, ARMOR_B2_ENDPOINT
  - ARMOR_B2_ACCESS_KEY_ID, ARMOR_B2_SECRET_ACCESS_KEY
  - ARMOR_BUCKET, ARMOR_MEK, ARMOR_PRESIGN_SECRET
  - ARMOR_SECONDARY_BACKEND_TYPE, ARMOR_SECONDARY_BACKEND_PATH
- **Cleanup:** Unsets all env vars after test

#### `encryptTestData(t, srv, data, compressed)` (Line 714)
- **Purpose:** Encrypts test data with full ARMOR encryption pipeline
- **Process:**
  1. Gets MEK from key manager
  2. Generates DEK and IV
  3. Creates encryptor with 64KB block size
  4. Encrypts data with HMAC table generation
  5. Wraps DEK with MEK
  6. Creates ARMORMetadata with all fields
- **Returns:** Encrypted data, HMAC table, ARMORMetadata

#### `storeTestObject(t, backend, ctx, bucket, key, encryptedData, hmacTable, armorMeta)` (Line 771)
- **Purpose:** Stores encrypted test object in backend with proper envelope format
- **Envelope Structure:** header + encrypted data + HMAC table
- **Metadata Conversion:** Uses `ToMetadata()` to convert ARMORMetadata to S3 metadata format

#### `generateTestToken(t, srv, bucket, key, expiration)` (Line 808)
- **Purpose:** Generates presigned share token for testing
- **Uses:** srv.presigner.GenerateToken()

#### `compressData(data)` (Line 820)
- **Purpose:** Compresses data using zstd encoder
- **Library:** github.com/klauspost/compress/zstd
- **Returns:** Compressed byte slice

#### `generateRandomData(size)` (Line 832)
- **Purpose:** Generates cryptographically random test data
- **Uses:** crypto/rand.Read()
- **Returns:** Random byte slice of specified size

---

## Test Data Patterns

### Data Sizes Tested
- **0 bytes** (empty)
- **1 byte** (single byte)
- **2-3 bytes** (<4 byte threshold)
- **Small text** (~50-100 bytes)
- **4KB** (one memory page)
- **Just under 64KB** (63KB - 64KB)
- **Exactly 64KB** (one encryption block)
- **Just over 64KB** (64KB - 65KB)
- **100KB** (medium binary)
- **128KB** (two blocks)
- **136KB** (multi-block legacy)
- **200KB** (range test)
- **250KB** (highly repetitive)
- **512KB** (large binary)

### Data Types Tested
- **Text:** ASCII, UTF-8, repeating patterns
- **Binary:** Random data, various byte values (0x00-0xFF)
- **Structured:** JSON records with repeated fields
- **Log data:** Timestamped log entries
- **Multilingual:** Emoji (4-byte UTF-8), CJK, Cyrillic, Greek
- **Edge cases:** Empty, single-byte, magic byte prefixes

### Compression Scenarios
- **Highly compressible:** Repeating text patterns, log entries, JSON
- **Poorly compressible:** Random binary data
- **Incompressible:** Small data, random data
- **Legacy compatibility:** Data with zstd magic but no compression flag

---

## Expected Test Coverage Areas

### ✅ Covered

1. **Corruption Handling**
   - Corrupted frame content
   - Truncated streams
   - Magic bytes only
   - Partial frames
   - Graceful error responses (HTTP 500)

2. **Empty/Small Objects**
   - 0-byte objects
   - Single-byte objects
   - 2-3 byte objects (<4 byte threshold)
   - All byte values (0x00, 0x20, 0x61, 0xFF)

3. **Decompression Paths**
   - Compressed objects → decompression
   - Uncompressed objects → bypass
   - Legacy objects → bypass
   - Small compressed (<4 bytes) → early return unchanged

4. **Size Boundaries**
   - Empty (0 bytes)
   - Single byte (1 byte)
   - <4 byte threshold (1-3 bytes)
   - Block boundaries (64KB, 128KB)
   - Various sizes up to 512KB

5. **Backward Compatibility**
   - Legacy objects without compression flag
   - Legacy objects with zstd magic prefix but no flag
   - Legacy multi-block objects

6. **Data Types**
   - Text (ASCII, UTF-8, multilingual)
   - Binary (random, various byte values)
   - Structured (JSON)
   - Logs (timestamped entries)
   - Highly repetitive (good compression ratio)

7. **Operations**
   - GET with decompression
   - Range requests with compression
   - Round-trip (compress → store → GET → decompress)
   - Share token generation and validation

8. **Error Responses**
   - HTTP 500 for corruption
   - Appropriate error messages
   - No server crashes/panics

### 📊 Test Statistics

- **Total Test Functions:** 12
- **Total Test Cases (including table-driven):** ~35+
- **Files:** 3 (share_decompress_test.go, share_decompress_corrupted_test.go, handlers_test.go)
- **Helper Functions:** 7
- **Data Sizes Tested:** 13 distinct size ranges
- **Data Types:** 6+ categories
- **Edge Cases:** Empty, single-byte, small, legacy, corruption

---

## Dependencies Between Tests

### Independent Tests
Most tests are independent and can run in parallel:
- All individual test functions use separate temp directories
- Each test creates its own server instance
- No shared state between tests

### Sequential Dependencies
None - all tests are fully independent

### Shared Fixtures
All tests share these helper functions:
- `setupTestEnvironment()` - isolation via temp dirs
- `loadTestConfig()` - consistent test config
- `encryptTestData()` - standard encryption pipeline
- `storeTestObject()` - standard storage format
- `generateTestToken()` - token generation
- `compressData()` - compression helper

---

## Next Steps for Failure Analysis

This inventory provides the foundation for:

1. **Identifying which tests cover specific failure scenarios**
2. **Mapping observed failures to test cases**
3. **Determining if existing tests need expansion**
4. **Adding new tests for uncovered edge cases**
5. **Validating fixes against comprehensive test suite**

When analyzing failures, reference this document to:
- Find which test validates the failing behavior
- Understand what edge cases are already covered
- Identify gaps in test coverage
- Ensure new fixes don't break existing tests

---

## Test Execution

Run all decompression tests:
```bash
go test -v ./internal/server -run "TestShare.*[Cc]ompress|TestShare.*[Ee]mpty|TestShare.*[Ss]mall|TestShare.*[Ss]ingle|TestShare.*[Ll]egacy|TestShare.*[Rr]ound"
```

Run corruption tests:
```bash
go test -v ./internal/server -run "TestShareGET_Corrupted"
```

Run all compression-related tests:
```bash
go test -v ./internal/server -run "Compress"
```

Run specific test:
```bash
go test -v ./internal/server -run "TestShareGET_CompressionBehavior"
```

---

*Document generated: 2026-08-12*
*Bead: bf-33ps49*
