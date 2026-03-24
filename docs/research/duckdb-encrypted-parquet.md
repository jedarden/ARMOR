# DuckDB + Encrypted Parquet on B2

## Problem Statement

Data stored on B2 is client-side encrypted. DuckDB's key performance optimizations for Parquet — column pruning, predicate pushdown, and range reads — require reading file structure metadata and fetching arbitrary byte ranges. Standard whole-file encryption makes the entire file opaque, forcing a full download + decrypt before DuckDB can touch it.

**Goal:** Encrypt Parquet files such that DuckDB can still perform selective reads, decrypting only the bytes it actually needs, while maintaining zero-knowledge security (neither B2 nor Cloudflare can read the data).

---

## How DuckDB Reads Parquet

Understanding what DuckDB needs helps design the encryption scheme:

```
┌─────────────────────────────────────┐
│          Parquet File Layout        │
├─────────────────────────────────────┤
│  Magic bytes (PAR1)                 │  ← 4 bytes
├─────────────────────────────────────┤
│  Row Group 0                        │
│    ├── Column Chunk: col_a (pages)  │  ← DuckDB reads only needed columns
│    ├── Column Chunk: col_b (pages)  │
│    └── Column Chunk: col_c (pages)  │
├─────────────────────────────────────┤
│  Row Group 1                        │
│    ├── Column Chunk: col_a (pages)  │
│    ├── Column Chunk: col_b (pages)  │
│    └── Column Chunk: col_c (pages)  │
├─────────────────────────────────────┤
│  Footer (Thrift-encoded metadata)   │  ← Schema, row group offsets,
│    ├── Schema                       │     column chunk locations,
│    ├── Row group metadata           │     min/max statistics
│    ├── Column chunk offsets         │
│    └── Min/max statistics           │
├─────────────────────────────────────┤
│  Footer length (4 bytes)            │
│  Magic bytes (PAR1)                 │  ← 4 bytes
└─────────────────────────────────────┘
```

**DuckDB's read sequence:**
1. Read last 8 bytes → footer length + magic
2. Read footer → schema, row group metadata, column offsets, statistics
3. Use statistics to **skip** row groups that don't match predicates (pushdown)
4. Issue **range reads** for only the column chunks in surviving row groups (pruning)
5. Decompress + decode only the fetched pages

A query like `SELECT col_a FROM data WHERE col_b > 100` might read <5% of the file.

---

## Approach 1: AES-CTR Block-Level Encryption (Recommended)

### How AES-CTR Enables Random Access

AES-CTR (Counter Mode) turns AES into a stream cipher where any byte offset can be decrypted independently:

```
Key + IV + Counter=0  → AES → Keystream Block 0  ⊕  Plaintext Block 0  = Ciphertext Block 0
Key + IV + Counter=1  → AES → Keystream Block 1  ⊕  Plaintext Block 1  = Ciphertext Block 1
Key + IV + Counter=N  → AES → Keystream Block N  ⊕  Plaintext Block N  = Ciphertext Block N
```

To decrypt block N, you only need the key, IV, and the value N — no dependency on prior blocks. This is exactly how LUKS/dm-crypt encrypts disks while allowing random I/O.

### File Format

```
┌─────────────────────────────────────────────────┐
│  b2-layer header (plaintext)                    │
│    ├── Magic: "B2L\x01"                         │
│    ├── Version: 1                               │
│    ├── Block size: 4096 (or 65536)              │
│    ├── IV/Nonce: 16 bytes                       │
│    ├── Wrapped DEK: variable                    │
│    ├── Original file size: 8 bytes              │
│    ├── Per-block HMAC size: 32 bytes            │
│    └── HMAC table offset: 8 bytes               │
├─────────────────────────────────────────────────┤
│  Encrypted Parquet data                         │
│    ├── Block 0: AES-CTR(ctr=0, plaintext[0:BS]) │
│    ├── Block 1: AES-CTR(ctr=1, plaintext[BS:2BS])│
│    ├── ...                                      │
│    └── Block N: AES-CTR(ctr=N, plaintext[N*BS:])│
├─────────────────────────────────────────────────┤
│  HMAC table (integrity)                         │
│    ├── HMAC(block 0): 32 bytes                  │
│    ├── HMAC(block 1): 32 bytes                  │
│    ├── ...                                      │
│    └── HMAC(block N): 32 bytes                  │
└─────────────────────────────────────────────────┘
```

### Read Path (Transparent to DuckDB)

