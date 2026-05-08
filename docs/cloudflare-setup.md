# Cloudflare DNS Setup for B2 Proxy

This guide explains how to configure Cloudflare DNS to serve as a zero-egress CDN proxy for Backblaze B2 downloads.

## Overview

ARMOR uses Cloudflare's Bandwidth Alliance to achieve **$0 egress** on all downloads. Cloudflare caches encrypted ciphertext at the edge — ARMOR decrypts on-the-fly before serving to clients. This works because:

1. **B2 bucket is public** (`allPublic`) — no auth headers = cacheable
2. **All data is encrypted** — public access to ciphertext is harmless without the MEK
3. **Cloudflare proxies via CNAME** — orange cloud enables caching and PNI routing

## Prerequisites

- A domain on Cloudflare (e.g., `example.com`)
- A Backblaze B2 bucket configured as `allPublic`
- The B2 bucket's "friendly hostname" (from B2 web UI or download URL)

## Step 1: Find Your B2 Bucket Friendly Hostname

1. Upload any file to your B2 bucket
2. In the B2 web UI, click on the file and look at the "Download URL" or "Friendly URL"
3. Extract the hostname portion — it will look like `f004.backblazeb2.com`

Example:
```
Download URL: https://f004.backblazeb2.com/file/my-bucket/my-file.txt
Friendly hostname: f004.backblazeb2.com
```

## Step 2: Create CNAME Record in Cloudflare

1. Go to **DNS** → **Records** in Cloudflare dashboard
2. Click **Add Record**
3. Configure:
   - **Type:** `CNAME`
   - **Name:** `armor-b2` (or your preferred subdomain)
   - **Target:** `<your-bucket-friendly-hostname>` (e.g., `f004.backblazeb2.com`)
   - **Proxy status:** ☁️ **Proxied** (orange cloud — REQUIRED)
   - **TTL:** Auto

Example:
```
Type: CNAME
Name: armor-b2
Target: f004.backblazeb2.com
Proxy status: Proxied (orange cloud)
```

Result: `armor-b2.example.com` → `f004.backblazeb2.com`

## Step 3: Configure SSL/TLS

1. Go to **SSL/TLS** → **Overview**
2. Set encryption mode to: **Full (strict)**

**Why Full (strict)?** B2 requires HTTPS and presents a valid certificate. Cloudflare must verify the origin certificate to prevent MITM attacks.

## Step 4: Disable Signed Exchanges (SXGs)

1. Go to **Speed** → **Optimization**
2. Find **Automatic Signed Exchanges**
3. Turn it **OFF**

**Why?** SXGs are incompatible with B2's response headers and will cause download failures.

## Step 5: Create URL Rewrite Transform Rule

ARMOR assembles Cloudflare URLs in the format:
```
https://<cf-domain>/file/<bucket>/<key>
```

If your CNAME target doesn't already include the `/file/<bucket>/` prefix, create a transform rule:

1. Go to **Rules** → **Transform Rules** → **Modify Response Header**
2. Click **Create Rule**
3. Configure:
   - **Rule name:** `ARMOR B2 Proxy URL Rewrite`
   - **Field:** `URI Path`
   - **Operator:** `does not match`
   - **Value:** `/file/<your-bucket-name>/*`
   - **Then:** Rewrite to `/file/<your-bucket-name>/` + incoming URI

For most setups, the standard CNAME to `f004.backblazeb2.com` works without URL rewriting because B2's friendly hostname already handles the `/file/` path correctly.

## Step 6: Response Header Cleanup (Optional)

Strip B2-specific headers that leak implementation details:

1. Go to **Rules** → **Transform Rules** → **Modify Response Header**
2. Click **Create Rule**
3. Configure:
   - **Rule name:** `ARMOR B2 Response Header Cleanup`
   - **When:** Incoming requests match `armor-b2.your-domain.com/*`
   - **Then:** Set static response headers
   - **Remove headers:**
     - `x-bz-file-name`
     - `x-bz-file-id`
     - `x-bz-content-sha1`
     - `x-bz-upload-timestamp`

## Step 7: Configure Caching

Set cache headers for optimal Cloudflare caching behavior:

### Option A: Via Cloudflare Cache Rules (Recommended)

