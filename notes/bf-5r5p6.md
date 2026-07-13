# Section 12B Test Structure Analysis

**Bead ID:** bf-5r5p6
**Task:** Explore Section 12B test structure in type_like_string_false_positive_test.rs
**Date:** 2026-07-13

## Overview

Section 12B is a comprehensive test suite for **multiline string scenarios with exclamation marks** in YAML. It spans lines 6728-9241 (2513 lines) and focuses on folded block scalar (`>`) syntax with exclamation marks in various positions.

## Structure

Section 12B is divided into three main subsections:

### 12B: Multiline String Scenarios with Exclamation Marks (Lines 6728-7456)

**Main tests:**
1. `test_folded_block_scalar_with_exclamation_marks` (lines 6732-6790)
   - Tests folded scalar indicators (`>`) with various modifiers
   - Tests continuation lines with exclamation marks

2. `test_literal_block_scalar_with_exclamation_marks` (lines 6792-6900)
   - Tests literal scalar indicators (`|`) with modifiers
   - Tests continuation lines with exclamation marks

**Pattern used:**
- `vec!` of test cases as strings
- Loop through each case and assert classification
- Separate test case arrays for indicator lines vs continuation lines
- Uses `classify_line_type()` function

### 12B.2: Folded Scalar Indicator Line Tests (Lines 7457-7662)

**Main tests:**
1. `test_folded_scalar_indicator_lines` (lines 7461-7483)
   - Basic `>` indicators with various key names

2. `test_folded_scalar_basic_modifiers` (lines 7485-7511)
   - Tests strip (`>-`) and keep (`>+`) modifiers

3. `test_folded_scalar_numeric_modifiers` (lines 7513-7557)
   - Tests numeric modifiers `>1` through `>9`
   - Tests `>-n` and `>+n` combinations

4. `test_folded_scalar_indented_indicators` (lines 7559-7600)
   - Tests 2-space, 4-space, tab, and mixed indentation

5. `test_folded_scalar_all_modifier_combinations` (lines 7602-7662)
   - Comprehensive test of all valid modifier combinations

**Pattern used:**
- Similar to 12B - vec! of strings with loop assertions
- Focuses on indicator line classification only

### 12B.1: Comprehensive Folded Block Scalar Tests with Exclamation (Lines 7663-8115)

**Main tests:**
1. `test_folded_scalar_indicator_classification` (lines 7667-7715)
   - Indicator lines at various indentation levels
   - All folded scalar indicator types

2. `test_folded_scalar_continuation_lines_with_exclamation` (lines 7717-7789)
   - Continuation lines with exclamation marks
   - Distinguishes Tag lines (starting with `!`) from content

3. `test_tab_indented_folded_scalars_with_exclamation` (lines 7791-7842)
   - Tab-indented folded scalars with continuation lines
   - Tests `detect_mapping_key()` extraction

4. `test_folded_scalar_various_indentation_levels` (lines 7844-7911)
   - Tests 0, 2, 4, 6, 8 space indentation
   - Tests tab indentation
   - Tests mixed spaces + tabs
   - Uses tuple pattern: `(line, expected_key, expected_value, should_detect)`

5. `test_folded_scalar_modifiers_comprehensive` (lines 7913-8004)
   - Tests all modifier types (>, >-, >+, >n, >-n, >+n)
   - Tests explicit indent levels 1-9

6. `test_folded_scalar_exclamation_positions_comprehensive` (lines 8006-8113)
   - Tests exclamation at start, end, middle, both ends
   - Tests multiple consecutive exclamation marks
   - Tests exclamation with numbers, special characters, quotes

**Pattern used:**
- More complex test case structure with tuples
- Tests both `classify_line_type()` and `detect_mapping_key()`
- Includes expected key/value extraction validation

### 12B.2: Basic Folded Scalar Indicator Tests (Lines 8116-9241)

**Main tests:**
1. `test_basic_folded_scalar_indicator_as_mapping_key` (lines 8120-8153)
   - Basic `>` indicator classification
   - Whitespace variations

2. `test_folded_scalar_with_continuation_content` (lines 8155-8199)
   - Indicator + following content line pairs
   - Tests both indicator and continuation classification

3. `test_folded_scalar_continuation_lines_with_exclamation_marks` (lines 8201-8266)
   - Continuation lines with exclamation marks
   - Tests different folded scalar indicators with continuations

4. `test_folded_scalar_continuation_lines_starting_with_exclamation` (lines 8268-8306)
   - Edge case: continuation lines starting with `!`
   - These are classified as Tag (syntactically correct YAML)

5. `test_folded_scalar_continuation_exclamation_various_contexts` (lines 8308-8392)
   - Exclamation in CSS-like, URL-like, regex-like contexts
   - Natural language, error messages, configuration values
   - Ensures continuation lines are NOT classified as Tag

6. `test_comprehensive_tab_indented_folded_scalars_with_exclamation` (lines 8394-8492)
   - Single, double, triple tab indentation
   - Tab-indented continuation lines
   - Tab-indented lines starting with `!` (should be Tag)

7. `test_comprehensive_various_indentation_levels_with_exclamation` (lines 8494-8582)
   - Tests 0, 2, 4, 6, 8, 10, 12 space indentation
   - Tests single, double, triple tab indentation
   - Uses tuple pattern: `(line, expected_type, should_detect_key, expected_key)`

8. `test_mixed_indentation_scenarios_with_folded_scalars` (lines 8584-8698)
   - Tab followed by spaces
   - Spaces followed by tab
   - Continuation lines with mixed indentation
   - Lines starting with `!` in mixed indentation

