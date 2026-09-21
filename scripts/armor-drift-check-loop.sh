#!/bin/sh
# Runtime for the GitOps-managed armor-drift-checker Deployment.
#
# The Deployment mounts this file from declarative-config and runs it in an
# alpine/git image. Keeping the scheduler here (rather than in a CronWorkflow)
# means the pod is continuously reconciled and the last report/metric remains
# available even when a check itself finds drift.

set -eu

INTERVAL_SECONDS=${DRIFT_CHECK_INTERVAL_SECONDS:-3600}
WORK_ROOT=${DRIFT_CHECK_WORK_ROOT:-/work}
REPORT_ROOT=${DRIFT_CHECK_REPORT_ROOT:-/report}
mkdir -p "$WORK_ROOT" "$REPORT_ROOT"

# The report endpoint is deliberately independent from the check process. A
# failed clone or check therefore leaves the previous JSON report readable and
# can publish a red check-success gauge below.
busybox httpd -f -p 8080 -h "$REPORT_ROOT" >/dev/null 2>&1 &

write_startup_failure() {
  now=$(date +%s)
  cat >"$REPORT_ROOT/metrics" <<EOF
# HELP armor_version_drift_alert Whether the checker needs operator action.
# TYPE armor_version_drift_alert gauge
armor_version_drift_alert 1
# HELP armor_version_drift_check_success Whether the most recent check completed without error or drift.
# TYPE armor_version_drift_check_success gauge
armor_version_drift_check_success 0
# HELP armor_version_drift_last_run_timestamp_seconds Unix time of the most recent check.
# TYPE armor_version_drift_last_run_timestamp_seconds gauge
armor_version_drift_last_run_timestamp_seconds $now
EOF
}

write_startup_failure

# The token is read by git's credential helper from the environment. It never
# appears in argv, a report, or the logs.
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0=credential.helper
export GIT_CONFIG_VALUE_0='!f() { test "$1" = get && echo "username=x-token" && echo "password=$FORGEJO_TOKEN"; }; f'

while :; do
  rc=2
  rm -rf "$WORK_ROOT/armor.next" "$WORK_ROOT/declarative-config.next"
  if git clone --single-branch --branch main https://git.ardenone.com/jedarden/ARMOR.git "$WORK_ROOT/armor.next" && git clone --depth 1 --single-branch --branch main https://git.ardenone.com/jedarden/declarative-config.git "$WORK_ROOT/declarative-config.next"; then
    rm -rf "$WORK_ROOT/armor" "$WORK_ROOT/declarative-config"
    mv "$WORK_ROOT/armor.next" "$WORK_ROOT/armor"
    mv "$WORK_ROOT/declarative-config.next" "$WORK_ROOT/declarative-config"

    cd "$WORK_ROOT/armor"
    python3 scripts/drift_check.py --config config/drift-config.json --manifests "$WORK_ROOT/declarative-config" --json --require-live --output "$REPORT_ROOT/report.json" --metrics-output "$REPORT_ROOT/metrics" || rc=$?
  else
    echo "ARMOR drift checker: repository refresh failed; retaining the last report" >&2
  fi

  case "$rc" in
    0) echo "ARMOR drift checker: fleet current" ;;
    1) echo "ARMOR drift checker ESCALATION: version drift or a missed correctness fix detected; inspect /report.json and /metrics" >&2 ;;
    2) echo "ARMOR drift checker ERROR: check could not complete; inspect logs and /metrics" >&2 ;;
  esac
  if [ "$rc" -eq 2 ]; then
    write_startup_failure
  fi
  sleep "$INTERVAL_SECONDS"
done
