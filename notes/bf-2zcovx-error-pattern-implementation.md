# Error Pattern Implementation Summary

## Bead: bf-2zcovx - Add common error pattern definitions

**Status:** ✓ COMPLETE

## Implementation

The predefined error patterns have been implemented in `internal/server/error_test_patterns.go`. The patterns are defined as exported package-level variables (struct instances), which is the idiomatic way to define "constants" for complex types in Go.

## Pattern Collections

### 1. CommonErrorPatterns (8 patterns)
Located at lines 330-458 in `error_test_patterns.go`

- **ResourceNotFound**: Standard 404 error for missing objects
  - Code: `NoSuchKey`, Status: 404
  - Keywords: not, found, exist, no such key
  - Max response time: 500ms

- **AccessDenied**: Authentication failure with invalid credentials
  - Code: `AccessDenied`, Status: 403
  - Keywords: access, denied, permission, authorized
  - Max response time: 300ms

- **InvalidRequest**: Malformed request parameters
  - Code: `InvalidRequest`, Status: 400
  - Keywords: invalid, request, malformed
  - Max response time: 200ms

- **UnsupportedMediaType**: Content-type validation error
  - Code: `UnsupportedMediaType`, Status: 415
  - Keywords: content, type, supported, media
  - Max response time: 200ms

- **MethodNotAllowed**: HTTP method validation error
  - Code: `MethodNotAllowed`, Status: 405
  - Keywords: method, allowed, supported
  - Max response time: 200ms

- **InternalServerError**: Server-side errors
  - Code: `InternalError`, Status: 500
  - Keywords: internal, error, server
  - Max response time: 1000ms

- **SignatureMismatch**: AWS signature validation failure
  - Code: `SignatureDoesNotMatch`, Status: 403
  - Keywords: signature, match, calculated
  - Max response time: 300ms

- **RequestExpired**: Timestamp validation error
  - Code: `RequestExpired`, Status: 403
  - Keywords: expired, timestamp, date
  - Max response time: 300ms

### 2. AuthErrorPatterns (6 patterns)
Located at lines 467-565 in `error_test_patterns.go`

- **MissingAuthHeader**: No Authorization header present (403)
- **InvalidAccessKeyId**: Access key not found or invalid (403)
- **SignatureDoesNotMatch**: Calculated signature doesn't match (403)
- **MissingDateHeader**: Required date header is missing (403)
- **RequestExpired**: Request timestamp is too old (403)
- **MalformedAuthHeader**: Authorization header format is invalid (403)

### 3. ClientErrorPatterns (4 patterns)
Located at lines 575-610 in `error_test_patterns.go`

- **BadRequest**: Generic 400 Bad Request errors
- **NotFound**: 404 Not Found errors (references CommonErrorPatterns.ResourceNotFound)
- **MethodNotAllowed**: 405 Method Not Allowed errors
- **UnsupportedMediaType**: 415 Unsupported Media Type errors

### 4. ServerErrorPatterns (2 patterns)
Located at lines 619-642 in `error_test_patterns.go`

- **InternalError**: 500 Internal Server Error
- **ServiceUnavailable**: 503 Service Unavailable

## Total Patterns: 20+

## Access Methods

### Direct Access
```go
pattern := CommonErrorPatterns.ResourceNotFound
fmt.Printf("Pattern: %s, Status: %d\n", pattern.Name, pattern.ExpectedStatus)
```

### By Error Code
```go
pattern := PatternForCode("NoSuchKey")
```

### By Category
```go
authPatterns := PatternsForCategory(CategoryAuth)
```

### All Patterns
```go
allPatterns := AllCommonPatterns()
```

## Documentation

Each pattern includes:
- **Name**: Human-readable pattern name
- **ExpectedCode**: S3 error code constant
- **ExpectedStatus**: HTTP status code
- **ExpectedMessage**: Expected error message text
- **ExpectedKeywords**: Alternative keywords for flexible matching
- **MinMessageLength**: Minimum acceptable message length
- **MaxResponseTime**: Maximum acceptable response time
- **Description**: Detailed explanation of what the pattern tests
- **Category**: Error category for organization

## Usage Example File

Created `error_pattern_usage_example.go` with 11 comprehensive examples demonstrating:
- Direct pattern access
- Pattern retrieval by code
- Pattern retrieval by category
- Custom pattern creation based on predefined patterns
- Pattern validation in testing
- Pattern metadata access
- Pattern usage in test scenarios
- Pattern comparison
- Response time validation

## Acceptance Criteria Met

✓ Add predefined error patterns as constants
  - Implemented as exported package-level variables (idiomatic Go)

✓ Include patterns for common error scenarios
  - 20+ patterns covering authentication, client errors, and server errors

✓ Make patterns easily accessible to other test files
  - Exported as public package variables with helper functions

✓ Add at least 3-5 common patterns
  - 8 common patterns, 6 auth patterns, 4 client patterns, 2 server patterns

✓ Each pattern should be well-documented
  - Every pattern has Name, Description, Category, ExpectedKeywords, and inline comments

## Files Modified/Created

1. **internal/server/error_test_patterns.go** - Contains all predefined error patterns
2. **internal/server/error_pattern_usage_example.go** - Usage examples and documentation
3. **internal/server/error_test_patterns_base_test.go** - Test patterns and execution helpers

## Integration

The patterns are integrated with:
- Error code constants (ErrorCodeAccessDenied, ErrorCodeNoSuchKey, etc.)
- Error category constants (CategoryAuth, CategoryNotFound, etc.)
- Mapping functions (CategoryForCode, ExpectedStatusCodeForCode)
- Test infrastructure (CommonErrorTestCase, AuthenticationErrorTestCase, etc.)

## Next Steps

The error pattern infrastructure is complete and ready for use in:
- HTTP error scenario testing
- Response validation
- CORS testing
- Authentication testing
- Content-type validation testing

All patterns are well-documented, easily accessible, and follow Go best practices.
