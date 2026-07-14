# ARMOR Test Table Extension Guide

## Overview

This guide provides comprehensive documentation on how to extend ARMOR's test table structure for new error types. It includes patterns, examples, and best practices for adding custom error test cases to the existing framework.

## Table of Contents

1. [Understanding Test Table Structure](#understanding-test-table-structure)
2. [Extension Patterns](#extension-patterns)
3. [Adding New Error Types](#adding-new-error-types)
4. [Category Organization](#category-organization)
5. [Real-World Examples](#real-world-examples)
6. [Best Practices](#best-practices)
7. [Troubleshooting](#troubleshooting)

## Understanding Test Table Structure

### Core Types

The ARMOR test framework is built on two primary types:

#### ARMORErrorTestCase

```go
type ARMORErrorTestCase struct {
    Name              string              // Test case name for reporting
    Description       string              // Detailed explanation of test purpose
    StatusCode        int                 // Expected HTTP status code
    ErrorCode         string              // Expected S3 error code
    Message           string              // Expected error message
    Path              string              // Request path that triggers error
    Method            string              // HTTP method (default: "GET")
    Headers           map[string]string   // Expected response headers
    Body              string              // Request body for POST/PUT
    QueryParams       map[string]string   // Query parameters
    ExpectXMLStructure bool              // Validate S3 XML format
    MinResponseTime   time.Duration       // Minimum expected response time
    MaxResponseTime   time.Duration       // Maximum expected response time
    Category          string              // Category for organization
    Tags              []string            // Tags for filtering
    SkipReason        string              // Skip test with reason
    SetupFunc         func(*testing.T, *ConfigurableErrorServer)  // Custom setup
    ValidateFunc      func(*testing.T, *http.Response)               // Custom validation
}
```

#### ARMORErrorTestTable

```go
type ARMORErrorTestTable struct {
    Name        string                           // Table name
    Description string                           // Table purpose
    TestCases   []ARMORErrorTestCase            // Collection of test cases
    SetupFunc   func(*testing.T)                // Global setup
    TeardownFunc func(*testing.T)               // Global teardown
}
```

### Predefined Tables

ARMOR provides several predefined test tables:

```go
// Basic S3 error scenarios (404, 403, 400, 500)
ARMORErrorTestTables.BasicErrorTests()

// Authentication errors (missing token, invalid key, signature mismatch)
ARMORErrorTestTables.AuthenticationErrors()

// Input validation errors (unsupported media type, method not allowed)
ARMORErrorTestTables.ValidationErrors()

// Server errors (503 service unavailable)
ARMORErrorTestTables.ServerErrors()

// S3 protocol compliance tests
ARMORErrorTestTables.S3ProtocolErrors()

// CORS header validation
ARMORErrorTestTables.CORSHeaders()

// All tests combined
ARMORErrorTestTables.AllTests()
```

## Extension Patterns

### Pattern 1: Extend Existing Table

The simplest pattern is to add custom test cases to an existing table:

```go
func TestExtendedBasicErrors(t *testing.T) {
    // Get the base table
    base := ARMORErrorTestTables.BasicErrorTests()

    // Define custom test cases
    customCases := []ARMORErrorTestCase{
        {
            Name:       "Custom rate limit error",
            StatusCode: 429,
            ErrorCode:  "TooManyRequests",
            Message:    "Rate limit exceeded",
            Path:       "/armor/blobs/file.dat",
            Category:   "RateLimit",
            Tags:       []string{"rate-limit", "429", "custom"},
        },
        {
            Name:       "Custom quota exceeded",
            StatusCode: 503,
            ErrorCode:  "SlowDown",
            Message:    "Reduce request rate",
            Path:       "/armor/blobs/file.dat",
            Headers: map[string]string{
                "Retry-After": "120",
            },
            Category: "Quota",
            Tags:     []string{"quota", "503", "custom"},
        },
    }

    // Create extended table
    extended := ARMORErrorTestTable{
        Name:        "Extended Basic Errors",
        Description: "Basic errors plus custom rate limit scenarios",
        TestCases:   append(base.TestCases, customCases...),
    }

    // Run all tests
    for _, tc := range extended.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

### Pattern 2: Use Helper Function

Use the built-in helper function for cleaner code:

```go
func TestExtendedWithHelper(t *testing.T) {
    base := ARMORErrorTestTables.AuthenticationErrors()

    customCases := []ARMORErrorTestCase{
        {
            Name:       "Custom IP whitelist error",
            StatusCode: 403,
            ErrorCode:  "AccessDenied",
            Message:    "IP address not whitelisted",
            Path:       "/armor/blobs/file.dat",
            Category:   "IPWhitelist",
            Tags:       []string{"ip", "whitelist", "403"},
        },
    }

    // Use ExtendARMORTestTable helper
    extended := ExtendARMORTestTable(base, customCases)

    for _, tc := range extended.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

### Pattern 3: Create New Category Table

For a completely new error category, create a dedicated table:

```go
func TestRateLimitErrors(t *testing.T) {
    // Create a new category-specific table
    rateLimitTable := ARMORErrorTestTable{
        Name:        "Rate Limit Error Tests",
        Description: "Tests for rate limiting and quota errors",
        TestCases: []ARMORErrorTestCase{
            {
                Name:              "Rate limit exceeded",
                Description:       "Tests 429 response when rate limit is exceeded",
                StatusCode:        429,
                ErrorCode:         "TooManyRequests",
                Message:           "Rate limit exceeded",
                Path:              "/armor/blobs/file.dat",
                ExpectXMLStructure: true,
                MaxResponseTime:   200 * time.Millisecond,
                Category:          "RateLimit",
                Tags:              []string{"rate-limit", "429", "quota"},
                Headers: map[string]string{
                    "Retry-After": "60",
                    "X-RateLimit-Remaining": "0",
                },
            },
            {
                Name:              "Request quota exceeded",
                Description:       "Tests 503 response when quota is exceeded",
                StatusCode:        503,
                ErrorCode:         "SlowDown",
                Message:           "Reduce request rate",
                Path:              "/armor/blobs/file.dat",
                ExpectXMLStructure: true,
                MaxResponseTime:   300 * time.Millisecond,
                Category:          "Quota",
                Tags:              []string{"quota", "503", "slow-down"},
                Headers: map[string]string{
                    "Retry-After": "120",
                },
            },
        },
    }

    for _, tc := range rateLimitTable.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

### Pattern 4: Merge Multiple Tables

Combine multiple tables for comprehensive testing:

```go
func TestComprehensiveErrorScenarios(t *testing.T) {
    // Get multiple predefined tables
    basic := ARMORErrorTestTables.BasicErrorTests()
    auth := ARMORErrorTestTables.AuthenticationErrors()
    validation := ARMORErrorTestTables.ValidationErrors()

    // Define custom rate limit table
    customRateLimit := ARMORErrorTestTable{
        Name:        "Custom Rate Limits",
        Description: "Custom rate limit error scenarios",
        TestCases: []ARMORErrorTestCase{
            {
                Name:       "429 too many requests",
                StatusCode: 429,
                ErrorCode:  "TooManyRequests",
                Message:    "Rate limit exceeded",
                Path:       "/armor/blobs/file.dat",
                Category:   "RateLimit",
            },
        },
    }

    // Merge all tables using helper
    merged := MergeARMORTestTables(basic, auth, validation, customRateLimit)

    t.Logf("Running %d test cases from merged table", len(merged.TestCases))

    for _, tc := range merged.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

## Adding New Error Types

### Step-by-Step Process

#### Step 1: Define Error Code Constants

First, add your error code constants:

```go
// In error_test_patterns.go or appropriate constants file
const (
    ErrorCodeCustomRateLimit      = "TooManyRequests"
    ErrorCodeCustomQuotaExceeded  = "SlowDown"
    ErrorCodeCustomMaintenance   = "ServiceUnavailable"
)
```

#### Step 2: Create Error Pattern (Optional)

If you want to use predefined patterns:

```go
// CustomErrorPatterns for your specific error types
var CustomErrorPatterns = struct {
    RateLimitExceeded    ErrorScenarioConfig
    QuotaExceeded        ErrorScenarioConfig
    MaintenanceMode      ErrorScenarioConfig
}{
    RateLimitExceeded: ErrorScenarioConfig{
        Name:               "RateLimitExceeded",
        Description:        "Rate limit exceeded - too many requests",
        Category:           string(CategoryRateLimit),
        ExpectedStatus:     429,
        ExpectedCode:       ErrorCodeCustomRateLimit,
        ExpectedMessage:    "Rate limit exceeded",
        ExpectedKeywords:   []string{"rate", "limit", "exceeded"},
        MinResponseTime:    10 * time.Millisecond,
        MaxResponseTime:    200 * time.Millisecond,
    },
    QuotaExceeded: ErrorScenarioConfig{
        Name:               "QuotaExceeded",
        Description:        "Quota exceeded - slow down requests",
        Category:           string(CategoryQuota),
        ExpectedStatus:     503,
        ExpectedCode:       ErrorCodeCustomQuotaExceeded,
        ExpectedMessage:    "Reduce request rate",
        ExpectedKeywords:   []string{"slow", "down", "reduce"},
        MinResponseTime:    10 * time.Millisecond,
        MaxResponseTime:    300 * time.Millisecond,
    },
    MaintenanceMode: ErrorScenarioConfig{
        Name:               "MaintenanceMode",
        Description:        "Service unavailable for maintenance",
        Category:           string(CategoryServer),
        ExpectedStatus:     503,
        ExpectedCode:       ErrorCodeCustomMaintenance,
        ExpectedMessage:    "Service temporarily unavailable",
        ExpectedKeywords:   []string{"maintenance", "unavailable"},
        MinResponseTime:    10 * time.Millisecond,
        MaxResponseTime:    500 * time.Millisecond,
    },
}
```

#### Step 3: Create Test Cases

Define test cases using your error patterns:

```go
func TestCustomRateLimitErrors(t *testing.T) {
    testCases := []ARMORErrorTestCase{
        {
            Name:              "Rate limit on GET request",
            Description:       "Tests 429 when rate limit exceeded on GET",
            StatusCode:        429,
            ErrorCode:         ErrorCodeCustomRateLimit,
            Message:           CustomErrorPatterns.RateLimitExceeded.ExpectedMessage,
            Path:              "/armor/blobs/file.dat",
            Method:            "GET",
            ExpectXMLStructure: true,
            MaxResponseTime:   CustomErrorPatterns.RateLimitExceeded.MaxResponseTime,
            Category:          "RateLimit",
            Tags:              []string{"rate-limit", "429", "get"},
            Headers: map[string]string{
                "Retry-After":           "60",
                "X-RateLimit-Remaining": "0",
                "X-RateLimit-Limit":     "1000",
            },
        },
        {
            Name:              "Rate limit on PUT request",
            Description:       "Tests 429 when rate limit exceeded on PUT",
            StatusCode:        429,
            ErrorCode:         ErrorCodeCustomRateLimit,
            Message:           CustomErrorPatterns.RateLimitExceeded.ExpectedMessage,
            Path:              "/armor/blobs/upload.dat",
            Method:            "PUT",
            Body:              "test data",
            ExpectXMLStructure: true,
            MaxResponseTime:   CustomErrorPatterns.RateLimitExceeded.MaxResponseTime,
            Category:          "RateLimit",
            Tags:              []string{"rate-limit", "429", "put"},
        },
    }

    table := CreateARMORTestTable(
        "Rate Limit Errors",
        "Tests for rate limiting error scenarios",
        testCases,
    )

    for _, tc := range table.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

#### Step 4: Add Custom Validation (Optional)

For complex scenarios, add custom validation:

```go
func TestRateLimitWithCustomValidation(t *testing.T) {
    tc := ARMORErrorTestCase{
        Name:       "Rate limit with header validation",
        StatusCode: 429,
        ErrorCode:  ErrorCodeCustomRateLimit,
        Message:    "Rate limit exceeded",
        Path:       "/armor/blobs/file.dat",
        ValidateFunc: func(t *testing.T, resp *http.Response) {
            t.Helper()

            // Standard validation
            ValidateErrorResponse(t, resp, 429, ErrorCodeCustomRateLimit)

            // Custom header validation
            retryAfter := resp.Header.Get("Retry-After")
            if retryAfter == "" {
                t.Error("Expected Retry-After header to be present")
            }

            rateLimitRemaining := resp.Header.Get("X-RateLimit-Remaining")
            if rateLimitRemaining != "0" {
                t.Errorf("Expected X-RateLimit-Remaining to be '0', got '%s'", rateLimitRemaining)
            }

            // Validate retry-after is numeric
            if retryAfter != "" {
                if _, err := strconv.Atoi(retryAfter); err != nil {
                    t.Errorf("Retry-After should be numeric, got '%s'", retryAfter)
                }
            }
        },
    }

    TestARMORErrorScenario(t, tc)
}
```

## Category Organization

### Define Custom Categories

Organize your tests by defining custom categories:

```go
// Custom error categories
const (
    CategoryRateLimit   = "RateLimit"
    CategoryQuota       = "Quota"
    CategoryIPWhitelist = "IPWhitelist"
    CategoryMaintenance = "Maintenance"
)
```

### Filter by Category

Use category filtering for selective test execution:

```go
func TestRateLimitCategoryOnly(t *testing.T) {
    // Create a table with mixed categories
    mixedTable := ARMORErrorTestTable{
        Name:        "Mixed Category Tests",
        Description: "Tests from multiple categories",
        TestCases: []ARMORErrorTestCase{
            {
                Name:       "Rate limit test",
                StatusCode: 429,
                ErrorCode:  ErrorCodeCustomRateLimit,
                Message:    "Rate limit exceeded",
                Path:       "/armor/blobs/file.dat",
                Category:   "RateLimit",
            },
            {
                Name:       "Quota test",
                StatusCode: 503,
                ErrorCode:  ErrorCodeCustomQuotaExceeded,
                Message:    "Quota exceeded",
                Path:       "/armor/blobs/file.dat",
                Category:   "Quota",
            },
            {
                Name:       "IP whitelist test",
                StatusCode: 403,
                ErrorCode:  ErrorCodeAccessDenied,
                Message:    "IP not whitelisted",
                Path:       "/armor/blobs/file.dat",
                Category:   "IPWhitelist",
            },
        },
    }

    // Filter and run only RateLimit category
    rateLimitTests := filterARMORTestsByCategory(mixedTable, "RateLimit")

    t.Logf("Running %d rate limit tests", len(rateLimitTests))

    for _, tc := range rateLimitTests {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

### Filter by Tags

Use tags for more granular filtering:

```go
func TestFilteredByTags(t *testing.T) {
    all := ARMORErrorTestTables.AllTests()

    // Filter by specific tags
    rateLimitTests := filterARMORTestsByTag(all, "rate-limit")
    quotaTests := filterARMORTestsByTag(all, "quota")

    t.Run("Rate limit tests", func(t *testing.T) {
        for _, tc := range rateLimitTests {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    })

    t.Run("Quota tests", func(t *testing.T) {
        for _, tc := range quotaTests {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    })
}
```

## Real-World Examples

### Example 1: Rate Limiting Error Tests

Complete example showing rate limit error testing:

```go
package server_test

import (
    "testing"
    "time"

    "github.com/jedarden/armor/internal/server"
)

func TestRateLimitErrorScenarios(t *testing.T) {
    // Define rate limit test cases
    rateLimitTests := []server.ARMORErrorTestCase{
        {
            Name:              "Too many requests - default rate limit",
            Description:       "Tests 429 response when exceeding default rate limit",
            StatusCode:        429,
            ErrorCode:         "TooManyRequests",
            Message:           "Rate limit exceeded",
            Path:              "/armor/blobs/file.dat",
            Method:            "GET",
            ExpectXMLStructure: true,
            MaxResponseTime:   200 * time.Millisecond,
            Category:          "RateLimit",
            Tags:              []string{"rate-limit", "429", "default"},
            Headers: map[string]string{
                "Retry-After":           "60",
                "X-RateLimit-Remaining": "0",
                "X-RateLimit-Limit":     "1000",
                "X-RateLimit-Reset":     "1625097600",
            },
        },
        {
            Name:              "Too many requests - API rate limit",
            Description:       "Tests 429 response when exceeding API-specific rate limit",
            StatusCode:        429,
            ErrorCode:         "TooManyRequests",
            Message:           "API rate limit exceeded",
            Path:              "/api/v1/blobs/file.dat",
            Method:            "GET",
            ExpectXMLStructure: true,
            MaxResponseTime:   200 * time.Millisecond,
            Category:          "RateLimit",
            Tags:              []string{"rate-limit", "429", "api"},
            Headers: map[string]string{
                "Retry-After":                 "60",
                "X-RateLimit-Remaining":       "0",
                "X-RateLimit-Limit":           "100",
                "X-RateLimit-Scope":           "api",
                "X-RateLimit-Reset-Unix":      "1625097600",
            },
        },
        {
            Name:              "Slow down - quota exceeded",
            Description:       "Tests 503 response when quota is exceeded",
            StatusCode:        503,
            ErrorCode:         "SlowDown",
            Message:           "Reduce request rate",
            Path:              "/armor/blobs/file.dat",
            Method:            "PUT",
            ExpectXMLStructure: true,
            MaxResponseTime:   300 * time.Millisecond,
            Category:          "Quota",
            Tags:              []string{"quota", "503", "slow-down"},
            Headers: map[string]string{
                "Retry-After": "120",
            },
        },
    }

    // Create test table
    table := server.CreateARMORTestTable(
        "Rate Limit and Quota Errors",
        "Comprehensive rate limiting and quota error scenarios",
        rateLimitTests,
    )

    // Run all tests
    for _, tc := range table.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            server.TestARMORErrorScenario(t, tc)
        })
    }
}
```

### Example 2: Geographic Access Errors

Complete example for geographic access control:

```go
func TestGeographicAccessErrors(t *testing.T) {
    geoTests := []server.ARMORErrorTestCase{
        {
            Name:              "Access denied - region blocked",
            Description:       "Tests 403 when accessing from blocked region",
            StatusCode:        403,
            ErrorCode:         "AccessDenied",
            Message:           "Access from this region is restricted",
            Path:              "/armor/blobs/file.dat",
            ExpectXMLStructure: true,
            MaxResponseTime:   250 * time.Millisecond,
            Category:          "GeoBlock",
            Tags:              []string{"geo", "region", "403"},
            Headers: map[string]string{
                "X-Geo-Blocked":      "true",
                "X-Geo-Region":       "blocked-region",
                "X-Allowed-Regions":  "us-east-1,us-west-2",
            },
        },
        {
            Name:              "Access denied - country blocked",
            Description:       "Tests 403 when accessing from blocked country",
            StatusCode:        403,
            ErrorCode:         "AccessDenied",
            Message:           "Access from this country is restricted",
            Path:              "/armor/blobs/file.dat",
            ExpectXMLStructure: true,
            MaxResponseTime:   250 * time.Millisecond,
            Category:          "GeoBlock",
            Tags:              []string{"geo", "country", "403"},
            Headers: map[string]string{
                "X-Geo-Blocked":      "true",
                "X-Geo-Country":      "XX",
                "X-Allowed-Countries": "US,CA,GB",
            },
        },
    }

    table := server.CreateARMORTestTable(
        "Geographic Access Control",
        "Tests for geographic access restrictions",
        geoTests,
    )

    for _, tc := range table.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            server.TestARMORErrorScenario(t, tc)
        })
    }
}
```

### Example 3: Temporary Maintenance Errors

Complete example for maintenance mode errors:

```go
func TestMaintenanceModeErrors(t *testing.T) {
    maintenanceTests := []server.ARMORErrorTestCase{
        {
            Name:              "Service unavailable - scheduled maintenance",
            Description:       "Tests 503 during scheduled maintenance window",
            StatusCode:        503,
            ErrorCode:         "ServiceUnavailable",
            Message:           "Service temporarily unavailable for maintenance",
            Path:              "/armor/blobs/file.dat",
            ExpectXMLStructure: true,
            MaxResponseTime:   500 * time.Millisecond,
            Category:          "Maintenance",
            Tags:              []string{"maintenance", "503", "scheduled"},
            Headers: map[string]string{
                "Retry-After":              "3600",
                "X-Maintenance-Reason":      "scheduled_upgrade",
                "X-Maintenance-Start-Time":  "2026-07-14T02:00:00Z",
                "X-Maintenance-End-Time":    "2026-07-14T04:00:00Z",
            },
        },
        {
            Name:              "Service unavailable - emergency maintenance",
            Description:       "Tests 503 during emergency maintenance",
            StatusCode:        503,
            ErrorCode:         "ServiceUnavailable",
            Message:           "Emergency maintenance in progress",
            Path:              "/armor/blobs/file.dat",
            ExpectXMLStructure: true,
            MaxResponseTime:   500 * time.Millisecond,
            Category:          "Maintenance",
            Tags:              []string{"maintenance", "503", "emergency"},
            Headers: map[string]string{
                "Retry-After":              "300",
                "X-Maintenance-Reason":      "emergency_maintenance",
                "X-Maintenance-Urgency":    "high",
            },
        },
    }

    table := server.CreateARMORTestTable(
        "Maintenance Mode Errors",
        "Tests for service unavailability during maintenance",
        maintenanceTests,
    )

    for _, tc := range table.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            server.TestARMORErrorScenario(t, tc)
        })
    }
}
```

## Best Practices

### 1. Use Consistent Naming

```go
// Good: Clear, descriptive names
{
    Name: "Rate limit exceeded on PUT request",
    ErrorCode: "TooManyRequests",
}

// Avoid: Vague names
{
    Name: "Error test",
    ErrorCode: "TooManyRequests",
}
```

### 2. Provide Detailed Descriptions

```go
// Good: Explains what and why
{
    Name: "Rate limit exceeded",
    Description: "Tests 429 response when exceeding 1000 requests/minute rate limit on PUT operations",
}

// Better: Also includes edge cases
{
    Name: "Rate limit exceeded",
    Description: "Tests 429 response when exceeding 1000 requests/minute rate limit on PUT operations. Validates Retry-After header and X-RateLimit-* headers are present and correctly formatted",
}
```

### 3. Use Categories and Tags Effectively

```go
{
    Category: "RateLimit",
    Tags: []string{
        "rate-limit",      // Error type
        "429",            // Status code
        "put",            // HTTP method
        "api",            // Service scope
        "default",        // Rate limit tier
    },
}
```

### 4. Set Realistic Timing Expectations

```go
{
    MinResponseTime: 10 * time.Millisecond,  // Lower bound for fast systems
    MaxResponseTime: 200 * time.Millisecond, // Upper bound for acceptable performance
}
```

### 5. Use Custom Validation for Complex Scenarios

```go
{
    ValidateFunc: func(t *testing.T, resp *http.Response) {
        t.Helper()

        // Standard validation
        ValidateErrorResponse(t, resp, 429, "TooManyRequests")

        // Complex header validation
        validateRateLimitHeaders(t, resp)
        validateRetryAfterFormat(t, resp)
    },
}
```

### 6. Organize Tables by Feature or Category

```go
// Organize by feature
rateLimitTable := CreateARMORTestTable("Rate Limiting", "...", rateLimitTests)
geoBlockTable := CreateARMORTestTable("Geo Blocking", "...", geoTests)
maintenanceTable := CreateARMORTestTable("Maintenance", "...", maintenanceTests)

// Or organize by error type
clientErrors := CreateARMORTestTable("4xx Client Errors", "...", clientTests)
serverErrors := CreateARMORTestTable("5xx Server Errors", "...", serverTests)
```

### 7. Document External Dependencies

```go
{
    Description: "Tests rate limiting. Requires rate limit middleware to be enabled. Set RATE_LIMIT_ENABLED=true in test environment.",
    SetupFunc: func(t *testing.T, server *ConfigurableErrorServer) {
        if os.Getenv("RATE_LIMIT_ENABLED") != "true" {
            t.Skip("Rate limiting not enabled")
        }
    },
}
```

## Troubleshooting

### Issue: Test Cases Not Running

**Problem**: Test cases defined in extended table don't execute.

**Solution**: Ensure you're iterating over the correct test cases:

```go
// Correct
for _, tc := range extended.TestCases {
    t.Run(tc.Name, func(t *testing.T) {
        TestARMORErrorScenario(t, tc)
    })
}

// Wrong
for _, tc := range base.TestCases {
    // Only runs base tests, not extensions
}
```

### Issue: Category Filter Returns No Results

**Problem**: `filterARMORTestsByCategory` returns empty slice.

**Solution**: Ensure category field is set and matches exactly:

```go
{
    Category: "RateLimit",  // Must match filter exactly
}

// Filter
filtered := filterARMORTestsByCategory(table, "RateLimit")
```

### Issue: Custom Validation Not Called

**Problem**: Custom `ValidateFunc` is not executed.

**Solution**: Ensure you're using `TestARMORErrorScenario`:

```go
// Correct - uses TestARMORErrorScenario
func TestCustomValidation(t *testing.T) {
    tc := ARMORErrorTestCase{
        ValidateFunc: func(t *testing.T, resp *http.Response) {
            // Custom logic
        },
    }
    TestARMORErrorScenario(t, tc)  // This calls ValidateFunc
}

// Wrong - manual test setup
func TestCustomValidationWrong(t *testing.T) {
    tc := ARMORErrorTestCase{
        ValidateFunc: func(t *testing.T, resp *http.Response) {
            // This won't be called
        },
    }
    // Manual setup doesn't use ValidateFunc
}
```

### Issue: Test Table Merge Creates Duplicates

**Problem**: Merged tables contain duplicate test cases.

**Solution**: Filter duplicates before merging:

```go
func mergeWithoutDuplicates(tables ...ARMORErrorTestTable) ARMORErrorTestTable {
    seen := make(map[string]bool)
    var uniqueCases []ARMORErrorTestCase

    for _, table := range tables {
        for _, tc := range table.TestCases {
            if !seen[tc.Name] {
                seen[tc.Name] = true
                uniqueCases = append(uniqueCases, tc)
            }
        }
    }

    return ARMORErrorTestTable{
        Name:        "Merged Without Duplicates",
        Description: "Merged tables with duplicates removed",
        TestCases:   uniqueCases,
    }
}
```

## Summary

This guide covers the complete process of extending ARMOR test tables:

1. **Understanding the structure**: ARMORErrorTestCase and ARMORErrorTestTable
2. **Extension patterns**: Four patterns for different use cases
3. **Adding new error types**: Step-by-step process with code examples
4. **Category organization**: How to organize and filter tests
5. **Real-world examples**: Complete, runnable examples
6. **Best practices**: Guidelines for maintainable tests
7. **Troubleshooting**: Common issues and solutions

For more information, see:
- `/home/coding/ARMOR/docs/error-testing-framework-guide.md` - Overall framework guide
- `/home/coding/ARMOR/internal/server/error_testing_base.go` - Core structures
- `/home/coding/ARMOR/internal/server/error_test_example_test.go` - Usage examples
