# ARMOR — Agent Guide

This is the canonical guide for any agent (or person) working in this
repository. It is deliberately short; every command in it is real and run in
CI or the local gate. The org-wide rules in the home-directory `CLAUDE.md`
still apply and nothing here overrides them.

## What ARMOR is

An S3-compatible proxy (Go) that encrypts objects before storing them in
Backblaze B2 and serves reads through Cloudflare for zero-egress cost. The
repository pins a scoped S3 operation matrix (auth, reads, range reads,
listing, overwrite, delete, single-PUT and documented multipart cases) in the
conformance suite under `tests/aws-cli-compatibility/`; this is not a blanket
full-S3 or fleet-release claim. Full product documentation starts at
[README.md](README.md); design and operations docs are indexed in
[docs/README.md](docs/README.md) (that index is enforced by a test, see below).
The release-image boundary and known limitations are maintained in
[docs/release-status.md](docs/release-status.md).

## Repository map

| Path | What it is |
|---|---|
| `cmd/armor` | The server binary and its subcommands (`serve`, `demo`, `check`, `decrypt`, `verify`, `migrate`, `client-config`, `version`, `help`) |
| `cmd/restore-verifier`, `cmd/armor-fleet` | Companion binaries (restore verification harness, fleet console). The old offline verifier `cmd/verify-objects` was deleted 2026-09-20 — stale against fingerprinted DEKs and v3, and fully folded into `armor verify` (armor-da67956d). The current source has focused coverage for ring fallback and v3 manifest/sidecar verification; release and live DR evidence remains bounded by [docs/release-status.md](docs/release-status.md) |
| `internal/` | All packages: `server` (S3 + admin handlers), `crypto`, `backend`, `config`, `keymanager`, `manifest`, `acl`, `canary`, `dashboard`, `presign`, `provenance`, `replication`, `restoreverifier`, `metrics`, `logging`, `b2keys`, `docsindex`, `version`, `testutil` |
| `tests/` | Go test suites outside the package tree: `integration/` (real B2, build-tagged), `aws-cli-compatibility/`, `docker-demo-smoke/`, `fixtures/`; plus the Python gate suites, enumerated in `tests/gate-suites.txt` — the single edit point for the gate inventory, held to every documented surface by `tests/test_gate_inventory.py` (per-suite detail in the `tests/README.md` Suites table) |
| `scripts/` | Operator tooling: `definition-of-done.sh`, `release-gate.sh`, `cut-release.sh`, drift check, starvation watch. See `scripts/README.md` |
| `docs/` | ADRs, runbooks, notes, plan. `docs/plan/plan.md` is the architecture and phase record |
| `config/drift-config.json` | Fleet drift-check configuration |
| `Dockerfile`, `Dockerfile.test` | The published image (multi-stage; the final stage MUST stay the armor server — enforced by `scripts/image-contract-gate.sh`) and the test image |
| `VERSION`, `CHANGELOG.md`, `compose.yaml` | The release counter, its notes, and the Compose image pin; only `scripts/cut-release.sh` changes them |
| `.beads/` | bead-rs work tracking. Never hand-edit |

Deployment manifests do **not** live here. They live in
`jedarden/declarative-config` (Forgejo) and ArgoCD applies them; see
"Deployments" below.

## Making a change

Four recipes cover the common change shapes. Each names every file the change
must touch and the test that fails when one is missed. Run the named test in
the same change as the code; run `scripts/definition-of-done.sh` before
pushing.

### Adding an S3 operation

1. Dispatch: add the branch in `HandleRoot`
   (`internal/server/handlers/handlers.go`), keyed off method + query
   parameter the way the existing `?uploads` / `?retention` branches are,
   calling a handler method on `*Handlers` in that package.
2. Classification: mirror the branch in `s3OperationForRequest`
   (`internal/server/handlers/s3_operation.go`) so `armor_errors_total`
   carries the new operation's name, and add a case to
   `TestS3OperationForRequest` (`s3_operation_test.go`). That classifier's
   documented contract is to stay in sync with `HandleRoot`.
3. Error codes: every `<Code>` the operation's `writeError` calls can emit
   gets a row in the *Error Codes by HTTP Status* table in
   [docs/error-responses.md](docs/error-responses.md), inside the
   `handlers-error-codes` markers. `internal/docsindex` diffs that table
   against the package source, so an unlisted code fails the gate.
