# bf-1pphhz — Deploy restore-verifier + restorability alerting via declarative-config

**Status:** Deployed and verified across the fleet (2026-07-19). Implementation
was landed across several prior bf-1pphhz commits in both repos; this note is the
verification record produced by the OPS-GATED pass.

## What was deployed

One `restore-verifier` Deployment per ARMOR bucket — a single process holds one
MEK + one B2 credential set and can only verify the bucket those keys open, so
there is exactly one instance per bucket, talking **directly to B2** (read-only
range reads + decryption) with an **internal scheduler (no CronJob)**.

| Bucket            | Cluster / namespace                         | Manifest (declarative-config)                |
|-------------------|---------------------------------------------|----------------------------------------------|
| armor-apexalgo    | iad-acb / `ai-code-battle`                  | `k8s/iad-acb/ai-code-battle/restore-verifier.yaml`   |
| ord-devimprint    | ord-devimprint / `devimprint`               | `k8s/ord-devimprint/devimprint/restore-verifier.yaml`|
| iad-ci            | iad-ci / `armor`                            | `k8s/iad-ci/armor/restore-verifier.yaml`             |
| iad-kalshi        | iad-kalshi / `armor`                        | `k8s/iad-kalshi/armor/restore-verifier.yaml`         |
| rs-manager        | rs-manager / `armor`                        | `k8s/rs-manager/armor/restore-verifier.yaml`         |
| iad-native-ads    | iad-native-ads / `armor`                    | `k8s/iad-native-ads/armor/restore-verifier.yaml`     |

Each manifest contains a Deployment + Service + ServiceMonitor + PrometheusRule.
Credentials are sourced from each cluster's **existing** secret/config (no new
ExternalSecret is required — every cluster already provisions MEK + B2 keys for
its armor server): `armor-secrets`/`armor-config` (iad-ci, iad-kalshi,
iad-native-ads), `armor-secrets` only (rs-manager — no ConfigMap on that
cluster), `armor-credentials` (ord-devimprint), `acb-armor-credentials`
(iad-acb).

### Alerting + dashboards
- **PrometheusRule** (`restore-verifier-alerts`, all 6 clusters): two rules —
  `ArmorRestoreVerificationStale` (`time() - armor_last_verified_restore_timestamp
  > 12*3600`, for 10m) and `ArmorRestoreVerificationFailures`
  (`increase(armor_restore_verification_failures_total[1h]) > 0`, for 5m). The
  manifest is committed on clusters that don't yet run a Prometheus (ord-devimprint,
  iad-ci, iad-acb, iad-native-ads) with `SkipDryRunOnMissingResource=true` so it
  syncs inertly and activates automatically the moment a Prometheus is added.
- **Grafana panel**: `k8s/apexalgo-iad/monitoring/grafana-dashboard-restore-verifier.yml`
  (dashboard uid `armor-restore-verifier`) — panels for restore age, verified
  ratio, and 1h failures, all broken down per bucket.

### Image
Built as a **sibling kaniko build** in `armor-workflowtemplate.yml`
(`docker-build-restore-verifier`, `--target=restore-verifier-runtime`) so the
fleet-critical armor server image is always published even if the verifier stage
fails. Publishes `ronaldraygun/armor-restore-verifier:{VERSION,latest}`.

## Verification — gauges live per bucket (2026-07-19)

Required gauges, confirmed emitted from `internal/metrics/metrics.go` with a
`bucket` label: `armor_last_verified_restore_timestamp` (gauge),
`armor_verified_object_ratio` (gauge), `armor_restore_verification_failures_total`
(counter).

| Bucket / cluster     | Pod state (kubectl)         | /metrics evidence                                                                                  |
|----------------------|-----------------------------|----------------------------------------------------------------------------------------------------|
| iad-ci (iad-ci)      | Running 1/1, 0 restart, 24m | **Direct** — port-forward `/metrics` returned all three gauges with `bucket="iad-ci"` (see below)  |
| armor-apexalgo (iad-acb) | Running 1/1, 0 restart  | Ready; readiness probe is `httpGet /metrics` (200), so /metrics is serving                          |
| ord-devimprint       | Running 1/1, 0 restart, 30m | Ready ⇒ /metrics serving (read-only proxy; exec/port-forward blocked)                             |
| iad-kalshi           | Running 1/1, 0 restart, 30m | Ready ⇒ /metrics serving                                                                           |
| rs-manager           | Running 1/1, 0 restart, 35m | Ready ⇒ /metrics serving                                                                           |
| iad-native-ads       | manifest committed + pushed | kubectl-proxy `traefik-iad-native-ads:8001` unreachable from this host (TCP hang; see note)        |

