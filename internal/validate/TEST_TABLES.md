# Validation Test Table Framework

## Overview

The validation test table framework provides a comprehensive, extensible system for writing table-driven tests for HTTP response validation in the ARMOR project. It offers prebuilt test tables for common scenarios, helper types for simplified test creation, and clear patterns for extension.

## Quick Start

### Using Prebuilt Tables

```go
for _, tc := range CommonValidationTests.StatusCodeValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

### Creating Custom Tests

```go
customCases := []ValidationTestCase{
    {
        Name:      "my custom test",
        Response:  createResponse(418),
        Expected:  418,
        WantValid: true,
        Category:  "Custom",
    },
}
```

## Available Test Tables

### CommonValidationTests

General validation tests for common scenarios.

**Available Tables:**
- `StatusCodeValidation()` - Status code validation (200, 404, 500, etc.)
- `ContentTypeValidation()` - Content-type validation (JSON, XML, charset handling)
- `ErrorStructureValidation()` - Error response structure validation
- `NilResponseHandling()` - Nil response safety tests

**Example:**

```go
for _, tc := range CommonValidationTests.StatusCodeValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

### AuthErrorTestTable

Authentication error validation tests (401, 403).

**Available Tables:**
- `StatusCodeValidation()` - Auth status code validation
- `ErrorMessageValidation()` - Auth error message pattern validation
- `ErrorCodeValidation()` - Auth error code detection (UNAUTHORIZED, FORBIDDEN, invalid_token)

**Example:**

```go
for _, tc := range AuthErrorTestTable.StatusCodeValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

### ValidationErrorTestTable

Validation error tests (400, 422).

**Available Tables:**
- `StatusCodeValidation()` - Validation status code validation
- `ErrorMessageValidation()` - Validation error message patterns

**Example:**

```go
for _, tc := range ValidationErrorTestTable.StatusCodeValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

### ServerErrorTestTable

Server error tests (500, 502, 503).

**Available Tables:**
- `StatusCodeValidation()` - Server status code validation
- `ErrorMessageValidation()` - Server error message patterns

**Example:**

```go
for _, tc := range ServerErrorTestTable.StatusCodeValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

### CORSErrorTestTable

CORS header validation tests.

**Available Tables:**
- `BasicValidation()` - Basic CORS header presence
- `WildcardValidation()` - Wildcard CORS origin validation
- `CredentialsValidation()` - CORS credentials header validation

### RateLimitErrorTestTable

Rate limiting error validation tests (429).

**Available Tables:**
- `StatusCodeValidation()` - Rate limit status code validation (429)
- `MessageValidation()` - Rate limit error message patterns (rate limit exceeded, too many requests, quota exceeded)
- `ErrorCodeValidation()` - Rate limit error code detection (RATE_LIMIT_EXCEEDED, TOO_MANY_REQUESTS)

**Example:**

```go
for _, tc := range RateLimitErrorTestTable.StatusCodeValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

### PaymentErrorTestTable

Payment and billing error validation tests (402).

**Available Tables:**
- `StatusCodeValidation()` - Payment status code validation (402)
- `MessageValidation()` - Payment error message patterns (payment required, billing failed, subscription expired)
- `ErrorCodeValidation()` - Payment error code detection (PAYMENT_REQUIRED, BILLING_FAILED, INSUFFICIENT_FUNDS)

**Example:**

