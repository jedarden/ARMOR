# Large-Object Range-Read Baseline

Established 2026-09-28 by `armor-f263499c`. This is the focused, repeatable
baseline for the open range-performance question in the README. The benchmark
is `TestLargeObjectRangeReadBaseline` in
[`tests/performance/large_range_test.go`](../../tests/performance/large_range_test.go).

## Procedure

Run it from the ARMOR checkout with no credentials or network dependency:

```sh
ARMOR_PERF_RUN=1 \
ARMOR_PERF_OUT=/tmp/armor-large-range-results \
go test ./tests/performance -run TestLargeObjectRangeReadBaseline -v -timeout 40m
```

The default run writes one synthetic, incompressible 64 MiB v3 single-PUT
object through the real authenticated S3 handler over a temporary filesystem
backend. It warms each range once, then takes five measured samples and
byte-compares every response with the exact plaintext slice. The report records
p50/p95 completion latency, backend fetches per sample, backend bytes per
sample, and the backend-bytes/response-bytes ratio. The raw JSON and rendered
Markdown are `large-range-results.json` and `large-range-results.md` under
`ARMOR_PERF_OUT`.

The range shapes are:

| Shape | Header | Why it is included |
|---|---|---|
| aligned-64KiB | `bytes=33554432-33619967` | one complete encryption block in the middle of the object |
| unaligned-32KiB | `bytes=33555666-33588433` | short read crossing a block boundary |
| aligned-1MiB | `bytes=16777216-17825791` | multi-block column/page-sized read |
| aligned-8MiB | `bytes=25165824-33554431` | larger contiguous range |
| suffix-64KiB | `bytes=-65536` | footer-like tail read on a large object |

To repeat the shape at a larger size, set
`ARMOR_PERF_LARGE_OBJECT=1GiB`. The focused run requires at least 40 MiB so
the fixed representative offsets remain valid. `ARMOR_PERF_READ_SAMPLES`
changes the sample count.

## Measured local baseline

Source: ARMOR `main` at `6812fc31` (the benchmark and this document were
added after the measurement; the measured server code was unchanged). Date:
2026-09-28. Host: `codinghome`, Linux amd64, Go `go1.26.5`,
`GOMAXPROCS=20`; loopback HTTP and temporary local filesystem; v3, 64 KiB
blocks, AES-GCM; five samples per row. Every row verified successfully.

| Range | Response | p50 ms | p95 ms | Backend fetches/sample | Backend bytes/sample | Backend/response |
|---|---:|---:|---:|---:|---:|---:|
| aligned-64KiB | 64 KiB | 1.42 | 8.50 | 5.0 | 65,696 B | 1.00× |
| unaligned-32KiB | 32 KiB | 2.46 | 4.76 | 5.0 | 65,696 B | 2.00× |
| aligned-1MiB | 1 MiB | 15.69 | 35.18 | 5.0 | 1,049,216 B | 1.00× |
| aligned-8MiB | 8 MiB | 93.07 | 117.48 | 5.0 | 8,392,832 B | 1.00× |
| suffix-64KiB | 64 KiB | 0.74 | 1.18 | 5.0 | 65,696 B | 1.00× |

The local shape is selective: a 64 KiB read from a 64 MiB object fetched about
64 KiB plus 160 bytes of envelope/table overhead per sample, rather than the
whole object. The unaligned 32 KiB read fetched one 64 KiB data block, so its
backend-to-response ratio was 2×. The 1 MiB and 8 MiB aligned reads fetched
their requested data plus the same small overhead. These are local service
shape measurements, not a claim that a remote B2 request has the same latency.

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
- The run covers a single-PUT v3 object. Multipart range behavior, compressed
  objects (which reject ranges), concurrent clients, and 1 GiB production
  objects need separate measurements.
- The test has no timing threshold in CI. The repeatable output is a baseline
  for comparing changes; it is not a performance SLO.
