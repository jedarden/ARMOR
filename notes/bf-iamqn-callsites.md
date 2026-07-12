# Validate() Call Sites

## Summary

Search completed for all Validate() method call sites and definitions in the ARMOR codebase.

## Trait Definition

### Schema Trait
- **src/schema.rs:274** - `fn validate(&self, value: &T) -> ValidationResult;` (trait method definition)

### Parser Trait
- **src/parsers/traits.rs:323** - `fn validate(&self, source: Input) -> Result<(), ParseError>;` (trait method definition)

### YamlParser Trait
- **src/parsers/yaml/parser.rs:34** - `fn validate_str(&self, content: &str) -> ValidationResult;` (trait method definition)
- **src/parsers/yaml/parser.rs:37** - `fn validate_file(&self, path: &std::path::Path) -> ValidationResult;` (trait method definition)

## Method Implementations (fn validate)

### Config Validators
- **src/parsers/config.rs:255** - `pub fn validate(&self, field: &str, value: &serde_yaml::Value) -> Result<(), String>` (ConfigValidator)
- **src/parsers/config.rs:537** - `pub fn validate(&self) -> Result<(), String>` (StrictConfigValidator)
- **src/parsers/config.rs:908** - `pub fn validate(&self) -> Result<(), String>` (Config impl)

### Schema Implementations
- **src/schema.rs:306** - `fn validate(&self, value: &i32) -> ValidationResult` (RangeSchema impl)
- **src/schema.rs:339** - `fn validate(&self, value: &i32) -> ValidationResult` (NonEmptyStringSchema impl)
- **src/schema.rs:415** - `fn validate(&self, value: &str) -> ValidationResult` (NonEmptyStringSchema impl)
- **src/schema.rs:450** - `fn validate(&self, value: &Vec<String>) -> ValidationResult` (NonEmptyVectorSchema impl)
- **src/schema.rs:481** - `fn validate(&self, config: &ServerConfig) -> ValidationResult` (ServerConfigSchema impl)
- **src/schema.rs:537** - `fn validate(&self, value: &Option<i32>) -> ValidationResult` (OptionalSchema impl)
- **src/schema.rs:578** - `fn validate(&self, value: &i32) -> ValidationResult` (BoundedSchema impl)
- **src/schema.rs:590** - `fn validate(&self, value: &u64) -> ValidationResult` (BoundedSchema impl)
- **src/schema.rs:622** - `fn validate(&self, username: &String) -> ValidationResult` (UsernameSchema impl)
- **src/schema.rs:637** - `fn validate(&self, age: &u8) -> ValidationResult` (AgeSchema impl)
- **src/schema.rs:658** - `fn validate(&self, user: &User) -> ValidationResult` (UserSchema impl)

### YamlParser Implementations
- **src/parsers/yaml/parser.rs:112** - `fn validate_str(&self, content: &str) -> ValidationResult` (YamlParser impl)
- **src/parsers/yaml/parser.rs:138** - `fn validate_file(&self, path: &std::path::Path) -> ValidationResult` (YamlParser impl)

### SyntaxValidator
- **src/parsers/yaml/syntax_validator.rs:65** - `pub fn validate(&self, content: &str) -> ValidationResult` (SyntaxValidator method)

### Test Implementations
- **tests/schema_validation_test.rs:25** - `fn validate(&self, value: &i32) -> Result<(), ParseError>` (AgeSchema)
- **tests/schema_validation_test.rs:41** - `fn validate(&self, value: &i32) -> Result<(), ParseError>` (PortSchema)
- **tests/schema_validation_test.rs:55** - `fn validate(&self, value: &str) -> Result<(), ParseError>` (NonEmptyStringSchema)
- **tests/schema_validation_test.rs:72** - `fn validate(&self, value: &u16) -> Result<(), ParseError>` (PortSchema)
- **tests/schema_validation_test.rs:90** - `fn validate(&self, config: &ServerConfig) -> Result<(), ParseError>` (ServerConfigSchema)
- **tests/schema_validation_test.rs:107** - `fn validate(&self, username: &String) -> Result<(), ParseError>` (UsernameSchema)
- **tests/schema_validation_test.rs:124** - `fn validate(&self, age: &u8) -> Result<(), ParseError>` (AgeSchema)
- **tests/schema_validation_test.rs:147** - `fn validate(&self, user: &User) -> Result<(), ParseError>` (UserSchema)
- **tests/schema_validation_test.rs:160** - `fn validate(&self, value: &Option<i32>) -> Result<(), ParseError>` (OptionalSchema)
- **tests/schema_validation_test.rs:600** - `fn validate(&self, user: &User) -> Result<(), ParseError>` (UserSchema variant)

