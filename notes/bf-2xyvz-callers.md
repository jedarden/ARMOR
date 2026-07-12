# Direct Validate() Callers Catalog

## Overview
This document catalogs all direct `.Validate()` method callers in the ARMOR codebase that may need YAMLError handling updates.

## Search Methodology
- Searched for `.Validate()` and `.validate()` patterns
- Excluded test files and test code
- Focused on production code paths
- Context: -3 lines before and after each match

## Findings Summary
**Total Direct Callers: 3**

### 1. ParserConfigBuilder::build() - Line 662
**File:** `src/parsers/config.rs`  
**Line:** 662  
**Function:** `ParserConfigBuilder::build()`  
**Code:**
```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Context:**
- Located in builder pattern implementation
- Calls `ParserConfig::validate()` method
- Returns `Result<ParserConfig, String>`
- Current error handling: Uses `?` operator, propagates `String` errors
- Error type: Returns `String` error messages

**Current Error Handling Pattern:**
```rust
self.config.validate()?;  // Returns Result<(), String>
```

**YAMLError Handling Consideration:**
This call validates configuration consistency (mutually exclusive options). The validation checks for:
- `warnings_as_errors` requires `emit_warnings` to be true
- Strict mode should not allow duplicates
- `strict_types` should align with strict mode

Since this is configuration validation (not YAML parsing), it may not need YAMLError handling, but should be reviewed for consistency with the overall error handling strategy.

---

### 2. ValidatorConfigBuilder::build() - Line 1007
**File:** `src/parsers/config.rs`  
**Line:** 1007  
**Function:** `ValidatorConfigBuilder::build()`  
**Code:**
```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;
    Ok(self.config)
}
```

**Context:**
- Located in builder pattern implementation
- Calls `ValidatorConfig::validate()` method
- Returns `Result<ValidatorConfig, String>`
- Current error handling: Uses `?` operator, propagates `String` errors
- Error type: Returns `String` error messages

**Current Error Handling Pattern:**
```rust
self.config.validate()?;  // Returns Result<(), String>
```

**YAMLError Handling Consideration:**
Similar to ParserConfig, this validates configuration consistency. The validation checks for:
- Strict mode requires `require_all_fields` to be true
- Strict mode requires `disallow_unknown_fields` to be true
- `warnings_as_errors` requires `emit_warnings` to be true

This is configuration validation (not YAML parsing), so YAMLError handling may not be applicable, but should be reviewed for consistency.

---

### 3. BasicParser::validate_str() - Line 121
**File:** `src/parsers/yaml/parser.rs`  
**Line:** 121  
**Function:** `BasicParser::validate_str()`  
**Code:**
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

**Context:**
- Core YAML validation method in the Parser trait implementation
- Calls `SyntaxValidator::validate()` method on line 121
- Returns `ValidationResult` (custom type with `valid`, `errors`, `warnings` fields)
- Current error handling: Direct assignment, no `?` operator
- Error type: Returns `ValidationResult` struct

**Current Error Handling Pattern:**
```rust
let mut result = validator.validate(content);
// Returns ValidationResult, not a Result type
```

**YAMLError Handling Consideration:**
This is the **most critical caller** for YAMLError handling. This method:
- Is the main entry point for YAML syntax validation
- Currently returns `ValidationResult` (not a `Result` type)
- Could be updated to return `Result<(), YAMLError>` or similar
- Would benefit from structured YAMLError types for better error reporting

---

## Validation Method Implementations

For context, here are the validate() method implementations being called:

### ParserConfig::validate() - Line 537
```rust
pub fn validate(&self) -> Result<(), String> {
    // Check for mutually exclusive or inconsistent options
    if self.warnings_as_errors && !self.emit_warnings {
        return Err("warnings_as_errors requires emit_warnings to be true".to_string());
    }

    // Strict mode should not allow duplicates
    if self.mode.is_strict() && self.allow_duplicates {
        return Err("Strict mode with allow_duplicates=true is inconsistent".to_string());
    }

    // Strict types should align with strict mode
    if self.strict_types && self.mode.is_lenient() {
        return Err("strict_types=true with lenient mode is inconsistent".to_string());
    }

    Ok(())
}
```

### ValidatorConfig::validate() - Line 908
```rust
pub fn validate(&self) -> Result<(), String> {
    // Check for mutually exclusive options
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

### SyntaxValidator::validate()
This is called from parser.rs line 121. Implementation is in `src/parsers/yaml/syntax_validator.rs`.

---

## Prioritization for Updates

### High Priority
1. **BasicParser::validate_str()** (line 121) - Primary YAML validation entry point

### Low Priority
2. **ParserConfigBuilder::build()** (line 662) - Configuration validation only
3. **ValidatorConfigBuilder::build()** (line 1007) - Configuration validation only

**Rationale:** The configuration validation methods (low priority) validate struct consistency rather than YAML content, so they may not need YAMLError handling. The parser's `validate_str()` method (high priority) is the actual YAML validation entry point and would benefit most from structured YAMLError types.

---

## Next Steps

1. Review YAMLError type definition to understand available error variants
2. Determine if configuration validation should use YAMLError or keep String errors
3. Update SyntaxValidator to return Result types with YAMLError
4. Update BasicParser::validate_str() to handle YAMLError properly
5. Add appropriate error conversion and propagation

---

**Generated:** 2026-07-12  
**Bead ID:** bf-1x2wu  
**Search Command:** `rg "\.validate\(" --type rust -g "!test*" -g "!*test*.rs" -C 3`
