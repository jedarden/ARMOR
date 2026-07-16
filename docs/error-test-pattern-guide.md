# Error Test Pattern Documentation

## Overview

The ARMOR Error Test Pattern is a comprehensive, table-driven testing framework for validating S3-compatible error responses. This guide explains how to use, extend, and maintain error tests in the ARMOR project.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Pattern Architecture](#pattern-architecture)
3. [Usage Examples](#usage-examples)
4. [Adding New Error Tests](#adding-new-error-tests)
5. [Test Table Structure](#test-table-structure)
6. [Validation Helpers](#validation-helpers)
7. [Common Error Types](#common-error-types)
8. [Troubleshooting](#troubleshooting)
9. [Best Practices](#best-practices)

## Quick Start

### Basic Error Test Example

The simplest way to write an error test is using the table-driven pattern:

```go
func TestMissingAuthentication(t *testing.T) {
    // Define test cases
    table := []testutil.TableTestCase[AuthTestInput, error]{
        {
            Name:        "request without auth header fails",
            Description: "Tests that missing Authorization header returns authentication error",
            Input: AuthTestInput{
                Description: "No Authorization header provided",
                SetupRequest: func(req *http.Request) {
                    // No auth header set
                },
                ValidateResponse: func(resp *httptest.ResponseRecorder) error {
                    if resp.Code != 403 {
                        return fmt.Errorf("expected status 403, got %d", resp.Code)
                    }
                    // Use validation helpers
                    if err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken"); err != nil {
                        return fmt.Errorf("error code validation failed: %w", err)
                    }
                    return nil
                },
            },
            ExpectedError: nil,
            ExpectError:   false,
        },
    }

    // Run the test table
    testutil.RunTable(t, table, func(input AuthTestInput) error {
        handler := createAuthMockHandler()
        req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

        if input.SetupRequest != nil {
            input.SetupRequest(req)
        }

        resp := testutil.MakeRequest(handler, req)

        if input.ValidateResponse != nil {
            return input.ValidateResponse(resp)
        }

        return nil
    })
}
```

## Pattern Architecture

### Three-Tier Structure

The error test pattern consists of three layers:

```
┌─────────────────────────────────────────┐
│   Generic Test Table Framework         │
│   (internal/testutil/table.go)         │
│   - TableTestCase                      │
│   - RunTable                           │
│   - Helper functions                   │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│   Validation Helpers                    │
│   (internal/testutil/validation_helpers.go) │
│   - Request builders                    │
│   - Response validators                 │
│   - S3 error parsing                    │
└─────────────────────────────────────────┘
              ↓
┌─────────────────────────────────────────┐
│   Error-Specific Tests                  │
│   (internal/server/*_test.go)          │
│   - Authentication errors               │
│   - Authorization errors                │
│   - Not found errors                   │
└─────────────────────────────────────────┘
```

### Core Components

1. **Test Input Structure**: Holds test case data
2. **Test Table**: Collection of test cases
3. **Runner Function**: Executes test logic
4. **Validation Functions**: Verify expected behavior

## Usage Examples

### Example 1: Authentication Error Test

```go
func TestAuthError_MissingAuthHeader(t *testing.T) {
    table := []testutil.TableTestCase[AuthTestInput, error]{
        {
            Name:        "request without auth header fails",
            Description: "Tests that missing Authorization header returns authentication error",
            Input: AuthTestInput{
                Description: "No Authorization header provided",
                SetupRequest: func(req *http.Request) {
                    // No auth header - date header alone is insufficient
                    req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
                },
                ValidateResponse: func(resp *httptest.ResponseRecorder) error {
                    if resp.Code != 403 {
                        return fmt.Errorf("expected status 403, got %d", resp.Code)
                    }
                    // Validate S3 error format
                    if err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken"); err != nil {
                        return fmt.Errorf("error code validation failed: %w", err)
                    }
                    // Validate error message contains expected substring (case-insensitive)
                    if err := testutil.ValidateErrorMessage(resp, "Missing Authentication"); err != nil {
                        return fmt.Errorf("error message validation failed: %w", err)
                    }
                    return nil
                },
            },
            ExpectedError: nil,
            ExpectError:   false,
        },
    }

    testutil.RunTable(t, table, func(input AuthTestInput) error {
        handler := createAuthMockHandler()
        req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

        if input.SetupRequest != nil {
            input.SetupRequest(req)
        }

        resp := testutil.MakeRequest(handler, req)

        if input.ValidateResponse != nil {
            return input.ValidateResponse(resp)
        }

        return nil
    })
}
```

### Example 2: Temporal Validation Test

```go
func TestAuthError_ExpiredToken(t *testing.T) {
    // Calculate timestamps for temporal testing
    now := time.Now().UTC()
    expiredTime := now.Add(-20 * time.Minute) // 20 minutes in the past
    futureTime := now.Add(20 * time.Minute)  // 20 minutes in the future

    table := []testutil.TableTestCase[AuthTestInput, error]{
        {
            Name:        "request with expired date header rejected",
            Description: "Tests that requests older than 15 minutes are rejected",
            Input: AuthTestInput{
                Description: "Request timestamp is outside allowed time window",
                SetupRequest: func(req *http.Request) {
                    req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
                    req.Header.Set("X-Amz-Date", expiredTime.Format("20060102T150405Z"))
                },
                ValidateResponse: func(resp *httptest.ResponseRecorder) error {
                    if resp.Code != 403 {
                        return fmt.Errorf("expected status 403, got %d", resp.Code)
                    }
                    // Validate expiration error code
                    if err := testutil.ValidateErrorCode(resp, "RequestExpired"); err != nil {
                        return fmt.Errorf("error code validation failed: %w", err)
                    }
                    return nil
                },
            },
            ExpectedError: nil,
            ExpectError:   false,
        },
    }

    testutil.RunTable(t, table, func(input AuthTestInput) error {
        handler := createAuthMockHandler()
        req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

        if input.SetupRequest != nil {
            input.SetupRequest(req)
        }

        resp := testutil.MakeRequest(handler, req)

        if input.ValidateResponse != nil {
            return input.ValidateResponse(resp)
        }

        return nil
    })
}
```

### Example 3: Using Comprehensive Assertions

```go
func TestAuthError_HTTPIntegration(t *testing.T) {
    table := []testutil.TableTestCase[AuthTestInput, error]{
        {
            Name:        "missing auth returns 403 with S3 error format",
            Description: "Tests that missing auth returns proper S3 error response",
            Input: AuthTestInput{
                Description: "Validate complete S3 error response structure",
                SetupRequest: func(req *http.Request) {
                    // No auth header
                },
                ValidateResponse: func(resp *httptest.ResponseRecorder) error {
                    // Use comprehensive assertion helpers from testutil
                    testutil.AssertAuthenticationError(t, resp)
                    return nil
                },
            },
            ExpectedError: nil,
            ExpectError:   false,
        },
    }

    testutil.RunTable(t, table, func(input AuthTestInput) error {
        handler := createAuthMockHandler()
        req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)

        if input.SetupRequest != nil {
            input.SetupRequest(req)
        }

        resp := testutil.MakeRequest(handler, req)

        if input.ValidateResponse != nil {
            return input.ValidateResponse(resp)
        }

        return nil
    })
}
```

## Adding New Error Tests

### Step 1: Define Your Test Input Structure

Create a structure to hold your test case data:

```go
// MyTestInput encapsulates inputs for your testing.
type MyTestInput struct {
    Description      string
    SetupRequest     func(*http.Request)
    ValidateResponse func(*httptest.ResponseRecorder) error
}
```

### Step 2: Create Test Cases Using TableTestCase

```go
table := []testutil.TableTestCase[MyTestInput, error]{
    {
        Name:        "descriptive test name",
        Description: "what this test validates",
        Input: MyTestInput{
            Description: "test scenario description",
            SetupRequest: func(req *http.Request) {
                // Configure the request
            },
            ValidateResponse: func(resp *httptest.ResponseRecorder) error {
                // Validate the response
                return nil
            },
        },
        ExpectedError: nil,
        ExpectError:   false,
    },
}
```

### Step 3: Create a Mock Handler (if needed)

```go
func createMyMockHandler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Implement mock behavior
        // Return appropriate errors
    })
}
```

### Step 4: Run the Test Table

```go
testutil.RunTable(t, table, func(input MyTestInput) error {
    handler := createMyMockHandler()
    req := testutil.NewTestRequest("GET", "/test-path", nil)

    if input.SetupRequest != nil {
        input.SetupRequest(req)
    }

    resp := testutil.MakeRequest(handler, req)

    if input.ValidateResponse != nil {
        return input.ValidateResponse(resp)
    }

    return nil
})
```

## Test Table Structure

### TableTestCase Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `Name` | string | Yes | Test case name for identification and reporting |
| `Description` | string | No | Detailed explanation of what is being tested |
| `Input` | I | Yes | The input to test (your custom type) |
| `ExpectedError` | E | No | The expected error (nil if no error expected) |
| `ExpectError` | bool | No | Whether an error is expected (convenience field) |
| `ErrorContains` | string | No | Substring that should be in the error message |
| `ErrorIs` | error | No | Error that should match using errors.Is |
| `Skip` | bool | No | Whether to skip this test case |
| `Tags` | []string | No | Tags for test filtering |

### Test Organization Patterns

#### Pattern 1: Organize by Error Type

```go
func TestAuthenticationErrors(t *testing.T) {
    t.Run("missing auth header", func(t *testing.T) {
        table := []testutil.TableTestCase[AuthTestInput, error]{...}
        testutil.RunTable(t, table, runner)
    })

    t.Run("invalid credentials", func(t *testing.T) {
        table := []testutil.TableTestCase[AuthTestInput, error]{...}
        testutil.RunTable(t, table, runner)
    })

    t.Run("expired token", func(t *testing.T) {
        table := []testutil.TableTestCase[AuthTestInput, error]{...}
        testutil.RunTable(t, table, runner)
    })
}
```

#### Pattern 2: Organize by Success/Failure

```go
func TestMyFeature(t *testing.T) {
    t.Run("success cases", func(t *testing.T) {
        table := []testutil.TableTestCase[MyTestInput, error]{
            // Success cases
        }
        testutil.RunTable(t, table, runner)
    })

    t.Run("error cases", func(t *testing.T) {
        table := []testutil.TableTestCase[MyTestInput, error]{
            // Error cases
        }
        testutil.RunTable(t, table, runner)
    })
}
```

## Validation Helpers

### Request Builders

#### NewTestRequest

Creates a basic test request:

```go
req := testutil.NewTestRequest("GET", "/test-bucket/test-key", nil)
```

#### WithAuthHeader

Adds AWS-style authorization header:

```go
req = testutil.WithAuthHeader(req, "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234")
```

#### WithDateHeader

Adds X-Amz-Date header:

```go
req = testutil.WithDateHeader(req, time.Now())
```

#### WithHeader

Adds a custom header:

```go
req = testutil.WithHeader(req, "X-Custom-Header", "value")
```

#### WithExpiredDate

Sets the date header to an expired timestamp:

```go
req = testutil.WithExpiredDate(req) // Date will be 20 minutes in the past
```

### Response Validators

#### ValidateStatusCode

```go
err := testutil.ValidateStatusCode(resp, 403)
if err != nil {
    t.Errorf("Status validation failed: %v", err)
}
```

#### ValidateErrorCode

```go
err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken")
if err != nil {
    t.Errorf("Error code validation failed: %v", err)
}
```

#### ValidateErrorMessage

```go
err := testutil.ValidateErrorMessage(resp, "authentication")
if err != nil {
    t.Errorf("Error message validation failed: %v", err)
}
```

#### ValidateContentType

```go
err := testutil.ValidateContentType(resp, "application/xml")
if err != nil {
    t.Errorf("Content-Type validation failed: %v", err)
}
```

### Comprehensive Assertions

#### AssertErrorResponse

Validates complete error response:

```go
testutil.AssertErrorResponse(t, resp, "MissingAuthenticationToken", 403)
```

#### AssertAuthenticationError

Validates authentication error:

```go
testutil.AssertAuthenticationError(t, resp)
```

#### AssertAuthorizationError

Validates authorization error:

```go
testutil.AssertAuthorizationError(t, resp)
```

#### AssertStatusCode

Validates response status code:

```go
testutil.AssertStatusCode(t, resp, 200)
```

#### AssertContentType

Validates response content type:

```go
testutil.AssertContentType(t, resp, "application/xml")
```

## Common Error Types

### Authentication Errors

#### Missing Authentication Token

```go
{
    Name:        "missing auth header",
    Description: "Tests that missing Authorization header returns authentication error",
    Input: AuthTestInput{
        Description: "No Authorization header provided",
        SetupRequest: func(req *http.Request) {
            // No auth header set
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            if resp.Code != 403 {
                return fmt.Errorf("expected status 403, got %d", resp.Code)
            }
            if err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken"); err != nil {
                return fmt.Errorf("error code validation failed: %w", err)
            }
            return nil
        },
    },
    ExpectedError: nil,
    ExpectError:   false,
}
```

#### Invalid Access Key ID

```go
{
    Name:        "invalid access key",
    Description: "Tests that an unknown access key is rejected",
    Input: AuthTestInput{
        Description: "Access key not found in credentials store",
        SetupRequest: func(req *http.Request) {
            req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=INVALIDKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
            req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            if resp.Code != 403 {
                return fmt.Errorf("expected status 403, got %d", resp.Code)
            }
            if err := testutil.ValidateErrorCode(resp, "InvalidAccessKeyId"); err != nil {
                return fmt.Errorf("error code validation failed: %w", err)
            }
            return nil
        },
    },
    ExpectedError: nil,
    ExpectError:   false,
}
```

#### Signature Does Not Match

```go
{
    Name:        "signature mismatch",
    Description: "Tests that incorrect signature is rejected",
    Input: AuthTestInput{
        Description: "Calculated signature does not match provided signature",
        SetupRequest: func(req *http.Request) {
            req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=wrongsignature")
            req.Header.Set("X-Amz-Date", time.Now().UTC().Format("20060102T150405Z"))
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            if resp.Code != 403 {
                return fmt.Errorf("expected status 403, got %d", resp.Code)
            }
            if err := testutil.ValidateErrorCode(resp, "SignatureDoesNotMatch"); err != nil {
                return fmt.Errorf("error code validation failed: %w", err)
            }
            return nil
        },
    },
    ExpectedError: nil,
    ExpectError:   false,
}
```

### Temporal Errors

#### Request Expired

```go
{
    Name:        "expired date header",
    Description: "Tests that requests older than 15 minutes are rejected",
    Input: AuthTestInput{
        Description: "Request timestamp is outside allowed time window",
        SetupRequest: func(req *http.Request) {
            expiredTime := time.Now().UTC().Add(-20 * time.Minute)
            req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
            req.Header.Set("X-Amz-Date", expiredTime.Format("20060102T150405Z"))
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            if resp.Code != 403 {
                return fmt.Errorf("expected status 403, got %d", resp.Code)
            }
            if err := testutil.ValidateErrorCode(resp, "RequestExpired"); err != nil {
                return fmt.Errorf("error code validation failed: %w", err)
            }
            return nil
        },
    },
    ExpectedError: nil,
    ExpectError:   false,
}
```

#### Missing Date Header

```go
{
    Name:        "missing date header",
    Description: "Tests that missing X-Amz-Date header is rejected",
    Input: AuthTestInput{
        Description: "X-Amz-Date header is required for authentication",
        SetupRequest: func(req *http.Request) {
            req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential=TESTKEY/20130524/us-east-1/s3/aws4_request, SignedHeaders=host, Signature=testsig")
            // No date header set
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            if resp.Code != 403 {
                return fmt.Errorf("expected status 403, got %d", resp.Code)
            }
            if err := testutil.ValidateErrorCode(resp, "MissingDateHeader"); err != nil {
                return fmt.Errorf("error code validation failed: %w", err)
            }
            return nil
        },
    },
    ExpectedError: nil,
    ExpectError:   false,
}
```

### Resource Errors

#### Not Found (404)

```go
{
    Name:        "resource not found",
    Description: "Tests that missing resource returns 404",
    Input: MyTestInput{
        Description: "Requested resource does not exist",
        SetupRequest: func(req *http.Request) {
            // Configure request for non-existent resource
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            if resp.Code != 404 {
                return fmt.Errorf("expected status 404, got %d", resp.Code)
            }
            if err := testutil.ValidateErrorCode(resp, "NoSuchKey"); err != nil {
                return fmt.Errorf("error code validation failed: %w", err)
            }
            return nil
        },
    },
    ExpectedError: nil,
    ExpectError:   false,
}
```

#### Access Denied (403)

```go
{
    Name:        "access denied",
    Description: "Tests that unauthorized access returns 403",
    Input: MyTestInput{
        Description: "User does not have permission to access resource",
        SetupRequest: func(req *http.Request) {
            // Configure request with restricted credentials
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            if resp.Code != 403 {
                return fmt.Errorf("expected status 403, got %d", resp.Code)
            }
            if err := testutil.ValidateErrorCode(resp, "AccessDenied"); err != nil {
                return fmt.Errorf("error code validation failed: %w", err)
            }
            return nil
        },
    },
    ExpectedError: nil,
    ExpectError:   false,
}
```

## Troubleshooting

### Common Issues and Solutions

#### Issue 1: Test Fails with "Expected status X, got Y"

**Problem:** HTTP status doesn't match expectation

**Solutions:**
1. Verify the expected status matches actual behavior
2. Check if request setup is correct (method, path, headers)
3. Ensure mock handler is properly configured
4. Verify authentication credentials are valid (if auth required)

**Debug Example:**
```go
// Add debug logging
t.Logf("Request: %s %s", req.Method, req.URL.Path)
t.Logf("Request headers: %v", req.Header)
t.Logf("Response status: %d", resp.Code)
t.Logf("Response body: %s", resp.Body.String())
```

#### Issue 2: Test Fails with "Expected code X, got Y"

**Problem:** S3 error code doesn't match expectation

**Solutions:**
1. Verify error code is correctly defined
2. Check if server is returning the expected error type
3. Ensure error code matches the actual failure scenario
4. Review error response XML to see actual code

**Debug Example:**
```go
// Parse and print error code
var s3Err testutil.S3Error
if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err == nil {
    t.Logf("Actual error code: %s", s3Err.Code)
    t.Logf("Actual error message: %s", s3Err.Message)
}
```

#### Issue 3: Test Fails with "Message validation failed"

**Problem:** Message content doesn't match keywords

**Solutions:**
1. Check if message keywords are too specific
2. Use keyword-based validation instead of exact match
3. Verify message isn't changing between server versions
4. Consider relaxing validation if message format varies

**Debug Example:**
```go
// Print actual message
var s3Err testutil.S3Error
if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err == nil {
    t.Logf("Actual message: %s", s3Err.Message)
    t.Logf("Expected keywords: %v", []string{"keyword1", "keyword2"})
}
```

#### Issue 4: Time-Related Tests Fail Intermittently

**Problem:** Tests using `time.Now()` fail when run near boundaries

**Solutions:**
1. Use UTC time consistently: `time.Now().UTC()`
2. Add margin to temporal checks
3. Use fixed timestamps in tests when possible
4. Account for time zone differences

**Best Practice:**
```go
// Good: Use UTC explicitly
now := time.Now().UTC()
expiredTime := now.Add(-20 * time.Minute)

// Avoid: Local time without explicit UTC
now := time.Now()
```

#### Issue 5: Mock Handler Not Behaving as Expected

**Problem:** Mock handler returns unexpected responses

**Solutions:**
1. Verify mock handler logic matches test expectations
2. Check header and parameter validation order
3. Ensure mock handler returns proper error codes
4. Validate mock handler's XML error format

**Debug Example:**
```go
// Add logging to mock handler
func createAuthMockHandler() http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("Request: %s %s", r.Method, r.URL.Path)
        log.Printf("Headers: %v", r.Header)
        
        auth := r.Header.Get("Authorization")
        date := r.Header.Get("X-Amz-Date")
        
        log.Printf("Auth: %s, Date: %s", auth, date)
        
        // ... rest of handler logic
    })
}
```

### Testing Tips

#### Tip 1: Start with Simple Tests

Begin with basic validation before adding complex scenarios:

```go
// Start simple
{
    Name: "basic validation",
    Input: MyTestInput{
        SetupRequest: func(req *http.Request) {
            // Minimal setup
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            // Basic status check only
            if resp.Code != 403 {
                return fmt.Errorf("expected 403, got %d", resp.Code)
            }
            return nil
        },
    },
}

// Then add complexity
{
    Name: "complete validation",
    Input: MyTestInput{
        SetupRequest: func(req *http.Request) {
            // Full setup
        },
        ValidateResponse: func(resp *httptest.ResponseRecorder) error {
            // Status, error code, message, headers
            // ... comprehensive validation
        },
    },
}
```

#### Tip 2: Use Descriptive Test Names

Clear names help identify failing tests:

```go
// Good: Descriptive
"missing auth header returns 403"
"expired token returns RequestExpired"
"invalid key returns InvalidAccessKeyId"

// Avoid: Vague
"auth test 1"
"error test"
"test case"
```

#### Tip 3: Validate Multiple Aspects

Don't stop at basic validation:

```go
ValidateResponse: func(resp *httptest.ResponseRecorder) error {
    // 1. Status code
    if resp.Code != 403 {
        return fmt.Errorf("expected status 403, got %d", resp.Code)
    }
    
    // 2. Error code
    if err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken"); err != nil {
        return fmt.Errorf("error code validation failed: %w", err)
    }
    
    // 3. Error message
    if err := testutil.ValidateErrorMessage(resp, "authentication"); err != nil {
        return fmt.Errorf("error message validation failed: %w", err)
    }
    
    // 4. Content type
    if err := testutil.ValidateContentType(resp, "application/xml"); err != nil {
        return fmt.Errorf("content type validation failed: %w", err)
    }
    
    return nil
}
```

## Best Practices

### 1. Use Table-Driven Tests

Prefer table-driven tests over individual test functions:

```go
// Good: Table-driven
func TestAuthErrors(t *testing.T) {
    table := []testutil.TableTestCase[AuthTestInput, error]{
        { /* case 1 */ },
        { /* case 2 */ },
        { /* case 3 */ },
    }
    testutil.RunTable(t, table, runner)
}

// Avoid: Separate functions
func TestAuthError1(t *testing.T) { /* ... */ }
func TestAuthError2(t *testing.T) { /* ... */ }
func TestAuthError3(t *testing.T) { /* ... */ }
```

### 2. Separate Test Data from Test Logic

Keep test configuration separate from execution:

```go
// Good: Clear separation
table := []testutil.TableTestCase[AuthTestInput, error]{
    {
        Name: "test name",
        Input: AuthTestInput{
            Description: "test description",
            SetupRequest: func(req *http.Request) { /* ... */ },
            ValidateResponse: func(resp *httptest.ResponseRecorder) error { /* ... */ },
        },
    },
}

testutil.RunTable(t, table, runner)
```

### 3. Use Helper Functions

Leverage validation helpers instead of manual checks:

```go
// Good: Use helpers
if err := testutil.ValidateErrorCode(resp, "MissingAuthenticationToken"); err != nil {
    return fmt.Errorf("error code validation failed: %w", err)
}

// Avoid: Manual parsing
var s3Err S3Error
if err := xml.Unmarshal(resp.Body.Bytes(), &s3Err); err != nil {
    return err
}
if s3Err.Code != "MissingAuthenticationToken" {
    return fmt.Errorf("wrong error code")
}
```

### 4. Add Comprehensive Documentation

Document complex test scenarios:

```go
{
    Name:        "expired date header rejected",
    Description: "Tests that requests older than 15 minutes are rejected. " +
                 "This validates the temporal validation window for authentication. " +
                 "The server rejects requests with timestamps outside the ±15 minute window.",
    Input: AuthTestInput{
        Description: "Request timestamp is 20 minutes in the past, outside allowed window",
        // ...
    },
}
```

### 5. Use UTC Time for Temporal Tests

Always use UTC for time-related tests:

```go
// Good: UTC
now := time.Now().UTC()
expiredTime := now.Add(-20 * time.Minute)
req.Header.Set("X-Amz-Date", expiredTime.Format("20060102T150405Z"))

// Avoid: Local time
now := time.Now()
expiredTime := now.Add(-20 * time.Minute)
```

### 6. Test Both Success and Failure Cases

Include both positive and negative test cases:

```go
table := []testutil.TableTestCase[AuthTestInput, error]{
    // Success case
    {
        Name:        "valid authentication succeeds",
        Description: "Tests that valid credentials are accepted",
        Input: AuthTestInput{
            SetupRequest: func(req *http.Request) {
                // Valid auth setup
            },
            ValidateResponse: func(resp *httptest.ResponseRecorder) error {
                if resp.Code != 200 {
                    return fmt.Errorf("expected 200, got %d", resp.Code)
                }
                return nil
            },
        },
        ExpectedError: nil,
        ExpectError:   false,
    },
    
    // Failure case
    {
        Name:        "invalid authentication fails",
        Description: "Tests that invalid credentials are rejected",
        Input: AuthTestInput{
            SetupRequest: func(req *http.Request) {
                // Invalid auth setup
            },
            ValidateResponse: func(resp *httptest.ResponseRecorder) error {
                if resp.Code != 403 {
                    return fmt.Errorf("expected 403, got %d", resp.Code)
                }
                return nil
            },
        },
        ExpectedError: nil,
        ExpectError:   false,
    },
}
```

### 7. Organize Tests Logically

Group related tests together:

```go
func TestAuthenticationErrors(t *testing.T) {
    t.Run("missing auth header", func(t *testing.T) {
        table := []testutil.TableTestCase[AuthTestInput, error]{...}
        testutil.RunTable(t, table, runner)
    })

    t.Run("invalid credentials", func(t *testing.T) {
        table := []testutil.TableTestCase[AuthTestInput, error]{...}
        testutil.RunTable(t, table, runner)
    })

    t.Run("temporal validation", func(t *testing.T) {
        table := []testutil.TableTestCase[AuthTestInput, error]{...}
        testutil.RunTable(t, table, runner)
    })
}
```

## Running Tests

### Run All Tests

```bash
go test ./internal/server/...
```

### Run Specific Test

```bash
go test -run TestAuthError_MissingAuthHeader ./internal/server/...
```

### Run with Verbosity

```bash
go test -v ./internal/server/...
```

### Run Specific Test Function with Verbosity

```bash
go test -v -run TestAuthError ./internal/server/...
```

## Additional Resources

### Documentation

- **Test Table Framework**: `internal/testutil/README.md`
- **Validation Helpers**: `internal/testutil/validation_helpers.go`
- **Error Testing Framework**: `docs/error-testing-framework-guide.md`

### Example Code

- **Authentication Error Tests**: `internal/server/auth_error_table_test.go`
- **Test Utilities**: `internal/testutil/*.go`

### Related Documentation

- **S3 Error Codes**: [AWS S3 Error Response Reference](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html)
- **HTTP Status Codes**: [MDN HTTP Status Reference](https://developer.mozilla.org/en-US/docs/Web/HTTP/Status)
- **Go Testing**: [Go Testing Package](https://golang.org/pkg/testing/)

---

**Last Updated:** 2026-07-16  
**ARMOR Version:** 0.1.1852+  
**Maintained By:** ARMOR Development Team
