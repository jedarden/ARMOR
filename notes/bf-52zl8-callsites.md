# Validate() Call Sites Catalog

## Summary

This document catalogs all locations in the ARMOR codebase where Validate() methods are called, categorized by type and error handling pattern.

## Validate() Method Types

### 1. Schema Trait Validation
- **Method**: `.validate(&value) -> ValidationResult/Result<(), ParseError>`
- **Location**: `src/schema.rs` - Schema trait
- **Usage**: Validates values against schema constraints (ranges, types, formats)
- **Return**: `ValidationResult` or `Result<(), ParseError>`

### 2. Config Struct Validation
- **Method**: `.validate() -> Result<(), String>`
- **Locations**:
  - `ParserConfig::validate()` at `src/parsers/config.rs:537`
  - `ValidatorConfig::validate()` at `src/parsers/config.rs:908`
- **Usage**: Validates configuration consistency (mutually exclusive options, mode alignment)
- **Return**: `Result<(), String>`

### 3. Syntax Validator Validation
- **Method**: `.validate(content: &str) -> ValidationResult`
- **Location**: `src/parsers/yaml/syntax_validator.rs:65`
- **Usage**: Validates YAML syntax (indentation, delimiters, structure)
- **Return**: `ValidationResult` struct with `valid: bool, errors: Vec<ValidationError>, warnings: Vec<ValidationError>`

### 4. Parser Trait Validation
- **Methods**:
  - `.validate_str(content: &str) -> ValidationResult`
  - `.validate_file(path: &Path) -> ValidationResult`
- **Location**: `src/parsers/yaml/parser.rs` - Parser trait
- **Usage**: Entry point for validation operations
- **Return**: `ValidationResult`

---

## Production Call Sites (Non-Test)

### High Priority - Direct Error Handling Required

#### 1. ParserConfigBuilder::build()
- **File**: `src/parsers/config.rs:662`
- **Code**: `self.config.validate()?`
- **Pattern**: Builder validation with `?` operator
- **Error Type**: `Result<(), String>` converted to builder error
- **Status**: ✅ **HAS ERROR HANDLING** - Uses `?` operator
- **Priority**: LOW - Already properly handled

```rust
pub fn build(self) -> Result<ParserConfig, String> {
    self.config.validate()?;  // Line 662
    Ok(self.config)
}
```

#### 2. ValidatorConfigBuilder::build()
- **File**: `src/parsers/config.rs:1007`
- **Code**: `self.config.validate()?`
- **Pattern**: Builder validation with `?` operator
- **Error Type**: `Result<(), String>` converted to builder error
- **Status**: ✅ **HAS ERROR HANDLING** - Uses `?` operator
- **Priority**: LOW - Already properly handled

```rust
pub fn build(self) -> Result<ValidatorConfig, String> {
    self.config.validate()?;  // Line 1007
    Ok(self.config)
}
```

#### 3. BasicParser::validate_str()
- **File**: `src/parsers/yaml/parser.rs:121`
- **Code**: `let mut result = validator.validate(content);`
- **Pattern**: Stores ValidationResult, does not use `?` operator
- **Error Type**: `ValidationResult` (struct with valid/errors/warnings fields)
- **Status**: ✅ **PROPERLY HANDLED** - ValidationResult is not a Result type, uses structured error collection
- **Priority**: LOW - Correct pattern for ValidationResult type

```rust
fn validate_str(&self, content: &str) -> ValidationResult {
    let validator = if self.config.is_strict() {
        SyntaxValidator::strict()
    } else {
        SyntaxValidator::lenient()
    };

    let mut result = validator.validate(content);  // Line 121

    // Enhanced detection continues...
    result
}
```

#### 4. BasicParser::validate_file()
- **File**: `src/parsers/yaml/parser.rs:155`
- **Code**: `self.validate_str(&content)`
- **Pattern**: Internal delegation, returns ValidationResult
- **Error Type**: `ValidationResult`
- **Status**: ✅ **PROPERLY HANDLED** - Delegates to validate_str which returns ValidationResult
- **Priority**: LOW - Correct delegation pattern

```rust
fn validate_file(&self, path: &std::path::Path) -> ValidationResult {
    let content = match std::fs::read_to_string(path) {
        Ok(content) => content,
        Err(err) => {
            return ValidationResult {
                valid: false,
                errors: vec![ValidationError::new(
                    path.display().to_string(),
                    format!("failed to read file: {}", err)
                )],
                warnings: Vec::new(),
            };
        }
    };

    self.validate_str(&content)  // Line 155
}
```

### Internal Validation Methods

#### 5. SyntaxValidator::validate() internal calls
- **File**: `src/parsers/yaml/syntax_validator.rs`
- **Lines**: 82, 86, 90, 99
- **Code**:
  - Line 82: `self.validate_indentation(line, line_num_1indexed, &context)`
  - Line 86: `self.validate_delimiters(line, line_num_1indexed)`
  - Line 90: `self.validate_structure(line, line_num_1indexed, &context)`
  - Line 99: `self.validate_final_structure(&context)`
