# ADR-003: Multipart object layout, read-path dispatch, and hard-fail part validation

**Status:** Accepted (documents the design as implemented July 2026); **§4's sequential-only enforcement is superseded by [ADR-015](015-out-of-order-multipart-uniform-part-size.md)** (uniform-part-size contract — out-of-order support shipped 2026-07-19); the client-facing compatibility contract is documented — with the executable test matrix — in the [multipart client-concurrency compatibility matrix](../multipart-client-compatibility.md)
**Date:** 2026-07-18 (amended 2026-07-19)

## Context

Single-PUT objects are stored on B2 as `[64-byte envelope header][encrypted blocks][HMAC table]`. Multipart uploads cannot use this layout: B2's `CompleteMultipartUpload` concatenates uploaded parts byte-for-byte, giving ARMOR no opportunity to prepend a header or append a trailing HMAC table to the assembled object.

The original plan called for multipart objects to carry an envelope header with a reserved-byte flag (`0x01`) marking the HMAC table as external. That design was never implementable — the header would have to live inside part 1, corrupting the first block's alignment — and what actually shipped diverged from it. The divergence went undocumented, and in July 2026 it bit hard: the read path assumed every object had an embedded header and inline HMAC table, so **every GET of a multipart-completed object returned a 500** (prefetch offset out of range), and the Range path failed the same way (bf-24sxh7). Separately, `UploadPart` derived each part's CTR counter from a running `EncryptedBytes` total that assumed in-order part arrival — but real clients (litestream, AWS SDKs with concurrency enabled) upload parts in parallel and complete with arbitrary order, producing HMAC verification failures at block boundaries (bf-2sq7gf). Both were fixed in 0.1.18xx; this ADR records the resulting design so the layout contract is explicit.

## Decision

Multipart-completed objects use a distinct on-B2 layout, and the read path dispatches on an explicit metadata marker:

1. **Layout:** the stored object is raw concatenated part ciphertext. No envelope header, no embedded HMAC table. Plaintext offset N corresponds to ciphertext offset N.
2. **Sidecar HMAC table:** per-block HMACs are stored as a JSON sidecar object at `.armor/hmac/<sha256-of-object-key>`, written at `CompleteMultipartUpload` from the per-part HMACs accumulated in the multipart state object (`.armor/multipart/<upload-id>.state`).
3. **Dispatch marker:** `CompleteMultipartUpload` sets `x-amz-meta-armor-multipart: true` in object metadata (via the metadata-replace step that also writes the standard `x-amz-meta-armor-*` fields). Both the full-GET and Range paths check this marker (`internal/server/handlers/handlers.go`) and switch to: load sidecar HMAC table, read data from offset 0, use absolute block indices for HMAC verification.
4. **CTR derivation from cumulative part sizes, sequential-only enforced:** `UploadPart` computes a part's starting block index from the cumulative sizes of all lower-numbered parts recorded in multipart state. Because a part's counter offset cannot be known until every lower-numbered part's size is known, **the shipped implementation enforces sequential part upload**: a part arriving before all lower-numbered parts is rejected with `InvalidPartOrder` ("Expected part 1, got part 7. ARMOR does not support out-of-order or concurrent part uploads…"). Verified empirically 2026-07-18: `aws s3 cp` with default concurrency is rejected; with `max_concurrent_requests = 1` a 50 MB multipart round-trip through HEAD is byte-identical (SHA-256 verified). **Consequence: standard concurrent S3 clients — aws cli defaults, litestream, most SDKs — cannot multipart-upload through ARMOR until configured for serial parts.** Whether to build true out-of-order support (e.g. uniform-part-size negotiation) or standardize on documented client configuration is the open decision tracked in bf-59unr3. **(Superseded — see the status note above; the compatibility contract that replaced this behavior is documented in [docs/multipart-client-compatibility.md](../multipart-client-compatibility.md).)**
5. **Hard-fail part validation:** any part pattern ARMOR cannot encrypt correctly is rejected at request time rather than stored corrupted:
   - Part arriving out of sequence → `InvalidPartOrder` (see above).
   - Intermediate (non-final) part whose size is not a multiple of the block size → `InvalidPartSize` (400), with a message telling the client to use a block-aligned part size (e.g. 5 MiB, 16 MiB).
   - Completion referencing unknown or inconsistent parts → `InvalidPart` / `InvalidPartOrder`.

   Rationale: the 2026-06 incident class (ADR-002) was silent corruption — writes reported success while storing wrong bytes. "Reject loudly" is a hard requirement for every path where correct encryption cannot be guaranteed. The deployed 0.1.42 fleet predates this enforcement: it silently mis-encrypted concurrent-part uploads. Confirmed 2026-07-18 on the freshest `ord-devimprint` litestream snapshot (written that day by 0.1.42): unreadable at HEAD (block-512 HMAC failure), ciphertext 65 MiB larger than the declared plaintext, and `x-amz-meta-armor-plaintext-sha256` set to the empty-string SHA — the stored objects are **corrupt at rest**, and no read-path fix can recover them. Remediation requires deploying fixed ARMOR, reconfiguring writers for sequential parts, forcing fresh backup baselines, and auditing the multipart-era objects (plan.md Phases 5–6).

