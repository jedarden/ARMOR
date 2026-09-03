# V1 Multipart Fixture V3 Conversions

This document catalogs all V1 multipart fixtures and their expected V3 conversion outcomes, including the part-by-part transformation of every part.

**Document Version:** 1.0  
**Generated:** 2026-09-03  
**Scope:** All V1 multipart fixtures in `tests/fixtures/migration/v1_multipart/` and the V1 multipart entry in `tests/fixtures/migration/generated_fixtures/`

**Companion documents:**

- `v1-single-part-fixtures.md` — V1 single-PUT fixtures (same directory)
- `v2-single-part-fixtures.md` — V2 single-PUT fixtures (same directory)

---

## V1 Multipart Source Format Overview

### Stored object layout (`stored_ciphertext.bin`)

Unlike V1 **single-PUT** objects, V1 multipart objects carry **no envelope header** and **no inline HMAC table**. The stored blob is the raw concatenation of the encrypted parts:

```
Offset  | Size        | Field
--------|-------------|--------------------------------------------------
0x00    | sum(partLen)| Ciphertext of part 1 (V1-encrypted, see below)
...     | partLen     | Ciphertext of part 2
...     | ...         | ... remaining parts concatenated in part order
```

- Each part is encrypted **independently**: the ARMOR block index restarts at 0 for every part.
- Version, IV, block size, plaintext size and SHA-256 all come from **S3 object metadata**, not from the blob.
- All version/layout detection for multipart objects trusts object metadata (`internal/server/server.go` read path: "Multipart objects have no envelope header — trust metadata version").

### V1 per-block counter derivation (vulnerable)

Each 64 KiB ARMOR block is encrypted with AES-256-CTR. The initial counter block for block `j` of a part is (`internal/crypto/encryptor.go`, `makeCounter`, Version1 branch):

```
counter_block = IV[0:12] || uint32BE(blockIndex)
```

Two distinct weaknesses follow, and both are what V3 exists to fix:

1. **Within a part:** `blockIndex` increments by 1 per 64 KiB block, but each block consumes `blockSize/16 = 4096` AES-CTR counter values. Block 1 therefore restarts at counter 1, which block 0 already used — adjacent blocks share keystream (a two-time pad *within* one part).
2. **Across parts:** every part restarts at `blockIndex = 0` with the same DEK and the same IV, so **part k block j uses exactly the same keystream as part 0 block j**. XORing the two ciphertexts cancels the keystream. This is the multipart-specific escalation of the V1 bug.

### V1 sidecar layout (`sidecar.bin`)

Multipart objects keep their per-block HMAC table in a **sidecar object**, not in the blob:

- **Path:** `.armor/hmac/<sha256hex(object_key)>`. The fixtures use the nominal key `test-object-key`, whose hash is `3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d`.
- **Content:** bare concatenation of 32-byte HMAC-SHA256 values, one per 64 KiB block, in **global block order** (part 0's blocks first, then part 1's, …). The generator computes these with a global running index across parts (`computeCombinedHMACTable` in `tests/fixtures/migration/generate_fixtures.go`), while encryption itself restarts the index per part.
- **HMAC input:** `HMAC-SHA256(hmacKey, uint32BE(blockIndex) || ciphertext_block)` with `hmacKey = HKDF-SHA256(DEK, info="armor-hmac-v1")` (`internal/crypto/hkdf.go`).
- **Size:** `32 × ceil(plaintext_length / 65536)` bytes — independent of part boundaries.

### V1 multipart object metadata

| Field | Meaning in V1 multipart fixtures |
|---|---|
| `x-amz-meta-armor-version` | `"1"` |
| `x-amz-meta-armor-multipart` | `"true"` — the flag that routes reads and migration to the sidecar path |
| `x-amz-meta-armor-part-size` | Nominal uniform part size. Present for uniform and variable-final layouts; **absent for non-uniform (ADR-011)** |
| `x-amz-meta-armor-wrapped-dek` | Bare base64 of the AES-KWP-wrapped DEK (40 bytes → 56 base64 chars). No `v2:` fingerprint prefix in V1 |
| `x-amz-meta-armor-iv` | Base64 of the 16-byte object IV (fixtures: `AwQFBgcICQoLDA0ODxAREg==` = `03 04 … 12`) |
| `x-amz-meta-armor-block-size` | `"65536"` (64 KiB) |
| `x-amz-meta-armor-plaintext-size` | Total plaintext size across all parts |
| `x-amz-meta-armor-sha256` | SHA-256 of the whole plaintext |

---

## V3 Target Format Overview

### Which V3 layout an object gets

Migration is a **decrypt → re-encrypt → re-upload** pipeline, and the *output* layout is chosen by plaintext size, not by the source layout (`internal/server/format_migration.go`, `migrateObject`):

