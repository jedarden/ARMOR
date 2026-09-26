# ARMOR Fleet Console

The fleet console (`cmd/armor-fleet`) is a single-binary web page that
aggregates the live health of every ARMOR instance in the fleet on one
screen. It is read-only end to end: it polls each instance's admin API
through SEAM's read-only Kubernetes proxy and offers no way to mutate
anything.

## What the console shows

One card per configured target, plus fleet summary tiles (total /
reachable / unreachable), refreshed from `/fleet.json` every 30 seconds in
the browser. Per reachable target:

- **Version** — from the instance's `/version` admin endpoint.
- **Canary status and message** — from `/armor/canary`, including the
  multipart canary flag. These are the ADR-002 canary surfaces; their
  contract is pinned in [Observability Contract](observability-contract.md).
- **Error Rate** — the value of the last `armor_errors_total` series in the
  instance's `/metrics` output. Despite the UI label this is a single
  labelled counter sample (one `code`/`operation` pair, cumulative since
  process start), not a computed rate; for real rate alerting scrape the
  instance directly and use `rate(armor_errors_total[5m])` as in
  [Metrics](metrics.md).
- **Restore Verifier** — every `armor_restore_verifier_*` gauge the
  instance exposes, name and value (the ADR-004 dual-path verification
  surfaces; see the [Restore Verifier Deployment
  Guide](restore-verifier-deployment-guide.md)).
- **Last Seen** — when this target was last polled.

An unreachable target's card shows the poll error instead (version, canary,
or metrics check failed) — which leg failed is part of the message.

The console's own `/metrics` endpoint publishes two gauges for external
scrapers: `armor_fleet_up_targets` and `armor_fleet_down_targets`.

## How it works

The console reads a targets YAML file listing the ARMOR instances to
monitor:

```yaml
- name: rs-manager
  cluster: rs-manager
  namespace: armor
  service: armor
  admin_port: 9001
```

`name` is the card label; `cluster`, `namespace`, `service`, and
`admin_port` locate the instance's admin Service (9001 is the ARMOR admin
listener convention, where `/version`, `/armor/canary`, and `/metrics`
live). `cmd/armor-fleet/test_targets.yaml` is a shape template — derive
real targets from the live inventory
(`python3 scripts/find-armor-deployments.py <declarative-config>`), not
from that file.

Each poll cycle (default every 60 s) the console issues three GETs per
target against SEAM's Kubernetes API proxy:

```
https://seam-rs-manager.tail1b1987.ts.net/k8s/<cluster>/api/v1/namespaces/<namespace>/services/<service>:<admin_port>/proxy/{version,armor/canary,metrics}
```

with a 10 s HTTP timeout per request and a 30 s bound on a full cycle. Any
failed leg marks the target unreachable until the next successful poll; the
last snapshot (healthy or not) stays served between polls. The console
never contacts a cluster's API server directly and never issues anything
but GET.

## Running it

```
armor-fleet -targets <file> [-listen <addr>] [-interval <seconds>] [-seam-token <token>]
```

| Flag | Default | Meaning |
|------|---------|---------|
| `-targets` | (required) | Path to the targets YAML file |
| `-listen` | `:8080` | Console listen address |
| `-interval` | `60` | Poll interval in seconds |
| `-seam-token` | `SEAM_TOKEN` env | SEAM bearer token |

Both the targets file and the token are required — the console exits
non-zero at startup without them. `make build` produces `bin/armor-fleet`.

## Deploying the image

CI builds the console on every release from the `armor-fleet-runtime`
stage of the repo `Dockerfile` and publishes it as
`ronaldraygun/armor-fleet:<VERSION>` (the `docker-build-fleet` step of the
`armor-build` WorkflowTemplate). It shares the server's release train: the
tag is the same `0.1.<counter>` as `ronaldraygun/armor:VERSION`, so a
console deployment tracks the fleet by pinning the version you want to
observe with.

- The image is **private on Docker Hub** (operator decision, 2026-09-19 —
  no GHCR mirror; see [Release Process](release-process.md)). Cluster
  workloads pull it with the fleet's existing pull secret; for local use,
  `make build` avoids the registry entirely.
- **Pin a semver tag**, optionally with a `@sha256:` digest like the ARMOR
  server pins use. Never `:latest` — CI publishes semver tags only; a
  `latest` visible in a registry is a stale leftover, not something CI
  pushed.
- Deployment manifests live in `jedarden/declarative-config` like every
  other ARMOR surface: a small Deployment (the console is a static binary
  on a `scratch` base), the targets file as a ConfigMap, and the SEAM token
  from an ExternalSecret — never a literal. As of 2026-09-26 no fleet
  console Deployment exists in declarative-config; the image is published
  every release and the console runs wherever an operator starts it. Add
  the manifest there (not here) to stand one up, and let ArgoCD sync it.

## Reaching it and authentication

The console serves three routes on `-listen` (default `:8080`): `/` (the
HTML dashboard), `/fleet.json` (the raw status map keyed by target name),
and `/metrics` (its own Prometheus gauges).

Authentication has two separate legs:

- **Outbound (console → SEAM):** every poll carries the SEAM token as a
  bearer token. It must hold the `k8s-ro:get` scope — the same read-only
  scope SEAM's Kubernetes proxy route requires of any client — and what
  the console can see is exactly what that token's SEAM identity is
  allowed to see. This is the only secret the console handles; deliver it
  by reference (OpenBao path / ExternalSecret / environment), never as a
  literal in a manifest or command line.
- **Inbound (browser → console):** none. The console has no login, no TLS,
  and no authorization of its own — like the ARMOR dashboard and the drift
  checker, it is protected by where it listens, not by credentials. Keep it
  on a tailnet-only surface; see the [Connection Guide](connection-guide.md)
  for the topology.

## Relationship to the rest of the fleet

- **Per-cluster ARMOR instances** are the data source: the console reads
  their admin API through SEAM rather than through per-cluster kubectl, so
  one tailnet-reachable SEAM endpoint covers every cluster SEAM fronts.
  Cluster-level detail still lives on each instance's own
  [Dashboard](dashboard.md) and `/metrics`.
- **The Fleet Drift Checker** ([ARMOR Drift
  Checker](armor-drift-checker.md)) is the complement, not a duplicate: it
  runs continuously on a schedule, compares declared image pins against
  live Deployments, and escalates on drift. The console is an on-demand
  live snapshot with no escalation path of its own. Version-staleness
  questions go to the drift checker and [Drift Check](drift-check.md);
  "is it healthy right now" is what this console answers.