## Consequences

- Any reader of ARMOR data — the server, `armor decrypt`, the restore-verifier — **must** check the multipart marker before assuming envelope layout. A reader that ignores it fails on every multipart object (this is exactly what bf-24sxh7 was).
- Reading a multipart object costs one extra sidecar GET (cacheable at the Cloudflare edge like any other object).
- Deleting or copying a multipart object must account for the sidecar (`.armor/hmac/<sha256(key)>`) or it leaks/breaks; CopyObject of multipart objects inherits this constraint.
- Clients with non-block-aligned part sizes get hard 400s instead of silent corruption; the error message documents the fix (choose an aligned part size).
- Known residual gap: `CompleteMultipartUpload` stores a placeholder plaintext SHA-256 (hash of empty string) instead of the true whole-object hash, weakening downstream SHA-based verification for multipart objects (bf-1v2ehf, open).
- The plan's earlier reserved-byte-flag description is superseded by this ADR.

## Addendum: Prefix-Composed Internal Locations and Read Fallback (2026-09-21)

The sidecar and multipart state paths above are bucket-root paths — the layout
this ADR originally shipped with. ADR-001's "Internal Namespaces" addendum
(2026-09-05) later required every internal writer to resolve `.armor/` beneath
`<ARMOR_PREFIX>.armor/`, and these writers were the last holdouts (census on
armor-5850c682, fix on armor-01f79985). The composed contract:

**Write side.** With a prefix in force, multipart upload state lives at
`<prefix>.armor/multipart/<upload-id>.state` (v2) and
`<prefix>.armor/multipart/<upload-id>/{meta.json,part-<n>.json}` (v3); the
HMAC sidecar lives at `<prefix>.armor/hmac/<sha256(prefix + client-key)>`. The
hash input includes the prefix on purpose: sidecars are named by the client
key, and two tenants sharing one bucket can hold the same client key — the
bare-key hash let their sidecars clobber each other (a wrong-tenant read then
failed HMAC verification closed, but one tenant's write still destroyed the
other's table). `FormatMigrator` and `KeyRotator` state
(`.armor/{migration,rotation}-state.json`) compose the same way, and the
dashboard's rotation-status read follows. With no prefix set, every one of
these paths is byte-for-byte the pre-2026-09-20 location.

**Read side — dual-location fallback for persistent state.** Sidecars are
persistent read state: prefixed deployments accumulated bucket-root sidecars
before this composition existed (ord-devimprint, `ARMOR_PREFIX=commitgraph/`),
and a bucket that gained its prefix later keeps its pre-prefix sidecars at the
root. Every sidecar loader — the server read paths, `armor decrypt`,
`armor verify`, the restore-verifier, and the migrator's sidecar walk — probes
`backend.SidecarLocations` order: the composed location first, then the
bucket root. Moving the write side without this fallback would orphan every
existing sidecar and 500 every multipart GET on exactly those deployments.
Migration and rotation state get the same read fallback (a run started before
the composition resumes instead of restarting); their saves always target the
composed location, so the state migrates forward on the next write. Object
walks (migration, rotation) and listing filters treat both locations as
internal, on the same both-branches rule as the backend's list filter.

**Ephemeral state composes with no fallback.** Multipart upload state lives
for the upload's lifetime only, so its loads do not fall back to the root: a
rolling deploy that moves the write location strands state for in-flight
uploads, and the client retries the upload — the cheap, correct outcome. This
is the same trade the manifest composition made.

**Operational consequence.** A B2 application key scoped to
`namePrefix <tenant>/` — denied bucket-root `.armor/` writes before — can now
perform every internal write a multipart upload needs inside its own
namespace.
