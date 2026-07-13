# Bead bf-pr810: Fix validator_test.go parameter issues

## Task Description
Fix NewValidationError calls in validator_test.go lines 833, 856-857. Add missing expectedType and actualType parameters. Also fix undefined ErrCodeDeprecatedFeature on line 609.

## Investigation Results

Upon investigation, the issues described in this bead have already been resolved in commit 93d4bc81 ("test(yamlutil): Update error constructor calls with type parameters") on 2026-07-13 at 12:01:17 PM EDT.

### Changes Made in Commit 93d4bc81

1. **Line 608**: Fixed to use `ErrCodeDuplicateKey` instead of empty string, and added missing parameters:
   - Before: `*NewValidationError("", "Duplicate key detected", "", "", "", 5, 0, ErrorTypeStructure, "")`
   - After: `*NewValidationError("", "Duplicate key detected", "", "", ErrCodeDuplicateKey, 5, 0, ErrorTypeStructure, "", "", "")`

2. **Line 609**: Fixed to use `ErrCodeValidationFailed` instead of empty string, and added missing parameters:
   - Before: `*NewValidationError("", "Deprecated YAML feature", "", "", "", 10, 0, ErrorTypeValidation, "")`
   - After: `*NewValidationError("", "Deprecated YAML feature", "", "", ErrCodeValidationFailed, 10, 0, ErrorTypeValidation, "", "", "")`

3. **Line 833**: Fixed to use `ErrCodeValidationFailed` and added missing parameters:
   - Before: `*NewValidationError("test.yaml", "Test warning", "", "", "", 1, 0, ErrorTypeStructure, "")`
   - After: `*NewValidationError("test.yaml", "Test warning", "", "", ErrCodeValidationFailed, 1, 0, ErrorTypeStructure, "", "", "")`

4. **Lines 856-857**: Fixed to use `ErrCodeValidationFailed` and added missing parameters:
   - Before: `*NewValidationError("test.yaml", "Warning 1", "", "", "", 0, 0, ErrorTypeStructure, "")`
   - After: `*NewValidationError("test.yaml", "Warning 1", "", "", ErrCodeValidationFailed, 0, 0, ErrorTypeStructure, "", "", "")`
   - Before: `*NewValidationError("test.yaml", "Warning 2", "", "", "", 0, 0, ErrorTypeStructure, "")`
   - After: `*NewValidationError("test.yaml", "Warning 2", "", "", ErrCodeValidationFailed, 0, 0, ErrorTypeStructure, "", "", "")`

### Note on ErrCodeDeprecatedFeature
The task mentioned fixing an "undefined ErrCodeDeprecatedFeature" on line 609, but this specific error code does not exist in the codebase. The actual issue was that line 609 was using an empty string `""` as the error code parameter instead of a valid error code. The fix correctly uses `ErrCodeValidationFailed`.

## Verification

All affected tests pass:
- `TestValidator_WarningSummary` ✓
- `TestWarningSummary_WithWarnings` ✓
- `TestWarningSummary_MultipleWarnings` ✓

The code compiles successfully with no errors.

## Conclusion

The bead's requirements have already been satisfied by commit 93d4bc81. No additional work is required.
