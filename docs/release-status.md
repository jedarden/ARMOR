# Release status and known limitations

**Status date:** 2026-10-16 · **Source version:** see [`VERSION`](../VERSION)

This page is the evidence boundary for operator documentation. `repository-tested`
means the current tree has focused passing tests, not proof of a published image
or a particular deployment. `active-regression` means at least one cited bead
still has an open support or verification boundary; the row names that boundary
even when a narrower behavior has passed. The compatibility and DR pages link
here so their examples do not silently become release claims.

Release metadata was backfilled through **`v0.1.1988`** under
`armor-5f0b4d23`, then repaired for image-backed **`v0.1.1993`** and
**`v0.1.1994`**. Authenticated
Docker Hub server and companion images exist for 0.1.1983, 0.1.1984,
0.1.1986–0.1.1988, 0.1.1993, and 0.1.1994; 0.1.1989–0.1.1992 have no server image and
are marked unpublished in the changelog. The public GHCR server mirror is
missing for 0.1.1984. The 0.1.1993 CI run failed while installing the Barman
client before publication; its tag and both releases were created by the
idempotent publisher after the image appeared. The 0.1.1994 workflow passed
lint, the release gate, image builds, and registry checks, then failed in the
real-client compatibility leg; its on-exit path delivered the operator alert,
and the idempotent publisher created the original-commit tag and both
non-draft releases after the image appeared. GitOps commits `810b34733` and
`b24dab46c` carry the Python-header and conservative-sensor fixes. These
artifact checks do not prove a green end-to-end client gate or fleet rollout.
The table identifies the last specific image proof where one is recorded.

## Current release posture

<!-- capability: named-key-read status: active-regression evidence: armor-8317f313 -->
<!-- capability: wrapped-dek-decoding status: repository-tested evidence: armor-4b1b6c64 armor-da67956d -->
<!-- capability: multipart-hmac-read status: repository-tested evidence: armor-1b272971 armor-7edd1237 -->
<!-- capability: v3-multipart-verification status: repository-tested evidence: armor-86a90341 armor-da67956d -->
<!-- capability: sigv4-authentication status: repository-tested evidence: armor-a3b04ef2 armor-4795decf -->
<!-- capability: presigned-get status: repository-tested evidence: armor-0163fb32 -->
<!-- capability: range-reads status: active-regression evidence: armor-817d9d92 armor-73564f97 armor-5694713b armor-272299ca armor-68e3d5a9 armor-ebd22a99 -->
<!-- capability: adr006-provider-outage-failover status: repository-tested evidence: armor-14a2e34c -->
<!-- capability: s3-operation-surface status: scope-limited -->

| Capability | Status | Current source evidence | Published-image evidence | Operator boundary |
|---|---|---|---|---|
| Named-key and ring-key reads | `active-regression` | Real key-manager and multi-key routing tests cover fingerprint selection and unknown metadata. The fix shipped, but `armor-8317f313` remains deferred. | **0.1.1969:** the bead records a published-image named-key PUT/GET round trip and image digest. | Active regression record: reconcile the deferred bead and recheck the target deployment's named-key/ring configuration before claiming fleet-wide support. |
| Fingerprinted wrapped-DEK decoding | `repository-tested` | CLI, server, and restore-verifier tests cover the `v2:<fingerprint>:<base64>` form and legacy values. `armor-4b1b6c64` and `armor-da67956d` are closed. | **None recorded** for the DR/escrow path; the client image matrix does not exercise it. | Verify the exact escrow package, key ring, and object population in a DR drill before making a deployment claim. |
| Multipart HMAC sidecar and read paths | `repository-tested` | V2/v3 full-read, range-read, prefix, sidecar, and corruption tests cover the current layouts. The prefix lookup fixes in `armor-1b272971` and `armor-7edd1237` are closed. | **None recorded** for v3 sidecar/DR behavior. The 0.1.1975 client harness writes v2 objects. | Preserve `.armor/hmac/` and verify existing multipart objects before migration or failover. |
| V3 multipart verification in CLI and DR | `repository-tested` | `armor verify`, `armor decrypt`, and restore-verifier tests cover manifest fallback, sidecar HMACs, part counters, and digests. `armor-86a90341` and `armor-da67956d` are closed. | **None recorded** for the CLI/DR path; the client image matrix exercises server S3 operations. | A server GET is not a DR drill. Run the verifier against real objects with the intended image and credentials. |
| SigV4 authentication, including AWS CLI | `repository-tested` | Request-shape and real-client tests cover accepted and rejected credentials; `armor-a3b04ef2` and `armor-4795decf` are closed. The six-client matrix binds AWS CLI authentication to its PUT/GET round trip. | **0.1.1975** is the last recorded AWS CLI image proof; it predates the six-client matrix. Backfilled tags through 0.1.1988 have no completed matrix proof. | AWS CLI works in the tested source paths. Check the image and credentials of the deployment before claiming its access is verified. |
| Presigned/share GET | `repository-tested` | The enabled in-process harness passes share GET and range/error cases; `armor-0163fb32` closed after correcting its test configuration. | **None recorded**; the share GET test skips external-image endpoint mode. | Enable `ARMOR_PRESIGN_ENABLED` and configure the base URL and secret, then probe the deployed route. |
| Byte-range reads | `active-regression` | Protocol and handler tests cover byte/status correctness; `armor-817d9d92` closed the large multipart read failure, `armor-73564f97` and `armor-5694713b` closed bounded read-ahead and offset coalescing, and `armor-272299ca` closed whole-part v3 multipart GET streaming (block-by-block through one pipelined range; on `main`, not yet in a release). [Local baseline](performance/large-object-range-read-baseline.md) and [live qualification](performance/production-b2-cloudflare-2026-09-28.md) bound performance evidence. `armor-68e3d5a9` and `armor-ebd22a99` remain open. | **0.1.1975** predates the six-client range matrix; the later backfilled tags do not prove the streaming read path, and the first live qualification run captured no valid throughput numbers. | Active regression boundary is large-object throughput and bounded reads; do not turn the incomplete live qualification into a production performance claim. |
| ADR-006 provider-outage failover and failback | `repository-tested` | The tagged `TestProviderOutageFailoverWritesAndFailback` filesystem drill and backend tests prove the explicit promotion sequence; `armor-14a2e34c` is closed. | **None recorded** for a live provider outage. | Run the live drill and record the image version and recovery inventory before treating a deployment as proven. |
| S3 operation surface | `scope-limited` | The current six-client matrix covers authentication, reads, ranges, listing, overwrite/delete, and multipart where supported. | **0.1.1975** is the last completed client-image evidence and predates that matrix; backfilled tags through 0.1.1988 have no completed matrix proof. | Use the matrix for the exact image under evaluation; this is a scoped operation claim. |

