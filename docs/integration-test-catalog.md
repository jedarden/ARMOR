# ARMOR Integration Test Catalog

Complete catalog of all 51 test files in `/tests` directory, categorized by feature area and test type.

## Summary

- **Total Test Files**: 51
- **Total Lines of Test Code**: ~52,000+
- **Largest Test**: `type_like_string_false_positive_test.rs` (14,580 lines)

---

## Scope Tracking & Scope Stack Tests

### Unit Tests (isolated component testing)

| Test File | Lines | Focus |
|-----------|-------|-------|
| `scope_stack_unit_test.rs` | 987 | Direct testing of ScopeStack methods (new, add_key, contains_key, depth, etc.) |
| `push_scope_unit_test.rs` | 309 | Testing push_scope functionality in isolation |
| `scope_stack_structure_test.rs` | 208 | Testing internal structure of ScopeStack |
| `scope_stack_test.rs` | 257 | Basic scope stack operations |

### Integration Tests (end-to-end workflows)

| Test File | Lines | Focus |
|-----------|-------|-------|
| `comprehensive_scope_tracking_test.rs` | 859 | Full scope tracking across complex scenarios |
| `scope_tracking_comprehensive_test.rs` | 958 | Comprehensive scope tracking with nesting |
| `scope_stack_verification_test.rs` | 895 | Verification that scope stack maintains correct state |
| `exit_to_scope_edge_cases_test.rs` | 604 | Edge cases when exiting scopes |
| `state_preservation_scope_exit_test.rs` | 710 | State preservation when exiting scopes |
| `target_scope_lookup_test.rs` | 393 | Looking up the correct target scope for operations |
| `sequence_scope_verification_test.rs` | 723 | Scope behavior in sequence contexts |

---

## Type Conversion Tests

### Integration Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `invalid_type_conversion_test.rs` | 2,983 | Invalid conversions (strings to non-strings, structs to scalars, etc.) |
| `int32_to_uint32_boundary_test.rs` | 857 | Boundary testing for int32 → uint32 conversions |
| `int32_to_uint32_error_detection_test.rs` | 513 | Error detection in int32 → uint32 conversions |
| `negative_int32_to_uint32_error_verification.rs` | 471 | Verification of negative int32 → uint32 errors |
| `negative_conversion_error_message_test.rs` | 248 | Error messages for negative number conversions |
| `type_like_string_false_positive_test.rs` | 14,580 | Preventing false positives when types look like strings |

---

## Error Handling & Parse Error Tests

### Unit Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `parse_error_unit_test.rs` | 607 | Unit tests for ParseError type and methods |

### Integration Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `parse_error_integration_test.rs` | 616 | End-to-end error creation and formatting workflows |
| `parse_error_full_lifecycle_integration_test.rs` | 743 | Full lifecycle of parse errors from creation to display |
| `parse_error_propagation_test.rs` | 237 | How errors propagate through function calls |
| `parse_error_display_test.rs` | 339 | Error message formatting and display |
| `error_code_validation_test.rs` | 307 | Validation of error codes |
| `error_messages_test.rs` | 52 | Basic error message tests |
| `malformed_error_message_test.rs` | 785 | Handling malformed error messages |
| `error_message_format_examples_test.rs` | 503 | Examples of error message formats |
| `validation_error_format_test.rs` | 264 | Format of validation errors |

---

## YAML Comment Filtering Tests

### Integration Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `yaml_comment_filtering_edge_cases_test.rs` | 672 | Edge cases in YAML comment detection |
| `yaml_comment_edge_case_test.rs` | 861 | Difficult edge cases in comment handling |
| `yaml_comment_false_positive_test.rs` | 820 | Preventing false positive comment detection |
| `yaml_comment_position_test.rs` | 388 | Comment position awareness |
| `yaml_block_scalar_indentation_comment_test.rs` | 881 | Comments in block scalar contexts |
| `yaml_folded_multiline_comment_test.rs` | 910 | Comments in folded multiline scalars |
| `yaml_folded_scalar_continuation_validation_test.rs` | 769 | Folded scalar continuation with comments |
| `yaml_literal_multiline_comment_test.rs` | 533 | Comments in literal multiline scalars |
| `yaml_multiline_quoted_scalar_comment_test.rs` | 536 | Comments in quoted multiline scalars |
| `yaml_plain_multiline_scalar_comment_test.rs` | 599 | Comments in plain multiline scalars |
| `yaml_indentation_and_mixed_scenarios_test.rs` | 1,539 | Mixed indentation and comment scenarios |
| `yaml_indent_without_keys_test.rs` | 354 | Indentation changes without key definitions |
| `inline_comment_detection_test.rs` | 757 | Inline comment detection and stripping |
| `comment_filtering_basic_test.rs` | 358 | Basic comment filtering functionality |

---

## Indentation Detection Tests

### Integration Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `indent_change_detection_test.rs` | 566 | Detecting and handling indent changes |
| `indent_without_key_test.rs` | 356 | Indentation changes that don't introduce keys |
| `false_positive_indent_key_test.rs` | 166 | Preventing false positives in indent key detection |
| `missing_colon_comprehensive_test.rs` | 227 | Handling missing colons with indent changes |

---

## Line Classification Tests

