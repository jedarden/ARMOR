# Validate() Call Sites in ARMOR

## Summary

All Validate() call sites discovered in the ARMOR codebase, categorized by file.

## Production Code (non-test)

### src/parsers/config.rs
- src/parsers/config.rs:662 - `self.config.validate()?;`
- src/parsers/config.rs:1007 - `self.config.validate()?;`

### src/parsers/yaml/parser.rs
- src/parsers/yaml/parser.rs:121 - `let mut result = validator.validate(content);`

### src/schema.rs (doc comments only)
- Lines 145-146, 216-218, 270-272 - Documentation examples only

### src/parsers/yaml/syntax_validator.rs (tests only)
- Lines 438, 453, 461, 470, 479, 487, 495, 503 - Test code

### src/parsers/traits.rs (doc comment only)
- Line 319 - Documentation example only

### src/parsers/yaml/syntax_detector_tests.rs (tests only)
- Lines 131, 567, 728 - Test code

## Test Code

### tests/schema_validation_test.rs (all entries)
- tests/schema_validation_test.rs:148 - `UsernameSchema.validate(&user.username)`
- tests/schema_validation_test.rs:150 - `AgeSchema.validate(&user.age)`
- Lines 180-184, 192-194, 202-206, 214-218, 230-236, 242, 250-253, 261-264, 276, 282, 290-292
- Lines 304, 312, 316, 325, 338, 351, 363, 375, 392, 409, 421, 429, 437, 449, 457, 465, 482, 498, 514, 526, 538, 545
- Lines 559-560, 563-564, 572, 575, 583, 586, 590, 621, 631, 647

### src/parsers/config.rs (tests)
- Lines 1202, 1205, 1216, 1224, 1232, 1297, 1300, 1312, 1320

### src/schema.rs (doc tests)
- Lines 318-319, 322, 326, 352-354, 357-358, 431-433, 436, 440, 462-463, 466, 505, 511, 518, 527, 551-552, 555, 560, 564
- Lines 602-606, 610-614, 660, 662, 675, 682, 693

### src/parsers/yaml/syntax_validator.rs (tests)
- Lines 438, 453, 461, 470, 479, 487, 495, 503

### src/parsers/yaml/syntax_detector_tests.rs (tests)
- Lines 131, 567, 728

## Total Count

- **Production (non-test) call sites: 3**
- **Test call sites: 100+** (comprehensive test coverage)

## Key Production Call Sites

1. **src/parsers/config.rs:662** - Config::reload() validates config after reload
2. **src/parsers/config.rs:1007** - Config::try_new() validates config during construction
3. **src/parsers/yaml/parser.rs:121** - YamlParser validates content during parsing

## Search Method

Searched using: `rg "\.validate\(" --type rust -n`

Generated: 2026-07-12
