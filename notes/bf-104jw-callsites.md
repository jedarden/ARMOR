# Validate() Call Sites Documentation

## Document Purpose
This document catalogs all `Validate()` call site locations found in the ARMOR Rust codebase. Created as part of bead **bf-cdc05** to provide structured documentation of validation method usage patterns.

## Bead Relationship
- **bf-2c889**: Initial search for Validate() call sites (focused on `src/parsers/config.rs`)
- **bf-52zl8**: Comprehensive Validate() catalog (entire codebase, all validate() methods)
- **bf-cdc05**: Documentation task to structure and present the findings
- **bf-4y58v**: Analyze Validate() callers for error handling patterns
- **bf-3bqt8**: Categorize Validate() callers by type (direct, wrapped, deferred)

## Overview
This document catalogs all locations in the ARMOR codebase where `validate()` methods are called. The codebase has two distinct `validate()` method families:

1. **Config validate()** - Returns `Result<(), String>`
2. **Schema/YAML validate()** - Returns `ValidationResult` (struct with `valid: bool` field)

## Production Code Call Sites

### 1. ParserConfigBuilder::build()
**Location:** `src/parsers/config.rs:662`
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
**Call Type:** Direct call
**Return Type:** `Result<(), String>`
**Error Handling:** ✅ Uses `?` operator to propagate errors
**Priority:** LOW - Already has correct error handling
**Categorization:** Direct call - validate() is invoked immediately in the build() method flow

---

### 2. ValidatorConfigBuilder::build()
**Location:** `src/parsers/config.rs:1007`
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
**Call Type:** Direct call
**Return Type:** `Result<(), String>`
**Error Handling:** ✅ Uses `?` operator to propagate errors
**Priority:** LOW - Already has correct error handling
**Categorization:** Direct call - validate() is invoked immediately in the build() method flow

---

### 3. YamlParser::validate_str()
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
**Category:** Wrapped call
**Return Type:** `ValidationResult` (struct)
**Error Handling:** ⚠️ Does NOT use `?` - stores result and manually checks `is_valid()`
**Priority:** LOW - Intentionally uses result struct directly (not a `Result` type)
**Categorization:** Wrapped call - validate() is wrapped within validate_str() method that combines basic validation with enhanced syntax detection and result merging
**Wrapper functionality:**
- Encapsulates validator selection (strict vs lenient)
- Combines basic validation with enhanced syntax detection
- Merges errors from both validation sources
- Provides unified ValidationResult interface

---

## Test Code Call Sites

All other `validate()` calls are in test code and use assertion patterns:

### Config Tests (`src/parsers/config.rs`)
- Lines 1202, 1205, 1216, 1224, 1232, 1297, 1300, 1312, 1320
- Pattern: `assert!(config.validate().is_ok())` or `assert!(config.validate().is_err())`
- **Call Type:** Direct call - validate() is called directly within assertion expressions

### Schema Tests (`src/schema.rs`)
- Lines 145, 146, 216, 218, 270-272, 318-326, 352-358, 431-440, 462-466, 505, 511, 518, 527, 551-552, 555, 560, 564, 602-614, 660, 662, 675, 682, 693
- Pattern: `assert!(schema.validate(&value).is_ok())` or `is_err()` assertions
- **Call Type:** Direct call - validate() is called directly within assertion expressions

### YAML Validator Tests (`src/parsers/yaml/syntax_validator.rs`)
- Lines 438, 453, 461, 470, 479, 487, 495, 503
- Pattern: `let result = validator.validate(yaml)`
- **Call Type:** Direct call - validate() is called directly and result stored for later inspection

### Syntax Detector Tests (`src/parsers/yaml/syntax_detector_tests.rs`)
- Lines 131, 567, 728
- Pattern: `let result = validator.validate(yaml)`
- **Call Type:** Direct call - validate() is called directly and result stored for later inspection

### Schema Validation Tests (`tests/schema_validation_test.rs`)
- Lines 148, 150, 180-264, 276, 282, 290-292, 304-575, 583, 586, 590, 621, 631, 647
- Pattern: `assert!(schema.validate(&value).is_ok())` or `let result = schema.validate(&value)`
- **Call Type:** Direct call - validate() is called directly within assertions or stored for inspection

---

## Summary by Call Type

### Production Code Call Sites (3 total)

#### Direct Calls (2)
validate() is invoked immediately in the execution flow:

1. **ParserConfigBuilder::build()** - Line 662
   - Call Type: Direct call
   - Pattern: `self.config.validate()?`
   - Error Handling: ✅ Proper with `?` operator

2. **ValidatorConfigBuilder::build()** - Line 1007
   - Call Type: Direct call
   - Pattern: `self.config.validate()?`
   - Error Handling: ✅ Proper with `?` operator

#### Wrapped Calls (1)
validate() is encapsulated within a higher-level method that provides additional functionality:

1. **YamlParser::validate_str()** - Line 121
   - Call Type: Wrapped call
   - Pattern: `validator.validate(content)` then manual inspection
   - Error Handling: ⚠️ Manual (intentionally uses ValidationResult struct)
   - Wrapper functionality: Combines basic validation with enhanced syntax detection, merges results

#### Deferred Calls (0)
No deferred calls found - no validate() calls are deferred via closures, futures, or lazy evaluation.