| Migrated plaintext size | Output path | Resulting V3 object |
|---|---|---|
| ≤ 5 MiB (`multipartThreshold()`) | `encryptAsSingle` | **V3 single-PUT**: `header(64) ‖ encrypted blocks ‖ trailer block table`, block size **4096** |
| > 5 MiB | `uploadAsMultipart` | **V3 multipart**: 5 MiB parts via a real multipart upload, block size **65536**, `multipart=true`, `part-size=5242880` |

### V3 single-PUT layout (the output for every small fixture)

```
Offset  | Size  | Field
--------|-------|-------------------------------------------------------
0x00    | 4     | Magic "ARMR"
0x04    | 1     | Version = 0x03
0x05    | 1     | BlockSizeLog2 (4096 → 12)
0x06    | 16    | IV (fresh random value, also written to metadata)
0x16    | 8     | PlaintextSize (uint64 LE, unchanged by migration)
0x1E    | 32    | PlaintextSHA-256 (unchanged by migration)
0x3E    | 2     | Reserved (0)
0x40    | var   | Encrypted blocks (V3 counters, part = 0)
end     | 36×N  | Trailer block table: per block [HMAC(32) ‖ clen(4) BE]
```

- Counter construction (`internal/crypto/v3.go`, `MakeV3Counter`): `IV[0:8] ‖ uint16BE(part) ‖ uint32BE(block) ‖ uint16BE(aesBlock)`. Single-PUT uses `part = 0`; the `aesBlock` field strides inside each 64 KiB/4 KiB block, eliminating both V1 weaknesses.
- Block table entry (`internal/crypto/block_table_v3.go`): 32-byte HMAC + 4-byte ciphertext length; the high bit of `clen` flags zstd compression (never set by migration — `compression_used: false`).
- V3 block HMAC input: `HMAC-SHA256(hmacKey, uint16BE(part) ‖ uint32BE(block) ‖ ciphertext_block)` (`ComputeV3BlockHMAC`).
- The old HMAC sidecar is **not** rewritten and the object no longer references one; sidecar lifecycle (delete/garbage-collection) is outside the migration write path.

### V3 multipart layout (the output for large objects)

Each part is encrypted with `crypto.EncryptPartV3(dek, iv, partNumber, blockSize, plaintext)` (`internal/crypto/encryptor.go:602`):

- Part numbers 1..10000 are carried **inside the counter block** (`uint16BE(part)`), so parts are independent streams that can be uploaded in any order with no size pinning and no cumulative-offset bookkeeping (the ADR-010/ADR-011 constraints disappear).
- Block index restarts at 0 **within each part namespace** — safe now because the part number disambiguates the counter.
- Per-(part, block) HMACs are persisted in a **gzip-compressed JSON sidecar** (`HMACTableSidecarV3`, `SaveHMACTableV3` in `internal/backend/multipart.go`) instead of the flat V1 table.

### V3 output metadata (`buildNewMetadata`)

All `x-amz-meta-armor-*` fields are dropped and re-emitted; non-ARMOR metadata is copied through:

| Field | Value after migration |
|---|---|
| `x-amz-meta-armor-version` | `"3"` (`currentWriteVersion`) |
| `x-amz-meta-armor-wrapped-dek` | `v2:<8-hex MEK fingerprint>:<base64>` of a **freshly generated** 32-byte DEK, AES-KWP-wrapped (40 bytes) |
| `x-amz-meta-armor-iv` | Base64 of a **freshly generated** 16-byte IV |
| `x-amz-meta-armor-block-size` | `"4096"` (single-PUT output) or `"65536"` (multipart output) |
| `x-amz-meta-armor-plaintext-size` | Unchanged (plaintext is preserved byte-for-byte) |
| `x-amz-meta-armor-sha256` | Unchanged |
| `x-amz-meta-armor-multipart` | Set to `"true"` **only** on the multipart output path (never copied from the source) |
| `x-amz-meta-armor-part-size` | `"5242880"` on the multipart output path; the source value is otherwise carried over on the single-PUT path |
| `content-type`, `etag`, `key-id`, `compressed`, `compression` | Preserved from source when present |

After writing, the migrator reads the object back, asserts the version equals the write version, decrypts it, and compares SHA-256.

---

## Conversion Routing For These Fixtures

Every fixture below carries `x-amz-meta-armor-multipart: "true"`, so migration takes the **multipart decrypt path**: unwrap DEK → load HMAC table from `.armor/hmac/<sha256(key)>` → decrypt the concatenated ciphertext with metadata IV/version → HMAC-verify with absolute (global) block indices.

