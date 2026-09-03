# V2 Multipart Fixture V3 Conversions

This document catalogs all V2 multipart fixtures and their expected V3 conversion outcomes, including the part-by-part transformation of every part.

**Document Version:** 1.0  
**Generated:** 2026-09-03  
**Scope:** All V2 multipart fixtures in `tests/fixtures/migration/v2_multipart/` and the V2 multipart entry in `tests/fixtures/migration/generated_fixtures/`

**Companion documents:**

- `v1-multipart-fixtures.md` — V1 multipart fixtures (same directory; the direct predecessor of this document)
- `v1-single-part-fixtures.md` — V1 single-PUT fixtures (same directory)
- `v2-single-part-fixtures.md` — V2 single-PUT fixtures (same directory)

---

## V2 Multipart Source Format Overview

### Stored object layout (`stored_ciphertext.bin`)

Like V1 multipart, V2 multipart objects carry **no envelope header** and **no inline HMAC table**. The stored blob is the raw concatenation of the encrypted parts:

```
Offset  | Size        | Field
--------|-------------|--------------------------------------------------
0x00    | sum(partLen)| Ciphertext of part 1 (V2-encrypted, see below)
...     | partLen     | Ciphertext of part 2
...     | ...         | ... remaining parts concatenated in part order
```

- Version, IV, block size, plaintext size and SHA-256 all come from **S3 object metadata**, not from the blob. The read path trusts metadata for multipart objects (`internal/server/server.go:1821` — "Multipart objects have no envelope header - trust metadata version"), and so does the migrator (`internal/server/format_migration.go:412-419`).
- Unlike V1, the parts are **not independent keystreams**: production V2 multipart uploads continue a single CTR counter stream across part boundaries (see next section). For decryption purposes the concatenated blob behaves as one continuous block sequence.

### V2 per-block counter derivation (fixed stride, cumulative across parts)

Each 64 KiB ARMOR block is encrypted with AES-256-CTR. The counter block for block `j` of an object is (`internal/crypto/encryptor.go:219`, `makeCounter`, Version2 branch):

```
counter_block = IV[0:12] || uint32BE(j × (blockSize / 16))
```

At the 64 KiB block size the stride is `65536 / 16 = 4096` AES blocks per ARMOR block (ADR-005, `docs/adr/005-ctr-counter-stride-fix.md`). This fixes V1's keystream reuse: consecutive blocks consume disjoint counter ranges.

How the part number enters `j` on the **write** path (`internal/server/handlers/handlers.go:3460-3500`):

| Layout | Production write mechanism | Starting counter |
|---|---|---|
| Uniform parts (ADR-005) | `Encryptor.EncryptWithStartingCounter(plaintext, startBlockIndex)` | `startBlockIndex = (partNumber − 1) × P / blockSize` where `P` is the uniform part size |
| Non-uniform parts (ADR-011) | `OffsetEncryptor.EncryptFromOffset(plaintext, cumulativeOffset)` at `crypto.Version2` | Derived from the part's cumulative **byte** offset in the object |

Both mechanisms produce the same result: **one continuous counter stream across the whole object**. Part `k` block `j` uses the global block index `j + (blocks before part k)`, so there is no cross-part keystream reuse — but parts are *not* independent streams, and correct encryption/decryption depends on cumulative-offset bookkeeping (the reason ADR-010 and ADR-011 exist at all).

**V2-specific limitation:** the 32-bit counter field holds `j × 4096`, so the counter space caps an object at `2^32 / 4096 = 2^20` blocks = **64 GiB at 64 KiB blocks** (`checkCounterSpace` / `checkCounterSpaceWithStart`, `internal/crypto/encryptor.go:242-264`: "object exceeds the Version 2 counter space; envelope v3 removes this limit"). V1's counter field is a raw block index (2^32 blocks ≈ 256 TiB before this becomes binding), so the 64 GiB ceiling is a V2-specific defect that V3 removes.

Note that block 0 is the point where V1 and V2 coincide: `0 × 4096 = 0`, so **the first block of a V1 object and a V2 object encrypted with the same DEK and IV are byte-identical**. The versions diverge from block 1 onward (V1 counter 1 vs V2 counter 4096).

### V2 sidecar layout (`sidecar.bin`)

Identical mechanism to V1 multipart (same sidecar object, same table format, same verification):

- **Path:** `.armor/hmac/<sha256hex(object_key)>`. Fixtures use the nominal key `test-object-key`, whose hash is `3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d`.
- **Content:** bare concatenation of 32-byte HMAC-SHA256 values, one per 64 KiB block, in **global block order** (part 0's blocks first, then part 1's, …).
- **HMAC input:** `HMAC-SHA256(hmacKey, uint32BE(globalBlockIndex) || ciphertext_block)` with `hmacKey = HKDF-SHA256(DEK, info="armor-hmac-v1")` (`internal/crypto/hkdf.go` — a single `HMACKeyInfo` constant shared by V1 and V2).
- **Size:** `32 × ceil(plaintext_length / 65536)` bytes — independent of part boundaries.
- ADR-011 boundary blocks may carry all-zero **placeholder** HMACs in real objects; the streaming read path skips them (`internal/crypto/decryptor.go:158`, `internal/server/handlers/handlers.go:1508`). The fixture generators never emit placeholders.

### V2 multipart object metadata

