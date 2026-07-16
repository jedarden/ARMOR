# Status Code and Error Message Validation - Implementation Summary

## Overview
This document summarizes the implementation of comprehensive HTTP status code and error message validation for the ARMOR project, fulfilling all acceptance criteria for bead bf-1wo5x8.

## Acceptance Criteria Status

### ✅ 1. Add function to validate expected vs actual status codes
**Implementation:** Multiple functions for status code validation

- **`HTTPStatusCodeIsValid(resp, expected)`** - Basic boolean validation
- **`ValidateStatusCodeInt(expected, actual)`** - Detailed validation with error reporting
- **`ValidateStatusCodePattern(pattern, actual)`** - Pattern-based validation (e.g., "4xx")
- **`ValidateStatusCodeWithDetails(resp, expected)`** - Comprehensive validation with result object

**Example:**
```go
// Basic validation
err := ValidateStatusCodeInt(200, actualCode)

// Pattern validation
err := ValidateStatusCodePattern("4xx", actualCode) // Matches 400-499

// Detailed validation with context
result := ValidateStatusCodeWithDetails(response, []int{200, 201, 204}, "POST /api/users")
```

### ✅ 2. Validate error message patterns and content
**Implementation:** Multiple functions for error message validation

- **`ValidateErrorMessage(body, pattern)`** - Substring and regex pattern matching
- **`ValidateErrorMessagePattern(body, pattern, caseInsensitive)`** - Flexible pattern validation
- **`ValidateErrorMessageWithDetails(body, pattern)`** - Comprehensive validation with result object

**Example:**
```go
// Substring matching
err := ValidateErrorMessage(responseBody, "not found")

// Regex pattern matching
err := ValidateErrorMessage(responseBody, "invalid.*token")

// Case-insensitive pattern matching
err := ValidateErrorMessage(responseBody, "(?i)authentication.*failed")

// Comprehensive validation
result := ValidateErrorMessageWithDetails(body, EnhancedErrorMessagePattern{
    Pattern: "unauthorized",
    MustContain: []string{"token", "expired"},
    MinLength: 10,
})
```

### ✅ 3. Support status code ranges (e.g., 4xx, 5xx)
**Implementation:** Multiple range validation functions

- **`ValidateStatusCodeRange(resp, StatusCodeRange)`** - Custom range validation
- **`ValidateStatusCodeRangeInt(pattern, actual)`** - Pattern-based range validation (4xx, 5xx, etc.)
- **`ParseStatusCodeRange(pattern)`** - Parse range patterns to get min/max
- **`GetStatusCodeRangeDescription(pattern)`** - Get human-readable descriptions

**Predefined Ranges:**
- `Range1xx` (100-199) - Informational
- `Range2xx` (200-299) - Success
- `Range3xx` (300-399) - Redirection
- `Range4xx` (400-499) - Client Error
- `Range5xx` (500-599) - Server Error

**Example:**
```go
// Using predefined ranges
err := ValidateStatusCodeRange(response, Range4xx)

// Using pattern validation
err := ValidateStatusCodeRangeInt("4xx", actualCode) // Validates 400-499

// Custom range
customRange := StatusCodeRange{Min: 200, Max: 204, Description: "Standard success"}
err := ValidateStatusCodeRange(response, customRange)
```

### ✅ 4. Check for specific error codes in responses
**Implementation:** Multiple error code validation functions

- **`ValidateErrorCode(body, expectedCode)`** - Exact error code matching
- **`ValidateErrorCodePattern(body, pattern)`** - Pattern-based error code matching
- **`ValidateErrorCodeAny(body, allowedCodes)`** - Multiple allowed error codes
- **`ErrorCodeInResponse(body, expectedCode)`** - Boolean check for error code presence
- **`ValidateStatusCodeAndErrorCode(resp, status, code)`** - Combined validation

**Example:**
```go
// Exact string error code
err := ValidateErrorCode(responseBody, "AUTH_FAILED")

// Numeric error code
err := ValidateErrorCode(responseBody, 401)

// Pattern matching
err := ValidateErrorCodePattern(responseBody, "AUTH_*") // Matches AUTH_FAILED, AUTH_REQUIRED, etc.

// Multiple allowed codes
allowedCodes := []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED", "INVALID_CREDENTIALS"}
err := ValidateErrorCodeAny(responseBody, allowedCodes)

// Combined status code and error code validation
valid, err := ValidateStatusCodeAndErrorCode(response, 401, "AUTH_FAILED")
```

### ✅ 5. Provide detailed mismatch information
**Implementation:** Comprehensive error reporting with multiple result types

- **`StatusCodeValidationResult`** - Detailed status code validation results
- **`ErrorMessageValidationResult`** - Detailed error message validation results
- **`ErrorCodeMatch`** - Error code matching details
- **`ValidationError`** - Comprehensive validation error with suggestions

**Features:**
- Expected vs actual values
- Field names where errors were found
- Response snippets for debugging
- Pattern-specific details
- Validation details list
- Contextual suggestions for fixing issues
- Range information for range validations
- Distance from valid range (for range mismatches)