What happens next depends on the plaintext size, which is why the catalog below lists both the **on-disk miniature** (94-byte / 256 KiB) and the **full-scale generator variant** (15 MiB) for each fixture:

- Miniatures (≤ 5 MiB) → **V3 single-PUT** output, despite arriving as multipart.
- Full-scale 15 MiB variants (> 5 MiB) → **V3 multipart** output with three 5 MiB parts.

---

## Fixture Catalog

### 1. uniform_parts

**Location:** `tests/fixtures/migration/v1_multipart/uniform_parts/`  
**Source pattern:** legacy ADR-005 uniform multipart contract  
**Produces the on-disk fixture:** `generate_fixtures.go` → `GenerateV1MultipartUniform(testPlaintext, 5 MiB)`  
**Full-scale variant:** `standalone_generator.go` → `GenerateV1Multipart(15 MiB, 5 MiB)`

#### Input structure (on-disk miniature)

**metadata.json:**

```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v1",
  "source_layout": "multipart-uniform",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [1],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V1 multipart with uniform part sizes (legacy ADR-005 contract)",
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
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-version": "1",
  "x-amz-meta-armor-wrapped-dek": "yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**stored_ciphertext.bin:** 94 bytes, raw ciphertext, no header. First bytes: `5f 02 2e fa 85 70 2c 62 5d 37 89 75 bd 7a 7b 4f …`

**sidecar.bin:** 32 bytes — one HMAC-SHA256 entry: `6e 71 a7 cd 60 f8 9d c0 ba 32 d3 13 13 30 17 33 64 5c 7e af 3b 81 e3 50 76 2e c1 26 01 b5 e8 00`

#### Input part map

| Part | Plaintext range | Size | Blocks | V1 counters used |
|---|---|---|---|---|
| 1 | bytes 0–93 | 94 B (final, short) | 1 | `IV[0:12] ‖ BE32(0)` |

#### Expected V3 output (on-disk miniature, ≤ 5 MiB → single-PUT path)

**Metadata fields:**

```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<fresh random base64>",
  "x-amz-meta-armor-block-size": "4096",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c"
}
```

Note: `x-amz-meta-armor-multipart` is **not** carried over; the migrated object is a V3 single-PUT.

**Object structure:**

- Envelope: `header(64 B) ‖ 94 B ciphertext ‖ trailer block table (36 B × 1 block)`
- `part = 0`, single block, V3 counter `IV[0:8] ‖ BE16(0) ‖ BE32(0) ‖ BE16(0..255)`
- `compression_used`: false; no sidecar

**Part-by-part conversion:**

| Source part | Source blocks (V1) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1, block 0 — counter `IV[0:12]‖BE32(0)`, HMAC in sidecar slot 0 | 94 B | Single-PUT part 0, block 0 | counter `IV[0:8]‖BE16(0)‖BE32(0)‖BE16(aes)`, HMAC in trailer entry 0 |

#### Full-scale generator variant (15 MiB)

With the generator's full-scale plaintext (15 MiB of `byte(i % 256)`, SHA-256 `5f496adeac6ba5b21a781fe82ecff7548255bcec7abd11705187a2c0f058827c`) the fixture becomes 3 uniform 5 MiB parts, and migration takes the **multipart output path**:

| Property | V1 source | V3 output |
|---|---|---|
| part_count | 3 | 3 |
| part sizes | 5242880 × 3 | 5242880 × 3 |
| blocks_per_part | [80, 80, 80] | [80, 80, 80] |
| sidecar | flat table, 7680 B (240 × 32) | gzip-JSON `HMACTableSidecarV3`, per-(part, block) HMACs |
| metadata block-size | 65536 | 65536 |
| metadata part-size | 5242880 | 5242880 |
| multipart flag | true | true |

Per-part counter mapping (the security core of the conversion):

| Source (V1, per-part restart — collides) | Destination (V3, part namespaced) |
|---|---|
| part 1 block j: `IV[0:12]‖BE32(j)` | part 1 block j: `IV[0:8]‖BE16(1)‖BE32(j)‖BE16(aes)` |
| part 2 block j: `IV[0:12]‖BE32(j)` — **same counter as part 1** | part 2 block j: `IV[0:8]‖BE16(2)‖BE32(j)‖BE16(aes)` |
| part 3 block j: `IV[0:12]‖BE32(j)` — **same counter as parts 1–2** | part 3 block j: `IV[0:8]‖BE16(3)‖BE32(j)‖BE16(aes)` |

**Transformation notes**

1. Version 1 → 3; DEK re-wrapped from bare base64 to `v2:<fingerprint>:<base64>`; **new random DEK and IV** (the original DEK is retired).
2. Plaintext, plaintext size and SHA-256 are preserved byte-for-byte; only the encryption layer is rebuilt.
3. Counter namespace changes from "per-part block index, IV-suffixed" to "part-number-namespaced (part, block, aesBlock)" — the cross-part keystream reuse disappears.
4. Part structure is preserved at this size (3 × 5 MiB in, 3 × 5 MiB out) because the output part size (5 MiB) equals the source's nominal part size.
5. HMAC moves from a flat global-index sidecar table to per-(part, block) HMACs identified by `BE16(part)‖BE32(block)`.

---

### 2. variable_final_part

**Location:** `tests/fixtures/migration/v1_multipart/variable_final_part/`  
**Source pattern:** ADR-010 variable final part exemption (Barman compatibility)  
**Produces the on-disk fixture:** `generate_fixtures.go` → `GenerateV1MultipartVariableFinal(testPlaintext, 3 MiB, 2 MiB)`  
**Full-scale variant:** `standalone_generator.go` → `GenerateV1MultipartVariableFinal(15 MiB, 3 MiB, 2 MiB)`

#### Input structure (on-disk miniature)

**metadata.json:**

```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v1",
  "source_layout": "multipart-variable-final",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [1],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V1 multipart with variable final part (ADR-010 exemption pattern)",
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
  "x-amz-meta-armor-version": "1",
  "x-amz-meta-armor-wrapped-dek": "yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**Key metadata difference vs `uniform_parts`:** `x-amz-meta-armor-part-size` is `"3145728"` (3 MiB) — the *nominal uniform* part size, not the final part's size. V1 metadata never records the final part's actual length; it is derived by subtraction.

