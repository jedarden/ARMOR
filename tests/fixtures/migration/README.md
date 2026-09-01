# ARMOR V1/V2 Format Migration Fixtures

This directory contains standalone tools and test fixtures for ARMOR's V1/V2 to V3 format migration testing.

## Critical Design Principle: Independence

**The fixture generator MUST NOT import any ARMOR internal packages.**

This is adversarial validation at its core: if the migration code has a bug, the fixture generator must not share that same bug, otherwise tests will pass incorrectly.

### Why Independence Matters

Consider this bug scenario:

1. **V1 counter derivation bug**: The V1 format has a known vulnerability where `counter = blockIndex` instead of `counter = blockIndex * (blockSize/16)`. This causes keystream reuse.

2. **Shared bug risk**: If both the migration decoder and the fixture generator import from `internal/crypto`, they share the same counter derivation code. If that code has a bug, both have the bug.

3. **False negative test**: The fixture generator creates "V1 ciphertext" with the bug, the migration decoder decrypts it with the same bug, and the test passes even though the implementation is wrong.

4. **Independent implementation**: By implementing the generator independently (no `internal/crypto` imports), we ensure bugs in one won't affect the other. If the migration code has a V1 decoding bug, tests using independently-generated fixtures will catch it.

## Standalone Generator: `standalone_generator.go`

This is the **primary fixture generator** for ARMOR format migration testing.

### Key Features

- **Fully independent**: Zero imports from ARMOR internal packages
- **Deterministic**: Uses fixed key material for reproducible fixtures
- **Comprehensive**: Covers V1/V2 single-PUT and multipart layouts
- **Validated**: Implements format specifications from first principles

### What It Implements Independently

The generator reimplements these primitives from the ARMOR format specification:

1. **Envelope header encoding** (`encodeEnvelopeHeader`)
   - Magic number: "ARMR"
   - Version field (0x01 for V1, 0x02 for V2)
   - Block size log2
   - IV, plaintext size, plaintext SHA256

2. **DEK wrapping** (`wrapDEK`)
   - AES-GCM encryption of DEK under MEK
   - 12-byte nonce + 40-byte ciphertext
   - Matches ARMOR's `internal/crypto.WrapDEK` format

3. **HMAC key derivation** (`deriveHMACKey`)
   - HKDF-SHA256 from DEK
   - Matches ARMOR's `internal/crypto.DeriveHMACKey` format

4. **V1 encryption** (`encryptV1`)
   - **Vulnerable counter derivation**: `counter = blockIndex`
   - This is the known V1 keystream reuse bug
   - Required for testing legacy V1 object migration

5. **V2 encryption** (`encryptV2`)
   - **Fixed counter derivation**: `counter = blockIndex * (blockSize/16)`
   - This is the security fix from ADR-005
   - Ensures no keystream reuse

### Usage

```bash
# Generate fixtures to /tmp/armor-fixtures
go run standalone_generator.go /tmp/armor-fixtures
```

### Fixture Structure

Each fixture creates a directory with:

```
<fixture-name>/
├── metadata.json           # Test expectations and properties
├── stored_ciphertext.bin   # Encrypted data as stored on B2
├── object_metadata.json    # B2 object metadata
└── sidecar.bin            # HMAC table (multipart only)
```

### Fixtures Generated

| Fixture | Description | Layout | Version |
|---------|-------------|--------|---------|
| `v1-single-explicit-short` | V1 with explicit version metadata | Single-PUT | V1 |
| `v1-single-implicit-short` | V1 with missing version metadata | Single-PUT | V1 (implicit) |
| `v2-single-short` | V2 single-PUT | Single-PUT | V2 |
| `v1-multipart-uniform` | V1 multipart, 256KB | Multipart | V1 |
| `v2-multipart-uniform` | V2 multipart, 256KB | Multipart | V2 |

### Test Integration

Migration tests should:

