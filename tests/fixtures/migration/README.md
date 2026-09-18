# ARMOR V1/V2 Fixture Generation Tooling

This directory contains comprehensive standalone tooling for generating V1/V2 ARMOR fixture data for format migration testing.

## Overview

The fixture generation tools create canonical test data representing all ARMOR format variants:

- **V1 single-PUT** (explicit/implicit/minimal metadata)
- **V2 single-PUT** (standard with full metadata)
- **V1/V2 multipart uniform**
- **V1/V2 multipart variable-final (ADR-010)**
- **V1/V2 multipart non-uniform (ADR-011)**
- **Malformed variants** (invalid version, envelope mismatches, corrupted HMAC, inconsistent metadata)
- **Contradictory variants** (version/layout mismatches)
- **Edge cases** (empty, single-byte, exact block boundary)

## Independence Guarantee

The standalone generator (`standalone_generator.go`) implements all crypto primitives **independently** of the ARMOR codebase:

- No imports from ARMOR internal packages
- Standalone envelope header encoding
- Standalone DEK wrapping (AES-KWP, RFC 5649)
- HKDF-SHA256 HMAC key derivation (`golang.org/x/crypto/hkdf`, info string
  `armor-hmac-v1` — the same value as the reader's `crypto.HMACKeyInfo`)
- Standalone V1/V2 counter derivation (counter block `IV[0:12] || BE uint32`,
  matching the reader's `Decryptor.makeCounter`)

This ensures adversarial validation: if the migration code has a bug, this generator will catch it.

The legacy generator in `canonical/` does **not** have this property — it calls
`internal/crypto` directly, so it can only reproduce what ARMOR already does.
Treat `standalone_generator.go` as the sole fixture oracle.

## Usage

### Using the standalone generator:

```bash
cd tests/fixtures/migration
go run standalone_generator.go .
```

Or use the convenience script:

```bash
cd tests/fixtures/migration
go run standalone_generator.go /tmp/fixture-out
```

Regenerate into a scratch directory and copy only the fixture sets you intend
to replace - a full run rewrites the legacy tree with current (deterministic,
KWP-wrapped) bytes, which the committed legacy dirs do not carry.

### Generated fixtures:

The regeneration set emitted under `generated_fixtures/` also gets a
top-level `manifest.json` (machine-readable, schema below). Each fixture
directory contains:

- `metadata.json` - Fixture metadata (plaintext SHA256, length, source version/layout, expected V3 outcome)
- `stored_ciphertext.bin` - The encrypted data as stored
- `object_metadata.json` - S3 object metadata
- `sidecar.bin` - HMAC sidecar (for multipart fixtures)

`generated_fixtures/manifest.json` records, per fixture:

- `fixture_id` - directory name, entries sorted and complete
- `format_version` - `v1` or `v2`
- `plaintext_sha256`, `plaintext_length` - the plaintext facts
- `v3_expected` - expected V3 layout (`is_multipart`, `part_count`, `blocks_per_part`, `sidecar_path`, …)
- `artifacts` - every emitted file with its byte length and SHA-256

Every entry and artifact hash is read back from disk after the write, so the
manifest provably describes the emitted bytes, and it is itself
deterministic.

### The explicit format-version field

`object_metadata.json` mirrors the object's `x-amz-meta-*` user metadata, and
the field that explicitly records the source format version is
**`x-amz-meta-armor-version`** (`"1"` for V1, `"2"` for V2):

- `generated_fixtures/v1-single-explicit-short` carries it, set to `"1"`. What
  reads it: `backend.ParseARMORMetadata` returns `Version: 1`, which drives
  migrator classification and multipart reads; single-PUT decryption builds
  its decryptor from the envelope header's version byte instead. The two
  agree on this fixture — a header that contradicts the field is the
  `malformed/v1_object_v2_metadata` class (the header wins at decrypt,
  inventory must reject).
- `generated_fixtures/v1-single-implicit-short` deliberately omits it — the
  omission variant. Expected reader behavior: `ParseARMORMetadata` defaults a
  missing (or unparsable) version to 1 for backward compatibility, and that
  default is what classification and multipart reads consult; single-PUT
  decryption never reads the metadata version at all — it builds its
  decryptor from the envelope header's version byte (`crypto.DecodeHeader`
  inside `decryptSingleObject`). This fixture's header byte says `1`,
  agreeing with the default, so the bytes decrypt identically to its explicit
  sibling: the omission exercises the metadata default path, not a different
  decryption.
- Every other generated fixture carries it, set to its `format_version`.

## Fixture Structure

```
tests/fixtures/migration/
├── standalone_generator.go       # Main generator (fully standalone)
├── (the regeneration set generated_fixtures/ is emitted by
│    standalone_generator.go: short plaintexts, KWP-wrapped DEKs)
├── canonical/
│   └── generate_fixtures.go      # Legacy generator (uses ARMOR crypto); kept
│                                 #   in its own directory because both programs
│                                 #   are `package main` declaring the same
│                                 #   symbols, which broke every ./... build
├── README.md                     # This file
├── v3-golden-outcomes.json/.yml  # Expected-outcome manifest (all fixtures,
│                                 #   incl. planning-only entries)
├── v3-golden-outcomes-computed.json/.yml
│                                 # Computed outcomes for the canonical set;
│                                 #   cross-checked by TestGoldenFixturesMigrate
├── v1_single_put/
│   ├── explicit_version/
│   ├── implicit_version/
│   └── minimal_metadata/
├── v2_single_put/
│   └── standard/
├── v1_multipart/
│   ├── uniform_parts/
│   ├── variable_final_part/
│   └── non_uniform_parts/
├── v2_multipart/
│   ├── uniform_parts/
│   ├── variable_final_part/
│   └── non_uniform_parts/
├── generated_fixtures/           # Regeneration set (short plaintexts)
│   ├── manifest.json             # Machine-readable manifest (schema above)
│   ├── v1-single-explicit-short/
│   ├── v1-single-implicit-short/
│   ├── v2-single-short/
│   ├── v1-multipart-uniform/
│   └── v2-multipart-uniform/
├── malformed/                    # 13 corrupt variants; each documents the
│   │                             #   specific failure it must produce
│   ├── invalid_version_string/
│   ├── invalid_envelope_magic/
│   ├── invalid_sidecar_format/
│   ├── envelope_version_mismatch/
│   ├── corrupted_hmac_table/
│   ├── corrupted_wrapped_dek_tag/
│   ├── inconsistent_part_metadata/
│   ├── truncated_ciphertext/
│   ├── truncated_sidecar/
│   ├── multipart_part_size_mismatch/
│   ├── multipart_contradictory_hashes/
│   ├── v1_object_v2_metadata/
│   └── v2_object_v1_metadata/
├── contradictory/
│   └── version_says_v1_layout_v2/
└── edge_cases/
    ├── empty_plaintext/
    ├── single_byte_plaintext/
    └── exact_block_boundary/
```

## Fixture Inventory and Expected V3 Outcomes

Every fixture directory below carries a documented expected V3 conversion
outcome: the target layout for valid fixtures, the specific failure a corrupt
fixture must produce otherwise. The long-form expectations live in
`v3-golden-outcomes.json` / `.yml` (this table is the index); the values in
`v3-golden-outcomes-computed.json` are cross-checked by
`TestGoldenFixturesMigrate`, and every pass/fail class in the table is pinned
by `internal/server/format_migration_fixture_matrix_test.go`
(`goldenFixtureMatrix`), which fails if a fixture dir appears on disk without
a row, or a row without its fixture.

Enforcement stages for corrupt fixtures, strictest first:

- **decrypt** — the legacy read path itself rejects the bytes, with the error
  identifying the defect (`expected_error_class` in the manifest).
- **accounting** — pure byte math catches it before any crypto (sidecar
  length vs 32-byte HMAC entries; entries vs block count).
- **classify** — decidable from metadata/structure only; the dry-run migrator
  must fail the object and process nothing.

### Valid fixtures (success: migrate, then read back the documented plaintext)

| Fixture | Source | Expected V3 layout |
|---|---|---|
| `v1_single_put/explicit_version` | V1 single, explicit version meta | single, 1 block; version → 3; DEK re-wrapped to v2 fingerprint format; counter derivation fixed |
| `v1_single_put/implicit_version` | V1 single, no version meta | single, 1 block; version detected from envelope header, then added to metadata |
| `v1_single_put/minimal_metadata` | V1 single, minimal meta | single, 1 block; missing metadata reconstructed from envelope header |
| `v2_single_put/standard` | V2 single, full meta | single, 1 block; version 2 → 3; already stride-safe counters |
| `v1_multipart/uniform_parts` | V1 multipart-uniform | structure preserved; per-(part, block) independent counters; sidecar table migrated |
| `v1_multipart/variable_final_part` | V1 multipart, ADR-010 | variable final part preserved; per-part independence |
| `v1_multipart/non_uniform_parts` | V1 multipart, ADR-011 | arbitrary part sizes preserved (no 64 KB alignment constraint) |
| `v2_multipart/uniform_parts` | V2 multipart-uniform | structure preserved; per-part independence completes |
| `v2_multipart/variable_final_part` | V2 multipart, ADR-010 | variable final part preserved; version bump |
| `v2_multipart/non_uniform_parts` | V2 multipart, ADR-011 | non-uniform parts preserved; version bump |
| `edge_cases/empty_plaintext` | V1, 0 bytes | valid empty V3 object; version bump only |
| `edge_cases/single_byte_plaintext` | V1, 1 byte | single, 1 partial block; padding handled by AEAD |
| `edge_cases/exact_block_boundary` | V1, 131072 bytes (2 × 64 KiB) | blocks exactly fill; no partial last block |
| `generated_fixtures/v1-single-explicit-short` | V1 single, 47 B | single, 1 block (regeneration set) |
| `generated_fixtures/v1-single-implicit-short` | V1 single implicit, 47 B | single, 1 block; implicit detection |
| `generated_fixtures/v2-single-short` | V2 single, 47 B | single, 1 block |
| `generated_fixtures/v1-multipart-uniform` | V1 multipart, 4 × 64 KiB blocks | 1 S3 part, 4 blocks; sidecar migrated |
| `generated_fixtures/v2-multipart-uniform` | V2 multipart, 4 × 64 KiB blocks | 1 S3 part, 4 blocks; sidecar migrated |

### Corrupt fixtures (failure: fail closed at the pinned stage)

| Fixture | Defect | Required failure (stage: what must fire) |
|---|---|---|
| `malformed/invalid_envelope_magic` | header magic `0xDEADBEEF`, not `ARMR` | decrypt: `invalid ARMOR magic` — header undecodable |
| `malformed/truncated_ciphertext` | stored bytes < header-declared table + data | decrypt: `ciphertext too short to contain HMAC table` |
| `malformed/corrupted_hmac_table` | bit flip in HMAC table | decrypt: `HMAC verification failed` |
| `malformed/corrupted_wrapped_dek_tag` | bit flip in wrapped-DEK auth tag | decrypt: `key unwrap failed` — KWP/auth-tag verification |
| `malformed/invalid_sidecar_format` | sidecar is neither a sidecar document nor a raw HMAC table | accounting: sidecar size not a multiple of the 32-byte HMAC |
| `malformed/truncated_sidecar` | final 32-byte HMAC entry missing | accounting: fewer HMAC entries than blocks |
| `malformed/invalid_version_string` | unparsable version metadata | classify: reader deliberately defaults to V1 (backward compat); dry-run migrator must fail the object |
| `malformed/inconsistent_part_metadata` | part count/size contradicts actual structure | classify: inventory accounting rejects |
| `malformed/multipart_part_size_mismatch` | declared part size (307200 B) vs actual (524288 B) | classify: derived part boundaries contradict metadata |
| `malformed/multipart_contradictory_hashes` | metadata sha256 ≠ envelope-header sha256 | classify: integrity cannot be established against either digest |
| `malformed/envelope_version_mismatch` | header V1, metadata V2 | classify: header-vs-metadata version compare |
| `malformed/v1_object_v2_metadata` | genuine V1 object claiming V2 | classify: version compare; V2 derivation cannot decrypt V1 ciphertext |
| `malformed/v2_object_v1_metadata` | genuine V2 object claiming V1 | classify: version compare; V1 derivation cannot decrypt V2 ciphertext |
| `contradictory/version_says_v1_layout_v2` | metadata + envelope header claim V1; stored layout is V2 (counter = blockIndex × 4096 AES blocks per 64 KiB block). Multi-block: 262144 B plaintext (4 × 64 KiB, plaintext SHA-256 `2312394b…`), so the contradiction is real — block 0 coincides (both derivations produce counter 0) and block 1 diverges (V1 derives counter 1, reusing bytes [16, 65536) of block 0's keystream; V2 wrote counter 4096) | classify: dry-run must fail the object — V1 derivation decrypts blocks 1–3 to garbage, so the header plaintext SHA-256 cannot match and plaintext-integrity enforcement fails the object closed |

No corrupt fixture may pass through every layer: if the read path and
accounting both accept it and the dry run processes it, the matrix test fails
even when individual layers had no opinion.

Entries in the golden manifest without an on-disk directory
(`v1_multipart_sidecar_missing`, `edge_case_very_large_object`,
`legacy_wrapped_dek_*`, …) are planning-only variants listed in the
manifest's `summary`; they arm in the matrix automatically when generated.



Each fixture includes:

## Fixture Metadata

Each fixture directory contains a `metadata.json` declaring:

- `plaintext_sha256`: SHA-256 hash of the original plaintext
- `plaintext_length`: Length of the original plaintext
- `source_version`: V1, V2, or malformed
- `source_layout`: single, multipart-uniform, multipart-variable-final, multipart-nonuniform
- `v3_expected`: Expected V3 outcome (is_multipart, part_count, blocks_per_part, compression_used, sidecar_path)
- `description`: Human-readable description
- `expected_migration_outcome`: success, failure, or skip
- `expected_failure_reason`: Reason for expected failure (if applicable)

## Key Features

### V1 Vulnerable Counter Derivation
The generator accurately replicates the V1 CTR keystream reuse bug:
- V1 counter = blockIndex (BUGGY - causes keystream reuse)
- V2 counter = blockIndex × (blockSize / 16) (FIXED)

### Comprehensive Coverage
- All V1/V2 format variants
- ADR-010 variable-final-part patterns
- ADR-011 non-uniform multipart patterns
- Malformed and contradictory variants for negative testing
- Edge cases for boundary conditions

### Deterministic Generation
- Fixed MEK: 0x01, 0x02, ..., 0x20
- Fixed DEK: 0x02, 0x03, ..., 0x21
- Fixed IV: 0x03, 0x04, ..., 0x12
- DEK wrapping is AES-KWP (RFC 5649), which has no nonce
- Plaintexts are fixed patterns (`i % 256`, an alphabet cycle, …), never random
- No clock or unseeded randomness anywhere in the generator; the only
  non-stdlib import is `golang.org/x/crypto/hkdf` (see Independence Guarantee)

This ensures reproducible fixtures across runs.

## Verifying Determinism

Two consecutive runs must produce byte-identical `generated_fixtures/` trees:

```bash
go run tests/fixtures/migration/standalone_generator.go /tmp/fixture-run1
go run tests/fixtures/migration/standalone_generator.go /tmp/fixture-run2

# byte-for-byte comparison of both trees
diff -r /tmp/fixture-run1/generated_fixtures /tmp/fixture-run2/generated_fixtures

# same comparison as sha256 manifests
(cd /tmp/fixture-run1 && find generated_fixtures -type f | sort | xargs sha256sum) > /tmp/run1.sums
(cd /tmp/fixture-run2 && find generated_fixtures -type f | sort | xargs sha256sum) > /tmp/run2.sums
diff /tmp/run1.sums /tmp/run2.sums   # must be empty
```

`go test ./tests/fixtures/migration/` enforces this durably:
`TestRegenerationSetIsDeterministic` runs the generator twice and compares the
trees, `TestCommittedRegenerationSetMatchesGenerator` fails if the committed
`generated_fixtures/` ever drifts from what the current generator produces,
and `TestRegenManifestCoversEveryFixture` pins the manifest schema and
cross-checks every entry against the fixture bytes on disk.

Last verified 2026-09-16: two consecutive runs produced identical
`sha256sum` output (18 files) and `diff -r` reported no differences; the
committed `generated_fixtures/` tree is exactly the run-1 output.

## Validating the Committed Set Against the Manifest

`generated_fixtures_validation_test.go` is the machine-checkable harness over
the whole committed set, driven entirely by
`generated_fixtures/manifest.json` as recorded data:

- `TestRegenFixturesMatchManifestAndDecrypt` — for EVERY manifest entry:
  every artifact still hashes to its recorded size and SHA-256; the
  per-fixture `metadata.json` agrees with the manifest row; the stored bytes
  match the V1/V2 layout the recorded `v3_expected` fields describe
  (single-PUT: 64-byte envelope header + ciphertext + embedded HMAC table
  sized `ceil(plaintext_length / block_size) × 32`, with the header's version
  byte, plaintext size and SHA-256 equal to the manifest row; multipart:
  headerless body plus a `sidecar.bin` of exactly
  `sum(blocks_per_part) × 32` bytes, `part_count` and `blocks_per_part`
  derivable from the recorded plaintext/part/block sizes, `sidecar_path` in
  the `.armor/hmac/<64-hex>` shape); and the legacy reader
  (`backend.ParseARMORMetadata` + `internal/crypto` `UnwrapDEK` /
  `DecodeHeader` / `NewDecryptorWithVersion` / `Decrypt` — the same
  primitives the production V1/V2 read path uses) decrypts every valid
  fixture to exactly the recorded plaintext SHA-256 and length.
- `TestRegenFixtureTamperIsDetected` — the loud-failure spot-check: for every
  fixture, flipping one bit of the ciphertext body must fail the legacy read,
  and so must flipping one bit of the HMAC table (the embedded table for
  single-PUT, `sidecar.bin` for multipart).

Expectation discipline: the `v3_expected` fields are consumed as recorded
data only — compared against facts derived from the artifacts and object
metadata, never recomputed by the migration implementation. The generator
itself stays independent of ARMOR packages (see "Independence Guarantee");
only the validation test imports `internal/`, because its job is to prove the
committed bytes satisfy the reader.

### Tamper spot-check

Flip one bit anywhere in
`generated_fixtures/v2-single-short/stored_ciphertext.bin` and run
`go test ./tests/fixtures/migration/`: both harness tests fail immediately
with `tampered or regenerated on disk (manifest records ... bytes sha256 ...)`,
naming the file and both digests. Restore with
`git checkout -- generated_fixtures/v2-single-short/stored_ciphertext.bin`.

Last verified 2026-09-17: bit flip at offset 70 failed both
`TestRegenFixturesMatchManifestAndDecrypt/v2-single-short` and
`TestRegenFixtureTamperIsDetected/v2-single-short`; after restore, the full
package is green. Editing the manifest to match tampered bytes does not get
past the harness — the HMAC verification inside the legacy read then fails in
the same run (`TestRegenFixtureTamperIsDetected` proves that path fires).

## Validation

Fixtures can be validated against the migration code:

1. Load fixture metadata
2. Verify plaintext SHA256 matches `plaintext_sha256` field
3. Run migration on `stored_ciphertext.bin`
4. Compare output with `v3_expected` fields
5. Verify `expected_migration_outcome` matches actual result

## Integration with Tests

- `internal/server/format_migration_test.go` and
  `format_migration_golden_test.go` validate whatever fixtures exist at run
  time; `v3-golden-outcomes-computed.json` is the golden input to
  `TestGoldenFixturesMigrate`.
- `internal/server/format_migration_fixture_matrix_test.go` pins each
  fixture to its documented pass/fail class (the "Enforcement stages" table
  above) and enforces bidirectional coverage between `goldenFixtureMatrix`
  and the on-disk fixture tree. A new fixture directory cannot land without
  declaring its class, and a class whose fixture disappears fails.

## Maintenance

When adding new fixture variants:

1. Add generator method to `FixtureGenerator` in `standalone_generator.go`
2. Add generation call to `main()` function
3. Update fixture documentation
4. Add corresponding test cases

## Related Documentation

- ADR-010: Variable Final Part multipart handling
- ADR-011: Non-uniform multipart support
- `docs/notes/format-migration-*` for migration design notes
