# ADR-013: Read throughput is bounded by unpipelined per-block ranged GETs

**Status:** Proposed
**Date:** 2026-08-08

## Context

Read throughput through a live ARMOR instance was measured for the first time
on 2026-08-08, while diagnosing why a Forgejo backup job was taking hours. The
numbers were not what anyone had assumed, and the gap is large enough to change
recovery-time expectations.

All figures below were measured from a pod on `iad-ci` against the production
`armor` deployment and the `iad-ci` B2 bucket in `us-west-002`. Object sizes are
real backup tarballs, not synthetic where noted.

### Measurements

| Path | Throughput | Notes |
|---|---|---|
| Cluster → Cloudflare (nearby anycast), download | ~100 MB/s | baseline; network is not the constraint inbound |
| Cluster → Cloudflare, upload | ~8 MB/s | egress is shaped ~12:1 against inbound |
| Direct B2 S3 read (`s3.us-west-002.backblazeb2.com`) | **~38 MB/s** | 154 MB object; billed egress |
| Cloudflare read (`ARMOR_CF_DOMAIN`, `/file/<bucket>/<key>`) | **~7.9 MB/s** | same 154 MB object; free egress (Bandwidth Alliance) |
| **Read through the ARMOR service** | **~1.5 MB/s** | 169 MB in 141 s; 154 MB in 86 s |
| Write through the ARMOR service | ~4 MB/s | vs ~5.5 MB/s direct B2 write — ~27% overhead |

Latency to `us-west-002` from `us-east-iad-1`: **74 ms** TCP connect, 151 ms to
TLS-complete. The bucket is cross-country from every cluster that reads it.

ARMOR's CPU during a read-heavy transfer peaked at **464m of its 1000m limit**
(~46%), so it is **not** CPU-throttled. Memory stayed near 230Mi of 1Gi.

### Mechanism

`internal/backend/b2.go`, `GetRangeWithHeaders` issues **one HTTP ranged GET per
block** against the Cloudflare download URL:

```go
cfURL := fmt.Sprintf("https://%s/file/%s/%s", b.cfDomain, bucket, url.PathEscape(key))
req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))
```

With `ARMOR_BLOCK_SIZE = 65536` and no pipelining, throughput is bounded by
`blockSize / RTT`:

```
65536 bytes / 0.074 s ≈ 0.89 MB/s   (strictly serial)
observed:              ~1.5 MB/s    (implies ~1–2 blocks in flight)
```

That arithmetic matching the observation is the basis for attributing the
shortfall to serialization rather than to bandwidth, Cloudflare, or B2.

Writes escape this because multipart parts are 64 MiB — 1024 blocks amortised
into one streamed PUT — which is why the write path shows only ~27% overhead
while the read path loses ~80%.

### Why this went unnoticed

Reads through ARMOR are rare in normal operation. The system is used as a backup
target: writes are continuous, reads happen only during verification or an
actual restore. `restore-verifier`'s `ModeDRDrill` — the path that matters for
disaster recovery, per [ADR-009](009-restore-verifier-armor-path-never-decrypts.md)
— exercises `verifyObjectDirectOnly`, which reads raw ciphertext from the
backend and decrypts locally. It therefore never exercises the service read path
at production object sizes, and would not have surfaced this.

## Decision

1. **Pipeline block fetches on the read path.** Issue N ranged GETs concurrently
   (N configurable, default 8–16) and reassemble in order. This is a bounded
   change confined to `GetRangeWithHeaders` and its callers; it does not touch
   the on-disk format, so it applies to every object already stored.

2. **Keep Cloudflare as the default read path.** The 5× throughput advantage of
   direct S3 is not worth paying egress for routine reads, and the CF path's
   ~7.9 MB/s is roughly 5× above what the service currently achieves — so
   pipelining, not switching networks, is where the win is.

3. **Do not raise `ARMOR_BLOCK_SIZE`.** It would help throughput arithmetically
   but changes the stored format and cannot apply to existing objects.
   Pipelining is strictly better value at no compatibility cost.

4. **Accept that `armor decrypt` uses the billed direct-S3 endpoint** for now,
   and document it rather than silently spending egress. Closing that gap is
   tracked separately; it is not a correctness issue, and in a genuine disaster
   the faster path is arguably the right one anyway.

## Consequences

**Expected improvement:** service reads should approach the Cloudflare path's
~7.9 MB/s ceiling, roughly a 5× gain. For the 8.7 GB Forgejo repo set that moves
a full restore through the service from ~1.6 hours to ~18 minutes.

**What this does not change:** disaster recovery does not depend on the service
read path at all. `armor decrypt` reads raw ciphertext and decrypts locally, and
was verified end to end on 2026-08-08 — a 12 MB object decrypted in 3 s with its
plaintext SHA-256 verified, producing a git repository whose HEAD matched
production exactly. The RTO figure that matters is unaffected by this ADR.

**Risk of the change:** concurrent range requests increase memory proportional to
`N × blockSize` (16 × 64 KiB = 1 MiB — negligible against the 1Gi limit) and
raise request rate against Cloudflare. Ordering must be preserved on reassembly,
and partial failures must fail the whole read rather than yielding a hole —
per-block HMACs would catch silent corruption, but a truncated read must not be
mistaken for a short object.

**Measurement caveat:** the direct-B2 figure varied between runs (13.6 MB/s on
one object, 38 MB/s on another), so absolute numbers carry real variance. The
robust findings are the *ratios* — CF is several times slower than direct S3,
and the service is several times slower than CF — and the block/RTT arithmetic
that explains the latter.
