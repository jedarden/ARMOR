# Task BF-25M1A: Update ValidationError in result_types_test.go

## Finding

All ValidationError constructions in `internal/yamlutil/result_types_test.go` are already using the `NewValidationError()` constructor.

### Analysis

The file contains:
- **3 ValidationError instances** - all already using `NewValidationError()`:
  - Line 424: `*NewValidationError("test.yaml", "required field missing", ...)`
  - Line 463: `*NewValidationError("test.yaml", "validation error", ...)`
  - Line 548: `*NewValidationError("config.yaml", "port out of range", ...)`
- **8 empty slice declarations** `[]ValidationError{}` - these are type declarations for empty slices, not struct constructions

### Conclusion

The task acceptance criteria are already met:
- ✓ All ValidationError constructions use NewValidationError()
- ✓ File compiles without errors
- ✓ No test logic needs changing

No code changes were required.
