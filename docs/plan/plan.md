# ARMOR Implementation Plan

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

Downloads route **through Cloudflare** for zero-egress via the Bandwidth Alliance PNI.

#### Download (GetObject — Range)

```
Client                    ARMOR                    Cloudflare              B2
  │                         │                         │                     │
  ├─ GET /bucket/key ──────▶│                         │                     │
  │   Range: bytes=X-Y      │                         │                     │
  │                         ├─ HeadObject (CF) ──────▶│ (fetch metadata)    │
  │                         │◀── x-amz-meta-armor-* ──┤                     │
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
```

B2 file info limit: 10 headers, total ≤7000 bytes. The above uses 7 headers at ~300 bytes total — well within limits.

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

Configurable per-server — not per-file — to keep the implementation simple. All files in a bucket use the same block size.

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

The two range reads (data blocks + HMAC entries) can be issued in parallel since they target different byte ranges of the same object. Cloudflare caches range responses, so repeated reads of the same blocks (common in DuckDB's access pattern) hit the edge cache.

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
| **ListObjectsV2** | Forward to B2; adjust `Size` in response to report plaintext sizes |
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
| **CreateMultipartUpload** | Generate DEK + IV; store in ARMOR's local state; forward to B2 |
| **UploadPart** | Encrypt part with CTR counter offset based on cumulative bytes; forward to B2 |
| **CompleteMultipartUpload** | Upload HMAC table as final operation; write `x-amz-meta-armor-*` to B2; forward completion |
| **AbortMultipartUpload** | Clean up local state; forward to B2 |
| **ListParts** | Forward to B2; adjust part sizes to plaintext sizes |
| **ListMultipartUploads** | Forward to B2 directly |

Multipart state (DEK, IV, counter offset, per-part HMACs) is held in server memory during the upload and persisted to a local state file for crash recovery.

### Operations Not Implemented

These B2 S3 API features are out of scope for v1:

- Pre-signed URLs (would expose ciphertext or require a signing proxy)
- Object tagging (B2 doesn't support it anyway)
- ACLs beyond bucket-level (B2 limitation)
- Object Lock / retention (passthrough possible in later version)
- Lifecycle configuration (passthrough possible in later version)
- Versioning (passthrough possible, but each version is independently encrypted)

### ListObjectsV2 Size Correction

B2 reports the ciphertext size (envelope header + encrypted blocks + HMAC table). ARMOR must correct this in listing responses:

```
reported_plaintext_size = int(object.metadata["x-amz-meta-armor-plaintext-size"])
```

If the metadata header is missing (non-ARMOR object), pass through the raw size unchanged. This allows mixed buckets (encrypted + unencrypted objects).

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
Used for: PutObject, UploadPart, CompleteMultipartUpload, DeleteObject, CopyObject, ListObjectsV2, HeadObject (when not cached), all bucket operations.

**Download client** — through Cloudflare domain:
```
https://armor-b2.example.com
```
Used for: GetObject (full and range). This routes through Cloudflare's edge network and PNI to B2, ensuring $0 egress. Cloudflare caches responses at the edge — repeated reads of the same encrypted blocks (common with DuckDB) are served from cache without hitting B2.

The download client does NOT use S3 auth headers (which would bypass Cloudflare caching). Instead, the Cloudflare Worker holds a read-only B2 application key and injects auth on the B2 side. ARMOR authenticates to the Worker via a shared secret or HMAC token in a custom header.

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

### Config File

```toml
# /etc/armor/config.toml (or ~/.config/armor/config.toml)

[server]
listen = "127.0.0.1:9000"      # Bind address. Localhost-only by default.
# listen = "0.0.0.0:9000"      # Expose to network (use with TLS)
tls_cert = ""                   # Path to TLS cert (optional, for HTTPS)
tls_key  = ""                   # Path to TLS key

[auth]
access_key_id     = "armor-local-key"
secret_access_key = "armor-local-secret"

[b2]
region            = "us-east-005"
endpoint          = "https://s3.us-east-005.backblazeb2.com"
access_key_id     = "<B2 application key ID>"
secret_access_key = "<B2 application key>"
# default_bucket  = "my-bucket"  # Optional: restrict to single bucket

[cloudflare]
download_domain   = "armor-b2.example.com"
# How ARMOR authenticates to the Cloudflare Worker
worker_auth_header = "X-Armor-Token"
worker_auth_token  = "<shared secret>"

[encryption]
block_size = 65536              # 64 KB (must be power of 2, ≥4096)
# MEK source: "file", "env", or "keyring"
mek_source = "file"
mek_path   = "/etc/armor/master.key"  # 32 bytes, hex or base64 encoded
# mek_source = "env"
# mek_env_var = "ARMOR_MASTER_KEY"

[cache]
metadata_max_entries = 10000
metadata_ttl_seconds = 300
```

### Environment Variable Overrides

Every config field can be overridden via environment variable:

```bash
ARMOR_LISTEN=0.0.0.0:9000
ARMOR_B2_REGION=us-east-005
ARMOR_B2_ACCESS_KEY_ID=...
ARMOR_B2_SECRET_ACCESS_KEY=...
ARMOR_CF_DOWNLOAD_DOMAIN=armor-b2.example.com
ARMOR_MEK=<hex-encoded 32-byte key>
```

### Kubernetes Deployment

ARMOR is designed to run as a sidecar or standalone pod:

```yaml
# Secret with B2 creds, MEK, and Cloudflare Worker token
apiVersion: v1
kind: Secret
metadata:
  name: armor-secrets
type: Opaque
stringData:
  b2-access-key-id: "..."
  b2-secret-access-key: "..."
  master-encryption-key: "..."
  cloudflare-worker-token: "..."
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
        - name: ARMOR_CF_WORKER_AUTH_TOKEN
          valueFrom:
            secretKeyRef:
              name: armor-secrets
              key: cloudflare-worker-token
```

---

## Cloudflare Worker

A minimal Cloudflare Worker sits between ARMOR's download client and B2. It:

1. Validates the `X-Armor-Token` header (shared secret)
2. Signs the request with B2 credentials (AWS SigV4 via `aws4fetch`)
3. Forwards to the B2 S3 endpoint
4. Strips `Authorization` from cache key so Cloudflare can cache responses
5. Sets `Cache-Control` headers for encrypted content (long TTL — ciphertext is immutable per-version)
6. Strips B2-specific response headers (`x-bz-*`)
7. Passes through `Range` request/response headers unchanged

The Worker holds a **read-only** B2 application key scoped to a single bucket. Even if the Worker token is compromised, an attacker gets read access to ciphertext only — useless without the MEK.

### Worker Code (Pseudocode)

```javascript
export default {
  async fetch(request, env) {
    // 1. Validate ARMOR token
    if (request.headers.get("X-Armor-Token") !== env.ARMOR_TOKEN) {
      return new Response("Unauthorized", { status: 401 });
    }

    // 2. Build B2 request (strip ARMOR auth, add B2 auth)
    const b2Url = new URL(request.url);
    b2Url.hostname = `s3.${env.B2_REGION}.backblazeb2.com`;

    const b2Request = new Request(b2Url, {
      method: request.method,
      headers: {
        "Range": request.headers.get("Range") || "",
      },
    });

    // 3. Sign with B2 credentials
    const signedRequest = await signAwsV4(b2Request, {
      accessKeyId: env.B2_ACCESS_KEY_ID,
      secretAccessKey: env.B2_SECRET_ACCESS_KEY,
      region: env.B2_REGION,
      service: "s3",
    });

    // 4. Fetch from B2 with caching
    const response = await fetch(signedRequest, {
      cf: {
        cacheEverything: true,
        cacheTtl: 86400, // 1 day (ciphertext doesn't change)
      },
    });

    // 5. Clean up response headers
    const cleanResponse = new Response(response.body, response);
    cleanResponse.headers.delete("x-bz-file-name");
    cleanResponse.headers.delete("x-bz-file-id");
    cleanResponse.headers.delete("x-bz-content-sha1");
    cleanResponse.headers.set("Cache-Control", "public, max-age=86400");

    return cleanResponse;
  },
};
```

### Cloudflare Setup Checklist

- [ ] Domain on Cloudflare (e.g., `example.com`)
- [ ] CNAME: `armor-b2.example.com` → Worker route (or use Workers custom domain)
- [ ] SSL mode: Full (strict)
- [ ] Disable Automatic Signed Exchanges (SXGs)
- [ ] Worker deployed with B2 read-only credentials and ARMOR token
- [ ] Cache rules: cache everything for the `armor-b2` subdomain

---

## Key Management

### MEK Lifecycle

```
armor key generate          → Generate 256-bit random MEK, write to mek_path
armor key export            → Print MEK in base64 for backup
armor key import <file>     → Replace MEK from backup file
armor key rotate            → Generate new MEK, re-wrap all DEKs via CopyObject
armor key verify            → Decrypt a test object to verify MEK is correct
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

This is an O(N) metadata operation — no data is re-uploaded. A 100,000-file bucket takes ~100K API calls. After May 2026, these are free. The rotation command is idempotent (safe to re-run if interrupted) — it tracks progress in a local state file.

### Multi-Key Support (v2)

For data classification (e.g., different keys for different prefixes):

```toml
[encryption.keys]
default = "/etc/armor/keys/default.key"
sensitive = "/etc/armor/keys/sensitive.key"

[encryption.key_routing]
"data/pii/*" = "sensitive"
"*" = "default"
```

The key ID is stored in `x-amz-meta-armor-key-id` so ARMOR knows which MEK to use for decryption.

---

## Language and Dependencies

### Language: Go

Go is the best fit for ARMOR:

- **HTTP server:** `net/http` stdlib is production-grade for S3 protocol handling
- **Concurrency:** Goroutines handle parallel range reads, concurrent client requests
- **Crypto:** `crypto/aes`, `crypto/cipher` (AES-CTR), `crypto/hmac` — all stdlib, hardware-accelerated via AES-NI
- **B2 SDK:** Official `github.com/Backblaze/blazer` + `github.com/aws/aws-sdk-go-v2` for S3
- **Single binary:** No runtime dependencies; trivial Docker image (`FROM scratch`)
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
| `golang.org/x/crypto/argon2` | MEK derivation from password |
| `github.com/pelletier/go-toml/v2` | Config file parsing |

### S3 Protocol Handling

Rather than implementing the full S3 XML protocol from scratch, use an existing S3 server framework:

- **Option A: `s3gw` / custom router** — Implement S3 XML request/response parsing for the ~15 operations we need. More control, less dependency.
- **Option B: MinIO's `cmd/gateway` pattern** — Fork MinIO's S3 handler and plug in ARMOR's backend. Heavy dependency but battle-tested S3 protocol compliance.
- **Recommended: Option A.** The S3 XML protocol for our operation subset is well-documented and manageable. DuckDB, boto3, and AWS CLI all use straightforward request patterns. A custom router avoids the weight of MinIO and gives full control over the encryption boundary.

---

## Implementation Phases

### Phase 1: Core (MVP)

**Goal:** DuckDB can `read_parquet('s3://bucket/path')` through ARMOR with full range-read support.

- [ ] S3 protocol handler: PutObject, GetObject (full + range), HeadObject, DeleteObject, ListObjectsV2
- [ ] AES-256-CTR encryption with per-block HMAC
- [ ] Envelope encryption (MEK wraps DEK per file)
- [ ] Encrypted object format (header + data blocks + HMAC table)
- [ ] Range read translation (plaintext offset → encrypted block offset)
- [ ] Dual backend: direct-to-B2 uploads, Cloudflare-routed downloads
- [ ] Config file + env var support
- [ ] `armor serve` command (starts the server)
- [ ] `armor key generate` command
- [ ] Cloudflare Worker (minimal: auth + sign + cache)
- [ ] Metadata cache (LRU, in-memory)
- [ ] Dockerfile + CI build

**Validation:** Point DuckDB at ARMOR, upload a Parquet file, run `SELECT` with predicates and column selection, verify only partial data is fetched.

### Phase 2: Production Hardening

**Goal:** Reliable for continuous use with operational tooling.

- [ ] Multipart upload (CreateMultipartUpload, UploadPart, CompleteMultipartUpload, AbortMultipartUpload)
- [ ] CopyObject (for rename and key rotation)
- [ ] `armor key rotate` command
- [ ] DeleteObjects (bulk delete)
- [ ] ListBuckets, CreateBucket, DeleteBucket, HeadBucket
- [ ] Graceful shutdown + in-flight request draining
- [ ] Structured logging (JSON)
- [ ] Prometheus metrics: request count, latency, bytes transferred, cache hit rate, encryption ops
- [ ] Health check endpoint (`/health`)
- [ ] Kubernetes manifests (Deployment, Service, Secret)
- [ ] Integration tests against real B2 + Cloudflare

### Phase 3: Advanced Features

**Goal:** Multi-user, multi-key, full S3 compatibility.

- [ ] Multi-key routing (different MEKs for different prefixes)
- [ ] Multiple auth credentials with per-key ACLs
- [ ] ListObjectVersions with per-version decryption
- [ ] Pre-signed URL proxy (ARMOR-signed URLs that trigger decrypt-on-fetch)
- [ ] Streaming encryption for very large uploads (>5 GB)
- [ ] Object Lock / retention passthrough
- [ ] Lifecycle rule passthrough
- [ ] Admin API: key management via B2 native API
- [ ] Web dashboard (optional): bucket browser, encryption status, cache stats

---

## Project Structure

```
ARMOR/
├── cmd/
│   └── armor/
│       └── main.go              # CLI entry point (serve, key generate/rotate/export)
├── internal/
│   ├── server/
│   │   ├── server.go            # HTTP server setup, middleware
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
│   │   ├── b2.go                # B2 S3 client (uploads, metadata, mutations)
│   │   ├── cloudflare.go        # Cloudflare download client (range reads)
│   │   └── cache.go             # Metadata LRU cache
│   └── config/
│       └── config.go            # TOML config + env var loading
├── worker/
│   ├── src/
│   │   └── index.js             # Cloudflare Worker source
│   └── wrangler.toml            # Worker deployment config
├── deploy/
│   ├── Dockerfile
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

- Start ARMOR against real B2 bucket + Cloudflare Worker
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
| Worker token compromise | Attacker gets read access to ciphertext only — useless without MEK |
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

## Decisions and Rationale

| Decision | Choice | Why |
|----------|--------|-----|
| Language | Go | Single binary, stdlib crypto (AES-NI), excellent HTTP server, S3 ecosystem |
| Encryption | AES-256-CTR | Random access decryption — the single most important property for range reads |
| Integrity | HMAC-SHA256 per block | Independent verification of any block without reading the full file |
| Key wrapping | AES-KWP (RFC 5649) | Standard, constant-time, no IV management for wrapping |
| Block size | 64 KB | Matches Parquet page sizes; 0.05% HMAC overhead; good range-read granularity |
| Upload path | Direct to B2 | Ingress is free; avoids Cloudflare body size limits and Worker CPU |
| Download path | Via Cloudflare | $0 egress via Bandwidth Alliance PNI; edge caching for repeated reads |
| Filenames | Plaintext (v1) | Enables DuckDB partition discovery; directory structure is not sensitive for this use case |
| Metadata storage | B2 S3 headers + envelope header | Headers enable fast HeadObject; envelope enables offline reading |
| Auth model | ARMOR-local SigV4 | B2 creds stay on server; standard S3 client compatibility |
