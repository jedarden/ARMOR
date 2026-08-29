# HTTP Status Code Validation Helper Functions

This document describes the HTTP status code validation helper functions available in the ARMOR test infrastructure. These helpers provide comprehensive and flexible status code validation for error response testing.

## Overview

The status code validation helpers are located in `internal/server/error_status_validation.go` and provide:

- **Single status code validation** - Check for exact status code matches
- **Multiple allowed status codes** - Validate against a list of acceptable codes
- **Range-based validation** - Check if status code falls within a range (e.g., 2xx, 4xx)
- **Non-asserting boolean functions** - For conditional logic without test failures
- **Convenience functions** - For common HTTP status code categories
- **S3-specific validators** - For common S3 error scenarios

## Function Reference

### Core Validation Functions

#### `ValidateStatusCode(t, response, expectedCode)`

Validates that a response has exactly the expected HTTP status code.

**Parameters:**
- `t *testing.T` - The testing instance
- `response` - Response object (*httptest.ResponseRecorder or *http.Response)
- `expectedCode int` - The expected HTTP status code

**Example:**
```go
w := httptest.NewRecorder()
w.WriteHeader(404)

ValidateStatusCode(t, w, 404) // Passes
ValidateStatusCode(t, w, 200) // Fails with error message
```

#### `ValidateStatusCodeAny(t, response, allowedCodes)`

Validates that a response has one of the allowed HTTP status codes.

**Parameters:**
- `t *testing.T` - The testing instance
- `response` - Response object
- `allowedCodes []int` - Slice of acceptable status codes

**Example:**
```go
w := httptest.NewRecorder()
w.WriteHeader(204)

ValidateStatusCodeAny(t, w, []int{200, 201, 204}) // Passes
ValidateStatusCodeAny(t, w, []int{403, 404})     // Fails
```

#### `ValidateStatusCodeRange(t, response, minCode, maxCode)`

Validates that a response's status code falls within a range.

**Parameters:**
- `t *testing.T` - The testing instance
- `response` - Response object
- `minCode int` - Minimum acceptable status code (inclusive)
- `maxCode int` - Maximum acceptable status code (inclusive)

**Example:**
```go
w := httptest.NewRecorder()
w.WriteHeader(204)

ValidateStatusCodeRange(t, w, 200, 299) // Passes (2xx success)
ValidateStatusCodeRange(t, w, 400, 499) // Fails (not 4xx)
```

### Non-Asserting Boolean Functions

#### `CheckStatusCode(response, expectedCode) bool`

Checks if a response has the expected status code without asserting.

**Returns:** `true` if status code matches, `false` otherwise

**Example:**
```go
if CheckStatusCode(w, 404) {
    // Handle 404 case
} else if CheckStatusCode(w, 403) {
    // Handle 403 case
}
```

#### `CheckStatusCodeAny(response, allowedCodes) bool`

Checks if a response has one of the allowed status codes without asserting.

**Returns:** `true` if status code is in allowed list, `false` otherwise

**Example:**
```go
allowedCodes := []int{200, 201, 204}
if CheckStatusCodeAny(w, allowedCodes) {
    // Handle success case
}
```

#### `CheckStatusCodeRange(response, minCode, maxCode) bool`

Checks if a response's status code falls within a range without asserting.

**Returns:** `true` if status code is in range, `false` otherwise

**Example:**
```go
if CheckStatusCodeRange(w, 200, 299) {
    // Handle any success code (2xx)
}
```

### Convenience Functions

#### `ValidateSuccessStatusCode(t, response)`

Validates that the response has a 2xx success status code (200-299).

**Common success codes:** 200 OK, 201 Created, 204 No Content

**Example:**
```go
ValidateSuccessStatusCode(t, w) // Validates 200-299 range
```

#### `ValidateClientErrorStatusCode(t, response)`

Validates that the response has a 4xx client error status code (400-499).

**Common client error codes:** 400 Bad Request, 403 Forbidden, 404 Not Found

**Example:**
```go
ValidateClientErrorStatusCode(t, w) // Validates 400-499 range
```

#### `ValidateServerErrorStatusCode(t, response)`

Validates that the response has a 5xx server error status code (500-599).

**Common server error codes:** 500 Internal Server Error, 502 Bad Gateway, 503 Service Unavailable

**Example:**
```go
ValidateServerErrorStatusCode(t, w) // Validates 500-599 range
```

#### `ValidateRedirectStatusCode(t, response)`

Validates that the response has a 3xx redirect status code (300-399).

**Common redirect codes:** 301 Moved Permanently, 302 Found, 304 Not Modified

**Example:**
```go
ValidateRedirectStatusCode(t, w) // Validates 300-399 range
```

### S3-Specific Validators

#### `ValidateS3NotFoundStatusCode(t, response)`

Validates that the response has HTTP 404 (Not Found) - used for missing S3 objects/buckets.

**Example:**
```go
ValidateS3NotFoundStatusCode(t, w) // Validates 404
```

#### `ValidateS3AccessDeniedStatusCode(t, response)`

Validates that the response has HTTP 403 (Forbidden) - used for S3 authentication/authorization errors.

**Example:**
```go
ValidateS3AccessDeniedStatusCode(t, w) // Validates 403
```

#### `ValidateS3BadRequestStatusCode(t, response)`

Validates that the response has HTTP 400 (Bad Request) - used for S3 validation errors.

**Example:**
```go
ValidateS3BadRequestStatusCode(t, w) // Validates 400
```

### Utility Functions

#### `GetStatusCodeDescription(code int) string`

Returns a human-readable description for an HTTP status code.

**Example:**
```go
desc := GetStatusCodeDescription(404)
// desc = "Not Found"

desc = GetStatusCodeDescription(500)
// desc = "Internal Server Error"
```

