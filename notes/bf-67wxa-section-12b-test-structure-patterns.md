# Section 12B Test Structure and Construction Patterns

**Bead ID:** bf-67wxa  
**Date:** 2026-07-13  
**Section:** 12B (Multiline String Scenarios with Exclamation Marks)  
**Test File:** `tests/type_like_string_false_positive_test.rs`

---

## Executive Summary

Section 12B provides a comprehensive testing infrastructure for YAML folded scalar explicit indent patterns. The infrastructure offers **three distinct approaches** for constructing tests, covering multiple indentation levels, modifier types, and edge cases. This document extracts the core construction patterns that can be reused for other YAML scalar types.

---

## Core Test Structure Pattern

### 1. Parameterized Testing Foundation

All tests follow a **tuple-based parameterized pattern**:

```rust
let test_cases = vec![
    (line_string, expected_key_string, expected_LineType),
    // ("  text: >1", "text", LineType::MappingKey),
];
```

**Tuple Structure:**
1. **`line_string`** - Complete YAML line to test
2. **`expected_key_string`** - Key name that should be extracted
3. **`expected_LineType`** - Expected classification result

### 2. Function Naming Convention

```rust
test_{scalar_type}_{modifier_type}_at_various_levels()
```

**Examples:**
- `test_folded_scalar_explicit_indent_modifiers_at_various_levels()`
- `test_literal_scalar_explicit_indent_modifiers_at_various_levels()`
- `test_mixed_indentation_scenarios_with_folded_scalars()`

### 3. Test Execution Loop Pattern

```rust
for (line, expected_key, expected_type) in test_cases {
    // 1. Verify line type classification
    let result = classify_line_type(line);
    assert_eq!(result, expected_type, "Descriptive message with: '{}'", line);
    
    // 2. For MappingKey types, verify key detection
    if result == LineType::MappingKey {
        let info = detect_mapping_key(line, 0);
        assert!(info.is_some(), "Should detect key for: '{}'", line);
        let detected = info.unwrap();
        assert_eq!(detected.key, expected_key, "Key mismatch for: '{}'", line);
    }
}
```

---

## Three Construction Approaches

### Approach 1: Macro-Based Generation (Recommended for Bulk Tests)

#### Generate Test Cases Macro

```rust
let test_cases = generate_folded_explicit_indent_tests!(
    "  ",                    // Base indentation
    "level1",               // Descriptive level name
    [">", ">-", ">+"],      // Modifiers to test
    [1, 2, 3, 4, 5],       // Indent numbers
    "template"             // Key prefix
);
```

**Parameters:**
- `$indent`: Base indentation string (e.g., `"  "`, `"    "`, `"\t"`)
- `$level_name`: Descriptive name for this indentation level
- `$modifiers`: Array of modifier patterns
- `$indent_nums`: Array of indent numbers (1-9)
- `$key_prefix`: Prefix for generated key names

#### Run Tests Macro

```rust
run_folded_scalar_tests!(test_cases);
```

**Assertions performed:**
1. Line type matches expected type
2. For `MappingKey` types, verifies key detection

### Approach 2: Helper Function Generation

#### Single Test Case Creation

```rust
let case = create_folded_scalar_test("  ", "my_key", ">", 2);
// Returns: ("  my_key: >2", "my_key", LineType::MappingKey)
```

**Parameters:**
- `indent: &str` - Base indentation
- `key: &str` - Key name
- `modifier: &str` - Modifier pattern (`">"`, `">-"`, or `"">+"`)
- `indent_level: u32` - Indent number (1-9)

#### Bulk Multi-Level Generation

```rust
let test_cases = generate_folded_scalar_tests_multi_level(
    &["text", "note", "message"],  // Keys
    &[">", ">-", ">+"],           // Modifiers
    &[1, 2, 3, 4]                 // Indent numbers
);
// Generates 60 cases: 5 levels × 3 keys × 3 modifiers × 4 indent numbers
```

**Levels covered:**
- Level 1: `"  "` (2 spaces)
- Level 2: `"    "` (4 spaces)
- Level 3: `"      "` (6 spaces)
- Level 4: `"        "` (8 spaces)
- Tab: `"\t"` (tab character)

