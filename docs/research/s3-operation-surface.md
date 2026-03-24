# S3 Operation Surface for b2-layer

## Design Principle

Users interact with b2-layer through normal file semantics: open, read, write, list, delete, stat. The encryption layer is invisible. b2-layer translates every operation into the appropriate S3 calls with encryption/decryption injected at the boundary.

```
┌─────────────────────────────────────────────┐
│              User / Application              │
│   (sees plaintext filenames, plaintext data) │
├─────────────────────────────────────────────┤
│              b2-layer interface              │
│   ├── Encrypt/decrypt data                  │
│   ├── Encrypt/decrypt filenames (optional)  │
│   ├── Manage DEKs, IVs, HMACs              │
│   └── Translate file ops → S3 calls         │
├─────────────────────────────────────────────┤
│          S3-Compatible API (boto3)           │
│   (sees ciphertext, encrypted metadata)     │
├─────────────────────────────────────────────┤
│              Backblaze B2                    │
└─────────────────────────────────────────────┘
```

---

## Operation Categories

### 1. Object Read Operations

| S3 Operation | b2-layer Use | Encryption Behavior |
|---|---|---|
| `GetObject` | Download a file | Fetch ciphertext → decrypt → return plaintext |
| `GetObject` (Range) | DuckDB byte-range reads | Translate plaintext range → encrypted block range → fetch → decrypt → return slice |
| `HeadObject` | Check existence, get size, freshness check | Read encrypted metadata → return decrypted original size, content type, modified time |

#### HeadObject Detail

`HeadObject` is used heavily and never transfers file data:

- **Cache freshness:** Compare ETag/LastModified against local manifest to decide if a re-download is needed
- **Existence check:** Before upload (avoid duplicates), before download (fail fast)
- **Size query:** Report original plaintext size (stored in encrypted file header or B2 file info metadata)
- **Metadata read:** Retrieve wrapped DEK, IV, block size — needed before any decrypt operation

b2-layer stores encryption parameters in B2's custom file info headers (`X-Bz-Info-*` via S3 `x-amz-meta-*`):

```
x-amz-meta-b2l-version: 1
x-amz-meta-b2l-iv: <base64>
x-amz-meta-b2l-wrapped-dek: <base64>
x-amz-meta-b2l-block-size: 16384
x-amz-meta-b2l-plaintext-size: 104857600
x-amz-meta-b2l-plaintext-sha256: <hex>
x-amz-meta-b2l-original-name: <encrypted or plaintext>
x-amz-meta-b2l-content-type: application/octet-stream
```

B2 file info limit: 10 headers, each key ≤50 bytes, total info ≤7000 bytes. This is sufficient for the fields above. The wrapped DEK is the largest field (~100-200 bytes base64).

---

### 2. Object Write Operations

| S3 Operation | b2-layer Use | Encryption Behavior |
|---|---|---|
| `PutObject` | Upload small files (≤ part size threshold) | Generate DEK → encrypt → upload ciphertext + metadata |
| `CreateMultipartUpload` | Begin large file upload | Generate DEK, IV; store pending state locally |
| `UploadPart` | Upload each part of a large file | Encrypt part's plaintext blocks with continuing CTR counter |
| `CompleteMultipartUpload` | Finalize large file | Append HMAC table, write encryption metadata to file info |
| `AbortMultipartUpload` | Cancel failed/abandoned upload | No crypto action; B2 cleans up parts |
| `ListMultipartUploads` | Find incomplete uploads for resume | Decrypt upload metadata from local state |
| `ListParts` | Resume interrupted upload | Determine which parts succeeded, resume from next |

#### Multipart Encryption Detail

Large files are split into S3 parts (minimum 5 MB for B2). Each part contains multiple encryption blocks:

```
Part 1 (5-100 MB):
  ├── Block 0:   AES-CTR(ctr=0)
  ├── Block 1:   AES-CTR(ctr=1)
  ├── ...
  └── Block N-1: AES-CTR(ctr=N-1)

Part 2 (5-100 MB):
  ├── Block N:   AES-CTR(ctr=N)     ← counter continues from part 1
  ├── Block N+1: AES-CTR(ctr=N+1)
  ├── ...
  └── Block M-1: AES-CTR(ctr=M-1)

... etc.
```

The CTR counter is continuous across parts so the assembled file is a single seekable encrypted stream. The HMAC table covering all blocks is uploaded as the final part or written as a sidecar object.

#### Resumable Uploads

On interruption:
1. `ListMultipartUploads` → find the in-progress upload ID
2. `ListParts` → determine which parts completed
3. Resume from the next part with the correct CTR counter offset
4. Local state file tracks: upload ID, DEK, IV, last completed part, running counter

---

### 3. Object Mutation Operations

| S3 Operation | b2-layer Use | Encryption Behavior |
|---|---|---|
| `DeleteObject` | Remove a file | Delete ciphertext; optionally delete local cache entry |
| `DeleteObjects` | Bulk remove | Batch delete; update local manifest |
| `CopyObject` | Server-side copy (rename, key rotation) | See below |

#### CopyObject and Key Rotation

