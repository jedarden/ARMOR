# Validate() Call Sites Catalog

**Bead ID:** bf-45l8s  
**Search Date:** 2026-07-12  
**Scope:** Entire ARMOR codebase  

## Summary

This codebase contains **two separate validation systems**:

1. **Rust code**: `validate()` method (lowercase `v`) - Schema trait for parser validation
2. **Go code**: `Validate()` method (uppercase `V`) - YAML schema validation system

### Rust Validation (validate)
- **3 production code call sites**
- **150+ test code call sites**
- Located in: `src/parsers/config.rs`, `src/parsers/yaml/parser.rs`, `src/schema.rs`, `tests/schema_validation_test.rs`

### Go Validation (Validate)
- **2 production code call sites**
- **4 test code call sites**
- Located in: `internal/yamlutil/schema.go`, `internal/yamlutil/schema_interfaces.go`

---

## Go Validation System - Validate() Method (Uppercase)

### Interface Definitions (4 total)

**File: internal/yamlutil/schema_interfaces.go**
```go
// Line 34: Schema interface
Validate() YAMLError

// Line 71: SchemaMap interface  
Validate(data map[string]interface{}) SchemaValidationResult

// Line 89: Constraint interface
Validate(value interface{}) *ConstraintError
```

**File: internal/yamlutil/schema.go**
```go
// Line 51: SchemaDefinition interface
Validate(value interface{}) error
```

### Method Implementations (8 total)

**File: internal/yamlutil/schema_interfaces.go**
```go
// Line 343: String constraint validation
func (sc *StringConstraintImpl) Validate(value interface{}) *ConstraintError

// Line 458: Number constraint validation
func (nc *NumberConstraintImpl) Validate(value interface{}) *ConstraintError

// Line 560: Array constraint validation
func (ac *ArrayConstraintImpl) Validate(value interface{}) *ConstraintError

// Line 647: Object constraint validation
func (oc *ObjectConstraintImpl) Validate(value interface{}) *ConstraintError

// Line 746: Boolean constraint validation
func (bc *BooleanConstraintImpl) Validate(value interface{}) *ConstraintError

// Line 795: Type constraint validation
func (tc *TypeConstraintImpl) Validate(value interface{}) *ConstraintError
```

**File: internal/yamlutil/schema.go**
```go
// Line 157: Schema validator
func (sv *SchemaValidator) Validate(data interface{}) SchemaValidationResult

// Line 770: Schema definition
func (s *SchemaDefinition) Validate(value interface{}) error
```

### Production Code Call Sites (2 total)

#### 1. `internal/yamlutil/schema.go:180`
```go
if err := sv.schema.Validate(data); err != nil {
```
**Context:** SchemaValidator.Validate() method  
**Usage:** Delegates to underlying schema's Validate() method

#### 2. `internal/yamlutil/schema.go:253`
```go
return sv.Validate(data)
```
**Context:** ReadAndValidate() helper function  
**Usage:** Chains into SchemaValidator.Validate() after reading YAML file

### Test Code Call Sites (4 total)

**File: internal/yamlutil/schema_validation_test.go**
```go
// Line 94: Table-driven test - schema with no parameters
err := tt.schema.Validate()

// Line 147: Direct test call to schema.Validate()
err := schema.Validate()

// Line 224: Table-driven test - validator with data
result := validator.Validate(tt.data)

// Line 310: Another table-driven test
result := validator.Validate(tt.data)
```

### Related Helper Function

**File: internal/yamlutil/parse_error_examples_test.go**
```go
// Line 474: parseAndValidate helper (wraps Validate())
result := parseAndValidate(path)
```

---

## Rust Validation System - validate() Method (Lowercase)

### Production Code Call Sites (3 total)

#### 1. `src/parsers/config.rs:662`
```rust
self.config.validate()?;
```
**Context:** ConfigParser struct, parsing YAML configuration  
**Usage:** Validates parsed configuration before returning

#### 2. `src/parsers/config.rs:1007`
```rust
self.config.validate()?;
```
**Context:** StrictConfigParser struct, parsing strict YAML configuration  
**Usage:** Validates parsed strict configuration before returning

#### 3. `src/parsers/yaml/parser.rs:121`
```rust
let mut result = validator.validate(content);
```
**Context:** YAML parsing pipeline  
**Usage:** Validates YAML content using syntax validator

### Test Code Call Sites (150+ total)

#### `src/parsers/config.rs` (14 call sites)
**Lines 1202-1320:** Unit test assertions in config parser tests  
- Testing validation success paths
- Testing validation error paths  
- Testing strict vs lenient config validation

#### `src/schema.rs` (60+ call sites)
**Lines 145-693:** Doctest examples and inline tests  
- Range validation examples
- Positive integer validation
- String validation
- Server config validation
- Option validation
- Composite user validation
- Edge case testing

#### `src/parsers/yaml/syntax_validator.rs` (9 call sites)
**Lines 438-503:** YAML syntax validator tests  
- Testing empty YAML
- Testing various YAML syntax patterns
- Testing validator error detection

#### `src/parsers/yaml/syntax_detector_tests.rs` (3 call sites)
**Lines 131, 567, 728:** YAML syntax detection tests  
- Testing syntax detection across various YAML patterns

#### `tests/schema_validation_test.rs` (100+ call sites)
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

### Rust Schema Trait Definition
**File:** `src/schema.rs:224`

```rust
pub trait Schema<T: ?Sized> {
    fn validate(&self, value: &T) -> Result<(), ParseError>;
}
```

### Rust Common Usage Patterns

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

### Rust Schema Implementations (22 total)

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
# Rust validate() search
rg "\.validate\(" --type rust -n | grep -v "//"

# Go Validate() search
rg "Validate\(" --type go -n | grep -v "//"

# Arrow notation search (none found in codebase)
rg --type rust -n -- "->validate\("
```

---

## Notes

- **Two separate systems**: The codebase has independent Rust and Go validation systems
- **Rust**: Uses lowercase `validate()` method, 3 production call sites, 150+ test sites
- **Go**: Uses uppercase `Validate()` method, 2 production call sites, 4 test sites
- All production call sites use method call syntax (`.validate()` or `.Validate()`)
- No arrow notation calls (`->validate()` or `->Validate()`) exist in the codebase
- Heavy test coverage across both systems
- Production usage is minimal and focused on configuration/validation pipelines
