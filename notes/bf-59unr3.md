# bf-59unr3: Reject Unsupported Multipart Patterns with 400 Errors

**Status:** ✅ ARMOR validation complete, external writers require pinning

**Date:** 2026-07-15

## Summary

This task required deciding whether to support or reject three multipart upload patterns that real SDKs use by default but ARMOR's CTR counter derivation doesn't support. The decision was to **reject these patterns with explicit 400 errors** rather than allowing silent corruption.

## Decision Made

ARMOR now hard-fails with 400 BadRequest for three unsupported multipart patterns:

### U6: Out-of-order / parallel part upload
- **Pattern:** boto3/SDKs upload parts concurrently by default
- **ARMOR behavior:** Rejects with 400 and error: "Parts must be uploaded in sequential order"
- **Validation:** handlers.go:2082-2086 checks that parts arrive sequentially

### U7: Part retry after network failure
- **Pattern:** All SDKs retry individual parts on network failure
- **ARMOR behavior:** Rejects with 400 and error: "Part X has already been uploaded"
- **Validation:** handlers.go:2073-2077 prevents duplicate part numbers

### U8: Non-block-aligned intermediate parts
- **Pattern:** Any part size that's not a multiple of 64KiB (65536 bytes)
- **ARMOR behavior:** Rejects with 400 and error: "Part size X is not a multiple of the block size"
- **Validation:** handlers.go:2092-2096 ensures part sizes % 65536 == 0

## Implementation Complete

### 1. Validation Logic (handlers.go:2069-2096)
```go
// U7: Reject part retries
if _, exists := state.PartSizes[int(partNumber)]; exists {
    h.writeError(w, "InvalidPart", "Part N has already been uploaded...", 400)
    return
}

// U6: Reject out-of-order parts
expectedPartNumber := len(state.PartSizes) + 1
if int(partNumber) != expectedPartNumber {
    h.writeError(w, "InvalidPartOrder", "Expected part X, got part Y...", 400)
    return
}

// U8: Reject non-block-aligned parts
if plaintextSize > 0 && plaintextSize%int64(state.BlockSize) != 0 {
    h.writeError(w, "InvalidPartSize", "Part size not a multiple of block size...", 400)
    return
}
```

### 2. Test Coverage (multipart_routing_test.go:347-480)
- ✅ U6_out_of_order_parts: Upload part 3 without part 2
- ✅ U6_parallel_parts_simulation: Upload part 2 before part 1
- ✅ U7_part_retry: Upload same part number twice
- ✅ U8_non_block_aligned_part: 10,000,000-byte part (10M % 65536 ≠ 0)
- ✅ U8_zero_byte_part_allowed: Edge case validation

All tests pass.

### 3. Documentation Updated
- docs/upload-retrieval-test-matrix.md:39-41 shows U6/U7/U8 as ✅ rejected with 400
- docs/upload-retrieval-test-matrix.md:66 shows bead as FIXED

## ARMOR Writers Status

### ✅ Internal ARMOR (compliant)
- **canary/canary.go:** Uses 2MB parts (2,097,152 % 65,536 = 0) and sequential upload
- **All ARMOR handlers:** Already enforce the new validation rules

### ⚠️ External Writers (require pinning)

#### commitgraph/onboard-worker/worker.py
- **Current:** Uses boto3 default TransferConfig (parallel uploads, 8MB parts)
- **Issue:** 8MB parts not block-aligned (8,388,608 % 65,536 = 32,768 ≠ 0)
- **Fix needed:** Configure TransferConfig
  ```python
  from boto3.s3.transfer import TransferConfig
  
  # Block-aligned part size (5,242,880 = 5MiB, which is 80 * 65536)
  # Disable parallel uploads (max_concurrency=1)
  config = TransferConfig(
      multipart_threshold=5242880,  # 5MiB
      multipart_chunksize=5242880,   # 5MiB (80 * 65536, block-aligned)
      max_concurrency=1,             # Sequential only
      num_download_attempts=1,       # No part retries
  )
  
  s3 = boto3.client('s3', ...)
  s3.upload_fileobj(buf, bucket, key, Config=config)
  ```

#### commitgraph/clone-worker/worker.py
- **Current:** Uses boto3 default (parallel uploads, 8MB parts)
- **Issue:** Same as onboard-worker
- **Fix needed:** Same TransferConfig configuration

#### commitgraph/user-enrichment-worker/worker.py
- **Current:** Uses boto3 default
- **Issue:** Same as onboard-worker
- **Fix needed:** Same TransferConfig configuration

## Client Requirements Documentation

All ARMOR multipart clients MUST:

1. **Upload parts sequentially** (part 1, then part 2, then part 3...)
   - Disable SDK parallel upload features
   - For boto3: `max_concurrency=1` in TransferConfig

2. **Never retry individual parts**
   - If any part fails, abort the entire upload and restart from CreateMultipartUpload
   - For boto3: Set appropriate retry logic at the upload level, not part level

3. **Use block-aligned part sizes**
   - All parts (except possibly the last) must be multiples of 65536 bytes
   - Recommended sizes: 5,242,880 (5MiB), 16,777,216 (16MiB), 33,554,432 (32MiB)
   - For boto3: `multipart_chunksize=5242880` (or other aligned size)

## Verification

```bash
# Run multipart validation tests
go test -v ./internal/server/handlers -run TestMultipartSuspectPatterns

# Run full multipart test suite
go test -v ./internal/server/handlers -run Multipart

# Verify canary compliance (2MB parts = block-aligned)
grep -n "partSize.*2.*1024.*1024" internal/canary/canary.go
```

## Related Beads

- **bf-24sxh7:** Multipart objects unreadable through GET (separate issue, also fixed)
- **bf-1v2ehf:** Multipart placeholder SHA-256 (separate issue)
- **bf-15sdaf:** Build issue blocking tests (separate issue)

## Commit

Committed as `7bfc98f3` on 2026-07-15: "fix(bf-59unr3): reject unsupported multipart patterns with 400 errors"
