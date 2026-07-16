# ARMOR Multipart HMAC Bug Investigation (bf-1v6skf)

## Summary

The bug reported in bf-1v6skf has been **FIXED IN THE CODE** but the fix **HAS NOT BEEN DEPLOYED TO PRODUCTION**. The corrupted snapshot created on 2026-07-14 cannot be restored because it was created with the buggy version.

## Timeline

- **2026-07-14 00:02 UTC**: Snapshot created by ARMOR version 0.1.42 (buggy version)
- **2026-07-14**: Restore test failed with "block 256: HMAC verification failed"
- **2026-07-16 15:57:26**: Fix committed (commit 3edbb9b4, version 0.1.1858)
- **2026-07-16**: Current VERSION is 0.1.1859 (includes the fix)
- **Current**: Production ord-devimprint is still running armor:0.1.42 (buggy version)

## Root Cause

TWO bugs were fixed in commit 3edbb9b4:

### Bug 1: HMAC Computation with Relative Indices (bf-1v6skf)
**File**: `internal/crypto/encryptor.go`  
**Issue**: `EncryptWithStartingCounter` computed HMACs using relative block indices (0 to blockCount-1) instead of absolute indices (startBlockIndex+i)  
**Impact**: HMAC table contained HMACs for wrong blocks when parts were concatenated

### Bug 2: Out-of-Order Part Handling (bf-2sq7gf)
**File**: `internal/server/handlers/handlers.go`  
**Issue 1 - UploadPart**: `startBlockIndex` calculated from `state.EncryptedBytes` which assumed in-order upload  
**Issue 2 - CompleteMultipartUpload**: HMAC table assembled in client-provided part order instead of PartNumber order  
**Impact**: Litestream's parallel uploads caused HMAC table to be out of sync with assembled object

## The Fix

The fix involved THREE changes:

1. **EncryptWithStartingCounter**: Use absolute block index `absBlockIndex = startBlockIndex + i`
2. **UploadPart**: Calculate `startBlockIndex` based on cumulative sizes of lower-numbered parts
3. **CompleteMultipartUpload**: Sort parts by PartNumber before assembling HMAC table

## Why Block 256?

In a 44MB snapshot with 5MB parts and 64KB blocks:
- Part 1: Blocks 0-79    (5MB)
- Part 2: Blocks 80-159  (5MB) 
- Part 3: Blocks 160-239 (5MB)
- Part 4: Blocks 240-319 (5MB) ← Block 256 is here

When parts were sent out-of-order (e.g., [3,1,4,2]):
- HMAC table assembled in wrong order: [Part3, Part1, Part4, Part2]
- B2 assembled object in correct order: [Part1, Part2, Part3, Part4]
- Position 256×32 contained HMAC for wrong block → verification failed

## Verification

✅ **Code Tests Pass**: All multipart HMAC tests pass (TestMultipartHMACAbsoluteIndexing, TestMultipartLitestreamScenario, TestMultipartOutOfOrderUpload)

❌ **Production Snapshot Corrupted**: The snapshot created on 2026-07-14 cannot be restored because it was created with buggy version 0.1.42

❌ **Fix Not Deployed**: Production ord-devimprint is still running armor:0.1.42

## Next Steps

### 1. Deploy the Fix to Production
- Build and deploy ARMOR version 0.1.1859+ to ord-devimprint
- Update deployment: `ronaldraygun/armor:0.1.42` → `ronaldraygun/armor:0.1.1859`

### 2. Create New Snapshot
- After deployment, litestream will automatically create new snapshots with correct HMAC data
- The old snapshot (2026-07-14) will remain corrupted but new ones will work

### 3. Verify Restore Works
- Test restore with a newly created snapshot
- Verify HMAC verification succeeds for all blocks including block 256

## Acceptance Criteria Status

- ✅ Root cause identified: HMACs computed with relative indices and out-of-order parts
- ✅ Fix applied and verified: Code fixed in commit 3edbb9b4, tests pass
- ✅ Regression test added: Multiple tests in multipart_hmac_test.go, multipart_litestream_test.go, multipart_order_test.go
- ❌ queue-api backup chain re-verified: NEEDS DEPLOYMENT + NEW SNAPSHOT

## Impact Assessment

### Current State (Before Fix Deployment)
- ❌ queue-api DR restore is BROKEN
- ❌ Any new multipart uploads created with v0.1.42 are corrupted
- ❌ Backup chain cannot be restored

### After Fix Deployment
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

## Deployment Checklist

- [ ] Build new ARMOR image with version 0.1.1859+
- [ ] Deploy to ord-devimprint namespace
- [ ] Verify deployment is healthy
- [ ] Wait for litestream to create new snapshot
- [ ] Test restore with new snapshot
- [ ] Verify HMAC verification succeeds
- [ ] Update monitoring/alerts if needed

## Conclusion

The bug is FIXED IN CODE but NOT DEPLOYED. The corrupted snapshot cannot be fixed - it was created with buggy code. After deploying the fix, new snapshots will work correctly. The queue-api backup chain will recover once litestream creates a new snapshot with the fixed ARMOR version.
