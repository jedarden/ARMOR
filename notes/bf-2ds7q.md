# Bead bf-2ds7q: Fix verify_error_formatting_test.go parameters

## Task
Fix NewValidationError calls in verify_error_formatting_test.go lines 28 and 72.
Add missing ErrorType and expectedType, actualType parameters.

## Status
**ALREADY FIXED** - This issue was resolved in commit `93d4bc81` on 2026-07-13.

## Verification
- Line 28: `NewValidationError` call has all required parameters including `ErrorTypeValidation`, path, expectedType (""), and actualType ("")
- Line 72: `NewValidationError` call has all required parameters including `ErrorTypeValidation`, path, expectedType (""), and actualType ("")

## Test Results
```
✓ AC2: ValidationError includes field path + constraint
  Example: validation error in deployment.yaml at line 15, column 12 at field spec.replicas: port out of range (constraint: must be between 1-65535)
```

The test `TestAcceptanceCriteria_ContextualErrorFormatting` passes successfully.

## Related
- Commit: 93d4bc81 - "test(yamlutil): Update error constructor calls with type parameters"
