# Validate() Call Sites - Detailed Context Documentation

**Bead ID:** bf-4yxbs  
**Task:** Document each Validate() call site with context  
**Date:** 2026-07-12  
**Workspace:** /home/coding/ARMOR

## Overview

This document provides detailed context for each production `Validate()` call site in the ARMOR codebase, including error handling patterns and the role of each call within its execution flow.

---

## Go Validation System - Production Call Sites

### Call Site 1: `internal/yamlutil/schema.go:180`

**Location:** Within `SchemaValidator.Validate(data interface{}) SchemaValidationResult` method

**Code Context:**
```go
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult {
    result := SchemaValidationResult{
        Valid:                true,
        Errors:               []SchemaValidationError{},
        Warnings:             []SchemaValidationError{},
        MissingRequiredFields: []string{},
        TypeMismatches:       []FieldTypeError{},
        ConstraintViolations: []ConstraintViolation{},
    }

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

    // THE CALL SITE:
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
    // ... additional field validation for SchemaDefinition
```

**Purpose:**
- Delegates validation to the underlying schema implementation
- This is the core validation step that validates raw data against schema rules
- Called after schema compilation succeeds

**Error Handling Pattern:**
1. **Dual-path error handling:**
   - If error implements `YAMLError` interface: extracts structured error code
   - If generic error: wraps with "Validation failed:" prefix
2. **Result mutation:** Sets `result.Valid = false` and appends to `result.Errors`
3. **Early return:** Returns immediately after validation failure (no further processing)
4. **No panic:** All errors are handled gracefully, converted to structured results

**Flow Context:**
- Entry point: Public API for validation
- Precondition: Schema compiled (or compile attempted)
- Postcondition: Returns SchemaValidationResult (never panics)
- Caller receives: Structured result with errors array for reporting

**Called From:**
- `ReadAndValidate()` (line 253) - after YAML parsing
- Direct external calls from application code
- Test code

---

### Call Site 2: `internal/yamlutil/schema.go:253`

**Location:** Within `ReadAndValidate(path string) SchemaValidationResult` method

**Code Context:**
```go
func (sv *SchemaValidator) ReadAndValidate(path string) SchemaValidationResult {
    result := SchemaValidationResult{
        Valid:                true,
        Errors:               []SchemaValidationError{},
        Warnings:             []SchemaValidationError{},
        MissingRequiredFields: []string{},
        TypeMismatches:       []FieldTypeError{},
        ConstraintViolations: []ConstraintViolation{},
    }

    // Read YAML file
    content, err := os.ReadFile(path)
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

    // THE CALL SITE:
    return sv.Validate(data)
}
```

**Purpose:**
- Chains into schema validation after successful YAML file read and parse
- Provides convenience API for file-based validation
- Completes the read-parse-validate pipeline

**Error Handling Pattern:**
1. **No direct error handling:** Relies on `sv.Validate()` to handle validation errors
2. **Upstream handling:** File read errors and parse errors handled before this call
3. **Pass-through:** Returns the complete SchemaValidationResult from Validate() unchanged
4. **Pipeline failure points:** 
   - File read failure → early return with error
   - YAML parse failure → early return with error
   - Validation failure → handled by Validate() method

**Flow Context:**
- Entry point: Convenience API for file-based validation
- Pipeline: Read file → Parse YAML → Validate data
- Precondition: File path exists and contains valid YAML syntax
- Postcondition: Returns complete validation result
- Caller receives: Same result structure as direct Validate() call

**Called From:**
- Application code that needs to validate YAML files directly
- CLI tools and utilities
- Test code that operates on file paths

---

## Rust Validation System - Production Call Sites

### Call Site 3: `src/parsers/config.rs:662`

**Location:** Within `ParserConfigBuilder::build(self) -> Result<ParserConfig, String>` method

