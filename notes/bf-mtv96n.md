# ErrorType and Category Types Structure Analysis

## Overview

The ARMOR validation package has **two parallel ErrorType systems** that serve different purposes:

1. **ErrorType enum** (`error_type.go`): Generic validation error categories
2. **String-based error types** (`error_categories.go`): HTTP/API-specific error types

This analysis documents both systems, their relationships, and integration points for error formatting helpers.

---

## 1. ErrorType Enum (error_type.go)

### Type Definition

```go
type ErrorType string
```

### Enum Values

| Constant | String Value | Purpose | Severity |
|----------|-------------|---------|----------|
| `ErrTypeRequired` | `"required"` | Required field is missing or empty | High |
| `ErrTypeFormat` | `"format"` | Value format is invalid (email, UUID pattern) | Medium |
| `ErrTypeRange` | `"range"` | Value outside acceptable numeric range | Medium |
| `ErrTypeLength` | `"length"` | String length or collection size invalid | Medium |
| `ErrTypeType` | `"type"` | Value type is incorrect (string vs int) | High |
| `ErrTypeValue` | `"value"` | Value invalid for domain-specific reasons | Low |
| `ErrTypeDuplicate` | `"duplicate"` | Duplicate value detected | High |
| `ErrTypeConflict` | `"conflict"` | Conflict with existing values/constraints | Medium |
| `ErrTypeUnknown` | `"unknown"` | Unknown error type (fallback) | Low |

### Key Methods

- `String() string` - Returns string representation
- `IsValid() bool` - Validates against known constants
- `Description() string` - Returns human-readable description
- `IsRequired()`, `IsFormat()`, etc. - Type checking methods

### ErrorType Collections

```go
// Grouped by logical category
StructuralErrorTypes = {ErrTypeRequired, ErrTypeType, ErrTypeLength}
SemanticErrorTypes = {ErrTypeFormat, ErrTypeRange, ErrTypeValue}
ConstraintErrorTypes = {ErrTypeDuplicate, ErrTypeConflict}
AllErrorTypes = {all 9 types}
```

---

## 2. String-Based Error Types (error_categories.go)

22+ string constants for HTTP/API validation scenarios including:
- HTTP status codes (status_code, status_code_range, status_code_class)
- Response content (content_type, response_structure, response_body, response_encoding)
- Error messages (error_message, error_message_pattern, error_code, error_detail)
- Headers (cors_headers, auth_headers, custom_headers)
- Schema and data (json_schema, data_validation, field_validation, type_validation)
- Performance (timeout, rate_limit, retry_exceeded)
- Custom (custom, unknown)

---

## 3. ErrorCategory System

### 6 Category Values

- `CategoryHTTP` - HTTP protocol-level errors
- `CategoryContent` - Response content errors  
- `CategoryValidation` - Data validation errors
- `CategoryPerformance` - Timing and rate-related errors
- `CategorySecurity` - Authentication and authorization errors
- `CategoryCustom` - Custom application-specific errors

### Dual Mapping System

Both ErrorType enum and string-based types have separate mappings to categories:
- `errorTypeCategoryMap` - String types to categories
- `categoryForErrorTypeEnum` - ErrorType enum to categories

---

## 4. ErrorSeverity System

### 5 Severity Levels

- `SeverityCritical` (4) - System-wide failures
- `SeverityHigh` (3) - Significant impact
- `SeverityMedium` (2) - Partial impact
- `SeverityLow` (1) - Minimal impact
- `SeverityInfo` (0) - Informational

### Dual Severity Mapping

Both ErrorType systems have default severity mappings via:
- `defaultSeverityForErrorType` - String types
- `defaultSeverityForErrorTypeEnum` - ErrorType enum

---

## 5. ValidationError Struct

The `ValidationError.ErrorType` field (string) unifies both systems:
- Can hold ErrorType enum values (e.g., "required", "format")
- Can hold string-based types (e.g., "status_code", "content_type")
- Can hold custom values

---

## 6. Integration Points Summary

### ErrorType Enum Functions (9 functions)
- FormatErrorType(), FormatErrorMessageWithType(), FormatErrorWithSeverity()
- FormatErrorByCategory(), FormatErrorWithCategoryAndSeverity()
- FormatFieldReferenceWithType(), FormatFieldReferenceWithCategory()
- GetCategoryForErrorTypeEnum(), GetSeverityForErrorTypeEnum()

### String-Based Type Functions (7 functions)
- FormatErrorMessage(), GetCategoryForErrorType(), GetDefaultSeverityForErrorType()
- IsValidErrorType(), ValidateErrorType(), GetErrorTypeDescription()
- GetErrorTypesInCategory()

### Cross-System Bridge Functions (4 functions)
- FormatErrorWithTypeDetection(), FormatValidationErrorWithAutoType()
- GetCategoryForErrorType() (handles both), GetDefaultSeverityForErrorType() (handles both)

### ValidationError Functions (6 functions)
- FormatValidationErrorFull(), FormatValidationErrorBrief()
- FormatValidationErrorWithTypeOverride(), FormatValidationErrorWithAutoType()
- FormatErrorList(), FormatErrorListSummary()

---

## 7. Recommended FormatError Implementation Pattern

```go
func FormatError(err ValidationError) string {
    // 1. Try to parse ErrorType enum from string
    errorType := ErrorTypeFromString(err.ErrorType)
    
    // 2. Get category and severity (works for both systems)
    var category ErrorCategory
    var severity ErrorSeverity
    
    if errorType.IsValid() && errorType != ErrTypeUnknown {
        // Use ErrorType enum mappings
        category = GetCategoryForErrorTypeEnum(errorType)
        severity = GetSeverityForErrorTypeEnum(errorType)
    } else {
        // Use string-based error type mappings
        category = GetCategoryForErrorType(err.ErrorType)
        severity = GetDefaultSeverityForErrorType(err.ErrorType)
    }
    
    // 3. Format with full context
    return fmt.Sprintf("[%s] [%s] %s: %s",
        FormatSeverityWithIndicator(severity),
        FormatCategory(category),
        err.ErrorType,
        err.Message)
}
```

### Compatibility Requirements

1. **Backward Compatibility** - Must work with existing string-based error types
2. **Type Safety** - Should prefer ErrorType enum when available  
3. **Graceful Degradation** - Handle unknown error types without panics
4. **Consistent Output** - Format both systems consistently

---

## 8. Existing Compatibility Status

✅ **The existing code already handles both systems correctly:**

- `GetCategoryForErrorType()` checks string map first, then tries ErrorType enum
- `GetDefaultSeverityForErrorType()` checks string map first, then tries ErrorType enum
- `FormatValidationErrorFull()` tries ErrorType enum first, falls back to string lookup
- `FormatErrorWithTypeDetection()` auto-detects enum from string
- `FormatValidationErrorWithAutoType()` auto-detects in ValidationError

The dual lookup strategy ensures backward compatibility while supporting type-safe enums.

---

## 9. Usage Recommendations

- **Use ErrorType enum** for generic validation errors (required, format, range, etc.)
- **Use string-based types** for HTTP/API-specific errors (status_code, content_type, etc.)
- **Use type detection functions** for automatic handling (FormatErrorWithTypeDetection)
- **Use full formatting functions** for comprehensive output (FormatValidationErrorFull)

The implementation provides excellent integration between both systems through automatic type detection and dual lookup strategies.
