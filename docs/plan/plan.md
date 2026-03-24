# ARMOR Implementation Plan

> **Status: Implementation Complete** (as of 2026-03-24)
>
> All planned features from Phases 1-3 are implemented. The only remaining item is the optional web dashboard.

## Overview

ARMOR is an S3-compatible proxy server that transparently encrypts and decrypts data between clients and Backblaze B2. Clients interact with ARMOR using standard S3 SDKs and tools (boto3, DuckDB `httpfs`, AWS CLI, etc.). ARMOR handles all cryptography invisibly — clients never see ciphertext.

```
                        ARMOR Server
                    ┌───────────────────┐
                    │                   │
   S3 Protocol      │   ┌───────────┐   │     S3 Protocol
  (plaintext)       │   │ Encryption │   │    (ciphertext)
 ──────────────────▶│   │   Layer    │───│──────────────────▶  Backblaze B2
 Client / DuckDB    │   └───────────┘   │                     (storage)
 ◀──────────────────│   ┌───────────┐   │◀──────────────────
  S3 Protocol       │   │ Decryption │   │    via Cloudflare
  (plaintext)       │   │   Layer    │   │    (free egress)
                    │   └───────────┘   │
                    │                   │
                    └───────────────────┘
```

**What makes this different from encrypting on the client:** The encryption boundary is inside the server, not on each client machine. Any S3-compatible tool works unmodified — DuckDB, pandas, rclone, AWS CLI, custom scripts — they all point at `localhost:9000` (or wherever ARMOR listens) and get transparent encryption with zero-egress downloads through Cloudflare.

### Statelessness Principle

ARMOR is **stateless by design.** Any ARMOR instance with the same configuration (MEK + B2 credentials + Cloudflare domain) can read, write, and manage the same data. There is no local state that is required for correctness:

- **All authoritative state lives in B2.** Encryption metadata (IV, wrapped DEK, plaintext size) is stored in B2 object headers and the envelope prepended to each object. Operational metadata (key rotation progress, provenance chain) is stored as objects under a `.armor/` prefix in B2.
- **In-memory caches are optional performance optimizations**, not state. Losing them (restart, failover) means slower first requests, not data loss or inconsistency.
- **Multiple ARMOR instances** can run concurrently against the same bucket. Reads are safe to parallelize. Writes are safe as long as clients don't write the same key concurrently (same constraint as raw S3).

This means ARMOR can be deployed as a sidecar, a standalone pod, or a Docker Compose service — and can be replaced, restarted, or scaled horizontally without migration or state transfer.

---

## Architecture

### Data Flow

#### Upload (PutObject)

```
Client                    ARMOR                         B2
  │                         │                            │
  ├─ PUT /bucket/key ──────▶│                            │
  │   Body: plaintext       │                            │
  │                         ├─ Generate random DEK       │
  │                         ├─ AES-CTR encrypt blocks    │
  │                         ├─ Compute per-block HMACs   │
  │                         ├─ Wrap DEK with MEK         │
  │                         ├─ Build ARMOR envelope      │
  │                         │                            │
  │                         ├─ PUT /bucket/key ─────────▶│
  │                         │   Body: envelope           │
  │                         │   x-amz-meta-armor-*:      │
  │                         │     iv, wrapped-dek,        │
  │                         │     plaintext-size,         │
  │                         │     block-size, version     │
  │                         │                            │
  │                         │◀── 200 OK ────────────────┤
  │◀── 200 OK ─────────────┤                            │
  │                         │                            │
```

Uploads go **direct to B2** (not through Cloudflare). B2 ingress is always free.

#### Download (GetObject — Full)

```
Client                    ARMOR                    Cloudflare              B2
  │                         │                         │                     │
  ├─ GET /bucket/key ──────▶│                         │                     │
  │                         ├─ GET /bucket/key ──────▶│                     │
  │                         │  (via CF domain)        ├── GET (PNI, $0) ──▶│
  │                         │                         │◀── ciphertext ─────┤
  │                         │◀── ciphertext ──────────┤                     │
  │                         │   (cached at edge)      │                     │
  │                         │                         │                     │
  │                         ├─ Read x-amz-meta-armor-*│                     │
  │                         ├─ Unwrap DEK with MEK    │                     │
  │                         ├─ Verify block HMACs     │                     │
  │                         ├─ AES-CTR decrypt         │                     │
  │                         │                         │                     │
  │◀── plaintext ───────────┤                         │                     │
  │                         │                         │                     │
```

Downloads route **through Cloudflare** for zero-egress via the Bandwidth Alliance PNI. The bucket is set to `allPublic` — this is safe because every object is AES-256-CTR ciphertext, useless without the MEK. ARMOR assembles the Cloudflare download URL itself:

```
https://<cloudflare_domain>/file/<bucket>/<key>
```

No Cloudflare Worker is needed. Cloudflare is configured with a CNAME pointing to the B2 bucket hostname, with the proxy (orange cloud) enabled. Cloudflare caches responses at the edge, and the B2→Cloudflare egress over PNI is free.

#### Download (GetObject — Range)

```
Client                    ARMOR                    Cloudflare              B2
  │                         │                         │                     │
  ├─ GET /bucket/key ──────▶│                         │                     │
  │   Range: bytes=X-Y      │                         │                     │
  │                         ├─ HeadObject (direct ───▶│──(S3 API to B2)────▶│
  │                         │   to B2, not CF)        │                     │
  │                         │◀── x-amz-meta-armor-* ──┤◀────────────────────┤
  │                         │                         │                     │
  │                         ├─ Translate range:        │                     │
  │                         │   plaintext [X,Y]       │                     │
  │                         │   → block [B_start,     │                     │
  │                         │           B_end]        │                     │
  │                         │   → encrypted byte      │                     │
  │                         │     [enc_off, enc_end]  │                     │
  │                         │                         │                     │
  │                         ├─ GET /bucket/key ──────▶│                     │
  │                         │   Range: bytes=          │                     │
  │                         │     enc_off-enc_end     │                     │
  │                         │◀── encrypted blocks ────┤                     │
  │                         │                         │                     │
  │                         ├─ Fetch HMAC entries     │                     │
  │                         │   for blocks            │                     │
  │                         │   [B_start..B_end]      │                     │
  │                         ├─ Verify HMACs           │                     │
  │                         ├─ AES-CTR decrypt blocks │                     │
  │                         ├─ Slice to [X,Y]         │                     │
  │                         │                         │                     │
  │◀── plaintext slice ────┤                         │                     │
  │   Content-Range:        │                         │                     │
  │     bytes X-Y/total     │                         │                     │
  │                         │                         │                     │
```

This is the core value of ARMOR — DuckDB issues range reads for Parquet column chunks, and ARMOR translates them into encrypted block-level fetches through Cloudflare, decrypts only the needed blocks, and returns the plaintext slice. DuckDB's column pruning and predicate pushdown work fully.

---

## Encryption Scheme

### Algorithm: AES-256-CTR with Per-Block HMAC-SHA256

