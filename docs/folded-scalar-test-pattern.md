# Folded Scalar Test Structure Pattern

**Task ID**: bf-ht2h0  
**Title**: Document the folded scalar test structure pattern  
**Completed**: 2026-07-13

## Overview

This document describes the comprehensive testing infrastructure for YAML folded scalar (explicit indent) patterns in the ARMOR project. The pattern provides parameterized testing across multiple indentation levels, modifier types, and indent numbers.

## Pattern Summary

The folded scalar test structure provides three approaches for adding new tests:

1. **Macro-based generation** - Recommended for bulk test cases
2. **Helper function generation** - For dynamic/computed test cases
3. **Manual specification** - For specific edge cases

## Test Structure Components

### 1. Macro-Based Generation

#### `generate_folded_explicit_indent_tests!` Macro

Generates test cases for a specific indentation level with given modifiers and indent numbers.

**Parameters:**
- `$indent`: Base indentation (e.g., `"  "`, `"    "`, `"\t"`)
- `$level_name`: Descriptive name for this indentation level
- `$modifiers`: Array of modifier patterns (e.g., `[">", ">-", ">+"]`)
- `$indent_nums`: Array of indent numbers (e.g., `[1, 2, 3, 4, 5]`)
- `$key_prefix`: Prefix for generated key names

**Returns:** `Vec<(String, String, LineType)>` where each tuple is `(line, expected_key, expected_type)`

**Example:**
```rust
let test_cases = generate_folded_explicit_indent_tests!(
    "  ",                    // 2-space base indentation
    "level1",               // Descriptive level name
    [">", ">-", ">+"],      // All three modifier types
    [1, 2, 3],             // Indent numbers 1-3
    "template"             // Key prefix for generated names
);
// Generates test cases like:
// ("  template_1: >1", "template_1", LineType::MappingKey)
// ("  template_2: >2", "template_2", LineType::MappingKey)
// ("  template_-1: >-1", "template_-1", LineType::MappingKey)
// etc.
```

#### `run_folded_scalar_tests!` Macro

Executes test cases with standard assertions for line type classification and key detection.

**Assertions performed:**
1. Line type matches expected type
2. For `MappingKey` types, verifies key detection works correctly

**Example:**
```rust
run_folded_scalar_tests!(test_cases);
```

### 2. Helper Function Generation

#### `create_folded_scalar_test()`

Creates a single folded scalar test case tuple.

**Parameters:**
- `indent: &str` - Base indentation
- `key: &str` - Key name
- `modifier: &str` - Modifier pattern (`">"`, `">-"`, or `">+"`)
- `indent_level: u32` - Indent number (1-9)

**Returns:** `(String, String, LineType)` - `(line, key, type)` tuple

**Example:**
```rust
let test_case = create_folded_scalar_test("  ", "my_key", ">", 2);
// Returns: ("  my_key: >2", "my_key", LineType::MappingKey)
```

#### `generate_folded_scalar_tests_multi_level()`

Bulk generates test cases for multiple indentation levels (levels 1-4 and tab).

**Parameters:**
- `keys: &[&str]` - Array of key names
- `modifiers: &[&str]` - Array of modifiers
- `indent_levels: &[u32]` - Array of indent numbers

**Returns:** `Vec<(String, String, LineType)>`

**Levels covered:**
- Level 1: `"  "` (2 spaces)
- Level 2: `"    "` (4 spaces)
- Level 3: `"      "` (6 spaces)
- Level 4: `"        "` (8 spaces)
- Tab: `"\t"` (tab character)

**Example:**
```rust
let test_cases = generate_folded_scalar_tests_multi_level(
    &["text", "note", "message"],  // keys
    &[">", ">-", ">+"],           // modifiers
    &[1, 2, 3, 4]                 // indent levels
);
// Generates 60 cases: 5 levels × 3 keys × 3 modifiers × 4 indent numbers
```

#### `generate_folded_scalar_tests_all_levels()`

Comprehensive generation across ALL indentation levels including level 0 (no indentation).

**Parameters:**
- `keys: &[&str]` - Array of key names
- `modifiers: &[&str]` - Array of modifiers
- `indent_levels: &[u32]` - Array of indent numbers

**Returns:** `Vec<(String, String, LineType)>`

**Levels covered:**
- Level 0: `""` (no indentation)
- Level 1: `"  "` (2 spaces)
- Level 2: `"    "` (4 spaces)
- Level 3: `"      "` (6 spaces)
- Level 4: `"        "` (8 spaces)
- Tab: `"\t"` (tab character)

**Example:**
```rust
let test_cases = generate_folded_scalar_tests_all_levels(
    &["sample"],            // single key for all levels
    &[">"],                  // plain modifier only
    &[1, 2],                // two indent numbers
);
// Generates 12 cases: 6 levels × 2 indent numbers
```

#### `generate_folded_scalar_tests_for_level()`

