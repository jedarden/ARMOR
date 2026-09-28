#!/usr/bin/env bash
# End-to-end verification of the ARMOR alerting stack on iad-ci (ADR-002 /
# ADR-004 §6). Verifies the four things activation promises, against the LIVE
# stack, over its tailnet-only routes:
#
#   1. collection    — VictoriaMetrics is scraping both armor targets (the
#                      server's admin mux and the restore-verifier listener)
#                      and the canary + restore series are fresh.
#   2. evaluation    — vmalert has the five shipped rules loaded, expressions
#                      matching the shipped contract, evaluating without
#                      error on a recent tick.
#   3. consistency   — the two state-driven alerts (restore staleness,
#                      multipart health) are active exactly when their
#                      expression says they should be on the live data —
#                      both directions, without forcing any state.
#   4. delivery      — Alertmanager is ready, its rendered config carries the
#                      ntfy webhook receiver, and every currently pending or
#                      firing vmalert alert is visible in Alertmanager. The
#                      config and alert payloads are fetched but never
#                      printed: they can contain delivery credentials.
#
# Route prerequisites (k8s/iad-ci/traefik/armor-alerting-vpn-ingressroute.yml
# in jedarden/declarative-config): the three *-iad-ci-ts.ardenone.com:8444
# hostnames resolve only from inside the tailnet — this script is a no-go from
# outside. The cluster's hosted control plane cannot port-forward headless
# (interactive OIDC), so these routes are the only path in.
#
# Usage: scripts/alerting-smoke-test.sh
#   VM_BASE / VMALERT_BASE / AM_BASE override the three endpoints.
#   Shape knobs for the clusters activated since (bead armor-cb731b20) — all
#   default to the iad-ci shape:
#     ARMOR_JOB_REGEX           regex for the scrape job label(s) carrying the
#                               armor targets. Dedicated stores name the job
#                               `armor` (default); ServiceMonitor-derived
#                               targets carry the Service name(s), e.g.
#                               'native-ads-scan-armor|armor-ledger'.
#     ARMOR_EXPECT_SERVER_TARGETS  how many :9001 targets the job should have
#                               (default 1; 2 on apexalgo-iad).
#     ARMOR_EXPECT_VERIFIER     1 (default) requires one :9002 target and the
#                               three restore gauges fresh; 0 for clusters
#                               with no restore-verifier Deployment (the
#                               restore rules then legitimately have no
#                               series — Phase 3 reads that as quiet).
#     ARMOR_EXPECT_CANARY       1 (default) requires the numeric canary
#                               counters fresh; 0 where the canary is
#                               disabled by config and its families are
#                               dropped at scrape (apexalgo-iad).
#     ARMOR_EXPECT_CANARY_TIMESTAMP 0 (default) leaves the optional diagnostic
#                               unix-seconds timestamp unchecked. Set to 1
#                               after rolling an image that exports it; it is
#                               not an input to any shipped alert.
#     ALERTING_REQUIRE_ACTIVE   0 (default) accepts a quiet stack after the
#                               evaluator, receiver, and delivery API pass.
#                               Set to 1 in a controlled drill that stages a
#                               pending/firing alert; no alert is synthesized
#                               by this read-only smoke test.
#   On the kube-prometheus-stack clusters (apexalgo-iad, ardenone-cluster)
#   VM_BASE and VMALERT_BASE are BOTH the cluster's Prometheus (it serves the
#   same /api/v1/query, /api/v1/rules and /api/v1/alerts shapes this script
#   already speaks), and AM_BASE its Alertmanager.
# Exit: 0 all checks pass; 1 one or more failed; 2 environment problem.

set -uo pipefail

