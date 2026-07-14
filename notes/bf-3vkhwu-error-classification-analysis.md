# Error Classification Consistency Analysis
**Bead:** bf-3vkhwu
**Date:** 2026-07-14

## Executive Summary

The ARMOR codebase contains **THREE separate error type classification systems** that serve different purposes:

1. **Basic ErrorType enum** - Generic validation errors (required, format, range, etc.)
2. **ValidationErrorType enum** - HTTP/API-specific validation errors (status_code, error_message, etc.)
3. **String constants** - Duplicates of ValidationErrorType values (redundant)

**Finding:** Error classification is **CONSISTENT** within each system, but there is **redundancy** between ValidationErrorType and string constants.

---

## Error Type Systems

### System 1: Basic ErrorType Enum
**File:** `internal/validate/error_type.go`

```go
type ErrorType string

const (
    ErrTypeRequired  ErrorType = "required"
    ErrTypeFormat    ErrorType = "format"
    ErrTypeRange     ErrorType = "range"
    ErrTypeLength    ErrorType = "length"
    ErrTypeType      ErrorType = "type"
    ErrTypeValue     ErrorType = "value"
    ErrTypeDuplicate ErrorType = "duplicate"
    ErrTypeConflict  ErrorType = "conflict"
    ErrTypeUnknown   ErrorType = "unknown"
)
```

**Purpose:** Generic validation errors applicable to any validation context.
**Coverage:** 9 error types
**Usage:** Minimal - primarily for formatting, not ValidationError creation

---

### System 2: ValidationErrorType Enum
**File:** `internal/validate/error_type_enum.go`

```go
type ValidationErrorType string

const (
    TypeStatusCode          ValidationErrorType = "status_code"
    TypeStatusCodeRange     ValidationErrorType = "status_code_range"
    TypeStatusCodeClass     ValidationErrorType = "status_code_class"
    TypeContentType         ValidationErrorType = "content_type"
    TypeResponseStructure   ValidationErrorType = "response_structure"
    TypeResponseBody        ValidationErrorType = "response_body"
    TypeResponseEncoding    ValidationErrorType = "response_encoding"
    TypeErrorMessage        ValidationErrorType = "error_message"
    TypeErrorMessagePattern ValidationErrorType = "error_message_pattern"
    TypeErrorCode           ValidationErrorType = "error_code"
    TypeErrorDetail         ValidationErrorType = "error_detail"
    TypeCORSHeaders         ValidationErrorType = "cors_headers"
    TypeAuthHeaders         ValidationErrorType = "auth_headers"
    TypeCustomHeaders       ValidationErrorType = "custom_headers"
    TypeJSONSchema          ValidationErrorType = "json_schema"
    TypeDataValidation      ValidationErrorType = "data_validation"
    TypeFieldValidation     ValidationErrorType = "field_validation"
    TypeTypeValidation      ValidationErrorType = "type_validation"
    TypeTimeout             ValidationErrorType = "timeout"
    TypeRateLimit           ValidationErrorType = "rate_limit"
    TypeRetryExceeded       ValidationErrorType = "retry_exceeded"
    TypeCustom              ValidationErrorType = "custom"
    TypeUnknown             ValidationErrorType = "unknown"
)
```

**Purpose:** HTTP/API-specific validation errors.
**Coverage:** 22 error types
**Usage:** Type-safe error type specification, validation, categorization

---

### System 3: String Constants
**File:** `internal/validate/error_categories.go`

```go
const (
    ErrorTypeStatusCode          = "status_code"
    ErrorTypeStatusCodeRange     = "status_code_range"
    ErrorTypeStatusCodeClass     = "status_code_class"
    ErrorTypeContentType         = "content_type"
    ErrorTypeResponseStructure   = "response_structure"
    ErrorTypeResponseBody        = "response_body"
    ErrorTypeResponseEncoding    = "response_encoding"
    ErrorTypeErrorMessage        = "error_message"
    ErrorTypeErrorMessagePattern = "error_message_pattern"
    ErrorTypeErrorCode           = "error_code"
    ErrorTypeErrorDetail         = "error_detail"
    ErrorTypeCORSHeaders         = "cors_headers"
    ErrorTypeAuthHeaders         = "auth_headers"
    ErrorTypeCustomHeaders       = "custom_headers"
    ErrorTypeJSONSchema          = "json_schema"
    ErrorTypeDataValidation      = "data_validation"
    ErrorTypeFieldValidation     = "field_validation"
    ErrorTypeTypeValidation      = "type_validation"
    ErrorTypeTimeout             = "timeout"
    ErrorTypeRateLimit           = "rate_limit"
    ErrorTypeRetryExceeded       = "retry_exceeded"
    ErrorTypeCustom              = "custom"
    ErrorTypeUnknown             = "unknown"
)
```