#### Specific Level Generation

```rust
let test_cases = generate_folded_scalar_tests_for_level(
    "level0",               // Specific level only
    &["text", "data"],
    &[">", ">-", ">+"],
    &[1, 2, 3, 4, 5],
);
```

**Level values:**
- `"level0"` → `""` (no indentation)
- `"level1"` → `"  "` (2 spaces)
- `"level2"` → `"    "` (4 spaces)
- `"level3"` → `"      "` (6 spaces)
- `"level4"` → `"        "` (8 spaces)
- `"tab"` → `"\t"` (tab character)

#### All-Levels Generation

```rust
let test_cases = generate_folded_scalar_tests_all_levels(
    &["sample"],            // Keys
    &[">"],                  // Modifiers
    &[1, 2]                 // Indent numbers
);
// Generates 12 cases: 6 levels × 2 indent numbers
```

**Levels covered:** All 6 levels including level 0

### Approach 3: Manual Specification (For Edge Cases)

```rust
let test_cases = vec![
    // Level 1: 2-space indentation
    ("  text: >1", "text", LineType::MappingKey),
    ("  data: >2", "data", LineType::MappingKey),
    
    // Keys with special characters
    ("  key!name: >1", "key!name", LineType::MappingKey),
    
    // Tab indentation
    ("\ttab_key: >1", "tab_key", LineType::MappingKey),
];
```

---

## Indentation Level Structure

### Organization Pattern

Tests are organized by indentation level with clear section headers:

```rust
let test_cases = vec![
    // ===== Level 1: 2-space indentation with explicit indent modifiers =====
    // Plain >n (n=1-9)
    // Strip >-n (n=1-9)
    // Keep >+n (n=1-9)
    
    // ===== Level 2: 4-space indentation with explicit indent modifiers =====
    // ...
];
```

### Indentation Levels Table

| Level | Indent String | Name | Spaces |
|-------|---------------|------|--------|
| 0     | `""`          | level0 | 0 spaces |
| 1     | `"  "`        | level1 | 2 spaces |
| 2     | `"    "`      | level2 | 4 spaces |
| 3     | `"      "`    | level3 | 6 spaces |
| 4     | `"        "`  | level4 | 8 spaces |
| Tab   | `"\t"`        | tab | Tab character |

### Mixed Indentation

Tests also cover mixed indentation patterns:
- `"\t  "` - tab + 2 spaces
- `"\t    "` - tab + 4 spaces
- etc.

---

## Modifier Categories Pattern

At each indentation level, test three modifier categories:

### 1. Plain Modifier (>n for n=1-9)

```rust
// Plain >n (n=1-9)
("  text1: >1", "text1", LineType::MappingKey),
("  text2: >2", "text2", LineType::MappingKey),
// ... up to >9
```

**Behavior:** Standard folded scalar - newlines folded into spaces

### 2. Strip Modifier (>-n for n=1-9)

```rust
// Strip >-n (n=1-9)
("  strip1: >-1", "strip1", LineType::MappingKey),
("  strip2: >-2", "strip2", LineType::MappingKey),
// ... up to >-9
```

**Behavior:** Removes trailing newlines from content

### 3. Keep Modifier (>+n for n=1-9)

```rust
// Keep >+n (n=1-9)
("  keep1: >+1", "keep1", LineType::MappingKey),
("  keep2: >+2", "keep2", LineType::MappingKey),
// ... up to >+9
```

**Behavior:** Preserves all trailing newlines as-is

### 4. Keys with Exclamation Marks

```rust
// Keys with exclamation marks at Level 1
("  key!1: >1", "key!1", LineType::MappingKey),
("  warn!2: >-2", "warn!2", LineType::MappingKey),
```

**Note:** These test keys containing `!` characters (not YAML tags, which start with `!!`)

---

## Complete Test Function Templates

### Template 1: Macro-Based (Recommended)