4. Compatibility: pin the wire behavior in
   `tests/aws-cli-compatibility/protocol_conformance_test.go` (runs even
   under `-short`); add real-client coverage in the per-binary legs when a
   client exercises the operation differently, and bind the operation in
   `client_matrix_test.go` — that test holds the client matrix and this
   file's CI claim to the legs that actually exist.
5. Verify: `go build ./... && go test ./internal/server/handlers/ ./internal/docsindex/ ./tests/aws-cli-compatibility/ -short`.

### Adding a configuration variable

1. Add the field to `Config` and read it in `Load()`
   (`internal/config/config.go`) with `getEnv("ARMOR_NAME", default)`;
   validate it there — append to `errs`, never validate in a consumer.
2. Add the row to the matching table in
   [docs/configuration.md](docs/configuration.md) in the same change.
   `TestConfigReferenceParity` (`internal/docsindex`) fails when the row is
   missing, when its Default disagrees with what the code declares, or when
   it omits a bound or accepted value the validation enforces. A variable
   consumed outside `internal/config` (companion binaries, handlers) is
   additionally held by `TestConsumerReferenceParity`.
3. Verify: `go build ./... && go test ./internal/docsindex/ -short` (the
   parity tests live there, not in `internal/config`).

### Adding an admin route

1. Write the handler as a method on `*Server` in `internal/server/`
   (`manifest_admin.go` is a small example of the shape) and register it in
   the admin mux in `internal/server/server.go`, beside the other
   `mux.HandleFunc("/admin/...", ...)` lines.
2. The token gate is automatic and fail-closed: every admin-mux path outside
   `adminPublicExact` / `adminPublicPrefixes` (`admin_auth.go`) requires the
   `ARMOR_ADMIN_TOKEN` bearer token and returns 403 when unset. Do not add
   the new route to those maps — the exemptions exist for probe and scrape
   paths only.
3. Docs: add the `<!-- endpoint: /admin/<name> -->` marker and a section in
   [docs/http-api-reference.md](docs/http-api-reference.md);
   `TestHTTPAPIReferenceCoversRegisteredRoutes` (`internal/server`) fails on
   any registered route missing its marker. Add a row to the Admin Endpoints
   summary in [docs/error-responses.md](docs/error-responses.md) when the
   route's error behavior is worth pinning.
4. Verify: `go test ./internal/server/ -run TestHTTPAPIReferenceCoversRegisteredRoutes -short`.

### Regenerating migration goldens

1. Generate into a scratch directory, never directly into the tree:
   `cd tests/fixtures/migration && go run standalone_generator.go /tmp/armor-fixtures`.
2. Copy in only the fixture set you intend to replace (normally
   `generated_fixtures/`) and commit it — a full run also emits `malformed/`
   and `edge_cases/`, which are not part of the committed tree. Never
   hand-edit a fixture: the committed tree must stay byte-identical to
   generator output.
3. Verify: `go test ./tests/fixtures/migration/ -short` (the generator gates,
   including `TestCommittedRegenerationSetMatchesGenerator`, which fails on
   drift) and `go test ./internal/server/ -run TestFormatMigration -short`
   (the in-package consumers that read the fixture tree).

### Test suites that need credentials or tools

Most of the suite skips by design when credentials or external binaries are
absent — `go test ./... -short` and the definition of done stay green without
any of these set. The gates:

