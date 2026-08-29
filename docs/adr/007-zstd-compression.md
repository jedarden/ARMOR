# ADR-007: zstd Compression for Single-PUT Objects

**Status:** Accepted
**Date:** 2026-08-23
**Related:** ADR-005 (uniform-part-size contract), bead armor-8d387bf4

## Context

ARMOR stores encrypted data in Backblaze B2 at ~$6–7/TB/month. For compressible data types — manifests, WAL segments, JSON logs, text-based configs — zstd compression can reduce storage costs by 2-5x with minimal CPU overhead. Parquet files already compress internally and benefit less from additional compression.

Compression must be **opt-in** (not default) to preserve the existing cost model and avoid surprising behavior changes. The design follows ADR-006's opt-in pattern: feature exists but is disabled unless explicitly enabled via environment variable.

## Decision

Add optional zstd compression for **single-PUT objects only** with the following properties:

1. **Opt-in via ARMOR_COMPRESS env var** (default: `false`). When set to `true`, ARMOR compresses plaintext before encryption on uploads.
2. **Compress-before-encrypt** — compression operates on plaintext, not ciphertext. Encrypted data is pseudo-random and does not compress.
3. **Reserved-byte flag mechanism** — use the 2-byte `Reserved` field in `EnvelopeHeader` to signal compression without a format-version bump. Legacy objects without the flag self-describe as uncompressed.
4. **Opportunistic pass-through** — if zstd does not shrink the data (e.g., already-compressed Parquet), store the original uncompressed. Avoids paying compression CPU for no benefit.
5. **Multipart uploads rejected** — when `ARMOR_COMPRESS=true`, `CreateMultipartUpload` MUST fail with `InvalidArgument` error. See ADR-007 (multipart-compression) for rationale.
6. **Range reads unsupported** — compressed objects reject byte-range requests with HTTP 416. Full-object download required. Variable-length encoding prevents fixed-offset seeking.

## Implementation

### Compress-Before-Encrypt Flow

```
Plaintext → zstd compress → Compressed Plaintext → AES-CTR encrypt → Ciphertext → B2
```

Compression happens **before** encryption because:
- Ciphertext is pseudo-random (AES-CTR keystream XOR) — entropy ~8 bits/byte
- Compression algorithms require exploitable patterns
- Compressing ciphertext would waste CPU with minimal size reduction

### Reserved-Byte Flag Mechanism

The `EnvelopeHeader` struct (`internal/crypto/envelope.go`) includes:

```go
type EnvelopeHeader struct {
    Magic         [4]byte // "ARMR"
    Version       uint8   // 0x01 or 0x02
    BlockSizeLog2 uint8   // log2(block_size)
    IV            [16]byte
    PlaintextSize uint64
    PlaintextSHA  [32]byte
    Reserved      [2]byte // Previously unused, now for compression flag
}
```

**Flag encoding** (one byte, second byte reserved for future use):
- `0x00` — Uncompressed (default, backward compatible)
- `0x01` — zstd compressed
- `0x02-0xFF` — Reserved for future compression types (gzip, zlib, etc.)

**Why reserved bytes instead of metadata?**
- Self-describing format — object metadata can be stripped or lost
- No format-version bump — legacy readers ignore unknown reserved values
- Immediate detection — decompressor knows before reading plaintext

### Opportunistic Pass-Through

Not all data compresses well. The implementation:

1. Compress the full plaintext with zstd default level
2. If compressed_size >= original_size, discard compressed version
3. Store uncompressed with `Reserved[0] = 0x00`

This ensures:
- Parquet files (already compressed) don't waste CPU
- Incompressible data (random, encrypted) doesn't bloat
- No decision needed per-object-type — automatic

### Multipart Upload Restriction

When `ARMOR_COMPRESS=true`, `CreateMultipartUpload` MUST return:

```xml
<Error>
    <Code>InvalidArgument</Code>
    <Message>Compression is not supported for multipart uploads. Use single-PUT uploads for compressed objects or disable compression (ARMOR_COMPRESS=false).</Message>
    <RequestId>...</RequestId>
</Error>
```

See ADR-007 (multipart-compression) for detailed rationale. Summary:
- Per-part compression breaks ADR-005's uniform-part-size contract
- CTR block alignment breaks on variable-length compressed parts
- Idempotent retries violated (same plaintext → different compressed output)

### Range Read Limitation

Compressed objects **MUST reject** byte-range requests:

```
GET /bucket/key HTTP/1.1
Range: bytes=1024-2047

HTTP/1.1 416 Range Not Satisfiable
Content-Type: application/xml

<Error>
    <Code>InvalidRange</Code>
    <Message>Range reads unsupported on compressed objects</Message>
</Error>
```

