# Indent-Related Scope Test Results

**Date:** 2026-07-13  
**Task:** Run indent-related scope tests  
**Bead:** bf-5y0n9a

## Test Results Summary

| Test File | Total Tests | Passed | Failed | Status |
|-----------|-------------|--------|--------|--------|
| `indent_change_detection_test.rs` | 23 | 23 | 0 | ✅ PASS |
| `indent_without_key_test.rs` | 15 | 15 | 0 | ✅ PASS |
| `false_positive_indent_key_test.rs` | 13 | 9 | 4 | ❌ FAIL |

**Overall:** 51/58 tests passed (88%)

## Detailed Results

### ✅ indent_change_detection_test.rs (23/23 passed)

All tests in this suite passed, confirming:
- Indent change detection works correctly across various scenarios
- Blank lines and comments are handled properly
- Deep nesting and complex structures are tracked correctly
- Kubernetes-style YAML is parsed correctly
- Indent transitions don't interfere with duplicate key detection

### ✅ indent_without_key_test.rs (15/15 passed)

All tests in this suite passed, confirming:
- Blank lines don't create duplicate key errors
- Scope tracking correctly distinguishes key-bearing from indent-only lines
- Comments with different indents are handled properly
- No false scope entry on blank line increases
- Scope exits work correctly with blank lines

### ❌ false_positive_indent_key_test.rs (9/13 passed - 4 failures)

This suite tests false positive prevention - lines that should NOT be treated as keys.

#### Failing Tests:

1. **test_special_chars_only_not_a_key** (line 48-56)
   - Tests: `"  :::"` and `"  @#:"` should NOT extract key context
   - Expected: `None`
   - Actual: Extracts `Some(KeyContext)` with key `"@#"` and `"::"`
   - Panics at: line 55
   - Issue: `extract_key_context` doesn't filter special character-only keys

2. **test_block_scalar_indicator_not_a_key** (line 76-85)
   - Tests: `"  |:"` should NOT extract key context (block scalar indicator)
   - Expected: `None`
   - Actual: Extracts `Some(KeyContext)` with key `"|"`
   - Panics at: line 83
   - Issue: Doesn't recognize `|` as a block scalar indicator

3. **test_sequence_dash_only_not_a_key** (line 87-95)
   - Tests: `"  -:"` should NOT extract valid key context
   - Expected: `None` or empty key
   - Actual: Extracts `Some(KeyContext)` with non-empty key
   - Panics at: line 93
   - Issue: Dash stripping logic not working correctly for dash+colon

4. **test_no_false_positive_from_complex_indent** (line 97-119)
   - Tests: YAML with `"    :::"` line should parse without duplicate key error
   - Expected: Parse success
   - Actual: Parse failure (likely due to `" :::"` being treated as a key)
   - Panics at: line 113
   - Issue: Integration-level false positive

#### Passing Tests (9/13):
- ✅ `test_colon_only_not_a_key` - Empty key after colon handled correctly
- ✅ `test_single_char_colon_not_valid_key` - Single char with colon allowed
- ✅ `test_whitespace_around_colon_not_a_key` - Empty key with whitespace handled
- ✅ `test_colon_in_value_context_not_a_key` - Multi-colon values handled
- ✅ `test_empty_after_colon_is_parent_key` - Parent key mapping works
- ✅ `test_comment_like_pattern_not_a_key` - Hash-prefixed keys handled
- ✅ `test_flow_collection_markers_not_in_key` - Flow collection markers rejected
- ✅ `test_multiple_colons_in_key_position` - URL-like patterns handled
- ✅ `test_empty_key_part_not_a_key` - Empty key part handled

## Analysis

The `extract_key_context` function in `src/parsers/yaml/scope.rs` (lines 1692-1739) needs enhancements to handle these edge cases:

### Current Filtering:
- Empty keys: ✅ Filtered (line 1702-1704)
- Flow collection markers (`{`, `}`, `[`, `]`): ✅ Filtered (line 1707-1709)
- Sequence dash prefix: ⚠️ Partially handled (lines 1712-1726)

### Missing Filtering:
1. **Block scalar indicators** (`|`, `>`): Not recognized as special YAML tokens
2. **Special character-only keys** (`:::`, `@#`, etc.): Not filtered
3. **Dash+colon edge cases** (`-:`): Dash stripping may not handle all cases

## Recommendations

To fix these failures, enhance `extract_key_context` with additional validation:

1. Add block scalar indicator check before returning context
2. Add special character-only key validation
3. Improve dash+colon edge case handling
4. Consider YAML key validation rules (alphanumeric, underscore, hyphen allowed)

## Files Generated

- Test output saved to: `notes/bf-5y0n9a-test-output.txt`

## Related Issues

This is related to bead `bf-692thf` which addresses false positive duplicate key errors from indent-only changes.
