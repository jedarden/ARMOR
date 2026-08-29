# ARMOR Error Testing Framework Guide

## Overview

The ARMOR Error Testing Framework is a comprehensive, pattern-based testing infrastructure for validating S3-compatible error responses. This guide is designed for new developers to understand how to use, extend, and maintain the error testing system.

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Core Concepts](#core-concepts)
3. [Quick Start Guide](#quick-start-guide)
4. [Using Predefined Patterns](#using-predefined-patterns)
5. [Creating Custom Patterns](#creating-custom-patterns)
6. [Test Infrastructure](#test-infrastructure)
7. [Validation Helpers](#validation-helpers)
8. [Best Practices](#best-practices)
9. [Common Patterns](#common-patterns)
10. [Troubleshooting](#troubleshooting)

## Architecture Overview

The error testing framework is organized into several interconnected files:

```
internal/server/
├── error_test_patterns.go              # Core pattern definitions
├── error_test_infrastructure_test.go   # Test helpers and fixtures
├── error_test_patterns_base_test.go    # Test table structures
├── error_test_patterns_test.go         # Framework tests
├── http_error_fixtures.go              # HTTP response fixtures
└── error_pattern_usage_example.go      # Usage examples
```

### File Responsibilities

| File | Purpose |
|------|---------|
| `error_test_patterns.go` | Defines all error pattern structures, constants, and predefined patterns |
| `error_test_infrastructure_test.go` | Provides test server fixtures, validation helpers, and utility functions |
| `error_test_patterns_base_test.go` | Contains test table structures and predefined test tables |
| `error_test_patterns_test.go` | Tests for the framework itself |
| `http_error_fixtures.go` | HTTP error response fixtures for integration testing |
| `error_pattern_usage_example.go` | Executable examples demonstrating pattern usage |

## Core Concepts

### 1. Error Patterns

Error patterns are reusable configurations that define expected error responses. They encapsulate:

- **HTTP Status Code**: The expected HTTP status (e.g., 404, 403)
- **S3 Error Code**: The S3-specific error code (e.g., "NoSuchKey")
- **Message Validation**: Expected message content and keywords
- **Performance Expectations**: Maximum acceptable response time
- **Category**: The error type (Auth, NotFound, InvalidRequest, etc.)

### 2. Test Tables

Test tables are collections of test cases that use error patterns. They enable:

- **Table-Driven Testing**: Run multiple scenarios with consistent validation
- **Composability**: Combine base patterns with custom scenarios
- **Organization**: Group related tests by category or feature

### 3. Validation Chain

The framework provides a fluent validation API for checking responses:

```go
VerifyErrorResponse(t, response).
    HTTPStatusCode(404).
    HasCode("NoSuchKey").
    MessageContainsAny("not", "found").
    Assert()
```

## Quick Start Guide

### Step 1: Use a Predefined Pattern

The simplest way to start is using predefined patterns:

```go
import "github.com/jedarden/armor/internal/server"

func TestMyFeatureErrors(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    // Use the ResourceNotFound pattern
    pattern := server.CommonErrorPatterns.ResourceNotFound
    
    // Make a request
    req := server.CreateTestRequest(t, "GET", "/bucket/nonexistent", nil, nil)
    duration, w := server.MeasureRequestTime(fixture.Handler, req)
    
    // Validate using pattern expectations
    server.VerifyErrorResponseWithTiming(t, w, duration).
        HTTPStatusCode(pattern.ExpectedStatus).
        HasCode(pattern.ExpectedCode).
        MessageContainsAny(pattern.ExpectedKeywords...).
        Assert()
}
```

### Step 2: Run a Predefined Test Table

For comprehensive testing, use predefined test tables:

```go
func TestAuthenticationErrors(t *testing.T) {
    fixture := server.NewTestServer(t)
    tests := server.StandardAuthenticationErrorTests()
    
    for _, tt := range tests {
        t.Run(tt.Name, func(t *testing.T) {
            server.RunAuthenticationTestCase(t, fixture, tt)
        })
    }
}
```

### Step 3: Extend with Custom Tests

Add your custom tests to the base patterns:

```go
func TestCustomScenarios(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    // Start with base patterns
    baseTests := server.StandardAuthenticationErrorTests()
    
    // Add custom tests
    customTests := []server.AuthenticationErrorTestCase{
        {
            CommonErrorTestCase: server.CommonErrorTestCase{
                Name:        "My custom auth scenario",
                Description: "Tests my specific authentication logic",
                SetupRequest: func(t *testing.T) *http.Request {
                    return createMyCustomRequest(t)
                },
                ExpectedStatus: 403,
                ExpectedCode:   server.ErrorCodeAccessDenied,
            },
            AuthErrorType: "CustomAuthFailure",
        },
    }
    
    // Combine and run
    allTests := append(baseTests, customTests...)
    server.RunAuthenticationErrorTable(t, fixture, allTests)
}
```

## Using Predefined Patterns

### Common Error Patterns

The framework provides 8 common error patterns for everyday use:

```go
server.CommonErrorPatterns.ResourceNotFound      // 404 - missing objects
server.CommonErrorPatterns.AccessDenied          // 403 - access denied
server.CommonErrorPatterns.InvalidRequest        // 400 - malformed requests
server.CommonErrorPatterns.UnsupportedMediaType   // 415 - wrong content type
server.CommonErrorPatterns.MethodNotAllowed      // 405 - unsupported HTTP method
server.CommonErrorPatterns.InternalServerError    // 500 - server errors
server.CommonErrorPatterns.SignatureMismatch     // 403 - signature validation
server.CommonErrorPatterns.RequestExpired         // 403 - expired requests
```

### Authentication-Specific Patterns

For detailed authentication testing, use auth-specific patterns:

```go
server.AuthErrorPatterns.MissingAuthHeader      // Missing Authorization header
server.AuthErrorPatterns.InvalidAccessKeyId      // Invalid access key
server.AuthErrorPatterns.SignatureDoesNotMatch   // Signature mismatch
server.AuthErrorPatterns.MissingDateHeader       // Missing X-Amz-Date
server.AuthErrorPatterns.RequestExpired          // Expired timestamp
server.AuthErrorPatterns.MalformedAuthHeader     // Malformed auth header
```

### Client vs Server Error Patterns

Organize tests by HTTP status categories:

```go
// Client errors (4xx)
clientPatterns := []server.ErrorScenarioConfig{
    server.ClientErrorPatterns.BadRequest,
    server.ClientErrorPatterns.NotFound,
    server.ClientErrorPatterns.MethodNotAllowed,
    server.ClientErrorPatterns.UnsupportedMediaType,
}

// Server errors (5xx)
serverPatterns := []server.ErrorScenarioConfig{
    server.ServerErrorPatterns.InternalError,
    server.ServerErrorPatterns.ServiceUnavailable,
}
```

### Accessing Patterns by Category

Dynamically retrieve patterns by error category:

```go
// Get all authentication error patterns
authPatterns := server.PatternsForCategory(server.CategoryAuth)

// Get all not-found patterns
notFoundPatterns := server.PatternsForCategory(server.CategoryNotFound)

// Iterate through patterns
for _, pattern := range authPatterns {
    fmt.Printf("Pattern: %s (%d)\n", pattern.Name, pattern.ExpectedStatus)
}
```

## Creating Custom Patterns

### Option 1: Extend an Existing Pattern

The recommended approach is to start with a predefined pattern:

```go
func createCustomNotFoundPattern() server.ErrorScenarioConfig {
    basePattern := server.CommonErrorPatterns.ResourceNotFound
    
    return server.ErrorScenarioConfig{
        Name:              "Custom Object Not Found",
        ExpectedCode:      basePattern.ExpectedCode,
        ExpectedStatus:    basePattern.ExpectedStatus,
        ExpectedMessage:   "Custom message: object not found",
        ExpectedKeywords:  []string{"custom", "not", "found"},
        MinMessageLength:   basePattern.MinMessageLength,
        MaxResponseTime:   basePattern.MaxResponseTime,
        Description:       "Custom not found scenario with specific messaging",
        Category:          basePattern.Category,
    }
}
```

### Option 2: Use the Pattern Builder

For programmatic pattern creation:

```go
func createCustomAuthPattern() server.ErrorScenarioConfig {
    return server.ErrorScenarioConfig{
        Name:              "Custom Auth Failure",
        ExpectedCode:      "CustomAuthError",
        ExpectedStatus:    403,
        ExpectedMessage:   "Custom authentication failed",
        ExpectedKeywords:  []string{"custom", "auth", "failed"},
        MinMessageLength:  20,
        MaxResponseTime:   300 * time.Millisecond,
        Description:       "Tests custom authentication scenario",
        Category:          string(server.CategoryAuth),
    }
}
```

### Option 3: Create from Scratch

For completely new error scenarios:

```go
var CustomErrorPatterns = struct {
    RateLimitExceeded server.ErrorScenarioConfig
    QuotaExceeded     server.ErrorScenarioConfig
}{
    RateLimitExceeded: server.ErrorScenarioConfig{
        Name:              "Rate Limit Exceeded",
        ExpectedCode:      "RateLimitExceeded",
        ExpectedStatus:    429,
        ExpectedMessage:   "Rate limit exceeded",
        ExpectedKeywords:  []string{"rate", "limit", "exceeded"},
        MinMessageLength:  20,
        MaxResponseTime:   100 * time.Millisecond,
        Description:       "Tests rate limiting behavior",
        Category:          "RateLimit",
    },
    QuotaExceeded: server.ErrorScenarioConfig{
        Name:              "Quota Exceeded",
        ExpectedCode:      "QuotaExceeded",
        ExpectedStatus:    429,
        ExpectedMessage:   "Storage quota exceeded",
        ExpectedKeywords:  []string{"quota", "exceeded"},
        MinMessageLength:  20,
        MaxResponseTime:   100 * time.Millisecond,
        Description:       "Tests quota enforcement",
        Category:          "Quota",
    },
}
```

## Test Infrastructure

### Test Server Fixture

The `TestServerFixture` provides a configured test server:

```go
fixture := server.NewTestServer(t)

// Access the HTTP handler
handler := fixture.Handler

// Access the configuration
config := fixture.Config
```

**Default Test Configuration:**
- Bucket: "test-bucket"
- Region: "us-east-005"
- Credentials: TESTACCESSKEY (full access), RESTRICTEDKEY (limited access)

### Request Creation Helpers

The framework provides helpers for creating common test requests:

```go
// Authentication failures
createMissingAuthHeaderRequest(t)
createInvalidKeyRequest(t)
createInvalidSignatureRequest(t)
createMalformedAuthRequest(t)
createMissingDateRequest(t)
createExpiredRequest(t)

// Non-authentication failures
createNotFoundRequest(t)
createMethodNotAllowedRequest(t)
createUnsupportedMediaTypeRequest(t)
createMissingContentTypeRequest(t)

// CORS requests
createPreflightRequest(t)
```

**When to use each helper:**

| Helper | When to Use |
|--------|-------------|
| `createMissingAuthHeaderRequest` | Testing missing Authorization header |
| `createInvalidKeyRequest` | Testing invalid access key ID |
| `createInvalidSignatureRequest` | Testing signature validation |
| `createMalformedAuthRequest` | Testing malformed auth header format |
| `createMissingDateRequest` | Testing missing X-Amz-Date header |
| `createExpiredRequest` | Testing timestamp validation |
| `createNotFoundRequest` | Testing 404 responses |
| `createMethodNotAllowedRequest` | Testing unsupported HTTP methods |
| `createUnsupportedMediaTypeRequest` | Testing content-type validation |
| `createPreflightRequest` | Testing CORS preflight |

### Test Case Structures

The framework provides hierarchical test case structures:

```go
// Base structure for all error tests
type CommonErrorTestCase struct {
    Name                    string
    Description             string
    SetupRequest            func(*testing.T) *http.Request
    ExpectedStatus          int
    ExpectedCode            string
    ExpectedMessageKeywords []string
    MinMessageLength        int
    MaxResponseTime         time.Duration
    ValidateResponse        func(*testing.T, *httptest.ResponseRecorder)
}

// Extended for authentication errors
type AuthenticationErrorTestCase struct {
    CommonErrorTestCase
    AccessKey        string
    AuthErrorType    string
    ExpectedAuthError error
}

// Extended for non-authentication errors
type NonAuthenticationErrorTestCase struct {
    CommonErrorTestCase
    ErrorCategory string
    RequiresAuth  bool
    ResourcePath  string
}

// Extended for CORS errors
type CORSErrorTestCase struct {
    CommonErrorTestCase
    Origin             string
    ExpectedCORSOrigin string
    ExpectedCORSMethods string
    ExpectedCORSHeaders string
    IsPreflight        bool
}
```

### Running Test Tables

Execute entire test tables with consistent reporting:

```go
// Authentication errors
server.RunAuthenticationErrorTable(t, fixture, authTests)

// Non-authentication errors
server.RunNonAuthenticationErrorTable(t, fixture, nonAuthTests)

// CORS errors
server.RunCORSErrorTable(t, fixture, corsTests)
```

## Validation Helpers

### Fluent Validation API

The framework provides a chainable validation API:

```go
server.VerifyErrorResponse(t, response).
    HTTPStatusCode(404).
    ContentType("application/xml").
    HasCode("NoSuchKey").
    HasMessage("The specified key does not exist").
    MessageContainsAny("not", "found").
    MessageMinLength(15).
    ResponseTime(500 * time.Millisecond).
    HasCORSHeaders().
    CORSOrigin("*").
    CORSMethods("GET, PUT, DELETE, HEAD, POST, OPTIONS").
    Assert()
```

### Validation Method Reference

| Method | Purpose | When to Use |
|--------|---------|-------------|
| `HTTPStatusCode(code)` | Validate HTTP status | Always required |
| `ContentType(type)` | Validate Content-Type header | Always required |
| `HasCode(code)` | Validate S3 error code | Always required |
| `HasMessage(msg)` | Validate exact message content | When exact message matters |
| `MessageContainsAny(keywords...)` | Validate message contains keywords | Prefer over exact message |
| `MessageMinLength(length)` | Validate minimum message length | Always required (default: 15) |
| `BodyNotEmpty()` | Ensure response has body | Always required |
| `HasXMLDeclaration()` | Ensure XML declaration present | Always required |
| `ResponseTime(duration)` | Validate response time | When performance matters |
| `HasCORSHeaders()` | Validate CORS headers present | For CORS scenarios |
| `CORSOrigin(origin)` | Validate CORS origin value | For CORS scenarios |
| `CORSMethods(methods)` | Validate CORS methods | For CORS scenarios |
| `CORSHeaders(headers)` | Validate CORS allowed headers | For CORS scenarios |

### Timing-Aware Validation

For performance testing, use timing-aware validation:

```go
// Measure request time
duration, w := server.MeasureRequestTime(fixture.Handler, request)

// Validate with timing
server.VerifyErrorResponseWithTiming(t, w, duration).
    HTTPStatusCode(404).
    ResponseTime(500 * time.Millisecond).  // Fails if exceeded
    Assert()
```

### Custom Validation

Add custom validation logic:

```go
testCase := server.CommonErrorTestCase{
    Name: "Custom validation example",
    SetupRequest: func(t *testing.T) *http.Request {
        return createMyRequest(t)
    },
    ExpectedStatus: 404,
    ExpectedCode:   "NoSuchKey",
    ValidateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
        // Custom validation beyond standard checks
        headers := w.Header()
        customHeader := headers.Get("X-Custom-Header")
        if customHeader != "expected-value" {
            t.Errorf("Expected custom header 'expected-value', got '%s'", customHeader)
        }
    },
}

server.RunCommonErrorValidations(t, w, testCase, duration)
```

## Best Practices

### 1. Use Predefined Patterns First

Always check if a predefined pattern fits your needs before creating custom ones:

```go
// Good: Use predefined pattern
pattern := server.CommonErrorPatterns.ResourceNotFound

// Avoid: Reinventing the wheel
customPattern := server.ErrorScenarioConfig{
    ExpectedCode: "NoSuchKey",
    ExpectedStatus: 404,
    // ... redefining existing pattern
}
```

### 2. Extend Rather Than Replace

When you need customization, extend existing patterns:

```go
// Good: Extend predefined pattern
basePattern := server.CommonErrorPatterns.ResourceNotFound
customPattern := basePattern
customPattern.ExpectedMessage = "Custom message"

// Avoid: Complete rewrite
customPattern := server.ErrorScenarioConfig{
    // ... duplicating all base pattern fields
}
```

### 3. Test Tables Over Individual Tests

Prefer table-driven testing over individual test cases:

```go
// Good: Table-driven test
tests := []server.AuthenticationErrorTestCase{
    { /* scenario 1 */ },
    { /* scenario 2 */ },
    { /* scenario 3 */ },
}
server.RunAuthenticationErrorTable(t, fixture, tests)

// Avoid: Separate test functions
func TestScenario1(t *testing.T) { /* ... */ }
func TestScenario2(t *testing.T) { /* ... */ }
func TestScenario3(t *testing.T) { /* ... */ }
```

### 4. Descriptive Test Names

Use clear, descriptive test names:

```go
// Good: Descriptive name
server.AuthenticationErrorTestCase{
    CommonErrorTestCase: server.CommonErrorTestCase{
        Name: "Access denied with expired timestamp",
        Description: "Verify that requests with timestamps older than 15 minutes return 403 RequestExpired",
        // ...
    },
}

// Avoid: Vague name
server.AuthenticationErrorTestCase{
    CommonErrorTestCase: server.CommonErrorTestCase{
        Name: "Auth test",
        // ...
    },
}
```

### 5. Keyword-Based Message Validation

Prefer keyword-based validation over exact message matching:

```go
// Good: Flexible keyword matching
ExpectedMessageKeywords: []string{"not", "found", "exist"}

// Avoid: Brittle exact matching
ExpectedMessageSubstring: "The specified key does not exist"
```

### 6. Set Realistic Response Time Expectations

Base response time expectations on your environment:

```go
// Good: Environment-aware timing
MaxResponseTime: 500 * time.Millisecond,  // Adjusted for test environment

// Avoid: Unrealistic expectations
MaxResponseTime: 1 * time.Millisecond,  // Too strict for most environments
```

### 7. Use Helper Functions Appropriately

Match request creation helpers to your test scenario:

```go
// Good: Use the right helper
createExpiredRequest(t)  // For timestamp validation

// Avoid: Misusing helpers
createNotFoundRequest(t)  // Wrong for testing timestamp validation
```

### 8. Validate Multiple Aspects

Don't stop at status code validation:

```go
// Good: Comprehensive validation
server.VerifyErrorResponse(t, response).
    HTTPStatusCode(404).
    HasCode("NoSuchKey").
    ContentType("application/xml").
    MessageContainsAny("not", "found").
    HasCORSHeaders().
    Assert()

// Avoid: Minimal validation
if response.StatusCode != 404 {
    t.Errorf("Expected 404")
}
```

## Common Patterns

### Pattern 1: Basic Error Validation

The most common pattern - validate a basic error response:

```go
func TestBasicError(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    req := server.CreateTestRequest(t, "GET", "/bucket/missing", nil, nil)
    w := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w, req)
    
    server.VerifyErrorResponse(t, w).
        HTTPStatusCode(404).
        HasCode("NoSuchKey").
        ContentType("application/xml").
        MessageMinLength(15).
        Assert()
}
```

### Pattern 2: Pattern-Based Table Test

Use predefined patterns for table-driven testing:

```go
func TestPatternBasedTable(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    scenarios := []struct {
        name    string
        pattern server.ErrorScenarioConfig
        request func(t *testing.T) *http.Request
    }{
        {
            name:    "Not found",
            pattern: server.CommonErrorPatterns.ResourceNotFound,
            request: server.createNotFoundRequest,
        },
        {
            name:    "Access denied",
            pattern: server.CommonErrorPatterns.AccessDenied,
            request: server.createInvalidKeyRequest,
        },
    }
    
    for _, scenario := range scenarios {
        t.Run(scenario.name, func(t *testing.T) {
            req := scenario.request(t)
            duration, w := server.MeasureRequestTime(fixture.Handler, req)
            
            server.VerifyErrorResponseWithTiming(t, w, duration).
                HTTPStatusCode(scenario.pattern.ExpectedStatus).
                HasCode(scenario.pattern.ExpectedCode).
                MessageContainsAny(scenario.pattern.ExpectedKeywords...).
                ResponseTime(scenario.pattern.MaxResponseTime).
                Assert()
        })
    }
}
```

### Pattern 3: Extending Standard Tables

Extend standard test tables with custom scenarios:

```go
func TestExtendedStandardTable(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    // Get standard tests
    baseTests := server.StandardAuthenticationErrorTests()
    
    // Add custom tests
    customTests := []server.AuthenticationErrorTestCase{
        {
            CommonErrorTestCase: server.CommonErrorTestCase{
                Name:        "Custom auth scenario",
                Description: "Test my custom authentication logic",
                SetupRequest: func(t *testing.T) *http.Request {
                    return createMyCustomRequest(t)
                },
                ExpectedStatus: 403,
                ExpectedCode:   "CustomAuthError",
            },
            AuthErrorType: "CustomAuthFailure",
        },
    }
    
    // Combine and run
    allTests := append(baseTests, customTests...)
    server.RunAuthenticationErrorTable(t, fixture, allTests)
}
```

### Pattern 4: Category-Based Testing

Test all errors in a specific category:

```go
func TestAllAuthErrors(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    authPatterns := server.PatternsForCategory(server.CategoryAuth)
    
    for _, pattern := range authPatterns {
        t.Run(pattern.Name, func(t *testing.T) {
            req := createRequestForPattern(t, pattern)
            duration, w := server.MeasureRequestTime(fixture.Handler, req)
            
            server.VerifyErrorResponseWithTiming(t, w, duration).
                HTTPStatusCode(pattern.ExpectedStatus).
                HasCode(pattern.ExpectedCode).
                MessageContainsAny(pattern.ExpectedKeywords...).
                Assert()
        })
    }
}
```

### Pattern 5: CORS Validation

Validate CORS headers on error responses:

```go
func TestCORSOnErrors(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    tests := []server.CORSErrorTestCase{
        {
            CommonErrorTestCase: server.CommonErrorTestCase{
                Name:         "404 with CORS",
                SetupRequest: server.createNotFoundRequest,
                ExpectedStatus: 404,
                ExpectedCode:   "NoSuchKey",
            },
            Origin:             "*",
            ExpectedCORSOrigin: "*",
            ExpectedCORSMethods: "GET, PUT, DELETE, HEAD, POST, OPTIONS",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.Name, func(t *testing.T) {
            req := tt.SetupRequest(t)
            req.Header.Set("Origin", tt.Origin)
            
            duration, w := server.MeasureRequestTime(fixture.Handler, req)
            
            server.VerifyErrorResponseWithTiming(t, w, duration).
                HTTPStatusCode(tt.ExpectedStatus).
                HasCode(tt.ExpectedCode).
                CORSOrigin(tt.ExpectedCORSOrigin).
                CORSMethods(tt.ExpectedCORSMethods).
                Assert()
        })
    }
}
```

## Troubleshooting

### Common Issues and Solutions

#### Issue 1: Test Fails with "Expected status X, got Y"

**Problem:** HTTP status doesn't match expectation

**Solutions:**
1. Verify the pattern's `ExpectedStatus` matches actual behavior
2. Check if request setup is correct (method, path, headers)
3. Ensure test server is properly configured
4. Verify authentication credentials are valid (if auth required)

```go
// Debug: Print actual response
t.Logf("Actual status: %d", w.Code)
t.Logf("Response body: %s", w.Body.String())
```

#### Issue 2: Test Fails with "Expected code X, got Y"

**Problem:** S3 error code doesn't match expectation

**Solutions:**
1. Verify error code is correctly defined in constants
2. Check if server is returning the expected error type
3. Ensure error code matches the actual failure scenario
4. Review error response XML to see actual code

```go
// Debug: Parse and print error code
var s3Err server.S3Error
xml.Unmarshal(w.Body.Bytes(), &s3Err)
t.Logf("Actual error code: %s", s3Err.Code)
```

#### Issue 3: Test Fails with "Message validation failed"

**Problem:** Message content doesn't match keywords

**Solutions:**
1. Check if message keywords are too specific
2. Use `ExpectedMessageKeywords` instead of exact match
3. Verify message isn't changing between server versions
4. Consider relaxing validation if message format varies

```go
// Debug: Print actual message
var s3Err server.S3Error
xml.Unmarshal(w.Body.Bytes(), &s3Err)
t.Logf("Actual message: %s", s3Err.Message)
```

#### Issue 4: Test Fails with "Response time exceeded"

**Problem:** Response took longer than expected

**Solutions:**
1. Adjust `MaxResponseTime` for your environment
2. Check if test environment is slower than expected
3. Verify no network delays or resource contention
4. Consider removing timing validation if not critical

```go
// Debug: Print actual duration
t.Logf("Actual duration: %v", duration)
```

#### Issue 5: CORS Validation Fails

**Problem:** CORS headers missing or incorrect

**Solutions:**
1. Ensure `Origin` header is set on request
2. Verify CORS is enabled in server configuration
3. Check if CORS headers match expected values
4. Use `SkipCORSValidation: true` for non-CORS scenarios

```go
// Debug: Print CORS headers
t.Logf("CORS Origin: %s", w.Header().Get("Access-Control-Allow-Origin"))
t.Logf("CORS Methods: %s", w.Header().Get("Access-Control-Allow-Methods"))
```

### Debug Mode

Enable detailed logging for troubleshooting:

```go
func TestWithDebugging(t *testing.T) {
    fixture := server.NewTestServer(t)
    
    req := server.CreateTestRequest(t, "GET", "/bucket/key", nil, nil)
    w := httptest.NewRecorder()
    fixture.Handler.ServeHTTP(w, req)
    
    // Debug output
    t.Logf("Request: %s %s", req.Method, req.URL.Path)
    t.Logf("Status: %d", w.Code)
    t.Logf("Headers: %v", w.Header())
    t.Logf("Body: %s", w.Body.String())
    
    // Then validate
    server.VerifyErrorResponse(t, w).
        HTTPStatusCode(404).
        Assert()
}
```

### Getting Help

If you're stuck:

1. **Check examples**: Review `error_pattern_usage_example.go`
2. **Read tests**: Look at `error_test_patterns_test.go`
3. **Verify patterns**: Run `error_patterns_verification_test.go`
4. **Consult docs**: Reference this guide and `error-responses.md`

## Additional Resources

### Code Examples

- **Usage Examples**: `internal/server/error_pattern_usage_example.go`
- **Framework Tests**: `internal/server/error_test_patterns_test.go`
- **Verification Tests**: `internal/server/error_patterns_verification_test.go`

### Documentation

- **Error Reference**: `docs/error-responses.md`
- **Header Specification**: `docs/error-header-spec.md`
- **Response Inventory**: `docs/error-response-inventory.md`

### Related Documentation

- **S3 Error Codes**: [AWS S3 Error Response Reference](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html)
- **HTTP Status Codes**: [MDN HTTP Status Reference](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status)
- **CORS Specification**: [MDN CORS Guide](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)

## Summary

The ARMOR Error Testing Framework provides:

- **8 common error patterns** for everyday use
- **6 authentication-specific patterns** for auth testing
- **Fluent validation API** for response checking
- **Table-driven test infrastructure** for comprehensive testing
- **Extensible architecture** for custom scenarios

### Key Takeaways

1. **Start with predefined patterns** - Don't reinvent the wheel
2. **Use table-driven tests** - Better organization and reporting
3. **Validate comprehensively** - Check status, code, message, headers
4. **Extend, don't replace** - Build on existing patterns
5. **Be flexible with messages** - Use keywords, not exact matches

### Quick Reference Card

```go
// Create test server
fixture := server.NewTestServer(t)

// Use predefined pattern
pattern := server.CommonErrorPatterns.ResourceNotFound

// Create request
req := server.CreateTestRequest(t, "GET", "/bucket/key", nil, nil)

// Execute and measure
duration, w := server.MeasureRequestTime(fixture.Handler, req)

// Validate response
server.VerifyErrorResponseWithTiming(t, w, duration).
    HTTPStatusCode(pattern.ExpectedStatus).
    HasCode(pattern.ExpectedCode).
    MessageContainsAny(pattern.ExpectedKeywords...).
    ResponseTime(pattern.MaxResponseTime).
    Assert()
```

---

**Last Updated:** 2026-07-14  
**Framework Version:** 0.1.1741  
**Maintained By:** ARMOR Development Team