AES-CTR is chosen specifically because it enables random-access decryption. Any block at offset N can be decrypted independently given only the key, IV, and counter value N. This is what makes range reads possible without decrypting the entire file.

### Envelope Encryption

```
Master Encryption Key (MEK)
│   256-bit AES key
│   Stored on ARMOR server, never leaves the machine
│   Derived from password via Argon2id, or generated randomly
│
└─▶ wraps (AES-KWP RFC 5649) ──▶ Data Encryption Key (DEK)
                                  │   256-bit, random per-file
                                  │   Wrapped copy stored in B2 metadata
                                  │
                                  └─▶ encrypts (AES-256-CTR) ──▶ File Data
                                      │
                                      └─▶ authenticates (HMAC-SHA256) ──▶ Per-Block MACs
```

- **One DEK per file.** Compromise of one DEK exposes one file.
- **MEK wraps all DEKs.** Rotating the MEK re-wraps DEKs via `CopyObject` metadata update — no data re-upload.
- **HMAC key** is derived from the DEK via HKDF: `hmac_key = HKDF-SHA256(dek, info="armor-hmac-v1")`. Separate from the encryption key to avoid key reuse.

### Encrypted Object Format (Stored on B2)

The object stored on B2 consists of a binary envelope containing the encrypted data and integrity information:

```
┌─────────────────────────────────────────────────────────────────┐
│  Envelope Header (fixed 64 bytes)                               │
│  ┌─────────────────────────────────────────────────────────────┐│
│  │  Magic:          "ARMR"              (4 bytes)              ││
│  │  Version:        0x01                (1 byte)               ││
│  │  Block size:     65536 (log2=16)     (1 byte, stores log2) ││
│  │  IV/Nonce:                           (16 bytes)             ││
│  │  Plaintext size:                     (8 bytes, uint64 LE)  ││
│  │  Block count:                        (4 bytes, uint32 LE)  ││
│  │  Plaintext SHA-256:                  (32 bytes — computed  ││
│  │                                       before encryption)    ││
│  │  Reserved/padding:                   (remainder to 64B)     ││
│  └─────────────────────────────────────────────────────────────┘│
├─────────────────────────────────────────────────────────────────┤
│  Encrypted Data Blocks                                          │
│  ┌──────────────┐┌──────────────┐     ┌──────────────┐         │
│  │  Block 0     ││  Block 1     │ ... │  Block N     │         │
│  │  (BLOCK_SIZE)││  (BLOCK_SIZE)│     │  (≤BLOCK_SZ) │         │
│  │  CTR=IV+0    ││  CTR=IV+1    │     │  CTR=IV+N    │         │
│  └──────────────┘└──────────────┘     └──────────────┘         │
├─────────────────────────────────────────────────────────────────┤
│  HMAC Table                                                     │
│  ┌──────────────┐┌──────────────┐     ┌──────────────┐         │
│  │  HMAC(blk 0) ││  HMAC(blk 1) │ ... │  HMAC(blk N) │         │
│  │  (32 bytes)  ││  (32 bytes)  │     │  (32 bytes)  │         │
│  └──────────────┘└──────────────┘     └──────────────┘         │
└─────────────────────────────────────────────────────────────────┘

Total overhead per file = 64 (header) + 32 × block_count (HMAC table)
  For a 1 GB file with 64 KB blocks: 64 + 32 × 16384 = ~512 KB (0.05%)
```

### B2 Object Metadata (S3 Headers)

In addition to the envelope, ARMOR stores metadata in B2's S3 custom headers for fast access without reading the object body:

```
x-amz-meta-armor-version:        1
x-amz-meta-armor-block-size:     65536
x-amz-meta-armor-plaintext-size: 104857600
x-amz-meta-armor-content-type:   application/octet-stream
x-amz-meta-armor-iv:             <base64, 16 bytes>
x-amz-meta-armor-wrapped-dek:    <base64, ~48 bytes>
x-amz-meta-armor-plaintext-sha256: <hex, 64 chars>
x-amz-meta-armor-etag:           <hex MD5 of plaintext>
```

B2 file info limit: 10 headers, total ≤7000 bytes. The above uses 8 headers at ~350 bytes total — within limits with 2 slots reserved for future use (e.g., `key-id` for multi-key).

### ETag Handling

B2 returns an ETag based on the ciphertext. Clients expect an ETag based on content. ARMOR computes a plaintext-based ETag at upload time: the hex-encoded MD5 of the plaintext content (matching standard S3 ETag semantics). This is stored in `x-amz-meta-armor-etag`. HeadObject, GetObject, and ListObjectsV2 return this value as the `ETag` header. Conditional requests (`If-None-Match`, `If-Match`) are evaluated against this plaintext ETag. The B2 ciphertext ETag is never exposed to clients. For multipart uploads, the ETag follows S3's multipart convention: `md5-of-part-md5s-N`.

The metadata headers enable `HeadObject` to return the plaintext file size and content type without fetching any object data. They also allow ARMOR to unwrap the DEK and compute range offsets before issuing the data fetch.

### Block Size Selection

**Default: 64 KB (65536 bytes)**

| Block Size | Blocks/GB | HMAC Table/GB | Min Range Read | DuckDB Suitability |
|-----------|----------|---------------|---------------|-------------------|
| 4 KB | 262,144 | 8 MB | 4 KB | Excellent granularity, high HMAC overhead |
| 16 KB | 65,536 | 2 MB | 16 KB | Good balance |
| **64 KB** | **16,384** | **512 KB** | **64 KB** | **Good — matches typical Parquet page size** |
| 1 MB | 1,024 | 32 KB | 1 MB | Too coarse for column-level reads |

64 KB aligns well with Parquet page sizes (typically 8 KB–1 MB, defaulting to 1 MB in many writers but with individual pages often smaller). It keeps HMAC overhead low (0.05%) while providing granular enough range reads for DuckDB's column-chunk access patterns.

The server's `ARMOR_BLOCK_SIZE` controls the block size for **new uploads only**. On reads, ARMOR always uses the block size from the file's envelope header (or `x-amz-meta-armor-block-size`). This means an ARMOR instance configured with 16 KB blocks can correctly read files written with 64 KB blocks — the per-file header is authoritative. Rule: **read from header, write from config.**

### Range Read Translation

Given a client request for plaintext bytes `[X, Y]`:

