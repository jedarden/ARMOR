# V1 Single-Part Fixture V3 Conversions

This document catalogs all V1 single-part fixtures and their expected V3 conversion outcomes.

**Document Version:** 1.0  
**Generated:** 2026-09-01  
**Scope:** All V1 single-PUT fixtures in `tests/fixtures/migration/v1_single_put/` and `tests/fixtures/migration/generated_fixtures/`

---

## V1 Single-PUT Format Overview

### V1 Envelope Structure

All V1 single-part fixtures use the following envelope format (binary layout):

```
Offset  | Size  | Field        | Description
--------|-------|--------------|----------------------------------------
0x00    | 4     | Magic        | "ARMR" (0x41524d52)
0x04    | 1     | Version      | 0x01 (V1)
0x05    | 16    | IV           | AES-256-CTR initialization vector
0x15    | 4     | Reserved     | Counter space (must be zero in V1)
0x19    | 32    | Plaintext    | SHA-256 hash of original plaintext
0x39    | 8     | Length       | Original plaintext length (little-endian)
0x41    | var   | Ciphertext   | AES-256-CTR encrypted data
```

**Key V1 Properties:**
- **Counter Derivation:** `counter = blockIndex * 4096` (vulnerable to nonce reuse)
- **DEK Wrapping:** Version 1 format (simple base64 encoding)
- **Block Size:** 65536 bytes (64 KiB)

### V3 Target Format

After migration, V1 fixtures become V3 single-part objects with:

```
Offset  | Size  | Field        | Description
--------|-------|--------------|----------------------------------------
0x00    | 4     | Magic        | "ARMR" (0x41524d52)
0x04    | 1     | Version      | 0x03 (V3)
0x05    | 16    | IV           | AES-256-CTR initialization vector
0x15    | 4     | Counter Base | Per-part independent counter base
0x19    | 32    | Plaintext    | SHA-256 hash of original plaintext
0x39    | 8     | Length       | Original plaintext length (little-endian)
0x41    | var   | Ciphertext   | AES-256-CTR encrypted data (re-encrypted)
```

**Key V3 Properties:**
- **Counter Derivation:** Per-part independent (fixes V1 vulnerability)
- **DEK Wrapping:** Version 2 format (`v2:<fingerprint>:<base64>`)
- **Block Size:** 65536 bytes (unchanged)

---

## Fixture Catalog

### 1. explicit_version

**Location:** `tests/fixtures/migration/v1_single_put/explicit_version/`

#### Input Structure

**metadata.json:**
```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v1",
  "source_layout": "single",
  "v3_expected": {
    "is_multipart": false,
    "compression_used": false
  },
  "description": "V1 single-PUT with explicit version metadata",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**
```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-version": "1",
  "x-amz-meta-armor-wrapped-dek": "yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**stored_ciphertext.bin structure:**
- Magic: `ARMR`
- Version: `0x01`
- IV: 16 bytes (`03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f 10 11 12`)
- Reserved: 4 zero bytes
- Plaintext SHA256: 32 bytes
- Plaintext Length: 8 bytes (`0x0000005E` = 94)
- Ciphertext: 94 bytes (encrypted with vulnerable V1 counter derivation)

#### Expected V3 Output

**Metadata Fields:**
```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<base64>",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c"
}
```

**Object Structure:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1]
- `compression_used`: false
- `sidecar_path`: null
- `manifest_reference`: null

**Transformation Notes:**
1. Version upgraded from 1 to 3
2. DEK re-wrapped with v2 fingerprint format
3. Counter derivation changed from vulnerable (`blockIndex * 4096`) to per-part independent
4. No structural changes (single-PUT → single-PUT)

---

### 2. implicit_version

**Location:** `tests/fixtures/migration/v1_single_put/implicit_version/`

#### Input Structure

**metadata.json:**
```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v1",
  "source_layout": "single",
  "v3_expected": {
    "is_multipart": false,
    "compression_used": false
  },
  "description": "V1 single-PUT with missing version metadata (implicit V1)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**
```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-wrapped-dek": "yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**Key Difference:** No `x-amz-meta-armor-version` field. Version must be detected from envelope header (magic + version byte).

**stored_ciphertext.bin structure:**
- Same envelope format as `explicit_version`
- Version detection comes from `0x01` byte at offset 0x04

#### Expected V3 Output

**Metadata Fields:**
```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<base64>",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c"
}
```

**Object Structure:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1]
- `compression_used`: false