### Helper Functions (validate_* naming)
- **src/parsers/yaml/syntax_validator.rs:111** - `fn validate_indentation(...)`
- **src/parsers/yaml/syntax_validator.rs:176** - `fn validate_delimiters(...)`
- **src/parsers/yaml/syntax_validator.rs:250** - `fn validate_structure(...)`
- **src/parsers/yaml/syntax_validator.rs:335** - `fn validate_final_structure(...)`
- **tests/parse_error_integration_test.rs:92** - `fn validate_config(value: &str) -> Result<()>`
- **tests/parse_error_full_lifecycle_integration_test.rs:63** - `fn validate_port_field(...)`
- **tests/parse_error_full_lifecycle_integration_test.rs:287** - `fn validate_config(...)`
- **tests/parse_error_full_lifecycle_integration_test.rs:704** - `fn validate_config_with_recovery(...)`

## Call Sites (.validate())

### Production Code

#### src/parsers/config.rs
- **662** - `self.config.validate()?;`
- **1007** - `self.config.validate()?;`

#### src/parsers/yaml/parser.rs
- **121** - `let mut result = validator.validate(content);`

#### src/parsers/yaml/syntax_validator.rs
- **438** - `let result = validator.validate("");`
- **453** - `let result = validator.validate(yaml);`
- **461** - `let result = validator.validate(yaml);`
- **470** - `let result = validator.validate(yaml);`
- **479** - `let result = validator.validate(yaml);`
- **487** - `let result = validator.validate(yaml);`
- **495** - `let result = validator.validate(yaml);`
- **503** - `let result = validator.validate(yaml);`

#### src/parsers/yaml/syntax_detector_tests.rs
- **131** - `let result = validator.validate(yaml);`
- **567** - `let result = validator.validate(yaml);`
- **728** - `let result = validator.validate(yaml);`

#### src/schema.rs (validation logic within impls)
- **660** - `UsernameSchema.validate(&user.username)`
- **662** - `AgeSchema.validate(&user.age)`

#### tests/schema_validation_test.rs
- **148** - `UsernameSchema.validate(&user.username)`
- **150** - `AgeSchema.validate(&user.age)`

### Test Code (src/parsers/config.rs - unit tests)
- **1202** - `assert!(config.validate().is_ok());`
- **1205** - `assert!(strict_config.validate().is_ok());`
- **1216** - `assert!(config.validate().is_err());`
- **1224** - `assert!(config2.validate().is_err());`
- **1232** - `assert!(config3.validate().is_err());`
- **1297** - `assert!(config.validate().is_ok());`
- **1300** - `assert!(strict_config.validate().is_ok());`
- **1312** - `assert!(config.validate().is_err());`
- **1320** - `assert!(config2.validate().is_err());`

### Test Code (src/schema.rs - examples/doctests)
- **145** - `assert!(schema.validate(&42).is_ok());`
- **146** - `assert!(schema.validate(&-5).is_err());`
- **216** - `NameSchema.validate(&config.name)`
- **218** - `PortSchema.validate(&config.port)`
- **270-272** - RangeSchema examples
- **318-326** - RangeSchema tests
- **352-358** - RangeSchema tests
- **431-440** - NonEmptyStringSchema tests
- **462-466** - NonEmptyVectorSchema tests
- **505-527** - ServerConfigSchema tests
- **551-564** - OptionalSchema tests
- **602-614** - BoundedSchema tests
- **660,662** - UserSchema composite validation
- **675-693** - UserSchema tests

### Test Code (tests/schema_validation_test.rs)
**Lines 180-647**: Extensive test coverage with 70+ validate() calls across all schema types:
- Integer range validation (180-184, 192-194, 559-575)
- String non-empty validation (202-206, 583-590)
- Port validation (214-218)
- Config validation (230-242)
- Username validation (250-253, 421, 429, 437)
- Age validation (261-264, 375, 449, 457, 465)
- User validation (276-282, 482, 498, 514, 621, 631, 647)
- Optional validation (290-292, 526, 538, 545)
- Negative test cases (304-545)

## Statistics

- **Trait definitions**: 2 (Schema, Parser) + 2 (YamlParser methods)
- **Method implementations**: 11 production + 10 test + 4 helper functions
- **Production call sites**: ~15
- **Test call sites**: ~100+
- **Total unique locations**: ~150+

## Categories

1. **Core validation trait**: `src/schema.rs:274` (Schema trait)
2. **Parser validation trait**: `src/parsers/traits.rs:323` (Parser trait)
3. **YAML validation**: `src/parsers/yaml/syntax_validator.rs:65`
4. **Config validation**: `src/parsers/config.rs` (multiple validators)
5. **Schema implementations**: `src/schema.rs` (multiple impl blocks)
6. **Test helpers**: `tests/parse_error_*_test.rs`
7. **Unit tests**: Extensive coverage across `src/schema.rs` and `tests/schema_validation_test.rs`