```go
for _, tc := range PaymentErrorTestTable.StatusCodeValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

### CORSErrorTestTable

CORS header validation tests.

**Available Tables:**
- `BasicValidation()` - Basic CORS header presence
- `WildcardValidation()` - Wildcard CORS origin validation
- `CredentialsValidation()` - CORS credentials header validation

**Example:**

```go
for _, tc := range CORSErrorTestTable.BasicValidation() {
    t.Run(tc.Name, func(t *testing.T) {
        config := &CORSConfig{AllowOrigin: tc.Expected.(string)}
        result := CORSHeadersIsValid(tc.Response, config)
        if result != tc.WantValid {
            t.Errorf("got %v, want %v", result, tc.WantValid)
        }
    })
}
```

## Helper Types

### ValidationTestCase

The core test case structure.

```go
tc := ValidationTestCase{
    Name:        "200 OK",
    Response:    createResponse(200),
    Expected:    200,
    WantValid:   true,
    Category:    "Success",
    Tags:        []string{"status", "2xx"},
    Description: "Tests that 200 status codes are valid",
}
```

**Fields:**
- `Name` - Test case name for identification
- `Description` - Optional detailed description
- `Response` - HTTP response to validate
- `Expected` - Expected value (status code, content type, etc.)
- `WantValid` - Whether validation should pass
- `Category` - Optional category for grouping
- `Tags` - Optional tags for filtering

### StatusCodeTestCase

Simplified type for status code tests.

```go
tc := StatusCodeTestCase{
    Name:           "200 OK",
    ResponseStatus: 200,
    Expected:       200,
    WantValid:      true,
    Category:       "Success",
}
```

**Convert to ValidationTestCase:**

```go
vtc := tc.ToTestCase()
```

### ContentTypeTestCase

Simplified type for content-type tests.

```go
tc := ContentTypeTestCase{
    Name:         "JSON with charset",
    ResponseType: "application/json; charset=utf-8",
    Expected:     "application/json",
    WantValid:    true,
}
```

**Convert to ValidationTestCase:**

```go
vtc := tc.ToTestCase()
```

### HTTPTestResponse

Helper for creating test responses.

```go
resp := HTTPTestResponse{
    StatusCode: 404,
    Headers: map[string]string{
        "Content-Type": "application/json",
    },
    Body: `{"error": "not found"}`,
}.ToResponse()
```

## Extension Patterns

### Pattern 1: Extend Existing Table

Add custom cases to a predefined table:

```go
base := AuthErrorTestTable.StatusCodeValidation()
customCases := []ValidationTestCase{
    {
        Name:      "custom auth test",
        Response:  createResponse(401),
        Expected:  401,
        WantValid: true,
        Category:  "CustomAuth",
    },
}
extended := ExtendTable(base, customCases)
```

### Pattern 2: Create New Category

Define a custom error type table:

```go
var CustomErrorTestTable = struct {
    StatusCodeValidation func() []ValidationTestCase
}{
    StatusCodeValidation: func() []ValidationTestCase {
        cases := []StatusCodeTestCase{
            {Name: "418 Teapot", ResponseStatus: 418, Expected: 418, WantValid: true},
        }
        result := make([]ValidationTestCase, len(cases))
        for i, c := range cases {
            result[i] = c.ToTestCase()
        }
        return result
    },
}
```

### Pattern 3: Filter Tests

Run specific subsets of tests:

```go
allTests := CommonValidationTests.StatusCodeValidation()
authTests := FilterTable(allTests, "", "AuthError")
jsonTests := FilterTable(allTests, "json", "")
```

### Pattern 4: Merge Tables

Combine tables for comprehensive testing:

```go
allErrorTests := MergeTables(
    AuthErrorTestTable.StatusCodeValidation(),
    ValidationErrorTestTable.StatusCodeValidation(),
    ServerErrorTestTable.StatusCodeValidation(),
)
```

## Helper Functions

### ExtendTable

Adds custom test cases to an existing table:

```go
extended := ExtendTable(base, customCases)
```

### FilterTable

Filters a table by tag or category:

```go
// Filter by category
clientTests := FilterTable(allTests, "", "ClientError")