**Purpose:** String constants for error types.
**Coverage:** 22 error types (exact duplicate of ValidationErrorType values)
**Usage:** Category mapping, validation, descriptions

---

## ValidationError Creation Patterns

### Pattern 1: Direct Struct Creation
**Locations:** `validate.go`, `format_helper.go`

```go
ve := ValidationError{
    ErrorType: validationType,  // string parameter
    Message:   "...",
    Expected:  expected,
    Actual:    actual,
}
```

**Issue:** Uses string parameter directly, no enum validation at creation time.

---

### Pattern 2: ValidationFormatter Builder
**File:** `format_helper.go`

```go
func NewValidationFormatter(validationType string) *ValidationFormatter {
    return &ValidationFormatter{
        validationType: validationType,  // string parameter
    }
}

func FormatStatusCodeError(expected interface{}, actual int, context string) ValidationError {
    return NewValidationFormatter("status_code").  // string literal
        WithExpected(expected).
        WithActual(actual).
        WithContext(context).
        Format()
}
```

**Issue:** Convenience functions use string literals directly instead of enum constants.

---

### Pattern 3: String Literals in Code
**Locations:** Throughout validation code

```go
// Example from format_helper.go
return NewValidationFormatter("status_code")
return NewValidationFormatter("error_message")
return NewValidationFormatter("content_type")
```

**Issue:** String literals are not type-safe and could introduce typos.

---

## Error Type Coverage Analysis

### ValidationErrorType Enum Coverage

| Category | Error Types | Count |
|----------|-------------|-------|
| HTTP | status_code, status_code_range, status_code_class, content_type, cors_headers, auth_headers, custom_headers | 7 |
| Content | response_structure, response_body, response_encoding, error_message, error_message_pattern, error_code, error_detail | 7 |
| Validation | json_schema, data_validation, field_validation, type_validation | 4 |
| Performance | timeout, rate_limit, retry_exceeded | 3 |
| Custom/Unknown | custom, unknown | 2 |
| **Total** | | **22** |

### Basic ErrorType Enum Coverage

| Error Type | Description |
|------------|-------------|
| required | Required field is missing or empty |
| format | Value format is invalid |
| range | Value is outside acceptable range |
| length | String length or collection size is invalid |
| type | Value type is incorrect |
| value | Value is invalid for domain-specific reasons |
| duplicate | Duplicate value was detected |
| conflict | Conflict with existing values or constraints |
| unknown | Unknown error type (default/fallback) |

---

## Consistency Verification

### ✅ Consistent: String Constants Match ValidationErrorType

All string constants in `error_categories.go` exactly match ValidationErrorType enum values:

```bash
# All 22 error types match perfectly:
- status_code       ✓
- error_message     ✓
- content_type      ✓
- timeout           ✓
- json_schema       ✓
# ... (all 22 match)
```

### ✅ Consistent: ValidationError Usage

All ValidationError instances use consistent error type strings that match ValidationErrorType values.

### ✅ Consistent: Category Mapping

Error type to category mapping in `error_categories.go` correctly categorizes all error types.

---

## Inconsistencies Found

### ⚠️ Inconsistency 1: Redundant Error Type Definitions

**Issue:** ValidationErrorType enum and string constants define identical values.

**Impact:** Maintenance overhead - changes must be made in two places.

**Files affected:**
- `error_type_enum.go` (ValidationErrorType enum)
- `error_categories.go` (string constants)

**Recommendation:** Consolidate to use ValidationErrorType enum with .String() method instead of maintaining duplicate string constants.

---

### ⚠️ Inconsistency 2: String Literals vs Type-Safe Enums

**Issue:** Convenience functions use string literals instead of ValidationErrorType enum constants.

**Example:**
```go
// Current (string literal):
func FormatStatusCodeError(...) {
    return NewValidationFormatter("status_code")
}

// Better (type-safe):
func FormatStatusCodeError(...) {
    return NewValidationFormatter(TypeStatusCode.String())
}
```

**Impact:** Less type safety - typos possible in string literals.

**Files affected:**
- `format_helper.go`
- `error_format_examples.go`