| Field | Meaning in V2 multipart fixtures |
|---|---|
| `x-amz-meta-armor-version` | `"2"` |
| `x-amz-meta-armor-multipart` | `"true"` — routes reads and migration to the sidecar path |
| `x-amz-meta-armor-part-size` | Nominal uniform part size. Present for uniform (`5242880`) and variable-final (`3145728`) layouts; **absent for non-uniform (ADR-011)** |
| `x-amz-meta-armor-wrapped-dek` | `v2:<MEK fingerprint>:<base64>` — the V2 fingerprinted format. Fixtures from `generate_fixtures.go` carry a **16-hex-char** fingerprint (`crypto.MEKFingerprint`, `internal/crypto/fingerprint.go:12`: first 8 bytes of `SHA-256(MEK)` hex-encoded) and a 40-byte AES-KWP body; fixtures from `standalone_generator.go` carry an **8-hex-char** fingerprint and a 60-byte AES-GCM body (see *Known Divergences*) |
| `x-amz-meta-armor-iv` | Base64 of the 16-byte object IV (fixtures: `AwQFBgcICQoLDA0ODxAREg==` = `03 04 … 12`) |
| `x-amz-meta-armor-block-size` | `"65536"` (64 KiB) |
| `x-amz-meta-armor-plaintext-size` | Total plaintext size across all parts |
| `x-amz-meta-armor-sha256` | SHA-256 of the whole plaintext |
| `x-amz-meta-armor-etag` | Plaintext SHA-256 hex. Set by **all three** `generate_fixtures.go` V2 multipart generators (`GenerateV2MultipartUniform`, `GenerateV2MultipartVariableFinal`, `GenerateV2MultipartNonUniform`), so **every** on-disk miniature carries it; the standalone full-scale variants (HEAD bytes) omit it. Carried over by migration as preserveable metadata |

---

## V3 Target Format Overview

Migration is a **decrypt → re-encrypt → re-upload** pipeline, and the *output* layout is chosen by plaintext size, not by the source layout (`internal/server/format_migration.go`, `migrateObject`):

| Migrated plaintext size | Output path | Resulting V3 object |
|---|---|---|
| ≤ 5 MiB (`multipartThreshold()`, `format_migration.go:1063`) | `encryptAsSingle` | **V3 single-PUT**: `header(64) ‖ encrypted blocks ‖ trailer block table`, block size **4096** |
| > 5 MiB | `uploadAsMultipart` | **V3 multipart**: 5 MiB parts via a real multipart upload, block size **65536**, `multipart=true`, `part-size=5242880` |

The full V3 layouts (envelope header, `MakeV3Counter`, trailer block table, `EncryptPartV3`, gzip-JSON `HMACTableSidecarV3`) are specified in `v1-multipart-fixtures.md` ("V3 Target Format Overview") and are identical for V2 sources; they are not repeated here. The short version of what V3 changes relative to a V2 multipart source:

- Counter becomes `IV[0:8] ‖ uint16BE(part) ‖ uint32BE(block) ‖ uint16BE(aesBlock)` — the part number lives **inside** the counter, so parts become independent, order-independent, size-independent streams. ADR-010/ADR-011 bookkeeping and the V2 64 GiB ceiling both disappear.
- HMAC binds part and block: `HMAC-SHA256(hmacKey, uint16BE(part) ‖ uint32BE(block) ‖ ciphertext_block)`.
- Integrity storage splits by output layout: trailer block table (36 B/block) for single-PUT, gzip-compressed JSON sidecar (`HMACTableSidecarV3`, `internal/backend/multipart.go`) for multipart.
- Fresh random 32-byte DEK (AES-KWP-wrapped, `v2:<16-hex fingerprint>:<base64>`) and fresh random 16-byte IV; plaintext, plaintext size and SHA-256 preserved byte-for-byte.

---

## Conversion Routing For These Fixtures

Every fixture below carries `x-amz-meta-armor-multipart: "true"` and `x-amz-meta-armor-version: "2"`, so `classifyObject` returns category `v2_multipart` (`format_migration.go:1108-1112`) — **provided `"2"` is in the migration's `includeVersions` scope**; the version check runs before the layout switch, so a V2 multipart object outside the requested scope is categorized `v2` and skipped.

Migration then takes the shared multipart decrypt path: unwrap DEK → load HMAC table from `.armor/hmac/<sha256(key)>` → decrypt the concatenated ciphertext with the **metadata** IV/version → HMAC-verify with absolute (global) block indices (`decryptMultipartObject`, `format_migration.go:567-603` — the same code path V1 multipart takes, with `version = 2` selecting the striding counter).

As with V1, what happens next depends only on plaintext size, which is why the catalog lists both the **on-disk miniature** (94-byte, working tree) and the **committed full-scale variant** (15 MiB, HEAD) for each fixture:

- Miniatures (≤ 5 MiB) → **V3 single-PUT** output, despite arriving as multipart.
- Committed 15 MiB variants (> 5 MiB) → **V3 multipart** output with three 5 MiB parts.

---

## Fixture Catalog

### 1. uniform_parts

**Location:** `tests/fixtures/migration/v2_multipart/uniform_parts/`  
**Source pattern:** ADR-005 uniform multipart with V2's fixed counter derivation  
**Produces the on-disk miniature:** `generate_fixtures.go` → `GenerateV2MultipartUniform(testPlaintext, 5 MiB)` (`tests/fixtures/migration/generate_fixtures.go:415`)  
**Produced the committed full-scale variant:** `standalone_generator.go` → `GenerateV2Multipart(multipartPlaintext, 5 MiB)` (`tests/fixtures/migration/standalone_generator.go:485`)

#### Input structure (on-disk miniature)

**metadata.json:**

```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v2",
  "source_layout": "multipart-uniform",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [1],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V2 multipart with uniform part sizes (fixed counter derivation)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**

```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-etag": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-multipart": "true",
  "x-amz-meta-armor-part-size": "5242880",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-version": "2",
  "x-amz-meta-armor-wrapped-dek": "v2:ae216c2ef5247a37:yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

The wrapped DEK is the V2 format over the same AES-KWP body the V1 miniatures use (`yuqSIns…`, 40 bytes): V1 and V2 fixtures share DEK, MEK and IV; only the metadata encoding and the counter derivation differ.

