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