```
HEADER_SIZE = 64  (fixed envelope header)
BLOCK_SIZE  = 65536
HMAC_SIZE   = 32

# Which encrypted blocks contain plaintext bytes [X, Y]?
block_start = X // BLOCK_SIZE
block_end   = Y // BLOCK_SIZE

# Encrypted byte range (within the B2 object)
enc_offset = HEADER_SIZE + (block_start * BLOCK_SIZE)
enc_end    = HEADER_SIZE + ((block_end + 1) * BLOCK_SIZE) - 1
# Clamp enc_end to actual encrypted data size (last block may be short)

# HMAC entries for verification
hmac_table_offset = HEADER_SIZE + (block_count * BLOCK_SIZE)
hmac_range_start  = hmac_table_offset + (block_start * HMAC_SIZE)
hmac_range_end    = hmac_table_offset + ((block_end + 1) * HMAC_SIZE) - 1

# Issue two range reads to Cloudflare (parallelized):
#   1. Encrypted blocks: Range: bytes=enc_offset-enc_end
#   2. HMAC entries:     Range: bytes=hmac_range_start-hmac_range_end
# Or combine into one if contiguous (they won't be for most partial reads)

# After fetch:
for i in range(block_start, block_end + 1):
    verify_hmac(encrypted_block[i], hmac_entries[i - block_start])

plaintext_blocks = aes_ctr_decrypt(encrypted_blocks, dek, iv, counter_start=block_start)
result = plaintext_blocks[X % BLOCK_SIZE : (X % BLOCK_SIZE) + (Y - X + 1)]
```

The two range reads (data blocks + HMAC entries) can be issued in parallel since they target different byte ranges of the same object.

**Cloudflare caching note:** Cloudflare caches range responses when the origin returns proper `Accept-Ranges: bytes` and `Content-Range` headers (B2 does). However, free-tier caching behavior for range requests is best-effort — a range miss may trigger a full origin fetch internally. ARMOR treats Cloudflare caching as a **performance optimization, not an architectural dependency.** If CF caches, latency improves; if not, the request still succeeds via origin. The `CF-Cache-Status` response header is tracked in metrics. Enterprise CF plans with Cache Reserve offer guaranteed range caching.

---

## S3 API Surface

ARMOR implements a subset of the S3 API — enough for standard tools to work. Operations are categorized by whether ARMOR transforms the request or passes it through.

### Transforming Operations (Encryption/Decryption)

These are the operations where ARMOR adds value:

| Operation | Client → ARMOR | ARMOR → B2 | Notes |
|-----------|---------------|------------|-------|
| **PutObject** | Receives plaintext body | Encrypts → uploads envelope + metadata | Direct to B2 (not via CF) |
| **GetObject** | Returns plaintext body | Fetches envelope via Cloudflare → decrypts | Range header supported |
| **HeadObject** | Returns plaintext size, content-type | Reads `x-amz-meta-armor-*` headers | No body transfer |
| **CopyObject** | Standard copy semantics | Re-wraps DEK if MEK changed; B2 server-side copy | Metadata-only update |

### Passthrough Operations (No Transformation)

These operations don't touch file data and pass through with minimal modification:

| Operation | Behavior |
|-----------|----------|
| **ListObjectsV2** | Forward to B2; adjust `Size` to plaintext sizes; **filter out `.armor/` prefixed keys** (internal objects are invisible to clients) |
| **DeleteObject** | Forward to B2 directly |
| **DeleteObjects** | Forward to B2 directly |
| **ListBuckets** | Forward to B2 directly |
| **CreateBucket** | Forward to B2 directly |
| **DeleteBucket** | Forward to B2 directly |
| **HeadBucket** | Forward to B2 directly |

### Multipart Upload Operations (Transforming)

Large files require multipart upload. ARMOR encrypts each part with a continuous CTR counter:

| Operation | Behavior |
|-----------|----------|
| **CreateMultipartUpload** | Generate DEK + IV; persist encrypted state to B2 at `.armor/multipart/<upload-id>.state`; forward to B2 |
| **UploadPart** | Encrypt part with CTR counter offset based on cumulative bytes; forward to B2 |
| **CompleteMultipartUpload** | Forward completion to B2; then upload HMAC table as sidecar object at `.armor/hmac/<key-sha256>`; write `x-amz-meta-armor-*` metadata via CopyObject with REPLACE |
| **AbortMultipartUpload** | Delete `.armor/multipart/<upload-id>.state` from B2; forward to B2 |
| **ListParts** | Forward to B2; adjust part sizes to plaintext sizes |
| **ListMultipartUploads** | Forward to B2 directly |

Multipart state (DEK, IV, counter offset, per-part HMACs) is persisted to B2 as an encrypted state object at `.armor/multipart/<upload-id>.state` on each operation. Any ARMOR instance can resume an interrupted multipart upload by reading this state object.

B2's multipart assembly concatenates parts byte-for-byte — there is no opportunity to append trailing data. Therefore, multipart-uploaded objects store the HMAC table as a **sidecar object** at `.armor/hmac/<sha256-of-key>` rather than inline. The envelope header for multipart objects includes a flag (`0x01` in the reserved byte) indicating the HMAC table is external. On download, ARMOR checks this flag and fetches the sidecar for HMAC verification.

### Operations Not Implemented

These B2 S3 API features are out of scope for v1:

