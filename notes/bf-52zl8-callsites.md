# Validate() Call Sites Catalog

**Generated:** 2026-07-12  
**Bead:** bf-52zl8  
**Workspace:** /home/coding/ARMOR (Rust codebase)

## Overview

This catalog documents all `validate()` method call sites in the ARMOR Rust codebase. The codebase contains **two distinct validation interfaces** with different return types:

1. **Schema trait** (`src/schema.rs`) - Returns `Result<(), ParseError>`
2. **Parser trait** (`src/parsers/traits.rs`) - Returns `Result<(), ParseError>`
3. **YAML validators** (`src/parsers/yaml/`) - Returns custom `ValidationResult` struct

---

## Production Code Call Sites (NON-TEST)

These are actual `validate()` calls in production code that may need error handling updates:

### 1. `src/parsers/config.rs:662`

**Location:** `ParserConfigBuilder::build()` method  
**Context:** Builder validation before returning final config

```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Call Type:** Direct method call  
**Error Handling:** ✅ **HAS ERROR HANDLING** - Uses `?` operator  
**Update Priority:** **LOW** - Already properly handles errors  
**Notes:** Validates ParserConfig in builder pattern before returning

---

### 2. `src/parsers/config.rs:1007`

**Location:** `ValidatorConfigBuilder::build()` method  
**Context:** Builder validation before returning final config

```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Call Type:** Direct method call  
**Error Handling:** ✅ **HAS ERROR HANDLING** - Uses `?` operator  
**Update Priority:** **LOW** - Already properly handles errors  
**Notes:** Validates ValidatorConfig in builder pattern before returning

---

### 3. `src/parsers/yaml/parser.rs:121`

**Location:** `YamlParser::validate_str()` method  
**Context:** Delegates to SyntaxValidator for YAML syntax validation

```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };
    
    let mut result = validator.validate(content);
    // ... further processing
}
```

**Call Type:** Delegated method call  
**Error Handling:** ⚠️ **DIFFERENT RETURN TYPE** - Returns custom struct, not `Result`  
**Update Priority:** **N/A** - Uses YAML-specific ValidationResult struct  
**Notes:** This calls `SyntaxValidator::validate()` which returns `ValidationResult` struct (with `valid` bool and `errors` Vec), NOT a `Result` type

---

## Validate() Method Definitions

These are the actual method/trait definitions (not call sites):

### Schema Trait

**Location:** `src/schema.rs:274`

```rust
pub trait Schema<T: ?Sized> {
    fn validate(&self, value: &T) -> ValidationResult;
}
```

**Return Type:** `ValidationResult = Result<(), ParseError>`  
**Implementations:** Multiple schema structs throughout `src/schema.rs`

---

### Parser Trait

**Location:** `src/parsers/traits.rs:323`

```rust
pub trait Parser<Input> {
    fn validate(&self, source: Input) -> Result<(), ParseError> {
        self.parse(source)?;
        Ok(())
    }
}
```

**Return Type:** `Result<(), ParseError>`  
**Default Implementation:** Calls `parse()` and discards result

---

### ParserConfig::validate()

**Location:** `src/parsers/config.rs:537`

```rust
impl ParserConfig {
    pub fn validate(&self) -> Result<(), String> {
        // Validation logic
    }
}
```

**Return Type:** `Result<(), String>`  
**Used by:** `ParserConfigBuilder::build()` at line 662

---

### ValidatorConfig::validate()

**Location:** `src/parsers/config.rs:908`

```rust
impl ValidatorConfig {
    pub fn validate(&self) -> Result<(), String> {
        // Validation logic
    }
}
```

**Return Type:** `Result<(), String>`  
**Used by:** `ValidatorConfigBuilder::build()` at line 1007

---

### SyntaxValidator::validate()

**Location:** `src/parsers/yaml/syntax_validator.rs:65`

```rust
impl SyntaxValidator {
    pub fn validate(&self, content: &str) -> ValidationResult {
        // Returns custom struct with valid/fields/errors
    }
}
```

**Return Type:** `ValidationResult` (custom struct from `src/parsers/yaml/types.rs`)  
**Used by:** `YamlParser::validate_str()` at line 121

---

## Test Code Call Sites (REFERENCE ONLY)

These validate() calls are in test code and do NOT need updates for production error handling:

### `src/parsers/config.rs` - Tests
- Lines 1202, 1205, 1216, 1224, 1232, 1297, 1300, 1312, 1320
- All use `assert!(config.validate().is_ok())` or `assert!(config.validate().is_err())`

### `src/schema.rs` - Doc Examples & Tests
- Lines 318, 319, 322, 326, 352, 353, 354, 357, 358, 431, 433, 436, 440, 462, 463, 466, 505, 511, 518, 527, 551, 552, 555, 560, 564, 602, 603, 604, 605, 606, 610, 611, 612, 613, 614, 660, 662, 675, 682, 693

### `src/parsers/yaml/syntax_validator.rs` - Tests
- Lines 438, 453, 461, 470, 479, 487, 495, 503

### `src/parsers/yaml/syntax_detector_tests.rs` - Tests
- Lines 131, 567, 728

### `tests/schema_validation_test.rs` - Comprehensive Tests
- Over 100 test calls throughout the file

---

## Categorization Summary

### By Call Type

| Category | Count | Description |
|----------|-------|-------------|
| **Direct Production Calls** | 2 | Lines 662, 1007 in config.rs |
| **Delegated Calls** | 1 | Line 121 in yaml/parser.rs |
| **Test/Doc Calls** | 100+ | All test files and doc examples |
| **Method Definitions** | 5 | Trait/impl definitions |

### By Update Priority

| Priority | Count | Locations |
|----------|-------|-----------|
| **HIGH** | 0 | No production sites lacking error handling |
| **MEDIUM** | 0 | No wrapped/deferred sites needing attention |
| **LOW** | 2 | Lines 662, 1007 (already have proper error handling) |
| **N/A** | 1 | Line 121 (uses different ValidationResult type) |

---

## Key Findings

1. **✅ All production validate() calls already have proper error handling**
   - Both builder pattern calls use `?` operator correctly
   - No missing error handling in production code

2. **⚠️ Two different validation result types exist**
   - `ValidationResult = Result<(), ParseError>` in schema.rs
   - `ValidationResult` struct in yaml/types.rs
   - These are NOT interchangeable - different call semantics

3. **🔧 No critical error handling gaps identified**
   - All production code properly handles validate() results
   - Test code uses appropriate assert macros

4. **📝 Parser trait provides default validate() implementation**
   - Delegates to `parse()` and discards result
   - No direct calls found in production code

---

## Recommendations

### No Action Required
All production `validate()` call sites already have appropriate error handling. The codebase is in good shape regarding validation error propagation.

### Future Considerations
- If adding new validate() calls, follow existing patterns:
  - Use `?` operator for Result-returning validate()
  - Handle ValidationResult struct appropriately for YAML validation
  - Prefer builder pattern with build() validation for configs

### Type Clarity
Consider renaming one of the ValidationResult types to avoid confusion:
- Keep `ValidationResult = Result<(), ParseError>` in schema.rs
- Rename `ValidationResult` struct in yaml/types.rs to something like `YamlValidationResult` or `SyntaxValidationResult`

---

## End of Catalog

**Total validate() call sites documented:** 3 production, 5 definitions, 100+ tests  
**Sites needing error handling updates:** 0  
**Sites needing review:** 0 (all properly handled)
