# ARMOR Release Process

How a change in this repository becomes a versioned artifact running on the
fleet, and how a correctness fix is proven to have reached every deployment.

1. [What a release is](#what-a-release-is)
2. [Cutting a release](#cutting-a-release)
3. [What CI does](#what-ci-does)
4. [Verifying a release](#verifying-a-release)
5. [Rolling the fleet forward](#rolling-the-fleet-forward)
6. [Correctness-fix propagation](#correctness-fix-propagation)
7. [Drift monitoring](#drift-monitoring)
8. [Troubleshooting](#troubleshooting)
9. [History](#history)

---

## What a release is

| Artifact | Where | Made by |
|---|---|---|
| `VERSION` (`0.1.<counter>`), the `compose.yaml` `ARMOR_VERSION` defaults and a `CHANGELOG.md` entry | this repo, one commit `release: armor <version>` | `scripts/cut-release.sh` |
| `ronaldraygun/armor:<version>` (server), `ronaldraygun/armor-restore-verifier:<version>`, `ronaldraygun/armor-fleet:<version>` | Docker Hub (private namespace) | CI |
| `ghcr.io/jedarden/armor:<version>` (server) | GHCR (public, anonymous-pull mirror; the only public image — `armor-restore-verifier` and `armor-fleet` stay private by operator decision, 2026-09-19) | CI |
| Annotated git tag `v<version>` at the release commit | Forgejo, mirrored to GitHub | CI |
| Release `ARMOR v<version>` with the CHANGELOG entry as body | Forgejo and GitHub | CI |

The version is a counter: the third component only increases and carries no
SemVer meaning, so every release may contain fixes and features. `CHANGELOG.md`
says what changed. Only semver tags are published; there is no floating tag to
pull — a `latest` tag visible on Docker Hub or GHCR is a stale leftover that
CI never pushes or updates (removing one needs registry package-delete scope
and is an operator action).

Two things never happen: CI never bumps `VERSION`, and nobody creates a `v*`
tag by hand. A hand-made tag desynchronizes the Forgejo and GitHub release
lists and the drift monitor, which is exactly what happened between
2026-09-01 and 2026-09-17 (twelve versions shipped without a tag or a release,
and `drift_check.py` could not see any of them).

## Cutting a release

Pre-conditions, on `main` in a checkout whose index is clean:

```bash
git pull --rebase origin main
scripts/definition-of-done.sh          # build, vet, script tests, short Go suite: must be green
```

Cut it:

```bash
scripts/cut-release.sh 0.1.1970        # or: make release V=0.1.1970
```

The script

- refuses a version that is not `MAJOR.MINOR.PATCH` or not newer than `VERSION`,
- refuses to run off `main` or with anything already staged (the release commit
  must contain nothing but `VERSION`, the `compose.yaml` `ARMOR_VERSION`
  defaults and `CHANGELOG.md`; other agents' unstaged
  edits in the shared checkout are left alone),
- rewrites every `${ARMOR_VERSION:-...}` default in `compose.yaml` to the new
  version, so the tracked demo and production compositions ship pinned to this
  release (`scripts/compose-version-parity.sh` — in the definition of done,
  the release gate and `make docker` — fails any drift left anywhere else, and
  `tests/docker-demo-smoke` re-checks it with a daemon),
- prepends a `## <version> (<date>)` entry to `CHANGELOG.md` listing every
  non-merge commit subject since the previous `v*` tag, minus bead-checkpoint
  and release commits (edit the entry afterwards if a subject needs rewording,
  then amend before pushing, or accept it as is),
- commits `release: armor <version>` and pushes to `origin main`.

`--dry-run` prints the entry without touching anything; `--no-push` commits
but leaves the push to you.

Which commit becomes the release is decided by CI as "the most recent commit
that changed `VERSION`", so push the release commit on its own or as the last
commit in a push. Pushing more commits after it before CI has run is
harmless: the tag still lands on the release commit.

## What CI does

Pushing a commit that changes `VERSION` reaches the `armor-build`
WorkflowTemplate in iad-ci (declarative-config
`k8s/iad-ci/argo-workflows/armor-workflowtemplate.yml`, triggered by
`k8s/iad-ci/argo-events/armor-sensor.yml`, which fires only for pushes whose
commit list touches `VERSION`). The run posts a `iad-ci/armor-build` commit
status on GitHub at start and at the end, which is what the README badge shows.

| Step | What it proves |
|---|---|
| `resolve-version` | The pushed commit changed `VERSION` and the value is `MAJOR.MINOR.PATCH`. A non-release push fails here by design |
| `lint` | `golangci-lint` clean |
| `test` | `scripts/release-gate.sh` with `ARMOR_RELEASE_RACE=1` (crypto, backend, restore-verifier, canary, config, cmd, handlers under `-race`) |
| `integration-test` | `tests/integration` compiles and runs in short mode — compile coverage only: every test in the suite skips without real-B2 credentials. Execution is the separate `armor-integration` leg (see [Live-B2 integration leg](#live-b2-integration-leg-armor-integration)) |
| `docker-build`, `docker-build-restore-verifier`, `docker-build-fleet`, `docker-build-ghcr` | The four images are built with kaniko from the pushed tree. They are siblings: the server image is published even if a companion build fails |
| `verify-*-image` | Each Docker Hub tag is resolvable through the registry API (the ghost-tag guard) |
| `compat-suite-test` | The freshly pushed server image serves AWS CLI and rclone end to end |
| `publish-release` | Runs `scripts/publish_release.py` (fetched from the released revision): creates the annotated tag `v<version>` at the pushed revision through the Forgejo API (an existing tag at another commit is a hard error, never moved), creates or refreshes the Forgejo release whose body carries the `CHANGELOG.md` entry for the version — fetched from the released revision through the same Forgejo raw API the step uses to fetch the publisher script — above the image digests observed in the registries, waits up to five minutes for the push mirror to carry the tag to GitHub, then creates or refreshes the GitHub release. Every action is idempotent, so a re-run of a partially published version completes it. Sibling of the step below: a release-API failure shows as a red workflow but never blocks the armor-test bump |
| `update-declarative-config` | Bumps the **armor-test** deployment (`k8s/iad-ci/armor-test/`) to the new version. Production deployments are never touched by CI |

The release body written by CI is the CHANGELOG entry for the version
followed by the digest table. A missing `CHANGELOG.md` or a missing section
for the version degrades to the digest table alone — logged by the publisher
and recorded as `"changelog": "unavailable"` in its JSON summary; any other
changelog fetch failure exits 5, so the workflow's public-Forgejo retry
covers it.

Watching a run (read-only):

```bash
kubectl --server=http://traefik-iad-ci:8001 get workflows -n argo-workflows \
  --sort-by=.metadata.creationTimestamp | grep armor-build | tail -5
kubectl --server=http://traefik-iad-ci:8001 get workflow <name> -n argo-workflows \
  -o jsonpath='{.status.phase} - {.status.message}'
```

Pods are deleted the moment a step finishes (`podGC: OnPodCompletion`);
completed-step logs are in VictoriaLogs for iad-ci
(`victorialogs-iad-ci-ts.ardenone.com:8444`) or the Argo UI within the TTL.

Submitting a build by hand (needs the write kubeconfig; the read-only proxy
cannot create). Pass the **full 40-character SHA** of the release commit as
`revision`, or the GitHub status posts fail silently:

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: armor-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: armor-build
  arguments:
    parameters:
      - name: git-repo
        value: jedarden/ARMOR
      - name: branch
        value: main
      - name: revision
        value: <full sha of the release commit>
EOF
```

## Verifying a release

Run this after the workflow succeeds. Docker Hub is private, so the manifest
check needs a logged-in Docker client; GHCR is public.

```bash
V=$(cat VERSION)
docker manifest inspect ronaldraygun/armor:$V >/dev/null && echo "hub armor ok"
docker manifest inspect ronaldraygun/armor-restore-verifier:$V >/dev/null && echo "hub restore-verifier ok"
docker manifest inspect ghcr.io/jedarden/armor:$V >/dev/null && echo "ghcr ok"
git fetch --tags origin && git tag --list "v$V"                       # tag on Forgejo
gh release view "v$V" -R jedarden/ARMOR --json isDraft,tagName        # GitHub release, isDraft must be false
awk "/^## $V /{f=1;next} /^## /{f=0} f" CHANGELOG.md | head           # the notes CI used
```

`gh release view` also matches drafts, so check `isDraft` rather than mere
existence.

### Live-B2 integration leg (`armor-integration`)

`armor-build` never executes `tests/integration`: the `integration-test`
step runs the suite in short mode without credentials, and every test in it
skips — which is exactly the decorative-gate failure ADR-002 recorded. The
execution leg is a separate WorkflowTemplate: `armor-integration`
(declarative-config
`k8s/iad-ci/argo-workflows/armor-integration-workflowtemplate.yml`). Each
run builds `./cmd/armor` from the revision under test, boots it against
the live iad-ci bucket scope isolated under `ARMOR_PREFIX=integration-tests/`
with its own `ARMOR_WRITER_ID` (so its manifest chain and every object it
writes are disjoint from the live deployment's keys in the same bucket),
and runs the suite with `-race` and **without** `-short` — omitting
`-short` is the point, since every test skips under it. Only
`TestMultipart5GB*` (a genuine 6 GiB upload, opt-in by the suite's own
design) is excluded by default via the `skip-tests` parameter; an empty
value runs the boundary pair too.

The leg runs nightly against current `main`
(`armor-integration-nightly` CronWorkflow). A red nightly flags a
live-surface regression the same day it lands; per release, confirm a run
has covered the release commit:

```bash
kubectl --server=http://traefik-iad-ci:8001 get workflows -n argo-workflows \
  | grep armor-integration | tail -5
kubectl --server=http://traefik-iad-ci:8001 get workflow <name> -n argo-workflows \
  -o jsonpath='{.status.phase} - {.status.message}'
```

If the newest green run predates the release commit, submit one for the
exact SHA by hand (needs the write kubeconfig):

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: armor-integration-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: armor-integration
  arguments:
    parameters:
      - name: branch
        value: main
      - name: revision
        value: <full sha of the release commit>
EOF
```

Credentials reach the run through the `armor-integration` ExternalSecret in
the same directory, which reads the live deployment's OpenBao paths
(`rs-manager/iad-ci/b2/iad-ci`, `rs-manager/iad-ci/armor`,
`rs-manager/iad-ci/armor/admin`); no values are stored in or passed through
this repository. The admin key-rotation tests rotate only the in-pod
server's ring — nothing shared with the live deployment.

## Rolling the fleet forward

Desired state lives only in `jedarden/declarative-config`; ArgoCD applies it.
Enumerate the current deployments from the manifests rather than from any list
in a document:

```bash
python3 scripts/find-armor-deployments.py ~/declarative-config
```

As of 2026-09-18 that is (ArgoCD application `<namespace>-ns-<cluster>`):

| Cluster | Manifests (`declarative-config/k8s/...`) |
|---|---|
| iad-ci | `iad-ci/armor/armor-deployment.yaml`, `iad-ci/armor/restore-verifier.yaml`; `iad-ci/armor-test/armor-test-deployment.yml` is bumped by CI |
| iad-kalshi | `iad-kalshi/armor/armor-deployment.yml`, `iad-kalshi/armor/restore-verifier.yaml` |
| rs-manager | `rs-manager/armor/armor-deployment.yml`, `rs-manager/armor/restore-verifier.yaml`, `rs-manager/armor/restore-verifier-acb-deployment.yml` |
| ord-devimprint | `ord-devimprint/devimprint/armor-deployment.yml`, `ord-devimprint/devimprint/restore-verifier.yaml` |
| ardenone-cluster | `ardenone-cluster/tradegraph-platform/armor/armor-deployment.yaml`; `ardenone-cluster/commitgraph-dashboard/parquet-mirror-deployment.yml` consumes the image |
| apexalgo-iad | `apexalgo-iad/needle-observability/armor-ledger.yml` |

To roll out:

1. Confirm a green `armor-integration` run covering the release commit (see
   [Verifying a release](#verifying-a-release)): the nightly exercises the
   S3 surface against real B2, which no other gate executes, and a red run
   against the release SHA is a fleet-roll blocker. Submit the manual
   per-release run if the nightly has not covered the SHA yet.
2. Edit the image tag(s) in the manifest(s): `ronaldraygun/armor:<version>`
   and, in the same change, `ronaldraygun/armor-restore-verifier:<version>`.
   Keep the digest pin form used in that file if it has one.
3. Commit with a message that names the version and the reason, push to
   declarative-config `origin` (Forgejo). ArgoCD syncs within minutes; a failed
   sync attempt is not retried for that revision, so check the Application if
   nothing has landed after ~20 minutes.
4. Verify each deployment:

   ```bash
   kubectl --server=http://traefik-<cluster>:8001 get pods -n <namespace> -l app=armor \
     -o jsonpath='{.items[*].spec.containers[0].image}'
   kubectl --server=http://traefik-<cluster>:8001 exec -n <namespace> deployment/armor -- armor check
   kubectl --server=http://traefik-<cluster>:8001 exec -n <namespace> deployment/armor -- \
     wget -qO- http://127.0.0.1:9001/version
   ```

   `armor check` exits 0 when config, backend, Cloudflare path and MEK all
   pass; 1 is a configuration error; 2 is connectivity or MEK failure.

Never change a running deployment with `kubectl` (`set image`, `rollout
restart`, `patch`, `scale`, `delete`): selfHeal reverts it and the change is
invisible to the next person. The only sanctioned write is the commit.

Record the declarative-config commit SHA, the clusters changed and the
verification output on the release bead before closing it.

## Correctness-fix propagation

A **correctness fix** addresses data corruption, encryption or decryption,
`x-amz-meta-armor-*` metadata handling, multipart integrity, authentication or
authorization, a race that causes inconsistency, or a CVE-level
confidentiality issue. UI, logging, metrics, performance-only and
documentation changes are not.

> A correctness fix is not resolved until every known ARMOR deployment is
> patched or explicitly tracked as pending. Merging to `main` is necessary,
> not sufficient.

Before closing the bead for a correctness fix:

1. Cut and verify the release (sections above).
2. Enumerate every deployment with `find-armor-deployments.py`.
3. Roll each one forward, or file a follow-up bead per deployment that cannot
   move yet (`bead create --title "Propagate ARMOR <version> to <cluster>" --priority 1 --issue-type task`)
   and link it with `bead dep`.
4. Verify each patched deployment (`armor check`, `/version`,
   `/armor/canary` healthy).
5. Put a propagation table on the closing bead:

   ```markdown
   | Cluster | Namespace | Previous | New | Status | Verified at |
   |---|---|---|---|---|---|
   | iad-ci | armor | 0.1.1963 | 0.1.1969 | Synced, canary healthy | 2026-09-14T12:30Z |
   | rs-manager | armor | 0.1.1963 | 0.1.1969 | Pending, bead armor-xxxx | - |
   ```

Timelines: critical correctness (corruption, encryption, security) within
24 h on all deployments; other correctness fixes within one week; everything
else at the next convenient rollout.

## Drift monitoring

`scripts/drift_check.py` classifies every deployment as `current`, `stale`,
`mismatched` or `unavailable` against the newest release tag and can file one
deduplicated alert bead. It runs daily in iad-ci
(`declarative-config/k8s/iad-ci/argo-workflows/armor-drift-check-*.yml`) and
on demand:

```bash
python3 scripts/drift_check.py                 # human report
python3 scripts/drift_check.py --json          # machine-readable
python3 scripts/drift_check.py --latest-tag v0.1.1970   # assert the latest when tags lag
```

The release list comes from the local checkout's `v*` tags when it has any
(`git fetch --tags` first) and from the GitHub tags API otherwise, which is
why CI must tag every release: an untagged release is invisible to the monitor.
Details: [drift-check.md](drift-check.md).

## Troubleshooting

- **`resolve-version` failed with "VERSION must change in the release commit".**
  The build ran for a push that did not change `VERSION`. Before 2026-09-18 the
  sensor's version filter was written as `filters.expr`, which is not a Sensor
  field, so every push built and failed here, turning the README badge red for
  non-release commits. The filter is now a gjson data filter on
  `body.commits.#(modified.#(=="VERSION")).id`. If it fires wrongly again, check
  the live Sensor's `spec.dependencies[0].filters` against the manifest.
- **The GitHub status stays `pending` or never appears.** The status posts wait
  up to two minutes for the Forgejo→GitHub mirror and need a full-length SHA.
  A manual submission without `revision` posts against `main`, which GitHub
  rejects. Re-run with the full SHA.
- **A version has images but no tag or release.** Re-run the workflow for that
  release commit (manual submission above), or publish by hand with the same
  idempotent script CI uses; credentials come from the environment only, never
  from arguments:

  ```bash
  FORGEJO_TOKEN="$(git credential fill <<< $'protocol=https\nhost=git.ardenone.com\n' | grep password | cut -d= -f2)" \
  GITHUB_TOKEN="$(gh auth token)" \
    python3 scripts/publish_release.py --version 0.1.1969 --commit <full sha of the release commit>
  ```

  `--dry-run` prints the planned mutations; `--tag-only` and `--no-github`
  narrow the scope. Exit 3 means the tag already exists at a different commit:
  stop and look, the script never moves a tag.
- **A version was cut but the build failed and never published an image.**
  Cut the next version; note in its CHANGELOG entry that it carries the
  unpublished one's changes (see 0.1.1969 and 0.1.1960 in `CHANGELOG.md`). Do
  not tag the unpublished version.
- **Ghost tag** (`verify-*-image` failed although kaniko exited 0): the
  registry never received the push. Re-run; if it persists, check the
  `docker-hub-registry` / `ghcr-jedarden-registry` secrets in iad-ci.
- **A release must be marked superseded** (a defect found after publishing):
  do not delete it. Edit the Forgejo and GitHub release to prerelease with a
  body starting "Superseded by v<next>: <why>", cut the next version, and add
  the same sentence to the CHANGELOG entry (see 0.1.1953–0.1.1955).

## History

Fleet-wide image tag changes recorded before this document became the
procedure above:

| Date | Tag | declarative-config commit | Clusters | Status |
|------|-----|---------------------------|----------|--------|
| 2026-08-13 | 0.1.1911 | `fe3e839e` | iad-ci, iad-kalshi, ord-devimprint, rs-manager (+ commitgraph consumer) | Synced and verified |
| 2026-08-10 | 0.1.1906 | `b5169cce` | iad-ci, iad-kalshi, ord-devimprint, rs-manager | Synced and verified |

On 2026-08-28 the sensor was first given a VERSION filter (declarative-config
`db840e7`) after every push produced a failing build; that filter used a field
the Sensor CRD does not have, which is why the symptom persisted until
2026-09-18.
