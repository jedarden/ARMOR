# V2 Single-Part Fixture V3 Conversions

This document catalogs all V2 single-part fixtures and their expected V3 conversion outcomes.

**Document Version:** 1.0  
**Generated:** 2026-09-01  
**Scope:** All V2 single-PUT fixtures in `tests/fixtures/migration/v2_single_put/` and `tests/fixtures/migration/generated_fixtures/`

---

## V2 Single-PUT Format Overview

### V2 Envelope Structure

All V2 single-part fixtures use the following envelope format (64-byte header):

```
Offset  | Size  | Field          | Description
--------|-------|----------------|----------------------------------------
0x00    | 4     | Magic          | "ARMR" (0x41524d52)
0x04    | 1     | Version        | 0x02 (V2)
0x05    | 1     | BlockSizeLog2  | log2(block_size), e.g., 16 for 64KB
0x06    | 16    | IV             | AES-256-CTR initialization vector
0x16    | 8     | PlaintextSize  | Original plaintext length (little-endian)
0x1E    | 32    | PlaintextSHA   | SHA-256 hash of original plaintext
0x3E    | 2     | Reserved       | Reserved for future use (pad to 64 bytes)
0x40    | var   | Ciphertext     | AES-256-CTR encrypted data
```

**Key V2 Properties:**
- **Counter Derivation:** `counter = blockIndex * (blockSize / 16)` (FIXED - no keystream reuse)
- **DEK Wrapping:** Version 2 format (`v2:<fingerprint>:<base64>`)
- **Block Size:** 65536 bytes (64 KiB, BlockSizeLog2 = 16)
- **Security:** V2 fixes the CTR keystream reuse vulnerability from V1

### V3 Target Format

After migration, V2 fixtures become V3 single-part objects with:

```
Offset  | Size  | Field          | Description
--------|-------|----------------|----------------------------------------
0x00    | 4     | Magic          | "ARMR" (0x41524d52)
0x04    | 1     | Version        | 0x03 (V3)
0x05    | 1     | BlockSizeLog2  | log2(block_size)
0x06    | 16    | IV             | AES-256-CTR initialization vector
0x16    | 8     | PlaintextSize  | Original plaintext length (little-endian)
0x1E    | 32    | PlaintextSHA   | SHA-256 hash of original plaintext
0x3E    | 2     | Reserved       | Compression flags (0x00 = none)
0x40    | var   | Ciphertext     | AES-256-CTR encrypted data
0x40+   | var   | HMAC Table     | Per-block HMAC-SHA256 entries (32 bytes each)
```

**Key V3 Properties:**
- **Counter Derivation:** Per-part independent (IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock))
- **DEK Wrapping:** Version 2 format (unchanged from V2)
- **Block Size:** 65536 bytes (unchanged)
- **HMAC:** Per-(part,block) HMAC-SHA256 for integrity verification
- **Compression:** Flags in Reserved[0] (0x00 = uncompressed)

---

## Fixture Catalog

### 1. standard

**Location:** `tests/fixtures/migration/v2_single_put/standard/`

#### Input Structure

**metadata.json:**
```json
{
  "plaintext_sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "plaintext_length": 94,
  "source_version": "v2",
  "source_layout": "single",
  "v3_expected": {
    "is_multipart": false,
    "compression_used": false
  },
  "description": "V2 single-PUT with full metadata (fixed counter derivation)",
  "expected_migration_outcome": "success"
}
```

**object_metadata.json (S3 object metadata):**
```json
{
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-etag": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-iv": "AwQFBgcICQoLDA0ODxAREg==",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-version": "2",
  "x-amz-meta-armor-wrapped-dek": "v2:ae216c2ef5247a37:yuqSInsrWAcC9/I2eXdQEU5j8GFYSShpTxvCWNRrGpqAtZPdbBePJQ=="
}
```

