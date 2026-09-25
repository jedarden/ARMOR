# Multipart Layout and Read-Path Contract

**Status:** Authoritative reference (2026-09-25, behavior as of 0.1.1975). This
document is the single normative statement of where multipart-related state
lives on B2, how paths compose under `ARMOR_PREFIX`, and how a reader must
dispatch. The decision records — [ADR-001](adr/001-bucket-prefix.md) (internal
namespaces), [ADR-003](adr/003-multipart-object-layout-and-read-path.md)
(headerless layout), [ADR-015](adr/015-out-of-order-multipart-uniform-part-size.md)
(part contract per format), [ADR-016](adr/016-multipart-metadata-finalization.md)
(manifest finalization) — remain the history and rationale; where any of them
and this document disagree on *current* behavior, this document wins and the
ADR needs an amendment.

Every path, name, and rule below was verified against the code paths cited
inline. If you change one, change the other in the same revision.

## 0. Terms

ARMOR has two different things called "manifest" and two different
`.armor/`-adjacent layouts. The confusion is itself a failure mode, so the
names are fixed here:

| Term | B2 location | Written by | Lifetime |
|---|---|---|---|
| **ciphertext object** | `<ARMOR_PREFIX><client-key>` | `UploadPart` → `CompleteMultipartUpload` assembly | until deleted |
| **HMAC sidecar** | `<ARMOR_PREFIX>.armor/hmac/<sha256>` | `CompleteMultipartUpload` | never re-written by the server after completion; not deleted when the object is deleted |
| **object manifest** | `<ARMOR_PREFIX><client-key>.armor-manifest` | `CompleteMultipartUpload` (ADR-016); operator repair | until deleted; replaced atomically on re-completion |
| **multipart upload state** | `<ARMOR_PREFIX>.armor/multipart/…` | `CreateMultipartUpload` / `UploadPart` | upload lifetime only (ephemeral) |
| **manifest index** | `<ARMOR_PREFIX>.armor/manifest/<writer-id>/…` | the in-process recorder, flushed periodically | service lifetime of the index (compacted into snapshots) |

The **object manifest** and the **manifest index** are unrelated. The object
manifest is the per-object metadata record ADR-016 introduced (one per
multipart-completed object, stored *beside* the ciphertext, inside the
client-visible key space). The manifest index is the listing-integrity index
(`internal/manifest`) that backs fast `HeadObject`/listing sizes. This
document covers the object manifest in §§4–6; the index appears only in §1's
composition table and §5's HEAD path.

## 1. Path composition

`ARMOR_PREFIX` normalizes to empty or exactly one trailing slash with no
leading slash (`config.normalizePrefix`). `Handlers.applyPrefix` /
`stripPrefix` (`internal/server/handlers/handlers.go`) translate between
client keys and stored keys; every internal writer composes the prefix itself
rather than relying on the caller to pass a prefixed key.

The internal namespace root is `<ARMOR_PREFIX>.armor/` — `"" + ".armor/"` with
no prefix. ADR-001's "Internal Namespaces" addendum is the rule: **every
writer of internal state resolves beneath the composed root**, and listings
hide internal keys under **both** roots (a bucket that gained its prefix after
ARMOR began writing keeps pre-prefix objects at the bucket root, and
`isInternalKey` (`internal/backend/backend.go`) filters both branches).

Authoritative locations, with and without a prefix:

| State | With prefix | Without prefix |
|---|---|---|
| HMAC sidecar (v1/v2 and v3 formats) | `<p>.armor/hmac/<sha256(p + client-key)>` | `.armor/hmac/<sha256(client-key)>` |
| Multipart state, format v2 | `<p>.armor/multipart/<upload-id>.state` | `.armor/multipart/<upload-id>.state` |
| Multipart state, format v3 | `<p>.armor/multipart/<upload-id>/meta.json` and `part-<n>.json` | `.armor/multipart/<upload-id>/{meta.json,part-<n>.json}` |
| Object manifest | `<p><client-key>.armor-manifest` | `<client-key>.armor-manifest` |
| Migration state | `<p>.armor/migration-state.json` | `.armor/migration-state.json` |
| Rotation state | `<p>.armor/rotation-state.json` | `.armor/rotation-state.json` |
| Manifest index | `<p>.armor/manifest/<writer-id>/{snapshot.json.gz,delta-<seq>.jsonl}` | `.armor/manifest/<writer-id>/…` |
| Provenance chain | `.armor/{chain,chain-head,chain-segments}/…` — **bucket root, not composed** (ADR-001 addendum, known exception) | same |

