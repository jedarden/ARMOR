# ARMOR Envelope Format v3

**Status:** Normative (Plan §8.11)

Envelope format v3 adds self-describing multipart parts, per-block compression, and removes the fragile cumulative offset dependency that required uniform part sizes and ordered uploads in v2.

## Version Identification

- **Header version byte:** `0x03` for single-PUT objects
- **Metadata header:** `x-amz-meta-armor-version: 3` on every v3 object (both single-PUT and multipart)
- **Sidecar version field:** `"version": 3` in `.armor/hmac/<sha256-of-key>` sidecar JSON

## Key Materials

Per-object keys, unchanged from v1/v2:

- **DEK:** 32-byte random data encryption key (generated per object)
- **IV:** 16-byte random initialization vector (generated per object)
- **HMAC key:** Derived via HKDF-SHA256: `hmacKey = HKDF-SHA256(DEK, info="armor-hmac-v1")`

The wrapped DEK and IV are stored in object metadata (`x-amz-meta-armor-wrapped-dek`) and the sidecar.

## Header Layout

64-byte binary header (magic and version are the only changes from v2):

```
Offset  Size  Field          Description
------  ----  -----          -----------
0       4     Magic          "ARMR" (0x41 52 4D 52)
4       1     Version        0x03 for v3
5       1     BlockSizeLog2  log2(block_size), e.g., 16 for 64 KiB, 18 for 256 KiB, 20 for 1 MiB
6       16    IV             Initialization vector for CTR mode
22      8     PlaintextSize  Total plaintext size in bytes (before encryption/compression)
30      32    PlaintextSHA   SHA-256 hash of plaintext (pre-compression)
62      2     Reserved       Reserved[0] = per-block compression flag, Reserved[1] = 0 (must be zero)
```

### Reserved Field Semantics

- **Reserved[0]:** Per-block compression enable flag for this object
  - `0x00`: No compression (all blocks stored raw)
  - `0x01`: Per-block zstd compression enabled
  - `0x02-0xFF`: Reserved for future compression types

- **Reserved[1]:** Must be zero (reserved for future use)

The compression flag is an object-level setting determined by:
1. Request metadata `x-amz-meta-armor-compress: true|false` (overrides all else)
2. `ARMOR_COMPRESS_RULES` environment variable (comma list of `suffix|content-type=zstd|none`)
3. Legacy `ARMOR_COMPRESS=true` (alias for `*=zstd`)

**Constraint:** `blockSize ≤ 1 MiB` (enforced by `BlockSizeLog2 ≤ 20`). This ensures the per-block AES counter index fits in 16 bits (`j < 2^16`).

## Counter Block Construction

AES-CTR mode requires a unique counter per 16-byte AES block. The counter block for AES block `j` (0-indexed) within ARMOR block `b` of part `p` is:

```
counter_block = IV[0:8] ‖ uint16(p) ‖ uint32(b) ‖ uint16(j)
```

All integers are **big-endian** (network byte order).

- **IV[0:8]:** First 8 bytes of the IV (fixed per object)
- **uint16(p):** Part number (0 for single-PUT, 1..10000 for multipart parts per S3/B2 limits)
- **uint32(b):** ARMOR block index within the part (0-based)
- **uint16(j):** AES block index within the ARMOR block (0-based, max `blockSize/16 - 1`)

### Counter Uniqueness

Distinct `(p, b)` tuples never share keystream by construction. The per-part `p` field means two different parts can use the same block index `b` without counter collision. This removes the v2 requirement that parts be uploaded in order with uniform sizes.

**Single-PUT shortcut:** `p = 0` for single-PUT objects (no part number).

### Counter Diagram

```
IV bytes 0-7 (8 bytes)    Part p (2 bytes)    Block b (4 bytes)    AES j (2 bytes)
┌──────────────┬────────────────┬─────────────────────┬────────────────┐
│ IV prefix    │ part number    │ block number       │ AES block      │
│ (fixed)      │ (0 or 1-10000) │ (0..N)             │ (0..65535)     │
└──────────────┴────────────────┴─────────────────────┴────────────────┘
       8 bytes            2 bytes           4 bytes           2 bytes
                              = 16 bytes (one AES block)
```

## Per-Block HMAC

Each ARMOR block (after compression and encryption) is authenticated with HMAC-SHA256:

```
hmac_input = uint16(p) ‖ uint32(b) ‖ ciphertext_block
hmac = HMAC-SHA256(hmacKey, hmac_input)
```

All integers are **big-endian**.

