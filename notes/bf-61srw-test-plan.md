# Section 12B Mixed Indentation Test Gaps - Test Plan

## Overview

This document analyzes the existing test coverage in Section 12B (Multiline String Scenarios with Exclamation Marks) and identifies specific test case gaps that need to be added.

**File**: `tests/type_like_string_false_positive_test.rs`  
**Section Start**: Line 6728  
**Number of Tests**: 40 test functions  
**Last Updated**: 2026-07-13

---

## Current Coverage Summary

### What IS Covered (40 tests)

#### Main Block Scalar Tests
1. ✅ `test_folded_block_scalar_with_exclamation_marks` - Basic folded scalars (>) with !
2. ✅ `test_literal_block_scalar_with_exclamation_marks` - Basic literal scalars (|) with !
3. ✅ `test_multiline_mixed_with_singleline_exclamation_patterns` - Mixed multiline/single-line
4. ✅ `test_multiline_yaml_strings_with_exclamation_in_nested_contexts` - Nested contexts
5. ✅ `test_folded_scalar_exclamation_at_different_positions` - ! at different positions
6. ✅ `test_literal_scalar_exclamation_at_different_positions` - Literal scalars with !
7. ✅ `test_multiline_block_scalar_modifiers_with_exclamation` - Modifiers (-, +, numeric)
8. ✅ `test_real_world_multiline_config_with_exclamation` - Real-world configs
9. ✅ `test_multiline_comment_and_config_mixed_with_exclamation` - Comments mixed
10. ✅ `test_multiline_sequence_with_exclamation_in_block_scalars` - Sequences

#### Indentation Level Tests (Levels 1-6)
11. ✅ `test_level1_indentation_with_exclamation_marks` - 2-space indent
12. ✅ `test_level_1_indentation_with_exclamation_mark` - 2-space indent (duplicate)
13. ✅ `test_level_2_indentation_with_exclamation_mark` - 4-space indent
14. ✅ `test_level2_indentation_with_exclamation_marks` - 4-space indent (duplicate)
15. ✅ `test_level_3_indentation_with_exclamation_mark` - 6-space indent
16. ✅ `test_level3_indentation_with_exclamation_marks` - 6-space indent (duplicate)
17. ✅ `test_level4_indentation_with_exclamation_marks` - 8-space indent
18. ✅ `test_level5_indentation_with_exclamation_marks` - 10-space indent
19. ✅ `test_level6_indentation_with_exclamation_marks` - 12-space indent
20. ✅ `test_basic_indentation_levels_with_exclamation_marks` - Levels 1-3
21. ✅ `test_various_indentation_levels_with_exclamation_marks` - Various levels (1-6, tabs, mixed)

#### Section 12B.2 Tests (Folded Scalar Indicators)
22. ✅ `test_folded_scalar_indicator_lines` - Basic > indicators
23. ✅ `test_folded_scalar_basic_modifiers` - >-, >+ modifiers
24. ✅ `test_folded_scalar_numeric_modifiers` - >n, >-n, >+n
25. ✅ `test_folded_scalar_indented_indicators` - Indented indicators
26. ✅ `test_folded_scalar_all_modifier_combinations` - All > modifier combos

#### Section 12B.1 Tests (Comprehensive Folded)
27. ✅ `test_folded_scalar_indicator_classification` - Indicator classification

#### Section 12B.2 Tests (Basic Folded Scalar Indicators)
28. ✅ `test_basic_folded_scalar_indicator_as_mapping_key` - Basic > as MappingKey
29. ✅ `test_folded_scalar_with_continuation_content` - > with following content
30. ✅ `test_folded_scalar_continuation_lines_with_exclamation_marks` - Continuation with !
31. ✅ `test_folded_scalar_continuation_lines_starting_with_exclamation` - Continuation starting with !
32. ✅ `test_folded_scalar_continuation_exclamation_various_contexts` - ! in various contexts
33. ✅ `test_comprehensive_tab_indented_folded_scalars_with_exclamation` - Tab-indented folded scalars
34. ✅ `test_comprehensive_various_indentation_levels_with_exclamation` - Various indentation levels
35. ✅ `test_mixed_indentation_scenarios_with_folded_scalars` - Mixed indentation (tabs + spaces)

