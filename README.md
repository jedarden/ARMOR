# ARMOR

**Authenticated Range-readable Managed Object Repository**

ARMOR is an S3-compatible proxy server that transparently encrypts data before storing it in [Backblaze B2](https://www.backblaze.com/cloud-storage) and serves downloads through Cloudflare for zero-egress cost. Any S3-compatible client — boto3, AWS CLI, DuckDB, rclone — works without modification.

- **Zero-knowledge encryption** — data is encrypted before it leaves ARMOR; B2 only ever stores ciphertext
- **Zero egress fees** — downloads route through Cloudflare via the Bandwidth Alliance
- **Seekable encryption** — AES-256-CTR with 64KB blocks enables byte-range reads without decrypting the whole file
- **DuckDB-compatible** — query encrypted Parquet files with column pruning and predicate pushdown intact
- **Multi-key routing** — different master keys for different path prefixes; automatic key selection per object

## Quick Start

### Docker

```bash
docker run -d \
  -p 9000:9000 \
  -p 9001:9001 \
  -e ARMOR_B2_REGION=us-east-005 \
  -e ARMOR_B2_ACCESS_KEY_ID=your-key-id \
  -e ARMOR_B2_SECRET_ACCESS_KEY=your-key-secret \
  -e ARMOR_BUCKET=your-bucket \
  -e ARMOR_CF_DOMAIN=your-cf-domain.example.com \
  -e ARMOR_MEK=$(openssl rand -hex 32) \
  -e ARMOR_AUTH_ACCESS_KEY=my-access-key \
  -e ARMOR_AUTH_SECRET_KEY=my-secret-key \
  ghcr.io/jedarden/armor:latest
```

### Client Configuration

Point any S3-compatible tool at ARMOR's listen address:

```bash
# AWS CLI
aws --endpoint-url http://localhost:9000 s3 cp file.txt s3://bucket/key

# boto3 (Python)
import boto3
s3 = boto3.client('s3',
    endpoint_url='http://localhost:9000',
    aws_access_key_id='my-access-key',
    aws_secret_access_key='my-secret-key')
s3.upload_file('local.txt', 'bucket', 'key')

# DuckDB
INSTALL httpfs;
LOAD httpfs;
SET s3_endpoint='localhost:9000';
SET s3_access_key_id='my-access-key';
SET s3_secret_access_key='my-secret-key';
SELECT * FROM read_parquet('s3://bucket/data.parquet');
```

## Cost Model

| Component | Cost |
|-----------|------|
| Storage | ~$6–7/TB/month |
| Egress (via Cloudflare Bandwidth Alliance) | $0 |
| B2 API calls | $0 |
| Cloudflare (free plan) | $0 |
| **Total** | **~$6–7/TB/month** |

## Architecture

### Upload Path (direct to B2 — ingress is free)

```
┌──────────┐     ┌──────────────┐     ┌──────────┐
│  Client   │────▶│    ARMOR     │────▶│    B2    │
│           │     │  encrypt +   │     │          │
│           │     │  upload      │     │          │
└──────────┘     └──────────────┘     └──────────┘
```

### Download Path (through Cloudflare — egress is free)

```
┌──────────┐     ┌────────────┐     ┌────────────┐     ┌──────────┐
│  Client   │◀───│ Cloudflare │◀───│  Cloudflare │◀───│    B2    │
│  ARMOR    │    │   Edge     │    │  PNI Link   │    │          │
│  decrypt  │    │  (cache)   │    │  (free)     │    │          │
└──────────┘     └────────────┘     └────────────┘     └──────────┘
```

### DuckDB Query Path (seekable decryption)

DuckDB issues byte-range GET requests for specific row groups and columns. ARMOR decrypts only the requested 64KB blocks, so column pruning and predicate pushdown remain effective:

```
DuckDB                          ARMOR                       Cloudflare → B2
  │                                │                              │
  ├─ read footer (last 8 bytes) ──▶├─ Range GET (encrypted) ────▶│
  │◀── decrypted footer ──────────┤◀── ciphertext ──────────────┤
  │                                │                              │
  ├─ read col_a, row group 3 ────▶├─ Range GET (3 blocks) ─────▶│
  │◀── decrypted column chunk ────┤◀── ciphertext ──────────────┤
  │                                │                              │
  └─ result set                    └                              └
```

## Encryption Design

```
Master Key (MEK)
 │  stored locally, never uploaded
 │
 └─▶ wraps ──▶ Data Encryption Key (DEK)
                │  random per-file, wrapped copy in B2 metadata
                │
                └─▶ encrypts ──▶ File Data
                                   AES-256-CTR, 64KB blocks
                                   per-block HMAC-SHA256
                                   seekable random access
```

Key rotation re-wraps DEKs without re-uploading file data — a metadata-only operation.

## Security Model

| Threat | Mitigation |
|--------|-----------|
| B2 breach | Client-side encryption — B2 only stores opaque blobs |
| Cloudflare inspection | Client-side encryption — Cloudflare only caches opaque blobs |
| Man-in-the-middle | TLS everywhere + client-side encryption |
| Key compromise | Envelope encryption — per-file DEKs limit blast radius; rotation re-wraps without re-uploading |
| Data corruption | SHA-256 integrity hash + per-block HMACs |
| Unauthorized access | Private bucket + Cloudflare Worker auth + scoped application keys |

## Configuration

ARMOR is configured via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_LISTEN` | No | `0.0.0.0:9000` | S3 API listen address |
| `ARMOR_ADMIN_LISTEN` | No | `127.0.0.1:9001` | Admin API (key rotation, canary, audit) |
| `ARMOR_B2_REGION` | Yes | — | B2 region (e.g., `us-east-005`) |
| `ARMOR_B2_ACCESS_KEY_ID` | Yes | — | B2 application key ID |
| `ARMOR_B2_SECRET_ACCESS_KEY` | Yes | — | B2 application key |
| `ARMOR_BUCKET` | Yes | — | B2 bucket name |
| `ARMOR_CF_DOMAIN` | Yes | — | Cloudflare domain CNAME'd to B2 |
| `ARMOR_MEK` | Yes | — | Master encryption key (hex, 32 bytes) |
| `ARMOR_AUTH_ACCESS_KEY` | No | (random) | Client access key |
| `ARMOR_AUTH_SECRET_KEY` | No | (random) | Client secret key |
| `ARMOR_BLOCK_SIZE` | No | `65536` | Encryption block size (bytes) |
| `ARMOR_WRITER_ID` | No | (hostname) | Provenance chain writer ID |
| `ARMOR_READYZ_CACHE_TTL` | No | `30` | Seconds to cache backend connectivity in `/readyz` |
| `ARMOR_DASHBOARD_USER` | No | — | Dashboard HTTP Basic Auth username |
| `ARMOR_DASHBOARD_PASS` | No | — | Dashboard HTTP Basic Auth password |
| `ARMOR_DASHBOARD_TOKEN` | No | — | Dashboard Bearer token |

### Multi-Key Routing

Route different path prefixes to different master keys:

```bash
ARMOR_MEK=<hex>                           # default key
ARMOR_MEK_SENSITIVE=<hex>                 # named key
ARMOR_MEK_ARCHIVE=<hex>                   # named key
ARMOR_KEY_ROUTES="data/pii/*=sensitive,archive/*=archive,*=default"
```

### Multi-Credential Configuration

```bash
ARMOR_AUTH_READONLY_ACCESS_KEY=reader-key
ARMOR_AUTH_READONLY_SECRET_KEY=reader-secret
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*"

ARMOR_AUTH_WRITER_ACCESS_KEY=writer-key
ARMOR_AUTH_WRITER_SECRET_KEY=writer-secret
ARMOR_AUTH_WRITER_ACL="mybucket:*,otherbucket:uploads/*"
```

## S3 API Coverage

### Transforming Operations (encryption/decryption applied)

| Operation | Support |
|-----------|---------|
| PutObject | Full (streaming for large files) |
| GetObject | Full (range reads) |
| HeadObject | Full (plaintext size, conditionals) |
| CopyObject | Full (DEK re-wrapping, cross-bucket) |
| CreateMultipartUpload | Full |
| UploadPart | Full |
| CompleteMultipartUpload | Full |
| AbortMultipartUpload | Full |
| ListParts | Full |
| ListMultipartUploads | Full |

### Passthrough Operations

| Operation | Support |
|-----------|---------|
| ListObjectsV2 | Full (size correction, `.armor/` filter) |
| DeleteObject | Full |
| DeleteObjects | Full |
| ListBuckets | Full |
| CreateBucket / DeleteBucket / HeadBucket | Full |
| Lifecycle configuration | Full |
| Object Lock / Retention / Legal Hold | Full |

## Web Dashboard

A web dashboard for bucket browsing, encryption status, and metrics is available on the admin port (default `127.0.0.1:9001`):

```bash
open http://localhost:9001/dashboard
```

Features:
- Bucket browsing with prefix-based navigation
- Encryption status badges per object (key name, ARMOR vs. unencrypted)
- Metadata cache hit rates
- Real-time metrics: requests, bytes transferred, uptime, canary status

See [docs/dashboard.md](docs/dashboard.md) for full documentation.

## Admin API

Key management and monitoring endpoints on the admin listener (`127.0.0.1:9001`):

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/healthz` | GET | Liveness check |
| `/readyz` | GET | Readiness check (verifies B2 connectivity) |
| `/metrics` | GET | Prometheus metrics |
| `/admin/key/verify` | GET | Verify MEK can decrypt the canary object |
| `/admin/key/rotate` | POST | Rotate MEK — re-wraps all DEKs, no file re-upload |
| `/admin/key/export` | GET | Export current MEK (`?confirm=yes`) |
| `/admin/audit` | GET | Walk provenance chains, verify integrity |
| `/admin/presign` | POST | Generate pre-signed share URL |
| `/armor/canary` | GET | Canary integrity status |
| `/dashboard` | GET | Web dashboard |

## Repository Structure

```
ARMOR/
├── cmd/armor/main.go          # Entrypoint
├── internal/
│   ├── server/                # S3 API handlers, auth
│   ├── crypto/                # Encryption, decryption, envelope key management
│   ├── backend/               # B2 S3 client, Cloudflare download routing
│   ├── canary/                # Self-healing integrity monitor
│   ├── config/                # Configuration loading (env vars)
│   ├── keymanager/            # Multi-key routing
│   ├── dashboard/             # Web dashboard UI and metrics
│   ├── presign/               # Pre-signed URL generation
│   ├── provenance/            # Cryptographic audit chain
│   ├── logging/               # Structured JSON logging
│   └── metrics/               # Prometheus metrics
├── deploy/kubernetes/         # Kubernetes manifests
├── tests/integration/         # Integration tests (requires real B2 + Cloudflare)
└── docs/
    ├── dashboard.md
    ├── cloudflare-setup.md    # DNS configuration for zero-egress downloads
    └── research/
```

## Documentation

- [Web Dashboard](docs/dashboard.md) — Bucket browsing, encryption status, cache statistics
- [Cloudflare Setup](docs/cloudflare-setup.md) — DNS configuration for zero-egress B2 downloads
- [Integration Tests](tests/integration/README.md) — Testing against real B2 + Cloudflare

## Disaster Recovery / Offline Decryption

ARMOR includes a standalone CLI tool `armor-decrypt` for recovering encrypted objects without a running ARMOR server. This enables disaster recovery scenarios where you have:

- The Master Encryption Key (MEK)
- Access to B2 (or a local copy of an encrypted object)

### Building the Decrypt Tool

```bash
go build -o armor-decrypt ./cmd/armor-decrypt
```

### Usage

#### Decrypt from B2

```bash
# Decrypt directly from B2 (requires B2 credentials)
armor-decrypt \
  -mek 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef \
  -input b2://my-bucket/path/to/file.encrypted \
  -output recovered-file.txt

# Using MEK from environment
export ARMOR_MEK=0123456789abcdef...
armor-decrypt -input b2://my-bucket/file -output recovered.txt

# With verbose output
armor-decrypt -mek HEX -input b2://bucket/file -v - output recovered.txt
```

#### Decrypt from Local File

For local files, you need the wrapped DEK (from `x-amz-meta-armor-wrapped-dek` metadata):

```bash
armor-decrypt \
  -mek 0123456789abcdef... \
  -input /path/to/encrypted.bin \
  -wrapped-dek WWF...base64... \
  -output plaintext.bin
```

### Key Requirements

- **MEK (Master Encryption Key)**: 32-byte hex string
- **For B2**: `ARMOR_B2_REGION`, `ARMOR_B2_ENDPOINT`, `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`
- **For local files**: Wrapped DEK (base64, from object metadata)

### Multi-Key Support

If your ARMOR deployment uses named keys (via `ARMOR_KEY_ROUTES`), specify the key ID:

```bash
armor-decrypt \
  -mek <hex-for-specific-key> \
  -input b2://bucket/file \
  -key-id sensitive \
  -output recovered.txt
```

The key ID comes from the `x-amz-meta-armor-key-id` metadata header.

### Verification

The decrypt tool automatically:

- Verifies per-block HMAC-SHA256 integrity
- Validates the plaintext SHA-256 checksum
- Detects corrupted blocks or wrong MEK

Exit codes:
- `0`: Success
- `1`: Decryption failed (wrong MEK, corrupted data, HMAC mismatch)

### Example Workflow

```bash
# 1. List objects to find the target
aws s3 ls --endpoint-url http://localhost:9000 s3://bucket/

# 2. Get metadata to see key requirements
aws s3api head-object --endpoint-url http://localhost:9000 \
  --bucket bucket --key file

# 3. Decrypt with the correct MEK
armor-decrypt -mek $ARMOR_MEK -input b2://bucket/file -output recovered

# 4. Verify the recovered file
sha256sum recovered  # Should match x-amz-meta-armor-plaintext-sha256
```

## License

MIT
