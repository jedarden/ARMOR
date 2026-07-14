# ErrorType and Category Types Structure Documentation

Bead ID: bf-mtv96n
Date: 2026-07-14

## Overview

This document explores the ErrorType and category types structure in the ARMOR validation package, documenting their current structure, usage patterns, and integration points for error formatting helpers.

## Part 1: ErrorType Enum Values and Purposes

### 1.1 ValidationErrorType Enum (HTTP/API Specific)

Located in: `internal/validate/error_type_enum.go`

The ValidationErrorType enum provides strongly-typed error categories for HTTP/API validation:

| Constant | String Value | Purpose | Category |
|----------|--------------|---------|----------|
| `TypeStatusCode` | "status_code" | HTTP status code validation failures | HTTP |
| `TypeStatusCodeRange` | "status_code_range" | Status code range validation (e.g., 4xx, 5xx) | HTTP |
| `TypeStatusCodeClass` | "status_code_class" | Status code class validation (1xx, 2xx, etc.) | HTTP |
| `TypeContentType` | "content_type" | Content-Type header validation | HTTP |
| `TypeResponseStructure` | "response_structure" | Response structure and format validation | Content |
| `TypeResponseBody` | "response_body" | Response body content validation | Content |
| `TypeResponseEncoding` | "response_encoding" | Response encoding validation | Content |
| `TypeErrorMessage` | "error_message" | Error message content validation | Content |
| `TypeErrorMessagePattern` | "error_message_pattern" | Error message pattern validation | Content |
| `TypeErrorCode` | "error_code" | Error code validation | Content |
| `TypeErrorDetail` | "error_detail" | Error detail validation | Content |
| `TypeCORSHeaders` | "cors_headers" | CORS headers validation | HTTP |
| `TypeAuthHeaders` | "auth_headers" | Authentication headers validation | HTTP |
| `TypeCustomHeaders` | "custom_headers" | Custom headers validation | HTTP |
| `TypeJSONSchema` | "json_schema" | JSON schema validation | Validation |
| `TypeDataValidation` | "data_validation" | Generic data validation | Validation |
| `TypeFieldValidation` | "field_validation" | Field-level validation | Validation |
| `TypeTypeValidation` | "type_validation" | Type validation | Validation |
| `TypeTimeout` | "timeout" | Timeout validation | Performance |
| `TypeRateLimit` | "rate_limit" | Rate limit validation | Performance |
| `TypeRetryExceeded` | "retry_exceeded" | Retry limit validation | Performance |
| `TypeCustom` | "custom" | Custom application-specific errors | Custom |
| `TypeUnknown` | "unknown" | Unknown error type (default/fallback) | Custom |

### 1.2 ErrorType Enum (Generic Validation)

Located in: `internal/validate/error_type.go`

The ErrorType enum provides fundamental validation error categories for any validation context:

| Constant | String Value | Purpose | Severity | Category |
|----------|--------------|---------|----------|----------|
| `ErrTypeRequired` | "required" | Required field is missing or empty | High | Validation |
| `ErrTypeFormat` | "format" | Value format is invalid (e.g., email pattern) | Medium | Validation |
| `ErrTypeRange` | "range" | Value outside acceptable numeric range | Medium | Validation |
| `ErrTypeLength` | "length" | String length or collection size is invalid | Medium | Validation |
| `ErrTypeType` | "type" | Value type is incorrect (e.g., string vs int) | High | Validation |
| `ErrTypeValue` | "value" | Value is invalid for domain-specific reasons | Low | Validation |
| `ErrTypeDuplicate` | "duplicate" | Duplicate value detected | High | Validation |
| `ErrTypeConflict` | "conflict" | Conflict with existing values or constraints | Medium | Validation |
| `ErrTypeUnknown` | "unknown" | Unknown error type (default/fallback) | Low | Custom |

**Key Difference**: 
- `ValidationErrorType` is for HTTP/API-specific validation
- `ErrorType` is for generic validation scenarios

## Part 2: Category Types and Hierarchy

### 2.1 ErrorCategory Enum

Located in: `internal/validate/error_categories.go`