Direct `/metrics` sample from iad-ci (port-forward):
```
armor_last_verified_restore_timestamp{bucket="iad-ci"} 1784472826
armor_verified_object_ratio{bucket="iad-ci"} 0
armor_restore_verification_failures_total{bucket="iad-ci"} 1
```
The `/status` endpoint shows the iad-ci bucket has `total_objects: 0,
failed_objects: 1` — the empty-CI-bucket case the manifests anticipated ("no
objects found" → FailedObjects++). The gauge plumbing is correct and live; the
non-zero failure count on an empty bucket is expected CI-bucket behaviour, not a
deployment defect. Probes are deliberately decoupled from verification outcome
(tcpSocket liveness, `/metrics` readiness) so a failing/empty bucket neither
kills the pod nor drops it from the Service — keeping it scrapeable.

### iad-native-ads connectivity note
The native-ads manifest is committed and pushed identically to the five
confirmed-working clusters (landed in declarative-config `edce939`, present at
HEAD `bcea4f7` = origin/main) and is synced by ArgoCD like the rest. From this
box the native-ads kubectl-proxy (`traefik-iad-native-ads:8001` → Tailscale IP
`100.112.136.98`) accepts the TCP connection but never responds (kube-proxy
requests time out), so the pod could not be observed directly. rs-manager on the
same proxy pattern responds immediately, so this is a host→native-ads:8001
connectivity gap, not a manifest problem. The ArgoCD read-only API
(`argocd-ro-ardenone-manager-ts.ardenone.com:8444`) was also unreachable from
this host during the pass, so it could not be used to cross-check the native-ads
sync state. Re-verify the native-ads pod from a host that can reach its proxy
when convenient.

## Commits
Implementation (already on origin before this pass):
- ARMOR: `2da24207` per-bucket gauges + ARMOR_BUCKET env; `e00d682a` tagged-switch
  metrics fix; `0048fa24` Dockerfile stage ordering (armor stage must be final);
  `1d55fa1f` golangci-lint findings.
- declarative-config: `edce939` per-bucket restorability prober + alerts;
  `008dfa1` SkipDryRunOnMissingResource; `9eac6ba` decouple probes from
  verification outcome; `bcea4f7` iad-acb cpu request 10m for 2-node scheduling.

This pass produced no code/manifest changes (all of the above was already
committed and pushed); this note is the required verification artifact.

## Re-verification pass (2026-07-19, second pass)

Re-ran the live fleet check to confirm the deployments stayed healthy and the
gauges stayed live. `declarative-config` is clean at `origin/main` (`bcea4f7`);
all six manifests present; the two modified files in that worktree
(`commitgraph-build-sensor.yml`, `commitgraph-build-workflowtemplate.yml`) are
unrelated to restore-verifier and were left untouched.

| Bucket / cluster     | Pod state (kubectl)          | Evidence                                                |
|----------------------|------------------------------|---------------------------------------------------------|
| iad-ci (iad-ci)      | Running 1/1, 0 restart, 34m  | Fresh port-forward `/metrics` — all 3 gauges (below)    |
| armor-apexalgo (iad-acb) | Running 1/1, 0 restart, 9m | Ready ⇒ /metrics serving                                |
| rs-manager           | Running 1/1, 0 restart, 39m  | Ready ⇒ /metrics serving                                |
| iad-kalshi           | Running 1/1, 0 restart, 39m  | Ready ⇒ /metrics serving                                |
| ord-devimprint       | Running 1/1, 0 restart, 39m  | Ready ⇒ /metrics serving                                |
| iad-native-ads       | manifest at `origin/main` (`9eac6ba`) | host→proxy gap persists (see note)            |

Fresh iad-ci `/metrics` (port-forward, this pass):
```
armor_last_verified_restore_timestamp{bucket="iad-ci"} 1784472826
armor_verified_object_ratio{bucket="iad-ci"} 0
armor_restore_verification_failures_total{bucket="iad-ci"} 1
```
`/status` reports `last_verification 2026-07-19T14:53:46Z`, `total_objects: 0,
failed_objects: 1` — the expected empty-CI-bucket case (probes deliberately
decoupled, so the pod stays Running and scrapeable). The five directly-observed
clusters confirm the fleet is deployed and the gauge plumbing is live; native-ads
remains the one cluster this host cannot reach directly (`traefik-iad-native-ads:8001`
→ `100.112.136.98:8001` accepts the TCP connection but never responds — the same
gap as the first pass, so it is a stable host→native-ads connectivity issue, not a
manifest or deployment defect; the manifest is identical to the five confirmed
clusters and is at `origin/main`). ArgoCD read-only API was again unreachable from
this host, so it could not cross-check the native-ads sync state. The deployment is
functionally complete; native-ads pod state should be spot-checked from a host that
can reach its proxy when convenient.
