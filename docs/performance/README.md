# ARMOR Performance Runbook

Reproducible, bounded throughput baselines for the ARMOR service read and
write paths. This replaces the one-off 2026-08-08 measurements in
[ADR-013](../adr/013-read-throughput-unpipelined-block-fetches.md) with a
repeatable harness — **never treat the ADR-013 figures (or any single past
run) as current production throughput**; re-run the harness.

## Layout

| Path | What it is |
|---|---|
| `tests/performance/` | The harness (Go package): instrumented in-process service, scenario matrix, deterministic shape tests, opt-in remote targets |
| `tests/performance/counting_backend.go` | Backend wrapper counting per-op requests/bytes and tracking max in-flight concurrency, with injectable per-call latency |
| `tests/performance/localenv.go` | Boots the real authenticated S3 mux (`server.NewWithBackend`) over a filesystem backend on a loopback `httptest` listener |
| `tests/performance/matrix.go` | The bounded scenario matrix, metrics, verification, `results.json` / `results.md` output |
| `tests/performance/harness_test.go` | CI-safe deterministic tests (request counts, tail-block selectivity, one-PUT-per-part, overlap detector, compressed-path boundary) |
| `tests/performance/baseline_test.go` | `TestRecordedBaseline` — the opt-in measurement run |
| `tests/performance/remote_test.go` | Opt-in remote targets: real ARMOR service, Cloudflare read path, labeled direct-backend comparison |

## What is measured

Scenario families (all against the v3 envelope, 64 KiB blocks, AES-GCM):

- **Reads** — full GET (cold = first GET of the key after its write; warm =
  repeated GETs of the same key), and range GETs: block-aligned,
  unaligned, and suffix (`bytes=-N`). Each sample is hash-verified against
  the payload that was written.
- **Writes** — v3 single-PUT; v3 multipart (sequential parts within one
  upload, 8 MiB parts ≥ the ADR-015 5 MiB uniform minimum); concurrent
  multipart across 4 clients. Every acknowledged write is verified by
  re-reading the object through the **ordinary service GET** (full object,
  SHA-256 compared) — never by direct backend decryption only.
- **Client concurrency** — 1 client (base scenarios) and 4 concurrent
  clients (each with its own key).
- **Compression** — the ADR-007 single-PUT compression path runs as a
  separate in-process service (`Compress: true`, compressible payloads;
  ranges and multipart are unsupported there by design). See the known
  findings below before trusting compressed read rows.
- **Object sizes** — 64 KiB (one block), 1 MiB, 64 MiB by default. 1 GiB is
  opt-in only (`ARMOR_PERF_SIZES=…,1GiB`) to keep the default run bounded.

Per scenario the harness records: payload MB/s (aggregate, p50, p95),
time-to-first-byte p50/p95, completion latency p50/p95, backend
GET/range + PUT/part request counts and bytes (deltas), max in-flight
backend concurrency, CPU seconds and allocation bytes for the scenario
(client and server share the process), peak RSS for the run, and peak
temporary-disk bytes behind the backend. Every result carries its sample
count.

## Running the local baseline

No credentials, no network, no B2. The service runs in-process over a
temp filesystem backend (honors `TMPDIR`):

```bash
# bounded default matrix (~15 s): 64KiB/1MiB/64MiB, writes verified via service read-back
ARMOR_PERF_RUN=1 go test ./tests/performance/ -run TestRecordedBaseline -v -timeout 40m

# results (JSON + markdown) land in:
#   $ARMOR_PERF_OUT/results.{json,md}   (default: <os.TempDir>()/armor-perf-results)
```

Knobs (all optional):

| Variable | Default | Meaning |
|---|---|---|
| `ARMOR_PERF_SIZES` | `64KiB,1MiB,64MiB` | comma list (`KiB`/`MiB`/`GiB`; 1 GiB opt-in) |
| `ARMOR_PERF_READ_SAMPLES` | `7` | warm-read samples per read scenario |
| `ARMOR_PERF_WRITE_SAMPLES` | `3` | samples per write scenario |
| `ARMOR_PERF_PART` | `8MiB` | uniform multipart part size (≥ 5 MiB, ADR-015) |
| `ARMOR_PERF_MULTIPART_CLIENTS` | `4` | clients in the concurrent-multipart scenario |
| `ARMOR_PERF_NO_COMPRESS` | unset | `1` skips the compressed-path scenarios |
| `ARMOR_PERF_OUT` | `<tmp>/armor-perf-results` | output directory |
| `ARMOR_PERF_COMMIT` | VCS build info | commit stamp when running from an export (exports carry no VCS metadata) |

