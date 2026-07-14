# Parsed Test Results Analysis
**Generated:** 2026-07-13  
**Test Framework:** Rust Cargo Test  
**Test Suite:** parsers::yaml::syntax_detector_tests

## Executive Summary
- **Total Tests:** 54
- **Passed:** 51 (94.4%)
- **Failed:** 3 (5.6%)
- **Ignored:** 0
- **Measured:** 0
- **Filtered out:** 195

## Test Results by Category

### 1. Delimiter Tests (22 tests)
**Pass Rate:** 21/22 (95.5%)

#### Passing Tests (21):
- ✅ test_accept_valid_delimiters
- ✅ test_delimiter_error_classification_mismatched_quotes
- ✅ test_delimiter_error_classification_missing_colon
- ✅ test_delimiter_error_classification_unclosed_brace
- ✅ test_delimiter_error_classification_unclosed_bracket
- ✅ test_delimiter_error_classification_unclosed_double_quote
- ✅ test_delimiter_error_classification_unclosed_single_quote
- ✅ test_delimiter_error_classification_unmatched_closing_bracket
- ✅ test_delimiter_error_type_codes
- ✅ test_delimiter_error_type_display
- ✅ test_detect_mismatched_quotes
- ✅ test_detect_missing_colon_after_key
- ✅ test_detect_unclosed_double_quote
- ✅ test_detect_unmatched_closing_brace
- ✅ test_detect_unmatched_closing_bracket
- ✅ test_detect_unmatched_opening_brace
- ✅ test_detect_unmatched_opening_bracket
- ✅ test_multiple_delimiter_errors_same_line
- ✅ test_nested_brackets_and_braces
- ✅ test_quote_escaping_detection

#### Failing Tests (1):
- ❌ **test_complex_delimiter_balance** - False positive duplicate key detection

---

### 2. Indentation Tests (14 tests)
**Pass Rate:** 14/14 (100%)

#### All Tests Passing:
- ✅ test_accept_consistent_spaces
- ✅ test_accept_four_space_indentation
- ✅ test_detect_inconsistent_indentation
- ✅ test_detect_large_indentation_increase
- ✅ test_detect_mixed_tabs_and_spaces
- ✅ test_detect_tab_only_indentation
- ✅ test_indentation_error_classification_excessive_increase
- ✅ test_indentation_error_classification_invalid_increase
- ✅ test_indentation_error_classification_invalid_level
- ✅ test_indentation_error_classification_mixed
- ✅ test_indentation_error_classification_tab_character
- ✅ test_indentation_error_type_codes
- ✅ test_indentation_error_type_display
- ✅ test_multiple_indentation_errors

---

### 3. Integration Tests (3 tests)
**Pass Rate:** 1/3 (33.3%)

#### Passing Tests (1):
- ✅ test_empty_and_comment_only_content
- ✅ test_multiple_error_types

#### Failing Tests (2):
- ❌ **test_complex_nested_structure** - Assertion failure: expected empty errors
- ❌ **test_valid_complete_yaml** - False positive duplicate key detection

---

### 4. Performance Tests (2 tests)
**Pass Rate:** 2/2 (100%)

#### All Tests Passing:
- ✅ test_deep_nesting_performance
- ✅ test_large_file_performance

---

### 5. Regression Tests (6 tests)
**Pass Rate:** 6/6 (100%)

#### All Tests Passing:
- ✅ test_flow_style_with_braces
- ✅ test_flow_style_with_brackets
- ✅ test_no_false_positives_for_anchors_and_aliases
- ✅ test_no_false_positives_for_quoted_keys
- ✅ test_no_false_positives_for_time_values
- ✅ test_no_false_positives_for_urls

---

### 6. Structure Tests (7 tests)
**Pass Rate:** 7/7 (100%)

#### All Tests Passing:
- ✅ test_accept_valid_mappings
- ✅ test_accept_valid_sequences
- ✅ test_detect_duplicate_keys_same_level
- ✅ test_detect_global_duplicate_keys
- ✅ test_detect_invalid_colon_at_start
- ✅ test_detect_invalid_sequence_syntax
- ✅ test_detect_nested_duplicate_keys

---

## Detailed Failure Analysis

