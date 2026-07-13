# Exclamation Mark Test Scenarios Summary

**Bead ID:** bf-4wmtf  
**Test File:** `tests/type_like_string_false_positive_test.rs`  
**Total Test Functions:** 182  
**Test Sections:** 15 major categories  

## Overview

This test suite verifies that the YAML parser correctly handles exclamation marks (`!`) in various contexts, ensuring that `!` characters that appear in comments, values, strings, and other non-tag contexts are not incorrectly classified as YAML tags.

## Test Organization by Section

### Section 1: Exclamation Mark in Comments (Not Tags)
**Purpose:** Verify that `!` in comments are treated as comments, not tags.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_exclamation_in_full_line_comment` | Comments starting with `#` containing `!` should be classified as `Comment`, not `Tag` |
| `test_exclamation_only_in_comment` | Comments with only `!` symbol should be `Comment`, not `Tag` |

**Test Cases:**
- `# ! important note`
- `#  TODO: fix this bug!`
- `#  WARNING: this is critical!`
- `# !`, `#  !`, `#!`

### Section 2: Exclamation Mark in Values (Not Tags)
**Purpose:** Verify that `!` appearing in mapping values are treated as `MappingKey`, not tags.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_exclamation_in_quoted_string_value` | `!` inside quoted strings should be `MappingKey`, not `Tag` |
| `test_exclamation_at_end_of_value` | Values ending with `!` should be `MappingKey`, not `Tag` |
| `test_exclamation_in_url_value` | `!` in URLs should be `MappingKey`, not `Tag` |

**Test Cases:**
- `key: "value with ! inside"`
- `key: '!important!'`
- `message: Hello World!`
- `url: http://example.com/path!query`

### Section 3: Exclamation Mark After Colon (Value, Not Tag)
**Purpose:** Verify that `!` appearing after colons in values are not treated as tags.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_exclamation_after_colon_in_value` | `!` in value part after colon should be `MappingKey`, not `Tag` |
| `test_exclamation_after_inline_comment` | `!` in inline comments should not be detected as tag |

**Test Cases:**
- `key: !value`
- `field: something!`
- `key: value # ! important comment`

### Section 4: False Positives - Values That Look Like Tags
**Purpose:** Verify that strings looking like tags but in quoted strings or values are not treated as tags.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_string_values_resembling_tags` | Quoted strings resembling `!tag` should be `MappingKey`, not `Tag` |
| `test_tag_like_patterns_in_values` | Tag-like patterns in values should be `MappingKey`, not `Tag` |

**Test Cases:**
- `description: "This is a !tag in text"`
- `pattern: !important`
- `selector: .class!important`

### Section 5: Exclamation in Sequence Items
**Purpose:** Verify that `!` in sequence items are treated as `SequenceItem`, not tags.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_exclamation_in_sequence_values` | `!` in sequence item values should be `SequenceItem`, not `Tag` |

**Test Cases:**
- `- item with !`
- `- "value!"`
- `- '!important'`

### Section 6: Edge Cases - Ambiguous Exclamation Positions
**Purpose:** Test edge cases where `!` position is ambiguous.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_exclamation_at_line_start_without_space` | Lines starting with `!` should be `Tag` (actual YAML tags) |
| `test_exclamation_at_line_start_with_space` | `!` followed by space should be `Tag` or `Unknown` |
| `test_multiple_exclamation_marks_at_start` | Multiple `!` at start should be `Tag` (YAML `!!` prefix) |
| `test_exclamation_in_indentation_context` | Indented values with `!` should be `MappingKey`, not `Tag` |
| `test_exclamation_at_deep_indentation_as_value` | Deep indentation with `!` should be `MappingKey`, not `Tag` |

**Test Cases:**
- `!tag` (actual tag)
- `!!str` (global tag)
- `! tag` (malformed)
- `  key: value!`

### Section 7: Type-like Strings (Some with !)
**Purpose:** Verify that type-like strings in various contexts are not treated as tags.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_type_like_strings_in_quoted_contexts` | Type-like strings in quotes should be `MappingKey` |
| `test_type_keywords_as_values` | Type keywords as values should be `MappingKey` |
| `test_error_messages_with_multiple_type_references` | Error messages with type references should be `MappingKey` |
| `test_type_like_strings_with_punctuation` | Type-like strings with punctuation should be `MappingKey` |
| `test_type_mentions_at_different_positions` | Type mentions at different positions |
| `test_lowercase_type_variations_in_values` | Lowercase type variations |
| `test_type_like_strings_in_unquoted_error_contexts` | Type-like strings in unquoted error contexts |

