# ADR-005: Out-of-order multipart parts via a uniform-part-size contract

**Status:** Implemented (design decided 2026-07-19; shipped on main 2026-07-19; amended 2026-08-07 for the single-part alignment exemption — bf-5tol4d core uniform-part-size contract, bf-4oi87m part-1 pinning + `503 SlowDown` deferral of earlier arrivals; amends ADR-003 §4)
**Date:** 2026-07-19

## Context

ADR-003 §4 records the shipped interim behavior: parts must arrive sequentially, because a part's CTR counter offset is derived from the cumulative sizes of all lower-numbered parts, which are unknowable until those parts have arrived. Out-of-order arrivals are rejected with `InvalidPartOrder`.

Live verification on 2026-07-18 showed what this costs: `aws s3 cp` with default settings cannot upload a multipart object through ARMOR at all (`InvalidPartOrder: Expected part 1, got part 7`), and the same is true of every standard concurrent uploader — AWS SDK transfer managers, litestream, rclone. ARMOR's entire product premise (plan.md "Goal") is that **unmodified standard S3 tools work**; a proxy that requires every client to be reconfigured for serial part upload contradicts it, and some clients (litestream) expose no such knob at all. The 2026-06/07 corruption incidents happened precisely because the pre-fix write path *accepted* concurrent parts and silently mis-encrypted them; the sequential-only enforcement stopped the corruption but replaced it with a hard compatibility break.

## Decision

Support out-of-order and concurrent part uploads by fixing the CTR geometry up front instead of deriving it from arrival history — a **uniform-part-size contract**:

1. **Part size `P` is established once per multipart upload**, from the first part that arrives: `P = ContentLength` of that part. `P` must be a multiple of the block size (else `InvalidPartSize`, as today) and ≥ B2's 5 MiB part minimum. `P` is persisted in the multipart state object.
2. **Counter offset is a function of part number only:** part `N` starts at block `(N−1) × P / blockSize`. This is computable the moment a part arrives, regardless of arrival order — the sequential-only rejection is removed.
3. **The final part is the only part allowed to differ:** a part with size < `P` is accepted (its offset still needs only `P` and `N`) and presumed final. At `CompleteMultipartUpload`, validate the contract: every part except the highest-numbered one must have size exactly `P`. Any violation → hard reject (`InvalidPart`/`InvalidPartSize`), never storage.
4. **Optimistic-`P` failure mode stays loud.** If the very first arriving part happens to be the short final part, `P` gets pinned too small and a later, larger part contradicts it. On the first contradiction (a part with size > `P`, or a second distinct size among non-final parts) ARMOR rejects the offending `UploadPart` and poisons the upload id so `CompleteMultipartUpload` fails with a clear message telling the client to retry the upload. With real clients this ordering is vanishingly rare (uploaders start parts roughly in order; concurrency reorders completions, not initiations by much) — and when it happens the result is a failed upload, never a corrupt object. This preserves ADR-002/ADR-003's invariant: any pattern ARMOR cannot encrypt correctly must fail loudly.
5. **Retries stay idempotent:** re-uploading part `N` re-encrypts at the same offset (same `N`, same `P`). Same-size re-uploads simply overwrite; a retry with a different size hits rule 4.
6. **Everything downstream is unchanged:** the headerless object layout, the `x-amz-meta-armor-multipart` marker, and the sidecar HMAC table with absolute block indices (ADR-003 §1–3) all work identically — per-part HMAC entries were already indexed absolutely.

## Alternatives considered

- **Keep sequential-only + document client configuration.** Pushes a per-client config burden onto every consumer forever, breaks clients with no serial knob (litestream), and contradicts the product goal. Rejected.
- **Server-side buffering/reordering of early parts.** Parts are up to 5 GB; buffering unbounded out-of-order arrivals in memory or scratch storage is a resource DoS vector and adds failure modes. Rejected.
- **Per-part IVs/envelopes.** Would make each part independently encryptable in any order, but changes the on-B2 format (a third layout), adds per-part overhead, and complicates range-read translation across part boundaries. Rejected — the uniform-size contract achieves order-independence with zero format change.
- **Explicit part-size negotiation (custom header on CreateMultipartUpload).** Removes the optimistic-`P` edge case but standard clients would never send it; it could be added later as an optional optimization without conflicting with this design.

## Amendment (2026-07-19): pin P from part number 1, defer earlier arrivals with SlowDown

Live testing of the initial implementation falsified rule 1's rarity assumption. With aws cli **defaults** on a 50 MB file, all 7 parts start concurrently and the *smallest* part — the short final one — reliably completes **first** (least bytes to transfer). P gets pinned to the final part's size and the upload is invalidated on the first full-size part. This is not a rare pathology: it is the *common case* whenever the part count is within the client's concurrency window (any file ≲ concurrency × part size, i.e. most files under ~80 MB for aws cli defaults).

