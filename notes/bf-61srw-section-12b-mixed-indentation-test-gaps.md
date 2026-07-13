# Section 12B Mixed Indentation Test Gaps Analysis

**Bead ID:** bf-61srw  
**Date:** 2026-07-13  
**Test File:** `tests/type_like_string_false_positive_test.rs`

## Executive Summary

Section 12B ("Multiline String Scenarios with Exclamation Marks") has comprehensive coverage for **folded scalars (>)** with mixed indentation (tabs + spaces combinations), but has **significant gaps** for other YAML constructs with mixed indentation.

The most critical gap is the absence of a corresponding test for **literal scalars (|)** with mixed indentation patterns.

---

## Current Coverage Overview

### Existing Mixed Indentation Tests

| Test Name | Line Range | Coverage | Indentation Patterns Tested |
|-----------|------------|----------|----------------------------|
| `test_mixed_indentation_scenarios_with_folded_scalars` | 9424-9537 | ✅ Folded (>) only | Tab+spaces, spaces+tab, modifiers, continuations |
| `test_complex_mixed_indentation_with_exclamation_marks` | 9814-9902 | ✅ Folded (>) only | Alternating tabs/spaces, complex patterns |
| `test_various_indentation_levels_with_exclamation_mark` | 9905-10064+ | ✅ All types | 2-12+ spaces, tabs, tab+space combos |

### Mixed Indentation Patterns Already Covered

✅ **Tab + Spaces combinations:**
- `\t ` (tab + 1 space)
- `\t  ` (tab + 2 spaces)
- `\t    ` (tab + 4 spaces)
- ` \t` (1 space + tab)
- `  \t` (2 spaces + tab)
- `    \t` (4 spaces + tab)

✅ **Alternating patterns:**
- `\t \t ` (tab, space, tab)
- `  \t  ` (spaces, tab, spaces)
- `\t    \t  ` (tab, spaces, tab, spaces)

✅ **With folded scalar modifiers:**
- `>-` (strip)
- `>+` (keep)
- `>2`, `>-2` (explicit indent)

✅ **Continuation lines with '!' in mixed indentation**
✅ **Tag detection with mixed indentation (lines starting with `!`)**

---

## Critical Gaps

### 🔴 Gap #1: Literal Scalars (|) with Mixed Indentation

**Severity:** HIGH  
**Missing Test:** `test_mixed_indentation_scenarios_with_literal_scalars`

**Impact:** Literal scalars (preserve newlines with `|`) have different parsing rules than folded scalars (convert newlines to spaces with `>`). Mixed indentation may behave differently.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_scenarios_with_literal_scalars() {
    let test_cases = vec![
        // Tab followed by spaces
        ("\t literal1: |", LineType::MappingKey, true, Some("literal1")),
        ("\t  literal2: |", LineType::MappingKey, true, Some("literal2")),
        ("\t    literal3: |", LineType::MappingKey, true, Some("literal3")),

        // Spaces followed by tab
        (" \tliteral4: |", LineType::MappingKey, true, Some("literal4")),
        ("  \tliteral5: |", LineType::MappingKey, true, Some("literal5")),
        ("    \tliteral6: |", LineType::MappingKey, true, Some("literal6")),

        // Continuation lines with mixed indentation and exclamation marks
        ("\t Literal! content! here!", LineType::Unknown, false, None),
        ("\t  More! mixed! indentation!", LineType::Unknown, false, None),

        // With modifiers
        ("\t literal: |-", LineType::MappingKey, true, Some("literal")),
        ("  \tliteral: |+", LineType::MappingKey, true, Some("literal")),
        ("\t  literal: |2", LineType::MappingKey, true, Some("literal")),

        // Tags in mixed indentation
        ("\t !important", LineType::Tag, false, None),
        ("  \t!value", LineType::Tag, false, None),
    ];
}
```

---

### 🟡 Gap #2: Double-Quoted Strings with Mixed Indentation

**Severity:** MEDIUM  
**Missing Test:** `test_mixed_indentation_with_double_quoted_strings`

**Impact:** Double-quoted strings allow escape sequences and have special '!' handling.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_double_quoted_strings() {
    let test_cases = vec![
        ("\t key1: \"value! with! bangs!\"", LineType::MappingKey, true, Some("key1")),
        ("  \tkey2: \"quoted! string! here!\"", LineType::MappingKey, true, Some("key2")),
        ("\t  key3: \"escaped \\! exclamation\"", LineType::MappingKey, true, Some("key3")),
        
        // Flow style in double quotes with mixed indentation
        ("\t flow: \"key1: !value1, key2: !value2\"", LineType::MappingKey, true, Some("flow")),
    ];
}
```

