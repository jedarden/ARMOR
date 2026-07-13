# Section 12B Test Structure and Construction Patterns Summary

**Bead ID:** bf-67wxa  
**Date:** 2026-07-13  
**Test File:** `tests/type_like_string_false_positive_test.rs`  
**Section:** 12B (lines 7824-12572)

---

## Executive Summary

Section 12B demonstrates comprehensive YAML folded and literal scalar test patterns across ~4,750 lines. The infrastructure provides three distinct approaches for test construction: manual specification, helper function generation, and macro-based generation. This summary extracts the key patterns and construction methods used throughout the section.

---

## Test Structure Overview

### Organization Pattern

```
Section 12B: Multiline String Scenarios with Exclamation Marks
├── Main folded/literal scalar tests (lines 7828-8036)
├── Basic modifier tests at various levels (lines 7939-8149)
├── Continuation line tests (lines 8152-10438)
├── Section 12B.1: Comprehensive folded block scalar tests (10653-11097)
├── Section 12B.2: Folded scalar indicator line tests (10447-10652)
└── Section 12B.3: Explicit indent infrastructure pattern (12581-12719+)
```

---

## Key Construction Patterns

### Pattern 1: Basic Manual Test Structure

**Best for:** Simple tests with clear, specific test cases

```rust
#[test]
fn test_folded_block_scalar_with_exclamation_marks() {
    let test_cases = vec![
        "description: >",               // Basic folded scalar
        "  folded_text: >",              // Indented folded scalar
        "\tmessage: >",                 // Tab-indented folded scalar
        "warning: >-",                  // Folded with strip modifier
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded block scalar indicator should be MappingKey: '{}'",
            line
        );
    }
}
```

**Characteristics:**
- Simple string array for test cases
- Direct iteration with basic assertions
- Descriptive error messages with placeholders
- Two-part structure: indicator lines + continuation lines

---

### Pattern 2: Parameterized Tuple Testing

**Best for:** Tests with expected key and type verification

```rust
#[test]
fn test_literal_scalar_basic_modifiers_at_various_indentation_levels() {
    let test_cases = vec![
        // Level 1: 2-space indentation with literal strip modifier (|-)
        ("  level1_text: |-", "level1_text", LineType::MappingKey),
        ("  warning!msg: |-", "warning!msg", LineType::MappingKey),

        // Level 2: 4-space indentation with literal keep modifier (|+)
        ("    level2_note: |+", "level2_note", LineType::MappingKey),
        ("    nested!info: |+", "nested!info", LineType::MappingKey),

        // Tab indentation with mixed modifiers
        ("\ttab_text: |-", "tab_text", LineType::MappingKey),
        ("\ttab!info: |+", "tab!info", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(result, expected_type, "Type mismatch for: '{}'", line);

        // Verify key detection for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(info.is_some(), "Should detect key for: '{}'", line);
            assert_eq!(info.unwrap().key, expected_key, "Key mismatch for: '{}'", line);
        }
    }
}
```

**Characteristics:**
- Tuple structure: `(input_line, expected_key, expected_type)`
- Multi-level verification: type classification + key detection
- Organized by indentation level with clear section headers
- Includes edge cases (exclamation marks, mixed indentation)

**Tuple Elements:**
1. **input_line** - Full YAML line to test
2. **expected_key** - Key name that should be detected
3. **expected_type** - LineType enum value expected

---

### Pattern 3: Macro-Based Generation

**Best for:** Bulk test generation with standardized patterns

#### 3a. Test Generation Macro

```rust
let test_cases = generate_folded_explicit_indent_tests!(
    "  ",                    // base: 2-space indentation
    "level1",               // level_name: descriptive identifier
    &[">", ">-", ">+"],     // modifiers: array of modifier patterns
    &[1, 2, 3, 4, 5],       // indent_nums: explicit indent numbers
    "test"                  // key_prefix: prefix for generated key names
);
// Generates: [("  test___1: >1", "test___1", MappingKey), ...]
```

#### 3b. Standardized Execution Macro

```rust
run_folded_scalar_tests!(test_cases);
```

**Characteristics:**
- Automatic test case generation for all combinations
- Standardized assertion pattern
- Consistent error messages
- Reduced code duplication

---

### Pattern 4: Helper Function Generation

**Best for:** Dynamic/computed test cases or custom patterns

#### Available Helper Functions

1. **`create_folded_scalar_test(indent, key, modifier, indent_level)`**
   ```rust
   let case = create_folded_scalar_test("  ", "my_key", ">", 2);
   // Returns: ("  my_key: >2", "my_key", LineType::MappingKey)
   ```

2. **`generate_folded_scalar_tests_multi_level(keys, modifiers, indent_levels)`**
   ```rust
   let cases = generate_folded_scalar_tests_multi_level(
       &["text", "content"],
       &[">", ">-", ">+"],
       &[1, 2, 3, 4, 5]
   );
   // Generates 60 cases: 5 levels × 2 keys × 3 modifiers × 5 indent_nums
   ```

