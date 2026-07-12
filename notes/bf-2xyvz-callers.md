# Validate() Callers Catalog

**Bead:** bf-1x2wu  
**Created:** 2026-07-12  
**Purpose:** Catalog all direct `validate()` method callers for YAMLError handling updates

---

## Summary

Total production (non-test) callers found: **3**

- **2 callers** return `Result<(), String>` - Need YAMLError handling updates
- **1 caller** returns `ValidationResult` - Different pattern, may not need updates

---

## Production Callers

### 1. ParserConfigBuilder::build()

**Location:** `src/parsers/config.rs:662`

**Context:**
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Method Called:**
```rust
impl ParserConfig {
    pub fn validate(&self) -> Result<(), String> {
        // Validates configuration consistency
        // Returns Err(String) on validation failure
    }
}
```

**Current Error Handling:**
- Uses `?` operator to propagate `String` error
- Error type: `Result<(), String>`

**YAMLError Update Required:** **YES**
- Should convert any YAML-specific validation errors to `YAMLError`
- Need to determine if validation can produce YAML parsing errors

---

### 2. ValidatorConfigBuilder::build()

**Location:** `src/parsers/config.rs:1007`

**Context:**
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Method Called:**
```rust
impl ValidatorConfig {
    pub fn validate(&self) -> Result<(), String> {
        // Validates mutually exclusive options
        // Returns Err(String) on validation failure
    }
}
```

**Current Error Handling:**
- Uses `?` operator to propagate `String` error
- Error type: `Result<(), String>`

**YAMLError Update Required:** **YES**
- Should convert any YAML-specific validation errors to `YAMLError`
- Need to determine if validation can produce YAML parsing errors

---

### 3. BasicParser::validate_str()

**Location:** `src/parsers/yaml/parser.rs:121`

**Context:**
```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };

    let mut result = validator.validate(content);
    
    // Merge with SyntaxDetector results
    if result.is_valid() {
        let mut detector = SyntaxDetector::new();
        let detector_result = detector.detect_to_validation_result(content);
        // ... merge logic
    }
    
    result
}
```

**Method Called:**
```rust
impl SyntaxValidator {
    pub fn validate(&self, content: &str) -> ValidationResult {
        // Returns ValidationResult with errors/warnings
    }
}
```

**Current Error Handling:**
- No error propagation (returns `ValidationResult` directly)
- Error type: `ValidationResult` (custom struct, not `Result`)

**YAMLError Update Required:** **MAYBE**
- Different pattern: returns `ValidationResult` instead of `Result`
- Need to check if `ValidationResult` should incorporate `YAMLError`
- This is YAML syntax validation, more likely to need YAMLError integration

---

## Excluded Callers (Test Code)

All other `validate()` calls are in:
- `tests/schema_validation_test.rs` - Comprehensive test suite
- `src/parsers/config.rs` test functions (lines 1202-1320)
- `src/parsers/yaml/syntax_validator.rs` test functions
- `src/parsers/yaml/syntax_detector_tests.rs`
- Doc comment examples in `src/schema.rs`

**Total test/excluded callers:** 100+ (all examples and unit tests)

---

## Next Steps for YAMLError Integration

### Priority 1: Investigate Validation Sources
- Determine if `ParserConfig::validate()` and `ValidatorConfig::validate()` can produce YAML parsing errors
- Check if validation logic involves YAML parsing internally

### Priority 2: SyntaxValidator Integration
- `BasicParser::validate_str()` is the most likely candidate for YAMLError
- Consider converting `ValidationResult` to include `YAMLError` variants
- Or create a parallel `validate_with_yaml_error()` method

### Priority 3: Update Builder Methods
- If YAMLError is needed, update `ParserConfigBuilder::build()` and `ValidatorConfigBuilder::build()` signatures
- Change return types from `Result<T, String>` to `Result<T, YAMLError>` (or use a custom error enum)

---

## Related Files

- `src/parsers/config.rs` - Configuration validation (lines 537, 908)
- `src/parsers/yaml/parser.rs` - Parser trait implementation (line 121)
- `src/parsers/yaml/syntax_validator.rs` - SyntaxValidator::validate (line 65)
- `src/parsers/traits.rs` - Parser trait definition

---

**Status:** Catalog complete, ready for systematic YAMLError integration
