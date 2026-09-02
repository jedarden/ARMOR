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

### 1. Duplicate Failure Recording

**Issue**: At line 340, failures are appended twice:
- Once immediately during error handling (lines 243, 305)
- Once at completion (line 340)

**Impact**: MigrationState will contain duplicate entries for each failure.

### 2. State Counter Increment

**Current behavior**:
- `state.FailedObjects` is incremented immediately (lines 242, 304)
- `state.FailedObjects` is also incremented at completion (line 338)

**Impact**: The counter would be double-counted if the immediate increments weren't already cumulative. However, examining the code shows that the immediate increments ARE cumulative (they add to the running state total), so line 338 would create an incorrect cumulative total if it weren't for the fact that `result.FailedObjects` tracks ONLY the current run's failures.

**Actually, looking closer**:
- `result.FailedObjects` = failures in THIS run (resets to 0 at line 182)
- `state.FailedObjects` = cumulative failures across ALL runs
- Line 242/304: `state.FailedObjects++` increments cumulative total
- Line 338: `state.FailedObjects += result.FailedObjects` adds current run to cumulative

This is CORRECT because the immediate increments and the completion increment happen on different data:
- Immediate: increment state counter directly
- Completion: add result counter to state counter

## Summary

The format migration failure recording mechanism:

1. ✅ **Records failures immediately** in both result and state
2. ✅ **Persisted to disk** via `.armor/migration-state.json`
3. ✅ **Thread-safe** using mutex locking
4. ⚠️ **Duplicate entries**: Failures are appended to state twice (immediate + at completion)
5. ✅ **Cumulative counts**: State tracks failures across runs, result tracks current run only