- **uint16(p):** Part number (0 for single-PUT, 1..10000 for multipart)
- **uint32(b):** Block index within the part (0-based)
- **ciphertext_block:** The encrypted (and possibly compressed) block bytes

The HMAC covers the part number and block index, binding the authentication to the specific block location and preventing block reordering attacks.

## Block Table Entry

Each block's table entry is fixed-width (36 bytes):

```
Offset  Size  Field    Description
------  ----  -----    -----------
0       32    HMAC     HMAC-SHA256 of the block (32 bytes)
32      4     CLen     Ciphertext length with compression flag (big-endian)
```

### CLen Field Format

The `CLen` field is a big-endian 32-bit integer with the high bit as the compression flag:

```
Bit 31 (MSB):   1 = zstd-compressed, 0 = raw
Bits 0-30 (LSB): Ciphertext length in bytes (≤ blockSize when raw)
```

**Examples:**

- Raw block of 65536 bytes: `0x0000FFFF` (compression bit clear)
- Compressed to 32768 bytes: `0x80008000` (compression bit set)
- Compressed to 12345 bytes: `0x80003039` (compression bit set, length 0x3039 = 12345)

### Reading CLen

```go
func ParseCLen(clen uint32) (length int, compressed bool) {
    compressed = (clen & 0x80000000) != 0
    length = int(clen & 0x7FFFFFFF)  // Mask off compression bit
    return
}

func EncodeCLen(length int, compressed bool) uint32 {
    if compressed {
        return uint32(length) | 0x80000000
    }
    return uint32(length)
}
```

## Per-Block Compression

Each plaintext block is independently compressed using zstd:

1. Compress the plaintext block with zstd (default level)
2. If compressed size < original size, store compressed and set the compression flag
3. Otherwise, store raw with compression flag clear

Compression is evaluated **per-block**, not per-object. A block with incompressible data (e.g., already compressed, encrypted, or random) is stored raw with no penalty.

### Compression Rules

The `ARMOR_COMPRESS_RULES` environment variable controls compression by file pattern:

```
ARMOR_COMPRESS_RULES=.jsonl=zstd,.wal=zstd,application/json=zstd,*=none
```

Patterns are evaluated in order; first match wins. Patterns are:

- Suffix match: `.jsonl`, `.wal`, `.parquet`, etc.
- Content-Type match: `application/json`, `text/csv`, etc.
- Wildcard `*` matches everything (use as final catch-all)

Values:
- `zstd`: Enable per-block compression
- `none`: Disable compression (store raw)

### Multipart Compression

**Multipart parts are NEVER compressed.** B2's 5 MiB minimum part size applies to the *ciphertext* part, and compression makes the final size unpredictable. Set `x-amz-meta-armor-compress: false` for all multipart uploads.

### Range Reads on Compressed Objects

Range reads work transparently on compressed objects. The read path:
1. Fetches the block table (one ranged GET at the end)
2. Maps plaintext offset to part and block by cumulative plaintext lengths
3. Computes ciphertext offset via prefix sums of `clen` (masking off compression bit)
4. Decrypts (and decompresses if flag set) only the requested blocks

## Single-PUT Layout

Single-PUT objects store everything contiguously:

```
┌─────────────┬──────────────────┬─────────────────────┬──────────────────┐
│ Header      │ Block 0          │ Block N             │ Block Table      │
│ (64 bytes)  │ (variable)       │ (variable)          │ (36 × N bytes)   │
└─────────────┴──────────────────┴─────────────────────┴──────────────────┘
```

- **Header:** Fixed 64 bytes
- **Blocks:** Concatenated ciphertext blocks (each `clen & 0x7FFFFFFF` bytes)
- **Block table:** `blockCount` entries of 36 bytes each (HMAC + CLen)

### Table Offset Calculation

The table offset is NOT explicitly stored. It is computed:

```
blockCount = ceil(plaintextSize / blockSize)
tableOffset = ciphertext_length - (36 * blockCount)
```

Where `ciphertext_length` comes from:
- `HeadObject` response for single-PUT reads
- Sidecar `parts[*].ciphertext_len` for multipart

This avoids storing redundant length information and allows validating the object size (if `ciphertext_length` mismatches the computed total, the object is corrupted).

### Read Path (Single-PUT)

1. `HeadObject` to get `ciphertext_length`
2. Fetch block table: `Range: bytes={tableOffset}-`
3. For each requested block:
   - Parse HMAC and CLen from table entry
   - Compute block offset by summing previous CLens (masked)
   - `Range` GET the block bytes
   - Verify HMAC
   - Decrypt with AES-CTR using the constructed counter
   - Decompress if compression flag set
