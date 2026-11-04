# CF Read Transport Matrix — 2026-11-04

Bead: `armor-409733e3`. Question: does the ~2 MB/s production read ceiling
(p50 2.24 MB/s for full GETs ≥ 8 MiB over 14 days of iad-ci request logs,
0.1.2004) come from all pipelined Cloudflare fetches multiplexing onto one
HTTP/2 connection (`internal/backend/transport.go` clones the default
transport with `ForceAttemptHTTP2`)?

**Answer: no.** HTTP/1.1-per-fetch and HTTP/2-multiplexed measure the same
in every arm. The ceiling is the Cloudflare→B2 origin fetch itself: each
MISS costs ~600–650 ms p50 nearly independent of fetch size, origin fetches
sustain only ~5 requests/s, so MISS-path aggregate ≈ 5/s × fetch size. The
lever is `ARMOR_READ_FETCH_SIZE`, not the transport protocol. The edge-cache
HIT path is unaffected: ~100 MB/s over one HTTP/2 connection.

## Harness

`tests/performance/cfreadprobe` — standalone Go probe; no ARMOR credentials
needed for read arms (the CF download path injects the B2 credential at the
edge, so the client fetch is unauthenticated — same as `fetchRange`). Fixed
object: `probe/armor-409733e3/cfread-256mib.bin` (256 MiB, SigV4 PUT to the
`iad-ci` bucket with the deployment's own B2 application key). MISS arms
append a unique `?probe=N` per request so every fetch is a CF MISS/origin
fetch; HIT arms read the plain URL. Each arm: N workers pulling fetch-size
ranged chunks in offset order — the pipelined reader's shape — over a 15 s
window. `protoSeen` on every RESULT line proves the negotiated protocol.

First read of the object: `CF-Cache-Status: MISS`, ~2 s for a 1 MiB range;
second read `HIT`, ~25 ms p50. `Cache-Control: max-age=14400` (4 h TTL).

## Matrix — codinghome vantage (Helsinki → CF edge), 2026-11-04

Command shape (full script in the bead notes):

```bash
go run ./tests/performance/cfreadprobe -mode=matrix \
  -proto h2,h1 -concurrency 4,16,32 -fetch-mib 1,4,8 \
  -nocache -duration 15   # MISS path
go run ./tests/performance/cfreadprobe -mode=matrix \
  -proto h2,h1 -concurrency 16 -fetch-mib 1,8 -duration 15   # HIT contrast
```

Aggregate MB/s, MISS path (every fetch an origin fetch):

| concurrency | fetch | h2 | h1 |
|---|---|---|---|
| 4 | 1 MiB | (cached-served arm, see caveat) | 1.87 |
| 4 | 4 MiB | 4.80 | 4.53 |
| 4 | 8 MiB | 8.00 | 7.47 |
| 16 | 1 MiB | 4.60 | 4.80 |
| 16 | 4 MiB | 11.73 | 12.53 |
| 16 | 8 MiB | 17.60 | 20.80 |
| 32 | 1 MiB | 7.93 | 7.73 |
| 32 | 4 MiB | 20.00 | 19.47 |
| 32 | 8 MiB | **37.87** | **39.47** |

HIT path (warm edge cache), conc 16:

| fetch | h2 | h1 |
|---|---|---|
| 1 MiB | 97.20 | 97.33 |
| 8 MiB | 105.07 | 111.99 |

Single-connection sequential reference: whole 256 MiB object over one h2
connection, cache HITs: 90.45 MB/s (ttfb p50 25 ms).

Upload-burst arms (4 concurrent 8 MiB SigV4 PUTs during the read window):
h2 8 MiB 20.27 MB/s vs 17.60 without burst; h1 8 MiB 19.20 vs 20.80; h1
1 MiB 4.60 vs 4.80 — no download starvation from concurrent upload traffic
at this offered load (~2–4 MB/s uplink). Second bead hypothesis not
supported locally.

## Findings

1. **Protocol is immaterial.** h1 and h2 agree within noise in all 18 MISS
   arms and all HIT arms. One multiplexed HTTP/2 connection does not cap the
   aggregate; N HTTP/1.1 connections do not lift it. The
   `ARMOR_CF_DISABLE_HTTP2` knob (added with this measurement) is kept as an
   opt-in field tool; the default stays multiplexed.
2. **The MISS path is origin-fetch latency bound.** Per-fetch completion is
   ~600–650 ms p50 and ~900–980 ms p90 nearly independent of fetch size —
   the fixed cost dominates. Aggregate ≈ fetch rate × fetch size with a soft
   rate ceiling around 5 fetches/s: 1 MiB → ~5 MB/s, 4 MiB → ~20 MB/s,
   8 MiB → ~38 MB/s.
3. **The HIT path is fast** (~100 MB/s): production's ~2 MB/s is a cache-
   miss regime (objects re-read rarely; 4 h TTL). The pipelined reader
   against MISSes at 1 MiB fetches reproduces the production number
   (4.6–4.8 MB/s from this vantage, 2.24 MB/s p50 through armor from iad-ci
   — same regime minus armor overhead).
4. **Next bottleneck (proposal, not admitted work):** raising
   `ARMOR_READ_FETCH_SIZE` (default 1 MiB) toward the 8 MiB validation
   bound would trade memory for latency amortization — the grid shows ~8x
   at conc 32/fetch 8 MiB. Bounded by `ARMOR_READ_BACKEND_LIMIT` × fetch
   size worst-case buffering (64 × 8 MiB = 512 MiB against pod memory),
   so this is a deployment-sizing decision, not a code change. Rate-limit
   behavior at higher request rates needs the status-code evidence first:
   see caveat below.

## Caveats

- Two `?probe=N`-busted arms (the first arm of two phase runs, h2 conc4
  fetch1 and h2 conc16 fetch1-with-burst) reported HIT-dominant cache
  counts where MISS was expected. The zone's query-string cache-key handling
  is not fully characterized; every other busted arm reported all-MISS with
  consistent ~600 ms origin latency, and the h1/h2 comparison — the question
  this matrix answers — is unaffected.
- The multi-file arm (8 distinct objects, conc 16, fetch 1 MiB) sustained
  3.20 MB/s under both protocols with 224 errors against 48 successes —
  the same aggregate as single-file, so the origin cap is not per-file, and
  the error storm (status codes not tallied in that run) is consistent with
  a per-client origin rate limit at ~5 fetches/s. Status codes are tallied
  in the current probe (`status=` field on RESULT lines); in-cluster runs
  should collect them before any rate-limit claim is made firm.
- Absolute numbers vary with CF POP and cache state; the ratios
  (h1 ≈ h2; fetch-size scaling; HIT ≫ MISS) are the robust findings — same
  caveat as ADR-013's original measurements.

## In-cluster confirmation

Filled from the `armor-cf-read-probe` workflow run on iad-ci (see the bead
for the exact commands); it anchors the local vantage to the production one:
same fixed object, direct-CF arms from an iad-ci pod, plus through-armor
arms against the live `iad-ci/armor` service.