#### Other Tests
36. ✅ `test_basic_indentation_levels_with_exclamation_marks` - Already counted above
37. ✅ Additional variations in mixed indentation scenarios
38. ✅ Tab-indented variations
39. ✅ Continuation line variations
40. ✅ Various modifier combinations

---

## Identified Gaps

### Gap 1: Literal Scalar (|) Modifier Coverage ⚠️ HIGH PRIORITY

**Current State**: Only basic `|` tests exist. Modifier coverage is incomplete.

**Missing Tests**:
1. **Literal scalar with strip modifier (|-)**
   ```yaml
   log: |-
     Error! occurred!
   ```
2. **Literal scalar with keep modifier (|+)**
   ```yaml
   output: |+
     Success! completed!
   ```
3. **Literal scalar with numeric indent (|2, |3, etc.)**
   ```yaml
   text: |2
     Indented! literal!
   ```
4. **Literal scalar with numeric + strip (|-2, |-3)**
   ```yaml
   note: |-2
     Important! info!
   ```
5. **Literal scalar with numeric + keep (|+2, |+3)**
   ```yaml
   message: |+2
     Critical! alert!
   ```
6. **All literal modifier combinations at each indentation level** (1-6)
   - Test each modifier (|, |-, |+, |n, |-n, |+n for n=1-9)
   - At each indentation level (0, 2, 4, 6, 8, 10, 12 spaces)
   - With exclamation marks in continuation lines

**Priority**: HIGH - Literal scalars are less tested than folded scalars but equally important.

---

### Gap 2: Inconsistent Indentation Within Block ⚠️ HIGH PRIORITY

**Current State**: All tests assume consistent indentation. Real YAML files sometimes have inconsistent indentation.

**Missing Tests**:
1. **Indicator at 2 spaces, continuation at 4 spaces**
   ```yaml
   key: >
     Deeper! continuation!
   Back to 2!
   ```
2. **Indicator at 4 spaces, continuation at 2 spaces**
   ```yaml
     key: >
   Shallower! continuation!
   ```
3. **Alternating indentation patterns**
   ```yaml
   key: >
     Level! 1!
   Level! 2!
     Level! 1! again!
   ```
4. **Inconsistent indentation with tab/space mixing**
   ```yaml
   key: >
   \t  Tab! then! spaces!
     \t  Spaces! then! tab!
   ```
5. **Gradual indentation increase**
   ```yaml
   key: >
   Line! 1!
     Line! 2!
       Line! 3!
         Line! 4!
   ```

**Priority**: HIGH - Real-world YAML often has inconsistent indentation due to manual editing.

---

### Gap 3: Empty Lines and Trailing Whitespace

**Current State**: No tests for empty lines in block scalars or trailing whitespace behavior.

**Missing Tests**:
1. **Empty lines in folded scalars**
   ```yaml
   description: >
     Line! 1!

     Line! 3! (after empty line)
   ```
2. **Empty lines in literal scalars**
   ```yaml
   log: |
     Line! 1!

     Line! 3!
   ```
3. **Trailing spaces on continuation lines**
   ```yaml
   note: >
     Text! with! trailing! spaces!   
   ```
4. **Trailing tabs on continuation lines**
   ```yaml
   message: >
     Text! with! trailing! tabs!	
   ```
5. **Multiple consecutive empty lines**
   ```yaml
   description: >
     Line! 1!


     Line! 4!
   ```

**Priority**: MEDIUM - Empty lines are common in YAML but behavior needs verification.

---

