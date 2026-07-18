# ADR-003: Multipart object layout, read-path dispatch, and hard-fail part validation

**Status:** Accepted (documents the design as implemented July 2026)
**Date:** 2026-07-18

## Context

Single-PUT objects are stored on B2 as `[64-byte envelope header][encrypted blocks][HMAC table]`. Multipart uploads cannot use this layout: B2's `CompleteMultipartUpload` concatenates uploaded parts byte-for-byte, giving ARMOR no opportunity to prepend a header or append a trailing HMAC table to the assembled object.

The original plan called for multipart objects to carry an envelope header with a reserved-byte flag (`0x01`) marking the HMAC table as external. That design was never implementable — the header would have to live inside part 1, corrupting the first block's alignment — and what actually shipped diverged from it. The divergence went undocumented, and in July 2026 it bit hard: the read path assumed every object had an embedded header and inline HMAC table, so **every GET of a multipart-completed object returned a 500** (prefetch offset out of range), and the Range path failed the same way (bf-24sxh7). Separately, `UploadPart` derived each part's CTR counter from a running `EncryptedBytes` total that assumed in-order part arrival — but real clients (litestream, AWS SDKs with concurrency enabled) upload parts in parallel and complete with arbitrary order, producing HMAC verification failures at block boundaries (bf-2sq7gf). Both were fixed in 0.1.18xx; this ADR records the resulting design so the layout contract is explicit.

## Decision

Multipart-completed objects use a distinct on-B2 layout, and the read path dispatches on an explicit metadata marker:

1. **Layout:** the stored object is raw concatenated part ciphertext. No envelope header, no embedded HMAC table. Plaintext offset N corresponds to ciphertext offset N.
2. **Sidecar HMAC table:** per-block HMACs are stored as a JSON sidecar object at `.armor/hmac/<sha256-of-object-key>`, written at `CompleteMultipartUpload` from the per-part HMACs accumulated in the multipart state object (`.armor/multipart/<upload-id>.state`).
3. **Dispatch marker:** `CompleteMultipartUpload` sets `x-amz-meta-armor-multipart: true` in object metadata (via the metadata-replace step that also writes the standard `x-amz-meta-armor-*` fields). Both the full-GET and Range paths check this marker (`internal/server/handlers/handlers.go`) and switch to: load sidecar HMAC table, read data from offset 0, use absolute block indices for HMAC verification.
4. **Order-independent CTR derivation:** `UploadPart` computes a part's starting block index from the cumulative sizes of all lower-numbered parts recorded in multipart state — never from arrival order. Parts may be uploaded in parallel, in any order, and retried idempotently.
5. **Hard-fail part validation:** any part pattern ARMOR cannot encrypt correctly is rejected at request time rather than stored corrupted:
   - Intermediate (non-final) part whose size is not a multiple of the block size → `InvalidPartSize` (400), with a message telling the client to use a block-aligned part size (e.g. 5 MiB, 16 MiB).
   - Completion referencing unknown or inconsistent parts → `InvalidPart` / `InvalidPartOrder`.

   Rationale: the 2026-06 incident class (ADR-002) was silent corruption — writes reported success while storing wrong bytes. "Reject loudly" is a hard requirement for every path where correct encryption cannot be guaranteed.

## Consequences

- Any reader of ARMOR data — the server, `armor-decrypt`, the restore-verifier — **must** check the multipart marker before assuming envelope layout. A reader that ignores it fails on every multipart object (this is exactly what bf-24sxh7 was).
- Reading a multipart object costs one extra sidecar GET (cacheable at the Cloudflare edge like any other object).
- Deleting or copying a multipart object must account for the sidecar (`.armor/hmac/<sha256(key)>`) or it leaks/breaks; CopyObject of multipart objects inherits this constraint.
- Clients with non-block-aligned part sizes get hard 400s instead of silent corruption; the error message documents the fix (choose an aligned part size).
- Known residual gap: `CompleteMultipartUpload` stores a placeholder plaintext SHA-256 (hash of empty string) instead of the true whole-object hash, weakening downstream SHA-based verification for multipart objects (bf-1v2ehf, open).
- The plan's earlier reserved-byte-flag description is superseded by this ADR.
