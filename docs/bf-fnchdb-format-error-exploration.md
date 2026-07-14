# FormatError and ErrorType Structure Documentation

**Bead ID:** bf-fnchdb  
**Date:** 2026-07-14  
**Purpose:** Explore and document the current FormatError function implementation and ErrorType enum structure

---

## 1. FormatError Function Signature and Usage

### Primary FormatError Function

**Location:** `internal/validate/format_helper.go:490-533`

```go
func FormatError(errorType ErrorType, message string, fieldName string) string
```

**Purpose:** Creates a formatted error message using ErrorType enum for type-safe error classification.

**Parameters:**
- `errorType ErrorType` - The ErrorType enum value (e.g., `ErrTypeRequired`, `ErrTypeFormat`)
- `message string` - The error message content
- `fieldName string` - Optional field name where the error occurred (can be empty string)

**Returns:** A formatted error message string with consistent structure: `"[errorType] fieldName: message"`

**Example Usage:**
```go
msg := validate.FormatError(validate.ErrTypeRequired, "This field is required", "email")
// Returns: "[required] email: This field is required"

msg := validate.FormatError(validate.ErrTypeFormat, "Invalid email format", "")
// Returns: "[format] Invalid email format"

msg := validate.FormatError(validate.ErrTypeRange, "Value out of range", "age")
// Returns: "[range] age: Value out of range"
```

### Backward Compatibility: FormatErrorString

**Location:** `internal/validate/format_helper.go:535-628`

```go
func FormatErrorString(errorType string, message string, fieldName ...string) string
```

**Purpose:** Provides backward compatibility for code that uses string-based error types while maintaining type-safe validation against the ErrorType enum.

**Features:**
- Validates string error types against the ErrorType enum
- Tracks invalid error types for debugging (via `GetInvalidErrorTypes()`)
- Maintains backward compatibility with any string value
- Handles whitespace-only error types

**Error Type Validation:**
- String error types are validated against the ErrorType enum
- If the error type is not recognized, it is tracked (does NOT cause errors)
- Invalid error types can be retrieved using `GetInvalidErrorTypes()`
- Tracking can be reset between tests using `ResetInvalidErrorTypeTracking()`

### Other FormatError Variants

1. **FormatErrorWithType** (Deprecated)
   - Alias for FormatError
   - Maintained for backward compatibility only

2. **FormatErrorMessage**
   - Location: `internal/validate/error_formatting.go:155-193`
   - Lower-level function that FormatError delegates to
   - Creates standardized error messages from components

---

## 2. ErrorType Enum Definition and Variants

**Location:** `internal/validate/error_type.go:39-104`

### ErrorType Type Definition

```go
type ErrorType string
```

**Purpose:** A strongly-typed enum representing common validation error categories that provides type safety and prevents typos.

### ErrorType Constants

The ErrorType enum defines 9 error type constants:

| Constant | String Value | Description |
|----------|--------------|-------------|
| `ErrTypeRequired` | "required" | Required field is missing or empty |
| `ErrTypeFormat` | "format" | Value format is invalid (email, UUID pattern) |
| `ErrTypeRange` | "range" | Value is outside acceptable numeric range |
| `ErrTypeLength` | "length" | String length or collection size is invalid |
| `ErrTypeType` | "type" | Value type is incorrect (string when int expected) |
| `ErrTypeValue` | "value" | Value is invalid for domain-specific reasons |
| `ErrTypeDuplicate` | "duplicate" | Duplicate value was detected |
| `ErrTypeConflict` | "conflict" | Conflict with existing values or constraints |
| `ErrTypeUnknown` | "unknown" | Unknown error type (default/fallback) |

### ErrorType Methods

#### Type Validation Methods
```go
func (et ErrorType) IsValid() bool
func (et ErrorType) Validate() error
func (et ErrorType) OrDefault() ErrorType
```

#### Predicate Methods
```go
func (et ErrorType) IsRequired() bool
func (et ErrorType) IsFormat() bool
func (et ErrorType) IsRange() bool
func (et ErrorType) IsLength() bool
func (et ErrorType) IsType() bool
func (et ErrorType) IsValue() bool
func (et ErrorType) IsDuplicate() bool
func (et ErrorType) IsConflict() bool
func (et ErrorType) IsUnknown() bool
```

#### Display Methods
```go
func (et ErrorType) String() string
func (et ErrorType) Description() string
```

### ErrorType Constructors

```go
func ErrorTypeFromString(s string) ErrorType
func MustParseErrorType(s string) ErrorType
```

**ErrorTypeFromString:**
- Returns ErrTypeUnknown if string doesn't match any known type
- Case-insensitive matching
- Handles invalid input gracefully

**MustParseErrorType:**
- Panics if the string doesn't match any known type
- Intended for initialization code with constant, correct values

### ErrorType Collections

```go
type ErrorTypeList []ErrorType
```

**Predefined Collections:**
- `AllErrorTypes` - All defined error types (9 types)
- `StructuralErrorTypes` - Data structure validation (Required, Type, Length)
- `SemanticErrorTypes` - Data meaning validation (Format, Range, Value)
- `ConstraintErrorTypes` - Constraint violations (Duplicate, Conflict)

---

## 3. Current Call Sites of FormatError

### By Function Type

#### FormatError (ErrorType enum)
No direct production call sites found. This is the new type-safe API that should be used going forward.

#### FormatErrorString (string-based with validation)
**Potential call sites identified:**
- Various validation functions throughout the codebase
- Backward compatibility wrappers
- Test files (not counted for production)

