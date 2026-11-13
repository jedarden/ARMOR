# Release status and known limitations

**Status date:** 2026-11-13 · **Source version:** see [`VERSION`](../VERSION)

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

<!-- release: latest-green version: 0.1.2012 evidence: armor-fdab65fe -->
**`v0.1.2012`** (`76a80d623abd62b47601a6e8a4fa11d0cbdcf06b`) is the latest fully
published, compat-green release. The repaired `armor-build-manual-cnfsq` run
completed successfully at 2026-11-13T00:40Z after the go-test resource fix in
declarative-config revision `80be9a652` (3 GiB memory request/limit and
`GOMAXPROCS=4`). Its VictoriaLogs evidence query was
`{cluster="iad-ci",namespace="argo-workflows"} kubernetes.pod_name:armor-build-manual-cnfsq`
over the run window; the go-test pod completed without eviction and the final
status pod posted `success` for the release revision. The real-client
compatibility suite passed its AWS CLI, rclone, boto3, DuckDB/httpfs,
litestream, and barman-cloud legs.

The publisher created the Forgejo tag/release and mirrored GitHub release;
`v0.1.2012` is present on `origin` and GitHub confirms the release is
non-draft. Registry verification recorded these immutable manifests:

| Image | Digest |
|---|---|
| `ronaldraygun/armor:0.1.2012` | `sha256:10d38bea6da5fd2123acaaab3e9dff047cd83edab407b8721726c8e13f22bfea` |
| `ronaldraygun/armor-restore-verifier:0.1.2012` | `sha256:5ecdaccf85ce89a42931148b18a0dca7354792f82471311ac8752dfd0cad6cab` |
| `ronaldraygun/armor-fleet:0.1.2012` | `sha256:727fda92c5a5e30233a8c3fb9573d203728e3a04c1d55eb25c907f51d99b95cd` |
| `ghcr.io/jedarden/armor:0.1.2012` | `sha256:01581ac19e72429f91d2349c96acd959217ca92531e0e4fbb1335c10a4e213d5` |

The exact-SHA `armor-integration` decision remains owned by `armor-033dd376`;
this release record covers the armor-build gate only. Consumers should pin by
the recorded digests, not by tag alone.

## Current release posture

