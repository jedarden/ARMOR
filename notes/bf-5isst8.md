# Expected vs Actual Value Display Implementation (bf-5isst8)

## Summary

Successfully implemented expected vs actual value display in the error formatting system for ARMOR's validation package.

## Implementation Details

### Core Components

1. **ExpectedActual Struct** (`internal/validate/optional_error_types.go`)
   - Represents expected vs actual value comparison
   - Supports any value type via interface{}
   - Methods: `HasExpected()`, `HasActual()`, `IsEmpty()`, `Mismatched()`

2. **FormatValidationErrorWithExpectedActual()** (`internal/validate/error_formatting.go`)
   - Extended error formatter accepting optional ExpectedActual parameter
   - Falls back to err.Expected/err.Actual when ExpectedActual is empty
   - ExpectedActual parameter takes precedence when provided

3. **FormatExpectedActual()** (`internal/validate/error_formatting.go`)
   - Formats ExpectedActual struct into readable string
   - Handles different value types appropriately:
     - **Integers**: Adds HTTP status code descriptions (e.g., "200 (OK)")
     - **Strings**: Wrapped in quotes, truncated at 100 chars
     - **Floats**: Formatted to 2 decimal places
     - **Slices of int**: Formats as "one of [X, Y, Z]" with descriptions
     - **Slices of string**: Formats as "['a', 'b', 'c']"
     - **Maps**: Formats as "{key: value, ...}", truncated at 5 items
     - **Slices of interface**: Formats as "[1, two, 3.0]"

### Key Features

1. **Empty Handling**: Returns empty string when ExpectedActual is empty (both values nil)
2. **Type-Specific Formatting**: Each type gets appropriate formatting
3. **Truncation**: Long strings (100+ chars), large maps (5+ items), and large slices (5+ items) are truncated
4. **Nil Handling**: Gracefully handles nil values for expected/actual
5. **Side-by-Side Display**: Shows "expected: X, actual: Y" format

### Test Coverage

Comprehensive test suite in `internal/validate/error_formatting_expected_actual_test.go`:
- `TestFormatExpectedActual`: 18 test cases covering all value types
- `TestFormatExpectedActualInline`: 6 test cases for compact format
- `TestFormatValidationErrorWithExpectedActual`: 12 test cases for integration
- `TestValidationErrorWithExpectedActualIntegration`: Integration tests
- All tests passing ✓

### Acceptance Criteria Met

- ✅ Extend error formatter to accept optional ExpectedActual parameter
- ✅ Format expected and actual values side-by-side when present
- ✅ Handle different value types (strings, numbers, objects, arrays)
- ✅ Omit value comparison section when ExpectedActual is nil/empty
- ✅ Add tests verifying values are displayed correctly when provided and omitted when not

## Example Usage

```go
// Basic usage with integers
ea := NewExpectedActual(200, 404)
err := ValidationError{
    ErrorType: "status_code",
    Message:   "Status code mismatch",
    FieldName: "response.status",
}
msg := FormatValidationErrorWithExpectedActual(err, true, nil, ea)
// Output: "[⚠️] High [status_code] response.status: Status code mismatch (expected: 200 (OK), actual: 404 (Not Found))"

// With strings
ea := NewExpectedActual("test@example.com", "invalid")
msg := FormatValidationErrorWithExpectedActual(err, false, nil, ea)
// Output: "[format] email: Invalid email format (expected: 'test@example.com', actual: 'invalid')"

// With slices
ea := ExpectedActual{Expected: []int{200, 201, 204}, Actual: 500}
msg := FormatValidationErrorWithExpectedActual(err, true, nil, ea)
// Output: "... (expected: one of [200 (OK), 201 (Created), 204 (No Content)], actual: 500 (Internal Server Error))"

// Empty ExpectedActual - values omitted
ea := ExpectedActual{}  // Empty
msg := FormatValidationErrorWithExpectedActual(err, true, nil, ea)
// Output: "[⚠️] High [required] email: Field is required" (no expected/actual shown)
```

## Files Modified

- `internal/validate/error_formatting.go`: Added FormatValidationErrorWithExpectedActual() and FormatExpectedActual()
- `internal/validate/optional_error_types.go`: ExpectedActual struct (already existed)
- `internal/validate/error_formatting_expected_actual_test.go`: Comprehensive test suite (517 lines)

## Status

**COMPLETE** - All acceptance criteria met, all tests passing.
