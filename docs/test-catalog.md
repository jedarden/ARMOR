# ARMOR Integration Test Catalog

**Generated:** 2026-07-13  
**Total Test Files:** 51  
**Total Lines:** 27,847

## Test Classification

### Integration Tests (19 files, ~9,200 lines)
Full lifecycle tests exercising multiple components together, realistic YAML parsing workflows.

### Unit Tests (32 files, ~18,600 lines)  
Focused tests for individual methods, data structures, and specific functionality.

---

## Category 1: Scope Tracking (7 files, 5,727 lines)

Tests for ScopeStack operations, scope entry/exit, sequence scope handling, and scope management.

| Test File | Lines | Type | Description |
|-----------|-------|------|-------------|
| `scope_stack_unit_test.rs` | 987 | Unit | Focused ScopeStack method coverage (new, add_key, push_scope, exit_to_scope, contains_key) |
| `scope_tracking_comprehensive_test.rs` | 958 | Integration | Comprehensive scope tracking including ScopeStack, sequence scope, key context, integration with SyntaxDetector |
| `scope_stack_verification_test.rs` | 895 | Integration | ScopeStack behavior verification with realistic parsing scenarios |
| `sequence_scope_verification_test.rs` | 723 | Integration | Sequence scope entry/exit tracking, nested sequences, mixed mappings |
| `state_preservation_scope_exit_test.rs` | 710 | Integration | State preservation when exiting scopes, scope isolation |
| `exit_to_scope_edge_cases_test.rs` | 604 | Integration | Edge cases for scope exit behavior, boundary conditions |
| `scope_stack_structure_test.rs` | 208 | Unit | ScopeStack internal structure verification |
| `scope_stack_test.rs` | 257 | Unit | Basic ScopeStack operations and properties |
| `target_scope_lookup_test.rs` | 393 | Integration | Target scope lookup functionality |

---

## Category 2: Type Conversion & Validation (7 files, 22,503 lines)

Tests for type checking, conversion errors, boundary conditions, and schema validation.

| Test File | Lines | Type | Description |
|-----------|-------|------|-------------|
| `type_like_string_false_positive_test.rs` | 14,580 | Integration | Massive test suite for type-like strings that aren't actual types, YAML tag false positives |
| `invalid_type_conversion_test.rs` | 2,983 | Integration | Invalid type conversions (string→non-string, struct→scalar, array/map→scalar) |
| `int32_to_uint32_boundary_test.rs` | 857 | Integration | int32 to uint32 boundary conditions, negative value rejection |
| `int32_to_uint32_error_detection_test.rs` | 513 | Integration | Negative int32 detection for uint32 fields, error messages |
| `negative_int32_to_uint32_error_verification.rs` | 471 | Integration | Verification of negative value error handling |
| `negative_conversion_error_message_test.rs` | 248 | Unit | Error message formatting for negative conversions |
| `schema_validation_test.rs` | 655 | Integration | Schema validation, field requirements, type constraints |

---

## Category 3: YAML Comment Filtering (13 files, 9,170 lines)

Tests for YAML comment detection, false positive prevention, and edge cases.

| Test File | Lines | Type | Description |
|-----------|-------|------|-------------|
| `yaml_indentation_and_mixed_scenarios_test.rs` | 1,539 | Integration | Comments at various indentations (0-12 spaces), nested structures, mixed scenarios |
| `yaml_folded_multiline_comment_test.rs` | 910 | Integration | Folded block scalars with comments, multi-line scenarios |
| `yaml_block_scalar_indentation_comment_test.rs` | 881 | Integration | Block scalar indentation with comment handling |
| `yaml_comment_edge_case_test.rs` | 861 | Integration | Edge cases in comment detection, special patterns |
| `yaml_comment_false_positive_test.rs` | 820 | Integration | False positive prevention (hashes in values, URLs with anchors) |
| `inline_comment_detection_test.rs` | 757 | Integration | Inline comment detection and stripping |
| `yaml_comment_filtering_edge_cases_test.rs` | 672 | Integration | Edge cases and false positive prevention |
| `yaml_plain_multiline_scalar_comment_test.rs` | 599 | Integration | Plain multiline scalar comment handling |
| `yaml_multiline_quoted_scalar_comment_test.rs` | 536 | Integration | Quoted scalar comment handling |
| `yaml_literal_multiline_comment_test.rs` | 533 | Integration | Literal block scalar comment handling |
| `yaml_comment_position_test.rs` | 388 | Integration | Comment position detection and validation |
| `comment_filtering_basic_test.rs` | 358 | Unit | Basic comment filtering functionality |
| `yaml_indent_without_keys_test.rs` | 354 | Integration | Indentation without keys scenarios |

