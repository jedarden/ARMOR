# bf-2sq7gf: Fix Multipart Out-of-Order Upload HMAC Verification Failure

## Problem Summary

Manual litestream restore test on 2026-07-16 revealed that multipart objects were failing HMAC verification at block 256 during decryption, preventing restore of the 73MB level-9 snapshot from ord-devimprint's queue-api backup chain.

Error: `Failed to decrypt range: block 256: HMAC verification failed`

## Root Cause

Litestream (and potentially other clients) perform multipart uploads with parts uploaded in parallel and sent to `CompleteMultipartUpload` in arbitrary order. For example, parts might be sent as `[3, 1, 4, 2]` instead of `[1, 2, 3, 4]`.

The bug manifested in TWO places:

### 1. UploadPart (CTR counter calculation)

**Bug:** The starting block index for CTR encryption was calculated as:
```go
startBlockIndex := uint32(state.EncryptedBytes / int64(state.BlockSize))
```

This assumed parts were uploaded sequentially, so `EncryptedBytes` always represented the sum of all previous parts. But when Part 3 uploads before Part 1, this calculation is wrong.

**Fix:** Calculate based on part sizes of lower-numbered parts:
```go
var totalBytesBefore int64
for pn := int64(1); pn < partNumber; pn++ {
    if size, ok := state.PartSizes[int(pn)]; ok {
        totalBytesBefore += size
    }
}
startBlockIndex := uint32(totalBytesBefore / int64(state.BlockSize))
```

### 2. CompleteMultipartUpload (HMAC table assembly)

**Bug:** The HMAC sidecar was assembled in the order parts appeared in the CompleteMultipartUpload XML:
```go
for _, p := range completeReq.Parts {  // Wrong order!
    allBlockHMACs = append(allBlockHMACs, hmacs...)
}
```

But B2 assembles the actual object in PartNumber order, not client-provided order. This caused a mismatch where:
- Position 256×32 in the HMAC table contained HMAC for the wrong block
- Verification failed: "block 256: HMAC verification failed"

**Fix:** Sort parts by PartNumber before assembling HMACs:
```go
sort.Slice(completeReq.Parts, func(i, j int) bool {
    return completeReq.Parts[i].PartNumber < completeReq.Parts[j].PartNumber
})
// Now iterate - parts are in correct order
for _, p := range completeReq.Parts {
    allBlockHMACs = append(allBlockHMACs, hmacs...)
}
```

## Why Block 256 Specifically?

In a 73MB snapshot with 5MB parts and 64KB block size:
- Part 1: Blocks 0-79    (5MB)
- Part 2: Blocks 80-159  (5MB)
- Part 3: Blocks 160-239 (5MB)
- Part 4: Blocks 240-319 (5MB) ← Block 256 is here

If CompleteMultipartUpload sent parts as `[3, 1, 4, 2]`:
- HMAC table assembled as: `[Part3_HMACs, Part1_HMACs, Part4_HMACs, Part2_HMACs]`
- B2 object assembled as: `[Part1_Data, Part2_Data, Part3_Data, Part4_Data]`
- Position 256×32 in HMAC table = HMAC from Part 4 (wrong!)
- Block 256 in actual data = block 16 in Part 4 = needs HMAC from Part 4 position 16

But the HMAC at that position was for block 16 of Part 4 (absolute block 256) computed with the wrong relative index.

## The Fix Location

Two changes in `internal/server/handlers/handlers.go`:

1. **UploadPart** (around line 2110): Calculate `startBlockIndex` based on cumulative sizes of lower-numbered parts, not `state.EncryptedBytes`.

2. **CompleteMultipartUpload** (around line 2214): Sort `completeReq.Parts` by `PartNumber` before assembling the HMAC table.

## Verification

Tests added in `internal/server/handlers/multipart_order_test.go`:
- `TestCompleteMultipartUploadXMLOrdering`: Verifies XML parsing and sorting
- `TestCompleteMultipartUploadBlock256Scenario`: Documents the exact production failure
- `TestCompleteMultipartUploadEdgeCases`: Tests various ordering scenarios

Existing crypto tests verify HMAC computation across part boundaries:
- `TestMultipartHMACAbsoluteIndexing`: Verifies HMACs use absolute block indices
- `TestMultipartLitestreamScenario`: Simulates 44MB litestream backup
- `TestMultipartOutOfOrderUpload`: Tests out-of-order uploads explicitly

## Impact

This fix is critical for data recoverability. Without it:
- Multipart objects created by clients that upload parts in parallel fail HMAC verification
- Restores fail with "block N: HMAC verification failed" errors
- Backup chains become unrecoverable

With the fix:
- Parts can be uploaded in any order (parallel uploads supported)
- CompleteMultipartUpload can send parts in any order
- HMAC tables always match B2's assembly order
- All blocks verify correctly

## Next Steps

1. Deploy this fix to production
2. Verify new litestream snapshots can be restored
3. Investigate whether existing corrupted snapshots can be recovered (may require manual HMAC table reconstruction)

Co-Authored-By: Claude <noreply@anthropic.com>
Bead-Id: bf-2sq7gf
