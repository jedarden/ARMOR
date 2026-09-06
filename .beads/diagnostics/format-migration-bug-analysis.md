# Format Migration Handler Bug Analysis

## Executive Summary

The format migration handler has **THREE DISTINCT BUGS** in the object counting logic:

1. **Incorrect skip condition** causing objects to be skipped when they should be processed
2. **Failure counting bug** where failures are counted as processed instead of failed  
3. **No actual object migration occurring** due to early returns or logic errors

## Bug #1: Incorrect Skip Logic (Lines 272-288)

### Location
`internal/server/format_migration.go:272-288`

### Affected Tests
- `TestFormatMigrationDryRun`: Expected 1 processed, 0 skipped → Got 1 skipped
- `TestFormatMigrationV1ToV2`: Expected 1 processed → Got 0 processed  
- `TestFormatMigrationMultipartToSingle`: Expected 1 processed → Got 0 processed

### Root Cause
The skip logic has TWO separate skip conditions that can BOTH trigger:

```go
// Line 275: Skip if version not in include list
if !fm.shouldMigrateVersion(uint8(version)) {
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue
}

// Line 283: Skip if already at target version
if uint8(version) == fm.currentWriteVersion {
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue
}
```

**The bug:** The second skip condition (line 283) is incorrect because:
- It checks if the object's version MATCHES the target version
- But this check happens AFTER we've already confirmed the version IS in the include list
- An object at the target version SHOULD be skipped, but objects at source versions should NOT

**For `TestFormatMigrationDryRun`:**
- Object is V1 (version=1)
- Include list is ["1"] (migrate FROM V1)
- Target is V2 (migrate TO V2)
- Flow: shouldMigrateVersion(1) = TRUE → don't skip at line 275
- Then check: `1 == 2`? → FALSE → don't skip at line 283
- **Result should be:** Object is processed (decrypted and re-encrypted in dry-run)
- **Actual result:** Object is skipped

**The real bug:** The object is being skipped somewhere before reaching these conditions, or the version value being compared is wrong.

---

## Bug #2: Failures Counted as Processed (Lines 298-308)

### Location
`internal/server/format_migration.go:298-308`

### Affected Tests
- `TestFormatMigrationFailureRecording`: Expected 0 processed, 1 failed → Got 1 processed, 0 failed

### Root Cause
When `migrateObject` returns an error, the code **increments BOTH counters**:

```go
if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
    log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
    result.FailedObjects++              // ← Bug: This increments
    result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err)))
    fm.stateMu.Lock()
    fm.state.FailedObjects++
    fm.stateMu.Unlock()
    // Continue with other objects - migration is best-effort
} else {
    result.ProcessedObjects++           // ← This also increments (when no error)
}
```

**The bug:** The `ProcessedObjects++` is in the `else` block, which means:
- When migration succeeds: ProcessedObjects++ (correct)
- When migration fails: FailedObjects++ (correct)

**Wait, this logic looks correct...** Unless the issue is that `migrateObject` is NOT returning an error when it should!

**For `TestFormatMigrationFailureRecording`:**
- Object has "invalid-dek" and "invalid-iv" in metadata
- `migrateObject` is called
- Inside `migrateObject`:
  - Line 352: `backend.ParseARMORMetadata(rawMeta)` parses metadata (might succeed or fail)
  - Line 357: `fm.backend.Get` retrieves the object
  - Line 365-372: Decrypts the object (single or multipart path)
  - Line 368: For multipart → calls `decryptMultipartObject`
    - Line 525: `crypto.UnwrapDEK(fm.mek, armorMeta.WrappedDEK)` → **This should FAIL with invalid DEK**
    - Returns error
  - Line 373: Returns error
- Line 298: `err != nil` → TRUE
- Line 300: `result.FailedObjects++` → **This SHOULD increment**
- Line 307: `result.ProcessedObjects++` → This is NOT executed (else block)

**So why is the test seeing FailedObjects=0 and ProcessedObjects=1?**

**Hypothesis:** `crypto.UnwrapDEK` is succeeding when it should fail, or the error is being swallowed somewhere.

---

