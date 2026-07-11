# Bead bf-2lflg: Add line:column context to ParseError messages

## Status: ✅ Complete

## Summary

Verified that ParseError messages include comprehensive line:column context. The implementation is already complete and all tests pass.

## Implementation Details

### ParseError.Error() Format

The `ParseError.Error()` method formats errors with full context:

```
parse error in <file> at line X, column Y: <message>
```

Example:
```
parse error in config.yaml at line 10, column 5: invalid syntax (expected: identifier, actual: 123)
```

### Position Information Capture

Position information is preserved through the parsing pipeline:

1. **Parser extraction** (`parser.go`): Line numbers extracted from yaml.v3 error messages
2. **Error types** (`errors.go`): `ParseError` struct includes `Line` and `Column` fields
3. **Error formatting**: `Error()` method includes line:column in all output

### Error Types with Line:Column Context

- `ParseError`: Full line:column support
- `ValidationError`: Full line:column support
- `SyntaxError`: Full line:column support
- `StructureError`: Line support
- `TypeMismatchError`: Line support
- `EnhancedParseError`: Full line:column with rich context

## Test Coverage

All tests pass:

- `TestErrorFormattingExamples/ParseError_with_line:column_context`: ✅ PASS
- `TestHumanReadableFormatting/consistent_error_prefix_format`: ✅ PASS
- All yamlutil tests: ✅ PASS

## Acceptance Criteria

✅ **ParseError messages include "line X, column Y" context**
- Format: `parse error in <file> at line X, column Y: <message>`
- Verified in tests checking for "line X" and "column Y" strings

✅ **Position information is preserved from the parse error source**
- `ParseError.Line` and `ParseError.Column` fields capture position
- Parser extracts line numbers from yaml.v3 errors via `extractErrorLine()`

✅ **Error format is consistent**
- All error types follow consistent pattern
- Line:column included when available
- File path always included

## Files Modified

No modifications needed - implementation was already complete.

## Verification

```bash
go test ./internal/yamlutil/... -v
# PASS
# ok github.com/jedarden/armor/internal/yamlutil (cached)
```
