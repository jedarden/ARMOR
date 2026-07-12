# Validate() Call Sites Catalog - ARMOR Rust Codebase

**Generated:** 2026-07-12  
**Bead:** bf-52zl8  
**Total Call Sites Found:** 142

## Executive Summary

This catalog documents all locations in the ARMOR Rust codebase where `validate()` methods are called. The call sites span three main validation systems:

1. **Config Validation** (2 production calls) - ParserConfig and ValidatorConfig builders
2. **Schema Validation** (93 test calls, 2 production calls) - Schema trait implementations
3. **YAML Syntax Validation** (12 production/test calls) - SyntaxValidator and Parser trait

## Categorization by Type

### Production Code (Non-Test)

| File | Line(s) | Type | Context | Return Type |
|------|---------|------|---------|-------------|
| `src/parsers/config.rs` | 662 | Config | ParserConfigBuilder::build() | `Result<(), String>` |
| `src/parsers/config.rs` | 1007 | Config | ValidatorConfigBuilder::build() | `Result<(), String>` |
| `src/parsers/yaml/parser.rs` | 121 | YAML | YamlParser::validate_str() | `ValidationResult` (custom struct) |
| `src/schema.rs` | 660-662 | Schema | CompositeSchema example (doc comment) | N/A (comment) |

### Test Code

| File | Count | Type | Purpose |
|------|-------|------|---------|
| `tests/schema_validation_test.rs` | 93 | Schema | Unit tests for Schema trait |
| `src/parsers/config.rs` | 10 | Config | ParserConfig/ValidatorConfig validation tests |
| `src/parsers/yaml/syntax_validator.rs` | 8 | YAML | SyntaxValidator unit tests |
| `src/parsers/yaml/syntax_detector_tests.rs` | 3 | YAML | SyntaxDetector integration tests |
| `src/schema.rs` | 26 | Schema | Schema trait documentation tests |

## Detailed Inventory

### 1. Config Validation (src/parsers/config.rs)

#### Production Calls

**Line 662** - `ParserConfigBuilder::build()`
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
- **Context:** Builder pattern finalization
- **Error Handling:** Uses `?` operator, propagates `String` error
- **Update Priority:** **LOW** - Already has proper error handling via `?`

**Line 1007** - `ValidatorConfigBuilder::build()`
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```
- **Context:** Builder pattern finalization
- **Error Handling:** Uses `?` operator, propagates `String` error
- **Update Priority:** **LOW** - Already has proper error handling via `?`

#### Test Calls (Lines 1202, 1205, 1216, 1224, 1232, 1297, 1300, 1312, 1320)

All test calls use:
- `assert!(config.validate().is_ok())` - Positive test cases
- `assert!(config.validate().is_err())` - Negative test cases
- **Update Priority:** **NONE** - Test code, no changes needed

### 2. YAML Syntax Validation (src/parsers/yaml/parser.rs)

**Line 121** - `YamlParser::validate_str()`
```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };
    
    let mut result = validator.validate(content);
    // ... merges with detector results
    result
}
```
- **Context:** Public API method, delegates to SyntaxValidator
- **Error Handling:** Returns `ValidationResult` (custom struct with `valid: bool, errors: Vec<ValidationError>`)
- **Update Priority:** **LOW** - Returns structured result type, not `Result`

### 3. Schema Validation (src/schema.rs)

#### Production Calls

**Lines 660-662** - Documentation example (comment only)
```rust
///     UsernameSchema.validate(&user.username)
///     AgeSchema.validate(&user.age)
```
- **Context:** Documentation example in `CompositeSchema` doc comment
- **Status:** Not executable code
- **Update Priority:** **NONE** - Documentation only

#### Test Calls (Lines 318-693)

All test calls follow the pattern:
```rust
assert!(schema.validate(&value).is_ok());  // Positive cases
let result = schema.validate(&invalid);    // Negative cases
```

- **Return Type:** `ValidationResult = Result<(), ParseError>`
- **Update Priority:** **NONE** - Test code only

### 4. YAML Syntax Validator Tests (src/parsers/yaml/syntax_validator.rs)

**Lines 438, 453, 461, 470, 479, 487, 495, 503**

All follow pattern:
```rust
let result = validator.validate(yaml_content);
```

- **Context:** Unit tests for `SyntaxValidator::validate()`
- **Return Type:** `ValidationResult` (custom struct)
- **Update Priority:** **NONE** - Test code only

## Return Type Analysis

### Type 1: `Result<(), String>` (Config validation)
- **Used by:** `ParserConfig::validate()`, `ValidatorConfig::validate()`
- **Current handling:** `?` operator (proper propagation)
- **Update needed:** NO

### Type 2: `ValidationResult = Result<(), ParseError>` (Schema validation)
- **Used by:** Schema trait implementations
- **Current handling:** Test assertions only
- **Update needed:** NO (no production callers)

### Type 3: `ValidationResult` struct (YAML validation)
- **Used by:** `SyntaxValidator::validate()`, `Parser::validate_str()`
- **Structure:** `{ valid: bool, errors: Vec<ValidationError> }`
- **Current handling:** Direct field access (`result.is_valid()`, `result.errors`)
- **Update needed:** NO (not a Result type)

## Priority Recommendations

### HIGH Priority
**None** - All production code has appropriate error handling.

### MEDIUM Priority  
**None** - No deferred or wrapped validation found.

### LOW Priority
**None** - The two production config calls already use `?` operator.

### NO Update Needed
- **Config validation (2 calls):** Already use `?` operator properly
- **YAML syntax validation (1 call):** Returns custom struct, not Result type
- **All test calls (139 calls):** Test code, no production impact

## Summary Statistics

| Category | Count | Percentage |
|----------|-------|------------|
| Test code | 139 | 97.9% |
| Production code | 3 | 2.1% |
| **Total** | **142** | **100%** |

## Conclusion

The ARMOR Rust codebase has excellent validation discipline:

1. **All production `validate()` calls use proper error handling**
   - Config builders use `?` operator (lines 662, 1007)
   - YAML parser returns structured result type (line 121)

2. **No deferred or wrapped validation found**
   - All validation is explicit and immediate
   - No patterns of ignoring validation errors

3. **Test coverage is comprehensive**
   - 139 test calls validate positive and negative cases
   - Only 3 production calls, all properly handled

**Recommendation:** No error handling updates are required. The existing code follows Rust best practices for validation error propagation.

---
**End of Catalog**