```python
BLOCK_SIZE = 4096  # or 65536 for fewer blocks on large files

class B2LayerFS:
    """Custom filesystem that decrypts on the fly."""

    def read(self, path, offset, length):
        # 1. Translate plaintext offset to encrypted block range
        block_start = offset // BLOCK_SIZE
        block_end = (offset + length - 1) // BLOCK_SIZE + 1

        # 2. Calculate byte range in encrypted file (accounting for header)
        enc_offset = HEADER_SIZE + (block_start * BLOCK_SIZE)
        enc_length = (block_end - block_start) * BLOCK_SIZE

        # 3. HTTP Range request to Cloudflare (→ B2 via PNI, free egress)
        encrypted_blocks = http_range_get(
            f"https://files.example.com/{path}",
            offset=enc_offset,
            length=enc_length
        )

        # 4. Verify per-block HMACs
        hmac_entries = self.fetch_hmacs(block_start, block_end)
        for i, block in enumerate(chunk(encrypted_blocks, BLOCK_SIZE)):
            assert verify_hmac(block, hmac_entries[i])

        # 5. AES-CTR decrypt with counter derived from block offset
        counter = int.from_bytes(iv, 'big') + block_start
        plaintext = aes_ctr_decrypt(encrypted_blocks, dek, nonce=counter)

        # 6. Slice to exact requested range
        trim_start = offset % BLOCK_SIZE
        return plaintext[trim_start : trim_start + length]
```

### DuckDB Integration Options

#### Option A: FUSE Mount
```bash
# Mount encrypted B2 bucket as a local directory
b2-layer mount /mnt/b2-decrypted --bucket my-bucket --domain files.example.com

# DuckDB queries it like local files
duckdb -c "SELECT * FROM read_parquet('/mnt/b2-decrypted/data/*.parquet') WHERE date > '2026-01-01'"
```

Uses `libfuse` / `fusepy`. The FUSE layer intercepts `read(offset, length)` calls and runs the decrypt logic above. DuckDB is completely unaware of the encryption.

#### Option B: DuckDB Custom Filesystem Extension
```python
import duckdb

# Register the encrypted filesystem
duckdb.register_filesystem(B2LayerFS(config))

# Query using a custom protocol prefix
duckdb.sql("SELECT * FROM read_parquet('b2l://my-bucket/data/*.parquet')")
```

DuckDB supports registering custom `FileSystem` implementations via its Python API (using `fsspec`-compatible interfaces). This avoids the FUSE overhead and kernel roundtrips.

#### Option C: httpfs with Decrypt Proxy
```sql
-- Point DuckDB's httpfs at a local decrypt proxy
SET s3_endpoint = 'localhost:8443';
SET s3_access_key_id = 'local-key';
SET s3_secret_access_key = 'local-secret';

SELECT * FROM read_parquet('s3://my-bucket/data/*.parquet');
```

Run a local HTTP server that speaks the S3 protocol, translates range requests into encrypted range fetches + decryption, and returns plaintext. DuckDB's built-in `httpfs` extension handles the S3 protocol; the proxy handles the crypto.

### Block Size Tradeoffs

| Block Size | Blocks per 1GB | HMAC Table Size | Range Read Overhead | Best For |
|-----------|----------------|-----------------|--------------------|----|
| 4 KB | 262,144 | 8 MB | Minimal (1 block = 4KB) | Small, heavily queried files |
| 16 KB | 65,536 | 2 MB | Low | General purpose |
| 64 KB | 16,384 | 512 KB | Moderate | Large files, sequential scans |
| 1 MB | 1,024 | 32 KB | High (1MB minimum read) | Archive/backup |

**Recommendation:** 16 KB or 64 KB. Matches typical Parquet page sizes and balances overhead vs granularity.

### Security Properties

| Property | Status |
|----------|--------|
| Confidentiality | AES-256-CTR with random per-file IV |
| Integrity | Per-block HMAC-SHA256 |
| Key management | Envelope encryption (DEK per file, wrapped by MEK) |
| Zero-knowledge | B2 and Cloudflare see only ciphertext |
| Random access | Full — any byte range independently decryptable |
| Tampering detection | Per-block — corrupted/modified blocks detected individually |

**Limitation:** AES-CTR is malleable — an attacker who can modify ciphertext can predictably flip plaintext bits. The per-block HMACs mitigate this (detect tampering before decryption), but the HMAC must be verified before using decrypted data.

---

## Approach 2: Parquet Modular Encryption (PME)

### Overview

PME is part of the Apache Parquet specification. It encrypts at the Parquet structural level rather than the byte level:

- Each **column** can be encrypted with a different key
- The **footer** can remain plaintext (exposing schema + stats) or be encrypted
- Encryption is AES-GCM (authenticated) per column chunk
- Key metadata is stored in the Parquet file itself

### Write Path (PyArrow)

```python
import pyarrow.parquet as pq
import pyarrow.parquet.encryption as pe

# Configure per-column encryption
encryption_config = pe.EncryptionConfiguration(
    footer_key="master-key-id",
    column_keys={
        "key-for-sensitive": ["col_secret", "col_pii"],
        "key-for-analytics": ["col_metric", "col_timestamp"],
    },
    plaintext_footer=True,  # Leave footer readable
)

kms_config = pe.KmsConnectionConfig(
    kms_instance_url="local://",  # or custom KMS
    key_access_token="...",
)

crypto_factory = pe.CryptoFactory(CustomKmsClient)
pq.write_table(
    table,
    "output.parquet",
    encryption_properties=crypto_factory.file_encryption_properties(
        kms_config, encryption_config
    ),
)
```

