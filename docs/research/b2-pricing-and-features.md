# Backblaze B2 Pricing and Features

## Pricing (Pay-As-You-Go)

### Storage
- **$6/TB/month** (increases to **$6.95/TB/month** effective May 1, 2026)
- First 10 GB free forever
- No minimum file size or storage duration fees

### Egress/Downloads
- Free egress up to **3x monthly average stored data**
- Beyond free tier: $0.01/GB ($10/TB)
- **Unlimited free egress** through CDN/compute partners: Cloudflare, Fastly, bunny.net, CacheFly, CoreWeave, Equinix Metal, Vultr, Cherry Servers, phoenixNAP

### API Transactions (Current, Before May 1 2026)
| Class | Operations | Free Tier | Paid Rate |
|-------|-----------|-----------|-----------|
| **A** (writes) | PutObject, UploadPart, CreateMultipartUpload, CompleteMultipartUpload, AbortMultipartUpload, DeleteObject(s) | Unlimited | Free |
| **B** (reads) | GetObject, HeadObject, GetObjectRetention, GetObjectLegalHold | 2,500/day | $0.004/10,000 |
| **C** (listing) | ListBuckets, ListObjectsV2, ListObjectVersions, CopyObject, CreateBucket | 2,500/day | $0.004/1,000 |

### API Transaction Fee Elimination (May 1, 2026)
**All standard API calls become free.** Class B and Class C transaction fees are eliminated entirely. Only Event Notification transactions retain separate pricing. Storage price increases from $6 to $6.95/TB to offset.

**Net impact for our use case:** Positive. High-read workloads (frequent downloads, listing) save significantly more on transaction fees than the $0.95/TB storage increase costs.

### Committed Tiers
| Tier | Price | Egress | Min Commitment |
|------|-------|--------|---------------|
| **B2 Reserve** | ~$1,560/yr for 20TB (1-3yr terms) | Unlimited to all destinations | 20TB |
| **B2 Overdrive** | $15/TB/month | Unlimited, up to 1 Tbps sustained | Multi-petabyte |

---

## S3-Compatible API

### Endpoints
- Format: `https://s3.<region>.backblazeb2.com`
- Path-style: `https://s3.<region>.backblazeb2.com/<bucket-name>/key`
- Virtual-hosted-style also supported
- **HTTPS only** (non-secure connections rejected)

### Regions
| Region | S3 Identifier | Location |
|--------|--------------|----------|
| US West | `us-west-001`, `us-west-002`, `us-west-004` | Sacramento + Phoenix |
| US East | (available) | Reston, Virginia |
| EU Central | `eu-central-003` | Amsterdam |
| Canada East | (newest) | Toronto |

### Supported S3 Operations
- **Objects:** PutObject, GetObject, HeadObject, DeleteObject(s), CopyObject
- **Multipart:** CreateMultipartUpload, UploadPart, CompleteMultipartUpload, AbortMultipartUpload, ListParts, ListMultipartUploads
- **Buckets:** CreateBucket, DeleteBucket, HeadBucket, ListBuckets
- **Listing:** ListObjectsV2, ListObjectVersions
- **ACL:** GetBucketAcl, PutBucketAcl (private and public-read only)
- **Encryption:** SSE-B2 and SSE-C headers
- **Object Lock:** Full support (Governance and Compliance modes)
- **Pre-signed URLs:** Supported for uploads and downloads

### S3 API Limitations
- **No SSE-KMS** (only SSE-B2 and SSE-C)
- **No IAM roles**
- **No object tagging** (GetObjectTagging returns empty)
- **No website configuration**
- **No object-level ACLs** (bucket-level only: private or public-read)
- **100 buckets per account maximum**
- **No key management** (must use B2 native API or web UI)
- **Single region per account** (cannot create cross-region buckets)
- **AWS Signature V4 only** (V2 not supported)

---

## Native API vs S3-Compatible API

| Aspect | B2 Native API | S3-Compatible API |
|--------|--------------|-------------------|
| Auth | `b2_authorize_account` → 24hr token | AWS Signature V4 |
| Key Management | Full (create/delete/list keys) | Not supported |
| Upload Flow | Requires `b2_get_upload_url` first; URL can go stale | Static URLs; server handles routing |
| Pre-signed URLs | Download authorization tokens | Standard S3 pre-signed URLs |
| SDKs | Official Java + Python | AWS SDKs in all languages |
| Ecosystem | Narrow, deep B2 control | Broad, drop-in S3 replacement |

**Recommendation:** Use S3-compatible API as the primary interface (broad SDK support, familiar tooling), with native API only for key management operations.

---

## Encryption Options

### SSE-B2 (Backblaze-Managed Keys)
- AES-256, B2 manages all keys
- Each file gets a unique key, wrapped by a global key
- Can be set as bucket default
- No additional cost

### SSE-C (Customer-Managed Keys)
- AES-256, customer provides key with each request
- B2 stores only a hash (cannot decrypt without customer key)
- No additional cost
- Encrypted files cannot be downloaded from web UI

### Key Points
- SSE disabled by default on new buckets
- Both Native and S3 APIs support SSE-B2 and SSE-C
- **No SSE-KMS support**
- Encryption is per-file (can mix encrypted/unencrypted in same bucket)

---

## Application Keys and Authentication

### Key Types
| Type | Scope | Use Case |
|------|-------|----------|
| **Master Key** | Full access, all buckets, all capabilities | Account administration |
| **Standard Key** | Scoped to bucket(s), prefix, capabilities, expiration | Application access |

### Capability Scoping
Keys can be restricted to specific combinations of:
- Bucket(s) and file name prefix
- Operations: `listFiles`, `readFiles`, `writeFiles`, `deleteFiles`, `shareFiles`
- Encryption: `readBucketEncryption`, `writeBucketEncryption`
- Retention: `readFileRetentions`, `writeFileRetentions`, `bypassGovernance`
- Expiration time

### S3 Auth Mapping
- Application Key ID → S3 Access Key ID
- Application Key → S3 Secret Access Key

---

## Buckets

- **allPublic:** Anyone can download without auth
- **allPrivate:** All downloads require authorization (default)
- Max 100 buckets per account
- Names globally unique across all B2 accounts
- Support CORS rules (up to 100 per bucket)
- Custom Cache-Control headers configurable
- Object Lock must be enabled at creation (cannot add later)

## Lifecycle Rules
- **daysFromUploadingToHiding:** Auto-hide after N days
- **daysFromHidingToDeleting:** Auto-delete hidden files after N days
- **daysFromStartingToCancelingUnfinishedLargeFiles:** Auto-cancel incomplete uploads
- Scoped by file name prefix
- Cannot override Object Lock retention