| Variable(s) | What they unlock |
|---|---|
| `ARMOR_INTEGRATION_TEST=1` plus `ARMOR_B2_ACCESS_KEY_ID`, `ARMOR_B2_SECRET_ACCESS_KEY`, `ARMOR_B2_REGION`, `ARMOR_BUCKET`, `ARMOR_CF_DOMAIN`, `ARMOR_MEK`, `ARMOR_AUTH_ACCESS_KEY`, `ARMOR_AUTH_SECRET_KEY`, `ARMOR_ADMIN_TOKEN` | `tests/integration` — the real-B2 suite. Runs via the `armor-integration` WorkflowTemplate (nightly, per release), never in `armor-build` |
| `ARMOR_ENDPOINT` (with `ARMOR_ADMIN_ENDPOINT`, `ARMOR_ADMIN_TOKEN` for admin legs) | Live-server integration legs (`tests/integration/awscli_test.go`, admin key management) that skip when the endpoint is unset |
| `ARMOR_TEST_ACCESS_KEY`, `ARMOR_TEST_SECRET_KEY` | `tests/rbac` against the live armor-test instance (values from OpenBao `secret/rs-manager/iad-ci/armor-test`) |
| `ARMOR_TEST_MEK` | Large-object format-migration integration leg (`tests/integration`) — additionally requires `ARMOR_LARGE_OBJECT_TESTS=1` (explicit opt-in; the in-memory 7 GiB fixture OOMs a CI pod) |
| `ARMOR_LARGE_OBJECT_TESTS=1` | The `TestLargeObjectMigration*` pair in `tests/integration` (with `ARMOR_TEST_MEK` and the other credential variables) |
| `ARMOR_TEST_ROTATION=1` | Expensive key-rotation integration tests |
| `ARMOR_KEY_ROUTES` | Key-routing integration test |
| `ARMOR_COMPAT_ENDPOINT` | `tests/aws-cli-compatibility` endpoint mode against a running server — what the armor-build image gate uses; in that mode a missing client is fatal, not a skip |
| `ARMOR_PERF_RUN=1` (with `ARMOR_PERF_OUT`, `ARMOR_PERF_SIZES`, `ARMOR_PERF_WRITE_VERSION`) | `tests/performance` measurement matrix |
| `ARMOR_SMALL_OBJECT_BENCH=1` | Latency-injected small-object GET benchmark (`internal/server/handlers`) |
| `ARMOR_B2_TEST_ENDPOINT`, `ARMOR_B2_TEST_REGION`, `ARMOR_B2_TEST_KEY_ID`, `ARMOR_B2_TEST_SECRET`, `ARMOR_B2_TEST_BUCKET` | Live secondary-backend B2 test (`internal/backend`) |
| `ARMOR_SMOKE_IMAGE`, `ARMOR_SMOKE_PULL`, `ARMOR_SMOKE_VERSION` | Demo-smoke image selection (`tests/docker-demo-smoke`); defaults to the repo `VERSION` pin |

## Build, test, and the gates

```bash
go build ./...                       # must pass on every commit
go vet ./...
go test ./... -short                 # unit suite; integration tests skip without credentials
scripts/definition-of-done.sh --fast # hermetic local gate: build + vet + script tests + CLI contract (-short); no network, no browser; per-stage timings
scripts/definition-of-done.sh        # full gate: --fast plus `go test ./... -short` and per-page Agentation browser mounts (dashboard, demo, fleet); `make dod-full`
scripts/cli-contract-gate.sh         # executable CLI/companion contract parity gate
scripts/release-gate.sh              # the gate CI and the Dockerfile run (crypto, backend, canary, handlers, config, cmd, image contract, publisher contract tests)
make build                           # every cmd/ binary into bin/ with the version injected
make help                            # the rest of the targets
```

- **go.mod is the single source of the Go toolchain version** (its `toolchain`
  directive). `Dockerfile` and `Dockerfile.test` pin `golang:<that
  version>-alpine` and must be bumped in the same change; CI builds with
  `GOTOOLCHAIN=local` on images at least that new. The pin is enforced by
  `scripts/toolchain-parity.sh`, which fails on drift and runs in the
  definition of done, the release gate, and `make docker`. A local `go`
  newer than the directive is fine — `make build` runs `toolchain-check`,
  which warns (never fails) on a mismatch.
- The definition of done for any code change is `scripts/definition-of-done.sh`
  green. CI (`armor-build` in iad-ci) additionally runs golangci-lint, the race
  release gate, an integration-suite compile, the Docker builds, a registry
  existence check for every image, and the real-client compatibility suite
  (AWS CLI, rclone, boto3, DuckDB/httpfs, litestream, and barman-cloud)
  against the freshly built image — every leg of that matrix is pinned
  present by `tests/aws-cli-compatibility/client_matrix_test.go`, which runs
  on every `go test`, including that image gate. The real-B2 suite itself never executes in
  `armor-build` — without credentials every test in it skips — so it is run by
  the separate `armor-integration` WorkflowTemplate in declarative-config
  (`k8s/iad-ci/argo-workflows/armor-integration-workflowtemplate.yml`):
  nightly against `main` via the `armor-integration-nightly` CronWorkflow, and
  per release per [docs/release-process.md](docs/release-process.md), where a
  green run covering the release commit gates the fleet roll.
