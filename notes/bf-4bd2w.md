# Task bf-4bd2w: Update ValidationError in errors_test.go and validator_test.go

## Status: Already Completed

This task was already completed in previous commits:
- `b795b194`: Replaced 4 ValidationError struct constructions in validator_test.go
- `2c8e1e49`: Replaced 3 ValidationError struct constructions in errors_test.go

## Verification

Current state of both test files:
- `internal/yamlutil/errors_test.go`: 13 NewValidationError() calls
- `internal/yamlutil/validator_test.go`: 5 NewValidationError() calls

No direct `ValidationError{...}` struct initializations remain in either file.

## Acceptance Criteria Met

✅ All 9 ValidationError constructions across both files use NewValidationError()
✅ Files compile without errors
✅ No test logic changed, only construction syntax

The task was completed prior to this bead being assigned.
