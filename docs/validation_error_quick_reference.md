# Validation Error Formatting - Quick Reference

## ValidationError Structure

```go
type ValidationError struct {
    // Required
    ValidationType string       // "status_code", "error_message", "content_type", "status_code_range"
    Expected       interface{}   // What was expected
    Actual         interface{}   // What was actually received

    // Optional Context
    Context         string       // Additional context (endpoint, operation)
    FieldName       string       // Field where error was found (for message validation)
    ResponseSnippet string       // Truncated response excerpt

    // Detailed Information
    PatternDetails     string   // Pattern matching failure info
    RangeInfo          string   // Range boundaries for range validation
    ValidationDetails  []string // Additional validation details

    // Actionable Guidance
    Suggestions []string       // Auto-generated or custom suggestions
}
```

## Quick Usage Examples

### 1. Basic Status Code Error

```go
err := validate.FormatStatusCodeError(200, 404, "GET /api/users")
// Output:
// status_code validation failed
//   Expected: 200 (OK)
//   Actual:   404 (Not Found)
//   Context:  GET /api/users
//   Suggestions: (auto-generated for 404)
```

### 2. Error Message Pattern Error

```go
err := validate.FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth")
// Output:
// error_message validation failed
//   Expected: invalid.*token
//   Actual:   access_denied
//   Field:    error
//   Context:  OAuth
//   Suggestions: (auto-generated for auth errors)
```

### 3. Status Code Range Error

```go
err := validate.FormatStatusCodeRangeError("4xx", 200, "error check")
// Output:
// status_code_range validation failed
//   Expected: 4xx (400-499)
//   Actual:   200
//   Context:  error check
//   Suggestions: (auto-generated for range mismatches)
```

### 4. Content Type Error

```go
err := validate.FormatContentTypeError("application/json", "text/html", "API response")
// Output:
// content_type validation failed
//   Expected: application/json
//   Actual:   text/html
//   Context:  API response
```

### 5. Using Builder Pattern

```go
err := validate.NewValidationFormatter("status_code").
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users").
    WithResponseSnippet(`{"error": "User not found"}`).
    Format()
```

### 6. Custom Error with Options

```go
err := validate.FormatCustomValidationError(
    "custom_field",
    "required_value",
    "actual_value",
    validate.WithContext("custom validation"),
    validate.WithResponseSnippet(`{"field": "actual_value"}`),
    validate.WithSuggestions("Check field value", "Verify configuration"),
)
```

## Validation Types

| Type | Expected | Actual | Use Case |
|------|----------|--------|----------|
| `status_code` | `int` or `[]int` | `int` | HTTP status code validation |
| `error_message` | `string` (pattern) | `string` (message) | Error message content validation |
| `status_code_range` | `string` (pattern) | `int` | Status code range validation |
| `content_type` | `string` | `string` | Content-Type header validation |

## Common Validation Types

### Status Code Validation

- **Single code:** `200`, `404`, `500`
- **Multiple codes:** `[]int{200, 201, 204}`
- **Error categories:** `4xx` (client error), `5xx` (server error)

### Error Message Patterns

- **Regex patterns:** `"invalid.*token"`, `"authentication.*failed"`
- **Substrings:** `"not found"`, `"unauthorized"`
- **Case-insensitive:** Auto-detected for patterns without regex metacharacters

### Status Code Ranges

- **Pattern format:** `"Nxx"` where N is 1-5
- **Valid ranges:** `"1xx"`, `"2xx"`, `"3xx"`, `"4xx"`, `"5xx"`
- **Examples:** `"2xx"` (200-299), `"4xx"` (400-49)

## Auto-Generated Suggestions

Suggestions are automatically generated based on validation type and values:

### 404 Not Found
- Verify the endpoint URL is correct
- Check if the resource ID or identifier exists
- Ensure the resource hasn't been deleted or moved

### 401 Unauthorized
- Verify authentication credentials are correct
- Check if API token or session has expired
- Ensure Authorization header is properly formatted

### 403 Forbidden
- Verify your account has permission to access this resource
- Check if additional scopes or roles are required
- Review API documentation for required permissions

### 500 Server Error
- Implement retry logic with exponential backoff
- Check service status page for ongoing issues
- Contact support if the issue persists

### Token Errors
- Refresh the authentication token
- Check token expiration time
- Implement automatic token refresh

### Rate Limiting
- Implement rate limiting and exponential backoff
- Check API quota limits
- Consider caching responses

## Required vs Optional Fields

### Required (must be populated)
- `ValidationType` - Category of validation
- `Expected` - What was expected
- `Actual` - What was actually received

### Optional (recommended but not required)
- `Context` - Additional context about the validation
- `FieldName` - Field where error was found
- `ResponseSnippet` - Response excerpt for debugging
- `PatternDetails` - Pattern matching information
- `RangeInfo` - Range boundaries
- `ValidationDetails` - Additional details
- `Suggestions` - Auto-generated if not provided

## Helper Functions

### Detection Functions
- `HTTPStatusCodeIsValid()` - Check status code validity
- `HTTPStatusCodeIsError()` - Check if status code indicates error
- `HTTPStatusCodeIsClientError()` - Check for 4xx errors
- `HTTPStatusCodeIsServerError()` - Check for 5xx errors
- `ContentTypeIsValid()` - Check Content-Type header
- `CORSHeadersIsValid()` - Check CORS headers

### Validation Functions
- `ValidateErrorMessagePattern()` - Validate error message against pattern
- `ValidateErrorMessage()` - Validate error message with detailed errors
- `ErrorCodeInResponse()` - Check for error code in response
- `ValidateStatusCodeAndErrorCode()` - Validate both status code and error code

### Result Types
- `StatusCodeValidationResult` - Detailed status code validation results
- `ErrorMessageValidationResult` - Error message validation results
- `ErrorCodeMatch` - Error code detection results

## Error Message Format

```
{validation_type} validation failed
  Expected: {expected_value} ({description})
  Actual:   {actual_value} ({description})
  Context:  {context}
  Field:    {field_name}
  Pattern:  {pattern_details}
  Range:    {range_info}
  Response: {response_snippet}
  Details:
    - {detail_1}
    - {detail_2}
  Suggestions:
    - {suggestion_1}
    - {suggestion_2}
    - {suggestion_3}
```

## Best Practices

### DO
- Provide context when possible (endpoint URL, operation type)
- Include response snippets for message validation failures
- Use specific field names when validating error messages
- Let suggestions auto-generate unless domain-specific guidance is needed
- Use convenience functions for common scenarios

### DON'T
- Leave Context empty if you have relevant information
- Include full response bodies (use snippets, max 200 chars)
- Hardcode suggestions for common scenarios
- Ignore ValidationErrorDetails for complex failures

## See Also

- [Validation Error Format Design](./validation_error_format_design.md) - Comprehensive design documentation
- `internal/validate/validate.go` - Core validation functions
- `internal/validate/format_helper.go` - Formatting helper functions
- `internal/validate/example_test.go` - Usage examples
