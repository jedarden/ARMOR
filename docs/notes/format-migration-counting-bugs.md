# Format Migration Counting Bugs - Root Cause Analysis

## Overview

Investigation of 5 test failures in format migration counting logic, examining the counter behavior in `internal/server/format_migration.go` against test expectations in `internal/server/format_migration_test.go`.

## Critical Bug #1: Dry Run Mode Path Missing Counter Increment

**Test:** `TestFormatMigrationDryRun`  
**Expected:** 1 processed object, 0 skipped objects  
**Actual:** 0 processed objects, 1 skipped object

**Root Cause:** In `migrateObject()` (lines 340-441), when `dryRun == true`, the function returns `nil` at line 373:

```go
if dryRun {
    // In dry run mode, just verify we can decrypt and count
    return nil
}
```

This return happens BEFORE the `result.ProcessedObjects++` increment at line 298 in `Migrate()`. However, reaching the `migrateObject()` call at line 289 means the object passed all filters:
- Has ARMOR metadata ✓
- Version in include list ✓  
- Passed metadata parsing ✓

**The Fix:** The early return in dry-run mode bypasses the counter increment at line 298. The dry-run success path needs to return a success indicator, or the counter increment needs to happen before the dry-run check.

**Code Reference:** `internal/server/format_migration.go:373`

---

## Critical Bug #2: V2 Objects Not Correctly Identified as Skip Candidates

**Test:** `TestFormatMigrationV2Skipped`  
**Expected:** 0 processed, 1 skipped  
**Actual:** 0 processed, 2 skipped (or incorrect count)

**Root Cause:** When checking versions at lines 252-279:

```go
if !fm.shouldMigrateVersion(uint8(version)) {
    result.SkippedObjects++
    fm.advanceCursor(obj.Key)
    continue
}
```

For V2 objects when `includeVersions = ["1"]`, `shouldMigrateVersion(2)` returns `false`, so they're counted as skipped. **This is actually correct behavior** - V2 objects should be skipped when only migrating V1→V2.

However, there may be an issue with V2 objects being counted as "V2" in one place but as something else in another, causing double-counting.

**Potential Issue:** The `countObjects()` function (lines 910-979) has similar logic but may count differently than the main `Migrate()` loop.

**Code Reference:** `internal/server/format_migration.go:275-278`

---

## Critical Bug #3: Multipart to Single-PUT Migration Not Counting Success

**Test:** `TestFormatMigrationMultipartToSingle`  
**Expected:** 1 processed object, version flipped to V2  
**Actual:** 0 processed objects (version not flipped)

**Root Cause:** In `migrateObject()`, when handling multipart objects (lines 356-366):

```go
isMultipart := rawMeta[armorMetaMultipart] == "true"
if isMultipart {
    // Multipart objects: load HMAC table from sidecar and decrypt
    plaintext, err = fm.decryptMultipartObject(armorMeta, obj.Key, reader)
```

Then during re-encryption (lines 376-399):

```go
if plaintextSize > fm.multipartThreshold() {
    // Use multipart upload for large objects
    err = fm.uploadAsMultipart(ctx, obj.Key, plaintext, plaintextSHA[:], rawMeta)
    ...
} else {
    // Re-encrypt as single-PUT with current write format
    ciphertext, newIV, newWrappedDEK, blockSize, mekFingerprint, err := fm.encryptAsSingle(plaintext)
```

The test object is ~29 bytes (well below 5MB threshold), so it should take the `else` branch. However:

1. The `buildNewMetadata()` call (line 392) may not preserve the `x-amz-meta-armor-multipart` flag when converting from multipart→single-PUT
2. OR the decryption of the multipart object is failing silently
3. OR the sidecar file (`.armor/hmac/<sha256>`) is not being found

Looking at the test (lines 586-591), the sidecar IS created. So the issue is likely in `decryptMultipartObject()` or in the metadata update path.

**Code Reference:** `internal/server/format_migration.go:356-399`

---

## Critical Bug #4: Failure Recording Not Populating Result Struct

**Test:** `TestFormatMigrationFailureRecording`  
**Expected:** 1 failed object, failure details populated  
**Actual:** 0 failed objects, empty failure list

**Root Cause:** In `Migrate()`, when metadata fetch fails (lines 236-246):

```go
rawMeta, err := fm.objectMetadata(ctx, obj)
if err != nil {
    log.Printf("Warning: failed to get metadata for %s: %v", obj.Key, err)
    result.FailedObjects++
    result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("failed to get metadata: %v", err)))
    fm.stateMu.Lock()
    fm.state.FailedObjects++
    fm.stateMu.Unlock()
    fm.advanceCursor(obj.Key)
    continue
}
```