Two composition rules beyond the table:

- **The sidecar hash input includes the prefix.** Sidecars are named by the
  *client* key, and two tenants sharing one bucket can hold the same client
  key; hashing the bare key let their sidecars clobber each other. The
  composed name hashes `prefix + client-key`
  (`backend.GetSidecarKey`, `internal/backend/multipart_sidecar_cache.go`).
- **`ARMOR_MANIFEST_PREFIX` is relative to `ARMOR_PREFIX`**
  (`internal/config/config.go`). It relocates the manifest index within the
  tenant's namespace and is validated to never escape it (`..` is rejected).
  Taking it as a bucket-root path instead was the original bug: every tenant
  loaded every other tenant's deltas and cross-tenant key collisions resolved
  to the wrong ciphertext ref.

**Fallback rule — the one exception to "composed location wins."** Persistent
read state probes **both** locations, composed first: HMAC sidecars
(`backend.SidecarLocations`), and migration/rotation state. Prefixed
deployments accumulated bucket-root sidecars before composition existed
(2026-09-20), and a bucket that gains its prefix later keeps its pre-prefix
sidecars at the root — without the probe every pre-prefix multipart GET would
500. Ephemeral state (multipart upload state) gets **no** fallback: a rolling
deploy that moves the write location strands in-flight uploads, and the client
retrying the upload is the cheap, correct outcome. Object walks (migration,
rotation) and listing filters treat both branches as internal on the same
both-roots rule.

## 2. Stored layouts

### 2.1 Single-PUT objects (for contrast)

`[64-byte envelope header][encrypted blocks][HMAC table]` — v1/v2 carry the
HMAC table inline after the blocks; v3 carries a trailer block table at
`ciphertext_length − 36×blockCount`. All size/offset arithmetic starts at the
header. This is the layout a reader must *not* assume for multipart objects.

### 2.2 Multipart-completed objects (both formats)

The stored object is **raw concatenated part ciphertext**: no envelope header,
no embedded or trailing HMAC table. Plaintext offset N corresponds to
ciphertext offset N. B2's `CompleteMultipartUpload` concatenates the parts
byte-for-byte, which is exactly why no header can exist — part 1's first block
must land at offset 0 for the CTR geometry to hold.

### 2.3 Format v2 multipart contract (legacy; `ARMOR_FORMAT_VERSION=2`)

Governed by ADR-015 as amended by ADR-011:

- Uniform part size `P`, pinned from part number 1; `P` ≥ B2's 5 MiB part
  minimum for multi-part uploads; every part except the highest-numbered one
  is exactly `P`; the final part may be short.
- Counter offset is a function of part number only: part N starts at block
  `(N−1)×P/blockSize`. A part numbered >1 arriving before part 1 is deferred
  with retryable `503 SlowDown`.
- A non-aligned `P` (from a non-aligned part 1) switches the upload to
  ADR-011 non-uniform mode: cumulative per-part offsets, boundary-block HMAC
  backfill, parts of any size accepted (deferred until predecessors arrive).
  The cumulative offsets are persisted at completion as
  `x-amz-meta-armor-non-uniform: true` +
  `x-amz-meta-armor-cumulative-sizes` (JSON map of part number → offset).
