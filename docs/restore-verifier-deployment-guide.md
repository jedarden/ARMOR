# Restore-Verifier Deployment Guide

## Overview

The restore-verifier is a standalone service for continuous backup verification that runs dual-path verification (ARMOR read path + armor decrypt direct) to prove that backups are restorable through both the normal server path and disaster recovery.

## Deployment Configuration

### Standard Deployment

For standard deployments where all objects are encrypted with a single MEK:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: restore-verifier-config
data:
  ARMOR_B2_REGION: "us-west-004"
  ARMOR_B2_ENDPOINT: "https://s3.us-west-004.backblazeb2.com"
  ARMOR_BUCKET: "my-bucket"
  ARMOR_MEK: "<64-char-hex-mek>"  # Active MEK only
  VERIFIER_CHECK_INTERVAL: "6h"
  VERIFIER_SAMPLE_SIZE: "10"
  VERIFIER_HTTP_LISTEN: ":9002"
```

### Key Rotation Deployment (MEK Ring)

During key rotation (Plan §8.13), the restore-verifier requires access to both the active MEK and any retired MEKs that may still have objects encrypted with them. This ensures the verifier can successfully decrypt and verify all objects in the bucket, regardless of which key was used to encrypt them.

#### Configuration with MEK Ring

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: restore-verifier-config
data:
  ARMOR_B2_REGION: "us-west-004"
  ARMOR_B2_ENDPOINT: "https://s3.us-west-004.backblazeb2.com"
  ARMOR_BUCKET: "my-bucket"
  ARMOR_MEK: "<64-char-hex-active-mek>"          # Active MEK (current)
  VERIFIER_MEK_RING: "<old-key-hex>,<another-old-key-hex>"  # Retired MEKs (comma-separated)
  VERIFIER_CHECK_INTERVAL: "6h"
  VERIFIER_SAMPLE_SIZE: "10"
  VERIFIER_HTTP_LISTEN: ":9002"
```

#### Key Rotation Procedure

1. **Before Rotation**: Deploy restore-verifier with only the active MEK
   ```yaml
   ARMOR_MEK: "<current-mek>"
   VERIFIER_MEK_RING: ""  # Empty - no retired keys
   ```

2. **During Rotation**: Add retired keys to the ring
   ```yaml
   ARMOR_MEK: "<new-active-mek>"
   VERIFIER_MEK_RING: "<old-mek-1>,<old-mek-2>"  # All retired keys
   ```

3. **After Rotation Complete**: Remove retired keys from the ring
   ```yaml
   ARMOR_MEK: "<current-mek>"
   VERIFIER_MEK_RING: ""  # Empty when no objects remain on old keys
   ```

#### Verification Strategy

The restore-verifier uses fingerprint-based key selection (Plan §8.13):
- **V2 format objects** (wrapped DEK includes fingerprint): Direct key selection by fingerprint from active key or ring
- **Legacy format objects** (no fingerprint): Trial unwrapping with active key, then each ring key in order

This means:
- Objects encrypted with the active key always work
- Objects encrypted with retired keys in the ring always work
- Objects encrypted with unknown keys fail with `ErrFingerprintNotFound`

## Environment Variables

### Required
- `ARMOR_B2_REGION`: B2 region (e.g., `us-west-004`)
- `ARMOR_B2_ENDPOINT`: B2 S3 API endpoint
- `ARMOR_B2_ACCESS_KEY_ID`: B2 application key ID
- `ARMOR_B2_SECRET_ACCESS_KEY`: B2 application key
- `ARMOR_MEK`: Master encryption key (hex, 64 chars)