| Category | String Value | Description | Typical Error Types |
|----------|--------------|-------------|---------------------|
| `CategoryHTTP` | "http" | HTTP protocol-level validation | status_code, content_type, cors_headers, auth_headers, custom_headers |
| `CategoryContent` | "content" | Response content validation | response_structure, response_body, response_encoding, error_message, error_code, error_detail |
| `CategoryValidation` | "validation" | Data validation | json_schema, data_validation, field_validation, type_validation |
| `CategoryPerformance` | "performance" | Timing and rate validation | timeout, rate_limit, retry_exceeded |
| `CategorySecurity` | "security" | Authentication and authorization | auth_headers (when used for auth validation) |
| `CategoryCustom` | "custom" | Custom application-specific errors | custom, unknown |

### 2.2 Category Hierarchy

Categories are organized in a flat structure (no inheritance), but they can be grouped logically:

- **Protocol Level**: CategoryHTTP
- **Data Level**: CategoryContent, CategoryValidation
- **Operational Level**: CategoryPerformance, CategorySecurity
- **Application Level**: CategoryCustom

### 2.3 ErrorSeverity Enum

Located in: `internal/validate/error_categorization.go`

| Severity | String Value | Description | Indicator |
|----------|--------------|-------------|-----------|
| `SeverityCritical` | "critical" | Critical error that prevents system functionality | ! |
| `SeverityHigh` | "high" | High-severity error that significantly impacts functionality | ⚠ |
| `SeverityMedium` | "medium" | Medium-severity error that partially impacts functionality | ■ |
| `SeverityLow` | "low" | Low-severity error with minimal impact | ○ |
| `SeverityInfo` | "info" | Informational message that doesn't represent a failure | i |

## Part 3: Integration Points with Error Formatting Functions

### 3.1 Primary Integration Points

#### 3.1.1 FormatError Function (format_helper.go:449)

Current implementation:
```go
func FormatError(errorType string, message string, fieldName ...string) string
```

**Current behavior**: Basic string formatting without ErrorType enum integration

**Integration opportunity**: 
- Accept `ErrorType` enum parameter
- Auto-detect severity from ErrorType
- Add category labeling
- Provide structured output

#### 3.1.2 Error Formatting Functions (error_formatting.go)

| Function | ErrorType Support | Category Support | Severity Support |
|----------|------------------|------------------|------------------|
| `FormatErrorWithType` | ✅ Full | ❌ No | ❌ No |
| `FormatErrorWithSeverity` | ✅ Full | ❌ No | ✅ Full |
| `FormatErrorByCategory` | ✅ Full | ✅ Full | ❌ No |
| `FormatErrorWithCategoryAndSeverity` | ✅ Full | ✅ Full | ✅ Full |
| `FormatErrorMessageWithType` | ✅ Full | ❌ No | ❌ No |
| `FormatErrorWithTypeDetection` | ✅ Auto-detect | ✅ Auto | ✅ Auto |
| `FormatValidationErrorFull` | ✅ Auto-detect | ❌ No | ✅ Auto |
| `FormatValidationErrorWithAutoType` | ✅ Auto-detect | ❌ No | ✅ Auto |

#### 3.1.3 Field Reference Formatting (error_formatting.go)

| Function | ErrorType Support | Category Support |
|----------|------------------|------------------|
| `FormatFieldReferenceWithType` | ✅ Full | ❌ No |
| `FormatFieldReferenceWithCategory` | ✅ Full | ✅ Full |
| `FormatFieldReferenceWithExplicitCategory` | ❌ No | ✅ Full |

### 3.2 Validation Error Structure

Located in: `internal/validate/error_types.go`

```go
type ValidationError struct {
    ErrorType    string      // Required: The error type identifier
    Message      string      // Required: Human-readable description
    Context      string      // Optional: Where/when validation occurred
    Expected     interface{} // Optional: Expected value
    Actual       interface{} // Optional: Actual value received
    FieldName    string      // Optional: Field where error found
    Location     string      // Optional: Position information
    RelatedFields []string   // Optional: Related fields
    PatternDetails string   // Optional: Pattern matching info
    RangeInfo     string     // Optional: Range boundaries
    ValidationDetails []string // Optional: Additional validation info
    ResponseSnippet string    // Optional: Response excerpt
    Suggestions   []string    // Optional: Resolution suggestions
}
```

