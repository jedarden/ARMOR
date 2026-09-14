# Golden Fixture Validation Report — 2026-09-13

Bead: armor-ee6ede36 ("Run golden validation from clean clone"), child of the
armor-2c7152ae fixture umbrella.

## Method

`internal/server/format_migration_golden_test.go` (landed with this report)
runs every committed fixture under `tests/fixtures/migration` through ARMOR's
real decrypt, dry-run and migration paths, in-process against an in-memory
backend — no bucket, no network. Validation was executed from a clean clone of
this repository (`~/scratch/armor-golden-20260912`, at main `6650cbfe`), not
the shared working tree, so the results reflect committed state.

The fixtures were produced by `standalone_generator.go`, an independent
reimplementation of the V1/V2 crypto, so green results are adversarial
evidence that the production read/migration paths accept third-party-generated
data.

Run: `go test ./internal/server/ -run 'TestGolden' -v` — **ok**, 0 failures.

## Per-fixture results

| Fixture | Decrypt | Dry-run | Migrate | Notes |
|---|---|---|---|---|
| v1_single_put/explicit_version | ok | ok | ok → V3 | sha a7f6f6d8…, 94 B |
| v1_single_put/implicit_version | ok | ok | ok → V3 | same plaintext |
| v1_single_put/minimal_metadata | ok | ok | ok → V3 | same plaintext |
| v1_multipart/{uniform,non_uniform,variable_final} | SKIP | SKIP | SKIP | known fixture wrap defect (below) |
| v2_single_put/standard | SKIP | ok | SKIP | stale committed content (below); dry-run unaffected (decrypts cleanly, HMAC-valid) |
| v2_multipart/{uniform,non_uniform,variable_final} | SKIP | SKIP | SKIP | known fixture wrap defect |
| edge_cases, contradictory, malformed | ok | ok | ok | failure-outcome fixtures all fail closed with a recorded reason (HMAC verify, unwrap AIV, EOF, missing sidecar) |
| corruption suite (11 derived variants) | — | — | all fail-closed | object left byte-identical, never migrated to V3 |

All corruption variants that are HMAC-reachable fail closed correctly:
corrupted ciphertext / HMAC table / wrapped DEK, truncated ciphertext or
header, missing sidecar — exactly one recorded failure each, original bytes
untouched.

## Defects found and pinned

The harness follows a pin-not-skip convention: each defect has a dedicated
test that fails with removal instructions the moment the defect is fixed, and
the affected subtests skip only while the defect reproduces.

1. **Fixture wrap defect (fixture-side, pre-existing pin).** Committed
   multipart fixtures wrap the DEK with AES-GCM (60 bytes) while production
   `crypto.UnwrapDEK` implements AES-KWP / RFC 5649 (40 bytes). Pinned by
   `TestGoldenFixtureWrapDefect`. The current generator still emits GCM wraps,
   so a generator fix must precede any fixture regeneration.

2. **Stale committed content in v2_single_put/standard (fixture-side, new).**
   The committed ciphertext decrypts cleanly (HMAC-valid) to 94 bytes with
   sha256 `b8aeffe9…`, but the fixture's own records — envelope header,
   `object_metadata.json` and `metadata.json`, which agree with each other —
   document `a7f6f6d8…` (the same plaintext the three v1_single_put fixtures
   actually contain). The recorded length (94 B) matches. Conclusion: the
   encrypted content is the stale side (generator drift — the current
   generator produces a 119-byte plaintext and a GCM wrap, so the committed
   bytes predate the current generator). The production V2 read path is
   sound: fixtures sharing the recorded plaintext decrypt to exactly the
   recorded sha. Pinned by `TestGoldenFixtureV2StandardStaleDoc`; the
   fixture-construction lineage should regenerate this fixture.

3. **Migrator multipart output defect (production-side, pre-existing pin).**
   `uploadAsMultipart` discards per-part HMAC tables and never persists a
   sidecar, so migrating any valid object above the 5 MiB threshold replaces
   it with an unreadable body and then fails its own read-back verify — a
   data-loss shape. Pinned by `TestGoldenMultipartMigratorDefect`.

4. **Migration path does not enforce header plaintext SHA (production-side,
   new).** `EnvelopeHeader.VerifyPlaintextSHA` is enforced by `cmd_decrypt`,
   the canary and the restore verifier, but not by the migrator's decrypt or
   read-back verify. Corrupting the envelope-header IV (keystream scramble,
   invisible to the ciphertext HMAC) therefore migrates with
   `FailedObjects=0` to a V3 object whose plaintext is garbage. Pinned by
   `TestGoldenMigrationIVIntegrityGap`.

Also documented (not a defect): `x-amz-meta-armor-iv` object metadata is
advisory — the envelope header is authoritative — so corrupting the metadata
field is correctly ignored by decrypt and migration.

## Reproduction

```bash
git clone <origin> ~/scratch/armor-golden-$(date +%Y%m%d)
cd ~/scratch/armor-golden-$(date +%Y%m%d)
PATH="$HOME/sdk/go/bin:$PATH" go test ./internal/server/ -run 'TestGolden' -v
```

## Handoffs

- Fixture-construction lineage: regenerate the fixture set once the
  standalone generator emits KWP wraps (defects 1+2); the pins re-arm
  automatically.
