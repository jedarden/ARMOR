# Validate() Call Sites Documentation

## Document Purpose
This document catalogs all `Validate()` call site locations found in the ARMOR Rust codebase. Created as part of bead **bf-cdc05** to provide structured documentation of validation method usage patterns.

## Bead Relationship
- **bf-2c889**: Initial search for Validate() call sites (focused on `src/parsers/config.rs`)
- **bf-52zl8**: Comprehensive Validate() catalog (entire codebase, all validate() methods)
- **bf-cdc05**: Documentation task to structure and present the findings

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
**Type:** Direct call with proper error handling
**Return Type:** `Result<(), String>`
**Error Handling:** ✅ Uses `?` operator to propagate errors
**Priority:** LOW - Already has correct error handling

---

### 2. ValidatorConfigBuilder::build()
**Location:** `src/parsers/config.rs:1007`
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
**Type:** Direct call with proper error handling
**Return Type:** `Result<(), String>`
**Error Handling:** ✅ Uses `?` operator to propagate errors
**Priority:** LOW - Already has correct error handling

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
**Type:** Direct call with manual result inspection
**Return Type:** `ValidationResult` (struct)
**Error Handling:** ⚠️ Does NOT use `?` - stores result and manually checks `is_valid()`
**Priority:** LOW - Intentionally uses result struct directly (not a `Result` type)

---

## Test Code Call Sites

All other `validate()` calls are in test code and use assertion patterns:

### Config Tests (`src/parsers/config.rs`)
- Lines 1202, 1205, 1216, 1224, 1232, 1297, 1300, 1312, 1320
- Pattern: `assert!(config.validate().is_ok())` or `assert!(config.validate().is_err())`

### Schema Tests (`src/schema.rs`)
- Lines 145, 146, 216, 218, 270-272, 318-326, 352-358, 431-440, 462-466, 505, 511, 518, 527, 551-552, 555, 560, 564, 602-614, 660, 662, 675, 682, 693
- Pattern: `assert!(schema.validate(&value).is_ok())` or `is_err()` assertions

### YAML Validator Tests (`src/parsers/yaml/syntax_validator.rs`)
- Lines 438, 453, 461, 470, 479, 487, 495, 503
- Pattern: `let result = validator.validate(yaml)`

### Syntax Detector Tests (`src/parsers/yaml/syntax_detector_tests.rs`)
- Lines 131, 567, 728
- Pattern: `let result = validator.validate(yaml)`

### Schema Validation Tests (`tests/schema_validation_test.rs`)
- Lines 148, 150, 180-264, 276, 282, 290-292, 304-575, 583, 586, 590, 621, 631, 647
- Pattern: `assert!(schema.validate(&value).is_ok())` or `let result = schema.validate(&value)`

---

## Summary by Category

### Direct Calls with Proper Error Handling (Production)
1. `ParserConfigBuilder::build()` - Line 662 ✅
2. `ValidatorConfigBuilder::build()` - Line 1007 ✅

### Direct Calls with Manual Result Inspection (Production)
1. `YamlParser::validate_str()` - Line 121 ⚠️ (uses struct, not Result)

### Test Calls (Assertion Pattern)
- ~80+ calls across test files using `assert!(...is_ok())` or `assert!(...is_err())`

### Test Calls (Variable Assignment)
- ~20 calls across test files storing result for later inspection

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
**Current Bead:** bf-cdc05 (documentation task)
**Related Bead:** bf-2c889 (initial Validate() search in config.rs)
