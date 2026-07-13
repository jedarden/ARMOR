# Folded Scalar Explicit Indent Test Infrastructure

**Bead ID:** bf-63gy6  
**Date:** 2026-07-13  
**Test File:** `tests/type_like_string_false_positive_test.rs`  
**Section:** 12B (Multiline String Scenarios with Exclamation Marks)

---

## Executive Summary

This document establishes the **test infrastructure and pattern** for folded scalar explicit indent modifier tests in Section 12B. The infrastructure is already implemented in `test_folded_scalar_explicit_indent_modifiers_at_various_levels()` and provides a comprehensive pattern that can be followed for other YAML scalar types.

---

## Existing Infrastructure

### Test Function Location

- **File:** `tests/type_like_string_false_positive_test.rs`
- **Function:** `test_folded_scalar_explicit_indent_modifiers_at_various_levels()`
- **Line Range:** ~7172-7389
- **Author:** bead bf-57cf0 (commit 3ac835ed)

### Test Coverage Summary

✅ **Comprehensive coverage of:**
- Folded scalar explicit indent modifiers: `>n`, `>-n`, `>+n` for n=1-9
- All base indentation levels: 2-space, 4-space, 6-space, 8-space, tab
- Keys with exclamation marks at all indentation levels
- Mixed indentation (tab + spaces)

---

## Explicit Indent Coverage Gap Analysis

### Current Level-Specific Test Coverage

#### ✅ Folded Scalar - 2-Space Level (COMPLETE)
- `test_folded_scalar_plain_explicit_indent_modifiers_at_2_space()` (line 8561)
- `test_folded_scalar_strip_explicit_indent_modifiers_at_2_space()` (line 8670)
- `test_folded_scalar_keep_explicit_indent_modifiers_at_2_space()` (line 13040)

#### ❌ Folded Scalar - Missing Level-Specific Tests
**4-Space Level:**
- `test_folded_scalar_plain_explicit_indent_modifiers_at_4_space()` - NOT IMPLEMENTED
- `test_folded_scalar_strip_explicit_indent_modifiers_at_4_space()` - NOT IMPLEMENTED
- `test_folded_scalar_keep_explicit_indent_modifiers_at_4_space()` - NOT IMPLEMENTED

**6-Space Level:**
- `test_folded_scalar_plain_explicit_indent_modifiers_at_6_space()` - NOT IMPLEMENTED
- `test_folded_scalar_strip_explicit_indent_modifiers_at_6_space()` - NOT IMPLEMENTED
- `test_folded_scalar_keep_explicit_indent_modifiers_at_6_space()` - NOT IMPLEMENTED

**8-Space Level:**
- `test_folded_scalar_plain_explicit_indent_modifiers_at_8_space()` - NOT IMPLEMENTED
- `test_folded_scalar_strip_explicit_indent_modifiers_at_8_space()` - NOT IMPLEMENTED
- `test_folded_scalar_keep_explicit_indent_modifiers_at_8_space()` - NOT IMPLEMENTED

**Tab Level:**
- `test_folded_scalar_plain_explicit_indent_modifiers_tab()` - NOT IMPLEMENTED
- `test_folded_scalar_strip_explicit_indent_modifiers_tab()` - NOT IMPLEMENTED
- `test_folded_scalar_keep_explicit_indent_modifiers_tab()` - NOT IMPLEMENTED

#### ❌ Literal Scalar - All Levels Missing (NO LEVEL-SPECIFIC TESTS)
**2-Space Level:**
- `test_literal_scalar_plain_explicit_indent_modifiers_at_2_space()` - NOT IMPLEMENTED
- `test_literal_scalar_strip_explicit_indent_modifiers_at_2_space()` - NOT IMPLEMENTED
- `test_literal_scalar_keep_explicit_indent_modifiers_at_2_space()` - NOT IMPLEMENTED