- Pre-signed URLs (would expose ciphertext or require a signing proxy)
- Object tagging (B2 doesn't support it anyway)
- ACLs beyond bucket-level (B2 limitation)
- Object Lock / retention (passthrough possible in later version)
- Lifecycle configuration (passthrough possible in later version)
- Versioning — B2 bucket versioning is **not enabled** in v1. Without versioning, CopyObject during key rotation overwrites in place and old wrapped DEKs do not persist. If versioning is enabled in a future version, key rotation must expire non-current versions after completion.

### ListObjectsV2 Size Correction

B2 reports the ciphertext size (envelope header + encrypted blocks + HMAC table). ARMOR must correct this in listing responses:

```
reported_plaintext_size = int(object.metadata["x-amz-meta-armor-plaintext-size"])
```

If the metadata header is missing (non-ARMOR object), pass through the raw size unchanged. This allows mixed buckets (encrypted + unencrypted objects).

**Mixed bucket caveat:** The B2 bucket is `allPublic`. Any object uploaded to B2 *without* ARMOR encryption is publicly accessible via the Cloudflare URL. ARMOR passes through unencrypted objects transparently (detected by absence of `x-amz-meta-armor-version`). This is useful for intentionally public files but is a foot-gun if users bypass ARMOR for uploads. All uploads intended to be private **must** go through ARMOR.

---

## Server Architecture

### Component Layout

```
┌────────────────────────────────────────────────────────────┐
│                       ARMOR Server                          │
│                                                            │
│  ┌──────────────────┐  ┌───────────────────────────────┐  │
│  │  S3 Protocol      │  │  Encryption Engine             │  │
│  │  Handler          │  │  ┌─────────────────────────┐  │  │
│  │  ┌──────────┐    │  │  │ AES-256-CTR encrypt/    │  │  │
│  │  │ Router   │    │  │  │ decrypt with block-     │  │  │
│  │  │ (mux)    │────│──│─▶│ level HMAC verification │  │  │
│  │  └──────────┘    │  │  └─────────────────────────┘  │  │
│  │  ┌──────────┐    │  │  ┌─────────────────────────┐  │  │
│  │  │ SigV4    │    │  │  │ Key Manager             │  │  │
│  │  │ Auth     │    │  │  │ MEK storage, DEK wrap/  │  │  │
│  │  └──────────┘    │  │  │ unwrap, key rotation    │  │  │
│  └──────────────────┘  │  └─────────────────────────┘  │  │
│                        └───────────────────────────────┘  │
│  ┌──────────────────┐  ┌───────────────────────────────┐  │
│  │  B2 Upload Client │  │  Cloudflare Download Client   │  │
│  │  (direct to B2   │  │  (via CF domain for free      │  │
│  │   S3 endpoint)   │  │   egress, range requests)     │  │
│  └──────────────────┘  └───────────────────────────────┘  │
│                                                            │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Metadata Cache (optional)                            │  │
│  │  LRU cache of HeadObject results: IV, wrapped DEK,   │  │
│  │  plaintext size. Avoids repeated HeadObject calls     │  │
│  │  for range-read sequences (DuckDB reads footer,      │  │
│  │  then columns — same file, multiple range reads).    │  │
│  └──────────────────────────────────────────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

### Dual Backend Clients

ARMOR maintains two HTTP clients to B2, using different paths:

**Upload client** — direct to B2 S3 endpoint:
```
https://s3.<region>.backblazeb2.com
```
Used for: PutObject, UploadPart, CompleteMultipartUpload, DeleteObject, CopyObject, ListObjectsV2, HeadObject, all bucket operations. HeadObject always goes direct to B2 (zero body bytes = no egress cost; the Cloudflare `/file/` path is not the S3 API and does not return `x-amz-meta-*` headers reliably).

**Download client** — through Cloudflare domain:
```
https://armor-b2.example.com/file/<bucket>/<key>
```
Used for: GetObject (full and range). ARMOR assembles this URL from the configured Cloudflare domain, bucket name, and object key. The request routes through Cloudflare's edge network and PNI to the public B2 bucket, ensuring $0 egress. Cloudflare caches responses at the edge — repeated reads of the same encrypted blocks (common with DuckDB) are served from cache without hitting B2.

No authentication is needed on the download path because the bucket is public. This is safe: every stored object is AES-256-CTR ciphertext, completely opaque without the MEK. Public access to ciphertext is equivalent to accessing `/dev/urandom`. This also means Cloudflare can freely cache responses (no `Authorization` header to bypass caching).

### Authentication

ARMOR accepts standard AWS Signature V4 requests. Clients configure credentials that ARMOR validates:

```
# Client-side (e.g., DuckDB, boto3, AWS CLI)
S3_ENDPOINT=http://localhost:9000
S3_ACCESS_KEY_ID=armor-local-key
S3_SECRET_ACCESS_KEY=armor-local-secret
```

These are **ARMOR-specific credentials**, not B2 credentials. ARMOR validates them locally and then uses its own B2 credentials to talk to the backend. This means:

- B2 credentials never leave the ARMOR server
- Multiple clients can share ARMOR with different access keys
- Access keys can be scoped per-bucket or per-prefix (ARMOR enforces this)

For a single-user deployment (the primary use case), a single static key pair in the config file is sufficient.

### Metadata Cache

DuckDB's Parquet read pattern generates multiple range reads against the same file in rapid succession:

1. Read footer (last 8 bytes → footer length → footer body)
2. Read column chunk A from row group 1
3. Read column chunk A from row group 3
4. Read column chunk B from row group 1
5. ...

Each range read requires the file's IV and DEK for decryption. Without caching, each range read would trigger a HeadObject call to B2. The metadata cache stores recently accessed file metadata (IV, wrapped DEK, plaintext size, block size) in an LRU cache:

```
Cache key:   (bucket, object_key)
Cache value: ArmorMetadata { iv, wrapped_dek, plaintext_size, block_size, content_type }
TTL:         5 minutes (configurable)
Max entries: 10,000 (configurable)
```

The unwrapped DEK is also cached (it's derived from the wrapped DEK + MEK, which are both in memory anyway). This avoids repeated AES-KWP unwrap operations.

---

## Configuration

ARMOR is configured exclusively via environment variables. No config files.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_LISTEN` | No | `0.0.0.0:9000` | S3 API listen address |
| `ARMOR_ADMIN_LISTEN` | No | `127.0.0.1:9001` | Admin API listen address (key rotation, canary, audit). Localhost-only by default — never expose externally. |
| `ARMOR_B2_REGION` | Yes | — | B2 region (e.g., `us-east-005`) |
| `ARMOR_B2_ENDPOINT` | No | `https://s3.{region}.backblazeb2.com` | B2 S3 endpoint (auto-derived from region) |
| `ARMOR_B2_ACCESS_KEY_ID` | Yes | — | B2 application key ID |
| `ARMOR_B2_SECRET_ACCESS_KEY` | Yes | — | B2 application key |
| `ARMOR_BUCKET` | Yes | — | B2 bucket name. Used for both uploads (direct to B2) and downloads (Cloudflare URL assembly). |
| `ARMOR_CF_DOMAIN` | Yes | — | Cloudflare domain CNAME'd to B2 bucket (e.g., `armor-b2.example.com`) |
| `ARMOR_MEK` | Yes | — | Master encryption key, hex-encoded 32 bytes. Generate with `openssl rand -hex 32`. |
| `ARMOR_AUTH_ACCESS_KEY` | No | (random on startup) | S3 access key ID for client auth to ARMOR |
| `ARMOR_AUTH_SECRET_KEY` | No | (random on startup) | S3 secret access key for client auth to ARMOR |
| `ARMOR_BLOCK_SIZE` | No | `65536` | Encryption block size for new uploads (power of 2, ≥4096). Existing files use their own block size from the envelope header. |
| `ARMOR_WRITER_ID` | No | (hostname) | Provenance chain writer ID. Set per cluster for multi-writer deployments. |
| `ARMOR_CACHE_MAX_ENTRIES` | No | `10000` | Metadata cache max entries |
| `ARMOR_CACHE_TTL` | No | `300` | Metadata cache TTL in seconds |

ARMOR assembles the Cloudflare download URL as:
```
https://${ARMOR_CF_DOMAIN}/file/${ARMOR_BUCKET}/<key>
```

### Deployment

ARMOR is deployed exclusively as a Docker container — there is no standalone binary distribution. It runs as a Kubernetes pod (sidecar or standalone), a Docker Compose service, or a plain `docker run`:

```yaml
# Secret with B2 creds and MEK
apiVersion: v1
kind: Secret
metadata:
  name: armor-secrets
type: Opaque
stringData:
  b2-access-key-id: "..."
  b2-secret-access-key: "..."
  master-encryption-key: "..."
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: armor
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: armor
        image: ghcr.io/jedarden/armor:latest
        ports:
        - containerPort: 9000
        env:
        - name: ARMOR_B2_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: b2-access-key-id
        - name: ARMOR_B2_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: b2-secret-access-key
        - name: ARMOR_MEK
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: master-encryption-key
        - name: ARMOR_BUCKET
          value: "my-bucket"
        - name: ARMOR_CF_DOMAIN
          value: "armor-b2.example.com"
```

---

## Cloudflare Setup

No Cloudflare Worker is needed. The B2 bucket is set to `allPublic` and Cloudflare acts as a pure caching CDN proxy. This is safe because every stored object is AES-256-CTR ciphertext — useless without the MEK that lives on the ARMOR server.

### How It Works

ARMOR assembles download URLs using the configured Cloudflare domain:

```
https://<cloudflare_domain>/file/<bucket>/<key>
```

Cloudflare receives the request, checks its edge cache, and on a miss forwards to the B2 origin over the Bandwidth Alliance PNI (free egress). The response is cached at the edge for subsequent requests.

Because the bucket is public and no `Authorization` header is sent, Cloudflare caches freely — no workarounds needed for the auth-header-bypasses-cache problem.

### DNS Configuration

```
armor-b2.example.com  CNAME  f004.backblazeb2.com  (proxied / orange cloud)
```

The CNAME target is the B2 bucket's friendly hostname. To find it, upload any file to the bucket and check the download URL in the B2 web UI — the hostname portion (e.g., `f004.backblazeb2.com`) is the CNAME target.

### Cloudflare Configuration

- [ ] Domain on Cloudflare (e.g., `example.com`)
- [ ] CNAME: `armor-b2.example.com` → B2 bucket friendly hostname (proxied)
- [ ] SSL mode: **Full (strict)** — B2 requires HTTPS
- [ ] **Disable Automatic Signed Exchanges (SXGs)** — incompatible with B2
- [ ] URL Rewrite Transform Rule (if needed): prepend `/file/<bucket>/` to paths
- [ ] Response header cleanup: strip `x-bz-file-name`, `x-bz-file-id`, `x-bz-content-sha1`, `x-bz-upload-timestamp` (optional, cosmetic)
- [ ] Cache-Control: set `public, max-age=86400` for all responses (ciphertext is immutable per-version)

### B2 Bucket Configuration

- [ ] Bucket type: `allPublic`
- [ ] Set `Cache-Control: public, max-age=86400` in bucket info (default for all files)
- [ ] CORS rules if browser access is needed (unlikely for ARMOR's server-to-B2 path)

### Why Public Is Safe

| Concern | Why it's not a risk |
|---------|-------------------|
| Anyone can download files | They get AES-256-CTR ciphertext — indistinguishable from random bytes without the MEK |
| File listing exposure | File names are visible, but file contents are opaque. If names are sensitive, enable filename encryption (v2 feature) |
| No access control | The access control boundary is ARMOR, not B2. ARMOR validates client auth before proxying |
| Cloudflare can inspect content | It's ciphertext. Cloudflare sees the same random-looking bytes as any other attacker |

---

## Key Management

### MEK Lifecycle

MEK generation is a one-time offline step before deployment:
```bash
openssl rand -hex 32    # → set as ARMOR_MEK env var
```

Key management operations are exposed as HTTP API endpoints on the admin listener (`ARMOR_ADMIN_LISTEN`, default `127.0.0.1:9001`):

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/admin/key/verify` | GET | Decrypt the canary object to verify MEK is correct |
| `/admin/key/rotate` | POST | Accept new MEK in request body, re-wrap all DEKs via CopyObject |
| `/admin/key/export` | GET | Return current MEK (base64). Requires explicit `?confirm=yes` parameter. |

In Kubernetes, operators interact via:
```bash
kubectl exec deploy/armor -- curl -s localhost:9001/admin/key/verify
```

### Key Rotation

When the MEK is rotated, every file's DEK must be re-wrapped:

```
For each object in bucket:
  1. HeadObject → read x-amz-meta-armor-wrapped-dek
  2. Unwrap DEK with old MEK
  3. Re-wrap DEK with new MEK
  4. CopyObject with MetadataDirective=REPLACE, new x-amz-meta-armor-wrapped-dek
```

This is an O(N) metadata operation — no data is re-uploaded. A 100,000-file bucket takes ~100K API calls. After May 2026, these are free. The rotation command is idempotent (safe to re-run if interrupted) — it tracks progress in B2 at `.armor/rotation-state.json`, so any ARMOR instance can resume an interrupted rotation.

### Multi-Key Support (v2)

For data classification (e.g., different keys for different prefixes), additional keys use named env vars:

```bash
ARMOR_MEK=<hex>                              # default key
ARMOR_MEK_SENSITIVE=<hex>                    # key named "sensitive"
ARMOR_MEK_ARCHIVE=<hex>                      # key named "archive"
ARMOR_KEY_ROUTES="data/pii/*=sensitive,archive/*=archive,*=default"
```

The key ID is stored in `x-amz-meta-armor-key-id` so ARMOR knows which MEK to use for decryption. No file paths, no volume mounts — consistent with env-var-only configuration.

---

## Language and Dependencies

### Language: Go

Go is the best fit for ARMOR:

- **HTTP server:** `net/http` stdlib is production-grade for S3 protocol handling
- **Concurrency:** Goroutines handle parallel range reads, concurrent client requests
- **Crypto:** `crypto/aes`, `crypto/cipher` (AES-CTR), `crypto/hmac` — all stdlib, hardware-accelerated via AES-NI
- **B2 SDK:** Official `github.com/Backblaze/blazer` + `github.com/aws/aws-sdk-go-v2` for S3
- **Docker-native:** Compiles to a single static binary → minimal Docker image (`FROM scratch`). ARMOR is deployed exclusively as a container.
- **Prior art:** MinIO gateway, SeaweedFS S3 proxy, Garage — all Go S3-compatible servers

### Key Dependencies

| Dependency | Purpose |
|-----------|---------|
| `aws-sdk-go-v2` | S3 client for B2 uploads, ListObjects, HeadObject, CopyObject, etc. |
| `aws-sdk-go-v2/service/s3` | S3 protocol types and signing |
| `blazer` (optional) | B2 native API for key management (create/delete/list application keys) |
| `crypto/aes`, `crypto/cipher` | AES-256-CTR encryption/decryption |
| `crypto/hmac`, `crypto/sha256` | Per-block HMAC-SHA256 |
| `golang.org/x/crypto/hkdf` | HKDF key derivation (DEK → HMAC key) |
| `golang.org/x/crypto/argon2` | MEK derivation from password (v2, optional) |

### S3 Protocol Handling

ARMOR implements its own S3 XML request/response handling for the ~15 operations in scope using Go's `net/http` and `encoding/xml`. The operation set is small and well-defined. MinIO's gateway pattern was rejected — it brings massive dependency weight for features ARMOR does not need and was deprecated upstream. A custom router gives full control over the encryption boundary and keeps the binary small.

---

## Implementation Phases

### Phase 1: Core (MVP) ✅ COMPLETE

**Goal:** DuckDB can `read_parquet('s3://bucket/path')` through ARMOR with full range-read support.

- [x] S3 protocol handler: PutObject, GetObject (full + range), HeadObject, DeleteObject, ListObjectsV2
- [x] AES-256-CTR encryption with per-block HMAC
- [x] Envelope encryption (MEK wraps DEK per file) with format versioning (version byte dispatch)
- [x] Encrypted object format (header + data blocks + HMAC table)
- [x] Range read translation (plaintext offset → encrypted block offset)
- [x] Parallel data + HMAC range fetch (errgroup, two concurrent range reads)
- [x] Pipelined stream decryption (decrypt-as-blocks-arrive, io.Pipe)
- [x] Pluggable backend interface with B2 S3 implementation
- [x] Dual backend paths: direct-to-B2 uploads, Cloudflare-routed downloads
- [x] Env var configuration (no config file required)
- [ ] Cloudflare DNS setup (CNAME + SSL + cache rules) — operational/deployment task
- [x] Metadata cache (LRU, in-memory)
- [x] Parquet footer pinning (in-memory, keyed by ETag)
- [x] Self-healing canary integrity monitor
- [x] Health check endpoints (`/healthz`, `/readyz`, `/armor/canary`)
- [x] Multi-stage Dockerfile (build + scratch runtime) + CI build + GHCR publish

**Validation:** Point DuckDB at ARMOR, upload a Parquet file, run `SELECT` with predicates and column selection, verify only partial data is fetched.

### Phase 2: Production Hardening ✅ COMPLETE

**Goal:** Reliable for continuous use with operational tooling.

- [x] Multipart upload (CreateMultipartUpload, UploadPart, CompleteMultipartUpload, AbortMultipartUpload)
- [x] Multipart state stored in B2 (`.armor/multipart/<upload-id>.state`) for crash recovery
- [x] CopyObject (for rename and key rotation)
- [x] Key rotation via API endpoint (re-wraps DEKs via CopyObject, progress in `.armor/rotation-state.json`)
- [x] DeleteObjects (bulk delete)
- [x] ListBuckets, CreateBucket, DeleteBucket, HeadBucket
- [x] Cryptographic provenance chain (per-writer branches, `.armor/chain-head/<writer-id>`)
- [x] Audit endpoint (`GET /armor/audit`) — walk and verify provenance chains
- [x] Graceful shutdown + in-flight request draining
- [x] Structured logging (JSON)
- [x] Prometheus metrics: request count, latency, bytes transferred, cache hit rate, encryption ops, canary status
- [x] Kubernetes manifests (Deployment, Service, Secret)
- [x] Integration tests against real B2 + Cloudflare

### Phase 3: Advanced Features ✅ COMPLETE

**Goal:** Multi-user, multi-key, full S3 compatibility.

- [x] Multi-key routing (different MEKs for different prefixes)
- [x] Multiple auth credentials with per-key ACLs
- [x] ListObjectVersions with per-version decryption
- [x] Pre-signed URL proxy (ARMOR-signed URLs that trigger decrypt-on-fetch)
- [x] Streaming encryption for very large uploads (>5 GB)
- [x] Object Lock / retention passthrough
- [x] Lifecycle rule passthrough
- [x] Admin API: key management via B2 native API
- [ ] Web dashboard (optional): bucket browser, encryption status, cache stats

---

## Project Structure

```
ARMOR/
├── Dockerfile                   # Multi-stage: Go build + scratch runtime
├── cmd/
│   └── armor/
│       └── main.go              # Entrypoint: starts S3 server, reads env vars
├── internal/
│   ├── server/
│   │   ├── server.go            # HTTP server setup, middleware, graceful shutdown
│   │   ├── router.go            # S3 operation routing
│   │   ├── auth.go              # SigV4 validation
│   │   └── handlers/
│   │       ├── get_object.go    # GetObject + range read logic
│   │       ├── put_object.go    # PutObject encryption
│   │       ├── head_object.go   # HeadObject metadata translation
│   │       ├── delete_object.go
│   │       ├── list_objects.go  # ListObjectsV2 with size correction
│   │       ├── multipart.go     # Multipart upload operations
│   │       ├── copy_object.go   # CopyObject + key rotation
│   │       └── bucket.go        # Bucket operations (passthrough)
│   ├── crypto/
│   │   ├── envelope.go          # Envelope format: header, blocks, HMAC table
│   │   ├── encryptor.go         # AES-CTR encrypt + HMAC per block
│   │   ├── decryptor.go         # AES-CTR decrypt + HMAC verify per block
│   │   ├── range.go             # Plaintext-to-encrypted range translation
│   │   ├── keys.go              # MEK load/generate, DEK wrap/unwrap (AES-KWP)
│   │   └── hkdf.go              # DEK → HMAC key derivation
│   ├── backend/
│   │   ├── backend.go           # Backend interface (pluggable storage)
│   │   ├── b2.go                # B2 S3 backend implementation
│   │   ├── cloudflare.go        # Cloudflare download client (range reads)
│   │   └── cache.go             # Metadata LRU cache
│   ├── canary/
│   │   └── canary.go            # Self-healing canary integrity monitor
│   ├── admin/
│   │   └── admin.go             # Admin API handlers (key rotate, audit, canary)
│   └── config/
│       └── config.go            # Env var configuration loading (no file parsing)
├── deploy/
│   └── kubernetes/
│       ├── deployment.yaml
│       ├── service.yaml
│       └── secret.yaml
├── docs/
│   ├── plan/
│   │   └── plan.md              # This file
│   └── research/
│       └── ...                  # Research documents
├── go.mod
├── go.sum
└── README.md
```

---

## Testing Strategy

### Unit Tests

- `crypto/` package: encrypt → decrypt roundtrip, range translation correctness, HMAC verification, envelope parsing, key wrap/unwrap
- `handlers/`: mock B2 backend, verify correct S3 XML responses, range header parsing, size correction in listings

### Integration Tests

- Start ARMOR against real B2 bucket + Cloudflare CDN
- Upload via boto3 → download via boto3 → verify content matches
- Upload → range read → verify partial content
- Upload Parquet → DuckDB query with `WHERE` clause → verify results + verify only partial data fetched (check ARMOR logs for range requests)
- Upload → delete → verify 404
- Upload → CopyObject → verify both copies decrypt correctly
- Key rotation → verify all files still decrypt

### Compatibility Tests

- AWS CLI: `aws s3 cp`, `aws s3 ls`, `aws s3 rm` against ARMOR
- DuckDB: `read_parquet('s3://...')` with httpfs extension
- boto3: full upload/download/list/delete cycle
- rclone: `rclone copy` to/from ARMOR

---

## Operational Considerations

### Threat Model

| Threat | Mitigation |
|--------|-----------|
| B2 data breach | All stored data is AES-256-CTR encrypted with per-file DEKs |
| Cloudflare CDN inspection | All cached content is ciphertext |
| ARMOR server compromise | MEK exposed — rotate immediately; per-file DEKs limit blast radius |
| Network sniffing (client ↔ ARMOR) | TLS on ARMOR listener, or localhost-only binding |
| Public bucket enumeration | Attacker can list/download ciphertext — useless without MEK |
| Bit-flipping attack on ciphertext | Per-block HMAC-SHA256 detects any modification |
| Block reordering/truncation | Block index is implicit in offset; HMAC table length validates block count |

### Performance Expectations

| Operation | Bottleneck | Expected Throughput |
|-----------|-----------|-------------------|
| Upload (small file) | AES-CTR encrypt + B2 upload latency | ~100 MB/s (AES-NI), limited by B2 RTT |
| Upload (large file, multipart) | AES-CTR encrypt + B2 multipart throughput | ~500 MB/s with parallel parts |
| Download (full, cache miss) | Cloudflare → B2 fetch + AES-CTR decrypt | ~200 MB/s (PNI throughput) |
| Download (full, cache hit) | Cloudflare edge → ARMOR → AES-CTR decrypt | ~500 MB/s+ (edge cache) |
| Range read (cache miss) | Cloudflare → B2 range fetch + decrypt | ~50 MB/s (latency-bound per request) |
| Range read (cache hit) | Cloudflare edge + decrypt | Sub-millisecond per block from edge |
| HeadObject (cached) | In-memory lookup | <1 μs |
| ListObjectsV2 | B2 listing latency | Same as raw B2 |

### Monitoring

Key metrics to export via Prometheus:

```
armor_requests_total{operation, status}        # Request count by S3 operation
armor_request_duration_seconds{operation}       # Latency histogram
armor_bytes_uploaded_total                      # Plaintext bytes received from clients
armor_bytes_downloaded_total                    # Plaintext bytes served to clients
armor_bytes_fetched_from_b2_total               # Ciphertext bytes fetched from B2/CF
armor_range_reads_total                         # Number of range read requests
armor_range_bytes_saved_total                   # Bytes NOT transferred due to range reads
armor_metadata_cache_hits_total                 # Cache hit count
armor_metadata_cache_misses_total               # Cache miss count
armor_encryption_operations_total{type}         # encrypt/decrypt/wrap/unwrap counts
armor_active_multipart_uploads                  # In-progress multipart uploads
```

---

## Key Features

### 1. Parquet Footer Pinning

Parquet files are immutable once written. DuckDB's first action on every query is reading the footer (schema, row group offsets, column statistics). Without pinning, every query triggers: range read to Cloudflare → block fetch → HMAC verify → decrypt — for the same unchanging bytes.

Footer pinning caches decrypted footers in memory on first access, keyed by `(bucket, key, ETag)`. Subsequent reads return from memory in microseconds. Footers are typically a few KB — caching thousands costs negligible memory. The ETag ensures cache coherence when a file is replaced.

**Impact:** 50-80% reduction in DuckDB query startup latency for repeated queries against the same files.

### 2. Pipelined Stream Decryption

Since AES-CTR blocks are independent, ARMOR decrypts each 64 KB block the instant it arrives from Cloudflare, streaming plaintext to the client while subsequent blocks are still in flight.

```
Cloudflare response body (streaming):
  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
  │ Block 0 │→│ Block 1 │→│ Block 2 │→│ Block 3 │→ ...
  └────┬────┘ └────┬────┘ └────┬────┘ └────┬────┘
       │           │           │           │
       ▼           ▼           ▼           ▼
    HMAC+decrypt  HMAC+decrypt HMAC+decrypt HMAC+decrypt
       │           │           │           │
       ▼           ▼           ▼           ▼
Client response body (streaming):
  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐
  │Plain  0 │→│Plain  1 │→│Plain  2 │→│Plain  3 │→ ...
  └─────────┘ └─────────┘ └─────────┘ └─────────┘
```

This applies to **any file type** — Parquet, binary, CSV, images, archives. The block-level pipeline is format-agnostic. For a 100 MB download, the client starts receiving plaintext after the first 64 KB arrives (~milliseconds), not after 100 MB is buffered.

**HMAC prefetch:** The HMAC table is at the end of the file, so it hasn't arrived when the first data blocks stream in. ARMOR issues a small range read for the HMAC table *before* starting the data stream. The table's offset and size are computable from the envelope header (`block_count × 32 bytes`). For a 1 GB file, the HMAC table is ~512 KB — negligible prefetch latency. Once the table is in memory, each data block is verified as it arrives.

```
1. Range read: HMAC table (small, fast)     ← prefetch
2. Stream read: envelope header (64 bytes)  ← parse, consume, do NOT forward to client
3. Stream read: block 0 → verify + decrypt → pipe to client
4. Stream read: block 1 → verify + decrypt → pipe to client
5. ... (stop before HMAC table offset — client gets plaintext size bytes only)
```

For full downloads, the Cloudflare response includes the entire B2 object (header + encrypted blocks + HMAC table). The streaming decryptor reads and discards the 64-byte header first (`io.ReadFull(body, headerBuf[:64])`), then processes data blocks, and stops reading at the HMAC table offset (already prefetched). For range reads, the encrypted byte offset calculation already accounts for the header (`enc_offset = HEADER_SIZE + ...`), so the header is never fetched.

**Impact:** Time-to-first-byte approaches raw Cloudflare latency plus one HMAC-table round-trip. Full downloads complete in decrypt-throughput time, not download-then-decrypt time.

### 3. Parallel Data + HMAC Range Fetch

Every range read requires two non-overlapping byte ranges from the same B2 object: the encrypted data blocks and the HMAC entries. These are always at different offsets (data blocks near the front, HMAC table at the end).

```go
g, ctx := errgroup.WithContext(ctx)

g.Go(func() error {
    dataBlocks, err = cf.GetRange(key, dataOffset, dataLen)
    return err
})
g.Go(func() error {
    hmacEntries, err = cf.GetRange(key, hmacOffset, hmacLen)
    return err
})

if err := g.Wait(); err != nil {
    return err
}
// Both fetches complete → verify + decrypt
```

Cuts range-read latency nearly in half for Cloudflare cache misses. For DuckDB queries issuing 20+ range reads, the savings compound.

**Impact:** Highest impact-to-complexity ratio of any feature. ~15 lines of code.

### 4. Pluggable Backend Abstraction

ARMOR's encryption logic is completely independent of the storage backend. A clean `Backend` interface decouples the crypto layer from B2:

```go
type Backend interface {
    Put(ctx context.Context, bucket, key string, body io.Reader, meta map[string]string) error
    Get(ctx context.Context, bucket, key string) (io.ReadCloser, map[string]string, error)
    GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error)
    Head(ctx context.Context, bucket, key string) (map[string]string, error)
    Delete(ctx context.Context, bucket, key string) error
    List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error)
    Copy(ctx context.Context, bucket, srcKey, dstKey string, meta map[string]string) error
}
```

The B2 S3 client becomes the first implementation. Future backends (MinIO, R2, Wasabi, local filesystem) are additional implementations. This also enables:
- Mock backend for unit tests (no cloud credentials needed)
- Tiered storage (hot/cold across providers)
- Air-gapped deployments with local-filesystem backend

**Impact:** Future-proofs the project; immediate testability improvement.

### 5. Encryption Format Versioning

The `ARMR` magic + version byte in the envelope header is already planned, but making it a first-class architectural pattern now — with an `EnvelopeReader` interface that dispatches on version — prevents a painful retrofit later when there are terabytes of v1 objects in production.

```go
type EnvelopeReader interface {
    ReadHeader(r io.Reader) (*EnvelopeHeader, error)
    DecryptBlock(block []byte, blockIndex uint32, dek, iv []byte) ([]byte, error)
    VerifyHMAC(block []byte, blockIndex uint32, hmacKey, expected []byte) error
}