- **Pattern**: Error collection with `if let Err(mut line_errors)` pattern
- **Status**: ✅ **PROPERLY HANDLED** - Error aggregation pattern
- **Priority**: LOW - Internal helper methods, correctly aggregating errors

```rust
pub fn validate(&self, content: &str) -> ValidationResult {
    let mut errors = Vec::new();
    let mut warnings = Vec::new();
    // ... loop over lines ...

    if let Err(mut line_errors) = self.validate_indentation(...) {
        errors.append(&mut line_errors);  // Line 82
    }

    if let Err(mut line_errors) = self.validate_delimiters(...) {
        errors.append(&mut line_errors);  // Line 86
    }

    if let Err(mut line_errors) = self.validate_structure(...) {
        errors.append(&mut line_errors);  // Line 90
    }

    if let Err(mut final_errors) = self.validate_final_structure(&context) {
        errors.append(&mut final_errors);  // Line 99
    }

    ValidationResult { valid: errors.is_empty(), errors, warnings }
}
```

---

## Test Call Sites

### Schema Validation Tests
- **File**: `src/schema.rs` (lines 318-693, many tests)
- **File**: `tests/schema_validation_test.rs` (comprehensive test suite)
- **Pattern**: `assert!(schema.validate(&value).is_ok())` or `.is_err()`
- **Status**: ✅ **NO CHANGES NEEDED** - Test assertions only

### Config Validation Tests
- **File**: `src/parsers/config.rs:1202-1320`
- **Pattern**: `assert!(config.validate().is_ok())` or `.is_err()`
- **Status**: ✅ **NO CHANGES NEEDED** - Test assertions only

### Syntax Validator Tests
- **File**: `src/parsers/yaml/syntax_validator.rs:438-503`
- **Pattern**: `let result = validator.validate(yaml); assert!(result.is_valid());`
- **Status**: ✅ **NO CHANGES NEEDED** - Test assertions only

---

## Error Handling Patterns

### Pattern 1: Result<(), String> with `?` Operator
- **Used by**: `ParserConfig::validate()`, `ValidatorConfig::validate()`
- **Call sites**:
  - `src/parsers/config.rs:662` - ParserConfigBuilder::build()
  - `src/parsers/config.rs:1007` - ValidatorConfigBuilder::build()
- **Status**: ✅ **CORRECT** - Proper error propagation

### Pattern 2: ValidationResult Struct
- **Used by**: `SyntaxValidator::validate()`, Parser trait methods
- **Call sites**:
  - `src/parsers/yaml/parser.rs:121` - BasicParser::validate_str()
  - `src/parsers/yaml/parser.rs:155` - BasicParser::validate_file()
- **Status**: ✅ **CORRECT** - Uses structured error aggregation, not Result type

### Pattern 3: Error Collection in Vec
- **Used by**: Internal validation methods in SyntaxValidator
- **Call sites**: `src/parsers/yaml/syntax_validator.rs:82,86,90,99`
- **Status**: ✅ **CORRECT** - Aggregates multiple errors for comprehensive reporting

---

## Findings Summary

### No Error Handling Updates Needed

All production call sites of Validate() methods in the ARMOR codebase are **properly handling errors**:

1. **Builder pattern calls** (lines 662, 1007): Use `?` operator correctly
2. **Parser validation** (lines 121, 155): Returns ValidationResult struct as designed
3. **Internal validation** (lines 82, 86, 90, 99): Aggregates errors correctly

### Categorization by Type

| Category | Count | Files | Status |
|----------|-------|-------|--------|
| Direct Config Validation | 2 | `src/parsers/config.rs` | ✅ Proper `?` operator |
| Parser Validation | 2 | `src/parsers/yaml/parser.rs` | ✅ Correct ValidationResult pattern |
| Internal Validation | 4 | `src/parsers/yaml/syntax_validator.rs` | ✅ Correct error aggregation |
| Test Call Sites | 100+ | `src/schema.rs`, `tests/` | ✅ Test assertions only |

### Priority Assessment

**All call sites are LOW priority for updates** because:

1. Config validation uses proper `Result` types with `?` operator
2. Parser validation correctly uses `ValidationResult` struct (not a Result type)
3. Internal methods properly aggregate multiple errors
4. All patterns match the intended error handling design

---

## Recommendations

### No Changes Required

The ARMOR codebase demonstrates **consistent and correct error handling patterns** across all Validate() call sites. The code properly distinguishes between:

1. **Result types** that should use `?` operator for early return
2. **ValidationResult structs** that collect multiple errors without early return
3. **Error aggregation** patterns that accumulate multiple validation failures

### Future Considerations

If new validate methods are added:
- Follow the existing patterns established in this catalog
- Use `Result<(), E>` with `?` operator for single-error scenarios (config validation)
- Use `ValidationResult` struct for multi-error scenarios (syntax validation)
- Document the expected error handling pattern in method documentation
