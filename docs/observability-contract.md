# Canary & Restore-Verifier Observability Contract

This document is the **published contract** for the integrity-observability
surface defined by [ADR-002](adr/002-multipart-corruption-detection-gaps.md)
(multipart write-path detection) and [ADR-004](adr/004-continuous-restore-verification.md)
(continuous dual-path restore verification). It pins, in one place:

- the status API response fields and state-transition rules (canary on the
  ARMOR server; restore-verifier as a standalone Deployment),
- the exact Prometheus series names, types, and labels each component emits,
- the freshness windows and alert thresholds that consume them,
- the failure-escalation and deduplication rules that make the escalation
  path storm-proof.

A field, series, label, or threshold listed here is contractual: renaming or
removing one is a breaking change and must update this document and the
contract tests in the same change. The contract tests live in:

- `internal/canary/contract_test.go` — status API shape and state transitions
- `internal/server/canary_endpoint_contract_test.go` — the `/armor/canary`
  endpoint's HTTP behavior: the not-configured shape, 405 on non-GET, and
  multipart-failure reporting independent of the small-object status
- `internal/metrics/observability_contract_test.go` — emitted series and
  gauge/counter transitions
- `internal/metrics/alert_rules_contract_test.go` — the shipped alert-rule
  set (expressions, hold durations, severities, component labels, scrape
  targets) against the in-repo copy of the declarative-config manifest, and
  the rule that every referenced series is actually exported
- `internal/restoreverifier/contract_test.go` — verifier status/health/ready/
  trigger endpoints and their transitions

The per-metric reference (with examples for the remaining, non-contractual
series such as request/cache/replication metrics) is [metrics.md](metrics.md).

## Component map

| Component | Process | Status API | Metrics endpoint |
|---|---|---|---|
| Integrity canary (small object, multipart, secondary backend) | in every ARMOR server | `GET /armor/canary` on the admin mux | `GET /metrics` on the admin mux |
| Restore-verifier (dual-path + DR-drill) | standalone Deployment (`cmd/restore-verifier`) | `/status`, `/bucket`, `/healthz`, `/readyz`, `POST /trigger` on its own listener (default `:9002`) | `GET /metrics` on the same listener |

