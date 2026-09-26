# Restore-Verifier Alerting Runbook (iad-ci)

Operator guide to the live ARMOR alerting pipeline: what is deployed, how to
verify it, and how to respond when it pages. The alert rule semantics —
expressions, hold durations, thresholds, and per-alert first response — are
the contract in
[observability-contract.md](../observability-contract.md) ("Alert rules");
this runbook covers the pipeline around them and the iad-ci-specific
operational facts that nowhere else records.

Status of the fleet: **the pipeline is live on every ARMOR deployment**
(iad-ci first, 2026-09-25, armor-afe279bd; every other cluster replicated in
the same change wave, armor-cb731b20). The estate split, by what each cluster
already ran:

| Cluster | Form | Tailnet endpoints (smoke-test env) |
|---|---|---|
| iad-ci | dedicated store: VictoriaMetrics + vmalert + Alertmanager | `vmetrics-iad-ci-ts` / `vmalert-iad-ci-ts` / `alertmanager-iad-ci-ts` `.ardenone.com:8444` |
| rs-manager | dedicated store (iad-ci shape, armor job only) | `vmetrics-rs-manager-ts` / `vmalert-rs-manager-ts` / `alertmanager-rs-manager-ts` `.ardenone.com:8444` |
| iad-kalshi | dedicated store | `vmetrics-iad-kalshi` / `vmalert-iad-kalshi` / `alertmanager-iad-kalshi` `.tail1b1987.ts.net` (Tailscale-operator Services) |
| ord-devimprint | dedicated store | `vmetrics-ord-devimprint` / `vmalert-ord-devimprint` / `alertmanager-ord-devimprint` `.tail1b1987.ts.net` (Tailscale-operator Services) |
| apexalgo-iad | kube-prometheus-stack: ServiceMonitors + PrometheusRule, cluster Prometheus + Alertmanager | `prometheus-apexalgo-iad-ts` / `alertmanager-apexalgo-iad-ts` `.ardenone.com:8444` (`VM_BASE` and `VMALERT_BASE` are both the Prometheus) |
| ardenone-cluster | kube-prometheus-stack (no verifier — server canary only) | `prometheus-ardenone-cluster-ts` / `alertmanager-ardenone-cluster-ts` `.ardenone.com:8444` |

All alerting clusters page the one shared ntfy topic (the cnpg-backup-watchdog
channel); each page names its cluster via vmalert's `-external.label` or the
Prometheus `externalLabels` equivalent. The per-cluster
`restore-verifier-monitoring.yaml.disabled` PrometheusRule/ServiceMonitor
manifests stay `.disabled` everywhere — no ARMOR cluster runs the Prometheus
Operator CRDs those objects need; the two kube-prometheus-stack clusters
consume the live PrometheusRule form instead. The verifier fleet itself —
four restore-verifier Deployments today
(`iad-ci/armor`, `iad-kalshi/armor`, `ord-devimprint/devimprint`, and
`rs-manager/armor` running `restore-verifier-acb`) — is defined in ADR-004's
"Fleet topology" section; `scripts/find-armor-deployments.py` enumerates it
mechanically, and `tests/test_restore_verifier_inventory.py` fails if this
runbook's count drifts from the fleet.

## 1. The pipeline, end to end

```
ARMOR server            restore-verifier
  admin :9001/metrics     :9002/metrics
        |                       |
        +───── VictoriaMetrics (vmetrics-server, monitoring ns, 30s scrape
                of job `armor`) ─────+
                                     |
              vmalert (monitoring ns) — evaluates the restore-verifier rule
              group every 30s against the store, remote-writes ALERTS state
                                     |
              Alertmanager (monitoring ns) — groups, delivers to the ntfy
              webhook (the cnpg-backup-watchdog topic; URL + bearer token are
              rendered from OpenBao by an ExternalSecret, never in git)
```

All three surfaces are reachable only from the tailnet, via the Traefik vpn
entrypoint (`k8s/iad-ci/traefik/armor-alerting-vpn-ingressroute.yml` in
`declarative-config`):

| Surface | URL | What to look at |
|---|---|---|
| metrics store | `https://vmetrics-iad-ci-ts.ardenone.com:8444` | `/api/v1/query`, `/targets` |
| rule evaluator | `https://vmalert-iad-ci-ts.ardenone.com:8444` | `/api/v1/rules`, `/api/v1/alerts` |
| delivery | `https://alertmanager-iad-ci-ts.ardenone.com:8444` | `/api/v2/alerts`, `/-/ready` |

The verifier's own `/status` endpoint (first response for the restore alerts)
is **not** routed through Traefik — read it from inside the cluster
(`kubectl exec` is the usual path; the credential-free read-only proxy
forbids exec, so use the real kubeconfig) or read the same gauges from the
store.

Everything that owns a piece of this lives in `declarative-config`
(`k8s/iad-ci/monitoring/`: `victoriametrics-application.yml` — the store and
the `armor` scrape job; `vmalert.yml` — the rule ConfigMap + evaluator;
`alertmanager.yml` — delivery config), except the rules themselves, which
ARMOR owns — see §3.

