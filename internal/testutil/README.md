# Generic Test Table Framework

## Overview

The `testutil` package provides a comprehensive, generic test table framework for writing table-driven tests in Go. Unlike HTTP-specific test frameworks, this framework is designed to work with any type of test case, any input type, and any error type.

## Features

- ✅ **Generic**: Works with any input and error types
- ✅ **Type-safe**: Strong typing prevents errors at compile time
- ✅ **Reusable**: Test tables can be shared across multiple test files
- ✅ **Extensible**: Easy to extend, filter, and manipulate test tables
- ✅ **Convenient**: Helper functions for common test patterns
- ✅ **Self-documenting**: Clear field names and comprehensive documentation

## Quick Start

### Basic Example

```go
func TestMyFunction(t *testing.T) {
    // Define test table
    table := []TableTestCase[string, error]{
        {
            Name:        "empty string returns error",
            Description: "Tests that empty string returns error",
            Input:       "",
            ExpectedError: ErrEmpty,
            ExpectError:  true,
        },
        {
            Name:        "valid string succeeds",
            Description: "Tests that valid string succeeds",
            Input:       "valid",
            ExpectedError: nil,
            ExpectError:  false,
        },
    }

    // Run test table
    RunTable(t, table, func(input string) error {
        return MyFunction(input)
    })
}
```

### Using Helper Functions

```go
func TestMyFunction(t *testing.T) {
    table := []TableTestCase[string, error]{
        SuccessCase("valid input", "test"),
        ErrorCase("invalid input", "", ErrEmpty),
        ErrorCaseWithMessage("format error", "invalid", ErrInvalidFormat, "invalid format"),
    }

    RunTable(t, table, func(input string) error {
        return MyFunction(input)
    })
}
```

### Using Table Builder

```go
func TestMyFunction(t *testing.T) {
    table := NewTableBuilder[string, error]().
        WithName("my tests").
        WithDescription("Test cases for my function").
        WithTags([]string{"validation"}).
        WithTestCase(TableTestCase[string, error]{
            Name: "test case",
            Input: "input",
            ExpectedError: nil,
            ExpectError: false,
        }).
        Build()

    RunTable(t, table.TestCases, func(input string) error {
        return MyFunction(input)
    })
}
```

## Core Types

### TableTestCase

Represents a single test case with the following fields:

- `Name`: Test case name (required)
- `Description`: Optional detailed description
- `Input`: The input to test (any type)
- `ExpectedError`: The expected error
- `ExpectError`: Whether an error is expected
- `ErrorContains`: Optional substring for error message validation
- `ErrorIs`: Optional error for `errors.Is` validation
- `Skip`: Whether to skip this test case
- `Tags`: Optional tags for filtering

### TableTestTable

Represents a collection of test cases:

- `Name`: Table name
- `Description`: Optional table description
- `TestCases`: Collection of test cases
- `Tags`: Optional tags for the entire table

## Helper Functions

### Test Case Creation

- `SuccessCase(name, input)`: Creates a test case expecting success
- `ErrorCase(name, input, error)`: Creates a test case expecting an error
- `ErrorCaseWithMessage(name, input, error, message)`: Creates a test case expecting an error with message validation

### Test Execution

- `RunTable(t, table, runner)`: Executes all test cases in a table

### Table Manipulation

- `ExtendTable(base, custom)`: Extends a table with custom test cases
- `FilterTable(table, tag)`: Filters a table by tag
- `FilterTableByTag(table, excludeTag)`: Filters a table to exclude tests with a specific tag

### Assertion Helpers

- `AssertNoError(t, err, msg)`: Asserts that no error occurred
- `AssertError(t, err, msg)`: Asserts that an error occurred
- `AssertErrorIs(t, err, expected, msg)`: Asserts that the error matches the expected error
- `AssertErrorContains(t, err, substring, msg)`: Asserts that the error message contains a substring

## Examples

### Example 1: String Validation

```go
func TestStringValidation(t *testing.T) {
    table := []TableTestCase[string, error]{
        SuccessCase("valid string", "test@example.com"),
        ErrorCase("empty string", "", ErrEmpty),
        ErrorCaseWithMessage("too short", "ab", ErrTooShort, "at least 3 characters"),
        ErrorCaseWithMessage("too long", strings.Repeat("a", 51), ErrTooLong, "exceed 50"),
    }

    RunTable(t, table, ValidateString)
}
```

### Example 2: Numeric Validation

```go
func TestNumericValidation(t *testing.T) {
    table := []TableTestCase[int, error]{
        {Name: "negative", Input: -1, ExpectError: true, ErrorContains: "non-negative"},
        {Name: "zero", Input: 0, ExpectError: false},
        {Name: "valid", Input: 50, ExpectError: false},
        {Name: "maximum", Input: 100, ExpectError: false},
        {Name: "exceeds max", Input: 101, ExpectError: true, ErrorContains: "exceed 100"},
    }

    RunTable(t, table, ValidateNumber)
}
```

### Example 3: Complex Input

