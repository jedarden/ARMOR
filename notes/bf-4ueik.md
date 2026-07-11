# ValidationError and NewValidationError Documentation

## Location
Both the `ValidationError` struct and `NewValidationError` function are located in:
**File:** `/home/coding/ARMOR/internal/yamlutil/errors.go`

## ValidationError Struct (lines 395-409)

```go
type ValidationError struct {
    FilePath     string    // Path to the file being validated
    FieldPath    string    // Dot-notation path to the invalid field (optional)
    Path         string    // Dot-notation field path (e.g., "spec.replicas")
    Message      string    // Human-readable error message
    Line         int       // Line number where error occurred (1-indexed)
    Column       int       // Column number where error occurred (1-indexed, optional)
    Constraint   string    // Constraint that was violated (optional)
    ContextStr   string    // Additional context about the validation state (optional)
    Err          error     // Underlying error for error wrapping (optional)
    ErrorCode    ErrorCode // Error code for programmatic handling (optional)
    Type         ErrorType // Category of error for type switching
    ExpectedType string    // Expected type for type mismatch errors (optional)
    ActualType   string    // Actual type found for type mismatch errors (optional)
}
```

### Struct Interface Implementations
- **Code()** (lines 412-417): Returns the error code or defaults to `ErrCodeValidationFailed`
- **YAMLErrorType()** (lines 420-433): Returns the error type, with smart inference from error code/message
- **Context()** (lines 436-438): Returns the context string
- **Error()** (lines 441-483): Formats a human-readable error message with location and context
- **Unwrap()** (lines 486-488): Returns the underlying error for error wrapping chains
- **String()** (lines 491-518): Returns a formatted multi-line error message with full context

## NewValidationError Function (lines 520-565)

```go
func NewValidationError(
    filePath string,
    message string,
    fieldPath string,
    constraint string,
    code ErrorCode,
    line int,
    column int,
    errorType ErrorType,
    path string
) *ValidationError
```

### Parameters
| Parameter | Type | Description |
|-----------|------|-------------|
| `filePath` | `string` | Path to the file being validated |
| `message` | `string` | Human-readable error message |
| `fieldPath` | `string` | Dot-notation path to the invalid field (optional) |
| `constraint` | `string` | Constraint that was violated (optional) |
| `code` | `ErrorCode` | Error code for programmatic handling (use empty string for default) |
| `line` | `int` | Line number where error occurred (1-indexed, use 0 if unknown) |
| `column` | `int` | Column number where error occurred (1-indexed, use 0 if unknown) |
| `errorType` | `ErrorType` | Category of error (use empty string for default ErrorTypeValidation) |
| `path` | `string` | Dot-notation field path (optional, for backward compatibility) |

### Behavior
- If `code` is empty, defaults to `ErrCodeValidationFailed`
- If `errorType` is empty, defaults to `ErrorTypeValidation`
- Returns a properly initialized `*ValidationError` that implements the `YAMLError` interface

### Example Usage (from documentation comments)
```go
err := NewValidationError(
    "config.yaml",
    "invalid port number",
    "server.port",
    "must be between 1-65535",
    ErrCodeInvalidValue,
    10,
    5,
    "",
    "spec.replicas",
)
```

## Related Error Codes
The `ErrorCode` type is defined in the same file (lines 158-269). Relevant validation error codes include:
- `ErrCodeValidationFailed` - General validation failure
- `ErrCodeRequiredField` - Missing required field
- `ErrCodeConstraintViolation` - Constraint violation
- `ErrCodeInvalidValue` - Invalid value

## Related Error Types
The `ErrorType` type is defined in the same file (lines 44-63). Relevant validation error types include:
- `ErrorTypeValidation` - General validation errors
- `ErrorTypeFieldNotFound` - Missing required fields
- `ErrorTypeConstraint` - Constraint violations