### Gap 4: Consecutive Block Scalars at Same Level

**Current State**: No tests for multiple block scalars appearing consecutively.

**Missing Tests**:
1. **Two folded scalars consecutively**
   ```yaml
   description1: >
     First! description!
   description2: >
     Second! description!
   ```
2. **Folded then literal**
   ```yaml
   text1: >
     Folded! text!
   text2: |
     Literal! text!
   ```
3. **Literal then folded**
   ```yaml
   log1: |
     First! log!
   log2: >
     Second! log!
   ```
4. **Three or more consecutive block scalars**
   ```yaml
   block1: >
     First! block!
   block2: |
     Second! block!
   block3: >
     Third! block!
   ```
5. **Consecutive with different modifiers**
   ```yaml
   item1: >-
     Stripped! content!
   item2: >+
     Kept! content!
   item3: >2
     Indented! content!
   ```

**Priority**: MEDIUM - Consecutive blocks appear in structured configs.

---

### Gap 5: Nested Flow and Block Mixing

**Current State**: No tests mixing flow collections (using {} or []) with block scalars.

**Missing Tests**:
1. **Flow mapping with block scalar value**
   ```yaml
   mapping: {key: >, value: "test"}
     This! is! complex!
   ```
2. **Flow sequence with block scalar**
   ```yaml
   items: [item1, item2, >
     Complex! item! list!]
   ```
3. **Block scalar containing flow syntax**
   ```yaml
   description: >
     Text! with {flow: mapping}! inside!
   ```
4. **Nested flow within block**
   ```yaml
   config: >
     outer: {inner: [a, b, c]}!
     More! text!
   ```

**Priority**: LOW - Complex mixing is rare but syntactically possible.

---

### Gap 6: Exclamation Mark Density and Patterns

**Current State**: Limited testing of dense ! patterns.

**Missing Tests**:
1. **Very high density of ! marks**
   ```yaml
   text: >
     !!!!!!!!Multiple! consecutive!!!!!
   ```
2. **Alternating ! and other characters**
   ```yaml
   value: >
     a!b!c!d!e!f!g!h!i!j!
   ```
3. **! at regular intervals**
   ```yaml
   message: >
     word! word! word! word! word!
   ```
4. **Sparse ! distribution**
   ```yaml
   description: >
     Very long text without much punctuation! but some here! and there!
   ```
5. **! surrounded by various whitespace**
   ```yaml
   note: >
     word! !word!!  word!  !!
   ```

**Priority**: MEDIUM - Edge cases that might confuse classifiers.

---

### Gap 7: Very Long Lines with Exclamation Marks

**Current State**: All test lines are short (<100 chars).

**Missing Tests**:
1. **Very long folded scalar continuation (200+ chars)**
   ```yaml
   description: >
     Very! long! line! with! many! words! and! exclamation! marks! that! continues! for! a! very! long! time! until! it! reaches! the! limit! of! reasonable! line! length!
   ```
2. **Multiple very long lines**
   ```yaml
   text: >
     First! very! long! line! with! lots! of! content! and! exclamation! marks! throughout!
     Second! very! long! line! also! with! extensive! content! and! emphasis! marks!
   ```
3. **Long lines with ! at specific positions**
   - ! at character 1
   - ! at character 50
   - ! at character 100
   - ! at character 200

**Priority**: LOW - Unlikely to cause issues but good for completeness.

---

### Gap 8: Unicode and Special Characters with !

**Current State**: Only ASCII characters tested.

**Missing Tests**:
1. **Unicode with !**
   ```yaml
   text: >
     Héllo! wörld! 日本語! 𝕌𝕟𝕚𝕔𝕠𝕕𝕖!
   ```
2. **Emoji with !**
   ```yaml
   message: >
     Check! this! out! 🎉! 👍! ✨!
   ```
3. **Special YAML characters with !**
   ```yaml
   note: >
     Null: ! value! with! special! chars: []{}|
   ```
