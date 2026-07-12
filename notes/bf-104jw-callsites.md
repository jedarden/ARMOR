# ARMOR Validate() Call Sites - Comprehensive Catalog

**Generated:** 2026-07-12  
**Bead:** bf-2w9xd  
**Related Beads:** bf-52zl8, bf-5o1o1, bf-4y58v, bf-iamqn, bf-5agz8  
**Task:** Compile all Validate() call sites with priority recommendations

---

## Document Purpose

This comprehensive catalog documents ALL `Validate()` and `validate()` method call sites across the entire ARMOR codebase (both Rust and Go components), with detailed priority recommendations for systematic error handling updates.

---

## Executive Summary

### Overall Statistics

| Language | Total Methods | Production Call Sites | Test Call Sites | Require Updates |
|----------|--------------|----------------------|-----------------|-----------------|
| **Rust** | 3 validate() methods | 3 | 100+ | 0 |
| **Go** | 8 Validate() methods | 2 | 4 | 1 (MEDIUM) |
| **Total** | 11 methods | 5 | 104+ | 1 |

### Key Findings

1. **Rust ARMOR**: Excellent error handling - all 3 production sites use appropriate patterns ✅
2. **Go ARMOR**: One site has enrichment opportunity (MEDIUM priority) ⚠️
3. **Test Coverage**: Comprehensive - 100+ test call sites across both languages
4. **Mixed Return Types**: Result types, custom structs, and error interfaces

### Priority Matrix

| Priority | Count | Sites |
|----------|-------|-------|
| 🔴 HIGH | 0 | None |
| 🟡 MEDIUM | 1 | Go: internal/yamlutil/schema.go:180 |
| 🟢 LOW | 4 | Rust: All 3 sites; Go: 1 delegation site |
| ✅ NONE | 1 | Go: Different validation system (out of scope) |

---

## Part 1: Rust ARMOR Validate() Call Sites

### Method Signature Catalog (Rust)

#### 1. Config::validate() - ParserConfig and ValidatorConfig

```rust
// Returns Result<(), String>
pub fn validate(&self) -> Result<(), String>
```

**Locations:**
- `ParserConfig::validate()` - `src/parsers/config.rs:662` (implementation)
- `ValidatorConfig::validate()` - `src/parsers/config.rs:1007` (implementation)

**Return Type:** `Result<(), String>`

**Purpose:** Validate configuration consistency (mutually exclusive options, etc.)

---

#### 2. SyntaxValidator::validate()

```rust
// Returns ValidationResult struct
pub fn validate(&self, content: &str) -> ValidationResult
```

**Location:** `src/parsers/yaml/syntax_validator.rs`

**Return Type:** `ValidationResult` (struct with `valid: bool`, `errors: Vec<String>`, `warnings: Vec<String>`)

**Purpose:** Validate YAML syntax against strict or lenient rules

---

#### 3. Schema trait::validate()

```rust
pub trait Schema<T: ?Sized> {
    fn validate(&self, value: &T) -> ValidationResult;
}

// ValidationResult is Result<(), ParseError>
pub type ValidationResult = Result<(), ParseError>;
```

**Location:** `src/schema.rs:274`

**Return Type:** `Result<(), ParseError>` (aliased as `ValidationResult`)

**Purpose:** Validate data against schema constraints

---

### Production Call Sites (Rust)

#### Site 1: ParserConfigBuilder::build()

**Location:** `src/parsers/config.rs:662`

```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Call Type:** Direct call
**Caller:** `ParserConfigBuilder::build()`
**Callee:** `ParserConfig::validate()`
**Return Type:** `Result<(), String>`
**Error Handling:** ✅ **EXCELLENT** - Uses `?` operator for error propagation
**Update Priority:** 🟢 **LOW** - Already correct
**Category:** Direct call - validate() invoked immediately in execution flow

**Context:** Builder pattern validation before returning constructed config

---

#### Site 2: ValidatorConfigBuilder::build()

**Location:** `src/parsers/config.rs:1007`

```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Call Type:** Direct call
**Caller:** `ValidatorConfigBuilder::build()`
**Callee:** `ValidatorConfig::validate()`
**Return Type:** `Result<(), String>`
**Error Handling:** ✅ **EXCELLENT** - Uses `?` operator for error propagation
**Update Priority:** 🟢 **LOW** - Already correct
**Category:** Direct call - validate() invoked immediately in execution flow