**stored_ciphertext.bin / sidecar.bin:** byte-identical to `uniform_parts` (same plaintext, same keys, same single block) — the fixture differs only in the declared layout and part-size metadata.

#### Input part map (on-disk miniature)

`splitIntoVariableFinalParts(94 B, 3 MiB, 2 MiB)`: 94 ≤ 2 MiB, so the whole object is the final part.

| Part | Plaintext range | Size | Blocks | V1 counters used |
|---|---|---|---|---|
| 1 (final) | bytes 0–93 | 94 B | 1 | `IV[0:12] ‖ BE32(0)` |

#### Expected V3 output (on-disk miniature, ≤ 5 MiB → single-PUT path)

Identical output shape to `uniform_parts`:

- Metadata: `version=3`, `block-size=4096`, fresh `iv`/`wrapped-dek` (`v2:` format), `plaintext-size=94`, `sha256` unchanged, `multipart` flag dropped. The stale source `part-size=3145728` value is carried over by `buildNewMetadata` on this path.
- Object: `header(64) ‖ 94 B ciphertext ‖ trailer block table (36 B)`, part 0, block 0.

**Part-by-part conversion:**

| Source part | Source blocks (V1) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1 (final, 94 B), block 0 | counter `IV[0:12]‖BE32(0)`, sidecar HMAC slot 0 | Single-PUT part 0, block 0 | counter `IV[0:8]‖BE16(0)‖BE32(0)‖BE16(aes)`, trailer entry 0 |

#### Full-scale generator variant (15 MiB)

`splitIntoVariableFinalParts(15 MiB, 3 MiB, 2 MiB)` yields **five 3 MiB parts** — with 15 MiB divisible by 3 MiB, the splitter emits five full uniform parts and never produces a smaller final part (remaining == uniformPartSize is not "< uniformPartSize"). At 15 MiB total the migrated object exceeds the 5 MiB threshold, so the output is a V3 multipart upload with three 5 MiB parts:

| Property | V1 source | V3 output |
|---|---|---|
| part_count | 5 | 3 |
| part sizes | 3145728 × 5 | 5242880 × 3 |
| blocks_per_part | [48, 48, 48, 48, 48] | [80, 80, 80] |
| sidecar | flat table, 7680 B | gzip-JSON V3 sidecar |
| metadata part-size | 3145728 | 5242880 (rewritten by the multipart output path) |

**Transformation notes**

1. The ADR-010 "variable final part" exemption exists because V1/V2 multipart needed the final part's length inferred and pinned the counter stream cumulatively. V3 removes the reason for the exemption: parts are independent streams (part number in the counter), so any part size mix is legal.
2. Part structure is **not** preserved here (5 × 3 MiB in → 3 × 5 MiB out); part count and sizes in `v3_expected` describe the *source* layout. Validation should assert plaintext SHA-256 equality and V3 readability, not part-count equality.
3. Because the source's final part was not block-aligned in general, V1 verification of such objects can carry zero ("placeholder") HMAC entries for boundary blocks; V3's per-(part, block) HMACs eliminate the placeholder concept.

---

