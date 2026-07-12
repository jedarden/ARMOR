# Validate() Call Sites Catalog

**Bead:** bf-45l8s
**Date:** 2026-07-12
**Total Call Sites Found:** 142

This document catalogs all locations where `.validate()` is called in the ARMOR codebase.

## Summary

The search found **142 call sites** across 8 files:
- `src/parsers/config.rs`: 11 call sites
- `src/schema.rs`: 60 call sites  
- `src/parsers/yaml/syntax_validator.rs`: 8 call sites
- `src/parsers/yaml/parser.rs`: 1 call site
- `src/parsers/yaml/syntax_detector_tests.rs`: 3 call sites
- `tests/schema_validation_test.rs`: 58 call sites
- `src/parsers/traits.rs`: 1 call site (documentation example)

## Detailed Call Sites

### src/parsers/config.rs (11 sites)

**Production Code (2 sites):**
- Line 662: `self.config.validate()?;`
- Line 1007: `self.config.validate()?;`

**Test Code (9 sites):**
- Line 1202: `assert!(config.validate().is_ok());`
- Line 1205: `assert!(strict_config.validate().is_ok());`
- Line 1216: `assert!(config.validate().is_err());`
- Line 1224: `assert!(config2.validate().is_err());`
- Line 1232: `assert!(config3.validate().is_err());`
- Line 1297: `assert!(config.validate().is_ok());`
- Line 1300: `assert!(strict_config.validate().is_ok());`
- Line 1312: `assert!(config.validate().is_err());`
- Line 1320: `assert!(config2.validate().is_err());`

### src/schema.rs (60 sites)

**Documentation Examples (7 sites):**
- Line 145: `assert!(schema.validate(&42).is_ok());`
- Line 146: `assert!(schema.validate(&-5).is_err());`
- Line 216: `NameSchema.validate(&config.name)`
- Line 218: `PortSchema.validate(&config.port)`
- Line 270: `assert!(schema.validate(&50).is_ok());`
- Line 271: `assert!(schema.validate(&0).is_err());`
- Line 272: `assert!(schema.validate(&101).is_err());`

**Range Schema Tests (12 sites):**
- Line 318: `assert!(schema.validate(&1).is_ok());`
- Line 319: `assert!(schema.validate(&100).is_ok());`
- Line 322: `let result = schema.validate(&0);`
- Line 326: `let result = schema.validate(&-5);`
- Line 352: `assert!(schema.validate(&10).is_ok());`
- Line 353: `assert!(schema.validate(&15).is_ok());`
- Line 354: `assert!(schema.validate(&20).is_ok());`
- Line 357: `assert!(schema.validate(&9).is_err());`
- Line 358: `assert!(schema.validate(&21).is_err());`
- Line 602: `assert!(i32_schema.validate(&50).is_ok());`
- Line 603: `assert!(i32_schema.validate(&10).is_ok());`
- Line 604: `assert!(i32_schema.validate(&100).is_ok());`
- Line 605: `assert!(i32_schema.validate(&9).is_err());`
- Line 606: `assert!(i32_schema.validate(&101).is_err());`
- Line 610: `assert!(u64_schema.validate(&5000).is_ok());`
- Line 611: `assert!(u64_schema.validate(&1000).is_ok());`
- Line 612: `assert!(u64_schema.validate(&10000).is_ok());`
- Line 613: `assert!(u64_schema.validate(&999).is_err());`
- Line 614: `assert!(u64_schema.validate(&10001).is_err());`

**String Schema Tests (9 sites):**
- Line 431: `assert!(schema.validate("hello").is_ok());`
- Line 432: `assert!(schema.validate("test string").is_ok());`
- Line 433: `assert!(schema.validate("  trimmed  ").is_ok());`
- Line 436: `let result = schema.validate("");`
- Line 440: `let result = schema.validate("   ");`
- Line 462: `assert!(schema.validate(&vec!["item1".to_string()]).is_ok());`
- Line 463: `assert!(schema.validate(&vec!["a".to_string(), "b".to_string()]).is_ok());`
- Line 466: `let result = schema.validate(&vec![]);`