## 2. Verify the pipeline

Two tools ship in this repo (see [scripts/README.md](../../scripts/README.md)
for full usage):

- **Live end-to-end:** `scripts/alerting-smoke-test.sh` — run from inside the
  tailnet, no arguments. Four phases, each mapping to one activation promise:
  collection (both armor targets up, numeric `armor_*` series fresh,
  cardinality guard), evaluation (all five shipped rules loaded in vmalert,
  expressions matching the contract, no eval errors, evaluations fresh),
  consistency (the state-driven alerts active exactly when their expression
  says so — both directions, no state forced), delivery (Alertmanager ready,
  ntfy receiver present in the *rendered* config — the config itself is never
  printed, it embeds the delivery token). Exit 0 is the pass condition.
- **Offline rule semantics:** `scripts/alerting-rule-unittest.sh` —
  `promtool test rules` in a pinned image against
  `scripts/alerting-rules-unittest.yaml`, proving the shipped expressions
  fire/quiet/resolves as contracted without waiting 12h or corrupting a real
  canary. Run this after any rule change (§3) and before any release that
  touches `internal/metrics`.

Run the smoke test after any change to the monitoring manifests, after a
cluster sync that touches `monitoring/`, and whenever a page seems implausible
(it distinguishes "alert firing because reality is bad" from "pipeline
broken" in one pass).

Per-cluster invocations (all from inside the tailnet; the shape knobs are
documented in the script header):

```bash
# iad-ci (defaults)
./scripts/alerting-smoke-test.sh

# rs-manager / iad-kalshi / ord-devimprint — dedicated-store replicas of the
# iad-ci shape; only the hostnames differ
VM_BASE=https://vmetrics-rs-manager-ts.ardenone.com:8444 \
VMALERT_BASE=https://vmalert-rs-manager-ts.ardenone.com:8444 \
AM_BASE=https://alertmanager-rs-manager-ts.ardenone.com:8444 \
  ./scripts/alerting-smoke-test.sh

# apexalgo-iad — kube-prometheus-stack: Prometheus is both store and rule
# evaluator; two armor targets, no verifier, canaries disabled at scrape
VM_BASE=https://prometheus-apexalgo-iad-ts.ardenone.com:8444 \
VMALERT_BASE=https://prometheus-apexalgo-iad-ts.ardenone.com:8444 \
AM_BASE=https://alertmanager-apexalgo-iad-ts.ardenone.com:8444 \
ARMOR_JOB_REGEX='native-ads-scan-armor|armor-ledger' \
ARMOR_EXPECT_SERVER_TARGETS=2 ARMOR_EXPECT_VERIFIER=0 ARMOR_EXPECT_CANARY=0 \
  ./scripts/alerting-smoke-test.sh

# ardenone-cluster — kube-prometheus-stack, one armor server, real canary,
# no verifier
VM_BASE=https://prometheus-ardenone-cluster-ts.ardenone.com:8444 \
VMALERT_BASE=https://prometheus-ardenone-cluster-ts.ardenone.com:8444 \
AM_BASE=https://alertmanager-ardenone-cluster-ts.ardenone.com:8444 \
ARMOR_EXPECT_VERIFIER=0 \
  ./scripts/alerting-smoke-test.sh
```

The iad-kalshi and ord-devimprint lines follow the rs-manager pattern with
their own `*.tail1b1987.ts.net` hostnames (table in the fleet-status section
above).

## 3. Changing a rule — three verbatim copies, one change

The rule group exists in three copies that must stay byte-identical:

1. `internal/metrics/testdata/restore-verifier-monitoring.yaml` — the
   in-repo fixture, pinned by `internal/metrics/alert_rules_contract_test.go`
   (which also pins the alert table in the observability contract);
2. the per-cluster `restore-verifier-monitoring.yaml.disabled` manifests in
   `declarative-config`;
3. the `vmalert-rules` ConfigMap inside
   `declarative-config/k8s/iad-ci/monitoring/vmalert.yml` (the only copy that
   evaluates today).

An expression, `for`, severity, or label change updates all three plus the
contract test's expected table and the observability-contract alert table in
the same change. `scripts/alerting-rule-unittest.sh` must pass before push.
vmalert re-reads rules on `-configCheckInterval=60s`; vmetrics re-reads its
scrape config on `-promscrape.configCheckInterval=1m` — both were set
deliberately, because the defaults ("read once, never again") turn a synced
config edit into a silently stale one.

## 4. iad-ci-specific facts that have already cost time

- **The verifier exposes the canary family but never runs a canary.** It
  links `internal/metrics` for its three restore gauges, so the whole
  `armor_*` family appears on `:9002` — including
  `armor_multipart_canary_healthy` at its before-first-check default `0`,
  permanently. The armor scrape job therefore **drops the canary families for
  the verifier target at scrape time** (`victoriametrics-application.yml`);
  without that drop, `ArmorMultipartCanaryUnhealthy` fires forever against
  the verifier's series and pages continuously (this happened on activation
  day, 2026-09-25). If a future verifier ever runs a canary of its own,
  remove that drop in the same change.
