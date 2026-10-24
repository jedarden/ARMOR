# Production B2/Cloudflare performance qualification

Attempted during the 2026-09-28 release-status cycle against the representative
`iad-ci/armor` deployment. This document records the result of the first live
qualification so release notes do not mistake an incomplete run for a
throughput measurement.

## Target and workload

| Field | Value |
|---|---|
| Deployment | `iad-ci/armor` |
| Image | `ronaldraygun/armor:0.1.1971@sha256:7c146c638026699fd67a8aa7f10c4a1398fe70045f3546ab37bb8979fbc14b1e` |
| B2 region/bucket | `us-west-002` / `iad-ci` |
| ARMOR path | tailnet S3 edge `armor-iad-ci-ts.ardenone.com:8444` |
| Public path | `b2-us-west-002.ardenone.com/file/iad-ci/<key>` |
| Runner | `codinghome`, Linux amd64 |
| Object format | v3, 64 KiB blocks, AES-256-CTR + HMAC-SHA256 |
| Default workload | 64 MiB synthetic incompressible object; 3 serialized PUTs; 5 full reads; 3 samples of 5 range shapes |
| Range shapes | aligned 64 KiB, unaligned 32 KiB, aligned 1 MiB, aligned 8 MiB, suffix 64 KiB |
| Credentials | read at runtime from OpenBao `rs-manager/iad-ci/armor` and `rs-manager/iad-ci/b2/iad-ci`; no values recorded |

The workflow uploads through ARMOR, verifies each acknowledged upload through
an ordinary ARMOR GET, then measures ARMOR plaintext reads, direct B2
ciphertext reads, and public Cloudflare ciphertext reads. The exact workflow
and rerun command are in the [performance runbook](README.md).

## Result

No valid production throughput result was produced.

- The bounded default run accepted the first large upload but its first
  ordinary ARMOR verification GET remained in the HTTP/2 response-body read for
  427.5 seconds until the run was interrupted. No result report was written.
- A diagnostic run using one 40 MiB object and one sample per operation timed
  out after 180 seconds in the same verification GET. It also produced no
  report.
- A diagnostic HTTP/1.1 retry failed during the large PUT with `use of closed
  network connection`; it is not performance evidence.

The benchmark's remote reader now stops only after the expected response byte
count and still validates status, size, and SHA-256. That prevents an absent
end-of-stream marker from making a healthy response unbounded, but it cannot
turn a deployment that does not deliver the expected bytes into a valid
measurement. The attempted runs used deferred cleanup; no raw result or
benchmark object identifier is retained here.

## Follow-up attempt — 2026-09-29

A bounded diagnostic retry used the same live deployment and image after the
pod had been ready for more than a day:

| Field | Value |
|---|---|
| Deployment/image | `iad-ci/armor`, `0.1.1971@sha256:7c146c638026699fd67a8aa7f10c4a1398fe70045f3546ab37bb8979fbc14b1e` |
| Probe workload | one 64 MiB synthetic object, one PUT, one ordinary verification GET |
| PUT result | HTTP 200 in 24.761 s |
| Verification result | no completion after 287.303 s; interrupted; no report written |
| Pod state | remained Ready with no restart during the probe |
| Cleanup | the uploaded benchmark object was deleted by its exact key |

The public Cloudflare path independently returned correct 206 responses for
the same object at byte 0, a mid-object range, and the tail (65,536, 65,536,
and 37 bytes respectively). Those individual checks completed in 1.221 s,
1.104 s, and 0.725 s. They are diagnostics only, not throughput results.

### Root cause and blocker

The failure is in the production read-path shape, not object creation or basic
Cloudflare reachability. A v3 single-PUT full GET calls the backend for the
whole ciphertext stream. `B2Backend.GetRangeWithHeaders` divides that span
into 64 KiB requests and allows 16 concurrent requests; the ordered reader
cannot advance past a missing block, and the individual Cloudflare requests
have no bounded backend deadline. Thus one delayed or wedged range request can
leave the ordinary ARMOR verification GET open indefinitely. The current
`0.1.1971` image fixed v3 multipart range batching and wide offsets, but did
not coalesce the single-PUT stream or add the required bounded read-ahead and
failure deadline.

The 0.1.1971-era blocker above is closed: both named beads landed
(`armor-73564f97` in v0.1.1987, `armor-5694713b` on `main`), and the
2026-10-24 rerun below completed the ordinary full verification GET and
produced the first valid production numbers for this workload.

## Qualification — 2026-10-24 (valid result)

A full rerun of the documented workload produced a valid, checked result on
2026-10-24 (run window 03:32:51–03:55:08 UTC; `TestRemoteBaseline` PASS in
1340 s, exit 0). This is the result the release-status register cites.

