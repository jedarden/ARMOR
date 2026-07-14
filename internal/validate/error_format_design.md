# Validation Error Formatting Data Structure Design

## Overview

This document defines the data structure for consistent validation error formatting in the ARMOR validation library. The design provides a structured approach to representing validation failures with rich context, actionable suggestions, and clear expected vs actual value comparisons.

## Core Design Principles

1. **Consistency**: All validation errors follow the same structural pattern
2. **Actionability**: Errors include suggestions for resolution
3. **Context**: Errors include sufficient context to understand the validation failure
4. **Flexibility**: The structure supports various validation types while maintaining consistency
5. **Composability**: Builders and helpers make error creation ergonomic

## Primary Data Structure: ValidationError

The `ValidationError` struct is the core type representing a validation failure.

```go
type ValidationError struct {
    // REQUIRED FIELDS
    
    // ValidationType indicates the category of validation being performed.
    // Common values: "status_code", "error_message", "content_type", "status_code_range"
    // This field is REQUIRED and must be non-empty.
    ValidationType string
    
    // Expected represents the value or condition that was expected.
    // Type can be: int, []int, string, or other comparable types
    // This field is REQUIRED.
    Expected interface{}
    
    // Actual represents the value or condition that was actually received.
    // Type should match Expected for meaningful comparison
    // This field is REQUIRED.
    Actual interface{}
    
    // OPTIONAL FIELDS - Contextual Information
    
    // Context provides additional information about the validation operation.
    // Examples: "GET /api/users/123", "OAuth token validation", "POST /api/orders"
    // This field is OPTIONAL but recommended for debugging.
    Context string
    
    // FieldName specifies the field where the error was found.
    // Primarily used for error message validation (e.g., "error", "message", "detail")
    // This field is OPTIONAL.
    FieldName string
    
    // ResponseSnippet contains a truncated excerpt from the response body.
    // Limited to ~200 characters for readability
    // This field is OPTIONAL but recommended for debugging.
    ResponseSnippet string
    
    // OPTIONAL FIELDS - Validation-Specific Details
    
    // PatternDetails contains information about pattern matching failures.
    // Used when validating against regex patterns (e.g., "regex pattern 'invalid.*token' did not match")
    // This field is OPTIONAL.
    PatternDetails string
    
    // RangeInfo specifies range boundaries for range validation failures.
    // Format: "400-499 (Client Error)" or similar
    // This field is OPTIONAL.
    RangeInfo string
    
    // ValidationDetails contains additional validation-specific information.
    // Each string provides a piece of granular detail about what was checked and what failed.
    // This field is OPTIONAL.
    ValidationDetails []string
    
    // OPTIONAL FIELDS - Resolution Guidance
    
    // Suggestions provides actionable recommendations for fixing the validation failure.
    // If nil or empty, suggestions are auto-generated based on validation type and values.
    // This field is OPTIONAL (auto-generated when not provided).
    Suggestions []string
}
```

### Field Classification Summary

| Field | Required | Purpose |
|-------|----------|---------|
| `ValidationType` | **Yes** | Identifies the validation category |
| `Expected` | **Yes** | What was expected |
| `Actual` | **Yes** | What was actually received |
| `Context` | No | Additional operation context |
| `FieldName` | No | Specific field name (message validation) |
| `ResponseSnippet` | No | Debugging excerpt from response |
| `PatternDetails` | No | Pattern matching information |
| `RangeInfo` | No | Range boundary information |
| `ValidationDetails` | No | Additional validation-specific details |
| `Suggestions` | No | Resolution recommendations (auto-generated if empty) |

## Supporting Types

### StatusCodeValidationResult

Detailed result for status code validation operations.

```go
type StatusCodeValidationResult struct {
    // Valid indicates whether the validation passed
    Valid bool
    
    // ActualCode is the HTTP status code from the response
    ActualCode int
    
    // ExpectedCodes contains the expected status code(s)
    ExpectedCodes []int
    
    // MatchedCode is the specific code that matched (if any)
    MatchedCode *int
    
    // MismatchDetails contains human-readable mismatch information
    MismatchDetails string
    
    // IsClientError indicates if actual code is 4xx
    IsClientError bool
    
    // IsServerError indicates if actual code is 5xx
    IsServerError bool
    
    // Category describes the general category: "success", "client_error", "server_error", "redirection", "other"
    Category string
}
```

### ErrorMessageValidationResult

Detailed result for error message content validation.

```go
type ErrorMessageValidationResult struct {
    // Valid indicates whether the error message validation passed
    Valid bool
    
    // Found indicates whether an error message field was found
    Found bool
    
    // Message is the actual error message content
    Message string
    
    // FieldName is the field where the message was found
    FieldName string
    
    // PatternMatched indicates whether the regex pattern matched
    PatternMatched bool
    
    // MustContainResults shows which required strings were found
    MustContainResults map[string]bool
    
    // MustNotContainResults shows which forbidden strings were found
    MustNotContainResults map[string]bool
    
    // LengthValidation indicates whether message length was valid
    LengthValidation bool
    
    // Issues contains a list of validation issues
    Issues []string
}
```

### ErrorCodeMatch

Represents a found error code in a response.

```go
type ErrorCodeMatch struct {
    // FieldName is the field where the error code was found
    FieldName string
    
    // CodeValue is the error code value (as string for flexibility)
    CodeValue string
    
    // NumericCode is the code parsed as integer (if applicable)
    NumericCode *int
    
    // MatchedPattern is the pattern that matched
    MatchedPattern string
    
    // Position describes where in the response the code was found
    Position string
}
```

## Builder Pattern: ValidationFormatter

The `ValidationFormatter` provides a fluent builder API for constructing ValidationError instances.

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

### Builder Methods