**Example:**
```go
result := ValidateStatusCodeWithDetails(response, 200, "POST /api/users")
if !result.Valid {
    fmt.Printf("Expected: %v\n", result.ExpectedCodes)
    fmt.Printf("Actual: %d\n", result.ActualCode)
    fmt.Printf("Category: %s\n", result.Category)
    fmt.Printf("Details: %s\n", result.MismatchDetails)
}
```

### ✅ 6. Include examples for common status code validations
**Implementation:** Comprehensive test suite with examples

**Test Files:**
- `status_code_validation_examples_test.go` - Original examples
- `status_code_validation_fixed_examples_test.go` - Fixed case-sensitive examples
- `error_code_validation_test.go` - Error code validation tests
- Multiple other test files for specific validation scenarios

**Common API Validation Scenarios:**
1. REST API - GET resource not found (404)
2. REST API - POST validation error (400)
3. REST API - Authentication failure (401)
4. REST API - Rate limit exceeded (429)
5. REST API - Server error (500)
6. REST API - Forbidden with permission error (403)
7. REST API - Success case with 201 (Created)

## Key Features

### Pattern Matching
- **Substring matching:** Case-sensitive substring search
- **Regex matching:** Full regex pattern support
- **Case-insensitive:** `(?i)` prefix for case-insensitive patterns
- **Wildcard patterns:** `AUTH_*` for prefix matching
- **Range patterns:** `4xx`, `5xx` for status code ranges

### Error Reporting
- **Detailed mismatch information:** Shows exactly what went wrong
- **Contextual suggestions:** Specific suggestions based on the error type
- **Field-level reporting:** Shows which field contained the error
- **Response snippets:** Includes truncated response for debugging
- **Pattern details:** Shows pattern-specific information
- **Range information:** For range validation failures

### Helper Functions
- **`GetErrorMessage(body)`** - Extract error message from response
- **`GetErrorCode(body)`** - Extract error code from response
- **`GetStatusCodeDescription(code)`** - Get human-readable status code description
- **`ParseStatusCodeRange(pattern)`** - Parse range patterns

## Usage Examples

### Basic Status Code Validation
```go
// Simple validation
if HTTPStatusCodeIsValid(response, 200) {
    // Response is OK
}

// Detailed validation
err := ValidateStatusCodeInt(200, response.StatusCode)
if err != nil {
    log.Printf("Status code validation failed: %v", err)
}
```

### Error Message Validation
```go
// Check error message contains substring
err := ValidateErrorMessage(responseBody, "not found")
if err != nil {
    log.Printf("Error message validation failed: %v", err)
}

// Case-insensitive regex pattern
err := ValidateErrorMessage(responseBody, "(?i)authentication.*failed")
```

### Status Code Range Validation
```go
// Validate 4xx range
err := ValidateStatusCodePattern("4xx", response.StatusCode)

// Validate any success code (2xx)
err := ValidateStatusCodePattern("2xx", response.StatusCode)

// Custom range
err := ValidateStatusCodeRange(response, StatusCodeRange{
    Min: 200, Max: 204, Description: "Standard success",
})
```

### Error Code Validation
```go
// Exact error code match
err := ValidateErrorCode(responseBody, "AUTH_FAILED")

// Pattern match
err := ValidateErrorCodePattern(responseBody, "AUTH_*")

// Multiple allowed codes
allowedCodes := []interface{}{"AUTH_FAILED", "TOKEN_EXPIRED"}
err := ValidateErrorCodeAny(responseBody, allowedCodes)
```

### Comprehensive Validation
```go
// Validate status code with detailed results
result := ValidateStatusCodeWithDetails(response, []int{200, 201}, "POST /api/users")
if !result.Valid {
    log.Printf("Status validation failed: %s", result.MismatchDetails)
}

// Validate error message with comprehensive results
pattern := EnhancedErrorMessagePattern{
    Pattern: "unauthorized",
    CaseInsensitive: true,
    MustContain: []string{"token", "expired"},
    MinLength: 10,
}
result := ValidateErrorMessageWithDetails(body, pattern)
if !result.Valid {
    log.Printf("Validation issues: %v", result.Issues)
}
```

## Test Coverage

The implementation includes comprehensive test coverage:

- **Unit tests:** Individual function testing
- **Integration tests:** Combined validation scenarios
- **Example tests:** Real-world API validation scenarios
- **Error case tests:** Validation failure scenarios
- **Range tests:** Status code range validation
- **Pattern tests:** Pattern matching for both error messages and codes

## Dependencies

The implementation depends on:
- Standard library `net/http` for HTTP types
- Standard library `encoding/json` for JSON parsing
- Standard library `regexp` for pattern matching
- Custom `ValidationError` type for error reporting

## Conclusion

All acceptance criteria have been fully implemented with:
- ✅ Multiple validation functions for status codes
- ✅ Comprehensive error message pattern validation
- ✅ Full support for status code ranges (1xx-5xx)
- ✅ Specific error code checking capabilities
- ✅ Detailed mismatch information with suggestions
- ✅ Extensive examples and test coverage

The validation system is production-ready and provides both simple boolean checks and detailed validation results with comprehensive error reporting.