#### FormatErrorMessage (low-level)
**Call sites:**
- `FormatError` function (line 532)
- `FormatErrorString` function (line 627)
- `FormatValidationErrorFull` function (error_formatting.go:511)

### Usage Pattern Analysis

**Current State:**
- Most existing code uses string-based error types
- New code should use `FormatError` with ErrorType enum
- `FormatErrorString` provides validation while maintaining compatibility

**Migration Pattern:**
```go
// Old (string-based, no validation)
msg := FormatErrorMessage("required", "Field is required", "email")

// New (type-safe with ErrorType enum)
msg := FormatError(ErrTypeRequired, "Field is required", "email")

// Backward compatible (string with validation)
msg := FormatErrorString("required", "Field is required", "email")
```

---

## 4. Integration Plan: ErrorType in FormatError

### Current Integration Status

**Already Integrated:** FormatError already uses ErrorType enum as its primary parameter.

**Implementation Details:**
1. FormatError takes `ErrorType` enum as first parameter
2. ErrorType is converted to string internally: `errorTypeStr := errorType.String()`
3. Delegates to FormatErrorMessage for consistent formatting
4. FormatErrorString provides backward compatibility with validation

### Recommended Usage Patterns

#### For New Code
**Use FormatError with ErrorType enum:**
```go
// Import the validate package
import "github.com/jedarden/ARMOR/internal/validate"

// Use ErrorType constants for type safety
err := validate.FormatError(
    validate.ErrTypeRequired,
    "This field is required",
    "email",
)
```

#### For Existing Code (Migration)
**Option 1: Direct Migration (Recommended)**
```go
// Before
msg := FormatErrorMessage("required", "Field is required", "email")

// After
msg := FormatError(ErrTypeRequired, "Field is required", "email")
```

**Option 2: Backward Compatible (Use FormatErrorString)**
```go
// Before
msg := FormatErrorMessage("required", "Field is required", "email")

// After (with validation)
msg := FormatErrorString("required", "Field is required", "email")
```

### Error Type Selection Guide

Choose ErrorType based on the nature of the validation failure:

| Scenario | Use ErrorType |
|----------|--------------|
| Field is missing or empty | `ErrTypeRequired` |
| Email/UUID/date format mismatch | `ErrTypeFormat` |
| Number outside min/max bounds | `ErrTypeRange` |
| String too short/long | `ErrTypeLength` |
| Wrong type (string vs number) | `ErrTypeType` |
| Value invalid for domain | `ErrTypeValue` |
| Unique constraint violated | `ErrTypeDuplicate` |
| Business logic conflict | `ErrTypeConflict` |
| Unknown/error fallback | `ErrTypeUnknown` |

### Integration Benefits

1. **Type Safety:** Compiler catches typos in error type constants
2. **Consistency:** Standardized error types across the codebase
3. **Discoverability:** IDE autocomplete shows all valid error types
4. **Documentation:** ErrorType constants serve as living documentation
5. **Refactoring:** Easy to rename or reorganize error types
6. **Validation:** Automatic tracking of invalid error types in legacy code

### Future Enhancements

**Potential additions to ErrorType system:**

1. **Severity Mapping:** Each ErrorType could have a default severity level
2. **HTTP Status Mapping:** Automatic HTTP status code selection based on ErrorType
3. **Localization:** ErrorType.Description() could support multiple languages
4. **Suggestions:** Common fixes per error type (already partially implemented)
5. **Error Codes:** Machine-readable error codes per ErrorType

### Testing Strategy

**Unit Tests:**
- Test each ErrorType constant for correct string representation
- Test ErrorTypeFromString with valid and invalid inputs
- Test FormatError with all ErrorType variants
- Test FormatErrorString validation tracking

**Integration Tests:**
- Test full validation flows using FormatError
- Test backward compatibility with FormatErrorString
- Test invalid error type tracking and reporting

**Example Test Pattern:**
```go
func TestFormatErrorIntegration(t *testing.T) {
    // Test type-safe FormatError
    msg := FormatError(ErrTypeRequired, "Field required", "email")
    expected := "[required] email: Field required"
    assert.Equal(t, expected, msg)

    // Test FormatErrorString with validation
    msg = FormatErrorString("required", "Field required", "email")
    assert.Equal(t, expected, msg)
    
    // Verify no invalid types tracked
    invalidTypes := GetInvalidErrorTypes()
    assert.Empty(t, invalidTypes)
}
```

---

## 5. Key Files and Locations

| File | Lines | Description |
|------|-------|-------------|
| `internal/validate/error_type.go` | 39-486 | ErrorType enum definition, methods, and helpers |
| `internal/validate/format_helper.go` | 490-637 | FormatError functions and validation logic |
| `internal/validate/error_formatting.go` | 155-193 | FormatErrorMessage low-level formatting |

---

## 6. Summary

**Current State:**
- ErrorType enum is well-defined with 9 comprehensive error categories
- FormatError already integrated with ErrorType as primary parameter
- FormatErrorString provides backward compatibility with validation
- System tracks invalid error types for debugging

**Integration Complete:**
- ErrorType enum is fully integrated into FormatError
- Type-safe error classification is available
- Backward compatibility is maintained
- Validation and tracking features are in place

**Recommended Action:**
Use FormatError with ErrorType enum for all new code. Gradually migrate existing code to use the type-safe API. FormatErrorString can be used during migration or when string-based error types are needed for dynamic scenarios.

**No Further Integration Required:** The ErrorType enum is already fully integrated into FormatError. The system is ready for production use with comprehensive type safety and backward compatibility.
