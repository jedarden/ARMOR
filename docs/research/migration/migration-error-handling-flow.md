# Migration Error Handling Flow

## Overview

This document traces the complete error handling flow for format migration failures, specifically focusing on how corrupted or invalid objects are detected, recorded, and handled.

## The Error Flow

### 1. Migrate() Main Loop (format_migration.go:256-315)

The main migration loop processes each object and handles errors:

```go
// Line 295
if err := fm.migrateObject(ctx, obj, rawMeta, dryRun); err != nil {
    log.Printf("Warning: failed to migrate %s: %v", obj.Key, err)
    result.FailedObjects++                                          // ✅ Increments failure counter
    result.Failures = append(result.Failures, fm.recordFailure(     // ✅ Records failure details
        obj.Key, 
        fmt.Sprintf("migration failed: %v", err)
    ))
    // Continue with other objects - migration is best-effort
} else {
    // Migration succeeded (including dry-run mode) - increment counter
    result.ProcessedObjects++
}
```

**Key behavior:** 
- `result.FailedObjects` is incremented (line 297)
- `recordFailure()` is called and the result is appended to `result.Failures` (line 298)
- Processing continues for remaining objects (best-effort migration)

### 2. recordFailure() (format_migration.go:827-833)

Creates a structured failure record:

```go
func (fm *FormatMigrator) recordFailure(key, reason string) MigrationFailure {
    return MigrationFailure{
        Key:    key,                                    // ✅ Records which object failed
        Reason: reason,                                // ✅ Records why it failed
        Time:   time.Now(),                            // ✅ Records when it failed
    }
}
```

**Key behavior:**
- Creates a `MigrationFailure` struct with all diagnostic information
- Returns the struct to be appended to the failures list

### 3. migrateObject() Error Return Points (format_migration.go:347-406)

The `migrateObject()` function can return errors at multiple points:

```go
func (fm *FormatMigrator) migrateObject(ctx context.Context, obj backend.ObjectInfo, rawMeta map[string]string, dryRun bool) error {
    // Parse ARMOR metadata
    armorMeta, ok := backend.ParseARMORMetadata(rawMeta)
    if !ok {
        return fmt.Errorf("object %s is not ARMOR-encrypted", obj.Key)  // ✅ Return point 1
    }

    // Get the object content
    reader, _, err := fm.backend.Get(ctx, fm.bucket, obj.Key)
    if err != nil {
        return fmt.Errorf("failed to get object: %w", err)              // ✅ Return point 2
    }
    defer reader.Close()

    // Decrypt the object
    var plaintext []byte
    isMultipart := rawMeta[armorMetaMultipart] == "true"
    if isMultipart {
        plaintext, err = fm.decryptMultipartObject(armorMeta, obj.Key, reader)
    } else {
        plaintext, err = fm.decryptSingleObject(armorMeta, reader)
    }
    if err != nil {
        return fmt.Errorf("failed to decrypt object: %w", err)         // ✅ Return point 3 (MAIN ONE FOR CORRUPTION)
    }

    // ... (re-encryption and upload) ...
    // More return points for upload failures
}
```

### 4. decryptSingleObject() Error Propagation (format_migration.go:451-499)

For single-PUT objects, the critical error path for corrupted data:

```go
func (fm *FormatMigrator) decryptSingleObject(armorMeta *backend.ARMORMetadata, reader io.Reader) ([]byte, error) {
    // Unwrap DEK - THIS IS WHERE THE CORRUPTED FIXTURE FAILS
    dek, err := crypto.UnwrapDEK(fm.mek, armorMeta.WrappedDEK)
    if err != nil {
        return nil, fmt.Errorf("failed to unwrap DEK: %w", err)        // ✅ Invalid base64 returns here
    }

    // Read envelope header - OR HERE if data is corrupted
    headerBuf := make([]byte, crypto.HeaderSize)
    if _, err := io.ReadFull(reader, headerBuf); err != nil {
        return nil, fmt.Errorf("failed to read envelope header: %w", err)
    }

    header, err := crypto.DecodeHeader(headerBuf)
    if err != nil {
        return nil, fmt.Errorf("failed to decode envelope header: %w", err) // ✅ Or here
    }
    // ... decryption continues ...
}
```