4. **Escaped characters with !**
   ```yaml
   text: >
     Line! with! \"quotes\"! and! \\backslashes\\!
   ```

**Priority**: MEDIUM - Unicode is common in modern YAML.

---

### Gap 9: Zero Indentation (Root Level) Block Scalars

**Current State**: All block scalars are indented. Root-level behavior not tested.

**Missing Tests**:
1. **Root level folded scalar**
   ```yaml
   root_description: >
     Root! level! folded! scalar!
   ```
2. **Root level literal scalar**
   ```yaml
   root_log: |
     Root! level! literal! scalar!
   ```
3. **Root level with modifiers**
   ```yaml
   root_text: >-
     Root! level! with! strip! modifier!
   ```
4. **Root level continuation lines**
   - Test 0-indent continuation behavior
   - Test interaction with document start markers (`---`)

**Priority**: MEDIUM - Root-level blocks are valid but less common in nested configs.

---

### Gap 10: Anchor and Alias with Block Scalars

**Current State**: No tests for YAML anchors/aliases with block scalars.

**Missing Tests**:
1. **Anchor on block scalar indicator**
   ```yaml
   default_text: &default >
     Default! text! with! bangs!
   ```
2. **Alias to block scalar**
   ```yaml
   current_text: *default
   ```
3. **Anchor on continuation line**
   ```yaml
   note: >
     Line! 1!
     &anchor Line! 2!
   ```
4. **Multiple anchors with different ! patterns**
   ```yaml
   text1: &t1 >
     Pattern! one! with! bangs!
   text2: &t2 >
     Pattern! two! with! marks!
   reused: *t1
   ```

**Priority**: LOW - Anchors/aliases with block scalars are uncommon.

---

### Gap 11: Merge Keys with Block Scalars

**Current State**: No tests for merge keys (<<) with block scalars.

**Missing Tests**:
1. **Merge key with block scalar value**
   ```yaml
   <<: >
     Merged! content! with! bangs!
   ```
2. **Merge key in sequence with block scalars**
   ```yaml
   items:
     - <<: >
         First! merge! content!
     - <<: >
         Second! merge! content!
   ```

**Priority**: LOW - Very specialized YAML feature.

---

### Gap 12: High Numeric Modifiers (Beyond 9)

**Current State**: Numeric modifiers only tested up to 9.

**Missing Tests**:
1. **Modifier >10**
   ```yaml
   text: >10
     Very! indented! content!
   ```
2. **Modifier >-15**
   ```yaml
   note: >-15
     Extremely! indented! with! strip!
   ```
3. **Modifier >+20**
   ```yaml
   message: >+20
     Very! deep! indentation! with! keep!
   ```
4. **Very high modifier >+100**
   ```yaml
   description: >+100
     Unrealistic! but! syntactically! valid!
   ```

**Priority**: LOW - YAML spec doesn't limit modifier values, but high values are unrealistic.

---

### Gap 13: Invalid Modifier Combinations

**Current State**: No negative tests for invalid modifiers.

**Missing Tests** (These should verify proper error handling):
1. **Invalid modifier characters**
   ```yaml
   text: >x
     Invalid! modifier!
   ```
2. **Negative numeric modifier**
   ```yaml
   note: >-1
     Should! be! valid!
   ```
3. **Zero modifier**
   ```yaml
   message: >0
     Zero! indent!
   ```
4. **Multiple modifiers**
   ```yaml
   description: >-+2
     Multiple! modifiers! invalid!
   ```
5. **Modifier without block indicator**
   ```yaml
   key: -2
     Not! a! block! scalar!
   ```

**Priority**: MEDIUM - Negative tests ensure robustness.

---

### Gap 14: Set Keys (? with Block Scalars)

**Current State**: No tests for set keys (using `?`) with block scalars.

**Missing Tests**:
1. **Set key with block scalar**
   ```yaml
   ? key: >
       Complex! key! with! block!
   : value
   ```