VM_BASE="${VM_BASE:-https://vmetrics-iad-ci-ts.ardenone.com:8444}"
VMALERT_BASE="${VMALERT_BASE:-https://vmalert-iad-ci-ts.ardenone.com:8444}"
AM_BASE="${AM_BASE:-https://alertmanager-iad-ci-ts.ardenone.com:8444}"
ARMOR_JOB_REGEX="${ARMOR_JOB_REGEX:-armor}"
ARMOR_EXPECT_SERVER_TARGETS="${ARMOR_EXPECT_SERVER_TARGETS:-1}"
ARMOR_EXPECT_VERIFIER="${ARMOR_EXPECT_VERIFIER:-1}"
ARMOR_EXPECT_CANARY="${ARMOR_EXPECT_CANARY:-1}"
ARMOR_EXPECT_CANARY_TIMESTAMP="${ARMOR_EXPECT_CANARY_TIMESTAMP:-0}"
ALERTING_REQUIRE_ACTIVE="${ALERTING_REQUIRE_ACTIVE:-0}"

for tool in curl python3; do
    command -v "$tool" >/dev/null 2>&1 || { echo "FATAL: $tool not on PATH" >&2; exit 2; }
done

# Freshness bound for "this is being collected": the SCRAPE is 30s, so a
# sample older than a few minutes means scraping stopped — not a quiet source
# (the underlying canary/verifier loops refresh on minute-scale, and the
# restore gauges carry their own 6h-scale staleness contract, which Phase 3
# reads rather than enforces). 10m leaves room for scrape gaps on a spot
# cluster.
MAX_SAMPLE_AGE_SECONDS=600
# vmalert evaluates the group every 30s; a stale evaluation means vmalert is
# down, the datasource is slow, or the -configCheckInterval trap fired.
MAX_RULE_EVAL_AGE_SECONDS="${MAX_RULE_EVAL_AGE_SECONDS:-300}"

PASS=0
FAIL=0

note() { printf '%s\n' "$*"; }
ok()   { note "PASS: $*"; PASS=$((PASS + 1)); }
bad()  { note "FAIL: $*"; FAIL=$((FAIL + 1)); }

# curl_json BASE PATH [curl args...] — GET, validate, print JSON. One fetch:
# validated and emitted from the same response.
curl_json() {
    local base="$1" path="$2"; shift 2
    local body
    body="$(curl -sS --max-time 15 -fsS "$base$path" "$@" 2>/dev/null)" || return 1
    printf '%s' "$body" | python3 -c 'import json,sys; json.load(sys.stdin)' >/dev/null 2>&1 || return 1
    printf '%s' "$body"
}

# vm_query EXPRESSION — prints the VM /api/v1/query result list (JSON array).
# An unreachable store or malformed answer degrades to an empty list: the
# caller's "series absent" diagnosis, which is what an unreachable endpoint
# means for collection purposes.
vm_query() {
    local out
    out="$(curl_json "$VM_BASE" /api/v1/query -G --data-urlencode "query=$1")" || { echo "[]"; return 0; }
    printf '%s' "$out" | python3 -c 'import json,sys; print(json.dumps(json.load(sys.stdin).get("data",{}).get("result",[])))' 2>/dev/null || echo "[]"
}

# value_of SELECTOR — freshest sample's value, or nothing when absent.
value_of() {
    vm_query "$1" | python3 -c '
import json,sys
r = json.load(sys.stdin)
print(r[0]["value"][1] if r else "")'
}

# epoch_of SELECTOR — freshest sample's unix timestamp, or nothing.
epoch_of() {
    vm_query "$1" | python3 -c '
import json,sys
r = json.load(sys.stdin)
print(int(float(r[0]["value"][0])) if r else "")'
}

note "== ARMOR alerting smoke test =="
note "store: $VM_BASE"
note "rules: $VMALERT_BASE"
note "delivery: $AM_BASE"
note ""

# ---------------------------------------------------------------- collection
note "-- Phase 1: collection (scrape of the armor targets)"

