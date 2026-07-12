# Validate() Call Sites Catalog (Go ARMOR)

**Generated:** 2026-07-12
**Bead:** bf-52zl8
**Task:** Catalog all Validate() call sites in ARMOR Go codebase
**Total distinct validate() methods:** 8
**Production call sites:** 2

---

## Executive Summary

The ARMOR Go codebase has **8 distinct `validate()` method signatures** across different types and interfaces:

1. **ValidatedSchema interface** (`Validate() YAMLError`) - Validates schema definition itself
2. **Schema interface** (`Validate(value interface{}) error`) - Validates data against schema
3. **SchemaValidator** (`Validate(data interface{}) SchemaValidationResult`) - Validates with detailed result
4. **Constraint interface** (`Validate(value interface{}) *ConstraintError`) - Validates constraints
5. **StringConstraintImpl** - String validation implementation
6. **NumberConstraintImpl** - Number validation implementation
7. **ArrayConstraintImpl** - Array validation implementation
8. **TypeConstraintImpl** - Type validation implementation

**Key Finding:** All production `validate()` call sites already have proper error handling. No systematic updates required.

---

## Method Signature Catalog

### 1. ValidatedSchema Interface

```go
// internal/yamlutil/schema_interfaces.go:31-34
type ValidatedSchema interface {
    // Validate checks if the schema definition itself is valid.
    // Returns a YAMLError if the schema has invalid configuration.
    Validate() YAMLError
    // ... other methods
}
```

**Definition:** `internal/yamlutil/schema_interfaces.go:31-34`

**Production Call Sites:** None

**Test Call Sites:**
- `internal/yamlutil/schema_validation_test.go:94` - Direct call in test table
- `internal/yamlutil/schema_validation_test.go:147` - Direct call in contract test

**Status:** ✅ **TEST-ONLY** - Interface definition only, no production usage

**Error Type:** Returns `YAMLError` (structured error with error codes)

---

### 2. Schema Interface

```go
// internal/yamlutil/schema.go:38-52
type Schema interface {
    // Validate validates the given value against the schema rules.
    //
    // The value parameter can be of any type:
    //   - Primitive types (string, int, float64, bool, etc.)
    //   - Struct types for typed validation
    //   - map[string]interface{} for dynamic YAML/JSON data
    //   - []interface{} for array validation
    //   - Any other custom type
    //
    // Returns nil if the value conforms to the schema rules.
    // Returns an error if the value violates any schema constraints.
    Validate(value interface{}) error
}
```

**Definition:** `internal/yamlutil/schema.go:38-52`

**Implementation:** `SchemaDefinition.Validate(value interface{}) error` at `internal/yamlutil/schema.go:767`

**Production Call Site:** `internal/yamlutil/schema.go:180`

```go
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
```

**Call Type:** Direct call with type assertion
**Error Handling:** ✅ **EXCELLENT** - Type-asserts to `YAMLError` for structured error information, falls back to generic error handling
**Update Priority:** **LOW** - Already has comprehensive error handling

**Test Call Sites:** None directly (tests use SchemaValidator.Validate() instead)

---

### 3. SchemaValidator.Validate()

```go
// internal/yamlutil/schema.go:157-206
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult
```

**Definition:** `internal/yamlutil/schema.go:157-206`

**Return Type:** `SchemaValidationResult` (struct with Valid bool, Errors []SchemaValidationError, Warnings []SchemaValidationError, etc.)

**Production Call Site:** `internal/yamlutil/schema.go:253`

```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult {
    // ... read and parse file ...
    // Validate against schema
    return sv.Validate(data)
}
```

**Call Type:** Direct delegation from ValidateFile
**Error Handling:** N/A - Returns custom struct, not error type
**Update Priority:** **N/A** - Custom return type

**Test Call Sites:**
- `internal/yamlutil/schema_validation_test.go:224` - Direct call in test table
- `internal/yamlutil/schema_validation_test.go:310` - Direct call in test table

---

### 4. Constraint Interface

```go
// internal/yamlutil/schema_interfaces.go:86-96
type Constraint interface {
    // Validate checks if a value satisfies this constraint.
    // Returns nil if the constraint is satisfied, or a ConstraintError if violated.
    Validate(value interface{}) *ConstraintError

    Description() string
    ConstraintType() string
}
```

**Definition:** `internal/yamlutil/schema_interfaces.go:86-96`

**Production Call Sites:** None (constraint implementations are defined but not called in production code)

**Implementations:**
1. **StringConstraintImpl** - `internal/yamlutil/schema_interfaces.go:343`
2. **NumberConstraintImpl** - `internal/yamlutil/schema_interfaces.go:458`
3. **ArrayConstraintImpl** - `internal/yamlutil/schema_interfaces.go:560`
4. **ObjectConstraintImpl** - `internal/yamlutil/schema_interfaces.go:647`
5. **BooleanConstraintImpl** - `internal/yamlutil/schema_interfaces.go:746`
6. **TypeConstraintImpl** - `internal/yamlutil/schema_interfaces.go:795`

