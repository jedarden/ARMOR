# ARMOR Multipart/HMAC Decrypt Fix (2026-07-22)

## Executive Summary

On 2026-07-19, ARMOR shipped a critical fix for multipart upload integrity verification. The bug caused all multipart objects to store a meaningless placeholder digest instead of the actual plaintext SHA-256, making corruption detection impossible for large objects. The fix was production-verified on 2026-07-22 with a 56.6MB real-world multipart round-trip on `iad-kalshi` cluster running ARMOR version 0.1.1901.

**Bead Reference:** See bead `bf-1v6skf` for DR restore verification context.

## The Bug

### What Was Broken

**Commit:** Introduced in early ADR-003 implementation, fixed in commit `9f6d5694` on 2026-07-19.

`CompleteMultipartUpload` was setting every multipart object's plaintext digest to `sha256("")` (the SHA-256 of an empty byte sequence) — a hardcoded placeholder:

```go
// Before fix (bf-1v2ehf)
const EmptyPlaintextSHA256Hex = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
// Every multipart object got this value — meaningless!
```

### Impact

- **No integrity verification:** Manifests, provenance records, and object metadata all carried a digest that couldn't distinguish valid content from corruption
- **Audit failures:** Any future audit comparing stored SHA against actual content would flag every large object as corrupted
- **False security:** The digest appeared valid but provided no real protection
- **Affects all large objects:** Multipart uploads are used for objects > 5MB (ARMOR's minimum multipart threshold)

### Root Cause

The ADR-003 multipart implementation had a gap: it tracked per-part HMACs for decryption integrity but never accumulated the per-part plaintext digests into a whole-object plaintext digest at completion time. The placeholder was a temporary measure that became permanent.

## The Fix

### Two-Part Solution (Commits `9f6d5694` and `23dc1dcd`)

#### Part 1: Real Per-Part Plaintext SHA-256 (Commit `9f6d5694`)

**Bead:** `bf-1v2ehf`

**Key Changes:**

1. **Per-part digest tracking in `MultipartState`:**
   ```go
   type MultipartState struct {
       // ... existing fields ...
       PartPlaintextSHAs map[int]string `json:"part_plaintext_shas"`
   }
   ```

2. **`UploadPart` records each part's plaintext SHA-256:**
   - Each part's plaintext is hashed as it arrives
   - Idempotent: same-size retry overwrites identical plaintext, so no-op on re-upload
   - Stored keyed by part number (parts arrive out of order per ADR-005)

3. **`CompleteMultipartUpload` combines digests in order:**
   ```go
   func CombinePartPlaintextSHAs(partSHAs map[int]string, partNumbers []int) (string, error) {
       h := sha256.New()
       for _, n := range partNumbers {
           digest, _ := hex.DecodeString(partSHAs[n])
           h.Write(digest)  // Feed raw 32-byte digests through hasher
       }
       return hex.EncodeToString(h.Sum(nil)), nil
   }
   ```

4. **Reader recomputation for verification:**
   ```go
   func ComputeMultipartDigest(plaintext []byte, partSize int64) string {
       // Split plaintext at P boundaries, hash each chunk, hash concatenated digests
       // Mirrors CombinePartPlaintextSHAs for verification
   }
   ```

5. **Legacy placeholder handling:**
   ```go
   func IsPlaceholderPlaintextSHA(s string) bool {
       return s == "" || s == EmptyPlaintextSHA256Hex  // "no digest declared"
   }
   ```
   - Old objects with placeholder aren't mis-enforced
   - New objects get full verification

#### Part 2: Part Size Pinning Fix (Commit `23dc1dcd`)

**Bead:** `bf-4oi87m`

**Problem:** The original ADR-005 "first arriving part pins P" rule failed in production:

- **Scenario:** `aws s3 cp` with DEFAULT concurrency (the exact product target)
- **File:** ~50MB upload starts all parts concurrently
- **Failure:** Short final part (fewest bytes) completes FIRST, pins P to final-part size
- **Contradiction:** First full-size part then contradicts P → upload invalidated
- **Frequency:** COMMON case for files under ~80MB with aws-cli defaults

**Fix — ADR-005 Amendment:**

1. **P pinned ONLY from part NUMBER 1:**
   - Part 1 is always full-size in well-formed uploads → uniform size correct by construction
   - Part number alone determines CTR offset: part N starts at block `(N-1)*P/BlockSize`

2. **Parts arriving before part 1 get retryable 503 SlowDown:**
   ```go
   if state.PartSize == 0 && partNumber != 1 {
       w.Header().Set("Retry-After", "1")
       h.writeError(w, "SlowDown",
           "Part 1 for this multipart upload has not been received yet...",
           http.StatusServiceUnavailable)
       return
   }
   ```
   - No state mutation, no backend upload, no PartSizes entry
   - Body left unread → no server memory cost
   - Standard clients (aws cli, SDKs, rclone, litestream) retry transparently

3. **Defense-in-depth contradiction detection (ADR-005 rule 4):**
   - Now catches only genuine contradictions:
     - Part larger than P
     - Two short parts (after part 1 pins P)
     - Same-part retry with different size
     - Client that never sends part 1 (fails loudly at Complete)
   - "Short final arrives first" path now SlowDown'd before it can pin P

### Streaming GET Path Verification

**`MultipartDigestAccumulator`** lets the streaming GET path verify multipart objects without buffering:

```go
type MultipartDigestAccumulator struct {
    combined      hash.Hash  // Combined per-part digest
    part          hash.Hash  // Current part's running hash
    blocksPerPart int64      // P/blockSize (final part may be shorter)
    blocksInPart  int64      // Blocks written to current part
}
```

- As each decrypted block arrives, fold into current part's hash
- When part boundary reached (or final block lands), feed part digest into combined hasher
- Matches `CombinePartPlaintextSHAs` behavior → identical to stored digest
- No buffering needed → works for multi-gigabyte objects

## Production Verification

### Test Execution (2026-07-22)

**Bead:** `bf-1v6skf`
**Cluster:** `iad-kalshi`
**ARMOR Version:** 0.1.1901
**Test Object:** 56.6MB real-production multipart upload
**Result:** Round-trip successful, plaintext integrity verified

### What Was Tested

1. **Multipart upload** via standard AWS S3 client
2. **Complete operation** stores real combined per-part digest (not placeholder)
3. **GET/decrypt** recomputes digest from plaintext → matches stored value
4. **No HMAC errors** during block-by-block decryption
5. **Metadata integrity** (x-amz-meta-armor-part-size correctly stored and used)

### Block Size Behavior

**ADR-005 Contract (Post-Fix):**

- **Part size P** must be block-aligned (rejected at upload if not)
- **P pinned from part 1** → deterministic for all subsequent parts
- **CTR offset calculation:** Part N starts at block `(N-1)*P/BlockSize`
- **Final part may be short** → handled correctly in digest computation
- **Part arrival order** doesn't matter (out-of-order per ADR-005)

**Edge Cases Handled:**

1. **Single-part uploads (< 5MB):** Use plain SHA-256 of whole plaintext
2. **Legacy pre-fix objects:** Placeholder treated as "no digest declared"
3. **Non-block-aligned part sizes:** Rejected at `UploadPart` with `InvalidPartSize`
4. **Concurrent part uploads:** SlowDown defers parts >1 until part 1 arrives
5. **Retry with different size:** Poisoned upload → Complete fails cleanly
6. **Missing part 1:** Fails loudly at Complete with clear error message

### Remaining Considerations

**Known Limitations:**

1. **Legacy objects:** Objects written before bf-1v2ehf still carry placeholder digest
   - Mitigation: `IsPlaceholderPlaintextSHA()` treats them as "no digest declared"
   - Not re-verified on GET (no false failures)
   - Can be re-uploaded to gain proper integrity digest

2. **Part size metadata required:** Objects without `x-amz-meta-armor-part-size` fall back to plain SHA-256
   - Affects: Legacy multipart objects, single-PUT objects (correct behavior)

3. **Memory for part 1:** Part 1 body must be read to pin P
   - Mitigated: Part 1 is typically 5-8MB (default part size range)
   - Not buffered after P is pinned

**No Remaining Critical Edge Cases:**

- All upload paths covered by ADR-005 rules
- Download verification matches upload computation exactly
- Streaming and batch verifiers both use same digest logic
- Restore verifier updated to handle multipart digests correctly

## Code Changes Summary

### Files Modified

1. **`internal/backend/multipart.go`** (+141 lines)
   - Added `PartPlaintextSHAs` to `MultipartState`
   - `CombinePartPlaintextSHAs()` — order-sensitive digest combination
   - `ComputeMultipartDigest()` — verifier recompute from plaintext
   - `MultipartDigestAccumulator` — streaming verification
   - `IsPlaceholderPlaintextSHA()` — legacy placeholder detection

2. **`internal/restoreverifier/verifier.go`** (+79 lines, modified)
   - `plaintextDigestForMetadata()` — metadata-aware digest computation
   - Updated to handle multipart digests correctly

3. **`internal/restoreverifier/verifier_test.go`** (+134 lines)
   - Test coverage for multipart digest verification

4. **`internal/server/handlers/handlers.go`** (+105 lines)
   - `UploadPart` records per-part plaintext SHA-256
   - `CompleteMultipartUpload` combines and stores real digest
   - Part 1 pinning logic with SlowDown defer for early parts

5. **`internal/server/handlers/multipart_routing_test.go`** (+120 lines)
   - Test updates for part 1 pinning and SlowDown behavior

### Testing

All tests green under `-race`:
- Multipart upload/download round-trips
- ADR-005 acceptance tests (including shuffled arrival)
- HMAC verification tests
- Restore verifier tests

## Related Beads

- **`bf-1v2ehf`**: Original multipart plaintext digest fix implementation
- **`bf-4oi87m`**: Part size pinning amendment (ADR-005)
- **`bf-1v6skf`**: Production verification and DR restore context
- **`bf-1pphhz`**: Restore verifier fleet verification

## Deployment Timeline

- **2026-07-19**: Code fixes committed (commits `9f6d5694`, `23dc1dcd`)
- **2026-07-19**: All tests green under `-race`
- **2026-07-22**: Production verification on `iad-kalshi` (56.6MB test object)
- **Version 0.1.1901**: Deployed to production cluster

## References

- **ADR-005**: Multipart Upload Ordering and Counter Management
- **ADR-003**: Original multipart encryption design
- **Commit `9f6d5694`**: Real per-part plaintext SHA-256 implementation
- **Commit `23dc1dcd`**: Part size pinning fix (ADR-005 amendment)
- **Commit `179e9eae`**: Production verification record

---

*Document created: 2026-08-23*
*ARMOR version at documentation: 0.1.1906*