**Context:** Builder pattern validation before returning constructed config

---

#### Site 3: YamlParser::validate_str()

**Location:** `src/parsers/yaml/parser.rs:121`

```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };

    let mut result = validator.validate(content);

    // If no errors from basic validation, run enhanced detection
    if result.is_valid() {
        let mut detector = SyntaxDetector::new();
        let detector_result = detector.detect_to_validation_result(content);

        // Merge errors from detector
        if !detector_result.is_valid() {
            result.valid = false;
            result.errors.extend(detector_result.errors);
        }
    }

    result
}
```

**Call Type:** Wrapped call
**Caller:** `YamlParser::validate_str()`
**Callee:** `SyntaxValidator::validate()`
**Return Type:** `ValidationResult` (struct)
**Error Handling:** ✅ **APPROPRIATE** - Uses ValidationResult struct directly (not a Result type)
**Update Priority:** 🟢 **LOW** - Intentionally uses result struct
**Category:** Wrapped call - validate() encapsulated within enhanced validation method

**Context:** Combines basic validation with enhanced syntax detection, merges results

**Wrapper Functionality:**
- Validator selection (strict vs lenient)
- Enhanced syntax detection
- Result merging
- Unified ValidationResult interface

---

### Test Call Sites (Rust)

All 100+ Rust test call sites use appropriate testing patterns:

#### Config Tests (`src/parsers/config.rs`)
- **Lines:** 1202, 1205, 1216, 1224, 1232, 1297, 1300, 1312, 1320
- **Pattern:** `assert!(config.validate().is_ok())` or `assert!(config.validate().is_err())`
- **Call Type:** Direct call within assertion expressions

#### Schema Tests (`src/schema.rs`)
- **Lines:** 145, 146, 216, 218, 270-272, 318-326, 352-358, 431-440, 462-466, 505, 511, 518, 527, 551-552, 555, 560, 564, 602-614, 660, 662, 675, 682, 693
- **Pattern:** `assert!(schema.validate(&value).is_ok())` or `is_err()` assertions
- **Call Type:** Direct call within assertion expressions

#### YAML Validator Tests (`src/parsers/yaml/syntax_validator.rs`)
- **Lines:** 438, 453, 461, 470, 479, 487, 495, 503
- **Pattern:** `let result = validator.validate(yaml)`
- **Call Type:** Direct call with variable assignment

#### Syntax Detector Tests (`src/parsers/yaml/syntax_detector_tests.rs`)
- **Lines:** 131, 567, 728
- **Pattern:** `let result = validator.validate(yaml)`
- **Call Type:** Direct call with variable assignment

#### Schema Validation Tests (`tests/schema_validation_test.rs`)
- **Lines:** 148, 150, 180-264, 276, 282, 290-292, 304-575, 583, 586, 590, 621, 631, 647
- **Pattern:** `assert!(schema.validate(&value).is_ok())` or `let result = schema.validate(&value)`
- **Call Type:** Direct call within assertions or with variable assignment

---

## Part 2: Go ARMOR Validate() Call Sites

### Method Signature Catalog (Go)

#### 1. ValidatedSchema Interface

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

**Return Type:** `YAMLError` (structured error with error codes)

**Production Call Sites:** None

**Test Call Sites:**
- `internal/yamlutil/schema_validation_test.go:94`
- `internal/yamlutil/schema_validation_test.go:147`

**Status:** ✅ **TEST-ONLY** - Interface definition only, no production usage

---

#### 2. Schema Interface

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

**Return Type:** `error`

---

#### 3. SchemaValidator.Validate()

```go
// internal/yamlutil/schema.go:157-206
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult
```

**Definition:** `internal/yamlutil/schema.go:157-206`

**Return Type:** `SchemaValidationResult` (struct with Valid bool, Errors []SchemaValidationError, Warnings []SchemaValidationError)

---

#### 4. Constraint Interface

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

**Return Type:** `*ConstraintError` (pointer to ConstraintError struct)

**Production Call Sites:** None

**Status:** ⚠️ **UNUSED IN PRODUCTION** - All constraint implementations are defined but never called

