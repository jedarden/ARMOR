# Validate() Call Sites Catalog

**Bead:** bf-52zl8  
**Date:** 2026-07-12  
**Purpose:** Catalog all Validate() call sites in ARMOR codebase for error handling updates

---

## Summary

- **Total Production Call Sites:** 3
- **Total Test Call Sites:** ~60 (across multiple test files)
- **Total Doctest/Documentation Sites:** ~15 (in Schema trait docs)

---

## Production Call Sites

### 1. ParserConfigBuilder::build()

**Location:** `src/parsers/config.rs:662`

**Context:**
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Caller Type:** Direct - Builder pattern completion method

**Error Handling:**
- Uses `?` operator to propagate validation errors
- Returns `Result<ParserConfig, String>`
- Error type: `String` (from `ParserConfig::validate()`)

**Implementation Being Called:** `ParserConfig::validate()` (line 537)
```rust
pub fn validate(&self) -> Result<(), String> {
    // Check for mutually exclusive or inconsistent options
    if self.warnings_as_errors && !self.emit_warnings {
        return Err("warnings_as_errors requires emit_warnings to be true".to_string());
    }
    if self.mode.is_strict() && self.allow_duplicates {
        return Err("Strict mode with allow_duplicates=true is inconsistent".to_string());
    }
    if self.strict_types && self.mode.is_lenient() {
        return Err("strict_types=true with lenient mode is inconsistent".to_string());
    }
    Ok(())
}
```

**Update Priority:** MEDIUM
- ✅ Already has proper error handling with `?` operator
- ⚠️ Returns generic `String` errors instead of typed `ParseError`
- 💡 Could benefit from rich error context (line numbers, file paths)

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

**Caller Type:** Direct - Builder pattern completion method

**Error Handling:**
- Uses `?` operator to propagate validation errors
- Returns `Result<ValidatorConfig, String>`
- Error type: `String` (from `ValidatorConfig::validate()`)

**Implementation Being Called:** `ValidatorConfig::validate()` (line 908)
```rust
pub fn validate(&self) -> Result<(), String> {
    if self.mode.is_strict() && !self.require_all_fields {
        return Err("Strict mode requires require_all_fields to be true".to_string());
    }
    if self.mode.is_strict() && !self.disallow_unknown_fields {
        return Err("Strict mode requires disallow_unknown_fields to be true".to_string());
    }
    if self.warnings_as_errors && !self.emit_warnings {
        return Err("warnings_as_errors requires emit_warnings to be true".to_string());
    }
    Ok(())
}
```

**Update Priority:** MEDIUM
- ✅ Already has proper error handling with `?` operator
- ⚠️ Returns generic `String` errors instead of typed `ParseError`
- 💡 Could benefit from rich error context

---

### 3. YamlParser::validate_str()

**Location:** `src/parsers/yaml/parser.rs:121`

**Context:**
```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    // Create syntax validator based on parser mode
    let validator = if self.config.is_strict() {
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

    result
}
```

**Caller Type:** Wrapped - Validates then merges with additional detection

**Error Handling:**
- Stores `ValidationResult` directly (no early return)
- Checks `is_valid()` before running additional validation
- Merges errors from multiple sources (syntax validator + detector)
- Returns: `ValidationResult` (custom struct with `valid`, `errors`, `warnings`)

**Implementation Being Called:** `SyntaxValidator::validate()` (src/parsers/yaml/syntax_validator.rs:65)
```rust
pub fn validate(&self, content: &str) -> ValidationResult {
    let mut errors = Vec::new();
    let mut warnings = Vec::new();
    
    // ... validation logic ...
    
    ValidationResult {
        valid: errors.is_empty(),
        errors,
        warnings,
    }
}
```

**Update Priority:** LOW
- ✅ Already has sophisticated error handling with merge logic
- ✅ Returns rich `ValidationResult` with structured errors
- ✅ No improvements needed

---

## Test Call Sites

### SyntaxValidator Unit Tests

**Location:** `src/parsers/yaml/syntax_validator.rs:438-503`

**Test Count:** 8 tests

**Tests:**
1. `test_validate_empty_content` (line 438)
2. `test_validate_simple_valid_yaml` (line 453)
3. `test_detect_tabs_in_strict_mode` (line 461)
4. `test_detect_mixed_indentation` (line 470)
5. `test_detect_unmatched_brace` (line 479)
6. `test_detect_unmatched_bracket` (line 487)
7. `test_invalid_block_scalar_indicator` (line 495)
8. `test_anchor_without_name` (line 503)

**Update Priority:** NONE - Tests are working as intended

---

### Schema Validation Tests

**Location:** `tests/schema_validation_test.rs`

**Test Count:** ~40 tests

**Coverage:** All Schema trait implementations

**Update Priority:** NONE - Tests are working as intended

---

## Documentation/Doctest Sites

**Location:** `src/schema.rs` (multiple lines)

**Purpose:** Examples in Schema trait docstrings

**Count:** ~15 examples

**Update Priority:** NONE - Documentation should remain as-is

---

## Categorization Summary

### By Call Type

| Type | Count | Sites |
|------|-------|-------|
| **Direct** | 2 | ParserConfigBuilder::build(), ValidatorConfigBuilder::build() |
| **Wrapped** | 1 | YamlParser::validate_str() |
| **Deferred** | 0 | None found |

### By Update Priority

| Priority | Count | Sites |
|----------|-------|-------|
| **HIGH** | 0 | None |
| **MEDIUM** | 2 | ParserConfigBuilder::build(), ValidatorConfigBuilder::build() |
| **LOW** | 1 | YamlParser::validate_str() |
| **NONE** | ~55 | All test and doc sites |

---

## Recommendations

### Immediate Actions

1. **ParserConfig::validate() and ValidatorConfig::validate()**
   - Consider changing return type from `Result<(), String>` to `Result<(), ParseError>`
   - This would provide rich error context (line numbers, file paths, code snippets)
   - Aligns with the rest of the codebase which uses `ParseError`

2. **Builder Pattern Consistency**
   - Current implementation is correct but could be enhanced
   - Consider adding `with_context()` methods to builders for better error messages

### No Changes Needed

- **YamlParser::validate_str()** - Already has excellent error handling with merge logic
- **All test sites** - Tests are comprehensive and working correctly
- **Documentation sites** - Examples are clear and helpful

---

## Related Files

| File | Lines | Purpose |
|------|-------|---------|
| `src/parsers/config.rs` | 537-554, 908-923 | Config validation implementations |
| `src/parsers/yaml/parser.rs` | 112-136 | YAML parser validation orchestration |
| `src/parsers/yaml/syntax_validator.rs` | 65-108 | Syntax validation implementation |
| `src/schema.rs` | 108-274 | Schema trait definition and examples |
| `tests/schema_validation_test.rs` | All | Comprehensive Schema tests |

---

## Notes

- All production call sites already have error handling (no silent failures)
- The main improvement opportunity is in error type consistency (String → ParseError)
- No deferred validation patterns found - all validation happens at call time