var readers = map[uint8]EnvelopeReader{
    0x01: &EnvelopeV1{},  // AES-256-CTR + HMAC-SHA256
    // 0x02: &EnvelopeV2{},  // Future: AES-256-GCM for non-range-read files
    // 0x03: &EnvelopeV3{},  // Future: post-quantum KEM layer
}
```

ARMOR reads any version forever. New uploads use the latest version. A version switch + one interface — costs almost nothing now, would be extremely expensive later.

**Impact:** Zero future migration pain when algorithms change.

### 6. Cryptographic Provenance Chain

Each upload computes a chain hash linking it to the previous upload:

```
chain_hash = SHA-256(prev_chain_hash || object_key || plaintext_sha256 || timestamp || writer_id)
```

Provenance entries are stored as lightweight objects under `.armor/chain/<writer_id>/<sequence>.json` in B2, each containing the object key, plaintext SHA-256, timestamp, and chain link hash. This avoids consuming per-object B2 metadata header slots (B2 has a 10-header limit; keeping provenance out of object headers leaves room for future metadata fields like `key-id` for multi-key support). The latest chain head is stored at `.armor/chain-head/<writer_id>` in B2.

`armor audit` (via API endpoint `GET /armor/audit`) walks the chain and verifies every link.

#### Multi-Writer Consistency

With multiple ARMOR instances writing concurrently (e.g., multiple spot clusters), naive chain appending creates conflicts. The solution is **per-writer chains that merge**:

```
Writer A (cluster-1):  A1 ─→ A2 ─→ A3 ─→ ...
Writer B (cluster-2):  B1 ─→ B2 ─→ ...
Writer C (cluster-3):  C1 ─→ C2 ─→ C3 ─→ C4 ─→ ...
```

Each ARMOR instance maintains its own chain branch, identified by `ARMOR_WRITER_ID` (configured per instance, defaults to hostname). Each writer stores its own chain head at `.armor/chain-head/<writer_id>` and appends entries as objects at `.armor/chain/<writer_id>/<sequence>.json`:

```json
{
  "sequence": 42,
  "object_key": "data/2026-03-24/part-0000.parquet",
  "plaintext_sha256": "a1b2c3...",
  "chain_hash": "d4e5f6...",
  "prev_chain_hash": "789abc...",
  "timestamp": "2026-03-24T14:30:00Z"
}
```

**Verification:** `GET /admin/audit` reads all chain heads from `.armor/chain-head/*`, walks each branch independently by reading the chain entry objects, and cross-references the full set of objects in the bucket. An object that belongs to no chain was uploaded outside ARMOR or tampered with. A gap in a chain indicates deletion.

**No coordination required between writers.** Each writer is fully independent. The merge is read-side only (during audit).

**Impact:** Tamper-evident audit trail for financial/trading data with zero coordination overhead between clusters.

### 7. Self-Healing Canary Integrity Monitor

On startup and at a configurable interval (default: every 5 minutes), ARMOR:

1. Uploads a small known-content canary file to `.armor/canary/<instance-id>` in B2
2. Downloads it through the full Cloudflare path
3. Decrypts and verifies the plaintext matches the known content
4. Verifies the HMAC chain
5. Reports status on the `/healthz` endpoint

```
Canary lifecycle:
  Upload (plaintext) → ARMOR encrypt → B2
  Download ← ARMOR decrypt ← Cloudflare ← B2
  Verify: plaintext matches? HMACs valid? MEK correct?
```

This single test exercises the **entire pipeline**: encryption, B2 upload, Cloudflare CDN, range reads (it reads the canary with a range request to verify that path too), HMAC verification, decryption, and MEK correctness.

#### Self-Healing

When the canary fails, ARMOR doesn't just alert — it attempts to diagnose and fix:

| Failure Mode | Detection | Self-Healing Action |
|---|---|---|
| Cloudflare DNS misconfigured | Canary download returns non-B2 response | Log error with expected vs actual hostname; set `/healthz` to unhealthy |
| Cloudflare cache serving stale data | Canary content doesn't match (old version) | Re-upload canary with a new unique key (`.armor/canary/<instance-id>/<timestamp>`), re-test against the new URL. No CF API credentials needed. |
| B2 connectivity lost | Upload or download times out / 5xx | Retry with exponential backoff; set `/healthz` to unhealthy after 3 failures |
| MEK mismatch | Decryption produces garbage (HMAC fails) | Log critical: "MEK does not match data in B2"; refuse to serve traffic |
| HMAC verification fails | Block HMAC doesn't match | Log critical: "Data integrity violation"; refuse to serve traffic for affected bucket |
| B2 silent corruption | Plaintext doesn't match known content despite valid HMAC | Log critical: "Canary content mismatch — possible B2 corruption or HMAC collision" |

The canary endpoint is exposed at `GET /armor/canary` and returns structured JSON:

```json
{
  "status": "healthy",
  "last_check": "2026-03-24T14:30:00Z",
  "upload_latency_ms": 45,
  "download_latency_ms": 12,
  "decrypt_verified": true,
  "hmac_verified": true,
  "cloudflare_cache_hit": true
}
```

Kubernetes liveness and readiness probes point at `/healthz`, which incorporates canary status. A failing canary causes the pod to be restarted (liveness) or removed from service (readiness), forcing traffic to a healthy ARMOR instance.

**Impact:** Catches pipeline problems before users hit them. Self-healing for transient issues. Hard-fail for integrity violations.

---

## Decisions and Rationale

| Decision | Choice | Why |
|----------|--------|-----|
| Deployment | Docker container only | Stateless; no binary distribution; consistent environment across clusters |
| Language | Go | Stdlib crypto (AES-NI), excellent HTTP server, static binary → minimal Docker image |
| Encryption | AES-256-CTR | Random access decryption — the single most important property for range reads |
| Integrity | HMAC-SHA256 per block | Independent verification of any block without reading the full file |
| Key wrapping | AES-KWP (RFC 5649) | Standard, constant-time, no IV management for wrapping |
| Block size | 64 KB | Matches Parquet page sizes; 0.05% HMAC overhead; good range-read granularity |
| Upload path | Direct to B2 | Ingress is free; avoids Cloudflare body size limits |
| Download path | Via Cloudflare CDN | $0 egress via Bandwidth Alliance PNI; edge caching; no Worker needed (public bucket + ciphertext) |
| Filenames | Plaintext (v1) | Enables DuckDB partition discovery; directory structure is not sensitive for this use case |
| Metadata storage | B2 S3 headers + envelope header | Headers enable fast HeadObject; envelope enables offline reading |
| Auth model | ARMOR-local SigV4 | B2 creds stay on server; standard S3 client compatibility |
| State | Stateless (all state in B2) | Any instance with the same config works; horizontal scaling; crash recovery |
| Backend | Pluggable interface | Future-proofs against provider changes; enables testing without cloud credentials |
| Provenance | Per-writer chains | Tamper-evident audit trail without coordination between concurrent writers |
