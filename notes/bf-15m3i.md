# Bead bf-15m3i: Update FileError in file_test.go

## Task
Replace all 14 direct FileError struct initializations in file_test.go with NewFileError() constructor calls.

## Status: ALREADY COMPLETED

The task was already completed in a previous session. Verification:

1. **No direct FileError constructions found**:
   ```bash
   grep -n "&FileError{" /home/coding/ARMOR/internal/yamlutil/file_test.go
   # (no output)
   ```

2. **All 14 instances already use NewFileError()**:
   ```bash
   grep -c "NewFileError" /home/coding/ARMOR/internal/yamlutil/file_test.go
   # Output: 14
   ```

3. **All tests pass**:
   ```bash
   go test ./internal/yamlutil -run "TestReadFile|TestFileError|TestFileExists|TestIsFileNotFoundError|TestIsPermissionError|TestReadFilePermissionDenied|TestReadFileSymlinks|TestReadFileEdgeCases|TestFileExistsEdgeCases|TestFileErrorMessageContent"
   # PASS
   ```

4. **File compiles**:
   ```bash
   go build ./internal/yamlutil/...
   # No errors
   ```

## Locations of NewFileError calls in file_test.go:
- Line 186: `err := NewFileError("/test/file.yaml", "read", "", nil)`
- Line 198: `err := NewFileError("/test/file.yaml", "resolve", "", nil)`
- Line 208: `err := NewFileError("/test/file.yaml", "read", "", underlyingErr)`
- Line 216: `err := NewFileError("/test/file.yaml", "read", "", nil)`
- Line 238: `err := NewFileError("/test/file.yaml", "read", "", os.ErrNotExist)`
- Line 246: `err := NewFileError("/test/file.yaml", "read", "", nil)`
- Line 254: `err := NewFileError("/test/file.yaml", "read", "", os.ErrClosed)`
- Line 276: `err := NewFileError("/test/file.yaml", "read", "", os.ErrPermission)`
- Line 284: `err := NewFileError("/test/file.yaml", "read", "", nil)`
- Line 292: `err := NewFileError("/test/file.yaml", "read", "", os.ErrClosed)`
- Line 710: `return NewFileError("/test/config.yaml", "read", "", os.ErrNotExist)`
- Line 717: `return NewFileError("/restricted/file.yaml", "read", "", os.ErrPermission)`
- Line 724: `return NewFileError("/invalid/path.yaml", "resolve", "", os.ErrNotExist)`
- Line 731: `return NewFileError("/test/file.yaml", "read", "custom error message", os.ErrNotExist)`

## Conclusion
No changes were required. The file already meets all acceptance criteria from the bead.