The two surfaces are scraped separately (see [Alert rules](#alert-rules)): the
multipart-canary gauge is exported by ARMOR, the restore gauges by the
restore-verifier — a scrape of one never implies the other is alive. That
separation is deliberate: the verifier must not share fate with the server it
verifies (ADR-004).

Auth: `GET /armor/canary` and `GET /metrics` on the ARMOR admin mux are
token-free (`adminPublicExact` in `internal/server/admin_auth.go`) because
kubelet probes and Prometheus scrape them. Every other admin path, including
`/armor/audit`, requires the `ARMOR_ADMIN_TOKEN` bearer token. The
restore-verifier's own listener exposes no authentication — it is
cluster-internal only; do not publish it through an ingress.

## Canary status API — `GET /armor/canary`

Returns HTTP 200 with the canary `Result` JSON (an unauthenticated GET; other
methods get 405). When the canary monitor is disabled or not configured, the
endpoint still returns 200 with:

```json
{"status":"unknown","error":"canary monitor not configured"}
```

### Response fields

| Field | Type | Meaning |
|---|---|---|
| `status` | string | Small-object canary status: `healthy`, `unhealthy`, or `unknown` |
| `last_check` | RFC3339 time | Last small-object check attempt (zero value until the first attempt) |
| `last_success` | — | **Not present in this response.** See `MarshalJSON` below |
| `consecutive_success` / `consecutive_failures` | — | **Not present in this response** |
| `last_error` | string | Error from the last failed check; omitted when the last check passed |
| `upload_latency_ms`, `download_latency_ms` | int | Latencies of the last completed check's upload and download phases |
| `decrypt_verified`, `hmac_verified` | bool | Pipeline stages that were exercised and passed on the last completed check |
| `cloudflare_cache_hit` | bool | `CF-Cache-Status` was `HIT`, `STALE`, or `REVALIDATED` on the last download |
| `multipart_healthy_status` | string | Multipart canary status: `healthy`, `unhealthy`, or `unknown` |
| `multipart_healthy` | bool | Boolean projection of `multipart_healthy_status` (`true` iff `healthy`) |
| `multipart_last_check` | RFC3339 time | Last multipart check attempt |
| `multipart_consecutive_fails` | int | Consecutive failed multipart checks since the last success |
| `multipart_last_error` | string | Error from the last failed multipart check; omitted when healthy/never run |
| `secondary_healthy_status` | string | Secondary-backend canary status (ADR-006); `unknown` when no secondary is configured |
| `secondary_healthy` | bool | Boolean projection of `secondary_healthy_status` |
| `secondary_last_check` | RFC3339 time | Last secondary-backend check attempt |
| `secondary_consecutive_fails` | int | Consecutive failed secondary checks since the last success |
| `secondary_last_error` | string | Error from the last failed secondary check; omitted when healthy/never run |
| `secondary_replication_lag_ms` | int | Replication lag reported by the replication queue (ms) |
| `secondary_queue_depth` | int | Replication queue depth at check time |

The struct served is `canary.Result` (`internal/canary/canary.go`). The
monitor's richer diagnostic form (`Monitor.MarshalJSON`, the `CanaryState`)
additionally carries `instance_id`, `last_success`, and
`consecutive_success`; the HTTP contract above is the `Result` shape.

`Monitor.GetStatus` never returns partial JSON: every field above is always
serialized except the three `*_last_error` fields, which are `omitempty`.

### Status transitions

All three canary families follow the same transition rules:

- **`unknown`** — the state of a freshly constructed monitor. A check that
  has never completed (in either direction) leaves the status `unknown`.
- **`healthy`** — a check attempt completed end-to-end: upload → download →
  HMAC verify → decrypt → plaintext-SHA verify. A success sets `last_check`
  (and `last_success`), increments `consecutive_success`, resets
  `consecutive_failures` to 0, and clears `last_error`.
- **`unhealthy`** — a check attempt failed after exhausting the retry budget
  (`maxRetries`, default 3, with `retryDelay` default 10s between attempts).
  A failure increments `consecutive_failures`, resets `consecutive_success`
  to 0, and records `last_error`. **One transient backend error never flips
  the status** — the retries absorb it.

The small-object canary writes/reads/deletes a unique ~1 KiB object every
`interval` (default 5m) through the full envelope pipeline. The multipart
canary runs a real `CreateMultipartUpload`/`UploadPart`/`CompleteMultipartUpload`
cycle — concurrent, shuffled part uploads included — at `multipartInterval`
(default 1h) with content sized above B2's 5 MiB non-final-part minimum
(two block-aligned 5.25 MiB parts for v1/v2; three random parts for v3), then
verifies byte-for-byte. Its status lives in `multipart_healthy_status` /
`multipart_healthy`, independently of the small-object status — a
small-object-only regression cannot mask a multipart one, and vice versa
(ADR-002 decision 1). The secondary-backend canary (ADR-006) writes to both
backends, byte-compares the envelopes, decrypts from the secondary, and
reports queue lag — its status is `unknown` forever when no secondary
backend is configured, which is not an alarm.

### How `/readyz` consumes the canary

When the canary monitor is running, it is authoritative for ARMOR readiness:
`/readyz` returns `ready=false` ("Not ready - canary check failed") when the
**small-object** status is not `healthy`. The response carries
`ready`, `canary_age_s` (seconds since the small-object `last_check`),
`multipart_canary_healthy` (boolean projection of `multipart_healthy_status`),
`manifest_flushed_s`, and `reason`. Note that `multipart_canary_healthy` is
**reported but not gating** in `/readyz` — a multipart regression is visible
in the response body and in Prometheus (below) without failing the pod's
readiness probe, because the multipart check's 1h cadence makes it a poor
probe signal. With the canary disabled (`status=unknown` case), `/readyz`
falls back to the manifest writer's flush recency.

## Canary metric series

Emitted by `internal/metrics.Metrics.PrometheusFormat` on the ARMOR admin
`/metrics` endpoint. All names are prefixed `armor_`. The canary family is
exactly the fourteen series below (plus the multipart histogram) — the
contract test in `internal/metrics` pins this set:

| Series | Type | Labels | Value contract |
|---|---|---|---|
| `armor_canary_checks_total` | counter | — | Incremented at the start of every small-object check cycle |
| `armor_canary_check_failures_total` | counter | — | Incremented once per check cycle that exhausted its retries |
| `armor_canary_last_check_time` | gauge | — | RFC3339 **string** of the last check start (a string value, not a timestamp number — see the string-valued gauge caveat below) |
| `armor_canary_last_check_error` | gauge | — | Last failure's error string; `""` after a passing check |
| `armor_multipart_canary_checks_total` | counter | — | As above, multipart family |
| `armor_multipart_canary_check_failures_total` | counter | — | As above, multipart family |
| `armor_multipart_canary_last_check_time` | gauge | — | RFC3339 string |
| `armor_multipart_canary_last_check_error` | gauge | — | Error string or `""` |
| `armor_multipart_canary_healthy` | gauge | — | `1` after a passing multipart check, `0` after a failed one; `0` also before the first completed check |
| `armor_secondary_canary_checks_total` | counter | — | Secondary-backend family (ADR-006) |
| `armor_secondary_canary_check_failures_total` | counter | — | Secondary-backend family |
| `armor_secondary_canary_last_check_time` | gauge | — | RFC3339 string |
| `armor_secondary_canary_last_check_error` | gauge | — | Error string or `""` |
| `armor_secondary_canary_healthy` | gauge | — | `1`/`0`; stays `0` when no secondary backend is configured |
| `armor_multipart_canary_upload_duration_seconds` | histogram | `operation` (`upload`\|`verify`), `status` (`success`\|`failure`) | `_sum`, `_count`, and `_last` per label pair; a label pair's series appear only once it has at least one observation |

**String-valued gauge caveat.** Every `*_last_check_time` and
`*_last_check_error` gauge carries a *string* value (RFC3339 timestamp or
error text), and the exporter renders it with an extra layer of JSON
quoting: `expvar.String.String()` already JSON-quotes and the exporter
applies `%q` on top. On the wire a timestamp reads
`armor_canary_last_check_time "\"2023-11-14T22:13:20Z\""` — a Prometheus
parser sees the value as `"2023-11-14T22:13:20Z"` **including the literal
quote characters**. Nothing alert-side may treat these as numeric or
timestamp-typed samples; they are diagnostic strings. (Fixing the double
quoting is an exporter change, not a documentation change, and would be a
breaking edit to this contract.)

There is **no** `armor_canary_healthy` series. Small-object canary health is
carried by `/armor/canary`'s `status` field and by `/readyz`; only the
multipart and secondary families have health gauges. (An earlier revision of
[metrics.md](metrics.md) documented `armor_canary_healthy`; that series has
never been exported and the entry has been removed.)

## Restore-verifier status APIs

The restore-verifier serves its status surface on its own listener
(`VERIFIER_HTTP_LISTEN`, default `:9002`). All GET endpoints reject non-GET
with 405.

### `GET /status` — all buckets

HTTP 200 with a JSON object keyed by bucket name; each value is a
`BucketState`:

| Field | Type | Meaning |
|---|---|---|
| `bucket` | string | Bucket name |
| `last_verification` | RFC3339 time | Last dual-path run attempt for this bucket (zero until the first run) |
| `last_success` | RFC3339 time | Last dual-path run in which **every** sampled object verified (zero = never) |
| `verified_object_ratio` | float | `verified/total` of the most recent run, in `[0,1]` |
| `total_objects`, `verified_objects`, `failed_objects` | int | Latest run's sample census |
| `recent_results` | array | Most recent per-object `VerificationResult`s (bounded ring; debugging + escalation evidence) |
| `historical_sample_size` | int | Configured per-bucket historical sample size |
| `drill_last_verification` | RFC3339 time | Last **direct-only DR-drill** attempt — deliberately separate from the dual-path fields |
| `drill_last_success` | RFC3339 time | Last drill in which recovery was proven with no ARMOR server in the loop (zero = never) |
| `drill_total_objects`, `drill_verified_objects`, `drill_failed_objects` | int | Latest drill run's census |

A drill that succeeds while the ARMOR read path is down records progress in
the `drill_*` fields **without** claiming the dual path is healthy — and a
dual-path run never advances the `drill_*` fields. The unexported discovery
scratch state (last enumeration walk) does not appear in this JSON.

### `VerificationResult` (members of `recent_results`)

| Field | Type | Meaning |
|---|---|---|
| `key`, `bucket` | string | Object identity |
| `status` | string | See the status table below |
| `path` | string | `armor`, `direct`, or `dual_match` |
| `timestamp` | RFC3339 time | When this verification ran |
| `artifact_type` | string | `sqlite`, `parquet`, `tar-gz`, or `generic` |
| `expected_sha256` | string | Plaintext SHA-256 declared in the object's ARMOR metadata (empty if none) |
| `armor_sha256` | string | Digest recovered via the ARMOR read path (empty if that path did not complete) |
| `direct_sha256` | string | Digest recovered direct-from-ciphertext via the `armor decrypt` logic (empty if that path did not complete) |
| `armor_path_latency_ms`, `direct_path_latency_ms` | int | Per-path restore latencies. **Unit caveat:** the fields hold `time.Duration` values, so the JSON integers are nanoseconds despite the `_ms` suffix — the `_ms` spelling is historical; pin the name, read the value as a duration |
| `error` | string | Failure detail; omitted on pass |
| `assertion_passed` | bool | Application-level artifact assertion (SQLite `PRAGMA integrity_check` + probes; tar listing + sampled extraction; Parquet footer + row count) |
| `assertion_error` | string | Assertion detail; omitted when none |

### Verification statuses, paths, and modes

`status` values: `pass`, `fail` (legacy umbrella), `pending`, `unknown`,
`conflict` (the two paths disagreed), `restore_error` (a path could not
produce plaintext), `checksum_error` (paths agreed but not with the expected
digest), `assertion_error` (digests matched but the artifact assertion
failed).

`path` values: `armor` (S3 GET through a live ARMOR instance), `direct`
(MEK unwrap + raw B2 fetch + decrypt, honoring the ADR-003 headerless
multipart layout + HMAC sidecar), `dual_match` (both ran and agreed — the
only passing path for a dual run).

Trigger modes (`POST /trigger?mode=`): `dual` (default) exercises both paths
and asserts agreement; `dr-drill` exercises **only** the direct path — the
"ARMOR server is gone" scenario (ADR-004 decision 2). An unknown mode is a
400, never silently treated as `dual`.

### `GET /bucket?bucket=<name>`

Single-bucket form of `/status`. `400` when the `bucket` parameter is
missing, `404` when the bucket is not configured, otherwise the same
`BucketState` JSON.

### `POST /trigger` [?mode=dual|dr-drill]

`202 Accepted` with a one-line confirmation; the run starts in the background
under the per-run deadline (`VERIFIER_RUN_TIMEOUT`, default 2h — the same
deadline applies to scheduled and triggered runs). `405` for non-POST, `400`
for an unknown mode.

### `GET /healthz` — verifier liveness-with-verdict

`200 OK` ("OK") iff **every** configured bucket simultaneously has zero
failed objects and a `last_verification` within the last 24h. Any failed
object, or any bucket never verified / stale beyond 24h, yields `503`
("Unhealthy"). With zero configured buckets the endpoint reports healthy
(there is nothing to fail) — that vacuous case is not a deployment shape.

### `GET /readyz` — has anything ever verified

`200 Ready` iff at least one configured bucket has a non-zero
`last_success`; `503 Not ready` otherwise (fresh start, or every run so far
has failed before any object verified).

## Restore-verifier metric series

Emitted on the restore-verifier's own `/metrics`, same `armor_` prefix. The
restore-verifier family is exactly the series below; the contract test in
`internal/metrics` pins the TYPE lines.

Per-run bookkeeping (labels: `bucket`):

| Series | Type | Value contract |
|---|---|---|
| `armor_restore_verifier_checks_total` | counter | One per completed bucket run (dual runs only; drills never bump dual-path counters) |
| `armor_restore_verifier_failures_total` | counter | Bucket runs that ended with ≥1 failed object |
| `armor_restore_verifier_objects_verified` | counter | Objects verified (incremented per successful bucket run) |
| `armor_restore_verifier_objects_failed` | counter | Objects that failed (incremented per failed bucket run) |
| `armor_restore_verifier_latency_millis` | gauge | Wall-clock duration of the last bucket run, in ms |

Restorability signal set (ADR-004 decision 6 — the series the alert rules
consume; labels: `bucket`):

| Series | Type | Value contract |
|---|---|---|
| `armor_last_verified_restore_timestamp` | gauge | Unix seconds of the most recent successful restore per bucket. **A bucket with no successful restore exports `0`**, so restore-age alerting is immediately eligible — there is no missing-series blind spot for a never-succeeded bucket |
| `armor_verified_object_ratio` | float gauge | `verified/total` of the latest run, clamped to `[0,1]`; NaN/±Inf collapse to 0 |
| `armor_restore_verification_failures_total` | counter | Cumulative failed-object count per bucket. **Monotone**: a caller reporting an older snapshot never decreases the published value (the Prometheus counter contract survives overlapping runs) |

Direct-only DR-drill gauges (labels: `bucket`; deliberately distinct series
so a drill never perturbs dual-path alerting):

| Series | Type | Value contract |
|---|---|---|
| `armor_drill_last_verified_timestamp` | gauge | Unix seconds of the most recent drill **attempt** (success or failure) — advances every run, including runs that fail before recovering anything |
| `armor_drill_last_success_timestamp` | gauge | Unix seconds of the most recent drill that actually proved recovery; **`0` until a drill has ever succeeded** |
| `armor_drill_verified_object_ratio` | float gauge | `recovered/total` of the latest drill, in `[0,1]` |
| `armor_drill_failures_total` | counter | Cumulative drill failure count |

Isolation contract: a drill run writes only `armor_drill_*` series; a dual
run writes only the restorability set above. Neither touches the other, so a
successful drill during an ARMOR outage is visible progress without
suppressing the dual-path restore-age alert.

## Freshness windows

Four distinct windows govern this surface. They are not interchangeable:

| Window | Value | Where it applies |
|---|---|---|
| Restore-age **alert** threshold | 12h (~2× the default 6h `VERIFIER_CHECK_INTERVAL`) | `ArmorRestoreVerificationStale` fires when `time() - armor_last_verified_restore_timestamp` exceeds it |
| Escalation **staleness** window | 24h (`VERIFIER_FRESHNESS_WINDOW` / `--escalation-freshness-window`) | The Escalator files at most one staleness bead per bucket per window |
| Verifier `/healthz` staleness | 24h (hardcoded) | A bucket whose `last_verification` is older, or zero, makes `/healthz` report 503 |
| Per-run deadline | 2h (`VERIFIER_RUN_TIMEOUT`, `DefaultRunTimeout`) | Bounds one verification or drill run (discovery + restores); a wedged run fails loudly instead of blocking the loop |

Canary cadences (not staleness windows, but they set the expected freshness
of every canary signal): small-object check every 5m, multipart check every
1h, secondary-backend check every 5m — all three run an immediate check at
startup, so `status=unknown` persists only if the monitor is disabled or the
first check has not completed.

## Alert rules

Shipped per cluster in `declarative-config` as
`restore-verifier-monitoring.yaml.disabled` (`.disabled` because the ARMOR
clusters run no Prometheus Operator CRDs; drop the suffix once a Prometheus
scrapes `/metrics`). Every cluster that ships the file carries the same rule
set (enumerate the current copies with `find declarative-config/k8s -name
restore-verifier-monitoring.yaml.disabled` rather than trusting a count
here); scrapes run at a 30s interval against the restore-verifier `metrics`
port and the ARMOR `admin-api` port.

The rule set is pinned in-repo: `internal/metrics/alert_rules_contract_test.go`
parses `internal/metrics/testdata/restore-verifier-monitoring.yaml` — a
verbatim copy of the shipped manifest — and asserts the alert table below,
each rule's expression/hold/severity/labels, the two ServiceMonitor scrape
targets, and that every series an expression references is one this repo's
metrics package actually exports. A change to a shipped rule must update the
fixture, the test's expected table, and this document in the same change.

| Alert | Expression | `for` | Intent |
|---|---|---|---|
| `ArmorRestoreVerificationStale` | `time() - armor_last_verified_restore_timestamp > 12 * 3600` | 10m | No successful restore registered in 12h — verifier down, cannot reach B2, or every run fails |
| `ArmorRestoreVerificationFailures` | `sum by (bucket) (rate(armor_restore_verification_failures_total[1h])) > 0` | 5m | New sampled-object failures — backups exist but are not restorable |
| `ArmorRestoreVerificationLowObjectRatio` | `armor_verified_object_ratio < 0.95` | 5m | <95% of the latest sample passed. A zero-object discovery result is inconclusive per ADR-014 — confirm the sample before remediating |
| `ArmorRestoreVerificationDualPathDivergence` | `sum by (bucket) (increase(armor_restore_verification_failures_total[1h])) > 0` | 5m | Dual-path failure framing of the same counter — the runbook is to compare the ARMOR and direct path evidence (a `conflict` localizes the fault to the serving path vs. the stored data) |
| `ArmorMultipartCanaryUnhealthy` | `armor_multipart_canary_healthy == 0` | 10m | Multipart canary failing while the small-object canary may still be green (ADR-002's exact blind spot, now instrumented) |

All five carry `severity: critical`; the restore-verifier rules label
`component: restore-verifier`, the canary rule `component: armor-canary`.
`ArmorMultipartCanaryUnhealthy` fires against ARMOR's admin metrics; the
other four against the restore-verifier's — the component label and the two
ServiceMonitors encode which is which.

`ArmorMultipartCanaryUnhealthy` failure semantics, end to end: the multipart
canary retries each cycle up to `maxRetries` (default 3) with `retryDelay`
(default 10s) between attempts; only after the budget is exhausted does the
status flip to `unhealthy` and the gauge to 0, so one transient B2 error
never pages. The gauge reads 0 also on a freshly starting pod, before the
first multipart check completes — the monitor runs one immediately at
startup, and the 10m `for` absorbs that window; a pod whose startup check
cannot finish within ~10m will page anyway, which is the intended bias
toward loudness given the 1h cadence of subsequent checks. Acknowledge a
firing instance by reading `/armor/canary` (`multipart_healthy_status`,
`multipart_last_error`, `multipart_consecutive_fails`) rather than the gauge
alone — the endpoint carries the failure detail the gauge cannot.

Escalation from an alert follows ADR-004 §5 (next section): one bead per
distinct active failure, never a retry loop.

## Escalation and deduplication (ADR-004 §5)

Escalation is off by default (`VERIFIER_ESCALATION=false`); enabling it
requires the canonical bead CLI (bead-rs `bead`) reachable in the verifier's
image and a writable beads workspace (bead-rs discovers the workspace from
the filer's working directory — `--escalation-workspace`, default
`<escalation-state dir>/beads-workspace` on the same volume as the dedupe
state). Both prerequisites are validated at startup
(`restoreverifier.BeadRSFiler.ValidateStartup`: a PATH lookup plus
`bead --version`, a writability probe, `bead init` provisioning of a fresh
workspace, and a `bead list` end-to-end read); a failed validation logs the
exact remediation and runs with filing disabled rather than degrading the
verification loop itself.

**Failure escalation.** For every non-passing object verification the
Escalator files at most one bead, identified by the dedupe key:

```
{bucket, object key, path, failure class}
```

`failure class` ∈ `restore_error`, `checksum_error`, `assertion_error`,
`dual_path_conflict` (any unrecognized non-pass status maps to
`restore_error` so it still escalates exactly once). The same object failing
the same way on the same path is **one** bead across scheduler ticks and
process restarts — the dedupe set is persisted
(`VERIFIER_ESCALATION_STATE`, default
`/var/lib/restore-verifier/escalation-state.json`; mount a volume for
restart survival). A different failure class, or the other path, on the same
object is a separate key and a separate bead: they are different things to
fix.

**Re-arming.** When an object next verifies `pass`, every dedupe key for it
is cleared (`ClearObject`), so a genuine regression after recovery files a
fresh bead. Escalation therefore tracks *active* failures: exactly one bead
per active breakage, never an unbounded accumulation.

**Staleness escalation.** When a bucket's `last_success` is zero or older
than the freshness window (24h default), one staleness bead per bucket per
window — deduped per window, never per tick. Drill runs never file beads
(either kind), avoiding a second dedupe key per object.

**No retries.** A failed filing records nothing in the dedupe set, so the
next scheduler tick may make one further attempt — bounded by the schedule
cadence, never an unbounded loop, and no counter/attempt beads are ever
filed. (The 2026-07 NEEDLE retry-storms are the anti-pattern.) A filer call
is itself bounded (`--escalation-exec-timeout`, default 10s) so a hung CLI
cannot stall the run.

**Store-level idempotence.** Every filing also carries a `--unique-ref`
(`restore-verifier:<sha256>` of the dedupe identity; staleness refs anchor to
the bucket's freshness-window index instead), which the bead CLI binds
atomically: a repeat create prints `EXISTING <id>` instead of filing a second
bead. Even a lost `VERIFIER_ESCALATION_STATE` file therefore cannot produce
two beads for the same distinct failure — the persisted dedupe set remains
the first line of defense, the ref the second.

**Bead payload.** Every escalation bead carries: object key, bucket,
deployment (`ARMOR_DEPLOYMENT`), provenance/writer version where available
(envelope version from object metadata; writer ID once chain lookup is
wired), and both-path evidence — expected/ARMOR/direct SHA-256, both path
latencies, and the error string (truncated at 4096 chars). Titles are
capped to the beads schema's 500-char limit with the failure class and path
kept intact. Beads are filed via `bead create` as issue-type `bug`, priority
1 (critical in bead-rs semantics), with an optional `--escalation-label`.

The dedupe and staleness-window behavior is unit-tested in
`internal/restoreverifier/escalation_test.go`; this document defers to it as
the executable specification of that mechanism.

## Dual-path evidence — how to read a failure

`recent_results` in `/status` (and every escalation bead) carries the two
paths' digests. Reading them:

- `armor_sha256` empty, `direct_sha256` populated, `restore_error` — the
  ARMOR read path failed while the ciphertext itself decrypts: serving-path
  fault.
- `direct_sha256` empty, `armor_sha256` populated — the DR path failed:
  suspect envelope/sidecar layout drift (ADR-003) or escrowed-key problems.
  This is the tripwire ADR-004 deliberately arms: an envelope change that
  forgets the verifier surfaces here first.
- Both populated but ≠ `expected_sha256` (`checksum_error`) — both paths
  agree the stored data is wrong: corruption, not a serving fault.
- Both populated, ≠ each other (`conflict` / `dual_path_conflict`) — the two
  paths disagree: localize before remediating, it is either a range/routing
  bug in ARMOR or divergent stored state.

A passing dual run records `path: dual_match` with all three digests equal.