<!-- capability: named-key-read status: repository-tested evidence: armor-8317f313 -->
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
| Named-key and ring-key reads | `repository-tested` | Real key-manager and multi-key routing tests cover fingerprint selection and unknown metadata. The fix (`e67804d6`) shipped in 0.1.1969 and `armor-8317f313` is closed. | **0.1.1969:** the bead records a published-image named-key PUT/GET round trip and image digest. | Recheck the target deployment's image version and named-key/ring configuration before relying on it for a specific fleet member. |
| Fingerprinted wrapped-DEK decoding | `repository-tested` | CLI, server, and restore-verifier tests cover the `v2:<fingerprint>:<base64>` form and legacy values. `armor-4b1b6c64` and `armor-da67956d` are closed. | **None recorded** for the DR/escrow path; the client image matrix does not exercise it. | Verify the exact escrow package, key ring, and object population in a DR drill before making a deployment claim. |
| Multipart HMAC sidecar and read paths | `repository-tested` | V2/v3 full-read, range-read, prefix, sidecar, and corruption tests cover the current layouts. The prefix lookup fixes in `armor-1b272971` and `armor-7edd1237` are closed. | **None recorded** for v3 sidecar/DR behavior. The 0.1.1975 client harness writes v2 objects. | Preserve `.armor/hmac/` and verify existing multipart objects before migration or failover. |
| V3 multipart verification in CLI and DR | `repository-tested` | `armor verify`, `armor decrypt`, and restore-verifier tests cover manifest fallback, sidecar HMACs, part counters, and digests. `armor-86a90341` and `armor-da67956d` are closed. | **None recorded** for the CLI/DR path; the client image matrix exercises server S3 operations. | A server GET is not a DR drill. Run the verifier against real objects with the intended image and credentials. |
| SigV4 authentication, including AWS CLI | `repository-tested` | Request-shape and real-client tests cover accepted and rejected credentials; `armor-a3b04ef2` and `armor-4795decf` are closed. The six-client matrix binds AWS CLI authentication to its PUT/GET round trip. | **0.1.1975** is the last recorded AWS CLI image proof; it predates the six-client matrix. Backfilled tags through 0.1.1988 have no completed matrix proof. | AWS CLI works in the tested source paths. Check the image and credentials of the deployment before claiming its access is verified. |
| Presigned/share GET | `repository-tested` | The enabled in-process harness passes share GET and range/error cases; `armor-0163fb32` closed after correcting its test configuration. | **None recorded**; the share GET test skips external-image endpoint mode. | Enable `ARMOR_PRESIGN_ENABLED` and configure the base URL and secret, then probe the deployed route. |
| Byte-range reads | `active-regression` | Protocol and handler tests cover byte/status correctness; `armor-817d9d92` closed the large multipart read failure, `armor-73564f97` and `armor-5694713b` closed bounded read-ahead and offset coalescing, and `armor-272299ca` closed whole-part v3 multipart GET streaming (block-by-block through one pipelined range; on `main`, not yet in a release). [Local baseline](performance/large-object-range-read-baseline.md) and the [live qualification](performance/production-b2-cloudflare-2026-09-28.md) bound performance evidence. `armor-68e3d5a9` recorded the 2026-10-24 rerun; `armor-ebd22a99` remains open. | **0.1.1975** predates the six-client range matrix; the later backfilled tags do not prove the streaming read path. The 2026-10-24 qualification on 0.1.1993 completed the full workload including all five range shapes on the per-block v3 path, and measured ARMOR full-GET plaintext throughput at 2.0 MB/s and ranged reads at 1.4–57 s p50; the coalesced range path (`792a619b`) and the streamed multipart GET are not yet in a release. | Active regression boundary is large-object throughput and bounded reads; the live qualification measured that boundary on one runner and one replica and is not a production performance claim. |
| ADR-006 provider-outage failover and failback | `repository-tested` | The tagged `TestProviderOutageFailoverWritesAndFailback` filesystem drill and backend tests prove the explicit promotion sequence; `armor-14a2e34c` is closed. | **None recorded** for a live provider outage. | Run the live drill and record the image version and recovery inventory before treating a deployment as proven. |
| S3 operation surface | `scope-limited` | The current six-client matrix covers authentication, reads, ranges, listing, overwrite/delete, and multipart where supported. Bucket administration (create/delete/root listing) and S3 object lock are explicitly outside the matrix: the exact-SHA `armor-integration` gate excludes both families by default (2026-10-31, armor-be3d322a) so every remaining gate failure is a claimed-operation failure. | **0.1.1975** is the last completed client-image evidence and predates that matrix; backfilled tags through 0.1.1988 have no completed matrix proof. | Use the matrix for the exact image under evaluation; this is a scoped operation claim. Bucket administration and object lock are unclaimed — see the known-limitations entry below; the gate exclusion is not evidence that they work. |

## Known limitations

