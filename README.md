# 🛡️ ARMOR

**Authenticated Range-readable Managed Object Repository**

---

## 🔤 What Does ARMOR Stand For?

| Letter | Word | Meaning |
|:------:|------|---------|
| **A** | Authenticated | Every operation is cryptographically verified — envelope encryption with per-file keys, per-block HMACs, and scoped access tokens |
| **R** | Range-readable | AES-CTR block-level encryption enables random-access decryption — tools like DuckDB can query encrypted Parquet files without downloading the whole thing |
| **M** | Managed | Transparent key lifecycle — automatic DEK generation, master key wrapping, and server-side key rotation via metadata-only copies |
| **O** | Object | Built on S3-compatible object storage (Backblaze B2) with full support for multipart uploads, versioning, and lifecycle rules |
| **R** | Repository | A unified encrypted data layer — upload, download, sync, query, and share files through a single interface |

---

## 🎯 What Is ARMOR?

ARMOR is an encrypted storage layer that wraps [Backblaze B2](https://www.backblaze.com/cloud-storage) and [Cloudflare's](https://www.cloudflare.com/) global edge network to provide:

- 🔐 **Zero-knowledge encryption** — data is encrypted before it leaves your machine
- 💸 **Zero egress fees** — all downloads route through Cloudflare via the Bandwidth Alliance
- 🔍 **Seekable encryption** — AES-CTR block-level scheme allows byte-range reads on encrypted files
- 🦆 **DuckDB integration** — query encrypted Parquet files with column pruning and predicate pushdown intact
- 🪄 **Transparent operation** — users see plaintext files; ARMOR handles all crypto invisibly

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
                                   AES-256-CTR, 16KB blocks
                                   per-block HMAC-SHA256
                                   seekable random access
```

---

## 📂 Repository Structure

```
ARMOR/
├── README.md
└── docs/
    ├── plan/           # Implementation plans
    └── research/       # Technical research
        ├── application-requirements.md
        ├── b2-pricing-and-features.md
        ├── bandwidth-alliance.md
        ├── cloudflare-architecture.md
        ├── duckdb-encrypted-parquet.md
        ├── s3-operation-surface.md
        └── sdks-and-encryption.md
```

---

## 🧰 Planned Interface

```bash
# 🔧 Setup
armor init                              # Initialize config, generate master key
armor configure                         # Set B2 credentials, bucket, Cloudflare domain

# 📤 Upload (direct to B2)
armor upload <file>                     # Encrypt + upload
armor upload <dir> --recursive          # Encrypt + upload directory tree

# 📥 Download (through Cloudflare)
armor download <remote> <local>         # Download + decrypt

# 📋 Management
armor ls [prefix]                       # List files
armor rm <remote>                       # Delete
armor sync <remote> <local>             # Incremental encrypted sync

# 🔑 Key management
armor key generate                      # Generate new master key
armor key rotate                        # Re-wrap all DEKs (no re-upload)
armor key export                        # Backup master key
```

---

## 📚 Research

Detailed technical research is in [`docs/research/`](docs/research/):

| 📄 Document | 📝 Coverage |
|------------|------------|
| [B2 Pricing & Features](docs/research/b2-pricing-and-features.md) | 💰 Pricing, S3 API, encryption options, auth model |
| [Bandwidth Alliance](docs/research/bandwidth-alliance.md) | 🌐 Zero-egress partners, Cloudflare integration, setup steps |
| [SDKs & Encryption](docs/research/sdks-and-encryption.md) | 🧰 b2sdk, boto3, client-side encryption patterns |
| [Cloudflare Architecture](docs/research/cloudflare-architecture.md) | ☁️ Workers, caching, security, upload/download paths |
| [DuckDB + Encrypted Parquet](docs/research/duckdb-encrypted-parquet.md) | 🦆 AES-CTR range reads, PME, seekable decryption |
| [S3 Operation Surface](docs/research/s3-operation-surface.md) | 🗺️ Full operation map for transparent encryption layer |
| [Application Requirements](docs/research/application-requirements.md) | 📋 Architecture, API design, infrastructure checklist |

---

## ⚖️ License

TBD