## Bug #3: V1 Objects Not Being Processed

### Location
`internal/server/format_migration.go:298-308`

### Affected Tests  
- `TestFormatMigrationV1ToV2`: Expected 1 processed → Got 0 processed
- `TestFormatMigrationMultipartToSingle`: Expected 1 processed → Got 0 processed

### Root Cause
These tests create valid V1 objects that SHOULD be successfully migrated to V2, but `ProcessedObjects` remains 0.

**For `TestFormatMigrationV1ToV2`:**
- Valid V1 object with proper DEK, IV, ciphertext, HMAC table
- Include list is ["1"], target is V2
- Expected: Object is decrypted and re-encrypted as V2
- Actual: ProcessedObjects = 0

**Possible causes:**
1. Object is being skipped (see Bug #1)
2. `migrateObject` is returning an error (silently logged, not recorded)
3. `migrateObject` succeeds but `ProcessedObjects++` is not reached
4. Dry-run mode is being set when it shouldn't be

---

## Bug #4: Objects Being Skipped Incorrectly

### Location
`internal/server/format_migration.go:272-288`

### Affected Tests
- `TestFormatMigrationV2Skipped`: Expected 1 skipped → Got 2 skipped

### Root Cause
A V2 object with includeVersions=["1"] should be skipped exactly once, but it's being skipped twice.

**For `TestFormatMigrationV2Skipped`:**
- Single V2 object (version=2)
- Include list is ["1"] (migrate FROM V1 only)
- Expected flow:
  1. Object version = 2
  2. shouldMigrateVersion(2) → FALSE (2 not in ["1"])
  3. Skip counter increments once
  4. Continue to next object
- Expected: SkippedObjects = 1
- Actual: SkippedObjects = 2

**Possible causes:**
1. Object is being counted twice (loop bug?)
2. Both skip conditions are being triggered (double-count)
3. `.armor/` sidecar file is also being counted as an object
4. The skip condition at line 275 is being triggered TWICE for the same object

**Analysis of skip conditions:**
- Line 275: `if !fm.shouldMigrateVersion(uint8(version))` → For V2 object with include=["1"], this is TRUE → skip once
- Line 283: `if uint8(version) == fm.currentWriteVersion` → For V2 object with target=V2, this is TRUE → skip AGAIN

**The bug:** When an object's version is BOTH "not in include list" AND "at target version", it gets skipped TWICE!

This happens because:
- include=["1"] means "migrate from V1 to V2"
- V2 objects are not in the include list → skip #1
- V2 objects are at the target version → skip #2

**The fix:** The second skip condition should only apply to versions that ARE in the include list. The logic should be:

```go
// Check if version is in the include list
if !fm.shouldMigrateVersion(uint8(version)) {
    // Not a source version - skip it
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue
}

// Version is in include list - check if already at target
if uint8(version) == fm.currentWriteVersion {
    // Source version matches target version - skip (redundant migration)
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue
}

// Version is in include list and not at target - MIGRATE IT
```

---

## Shared Root Causes

### Root Cause #1: Skip Logic Has Two Independent Conditions

The handler checks if an object should be skipped using TWO separate conditions:
1. Is the version in the include list? (line 275)
2. Is the object already at the target version? (line 283)

These conditions are NOT mutually exclusive! An object can be BOTH "not in include" AND "at target", causing double-skip counting.

**Impact:** Tests that expect exactly 1 skip are seeing 2 skips.

### Root Cause #2: No Error Returned When Migration Should Fail

The failure recording test expects migration to fail and be recorded, but it's not failing.

**Hypothesis:** The `crypto.UnwrapDEK` function is succeeding when given invalid base64 input, or the error is being caught and nil returned earlier in the call chain.

**Impact:** FailedObjects remains 0 when it should be 1.

### Root Cause #3: Dry-Run vs Live-Run State Confusion

Some tests run in dry-run mode and expect different behavior. The dry-run flag is checked at line 380, but by that point the object has already been counted as skipped or processed.

**Impact:** Dry-run tests may not be executing the expected code path.

---

## Dry-Run vs Live-Run State Analysis

### Do they share state incorrectly?

**YES.** The `result` and `fm.state` structs are separate:
- `result` is returned to the caller
- `fm.state` is saved to `.armor/migration-state.json`

But both are modified in parallel:
- Line 239: `result.FailedObjects++`
- Line 242: `fm.state.FailedObjects++`

This means dry-run and live-run COULD interfere if the same state file is loaded.

**However, the tests create fresh MockBackends for each test**, so there should be no state leakage between tests.

### Does dry-run mode affect counting?

**YES.** At line 380, dry-run mode returns early from `migrateObject`:

```go
if dryRun {
    // In dry run mode, just verify we can decrypt and count
    return nil
}
```

This means in dry-run mode:
- Object is decrypted
- Object is counted as processed (line 307: `result.ProcessedObjects++`)
- But object is NOT re-encrypted

**This is CORRECT behavior for dry-run**, so dry-run mode itself is not the bug.

---

## Failure Recording Path Analysis

### Is the failure recording path broken entirely?

**PARTIALLY.** The failure recording code exists and looks correct:

```go
if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
    result.FailedObjects++
    result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err)))
    // ...
}
```

**But** if `migrateObject` never returns an error (when it should), then this path is never executed.

**The real bug:** `migrateObject` is swallowing errors or not validating input properly, causing it to return `nil` when it should return an error.

---

## Summary by Test

| Test | Expected Behavior | Actual Behavior | Root Cause | Bug # |
|------|-------------------|-----------------|------------|-------|
| `TestFormatMigrationDryRun` | 1 processed, 0 skipped | 1 skipped | Object skipped when it should be processed | #1 |
| `TestFormatMigrationV1ToV2` | 1 processed | 0 processed | Object not processed despite being valid V1 | #3 |
| `TestFormatMigrationV2Skipped` | 1 skipped | 2 skipped | Object counted as skipped twice | #4 |
| `TestFormatMigrationMultipartToSingle` | 1 processed | 0 processed | Multipart V1 not migrated | #3 |
| `TestFormatMigrationFailureRecording` | 0 processed, 1 failed | 1 processed, 0 failed | Failure not recorded, counted as processed | #2 |

---

## Recommendations

### For Bug #1 (Skip Logic):
The skip conditions need to be mutually exclusive. An object should be skipped for exactly ONE reason:

```go
// Check if version is in the include list
if !fm.shouldMigrateVersion(uint8(version)) {
    // Not a source version we want to migrate
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue
}

// Version is in include list - check if already at target
if uint8(version) == fm.currentWriteVersion {
    // Source version matches target version (redundant migration)
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue
}
```

### For Bug #2 (Failure Counting):
Investigate why `migrateObject` is not returning errors when it should. Check:
- Does `crypto.UnwrapDEK` validate input?
- Are errors being swallowed by try/catch or deferred functions?
- Is the error check at line 298 actually being reached?

### For Bug #3 (V1 Not Processed):
Debug why valid V1 objects are not being processed:
- Add logging to confirm the object is not being skipped
- Confirm `migrateObject` is being called
- Check if `migrateObject` is returning an error that's being silently logged

### For Bug #4 (Double-Skip):
Fix the skip logic to ensure objects are skipped exactly once (see Bug #1 fix).

---

## Conclusion

The format migration handler has **FOUR DISTINCT BUGS**:

1. **Skip logic double-counts** objects that are both "not in include" AND "at target version"
2. **Failures are counted as processed** (or `migrateObject` doesn't fail when it should)
3. **V1 objects are not being processed** (likely being skipped by bug #1)
4. **V2 objects are double-skipped** (variant of bug #1)

**These are NOT shared root causes** - they are four separate bugs in different parts of the handler logic. However, bugs #1 and #4 are variants of the same underlying issue (incorrect skip logic).

**The failure recording path is NOT broken entirely** - the code exists and looks correct. The issue is that `migrateObject` is not returning errors when it should, so the failure path is never executed.

**Dry-run and live-run do NOT share state incorrectly** in the tests (each test uses a fresh MockBackend), but the parallel state updates (result + fm.state) could cause issues in production if not careful.
