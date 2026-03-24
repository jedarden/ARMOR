# b2-layer Application Requirements

## Goal

Build an application that:
1. **Uploads** data to Backblaze B2 with client-side encryption
2. **Downloads** data through B2→Cloudflare egress (zero egress fees)
3. **Decrypts** data locally for use
4. **Wraps** both the B2 S3-compatible API and optionally the native B2 API

## Cost Model

| Component | Cost |
|-----------|------|
| Storage | ~$6/TB/month (→ $6.95 after May 2026) |
| Egress (via Cloudflare) | $0 |
| API calls (after May 2026) | $0 |
| Cloudflare (free plan) | $0 |
| **Total** | **~$6-7/TB/month** |

---

## Core Architecture

```
UPLOAD PATH (direct to B2, no Cloudflare):
┌──────────────┐      ┌─────────────────┐      ┌──────────┐
│  b2-layer    │      │  b2-layer       │      │          │
│  CLI/client  │─────▶│  encrypt + pack │─────▶│   B2     │
│              │      │  (local)        │      │          │
└──────────────┘      └─────────────────┘      └──────────┘

DOWNLOAD PATH (through Cloudflare, zero egress):
┌──────────────┐      ┌──────────────────┐      ┌──────────┐      ┌──────────┐
│  b2-layer    │      │  Cloudflare      │      │ CF       │      │          │
│  CLI/client  │◀─────│  Edge/Worker     │◀─────│ cache or │◀─────│   B2     │
│  decrypt     │      │                  │      │ PNI      │      │          │
└──────────────┘      └──────────────────┘      └──────────┘      └──────────┘
```

---

## Required Components

### 1. Encryption Layer
- **Client-side encryption** before any data leaves the machine
- Envelope encryption: random per-file DEK wrapped by master key
- Algorithm: AES-256-GCM (widely supported, hardware-accelerated) or ChaCha20-Poly1305
- Large file support: chunked AEAD with chunk indexing
- Key derivation from password: Argon2id
- Encrypted metadata stored alongside ciphertext (as B2 file info or prepended header)

### 2. B2 Interface Layer (Dual API)
- **Primary: S3-compatible API via boto3** — uploads, downloads, listing, pre-signed URLs
- **Secondary: B2 native API via b2sdk** — application key management, download authorization tokens
- Abstraction that can use either backend transparently
- Multipart upload support for large files (auto-select at configurable threshold)

### 3. Cloudflare Download Proxy
- **Cloudflare Worker** that:
  - Holds B2 read credentials (application key scoped to read-only, single bucket)
  - Validates client authentication (HMAC signed URLs or token)
  - Proxies requests to private B2 bucket
  - Sets appropriate cache headers
  - Strips B2-specific response headers
- Custom domain CNAME to Cloudflare
- Cache-Control configuration for optimal hit rates

### 4. Upload Manager
- Direct uploads to B2 (bypassing Cloudflare — ingress is free)
- Pre-signed URL generation for browser/remote uploads
- Automatic multipart for large files
- Progress tracking and resumable uploads
- Integrity verification (SHA-256 of plaintext stored in encrypted metadata)

### 5. Key Management
- Master key storage (local keyring, file, or hardware token)
- Key rotation support (re-wrap DEKs with new master key without re-encrypting data)
- Multiple key support (different keys for different data classifications)
- Key export/backup mechanism

### 6. File Metadata
Each encrypted file needs associated metadata:
- Original filename
- Original file size
- Content type
- Encryption algorithm and parameters
- Wrapped DEK
- Chunk size (for large files)
- Chunk count
- Plaintext SHA-256 hash
- Timestamp

Store as: prepended binary header on the encrypted blob, or B2 file info metadata, or sidecar `.meta` file.

---

## API Surface (Proposed)

