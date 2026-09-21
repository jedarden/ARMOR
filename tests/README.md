# ARMOR test suites

Every suite under `tests/`: what it covers, the command that runs it, what it
needs, and whether CI runs it. The local gate is
[scripts/definition-of-done.sh](../scripts/definition-of-done.sh) (`make dod`
is the fast lane); CI is the `armor-build` WorkflowTemplate in iad-ci — its
manifest lives in `declarative-config` at
`k8s/iad-ci/argo-workflows/armor-workflowtemplate.yml`.

Use `python3 -m pytest`, not the `pytest` shim — the shim's shebang is stale
on NixOS hosts. The definition-of-done script makes the same choice.

## Suites

| Path | What it covers | Command | Prerequisites | Runs in CI? |
|---|---|---|---|---|
| Go unit suite (`internal/...`, `cmd/...`) | Server, crypto, backends, handlers, manifest, CLI | `go test ./... -short` (`make test` vets first) | Go toolchain only | Partially — the `go-test` step runs `ARMOR_RELEASE_RACE=1 ./scripts/release-gate.sh` (release-critical subsets, `-race`); the full `-short` suite is the local gate and also runs inside the `Dockerfile.test` image build (`make docker`) |
| `tests/integration/` | Real B2 + Cloudflare round-trips: put/get, ranges, listing, multipart (incl. the 5 GiB boundary), lifecycle, object lock, copy, admin keys, replication | Live: `go test -tags=integration ./tests/integration/...` — **credential-gated**: `ARMOR_INTEGRATION_TEST=1` plus B2/Cloudflare/auth variables, see [integration/README.md](integration/README.md). Compile-only (no credentials): `go test -tags=integration ./tests/integration/... -run '^$'` | Live run needs a reachable ARMOR server plus real B2/Cloudflare credentials | Compile + enumerate only — the `go-integration-test` step runs it under `-short`, which skips without a backend; live verification happens against canary/staging, never from iad-ci |
| `tests/aws-cli-compatibility/` | Real `aws` CLI and `rclone` round-trips, `aws-sdk-go-v2` transfer managers, and the `TestHarness_GET_*` direct-GET paths against the real in-process request pipeline | Full: `go test ./tests/aws-cli-compatibility/` (`make compat`). Without the CLIs: `go test -short ./tests/aws-cli-compatibility/` (runs `TestVerify_*` and harness tests only) | `aws` and `rclone` on `PATH` for the CLI legs (skipped cleanly when absent, and always under `-short`); optional `ARMOR_COMPAT_ENDPOINT` mode targets a live server — see [TEST_AGAINST_RUNNING_SERVER.md](aws-cli-compatibility/TEST_AGAINST_RUNNING_SERVER.md) | Yes — the `compat-suite-test` step installs aws CLI 2.17.0 + rclone v1.65.0 and runs the CLI/harness/verify gate against the freshly built image |
| `tests/docker-demo-smoke/` | Replays the README "Local demo (Docker only)" workflow verbatim against the pinned `ghcr.io/jedarden/armor:<VERSION>` image; also validates the tracked `compose.yaml` profiles | `make test-docker-demo` | Docker daemon (skips cleanly under `-short` or without one); `ARMOR_SMOKE_PULL=1` pulls the published tag instead of building, `ARMOR_SMOKE_IMAGE` overrides the reference | Never — needs a daemon; run by hand before changing the demo workflow or `compose.yaml` |
| `tests/rbac/` | RBAC verb coverage against B2 objects with the armor-test credentials (the ADR-012 allow/deny outcomes) | `go test -tags=integration ./tests/rbac/...` — **credential-gated**: needs a live ARMOR endpoint port-forwarded to `localhost:9000` (`kubectl --kubeconfig ~/.kube/iad-ci.kubeconfig -n armor-test port-forward svc/armor-test 9000:9000`) plus `ARMOR_TEST_ACCESS_KEY`/`ARMOR_TEST_SECRET_KEY` exported from OpenBao (`bao-as rs-manager bao kv get -field=AUTH_ACCESS_KEY secret/rs-manager/iad-ci/armor-test`, same for `-field=AUTH_SECRET_KEY`); the suite skips when the env is unset (armor-ad708bfd removed the literals that used to be compiled in) | Live server on `localhost:9000` | Never |
| `tests/performance/` | Deterministic harness shape tests (request counts, tail-block selectivity, one-PUT-per-part, overlap detector) plus opt-in throughput baselines and remote targets | Shape tests: `go test ./tests/performance -short`. Measurement: `ARMOR_PERF_RUN=1 go test ./tests/performance -run TestRecordedBaseline -v -timeout 60m` | `ARMOR_PERF_RUN=1` for measurement; remote targets opt-in via `ARMOR_PERF_ENDPOINT` and friends — procedure and output layout in [docs/performance/README.md](../docs/performance/README.md) | Shape tests ride along in any `-short` run; measurement never runs in CI |
| `tests/fixtures/` | Static data (corruption inventory, deployment windows, filtered objects) and the V1/V2/V3 migration golden fixtures consumed by the format-migration tests | Consumed by suites, not run directly; regenerate goldens with `tests/fixtures/migration/standalone_generator.go` — see [fixtures/README.md](fixtures/README.md) | — | n/a (inputs to other suites) |
| `tests/test_drift_check.py` | Unit tests for `scripts/drift_check.py`: the three deployment states, alert fingerprint/dedup, the live `/version` probe, CLI exit codes | `python3 -m pytest tests/test_drift_check.py -q` | python3 + pytest; network-free | Not in armor-build — the `armor-drift-check` CronWorkflow runs the drift script itself on a schedule, not its tests. Part of the local gate (both `--fast` and full) |
| `tests/test_publish_release.py` | Unit tests for `scripts/publish_release.py`: annotated tag + Forgejo/GitHub release publishing, idempotency, refusal to move a tag; every HTTP call is faked | `python3 -m pytest tests/test_publish_release.py -q` | python3 + pytest; network-free | Never — run by hand after touching the publisher (referenced from `scripts/README.md`) |
| `tests/__init__.py` | Package marker so pytest imports the two script-test files under stable names | — | — | — |

## The removed Python test-table framework

Until September 2026 this directory also carried a pytest "test-table"
framework — `test_helpers.py`, `test_tables.py`, ~25 root-level `test_*.py`
and `verify_*.py` files, `tests/error_tests/`, and the six guides
(`TEST_TABLE_GUIDE.md`, `TEST_TABLE_QUICKSTART.md`,
`TEST_TABLE_EXTENSION_GUIDE.md`, `TEST_TABLE_EXTENSION_SUMMARY.md`,
`README_JSON_VALIDATION_TESTS.md`, `validation_test_summary.md`) that
described its helper pattern. Nothing ran it, and it broke pytest collection
when its helpers drifted, so it was removed in commit `663f81c2`; the guides
are retrievable from git history, e.g.
`git show 663f81c2^:tests/TEST_TABLE_GUIDE.md`. The pytest targets in this
directory today are exactly the two `test_*.py` files in the table above.