**Envelope Header (first 64 bytes of stored_ciphertext.bin):**
```
Offset  | Hex                      | Field          | Value
--------|--------------------------|----------------|----------------------------------------
0x00    | 41 52 4d 52             | Magic          | "ARMR"
0x04    | 02                       | Version        | 0x02 (V2)
0x05    | 10                       | BlockSizeLog2  | 16 (65536 bytes)
0x06    | 03 04 05 06 07 08       | IV[0:10]       | 03 04 05 06 07 08 09 0a 0b 0c
        | 09 0a 0b 0c 0d 0e       | IV[10:16]      | 0d 0e 0f 10 11 12
0x16    | 5e 00 00 00 00 00 00 00 | PlaintextSize  | 94 bytes (0x5E)
0x1E    | a7 f6 f6 d8 cf d8 a8 f4 | PlaintextSHA   | a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c
        | 87 d7 2b d3 17 03 a5 75 |                |
        | f9 38 85 fe 6c fe a9 84 |                |
        | 3f 8a 47 52 eb 69 f1 2c |                |
0x3E    | 00 00                    | Reserved       | Padding to 64 bytes
0x40    | 69 8f a1 aa...           | Ciphertext     | [94 bytes encrypted]
```

**V2 Counter Derivation (FIXED):**
- Formula: `counter = blockIndex * (blockSize / 16)`
- For this fixture: `counter = blockIndex * 4096`
- **Security Fix:** Strides by block size to prevent keystream reuse
- Contrast with V1: `counter = blockIndex * 4096` (vulnerable - adjacent blocks share keystream)

#### Expected V3 Output

**Metadata Fields:**
```json
{
  "x-amz-meta-armor-version": "3",
  "x-amz-meta-armor-wrapped-dek": "v2:<fingerprint>:<base64>",
  "x-amz-meta-armor-iv": "<base64>",
  "x-amz-meta-armor-block-size": "65536",
  "x-amz-meta-armor-plaintext-size": "94",
  "x-amz-meta-armor-sha256": "a7f6f6d8cfd8a8f487d72bd31703a575f93885fe6cfea9843f8a4752eb69f12c",
  "x-amz-meta-armor-etag": "<computed-etag>"
}
```

**Object Structure:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1]
- `compression_used`: false
- `sidecar_path`: null
- `manifest_reference`: null

**Envelope Structure (V3):**
```
Offset  | Size  | Field          | Description
--------|-------|----------------|----------------------------------------
0x00    | 4     | Magic          | "ARMR" (unchanged)
0x04    | 1     | Version        | 0x03 (upgraded from 0x02)
0x05    | 1     | BlockSizeLog2  | 16 (unchanged)
0x06    | 16    | IV             | Same as V2 (re-encryption with per-part counter)
0x16    | 8     | PlaintextSize  | 94 (unchanged)
0x1E    | 32    | PlaintextSHA   | Same as V2 (unchanged)
0x3E    | 2     | Reserved       | 0x0000 (compression = none)
0x40    | var   | Ciphertext     | Re-encrypted with V3 per-part counter
0x40+   | 32    | HMAC Table     | 1 × HMAC-SHA256 (32 bytes)
```

**V3 Counter Derivation (PER-PART INDEPENDENT):**
- Formula: `counter = IV[0:8] || uint16(0) || uint32(blockIndex) || uint16(aesBlockIndex)`
- For this fixture: `counter = IV[0:8] || 0x0000 || blockIndex || aesBlockIndex`
- **Security:** Per-part independent counter prevents cross-part keystream overlap

**Transformation Notes:**
1. Version upgraded from 2 to 3
2. DEK wrapping unchanged (already V2 format)
3. Counter derivation changed from fixed V2 striding to per-part independent V3
4. HMAC table added (32 bytes for 1 block)
5. No structural changes (single-PUT → single-PUT)
6. Reserved field set to 0x0000 (no compression)

---

### 2. v2-single-short

**Location:** `tests/fixtures/migration/generated_fixtures/v2-single-short/`

#### Input Structure

**metadata.json:**
```json
{
  "plaintext_sha256": "42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe",
  "plaintext_length": 47,
  "source_version": "v2",
  "source_layout": "single",
  "v3_expected": {
    "is_multipart": false,
    "compression_used": false
  },
  "description": "V2 single-PUT object with fixed counter derivation",
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
  "x-amz-meta-armor-version": "2",
  "x-amz-meta-armor-wrapped-dek": "v2:ae216c2e:hAPftUjTO3CbFY1/0w7vajQ71dU+3pMk0wEoBs5KFF6no2hj9wDVui96PQl8GkoI6ANzapXjE726ObDo"
}
```

