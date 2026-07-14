# Validation Error Formatting Data Structure Design

## Overview

This document describes the data structure design for consistent validation error formatting in ARMOR. The structure provides comprehensive, actionable error messages that help developers diagnose and fix validation failures quickly.

## Core Data Structure

### ValidationError (Primary Structure)

The `ValidationError` struct is the primary data structure for all validation errors. It implements the `error` interface and provides rich context about validation failures.

```go
type ValidationError struct {
    // Required Fields
    ValidationType string              // Category of validation (e.g., "status_code", "error_message")
    Expected       interface{}         // What was expected
    Actual         interface{}         // What was actually received

    // Context Fields (Optional but Recommended)
    Context        string              // Additional context about the validation operation
    FieldName      string              // Specific field where error was found (for message validation)
    ResponseSnippet string             // Truncated response excerpt for debugging

    // Detailed Information Fields (Optional)
    PatternDetails     string          // Pattern matching failure information
    RangeInfo          string          // Range boundaries for range validation failures
    ValidationDetails  []string        // Additional validation-specific details

    // Actionable Guidance
    Suggestions        []string        // Suggestions for fixing the issue
}
```

## Field Specifications

### Required Fields

| Field | Type | Description | Example Values |
|-------|------|-------------|----------------|
| `ValidationType` | string | Category of validation being performed | `"status_code"`, `"error_message"`, `"content_type"`, `"status_code_range"` |
| `Expected` | interface{} | The expected value for validation | `200`, `[]int{200, 201}`, `"invalid.*token"` |
| `Actual` | interface{} | The actual value received | `404`, `"access_denied"`, `"text/html"` |

### Optional Context Fields

| Field | Type | Description | When to Use |
|-------|------|-------------|-------------|
| `Context` | string | Additional context about the validation operation | Include endpoint URL, operation type, or test scenario |
| `FieldName` | string | Specific field where error was found | For error message validation (`"error"`, `"message"`, `"detail"`) |
| `ResponseSnippet` | string | Truncated response excerpt for debugging | When response body is relevant to understanding the error |

### Detailed Information Fields (Optional)

| Field | Type | Description | When to Use |
|-------|------|-------------|-------------|
| `PatternDetails` | string | Information about pattern matching failures | When validating against regex patterns |
| `RangeInfo` | string | Range boundaries for range validation failures | When validating against status code ranges (`4xx`, `5xx`) |
| `ValidationDetails` | []string | Additional validation-specific details | For granular information about what was checked and what failed |

### Suggestions Field

| Field | Type | Description | Behavior |
|-------|------|-------------|----------|
| `Suggestions` | []string | Actionable suggestions for fixing the issue | Auto-generated based on validation type if not provided |

## Supporting Data Structures

### StatusCodeValidationResult

Used for detailed status code validation results:

```go
type StatusCodeValidationResult struct {
    Valid             bool      // Whether validation passed
    ActualCode        int       // HTTP status code from response
    ExpectedCodes     []int     // Expected status code(s)
    MatchedCode       *int      // Specific code that matched (if any)
    MismatchDetails   string    // Human-readable mismatch information
    IsClientError     bool      // Whether actual code is 4xx
    IsServerError     bool      // Whether actual code is 5xx
    Category          string    // General category of actual code
}
```

### ErrorMessageValidationResult

Used for comprehensive error message validation:

```go
type ErrorMessageValidationResult struct {
    Valid                 bool              // Whether validation passed
    Found                 bool              // Whether error message field was found
    Message               string            // Actual error message content
    FieldName             string            // Field where message was found
    PatternMatched        bool              // Whether regex pattern matched
    MustContainResults    map[string]bool   // Which required strings were found
    MustNotContainResults map[string]bool   // Which forbidden strings were found
    LengthValidation      bool              // Whether message length was valid
    Issues                []string          // List of validation issues found
}
```

### ErrorCodeMatch

Used for error code detection:

```go
type ErrorCodeMatch struct {
    FieldName      string    // Name of field where code was found
    CodeValue      string    // Actual error code value
    NumericCode    *int      // Code parsed as integer (if applicable)
    MatchedPattern string    // Pattern that matched this code
    Position       string    // Where in response the code was found
}
```

