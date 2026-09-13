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
