# Decompression Verification Integration Points Design

## Overview

This document designs the integration points for decompression verification helpers with ARMOR's existing GET and range request infrastructure. It identifies where verification fits into the call chain, how to pass decompressor context, and what refactoring is needed.

## Existing Infrastructure Analysis

### Current GET Flow

**Entry Point:** `handlers.GetObject` (`internal/server/handlers/handlers.go:711`)

```
GetObject()
├── Get metadata (backend.Head)
├── Parse ARMOR metadata
├── Get MEK/DEK for decryption
├── Create decryptor
├── Check conditional headers
├── Branch: Range request OR Full object
│   ├── handleRangeRequest() [lines 1154-1321]
│   └── handleFullObjectStream() [lines 834-1121]
└── Return response
```

### Current Range Request Flow

**Function:** `handleRangeRequest` (`internal/server/handlers/handlers.go:1154`)

```
handleRangeRequest()
├── Parse range header (parseRangeHeader)
├── Check footer cache
├── Load HMAC table (sidecar or embedded)
├── Translate range to encrypted blocks (crypto.TranslateRange)
├── Fetch encrypted blocks + HMAC (parallel via errgroup)
├── Decrypt range (decryptor.DecryptRange)
├── Cache Parquet footer (if applicable)
└── Write 206 Partial Response
```

### Current Full Object Stream Flow

**Function:** `handleFullObjectStream` (`internal/server/handlers/handlers.go:834`)

```
handleFullObjectStream()
├── Prefetch HMAC table
├── Stream encrypted data from backend
├── Parse/discard header (single-PUT only)
├── Set response headers
├── Stream decrypt via io.Pipe:
│   ├── Read encrypted blocks
│   ├── Verify HMAC per block
│   ├── Decrypt block
│   ├── Update plaintext digest
│   └── Write plaintext to pipe
├── Verify plaintext SHA-256 digest
└── Stream to client (decompress if compressed)
```

### Existing Decompression Functions

**Location:** `internal/crypto/decryptor.go`

```go
func Decompress(compressed []byte) ([]byte, error)
func DecompressGzip(compressed []byte) ([]byte, error)
func DecompressZlib(compressed []byte) ([]byte, error)
```

**Existing Error Types:** `DecompressionError` with classification:
- `ErrTypeClient`: Data corruption, truncation, invalid input
- `ErrTypeServer`: Infrastructure errors

## Verification Helper Functions

**Location:** `internal/crypto/verify_decompress.go`

```go
// Core verification functions
func VerifyDecompression(decompressed, expected []byte) *VerifyResult
func VerifyRangeDecompression(decompressed, expected []byte, rangeStart int64) *VerifyResult
func VerifyDecompressionWithContext(decompressed, expected []byte, context string) *VerifyResult

// Analysis functions
func AnalyzeByteDifferences(decompressed, expected []byte) *ByteStats
```

**Return Types:**
```go
type VerifyResult struct {
    Pass       bool               // true if verification passed
    Diagnostic string             // human-readable details
    Error      *VerificationError // structured error info
}

type VerificationError struct {
    Offset   int64   // byte position of first difference
    Expected []byte  // expected bytes at offset
    Actual   []byte  // actual bytes at offset
    Context  string  // hexdump context around error
}
```

## Integration Architecture

### Option A: Verify After Decompression (Recommended for Production)

**When:** After decompression, before writing to HTTP response

**Where:** 
- `handleFullObjectStream` - line 1088-1120 (decompression stream)
- `handleRangeRequest` - line 1296-1300 (after decryption)

**Flow:**
```
Decrypt → Decompress → Verify → Write to HTTP Response
```

**Pros:**
- Catches decompression bugs/corruption before client receives data
- Minimal impact on critical path (verification is O(n) with fast short-circuit)
- Clean separation of concerns
- Can log verification failures without breaking existing behavior

