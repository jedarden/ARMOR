# BF-3BNPE: Update NewValidationError calls to pass path parameter

## Status: Already Completed

This task was already completed in commit d8d59082:
- "test(bf-3bnpe): update NewValidationError calls to pass path parameter in example and formatting tests"

## Verification

Verified on 2026-07-12 that all acceptance criteria are met:

### 1. All NewValidationError calls pass path parameter ✓
- `internal/yamlutil/error_message_format_examples_test.go`: 8 calls - all pass path parameter
- `internal/yamlutil/verify_formatting_test.go`: 2 calls - all pass path parameter  
- `internal/yamlutil/validation_error_demo_test.go`: 3 calls - all pass path parameter

Total: 13 NewValidationError calls across 3 files

### 2. Path values reflect actual validation error location ✓
- When fieldPath is available, it is used as the path parameter
- When fieldPath is empty, filePath is used as fallback
- Examples:
  - `NewValidationError(..., "server.port", ..., "server.port")` - uses fieldPath
  - `NewValidationError(..., "", ..., "config.yaml")` - uses filePath when no fieldPath

### 3. Tests still pass after changes ✓
All tests in the affected files pass:
- `TestErrorFormattingExamples` - PASS
- `TestValidationErrorDemo` - PASS
- All ValidationError formatting tests - PASS

## Changes Made in Original Commit

Commit d8d59082 made the following changes:
- Updated line 307 in `error_message_format_examples_test.go`
- Updated line 112 in `verify_formatting_test.go`
- Both changes added the path parameter to NewValidationError calls

No additional changes needed.