### ValidationFormatter (Builder Pattern)

Provides a fluent API for constructing ValidationError:

```go
type ValidationFormatter struct {
    validationType     string
    expected           interface{}
    actual             interface{}
    context            string
    responseSnippet    string
    fieldName          string
    patternDetails     string
    rangeInfo          string
    validationDetails  []string
    customSuggestions  []string
}
```

## Validation Type Categories

### 1. Status Code Validation (`status_code`)

**Purpose:** Validate HTTP response status codes

**Expected Values:**
- Single code: `200`
- Multiple codes: `[]int{200, 201, 204}`

**Actual Values:**
- Integer status code: `404`

**Common Suggestions:**
- Client errors (4xx): Check request parameters, authentication, resource existence
- Server errors (5xx): Retry logic, service status, support contact

**Example:**
```
status_code validation failed
  Expected: 200 (OK)
  Actual:   404 (Not Found)
  Context:  GET /api/users/123
  Suggestions:
    - Verify the endpoint URL is correct
    - Check if the resource ID or identifier exists
    - Ensure the resource hasn't been deleted or moved
```

### 2. Error Message Validation (`error_message`)

**Purpose:** Validate error message content against patterns

**Expected Values:**
- Regex pattern: `"invalid.*token"`
- Substring: `"not found"`

**Actual Values:**
- String: `"access_denied"`

**Common Suggestions:**
- Review error message for specific details
- Check API documentation for error type
- Verify request parameters match requirements

**Example:**
```
error_message validation failed
  Expected: invalid.*token
  Actual:   access_denied
  Context:  OAuth token validation
  Field:    error
  Response: {"error": "access_denied", "error_description": "User denied authorization"}
  Suggestions:
    - Review the error message for specific details
    - Check API documentation for this error type
    - Verify request parameters match requirements
```

### 3. Status Code Range Validation (`status_code_range`)

**Purpose:** Validate status codes against range patterns

**Expected Values:**
- Range pattern: `"4xx"`, `"5xx"`, `"2xx"`

**Actual Values:**
- Integer status code: `404`

**Common Suggestions:**
- Success (2xx): Update test expectations if this is expected
- Client errors (4xx): Review request parameters, credentials
- Server errors (5xx): Check for temporary issues, retry logic

**Example:**
```
status_code_range validation failed
  Expected: 4xx (400-499)
  Actual:   200
  Context:  error response check
  Range:    400-499 (Client Error)
  Details:
    - Status code 200 is outside range 400-499
    - Range '4xx' represents Client Error
  Suggestions:
    - Review request parameters for errors
    - Check authentication credentials
    - Verify the resource exists and is accessible
```

### 4. Content Type Validation (`content_type`)

**Purpose:** Validate Content-Type headers

**Expected Values:**
- MIME type: `"application/json"`

**Actual Values:**
- MIME type with parameters: `"text/html"`

**Common Suggestions:**
- Verify Content-Type header matches request body format
- Check if charset or boundary parameters are needed
- Ensure body is properly formatted for content type

**Example:**
```
content_type validation failed
  Expected: application/json
  Actual:   text/html
  Context:  API response
  Suggestions:
    - Verify Content-Type header matches request body format
    - Check if charset or boundary parameters are needed
    - Ensure the body is properly formatted for the content type
```

## Error Formatting Flow

### 1. Detection Phase

```
Validation Check → Failure Detected → Collect Context
```

### 2. Context Collection

```
Actual Value + Expected Value + Context Information
```

### 3. Structure Population

```
ValidationError{
    ValidationType: "...",
    Expected: ...,
    Actual: ...,
    Context: "...",
    ...
}
```

### 4. Suggestion Generation

```
Auto-generate based on validation type and values
```

### 5. Error Message Formatting

```
ValidationError.Error() → Formatted multi-line message
```

## Design Decisions

### 1. Interface{} for Expected/Actual

**Decision:** Use `interface{}` for `Expected` and `Actual` fields.

**Rationale:**
- Supports different validation types (int, string, []int)
- Maintains type safety through validation functions
- Allows flexibility for future validation types

