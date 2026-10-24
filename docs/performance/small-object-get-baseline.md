# Small-Object Full-GET Baseline — Request Counts and Latency

Established 2026-09-28 by [armor-50a36688]. Why it exists: commitgraph is
moving its ranking reads to many small per-repo Parquet objects served by
ARMOR (~620k objects, median ~23 KB; commitgraph bead `commitgr-ff1e5255`).
For objects that small, full-GET latency is set by the number of **sequential
backend round trips** the read path makes, not by bandwidth. This document
records the three halves of that baseline so later beads can lower the round
trip count *deliberately*, with a pinned test that fails when the count rises
and published before/after numbers.

| Half | What it measures | Where it lives |
|---|---|---|
| Request counts | Backend calls per full GET, per envelope shape | Pinned tests: `internal/server/handlers/get_request_count_test.go` |
| Latency-injected | Per-object latency with a fixed artificial RTT per backend call | `internal/server/handlers/small_object_get_bench_test.go` (explicit-run) |
| Production | Real full GETs of real objects from inside the cluster | `armor-get-probe` Deployment (see below) |

**Read-path code is unchanged by the baseline bead.** No optimization is
claimed here; these are the numbers to beat.

## 1. Backend request counts per full GET (pinned)

`TestSmallObjectFullGETRequestCount*` in
`internal/server/handlers/get_request_count_test.go` drives one full-object
GET through the HTTP surface over a call-recording backend and asserts the
exact call sequence. A change that adds a round trip fails CI; a change that
removes one updates the pin **and this table in the same bead**.

| Write version | Object shape | Backend calls | Critical path | Sequence |
|---|---|---|---|---|
| v1 | single-PUT ≤ threshold | **3** | **2** | manifest GET (404 miss) ∥ HEAD → one whole-envelope GetRange (header + data + inline HMAC table fetched together, parsed locally) |
| v2 | single-PUT ≤ threshold | **3** | **2** | same as v1 (v1/v2 share the inline-HMAC layout) |
| v3 | single-PUT ≤ threshold | **3** | **2** | manifest GET (404 miss) ∥ HEAD → one whole-envelope GetRange (header + data + trailer block table fetched together, parsed locally) |
| v1/v2/v3 | single-PUT > threshold | **5** | **4** | the pre-armor-98864f12 streaming sequence: header GetRange → HMAC-table/trailer GetRange → data GetRange, unchanged |
| v3 | multipart | **5** | **4** | manifest GET (hit) ∥ plaintext-key HEAD (hit, discarded) → combined freshness/size HEAD → HMAC sidecar GetDirect → one GetRange covering the whole concatenated ciphertext |

The threshold is `ARMOR_SMALL_OBJECT_FETCH_BYTES` (default 1 MiB) against the
**stored** ciphertext size the GET's HEAD reports; `0` restores the streaming
reads everywhere, and multipart objects never single-fetch.

The table reflects armor-98864f12 (2026-10-24): a full GET of a single-PUT
object at or below the threshold reads the entire stored envelope in ONE
ranged request and slices header, data blocks and the inline HMAC table or
v3 trailer block table out of that buffer locally, replacing two further
sequential round trips with extra bytes. Every block's HMAC is verified as
before; a truncated, over-long, or wrongly-ranged response fails the request.
Buffered fetches are capped at `ARMOR_READ_BACKEND_LIMIT` live at once, so
aggregate buffered ciphertext is bounded at that cap × the threshold.

Before that, armor-420c15a5 (2026-10-24) had made GetObject issue the manifest
probe and the legacy plaintext-key HEAD concurrently and then decide exactly
as the sequential path did — an existing manifest wins and the HEAD result is
discarded, a miss consumes it. The overlap costs one extra backend request on
the manifest path (the multipart row went 4 → 5, its probe HEAD discarded) and
removes one sequential round trip from every single-PUT read: the first two
calls are one round trip of wall time, not two. Previously (armor-c3199f1c,
2026-10-23): GetObject read the envelope header once and passed it to the
downstream helpers, and the multipart path's ADR-016 freshness check and
ciphertext-size lookup shared one authoritative HEAD. armor-272299ca
(2026-10-24) replaced the multipart GET's one-range-per-part fetch with a
single ranged request over the whole stored object. Range reads additionally
reuse the v3 trailer block table across requests through a
stored-generation-keyed cache — not visible in a first-GET pin; repeated
same-version ranges skip the trailer fetch entirely. Since armor-98864f12 the
steady-state benefit of that cache applies to over-threshold full GETs and to
range reads; a small in-threshold full GET never fetches the table at all.