**Ordinary CI never runs measurement.** `go test ./... -short` skips the
baseline and remote tests entirely; `harness_test.go` adds only fast,
deterministic shape tests (no timing thresholds anywhere in CI paths).

### Interpretation boundary

The local baseline measures ARMOR's **per-request service overhead envelope**
(crypto, request handling, backend call shapes) on loopback + local disk. It
is the stable yardstick for optimization work — same-machine reruns compare
directly. It is **not** production throughput: no network, no B2, no
Cloudflare. Production-shaped numbers require the remote mode below,
originally from a pod on `iad-ci` per ADR-013's methodology.

## Remote measurements (opt-in, never in CI)

```bash
# real ARMOR service (SigV4; credentials by environment reference only)
AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... \
ARMOR_PERF_ENDPOINT=https://<armor-host>:2443 \
ARMOR_PERF_BUCKET=<bucket> ARMOR_PERF_REGION=<region> \
go test ./tests/performance/ -run TestRemoteBaseline -v -timeout 60m

# add the Cloudflare read path — reads the SAME ciphertext object the service
# uploaded (like-for-like); records CF-Cache-Status per sample (HIT / MISS /
# EXPIRED / DYNAMIC reported separately, large objects may exceed CDN limits
# and stay MISS/DYNAMIC)
ARMOR_PERF_CF_BASE=https://<cf-domain>/file/<bucket> ...

# add the explicitly labeled direct-backend comparison — billed egress,
# results are recorded under the name "direct-backend-egress-billed" and
# exist only to contextualize the CF/service ratios (ADR-013)
ARMOR_PERF_DIRECT_S3=https://s3.<region>.backblazeb2.com ...
```

Every remote download is hash-verified against a payload the run uploaded
itself under the dedicated `armor-bench/` prefix; cleanup deletes exactly the
keys the run created. Results go to `$ARMOR_PERF_OUT/remote-results.json`.

Credentials travel only as environment references. No secret value, and no
unpublished object identifier, is ever written to results or logs.

## Recorded baseline — 2026-09-18

Measured from a clean export of ARMOR `4c062f0a` (the harness itself lands in
the following commit; nothing in the harness changed the measured code).
Raw JSON: produced by `ARMOR_PERF_RUN=1 ARMOR_PERF_COMMIT=4c062f0a
go test ./tests/performance/ -run TestRecordedBaseline` — 29 scenarios,
10.7 s wall.

- Environment: codinghome, linux/amd64, GOMAXPROCS=20 (i5-13500, 14 cores),
  Go 1.25.0, loopback + local temp filesystem backend
- Peak RSS 550 MiB; peak temp-disk 1225 MiB (harness-owned, removed after)
- Sample counts are per-row; p50/p95 over per-sample values