**Status:** ⚠️ **UNUSED IN PRODUCTION** - All constraint implementations are defined but never called

**Return Type:** `*ConstraintError` (pointer to ConstraintError struct)

---

## Production Call Site Details

### Site 1: SchemaValidator.Validate() → Schema.Validate()

**Location:** `internal/yamlutil/schema.go:180`
**Caller:** `SchemaValidator.Validate(data interface{}) SchemaValidationResult`
**Callee:** `Schema.Validate(value interface{}) error` (implemented by `SchemaDefinition`)

**Context:**
```go
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult {
    result := SchemaValidationResult{ /* ... */ }

    // Compile schema if not already compiled
    if !sv.compiled {
        if err := sv.compileSchema(); err != nil {
            result.Valid = false
            result.Errors = append(result.Errors, SchemaValidationError{
                Message: fmt.Sprintf("Invalid schema: %v", err),
            })
            return result
        }
        sv.compiled = true
    }

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

    // For SchemaDefinition, do detailed field validation
    if schemaDef, ok := sv.schema.(*SchemaDefinition); ok {
        // ... field validation ...
    }

    result.Valid = !result.HasErrors()
    return result
}
```

**Call Type:** Direct call with type assertion
**Error Handling:** ✅ **EXCELLENT**
- Uses type assertion to check for `YAMLError`
- Extracts error code from YAMLError
- Falls back to generic error handling for non-YAMLError types
- Populates SchemaValidationResult with structured error information

**Update Priority:** **LOW** - Already comprehensive

---

### Site 2: SchemaValidator.ValidateFile() → SchemaValidator.Validate()

**Location:** `internal/yamlutil/schema.go:253`
**Caller:** `SchemaValidator.ValidateFile(filePath string) SchemaValidationResult`
**Callee:** `SchemaValidator.Validate(data interface{}) SchemaValidationResult`

**Context:**
```go
func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult {
    result := SchemaValidationResult{ /* ... */ }

    // Read file content
    content, err := os.ReadFile(filePath)
    if err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Failed to read file: %v", err),
        })
        return result
    }

    // Parse YAML
    var data map[string]interface{}
    if err := yaml.Unmarshal(content, &data); err != nil {
        result.Valid = false
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Failed to parse YAML: %v", err),
        })
        return result
    }

    // Validate against schema
    return sv.Validate(data)
}
```

**Call Type:** Direct delegation
**Error Handling:** N/A - Returns SchemaValidationResult
**Update Priority:** **N/A** - Delegates to Validate() which handles errors

---

## Test Call Site Details

### Test Site 1: ValidatedSchema interface contract test

**Location:** `internal/yamlutil/schema_validation_test.go:94`

**Context:**
```go
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        err := tt.schema.Validate()

        if tt.wantErr {
            if err == nil {
                t.Errorf("%s: Validate() expected error but got nil", tt.name)
                return
            }
            // ... error type checking ...
        } else {
            if err != nil {
                t.Errorf("%s: Validate() unexpected error: %v", tt.name, err)
            }
        }
    })
}
```

**Status:** ✅ **TEST-ONLY**

---

### Test Site 2: SchemaDefinition interface compliance test

**Location:** `internal/yamlutil/schema_validation_test.go:147`

**Context:**
```go
schema := &SchemaDefinition{
    // ... setup ...
}

// Validate method should work
err := schema.Validate()
if err != nil {
    t.Errorf("Schema.Validate() unexpected error: %v", err)
}
```

**Status:** ✅ **TEST-ONLY**

---

### Test Site 3: SchemaValidator validator tests

**Location:** `internal/yamlutil/schema_validation_test.go:224, 310`

**Context:**
```go
validator := NewSchemaValidator(schema)
result := validator.Validate(tt.data)

if tt.wantErr {
    if result.Valid {
        t.Errorf("%s: Validate() expected errors but got valid result", tt.name)
    }
    if !result.HasErrors() {
        t.Errorf("%s: Validate() should have errors", tt.name)
    }
} else {
    if !result.Valid {
        t.Errorf("%s: Validate() unexpected errors: %v", tt.name, result.Errors)
    }
}
```

**Status:** ✅ **TEST-ONLY**

---

## Categorization Summary

### By Update Priority

| Method | Priority | Reason |
|--------|----------|--------|
| `SchemaValidator.Validate()` (line 180) | LOW | Already has comprehensive YAMLError handling |
| `SchemaValidator.ValidateFile()` (line 253) | N/A | Delegates to Validate() |
| `ValidatedSchema.Validate()` | N/A | Test-only interface |
| `Schema.Validate()` | N/A | Test-only interface |
| `Constraint.Validate()` implementations | N/A | Unused in production |

### By Type

