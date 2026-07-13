# Section 12B Folded Scalar Test Pattern Analysis

**Bead:** bf-4r3ae  
**Date:** 2026-07-13  
**Task:** Research existing test patterns for folded scalar tests in `type_like_string_false_positive_test.rs` Section 12B

---

## Overview

Section 12B focuses on multiline string scenarios with YAML folded scalars (`>`). The section is organized into multiple subsections:

- **12B:** Multiline String Scenarios with Exclamation Marks (basic folded scalar indicators)
- **12B.1:** Comprehensive Folded Block Scalar Tests with Exclamation
- **12B.2:** Basic Folded Scalar Indicator Tests
- **12B.3:** Folded Scalar Explicit Indent Infrastructure Pattern (template/macro-based)

---

## Test Pattern Structure

### 1. Basic Test Function Pattern

The fundamental test function structure for folded scalars follows this pattern:

```rust
#[test]
fn test_folded_block_scalar_with_<variant>() {
    // 1. Define test cases as vec of tuples: (line, expected_key, expected_type)
    let test_cases = vec![
        ("  text: >", "text", LineType::MappingKey),
        ("  warning: >-", "warning", LineType::MappingKey),
        ("    note: >+", "note", LineType::MappingKey),
    ];

    // 2. Iterate through test cases with standard assertions
    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        
        // Assert line type classification
        assert_eq!(result, expected_type, "test failed: '{}'", line);
        
        // If MappingKey, verify key detection
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(info.is_some(), "Should detect mapping key: '{}'", line);
            let detected = info.unwrap();
            assert_eq!(detected.key, expected_key, "Key mismatch: '{}'", line);
        }
    }
}
```

**Key characteristics of a good folded scalar test:**
1. Tests various indentation levels (2-space, 4-space, 6-space, 8-space, tab)
2. Tests all three modifiers: `>` (plain), `>-` (strip), `>+` (keep)
3. Tests explicit indent numbers (e.g., `>2`, `>-3`, `>+4`)
4. Includes exclamation marks in keys to test false-positive scenarios
5. Uses descriptive comments to group tests by indentation level
6. Uses `assert_eq!` with meaningful error messages

---

## Macro-Based Infrastructure Pattern (Section 12B.3)

Section 12B.3 provides a reusable macro infrastructure for generating folded scalar tests systematically.

### Macro: `generate_folded_explicit_indent_tests!`

**Purpose:** Generate test cases for folded scalars with explicit indent modifiers (`>-n`, `>+n`, `>n`)

**Signature:**
```rust
generate_folded_explicit_indent_tests!(
    $indent:expr,      // Base indentation (e.g., "  ", "\t")
    $level_name:expr,  // Descriptive name (e.g., "level1", "tab")
    $modifiers:expr,   // Array of modifiers: [">", ">-", ">+"]
    $indent_nums:expr, // Array of indent numbers: [1, 2, 3, 4]
    $key_prefix:expr   // Prefix for generated key names
)
```

**Example usage:**
```rust
let test_cases = generate_folded_explicit_indent_tests!(
    "  ",                    // 2-space base indentation
    "level1",               // Descriptive level name
    [">", ">-", ">+"],      // All three modifier types
    [1, 2, 3],             // Indent numbers 1-3
    "template"             // Key prefix for generated names
);
// Generates cases like:
// ("  template__1: >1", "template__1", LineType::MappingKey)
// ("  template_-_1: >-1", "template_-_1", LineType::MappingKey)
// ("  template_+_1: >+1", "template_+_1", LineType::MappingKey)
```

---

### Macro: `run_folded_scalar_tests!`

**Purpose:** Execute test cases with standard assertions (line type + key detection)

**Signature:**
```rust
run_folded_scalar_tests!($test_cases:expr)
```

**Example usage:**
```rust
run_folded_scalar_tests!(test_cases);
// Internally iterates and performs:
// 1. assert_eq!(result, expected_type, "...")
// 2. if MappingKey: detect_mapping_key() + assert key matches
```

---

### Helper Functions

#### `create_folded_scalar_test`

**Purpose:** Non-macro alternative for building individual test cases

**Signature:**
```rust
fn create_folded_scalar_test(
    indent: &str,
    key: &str,
    modifier: &str,
    indent_level: u32,
) -> (String, String, LineType)
```