### Section 8: Bang Character in Different Contexts
**Purpose:** Test `!` (bang) in various YAML contexts.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_bang_in_flow_style_values` | `!` in flow style mappings/sequences |
| `test_bang_in_block_scalar` | Block scalar indicators should not be affected by `!` |
| `test_bang_after_quoted_value` | `!` after quoted value should still be `MappingKey` |
| `test_bang_in_key_name` | `!` as part of key name should be `MappingKey` |
| `test_bang_with_colon_variations` | Various spacing patterns with colon and `!` |
| `test_bang_at_end_of_quoted_string` | `!` at end of quoted string |
| `test_multiple_consecutive_bangs_in_values` | Multiple consecutive `!` in values |
| `test_bang_in_numeric_contexts` | `!` in numeric contexts |
| `test_bang_with_boolean_like_values` | `!` with boolean-like values |

**Test Cases:**
- `key: {subkey: value!}`
- `key: "value"!`
- `key!name: value`
- `key:!value`

### Section 9: Tag vs Mapping Key Ambiguity
**Purpose:** Test ambiguous cases between tags and mapping keys.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_tag_vs_mapping_key_ambiguity` | Ambiguous cases should be correctly classified |
| `test_tag_like_string_end_of_line` | Tag-like strings at end of line |
| `test_exclamation_with_special_yaml_chars` | `!` with special YAML characters |
| `test_ambiguous_tag_with_trailing_content` | Ambiguous tags with trailing content |

### Section 10: Special YAML Tag Patterns vs False Positives
**Purpose:** Verify valid YAML tags vs false positive patterns.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_valid_yaml_tag_patterns` | Valid YAML tag patterns should be `Tag` |
| `test_valid_yaml_tag_patterns_with_indents` | Indented valid YAML tags should be `Tag` |
| `test_invalid_tag_patterns` | Invalid tag patterns handling |
| `test_tag_like_false_positives_in_values` | Tag-like false positives in values |
| `test_tag_like_false_positives_in_quoted_strings` | Tag-like false positives in quoted strings |
| `test_tag_like_false_positives_in_sequence_items` | Tag-like false positives in sequence items |
| `test_tag_like_false_positives_in_flow_collections` | Tag-like false positives in flow collections |
| `test_actual_yaml_tags_vs_string_values` | Actual YAML tags vs string values |

**Valid Tag Patterns:**
- `!tag`, `!!str`, `!!map`, `!!seq`
- `!custom_type`, `!ns:tag`
- `!my-tag`, `!my_tag`
- `!example.com:tag`

**Invalid Patterns:**
- `!123`, `!$`, `!@tag`
- `! space`, `! tag`
- `!`, `!!`

### Section 11: Whitespace and Exclamation Combinations
**Purpose:** Test various whitespace patterns with `!`.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_whitespace_before_exclamation` | Various whitespace patterns before `!` |
| `test_exclamation_with_special_whitespace` | `!` with Unicode whitespace |
| `test_whitespace_only_before_exclamation` | Whitespace followed by `!` should be `Tag` |
| `test_exclamation_with_whitespace_variations_in_values` | Whitespace variations in values |
| `test_exclamation_in_comments_with_whitespace` | Whitespace in comments with `!` |
| `test_exclamation_with_leading_whitespace_in_mapping_keys` | Leading whitespace in mapping keys |
| `test_exclamation_at_sequence_item_with_whitespace` | Sequence items with whitespace |
| `test_unicode_exclamation_mark_variations` | Unicode exclamation mark variations |
| `test_whitespace_combinations_with_exclamation_in_different_contexts` | Whitespace combinations in different contexts |
| `test_tab_vs_space_before_exclamation` | Tab vs space before `!` |
| `test_extended_unicode_whitespace_with_exclamation` | Extended Unicode whitespace |
| `test_zero_width_characters_with_exclamation` | Zero-width characters |
| `test_multiple_consecutive_whitespace_before_exclamation` | Multiple consecutive whitespace |
| `test_exclamation_with_whitespace_in_flow_sequences` | Flow sequences with whitespace |
| `test_exclamation_with_whitespace_in_flow_mappings` | Flow mappings with whitespace |
| `test_exclamation_at_different_positions_after_whitespace` | Different positions after whitespace |
| `test_unicode_exclamation_with_whitespace_combinations` | Unicode with whitespace |
| `test_whitespace_preserves_yaml_tag_detection` | Whitespace preserves tag detection |
| `test_edge_case_long_whitespace_sequences` | Long whitespace sequences |
| `test_whitespace_in_sequence_items_with_exclamation` | Sequence items with whitespace |

