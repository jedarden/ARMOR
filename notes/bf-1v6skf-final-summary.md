# ARMOR Multipart HMAC Verification Bug - Final Summary (bf-1v6skf)

## Executive Summary

The multipart HMAC verification bug reported in bf-1v6skf has been **ROOT CAUSE IDENTIFIED AND FIXED** in the codebase. However, the fix has **NOT BEEN DEPLOYED TO PRODUCTION**, which is why the restore test failed on 2026-07-14.

## Key Findings

### 1. Root Cause - TWO Bugs Fixed in Commit 3edbb9b4 (2026-07-16)

**Bug #1: HMAC Computation with Relative Indices**
- **File**: `internal/crypto/encryptor.go`
- **Issue**: `EncryptWithStartingCounter` computed HMACs using relative block indices instead of absolute indices
- **Impact**: HMAC table contained HMACs for wrong blocks when parts were concatenated

**Bug #2: Out-of-Order Part Handling**
- **File**: `internal/server/handlers/handlers.go`
- **Issue**: Parts uploaded in parallel (litestream behavior) caused incorrect startBlockIndex calculation and HMAC table assembly
- **Impact**: HMAC table was out of sync with assembled object

### 2. Why It Failed Specifically at Block 256

In a 44MB snapshot with 5MB parts and 64KB blocks:
- Part 1: Blocks 0-79 (5MB)
- Part 2: Blocks 80-159 (5MB)
- Part 3: Blocks 160-239 (5MB)
- Part 4: Blocks 240-319 (5MB) ← **Block 256 is here**

When litestream sent parts out-of-order (e.g., [3,1,4,2]):
- HMAC table assembled in wrong order: [Part3, Part1, Part4, Part2]
- B2 assembled object in correct order: [Part1, Part2, Part3, Part4]
- Position 256×32 in HMAC table contained HMAC for wrong block
- Verification failed: "block 256: HMAC verification failed"

### 3. Timeline Analysis

- **2026-07-14 00:02 UTC**: Snapshot created with buggy ARMOR v0.1.42
- **2026-07-14**: Restore test failed (reported in task)
- **2026-07-16 15:57:26**: Fix committed (commit 3edbb9b4, version 0.1.1858)
- **Current**: Production ord-devimprint still running armor:0.1.42 (NOT FIXED)

The snapshot was created **BEFORE** the fix was committed, so it contains corrupted HMAC data that cannot be repaired.

## The Fix (Commit 3edbb9b4)

Three critical changes:

1. **EncryptWithStartingCounter**: Use absolute block index `absBlockIndex = startBlockIndex + i`
2. **UploadPart**: Calculate `startBlockIndex` based on cumulative sizes of lower-numbered parts
3. **CompleteMultipartUpload**: Sort parts by PartNumber before assembling HMAC table

## Verification Results

✅ **All Tests Pass**:
- `TestMultipartHMACAbsoluteIndexing` - Verifies HMACs use absolute indices
- `TestMultipartLitestreamScenario` - Simulates 44MB litestream backup  
- `TestMultipartOutOfOrderUpload` - Tests out-of-order uploads explicitly

✅ **Code Review**: Fix is correct and comprehensive

❌ **Production Status**: Fix NOT deployed (still on buggy version 0.1.42)

❌ **Snapshot Status**: 2026-07-14 snapshot is permanently corrupted

## Acceptance Criteria Status

- ✅ **Root cause identified**: HMACs computed with relative indices + out-of-order part handling
- ✅ **Fix applied and verified**: Code fixed in commit 3edbb9b4, all tests pass
- ✅ **Regression test added**: Multiple comprehensive tests added
- ❌ **queue-api backup chain re-verified**: BLOCKED - requires deployment + new snapshot

## Deployment Requirements

The fix needs to be deployed to production:

1. **Build**: Create Docker image with version 0.1.1859+
2. **Update**: Change declarative-config to use new image
3. **Deploy**: ArgoCD will sync to ord-devimprint
4. **Verify**: Check deployment health and logs
5. **Wait**: Litestream will create new snapshot automatically
6. **Test**: Verify restore works with new snapshot

See `notes/bf-1v6skf-deployment-guide.md` for detailed deployment instructions.

## Impact Assessment

### Before Deployment (Current State)
- ❌ queue-api DR restore is BROKEN
- ❌ New multipart uploads with current version are corrupted
- ❌ Backup chain cannot be restored

### After Deployment
- ✅ New multipart uploads will have correct HMACs
- ✅ New snapshots will be restorable
- ⚠️ Old snapshots created with v0.1.42 remain corrupted
- ✅ Litestream will create new clean snapshots automatically

## Files Modified in Fix

1. `internal/crypto/encryptor.go` - Fixed HMAC computation (absolute indices)
2. `internal/server/handlers/handlers.go` - Fixed part ordering and startBlockIndex calculation
3. `internal/crypto/multipart_hmac_test.go` - New regression tests
4. `internal/crypto/multipart_litestream_test.go` - Litestream-specific scenario tests
5. `internal/crypto/multipart_out_of_order_test.go` - Out-of-order upload tests
6. `internal/server/handlers/multipart_order_test.go` - Integration-level tests

## Conclusion

The multipart HMAC verification bug has been **COMPLETELY FIXED** in the codebase with comprehensive regression tests. The fix is verified and ready for deployment. However, the fix has **NOT BEEN DEPLOYED TO PRODUCTION**, which is why the restore test failed.

The corrupted snapshot created on 2026-07-14 cannot be repaired - it was created with buggy code. After deploying the fix, new snapshots will work correctly and the queue-api backup chain will recover once litestream creates a new snapshot with the fixed ARMOR version.

**NEXT STEP**: Deploy ARMOR version 0.1.1859+ to ord-devimprint via declarative-config update and CI/CD pipeline.

---

**Bead ID**: bf-1v6skf  
**Status**: CODE FIX COMPLETE - DEPLOYMENT REQUIRED  
**Commit**: 3edbb9b4 (fix) + 1aba3d28 (documentation)  
**Version**: 0.1.1859 (includes fix)  
**Production Version**: 0.1.42 (BUGGY - NEEDS UPGRADE)
