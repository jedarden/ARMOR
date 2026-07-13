# type_like_string_false_positive Test Execution Results

## Task
Run `cargo test type_like_string_false_positive` and capture all test output including stderr, execution time, and exit code.

## Execution Details

**Command**: `cargo test --test type_like_string_false_positive_test`

**Test Statistics**:
- Total tests: 262
- Passed: 258
- Failed: 4
- Ignored: 0
- Exit code: 101 (test failure)
- Execution time: 0.01s

## Failed Tests

### 1. test_detect_mapping_key_sequence_items_rejected
**File**: `tests/type_like_string_false_positive_test.rs:2110`
**Error**: Sequence item should be rejected by detect_mapping_key: `- !ns:tag`
The test expects that sequence items (lines starting with `-`) containing tag-like patterns should be rejected by the `detect_mapping_key` function, but the function is incorrectly accepting them.

### 2. test_folded_style_scalars_with_exclamation
**File**: `tests/type_like_string_false_positive_test.rs:4149`
**Error**: Folded scalar continuation should be Unknown or Tag: `'  This is important! Read carefully.'` (got MappingKey)
In folded scalars (YAML's `>` style), continuation lines with exclamation marks should be classified as `Unknown` or `Tag`, but they're being misclassified as `MappingKey`.

### 3. test_literal_style_scalars_with_exclamation
**File**: `tests/type_like_string_false_positive_test.rs:4216`
**Error**: Literal scalar patterns with ! should be valid: `'  !start and end!'`
In literal scalars (YAML's `|` style), lines with exclamation marks should be accepted as valid content, but they're being rejected.

### 4. test_multiline_comment_and_config_mixed_with_exclamation
**File**: `tests/type_like_string_false_positive_test.rs:7255`
**Error**: assertion `left == right` failed: Mixed multiline line 4 should be Unknown: `'  This is a multiline'` (left: MappingKey, right: Unknown)
In a mixed multiline context with comments and config, a line with continuation content containing exclamation marks should be classified as `Unknown`, but it's being misclassified as `MappingKey`.

## Common Pattern
All four failures appear to be related to the same underlying issue: the `detect_mapping_key` function is incorrectly classifying lines with exclamation marks as `MappingKey` when they should be classified as `Unknown`, `Tag`, or should be rejected entirely (in the case of sequence items).

## Compiler Warnings
The test run also produced 14 compiler warnings about unused variables and dead code in:
- `src/parsers/yaml/parser.rs`
- `src/parsers/yaml/syntax_validator.rs`
- `src/parsers/yaml/syntax_detector.rs`
- `src/parsers/traits.rs`
- `src/parsers/yaml/line_parser.rs`

## Full Output
The complete test output has been saved to `notes/bf-ha5ik-test-output.txt`.

## Test Location
The test file is located at: `tests/type_like_string_false_positive_test.rs`

This test suite validates false positive detection for type-like strings in YAML documents, particularly focusing on edge cases involving exclamation marks (which denote YAML tags) and ensuring they are not misclassified as mapping keys.