### 3. non_uniform_parts

**Location:** `tests/fixtures/migration/v1_multipart/non_uniform_parts/`  
**Source pattern:** ADR-011 non-uniform multipart (Barman `chunk_size + N*512` style)  
**Produces the on-disk fixture:** `generate_fixtures.go` → `GenerateV1MultipartNonUniform(testPlaintext, [1 MiB, 2 MiB, 3 MiB])`  
**Full-scale variant:** `standalone_generator.go` → `GenerateV1MultipartNonUniform(15 MiB, [1 MiB, 2 MiB, 3 MiB])`

#### Input structure (on-disk miniature)

**metadata.json:**

```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v1",
  "source_layout": "multipart-nonuniform",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [1],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V1 multipart with non-uniform part sizes (ADR-011 pattern)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**

```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-multipart": "true",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-version": "1",
  "x-amz-meta-armor-wrapped-dek": "yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**Key metadata difference:** there is **no `x-amz-meta-armor-part-size` at all** — non-uniform layout cannot be summarized by one number. This is the fixture that exercises "multipart without part-size metadata" during migration.

**stored_ciphertext.bin / sidecar.bin:** byte-identical to the other two miniatures.

#### Input part map (on-disk miniature)

`splitIntoNonUniformParts(94 B, [1 MiB, 2 MiB, 3 MiB])`: the first claimed size already covers the object, so a single 94-byte part is emitted.

| Part | Plaintext range | Size | Blocks | V1 counters used |
|---|---|---|---|---|
| 1 | bytes 0–93 | 94 B | 1 | `IV[0:12] ‖ BE32(0)` |

#### Expected V3 output (on-disk miniature, ≤ 5 MiB → single-PUT path)

Same output shape as the other miniatures:

- Metadata: `version=3`, `block-size=4096`, fresh `iv`/`wrapped-dek` (`v2:` format), `plaintext-size=94`, `sha256` unchanged, `multipart` dropped, and — because the source had no `part-size` — no `part-size` is emitted either.
- Object: `header(64) ‖ 94 B ciphertext ‖ trailer block table (36 B)`, part 0, block 0.

**Part-by-part conversion:**

| Source part | Source blocks (V1) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1 (94 B), block 0 | counter `IV[0:12]‖BE32(0)`, sidecar HMAC slot 0 | Single-PUT part 0, block 0 | counter `IV[0:8]‖BE16(0)‖BE32(0)‖BE16(aes)`, trailer entry 0 |

#### Full-scale generator variant (15 MiB)

The claimed sizes are `[1 MiB, 2 MiB, remainder]`, so part 3 absorbs 12 MiB:

| Property | V1 source | V3 output |
|---|---|---|
| part_count | 3 | 3 |
| part sizes | 1048576, 2097152, 12582912 | 5242880, 5242880, 5242880 |
| blocks_per_part | [16, 32, 192] | [80, 80, 80] |
| sidecar | flat table, 7680 B | gzip-JSON V3 sidecar |
| metadata part-size | absent | 5242880 |

**Transformation notes**

1. V1/V2 handled non-uniform parts with cumulative-offset encryption (`OffsetEncryptor` / `EncryptWithStartingCounter`, ADR-011) plus zero-placeholder HMACs for boundary blocks. V3 replaces all of it with per-part counters and per-(part, block) HMACs.
2. Part boundaries are **not** preserved: the V3 multipart output always re-splits at 5 MiB. Only the total plaintext is invariant.
3. This is the only V1 multipart fixture whose source metadata lacks `part-size`; migration must not require it.

---

### 4. v1-multipart-uniform (generated)

**Location:** `tests/fixtures/migration/generated_fixtures/v1-multipart-uniform/`  
**Produced by:** `standalone_generator.go` → `GenerateV1Multipart(mediumPlaintext, 5 MiB)` (256 KiB medium plaintext)

#### Input structure

**metadata.json:**

```json
{
  "plaintext_sha256": "2312394bd99545d9de131c24efb781e765ac1aec243f2ed9347597a793a415e9",
  "plaintext_length": 262144,
  "source_version": "v1",
  "source_layout": "multipart-uniform",
  "v3_expected": {
    "is_multipart": true,
    "part_count": 1,
    "blocks_per_part": [4],
    "compression_used": false,
    "sidecar_path": ".armor/hmac/3c94d277a57c4078ce27e75ee1e8f605c2b382dc3108996af09e6c175cd38c9d"
  },
  "description": "V1 multipart object with HMAC sidecar",
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
  "x-amz-meta-armor-version": "1",
  "x-amz-meta-armor-wrapped-dek": "hYHtzNISt/ylaRF64mgFxBo7tckG5Qc8+1xsb5Bsl5hMh2AVcdtazEj3AnvUsGl1vp087CJs1VkoGKBN"
}
```