**Transformation Notes:**
1. Version detection from envelope header (magic + version byte)
2. Version field added to metadata (was absent in source)
3. V1 vulnerable counter derivation fixed
4. Same plaintext integrity preserved

---

### 3. minimal_metadata

**Location:** `tests/fixtures/migration/v1_single_put/minimal_metadata/`

#### Input Structure

**metadata.json:**
```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v1",
  "source_layout": "single",
  "v3_expected": {
    "is_multipart": false,
    "compression_used": false
  },
  "description": "V1 single-PUT with minimal metadata (only required fields)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**
```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-wrapped-dek": "yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**Key Difference:** Missing optional metadata fields:
- No `x-amz-meta-armor-version` (implicit V1 detection)
- No `x-amz-meta-armor-plaintext-size`
- No `x-amz-meta-armor-sha256`

These fields must be reconstructed from the envelope header during migration.

**stored_ciphertext.bin structure:**
- Same envelope format as other fixtures
- All missing metadata reconstructable from header:
  - `plaintext_size` from offset 0x39
  - `sha256` from offset 0x19

#### Expected V3 Output

**Metadata Fields:**
```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<base64>",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c"
}
```

**Object Structure:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1]
- `compression_used`: false

**Transformation Notes:**
1. Missing optional metadata fields reconstructed from envelope header
2. Version field added
3. DEK re-wrapped with v2 format
4. Counter derivation fixed

---

### 4. v1-single-explicit-short

**Location:** `tests/fixtures/migration/generated_fixtures/v1-single-explicit-short/`

#### Input Structure

**metadata.json:**
```json
{
  "plaintext_sha256": "42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe",
  "plaintext_length": 47,
  "source_version": "v1",
  "source_layout": "single",
  "v3_expected": {
    "is_multipart": false,
    "compression_used": false
  },
  "description": "V1 single-PUT with explicit version metadata",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**
```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-plaintext-size": "47",
  "x-amz-meta-armor-sha256": "42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe",
  "x-amz-meta-armor-version": "1",
  "x-amz-meta-armor-wrapped-dek": "pIELDGkx0GdneWjs6kT6g3EwJLV+gDNsmmhXf9WkPlzcVGnptl8QuDmU54zVBM4OyidFAi2Cr+rOilyj"
}
```

**stored_ciphertext.bin structure:**
- Magic: `ARMR`
- Version: `0x01`
- IV: 16 bytes (`03 04 05 06 07 08 09 0a 0b 0c 0d 0e 0f 10 11 12`)
- Reserved: 4 zero bytes
- Plaintext SHA256: 32 bytes
- Plaintext Length: 8 bytes (`0x0000002F` = 47)
- Ciphertext: 47 bytes

#### Expected V3 Output

**Metadata Fields:**
```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<base64>",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "47",
  "x-amz-meta-armor-sha256": "42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe"
}
```

**Object Structure:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1]
- `compression_used`: false

**Transformation Notes:**
- Same conversion path as `explicit_version`
- Shorter plaintext (47 bytes vs 94 bytes)
- Demonstrates V1→V3 migration works for small objects

---

### 5. v1-single-implicit-short

**Location:** `tests/fixtures/migration/generated_fixtures/v1-single-implicit-short/`

#### Input Structure

**metadata.json:**
```json
{
  "plaintext_sha256": "42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe",
  "plaintext_length": 47,
  "source_version": "v1",
  "source_layout": "single",
  "v3_expected": {
    "is_multipart": false,
    "compression_used": false
  },
  "description": "V1 single-PUT with missing version metadata (implicit V1)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**
```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-plaintext-size": "47",
  "x-amz-meta-armor-sha256": "42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe",
  "x-amz-meta-armor-wrapped-dek": "8aacfqNwLnCCdcK46IQRHaHc/ypoMhmEPjrh6/O+eEM5qaWFEnxHhlYRciQQNE3a0v0DaSwFDnmI9JzP"
}
```

**Key Difference:** No `x-amz-meta-armor-version` field (implicit V1 detection).

**stored_ciphertext.bin structure:**
- Same envelope format as `v1-single-explicit-short`
- Version detected from `0x01` byte at offset 0x04

#### Expected V3 Output

**Metadata Fields:**
```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<base64>",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "47",
  "x-amz-meta-armor-sha256": "42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe"
}
```

**Object Structure:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1]
- `compression_used`: false

