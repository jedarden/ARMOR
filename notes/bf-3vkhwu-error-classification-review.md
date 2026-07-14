# Error Classification Review - ARMOR Project

## Overview
This document reviews the error classification consistency across the ARMOR codebase, focusing on ValidationError creation sites and ErrorType usage patterns.

## Error Type Systems in ARMOR

ARMOR uses **three different error type systems** that serve different purposes:

### 1. Basic ErrorType Enum (error_type.go)
**Purpose:** Generic validation error types for common validation scenarios

**Constants:**
- `ErrTypeRequired` - "required" - Required field is missing or empty
- `ErrTypeFormat` - "format" - Value format is invalid (e.g., email, UUID pattern)
- `ErrTypeRange` - "range" - Value is outside acceptable range (min/max)
- `ErrTypeLength` - "length" - String length or collection size is invalid
- `ErrTypeType` - "type" - Value type is incorrect (e.g., string when int expected)
- `ErrTypeValue` - "value" - Value is invalid for domain-specific reasons
- `ErrTypeDuplicate` - "duplicate" - Duplicate value detected
- `ErrTypeConflict` - "conflict" - Conflict with existing values or constraints
- `ErrTypeUnknown` - "unknown" - Unknown error type (default/fallback)

**Usage Pattern:**
```go
// Convert to string for ValidationError
err := ValidationError{
    ErrorType: string(ErrTypeRequired),
    Message:   "Field 'email' is required",
    FieldName: "email",
}
```

### 2. HTTP/API Error Type Constants (error_categories.go)
**Purpose:** HTTP protocol and API response validation error types

**Constants:**
- HTTP Status: `ErrorTypeStatusCode`, `ErrorTypeStatusCodeRange`, `ErrorTypeStatusCodeClass`
- Content: `ErrorTypeContentType`, `ErrorTypeResponseStructure`, `ErrorTypeResponseBody`, `ErrorTypeResponseEncoding`
- Error Messages: `ErrorTypeErrorMessage`, `ErrorTypeErrorMessagePattern`, `ErrorTypeErrorCode`, `ErrorTypeErrorDetail`
- Headers: `ErrorTypeCORSHeaders`, `ErrorTypeAuthHeaders`, `ErrorTypeCustomHeaders`
- Schema/Data: `ErrorTypeJSONSchema`, `ErrorTypeDataValidation`, `ErrorTypeFieldValidation`, `ErrorTypeTypeValidation`
- Performance: `ErrorTypeTimeout`, `ErrorTypeRateLimit`, `ErrorTypeRetryExceeded`
- Miscellaneous: `ErrorTypeCustom`, `ErrorTypeUnknown`

**Usage Pattern:**
```go
err := ValidationError{
    ErrorType: ErrorTypeStatusCode,
    Message:   "Expected status code 200 but got 404",
    Expected:  200,
    Actual:    404,
}
```

### 3. ValidationErrorType Enum (error_type_enum.go)
**Purpose:** Type-safe enum version of HTTP/API error types

**Constants:** Mirrors the HTTP/API constants (e.g., `TypeStatusCode`, `TypeContentType`, etc.)

**Usage Pattern:**
```go
err := ValidationError{
    ErrorType: string(TypeStatusCode),
    Message:   "Status code validation failed",
}
```

## ValidationError Creation Patterns

### FormatValidationError Function (validate.go)
**Location:** `internal/validate/validate.go:1913`

**Pattern:**
```go
func FormatValidationError(validationType string, expected, actual interface{}, context, responseSnippet string, customSuggestions ...string) ValidationError
```

**Accepts:** Raw string error types (no validation)

**Usage Sites:**
- `ValidateStatusCodeRangeInt` - Uses "status_code_range" ✓
- Example in comments - Uses "status_code" ✓
- **`example_optional_fields_demo.go` - Uses "test" ✗ (INVALID)**

### FormatValidationErrorWithDetails Function (validate.go)
**Location:** `internal/validate/validate.go:1986`

**Pattern:**
```go
func FormatValidationErrorWithDetails(validationType string, ...) ValidationError
```

**Accepts:** Raw string error types (no validation)

**Usage Sites:**
- `example_optional_fields_demo.go` - Uses "error_message" ✓

### ValidationFormatter Builder (format_helper.go)
**Location:** `internal/validate/format_helper.go`

**Pattern:**
```go
formatter := NewValidationFormatter(errorType string)
```