```go
type ComplexInput struct {
    Name   string
    Count  int
    Active bool
}

func TestComplexInput(t *testing.T) {
    table := []TableTestCase[ComplexInput, error]{
        {
            Name: "valid input",
            Input: ComplexInput{Name: "test", Count: 5, Active: true},
            ExpectError: false,
        },
        {
            Name: "empty name",
            Input: ComplexInput{Name: "", Count: 5, Active: true},
            ExpectError: true,
            ExpectedError: ErrEmptyName,
        },
        {
            Name: "negative count",
            Input: ComplexInput{Name: "test", Count: -1, Active: true},
            ExpectError: true,
            ExpectedError: ErrNegativeCount,
        },
    }

    RunTable(t, table, ProcessComplexInput)
}
```

### Example 4: Table Extension

```go
func TestExtendedValidation(t *testing.T) {
    // Base test cases
    baseCases := []TableTestCase[string, error]{
        SuccessCase("valid 1", "test1"),
        SuccessCase("valid 2", "test2"),
        ErrorCase("invalid", "", ErrEmpty),
    }

    // Custom cases
    customCases := []TableTestCase[string, error]{
        {
            Name: "custom case",
            Input: "custom",
            ExpectError: false,
            Tags: []string{"custom"},
        },
    }

    // Extend and run
    extended := ExtendTable(baseCases, customCases)
    RunTable(t, extended, ValidateInput)
}
```

### Example 5: Filtered Tests

```go
func TestFilteredValidation(t *testing.T) {
    allTests := []TableTestCase[string, error]{
        {Name: "test 1", Input: "test1", ExpectError: false, Tags: []string{"smoke"}},
        {Name: "test 2", Input: "test2", ExpectError: false, Tags: []string{"regression"}},
        {Name: "test 3", Input: "test3", ExpectError: false, Tags: []string{"smoke"}},
    }

    // Run only smoke tests
    smokeTests := FilterTable(allTests, "smoke")
    RunTable(t, smokeTests, ValidateInput)

    // Run only regression tests
    regressionTests := FilterTable(allTests, "regression")
    RunTable(t, regressionTests, ValidateInput)
}
```

## Best Practices

### 1. Use Descriptive Names

```go
✓ Good: "empty string returns error"
✗ Bad: "test 1"
```

### 2. Add Descriptions for Complex Tests

```go
{
    Name: "inactive with high count",
    Description: "Tests that inactive status with count > 5 returns error",
    Input: ComplexInput{Active: false, Count: 10},
    ExpectError: true,
}
```

### 3. Use Helper Functions When Appropriate

```go
✓ Good: SuccessCase("valid input", "test")
✓ Good: ErrorCase("invalid input", "", ErrEmpty)
✗ Bad: Repetitive test case creation
```

### 4. Use Tags for Filtering

```go
{
    Name: "critical test",
    Input: "input",
    ExpectError: false,
    Tags: []string{"smoke", "critical", "auth"},
}
```

### 5. Validate Error Messages

```go
{
    Name: "invalid input",
    Input: "invalid",
    ExpectedError: ErrInvalid,
    ExpectError: true,
    ErrorContains: "invalid format", // Validate error message
}
```

### 6. Organize Tests by Category

```go
func TestMyFunction(t *testing.T) {
    t.Run("success cases", func(t *testing.T) {
        table := []TableTestCase[string, error]{...}
        RunTable(t, table, MyFunction)
    })

    t.Run("error cases", func(t *testing.T) {
        table := []TableTestCase[string, error]{...}
        RunTable(t, table, MyFunction)
    })
}
```

## Advanced Features

### Custom Error Types

The framework works with any error type:

```go
type MyError struct {
    Code int
    Message string
}

func (e MyError) Error() string {
    return fmt.Sprintf("%d: %s", e.Code, e.Message)
}

func TestCustomError(t *testing.T) {
    table := []TableTestCase[string, MyError]{
        {
            Name: "returns custom error",
            Input: "invalid",
            ExpectedError: MyError{Code: 400, Message: "invalid"},
            ExpectError: true,
        },
    }

    RunTable(t, table, MyFunction)
}
```

### Table-Level Organization

```go
func TestComprehensiveValidation(t *testing.T) {
    tables := []struct {
        name string
        cases []TableTestCase[string, error]
    }{
        {
            name: "empty input",
            cases: []TableTestCase[string, error]{...},
        },
        {
            name: "invalid format",
            cases: []TableTestCase[string, error]{...},
        },
    }

    for _, table := range tables {
        t.Run(table.name, func(t *testing.T) {
            RunTable(t, table.cases, ValidateInput)
        })
    }
}
```

## Running Tests

### Run All Tests

```bash
go test ./internal/testutil/...
```

### Run Specific Test

```bash
go test -run TestBasicValidation ./internal/testutil/...
```

### Run with Verbosity

```bash
go test -v ./internal/testutil/...
```

## Comparison with Other Frameworks

### vs. HTTP-Specific Frameworks

- **Generic**: Works with any type, not just HTTP
- **Flexible**: No predefined response structure
- **Simple**: Fewer dependencies on HTTP types

### vs. Basic Table-Driven Tests

- **More structured**: Clear separation of test data and logic
- **Helper functions**: Convenience functions for common patterns
- **Filtering**: Built-in support for test filtering
- **Extension**: Easy to extend and manipulate tables

## Contributing

When adding new features to the test table framework:

1. Maintain backward compatibility
2. Add comprehensive examples
3. Update this documentation
4. Add tests for new features

## License

Part of the ARMOR project.
