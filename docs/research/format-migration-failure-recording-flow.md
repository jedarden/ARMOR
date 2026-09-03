# Format Migration Failure Recording Flow

This document traces how format migration records failures through the code path.

> **Source pin (2026-09-02):** everything below describes
> `internal/server/format_migration.go` as it stands after `c5b9f8b1`
> (with the `recordFailure` comment correction that accompanies it in the
> working tree), where all `FormatMigration` tests pass. Every line number
> cited matches that source exactly.

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
(`format_migration_test.go:627`) exists to catch both ways this
invariant can break. It migrates a single corrupted object, which must
produce exactly one failure, and asserts that count on both levels:
1 on the returned result (`format_migration_test.go:659-664`) and 1 on
the persisted state (`format_migration_test.go:681-686`). Each
assertion fails in a different direction when the invariant is broken —
0 if a failure site forgets its state write, 2 if the completion merge
re-adds this run's failure data. The test's own comment spells out both
directions (`format_migration_test.go:675-679`), and both have happened
historically (Symptoms 1 and 2 below). Which writer is the single one
has legitimately differed across the tree's fix lineages — merge-only
accumulation at `d16fc12d`, failure-site writes in the current tree —
and both forms pass the test, because the invariant the test guards is
*exactly one writer*, not which site it is. The current tree is the
failure-site form; do not rework it into merge-only accumulation (see
Root Cause History). Citations in this section were grep-verified
against the working tree on 2026-09-03.

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
