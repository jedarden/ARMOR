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
| Object format | v3, 64 KiB blocks, AES-GCM |
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

The qualification remains blocked by primary bead
[`armor-73564f97`](https://git.ardenone.com/jedarden/ARMOR/issues/armor-73564f97)
(bounded read-ahead and larger backend ranges), with related v3 coalescing
work in
[`armor-5694713b`](https://git.ardenone.com/jedarden/ARMOR/issues/armor-5694713b).
No valid production throughput numbers exist until one of those fixes is
published and a deployment completes the ordinary full verification GET.

## Release-status boundary

This is an operational failure signal, not a production throughput number.
The current release status must continue to say that B2/Cloudflare large-object
performance is unmeasured and that range support needs a healthy deployment
qualification. Re-run the workflow after the deployment read path and large
PUT path are healthy, then replace this result with the generated summary and
retain the image digest, runner, workload, cache statuses, and limits.

The result is bounded by one runner and one replica. It is not a load test,
capacity claim, SLO, or evidence for multipart performance. B2 origin rows
also require a reference fetch of the encrypted object and may incur provider
egress; Cloudflare cache state and edge selection are time-dependent.
