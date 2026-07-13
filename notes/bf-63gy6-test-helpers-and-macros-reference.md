# Section 12B Test Helper Functions and Macros Reference

**Bead ID:** bf-63gy6  
**Date:** 2026-07-13  
**Test File:** `tests/type_like_string_false_positive_test.rs`  
**Section:** Available helper utilities for implementing Section 12B tests

---

## Overview

This document provides a **quick reference** for the helper functions and macros available in Section 12B test infrastructure. These utilities reduce code duplication and provide consistent test patterns.

---

## Available Helpers

### 1. Macro: `generate_folded_explicit_indent_tests`

**Location:** Lines ~73-89  
**Purpose:** Generate folded scalar explicit indent test cases at a specific indentation level

#### Signature
```rust
macro_rules! generate_folded_explicit_indent_tests {
    ($indent:expr, $level_name:expr, $modifiers:expr, $indent_nums:expr, $key_prefix:expr) => { ... }
}
```

#### Parameters
- **`$indent`**: The base indentation (e.g., `"  "`, `"    "`, `"\t"`)
- **`$level_name`**: Descriptive name for this indentation level (e.g., `"level1"`, `"tab"`)
- **`$modifiers`**: Array of modifier patterns (e.g., `[">", ">-", ">+"]`)
- **`$indent_nums`**: Array of indent numbers (e.g., `[1, 2, 3, 4, 5]`)
- **`$key_prefix`**: Prefix for generated key names (e.g., `"test"`)

#### Returns
`Vec<(String, String, LineType)>` - Vector of tuples containing:
1. The complete YAML line
2. The expected key name
3. The expected LineType (always `MappingKey` for this macro)

#### Usage Example
```rust
let test_cases = generate_folded_explicit_indent_tests(
    "  ",              // 2-space indent
    "level1",          // level name
    &[">", ">-", ">+"], // all modifiers
    &[1, 2, 3, 4, 5],  // indent levels 1-5
    "test"             // key prefix
);
// Generates: [("  test___1: >1", "test___1", MappingKey), ...]
```

#### Notes
- Keys are auto-generated with pattern: `{level_name}_{modifier}_{num}`
- Modifier character (`>`) is stripped from the key name
- Use this for **bulk generation** of similar test cases

---

### 2. Macro: `run_folded_scalar_tests`

**Location:** Lines ~95-123  
**Purpose:** Execute folded scalar tests with standard assertion pattern

#### Signature
```rust
macro_rules! run_folded_scalar_tests {
    ($test_cases:expr) => { ... }
}
```

#### Parameters
- **`$test_cases`**: Expression yielding `Vec<(line, expected_key, expected_type)>`

#### Behavior
1. Iterates through all test cases
2. Asserts `LineType` classification matches expected
3. For `MappingKey` types, additionally verifies:
   - Key detection succeeds
   - Detected key name matches expected

#### Usage Example
```rust
let test_cases = vec![
    ("  key: >1", "key", LineType::MappingKey),
    ("  another: >-2", "another", LineType::MappingKey),
];

run_folded_scalar_tests!(test_cases);
```

#### Notes
- Use this to **standardize assertions** across test functions
- Provides consistent error messages
- Handles both type and key name verification

---

### 3. Function: `create_folded_scalar_test`

**Location:** Lines ~127-136  
**Purpose:** Create a single folded scalar test case tuple

#### Signature
```rust
fn create_folded_scalar_test(
    indent: &str,
    key: &str,
    modifier: &str,
    indent_level: u32,
) -> (String, String, LineType)
```

#### Parameters
- **`indent`**: Base indentation (e.g., `"  "`, `"\t"`)
- **`key`**: The key name (e.g., `"my_key"`)
- **`modifier`**: Modifier pattern (e.g., `">"`, `">-"`, `">+"`)
- **`indent_level`**: Explicit indent number (1-9)

#### Returns
Tuple of `(line, key, LineType::MappingKey)`

#### Usage Example
```rust
let test_case = create_folded_scalar_test("  ", "description", ">", 2);
// Returns: ("  description: >2", "description", MappingKey)
```

#### Notes
- Use this for **individual test case creation**
- Provides a **non-macro alternative** to `generate_folded_explicit_indent_tests`
- Useful when you need custom key names or special cases

---

### 4. Function: `generate_folded_scalar_tests_multi_level`

**Location:** Lines ~140-169  
**Purpose:** Bulk generate folded scalar test cases for multiple indentation levels

#### Signature
```rust
fn generate_folded_scalar_tests_multi_level(
    keys: &[&str],
    modifiers: &[&str],
    indent_levels: &[u32],
) -> Vec<(String, String, LineType)>
```

#### Parameters
- **`keys`**: Array of key names (e.g., `&["text", "description"]`)
- **`modifiers`**: Array of modifier patterns (e.g., `&[">", ">-", ">+"]`)
- **`indent_levels`**: Array of indent numbers (e.g., `&[1, 2, 3, 4, 5]`)

#### Returns
Vector of test case tuples for **all combinations** of:
- 5 indentation levels (2-space, 4-space, 6-space, 8-space, tab)
- All provided keys
- All provided modifiers
- All provided indent levels

#### Usage Example
```rust
let test_cases = generate_folded_scalar_tests_multi_level(
    &["text", "content"],
    &[">", ">-", ">+"],
    &[1, 2, 3, 4, 5]
);
// Generates 60 test cases: 5 levels × 2 keys × 3 modifiers × 5 indent_nums
```