`CopyObject` is a server-side operation — B2 copies the object without downloading it. This enables:

**Rename (no re-encryption):**
```
CopyObject(src="old-name", dst="new-name") → DeleteObject("old-name")
```
Ciphertext is identical. Only the remote key (path) changes. Update metadata if filename is stored in file info.

**DEK re-wrap (key rotation without re-uploading data):**
When the master encryption key (MEK) rotates, every file's DEK must be re-wrapped:
1. `HeadObject` → read current wrapped DEK
2. Unwrap DEK with old MEK
3. Re-wrap DEK with new MEK
4. `CopyObject` with `MetadataDirective=REPLACE` → update `x-amz-meta-b2l-wrapped-dek`
5. Data never moves — server-side copy with new metadata

This makes key rotation an O(n) metadata operation, not an O(n × filesize) re-encryption.

**Full re-encryption (when DEK is compromised):**
Cannot use `CopyObject` — must download, re-encrypt with new DEK, re-upload. Expensive but rare.

---

### 4. Listing and Discovery

| S3 Operation | b2-layer Use | Encryption Behavior |
|---|---|---|
| `ListObjectsV2` | Browse files, `ls` equivalent | Decrypt filenames if encrypted; map remote paths to plaintext names |
| `ListObjectsV2` (prefix) | Partition discovery for DuckDB | Translate plaintext prefix to encrypted prefix (if filenames encrypted) or pass through |
| `ListObjectsV2` (delimiter) | Directory-like navigation | Use `/` delimiter; decrypt returned prefixes |
| `ListObjectVersions` | Version history | Show versions with decrypted metadata per version |

#### Filename Encryption Impact on Listing

If filenames are encrypted, `ListObjectsV2` returns opaque names. Two strategies:

**Strategy A: Deterministic filename encryption (recommended for partitioned data)**
- Same plaintext path always produces the same ciphertext path
- Enables prefix-based listing (`ListObjectsV2(Prefix="enc(data/date=2026-03-01/)")`)
- Tradeoff: reveals whether two files share a prefix (acceptable for directory structure)
- Implementation: AES-SIV or HMAC-based deterministic encryption on path components

**Strategy B: Randomized filenames + metadata index**
- Remote filenames are random UUIDs
- A separate index object maps plaintext names → remote names
- Must fetch and decrypt index before any listing operation
- Stronger privacy (no structural leakage) but adds complexity and a synchronization bottleneck
- Index must be updated atomically on every upload/delete

**Strategy C: Plaintext filenames (simplest)**
- Don't encrypt filenames at all
- Directory structure and file names are visible to B2/Cloudflare
- File contents remain encrypted
- Acceptable when directory structure isn't sensitive (e.g., `data/date=2026-03-01/part-0000.parquet`)

**Recommendation for v1:** Strategy C (plaintext filenames). For partitioned Parquet data, the directory structure (`date=YYYY-MM-DD/`) is not sensitive, and plaintext paths enable DuckDB partition discovery to work naturally. File contents are still fully encrypted.

---

### 5. Pre-signed URLs

| S3 Operation | b2-layer Use | Encryption Behavior |
|---|---|---|
| `generate_presigned_url` (GET) | Shareable download link | URL points to Cloudflare Worker; recipient gets ciphertext (needs b2-layer to decrypt) OR Worker decrypts if authorized |
| `generate_presigned_url` (PUT) | External upload link | Uploader sends plaintext; b2-layer must encrypt first — or uploader uses b2-layer client |

#### Download Sharing Patterns

**Pattern A: Share with b2-layer users**
Generate a Cloudflare URL with embedded HMAC token. Recipient must have b2-layer installed and the correct master key to decrypt after download.

```python
url = layer.share("data/report.parquet", expires_in=86400)
# → https://files.example.com/data/report.parquet?token=<hmac>&expires=<ts>
# Recipient: b2-layer download <url> report.parquet
```

**Pattern B: Share plaintext (decrypt at edge)**
Cloudflare Worker decrypts on the fly for authorized recipients. Requires the Worker to have access to encryption keys — weakens zero-knowledge model but useful for sharing with non-b2-layer users.

**Pattern C: Pre-signed S3 URL (bypass Cloudflare)**
Direct B2 pre-signed URL. Recipient downloads ciphertext directly from B2. Uses B2's 3x free egress allowance (not Cloudflare). Useful for server-to-server transfers where the peer has b2-layer.

#### Upload from External Sources

External uploaders without b2-layer cannot encrypt data themselves. Options:
1. **Staging bucket:** Uploader PUTs plaintext to a staging bucket via pre-signed URL → b2-layer encrypts + moves to encrypted bucket (requires a processing step)
2. **Upload proxy:** b2-layer runs an upload endpoint that accepts plaintext, encrypts, and forwards to B2
3. **Client SDK:** Distribute a lightweight encryption-only SDK/library that external uploaders integrate

---

### 6. Sync and Diffing

| S3 Operation | b2-layer Use | Encryption Behavior |
|---|---|---|
| `ListObjectsV2` + `HeadObject` | Incremental sync | Compare ETags and LastModified against local manifest |
| `GetObject` (conditional) | Conditional download | `If-None-Match` / `If-Modified-Since` to avoid re-downloading unchanged files |

