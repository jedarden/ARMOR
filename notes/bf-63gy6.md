# Folded Scalar Explicit Indent Test Infrastructure

**Bead:** bf-63gy6
**Date:** 2026-07-13
**Status:** Infrastructure already complete in Section 12B.3

## Summary

The test infrastructure for folded scalar explicit indent tests is **already fully implemented** in `tests/type_like_string_false_positive_test.rs` Section 12B.3 (starting at line 13267). No new infrastructure needs to be created.

## Existing Infrastructure

### 1. Macros

**`generate_folded_explicit_indent_tests!`** (line 772)
- Generates test cases for a single indentation level
- Parameters: `indent`, `level_name`, `modifiers`, `indent_nums`, `key_prefix`
- Returns: `Vec<(String, String, LineType)>`

**`run_folded_scalar_tests!`** (line 791)
- Executes test cases with standard assertions
- Performs two-level validation:
  1. Line type classification
  2. Key detection (conditional on MappingKey)
- Provides detailed error messages

### 2. Helper Functions

**`create_folded_scalar_test()`** (line 992)
- Creates a single test case tuple
- Parameters: `indent`, `key`, `modifier`, `indent_level`
- Returns: `(line, key, LineType::MappingKey)`

**`generate_folded_scalar_tests_multi_level()`** (line 1048)
- Bulk generates tests for 5 indentation levels (levels 1-4 + tab)
- Excludes level 0
- Parameters: `keys`, `modifiers`, `indent_levels`

**`generate_folded_scalar_tests_all_levels()`** (line 1134)
- **Comprehensive version** - includes all 6 levels (0-4 + tab)
- Parameters: `keys`, `modifiers`, `indent_levels`
- Use for complete coverage

**`generate_folded_scalar_tests_for_level()`** (line 1226)
- Generates tests for a specific indentation level
- Parameters: `level`, `keys`, `modifiers`, `indent_nums`
- Use for focused testing

### 3. Template Examples

**`test_folded_scalar_explicit_indent_template_example`** (line 13400)
- Basic template showing macro usage
- 2-space indentation example

**`test_folded_scalar_explicit_indent_tab_template`** (line 13421)
- Tab indentation template

**`test_folded_scalar_explicit_indent_helper_function_example`** (line 13437)
- Demonstrates helper function usage
- Shows manual tuple construction

**`test_folded_scalar_macro_example`** (line 13460)
- Comprehensive demonstration
- Multiple combinations: 2-space, 4-space, tab
- Shows different modifier sets

### 4. Available Modifiers

- `>` - Plain folded scalar (default)
- `>-` - Strip modifier (removes trailing newlines)
- `>+` - Keep modifier (preserves trailing newlines)

### 5. Indentation Levels

| Level ID | Indentation | Description |
|----------|-------------|-------------|
| `level0` | `""` | No indentation (root level) |
| `level1` | `"  "` | 2 spaces |
| `level2` | `"    "` | 4 spaces |
| `level3` | `"      "` | 6 spaces |
| `level4` | `"        "` | 8 spaces |
| `tab` | `"\t"` | Tab character |

### 6. Test Case Structure

Each test case is a tuple: `(line: String, expected_key: String, expected_type: LineType)`

**Example:**
```rust
("  my_key: >1", "my_key", LineType::MappingKey)
```

## Pattern for New Tests

### Approach 1: Macro-based (Recommended)

```rust
#[test]
fn test_folded_scalar_<variant>() {
    let test_cases = generate_folded_explicit_indent_tests!(
        "  ",                    // indent: 2 spaces
        "level1",               // level_name: descriptive name
        [">", ">-", ">+"],      // modifiers: array of modifier patterns
        [1, 2, 3, 4, 5],       // indent_nums: array of indent numbers
        "test"                  // key_prefix: prefix for generated key names
    );

    run_folded_scalar_tests!(test_cases);
}
```

### Approach 2: Helper Function

```rust
#[test]
fn test_folded_scalar_custom_pattern() {
    let mut cases = vec![];

    // Add custom test cases
    cases.push(create_folded_scalar_test("  ", "my_key", ">", 2));
    cases.push(create_folded_scalar_test("    ", "another", ">-", 3));

    run_folded_scalar_tests!(cases);
}
```

### Approach 3: Manual Specification

```rust
#[test]
fn test_folded_scalar_manual() {
    let test_cases = vec![
        ("  key: >1".to_string(), "key".to_string(), LineType::MappingKey),
        ("root: >2".to_string(), "root".to_string(), LineType::MappingKey),
    ];

    run_folded_scalar_tests!(test_cases);
}
```

## Naming Conventions

Test function names should follow: `test_folded_scalar_<variant>_<details>()`

Examples:
- `test_folded_scalar_plain_explicit_indent_modifiers_at_2_space()`
- `test_folded_scalar_strip_explicit_indent_at_level3()`
- `test_folded_scalar_keep_explicit_indent_tab()`

## Coverage Strategy

For **comprehensive coverage** of a new folded scalar variant:

1. **Use `generate_folded_scalar_tests_all_levels()`** for all 6 levels
2. **Include all modifiers**: `[">", ">-", ">+"]`
3. **Test multiple indent numbers**: `[1, 2, 3, 4, 5]` or `[1, 2, 3, 4, 5, 6, 7, 8, 9]`

For **focused testing** on a specific scenario:

1. **Use `generate_folded_scalar_tests_for_level()`** for one level
2. **Specify relevant modifiers only**
3. **Use indent numbers appropriate to the test**

## Documentation References

- **Section 12B.3**: Complete infrastructure pattern documentation (line 13267)
- **Section 12B**: Comprehensive folded scalar test patterns (line 7816)
- **Section 12B.1**: Folded block scalar tests (line 10726)
- **Section 12B.2**: Folded scalar indicator line tests (line 10520)

## Implementation Status

✅ **COMPLETE** - All infrastructure is already in place and functional.

### What Exists:
- ✅ Macros for test generation
- ✅ Helper functions for bulk creation
- ✅ Template examples showing all approaches
- ✅ Comprehensive documentation
- ✅ Working test implementations

### What Future Beads Should Do:
1. **Copy the template functions** from Section 12B.3
2. **Modify for specific test variants** as needed
3. **Use the existing macros/helpers** to generate test cases
4. **Follow the naming conventions**
5. **Run `run_folded_scalar_tests!`** for validation

## Notes

- The infrastructure supports parameterized testing across all combinations
- No new macros or helper functions are needed
- The pattern is already documented and proven
- Focus on implementing specific test variants using the existing infrastructure

## Related Beads

- **bf-ht2h0**: Pattern documentation
- **bf-2w54h**: Enhanced helper macros supporting all indent levels
- **bf-68ime**: Section 12B comprehensive analysis