```rust
#[test]
fn test_folded_scalar_explicit_indent_<variant>() {
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

### Template 2: Helper Function

```rust
#[test]
fn test_folded_scalar_custom_pattern() {
    let mut cases = vec![];

    cases.push(create_folded_scalar_test("  ", "my_key", ">", 2));
    cases.push(create_folded_scalar_test("    ", "another", ">-", 3));
    cases.push(create_folded_scalar_test("\t", "tab_key", ">+", 1));

    run_folded_scalar_tests!(cases);
}
```

### Template 3: Multi-Level Bulk

```rust
#[test]
fn test_folded_scalar_multi_level() {
    let test_cases = generate_folded_scalar_tests_multi_level(
        &["text", "note", "message"],
        &[">", ">-", ">+"],
        &[1, 2, 3, 4]
    );

    run_folded_scalar_tests!(test_cases);
}
```

### Template 4: Manual Edge Cases

```rust
#[test]
fn test_folded_scalar_edge_cases() {
    let test_cases = vec![
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

---

## Coverage Matrix

### Current Coverage (Section 12B Folded Scalars)

| Modifier | Indent Levels | n Range | Total Cases |
|----------|---------------|---------|-------------|
| `>n` | 2, 4, 6, 8, tab, mixed | 1-9 | ~54 |
| `>-n` | 2, 4, 6, 8, tab, mixed | 1-9 | ~54 |
| `>+n` | 2, 4, 6, 8, tab, mixed | 1-9 | ~54 |
| Keys with `!` | All levels | Various | ~30 |
| **Total** | | | **~190+** |

### Recommended Coverage for New Tests

**Minimum Coverage:**
- At least 2 indentation levels (recommend level1 and level2)
- At least 3 indent numbers (e.g., 1, 3, 5)
- All 3 modifier types

**Comprehensive Coverage:**
- All 6 indentation levels (0, 1, 2, 3, 4, tab)
- All 3 modifier types
- All 9 indent numbers (1-9)
- Edge cases with special characters in keys

---

## Key Takeaways

### For Following the Pattern

1. **Use parameterized testing:** Store test cases as `Vec<(line, expected_key, expected_type)>`
2. **Organize by indentation level:** Clear section headers for each level
3. **Test all modifier variants:** Plain, strip, keep for n=1-9
4. **Include edge cases:** Keys with special characters at each level
5. **Validate both type and key:** Check LineType and detected key name
6. **Use descriptive error messages:** Include the actual line content in assertions
7. **Document comprehensively:** Explain what's tested in the function header

### Pattern Selection Guide

- **Bulk coverage** → Use macro-based generation (Approach 1)
- **Dynamic/computed cases** → Use helper functions (Approach 2)
- **Specific edge cases** → Use manual specification (Approach 3)

---

## Applicability to Other YAML Constructs

This infrastructure pattern can be followed for other YAML scalar types:

### Immediate Applications
- Literal scalars (`|`) with explicit indent modifiers
- Mixed indentation scenarios with literal scalars
- Double-quoted strings with mixed indentation
- Single-quoted strings with mixed indentation

### Medium Priority
- Flow style mappings with mixed indentation
- Flow style sequences with mixed indentation
- Multi-line collections with mixed indentation

### Pattern Extension
For each new YAML construct, create:
1. Helper macros matching the pattern (e.g., `generate_literal_explicit_indent_tests!`)
2. Helper functions (e.g., `create_literal_scalar_test()`)
3. Test execution macro (e.g., `run_literal_scalar_tests!`)
4. Comprehensive documentation following this template

---

## Verification Commands

```bash
# Run the specific test
cargo test test_folded_scalar_explicit_indent_modifiers_at_various_levels

# Run all Section 12B tests
cargo test -- section-12b

# Check test coverage
cargo test -- --nocapture 2>&1 | grep -A5 "folded scalar"
```

---

## References

- **Commit:** 3ac835ed (bf-57cf0 implementation)
- **Test File:** `tests/type_like_string_false_positive_test.rs`
- **Section:** 12B (Multiline String Scenarios with Exclamation Marks)
- **Infrastructure Documentation:** 
  - `notes/bf-63gy6-folded-scalar-explicit-indent-test-infrastructure.md`
  - `tests/folded_scalar_test_infrastructure.md`
  - `docs/folded-scalar-test-pattern.md`

---

**End of Pattern Documentation**
