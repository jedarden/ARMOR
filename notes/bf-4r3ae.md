# Research: Folded Scalar Test Patterns in Section 12B

**Bead:** bf-4r3ae  
**Date:** 2026-07-13  
**Purpose:** Document existing test patterns for folded scalars to guide future test development

## Overview

Section 12B of `tests/type_like_string_false_positive_test.rs` contains comprehensive tests for YAML folded block scalars (using the `>` indicator). These tests verify that folded scalars are correctly classified as `MappingKey` and that continuation lines are handled appropriately.

---

## Test Pattern Structure

### 1. Basic Test Case Format

All folded scalar tests use a consistent tuple structure:

```rust
(input_line, expected_key_name, expected_line_type)
```

**Example:**
```rust
("  text: >-", "text", LineType::MappingKey)
```

- `input_line`: The YAML line being tested (includes indentation, key, and folded scalar indicator)
- `expected_key_name`: The key name that should be extracted
- `expected_line_type`: Should be `LineType::MappingKey` for folded scalar indicator lines

---

### 2. Two-Part Assertion Pattern

Every test follows a two-part assertion pattern:

**First Assertion - Line Type Classification:**
```rust
let result = classify_line_type(line);
assert_eq!(result, expected_type, "...");
```

**Second Assertion - Key Detection (if MappingKey):**
```rust
if result == LineType::MappingKey {
    let info = detect_mapping_key(line, 0);
    assert!(info.is_some(), "Should detect mapping key: '{}'", line);
    let detected = info.unwrap();
    assert_eq!(detected.key, expected_key, "Key mismatch...");
}
```

---

### 3. Folded Scalar Indicator Patterns

Folded scalars use the `>` indicator with three modifier types:

| Modifier | Meaning |
|----------|---------|
| `>`      | Plain folded scalar |
| `>-`     | Folded with strip modifier (removes trailing newlines) |
| `>+`     | Folded with keep modifier (preserves trailing newlines) |

**Explicit Indent Modifiers:**
- `>n`, `>-n`, `>+n` where n = 1-9
- Example: `>-2` means "folded scalar with strip modifier, 2×indent = 4 spaces"

---

### 4. Indentation Levels

Tests cover 5 standard indentation levels:

| Level | Indentation | Name |
|-------|-------------|------|
| 1 | `"  "` (2 spaces) | level1 |
| 2 | `"    "` (4 spaces) | level2 |
| 3 | `"      "` (6 spaces) | level3 |
| 4 | `"        "` (8 spaces) | level4 |
| Tab | `"\t"` | tab |
| Mixed | `"\t  "`, `"\t    "` | mixed |

---

## What Makes a Good Folded Scalar Test

### 1. Comprehensive Coverage

Good tests cover:
- **All modifier types:** `>`, `>-`, `>+`
- **All explicit indent levels:** 1-9 (e.g., `>1` through `>9`)
- **All base indentation levels:** 2-space, 4-space, 6-space, 8-space, tab
- **Edge cases:** Keys with `!`, single-char keys, multiple `!` in keys
- **Continuation lines:** Lines following the indicator line

### 2. Test Naming Convention

Tests follow a descriptive naming pattern:
```
test_folded_scalar_<variant>_<modifier_type>_at_<indentation>
```

**Examples:**
- `test_folded_scalar_basic_modifiers_at_various_indentation_levels`
- `test_folded_scalar_plain_explicit_indent_modifiers_at_2_space`
- `test_folded_scalar_strip_explicit_indent_modifiers_at_2_space`

### 3. Continuation Line Testing

Continuation lines (lines following the folded scalar indicator) use a different pattern:

```rust
// Accept multiple possible types for continuation lines
let continuation_lines = vec![
    ("  Content line with! text", vec![LineType::MappingKey, LineType::Unknown]),
];

for (line, expected_types) in continuation_lines {
    let result = classify_line_type(line);
    assert!(
        expected_types.contains(&result),
        "Should be one of {:?}: '{}' (got {:?})",
        expected_types, line, result
    );
    
    // Continuation lines should NOT detect as mapping keys
    let info = detect_mapping_key(line, 0);
    assert!(info.is_none(), "Should NOT detect mapping key: '{}'", line);
}
```

---

## Macro and Helper Function Patterns

### 1. Generation Macro

**`generate_folded_explicit_indent_tests!`** - Auto-generates test cases:

```rust
let test_cases = generate_folded_explicit_indent_tests!(
    "  ",                    // indent: 2 spaces
    "level1",               // level_name: descriptive name
    [">", ">-", ">+"],      // modifiers: array of modifier patterns
    [1, 2, 3, 4, 5],       // indent_nums: array of indent numbers
    "test"                  // key_prefix: prefix for generated key names
);
```

**Output:** `Vec<(String, String, LineType)>`

### 2. Runner Macro

**`run_folded_scalar_tests!`** - Executes test cases with standard assertions:

```rust
run_folded_scalar_tests!(test_cases);
```

This macro automatically:
1. Iterates over all test cases
2. Calls `classify_line_type()` for each
3. Asserts line type matches expected
4. If `MappingKey`, calls `detect_mapping_key()` and asserts key is correct

### 3. Helper Functions

**`create_folded_scalar_test()`** - Creates a single test case tuple:
```rust
fn create_folded_scalar_test(
    indent: &str,
    key: &str,
    modifier: &str,
    indent_level: u32,
) -> (String, String, LineType)
```

**`generate_folded_scalar_tests_multi_level()`** - Bulk generates tests across all indentation levels:
```rust
fn generate_folded_scalar_tests_multi_level(
    keys: &[&str],
    modifiers: &[&str],
    indent_levels: &[u32],
) -> Vec<(String, String, LineType)>
```

---

## Test Organization by Section

### Section 12B.1: Basic Folded Block Scalars
- Location: Lines 6889-6947
- Tests: Basic `>`, `>-`, `>+` indicators with `!` in keys
- Coverage: Simple cases, continuation lines

### Section 12B.2: Comprehensive Tests
- Location: Lines 7165-7327
- Tests: Basic modifiers at all indentation levels
- Coverage: Level 1-4, tab, mixed indentation, keys with `!`

### Section 12B.3: Explicit Indent Tests
- Location: Lines 7330-7655
- Tests: Explicit indent modifiers `>n`, `>-n`, `>+n` for n=1-9
- Coverage: All combinations, split into focused tests (plain, strip, keep)

### Section 12B.4: Infrastructure Pattern
- Location: Lines 11642-11840
- Documentation: Template patterns for future tests
- Examples: Shows how to use macros and helpers

---

## Key Learnings

1. **Consistency is critical:** All tests use the same tuple structure and two-assertion pattern
2. **Coverage matters:** Good tests test all modifier types, all indent levels, and edge cases
3. **Infrastructure exists:** Use the provided macros and helpers instead of writing raw test cases
4. **Continuation lines differ:** They accept multiple possible types and should NOT detect as mapping keys
5. **Documentation is integrated:** Section 12B.3 serves as both documentation and executable template

---

## References

- File: `/home/coding/ARMOR/tests/type_like_string_false_positive_test.rs`
- Lines 21-174: Macro and helper definitions with pattern documentation
- Lines 6885-11840: Section 12B folded scalar tests
- Related bead: bf-63gy6 (infrastructure pattern setup)

---

**Next Steps:** When creating new folded scalar tests, copy the template from Section 12B.3 and modify for the specific variant being tested.
