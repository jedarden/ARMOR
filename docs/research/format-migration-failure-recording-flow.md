# Format Migration Failure Recording Flow

This document traces how format migration currently records failures through the code path.

## Overview

The format migration failure recording mechanism operates at two levels:
- **result-level**: In-memory counters for the current run
- **state-level**: Persistent counters stored in `.armor/migration-state.json`

## Code Flow Traces

### 1. Metadata Fetch Failure (Lines 236-247)

**Location**: `internal/server/format_migration.go:236-247`

```go
rawMeta, err := fm.objectMetadata(ctx, obj)
if err != nil {
    log.Printf("Warning: failed to get metadata for %s: %v", obj.Key, err)
    result.FailedObjects++                                          // Line 239
    result.Failures = append(result.Failures, fm.recordFailure(    // Line 240
        obj.Key, fmt.Sprintf("failed to get metadata: %v", err)))
    fm.stateMu.Lock()                                               // Line 241
    fm.state.FailedObjects++                                       // Line 242
    fm.state.Failures = append(fm.state.Failures, fm.recordFailure(// Line 243
        obj.Key, fmt.Sprintf("failed to get metadata: %v", err)))
    fm.stateMu.Unlock()                                             // Line 244
    fm.advanceCursor(obj.Key)                                       // Line 245
    continue
}
```

**Recording sequence:**
1. Line 239: Increment `result.FailedObjects` counter
2. Line 240: Append failure record to `result.Failures`
3. Line 241-244: Lock state mutex, increment `state.FailedObjects`, append to `state.Failures`, unlock
4. Line 245: Advance cursor to `obj.Key`
5. `continue`: Skip to next object

**State update pattern**: ✅ State is updated **immediately** (within the same iteration)

### 2. Migration Failure (Lines 299-312)

**Location**: `internal/server/format_migration.go:299-312`

```go
if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
    log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
    result.FailedObjects++                                          // Line 301
    result.Failures = append(result.Failures, fm.recordFailure(    // Line 302
        obj.Key, fmt.Sprintf("migration failed: %v", err)))
    fm.stateMu.Lock()                                               // Line 303
    fm.state.FailedObjects++                                       // Line 304
    fm.state.Failures = append(fm.state.Failures, fm.recordFailure(// Line 305
        obj.Key, fmt.Sprintf("migration failed: %v", err)))
    fm.stateMu.Unlock()                                             // Line 306
    // Continue with other objects - migration is best-effort
} else {
    // Migration succeeded (including dry-run mode) - increment counter
    result.ProcessedObjects++                                     // Line 310
}
```

**Recording sequence:**
1. Line 301: Increment `result.FailedObjects` counter
2. Line 302: Append failure record to `result.Failures`
3. Line 303-306: Lock state mutex, increment `state.FailedObjects`, append to `state.Failures`, unlock
4. Comment states migration is best-effort, so continues to next object

**State update pattern**: ✅ State is updated **immediately** (within the same iteration)

### 3. Failure Record Creation (Lines 863-873)

**Location**: `internal/server/format_migration.go:863-873`

```go
// recordFailure creates a failure record for a failed migration.
// The returned record is appended to result.Failures by the caller,
// and also immediately appended to fm.state.Failures to ensure persistence
// even if the migration is interrupted before completion.
func (fm *FormatMigrator) recordFailure(key, reason string) MigrationFailure {
    return MigrationFailure{
        Key:    key,
        Reason: reason,
        Time:   time.Now(),
    }
}
```

**Function behavior:**
- Creates a `MigrationFailure` struct with:
  - `Key`: object key that failed
  - `Reason`: formatted error message
  - `Time`: current timestamp
- Returns the struct to caller for appending

**Caller responsibility**: Both result and state appending are done by the caller (see sections 1 & 2)

### 4. Completion Merge (Lines 331-356)

**Location**: `internal/server/format_migration.go:331-356`