- **`0.1.2002` was tagged but never released.** `armor-build-manual-9v4rg`
  resolved and built the correct release commit
  `f2be11e911a2ebe3012bd5e72d7f63327bfada12`
  (`resolve-version`: "releasing VERSION=0.1.2002 from f2be11e9…"), and
  kaniko pushed the registry tag before the compatibility gate ran; the
  compat leg then failed on user-metadata handling (fixed by `2d2b679e` and
  `c36637f3`). No `v0.1.2002` tag or Forgejo/GitHub release exists. Treat
  `0.1.2002` images as artifacts of a failed gate, not a release: do not
  deploy them. See the `v0.1.2004` record above for the next published
  version.
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
- **Bucket administration and S3 object lock are outside the release claim.**
  ARMOR is a single-bucket encryption proxy: every deployment configures one
  bucket, and every fleet B2 credential is an application key scoped to that
  bucket. A bucket-scoped key cannot create or delete buckets, and "listing"
  in the claim matrix means object listing inside the configured bucket —
  root `ListBuckets` is not claimed. The `CreateBucket`/`DeleteBucket`/
  `ListBuckets` handlers exist as backend pass-throughs (dispatch is
  classified in `s3_operation.go` for metrics), but a backend denial under a
  bucket-scoped key surfaces as a generic `500 InternalError`, not a
  client-scoped error — observed in the `armor-integration` nightly on the
  iad-ci scope. Object Lock is unsupported: `?object-lock`, `?retention`,
  and `?legal-hold` return `501 NotImplemented`, as do
  `x-amz-object-lock-mode`, `x-amz-object-lock-retain-until-date`, and
  `x-amz-object-lock-legal-hold` request headers. ARMOR rejects these
  requests before reading or forwarding their bodies, so clients cannot
  mistake an unprotected object for a WORM-protected one. Do not point S3
  clients that depend on object lock at ARMOR. Both families are excluded
  from the exact-SHA `armor-integration`
  gate's default `skip-tests` (scope decision 2026-10-31, armor-be3d322a;
  pattern in `k8s/iad-ci/argo-workflows/armor-integration-workflowtemplate.yml`)
  so the gate's remaining failures are all claimed-operation failures — the
  exclusion is not evidence that these operations work, and claimed-operation
  tests must never be added to that pattern. The explicit rejection is a
  safety boundary, not an object-lock support claim.
- **Restore-verifier alerting is not yet a fleet-wide release claim.** The
  GitOps rollout now contains the scrape, rule-evaluation, Alertmanager route,
  and tailnet verification surfaces for all five verifier scopes. The live
  smoke test passes on `iad-ci` and `rs-manager` (14/14 checks each). The
  `iad-kalshi` and `ord-devimprint` endpoints were unreachable during the
  2026-09-29 check. On 2026-10-16, `ardenone-cluster` Prometheus accepted the
  new `0.1.1993` verifier scrape (`health=up`, empty `lastError`), confirming
  the numeric exposition fix (`armor-c866e4ca`) reached that target. Its
  verification ratio was 7/11 then (four v3 multipart sidecar lookups 404 at
  the tenant path, 403 at the root fallback); on the 0.1.2004 roll it is
  9/11 (2026-10-30, improved from 8/11 across verification cycles on the
  same image), with the remaining two v3 sidecar reads returning B2 403
  `not entitled` (`armor-69b4983d`, proposal pending).
  Treat restore failures as paged only on the two smoke-verified clusters
  until the remaining endpoint and restore checks pass; a running verifier
  without a passing scrape and delivery path is not an alerting guarantee.
  See the [restore-verifier alerting runbook](runbooks/restore-verifier-alerting.md)
  for the exact commands and current boundary.
