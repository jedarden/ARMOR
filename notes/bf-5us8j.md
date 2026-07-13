# bf-5us8j: generate_folded_scalar_tests_all_levels Implementation

## Status: ALREADY IMPLEMENTED

The `generate_folded_scalar_tests_all_levels()` function already exists in the codebase.

## Location
- File: `tests/type_like_string_false_positive_test.rs`
- Line: 306

## Implementation Details

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

## Acceptance Criteria Verification

1. ✅ Implement `generate_folded_scalar_tests_all_levels()` function - EXISTS at line 306
2. ✅ Support comprehensive generation including level 0 (no indent) - YES, line 315
3. ✅ Accept keys, modifiers, indent_levels parameters - YES, lines 307-309
4. ✅ Return Vec of test case tuples for all 6 levels - YES, 6 levels (0-4 + tab)
5. ✅ Function compiles - VERIFIED with cargo check

## Coverage
The function generates test cases for 6 indentation levels:
- Level 0: "" (no indentation)
- Level 1: "  " (2 spaces)
- Level 2: "    " (4 spaces)
- Level 3: "      " (6 spaces)
- Level 4: "        " (8 spaces)
- Tab: "\t" (tab character)
