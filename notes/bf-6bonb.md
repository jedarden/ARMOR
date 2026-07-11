# Bead bf-6bonb: Define ParseError Struct

**Status:** Already Implemented (Verified 2026-07-11)

## Summary

This bead's requirements were already fully implemented in a previous session. The `ParseError` struct is defined in `internal/yamlutil/errors.go` (lines 98-176).

## Implementation Verified

### 1. ParseError Struct Definition
```go
type ParseError struct {
    FilePath   string      // Path to the file being parsed
    Line       int         // Line number where error occurred (1-indexed)
    Column     int         // Column number where error occurred (1-indexed)
    Message    string      // Human-readable error message
    ContextStr string      // Additional context about the parsing state
    Err        error       // Underlying error for error wrapping
    ErrorType  ErrorType   // Specific type of parse error
    ErrorCode  ErrorCode   // Error code for programmatic handling
}
```

### 2. YAMLError Interface Methods Implemented

**Code() Method** (lines 114-119):
- Returns the ErrorCode field
- Falls back to `ErrCodeParseError` if not set

**Error() Method** (lines 132-137):
- Returns formatted error message with position context
- Format: `"parse error in {filepath} at line {line}: {message}"`
- Handles cases without line numbers

**Bonus Methods** (also implemented):
- `YAMLErrorType()` - Returns error category
- `Context()` - Returns additional context string
- `Unwrap()` - Supports error wrapping chains

### 3. Constructor Function

**NewParseError()** (lines 157-176):
- Takes filePath, message, line, column, and errorCode parameters
- Uses provided code or defaults to `ErrCodeParseError`
- Properly initializes all fields including `ErrorType = ErrorTypeParse`

## Test Coverage

All tests pass:
```
=== RUN   TestNewParseError
--- PASS: TestNewParseError (0.00s)
PASS
```

## Conclusion

No additional implementation needed. The ParseError struct was fully implemented in a previous session and meets all acceptance criteria.
