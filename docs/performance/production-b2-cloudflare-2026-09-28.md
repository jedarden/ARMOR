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
