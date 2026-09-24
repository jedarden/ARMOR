# Changelog

All notable changes to ARMOR, newest first. Each entry is written by
`scripts/cut-release.sh` in the same commit that bumps `VERSION`; CI (the
`armor-build` workflow in declarative-config) then publishes the images, tags
`v<version>`, and copies the entry into the Forgejo and GitHub releases.

The version number is `0.1.<counter>`: the third component only increases and
carries no SemVer meaning. Any release may contain fixes and features, so read
the entry rather than the number. A version cut whose build never published an
image is folded into the next version that did and is noted there.

Entries before 0.1.1958 were written by hand on the Forgejo releases and are
reproduced here; entries from 0.1.1958 on were reconstructed from git history
on 2026-09-18 (bead armor-43ec803f).

## 0.1.1973 (2026-09-24)

- fix(lint): clear the six staticcheck findings that failed armor-build lint (armor-359e1dc9)
- test(migration): fixture reproducibility gate (armor-8a72436e)
- test(migration): reconcile golden manifests with fixture tree (armor-73cb73d8)
- test(migration): add nine malformed fixture generators (armor-ce3af135)
- docs(dr): reconcile /admin/key/export doc with the real handler response (armor-bf3887f6)
- docs(armor): record the rs-manager MEK rotation and file its follow-ups (armor-0f01e341)
- test(migration): regenerate drifted fixtures with KWP wraps (armor-d0e65d93)
- fix(check): correct config and fingerprint probe status logic (armor-0f200adb)
- fix(compose): bump ARMOR_VERSION default to 0.1.1972 to match VERSION (armor-359e1dc9)
- feat(migration): expose classification in migrate CLI JSON report and docs (armor-f94e726d)
- feat(migration): classification wired into the countObjects inventory pass (armor-7f1aac2b)
- feat(migration): contradictory metadata category in ClassifyMigrationObject (armor-a20d398c)
- feat(migration): pure deterministic ClassifyMigrationObject classifier (armor-fd3069cf)
- fix(migration): enforce header plaintext SHA in decryptSingleObject (armor-7b39fc12)
- feat(migration): count reporting by source/layout/size/fingerprint/outcome (armor-796944b7)
- docs(readme): swap branch checks badge for a release badge (armor-46181ec8)
- feat(release): prepend the CHANGELOG entry to the release body (armor-4a20c3b3)
- fix(prefix): compose ARMOR_PREFIX into internal state and HMAC-sidecar writers (armor-01f79985)
- fix(verify): current formats, report rows, non-zero exit; delete verify-objects (armor-da67956d)
- fix(gate): verify git archives without VCS metadata (armor-c5d08177)
- feat(drift): deploy continuous fleet checker (armor-c5d08177)
- fix(acl): deny nil credential in CheckACL instead of panicking (armor-b7ace452)
- test(rbac): load armor-test credentials from env, not source (armor-ad708bfd)
- test(restoreverifier): tenant-prefix exclusion coverage for shared-bucket discovery (armor-bf592560)
- test(cmd/verify): re-scope TestVerifyV3CorruptedSidecar to the inline-verify boundary (armor-99447be8)
- fix(cmd/verify): derive test corruption offsets from object layout (armor-83760c9e)
- fix(restoreverifier): single shared discovery walk, bounded runs, drill sample reuse (armor-851dca86)
- test(srvtest): provider-outage restore drill from the secondary alone (armor-307cab32)
- fix(compose): bump ARMOR_VERSION default to 0.1.1971 to match VERSION (armor-0142d10a)
- test(crypto): durable v3 block benchmark harness (armor-aabc02b7)
- perf(crypto): one CTR stream per ARMOR block in EncryptBlockV3 (armor-2164a738)
- perf(crypto): one CTR stream per ARMOR block in both v3 decrypt paths (armor-7b40cb59)
- fix(restoreverifier): verify v3 multipart objects via manifest metadata and per-part decrypt (armor-86a90341)
- test(crypto): pin v3 CTR counter semantics with reference-keystream oracle (armor-eb539cde)

## 0.1.1972 (2026-09-23)