---

## Category 4: Parse Error Handling (9 files, 3,361 lines)

Tests for error creation, display, propagation, and formatting.

| Test File | Lines | Type | Description |
|-----------|-------|------|-------------|
| `parse_error_full_lifecycle_integration_test.rs` | 743 | Integration | Full lifecycle of ParseError (creation, display, propagation, context) |
| `parse_error_integration_test.rs` | 616 | Integration | ParseError integration with YAML parsing workflows |
| `parse_error_unit_test.rs` | 607 | Unit | Focused ParseError method coverage |
| `parse_error_display_test.rs` | 339 | Unit | ParseError display formatting |
| `parse_error_propagation_test.rs` | 237 | Integration | Error propagation through parsing layers |
| `malformed_error_message_test.rs` | 785 | Integration | Malformed error message handling and recovery |
| `error_message_format_examples_test.rs` | 503 | Unit | Error message format examples and verification |
| `error_code_validation_test.rs` | 307 | Unit | ErrorCode and ErrorType integration verification |
| `error_messages_test.rs` | 52 | Unit | Basic error message functionality |
| `validation_error_format_test.rs` | 264 | Unit | ValidationError formatting and structure |
| `acceptance_criteria_verification_test.rs` | 197 | Integration | Acceptance criteria verification for contextual error formatting |

---

## Category 5: Indentation & Structure Analysis (5 files, 1,545 lines)

Tests for indentation detection, key validation, and structure analysis.

| Test File | Lines | Type | Description |
|-----------|-------|------|-------------|
| `indent_change_detection_test.rs` | 566 | Integration | Indentation change detection and scope transitions |
| `indent_without_key_test.rs` | 356 | Integration | Indentation changes without key scenarios |
| `false_positive_indent_key_test.rs` | 166 | Integration | False positive prevention for indent-key detection |
| `line_classification_test.rs` | 301 | Unit | Line type classification (comment, blank, content) |
| `yaml_folded_scalar_continuation_validation_test.rs` | 769 | Integration | Folded scalar continuation with indentation |
| `missing_colon_comprehensive_test.rs` | 227 | Integration | Missing colon detection and handling |

---

## Category 6: Nested Structure & Duplicate Detection (2 files, 1,112 lines)

Tests for nested structures and duplicate key detection.

| Test File | Lines | Type | Description |
|-----------|-------|------|-------------|
| `nested_duplicate_detection_test.rs` | 878 | Integration | Nested structure duplicate key detection |
| `yaml_folded_scalar_continuation_validation_test.rs` | 769 | Integration | Folded scalar continuation validation |

---

## Category 7: Unit Tests & Smoke Tests (8 files, 1,085 lines)

Basic unit tests and smoke tests for core functionality.

| Test File | Lines | Type | Description |
|-----------|-------|------|-------------|
| `push_scope_unit_test.rs` | 309 | Unit | Focused unit tests for push_scope method |
| `result_dataclass_test.rs` | 87 | Unit | Result dataclass functionality |
| `status_enum_smoke_test.rs` | 50 | Unit | Status enum basic smoke test |

---

## Summary by Test Type

