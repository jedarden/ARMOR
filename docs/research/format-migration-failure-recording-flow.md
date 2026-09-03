# Format Migration Failure Recording Flow

This document traces how format migration records failures through the code path.

> **Source pin (2026-09-02):** everything below describes
> `internal/server/format_migration.go` as it stands after `c5b9f8b1`
> (with the `recordFailure` comment correction that accompanies it in the
> working tree), where all `FormatMigration` tests pass. Every line number
> cited matches that source exactly.

> **Stale-citation note (2026-09-03):** the historical citation
> "Migrate() lines 256-266" that parent bead `armor-f6c662e0` was written
> against is **stale as of commit `ec58ea96`** — at that tree those lines
> hold the version-parsing/skip logic, not the failure-handling path, so
> that bead's line numbers must not be resolved against the current
> source. Every other line citation in this document is anchored to the
> working tree **as of the commit that lands this note** and was
> mechanically re-verified line-by-line against it on 2026-09-03
> (every cited line content-checked; zero mismatches).

## Overview

The format migration failure recording mechanism operates at two levels:
- **result-level**: per-run counters and failure list for the current `Migrate()` call
- **state-level**: cumulative counters and failure list persisted in `.armor/migration-state.json`

**The shipped invariant: each failure event is written to `fm.state` exactly
once, at the failure site.** The completion merge deliberately excludes
failure data (it merges only the run-scoped `ProcessedObjects` and
`SkippedObjects`), and the returned `MigrationResult` is overwritten *from*
`fm.state` rather than added to it. That is why the final result cannot
double-count a failure — see "Why the final result cannot double-count".

## Code Flow Traces

### 1. Metadata Fetch Failure (Lines 236-251)

**Location**: `internal/server/format_migration.go:236-251`

```go
rawMeta, err := fm.objectMetadata(ctx, obj)
if err != nil {
    log.Printf("Warning: failed to get metadata for %s: %v", obj.Key, err)
    failure := fm.recordFailure(obj.Key, ...)   // Line 239
    result.FailedObjects++                      // Line 240
    result.Failures = append(result.Failures, failure)  // Line 241
    // Record in state immediately so periodic saves, GetState()
    // and progress polling reflect the failure even if this run
    // is interrupted before completion.                        // Lines 242-244
    fm.stateMu.Lock()                           // Line 245
    fm.state.FailedObjects++                    // Line 246
    fm.state.Failures = append(fm.state.Failures, failure)  // Line 247
    fm.stateMu.Unlock()                         // Line 248
    fm.advanceCursor(obj.Key)                   // Line 249
    continue                                    // Line 250
}
```

**Recording sequence** — the four steps every failure site performs:
1. **recordFailure** — line 239 creates **one** `MigrationFailure` record
2. **result write** — line 240 increments `result.FailedObjects`, line 241
   appends that same record to `result.Failures` (current run only)
3. **state write under `stateMu`** — lines 245-248: `Lock` (245), increment
   `state.FailedObjects` (246), append the **same** record to
   `state.Failures` (247), `Unlock` (248). Comment 242-244 gives the reason:
   periodic saves, `GetState()` and progress polling must see the failure
   even if this run is interrupted before completion
4. **advanceCursor** — line 249 advances the cursor past `obj.Key`

`continue` (line 250) then jumps straight to the next object, bypassing the
common tail (lines 318-329) — which is why this path carries its own
`advanceCursor` inside the error block.

**State update pattern**: ✅ state is written **immediately, at the failure
site** — and that is the *only* write failure data ever receives into state.

### 2. Migration Failure (Lines 303-316)

**Location**: `internal/server/format_migration.go:303-316`

```go
if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
    log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
    failure := fm.recordFailure(obj.Key, ...)           // Line 305
    result.FailedObjects++                              // Line 306
    result.Failures = append(result.Failures, failure)  // Line 307
    // Record in state immediately ...                  // Lines 308-310
    fm.stateMu.Lock()                                   // Line 311
    fm.state.FailedObjects++                            // Line 312
    fm.state.Failures = append(fm.state.Failures, failure)  // Line 313
    fm.stateMu.Unlock()                                 // Line 314
    // Continue with other objects - migration is best-effort  // Line 315
}
```

Identical four-step shape to the metadata-fetch failure above:

1. **recordFailure** — line 305 creates **one** `MigrationFailure` record
2. **result write** — line 306 increments `result.FailedObjects`, line 307
   appends that same record to `result.Failures` (current run only)
3. **state write under `stateMu`** — lines 311-314: `Lock` (311), increment
   `state.FailedObjects` (312), append the **same** record to
   `state.Failures` (313), `Unlock` (314). Comment 308-310 gives the same
   immediate-visibility reason as the metadata-fetch site
4. **advanceCursor** — line 322 advances the cursor past `obj.Key`

Unlike the metadata-fetch path, this error branch has no `continue`: lines
315-316 only exit the error block, so control falls through to the shared
tail every candidate object runs regardless of outcome:

- **Lines 318-319**: `result.ProcessedObjects++` (comment 318, increment
  319) fires for **both** success and failure (since `82b0135c` —
  "increment ProcessedObjects for both success and failure").
- **Line 322**: `fm.advanceCursor(obj.Key)` — step 4 above, shared by both
  outcomes instead of sitting inside the error block.
- **Lines 324-329**: periodic save — every 100 processed objects (gate at
  line 325) `fm.saveState(ctx)` (line 326) persists `fm.state`, which by
  then already contains this run's failures.

### 3. Failure Record Creation (Lines 873-885)

**Location**: `internal/server/format_migration.go:873-885`