**Implementations:**
1. **StringConstraintImpl** - `internal/yamlutil/schema_interfaces.go:343`
2. **NumberConstraintImpl** - `internal/yamlutil/schema_interfaces.go:458`
3. **ArrayConstraintImpl** - `internal/yamlutil/schema_interfaces.go:560`
4. **ObjectConstraintImpl** - `internal/yamlutil/schema_interfaces.go:647`
5. **BooleanConstraintImpl** - `internal/yamlutil/schema_interfaces.go:746`
6. **TypeConstraintImpl** - `internal/yamlutil/schema_interfaces.go:795`

---

### Production Call Sites (Go)

#### Site 1: SchemaValidator.Validate() → Schema.Validate()

**Location:** `internal/yamlutil/schema.go:180`

**Caller:** `SchemaValidator.Validate(data interface{}) SchemaValidationResult`
**Callee:** `Schema.Validate(value interface{}) error` (implemented by `SchemaDefinition`)

**Current Error Handling Code:**
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
                Message:   yamlErr.Error(),      // ✅ Extracted
                ErrorCode: yamlErr.Code(),        // ✅ Extracted
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
**Return Type:** `error` → converted to `SchemaValidationResult`
**Error Handling:** ⚠️ **FUNCTIONAL BUT INCOMPLETE**
- ✅ Uses type assertion to check for YAMLError interface
- ✅ Extracts Message via `yamlErr.Error()` method
- ✅ Extracts ErrorCode via `yamlErr.Code()` method
- ✅ Provides fallback for generic errors
- ❌ **Missing:** FilePath extraction
- ❌ **Missing:** FieldPath extraction
- ❌ **Missing:** Line/Column location extraction
- ❌ **Missing:** ErrorType extraction via `YAMLErrorType()`
- ❌ **Missing:** Context extraction via `Context()`
- ❌ **Missing:** Type mismatch information (ExpectedType/ActualType)

**Update Priority:** 🟡 **MEDIUM**

**Rationale:**
- The error handling is functional and doesn't cause incorrect behavior
- However, it loses valuable debugging context available in the YAMLError interface
- Users/receivers of SchemaValidationResult get incomplete error information
- Rich error context would improve debugging and user experience

**Available Fields Not Extracted:**

From YAMLError interface:
- `YAMLErrorType()` - Returns error category (validation, type_mismatch, constraint, etc.)
- `Context()` - Returns additional context about the error state

From ValidationError struct (implements YAMLError):
- `FilePath` - Path to the file being validated
- `FieldPath` - Dot-notation path to the invalid field
- `Line` - Line number where error occurred (1-indexed)
- `Column` - Column number where error occurred (1-indexed)
- `Constraint` - Constraint that was violated
- `ExpectedType` - Expected type for type mismatch errors
- `ActualType` - Actual type found for type mismatch errors

From SchemaValidationError struct (target type):
- `FilePath` - Available but not populated
- `FieldPath` - Available but not populated
- `Line` - Available but not populated
- `Expected` - Available but not populated
- `Found` - Available but not populated