#### Sync Algorithm

```
1. ListObjectsV2(prefix) → remote file list with ETags, sizes, timestamps
2. For each remote file:
   a. Check local manifest: does this ETag exist locally?
   b. If yes → skip (already cached and decrypted)
   c. If no → download ciphertext → decrypt → write to cache → update manifest
3. For each local cache entry:
   a. Check if remote file still exists
   b. If no → evict from cache (file deleted remotely)
4. Update manifest with new ETags and timestamps
```

#### Partition-Aware Sync

For Hive-style partitioned Parquet (`date=YYYY-MM-DD/`), sync can be scoped:

```python
# Only sync last 7 days of data
layer.sync(
    remote="data/",
    local="~/.b2-layer/cache/data/",
    partition_filter={"date": ">= 2026-03-16"},
)
```

Translates to `ListObjectsV2(Prefix="data/date=2026-03-16/")` through `ListObjectsV2(Prefix="data/date=2026-03-23/")` — only fetches listing for matching partitions.

---

### 7. Bucket and Key Management (Native B2 API)

These operations use the B2 native API (not S3) because B2's S3 API doesn't support key management:

| B2 Native Operation | b2-layer Use |
|---|---|
| `b2_create_key` | Create scoped application keys (read-only for Worker, write for uploads) |
| `b2_delete_key` | Revoke compromised or rotated keys |
| `b2_list_keys` | Audit active keys |
| `b2_create_bucket` | Initial setup |
| `b2_update_bucket` | Set lifecycle rules, CORS, cache-control defaults |

These are admin operations, not part of the normal read/write path. Exposed via `b2-layer admin` subcommands.

---

### 8. Lifecycle and Versioning

| S3 Operation | b2-layer Use | Encryption Behavior |
|---|---|---|
| `PutBucketLifecycleConfiguration` | Auto-expire old versions | No crypto impact; B2 deletes ciphertext after retention period |
| `ListObjectVersions` | Browse version history | Each version has its own DEK/IV; metadata per version |
| `GetObject` (versionId) | Retrieve specific version | Decrypt with that version's DEK |
| `DeleteObject` (versionId) | Remove specific version | Delete ciphertext; remove DEK from metadata |

#### Version-Aware Encryption

Each version of a file gets its own DEK and IV. This means:
- Overwriting a file generates a completely new encryption envelope
- Old versions remain independently decryptable (as long as the MEK that wrapped their DEK is retained)
- Key rotation only needs to re-wrap DEKs for the current version (old versions can retain old wrapping if MEK is archived)

---

## Complete Operation Map

### Read Path (through Cloudflare, $0 egress)

```
stat(path)     → HeadObject       → decrypt metadata → return size, type, mtime
read(path)     → GetObject        → decrypt full file → return plaintext
read(path,     → GetObject(Range) → decrypt block range → return plaintext slice
  offset, len)
ls(prefix)     → ListObjectsV2    → (decrypt filenames) → return entries
versions(path) → ListObjectVersions → decrypt per-version metadata → return history
```

### Write Path (direct to B2, $0 ingress)

```
write(path, data)        → generate DEK → encrypt → PutObject + metadata
write(path, large_data)  → generate DEK → encrypt parts → Multipart Upload
delete(path)             → DeleteObject
rename(src, dst)         → CopyObject + DeleteObject (no re-encryption)
rotate_keys()            → HeadObject × N → re-wrap DEKs → CopyObject × N
```

### Sync Path

```
sync(remote, local, filter) → ListObjectsV2 → diff against manifest
                            → GetObject (changed files only)
                            → decrypt → write to cache
                            → update manifest
```

### Admin Path (B2 native API)

```
create_key(scope)  → b2_create_key
revoke_key(id)     → b2_delete_key
setup_bucket()     → b2_create_bucket + b2_update_bucket
```

---

## Transparency Guarantee

From the user's perspective, every operation behaves identically to working with a local filesystem or an unencrypted S3 bucket:

| User Action | What They See | What Actually Happens |
|---|---|---|
| `layer.upload("report.parquet")` | File uploaded | DEK generated, data encrypted, ciphertext + metadata uploaded to B2 |
| `layer.download("report.parquet")` | File appears locally | Ciphertext fetched via Cloudflare, decrypted, plaintext written |
| `layer.ls("data/")` | List of filenames | ListObjectsV2, filenames decoded/decrypted |
| `duckdb.sql("SELECT ... FROM read_parquet('b2l://...')")` | Query results | Footer range-read + decrypted, column chunks range-read + decrypted, only needed bytes transferred |
| `layer.sync("data/", local_dir)` | Local dir updated | Incremental diff, only changed files fetched + decrypted |
| `layer.share("report.parquet")` | Shareable URL | HMAC-signed Cloudflare URL with expiry |
| `layer.key_rotate()` | "Done" | All DEKs re-wrapped server-side via CopyObject, no data re-uploaded |

No ciphertext, IVs, DEKs, or HMACs are ever visible to the user.