**Transformation Notes:**
- Same conversion path as `implicit_version`
- Version detection from envelope header
- Demonstrates implicit version detection works for small objects

---

## Conversion Summary Matrix

| Fixture Name | Version Detection | Missing Fields Reconstructed | Size | Expected Outcome |
|--------------|-------------------|------------------------------|------|------------------|
| `explicit_version` | Explicit (`x-amz-meta-armor-version: 1`) | None | 94 bytes | Success |
| `implicit_version` | Implicit (envelope header) | None | 94 bytes | Success |
| `minimal_metadata` | Implicit (envelope header) | `version`, `plaintext-size`, `sha256` | 94 bytes | Success |
| `v1-single-explicit-short` | Explicit (`x-amz-meta-armor-version: 1`) | None | 47 bytes | Success |
| `v1-single-implicit-short` | Implicit (envelope header) | None | 47 bytes | Success |

---

## Common V3 Conversion Steps

All V1 single-part fixtures follow the same conversion path:

### 1. Version Detection
- **Explicit:** Check `x-amz-meta-armor-version` metadata field
- **Implicit:** Parse envelope header, verify magic bytes (`ARMR`), read version byte (`0x01`)

### 2. Metadata Reconstruction
If missing fields exist in source metadata:
- `x-amz-meta-armor-plaintext-size` → Read from envelope offset `0x39`
- `x-amz-meta-armor-sha256` → Read from envelope offset `0x19`
- `x-amz-meta-armor-version` → Set to `"3"` (was implicit in V1)

### 3. DEK Re-wrapping
- Parse existing V1 wrapped DEK from `x-amz-meta-armor-wrapped-dek`
- Unwrap with master key
- Re-wrap with V2 format: `v2:<fingerprint>:<base64>`

### 4. Counter Space Fix
- **V1 (vulnerable):** `counter = blockIndex * 4096`
- **V3 (secure):** Per-part independent counter base

### 5. Re-encryption
- Decrypt ciphertext with V1 counter derivation
- Re-encrypt with V3 per-part counters
- Preserve IV, plaintext length, and SHA256 hash

### 6. Metadata Update
- Set `x-amz-meta-armor-version` to `"3"`
- Update `x-amz-meta-armor-wrapped-dek` to V2 format
- Keep `x-amz-meta-armor-iv`, `x-amz-meta-armor-block-size`, `x-amz-meta-armor-plaintext-size`, `x-amz-meta-armor-sha256`

---

## V3 Single-PUT Object Layout

After migration, all V1 single-part fixtures become:

```
┌─────────────────────────────────────────────────────────────┐
│                    V3 Single-PUT Object                     │
├─────────────────────────────────────────────────────────────┤
│ Metadata (S3 Object Metadata):                               │
│   x-amz-meta-armor-version: "3"                             │
│   x-amz-meta-armor-wrapped-dek: "v2:<fingerprint>:<base64>"  │
│   x-amz-meta-armor-iv: "<base64>"                            │
│   x-amz-meta-armor-block-size: "65536"                       │
│   x-amz-meta-armor-plaintext-size: "<original>"              │
│   x-amz-meta-armor-sha256: "<original>"                      │
├─────────────────────────────────────────────────────────────┤
│ Ciphertext Blob (stored_ciphertext.bin):                      │
│   [Envelope Header 65 bytes]                                 │
│   [Encrypted Data]                                            │
└─────────────────────────────────────────────────────────────┘
```

**Key properties:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1] (single block)
- `compression_used`: false
- `sidecar_path`: null (no sidecar in single-part)
- `manifest_reference`: null (no multipart manifest)

---

## Testing Coverage

These fixtures validate the migration path covers:

✅ **Version detection** (explicit vs implicit)  
✅ **Metadata completeness** (full, partial, minimal)  
✅ **Object sizes** (small: 47 bytes, medium: 94 bytes)  
✅ **DEK re-wrapping** (V1 → V2 format)  
✅ **Counter derivation fix** (vulnerable → secure)  
✅ **Plaintext integrity preservation** (SHA256 verification)  
✅ **Backward compatibility** (V1 envelope parsing)  

---

## Related Documentation

- **V1 Multipart Fixtures:** See `v1-multipart-fixtures.md`
- **V2 Fixtures:** See `v2-fixtures.md` (TODO)
- **Migration Implementation:** See `cmd/armor/cmd_migrate.go`
- **Golden Outcomes:** See `tests/fixtures/migration/v3-golden-outcomes.json`

---

**Document End**