9. `test_odd_indentation_levels_with_exclamation_marks` (lines 8700-8796)
   - Tests 1, 3, 5, 7, 9, 11 space indentation
   - Odd indentation with folded scalar modifiers
   - Odd indentation with exclamation in various positions

10. `test_deep_indentation_levels_with_exclamation_marks` (lines 8798-8888)
    - Tests 14, 16, 18, 20, 24 space indentation
    - Deep indentation with folded scalar modifiers
    - Deep indentation with exclamation at various positions

11. `test_extensive_tab_indentation_with_exclamation_marks` (lines 8890-8972)
    - Tests 4, 5, 6 tabs indentation
    - Deep tab indentation with folded scalar modifiers
    - Deep tab indentation with exclamation at various positions

12. `test_complex_mixed_indentation_with_exclamation_marks` (lines 8974-9063)
    - Tab followed by multiple spaces
    - Multiple spaces followed by tab
    - Alternating tabs and spaces
    - Complex mixed indentation with modifiers

13. `test_various_indentation_levels_with_exclamation_mark` (lines 9065-9240)
    - **Most comprehensive indentation test**
    - Tests 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 16 space indentation
    - Tests single, double, triple tab indentation
    - Tests tab + spaces combinations
    - Distinguishes Tag lines (starting with `!`) from MappingKey

## Indentation Levels Currently Covered

### Space Indentation (by level):
- 0 spaces (root level)
- 1 space (odd)
- 2 spaces
- 3 spaces (odd)
- 4 spaces
- 5 spaces (odd)
- 6 spaces
- 7 spaces (odd)
- 8 spaces
- 9 spaces (odd)
- 10 spaces
- 11 spaces (odd)
- 12 spaces
- 14 spaces (deep)
- 16 spaces (deep)
- 18 spaces (very deep)
- 20 spaces (extreme)
- 24 spaces (ultra deep)

### Tab Indentation:
- Single tab (`\t`)
- Double tab (`\t\t`)
- Triple tab (`\t\t\t`)
- 4 tabs (`\t\t\t\t`)
- 5 tabs (`\t\t\t\t\t`)
- 6 tabs (`\t\t\t\t\t\t`)

### Mixed Tab + Spaces:
- Tab + 1 space
- Tab + 2 spaces
- Tab + 4 spaces
- Tab + 6 spaces
- 1 space + tab
- 2 spaces + tab
- 4 spaces + tab
- Tab + space + tab
- 2 spaces + tab + 2 spaces
- And more complex combinations

## Test Pattern Summary

### Pattern 1: Simple String Vector
```rust
let test_cases = vec![
    "key: >",
    "name: >",
    // ...
];
for line in test_cases {
    let result = classify_line_type(line);
    assert_eq!(result, LineType::MappingKey, "...", line);
}
```

### Pattern 2: Tuple Vector (for `detect_mapping_key` tests)
```rust
let test_cases: Vec<(&str, Option<&str>, Option<&str>, bool)> = vec![
    ("key: >", Some("key"), None, true),
    ("  content!", None, None, false),
];
for (line, expected_key, expected_value, should_detect) in test_cases {
    let info = detect_mapping_key(line, 0);
    // assertions based on should_detect
}
```

### Pattern 3: Classification + Detection Tuple
```rust
let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
    ("key: >", LineType::MappingKey, true, Some("key")),
    ("  content!", LineType::MappingKey, false, None),
];
for (line, expected_type, should_detect_key, expected_key) in test_cases {
    let result = classify_line_type(line);
    // classification assertions
    let info = detect_mapping_key(line, 0);
    // detection assertions
}
```

### Pattern 4: Indicator + Continuation Pairs
```rust
let test_cases = vec![
    ("description: >", "  This is folded content"),
    ("text: >", "  Line with content"),
];
for (indicator_line, content_line) in test_cases {
    // Test indicator line
    let indicator_result = classify_line_type(indicator_line);
    // Test content line
    let content_result = classify_line_type(content_line);
}
```

## Key Findings

1. **Comprehensive coverage:** Section 12B is extremely thorough, covering:
   - All folded scalar modifiers (`>`, `>-`, `>+`, `>n`, `>-n`, `>+n`)
   - All indentation levels from 0-24 spaces
   - All tab combinations from 1-6 tabs
   - Mixed tab + space combinations
   - Exclamation marks at all positions (start, middle, end, multiple)

2. **Consistent pattern:** Tests use `classify_line_type()` as primary classification
   - Continuation lines accept either `LineType::MappingKey` or `LineType::Unknown`
   - Lines starting with `!` are correctly classified as `LineType::Tag`

3. **Two function approach:**
   - `classify_line_type()` - for line classification
   - `detect_mapping_key()` - for key/value extraction with indentation context

4. **Edge cases covered:**
   - Lines starting with `!` (Tag vs continuation ambiguity)
   - Odd indentation levels
   - Very deep indentation (14-24 spaces, 4-6 tabs)
   - Mixed tabs and spaces
   - Exclamation in various contexts (CSS, URLs, regex, natural language)

## Implementation Notes

For adding new indentation tests, follow the most comprehensive pattern (Pattern 3):
```rust
let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
    // (line, expected_type, should_detect_key, expected_key)
];
```

This pattern validates:
1. Correct line type classification
2. Correct mapping key detection behavior
3. Correct key extraction when detected

## Conclusion

Section 12B provides a robust, comprehensive test suite for folded block scalar scenarios with exclamation marks. The coverage is extremely thorough, with systematic testing of:
- All modifier combinations
- All realistic indentation levels (0-24 spaces, 1-6 tabs)
- All exclamation mark positions
- Edge cases (odd indentation, deep nesting, mixed indentation)

The test patterns are consistent and reusable, making it easy to add new test cases following the established structure.