Notes for anyone lowering these:

- Call 1 is the `readManifest` GET, which **misses for every non-multipart
  object**. Since armor-420c15a5 it overlaps the HEAD instead of serializing
  in front of it, so its cost on the critical path is zero on the legacy
  path; the request-count tests prove the overlap deterministically (the
  guarded manifest GET blocks until a HEAD is recorded).
- The HEAD exists to distinguish the legacy (v1/v2/inline-HMAC) path from
  the v3 path before either is committed. armor-98864f12 investigated
  dropping it for small objects and found it cannot go yet: the envelope
  header carries only version, block size, IV, plaintext size and plaintext
  SHA, while the read path also needs the wrapped DEK, MEK fingerprint and
  key ID (key unwrap must precede any header use), the response-side
  ContentType/ETag/LastModified, the per-object compression decision
  (Compressed/CompressionType), the v3 CiphertextSize, and the multipart
  flag — all HEAD-only today. Any change that moves them needs its own bead
  and must preserve armor-c3199f1c's freshness rules.
- Multipart cannot be "small" (ADR-015 requires uniform parts ≥ 5 MiB), so
  its row pins the fixed per-GET overhead, not a small-object end to end.

Test names: `TestSmallObjectFullGETRequestCountSinglePUTV1/V2/V3`,
`TestSmallObjectFullGETRequestCountMultipartV3`,
`TestOverThresholdObjectKeepsStreamingPath` (the > threshold row).

## 2. Latency-injected benchmark

`TestSmallObjectFullGETLatencyMatrix` in
`internal/server/handlers/small_object_get_bench_test.go` reuses the counting
backend with a fixed sleep before every call, so per-object latency is set
purely by the pinned round trip count. It skips in every normal gate.

Command (as run for the numbers below):

```sh
ARMOR_SMALL_OBJECT_BENCH=1 go test ./internal/server/handlers/ \
  -run TestSmallObjectFullGETLatencyMatrix -v -count=1 -timeout 45m
```

Environment: codinghome, linux/amd64 (2026-10-24 matrix: go1.25.14, the
go.mod toolchain; 2026-10-15 matrix: go1.26.5). RTT =
**74 ms** per backend call (the measured cluster→`us-west-002` round trip in
[docs/plan/plan.md](../plan/plan.md) ADR-013 note;
`ARMOR_SMALL_OBJECT_BENCH_RTT_MS` overrides). v2 and v3 writes, 64 KiB
blocks, AES-256-CTR + HMAC-SHA256, and the block and footer caches configured
by the test harness.

**2026-10-24 (single fetch, armor-98864f12).** Steady state: every cell
GETs one hot object; every in-threshold cell makes exactly 3 backend calls
per GET — the overlapped manifest probe ∥ HEAD pair, then one whole-envelope
ranged request parsed locally (no warm-up term: even v3's trailer table is
sliced out of the fetched buffer, so the generation-keyed table cache is
never engaged). The matrix's threshold clears the largest cell's stored
size and its buffer-slot cap clears c256 (both knobs set by the harness —
see `runBenchCell` — so the cells measure the single-fetch path itself;
production's 1 MiB default and the cap-overload streaming fallback are
pinned by the boundary and cap tests, not measured here). Two A/B cells
(32 KiB / c16, `mode=streaming`) run with the single fetch disabled in the
same run at the same RTT. p50 / p95 (ms) and inverse mean latency per client
(shown as /s); these rates are not aggregate throughput; **n =
max(2×concurrency, 32) GETs per cell**:

| Write version | Size | c=1 | c=16 | c=64 | c=256 |
|---|---|---|---|---|---|
| v2 | 4 KiB | 148.6 / 149.0 · 6.73/s | 149.6 / 150.6 · 6.68/s | 157.4 / 161.2 · 6.38/s | 161.6 / 176.2 · 6.11/s |
| v2 | 32 KiB | 149.0 / 149.8 · 6.71/s | 149.1 / 150.1 · 6.70/s | 154.8 / 160.9 · 6.42/s | 170.6 / 198.8 · 5.71/s |
| v2 | 256 KiB | 149.6 / 152.4 · 6.66/s | 153.5 / 160.8 · 6.42/s | 178.8 / 205.3 · 5.58/s | 205.4 / 245.0 · 4.90/s |
| v2 | 1 MiB | 152.7 / 154.0 · 6.52/s | 157.2 / 199.7 · 5.68/s | 185.9 / 215.2 · 5.23/s | 273.9 / 336.5 · 3.62/s |
| v3 | 4 KiB | 148.5 / 148.7 · 6.73/s | 148.6 / 148.7 · 6.73/s | 150.8 / 152.4 · 6.64/s | 155.3 / 160.5 · 6.44/s |
| v3 | 32 KiB | 148.6 / 149.2 · 6.72/s | 150.1 / 151.5 · 6.66/s | 155.4 / 166.2 · 6.34/s | 164.5 / 174.2 · 6.06/s |
| v3 | 256 KiB | 150.0 / 152.1 · 6.66/s | 152.8 / 154.4 · 6.55/s | 191.8 / 201.2 · 5.24/s | 182.6 / 200.9 · 5.49/s |
| v3 | 1 MiB | 153.6 / 160.6 · 6.45/s | 164.3 / 189.4 · 5.71/s | 218.2 / 242.2 · 4.64/s | 270.3 / 331.9 · 3.71/s |

A/B legs from the same run (`mode=streaming`, single fetch disabled):
v2 32 KiB c16 p50 = 297.4 ms / p95 = 298.3 ms · 3.36/s; v3 32 KiB c16
p50 = 224.1 ms / p95 = 298.8 ms · 3.83/s (the v3 p95 tail is the one
first-GET trailer-table fetch the streaming path still pays).

**2026-10-24 (overlapped probe, armor-420c15a5).** Steady state: every cell
GETs one hot object, so v3's trailer block table is generation-keyed
cache-resident after the first GET (armor-c3199f1c) — the pin verifies v2
makes 5 backend calls per GET and v3 makes 4 plus exactly one first-GET table
fetch. p50 / p95 (ms) and inverse mean latency per client (shown as /s);
these rates are not aggregate throughput; **n = max(2×concurrency, 32) GETs
per cell**:

| Write version | Size | c=1 | c=16 | c=64 | c=256 |
|---|---|---|---|---|---|
| v2 | 4 KiB | 296.9 / 297.0 · 3.37/s | 298.0 / 298.8 · 3.36/s | 299.8 / 300.9 · 3.34/s | 308.1 / 315.4 · 3.25/s |
| v2 | 32 KiB | 296.9 / 297.2 · 3.37/s | 298.0 / 298.7 · 3.36/s | 299.7 / 302.5 · 3.33/s | 305.3 / 310.8 · 3.27/s |
| v2 | 256 KiB | 297.7 / 298.3 · 3.36/s | 299.1 / 300.8 · 3.34/s | 304.3 / 307.3 · 3.29/s | 315.6 / 328.8 · 3.17/s |
| v2 | 1 MiB | 299.5 / 300.1 · 3.34/s | 303.4 / 306.6 · 3.29/s | 314.2 / 323.0 · 3.19/s | 362.8 / 404.1 · 2.77/s |
| v3 | 4 KiB | 222.7 / 222.8 · 4.44/s | 223.0 / 297.3 · 3.85/s | 224.5 / 298.3 · 3.84/s | 232.2 / 303.9 · 3.77/s |
| v3 | 32 KiB | 222.7 / 223.2 · 4.44/s | 224.1 / 298.4 · 3.83/s | 228.2 / 302.6 · 3.80/s | 245.0 / 310.9 · 3.70/s |
| v3 | 256 KiB | 223.9 / 225.5 · 4.42/s | 226.6 / 301.7 · 3.80/s | 236.5 / 311.0 · 3.72/s | 273.0 / 327.1 · 3.52/s |
| v3 | 1 MiB | 225.8 / 226.4 · 4.38/s | 235.0 / 315.1 · 3.68/s | 250.7 / 328.3 · 3.56/s | 321.9 / 409.4 · 3.07/s |