## The Corrupted Fixture Test

### Test Setup (format_migration_test.go:628-638)

```go
// Create a corrupted object that will fail migration
metadata := map[string]string{
    "x-amz-meta-armor-version":    "1",
    "x-amz-meta-armor-wrapped-dek": "invalid-dek",  // ❌ Invalid base64
    "x-amz-meta-armor-iv":         "invalid-iv",    // ❌ Invalid base64
}

mockBackend.objects["corrupted.dat"] = &MockObject{
    Data:     []byte("corrupted data"),
    Metadata: metadata,
}
```

### Expected Failure Path

For this corrupted object:

1. **Migrate() loops over objects** → finds "corrupted.dat" (line ~220-240)
2. **Version parsing succeeds** → version "1" is valid (lines 256-268)
3. **Version check passes** → V1 is in the include list (line 273)
4. **migrateObject() is called** (line 295)
5. **ParseARMORMetadata succeeds** → returns metadata struct (line 349)
6. **backend.Get succeeds** → returns the mock object (line 355)
7. **decryptSingleObject is called** (line 369)
8. **unwrapDEK FAILS** → "invalid-dek" is not valid base64 (line 453-456)
   ```go
   dek, err := crypto.UnwrapDEK(fm.mek, armorMeta.WrappedDEK)
   if err != nil {
       return nil, fmt.Errorf("failed to unwrap DEK: %w", err)
   }
   ```
9. **Error propagates back** → "failed to decrypt object: failed to unwrap DEK: ..." (line 372)
10. **migrateObject returns error** (line 372)
11. **Migrate() handles the error** (line 295-298):
    - `result.FailedObjects++` → becomes 1
    - `recordFailure("corrupted.dat", "migration failed: ...")` → creates failure record
    - Failure is appended to `result.Failures` list

### Test Verification (format_migration_test.go:650-665)

```go
// Verify failure was recorded
if result.FailedObjects != 1 {
    t.Errorf("Expected 1 failed object, got %d", result.FailedObjects)
}

if len(result.Failures) != 1 {
    t.Errorf("Expected 1 failure record, got %d", len(result.Failures))
}

if result.Failures[0].Key != "corrupted.dat" {
    t.Errorf("Expected failure for 'corrupted.dat', got: %s", result.Failures[0].Key)
}

if result.Failures[0].Reason == "" {
    t.Error("Expected failure reason to be recorded")
}
```

## Summary: Complete Failure Flow

```
Corrupted object (invalid base64 wrapped-dek)
    ↓
migrateObject() called
    ↓
decryptSingleObject() called
    ↓
crypto.UnwrapDEK() called with "invalid-dek"
    ↓
unwrapDEK FAILS (base64 decode error)
    ↓
decryptSingleObject returns: "failed to unwrap DEK: ..."
    ↓
migrateObject returns: "failed to decrypt object: failed to unwrap DEK: ..."
    ↓
Migrate() loop catches error (line 295)
    ↓
result.FailedObjects++ (line 297)
    ↓
recordFailure() creates MigrationFailure{Key: "corrupted.dat", Reason: "...", Time: now} (line 298)
    ↓
MigrationFailure appended to result.Failures (line 298)
    ↓
Processing continues with next object
```

## Error Is Not Swallowed

Based on the code analysis, **the error is NOT swallowed**. The complete flow shows:

✅ `unwrapDEK()` returns an error for invalid base64
✅ `decryptSingleObject()` propagates the error
✅ `migrateObject()` propagates the error
✅ `Migrate()` catches the error, increments `FailedObjects`, and calls `recordFailure()`
✅ `recordFailure()` creates a structured record in the failures list

## Where Could Errors Be Lost?

The current implementation is correct and does not swallow errors. However, if someone were to modify the code, errors could be lost if:

1. **A middleware function swallows the error** - if decryptSingleObject or decryptMultipartObject were modified to log and return nil instead of propagating
2. **The error check is removed** - if line 295's `if err != nil` check were removed or commented out
3. **A panic is caught and not re-thrown** - if a panic handler exists that catches panics and continues without recording them

**Hypothesis:** The failure flow is working as designed. The test verifies that corrupted objects are properly recorded in the failure list and do not cause the migration to abort prematurely.
