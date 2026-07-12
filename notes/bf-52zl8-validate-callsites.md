# Validate() Call Sites Catalog - ARMOR Codebase

**Bead:** bf-52zl8  
**Generated:** 2025-01-12  
**Purpose:** Catalog all Validate() call sites for error handling updates (bf-68hqo)

---

## Summary

Total Validate() interfaces/implementations found: **19**  
Total actual call sites (invocations): **3**  
Call sites needing error handling updates: **1** (HIGH PRIORITY)

---

## 1. Interface Definitions

### 1.1 ValidatedSchema Interface
**File:** `internal/yamlutil/schema_interfaces.go:34`  
**Method:** `Validate() YAMLError`

```go
type ValidatedSchema interface {
    Validate() YAMLError  // Validates schema definition itself
    Name() string
    Description() string
    Version() string
}
```

**Purpose:** Validates schema definitions themselves  
**Returns:** YAMLError from error hierarchy  
**Callers:** None found (schema self-validation)  
**Update Priority:** LOW - not currently invoked

---

### 1.2 Schema Interface
**File:** `internal/yamlutil/schema.go:51`  
**Method:** `Validate(value interface{}) error`

```go
type Schema interface {
    Validate(value interface{}) error
}
```

**Purpose:** Generic validation interface for any value type  
**Returns:** `error` (generic error type)  
**Implementations:** SchemaDefinition  
**Update Priority:** N/A - interface definition

---

### 1.3 SchemaValidationHandler Interface
**File:** `internal/yamlutil/schema_interfaces.go:71`  
**Method:** `Validate(data map[string]interface{}) SchemaValidationResult`

```go
type SchemaValidationHandler interface {
    Validate(data map[string]interface{}) SchemaValidationResult
    ValidateFile(filePath string) SchemaValidationResult
    ValidateSchema(schema ValidatedSchema) YAMLError
    ValidateValue(fieldPath string, value interface{}, fieldDef *FieldDefinition) YAMLError
}
```

**Purpose:** Schema-based validation with comprehensive error reporting  
**Returns:** SchemaValidationResult with detailed errors  
**Implementations:** SchemaValidator  
**Update Priority:** N/A - interface definition

---

### 1.4 Constraint Interface
**File:** `internal/yamlutil/schema_interfaces.go:89`  
**Method:** `Validate(value interface{}) *ConstraintError`

```go
type Constraint interface {
    Validate(value interface{}) *ConstraintError
    Description() string
    ConstraintType() string
}
```

**Purpose:** Base interface for all validation constraints  
**Returns:** *ConstraintError or nil  
**Implementations:** StringConstraint, NumberConstraint, ArrayConstraint, ObjectConstraint, BooleanConstraint, TypeConstraint  
**Update Priority:** N/A - interface definition

---

## 2. Method Implementations

### 2.1 SchemaDefinition.Validate()
**File:** `internal/yamlutil/schema.go:770`  
**Signature:** `func (s *SchemaDefinition) Validate(value interface{}) error`

**Implementation Details:**
- Returns generic `error` type
- Creates YAMLError instances (NewValidationError, NewTypeMismatchError, etc.)
- Callers must type-assert to access YAMLError methods

**Current Error Handling:**
```go
if value == nil {
    return NewValidationError("", "value cannot be nil", "", "", 
        ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")
}

data, ok := value.(map[string]interface{})
if !ok {
    return NewTypeMismatchError("", "", "map[string]interface{}", 
        fmt.Sprintf("%T", value), "", 0, ErrCodeTypeMismatch)
}

// Field validation with YAMLError returns
if fieldDef.Required {
    if _, exists := data[fieldName]; !exists {
        return NewFieldNotFoundError("", fieldName, 0, ErrCodeRequiredField)
    }
}

if err := s.validateField(fieldValue, fieldDef, fieldName); err != nil {
    return err  // Already YAMLError type
}
```

**Update Priority:** **MEDIUM** - Already returns YAMLError types, wrapped in generic `error` interface

---

### 2.2 SchemaValidator.Validate()
**File:** `internal/yamlutil/schema.go:157`  
**Signature:** `func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult`

**Implementation Details:**
- Calls `sv.schema.Validate(data)` at line 180
- **HIGH PRIORITY CALL SITE** - needs error handling update
- Currently uses type assertion to handle YAMLError

**Current Error Handling:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    // Type assertion to extract YAMLError information
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

**Update Priority:** **HIGH** - Main call site that extracts YAMLError via type assertion

---

### 2.3 SchemaValidator.ValidateFile()
**File:** `internal/yamlutil/schema.go:222`  
**Signature:** `func (sv *SchemaValidator) ValidateFile(filePath string) SchemaValidationResult`

**Implementation Details:**
- Calls `sv.Validate(data)` at line 253
- Indirect caller of Validate()
- No direct YAMLError handling (delegates to Validate())

**Current Error Handling:**
```go
// Validate against schema
return sv.Validate(data)
```

**Update Priority:** **LOW** - Delegates to Validate(), no direct error handling

---

### 2.4 Constraint Validators