Amended rules:

1. **P is pinned only from part number 1** — which by construction is never the short final part.
2. **A part arriving before part 1 has pinned P is answered with `503 SlowDown`** (no body consumed beyond need, nothing stored). SlowDown is retryable per the S3 contract; every standard client (aws cli, SDK transfer managers, litestream) retries the part transparently, and part 1 — started in the client's first batch — lands within the retry window. No buffering, no state, no corruption window.
3. The contradiction detection from rule 4 stays as defense-in-depth (e.g., a client that never sends part 1).

Acceptance for the amendment: a 50 MB `aws s3 cp` with **default** concurrency must round-trip byte-identically.

## Amendment (2026-08-07 UTC): alignment is required only of parts something is placed after

Rule 1 required *every* part to be block-aligned, while rule 3 exempted the final part from the uniform *size*. Nothing reconciled the two for the part that is both first and final, and the check that enforced rule 1 ran at `UploadPart` — before the part count is knowable. The result: **a multipart upload whose entire payload fits in one part could never complete.**

This is not hypothetical. `barman-cloud-backup` of a small Postgres emits exactly one part sized by the data. Live on `commitgraph-db` (ord-devimprint, 2026-08-07T02:08Z) every base backup failed with `InvalidPartSize: Part size 11917312 is not a multiple of the block size (65536 bytes)` — 11,917,312 is 55,296 bytes past a boundary. The same signature explains the `queue-db` (iad-ci) and `forgejo-postgres` failures; the previously-recorded workaround of disabling `data.compression` only helps when the payload is large enough to produce a full-size part *before* the short final one, which a small database never does.

Alignment exists for exactly one purpose: to keep the `(N−1)×P/blockSize` offset of a *following* part on a block boundary. It therefore has no force for a part nothing follows. Amended rules:

1. **Part 1 may be any size.** It always starts at block 0 — `(1−1)×P/blockSize` is 0 for every `P` — so its own ciphertext and its absolute HMAC indices are correct regardless. A non-aligned part 1 pins a non-aligned `P`, which marks the upload **single-part-only**.
2. **Any part >1 on a single-part-only upload is rejected and poisons it**, since it could not be placed on a block boundary. Enforced before the body is read, so a large deferred part costs no memory.
3. **The presumed-final part (size < `P`) may be any size.** Nothing is placed after it, and its partial trailing block is exactly what every ordinary non-multipart `PUT` of arbitrary size already produces.
4. **`CompleteMultipartUpload` backstops rule 2:** more than one part with a non-aligned `P` is rejected before assembly, so a violating state that reached Complete anyway never becomes a stored object.

A non-aligned regular part — one that another part *is* placed after — is still rejected exactly as before.

Acceptance: a single-part multipart upload of 11,917,312 bytes must round-trip byte-identically, including a range read inside the partial trailing block (`TestMultipartLonePartByteVerification`).

## Valid Upload Patterns (Examples)

The contract supports these multipart upload patterns:

### Pattern 1: Fully-aligned multi-part upload (standard case)

All parts except possibly the last are block-aligned (multiple of 65536 bytes):

```
Part 1: 16,777,216 bytes  (16 MiB, = 256 × 65536) ✓ aligned
Part 2: 16,777,216 bytes  (16 MiB, = 256 × 65536) ✓ aligned
Part 3: 16,777,216 bytes  (16 MiB, = 256 × 65536) ✓ aligned
Part 4:  8,388,608 bytes  ( 8 MiB, = 128 × 65536) ✓ aligned (final)
→ P = 16,777,216 (aligned), 4 parts, total = 58,720,256 bytes
```

This is the standard pattern used by most S3 clients (aws cli, SDKs, rclone). All parts are block-aligned, so every CTR offset is on a boundary.

### Pattern 2: Aligned parts with short final part (standard S3 semantics)

All regular parts are aligned; the final part may be any size < P:

```
Part 1: 16,777,216 bytes  (16 MiB, = 256 × 65536) ✓ aligned
Part 2: 16,777,216 bytes  (16 MiB, = 256 × 65536) ✓ aligned
Part 3: 16,777,216 bytes  (16 MiB, = 256 × 65536) ✓ aligned
Part 4:  5,200,000 bytes  (5.2 MiB, NOT a multiple of 65536) ✓ short final
→ P = 16,777,216 (aligned), 4 parts, total = 55,055,216 bytes
```

The short final part (5.2 MiB) is accepted because nothing is placed after it. Its partial trailing block is exactly what a non-multipart PUT of arbitrary size already produces.

### Pattern 3: Single short part (barman-cloud-backup case)

A single part that is both first and final, regardless of alignment:

```
Part 1: 11,917,312 bytes  (11.9 MiB, NOT a multiple of 65536) ✓ single part
→ P = 11,917,312 (non-aligned), 1 part, total = 11,917,312 bytes
```