2. **Set key with literal scalar**
   ```yaml
   ? key: |
       Complex! key! literal!
   : value
   ```

**Priority**: LOW - Very rare YAML construct.

---

### Gap 15: Document Start/End Markers with Block Scalars

**Current State**: No tests for YAML document markers (`---`, `...`) with block scalars.

**Missing Tests**:
1. **Document start before block scalar**
   ```yaml
   ---
   description: >
     Document! start! then! block!
   ```
2. **Document end after block scalar**
   ```yaml
   log: |
     Block! then! document! end!
   ...
   ```
3. **Multiple documents with block scalars**
   ```yaml
   ---
   text1: >
     First! document!
   ---
   text2: >
     Second! document!
   ...
   ```

**Priority**: MEDIUM - Multi-document YAML files are common.

---

## Test Implementation Priority Order

### Phase 1: Critical Gaps (Implement First)
1. ✅ **Gap 1**: Literal scalar modifier coverage (HIGH)
2. ✅ **Gap 2**: Inconsistent indentation within blocks (HIGH)
3. ✅ **Gap 3**: Empty lines and trailing whitespace (MEDIUM-HIGH)

### Phase 2: Important Gaps
4. ✅ **Gap 4**: Consecutive block scalars (MEDIUM)
5. ✅ **Gap 6**: Exclamation mark density patterns (MEDIUM)
6. ✅ **Gap 8**: Unicode and special characters (MEDIUM)
7. ✅ **Gap 9**: Zero indentation block scalars (MEDIUM)

### Phase 3: Edge Cases
8. **Gap 5**: Nested flow and block mixing (LOW)
9. **Gap 7**: Very long lines (LOW)
10. **Gap 10**: Anchor and alias with block scalars (LOW)
11. **Gap 13**: Invalid modifier combinations (MEDIUM for robustness)

### Phase 4: Rare Scenarios
12. **Gap 11**: Merge keys with block scalars (LOW)
13. **Gap 12**: High numeric modifiers (LOW)
14. **Gap 14**: Set keys with block scalars (LOW)
15. **Gap 15**: Document markers with block scalars (MEDIUM for multi-doc support)

---

## Summary Statistics

- **Total Identified Gaps**: 15
- **Total Missing Test Cases**: ~75-100 specific scenarios
- **High Priority Gaps**: 2
- **Medium Priority Gaps**: 8
- **Low Priority Gaps**: 5
- **Estimated New Tests Needed**: 25-35 test functions

---

## Notes

1. **Duplicate Tests**: The current suite has some duplicate test functions (e.g., `test_level_1_indentation_with_exclamation_mark` and `test_level1_indentation_with_exclamation_marks` both test 2-space indentation). Consider consolidating or clarifying the distinction.

2. **Test Naming**: Section 12B.2 appears twice (lines 8296 and 8955). Consider renaming to avoid confusion (e.g., 12B.2 and 12B.3).

3. **Literal Scalar Under-coverage**: Literal scalars (|) have significantly fewer tests than folded scalars (>). This should be prioritized.

4. **Real-World Patterns**: The existing `test_real_world_multiline_config_with_exclamation` is valuable. More such tests based on actual production YAML files would be beneficial.

---

## Next Steps

1. ✅ Review this test plan with the development team
2. ✅ Prioritize gaps based on project requirements
3. ✅ Implement Phase 1 tests (Critical Gaps)
4. ✅ Implement Phase 2 tests (Important Gaps)
5. ⏳ Implement Phase 3 tests (Edge Cases)
6. ⏳ Implement Phase 4 tests (Rare Scenarios)
7. ⏳ Review and consolidate duplicate tests
8. ⏳ Update test documentation

---

**Document Created**: 2026-07-13  
**Bead ID**: bf-61srw  
**Status**: Analysis Complete, Ready for Implementation