Note the wrapped DEK is 60 bytes of binary (base64 80 chars, no padding) — the standalone generator wraps with AES-GCM (`nonce(12) ‖ ciphertext(32) ‖ tag(16)`), whereas ARMOR's `crypto.WrapDEK` uses AES-KWP and produces 40 bytes. This is intentional: the standalone generator is an adversarial, independent implementation (see `tests/fixtures/migration/README.md`).

**stored_ciphertext.bin:** 262144 bytes, raw ciphertext (4 blocks × 64 KiB), no header. Plaintext is the deterministic `byte(i % 256)` pattern.

**sidecar.bin:** 128 bytes — 4 × 32-byte HMAC entries (one per block).

#### Input part map

| Part | Plaintext range | Size | Blocks | V1 counters used |
|---|---|---|---|---|
| 1 | bytes 0–262143 | 256 KiB (final, short) | 4 | `LE32(blockIndex)` zero-padded — see note |

**Standalone-generator crypto note:** this fixture's generator derives counters and HMAC keys independently of ARMOR — counter block `uint32LE(blockIndex)` zero-padded (no IV prefix) and HMAC key `HMAC-SHA256(DEK, "armor-hmac-key")` instead of HKDF-SHA256(DEK, info=`"armor-hmac-v1"`). Its purpose is cross-validation of the generator itself; run under the production V1 read path its HMAC verification will not succeed. Treat it as generator-vs-generator golden material, not as a migrator input expectation.

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

- `header(64) ‖ 262144 B ciphertext ‖ trailer block table (36 B × 64 blocks = 2304 B)` at block size 4096
- `part = 0`; blocks 0..63 with V3 counters
- `compression_used`: false; no sidecar; `multipart` flag dropped
- Source `part-size=5242880` is carried over into the migrated metadata by `buildNewMetadata` even though the output is single-PUT

**Part-by-part conversion:**

| Source part | Source blocks (V1) | Destination | Destination blocks (V3) |
|---|---|---|---|
| Part 1 (256 KiB), blocks 0–3 (64 KiB each) | sidecar HMAC slots 0–3 | Single-PUT part 0, blocks 0–63 (4 KiB each) | trailer entries 0–63, counter `IV[0:8]‖BE16(0)‖BE32(b)‖BE16(aes)` |

**Transformation notes**

- Demonstrates the multipart→single-PUT downgrade path for a multi-block object (the 94-byte miniatures only exercise a single block).
- Output block count (64) differs from source block count (4) because `encryptAsSingle` uses 4 KiB blocks; block count is a derived property, not an invariant.

---

## Negative-Path V1 Multipart Expectations

`tests/fixtures/migration/v3-golden-outcomes.json` defines two V1 multipart failure cases (no on-disk fixture directories; they are golden expectations for corrupt variants, generated by the `malformed/` generator functions):

| Golden entry | Defect | Expected outcome | Expected failure |
|---|---|---|---|
| `v1_multipart_sidecar_corrupt` | Bit-flipped sidecar HMAC table | failure (`integrity_check_failed`) | HMAC verification failed — corrupt sidecar cannot be validated |
| `v1_multipart_sidecar_missing` | Sidecar object absent | failure (`missing_sidecar`) | Sidecar not found — cannot verify HMAC integrity |

Both cases must fail **before** any re-encrypted bytes are written: the multipart decrypt path verifies HMACs while decrypting, and the migrator writes the replacement object only after decryption succeeds.

---

## Conversion Summary Matrix

| Fixture | Source layout | Plaintext | Source parts × blocks | Migrated output layout | Output block size | Sidecar after migration |
|---|---|---|---|---|---|---|
| `v1_multipart/uniform_parts` | multipart-uniform | 94 B | 1 × [1] | V3 single-PUT | 4096 | none (old sidecar orphaned) |
| `v1_multipart/variable_final_part` | multipart-variable-final | 94 B | 1 × [1] | V3 single-PUT | 4096 | none |
| `v1_multipart/non_uniform_parts` | multipart-nonuniform | 94 B | 1 × [1] | V3 single-PUT | 4096 | none |
| `generated_fixtures/v1-multipart-uniform` | multipart-uniform | 256 KiB | 1 × [4] | V3 single-PUT | 4096 | none |
| full-scale `uniform_parts` (15 MiB) | multipart-uniform | 15 MiB | 3 × [80,80,80] | V3 multipart, 3 × 5 MiB | 65536 | gzip-JSON V3 sidecar |
| full-scale `variable_final_part` (15 MiB) | multipart-variable-final | 15 MiB | 5 × [48×5] | V3 multipart, 3 × 5 MiB | 65536 | gzip-JSON V3 sidecar |
| full-scale `non_uniform_parts` (15 MiB) | multipart-nonuniform | 15 MiB | 3 × [16,32,192] | V3 multipart, 3 × 5 MiB | 65536 | gzip-JSON V3 sidecar |

