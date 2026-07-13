# Bead bf-5us8j: generate_folded_scalar_tests_all_levels Verification

## Task
Add `generate_folded_scalar_tests_all_levels()` helper function.

## Verification Result
**Function already implemented and fully functional.**

### Implementation Location
`/home/coding/ARMOR/tests/type_like_string_false_positive_test.rs`

### Acceptance Criteria Verification
| Criterion | Status | Details |
|-----------|--------|---------|
| Implement function | ✅ Complete | Function exists with full implementation |
| Support level 0 (no indent) | ✅ Complete | Includes `("", "level0")` in indent list |
| Accept keys, modifiers, indent_levels | ✅ Complete | Function signature: `fn generate_folded_scalar_tests_all_levels(keys: &[&str], modifiers: &[&str], indent_levels: &[u32])` |
| Return Vec for all 6 levels | ✅ Complete | Returns `Vec<(String, String, armor::parsers::yaml::LineType)>` covering level0, level1, level2, level3, level4, tab |
| Function compiles | ✅ Complete | `cargo check` passes with no errors |

### Implementation Details
```rust
fn generate_folded_scalar_tests_all_levels(
    keys: &[&str],
    modifiers: &[&str],
    indent_levels: &[u32],
) -> Vec<(String, String, armor::parsers::yaml::LineType)> {
    let mut cases = vec![];

    // All indent levels including level 0 (no indentation)
    let indents = vec![
        ("", "level0"),      // Level 0: no indentation
        ("  ", "level1"),    // Level 1: 2 spaces
        ("    ", "level2"),  // Level 2: 4 spaces
        ("      ", "level3"),// Level 3: 6 spaces
        ("        ", "level4"),// Level 4: 8 spaces
        ("\t", "tab"),      // Tab indentation
    ];

    for (indent, level_name) in indents {
        for key in keys {
            for modifier in modifiers {
                for indent_level in indent_levels {
                    let full_key = format!("{}_{}", level_name, key);
                    cases.push(create_folded_scalar_test(
                        indent,
                        &full_key,
                        modifier,
                        *indent_level,
                    ));
                }
            }
        }
    }

    cases
}
```

### Context
This function appears to have been implemented as part of bead `bf-2w54h` (helper macros for parameterized folded scalar testing). The function provides comprehensive coverage across all 6 indentation levels including level 0 (no indentation).

### Test Coverage
The function is used in `test_folded_scalar_all_levels_comprehensive()` test, demonstrating its usage with:
- Single key for all levels: `&["sample"]`
- Plain modifier only: `&[">"]`
- Two indent numbers: `&[1, 2]`
- Generates 12 test cases: 6 levels × 1 key × 1 modifier × 2 indent_nums
