# Cloudflare + B2 Architecture Patterns

## Cloudflare Workers with B2

### Reference Implementations

**1. `cloudflare-b2` (read-only proxy)**
- Accepts unsigned requests from end users
- Signs them with AWS V4 via `aws4fetch` library
- Forwards to private B2 bucket
- Three bucket routing modes: fixed, path-based, host-based
- Range request retry logic for files >2GB
- Handles Cloudflare HEAD→GET conversion quirk

**2. `cloudflare-b2-proxy` (full S3 proxy)**
- Validates incoming AWS V4 signatures
- Re-signs requests for B2
- Optional async webhook notifications
- Clients use standard S3 SDKs pointed at Worker endpoint

### Workers Pricing
| Tier | Requests | CPU Time | Cost |
|------|----------|----------|------|
| Free | 100K/day | 10ms/invocation | $0 |
| Paid | 10M/month + $0.50/M overage | 30M CPU-ms/month | $5/month |

No data transfer charges from Workers.

---

## CDN Caching Strategy

### The Authorization Header Problem
Cloudflare **automatically bypasses cache** for any request with an `Authorization` header, even with `cacheEverything: true`.

**Workarounds:**
1. **Workers Cache API** — Strip auth from cache key, validate separately
2. **HMAC token validation** — Use URL-embedded tokens instead of headers
3. **Enterprise `cacheKey` option** — Ignore auth header in cache key

### Cache Configuration

#### B2 Default Behavior
B2 does not set `Cache-Control` headers by default (changed Sept 2021). You must set them explicitly:
- On the B2 bucket: set `Cache-Control` in bucket info
- On individual files: set during upload
- Via Cloudflare: override with Cache Rules or Workers

#### Recommended Cache Rules
```
Cache-Control: public, max-age=86400    # 1 day for mutable content
Cache-Control: public, max-age=31536000 # 1 year for immutable/versioned content
```

#### Cache Purge Strategies
- **By URL:** `POST /client/v4/zones/{zone}/purge_cache` with specific URLs
- **By tag:** Enterprise only
- **Everything:** Available but expensive (cold cache = all requests hit B2)

---

## Security Architecture

### Recommended: Private Bucket + Worker Proxy

```
┌──────────┐     ┌──────────────────┐     ┌──────────────┐
│  Client   │────▶│ Cloudflare Worker │────▶│ B2 (private) │
│           │◀────│  (holds B2 creds) │◀────│              │
└──────────┘     └──────────────────┘     └──────────────┘
```

- B2 bucket set to `allPrivate`
- Worker is the **only** entity with B2 credentials
- No credentials exposed to clients
- Worker validates client auth before proxying

### Application Key Scoping (Principle of Least Privilege)

| Key Scope | Capabilities | Use Case |
|-----------|-------------|----------|
| Read-only, single bucket | `readFiles`, `listFiles` | Download Worker |
| Write-only, single bucket, prefix | `writeFiles` | Upload service |
| Full, single bucket | All file ops | Admin/management |

### Signed URL Patterns

| Method | Expiry | Implementation |
|--------|--------|---------------|
| B2 S3 pre-signed URL | 1s – 7 days | `generate_presigned_url()` via boto3 |
| B2 native auth token | 24 hours | `b2_get_download_authorization` |
| Custom HMAC token (Worker) | Configurable | Worker validates HMAC signature on URL |

### Layering Cloudflare Access/Zero Trust
- Place Cloudflare Access in front of the Worker
- User authenticates via SSO/OIDC before reaching Worker
- Worker receives validated JWT from Access
- Enables per-user access control without custom auth

---

## Upload Path Architecture

### Recommendation: Direct to B2 (Not Through Cloudflare)

**Why:**
- B2 ingress is **always free** (no upload egress fees)
- Cloudflare body size limits: 100MB (free), 500MB (enterprise)
- Workers CPU time is consumed on upload proxying
- No caching benefit for uploads

### Pattern: Server-Generated Pre-signed Upload URLs

```
┌──────────┐     ┌──────────────┐     ┌──────────────┐
│  Client   │────▶│  Your Server  │     │              │
│           │◀────│ (generates    │     │     B2       │
│           │     │  presigned    │     │              │
│           │─────│──URL)─────────│────▶│              │
└──────────┘     └──────────────┘     └──────────────┘
  1. Request URL    2. Return URL       3. Direct upload
```

1. Client requests upload URL from your server
2. Server generates S3 pre-signed PUT URL (scoped to specific key, expiry)
3. Client uploads directly to B2 using the pre-signed URL
4. CORS rules on B2 bucket allow browser uploads

### Large File Uploads
- Files >5 GB require S3 multipart upload
- Pre-signed URLs can be generated for each part
- Client orchestrates multipart upload directly to B2
- B2 does **not** support tus protocol natively

---

## Architecture Patterns

### Pattern 1: Encrypted File Storage (Our Target)

```
UPLOAD:
Client → encrypt locally → direct PUT to B2 (pre-signed URL)

DOWNLOAD:
Client → Cloudflare Worker → B2 (private) → Cloudflare cache → Client → decrypt locally
```

- All encryption/decryption happens on the client
- B2 stores opaque encrypted blobs
- Cloudflare caches encrypted content (safe — it's opaque)
- Worker handles auth and B2 credential injection
- **Zero egress fees** (all downloads through Cloudflare)
- **Zero knowledge** (neither B2 nor Cloudflare can read content)

### Pattern 2: Web Application Image CDN

```
Browser → files.example.com → Cloudflare CDN → B2 (public)
```

- Public bucket, CNAME + Transform Rules
- Long cache TTLs for immutable assets
- Lowest cost, simplest setup

### Pattern 3: Private File Sharing (HMAC Signed URLs)

```
App Server generates signed URL → Client fetches → Worker validates HMAC → B2
```

- Time-limited access to private content
- No B2 credentials exposed to clients
- Worker validates HMAC before proxying

### Pattern 4: Backup/Archive (No CDN)

```
Server → encrypt → direct upload to B2
Server → direct download from B2 → decrypt
```

- No Cloudflare in the path
- Use 3x free egress allowance or B2 Reserve for unlimited egress
- Simpler architecture for server-to-server workloads

### Pattern 5: Multi-Bucket Routing

```
Worker receives request → routes to correct B2 bucket based on:
  - Hostname (virtual hosting)
  - URL path prefix
  - Request headers
```

- Single Worker handles multiple buckets
- Each bucket can have different access policies

---

## Terms of Service Considerations

### Cloudflare Self-Serve ToS
Cloudflare restricts serving "video or a disproportionate percentage of pictures, audio files, or other large files" from external origins through CDN on self-serve plans without using paid services (Stream, Images, Developer Platform).

**Mitigations:**
- Content served through **Workers** (a compute service) appears compliant
- **Encrypted blobs** are not "pictures" or "video" — they're opaque binary data
- B2+Cloudflare is **actively promoted** by both companies
- Enterprise plan has no such restriction
- R2 is the nuclear option if ToS becomes an issue

### For Our Use Case
Our encrypted file storage pattern routes through Workers (compute) and serves opaque encrypted blobs (not media files). This should be fully compliant with Cloudflare's ToS.
