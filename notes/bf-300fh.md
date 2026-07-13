# Bead bf-300fh: Fix Direct Field Access in errors_test.go

## Status: Already Completed

## Findings

The direct field access issues at errors_test.go lines 796-797 were **already fixed** in commit `ed582fa8` (Mon Jul 13 11:19:30 2026).

### Before (from commit message):
```go
err := NewValidationError(
    "config.yaml",
    tt.message,
    tt.fieldPath,
    "must be valid",
    ErrCodeInvalidValue,
    10,
    5,
    ErrorTypeValidation,
    tt.fieldPath,
)
// Set type information directly
err.ExpectedType = tt.expectedType
err.ActualType = tt.actualType
```

### After (current state):
```go
err := NewValidationError(
    "config.yaml",
    tt.message,
    tt.fieldPath,
    "must be valid",
    ErrCodeInvalidValue,
    10,
    5,
    ErrorTypeValidation,
    tt.fieldPath,
    tt.expectedType,  // Now passed to constructor
    tt.actualType,    // Now passed to constructor
)
```

## Verification

- ✅ No direct field access at lines 796-797 (already using constructor)
- ✅ Field initialization through constructor parameters (correctly implemented)
- ✅ Pattern matches guidance from bf-558ti (error construction patterns)

## Notes

The constructor signature for `NewValidationError` was updated to include `expectedType` and `actualType` as parameters, enforcing proper initialization through the constructor rather than post-creation field assignment.

This bead was created based on an older state of the codebase before the fix was committed.