---

### 🟡 Gap #3: Single-Quoted Strings with Mixed Indentation

**Severity:** MEDIUM  
**Missing Test:** `test_mixed_indentation_with_single_quoted_strings`

**Impact:** Single-quoted strings have different escaping rules.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_single_quoted_strings() {
    let test_cases = vec![
        ("\t key1: 'value! with! bangs!'", LineType::MappingKey, true, Some("key1")),
        ("  \tkey2: 'single! quoted! string!'", LineType::MappingKey, true, Some("key2")),
        ("\t  key3: ''!empty! with! bangs!'", LineType::MappingKey, true, Some("key3")),
    ];
}
```

---

### 🟡 Gap #4: Flow Style Mappings with Mixed Indentation

**Severity:** MEDIUM  
**Missing Test:** `test_mixed_indentation_with_flow_style_mappings`

**Impact:** Flow style uses `{}` for inline mappings.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_flow_style_mappings() {
    let test_cases = vec![
        ("\t mapping: {key1: !value1, key2: !value2}", LineType::MappingKey, true, Some("mapping")),
        ("  \tflow: {nested: {key: !value}}", LineType::MappingKey, true, Some("flow")),
        ("\t  inline: {!tag: value, key!bang: !another}", LineType::MappingKey, true, Some("inline")),
    ];
}
```

---

### 🟡 Gap #5: Flow Style Sequences with Mixed Indentation

**Severity:** MEDIUM  
**Missing Test:** `test_mixed_indentation_with_flow_style_sequences`

**Impact:** Flow style uses `[]` for inline sequences.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_flow_style_sequences() {
    let test_cases = vec![
        ("\t items: [item1!, item2!, item3!]", LineType::MappingKey, true, Some("items")),
        ("  \tlist: [nested! [item!], simple!]", LineType::MappingKey, true, Some("list")),
        ("\t  values: [!tag, value!, !another]", LineType::MappingKey, true, Some("values")),
    ];
}
```

---

### 🟢 Gap #6: Multi-Line Collections with Mixed Indentation

**Severity:** LOW  
**Missing Test:** `test_mixed_indentation_with_multiline_collections`

**Impact:** Multi-line sequences and mappings with mixed indentation.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_multiline_collections() {
    let test_cases = vec![
        // Multi-line sequences
        ("\t - item1!", LineType::SequenceItem, false, None),
        ("  \t- item! with! bangs!", LineType::SequenceItem, false, None),
        
        // Multi-line mappings
        ("\t ? !key1", LineType::MappingKey, false, None),
        ("  \t: !value1", LineType::MappingKey, false, None),
    ];
}
```

---

### 🟢 Gap #7: Document Markers with Mixed Indentation

**Severity:** LOW  
**Missing Test:** `test_mixed_indentation_with_document_markers`

**Impact:** Document start/end markers with mixed indentation.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_document_markers() {
    let test_cases = vec![
        ("\t ---", LineType::DocumentMarker, false, None),
        ("  \t---", LineType::DocumentMarker, false, None),
        ("\t  ...", LineType::DocumentMarker, false, None),
        ("  \t...", LineType::DocumentMarker, false, None),
    ];
}
```

---

### 🟢 Gap #8: Comments with Mixed Indentation

**Severity:** LOW  
**Missing Test:** `test_mixed_indentation_with_comments`

**Impact:** Comment lines (#) with '!' in them might be misclassified.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_comments() {
    let test_cases = vec![
        ("\t # Comment with! exclamation!", LineType::Comment, false, None),
        ("  \t# Another! comment! here!", LineType::Comment, false, None),
        ("\t  # !Important! note!", LineType::Comment, false, None),
    ];
}
```

---

### 🟢 Gap #9: Anchors and Aliases with Mixed Indentation

**Severity:** LOW  
**Missing Test:** `test_mixed_indentation_with_anchors_and_aliases`

