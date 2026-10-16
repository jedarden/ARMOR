# Large-Object Range-Read Baseline

Established 2026-09-28 by `armor-f263499c`; corrected and remeasured by
`armor-2af339bc` after the original run was found to have written v2 while
labelling its rows v3. This is the focused, repeatable baseline for the open
range-performance question in the README. The benchmark
is `TestLargeObjectRangeReadBaseline` in
[`tests/performance/large_range_test.go`](../../tests/performance/large_range_test.go).

## Procedure

Run it from the ARMOR checkout with no credentials or network dependency:

```sh
ARMOR_PERF_RUN=1 \
ARMOR_PERF_OUT="$PWD/.perf-results" \
go test ./tests/performance -run TestLargeObjectRangeReadBaseline -v -count=1 -timeout 40m
```

The default run writes synthetic, incompressible 64 MiB v3 single-PUT and
multipart objects through the real authenticated S3 handler over a temporary
filesystem backend. Set `ARMOR_PERF_WRITE_VERSION=2` to measure v2 single-PUT.
The harness checks the server's effective write version and the stored object
format before accepting a row. It warms each range once, then takes five measured samples and
byte-compares every response with the exact plaintext slice. The report records
p50/p95 completion latency, backend fetches per sample, backend bytes per
sample, and the backend-bytes/response-bytes ratio. The raw JSON and rendered
Markdown are `large-range-results.json` and `large-range-results.md` under
`ARMOR_PERF_OUT`.

The range shapes are:

| Write version | Write path | Shape | Header | Why it is included |
|---|---|---|---|---|
| v2, v3 | single-PUT | aligned-64KiB | `bytes=33554432-33619967` | one complete encryption block in the middle of the object |
| v2, v3 | single-PUT | unaligned-32KiB | `bytes=33555666-33588433` | short read crossing a block boundary |
| v2, v3 | single-PUT | aligned-1MiB | `bytes=16777216-17825791` | multi-block column/page-sized read |
| v3 | multipart | aligned-1MiB | `bytes=16777216-17825791` | same span across one part |
| v2, v3 | single-PUT | aligned-8MiB | `bytes=25165824-33554431` | larger contiguous range |
| v2, v3 | single-PUT | suffix-64KiB | `bytes=-65536` | footer-like tail read on a large object |
| v3 | multipart | suffix-64KiB | `bytes=-65536` | footer-like tail read in the last part |
| v3 | multipart | cross-part-boundary-64KiB | `bytes=8355840-8421375` | read spanning two 8 MiB parts |

To repeat the shape at a larger size, set
`ARMOR_PERF_LARGE_OBJECT=1GiB`. The focused run requires at least 40 MiB so
the fixed representative offsets remain valid. `ARMOR_PERF_READ_SAMPLES`
changes the sample count. `ARMOR_PERF_BACKEND_RTT_MS` injects a fixed delay
per backend call for latency comparisons; its default is zero.

## Corrected local baseline — 2026-10-15

The original 2026-09-28 run labelled its five rows v3, but its server wrote
v2. In particular, five fetches and 65,696 backend bytes for a 64 KiB
aligned range are v2 measurements. These replacement runs used a clean
export of ARMOR server source `4ba4cb11` with only the
`armor-2af339bc` harness changes overlaid. Host: codinghome, Linux amd64,
Go 1.26.5, GOMAXPROCS=20; loopback HTTP and a temporary filesystem backend.
Each run used an incompressible 64 MiB object, 64 KiB crypto blocks,
AES-256-CTR + HMAC-SHA256, and five measured warm-metadata samples per row.
The running server version and the stored object format were checked before
accepting each row. All rows below verified the exact plaintext bytes.