**Rationale:**
- zstd, gzip, zlib are variable-length frame encodings
- No block-aligned seeking within compressed stream
- Byte range into compressed ciphertext returns corrupt data after decryption
- DuckDB columnar queries require range reads — compressed Parquet is incompatible

Clients that need range access must use uncompressed objects (disable compression or use multipart).

## Configuration

### Environment Variables

```bash
# Legacy global compression (alias for *=zstd)
ARMOR_COMPRESS=true  # Enable zstd compression for all single-PUT uploads

# Fine-grained compression rules (recommended)
ARMOR_COMPRESS_RULES=".jsonl=zstd,.wal=zstd,application/json=zstd,text/plain=zstd,*=none"

# Rules: comma-separated <suffix>|<content-type>=zstd|none
# First match wins. Wildcard *=none should be last to catch unmatched objects.
# Examples:
#   .jsonl=zstd                    # Compress files ending in .jsonl
#   .wal=zstd                     # Compress WAL files
#   application/json=zstd         # Compress JSON content-type
#   text/plain=zstd               # Compress plain text
#   application/octet-stream=none  # Don't compress binary data
#   *=none                        # Catch-all: don't compress anything else
```

**Default:** `false` (compression disabled)

**Scope:** Single-PUT uploads only (rejected for multipart)

**Per-Request Override:**

Clients can override compression rules per-request using the `x-amz-meta-armor-compress` metadata header:

```bash
# Force compression for this request (overrides rules)
curl -X PUT http://armor:9000/bucket/key \
  -H "x-amz-meta-armor-compress: true" \
  --data-binary @file.jsonl

# Force no compression for this request
curl -X PUT http://armor:9000/bucket/key \
  -H "x-amz-meta-armor-compress: false" \
  --data-binary @file.jsonl
```

