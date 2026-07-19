# ADR-005: Out-of-order multipart parts via a uniform-part-size contract

**Status:** Accepted (design decided 2026-07-19; implementation pending — amends ADR-003 §4)
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

## Consequences

- Standard concurrent clients (aws cli defaults, SDK uploaders, litestream, rclone) work against ARMOR unmodified — the serial-configuration caveat in plan.md and the litestream deployment note in bf-4qq1 disappear once this ships.
- The multipart canary and integration tests can (and must) exercise genuinely concurrent uploads.
- `InvalidPartOrder` disappears from the error surface for well-formed uploads; `InvalidPartSize` semantics narrow to the uniformity/alignment contract.
- A pathological client that uploads *only* its short final part first gets a loud failed upload and must retry — accepted trade-off, documented in the error message.
- ADR-003 §4 (sequential-only) is superseded by this ADR; §5's hard-fail principle is unchanged and inherited.
