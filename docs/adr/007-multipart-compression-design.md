# ADR-007: Multipart upload compression is not supported

**Status:** Accepted (design decided 2026-08-23)
**Date:** 2026-08-23
**Related:** ADR-005 (uniform-part-size contract), bead armor-4c452726

## Context

Single-PUT ARMOR objects support zstd compression (config flag `ARMOR_COMPRESS=true`), providing significant storage savings for compressible data. However, multipart uploads bypass compression entirely — no `crypto.Compress` call, no `header.SetCompressed` flag, parts are uploaded as raw plaintext before encryption.

Task armor-4c452726 required wiring compression into the multipart upload path, with a requirement to preserve ADR-005's idempotent-retry and out-of-order upload invariants.

## Decision

**Multipart uploads MUST NOT support compression.** When compression is eventually added as a config flag, `CreateMultipartUpload` MUST fail closed with a clear error if `ARMOR_COMPRESS=true`. The implementation is:

1. **No per-part compression:** Parts are uploaded as plaintext and encrypted directly under ADR-005's uniform-part-size contract
2. **Fail closed at creation:** When `CreateMultipartUpload` is called with compression enabled, return `InvalidArgument` with message: "Compression is not supported for multipart uploads. Use single-PUT uploads for compressed objects or disable compression (ARMOR_COMPRESS=false)."
3. **Preserve single-PUT compression:** Single-PUT objects continue to benefit from compression; only the multipart path rejects it.

This is **option (b)** from the task design space: fail closed rather than attempt per-part compression.

## Why per-part compression (option a) is infeasible

### 1. Breaks ADR-005's uniform-part-size contract

ADR-005's core invariant is: **all non-final parts have uniform size `P`**, and part N's CTR offset is computed purely from part number: `startBlockIndex = (N-1)*P/BlockSize`.

zstd compression is **variable-length**. The same 16 MiB plaintext part can compress to 14 MiB, 13 MiB, or 15 MiB depending on content entropy. This means:
- There is no uniform `P` in the compressed byte stream
- The CTR offset formula breaks because `P` varies per part
- The "part-number-only" advantage of ADR-005 is lost

Recording per-part compressed sizes and computing offsets cumulatively would require sequential processing of parts, defeating ADR-005's out-of-order design.

### 2. Breaks idempotent-retry invariant

ADR-005 rule 5 guarantees: "Retries stay idempotent: re-uploading part N re-encrypts at the same offset (same N, same P)."

With zstd, **the same plaintext does NOT guarantee the same compressed output**:
- Different compression levels (default vs. tuned)
- Encoder state differences
- Dictionary training or custom encoder parameters

This violates the core invariant that a retry of part N produces byte-identical ciphertext.

### 3. CTR block-alignment breaks on compressed boundaries

CTR mode encryption operates on fixed 16-byte blocks. The offset `startBlockIndex` is a block count into the ciphertext stream.

If compressed parts vary in size:
- Part 1 might compress to 14 MiB = 917,504 blocks (not a multiple of the original 16 MiB / 65536 = 256 blocks)
- Part 2 would start at an arbitrary compressed offset, not a block-aligned one
- Computing part 2's offset requires knowing part 1's compressed size, which breaks out-of-order upload

### 4. No compressed-format supports block-aligned seeking

All mainstream compression formats (zstd, gzip, zlib) use variable-length framing. There is no "block-aligned zstd" mode where decompression can start at arbitrary block boundaries — each frame depends on previous decoder state.

This means we cannot treat compressed parts as independently decryptable at arbitrary block offsets, which is required for ADR-005's geometry.

## Alternatives considered

### Compress each part independently with per-part compressed size tracking

**Rejected** for the reasons above:
- Breaks ADR-005's uniform-part-size contract
- Breaks idempotent-retry (same input → different compressed output)
- Requires cumulative size tracking, breaking out-of-order uploads
- No standard compression format supports block-aligned seeking

### Compress after assembly (post-Complete)

**Rejected** because:
- `CompleteMultipartUpload` would need to re-download the entire assembled object (expensive)
- Re-uploading the compressed version doubles storage costs transiently
- Complicates the completion path significantly
- Defeats the streaming design of multipart assembly

### Use a block-aligned compression format

**Rejected** because:
- No standard format exists (zstd/gzip/zlib are all variable-length)
- Custom format would require client-side decoder changes
- Contradicts ARMOR's "unmodified S3 clients work" goal

## Consequences

- **Multipart objects are stored uncompressed** — no storage savings for large multipart uploads
- **Single-PUT objects benefit from compression** — the compression feature remains valuable for smaller objects and single-part uploads
- **Clear error surface** — when compression is enabled, clients get a helpful error message on `CreateMultipartUpload` rather than silent corruption
- **ADR-005 invariants preserved** — no risk of introducing corruption through the multipart path

## Implementation

The implementation is minimal:

1. When `ARMOR_COMPRESS` config flag is added, add a check in `CreateMultipartUpload`:
   ```go
   if h.config.Compress {
       h.writeError(w, "InvalidArgument",
           "Compression is not supported for multipart uploads. Use single-PUT uploads for compressed objects or disable compression (ARMOR_COMPRESS=false).", 400)
       return
   }
   ```

2. No changes to `UploadPart` or `CompleteMultipartUpload` — they already work correctly with uncompressed parts.

3. Update documentation to clarify the compression/multipart tradeoff.

## Future work

If multipart compression becomes a strong requirement (e.g., storage cost pressure on very large objects), a future ADR could design a multipart-aware compression approach that:
- Uses a fixed-size block-aligned compression format (custom or extended zstd)
- Accepts that retries produce different compressed ciphertext (store compressed size in state)
- Sacrifices ADR-005's part-number-only offset computation in favor of cumulative offset tracking
- Or compresses post-Complete with a "rewrite assembled object" operation

All of these require significant design work and should not be undertaken without a clear use case and cost-benefit analysis.