**Struct Schema Tests (10 sites):**
- Line 505: `assert!(schema.validate(&valid_config).is_ok());`
- Line 511: `assert!(schema.validate(&valid_config2).is_ok());`
- Line 518: `let result = schema.validate(&invalid_host);`
- Line 527: `let result = schema.validate(&invalid_port);`
- Line 660: `UsernameSchema.validate(&user.username)`
- Line 662: `AgeSchema.validate(&user.age)`
- Line 675: `assert!(schema.validate(&valid_user).is_ok());`
- Line 682: `let result = schema.validate(&invalid_username);`
- Line 693: `let result = schema.validate(&invalid_age);`

**Optional Schema Tests (7 sites):**
- Line 551: `assert!(schema.validate(&Some(42)).is_ok());`
- Line 552: `assert!(schema.validate(&Some(1)).is_ok());`
- Line 555: `let result = schema.validate(&None);`
- Line 560: `let result = schema.validate(&Some(0));`
- Line 564: `let result = schema.validate(&Some(-5));`

### src/parsers/yaml/syntax_validator.rs (8 sites)

All in test code:
- Line 438: `let result = validator.validate("");`
- Line 453: `let result = validator.validate(yaml);`
- Line 461: `let result = validator.validate(yaml);`
- Line 470: `let result = validator.validate(yaml);`
- Line 479: `let result = validator.validate(yaml);`
- Line 487: `let result = validator.validate(yaml);`
- Line 495: `let result = validator.validate(yaml);`
- Line 503: `let result = validator.validate(yaml);`

### src/parsers/yaml/parser.rs (1 site)

**Production Code:**
- Line 121: `let mut result = validator.validate(content);`

### src/parsers/yaml/syntax_detector_tests.rs (3 sites)

All in test code:
- Line 131: `let result = validator.validate(yaml);`
- Line 567: `let result = validator.validate(yaml);`
- Line 728: `let result = validator.validate(yaml);`

### tests/schema_validation_test.rs (58 sites)

All test code covering comprehensive validation scenarios.

### src/parsers/traits.rs (1 site)

**Documentation Example:**
- Line 319: `if parser.validate("key: value").is_ok()`

## Analysis

### Production vs Test Code
- **Production code:** 4 call sites
  - `src/parsers/config.rs`: 2 sites
  - `src/parsers/yaml/parser.rs`: 1 site
  - (1 documentation example in traits.rs)
  
- **Test code:** 138 call sites
  - Comprehensive test coverage across schema types
  - Edge case testing
  - Integration tests

### Call Patterns
1. **Direct validation:** `schema.validate(&value)`
2. **Result checking:** `schema.validate(&value).is_ok()` / `.is_err()`
3. **Error capture:** `let result = schema.validate(&value)`
4. **Chained operations:** `self.config.validate()?`

### File Locations Summary
| File | Count | Type |
|------|-------|------|
| `src/parsers/config.rs` | 11 | Production + Tests |
| `src/schema.rs` | 60 | Tests + Docs |
| `src/parsers/yaml/syntax_validator.rs` | 8 | Tests |
| `src/parsers/yaml/parser.rs` | 1 | Production |
| `src/parsers/yaml/syntax_detector_tests.rs` | 3 | Tests |
| `tests/schema_validation_test.rs` | 58 | Tests |
| `src/parsers/traits.rs` | 1 | Docs |
| **Total** | **142** | |

## Search Method

```bash
rg --type rust -n '\.validate\(' /home/coding/ARMOR
```

This search pattern finds:
- `.validate(` - method calls on struct instances
- Excludes `->validate(` - calls through references (none found)
- Excludes function definitions `fn validate()`
- Excludes `///` doc comments (but includes doc examples with code)

---

**Generated for bead bf-45l8s**
**Last updated:** 2026-07-12
