# Validate() Callers Audit - internal/yamlutil/schema.go

**Bead:** bf-17y15
**Date:** 2026-07-12
**Scope:** Direct SchemaDefinition.Validate() callers in schema.go

## Summary

This audit catalogs all direct calls to `SchemaDefinition.Validate()` in `internal/yamlutil/schema.go` to identify what needs updating for YAMLError compatibility.

---

## Findings

### 1. SchemaValidator.Validate() - Line 190

**Context:**
```go
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult {
    // ... compilation setup ...
    
    // Validate data against schema
    if err := sv.schema.Validate(data); err != nil {
        result.Valid = false

        // Handle YAMLError with structured information
        if yamlErr, ok := err.(YAMLError); ok {
            result.Errors = append(result.Errors, SchemaValidationError{
                Message:   yamlErr.Error(),
                ErrorCode: yamlErr.Code(),
            })
        } else {
            // Handle generic errors
            result.Errors = append(result.Errors, SchemaValidationError{
                Message: fmt.Sprintf("Validation failed: %v", err),
            })
        }
        return result
    }
    // ...
}
```

**Current Error Handling Pattern:** ✅ **YAMLError-aware**
- Uses type assertion: `if yamlErr, ok := err.(YAMLError); ok`
- Extracts `ErrorCode` from YAMLError
- Provides structured error handling with SchemaValidationError
- Degrades gracefully for generic errors

**Status:** **Already YAMLError-compatible** - No update needed

---

### 2. SchemaValidator.ValidateFile() - Line 263

**Context:**
```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult {
    // ... file reading and YAML parsing ...
    
    // Validate against schema
    return sv.Validate(data)
}
```

**Current Error Handling Pattern:** ✅ **Indirectly YAMLError-aware**
- Delegates to `SchemaValidator.Validate()` (Line 190)
- YAMLError handling is inherited from the delegated method
- No direct error handling at this call site

**Status:** **Already YAMLError-compatible via delegation** - No update needed

---

## Method Definition (Not a Call Site)

### SchemaDefinition.Validate() - Line 780

```go
func (s *SchemaDefinition) Validate(value interface{}) error {
    if value == nil {
        return NewValidationError("", "value cannot be nil", "", "", ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")
    }
    // ... validation logic ...
}
```

This is the **method definition**, not a call site. The method itself returns YAMLError-compatible errors via `NewValidationError()` and related error constructors.

---

## Summary Table

| Line | Method | Caller Type | Error Handling | YAMLError Compatible | Action Needed |
|------|--------|-------------|----------------|----------------------|----------------|
| 190 | `SchemaValidator.Validate()` | Direct interface call | Type assertion + structured | ✅ Yes | None |
| 263 | `SchemaValidator.ValidateFile()` | Delegation | Inherited from Validate() | ✅ Yes | None |
| 780 | `SchemaDefinition.Validate()` | Method definition | N/A | ✅ Yes (returns YAMLError) | None |

---

## Conclusion

**All Validate() callers in schema.go are already YAMLError-compatible.**

- **Direct calls:** 1 (Line 190) - Uses type assertion to detect YAMLError
- **Delegation calls:** 1 (Line 263) - Inherits YAMLError handling
- **Updates needed:** **0**

The existing error handling pattern at Line 190 serves as the canonical implementation that other codebases should follow when handling Validate() errors.