### The DuckDB Problem

**DuckDB does not support PME.** Its Parquet reader will:
1. Successfully read the plaintext footer (schema, offsets, stats)
2. Fail when it tries to decompress encrypted column chunks

This means PME requires a middleware layer:

```
DuckDB query → identify needed columns/row groups (from plaintext footer)
            → b2-layer fetches encrypted column chunks via range requests
            → decrypts with PyArrow PME reader
            → reconstructs partial plaintext Parquet in memory
            → feeds to DuckDB
```

### PME vs AES-CTR Comparison

| Dimension | AES-CTR Block | Parquet Modular Encryption |
|-----------|--------------|---------------------------|
| Granularity | Byte-level (any offset) | Column-chunk level |
| DuckDB transparent | Yes (FUSE/custom FS) | No (middleware required) |
| Format standard | Custom b2-layer format | Apache Parquet spec |
| Write tooling | Custom (b2-layer) | PyArrow (well-supported) |
| Read tooling | Custom (b2-layer) | PyArrow, parquet-mr (Java) |
| Per-column keys | No (single DEK per file) | Yes |
| Footer visibility | Encrypted (opaque) | Configurable (plaintext or encrypted) |
| Interoperability | Only b2-layer can read | Any PME-compatible reader |
| Min fetch size | 1 block (4-64KB) | 1 column chunk (variable, often MB) |

---

## Approach 3: Decrypt Proxy with Local Cache (Simplest)

If the dataset fits on disk and query latency isn't critical:

```
b2-layer sync → decrypt to local cache → DuckDB reads local Parquet
```

### Implementation

```python
# Selective sync based on partition structure
# Only sync partitions that match a predicate
b2_layer.sync(
    remote="data/",
    local="~/.b2-layer/cache/data/",
    filter="date >= 2026-01-01",  # Only sync recent partitions
)

# DuckDB queries local cache
duckdb.sql("""
    SELECT * FROM read_parquet('~/.b2-layer/cache/data/**/*.parquet')
    WHERE date > '2026-03-01'
""")
```

### Cache Management

```
~/.b2-layer/cache/
├── data/
│   ├── date=2026-03-01/
│   │   └── part-0000.parquet    (decrypted)
│   └── date=2026-03-02/
│       └── part-0000.parquet    (decrypted)
└── .cache-manifest.json         (maps remote→local, tracks freshness)
```

- **Incremental sync:** Compare B2 ETags/lastModified against manifest
- **Partition-aware:** Only sync partitions matching a predicate
- **Eviction:** LRU or TTL-based, configurable max cache size
- **Plaintext at rest:** Acceptable on a dedicated server; use dm-crypt/LUKS on the cache volume for defense in depth

---

## Recommendation

### For b2-layer v1: Local Cache (Approach 3)
- Simplest to implement
- DuckDB works out of the box with zero integration code
- Partition-aware sync minimizes download volume
- Acceptable for a dedicated server where disk space is available

### For b2-layer v2: AES-CTR + Custom Filesystem (Approach 1)
- Full random-access decryption
- DuckDB column pruning and predicate pushdown work transparently
- Only the bytes DuckDB needs are fetched and decrypted
- Zero local cache required — true streaming decryption
- Cloudflare supports HTTP Range requests, so per-block fetches cost $0 egress

### PME: Keep as an option but don't build around it
- Lack of DuckDB support is a hard blocker for transparent integration
- If DuckDB adds PME support in the future, it becomes the cleanest solution
- PyArrow PME is worth using on the write path regardless — it's a standard format

---

## End-to-End Data Flow (v2 Target)

```
WRITE PATH:
  DataFrame
    → PyArrow write Parquet to buffer
    → b2-layer AES-CTR encrypt (16KB blocks, per-file DEK, HMAC table)
    → Direct upload to B2 (ingress free)

QUERY PATH:
  DuckDB: SELECT col_a FROM read_parquet('b2l://bucket/data/*.parquet') WHERE col_b > 100

  1. b2-layer FS: Range read last 8 bytes → footer length
  2. b2-layer FS: Range read footer → decrypt → return schema, stats, offsets to DuckDB
  3. DuckDB: predicate pushdown eliminates row groups 0, 2, 5 (stats say col_b ≤ 100)
  4. DuckDB: column pruning → only need col_a chunks from row groups 1, 3, 4
  5. b2-layer FS: 3 range requests to Cloudflare → decrypt 3 column chunks
  6. DuckDB: decompress + decode → result set

  Data transferred: ~3 column chunks (KB–MB) instead of full file (MB–GB)
  Egress cost: $0 (Cloudflare PNI)
  Decryption scope: only the blocks DuckDB requested
```