3. **`generate_folded_scalar_tests_all_levels(keys, modifiers, indent_nums)`**
   ```rust
   let cases = generate_folded_scalar_tests_all_levels(
       &["sample"],
       &[">"],
       &[1, 2]
   );
   // Generates 12 cases: 6 levels × 2 indent_nums
   ```

4. **`generate_folded_scalar_tests_for_level(level_name, keys, modifiers, indent_nums)`**
   ```rust
   let cases = generate_folded_scalar_tests_for_level(
       "level0",
       &["text", "data"],
       &[">", ">-", ">+"],
       &[1, 2, 3, 4, 5]
   );
   ```

**Characteristics:**
- Non-macro alternatives for flexibility
- Better debugging capability (can inspect intermediate results)
- Reusable across different test contexts
- Composable patterns

---

## Modifier System

### Modifier Types

| Modifier | Pattern | Description | Example |
|----------|---------|-------------|---------|
| Plain | `>`, `\|` | Default behavior | `"text: >"` |
| Strip | `>-`, `\|-` | Removes trailing newlines | `"text: >-"` |
| Keep | `>+`, `\|+` | Preserves trailing newlines | `"text: >+"` |
| Explicit Indent | `>n`, `>-n`, `>+n` | Specifies indentation level (n=1-9) | `"text: >2"` |

### Explicit Indent Numbers

- **Range:** 1-9
- **Meaning:** `n × base_indent_level` spaces
- **Example:** At 2-space base with `>2`, content indent = 4 spaces

---

## Indentation Level System

### Standard Levels

| Level | Indent | Name | Usage |
|-------|--------|------|-------|
| 0 | `""` | level0 | Document root |
| 1 | `"  "` | level1 | Most common (2-space) |
| 2 | `"    "` | level2 | Nested (4-space) |
| 3 | `"      "` | level3 | Deep nested (6-space) |
| 4 | `"        "` | level4 | Very deep (8-space) |
| Tab | `"\t"` | tab | Tab-based indentation |
| Mixed | `"\t  "`, `"\t    "` | mixed | Tab + spaces combinations |

### Level Organization Pattern

```rust
let test_cases = vec![
    // ===== Level 1: 2-space indentation =====
    ("  key: >1", "key", LineType::MappingKey),

    // ===== Level 2: 4-space indentation =====
    ("    key: >2", "key", LineType::MappingKey),

    // ===== Level 3: 6-space indentation =====
    ("      key: >3", "key", LineType::MappingKey),

    // ===== Tab indentation =====
    ("\tkey: >1", "key", LineType::MappingKey),

    // ===== Mixed indentation =====
    ("\t  key: >1", "key", LineType::MappingKey),
];
```

---

## Assertion Patterns

### Type Classification Assertion

```rust
assert_eq!(
    result, expected_type,
    "Expected type mismatch for: '{}'",
    line
);
```

### Key Detection Assertion (for MappingKey types)

```rust
if result == LineType::MappingKey {
    let info = detect_mapping_key(line, 0);
    assert!(
        info.is_some(),
        "Should detect mapping key for: '{}'",
        line
    );
    let detected = info.unwrap();
    assert_eq!(
        detected.key, expected_key,
        "Key mismatch for: '{}' - expected '{}', got '{}'",
        line, expected_key, detected.key
    );
}
```

### Flexible Type Assertion (continuation lines)

```rust
assert!(
    result == LineType::MappingKey || result == LineType::Unknown,
    "Continuation should be MappingKey or Unknown: '{}'",
    line
);
```

### Multi-Type Assertion (vector of acceptable types)

```rust
let expected_types = vec![LineType::Tag, LineType::MappingKey, LineType::Unknown];
assert!(
    expected_types.contains(&result),
    "Should be one of {:?}: '{}' (got {:?})",
    expected_types, line, result
);
```

---

## Naming Conventions

### Test Function Names

```rust
test_folded_scalar_explicit_indent_modifiers_at_various_levels()
test_literal_scalar_basic_modifiers_at_various_indentation_levels()
test_folded_scalar_continuation_lines_with_exclamation_marks()
test_literal_scalar_continuation_lines_with_exclamation_marks()
```

**Pattern:** `test_{scalar_type}_{modifier_type}_{details}()`

- **scalar_type:** `folded_scalar`, `literal_scalar`
- **modifier_type:** `explicit_indent`, `basic_modifiers`, `continuation_lines`
- **details:** `at_various_levels`, `with_exclamation_marks`

### Key Naming (Generated)

```rust
"level1_text", "level2_note", "tab_info", "mixed_tab_spaces"
```

**Pattern:** `{level_name}_{key_name}` or `{level_name}_{modifier}_{num}`

---

## Edge Case Coverage

### Exclamation Mark Patterns

