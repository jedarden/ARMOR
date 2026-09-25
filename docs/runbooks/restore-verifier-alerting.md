# Restore-Verifier Alerting Runbook (iad-ci)

Operator guide to the live ARMOR alerting pipeline: what is deployed, how to
verify it, and how to respond when it pages. The alert rule semantics —
expressions, hold durations, thresholds, and per-alert first response — are
the contract in
[observability-contract.md](../observability-contract.md) ("Alert rules");
this runbook covers the pipeline around them and the iad-ci-specific
operational facts that nowhere else records.

Status of the fleet: **iad-ci is the only cluster where the rules are
evaluated** (activated 2026-09-25). The per-cluster
`restore-verifier-monitoring.yaml.disabled` PrometheusRule/ServiceMonitor
manifests stay `.disabled` everywhere — no ARMOR cluster runs the Prometheus
Operator CRDs — and the other three verifier deployments (ord-devimprint,
iad-kalshi, rs-manager) have no evaluator at all. `docs/plan/plan.md`
(Phase 6 and the §8 estate questions) is the authoritative activation
history; ADR-004's status paragraph still describes the pre-activation state.

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
- **The `.disabled` suffix stays on iad-ci.** Activation there did not drop
  the suffix (there are no Prometheus Operator CRDs to accept the objects);
  it added the vmalert evaluator instead. The observability-contract line
  about dropping the suffix applies to a future kube-prometheus-stack
  cluster, not to this one.
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
   deployment-level feature that is currently off everywhere — see ADR-004's
   "Enabling escalation" recipe; until it is provisioned, the alert is the
   only paging signal and the human files the bead.
5. If a page seems wrong, run `scripts/alerting-smoke-test.sh` first: it
   separates "pipeline broken" from "reality is bad" in one pass.

## 6. Activating another cluster

Not a copy-paste of iad-ci: the other verifier deployments have no metrics
store at all today, and the estate decision (VictoriaMetrics+vmalert
everywhere vs. kube-prometheus-stack where CRDs would land) is tracked as an
open question in `docs/plan/plan.md` §8. The reusable iad-ci pieces are the
scrape job shape, the vmalert Deployment, and the alertmanager delivery
config; the rules always come from the contract-pinned fixture (§3).