This looks correct - it increments both `result.FailedObjects` and appends to `result.Failures`.

However, for the test setup (lines 628-638):

```go
metadata := map[string]string{
    "x-amz-meta-armor-version":    "1",
    "x-amz-meta-armor-wrapped-dek": "invalid-dek",
    "x-amz-meta-armor-iv":         "invalid-iv",
}
```

The object has valid ARMOR metadata structure but invalid VALUES. This means:
1. `objectMetadata()` succeeds (metadata exists)
2. `ParseARMORMetadata()` succeeds (structure is valid)
3. The object reaches `migrateObject()`
4. Inside `migrateObject()`, `crypto.UnwrapDEK()` fails with invalid wrapped DEK
5. Error is returned from `migrateObject()`
6. At line 289-296, the error IS caught and should increment `result.FailedObjects`

**The Issue:** The test creates a V1 object with invalid DEK values. When `migrateObject()` calls `decryptSingleObject()` → `crypto.UnwrapDEK()`, this fails with "invalid DEK" error. This error IS caught at lines 289-296.

**Wait - looking more closely at lines 289-299:**

```go
if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
    log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
    result.FailedObjects++
    result.Failures = append(result.Failures, fm.recordFailure(obj.Key, fmt.Sprintf("migration failed: %v", err)))
    fm.stateMu.Lock()
    fm.state.FailedObjects++
    fm.stateMu.Unlock()
    // Continue with other objects - migration is best-effort
} else {
    result.ProcessedObjects++
}
```

This SHOULD work. The fact that it doesn't suggests:
- Either `migrateObject()` is returning `nil` when it should return an error
- Or there's a panic/silent failure
- Or the test assertions are checking the wrong struct

**Code Reference:** `internal/server/format_migration.go:289-299`

---

## Critical Bug #5: State vs Result Counter Desynchronization

**Root Cause:** At the end of `Migrate()` (lines 318-326):

```go
fm.stateMu.Lock()
fm.state.Status = "completed"
fm.state.LastUpdated = time.Now()
fm.state.Failures = result.Failures
fm.state.ProcessedObjects = result.ProcessedObjects
fm.state.SkippedObjects = result.SkippedObjects
fm.state.FailedObjects = result.FailedObjects
fm.stateMu.Unlock()
```

The state is updated FROM the result. But during migration:
- `result` counters are incremented directly
- `state` counters are ALSO incremented directly in some places (lines 242, 294)
- At the end, `state` is overwritten with `result` values

This creates a situation where:
1. If `state.FailedObjects` is incremented during migration
2. Then `state.FailedObjects = result.FailedObjects` overwrites it
3. Any state saved during migration loses the increment

**The Fix:** Only increment ONE set of counters. Either:
- Increment only `result` counters and copy to `state` at the end (current pattern, but incomplete)
- OR increment only `state` counters and read from `state` for the result

**Code Reference:** `internal/server/format_migration.go:318-326`

---

## Summary

### Counters Shared Incorrectly
1. **Dry-run success path** bypasses `ProcessedObjects` increment
2. **State vs Result counters** are both incremented during migration, then state is overwritten

### Dry-run vs Live-run State Sharing
- Dry-run mode exits `migrateObject()` early (line 373) before success is counted
- This is the primary bug causing TestFormatMigrationDryRun to fail

### V2 Detection Issues  
- V2 objects ARE correctly identified as skip candidates (this is working)
- But may be double-counted between `countObjects()` and `Migrate()`

### Failure Recording
- Failure recording logic IS present and looks correct (lines 289-296, 239-246)
- Test failure suggests either:
  - `migrateObject()` returns nil instead of error for invalid DEK
  - OR test assertions are checking wrong result struct

### Version Flip Missing
- Multipart→SinglePUT migration may fail silently during decryption
- OR `buildNewMetadata()` doesn't properly update version flag
- Needs investigation in `decryptMultipartObject()` and metadata building

## Recommended Fix Priority

1. **HIGH:** Fix dry-run counter increment (move or duplicate the success counter before early return)
2. **HIGH:** Consolidate state/result counter updates (choose one source of truth)
3. **MEDIUM:** Verify `migrateObject()` error returns for all failure modes
4. **MEDIUM:** Debug multipart→singlePUT decryption path
5. **LOW:** Verify `countObjects()` matches `Migrate()` filtering logic