#### StringConstraintImpl.Validate()
**File:** `internal/yamlutil/schema_interfaces.go:343`  
**Signature:** `func (sc *StringConstraintImpl) Validate(value interface{}) *ConstraintError`

**Update Priority:** **LOW** - Returns ConstraintError (YAMLError subtype)

#### NumberConstraintImpl.Validate()
**File:** `internal/yamlutil/schema_interfaces.go:458`  
**Signature:** `func (nc *NumberConstraintImpl) Validate(value interface{}) *ConstraintError`

**Update Priority:** **LOW** - Returns ConstraintError (YAMLError subtype)

#### ArrayConstraintImpl.Validate()
**File:** `internal/yamlutil/schema_interfaces.go:560`  
**Signature:** `func (ac *ArrayConstraintImpl) Validate(value interface{}) *ConstraintError`

**Update Priority:** **LOW** - Returns ConstraintError (YAMLError subtype)

#### ObjectConstraintImpl.Validate()
**File:** `internal/yamlutil/schema_interfaces.go:647`  
**Signature:** `func (oc *ObjectConstraintImpl) Validate(value interface{}) *ConstraintError`

**Update Priority:** **LOW** - Returns ConstraintError (YAMLError subtype)

#### BooleanConstraintImpl.Validate()
**File:** `internal/yamlutil/schema_interfaces.go:746`  
**Signature:** `func (bc *BooleanConstraintImpl) Validate(value interface{}) *ConstraintError`

**Update Priority:** **LOW** - Returns ConstraintError (YAMLError subtype)

#### TypeConstraintImpl.Validate()
**File:** `internal/yamlutil/schema_interfaces.go:795`  
**Signature:** `func (tc *TypeConstraintImpl) Validate(value interface{}) *ConstraintError`

**Update Priority:** **LOW** - Returns ConstraintError (YAMLError subtype)

---

## 3. Actual Call Sites (Invocations)

### 3.1 SchemaValidator.Validate() → Schema.Validate()
**File:** `internal/yamlutil/schema.go:180`  
**Code:** `if err := sv.schema.Validate(data); err != nil`

**Context:** Called within SchemaValidator.Validate() method  
**Callee:** Schema interface (implemented by SchemaDefinition)  
**Error Handling:** Type assertion to YAMLError  
**Update Priority:** **HIGH** - Main integration point