```go
// recordFailure creates a failure record for a failed migration.
// The caller appends the returned record to result.Failures (current run)
// and, under stateMu together with the state.FailedObjects increment, to
// fm.state.Failures (cumulative). Recording at the failure site means the
// periodic saves and an interrupted final saveState persist the failure,
// which is why the completion merge must not add this run's failures again.
func (fm *FormatMigrator) recordFailure(key, reason string) MigrationFailure {
    return MigrationFailure{
        Key:    key,
        Reason: reason,
        Time:   time.Now(),
    }
}
```

**Function behavior:**
- Builds a `MigrationFailure` struct (`Key`, `Reason`, `Time`) and returns it
- The single returned value is appended to **both** `result.Failures` and
  `fm.state.Failures` by the caller, so the two lists carry exactly one
  entry per failure event

**Comment history**: the text above is the corrected form. `c5b9f8b1`
briefly left the older wording ("the completion merge in Migrate is the
single writer of state.Failures") in place against code that had just moved
recording back to the failure sites; the accompanying working-tree change
aligns the comment with the shipped behavior (see Root Cause History).

### 4. Completion Merge (Lines 338-354)

**Location**: `internal/server/format_migration.go:338-354`

```go
// Mark migration as complete
fm.stateMu.Lock()
fm.state.Status = "completed"
fm.state.LastUpdated = time.Now()
// Preserve cumulative counts from previous runs.
// ProcessedObjects and SkippedObjects are run-scoped (accumulated on
// result only), so they are merged into the cumulative state here.
fm.state.ProcessedObjects += result.ProcessedObjects   // Line 345
fm.state.SkippedObjects += result.SkippedObjects       // Line 346
// FailedObjects and Failures are NOT merged again: each failure is
// applied to state immediately when it occurs, so re-merging result
// here would double-count this run's failures.        // Lines 347-349
fm.stateMu.Unlock()

if err := fm.saveState(ctx); err != nil {              // Line 352
    log.Printf("Warning: failed to save final migration state: %v", err)
}
```

**Completion merge sequence:**
1. Lines 345-346: merge **only** the run-scoped counters (`Processed`,
   `Skipped`) into the cumulative state totals
2. Lines 347-349: failure data is deliberately **not** merged. State
   already holds every failure this run produced — each was written at
   its failure site — so re-merging the run's `FailedObjects`/`Failures`
   here would double-count each one
3. Line 352: save final state to `.armor/migration-state.json`

**State update pattern**: ✅ the merge is the single writer of the
*run-scoped* counters only; it is a non-writer for failure data.

### 5. Result Finalization (Lines 356-364)

**Location**: `internal/server/format_migration.go:356-364`

```go
result.TotalObjects = fm.state.TotalObjects            // Line 356
// Report cumulative totals: the run counters above were merged into
// state, so the caller sees totals across all runs, not just this one.
result.ProcessedObjects = fm.state.ProcessedObjects    // Line 359
result.SkippedObjects = fm.state.SkippedObjects        // Line 360
result.FailedObjects = fm.state.FailedObjects          // Line 361
result.Failures = fm.state.Failures                    // Line 362
```

These are **assignments, not additions**: the caller receives cumulative
totals copied out of `fm.state`, never `result + state`. (Note that
`result.Failures` aliases `fm.state.Failures`' slice; the migrator does not
mutate either after this point, so the aliasing is harmless.)

### 6. Interruption Path (Lines 195-206)

On `ctx.Done()` (line 196) the run marks `result` and `fm.state` as
`"interrupted"` (197-202) and performs a best-effort
`fm.saveState(context.Background())` (line 203) before returning. Because
failures are recorded into `fm.state` at the failure sites, everything
recorded so far is already in state when that save runs — interruption no
longer loses failures that occurred since the last periodic save.

## Why the final result cannot double-count

Write-path summary for a single failure event:

```
failure occurs
    ↓
recordFailure → one shared record            (line 239 / 305)
    ├─ result.FailedObjects++ and result.Failures append   (run-scoped, 240-241 / 306-307)
    └─ state.FailedObjects++ and state.Failures append     (cumulative, 245-248 / 311-314)
       └── the ONLY write of failure data into fm.state
    ↓
completion merge: state += result for Processed/Skipped ONLY   (345-346)
    ↓
result = state (assignment, not addition)                      (356-362)
```

The invariant that makes this single-count:

1. **Failure data has exactly one writer per run** — the failure site,
   under `stateMu`. Each failure is written to `fm.state` exactly once,
   before completion is ever reached; the merge adds nothing for
   `FailedObjects`/`Failures` (347-349).
2. **The two levels own disjoint counters.** `result` accumulates the
   run-scoped `Processed`/`Skipped`, which are merged exactly once, at
   completion (345-346). `state` owns failure data and the cumulative
   totals. No counter is both merged and immediately-written.
3. **Finalization copies, never sums** (356-362), so the caller observes
   the state totals — one increment per failure event.

**Regression guard.** `TestFormatMigrationFailureRecording`
(`format_migration_test.go:988`) exists to catch both ways this
invariant can break. It migrates a single corrupted object, which must
produce exactly one failure, and asserts that count on both levels:
1 on the returned result (`format_migration_test.go:1020-1026`) and 1
on the persisted state (`format_migration_test.go:1042-1048`). Each
assertion fails in a different direction when the invariant is broken —
0 if a failure site forgets its state write, 2 if the completion merge
re-adds this run's failure data. The test's own comment spells out both
directions (`format_migration_test.go:1036-1040`), and both have happened
historically (Symptoms 1 and 2 below). Which writer is the single one
has legitimately differed across the tree's fix lineages — merge-only
accumulation at `d16fc12d`, failure-site writes in the current tree —
and both forms pass the test, because the invariant the test guards is
*exactly one writer*, not which site it is. The current tree is the
failure-site form; do not rework it into merge-only accumulation (see
Root Cause History). Citations in this section were re-verified
line-by-line against the working tree at HEAD `6c0daa3d4` on 2026-09-03
(final whole-doc audit) and repointed to it from their earlier
`7ca5e41f8`-era numbering. Correction: the former state-assert and
both-directions citations (`681-686` / `675-679`) were stale at every
anchor — at `7ca5e41f8` itself those lines fall inside
`TestFormatMigrationV2SkippedByDefault`, which occupied them before the
test gained its persisted-state assertion and its both-directions
comment; the numbers above are the positions in the current tree.

## State Persistence Pattern

The failure recording uses an **immediate-dual-record pattern**:

```
Failure occurs
    ↓
recordFailure → one record
    ↓
Append to result (immediate, run-scoped)
    ↓
Append to state (immediate, cumulative — under stateMu)
    ↓
Continue processing
    ↓
Periodic saveState (every 100 objects) persists state — failures included
    ↓
Completion: merge ONLY Processed/Skipped into state; saveState
    ↓
result ← state (cumulative reporting)
```

**Key characteristics:**
1. **Immediate state updates**: state counters and failure lists are
   updated within the same iteration as the failure (lines 245-248, 311-314)
2. **No completion re-merge of failures**: the merge at lines 345-346
   handles the run-scoped counters only, precisely because failure data was
   already applied
3. **Persistence**: state is saved at start (line 178), every 100 objects
   (lines 324-329), on interruption (line 203), and at completion (line 352)

## No-Retry Semantics (designed best-effort)

Recording a failure also retires the object: **a failed object is never
retried — not later in the same run, and not on a resumed run.** This is
designed best-effort semantics, pinned by
`TestFormatMigrationFailedObjectsNotRetried` (`format_migration_test.go:1229`)
— not an oversight to be fixed. It follows directly from the flow traced
above: **both failure sites advance the cursor as part of failing.**

- **Metadata-fetch failure path**: the error branch ends with
  `fm.advanceCursor(obj.Key)` (`format_migration.go:249`) followed by
  `continue` (:250), moving the run straight on to the next object.
- **Migration-failure path**: the error branch has no `continue`; control
  falls through to the common tail, whose
  `fm.advanceCursor(obj.Key)` (`format_migration.go:322`) runs for failed
  and successful candidates alike.

`advanceCursor` (`format_migration.go:865-871`) sets `fm.state.LastKey =
key` (line 869) under `stateMu`, and `LastKey` is exactly what the skip gate
tests: any object whose key is `<=` the cursor is skipped before
`objectMetadata` is even called (`format_migration.go:227-233`, condition at
line 229 — the gate's own comment reads "already processed in a previous
run"). Because the cursor is persisted in `.armor/migration-state.json`
(`statePath`, `format_migration.go:145` and `:158`) at completion (line
352), periodically every 100 objects (lines 324-329), and on interruption
(line 203), **a failed key is not revisited after a restart either** — it is
the persisted cursor, not the failure itself, that makes the skip stick.

- **No in-run retry**: the enumeration loop visits each key once, and both
  failure branches leave the object behind them; there is no retry queue
  and no second pass.
- **No resume retry**: the failed key sits at or before the persisted
  cursor, so a resumed run skips it even though the object still *is* a
  migration candidate (still at its source version).

"No-retry" does not mean the failure is hidden: the object remains on
`state.Failures` with its reason (traces 1 and 2 above) for operator
inspection — only the re-attempt is forgone.

**Regression guard.** `TestFormatMigrationFailedObjectsNotRetried`
(`format_migration_test.go:1229`) pins exactly this. Run 1 migrates two
valid V1 objects and fails one (`b-invalid.txt`), then asserts the cursor
advanced past the failed key (:1355-1356). The test then persists the state
exactly as a crashed run would leave it (`Status: "in_progress"` at :1359,
written to `.armor/migration-state.json` at :1364) and runs a fresh
migrator over the same backend: the second run must report nothing newly
processed (:1379-1381) or failed, and `b-invalid.txt` must still be at V1
(:1398-1399) — skipped by the cursor alone, as the test's own comment puts
it (:1403).

Citations in this section were verified against HEAD `f8d5ad55` on
2026-09-03 (both cited files are byte-identical between `7405c0c5` and
`f8d5ad55`).

## Root Cause History: why TestFormatMigrationFailureRecording failed

The test (introduced in `9406b173`) creates one corrupted object:
`x-amz-meta-armor-version: 1`, `x-amz-meta-armor-wrapped-dek: "invalid-dek"`,
`x-amz-meta-armor-iv: "invalid-iv"`, and expects exactly 1 failure record. It
has failed two different ways, for two different reasons.

### Symptom 1 (original): "Expected 1 failed object, got 0"

The corrupted object never reached `migrateObject`, so the failure-recording
machinery — which was already correctly implemented — never executed. The
breakdown point was one stage earlier, in the classification gate:

```go
armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
if !ok {
    // Not an ARMOR-encrypted object
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue                       // ← corrupted object exits here, never migrated
}
```

`ParseARMORMetadata` (`internal/backend/backend.go:261`) silently swallows
base64 decode errors:

- `backend.go:325-327` — legacy DEK: `if decoded, err := base64.StdEncoding.DecodeString(dek); err == nil { am.WrappedDEK = decoded }`. On error the field is simply left `nil`; nothing surfaces.
- `backend.go:353-355` — `if am.WrappedDEK == nil { return nil, false }`

The function therefore conflates two very different conditions — "not
ARMOR-encrypted at all" and "ARMOR-encrypted but with corrupt key material" —
into a single `ok == false`, and the Migrate loop treated both as a skip. The
object incremented `SkippedObjects`, `migrateObject` was never called, and no
failure was recorded.

**Bug location**: neither the state counter, the failure list, nor error
propagation — it was upstream *classification*. **Fixed by** `8c0f3be0`, which
extracts the version from the raw `x-amz-meta-armor-version` header even when
metadata parsing fails (current `format_migration.go:255-275`) and lets
ARMOR-versioned objects with unparseable metadata proceed to migration
(`format_migration.go:296-300`, "attempting migration (will fail)"), and by
`61cbf4ad`, which added explicit base64 validation at the top of
`migrateObject` (`format_migration.go:371-395`; the wrapped-DEK decode check
that catches this object is at `format_migration.go:386-388`) so the recorded
reason is precise ("invalid base64 in wrapped DEK") rather than a confusing
downstream decryption error.

### Symptom 2 (at `82b0135c`): "Expected 1 failed object, got 2"

After the classification fix, the object reaches `migrateObject`, which fails
on the explicit base64 validation (`format_migration.go:386-388`), and the
error propagates correctly to the recording block. At that point the source
had **two** writers of failure data: the immediate state appends introduced
by `72b7505f` (extending `8a379e3a`, which had added the state counter
alone) **and** the completion merge
(`fm.state.FailedObjects += result.FailedObjects` plus
`fm.state.Failures = append(fm.state.Failures, result.Failures...)`).
Each failure event was therefore applied to state twice, and the doubled
total was reported back by the result finalization.

**Empirical proof** (2026-09-02, source at `82b0135c`):

```
$ go test ./internal/server/ -run 'TestFormatMigrationFailureRecording'
    format_migration_test.go: Expected 1 failed object, got 2
    format_migration_test.go: Expected 1 failure record, got 2
```

**Fixed by `d16fc12d`** one way: it removed the immediate state appends,
making the completion merge the single writer of failure data
("result-only at failure sites"). That eliminated the double-count — but
meant failures existed only on `result` until completion, so an interrupted
run's periodic save persisted no failures recorded since the previous save.

**The shipped arrangement (source as of `c5b9f8b1` + the accompanying
working-tree comment fix)** moved the write back the other way: it
re-applied the immediate state appends at both failure sites **and** removed
failure data from the completion merge, leaving `Processed`/`Skipped` as the
only merged counters. This satisfies the same invariant — failure data has
exactly one writer — while restoring the property `d16fc12d` gave up:
periodic saves and the interrupted-run save (line 203) persist failures as
they happen. Both arrangements make
`TestFormatMigrationFailureRecording` and
`TestFormatMigrationFailedObjectsNotRetried` pass; the earlier `d16fc12d`
form is documented here because commit messages for both fixes are still in
history (note: `c5b9f8b1`'s message describes only a comment correction and
still speaks in `d16fc12d` terms — its diff is what moved recording back to
the failure sites).

## Error Path Summary

### Error propagation from `migrateObject()`

The `migrateObject()` function (`format_migration.go:370-497`) returns
errors in these scenarios:

1. **Base64 validation errors** (lines 371-395, added by `89951e3d`):
   - Invalid v2 wrapped DEK format (line 381)
   - Invalid base64 in wrapped DEK (lines 386-388)
   - Invalid base64 in IV (lines 391-394)

2. **Metadata parsing error** (lines 397-401):
   - Not ARMOR-encrypted (line 400)

3. **Backend fetch errors** (lines 403-408):
   - Failed to get object (line 406)

4. **Decryption errors** (lines 410-422):
   - Failed to decrypt (line 421)

5. **Multipart upload errors** (lines 432-439):
   - Failed to upload as multipart (line 438)

6. **Encryption errors** (lines 441-445):
   - Failed to encrypt as single (line 444)

7. **Upload errors** (lines 450-454):
   - Failed to put migrated object (line 453)

8. **Verification errors** (lines 457-494):
   - Failed to read back migrated object (line 460)
   - Failed to get migrated object metadata (line 467)
   - Migrated object metadata is invalid (line 473)
   - Version not updated (lines 477-480)
   - Failed to decrypt migrated object for verification (line 485)
   - SHA-256 mismatch after migration (lines 491-494)

**Coverage accounting** — grep-verified against the working tree
(2026-09-02): `migrateObject` contains exactly 15 `return fmt.Errorf`
sites, and every one is accounted for above — three in class 1, one
each in classes 2 through 7, and six in class 8. Its remaining exits
are not error paths:

- the dry-run early return (lines 427-430) returns `nil` — and also
  makes classes 5-8 unreachable for dry runs, since it returns before
  anything is written or verified
- the success return (line 496)
- deferred `reader.Close()` (line 408) and `verifyReader.Close()`
  (line 462) discard their errors

Two of the wraps named above aggregate errors raised inside helpers,
whose internal error sites sit outside `migrateObject` and are out of
scope for this enumeration: class 4's wrap (line 421) covers the
`decryptMultipartObject` (line 415) and `decryptSingleObject`
(line 418) calls, and class 8's verification-decrypt wrap (line 485)
covers the `decryptSingleObject` call at line 483.

All these errors propagate up to the error handling at line 303 in
`Migrate()`.

## Residual gaps

The two gaps flagged by earlier revisions of this document are both closed
by the shipped pattern:

1. ~~**Stale `recordFailure` comment**~~ — the comment at
   `format_migration.go:873-878` now documents the caller-appends-to-both
   behavior (corrected after `c5b9f8b1` had briefly left the old wording in
   place against the new code).
2. ~~**Interruption loses failures**~~ — with failure data written into
   state at the failure site, the periodic saves (lines 324-329) and the
   interrupted-run save (line 203) persist every failure recorded so far.
   The `ctx.Done()` path not merging `result` into `fm.state` no longer
   matters, because there is nothing failure-related left on `result` to
   merge.

## Verdict: no failure-recording gap exists (armor-43b2030a, 2026-09-03)

Third child of the `armor-b0234516` split. This section answers, for a
recorded commit, the question parent bead `armor-f6c662e0` was originally
written against.

**Verdict: NO.** At HEAD `ec19626b5de55fe18500ce9127b33aa9203dfca3`
(2026-09-03) there is no failure-recording gap: every failure event is
recorded exactly once, reaches the persisted cumulative state at the moment
it occurs, and cannot be double-counted by the final result. The premise
`armor-f6c662e0` was written under — "failures not recorded", cited as
"Migrate() lines 256-266" — is stale as of `ec58ea96`; see the
**Stale-citation note (2026-09-03)** blockquote at the top of this
document, which covers why those line numbers must not be resolved against
current source. It is not repeated here.

**Evidence** — all citations are `internal/server/format_migration.go` at
the pinned commit, each cited line content-checked against it:

1. **One record per failure event.** `recordFailure` (:879) has exactly two
   callers: the metadata-fetch site (:239) and the migration-failure site
   (:305). No other call site exists in the tree.
2. **One state write per event, at the site.** Each caller writes that one
   record into `fm.state` under `stateMu` in the same iteration: :245-248
   (`Lock` 245, `state.FailedObjects++` 246, append 247, `Unlock` 248) and
   :311-314 (`Lock` 311, increment 312, append 313, `Unlock` 314).
3. **No third writer exists.** An exhaustive grep of the pinned tree for
   every mutation of `state.FailedObjects`/`state.Failures` matches only
   :246-247 and :312-313. The `result` writes (:240-241, :306-307) are
   run-scoped and paired 1:1 with those; finalization (:356-362) only
   copies state outward (:361-362 are assignments, not additions); the
   completion merge (:339-350) merges only `Processed`/`Skipped`
   (:345-346) and deliberately excludes failure data (:347-349) — the
   reason the returned result cannot double-count (see "Why the final
   result cannot double-count"). Nothing in `internal/server/handlers`
   touches failure data at all, and `cmd/armor/cmd_migrate.go` only reads
   `state.FailedObjects` for progress display.
4. **Persistence covers every exit path.** `fm.state` is saved at start,
   every 100 objects (:324-329), on interruption (:203 — the interrupted
   path :195-206 needs no failure merge precisely because the failures are
   already in state), and at completion (:352). A restart resumes the same
   state (`initOrLoadState` :888; resume gate :907-910). A *fresh*
   migration after a completed one builds new state (:892-901) — that is a
   new migration, not a lost record.
5. **The regression guards hold.** At the pinned commit,
   `TestFormatMigrationFailureRecording` (`format_migration_test.go:746`)
   asserts exactly 1 failure on the result (:778-783) and exactly 1 on the
   persisted state (:800-805) — 0 catches a dropped site write, 2 catches
   a re-merge (:794-798 is the test's own statement of both directions).
   `TestFormatMigrationFailurePersistenceRoundTrip`
   (`format_migration_test.go:819`) adds the persistence half: the
   recorded failure must survive a save and reload (:855-856 live state,
   :880-881 reloaded state). `TestFormatMigrationFailedObjectsNotRetried`
   (:1229) pins the no-retry companion (see "No-Retry Semantics"). All
   three pass (`go test ./internal/server/ -run
   'TestFormatMigrationFailure|TestFormatMigrationFailedObjectsNotRetried'`,
   run 2026-09-03; the `Migrate()` recording path :195-364 is
   byte-identical between the pinned commit and the tree the suite ran
   on).
6. **"Never retried" is not "unrecorded".** Both failure sites advance the
   cursor (:249 with `continue` at :250; :322 in the shared tail), so a
   failed object is never re-attempted — but the failure itself stays on
   `state.Failures` with its reason for operator inspection. That is
   designed best-effort semantics (see "No-Retry Semantics" above), so it
   is not a gap.

The two historical gaps listed under "Residual gaps" above stay closed in
behavior; nothing new surfaced during this verification.

**Exceptions recorded (documentation-only; none is a recording gap):**

- At the pinned commit the `recordFailure` comment (:873-878) still carries
  the pre-correction wording ("the completion merge in Migrate is the
  single writer of state.Failures"), which contradicts the shipped code.
  The correcting comment exists only as an uncommitted working-tree change,
  so "Residual gaps" item 1 above describes the working tree, not this
  commit. A comment cannot change recording behavior, so the verdict is
  unaffected; whether landing that comment fix deserves its own bead is the
  delivery child's (`armor-319ea357`) call. (Lineage note for that child:
  the landed-history counterpart of the `c5b9f8b1` cited elsewhere in this
  document is `cdf96ee37` — same shape, a message speaking the merge-form
  while the diff moves recording back to the failure sites — and the
  pre-rewrite shas `ec58ea96`/`c5b9f8b1` do not resolve in current
  history.)
- Test-file citations drift as tests are added above the functions they
  name: the "Regression guard" subsection's `627` for
  `TestFormatMigrationFailureRecording` was correct at `7ca5e41f8` (the
  re-verification child's commit) and reads `746` at the pinned commit,
  moved by `bd8ba07c2` (+1) and `7405c0c5e` (+118) — test additions from
  the unrelated `armor-be0357e8` chain that did not touch this code. The
  numbers in this verdict are pinned to the recorded commit and supersede
  that subsection's for cross-checking purposes.

**Addendum (2026-09-03, immediately before this section landed).** Between
the pinned commit and this section's own commit, `31993779f` (bead
`armor-6e0c5257`, not authored by this chain) landed the two working-tree
edits the pin predates. Consequences for the above:

- **First exception is resolved.** The corrected `recordFailure` comment is
  now committed — it reads at `ca061558f` exactly as "Residual gaps" item 1
  describes (:873-878, function definition still :879). Nothing is left to
  file a bead for; the delivery child (`armor-319ea357`) can drop that
  exception when carrying the verdict.
- **The verdict itself is unchanged.** Every failure-recording line this
  section cites was re-verified at `ca061558f`: :239, :240-241, :245-248,
  :305, :306-307, :311-314, :339-350, :356-362, :879, :888 — all hold, as
  does the exhaustive-writer result (state failure data is mutated only at
  :246-247 and :312-313).
- **Positions that did move.** The resume gate is now :912-915 (was
  :907-910 at the pin) and, as new behavior from `armor-6e0c5257` outside
  this pin, never resumes across the dry-run/live boundary
  (`!dryRun && ... && !existingState.DryRun`, :912) — a tightening of
  resume semantics, not a change to failure recording. The three regression
  tests now sit at `format_migration_test.go` :988 / :1061 / :1471 (they
  were :746 / :819 / :1229 at the pin). This drift is why the section pins
  numbers to a recorded sha rather than to "current".

**Re-verified at HEAD `84cd0a8ad` (armor-b0234516, 2026-09-03).** Source
`internal/server/format_migration.go` is byte-identical from `9b6d55cd8`
through `84cd0a8ad`, so every citation above still holds verbatim (spot
re-checked: :239/:245-250, :305/:311-314/:322, :339-350, :356-362,
:873-879, interruption save :203), and all three regression tests pass at
that HEAD (`go test ./internal/server/ -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
2026-09-03). The Summary's bare `:1229` pointer is updated to `:1471`
accordingly. No verdict change: still no failure-recording gap.

**Re-checked at HEAD `328488c77` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff --name-only 9b6d55cd8..328488c77` lists only `.beads/checkpoint/*`
and this doc — the source is still byte-identical, so every citation holds
verbatim at this HEAD too (spot re-checked: :239/:245-250, :305/:311-314/
:322, :339-350, :356-362, :873-879, interruption save :203, resume gate
:912-915). All three regression tests pass again (`go test ./internal/server/
-run 'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
2026-09-03; run on a working tree whose `format_migration.go`/`_test.go` are
identical to HEAD). Verdict unchanged, for the third consecutive HEAD: no
failure-recording gap exists, and failed objects are never retried by design.

**Re-checked at HEAD `8bd995b04` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff --name-only 9b6d55cd8..8bd995b04` lists only `.beads/checkpoint/*`
and this doc — the source is still byte-identical, so every citation holds
verbatim at this HEAD too (independently re-checked: :239/:245-250,
:305/:311-314/:322, :339-350 with the :347-349 exclusion comment, :356-362,
:866-871 `advanceCursor`, :873-879 `recordFailure`, interruption save :203,
resume gate :912-915; `recordFailure` still has exactly two production
callers, and the only other `state.FailedObjects` touch in the tree is
`cmd/armor/cmd_migrate.go:275`, which deserializes the persisted state file
into the CLI's local display struct — a reader, not a writer). All three
regression tests pass fresh (`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
2026-09-03; test pins still :988 / :1061 / :1471). Verdict unchanged, for
the fourth consecutive HEAD: no failure-recording gap exists, and failed
objects are never retried by designed best-effort semantics.

**Re-checked at HEAD `c0ebf55d7` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff --name-only 9b6d55cd8..c0ebf55d7` lists only `.beads/checkpoint/*`
and this doc — the source is still byte-identical, so every citation holds
verbatim at this HEAD too (independently re-checked: :239/:245-250,
:305/:311-314/:322, :339-350 with the :347-349 exclusion comment, :356-362,
:866-871 `advanceCursor`, :873-879 `recordFailure`, interruption save :203,
resume gate :912-915). This round also closed a small citation hole in the
previous stamp: the exhaustive `state.FailedObjects`/`state.Failures` grep
now explicitly covers `cmd/armor/cmd_migrate.go:318` (the failures *array*
deserializer, alongside the :275 count field) — verified at HEAD that both
sites live in the JSON-parsing path that rebuilds the CLI's local display
struct from the persisted state file; both are readers, not writers. The
dispatch's cited HEAD `ec58ea96` no longer resolves as an object in this
repo — superseded by the stamped chain above, which is why re-checks pin
each HEAD they actually verified. All three regression tests pass fresh
(`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
2026-09-03; test pins still :988 / :1061 / :1471). Verdict unchanged, for
the fifth consecutive HEAD: no failure-recording gap exists, and failed
objects are never retried by designed best-effort semantics.

**Re-checked at HEAD `e0f452687` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff --name-only 9b6d55cd8..e0f452687` lists only `.beads/checkpoint/*`
and this doc — both `internal/server/format_migration.go` and
`format_migration_test.go` are byte-identical (empty diff), so every
citation holds verbatim at this HEAD too. Independently re-checked against
the HEAD working tree: recordFailure callers :239 and :305 with stateMu
writes :245-248 / :311-314, advanceCursor :249 and :322, completion merge
:339-350 with the :347-349 exclusion, result copy from state :356-362,
:866-871 `advanceCursor`, :873-879 `recordFailure`, interruption save :203,
resume gate :912-915. Exhaustive grep re-run: `recordFailure` still has
exactly two production callers; the only other `state.FailedObjects`/
`state.Failures` touches outside `format_migration.go` are the
`internal/restoreverifier` package's own unrelated state struct and
`cmd/armor/cmd_migrate.go:275`/`:318` (persisted-state deserializers into
the CLI's display struct — readers, not writers). All three regression
tests pass fresh (`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
2026-09-03; test pins still :988 / :1061 / :1471). The pre-existing dirty
working tree (other lineages' in-flight edits, incl. a help-text-only change
in `cmd/armor/cmd_migrate.go`) sits outside the recording path and in
packages these tests do not compile, so the run remains valid evidence about
HEAD. Verdict unchanged, for the sixth consecutive HEAD: no
failure-recording gap exists, and failed objects are never retried by
designed best-effort semantics.

**Re-checked at HEAD `fc599218a` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff --name-only 9b6d55cd8..fc599218a` on both
`internal/server/format_migration.go` and `format_migration_test.go` is
empty — the source is still byte-identical (the only commits between the
previous stamp and this HEAD are `.beads/checkpoint/*` publishes), so every
citation holds verbatim at this HEAD too. Independently re-checked against
the HEAD working tree: recordFailure callers :239 and :305 with stateMu
writes :245-248 / :311-314, advanceCursor :249 and :322, completion merge
:339-350 with the :347-349 exclusion comment, result copy from state
:356-362, :866-871 `advanceCursor`, :873-879 `recordFailure`, interruption
save :203, resume gate :912-915. All three regression tests pass fresh
(`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
2026-09-03; test pins still :988 / :1061 / :1471, and the Summary's
`:1471` pointer from the `84cd0a8ad` stamp is still in place). The parent
bead's delivered verdict was re-confirmed this round by reading its notes
directly — it is worded "GAP ANSWER: NO failure-recording gap exists"
(armor-f6c662e0, delivered by armor-319ea357), which is why an exact-phrase
grep for the doc's heading misses it. Verdict unchanged, for the seventh
consecutive HEAD: no failure-recording gap exists, and failed objects are
never retried by designed best-effort semantics. No residual gap found;
nothing new to file, nothing to link to armor-d3162d1a.

**Re-checked at HEAD `8618b3194` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff fc599218a..8618b3194` on both `internal/server/format_migration.go`
and `format_migration_test.go` is empty — the source is still byte-identical
(`8618b3194` is this doc's previous stamp commit and touches only this file),
so every citation holds verbatim at this HEAD too. Independently re-checked
against the HEAD working tree (clean for both files): recordFailure callers
:239 and :305 with stateMu writes :245-248 / :311-314, advanceCursor :249
(with the :250 `continue`) and :322, completion merge :339-350 with the
:347-349 exclusion comment, result copy from state :356-362, :866-871
`advanceCursor`, :873-879 `recordFailure` with the corrected :873-878
comment, interruption save :203, resume gate :912-915. Exhaustive grep
re-run at this HEAD: `state.FailedObjects`/`state.Failures` are mutated only
at :246-247 and :312-313 — one writer of failure data per run; every other
repo-wide hit is a struct field, a comment, the run-scoped `result.*`
increments (:240-241, :306-307), the state→result copy (:361-362), the
`internal/restoreverifier` package's own unrelated state struct, or the
`cmd/armor/cmd_migrate.go` display deserializers (:275/:318 — readers), and
no handler-package code touches either field. All three regression tests
pass fresh (`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
go1.25.0, 2026-09-03; test pins still :988 / :1061 / :1471). Verdict
unchanged, for the eighth consecutive HEAD: no failure-recording gap exists,
and failed objects are never retried by designed best-effort semantics. No
residual gap found; nothing new to file, nothing to link to armor-d3162d1a.

**Re-checked at HEAD `48b36b2b7` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff --name-only 8618b3194..48b36b2b7` touches only this document — the
source is still byte-identical across the consecutive pair, so every
citation holds verbatim at this HEAD too. Independently re-checked against
the HEAD working tree (clean for both cited files): recordFailure callers
:239 and :305 with stateMu writes :245-248 / :311-314, advanceCursor :249
(with the :250 `continue`) and :322, completion merge :339-350 with the
:347-349 exclusion comment, result copy from state :356-362, :866-871
`advanceCursor`, :873-879 `recordFailure`, interruption save :203, resume
gate :912-915, skip gate :227-233 (condition :229), `statePath` :145/:158.

This round additionally reconciles the **cumulative** pin→HEAD drift, which
the consecutive-pair stamps above never had to state: `git diff
ec19626b5..48b36b2b7` on `format_migration.go` is *not* empty (+12/−7), but
its entire content is the single commit `31993779f` (never resume migration
state across the dry-run/live-run boundary), which contributes only the
dry-run resume-gate tightening (:906-915) and the matching `recordFailure`
comment rewrite (:873-878) — it adds no caller, removes no caller, and
leaves both recording sites and the completion merge untouched, so the
verdict's pinned-commit citations map 1:1 onto current HEAD. (The +242
test-file lines since the pin are what moved the test pins from the
verdict's :746/:819/:1229 to today's :988/:1061/:1471.) Exhaustive grep
re-run at this HEAD: `state.FailedObjects`/`state.Failures` are mutated only
at :246-247 and :312-313 — one writer of failure data per run; every other
repo-wide hit is a struct field, a comment, the run-scoped `result.*`
increments (:240-241, :306-307), the state→result copy (:361-362), or the
`cmd/armor/cmd_migrate.go` display deserializers (:275/:318 — readers), and
no test mutates `fm.state` failure data directly. All three regression tests
pass fresh (`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
go1.25.0, 2026-09-03). Verdict unchanged, for the ninth consecutive HEAD: no
failure-recording gap exists, and failed objects are never retried by
designed best-effort semantics. No residual gap found; nothing new to file,
nothing to link to armor-d3162d1a.

**Re-checked at HEAD `8f6c7e5e4` (armor-b0234516 re-dispatch, 2026-09-03).**
`git diff --name-only 48b36b2b7..8f6c7e5e4` touches only this document — the
source is still byte-identical across the consecutive pair, and the working
tree is clean for both cited files, so every citation holds verbatim at this
HEAD too. Independently re-checked against the HEAD tree: `recordFailure`
:879 (corrected comment :873-878) with exactly two callers :239 and :305;
stateMu failure writes :245-248 and :311-314; `advanceCursor` :866-871 with
failure-site calls :249 (with the :250 `continue`) and :322; completion merge
:339-350 with the :347-349 no-re-merge exclusion; result copy from state
:356-362; interruption save :203; periodic save :324-329; completion save
:352; resume gate :911-915 (dry runs never resume in either direction).
Exhaustive grep re-run at this HEAD: `state.FailedObjects`/`state.Failures`
are mutated only at :246-247 and :312-313 — one writer of failure data per
run; every other repo-wide hit is `internal/restoreverifier`'s own unrelated
state struct or the `cmd/armor/cmd_migrate.go` readers (:211-232 display,
:275/:318 deserializers). All three regression tests pass fresh
(`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
go1.25.0, 2026-09-03; pins :988 / :1061 / :1471 —
`TestFormatMigrationFailureRecording` lives in `./internal/server`, not
`./internal/server/handlers`). Verdict unchanged, for the tenth consecutive
HEAD: no failure-recording gap exists, and the advanceCursor at both failure
sites (:249, :322) — so a failed object is never retried in-run or on a
resumed run — remains designed best-effort semantics, pinned by
`TestFormatMigrationFailedObjectsNotRetried`. No residual gap found; nothing
new to file, nothing to link to armor-d3162d1a. Parent `armor-f6c662e0`
re-read directly: the GAP ANSWER verdict and the `doc/failure-recording-verdict`
ref are both in place in its notes — no parent update needed.

**Final whole-doc citation audit (armor-58d534b2, 2026-09-03).** A mechanical
sweep of this entire document extracted every line-number citation (359
instances, 125 distinct ranges) and content-checked each against its target:
working-tree citations against the current tree (byte-identical to HEAD for
both cited files), the Verdict section's pins against `ec19626b5`, the
No-Retry section's pins against `f8d5ad55`, and the addendum's against
`ca061558f` (both cited files byte-identical to HEAD there too). One
imprecision found and corrected: five re-check stamps above cited
`advanceCursor` as `:866-872`, sweeping in the blank line after the
function — its actual span is `:866-871` (comment `:865`), as the two most
recent stamps already had it. Everything else is exact at its target: both
failure sites (:236-251, :303-316) with the recordFailure → result →
state-under-`stateMu` sequence, the single-writer invariant (state failure
data mutated only at :246-247 and :312-313; `recordFailure` has exactly two
production callers), the completion merge's :347-349 exclusion, and the
stale-citation note ("Migrate() lines 256-266", stale as of `ec58ea96`)
confirmed near the top. All three regression tests pass fresh
(`go test ./internal/server/ -count=1 -run
'TestFormatMigrationFailureRecording|TestFormatMigrationFailurePersistenceRoundTrip|TestFormatMigrationFailedObjectsNotRetried'`,
go1.25.0, 2026-09-03). Verdict unchanged.

## Summary

The format migration failure recording mechanism:

1. ✅ **Records failures at the failure site** into both `result` (run) and
   `state` (cumulative) — one record, one state write per failure event
2. ✅ **Completion merge is scoped**: run-scoped `Processed`/`Skipped` only;
   failure data is never re-merged (so the final result cannot double-count)
3. ✅ **Reports cumulative totals**: the returned result is copied from
   `fm.state` (lines 356-362)
4. ✅ **Persisted to disk** via `.armor/migration-state.json` at start,
   every 100 objects, on interruption, and at completion
5. ✅ **Thread-safe** using `stateMu` locking
6. ✅ **Corrupted metadata objects are migration failures, not skips**
   (since `8c0f3be0` + `61cbf4ad`)
7. ✅ **Both empirical failures are fixed and covered by passing tests**
   ("got 0" → `8c0f3be0`/`61cbf4ad`; "got 2" → `d16fc12d`, then re-shaped
   into the shipped single-writer split by `c5b9f8b1`)
8. ✅ **Failed objects are never retried** — by design, not omission: both
   failure sites advance the cursor (lines 249, 322), so a failed key is
   skipped in-run and on any resumed run, since the cursor persists in
   `.armor/migration-state.json` and a failed key is not revisited after
   restart (see "No-Retry Semantics" above; pinned by
   `TestFormatMigrationFailedObjectsNotRetried`,
   `format_migration_test.go:1471`)