**2026-10-15 (pre-overlap, 6-call sequential read path), kept as the
before-numbers:**

| Write version | Size | c=1 | c=16 | c=64 | c=256 |
|---|---|---|---|---|---|
| v2 | 4 KiB | 445.2 / 445.3 · 2.25/s | 446.4 / 447.1 · 2.24/s | 447.7 / 449.3 · 2.23/s | 450.9 / 453.7 · 2.22/s |
| v2 | 32 KiB | 445.3 / 446.1 · 2.24/s | 447.2 / 447.7 · 2.24/s | 448.2 / 449.2 · 2.23/s | 450.8 / 454.8 · 2.22/s |
| v2 | 256 KiB | 446.0 / 447.7 · 2.24/s | 447.1 / 448.6 · 2.23/s | 452.0 / 454.4 · 2.21/s | 453.0 / 472.7 · 2.19/s |
| v2 | 1 MiB | 447.9 / 450.0 · 2.23/s | 453.3 / 456.4 · 2.20/s | 456.0 / 469.1 · 2.18/s | 474.6 / 514.3 · 2.09/s |
| v3 | 4 KiB | 445.2 / 445.3 · 2.25/s | 445.4 / 445.5 · 2.25/s | 446.3 / 446.4 · 2.24/s | 450.2 / 451.9 · 2.22/s |
| v3 | 32 KiB | 445.3 / 445.7 · 2.25/s | 447.3 / 448.0 · 2.24/s | 448.0 / 449.3 · 2.23/s | 453.7 / 458.6 · 2.20/s |
| v3 | 256 KiB | 446.1 / 446.9 · 2.24/s | 448.3 / 450.6 · 2.23/s | 452.9 / 456.3 · 2.21/s | 456.6 / 475.9 · 2.18/s |
| v3 | 1 MiB | 448.3 / 449.2 · 2.23/s | 455.1 / 457.6 · 2.20/s | 463.6 / 475.1 · 2.16/s | 485.4 / 535.7 · 2.04/s |

Reading (2026-10-24 single-fetch matrix):

- **p50 ≈ 2 × RTT = 148 ms at every version and size** — one whole-envelope
  request after the overlapped probe pair. Against the same-day
  armor-420c15a5 matrix (v2 4 × RTT = 296 ms, v3 3 × RTT = 222 ms) the small
  cells dropped ~148 ms (−50%) on v2 and ~74 ms (−33%) on v3, and per-client
  rates rose from ~3.3/s to ~6.7/s (v2) and ~4.4/s to ~6.7/s (v3) at c=1.
  The same-run A/B legs (v2 streaming 297.4 ms, v3 streaming 224.1 ms at
  32 KiB / c16) isolate the single fetch as the entire remaining delta.
- Object size is nearly irrelevant to latency below 256 KiB; the residual
  size term (v2 1 MiB c=256 p50 273.9 ms vs 4 KiB 161.6 ms) is this
  in-memory backend's decrypt + HMAC + copy of the whole buffered envelope,
  not round trips.
- The v3 streaming p95 ≈ 298 ms tail is gone from the single-fetch cells:
  no table fetch happens at all, so there is no warm-up outlier to absorb.
- At 1 ms injected RTT the same matrix runs in seconds and is a quick
  sanity check when re-pinning counts.

## 3. Production baseline (in-cluster probe)

The production leg is the `armor-get-probe` Deployment in the `commitgraph`
namespace of `ord-devimprint`
(`declarative-config/k8s/ord-devimprint/commitgraph/armor-get-probe-deployment.yaml`,
deployed by declarative-config commit `9957c17f`, 2026-09-28):

- **Key list supplied by manifest, never by listing the prefix.** The
  keylist container queries commitgraph's Postgres read replica — the same
  `repos`/`repo_scan_state` record `pkg/verification/parquet_reader.go`
  builds its keys from — for 6 003 existing `commits.parquet` artifacts
  (provider=github, 1–500 commits, random order). The probe issues **no
  ListObjects call of any kind**.
