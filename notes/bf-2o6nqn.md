# Bead bf-2o6nqn: Assertion Error Messages and Return Logic

## Summary

This bead implemented comprehensive assertion error messages and return logic for validation functions in the ARMOR codebase. The implementation supports both boolean-only mode (for flexible use) and assertion mode with descriptive errors.

## Implementation Details

### Files Modified

1. **internal/server/content_type_validation.go** - Content-Type validation with assertion support
2. **internal/server/error_status_validation.go** - HTTP status code validation with assertion support  
3. **internal/server/content_type_validation_test.go** - Comprehensive test coverage

### Key Features Implemented

#### 1. Boolean Return Indicating Match Success
Both `ContentTypeMatchResult` and `StatusCodeMatchResult` structs include a `Match` boolean field:
```go
type ContentTypeMatchResult struct {
    Match bool
    Expected string
    Actual string
    ResponseContext string
    Error string
}
```

#### 2. Detailed Error Messages on Assertion Failure
The `buildContentTypeMismatchError` and `buildStatusCodeMismatchError` functions create comprehensive error messages:
```
Content-Type mismatch:
  Expected: application/json
  Actual:   text/plain
  Context:  httptest.ResponseRecorder (status: 200)
```

#### 3. Response Object Context in Errors
The `getResponseContext` and `getStatusCodeResponseContext` helpers extract contextual information:
- Response type (httptest.ResponseRecorder or http.Response)
- HTTP status code when available

#### 4. Optional Assertion Mode vs Boolean-Only Mode
The `AssertContentType` and `AssertStatusCode` functions accept an `assertMode` parameter:
- `assertMode=false`: Returns result with `Match` boolean for conditional logic
- `assertMode=true`: Populates detailed `Error` message for test assertions

#### 5. Clean, Readable Error Output
Error messages are formatted with:
- Clear section labels (Expected:, Actual:, Context:)
- Structured layout with proper indentation
- Human-readable status code descriptions (e.g., "404 (Not Found)")

### Usage Examples

#### Boolean Mode (for conditional logic)
```go
result := AssertContentType(w, "application/json", false)
if !result.Match {
    // Handle non-JSON case
}
```

#### Assertion Mode (for test assertions)
```go
result := AssertContentType(w, "application/json", true)
if !result.Match {
    t.Error(result.Error)  // Full error with expected vs actual
}
```

### Test Coverage

Comprehensive tests verify:
- ✅ Boolean mode with success and failure cases
- ✅ Assertion mode with detailed error messages
- ✅ Error message includes expected, actual, and context
- ✅ Support for multiple allowed types
- ✅ Integration with both httptest.ResponseRecorder and http.Response
- ✅ Enhanced error messages for debugging

## Acceptance Criteria Met

- ✅ Returns boolean indicating match success
- ✅ When assertion fails, error message clearly shows expected vs actual
- ✅ Error message includes the response object context
- ✅ Supports optional assertion mode vs boolean-only mode
- ✅ Clean, readable error output for debugging

## Testing Results

All assertion-related tests pass successfully:
- TestAssertContentType_BooleanMode_Success
- TestAssertContentType_BooleanMode_Failure
- TestAssertContentType_AssertionMode_DetailedErrors
- TestAssertContentTypeAny_BooleanMode_Success
- TestAssertContentTypeAny_AssertionMode_DetailedErrors
- TestEnhancedErrorMessages_AssertContentType
- And more...

## Related Work

This implementation builds on the existing content-type and status code validation infrastructure, adding flexible assertion capabilities while maintaining backward compatibility with existing code.