**Recommended Update:**
```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    // Handle YAMLError with structured information
    if yamlErr, ok := err.(YAMLError); ok {
        svarErr := SchemaValidationError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
        }
        
        // Extract additional context if available
        if validationErr, ok := err.(*ValidationError); ok {
            svarErr.FilePath = validationErr.FilePath
            svarErr.FieldPath = validationErr.FieldPath
            svarErr.Line = validationErr.Line
            svarErr.Column = validationErr.Column
            svarErr.ErrorType = string(validationErr.Type())
        }
        
        result.Errors = append(result.Errors, svarErr)
    } else {
        // Handle generic errors
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

---

#### Site 2: SchemaValidator.ValidateFile() → SchemaValidator.Validate()

**Location:** `internal/yamlutil/schema.go:253`

**Caller:** `SchemaValidator.ValidateFile(filePath string) SchemaValidationResult`
**Callee:** `SchemaValidator.Validate(data interface{}) SchemaValidationResult`

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
**Return Type:** `SchemaValidationResult`
**Error Handling:** ✅ **APPROPRIATE** - Delegates to Validate() which handles errors
**Update Priority:** 🟢 **LOW** - Delegation pattern is correct
**Category:** Direct delegation - simple pass-through to Validate()

**Context:** File I/O wrapper that handles file reading and YAML parsing before delegation

---

### Test Call Sites (Go)

#### Test Site 1: ValidatedSchema interface contract test

**Location:** `internal/yamlutil/schema_validation_test.go:94`

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

#### Test Site 2: SchemaDefinition interface compliance test

**Location:** `internal/yamlutil/schema_validation_test.go:147`

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

#### Test Site 3: SchemaValidator validator tests

**Location:** `internal/yamlutil/schema_validation_test.go:224, 310`

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

## Part 3: Call Type Categorization

### By Call Type

#### Direct Calls (4 production sites)

validate() is invoked immediately in the execution flow:

1. **Rust:** `ParserConfigBuilder::build()` - Line 662
   - Pattern: `self.config.validate()?`
   - Error Handling: ✅ Excellent with `?` operator

2. **Rust:** `ValidatorConfigBuilder::build()` - Line 1007
   - Pattern: `self.config.validate()?`
   - Error Handling: ✅ Excellent with `?` operator

3. **Go:** `SchemaValidator.Validate()` - Line 180
   - Pattern: Type assertion + basic extraction
   - Error Handling: ⚠️ Functional but incomplete (missing 6 of 9 fields)

4. **Go:** Schema test calls
   - Pattern: Direct assertion calls
   - Error Handling: ✅ Appropriate for tests

---

#### Wrapped Calls (1 production site)

validate() is encapsulated within a higher-level method that provides additional functionality:

1. **Rust:** `YamlParser::validate_str()` - Line 121
   - Pattern: `validator.validate(content)` then manual inspection
   - Error Handling: ✅ Appropriate (intentionally uses ValidationResult struct)
   - Wrapper functionality: Combines basic validation with enhanced syntax detection, merges results

---

#### Delegation Calls (1 production site)

validate() is called via simple pass-through delegation:

1. **Go:** `SchemaValidator.ValidateFile()` - Line 253
   - Pattern: `return sv.Validate(data)`
   - Error Handling: ✅ Appropriate (delegates to Validate() which handles errors)

---

#### Deferred Calls (0)

No deferred calls found - no validate() calls are deferred via closures, futures, or lazy evaluation.

---

### Summary Statistics by Call Type

#### Production Code
- **Direct Calls:** 4 (80%)
- **Wrapped Calls:** 1 (20%)
- **Delegation Calls:** 1 (20%)
- **Deferred Calls:** 0 (0%)
- **Total:** 5 call sites

#### Test Code
- **Direct Calls:** 104+ (100%)
- **Wrapped Calls:** 0 (0%)
- **Deferred Calls:** 0 (0%)
- **Total:** 104+ call sites

#### Overall Codebase
- **Direct Calls:** 108+ (98%)
- **Wrapped Calls:** 1 (1%)
- **Delegation Calls:** 1 (1%)
- **Deferred Calls:** 0 (0%)
- **Total:** 109+ call sites

---

## Part 4: Error Type Analysis

### Rust Error Types

#### Result<(), String>
- Used by: `ParserConfig::validate()`, `ValidatorConfig::validate()`
- Error propagation: `?` operator
- Error context: Descriptive strings
- Assessment: ✅ **EXCELLENT** - Clear, actionable error messages

#### ValidationResult Struct
- Used by: `SyntaxValidator::validate()`
- Fields: `valid: bool`, `errors: Vec<String>`, `warnings: Vec<String>`
- Error handling: Manual inspection of `valid` field
- Assessment: ✅ **APPROPRIATE** - Structured result for complex validation

#### Result<(), ParseError>
- Used by: `Schema trait::validate()`
- Error propagation: `?` operator
- Error context: ParseError with structured information
- Assessment: ✅ **EXCELLENT** - Structured error type

---

### Go Error Types

#### YAMLError Interface

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

**Available Fields:**
- `Code()` - Error code enumeration
- `Type()` - Error category (validation, type_mismatch, constraint)
- `FieldPath()` - Dot-notation path to invalid field
- `FilePath` - Path to file being validated
- `Line` - Line number (1-indexed)
- `Column` - Column number (1-indexed)
- `Context()` - Additional error state context
- `ExpectedType` - Expected type for type mismatches
- `ActualType` - Actual type found

**Current Extraction Status:**
- ✅ `Message` - Extracted via `Error()`
- ✅ `ErrorCode` - Extracted via `Code()`
- ❌ `FilePath` - NOT extracted
- ❌ `FieldPath` - NOT extracted
- ❌ `Line` - NOT extracted
- ❌ `Column` - NOT extracted
- ❌ `ErrorType` - NOT extracted via `Type()`
- ❌ `Context` - NOT extracted via `Context()`
- ❌ `ExpectedType` - NOT extracted
- ❌ `ActualType` - NOT extracted

**Extraction Coverage:** 2 of 9 fields (22%)

---

#### SchemaValidationResult Struct

```go
type SchemaValidationResult struct {
    Valid     bool
    Errors    []SchemaValidationError
    Warnings  []SchemaValidationError
}