- Multipart migration-path lineage: defects 3+4 are production code fixes
  (`uploadAsMultipart` HMAC sidecar; header plaintext-SHA enforcement in the
  migrator's decrypt/read-back verify).

## Delta 2026-09-14 (re-validation at current main)

Re-run of the same suite from a fresh clean clone (`~/scratch/armor-golden-20260914`)
at pushed origin/main `0fee67ec0`, per armor-9d9cdda9 (re-scope of armor-ee6ede36):

- `PATH="$HOME/sdk/go/bin:$PATH" go test ./internal/server/ -run 'TestGolden' -v
  -buildvcs=false` — **exit 0**, `ok github.com/jedarden/armor/internal/server`,
  0 failures, 20 subtest skips.
- Harness and fixtures byte-unchanged since the baseline commit `d5c78ffd0`:
  `git diff --stat d5c78ffd0..HEAD -- internal/server/format_migration_golden_test.go
  tests/fixtures/` is empty; only production code has moved on main.
- Per-fixture results: **identical to the baseline table**. v1_single_put ×3
  decrypt/dry-run/migrate ok → V3 with the same plaintext sha `a7f6f6d8…` (94 B);
  the six multipart fixtures and v2_single_put/standard SKIP in exactly the
  baseline slots (all three phases for multiparts; decrypt+migrate for
  v2_single_put/standard, whose dry-run stays ok); all four pin tests
  (`TestGoldenFixtureWrapDefect`, `TestGoldenFixtureV2StandardStaleDoc`,
  `TestGoldenMultipartMigratorDefect`, `TestGoldenMigrationIVIntegrityGap`)
  still pass, i.e. all four pinned defects still reproduce; the corruption
  suite still fails closed on every variant with the same recorded reasons
  (HMAC verify, unwrap AIV, envelope-header EOF, missing sidecar), objects
  left byte-untouched.
- Prose correction to the baseline, not a behavior delta: the baseline's
  corruption-suite row says "11 derived variants", but the harness table
  statically defines **10** corruption cases (5 single + 5 multipart) and the
  byte-identical baseline harness executed the same 10.
- No new failures; no new bead filed. Ownership of the four pinned defects is
  unchanged (fixture-side: armor-be6e5146; production-side: multipart
  migration lineage).

## Delta 2026-09-14b (complete fixture set: malformed/ + contradictory/ landed)

Re-run over the **complete 32-directory fixture set**, per armor-45aeb954
(child of the armor-2c7152ae umbrella). The two earlier sections validated
the five tracked fixture families; the malformed/ (12), contradictory/ (1),
edge_cases/ (3) and generated_fixtures/ (5) categories exist only as
untracked working-tree data (never git-tracked in any commit; the
fixture-construction lineage's on-disk output, stable since 2026-09-03).

Method: clean HEAD export of `0e4b9556e` via `git checkout-index
--prefix=<tmp>/ -a` (verified byte-identical to `git archive HEAD`), plus an
explicit copy of those four untracked fixture categories into the export's
`tests/fixtures/migration/`. The shared working tree was not used (a parallel
lineage holds in-flight fixture regeneration and a broken-by-design untracked
`format_migration_classify.go`). Run:
`PATH="$HOME/sdk/go/bin:$PATH" go test ./internal/server/ -run 'TestGolden'
-v -buildvcs=false` (go1.25.0). Exit 1: **205 subtests — 152 PASS, 49 SKIP,
4 FAIL**; all three `TestGoldenFixtureMatrix*` tests PASS; all four pin
tests PASS (the pinned defects all still reproduce); the corruption suite
fails closed on every variant.

Tracked-fixture results are **identical to the baseline table** (v1_single_put
×3 ok; all six multipart fixtures skip on the wrap defect;
v2_single_put/standard skips decrypt+migrate on the stale-content pin; dry-run
stays ok for it). The four failures are all in the two newly-covered
categories, and all are **explained harness-vs-fixture-tree gaps**, not
computed-vs-committed golden-outcome deltas — the committed
`v3-golden-outcomes.json/.yml`, `v3-golden-outcomes-computed.json` and every
fixture manifest agree with the matrix 32/32 (armor-1b0294b5), so no committed
golden file changes:

1. `TestGoldenFixturesDecrypt/malformed/invalid_sidecar_format` and
   `/malformed/truncated_sidecar` — the golden harness's unconditional
   sidecar-accounting precondition (`format_migration_golden_test.go:399-406`)
   `t.Fatalf`s on the fixture's *intended* defect before the outcome switch
   can classify it; it predates corrupt-sidecar fixtures. The matrix handles
   both correctly as `stageAccounting` and passes, and production dry-run +
   migrate already record failed-as-expected for both (currently at the known
   GCM-vs-KWP wrap defect, like every multipart-generation fixture). Harness
   gap: the precondition needs to be outcome-aware.
2. `TestGoldenFixturesDryRun` + `TestGoldenFixturesMigrate`
   `/contradictory/version_says_v1_layout_v2` — the fixture is the known
   vacuous single-block contradiction (94 B = 1 block; V1 and V2 counter
   derivation coincide on block 0, `goldenVacuousNote` in the matrix, which
   skips it there). The golden harness lacks that skip and asserts
   `FailedObjects=1` while the dry run correctly records 0 — the contradiction
   is undetectable at this committed size. Harness gap: the golden dry-run /
   migrate failure branches need the matrix's vacuous pointer-skip, re-arming
   when armor-be6e5146 regenerates the fixture multi-block.

Zero unexplained deltas. Per the armor-9d9cdda9 protocol (document, file
exactly one bead, do not fix inside a validation bead), the two harness gaps
are filed as **armor-abbddad5** (blocks the umbrella armor-2c7152ae, related
to armor-be6e5146); this validation run changed no harness, fixture or golden
code.