```go
// Mark migration as complete
fm.stateMu.Lock()
fm.state.Status = "completed"
fm.state.LastUpdated = time.Now()
// Preserve cumulative counts from previous runs
// result.* contains only current run increments, state.* contains cumulative totals
fm.state.ProcessedObjects += result.ProcessedObjects               // Line 336
fm.state.SkippedObjects += result.SkippedObjects                   // Line 337
fm.state.FailedObjects += result.FailedObjects                     // Line 338
// Merge failures (append current run failures to existing list)
fm.state.Failures = append(fm.state.Failures, result.Failures...) // Line 340
fm.stateMu.Unlock()

if err := fm.saveState(ctx); err != nil {
    log.Printf("Warning: failed to save final migration state: %v", err)
}
```

**Completion merge sequence:**
1. Line 336-338: Add result counters to state counters (cumulative total)
2. Line 340: Append all result failures to state failures list
3. Line 343: Save final state to `.armor/migration-state.json`

**State update pattern**: ✅ State is updated **at completion** (after all objects processed)

## Error Path Summary

### Error propagation from `migrateObject()`

The `migrateObject()` function (lines 359-487) returns errors in these scenarios:

1. **Base64 validation errors** (lines 363-385):
   - Invalid base64 in wrapped DEK (line 377)
   - Invalid base64 in IV (line 383)

2. **Metadata parsing error** (lines 387-391):
   - Not ARMOR-encrypted (line 390)

3. **Backend fetch errors** (lines 394-397):
   - Failed to get object (line 396)

4. **Decryption errors** (lines 400-412):
   - Failed to decrypt (line 411)

5. **Encryption errors** (lines 432-435):
   - Failed to encrypt as single (line 434)

6. **Upload errors** (lines 441-444):
   - Failed to put migrated object (line 443)

7. **Multipart upload errors** (lines 426-429, 674-779):
   - Failed to upload as multipart (line 428)

8. **Verification errors** (lines 448-485):
   - Failed to read back migrated object (line 450)
   - Failed to get migrated object metadata (line 457)
   - Migrated object metadata is invalid (line 463)
   - Version not updated (line 468-470)
   - Failed to decrypt migrated object for verification (line 475)
   - SHA-256 mismatch after migration (line 482-484)

All these errors propagate up to the error handling at line 299 in `Migrate()`.

## State Persistence Pattern

> **Note (2026-09-02):** everything in this section and the three code-flow
> traces above describes source at `82b0135c`. The dual-append pattern below
> is what caused the double-counting in "Potential Issues"; the fix removes
> the immediate state appends and keeps the completion merge as the single
> state writer. Line numbers for the corrected flow are in "Where the
> recording path stands after both fixes".

The failure recording uses a **dual-append pattern**:

```
Failure occurs
    ↓
Append to result (immediate)
    ↓
Append to state (immediate)
    ↓
Continue processing
    ↓
Completion: merge result into state again (cumulative)
```

**Key characteristics:**
1. **Immediate state updates**: Both result and state counters are updated within the same iteration when a failure occurs
2. **Duplicate appending at completion**: At completion, result failures are appended again to state (line 340), creating duplicates
3. **Persistence**: State is saved to `.armor/migration-state.json` periodically (every 100 objects, line 317-321) and at completion (line 343)

## Potential Issues

### 1. Duplicate Failure Recording (real, empirically confirmed)

**Issue**: At line 340, failures are appended twice:
- Once immediately during error handling (lines 243, 305)
- Once at completion (line 340)

**Impact**: MigrationState will contain duplicate entries for each failure.

### 2. State Counter Increment (double-counted — the earlier analysis here was wrong)

An earlier version of this document concluded that the immediate
`state.FailedObjects++` (lines 242, 304) plus the completion merge
`state.FailedObjects += result.FailedObjects` (line 338) were "correct because
they happen on different data". **That conclusion is incorrect.** The two
increments both count the *same* failure event of the *same* run:

- Line 242/304: the failure event increments `state.FailedObjects` (cumulative) immediately
- Line 338: the same failure event is inside `result.FailedObjects` (current run), which is added to the cumulative total again

Because `result.FailedObjects` is then **overwritten** with
`fm.state.FailedObjects` at the end of `Migrate()` (line 346, cumulative
reporting), the caller sees every failure from this run twice.