## Known limitations

- **Release proof is version-specific.** A passing test on `main` does not
  upgrade an older fleet image. Record the image tag/digest, test object shape,
  MEK/ring fingerprints, and the focused command when accepting a rollout.
- **The DR path is not interchangeable with the live read path.** `armor
  decrypt` and the restore verifier need B2 metadata, the wrapped DEK, the
  correct MEK/ring, and multipart sidecars. A successful server GET alone is
  not a DR drill.
- **Multipart integrity depends on internal state.** Do not delete or rewrite
  `.armor/hmac/`, manifests, or multipart metadata with native B2 tools. Older
  objects and objects produced during a failed release need explicit
  verification before migration or failover.
- **Presigned URLs are opt-in.** A disabled or incompletely configured presign
  route returns an authorization/configuration failure; that is not evidence
  about the ordinary SigV4 S3 route.
- **Range correctness and range performance are different contracts.** The
  current tests protect byte and status semantics, while the focused local
  baseline measures request count, backend bytes, and latency for representative
  large-object spans. It does not measure B2, Cloudflare, production network
  tails, or bounded-memory behavior; the active read-ahead work still concerns
  those limits.
- **Compatibility is scoped.** The supported examples are the operations in
  the compatibility matrix, against the tested image and configuration. Do not
  generalize them to untested S3 features or to a deployment whose image or
  configuration has not been checked.
- **Restore-verifier alerting is not yet a fleet-wide release claim.** The
  GitOps rollout now contains the scrape, rule-evaluation, Alertmanager route,
  and tailnet verification surfaces for all five verifier scopes. The live
  smoke test passes on `iad-ci` and `rs-manager` (14/14 checks each). The
  `iad-kalshi` and `ord-devimprint` endpoints were unreachable during the
  2026-09-29 check. On 2026-10-16, `ardenone-cluster` Prometheus accepted the
  new `0.1.1993` verifier scrape (`health=up`, empty `lastError`), confirming
  the numeric exposition fix (`armor-c866e4ca`) reached that target. Its
  verification ratio is 7/11: four v3 multipart sidecar lookups return 404
  at the tenant path and 403 at the root fallback (`armor-69b4983d`, proposal
  pending).
  Treat restore failures as paged only on the two smoke-verified clusters
  until the remaining endpoint and restore checks pass; a running verifier
  without a passing scrape and delivery path is not an alerting guarantee.
  See the [restore-verifier alerting runbook](runbooks/restore-verifier-alerting.md)
  for the exact commands and current boundary.
- **Fleet rollout evidence (2026-10-16, `armor-769e3d53`).** All 15 ARMOR and
  restore-verifier manifests across `iad-ci`, `iad-kalshi`, `ord-devimprint`,
  `rs-manager`, `apexalgo-iad`, and `ardenone-cluster` were bumped from
  `0.1.1970`/`0.1.1971`/`0.1.1975` to digest-pinned `0.1.1993`
  (`armor` `sha256:1845b2b4…`, `armor-restore-verifier` `sha256:581d25d4…`)
  via declarative-config commits `d8faa9f10`–`5807786f3`; `armor-test` gained
  the digest pin it lacked. The manifest-only drift check classifies all 15
  pins as current with zero stale or mismatched entries. The configured
  host-side live drift check reported 15 unavailable entries, so it is not
  yet independent evidence of running versions. Read-only pod inspection
  confirms all 15 workloads Ready on `0.1.1993` with zero restarts.
  `iad-ci`, `iad-kalshi`, and `rs-manager` verifiers reported 11/11 on the
  new image; `ardenone-cluster` reported 7/11. The `ord-devimprint` verifier
  reached Ready through GitOps commit `231a87a97`, but its first verification
  cycle was still running when checked. The `rs-manager` verifier reached
  Ready and completed 11/11 through GitOps commit `e15c5d6c0`. Argo had
  rejected `Recreate` against the retained live `rollingUpdate` field, so
  both verifiers use zero-surge rolling updates to drain their RWO PVC pods
  first. This is not a client-matrix or DR-drill proof (see the capability
  table above).

## How to promote a capability

For a release or fleet rollout, run the focused repository tests, run the
real-client/image compatibility leg where applicable, and execute the relevant
live or DR probe. Update this page and the status manifest in the same change,
with the image version and evidence. Until then, use the
`active-regression`, `release-pending`, `repository-tested`, or `scope-limited`
wording above; never turn an active regression into a historical-only note.