Part 1 always starts at block 0, so its ciphertext and HMAC indices are correct regardless of size. A non-aligned P marks the upload as single-part-only; any attempt to add part 2 would be rejected.

### Pattern 4: Single aligned part (valid but unusual)

A single part that happens to be block-aligned:

```
Part 1: 16,777,216 bytes  (16 MiB, = 256 × 65536) ✓ single part
→ P = 16,777,216 (aligned), 1 part, total = 16,777,216 bytes
```

Valid, but clients typically use this size only when they expect more data (Pattern 1 or 2).

## Invalid Patterns (Rejected)

These patterns violate the contract and are rejected before storage:

### Invalid 1: Non-aligned regular part

A part >1 with size not equal to P:

```
Part 1: 16,777,216 bytes  (16 MiB) ✓ P pinned
Part 2: 15,000,000 bytes  (15 MiB, ≠ P, > P) ✗ rejected, upload poisoned
```

Reason: Part 2's offset would be `(2-1) × 16,777,216 / 65536 = 256` blocks (on boundary), but its content is sized for offset 240. This would corrupt the CTR keystream.

### Invalid 2: Two presumed-final parts

Two parts both smaller than P:

```
Part 1: 16,777,216 bytes  (16 MiB) ✓ P pinned
Part 2:  5,000,000 bytes  (5 MiB, < P) ✓ accepted as presumed-final
Part 3:  3,000,000 bytes  (3 MiB, < P) ✗ rejected, upload poisoned
```

Reason: Part 3 contradicts the presumption that part 2 was final. The contract allows at most one short part (the final one).

### Invalid 3: Attempting to add part 2 to a single-part-only upload

Adding a second part after a non-aligned part 1:

```
Part 1: 11,917,312 bytes  (11.9 MiB, not aligned) ✓ P pinned, single-part-only
Part 2: 16,777,216 bytes  (16 MiB)                 ✗ rejected, upload poisoned
```

Reason: Part 2's offset would be `(2-1) × 11,917,312 / 65536 = 181.8` blocks (NOT on boundary), corrupting encryption.

## Edge Cases

### Zero-byte final part

An empty final part (size 0) is accepted as the presumed-final part:

```
Part 1: 16,777,216 bytes  (16 MiB) ✓ aligned
Part 2:          0 bytes  (0 bytes) ✓ zero-byte final part
→ P = 16,777,216, 2 parts, total = 16,777,216 bytes
```

The zero-byte part contributes no data but marks completion. This matches standard S3 behavior.

### Read-back contract: no server-side padding

The option-a implementation does not zero-pad a short or empty final part. The
plaintext object length is the sum of the uploaded part sizes, so `GET` returns
exactly those bytes, `HEAD` reports that exact `Content-Length`, and `Range`
requests are bounded by that same length. In the zero-byte-final example above,
the completed object is exactly the bytes from part 1; the empty part contributes
no bytes and no padding.

### Minimum-size single part

A single part at B2's 5 MiB minimum:

```
Part 1: 5,242,880 bytes  (5 MiB, = 80 × 65536) ✓ aligned single part
→ P = 5,242,880, 1 part, total = 5,242,880 bytes
```

Valid and aligned. The same size without alignment would also be valid as a single part (Pattern 3).

### Sub-block single part

A single part smaller than one full block (65536 bytes):

```
Part 1: 40,000 bytes  (40 KiB, < 65536) ✓ single part
→ P = 40,000 (non-aligned), 1 part, total = 40,000 bytes
```

Valid because part 1 starts at block 0. The partial block is encrypted normally; no corruption possible.

## Rationale Summary

The alignment invariant serves exactly one purpose: **keep the CTR counter offset on a block boundary for every part that has a follower**. Parts with no follower (part 1 in single-part uploads, the presumed-final part) have no follower to misalign, so their alignment is irrelevant. This interpretation:

1. Preserves correctness for all multi-part uploads (regular parts still aligned)
2. Enables standard S3 client compatibility (short final parts are normal)
3. Fixes the barman-cloud-backup outage (single-part uploads now work)
4. Requires no format change, no new metadata fields, and no range-read complexity

## Consequences

- Standard concurrent clients (aws cli defaults, SDK uploaders, litestream, rclone) work against ARMOR unmodified — the serial-configuration caveat in plan.md has been removed and the litestream deployment note in bf-4qq1 is void.
- The multipart canary uploads its parts concurrently and the integration tests exercise genuinely concurrent (aws-cli-default-style) uploads.
- `InvalidPartOrder` disappears from the error surface for well-formed uploads; `InvalidPartSize` semantics narrow to the uniformity/alignment contract.
- A pathological client that uploads *only* its short final part first gets a loud failed upload and must retry — accepted trade-off, documented in the error message.
- ADR-003 §4 (sequential-only) is superseded by this ADR; §5's hard-fail principle is unchanged and inherited.