---

## Common V1 Multipart Conversion Steps

1. **Classify.** `classifyObject` reads `x-amz-meta-armor-version` (absent → `non_armor`, unparseable → `malformed`, `3` → already migrated) and the `x-amz-meta-armor-multipart` flag; V1 + multipart → category `v1_multipart`, migrate.
2. **Validate metadata.** Wrapped DEK (with or without `v2:` prefix) and IV must be valid base64 before anything is fetched; failures are recorded, not retried.
3. **Unwrap DEK.** AES-KWP unwrap with the MEK (`crypto.UnwrapDEK`, 40-byte wrapped form); derive the HMAC key via HKDF-SHA256.
4. **Load the sidecar.** GET `.armor/hmac/<sha256hex(key)>`. Missing → fail (`missing_sidecar`); HMAC mismatch during decryption → fail (`integrity_check_failed`).
5. **Decrypt.** Concatenated ciphertext, metadata IV, metadata block size, metadata version; per-block HMAC verification over absolute (global) block indices; zero-placeholder HMACs (ADR-011 boundary blocks) are skipped, not treated as corruption.
6. **Hash the plaintext.** SHA-256 over the recovered plaintext; this value is written to the new metadata and re-checked after upload.
7. **Route by size.** > 5 MiB → `uploadAsMultipart`; otherwise → `encryptAsSingle`. The source layout plays no part in this decision.
8. **Re-encrypt.** Fresh random 32-byte DEK (AES-KWP-wrapped, emitted as `v2:<fingerprint>:<base64>`) and fresh random 16-byte IV; V3 per-(part, block, aesBlock) counters; per-block HMACs into the trailer block table (single-PUT) or the gzip-JSON sidecar (multipart).
9. **Re-emit metadata.** All `x-amz-meta-armor-*` rebuilt per the table above; non-ARMOR metadata and content-type/etag/key-id/compression fields preserved.
10. **Verify.** Read back, assert version == write version, decrypt, compare SHA-256; abort the migration record on mismatch.

---

## Version-Specific Multipart Notes

**V1 (these fixtures)**

- Counter block `IV[0:12] ‖ BE32(blockIndex)` with per-part restart: keystream reuse both within a part (adjacent blocks) and across parts (identical counters). This is the primary security defect migration remediates.
- HMAC table in a flat sidecar keyed by global block index; object carries no header, so version/IV/size come exclusively from metadata.
- Part sizes: uniform (ADR-005), variable-final (ADR-010, `part-size` = nominal uniform size), non-uniform (ADR-011, no `part-size` metadata at all).

**V2 multipart (for contrast; see `v2-multipart` fixtures doc)**

- Counter strides by `blockSize/16`, fixing within-object reuse, but multipart V2 still threads one cumulative counter stream through parts (`EncryptWithStartingCounter` / `OffsetEncryptor`) — cross-part independence comes from offset bookkeeping, not from the counter layout, and the 2^32 counter cap bounds object size (64 GiB at 64 KiB blocks).

**V3 (target)**

- Counter block `IV[0:8] ‖ BE16(part) ‖ BE32(block) ‖ BE16(aesBlock)`: part number is inside the counter, so parts are order-independent, size-independent streams; out-of-order uploads are safe.
- HMAC input binds part and block (`BE16(part) ‖ BE32(block) ‖ ciphertext`), making block reordering and part reordering detectable.
- Block size ≤ 1 MiB enforced; counter space is effectively unbounded (no V2 64 GiB ceiling).
- Integrity storage splits by layout: trailer block table (36 B/block) for single-PUT, gzip-compressed JSON sidecar (`HMACTableSidecarV3`) for multipart.

---

## Known Divergences To Keep In Mind When Writing Tests

These are factual observations about the current tree (2026-09-03, working tree at the time of writing). Re-verify against HEAD before relying on them.

