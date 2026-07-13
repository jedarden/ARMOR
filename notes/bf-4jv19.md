# Bead bf-4jv19: create_folded_scalar_test Helper Function

## Status: Already Implemented

The `create_folded_scalar_test()` helper function was already implemented in:
- **File**: `/home/coding/ARMOR/tests/type_like_string_false_positive_test.rs`
- **Line**: 164

## Verification

All acceptance criteria met:

### ✅ Implement create_folded_scalar_test() function
Function exists and compiles successfully.

### ✅ Support parameters: indent, key, modifier, indent_level
```rust
fn create_folded_scalar_test(
    indent: &str,
    key: &str,
    modifier: &str,
    indent_level: u32,
) -> (String, String, armor::parsers::yaml::LineType)
```

### ✅ Return (line, key, LineType) tuple
Returns `(String, String, LineType)` as required.

### ✅ Function compiles and is callable
- `cargo check --tests` - passed successfully
- Function is used by `generate_folded_scalar_tests_multi_level()` and other test helpers

## Implementation Details

The function creates individual test case tuples for folded scalar testing by:
1. Combining modifier and indent_level: `format!("{}{}", modifier, indent_level)`
2. Creating the test line: `format!("{}{}: {}", indent, key, modifier_str)`
3. Returning tuple: `(line, key.to_string(), LineType::MappingKey)`

## Example Usage

```rust
let test_case = create_folded_scalar_test("  ", "my_key", ">", 2);
// Returns: ("  my_key: >2", "my_key", LineType::MappingKey)
```

This function is a core building block for parameterized folded scalar testing in the ARMOR test suite.
