# Direct ValidationError Struct Instantiations - Search Results

## Task: Search for remaining direct ValidationError struct instantiations

### Search Scope
Searched the ARMOR codebase for patterns like `ValidationError{` that don't use the `NewValidationError()` constructor, excluding:
- Test files (`*_test.go`)
- The constructor functions themselves
- Empty slice initializations

---

## Findings

### 1. **internal/yamlutil/schema.go** - NEEDS UPDATING (4 instances)

This file contains 4 direct `SchemaValidationError{}` instantiations that should use the `NewSchemaValidationError()` constructor:

#### Line 134-136
```go
result.Errors = append(result.Errors, SchemaValidationError{
    Message: fmt.Sprintf("Invalid schema: %v", err),
})
```
**Location:** Inside `SchemaValidator.Validate()` method
**Context:** Schema validation failed error
**Should use:** `NewSchemaValidationError(filePath, schemaPath, fieldPath, message, expected, found, line, errorCode)`

#### Line 169-171
```go
result.Errors = append(result.Errors, SchemaValidationError{
    Message: fmt.Sprintf("Failed to read file: %v", err),
})
```
**Location:** Inside `SchemaValidator.ValidateFile()` method
**Context:** File read error during schema validation
**Should use:** `NewSchemaValidationError(filePath, schemaPath, fieldPath, message, expected, found, line, errorCode)`

#### Line 179-181
```go
result.Errors = append(result.Errors, SchemaValidationError{
    Message: fmt.Sprintf("Failed to parse YAML: %v", err),
})
```
**Location:** Inside `SchemaValidator.ValidateFile()` method
**Context:** YAML parsing error during schema validation
**Should use:** `NewSchemaValidationError(filePath, schemaPath, fieldPath, message, expected, found, line, errorCode)`

#### Line 212-215
```go
result.Warnings = append(result.Warnings, SchemaValidationError{
    FieldPath:      sv.joinPath(pathPrefix, fieldName),
    Message:        "Unknown field in strict mode",
})
```
**Location:** Inside `SchemaValidator.validateFields()` method
**Context:** Unknown field warning in strict mode
**Should use:** `NewSchemaValidationError(filePath, schemaPath, fieldPath, message, expected, found, line, errorCode)`

---

### 2. **internal/yamlutil/validator.go** - ACCEPTABLE (1 instance)

#### Line 50-58 (ToValidationError conversion method)
```go
func (ve LocalValidationError) ToValidationError() ValidationError {
    return ValidationError{
        FilePath:   ve.FilePath,
        Message:    ve.Message,
        ContextStr: ve.Context,
        Line:       ve.Line,
        Column:     ve.Column,
        Type:       ve.Type,
        Path:       "", // Path is context-specific and populated by caller if needed
    }
}
```

**Status:** This is ACCEPTABLE as-is
**Reason:** This is a legitimate type conversion method that converts from `LocalValidationError` (internal type) to `ValidationError` (public type). Direct struct instantiation is appropriate for conversion methods between related types.

---

### 3. **internal/yamlutil/errors.go** - OK (constructor functions)

#### Line 561-571 (inside NewValidationError)
This is inside the `NewValidationError()` constructor function itself, so direct struct initialization is expected and correct.

#### Line 590-601 (inside NewSchemaValidationError)
This is inside the `NewSchemaValidationError()` constructor function itself, so direct struct initialization is expected and correct.

---

## Items NOT Counted as Direct Instantiations

### Empty Slice Initializations
These are NOT direct struct instantiations with field values - they're just empty slice declarations:
- `Errors: []ValidationError{}`
- `Warnings: []ValidationError{}`
- `Errors: []SchemaValidationError{}`
- `Warnings: []SchemaValidationError{}`

### Comments
- `//result := Err[Config](ValidationError{Message: "required field missing"})` in result.go (commented out code)

---

## Summary Table

| File | Line | Type | Status | Notes |
|------|------|------|--------|-------|
| internal/yamlutil/schema.go | 134 | SchemaValidationError | **NEEDS FIXING** | Missing constructor call |
| internal/yamlutil/schema.go | 169 | SchemaValidationError | **NEEDS FIXING** | Missing constructor call |
| internal/yamlutil/schema.go | 179 | SchemaValidationError | **NEEDS FIXING** | Missing constructor call |
| internal/yamlutil/schema.go | 212 | SchemaValidationError | **NEEDS FIXING** | Missing constructor call |
| internal/yamlutil/validator.go | 50 | ValidationError | OK | Conversion method |
| internal/yamlutil/errors.go | 561 | ValidationError | OK | Inside constructor |
| internal/yamlutil/errors.go | 590 | SchemaValidationError | OK | Inside constructor |

---

## Next Steps

To fix the 4 instances in `internal/yamlutil/schema.go`:

1. **Line 134**: Replace with appropriate `NewSchemaValidationError()` call
2. **Line 169**: Replace with appropriate `NewSchemaValidationError()` call
3. **Line 179**: Replace with appropriate `NewSchemaValidationError()` call
4. **Line 212**: Replace with appropriate `NewSchemaValidationError()` call

Each replacement should:
- Pass the available context (filePath, message, fieldPath where available)
- Use appropriate error codes from the ErrorCode constants
- Maintain the same error semantics while using the constructor pattern
