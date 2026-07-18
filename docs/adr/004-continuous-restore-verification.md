# ADR-004: Continuous dual-path restore verification

**Status:** Accepted (harness implemented; assertions and deployment pending — see plan.md Phase 6)
**Date:** 2026-07-18

## Context

The 2026-06 multipart corruption incident (ADR-002) had two halves. ADR-002 closed the first: write-path corruption went undetected because the canary never exercised multipart. The second half is about restores: **a backup existed, passed every object-level check that ran, and was still unrecoverable when actually needed.** `queue-api`'s litestream chain on `ord-devimprint` had no valid restore point for ~40 days, and nothing running in production could have said so, because nothing ever performed a restore. Object presence, `ContentLength`, canary health, even per-block HMAC validity of what was stored — none of these prove that what comes out of a restore is the artifact the application needs.

A second lesson from the same incident: the verification path must not share fate with the thing it verifies. A bug in ARMOR's read path (bf-24sxh7 made every multipart GET 500) would make a verifier built solely on the ARMOR read path go red for availability reasons — or worse, a verifier that only used ARMOR could never detect that ARMOR itself is the corrupting component, and it cannot prove the "ARMOR server is gone" DR scenario works at all.

## Decision

Backups stored through ARMOR are continuously proven restorable by a dedicated **restore-verifier** (`cmd/restore-verifier`, `internal/restoreverifier`), with these properties:

1. **Real restores, continuously.** Per bucket, the verifier restores (a) the most recent backup object set and (b) a random sample of historical objects, on an internal scheduling loop. "Verified" is defined as a completed restore with content assertions — never object presence, size, or canary health.
2. **Two independent paths, both must pass:**
   - the **ARMOR read path** (standard S3 GET through a live ARMOR instance), proving the production consumer experience;
   - **direct-to-ciphertext** (the `armor-decrypt` logic against raw B2 objects with the escrowed MEK, honoring the ADR-003 multipart layout), proving recoverability with no ARMOR server in the loop — the actual DR scenario.
   Divergence between the paths is itself a first-class failure signal (it localizes the fault to ARMOR's serving path vs. the stored data).
3. **Application-level assertions per artifact class,** beyond SHA-256 comparison: SQLite gets `PRAGMA integrity_check` plus row-count/recency probes; tar/gzip gets listing + sampled extraction; Parquet gets footer parse + a DuckDB row-count query through the range-read path (regression-testing range translation on real data); everything else gets the generic checksum path.
4. **Deployment form:** a long-running Deployment with an internal scheduling loop (per the workspace no-CronJobs convention), deployed via declarative-config, one instance covering all ARMOR buckets.
5. **Escalation, not retry:** every verification failure files a bead carrying object key, bucket, deployment, provenance writer version, and both-path evidence. Staleness (no verified restore within the freshness window) escalates identically. Escalation is one bead per distinct failure — the mechanism must be storm-proof (no per-tick re-filing, no unbounded retries; the 2026-07 NEEDLE retry-storms are the anti-pattern).
6. **Metrics:** per-bucket gauges (`armor_last_verified_restore_timestamp`, `armor_verified_object_ratio`, `armor_restore_verification_failures_total`) with alerting on restore-age and failures via declarative-config.

## Consequences

- Restore verification consumes real bandwidth and API calls (downloads through both paths). This is accepted cost: the Cloudflare path is free egress, and direct-B2 samples are bounded by the sampling policy.
- The verifier holds the MEK (it decrypts), so it is deployed with the same secret-handling posture as ARMOR itself.
- The dual-path requirement means the verifier links ARMOR's crypto internals rather than shelling out — envelope/multipart layout changes (ADR-003) must keep the verifier in lockstep; a layout change that forgets the verifier shows up as a direct-path failure, which is the intended tripwire.
- Boundary: ARMOR proves restorability of what ARMOR stores. Estate-wide restore proof for non-ARMOR streams (CNPG/barman, restic, Velero) belongs to a separate engine (DRILL); metric and bead conventions stay compatible so results can be aggregated.

## Current state (2026-07-18)

Implemented: harness with dual-path verification, SHA comparison, per-bucket state, status/trigger endpoints, metrics hooks. Not implemented: the three artifact-class assertions are stubs (`return nil`); no deployment manifests in declarative-config; no PrometheusRule/Grafana; no bead-filing escalation; no scheduled `armor-decrypt`-only DR drill. Tracked in plan.md Phase 6 beads.

**Known defect in the direct path (verified 2026-07-18):** `armor-decrypt` cannot read multipart objects at all — it fails with `invalid ARMOR magic` because it still implements the never-shipped reserved-byte envelope design (expects a 64-byte header at offset 0 and, for local files, a local sidecar path) instead of the shipped ADR-003 layout (headerless ciphertext, `x-amz-meta-armor-multipart` marker, sidecar object in B2). Until fixed, the "ARMOR server is gone" recovery path does not exist for exactly the object class that matters most (large backups). This is exactly the failure mode the dual-path tripwire is designed to catch — the tripwire fired on its first real use.
