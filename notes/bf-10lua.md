# Bead bf-10lua: Update NewValidationError callers in remaining test files

## Summary

Verified that all NewValidationError calls in the three target test files have been updated with appropriate path parameters.

## Files Verified

### 1. internal/yamlutil/verify_error_formatting_test.go
- Line 28: `NewValidationError(..., "spec.replicas")` ✓
- Line 72: `NewValidationError(..., "spec.replicas")` ✓

### 2. internal/yamlutil/result_types_test.go
- Line 424: `NewValidationError(..., "server.name")` ✓
- Line 463: `NewValidationError(..., "")` ✓ (generic validation error with empty path)
- Line 548: `NewValidationError(..., "server.port")` ✓

### 3. internal/yamlutil/validation_error_demo_test.go
- Lines 15-25: `NewValidationError(..., "server.port")` ✓
- Lines 31-41: `NewValidationError(..., "spec.template.spec.containers[0].image")` ✓
- Lines 47-57: `NewValidationError(..., "spec.replicas")` ✓

## Acceptance Criteria Met

- ✅ All calls across the 3 files pass path parameter
- ✅ Path values are contextually appropriate (field paths or empty for generic errors)
- ✅ Tests compile and pass (all yamlutil tests pass)

## Notes

This work was completed in previous commits:
- `d43aeb59 test(result_types_test): update NewValidationError call to pass path parameter`
- `b2511939 test(yamlutil): update NewValidationError callers to pass path parameter`

This bead verified that all remaining test files were properly updated.