**Envelope Header (first 64 bytes of stored_ciphertext.bin):**
```
Offset  | Hex                      | Field          | Value
--------|--------------------------|----------------|----------------------------------------
0x00    | 41 52 4d 52             | Magic          | "ARMR"
0x04    | 02                       | Version        | 0x02 (V2)
0x05    | 10                       | BlockSizeLog2  | 16 (65536 bytes)
0x06    | 03 04 05 06 07 08       | IV[0:10]       | 03 04 05 06 07 08 09 0a 0b 0c
        | 09 0a 0b 0c 0d 0e       | IV[10:16]      | 0d 0e 0f 10 11 12
0x16    | 2f 00 00 00 00 00 00 00 | PlaintextSize  | 47 bytes (0x2F)
0x1E    | 42 17 8e 32 a6 46 da 41 | PlaintextSHA   | 42178e32a646da41221c1a86808f6314fc0fde7bd9e51a091e4ec48a9315bbfe
        | 22 1c 1a 86 80 8f 63 14 |                |
        | fc 0f de 7b d9 e5 1a 09 |                |
        | 1e 4e c4 8a 93 15 bb fe |                |
0x3E    | 00 00                    | Reserved       | Padding to 64 bytes
0x40    | 69 8f a1 aa...           | Ciphertext     | [47 bytes encrypted]
```

**V2 Counter Derivation:**
- Formula: `counter = blockIndex * (blockSize / 16)`
- For this fixture: `counter = blockIndex * 4096`
- **Security:** Fixed striding prevents keystream reuse

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
- `sidecar_path`: null
- `manifest_reference`: null

**Envelope Structure (V3):**
- Same structure as `standard` fixture
- Smaller plaintext size (47 bytes vs 94 bytes)
- HMAC table: 32 bytes for 1 block

**Transformation Notes:**
- Same conversion path as `standard`
- Demonstrates V2→V3 migration works for small objects
- V2 DEK wrapping already in v2 format (no re-wrapping needed)
- Counter derivation upgraded from V2 striding to V3 per-part independent

---

## Conversion Summary Matrix

| Fixture Name | Version Detection | DEK Format | Counter Derivation | Size | Expected Outcome |
|--------------|-------------------|------------|-------------------|------|------------------|
| `standard` | Explicit (`x-amz-meta-armor-version: 2`) | V2 (`v2:<fingerprint>:<base64>`) | V2: `blockIndex * 4096` → V3: Per-part independent | 94 bytes | Success |
| `v2-single-short` | Explicit (`x-amz-meta-armor-version: 2`) | V2 (`v2:<fingerprint>:<base64>`) | V2: `blockIndex * 4096` → V3: Per-part independent | 47 bytes | Success |

---

## Common V3 Conversion Steps

All V2 single-part fixtures follow the same conversion path:

### 1. Version Detection
- **Explicit:** Check `x-amz-meta-armor-version` metadata field (value: `"2"`)
- **Implicit:** Parse envelope header, verify magic bytes (`ARMR`), read version byte (`0x02`)

### 2. Metadata Validation
- Verify `x-amz-meta-armor-etag` matches plaintext SHA256 (entity tag for integrity)
- Validate `x-amz-meta-armor-block-size` is `65536` (BlockSizeLog2 = 16)
- Ensure all required fields present (no reconstruction needed unlike V1)

### 3. DEK Re-wrapping (SKIPPED for V2)
- V2 already uses V2 DEK wrapping format: `v2:<fingerprint>:<base64>`
- No re-wrapping needed (contrast with V1 which needs V1→V2 conversion)

### 4. Counter Space Upgrade
- **V2 (fixed striding):** `counter = blockIndex * (blockSize / 16)`
- **V3 (per-part independent):** `counter = IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock)`

### 5. Re-encryption
- Decrypt ciphertext with V2 counter derivation
- Re-encrypt with V3 per-part counters
- Preserve IV, plaintext length, and SHA256 hash
- **Note:** Ciphertext changes because counter derivation changes

### 6. HMAC Table Generation
- Generate per-block HMAC-SHA256: `HMAC(hmacKey, uint16(0)||uint32(blockIndex)||ciphertext)`
- Append HMAC table after ciphertext
- Single block → 32 bytes HMAC data

### 7. Metadata Update
- Set `x-amz-meta-armor-version` to `"3"`
- Keep `x-amz-meta-armor-wrapped-dek` unchanged (already V2 format)
- Update `x-amz-meta-armor-etag` to new computed ETag
- Keep `x-amz-meta-armor-iv`, `x-amz-meta-armor-block-size`, `x-amz-meta-armor-plaintext-size`, `x-amz-meta-armor-sha256`
- Set Reserved field to `0x0000` (no compression)