| Write version | Write path | Range | Response | p50 / p95 ms | Backend fetches/sample | Backend bytes/sample | Backend/response |
|---|---|---|---:|---:|---:|---:|---:|
| v2 | single-put | aligned-64KiB | 64 KiB | 0.69 / 0.71 | 5.0 | 65,696 B | 1.00× |
| v2 | single-put | unaligned-32KiB | 32 KiB | 0.63 / 0.65 | 5.0 | 65,696 B | 2.00× |
| v2 | single-put | aligned-1MiB | 1 MiB | 5.72 / 5.99 | 5.0 | 1,049,216 B | 1.00× |
| v2 | single-put | aligned-8MiB | 8 MiB | 29.82 / 40.54 | 5.0 | 8,392,832 B | 1.00× |
| v2 | single-put | suffix-64KiB | 64 KiB | 0.57 / 1.03 | 5.0 | 65,696 B | 1.00× |
| v3 | single-put | aligned-64KiB | 64 KiB | 0.55 / 0.67 | 5.0 | 102,528 B | 1.56× |
| v3 | single-put | unaligned-32KiB | 32 KiB | 0.53 / 0.56 | 5.0 | 102,528 B | 3.13× |
| v3 | single-put | aligned-1MiB | 1 MiB | 3.61 / 4.04 | 20.0 | 1,085,568 B | 1.04× |
| v3 | single-put | aligned-8MiB | 8 MiB | 31.76 / 33.43 | 132.0 | 8,425,600 B | 1.00× |
| v3 | single-put | suffix-64KiB | 64 KiB | 0.42 / 0.48 | 5.0 | 102,528 B | 1.56× |
| v3 | multipart | aligned-1MiB | 1 MiB | 2.91 / 3.16 | 2.0 | 1,049,307 B | 1.00× |
| v3 | multipart | cross-part-boundary-64KiB | 64 KiB | 0.49 / 0.75 | 3.0 | 131,803 B | 2.01× |
| v3 | multipart | suffix-64KiB | 64 KiB | 0.46 / 0.55 | 2.0 | 66,267 B | 1.01× |

### Fixed 5 ms backend-call latency

The same 64 MiB workloads were rerun with
`ARMOR_PERF_BACKEND_RTT_MS=5`. This isolates the cost of sequential backend
fetches on the local read path; it is a controlled delay, not measured B2
latency. Each row has five measured warm-metadata samples and exact-body
verification.

| Write version | Write path | Range | Backend fetches/sample | p50 / p95 ms at 5 ms/call |
|---|---|---|---:|---:|
| v2 | single-put | aligned-64KiB | 5.0 | 26.59 / 26.64 |
| v2 | single-put | unaligned-32KiB | 5.0 | 26.52 / 26.63 |
| v2 | single-put | aligned-1MiB | 5.0 | 30.39 / 31.06 |
| v2 | single-put | aligned-8MiB | 5.0 | 49.20 / 51.79 |
| v2 | single-put | suffix-64KiB | 5.0 | 26.46 / 26.56 |
| v3 | single-put | aligned-64KiB | 5.0 | 31.68 / 31.76 |
| v3 | single-put | unaligned-32KiB | 5.0 | 31.67 / 31.78 |
| v3 | single-put | aligned-1MiB | 20.0 | 114.05 / 117.76 |
| v3 | single-put | aligned-8MiB | 132.0 | 741.98 / 778.44 |
| v3 | single-put | suffix-64KiB | 5.0 | 31.72 / 31.76 |
| v3 | multipart | aligned-1MiB | 2.0 | 24.22 / 24.65 |
| v3 | multipart | cross-part-boundary-64KiB | 3.0 | 26.86 / 26.96 |
| v3 | multipart | suffix-64KiB | 2.0 | 21.29 / 21.49 |

For aligned multi-block single-PUT ranges, v3 made 20 fetches at 1 MiB
and 132 at 8 MiB in this implementation; v2 made five for either span.
At 5 ms injected latency, those sequential calls dominate the v3 8 MiB
row. The v3 multipart path used two fetches for a 1 MiB aligned range and
three for a 64 KiB range crossing an 8 MiB part boundary. These counts,
bytes, and verified response shapes are the before measurements for work
aimed at v3 range coalescing.

## Limits

- The benchmark uses a loopback listener and filesystem backend. It excludes
  B2 latency, Cloudflare cache behavior, TLS, WAN variability, and production
  contention. It must not be quoted as production throughput.
- Samples are warm-metadata samples. The one-time header/metadata lookup is
  deliberately excluded from the counters after the warm-up; first-request
  latency is therefore not represented by this table.
- Backend bytes are instrumented `Get`/`GetRange` bytes returned to ARMOR,
  including ciphertext and envelope/HMAC metadata. They are not billing
  counters and do not describe any provider-side read-ahead or CDN cache fill.
- The run covers v2 and v3 single-PUT plus selected v3 multipart ranges,
  including a part boundary. Compressed objects (which reject ranges),
  concurrent clients, and 1 GiB production objects need separate measurements.
- The test has no timing threshold in CI. The repeatable output is a baseline
  for comparing changes; it is not a performance SLO.