- **Counter space ceiling:** v2 stores `blockIndex × (blockSize/16)` in a
  uint32. At 64 KiB blocks that is 2²⁰ blocks = **64 GiB**; completion rejects
  anything larger with `InternalError` ("object exceeds the Version 2 counter
  space; envelope v3 removes this limit"). This is the only size bound left on
  the multipart path.

### 2.4 Format v3 multipart contract (default)

No part-order or part-size contract at all: parts carry independent counters
in their own counter namespace, so any order and any sizes are accepted. The
per-part geometry (part boundaries, per-block ciphertext lengths, per-block
HMACs bound to the (part number, block index) pair) lives entirely in the v3
sidecar — §3.2.

## 3. The HMAC sidecar

### 3.1 Naming and locations

```
composed := <ARMOR_PREFIX> + ".armor/hmac/" + hex(sha256(ARMOR_PREFIX + client-key))
root     := ".armor/hmac/" + hex(sha256(client-key))
probe order: composed, then root   (single location when no prefix is set)
```

`backend.GetSidecarKey` / `backend.SidecarLocations`
(`internal/backend/multipart_sidecar_cache.go`) are the only name derivations;
nothing else may construct the path. The `key` argument is always the
**client** key — `CompleteMultipartUpload` saves with the unprefixed key while
the ciphertext is stored under the prefixed key. Readers that hold a stored
key strip the prefix first (`restoreverifier.clientKey`,
`armor verify`'s `verifyClientKey`, the migrator's
`loadHMCTableFromSidecar`, `armor decrypt -b2-prefix`).

Deletion removes **every** location (`DeleteHMACTable`); backend deletes are
idempotent for absent keys, so deleting a composed-only sidecar also clears
the absent root twin.

### 3.2 Formats

**v1/v2 sidecar** — plain JSON (`backend.HMACTableSidecar`):

```json
{"key": "<client-key>", "block_hmacs": [["base64 hmac"], ...],
 "block_size": 65536, "version": 2}
```

`block_hmacs` is the flat whole-object table indexed by **absolute block
index** (part boundaries already resolved at completion), which is why the v2
read paths can slice it for any range without knowing part geometry.

**v3 sidecar** — gzip-compressed JSON (`backend.HMACTableSidecarV3`):

```json
{"version": 3, "block_size": 65536,
 "parts": [{"n": 1, "plaintext_len": 16777216, "ciphertext_len": 16777216,
            "blocks": [["<hmac b64>", "<4-byte big-endian clen, b64>"], ...]}]}
```

Per part: number, plaintext and ciphertext lengths, and per-block
`[hmac, clen]` pairs (the high bit of `clen` marks a compressed block). The
v3 sidecar is also the only record of the part boundaries, so a v3 read is
impossible without it — there is no geometry fallback.

### 3.3 Lifecycle

1. **Written once**, by `CompleteMultipartUpload`, after the B2 assembly
   succeeds and before the object manifest is written (§6). v3 uploads save
   the per-part structure; v1/v2 uploads save the flattened table.
2. **Read on every GET/Range** of a multipart object, through the §3.1 probe
   order. The server caches sidecars in process keyed by
   `(bucket, prefixed ciphertext key, etag)`
   (`internal/backend/multipart_sidecar_cache.go`) so repeated range reads
   cost one backend GET.
3. **Never re-written** by the server afterwards. Rotation and migration
   rewrite the *ciphertext* and its metadata but leave the sidecar untouched —
   its name is a function of the client key, which does not change.
4. **Not deleted when the object is deleted.** `DeleteObject` removes only the
   ciphertext (§7); the sidecar is then an orphan. It is harmless (nothing
   reads it without the dispatch marker) but it holds storage until pruned
   out-of-band.
5. **Aborted uploads never wrote one** — the sidecar is written only at
   completion, so `AbortMultipartUpload` needs only to drop upload state.

## 4. The object manifest (ADR-016)

**Key:** `<ARMOR_PREFIX><client-key>.armor-manifest` — the prefix composes
because the manifest is named by `applyPrefix(key) + ".armor-manifest"`
(`handlers.manifestKeyFor`). It is an **ordinary client-visible object**,
deliberately *outside* the `.armor/` namespace: it appears in listings, and
nothing filters it. Consumers must tolerate (and skip) the suffix;
`restoreverifier.isManifestObject` is the reference filter for walkers.

**Content type:** `application/x-armor-manifest+json`.

**Body** (`backend.ManifestBody`, treated as advisory/debugging — the metadata
headers on the manifest object are authoritative):

```json
{"ciphertext_object": "<prefixed key>", "upload_id": "<id>",
 "completed_at": "2026-09-25T12:00:00Z", "metadata": {"...": "..."}}
```

**Metadata carried on the manifest object** (superset of
`backend.ARMORMetadata.ToMetadata`, plus the multipart completion fields):

| Header | Meaning |
|---|---|
| `x-amz-meta-armor-version` | envelope format (2 or 3 for multipart) |
| `x-amz-meta-armor-block-size` | encryption block size |
| `x-amz-meta-armor-plaintext-size` | whole-object plaintext size |
| `x-amz-meta-armor-plaintext-sha256` | combined per-part digest — `ComputeMultipartDigest` over the decrypted plaintext split at `part-size` boundaries, **not** a plain SHA-256 (v2; the part-size header makes the form unambiguous). Pre-bf-1v2ehf objects carry the empty-string SHA placeholder, which verifiers treat as "no digest declared" |
| `x-amz-meta-armor-etag` | the assembled ciphertext's ETag |
| `x-amz-meta-armor-content-type` | original content type |
| `x-amz-meta-armor-iv` | base64 IV (multipart objects have no header to carry it) |
| `x-amz-meta-armor-wrapped-dek` | `v2:<fingerprint>:<base64>` (or legacy base64) |
| `x-amz-meta-armor-key-id` | present only for non-default keys |
| `x-amz-meta-armor-multipart` | `"true"` — the layout dispatch marker (§5) |
| `x-amz-meta-armor-part-size` | uniform `P` (v2); lets any verifier reproduce the combined digest |
| `x-amz-meta-armor-non-uniform` / `x-amz-meta-armor-cumulative-sizes` | ADR-011 mode and the part-number → offset map (v2 non-uniform only) |
| `x-amz-meta-armor-ciphertext-ref` | the prefixed ciphertext key |
| `x-amz-meta-armor-completed-at` | RFC3339 completion timestamp; input to the freshness gate |
| `x-amz-meta-armor-compressed` / `-compression-type` | only when compressed |
| `x-amz-meta-armor-manifest-repaired-at` | added by operator repair (`notes/manifest-repair-quarantine.md`) |
| `x-amz-meta-armor-quarantined` / `-quarantine-reason` | added by operator quarantine |

**Lifecycle.** Written once per completion, atomically replacing any previous
manifest for the same key. A failed completion retry rewrites it. Operator
tools: `POST /admin/manifest/repair` (rebuild from a fresh manifest when the
ciphertext was overwritten) and `POST /admin/manifest/quarantine`
(definitive unreadable verdict).

**Era split — the fact every reader must internalize.** Objects completed
before ADR-016 shipped (pre-2026-08-31) carry their ARMOR metadata **on the
ciphertext object** and have **no manifest**. Objects completed after carry
**no metadata on the ciphertext** — the B2 multipart completion writes bare
concatenated parts — and the manifest beside them is the *only* source of
version, IV, DEK, and sizes. The server's restore-verifier states this
plainly: a manifest-era multipart object heads with an empty metadata map.

## 5. Metadata dispatch — the read contract

This is the algorithm **every** ARMOR reader is required to implement
(ADR-003's consequence, restated normatively). The order matters.

1. **Resolve the manifest.** Read
   `<prefix><client-key>.armor-manifest`.
   - Present → it is the metadata authority. Apply the operator gates first:
     quarantined → hard `403` (non-retryable, by design — retrying clients
     must stop); then the freshness gate: reject (retryable `500`) only when
     the ciphertext's LastModified is **strictly newer** than
     `completed-at`, which detects an overwrite that landed between the two
     manifest writes. A ciphertext *older* than the manifest is the normal
     healthy ordering (assembly precedes finalization).
   - Absent → fall back to `Head` of the ciphertext; if it parses as ARMOR
     metadata (a wrapped DEK is present — `backend.ParseARMORMetadata`), the
     object itself is the metadata authority (single-PUT objects, and
     multipart objects from the pre-ADR-016 era).
   - Neither → the object is not ARMOR-encrypted; passthrough semantics apply.
2. **Dispatch on the multipart marker:**
   `metadata["x-amz-meta-armor-multipart"] == "true"` → headerless layout:
   data from offset 0, HMACs from the sidecar (§3), version trusted from
   metadata (there is no envelope header to read). Anything else → envelope
   layout: read the 64-byte header, trust *its* version/IV/block size.
3. **Format dispatch within multipart:**
   - `version == 3` → load the gzip sidecar; full GET streams part by part;
     range reads compute part + block from the sidecar's per-part geometry
     (`handlers/v3_multipart_get.go`, `v3_multipart_range.go`).
   - `version == 2` (or v1) → load the flat sidecar; full GET and range reads
     slice it by absolute block index over the uniform geometry
     (`handlers.handleFullObjectStream` / `handleRangeRequest`); ADR-011
     non-uniform objects use the cumulative-offset map instead
     (`decryptNonUniformParts`).
4. **Verify HMACs closed.** A sidecar from the wrong tenant fails verification
   closed — that is the designed failure for a hash collision, and it is why
   the sidecar hash includes the prefix (§1).

**Reader conformance matrix** (who implements what today):

| Reader | Manifest fallback | Sidecar probe order | Notes |
|---|---|---|---|
| Server GET / Range | yes (`GetObject`) | yes | the reference implementation of steps 1–4 |
| Server HEAD | yes, via manifest index fast path → manifest → ciphertext Head | n/a | `HeadObject` serves size/ETag from the in-memory index when warm |
| `armor verify` | yes (`loadManifestMetadataForVerify`) | yes | quick mode's multipart stand-in: sidecar loads, parses, and accounts for exactly the stored ciphertext bytes |
| restore-verifier, both legs | yes (`resolveARMORMetadata` / `loadManifestMetadata`) | yes, via `WithKeyPrefix` managers | the ARMOR leg never decrypts (ADR-009); the direct leg decrypts with the escrowed MEK |
| format migrator | n/a (operates per object) | yes (`loadHMCTableFromSidecar`) | strips the prefix before hashing |
| key rotation | metadata-preserving in-place copy | untouched | rotation must clone the **full** raw metadata and overwrite only the wrapped DEK — rebuilding from `ToMetadata()` would drop the multipart marker and reintroduce bf-24sxh7 |
| `armor decrypt` | **no manifest fallback** | yes (`-b2-prefix`) | known gap: a manifest-era multipart object carries no object metadata, so offline decrypt of one fails before any layout logic runs. Until closed, decrypt manifest-era multipart objects through the restore-verifier's direct leg |
| `CopyObject` | **no manifest consult** | not copied | §7 — a multipart copy does not carry the sidecar (or the manifest) to the destination key |

## 6. Large-object finalization

**Why the manifest exists at all.** The pre-ADR-016 completion stamped final
metadata with a same-source/same-destination `CopyObject`. B2 caps
`b2_copy_file` at 5 GB — objects beyond it failed outright — and even below
the cap the replace left a corruption window between assembly and stamping
during which the object existed as bare ciphertext with no metadata, plus a
deadline risk on multi-GB copies. Metadata is only fully known *after*
assembly (size, ETag, combined SHA), so nothing can be set at initiation.

**The completion sequence as implemented** (`handlers.CompleteMultipartUpload`),
in order:

1. Validate the part contract (format-specific, §2.3/§2.4) and the v2 counter
   ceiling; reject loudly before assembly (`InvalidPart` / `InvalidPartSize` /
   `InternalError`).
2. `backend.CompleteMultipartUpload` — B2 assembles the ciphertext under the
   prefixed key.
3. On `NoSuchUpload` (the completion outlived the HTTP request and a retry
   arrived): recover only if `Head` shows the exact expected ciphertext size
   and a LastModified not before the upload's creation — a stale same-key
   object is never blessed. The whole finalization below is idempotent, so a
   retry that recovers here simply redoes it.
4. Save the HMAC sidecar (§3.3 step 1). Failure → `500`; the object exists but
   is not yet readable.
5. Combine the per-part plaintext digests into the whole-object digest (the
   order-sensitive step; replaces the bf-1v2ehf empty-string placeholder).
6. Build the metadata set (§4 table), write the object manifest. Failure →
   `500`; ciphertext + sidecar exist, manifest does not; a client or operator
   retry of the completion rewrites both.
7. Delete the multipart upload state (best-effort — a leftover state object is
   inert, and the abort path's delete is the same call).
8. Record the put in the manifest index (and provenance), then answer the
   client.

**What bounds object size now:** nothing on the multipart path except the v2
counter ceiling (§2.3) and B2's own limits. The manifest pattern is
size-independent — it writes one small object and performs no data movement.

## 7. Lifecycle under S3 operations

| Operation | Ciphertext | HMAC sidecar | Object manifest | Upload state |
|---|---|---|---|---|
| Complete | assembled | written | written | deleted |
| Abort | (never assembled) | never written | never written | deleted |
| `DeleteObject` | deleted | **orphaned** | **orphaned** | n/a |
| `CopyObject` (non-multipart source) | copied, DEK re-wrapped for the destination key | n/a | n/a | n/a |
| `CopyObject` (multipart source) | copied | **not copied — destination has no sidecar at its own name** | **not copied** | n/a |
| Rotation (in-place copy) | rewritten in place, wrapped DEK replaced, all other metadata preserved | untouched (name is a function of the client key) | untouched | n/a |
| Migration | rewritten in format v3 | consumed; target sidecar written per migrated object | target manifest written per migrated object | n/a |

The multipart rows of `CopyObject` are the contract's sharpest edges, and both
are as implemented today:

- **Pre-ADR-016 source** (metadata on the object): the copy re-wraps the DEK
  and copies metadata to the destination, but the destination GET then
  dispatches on the multipart marker and looks for a sidecar named by the
  **destination** client key — which does not exist — and fails. Reads of the
  copy 500; only a same-key (in-place) copy works, which is exactly the shape
  rotation uses.
- **Manifest-era source**: the ciphertext heads with no metadata, so
  `CopyObject` takes the passthrough branch and lands **raw ciphertext** at
  the destination with no metadata and no manifest. A subsequent GET serves
  those bytes as if they were plaintext. Do not copy multipart objects across
  keys; copy by re-uploading (or GET-decrypt-PUT) until this is fixed.

`DeleteObject` similarly removes only the ciphertext: the sidecar and manifest
orphan (storage leak, no correctness effect — nothing dispatches on a deleted
object), and the manifest index records the delete so listings stay correct.

## 8. Where the contract is pinned

The executable anchors, for when this document and the code disagree:

- `internal/backend/multipart_sidecar_cache.go` — sidecar naming, hash input,
  `SidecarLocations` probe order.
- `internal/backend/multipart.go` — state layouts and save/load/delete; the
  ephemeral-no-fallback rule lives in `WithKeyPrefix`'s contract comment.
- `internal/server/handlers/manifest_repair.go` — manifest suffix, content
  type, repair/quarantine metadata and gates.
- `internal/server/handlers/handlers.go` — `GetObject` dispatch (§5),
  `CompleteMultipartUpload` sequence (§6), `DeleteObject`/`CopyObject`
  lifecycle (§7).
- `internal/server/srvtest/harness.go` — asserts manifests live beside the
  data key, not under `.armor/`.
- `internal/restoreverifier/verifier.go` — the independent reimplementation of
  §5 every verifier claim depends on.
- `docs/multipart-client-compatibility.md` — the client-facing consequences of
  §2's write contracts, with the test matrix.
- `docs/runbooks/prefix-cutover-and-legacy-state.md` — the operational
  procedure for §1's composition when arming a prefix on an existing bucket.

## Related

- [ADR-001](adr/001-bucket-prefix.md) — shared bucket, internal namespaces
- [ADR-003](adr/003-multipart-object-layout-and-read-path.md) — headerless
  layout and the original read-path failure
- [ADR-011](adr/011-barman-stays-on-armor-non-uniform-multipart.md) —
  non-uniform parts
- [ADR-015](adr/015-out-of-order-multipart-uniform-part-size.md) —
  uniform-part-size contract (v2)
- [ADR-016](adr/016-multipart-metadata-finalization.md) — manifest
  finalization decision and alternatives
- [Envelope V3 Format](format/envelope-v3.md) — single-PUT v3 envelope
- [Multipart Client-Concurrency Compatibility Matrix](multipart-client-compatibility.md)