- Two tests guard documentation: `internal/docsindex` fails when a file under
  `docs/` is missing from `docs/README.md` (or linked twice, or a link is
  broken), and when an `ARMOR_*` variable read by `internal/config`
  (`TestConfigReferenceParity`) or consumed by the companion binaries and
  packages outside it (`TestConsumerReferenceParity`) lacks a row in
  `docs/configuration.md` — or its row's `Default` disagrees with the
  default the code declares, or the row omits a bound or accepted value the
  code's validation enforces. Add or update the
  configuration-reference row in the same change as the code.
- Python: the gate suites enumerated in `tests/gate-suites.txt` — the
  single edit point for the gate inventory; every listed suite runs in both
  the `--fast` and the full definition of done
  (`tests/test_gate_inventory.py` holds the inventory file, the script's
  pytest invocation, the AGENTS.md map row and this bullet, the script's
  own header, the `scripts/README.md` gate section, and the
  `tests/README.md` Suites table to each other; the publisher contract
  suite also runs in the release gate), via `python3 -m pytest`
  (the `pytest` shim has a stale shebang on NixOS hosts).
- Prohibited deployment constructs are machine-enforced:
  `scripts/prohibited-constructs-gate.sh` fails the tree on `.github/workflows/*`
  files, real `kind: Job` / `kind: CronJob` manifest lines, `:latest` image
  tags, and unpinned `ronaldraygun/*` image references — examples and notes
  included; comment lines that merely describe the rules are exempt. It runs
  in the definition of done and the release gate, and
  `tests/fixtures/prohibited-constructs/` holds one regression fixture per
  rule.
- The Dockerfile default image is machine-enforced the same way:
  `scripts/image-contract-gate.sh` fails the tree unless the LAST
  `Dockerfile` stage is the armor server (`ENTRYPOINT ["/armor"]`, no CMD)
  and the companion `--target` stages (`restore-verifier-runtime`,
  `armor-fleet-runtime`) stay present under those names with their
  entrypoints — an untargeted build publishes the last stage as
  `ronaldraygun/armor`, and images 0.1.1833–0.1.1870 shipped
  `/restore-verifier` there after a multi-stage reorder. The same gate pins
  one build-context contract: `.dockerignore` must not exclude `docs/` —
  the image-scope release gate's server leg reads
  `docs/http-api-reference.md` from the build context, and 0.1.1995
  published no image after that exclusion crept in (armor-81ab7826). It
  runs in the definition of done, the release gate (hence inside the
  Docker build itself) and `make docker`, with its own pytest suite
  (`tests/test_image_contract_gate.py`).
- Never commit build output (`bin/`, `*.test`) or caches; `.gitignore` covers
  them.

## Beads (work tracking)

This workspace uses **bead-rs** (`bead`), declared in `.needle.yaml`
(`bead_cli.backend: bead-rs`). Do not run `bf`/`br` here.

- Every repository change is covered by a bead. Search first
  (`bead list --status open`); create one before editing if none fits
  (`bead create --title ... --priority 2 --issue-type task --description ...`).
- `bead update <id> --notes "..."` **replaces** the notes; read `bead show <id>`
  first and carry the existing text forward.
- Close with `bead close <id> --reason "..."` and record on the bead: what
  changed, the paths, the verification commands and their outcomes, the
  commit(s), and for deployment changes the declarative-config commit and
  ArgoCD application.
- Every mutation auto-publishes `.beads/checkpoint/`; commit those files with
  your change (`git add .beads/checkpoint`) so the durable checkpoint stays
  current. `bead sync flush-only` is the idempotent manual flush.

## Commits

- Work on `main`. No branches, no PRs.
- Stage precise paths, never `git add -A`/`.`/`-a`. The checkout is shared
  with other agents whose in-flight edits must not ride along:
  `git add <paths> && git commit -m "..." -- <paths>`, then check `git log -1`.
- One commit per completion point, message `type(scope): subject (bead-id)`.
- Push to Forgejo `origin` only; GitHub is a read-only mirror. Never force-push.
- Git identity: `jedarden <github@jedarden.com>`.

## Releases

Read [docs/release-process.md](docs/release-process.md) before cutting one.
The short form:

1. `scripts/definition-of-done.sh` is green on `main`.
2. `scripts/cut-release.sh <version>` (or `make release V=<version>`) writes
   `VERSION`, bumps the `compose.yaml` `ARMOR_VERSION` defaults to match, and
   writes a `CHANGELOG.md` entry generated from the commits since the
   previous tag — one `release: armor <version>` commit, which it pushes.
   `scripts/compose-version-parity.sh` (definition of done, release gate,
   `make docker`) fails any tree where the defaults lag `VERSION`.
3. CI does everything else: builds and publishes `ronaldraygun/armor`,
   `ronaldraygun/armor-restore-verifier`, `ronaldraygun/armor-fleet` and the
   public mirror `ghcr.io/jedarden/armor` (the server image is the only
   public artifact; restore-verifier and fleet stay private by operator
   decision, 2026-09-19), verifies each tag exists in the
   registry, runs the compatibility suite, then runs
   `scripts/publish_release.py`, which creates the annotated git tag
   `v<version>`, the Forgejo release and the GitHub release (idempotent; the
   same script backfills a version by hand).
4. Roll the fleet forward by editing image tags in declarative-config.

Versions are `0.1.<counter>`: the third component only increases and carries
no SemVer meaning. Never bump `VERSION` without `cut-release.sh`, never create
a `v*` tag by hand, and never use `:latest`. Only semver tags are published;
there is no floating tag to pull — a `latest` visible in a registry is a stale
leftover, not something CI pushes.

## Deployments

- Live manifests: `declarative-config/k8s/<cluster>/<namespace>/armor-deployment.y*ml`
  and `restore-verifier*.y*ml`. Enumerate the current inventory with
  `python3 scripts/find-armor-deployments.py ~/declarative-config` rather than
  trusting any list in a document.
- The five primary ARMOR server Deployments in the release bump list are
  `iad-ci/armor`, `iad-ci/armor-test`, `iad-kalshi/armor`,
  `ord-devimprint/devimprint`, and `rs-manager/armor`. Auxiliary consumers of
  the ARMOR image (such as sidecars) are outside that list.
- Restore-verifier topology is **one Deployment per ARMOR bucket scope**
  (bucket + MEK + B2 credential set), usually co-located with that scope's
  ARMOR proxy and reusing its env sources — never one fleet-wide verifier
  (ADR-004, Addendum + "Fleet topology"). Five restore-verifier Deployments
  run today:
  `iad-ci/armor`, `iad-kalshi/armor`, `ord-devimprint/devimprint`,
  `rs-manager/armor/restore-verifier-acb`, and
  `ardenone-cluster/tradegraph-platform`.
  `tests/test_restore_verifier_inventory.py` pins the prose to the
  enumeration above with a golden inventory and compares it with
  declarative-config — change the golden, the primary ARMOR list, and every
  prose count in the same change as the fleet.
- Change desired state only by committing to declarative-config; ArgoCD syncs
  it (`<namespace>-ns-<cluster>` applications). `kubectl` mutations
  (`apply`, `patch`, `set image`, `rollout restart`, `delete`) are prohibited
  and are reverted by selfHeal anyway. Read-only `kubectl get/describe/logs`
  via `--server=http://traefik-<cluster>:8001` is fine, as is
  `kubectl exec ... -- armor check` for verification.
- Image references are pinned semver tags. Every release publishes matching
  `armor` and `armor-restore-verifier` tags; bump both.
- Version drift across the fleet: `python3 scripts/drift_check.py`
  (`docs/drift-check.md`).

## Hard rules (org-wide, repeated because they get broken)

- No `.github/workflows/*`; CI is Argo Workflows in iad-ci, templates in
  `declarative-config/k8s/iad-ci/argo-workflows/`.
- No `kind: Job` / `kind: CronJob` anywhere, including examples and notes.
- No `:latest` and no bare git SHAs for `ronaldraygun/*` images.
- The first three bullets are enforced by `scripts/prohibited-constructs-gate.sh`
  (definition of done + release gate); it also fails on unpinned `ronaldraygun/*`
  `image:` references. Regression fixtures:
  `tests/fixtures/prohibited-constructs/`.
- Secrets travel by reference. Never put a credential value in a file, commit,
  bead, doc, log, chat message or command line. OpenBao paths and
  `ExternalSecret` references are how values reach a cluster.
- No per-worker git worktrees or disposable clones to "isolate" work on this
  repo; one shared checkout, bead-level serialization.
