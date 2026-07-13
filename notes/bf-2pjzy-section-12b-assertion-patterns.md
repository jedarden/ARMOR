# Section 12B: Assertion Patterns and Coverage Gaps

**Generated:** 2026-07-13  
**Bead:** bf-2pjzy  
**Source:** `tests/type_like_string_false_positive_test.rs` (Section 12B, line 7824+)

---

## Overview

Section 12B tests multiline string scenarios (folded `>` and literal `|` block scalars) with exclamation marks. It demonstrates comprehensive testing patterns but has documented gaps in explicit indent modifier coverage.

---

## Assertion Patterns

### Pattern 1: Basic Classification Tests

**Structure:** `vec!` of input lines with `assert_eq!` assertions

```rust
let test_cases = vec![
    "description: >",
    "  folded_text: >",
    "    note: >",
];

for line in test_cases {
    let result = classify_line_type(line);
    assert_eq!(
        result,
        LineType::MappingKey,
        "Folded scalar indicator should be MappingKey: '{}'",
        line
    );
}
```

**Used in:**
- `test_folded_block_scalar_with_exclamation_marks()` (line 7828)
- `test_folded_scalar_indicator_lines()` (line 10451)
- `test_basic_folded_scalar_indicator_as_mapping_key()` (line 11110)

---

### Pattern 2: Tuple-Based Tests with Expected Key/Type

**Structure:** `vec!` of tuples `(line, expected_key_name, expected_line_type)`

```rust
let test_cases = vec![
    ("  level1_text: |-", "level1_text", LineType::MappingKey),
    ("  warning!msg: |-", "warning!msg", LineType::MappingKey),
    ("  error!log: |-", "error!log", LineType::MappingKey),
];

for (line, expected_key, expected_type) in test_cases {
    let result = classify_line_type(line);
    assert_eq!(result, expected_type, "...");
}
```

**Used in:**
- `test_literal_scalar_basic_modifiers_at_various_indentation_levels()` (line 7939)
- `test_folded_scalar_basic_modifiers_at_various_indentation_levels()` (line 8098)

---

### Pattern 3: Continuation Lines with Multiple Allowed Types

**Structure:** `vec!` of tuples `(line, vec![allowed_types])`

```rust
let continuation_lines = vec![
    ("  This is literal text with! exclamation marks", vec![LineType::MappingKey, LineType::Unknown]),
    ("    !Start! Middle! End!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
];

for (line, expected_types) in continuation_lines {
    let result = classify_line_type(line);
    assert!(
        expected_types.contains(&result),
        "Should be one of {:?}: '{}' (got {:?})",
        expected_types, line, result
    );
}
```

**Used in:**
- `test_literal_block_scalar_with_exclamation_marks()` (line 7888)
- `test_folded_scalar_continuation_lines_with_exclamation_marks()` (line 10708)
- `test_folded_scalar_with_continuation_content()` (line 11146)

---

### Pattern 4: Conditional Assertions Based on Line Content

**Structure:** Check line properties (e.g., starts with `!`) then assert accordingly

```rust
for line in continuation_lines {
    let result = classify_line_type(line);
    let starts_with_bang = line.trim().starts_with('!');
    if starts_with_bang {
        assert_eq!(result, LineType::Tag, "...");
    } else {
        assert!(result == LineType::MappingKey || result == LineType::Unknown, "...");
    }
}
```

**Used in:**
- `test_folded_scalar_continuation_lines_with_exclamation()` (line 10708)

---

### Pattern 5: Macro-Based Test Generation (Section 12B.3)

**Structure:** Use macros to generate test cases programmatically

```rust
let test_cases = generate_folded_explicit_indent_tests!(
    "  ",                    // 2-space base indentation
    "level1",               // Descriptive level name
    [">", ">-", ">+"],      // All three modifier types
    [1, 2, 3],             // Indent numbers 1-3
    "template"             // Key prefix
);

run_folded_scalar_tests!(test_cases);
```

**Used in:**
- `test_folded_scalar_explicit_indent_template_example()` (line 12715)
- Template for future explicit indent tests

---

## Test Coverage by Section

### Section 12B: Main Tests (line 7824)

✅ **Covered:**
- Folded scalars (`>`) with exclamation marks in keys
- Literal scalars (`|`) with exclamation marks in keys
- Basic modifiers (`>-`, `>+`, `|-`, `|+`)
- Continuation lines with `!` at various positions
- All indentation levels (2, 4, 6, 8 space + tab)
- Keys ending with `!`, multiple `!`, consecutive `!`

---

### Section 12B.1: Comprehensive Folded Tests (line 10653)

✅ **Covered:**
- Folded scalar indicator classification (all modifiers)
- Continuation lines with exclamation marks
- Exclamation at start, middle, end of lines
- Multiple consecutive exclamation marks
- Tab-indented continuation lines