Generates test cases for a SPECIFIC indent level only.

**Parameters:**
- `level: &str` - The indent level (`"level0"` through `"level4"`, or `"tab"`)
- `keys: &[&str]` - Array of key names
- `modifiers: &[&str]` - Array of modifiers
- `indent_nums: &[u32]` - Array of indent numbers

**Returns:** `Vec<(String, String, LineType)>`

**Level values:**
- `"level0"` → `""` (no indentation)
- `"level1"` → `"  "` (2 spaces)
- `"level2"` → `"    "` (4 spaces)
- `"level3"` → `"      "` (6 spaces)
- `"level4"` → `"        "` (8 spaces)
- `"tab"` → `"\t"` (tab character)

**Example:**
```rust
let test_cases = generate_folded_scalar_tests_for_level(
    "level0",               // no indentation
    &["text", "data"],
    &[">", ">-", ">+"],
    &[1, 2, 3, 4, 5],
);
// Generates 30 cases: 2 keys × 3 modifiers × 5 indent numbers
```

### 3. Manual Test Case Specification

For specific edge cases or custom test patterns, manually specify test cases as tuples:

**Format:** `(line_string, expected_key_string, expected_LineType)`

**Example:**
```rust
let test_cases = vec![
    // Level 1: 2-space indentation
    ("  text: >1", "text", LineType::MappingKey),
    ("  data: >2", "data", LineType::MappingKey),
    
    // Keys with special characters
    ("  key!name: >1", "key!name", LineType::MappingKey),
    ("  field_name: >2", "field_name", LineType::MappingKey),
    
    // Tab indentation
    ("\ttab_key: >1", "tab_key", LineType::MappingKey),
];

for (line, expected_key, expected_type) in test_cases {
    let result = classify_line_type(&line);
    assert_eq!(result, expected_type, "Test failed for: {}", line);
    
    if result == LineType::MappingKey {
        let info = detect_mapping_key(&line, 0);
        assert!(info.is_some(), "Should detect key for: {}", line);
        let detected = info.unwrap();
        assert_eq!(detected.key, expected_key, "Key mismatch for: {}", line);
    }
}
```

## Available Modifiers

Folded scalars support three modifier types:

1. **Plain `>`** - Default folded scalar behavior
   - Newlines are folded into spaces
   - Trailing newlines are handled per default YAML spec

2. **Strip `>-`** - Removes trailing newlines
   - Strips trailing newlines from content
   - Useful for clean text without trailing whitespace

3. **Keep `>+`** - Preserves trailing newlines
   - Keeps all trailing newlines as-is
   - Useful when exact whitespace preservation matters

## Indent Numbers

Explicit indent numbers range from 1 to 9:
- Format: `{modifier}{n}` where n ∈ [1, 9]
- Examples: `>1`, `>2`, `>-3`, `>+5`

**Content indentation calculation:**
- Content indentation = `indent_number × base_indent_level`
- Example: At 2-space base indent with `>2`, content starts at 4 spaces

## Indentation Levels

Six indentation levels are supported:

| Level | Indent String | Description |
|-------|---------------|-------------|
| 0     | `""`          | No indentation (key at document start) |
| 1     | `"  "`        | 2 spaces (most common) |
| 2     | `"    "`      | 4 spaces |
| 3     | `"      "`    | 6 spaces |
| 4     | `"        "`  | 8 spaces |
| Tab   | `"\t"`        | Tab character |

## Naming Conventions

Test function names should follow this pattern:

```
test_folded_scalar_{variant}_{details}()
```

**Examples:**
- `test_folded_scalar_plain_explicit_indent_modifiers_at_2_space()`
- `test_folded_scalar_strip_explicit_indent_at_level3()`
- `test_folded_scalar_keep_explicit_indent_tab()`
- `test_folded_scalar_level0_all_modifiers()`
- `test_folded_scalar_all_levels_comprehensive()`

## Complete Test Function Templates

### Template 1: Macro-Based Test (Recommended)