**4-Space, 6-Space, 8-Space, Tab Levels:** - ALL NOT IMPLEMENTED
(Repeat above patterns for each level)

---

### Bulk vs Level-Specific Testing

#### ✅ Bulk Tests (EXISTING)
- `test_folded_scalar_explicit_indent_modifiers_at_various_levels()` (line 8342)
- `test_literal_scalar_explicit_indent_modifiers_at_various_levels()` (line 8700)
- These tests provide comprehensive coverage across all levels in a single function

#### ❌ Level-Specific Tests (MISSING)
- Dedicated test functions for each indentation level
- Enables easier debugging of level-specific issues
- Follows Section 12B.3 pattern for granular testing
- Provides better test isolation and clearer failure messages

---

### Skeleton Template Reference

**Location:** `tests/type_like_string_false_positive_test.rs` (line 13153)

**Function:** `test_folded_scalar_explicit_indent_skeleton()`

**Usage:**
1. Copy the skeleton function at line 13153
2. Rename following pattern: `test_<scalar_type>_<modifier>_explicit_indent_<level>()`
3. Replace placeholder test cases with actual data
4. Update indentation level (e.g., `"    "` for 4-space, `"\t"` for tab)

**Example:**
```rust
#[test]
fn test_folded_scalar_plain_explicit_indent_modifiers_at_4_space() {
    let test_cases = vec![
        ("    text1: >1", "text1", LineType::MappingKey),
        ("    text2: >2", "text2", LineType::MappingKey),
        // ... more cases for n=1-9
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(result, expected_type, "...");
        // ... key validation
    }
}
```

---

### Recommended Test Additions (Section 12B.3 Pattern)

Following the Section 12B.3 infrastructure pattern (lines 12685-12700), add tests in this order:

**Priority 1 - Folded Scalar Level-Specific Tests:**
1. `test_folded_scalar_plain_explicit_indent_modifiers_at_4_space()`
2. `test_folded_scalar_strip_explicit_indent_modifiers_at_4_space()`
3. `test_folded_scalar_keep_explicit_indent_modifiers_at_4_space()`
4. Repeat for 6-space, 8-space, tab levels

**Priority 2 - Literal Scalar Level-Specific Tests:**
1. `test_literal_scalar_plain_explicit_indent_modifiers_at_2_space()`
2. `test_literal_scalar_strip_explicit_indent_modifiers_at_2_space()`
3. `test_literal_scalar_keep_explicit_indent_modifiers_at_2_space()`
4. Repeat for 4-space, 6-space, 8-space, tab levels

**Priority 3 - Continuation Line Validation:**
Add continuation line validation for each new level-specific test (following pattern at lines 13005-13029)

---

## Test Pattern Structure

### 1. Function Signature and Documentation

```rust
#[test]
fn test_folded_scalar_explicit_indent_modifiers_at_various_levels() {
    // Test folded scalars with explicit indent modifiers: >n, >-n, >+n for n=1-9
    // Tested at various base indentation levels: 2-space, 4-space, 6-space, 8-space, tab
    // This provides comprehensive coverage of explicit indent specification for folded scalars
```

**Pattern:**
- Clear descriptive function name: `test_{scalar_type}_{modifier_type}_at_various_levels()`
- Top-level comment explaining what's tested
- List of modifiers and indentation levels covered

### 2. Test Case Structure

```rust
let test_cases = vec![
    // ===== Level 1: 2-space indentation with explicit indent modifiers =====
    // Plain >n (n=1-9)
    ("  text1: >1", "text1", LineType::MappingKey),
    ("  text2: >2", "text2", LineType::MappingKey),
    // ... more cases
];
```

**Pattern:**
- Use `Vec<(line, expected_key, expected_type)>` for parameterized testing
- Organize by indentation level with clear section headers
- Use comment blocks to categorize test cases (plain, strip, keep modifiers)
- Include tuple elements:
  1. **Line:** The full YAML line to test
  2. **Expected key:** The key name that should be detected
  3. **Expected type:** The LineType that should be returned