1. **Load fixture**: Read `metadata.json`, `stored_ciphertext.bin`, `object_metadata.json`
2. **Create mock backend**: Present fixture data as if it were a B2 object
3. **Run migration**: Call format migration with fixture object
4. **Verify outcome**: Compare against `expected_migration_outcome` in metadata
5. **Verify integrity**: Decrypt migrated object, compute SHA-256, compare to `plaintext_sha256`

## Legacy Generators (DO NOT USE FOR NEW FIXTURES)

Two older generators exist in this directory but are **deprecated** for new fixture generation:

- `v1v2_fixture_generator.go` - Imports from `internal/crypto` (not independent)
- `generate_fixtures.go` - Comprehensive but shares code with migration

### Why They're Problematic

Both generators import `github.com/jedarden/armor/internal/crypto`, which violates the independence principle. They can still be used to understand fixture structure, but new fixtures should use `standalone_generator.go`.

## Format Specifications

### V1 vs V2 Counter Derivation

The critical difference between V1 and V2 is counter derivation:

**V1 (Vulnerable)**:
```go
counter = uint32(blockIndex)  // BUG: causes keystream reuse
```

**V2 (Fixed)**:
```go
aesBlocksPerArmorBlock := blockSize / 16  // 4096 for 64KB blocks
counter = uint32(blockIndex * aesBlocksPerArmorBlock)  // FIXED
```

### Why V1 Is Broken

For 64KB blocks and AES-128 (16-byte blocks):
- V1: block 0 uses counter 0, block 1 uses counter 1
- V2: block 0 uses counter 0, block 1 uses counter 4096

In V1, adjacent 64KB blocks share overlapping keystream:
- `keystream[0][16:] == keystream[1][:65520]`
- This is a cryptographic vulnerability

### Single-PUT vs Multipart

**Single-PUT**:
```
[64-byte envelope header][encrypted blocks][HMAC table]
```

**Multipart**:
```
[encrypted blocks]  (no embedded header)
HMAC table in sidecar: .armor/hmac/<sha256(key)>
```

## Testing Strategy

### Property-Based Testing

Fixtures enable property-based testing of migration:

1. **Round-trip property**: Encrypt → Migrate → Decrypt → Verify SHA-256
2. **Metadata preservation**: All B2 metadata preserved
3. **Version upgrade**: V1/V2 version field updated to V3
4. **Plaintext integrity**: SHA-256 unchanged through migration

### Adversarial Coverage

Every edge case and corruption pattern should have a fixture:
- Empty plaintext
- Single-byte plaintext
- Exact block boundary (no partial last block)
- One byte over boundary
- Invalid version strings
- Corrupted envelope headers
- Mismatched HMAC tables

## Adding New Fixtures

1. **Implement in `standalone_generator.go`**
   - Add a new `Generate*` method
   - Implement encryption independently (no `internal/crypto` imports)
   - Add to `main()` generation list

2. **Run generator**
   ```bash
   go run standalone_generator.go /tmp/test-fixtures
   ```

3. **Verify fixture files**
   - Check `metadata.json` structure
   - Verify ciphertext can be decoded by migration code
   - Confirm SHA-256 matches expected plaintext

4. **Update this README**
   - Document the new fixture in the fixtures table
   - Explain what edge case it tests

## References

- **ADR-005**: V1 keystream reuse vulnerability and V2 fix
- **ADR-010**: Variable final part size (barman incompatibility)
- **ADR-011**: Non-uniform multipart support
- **ADR-016**: Metadata manifest object pattern
- **`docs/notes/format-migration-fixture-schema.md`**: Complete fixture schema

## Verification

To verify the generator produces valid fixtures:

```bash
# Generate fixtures
go run standalone_generator.go /tmp/test-fixtures

# Check fixture structure
ls -la /tmp/test-fixtures/v1-single-explicit-short/

# Verify metadata JSON
cat /tmp/test-fixtures/v1-single-explicit-short/metadata.json | jq .

# Verify object metadata
cat /tmp/test-fixtures/v1-single-explicit-short/object_metadata.json | jq .
```