- `NewValidationFormatter(validationType string) *ValidationFormatter`
- `WithExpected(expected interface{}) *ValidationFormatter`
- `WithActual(actual interface{}) *ValidationFormatter`
- `WithContext(context string) *ValidationFormatter`
- `WithResponseSnippet(snippet string) *ValidationFormatter`
- `WithFieldName(fieldName string) *ValidationFormatter`
- `WithPatternDetails(details string) *ValidationFormatter`
- `WithRangeInfo(info string) *ValidationFormatter`
- `WithValidationDetails(details ...string) *ValidationFormatter`
- `WithSuggestions(suggestions ...string) *ValidationFormatter`
- `Format() ValidationError`

### Example Usage

```go
err := NewValidationFormatter("status_code").
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users/123").
    WithResponseSnippet(`{"error": "User not found"}`).
    Format()
```

## Convenience Functions

For common validation scenarios, dedicated functions provide a more ergonomic API:

### Status Code Validation

```go
func FormatStatusCodeError(expected interface{}, actual int, context string) ValidationError
```

### Error Message Validation

```go
func FormatErrorMessageError(expectedPattern, actualMessage, fieldName, context string) ValidationError
```

### Status Code Range Validation

```go
func FormatStatusCodeRangeError(pattern string, actual int, context string) ValidationError
```

### Content-Type Validation

```go
func FormatContentTypeError(expected, actual, context string) ValidationError
```

### Custom Validation

```go
func FormatCustomValidationError(
    validationType string,
    expected, actual interface{},
    options ...FormatOption,
) ValidationError
```

## Format Options Pattern

For maximum flexibility, the `FormatOption` pattern allows optional configuration:

```go
type FormatOption func(*FormatConfig)

// Available options:
func WithContext(context string) FormatOption
func WithResponseSnippet(snippet string) FormatOption
func WithFieldName(fieldName string) FormatOption
func WithPatternDetails(details string) FormatOption
func WithRangeInfo(info string) FormatOption
func WithValidationDetails(details ...string) FormatOption
func WithSuggestions(suggestions ...string) FormatOption
```

### Example Usage

```go
err := FormatCustomValidationError(
    "custom_field",
    "required_value",
    "actual_value",
    WithContext("custom validation"),
    WithResponseSnippet(`{"field": "actual_value"}`),
    WithSuggestions("Check field value", "Verify configuration"),
)
```

## Error Formatting Output

The `ValidationError.Error()` method formats the error as a structured multi-line message:

```
{validation_type} validation failed
  Expected: {expected_value}
  Actual:   {actual_value}
  Context:  {context}           // if provided
  Field:    {field_name}        // if provided
  Pattern:  {pattern_details}   // if provided
  Range:    {range_info}        // if provided
  Response: {response_snippet}  // if provided
  Details:                      // if provided
    - {detail_1}
    - {detail_2}
  Suggestions:
    - {suggestion_1}
    - {suggestion_2}
```

## Design Decisions

### 1. Interface{} for Expected/Actual Values

**Decision**: Use `interface{}` for Expected and Actual fields.

**Rationale**: 
- Flexibility to handle different validation types (int codes, string patterns, ranges)
- Type safety is maintained through validation functions
- Allows for single struct definition across all validation types

**Trade-off**: Requires type assertions in consumer code, but this is acceptable for a validation library.

### 2. Auto-Generated Suggestions

**Decision**: Suggestions are auto-generated when not explicitly provided.

**Rationale**:
- Reduces boilerplate in common validation scenarios
- Ensures consistent quality of suggestions
- Allows override for domain-specific cases

**Implementation**: The `generateSuggestions()` function analyzes validation type and values to produce relevant suggestions.

### 3. Optional Context Fields

**Decision**: Context, ResponseSnippet, and field-specific details are optional.

**Rationale**:
- Not all validation scenarios have access to full response data
- Allows for lightweight error creation when context is unavailable
- Maintains consistency across all error types

### 4. Structured Error Format

**Decision**: Multi-line structured format with clear sections.

**Rationale**:
- Readable in logs and console output
- Machine-parsable if needed
- Follows common error message patterns

### 5. Builder Pattern

**Decision**: Provide fluent builder API alongside direct struct construction.

**Rationale**:
- Ergonomic for common cases
- Flexible for advanced scenarios
- Allows progressive construction of complex errors

## Validation Type Categories

The following validation types are currently supported:

| Type | Description | Expected Value Type | Actual Value Type |
|------|-------------|---------------------|-------------------|
| `status_code` | HTTP status code validation | int or []int | int |
| `error_message` | Error message content validation | string (pattern) | string (message) |
| `content_type` | Content-Type header validation | string | string |
| `status_code_range` | Status code range validation | string (pattern) | int |

## Extensibility

### Adding New Validation Types

To add a new validation type:

1. Define the type name (e.g., `"header_validation"`)
2. Add type-specific suggestion generation in `generateSuggestions()`
3. Optionally create a convenience function `Format{Type}Error()`
4. Update documentation

### Custom Suggestions

For domain-specific validation scenarios:

```go
err := NewValidationFormatter("custom_validation").
    WithExpected(expected).
    WithActual(actual).
    WithSuggestions(
        "Check the configuration file",
        "Verify the service is running",
        "Review the logs for details",
    ).
    Format()
```

## Summary

The validation error formatting data structure provides:

1. **Consistency**: Single `ValidationError` type for all validation failures
2. **Rich Context**: Optional fields provide debugging information
3. **Actionability**: Auto-generated or custom suggestions guide resolution
4. **Ergonomics**: Builder pattern and convenience functions simplify usage
5. **Extensibility**: Design supports new validation types and custom scenarios

The structure balances flexibility and consistency, making it suitable for a wide range of validation scenarios while maintaining a clear, predictable error format.