Valid values: `true` (compress), `false` (don't compress). Invalid values return HTTP 400.

### README Configuration Table Entry

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_COMPRESS` | No | `false` | Legacy alias for `ARMOR_COMPRESS_RULES="*=zstd"` (all files compressed). Prefer `ARMOR_COMPRESS_RULES` for fine-grained control. Multipart uploads are rejected when compression is enabled. Compressed objects do not support byte-range reads. |
| `ARMOR_COMPRESS_RULES` | No | — | Comma-separated compression rules: `<suffix>|<content-type>=zstd|none`. First match wins. Examples: `.jsonl=zstd,.wal=zstd,application/json=zstd,*=none`. Per-request override via `x-amz-meta-armor-compress: true|false` header. Only applies to v3 single-PUT format. |

## Multipart Uploads and Compression

**Multipart uploads do NOT support compression.** When `ARMOR_COMPRESS_RULES` or `ARMOR_COMPRESS=true` is configured, `CreateMultipartUpload` returns HTTP 400 `InvalidArgument` with the message:

```
Compression is not supported for multipart uploads. Use single-PUT uploads for compressed objects or disable compression (ARMOR_COMPRESS=false or ARMOR_COMPRESS_RULES empty).
```

**Why multipart parts are never compressed:**

1. **ADR-005 Contract Violation:** Multipart uploads require uniform part sizes for out-of-order upload support. Per-part compression produces variable-sized parts, breaking the uniform-part-size invariant.

2. **CTR Block Alignment:** AES-CTR encryption requires block-aligned ciphertext. Compression changes part sizes, making CTR block offsets unpredictable without cumulative size tracking.

3. **Idempotent Retries:** zstd compression is not deterministic — the same input can produce different compressed outputs. This breaks idempotent upload retries, a key design goal.

4. **No Compression Format Supports Block-Aligned Seeking:** zstd, gzip, and zlib are stream formats. No mainstream format supports seeking within compressed streams without decompressing from the start.

5. **Design Simplicity:** Single-PUT compression provides the core benefit (storage cost reduction) for compressible workloads without the complexity of per-part compressed size tracking and custom compression formats.

**For multipart use cases:** Use single-PUT uploads for objects that benefit from compression (manifests, WAL segments, JSON logs). Multipart remains available for large files that don't compress well (media files, encrypted data, already-compressed formats).

## v3 Format Relationship

Compression rules apply **only to v3 single-PUT format** (`ARMOR_FORMAT_VERSION=3`). The v3 format (see `internal/crypto/v3.go`) stores the block table in a trailer rather than inline, enabling cleaner integration with compression:

- **v2 format:** Header → Encrypted Blocks → Inline HMAC Table (within the stream)
- **v3 format:** Header → Encrypted Blocks → Trailer Block Table (after all blocks)

Compression operates on plaintext before encryption, producing a compressed plaintext stream that is then encrypted with v3 semantics. The compression flag is stored in the envelope header's reserved bytes (see ADR-007 Reserved-Byte Flag Mechanism).

**Why v3-only:** v2's inline HMAC table is embedded within the encrypted block stream, making the format less suitable for compression integration. v3's trailer design separates concerns: compression operates on plaintext, encryption on the result, and the trailer block table follows cleanly.

**Configuration:** Set `ARMOR_FORMAT_VERSION=3` to enable v3 writes. v3 reads are always supported regardless of write version.

## Cost Model Impact

### README Cost Model Table Addition

| Component | Cost | Notes |
|-----------|------|-------|
| Storage | ~$6–7/TB/month | Base B2 storage cost |
| **Compression savings** | **Varies by data type** | Optional zstd reduces storage for compressible data: manifests (2-5x), WAL (3-5x), JSON logs (2-4x). Parquet/columnar: minimal additional benefit (already compressed internally). |
| Egress (via Cloudflare Bandwidth Alliance) | $0 | Unchanged |
| B2 API calls | $0 | Unchanged |
| Cloudflare (free plan) | $0 | Unchanged |

### When Compression Helps

**Good compression candidates:**
- Database WAL segments (PostgreSQL, SQLite)
- JSON/NDJSON logs and event streams
- YAML/TOML/config files
- Text-based manifests and indexes
- CSV files (text-based, not already compressed)

**Poor compression candidates:**
- Parquet/ORC/Avro (already compressed columnar)
- Image/video media files (JPEG, MP4, PNG)
- Already-compressed formats (gzip, zip, tar.gz)
- Encrypted data (pseudo-random ciphertext)

## Alternatives Considered

### Compress after encryption (ciphertext compression)

**Rejected** because:
- AES-CTR ciphertext is pseudo-random — entropy ~8 bits/byte
- Compression algorithms achieve no meaningful size reduction
- Wastes CPU cycles with no storage benefit

### Support compression for multipart uploads

**Rejected** — see ADR-007 (multipart-compression) for full analysis. Summary:
- Breaks ADR-005's uniform-part-size invariant
- Requires per-part compressed size tracking (breaks out-of-order uploads)
- No standard compression format supports block-aligned seeking
- Idempotent retries violated (same input → different compressed output)

### Default compression enabled

**Rejected** because:
- Breaks existing ~$6–7/TB/month cost model promise
- Parquet users (columnar queries) would lose range-read capability
- Surprise behavior change for existing deployments
- Opt-in follows ADR-006 pattern (operator explicitly chooses feature)

### Support range reads over compressed data

**Rejected** because:
- No mainstream compression format (zstd, gzip, zlib) supports block-aligned seeking
- Would require custom compression format or proxy-level decompression + re-encryption
- Defeats the "seekable encryption" design goal for compressed objects
- DuckDB column pruning/predicate pushdown incompatible with compression

## Consequences

- **Storage cost reduction** for compressible data types when `ARMOR_COMPRESS_RULES` or `ARMOR_COMPRESS=true` is configured
- **Fine-grained compression control** via suffix and content-type rules (e.g., compress `.jsonl` and `application/json` but not `.mp4`)
- **Per-request override** via `x-amz-meta-armor-compress` header for client-side control
- **Multipart uploads rejected** when compression rules exist — operators must choose single-PUT for compressed objects or disable compression
- **Range reads unsupported** for compressed objects — full-object download required
- **CPU overhead** on uploads for compression work (zstd is fast; ~100-300 MB/s/core)
- **No format-version bump** — reserved-byte mechanism backward compatible with legacy objects
- **Opt-in only** — existing deployments unaffected unless explicitly configured
- **Clear error surface** — multipart creation, range reads, and invalid override values return helpful errors
- **v3 format required** for compression rules (v3 trailer block table design; set `ARMOR_FORMAT_VERSION=3`)
- **Streaming disabled** when compression rules are configured (requires buffering for rule evaluation and compression)

## Future Work

If multipart compression becomes a strong requirement, a future ADR could design:
- Fixed-size block-aligned compression format (custom or extended zstd)
- Cumulative offset tracking for part sizes (sacrifices ADR-005's part-number-only advantage)
- Post-Complete compression and rewrite operation
- Accept non-idempotent retries (store compressed size per retry)

All require significant design work and should not be undertaken without clear use case and cost-benefit analysis.

**v3 Format Evolution:** The current compression rules system is tied to v3 format (`ARMOR_FORMAT_VERSION=3`). Future format versions could explore:
- Block-level compression (compress individual 64KB blocks independently)
- Compression metadata in trailer (compressed sizes per block for seeking)
- Hybrid approaches (compress some blocks, not others based on compressibility detection)

## Related Documentation

- ADR-005: Out-of-order multipart uploads with uniform part sizes
- ADR-006: Dual-backend async replication (opt-in pattern reference)
- ADR-007 (multipart): Multipart uploads do not support compression
- ADR-012: Authorization action verbs and consumer separation
- **v3 Format Specification:** `internal/crypto/v3.go` — v3 single-PUT format with trailer block table