### Failure 1: test_complex_delimiter_balance
**Category:** Delimiter Tests  
**Error Type:** False positive duplicate key detection

**Debug Output:**
```
DEBUG - Found 2 errors in complex delimiter YAML:
  0 path=key_{name line=Some(3) msg=duplicate key '{name' at same indentation level
  1 path=key_{name line=Some(2) msg=duplicate key '{name' appears 2 times in document
```

**Issue:** The parser incorrectly interprets complex delimiter structures (likely containing `{` or `}` characters in keys) as duplicate keys, when they are actually valid flow-style YAML syntax.

---

### Failure 2: test_complex_nested_structure  
**Category:** Integration Tests  
**Error Type:** Assertion failure

**Debug Output:**
```
assertion failed: errors.is_empty()
```

**Issue:** The test expected no errors for a valid complex nested YAML structure, but the parser produced errors. The specific error details are not visible in the output, but this suggests the parser is incorrectly flagging valid nested structures as invalid.

---

### Failure 3: test_valid_complete_yaml
**Category:** Integration Tests  
**Error Type:** False positive duplicate key detection

**Debug Output:**
```
DEBUG - Found 2 errors in valid YAML:
  0 path=key_port line=Some(5) msg=duplicate key 'port' appears 2 times in document
  1 path=key_host line=Some(4) msg=duplicate key 'host' appears 2 times in document
```

**Issue:** The parser is incorrectly detecting 'port' and 'host' as duplicate keys in what appears to be valid YAML, likely in different scopes or contexts where duplicates are allowed (e.g., different mapping levels).

---

## Common Failure Patterns

### Pattern 1: False Positive Duplicate Key Detection
**Affected Tests:** 2/3 failures
- `test_complex_delimiter_balance` - detects `{name` as duplicate
- `test_valid_complete_yaml` - detects `port` and `host` as duplicates

**Root Cause:** The duplicate key detection logic doesn't properly account for:
1. Flow-style syntax with braces/brackets in keys
2. Different mapping scopes/levels where duplicate keys are valid

### Pattern 2: Complex Nested Structure Handling
**Affected Tests:** 1/3 failures
- `test_complex_nested_structure`

**Root Cause:** The parser incorrectly validates complex nested YAML structures, producing errors for valid YAML.

---

## Warnings Summary

### Compiler Warnings (15 total):
1. **Unused imports:** `ValidationError` (1)
2. **Unused variables:** `content`, `delimiter`, `context`, `line_num`, `char_pos`, `quote_char`, `parallelism` (8)
3. **Unused field:** `check_duplicate_keys` (1)
4. **Dead code:** `detect_mapping_key_simple` function (1)
5. **Other:** Unnecessary mut, useless comparison (2)

**Notable:** `check_duplicate_keys` field is never read despite being part of duplicate key detection - this may indicate incomplete implementation.

---

## Test Coverage Assessment

**Strong Coverage:**
- Indentation validation: 100% pass rate (14/14)
- Performance testing: 100% pass rate (2/2)
- Regression testing: 100% pass rate (6/6)
- Structure validation: 100% pass rate (7/7)

**Weak Coverage:**
- Integration testing: 33% pass rate (1/3) - **CRITICAL ISSUE**
- Delimiter testing: 95% pass rate (21/22)

---

## Recommendations

### High Priority:
1. **Fix duplicate key detection logic** - The parser is producing false positives for valid YAML
2. **Investigate complex nested structure handling** - 1/3 integration tests failing
3. **Complete `check_duplicate_keys` field implementation** - Currently unused

### Medium Priority:
1. Add unit tests for flow-style YAML with complex delimiters
2. Improve scope/context awareness in duplicate key detection
3. Clean up compiler warnings (15 warnings)

### Low Priority:
1. Remove dead code (`detect_mapping_key_simple`)
2. Add more integration test cases for edge cases

---

## Conclusion

The YAML parser has strong performance in indentation, regression, and structure validation (100% pass rates), but has **critical issues** with:
1. False positive duplicate key detection (2 failures)
2. Complex nested structure validation (1 failure)

These failures indicate the parser is overly aggressive in detecting errors, flagging valid YAML as invalid. This suggests the duplicate key detection and scope/context analysis logic needs refinement.
