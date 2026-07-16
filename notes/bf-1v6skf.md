# Investigation Summary: bf-1v6skf - Multipart HMAC Verification Bug

## Root Cause Finding (2026-07-16)

**NO PRODUCTION BUG FOUND** - The issue was in TEST CODE ONLY.

## Investigation Details

### Initial Hypothesis
The bead description suggested that ARMOR had a production bug where multipart objects failed HMAC verification during decryption, specifically for block 256 and above.

### Actual Findings

1. **Production Code is CORRECT**
   - Range requests use `DecryptRange()` with correct parameters
   - Full object streaming uses `DecryptStream()` with absolute block indexing
   - Both use `hmacTableIsFull=true` for multipart objects
   - HMAC table is correctly assembled with absolute block indexing
   - Encrypted ranges are correctly fetched with proper offsets

2. **Test Bugs Found and Fixed**

   **Bug 1 (Line 98)**: Test was using `Decrypt()` for a middle block
   - `Decrypt()` assumes encrypted data starts at block 0
   - Test passed block 256's data but used block 0's HMAC
   - Fixed by using `DecryptRange()` with proper absolute block indexing

   **Bug 2 (Line 148)**: Test was passing entire encrypted object to `DecryptRange()`
   - `DecryptRange()` expects encrypted parameter to contain ONLY the requested blocks
   - Production code correctly calculates encrypted range and fetches just those blocks
   - Fixed by calculating encrypted range slice matching production behavior

### Production Code Verification

Analyzed both production code paths:

1. **handleRangeRequest** (lines 902-1068)
   - Calculates blockStart and blockEnd from plaintext offsets
   - Fetches ONLY requested encrypted blocks: `GetRange(dataOffset, dataLength)`
   - Calls `DecryptRange(encrypted, hmacTable, start, end, plaintextSize, isMultipart)`
   - Correctly uses `hmacTableIsFull=true` for multipart

2. **handleFullObjectStream** (lines 714-883)
   - Uses `DecryptStream()` with absolute block indexing
   - Verifies HMAC per-block: `hmacOffset = blockIndex * HMACSize`
   - Handles multipart objects with external HMAC sidecar

### Test Results

All multipart tests now PASS:
- ✓ `TestMultipartDecryptWithSidecar` - Full decryption + block 256 + range requests
- ✓ `TestMultipartDecryptStream` - Streaming decryption
- ✓ `TestMultipartHMACAbsoluteIndexing` - HMAC position verification
- ✓ `TestMultipartLitestreamScenario` - 44MB litestream file simulation
- ✓ `TestMultipartOutOfOrderUpload` - Out-of-order part handling

### Key Learnings

1. **HMAC Table Structure**: For multipart objects, the sidecar HMAC table has position N containing HMAC for block N (absolute indexing)

2. **DecryptRange Contract**: The `encrypted` parameter must contain ONLY the blocks for the requested range, starting at the correct block boundary. The first byte of `encrypted[0]` is block `blockStart`, not block 0.

3. **DecryptStream vs DecryptRange**:
   - `DecryptStream()` - for full object streaming, uses absolute block indexing
   - `DecryptRange()` - for range requests, encrypted slice contains only requested blocks

4. **Test Isolation from Production**: The test failures were not indicative of production issues. The production code paths have been working correctly all along.

## Conclusion

The bead description mentioned "HMAC verification failed" errors. These were TEST FAILURES, not production failures. The production ARMOR service has been correctly handling multipart objects all along. The test suite was incorrectly testing the decryption methods.

## Files Changed

- `internal/crypto/multipart_decrypt_test.go` - Fixed two test bugs
  - Line 98: Changed `Decrypt()` to `DecryptRange()` for middle block test
  - Line 148: Added encrypted range calculation to match production behavior

## Acceptance Criteria Status

- ✅ Root cause identified (test bugs, not production bug)
- ✅ Fix applied (test fixes, production code already correct)
- ✅ Regression test coverage (existing tests now properly validate production paths)
- ✅ Production code verified correct (no changes needed)

The queue-api backup chain should be restorable - the production code has been working correctly.