### Integration Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `line_classification_test.rs` | 301 | Classification of YAML line types (keys, values, comments, etc.) |

---

## Data Validation & Schema Tests

### Integration Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `schema_validation_test.rs` | 655 | Schema validation workflows |
| `result_dataclass_test.rs` | 87 | Result type dataclass behavior |
| `status_enum_smoke_test.rs` | 50 | Status enum basic smoke test |
| `acceptance_criteria_verification_test.rs` | 197 | Verification that acceptance criteria are met |

---

## Duplicate Detection Tests

### Integration Tests

| Test File | Lines | Focus |
|-----------|-------|-------|
| `nested_duplicate_detection_test.rs` | 878 | Duplicate key detection in nested structures |

---

## Test Type Summary

### Unit Tests (6 files)
Tests that verify individual components in isolation:
- `scope_stack_unit_test.rs`
- `push_scope_unit_test.rs`
- `parse_error_unit_test.rs`
- `scope_stack_structure_test.rs`
- `scope_stack_test.rs`
- `result_dataclass_test.rs` (borderline unit/integration)

### Integration Tests (45 files)
Tests that verify end-to-end workflows and component interaction:
- All other test files
- These test the complete parsing pipeline from input to output

---

## Test Size Distribution

### Very Large (1000+ lines)
- `type_like_string_false_positive_test.rs` - 14,580 lines
- `invalid_type_conversion_test.rs` - 2,983 lines
- `yaml_indentation_and_mixed_scenarios_test.rs` - 1,539 lines

### Large (500-999 lines)
- `scope_stack_unit_test.rs` - 987 lines
- `scope_tracking_comprehensive_test.rs` - 958 lines
- `yaml_folded_multiline_comment_test.rs` - 910 lines
- `scope_stack_verification_test.rs` - 895 lines
- `yaml_block_scalar_indentation_comment_test.rs` - 881 lines
- `nested_duplicate_detection_test.rs` - 878 lines
- `yaml_comment_edge_case_test.rs` - 861 lines
- `comprehensive_scope_tracking_test.rs` - 859 lines
- `int32_to_uint32_boundary_test.rs` - 857 lines
- `yaml_comment_false_positive_test.rs` - 820 lines
- `malformed_error_message_test.rs` - 785 lines
- `yaml_folded_scalar_continuation_validation_test.rs` - 769 lines
- `inline_comment_detection_test.rs` - 757 lines
- `parse_error_full_lifecycle_integration_test.rs` - 743 lines
- `sequence_scope_verification_test.rs` - 723 lines
- `state_preservation_scope_exit_test.rs` - 710 lines
- `yaml_comment_filtering_edge_cases_test.rs` - 672 lines
- `schema_validation_test.rs` - 655 lines
- `parse_error_integration_test.rs` - 616 lines
- `parse_error_unit_test.rs` - 607 lines
- `exit_to_scope_edge_cases_test.rs` - 604 lines
- `yaml_plain_multiline_scalar_comment_test.rs` - 599 lines
- `indent_change_detection_test.rs` - 566 lines
- `yaml_multiline_quoted_scalar_comment_test.rs` - 536 lines
- `yaml_literal_multiline_comment_test.rs` - 533 lines
- `int32_to_uint32_error_detection_test.rs` - 513 lines
- `error_message_format_examples_test.rs` - 503 lines

### Medium (200-499 lines)
- `negative_int32_to_uint32_error_verification.rs` - 471 lines
- `target_scope_lookup_test.rs` - 393 lines
- `yaml_comment_position_test.rs` - 388 lines
- `comment_filtering_basic_test.rs` - 358 lines
- `indent_without_key_test.rs` - 356 lines
- `yaml_indent_without_keys_test.rs` - 354 lines
- `parse_error_display_test.rs` - 339 lines
- `push_scope_unit_test.rs` - 309 lines
- `error_code_validation_test.rs` - 307 lines
- `line_classification_test.rs` - 301 lines
- `validation_error_format_test.rs` - 264 lines
- `scope_stack_test.rs` - 257 lines
- `negative_conversion_error_message_test.rs` - 248 lines
- `parse_error_propagation_test.rs` - 237 lines
- `missing_colon_comprehensive_test.rs` - 227 lines
- `scope_stack_structure_test.rs` - 208 lines

### Small (1-199 lines)
- `acceptance_criteria_verification_test.rs` - 197 lines
- `false_positive_indent_key_test.rs` - 166 lines
- `result_dataclass_test.rs` - 87 lines
- `error_messages_test.rs` - 52 lines
- `status_enum_smoke_test.rs` - 50 lines

---

## Feature Area Breakdown by Line Count

| Feature Area | Files | Total Lines |
|--------------|-------|-------------|
| YAML Comment Filtering | 14 | ~8,900 |
| Scope Tracking | 11 | ~6,700 |
| Type Conversion | 5 | ~19,400 |
| Error Handling | 9 | ~4,300 |
| Indentation Detection | 4 | ~1,300 |
| Line Classification | 1 | ~300 |
| Schema Validation | 3 | ~900 |
| Duplicate Detection | 1 | ~878 |

---

## Bead Tracking

This catalog was created for bead: **bf-4538tw** - "Catalog all ARMOR integration tests"

Generated: 2026-07-13