- **String-valued gauges are never stored.** `*_last_check_time` and
  `*_last_check_error` are RFC3339/error strings on the wire (contract
  caveat); a Prometheus-compatible store drops non-numeric samples at ingest,
  so these series are *expected* to be absent from vmetrics. The smoke test
  asserts the numeric canary counters instead. Nothing alert-side may treat
  the string gauges as samples.
- **The `.disabled` suffix stays everywhere.** No ARMOR cluster runs the
  Prometheus Operator CRDs the manifests' ServiceMonitor/PrometheusRule
  objects need — iad-ci and the three dedicated-store replicas evaluate via
  vmalert instead, and the two kube-prometheus-stack clusters (apexalgo-iad,
  ardenone-cluster) consume a live PrometheusRule carrying the same group.
  The observability-contract line about dropping the suffix applies to a
  future kube-prometheus-stack verifier cluster; the server-only clusters
  that have the stack today got the CRD form without touching these files.
- **vmalert's API reports `for` durations in seconds** (`600`), while the
  fixtures write `10m`. The smoke test normalizes; a hand-rolled comparison
  against the fixture strings will false-fail.
- **Alertmanager's rendered config is secret-bearing.** Assert on receiver
  names and `/api/v2` status, never fetch-and-print the config body (the
  smoke test's Phase 4 shows the pattern).

## 5. When a page arrives

1. Read the alert's `instance` label: `:9001` = the ARMOR server,
   `:9002` = the restore-verifier. The observability contract's alert table
   gives the expression and the intended first response per alert.
2. `ArmorRestoreVerification*` alerts: read the verifier's `/status`
   (recent results carry both-path evidence) and
   `armor_restore_verification_failures_total` /
   `armor_verified_object_ratio` from the store. A **zero-object discovery
   result is inconclusive** (ADR-014) — confirm the sample before treating a
   low ratio as corruption.
3. `ArmorMultipartCanaryUnhealthy`: read `/armor/canary` on the ARMOR server
   (`multipart_last_error`, `multipart_consecutive_fails`) — the gauge alone
   cannot tell you which stage failed.
4. Escalate per ADR-004 §5: one bead per distinct active failure, never a
   retry loop. The verifier's built-in escalation (bead filing) is a
   deployment-level feature — enabled on `iad-ci/armor` (2026-09-25,
   armor-babc0b2b), off on the other deployments (enablement recipe:
   [restore-verifier deployment guide, "Failure/staleness
   escalation"](../restore-verifier-deployment-guide.md)); where it is off,
   the alert is the only paging signal and the human files the bead.
5. If a page seems wrong, run `scripts/alerting-smoke-test.sh` first: it
   separates "pipeline broken" from "reality is bad" in one pass.

## 6. Activating another cluster

Done fleet-wide 2026-09-25 (armor-cb731b20); the estate question in
`docs/plan/plan.md` §8 (Phase 6 leftover) records the outcome. The form is
NOT a copy-paste of iad-ci — it follows what the cluster already runs:

- **Cluster has a kube-prometheus-stack** (apexalgo-iad,
  ardenone-cluster): no second store. A `ServiceMonitor` beside each armor
  Service (admin-api port, armor_* keep-list, canary families dropped at
  scrape where `ARMOR_CANARY_DISABLED=true` pins the gauge at 0 forever),
  the contract-pinned rule group as a `PrometheusRule` (with whatever rule
  selector labels that cluster's Prometheus CR actually selects on —
  ardenone requires the `release=` label), and an Alertmanager config
  ExternalSecret whose root route stays on the chart-default `'null'`
  receiver with a `component=~"restore-verifier|armor-canary"` sub-route to
  ntfy, so nothing else on the shared Alertmanager starts paging. Cluster
  attribution comes from `prometheusSpec.externalLabels`.
- **Cluster has no store** (rs-manager, iad-kalshi, ord-devimprint): the
  iad-ci shape — `victoriametrics-application.yml` (armor job + self-scout
  only, 5Gi sata, requests under the cluster's per-pod cap), `vmalert.yml`
  (rules verbatim from §3's fixture, `-external.label=cluster=<name>`),
  `alertmanager.yml` (ExternalSecret rendering the ntfy receiver from
  OpenBao). Tailnet read paths follow the cluster's existing mechanism
  (Traefik vpn entrypoint + tailnet DNS records, or Tailscale-operator
  companion Services).

Whatever the form, the invariants are the same: the rule group is byte-identical
to §3's fixture; the verifier target keeps exactly the restore trio (never-run
canary families are dropped at scrape); string-valued gauges are expected to be
absent from any store; the `.disabled` manifests stay `.disabled`; and the new
`monitoring/` manifests sync through the cluster's normal ArgoCD path — no
`kubectl apply` anywhere. A new ARMOR deployment on an already-activated
cluster needs only its scrape surface added (a static target in the armor job,
or a ServiceMonitor) — the evaluator and delivery are cluster-level and
already running. The fleet smoke-test invocations for the current clusters
are in §2.
