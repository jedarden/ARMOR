# Bandwidth Alliance and Zero-Egress Partners

## How the Bandwidth Alliance Works

The Bandwidth Alliance is a coalition of cloud and CDN providers (founded by Cloudflare, 2018) that discount or waive egress fees for shared customers.

### Technical Mechanism: Private Network Interconnection (PNI)

1. Cloudflare and partner providers are **co-located in the same data center facilities**
2. They connect via **direct fiber (PNI)** — no transit provider middleman
3. Zero marginal cost for data transfer over these settlement-free peering links
4. Both companies pass the zero cost through to customers

This is not a promotional discount — it's a structural economic reality of shared infrastructure.

### Backblaze Peering Infrastructure (AS40401)

| Exchange Point | Location | Capacity |
|---|---|---|
| AMS-IX | Amsterdam | 100G |
| Equinix Ashburn | Virginia | 100G |
| Equinix San Jose | California | 100G |
| TorIX | Toronto | 2x 100G |

Total network capacity: 500-1000 Gbps. Peering policy: selective.

---

## Complete Partner List

### CDN Partners (Zero B2 Egress)

| Partner | Egress Cost | Notes |
|---------|-------------|-------|
| **Cloudflare** | $0 | Founding Alliance member; direct PNI; all plan tiers including free |
| **Fastly** | $0 | Direct PNI |
| **bunny.net** | $0 | 74+ global edge locations |
| **CacheFly** | $0 | Direct PNI |

### Compute/Infrastructure Partners (Zero B2 Egress)

| Partner | Egress Cost | Notes |
|---------|-------------|-------|
| **Vultr** | $0 | Cloud compute |
| **Cherry Servers** | $0 | Bare metal |
| **CoreWeave** | $0 | GPU compute |
| **Equinix Metal** | $0 | Bare metal (formerly Packet) |
| **phoenixNAP** | $0 | Bare metal/cloud |

### Other Alliance Members (via Cloudflare)
Oracle, Alibaba Cloud, Zenlayer, Tencent Cloud, DreamHost, Scaleway, DigitalOcean (discounted), Google Cloud (75% reduction via CDN Interconnect), Microsoft Azure.

---

## Cloudflare + B2 Integration (Primary Focus)

### Traffic Flow

#### Cache Hit (most common after warm-up)
```
Client → Cloudflare Edge PoP → [Cache Hit] → Client
```
Zero B2 contact. Zero egress. Zero API calls.

#### Cache Miss
```
Client → Cloudflare Edge PoP → [Miss] → PNI → B2 Origin
B2 Origin → PNI → Cloudflare Edge → [Cache Store] → Client
```
B2 egress is free because traffic stays on settlement-free peering link. One Class B API call consumed.

### Setup Requirements

#### 1. DNS (CNAME)
```
files.example.com  CNAME  f004.backblazeb2.com  (proxied/orange cloud)
```

#### 2. SSL/TLS
- Set to **Full (strict)** — B2 requires HTTPS
- **Disable Automatic Signed Exchanges (SXGs)** — incompatible with B2

#### 3. URL Rewrite (Transform Rule)
B2 requires `/file/<bucket-name>/` prefix. Create a rewrite rule:
- Condition: `not starts_with(http.request.uri.path, "/file/<bucket-name>")`
- Dynamic rewrite: `concat("/file/<bucket-name>", http.request.uri.path)`

#### 4. Response Header Cleanup
Strip B2 headers that leak origin info:
- `x-bz-file-name`, `x-bz-file-id`, `x-bz-content-sha1`
- `x-bz-upload-timestamp`, `x-bz-info-src_last_modified_millis`

#### 5. Cache-Control
Set `cache-control: public, max-age=31536000` for successful responses.
B2's default `max-age=0, no-cache, no-store` only applies to errors.

#### 6. CORS (if browser access needed)
Match file extensions and set `Access-Control-Allow-Origin` as needed.

### Cloudflare R2 vs B2 + Cloudflare CDN

| Dimension | Cloudflare R2 | B2 + Cloudflare CDN |
|-----------|--------------|---------------------|
| Storage | $0.015/GB/mo | **$0.006/GB/mo (2.5x cheaper)** |
| Egress | $0 (always) | $0 (through Cloudflare only) |
| Class A ops | $4.50/million | **Free** |
| Class B ops | $0.36/million | Free (after May 2026) |
| Non-CF egress | $0 | $0.01/GB |
| Setup complexity | Simple | Requires CNAME + Transform Rules |
| S3 portability | Cloudflare-native | S3-compatible, portable |
| Content restrictions | None | CF self-serve ToS applies |

**B2 wins** for storage-heavy workloads (2.5x cheaper/GB) and multi-cloud portability.
**R2 wins** for simpler setup and zero egress without CDN requirement.

### Critical Limitations and Gotchas

1. **Cloudflare Self-Serve ToS (biggest risk):** Self-serve accounts may not use CDN to serve "video or a disproportionate percentage of pictures, audio files, or other large files" without paid services (Stream, Images, Developer Platform). Term "disproportionate" is undefined. **Content served through Workers (a compute service) appears compliant.** B2+Cloudflare is still actively promoted by both companies.

2. **SSL must be Full (Strict):** Cloudflare defaults to Flexible which breaks B2.

3. **Authorization header kills caching:** Cloudflare bypasses cache for requests with `Authorization` header, even with `cacheEverything: true`. Workaround: Workers with Cache API and stripped cache keys.

4. **B2 data center proximity:** Limited to US-West (Sacramento), US-East (Reston), EU-Central (Amsterdam), Canada (Toronto). Cache miss latency depends on proximity.

5. **Public buckets for CDN:** Private content requires Workers or signed URLs for auth.

6. **No wildcard header removal:** B2 `x-bz-*` headers must be removed individually.

---

## Other Zero-Egress Pathways

| Pathway | Egress Included | Requirements |
|---------|----------------|-------------|
| 3x free monthly egress | 3x average stored data | None (all pay-as-you-go) |
| CDN/Compute partners | Unlimited | Route through partner network |
| B2 Reserve | Unlimited to all destinations | 20TB+ annual commitment |
| B2 Overdrive | Unlimited, 1 Tbps sustained | Multi-petabyte, $15/TB/mo |