| Type | Count | Methods |
|------|-------|---------|
| Production call sites | 2 | SchemaValidator.Validate() → Schema.Validate(), ValidateFile() → Validate() |
| Test-only implementations | 6 | All constraint implementations, ValidatedSchema tests |
| Unused methods | 6 | All constraint implementations |
| Proper error handling | 1 | SchemaValidator.Validate() (line 180) |
| Custom return type | 1 | SchemaValidator.Validate() returns SchemaValidationResult |

### By Return Type

| Method | Return Type | Call Sites |
|--------|-------------|------------|
| `ValidatedSchema.Validate()` | `YAMLError` | Test-only |
| `Schema.Validate()` | `error` | SchemaValidator line 180 |
| `SchemaValidator.Validate()` | `SchemaValidationResult` | ValidateFile line 253 |
| `Constraint.Validate()` | `*ConstraintError` | None (unused) |

---

## Error Type Analysis

### YAMLError Type Hierarchy

The codebase uses a structured error type hierarchy:

```go
// YAMLError interface (internal/yamlutil/errors.go)
type YAMLError interface {
    error
    Code() ErrCode
    Type() ErrorType
    FieldPath() string
    // ... other methods
}
```

**Error types in hierarchy:**
- `SchemaLoadError` - Schema loading failures
- `SchemaValidationError` - Schema definition invalid
- `ValidationError` - Data validation failures
- `FieldNotFoundError` - Required field missing
- `TypeMismatchError` - Type constraint violation
- `ConstraintError` - Constraint violation

### Error Conversion Status

**Current State:**

1. **SchemaValidator.Validate()** (line 180): ✅ **CORRECT**
   - Type-asserts errors to `YAMLError`
   - Extracts structured error information (Code, Message)
   - Falls back to generic error handling
   - No conversion needed - already handles YAMLError properly

2. **All other methods**: N/A - Either return custom types or are test-only

---

## Constraint Implementations (Unused)

All constraint implementations are **defined but never called in production code**:

| Implementation | Location | Validates | Status |
|----------------|----------|-----------|--------|
| `StringConstraintImpl` | schema_interfaces.go:343 | String length, pattern, allowed values | ⚠️ UNUSED |
| `NumberConstraintImpl` | schema_interfaces.go:458 | Numeric range, multiples | ⚠️ UNUSED |
| `ArrayConstraintImpl` | schema_interfaces.go:560 | Array length, uniqueness | ⚠️ UNUSED |
| `ObjectConstraintImpl` | schema_interfaces.go:647 | Required fields, property count | ⚠️ UNUSED |
| `BooleanConstraintImpl` | schema_interfaces.go:746 | Boolean value validation | ⚠️ UNUSED |
| `TypeConstraintImpl` | schema_interfaces.go:795 | Runtime type checking | ⚠️ UNUSED |

**Note:** These appear to be part of an incomplete refactoring or planned feature. The `Constraint` interface exists but no production code uses it.

---

## Analysis Summary

### Key Finding

**All production Validate() call sites already have proper error handling.**

1. **SchemaValidator.Validate()** (line 180) - Uses type assertion to handle YAMLError with structured error information
2. **SchemaValidator.ValidateFile()** (line 253) - Delegates to Validate() which handles errors properly

### No Migration Required

The Go ARMOR codebase already implements proper error handling for all Validate() call sites:
- YAMLError type assertions are in place
- Structured error information (error codes, field paths) is extracted
- Fallback generic error handling exists
- Custom return types are used where appropriate

### External Usage

**No external packages use yamlutil validation** - The yamlutil package is not imported by any other package in the ARMOR codebase. All usage is internal to yamlutil or in tests.

---

## Update Priority Matrix

| Site | Priority | Action Required |
|------|----------|-----------------|
| internal/yamlutil/schema.go:180 | ✅ LOW | Already correct - YAMLError handling in place |
| internal/yamlutil/schema.go:253 | ✅ N/A | Delegates to Validate() which handles errors |
| All test code | ✅ N/A | Test-only, no changes needed |
| Constraint implementations | ⚠️ UNUSED | Not called in production code |

**Total sites requiring updates: 0**

---

## Conclusion

The ARMOR Go codebase has **8 distinct Validate() method signatures**:

- **2 production call sites** - Both correctly implemented ✅
- **4 test call sites** - All use appropriate testing patterns ✅
- **6 constraint implementations** - Defined but unused ⚠️

**No systematic updates are required.** The error handling infrastructure is already in place and functioning correctly for all production code paths.

### Comparison with Rust ARMOR

The Go ARMOR codebase differs significantly from the Rust version:
- **Rust**: 142+ Validate() call sites across multiple traits and implementations
- **Go**: 8 Validate() methods, 2 production call sites (both properly handled)

The Go version's smaller validation footprint means all call sites already have proper error handling, unlike the Rust version which required systematic updates.

---

**Generated:** 2026-07-12
**Bead:** bf-52zl8
**Workspace:** /home/coding/ARMOR