- feat(migration): expose classification in migrate CLI JSON report and docs (armor-f94e726d)
- feat(migration): classification wired into the countObjects inventory pass (armor-7f1aac2b)
- feat(migration): contradictory metadata category in ClassifyMigrationObject (armor-a20d398c)
- feat(migration): pure deterministic ClassifyMigrationObject classifier (armor-fd3069cf)
- fix(migration): enforce header plaintext SHA in decryptSingleObject (armor-7b39fc12)
- feat(migration): count reporting by source/layout/size/fingerprint/outcome (armor-796944b7)
- docs(readme): swap branch checks badge for a release badge (armor-46181ec8)
- feat(release): prepend the CHANGELOG entry to the release body (armor-4a20c3b3)
- fix(prefix): compose ARMOR_PREFIX into internal state and HMAC-sidecar writers (armor-01f79985)
- fix(verify): current formats, report rows, non-zero exit; delete verify-objects (armor-da67956d)
- fix(gate): verify git archives without VCS metadata (armor-c5d08177)
- feat(drift): deploy continuous fleet checker (armor-c5d08177)
- fix(acl): deny nil credential in CheckACL instead of panicking (armor-b7ace452)
- test(rbac): load armor-test credentials from env, not source (armor-ad708bfd)
- test(restoreverifier): tenant-prefix exclusion coverage for shared-bucket discovery (armor-bf592560)
- test(cmd/verify): re-scope TestVerifyV3CorruptedSidecar to the inline-verify boundary (armor-99447be8)
- fix(cmd/verify): derive test corruption offsets from object layout (armor-83760c9e)
- fix(restoreverifier): single shared discovery walk, bounded runs, drill sample reuse (armor-851dca86)
- test(srvtest): provider-outage restore drill from the secondary alone (armor-307cab32)
- fix(compose): bump ARMOR_VERSION default to 0.1.1971 to match VERSION (armor-0142d10a)
- test(crypto): durable v3 block benchmark harness (armor-aabc02b7)
- perf(crypto): one CTR stream per ARMOR block in EncryptBlockV3 (armor-2164a738)
- perf(crypto): one CTR stream per ARMOR block in both v3 decrypt paths (armor-7b40cb59)
- fix(restoreverifier): verify v3 multipart objects via manifest metadata and per-part decrypt (armor-86a90341)
- test(crypto): pin v3 CTR counter semantics with reference-keystream oracle (armor-eb539cde)

## 0.1.1971 (2026-09-19)

- fix(multipart): batch v3 range block fetches per part; int64 part offsets (armor-817d9d92)
- test(prefix): pin internal-writer .armor/ placement under ARMOR_PREFIX (armor-5850c682)
- docs(readme): name all nine armor subcommands in Repository structure (armor-96564423)
- test(drift-check): parent acceptance CLI fixture + tags_behind_version docs (armor-c843c9e7)
- test(drift-check): classify_fleet + mixed-fleet CLI coverage for the VERSION floor (armor-f469fe8d)
- feat(drift-check): VERSION floor for the approved latest (armor-9c1449b0)
- feat(scripts): list the ghcr.io restore-verifier mirror in the release body (armor-1ea3cb50)
- test(migration): encode contradiction failure in golden outcomes, disarm vacuous skips (armor-a4c372f7)
- test(migration): regenerate version_says_v1_layout_v2 as validated multi-block fixture (armor-c7b91d95)
- fix(crypto): commit v3 golden vectors, one authoritative generator, working -update flag (armor-5303ea09)
- docs(readme): rewrite Repository Structure, add contributing block and compose demo (armor-b14976f8)
- docs(release): restore-verifier image is publicly mirrored to GHCR (armor-1ea3cb50)
- docs(tests): one tests/README.md covering every suite; link it from the docs index (armor-75f52683)
- docs(archive): annotate pruned notes/ links as removed (armor-12ca1a63)
- feat(compose): track compose.yaml (demo + production profiles) and exercise it in the demo smoke test (armor-76be93d0)
- docs(config): move the full environment-variable reference to docs/configuration.md (armor-262777b7)
- fix(config): make named-credential errors deterministic and repair the scrubbed test fixtures so internal/config is green again (armor-927879ce)
- build(toolchain): declare go1.25.14 in go.mod, pin Dockerfile builders, add make toolchain-check (armor-2239f5b1)
- refactor(cmd): per-subcommand flag sets, armor help, subcommand list from --help (armor-da6e4721)
- feat(version): fall back to debug.ReadBuildInfo for go-install builds (armor-cd8d4712)
- docs(release): state semver-only image publishing (armor-0d603b24)
- docs(readme): move Authentication/ACL grammar into docs/authentication.md (armor-08246821)
- docs(readme): move offline-decrypt runbook into docs/disaster-recovery.md (armor-126ea386)

## 0.1.1970 (2026-09-18)