// Filter by tag
jsonTests := FilterTable(allTests, "json", "")
```

### MergeTables

Combines multiple test tables:

```go
combined := MergeTables(table1, table2, table3)
```

## Complete Examples

### Example 1: Basic Status Code Validation

```go
func TestStatusCodeValidation(t *testing.T) {
    for _, tc := range CommonValidationTests.StatusCodeValidation() {
        t.Run(tc.Name, func(t *testing.T) {
            result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
            if result != tc.WantValid {
                t.Errorf("HTTPStatusCodeIsValid() = %v, want %v", result, tc.WantValid)
            }
        })
    }
}
```

### Example 2: Authentication Error Testing

```go
func TestAuthErrors(t *testing.T) {
    t.Run("StatusCode", func(t *testing.T) {
        for _, tc := range AuthErrorTestTable.StatusCodeValidation() {
            t.Run(tc.Name, func(t *testing.T) {
                result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
                if result != tc.WantValid {
                    t.Errorf("got %v, want %v", result, tc.WantValid)
                }
            })
        }
    })

    t.Run("MessagePatterns", func(t *testing.T) {
        for _, tc := range AuthErrorTestTable.ErrorMessageValidation() {
            t.Run(tc.Name, func(t *testing.T) {
                result := ValidateErrorMessageWithDetails(tc.ResponseBody, tc.Pattern)
                if result.Found != tc.WantMessageFound {
                    t.Errorf("got %v, want %v", result.Found, tc.WantMessageFound)
                }
            })
        }
    })
}
```

### Example 2a: Rate Limit Error Testing

```go
func TestRateLimitErrors(t *testing.T) {
    t.Run("StatusCode", func(t *testing.T) {
        for _, tc := range RateLimitErrorTestTable.StatusCodeValidation() {
            t.Run(tc.Name, func(t *testing.T) {
                result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
                if result != tc.WantValid {
                    t.Errorf("got %v, want %v", result, tc.WantValid)
                }
            })
        }
    })

    t.Run("MessagePatterns", func(t *testing.T) {
        for _, tc := range RateLimitErrorTestTable.MessageValidation() {
            t.Run(tc.Name, func(t *testing.T) {
                result := ValidateErrorMessageWithDetails(tc.ResponseBody, tc.Pattern)
                if result.Found != tc.WantMessageFound {
                    t.Errorf("got %v, want %v", result.Found, tc.WantMessageFound)
                }
            })
        }
    })
}
```

### Example 2b: Payment Error Testing

```go
func TestPaymentErrors(t *testing.T) {
    t.Run("StatusCode", func(t *testing.T) {
        for _, tc := range PaymentErrorTestTable.StatusCodeValidation() {
            t.Run(tc.Name, func(t *testing.T) {
                result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
                if result != tc.WantValid {
                    t.Errorf("got %v, want %v", result, tc.WantValid)
                }
            })
        }
    })

    t.Run("ErrorCodeDetection", func(t *testing.T) {
        for _, tc := range PaymentErrorTestTable.ErrorCodeValidation() {
            t.Run(tc.Name, func(t *testing.T) {
                found := ValidateErrorCodeInResponse(tc.ResponseBody, tc.ExpectedMessage, "")
                if found != tc.WantFound {
                    t.Errorf("got %v, want %v", found, tc.WantFound)
                }
            })
        }
    })
```

### Example 3: Custom Error Type (418 I'm a Teapot)

```go
func TestCustomErrorTypes(t *testing.T) {
    // Define custom table for a unique error type
    var TeapotErrorTestTable = struct {
        StatusCodeValidation func() []ValidationTestCase
        MessageValidation    func() []MessageValidationTestCase
    }{
        StatusCodeValidation: func() []ValidationTestCase {
            cases := []StatusCodeTestCase{
                {
                    Name:           "418 I'm a teapot",
                    ResponseStatus: 418,
                    Expected:       418,
                    WantValid:      true,
                    Category:       "HTCPCP",
                    Description:    "Tests HTCPCP protocol status code",
                    Tags:           []string{"custom", "teapot", "418", "htcpcp"},
                },
            }
            result := make([]ValidationTestCase, len(cases))
            for i, c := range cases {
                result[i] = c.ToTestCase()
            }
            return result
        },
        MessageValidation: func() []MessageValidationTestCase {
            return []MessageValidationTestCase{
                {
                    ValidationTestCase: ValidationTestCase{
                        Name:        "teapot message pattern",
                        Description: "Tests HTCPCP error message",
                        Category:    "HTCPCPMessage",
                        Tags:        []string{"custom", "teapot", "message"},
                    },
                    ResponseBody: map[string]interface{}{
                        "error": "I'm a teapot - this server is a teapot, not a coffee machine",
                    },
                    Pattern: EnhancedErrorMessagePattern{
                        MustContain: []string{"teapot"},
                    },
                    WantMessageFound: true,
                },
            }
        },
    }

    // Run status code tests
    for _, tc := range TeapotErrorTestTable.StatusCodeValidation() {
        t.Run(tc.Name, func(t *testing.T) {
            result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
            if result != tc.WantValid {
                t.Errorf("got %v, want %v", result, tc.WantValid)
            }
        })
    }

    // Run message validation tests
    for _, tc := range TeapotErrorTestTable.MessageValidation() {
        t.Run(tc.Name, func(t *testing.T) {
            result := ValidateErrorMessageWithDetails(tc.ResponseBody, tc.Pattern)
            if result.Found != tc.WantMessageFound {
                t.Errorf("got %v, want %v", result.Found, tc.WantMessageFound)
            }
        })
    }
}
```

### Example 4: Extending and Filtering

```go
func TestExtendedAuthTests(t *testing.T) {
    // Get base table
    base := AuthErrorTestTable.StatusCodeValidation()

    // Add custom cases
    custom := []ValidationTestCase{
        {Name: "Custom", Response: createResponse(401), Expected: 401, WantValid: true},
    }
    extended := ExtendTable(base, custom)

    // Filter for specific category
    authTests := FilterTable(extended, "", "AuthError")

    // Run tests
    for _, tc := range authTests {
        t.Run(tc.Name, func(t *testing.T) {
            result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
            if result != tc.WantValid {
                t.Errorf("got %v, want %v", result, tc.WantValid)
            }
        })
    }
}
```

## Best Practices

### 1. Use Descriptive Names

Test case names should clearly describe what is being tested:

```go
✓ Good: "404 Not Found matches expected 404"
✗ Bad: "test 1"
```

### 2. Add Categories and Tags

Organize tests for maintainability:

```go
tc := ValidationTestCase{
    Name:      "404 Not Found",
    Category:  "ClientError",
    Tags:      []string{"status", "4xx", "not-found"},
    // ...
}
```

### 3. Document Complex Tests

Add descriptions for complex test scenarios:

```go
tc := ValidationTestCase{
    Name:        "CORS with credentials",
    Description: "Tests CORS headers with Allow-Credentials: true",
    // ...
}
```

### 4. Prefer Extension Over Creation

Extend existing tables rather than creating from scratch:

```go
✓ Good: extended := ExtendTable(base, custom)
✗ Bad: Create everything from scratch
```

### 5. Use Helper Types When Appropriate

Use simplified types for common test patterns:

```go
✓ Good for status codes: StatusCodeTestCase{ResponseStatus: 200, ...}
✓ Good for content-type: ContentTypeTestCase{ResponseType: "application/json", ...}
```

## Adding New Error Types

To add support for a new error type:

1. **Create a new test table variable:**

```go
var NewErrorTestTable = struct{
    StatusCodeValidation func() []ValidationTestCase
    MessageValidation func() []MessageValidationTestCase
}{
    // Implementation
}
```

2. **Implement the test cases:**

```go
StatusCodeValidation: func() []ValidationTestCase {
    cases := []StatusCodeTestCase{
        {Name: "418 Teapot", ResponseStatus: 418, Expected: 418, WantValid: true},
    }
    result := make([]ValidationTestCase, len(cases))
    for i, c := range cases {
        result[i] = c.ToTestCase()
    }
    return result
},
```

3. **Document the new table:**

Add usage examples and descriptions in comments.

4. **Add to documentation:**

Update this file with the new table's description and usage examples.

## Testing Guidelines

### Running All Tables

```go
func TestAllValidationTables(t *testing.T) {
    tables := []struct {
        name string
        tests []ValidationTestCase
    }{
        {"Common", CommonValidationTests.StatusCodeValidation()},
        {"Auth", AuthErrorTestTable.StatusCodeValidation()},
        {"Validation", ValidationErrorTestTable.StatusCodeValidation()},
        {"Server", ServerErrorTestTable.StatusCodeValidation()},
        {"RateLimit", RateLimitErrorTestTable.StatusCodeValidation()},
        {"Payment", PaymentErrorTestTable.StatusCodeValidation()},
    }

    for _, table := range tables {
        t.Run(table.name, func(t *testing.T) {
            for _, tc := range table.tests {
                t.Run(tc.Name, func(t *testing.T) {
                    result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
                    if result != tc.WantValid {
                        t.Errorf("got %v, want %v", result, tc.WantValid)
                    }
                })
            }
        })
    }
}
```

### Running Specific Categories

```go
func TestOnlyAuthErrors(t *testing.T) {
    authTests := FilterTable(
        AuthErrorTestTable.StatusCodeValidation(),
        "", "AuthError",
    )

    for _, tc := range authTests {
        t.Run(tc.Name, func(t *testing.T) {
            result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
            if result != tc.WantValid {
                t.Errorf("got %v, want %v", result, tc.WantValid)
            }
        })
    }
}
```

## File Organization

- **test_tables.go** - Core test table structures and prebuilt tables
- **test_tables_example_test.go** - Example usage and demonstrations
- **TEST_TABLES.md** - This documentation file

## Summary

The validation test table framework provides:

- ✅ **Prebuilt test tables** for common error scenarios
- ✅ **Helper types** for simplified test creation
- ✅ **Extension patterns** for custom test cases
- ✅ **Filter and merge** helpers for test organization
- ✅ **Type safety** with strongly-typed structures
- ✅ **Self-documenting** with clear field names and descriptions
- ✅ **Extensible** design for new error types

Use this framework to write consistent, maintainable validation tests across the ARMOR project.