**Example:**
```rust
cases.push(create_folded_scalar_test("  ", "my_key", ">", 2));
// Returns: ("  my_key: >2", "my_key", LineType::MappingKey)
```

---

#### `generate_folded_scalar_tests_multi_level`

**Purpose:** Bulk generate test cases for multiple indentation levels

**Signature:**
```rust
fn generate_folded_scalar_tests_multi_level(
    keys: &[&str],
    modifiers: &[&str],
    indent_levels: &[u32],
) -> Vec<(String, String, LineType)>
```

**Indentation levels used:**
- `"  "` (level1) - 2 spaces
- `"    "` (level2) - 4 spaces
- `"      "` (level3) - 6 spaces
- `"        "` (level4) - 8 spaces
- `"\t"` (tab) - Tab character

---

## Template Test Function Pattern

### Copy-Paste Template (from Section 12B.3)

```rust
#[test]
fn test_folded_scalar_explicit_indent_<variant>() {
    // Generate test cases for 2-space indentation with various modifiers
    let test_cases = generate_folded_explicit_indent_tests!(
        "  ",                    // indent: 2 spaces
        "level1",               // level_name: descriptive name
        [">", ">-", ">+"],      // modifiers: array of modifier patterns
        [1, 2, 3, 4, 5],       // indent_nums: array of indent numbers
        "test"                  // key_prefix: prefix for generated key names
    );

    // Run tests with standard assertions
    run_folded_scalar_tests!(test_cases);
}
```

---

## What Makes a Good Folded Scalar Test Pattern

### 1. **Comprehensive Indentation Coverage**
- Test all standard YAML indentation levels: 2, 4, 6, 8 spaces
- Test tab indentation
- Test mixed indentation (tab + spaces)

### 2. **Complete Modifier Coverage**
- `>` (plain folded scalar)
- `>-` (strip modifier - removes trailing newlines)
- `>+` (keep modifier - preserves trailing newlines)

### 3. **Explicit Indent Numbers**
- Test numbers 1-9 for all modifiers (e.g., `>1` through `>9`, `>-1` through `>-9`)
- Tests that the parser correctly handles the indent multiplier

### 4. **Exclamation Mark Handling**
- Keys with `!` at end: `key!`
- Keys with `!` in middle: `key!name`
- Keys with multiple `!`: `key!!name`, `key!bang!`
- Tests false-positive prevention (ensuring `!` doesn't trigger Tag classification)

### 5. **Consistent Assertion Pattern**
- Always verify line type classification
- For `MappingKey` type, always verify key detection
- Use descriptive error messages with the actual line content

### 6. **Clear Test Organization**
- Group by indentation level with comment headers
- Use descriptive test function names
- Document what specific scenario is being tested

---

## Continuation Line Testing

Folded scalars also test continuation lines (the content lines following the indicator):

```rust
let continuation_lines = vec![
    "  This is folded text with! exclamation marks",
    "    Multiple! exclamations! in! folded! style",
    "\tMore! content! with! bangs!",
];

for line in continuation_lines {
    let result = classify_line_type(line);
    assert!(
        result == LineType::MappingKey || result == LineType::Unknown,
        "Folded scalar continuation should be MappingKey or Unknown: '{}'",
        line
    );
}
```

---

## Related Beads

Based on the Section 12B.3 documentation, these beads are related to folded scalar test infrastructure:

- **bf-63gy6:** Folded Scalar Explicit Indent Infrastructure Pattern (Section 12B.3)
- **bf-45gyh:** Folded scalar strip explicit indent tests at 2-space
- **bf-5rzoh:** Folded scalar keep explicit indent tests at 2-space
- **bf-15c4t:** Related folded scalar work

---

## Summary

Section 12B demonstrates a well-structured, scalable approach to testing YAML folded scalar parsing:

1. **Basic tests** use manual `vec!` definitions for clarity and specific scenarios
2. **Macro infrastructure** provides systematic coverage of modifier × indent combinations
3. **Helper functions** offer flexibility for custom test generation
4. **Template functions** serve as copy-paste patterns for new tests
5. **Standard assertions** ensure consistent testing behavior across all tests

The pattern ensures comprehensive coverage of folded scalar syntax while maintaining test code readability and maintainability.
