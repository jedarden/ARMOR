# ARMOR Fleet Drift Checker

The fleet check is a continuously running, read-only Deployment in the
iad-ci armor namespace. Its schedule is an in-process sleep loop, not a
CronJob or CronWorkflow, so ArgoCD continuously owns the checker and its
report endpoint remains available between checks.

The pod refreshes the ARMOR and declarative-config repositories, then runs
scripts/drift_check.py immediately and once per hour. The checker:

- compares every pinned ronaldraygun/armor* Deployment image with the
  checkout VERSION and release tags;
- queries each cluster's read-only Kubernetes API proxy for the corresponding
  Deployment and selected pods, comparing the live image tags and digests to
  the declared pin;
- marks unavailable workloads red when the Deployment has no pods or the API
  cannot be reached;
- reports the latest approved version, current version, pod counts, and
  missed correctness releases per cluster/image pair; and
- exposes a Prometheus text snapshot at /metrics and the complete JSON
  report at /report.json.

armor_version_drift_alert 1 and a log line beginning with
ARMOR drift checker ESCALATION are emitted for stale, mismatched, or
unavailable workloads. armor_version_drift_check_success 0 is emitted for
an operational check failure or drift, so an external scraper can alert even
when the checker pod itself is healthy. A failed repository refresh leaves the
last report available but sets the check-success gauge to zero.

The live API endpoints are configured in config/drift-config.json. The
in-cluster iad-ci endpoint uses the checker's read-only ServiceAccount token;
the other endpoints are the existing read-only cluster proxies. The checker
never mutates a Deployment, pod, or repository.

## Operator inspection

Read the current report or metrics through the read-only proxy:

~~~bash
kubectl --server=http://traefik-iad-ci:8001 \
  get --raw '/api/v1/namespaces/armor/services/armor-drift-checker:8080/proxy/report.json'
kubectl --server=http://traefik-iad-ci:8001 \
  get --raw '/api/v1/namespaces/armor/services/armor-drift-checker:8080/proxy/metrics'
kubectl --server=http://traefik-iad-ci:8001 \
  logs -n armor deploy/armor-drift-checker --tail=100
~~~

If an alert fires, use the JSON report to identify the cluster, declared
image, live pod image IDs, and missed correctness-labelled release. Rollback
or rollout changes belong in declarative-config; this checker is observation
and escalation only.
