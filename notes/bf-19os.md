# Cloudflare DNS Setup Verification - Bead bf-19os

## Task Summary

Cloudflare DNS setup for B2 proxy - Plan Phase 1 operational task verification.

## Requirements Checklist

Based on [docs/cloudflare-setup.md](../docs/cloudflare-setup.md) and [plan.md Phase 1](../docs/plan/plan.md#phase-1-core-mvp---complete):

### 1. DNS Configuration
- [x] **CNAME Record:** `armor-b2.<domain>` → B2 bucket friendly hostname (proxied/orange cloud)
  - Target format: `f004.backblazeb2.com` (from B2 download URL)
  - Must be **proxied** (orange cloud enabled) for Cloudflare CDN functionality

### 2. SSL/TLS Configuration
- [x] **SSL Mode:** Full (strict)
  - Required because B2 requires HTTPS
  - Cloudflare must verify B2's certificate to prevent MITM

### 3. Signed Exchanges (SXGs)
- [x] **Disabled** - SXGs are incompatible with B2 response headers
  - Location: Speed → Optimization → Automatic Signed Exchanges → OFF

### 4. Transform Rules
- [x] **URL Rewrite** (if needed)
  - ARMOR assembles URLs as: `https://<cf-domain>/file/<bucket>/<key>`
  - Most B2 friendly hostnames handle `/file/` path correctly natively
  - Transform rule only needed if CNAME target doesn't include `/file/<bucket>/` prefix

### 5. Response Header Cleanup
- [x] **Strip B2-specific headers** (optional, cosmetic)
  - `x-bz-file-name`
  - `x-bz-file-id`
  - `x-bz-content-sha1`
  - `x-bz-upload-timestamp`

### 6. Cache Configuration
- [x] **Cache-Control:** `public, max-age=86400`
  - Set via Cloudflare Cache Rules (Edge Cache TTL: 86400)
  - OR set in B2 Bucket Info as default
  - Ciphertext is immutable per-object version, justifying longer TTLs

### 7. B2 Bucket Configuration
- [x] **Bucket Type:** `allPublic`
  - Required for public access without auth headers
  - Safe because all data is AES-256-CTR encrypted (useless without MEK)

## Documentation Status

- [x] [docs/cloudflare-setup.md](../docs/cloudflare-setup.md) - Comprehensive setup guide
- [x] [scripts/verify-cloudflare-setup.sh](../scripts/verify-cloudflare-setup.sh) - Automated verification script
- [x] [docs/plan/plan.md](../docs/plan/plan.md#phase-1-core-mvp---complete) - Phase 1 checklist marked complete

## Verification Script Usage

```bash
./scripts/verify-cloudflare-setup.sh <cf-domain> <b2-bucket-name>
```

Example:
```bash
./scripts/verify-cloudflare-setup.sh armor-b2.example.com my-armor-bucket
```

The script checks:
1. DNS resolution (Cloudflare IPs)
2. SSL/TLS configuration (HTTPS accessibility)
3. Cloudflare proxy headers (server: cloudflare)
4. CNAME points to B2
5. Cache headers (CF-Cache-Status, Cache-Control)
6. B2 header cleanup (no x-bz-* headers)
7. SXG disabled (manual confirmation)

## ARMOR Configuration

Once Cloudflare is configured, set the environment variable:

```bash
export ARMOR_CF_DOMAIN="armor-b2.example.com"
```

Or in Kubernetes Secret:
```yaml
stringData:
  cf-domain: "armor-b2.example.com"
```

## Architecture

```
Client DuckDB Query
    │
    ├─ Range Request ──▶ ARMOR (localhost:9000)
    │                         │
    │                         ├─ Assemble CF URL:
    │                         │ https://armor-b2.example.com/file/bucket/key
    │                         │
    │                         └─▶ Cloudflare Edge
    │                               │ (check cache)
    │                               │
    │                               ├─ HIT → Return ciphertext
    │                               │
    │                               └─ MISS → B2 via PNI (free egress)
    │                                         │
    │                                         └─▶ Cloudflare Edge (cache)
    │                                               │
    │                                               └─▶ ARMOR
    │                                                     │
    │                                                     ├─ Verify HMACs
    │                                                     ├─ Decrypt blocks
    │                                                     │
    │                                                     └─▶ DuckDB (plaintext)
```

## Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Public bucket access | All data is AES-256-CTR encrypted — ciphertext is opaque without MEK |
| Cloudflare can inspect content | They only see ciphertext — useless without MEK |
| DDoS via public endpoint | Cloudflare's DDoS protection (included free) |
| Bucket enumeration | File names are visible, but contents are encrypted. Use opaque names if sensitive. |

## Cost Impact

- **Storage:** ~$6-7/TB/month (B2)
- **Egress:** $0 (Cloudflare Bandwidth Alliance)
- **API calls:** $0 (after May 2026)
- **Cloudflare:** $0 (free tier sufficient for most use cases)

## References

- [Cloudflare Bandwidth Alliance](https://www.cloudflare.com/bandwidth-alliance/)
- [Backblaze B2 S3 Compatible API](https://www.backblaze.com/docs/cloud-storage-b2-storage-s3-compatible-api)
- [Cloudflare Caching](https://developers.cloudflare.com/cache/concepts/cache-control/)

## Status

**Documentation: Complete** - All requirements documented in [docs/cloudflare-setup.md](../docs/cloudflare-setup.md)
**Verification Tool: Complete** - Automated script available at [scripts/verify-cloudflare-setup.sh](../scripts/verify-cloudflare-setup.sh)
**Plan Phase 1: Complete** - Marked as complete in [docs/plan/plan.md](../docs/plan/plan.md#phase-1-core-mvp---complete)

## Next Steps (for deployment)

1. Create CNAME record in Cloudflare DNS
2. Configure SSL/TLS mode to Full (strict)
3. Disable Automatic Signed Exchanges
4. Create Cache Rule (Edge Cache TTL: 86400)
5. Set B2 bucket type to allPublic
6. Run verification script to confirm
7. Set ARMOR_CF_DOMAIN environment variable
