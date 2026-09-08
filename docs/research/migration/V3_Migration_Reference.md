# V3 Migration Reference

The canonical, consolidated guide to ARMOR V1/V2 → V3 format migration: what
every source format looks like, what migration turns it into, how the migrator
decides, what fails and how, and what an operator must know before running a
production migration.

**Status:** Reference (2026-09-08)
**Basis:** `internal/server/format_migration.go` and `internal/backend/backend.go`
at commit `89d1aa0c1`, consolidated from the per-family fixture documents listed
below. Empirical fixture observations were measured at commit `253d80134` and
re-verified where cited.

**Detailed companion documents** (the per-fixture source material for this
guide — every JSON block and byte-level table lives there, not here):

| Document | Covers |
|---|---|
| [`v1-single-part-fixtures.md`](v1-single-part-fixtures.md) | V1 single-PUT fixtures (5) |
| [`v2-single-part-fixtures.md`](v2-single-part-fixtures.md) | V2 single-PUT fixtures (2) |
| [`v1-multipart-fixtures.md`](v1-multipart-fixtures.md) | V1 multipart fixtures (4) + full-scale variants + **the authoritative V3 target-format spec** |
| [`v2-multipart-fixtures.md`](v2-multipart-fixtures.md) | V2 multipart fixtures (4) + full-scale variants |
| [`malformed-edge-case-fixtures.md`](malformed-edge-case-fixtures.md) | 13 malformed + 3 edge-case + 1 contradictory fixture, with observed vs. intended behavior |
| [`migration-error-handling-flow.md`](migration-error-handling-flow.md) | Failure plumbing: `recordFailure`, `MigrationFailure`, best-effort continuation |

---

## 1. Executive summary

- **Migration is a decrypt → re-encrypt → re-upload pipeline, not an in-place
  rewrite.** Each object is decrypted under its source format, then
  re-encrypted with a **fresh random 32-byte DEK** (AES-KWP-wrapped, emitted as
  `v2:<16-hex fingerprint>:<base64>`) and a **fresh random 16-byte IV**. The
  plaintext, its length, and its SHA-256 are the invariants; every other byte
  of the stored object changes.
- **The output layout is chosen by plaintext size alone** (`migrateObject`,
  `format_migration.go:434`): ≤ 5 MiB (`multipartThreshold()`) → V3
  **single-PUT** (4 KiB blocks, trailer block table); > 5 MiB → V3
  **multipart** (64 KiB blocks, 5 MiB parts, gzip-JSON HMAC sidecar). The
  source layout — single vs. multipart, uniform vs. non-uniform — plays **no
  part** in that decision. A 94-byte V1 multipart object migrates to a V3
  single-PUT object.
- **What V3 buys:** per-part-independent CTR counters
  (`IV[0:8] ‖ BE16(part) ‖ BE32(block) ‖ BE16(aesBlock)`) that eliminate V1's
  cross-part keystream reuse and V2's cumulative-offset bookkeeping and 64 GiB
  counter ceiling; part-and-block-bound HMACs; and integrity storage that
  lives in the object (trailer table) for single-PUT output.
- **Migration fails closed on every structural defect it can see** — bad
  magic, corrupt DEK wrap, missing/short sidecar, per-block HMAC mismatch —
  and never writes a replacement object before decryption succeeds. Failures
  are recorded per-object (`FailedObjects++`, `MigrationFailure{Key, Reason,
  Time}`) and the run continues (best-effort); `armor migrate` exits 1 when
  anything failed.
- **The one known gap is silent corruption on a lying envelope.** The
  migrator never compares decrypted plaintext against any declared SHA-256 and
  selects the counter derivation from the **envelope header's version byte**
  (single-PUT), never from `x-amz-meta-armor-version`. A self-consistent
  object whose header byte lies about its ciphertext's counter layout decrypts
  to garbage, and the garbage's hash is blessed into the V3 metadata with a
  green summary (§8). On single-block objects the lie is invisible because V1
  and V2 counters coincide at block 0.
- **Fixture learnings, in one line each:** the shipped `malformed/*` fixtures
  cannot reach their intended defects (the standalone generator's crypto has
  diverged from production in four ways, so everything fails at DEK unwrap
  first); the V2 multipart fixture ciphertext does not match the production
  V2 counter derivation (sidecar HMACs still verify, so such input migrates
  "successfully" to garbage); `v3_expected` in fixture metadata describes the
  *source* layout, not the migrator's output; and `v3-golden-outcomes.json` is
  illustrative, not authoritative — the committed fixture bytes are.

---

## 2. Format comparison (V1 / V2 / V3)

### 2.1 Single-PUT envelope header

V1 and V2/V3 differ structurally; V2 and V3 share the 64-byte layout. The
authoritative definition is `crypto.EnvelopeHeader`
(`internal/crypto/envelope.go`).

| Offset | Size | Field | V1 | V2 | V3 |
|---|---|---|---|---|---|
| 0x00 | 4 | Magic `"ARMR"` | ✓ | ✓ | ✓ |
| 0x04 | 1 | Version byte | `0x01` | `0x02` | `0x03` |
| 0x05 | 1 | BlockSizeLog2 | *(absent — V1 header is 65 bytes with a 4-byte reserved field)* | ✓ (16 = 64 KiB) | ✓ |
| 0x06 | 16 | IV | ✓ | ✓ | ✓ (fresh at migration) |
| 0x16 | 8 | PlaintextSize | ✓ (V1: at 0x39 after a 32-byte SHA field) | ✓ little-endian | ✓ little-endian |
| 0x1E | 32 | PlaintextSHA-256 | ✓ | ✓ | ✓ (unchanged by migration) |
| 0x3E | 2 | Reserved | zero | zero | compression flags (0x00 = none; migration never compresses) |
| 0x40+ | var | Ciphertext | ✓ | ✓ | ✓ + **trailer block table** (36 B/block: 32 B HMAC + 4 B clen) |

