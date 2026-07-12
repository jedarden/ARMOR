# Validate() Call Sites Catalog

**Bead ID:** bf-45l8s  
**Search Date:** 2026-07-12  
**Scope:** Entire ARMOR codebase  

## Summary

Found **3 production code call sites** and **150+ test code call sites** for the `validate()` method from the `Schema` trait.

**Note:** The actual method name is `validate()` (lowercase `v`), not `Validate()` (uppercase `V`). The task description used uppercase, but the Rust trait method uses lowercase convention.

---

## Production Code Call Sites (3 total)

### 1. `src/parsers/config.rs:662`
```rust
self.config.validate()?;
```
**Context:** ConfigParser struct, parsing YAML configuration  
**Usage:** Validates parsed configuration before returning

### 2. `src/parsers/config.rs:1007`
```rust
self.config.validate()?;
```
**Context:** StrictConfigParser struct, parsing strict YAML configuration  
**Usage:** Validates parsed strict configuration before returning

### 3. `src/parsers/yaml/parser.rs:121`
```rust
let mut result = validator.validate(content);
```
**Context:** YAML parsing pipeline  
**Usage:** Validates YAML content using syntax validator

---

## Test Code Call Sites (150+ total)

### `src/parsers/config.rs` (14 call sites)
**Lines 1202-1320:** Unit test assertions in config parser tests  
- Testing validation success paths
- Testing validation error paths  
- Testing strict vs lenient config validation

### `src/schema.rs` (60+ call sites)
**Lines 145-693:** Doctest examples and inline tests  
- Range validation examples
- Positive integer validation
- String validation
- Server config validation
- Option validation
- Composite user validation
- Edge case testing

### `src/parsers/yaml/syntax_validator.rs` (9 call sites)
**Lines 438-503:** YAML syntax validator tests  
- Testing empty YAML
- Testing various YAML syntax patterns
- Testing validator error detection

### `src/parsers/yaml/syntax_detector_tests.rs` (3 call sites)
**Lines 131, 567, 728:** YAML syntax detection tests  
- Testing syntax detection across various YAML patterns

### `tests/schema_validation_test.rs` (100+ call sites)
**Lines 148-647:** Comprehensive Schema trait unit tests  
- Positive integer validation (success & error paths)
- Range validation (boundaries, edge cases)
- String validation (empty, whitespace, content)
- Port validation
- Server config validation
- Username validation (length constraints)
- Age validation (range constraints)
- Composite user validation (multiple fields)
- Option validation (Some/None handling)
- Error message formatting verification
- Error type classification verification

---

## Implementation Details

### Schema Trait Definition
**File:** `src/schema.rs:224`

```rust
pub trait Schema<T: ?Sized> {
    fn validate(&self, value: &T) -> Result<(), ParseError>;
}
```

### Common Usage Patterns

1. **Config validation:**
   ```rust
   self.config.validate()?;
   ```

2. **Test assertions:**
   ```rust
   assert!(schema.validate(&value).is_ok());
   let result = schema.validate(&invalid_value);
   assert!(result.is_err());
   ```

3. **Composite validation:**
   ```rust
   UsernameSchema.validate(&user.username)
       .map_err(|e| e.with_path("username"))?;
   AgeSchema.validate(&user.age)
       .map_err(|e| e.with_path("age"))?;
   ```

---

## Related Implementations

### Schema Implementations (22 total)

**In src/schema.rs:**
- `PortSchema` for `u16` (lines 92, 201)
- `PositiveIntegerSchema` for `i32` (line 134)
- `ServerConfigSchema` for `ServerConfig` (line 162)
- `NameSchema` for `String` (line 190)
- `ConfigSchema` for `Config` (line 213)
- `RangeSchema<T>` generic (lines 258, 577, 589)
- `PositiveSchema` for `i32` (line 305)
- `NonEmptyStringSchema` for `str` (line 414)
- `NonEmptyVecSchema` for `Vec<String>` (line 449)
- `PositiveValueSchema` for `Option<i32>` (line 536)
- `UsernameSchema` for `String` (line 621)
- `AgeSchema` for `u8` (line 636)
- `UserSchema` for `User` (line 657)

**In tests/schema_validation_test.rs:**
- Mock implementations for testing (8 schemas)

---

## Search Method

```bash
# Used ripgrep to search for all .validate() patterns
rg "\.validate\(" --type rust -n

# Arrow notation search (none found in codebase)
rg --type rust -n -- "->validate\("
```

---

## Notes

- All production call sites use method call syntax (`.validate()`)
- No arrow notation calls (`->validate()`) exist in the codebase
- Heavy test coverage with 150+ test assertions
- Production usage is minimal and focused on configuration validation