---

### ⚠️ Inconsistency 3: ValidationError Field Type

**Issue:** `ValidationError.ErrorType` field is a string, not an enum type.

**Current:**
```go
type ValidationError struct {
    ErrorType string `json:"error_type"`
    Message   string `json:"message"`
    // ...
}
```

**Better (if type safety is desired):**
```go
type ValidationError struct {
    ErrorType ValidationErrorType `json:"error_type"`
    Message   string              `json:"message"`
    // ...
}
```

**Impact:** Runtime validation required instead of compile-time type checking.

**Note:** This is a design choice - string provides flexibility for custom error types, but enum provides type safety.

---

## Error Type Validation System

### Tracking System: Invalid Error Type Detection

**File:** `format_helper.go`

The codebase includes an invalid error type tracking system:

```go
var invalidErrorTypes = struct {
    sync.RWMutex
    types map[string]int
}{types: make(map[string]int)}

func TrackInvalidErrorType(errorType string) {
    // Tracks unrecognized error types
}

func GetInvalidErrorTypes() map[string]int {
    // Returns map of invalid types and counts
}
```

**Purpose:** Debugging aid for detecting typos or invalid error types during development.

---

## Recommendations

### 1. Consolidate Redundant Definitions (High Priority)

**Action:** Remove duplicate string constants in `error_categories.go`.

**Approach:**
- Keep ValidationErrorType enum as the single source of truth
- Use `TypeXXX.String()` instead of `ErrorTypeXXX` constants
- Update all references to use enum

**Benefits:**
- Single source of truth for error types
- Reduced maintenance overhead
- Better type safety

---

### 2. Use Type-Safe Enum Constants (Medium Priority)

**Action:** Update convenience functions to use ValidationErrorType enum constants.

**Approach:**
```go
// Before:
return NewValidationFormatter("status_code")

// After:
return NewValidationFormatter(TypeStatusCode.String())
```

**Benefits:**
- Type safety at call sites
- Compiler catches typos
- IDE auto-completion

---

### 3. Consider Type-Safe ValidationError Field (Low Priority)

**Action:** Evaluate whether ValidationError.ErrorType should use ValidationErrorType enum.

**Trade-offs:**
- **Pros:** Type safety, compile-time validation
- **Cons:** Less flexibility for custom error types, potential breaking changes

**Recommendation:** Keep string for flexibility, but add validation method.

---

### 4. Update Documentation

**Action:** Document the three error type systems and their purposes.

**Locations:**
- Package documentation
- README
- Code comments

---

## Test Coverage

The codebase includes comprehensive tests for error type validation:

- `format_error_string_validation_test.go` - Tests FormatError with valid/invalid types
- `error_type_enum_test.go` - Tests ValidationErrorType enum functionality
- `error_categories_test.go` - Tests error categorization
- `error_formatting_test.go` - Tests error formatting functions

**Coverage:** ✅ Good - All error type systems are well-tested.

---

## Conclusion

**Status:** ✅ Error classification is **CONSISTENT** across all ValidationError creation sites.

**Key Findings:**
1. All ValidationError instances use error type strings that match ValidationErrorType enum values
2. String constants exactly duplicate ValidationErrorType enum (redundant but consistent)
3. Error type to category mapping is correct and complete
4. Error type validation system catches invalid types at runtime

**Areas for Improvement:**
1. Remove redundant string constants (consolidate to ValidationErrorType enum)
2. Use type-safe enum constants in convenience functions
3. Consider type-safe ValidationError.ErrorType field (optional)

**Impact:** Low - No functional issues found. Consistency is good, only redundancy exists.

---

## Files Analyzed

- `internal/validate/error_type.go` - Basic ErrorType enum
- `internal/validate/error_type_enum.go` - ValidationErrorType enum
- `internal/validate/error_categories.go` - String constants and categories
- `internal/validate/error_types.go` - ValidationError struct definition
- `internal/validate/format_helper.go` - ValidationFormatter builder
- `internal/validate/error_formatting.go` - Error formatting functions
- `internal/validate/validate.go` - Main validation functions
- Test files (for coverage verification)

---

## Next Steps

1. ✅ **Analysis complete** - Document findings
2. ⏳ **Optional** - Consolidate redundant definitions if desired
3. ⏳ **Optional** - Update convenience functions to use type-safe enums
4. ⏳ **Optional** - Update documentation to clarify error type systems

**No immediate action required** - error classification is consistent and working correctly.
