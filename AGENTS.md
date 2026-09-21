# ARMOR — Agent Guide

This is the canonical guide for any agent (or person) working in this
repository. It is deliberately short; every command in it is real and run in
CI or the local gate. The org-wide rules in the home-directory `CLAUDE.md`
still apply and nothing here overrides them.

## What ARMOR is

An S3-compatible proxy (Go) that encrypts objects before storing them in
Backblaze B2 and serves reads through Cloudflare for zero-egress cost. Any S3
client works unmodified. Full product documentation starts at
[README.md](README.md); design and operations docs are indexed in
[docs/README.md](docs/README.md) (that index is enforced by a test, see below).

## Repository map

| Path | What it is |
|---|---|
| `cmd/armor` | The server binary and its subcommands (`serve`, `demo`, `check`, `decrypt`, `verify`, `migrate`, `client-config`, `version`, `help`) |
| `cmd/restore-verifier`, `cmd/armor-fleet` | Companion binaries (restore verification harness, fleet console). The old offline verifier `cmd/verify-objects` was deleted 2026-09-20 — stale against fingerprinted DEKs and v3, and fully folded into `armor verify` (armor-da67956d), which now unwraps fingerprinted DEKs with ring fallback, verifies v3 multipart objects via manifest + sidecar, emits one report row per object, and exits non-zero on any failure |
| `internal/` | All packages: `server` (S3 + admin handlers), `crypto`, `backend`, `config`, `keymanager`, `manifest`, `acl`, `canary`, `dashboard`, `presign`, `provenance`, `replication`, `restoreverifier`, `metrics`, `logging`, `b2keys`, `docsindex`, `version`, `testutil` |
| `tests/` | Go test suites outside the package tree: `integration/` (real B2, build-tagged), `aws-cli-compatibility/`, `docker-demo-smoke/`, `fixtures/`; plus `test_drift_check.py` for the drift tooling |
| `scripts/` | Operator tooling: `definition-of-done.sh`, `release-gate.sh`, `cut-release.sh`, drift check, starvation watch. See `scripts/README.md` |
| `docs/` | ADRs, runbooks, notes, plan. `docs/plan/plan.md` is the architecture and phase record |
| `config/drift-config.json` | Fleet drift-check configuration |
| `Dockerfile`, `Dockerfile.test` | The published image (multi-stage; the final stage MUST stay the armor server) and the test image |
| `VERSION`, `CHANGELOG.md` | The release counter and its notes; only `scripts/cut-release.sh` changes them |
| `.beads/` | bead-rs work tracking. Never hand-edit |

Deployment manifests do **not** live here. They live in
`jedarden/declarative-config` (Forgejo) and ArgoCD applies them; see
"Deployments" below.

## Build, test, and the gates

```bash
go build ./...                       # must pass on every commit
go vet ./...
go test ./... -short                 # unit suite; integration tests skip without credentials
scripts/definition-of-done.sh --fast # build + vet + python script tests (the local gate)
scripts/definition-of-done.sh        # --fast plus `go test ./... -short`
scripts/release-gate.sh              # the gate CI and the Dockerfile run (crypto, backend, canary, handlers, config, cmd)
make build                           # every cmd/ binary into bin/ with the version injected
make help                            # the rest of the targets
```

- **go.mod is the single source of the Go toolchain version** (its `toolchain`
  directive). `Dockerfile` and `Dockerfile.test` pin `golang:<that
  version>-alpine` and must be bumped in the same change; CI builds with
  `GOTOOLCHAIN=local` on images at least that new. A local `go` newer than the
  directive is fine — `make build` runs `toolchain-check`, which warns (never
  fails) on a mismatch.
- The definition of done for any code change is `scripts/definition-of-done.sh`
  green. CI (`armor-build` in iad-ci) additionally runs golangci-lint, the race
  release gate, an integration-suite compile, the Docker builds, a registry
  existence check for every image, and an AWS CLI / rclone compatibility suite
  against the freshly built image.
- Two tests guard documentation: `internal/docsindex` fails when a file under
  `docs/` is missing from `docs/README.md` (or linked twice, or a link is
  broken), and when an `ARMOR_*` variable read by `internal/config` is absent
  from `README.md`. Add the doc to the index and the variable to the README
  configuration table in the same change.
- Python: `tests/test_drift_check.py` (run by the definition of done) and
  `tests/test_publish_release.py`, both via `python3 -m pytest` (the `pytest`
  shim has a stale shebang on NixOS hosts).
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
   `VERSION` and a `CHANGELOG.md` entry generated from the commits since the
   previous tag, commits `release: armor <version>`, and pushes.
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
- Secrets travel by reference. Never put a credential value in a file, commit,
  bead, doc, log, chat message or command line. OpenBao paths and
  `ExternalSecret` references are how values reach a cluster.
- No per-worker git worktrees or disposable clones to "isolate" work on this
  repo; one shared checkout, bead-level serialization.