- **Read-only by construction**: the probe container's only S3 verb is GET;
  the SQL runs inside `BEGIN READ ONLY` against the `-ro` replica.
- Pure-stdlib Python SigV4 client, **exactly one HTTP request per object**
  (no client-side stat/prefetch), keep-alive connections per worker.
- A **Deployment with an internal scheduling loop** (org rule: never a Job),
  one cycle every 6 h; keys refresh daily.

Environment: probe pod in `commitgraph` ns → `armor.devimprint.svc:9000`
(in-cluster, HTTP). ARMOR `ronaldraygun/armor:0.1.1975@sha256:85a7d883…`, 12
replicas, `ARMOR_PREFIX=commitgraph/`, 64 KiB blocks, v3 writes, block-size
cache 1000 entries/300 s. Object set per cycle cell is a random sample of
real artifacts; bodies are plaintext sizes (p50 ≈ 13–15 KB, ~0.87 of the
sample ≤ 64 KiB).

The probe did not inspect the envelope version of each pre-existing object.
The configured v3 write default does not establish the format of every object
in this set, so these production observations are version-unattributed.

Harvest command (results are the `[armor-get-probe]` JSON lines):

```sh
kubectl --server=http://traefik-ord-devimprint:8001 \
  logs -n commitgraph deploy/armor-get-probe -c probe
```

First full cycle, 2026-09-28 06:57–07:02 UTC (300 / 800 / 1 600 GETs at
c=1 / 8 / 32), all objects and the ≤ 64 KiB subpopulation:

| Write version | Cell | n (ok) | errors | objs/s | p50 ms | p95 ms | p99 ms | ≤64 KiB p50 / p95 / p99 ms |
|---|---|---|---|---|---|---|---|---|
| uninspected | c=1 | 295 | 2×404, 3×500 | 1.55 | 587.9 | 952.9 | 1418.8 | 563.3 / 868.7 / 1303.5 |
| uninspected | c=8 | 799 | 1×500 | 12.22 | 582.8 | 976.3 | 1604.9 | 569.3 / 884.1 / 1161.7 |
| uninspected | c=32 | 1600 | 0 | 44.79 | 599.9 | 1029.5 | 1459.0 | 577.2 / 928.1 / 1242.8 |

Reading:

- **Production small-object p50 is ~0.56–0.60 s at every concurrency**, close
  to the latency-injected 6 × 74 ms = 444 ms model. The ~120–155 ms gap and
  the wider tail are the real B2 latency distribution (74 ms is a mean, not a
  ceiling), proxy CPU, and TLS-free but real network hops. The round trip
  count explains the bulk of the latency: **the model is validated**.
- Throughput scales with concurrency (12.2 objs/s at c=8, 44.8 at c=32
  across 12 ARMOR pods) — per-object latency is flat, so a commitgraph reader
  that wants more objects/s gets them by widening, not by waiting.
- Error rates: 6 non-200 responses in 2 694 GETs (0.22%) — 2×404 (sampled
  repos whose artifact is not (yet) present; excluded from timing) and 4×500
  (transient ARMOR/backend errors). Watch, don't alert, at this level.

**Probe disposition (explicit decision, armor-50a36688): kept.** It costs
~70 mCPU idle and one ~5-minute cycle per 6 h, and gives the small-object
beads that depend on this baseline a standing regression signal. Remove it
by deleting the manifest file in declarative-config (ArgoCD prunes with it).

**Pending re-harvest (armor-420c15a5):** the §2 matrices above were re-run
2026-10-24 against the overlapped-probe read path, but the production table
still reflects the 0.1.1975 sequential read path. One probe cycle is to be
harvested after the release carrying armor-420c15a5 rolls to
ord-devimprint, and the table above replaced with its numbers (expect
small-object p50 to fall by roughly one 74 ms round trip for
single-PUT objects, plus the c3199f1c header-dedup saving that also shipped
unmeasured).

## Using this baseline

- A bead that removes a round trip updates the pins in
  `get_request_count_test.go`, re-runs both benchmarks, re-harvests one probe
  cycle, and updates every table here in the same change.
- Never quote the 2026-08-08 ADR-013 figures, a local microbenchmark, or a
  single probe cycle as "current throughput" — re-run the harness
  ([performance/README.md](README.md)).