**Integration point**: ValidationError.ErrorType is currently a string. It can accept:
1. String values from ValidationErrorType enum
2. String values from ErrorType enum
3. Custom string values

## Part 4: Plan for ErrorType Usage in FormatError

### 4.1 Proposed Enhancement to FormatError

**Current signature**:
```go
func FormatError(errorType string, message string, fieldName ...string) string
```

**Proposed enhanced signatures**:

1. **Primary enhancement** (backward compatible):
```go
func FormatError(errorType interface{}, message string, fieldName ...string) string
```

Behavior:
- If `errorType` is `string`: Use current behavior
- If `errorType` is `ErrorType`: Convert to string, add auto-severity, auto-category
- If `errorType` is `ValidationErrorType`: Convert to string, add auto-severity, auto-category

2. **New explicit function** (recommended for clarity):
```go
func FormatErrorWithType(errorType ErrorType, message string, fieldName string, includeSeverity bool) string
```

3. **Full-featured function**:
```go
func FormatErrorComplete(options FormatErrorOptions) string
```

Where:
```go
type FormatErrorOptions struct {
    ErrorType         ErrorType
    Message          string
    FieldName        string
    IncludeSeverity  bool
    IncludeCategory  bool
    CustomSeverity   ErrorSeverity
    CustomCategory   ErrorCategory
}
```

### 4.2 Integration Strategy

**Phase 1: Backward Compatibility**
- Keep existing `FormatError` function unchanged
- Add new `FormatErrorWithType` function
- Add `FormatErrorWithDetection` (auto-detect from string)

**Phase 2: Auto-Enhancement**
- When ErrorType enum is provided, automatically:
  - Detect severity via `GetSeverityForErrorTypeEnum`
  - Detect category via `GetCategoryForErrorTypeEnum`
  - Format severity indicator via `FormatSeverityWithIndicator`
  - Format category label via `FormatCategory`

**Phase 3: Unified Interface**
```go
// Recommended usage patterns:

// 1. Simple string (backward compatible)
msg := FormatError("status_code", "Expected 200", "response")
// Returns: "[status_code] response: Expected 200"

// 2. With ErrorType enum (auto severity and category)
msg := FormatErrorWithType(ErrTypeRequired, "Field is required", "email", true)
// Returns: "[⚠] High [required] email: Field is required"

// 3. With auto-detection from string
msg := FormatErrorWithDetection("required", "Field is required", "email")
// Returns: "[⚠] High [required] email: Field is required"

// 4. Full control
msg := FormatErrorComplete(FormatErrorOptions{
    ErrorType: ErrTypeFormat,
    Message: "Invalid email format",
    FieldName: "email",
    IncludeSeverity: true,
    IncludeCategory: true,
})
// Returns: "[■] Medium [Data Validation] [format] email: Invalid email format"
```

### 4.3 Error Type to Function Mapping

| Error Type | Recommended Function | Output Format |
|------------|---------------------|---------------|
| String only | `FormatError` | `[type] field: message` |
| ErrorType enum | `FormatErrorWithType` | `[severity] [type] field: message` |
| String + detection | `FormatErrorWithDetection` | `[severity] [type] field: message` |
| Full control | `FormatErrorComplete` | `[severity] [category] [type] field: message` |

## Part 5: Compatibility Requirements

### 5.1 Backward Compatibility

**Must preserve**:
1. Existing `FormatError` function signature
2. Existing output format for string-only input
3. Support for all string error types currently in use
4. Optional `fieldName` parameter behavior

**Can extend**:
1. New functions with enhanced capabilities
2. Optional additional parameters
3. New configuration options

**Must avoid**:
1. Breaking changes to existing function signatures
2. Changes to output format for existing usage patterns
3. Removing any current functionality

### 5.2 Error Type Compatibility

The validation package has TWO ErrorType systems:

**ValidationErrorType** (HTTP/API):
- Purpose: HTTP and API validation errors
- Usage: `TypeStatusCode`, `TypeContentType`, etc.
- Source: `error_type_enum.go`
- Mapping: Via `GetCategoryForErrorType` and `GetDefaultSeverityForErrorType`