4. Return plaintext

## Multipart Layout

Multipart uploads store state **per-part** instead of a single monolithic `.state` file:

### Per-Part State Objects

Each part gets its own JSON object:

```
.armor/multipart/<upload-id>/part-<n>.json
```

Where `<n>` is the S3/B2 part number (1..10000).

**Schema:**

```json
{
  "n": 1,
  "plaintext_len": 10485760,
  "ciphertext_len": 10485836,
  "blocks": [
    ["base64hmac", 65536],
    ["base64hmac", 65536],
    ...
  ]
}
```

- **n:** Part number (matches filename)
- **plaintext_len:** Total plaintext size for this part (sum of all block plaintext sizes)
- **ciphertext_len:** Total ciphertext size for this part (sum of all `clen` values)
- **blocks:** Array of `[hmac_b64, clen]` pairs, one per block
  - `hmac_b64`: Base64-encoded HMAC-SHA256 (32 bytes decoded)
  - `clen`: Ciphertext length with compression flag (integer, same encoding as single-PUT)

### Upload Metadata Object

The shared upload metadata:

```
.armor/multipart/<upload-id>/meta.json
```

**Schema:**

```json
{
  "iv": "base64iv",
  "wrapped_dek": "v2:<fp16>:<base64>",
  "key_id": "bucket-key",
  "content_type": "application/octet-stream",
  "compress": false
}
```

- **iv:** Base64-encoded 16-byte IV
- **wrapped_dek:** Fingerprint-prefixed wrapped DEK (v2 format)
- **key_id:** Named key used for wrapping
- **content_type:** Original Content-Type from upload init
- **compress:** Compression setting (always `false` for multipart)

### Multipart Write Workflow

1. `CreateMultipartUpload`:
   - Generate DEK and IV
   - Wrap DEK with the named key
   - Write `.armor/multipart/<upload-id>/meta.json`

2. `UploadPart` (for each part):
   - Encrypt blocks with AES-CTR (using part number in counter)
   - Compute per-block HMAC
   - Optionally compress blocks (only if `compress: true` and part size allows)
   - Write part ciphertext to B2
   - Write `.armor/multipart/<upload-id>/part-<n>.json` with block table

3. `CompleteMultipartUpload`:
   - Read all per-part JSON objects
   - Construct sidecar (see below)
   - Write sidecar to `.armor/hmac/<sha256-of-key>`
	   - B2 Complete → concatenate parts into ciphertext object (NO metadata)
	   - **Write manifest object** to `<key>.armor-manifest` with all ARMOR metadata (version, wrapped DEK, etc.)
   - Delete all `.armor/multipart/<upload-id>/part-*.json` and `meta.json`

### Multipart Read Workflow

1. `HeadObject` to get metadata (includes `x-amz-meta-armor-version: 3`)
2. Fetch sidecar from `.armor/hmac/<sha256-of-key>` (key from wrapped DEK fingerprint)
3. For each requested byte range:
   - Locate part by cumulative `plaintext_len`
   - Locate block within part by `(offset - part_start) / blockSize`
   - Compute ciphertext offset from part start + block prefix sums
   - `Range` GET the block
   - Verify HMAC and decrypt (same as single-PUT)

### Part Independence

Because each part uses its own `p` value in the counter construction:
- Parts can be uploaded in any order
- Parts can have different sizes
- Multiple replicas can handle parts of the same upload concurrently without coordination
- No `SlowDown` retries from cumulative offset calculation errors

## Sidecar Format

The sidecar at `.armor/hmac/<sha256-of-key>` stores all block authentication data:

**Filename:** `SHA-256(dearmor-key)` where `dearmor-key` is constructed from the wrapped DEK fingerprint and object key.

**Format:** gzip-compressed JSON

**Schema:**

```json
{
  "version": 3,
  "block_size": 65536,
  "parts": [
    {
      "n": 0,
      "plaintext_len": 1048576,
      "ciphertext_len": 1048612,
      "blocks": [
        ["sgV+hG0+v2FQMRYxL5VxmNKHG8A/j8CaCvFHKQbyoj0=", 65536],
        ["EdXZYL8q5s2oGN6RwMWRuHLMxlPk0hJe5BJnRhPcgDo=", 65536],
        ...
      ]
    },
    {
      "n": 1,
      "plaintext_len": 5242880,
      "ciphertext_len": 5242916,
      "blocks": [
        ["Kp+xmNKHG8A/j8CaCvFHKQbyoj0sgV+hG0+v2FQMRYxL=", 65536],
        ...
      ]
    }
  ]
}
```