**Supported codes include:**
- 1xx: Informational (100 Continue, 101 Switching Protocols, 102 Processing)
- 2xx: Success (200 OK, 201 Created, 202 Accepted, 204 No Content, 206 Partial Content)
- 3xx: Redirection (300 Multiple Choices, 301 Moved Permanently, 302 Found, 304 Not Modified, 307/308 Redirects)
- 4xx: Client Errors (400 Bad Request, 401 Unauthorized, 403 Forbidden, 404 Not Found, 405 Method Not Allowed, 409 Conflict, 410 Gone, 422 Unprocessable Entity, 429 Too Many Requests)
- 5xx: Server Errors (500 Internal Server Error, 502 Bad Gateway, 503 Service Unavailable, 504 Gateway Timeout)

## Usage Patterns

### Basic Error Response Validation

```go
func TestMyError(t *testing.T) {
    fixture := NewTestServer(t)
    
    req := CreateTestRequest(t, "GET", "/test-bucket/nonexistent", nil, nil)
    w := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w, req)
    
    // Validate status code
    ValidateStatusCode(t, w, 404)
    
    // Or use S3-specific validator
    ValidateS3NotFoundStatusCode(t, w)
}
```

### Multiple Acceptable Status Codes

```go
func TestFlexibleResponse(t *testing.T) {
    fixture := NewTestServer(t)
    
    req := CreateTestRequest(t, "PUT", "/test-bucket/key", nil, nil)
    w := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w, req)
    
    // Accept either 200 OK or 201 Created
    ValidateStatusCodeAny(t, w, []int{200, 201})
}
```

### Category-Based Validation

```go
func TestErrorCategories(t *testing.T) {
    fixture := NewTestServer(t)
    
    // Test client error (4xx)
    req := CreateTestRequest(t, "POST", "/test-bucket/", nil, nil)
    w := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w, req)
    ValidateClientErrorStatusCode(t, w) // 400-499
    
    // Test success (2xx)
    req2 := CreateTestRequest(t, "GET", "/test-bucket/key", nil, nil)
    w2 := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w2, req2)
    ValidateSuccessStatusCode(t, w2) // 200-299
}
```

### Conditional Logic Without Assertions

```go
func TestConditionalHandling(t *testing.T) {
    fixture := NewTestServer(t)
    
    req := CreateTestRequest(t, "GET", "/test-bucket/key", nil, nil)
    w := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w, req)
    
    if CheckStatusCode(w, 404) {
        t.Log("Key not found - expected behavior")
    } else if CheckStatusCodeAny(w, []int{403, 401}) {
        t.Log("Access denied - check credentials")
    } else if CheckStatusCodeRange(w, 200, 299) {
        t.Log("Success - key found")
    } else {
        t.Errorf("Unexpected status code: %d", w.Code)
    }
}
```

### Integration with Error Response Validator

```go
func TestCompleteValidation(t *testing.T) {
    fixture := NewTestServer(t)
    
    req := CreateTestRequest(t, "GET", "/test-bucket/nonexistent", nil, nil)
    w := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w, req)
    
    // Combine status code validation with other validations
    VerifyErrorResponse(t, w).
        HTTPStatusCode(404).
        ContentType("application/xml").
        HasCode("NoSuchKey").
        MessageMinLength(15).
        Assert()
}
```

## Error Messages

The validation functions provide clear, informative error messages that include:

1. **Expected vs. actual status codes** - Shows what was expected and what was received
2. **Human-readable descriptions** - Includes status code names (e.g., "Not Found" for 404)
3. **Allowed code lists** - For multiple code validation, shows all acceptable codes
4. **Range descriptions** - For range validation, shows the category (e.g., "Success", "Client Error")

**Example error messages:**

```
Expected HTTP status code 404 (Not Found), got 200 (OK)
Expected HTTP status code to be one of [[200 (OK) 201 (Created) 204 (No Content)]], got 404 (Not Found)
Expected HTTP status code in range [200-299] (Success), got 400 (Bad Request)
```

## Testing

The validation helpers are thoroughly tested in `internal/server/error_status_validation_test.go`. Test coverage includes:

- Single status code validation (positive and negative cases)
- Multiple allowed status codes
- Status code range validation
- Non-asserting boolean functions
- Status code descriptions
- Convenience functions for all categories
- S3-specific validators
- Integration tests with real HTTP responses
- Real-world usage scenarios

Run the tests with:
```bash
go test -v ./internal/server -run TestValidateStatusCode
go test -v ./internal/server -run TestCheckStatusCode
go test -v ./internal/server -run TestValidateSuccessStatusCode
go test -v ./internal/server -run TestValidateS3
```

## Best Practices

1. **Use specific validators when possible** - Instead of `ValidateStatusCode(t, w, 404)`, use `ValidateS3NotFoundStatusCode(t, w)` for S3 tests
2. **Combine with other validations** - Use status code validation alongside content-type, error code, and message validation
3. **Use range validation for categories** - Use `ValidateClientErrorStatusCode(t, w)` instead of checking multiple specific codes
4. **Leverage boolean functions for complex logic** - Use `CheckStatusCode*` functions when you need conditional behavior
5. **Test both success and failure paths** - Ensure your tests cover both expected and unexpected status codes

## Migration Guide

If you're currently using manual status code checking:

**Before:**
```go
if w.Code != 404 {
    t.Errorf("Expected 404, got %d", w.Code)
}
```

**After:**
```go
ValidateStatusCode(t, w, 404)
// Or for S3 errors:
ValidateS3NotFoundStatusCode(t, w)
```

**Before:**
```go
if w.Code < 200 || w.Code > 299 {
    t.Errorf("Expected success code, got %d", w.Code)
}
```

**After:**
```go
ValidateSuccessStatusCode(t, w)
```