**Cons:**
- Requires expected data (need to know what we're verifying against)
- Adds latency for verification pass

**Implementation Points:**

1. **Full Object Download** (`handleFullObjectStream`):
```go
// Line 1088-1120: After decompression, before io.Copy
if armorMeta.Compressed {
    // ... existing decompression setup ...
    
    // NEW: Buffer decompressed data for verification
    var decompressedBuf bytes.Buffer
    tee := io.TeeReader(decompressor, &decompressedBuf)
    
    // Stream to client (existing)
    _, err = io.Copy(w, tee)
    if err != nil {
        // ... existing error handling ...
    }
    
    // NEW: Verify after streaming
    if expectedData := getExpectedData(bucket, key); expectedData != nil {
        result := crypto.VerifyDecompression(decompressedBuf.Bytes(), expectedData)
        if !result.Pass {
            log.Printf("Verification failed for %s/%s: %s", bucket, key, result.Diagnostic)
            // Don't fail the request - data already sent to client
            // But log for monitoring and escalation
        }
    }
}
```

2. **Range Request** (`handleRangeRequest`):
```go
// Line 1296-1300: After decryption, before writing response
plaintext, err := decryptor.DecryptRange(encrypted, hmacTable, start, end, plaintextSize, isMultipart)
if err != nil {
    h.writeError(w, "InternalError", fmt.Sprintf("Failed to decrypt range: %v", err), 500)
    return
}

// NEW: Verify decompressed range
if expectedRange := getExpectedRange(bucket, key, start, end); expectedRange != nil {
    result := crypto.VerifyRangeDecompression(plaintext, expectedRange, start)
    if !result.Pass {
        // Log verification failure but still return data (defensive)
        log.Printf("Range verification failed for %s/%s [%d-%d]: %s", bucket, key, start, end, result.Diagnostic)
    }
}

// Continue to existing response writing
```

### Option B: Verify on Restore Path Only (Recommended for Initial Rollout)

**When:** In restore verifier, NOT in hot GET path

**Where:** `internal/restoreverifier/verifier.go`

**Rationale:** 
- Verification is most valuable for restore operations (DR validation)
- Keeps hot GET path fast and simple
- Allows verification to be tested and refined before production deployment
- Aligns with ADR-004 restore verification design

**Implementation:**

```go
// In restoreverifier package
func (v *Verifier) VerifyObject(ctx context.Context, bucket, key string) error {
    // ... existing restore logic ...
    
    // After ARMOR GET path
    armorData, err := v.armorBackend.Get(ctx, bucket, key)
    if err != nil {
        return fmt.Errorf("ARMOR get failed: %w", err)
    }
    
    // After direct decrypt path
    directData, err := v.directBackend.Decrypt(ctx, bucket, key)
    if err != nil {
        return fmt.Errorf("direct decrypt failed: %w", err)
    }
    
    // NEW: Verify decompression agreement
    result := crypto.VerifyDecompressionWithContext(
        armorData,
        directData,
        fmt.Sprintf("bucket=%s,key=%s", bucket, key),
    )
    
    if !result.Pass {
        return fmt.Errorf("decompression verification failed: %s", result.Diagnostic)
    }
    
    return nil
}
```

## Context Passing Design

### Decompression Context Structure

To pass decompressor state to verification, we need a lightweight context object:

```go
// DecompressionContext holds state needed for verification
type DecompressionContext struct {
    // Original plaintext info (for verification)
    OriginalPlaintextSHA []byte  // From envelope header or metadata
    OriginalSize         int64   // Plaintext size
    CompressionType     string  // "gzip", "zlib", "zstd", ""
    
    // Decompression state (for diagnostics)
    DecompressedSize     int64   // Size after decompression
    DecompressionError   error   // Any decompression error encountered
    
    // Verification state
    Verified            bool    // Whether verification passed
    VerificationResult   *VerifyResult
}
```

### Integration Points for Context Passing

1. **In `handleFullObjectStream`:**
```go
ctx := &DecompressionContext{
    OriginalPlaintextSHA: header.PlaintextSHA[:],
    OriginalSize:         plaintextSize,
    CompressionType:      armorMeta.CompressionType,
}

// After decompression
ctx.DecompressedSize = decompressedBuf.Len()
ctx.DecompressionError = decompErr

// After verification
ctx.Verified = (result != nil && result.Pass)
ctx.VerificationResult = result

// Log for monitoring
logDecompressionMetrics(ctx, bucket, key)
```

2. **In `handleRangeRequest`:**
```go
ctx := &DecompressionContext{
    OriginalSize:     plaintextSize,
    CompressionType:  armorMeta.CompressionType,
    RangeStart:       start,
    RangeEnd:         end,
}

// After decryption/decompression
ctx.DecompressedSize = int64(len(plaintext))

// After verification
ctx.Verified = (result != nil && result.Pass)
```

## Expected Data Source Design

The biggest integration challenge is: **what do we verify against?**

### Option 1: Store Reference Copy (Recommended for DR)

**Approach:** Store a separate reference copy of original plaintext for verification

**Storage:** Separate bucket/key, e.g., `.armor/reference/<bucket>/<key>`

**Pros:**
- Independent verification (doesn't trust ARMOR path)
- Useful for DR validation
- Can detect silent corruption in ARMOR path

**Cons:**
- Doubles storage costs
- Adds write path complexity

**Implementation:**
```go
func getExpectedData(bucket, key string) []byte {
    refKey := fmt.Sprintf(".armor/reference/%s/%s", bucket, key)
    data, err := backend.Get(ctx, bucket, refKey)
    if err != nil {
        return nil  // No reference copy available
    }
    return data
}
```

### Option 2: Verify Against SHA-256 (Recommended for Production)

**Approach:** Verify decompressed SHA-256 matches stored SHA-256

**Source:** 
- Single-PUT: Envelope header `PlaintextSHA` field
- Multipart: Metadata `x-amz-meta-armor-plaintext-sha`

**Pros:**
- No additional storage
- Fast (SHA-256 already computed in handlers)
- No external dependencies

**Cons:**
- Only verifies digest, not byte-for-byte correctness
- Can't detect silent corruption that preserves SHA-256 (collision attacks)

**Implementation:**
```go
func verifyDecompressedSHA256(decompressed []byte, expectedSHA string) error {
    computedSHA := sha256.Sum256(decompressed)
    computedHex := hex.EncodeToString(computedSHA[:])
    
    if computedHex != expectedSHA {
        return fmt.Errorf("SHA-256 mismatch: computed %s, expected %s", computedHex, expectedSHA)
    }
    
    return nil
}
```

### Option 3: Verify on Dual-Path Restore (ADR-004)

**Approach:** Compare ARMOR GET path with direct decrypt path

**When:** Restore operations only

**Pros:**
- Independent paths (defensive in depth)
- No additional storage
- Detects ARMOR-specific corruption

**Cons:**
- Only available on restore path
- Requires direct decrypt access
- Higher latency (two paths)

**Implementation:** See Option B above

## Recommended Integration Strategy

### Phase 1: SHA-256 Verification (Immediate, Low Risk)

**Goal:** Add lightweight verification without changing data flow

**Implementation:**
1. Wrap decompression with SHA-256 computation
2. Compare against stored SHA-256 from metadata/header
3. Log mismatches for monitoring

**Code Changes:**
- `handleFullObjectStream`: Add SHA-256 verification after decompression
- `handleRangeRequest`: Skip (range verification not supported for compressed objects - already rejected at line 819-825)

**Risk:** Low - SHA-256 already computed, just adding comparison

### Phase 2: Restore Path Verification (ADR-004 Alignment)

**Goal:** Integrate with restore verifier dual-path checks

**Implementation:**
1. In `restoreverifier`, compare ARMOR vs direct decrypt output
2. Use `VerifyDecompressionWithContext` for detailed diagnostics
3. Escalate failures to beads

**Code Changes:**
- `internal/restoreverifier/verifier.go`: Add `VerifyDecompression` call
- Create escalation bead on mismatch

**Risk:** Medium - new code path, but isolated to restore operations

### Phase 3: Full Byte-for-Byte Verification (Future Enhancement)

**Goal:** Complete verification against reference copies

**Implementation:**
1. Store reference copies on PUT (optional, opt-in)
2. Verify GET against reference when available
3. Fall back to SHA-256 verification when reference unavailable

**Code Changes:**
- `PutObject`: Add reference copy storage
- `GetObject`: Add `VerifyDecompression` call when reference exists
- Add metrics for verification pass/fail rates

**Risk:** High - storage cost increase, behavior changes

## Test Injection Points

### Unit Test Points

1. **Decompression + Verification Together:**
```go
func TestDecompressAndVerify(t *testing.T) {
    original := []byte("Hello, World!")
    compressed := compressData(original)
    decompressed, err := crypto.Decompress(compressed)
    
    result := crypto.VerifyDecompression(decompressed, original)
    if !result.Pass {
        t.Errorf("Verification failed: %s", result.Diagnostic)
    }
}
```

2. **Corruption Detection:**
```go
func TestVerifyCorruption(t *testing.T) {
    original := []byte("Hello, World!")
    corrupted := []byte("Hello, World!")
    corrupted[5] = 0x00  // Corrupt one byte
    
    result := crypto.VerifyDecompression(corrupted, original)
    if result.Pass {
        t.Error("Expected verification to fail")
    }
    if result.Error.Offset != 5 {
        t.Errorf("Expected offset 5, got %d", result.Error.Offset)
    }
}
```

### Integration Test Points

1. **Full GET Path:**
```go
func TestGetObjectWithVerification(t *testing.T) {
    // Setup: Upload test object
    original := generateTestPayload(1024)
    uploadTestObject(t, "test-bucket", "test-key", original)
    
    // Test: GET and verify
    retrieved := getObjectViaARMOR(t, "test-bucket", "test-key")
    result := crypto.VerifyDecompression(retrieved, original)
    
    if !result.Pass {
        t.Errorf("GET verification failed: %s", result.Diagnostic)
    }
}
```

2. **Range Request Path:**
```go
func TestGetRangeWithVerification(t *testing.T) {
    // Setup: Upload test object
    original := generateTestPayload(1024)
    uploadTestObject(t, "test-bucket", "test-key", original)
    
    // Test: GET range [100-199] and verify
    rangeData := getRangeViaARMOR(t, "test-bucket", "test-key", 100, 199)
    expectedRange := original[100:200]
    
    result := crypto.VerifyRangeDecompression(rangeData, expectedRange, 100)
    if !result.Pass {
        t.Errorf("Range verification failed: %s", result.Diagnostic)
    }
}
```

## Metrics and Monitoring

### Verification Metrics

**Counters:**
- `decompression_verification_total{status="pass|fail"}`
- `decompression_verification_corruption_offset{bucket}` - histogram of corruption locations
- `decompression_verification_byte_mismatches` - histogram of mismatch counts

**Gauges:**
- `decompression_verification_pass_rate` - rolling 5-minute rate

**Logs:**
- On failure: log `VerificationResult.Diagnostic` with bucket/key context
- On success: log nothing (or debug level)

### Alerting

**Critical Alerts:**
- Verification failure rate > 0.1% (potential data corruption)
- Verification failures with same offset pattern (systematic corruption)

**Warning Alerts:**
- Verification failures isolated to single object (possible bit flip)
- Missing expected data for verification (configuration issue)

## Error Handling Strategy

### When Verification Fails

**Hot GET Path (Production):**
1. Log the failure with full diagnostics
2. Emit metrics for monitoring
3. **Still return data to client** (defensive - don't break reads)
4. Consider returning additional header: `X-Armor-Verification: failed`

**Restore Path (ADR-004):**
1. Fail the restore operation
2. Return detailed error to caller
3. Create escalation bead
4. Trigger alert for investigation

### Missing Expected Data

**Behavior:** Graceful degradation

```go
if expectedData == nil {
    // No reference available - skip verification
    log.Printf("No expected data for verification: %s/%s", bucket, key)
    return nil
}
```

## Backward Compatibility

### No Breaking Changes

**Existing Behavior:**
- All existing GET operations continue to work
- Range requests continue to work (compressed objects already rejected)
- Error responses unchanged

**New Behavior:**
- Additional logging (no functional change)
- Optional verification (can be disabled via feature flag)
- Additional HTTP response header (optional)

**Rollout Plan:**
1. Deploy with verification disabled (feature flag)
2. Enable in staging/test environment
3. Monitor metrics and logs
4. Gradual rollout to production (10% → 50% → 100%)

## Summary of Integration Points

### Files to Modify

1. **`internal/server/handlers/handlers.go`:**
   - `handleFullObjectStream` (line 1088-1120): Add verification after decompression
   - `handleRangeRequest` (line 1296-1300): Add verification after decryption (optional, low priority)

2. **`internal/restoreverifier/verifier.go`:**
   - Add dual-path verification using `VerifyDecompressionWithContext`

3. **`internal/crypto/context.go` (new):**
   - Define `DecompressionContext` struct

### Dependencies

**None required** - verification helpers are self-contained and don't require external dependencies.

### Performance Impact

**Negligible** - verification is O(n) byte comparison with fast short-circuit on first mismatch. For 1GB objects:
- SHA-256 computation: ~100ms (already done)
- Byte comparison: <10ms on first mismatch
- Full forensic analysis: ~100ms (only on failures)

### Security Considerations

**Defense in Depth:**
- HMAC verification already ensures ciphertext integrity
- SHA-256 digest verification ensures plaintext integrity
- Byte-for-byte verification catches decompression bugs
- No new attack surface (verification is read-only)

## Conclusion

The verification helpers integrate cleanly into ARMOR's existing GET infrastructure with minimal refactoring. The recommended phased approach allows gradual rollout:

1. **Phase 1:** SHA-256 verification (low risk, immediate value)
2. **Phase 2:** Restore path verification (ADR-004 alignment)
3. **Phase 3:** Full byte-for-byte verification (optional, future enhancement)

The design maintains backward compatibility while adding powerful corruption detection capabilities. All integration points are well-defined and testable.
