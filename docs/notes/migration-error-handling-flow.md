# Migration Error Handling and Failure Recording Flow Analysis

## Overview
This document analyzes the error handling and failure recording flow in `internal/server/format_migration.go`, specifically focusing on the `Migrate()` function and how failures are tracked and persisted.

## Main Error Flow

### 1. Entry Point: Migrate() Function (lines 163-357)

The `Migrate()` function implements a **best-effort migration strategy** - it continues processing other objects even when individual objects fail.

### 2. Error Recording Points

There are **two primary error recording points** in the migration loop:

#### Error Point 1: Object Metadata Retrieval (lines 237-246)

```go
rawMeta, err := fm.objectMetadata(ctx, obj)
if err != nil {
    log.Printf("Warning: failed to get metadata for %s: %v", obj.Key, err)
    result.FailedObjects++
    result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("failed to get metadata: %v", err)))
    fm.stateMu.Lock()
    fm.state.FailedObjects++
    fm.state.Failures = append(fm.state.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("failed to get metadata: %v", err)))
    fm.stateMu.Unlock()
    fm.advanceCursor(obj.Key)
    continue
}
```

**What happens:**
- Increment `result.FailedObjects` counter (current run only)
- Create and append failure record to `result.Failures` (current run only)
- Acquire state mutex lock
- Increment `fm.state.FailedObjects` counter (cumulative)
- Append failure record to `fm.state.Failures` (cumulative)
- Release state mutex lock
- Advance cursor to continue with next object
- Continue processing (migration continues)

#### Error Point 2: Object Migration Failure (lines 299-306)

```go
if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
    log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
    result.FailedObjects++
    result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err)))
    fm.stateMu.Lock()
    fm.state.FailedObjects++
    fm.state.Failures = append(fm.state.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err)))
    fm.stateMu.Unlock()
    // Continue with other objects - migration is best-effort
} else {
    // Migration succeeded (including dry-run mode) - increment counter
    result.ProcessedObjects++
}
```

**What happens:**
- Increment `result.FailedObjects` counter (current run only)
- Create and append failure record to `result.Failures` (current run only)
- Acquire state mutex lock
- Increment `fm.state.FailedObjects` counter (cumulative)
- Append failure record to `fm.state.Failures` (cumulative)
- Release state mutex lock
- Continue processing (migration continues)

## Failure Recording Mechanism

### recordFailure() Function (lines 867-873)

```go
func (fm *FormatMigrator) recordFailure(key, reason string) MigrationFailure {
    return MigrationFailure{
        Key:    key,
        Reason: reason,
        Time:   time.Now(),
    }
}
```

**Key characteristics:**
- Creates a `MigrationFailure` struct with three fields:
  - `Key`: The object key that failed
  - `Reason`: Human-readable error message
  - `Time`: Timestamp of when the failure occurred
- Returns the failure record (caller is responsible for appending)

### MigrationFailure Struct (lines 79-84)

```go
type MigrationFailure struct {
    Key     string    `json:"key"`
    Reason  string    `json:"reason"`
    Time    time.Time `json:"time"`
    Details string    `json:"details,omitempty"`
}
```

**Note:** The `Details` field exists in the struct but is **not populated** in the current implementation.

## State Management

### Dual Counter Pattern

The migration maintains **two parallel failure tracking systems**:

1. **result.FailedObjects** - Current run only (in-memory)
2. **fm.state.FailedObjects** - Cumulative across runs (persisted to `.armor/migration-state.json`)

### Final State Update (lines 330-341)

At migration completion, the final state is updated:

```go
fm.stateMu.Lock()
fm.state.Status = "completed"
fm.state.LastUpdated = time.Now()
// Preserve cumulative counts from previous runs
// result.* contains only current run increments, state.* contains cumulative totals
fm.state.ProcessedObjects += result.ProcessedObjects
fm.state.SkippedObjects += result.SkippedObjects
fm.state.FailedObjects += result.FailedObjects
// Merge failures (append current run failures to existing list)
fm.state.Failures = append(fm.state.Failures, result.Failures...)
fm.stateMu.Unlock()
```

