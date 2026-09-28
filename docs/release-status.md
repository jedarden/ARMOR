# Release status and known limitations

**Status date:** 2026-09-28 · **Source version:** see [`VERSION`](../VERSION)

This page is the evidence boundary for operator documentation. “Repository
tested” means the committed tree has focused tests for the behavior. It does
not prove that a published image, a particular fleet deployment, or a
credential/key-ring combination has exercised that behavior. “Release-pending”
means that release-image or live deployment evidence is still required before
an operator should treat the capability as verified. The compatibility and DR
pages link here so their examples do not silently become release claims.

## Current release posture

<!-- capability: named-key-read status: release-pending -->
<!-- capability: wrapped-dek-decoding status: release-pending -->
<!-- capability: multipart-hmac-read status: release-pending -->
<!-- capability: v3-multipart-verification status: release-pending -->
<!-- capability: sigv4-authentication status: release-pending -->
<!-- capability: presigned-get status: release-pending -->
<!-- capability: range-reads status: repository-tested -->
<!-- capability: s3-operation-surface status: scope-limited -->

| Capability | Status | What the source tree demonstrates | Operator boundary |
|---|---|---|---|
| Named-key and ring-key reads | `release-pending` | Focused key-manager and multi-key routing tests cover fingerprint selection and rejection of unknown metadata. | On a deployment, confirm the running image and the named key/ring census before relying on non-default keys. Historical failure: `armor-8317f313`. |
| Fingerprinted wrapped-DEK decoding | `release-pending` | CLI, server, and restore-verifier tests cover the `v2:<fingerprint>:<base64>` form and legacy values. | A DR drill must exercise the exact escrow package and object population; old images had decode/version failures. Historical incident: `armor-4b1b6c64`. |
| Multipart HMAC sidecar and read paths | `release-pending` | In-tree v2/v3 full-read, range-read, prefix, sidecar, and corruption tests cover the current layouts. | Preserve `.armor/hmac/` and deploy the sidecar reader fix before trusting existing multipart data. A historical prefix mismatch caused 500s: `armor-1b272971`. |
| V3 multipart verification in CLI and DR | `release-pending` | `armor verify`, `armor decrypt`, and restore-verifier tests cover manifest fallback, sidecar HMACs, part counters, and digest outcomes. | Prove it against the published image and a real v3 multipart object; old completed objects with empty B2 metadata were not visible to the verifier. Historical incident: `armor-86a90341`. |
| SigV4 authentication | `release-pending` | The protocol suite and request-shape tests cover accepted and rejected credentials, including AWS CLI and barman-shaped requests. | Run the real AWS CLI against the release image and inspect the server-side denial class. A live rejection remains a release/endpoint concern: `armor-a3b04ef2`. |
| Presigned/share GET | `release-pending` | Enabled-harness tests cover share GETs and range/error responses. | `ARMOR_PRESIGN_ENABLED`, absolute base URL, and the dedicated secret must be configured; test the enabled route, not only the admin URL. Historical 403 harness failure: `armor-0163fb32`. |
| Byte-range reads | `repository-tested` | Protocol and handler tests cover unaligned, suffix, multipart-boundary, prefix, and corruption cases; the focused [large-object range baseline](performance/large-object-range-read-baseline.md) records local latency and backend-byte shape. | The baseline is loopback + filesystem only, not B2/Cloudflare production evidence. Coalesced fetches, bounded streaming, and production-shaped performance remain open work (`armor-73564f97`, `armor-5694713b`, `armor-272299ca`). |
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
  tails, or bounded-memory behavior; the open read-ahead work still concerns
  those limits.
- **Compatibility is scoped.** The supported examples are the operations in
  the compatibility matrix, against the tested image and configuration. Do not
  generalize them to untested S3 features or to a deployment whose image or
  configuration has not been checked.

## How to promote a capability

For a release or fleet rollout, run the focused repository tests, run the
real-client/image compatibility leg where applicable, and execute the relevant
live or DR probe. Update this page and the status manifest in the same change,
with the image version and evidence. Until then, use the `release-pending`,
`repository-tested`, or `scope-limited` wording above.