- **version:** Format version (integer 3)
- **block_size:** ARMOR block size (same for all blocks, from header)
- **parts:** Array of part descriptors
  - **n:** Part number (0 for single-PUT, 1..10000 for multipart)
  - **plaintext_len:** Cumulative plaintext length for this part
  - **ciphertext_len:** Cumulative ciphertext length for this part
  - **blocks:** Array of `[hmac_b64, clen]` tuples
    - `hmac_b64`: Base64-encoded HMAC-SHA256 (32 bytes decoded → 44 chars base64)
    - `clen`: Integer with compression flag in high bit

### Sidecar Generation

The sidecar is written at `CompleteMultipartUpload` time:

1. Read all `.armor/multipart/<upload-id>/part-*.json` objects
2. Aggregate into the JSON structure above
3. Gzip-compress the JSON
4. Write to `.armor/hmac/<sha256-of-key>` (object key is SHA-256 hash of some derivation of the wrapped DEK and object key)

### Sidecar Size

For a 100 GiB object with 1 MiB block size:
- Block count: ~102,400 blocks
- Sidecar (uncompressed): ~102,400 entries × (44 + 8 + overhead) ≈ 7 MB
- Sidecar (gzip-compressed): ~3-4 MB

The sidecar must be fetched before the first byte can be read, but is small enough to cache in memory.

## Version Discrimination

Read path dispatches on the metadata header:

- **Missing `x-amz-meta-armor-version`:** Assume v1 (legacy)
- **`x-amz-meta-armor-version: 1` or `2`:** Use v1/v2 read path
- **`x-amz-meta-armor-version: 3`:** Use v3 read path
  - Single-PUT: Header indicates `Version == 0x03`, read block table from end
  - Multipart: Fetch sidecar, use per-part block tables

## Backward Compatibility

v1 and v2 readers are supported indefinitely. The v3 implementation must:

1. Preserve v1/v2 read paths unchanged
2. Detect version from header byte (single-PUT) or metadata (multipart)
3. Dispatch to appropriate reader

v1 and v2 objects are never automatically migrated to v3. Migration is opt-in via a separate format-migration endpoint.

## Security Properties

### Counter Uniqueness

The per-part counter construction guarantees that no two AES blocks share the same counter keystream:

- Each `(p, b, j)` triple is unique
- Single-PUT uses `p = 0`
- Multipart uses S3/B2 part numbers (1..10000)
- Part numbers cannot collide across different uploads

### HMAC Binding

Per-block HMAC covers:
- Part number (prevents cross-part block swapping)
- Block index (prevents reordering within a part)
- Ciphertext (authenticates the encrypted data)

### Compression Side-Channel

Per-block compression could expose plaintext patterns via size side-channels (e.g., compressible vs. incompressible blocks). This is acceptable because:

- Compression is optional and user-controlled
- The side-channel is limited to block-level size information
- The ciphertext is still authenticated (HMAC covers compressed blocks)
- Plaintext is already encrypted before compression

## Test Vectors

See `internal/crypto/testdata/v3/` for JSON test vectors covering:

1. **1-block-single-put.json:** Minimal single-PUT object (1 block, uncompressed)
2. **3-block-compressed.json:** Single-PUT object (3 blocks, middle block compressible)
3. **2-part-multipart.json:** Multipart object (2 parts, different sizes)

Each test vector includes:
- Input plaintext
- DEK and IV (fixed for reproducibility)
- Expected header bytes
- Expected block table entries
- Expected ciphertext blocks
- Complete sidecar JSON

Run the generator with:
```bash
go test ./internal/crypto -run TestGenerateV3Vectors -update
```

## Migration from v2

Migrating v2 objects to v3 is **optional** and done via a dedicated endpoint:

```
POST /admin/format/migrate?include=v2&target-version=3
```

Migration workflow:
1. List objects with `x-amz-meta-armor-version: 2`
2. For each object:
   - Download v2 ciphertext
   - Decrypt with v2 reader
   - Encrypt with v3 writer (single-PUT or multipart as appropriate)
   - Upload to new object key
   - Verify with v3 reader
   - Delete original v2 object

The migration is **not** a precondition for enabling v3 as the default write format. v2 objects can coexist with v3 objects indefinitely.
