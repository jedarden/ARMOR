# ADR-002: Close detection gaps that let the multipart-upload corruption bug run 40 days undetected

**Status:** Accepted
**Date:** 2026-07-11

## Context

Commits `b96d7eb`, `3a1526a`, and `c66e264` (landed 2026-06-10/11, versions 0.1.35–~0.1.41) fixed a real correctness bug: ARMOR only routed multipart-upload `POST` calls; a standard client's `PUT ?partNumber&uploadId` fell through to plain `PutObject`. Each part got stored as if it were the whole object ("last part wins") while B2's real multipart upload never completed. Any object ≥5MiB written through an affected ARMOR instance via multipart semantics was silently corrupted — the write reported success, but the stored bytes were wrong.

`ord-devimprint`'s ARMOR deployment was on `0.1.19` (last bumped 2026-05-31, before the fix) and was never caught up. This was discovered 2026-07-10/11 while investigating why `queue-api`'s litestream backup couldn't be restored: the backup's only surviving snapshot didn't decode, and two more objects were corrupted live *during the investigation*. Net effect: **queue-api's B2 backup chain had no valid restore point for a ~40-day window, and this was never flagged by anything running in production.** Full incident trail in project memory (`project_commitgraph_split`).

This same fix had already been deployed to `iad-native-ads` after an earlier incident there (barman WAL archiving). `ord-devimprint` was simply missed in that remediation pass. Two more clusters (`iad-ci` at 0.1.24, `iad-kalshi`/`rs-manager` at 0.1.13) are still on old versions and have not been checked.

### Why existing safeguards didn't catch this

ARMOR already has real integrity infrastructure — this incident is not a design gap in the crypto/verification model, it's a **coverage** gap in what that infrastructure actually exercises:

1. **The self-healing canary monitor** (`internal/canary`) has been running continuously in production since Phase 1, checking upload→download→decrypt→HMAC-verify→SHA-verify every 5 minutes, and has been reporting healthy the entire time. It uses a fixed `canarySize` of 1024 bytes (`internal/server/server.go:117`) and calls `backend.Put()` directly — a single-object write. **It structurally never exercises `CreateMultipartUpload`/`UploadPart`/`CompleteMultipartUpload`**, which is exactly where the bug lived. A monitor built to catch exactly this class of regression has a blind spot sized precisely to miss it.
2. **`TestMultipartUpload`** (`tests/integration/integration_test.go:525`) already does a real 3-part, 15MB multipart upload/download cycle — but only asserts `ContentLength` matches the expected size. It never reads back and compares the actual bytes against what was uploaded. A corruption that happens to preserve object length (or a test/CI setup where this particular test isn't gating deploys) can pass silently.
3. **No mechanism tracks version drift** across ARMOR's several cluster deployments. A correctness fix landed on `main` and reached one cluster after a prior incident; nothing flagged that three other clusters (`ord-devimprint`, `iad-ci`, `iad-kalshi`/`rs-manager`) were still running the vulnerable version.

## Decision

Close all three gaps. This ADR does **not** propose changes to the encryption/HMAC/provenance design — that infrastructure is sound for correctly-routed requests. It closes the coverage and operational gaps that let a routing regression run undetected.

1. **Extend the canary monitor to cover the multipart path.** Add a second, longer-interval check (e.g., once per hour, alongside the existing 5-minute small-object check) that uploads a canary object sized just above the multipart threshold via real `CreateMultipartUpload`/`UploadPart`×N/`CompleteMultipartUpload` calls, then downloads and verifies byte-for-byte + HMAC + plaintext-SHA, reusing the existing `Monitor`'s verification logic rather than duplicating it. Surface a distinct `multipart_healthy` status alongside the existing canary status in `/armor/canary` and Prometheus metrics, so a multipart-specific regression is visible independently of the small-object check staying green.
2. **Strengthen `TestMultipartUpload` (and add sibling tests) to verify actual content, not just length.** Read the downloaded object back and compare bytes against what was uploaded per-part (e.g., seed each part with distinguishable content, not just a single marker byte). Add a variant that also exercises a non-multiple-of-part-size final part and a single-part multipart upload (edge cases the current test doesn't cover). Confirm this test actually gates the build pipeline that produces deployed images — a real content-correctness bug should never be able to ship.
3. **Add a lightweight version-drift check across known ARMOR deployments.** A scheduled check (can live in this repo or as an ops script) that reads each cluster's deployed ARMOR image tag from `declarative-config` and compares against the latest GitHub release, flagging any deployment more than N releases or M days behind — and flagging correctness-labeled releases distinctly from routine bumps, since those are the ones where staying behind is actually dangerous.
4. **Document a fix-propagation checklist.** Any commit fixing a correctness/data-integrity bug must enumerate every known ARMOR deployment (see the cluster list in `docs/disaster-recovery.md` / declarative-config) and confirm each is either patched or explicitly tracked as pending — before the fix is considered resolved, not just merged.

## Consequences

- Canary checks get more expensive (a multipart check involves more round-trips than the existing single-object check) — mitigated by running it on a longer interval than the existing 5-minute check.
- The version-drift check and propagation checklist are process/tooling additions, not core ARMOR features — they can live alongside the codebase without changing ARMOR's own request-handling surface.
- This does not retroactively recover any data corrupted before the fix was deployed everywhere — see `docs/disaster-recovery.md` for what's recoverable. It only prevents a similar regression from running undetected for weeks next time.

Related: `docs/disaster-recovery.md`, project memory `reference_armor_multipart_corruption_bug`, `project_commitgraph_split`.
