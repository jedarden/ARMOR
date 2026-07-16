# Multipart HMAC Verification Bug Fix (bf-1v6skf)

## Issue Summary

ARMOR failed HMAC verification on large multipart-uploaded objects during decryption. The error "block 256: HMAC verification failed" occurred when trying to restore a 44MB litestream snapshot, even though the object was successfully encrypted by ARMOR on the same day.

## Root Cause

The bug was in `internal/crypto/encryptor.go` in the `EncryptWithStartingCounter` method, which is used for multipart uploads.

### The Bug (lines 148-152 in encryptor.go.bak)

```go
// Compute HMAC for this block
// CRITICAL: Use relative block index (i) not absolute (startBlockIndex+i)
// The HMAC table for each part uses local indexing (0 to blockCount-1)
// When parts are concatenated during CompleteMultipartUpload, this creates
// a global table where position N corresponds to block N.
hmacValue := e.computeBlockHMAC(encryptedBlock, i)  // ❌ WRONG: relative index
```

**Problem:** The code used the relative block index `i` (0 to blockCount-1 within the part) instead of the absolute block index across all parts.

### The Fix (lines 147-155 in encryptor.go)

```go
// Compute HMAC for this block
// CRITICAL: Use absolute block index (startBlockIndex+i)
// The HMAC table for each part uses absolute block indexing (0 to totalBlocks-1)
// When parts are concatenated during CompleteMultipartUpload, this creates
// a global table where position N corresponds to block N in the entire object.
// During decryption, HMACs are verified with absolute indices, so we must
// compute them with absolute indices here too.
absBlockIndex := startBlockIndex + i
hmacValue := e.computeBlockHMAC(encryptedBlock, absBlockIndex)  // ✓ CORRECT
```

## Why This Caused Failure

### Multipart Upload Flow

1. **Part 1** (blocks 0-79, 5MB):
   - Encryption: Used CTR counters 0-79 ✓
   - HMAC computation (BUGGY): Computed HMACs for blocks 0-79 using relative indices 0-79
   - Result: Part 1 HMACs were correct by accident (relative == absolute for first part)

2. **Part 2** (blocks 80-159, 5MB):
   - Encryption: Used CTR counters 80-159 ✓
   - HMAC computation (BUGGY): Computed HMACs for blocks 80-159 using relative indices 0-79
   - Result: HMACs were computed with WRONG indices! Should have been 80-159, but were 0-79

3. **CompleteMultipartUpload**:
   - Concatenated all part HMACs into a single sidecar table
   - Position 0-79: HMACs for blocks 0-79 (correct)
   - Position 80-159: HMACs for blocks 80-159 (❌ WRONG - actually computed for blocks 0-79)

4. **GetObject/Decrypt**:
   - For block 256 (in part 4), fetched HMAC from position 256*32 in sidecar
   - Verified HMAC using absolute index 256
   - Verification FAILED because the HMAC at position 256 was computed for block 256-240=16

## Impact

- **queue-api on ord-devimprint**: Litestream backup chain could not be restored
- **Any large multipart object (>5MB)**: Objects crossing part boundaries would fail HMAC verification
- **Silent corruption**: The bug only manifested during decryption; encryption appeared to succeed

## Verification

Added `TestMultipartHMACAbsoluteIndexing` in `internal/crypto/multipart_hmac_test.go` which:
1. Encrypts 3 parts (15MB total, 240 blocks)
2. Verifies HMACs at part boundaries (blocks 79, 80, 159, 160)
3. Decrypts the concatenated encrypted data
4. Confirms plaintext matches across all parts

Test output confirms:
```
✓ Block 0 (first block of part 1) HMAC verified
✓ Block 79 (last block of part 1) HMAC verified
✓ Block 80 (first block of part 2) HMAC verified
✓ Block 159 (last block of part 2) HMAC verified
✓ Block 160 (first block of part 3) HMAC verified
✓ Block 239 (last block of part 3) HMAC verified
✓ All HMAC verifications passed across part boundaries
✓ Decryption produced correct plaintext across all parts
```

## Acceptance Criteria

- [x] Root cause identified: HMACs computed with relative indices instead of absolute
- [x] Fix applied: Changed to use `absBlockIndex = startBlockIndex + i`
- [x] Fix verified: Test passes with content-level verification across part boundaries
- [x] Regression test added: `TestMultipartHMACAbsoluteIndexing`
- [ ] queue-api backup chain re-verified (requires production access)

## Related Files

- `internal/crypto/encryptor.go` - Fixed HMAC computation
- `internal/crypto/multipart_hmac_test.go` - Regression test
- `internal/crypto/decryptor.go` - Verification logic (unchanged, already correct)

## Next Steps

1. Test against actual production data once queue-api backup is re-verified
2. Monitor for any other multipart-related issues
3. Consider adding integration test with actual B2 backend for end-to-end verification
