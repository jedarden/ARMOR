# Catalog of Validate() Call Sites in ARMOR

**Bead:** bf-52zl8  
**Date:** 2026-07-12  
**Total Call Sites Found:** 142 (138 tests + 4 production)

## Overview

This document catalogs all locations where `validate()` methods are called in the ARMOR codebase. The codebase contains **multiple different `validate()` methods** with different signatures and return types, which is important for understanding error handling requirements.

## Validate() Method Signatures

| Type | Method Signature | Return Type | Defined In |
|------|------------------|-------------|------------|
| `ParserConfig` | `fn validate(&self) -> ...` | `Result<(), String>` | `src/parsers/config.rs:537` |
| `ValidatorConfig` | `fn validate(&self) -> ...` | `Result<(), String>` | `src/parsers/config.rs:908` |
| `ValidationHook` | `fn validate(&self, field: &str, value: &serde_yaml::Value) -> ...` | `Result<(), String>` | `src/parsers/config.rs:255` |
| `Schema<T>` | `fn validate(&self, value: &T) -> ...` | `ValidationResult` | `src/schema.rs:274` |
| `SyntaxValidator` | `fn validate(&self, content: &str) -> ...` | `ValidationResult` | `src/parsers/yaml/syntax_validator.rs:65` |
| `Parser` trait | `fn validate(&self, source: Input) -> ...` | `Result<(), ParseError>` | `src/parsers/traits.rs:323` |

**Note:** `ValidationResult` is an alias for `Result<(), ParseError>`.

---

## Production Call Sites (4 total)

These are the **non-test** call sites that need review for error handling.

### 1. `src/parsers/config.rs:662`
**Method:** `ParserConfig::validate()`  
**Context:** Inside `ParserConfigBuilder::build()` method  
**Code:**
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
**Error Handling:** ✅ **PROPER** - Uses `?` operator to propagate errors  
**Update Priority:** **NONE** - Already correct

---

### 2. `src/parsers/config.rs:1007`
**Method:** `ValidatorConfig::validate()`  
**Context:** Inside `ValidatorConfigBuilder::build()` method  
**Code:**
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
**Error Handling:** ✅ **PROPER** - Uses `?` operator to propagate errors  
**Update Priority:** **NONE** - Already correct

---

### 3. `src/parsers/yaml/parser.rs:121`
**Method:** `SyntaxValidator::validate()`  
**Context:** Inside `YamlParser::validate_str()` method  
**Code:**
```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };
    
    let mut result = validator.validate(content);
    
    // Uses result.is_valid() to check status
    if result.is_valid() {
        // ... more validation
    }
    result
}
```
**Error Handling:** ✅ **PROPER** - Returns `ValidationResult` directly, caller checks validity  
**Update Priority:** **NONE** - Already correct

---

### 4. `src/parsers/config.rs:255` (Method Definition)
**Method:** `ValidationHook::validate()`  
**Context:** This is the method **definition**, not a call site  
**Update Priority:** **N/A** - Not a call site

---

## Test Call Sites (138 total)

All test call sites are categorized below. These **do not need error handling updates** as they are in test code.

### `tests/schema_validation_test.rs` (91 calls)
All test assertions and result captures for `Schema<T>::validate()`.  
**Pattern:** `assert!(schema.validate(&value).is_ok())` or `let result = schema.validate(&value)`

### `src/schema.rs` (doctests and unit tests) (35 calls)
All doctest examples and unit test assertions.  
**Pattern:** Documentation examples and test assertions

### `src/parsers/yaml/syntax_validator.rs` (7 calls)
All unit test calls to `SyntaxValidator::validate()`.  
**Pattern:** `let result = validator.validate(yaml);`

### `src/parsers/yaml/syntax_detector_tests.rs` (3 calls)
All integration test calls.  
**Pattern:** `let result = validator.validate(yaml);`

### `src/parsers/config.rs` (2 calls)
Test assertions in `#[cfg(test)]` modules.  
**Pattern:** `assert!(config.validate().is_ok())` or `assert!(config.validate().is_err())`

---

## Summary by Category

| Category | Count | Update Required |
|----------|-------|-----------------|
| Production with `?` operator | 2 | ✅ No - Already correct |
| Production returning `ValidationResult` | 1 | ✅ No - Already correct |
| Method definitions (not call sites) | 1 | N/A |
| Test code | 138 | ❌ No - Tests exempt |

---

## Key Findings

### ✅ Good News
- **ALL** production `validate()` call sites already have **proper error handling**
- No call sites need error handling updates
- The two `Result<(), String>` return type call sites both use `?` operator correctly
- The `ValidationResult` call site properly returns the result for caller to check

### Types of validate() Methods

1. **Self-validating configs** (`ParserConfig`, `ValidatorConfig`): Return `Result<(), String>` and check internal consistency
2. **Schema validation** (`Schema<T>`): Validates data against schema rules, returns `ValidationResult`
3. **Syntax validation** (`SyntaxValidator`): Validates YAML syntax, returns `ValidationResult`
4. **Parser validation** (`Parser` trait): Validates input can be parsed, returns `Result<(), ParseError>`
5. **Field validation** (`ValidationHook`): Validates specific fields, returns `Result<(), String>`

### Error Handling Patterns

| Pattern | Used By | Status |
|---------|---------|--------|
| `result?` | `ParserConfig`, `ValidatorConfig` | ✅ Correct |
| Return `ValidationResult` | `SyntaxValidator` | ✅ Correct |
| `assert!(result.is_ok())` | Tests | ✅ Correct for tests |
| `let result = ...` | Tests | ✅ Correct for tests |

---

## Recommendation

**NO ERROR HANDLING UPDATES NEEDED**

All 4 production call sites already handle errors correctly:
- 2 use the `?` operator to propagate `Result<(), String>` errors
- 1 properly returns `ValidationResult` for the caller to inspect
- 1 is a method definition (not a call site)

The remaining 138 call sites are all in test code and are exempt from error handling requirements.

---

## Appendix: Full Call Site List

### Production Code (Non-Test)
1. `src/parsers/config.rs:662` - `ParserConfigBuilder::build()` - `self.config.validate()?`
2. `src/parsers/config.rs:1007` - `ValidatorConfigBuilder::build()` - `self.config.validate()?`
3. `src/parsers/yaml/parser.rs:121` - `YamlParser::validate_str()` - `let mut result = validator.validate(content)`

### Test Code (138 total)
- `tests/schema_validation_test.rs` - 91 calls (lines 148-647)
- `src/schema.rs` - 35 calls (doctests and unit tests, lines 145-693)
- `src/parsers/yaml/syntax_validator.rs` - 7 calls (lines 438-503)
- `src/parsers/yaml/syntax_detector_tests.rs` - 3 calls (lines 131, 567, 728)
- `src/parsers/config.rs` - 2 calls (test assertions, lines 1202-1320)

---

**End of Catalog**
