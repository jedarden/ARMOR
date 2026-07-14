# Validation Package Types and Relationships

This document provides comprehensive documentation of all types in the validation package and their relationships.

## Overview

The validation package provides a comprehensive system for HTTP API validation with standardized error reporting. The types are organized into several categories:

- **Core Error Types**: `ValidationError` and related constants
- **Error Categorization**: `ErrorCategory`, `ValidationErrorType`
- **Helper Types**: Configuration and result types for specific validation scenarios
- **Builder Types**: Fluent API for constructing validation errors

## Core Error Type: ValidationError

### Purpose

`ValidationError` is the primary data structure for representing validation failures. It provides a standardized, machine-readable format for validation errors across different types of API validations.

### Structure

```go
type ValidationError struct {
    // Required fields
    ErrorType string  // The validation category (e.g., "status_code", "error_message")
    Message   string  // Human-readable description of the validation failure

    // Optional fields for additional context
    Context           string        // Where/when the validation occurred
    Expected          interface{}   // The expected value
    Actual            interface{}   // The actual value received
    FieldName         string        // Specific field where the error was found
    Location          string        // Position information
    RelatedFields     []string      // Related fields for additional context
    PatternDetails    string        // Pattern matching failure information
    RangeInfo         string        // Range boundary information
    ValidationDetails []string      // Additional validation-specific information
    ResponseSnippet   string        // Truncated response excerpt
    Suggestions       []string      // Actionable recommendations
}
```

### Key Relationships

1. **With ErrorType Constants**: Uses constants from `error_categories.go`
2. **With ValidationFormatter**: Built using the builder pattern
3. **With FormatOption**: Configured using functional options
4. **With ValidationErrors**: Can be serialized to maps and JSON

### Usage Example

```go
// Direct construction
err := ValidationError{
    ErrorType: ErrorTypeStatusCode,
    Message:   "Expected status code 200 but got 404",
    Expected:  200,
    Actual:    404,
    Context:   "GET /api/users/123",
}

// Using the builder pattern
err := NewValidationFormatter(ErrorTypeStatusCode).
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users/123").
    Format()
```

## Error Categorization Types

### ErrorCategory

`ErrorCategory` is a high-level categorization type that groups related error types:

```go
type ErrorCategory string

const (
    CategoryHTTP        ErrorCategory = "http"
    CategoryContent     ErrorCategory = "content"
    CategoryValidation  ErrorCategory = "validation"
    CategoryPerformance ErrorCategory = "performance"
    CategorySecurity    ErrorCategory = "security"
    CategoryCustom      ErrorCategory = "custom"
)
```

### ValidationErrorType

`ValidationErrorType` is a strongly-typed enum for validation error categories:

```go
type ValidationErrorType string

const (
    TypeStatusCode          ValidationErrorType = "status_code"
    TypeStatusCodeRange     ValidationErrorType = "status_code_range"
    TypeContentType         ValidationErrorType = "content_type"
    TypeErrorMessage        ValidationErrorType = "error_message"
    // ... and more
)
```

### Type Relationships

```
ValidationErrorType (enum)
    ↓ .String() → string
    ↓ .Category() → ErrorCategory
    ↓ .Description() → string

ErrorCategory (enum)
    ↓ String() → string
    ↓ GetErrorTypesInCategory() → []string

ValidationError (struct)
    ↓ ErrorType field → string (should use ValidationErrorType.String())
    ↓ Validate() → error
    ↓ ToMap() → map[string]interface{}
```

## Helper Types

### ValidationFormatter (Builder Pattern)

`ValidationFormatter` provides a fluent API for constructing `ValidationError` instances:

```go
type ValidationFormatter struct {
    validationType     string
    expected          interface{}
    actual            interface{}
    context           string
    responseSnippet   string
    fieldName         string
    patternDetails    string
    rangeInfo         string
    validationDetails []string
    customSuggestions []string
}
```

**Relationships**:
- **Creates**: `ValidationError` instances
- **Uses**: Error type constants for validation types
- **Configures**: All ValidationError fields through builder methods

**Usage Example**:
```go
err := NewValidationFormatter(ErrorTypeStatusCode).
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users").
    WithSuggestions("Check endpoint URL").
    Format()
```

### FormatOption (Functional Options Pattern)

`FormatOption` provides functional configuration for `FormatCustomValidationError`:

```go
type FormatOption func(*FormatConfig)

type FormatConfig struct {
    Context            string
    ResponseSnippet    string
    FieldName          string
    PatternDetails     string
    RangeInfo          string
    ValidationDetails  []string
    Suggestions        []string
}
```