### Integration Tests (19 files)
1. `scope_tracking_comprehensive_test.rs`
2. `scope_stack_verification_test.rs`
3. `sequence_scope_verification_test.rs`
4. `state_preservation_scope_exit_test.rs`
5. `exit_to_scope_edge_cases_test.rs`
6. `target_scope_lookup_test.rs`
7. `type_like_string_false_positive_test.rs`
8. `invalid_type_conversion_test.rs`
9. `int32_to_uint32_boundary_test.rs`
10. `int32_to_uint32_error_detection_test.rs`
11. `negative_int32_to_uint32_error_verification.rs`
12. `schema_validation_test.rs`
13. `yaml_indentation_and_mixed_scenarios_test.rs`
14. `yaml_folded_multiline_comment_test.rs`
15. `yaml_block_scalar_indentation_comment_test.rs`
16. `yaml_comment_edge_case_test.rs`
17. `yaml_comment_false_positive_test.rs`
18. `inline_comment_detection_test.rs`
19. `yaml_comment_filtering_edge_cases_test.rs`
20. `yaml_plain_multiline_scalar_comment_test.rs`
21. `yaml_multiline_quoted_scalar_comment_test.rs`
22. `yaml_literal_multiline_comment_test.rs`
23. `yaml_comment_position_test.rs`
24. `yaml_indent_without_keys_test.rs`
25. `parse_error_full_lifecycle_integration_test.rs`
26. `parse_error_integration_test.rs`
27. `parse_error_propagation_test.rs`
28. `malformed_error_message_test.rs`
29. `acceptance_criteria_verification_test.rs`
30. `indent_change_detection_test.rs`
31. `indent_without_key_test.rs`
32. `false_positive_indent_key_test.rs`
33. `missing_colon_comprehensive_test.rs`
34. `yaml_folded_scalar_continuation_validation_test.rs`
35. `nested_duplicate_detection_test.rs`

### Unit Tests (16 files)
1. `scope_stack_unit_test.rs`
2. `scope_stack_structure_test.rs`
3. `scope_stack_test.rs`
4. `push_scope_unit_test.rs`
5. `negative_conversion_error_message_test.rs`
6. `error_message_format_examples_test.rs`
7. `error_code_validation_test.rs`
8. `error_messages_test.rs`
9. `validation_error_format_test.rs`
10. `line_classification_test.rs`
11. `parse_error_unit_test.rs`
12. `parse_error_display_test.rs`
13. `comment_filtering_basic_test.rs`
14. `result_dataclass_test.rs`
15. `status_enum_smoke_test.rs`

---

## Summary by Feature Area

| Feature Area | Files | Lines | Test Type Ratio |
|--------------|-------|-------|-----------------|
| **Scope Tracking** | 9 | 5,727 | 3 Unit / 6 Integration |
| **Type Conversion** | 7 | 22,503 | 1 Unit / 6 Integration |
| **Comment Filtering** | 13 | 9,170 | 2 Unit / 11 Integration |
| **Parse Error Handling** | 11 | 3,361 | 5 Unit / 6 Integration |
| **Indentation & Structure** | 6 | 1,545 | 1 Unit / 5 Integration |
| **Nested Structures** | 2 | 1,112 | 0 Unit / 2 Integration |
| **Basic Unit Tests** | 3 | 446 | 3 Unit / 0 Integration |

---

## Test Bead References

Many tests reference specific beads for traceability:
- **bf-bdz6iz**: Scope tracking comprehensive tests
- **bf-68arep**: ScopeStack unit tests
- **bf-13c81**: YAML comment filtering edge cases
- **bf-3xefd**: YAML indentation and mixed scenarios
- **bf-463jg**: Indentation level test functions
- **bf-68hqo**: ErrorCode and ErrorType integration
- **bf-355bv**: Contextual error message formatting
- **bf-1ccile**: Sequence scope verification
- **bf-4pkivk**: push_scope unit tests
- **bf-rn9gx**: Type-like string false positive tests
- **bf-63gy6**: Initial infrastructure pattern documentation
- **bf-68ime**: Section 12B comprehensive analysis

---

## Notes

- **Largest test file**: `type_like_string_false_positive_test.rs` (14,580 lines) - Comprehensive false positive prevention for type-like strings
- **Most comprehensive category**: Type Conversion & Validation (22,503 lines total) - Critical for ARMOR's core functionality
- **Highest integration test ratio**: Comment Filtering (11/13 integration) - Requires full YAML parsing context
- **Critical integration tests**: Full lifecycle tests verify end-to-end behavior across multiple components