**Unicode Whitespace Tests:**
- Zero-width space (`\u{200B}`)
- Ideographic space (`\u{3000}`)
- Non-breaking space (`\u{00A0}`)
- En space, Em space, Thin space

### Section 12: Type-like Strings with Typos
**Purpose:** Test type-like strings with common typos and variations.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_type_name_typos_in_values` | Type name typos |
| `test_type_name_case_variations` | Case variations |
| `test_partial_type_matches_in_values` | Partial matches |
| `test_common_type_misspellings` | Common misspellings |
| `test_transposed_letter_typos` | Transposed letters |
| `test_double_letter_typos` | Double letter errors |
| `test_missing_letter_typos` | Missing letters |
| `test_extra_letter_typos` | Extra letters |
| `test_type_name_with_numbers` | Type names with numbers |
| `test_type_name_with_underscores` | Type names with underscores |
| `test_type_name_with_hyphens` | Type names with hyphens |
| `test_reversed_type_names` | Reversed type names |
| `test_alternative_type_names` | Alternative names |
| `test_programming_language_types` | Programming language types |
| `test_sql_data_types` | SQL data types |
| `test_json_schema_types` | JSON schema types |
| `test_truncated_type_names` | Truncated names |
| `test_all_lowercase_vs_uppercase_variations` | Case variations |
| `test_reversed_and_scrambled_type_names` | Reversed/scrambled names |
| `test_keyboard_adjacent_typos` | Keyboard adjacent typos |
| `test_repeated_character_typos` | Repeated character typos |
| `test_vowel_substitution_typos` | Vowel substitutions |
| `test_type_name_leading_trailing_junk` | Leading/trailing junk |
| `test_type_name_in_context_of_error_message` | Type names in error messages |
| `test_multiple_typos_in_single_type_name` | Multiple typos |

### Section 13: Type-like Strings in Complex Contexts
**Purpose:** Test type-like strings in various nested and complex contexts.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_detect_mapping_key_with_type_name_typos` | Mapping key detection with typos |
| `test_detect_mapping_key_with_typos_in_quoted_values` | Typos in quoted values |
| `test_detect_mapping_key_with_typos_in_error_messages` | Typos in error messages |
| `test_detect_mapping_key_with_keyboard_adjacent_typos` | Keyboard adjacent typos |
| `test_detect_mapping_key_with_vowel_substitution_typos` | Vowel substitution typos |
| `test_detect_mapping_key_with_leading_trailing_junk` | Leading/trailing junk |
| `test_detect_mapping_key_with_multiple_typos_combined` | Multiple combined typos |