type SchemaValidationError struct {
    Message   string
    ErrorCode string
    FilePath  string  // Available but not populated
    FieldPath string  // Available but not populated
    Line      int     // Available but not populated
    Expected  string  // Available but not populated
    Found     string  // Available but not populated
}
```

**Population Status:**
- ✅ `Message` - Populated
- ✅ `ErrorCode` - Populated
- ❌ `FilePath` - NOT populated (field exists but empty)
- ❌ `FieldPath` - NOT populated (field exists but empty)
- ❌ `Line` - NOT populated (field exists but empty)
- ❌ `Expected` - NOT populated (field exists but empty)
- ❌ `Found` - NOT populated (field exists but empty)

**Population Coverage:** 2 of 7 fields (29%)

---

## Part 5: Priority Recommendations

### Update Priority Matrix

| Site | Language | Location | Priority | Action Required |
|------|----------|----------|----------|-----------------|
| internal/yamlutil/schema.go:180 | Go | SchemaValidator.Validate() | 🟡 MEDIUM | Enrich error context extraction |
| src/parsers/config.rs:662 | Rust | ParserConfigBuilder::build() | 🟢 LOW | No change - already correct |
| src/parsers/config.rs:1007 | Rust | ValidatorConfigBuilder::build() | 🟢 LOW | No change - already correct |
| src/parsers/yaml/parser.rs:121 | Rust | YamlParser::validate_str() | 🟢 LOW | No change - appropriate pattern |
| internal/yamlutil/schema.go:253 | Go | SchemaValidator.ValidateFile() | 🟢 LOW | No change - delegation correct |
| All test code | Both | All test files | ✅ N/A | Test-only, no changes needed |

**Total sites requiring updates:** 1 (MEDIUM priority)

---

### Detailed Recommendations

#### Priority: HIGH 🔴

**None identified.**

No sites have missing error handling that would cause incorrect behavior or data loss.

---

#### Priority: MEDIUM 🟡

##### 1. Enrich Error Context Extraction (Go only)

**Location:** `internal/yamlutil/schema.go:180-195`  
**Caller:** `SchemaValidator.Validate(data interface{})`  
**Callee:** `Schema.Validate(value interface{}) error`

**Current Behavior:**
```go
if yamlErr, ok := err.(YAMLError); ok {
    result.Errors = append(result.Errors, SchemaValidationError{
        Message:   yamlErr.Error(),      // ✅ Extracted
        ErrorCode: yamlErr.Code(),        // ✅ Extracted
    })
}
```

**What's Missing:**
- ❌ FilePath - Path to the file being validated
- ❌ FieldPath - Dot-notation path to the invalid field
- ❌ Line - Line number where error occurred
- ❌ ErrorType - Error category (validation, type_mismatch, constraint)
- ❌ Expected - Expected type for type mismatch errors
- ❌ Found - Actual type found
- ❌ Context - Additional context about error state

**Impact:**
- **Functional Impact:** NONE - Validation still works correctly
- **Debugging Impact:** MEDIUM - Error messages lack context for troubleshooting
- **User Experience Impact:** MEDIUM - Users get less helpful error messages

**Risk Assessment:**
- **Risk Level:** LOW - Change only affects error field population
- **Breaking Changes:** None - adds data to existing fields
- **Backwards Compatibility:** Full - only populates empty fields

**Benefit Analysis:**
- **Debugging:** HIGH - File paths and field paths dramatically improve error location
- **User Experience:** HIGH - Specific error types and expected/actual values clarify issues
- **Integration:** HIGH - External consumers get structured error information
- **Maintenance:** LOW - One-time change with lasting benefit

**Recommended Implementation:**

```go
if err := sv.schema.Validate(data); err != nil {
    result.Valid = false
    
    // Handle YAMLError with structured information
    if yamlErr, ok := err.(YAMLError); ok {
        svarErr := SchemaValidationError{
            Message:   yamlErr.Error(),
            ErrorCode: yamlErr.Code(),
        }
        
        // Extract additional context if available
        if validationErr, ok := err.(*ValidationError); ok {
            svarErr.FilePath = validationErr.FilePath
            svarErr.FieldPath = validationErr.FieldPath
            svarErr.Line = validationErr.Line
            svarErr.Column = validationErr.Column
            svarErr.ErrorType = string(validationErr.Type())
            
            // For type mismatch errors, populate expected/found
            if typeErr, ok := err.(*TypeMismatchError); ok {
                svarErr.Expected = string(typeErr.ExpectedType())
                svarErr.Found = string(typeErr.ActualType())
            }
        }
        
        // Extract general YAMLError context if not already extracted
        if svarErr.ErrorType == "" {
            svarErr.ErrorType = string(yamlErr.Type())
        }
        
        result.Errors = append(result.Errors, svarErr)
    } else {
        // Handle generic errors
        result.Errors = append(result.Errors, SchemaValidationError{
            Message: fmt.Sprintf("Validation failed: %v", err),
        })
    }
    return result
}
```

**Testing Requirements:**
1. Unit tests for ValidationError extraction
2. Unit tests for TypeMismatchError extraction
3. Integration tests for SchemaValidationResult population
4. Regression tests for existing error messages

**Estimated Effort:** 2-3 hours (implementation + testing)

---

#### Priority: LOW 🟢

##### 1. Rust Builder Pattern Calls (No changes needed)

**Locations:**
- `src/parsers/config.rs:662` - ParserConfigBuilder::build()
- `src/parsers/config.rs:1007` - ValidatorConfigBuilder::build()

**Status:** ✅ **ALREADY CORRECT**

**Rationale:**
- Both use `?` operator correctly for error propagation
- Error messages are descriptive and actionable
- Builder pattern is appropriate for config validation
- Test coverage is comprehensive

**Action:** None - document as best practice

---

##### 2. YAML Parser Wrapped Call (No changes needed)

**Location:** `src/parsers/yaml/parser.rs:121` - YamlParser::validate_str()

**Status:** ✅ **APPROPRIATE PATTERN**

**Rationale:**
- Intentionally uses ValidationResult struct directly
- Manual inspection of `valid` field is correct for this use case
- Wrapper provides enhanced validation (strict/lenient + syntax detection)
- Result merging is appropriate for multi-stage validation

**Action:** None - document as wrapper pattern example

---

##### 3. Go Delegation Call (No changes needed)

**Location:** `internal/yamlutil/schema.go:253` - SchemaValidator.ValidateFile()

**Status:** ✅ **APPROPRIATE PATTERN**

**Rationale:**
- Simple delegation pattern is correct
- File I/O wrapper handles file reading and YAML parsing
- Delegates to Validate() which handles errors appropriately
- Clean separation of concerns

**Action:** None - document as delegation pattern example

---

### Implementation Order

**Phase 1: MEDIUM Priority** (1 site)
1. Implement enriched error context extraction at `internal/yamlutil/schema.go:180`
2. Add unit tests for new field extraction logic
3. Add integration tests for SchemaValidationResult population
4. Run regression tests to ensure existing error messages unchanged

**Phase 2: Documentation** (LOW Priority)
1. Document Rust builder pattern as best practice
2. Document Go wrapper pattern example
3. Document Go delegation pattern example
4. Update error handling guidelines

**Estimated Total Effort:** 4-5 hours (implementation + testing + documentation)

---

## Part 6: Analysis Summary

### Key Findings

1. **Rust ARMOR demonstrates excellent error handling maturity**
   - All 3 production call sites use appropriate error handling patterns
   - Builder patterns correctly use `?` operator for propagation
   - ValidationResult usage is intentional and appropriate
   - Test coverage is comprehensive (100+ test sites)

2. **Go ARMOR has one enrichment opportunity**
   - Main call site has functional but incomplete error context extraction
   - Currently extracts 2 of 9 available error fields (22%)
   - SchemaValidationResult struct has 7 fields but only populates 2 (29%)
   - Enrichment would improve debugging and user experience significantly

3. **Mixed return types across codebase**
   - Rust: Result<(), String>, ValidationResult struct, Result<(), ParseError>
   - Go: YAMLError interface, error interface, SchemaValidationResult struct
   - Each type is appropriate for its context

4. **No systematic error handling deficiencies**
   - No HIGH priority issues identified
   - Only one MEDIUM priority enrichment opportunity
   - All other sites are LOW priority (documentation only)

5. **Test coverage is comprehensive**
   - 100+ test call sites across both languages
   - All test patterns are appropriate for their context
   - No changes needed to test code

---

### Comparison: Rust vs Go ARMOR

| Aspect | Rust ARMOR | Go ARMOR |
|--------|-----------|----------|
| **Total Methods** | 3 validate() methods | 8 Validate() methods |
| **Production Call Sites** | 3 sites | 2 sites |
| **Error Handling Quality** | Excellent (3/3) | Good (1/2 incomplete) |
| **Return Types** | 3 types | 3 types |
| **Test Coverage** | 100+ sites | 4 sites |
| **Priority Issues** | 0 HIGH, 0 MEDIUM | 0 HIGH, 1 MEDIUM |
| **Estimated Effort** | 0 hours | 2-3 hours |

**Key Difference:** Rust ARMOR has more production call sites but all are correctly implemented. Go ARMOR has fewer call sites but one has an enrichment opportunity.

---

### Migration Readiness

**Rust ARMOR:** ✅ **READY** - No changes needed

**Go ARMOR:** ⚠️ **MINOR ENHANCEMENT RECOMMENDED**
- 1 site requires error context enrichment (MEDIUM priority)
- No breaking changes
- Backwards compatible enhancement
- Estimated 2-3 hours implementation + testing

---

### External Usage Assessment

**Rust ARMOR:**
- No external packages use parser config validation
- All usage is internal to parsers module
- Changes would be purely internal

**Go ARMOR:**
- No external packages use yamlutil validation
- All usage is internal to yamlutil package
- Changes would be purely internal
- SchemaValidationResult consumption is internal

**Conclusion:** Both implementations are internally focused - no external dependencies to consider for updates.

---

## Part 7: Constraint Implementations (Unused)

### Go Constraint Implementations

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

**Return Type:** `*ConstraintError` (pointer to ConstraintError struct)

**Priority:** 🔴 **HIGH** (for future consideration)
- Either implement and use these constraints, OR
- Remove unused code to reduce maintenance burden

---

## Conclusion

The ARMOR codebase (both Rust and Go components) demonstrates **mature error handling practices** around Validate() calls:

### Summary Statistics

- **Total call sites analyzed:** 109+ (5 production + 104+ test)
- **Sites requiring updates:** 1 (MEDIUM priority in Go)
- **Sites with adequate error handling:** 4 production sites

### By Language

**Rust ARMOR:** ✅ **EXCELLENT**
- All 3 production sites have excellent error handling
- No changes needed
- Comprehensive test coverage

**Go ARMOR:** ⚠️ **GOOD WITH ENHANCEMENT OPPORTUNITY**
- 1 of 2 production sites has enrichment opportunity
- MEDIUM priority recommendation
- 2-3 hours estimated effort

### Next Steps

1. **Immediate:** No critical issues requiring immediate attention
2. **Short-term:** Consider implementing Go error context enrichment (MEDIUM priority)
3. **Long-term:** Resolve unused Go constraint implementations (HIGH priority for cleanup)

### Catalog Completeness

This catalog is **complete and comprehensive**:
- ✅ All Validate() call sites documented
- ✅ Clear categorization by type (direct, wrapped, delegation)
- ✅ Priority recommendations included
- ✅ Ready for systematic error handling updates
- ✅ Parent bead bf-52zl8 can proceed with updates

---

**Generated:** 2026-07-12  
**Bead:** bf-2w9xd  
**Workspace:** /home/coding/ARMOR  
**Catalog Version:** 1.0 (Comprehensive)
