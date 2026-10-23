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

| Write version | Object shape | Backend calls | Sequence |
|---|---|---|---|
| v1 | single-PUT | **5** | manifest GET (404 miss) → HEAD → header GetRange → HMAC-table GetRange → data GetRange |
| v2 | single-PUT | **5** | same as v1 (v1/v2 share the inline-HMAC read path) |
| v3 | single-PUT | **5** | manifest GET (404 miss) → HEAD → header GetRange → trailer block-table GetRange → data GetRange |
| v3 | multipart | **3 + 1/part** | manifest GET (hit) → combined freshness/size HEAD → HMAC sidecar GetDirect → one GetRange per part |

The table reflects armor-c3199f1c (2026-10-23): GetObject reads the envelope
header once and passes it to the downstream helpers (previously the header was
read twice on every single-PUT GET), and the multipart path's ADR-016 freshness
check and ciphertext-size lookup share one authoritative HEAD (previously two
HEADs of the same object). Range reads additionally reuse the v3 trailer block
table across requests through a stored-generation-keyed cache — not visible in
a first-GET pin; repeated same-version ranges skip the trailer fetch entirely.

Notes for anyone lowering these:

- Call 1 is the `readManifest` GET, which **misses for every non-multipart
  object** — a pure wasted round trip on the single-PUT path.
- The HEAD exists to distinguish the legacy (v1/v2/inline-HMAC) path from
  the v3 path before either is committed.
- Multipart cannot be "small" (ADR-015 requires uniform parts ≥ 5 MiB), so
  its row pins the fixed per-GET overhead, not a small-object end to end.

Test names: `TestSmallObjectFullGETRequestCountSinglePUTV1/V2/V3`,
`TestSmallObjectFullGETRequestCountMultipartV3`.

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

Environment: codinghome, go1.26.5 linux/amd64, 2026-10-15 (clean source
export). RTT = **74 ms** per backend call
(the measured cluster→`us-west-002` round trip in
[docs/plan/plan.md](../plan/plan.md) ADR-013 note;
`ARMOR_SMALL_OBJECT_BENCH_RTT_MS` overrides). v2 and v3 writes, 64 KiB
blocks, AES-256-CTR + HMAC-SHA256, and the block and footer caches configured
by the test harness.

p50 / p95 (ms) and inverse mean latency per client (shown as /s) per cell.
These rates are not aggregate throughput. **n = max(2×concurrency, 32) GETs
per cell, each GET verified to make exactly 6 backend calls**:

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

Reading:

- **p50 ≈ 6 × RTT = 444 ms at every size and both envelope versions.** The
  bead's source-inspection estimate (~370 ms at 5 calls) was one round trip
  short: the measured count is 6.
- Object size is irrelevant below ~1 MiB — the round trips are the latency,
  exactly the claim this baseline pins.
- Per-client latency stays near six RTTs as concurrency rises for small
  objects. At 1 MiB and high concurrency it rises modestly; this in-memory
  backend has no bandwidth term, so local compute and scheduling account for
  the added time.
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

## Using this baseline

- A bead that removes a round trip updates the pins in
  `get_request_count_test.go`, re-runs both benchmarks, re-harvests one probe
  cycle, and updates every table here in the same change.
- Never quote the 2026-08-08 ADR-013 figures, a local microbenchmark, or a
  single probe cycle as "current throughput" — re-run the harness
  ([performance/README.md](README.md)).
