# Bead bf-2lflg: Add line:column context to ParseError messages

## Task Completion Summary

This bead requested adding line:column context to ParseError messages. Upon verification, this functionality was already fully implemented in the codebase.

## Verification Results

### Current Implementation Status

The `ParseError.Error()` method in `/home/coding/ARMOR/internal/yamlutil/errors.go` (lines 306-339) already includes:

1. **Line context**: When `Line > 0`, the error message includes "line X"
2. **Column context**: When `Column > 0`, the error message includes "column Y"  
3. **Combined format**: "parse error in {file} at line X, column Y: {message}"

### Acceptance Criteria Verification

All acceptance criteria are met:

✅ **ParseError messages include "line X, column Y" context**
- Implemented in `ParseError.Error()` method lines 311-315
- Example output: `"parse error in config.yaml at line 10, column 5: invalid syntax"`

✅ **Position information is preserved from the parse error source**
- `ParseError` struct has `Line int` and `Column int` fields (lines 277-278)
- These fields are populated by `NewParseError()` constructor (line 365)
- Position information is properly stored and formatted

✅ **Error format is consistent**
- Format pattern: `"parse error in {filepath} at line {line}, column {column}: {message}"`
- When column is 0, format is: `"parse error in {filepath} at line {line}: {message}"`
- When line is 0, format is: `"parse error in {filepath}: {message}"`

### Test Coverage

All tests pass:
- `TestNewParseError/basic_parse_error` - Tests full line:column format
- `TestNewParseError/parse_error_with_line_and_column` - Tests position tracking
- `TestNewParseError/parse_error_without_line_number` - Tests fallback format
- `TestNewParseError/parse_error_with_expected_and_actual` - Tests with additional details

### Example Outputs

```go
// With line and column
err := NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "", "")
// Output: "parse error in config.yaml at line 10, column 5: invalid syntax"

// With line only
err := NewParseError("test.yaml", "unexpected token", 3, 0, "", "", "")
// Output: "parse error in test.yaml at line 3: unexpected token"

// Without position
err := NewParseError("unknown.yaml", "file is corrupted", 0, 0, "", "", "")
// Output: "parse error in unknown.yaml: file is corrupted"
```

## Conclusion

The task requirements were already fully implemented. The `ParseError.Error()` method properly formats error messages with line and column context, position information is preserved through the struct fields, and the format is consistent across all use cases.
