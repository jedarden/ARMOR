# 🛡️ ARMOR

**Authenticated Range-readable Managed Object Repository**

---

## 🔤 What Does ARMOR Stand For?

| Letter | Word | Meaning |
|:------:|------|---------|
| **A** | Authenticated | Every operation is cryptographically verified — envelope encryption with per-file keys, per-block HMACs, and scoped access tokens |
| **R** | Range-readable | AES-CTR block-level encryption enables random-access decryption — tools like DuckDB can query encrypted Parquet files without downloading the whole thing |
| **M** | Managed | Transparent key lifecycle — automatic DEK generation, master key wrapping, and server-side key rotation via metadata-only copies |
| **O** | Object | Built on S3-compatible object storage (Backblaze B2) with full support for multipart uploads, lifecycle rules, and object lock |
| **R** | Repository | A unified encrypted data layer — upload, download, sync, query, and share files through a single interface |

---

## 🎯 What Is ARMOR?

ARMOR is an **S3-compatible proxy server** that transparently encrypts and decrypts data between clients and [Backblaze B2](https://www.backblaze.com/cloud-storage). It leverages [Cloudflare's](https://www.cloudflare.com/) global edge network for zero-egress downloads.

- 🔐 **Zero-knowledge encryption** — data is encrypted before it leaves ARMOR; B2 only stores ciphertext
- 💸 **Zero egress fees** — all downloads route through Cloudflare via the Bandwidth Alliance
- 🔍 **Seekable encryption** — AES-256-CTR with 64KB blocks enables byte-range reads on encrypted files
- 🦆 **DuckDB integration** — query encrypted Parquet files with column pruning and predicate pushdown intact
- 🪄 **Transparent operation** — any S3-compatible tool (boto3, AWS CLI, DuckDB, rclone) works unmodified
- 🔑 **Multi-key support** — different MEKs for different prefixes with automatic key routing

---

## 💰 Cost Model

| Component | Cost |
|-----------|------|
| 💾 Storage | ~$6–7/TB/month |
| 📤 Egress (via Cloudflare) | $0 |
| 📡 API calls (after May 2026) | $0 |
| 🌐 Cloudflare (free plan) | $0 |
| **Total** | **~$6–7/TB/month** |

---

## 🏗️ Architecture

### 📤 Upload Path (direct to B2 — ingress is free)

```
┌──────────┐     ┌──────────────┐     ┌──────────┐
│  Client   │────▶│    ARMOR     │────▶│    B2    │
│           │     │  encrypt +   │     │          │
│           │     │  upload      │     │          │
└──────────┘     └──────────────┘     └──────────┘
```

### 📥 Download Path (through Cloudflare — egress is free)

```
┌──────────┐     ┌────────────┐     ┌────────────┐     ┌──────────┐
│  Client   │◀───│ Cloudflare │◀───│  Cloudflare │◀───│    B2    │
│  ARMOR    │    │   Edge     │    │  PNI Link   │    │          │
│  decrypt  │    │  (cache)   │    │  (free)     │    │          │
└──────────┘     └────────────┘     └────────────┘     └──────────┘
```

### 🦆 DuckDB Query Path (seekable decryption)

```
DuckDB                          ARMOR FS                    Cloudflare → B2
  │                                │                              │
  ├─ read footer (last 8 bytes) ──▶├─ Range GET (encrypted) ────▶│
  │◀── decrypted footer ──────────┤◀── ciphertext ──────────────┤
  │                                │                              │
  ├─ read col_a, row group 3 ────▶├─ Range GET (3 blocks) ─────▶│
  │◀── decrypted column chunk ────┤◀── ciphertext ──────────────┤
  │                                │                              │
  └─ result set                    └                              └
```

---

## 🔐 Security Model

| 🛡️ Threat | ✅ Mitigation |
|-----------|-------------|
| B2 breach | Client-side encryption — B2 only stores opaque blobs |
| Cloudflare inspection | Client-side encryption — Cloudflare only caches opaque blobs |
| Man-in-the-middle | TLS everywhere + client-side encryption |
| Key compromise | Envelope encryption — per-file DEKs limit blast radius; key rotation re-wraps without re-uploading |
| Data corruption | SHA-256 integrity hash + per-block HMACs |
| Unauthorized access | Private bucket + Cloudflare Worker auth + scoped application keys |

---

## 🔑 Encryption Design

```
🔑 Master Key (MEK)
 │  stored locally, never uploaded
 │
 └─▶ wraps ──▶ 🔑 Data Encryption Key (DEK)
                │  random per-file, wrapped copy in B2 metadata
                │
                └─▶ encrypts ──▶ 📦 File Data
                                   AES-256-CTR, 64KB blocks
                                   per-block HMAC-SHA256
                                   seekable random access
```

---

## 🚀 Quick Start

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

Point any S3-compatible tool at ARMOR:

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

---

## ⚙️ Configuration

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
| `ARMOR_BLOCK_SIZE` | No | `65536` | Encryption block size |
| `ARMOR_WRITER_ID` | No | (hostname) | Provenance chain writer ID |
| `ARMOR_READYZ_CACHE_TTL` | No | `30` | Seconds to cache backend connectivity check in `/readyz` (only used when canary is disabled) |
| `ARMOR_DASHBOARD_USER` | No | (empty) | Dashboard HTTP Basic Auth username |
| `ARMOR_DASHBOARD_PASS` | No | (empty) | Dashboard HTTP Basic Auth password |
| `ARMOR_DASHBOARD_TOKEN` | No | (empty) | Dashboard Bearer token for authentication |

### Multi-Key Configuration

```bash
# Default key
ARMOR_MEK=<hex>

# Named keys for different prefixes
ARMOR_MEK_SENSITIVE=<hex>
ARMOR_MEK_ARCHIVE=<hex>

# Route prefixes to keys
ARMOR_KEY_ROUTES="data/pii/*=sensitive,archive/*=archive,*=default"
```

### Multi-Credential Configuration

```bash
# Multiple auth credentials with ACLs
ARMOR_AUTH_READONLY_ACCESS_KEY=reader-key
ARMOR_AUTH_READONLY_SECRET_KEY=reader-secret
ARMOR_AUTH_READONLY_ACL="mybucket:readonly/*"

ARMOR_AUTH_WRITER_ACCESS_KEY=writer-key
ARMOR_AUTH_WRITER_SECRET_KEY=writer-secret
ARMOR_AUTH_WRITER_ACL="mybucket:*,otherbucket:uploads/*"
```

---

## 📂 Repository Structure

```
ARMOR/
├── README.md
├── Dockerfile
├── go.mod
├── go.sum
├── cmd/armor/main.go          # Entrypoint
├── internal/
│   ├── server/                # S3 server, handlers, auth
│   ├── dashboard/             # Web dashboard UI and metrics
│   ├── crypto/                # Encryption, decryption, envelope
│   ├── backend/               # B2 S3 client, Cloudflare downloads
│   ├── canary/                # Self-healing integrity monitor
│   ├── config/                # Configuration loading
│   ├── keymanager/            # Multi-key routing
│   ├── presign/               # Pre-signed URL sharing
│   ├── provenance/            # Cryptographic audit chain
│   ├── logging/               # Structured JSON logging
│   └── metrics/               # Prometheus metrics
├── deploy/kubernetes/         # Kubernetes manifests
├── tests/integration/         # Integration tests
└── docs/
    ├── dashboard.md           # Web dashboard documentation
    ├── plan/                  # Implementation plan
    └── research/              # Technical research
```

---

## 🎨 Web Dashboard

ARMOR includes a web dashboard for bucket browsing, encryption status visualization, and cache statistics. The dashboard is accessible on the admin port (default `127.0.0.1:9001`):

```bash
# Local access
open http://localhost:9001/dashboard

# Kubernetes port-forward
kubectl port-forward svc/armor 9001:9001
open http://localhost:9001/dashboard
```

**Features:**
- 📂 **Bucket browsing** - Navigate encrypted buckets with prefix-based navigation
- 🔐 **Encryption status** - Visual badges showing which objects use ARMOR encryption and which keys
- 📊 **Cache statistics** - Monitor metadata cache hit rates and effectiveness
- 📈 **System metrics** - Real-time metrics for requests, bytes transferred, uptime, and canary status
- 🔍 **Object details** - Detailed metadata view for individual objects

For full documentation, see [docs/dashboard.md](docs/dashboard.md).

## 🔧 Admin API

Key management endpoints on the admin listener (default `127.0.0.1:9001`):

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/healthz` | GET | Health check (Kubernetes liveness) |
| `/readyz` | GET | Readiness check (Kubernetes readiness) |
| `/metrics` | GET | Prometheus metrics |
| `/admin/key/verify` | GET | Verify MEK can decrypt canary |
| `/admin/key/rotate` | POST | Rotate MEK (re-wrap all DEKs) |
| `/admin/key/export` | GET | Export current MEK (`?confirm=yes`) |
| `/admin/audit` | GET | Walk provenance chains, verify integrity |
| `/admin/presign` | POST | Generate pre-signed share URL |
| `/armor/canary` | GET | Canary integrity status |
| `/dashboard` | GET | Web dashboard UI |
| `/dashboard/object` | GET | Object details (JSON) |
| `/dashboard/metrics` | GET | Dashboard metrics (JSON) |

---

## 📋 S3 API Coverage

### Transforming Operations (encryption/decryption)

| Operation | Support |
|-----------|---------|
| PutObject | ✅ Full (with streaming for large files) |
| GetObject | ✅ Full (with range reads) |
| HeadObject | ✅ Full (plaintext size, conditionals) |
| CopyObject | ✅ Full (DEK re-wrapping, cross-bucket) |
| CreateMultipartUpload | ✅ Full |
| UploadPart | ✅ Full |
| CompleteMultipartUpload | ✅ Full |
| AbortMultipartUpload | ✅ Full |
| ListParts | ✅ Full |
| ListMultipartUploads | ✅ Full |

### Passthrough Operations

| Operation | Support |
|-----------|---------|
| ListObjectsV2 | ✅ Full (size correction, .armor/ filter) |
| DeleteObject | ✅ Full |
| DeleteObjects | ✅ Full |
| ListBuckets | ✅ Full |
| CreateBucket | ✅ Full |
| DeleteBucket | ✅ Full |
| HeadBucket | ✅ Full |
| GetBucketLifecycleConfiguration | ✅ Full |
| PutBucketLifecycleConfiguration | ✅ Full |
| DeleteBucketLifecycleConfiguration | ✅ Full |
| GetObjectLockConfiguration | ✅ Full |
| PutObjectLockConfiguration | ✅ Full |
| GetObjectRetention | ✅ Full |
| PutObjectRetention | ✅ Full |
| GetObjectLegalHold | ✅ Full |
| PutObjectLegalHold | ✅ Full |

---

## 📚 Documentation

- **[Web Dashboard](docs/dashboard.md)** — Bucket browsing, encryption status, and cache statistics
- **[Implementation Plan](docs/plan/plan.md)** — Full architecture and implementation details
- **[Integration Tests](tests/integration/README.md)** — Testing against real B2 + Cloudflare
- **[Research](docs/research/)** — Technical research on B2, Cloudflare, encryption, and DuckDB

---

## ⚖️ License

MIT