---

### Section 12B.2: Indicator Line Tests (lines 10447, 11106)

✅ **Covered:**
- Basic indicators (`>`)
- Basic modifiers (`>-`, `>+`)
- Numeric modifiers (`>1` through `>9`, `>-1` through `>-9`, `>+1` through `>+4`)
- Continuation line patterns
- Whitespace variations

---

### Section 12B.3: Infrastructure Pattern (line 12581)

✅ **Covered:**
- Template test function
- Macro-based generation pattern
- Helper function documentation
- Level-specific testing pattern

---

## Documented Coverage Gaps

### ❌ **Gap 1: Folded Scalar Explicit Indent - Level-Specific Tests**

**Status:** Only Level 1 (2-space) has dedicated explicit indent tests

**Missing tests:**
```rust
// Level 2 (4-space) - NOT IMPLEMENTED
test_folded_scalar_plain_explicit_indent_modifiers_at_4_space()
test_folded_scalar_strip_explicit_indent_modifiers_at_4_space()
test_folded_scalar_keep_explicit_indent_modifiers_at_4_space()

// Level 3 (6-space) - NOT IMPLEMENTED
test_folded_scalar_*_explicit_indent_modifiers_at_6_space()

// Level 4 (8-space) - NOT IMPLEMENTED
test_folded_scalar_*_explicit_indent_modifiers_at_8_space()

// Tab indentation - PARTIAL (template exists)
test_folded_scalar_*_explicit_indent_modifiers_tab()
```

**Existing:** Only covered in "various_levels" bulk tests (line 8250+)

**Reference:** Lines 120-129 in documentation comments

---

### ❌ **Gap 2: Literal Scalar Explicit Indent - All Levels**

**Status:** Only bulk "various_levels" test exists

**Missing tests:**
```rust
// Level 1 (2-space) - NOT IMPLEMENTED
test_literal_scalar_plain_explicit_indent_modifiers_at_2_space()
test_literal_scalar_strip_explicit_indent_modifiers_at_2_space()
test_literal_scalar_keep_explicit_indent_modifiers_at_2_space()

// Levels 2, 3, 4, tab - NOT IMPLEMENTED
// (Repeat above patterns for 4-space, 6-space, 8-space, tab)
```

**Existing:** `test_literal_scalar_explicit_indent_modifiers_at_various_levels()` (line 8700)

**Reference:** Lines 134-139 in documentation comments

---

### ❌ **Gap 3: Per-Level Continuation Line Validation**

**Status:** Basic modifiers have level-specific continuation tests; explicit indent does not

**Missing:**
- Continuation line validation for explicit indent modifiers at each level
- Per-level exclamation mark patterns for explicit indent
- Dedicated test functions (not bulk "various_levels" tests)

**Reference:** Lines 145-149 in documentation comments

---

## Template for Adding Missing Tests

**Located at:** Line 12991 - `test_folded_scalar_explicit_indent_skeleton()`

**To add coverage, copy this pattern:**

```rust
#[test]
fn test_folded_scalar_<modifier>_explicit_indent_modifiers_at_<level>_space() {
    let test_cases = generate_folded_explicit_indent_tests!(
        "    ",                 // 4-space for level 2
        "level2",              // level name
        [">", ">-", ">+"],     // modifiers
        [1, 2, 3, 4, 5],      // indent numbers
        "test"                 // key prefix
    );
    
    run_folded_scalar_tests!(test_cases);
}
```

**Repeat for:**
- Modifiers: `plain`, `strip`, `keep`
- Levels: 2-space, 4-space, 6-space, 8-space, tab
- Scalar types: folded (`>`), literal (`|`)

---

## Summary Statistics

| Category | Covered | Missing | Coverage |
|----------|---------|---------|----------|
| Folded scalar basic modifiers | ✅ All levels | ❌ None | 100% |
| Folded scalar explicit indent | ✅ Level 1 only | ❌ Levels 2-4, tab | 20% |
| Literal scalar basic modifiers | ✅ All levels | ❌ None | 100% |
| Literal scalar explicit indent | ✅ Bulk only | ❌ All level-specific | 0% |
| Continuation line patterns | ✅ Comprehensive | ❌ Explicit indent per-level | 70% |

**Overall Section 12B Coverage:** ~58% (comprehensive in basic patterns, gaps in explicit indent)

---

## Recommendations

1. **Use Section 12B.3 infrastructure** (line 12581) for new tests
2. **Copy the skeleton template** (line 12991) for missing level-specific tests
3. **Follow naming convention:** `test_<scalar_type>_<modifier>_explicit_indent_<level>()`
4. **Add continuation line validation** for each new level-specific test
5. **Include exclamation mark patterns** in all new tests (keys ending with `!`, multiple `!`, etc.)

---

**Document end**
