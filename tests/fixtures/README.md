# Test Fixtures

Static test data and regression fixtures for the ARMOR test suites. Nothing
here is generated at test time; the one generator lives next to the data it
produces and is run by hand (see `migration/` below).

## Layout

| Path | What it is |
|---|---|
| `migration/` | The V1/V2 format-migration fixture tree and its standalone generator — the committed data must stay byte-identical to generator output. See [migration/README.md](migration/README.md) for the format catalog, the independence guarantee, and the regeneration procedure |
| `prohibited-constructs/` | One regression fixture per `scripts/prohibited-constructs-gate.sh` rule: a `.github/workflows/*` path, `kind: Job`, `kind: CronJob`, a `:latest` tag, and an unpinned `ronaldraygun/*` reference. The gate suite runs the script against each fixture and expects it to fail |
| `corruption-inventory-raw.json`, `summary.json`, `filtered_objects.csv`, `deployment-windows.json`, `restore-config.yml` | Operational snapshots consumed by the archived scripts in `scripts/archive/` (unaudited-bucket enumeration, deployment-window parsing, affected-object filtering) — inputs and outputs of past fleet audits, kept as data, not as code |

## Rules

- Never hand-edit anything under `migration/`. Regenerate with the standalone
  generator into a scratch directory and commit the replaced set;
  `TestCommittedRegenerationSetMatchesGenerator` fails on drift between the
  committed tree and generator output.
- A new `prohibited-constructs/` fixture lands in the same change as the gate
  rule (or fixture shape) it exercises.
- There is no Python package here. S3 error behavior is pinned by the Go
  suites in `internal/server/` and documented in
  [docs/error-responses.md](../../docs/error-responses.md); the former
  `error_scenarios.py` helpers described an API ARMOR never had and were
  removed.