**Current Code:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
        })
    } else {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

**Recommended Update (after bf-68hqo):**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    // Direct YAMLError methods (no type assertion needed)
    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:      yamlErr.Error(),
            ErrorCode:    yamlErr.Code(),
            FilePath:     yamlErr.FilePath(),
            Line:         yamlErr.Line(),
            Column:       yamlErr.Column(),
            ErrorType:    yamlErr.Type(),
        })
    } else {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

---

### 3.2 SchemaValidator.ValidateFile() → SchemaValidator.Validate()
**File:** `internal/yamlutil/schema.go:253`  
**Code:** `return sv.Validate(data)`

**Context:** Called within SchemaValidator.ValidateFile() method  
**Callee:** SchemaValidator.Validate()  
**Error Handling:** Delegates to Validate()  
**Update Priority:** **LOW** - Indirect caller

**Current Code:**
```go
// Validate against schema
return sv.Validate(data)
```

**No update needed** - delegates to Validate() which handles YAMLError

---

### 3.3 Validator.ValidateStringWithPath()
**File:** `internal/yamlutil/validator.go:110`  
**Code:** `return v.ValidateStringWithPath(yamlContent, "<string>")`

**Context:** Delegates toWithPath variant  
**Callee:** Validator.ValidateStringWithPath()  
**Error Handling:** Direct delegation  
**Update Priority:** **NONE** - Different validator (not Schema-based)

---

## 4. Related Validation Methods (Non-Schema)

### 4.1 Validator.ValidateString()
**File:** `internal/yamlutil/validator.go:109`  
**Returns:** ValidationResult (not YAMLError)  
**Update Priority:** NONE - Different validation system

### 4.2 Validator.ValidateFile()
**File:** `internal/yamlutil/validator.go:152`  
**Returns:** ValidationResult (not YAMLError)  
**Update Priority:** NONE - Different validation system

### 4.3 Validator.ValidateMultipleFiles()
**File:** `internal/yamlutil/validator.go:312`  
**Returns:** []ValidationResult (not YAMLError)  
**Update Priority:** NONE - Different validation system

### 4.4 DefaultSyntaxValidator.ValidateSyntax()
**File:** `internal/yamlutil/syntax_validator.go:364`  
**Returns:** SyntaxValidationResult (not YAMLError)  
**Update Priority:** NONE - Different validation system

### 4.5 ValidateMappingKeyIndent()
**File:** `internal/yamlutil/key_detection.go:222`  
**Returns:** bool (not error)  
**Update Priority:** NONE - Utility function

---

## 5. Categorization by Update Priority

### HIGH PRIORITY (Requires immediate update)

1. **internal/yamlutil/schema.go:180** - `sv.schema.Validate(data)`
   - **Reason:** Main integration point between SchemaValidator and Schema
   - **Current:** Type assertion to extract basic YAMLError info
   - **Needed:** Extract full YAMLError context (FilePath, Line, Column, Type)
   - **Impact:** Critical for rich error reporting

### MEDIUM PRIORITY (Consider for consistency)

1. **internal/yamlutil/schema.go:770** - SchemaDefinition.Validate()
   - **Reason:** Implementation already returns YAMLError types
   - **Current:** Returns generic `error` interface
   - **Needed:** Consider changing signature to return YAMLError directly
   - **Impact:** Eliminates type assertions at call sites

### LOW PRIORITY (No immediate action needed)

1. **internal/yamlutil/schema.go:253** - `sv.Validate(data)` delegation
   - **Reason:** Indirect caller, delegates error handling
   - **Impact:** No changes needed

2. **All Constraint validators** - Return *ConstraintError
   - **Reason:** Already return YAMLError subtype correctly
   - **Impact:** No changes needed

3. **ValidatedSchema.Validate()** - Schema self-validation
   - **Reason:** No actual callers found
   - **Impact:** No changes needed until invoked

---

## 6. Call Flow Diagram

```
External Call
    ↓
SchemaValidator.ValidateFile(path)
    ↓
[Parse YAML file]
    ↓
SchemaValidator.Validate(data)
    ↓
SchemaValidator.Validate() ←── HIGH PRIORITY UPDATE SITE
    ↓
sv.schema.Validate(data) ←── Schema interface call
    ↓
SchemaDefinition.Validate(value) ←── MEDIUM PRIORITY (signature)
    ↓
[YAMLError types returned: ValidationError, TypeMismatchError, etc.]
    ↓
Type assertion at line 184-188
    ↓
SchemaValidationResult populated
```

---

## 7. Error Handling Patterns

### Pattern 1: Type Assertion (Current - Line 180-195)
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    if yamlErr, ok := err.(YAMLError); ok {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
        })
    } else {
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

### Pattern 2: YAMLError Creation (SchemaDefinition - Line 770+)
```go
return NewValidationError("", "value cannot be nil", "", "", 
    ErrCodeValidationFailed, 0, 0, ErrorTypeValidation, "")

return NewTypeMismatchError("", "", "map[string]interface{}", 
    fmt.Sprintf("%T", value), "", 0, ErrCodeTypeMismatch)

return NewFieldNotFoundError("", fieldName, 0, ErrCodeRequiredField)
```

### Pattern 3: Direct YAMLError Return (Constraint validators)
```go
func (sc *StringConstraintImpl) Validate(value interface{}) *ConstraintError {
    if !sc.validate(value) {
        return &ConstraintError{
            ConstraintType: sc.type,
            Message:        fmt.Sprintf("..."),
            Value:          value,
        }
    }
    return nil
}
```

---

## 8. Recommendations for Systematic Updates

### Phase 1: Update HIGH Priority Site
1. **File:** `internal/yamlutil/schema.go:180-195`
2. **Changes:**
   - Extract FilePath, Line, Column, Type from YAMLError
   - Populate SchemaValidationError with full context
   - Maintain backward compatibility with generic error fallback

### Phase 2: Update MEDIUM Priority Site (Optional)
1. **File:** `internal/yamlutil/schema.go:770`
2. **Changes:**
   - Consider changing signature: `Validate(value interface{}) YAMLError`
   - Eliminates type assertions at call sites
   - Breaking change - requires coordination

### Phase 3: Document Error Handling Pattern
1. Add documentation comments showing YAMLError handling pattern
2. Create helper function: `convertYAMLErrorToSchemaValidationError(err error)`
3. Update all call sites to use helper

### Phase 4: Testing
1. Add unit tests for YAMLError extraction
2. Add integration tests for full error context propagation
3. Verify error reporting in CLI/UI output

---

## 9. Related Files

### Core Schema Implementation
- `internal/yamlutil/schema.go` - Schema interface, SchemaDefinition, SchemaValidator
- `internal/yamlutil/schema_interfaces.go` - Validation interfaces, Constraint interfaces

### Error Types (bf-68hqo)
- `internal/yamlutil/errors.go` - YAMLError hierarchy implementation

### Validation Results
- `internal/yamlutil/result_types.go` - SchemaValidationResult, ValidationResult

### Other Validators
- `internal/yamlutil/validator.go` - Generic YAML validator
- `internal/yamlutil/syntax_validator.go` - Syntax-specific validation
- `internal/yamlutil/key_detection.go` - Indentation validation

### Configuration
- `internal/yamlutil/config.go` - Validator configuration, custom validators

### Documentation
- `internal/yamlutil/doc.go` - Usage examples (call sites in comments)

---

## 10. Statistics

| Category | Count |
|----------|-------|
| Interface Definitions | 4 |
| Method Implementations | 11 |
| Actual Call Sites | 3 |
| HIGH Priority Updates | 1 |
| MEDIUM Priority Updates | 1 |
| LOW/NO Priority Updates | 17 |

---

**Next Steps:** Proceed with HIGH priority site update (bf-68hqo integration)