```rust
#[test]
fn test_folded_scalar_explicit_indent_<variant>() {
    // Generate test cases for specific indentation and modifiers
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

### Template 2: Helper Function Test

```rust
#[test]
fn test_folded_scalar_custom_pattern() {
    let mut cases = vec![];

    // Add custom test cases using helper function
    cases.push(create_folded_scalar_test("  ", "my_key", ">", 2));
    cases.push(create_folded_scalar_test("    ", "another", ">-", 3));
    cases.push(create_folded_scalar_test("\t", "tab_key", ">+", 1));

    // Run with standard macro
    run_folded_scalar_tests!(cases);
}
```

### Template 3: Multi-Level Bulk Test

```rust
#[test]
fn test_folded_scalar_multi_level() {
    let test_cases = generate_folded_scalar_tests_multi_level(
        &["text", "note", "message"],  // keys
        &[">", ">-", ">+"],           // modifiers
        &[1, 2, 3, 4]                 // indent levels
    );

    run_folded_scalar_tests!(test_cases);
}
```

### Template 4: Comprehensive All-Levels Test

```rust
#[test]
fn test_folded_scalar_comprehensive() {
    let test_cases = generate_folded_scalar_tests_all_levels(
        &["sample"],            // single key for all levels
        &[">"],                  // plain modifier only
        &[1, 2],                // two indent numbers
    );
    // Generates 12 cases: 6 levels × 2 indent numbers
    run_folded_scalar_tests!(test_cases);
}
```

### Template 5: Level-Specific Test

```rust
#[test]
fn test_folded_scalar_level0_only() {
    let test_cases = generate_folded_scalar_tests_for_level(
        "level0",               // no indentation
        &["text", "data"],
        &[">", ">-", ">+"],
        &[1, 2, 3, 4, 5],
    );
    run_folded_scalar_tests!(test_cases);
}
```

### Template 6: Manual Edge Case Test

```rust
#[test]
fn test_folded_scalar_edge_cases() {
    let test_cases = vec![
        // Specific edge cases that need manual testing
        ("  key!with!bangs: >1", "key!with!bangs", LineType::MappingKey),
        ("  _underscore_start: >2", "_underscore_start", LineType::MappingKey),
        ("  camelCaseKey: >3", "camelCaseKey", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(&line);
        assert_eq!(result, expected_type, "Test failed for: {}", line);
        
        if result == LineType::MappingKey {
            let info = detect_mapping_key(&line, 0);
            assert!(info.is_some(), "Should detect key for: {}", line);
            let detected = info.unwrap();
            assert_eq!(detected.key, expected_key, "Key mismatch for: {}", line);
        }
    }
}
```

## Adding New Test Cases

### Quick Start: Choose Your Approach

1. **For bulk coverage** → Use macro-based generation (Template 1 or 3)
2. **For dynamic cases** → Use helper functions (Template 2 or 5)
3. **For edge cases** → Use manual specification (Template 6)

### Step-by-Step Example

Let's add tests for 4-space indentation with strip modifier:

#### Step 1: Choose the template
Use Template 1 for macro-based generation.

#### Step 2: Define the test function
```rust
#[test]
fn test_folded_scalar_strip_explicit_indent_at_4_space() {
    // Test folded scalars with strip explicit indent >-n at 4-space level
}
```

#### Step 3: Generate test cases
```rust
let test_cases = generate_folded_explicit_indent_tests!(
    "    ",                 // 4-space indentation
    "level2",              // level2 = 4 spaces
    &[">-"],               // strip modifier only
    &[1, 2, 3, 4, 5],     // indent numbers 1-5
    "test"                // key prefix
);
```

#### Step 4: Run tests
```rust
run_folded_scalar_tests!(test_cases);
```

#### Complete function:
```rust
#[test]
fn test_folded_scalar_strip_explicit_indent_at_4_space() {
    // Test folded scalars with strip explicit indent >-n at 4-space level
    let test_cases = generate_folded_explicit_indent_tests!(
        "    ",                 // 4-space indentation
        "level2",              // level2 = 4 spaces
        &[">-"],               // strip modifier only
        &[1, 2, 3, 4, 5],     // indent numbers 1-5
        "test"                // key prefix
    );

    run_folded_scalar_tests!(test_cases);
}
```

## Test Coverage Guidelines

### Minimum Coverage
For each modifier type (`>`, `>-`, `>+`):
- Test at least 2 indentation levels (recommend level1 and level2)
- Test at least 3 indent numbers (e.g., 1, 3, 5)

### Comprehensive Coverage
For complete coverage:
- Test all 6 indentation levels (0, 1, 2, 3, 4, tab)
- Test all 3 modifier types
- Test all 9 indent numbers (1-9)
- Test edge cases with special characters in keys

### Edge Cases to Test
- Keys with exclamation marks (not at start - those are tags)
- Keys with underscores
- Keys with numbers
- CamelCase keys
- Very long keys
- Single character keys
- Tab indentation

## Related Documentation

- **YAML Module Design**: `/home/coding/ARMOR/docs/yaml-parser-module-design.md`
- **Line Type Classification**: `src/parsers/yaml/syntax_detector.rs`
- **Test File**: `tests/type_like_string_false_positive_test.rs`

## Related Beads

- **bf-2w54h**: Set up helper macros for parameterized folded scalar testing
- **bf-41ba1**: Add folded scalar explicit indent test skeleton
- **bf-63gy6**: Add folded scalar explicit indent test infrastructure
- **bf-45gyh**: Add folded scalar strip explicit indent tests at 2-space
- **bf-5rzoh**: Add folded scalar keep explicit indent tests at 2-space
- **bf-4r3ae**: Document folded scalar test patterns from Section 12B
