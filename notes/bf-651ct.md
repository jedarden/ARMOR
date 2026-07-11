# Bead bf-651ct: Handle nil/empty path cases in NewValidationError

## Status: ✅ COMPLETED

This work was already completed in commit `e805fefc`.

## Implementation

The `NewValidationError` function in `internal/yamlutil/errors.go` now properly handles nil and empty path values:

### Logic Added (lines 554-559)
```go
// Handle nil/empty path: use fieldPath as fallback if path is empty
// This ensures the Path field is populated when available
validPath := path
if validPath == "" && fieldPath != "" {
    validPath = fieldPath
}
```

### Behavior
- **Empty path + non-empty fieldPath**: Uses fieldPath as fallback → Path field populated
- **Non-empty path**: Uses provided path value as-is → Path field populated
- **Both empty**: Path field remains empty string → No crash

### Test Coverage
Added comprehensive test suite in `internal/yamlutil/path_test.go`:
- Empty string path uses fieldPath as fallback ✓
- Non-empty path is stored correctly ✓
- Both path and fieldPath empty stays empty ✓
- Path with nested field path ✓
- Empty path with fieldPath uses fieldPath as fallback ✓

All tests passing.
