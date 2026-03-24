# B2 SDKs and Client-Side Encryption Patterns

## B2 Python SDK (b2sdk)

**Version:** 2.10.4 (2026-03-02), Python >=3.9, MIT license.

```python
from b2sdk.v3 import InMemoryAccountInfo, B2Api

info = InMemoryAccountInfo()
b2_api = B2Api(info)
b2_api.authorize_account("production", app_key_id, app_key)
```

### Upload APIs
| Method | Use Case |
|--------|----------|
| `upload_local_file()` | File from disk |
| `upload_bytes()` | In-memory bytes |
| `upload()` | Custom source |
| `upload_unbound_stream()` | Unknown-length streams |

All accept optional `encryption` parameter for SSE.

### Download APIs
| Method | Use Case |
|--------|----------|
| `download_file_by_name()` | Download by bucket + path |
| `download_file_by_id()` | Download by B2 file ID |

Both return `DownloadedFile` with `.save_to()`. Accept `encryption` for SSE-C and `range_` for partial reads.

### Large File Support
- SDK auto-selects multipart for large files via transfer planner
- B2 minimum part size: 5 MB, recommended: 100 MB, max: 5 GB
- Fine-grained control: `list_unfinished_large_files()`, `cancel_large_file()`, `list_parts()`, `concatenate()`, `concatenate_stream()`

### SDK Encryption (Server-Side Only)
```python
from b2sdk.v3 import EncryptionSetting, EncryptionMode, EncryptionKey

# SSE-B2
enc = EncryptionSetting(mode=EncryptionMode.SSE_B2)

# SSE-C
key = EncryptionKey(secret=b'32-byte-key-here', key_id='my-key-id')
enc = EncryptionSetting(mode=EncryptionMode.SSE_C, key=key)
```
Key IDs stored in file metadata under `sse_c_key_id`. **No built-in client-side encryption.**

---

## B2 CLI Tool

Built on b2sdk v3. Commands: `b2 file upload/download/copy`, `b2 sync`, `b2 ls`, `b2 rm`, bucket management. Credentials cached in SQLite; `B2_*` env vars avoid disk persistence.

---

## S3-Compatible Access via boto3

```python
import boto3
from botocore.config import Config

b2_s3 = boto3.client(
    's3',
    endpoint_url='https://s3.us-west-004.backblazeb2.com',
    aws_access_key_id=app_key_id,
    aws_secret_access_key=app_key,
    config=Config(signature_version='s3v4')
)
```

Requires boto3 >= 1.28.0. Supports all standard S3 operations. Pre-signed URLs work normally.

### S3 Limitations with B2
- No SSE-KMS, no IAM roles, no object tagging
- No website config, no object-level ACLs
- HTTPS only, Signature V4 only
- No key management (use native API)

---

## Other Language SDKs

| Language | SDK | Notes |
|----------|-----|-------|
| **Go** | `github.com/Backblaze/blazer` (official) | io.Writer/io.Reader API, auto multipart at 100MB, zero third-party deps |
| **Java** | `b2-sdk-java` (official) | Builder pattern, parallel multipart, thread-safe |
| **Rust** | `b2-client`, `backblaze-b2-client` | Community crates; Tokio-based |
| **All others** | Via S3 API | AWS SDKs for .NET, PHP, Ruby, JavaScript v3, etc. |

---

## Client-Side Encryption Patterns

Since B2's SSE only protects data at rest on B2's servers (B2 or transit provider can still see plaintext during transfer for SSE-B2), **client-side encryption is essential** for true zero-knowledge security.

### Library Options

| Library | Algorithm | Streaming Support | Notes |
|---------|-----------|-------------------|-------|
| **cryptography** (Python) | AES-256-GCM, Fernet | Manual chunking required | Most flexible, widely used |
| **PyNaCl/libsodium** | XSalsa20+Poly1305 (SecretBox), SecretStream | SecretStream handles chunking (~64KB) | Cleanest streaming API, ~1GB/s |
| **pyrage** | age encryption (X25519+ChaCha20-Poly1305) | Built-in streaming | Rust bindings, interop with age CLI |

### Recommended Pattern: Envelope Encryption

```
┌─────────────────────────────────────────────────┐
│                  Master Key (MEK)                │
│          (stored securely, never uploaded)       │
└──────────────────────┬──────────────────────────┘
                       │ wraps
                       ▼
┌─────────────────────────────────────────────────┐
│            Data Encryption Key (DEK)             │
│      (random per-file, wrapped copy stored       │
│       alongside ciphertext in B2)                │
└──────────────────────┬──────────────────────────┘
                       │ encrypts
                       ▼
┌─────────────────────────────────────────────────┐
│              Encrypted File Data                 │
│        (AES-256-GCM or ChaCha20-Poly1305)       │
└─────────────────────────────────────────────────┘
```

1. Generate random per-file DEK
2. Encrypt data with DEK using AES-256-GCM
3. Wrap DEK with master key
4. Store wrapped DEK alongside ciphertext (as B2 file metadata or prepended header)

### Large File Encryption

For files too large to fit in memory:

**Chunked AEAD approach:**
- Split into fixed-size chunks (e.g., 1 MB)
- Each chunk encrypted with per-chunk nonce
- Chunk index included in associated data (prevents reordering/truncation)
- Final chunk marker prevents truncation attacks

**PyNaCl SecretStream** handles this natively with ~64KB chunks.

### Key Derivation (from password)

| Algorithm | Strength | Notes |
|-----------|----------|-------|
| **Argon2id** | Best | Memory-hard, side-channel resistant |
| **scrypt** | Strong | Memory-hard, used by rclone |
| **PBKDF2-HMAC-SHA256** | Minimum viable | Needs 1.2M+ iterations |

### Interaction with Cloudflare CDN

- Encrypted objects **cache normally** at Cloudflare (cached by URL/headers, not content)
- Cloudflare **cannot** compress, transform, or inspect encrypted content
- This is actually **desirable** for our security model — CDN is a dumb cache of opaque blobs
- Must set explicit `Cache-Control` headers (B2 defaults to no-cache)
- Use Workers for authenticated access to private B2 buckets

---

## Existing Open-Source Wrappers

### rclone crypt
- NaCl SecretBox (XSalsa20 + Poly1305), 64KB chunks
- scrypt key derivation from password
- EME/AES-256 filename encryption
- 0.05% size overhead
- Two-layer remote: `crypt` wrapping `b2` backend
- **Most mature B2+encryption solution**

### restic
- AES-256-CTR + Poly1305-AES
- Content-addressed deduplication
- Password-derived repository key
- Direct B2 backend support

### Kopia
- AES-256-GCM or ChaCha20-Poly1305
- PBKDF + HKDF key derivation
- Deduplication
- Go-based, GUI + CLI
- Direct B2 backend

### duplicity
- GPG encryption (symmetric AES or asymmetric)
- tar-based archives
- B2 backend support