**Critical observation:** The state update happens **only at completion**. However, failures are recorded immediately when they occur (within the error handling blocks), so they're persisted even if migration is interrupted.

## migrateObject() Error Paths

The `migrateObject()` function (lines 360-487) can return errors at multiple points:

### Validation Errors (lines 363-391)
- Invalid v2 wrapped DEK format (line 371)
- Invalid base64 in wrapped DEK (line 377)
- Invalid base64 in IV (line 383)
- Not ARMOR-encrypted (line 390)

### Decryption Errors (lines 393-412)
- Failed to get object (line 396)
- Failed to decrypt multipart object (line 405)
- Failed to decrypt single object (line 411)

### Encryption Errors (lines 417-444)
- Failed to upload as multipart (line 428)
- Failed to encrypt as single (line 434)
- Failed to put migrated object (line 443)

### Verification Errors (lines 448-486)
- Failed to read back migrated object (line 450)
- Failed to get migrated object metadata (line 457)
- Migrated object metadata is invalid (line 463)
- Version not updated (line 469)
- Failed to decrypt migrated object for verification (line 475)
- SHA-256 mismatch after migration (line 482)

**All errors from migrateObject() are properly returned** and caught by the error handling in the main migration loop (lines 299-306).

## Potential Gaps and Issues

### 1. Duplicate Failure Records

**Issue:** The code records the same failure **twice**:
- Once in `result.Failures`
- Once in `fm.state.Failures`

**Current behavior:**
```go
result.Failures = append(result.Failures, fm.recordFailure(obj.Key, ...))
fm.state.Failures = append(fm.state.Failures, fm.recordFailure(obj.Key, ...))
```

This creates two separate `MigrationFailure` structs with potentially different timestamps (since `recordFailure()` calls `time.Now()`).

**Impact:** At migration completion (line 340), the failures are merged again:
```go
fm.state.Failures = append(fm.state.Failures, result.Failures...)
```

This means failures recorded during the run are duplicated in the final state.

### 2. State Persistence Timing

**Current behavior:** Failures are appended to `fm.state.Failures` immediately when they occur, but the state file is only saved periodically (every 100 processed objects).

**Gap:** If the migration crashes after recording a failure but before the next periodic save, the failure is lost. The state is not saved immediately after recording a failure.

**Recommendation:** Consider calling `fm.saveState(ctx)` immediately after recording a failure for critical migrations.

### 3. Missing Details Field

**Issue:** The `MigrationFailure.Details` field is never populated in the current implementation.

**Impact:** Failures only have the high-level error message (e.g., "migration failed: failed to decrypt object") but lack additional context that might help diagnose issues.

### 4. No Error Aggregation

**Current behavior:** Each failed object generates a separate failure record. There's no aggregation or summary of common failure patterns.

**Potential improvement:** Group failures by error type to provide better visibility into migration issues.

## Error Flow Summary

```
1. Object enters migration pipeline
   |
2. Get object metadata (fm.objectMetadata)
   |
   ├─ SUCCESS → Continue to version check
   │              |
   │              ├─ Should migrate → Call migrateObject()
   │              |                  |
   │              |                  ├─ SUCCESS → Increment ProcessedObjects
   │              |                  └─ ERROR → Record failure, continue
   │              |
   │              └─ Should skip → Increment SkippedObjects, continue
   |
   └─ ERROR → Record failure, increment FailedObjects, continue

```

## Conclusion

The current error handling flow:
- ✅ Properly returns errors from `migrateObject()`
- ✅ Records failures in both result and state
- ✅ Continues processing after failures (best-effort)
- ✅ Advances cursor to prevent re-processing failed objects
- ✅ Thread-safe with mutex protection
- ⚠️ Creates duplicate failure records
- ⚠️ May lose failures if crash occurs before periodic save
- ⚠️ Doesn't populate the Details field for better diagnostics

The error flow is **functionally correct** but has opportunities for improvement in failure record de-duplication, more immediate persistence, and richer diagnostic information.