### Section 14: Type-like Strings in Nested Structures
**Purpose:** Test type-like strings in various nested structures.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_type_like_in_nested_structures` | Nested structures |
| `test_type_like_with_special_chars` | With special characters |
| `test_type_like_in_multiline_values` | Multiline values |
| `test_type_like_in_documentation` | Documentation contexts |
| `test_type_like_in_error_descriptions` | Error descriptions |
| `test_type_like_in_config_descriptions` | Config descriptions |
| `test_type_like_in_validation_messages` | Validation messages |
| `test_type_like_in_api_responses` | API responses |
| `test_type_like_in_schema_definitions` | Schema definitions |
| `test_type_like_in_log_messages` | Log messages |
| `test_type_like_in_comments_inline` | Inline comments |
| `test_type_like_with_exclamation_complex` | Complex with `!` |
| `test_type_like_in_enum_values` | Enum values |
| `test_type_like_in_regex_patterns` | Regex patterns |
| `test_type_like_in_conversion_contexts` | Conversion contexts |
| `test_type_like_in_function_descriptions` | Function descriptions |
| `test_type_like_in_template_strings` | Template strings |
| `test_type_like_in_nested_flow_collections` | Nested flow collections |
| `test_type_like_in_mixed_collections` | Mixed collections |
| `test_type_like_in_deeply_nested_mappings` | Deeply nested mappings |
| `test_type_like_in_complex_production_yaml` | Complex production YAML |
| `test_type_like_in_nested_sequence_structures` | Nested sequences |
| `test_type_like_in_flow_collection_nesting` | Flow collection nesting |
| `test_type_like_in_kubernetes_style_config` | Kubernetes-style configs |
| `test_type_like_in_hierarchical_config_tree` | Hierarchical config trees |
| `test_type_like_in_microservice_architecture_config` | Microservice configs |
| `test_type_like_in_data_pipeline_config` | Data pipeline configs |
| `test_type_like_in_multi_environment_config` | Multi-environment configs |
| `test_type_like_in_nested_validation_schemas` | Nested validation schemas |
| `test_type_like_in_complex_flow_sequences` | Complex flow sequences |
| `test_type_like_in_realistic_app_config` | Realistic app configs |
| `test_type_like_in_edge_case_nested_structures` | Edge case nested structures |
| `test_type_like_in_mixed_indentation_scenarios` | Mixed indentation |

### Section 15: Real-world Configuration Files
**Purpose:** Test exclamation marks in realistic configuration scenarios.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_detect_mapping_key_in_nested_context` | Nested context detection |
| `test_complete_extraction_pipeline_verification` | Complete pipeline verification |
| `test_real_world_config_with_exclamation` | Real-world configs |
| `test_production_yaml_app_config` | Production app configs |
| `test_cicd_pipeline_config` | CI/CD pipeline configs |
| `test_kubernetes_deployment_config` | Kubernetes deployment configs |
| `test_database_connection_config` | Database connection configs |
| `test_logging_config_with_exclamation` | Logging configs |
| `test_feature_flags_config` | Feature flag configs |
| `test_api_gateway_config` | API gateway configs |
| `test_docker_compose_config` | Docker Compose configs |
| `test_monitoring_alerts_config` | Monitoring/alert configs |
| `test_message_template_config` | Message templates |
| `test_simple_message_patterns_with_exclamation` | Simple message patterns |
| `test_basic_config_patterns_exclamation_variations` | Basic config patterns |
| `test_css_and_ui_config` | CSS and UI configs |
| `test_build_configuration` | Build configurations |
| `test_security_config` | Security configurations |
| `test_multiline_scenario_with_exclamation` | Multiline scenarios |
| `test_complex_multiline_production_config` | Complex multiline configs |
| `test_multiline_with_inline_comments_and_exclamation` | Multiline with comments |
| `test_quoted_values_with_exclamation_variations` | Quoted value variations |
| `test_real_world_env_config` | Environment configs |
| `test_microservices_config` | Microservice configs |
| `test_deployment_strategy_config` | Deployment strategy configs |
| `test_rate_limiting_config` | Rate limiting configs |
| `test_mixed_quoted_unquoted_values_with_exclamation` | Mixed quoted/unquoted |
| `test_complex_user_interface_messages` | UI messages |
| `test_api_response_messages_with_exclamation` | API response messages |
| `test_configuration_validation_messages` | Validation messages |
| `test_complex_multiline_block_with_exclamation` | Complex multiline blocks |
| `test_web_server_configuration_with_exclamation` | Web server configs |
| `test_notification_system_config` | Notification configs |
| `test_backup_and_storage_config` | Backup/storage configs |
| `test_performance_tuning_config` | Performance tuning configs |
| `test_mobile_app_configuration` | Mobile app configs |
| `test_cloud_infrastructure_config` | Cloud infrastructure configs |
| `test_content_management_config` | Content management configs |
| `test_email_notification_templates` | Email templates |
| `test_load_balancer_config` | Load balancer configs |
| `test_cdn_configuration` | CDN configs |
| `test_message_queue_configuration` | Message queue configs |
| `test_analytics_tracking_config` | Analytics configs |
| `test_developer_portal_config` | Developer portal configs |
| `test_internationalization_config` | Internationalization configs |

