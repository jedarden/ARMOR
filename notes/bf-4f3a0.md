# Validate() Callers in schema.go

**Task:** Identify all direct Validate() callers in internal/yamlutil/schema.go that need YAMLError handling
**Date:** 2026-07-12
**Bead:** bf-4f3a0

## Summary

Found **1 direct Validate() interface caller** in schema.go that requires YAMLError handling. The existing code already implements proper YAMLError type assertion patterns.

## Direct Validate() Interface Callers

### 1. SchemaValidator.Validate() method
**Location:** Line 208
**Code:** `if err := sv.schema.Validate(data); err != nil {`
**Context:** Validates data against the schema after compilation

**Error Handling Pattern (Lines 212-222):**
```go
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
```

**Status:** ✅ **ALREADY HANDLES YAMLError** - Uses type assertion to extract ErrorCode

## Indirect Validate() Callers

### 2. SchemaValidator.ValidateFile() method
**Location:** Line 281
**Code:** `return sv.Validate(data)`
**Context:** Delegates to SchemaValidator.Validate() after parsing YAML file

**Error Handling:** Inherits structured error handling from SchemaValidator.Validate()

**Status:** ✅ **NO ACTION NEEDED** - Indirect caller that delegates to method #1

## Validate() Implementation

### SchemaDefinition.Validate() method
**Location:** Line 813
**Type:** Schema interface implementation (not a caller)
**Context:** This is the actual Validate() method implementation for SchemaDefinition

## Other Related Methods

### compileSchema() method
**Location:** Lines 285-297
**Calls:** `schemaDef.Compile()` (not Validate())
**Error Handling:** Lines 289-294 - Already implements YAMLError type assertion
**Status:** ✅ **ALREADY HANDLES YAMLError**

### LoadSchema() function
**Location:** Lines 629-691
**Calls:** `schemaDef.Compile()` (not Validate())
**Error Handling:** Lines 677-687 - Already implements YAMLError type assertion
**Status:** ✅ **ALREADY HANDLES YAMLError**

## Error Handling Pattern Template

All YAMLError handling in schema.go follows this consistent pattern:

```go
if yamlErr, ok := err.(YAMLError); ok {
    // Structured error with ErrorCode
    return/handle yamlErr
} else {
    // Generic error
    return/handle fmt.Errorf("...: %w", err)
}
```

## Conclusion

**All Validate() callers in schema.go already implement proper YAMLError handling.**

The codebase consistently uses type assertion patterns to extract YAMLError error codes when available, falling back to generic error handling for non-YAMLError errors.

## Callers Needing Updates

**None** - All Validate() callers already handle YAMLError correctly.
