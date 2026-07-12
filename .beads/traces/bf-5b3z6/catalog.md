# ValidationError Instantiations Catalog

**Generated:** 2026-07-12  
**Purpose:** Catalog all ValidationError{} instantiations in the ARMOR codebase  
**Bead ID:** bf-30p0u

## Summary

- **Total ValidationError{} instantiations found:** 9
- **With Path field populated:** 3
- **Missing Path field:** 6
- **In production code:** 2
- **In test code:** 7

---

## ValidationError{} Instantiations

### Production Code

#### 1. `internal/yamlutil/validator.go:50`
- **Function:** `LocalValidationError.ToValidationError()`
- **Context:** Converts internal LocalValidationError to standard ValidationError
- **Code:**
  ```go
  return ValidationError{
      FilePath:   ve.FilePath,
      Message:    ve.Message,
      ContextStr: ve.Context,
      Line:       ve.Line,
      Column:     ve.Column,
      Type:       ve.Type,
      Path:       "", // Path is context-specific and populated by caller if needed
  }
  ```
- **Path Status:** ⚠️ **MISSING** - Explicitly set to empty string
- **Notes:** This is the primary conversion method that creates ValidationError from LocalValidationError

#### 2. `internal/yamlutil/errors.go:561`
- **Function:** `NewValidationError()`
- **Context:** Constructor function for ValidationError
- **Code:**
  ```go
  return &ValidationError{
      FilePath:   filePath,
      Message:    message,
      FieldPath:  fieldPath,
      Constraint: constraint,
      ErrorCode:  errorCode,
      Line:       line,
      Column:     column,
      Type:       eType,
      Path:       validPath,
  }
  ```
- **Path Status:** ✅ **PRESENT** - Sets `Path: validPath` (uses fieldPath as fallback)
- **Notes:** This is the recommended constructor that properly initializes the Path field

---

### Test Code

#### 3. `internal/yamlutil/validator_test.go:843`
- **Function:** `TestWarningSummary_SingleWarning()`
- **Context:** Testing warning summary output
- **Code:**
  ```go
  result.Warnings = append(result.Warnings, ValidationError{
      Type:     ErrorTypeStructure,
      Message:  "Test warning",
      FilePath: "test.yaml",
      Line:     1,
      Path:     "",
  })
  ```
- **Path Status:** ⚠️ **MISSING** - Explicitly set to empty string
- **Notes:** Test code - acceptable

#### 4. `internal/yamlutil/validator_test.go:872`
- **Function:** `TestWarningSummary_MultipleWarnings()`
- **Context:** Testing multiple warnings
- **Code:**
  ```go
  ValidationError{Type: ErrorTypeStructure, Message: "Warning 1", FilePath: "test.yaml"},
  ```
- **Path Status:** ⚠️ **MISSING** - No Path field specified
- **Notes:** Test code - acceptable

#### 5. `internal/yamlutil/validator_test.go:873`
- **Function:** `TestWarningSummary_MultipleWarnings()`
- **Context:** Testing multiple warnings
- **Code:**
  ```go
  ValidationError{Type: ErrorTypeStructure, Message: "Warning 2", FilePath: "test.yaml"},
  ```
- **Path Status:** ⚠️ **MISSING** - No Path field specified
- **Notes:** Test code - acceptable

#### 6. `internal/yamlutil/errors_test.go:33`
- **Function:** `TestIsYAMLError()`
- **Context:** Testing error type detection
- **Code:**
  ```go
  err: &ValidationError{FilePath: "test.yaml"},
  ```
- **Path Status:** ⚠️ **MISSING** - No Path field specified
- **Notes:** Test code - acceptable

#### 7. `internal/yamlutil/errors_test.go:81`
- **Function:** `TestGetYAMLErrorType()`
- **Context:** Testing error type extraction
- **Code:**
  ```go
  err: &ValidationError{FilePath: "test.yaml"},
  ```
- **Path Status:** ⚠️ **MISSING** - No Path field specified
- **Notes:** Test code - acceptable

#### 8. `internal/yamlutil/errors_test.go:163`
- **Function:** `TestGetYAMLErrorType()`
- **Context:** Testing error type extraction
- **Code:**
  ```go
  err: &ValidationError{FilePath: "test.yaml"},
  ```
- **Path Status:** ⚠️ **MISSING** - No Path field specified
- **Notes:** Test code - acceptable

---

### Uses Constructor (HAS Path via NewValidationError)

#### 9. `internal/yamlutil/result_types_test.go:424`
- **Function:** `TestValidationResult_IsValid_Basic()`
- **Context:** Testing validation result validity
- **Code:**
  ```go
  *NewValidationError("test.yaml", "required field missing", "server.name", "", ErrCodeRequiredField, 5, 0, "", "server.name"),
  ```
- **Path Status:** ✅ **PRESENT** - Via constructor
- **Notes:** Uses NewValidationError constructor - proper usage

#### 10. `internal/yamlutil/result_types_test.go:463`
- **Function:** `TestValidationResult_IsValid_Consistency()`
- **Context:** Testing validation result consistency
- **Code:**
  ```go
  *NewValidationError("test.yaml", "validation error", "", "", ErrCodeValidationFailed, 0, 0, "", ""),
  ```
- **Path Status:** ✅ **PRESENT** - Via constructor (though empty)
- **Notes:** Uses NewValidationError constructor - proper usage

---

## SchemaValidationError{} Instantiations