**Trade-off:** Requires type assertions when accessing these values.

### 2. Optional vs Required Fields

**Decision:** Only `ValidationType`, `Expected`, and `Actual` are strictly required.

**Rationale:**
- Core validation can function with minimal information
- Additional context enhances debugging but isn't always available
- Allows progressive enhancement of error messages

### 3. Auto-Generated Suggestions

**Decision:** Generate suggestions automatically when not explicitly provided.

**Rationale:**
- Reduces boilerplate in validation code
- Ensures consistent, helpful error messages
- Allows custom suggestions for domain-specific scenarios

**Implementation:** `generateSuggestions()` function maps validation types to appropriate suggestions.

### 4. Response Snippet Truncation

**Decision:** Truncate response snippets to 200 characters.

**Rationale:**
- Prevents excessively long error messages
- Provides enough context for debugging
- Handles large response bodies gracefully

**Implementation:** `extractResponseSnippet()` helper function.

### 5. Separate Result Types

**Decision:** Maintain separate result types (`StatusCodeValidationResult`, `ErrorMessageValidationResult`).

**Rationale:**
- Each validation type has unique metadata
- Allows type-safe access to validation-specific information
- Prevents bloating the main `ValidationError` struct

**Trade-off:** More types to maintain, but clearer API.

## Usage Patterns

### Basic Usage

```go
err := validate.FormatValidationError(
    "status_code",
    200,
    404,
    "GET /api/users",
    `{"error": "User not found"}`,
)
```

### Builder Pattern

```go
err := validate.NewValidationFormatter("error_message").
    WithExpected("invalid.*token").
    WithActual("access_denied").
    WithFieldName("error").
    WithContext("OAuth validation").
    WithResponseSnippet(`{"error": "access_denied"}`).
    Format()
```

### Convenience Functions

```go
// Status code error
err := validate.FormatStatusCodeError(200, 404, "GET /api/users")

// Error message error
err := validate.FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth validation")

// Status code range error
err := validate.FormatStatusCodeRangeError("4xx", 200, "error response check")

// Content type error
err := validate.FormatContentTypeError("application/json", "text/html", "API response")
```

### Custom Formatting with Options

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

## Extensibility

### Adding New Validation Types

1. **Define the validation type string** (e.g., `"custom_validation"`)
2. **Implement suggestion generation** in `generateSuggestions()`
3. **Add convenience function** (optional): `FormatCustomValidationError()`
4. **Document examples** in this design document

### Custom Suggestion Logic

Override auto-generated suggestions by providing custom ones:

```go
err := validate.NewValidationFormatter("status_code").
    WithExpected(200).
    WithActual(404).
    WithSuggestions(
        "Custom suggestion 1",
        "Custom suggestion 2",
    ).
    Format()
```

### Enhanced Validation Patterns

For complex validation requirements, use the detailed validation functions:

- `ValidateStatusCodeWithDetails()` → Returns `StatusCodeValidationResult`
- `ValidateErrorMessageWithDetails()` → Returns `ErrorMessageValidationResult`
- `FindErrorCodesInResponse()` → Returns `[]ErrorCodeMatch`

## Best Practices

### DO

- **Provide context** when possible (endpoint URL, operation type)
- **Include response snippets** for message validation failures
- **Use specific field names** when validating error messages
- **Let suggestions auto-generate** unless domain-specific guidance is needed
- **Use convenience functions** for common validation scenarios

### DON'T

- **Don't leave Context empty** if you have relevant information
- **Don't include full response bodies** in ResponseSnippet (use snippets)
- **Don't hardcode suggestions** for common scenarios (use auto-generation)
- **Don't ignore ValidationErrorDetails** for complex validation failures

## Summary

The validation error formatting data structure provides:

1. **Consistency:** All validation errors use the same base structure
2. **Actionability:** Auto-generated suggestions help fix issues quickly
3. **Flexibility:** Optional fields allow progressive enhancement
4. **Extensibility:** Easy to add new validation types
5. **Debuggability:** Rich context and detailed information

The design prioritizes developer experience by providing clear, actionable error messages that reduce debugging time and improve test reliability.