**Code Context:**
```rust
/// This method validates the configuration before returning it.
///
/// # Returns
///
/// * `Ok(ParserConfig)` - Valid configuration
/// * `Err(String)` - Configuration validation failed
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Purpose:**
- Final validation step in builder pattern
- Ensures configuration is valid before releasing to caller
- Prevents invalid configurations from being used

**Error Handling Pattern:**
1. **`?` operator propagation:** Validation errors propagate up the call stack
2. **Type preservation:** Returns `Result<ParserConfig, String>` with descriptive error messages
3. **Early return:** On validation failure, returns `Err(String)` immediately
4. **No panic:** Validation failures convert to `Err` results

**Flow Context:**
- Entry point: Final step of builder pattern
- Precondition: All builder fields set via builder methods
- Postcondition: Returns validated config or error
- Caller receives: `Result` type that must be handled explicitly

**Called From:**
- Application code constructing parser configurations
- Builder pattern usage sites
- Test code creating test fixtures

**Related Method:**
- `build_unchecked()` (line 668) - bypasses validation for internal use only

---

### Call Site 4: `src/parsers/config.rs:1007`

**Location:** Within `ValidatorConfigBuilder::build(self) -> Result<ValidatorConfig, String>` method

**Code Context:**
```rust
/// This method validates the configuration before returning it.
///
/// # Returns
///
/// * `Ok(ValidatorConfig)` - Valid configuration
/// * `Err(String)` - Configuration validation failed
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Purpose:**
- Final validation step in validator config builder pattern
- Ensures validator configuration is valid before releasing
- Parallel pattern to ParserConfigBuilder (same design)

**Error Handling Pattern:**
1. **`?` operator propagation:** Validation errors propagate up the call stack
2. **Type preservation:** Returns `Result<ValidatorConfig, String>` with descriptive error messages
3. **Early return:** On validation failure, returns `Err(String)` immediately
4. **No panic:** Validation failures convert to `Err` results

**Flow Context:**
- Entry point: Final step of validator builder pattern
- Precondition: All validator builder fields set
- Postcondition: Returns validated validator config or error
- Caller receives: `Result` type that must be handled explicitly

**Called From:**
- Application code constructing validator configurations
- Builder pattern usage sites
- Test code creating validator test fixtures

**Related Method:**
- `build_unchecked()` (line 1013) - bypasses validation for internal use only

---

### Call Site 5: `src/parsers/yaml/parser.rs:121`

**Location:** Within YAML parsing pipeline in parser implementation

**Code Context:**
```rust
// Determine validator based on mode
let validator = if config.strict {
    SyntaxValidator::strict()
} else {
    SyntaxValidator::lenient()
};

// Run syntax validation
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
```

**Purpose:**
- Validates YAML syntax before parsing
- Provides early error detection for malformed YAML
- Supports both strict and lenient validation modes

**Error Handling Pattern:**
1. **Result-based handling:** Returns `ValidationResult` struct (not Result type)
2. **Conditional enhancement:** Only runs enhanced detection if basic validation passes
3. **Error accumulation:** Merges errors from multiple validators
4. **Mutable state:** Updates result.valid flag and extends errors array
5. **No early return:** Continues to collect all validation errors

**Flow Context:**
- Entry point: Syntax validation in YAML parsing pipeline
- Precondition: YAML content loaded as string
- Postcondition: Returns ValidationResult with syntax errors
- Follow-up: Enhanced detection runs if syntax is valid
- Caller receives: ValidationResult that may contain multiple errors

**Called From:**
- YAML parser initialization
- Syntax checking utilities
- Test code validating YAML syntax

**Key Difference:**
- Unlike Go and other Rust sites, this returns a `ValidationResult` struct (not `Result` enum)
- More permissive: collects all errors rather than failing fast

---

## Summary of Error Handling Patterns

### Go (Validate() uppercase):
1. **Dual-path error handling:** YAMLError vs generic error
2. **Result mutation:** Modifies result struct in-place
3. **Early return:** Returns immediately on validation failure
4. **Structured errors:** Captures error codes and messages

### Rust Builder Patterns (validate() lowercase):
1. **`?` operator:** Propagates errors using Rust's try operator
2. **Result types:** Returns `Result<T, String>` for explicit handling
3. **Early return:** `?` operator returns immediately on error
4. **No panics:** All validation failures convert to Err results

### Rust Syntax Validator:
1. **Result struct:** Returns ValidationResult (not Result enum)
2. **Error accumulation:** Collects multiple errors before returning
3. **Conditional enhancement:** Runs additional checks if basic validation passes
4. **Mutable state:** Updates result struct in-place

## Common Patterns Across All Call Sites

1. **All use early return or early exit** - no validation errors are silently ignored
2. **All preserve error information** - errors are wrapped or structured for debugging
3. **All separate validation from business logic** - validation is a distinct step
4. **All provide explicit error types** - callers can distinguish validation failures
5. **No panics in validation paths** - all errors handled gracefully

## Design Observations

1. **Builder pattern ubiquity:** Both Go and Rust use builder patterns with validation at build time
2. **Result-based returns:** All validation returns results/structs rather than throwing exceptions
3. **Schema delegation:** Go delegates to underlying schema implementations
4. **Mode selection:** Rust YAML parser selects validator based on strict/lenient mode
5. **Error accumulation vs fail-fast:** Syntax validator accumulates, others fail fast