> Note: the envelope table in `v1-single-part-fixtures.md` predates
> `BlockSizeLog2`; treat `crypto.EnvelopeHeader` as authoritative, as flagged
> in `malformed-edge-case-fixtures.md` §9.

**Multipart objects (V1 and V2) have no envelope header at all** — the stored
blob is the raw concatenation of encrypted parts, and version, IV, block size,
plaintext size and SHA-256 come exclusively from S3 object metadata. The read
path and the migrator both trust metadata for multipart objects
(`internal/server/server.go` — "Multipart objects have no envelope header —
trust metadata version").

### 2.2 Counter derivation (the security core)

| | V1 | V2 | V3 |
|---|---|---|---|
| Counter block | `IV[0:12] ‖ BE32(blockIndex)` | `IV[0:12] ‖ BE32(blockIndex × blockSize/16)` | `IV[0:8] ‖ BE16(part) ‖ BE32(block) ‖ BE16(aesBlock)` |
| Within an object | **Broken** — adjacent 64 KiB blocks reuse keystream | Fixed — stride 4096 AES blocks per ARMOR block | Fixed — part/block/aesBlock namespacing |
| Across parts (multipart) | **Broken** — every part restarts at index 0 with the same DEK/IV, so part *k* block *j* reuses part 0's keystream | One continuous stream via cumulative offsets (`EncryptWithStartingCounter`, `OffsetEncryptor`) — no reuse, but parts are *interdependent* | **Independent** — the part number is inside the counter; parts are order- and size-independent streams |
| Counter-space ceiling | 2³² blocks (≈256 TiB at 64 KiB) — not binding | 2³² AES counters ÷ 4096 = **64 GiB** — binding, V2-specific | Effectively unbounded |
| Block 0 | `IV[0:12] ‖ BE32(0)` | **identical** — V1 and V2 first blocks are byte-identical for the same DEK/IV | different |

Why ADR-010 (variable final part) and ADR-011 (non-uniform parts) exist:
V1/V2 multipart pin the counter stream cumulatively, so final-part length must
be inferred and part boundaries tracked. V3 removes the reason for both
exemptions — any part-size mix is legal.

### 2.3 Integrity (HMAC)

| | V1 | V2 | V3 |
|---|---|---|---|
| HMAC key | `HKDF-SHA256(DEK, info="armor-hmac-v1")` | **identical** | derived per part/block usage, same HKDF info |
| HMAC input | `uint32BE(globalBlockIndex) ‖ ciphertext` | **identical** | `BE16(part) ‖ BE32(block) ‖ ciphertext` — binds position, so block/part reordering is detectable |
| Storage (single-PUT source) | trailer table after ciphertext | trailer table | trailer block table, 36 B/block (HMAC 32 B + clen 4 B; high bit of clen = zstd flag, never set by migration) |
| Storage (multipart source) | flat sidecar `.armor/hmac/<sha256hex(object_key)>`, global block order | **identical** | gzip-compressed JSON sidecar (`HMACTableSidecarV3`, per-(part, block)) for multipart output; trailer table for single-PUT output |
| Placeholder entries | ADR-011 boundary blocks may carry all-zero HMACs; streaming reads skip them | **identical** | concept eliminated (per-(part, block) HMACs) |

### 2.4 DEK wrapping and object metadata

| | V1 | V2 | V3 (output of migration) |
|---|---|---|---|
| `x-amz-meta-armor-wrapped-dek` | bare base64 of 40-byte AES-KWP body | `v2:<fingerprint>:<base64>` | `v2:<16-hex fingerprint>:<base64>` — always a **fresh** DEK |
| `x-amz-meta-armor-version` | `"1"` (or absent = implicit V1) | `"2"` | `"3"` |
| `x-amz-meta-armor-block-size` | `65536` | `65536` | `4096` (single-PUT output) / `65536` (multipart output) |
| `x-amz-meta-armor-part-size` | nominal uniform size; absent for non-uniform (ADR-011) | same | `5242880` on multipart output; **source value carried over otherwise** (including stale values, §8) |
| `x-amz-meta-armor-multipart` | `"true"` | `"true"` | set **only** by the multipart output path; never copied from source |
| `x-amz-meta-armor-iv` | base64, object IV | same | base64, **fresh** random IV |
| `x-amz-meta-armor-sha256` / `-plaintext-size` | preserved | preserved | preserved (plaintext is byte-identical) |
| `x-amz-meta-armor-etag` | n/a | plaintext SHA-256 hex (generator-set) | copied through when present |

`buildNewMetadata` (`format_migration.go:792`) drops **all**
`x-amz-meta-armor-*` fields and re-emits them; non-ARMOR metadata and
`content-type`, `etag`, `key-id`, `compressed`, `compression` are preserved
when present.

### 2.5 Part layout changes

| Source | Migrated output (≤ 5 MiB) | Migrated output (> 5 MiB) |
|---|---|---|
| V1/V2 single-PUT | V3 single-PUT, 4 KiB blocks, trailer table | (n/a at fixture scale) |
| V1/V2 multipart, 94 B miniature | V3 single-PUT, 1 × 4 KiB block; `multipart` flag dropped; old sidecar orphaned | — |
| V1/V2 multipart, 256 KiB generated | V3 single-PUT, 64 × 4 KiB blocks (source had 4 × 64 KiB — block count is a derived property, not an invariant) | — |
| V1/V2 multipart uniform, 15 MiB | — | V3 multipart, 3 × 5 MiB (source 3 × 5 MiB preserved *coincidentally* — output part size equals source nominal size) |
| V1/V2 multipart variable-final, 15 MiB | — | V3 multipart, 3 × 5 MiB (source 5 × 3 MiB — **part structure not preserved**) |
| V1/V2 multipart non-uniform, 15 MiB | — | V3 multipart, 3 × 5 MiB (source 1/2/12 MiB — **re-split at 5 MiB always**) |

The only invariants across part layout: total plaintext, its length, and its
SHA-256. `v3_expected.part_count` / `blocks_per_part` in fixture
`metadata.json` describe the **source** layout and must not be asserted
against migrator output.

---

## 3. Migration decision tree

Every object in the bucket passes through the same gates
(`Migrate()` at `format_migration.go:163` → `migrateObject()` at :370).
Code location citations are to `internal/server/format_migration.go` at
`89d1aa0c1` unless otherwise noted.

```text
For each object (listed in key order; .armor/* internal objects excluded;
keys <= state.LastKey skipped as already processed in a resumable run):
│
├─ GET metadata fails?
│    └─ YES → record failure "failed to get metadata", advance cursor, continue
│
├─ ParseARMORMetadata (internal/backend/backend.go:292)
│    parse fails (!ok — required field such as wrapped DEK missing)?
│    ├─ x-amz-meta-armor-version absent → SKIP (non-ARMOR), SkippedObjects++
│    ├─ version string unparseable      → SKIP (invalid version), SkippedObjects++
│    └─ version parses
│         ├─ not in includeVersions → SKIP (out of scope), SkippedObjects++
│         └─ in includeVersions     → ATTEMPT migration anyway
│                                      (it will fail and be recorded)
│    parse succeeds:
│         version = am.Version  (unparseable/unknown version string in
│         metadata DEFAULTS TO 1 when a wrapped DEK is present — see FAQ Q4)
│         ├─ version not in includeVersions → SKIP (out of scope)
│         ├─ version == currentWriteVersion (3) → SKIP (already migrated)
│         └─ otherwise → migrateObject
│
└─ migrateObject (format_migration.go:370)
   │
   ├─ dryRun? → decrypt, then STOP (`if dryRun { return nil }`) —
   │            same failures as a real run, no write
   │
   ├─ Pre-validate metadata: wrapped DEK (strip optional `v2:<fp>:` prefix,
   │  base64) and IV (base64) must decode → else record failure BEFORE fetch
   │
   ├─ x-amz-meta-armor-multipart == "true"?
   │    ├─ YES → decryptMultipartObject (:570)
   │    │        load sidecar .armor/hmac/<sha256hex(key)> (:607)
   │    │        ├─ sidecar missing → FAIL `missing_sidecar`
   │    │        ├─ len(table) < blocks × 32 → FAIL `HMAC table too short: got N, need M`
   │    │        └─ decrypt concatenated blob as ONE block sequence, metadata
   │    │           IV/version/block-size, per-block HMAC over GLOBAL indices
   │    │           (Decryptor.Decrypt verifies EVERY slot — no placeholder
   │    │           exemption, unlike the streaming read path)
   │    │           └─ any HMAC mismatch → FAIL `block N: HMAC verification failed`
   │    └─ NO  → decryptSingleObject (:500)
   │             unwrap DEK (AES-KWP, 40-byte body)
   │             │    └─ failure → FAIL `failed to unwrap DEK: …`
   │             read 64-byte envelope header
   │             ├─ bad magic            → FAIL `invalid ARMOR magic`
   │             ├─ tail too small for
   │             │  the HMAC table       → FAIL `ciphertext too short to contain HMAC table: got N, need M`
   │             └─ **counter derivation is selected by the HEADER's version
   │                byte**, never by x-amz-meta-armor-version (FAQ Q3)
   │
   ├─ any decrypt failure → record Failure{Key, Reason, Time}, FailedObjects++,
   │                        object left byte-identical, loop CONTINUES (best-effort)
   │
   └─ decrypt OK:
        SHA-256 the recovered plaintext (this recomputed value — not any
        declared value — is written to the new metadata; see §8)
        │
        ├─ plaintextSize > 5 MiB (multipartThreshold, :1063)
        │    └─ uploadAsMultipart (:684) — 64 KiB blocks, 5 MiB parts,
        │       gzip-JSON sidecar, multipart=true, part-size=5242880
        └─ otherwise
             └─ encryptAsSingle (:629) — blockSize = 4096 hardcoded (:644),
                fresh DEK + IV, trailer block table
        │
        └─ Verify: read the object back, assert version == write version,
           decrypt, compare SHA-256 → ProcessedObjects++
           (note: read-back always uses decryptSingleObject (:483), which
           expects an envelope header — see Known Issues #4)
```

Post-run: failures live in `.armor/migration-state.json` and in the run
result; `armor migrate` prints `Migration had N failures.` and exits 1 when
any object failed. A re-run resumes after `LastKey` — failures are not
retried specially; they fail again unless the object changed.

---

## 4. V1 → V3 transformation reference

Common path for all V1 sources (full per-fixture detail:
[`v1-single-part-fixtures.md`](v1-single-part-fixtures.md),
[`v1-multipart-fixtures.md`](v1-multipart-fixtures.md)):

| # | Step | V1 in | V3 out |
|---|---|---|---|
| 1 | Version detection | explicit `x-amz-meta-armor-version: "1"`, **or absent (implicit V1)**, or unparseable-with-DEK (defaults to 1) | metadata `x-amz-meta-armor-version: "3"` always emitted |
| 2 | Metadata reconstruction | missing `plaintext-size` / `sha256` reconstructed from the envelope header (offsets 0x39 / 0x19 in the legacy layout) | always present |
| 3 | DEK re-wrapping | bare base64 AES-KWP body, **no fingerprint prefix** | unwrap → **fresh DEK** → `v2:<fingerprint>:<base64>` |
| 4 | Counter fix | `IV[0:12] ‖ BE32(blockIndex)`, per-part restart | `IV[0:8] ‖ BE16(part) ‖ BE32(block) ‖ BE16(aesBlock)` |
| 5 | Re-encryption | source keystream (vulnerable) | decrypt + re-encrypt with fresh DEK/IV; plaintext preserved |
| 6 | Integrity storage | flat sidecar (multipart) or trailing table (single-PUT), global indices | trailer table (4 KiB blocks, single-PUT out) or gzip-JSON sidecar (multipart out), (part, block) indices |
| 7 | Multipart flag | `"true"` on multipart sources | dropped on single-PUT output; re-set by multipart output |

**Before/after (V1 multipart 94-byte miniature → V3 single-PUT):**

```jsonc
// BEFORE (object_metadata.json)
{
  "x-amz-meta-armor-version": "1",
  "x-amz-meta-armor-multipart": "true",
  "x-amz-meta-armor-part-size": "5242880",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8…69f12c",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-wrapped-dek": "yuqSIns…ZPdbBePJQ=="   // bare base64, 40-byte KWP body
}
// stored_ciphertext.bin: raw concatenated part ciphertext, NO header
// sidecar.bin: flat 32-byte-per-block HMAC table at .armor/hmac/<sha256hex(key)>

// AFTER
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-block-size": "4096",                  // single-PUT output block size
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8…69f12c",           // unchanged
  "x-amz-meta-armor-iv": "<fresh random base64>",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>"
  // "multipart" dropped; "part-size" CARRIED OVER (stale, §8)
}
// stored object: header(64) ‖ 94 B ciphertext ‖ trailer table (36 B) — 194 bytes
// old sidecar: orphaned, neither rewritten nor deleted
```

V1-specific notes:

- **Implicit version detection** (`implicit_version`, `minimal_metadata`):
  version comes from the envelope header byte; metadata gets a version field
  for the first time.
- **Cross-part keystream reuse** is *the* V1 multipart defect: every part
  encrypts block *j* with counter `IV[0:12] ‖ BE32(j)`. V3's part-namespaced
  counters eliminate it (per-part mapping table in
  [`v1-multipart-fixtures.md`](v1-multipart-fixtures.md) §uniform_parts).
- **`part-size` absence is legal** (non-uniform, ADR-011): migration must not
  require it, and `buildNewMetadata` emits no `part-size` when the source had
  none.
- **Negative goldens:** `v1_multipart_sidecar_corrupt` →
  `integrity_check_failed`; `v1_multipart_sidecar_missing` →
  `missing_sidecar`. Both must fail **before** any write.

## 5. V2 → V3 transformation reference

Common path (full detail:
[`v2-single-part-fixtures.md`](v2-single-part-fixtures.md),
[`v2-multipart-fixtures.md`](v2-multipart-fixtures.md)). The pipeline is the
same as V1's — same classification shape, same sidecar, same decrypt entry
point, same output routing. What differs:

| # | Step | V2 in | V3 out |
|---|---|---|---|
| 1 | Version detection | explicit `"2"` | `"3"` |
| 2 | DEK re-wrapping | already `v2:<fingerprint>:<base64>` — **parses, but is still replaced** with a fresh DEK (all migrations re-key) | fresh DEK, `v2:` format |
| 3 | Counter upgrade | `IV[0:12] ‖ BE32(j × blockSize/16)`; multipart threads **one continuous stream** across parts | part-namespaced counters — a structural upgrade, not a keystream-reuse fix |
| 4 | Ceiling removal | 64 GiB counter-space ceiling (`checkCounterSpace`) | removed |
| 5 | Integrity storage | flat global-index sidecar (multipart); trailer table (single-PUT) | trailer table (4 KiB, single-PUT out) / gzip-JSON (multipart out) |
| 6 | Metadata | `etag` present on generator-built miniatures | carried through when present |

**Before/after (V2 single-PUT `standard`, 94 B → V3 single-PUT):**

```jsonc
// BEFORE
{
  "x-amz-meta-armor-version": "2",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8…69f12c",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-wrapped-dek": "v2:ae216c2ef5247a37:yuqSIns…PJQ==",
  "x-amz-meta-armor-etag": "a7f6f6d8…69f12c"
}
// stored_ciphertext.bin: 64-byte header (version 0x02) ‖ 94 B ciphertext ‖ 32 B HMAC

// AFTER
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-block-size": "4096",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8…69f12c",
  "x-amz-meta-armor-iv": "<fresh random base64>",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-etag": "<carried through>"
}
// stored object: 64-byte header (version 0x03, BlockSizeLog2=12) ‖ ciphertext
//                ‖ trailer block table (36 B)
```

V2-specific notes:

- **Block 0 equivalence:** V1 and V2 coincide at block 0 (`0 × 4096 = 0`), so
  a single-block object mislabeled V1/V2 decrypts correctly *by accident* —
  this hides the §8 corruption class from small-fixture test suites.
- **Fixture ciphertext caveat:** the on-disk V2 multipart fixture bytes were
  produced with LE no-IV counters, not the production derivation — the HMAC
  sidecars still verify (they hash the same ciphertext bytes), so such input
  *migrates successfully to garbage* unless regenerated. See Known Issues #2
  before writing any test that asserts `plaintext_sha256` preservation on V2
  multipart fixture bytes.
- **No V2-specific negative goldens** exist; the sidecar failure modes are
  version-agnostic and only golden-documented under V1 labels.

## 6. Error handling reference

Failure plumbing detail:
[`migration-error-handling-flow.md`](migration-error-handling-flow.md);
per-fixture evidence: [`malformed-edge-case-fixtures.md`](malformed-edge-case-fixtures.md).

### 6.1 Failure taxonomy

| Phase | Condition | Error shape | Written? |
|---|---|---|---|
| List/metadata | metadata GET fails | `failed to get metadata: …` | no |
| Classification | no ARMOR version | *(silent skip, `SkippedObjects++`)* | no |
| Classification | version out of include list / == 3 | *(silent skip)* | no |
| Pre-validation | wrapped DEK or IV not valid base64/decodable | recorded before any fetch | no |
| DEK unwrap | wrong wrapped length / corrupted AIV | `failed to unwrap DEK: invalid key length: wrapped DEK must be 40 bytes` / `key unwrap failed: invalid AIV` | no |
| Envelope | magic != `ARMR` | `failed to decode envelope header: invalid ARMOR magic` | no |
| Envelope | tail smaller than the HMAC table | `ciphertext too short to contain HMAC table: got N, need M` | no |
| Per-block HMAC | corrupted table or ciphertext | `failed to decrypt: block N: HMAC verification failed` | no |
| Multipart sidecar | sidecar object absent | `missing_sidecar` | no |
| Multipart sidecar | table shorter than blocks × 32 | `HMAC table too short: got N, need M` | no |

Every decrypt-phase failure: `FailedObjects++`, a
`MigrationFailure{Key, Reason, Time}` appended to `result.Failures` **and**
recorded into `fm.state` immediately (under `stateMu`) so periodic saves and
`GetState()` reflect it even if the run is interrupted, the object left
byte-identical, and the loop continues.

### 6.2 Behavioral properties

- **Best-effort, not fail-fast.** One bad object never blocks the bucket.
- **Fail-closed on writes.** The replacement object is written only after
  decryption (and HMAC verification) fully succeeded — no partial outputs.
- **Dry run is representative.** `dryRun` stops after decrypt
  (`migrateObject`), so it reports the same failures as a live run with no
  writes. Note the §8 caveat: dry run also reports the same false success on
  the silent-corruption case.
- **Resumable.** State (`.armor/migration-state.json`) tracks `LastKey`;
  re-runs skip keys ≤ `LastKey`. Failures are not retried specially.
- **Exit code.** `armor migrate` exits 1 with `Migration had N failures.`
  when anything failed.

### 6.3 Edge cases that succeed

All three `edge_cases/` fixtures are production-format and migrate
successfully today — they are the only non-`success`-by-accident fixtures
with exact observed V3 outputs:

| Fixture | Input | Observed V3 output |
|---|---|---|
| `empty_plaintext` | 0 bytes; stored object is 64 bytes (header only) | success; 64-byte V3 object; `plaintext-size: "0"`; sha `e3b0c442…` (empty-string hash). Zero blocks → **no integrity data at all** |
| `single_byte_plaintext` | 1 byte | success; `block-size: "4096"`; sha preserved — the minimal partial block |
| `exact_block_boundary` | 131072 B = exactly 2 × 64 KiB | success; source's 2 × 64 KiB blocks re-emitted as **32 × 4 KiB** — boundary property not preserved by the write path |

---

## 7. Fixture-by-fixture quick reference

Condensed index; the linked document per row has the input JSON, envelope
hex, part maps and per-part counter tables.

### V1 single-PUT — [`v1-single-part-fixtures.md`](v1-single-part-fixtures.md)

| Fixture | Distinguisher | V3 outcome |
|---|---|---|
| `v1_single_put/explicit_version` | explicit version `"1"` | single-PUT, all metadata re-emitted |
| `v1_single_put/implicit_version` | version metadata absent — header-detected | single-PUT; version field added |
| `v1_single_put/minimal_metadata` | no version, no plaintext-size, no sha256 — reconstructed from header | single-PUT |
| `generated_fixtures/v1-single-explicit-short` | 47 B plaintext | single-PUT |
| `generated_fixtures/v1-single-implicit-short` | 47 B, implicit version | single-PUT |

### V2 single-PUT — [`v2-single-part-fixtures.md`](v2-single-part-fixtures.md)

| Fixture | Distinguisher | V3 outcome |
|---|---|---|
| `v2_single_put/standard` | full metadata incl. `etag` | single-PUT; etag carried |
| `generated_fixtures/v2-single-short` | 47 B | single-PUT |

### V1 multipart — [`v1-multipart-fixtures.md`](v1-multipart-fixtures.md)

| Fixture | Distinguisher | V3 outcome (miniature / full-scale 15 MiB) |
|---|---|---|
| `v1_multipart/uniform_parts` | ADR-005 uniform; `part-size` present | single-PUT / multipart 3 × 5 MiB |
| `v1_multipart/variable_final_part` | ADR-010; `part-size` = nominal 3 MiB | single-PUT (stale part-size carried) / multipart 3 × 5 MiB (5 × 3 MiB in) |
| `v1_multipart/non_uniform_parts` | ADR-011; **no `part-size`** | single-PUT (no part-size emitted) / multipart 3 × 5 MiB (1/2/12 MiB in) |
| `generated_fixtures/v1-multipart-uniform` | 256 KiB, standalone-generator crypto | single-PUT, 64 × 4 KiB blocks |

### V2 multipart — [`v2-multipart-fixtures.md`](v2-multipart-fixtures.md)

| Fixture | Distinguisher | V3 outcome (miniature / full-scale 15 MiB) |
|---|---|---|
| `v2_multipart/uniform_parts` | ADR-005 + `etag`; continuous counter stream | single-PUT / multipart 3 × 5 MiB |
| `v2_multipart/variable_final_part` | ADR-010; `part-size` 3145728 | single-PUT (stale part-size carried) / multipart 3 × 5 MiB (5 × 3 MiB in) |
| `v2_multipart/non_uniform_parts` | ADR-011; no `part-size` | single-PUT / multipart 3 × 5 MiB (1/2/12 MiB in) |
| `generated_fixtures/v2-multipart-uniform` | 256 KiB on-disk, 8-hex fp AES-GCM DEK | single-PUT, 64 × 4 KiB blocks |

### Malformed / edge / contradictory — [`malformed-edge-case-fixtures.md`](malformed-edge-case-fixtures.md)

| Fixture | Intended defect | As shipped | Format-aligned expectation |
|---|---|---|---|
| `invalid_version_string` | version `"not-a-number"` | failed at DEK length | **success** — label corrected to 3 |
| `envelope_version_mismatch` | header 0x01, metadata `"2"` | failed at DEK length | no check exists; header-true → success; header-lie → lucky on 1 block, **silent corruption** multi-block |
| `corrupted_hmac_table` | XOR-flipped trailer HMAC | failed at DEK length | `block 0: HMAC verification failed` |
| `corrupted_wrapped_dek_tag` | flipped bits in DEK wrap | failed at DEK length | `key unwrap failed: invalid AIV` |
| `v1_object_v2_metadata` | honest V1 object labeled `"2"` | failed at DEK length | **success** — header wins |
| `v2_object_v1_metadata` | honest V2 object labeled `"1"` | failed at DEK length | **success** — header wins |
| `invalid_envelope_magic` | magic `DE AD BE EF` | failed at DEK length | `invalid ARMOR magic` |
| `truncated_ciphertext` | cut to 164/215 B | failed at DEK length | `block 0: HMAC verification failed` (or table-fit error) |
| `inconsistent_part_metadata` | part-count 999, part-size 1 | failed at DEK length | part metadata never validated; sidecar-consistent → success |
| `multipart_part_size_mismatch` | declared 300 KiB vs actual 512 KiB | failed at DEK length | success; stale `part-size` carried forward below threshold |
| `multipart_contradictory_hashes` | metadata sha ≠ real sha | failed at DEK length | hash never checked; recomputed value written |
| `invalid_sidecar_format` | 14 B garbage sidecar | rewrapped: `HMAC table too short: got 14, need 1024` | same (size-based, key-independent) |
| `truncated_sidecar` | 992 B vs 1024 needed | rewrapped: `HMAC table too short: got 992, need 1024` | same |
| `edge_cases/*` (3) | empty / 1 B / exact boundary | **success ×3** | same |
| `contradictory/version_says_v1_layout_v2` | V2 ciphertext, V1 header+label | **"success" with destroyed plaintext** | with a plaintext cross-check in place: failure |

Group totals: **3 successes with correct data, 13 recorded failures,
1 silent corruption** — and 4 metadata-only defect classes that no code
checks.

---

## 8. Known issues and mitigations

In impact order. Items 1–4 are the fixture-doc "Known Divergences," verified
against the current tree; re-verify before relying on specific line numbers.

1. **No plaintext cross-check → silent corruption on a lying envelope.**
   `decryptSingleObject` picks the counter derivation from the header's
   version byte; HMACs authenticate ciphertext + block index only — not the
   version byte, not the plaintext. An object whose ciphertext was produced
   under a different counter layout than its header declares decrypts to
   garbage, the garbage's SHA is recomputed and written, and migration
   reports success. Empirically demonstrated in pure production crypto
   (`contradictory/version_says_v1_layout_v2`, plus a 70 000-byte synthesis);
   single-block objects dodge it because V1/V2 counters coincide at block 0.
   **Mitigation (recommended fix):** before re-encrypting, compare the
   decrypted plaintext against a declared SHA-256 when one exists
   (`x-amz-meta-armor-sha256`, or the header's `PlaintextSHA` for
   single-PUT); on mismatch, record a failure and leave the object untouched.
   `crypto.ErrPlaintextMismatch` and `EnvelopeHeader.VerifyPlaintextSHA`
   already exist and are used by the canary and restore-verifier paths — the
   migrator is the one decrypt path that skips them.
2. **V2 multipart fixture ciphertext does not match production counters.**
   Both generators XOR with `uint32LE(blockIndex × stride)` zero-padded into
   an all-zero counter block with **no IV**; production derives
   `IV[0:12] ‖ BE32(blockIndex × stride)`. Sidecar HMACs still verify, so
   such input decrypts "successfully" to non-fixture plaintext and migrates
   green — the migrator recomputes SHA rather than comparing. The V1
   miniatures do *not* have this problem. **Mitigation:** regenerate the V2
   fixtures with the production counter derivation before asserting
   `plaintext_sha256` preservation against them; until then assert only
   against the fixtures' own self-consistent values.
3. **Shipped `malformed/*` fixtures cannot reach their intended defects.**
   The standalone generator diverges from production in four ways (AES-GCM
   60-byte DEK wrap vs. AES-KWP 40-byte; BE `PlaintextSize` vs. LE; different
   HMAC-key derivation; LE no-IV counters), so all 13 fail at
   `wrapped DEK must be 40 bytes` before their defect is reached. **Mitigation:**
   regenerate against production crypto (or generate with `internal/crypto`
   itself) and correct the `expected_failure_reason` strings to the observed
   messages.
4. **Migrator re-encryption and verification predate the V3 encryptors.**
   `encryptAsSingle`/`uploadAsMultipart` call the generic
   `Encryptor.Encrypt` with `currentWriteVersion = 3`; `makeCounter` has no
   V3 branch and `computeBlockHMAC` binds only a 32-bit block index, while
   the production V3 write path uses `EncryptV3`/`EncryptPartV3` with
   part-namespaced counters. Post-migration read-back always goes through
   `decryptSingleObject`, which expects an envelope header — objects that
   took the multipart output path don't have one. **Mitigation:** any
   fixture-driven test must verify that migrated objects decrypt under the
   **V3 read path**, not merely that migration reported success; fix the
   verification path before trusting large-object migration results.
5. **Stale `part-size` carry-over.** Below the 5 MiB threshold,
   `buildNewMetadata` copies the source `part-size` onto an object that is
   no longer multipart (e.g. `307200` from `multipart_part_size_mismatch`).
   Not a correctness bug — nothing consumes it on a single-PUT object — but
   it can mislead range-read planners. **Mitigation:** stop copying
   `part-size` on the single-PUT output path (or emit it only on the
   multipart path).
6. **Structural multipart metadata is never validated.** `part-count`,
   `part-size`, and `x-amz-meta-armor-sha256` are consulted by no migration
   code path; outcome is decided entirely by sidecar/ciphertext consistency.
   Real buckets carry stale values from aborted multipart attempts.
   **Mitigation:** validate structural metadata at classification time and
   defect-report contradictions (the §8.1 cross-check subsumes the hash
   case).
7. **No placeholder-HMAC exemption in the migrator.** `Decryptor.Decrypt`
   verifies every sidecar slot strictly; the server's streaming read path
   skips all-zero ADR-011 boundary placeholders. A real ADR-011 object
   carrying placeholders would fail migration while reads of the same object
   succeed. The shipped fixtures never emit placeholders, so the gap is
   unexercised. **Mitigation:** mirror the read path's placeholder skip in
   `decryptMultipartObject`, or pre-scan and report.
8. **Orphaned V1/V2 sidecars.** After migration the object no longer
   references its old `.armor/hmac/<sha256hex(key)>` sidecar, but nothing
   deletes it (the large-object integration test cleans up explicitly).
   **Mitigation:** a post-migration sweep, or accept the storage overhead.
9. **`v3-golden-outcomes.json` (and `.yml`) are illustrative.** The golden
   file's part layouts for variable-final and non-uniform entries do not
   match the committed fixture bytes (e.g. it describes
   `v2_multipart_variable_final_part` as [5, 5, 1] MiB `[80, 80, 16]`; the
   committed bytes are five 3 MiB parts `[48 × 5]`). Only the uniform
   entries match. **Mitigation:** treat the committed fixture bytes and
   their `metadata.json` as authoritative; regenerate the golden file from
   the generator.
10. **Output block size differs from every golden expectation.**
    `encryptAsSingle` hardcodes `blockSize = 4096` (`format_migration.go:644`),
    so migrated small objects differ from their source (65536) and from the
    golden file's `"65536"`, which only holds for the multipart output path.
    Documented here so tests assert `4096` on the single-PUT path.

---

## 9. Production migration guidance

Lessons from the fixture work, ordered as an operator would hit them.

### 9.1 Pre-migration checklist

1. **Inventory before you migrate.** Classify the bucket first: how many
   objects per version (1/2/3/none), how many multipart, and how many
   multipart objects have a sidecar at `.armor/hmac/<sha256hex(key)>`.
   Objects missing a sidecar will fail with `missing_sidecar` — find them
   before the run, not in its failure list.
2. **MEK availability for *all* wrapped DEKs.** Migration unwraps every
   source DEK with the current MEK. Objects wrapped under a retired MEK
   fingerprint fail at unwrap. Confirm the MEK ring covers every fingerprint
   present in `x-amz-meta-armor-wrapped-dek` values (V1 objects carry bare
   bodies — they were all wrapped under the then-active MEK).
3. **Know the 5 MiB routing consequence.** Small multipart objects come out
   as single-PUT objects with 4 KiB blocks and no sidecar; large objects come
   out as 5 MiB-part multipart objects. Downstream tooling that assumes
   "multipart in ⇒ multipart out" or "block size 65536" is wrong after
   migration.
4. **Run dry-run first — and read its failures.** Dry run decrypts everything
   and writes nothing, reporting the same failure set as the live run
   (§6.2). Its counts (`processed/skipped/failed`) are the basis for
   sizing the maintenance window.
5. **Snapshot or verify restore capability.** The §8.1 corruption class and
   every HMAC failure path leave objects untouched — but the silent-corruption
   class **overwrites in place**. Until the plaintext cross-check lands, a
   lying header destroys data with a green summary. Have a point-in-time
   copy (or a verified backup/replica) for the objects being migrated.
6. **Check for ADR-011 placeholder HMACs** if any bucket was written by a
   version that emitted them: those objects fail migration with
   `HMAC verification failed` while ordinary reads succeed (§8.7). Sweep for
   all-zero 32-byte sidecar entries before the run.

### 9.2 Running the migration

7. **Use `--include-versions` deliberately.** The include list gates before
   the already-at-target check. `["1","2"]` migrates both and leaves V3
   objects skipped-as-out-of-scope; an object whose version metadata is
   absent counts as V1.
8. **Expect exit 1 if anything failed** (`Migration had N failures.`). That
   is the designed signal, not a crashed run: every other object was
   processed.
9. **Interrupts are safe.** State persists to `.armor/migration-state.json`
   (`LastKey`); a re-run resumes after the last completed key. Failures are
   *not* retried specially — a failed object fails again on re-run unless the
   object itself changed.
10. **Watch the failure reasons, not just counts.** The reason string tells
    you which repair applies (§6.1): `wrapped DEK must be 40 bytes` (wrong
    MEK or foreign-format object), `invalid ARMOR magic` (possibly repairable
    — restore the 4 magic bytes from a verified copy), `HMAC table too
    short` (restore the sidecar from a replica), `block N: HMAC verification
    failed` (restore from backup; the data cannot self-repair).

### 9.3 Post-migration verification

11. **Verify with the V3 read path, not the migrator's own summary** (§8.4).
    Decrypt a sample of migrated objects — including at least one that took
    the multipart output path, which the migrator's read-back does not
    currently cover — and compare SHA-256 against the pre-migration value
    you recorded in step 1. `internal/restoreverifier` exists for exactly
    this shape of check.
12. **Compare SHA-256 manifests, before vs. after.** Migration preserves
    plaintext SHA-256 by design; any object whose hash changed is the §8.1
    corruption class and needs restore-from-backup. (A hash you never
    recorded cannot be checked after the fact — record it first.)
13. **Plan sidecar cleanup.** Old `.armor/hmac/*` flat sidecars of migrated
    multipart objects are orphaned, not deleted (§8.8). Reclaim the space
    with a post-migration sweep once verification has passed.
14. **Re-run to convergence, then assert zero.** After fixing the causes
    behind individual failures, a final full run should report
    `processed = candidate count, failed = 0`. Anything still failing is a
    permanent-loss candidate for restore-from-backup triage.

---

## 10. Migration FAQ

**Q1. Does a V1 multipart object stay multipart after migration?**
Only if its plaintext exceeds 5 MiB. Routing is by plaintext size alone; the
source layout is irrelevant. A 94-byte or 256 KiB multipart fixture becomes a
V3 single-PUT object.

**Q2. Is the DEK re-used? Is the IV?**
No and no. Every migrated object gets a freshly generated 32-byte DEK
(AES-KWP-wrapped) and a fresh 16-byte IV. Even a V2 source whose DEK is
already in `v2:` format is re-keyed. Plaintext bytes, length and SHA-256 are
the only invariants.

**Q3. Which version wins when the envelope header and `x-amz-meta-armor-version`
disagree?**
For single-PUT objects, the **header** — decryption selects the counter
derivation from the header's version byte; the metadata label is advisory
(routing + rewritten output). For multipart objects there is no header, so
metadata is all there is. The dangerous case is a lying *header* (§8.1); a
lying *label* merely gets corrected.

**Q4. What happens to an object whose version string is garbage?**
It depends on the rest of its metadata. With a wrapped DEK present,
`ParseARMORMetadata` defaults the version to **1** and the object enters the
V1 migration path (a genuine V1 object with a torn label migrates fine and
gets labeled 3). Only when parsing fails outright (required fields missing)
does the direct-parse fallback skip it.

**Q5. Are V1 and V2 handled by different code paths?**
Almost not at all. Same classification shape, same sidecar mechanism, same
HMAC key derivation (`HKDF-SHA256(DEK, "armor-hmac-v1")`), same decrypt
entry point — only the counter stride differs (V1 stride 1, V2 stride
`blockSize/16`), selected by version. V1 wrapped-DEK parsing also accepts the
bare (no-`v2:`) form.

**Q6. What does migration do about V1's cross-part keystream reuse?**
Removes it structurally. V3 counters embed the part number
(`BE16(part)`), so each part is an independent keystream; the same (DEK, IV)
pair can no longer produce identical keystream in two parts.

**Q7. Why does my migrated small object have 4096-byte blocks when everything
else is 65536?**
`encryptAsSingle` hardcodes 4 KiB blocks for the single-PUT output path
(`format_migration.go:644`). Block size is a property of the output path,
not of the source. Known and documented (§8.10).

**Q8. Does migration preserve `part-size`, `etag`, `content-type`?**
`part-size` and `etag` are carried over when present — including *stale*
`part-size` values on objects that stop being multipart (§8.5).
`content-type`, `key-id`, `compressed`, `compression` are preserved. The
`multipart` flag is never copied; it is set only by the multipart output
path. Non-ARMOR user metadata is copied through.

**Q9. What happens to the old HMAC sidecar?**
Nothing — it is neither rewritten nor deleted. The migrated object no longer
references it (single-PUT output) or gets a new gzip-JSON one (multipart
output). Clean up post-verification (§9.3 #13).

**Q10. A real ADR-011 object with zero-placeholder HMACs fails migration but
reads fine. Which is right?**
Both are behaving as coded: the streaming read path skips all-zero
placeholder entries; the migrator's `Decryptor.Decrypt` verifies every slot
strictly (§8.7). Treat the migration failure as a defect to fix in the
migrator, not as corruption.

**Q11. Is dry-run a faithful rehearsal?**
For *failures*, yes — dry run decrypts everything and records the same
failures. For *correctness*, no: dry run cannot surface the silent-corruption
class, because nothing compares decrypted plaintext to a declared hash
(§8.1). A green dry run is necessary but not sufficient.

**Q12. Which fixture artifacts are authoritative when they disagree?**
The committed fixture bytes and their `metadata.json`. `v3_expected` blocks
describe the *source* layout (not migrator output), `v3-golden-outcomes.json`
is illustrative and partially stale, and the `malformed/*`
`expected_failure_reason` strings describe intended behavior the code does
not have (§8.3, §8.9).

**Q13. Where is the code?**
`internal/server/format_migration.go` (migrator),
`internal/backend/backend.go:292` (`ParseARMORMetadata`),
`internal/crypto/{envelope,v3,encryptor,block_table_v3,decryptor,hkdf,fingerprint}.go`
(formats and primitives), `internal/backend/multipart.go`
(`HMACTableSidecarV3`), `cmd/armor/cmd_migrate.go` (CLI),
`tests/fixtures/migration/` (fixtures and generators).

---

## 11. Related documentation

- **Per-fixture detail:** the six companion documents in the header table
- **ADRs:** `docs/adr/005-ctr-counter-stride-fix.md` (V2 stride),
  `docs/adr/010-barman-multipart-incompatibility.md` (variable final part),
  `docs/adr/011-barman-stays-on-armor-non-uniform-multipart.md`
- **Fixture tooling:** `tests/fixtures/migration/README.md`,
  `generate_fixtures.go` (ARMOR-crypto-based miniatures),
  `standalone_generator.go` (independent-crypto oracle),
  `v3-golden-outcomes.json` (+ `.yml`)
- **Tests:** `internal/server/format_migration_test.go`
  (`TestFormatMigrationMultipartToSingle` and the version-parameterized
  mock-backend suite), `tests/integration/format_migration_large_object_test.go`
  (B2-backed legacy-object migration)

---

**Document Version:** 1.0
**Date:** 2026-09-08
**Consolidates:** the five fixture-documentation beads' output (V1 single-part,
V2 single-part, V1 multipart, V2 multipart, malformed/edge-case) plus the
error-handling flow document.