**Impact:** YAML anchors (&) and aliases (*) with mixed indentation.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_anchors_and_aliases() {
    let test_cases = vec![
        ("\t anchor: &default !value", LineType::MappingKey, true, Some("anchor")),
        ("  \talias: *default", LineType::MappingKey, true, Some("alias")),
        ("\t  key: &anchor! !value!", LineType::MappingKey, true, Some("key")),
    ];
}
```

---

### 🟢 Gap #10: Merge Keys with Mixed Indentation

**Severity:** LOW  
**Missing Test:** `test_mixed_indentation_with_merge_keys`

**Impact:** Merge keys (<<) with mixed indentation.

**Suggested Test Cases:**
```rust
fn test_mixed_indentation_with_merge_keys() {
    let test_cases = vec![
        ("\t <<: *anchor!", LineType::MappingKey, false, None),
        ("  \t<<: {!merge: !value}", LineType::MappingKey, false, None),
        ("\t  merge: <<", LineType::MappingKey, true, Some("merge")),
    ];
}
```

---

## Test Plan

### Phase 1: Critical - Add Missing Literal Scalar Tests (HIGH)

**Action:** Create `test_mixed_indentation_scenarios_with_literal_scalars()`

This test should mirror `test_mixed_indentation_scenarios_with_folded_scalars()` but for literal scalars (`|`).

**Estimated Lines:** ~150 lines  
**Priority:** P0 (Critical)

### Phase 2: Medium Priority - String Types (MEDIUM)

**Action:** Create tests for quoted strings with mixed indentation:
- `test_mixed_indentation_with_double_quoted_strings()`
- `test_mixed_indentation_with_single_quoted_strings()`

**Estimated Lines:** ~100 lines total  
**Priority:** P1 (Medium)

### Phase 3: Medium Priority - Flow Style (MEDIUM)

**Action:** Create tests for flow style with mixed indentation:
- `test_mixed_indentation_with_flow_style_mappings()`
- `test_mixed_indentation_with_flow_style_sequences()`

**Estimated Lines:** ~100 lines total  
**Priority:** P1 (Medium)

### Phase 4: Low Priority - Edge Cases (LOW)

**Action:** Create tests for less common YAML features:
- `test_mixed_indentation_with_multiline_collections()`
- `test_mixed_indentation_with_document_markers()`
- `test_mixed_indentation_with_comments()`
- `test_mixed_indentation_with_anchors_and_aliases()`
- `test_mixed_indentation_with_merge_keys()`

**Estimated Lines:** ~150 lines total  
**Priority:** P2 (Low)

---

## Summary Table

| Gap # | Description | Severity | Priority | Est. Lines |
|-------|-------------|----------|----------|------------|
| 1 | Literal scalars (|) with mixed indentation | HIGH | P0 | ~150 |
| 2 | Double-quoted strings with mixed indentation | MEDIUM | P1 | ~50 |
| 3 | Single-quoted strings with mixed indentation | MEDIUM | P1 | ~50 |
| 4 | Flow style mappings with mixed indentation | MEDIUM | P1 | ~50 |
| 5 | Flow style sequences with mixed indentation | MEDIUM | P1 | ~50 |
| 6 | Multi-line collections with mixed indentation | LOW | P2 | ~40 |
| 7 | Document markers with mixed indentation | LOW | P2 | ~30 |
| 8 | Comments with mixed indentation | LOW | P2 | ~30 |
| 9 | Anchors and aliases with mixed indentation | LOW | P2 | ~30 |
| 10 | Merge keys with mixed indentation | LOW | P2 | ~20 |

**Total Estimated Lines:** ~500 lines

---

## Recommendations

1. **Immediate Action:** Implement Gap #1 (literal scalars) as it's the most critical missing test that mirrors existing folded scalar tests.

2. **Short-term:** Implement Gaps #2-5 (quoted strings and flow style) as these are common YAML patterns.

3. **Long-term:** Implement Gaps #6-10 (edge cases) to achieve comprehensive coverage.

4. **Code Reuse:** Consider creating a helper macro or function to reduce duplication in mixed indentation test patterns.

---

## Verification

Once tests are implemented, verify:
```bash
# Run the specific test section
cargo test test_mixed_indentation_scenarios_with

# Run all Section 12B tests
cargo test -- section-12b

# Check for any failing tests
cargo test 2>&1 | grep -A5 "mixed_indentation"
```