**Note:** SchemaValidationError is a separate struct that does not have a `Path` field. It has `FieldPath` instead.

### Production Code

#### 1. `internal/yamlutil/errors.go:592`
- **Function:** `NewSchemaValidationError()`
- **Context:** Constructor for SchemaValidationError
- **Code:**
  ```go
  return &SchemaValidationError{
      FilePath:   filePath,
      SchemaPath: schemaPath,
      FieldPath:  fieldPath,
      Message:    message,
      Expected:   expected,
      Found:      found,
      Line:       line,
      ErrorCode:  errorCode,
  }
  ```
- **Path Status:** N/A - SchemaValidationError does not have Path field
- **Notes:** Uses FieldPath instead

#### 2. `internal/yamlutil/schema.go:134`
- **Function:** `SchemaValidator.Validate()`
- **Context:** Schema validation error for invalid schema
- **Code:**
  ```go
  result.Errors = append(result.Errors, SchemaValidationError{
      Message: fmt.Sprintf("Invalid schema: %v", err),
  })
  ```
- **Path Status:** N/A - Only Message field populated
- **Notes:** Minimal initialization - missing FilePath and other fields

#### 3. `internal/yamlutil/schema.go:169`
- **Function:** `SchemaValidator.ValidateFile()`
- **Context:** File read error during validation
- **Code:**
  ```go
  result.Errors = append(result.Errors, SchemaValidationError{
      Message: fmt.Sprintf("Failed to read file: %v", err),
  })
  ```
- **Path Status:** N/A - Only Message field populated
- **Notes:** Minimal initialization - missing FilePath and other fields

#### 4. `internal/yamlutil/schema.go:179`
- **Function:** `SchemaValidator.ValidateFile()`
- **Context:** YAML parse error
- **Code:**
  ```go
  result.Errors = append(result.Errors, SchemaValidationError{
      Message: fmt.Sprintf("Failed to parse YAML: %v", err),
  })
  ```
- **Path Status:** N/A - Only Message field populated
- **Notes:** Minimal initialization - missing FilePath and other fields

#### 5. `internal/yamlutil/schema.go:212`
- **Function:** `SchemaValidator.validateFields()`
- **Context:** Unknown field warning in strict mode
- **Code:**
  ```go
  result.Warnings = append(result.Warnings, SchemaValidationError{
      FieldPath:      sv.joinPath(pathPrefix, fieldName),
      Message:        "Unknown field in strict mode",
  })
  ```
- **Path Status:** N/A - Has FieldPath, not Path
- **Notes:** Properly populates FieldPath

---

## Test Code SchemaValidationError

#### 6. `internal/yamlutil/errors_test.go:43`
- **Function:** `TestIsYAMLError()`
- **Context:** Testing error type detection
- **Code:**
  ```go
  err: &SchemaValidationError{FilePath: "test.yaml"},
  ```
- **Path Status:** N/A - SchemaValidationError does not have Path field

#### 7. `internal/yamlutil/errors_test.go:91`
- **Function:** `TestGetYAMLErrorType()`
- **Context:** Testing error type extraction
- **Code:**
  ```go
  err: &SchemaValidationError{FilePath: "test.yaml"},
  ```
- **Path Status:** N/A - SchemaValidationError does not have Path field

---

## Analysis

### Critical Issue

The most significant issue is in **`internal/yamlutil/validator.go:50`** in the `ToValidationError()` method:

```go
func (ve LocalValidationError) ToValidationError() ValidationError {
    return ValidationError{
        FilePath:   ve.FilePath,
        Message:    ve.Message,
        ContextStr: ve.Context,
        Line:       ve.Line,
        Column:     ve.Column,
        Type:       ve.Type,
        Path:       "", // ⚠️ ALWAYS EMPTY
    }
}
```

This method is the primary converter for ValidationError creation and **always sets Path to an empty string**, even though:
1. The ValidationError struct has a Path field
2. The NewValidationError constructor properly handles Path
3. Path is meant to provide the dot-notation field path (e.g., "spec.replicas")

### Recommendation

The `ToValidationError()` method should populate the Path field. Since LocalValidationError doesn't have a direct equivalent, the Path could be:
- Set to the FieldPath if available in the calling context
- Populated by the caller after conversion (as the comment suggests)
- The method could accept an optional path parameter

### SchemaValidationError Issues

The SchemaValidationError instantiations in schema.go (lines 134, 169, 179) are minimally populated with only a Message field. They should include at minimum:
- FilePath (known from the function parameter)
- Line number (if available)

---

## Summary Statistics

- **Total ValidationError{} instantiations:** 9
- **With Path field populated:** 3 (1 via constructor with logic, 2 via NewValidationError)
- **Missing Path field:** 6
- **Production code missing Path:** 1 (validator.go:50)
- **Test code missing Path:** 5 (acceptable for tests)

- **Total SchemaValidationError{} instantiations:** 7
- **Minimal initialization (Message only):** 3
- **Proper initialization:** 4 (including constructors and tests)

---

## Recommendations

### High Priority
1. **Fix `validator.go:50`** - The `ToValidationError()` method should populate the Path field or accept it as a parameter

### Medium Priority
2. **Improve SchemaValidationError initialization** in schema.go lines 134, 169, 179 to include FilePath and other known fields

### Low Priority
3. Consider standardizing test ValidationError instantiations to use NewValidationError for consistency

---

## End of Catalog

**Total files analyzed:** 4  
**Total instantiations cataloged:** 16 (9 ValidationError + 7 SchemaValidationError)