**Relationships**:
- **Configures**: `FormatCustomValidationError` function
- **Modifies**: `FormatConfig` struct
- **Creates**: Custom `ValidationError` instances

**Usage Example**:
```go
err := FormatCustomValidationError(
    "custom_field",
    "expected_value",
    "actual_value",
    WithContext("custom validation"),
    WithFieldName("field.name"),
    WithSuggestions("Check field value"),
)
```

### CORSConfig (CORS Validation)

`CORSConfig` specifies expected CORS header values for validation:

```go
type CORSConfig struct {
    AllowOrigin     string
    AllowMethods    string
    AllowHeaders    string
    AllowCredentials bool
    ExposeHeaders   string
    MaxAge          string
}
```

**Relationships**:
- **Validates**: HTTP response headers
- **Used by**: `CORSHeadersIsValid` function
- **Related to**: `ValidationError` with `ErrorTypeCORSHeaders`

### ErrorResponseFieldNames (Field Name Configuration)

`ErrorResponseFieldNames` specifies custom field names for error response validation:

```go
type ErrorResponseFieldNames struct {
    PrimaryFieldName   string
    SecondaryFieldName string
}
```

**Relationships**:
- **Configures**: Which fields to check in error responses
- **Used by**: `ErrorResponseStructureIsValid` function
- **Related to**: `ValidationError` with `FieldName` field

### ErrorMessagePattern (Pattern Matching)

`ErrorMessagePattern` defines pattern matching configuration for error messages:

```go
type ErrorMessagePattern struct {
    Pattern         string
    CaseInsensitive bool
    MatchAny        bool
    FieldNames      []string
}
```

**Relationships**:
- **Validates**: Error message content against regex patterns
- **Used by**: `ValidateErrorMessagePatternWithConfig` function
- **Related to**: `ValidationError` with `PatternDetails` field

## Validation Result Types

### StatusCodeValidationResult

`StatusCodeValidationResult` provides detailed information about status code validation:

```go
type StatusCodeValidationResult struct {
    Valid            bool
    ActualCode       int
    ExpectedCodes    []int
    MatchedCode      *int
    MismatchDetails  string
    IsClientError    bool
    IsServerError    bool
    Category         string
}
```

**Relationships**:
- **Returned by**: `ValidateStatusCodeWithDetails` function
- **Provides**: More detailed information than boolean functions
- **Related to**: `ValidationError` with `ErrorTypeStatusCode`

### ErrorMessageValidationResult

`ErrorMessageValidationResult` provides detailed information about error message validation:

```go
type ErrorMessageValidationResult struct {
    Valid                 bool
    Found                 bool
    Message               string
    FieldName             string
    PatternMatched        bool
    MustContainResults    map[string]bool
    MustNotContainResults map[string]bool
    LengthValidation      bool
    Issues                []string
}
```

**Relationships**:
- **Returned by**: `ValidateErrorMessageWithDetails` function
- **Provides**: Comprehensive validation results
- **Related to**: `ValidationError` with `ErrorTypeErrorMessage`

## Range and Pattern Types

### StatusCodeRange

`StatusCodeRange` defines a range of status codes for flexible validation:

```go
type StatusCodeRange struct {
    Min         int
    Max         int
    Description string
}
```

**Relationships**:
- **Used by**: `ValidateStatusCodeRange` function
- **Related to**: `ValidationError` with `ErrorTypeStatusCodeRange` and `RangeInfo` field

**Common Ranges**:
```go
var (
    Range1xx = StatusCodeRange{Min: 100, Max: 199, Description: "Informational"}
    Range2xx = StatusCodeRange{Min: 200, Max: 299, Description: "Success"}
    Range3xx = StatusCodeRange{Min: 300, Max: 399, Description: "Redirection"}
    Range4xx = StatusCodeRange{Min: 400, Max: 499, Description: "Client Error"}
    Range5xx = StatusCodeRange{Min: 500, Max: 599, Description: "Server Error"}
)
```

### EnhancedErrorMessagePattern

`EnhancedErrorMessagePattern` defines advanced pattern matching for error messages:

```go
type EnhancedErrorMessagePattern struct {
    FieldNames      *ErrorResponseFieldNames
    Pattern         string
    CaseInsensitive bool
    MustContain    []string
    MustNotContain []string
    MinLength       int
    MaxLength       int
}
```

**Relationships**:
- **Extends**: Basic `ErrorMessagePattern` with more validation options
- **Used by**: `ValidateErrorMessageWithDetails` function
- **Related to**: `ValidationError` with multiple validation detail fields

## Error Type Collections

### ErrorTypeGroup

`ErrorTypeGroup` represents a collection of related error types for bulk operations:

```go
type ErrorTypeGroup []string
```

**Predefined Groups**:
```go
var (
    HTTPErrorTypes        ErrorTypeGroup  // All HTTP-related types
    ContentErrorTypes     ErrorTypeGroup  // All content-related types
    ValidationErrorTypes  ErrorTypeGroup  // All validation-related types
    PerformanceErrorTypes ErrorTypeGroup  // All performance-related types
    StatusCodeErrorTypes  ErrorTypeGroup  // Status code types
    MessageErrorTypes     ErrorTypeGroup  // Error message types
    HeaderErrorTypes      ErrorTypeGroup  // Header validation types
)
```

**Relationships**:
- **Groups**: Related error type constants
- **Provides**: Convenient access to error type categories
- **Used by**: Error filtering and categorization

## Type Conversion Flow

### String to ValidationErrorType

```go
// Input: string
errorTypeStr := "status_code"

// Convert to ValidationErrorType
errorType := ValidationErrorTypeFromString(errorTypeStr)

// Get string representation
errorType.String() // "status_code"

// Get category
errorType.Category() // CategoryHTTP

// Get description
errorType.Description() // "HTTP status code validation"

// Use in ValidationError
ve := ValidationError{
    ErrorType: errorType.String(),
    Message:   "Validation failed",
}
```

### ValidationError to Map/JSON

```go
// Create ValidationError
ve := ValidationError{
    ErrorType: ErrorTypeStatusCode,
    Message:   "Expected 200 but got 404",
    Expected:  200,
    Actual:    404,
}

// Convert to map
data := ve.ToMap()
// map[string]interface{}{
//     "error_type": "status_code",
//     "message": "Expected 200 but got 404",
//     "expected": 200,
//     "actual": 404,
// }

// Convert to JSON
jsonBytes, _ := json.Marshal(ve)
// {"error_type":"status_code","message":"Expected 200 but got 404","expected":200,"actual":404}
```

### ValidationFormatter to ValidationError

```go
// Create ValidationFormatter
formatter := NewValidationFormatter(ErrorTypeStatusCode).
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users")

// Build ValidationError
ve := formatter.Format()

// Result is a ValidationError with all configured fields
```

## Error Type Validation

### Validation Chain

```go
// 1. Create error type from string
errorType := ValidationErrorTypeFromString("status_code")

// 2. Validate the error type
if err := errorType.Validate(); err != nil {
    log.Printf("Invalid error type: %v", err)
}

// 3. Get category information
category := errorType.Category()

// 4. Use in ValidationError
ve := ValidationError{
    ErrorType: errorType.String(),
    Message:   "Validation failed",
}

// 5. Validate the ValidationError
if err := ve.Validate(); err != nil {
    log.Printf("Invalid ValidationError: %v", err)
}
```

### Type Safety Patterns

```go
// Pattern 1: Use constants directly
ve := ValidationError{
    ErrorType: ErrorTypeStatusCode,  // Type-safe constant
    Message:   "Failed",
}

// Pattern 2: Use enum type
errorType := TypeStatusCode
ve := ValidationError{
    ErrorType: errorType.String(),  // Convert enum to string
    Message:   "Failed",
}

// Pattern 3: Validate user input
errorTypeStr := getUserInput()
if !IsValidErrorType(errorTypeStr) {
    return fmt.Errorf("invalid error type: %s", errorTypeStr)
}
ve := ValidationError{
    ErrorType: errorTypeStr,
    Message:   "Failed",
}
```

## Integration Examples

### Example 1: HTTP Status Code Validation

```go
// Validate status code
result := ValidateStatusCodeWithDetails(resp, 200)

if !result.Valid {
    // Create ValidationError using the builder
    err := NewValidationFormatter(ErrorTypeStatusCode).
        WithExpected(200).
        WithActual(result.ActualCode).
        WithContext("GET /api/users").
        Format()

    // Log the error
    log.Printf("Validation failed: %v", err)

    // Or convert to JSON for API response
    jsonBytes, _ := json.Marshal(err.ToMap())
}
```

### Example 2: Error Message Pattern Validation

```go
// Validate error message pattern
pattern := EnhancedErrorMessagePattern{
    FieldNames: DefaultErrorResponseFieldNames(),
    Pattern:    "invalid.*token",
    MustContain: []string{"expired", "invalid"},
}

result := ValidateErrorMessageWithDetails(responseBody, pattern)

if !result.Valid {
    // Create ValidationError with details
    err := FormatValidationErrorWithDetails(
        ErrorTypeErrorMessagePattern,
        "invalid.*token",
        result.Message,
        "OAuth token validation",
        extractResponseSnippet(responseBody),
        result.FieldName,
        "",
        nil,
        "Pattern did not match",
        "",
        result.Issues,
    )

    log.Printf("Message validation failed: %v", err)
}
```

