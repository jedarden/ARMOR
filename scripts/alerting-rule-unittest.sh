#!/usr/bin/env bash
# Offline semantics test for the shipped ARMOR alert set (ADR-002 / ADR-004
# §6): extracts the contract-pinned rule group from
# internal/metrics/testdata/restore-verifier-monitoring.yaml and runs it
# through promtool's rule test engine against
# scripts/alerting-rules-unittest.yaml — freshness-failure firing and
# multipart health transitions, provable in seconds rather than by waiting
# 12h on the live stack or breaking a real canary.
#
# This is the offline half of activation verification; the live half is
# scripts/alerting-smoke-test.sh. Both must pass for the alerting surface to
# count as active.
#
# promtool runs in the pinned prom/prometheus image (PROMTOOL_IMAGE
# overrides). Only docker.io is pulled; the pin must move with a verified
# tag, never float.
#
# Exit: 0 all scenarios pass; 1 test failure; 2 environment problem.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FIXTURE="$REPO_ROOT/internal/metrics/testdata/restore-verifier-monitoring.yaml"
UNITTEST_SRC="$REPO_ROOT/scripts/alerting-rules-unittest.yaml"
PROMTOOL_IMAGE="${PROMTOOL_IMAGE:-prom/prometheus:v3.6.0}"

command -v docker >/dev/null 2>&1 || { echo "FATAL: docker not on PATH" >&2; exit 2; }
command -v python3 >/dev/null 2>&1 || { echo "FATAL: python3 not on PATH" >&2; exit 2; }
# codinghome's docker daemon lives on the user socket; harmless elsewhere.
if [ -z "${DOCKER_HOST:-}" ] && [ -S /run/user/1000/docker.sock ]; then
    export DOCKER_HOST=unix:///run/user/1000/docker.sock
fi

[ -f "$FIXTURE" ] || { echo "FATAL: fixture missing: $FIXTURE" >&2; exit 2; }
[ -f "$UNITTEST_SRC" ] || { echo "FATAL: unittest missing: $UNITTEST_SRC" >&2; exit 2; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Materialize rules.yml: the PrometheusRule doc's rule group, and nothing
# else — promtool wants rule groups, not ServiceMonitor CRs.
python3 - "$FIXTURE" "$WORK/rules.yml" <<'PYEOF'
import sys
import yaml

fixture, out = sys.argv[1], sys.argv[2]
promrule = None
for doc in yaml.safe_load_all(open(fixture)):
    if isinstance(doc, dict) and doc.get("kind") == "PrometheusRule":
        promrule = doc
if promrule is None:
    sys.exit("no PrometheusRule document in fixture")
groups = promrule["spec"]["groups"]
yaml.safe_dump({"groups": groups}, open(out, "w"), sort_keys=False, allow_unicode=True)
PYEOF

cp "$UNITTEST_SRC" "$WORK/unittest.yaml"

# The mount is read-only so a promtool bug cannot write into the workspace;
# the container user is arbitrary, hence the world-readable files.
chmod -R a+rX "$WORK"

docker run --rm --entrypoint promtool \
    -v "$WORK":/work:ro \
    "$PROMTOOL_IMAGE" \
    test rules /work/unittest.yaml