```rust
// Keys ending with !
"  key!bang!: >",           // Multiple ! in key
"    deep!nest!ed: >",       // ! at different positions
"\ttab!tab!key: >",         // Tab with multiple !

// Multiple consecutive !
"    multiple!!!here: >",   // Consecutive !
"  spaced!out!keys!: >",    // Multiple spaced !

// ! with special positions
"  !important!: >",         // ! at start of value
"    !!double!: >",         // Double ! at start
```

### Indentation Edge Cases

```rust
// Mixed tab + spaces
"\t  two_space_tab!key: >",    // Tab + 2 spaces
"\t    four_space_tab!key: >",  // Tab + 4 spaces

// Single character keys
"  a!: |-",
"    b!: |+",
"\tc!: >",

// Very deep indentation
"          deep10: >",           // 10 spaces (beyond level 4)
"\t\t\t\ttab4: >",              // 4 tabs
```

---

## Template Test Functions

### Template 1: Basic Manual Test

```rust
#[test]
fn test_{variant}() {
    let test_cases = vec![
        "{line}",
        "{line}",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(result, LineType::MappingKey, "Test failed: '{}'", line);
    }
}
```

### Template 2: Parameterized Test

```rust
#[test]
fn test_{variant}_{level}() {
    let test_cases = vec![
        ("  {key}: >1", "{key}", LineType::MappingKey),
        ("  {key}: >2", "{key}", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(result, expected_type, "Type mismatch: '{}'", line);

        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(info.is_some(), "Key detection failed: '{}'", line);
            assert_eq!(info.unwrap().key, expected_key, "Key mismatch: '{}'", line);
        }
    }
}
```

### Template 3: Macro-Generated Test

```rust
#[test]
fn test_{variant}_{level}() {
    let test_cases = generate_folded_explicit_indent_tests!(
        "  ",                    // indent
        "level1",               // level_name
        &[">", ">-", ">+"],     // modifiers
        &[1, 2, 3, 4, 5],       // indent_nums
        "test"                  // key_prefix
    );

    run_folded_scalar_tests!(test_cases);
}
```

### Template 4: Helper Function Test

```rust
#[test]
fn test_{variant}_{level}() {
    let mut cases = vec![];

    // Build custom cases
    cases.push(create_folded_scalar_test("  ", "key1", ">", 1));
    cases.push(create_folded_scalar_test("    ", "key2", ">-", 2));

    // Or add manually
    cases.push(("\tkey3: >3".to_string(), "key3".to_string(), LineType::MappingKey));

    run_folded_scalar_tests!(cases);
}
```

---

## Coverage Metrics

### Section 12B Coverage

| Category | Variants | Levels | Total Cases |
|----------|----------|--------|-------------|
| Folded scalars basic | >, >-, >+ | 6 | ~72 |
| Folded scalars explicit indent | >n, >-n, >+n (n=1-9) | 6 | ~162 |
| Literal scalars basic | \|, \|-, \|+ | 6 | ~72 |
| Literal scalars explicit indent | \|n, \|-n, \|+n (n=1-9) | 6 | ~162 |
| Continuation lines | Various | 6 | ~150 |
| Edge cases | ! patterns, mixed indent | Various | ~80 |
| **Total** | | | **~700+** |

---

## Best Practices

### 1. Choose the Right Pattern

- **Simple tests →** Manual Pattern 1
- **Structured tests →** Parameterized Pattern 2
- **Bulk generation →** Macro Pattern 3
- **Custom/dynamic →** Helper Pattern 4

### 2. Organize by Indentation Level

```rust
let test_cases = vec![
    // ===== Level 1 =====
    // cases here

    // ===== Level 2 =====
    // cases here
];
```

### 3. Use Descriptive Error Messages

```rust
assert_eq!(result, expected, "Context: '{}' - expected {:?}, got {:?}", line, expected, result);
```

### 4. Test Both Type and Key

```rust
// Always verify type
assert_eq!(result, expected_type, "...");

// For MappingKey, also verify key
if result == LineType::MappingKey {
    assert!(detect_mapping_key(line).is_some(), "...");
}
```

### 5. Include Edge Cases

- Exclamation marks in various positions
- Mixed indentation (tab + spaces)
- Single character keys
- Very deep indentation
- Consecutive special characters

---

## Summary

Section 12B demonstrates a mature, comprehensive test infrastructure for YAML scalar parsing. The patterns range from simple manual tests to sophisticated macro-based generation, providing flexibility for different testing scenarios. The key takeaways are:

1. **Parameterized testing** with tuple structure for structured verification
2. **Macro-based generation** for bulk test case creation
3. **Helper functions** for flexible, composable test patterns
4. **Comprehensive coverage** across modifiers, levels, and edge cases
5. **Standardized assertions** with descriptive error messages
6. **Clear organization** by indentation level and modifier type

These patterns can be applied to other YAML construct testing (flow collections, document markers, anchors/aliases) by adapting the modifier and level systems appropriately.

---

**End of Section 12B Test Structure and Construction Patterns Summary**