#### Notes
- Generates **comprehensive test suites** automatically
- Keys are auto-named with level prefix: `level1_text`, `tab_content`, etc.
- Ideal for **rapid prototyping** of test coverage

---

## When to Use Each Helper

### Decision Tree

```
Need to create test cases?
│
├─ Many similar cases at multiple levels?
│  └─ Use: generate_folded_scalar_tests_multi_level()
│     (Fastest for bulk generation)
│
├─ Cases at a single level with patterned keys?
│  └─ Use: generate_folded_explicit_indent_tests!()
│     (Macro with more control)
│
├─ Need individual custom cases?
│  └─ Use: create_folded_scalar_test()
│     (Non-macro, explicit control)
│
└─ Need to run tests with standard assertions?
   └─ Use: run_folded_scalar_tests!()
      (Consistent verification pattern)
```

---

## Implementation Examples

### Example 1: Quick Comprehensive Test

```rust
#[test]
fn test_literal_scalar_comprehensive() {
    let test_cases = generate_folded_scalar_tests_multi_level(
        &["text", "content", "description"],
        &["|", "|-", "|+"],
        &[1, 2, 3, 4, 5, 6, 7, 8, 9]
    );
    
    run_folded_scalar_tests!(test_cases);
}
```

### Example 2: Single Level with Custom Keys

```rust
#[test]
fn test_folded_scalar_level1_only() {
    let test_cases = generate_folded_explicit_indent_tests!(
        "  ",              // 2-space indent
        "level1",
        &[">", ">-", ">+"],
        &[1, 2, 3, 4, 5],
        "custom_key"
    );
    
    run_folded_scalar_tests!(test_cases);
}
```

### Example 3: Manual Test Cases with Helpers

```rust
#[test]
fn test_mixed_indentation_edge_cases() {
    let mut test_cases = vec![];
    
    // Custom tab+space cases
    test_cases.push(create_folded_scalar_test("\t  ", "tab_space", ">", 2));
    test_cases.push(create_folded_scalar_test("  \t", "space_tab", ">-", 3));
    
    // Add a few more manually
    test_cases.push(("\t\tdeep: >4".to_string(), "deep".to_string(), LineType::MappingKey));
    
    run_folded_scalar_tests!(test_cases);
}
```

---

## Limitations and Considerations

### Macro Limitations

1. **`generate_folded_explicit_indent_tests!`**
   - Always generates `MappingKey` type (not flexible for other LineTypes)
   - Auto-generates keys with specific pattern
   - Works only for folded scalars (`>`), not literal (`|`)

2. **`run_folded_scalar_tests!`**
   - Hardcoded assertion messages for "folded scalar"
   - Always checks both type and key (may not want key check for non-MappingKey)

### Function Limitations

1. **`create_folded_scalar_test`**
   - Returns hardcoded `LineType::MappingKey`
   - Works only for folded scalars

2. **`generate_folded_scalar_tests_multi_level`**
   - Limited to 5 predefined indentation levels
   - Returns hardcoded `LineType::MappingKey`
   - Works only for folded scalars

---

## Extending the Helpers

### For Literal Scalars (`|`)

Create analogous helpers:

```rust
macro_rules! generate_literal_explicit_indent_tests {
    ($indent:expr, $level_name:expr, $modifiers:expr, $indent_nums:expr, $key_prefix:expr) => {{
        let mut cases = vec![];
        
        for modifier in $modifiers.iter() {
            for num in $indent_nums.iter() {
                let modifier_str = format!("{}{}", modifier, num);
                let key_name = format!("{}_{}_{}", $key_prefix, modifier.replace("|", "").trim(), num);
                let line = format!("{}{}: {}", $indent, key_name, modifier_str);
                
                cases.push((line, key_name, armor::parsers::yaml::LineType::MappingKey));
            }
        }
        
        cases
    }};
}
```

### For Mixed Indentation

Modify helpers to accept mixed indentation patterns:

```rust
fn create_mixed_indent_test(
    prefix: &str,      // e.g., "\t  "
    key: &str,
    modifier: &str,
    indent_level: u32,
) -> (String, String, LineType) {
    let modifier_str = format!("{}{}", modifier, indent_level);
    let line = format!("{}{}: {}", prefix, key, modifier_str);
    (line, key.to_string(), LineType::MappingKey)
}
```

---

## Best Practices

1. **Use helpers for bulk generation:** Save time and reduce errors
2. **Prefer functions over macros:** When you need flexibility or debugging
3. **Standardize assertions:** Use `run_folded_scalar_tests!` for consistency
4. **Document custom patterns:** If you create new helpers, add documentation like this
5. **Test the helpers:** Verify generated test cases are correct before using them

---

## Verification

To test helper functionality:

```bash
# Run a test using the helpers
cargo test test_folded_scalar_explicit_indent_modifiers_at_various_levels

# Check if helpers compile correctly
cargo test --no-run

# Verify generated test cases (add debug output temporarily)
```

---

## References

- **Main Documentation:** `notes/bf-63gy6-folded-scalar-explicit-indent-test-infrastructure.md`
- **Test File:** `tests/type_like_string_false_positive_test.rs`
- **Section:** 12B Helper Utilities (lines ~60-170)

---

**End of Helper Reference**