### Section 16: Multiline YAML Strings with Exclamation
**Purpose:** Test exclamation marks in multiline YAML string contexts.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_multiline_yaml_strings_with_exclamation` | Multiline strings |
| `test_folded_style_scalars_with_exclamation` | Folded style scalars |
| `test_literal_style_scalars_with_exclamation` | Literal style scalars |
| `test_mixed_multiline_with_singleline_exclamation_patterns` | Mixed multiline/singleline |
| `test_folded_and_literal_mixed_contexts` | Mixed folded/literal |
| `test_multiline_edge_cases_with_exclamation` | Multiline edge cases |

### Section 17: Folded/Literal Scalar Variations
**Purpose:** Test exclamation marks in various block scalar contexts.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_folded_block_scalar_with_exclamation_marks` | Folded block scalars |
| `test_literal_block_scalar_with_exclamation_marks` | Literal block scalars |
| `test_multiline_mixed_with_singleline_exclamation_patterns` | Mixed patterns |
| `test_multiline_yaml_strings_with_exclamation_in_nested_contexts` | Nested contexts |
| `test_folded_scalar_exclamation_at_different_positions` | Different positions |
| `test_literal_scalar_exclamation_at_different_positions` | Literal positions |
| `test_multiline_block_scalar_modifiers_with_exclamation` | Block scalar modifiers |
| `test_real_world_multiline_config_with_exclamation` | Real-world multiline configs |
| `test_multiline_comment_and_config_mixed_with_exclamation` | Comments and configs |
| `test_multiline_sequence_with_exclamation_in_block_scalars` | Sequences in block scalars |

