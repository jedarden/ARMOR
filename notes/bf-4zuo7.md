# Task bf-4zuo7: TypeMismatchError and SchemaValidationError Constructor Replacements

## Status: Already Completed

This task requested replacement of direct struct initializations with constructor calls in `internal/yamlutil/errors_test.go`:
- 3 TypeMismatchError instances → `NewTypeMismatchError()`
- 2 SchemaValidationError instances → `NewSchemaValidationError()`

## Previously Completed By

### TypeMismatchError (bf-3sh3e)
- Commit: `426fd0d0`
- Date: 2026-07-12 15:19:55
- Replaced 3 instances in `errors_test.go`
- Also replaced 1 instance in `debug_helpers_test.go`

### SchemaValidationError (bf-46pf0)
- Commit: `93f704b0`
- Date: 2026-07-12 14:59:20
- Replaced 2 instances in `errors_test.go`
- Also documented that ValidationError and FileError replacements were already complete

## Verification

All acceptance criteria met:
- ✅ All 3 TypeMismatchError constructions use `NewTypeMismatchError()`
- ✅ All 2 SchemaValidationError constructions use `NewSchemaValidationError()`
- ✅ File compiles (`go build ./internal/yamlutil/...`)
- ✅ Tests pass (`go test ./internal/yamlutil/...`)
- ✅ No test logic changed

No direct struct initializations (`&TypeMismatchError{` or `&SchemaValidationError{`) remain in the file.