1. **`v3_expected` in fixture metadata describes the source layout, not the migrator's output.** `part_count`/`blocks_per_part` are computed by the generator from the source part size; the migrator re-routes output by plaintext size, so the 94-byte and 256 KiB fixtures convert to V3 *single-PUT* objects even though `v3_expected.is_multipart` is `true`. Fixture-driven tests should assert SHA-256 preservation and V3 readability, and treat `v3_expected` as source-layout documentation.
2. **`v3-golden-outcomes.json` does not match the generator's current full-scale parameters.** The golden file describes `v1_multipart_variable_final_part` as a 3-part [5, 5, 1] MiB source with outcome `blocks_per_part [80, 80, 16]`, while the generator produces a 15 MiB / five-3-MiB-part source. The generator (and the committed fixture bytes) are authoritative; the golden file is illustrative.
3. **Output block size is 4096 on the single-PUT path.** `encryptAsSingle` hardcodes `blockSize = 4096` (`format_migration.go:644`), so migrated small objects differ in block size from both their V1 source (65536) and the golden file's `"65536"` expectation, which only holds for the multipart output path.
4. **The migrator's re-encryption calls predate the V3-specific encryptors.** `encryptAsSingle`/`uploadAsMultipart` use the generic `Encryptor.Encrypt` (`internal/crypto/encryptor.go:76`), whose counter construction (`makeCounter`) implements V1/V2 semantics and has no Version3 branch, and whose `computeBlockHMAC` binds only a 32-bit block index — whereas the production V3 write path uses `EncryptV3`/`EncryptV3Stream`/`EncryptPartV3` with trailer block tables and part-namespaced counters (`internal/server/handlers/handlers.go:506, 788, 3440`). Fixture-driven tests should verify that migrated objects actually decrypt under the V3 read path; this document's "expected V3 output" describes the V3 format the fixtures are meant to validate.
5. **Post-migration verification assumes a single-PUT output.** `migrateObject` always re-reads and decrypts via `decryptSingleObject` (`format_migration.go:483`), which expects an envelope header; objects that took the `uploadAsMultipart` path have no header.
6. **The old HMAC sidecar is neither rewritten nor deleted.** After a successful migration the object no longer references it, but the sidecar object remains in the bucket (the integration test `tests/integration/format_migration_large_object_test.go` deletes it explicitly during cleanup).
7. **Two generator implementations with different crypto.** `generate_fixtures.go` (which produced the current on-disk `v1_multipart/*` miniatures) uses ARMOR's own `crypto.WrapDEK`/`crypto.DeriveHMACKey` and ARMOR's V1 counter layout; `standalone_generator.go` (which produced `generated_fixtures/*`) reimplements them independently with different counter padding, HMAC-key derivation, and DEK wrapping (AES-GCM, 60-byte). Fixtures from the two generators are not byte-comparable, and the standalone ones exercise the production read path differently (see the per-fixture note above).

---

## Testing Coverage

The V1 multipart fixtures validate that migration covers:

✅ Multipart detection via `x-amz-meta-armor-multipart` (no envelope header to inspect)  
✅ Sidecar-located HMAC table loading and global-index verification  
✅ Plaintext, plaintext-size and SHA-256 preservation across re-encryption  
✅ Cross-part keystream-reuse remediation (part-namespaced V3 counters)  
✅ Multipart → single-PUT downgrade for objects under the 5 MiB threshold  
✅ Multipart → multipart preservation with re-split at 5 MiB for large objects  
✅ ADR-005/010/011 source variants (uniform, variable-final, non-uniform; `part-size` present or absent)  
✅ Negative paths: missing or corrupt sidecar must fail closed before any write  
✅ Metadata re-emission: version 3, `v2:` wrapped-DEK format, fresh IV, preserved SHA-256

**Where the conversion is exercised in tests:** `internal/server/format_migration_test.go` (`TestFormatMigrationMultipartToSingle` — mock backend, V1 multipart object + sidecar → migrated, version bumped) and `tests/integration/format_migration_large_object_test.go` (`generate_legacy_v1_multipart_object` → `migrate_v1_multipart_to_v3` — B2-backed, target `crypto.Version3`, sidecar cleanup).

---

## Related Documentation

- **V1 single-PUT fixtures:** `docs/research/migration/v1-single-part-fixtures.md`
- **V2 single-PUT fixtures:** `docs/research/migration/v2-single-part-fixtures.md`
- **Migration error handling:** `docs/research/migration/migration-error-handling-flow.md`
- **Golden outcomes:** `tests/fixtures/migration/v3-golden-outcomes.json` (+ `.yml`)
- **Fixture generators:** `tests/fixtures/migration/generate_fixtures.go` (ARMOR-crypto-based, produced the current `v1_multipart/*` bytes), `tests/fixtures/migration/standalone_generator.go` (independent crypto)
- **Migration implementation:** `internal/server/format_migration.go`, `cmd/armor/cmd_migrate.go`
- **V3 crypto primitives:** `internal/crypto/v3.go`, `internal/crypto/encryptor.go` (`EncryptV3`, `EncryptPartV3`), `internal/crypto/block_table_v3.go`, `internal/backend/multipart.go` (`HMACTableSidecarV3`)

---

**Document End**
