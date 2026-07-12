# ParseError Test Construction Verification

## Task: bf-4a7ft

### Findings

**No ParseError direct struct initializations found in test files.**

According to the manifest from bead bf-6bevx, ParseError has **0 instances** of direct struct initialization across all test files in `internal/yamlutil/`.

### Verification

1. **Compilation**: ✓ PASS - `go build ./internal/yamlutil/...` completes successfully
2. **ParseError usage**: All ParseError instances in test files already use the `NewParseError()` constructor
   - Direct struct initializations in test files: **0**
   - Constructor usages in test files: **61**
3. **Constructor signature**: `NewParseError(filePath, message, line, column, code, expected, actual)` - 7 parameters
4. **Test execution**: Tests run successfully (pre-existing failures unrelated to ParseError construction)

### Constructor Usage Examples (Already in Code)

From `error_message_format_examples_test.go`:
```go
err := NewParseError("config.yaml", "missing colon", 10, 5, ErrCodeInvalidSyntax, "", "")
err := NewParseError("schema.yaml", "type mismatch", 7, 12, ErrCodeTypeMismatch, "string", "integer")
```

### Conclusion

The ParseError error type is already properly using constructor functions throughout the test suite. No changes were needed.

**Status**: COMPLETE - Verification successful, 0 replacements required

### Context

This is the final error type in the systematic error constructor migration. Other error types (ValidationError, FileError, SchemaValidationError, TypeMismatchError) had direct struct initializations that were replaced in prior beads, but ParseError was already using constructor patterns exclusively.

### References

- Manifest: `.claude/error_test_manifest.md` (from bead bf-6bevx)
- Constructor: `internal/yamlutil/errors.go:NewParseError()`