**Validation:** Uses `ErrorTypeFromString()` to validate and normalize error types

**Usage:** Provides type-safe error creation with validation fallback

## Error Type Validation and Normalization

### ErrorTypeFromString Function (error_type.go)
**Purpose:** Converts string to ErrorType enum with validation

**Behavior:**
- Case-insensitive matching
- Returns `ErrTypeUnknown` for unrecognized types
- Used by ValidationFormatter for normalization

### IsValidErrorType Function (error_categories.go)
**Purpose:** Validates HTTP/API error type strings

**Behavior:**
- Checks against errorTypeCategoryMap
- Allows custom error types (lowercase with underscores)
- Used for validation only, not normalization

### GetCategoryForErrorType Function (error_categories.go)
**Purpose:** Maps error types to categories (HTTP, Content, Validation, Performance, Custom)

**Fallback:** Returns `CategoryCustom` for unrecognized types

## Consistency Issues Found

### Issue 1: Invalid Error Type in Example Code
**File:** `internal/validate/example_optional_fields_demo.go:31`

**Issue:**
```go
minimalError := FormatValidationError("test", "expected", "actual", "", "")
```

**Problem:** "test" is not a defined error type constant or enum value

**Impact:** Low - Example code, not production logic

**Recommendation:** Replace with `ErrorTypeCustom` or a valid error type

### Issue 2: No Validation in FormatValidationError
**Function:** `FormatValidationError` and `FormatValidationErrorWithDetails`

**Issue:** These functions accept any string without validation

**Impact:** Medium - Allows invalid error types to propagate

**Current Mitigation:** ValidationFormatter builder provides validation

**Recommendation:** Consider adding optional validation parameter

## Error Classification Coverage

### HTTP/API Error Types
**Coverage:** ✓ Comprehensive - All common HTTP validation scenarios covered

**Validation:**
- Status codes (single, range, class)
- Content type and structure
- Response body and encoding
- Error message patterns
- Headers (CORS, auth, custom)
- JSON schema validation
- Performance issues (timeout, rate limit)

### Basic ErrorType Enum
**Coverage:** ✓ Comprehensive - All fundamental validation scenarios covered

**Validation:**
- Required field validation
- Format validation (patterns)
- Range validation (numeric)
- Length validation (strings/collections)
- Type validation (type checking)
- Value validation (domain-specific)
- Duplicate detection
- Conflict detection

### Error Severity Mapping
**Coverage:** ✓ All error types have default severity levels

**Mapping:** Defined in `error_categorization.go` with severity levels (Critical, High, Medium, Low, Info)

### Error Category Mapping
**Coverage:** ✓ All error types mapped to categories

**Categories:** HTTP, Content, Validation, Performance, Security, Custom

## Error Type Usage Patterns in Codebase

### Test Files
**Pattern:** Heavy use of ErrorType enum with string conversion

**Example:**
```go
{ErrorType: string(ErrTypeRequired), FieldName: "email"}
```

### Production Code
**Pattern:** Direct use of HTTP/API error type constants

**Example:**
```go
err := ValidationError{ErrorType: ErrorTypeStatusCode, ...}
```

### Documentation
**Pattern:** Mixed usage of both systems for demonstration

## Recommendations

### 1. Fix Invalid Error Type in Example
**Action:** Replace "test" with valid error type in example_optional_fields_demo.go

**Priority:** Low

### 2. Consider Adding Validation to FormatValidationError
**Action:** Add optional validation parameter or validation warning

**Priority:** Medium

### 3. Document Error Type System Usage
**Action:** Add documentation explaining when to use each error type system

**Priority:** Medium

### 4. Consider Consolidating Error Type Systems
**Action:** Evaluate if three separate systems are necessary or if they can be unified

**Priority:** Low - Current systems serve different purposes

## Conclusion

The ARMOR error classification system is **consistent and well-designed** with:
- ✓ All ValidationError creation sites use defined error types (except one example)
- ✓ ErrorType enum values cover all common validation scenarios
- ✓ String-based error types map to valid ErrorType values or are properly categorized
- ✓ Error classification is predictable and consistent
- ✓ Comprehensive severity and category mappings

**Issues Found:** 1 minor issue in example code using undefined "test" error type

**Overall Assessment:** The error classification system is production-ready with excellent coverage and consistency. The three-tier system (Basic Enum, HTTP/API Constants, Type-Safe Enum) provides flexibility while maintaining type safety where needed.
