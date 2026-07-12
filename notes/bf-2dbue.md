# Task bf-2dbue: FileError Construction Replacement

## Status: Already Completed

This task was to replace `&FileError{...}` struct constructions with `NewFileError()` calls in the following test functions in `internal/yamlutil/file_test.go`:
- TestReadFileSymlinks (lines 412-531)
- TestReadFileEdgeCases (lines 533-615)
- TestFileExistsEdgeCases (lines 617-698)
- TestFileErrorMessageContent (lines 700-755)

## Investigation Results

All specified test functions already use `NewFileError()` constructor calls instead of direct `&FileError{...}` struct initialization:

### TestFileErrorMessageContent (lines 700-755)
```go
createError: func() *FileError {
    return NewFileError("/test/config.yaml", "read", "", os.ErrNotExist)
},
```

The other test functions (TestReadFileSymlinks, TestReadFileEdgeCases, TestFileExistsEdgeCases) check errors returned by the `ReadFile()` function, which already uses `NewFileError()` internally.

## Verification

All tests compile and pass successfully:
- TestReadFileSymlinks: PASS (4/4 subtests)
- TestReadFileEdgeCases: PASS (4/4 subtests)  
- TestFileExistsEdgeCases: PASS (5/5 subtests)
- TestFileErrorMessageContent: PASS (4/4 subtests)

## Git History

This work was completed in previous commits:
- `02e4eb46`: "test: replace FileError constructions with NewFileError() constructor"
- `d593da9c`: "fix(yamlutil): correct NewFileError signature to match test usage"

No changes were required for this task.
