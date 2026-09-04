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
- Standalone DEK wrapping (AES-GCM)
- Standalone HMAC key derivation
- Standalone V1/V2 counter derivation

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
./generate_fixtures.sh
```

### Generated fixtures:

Each fixture directory contains:

- `metadata.json` - Fixture metadata (plaintext SHA256, length, source version/layout, expected V3 outcome)
- `stored_ciphertext.bin` - The encrypted data as stored
- `object_metadata.json` - S3 object metadata
- `sidecar.bin` - HMAC sidecar (for multipart fixtures)

## Fixture Structure

```
tests/fixtures/migration/
├── standalone_generator.go       # Main generator (fully standalone)
├── generate_fixtures.sh          # Convenience script
├── canonical/
│   └── generate_fixtures.go      # Legacy generator (uses ARMOR crypto); kept
│                                 #   in its own directory because both programs
│                                 #   are `package main` declaring the same
│                                 #   symbols, which broke every ./... build
├── README.md                     # This file
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
├── malformed/
│   ├── invalid_version_string/
│   ├── envelope_version_mismatch/
│   ├── corrupted_hmac_table/
│   └── inconsistent_part_metadata/
├── contradictory/
│   └── version_says_v1_layout_v2/
└── edge_cases/
    ├── empty_plaintext/
    ├── single_byte_plaintext/
    └── exact_block_boundary/
```

## Fixture Metadata

Each fixture includes:

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

This ensures reproducible fixtures across runs.

## Validation

Fixtures can be validated against the migration code:

1. Load fixture metadata
2. Verify plaintext SHA256 matches `plaintext_sha256` field
3. Run migration on `stored_ciphertext.bin`
4. Compare output with `v3_expected` fields
5. Verify `expected_migration_outcome` matches actual result

## Integration with Tests

See `internal/server/format_migration_test.go` for examples of how these fixtures are used in migration tests.

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
