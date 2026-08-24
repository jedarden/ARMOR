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

### Environment Variable

```bash
ARMOR_COMPRESS=true  # Enable zstd compression for single-PUT uploads
```

**Default:** `false` (compression disabled)

**Scope:** Single-PUT uploads only (rejected for multipart)

### README Configuration Table Entry

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `ARMOR_COMPRESS` | No | `false` | Enable zstd compression for single-PUT uploads. Multipart uploads are rejected when enabled. Compressed objects do not support byte-range reads. |

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

- **Storage cost reduction** for compressible data types when `ARMOR_COMPRESS=true`
- **Multipart uploads rejected** when compression enabled — operators must choose one or the other per workload
- **Range reads unsupported** for compressed objects — full-object download required
- **CPU overhead** on uploads for compression work (zstd is fast; ~100-300 MB/s/core)
- **No format-version bump** — reserved-byte mechanism backward compatible with legacy objects
- **Opt-in only** — existing deployments unaffected unless explicitly enabled
- **Clear error surface** — multipart creation and range reads return helpful errors

## Future Work

If multipart compression becomes a strong requirement, a future ADR could design:
- Fixed-size block-aligned compression format (custom or extended zstd)
- Cumulative offset tracking for part sizes (sacrifices ADR-005's part-number-only advantage)
- Post-Complete compression and rewrite operation
- Accept non-idempotent retries (store compressed size per retry)

All require significant design work and should not be undertaken without clear use case and cost-benefit analysis.

## Related Documentation

- ADR-005: Out-of-order multipart uploads with uniform part sizes
- ADR-006: Dual-backend async replication (opt-in pattern reference)
- ADR-007 (multipart): Multipart uploads do not support compression
- ADR-012: Authorization action verbs and consumer separation