- fix(restoreverifier): size the v3 single-PUT trailer with BlockTableEntrySize on the ARMOR path (armor-8d4420fc)
- feat(cmd): add armor version --json with format_write_version (armor-fe7653ef)
- docs(readme): point quick start at the public GHCR image, add Install section (armor-6900d193)
- docs: rewrite the release process as a procedure; truth-up the docs index and plan status (armor-1ed07b10, armor-ac7faf72)
- drift: read release tags from the local checkout first, add --latest-tag, ignore workflow manifests, drop decommissioned clusters (armor-26be2614)
- docs(readme): GHCR-first quick start, subcommand table, complete configuration and endpoint references, current repo map (armor-0c37405f)
- docs: add AGENTS.md and a CLAUDE.md that imports it (armor-5e68c2ec)
- build: repair the Makefile, fix the go-version prefix in 'armor version', ignore local caches (armor-db388027)
- chore(tests): reduce tests/__init__.py to a package marker after the framework removal (armor-c70bb108)
- feat(scripts): publish_release.py, idempotent tag + Forgejo/GitHub release publisher (armor-4a03c2f6, armor-b00ee40c)
- chore: prune misleading clutter that no runner or deployment uses (armor-c70bb108)
- perf(tests): reproducible bounded throughput benchmark harness + runbook (armor-fd19a839)
- test(multipart): back off SDK-shape deferral retries like a real transfer manager (armor-13bf495b)
- feat(scripts): automate fleet version-drift detection with dedup alerts (armor-3d3cfdca)
- docs(client-config): embed multipart contract in every tool config; repair client-config tests (armor-fdbbd145)
- fix(srvtest): tolerate in-flight put temps in Snapshot listings (armor-114138b9)
- feat(metrics): complete restore verification alert signals
- test(replication): pin overwrite convergence and delete non-propagation for ADR-006 (armor-6a46d263)
- test(docker-demo): smoke test replaying the README Docker-only demo workflow (armor-8fb3e996)
- test(srvtest): secondary outage/slow windows never block acks and fully drain with gauge coherence (armor-e32d8c27)
- docs(multipart): client-concurrency compatibility matrix; reconcile README claim with ADR-003 §4 (armor-13bf495b)
- feat(tests): server-level dual-backend harness + primary-ack-to-secondary landing e2e (armor-8e26f4ef)
- feat(adr-006): publish replication queue stats to server metrics + credential-hygiene tests
- test(manifest): pre-prefix coexistence and prefixed provenance delta-walk coverage (armor-8636e2d5)
- feat(docs): enforce documentation-index consistency via internal/docsindex
- docs(dr): correct plaintext SHA metadata header in failover validation steps (armor-cdc2c6a7)
- test(migration): manifest-driven regeneration-set validation harness (armor-b8aa06b2)
- test(migration): deepen omission-variant fixture pins (armor-3aac661d)
- feat(migration): reader-aligned V1 explicit-version fixture (armor-53bcdd78)
- docs(dr): full ADR-006 provider-outage recovery procedure (armor-cdc2c6a7)
- docs(scripts): relabel remaining contract-sense ADR-005 citations to ADR-015 (armor-d8e0f80d)
- test(listing): dual-location internal-namespace filters under ARMOR_PREFIX (armor-0d31cf6e)
- feat(migration): deterministic standalone fixture generator foundation (armor-19065d7e)
- fix(multipart): relabel uniform-part-size contract citations ADR-005 -> ADR-015 (armor-d8e0f80d)
- fix: harden secondary replication configuration and queue
- test(config): confine ARMOR_MANIFEST_PREFIX to the tenant namespace (armor-d69329fe)
- docs(adr): repair ADR numbering and cross-references (armor-0d1596e3)
- feat: implement ADR-006 async secondary replication
- docs(metrics): document the restorability gauges and the declarative-config alert rules
- chore: stop tracking local worker runtime
- fix(beads): rebuild checkpoint from authoritative live store
- fix(checkpoint): repair JSON truncated by the gitleaks-remediation filter-repo pass
- test(migration): align golden harness with malformed/contradictory fixtures (armor-abbddad5)
- docs(migration): record complete-fixture-set golden validation delta (armor-45aeb954)
- docs(migration): document expected V3 outcome for every fixture dir (armor-1b0294b5)
- test(migration): pin fixture matrix with per-fixture pass/fail classes (armor-b47b5435)
- docs(migration): record 2026-09-14 golden fixture validation delta

## 0.1.1969 (2026-09-14)

- fix(decrypt): decrypt v3 multipart objects per part and pin the `cmd/armor` fixtures to the server wire form (armor-2eae59f1)
- fix(config): trim `ARMOR_ADMIN_TOKEN` at load so a provisioned trailing newline cannot silently disable the admin API
- fix(acl): `abort` is its own ACL verb; AbortMultipartUpload is no longer covered only by `delete` (armor-7bee0797)

Also carries the changes of 0.1.1966, 0.1.1967 and 0.1.1968 (cut 2026-09-13), whose builds did not publish an image:

- fix(restoreverifier): continue discovery past backend-filtered empty pages (armor-8290de05)
- fix(server): bound the startup manifest load (`ARMOR_MANIFEST_LOAD_TIMEOUT`, default 480s) so a slow cold start cannot crash-loop the pod
- fix(server): batch `DeleteObjects` reaches per-key ACL enforcement (armor-da872b29)
- fix(keymanager): resolve non-default named keys on an empty-keyID fingerprint lookup (armor-8317f313)
- fix(restore-verify): fetch multipart HMAC sidecars by the client key (armor-7edd1237)
- tooling: the workspace starvation watcher gains an empty-database integrity check

