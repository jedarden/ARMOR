# Release status and known limitations

**Status date:** 2026-09-29 · **Source version:** see [`VERSION`](../VERSION)

This page is the evidence boundary for operator documentation. “Repository
tested” means the committed tree has focused tests for the behavior. It does
not prove that a published image, a particular fleet deployment, or a
credential/key-ring combination has exercised that behavior. “Release-pending”
means that release-image or live deployment evidence is still required before
an operator should treat the capability as verified. The compatibility and DR
pages link here so their examples do not silently become release claims.
“Active-regression” is stronger: an implementation or verification bead has
reported a current failure or an unclosed support boundary, so test coverage
must not be presented as currently working support.

## Current release posture

<!-- capability: named-key-read status: active-regression evidence: armor-8317f313 -->
<!-- capability: wrapped-dek-decoding status: active-regression evidence: armor-4b1b6c64 armor-da67956d -->
<!-- capability: multipart-hmac-read status: active-regression evidence: armor-1b272971 armor-7edd1237 -->
<!-- capability: v3-multipart-verification status: active-regression evidence: armor-86a90341 armor-da67956d -->
<!-- capability: sigv4-authentication status: active-regression evidence: armor-a3b04ef2 armor-4795decf -->
<!-- capability: presigned-get status: active-regression evidence: armor-0163fb32 -->
<!-- capability: range-reads status: active-regression evidence: armor-817d9d92 armor-73564f97 armor-5694713b armor-272299ca -->
<!-- capability: adr006-provider-outage-failover status: repository-tested evidence: armor-14a2e34c -->
<!-- capability: s3-operation-surface status: scope-limited -->

| Capability | Status | What the source tree demonstrates | Operator boundary |
|---|---|---|---|
| Named-key and ring-key reads | `active-regression` | Focused key-manager and multi-key routing tests cover fingerprint selection and rejection of unknown metadata. | Active regression: current support is not established for non-default keys. The P0 regression record is `armor-8317f313`; its implementation and release evidence must be resolved before relying on named-key reads. |
| Fingerprinted wrapped-DEK decoding | `active-regression` | CLI, server, and restore-verifier tests cover the `v2:<fingerprint>:<base64>` form and legacy values. | Active regression: test coverage does not establish working DR support. Track the decode and verifier evidence in `armor-4b1b6c64` and `armor-da67956d`; exercise the exact escrow package and object population before promotion. |
| Multipart HMAC sidecar and read paths | `active-regression` | In-tree v2/v3 full-read, range-read, prefix, sidecar, and corruption tests cover the current layouts. | Active regression: preserve `.armor/hmac/` and do not rely on existing multipart data until the sidecar reader/write-path evidence is current. The implementation records are `armor-1b272971` and `armor-7edd1237`. |
| V3 multipart verification in CLI and DR | `active-regression` | `armor verify`, `armor decrypt`, and restore-verifier tests cover manifest fallback, sidecar HMACs, part counters, and digest outcomes. | Active regression: a server GET is not DR proof. Published-image and real-object verification remain unestablished; use `armor-86a90341` and `armor-da67956d` for the existing verifier implementation work. |
| SigV4 authentication, including AWS CLI | `active-regression` | The protocol suite and request-shape tests cover accepted and rejected credentials, including AWS CLI and barman-shaped requests. | Active regression: a passing request-shape test is not current AWS CLI support evidence. Reproduce against the release image and classify the denial using `armor-a3b04ef2` and `armor-4795decf` before promotion. |
| Presigned/share GET | `active-regression` | Enabled-harness tests cover share GETs and range/error responses. | Active regression: configure `ARMOR_PRESIGN_ENABLED`, an absolute base URL, and the dedicated secret, then prove the enabled route. The existing 403 regression evidence is `armor-0163fb32`; do not describe it as only historical. |
| Byte-range reads | `active-regression` | Protocol and handler tests cover unaligned, suffix, multipart-boundary, prefix, and corruption cases; the focused [large-object range baseline](performance/large-object-range-read-baseline.md) records local latency and backend-byte shape; the [first production B2/Cloudflare qualification](performance/production-b2-cloudflare-2026-09-28.md) records the workload and an incomplete live result. | Active regression: correctness coverage does not establish currently working large-object support or production performance. The active read-path and bounded-throughput work is tracked by `armor-817d9d92`, `armor-73564f97`, `armor-5694713b`, and `armor-272299ca`. |
| ADR-006 provider-outage failover and failback | `repository-tested` | The tagged `TestProviderOutageFailoverWritesAndFailback` drill destroys the primary, promotes the replicated filesystem backend, proves reads and writes (including replicated metadata) through the promotion, re-uploads the pre-outage object for replacement backfill, and serves old/new data after failback. Backend unit coverage pins the explicit promotion decision. | Release-pending: this is credential-free repository evidence using filesystem backends; run the live provider-outage drill and record the image/version and recovery inventory before treating a deployment as proven. |
| S3 operation surface | `scope-limited` | The documented matrix covers authentication, reads, ranges, listing, overwrite/delete, and multipart where a client supports it. | ARMOR is not a claim of full AWS S3 API compatibility. Use the matrix and the image-gate client run for the exact release under evaluation. |

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
  2026-09-29 check, and `ardenone-cluster`'s Prometheus rejected the
  restore-verifier scrape because its deployed `0.1.1975` image still emits a
  quoted legacy gauge. Treat restore failures as paged only on the two
  smoke-verified clusters until the remaining endpoint/image checks pass; a
  running verifier without a passing scrape and delivery path is not an
  alerting guarantee. See the [restore-verifier alerting runbook](runbooks/restore-verifier-alerting.md)
  for the exact commands and current boundary.

## How to promote a capability

For a release or fleet rollout, run the focused repository tests, run the
real-client/image compatibility leg where applicable, and execute the relevant
live or DR probe. Update this page and the status manifest in the same change,
with the image version and evidence. Until then, use the
`active-regression`, `release-pending`, `repository-tested`, or `scope-limited`
wording above; never turn an active regression into a historical-only note.
