# ADR-004: Continuous dual-path restore verification

**Status:** Accepted (implemented and deployed — the artifact-class assertions and the deployment manifests have both landed; the deployment form that shipped is one restore-verifier Deployment per bucket scope rather than one fleet-wide instance, see the [Addendum: Per-Cluster Deployment Form](#addendum-per-cluster-deployment-form-2026-09-25) and the [Fleet topology](#fleet-topology-scope-discovery-secrets-metrics-alerting-2026-09-25) sections below. Phase record: plan.md Phase 6)
**Date:** 2026-07-18

## Context

The 2026-06 multipart corruption incident (ADR-002) had two halves. ADR-002 closed the first: write-path corruption went undetected because the canary never exercised multipart. The second half is about restores: **a backup existed, passed every object-level check that ran, and was still unrecoverable when actually needed.** `queue-api`'s litestream chain on `ord-devimprint` had no valid restore point for ~40 days, and nothing running in production could have said so, because nothing ever performed a restore. Object presence, `ContentLength`, canary health, even per-block HMAC validity of what was stored — none of these prove that what comes out of a restore is the artifact the application needs.

A second lesson from the same incident: the verification path must not share fate with the thing it verifies. A bug in ARMOR's read path (bf-24sxh7 made every multipart GET 500) would make a verifier built solely on the ARMOR read path go red for availability reasons — or worse, a verifier that only used ARMOR could never detect that ARMOR itself is the corrupting component, and it cannot prove the "ARMOR server is gone" DR scenario works at all.

## Decision

Backups stored through ARMOR are continuously proven restorable by a dedicated **restore-verifier** (`cmd/restore-verifier`, `internal/restoreverifier`), with these properties:

1. **Real restores, continuously.** Per bucket, the verifier restores (a) the most recent backup object set and (b) a random sample of historical objects, on an internal scheduling loop. "Verified" is defined as a completed restore with content assertions — never object presence, size, or canary health.
2. **Two independent paths, both must pass:**
   - the **ARMOR read path** (standard S3 GET through a live ARMOR instance), proving the production consumer experience;
   - **direct-to-ciphertext** (the `armor decrypt` logic against raw B2 objects with the escrowed MEK, honoring the ADR-003 multipart layout), proving recoverability with no ARMOR server in the loop — the actual DR scenario.
   Divergence between the paths is itself a first-class failure signal (it localizes the fault to ARMOR's serving path vs. the stored data).
3. **Application-level assertions per artifact class,** beyond SHA-256 comparison: SQLite gets `PRAGMA integrity_check` plus row-count/recency probes; tar/gzip gets listing + sampled extraction; Parquet gets footer parse + a DuckDB row-count query through the range-read path (regression-testing range translation on real data); everything else gets the generic checksum path.
4. **Deployment form:** a long-running Deployment with an internal scheduling loop (per the workspace no-CronJobs convention), deployed via declarative-config, one instance covering all ARMOR buckets. *(The "one instance" half was amended on 2026-09-25 — see the [Addendum: Per-Cluster Deployment Form](#addendum-per-cluster-deployment-form-2026-09-25) and the [Fleet topology](#fleet-topology-scope-discovery-secrets-metrics-alerting-2026-09-25) sections below.)*
5. **Escalation, not retry:** every verification failure files a bead carrying object key, bucket, deployment, provenance writer version, and both-path evidence. Staleness (no verified restore within the freshness window) escalates identically. Escalation is one bead per distinct failure — the mechanism must be storm-proof (no per-tick re-filing, no unbounded retries; the 2026-07 NEEDLE retry-storms are the anti-pattern).
6. **Metrics:** per-bucket gauges (`armor_last_verified_restore_timestamp`, `armor_verified_object_ratio`, `armor_restore_verification_failures_total`) with alerting on restore-age and failures via declarative-config.

## Consequences

- Restore verification consumes real bandwidth and API calls (downloads through both paths). This is accepted cost: the Cloudflare path is free egress, and direct-B2 samples are bounded by the sampling policy.
- The verifier holds the MEK (it decrypts), so it is deployed with the same secret-handling posture as ARMOR itself.
- The dual-path requirement means the verifier links ARMOR's crypto internals rather than shelling out — envelope/multipart layout changes (ADR-003) must keep the verifier in lockstep; a layout change that forgets the verifier shows up as a direct-path failure, which is the intended tripwire.
- Boundary: ARMOR proves restorability of what ARMOR stores. Estate-wide restore proof for non-ARMOR streams (CNPG/barman, restic, Velero) belongs to a separate engine (DRILL); metric and bead conventions stay compatible so results can be aggregated.

## Current state (2026-07-18)

Implemented: harness with dual-path verification, SHA comparison, per-bucket state, status/trigger endpoints, metrics hooks. Not implemented (as of this snapshot): the three artifact-class assertions are stubs (`return nil`); no deployment manifests in declarative-config; no PrometheusRule/Grafana; no bead-filing escalation; no scheduled `armor decrypt`-only DR drill. Tracked in plan.md Phase 6 beads — every item in this list has since landed; see plan.md Phase 6 for the current record.

**Escalation enablement (2026-09-25, armor-babc0b2b):** the bead-filing
escalation of §5 is deployed on `iad-ci/armor` — the restore-verifier image
ships the canonical bead-rs CLI (pinned `bead` release binary, checksum-pinned
at build; the runtime stage moved `scratch` → `debian:bookworm-slim` because
the release binary is glibc-dynamic), a PVC (`sata`) backs the persisted
dedupe state and the beads workspace, startup validation (CLI runnable,
volume writable, workspace provisioned/opened) logs `ESCALATION FILING
DISABLED —` with the remediation instead of crash-looping the verifier, and
every filing carries a `--unique-ref` so the bead store itself rejects
duplicates (a lost state file cannot file twice). The other three
restore-verifier Deployments keep escalation off until each gets the volume
+ env. See the [restore-verifier deployment guide](../restore-verifier-deployment-guide.md).

**Known defect in the direct path (verified 2026-07-18; fixed 2026-07-19, bf-5jc1j8):** `armor decrypt` could not read multipart objects at all — it failed with `invalid ARMOR magic` because it implemented the never-shipped reserved-byte envelope design (expected a 64-byte header at offset 0 and, for local files, a local sidecar path) instead of the shipped ADR-003 layout (headerless ciphertext, `x-amz-meta-armor-multipart` marker, sidecar object in B2). The "ARMOR server is gone" recovery path therefore did not exist for exactly the object class that matters most (large backups) — the failure mode the dual-path tripwire is designed to catch, which fired on its first real use. **Fixed:** `decryptB2` now dispatches on the `x-amz-meta-armor-multipart` marker (mirroring the server's GET path and the restore-verifier's direct path): for multipart objects it reads headerless ciphertext from offset 0, loads the JSON HMAC sidecar via `MultipartStateManager.LoadHMACTable`, and verifies with absolute block indices; single-PUT objects keep the envelope-header path. Local-file mode accepts a JSON sidecar alongside the headerless ciphertext (`-sidecar`) plus the object IV (`-iv`). Covered by round-trip and corruption tests against a real headerless + sidecar fixture. The placeholder whole-object SHA (ADR-003 gap bf-1v2ehf) means multipart objects still have no header SHA to verify — per-block HMAC verification is the integrity guarantee.

## Addendum: Per-Cluster Deployment Form (2026-09-25)

Decision 4 specifies "a long-running Deployment … one instance covering all
ARMOR buckets". The long-running-Deployment half (internal scheduling loop, no
CronJob, declarative-config) shipped exactly as written; the "one instance"
half did not. What shipped instead is **one restore-verifier Deployment per
bucket scope, deployed per cluster** — as of this addendum, four:
`iad-ci/armor`, `iad-kalshi/armor`, `ord-devimprint/devimprint`, and
`rs-manager/armor` (`restore-verifier-acb`, which verifies apexalgo-iad's
ai-code-battle bucket from rs-manager while that cluster's ArgoCD connection
is broken — still one bucket, one MEK). The manifests live at
`declarative-config/k8s/<cluster>/<namespace>/restore-verifier*.y*ml`;
`scripts/find-armor-deployments.py` enumerates the live inventory and the
[restore-verifier deployment guide](../restore-verifier-deployment-guide.md)
is the operational reference. Each Deployment carries exactly one
`ARMOR_BUCKET` (`ARMOR_BUCKET_ALIASES` are alternate names for the same
bucket, not additional buckets) and one `ARMOR_MEK` — plus retired ring keys
where rotation has run — referenced from that scope's own secret store via
ExternalSecret.

Why the single fleet-wide instance lost out: the verifier decrypts, so every
input it needs is scoped the way the data it verifies is scoped, and no
credential set legitimately spans scopes.

- **MEK scoping.** Each ARMOR instance encrypts with its own MEK, delivered
  as that cluster's Secret from that cluster's OpenBao prefix. One central
  verifier would need every cluster's MEK escrowed into a single namespace —
  the one credential that decrypts everything, concentrated in one Deployment,
  crossing the per-cluster secret boundary the rest of the fleet maintains.
- **B2 credential scoping.** B2 application keys are bucket-scoped, so a
  fleet-wide verifier cannot even list, let alone fetch, every bucket. The
  `acb` stand-in is the exception that proves the shape: it exists because a
  bucket's verifier must run where that bucket's secret reference can sync,
  and it is itself single-bucket, single-MEK.
- **Read-path scoping.** The ARMOR read path goes through each deployment's
  own Cloudflare-fronted domain (`ARMOR_CF_DOMAIN`); there is no shared
  endpoint that exercises every consumer experience.
- **Failure isolation.** Per-scope deployments keep a verifier outage, a
  freshness gap, or an escalation-filing failure contained to the bucket it
  verifies; fleet scope would couple them.

Decision 4's coverage intent is unchanged: every ARMOR bucket is proven
restorable by a continuously-running dual-path verifier. "One instance" holds
at fleet scope only as the union of the per-scope instances.

## Fleet topology: scope, discovery, secrets, metrics, alerting (2026-09-25)

The Addendum fixes the deployment *form* and why it won. This section records
the operating contract for the fleet that form produces (armor-79255e46), so
the runbooks and the deployment guide have one place to point at.

**Scope.** One Deployment verifies exactly one bucket scope (bucket + MEK +
B2 credential set); a Deployment never straddles scopes. Fleet coverage is
the union of the Deployments, and it is a per-scope deployment decision
recorded in declarative-config — an ARMOR proxy Deployment does not by itself
imply a verifier, and several proxies have none today.

**Inventory.** Four restore-verifier Deployments make up the fleet as of
2026-09-25:

| Deployment | Cluster/namespace | Scope | Notes |
|---|---|---|---|
| `restore-verifier` | `iad-ci/armor` | bucket `iad-ci` | escalation bead-filing and alert evaluation live here |
| `restore-verifier` | `iad-kalshi/armor` | bucket `kalshi-tape` | |
| `restore-verifier` | `ord-devimprint/devimprint` | devimprint bucket (key `bucket` in `armor-credentials`), `ARMOR_PREFIX=commitgraph/` | |
| `restore-verifier-acb` | `rs-manager/armor` | bucket `armor-apexalgo` | stand-in for apexalgo-iad while that cluster's ArgoCD sync is broken; direct-to-B2, no co-located proxy for this bucket |

**The authoritative enumeration is mechanical, not this table:**
`python3 scripts/find-armor-deployments.py ~/declarative-config`, filtered to
`image_type == armor-restore-verifier`.
`tests/test_restore_verifier_inventory.py` pins that inventory (golden
snapshot 2026-09-25) and fails when this ADR, the
[deployment guide](../restore-verifier-deployment-guide.md), the
[alerting runbook](../runbooks/restore-verifier-alerting.md), or plan.md's
bump list drifts from it. When the fleet changes — scope added, retired, or
re-homed — update the test's golden inventory and every prose count in the
same change.

**Discovery.** Env-driven per
[ADR-014](014-restore-verifier-discovery-reliability.md): `ARMOR_BUCKET`
always, `ARMOR_PREFIX` only where the Deployment itself sets it —
ord-devimprint does today. Co-located verifiers read the proxy's env sources
key-by-key, but the prefix is not inherited implicitly: iad-kalshi's proxy
sets `ARMOR_PREFIX=iad-kalshi/` while its verifier sets no prefix env, so
prefixed objects are invisible to that Deployment (a verifier for a
prefix-namespaced bucket must set `ARMOR_PREFIX` in its own env; the false
"no objects found" readings this produced are the discovery gap ADR-014
records).

**Secrets.** Every verifier takes its scope's values by reference
(`secretKeyRef`/`configMapKeyRef`, never literals) from that scope's own
store, synced by ExternalSecret. Three patterns in the fleet today:
co-located verifiers reuse the proxy's `armor-config` ConfigMap and
`armor-secrets` Secret (iad-ci, iad-kalshi); ord-devimprint keeps every
B2/MEK value in `armor-credentials`; the standalone `restore-verifier-acb`
has a dedicated ExternalSecret
(`restore-verifier-acb-b2-credentials`, backed by OpenBao
`rs-manager/iad-acb/armor`). Escalation state needs a PVC (`sata`) at
`/var/lib/restore-verifier` where `VERIFIER_ESCALATION=true` — iad-ci only,
today.

**Metrics.** Every Deployment serves `/metrics` on a `:9002` Service.
Collection and rule evaluation are estate-local and currently iad-ci-only
(VictoriaMetrics + vmalert, activated 2026-09-25); the per-cluster
`restore-verifier-monitoring.yaml.disabled` manifests stay `.disabled`
elsewhere (no Prometheus Operator CRDs — rs-manager has none at all), so the
other three Deployments' gauges are exposed but uncollected.

**Alert routing.** iad-ci: vmalert → Alertmanager → the ntfy webhook — the
pipeline and responses are the
[restore-verifier alerting runbook](../runbooks/restore-verifier-alerting.md).
Bead-filing escalation (Decision 5) is likewise a per-Deployment feature,
enabled only on iad-ci; day-to-day fleet status stays in the
[deployment guide](../restore-verifier-deployment-guide.md).

**Where the stale counts came from.** The original deployment (bf-1pphhz)
targeted six scopes — `iad-acb` (bucket `armor-apexalgo`), `iad-ci`,
`iad-kalshi`, `ord-devimprint`, `rs-manager`, `iad-native-ads` — which is
where the bead-era "six" comes from; plan.md's bump list later said "five
(rs-manager ×2 incl. acb)". The live four are what survives: `iad-native-ads`
went with its cluster (decommissioned 2026-07-27, declarative-config
`45ae70b5`), the apexalgo-iad AI-battle estate retired 2026-08-22
(`af2a78a5`) leaving bucket `armor-apexalgo` to be verified cross-cluster by
`restore-verifier-acb`, and the plain `rs-manager/armor` verifier was removed
2026-09-23 (`abe7dd0c`) because everything in its bucket's listing belonged
to a foreign scope (armor-0f9efb09 — it could only ever fail on foreign
MEKs).