### Optional
- `VERIFIER_MEK_RING`: Comma-separated list of retired MEKs (hex, 64 chars each) for key rotation support
- `VERIFIER_CHECK_INTERVAL`: Verification check interval (default: `6h`)
- `VERIFIER_SAMPLE_SIZE`: Historical sample size (default: `10`)
- `VERIFIER_HTTP_LISTEN`: HTTP listen address (default: `:9002`)
- `VERIFIER_DR_DRILL_INTERVAL`: Direct-only DR drill interval (default: disabled). See [Scheduled DR drills](#scheduled-dr-drills-production-cadence) for the production cadence, scheduling semantics, and the safe-pause procedure.
- `VERIFIER_RUN_TIMEOUT`: Per-run deadline for verification and DR-drill runs (default: `2h`). A run that exceeds it — a discovery walk wedged in a slow bucket region, or a stalled restore — fails visibly (failed-enumeration ledger and gauges, a log line naming the deadline) instead of silently blocking the loop. Discovery also logs progress roughly every 30s, so a long enumeration is observable rather than presenting as a hung verifier.
- `VERIFIER_ESCALATION`: `true` enables failure/staleness bead filing (default: disabled). Requires the in-image bead CLI and a writable volume at `/var/lib/restore-verifier` — see [Failure/staleness escalation](#failurestaleness-escalation-bead-filing) for the prerequisites, startup validation, and enablement recipe.
- `VERIFIER_FRESHNESS_WINDOW`: Staleness escalation window (default: `24h`) — at most one staleness bead per bucket per window.
- `ARMOR_DEPLOYMENT`: Deployment identity recorded in escalation bead bodies (e.g. `iad-ci/armor`).

## HTTP Endpoints

- `GET /status`: Verification status for all buckets
- `GET /bucket?bucket=X`: Status for specific bucket
- `POST /trigger`: Trigger immediate verification run (dual path)
- `POST /trigger?mode=dr-drill`: Trigger direct-only DR drill
- `GET /healthz`: Liveness check
- `GET /readyz`: Readiness check
- `GET /metrics`: Prometheus metrics

## Scheduled DR drills (production cadence)

`VERIFIER_DR_DRILL_INTERVAL` > 0 starts a second scheduler leg: the direct-only
DR drill (`ModeDRDrill` — MEK unwrap → raw B2 fetch → ADR-003-aware decrypt →
checksum → artifact assertion, the "ARMOR server is gone" recovery from
[ADR-004](adr/004-continuous-restore-verification.md)) runs for every
configured bucket on its own ticker, independent of `VERIFIER_CHECK_INTERVAL`.

**Production cadence:** all four restore-verifier Deployments set
`VERIFIER_DR_DRILL_INTERVAL: "24h"` (declarative-config commit `1550e3e8`,
2026-08-28): `iad-ci/armor`, `iad-kalshi/armor`,
`ord-devimprint/devimprint`, and `rs-manager/armor` (`restore-verifier-acb`).
Live pods confirm scheduled drills execute and report — e.g. iad-ci
2026-09-24 and rs-manager 2026-09-25 both logged a full drill with every
sampled object recovered direct-only.

### Scheduling semantics

- **First drill one full interval after pod start**, deliberately not chained
  behind the startup dual-path run (armor-851dca86). The schedule is
  interval-from-start, not wall-clock-anchored: a pod restart resets it, and a
  crash-looping pod never accumulates backlog.
- **At most one run at a time.** Dual runs and drills share one scheduler
  loop; a tick that lands while a run executes is dropped, so runs never chain
  back-to-back — the next run starts on the following tick.
- **A drill reuses the most recent dual-path enumeration** instead of
  re-walking the bucket (on a cold start it enumerates once itself).
- **Each drill run is bounded by `VERIFIER_RUN_TIMEOUT`** like a dual run. A
  run that hits the deadline records a failed enumeration: the drill-restore-age
  gauge advances and the drill failure counter increments, so the failure is
  visible rather than silent.

### How results are reported

- `GET /status` and `GET /bucket?bucket=X` carry the `drill_*` state fields
  (`drill_last_verification`, `drill_last_success`, `drill_total_objects`,
  `drill_verified_objects`, `drill_failed_objects`) — deliberately separate
  from the dual-path fields: a drill that succeeds while the ARMOR read path is
  down records progress without claiming dual-path health, and a dual run never
  advances the `drill_*` fields.
- Prometheus gauges, per bucket: `armor_drill_last_verified_timestamp`,
  `armor_drill_last_success_timestamp`, `armor_drill_verified_object_ratio`,
  `armor_drill_failures_total` (full series contract:
  [docs/observability-contract.md](observability-contract.md)).
- Log lines name each phase: `Starting DR-drill (direct-only) verification
  run`, `DR-drilling bucket (direct-only): <bucket>`,
  `Bucket <bucket> DR-drill complete: N/N recovered direct-only`.
- Escalation stays dual-path-owned: a drill failure never files a bead — the
  next dual run re-finds the failure and files there. Drill failures surface
  through the gauges and logs only.
- Scheduler-level proof: `TestStartScheduledDrillExecutesAndReports` (a
  started verifier must drill on its own ticker, recover direct-only, publish
  the gauges, and leave the dual-path ledger untouched),
  `TestStartWithoutDrillIntervalLeavesDrillPaused` (unset interval = paused),
  and `TestStartScheduledDrillFailureSignalsWithoutRetryStorm` (a damaged
  ciphertext must fail subsequent drills into exactly these signals — stale
  `drill_last_success`, advancing failure counter, zero ratio — with no bead
  filed, no ARMOR-read fallback, and a run rate still bounded by one per tick)
  in `internal/restoreverifier/drill_schedule_test.go`.

### Pausing for maintenance

Drills are read-only (B2 range reads plus local decryption — they never write
to the bucket), so pausing is about read traffic and honest signals, never
data safety.

**To pause:** set `VERIFIER_DR_DRILL_INTERVAL` to `"0"` (or remove the env
entry) in the deployment's manifest in `declarative-config` and let ArgoCD
sync — the env change rolls the pod. Do not `kubectl patch`. Paused means:

- no scheduled drill fires (pinned by
  `TestStartWithoutDrillIntervalLeavesDrillPaused`);
- the scheduler loop and dual-path verification continue unchanged;
- nothing queues up — no drill debt accumulates; the first drill after
  resuming is simply one interval after the new value takes effect;
- on-demand drills still work: `POST /trigger?mode=dr-drill` is unaffected,
  so a post-maintenance recovery proof is always available.

**When to pause:** B2 maintenance that could fail raw object reads (application-key
rotation, lifecycle changes) so the drill failure counters record real
corruption rather than an environmental outage, and capacity-sensitive windows
(a drill downloads its full sample through the direct path). For a B2-only
window, leave the dual path running — its ARMOR read path goes through
Cloudflare and may still be healthy; if B2 is fully down both paths fail,
which is the honest signal.

## Failure/staleness escalation (bead filing)

With `VERIFIER_ESCALATION=true` the verifier files exactly one bead per
distinct active verification failure and one staleness bead per bucket per
freshness window ([ADR-004 §5](adr/004-continuous-restore-verification.md),
[observability contract](observability-contract.md)). Filing is storm-proof
at two layers: a persisted dedupe set (`VERIFIER_ESCALATION_STATE`) and a
`--unique-ref` the bead store binds atomically, so even a lost state file
cannot produce a duplicate bead.

### Prerequisites (validated at startup)

1. **The canonical bead-rs CLI in the image.** The `restore-verifier-runtime`
   image stage ships `bead` at `/usr/local/bin/bead` (pinned release binary,
   checksum-verified at build time). The image base is `debian:bookworm-slim`
   for this reason — the bead release binary is glibc-dynamic and cannot run
   on the old `scratch` stage.
2. **A writable volume mounted at `/var/lib/restore-verifier`** holding both
   the dedupe state file and the beads workspace
   (`<state dir>/beads-workspace` by default). Persist it with a PVC so the
   dedupe set and the filed beads survive pod restarts; use storage class
   `sata` on Rackspace/OpenStack clusters.

At startup with escalation enabled the verifier validates both: it looks up
the CLI, runs `bead --version`, probes the workspace directory writable,
provisions a fresh workspace with `bead init` when missing, and opens it
with `bead list`. A failed validation logs `ESCALATION FILING DISABLED —`
with the exact remediation and the verifier keeps verifying (a crash-looping
verifier proves nothing); it files nothing until the prerequisite is fixed.

### Enabling on a deployment

```yaml
env:
  - name: VERIFIER_ESCALATION
    value: "true"
  - name: ARMOR_DEPLOYMENT
    value: "iad-ci/armor"        # recorded in every bead body
volumes:
  - name: escalation
    persistentVolumeClaim:
      claimName: restore-verifier-escalation
volumeMounts:
  - name: escalation
    mountPath: /var/lib/restore-verifier
```

Optional knobs (all env-driven): `VERIFIER_ESCALATION_WORKSPACE` (default
`/var/lib/restore-verifier/beads-workspace`, created and initialized at
startup), `VERIFIER_ESCALATION_BEAD_BINARY` (default `bead`),
`VERIFIER_ESCALATION_UNIQUE_REF_NAMESPACE` (default `restore-verifier`),
`VERIFIER_ESCALATION_LABEL`, `VERIFIER_FRESHNESS_WINDOW` (staleness window,
default `24h`), `VERIFIER_ESCALATION_STATE`, `VERIFIER_ESCALATION_EXEC_TIMEOUT`
(default `10s`).

**Fleet status:** enabled on `iad-ci/armor` (2026-09-25, armor-babc0b2b).
The other three restore-verifier Deployments keep escalation off until each
gets the volume + env; filing there is inert (`VERIFIER_ESCALATION` unset).

### Reading the escalation workspace

Filed beads land in the pod's workspace SQLite store (with the checkpoint
published alongside it under `beads-workspace/.beads/checkpoint/`). To
triage from the pod, run the CLI from the workspace directory — bead-rs
discovers the workspace by walking up from the current directory:

```bash
kubectl exec -n armor deploy/restore-verifier -- \
  sh -c 'cd /var/lib/restore-verifier/beads-workspace && bead list --limit 20'
```

## Metrics

The restore-verifier exposes Prometheus metrics:

- `armor_restore_verification_status_total`: Total verification runs by status
- `armor_restore_verification_duration_seconds`: Verification run duration
- `armor_restore_verification_failures_total`: Total verification failures
- `armor_restore_path_comparison_total`: Dual-path comparison results
- `armor_drill_last_verified_timestamp`: Last direct-only DR-drill attempt, per bucket
- `armor_drill_last_success_timestamp`: Last drill in which direct-only recovery was proven (0 = never), per bucket
- `armor_drill_verified_object_ratio`: Latest drill run's recovered/total ratio, per bucket
- `armor_drill_failures_total`: Cumulative drill failure count, per bucket

The per-bucket restorability gauges backing the ADR-004 alerts
(`armor_last_verified_restore_timestamp`, `armor_verified_object_ratio`,
`armor_restore_verification_failures_total`) and the full series contract are
pinned in [docs/observability-contract.md](observability-contract.md).

## Operational Notes

### Key Rotation Transition

When rotating MEKs (Plan §8.13):

1. **Add new key to OpenBao** as the active key
2. **Update restore-verifier ConfigMap** to include the old key in `VERIFIER_MEK_RING`
3. **Rolling restart** the restore-verifier deployment
4. **Monitor metrics** - verification failures should not increase
5. **Run POST /admin/key/rotate** on ARMOR server to re-wrap objects
6. **Remove old key from `VERIFIER_MEK_RING`** once all objects are re-wrapped
7. **Final rolling restart** to clean up the ring

### Verification During Rotation

- **Before rotation**: All objects encrypted with current key → 100% success
- **During rotation**: Mixed objects (current + old keys) → 100% success with ring configured
- **After rotation**: All objects encrypted with new key → 100% success

### Troubleshooting

**Symptom**: `restore-verifier` reports checksum conflicts on all objects

**Cause**: MEK ring not configured during key rotation

**Solution**: Add retired MEKs to `VERIFIER_MEK_RING` environment variable

**Symptom**: `ErrFingerprintNotFound` in logs

**Cause**: Object encrypted with unknown MEK not in active key or ring

**Solution**: Verify the retired MEK is included in `VERIFIER_MEK_RING` and the fingerprint matches

## Image

CI publishes the restore-verifier image as
`ronaldraygun/armor-restore-verifier:<version>` on Docker Hub, **private**
to the ardenone fleet (pulled with the `docker-hub-registry` pull secret).
There is no public mirror: by operator decision (2026-09-19) the ARMOR
server image is the only public artifact. Only released semver tags exist;
there is no floating tag.

Running the verifier outside the fleet means building it from source at the
release tag; it is a named stage of the repository's Dockerfile:

```bash
git clone --branch v<version> https://github.com/jedarden/ARMOR.git && cd ARMOR
docker build --target restore-verifier-runtime --build-arg VERSION=<version> \
  -t armor-restore-verifier:<version> .
```

## Deployment Example

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: restore-verifier
  labels:
    app: restore-verifier
spec:
  replicas: 1
  selector:
    matchLabels:
      app: restore-verifier
  template:
    metadata:
      labels:
        app: restore-verifier
    spec:
      containers:
      - name: restore-verifier
        image: ronaldraygun/armor-restore-verifier:<released-version>   # private; needs the pull secret, or use the locally built tag
        envFrom:
        - configMapRef:
            name: restore-verifier-config
        - secretRef:
            name: armor-b2-credentials
        ports:
        - containerPort: 9002
          name: http
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
        readinessProbe:
          httpGet:
            path: /readyz
            port: http
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

## References

- Plan §8.13: MEK key ring — multiple concurrent MEKs
- ADR-004: Continuous restore verification
- ADR-009: Restore-verifier ARMOR path never decrypts
- ADR-014: Restore-verifier discovery reliability
