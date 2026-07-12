# Task bf-3h6pp: Already Completed

## Task Description
Replace all 14 direct FileError struct initializations in `internal/yamlutil/file_test.go` with NewFileError() constructor calls.

## Status: Already Completed

This work was already completed in commit `95a428a9` on 2026-07-12:
```
95a428a9 test: replace FileError constructions with NewFileError() constructor
```

## Verification

- Confirmed 14 `NewFileError()` calls exist in `internal/yamlutil/file_test.go`
- No direct `&FileError{...}` constructions found
- All file_test.go tests pass successfully
- No file changes required

## Acceptance Criteria (All Met)
- ✅ All 14 FileError constructions use NewFileError()
- ✅ Tests compile and pass
- ✅ No test logic changed (work was done in a previous commit)