**ErrorType** (Generic):
- Purpose: Generic validation errors
- Usage: `ErrTypeRequired`, `ErrTypeFormat`, etc.
- Source: `error_type.go`
- Mapping: Via `GetCategoryForErrorTypeEnum` and `GetSeverityForErrorTypeEnum`

**Compatibility requirement**: FormatError functions must handle both systems gracefully.

### 5.3 String to Enum Detection

The package already provides detection functions:

```go
// For ValidationErrorType (HTTP/API)
func ValidationErrorTypeFromString(s string) ValidationErrorType

// For ErrorType (Generic)
func ErrorTypeFromString(s string) ErrorType
```

**Integration requirement**: FormatError should leverage these for auto-detection.

### 5.4 Category and Severity Automatic Detection

**Current mapping functions**:

For ValidationErrorType:
```go
func GetCategoryForErrorType(errorType string) ErrorCategory
func GetDefaultSeverityForErrorType(errorType string) ErrorSeverity
```

For ErrorType:
```go
func GetCategoryForErrorTypeEnum(errorType ErrorType) ErrorCategory
func GetSeverityForErrorTypeEnum(errorType ErrorType) ErrorSeverity
```

**Integration requirement**: Enhanced FormatError functions should use these for automatic enrichment.

### 5.5 Existing Error Categorization Compatibility

The package uses a hierarchical categorization system:

**Error groups by category**:
- `HTTPErrorTypes`: status_code, content_type, cors_headers, auth_headers, custom_headers
- `ContentErrorTypes`: response_structure, response_body, error_message, error_code
- `ValidationErrorTypes`: json_schema, data_validation, field_validation, type_validation
- `PerformanceErrorTypes`: timeout, rate_limit, retry_exceeded

**Compatibility requirement**: Enhanced functions should respect these groupings and support filtering by category.

### 5.6 ValidationFormatter Compatibility

The `ValidationFormatter` builder pattern:
```go
formatter := NewValidationFormatter("status_code").
    WithExpected(200).
    WithActual(404).
    WithContext("GET /api/users").
    Format()
```

**Integration requirement**: Any FormatError enhancements should align with ValidationFormatter patterns and potentially accept ValidationFormatter instances.

## Summary and Recommendations

### Current State
- ✅ Two well-defined ErrorType enums (HTTP-specific and generic)
- ✅ Clear category hierarchy with 6 categories
- ✅ Comprehensive severity system with 5 levels
- ✅ Rich formatting functions with varying degrees of integration
- ❌ FormatError function has minimal ErrorType integration
- ❌ Inconsistent ErrorType usage across formatting functions

### Recommended Actions
1. **Enhance FormatError** to accept ErrorType enum parameter (backward compatible)
2. **Add FormatErrorWithType** for explicit ErrorType enum usage with auto-severity
3. **Add FormatErrorWithDetection** for automatic string-to-enum detection
4. **Add FormatErrorComplete** for full control over all formatting options
5. **Document recommended usage patterns** for different scenarios
6. **Ensure consistency** between ValidationErrorType and ErrorType handling

### Priority Integration Points
1. **High Priority**: FormatError enhancement (primary entry point)
2. **Medium Priority**: Unified formatting interface
3. **Low Priority**: ValidationFormatter integration

### Testing Requirements
1. Test backward compatibility with existing string-only usage
2. Test new ErrorType enum functionality
3. Test auto-detection from strings
4. Test category and severity auto-enrichment
5. Test both ErrorType systems (ValidationErrorType and ErrorType)

## Files Referenced

- `internal/validate/error_type_enum.go` - ValidationErrorType enum
- `internal/validate/error_type.go` - ErrorType enum
- `internal/validate/error_categories.go` - ErrorCategory definitions
- `internal/validate/error_categorization.go` - Severity and categorization logic
- `internal/validate/error_formatting.go` - Enhanced formatting functions
- `internal/validate/format_helper.go` - FormatError and ValidationFormatter
- `internal/validate/error_types.go` - ValidationError struct