- **Fleet rollout evidence (2026-10-30, `armor-769e3d53`).** The 15 ARMOR
  and restore-verifier manifests across `iad-ci`, `iad-kalshi`,
  `ord-devimprint`, `rs-manager`, `apexalgo-iad`, and `ardenone-cluster` pin
  the latest fully published release, `0.1.2004`, by digest:
  `armor` `sha256:f39dbcaca179…` and `armor-restore-verifier`
  `sha256:572046ce8f6…`. The GitOps commits are `1ac8d6546` (iad-ci),
  `f5b932f23` (armor-test), `3ca9918c3` (iad-kalshi), `e4a641c72`
  (ord-devimprint), `145bbf4c3` (rs-manager), and `c99284d11`
  (ardenone-cluster plus apexalgo-iad); all were pushed to declarative-config
  `origin/main`. `armor-test` has the required digest pin, and the inventory
  script reports 15 current pins with no remaining 0.1.1993 references.
  After PVC/startup convergence, read-only live inspection on 2026-10-30
  found all 15 workloads Ready with zero restarts on the pinned digests,
  re-confirmed at 09:00–09:16Z and again 13:30–14:05Z. Four of five verifier
  scopes completed 11/11 runs on 0.1.2004: `iad-ci` (latest cycle 10:59Z),
  `rs-manager` armor-apexalgo (latest 12:07Z), `iad-kalshi` kalshi-tape
  (latest 12:15Z), and `ord-devimprint` devimprint (latest 13:29Z, a full
  1h22m re-discovery cycle over 19.2M listed keys). `ardenone-cluster`
  completes every cycle but fluctuates 7–9 of 11 by sample (8/11 00:06Z,
  9/11 06:06Z, 7/11 12:06Z on the same image): failed samples are legacy v3
  sidecar reads returning B2 403 `not entitled`, concentrated in two cache
  prefixes (`options-hypothesis-suite-v1`, `oi-positioning-volume-pilot-v1`)
  plus two sha256-named objects — the pre-roll root-sidecar writes
  (`armor-69b4983d`, proposal pending, unrelated to the image; the same
  failures were present on 0.1.1993). Stranded `ContainerStatusUnknown` pods from
  dead nodes on `iad-kalshi`/`ord-devimprint` predate the roll and are
  excluded from the running fleet by the drift probe
  (`armor-9a947928`). The live drift check now verifies all 15 deployments
  current against tag and registry digest (15 current / 0 stale / 0
  mismatched / 0 unavailable, `needs_update=0`; reproduced 2026-10-30
  09:16Z and 14:05Z with
  `--cluster-api iad-ci=http://traefik-iad-ci:8001`, since the committed
  iad-ci URL is the in-cluster address whose serviceaccount token a
  codinghome run does not have — the default invocation reports those three
  entries unavailable with `needs_update=0`); its exit code stays 1 solely
  on the `tags_behind_version` warning because the 0.1.2006 cut
  (2026-10-30 11:16Z) is ahead of the newest recorded release `v0.1.2004`.
  The 0.1.2006 build pushed the git tag and the public
  `ghcr.io/jedarden/armor:0.1.2006` server image (verified anonymously
  2026-10-30, with `0.1.2004` present and `0.1.2005` absent as controls),
  but the Forgejo and GitHub releases are absent — `publish_release.py`
  died between the tag push and release creation, whose standing resolution
  is a fresh cut, never a resubmit; the private `armor-restore-verifier`
  0.1.2006 tag cannot be verified from codinghome this session. A fleet
  re-roll to the next fully published release carries the list
  HEAD-fan-out default raise (`armor-45abc172`) and
  bind-before-manifest-load (`armor-3b7595b6`) — until then this warning is
  a release-chain state, not fleet drift. The ardenone scrape wiring was
  reverified
  structurally for this roll (ServiceMonitor → Service port `metrics`/9002
  → pod, Prometheus selecting all ServiceMonitors; the Endpoints object
  carried exactly one ready address — the running 0.1.2004 verifier pod —
  on 2026-10-30); a fresh target-state read remains blocked from codinghome
  on two independent paths (the Traefik vpn entrypoint on 443 is not
  ACL-granted to `tag:codinghome`, and `services/proxy`/`pods/proxy` are
  Forbidden to the read-only observer), so the 0.1.1993 `health=up`
  acceptance above carries over via the numeric-exposition fix being part
  of 0.1.2004. This
  is not a client-matrix or DR-drill proof (see the capability table above).

## How to promote a capability

For a release or fleet rollout, run the focused repository tests, run the
real-client/image compatibility leg where applicable, and execute the relevant
live or DR probe. Update this page and the status manifest in the same change,
with the image version and evidence. Until then, use the
`active-regression`, `release-pending`, `repository-tested`, or `scope-limited`
wording above; never turn an active regression into a historical-only note.

The `latest-green` marker near the top of this page is the machine-read
record of the newest fully published, compat-green release. The README Quick
Start's `export ARMOR_VERSION=` line and every `compose.yaml`
`${ARMOR_VERSION:-...}` default must pin that same version — a cut whose
publication has not completed leaves them where they are, so consumers never
land on a tag without releases. `scripts/record-green-release.sh <version>`
moves the marker and both pin surfaces in one edit once a publication is
verified green, and `scripts/compose-version-parity.sh` (definition of done,
release gate, `make docker`) fails any tree where the three disagree.