## 0.1.1965 (2026-09-13)

- fix(server): `GET /admin/key/ring` answers from the manifest instead of walking the bucket; `?census=head` keeps the full walk
- feat(server): golden-fixture validation harness for the V3 migration, with a validation report
- docs: consolidated V3 migration reference; catalog of malformed and edge-case fixture handling
- tooling: workspace-local starvation watcher over pluck diagnostics

## 0.1.1964 (2026-09-07)

- fix: serialize create-only B2 writes so two `If-None-Match: *` puts cannot both succeed (armor-1a098fe6)

## 0.1.1963 (2026-09-07)

- feat: honor create-only object puts (`If-None-Match: *`) (armor-1a098fe6)
- docs(mek-rotation): iad-kalshi rotation state recorded

## 0.1.1962 (2026-09-06)

- feat(bucket-alias): serve legacy bucket names from `ARMOR_BUCKET` via `ARMOR_BUCKET_ALIASES` (ADR-001 tenant consolidation)
- fix(manifest): compose the manifest prefix with `ARMOR_PREFIX` (armor-1f07c95f)
- fix(canary): apply the ADR-001 prefix to canary heartbeat keys
- fix(log): fingerprint bucket names and access-key IDs in startup output
- fix(key-rotation): check the LastKey resume skip before per-object inspection
- fix(dashboard): `/dashboard/presign` proxies to `/admin/presign`; `/dashboard` is exempt from the admin-token gate only when it has its own auth
- docs: unified-bucket tenant onboarding runbook; iad-ci MEK re-wrap recorded

## 0.1.1961 (2026-09-04)

- fix(multipart): load the v3 HMAC sidecar by the (hashed) client key, not the prefixed key

## 0.1.1960 (2026-09-04)

- fix(rotation): resume interrupted rotations and checkpoint per object

Also carries the changes of 0.1.1958 and 0.1.1959 (cut 2026-09-04), whose builds did not publish an image:

- feat(admin): manifest repair and quarantine endpoints for stale ciphertext reads (armor-47c72ba1)
- fix(handlers): invert the `verifyCiphertextFreshness` staleness comparison
- fix(config): record the `ARMOR_AUTH_FILE` path so hot-reload actually starts (armor-ed6f9783)
- fix(admin): admin listener read/write timeouts are configurable (`ARMOR_ADMIN_READ_TIMEOUT`, `ARMOR_ADMIN_WRITE_TIMEOUT`) so long rotations survive (armor-84077175)
- feat(migration): explicit target validation, include-parameter normalization, persisted failure records, corrected processed/skipped counters, no state resume across the dry-run/live boundary
- test/docs(migration): V1/V2 fixture generators and V3 conversion catalogs

## 0.1.1957 (2026-09-01)

Fixes production multipart uploads through slow B2 backends by allowing distinct format-v3 parts to reach the backend concurrently. Same-part retries remain serialized, and legacy v2 retains its shared-state lock.

Verified by Argo workflow armor-build-n8hvc: lint, normal and race release gates, integration tests, server/restore-verifier/fleet/GHCR builds, registry checks, and published-image compatibility all succeeded.

## 0.1.1956 (2026-09-01)

Production fix release for multi-gigabyte S3 request lifecycles. Disables the public S3 server global ReadTimeout and WriteTimeout, which terminated valid long-running multipart uploads/completion responses, while retaining 30-second read-header and 2-minute idle connection protections. Includes the multipart manifest finalization and key-ring parser fixes from v0.1.1954 plus subsequent correctness fixes.

## 0.1.1955 (2026-09-01)

Superseded by 0.1.1956. Do not deploy for multi-gigabyte workloads: disabling WriteTimeout alone was insufficient; the remaining global ReadTimeout still allowed a valid long-running S3 upload lifecycle to end with a closed connection.

## 0.1.1954 (2026-09-01)

Superseded by 0.1.1955. Do not deploy for multi-gigabyte workloads: multipart finalization can legitimately exceed the S3 server's hard-coded 30-minute WriteTimeout, closing the client connection before a valid response.

## 0.1.1953 (2026-08-31)

Superseded by 0.1.1954: misclassified `ARMOR_MEK_RING` during startup and is not deployable where that variable is present.

Critical correctness release for large multipart objects:

- Replaces the post-completion whole-object metadata CopyObject with the ADR-016 manifest finalization protocol, removing Backblaze B2's 5 GiB CopyObject ceiling.
- Completes nested filesystem multipart uploads and hardens v3 production read paths.
- Adds logical 5 GiB boundary, timeout, retry, manifest, and client compatibility coverage.