**stored_ciphertext.bin:** 94 bytes, raw ciphertext, no header. First bytes: `69 8f a1 aa fe 78 74 94 df b1 c7 bc 36 cf 0e 95 …` (see *Known Divergences* #1 for what these bytes do and do not correspond to).

**sidecar.bin:** 32 bytes — one HMAC-SHA256 entry: `b0 92 5d 78 83 71 1d 90 d8 b4 24 fb d2 2e 2a a4 c0 01 cf 5d 00 5b 1c a3 63 d1 98 b1 3f c5 84 d9`

#### Input part map (on-disk miniature)

`splitIntoUniformParts(94 B, 5 MiB)`: a single short part.

| Part | Plaintext range | Size | Blocks | V2 counters used |
|---|---|---|---|---|
| 1 | bytes 0–93 | 94 B (final, short) | 1 | `IV[0:12] ‖ BE32(0 × 4096)` = `IV[0:12] ‖ BE32(0)` |

#### Expected V3 output (on-disk miniature, ≤ 5 MiB → single-PUT path)

**Metadata fields:**

```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<fresh random base64>",
  "x-amz-meta-armor-block-size": "4096",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "<sha256 of the recovered plaintext>",
  "x-amz-meta-armor-etag": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c"
}
```

- `x-amz-meta-armor-multipart` is **not** carried over; the migrated object is a V3 single-PUT.
- The source `x-amz-meta-armor-part-size=5242880` **is** carried over by `buildNewMetadata` (`format_migration.go:820-821`) even though the output is single-PUT.
- `x-amz-meta-armor-etag` is copied through (`format_migration.go:826-827`).
- On the **intended** (production-readable) input, `x-amz-meta-armor-sha256` is preserved byte-for-byte; see *Known Divergences* #1 for what the current fixture bytes actually produce.

**Object structure:**

- Envelope: `header(64 B) ‖ 94 B ciphertext ‖ trailer block table (36 B × 1 block)` — 194 bytes total
- `part = 0`, single 4 KiB block, V3 counter `IV[0:8] ‖ BE16(0) ‖ BE32(0) ‖ BE16(0..255)`
- `compression_used`: false; the old HMAC sidecar is not rewritten and the object no longer references one

**Part-by-part conversion:**

| Source part | Source blocks (V2) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1, block 0 — counter `IV[0:12]‖BE32(0)`, HMAC in sidecar slot 0 | 94 B | Single-PUT part 0, block 0 | counter `IV[0:8]‖BE16(0)‖BE32(0)‖BE16(aes)`, HMAC in trailer entry 0 |

#### Committed full-scale variant (15 MiB, HEAD)

With the standalone generator's full-scale plaintext (15 MiB of `byte(i % 256)`, SHA-256 `5f496adeac6ba5b21a781fe82ecff7548255bcec7abd11705187a2c0f058827c`) the fixture becomes 3 uniform 5 MiB parts, and migration takes the **multipart output path**:

| Property | V2 source (HEAD bytes) | V3 output |
|---|---|---|
| part_count | 3 | 3 |
| part sizes | 5242880 × 3 | 5242880 × 3 |
| blocks_per_part | [80, 80, 80] | [80, 80, 80] |
| sidecar | flat table, 7680 B (240 × 32) | gzip-JSON `HMACTableSidecarV3`, per-(part, block) HMACs |
| metadata block-size | 65536 | 65536 |
| metadata part-size | 5242880 | 5242880 (rewritten by the multipart output path) |
| multipart flag | true | true |
| wrapped DEK (HEAD bytes) | `v2:ae216c2e:guRAp6eZ…` (8-hex fingerprint, 60-byte AES-GCM body) | `v2:<16-hex fingerprint>:<base64>` (fresh DEK, AES-KWP) |

Because V2 threads **one continuous counter stream** through the parts, the per-part mapping is a pure re-indexing — the same plaintext block always recovers, regardless of which part it sat in:

| Source (V2, cumulative global index) | Destination (V3, part-namespaced) |
|---|---|
| part 1 block j: global index j — counter `IV[0:12]‖BE32(4096j)` | part 1 block j: `IV[0:8]‖BE16(1)‖BE32(j)‖BE16(aes)` |
| part 2 block j: global index 80+j — counter `IV[0:12]‖BE32(4096(80+j))` | part 2 block j: `IV[0:8]‖BE16(2)‖BE32(j)‖BE16(aes)` |
| part 3 block j: global index 160+j — counter `IV[0:12]‖BE32(4096(160+j))` | part 3 block j: `IV[0:8]‖BE16(3)‖BE32(j)‖BE16(aes)` |

**Transformation notes**

1. This is the fixture family where migration is a **structural version bump rather than a keystream-reuse fix**: V2 already prevents counter reuse within the object. What migration adds is per-part independence (part number in the counter), the trailer/gzip-JSON integrity formats, and freedom from the 64 GiB counter ceiling and the cumulative-offset bookkeeping.
2. Part structure is preserved here only because the output part size (5 MiB) equals the source's nominal part size; part boundaries are otherwise an implementation detail of the output path, not an invariant.
3. The flat global-index sidecar is replaced by per-(part, block) HMACs identified by `BE16(part)‖BE32(block)`; the old sidecar object is neither rewritten nor deleted.

---

### 2. variable_final_part

**Location:** `tests/fixtures/migration/v2_multipart/variable_final_part/`  
**Source pattern:** ADR-010 variable final part exemption (Barman compatibility) under V2 counters  
**Produces the on-disk miniature:** `generate_fixtures.go` → `GenerateV2MultipartVariableFinal(testPlaintext, 3 MiB, 2 MiB)` (`generate_fixtures.go:557`)  
**Produced the committed full-scale variant:** `standalone_generator.go` → `GenerateV2MultipartVariableFinal(multipartPlaintext, 3 MiB, 2 MiB)` (`standalone_generator.go:814`)

#### Input structure (on-disk miniature)

**metadata.json:**

```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v2",
  "source_layout": "multipart-variable-final",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [1],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V2 multipart with variable final part (ADR-010 exemption pattern)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**

```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-multipart": "true",
  "x-amz-meta-armor-part-size": "3145728",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-version": "2",
  "x-amz-meta-armor-wrapped-dek": "v2:ae216c2ef5247a37:yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**Key metadata difference vs `uniform_parts`:** `x-amz-meta-armor-part-size` is `"3145728"` (3 MiB) — the *nominal uniform* part size, not the final part's size. As in V1, the final part's actual length is derived by subtraction and never recorded. `x-amz-meta-armor-etag` (plaintext SHA-256 hex) **is** present here, same as in `uniform_parts` — all three `generate_fixtures.go` V2 multipart generators set it; it is absent only from the committed full-scale variant, whose `standalone_generator.go` producer does not emit an etag for V2 multipart objects.

**stored_ciphertext.bin / sidecar.bin:** byte-identical to `uniform_parts` (same plaintext, same keys, same single block) — the fixture differs only in the declared layout and part-size metadata.

#### Input part map (on-disk miniature)

`splitIntoVariableFinalParts(94 B, 3 MiB, 2 MiB)`: 94 ≤ 2 MiB, so the whole object is the final part.

| Part | Plaintext range | Size | Blocks | V2 counters used |
|---|---|---|---|---|
| 1 (final) | bytes 0–93 | 94 B | 1 | `IV[0:12] ‖ BE32(0)` |

#### Expected V3 output (on-disk miniature, ≤ 5 MiB → single-PUT path)

Identical output shape to `uniform_parts`:

- Metadata: `version=3`, `block-size=4096`, fresh `iv`/`wrapped-dek` (`v2:` format), `plaintext-size=94`, `multipart` flag dropped — the **stale source `part-size=3145728` is carried over** by `buildNewMetadata` on this path, and so is `etag`.
- Object: `header(64) ‖ 94 B ciphertext ‖ trailer block table (36 B)`, part 0, block 0.

**Part-by-part conversion:**

| Source part | Source blocks (V2) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1 (final, 94 B), block 0 | counter `IV[0:12]‖BE32(0)`, sidecar HMAC slot 0 | Single-PUT part 0, block 0 | counter `IV[0:8]‖BE16(0)‖BE32(0)‖BE16(aes)`, trailer entry 0 |

#### Committed full-scale variant (15 MiB, HEAD)

`splitIntoVariableFinalParts(15 MiB, 3 MiB, 2 MiB)` yields **five 3 MiB parts** — with 15 MiB divisible by 3 MiB the splitter emits five full uniform parts and never produces a smaller final part (identical behavior to the V1 generator; the committed `v3_expected` records `part_count: 5`, `blocks_per_part: [48, 48, 48, 48, 48]`). At 15 MiB total the migrated object exceeds the 5 MiB threshold, so the output is a V3 multipart upload with three 5 MiB parts:

| Property | V2 source (HEAD bytes) | V3 output |
|---|---|---|
| part_count | 5 | 3 |
| part sizes | 3145728 × 5 | 5242880 × 3 |
| blocks_per_part | [48, 48, 48, 48, 48] | [80, 80, 80] |
| sidecar | flat table, 7680 B | gzip-JSON V3 sidecar |
| metadata part-size | 3145728 | 5242880 (rewritten by the multipart output path) |

**Transformation notes**

1. The ADR-010 exemption exists because V1/V2 multipart pin the counter stream cumulatively and need the final part's length inferred. V3 removes the reason for the exemption: parts are independent streams (part number in the counter), so any part-size mix is legal.
2. Part structure is **not** preserved (5 × 3 MiB in → 3 × 5 MiB out); `v3_expected.part_count`/`blocks_per_part` describe the *source* layout. Validation should assert plaintext SHA-256 equality and V3 readability, not part-count equality.
3. Real ADR-010/ADR-011 objects can carry zero-placeholder HMACs on boundary blocks. The server's streaming read path skips them, but the migrator's `decryptMultipartObject` uses `Decryptor.Decrypt` (`internal/crypto/decryptor.go:70`), which verifies **every** slot strictly — see *Known Divergences* #5.

---

### 3. non_uniform_parts

**Location:** `tests/fixtures/migration/v2_multipart/non_uniform_parts/`  
**Source pattern:** ADR-011 non-uniform multipart (Barman `chunk_size + N*512` style) under V2 counters  
**Produces the on-disk miniature:** `generate_fixtures.go` → `GenerateV2MultipartNonUniform(testPlaintext, [1 MiB, 2 MiB, 3 MiB])` (`generate_fixtures.go:698`)  
**Produced the committed full-scale variant:** `standalone_generator.go` → `GenerateV2MultipartNonUniform(multipartPlaintext, [1 MiB, 2 MiB, 3 MiB])` (`standalone_generator.go:931`)

#### Input structure (on-disk miniature)

**metadata.json:**

```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v2",
  "source_layout": "multipart-nonuniform",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [1],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V2 multipart with non-uniform part sizes (ADR-011 pattern)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**

```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-etag": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-multipart": "true",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-version": "2",
  "x-amz-meta-armor-wrapped-dek": "v2:ae216c2ef5247a37:yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**Key metadata difference:** there is **no `x-amz-meta-armor-part-size` at all** — non-uniform layout cannot be summarized by one number. This is the fixture that exercises "multipart without part-size metadata" during migration; `buildNewMetadata` emits no `part-size` on the single-PUT output path, and the multipart output path adds its own. (`x-amz-meta-armor-etag` is present, as in the other two miniatures.)

**stored_ciphertext.bin / sidecar.bin:** byte-identical to the other two miniatures (verified: all three miniatures share the same `stored_ciphertext.bin` and `sidecar.bin` bytes).

#### Input part map (on-disk miniature)

`splitIntoNonUniformParts(94 B, [1 MiB, 2 MiB, 3 MiB])`: the first claimed size already covers the object, so a single 94-byte part is emitted.

| Part | Plaintext range | Size | Blocks | V2 counters used |
|---|---|---|---|---|
| 1 | bytes 0–93 | 94 B | 1 | `IV[0:12] ‖ BE32(0)` |

#### Expected V3 output (on-disk miniature, ≤ 5 MiB → single-PUT path)

Same output shape as the other miniatures:

- Metadata: `version=3`, `block-size=4096`, fresh `iv`/`wrapped-dek` (`v2:` format), `plaintext-size=94`, `multipart` dropped, and — because the source had no `part-size` — no `part-size` is emitted either. `etag` is carried over.
- Object: `header(64) ‖ 94 B ciphertext ‖ trailer block table (36 B)`, part 0, block 0.

**Part-by-part conversion:**

| Source part | Source blocks (V2) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1 (94 B), block 0 | counter `IV[0:12]‖BE32(0)`, sidecar HMAC slot 0 | Single-PUT part 0, block 0 | counter `IV[0:8]‖BE16(0)‖BE32(0)‖BE16(aes)`, trailer entry 0 |

#### Committed full-scale variant (15 MiB, HEAD)

The claimed sizes are `[1 MiB, 2 MiB, remainder]`, so part 3 absorbs 12 MiB (committed `v3_expected`: `part_count: 3`, `blocks_per_part: [16, 32, 192]`):

| Property | V2 source (HEAD bytes) | V3 output |
|---|---|---|
| part_count | 3 | 3 |
| part sizes | 1048576, 2097152, 12582912 | 5242880, 5242880, 5242880 |
| blocks_per_part | [16, 32, 192] | [80, 80, 80] |
| sidecar | flat table, 7680 B | gzip-JSON V3 sidecar |
| metadata part-size | absent | 5242880 |

**Transformation notes**

1. V2 handled non-uniform parts with the cumulative-offset `OffsetEncryptor` (`EncryptFromOffset`, `internal/crypto/offset_encryptor.go:61`) plus zero-placeholder HMACs for boundary blocks. V3 replaces all of it with per-part counters and per-(part, block) HMACs.
2. Part boundaries are **not** preserved: the V3 multipart output always re-splits at 5 MiB. Only the total plaintext is invariant.
3. This is the only V2 multipart fixture whose source metadata lacks `part-size`; migration must not require it.

---

### 4. v2-multipart-uniform (generated)

**Location:** `tests/fixtures/migration/generated_fixtures/v2-multipart-uniform/` (untracked; produced by `standalone_generator.go`)  
**On-disk bytes produced by:** an older generator run with `mediumPlaintext` (256 KiB); the generator's current `main()` passes the 15 MiB `multipartPlaintext` to the same function (`standalone_generator.go:1597`), so a fresh run rewrites this fixture at full scale

#### Input structure (256 KiB on-disk bytes)

**metadata.json:**

```json
{
  "plaintext_sha256": "2312394bd99545d9de131c24efb781e765ac1aec243f2ed9347597a793a415e9",
  "plaintext_length": 262144,
  "source_version": "v2",
  "source_layout": "multipart-uniform",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [4],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V2 multipart object with HMAC sidecar",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**

```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-multipart": "true",
  "x-amz-meta-armor-part-size": "5242880",
  "x-amz-meta-armor-plaintext-size": "262144",
  "x-amz-meta-armor-sha256": "2312394bd99545d9de131c24efb781e765ac1aec243f2ed9347597a793a415e9",
  "x-amz-meta-armor-version": "2",
  "x-amz-meta-armor-wrapped-dek": "v2:ae216c2e:ZWv52qG0u/0q56PUMxOYj0lA5qOyxOp0SLSlHW2pd4TAZMuJTlm6OGRW81ctLC2PrkkMOqVbOXsUZ0/I"
}
```

Note the wrapped DEK: 8-hex fingerprint and a 60-byte AES-GCM body (`nonce(12) ‖ ciphertext(32) ‖ tag(16)`), versus ARMOR's `crypto.WrapDEK` AES-KWP 40-byte form behind the 16-hex fingerprint of the miniatures. See *Known Divergences* #6.

**stored_ciphertext.bin:** 262144 bytes, raw ciphertext (4 blocks × 64 KiB), no header. First bytes: `28 dc ee e6 a8 5d 1f fa …`. Plaintext is the deterministic `byte(i % 256)` pattern.

**sidecar.bin:** 128 bytes — 4 × 32-byte HMAC entries (one per block), first bytes `7a fe 1d 85 08 c2 90 c4 …`.

#### Input part map

`GenerateV2Multipart` encrypts the **whole plaintext as one V2 stream** (`standalone_generator.go:490`) and then computes `partCount`/`blocksPerPart` arithmetically from the 5 MiB part size — for 256 KiB that is one nominal part of 4 blocks:

| Part | Plaintext range | Size | Blocks | V2 counters used |
|---|---|---|---|---|
| 1 | bytes 0–262143 | 256 KiB (final, short) | 4 | global indices 0–3 — counter `IV-free: LE32(4096 × j)` per the standalone derivation (see *Known Divergences* #1) |

Because the ciphertext is a single continuous V2 stream, this fixture's *shape* matches what a production V2 multipart writer emits for a one-part object — unlike the standalone variable-final and non-uniform generators, which restart the counter per part (see *Known Divergences* #6).

#### Expected V3 output (256 KiB ≤ 5 MiB → single-PUT path)

**Metadata fields:**

```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<fresh random base64>",
  "x-amz-meta-armor-block-size": "4096",
  "x-amz-meta-armor-plaintext-size": "262144",
  "x-amz-meta-armor-sha256": "2312394bd99545d9de131c24efb781e765ac1aec243f2ed9347597a793a415e9"
}
```

**Object structure:**

- `header(64) ‖ 262144 B ciphertext ‖ trailer block table (36 B × 64 blocks = 2304 B)` at block size 4096 — 264512 bytes total
- `part = 0`; blocks 0..63 with V3 counters
- `compression_used`: false; no sidecar; `multipart` flag dropped
- Source `part-size=5242880` is carried over into the migrated metadata by `buildNewMetadata` even though the output is single-PUT

**Part-by-part conversion:**

| Source part | Source blocks (V2) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1 (256 KiB), blocks 0–3 (64 KiB each) | sidecar HMAC slots 0–3 | Single-PUT part 0, blocks 0–63 (4 KiB each) | trailer entries 0–63, counter `IV[0:8]‖BE16(0)‖BE32(b)‖BE16(aes)` |

**Transformation notes**

- Demonstrates the multipart→single-PUT downgrade path for a multi-block object (the 94-byte miniatures only exercise a single block).
- Output block count (64) differs from source block count (4) because `encryptAsSingle` uses 4 KiB blocks; block count is a derived property, not an invariant.

---

## Negative-Path V2 Multipart Expectations

`tests/fixtures/migration/v3-golden-outcomes.json` defines **no V2-specific failure entries** — the only multipart failure goldens are the V1-labeled `v1_multipart_sidecar_corrupt` and `v1_multipart_sidecar_missing`. The failure modes themselves are version-agnostic, because both versions share the sidecar mechanism and the same decrypt path (only the counter derivation differs):

| Condition | Expected outcome | Where it fails |
|---|---|---|
| Sidecar object absent | failure (`missing_sidecar`) | `loadHMCTableFromSidecar`, before any write |
| Sidecar HMAC mismatch | failure (`integrity_check_failed`) | `Decryptor.Decrypt` during `decryptMultipartObject`, before any write |
| Invalid `v2:` wrapped DEK / base64 | failure before fetch | metadata pre-validation (`format_migration.go:375-395`) |

Both integrity failures must happen **before** any re-encrypted bytes are written: the migrator writes the replacement object only after decryption succeeds.

---

## Conversion Summary Matrix

| Fixture | Source layout | Plaintext | Source parts × blocks | Migrated output layout | Output block size | Sidecar after migration |
|---|---|---|---|---|---|---|
| `v2_multipart/uniform_parts` (miniature) | multipart-uniform | 94 B | 1 × [1] | V3 single-PUT | 4096 | none (old sidecar orphaned) |
| `v2_multipart/variable_final_part` (miniature) | multipart-variable-final | 94 B | 1 × [1] | V3 single-PUT | 4096 | none |
| `v2_multipart/non_uniform_parts` (miniature) | multipart-nonuniform | 94 B | 1 × [1] | V3 single-PUT | 4096 | none |
| `generated_fixtures/v2-multipart-uniform` (256 KiB) | multipart-uniform | 256 KiB | 1 × [4] | V3 single-PUT | 4096 | none |
| full-scale `uniform_parts` (15 MiB, HEAD) | multipart-uniform | 15 MiB | 3 × [80,80,80] | V3 multipart, 3 × 5 MiB | 65536 | gzip-JSON V3 sidecar |
| full-scale `variable_final_part` (15 MiB, HEAD) | multipart-variable-final | 15 MiB | 5 × [48×5] | V3 multipart, 3 × 5 MiB | 65536 | gzip-JSON V3 sidecar |
| full-scale `non_uniform_parts` (15 MiB, HEAD) | multipart-nonuniform | 15 MiB | 3 × [16,32,192] | V3 multipart, 3 × 5 MiB | 65536 | gzip-JSON V3 sidecar |

---

## Common V2 Multipart Conversion Steps

1. **Classify.** `classifyObject` reads `x-amz-meta-armor-version` (absent → `non_armor`, unparseable → `malformed`, `3` → already migrated), checks the version against `includeVersions` (not listed → categorized `v2`, skipped), then the `x-amz-meta-armor-multipart` flag → category `v2_multipart`, migrate.
2. **Validate metadata.** The wrapped DEK may carry the `v2:<fingerprint>:` prefix (stripped before base64 decode) or be bare; IV must be valid base64. Failures are recorded, not retried.
3. **Unwrap DEK.** AES-KWP unwrap with the MEK (`crypto.UnwrapDEK`); derive the HMAC key via HKDF-SHA256(DEK, `"armor-hmac-v1"`) — the same derivation V1 uses.
4. **Load the sidecar.** GET `.armor/hmac/<sha256hex(key)>`. Missing → fail (`missing_sidecar`).
5. **Decrypt.** Concatenated ciphertext treated as **one continuous block sequence**: per-block HMAC verification over absolute (global) block indices, then CTR decryption with counter `IV[0:12] ‖ BE32(globalIndex × blockSize/16)`. HMAC mismatch → fail (`integrity_check_failed`).
6. **Hash the plaintext.** SHA-256 over the recovered plaintext; this value (not the source metadata's) is written to the new metadata and re-checked after upload.
7. **Route by size.** > 5 MiB → `uploadAsMultipart`; otherwise → `encryptAsSingle`. The source layout plays no part in this decision.
8. **Re-encrypt.** Fresh random 32-byte DEK (AES-KWP-wrapped, `v2:<16-hex fingerprint>:<base64>`) and fresh random 16-byte IV; V3 part-namespaced counters; per-block HMACs into the trailer block table (single-PUT) or the gzip-JSON sidecar (multipart).
9. **Re-emit metadata.** All `x-amz-meta-armor-*` rebuilt per the rules above; `part-size`, `etag`, `content-type`, `key-id`, `compressed`, `compression` carried over when present; `multipart` set only by the multipart output path.
10. **Verify.** Read back, assert version == write version, decrypt via `decryptSingleObject`, compare SHA-256; abort the migration record on mismatch.

---

## V1 vs V2 Multipart: What Actually Differs

For the migrator the two versions share almost the entire pipeline (same classification shape, same sidecar, same decrypt entry point, same output routing). The differences:

| Aspect | V1 multipart | V2 multipart |
|---|---|---|
| Counter block | `IV[0:12] ‖ BE32(blockIndex)` — stride 1 | `IV[0:12] ‖ BE32(blockIndex × blockSize/16)` — stride 4096 at 64 KiB |
| Cross-part counters | Every part restarts at block index 0 → **part k block j reuses part 0's keystream** (the defect migration remediates) | One continuous stream via cumulative offsets → no reuse, but parts are *interdependent* |
| Write-path machinery | Per-part independent encryption | `EncryptWithStartingCounter` (uniform, ADR-005) / `OffsetEncryptor` cumulative byte offsets (non-uniform, ADR-011) |
| Counter-space ceiling | 2^32 blocks (≈256 TiB at 64 KiB) — not binding | 2^32 AES counters ÷ 4096 per block = **64 GiB** — binding, V2-specific |
| Why ADR-010/ADR-011 exist | Final-part length inference + cumulative pinning | Same (the cumulative stream is what needs the bookkeeping); V3 removes the reason for both |
| Wrapped DEK metadata | Bare base64 of AES-KWP body | `v2:<fingerprint>:<base64>` (both accepted by the migrator's validator) |
| Block 0 keystream | `IV[0:12] ‖ BE32(0)` | identical — V1 and V2 first blocks are byte-identical for the same DEK/IV |
| HMAC key / table format | HKDF-SHA256(DEK, `"armor-hmac-v1"`), flat global-index sidecar | **identical** |
| Migration motivation | Fix keystream reuse | Structural: per-part independence, new integrity formats, ceiling removal |
| Fixture miniatures | Production-compatible ciphertext (`generate_fixtures.go` `encryptV1` matches `makeCounter`) | **Not** production-compatible — see *Known Divergences* #1 |
| Negative goldens | Two dedicated entries (sidecar corrupt / missing) | None — the shared failure modes are only golden-documented under V1 |

---

## Known Divergences To Keep In Mind When Writing Tests

These are factual observations about the current tree (2026-09-03). Re-verify against HEAD before relying on them; #1–#4 are shared with the V1 fixtures and are restated here with V2-specific consequences.

1. **The V2 fixture ciphertext does not match the production V2 counter derivation — for either generator.** Production derives `counter_block = IV[0:12] ‖ BE32(blockIndex × stride)` (`internal/crypto/encryptor.go:219-236`), but both generators XOR with `uint32LE(blockIndex × stride)` zero-padded into an all-zero counter block, **with no IV** (`generate_fixtures.go:944-948`, `standalone_generator.go:278-280`). Verified empirically against the on-disk miniature (DEK `02…21`, IV `03…12`, 94-byte plaintext): the fixture bytes are reproduced by the generator's counter, decrypting them under the production counter yields garbage, and the production derivation would instead produce `5f 02 2e fa 85 70 2c 62 5d 37 89 75 bd 7a 7b 4f` — exactly the V1 miniature's first bytes, because V1 and V2 coincide at block 0. Consequences: (a) sidecar HMACs still verify (both sides hash the same ciphertext bytes with the same key), so decryption *succeeds*; (b) the recovered plaintext is not the fixture plaintext, so any test asserting `metadata.json.plaintext_sha256` preservation fails; (c) the migrator itself does not compare against the source's `sha256` metadata — it recomputes SHA-256 from what it decrypted and writes that — so it reports **success** while migrating garbage. The V1 miniatures do not have this problem (`encryptV1` matches production). Fixture-driven V2 tests must either fix the generator counter or assert only against the fixtures' own self-consistent values.
2. **`v3_expected` in fixture metadata describes the source layout, not the migrator's output.** `part_count`/`blocks_per_part` are computed by the generators from the source part size; the migrator re-routes output by plaintext size, so the 94-byte and 256 KiB fixtures convert to V3 *single-PUT* objects even though `v3_expected.is_multipart` is `true`. Assert SHA-256 preservation and V3 readability; treat `v3_expected` as source-layout documentation.
3. **`v3-golden-outcomes.json` does not match the committed fixture bytes.** The golden file describes `v2_multipart_variable_final_part` as a 3-part [5, 5, 1] MiB source (`blocks_per_part [80, 80, 16]`) and `v2_multipart_non_uniform_parts` as 4 parts [5, 10, 5, 2] MiB (`[80, 160, 80, 32]`), while the committed bytes are a five-3-MiB-part source (`[48, 48, 48, 48, 48]`) and a [1, 2, 12] MiB source (`[16, 32, 192]`) respectively. Only the uniform entry (3 × 5 MiB, `[80, 80, 80]`) matches. The committed fixture bytes and their `metadata.json` are authoritative; the golden file is illustrative.
4. **The migrator's re-encryption calls predate the V3-specific encryptors** (shared with V1). `encryptAsSingle`/`uploadAsMultipart` use the generic `Encryptor.Encrypt` with `fm.currentWriteVersion` (3); `makeCounter` has no Version3 branch, so a V3-labeled generic encryptor falls into the V1-style counter branch, and `uploadAsMultipart` calls `Encrypt` per part, which restarts the block index per part and discards the returned HMAC tables. Fixture-driven tests should verify that migrated objects actually decrypt under the V3 read path; this document's "expected V3 output" describes the V3 format the fixtures are meant to validate. Post-migration verification also assumes a single-PUT output (`migrateObject` always re-reads via `decryptSingleObject`), and the old HMAC sidecar is neither rewritten nor deleted.
5. **The migrator's multipart decrypt has no placeholder-HMAC exemption.** `Decryptor.Decrypt` verifies every sidecar slot strictly (`internal/crypto/decryptor.go:70-108`), while the server's streaming read path and `DecryptRange` skip all-zero ADR-011 boundary placeholders (`decryptor.go:158`, `handlers.go:1508`). A real ADR-011 V2 object carrying zero placeholders would migrate as an HMAC failure even though reads of the same object succeed. The fixture generators never emit placeholders, so the on-disk fixtures do not exercise this gap.
6. **Two generator implementations with different crypto, writing into the same directories.** `standalone_generator.go` (which produced the committed 15 MiB `v2_multipart/*` bytes and `generated_fixtures/*`) wraps DEKs with AES-GCM (60 bytes) behind an 8-hex fingerprint, derives its HMAC key as `HMAC-SHA256(DEK, "armor-hmac-key")` instead of HKDF, and uses the LE no-IV counters of #1. `generate_fixtures.go` (which produced the current 94-byte miniatures) uses ARMOR's own AES-KWP wrap, 16-hex fingerprint and HKDF HMAC key — but still the LE no-IV V2 counter of #1. Whichever generator ran last owns `v1_multipart/*` and `v2_multipart/*`, which is why the miniatures and the committed full-scale bytes have different metadata shapes (16- vs 8-hex fingerprint; `x-amz-meta-armor-etag` present in all three miniatures — every `generate_fixtures.go` V2 multipart generator sets it — but absent from all three standalone full-scale variants). Additionally, the standalone uniform generator encrypts the whole object as one stream (production-shaped), while its variable-final and non-uniform generators restart the counter **per part** via `encryptMultipart` (`standalone_generator.go:725-756`) — for a multi-part object that reproduces the cross-part keystream reuse V2 was supposed to have fixed, so those shapes should not be treated as production V2 semantics.
7. **Output block size is 4096 on the single-PUT path.** `encryptAsSingle` hardcodes `blockSize = 4096` (`format_migration.go:644`), so migrated small objects differ in block size from their V2 source (65536) and from the golden file's `"65536"` expectation, which only holds for the multipart output path.

---

## Testing Coverage

The V2 multipart fixtures validate that migration covers:

✅ Multipart detection via `x-amz-meta-armor-multipart` with `version=2` (no envelope header to inspect)  
✅ `v2_multipart` classification, including the `includeVersions` scope gate  
✅ Sidecar-located HMAC table loading and global-index verification (shared with V1)  
✅ Plaintext, plaintext-size and SHA-256 preservation across re-encryption (given production-readable input bytes — see divergence #1)  
✅ Structural upgrade: cumulative-counter V2 parts → part-namespaced V3 counters (no behavior change to plaintext)  
✅ V2's 64 GiB counter ceiling removed by the V3 counter layout  
✅ Multipart → single-PUT downgrade for objects under the 5 MiB threshold  
✅ Multipart → multipart preservation with re-split at 5 MiB for large objects  
✅ ADR-005/010/011 source variants (uniform, variable-final, non-uniform; `part-size` present or absent)  
✅ `v2:` fingerprinted wrapped-DEK parsing (and bare-base64 tolerance)  
✅ Metadata re-emission: version 3, fresh DEK/IV, `part-size`/`etag` carry-over rules  
✅ Fail-closed on missing/corrupt sidecar before any write

**Where the conversion is exercised in tests:** `internal/server/format_migration_test.go` (mock-backend migration tests, version-parameterized) and `tests/integration/format_migration_large_object_test.go` (B2-backed legacy-object migration). The V1-labeled multipart test generates its object through ARMOR's own crypto; there is currently no test that migrates the on-disk V2 multipart fixture bytes.

---

## Related Documentation

- **V1 multipart fixtures:** `docs/research/migration/v1-multipart-fixtures.md` (V3 target-format details live there)
- **V1 single-PUT fixtures:** `docs/research/migration/v1-single-part-fixtures.md`
- **V2 single-PUT fixtures:** `docs/research/migration/v2-single-part-fixtures.md`
- **Migration error handling:** `docs/research/migration/migration-error-handling-flow.md`
- **ADRs:** `docs/adr/005-ctr-counter-stride-fix.md` (the V2 stride fix), `docs/adr/010-barman-multipart-incompatibility.md` (variable final part), `docs/adr/011-barman-stays-on-armor-non-uniform-multipart.md` (non-uniform parts)
- **Golden outcomes:** `tests/fixtures/migration/v3-golden-outcomes.json` (+ `.yml`)
- **Fixture generators:** `tests/fixtures/migration/generate_fixtures.go` (ARMOR-crypto-based, produced the current miniatures), `tests/fixtures/migration/standalone_generator.go` (independent crypto, produced the committed full-scale bytes)
- **Migration implementation:** `internal/server/format_migration.go`, `cmd/armor/cmd_migrate.go`
- **V2 crypto primitives:** `internal/crypto/encryptor.go` (`makeCounter`, `EncryptWithStartingCounter`), `internal/crypto/offset_encryptor.go` (`EncryptFromOffset`), `internal/crypto/decryptor.go` (`Decrypt`, `makeCounter`)
- **V3 crypto primitives:** `internal/crypto/v3.go`, `internal/crypto/encryptor.go` (`EncryptV3`, `EncryptPartV3`), `internal/crypto/block_table_v3.go`, `internal/backend/multipart.go` (`HMACTableSidecarV3`)

---

**Document End**