**Empirical proof** (2026-09-02, source at `82b0135c`):

```
$ go test ./internal/server/ -run 'TestFormatMigrationFailureRecording'
    format_migration_test.go:652: Expected 1 failed object, got 2
    format_migration_test.go:656: Expected 1 failure record, got 2
```

The duplicate failure-list entries (issue 1) and the doubled counter (issue 2)
are the same defect: the dual-append pattern plus the completion merge counts
each failure event twice, in both the counter and the list.

**Fix direction**: record failures to `result` only at the failure sites;
let the completion merge (lines 336-340) be the single writer of state.
This is what the working tree does as of 2026-09-02 and
`TestFormatMigrationFailureRecording` passes with it. The trade-off is that
state no longer contains failures recorded after the last periodic save if the
run is interrupted — see "Residual gaps" below.

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
- `backend.go:353-354` — `if am.WrappedDEK == nil { return nil, false }`

The function therefore conflates two very different conditions — "not
ARMOR-encrypted at all" and "ARMOR-encrypted but with corrupt key material" —
into a single `ok == false`, and the Migrate loop treated both as a skip. The
object incremented `SkippedObjects`, `migrateObject` was never called, and no
failure was recorded.

**Bug location**: neither the state counter, the failure list, nor error
propagation — it was upstream *classification*. **Fixed by** `8c0f3be0`, which
extracts the version from the raw `x-amz-meta-armor-version` header even when
metadata parsing fails (current `format_migration.go:247-274`) and lets
ARMOR-versioned objects with unparseable metadata proceed to migration
(`format_migration.go:288-292`, "attempting migration (will fail)"), and by
`61cbf4ad`, which added explicit base64 validation at the top of
`migrateObject` (`format_migration.go:357-378`) so the recorded reason is
precise ("invalid base64 in wrapped DEK") rather than a confusing downstream
decryption error.

### Symptom 2 (at `82b0135c`): "Expected 1 failed object, got 2"

After the classification fix, the object reaches `migrateObject`, which fails
on the explicit base64 validation (`format_migration.go:369`), and the error
propagates correctly to the recording block. The failure is then recorded
**twice** — see "Potential Issues → 2" above: once by the immediate
`fm.state.FailedObjects++` / `fm.state.Failures` append (lines 242-243 /
304-305) and once by the completion merge (lines 338, 340), with the doubled
total reported back at line 346.

### Where the recording path stands after both fixes

- `format_migration.go:236-241` (metadata fetch failure) and `:294-299`
  (migration failure): record to `result` only.
- `format_migration.go:330, 332` (completion merge): the single writer of
  `state.FailedObjects` / `state.Failures`.
- `format_migration.go:337-346`: `result` fields are overwritten with
  cumulative state totals before return, so a resumed run with zero new
  failures still reports the earlier run's failures. Intended for the
  cumulative API; worth remembering when writing assertions.

### Residual gaps (not regressions, but open)

1. **Stale comment**: `recordFailure` (`format_migration.go:856-859`) still
   claims the record is "also immediately appended to fm.state.Failures to
   ensure persistence even if the migration is interrupted before completion".
   That is no longer true — persistence now happens only at the completion
   merge.
2. **Interruption loses failures**: the `ctx.Done()` path (~lines 188-201)
   returns without merging `result.Failures` (or `result.FailedObjects`) into
   `fm.state`, and `saveState` persists only `fm.state` — so failures recorded
   since the last periodic save are absent from `.armor/migration-state.json`
   if the run is interrupted.

## Summary

The format migration failure recording mechanism:

1. ✅ **Records failures** in `result` at the failure site; state written once at completion
2. ✅ **Persisted to disk** via `.armor/migration-state.json`
3. ✅ **Thread-safe** using mutex locking
4. ✅ **Corrupted metadata objects are migration failures, not skips** (since `8c0f3be0` + `61cbf4ad`)
5. ⚠️ **Failures since the last periodic save are lost on interruption** (see Residual gaps)
6. ⚠️ **`recordFailure` doc comment is stale** (see Residual gaps)