### 3. Indentation Level Structure

```rust
// ===== Level 1: 2-space indentation with explicit indent modifiers =====
// ===== Level 2: 4-space indentation with explicit indent modifiers =====
// ===== Level 3: 6-space indentation with explicit indent modifiers =====
// ===== Level 4: 8-space indentation with explicit indent modifiers =====
// ===== Tab indentation with explicit indent modifiers =====
// ===== Mixed indentation (tab + spaces) =====
```

**Pattern:**
- Level 1: 2-space (  )
- Level 2: 4-space (    )
- Level 3: 6-space (      )
- Level 4: 8-space (        )
- Tab indentation: (\t)
- Mixed indentation: (\t  , \t    , etc.)

### 4. Modifier Categories

At each indentation level, test three categories:

#### A. Plain Modifier (>n for n=1-9)
```rust
// Plain >n (n=1-9)
("  text1: >1", "text1", LineType::MappingKey),
("  text2: >2", "text2", LineType::MappingKey),
// ... up to >9
```

#### B. Strip Modifier (>-n for n=1-9)
```rust
// Strip modifier >-n (n=1-9)
("  strip1: >-1", "strip1", LineType::MappingKey),
("  strip2: >-2", "strip2", LineType::MappingKey),
// ... up to >-9
```

#### C. Keep Modifier (>+n for n=1-9)
```rust
// Keep modifier >+n (n=1-9)
("  keep1: >+1", "keep1", LineType::MappingKey),
("  keep2: >+2", "keep2", LineType::MappingKey),
// ... up to >+9
```

#### D. Keys with Exclamation Marks
```rust
// Keys with exclamation marks at Level 1
("  key!1: >1", "key!1", LineType::MappingKey),
("  warn!2: >-2", "warn!2", LineType::MappingKey),
```

### 5. Test Execution Loop

```rust
for (line, expected_key, expected_type) in test_cases {
    let result = classify_line_type(line);
    assert_eq!(
        result, expected_type,
        "Folded scalar explicit indent modifier test failed: '{}' - expected {:?}, got {:?}",
        line, expected_type, result
    );

    // Verify that the key is correctly detected for MappingKey types
    if result == LineType::MappingKey {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key for folded scalar with explicit indent modifier: '{}'",
            line
        );
        let detected = info.unwrap();
        assert_eq!(
            detected.key, expected_key,
            "Key mismatch for folded scalar with explicit indent modifier: '{}' - expected '{}', got '{}'",
            line, expected_key, detected.key
        );
    }
}
```

**Pattern:**
- Iterate through all test cases
- Verify `LineType` classification
- For `MappingKey` types, also verify the detected key name
- Use descriptive error messages with the actual line content

---

## How to Follow This Pattern

### For Other Scalar Types

When implementing similar tests for other YAML constructs (literal scalars, mixed indentation, etc.), follow this structure:

#### 1. Function Naming
```rust
// Pattern: test_{construct}_{modifier_type}_at_various_levels()
fn test_literal_scalar_explicit_indent_modifiers_at_various_levels()
fn test_mixed_indentation_scenarios_with_folded_scalars()
fn test_mixed_indentation_scenarios_with_literal_scalars()
```

#### 2. Test Case Organization
```rust
let test_cases = vec![
    // ===== Level 1: 2-space indentation =====
    // Modifier variant 1
    // Modifier variant 2
    // Modifier variant 3
    
    // ===== Level 2: 4-space indentation =====
    // ...
];
```

#### 3. Tuple Structure
Use `Vec<(input, expected_key, expected_type)>` for parameterized testing:
- **input:** The line to test
- **expected_key:** The key name (or `None`/`"none"` if not applicable)
- **expected_type:** The LineType enum value

