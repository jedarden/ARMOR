# Validate() Call Sites Inventory

**Search performed:** 2026-07-12
**Search patterns:** `\.validate\(` across Rust files

## Summary Statistics

- **Total call sites found:** 145
- **Production code (non-test):** 3 call sites
- **Test code:** 142 call sites

## Production Code Call Sites (non-test)

### src/parsers/config.rs
- **Line 662:** `self.config.validate()?;`
  - Context: Called in `Config::reload()` method
  - Purpose: Validates configuration after reload
- **Line 1007:** `self.config.validate()?;`
  - Context: Called in `Config::try_new()` method
  - Purpose: Validates configuration during construction

### src/parsers/yaml/parser.rs
- **Line 121:** `let mut result = validator.validate(content);`
  - Context: Called in `YamlParser::parse()` method
  - Purpose: Validates YAML content during parsing

### src/parsers/traits.rs (documentation only)
- **Line 319:** Documentation comment example `if parser.validate("key: value").is_ok() {`
  - Context: Trait documentation example
  - Purpose: Shows usage pattern

## Test Code Call Sites

### src/parsers/config.rs (unit tests)
- Line 1202: `assert!(config.validate().is_ok());`
- Line 1205: `assert!(strict_config.validate().is_ok());`
- Line 1216: `assert!(config.validate().is_err());`
- Line 1224: `assert!(config2.validate().is_err());`
- Line 1232: `assert!(config3.validate().is_err());`
- Line 1297: `assert!(config.validate().is_ok());`
- Line 1300: `assert!(strict_config.validate().is_ok());`
- Line 1312: `assert!(config.validate().is_err());`
- Line 1320: `assert!(config2.validate().is_err());`

### src/parsers/yaml/syntax_validator.rs (tests)
- Line 438: `let result = validator.validate("");`
- Line 453: `let result = validator.validate(yaml);`
- Line 461: `let result = validator.validate(yaml);`
- Line 470: `let result = validator.validate(yaml);`
- Line 479: `let result = validator.validate(yaml);`
- Line 487: `let result = validator.validate(yaml);`
- Line 495: `let result = validator.validate(yaml);`
- Line 503: `let result = validator.validate(yaml);`

### src/parsers/yaml/syntax_detector_tests.rs
- Line 131: `let result = validator.validate(yaml);`
- Line 567: `let result = validator.validate(yaml);`
- Line 728: `let result = validator.validate(yaml);`

### src/schema.rs (examples and doctests)
- Lines 145-146, 216, 218, 270-272: Documentation string examples
- Lines 318-326: RangeSchema doctest examples
- Lines 352-358: InclusiveRangeSchema doctest examples
- Lines 431-440: NonEmptyStringSchema doctest examples
- Lines 462-466: NonEmptyVecSchema doctest examples
- Lines 505-527: ServerConfigSchema doctest examples
- Lines 551-564: OptionSchema doctest examples
- Lines 602-614: CombineAndSchema doctest examples
- Lines 660-662, 675-693: StructSchema doctest examples

### tests/schema_validation_test.rs (comprehensive test suite)
- Lines 148, 150: Nested validation in UserSchema
- Lines 180-290: Successful validation tests across all schema types
- Lines 304-545: Error case tests
- Lines 559-575: Range validation tests
- Lines 583-647: Additional validation scenarios

## Raw List (file:line format)

### Production
```
src/parsers/config.rs:662
src/parsers/config.rs:1007
src/parsers/yaml/parser.rs:121
```

### Tests (abbreviated - see full grep output for complete list)
```
src/parsers/config.rs:1202
src/parsers/config.rs:1205
src/parsers/config.rs:1216
src/parsers/config.rs:1224
src/parsers/config.rs:1232
src/parsers/config.rs:1297
src/parsers/config.rs:1300
src/parsers/config.rs:1312
src/parsers/config.rs:1320
src/parsers/yaml/syntax_validator.rs:438
src/parsers/yaml/syntax_validator.rs:453
src/parsers/yaml/syntax_validator.rs:461
src/parsers/yaml/syntax_validator.rs:470
src/parsers/yaml/syntax_validator.rs:479
src/parsers/yaml/syntax_validator.rs:487
src/parsers/yaml/syntax_validator.rs:495
src/parsers/yaml/syntax_validator.rs:503
src/parsers/yaml/syntax_detector_tests.rs:131
src/parsers/yaml/syntax_detector_tests.rs:567
src/parsers/yaml/syntax_detector_tests.rs:728
src/schema.rs:318
src/schema.rs:319
src/schema.rs:322
src/schema.rs:326
[... and 100+ more in src/schema.rs and tests/schema_validation_test.rs]
```

## Notes

- Search pattern used: `\.validate\(` (method calls only)
- Trait definitions (`fn validate(&self, ...)`) excluded
- Docstring examples included but marked as documentation
- No false positives detected in manual review
