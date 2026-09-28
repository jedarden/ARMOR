# Companion Binary CLI Reference

Operator-facing contract for ARMOR's two companion binaries: the continuous
restore verifier (`cmd/restore-verifier`, shipped as the
`armor-restore-verifier` image) and the fleet console (`cmd/armor-fleet`,
shipped as the `armor-fleet` image). Neither takes subcommands — each is one
long-running process configured entirely by flags and environment variables.
The `armor` binary's own subcommands are the [CLI Command
Reference](cli-reference.md). The verifier's deployment topology is the
[Restore Verifier Deployment Guide](restore-verifier-deployment-guide.md)
(ADR-004: one Deployment per bucket scope); the console's operations are the
[Fleet Console](fleet-console.md) page.

Every claim here is enforced by the smoke tests beside each binary —
`cmd/restore-verifier/cli_reference_test.go` and
`cmd/armor-fleet/cli_reference_test.go`: every flag the binary registers
appears in this document and every flag named here exists in that binary's
registry, every environment variable named here is read by the
implementation, and the binaries themselves are built and exercised for the
documented version, help, usage-error, startup-validation, HTTP-surface, and
graceful-shutdown behaviors. The secret-safety tests plant distinctive
credential values (the verifier's B2 secret key, MEK, and MEK-ring entries;
the console's SEAM token), drive the documented validation and failed-poll
paths, and pin that the binaries never echo a credential value into their
logs or served JSON — so this page cannot silently drift from the code.

## Release status and known limitations

The verifier and fleet-console contracts below describe the current source
tree, not every image already deployed. Restore-verifier has focused coverage
for fingerprinted DEKs, v3 multipart manifests, sidecars, and direct-only DR
drills; release-image and live bucket evidence is still required before an
operator should call a rollout complete. In particular, old images and
multipart objects with missing B2 metadata have produced false or incomplete
verification results. Keep the [release-status and known-limitations register](release-status.md)
beside this reference when planning a rollout.

## Shared behavior

- **Single-command processes.** No subcommands, no positional arguments.
  Positional arguments are not validated: `flag` stops parsing at the first
  one, so a stray word starts the service with defaults (and any flags after
  that word are silently not parsed). Scripts should pass flags only.
- `-h` and `--help` print the usage text (to stderr) and exit 0 without
  starting the service. An undefined or malformed flag prints the usage to
  stderr and exits 2.
- Both binaries log operational lines to stderr through the standard logger
  (timestamped plain text, not JSON) and serve HTTP until SIGINT or SIGTERM,
  both of which trigger a graceful shutdown and exit 0.
- `restore-verifier --version` (also `-v`) prints the version line and exits
  0 before any configuration is read. armor-fleet has no version flag.

Exit-code conventions used by both binaries:

| Code | Meaning |
|------|---------|
| 0 | Success: graceful SIGINT/SIGTERM shutdown; also the version and help paths |
| 1 | Startup or runtime failure: invalid configuration, missing credentials, unreadable inputs, listener bind failure |
| 2 | Usage error: undefined or malformed flag, missing required flag or credential |

## `restore-verifier`

Continuous dual-path backup verification: on a schedule it samples stored
objects and restores them through both the ARMOR read path (proxy live) and
the direct decrypt path (B2 fetch + MEK unwrap + decrypt + integrity checks,
the disaster-recovery path — ADR-004). Per ADR-009 the ARMOR leg never
decrypts; agreement between the two legs is the point. The process is meant
to run forever beside the ARMOR proxy whose bucket scope it verifies.

### Flags

| Flag | Meaning |
|------|---------|
| `-bucket` | A bucket scope to verify: `name[,prefix][,artifact_type][,enabled][,sample_size]`. Repeatable; every instance gets at least one. `prefix` is normalized ADR-001-style (leading slashes stripped, exactly one trailing slash); `artifact_type` is `sqlite`, `parquet`, `tar-gz`, or `generic`; `enabled` is `true`/`1` or `false`/`0` (default true); `sample_size` defaults to the `-sample-size` value. With no `-bucket` at all, one scope is derived from `ARMOR_BUCKET` + `ARMOR_PREFIX` |
| `-b2-region` | B2 region, e.g. `us-west-004` (default `ARMOR_B2_REGION`) |
| `-b2-endpoint` | B2 S3 endpoint (default `ARMOR_B2_ENDPOINT`); when empty and a region is set it is derived as `https://s3.<region>.backblazeb2.com`, so a Deployment can reuse the cluster's `ARMOR_B2_REGION` alone |
| `-b2-access-key` | B2 access key ID (default `ARMOR_B2_ACCESS_KEY_ID`) |
| `-b2-secret-key` | B2 secret key (default `ARMOR_B2_SECRET_ACCESS_KEY`) |
| `-cf-domain` | Cloudflare domain for reads through the proxy leg (default `ARMOR_CF_DOMAIN`) |
| `-mek` | Active master encryption key, 32-byte hex (default `ARMOR_MEK`) |
| `-mek-ring` | Comma-separated retired MEKs, 32-byte hex each (default `VERIFIER_MEK_RING`); lets mid-rotation buckets verify objects written under retired keys. Each entry is fingerprinted at startup and logged |
| `-block-size` | Encryption block size (default 65536) |
| `-read-concurrency` | Maximum concurrent ranged reads (default `ARMOR_READ_CONCURRENCY` or 16) |
| `-check-interval` | Verification cadence (default `VERIFIER_CHECK_INTERVAL` or 6h). One dual-path run per interval; a tick that fires while a run executes is dropped, so runs never chain back-to-back |
| `-sample-size` | Default number of historical objects sampled per bucket per run (default `VERIFIER_SAMPLE_SIZE` or 10) |
| `-http-listen` | HTTP listen address (default `VERIFIER_HTTP_LISTEN` or `:9002`) |
| `-run-timeout` | Per-run deadline for a verification or DR-drill run (default `VERIFIER_RUN_TIMEOUT` or 2h; 0 or negative selects the default). A run that exceeds it fails visibly instead of blocking the loop forever |
| `-exclude-prefixes` | Comma-separated stored-key prefixes to skip during sampling (default `VERIFIER_EXCLUDE_PREFIXES`) — tenant namespaces of a shared bucket whose MEKs this instance does not hold, so their DEKs do not false-alarm as keying gaps |
| `-dr-drill-interval` | Cadence of the periodic direct-only DR drill (default `VERIFIER_DR_DRILL_INTERVAL` or 0 = disabled). Independent of `-check-interval`; the drill always remains available on demand via the trigger endpoint |
| `-escalation` | File one bead per distinct verification failure plus one staleness bead per freshness window, through the bead-rs CLI (default `VERIFIER_ESCALATION` or false). Opt-in; without it failures surface via metrics only |
| `-escalation-deployment` | Deployment name recorded in escalation bead bodies (default `ARMOR_DEPLOYMENT`) |
| `-escalation-freshness-window` | Escalate once per window when no verified restore occurs (default `VERIFIER_FRESHNESS_WINDOW` or 24h) |
| `-escalation-state` | Path of the persisted dedupe-state file (default `VERIFIER_ESCALATION_STATE` or `/var/lib/restore-verifier/escalation-state.json`); mount a volume here so dedupe survives restarts |
| `-escalation-workspace` | Beads workspace root the bead CLI runs in (default `VERIFIER_ESCALATION_WORKSPACE` or `<escalation-state dir>/beads-workspace`, created and initialized at startup) |
| `-escalation-label` | Optional label applied to every escalation bead (default `VERIFIER_ESCALATION_LABEL`) |
| `-escalation-bead-binary` | Path to the bead-rs CLI used to file beads (default `VERIFIER_ESCALATION_BEAD_BINARY` or `bead`) |
| `-escalation-unique-ref-namespace` | bead unique-ref namespace making filings idempotent at the bead store (default `VERIFIER_ESCALATION_UNIQUE_REF_NAMESPACE` or `restore-verifier`; empty disables unique refs) |
| `-escalation-exec-timeout` | Per-call timeout for a single bead create (default `VERIFIER_ESCALATION_EXEC_TIMEOUT` or 10s) |

### Environment

Every flag above takes its value from argv or, when unset there, from the
environment named in its row. Exactly two variables have no flag:
`ARMOR_BUCKET` and `ARMOR_PREFIX` build the single default bucket scope when
no `-bucket` flag is passed. The environment-only path is how a
restore-verifier Deployment reuses its co-located ARMOR proxy's config
sources unchanged.

### Outputs

- **Logs:** timestamped lines on stderr — startup validation results, ring
  fingerprints, per-run discovery/restore progress, every failure with its
  reason, and the graceful-shutdown line.
- **HTTP** on `-http-listen`:

| Endpoint | Behavior |
|----------|----------|
| `GET /status` | Verification status for every configured bucket, as a JSON object keyed by bucket name |
| `GET /bucket?bucket=X` | Status for one bucket; 400 without the parameter, 404 for an unknown bucket |
| `POST /trigger` | 202; runs a dual-path verification immediately in the background. `?mode=dr-drill` runs the direct-only DR drill instead, `?mode=dual` is the explicit default, and any other mode is a 400 so a typo in automation fails loudly. A triggered run applies `-run-timeout` internally but is not cancelled by shutdown |
| `GET /healthz` | 200 only while every bucket has zero failed objects and was verified within the last 24 hours; 503 otherwise (including before the first verification finishes) |
| `GET /readyz` | 200 once at least one bucket has had a successful verification; 503 before that |
| `GET /metrics` | Prometheus metrics (`armor_restore_verifier_*`) |

### Exit codes

- **0** — `--version`/`-v`, the help paths, or a graceful SIGINT/SIGTERM
  shutdown: the verification loop is stopped, then the HTTP server drains
  with a 30-second cap.
- **1** — startup validation or a fatal runtime error, each logged with its
  reason: missing B2 credentials (region, endpoint, access key, or secret
  key), missing MEK, MEK that is not valid hex or not 32 bytes, an invalid
  MEK-ring entry, B2 backend construction failure, no bucket scope (no
  `-bucket` flags and no `ARMOR_BUCKET`), and failure to bind the HTTP
  listener.
- **2** — an undefined or malformed flag.

Positional arguments are ignored (see Shared behavior).

### Deployment behavior

- One Deployment per ARMOR bucket scope (bucket + MEK + B2 credential set),
  usually co-located with that scope's ARMOR proxy and reusing its env
  sources — never one fleet-wide verifier (ADR-004). The authoritative
  inventory and the YAML shape are the
  [deployment guide](restore-verifier-deployment-guide.md).
- The verification loop starts immediately (first run at startup, then one
  run per `-check-interval`). The first periodic DR drill is deliberately
  scheduled one full drill interval after start rather than chained behind
  the startup verification, so a slow first run cannot delay the loop.
- Escalation is off until a deployment sets `-escalation`. When on, a failed
  startup validation (missing bead CLI, unwritable workspace) is loud but
  not fatal: the exact remediation is logged and the verifier keeps running
  with filing disabled — a crash-looping verifier proves nothing.
- Graceful shutdown: SIGINT/SIGTERM stops scheduling, waits for the loop to
  acknowledge, then drains HTTP for up to 30 seconds.

### Safe use

- **The verifier is read-only against B2.** It downloads objects and
  manifests and never writes, deletes, or re-encrypts anything; the only
  state it writes is its own escalation state file and bead workspace.
- **`ARMOR_MEK` and `VERIFIER_MEK_RING` are key material.** Pass them by
  environment (a Deployment env source), never on a command line — argv
  lands in shell history and `ps`. The ring is as sensitive as the active
  MEK.
- **The HTTP surface is unauthenticated.** Anyone who can reach the listener
  can trigger verification runs (real B2 egress) or read status. Keep
  `-http-listen` on a cluster-internal address; the alerting pipeline
  scrapes it from inside the cluster.
- **Size `-run-timeout` for the slowest healthy bucket**, never smaller to
  force failures quiet — the default fits the slowest fleet bucket at the
  time of writing, and a run that legitimately needs longer should get a
  larger value explicitly.
- **On a shared bucket (ADR-001), set `-exclude-prefixes`** to the other
  tenants' prefixes; without it their objects are sampled, fail DEK unwrap,
  and surface as keying-gap noise instead of the scope's real result.
- Escalation files beads into a real bead workspace on the mounted state
  volume; treat that volume as part of the deployment's state, and expect
  one bead per distinct failure rather than one per run (the dedupe set and
  unique refs are the storm-proofing).

## `armor-fleet`

The fleet console: a single-binary web page that aggregates the live health
of every configured ARMOR instance — version, canary status, error counter,
restore-verifier gauges — by polling each instance's admin API through
SEAM's read-only Kubernetes proxy. It is read-only end to end; what the
page shows and how it complements the drift checker are the
[Fleet Console](fleet-console.md) page.

### Flags

| Flag | Meaning |
|------|---------|
| `-targets` | Required. Path to the targets YAML file: a list of `name`, `cluster`, `namespace`, `service`, `admin_port` entries, every field required — name labels the card, the rest locate the instance's admin Service through SEAM. A file that is unreadable, unparseable, empty, or has an entry missing a field is a startup failure |
| `-listen` | Console HTTP listen address (default `:8080`) |
| `-interval` | Poll interval in seconds (default 60). The first poll runs synchronously at startup, before the HTTP server binds |
| `-seam-token` | SEAM bearer token; required, from this flag or `SEAM_TOKEN`. Missing from both is a usage error |

### Environment

`SEAM_TOKEN` — the SEAM bearer token when `-seam-token` is not passed. The
token needs only SEAM's read-only `k8s-ro` scope: the console issues GETs
against `/k8s/<cluster>/api/v1/namespaces/<namespace>/services/<service>:<admin_port>/proxy`
and nothing else.

### Outputs

- **Logs:** startup and poll diagnostics on stderr (targets loaded, listener
  address, per-target poll failures with which leg failed).
- **HTTP** on `-listen`:

| Endpoint | Behavior |
|----------|----------|
| `GET /` | The fleet dashboard HTML (self-contained; cards per target plus reachable/unreachable tiles, refreshed in the browser every 30 seconds) |
| `GET /fleet.json` | The live status document: a JSON object keyed by target name, each entry carrying version, canary health/message, the error-counter sample, restore-verifier gauges, last-seen, reachability, and the poll error when unreachable |
| `GET /metrics` | Prometheus metrics: `armor_fleet_up_targets` and `armor_fleet_down_targets` |
| `GET /agentation.js` | The vendored Agentation toolbar module wired into the page |

Per target each poll makes three GETs — `/version`, `/armor/canary`,
`/metrics` — each with a 10-second client timeout and the whole cycle budget
30 seconds; the first failed leg marks the target unreachable with that
leg's error and skips the rest. A failed poll never stops the process.

### Exit codes

- **0** — graceful SIGINT/SIGTERM shutdown (HTTP drain capped at 5 seconds).
- **1** — startup or runtime failure: targets file unreadable or
  unparseable, zero targets in it, an entry missing a required field, HTTP
  listener bind failure, or a shutdown error.
- **2** — missing `-targets`, or no SEAM token from flag and environment, or
  an undefined flag.

Positional arguments are ignored (see Shared behavior).

### Deployment behavior

- Runs as a single console instance (not per cluster): one Deployment, one
  targets file derived from the live fleet inventory, private pinned-semver
  image. The ops detail lives in [Fleet Console](fleet-console.md).
- Polling starts before the listener binds, so the first `/fleet.json` fetch
  already has data. Later polls replace per-target state atomically;
  `/fleet.json` always answers, even while every target is unreachable.
- The browser re-fetches `/fleet.json` every 30 seconds; `-interval` governs
  only the server-side polling cadence.

### Safe use

- **The SEAM token is a credential.** Prefer `SEAM_TOKEN` in the
  Deployment's env source over `-seam-token` on a command line — argv lands
  in shell history and `ps`. The read-only scope is the boundary: the token
  cannot mutate anything even if it leaks.
- **Keep `-listen` off public interfaces** (loopback or tailnet). The
  console exposes fleet topology (clusters, namespaces, services, ports) and
  instance health to anyone who can reach it.
- The console is read-only by construction — it has no write path to any
  target — but it is still an information disclosure surface; treat its
  listener like the other admin surfaces.
- The card labeled Error Rate is a single labelled `armor_errors_total`
  counter sample, not a computed rate; alert on `rate(armor_errors_total[5m])`
  scraped from the instance, per [Metrics](metrics.md).