### Test Code Call Sites (100+ total)

All test validate() calls are **direct calls**:

#### Config Tests (10 calls)
- Lines 1202, 1205, 1216, 1224, 1232, 1297, 1300, 1312, 1320
- Call Type: Direct call within assertion expressions
- Pattern: `assert!(config.validate().is_ok())` or `assert!(config.validate().is_err())`

#### Schema Tests (~45 calls)
- Lines 145, 146, 216, 218, 270-272, 318-326, 352-358, 431-440, 462-466, 505, 511, 518, 527, 551-552, 555, 560, 564, 602-614, 660, 662, 675, 682, 693
- Call Type: Direct call within assertion expressions
- Pattern: `assert!(schema.validate(&value).is_ok())` or `is_err()` assertions

#### YAML Validator Tests (8 calls)
- Lines 438, 453, 461, 470, 479, 487, 495, 503
- Call Type: Direct call with variable assignment
- Pattern: `let result = validator.validate(yaml)`

#### Syntax Detector Tests (3 calls)
- Lines 131, 567, 728
- Call Type: Direct call with variable assignment
- Pattern: `let result = validator.validate(yaml)`

#### Schema Validation Tests (~40 calls)
- Lines 148, 150, 180-264, 276, 282, 290-292, 304-575, 583, 586, 590, 621, 631, 647
- Call Type: Direct call within assertions or with variable assignment
- Pattern: `assert!(schema.validate(&value).is_ok())` or `let result = schema.validate(&value)`

---

## Summary Statistics by Call Type

### Production Code
- **Direct Calls:** 2 (66.7%)
- **Wrapped Calls:** 1 (33.3%)
- **Deferred Calls:** 0 (0%)
- **Total:** 3 call sites

### Test Code
- **Direct Calls:** 100+ (100%)
- **Wrapped Calls:** 0 (0%)
- **Deferred Calls:** 0 (0%)
- **Total:** 100+ call sites

### Overall Codebase
- **Direct Calls:** 102+ (99%)
- **Wrapped Calls:** 1 (1%)
- **Deferred Calls:** 0 (0%)
- **Total:** 103+ call sites

### Key Findings
1. **Mixed call patterns in production** - 2 direct calls and 1 wrapped call in production code
2. **Single wrapped call** - YamlParser::validate_str() is the only wrapper, providing enhanced validation by combining basic validation with syntax detection
3. **No deferred calls** - No lazy evaluation, closures, or async deferral of validation anywhere in codebase
4. **Consistent test patterns** - All test calls are direct assertions or variable assignments
5. **Test coverage** - Extensive test coverage with 100+ direct calls in test code
6. **Error handling** - All production calls have appropriate error handling for their context

---

## Analysis

### No Error Handling Issues Found
All production code call sites already have appropriate error handling:

1. **Builder pattern calls** - Both use the `?` operator correctly to propagate `Result<(), String>` errors
2. **YAML parser call** - Intentionally uses the `ValidationResult` struct directly, not a `Result` type, so `?` is not applicable

### No Systematic Updates Needed
Based on this catalog:
- **Priority 1 (Critical)**: 0 sites
- **Priority 2 (Important)**: 0 sites
- **Priority 3 (Nice to have)**: 0 sites

All validate() call sites in production code already handle errors appropriately. The test code patterns are also correct for their context (assertions and variable inspection).

---

## Method Signatures

### Config::validate()
```rust
// ParserConfig::validate()
impl ParserConfig {
    pub fn validate(&self) -> Result<(), String> {
        // Checks for mutually exclusive/inconsistent options
        if self.warnings_as_errors && !self.emit_warnings {
            return Err("warnings_as_errors requires emit_warnings to be true".to_string());
        }
        // ...
    }
}

// ValidatorConfig::validate()
impl ValidatorConfig {
    pub fn validate(&self) -> Result<(), String> {
        // Checks for mutually exclusive options
        if self.mode.is_strict() && !self.require_all_fields {
            return Err("Strict mode requires require_all_fields to be true".to_string());
        }
        // ...
    }
}
```

### SyntaxValidator::validate()
```rust
impl SyntaxValidator {
    pub fn validate(&self, content: &str) -> ValidationResult {
        // Returns ValidationResult struct, not Result
        ValidationResult {
            valid: errors.is_empty(),
            errors,
            warnings,
        }
    }
}
```

### Schema trait::validate()
```rust
pub trait Schema<T: ?Sized> {
    fn validate(&self, value: &T) -> ValidationResult;
}

// ValidationResult is Result<(), ParseError>
pub type ValidationResult = Result<(), ParseError>;
```

---

## Generated Information

**Tool:** `rg` (ripgrep)
**Search Patterns:**
- `\.validate\(` - Find all validate() method calls
- `fn validate` - Find all validate() method definitions

**Date Generated:** 2026-07-12
**Original Bead:** bf-52zl8 (comprehensive Validate() catalog)
**Documentation Bead:** bf-cdc05 (documentation task)
**Analysis Bead:** bf-4y58v (error handling analysis)
**Categorization Bead:** bf-3bqt8 (call type categorization)
**Related Bead:** bf-2c889 (initial Validate() search in config.rs)