#### 4. Validation Loop
```rust
for (line, expected_key, expected_type) in test_cases {
    let result = classify_line_type(line);
    assert_eq!(result, expected_type, "Descriptive message with placeholders");
    
    // Additional validation for MappingKey types
    if result == LineType::MappingKey {
        let info = detect_mapping_key(line, 0);
        // Verify key detection
    }
}
```

---

## Potential Helper Macros

### Current State
No helper macros are currently used. The tests use explicit `vec![]` arrays and straightforward loops.

### Potential Improvements

#### Option 1: Test Case Generation Macro
```rust
macro_rules! explicit_indent_tests {
    ($indent:expr, $modifier:expr, $key_prefix:expr) => {
        // Generate test cases for n=1-9
    };
}
```

#### Option 2: Assertion Helper Macro
```rust
macro_rules! assert_line_classification {
    ($line:expr, $expected_type:expr, $expected_key:expr) => {
        let result = classify_line_type($line);
        assert_eq!(result, $expected_type, "Classification failed for: '{}'", $line);
        if result == LineType::MappingKey {
            let info = detect_mapping_key($line, 0);
            assert!(info.is_some(), "Should detect key for: '{}'", $line);
            assert_eq!(info.unwrap().key, $expected_key, "Key mismatch for: '{}'", $line);
        }
    };
}
```

**Recommendation:** While helper macros could reduce duplication, the current explicit pattern is clear and maintainable. Consider macros only if implementing many more similar test functions.

---

## Coverage Matrix

### Current Coverage (Folded Scalars with Explicit Indent)

| Modifier | Indent Levels | n Range | Total Cases |
|----------|---------------|---------|-------------|
| `>n` | 2, 4, 6, 8, tab, mixed | 1-9 | ~54 |
| `>-n` | 2, 4, 6, 8, tab, mixed | 1-9 | ~54 |
| `>+n` | 2, 4, 6, 8, tab, mixed | 1-9 | ~54 |
| Keys with `!` | All levels | Various | ~30 |
| **Total** | | | **~190+** |

---

## Future Test Implementations

Using this infrastructure pattern, implement:

### Immediate (High Priority)
- [ ] Literal scalars (`|`) with explicit indent modifiers at various levels
- [ ] Mixed indentation scenarios with literal scalars

### Medium Priority
- [ ] Double-quoted strings with mixed indentation
- [ ] Single-quoted strings with mixed indentation
- [ ] Flow style mappings with mixed indentation
- [ ] Flow style sequences with mixed indentation

### Low Priority
- [ ] Multi-line collections with mixed indentation
- [ ] Document markers with mixed indentation
- [ ] Comments with mixed indentation
- [ ] Anchors and aliases with mixed indentation
- [ ] Merge keys with mixed indentation

---

## Verification

To verify tests follow the correct pattern:

```bash
# Run the specific test
cargo test test_folded_scalar_explicit_indent_modifiers_at_various_levels

# Run all Section 12B tests
cargo test -- section-12b

# Check test coverage
cargo test -- --nocapture 2>&1 | grep -A5 "folded scalar"
```

---

## Key Takeaways for Following Beads

1. **Use parameterized testing:** Store test cases as `Vec<(line, expected_key, expected_type)>`
2. **Organize by indentation level:** Clear section headers for each level (2, 4, 6, 8, tab, mixed)
3. **Test all modifier variants:** Plain, strip, keep for n=1-9
4. **Include edge cases:** Keys with exclamation marks at each level
5. **Validate both type and key:** Check LineType and detected key name
6. **Use descriptive error messages:** Include the actual line content in assertions
7. **Document comprehensively:** Explain what's tested in the function header

---

## References

- **Commit:** 3ac835ed (bf-57cf0 implementation)
- **Test File:** `tests/type_like_string_false_positive_test.rs`
- **Section:** 12B (Multiline String Scenarios with Exclamation Marks)
- **Related Documentation:** `notes/bf-61srw-section-12b-mixed-indentation-test-gaps.md`

---

**End of Infrastructure Documentation**