### Example 3: CORS Header Validation

```go
// Define expected CORS configuration
config := &CORSConfig{
    AllowOrigin:     "https://example.com",
    AllowMethods:    "GET, POST, OPTIONS",
    AllowHeaders:    "Content-Type, Authorization",
    AllowCredentials: true,
}

// Validate CORS headers
if !CORSHeadersIsValid(resp, config) {
    // Create ValidationError
    err := FormatValidationError(
        ErrorTypeCORSHeaders,
        "CORS headers configured",
        "CORS headers missing or invalid",
        "GET /api/data",
        "",
    )

    log.Printf("CORS validation failed: %v", err)
}
```

## Best Practices

### 1. Use Constants for Error Types

❌ **Avoid**: Magic strings
```go
ve := ValidationError{
    ErrorType: "status_code",  // Typo-prone
    Message:   "Failed",
}
```

✅ **Prefer**: Type-safe constants
```go
ve := ValidationError{
    ErrorType: ErrorTypeStatusCode,  // Type-safe
    Message:   "Failed",
}
```

### 2. Validate Error Types

❌ **Avoid**: Assuming input is valid
```go
func CreateError(errorType string) ValidationError {
    return ValidationError{ErrorType: errorType}  // No validation
}
```

✅ **Prefer**: Validate before use
```go
func CreateError(errorType string) (ValidationError, error) {
    if err := ValidateErrorType(errorType); err != nil {
        return ValidationError{}, err
    }
    return ValidationError{ErrorType: errorType}, nil
}
```

### 3. Use Builder Pattern for Complex Errors

❌ **Avoid**: Direct construction with many fields
```go
ve := ValidationError{
    ErrorType: "status_code",
    Message:   "Failed",
    Expected:  200,
    Actual:    404,
    Context:   "GET /api/users",
    FieldName: "",
    Location:  "",
    // ... many more fields
}
```

✅ **Prefer**: Builder pattern for readability
```go
ve := NewValidationFormatter(ErrorTypeStatusCode).
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users").
    Format()
```

### 4. Leverage Type Categories

❌ **Avoid**: Manual category checking
```go
if errorType == "status_code" || errorType == "content_type" || errorType == "cors_headers" {
    // Handle HTTP errors
}
```

✅ **Prefer**: Category-based logic
```go
if IsHTTPErrorType(errorType) {
    // Handle HTTP errors
}
```

## Type Relationship Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                    ValidationError (Core)                    │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ ErrorType (string) ←── ErrorType constants          │   │
│  │ Message   (string)                                   │   │
│  │ Expected  (interface{})                              │   │
│  │ Actual    (interface{})                              │   │
│  │ ... optional fields ...                             │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ uses
                           ↓
┌─────────────────────────────────────────────────────────────┐
│              ErrorType Constants & Enums                     │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ ErrorTypeStatusCode, ErrorTypeErrorMessage, ...     │   │
│  │ (string constants from error_categories.go)          │   │
│  └─────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ TypeStatusCode, TypeErrorMessage, ...               │   │
│  │ (ValidationErrorType enum from error_type_enum.go)  │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ categorizes
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                    ErrorCategory (Enum)                      │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ CategoryHTTP, CategoryContent, CategoryValidation, │   │
│  │ CategoryPerformance, CategorySecurity, CategoryCustom│   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ creates
                           ↓
┌─────────────────────────────────────────────────────────────┐
│              Builder & Helper Types                         │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ ValidationFormatter (builder pattern)                │   │
│  │ FormatOption (functional options)                    │   │
│  │ CORSConfig, ErrorResponseFieldNames, ...             │   │
│  │ StatusCodeValidationResult, ErrorMessagePattern, ...│   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                           │
                           │ organizes
                           ↓
┌─────────────────────────────────────────────────────────────┐
│                 Error Type Collections                       │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ ErrorTypeGroup, ValidationErrorTypes                 │   │
│  │ HTTPErrorTypes, ContentErrorTypes, ...              │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Summary

The validation package provides a comprehensive type system for HTTP API validation:

1. **ValidationError** is the core error representation
2. **ErrorCategory** groups related error types
3. **ValidationErrorType** provides strongly-typed error constants
4. **Helper types** enable flexible validation scenarios
5. **Builder types** provide ergonomic construction APIs
6. **Result types** provide detailed validation information

All types work together to provide type-safe, flexible, and comprehensive validation error reporting for HTTP API testing and validation.