### CLI Interface
```bash
# Setup
b2-layer init                          # Initialize config, generate/import master key
b2-layer configure                     # Set B2 credentials, bucket, Cloudflare domain

# Upload (direct to B2)
b2-layer upload <file>                 # Encrypt + upload single file
b2-layer upload <directory> --recursive # Encrypt + upload directory tree
b2-layer upload --stdin --name <name>  # Encrypt + upload from stdin

# Download (through Cloudflare)
b2-layer download <remote-path> <local-path>  # Download + decrypt
b2-layer download <remote-path> --stdout       # Download + decrypt to stdout

# Management
b2-layer ls [prefix]                   # List files (shows original names)
b2-layer rm <remote-path>              # Delete file from B2
b2-layer info <remote-path>            # Show file metadata
b2-layer verify <remote-path>          # Verify integrity without downloading

# Key management
b2-layer key generate                  # Generate new master key
b2-layer key export                    # Export master key for backup
b2-layer key import <file>             # Import master key
b2-layer key rotate                    # Re-wrap all DEKs with new master key
```

### Library Interface (Python)
```python
from b2_layer import B2Layer

layer = B2Layer(config_path="~/.b2-layer/config.toml")

# Upload
layer.upload("local/file.txt", "remote/file.txt")

# Download (through Cloudflare)
layer.download("remote/file.txt", "local/file.txt")

# List
for entry in layer.ls("remote/prefix/"):
    print(entry.name, entry.size, entry.uploaded_at)

# Pre-signed upload URL (for external clients)
url = layer.presigned_upload("remote/destination.txt", expires_in=3600)

# Pre-signed download URL (through Cloudflare)
url = layer.presigned_download("remote/file.txt", expires_in=3600)
```

---

## Infrastructure Requirements

### B2 Setup
- [ ] B2 account
- [ ] Private bucket (US-East or US-West depending on latency needs)
- [ ] Application keys: one read-only (for Cloudflare Worker), one read-write (for uploads)

### Cloudflare Setup
- [ ] Domain on Cloudflare (can use existing)
- [ ] CNAME record pointing to B2 bucket hostname
- [ ] SSL set to Full (strict)
- [ ] Worker deployed with read-only B2 credentials
- [ ] Cache rules configured

### Local Setup
- [ ] Master key generated and securely stored
- [ ] Config file with B2 credentials, bucket name, Cloudflare domain

---

## Security Model

| Threat | Mitigation |
|--------|-----------|
| B2 breach | Client-side encryption — B2 only stores opaque blobs |
| Cloudflare inspection | Client-side encryption — CF only caches opaque blobs |
| Man-in-middle | TLS everywhere + client-side encryption |
| Key compromise | Key rotation re-wraps DEKs; per-file DEKs limit blast radius |
| Unauthorized access | Private bucket + Worker auth + application key scoping |
| Data corruption | SHA-256 integrity hash of plaintext stored in metadata |
| Replay/reorder attacks | Chunk indexing in AEAD associated data |

---

## Language Decision

**Recommended: Python** for initial implementation.
- b2sdk is Python-native with full feature coverage
- boto3 for S3-compatible access
- `cryptography` library for AES-256-GCM
- Rich CLI framework (click/typer)
- Fastest path to working prototype

**Future consideration: Rust or Go** for performance-critical paths.
- Go: `blazer` SDK is official and mature
- Rust: community crates exist; best for CLI distribution (single binary)

---

## Open Questions

1. **Metadata storage strategy:** Prepended header vs B2 file info vs sidecar file? Prepended header is most portable; B2 file info is limited to 10 key-value pairs of 50 bytes each; sidecar doubles the file count.

2. **Filename encryption:** Should remote filenames reveal anything about the original? rclone crypt encrypts filenames with EME/AES-256. We could use deterministic encryption (enables exact-match lookup) or randomized (no information leakage but requires metadata index).

3. **Chunk size for large files:** 1 MB (more granular resume), 5 MB (matches B2 minimum part size), or larger? Tradeoff between resume granularity and overhead.

4. **Config storage:** `~/.b2-layer/config.toml` or XDG config directory? Master key in system keyring (keyctl/macOS Keychain) or encrypted file?

5. **Cloudflare Worker deployment:** Wrangler CLI, or manual dashboard, or Terraform? Should the Worker code live in this repo?