---

## V3 Single-PUT Object Layout

After migration, all V2 single-part fixtures become:

```
┌────────────────────────────────────────────────────────────────┐
│                     V3 Single-PUT Object                       │
├────────────────────────────────────────────────────────────────┤
│ Metadata (S3 Object Metadata):                                   │
│   x-amz-meta-armor-version: "3"                               │
│   x-amz-meta-armor-wrapped-dek: "v2:<fingerprint>:<base64>"     │
│   x-amz-meta-armor-iv: "<base64>"                              │
│   x-amz-meta-armor-block-size: "65536"                          │
│   x-amz-meta-armor-plaintext-size: "<original>"                │
│   x-amz-meta-armor-sha256: "<original>"                        │
│   x-amz-meta-armor-etag: "<computed>"                          │
├────────────────────────────────────────────────────────────────┤
│ Ciphertext Blob (stored_ciphertext.bin):                         │
│   [Envelope Header 64 bytes]                                   │
│     - Magic: "ARMR"                                            │
│     - Version: 0x03                                            │
│     - BlockSizeLog2: 16                                        │
│     - IV: [16 bytes]                                           │
│     - PlaintextSize: [8 bytes]                                 │
│     - PlaintextSHA: [32 bytes]                                 │
│     - Reserved: [2 bytes]                                      │
│   [Encrypted Data]                                               │
│   [HMAC Table 32 bytes]                                         │
└────────────────────────────────────────────────────────────────┘
```

**Key properties:**
- `is_multipart`: false
- `part_count`: 1
- `blocks_per_part`: [1] (single block)
- `compression_used`: false
- `sidecar_path`: null (no sidecar in single-part)
- `manifest_reference`: null (no multipart manifest)
- `hmac_entries`: 1 (32 bytes total)

---

## V1 vs V2 Key Differences

The V2 format introduced critical security improvements over V1:

### Security Vulnerability Fix

**V1 (VULNERABLE):**
```go
counter = blockIndex * 4096
```
- Adjacent blocks use adjacent counters (0, 4096, 8192, ...)
- **Vulnerability:** First AES block of adjacent data blocks shares keystream
- Enables keystream reuse attacks on data with predictable structure

**V2 (FIXED):**
```go
counter = blockIndex * (blockSize / 16)
```
- Strides counter by block size / 16 (4096 for 64KB blocks)
- **Fix:** Prevents keystream reuse across blocks
- Each data block uses distinct counter space

**V3 (ENHANCED):**
```go
counter = IV[0:8] || uint16(part) || uint32(block) || uint16(aesBlock)
```
- Per-part independent counter construction
- Adds HMAC integrity verification
- Multipart-safe counter isolation

### Metadata Differences

**V1 → V2 Changes:**
1. Added `x-amz-meta-armor-etag` field for entity tag validation
2. DEK wrapping changed from V1 (simple base64) to V2 (`v2:<fingerprint>:<base64>`)
3. Counter derivation changed from vulnerable to secure
4. Reserved field semantics (V1: must be zero; V2: reserved for future use)

**V2 → V3 Changes:**
1. Version field from `"2"` to `"3"`
2. Counter derivation from fixed striding to per-part independent
3. Added HMAC table for per-block integrity
4. Reserved field used for compression flags

---

## Testing Coverage

These fixtures validate the migration path covers:

✅ **Version detection** (explicit V2 version field)  
✅ **DEK format validation** (V2 format, no re-wrapping needed)  
✅ **Counter derivation upgrade** (V2 striding → V3 per-part independent)  
✅ **HMAC generation** (per-block integrity verification)  
✅ **Object sizes** (small: 47 bytes, medium: 94 bytes)  
✅ **Metadata preservation** (SHA256, IV, block size, plaintext size)  
✅ **Backward compatibility** (V2 envelope parsing)  
✅ **Security improvements** (V1 vulnerable → V2 fixed → V3 enhanced)

---

## Related Documentation

- **V1 Single-Part Fixtures:** See `v1-single-part-fixtures.md`
- **V1 Multipart Fixtures:** See `v1-multipart-fixtures.md`
- **V2 Multipart Fixtures:** See `v2-multipart-fixtures.md`
- **Migration Implementation:** See `cmd/armor/cmd_migrate.go`
- **Envelope Format:** See `internal/crypto/envelope.go`

---

**Document End**