1. Go to **Rules** → **Cache Rules**
2. Click **Create Rule**
3. Configure:
   - **Rule name:** `ARMOR B2 Cache Rule`
   - **When:** Incoming requests match `armor-b2.your-domain.com/*`
   - **Then:** Set cache settings
   - **Browser Cache TTL:** `Respect Existing Headers`
   - **Edge Cache TTL:** `86400` (1 day)
   - **Cache Level:** `Standard`

### Option B: Via B2 Bucket Lifecycle Rules

Set default `Cache-Control` on the B2 bucket:

1. Go to B2 web UI → **Buckets** → your bucket
2. **Settings** → **Bucket Info**
3. Add `Cache-Control: public, max-age=86400`

**Why these values?**
- `public` — allows Cloudflare and intermediate caches to store responses
- `max-age=86400` — 1-day edge cache TTL (ciphertext is immutable per-object version)
- Longer TTLs reduce B2 API calls for hot data

## Step 8: Verify Configuration

Test the Cloudflare proxy:

```bash
# Test 1: DNS resolution
dig +short armor-b2.example.com
# Should return Cloudflare IPs

# Test 2: HTTP response (should include CF-Cache-Status)
curl -I https://armor-b2.example.com/file/your-bucket/test-file.txt

# Look for these headers:
# CF-Cache-Status: HIT/MISS
# Server: cloudflare
```

Expected response:
```
HTTP/2 200
cf-cache-status: MISS
server: cloudflare
...
```

## Step 9: Configure ARMOR

Set the `ARMOR_CF_DOMAIN` environment variable to your Cloudflare domain:

```bash
export ARMOR_CF_DOMAIN="armor-b2.example.com"
```

ARMOR will now assemble download URLs as:
```
https://armor-b2.example.com/file/your-bucket/object-key
```

## B2 Bucket Configuration Checklist

Ensure your B2 bucket is configured correctly:

- [ ] **Bucket type:** `allPublic` — required for public access without auth headers
- [ ] **Object Lock:** Disabled (unless you need WORM storage)
- [ ] **Default Cache-Control:** `public, max-age=86400` (set in bucket info)
- [ ] **CORS:** Not needed for ARMOR (server-to-B2 path uses S3 API)

## Cloudflare Page Rules (Optional)

For additional control, create page rules:

1. Go to **Rules** → **Page Rules**
2. Create rule for `*armor-b2.example.com/file/*`
3. Settings:
   - **Cache Level:** Cache Everything
   - **Edge Cache TTL:** 1 day
   - **Browser Cache TTL:** Respect Existing Headers

## Troubleshooting

### Cloudflare returns 521/522 errors

- Check that B2 bucket is `allPublic`
- Verify CNAME target is correct (use `dig` to check)
- Ensure SSL mode is **Full (strict)**

### Downloads fail with 403/404

- B2 bucket must be `allPublic`
- Check that object key is URL-encoded in ARMOR's download URL
- Verify Cloudflare proxy (orange cloud) is enabled

### Poor cache hit rate

- Increase `max-age` in Cache-Control header
- Check that `Cache-Level` is set to **Cache Everything** or **Standard**
- Ensure no `Authorization` headers are sent (public bucket = no auth)

### SXG errors

- Verify **Automatic Signed Exchanges** is disabled
- Clear Cloudflare cache after disabling

## Security Considerations

| Concern | Mitigation |
|---------|-----------|
| Public bucket access | All data is AES-256-CTR encrypted — ciphertext is opaque without MEK |
| Cloudflare can inspect content | They only see ciphertext — useless without MEK |
| DDoS via public endpoint | Cloudflare's DDoS protection (included free) |
| Bucket enumeration | File names are visible, but contents are encrypted. Use opaque names if sensitive. |

## Architecture Diagram

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

## Cost Impact

- **Storage:** ~$6-7/TB/month (B2)
- **Egress:** $0 (Cloudflare Bandwidth Alliance)
- **API calls:** $0 (after May 2026)
- **Cloudflare:** $0 (free tier sufficient for most use cases)

## References

- [Cloudflare Bandwidth Alliance](https://www.cloudflare.com/bandwidth-alliance/)
- [Backblaze B2 S3 Compatible API](https://www.backblaze.com/docs/cloud-storage-b2-storage-s3-compatible-api)
- [Cloudflare Caching](https://developers.cloudflare.com/cache/concepts/cache-control/)