### Section 18: Comprehensive Indentation Tests
**Purpose:** Test exclamation marks at various indentation levels.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_various_indentation_levels_with_exclamation_marks` | Various indentation levels |
| `test_folded_scalar_indicator_lines` | Indicator lines |
| `test_folded_scalar_basic_modifiers` | Basic modifiers |
| `test_folded_scalar_numeric_modifiers` | Numeric modifiers |
| `test_folded_scalar_indented_indicators` | Indented indicators |
| `test_folded_scalar_all_modifier_combinations` | All modifier combinations |
| `test_folded_scalar_indicator_classification` | Indicator classification |
| `test_folded_scalar_continuation_lines_with_exclamation` | Continuation lines |
| `test_tab_indented_folded_scalars_with_exclamation` | Tab indentation |
| `test_folded_scalar_various_indentation_levels` | Various levels |
| `test_folded_scalar_modifiers_comprehensive` | Comprehensive modifiers |
| `test_folded_scalar_exclamation_positions_comprehensive` | Comprehensive positions |
| `test_basic_folded_scalar_indicator_as_mapping_key` | Indicator as mapping key |
| `test_folded_scalar_with_continuation_content` | Continuation content |
| `test_folded_scalar_continuation_lines_with_exclamation_marks` | Continuation with `!` |
| `test_folded_scalar_continuation_lines_starting_with_exclamation` | Starting with `!` |
| `test_folded_scalar_continuation_exclamation_various_contexts` | Various contexts |
| `test_comprehensive_tab_indented_folded_scalars_with_exclamation` | Comprehensive tab tests |
| `test_comprehensive_various_indentation_levels_with_exclamation` | Comprehensive indentation |
| `test_mixed_indentation_scenarios_with_folded_scalars` | Mixed scenarios |
| `test_odd_indentation_levels_with_exclamation_marks` | Odd indentation levels |
| `test_deep_indentation_levels_with_exclamation_marks` | Deep indentation levels |
| `test_extensive_tab_indentation_with_exclamation_marks` | Extensive tab indentation |
| `test_complex_mixed_indentation_with_exclamation_marks` | Complex mixed indentation |
| `test_various_indentation_levels_with_exclamation_mark` | Final indentation tests |

### Section 19: Error Code Patterns
**Purpose:** Test exclamation marks in error code contexts.

| Test Function | What It Verifies |
|---------------|------------------|
| `test_error_code_patterns_in_values` | Error code patterns |
| `test_invalid_error_code_formats` | Invalid formats |
| `test_error_codes_with_descriptions` | With descriptions |
| `test_multiple_error_codes_in_values` | Multiple error codes |
| `test_error_code_case_variations` | Case variations |
| `test_error_codes_in_nested_structures` | Nested structures |
| `test_delimiter_error_variations` | Delimiter variations |
| `test_error_codes_with_context` | With context |
| `test_warning_and_info_codes` | Warning/info codes |
| `test_critical_error_codes` | Critical errors |
| `test_error_codes_with_special_separators` | Special separators |
| `test_error_code_boundaries` | Error code boundaries |
| `test_mixed_error_types_in_sequence` | Mixed error types |
| `test_error_codes_in_quoted_strings` | In quoted strings |
| `test_error_codes_with_exclamation` | With `!` |
| `test_custom_error_code_formats` | Custom formats |
| `test_hex_error_codes` | Hexadecimal codes |

## Summary

The test suite contains **182 test functions** organized into **19 major sections** that comprehensively cover exclamation mark handling in YAML parsing:

1. **Comment contexts** (2 tests) - `!` in comments should be treated as comments
2. **Value contexts** (3 tests) - `!` in values should be treated as mapping keys
3. **After-colon contexts** (2 tests) - `!` after colons in values
4. **False positive patterns** (2 tests) - Strings looking like tags but aren't
5. **Sequence contexts** (1 test) - `!` in sequence items
6. **Edge cases** (5 tests) - Ambiguous positions and indentation
7. **Type-like strings** (7 tests) - Type-like strings in various contexts
8. **Bang character contexts** (9 tests) - `!` in various YAML contexts
9. **Tag vs key ambiguity** (4 tests) - Ambiguous classification cases
10. **Valid/invalid tag patterns** (8 tests) - YAML tag pattern verification
11. **Whitespace combinations** (20 tests) - Various whitespace patterns with `!`
12. **Type typos** (27 tests) - Type-like strings with typos
13. **Complex typos** (7 tests) - Multiple combined typos
14. **Nested structures** (30 tests) - Type-like in nested contexts
15. **Real-world configs** (48 tests) - Production configuration scenarios
16. **Multiline strings** (6 tests) - Multiline YAML contexts
17. **Block scalars** (10 tests) - Folded/literal scalar contexts
18. **Indentation levels** (23 tests) - Various indentation scenarios
19. **Error codes** (17 tests) - Error code patterns with `!`

## Key Insights

1. **Comprehensive Coverage**: The tests cover virtually every context where `!` can appear in YAML documents
2. **False Positive Prevention**: Emphasis on ensuring `!` in non-tag contexts aren't misclassified as tags
3. **Real-world Scenarios**: Extensive testing of production configuration patterns
4. **Unicode Support**: Testing of Unicode whitespace and exclamation mark variations
5. **Edge Case Coverage**: Thorough testing of ambiguous positions and special cases

## Test Bead Reference

This test suite was created as part of bead **bf-rn9gx** with the following acceptance criteria:
- Test messages with type-like strings that aren't real types
- Test false positive scenarios
- Verify extraction correctly rejects these cases

---

**Generated:** 2026-07-13  
**Task:** bf-4wmtf - Identify exclamation mark test scenarios  
**File:** tests/type_like_string_false_positive_test.rs  
**Total Lines:** 9,066+ lines of comprehensive test coverage