TARGETS_UP="$(vm_query "up{job=~\"^($ARMOR_JOB_REGEX)\$\"}" | python3 -c '
import json,sys
r = json.load(sys.stdin)
up = {s["metric"].get("instance","?"): s["value"][1] for s in r}
print(len([v for v in up.values() if v == "1"]), len(up))
for i, v in sorted(up.items()):
    print(i, v)' 2>/dev/null)"

EXPECTED_TOTAL=$((ARMOR_EXPECT_SERVER_TARGETS + ARMOR_EXPECT_VERIFIER))

if [ -z "$TARGETS_UP" ]; then
    bad "up{job=~\"^($ARMOR_JOB_REGEX)$\"} unreadable — store unreachable or the armor job absent"
else
    TOTAL="$(printf '%s\n' "$TARGETS_UP" | head -1)"
    SERVER_UP="$(printf '%s\n' "$TARGETS_UP" | awk -v n="$ARMOR_EXPECT_SERVER_TARGETS" '$1 ~ /:9001$/ {c++} END {print c+0}')"
    VERIFIER_UP="$(printf '%s\n' "$TARGETS_UP" | awk '$1 ~ /:9002$/ {c++} END {print c+0}')"
    if [ "$TOTAL" = "$EXPECTED_TOTAL $EXPECTED_TOTAL" ] \
       && [ "$SERVER_UP" -eq "$ARMOR_EXPECT_SERVER_TARGETS" ] \
       && [ "$VERIFIER_UP" -eq "$ARMOR_EXPECT_VERIFIER" ]; then
        ok "all $EXPECTED_TOTAL armor targets scraped, up=1 (server :9001 x$ARMOR_EXPECT_SERVER_TARGETS, verifier :9002 x$ARMOR_EXPECT_VERIFIER)"
    else
        bad "armor scrape targets wrong: total=[$TOTAL] want $EXPECTED_TOTAL up; server_9001=[$SERVER_UP] want $ARMOR_EXPECT_SERVER_TARGETS; verifier_9002=[$VERIFIER_UP] want $ARMOR_EXPECT_VERIFIER"
    fi
fi

check_fresh_series() {
    local series="$1" label="$2"
    local val epoch age
    val="$(value_of "$series")"
    epoch="$(epoch_of "$series")"
    if [ -z "$val" ] || [ -z "$epoch" ]; then
        bad "$label: series $series absent from the store"
        return
    fi
    age=$(( $(date +%s) - epoch ))
    if [ "$age" -gt "$MAX_SAMPLE_AGE_SECONDS" ]; then
        bad "$label: series $series is ${age}s stale (>${MAX_SAMPLE_AGE_SECONDS}s)"
    else
        ok "$label: $series fresh (${age}s old, value=$val)"
    fi
}

# Restore-verifier gauges (the ArmorRestoreVerification* alert inputs).
# Skipped on clusters with no restore-verifier Deployment: the gauges do not
# exist there, and their absence is the correct state — not a collection
# failure.
if [ "$ARMOR_EXPECT_VERIFIER" = "1" ]; then
    check_fresh_series 'armor_last_verified_restore_timestamp' 'restore-verifier'
    check_fresh_series 'armor_verified_object_ratio'           'restore-verifier'
    check_fresh_series 'armor_restore_verification_failures_total' 'restore-verifier'
fi
# Canary gauges from the ARMOR server (the ArmorMultipartCanaryUnhealthy input).
# The former *_last_check_time / *_last_check_error string gauges no longer
# exist on the wire: RFC3339/error-string samples are non-numeric and a
# Prometheus-compatible store drops them at ingest, so they were converted to
# numeric unix-seconds *_last_check_timestamp gauges (the error text stays on
# /armor/canary and the status surfaces). Asserting the timestamp gauge here
# proves the converted series actually lands in the store — the check the old
# string gauges could never pass. Skipped where the canary is disabled by
# config (ARMOR_CANARY_DISABLED=true) and the canary families are dropped at
# scrape — same reasoning: absence is correct there.
if [ "$ARMOR_EXPECT_CANARY" = "1" ]; then
    check_fresh_series 'armor_multipart_canary_healthy'         'armor canary'
    check_fresh_series 'armor_canary_checks_total'              'armor canary'
    check_fresh_series 'armor_multipart_canary_checks_total'    'armor canary'
    if [ "$ARMOR_EXPECT_CANARY_TIMESTAMP" = "1" ]; then
        check_fresh_series 'armor_multipart_canary_last_check_timestamp' 'armor canary diagnostic'
    fi
fi

# Cardinality guard: this job sits on a size-capped store (20Gi on iad-ci,
# 5Gi on the replicated stores, a 4GB retentionSize on the operator stacks).
# A series explosion here is the loud failure the armor_* keep-list is
# supposed to produce, not a slow disk death.
SERIES_COUNT="$(value_of "count({job=~\"^($ARMOR_JOB_REGEX)\$\"})" | cut -d. -f1)"
if [ -n "$SERIES_COUNT" ] && [ "$SERIES_COUNT" -lt 20000 ] 2>/dev/null; then
    ok "armor job series count sane ($SERIES_COUNT)"
else
    bad "armor job series count is ${SERIES_COUNT:-unreadable} (missing or >=20000 — cardinality blowup or keep-list regression)"
fi

note ""

# ----------------------------------------------------------------- evaluation
note "-- Phase 2: alert evaluation (vmalert rule health + shipped-rule pin)"

RULES_CHECK="$(curl_json "$VMALERT_BASE" /api/v1/rules 2>/dev/null | python3 -c '
import json,sys,os,time
want = {
    "ArmorRestoreVerificationStale": (
        "time() - armor_last_verified_restore_timestamp > 12 * 3600", "10m"),
    "ArmorRestoreVerificationFailures": (
        "sum by (bucket) ( rate(armor_restore_verification_failures_total[1h]) ) > 0", "5m"),
    "ArmorRestoreVerificationLowObjectRatio": (
        "armor_verified_object_ratio < 0.95", "5m"),
    "ArmorRestoreVerificationDualPathDivergence": (
        "sum by (bucket) ( increase(armor_restore_verification_failures_total[1h]) ) > 0", "5m"),
    "ArmorMultipartCanaryUnhealthy": (
        "armor_multipart_canary_healthy == 0", "10m"),
}
def collapse(s):
    # Prometheus-compatible APIs are allowed to canonicalize optional
    # whitespace around parentheses (the ardenone Prometheus does this),
    # while the contract fixture preserves the authored layout. Normalize
    # only that insignificant punctuation whitespace; keep all operator and
    # token spacing significant so a real expression change still fails.
    import re
    return re.sub(r"\s*([()])\s*", r"\1", " ".join(s.split()))
def dur_seconds(d):
    # The vmalert rules API reports duration in plain SECONDS (600), while
    # the shipped contract fixtures write Prometheus duration strings
    # ("10m"). Normalize both before comparing so the pin is about the
    # value, not the API unit convention.
    d = str(d).strip()
    if d.endswith("ms"): return float(d[:-2]) / 1000.0
    if d.endswith("s"):  return float(d[:-1])
    if d.endswith("m"):  return float(d[:-1]) * 60
    if d.endswith("h"):  return float(d[:-1]) * 3600
    return float(d)
max_eval_age = int(os.environ.get("MAX_RULE_EVAL_AGE_SECONDS", "300"))
try:
    groups = json.load(sys.stdin)["data"]["groups"]
except Exception as e:
    print(f"unparseable /api/v1/rules response: {e}"); sys.exit()
g = [x for x in groups if x.get("name") == "restore-verifier"]
if not g:
    print("group restore-verifier absent — vmalert has no ARMOR rules loaded"); sys.exit()
problems = []
seen = {}
for r in g[0].get("rules", []):
    seen[r["name"]] = r
for name, (expr, dur) in want.items():
    r = seen.get(name)
    if r is None:
        problems.append(f"{name}: absent from group"); continue
    gexpr = collapse(r.get("query", ""))
    want_expr = collapse(expr)
    if gexpr != want_expr:
        problems.append(f"{name}: expr drifted from the shipped contract:\n  got  {gexpr}\n  want {expr}")
    try:
        got_s, want_s = dur_seconds(r.get("duration", "")), dur_seconds(dur)
    except (TypeError, ValueError):
        got_s = want_s = None
    if got_s is None or got_s != want_s:
        problems.append(f"{name}: for={r.get('duration')!r}, want {dur!r}")
    gerr = r.get("lastError", "")
    if gerr:
        problems.append(f"{name}: lastError={gerr!r} — rule failing to evaluate")
    geval = r.get("lastEvaluation", "")
    if geval:
        try:
            ev = time.mktime(time.strptime(geval, "%Y-%m-%dT%H:%M:%SZ"))
            age = time.time() - ev
            if age > max_eval_age:
                problems.append(f"{name}: lastEvaluation {int(age)}s ago — evaluation stalled")
        except ValueError:
            pass  # timestamp format change; lastError/freshness checks still cover the failure
extra = sorted(set(seen) - set(want))
if extra:
    problems.append(f"unexpected rules in group: {extra}")
if not problems:
    print("OK")
else:
    print("\n".join(problems))' 2>&1)"

if [ "$RULES_CHECK" = "OK" ]; then
    ok "all 5 shipped rules loaded in group restore-verifier — expressions match the contract, for-durations match, no eval errors, evaluations fresh"
else
    bad "vmalert rule check:
$RULES_CHECK"
fi

note ""

# ---------------------------------------------------------------- consistency
note "-- Phase 3: alert/data consistency (staleness + multipart, both directions)"

# The authoritative active set is vmalert's own API (an alert is listed there
# while pending or firing; the ALERTS store series only exists because
# vmalert remote-writes its state, and lags the API by a tick). Pending
# covers the `for` hold; demanding `firing` would false-fail a condition
# that started <10m ago. One mismatch is retried once after a full
# evaluation tick + margin before being called inconsistent — a condition
# that crossed the threshold between vmalert's ticks otherwise races.
active_alerts() {
    local alertname="$1" hits
    hits="$(curl_json "$VMALERT_BASE" /api/v1/alerts 2>/dev/null | python3 -c '
import json,sys
try:
    alerts = json.load(sys.stdin)["data"]["alerts"]
except Exception:
    print(-1); sys.exit()
name = sys.argv[1]
print(len([a for a in alerts
           if a.get("labels", {}).get("alertname") == name
           and a.get("state") in ("pending", "firing")]))' "$alertname" 2>/dev/null)"
    printf '%s' "$hits"
}

consistency_check() {
    local alertname="$1" expr="$2" expr_hits alert_hits
    for attempt in 1 2; do
        expr_hits="$(vm_query "$expr" | python3 -c 'import json,sys; print(len(json.load(sys.stdin)))')"
        alert_hits="$(active_alerts "$alertname")"
        if { [ "$expr_hits" -gt 0 ] && [ "$alert_hits" -gt 0 ]; } \
           || { [ "$expr_hits" -eq 0 ] && [ "$alert_hits" -eq 0 ]; }; then
            if [ "$expr_hits" -gt 0 ]; then
                ok "$alertname active and its expression matches ($expr_hits bucket(s) over threshold) — consistent"
            else
                ok "$alertname quiet and its expression over threshold nowhere — consistent"
            fi
            return
        fi
        [ "$attempt" -eq 1 ] && sleep 45
    done
    bad "$alertname INCONSISTENT: expression hits=$expr_hits, active alerts=$alert_hits"
}

consistency_check "ArmorRestoreVerificationStale" \
    'time() - armor_last_verified_restore_timestamp > 12 * 3600'
consistency_check "ArmorMultipartCanaryUnhealthy" \
    'armor_multipart_canary_healthy == 0'

# Headline for the operator reading the output: age of the newest verified
# restore against the 12h contract window. OUTSIDE the window with Phase 3
# green is the correct outcome — the alert is firing because reality is bad.
AGE="$(value_of 'time() - armor_last_verified_restore_timestamp' | cut -d. -f1)"
if [ -n "$AGE" ] && [ "$AGE" -gt $((12 * 3600)) ]; then
    note "NOTE: newest verified restore is $((AGE / 3600))h old — outside the 12h freshness window."
    note "      ArmorRestoreVerificationStale is (correctly) firing; the consistency check above is the proof."
fi

note ""

# ------------------------------------------------------------------- delivery
note "-- Phase 4: delivery (Alertmanager readiness, receiver, and alert handoff)"

AM_READY="$(curl -sS --max-time 15 -fsS "$AM_BASE/-/ready" 2>/dev/null || true)"
if [ "$AM_READY" = "OK" ]; then
    ok "Alertmanager reports ready"
else
    bad "Alertmanager /-/ready did not return OK (got: ${AM_READY:-nothing})"
fi

# The rendered config embeds the ntfy URL and its bearer token — assert on
# the receiver NAME only and never print the config body.
if curl_json "$AM_BASE" /api/v2/status 2>/dev/null | python3 -c '
import json,sys
s = json.load(sys.stdin)
cfg = (s.get("config") or {}).get("original", "")
sys.exit(0 if "name: ntfy" in cfg and "webhook_configs" in cfg else 1)' 2>/dev/null; then
    ok "Alertmanager active config routes to the ntfy webhook receiver (config not printed — it embeds the delivery token)"
else
    bad "Alertmanager active config lacks the ntfy webhook receiver — alerts would fire but never page"
fi

# A healthy receiver configuration is necessary but not sufficient: when an
# alert is already pending/firing, Alertmanager must have received the same
# alert. Compare a small label identity only; never print either response
# because annotations and rendered config can carry sensitive delivery data.
VMALERT_ALERTS_FILE="$(mktemp)"
AM_ALERTS_FILE="$(mktemp)"
trap 'rm -f "$VMALERT_ALERTS_FILE" "$AM_ALERTS_FILE"' EXIT
if curl_json "$VMALERT_BASE" /api/v1/alerts >"$VMALERT_ALERTS_FILE" 2>/dev/null \
   && curl_json "$AM_BASE" /api/v2/alerts >"$AM_ALERTS_FILE" 2>/dev/null; then
    DELIVERY_CHECK="$(VMALERT_ALERTS_FILE="$VMALERT_ALERTS_FILE" \
        AM_ALERTS_FILE="$AM_ALERTS_FILE" \
        ALERTING_REQUIRE_ACTIVE="$ALERTING_REQUIRE_ACTIVE" \
        python3 -c '
import json, os, sys

try:
    with open(os.environ["VMALERT_ALERTS_FILE"], encoding="utf-8") as handle:
        vm = json.load(handle)
    with open(os.environ["AM_ALERTS_FILE"], encoding="utf-8") as handle:
        am = json.load(handle)
except (KeyError, OSError, ValueError) as exc:
    print(f"unreadable alert API response: {exc}")
    sys.exit(2)

def identity(alert):
    labels = alert.get("labels", {})
    return tuple((key, labels.get(key, ""))
                 for key in ("alertname", "component", "bucket", "instance"))

if vm.get("status") != "success" or not isinstance(vm.get("data", {}).get("alerts"), list):
    print("vmalert alert API did not return a successful alerts list")
    sys.exit(2)
if not isinstance(am, list):
    print("Alertmanager alert API did not return an alerts list")
    sys.exit(2)
source = vm["data"]["alerts"]
source = [a for a in source if a.get("state") in ("pending", "firing")]
received = {
    identity(a) for a in am
    if a.get("status", {}).get("state") in ("active", "pending")
}
missing = [identity(a) for a in source if identity(a) not in received]
if missing:
    print(f"{len(missing)} pending/firing vmalert alert(s) absent from Alertmanager")
    sys.exit(1)
if source:
    print(f"Alertmanager received all {len(source)} pending/firing vmalert alert(s)")
elif os.environ.get("ALERTING_REQUIRE_ACTIVE") == "1":
    print("no pending/firing vmalert alert available for the required delivery drill")
    sys.exit(1)
else:
    print("no pending/firing alert to correlate; receiver and delivery APIs are healthy")' )"
    if [ "$?" -eq 0 ]; then
        ok "$DELIVERY_CHECK"
    else
        bad "$DELIVERY_CHECK"
    fi
else
    bad "could not read both vmalert and Alertmanager alert APIs"
fi

note ""
note "== $PASS passed, $FAIL failed =="

[ "$FAIL" -eq 0 ]