| scenario | B | n | cl | agg MB/s | p50 MB/s | p95 MB/s | TTFB p50 ms | compl p50 ms | p95 ms | verified |
|---|---|---|---|---|---|---|---|---|---|---|
| read-full-cold/v3/64kib | 64K | 1 | 1 | 100.6 | 100.6 | 100.6 | 0.55 | 0.62 | 0.62 | ✓ |
| read-full-warm/v3/64kib | 64K | 7 | 1 | 87.7 | 94.5 | 108.4 | 0.61 | 0.66 | 1.05 | ✓ |
| read-range-aligned/v3/64kib | 32K | 7 | 1 | 47.8 | 42.3 | 71.6 | 0.69 | 0.74 | 0.79 | ✓ |
| read-range-unaligned/v3/64kib | 32K | 7 | 1 | 58.5 | 56.3 | 65.7 | 0.50 | 0.55 | 0.60 | ✓ |
| read-range-suffix/v3/64kib | 64K | 7 | 1 | 108.9 | 107.2 | 147.7 | 0.52 | 0.58 | 0.73 | ✓ |
| write-single-put/v3/64kib | 64K | 3 | 1 | 72.0 | 75.3 | 82.5 | — | 0.83 | 1.02 | ✓ |
| read-warm/v3/64kib/clients=4 | 64K | 28 | 4 | 199.5 | 58.8 | 99.2 | 0.93 | 1.05 | 1.39 | ✓ |
| read-full-cold/v3/1mib | 1M | 1 | 1 | 341.2 | 341.2 | 341.2 | 0.65 | 2.93 | 2.93 | ✓ |
| read-full-warm/v3/1mib | 1M | 7 | 1 | 397.0 | 398.2 | 451.2 | 0.46 | 2.51 | 2.83 | ✓ |
| read-range-aligned/v3/1mib | 64K | 7 | 1 | 84.7 | 88.0 | 103.7 | 0.64 | 0.71 | 0.92 | ✓ |
| read-range-unaligned/v3/1mib | 32K | 7 | 1 | 37.7 | 36.4 | 58.2 | 0.82 | 0.86 | 1.30 | ✓ |
| read-range-suffix/v3/1mib | 64K | 7 | 1 | 73.6 | 70.8 | 116.8 | 0.82 | 0.88 | 1.28 | ✓ |
| write-single-put/v3/1mib | 1M | 3 | 1 | 104.5 | 107.1 | 113.7 | — | 9.34 | 10.58 | ✓ |
| read-warm/v3/1mib/clients=4 | 1M | 28 | 4 | 942.1 | 290.3 | 327.4 | 0.85 | 3.41 | 6.93 | ✓ |
| read-full-cold/v3/64mib | 64M | 1 | 1 | 489.3 | 489.3 | 489.3 | 0.51 | 130.80 | 130.80 | ✓ |
| read-full-warm/v3/64mib | 64M | 7 | 1 | 541.3 | 557.0 | 587.5 | 0.53 | 114.90 | 129.44 | ✓ |
| read-range-aligned/v3/64mib | 64K | 3 | 1 | 109.9 | 109.7 | 115.9 | 0.50 | 0.57 | 0.60 | ✓ |
| read-range-unaligned/v3/64mib | 32K | 3 | 1 | 61.6 | 61.0 | 63.0 | 0.46 | 0.51 | 0.51 | ✓ |
| read-range-suffix/v3/64mib | 64K | 3 | 1 | 134.4 | 137.4 | 141.6 | 0.39 | 0.46 | 0.50 | ✓ |
| write-single-put/v3/64mib | 64M | 3 | 1 | 233.1 | 229.2 | 250.8 | — | 279.22 | 289.29 | ✓ |
| read-warm/v3/64mib/clients=4 | 64M | 28 | 4 | 1443.7 | 367.7 | 432.3 | 0.64 | 164.41 | 226.02 | ✓ |
| write-multipart/v3/64mib (8×8MiB) | 64M | 3 | 1 | 51.7 | 95.9 | 126.6 | — | 590.00 | 769.21 | ✓ |
| write-multipart-concurrent/v3/64mib/clients=4 | 64M | 2 | 4 | 74.5 | 100.2 | 301.0 | — | 639.29 | 917.83 | ✓ |
| compressed rows (6 scenarios) | — | — | — | see known finding F1 | | | | | | ✗ |

Headline local-shape facts (useful comparators for optimization beads):

- Full-GET service reads sustain **~540 MB/s** at 64 MiB single-client
  locally, **~1.4 GB/s** aggregate at 4 clients — the service layer is not
  the throughput ceiling; the 2026-08-08 production figures were network/
  block-fetch-bound, not crypto-bound.
- Range reads are selective: a 64 KiB suffix of a 64 MiB object costs ~0.5 ms
  and 15 backend calls, not a full-object fetch (pinned by
  `TestSuffixRangeFetchesOnlyTail`).
- Multipart write (sequential parts) is ~4.2× slower than single-PUT per byte
  locally (51.7 vs 233.1 MB/s agg) — per-part overhead dominates at 8 MiB
  parts on local disk; concurrent clients only partially amortize it (74.5
  MB/s at 4 clients).

## Known findings at record time (pre-existing on `4c062f0a`)

- **F1 — compressed read path broken.** Full GET of a compressed object
  returns 200 but the payload fails SHA-256 verification; server logs
  `decompression streaming error: HMAC table too short at block 0 (zstd)`.
  ARMOR's own `share_decompress` tests fail on the same clean export. The
  compressed rows above are recorded honestly (verified=false). A separate
  bead should track the repair; rerun `ARMOR_PERF_RUN=1` after it lands to
  fill the compressed rows.
- **F2 — concurrent part uploads to one upload ID refused.** Uploading
  multiple parts of the same multipart upload concurrently can fail with
  503 on committed main. The harness therefore models write concurrency at
  the client level; revisit if concurrent same-upload parts are ever
  supported.

## Cleanup and credentials policy

- All benchmark objects live under the dedicated `armor-bench/` key prefix;
  local cleanup deletes exactly that prefix plus the temp backend directory.
  Remote cleanup deletes exactly the keys the run uploaded.
- Credentials only ever travel as environment variable references
  (`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` fetched from the operator's
  own store); they are never recorded in results, logs, or this document.
- The local harness's credentials are the throwaway in-process SigV4
  constants from `internal/server/srvtest` — they authenticate nothing.