| Field | Value |
|---|---|
| Deployment | `iad-ci/armor` (1 replica, Ready throughout, no restarts) |
| Image | `ronaldraygun/armor:0.1.1993@sha256:1845b2b4a704b30542e656bd0c87ec6b54539d732e50a83657a48623fddc9a60` |
| Harness source | ARMOR `main` @ `abe39288`; the deployed server is the 0.1.1993 image and the two are recorded separately |
| Blocker status | `armor-73564f97` (1 MiB coalesced backend fetches, bounded read-ahead window, 2 m per-request deadline, backend budget) shipped in v0.1.1987 and is deployed. `armor-5694713b` (v3 range coalescing, `792a619b`) is on `main` but **not** in this image, so the ARMOR range rows measure the per-block v3 range path over the new bounded backend. |
| B2 region/bucket | `us-west-002` / `iad-ci` |
| Runner | `codinghome`, Linux amd64, GOMAXPROCS=20, Go 1.25.14 |
| Workload | default: 64 MiB synthetic incompressible object; 3 serialized single-PUT uploads, each followed by an untimed ordinary ARMOR verification GET; 5 warm full reads; 3 samples × 5 range shapes; the last object re-read through b2-origin and the public Cloudflare path |
| Checks | all 19 reported rows passed their SHA-256 full-body (or exact-slice, for ranges) comparison; no row was accepted without its payload check |
| Cache | public Cloudflare path served `CF-Cache-Status: DYNAMIC` on every leg |
| Cleanup | the run's three `armor-bench/production/<run-id>/` keys deleted (HTTP 204) |

Latency and throughput (p50 unless noted; ciphertext rows carry the v3
envelope, 67,145,792 B):

| Path | Scenario | p50 | p95 | Payload MB/s p50 |
|---|---|---:|---:|---:|
| armor | upload single-PUT (n=3) | 345.1 s | 403.3 s | 0.2 |
| armor | full GET warm (n=5) | 32.4 s | 38.5 s | 2.0 |
| armor | range aligned 64 KiB (n=3) | 1.43 s | 2.25 s | — |
| armor | range unaligned 32 KiB (n=3) | 2.95 s | 4.00 s | — |
| armor | range aligned 1 MiB (n=3) | 10.4 s | 10.5 s | — |
| armor | range aligned 8 MiB (n=3) | 57.0 s | 60.1 s | — |
| armor | range suffix 64 KiB (n=3) | 1.63 s | 3.87 s | — |
| b2-origin | full GET (n=5) | 4.10 s | 4.44 s | 15.6 |
| b2-origin | range aligned 8 MiB (n=3) | 608 ms | 693 ms | 13.2 |
| b2-origin | range 32–64 KiB (n=3 each) | ~181 ms | ~186 ms | — |
| cloudflare | full GET (n=5) | 4.43 s | 12.9 s | 14.5 |
| cloudflare | range aligned 8 MiB (n=3) | 694 ms | 695 ms | 11.5 |
| cloudflare | range 32–64 KiB (n=3 each) | ~200 ms | ~218 ms | — |

Interpretation:

- The blocker is fixed in the deployed shape: each of the three uploads was
  followed by a completed ordinary ARMOR verification GET (server-side 200
  delivering all 67,108,864 plaintext bytes in ~30 s), where the 0.1.1971
  runs never delivered. The bounded backend read path
  (`armor-73564f97`) is what changed.
- Uploads are dominated by B2 PUT time, which varied 12 s → 315 s → 375 s
  across the three samples (per-request `backend_put_ms` in the server log).
  The ARMOR upload MB/s row is therefore the least stable row in the set.
- ARMOR plaintext reads (2.0 MB/s full GET) remain ~7× slower than the same
  ciphertext over direct Cloudflare (14.5 MB/s): the streamed decrypt
  pipeline, not transport, is the read-throughput boundary. Large-object
  read throughput stays the active regression boundary; this run measures it,
  it does not lift it.
- The 8 MiB aligned ARMOR range (57 s p50) reflects the deployed image's
  per-block v3 range path (~128 sequential 64 KiB backend fetches per
  sample). Re-measure once `armor-5694713b`'s span coalescing ships in a
  release; the small range shapes do not exercise that path.
- `CF-Cache-Status: DYNAMIC` on every leg means these public objects were
  served uncached; the Cloudflare rows measure edge + origin, not a
  cache-hit path.

## Release-status boundary

The register cites the 2026-10-24 figures with these limits: one runner and
one replica, single client, synthetic objects. It is not a load test,
capacity claim, SLO, or evidence for multipart performance. ARMOR plaintext
read throughput and the large-PUT path remain the active regression
boundary — the bounded read path removed the unbounded-hang failure, it did
not make 2 MB/s a performance claim. B2 origin rows required a reference
fetch of the encrypted object and incurred provider egress; Cloudflare cache
state and edge selection are time-dependent (`DYNAMIC` throughout this run).
Re-run the workload per release once the range-span coalescing
(`armor-5694713b`) and whole-part multipart streaming (`armor-272299ca`)
fixes reach a deployed image.
