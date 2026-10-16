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
| `tests/performance/large_range_test.go` | `TestLargeObjectRangeReadBaseline` — focused large-object range latency and backend-byte measurement |
| `tests/performance/remote_test.go` | Opt-in remote targets: real ARMOR service, Cloudflare read path, labeled direct-backend comparison |
| [small-object-get-baseline.md](small-object-get-baseline.md) | The small-object full-GET baseline (armor-50a36688): per-GET backend request counts pinned by tests, the latency-injected matrix, and the in-cluster `armor-get-probe` production numbers |
| [large-object-range-read-baseline.md](large-object-range-read-baseline.md) | The focused large-object range baseline: exact plaintext verification, warm-metadata latency, backend fetch counts/bytes, and interpretation limits |
| [production-b2-cloudflare-2026-09-28.md](production-b2-cloudflare-2026-09-28.md) | First live B2/Cloudflare qualification result: workload, deployment, failed large-object qualification, and release-status boundary |

## What is measured

Scenario families use the selected v2 or v3 single-PUT envelope (v3 by
default), 64 KiB blocks, and AES-256-CTR + HMAC-SHA256. Multipart scenarios
use v3. Every local result records its effective write version.

- **Reads** — full GET (cold = first GET of the key after its write; warm =
  repeated GETs of the same key), and range GETs: block-aligned,
  unaligned, and suffix (`bytes=-N`). Each sample is hash-verified against
  the payload that was written.
