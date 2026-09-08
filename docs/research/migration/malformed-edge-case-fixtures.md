# Malformed & Edge-Case Migration Fixtures — Expected V3 Handling

This document catalogs every fixture under `tests/fixtures/migration/malformed/`,
`tests/fixtures/migration/edge_cases/`, and `tests/fixtures/migration/contradictory/`,
and states what the format migrator actually does with each one: the expected
error message, the expected V3 output, or — where the two disagree — the gap
between the fixture's declared intent and observed behavior.

**Scope:** 17 fixtures (13 malformed, 3 edge cases, 1 contradictory).
**Companion docs:** `migration-error-handling-flow.md` (failure plumbing),
`v1-single-part-fixtures.md`, `v1-multipart-fixtures.md`,
`v2-single-part-fixtures.md`, `v2-multipart-fixtures.md` (valid fixtures).

---

## 1. How these expectations were established

Three layers of evidence, all reproducible against the current tree:

1. **Declared intent** — each fixture's `metadata.json` carries
   `expected_migration_outcome` and `expected_failure_reason`, written by the
   fixture generator (`tests/fixtures/migration/standalone_generator.go`).
2. **Observed behavior** — the production `FormatMigrator`
   (`internal/server/format_migration.go`) was run over every fixture exactly
   as shipped, inside a `MockBackend`, with:
   - MEK = `0x01 0x02 … 0x20` (the generator's deterministic MEK)
   - `currentWriteVersion = 3`, `includeVersions = ["1", "2"]`
   - `dryRun = false` (full re-encrypt + read-back verification path)
   - object key `fixtures/<group>/<name>`; multipart sidecars placed at
     `.armor/hmac/<sha256-hex(object key)>` (the path
     `loadHMCTableFromSidecar` derives)
3. **Format-aligned synthesis** — because the shipped `malformed/*` fixtures
   cannot reach their intended defects through the production code path (see
   §3), each defect was also re-created on a **production-format** object
   (built with `crypto.NewEncryptorWithVersion`, `crypto.WrapDEK`,
   `crypto.NewEnvelopeHeaderWithVersion`) and run through the same migrator.
   That yields the error message the migrator emits when it actually reaches
   the defect, and is what will hold once the fixtures are regenerated against
   production crypto.

Success cases were additionally verified by decrypting the migrated object and
comparing its plaintext SHA-256 against the fixture's declared
`plaintext_sha256`.

---

## 2. The migration decision flow (what can happen to a fixture)

Every object passes through the same gates (`Migrate()` → `migrateObject()`):

| Gate | Code | Outcome for the object |
|---|---|---|
| No `x-amz-meta-armor-version` | `Migrate()` | **skipped** (not ARMOR), `SkippedObjects++` |
| Version string unparseable | `ParseARMORMetadata` defaults it to **1** | **not skipped** — enters the V1 migration path |
| Version not in include list | `shouldMigrateVersion` | **skipped**, `SkippedObjects++` |
| Version == target (3) | `Migrate()` | **skipped** (already migrated) |
| `x-amz-meta-armor-multipart: "true"` | `decryptMultipartObject` | sidecar at `.armor/hmac/<sha256(key)>` is the HMAC table; **no envelope header is read** |
| otherwise | `decryptSingleObject` | 64-byte envelope header is read from the object; **metadata version is ignored for decryption** — the header's version byte picks the counter derivation |
| Decrypt error | `Migrate()` loop | **failed**: `FailedObjects++`, `MigrationFailure{Key, Reason, Time}` recorded immediately under `stateMu`, object left untouched, loop continues (best-effort) |
| Decrypt OK | `migrateObject` | re-encrypt at target version, put, read back, decrypt again, compare SHA — then **processed** |

Failure records land in `.armor/migration-state.json` and in the run result;
`armor migrate` prints `Migration had N failures.` and **exits 1** when any
object failed. A failing object never blocks the rest of the bucket, and a
re-run reprocesses everything after `LastKey` (failures are not retried
specially — they just fail again unless the underlying object changed).

Dry-run runs the decrypt and stops (`if dryRun { return nil }` in
`migrateObject`) — so dry-run reports the same failures as a real run, and the
same false "success" for the corruption case in §6.

---

## 3. Ground truth: the shipped `malformed/*` fixtures cannot reach their defects

The malformed fixtures were produced by the *standalone* generator, which
deliberately re-implements the crypto ("adversarial validation"). Its
implementation has diverged from production in four ways, and each divergence
triggers an earlier error than the defect the fixture was built to exercise:

| # | Aspect | Standalone generator | Production (`internal/crypto`) | Consequence |
|---|---|---|---|---|
| 1 | DEK wrap | AES-GCM: `nonce(12) ‖ ct‖tag` = **60 bytes** (`wrapDEK`) | AES-KWP (RFC 5649): **40 bytes** (`WrapDEK`/`UnwrapDEK`) | every `malformed/*` object fails at unwrap before its defect is reached |
| 2 | Header `PlaintextSize` endianness | **big-endian** at bytes 22..30 (`encodeEnvelopeHeader`) | **little-endian** (`DecodeHeader`) | single-PUT objects that get past unwrap misread the size (119 → ≈8.6·10¹⁸), so the HMAC-table fit check fails with a nonsense `need` |
| 3 | HMAC key derivation | `HMAC-SHA256(DEK, "armor-hmac-key")` | `HKDF-SHA256(DEK, info="armor-hmac-v1")` (`DeriveHMACKey`) | multipart objects that get past unwrap fail per-block HMAC verification |
| 4 | CTR counter layout | counter in the **first 4 bytes, little-endian, IV unused** | `IV[0:12] ‖ uint32-BE(counter)` (`makeCounter`) | masked today by #2/#3; would silently corrupt plaintext if they were fixed (see §6) |

**Observed funnel for all 13 `malformed/*` fixtures, as shipped:**

```
processed=1 skipped=0 failed=1
migration failed: failed to decrypt object: failed to unwrap DEK: invalid key length: wrapped DEK must be 40 bytes
```

Every one of them — including `invalid_version_string`, which `ParseARMORMetadata`
treats as version 1 rather than skipping. The object is left byte-identical, a
failure record is written, and the migration continues.

Two of the 13 do reach a genuine defect-shaped error even today, because their
defect is *size-based* and independent of key derivation: `invalid_sidecar_format`
and `truncated_sidecar` (see catalog).

**Implication:** until the malformed fixtures are regenerated against
production crypto, they only prove "the migrator rejects a wrapped DEK of the
wrong length". The catalog below gives both the as-is behavior and the
format-aligned expectation so the fixtures can be checked against intent after
regeneration.

---

## 4. Fixture catalog — `malformed/`

Each entry: input structure, declared expectation, observed behavior as
shipped, expected behavior once the object is format-aligned, and why the case
matters.

Plaintext for the small fixtures is the 119-byte generator string
(`"ARMOR migration test data - V1/V2 to V3 fixture …"`,
SHA-256 `151b2cfb…de377c`); the multipart ones use 2 MiB of
`byte(i % 256)` (SHA-256 `91d3beb8…bd1938`), 512 KiB parts, 64 KiB blocks.

---

### 4.1 `malformed/invalid_version_string`

- **Input:** genuine single-PUT object whose `x-amz-meta-armor-version` is the
  string `"not-a-number"`. Everything else (wrapped DEK, IV, envelope) is
  intact.
- **Declared:** failure — "invalid version format".
- **Observed (as shipped):** **failed=1** at DEK unwrap (generator divergence #1).
  Notably it is *not* skipped: `ParseARMORMetadata` silently defaults an
  unparseable version to `1`, so the object enters the V1 migration path.
- **Format-aligned expectation:** **success**. A genuine V1 object with a
  garbage version label decrypts correctly (decryption follows the envelope
  header, not the label), and the label is rewritten to `3`. Verified with a
  production-format synthesis: `processed=1 failed=0`, migrated plaintext SHA
  matches. The only paths that treat a bad version string as an error are the
  direct-parse fallbacks in `Migrate()`/`countObjects`, which require
  `ParseARMORMetadata` to have returned `!ok` (no wrapped DEK at all).
- **Behavior:** transform (label corrected).
- **Recovery:** none needed.
- **Why it matters:** a torn or hand-edited metadata write must not make an
  object un-migratable; ARMOR chooses robustness. The flip side: **version
  metadata is advisory**, so it cannot be relied on to route objects that lie
  about their version in the other direction (see 4.5/4.6).

### 4.2 `malformed/envelope_version_mismatch`

- **Input:** envelope header says `0x01`; `x-amz-meta-armor-version` says
  `"2"`. Ciphertext produced with V2 counter derivation.
- **Declared:** failure — "envelope header version (1) != metadata version (2)".
- **Observed (as shipped):** **failed=1** at DEK unwrap.
- **Format-aligned expectation:** the *declared* error never happens — there
  is **no code path that compares the header version to the metadata
  version**. Outcome depends on which one is lying:
  - genuine ciphertext matching the **header** → success, label corrected (see 4.5);
  - ciphertext matching the **metadata** label instead of the header (this
    fixture's shape) → decryption uses the wrong counter derivation — which,
    on this 119-byte **single-block** object, still yields the correct
    plaintext (V1 and V2 counters coincide on block 0; the production-format
    synthesis of exactly this shape migrates green with a matching SHA), but
    on any **multi-block** object the same lie silently corrupts every block
    after the first (see §6).
- **Behavior:** transform, with a corruption hazard that only manifests on
  multi-block objects.
- **Recovery:** none automatic. Detection requires the plaintext-SHA
  cross-check proposed in §7.
- **Why it matters:** a partial metadata rewrite (version bumped, envelope not
  re-encrypted) is exactly this shape.

### 4.3 `malformed/corrupted_hmac_table`

- **Input:** single-PUT object; the first byte of the trailing 32-byte HMAC
  entry is XOR-flipped (`0xFF`) in the stored envelope.
- **Declared:** failure — "HMAC verification fails due to corrupted table".
- **Observed (as shipped):** **failed=1** at DEK unwrap.
- **Format-aligned expectation:** **failure** with the declared mechanism:

  ```
  migration failed: failed to decrypt object: failed to decrypt: block 0: HMAC verification failed
  ```

  (`Decryptor.Decrypt` verifies each block's HMAC before decrypting it;
  `ErrHMACMismatch` is wrapped with the block index.)
- **Behavior:** fail, object untouched.
- **Recovery:** only from a backup/replica — the HMAC protects integrity, it
  does not repair. A re-run fails identically.
- **Why it matters:** bit rot in the stored HMAC table must abort
  re-encryption; migrating would otherwise republish data whose integrity can
  no longer be proven.

### 4.4 `malformed/corrupted_wrapped_dek_tag`

- **Input:** wrapped-DEK metadata with bit flips in the final bytes (the
  generator targets the GCM tag region); still valid base64 of the correct
  length for its format.
- **Declared:** failure — "DEK unwrap fails GCM authentication".
- **Observed (as shipped):** **failed=1** at the 40-byte length check —
  production never reaches an authentication check because the blob is 60
  bytes, not 40.
- **Format-aligned expectation:** **failure** with a KWP integrity error:

  ```
  migration failed: failed to decrypt object: failed to unwrap DEK: key unwrap failed: invalid AIV
  ```

  (production `UnwrapDEK` validates the RFC 5649 AIV/MLI after unwrapping;
  production does not use GCM for DEK wrapping at all).
- **Behavior:** fail, object untouched.
- **Recovery:** none — the DEK is unrecoverable from a corrupted wrap.
  Restore from backup, or re-encrypt from the source outside ARMOR.
- **Why it matters:** a single flipped bit in metadata must fail *closed*.
  This is the one defect where the as-is error (length) and the aligned error
  (AIV) differ in wording but agree in outcome.

### 4.5 `malformed/v1_object_v2_metadata`

- **Input:** fully self-consistent genuine V1 object (V1 header, V1 counter
  derivation, valid HMAC table) whose metadata claims `"2"`.
- **Declared:** failure — "V2 counter derivation cannot decrypt V1 ciphertext".
- **Observed (as shipped):** **failed=1** at DEK unwrap.
- **Format-aligned expectation:** **success, correct plaintext** (synthesis:
  `processed=1 failed=0`, SHA matches). The declared failure is based on a
  wrong model of the code: decryption selects the counter derivation from the
  **envelope header's** version byte (`NewDecryptorWithVersion(…,
  header.Version)` in `decryptSingleObject`), never from
  `x-amz-meta-armor-version`. The metadata label only steers include-list
  routing and gets rewritten to `3`.
- **Behavior:** transform (label corrected).
- **Recovery:** none needed.
- **Why it matters:** the header wins, so a mislabeled-but-honest object is
  safe. The dangerous direction is the header lying (§6), not the metadata.

### 4.6 `malformed/v2_object_v1_metadata`

- **Input:** genuine V2 object (V2 header + V2 derivation) whose metadata
  claims `"1"` with a V1-style (no `v2:` prefix) wrapped DEK.
- **Declared:** failure — "V1 counter derivation cannot decrypt V2 ciphertext".
- **Observed (as shipped):** **failed=1** at DEK unwrap.
- **Format-aligned expectation:** **success, correct plaintext** — same
  reasoning as 4.5: the header's `0x02` drives decryption; the `"1"` label is
  cosmetic and corrected to `3`.
- **Behavior:** transform (label corrected).
- **Recovery:** none needed.
- **Why it matters:** same as 4.5, from the other direction; together they
  pin down that *metadata version is never used for decryption*.

### 4.7 `malformed/invalid_envelope_magic`

- **Input:** the 4 magic bytes of the stored envelope are overwritten with
  `DE AD BE EF`.
- **Declared:** failure — "cannot decode envelope header - invalid magic".
- **Observed (as shipped):** **failed=1** at DEK unwrap.
- **Format-aligned expectation:** **failure**, and this is the one single-PUT
  fixture whose intended error is reached even after the key divergences are
  repaired (magic is checked before the size field is parsed):

  ```
  migration failed: failed to decrypt object: failed to decode envelope header: invalid ARMOR magic
  ```

  (`DecodeHeader` → `ErrInvalidMagic`, wrapped by `decryptSingleObject`.)
- **Behavior:** fail, object untouched.
- **Recovery:** none from the object itself. If only the first 4 bytes are
  damaged, an operator can restore the magic (`41 52 4d 52`) and the object
  becomes migratable again — worth attempting only with a verified backup of
  the damaged header.
- **Why it matters:** non-ARMOR bytes at offset 0 must hard-fail; migrating
  would treat arbitrary data as an envelope.

### 4.8 `malformed/truncated_ciphertext`

- **Input:** stored object cut to 164 of 215 bytes — the header plus roughly
  the first two-thirds of the ciphertext; the HMAC table is entirely gone.
  The header still declares 119 bytes of plaintext.
- **Declared:** failure — "stored data is shorter than the header's declared
  plaintext size; envelope truncated".
- **Observed (as shipped):** **failed=1** at DEK unwrap.
- **Format-aligned expectation:** **failure**, but *not* by a length check —
  the migrator never compares stored length against the declared plaintext
  size. It only checks that the tail can hold the HMAC table
  (`decryptSingleObject`: `ciphertext too short to contain HMAC table`). With
  164 bytes the table *fits* (64 header + 68 data + 32 table), so truncation
  surfaces one block later as:

  ```
  migration failed: failed to decrypt object: failed to decrypt: block 0: HMAC verification failed
  ```

  Only a truncation deep enough to eat into the last 32 bytes produces the
  length error, with real numbers:

  ```
  migration failed: failed to decrypt object: ciphertext too short to contain HMAC table: got 6, need 32
  ```

  (Contrast the as-shipped REWRAP output, where divergence #2 turns the same
  fixture into `… need 4186940278571008` — a nonsense number is itself a
  reliable symptom of the endianness divergence.)
- **Behavior:** fail, object untouched.
- **Recovery:** none — data is gone; restore from backup/replica.
- **Why it matters:** interrupted uploads and failed multipart completes are
  the most common real-world corruption. HMAC verification catches it here,
  but only because the HMAC table is append-trailing; a range-read path that
  skips table verification would not.

### 4.9 `malformed/inconsistent_part_metadata`

- **Input:** stored as a *single-PUT-shaped* 119-byte envelope, but metadata
  says `x-amz-meta-armor-multipart: "true"` with `part-count: "999"` and
  `part-size: "1"`; a 32-byte sidecar ships alongside. (Its `metadata.json`
  also carries a success-shaped `v3_expected` block — `part_count: 1` and a
  sidecar path — which contradicts its own `expected_migration_outcome:
  "failure"`; the malformed fixtures' `v3_expected` blocks are stale and
  should not be read as expectations.)
- **Declared:** failure — "part count/size in metadata doesn't match actual
  structure".
- **Observed (as shipped):** **failed=1** at DEK unwrap; with the DEK
  re-wrapped to production format, **failed=1** with
  `failed to decrypt with HMAC: block 0: HMAC verification failed`
  (divergence #3 — the generator's HMAC key derivation, not the part
  metadata, is what fails).
- **Format-aligned expectation:** **the part-count/part-size values are never
  consulted by the migrator.** The read path branches only on
  `x-amz-meta-armor-multipart == "true"`, then treats the whole stored object
  as ciphertext and pulls the HMAC table from the sidecar. Outcome is decided
  entirely by sidecar/ciphertext consistency: if the sidecar authenticates
  the bytes, the object **migrates successfully** — with whatever plaintext
  those bytes decrypt to — regardless of `part-count: 999` or `part-size: 1`.
- **Behavior:** transform, with a corruption hazard (sidecar-dependent).
- **Recovery:** none automatic; see §7.
- **Why it matters:** real buckets carry stale part metadata from aborted
  multipart attempts. Nothing validates it today — the fixture documents a
  check that does not exist.

### 4.10 `malformed/multipart_part_size_mismatch`

- **Input:** valid V1 multipart object, 2 MiB / four 512 KiB parts, metadata
  claims `x-amz-meta-armor-part-size: "307200"` (300 KiB) instead of 524288.
- **Declared:** failure — "declared part size does not match actual part
  boundaries".
- **Observed (as shipped):** **failed=1** at DEK unwrap; DEK-rewrapped:
  **failed=1** at `block 0: HMAC verification failed` (divergence #3).
- **Format-aligned expectation:** with a self-consistent sidecar the object
  **migrates successfully** — `x-amz-meta-armor-part-size` is not read during
  decryption. Two after-effects worth knowing:
  - for objects below the 5 MiB multipart threshold the migration re-emits a
    single-PUT object (4 KiB blocks) but **copies the stale `part-size`
    forward** (`buildNewMetadata` preserves `armorMetaPartSize`), leaving
    `x-amz-meta-armor-part-size: 307200` on an object that is no longer
    multipart;
  - above the threshold, `uploadAsMultipart` overwrites it with the real
    5 MiB part size.
- **Behavior:** transform (metadata carried through).
- **Recovery:** none needed for correctness; the stale label can mislead
  range-read planners, so post-migration reporting flags it.
- **Why it matters:** same class as 4.9 — declared-structure lies are not
  validated, only sidecar/ciphertext consistency is.

### 4.11 `malformed/multipart_contradictory_hashes`

- **Input:** valid V1 multipart object whose `x-amz-meta-armor-sha256`
  (`13334f02…`) differs from the plaintext's real hash (`91d3beb8…`).
- **Declared:** failure — "metadata sha256 != envelope header sha256;
  integrity cannot be established". (Note: a multipart object has **no
  envelope header**, so the comparison the fixture describes cannot happen as
  stated.)
- **Observed (as shipped):** **failed=1** at DEK unwrap; DEK-rewrapped:
  **failed=1** at `block 0: HMAC verification failed` (divergence #3).
- **Format-aligned expectation:** **migrates successfully.** No code path
  compares `x-amz-meta-armor-sha256` against anything during migration. After
  decryption the SHA is **recomputed from the decrypted plaintext** and that
  value is written to the migrated object — the contradiction is silently
  resolved in favor of whatever the decrypt produced. If the sidecar is
  honest, the recomputed value equals the true plaintext hash and the stale
  metadata is quietly corrected; if the plaintext itself is wrong (wrong
  keystream, §6), the wrong hash is blessed into the V3 metadata.
- **Behavior:** transform (hash recomputed and overwritten).
- **Recovery:** none automatic.
- **Why it matters:** the same gap as §6, seen from the metadata side —
  the migrator treats the recomputed hash as truth rather than treating a
  mismatch with prior metadata as a signal.

### 4.12 `malformed/invalid_sidecar_format`

- **Input:** multipart object whose sidecar is 14 bytes of malformed
  protobuf-style wire format (tag `0x0A` + varint length `0x7FFFFFFF` running
  past the end).
- **Declared:** failure — "cannot parse HMAC sidecar - invalid structure".
- **Observed (as shipped):** **failed=1** at DEK unwrap; DEK-rewrapped, this
  fixture **does reach a genuine defect error**:

  ```
  migration failed: failed to decrypt object: failed to decrypt with HMAC: HMAC table too short: got 14, need 1024
  ```

- **Format-aligned expectation:** identical (the check is size-only). The
  declared wording is misleading in one respect: the migrator **does not parse
  the sidecar as a document at all** — `loadHMCTableFromSidecar` treats it as
  a raw table of 32-byte HMAC-SHA256 entries. "Invalid structure" is detected
  purely as `len(table) < blockCount × 32`.
- **Behavior:** fail, object untouched, sidecar untouched.
- **Recovery:** replace the sidecar from a replica. Regenerating it is
  impossible without the plaintext.
- **Why it matters:** the sidecar is load-bearing for multipart integrity; a
  truncated/garbage sidecar must fail closed. This is the cleanest
  malformed-sidecar signal the migrator can give.

### 4.13 `malformed/truncated_sidecar`

- **Input:** multipart object whose sidecar lost its final 32-byte entry:
  992 bytes (31 entries) against 32 blocks (needs 1024).
- **Declared:** failure — "HMAC table has fewer entries than the object has
  blocks".
- **Observed (as shipped):** **failed=1** at DEK unwrap; DEK-rewrapped, the
  genuine defect error:

  ```
  migration failed: failed to decrypt object: failed to decrypt with HMAC: HMAC table too short: got 992, need 1024
  ```

- **Format-aligned expectation:** identical.
- **Behavior:** fail, object untouched.
- **Recovery:** restore the sidecar from a replica.
- **Why it matters:** "last entry lost" is the natural damage pattern for a
  partially-written sidecar; the size check catches it before any block is
  trusted.

---

## 5. Fixture catalog — `edge_cases/`

All three are genuine production-format V1 single-PUT objects (40-byte KWP
wrapped DEKs, little-endian header sizes) and all three **migrate
successfully today**. These are the only fixtures in the three groups that
currently produce V3 output, and the observed outputs below are exact.

### 5.1 `edge_cases/empty_plaintext`

- **Input:** zero-length plaintext (`plaintext-length: 0`, SHA-256
  `e3b0c44298fc…52b855`, the empty-string hash). Stored object is **64 bytes**
  — header only, no ciphertext, no HMAC entries.
- **Expected migration behavior:** transform → success.
- **Observed V3 output:**

  ```
  processed=1 skipped=0 failed=0
  x-amz-meta-armor-version:        "3"
  x-amz-meta-armor-wrapped-dek:    "v2:ae216c2e…:<base64>"   (fresh DEK, v2 fingerprint format)
  x-amz-meta-armor-block-size:     "4096"                     (migration's single-PUT block size, not the source's 65536)
  x-amz-meta-armor-plaintext-size: "0"
  x-amz-meta-armor-sha256:         "e3b0c44298fc1c14…b7852b855"  (unchanged; sha256("") )
  stored object: 64 bytes (V3 header only)
  ```

  Decrypt path: `numBlocks = 0` → empty HMAC table → `Decrypt` returns empty
  plaintext without verifying anything (there is nothing to verify).
- **Recovery:** n/a.
- **Why it matters:** zero-byte objects exercise the `ceil(0/65536) = 0` edge
  in block accounting — a fencepost error there would corrupt every empty
  object. Also documents that zero-block objects carry **no integrity data at
  all**.

### 5.2 `edge_cases/single_byte_plaintext`

- **Input:** 1-byte plaintext `"A"` (SHA-256 `559aead0…fdffd`); stored object
  is 97 bytes = 64 header + 1 ciphertext byte + 32 HMAC (one partial block).
- **Expected migration behavior:** transform → success.
- **Observed V3 output:**

  ```
  processed=1 skipped=0 failed=0
  x-amz-meta-armor-version:        "3"
  x-amz-meta-armor-block-size:     "4096"
  x-amz-meta-armor-plaintext-size: "1"
  x-amz-meta-armor-sha256:         "559aead08264d579…08fdffd"  (preserved)
  ```

  Migrated plaintext SHA-256 verified equal to the declared value.
- **Recovery:** n/a.
- **Why it matters:** the minimal partial block. Every `ceil`/`min` in the
  split-ciphertext/HMAC-table logic in `decryptSingleObject` is exercised at
  its smallest non-empty case.

### 5.3 `edge_cases/exact_block_boundary`

- **Input:** 131072 bytes = exactly 2 × 65536, so the last block is full — no
  partial final block (SHA-256 `59f410ae…017850`); stored object is
  131200 bytes.
- **Expected migration behavior:** transform → success.
- **Observed V3 output:**

  ```
  processed=1 skipped=0 failed=0
  x-amz-meta-armor-version:        "3"
  x-amz-meta-armor-block-size:     "4096"
  x-amz-meta-armor-plaintext-size: "131072"
  x-amz-meta-armor-sha256:         "59f410ae5e179624…db017850"  (preserved)
  ```

  Migrated plaintext SHA-256 verified equal to the declared value.
- **Recovery:** n/a.
- **Why it matters:** exact multiples are where `+1` fenceposts over-allocate
  or drop a final block. Note the boundary property is **not preserved** by
  migration: the source's two 64 KiB blocks are re-emitted as 32 × 4 KiB
  blocks (`encryptAsSingle` hard-codes `blockSize = 4096`); the boundary is
  exercised on the read path, not the write path.

---

## 6. Fixture catalog — `contradictory/`, and the corruption gap

### 6.1 `contradictory/version_says_v1_layout_v2`

- **Input:** a production-format, *self-consistent* single-PUT object whose
  ciphertext was produced with **V2 counter derivation** but whose envelope
  header byte says `0x01` and whose metadata says `"1"` (94-byte plaintext,
  SHA-256 `a7f6f6d8…69f12c`, matching `v1_single_put/explicit_version`).
- **Declared:** failure — "version/header mismatch (V1 header with V2
  ciphertext)".
- **Observed (as shipped):** **processed=1, skipped=0, failed=0** — the
  migration *succeeds*, and the object is destroyed:

  ```
  post-migration: version="3"  blockSize="4096"  plainSize="94"
                  sha256 = b8aeffe913917603bb54e772347f9a9b7f0f453eba4523b283b478dd7d4dae23
  declared        sha256 = a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c
                  → MISMATCH: the migrated object contains irrecoverable garbage
  ```

- **Mechanism (each step empirically isolated):**
  1. Unlike every `malformed/*` fixture, this object's wrapped DEK is
     production format (40-byte KWP), so unwrap **succeeds** and the migrator
     proceeds to the data path.
  2. The stored HMAC table **verifies** — per-block HMACs authenticate
     ciphertext bytes + block index only, not the version byte and not the
     plaintext, so a keystream derived from the wrong counter layout passes
     every integrity check the migrator performs.
  3. `decryptSingleObject` selects the counter derivation from the **header's
     version byte**; this object's ciphertext was produced with a different
     counter layout than the header declares, so the recovered plaintext is
     garbage: `b8aeffe9…` vs the declared `a7f6f6d8…`.
  4. The migrator then **recomputes** the plaintext SHA from the garbage and
     writes it into the V3 metadata. No declared value is ever consulted, so
     nothing mismatches and nothing fails.
  5. The class is reproducible in **pure production crypto** — no generator
     divergence required. A 70 000-byte (multi-block) object encrypted with
     V2 whose header byte says `0x01`: `processed=1 failed=0`, migrated
     plaintext SHA `0f6d9cdcc7cd56467c6125bc305e8f078805c4b538bda635d1ae6a6a21939f0a`
     ≠ the true SHA. Single-block versions of the same lie decrypt *correctly*
     by accident (V1 and V2 counters coincide on block 0 — synthesis
     `synth/v2_ciphertext_v1_header_label_v2`: green, SHA matches), which is
     exactly why small-fixture test suites can miss this corruption class
     entirely.
- **Behavior:** transform — **silent data destruction**.
- **Recovery:** none after the fact; the source object has been overwritten
  in place. Prevention is the only remedy (§7).
- **Why it matters:** this is the single most important fixture in the set.
  It demonstrates that a *self-consistent* object with one lying byte can be
  republished as a healthy V3 object with a green migration summary. The
  fixture's own `expected_migration_outcome: "failure"` describes the behavior
  the code **should** have, not the behavior it has.

### 6.2 Gap summary and recommendations

The gap is narrow and specific: **the migration path never verifies decrypted
plaintext against any declared value.** Everything else (magic, version
validity, table sizes, per-block HMACs, DEK wrap integrity) fails closed.

Recommended, in impact order:

1. **Cross-check the decrypted plaintext** against a declared SHA-256 when one
   exists (`x-amz-meta-armor-sha256`, or the envelope header's
   `PlaintextSHA` for single-PUT) before re-encrypting; on mismatch, record a
   failure and leave the object untouched. This converts every silent-
   corruption case in this document (4.2, 4.11, 6.1) into a recorded failure.
   `crypto.ErrPlaintextMismatch` and `EnvelopeHeader.VerifyPlaintextSHA`
   already exist and are used by the canary and restore-verifier paths — the
   migrator is the one decrypt path that skips them.
2. **Compare the envelope header's version byte to `x-amz-meta-armor-version`
   for single-PUT objects** and fail (or at least defect-report) on
   disagreement, since the header is what decryption trusts.
3. **Validate multipart structural metadata** (`part-count`, `part-size`
   against plaintext size and stored length) at classification time, and stop
   copying a stale `part-size` onto migrated single-PUT objects (4.9, 4.10).
4. **Regenerate `malformed/*` against production crypto** (fix the four
   divergences in §3, or generate them with `internal/crypto` itself),
   otherwise those fixtures keep proving only the wrong-DEK-length error.
   The `expected_failure_reason` strings in their `metadata.json` should be
   corrected to the observed messages above at the same time.

---

## 7. Summary matrix

| Fixture | Declared | Observed as shipped | Format-aligned expectation |
|---|---|---|---|
| `malformed/invalid_version_string` | failure | failed (DEK length) | **success** — label corrected to 3 |
| `malformed/envelope_version_mismatch` | failure | failed (DEK length) | no check exists; header-true → success; header-lie → correct by luck on 1 block, **silent corruption** on multi-block |
| `malformed/corrupted_hmac_table` | failure | failed (DEK length) | failure: `block 0: HMAC verification failed` |
| `malformed/corrupted_wrapped_dek_tag` | failure | failed (DEK length) | failure: `key unwrap failed: invalid AIV` |
| `malformed/v1_object_v2_metadata` | failure | failed (DEK length) | **success** — header wins, label corrected |
| `malformed/v2_object_v1_metadata` | failure | failed (DEK length) | **success** — header wins, label corrected |
| `malformed/invalid_envelope_magic` | failure | failed (DEK length) | failure: `invalid ARMOR magic` |
| `malformed/truncated_ciphertext` | failure | failed (DEK length) | failure: `block 0: HMAC verification failed` (or table-fit error if cut into the table) |
| `malformed/inconsistent_part_metadata` | failure | failed (DEK length) | part metadata never validated; sidecar-consistent → success |
| `malformed/multipart_part_size_mismatch` | failure | failed (DEK length) | part-size never validated; success + stale label carried forward |
| `malformed/multipart_contradictory_hashes` | failure | failed (DEK length) | hash never checked; success, hash recomputed/overwritten |
| `malformed/invalid_sidecar_format` | failure | failed (DEK length); rewrapped: `HMAC table too short: got 14, need 1024` | same error (size-based, key-independent) |
| `malformed/truncated_sidecar` | failure | failed (DEK length); rewrapped: `HMAC table too short: got 992, need 1024` | same error |
| `edge_cases/empty_plaintext` | success | **success**, 64-byte V3 object, sha `e3b0c4…` | same |
| `edge_cases/single_byte_plaintext` | success | **success**, V3 `block-size 4096`, sha preserved | same |
| `edge_cases/exact_block_boundary` | success | **success**, V3 `block-size 4096`, sha preserved | same |
| `contradictory/version_says_v1_layout_v2` | failure | **"success" with corrupted plaintext** (sha `b8aeffe9…` ≠ `a7f6f6d8…`) | with recommendation §7.1 in place: failure |

Counting the groups: **3 successes with correct data, 13 recorded failures,
1 silent corruption** — and 4 metadata-only defects that no code checks.

---

## 8. Reproducing the measurements

- Build a `MockBackend` bucket containing one fixture per run: object key
  `fixtures/<group>/<name>` with `object_metadata.json` as metadata and
  `stored_ciphertext.bin` as data; for multipart fixtures add the sidecar at
  `.armor/hmac/` + hex(sha256(object key)).
- MEK = bytes `0x01..0x20`; `NewFormatMigrator(backend, "bucket", mek, keyID,
  3, []string{"1","2"}, nil)`; `Migrate(ctx, false, 1)`.
- Assert on `result.ProcessedObjects/SkippedObjects/FailedObjects` and
  `result.Failures[i].Reason`; for successes, decrypt the migrated object with
  `crypto` and compare SHA-256 to `metadata.json:plaintext_sha256`.
- The "format-aligned" column: rebuild each object with
  `crypto.WrapDEK` / `crypto.NewEncryptorWithVersion` /
  `crypto.NewEnvelopeHeaderWithVersion` (which writes the size field
  little-endian) before applying the corruption.

---

## 9. Related documentation

- `migration-error-handling-flow.md` — failure recording plumbing
  (`recordFailure`, `MigrationFailure`, best-effort continuation)
- `v1-single-part-fixtures.md`, `v2-single-part-fixtures.md` — note: the
  envelope layout table in the V1 doc predates `BlockSizeLog2`; the
  authoritative 64-byte layout is `crypto.EnvelopeHeader`
  (`internal/crypto/envelope.go`): Magic(4) Version(1) BlockSizeLog2(1)
  IV(16) PlaintextSize(8, LE) PlaintextSHA(32) Reserved(2, compression flag)
- `v1-multipart-fixtures.md`, `v2-multipart-fixtures.md` — valid multipart fixtures

---

**Document Version:** 1.0
**Date:** 2026-09-08
**Basis:** empirical runs of `FormatMigrator` at commit `253d80134` against the
shipped fixture bytes, plus production-format defect synthesis; error messages
quoted verbatim from observed output.