- **Writes** — selected-version single-PUT; v3 multipart (sequential parts within one
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

The focused large-object range run uses one 64 MiB object by default and
measures a block-aligned 64 KiB read, an unaligned 32 KiB read, aligned 1 MiB
and 8 MiB reads, and a 64 KiB suffix. It warms each range once, compares every
sample with the exact plaintext slice, and writes
`large-range-results.{json,md}`. Set `ARMOR_PERF_LARGE_OBJECT=1GiB` to make the
same run exercise a 1 GiB object. The v3 run also measures multipart ranges,
including a read across a part boundary. Set `ARMOR_PERF_BACKEND_RTT_MS` to
inject a fixed delay per backend call for a controlled latency comparison.

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
# verified uncompressed matrix: 64KiB/1MiB/64MiB, writes checked by service read-back
ARMOR_PERF_RUN=1 ARMOR_PERF_NO_COMPRESS=1 go test ./tests/performance/ -run TestRecordedBaseline -v -count=1 -timeout 40m

# v2 comparison, explicitly labelled in every output row
ARMOR_PERF_RUN=1 ARMOR_PERF_NO_COMPRESS=1 ARMOR_PERF_WRITE_VERSION=2 go test ./tests/performance/ -run TestRecordedBaseline -v -count=1 -timeout 40m

# focused large-object range report (default: one 64 MiB object, five samples)
ARMOR_PERF_RUN=1 go test ./tests/performance/ -run TestLargeObjectRangeReadBaseline -v -count=1 -timeout 40m

# focused v2 comparison
ARMOR_PERF_RUN=1 ARMOR_PERF_WRITE_VERSION=2 go test ./tests/performance/ -run TestLargeObjectRangeReadBaseline -v -count=1 -timeout 40m

# results (JSON + markdown) land in:
#   $ARMOR_PERF_OUT/results.{json,md}   (default: <os.TempDir>()/armor-perf-results)
```

Knobs (all optional):

| Variable | Default | Meaning |
|---|---|---|
| `ARMOR_PERF_SIZES` | `64KiB,1MiB,64MiB` | comma list (`KiB`/`MiB`/`GiB`; 1 GiB opt-in) |
| `ARMOR_PERF_WRITE_VERSION` | `3` | local single-PUT format: `2` or `3`; the v3 run also measures multipart |
| `ARMOR_PERF_READ_SAMPLES` | `7` | warm-read samples per read scenario |
| `ARMOR_PERF_WRITE_SAMPLES` | `3` | samples per write scenario |
| `ARMOR_PERF_PART` | `8MiB` | uniform multipart part size (≥ 5 MiB, ADR-015) |
| `ARMOR_PERF_MULTIPART_CLIENTS` | `4` | clients in the concurrent-multipart scenario |
| `ARMOR_PERF_BACKEND_RTT_MS` | `0` | fixed per-backend-call delay in the focused large-range run only |
| `ARMOR_PERF_NO_COMPRESS` | unset | `1` runs the currently verified uncompressed comparison; unset includes the compressed diagnostic rows |
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

## Production measurements (opt-in, never in CI)

`TestRemoteBaseline` is the production-shaped workflow. It is deliberately
single-client and bounded: one 64 MiB object by default, three serialized
single-PUT uploads, five warm full reads, and three samples of each aligned,
unaligned, multi-block, and suffix range. It exercises the same object through
three paths:

| Target | What it measures | Payload verified against |
|---|---|---|
| `armor-service` | authenticated ARMOR upload and plaintext GET/range, including ARMOR's B2/Cloudflare read path | synthetic plaintext |
| `b2-origin` | direct B2 S3 GET/range of the encrypted object created by ARMOR | exact ciphertext fetched once from B2 |
| `cloudflare` | public `/file/<bucket>/<key>` GET/range of that same encrypted object | exact B2 ciphertext |

The B2 and Cloudflare rows are intentionally ciphertext transport rows; they
must not be compared with ARMOR plaintext bytes without accounting for the v3
envelope. Each acknowledged upload is verified by an untimed ordinary ARMOR
GET. Each measured response checks status, byte count, and SHA-256 (and each
range checks the exact requested slice). Results include p50/p95 latency and
throughput, time to first byte, HTTP status counts, and `CF-Cache-Status`.

For the representative `iad-ci/armor` deployment, run from a tailnet-capable
host. Keep credential values in the operator's secret store; the assignments
below are placeholders, not values to paste into a transcript:

```bash
AWS_ACCESS_KEY_ID=<armor-auth-access-key> \
AWS_SECRET_ACCESS_KEY=<armor-auth-secret-key> \
ARMOR_PERF_B2_ACCESS_KEY_ID=<b2-key-id> \
ARMOR_PERF_B2_SECRET_ACCESS_KEY=<b2-application-key> \
ARMOR_PERF_ENDPOINT=https://<armor-s3-host>:8444 \
ARMOR_PERF_B2_ENDPOINT=https://s3.<region>.backblazeb2.com \
ARMOR_PERF_CF_BASE=https://<cf-domain> \
ARMOR_PERF_BUCKET=<bucket> ARMOR_PERF_REGION=<region> \
ARMOR_PERF_OUT=/tmp/armor-production-results \
go test ./tests/performance/ -run '^TestRemoteBaseline$' -count=1 -v -timeout 90m
```

`ARMOR_PERF_CF_BASE` may instead include `/file/<bucket>`; the harness accepts
both forms. `ARMOR_PERF_B2_KEY_PREFIX` is required when ARMOR's configured
`ARMOR_PREFIX` makes the physical B2 key differ from the S3 key. The old
`ARMOR_PERF_DIRECT_S3` variable remains an alias for
`ARMOR_PERF_B2_ENDPOINT`. Use `ARMOR_PERF_SIZES=128MiB` or `1GiB` to change the
large-object shape only after confirming the deployment and runner have enough
memory and time. `ARMOR_PERF_UPLOAD_SAMPLES`, `ARMOR_PERF_READ_SAMPLES`, and
`ARMOR_PERF_RANGE_SAMPLES` tune the bounded sample counts.

The run creates keys only below `armor-bench/production/<run-id>/`, deletes
exactly those keys in a deferred cleanup, and writes
`remote-results.{json,md}` under `ARMOR_PERF_OUT`. Do not commit raw results
that contain private endpoints or object names. A release-status entry should
record the date, source commit, deployment image/digest, runner location,
region/bucket, workload, sample counts, the generated result summary, and the
limits below.

### Production interpretation limits

- This is one runner and one deployment replica, not a load test, SLO, or
  capacity claim. It does not measure concurrent clients or multipart upload.
- ARMOR upload numbers include request handling, encryption, and the B2 PUT;
  direct B2-origin download rows measure ciphertext transport. Cloudflare rows
  include the selected edge and current cache state.
- A B2 reference fetch is required to verify CDN bytes and is itself an extra
  direct-B2 read outside the reported samples. B2-origin downloads may incur
  provider egress; use the smallest acceptable sample count.
- Cache state, edge selection, WAN path, B2 contention, deployment load, and
  object lifecycle can change between runs. `CF-Cache-Status` is evidence for
  that run, not a permanent cache guarantee.
- The v3 object envelope and the ARMOR prefix are production configuration;
  record both. Raw B2/Cloudflare response size is not plaintext object size.

The first live qualification against `iad-ci/armor` is recorded in
[production-b2-cloudflare-2026-09-28.md](production-b2-cloudflare-2026-09-28.md);
it produced no valid throughput numbers because the large-object verification
GET did not deliver its expected bytes before the bounded timeout.

## Focused large-object range baseline

The measured focused baseline and its limits are in
[large-object-range-read-baseline.md](large-object-range-read-baseline.md).
It is intentionally separate from the full read/write matrix because range
work is dominated by the requested span, block alignment, metadata/table
fetches, and backend round trips rather than whole-object throughput.

For `armor-5694713b`, `armor-98864f12`, `armor-272299ca`, and
`armor-73564f97`, before/after throughput claims must cite this harness's
command and report rows. Record the `write_version`, write path, object and
range sizes, sample counts, injected RTT, source commit, and environment on
both sides. Use the v3 single-PUT and multipart rows for changes to those
paths; use the [small-object request-count and latency suite](small-object-get-baseline.md)
for small full-GET changes. The original 2026-09-18 local rows were v2
despite their `v3` labels and cannot serve as v3 before numbers.

## Corrected local matrix baseline — 2026-10-15

The 2026-09-18 matrix was labelled v3 but the local server wrote v2. Its old
v3 rows are withdrawn. These replacement runs used a clean export of server
source `4ba4cb11` with only the `armor-2af339bc` performance harness changes
overlaid; the server source was unchanged. Host: codinghome, Linux amd64,
Go 1.26.5, GOMAXPROCS=20; loopback and temporary filesystem backend,
64 KiB encryption blocks, AES-256-CTR + HMAC-SHA256. Each stored object
header or multipart manifest and the running server version were checked
against the row label. Both published runs disabled compression; v2 had 21
verified rows and v3 had 23 verified rows. The v3 run includes sequential
and four-client multipart writes. Values are local measurements, not B2 or
Cloudflare throughput.

| Write version | Scenario | Payload | n | Clients | Aggregate MB/s | TTFB p50 ms | Completion p50 / p95 ms | Verified |
|---|---|---:|---:|---:|---:|---:|---:|---|
| v2 | `read-full-cold/v2/64kib` | 64 KiB | 1 | 1 | 145.7 | 0.37 | 0.43 / 0.43 | yes |
| v2 | `read-full-warm/v2/64kib` | 64 KiB | 7 | 1 | 75.1 | 0.61 | 0.68 / 1.49 | yes |
| v2 | `read-range-aligned/v2/64kib` | 32 KiB | 7 | 1 | 43.1 | 0.60 | 0.66 / 1.22 | yes |
| v2 | `read-range-unaligned/v2/64kib` | 32 KiB | 7 | 1 | 51.9 | 0.54 | 0.62 / 0.70 | yes |
| v2 | `read-range-suffix/v2/64kib` | 64 KiB | 7 | 1 | 105.1 | 0.48 | 0.64 / 0.72 | yes |
| v2 | `write-single-put/v2/64kib` | 64 KiB | 3 | 1 | 48.6 | — | 1.22 / 1.68 | yes |
| v2 | `read-full-warm/v2/64kib/clients=4` | 64 KiB | 28 | 4 | 151.6 | 0.89 | 0.96 / 4.62 | yes |
| v2 | `read-full-cold/v2/1mib` | 1 MiB | 1 | 1 | 406.5 | 0.39 | 2.46 / 2.46 | yes |
| v2 | `read-full-warm/v2/1mib` | 1 MiB | 7 | 1 | 405.8 | 0.53 | 2.54 / 2.68 | yes |
| v2 | `read-range-aligned/v2/1mib` | 64 KiB | 7 | 1 | 105.3 | 0.52 | 0.60 / 0.73 | yes |
| v2 | `read-range-unaligned/v2/1mib` | 32 KiB | 7 | 1 | 61.7 | 0.44 | 0.49 / 0.62 | yes |
| v2 | `read-range-suffix/v2/1mib` | 64 KiB | 7 | 1 | 109.5 | 0.48 | 0.57 / 0.67 | yes |
| v2 | `write-single-put/v2/1mib` | 1 MiB | 3 | 1 | 120.8 | — | 8.48 / 8.56 | yes |
| v2 | `read-full-warm/v2/1mib/clients=4` | 1 MiB | 28 | 4 | 1265.5 | 0.76 | 3.03 / 3.65 | yes |
| v2 | `read-full-cold/v2/64mib` | 64 MiB | 1 | 1 | 459.2 | 0.60 | 139.38 / 139.38 | yes |
| v2 | `read-full-warm/v2/64mib` | 64 MiB | 7 | 1 | 518.0 | 0.56 | 123.61 / 129.60 | yes |
| v2 | `read-range-aligned/v2/64mib` | 64 KiB | 3 | 1 | 107.6 | 0.53 | 0.60 / 0.61 | yes |
| v2 | `read-range-unaligned/v2/64mib` | 32 KiB | 3 | 1 | 53.5 | 0.53 | 0.58 / 0.65 | yes |
| v2 | `read-range-suffix/v2/64mib` | 64 KiB | 3 | 1 | 106.0 | 0.53 | 0.61 / 0.63 | yes |
| v2 | `write-single-put/v2/64mib` | 64 MiB | 3 | 1 | 235.3 | — | 276.05 / 279.70 | yes |
| v2 | `read-full-warm/v2/64mib/clients=4` | 64 MiB | 28 | 4 | 1757.5 | 0.53 | 143.50 / 153.02 | yes |
| v3 | `read-full-cold/v3/64kib` | 64 KiB | 1 | 1 | 100.5 | 0.55 | 0.62 / 0.62 | yes |
| v3 | `read-full-warm/v3/64kib` | 64 KiB | 7 | 1 | 75.0 | 0.64 | 0.69 / 1.54 | yes |
| v3 | `read-range-aligned/v3/64kib` | 32 KiB | 7 | 1 | 33.2 | 0.91 | 0.96 / 1.30 | yes |
| v3 | `read-range-unaligned/v3/64kib` | 32 KiB | 7 | 1 | 42.6 | 0.67 | 0.71 / 1.07 | yes |
| v3 | `read-range-suffix/v3/64kib` | 64 KiB | 7 | 1 | 104.6 | 0.51 | 0.57 / 0.81 | yes |
| v3 | `write-single-put/v3/64kib` | 64 KiB | 3 | 1 | 74.0 | — | 0.85 / 0.93 | yes |
| v3 | `read-full-warm/v3/64kib/clients=4` | 64 KiB | 28 | 4 | 192.8 | 0.96 | 1.09 / 1.76 | yes |
| v3 | `read-full-cold/v3/1mib` | 1 MiB | 1 | 1 | 329.3 | 0.66 | 3.04 / 3.04 | yes |
| v3 | `read-full-warm/v3/1mib` | 1 MiB | 7 | 1 | 280.5 | 0.70 | 3.21 / 4.88 | yes |
| v3 | `read-range-aligned/v3/1mib` | 64 KiB | 7 | 1 | 112.6 | 0.44 | 0.58 / 0.92 | yes |
| v3 | `read-range-unaligned/v3/1mib` | 32 KiB | 7 | 1 | 51.1 | 0.55 | 0.61 / 0.92 | yes |
| v3 | `read-range-suffix/v3/1mib` | 64 KiB | 7 | 1 | 128.3 | 0.40 | 0.45 / 0.66 | yes |
| v3 | `write-single-put/v3/1mib` | 1 MiB | 3 | 1 | 166.8 | — | 6.44 / 6.64 | yes |
| v3 | `read-full-warm/v3/1mib/clients=4` | 1 MiB | 28 | 4 | 765.3 | 0.95 | 4.94 / 6.14 | yes |
| v3 | `read-full-cold/v3/64mib` | 64 MiB | 1 | 1 | 414.1 | 0.61 | 154.56 / 154.56 | yes |
| v3 | `read-full-warm/v3/64mib` | 64 MiB | 7 | 1 | 394.3 | 0.66 | 163.13 / 179.91 | yes |
| v3 | `read-range-aligned/v3/64mib` | 64 KiB | 3 | 1 | 90.3 | 0.62 | 0.68 / 0.78 | yes |
| v3 | `read-range-unaligned/v3/64mib` | 32 KiB | 3 | 1 | 53.3 | 0.56 | 0.60 / 0.61 | yes |
| v3 | `read-range-suffix/v3/64mib` | 64 KiB | 3 | 1 | 120.4 | 0.45 | 0.52 / 0.58 | yes |
| v3 | `write-single-put/v3/64mib` | 64 MiB | 3 | 1 | 245.4 | — | 260.58 / 269.23 | yes |
| v3 | `read-full-warm/v3/64mib/clients=4` | 64 MiB | 28 | 4 | 1402.0 | 0.68 | 180.13 / 190.67 | yes |
| v3 | `write-multipart/v3/64mib` | 64 MiB | 3 | 1 | 52.4 | — | 455.50 / 835.85 | yes |
| v3 | `write-multipart-concurrent/v3/64mib/clients=4` | 64 MiB | 2 | 4 | 78.2 | — | 590.60 / 995.08 | yes |

The 64 MiB warm full-GET row measured 518.0 MB/s for v2 and 394.3 MB/s
for v3 in these separate local runs. Use the focused range baseline for
backend request counts; the matrix rows include cryptography, loopback HTTP,
and filesystem effects. The local matrix does not establish production
capacity or a stable cross-version throughput ratio.

A diagnostic v3 run with compression enabled failed verification for the
1 MiB and 64 MiB compressed rows (`invalid v3 trailer offset` or plaintext
SHA mismatch). Its four-client multipart write row also failed during
part listing; that row passed in the uncompressed rerun above. Neither
failed row is used as throughput evidence. Compression needs a separately
verified result before it can be included in a comparison.

## Cleanup and credentials policy

- All benchmark objects live under the dedicated `armor-bench/` key prefix;
  local cleanup deletes exactly that prefix plus the temp backend directory.
  Remote cleanup deletes exactly the keys the run uploaded.
- Credentials only ever travel as environment variable references
  (`AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` fetched from the operator's
  own store); they are never recorded in results, logs, or this document.
- The local harness's credentials are the throwaway in-process SigV4
  constants from `internal/server/srvtest` — they authenticate nothing.
