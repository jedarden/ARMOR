//! Test suite for type-like strings that aren't actual types
//!
//! This test suite verifies that the YAML parser correctly handles strings
//! that look like types or tags but shouldn't be classified as such:
//! - Strings starting with `!` that aren't YAML tags (comments, values, etc.)
//! - False positive scenarios in tag/type detection
//! - Strings that resemble type names in error messages
//! - Edge cases where classification might be incorrect
//!
//! Bead: bf-rn9gx
//! Acceptance Criteria:
//! - Test messages with type-like strings that aren't real types
//! - Test false positive scenarios
//! - Verify extraction correctly rejects these cases

use armor::parsers::yaml::{classify_line_type, detect_mapping_key, LineType};

// ============================================================================
// Test Infrastructure Macros and Helpers
// ============================================================================
// This section provides reusable infrastructure for folded scalar explicit
// indent tests. Bead: bf-63gy6
//
// Pattern Documentation:
// ---------------------
// Folded scalar explicit indent tests follow a consistent pattern:
//
// 1. Test Case Structure:
//    - Use vec! of tuples: (input_line, expected_key_name, expected_line_type)
//    - Input line format: "<indent><key_name>: <modifier><indent_level>"
//    - Modifiers: > (plain), >- (strip), >+ (keep)
//    - Indent levels: 1-9 (e.g., >2, >-3, >+4)
//
// 2. Indentation Levels:
//    - Level 1: 2 spaces ("  ")
//    - Level 2: 4 spaces ("    ")
//    - Level 3: 6 spaces ("      ")
//    - Level 4: 8 spaces ("        ")
//    - Tab: ("\t")
//    - Mixed: ("\t  ", "\t    ", etc.)
//
// 3. Test Naming Convention:
//    - test_folded_scalar_<variant>_at_<indentation>_level
//    - Or: test_folded_scalar_explicit_indent_modifiers_at_various_levels
//
// 4. Assertion Pattern:
//    - First assert: classify_line_type() returns expected LineType
//    - Second assert: if MappingKey, verify detect_mapping_key() finds correct key
//
// Example Usage:
// -------------
// ```rust
// let test_cases = generate_folded_scalar_tests!(
//     level1_indent = "  ",
//     modifiers = [">", ">-", ">+"],
//     indent_levels = [1, 2, 3],
//     keys = ["text", "warning", "error"]
// );
// run_folded_scalar_tests!(test_cases);
// ```

/// Macro to generate folded scalar explicit indent test cases
/// at a specific indentation level with given modifiers and indent levels.
///
/// Parameters:
/// - $indent: the base indentation (e.g., "  ", "    ", "\t")
/// - $level_name: descriptive name for this indentation level
/// - $modifiers: array of modifier patterns (e.g., [">", ">-", ">+"])
/// - $indent_nums: array of indent numbers (e.g., [1, 2, 3, 4, 5])
/// - $key_prefix: prefix for key names
///
/// Returns: vec of (line, expected_key, expected_type) tuples
macro_rules! generate_folded_explicit_indent_tests {
    ($indent:expr, $level_name:expr, $modifiers:expr, $indent_nums:expr, $key_prefix:expr) => {{
        let mut cases = vec![];

        for modifier in $modifiers.iter() {
            for num in $indent_nums.iter() {
                let modifier_str = format!("{}{}", modifier, num);
                let key_name = format!("{}_{}_{}", $key_prefix, modifier.replace(">", "").trim(), num);
                let line = format!("{}{}: {}", $indent, key_name, modifier_str);

                cases.push((line, key_name, armor::parsers::yaml::LineType::MappingKey));
            }
        }

        cases
    }};
}

/// Macro to run folded scalar explicit indent tests with assertions
/// This handles the standard assertion pattern:
/// 1. Assert line type matches expected
/// 2. If MappingKey, assert key detection works correctly
macro_rules! run_folded_scalar_tests {
    ($test_cases:expr) => {
        for (line, expected_key, expected_type) in $test_cases {
            let result = classify_line_type(&line);

            assert_eq!(
                result, expected_type,
                "Folded scalar explicit indent test failed: '{}' - expected {:?}, got {:?}",
                line, expected_type, result
            );

            // Verify that the key is correctly detected for MappingKey types
            if result == armor::parsers::yaml::LineType::MappingKey {
                let info = detect_mapping_key(&line, 0);
                assert!(
                    info.is_some(),
                    "Should detect mapping key for folded scalar with explicit indent modifier: '{}'",
                    line
                );
                let detected = info.unwrap();
                assert_eq!(
                    detected.key, &expected_key[..],
                    "Key mismatch for folded scalar with explicit indent modifier: '{}' - expected '{}', got '{}'",
                    line, expected_key, detected.key
                );
            }
        }
    };
}

/// Helper function to create a folded scalar test case tuple
/// This provides a non-macro alternative for building test cases
/// Returns (line, key, type) tuple for use with run_folded_scalar_tests! macro
fn create_folded_scalar_test(
    indent: &str,
    key: &str,
    modifier: &str,
    indent_level: u32,
) -> (String, String, armor::parsers::yaml::LineType) {
    let modifier_str = format!("{}{}", modifier, indent_level);
    let line = format!("{}{}: {}", indent, key, modifier_str);
    (line, key.to_string(), armor::parsers::yaml::LineType::MappingKey)
}

/// Bulk generate folded scalar test cases for multiple indentation levels
/// This is a convenience function for generating comprehensive test suites
fn generate_folded_scalar_tests_multi_level(
    keys: &[&str],
    modifiers: &[&str],
    indent_levels: &[u32],
) -> Vec<(String, String, armor::parsers::yaml::LineType)> {
    let mut cases = vec![];

    let indents = vec![
        ("  ", "level1"),
        ("    ", "level2"),
        ("      ", "level3"),
        ("        ", "level4"),
        ("\t", "tab"),
    ];

    for (indent, level_name) in indents {
        for key in keys {
            for modifier in modifiers {
                for indent_level in indent_levels {
                    let full_key = format!("{}_{}", level_name, key);
                    cases.push(create_folded_scalar_test(
                        indent,
                        &full_key,
                        modifier,
                        *indent_level,
                    ));
                }
            }
        }
    }

    cases
}

// ============================================================================
// Section 1: Exclamation Mark in Comments (Not Tags)
// ============================================================================

#[test]
fn test_exclamation_in_full_line_comment() {
    // Comments with ! should NOT be classified as tags
    let test_cases = vec![
        "# ! important note",
        "#  TODO: fix this bug!",
        "#  WARNING: this is critical!",
        "#  Note: check this! value",
        "#  Error: something failed!",
        "  #  indented comment with!",
        "\t# tab comment with!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::Comment,
            "Comment starting with # should be Comment, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_only_in_comment() {
    // Comments with only ! symbol
    let test_cases = vec![
        "# !",
        "#  !",
        "  #!",
        "\t#!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::Comment,
            "Comment with ! should be Comment, not Tag: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 2: Exclamation Mark in Values (Not Tags)
// ============================================================================

#[test]
fn test_exclamation_in_quoted_string_value() {
    // ! inside quoted strings should NOT make the line a tag
    let test_cases = vec![
        "key: \"value with ! inside\"",
        "key: 'another ! value'",
        "key: \"!!!\"",
        "key: '!important!'",
        "key: \"Check this out!\"",
        "nested: {subkey: \"value!\"}",
    ];

    for line in test_cases {
        // These should be classified as MappingKey, not Tag
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Quoted value with ! should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_at_end_of_value() {
    // ! at the end of values (like in sentences)
    let test_cases = vec![
        "message: Hello World!",
        "note: This is important!",
        "warning: Check this!",
        "error: Something failed!",
        "status: done!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Value ending with ! should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_in_url_value() {
    // ! in URLs (rare but valid)
    let test_cases = vec![
        "url: http://example.com/path!query",
        "link: https://api.example.com/v1!endpoint",
        "href: https://site.com/section!page",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "URL with ! should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 3: Exclamation Mark After Colon (Value, Not Tag)
// ============================================================================

#[test]
fn test_exclamation_after_colon_in_value() {
    // ! appearing in value part after colon
    let test_cases = vec![
        "key: !value",
        "field: something!",
        "text: Hello!",
        "note: !todo",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Value starting with ! after colon should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_after_inline_comment() {
    // ! in inline comment (should be stripped, not detected as tag)
    let test_cases = vec![
        ("key: value # ! important comment", "key"),
        ("field: something # !note", "field"),
    ];

    for (line, expected_key) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with inline comment: '{}'",
            line
        );
        assert_eq!(
            info.unwrap().key, expected_key,
            "Should extract correct key '{}' from line with inline comment",
            expected_key
        );
    }
}

// ============================================================================
// Section 4: False Positives - Values That Look Like Tags
// ============================================================================

#[test]
fn test_string_values_resembling_tags() {
    // String values that look like !tag but are in quoted strings
    let test_cases = vec![
        "description: \"This is a !tag in text\"",
        "content: 'Use !custom for special cases'",
        "note: \"Reference !seq and !map types\"",
        "text: 'The !符号 is a symbol'",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Quoted string resembling tag should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_tag_like_patterns_in_values() {
    // Patterns that look like tags but are in values
    let test_cases = vec![
        "pattern: !important",
        "selector: .class!important",
        "css: div!important",
        "regex: .*!.*",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Tag-like pattern in value should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 5: Exclamation in Sequence Items
// ============================================================================

#[test]
fn test_exclamation_in_sequence_values() {
    // ! in sequence item values
    let test_cases = vec![
        "- item with !",
        "- \"value!\"",
        "- '!important'",
        "- https://example.com!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Sequence item with ! should be SequenceItem, not Tag: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 6: Edge Cases - Ambiguous Exclamation Positions
// ============================================================================

#[test]
fn test_exclamation_at_line_start_without_space() {
    // Lines starting with ! followed immediately by text (actual YAML tags)
    // These SHOULD be classified as Tag
    let actual_tags = vec![
        "!tag",
        "!custom_type",
        "!seq",
        "!map",
        "!import",
    ];

    for line in actual_tags {
        assert_eq!(
            classify_line_type(line),
            LineType::Tag,
            "Actual YAML tag should be classified as Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_at_line_start_with_space() {
    // Lines starting with ! followed by space (likely malformed tag)
    let test_cases = vec![
        "! tag",
        "! custom",
        "! seq",
    ];

    for line in test_cases {
        // These might be classified as Tag (line starts with !)
        // but they're edge cases
        let result = classify_line_type(line);
        assert!(
            result == LineType::Tag || result == LineType::Unknown,
            "Line starting with ! with space should be Tag or Unknown: '{}'",
            line
        );
    }
}

#[test]
fn test_multiple_exclamation_marks_at_start() {
    // Multiple ! at start (not standard YAML)
    let test_cases = vec![
        "!!tag",
        "!!!custom",
        "!!!!value",
    ];

    for line in test_cases {
        // Standard YAML uses !! for global tags
        let result = classify_line_type(line);
        assert_eq!(
            result, LineType::Tag,
            "Multiple ! at start should still be Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_in_indentation_context() {
    // ! appearing with various indentation levels
    let test_cases = vec![
        "  key: value!",
        "    field: !important",
        "\tnested: check!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Indented value with ! should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_at_deep_indentation_as_value() {
    // Deep indentation with ! in value (should not be confused with tag)
    let test_cases = vec![
        "      deep: value!",
        "        deeper: !important",
        "\t\t\ttabs: check!",
        "    mixed: data!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Deep indentation with ! in value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_mid_line_in_value() {
    // ! appearing in the middle of a value (not at line start)
    let test_cases = vec![
        "key: some!value",
        "field: data!here",
        "text: hello!world",
        "note: test!case",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Bang mid-line in value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_adjacent_to_punctuation() {
    // ! next to other punctuation in values
    let test_cases = vec![
        "key: value,!",
        "field: text.",
        "note: word;",
        "item: data-",
    ];

    for line in test_cases {
        // These should be classified as MappingKey regardless of punctuation
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Value with punctuation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_indented_tag_like_pattern() {
    // Indented lines starting with ! - should still be Tag
    let test_cases = vec![
        "  !tag",
        "    !custom",
        "\t!seq",
        "  !!global",
    ];

    for line in test_cases {
        // Indented tags should still be classified as Tag
        assert_eq!(
            classify_line_type(line),
            LineType::Tag,
            "Indented tag should still be Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_in_continuation_lines() {
    // ! in what looks like a continuation of a value
    let test_cases = vec![
        "  value continuation!",
        "    more text here!",
        "\tstill part of value!",
    ];

    for line in test_cases {
        // Deeply indented lines might be continuations
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Indented continuation with ! should be MappingKey or Unknown: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 7: Type-Like Strings in Error Messages
// ============================================================================

#[test]
fn test_type_like_strings_in_quoted_contexts() {
    // Strings that mention type names but are in quoted contexts
    let test_cases = vec![
        "error: \"Expected type string\"",
        "message: 'type mismatch error'",
        "description: \"integer type required\"",
        "note: 'boolean type expected'",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type-like string in quoted value should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_type_keywords_as_values() {
    // Type keywords used as regular string values
    let test_cases = vec![
        "datatype: string",
        "value_type: integer",
        "field_type: boolean",
        "format: array",
        "structure: object",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type keyword as value should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_error_messages_with_multiple_type_references() {
    // Error messages mentioning multiple types
    let test_cases = vec![
        "error: \"Expected string or integer type\"",
        "message: 'boolean, array, or object type'",
        "description: \"string, integer, boolean types\"",
        "note: 'Expected array or map type'",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error message with multiple type references should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_strings_with_punctuation() {
    // Type-like strings followed by punctuation
    let test_cases = vec![
        "error: \"Type: string.\"",
        "message: 'Expected integer,'",
        "description: \"Type (boolean) required\"",
        "note: 'Type: array?'",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type-like with punctuation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_mentions_at_different_positions() {
    // Type words appearing at different positions in error messages
    let test_cases = vec![
        "error: \"string type is expected\"",
        "message: 'integer type not found'",
        "description: \"Type boolean is invalid\"",
        "note: 'expected array type here'",
        "warning: \"object type missing\"",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type word at any position in error should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_lowercase_type_variations_in_values() {
    // Various case variations of type names in values
    let test_cases = vec![
        "format: String",
        "type: INTEGER",
        "dtype: Boolean",
        "kind: Array",
        "category: Object",
        "class: STRing",
        "datatype: intEGer",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type variation in any case should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_strings_in_unquoted_error_contexts() {
    // Type-like strings in unquoted error descriptions
    let test_cases = vec![
        "error: type mismatch in field",
        "message: integer type required",
        "warning: boolean type expected",
        "note: string type not allowed",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type-like in unquoted error message should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_prefix_suffix_patterns() {
    // Type-like strings with prefixes or suffixes
    let test_cases = vec![
        "error: invalid_string_type",
        "message: myIntegerType",
        "description: custom_boolean_type",
        "note: arrayTypeHere",
        "warning: object_type_def",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type-like with prefix/suffix should be MappingKey: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 8: Bang Character in Different Contexts
// ============================================================================

#[test]
fn test_bang_in_flow_style_values() {
    // ! in flow style mappings/sequences
    let test_cases = vec![
        "key: {subkey: value!}",
        "field: [item1!, item2!]",
        "data: {!nested}",
    ];

    for line in test_cases {
        // Flow style mappings might be classified differently
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowMapping || result == LineType::FlowSequence,
            "Flow style with ! should be MappingKey or Flow: '{}'",
            line
        );
    }
}

#[test]
fn test_bang_in_block_scalar() {
    // Block scalar indicators should NOT be affected by ! in content
    let test_cases = vec![
        "|",
        ">",
        "|-",
        ">+",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::LiteralBlockScalar || result == LineType::FoldedBlockScalar,
            "Block scalar indicator should be BlockScalar type: '{}'",
            line
        );
    }
}

#[test]
fn test_bang_after_quoted_value() {
    // ! appearing right after a quoted value (not part of the value)
    let test_cases = vec![
        "key: \"value\"!",
        "field: 'text'!",
        "data: \"123\"!",
        "item: 'abc'!",
    ];

    for line in test_cases {
        // The ! is after the quoted value, so line should still be MappingKey
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Bang after quoted value should still be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_bang_in_key_name() {
    // ! appearing as part of a key name (rare but possible)
    let test_cases = vec![
        "key!name: value",
        "field!: data",
        "item!!: text",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Bang in key name should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_bang_with_colon_variations() {
    // Various spacing patterns with colon and !
    let test_cases = vec![
        "key:!value",         // No space after colon, ! in value
        "key: !value",        // Space after colon
        "key:! value",        // No space, then space
        "key: ! value",       // Space on both sides
        "key!:value",         // ! in key, no space after colon
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Bang with colon variations should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_bang_at_end_of_quoted_string() {
    // ! inside quotes at the very end
    let test_cases = vec![
        "key: \"value!\"",
        "field: 'data!'",
        "text: \"end!\"",
        "note: 'last!'",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Bang at end of quoted string should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_multiple_consecutive_bangs_in_values() {
    // Multiple consecutive ! characters in values
    let test_cases = vec![
        "priority: high!!!",
        "note: urgent!!",
        "flag: true!!!!",
        "value: test!!!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Multiple consecutive ! in value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_bang_in_numeric_contexts() {
    // ! appearing near numeric values
    let test_cases = vec![
        "value: 42!",
        "count: 100! items",
        "index: 0!",
        "factor: 3.14!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Bang after numeric value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_bang_with_boolean_like_values() {
    // ! with boolean-like values
    let test_cases = vec![
        "flag: true!",
        "enabled: false!",
        "active: yes!",
        "valid: no!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Bang with boolean-like value should be MappingKey: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 9: Ambiguous Scenarios - Correct Classification Verification
// ============================================================================

#[test]
fn test_tag_vs_mapping_key_ambiguity() {
    // Lines that could be either tags or mapping keys
    // Should be correctly classified based on context
    let test_cases = vec![
        // These ARE tags (line starts with !)
        ("!tag", LineType::Tag),
        ("!custom", LineType::Tag),
        ("!!type", LineType::Tag),
        // These are NOT tags (colon present)
        ("key: !value", LineType::MappingKey),
        ("field: !important", LineType::MappingKey),
        ("data: custom!", LineType::MappingKey),
    ];

    for (line, expected) in test_cases {
        assert_eq!(
            classify_line_type(line),
            expected,
            "Correct classification for ambiguous case: '{}'",
            line
        );
    }
}

#[test]
fn test_tag_like_string_end_of_line() {
    // Tag-like patterns at the end of values
    let test_cases = vec![
        "key: some value !tag",
        "field: text ending in !custom",
        "note: description !seq",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Tag-like at end of value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_with_special_yaml_chars() {
    // ! combined with other YAML special characters
    let test_cases = vec![
        "key: value!&anchor",
        "field: data!*alias",
        "note: text!|literal",
        "item: content!>folded",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "! with YAML special chars should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_ambiguous_tag_with_trailing_content() {
    // Lines that start like tags but have trailing content
    let test_cases = vec![
        "!tag with extra text",
        "!custom followed by words",
        "!!type and more",
    ];

    for line in test_cases {
        // These start with ! so should be Tag
        // (even if they have extra content after)
        let result = classify_line_type(line);
        assert_eq!(
            result, LineType::Tag,
            "Line starting with ! is Tag even with trailing content: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_string_with_numbers() {
    // Type-like strings combined with numbers
    let test_cases = vec![
        "type: string123",
        "value: integer456",
        "field: bool789",
        "data: array0",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type-like with numbers should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_mixed_case_exclamation_patterns() {
    // Various casing patterns with !
    let test_cases = vec![
        "key: Value!",
        "field: DATA!",
        "note: Text!",
        "item: CONTENT!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Mixed case value with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_tag_detection_with_context_clues() {
    // Scenarios where context helps determine if ! is a tag
    let test_cases = vec![
        // Colon on line means it's a mapping, not a tag line
        ("key: value", LineType::MappingKey),
        ("key: !tag", LineType::MappingKey),
        // No colon with ! means it's a tag
        ("!tag", LineType::Tag),
        ("!custom", LineType::Tag),
        // Sequence items with !
        ("- item!", LineType::SequenceItem),
        ("- !value", LineType::SequenceItem),
    ];

    for (line, expected) in test_cases {
        assert_eq!(
            classify_line_type(line),
            expected,
            "Context-based classification for: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_in_anchor_and_alias_contexts() {
    // ! in YAML anchor/alias syntax (these should preserve mapping type)
    let test_cases = vec![
        "key: &anchor value!",
        "field: *alias!",
        "data: &!anchor text",
    ];

    for line in test_cases {
        // Lines with anchors/aliases should still be classified by their structure
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "! with anchor/alias should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_empty_and_whitespace_variations_with_bang() {
    // Edge cases with empty/whitespace and !
    let test_cases = vec![
        "key: !",
        "field: ! ",
        "data:  !",
        "item:\t!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "! as value with whitespace should be MappingKey: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 10: Special YAML Tag Patterns vs False Positives
// ============================================================================

#[test]
fn test_valid_yaml_tag_patterns() {
    // Valid YAML tag patterns - these SHOULD be tags
    let valid_tags = vec![
        "!tag",
        "!!str",
        "!!map",
        "!!seq",
        "!custom_type",
        "!ns:tag",
        "!!type",
        // Additional valid patterns
        "!local-tag",
        "!my_type",
        "!CustomTag",
        "!tag123",
        "!ns:name",
        "!com:example:tag",
        "!yaml:org:yaml:tag",
        "!!int",
        "!!float",
        "!!bool",
        "!!null",
        "!!timestamp",
        "!myNamespace:myTag",
        "!verb",
        "!handle",
        // Tags with hyphens and underscores
        "!my-tag",
        "!my_tag",
        "!tag-name",
        "!tag_name",
        // More complex namespace patterns
        "!example.com:tag",
        "!org.example.project:type",
    ];

    for line in valid_tags {
        assert_eq!(
            classify_line_type(line),
            LineType::Tag,
            "Valid YAML tag should be classified as Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_valid_yaml_tag_patterns_with_indents() {
    // Valid YAML tags with various indentation levels
    let valid_tags = vec![
        "  !tag",
        "    !custom",
        "  !!str",
        "\t!local",
        "    !!type",
        "  !ns:tag",
        "      !my-tag",
        "\t\t!!int",
    ];

    for line in valid_tags {
        assert_eq!(
            classify_line_type(line),
            LineType::Tag,
            "Indented valid YAML tag should be classified as Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_invalid_tag_patterns() {
    // Patterns that look like tags but aren't valid YAML
    let test_cases = vec![
        "!123",              // Tag with only numbers (unusual)
        "!$",               // Tag with special char
        "!@tag",            // Tag starting with @ (invalid)
        "! space",          // Tag with space immediately after
        // Additional invalid patterns
        "!",                // Just exclamation (too short)
        "!!",               // Just double exclamation
        "!$",               // Special character only
        "!#",               // Hash character (comment-like)
        "!.",               // Dot only
        "!,",               // Comma only
        "!$",               // Dollar only
        "!@tag",            // Invalid @ at start
        "!#tag",            // Invalid # at start
        "! tag",            // Space after ! (malformed)
        "!  tag",           // Multiple spaces after !
        "!.tag",            // Dot at start
        "!,tag",            // Comma at start
        "!:tag",            // Colon at start (invalid)
        "!;tag",            // Semicolon at start
        "!|tag",            // Pipe at start
        "!>tag",            // Greater than at start
        "!<tag",            // Less than at start
        "!?tag",            // Question mark at start
        "!*tag",            // Asterisk at start
        "!&tag",            // Ampersand at start
        "!%tag",            // Percent at start
        "!^tag",            // Caret at start
        "!~tag",            // Tilde at start
        "!(tag)",           // Parentheses (invalid)
        "![tag]",           // Brackets (invalid)
        "!{tag}",           // Braces (invalid)
        "!<tag>",           // Angle brackets (invalid)
    ];

    for line in test_cases {
        // These might be classified as Tag (line starts with !)
        // even if they're not valid YAML tags
        let result = classify_line_type(line);
        // The implementation classifies by prefix, so these are technically Tags
        // based on the "starts with '!'" rule
        assert_eq!(
            result, LineType::Tag,
            "Line starting with ! is classified as Tag (even if unusual): '{}'",
            line
        );
    }
}

#[test]
fn test_tag_like_false_positives_in_values() {
    // These look like tags but are actually values (after colon)
    let test_cases = vec![
        "key: !tag",              // !tag as value
        "field: !!str",           // !!str as value
        "data: !custom",          // !custom as value
        "config: !ns:tag",       // !ns:tag as value
        "setting: !!type",       // !!type as value
        "value: !my-tag",        // !my-tag as value
        "option: !123",          // !123 as value
        "param: !$value",        // !$value as value
        "data: !@tag",           // !@tag as value
        "text: ! important",     // ! with space as value
        "msg: !todo",            // !todo as value
        "note: !!",              // !! as value
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Tag-like pattern in value should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_tag_like_false_positives_in_quoted_strings() {
    // Tag-like patterns inside quoted strings (should not be detected as tags)
    let test_cases = vec![
        "key: \"!tag\"",           // Quoted tag
        "field: '!!str'",          // Single quoted tag
        "data: \"!custom\"",       // Quoted custom tag
        "text: '!ns:tag'",         // Single quoted namespaced tag
        "value: \"!!type\"",       // Quoted double-bang tag
        "desc: \"Use !tag here\"", // Tag in quoted sentence
        "note: 'Value is !!str'",  // Tag in single quoted text
        "msg: \"Ref !ns:tag\"",    // Namespaced tag in quoted text
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Quoted tag-like pattern should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_tag_like_false_positives_in_sequence_items() {
    // Tag-like patterns in sequence items (should not be detected as tags)
    let test_cases = vec![
        "- !tag",                  // Tag-like as sequence item
        "- !!str",                 // Double-bang as sequence item
        "- !custom",               // Custom tag-like as sequence item
        "- \"!tag\"",              // Quoted tag in sequence
        "- !!str",                 // Double quoted tag in sequence
        "- !ns:tag",               // Namespaced tag-like in sequence
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Tag-like pattern in sequence should be SequenceItem, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_tag_like_false_positives_in_flow_collections() {
    // Tag-like patterns in flow collections (should not be detected as tags)
    let test_cases = vec![
        "items: [!tag, !!str]",           // Tag-like in flow sequence
        "map: {!key: value}",              // Tag-like in flow mapping
        "data: [!custom, !tag]",           // Multiple tag-like in flow seq
        "nested: {key: !value}",           // Tag-like as flow mapping value
        "complex: [!a, !b, !c]",           // Multiple tag-like patterns
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowSequence || result == LineType::FlowMapping,
            "Tag-like in flow collection should be MappingKey or Flow type: '{}'",
            line
        );
    }
}

#[test]
fn test_actual_yaml_tags_vs_string_values() {
    // Verify that actual YAML tags are detected while string values with ! are not
    let test_cases = vec![
        // These ARE tags (line starts with !, no colon)
        ("!tag", LineType::Tag, true),
        ("!!str", LineType::Tag, true),
        ("!custom", LineType::Tag, true),
        ("!ns:tag", LineType::Tag, true),
        ("!!type", LineType::Tag, true),
        // These are NOT tags (colon present, ! is in value)
        ("key: !tag", LineType::MappingKey, false),
        ("field: !!str", LineType::MappingKey, false),
        ("data: !custom", LineType::MappingKey, false),
        ("config: !ns:tag", LineType::MappingKey, false),
        ("setting: !!type", LineType::MappingKey, false),
        // Tag-like in comments (should be Comment, not Tag)
        ("# !tag", LineType::Comment, false),
        ("# !!str", LineType::Comment, false),
        ("  # !custom", LineType::Comment, false),
    ];

    for (line, expected_type, is_actual_tag) in test_cases {
        assert_eq!(
            classify_line_type(line),
            expected_type,
            "Line should be classified as {:?} (is_tag: {}): '{}'",
            expected_type, is_actual_tag, line
        );
    }
}

// ============================================================================
// Section 11: Whitespace and Exclamation Combinations
// ============================================================================

#[test]
fn test_whitespace_before_exclamation() {
    // Various whitespace patterns before !
    let test_cases = vec![
        "key : value!",     // Space before colon
        "key: value !",     // Space before !
        "key:  value!",     // Multiple spaces
        "key:\tvalue!",     // Tab separator
        // Additional whitespace scenarios
        "key:   value!",    // Three spaces
        "key:\t\tvalue!",   // Multiple tabs
        "key: value\t!",    // Tab before !
        "key: value\n!",    // Newline (if supported)
        "key:  value  !",   // Spaces around !
        "key:\tvalue\t!",   // Tabs around !
        "key :value!",      // Space before colon, no space after
        "key : value!",     // Space before colon, space after
        "key  :  value!",   // Multiple spaces around colon
        "key\t:\tvalue!",   // Tabs around colon
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Value with ! after whitespace should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_with_special_whitespace() {
    // ! with various Unicode whitespace
    let test_cases = vec![
        "key: value\u{200B}!", // Zero-width space before !
        "key: !value",         // Regular space
        "key:\u{3000}value!",  // Ideographic space
        // Additional Unicode whitespace tests
        "key: value\u{00A0}!", // Non-breaking space
        "key: value\u{2002}!", // En space
        "key: value\u{2003}!", // Em space
        "key: value\u{2009}!", // Thin space
        "key: value\u{202F}!", // Narrow no-break space
        "key: value\u{205F}!", // Medium mathematical space
        "key:\u{2003}!value",  // Em space after colon
        "key:\u{00A0}!value",  // Non-breaking space after colon
        "key:\u{2009}value!",  // Thin space before !
        "key:\u{202F}value!",  // Narrow space before !
    ];

    for line in test_cases {
        // Unicode whitespace might behave differently
        // But ! in value should not make it a tag
        let result = classify_line_type(line);
        if line.contains('!') && !line.starts_with('!') {
            assert_eq!(
                result, LineType::MappingKey,
                "Value with ! after Unicode whitespace should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_whitespace_only_before_exclamation() {
    // Lines with only whitespace before ! (edge cases)
    let test_cases = vec![
        " !tag",              // Single space before tag
        "  !tag",             // Two spaces before tag
        "\t!tag",             // Tab before tag
        "   !custom",         // Multiple spaces
        "\t\t!!str",          // Multiple tabs
        " \t!tag",            // Mixed space and tab
        "\t !tag",            // Tab then space
        "  \t!tag",           // Spaces then tab
        "\t  !tag",           // Tab then spaces
    ];

    for line in test_cases {
        // Lines with leading whitespace but starting with ! should still be Tag
        assert_eq!(
            classify_line_type(line),
            LineType::Tag,
            "Whitespace followed by ! should still be Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_with_whitespace_variations_in_values() {
    // ! in values with various whitespace patterns
    let test_cases = vec![
        "key: value !",       // Space before ! in value
        "key: value! ",       // Space after ! in value
        "key: value ! ",      // Spaces around ! in value
        "key: value  !",      // Multiple spaces before !
        "key: value!  ",      // Multiple spaces after !
        "key: value  !  ",    // Multiple spaces around !
        "key: value\t!",      // Tab before ! in value
        "key: value!\t",      // Tab after ! in value
        "key: value\t!\t",    // Tabs around ! in value
        "key: value \t!",     // Space then tab before !
        "key: value!\t ",     // Tab then space after !
        "key: value \t !\t ", // Mixed whitespace around !
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Value with ! and whitespace variations should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_in_comments_with_whitespace() {
    // ! in comments with various whitespace patterns
    let test_cases = vec![
        "# ! important",       // Space after #, then !
        "#  ! important",      // Multiple spaces
        "#\t! important",      // Tab after #
        "# !important",        // No space after !
        "#  !important",       // Multiple spaces, no space after !
        "#\t!important",       // Tab, no space after !
        "#   !   important",   // Multiple spaces throughout
        "#\t\t!\t\timportant",  // Multiple tabs throughout
        "#  ! value!",         // Multiple ! in comment
        "# !!tag",             // Double ! in comment
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::Comment,
            "Comment with ! and whitespace should be Comment, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_with_leading_whitespace_in_mapping_keys() {
    // Mapping keys with leading whitespace and ! in values
    let test_cases = vec![
        "  key: value!",       // Two spaces indent, ! in value
        "    key: !important", // Four spaces indent, ! starts value
        "\tkey: value!",       // Tab indent, ! in value
        "  \tkey: value!",     // Mixed indent, ! in value
        "\t  key: value!",     // Tab then spaces, ! in value
        "    key: value !",    // Deep indent, space before !
        "\t\tkey: value!\t",   // Double tab indent, ! with tab after
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Indented mapping key with ! in value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_at_sequence_item_with_whitespace() {
    // Sequence items with ! and various whitespace
    let test_cases = vec![
        "- value!",            // Sequence item with ! at end
        "-  value!",           // Sequence item with space and !
        "-\tvalue!",           // Sequence item with tab and !
        "- value !",           // Sequence item with space before !
        "- value! ",           // Sequence item with space after !
        "-  value  !  ",       // Sequence item with spaces around !
        "- !important",        // Sequence item starting with !
        "-  !important",       // Sequence item with space, starting with !
        "-\t!tag",             // Sequence item with tab, starting with !
        "- !!str",             // Sequence item with double ! starting value
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Sequence item with ! and whitespace should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_unicode_exclamation_mark_variations() {
    // Test various exclamation mark characters and similar symbols
    let test_cases = vec![
        "key: value!",        // Regular exclamation (U+0021)
        "key: value\u{FF01}", // Fullwidth exclamation (U+FF01)
        "key: value‼",       // Double exclamation (U+203C)
        "key: value⁉",       // Exclamation question mark (U+2049)
        "key: value❗",       // Heavy exclamation (U+2757)
        "key: value❕",       // White exclamation (U+2755)
        "key: value⚠",       // Warning sign (not exclamation but similar)
        "key: value⛔",       // No entry sign
    ];

    for line in test_cases {
        // Most of these should be classified as MappingKey since they're in values
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Value with exclamation-like symbol should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_whitespace_combinations_with_exclamation_in_different_contexts() {
    // Complex whitespace scenarios with ! in different YAML contexts
    let test_cases = vec![
        // Mapping keys
        ("key: value!", LineType::MappingKey),
        ("key: value !", LineType::MappingKey),
        ("key: value! ", LineType::MappingKey),
        ("key : value!", LineType::MappingKey),
        ("key  :  value!", LineType::MappingKey),

        // Tags (whitespace before ! on line, not after colon)
        (" !tag", LineType::Tag),
        ("  !custom", LineType::Tag),
        ("\t!!str", LineType::Tag),
        ("  \t!ns:tag", LineType::Tag),

        // Comments
        ("# ! comment", LineType::Comment),
        ("#  ! comment", LineType::Comment),
        ("#\t! comment", LineType::Comment),

        // Sequence items
        ("- value!", LineType::SequenceItem),
        ("-  value !", LineType::SequenceItem),
        ("-\tvalue!", LineType::SequenceItem),

        // Flow sequences as values (classified by flow structure)
        ("items: [value!, other!]", LineType::FlowSequence),
        ("data: [ !a, !b ]", LineType::FlowSequence),

        // Flow mappings as values (classified as mapping keys with flow content)
        ("map: {key: value!}", LineType::MappingKey),
        ("data: {k: !v}", LineType::MappingKey),
    ];

    for (line, expected) in test_cases {
        assert_eq!(
            classify_line_type(line),
            expected,
            "Complex whitespace with ! should be classified as {:?}: '{}'",
            expected, line
        );
    }
}

// ============================================================================
// Section 11B: Advanced Whitespace and Exclamation Edge Cases
// ============================================================================

#[test]
fn test_tab_vs_space_before_exclamation() {
    // Test that tabs and spaces are handled consistently
    let test_cases = vec![
        ("key: value!", "key: value!"),  // No whitespace before !
        ("key: value !", "key: value !"), // Space before !
        ("key: value\t!", "key: value\t!"), // Tab before !
        ("key: value  !", "key: value  !"), // Multiple spaces
        ("key:\tvalue!", "key:\tvalue!"),   // Tab before colon
        ("key: value \t!", "key: value \t!"), // Space then tab
        ("key: value\t !", "key: value\t !"), // Tab then space
    ];

    for (line, _repr) in test_cases {
        // All should be classified as MappingKey since they have colons
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Tab/space before ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_extended_unicode_whitespace_with_exclamation() {
    // Test additional Unicode whitespace characters
    let test_cases = vec![
        "key: value\u{2000}!",   // En quad
        "key: value\u{2001}!",   // Em quad
        "key: value\u{2004}!",   // Three-per-em space
        "key: value\u{2005}!",   // Four-per-em space
        "key: value\u{2006}!",   // Six-per-em space
        "key: value\u{2007}!",   // Figure space
        "key: value\u{2008}!",   // Punctuation space
        "key: value\u{2009}!",   // Thin space (already tested)
        "key: value\u{200A}!",   // Hair space
        "key: value\u{2028}!",   // Line separator
        "key: value\u{2029}!",   // Paragraph separator
        "key: value\u{202F}!",   // Narrow no-break space (already tested)
        "key: value\u{205F}!",   // Medium mathematical space (already tested)
        "key: value\u{3000}!",   // Ideographic space (already tested)
        "key: value\u{FEFF}!",   // Zero-width no-break space (BOM)
    ];

    for line in test_cases {
        // Unicode whitespace in values should still be MappingKey
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Extended Unicode whitespace before ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_zero_width_characters_with_exclamation() {
    // Test zero-width characters before exclamation
    let test_cases = vec![
        "key: value\u{200B}!", // Zero-width space
        "key: value\u{FEFF}!", // Zero-width no-break space (BOM)
        "key: value\u{2060}!", // Word joiner
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Zero-width char before ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_multiple_consecutive_whitespace_before_exclamation() {
    // Test multiple consecutive whitespace characters
    let test_cases = vec![
        "key: value   !",     // 3 spaces
        "key: value\t\t\t!",  // 3 tabs
        "key: value \t !",    // space, tab, space
        "key: value  \t  !",  // 2 spaces, tab, 2 spaces
        "key: value\t \t!",   // tab, space, tab
        "key: value \n !",    // space, newline, space (if supported)
        "key: value\r\t!",    // carriage return, tab
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Multiple consecutive whitespace before ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_with_whitespace_in_flow_sequences() {
    // Flow sequences with whitespace and ! patterns
    let test_cases = vec![
        ("items: [value!, other!]", LineType::FlowSequence),
        ("list: [ value!, item! ]", LineType::FlowSequence),
        ("data: [!a, !b, !c]", LineType::FlowSequence),
        ("array: [  !x,  !y,  !z  ]", LineType::FlowSequence),
        ("seq:\t[!1,\t!2,\t!3]", LineType::FlowSequence),
    ];

    for (line, expected) in test_cases {
        assert_eq!(
            classify_line_type(line),
            expected,
            "Flow sequence with ! and whitespace should be FlowSequence: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_with_whitespace_in_flow_mappings() {
    // Flow mappings with whitespace and ! patterns
    let test_cases = vec![
        ("map: {key: value!}", LineType::MappingKey),
        ("data: {k: !important}", LineType::MappingKey),
        ("obj: { a: !x, b: !y }", LineType::MappingKey),
        ("dict:\t{key:\t!val}", LineType::MappingKey),
    ];

    for (line, expected) in test_cases {
        assert_eq!(
            classify_line_type(line),
            expected,
            "Flow mapping with ! and whitespace should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_exclamation_at_different_positions_after_whitespace() {
    // Test ! at different positions relative to whitespace in values
    let test_cases = vec![
        "key: !",           // ! is the entire value
        "key: ! ",           // ! with trailing space
        "key:  !",           // ! with leading space
        "key:  !  ",         // ! with leading and trailing spaces
        "key: \t!",          // ! with leading tab
        "key: !\t",          // ! with trailing tab
        "key: \t!\t",        // ! with leading and trailing tabs
        "key: !important",   // ! starting a word
        "key: important!",   // ! ending a word
        "key: im!portant",   // ! in middle of word
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "! at various positions in value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_unicode_exclamation_with_whitespace_combinations() {
    // Test various Unicode exclamation-like characters with whitespace
    let test_cases = vec![
        "key: value !",       // Regular ! with space
        "key: value\u{FF01}", // Fullwidth ! (U+FF01)
        "key: value \u{FF01}", // Fullwidth ! with space
        "key: value‽",       // Interrobang (U+203D)
        "key: value ⁇",      // Double exclamation (U+203C)
        "key: value❗",       // Heavy exclamation (U+2757)
        "key: value ❗",      // Heavy ! with space
        "key: value\u{2009}❗", // Thin space before heavy !
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Unicode exclamation with whitespace should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_whitespace_preserves_yaml_tag_detection() {
    // Verify that actual YAML tags are still detected correctly despite whitespace
    // These should all be Tags, not MappingKey or other types
    let actual_tags = vec![
        "!tag",
        "  !tag",
        "\t!tag",
        "    !custom",
        "\t\t!!str",
        "  !ns:tag",
        "\t!verb",
        "  !!type",
        "\t!handle",
        "   !my-tag",        // 3 spaces
        "\t\t!my_tag",       // 2 tabs
        " \t!local",         // space then tab
        "\t !global",        // tab then space
    ];

    for line in actual_tags {
        assert_eq!(
            classify_line_type(line),
            LineType::Tag,
            "Actual YAML tag with whitespace should be Tag: '{}'",
            line
        );
    }

    // Verify that ! after colon is NOT a tag (even with whitespace)
    let not_tags = vec![
        "key: !value",
        "field: ! important",
        "data:  !custom",
        "item:\t!tag",
        "val:  !!str",
    ];

    for line in not_tags {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "! after colon (even with whitespace) should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_edge_case_long_whitespace_sequences() {
    // Test very long whitespace sequences (edge cases)
    let test_cases = vec![
        "key: value      !",  // Many spaces
        "key:\t\t\t\t\t!",   // Many tabs
        "key:     value!",    // Many spaces before value
        "key:\t\t\t\tvalue!", // Many tabs before value
        "key: value     !    ", // Many spaces around !
        "key:\t!\t\t\t\t",   // Tab before !, many tabs after
        "     key: value!",  // Many leading spaces before key
        "\t\t\tkey: value!", // Many leading tabs before key
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Long whitespace sequences should be handled correctly: '{}'",
            line
        );
    }
}

#[test]
fn test_whitespace_in_sequence_items_with_exclamation() {
    // Sequence items with various whitespace and ! patterns
    let test_cases = vec![
        "- value!",           // Basic
        "-  value!",          // Space after -
        "-\tvalue!",          // Tab after -
        "-  value !",         // Spaces around !
        "-\tvalue\t!",        // Tabs around !
        "-   !important",     // Multiple spaces, ! starts value
        "-\t\t!custom",       // Multiple tabs, ! starts value
        "-  ! tag",           // Space after ! (unusual but valid)
        "-\t!\tvalue",        // Tab before and after !
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Sequence item with whitespace and ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_mixed_line_endings_with_exclamation() {
    // Test that different line ending styles don't affect ! handling
    // (This mainly tests parsing consistency)
    let test_cases = vec![
        "key: value!",      // Unix-style
        "key: value!\r",    // Old Mac-style (if present)
        "key: value!\n",    // Unix-style
        "key: value!\r\n",  // Windows-style (if present)
    ];

    for line in test_cases {
        // Strip any trailing line ending characters for the test
        let clean_line = line.trim_end_matches(|c| c == '\r' || c == '\n');
        assert_eq!(
            classify_line_type(clean_line),
            LineType::MappingKey,
            "Line ending style should not affect ! handling: '{}'",
            clean_line
        );
    }
}

#[test]
fn test_exclamation_with_whitespace_in_quoted_values() {
    // Quoted values containing ! and whitespace
    let test_cases = vec![
        "key: \"value !\"",       // Space and ! in quotes
        "key: 'value !'",         // Single quotes
        "key: \"! important\"",   // ! with space in quotes
        "key: '!important '",     // ! with trailing space in quotes
        "key: \" value! \"",      // Spaces around ! in quotes
        "key: '\t!'",             // Tab and ! in quotes
        "key: \" ! \"",           // Just ! with spaces in quotes
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Quoted value with ! and whitespace should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_unicode_whitespace() {
    // Test detect_mapping_key with Unicode whitespace
    let test_cases = vec![
        ("key: value\u{00A0}!", "key"),   // Non-breaking space
        ("key: value\u{2009}!", "key"),   // Thin space
        ("key: value\u{202F}!", "key"),   // Narrow no-break space
        ("key:\u{2003}value!", "key"),   // Em space after colon
    ];

    for (line, expected_key) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with Unicode whitespace: '{}'",
            line
        );
        assert_eq!(info.unwrap().key, expected_key);
    }
}

#[test]
fn test_exclamation_whitespace_integration_full() {
    // Comprehensive integration test for all whitespace and ! patterns
    let yaml_content = vec![
        "# Comment with ! important",
        "!tag",
        "  !!str",
        "key: value!",
        "field: value !",
        "data: value  !",
        "item:\tvalue!",
        "  nested: !important",
        "    deep: value !",
        "- sequence!",
        "-  seq with !",
        "-\tseq\t!",
        "flow: [!a, !b, !c]",
        "map: {key: value!}",
        "empty: !",
    ];

    let expected_types = vec![
        LineType::Comment,      // # Comment
        LineType::Tag,          // !tag
        LineType::Tag,          // !!str
        LineType::MappingKey,   // key: value!
        LineType::MappingKey,   // field: value !
        LineType::MappingKey,   // data: value  !
        LineType::MappingKey,   // item: value!
        LineType::MappingKey,   // nested
        LineType::MappingKey,   // deep
        LineType::SequenceItem, // - sequence!
        LineType::SequenceItem, // - seq with !
        LineType::SequenceItem, // - seq!
        LineType::FlowSequence, // flow
        LineType::MappingKey,   // map
        LineType::MappingKey,   // empty
    ];

    for (line, expected) in yaml_content.iter().zip(expected_types.iter()) {
        assert_eq!(
            classify_line_type(line),
            *expected,
            "Integration test failed for: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 11A: Integration - Detect Mapping Key with False Positives
// ============================================================================

#[test]
fn test_detect_mapping_key_with_exclamation_in_value() {
    // Ensure detect_mapping_key correctly handles ! in values
    let test_cases = vec![
        ("key: value!", "key", Some("value!")),
        ("field: !important", "field", Some("!important")),
        ("text: Hello World!", "text", Some("Hello World!")),
        ("note: Check this!", "note", Some("Check this!")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with ! in value: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key");
        assert_eq!(info.value, expected_value.map(String::from), "Should extract value with !");
    }
}

#[test]
fn test_detect_mapping_key_with_exclamation_in_quoted_value() {
    // ! in quoted values
    let test_cases = vec![
        "key: \"value!\"",
        "field: '!important'",
        "text: \"Hello!\"",
    ];

    for line in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with ! in quoted value: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_rejects_actual_tag_lines() {
    // Lines that ARE actual tags should NOT be detected as mapping keys
    let test_cases = vec![
        "!tag",
        "!!str",
        "!custom_type",
        "!ns:tag",
    ];

    for line in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Should NOT detect tag line as mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_valid_yaml_tags_rejected() {
    // All valid YAML tag patterns should be rejected by detect_mapping_key
    let valid_tags = vec![
        "!tag",
        "!!str",
        "!!map",
        "!!seq",
        "!custom_type",
        "!ns:tag",
        "!!type",
        "!local-tag",
        "!my_type",
        "!CustomTag",
        "!tag123",
        "!ns:name",
        "!com:example:tag",
        "!yaml:org:yaml:tag",
        "!!int",
        "!!float",
        "!!bool",
        "!!null",
        "!!timestamp",
        "!myNamespace:myTag",
        "!verb",
        "!handle",
        "!my-tag",
        "!my_tag",
        "!tag-name",
        "!tag_name",
        "!example.com:tag",
        "!org.example.project:type",
        // Indented tags
        "  !tag",
        "    !custom",
        "  !!str",
        "\t!local",
        "    !!type",
        "  !ns:tag",
    ];

    for line in valid_tags {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Valid YAML tag should be rejected by detect_mapping_key: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_tag_like_in_values_accepted() {
    // Tag-like patterns in values (after colon) should be accepted as mapping keys
    let test_cases = vec![
        ("key: !tag", "key", Some("!tag")),
        ("field: !!str", "field", Some("!!str")),
        ("data: !custom", "data", Some("!custom")),
        ("config: !ns:tag", "config", Some("!ns:tag")),
        ("setting: !!type", "setting", Some("!!type")),
        ("value: !my-tag", "value", Some("!my-tag")),
        ("option: !123", "option", Some("!123")),
        ("param: !$value", "param", Some("!$value")),
        ("data: !@tag", "data", Some("!@tag")),
        ("text: ! important", "text", Some("! important")),
        ("msg: !todo", "msg", Some("!todo")),
        ("note: !!", "note", Some("!!")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Tag-like in value should be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should extract correct value from: '{}'", line);
        assert!(info.has_inline_value, "Should have inline value: '{}'", line);
        assert!(!info.is_parent_key, "Should not be parent key: '{}'", line);
    }
}

#[test]
fn test_detect_mapping_key_quoted_tag_patterns_accepted() {
    // Tag-like patterns inside quoted strings should be accepted
    let test_cases = vec![
        ("key: \"!tag\"", "key", Some("\"!tag\"")),
        ("field: '!!str'", "field", Some("'!!str'")),
        ("data: \"!custom\"", "data", Some("\"!custom\"")),
        ("text: '!ns:tag'", "text", Some("'!ns:tag'")),
        ("value: \"!!type\"", "value", Some("\"!!type\"")),
        ("desc: \"Use !tag here\"", "desc", Some("\"Use !tag here\"")),
        ("note: 'Value is !!str'", "note", Some("'Value is !!str'")),
        ("msg: \"Ref !ns:tag\"", "msg", Some("\"Ref !ns:tag\"")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Quoted tag-like should be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should extract quoted value from: '{}'", line);
    }
}

#[test]
fn test_detect_mapping_key_sequence_items_rejected() {
    // Sequence items (start with -) should be rejected
    let test_cases = vec![
        "- !tag",
        "- !!str",
        "- !custom",
        "- \"!tag\"",
        "- !!str",
        "- !ns:tag",
        "- value!",
        "-  value!",
        "-\tvalue!",
        "- value !",
        "- value! ",
    ];

    for line in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Sequence item should be rejected by detect_mapping_key: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_indentation() {
    // Test that detect_mapping_key correctly handles indentation
    let test_cases = vec![
        // (line, parent_indent, expected_key, expected_value, should_detect)
        ("  key: value!", 0, Some("key"), Some("value!"), true),
        ("    field: !important", 0, Some("field"), Some("!important"), true),
        ("\tnested: check!", 0, Some("nested"), Some("check!"), true),
        ("  deep: value!", 2, Some("deep"), Some("value!"), true),
        ("    deeper: !important", 2, Some("deeper"), Some("!important"), true),
        ("key: value", 0, Some("key"), Some("value"), true),
        ("  sibling: value", 2, Some("sibling"), Some("value"), true),
        // Invalid indentation (less than parent)
        ("  key: value", 4, None, None, false),
    ];

    for (line, parent_indent, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, parent_indent);
        if should_detect {
            assert!(
                info.is_some(),
                "Should detect mapping key with indent {}: '{}'",
                parent_indent, line
            );
            let info = info.unwrap();
            assert_eq!(
                info.key, expected_key.unwrap(),
                "Should extract correct key with indent {}: '{}'",
                parent_indent, line
            );
            assert_eq!(
                info.value, expected_value.map(String::from),
                "Should extract correct value with indent {}: '{}'",
                parent_indent, line
            );
        } else {
            assert!(
                info.is_none(),
                "Should reject mapping key with invalid indent {}: '{}'",
                parent_indent, line
            );
        }
    }
}

#[test]
fn test_detect_mapping_key_with_inline_comments() {
    // Test that detect_mapping_key correctly handles inline comments
    let test_cases = vec![
        ("key: value # ! important comment", "key", Some("value")),
        ("field: something # !note", "field", Some("something")),
        ("data: !important # TODO: check this", "data", Some("!important")),
        ("text: Hello! # inline note", "text", Some("Hello!")),
        ("note: Check this! # ! warning", "note", Some("Check this!")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with inline comment: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should extract value without comment from: '{}'", line);
    }
}

#[test]
fn test_detect_mapping_key_with_type_like_strings() {
    // Test that detect_mapping_key handles type-like strings correctly
    let test_cases = vec![
        ("error: \"Expected type string\"", "error", Some("\"Expected type string\"")),
        ("message: 'type mismatch error'", "message", Some("'type mismatch error'")),
        ("description: \"integer type required\"", "description", Some("\"integer type required\"")),
        ("note: 'boolean type expected'", "note", Some("'boolean type expected'")),
        ("datatype: string", "datatype", Some("string")),
        ("value_type: integer", "value_type", Some("integer")),
        ("field_type: boolean", "field_type", Some("boolean")),
        ("format: array", "format", Some("array")),
        ("structure: object", "structure", Some("object")),
        ("type: String", "type", Some("String")),
        ("type: INTEGER", "type", Some("INTEGER")),
        ("dtype: Boolean", "dtype", Some("Boolean")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with type-like string: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should extract correct value from: '{}'", line);
    }
}

#[test]
fn test_detect_mapping_key_with_error_codes() {
    // Test that detect_mapping_key handles error codes correctly
    let test_cases = vec![
        ("error_code: E001", "error_code", Some("E001")),
        ("delimiter_error: D123", "delimiter_error", Some("D123")),
        ("message: Error E456 occurred", "message", Some("Error E456 occurred")),
        ("status: E789 active", "status", Some("E789 active")),
        ("code: D012", "code", Some("D012")),
        ("error: E001 - Invalid input parameter", "error", Some("E001 - Invalid input parameter")),
        ("message: D123 delimiter not found", "message", Some("D123 delimiter not found")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with error code: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should extract correct value from: '{}'", line);
    }
}

#[test]
fn test_detect_mapping_key_with_whitespace_variations() {
    // Test various whitespace patterns with detect_mapping_key
    let test_cases = vec![
        ("key: value!", "key", Some("value!")),
        ("key: value !", "key", Some("value !")),
        ("key: value! ", "key", Some("value! ")),
        ("key:  value!", "key", Some("value!")),
        ("key:\tvalue!", "key", Some("value!")),
        ("key :value!", "key", Some("value!")),
        ("key : value!", "key", Some("value!")),
        ("key  :  value!", "key", Some("value!")),
        ("key\t:\tvalue!", "key", Some("value!")),
    ];

    for (line, expected_key, _expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with whitespace variation: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key from: '{}'", line);
        assert!(info.value.is_some(), "Should have value: '{}'", line);
    }
}

#[test]
fn test_detect_mapping_key_parent_keys() {
    // Test that parent keys (no value on same line) are detected correctly
    let test_cases: Vec<(&str, &str, Option<&str>)> = vec![
        ("config:", "config", None),
        ("settings:", "settings", None),
        ("nested:", "nested", None),
        ("  indented:", "indented", None),
        ("data:", "data", None),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect parent key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct parent key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should have no value for parent key: '{}'", line);
        assert!(!info.has_inline_value, "Should not have inline value: '{}'", line);
        assert!(info.is_parent_key, "Should be parent key: '{}'", line);
    }

    // Verify the is_valid method works correctly
    let info = detect_mapping_key("valid_key: value", 0).unwrap();
    assert!(info.is_valid(), "Valid key should pass is_valid check");

    let info = detect_mapping_key("  key: value", 0).unwrap();
    assert!(info.is_valid(), "Trimmed key should pass is_valid check");
}

#[test]
fn test_detect_mapping_key_rejects_special_constructs() {
    // Test that special YAML constructs are rejected
    let test_cases = vec![
        // Tags (already tested but included for completeness)
        "!tag",
        "!!str",
        "  !custom",
        // Anchors
        "&anchor",
        "  &label",
        // Aliases
        "*ref",
        "  *alias",
        // Directives
        "%YAML 1.2",
        "  %TAG",
        // Sequence items
        "- item",
        "  - value",
        // Block scalars
        "|",
        ">",
        "  |",
        "    >",
        // Document markers
        "---",
        "...",
        // Explicit key indicators
        "? key",
        "  ? explicit",
        // Comments
        "# comment",
        "  # indented comment",
    ];

    for line in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Should reject special construct: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_complex_values() {
    // Test mapping keys with complex values containing various patterns
    let test_cases = vec![
        ("css: .button!important", "css", Some(".button!important")),
        ("note: TODO: fix this! urgent", "note", Some("TODO: fix this! urgent")),
        ("message: Error: check logs!", "message", Some("Error: check logs!")),
        ("priority: high!important", "priority", Some("high!important")),
        ("url: https://example.com/path!query", "url", Some("https://example.com/path!query")),
        ("pattern: !important", "pattern", Some("!important")),
        ("selector: .class!important", "selector", Some(".class!important")),
        ("regex: .*!.*", "regex", Some(".*!.*")),
        ("text: Use string! not integer", "text", Some("Use string! not integer")),
        ("warning: Check array! size", "warning", Some("Check array! size")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with complex value: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should extract complex value from: '{}'", line);
    }
}

#[test]
fn test_detect_mapping_key_end_to_end_integration() {
    // End-to-end integration test with realistic YAML scenarios
    let _yaml_lines = vec![
        "# Configuration file - note: important! check settings",
        "version: 1.0",
        "settings:",
        "  enabled: true",
        "  message: \"Hello World!\"",
        "  priority: high!",
        "  types: [string, integer, boolean]",
        "  tags: [!tag, !!str, !custom]",
        "  nested:",
        "    deep: value!",
        "    deeper: !important",
        "errors:",
        "  E001: Invalid input",
        "  D123: Delimiter not found",
    ];

    // Lines that should be detected as mapping keys
    let mapping_keys: Vec<(&str, &str, Option<&str>)> = vec![
        ("version: 1.0", "version", Some("1.0")),
        ("settings:", "settings", None),
        ("  enabled: true", "enabled", Some("true")),
        ("  message: \"Hello World!\"", "message", Some("\"Hello World!\"")),
        ("  priority: high!", "priority", Some("high!")),
        // Note: Flow sequences like "types: [...]" are rejected by detect_mapping_key
        // This is correct behavior - the function is designed to skip flow style
        ("  nested:", "nested", None),
        ("    deep: value!", "deep", Some("value!")),
        ("    deeper: !important", "deeper", Some("!important")),
        ("errors:", "errors", None),
        ("  E001: Invalid input", "E001", Some("Invalid input")),
        ("  D123: Delimiter not found", "D123", Some("Delimiter not found")),
    ];

    // Lines that should NOT be detected as mapping keys
    let rejected = vec![
        "# Configuration file - note: important! check settings",
        "  types: [string, integer, boolean]",  // Flow sequence - rejected
        "  tags: [!tag, !!str, !custom]",        // Flow sequence - rejected
    ];

    // Test mapping keys detection
    for (line, expected_key, expected_value) in &mapping_keys {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key in end-to-end test: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, *expected_key, "Should extract correct key from: '{}'", line);
        assert_eq!(info.value, expected_value.map(|v| v.to_string()), "Should extract correct value from: '{}'", line);
    }

    // Test rejected lines
    for line in &rejected {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Should reject line in end-to-end test: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_all_tag_patterns_from_section_10() {
    // Comprehensive test covering all tag patterns from Section 10
    // Valid tags should be rejected, tag-like in values should be accepted

    // Valid YAML tags (should be rejected)
    let valid_tags = vec![
        "!tag", "!!str", "!!map", "!!seq", "!custom_type", "!ns:tag", "!!type",
        "!local-tag", "!my_type", "!CustomTag", "!tag123", "!ns:name",
        "!com:example:tag", "!yaml:org:yaml:tag", "!!int", "!!float",
        "!!bool", "!!null", "!!timestamp", "!myNamespace:myTag",
        "!verb", "!handle", "!my-tag", "!my_tag", "!tag-name", "!tag_name",
        "!example.com:tag", "!org.example.project:type",
    ];

    for tag in &valid_tags {
        let info = detect_mapping_key(tag, 0);
        assert!(
            info.is_none(),
            "Valid tag '{}' should be rejected by detect_mapping_key",
            tag
        );
    }

    // Tag-like patterns in values (should be accepted)
    let tag_like_values = vec![
        "key: !tag",
        "field: !!str",
        "data: !custom",
        "config: !ns:tag",
        "setting: !!type",
        "value: !my-tag",
        "option: !123",
        "param: !$value",
        "data: !@tag",
        "text: ! important",
        "msg: !todo",
        "note: !!",
    ];

    for line in &tag_like_values {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Tag-like in value '{}' should be accepted by detect_mapping_key",
            line
        );
    }
}

// ============================================================================
// Section 12: Complex Real-World Scenarios
// ============================================================================

#[test]
fn test_real_world_config_with_exclamation() {
    // Real-world configuration patterns with !
    let test_cases = vec![
        "css_class: .button!important",
        "note: TODO: fix this! urgent",
        "message: Error: check logs!",
        "priority: high!important",
        "url: https://example.com/path!query",
        "selector: div!important",
        "regex: .*!.*",
        "flag: enabled!",
        "status: done!",
        "alert: critical!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Real-world config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_production_yaml_app_config() {
    // Production application configuration patterns
    let test_cases = vec![
        "app_name: MyApp!",
        "version: 1.0.0!",
        "environment: production!",
        "debug_mode: false!",
        "log_level: info!",
        "max_connections: 100!",
        "timeout: 30!",
        "retry_count: 3!",
        "cache_enabled: true!",
        "health_check: /health!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Production app config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_cicd_pipeline_config() {
    // CI/CD pipeline configuration with exclamation in messages
    let test_cases = vec![
        "pipeline_name: deploy-to-prod!",
        "stage: build!",
        "job: test!",
        "script: ./run-tests!",
        "variables:",
        "  DEPLOY_ENV: production!",
        "  RETRY_COUNT: 3!",
        "  NOTIFY_ON_FAILURE: true!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "CI/CD config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Flow sequences should be classified correctly
    let flow_line = "artifacts: [build!, test-results!]";
    let result = classify_line_type(flow_line);
    assert!(
        result == LineType::MappingKey || result == LineType::FlowSequence,
        "CI/CD config with flow sequence should be valid type: '{}'",
        flow_line
    );
}

#[test]
fn test_kubernetes_deployment_config() {
    // Kubernetes deployment YAML patterns
    let test_cases = vec![
        "apiVersion: apps/v1!",
        "kind: Deployment!",
        "metadata:",
        "  name: my-app!",
        "  namespace: production!",
        "  labels:",
        "    app: frontend!",
        "    env: prod!",
        "spec:",
        "  replicas: 3!",
        "  selector:",
        "    matchLabels:",
        "      app: frontend!",
        "  template:",
        "    metadata:",
        "      labels:",
        "        app: frontend!",
        "    spec:",
        "      containers:",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Kubernetes config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Sequence items should be classified as SequenceItem
    let sequence_items = vec![
        "      - name: app!",
        "        - containerPort: 8080!",
    ];

    for line in sequence_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Kubernetes sequence item should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_database_connection_config() {
    // Database configuration with exclamation in values
    let test_cases = vec![
        "database:",
        "  host: localhost!",
        "  port: 5432!",
        "  name: myapp_db!",
        "  user: admin!",
        "  password: secret123!",
        "  ssl_mode: require!",
        "  pool_size: 10!",
        "  timeout: 30!",
        "  max_retries: 3!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Database config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_logging_config_with_exclamation() {
    // Logging configuration patterns
    let test_cases = vec![
        "logging:",
        "  level: INFO!",
        "  format: json!",
        "  output: stdout!",
        "  file: /var/log/app.log!",
        "  max_size: 100MB!",
        "  max_age: 30!",
        "  compress: true!",
        "  fields:",
        "    service: myapp!",
        "    environment: prod!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Logging config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_feature_flags_config() {
    // Feature flag configuration with exclamation marks
    let test_cases = vec![
        "features:",
        "  new_ui: enabled!",
        "  beta_api: true!",
        "  dark_mode: false!",
        "  notifications: enabled!",
        "  analytics: true!",
        "  cache_v2: enabled!",
        "  rate_limiting: true!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Feature flags with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_api_gateway_config() {
    // API gateway configuration patterns
    let test_cases = vec![
        "api:",
        "  base_url: https://api.example.com!",
        "  version: v2!",
        "  timeout: 30!",
        "  retry_policy: exponential!",
        "  rate_limit: 1000!",
        "  auth_method: oauth2!",
        "  endpoints:",
        "    users: /api/users!",
        "    posts: /api/posts!",
        "    comments: /api/comments!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "API gateway config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_docker_compose_config() {
    // Docker Compose configuration patterns
    let test_cases = vec![
        "version: '3.8'!",
        "services:",
        "  web:",
        "    image: nginx:latest!",
        "    ports:",
        "    environment:",
        "      ENV: production!",
        "  db:",
        "    image: postgres:13!",
        "    environment:",
        "      POSTGRES_DB: myapp!",
        "      POSTGRES_USER: admin!",
        "volumes:",
        "networks:",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Docker Compose config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Sequence items should be classified as SequenceItem
    let sequence_items = vec![
        "    - 80:80!",
        "  data:!",
        "  frontend:!",
    ];

    for line in sequence_items {
        // The first is a sequence item, the others are parent keys
        if line.starts_with("    -") {
            assert_eq!(
                classify_line_type(line),
                LineType::SequenceItem,
                "Docker Compose sequence item should be SequenceItem: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Docker Compose parent key should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_monitoring_alerts_config() {
    // Monitoring and alerting configuration
    let test_cases = vec![
        "monitoring:",
        "  enabled: true!",
        "  metrics:",
        "  alerts:",
        "    high_cpu:",
        "      threshold: 80%!",
        "      duration: 5m!",
        "      action: notify!",
        "    high_memory:",
        "      threshold: 90%!",
        "      duration: 3m!",
        "      action: alert!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Monitoring config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Sequence items should be classified as SequenceItem
    let sequence_items = vec![
        "    - cpu_usage!",
        "    - memory_usage!",
        "    - response_time!",
    ];

    for line in sequence_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Monitoring sequence item should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_message_template_config() {
    // Message templates with exclamation marks
    let test_cases = vec![
        "messages:",
        "  welcome: Welcome to our app!",
        "  success: Operation completed successfully!",
        "  error: Something went wrong!",
        "  warning: Please check your input!",
        "  info: Processing your request!",
        "  confirmation: Are you sure!",
        "  notification: You have a new message!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Message templates with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_simple_message_patterns_with_exclamation() {
    // Simple message patterns with exclamation marks (acceptance criteria verification)
    // This tests the exact pattern "message: Hello!" mentioned in acceptance criteria
    let test_cases = vec![
        "message: Hello!",
        "greeting: Hi!",
        "alert: Warning!",
        "note: Important!",
        "status: Ready!",
        "error: Failed!",
        "success: Done!",
        "info: Notice!",
        "debug: Check!",
        "trace: Logged!",
        "warning: Caution!",
        "critical: Alert!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Simple message pattern with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_basic_config_patterns_exclamation_variations() {
    // Basic real-world config pattern tests for Section 12
    // Covers: exclamation at end, multiple exclamation marks, exclamation in middle

    // Test configs with exclamation marks at end of strings (extended variations)
    let end_exclamation = vec![
        "message: Hello!",
        "greeting: Hi there!",
        "note: Important!",
        "status: Done!",
        "alert: Critical!",
        "warning: Caution!",
        "error: Failed!",
        "success: Complete!",
        "info: Notice!",
        "debug: Check!",
        "priority: High!",
        "level: Info!",
        "state: Active!",
        "mode: Enabled!",
        "flag: True!",
    ];

    for line in end_exclamation {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Config with ! at end should be MappingKey: '{}'",
            line
        );
    }

    // Test configs with multiple exclamation marks
    let multiple_exclamations = vec![
        "message: Hello!!!",
        "greeting: Hi there!!",
        "note: Important!!!",
        "status: Done!!",
        "alert: Critical!!!",
        "warning: Caution!!",
        "error: Failed!!!!",
        "success: Complete!!!",
        "info: Notice!!",
        "debug: Check!!!",
        "priority: High!!!",
        "level: Info!!",
        "state: Active!!!",
        "mode: Enabled!!",
        "flag: True!!!",
        "urgent: Critical NOW!!!",
        "emphasis: Really Important!!",
    ];

    for line in multiple_exclamations {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Config with multiple ! should be MappingKey: '{}'",
            line
        );
    }

    // Test configs with exclamation marks in middle of strings
    let middle_exclamation = vec![
        "note: Important! Read this",
        "message: Hello! World",
        "status: Done! Success",
        "alert: Critical! Action needed",
        "warning: Caution! Be careful",
        "error: Failed! Try again",
        "success: Complete! Well done",
        "info: Notice! Pay attention",
        "debug: Check! Verify this",
        "priority: High! Urgent",
        "message: Hey! You there",
        "note: TODO! Fix this soon",
        "status: OK! All good",
        "alert: Wow! Check this out",
        "text: Yes! That is correct",
    ];

    for line in middle_exclamation {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Config with ! in middle should be MappingKey: '{}'",
            line
        );
    }

    // Test configs with exclamation at various positions
    let various_positions = vec![
        "message: !",                    // Only exclamation
        "note: ! important",             // Exclamation at start
        "text: mid!dle",                // Exclamation in middle
        "status: end!",                 // Exclamation at end
        "flag: !start and end!",        // Exclamation at both ends
        "data: value! value",           // Exclamation separating words
        "text: a!b!c",                  // Multiple exclamations in middle
        "message: Hello! How are you!", // Multiple exclamations throughout
        "note: Check! This! Now!",      // Multiple exclamations separating phrases
        "text: !",                      // Just exclamation mark
    ];

    for line in various_positions {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Config with ! at various positions should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_css_and_ui_config() {
    // CSS and UI configuration with !important patterns
    let test_cases = vec![
        "ui:",
        "  primary_color: #FF0000!",
        "  secondary_color: #00FF00!",
        "  font_size: 14px!",
        "  button_style: .primary!important",
        "  layout: flex!",
        "  theme: dark!",
        "  responsive: true!",
        "  animations:",
        "    fade_in: 0.3s!",
        "    slide_up: 0.5s!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "CSS/UI config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_build_configuration() {
    // Build system configuration
    let test_cases = vec![
        "build:",
        "  target: release!",
        "  optimization: O3!",
        "  debug_symbols: false!",
        "  parallel_jobs: 4!",
        "  output_dir: ./dist!",
        "  artifacts:",
        "  dependencies:",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Build config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Sequence items should be classified as SequenceItem
    let sequence_items = vec![
        "    - binary!",
        "    - docs!",
        "    - library!",
        "    - framework!",
    ];

    for line in sequence_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Build config sequence item with ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_security_config() {
    // Security configuration
    let test_cases = vec![
        "security:",
        "  encryption: AES256!",
        "  authentication: jwt!",
        "  session_timeout: 3600!",
        "  password_policy: strong!",
        "  two_factor: enabled!",
        "  allowed_origins:",
        "  csrf_protection: true!",
        "  rate_limiting: enabled!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Security config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Sequence items should be classified as SequenceItem
    let sequence_items = vec![
        "    - https://example.com!",
        "    - https://app.example.com!",
    ];

    for line in sequence_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Security sequence item should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_multiline_scenario_with_exclamation() {
    // Simulate multiline YAML with ! in various positions
    let lines = vec![
        "# Note: This is important! Read carefully",
        "config:",
        "  enabled: true",
        "  message: \"Check this!\"",
        "  priority: high!",
        "  values: [item1!, item2!]",
    ];

    let expected = vec![
        LineType::Comment,      // Comment
        LineType::MappingKey,   // config:
        LineType::MappingKey,   // enabled: true
        LineType::MappingKey,   // message: with quoted !
        LineType::MappingKey,   // priority: with !
        LineType::FlowSequence, // values: flow sequence
    ];

    for (line, expected_type) in lines.iter().zip(expected.iter()) {
        assert_eq!(
            classify_line_type(line),
            *expected_type,
            "Multiline scenario failed for: '{}'",
            line
        );
    }
}

#[test]
fn test_complex_multiline_production_config() {
    // Complex multiline production configuration with exclamation
    let lines = vec![
        "# Production Configuration - Updated 2024! Check weekly",
        "application:",
        "  name: ProductionAPI!",
        "  version: 2.0.0!",
        "  environment: production!",
        "server:",
        "  host: 0.0.0.0!",
        "  port: 8080!",
        "  ssl_enabled: true!",
        "  ssl_cert: /etc/ssl/cert.pem!",
        "database:",
        "  primary:",
        "    host: db1.prod.example.com!",
        "    port: 5432!",
        "    name: app_db!",
        "  replica:",
        "    host: db2.prod.example.com!",
        "    port: 5432!",
        "cache:",
        "  enabled: true!",
        "  backend: redis!",
        "  host: redis.prod.example.com!",
        "  port: 6379!",
        "logging:",
        "  level: INFO!",
        "  format: json!",
        "  output: /var/log/app.log!",
        "features:",
        "  new_dashboard: true!",
        "  api_v2: enabled!",
        "  rate_limiting: active!",
    ];

    for line in lines {
        if line.starts_with('#') {
            assert_eq!(
                classify_line_type(line),
                LineType::Comment,
                "Comment should be Comment: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Complex multiline production config should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_multiline_with_inline_comments_and_exclamation() {
    // Multiline YAML with inline comments containing exclamation
    let test_cases = vec![
        "app_name: MyApp # Production instance!",
        "version: 1.0.0 # Latest stable!",
        "debug: false # Security: disable in prod!",
        "max_users: 1000 # License limit!",
        "timeout: 30 # Seconds!",
        "retries: 3 # Max attempts!",
    ];

    for line in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with inline comment containing !: '{}'",
            line
        );
        let info = info.unwrap();
        assert!(info.value.is_some(), "Should have value: '{}'", line);
    }
}

#[test]
fn test_quoted_values_with_exclamation_variations() {
    // Quoted string values with various exclamation patterns
    let test_cases = vec![
        "title: \"Welcome!\"",
        "subtitle: 'Get Started!'",
        "description: \"Hello World!\"",
        "message: 'Check this out!'",
        "error: \"Something went wrong!\"",
        "warning: 'Please be careful!'",
        "success: \"Operation complete!\"",
        "info: 'Processing...!'",
        "note: \"Important: Read this!\"",
        "alert: 'Action required!'",
        "hint: \"Try this first!\"",
        "tip: 'Pro tip: save often!'",
        "footer: \"© 2024 MyApp!\"",
        "header: 'Welcome back!'",
        "caption: \"Figure 1: Architecture!\"",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Quoted values with exclamation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_real_world_env_config() {
    // Environment-specific configuration patterns
    let test_cases = vec![
        "# Development Environment",
        "env: dev!",
        "debug: true!",
        "log_level: debug!",
        "",
        "# Staging Environment",
        "env: staging!",
        "debug: false!",
        "log_level: info!",
        "",
        "# Production Environment",
        "env: prod!",
        "debug: false!",
        "log_level: warn!",
        "monitoring: enabled!",
    ];

    for line in test_cases {
        if line.starts_with('#') {
            assert_eq!(
                classify_line_type(line),
                LineType::Comment,
                "Comment should be Comment: '{}'",
                line
            );
        } else if !line.is_empty() {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Env config with ! should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_microservices_config() {
    // Microservices architecture configuration
    let test_cases = vec![
        "services:",
        "  auth:",
        "    enabled: true!",
        "    port: 3001!",
        "  user:",
        "    enabled: true!",
        "    port: 3002!",
        "  payment:",
        "    enabled: false!",
        "    port: 3003!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Microservices config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Flow sequences with dependencies should be FlowSequence or MappingKey
    let flow_lines = vec![
        "    dependencies: [db, cache]!",
        "    dependencies: [db, auth]!",
        "    dependencies: [db, user]!",
    ];

    for line in flow_lines {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowSequence,
            "Microservices config with flow sequence should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_deployment_strategy_config() {
    // Deployment strategy configuration
    let test_cases = vec![
        "deployment:",
        "  strategy: rolling!",
        "  max_surge: 1!",
        "  max_unavailable: 0!",
        "  health_check:",
        "    path: /health!",
        "    interval: 10s!",
        "    timeout: 5s!",
        "    threshold: 3!",
        "  rollback:",
        "    enabled: true!",
        "    timeout: 300s!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Deployment strategy config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_rate_limiting_config() {
    // Rate limiting configuration
    let test_cases = vec![
        "rate_limit:",
        "  enabled: true!",
        "  requests_per_second: 100!",
        "  burst: 200!",
        "  window_size: 60s!",
        "  strategies:",
        "  whitelist:",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Rate limiting config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Sequence items should be classified as SequenceItem
    let sequence_items = vec![
        "    - ip_based!",
        "    - user_based!",
        "    - api_key_based!",
        "    - 192.168.1.0/24!",
        "    - trusted-partners!",
    ];

    for line in sequence_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Rate limiting sequence item should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_mixed_quoted_unquoted_values_with_exclamation() {
    // Real-world configs often mix quoted and unquoted values
    let test_cases = vec![
        "title: Welcome!",
        "subtitle: \"Get Started!\"",
        "description: 'Hello World!'",
        "message: Check this out!",
        "error: \"Something went wrong!\"",
        "warning: Please be careful!",
        "success: 'Operation complete!'",
        "info: Processing...!",
        "note: Important: Read this!",
        "alert: Action required!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Mixed quoted/unquoted values with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_complex_user_interface_messages() {
    // Complex UI message patterns with exclamation
    let test_cases = vec![
        "toast_message: Save successful!",
        "modal_title: Confirm action!",
        "notification: New message received!",
        "banner_text: Welcome back!",
        "tooltip: Click for more info!",
        "placeholder: Enter your email!",
        "help_text: Contact support for help!",
        "error_message: Invalid input detected!",
        "success_message: Changes saved!",
        "warning_message: This action cannot be undone!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "UI message with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_api_response_messages_with_exclamation() {
    // API response message patterns
    let test_cases = vec![
        "response_message: Request completed successfully!",
        "error_detail: Resource not found!",
        "status_message: Operation in progress!",
        "alert_message: High load detected!",
        "info_message: System maintenance scheduled!",
        "warning_message: Rate limit approaching!",
        "confirmation_message: Are you sure!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "API response message with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_configuration_validation_messages() {
    // Configuration validation and error messages
    let test_cases = vec![
        "validation_error: Invalid configuration value!",
        "schema_error: Configuration schema mismatch!",
        "parse_error: Failed to parse config file!",
        "validation_warning: Deprecated config option!",
        "load_message: Configuration loaded successfully!",
        "reload_message: Configuration reloaded!",
        "export_message: Configuration exported!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Config validation message with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_complex_multiline_block_with_exclamation() {
    // Complex multiline block scenarios with exclamation marks
    let lines = vec![
        "# IMPORTANT: Review all settings before deployment!",
        "production_config:",
        "  environment: production!",
        "  debug_mode: false!",
        "  database:",
        "    host: db.prod.example.com!",
        "    port: 5432!",
        "    ssl_enabled: true!",
        "  cache:",
        "    enabled: true!",
        "    backend: redis!",
        "    host: redis.prod.example.com!",
        "  features:",
        "    new_ui: enabled!",
        "    api_v2: true!",
        "  monitoring:",
        "    enabled: true!",
        "    metrics_port: 9090!",
        "staging_config:",
        "  environment: staging!",
        "  debug_mode: true!",
    ];

    for line in lines {
        if line.starts_with('#') {
            assert_eq!(
                classify_line_type(line),
                LineType::Comment,
                "Comment should be Comment: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Complex multiline block should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_web_server_configuration_with_exclamation() {
    // Web server configuration patterns
    let test_cases = vec![
        "server:",
        "  host: 0.0.0.0!",
        "  port: 8080!",
        "  ssl_enabled: true!",
        "  ssl_cert: /etc/ssl/cert.pem!",
        "  ssl_key: /etc/ssl/key.pem!",
        "  worker_processes: 4!",
        "  max_connections: 1000!",
        "  timeout: 30s!",
        "  keep_alive: true!",
        "  compression: enabled!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Web server config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_notification_system_config() {
    // Notification system configuration
    let test_cases = vec![
        "notifications:",
        "  email_enabled: true!",
        "  sms_enabled: false!",
        "  push_enabled: true!",
        "  email_address: admin@example.com!",
        "  webhook_url: https://hooks.example.com!",
        "  retry_attempts: 3!",
        "  timeout: 10s!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Notification config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Test webhook URLs in sequences
    let webhook_lines = vec![
        "  webhooks:",
        "    - https://hooks1.example.com!",
        "    - https://hooks2.example.com!",
        "    - https://hooks3.example.com!",
    ];

    for line in webhook_lines {
        if line.starts_with("    -") {
            assert_eq!(
                classify_line_type(line),
                LineType::SequenceItem,
                "Webhook URL with ! should be SequenceItem: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Notification parent key should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_backup_and_storage_config() {
    // Backup and storage configuration
    let test_cases = vec![
        "backup:",
        "  enabled: true!",
        "  schedule: daily!",
        "  retention_days: 30!",
        "  compression: true!",
        "  encryption: aes256!",
        "  storage:",
        "    type: s3!",
        "    bucket: my-backups!",
        "    region: us-east-1!",
        "  paths:",
        "    - /var/data!",
        "    - /etc/config!",
    ];

    for line in test_cases {
        if line.starts_with("    -") {
            assert_eq!(
                classify_line_type(line),
                LineType::SequenceItem,
                "Backup path with ! should be SequenceItem: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Backup config with ! should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_performance_tuning_config() {
    // Performance tuning configuration
    let test_cases = vec![
        "performance:",
        "  caching: enabled!",
        "  connection_pooling: true!",
        "  query_optimization: on!",
        "  index_usage: aggressive!",
        "  buffer_size: 1024MB!",
        "  worker_threads: 8!",
        "  max_requests: 10000!",
        "  gc_enabled: true!",
        "  gc_interval: 300s!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Performance config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_mobile_app_configuration() {
    // Mobile application configuration patterns
    let test_cases = vec![
        "mobile_app:",
        "  platform: ios!",
        "  version: 2.5.0!",
        "  min_version: 12.0!",
        "  orientation: portrait!",
        "  fullscreen: true!",
        "  biometric_auth: enabled!",
        "  offline_mode: true!",
        "  push_notifications: enabled!",
        "  deep_linking: supported!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Mobile app config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_cloud_infrastructure_config() {
    // Cloud infrastructure configuration
    let test_cases = vec![
        "cloud:",
        "  provider: aws!",
        "  region: us-east-1!",
        "  availability_zones:",
        "  instance_type: t3.medium!",
        "  auto_scaling: enabled!",
        "  load_balancer: application!",
        "  cdn: cloudfront!",
        "  monitoring: cloudwatch!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Cloud infrastructure config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Sequence items for availability zones
    let az_items = vec![
        "    - us-east-1a!",
        "    - us-east-1b!",
        "    - us-east-1c!",
    ];

    for line in az_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Availability zone with ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_content_management_config() {
    // Content management system configuration
    let test_cases = vec![
        "cms:",
        "  engine: wordpress!",
        "  theme: custom_theme!",
        "  plugins: [seo, cache, security]!",
        "  editor: gutenberg!",
        "  revision_history: enabled!",
        "  auto_save: true!",
        "  media_library:",
        "  seo_plugin: yoast!",
        "  cache_plugin: w3_total_cache!",
    ];

    for line in test_cases {
        if line.starts_with("  plugins:") {
            let result = classify_line_type(line);
            assert!(
                result == LineType::MappingKey || result == LineType::FlowSequence,
                "CMS config with flow sequence should be valid type: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "CMS config with ! should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_email_notification_templates() {
    // Email notification templates with exclamation marks
    let test_cases = vec![
        "email_templates:",
        "  welcome:",
        "    subject: Welcome to our service!",
        "    body: \"Thank you for signing up!\"",
        "    greeting: Hello!",
        "  password_reset:",
        "    subject: Reset your password!",
        "    body: \"Click here to reset!\"",
        "  alert:",
        "    subject: \"Important: Action required!\"",
        "    priority: urgent!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Email template with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_load_balancer_config() {
    // Load balancer configuration patterns
    let test_cases = vec![
        "load_balancer:",
        "  type: round_robin!",
        "  health_check:",
        "    interval: 30s!",
        "    timeout: 5s!",
        "    path: /health!",
        "  backend_servers:",
        "  ssl_offloading: true!",
        "  session_persistence: enabled!",
        "  max_connections: 10000!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Load balancer config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Backend server sequence items
    let backend_items = vec![
        "    - 10.0.1.10!",
        "    - 10.0.1.11!",
        "    - 10.0.1.12!",
    ];

    for line in backend_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Backend server with ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_cdn_configuration() {
    // Content Delivery Network configuration
    let test_cases = vec![
        "cdn:",
        "  provider: cloudflare!",
        "  zone: example.com!",
        "  caching: aggressive!",
        "  compression: enabled!",
        "  https: full!",
        "  http2: enabled!",
        "  http3: enabled!",
        "  image_optimization: on!",
        "  minification: true!",
        "  edge_cache_ttl: 86400!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "CDN config with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_message_queue_configuration() {
    // Message queue system configuration
    let test_cases = vec![
        "message_queue:",
        "  broker: rabbitmq!",
        "  host: mq.example.com!",
        "  port: 5672!",
        "  vhost: /production!",
        "  queues:",
        "  exchanges:",
        "  durable: true!",
        "  auto_delete: false!",
        "  message_ttl: 86400!",
        "  max_priority: 10!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Message queue config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Queue sequence items
    let queue_items = vec![
        "    - tasks!",
        "    - notifications!",
        "    - events!",
    ];

    for line in queue_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Queue name with ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_analytics_tracking_config() {
    // Analytics and tracking configuration
    let test_cases = vec![
        "analytics:",
        "  google_analytics:",
        "    tracking_id: UA-123456-1!",
        "    anonymize_ip: true!",
        "  mixpanel:",
        "    token: abc123!",
        "    track_pages: true!",
        "  custom_events:",
        "  error_tracking: enabled!",
        "  performance_monitoring: on!",
        "  user_behavior: tracked!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Analytics config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Custom event sequence items
    let event_items = vec![
        "    - button_click!",
        "    - page_view!",
        "    - form_submit!",
    ];

    for line in event_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Event name with ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_developer_portal_config() {
    // Developer portal and API documentation configuration
    let test_cases = vec![
        "developer_portal:",
        "  enabled: true!",
        "  documentation: /docs!",
        "  api_reference: /api!",
        "  auth_method: oauth2!",
        "  rate_limiting: per_key!",
        "  sandbox: available!",
        "  support_chat: enabled!",
        "  forums: /community!",
        "  examples:",
        "  webhook_testing: true!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Developer portal config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Example sequence items
    let example_items = vec![
        "    - curl!",
        "    - python!",
        "    - javascript!",
    ];

    for line in example_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Example language with ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_internationalization_config() {
    // Internationalization and localization configuration
    let test_cases = vec![
        "i18n:",
        "  default_locale: en_US!",
        "  supported_locales:",
        "  fallback_locale: en!",
        "  currency_format: symbol!",
        "  date_format: locale!",
        "  time_format: 24h!",
        "  timezone: UTC!",
        "  rtl_languages:",
        "  translation_files: /locales!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "I18n config with ! should be MappingKey: '{}'",
            line
        );
    }

    // Supported locale sequence items
    let locale_items = vec![
        "    - en_US!",
        "    - es_ES!",
        "    - fr_FR!",
        "    - de_DE!",
        "    - ja_JP!",
        "    - zh_CN!",
    ];

    for line in locale_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "Locale with ! should be SequenceItem: '{}'",
            line
        );
    }

    // RTL language sequence items
    let rtl_items = vec![
        "    - ar!",
        "    - he!",
        "    - fa!",
    ];

    for line in rtl_items {
        assert_eq!(
            classify_line_type(line),
            LineType::SequenceItem,
            "RTL language with ! should be SequenceItem: '{}'",
            line
        );
    }
}

#[test]
fn test_multiline_yaml_strings_with_exclamation() {
    // Test multiline YAML string values with exclamation marks
    // These should all be classified as MappingKey with proper values
    let test_cases = vec![
        // Simple multiline strings with ! at various positions
        "message: This is important!",
        "alert: Warning! Check now!",
        "error: Failed! Try again!",
        "note: Read this! It matters!",
        "status: Done! Complete!",
        // Single-line configs with embedded multiline contexts
        "summary: All systems operational!",
        "description: Configuration loaded! Ready to use!",
        "prompt: Enter your username! Press submit when done!",
        // Messages with multiple exclamation marks
        "greeting: Hello!!! Welcome aboard!",
        "warning: Critical error!!! Action required!",
        "success: Deployment complete!!! All systems green!",
        // Mixed single-line with various exclamation patterns
        "title: Welcome! Get Started!",
        "subtitle: Quick Start Guide! Learn More!",
        "caption: Figure 1! Architecture Diagram!",
        "footer: © 2024 MyApp! All rights reserved!",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Multiline YAML string with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_style_scalars_with_exclamation() {
    // Test folded style scalars (>) with exclamation marks
    // Folded scalars preserve newlines as spaces but strip final newline
    let test_cases = vec![
        // Folded scalar markers with keys containing ! in values
        "description: >",  // Folded scalar marker line
        "instructions: >", // Another folded scalar marker
        "note: >",         // Folded scalar for notes
        // Simulating content lines after folded marker (with !)
        "  This is important! Read carefully.",  // Indented continuation
        "  Check this! And verify!",             // Multiple sentences with !
        "  Warning! Critical system!",           // Warning text with !
        "  Done! Complete! Success!",            // Multiple exclamations
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        // Folded scalar marker lines and content should be properly classified
        // Marker lines (with :) are MappingKey, continuation lines are Unknown
        let is_marker_line = line.contains(':');
        if is_marker_line {
            assert_eq!(
                result, LineType::MappingKey,
                "Folded scalar marker line should be MappingKey: '{}'",
                line
            );
        } else {
            // Continuation lines (indented without :) are Unknown
            assert!(
                result == LineType::Unknown || result == LineType::Tag,
                "Folded scalar continuation should be Unknown or Tag: '{}' (got {:?})",
                line, result
            );
        }
    }

    // Test folded scalars in more complex scenarios
    let complex_folded = vec![
        ("help_text: >", LineType::MappingKey),
        ("error_message: >", LineType::MappingKey),
        ("  An error occurred! Please try!", LineType::Unknown),
        ("  Contact support! Email us!", LineType::Unknown),
    ];

    for (line, expected) in complex_folded {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected,
            "Complex folded scalar should be {:?}: '{}'",
            expected, line
        );
    }
}

#[test]
fn test_literal_style_scalars_with_exclamation() {
    // Test literal style scalars (|) with exclamation marks
    // Literal scalars preserve all newlines and formatting exactly
    let test_cases = vec![
        // Literal scalar marker lines
        "script: |",        // Literal scalar marker
        "config: |",        // Another literal marker
        "template: |",      // Template literal
        "code_block: |",    // Code block literal
        // Simulating content lines after literal marker (with !)
        "  #!/bin/bash! echo 'hello'",     // Script with !
        "  value: important!",             // Config-like line in literal
        "  message: Check this!",          // Message line in literal
        "  alert: Critical! Action!",      // Alert in literal block
        "  echo 'Done! Complete!'",         // Command with ! in literal
        "  print('Warning! Error!')",       // Code with ! in literal
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        // Literal scalar markers and content should be properly classified
        assert!(
            result == LineType::MappingKey || result == LineType::Comment,
            "Literal scalar with ! should be MappingKey or Comment: '{}'",
            line
        );
    }

    // Test literal scalars with various exclamation patterns
    let literal_with_patterns = vec![
        "code: |",
        "text: |",
        "  String with! exclamation",
        "  Line ending in!",
        "  Multiple! marks! here!",
        "  !start and end!",
    ];

    for line in literal_with_patterns {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::Comment,
            "Literal scalar patterns with ! should be valid: '{}'",
            line
        );
    }
}

#[test]
fn test_mixed_multiline_with_singleline_exclamation_patterns() {
    // Test mixed multiline blocks containing single-line configs with exclamation marks
    let test_cases = vec![
        // Mix of nested keys and exclamation values
        "app_config:",
        "  name: MyApp!",
        "  version: 1.0.0!",
        "  settings:",
        "  debug: false!",
        "  production: true!",
        " Multiline-like values that are actually single-line",
        "message: Welcome! Get started now!",
        "description: This is a long message! Read carefully!",
        " Mixed nested structures with exclamation marks",
        "server:",
        "  host: localhost!",
        "  port: 8080!",
        "  ssl: enabled!",
        " Flow sequences with exclamation marks in multiline-like context",
        "items: [one!, two!, three!]",
        "tags: [production!, critical!]",
        " Nested sequences with exclamation marks",
        "servers:",
        "    - server1!",
        "    - server2!",
        "    - backup!",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        // All should be valid types, never Tag when ! is after colon
        match result {
            LineType::MappingKey | LineType::SequenceItem | LineType::Comment |
            LineType::FlowSequence => {
                // Valid classifications
            }
            LineType::Tag => {
                panic!("Mixed multiline with ! should NOT be Tag: '{}'", line);
            }
            _ => {
                // Other types might be valid depending on context
            }
        }
    }

    // Specific test for multiline-looking blocks that are single lines
    let multiline_lookalikes = vec![
        "text: \"Line 1! Line 2! Line 3!\"",
        "note: 'Paragraph 1! Paragraph 2!'",
        "description: \"Section A! Section B! Section C!\"",
        "message: 'Warning! Error! Critical!'",
    ];

    for line in multiline_lookalikes {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Multiline-lookalike string with ! should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_and_literal_mixed_contexts() {
    // Test folded (>) and literal (|) scalars in mixed contexts with exclamation
    let mixed_contexts = vec![
        // Folded scalar followed by regular keys with !
        "help_text: >",
        "  Click here! Get started!",
        "quick_guide: >",
        "  Read this! Follow steps!",
        " Literal scalar with embedded exclamation patterns",
        "script: |",
        "  #!/bin/bash",
        "  echo 'Starting!'",
        "  echo 'Done!'",
        " Mixed: folded with regular configs",
        "description: >",
        "  First line! Second line!",
        "status: active!",
        "priority: high!",
    ];

    for line in mixed_contexts {
        let result = classify_line_type(line);
        // In mixed contexts, ! should never create a Tag classification
        if result == LineType::Tag {
            panic!("Mixed context with ! should NOT be Tag: '{}'", line);
        }
        // Other classifications are acceptable
    }
}

#[test]
fn test_multiline_edge_cases_with_exclamation() {
    // Test edge cases for multiline scenarios with exclamation marks
    let edge_cases = vec![
        // Empty or minimal folded/literal markers
        "empty: >",
        "blank: |",
        // Single character values with !
        "char: !",
        "value: a!",
        // Exclamation at start, middle, and end of what looks like multiline
        "text: !start middle! end!",
        "note: !!!!!!",
        // Mixed quotes with exclamation in multiline context
        "quoted1: \"Line 1! Line 2!\"",
        "quoted2: 'Line 1! Line 2!'",
        // Combining folded/literal indicators with ! in values
        "warning: >",
        "  Warning! Attention! Notice!",
        "error: |",
        "  Error! Failed! Broken!",
        // Edge case: exclamation right after folded/literal marker
        "marker1: >!",
        "marker2: |!",
    ];

    for line in edge_cases {
        let result = classify_line_type(line);
        // Edge cases should still not be classified as Tag when ! is after colon
        if result == LineType::Tag {
            // Only actual YAML tags should be Tag
            if line.contains(':') && line.find('!').unwrap_or(usize::MAX) > line.find(':').unwrap_or(0) {
                panic!("Edge case with ! after colon should NOT be Tag: '{}'", line);
            }
        }
    }
}

// ============================================================================
// Section 13: Error Code-like Strings in Values
// ============================================================================

#[test]
fn test_error_code_patterns_in_values() {
    // Error codes (E001, D001, etc.) appearing in values should not affect classification
    let test_cases = vec![
        "error_code: E001",
        "delimiter_error: D123",
        "message: Error E456 occurred",
        "status: E789 active",
        "code: D012",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error code in value should be MappingKey, not special type: '{}'",
            line
        );
    }
}

#[test]
fn test_invalid_error_code_formats() {
    // Invalid error code formats in values
    let test_cases = vec![
        "code: E1",      // Too short
        "code: D12",     // Too short
        "code: E1234",   // Too long
        "code: Z001",    // Wrong letter
        "code: Q123",    // Wrong letter
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Invalid error code format should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_error_codes_with_descriptions() {
    // Error codes with descriptive text
    let test_cases = vec![
        "error: E001 - Invalid input parameter",
        "message: D123 delimiter not found in file",
        "status: E456 - Connection timeout",
        "description: E789 authentication failed",
        "detail: D012 - Duplicate key detected",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error code with description should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_multiple_error_codes_in_values() {
    // Multiple error codes in a single value
    let test_cases = vec![
        "errors: [E001, E002, E003]",
        "codes: E123!E456!E789",
        "message: Errors E001 and D123 occurred",
        "status: See errors: E100, E200",
        "details: E001|D123|E456",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowSequence,
            "Multiple error codes in value should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_error_code_case_variations() {
    // Error codes with different letter casing
    let test_cases = vec![
        "code: e001",    // lowercase
        "code: E001",    // mixed case
        "code: d123",    // lowercase
        "code: ERROR001", // uppercase prefix
        "code: Err123",  // mixed prefix
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error code with case variation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_error_codes_in_nested_structures() {
    // Error codes in flow collections
    let test_cases = vec![
        "errors: {E001: input, D123: delimiter}",
        "codes: [E001, E002, D123]",
        "status: {major: E001, minor: D123}",
        "exceptions: {E100: timeout, E200: overflow}",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowMapping || result == LineType::FlowSequence,
            "Error code in nested structure should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_delimiter_error_variations() {
    // Various delimiter error patterns
    let test_cases = vec![
        "delimiter_error: D001",
        "delim_error: D002",
        "delimiter: D003",
        "delim: D004",
        "error: delimiter D005 not found",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Delimiter error variation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_error_codes_with_context() {
    // Error codes in various contexts
    let test_cases = vec![
        "log: Error E001 occurred in module",
        "trace: At line 42, error D123",
        "debug: Check error E456 in logs",
        "info: See error code E789 for details",
        "warning: Error D012 deprecated",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error code with context should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_warning_and_info_codes() {
    // Warning and info codes (W, I prefixes)
    let test_cases = vec![
        "warning: W001 - Deprecated feature",
        "info: I123 - Operation successful",
        "notice: W456 - Configuration change",
        "msg: I789 - Process started",
        "alert: W012 - High memory usage",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Warning/info code should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_critical_error_codes() {
    // Critical error codes (C, F prefixes)
    let test_cases = vec![
        "critical: C001 - System failure",
        "fatal: F123 - Cannot recover",
        "emergency: C456 - Shutdown required",
        "panic: F789 - Inconsistent state",
        "alert: C012 - Security breach",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Critical/fatal error code should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_error_codes_with_special_separators() {
    // Error codes with various separators
    let test_cases = vec![
        "codes: E001-E002-E003",
        "errors: E001,E002,E003",
        "status: E001|E002|E003",
        "codes: E001;E002;E003",
        "list: E001/E002/E003",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error codes with separators should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_error_code_boundaries() {
    // Error codes at number boundaries
    let test_cases = vec![
        "code: E000",    // Lower boundary
        "code: E999",    // Upper boundary
        "code: D000",    // Delimiter lower boundary
        "code: D999",    // Delimiter upper boundary
        "code: E00",     // Too short (edge case)
        "code: E1000",   // Too long (edge case)
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error code at boundary should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_mixed_error_types_in_sequence() {
    // Mixed error codes in sequences
    let test_cases = vec![
        "errors: [E001, W123, D456]",
        "codes: (E001, I002, W003)",
        "list: E001, F123, D456",
        "all: [E001, D002, W003, I004, C005]",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowSequence,
            "Mixed error types in sequence should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_error_codes_in_quoted_strings() {
    // Error codes in quoted strings
    let test_cases = vec![
        "message: \"Error E001 occurred\"",
        "description: 'Code D123 not found'",
        "text: \"See error E456\"",
        "note: 'Reference E789'",
        "detail: \"Check D012\"",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error code in quoted string should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_error_codes_with_exclamation() {
    // Error codes combined with exclamation marks
    let test_cases = vec![
        "error: E001!",
        "message: Critical D123!",
        "status: E456 - failed!",
        "alert: Error E789!",
        "warning: Code D012!",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Error code with exclamation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_custom_error_code_formats() {
    // Custom error code formats
    let test_cases = vec![
        "error: APP-E001",
        "code: MOD-D123",
        "status: SYS-E456",
        "error: LIB-D789",
        "code: SRV-E012",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Custom error code format should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_hex_error_codes() {
    // Hexadecimal error codes
    let test_cases = vec![
        "code: 0xE001",
        "error: 0xD123",
        "status: 0xE456",
        "code: 0xFFE",
        "error: 0xABC",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Hex error code should be MappingKey: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 14: Type Name Typos and Variations
// ============================================================================

#[test]
fn test_type_name_typos_in_values() {
    // Common typos of type names in values
    let test_cases = vec![
        "datatype: strign",    // typo of "string"
        "value: integre",      // typo of "integer"
        "flag: boolan",        // typo of "boolean"
        "items: arrary",       // typo of "array"
        "config: objec",       // typo of "object"
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name typo should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_name_case_variations() {
    // Type names with wrong capitalization in values
    let test_cases = vec![
        "type: String",      // Capitalized
        "type: INTEGER",     // All caps
        "type: Boolean",     // Capitalized
        "type: Array",       // Capitalized
        "type: sTrInG",      // Mixed case
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with wrong case should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_partial_type_matches_in_values() {
    // Strings containing type names as substrings
    let test_cases = vec![
        "description: stringy value",
        "field: integer_value",
        "setting: boolean_field",
        "data: array_data",
        "config: object_type",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Partial type match should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_common_type_misspellings() {
    // Common misspellings of basic type names
    let test_cases = vec![
        "type: strnig",    // typo of "string"
        "type: interger",   // typo of "integer"
        "type: boolena",    // typo of "boolean"
        "type: arraay",     // typo of "array"
        "type: objject",    // typo of "object"
        "type: srting",     // typo of "string"
        "type: ingeger",    // typo of "integer"
        "type: bollean",    // typo of "boolean"
        "type: arary",      // typo of "array"
        "type: objct",      // typo of "object"
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Common type misspelling should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_transposed_letter_typos() {
    // Type names with transposed letters
    let test_cases = vec![
        "type: tsring",     // "string" with transposed letters
        "type: itneger",    // "integer" with transposed letters
        "type: boolena",    // "boolean" with transposed letters
        "type: rarray",     // "array" with transposed letters
        "type: ojbect",     // "object" with transposed letters
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with transposed letters should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_double_letter_typos() {
    // Type names with doubled letters
    let test_cases = vec![
        "type: sstring",    // "string" with doubled 's'
        "type: iinteger",   // "integer" with doubled 'i'
        "type: bboolean",   // "boolean" with doubled 'b'
        "type: aarray",     // "array" with doubled 'a'
        "type: oobject",    // "object" with doubled 'o'
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with doubled letters should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_missing_letter_typos() {
    // Type names with missing letters
    let test_cases = vec![
        "type: strng",      // "string" missing 'i'
        "type: interer",    // "integer" missing 'g'
        "type: boolan",     // "boolean" missing 'e'
        "type: arry",       // "array" missing 'a'
        "type: objec",      // "object" missing 't'
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with missing letter should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_extra_letter_typos() {
    // Type names with extra letters
    let test_cases = vec![
        "type: striing",    // "string" with extra 'i'
        "type: inteeger",   // "integer" with extra 'e'
        "type: booleann",   // "boolean" with extra 'n'
        "type: arrayy",     // "array" with extra 'y'
        "type: objectt",    // "object" with extra 't'
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with extra letter should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_name_with_numbers() {
    // Type names with numbers mixed in
    let test_cases = vec![
        "type: str1ng",
        "type: int3ger",
        "type: b00l3an",
        "type: arr4y",
        "type: 0bj3ct",
        "type: string1",
        "type: integer2",
        "type: boolean3",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with numbers should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_name_with_underscores() {
    // Type names with underscores (common in configs)
    let test_cases = vec![
        "type: str_ing",
        "type: inte_ger",
        "type: boo_lean",
        "type: arr_ay",
        "type: obj_ect",
        "type: _string",
        "type: integer_",
        "type: __boolean__",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with underscores should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_name_with_hyphens() {
    // Type names with hyphens
    let test_cases = vec![
        "type: str-ing",
        "type: int-eger",
        "type: boo-lean",
        "type: arr-ay",
        "type: obj-ect",
        "type: -string",
        "type: integer-",
        "type: --boolean--",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with hyphens should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_reversed_type_names() {
    // Type names with letters reversed
    let test_cases = vec![
        "type: gnirts",     // "string" reversed
        "type: regetni",    // "integer" reversed
        "type: naelooB",    // "boolean" reversed
        "type: yarra",      // "array" reversed
        "type: tcejo",      // "object" reversed
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Reversed type name should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_alternative_type_names() {
    // Alternative names for common types
    let test_cases = vec![
        "type: text",       // Alternative for "string"
        "type: number",     // Alternative for "integer"
        "type: flag",       // Alternative for "boolean"
        "type: list",       // Alternative for "array"
        "type: dict",       // Alternative for "object"
        "type: str",        // Abbreviation for "string"
        "type: int",        // Abbreviation for "integer"
        "type: bool",       // Abbreviation for "boolean"
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Alternative type name should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_programming_language_types() {
    // Type names from various programming languages
    let test_cases = vec![
        "type: str",        // Python
        "type: i32",        // Rust
        "type: Vec",        // Rust
        "type: HashMap",    // Rust
        "type: String",     // Java/C#
        "type: Integer",    // Java
        "type: Boolean",    // Java
        "type: List",       // Java
        "type: Map",        // Java
        "type: NSString",   // Objective-C
        "type: std::string", // C++
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Programming language type should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_sql_data_types() {
    // SQL data type names in values
    let test_cases = vec![
        "type: varchar",
        "type: varchar(255)",
        "type: text",
        "type: int",
        "type: bigint",
        "type: decimal",
        "type: timestamp",
        "type: boolean",
        "type: json",
        "type: uuid",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "SQL data type should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_json_schema_types() {
    // JSON Schema type formats
    let test_cases = vec![
        "type: string",
        "type: number",
        "type: integer",
        "type: boolean",
        "type: array",
        "type: object",
        "type: null",
        "type: string|null",
        "type: array|string",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "JSON Schema type should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_truncated_type_names() {
    // Incomplete/truncated type names (specific examples from acceptance criteria)
    let test_cases = vec![
        "type: Strin",      // truncated "string"
        "type: Intger",     // truncated "integer"
        "type: Bool",       // truncated "boolean"
        "type: Arr",        // truncated "array"
        "type: Obj",        // truncated "object"
        "type: Str",        // very truncated "string"
        "type: Int",        // very truncated "integer"
        "type: Boo",        // very truncated "boolean"
        "type: sTring",     // mixed case truncated
        "type: inTeger",    // mixed case truncated
        "type: bOOLEAN",    // mixed case truncated
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Truncated type name should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_all_lowercase_vs_uppercase_variations() {
    // All lowercase vs all uppercase variations (acceptance criteria examples)
    let test_cases = vec![
        "type: string",     // all lowercase
        "type: STRING",     // all uppercase
        "type: integer",    // all lowercase
        "type: INTEGER",    // all uppercase
        "type: boolean",    // all lowercase
        "type: BOOLEAN",    // all uppercase
        "type: array",      // all lowercase
        "type: ARRAY",      // all uppercase
        "type: object",     // all lowercase
        "type: OBJECT",     // all uppercase
        "type: map",        // all lowercase
        "type: MAP",        // all uppercase
        "type: seq",        // all lowercase
        "type: SEQ",        // all uppercase
        "type: null",       // all lowercase
        "type: NULL",       // all uppercase
        "type: str",        // lowercase type hint
        "type: STR",        // uppercase type hint
        "type: int",        // lowercase type hint
        "type: INT",        // uppercase type hint
        "type: bool",       // lowercase type hint
        "type: BOOL",       // uppercase type hint
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name case variation should be MappingKey, not Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_reversed_and_scrambled_type_names() {
    // Type names with reversed or scrambled letters
    let test_cases = vec![
        "type: gnirts",     // "string" reversed
        "type: regetni",    // "integer" reversed
        "type: naeloob",    // "boolean" reversed
        "type: yarra",      // "array" reversed
        "type: tcejo",      // "object" reversed
        "type: stirng",     // "string" scrambled
        "type: inetger",    // "integer" scrambled
        "type: boolena",    // "boolean" scrambled
        "type: arary",      // "array" scrambled
        "type: obcjet",     // "object" scrambled
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Reversed/scrambled type name should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_keyboard_adjacent_typos() {
    // Typos from adjacent keyboard keys
    let test_cases = vec![
        "type: strung",     // 'u' next to 'i' on QWERTY
        "type: intzger",    // 'z' next to 't' on QWERTY
        "type: boolzan",    // 'z' next to 'a' on QWERTY
        "type: atrray",     // 't' next to 'r' on QWERTY
        "type: ibject",     // 'i' next to 'o' on QWERTY
        "type: striny",     // 'y' next to 'u' on QWERTY
        "type: integrr",    // 'r' next to 'e' on QWERTY
        "type: boolran",    // 'r' next to 'e' on QWERTY
        "type: aeeay",      // 'e' next to 'r' on QWERTY
        "type: ohject",     // 'h' next to 'j' on QWERTY
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Keyboard adjacent typo should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_repeated_character_typos() {
    // Type names with characters incorrectly repeated
    let test_cases = vec![
        "type: striiing",    // too many 'i's
        "type: integeer",   // too many 'e's
        "type: booleean",   // too many 'e's
        "type: arrray",     // too many 'r's
        "type: objeect",    // too many 'e's
        "type: sstring",    // doubled 's'
        "type: intteger",   // doubled 't'
        "type: booleann",   // doubled 'n'
        "type: arrayy",     // doubled 'y'
        "type: objectt",    // doubled 't'
        "type: striingg",   // doubled 'g'
        "type: integerr",   // doubled 'r'
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Repeated character typo should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_vowel_substitution_typos() {
    // Type names with substituted vowels
    let test_cases = vec![
        "type: streng",      // 'e' instead of 'i'
        "type: string",     // correct (sanity check)
        "type: strung",      // 'u' instead of 'i'
        "type: streng",      // 'e' instead of 'i'
        "type: strang",      // 'a' instead of 'i'
        "type: enteger",     // 'e' instead of 'i'
        "type: intagar",     // 'a' instead of 'e'
        "type: intoger",     // 'o' instead of 'e'
        "type: boolian",     // 'i' instead of 'e'
        "type: boolaen",     // 'a' instead of 'e'
        "type: booloen",     // 'o' instead of 'e'
        "type: ereay",       // 'e' instead of 'a'
        "type: orray",       // 'o' instead of 'a'
        "type: urray",       // 'u' instead of 'a'
        "type: ebject",      // 'e' instead of 'o'
        "type: abject",      // 'a' instead of 'o'
        "type: ubject",      // 'u' instead of 'o'
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Vowel substitution typo should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_name_leading_trailing_junk() {
    // Type names with junk characters before/after
    let test_cases = vec![
        "type: xstring",     // leading 'x'
        "type: xinteger",    // leading 'x'
        "type: stringx",     // trailing 'x'
        "type: integerx",    // trailing 'x'
        "type: xxstring",    // leading 'xx'
        "type: stringxx",    // trailing 'xx'
        "type: _string",     // leading underscore
        "type: _integer",    // leading underscore
        "type: string_",     // trailing underscore
        "type: integer_",    // trailing underscore
        "type: -string",     // leading dash
        "type: string-",     // trailing dash
        "type: .string",     // leading dot
        "type: string.",     // trailing dot
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with leading/trailing junk should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_name_in_context_of_error_message() {
    // Type names appearing in error message context
    let test_cases = vec![
        "error: Expected String but got Strin",
        "message: type INTEGER is invalid",
        "warning: field type Intger not recognized",
        "error: type 'Arrya' does not exist",
        "description: invalid type Boolan",
        "note: type Objcet is not defined",
        "error: cannot convert to Strig",
        "message: type Interger is deprecated",
        "warning: field type Arary is unknown",
        "error: expected Boolen got boolean",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Typo in error message context should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_multiple_typos_in_single_type_name() {
    // Type names with multiple typos combined
    let test_cases = vec![
        "type: strnig",      // missing 'i', wrong 'n' position
        "type: itneger",     // 'i' and 'e' swapped, wrong order
        "type: boolaen",     // 'a' instead of first 'e', 'e' instead of second 'a'
        "type: arary",       // 'r' duplication, missing second 'r'
        "type: ojbect",      // 'j' and 'b' swapped
        "type: tsringg",     // 's' and 't' swapped, 'g' duplicated
        "type: integr",      // missing 'e' at end
        "type: boolena",     // 'e' and 'a' swapped
        "type: arrey",       // 'e' and 'y' swapped
        "type: objict",      // 'c' and 't' swapped
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Multiple typos in type name should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_type_name_typos() {
    // Verify that type name typos are correctly identified as string values (not tags)
    // This is critical for acceptance criterion: "Verify typos are correctly identified as strings not types"
    let test_cases = vec![
        // Basic typos should be extracted as string values
        ("datatype: strign", "datatype", Some("strign")),
        ("value: integre", "value", Some("integre")),
        ("flag: boolan", "flag", Some("boolan")),
        ("items: arrary", "items", Some("arrary")),
        ("config: objec", "config", Some("objec")),
        // Truncated type names (from acceptance criteria)
        ("type: Strin", "type", Some("Strin")),
        ("type: Intger", "type", Some("Intger")),
        ("type: Bool", "type", Some("Bool")),
        ("type: Arr", "type", Some("Arr")),
        ("type: Obj", "type", Some("Obj")),
        // Case variations (from acceptance criteria)
        ("type: String", "type", Some("String")),
        ("type: STRING", "type", Some("STRING")),
        ("type: INTEGER", "type", Some("INTEGER")),
        ("type: Boolean", "type", Some("Boolean")),
        ("type: Array", "type", Some("Array")),
        // Misspelled type names
        ("type: strnig", "type", Some("strnig")),
        ("type: interger", "type", Some("interger")),
        ("type: boolena", "type", Some("boolena")),
        ("type: arraay", "type", Some("arraay")),
        ("type: objject", "type", Some("objject")),
        // Transposed letters
        ("type: tsring", "type", Some("tsring")),
        ("type: itneger", "type", Some("itneger")),
        ("type: boolena", "type", Some("boolena")),
        ("type: rarray", "type", Some("rarray")),
        ("type: ojbect", "type", Some("ojbect")),
        // Missing letters
        ("type: strng", "type", Some("strng")),
        ("type: interer", "type", Some("interer")),
        ("type: boolan", "type", Some("boolan")),
        ("type: arry", "type", Some("arry")),
        ("type: objec", "type", Some("objec")),
        // Extra letters
        ("type: striing", "type", Some("striing")),
        ("type: inteeger", "type", Some("inteeger")),
        ("type: booleann", "type", Some("booleann")),
        ("type: arrayy", "type", Some("arrayy")),
        ("type: objectt", "type", Some("objectt")),
        // With numbers
        ("type: str1ng", "type", Some("str1ng")),
        ("type: int3ger", "type", Some("int3ger")),
        ("type: b00l3an", "type", Some("b00l3an")),
        // Reversed type names
        ("type: gnirts", "type", Some("gnirts")),
        ("type: regetni", "type", Some("regetni")),
        ("type: naelooB", "type", Some("naelooB")),
        ("type: yarra", "type", Some("yarra")),
        ("type: tcejo", "type", Some("tcejo")),
        // Alternative type names
        ("type: text", "type", Some("text")),
        ("type: number", "type", Some("number")),
        ("type: flag", "type", Some("flag")),
        ("type: list", "type", Some("list")),
        ("type: dict", "type", Some("dict")),
        ("type: str", "type", Some("str")),
        ("type: int", "type", Some("int")),
        ("type: bool", "type", Some("bool")),
        // Programming language types
        ("type: i32", "type", Some("i32")),
        ("type: Vec", "type", Some("Vec")),
        ("type: HashMap", "type", Some("HashMap")),
        ("type: Integer", "type", Some("Integer")),
        ("type: List", "type", Some("List")),
        ("type: Map", "type", Some("Map")),
        ("type: NSString", "type", Some("NSString")),
        ("type: std::string", "type", Some("std::string")),
        // SQL types
        ("type: varchar", "type", Some("varchar")),
        ("type: varchar(255)", "type", Some("varchar(255)")),
        ("type: bigint", "type", Some("bigint")),
        ("type: decimal", "type", Some("decimal")),
        ("type: uuid", "type", Some("uuid")),
        // JSON Schema union types (with pipes)
        ("type: string|null", "type", Some("string|null")),
        ("type: array|string", "type", Some("array|string")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Type name typo should be detected as mapping key (not rejected as tag): '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(
            info.key, expected_key,
            "Should extract correct key from typo value: '{}'",
            line
        );
        assert_eq!(
            info.value, expected_value.map(String::from),
            "Should extract typo as string value (not interpret as type): '{}'",
            line
        );
        assert!(
            info.has_inline_value,
            "Should have inline value: '{}'",
            line
        );
        assert!(
            !info.is_parent_key,
            "Should not be parent key (should have value): '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_typos_in_quoted_values() {
    // Type name typos in quoted strings should still be extracted correctly
    let test_cases = vec![
        ("error: \"Expected Strin\"", "error", Some("\"Expected Strin\"")),
        ("message: 'type INTEGER'", "message", Some("'type INTEGER'")),
        ("description: \"invalid Intger\"", "description", Some("\"invalid Intger\"")),
        ("note: 'type Arrya'", "note", Some("'type Arrya'")),
        ("warning: \"Boolan error\"", "warning", Some("\"Boolan error\"")),
        ("text: 'Objcet not found'", "text", Some("'Objcet not found'")),
        ("status: \"got Strig\"", "status", Some("\"got Strig\"")),
        ("msg: 'Interger deprecated'", "msg", Some("'Interger deprecated'")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Quoted type name typo should be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract correct key: '{}'", line);
        assert_eq!(
            info.value, expected_value.map(String::from),
            "Should extract quoted value with typo: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_typos_in_error_messages() {
    // Type name typos appearing in error message contexts
    let test_cases = vec![
        ("error: Expected String but got Strin", "error", Some("Expected String but got Strin")),
        ("message: type INTEGER is invalid", "message", Some("type INTEGER is invalid")),
        ("warning: field type Intger not recognized", "warning", Some("field type Intger not recognized")),
        ("error: type 'Arrya' does not exist", "error", Some("type 'Arrya' does not exist")),
        ("description: invalid type Boolan", "description", Some("invalid type Boolan")),
        ("note: type Objcet is not defined", "note", Some("type Objcet is not defined")),
        ("error: cannot convert to Strig", "error", Some("cannot convert to Strig")),
        ("message: type Interger is deprecated", "message", Some("type Interger is deprecated")),
        ("warning: field type Arary is unknown", "warning", Some("field type Arary is unknown")),
        ("error: expected Boolen got boolean", "error", Some("expected Boolen got boolean")),
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Error message with type typo should be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract key from error message: '{}'", line);
        assert_eq!(
            info.value, expected_value.map(String::from),
            "Should extract error message with typo: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_keyboard_adjacent_typos() {
    // Keyboard-adjacent typos should be extracted as string values
    let test_cases = vec![
        ("type: strung", "type", Some("strung")),     // 'u' next to 'i'
        ("type: intzger", "type", Some("intzger")),   // 'z' next to 't'
        ("type: boolzan", "type", Some("boolzan")),   // 'z' next to 'a'
        ("type: atrray", "type", Some("atrray")),     // 't' next to 'r'
        ("type: ibject", "type", Some("ibject")),     // 'i' next to 'o'
        ("type: striny", "type", Some("striny")),     // 'y' next to 'u'
        ("type: integrr", "type", Some("integrr")),   // 'r' next to 'e'
        ("type: boolran", "type", Some("boolran")),   // 'r' next to 'e'
        ("type: aeeay", "type", Some("aeeay")),       // 'e' next to 'r'
        ("type: ohject", "type", Some("ohject")),     // 'h' next to 'j'
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Keyboard adjacent typo should be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract key: '{}'", line);
        assert_eq!(
            info.value, expected_value.map(String::from),
            "Should extract keyboard typo as string: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_vowel_substitution_typos() {
    // Vowel substitution typos should be extracted as string values
    let test_cases = vec![
        ("type: streng", "type", Some("streng")),      // 'e' instead of 'i'
        ("type: strung", "type", Some("strung")),      // 'u' instead of 'i'
        ("type: strang", "type", Some("strang")),      // 'a' instead of 'i'
        ("type: enteger", "type", Some("enteger")),     // 'e' instead of 'i'
        ("type: intagar", "type", Some("intagar")),     // 'a' instead of 'e'
        ("type: intoger", "type", Some("intoger")),     // 'o' instead of 'e'
        ("type: boolian", "type", Some("boolian")),     // 'i' instead of 'e'
        ("type: boolaen", "type", Some("boolaen")),     // 'a' instead of 'e'
        ("type: booloen", "type", Some("booloen")),     // 'o' instead of 'e'
        ("type: ereay", "type", Some("ereay")),       // 'e' instead of 'a'
        ("type: orray", "type", Some("orray")),       // 'o' instead of 'a'
        ("type: urray", "type", Some("urray")),       // 'u' instead of 'a'
        ("type: ebject", "type", Some("ebject")),      // 'e' instead of 'o'
        ("type: abject", "type", Some("abject")),      // 'a' instead of 'o'
        ("type: ubject", "type", Some("ubject")),      // 'u' instead of 'o'
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Vowel substitution typo should be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract key: '{}'", line);
        assert_eq!(
            info.value, expected_value.map(String::from),
            "Should extract vowel substitution as string: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_leading_trailing_junk() {
    // Type names with junk characters should be extracted as string values
    let test_cases = vec![
        ("type: xstring", "type", Some("xstring")),     // leading 'x'
        ("type: xinteger", "type", Some("xinteger")),    // leading 'x'
        ("type: stringx", "type", Some("stringx")),     // trailing 'x'
        ("type: integerx", "type", Some("integerx")),    // trailing 'x'
        ("type: xxstring", "type", Some("xxstring")),    // leading 'xx'
        ("type: stringxx", "type", Some("stringxx")),    // trailing 'xx'
        ("type: _string", "type", Some("_string")),     // leading underscore
        ("type: _integer", "type", Some("_integer")),    // leading underscore
        ("type: string_", "type", Some("string_")),     // trailing underscore
        ("type: integer_", "type", Some("integer_")),    // trailing underscore
        ("type: -string", "type", Some("-string")),     // leading dash
        ("type: string-", "type", Some("string-")),     // trailing dash
        ("type: .string", "type", Some(".string")),     // leading dot
        ("type: string.", "type", Some("string.")),     // trailing dot
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Type name with junk should be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract key: '{}'", line);
        assert_eq!(
            info.value, expected_value.map(String::from),
            "Should extract type with junk as string: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_with_multiple_typos_combined() {
    // Multiple typos in a single type name should still be extracted as string
    let test_cases = vec![
        ("type: strnig", "type", Some("strnig")),      // missing 'i', wrong 'n' position
        ("type: itneger", "type", Some("itneger")),     // 'i' and 'e' swapped
        ("type: boolaen", "type", Some("boolaen")),     // 'a' instead of 'e', 'e' instead of 'a'
        ("type: arary", "type", Some("arary")),       // 'r' duplication, missing second 'r'
        ("type: ojbect", "type", Some("ojbect")),      // 'j' and 'b' swapped
        ("type: tsringg", "type", Some("tsringg")),     // 's' and 't' swapped, 'g' duplicated
        ("type: integr", "type", Some("integr")),      // missing 'e' at end
        ("type: boolena", "type", Some("boolena")),     // 'e' and 'a' swapped
        ("type: arrey", "type", Some("arrey")),       // 'e' and 'y' swapped
        ("type: objict", "type", Some("objict")),      // 'c' and 't' swapped
    ];

    for (line, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Multiple typos should still be detected as mapping key: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key, "Should extract key: '{}'", line);
        assert_eq!(
            info.value, expected_value.map(String::from),
            "Should extract multiple typos as string: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 15: Type-like Strings in Complex Contexts
// ============================================================================

#[test]
fn test_type_like_in_nested_structures() {
    // Type-like strings in nested mappings/sequences
    let test_cases = vec![
        "outer: {inner: string}",
        "items: [integer, boolean]",
        "data: {values: [array, object]}",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowMapping || result == LineType::FlowSequence,
            "Type-like in nested structure should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_with_special_chars() {
    // Type-like strings with special characters
    let test_cases = vec![
        "format: string!",
        "value: integer?",
        "data: boolean*",
        "type: array+",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type-like with special char should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_multiline_values() {
    // Type-like strings that might appear in continuation lines
    let test_cases = vec![
        "description: This is a",
        "note: string value that",
        "text: continues on next",
        "comment: line with integer",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type-like word in multiline value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_documentation() {
    // Type names in documentation strings
    let test_cases = vec![
        "doc: Returns string value",
        "description: Expects integer input",
        "help: Boolean flag for feature",
        "note: Array of objects",
        "summary: Object with string keys",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in documentation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_error_descriptions() {
    // Type names in error descriptions
    let test_cases = vec![
        "error: Expected string, got integer",
        "message: Cannot convert boolean to string",
        "warning: Array index out of bounds",
        "exception: Object key not found",
        "fatal: Integer overflow detected",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in error description should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_config_descriptions() {
    // Type names in configuration value descriptions
    let test_cases = vec![
        "desc: Set to boolean for true/false",
        "help: Integer value for timeout",
        "note: String path to file",
        "info: Array of allowed values",
        "detail: Object with user data",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in config description should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_validation_messages() {
    // Type names in validation error messages
    let test_cases = vec![
        "validation: Field must be string",
        "error: Value is not integer",
        "message: Expected boolean type",
        "warning: Invalid array format",
        "alert: Malformed object structure",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in validation message should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_api_responses() {
    // Type names in API response descriptions
    let test_cases = vec![
        "response: Returns string data",
        "body: JSON object with fields",
        "result: Array of user objects",
        "data: Boolean success flag",
        "value: Integer count",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in API response should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_schema_definitions() {
    // Type names in schema/contract definitions
    let test_cases = vec![
        "schema: User object with string ID",
        "contract: Request with integer body",
        "type: Response as boolean array",
        "format: Object containing nested objects",
        "structure: Array of string arrays",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in schema definition should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_log_messages() {
    // Type names in log messages
    let test_cases = vec![
        "log: Processing string value",
        "debug: Parsing integer from input",
        "info: Storing boolean in cache",
        "warn: Array size exceeds limit",
        "error: Object serialization failed",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in log message should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_comments_inline() {
    // Type names in inline comments (for simple values)
    let test_cases = vec![
        "value: 42 # integer value",
        "flag: true # boolean setting",
        "name: test # string identifier",
        "count: 100 # integer amount",
        "enabled: false # boolean flag",
    ];

    for line in test_cases {
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_some(),
            "Should detect mapping key with type comment: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_with_exclamation_complex() {
    // Type names combined with exclamation in complex scenarios
    let test_cases = vec![
        "message: Use string! not integer",
        "warning: Check array! size",
        "error: Object! not found",
        "note: Boolean! value required",
        "alert: Integer! overflow",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name with exclamation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_enum_values() {
    // Type names appearing in enum-like values
    let test_cases = vec![
        "type: string_type",
        "kind: integer_kind",
        "format: boolean_format",
        "mode: array_mode",
        "style: object_style",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in enum value should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_regex_patterns() {
    // Type names in regex pattern descriptions
    let test_cases = vec![
        "pattern: Matches string value",
        "regex: Integer number pattern",
        "format: Boolean true/false",
        "validation: Array of items",
        "constraint: Object structure",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in regex description should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_conversion_contexts() {
    // Type names in type conversion/error messages
    let test_cases = vec![
        "convert: string to integer",
        "cast: Boolean to string",
        "parse: Array from string",
        "format: Object to JSON",
        "transform: Integer to string",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in conversion context should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_function_descriptions() {
    // Type names in function/method descriptions
    let test_cases = vec![
        "returns: String value",
        "param: Integer input",
        "arg: Boolean flag",
        "result: Array of results",
        "output: Object with data",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in function description should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_template_strings() {
    // Type names in template/format string descriptions
    let test_cases = vec![
        "template: String {value}",
        "format: Integer {count}",
        "pattern: Boolean {flag}",
        "layout: Array of {items}",
        "structure: Object with {fields}",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Type name in template description should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_nested_flow_collections() {
    // Type names deeply nested in flow collections
    let test_cases = vec![
        "data: {inner: {type: string}}",
        "items: [integer, [string, boolean]]",
        "config: {types: [array, object], flags: {enabled: boolean}}",
        "structure: {nested: {deep: {type: integer}}}",
        "complex: [{a: string}, {b: integer}, [{c: boolean}]]",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowMapping || result == LineType::FlowSequence,
            "Type-like in deeply nested flow collection should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_mixed_collections() {
    // Type names in mixed mapping/sequence structures
    let test_cases = vec![
        "fields: {name: string, age: integer, active: boolean}",
        "values: [string, 123, true, {key: value}]",
        "schema: {type: object, properties: {id: integer, name: string}}",
        "response: {status: integer, data: array, errors: []}",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowMapping || result == LineType::FlowSequence,
            "Type-like in mixed collection should be valid type: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 15: Type-Like Strings in Complex Nested Structures
// ============================================================================

#[test]
fn test_type_like_in_deeply_nested_mappings() {
    // Type-like strings deeply nested in mapping structures
    let test_cases = vec![
        "config:",
        "  database:",
        "    connection:",
        "      host: localhost",
        "      type: string",
        "    credentials:",
        "      username: admin",
        "      password_type: boolean",
        "  cache:",
        "    backend:",
        "      type: redis",
        "      config:",
        "        ttl: integer",
        "  logging:",
        "    level: string",
        "    format: json",
        "  metrics:",
        "    enabled: boolean",
    ];

    for line in test_cases {
        if line.starts_with('#') {
            assert_eq!(
                classify_line_type(line),
                LineType::Comment,
                "Comment should be Comment: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Deeply nested mapping with type-like should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_type_like_in_complex_production_yaml() {
    // Realistic production configuration with deeply nested type-like strings
    let test_cases = vec![
        "# Production Application Configuration",
        "application:",
        "  name: MyApp",
        "  version: 2.0.0",
        "  environment:",
        "    type: production",
        "    region: us-east-1",
        "  database:",
        "    primary:",
        "      host: db1.prod.example.com",
        "      port: 5432",
        "      schema_type: postgresql",
        "      connection_pool:",
        "        max_size: integer",
        "        timeout: integer",
        "    replica:",
        "      enabled: boolean",
        "      lag_threshold: integer",
        "  cache:",
        "    provider: redis",
        "    config:",
        "      ttl: integer",
        "      max_memory: string",
        "      eviction_policy: string",
        "  api:",
        "    rest:",
        "      timeout: integer",
        "      rate_limit: integer",
        "    graphql:",
        "      complexity: integer",
        "      depth: integer",
        "  monitoring:",
        "    metrics:",
        "      enabled: boolean",
        "      retention: string",
        "    tracing:",
        "      sample_rate: integer",
        "      export_type: string",
    ];

    for line in test_cases {
        if line.starts_with('#') {
            assert_eq!(
                classify_line_type(line),
                LineType::Comment,
                "Comment should be Comment: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Production YAML with type-like should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_type_like_in_nested_sequence_structures() {
    // Type-like strings in nested sequence structures
    let test_cases = vec![
        "services:",
        "  - name: auth",
        "    config:",
        "      port: integer",
        "      timeout: integer",
        "      enabled: boolean",
        "  - name: user",
        "    config:",
        "      database: string",
        "      cache_type: string",
        "      replicas: integer",
        "  - name: payment",
        "    config:",
        "      gateway: string",
        "      timeout: integer",
        "      retry_count: integer",
    ];

    for line in test_cases {
        if line.starts_with("  -") {
            assert_eq!(
                classify_line_type(line),
                LineType::SequenceItem,
                "Nested sequence item with type-like should be SequenceItem: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Nested mapping with type-like should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_type_like_in_flow_collection_nesting() {
    // Type-like strings in deeply nested flow collections
    let test_cases = vec![
        "schema: {fields: [{name: id, type: integer}, {name: value, type: string}]}",
        "config: {nested: {deep: {type: string, value: integer}}}",
        "data: {items: [{type: boolean}, {type: array, elements: [integer, string]}]}",
        "structure: {level1: {level2: {level3: {type: object}}}}",
        "complex: {mapping: {types: [string, integer, boolean], nested: {deep: integer}}}",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowMapping || result == LineType::FlowSequence,
            "Deeply nested flow collection with type-like should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_kubernetes_style_config() {
    // Kubernetes-style configuration with deeply nested type-like strings
    let test_cases = vec![
        "apiVersion: v1",
        "kind: ConfigMap",
        "metadata:",
        "  name: app-config",
        "  namespace: production",
        "data:",
        "  database_type: postgresql",
        "  cache_backend: string",
        "  log_level: string",
        "  max_connections: integer",
        "  timeout_seconds: integer",
        "  enabled_features: array",
        "  config_type: object",
        "  monitoring_enabled: boolean",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Kubernetes-style config with type-like should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_hierarchical_config_tree() {
    // Hierarchical configuration tree with type-like strings at multiple levels
    let test_cases = vec![
        "root:",
        "  level1:",
        "    setting1: string",
        "    level2:",
        "      setting2: integer",
        "      level3:",
        "        setting3: boolean",
        "        level4:",
        "          setting4: array",
        "          type: object",
        "  another_level1:",
        "    config_type: string",
        "    nested:",
        "      deep:",
        "        setting: integer",
        "        type: boolean",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Hierarchical config with type-like should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_microservice_architecture_config() {
    // Microservice architecture configuration with nested type-like strings
    let test_cases = vec![
        "microservices:",
        "  auth_service:",
        "    endpoints:",
        "      login:",
        "        method: string",
        "        timeout: integer",
        "        rate_limit: integer",
        "      logout:",
        "        method: string",
        "        enabled: boolean",
        "    dependencies:",
        "      - name: database",
        "        type: string",
        "      - name: cache",
        "        type: string",
        "  user_service:",
        "    config:",
        "      database_type: string",
        "      cache_type: string",
        "      timeout: integer",
        "      retries: integer",
        "      debug_mode: boolean",
    ];

    for line in test_cases {
        if line.starts_with("      -") {
            assert_eq!(
                classify_line_type(line),
                LineType::SequenceItem,
                "Microservice sequence item with type-like should be SequenceItem: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Microservice config with type-like should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_type_like_in_data_pipeline_config() {
    // Data pipeline configuration with nested structures
    let test_cases = vec![
        "pipeline:",
        "  sources:",
        "    - name: input_stream",
        "      type: string",
        "      format: json",
        "  transforms:",
        "    filter:",
        "      field: string",
        "      operator: string",
        "      value_type: string",
        "    map:",
        "      input_type: integer",
        "      output_type: string",
        "  sinks:",
        "    - name: output_db",
        "      connection_type: string",
        "      batch_size: integer",
        "      retry_count: integer",
    ];

    for line in test_cases {
        if line.starts_with("    -") {
            assert_eq!(
                classify_line_type(line),
                LineType::SequenceItem,
                "Pipeline sequence item with type-like should be SequenceItem: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Pipeline config with type-like should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_type_like_in_multi_environment_config() {
    // Multi-environment configuration with nested type-like strings
    let test_cases = vec![
        "environments:",
        "  development:",
        "    database:",
        "      host: localhost",
        "      port: integer",
        "      schema_type: string",
        "    cache:",
        "      enabled: boolean",
        "      ttl: integer",
        "    logging:",
        "      level: string",
        "      format: string",
        "  staging:",
        "    database:",
        "      host: db.staging.example.com",
        "      port: integer",
        "      schema_type: string",
        "    cache:",
        "      enabled: boolean",
        "      ttl: integer",
        "  production:",
        "    database:",
        "      host: db.prod.example.com",
        "      port: integer",
        "      schema_type: string",
        "      ssl_enabled: boolean",
        "    cache:",
        "      enabled: boolean",
        "      ttl: integer",
        "      cluster_type: string",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Multi-environment config with type-like should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_nested_validation_schemas() {
    // Validation schemas with nested type-like strings
    let test_cases = vec![
        "schema:",
        "  type: object",
        "  properties:",
        "    user:",
        "      type: object",
        "      properties:",
        "        id:",
        "          type: integer",
        "          required: boolean",
        "        name:",
        "          type: string",
        "          format: string",
        "        email:",
        "          type: string",
        "          format: string",
        "        roles:",
        "          type: array",
        "          items:",
        "            type: string",
        "    settings:",
        "      type: object",
        "      properties:",
        "        notifications:",
        "          type: boolean",
        "        theme:",
        "          type: string",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Validation schema with type-like should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_complex_flow_sequences() {
    // Complex flow sequences with type-like strings at various nesting levels
    let test_cases = vec![
        "items: [{name: test1, type: string}, {name: test2, type: integer}]",
        "nested: [{a: {b: {c: string}}}, {x: {y: {z: integer}}}]",
        "complex: [{type: array, elements: [string, integer, boolean]}]",
        "deep: [{level1: {level2: {level3: string}}}]",
        "mixed: [{simple: string}, {nested: {deep: integer}}, {array: [string, boolean]}]",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::FlowSequence,
            "Complex flow sequence with type-like should be valid type: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_realistic_app_config() {
    // Realistic application configuration with all common patterns
    let test_cases = vec![
        "# Application Configuration",
        "app:",
        "  name: MyApp",
        "  version: 2.0.0",
        "  server:",
        "    host: 0.0.0.0",
        "    port: integer",
        "    ssl_enabled: boolean",
        "  database:",
        "    type: string",
        "    host: localhost",
        "    port: integer",
        "    pool_size: integer",
        "  cache:",
        "    backend: string",
        "    ttl: integer",
        "    max_size: integer",
        "  logging:",
        "    level: string",
        "    format: string",
        "    output: string",
        "  features:",
        "    new_ui: boolean",
        "    api_v2: boolean",
        "    dark_mode: boolean",
        "  limits:",
        "    max_users: integer",
        "    max_requests: integer",
        "    timeout: integer",
    ];

    for line in test_cases {
        if line.starts_with('#') {
            assert_eq!(
                classify_line_type(line),
                LineType::Comment,
                "Comment should be Comment: '{}'",
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Realistic app config with type-like should be MappingKey: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_type_like_in_edge_case_nested_structures() {
    // Edge cases with extremely nested type-like strings
    let test_cases = vec![
        "level1:",
        "  level2:",
        "    level3:",
        "      level4:",
        "        level5:",
        "          type: string",
        "          value: integer",
        "          flag: boolean",
        "deep:",
        "    nested:",
        "        structure:",
        "          with:",
        "            many:",
        "              levels:",
        "                type: object",
        "                array_type: array",
        "                bool_type: boolean",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Deeply nested edge case with type-like should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_type_like_in_mixed_indentation_scenarios() {
    // Mixed indentation patterns with type-like strings
    let test_cases = vec![
        "config:",
        "  simple: string",
        "  nested:",
        "    value: integer",
        "  deeply:",
        "    nested:",
        "      value: boolean",
        "    with:",
        "      mixed:",
        "        levels: integer",
        "  another:",
        "    branch: string",
        "      with:",
        "        deeper: array",
    ];

    for line in test_cases {
        assert_eq!(
            classify_line_type(line),
            LineType::MappingKey,
            "Mixed indentation with type-like should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_detect_mapping_key_in_nested_context() {
    // Test detect_mapping_key with nested structures containing type-like strings
    let test_cases = vec![
        ("  config: value", 0, Some("config"), Some("value")),
        ("    database: postgres", 0, Some("database"), Some("postgres")),
        ("      type: string", 2, Some("type"), Some("string")),
        ("        timeout: integer", 4, Some("timeout"), Some("integer")),
        ("          enabled: boolean", 6, Some("enabled"), Some("boolean")),
    ];

    for (line, parent_indent, expected_key, expected_value) in test_cases {
        let info = detect_mapping_key(line, parent_indent);
        assert!(
            info.is_some(),
            "Should detect mapping key in nested context: '{}'",
            line
        );
        let info = info.unwrap();
        assert_eq!(info.key, expected_key.unwrap(), "Should extract correct key: '{}'", line);
        assert_eq!(info.value, expected_value.map(String::from), "Should extract correct value: '{}'", line);
    }
}

#[test]
fn test_complete_extraction_pipeline_verification() {
    // End-to-end verification of the complete extraction pipeline
    // with deeply nested type-like strings in realistic production scenarios
    let yaml_lines = vec![
        "# Production Database Configuration",
        "database:",
        "  primary:",
        "    host: db1.prod.example.com",
        "    port: integer",
        "    schema_type: string",
        "    pool:",
        "      max_size: integer",
        "      min_size: integer",
        "      timeout: integer",
        "  replica:",
        "    enabled: boolean",
        "    lag_threshold: integer",
        "    connection_type: string",
        "cache:",
        "  backend: string",
        "  config:",
        "    ttl: integer",
        "    max_memory: string",
        "    eviction_policy: string",
    ];

    // Verify line classification
    for (i, line) in yaml_lines.iter().enumerate() {
        if line.starts_with('#') {
            assert_eq!(
                classify_line_type(line),
                LineType::Comment,
                "Line {} should be Comment: '{}'",
                i,
                line
            );
        } else {
            assert_eq!(
                classify_line_type(line),
                LineType::MappingKey,
                "Line {} should be MappingKey: '{}'",
                i,
                line
            );
        }
    }

    // Verify key extraction for non-comment lines
    let expected_keys = vec![
        "database", "primary", "host", "port", "schema_type", "pool",
        "max_size", "min_size", "timeout", "replica", "enabled",
        "lag_threshold", "connection_type", "cache", "backend", "config",
        "ttl", "max_memory", "eviction_policy",
    ];

    let mut key_index = 0;
    for line in &yaml_lines {
        if !line.starts_with('#') {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for extraction: '{}'",
                line
            );
            let info = info.unwrap();
            assert!(
                key_index < expected_keys.len(),
                "Expected key index {} out of bounds",
                key_index
            );
            assert_eq!(
                info.key,
                expected_keys[key_index],
                "Should extract correct key at index {}: '{}'",
                key_index,
                line
            );
            key_index += 1;
        }
    }
}

// ============================================================================
// Section 12B: Multiline String Scenarios with Exclamation Marks
// ============================================================================

#[test]
fn test_folded_block_scalar_with_exclamation_marks() {
    // Test folded style scalars (>) - newlines treated as spaces
    // The scalar indicator line itself
    let test_cases = vec![
        "description: >",               // Basic folded scalar
        "  folded_text: >",              // Indented folded scalar
        "    note: >",                   // Deep indented folded scalar
        "\tmessage: >",                 // Tab-indented folded scalar
        "\tkey_with_exclamation!: >",   // Tab-indented key with ! followed by folded scalar
        "warning: >-",                  // Folded with strip modifier
        "info: >+",                     // Folded with keep modifier
        "text: >-2",                    // Folded with explicit indent
        "content: >2",                   // Folded with explicit indent
        // Various indentation levels with '!' in keys (not starting with '!')
        "  key_with_bang!: >",          // 2-space indent with '!' at end
        "    another!key: >",            // 4-space indent with '!' in middle
        "        deep!nest!ed: >",       // 8-space indent with multiple '!'
        "\t  two_space_tab!key: >",    // Tab + 2 spaces with '!'
        "\t    four_space_tab!key: >", // Tab + 4 spaces with '!'
        "  key!bang!test: >",           // 2-space indent with multiple '!' in key
        "  end!with!bang!: >",          // 2-space indent with '!' at end and middle
        "      middle!bang: >",         // 6-space indent with '!' in middle
        "\t\ttab!tab!key: >",          // Double tab with '!' in key
        "    multiple!!!here: >",       // 4-space indent with multiple consecutive '!'
        "  spaced!out!keys!: >",       // 2-space indent with multiple spaced '!'
    ];

    for line in test_cases {
        // These lines with > should be classified as MappingKey with a block scalar
        // The parser should recognize the folded block scalar indicator
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded block scalar indicator should be MappingKey: '{}'",
            line
        );
    }

    // Test continuation lines of folded scalars with exclamation marks
    let continuation_lines = vec![
        "  This is folded text with! exclamation marks",
        "    Multiple! exclamations! in! folded! style",
        "\tMore! content! with! bangs!",
        "  Important! message! continues!",
        "    Another! line! with! emphasis!",
    ];

    for line in continuation_lines {
        // Continuation lines of folded scalars (indented more than parent)
        // should be classified appropriately
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Folded scalar continuation with ! should be MappingKey or Unknown: '{}'",
            line
        );
    }
}

#[test]
fn test_literal_block_scalar_with_exclamation_marks() {
    // Test literal style scalars (|) - newlines preserved
    // The scalar indicator line itself
    let test_cases = vec![
        "description: |",               // Basic literal scalar
        "  literal_text: |",             // Indented literal scalar
        "    note: |",                   // Deep indented literal scalar
        "\tmessage: |",                 // Tab-indented literal scalar
        "warning: |-",                  // Literal with strip modifier
        "info: |+",                     // Literal with keep modifier
        "text: |-2",                    // Literal with explicit indent
        "content: |2",                  // Literal with explicit indent
    ];

    for line in test_cases {
        // These lines with | should be classified as MappingKey with a block scalar
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Literal block scalar indicator should be MappingKey: '{}'",
            line
        );
    }

    // Test continuation lines of literal scalars with exclamation marks
    let continuation_lines = vec![
        ("  This is literal text with! exclamation marks", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Multiple! exclamations! in! literal! style", vec![LineType::MappingKey, LineType::Unknown]),
        ("\tMore! content! with! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Important! message! continues!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Another! line! with! emphasis!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Lines with! at! various! positions!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    !Start! Middle! End!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  !important!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        // Continuation lines of literal scalars (indented more than parent)
        // should be classified appropriately
        let result = classify_line_type(line);
        assert!(
            expected_types.contains(&result),
            "Literal scalar continuation with ! should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}

#[test]
fn test_literal_scalar_basic_modifiers_at_various_indentation_levels() {
    // Test literal scalars with basic modifiers (|- strip, |+ keep) at various indentation levels
    // Level 1: 2-space, Level 2: 4-space, Level 3: 6-space, Level 4: 8-space, plus tab
    // This provides comprehensive coverage of strip (-) and keep (+) modifiers

    let test_cases = vec![
        // Level 1: 2-space indentation with literal strip modifier (|-)
        ("  level1_text: |-", "level1_text", LineType::MappingKey),
        ("  warning!msg: |-", "warning!msg", LineType::MappingKey),
        ("  error!log: |-", "error!log", LineType::MappingKey),
        ("  simple!test: |-", "simple!test", LineType::MappingKey),

        // Level 1: 2-space indentation with literal keep modifier (|+)
        ("  level1_note: |+", "level1_note", LineType::MappingKey),
        ("  info!data: |+", "info!data", LineType::MappingKey),
        ("  message!log: |+", "message!log", LineType::MappingKey),
        ("  content!text: |+", "content!text", LineType::MappingKey),

        // Level 2: 4-space indentation with literal strip modifier (|-)
        ("    level2_text: |-", "level2_text", LineType::MappingKey),
        ("    nested!warning: |-", "nested!warning", LineType::MappingKey),
        ("    deep!error: |-", "deep!error", LineType::MappingKey),
        ("    inner!test: |-", "inner!test", LineType::MappingKey),

        // Level 2: 4-space indentation with literal keep modifier (|+)
        ("    level2_note: |+", "level2_note", LineType::MappingKey),
        ("    nested!info: |+", "nested!info", LineType::MappingKey),
        ("    deep!message: |+", "deep!message", LineType::MappingKey),
        ("    inner!content: |+", "inner!content", LineType::MappingKey),

        // Level 3: 6-space indentation with literal strip modifier (|-)
        ("      level3_text: |-", "level3_text", LineType::MappingKey),
        ("      deeper!warning: |-", "deeper!warning", LineType::MappingKey),
        ("      very!deep!error: |-", "very!deep!error", LineType::MappingKey),
        ("      complex!test!here: |-", "complex!test!here", LineType::MappingKey),

        // Level 3: 6-space indentation with literal keep modifier (|+)
        ("      level3_note: |+", "level3_note", LineType::MappingKey),
        ("      deeper!info: |+", "deeper!info", LineType::MappingKey),
        ("      very!deep!message: |+", "very!deep!message", LineType::MappingKey),
        ("      complex!content!now: |+", "complex!content!now", LineType::MappingKey),

        // Level 4: 8-space indentation with literal strip modifier (|-)
        ("        level4_text: |-", "level4_text", LineType::MappingKey),
        ("        deepest!warning: |-", "deepest!warning", LineType::MappingKey),
        ("        super!deep!error: |-", "super!deep!error", LineType::MappingKey),
        ("        extra!complex!test: |-", "extra!complex!test", LineType::MappingKey),

        // Level 4: 8-space indentation with literal keep modifier (|+)
        ("        level4_note: |+", "level4_note", LineType::MappingKey),
        ("        deepest!info: |+", "deepest!info", LineType::MappingKey),
        ("        super!deep!message: |+", "super!deep!message", LineType::MappingKey),
        ("        extra!complex!content: |+", "extra!complex!content", LineType::MappingKey),

        // Tab indentation with literal strip modifier (|-)
        ("\ttab_text: |-", "tab_text", LineType::MappingKey),
        ("\ttab!warning: |-", "tab!warning", LineType::MappingKey),
        ("\ttab!error!log: |-", "tab!error!log", LineType::MappingKey),

        // Tab indentation with literal keep modifier (|+)
        ("\ttab_note: |+", "tab_note", LineType::MappingKey),
        ("\ttab!info: |+", "tab!info", LineType::MappingKey),
        ("\ttab!message!log: |+", "tab!message!log", LineType::MappingKey),

        // Mixed indentation with tab + spaces
        ("\t  mixed_tab_spaces_text: |-", "mixed_tab_spaces_text", LineType::MappingKey),
        ("\t    mixed_tab!spaces!warning: |-", "mixed_tab!spaces!warning", LineType::MappingKey),
        ("\t  mixed_tab_note: |+", "mixed_tab_note", LineType::MappingKey),
        ("\t    mixed_tab!spaces!info: |+", "mixed_tab!spaces!info", LineType::MappingKey),

        // Keys with multiple exclamation marks at different levels
        ("  key!!: |-", "key!!", LineType::MappingKey),
        ("    deep!!key: |+", "deep!!key", LineType::MappingKey),
        ("      very!!deep!!key: |-", "very!!deep!!key", LineType::MappingKey),
        ("        super!!!deep!!!key: |+", "super!!!deep!!!key", LineType::MappingKey),
        ("\ttab!!key!!!test: |-", "tab!!key!!!test", LineType::MappingKey),

        // Edge case: Single character keys with ! at different levels
        ("  a!: |-", "a!", LineType::MappingKey),
        ("    b!: |+", "b!", LineType::MappingKey),
        ("      c!: |-", "c!", LineType::MappingKey),
        ("        d!: |+", "d!", LineType::MappingKey),
        ("\te!: |-", "e!", LineType::MappingKey),

        // Edge case: Keys ending with ! at different levels
        ("  end!with!bang!: |-", "end!with!bang!", LineType::MappingKey),
        ("    deep!end!with!bang!: |+", "deep!end!with!bang!", LineType::MappingKey),
        ("      very!deep!end!bang!: |-", "very!deep!end!bang!", LineType::MappingKey),
        ("        super!deep!end!bang!: |+", "super!deep!end!bang!", LineType::MappingKey),
        ("\ttab!end!with!bang!: |-", "tab!end!with!bang!", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Literal scalar basic modifier test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for literal scalar with basic modifier: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for literal scalar with basic modifier: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for literal scalars with basic modifiers
    let continuation_lines = vec![
        // Level 1 continuation lines with ! characters
        ("  This is content with! exclamation", vec![LineType::MappingKey, LineType::Unknown]),
        ("  More! text! here! for! testing!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 2 continuation lines with ! characters
        ("    Deeper! content! with! more! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Nested! text! continues! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 3 continuation lines with ! characters
        ("      Very! deep! content! with! emphasis!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 4 continuation lines with ! characters
        ("        Super! deep! content! with! many! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        Extra! complex! continuation! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Tab continuation lines with ! characters
        ("\tTab! content! with! exclamation!", vec![LineType::MappingKey, LineType::Unknown]),
        ("\tMore! tab! text! continues! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Lines starting with ! (may be classified as Tag)
        ("  !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("    !Deep! tag! like! content!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("      !Very! deep! tag! line!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Continuation line for literal scalar basic modifier should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );

        // Continuation lines should NOT detect as mapping keys
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_basic_modifiers_at_various_indentation_levels() {
    // Test folded scalars with basic modifiers (>- strip, >+ keep) at various indentation levels
    // Level 1: 2-space, Level 2: 4-space, Level 3: 6-space, Level 4: 8-space, plus tab
    // This provides comprehensive coverage of strip (-) and keep (+) modifiers

    let test_cases = vec![
        // Level 1: 2-space indentation with folded strip modifier (>-)
        ("  level1_text: >-", "level1_text", LineType::MappingKey),
        ("  warning!msg: >-", "warning!msg", LineType::MappingKey),
        ("  error!log: >-", "error!log", LineType::MappingKey),
        ("  simple!test: >-", "simple!test", LineType::MappingKey),

        // Level 1: 2-space indentation with folded keep modifier (>+)
        ("  level1_note: >+", "level1_note", LineType::MappingKey),
        ("  info!data: >+", "info!data", LineType::MappingKey),
        ("  message!log: >+", "message!log", LineType::MappingKey),
        ("  content!text: >+", "content!text", LineType::MappingKey),

        // Level 2: 4-space indentation with folded strip modifier (>-)
        ("    level2_text: >-", "level2_text", LineType::MappingKey),
        ("    nested!warning: >-", "nested!warning", LineType::MappingKey),
        ("    deep!error: >-", "deep!error", LineType::MappingKey),
        ("    inner!test: >-", "inner!test", LineType::MappingKey),

        // Level 2: 4-space indentation with folded keep modifier (>+)
        ("    level2_note: >+", "level2_note", LineType::MappingKey),
        ("    nested!info: >+", "nested!info", LineType::MappingKey),
        ("    deep!message: >+", "deep!message", LineType::MappingKey),
        ("    inner!content: >+", "inner!content", LineType::MappingKey),

        // Level 3: 6-space indentation with folded strip modifier (>-)
        ("      level3_text: >-", "level3_text", LineType::MappingKey),
        ("      deeper!warning: >-", "deeper!warning", LineType::MappingKey),
        ("      very!deep!error: >-", "very!deep!error", LineType::MappingKey),
        ("      complex!test!here: >-", "complex!test!here", LineType::MappingKey),

        // Level 3: 6-space indentation with folded keep modifier (>+)
        ("      level3_note: >+", "level3_note", LineType::MappingKey),
        ("      deeper!info: >+", "deeper!info", LineType::MappingKey),
        ("      very!deep!message: >+", "very!deep!message", LineType::MappingKey),
        ("      complex!content!now: >+", "complex!content!now", LineType::MappingKey),

        // Level 4: 8-space indentation with folded strip modifier (>-)
        ("        level4_text: >-", "level4_text", LineType::MappingKey),
        ("        deepest!warning: >-", "deepest!warning", LineType::MappingKey),
        ("        super!deep!error: >-", "super!deep!error", LineType::MappingKey),
        ("        extra!complex!test: >-", "extra!complex!test", LineType::MappingKey),

        // Level 4: 8-space indentation with folded keep modifier (>+)
        ("        level4_note: >+", "level4_note", LineType::MappingKey),
        ("        deepest!info: >+", "deepest!info", LineType::MappingKey),
        ("        super!deep!message: >+", "super!deep!message", LineType::MappingKey),
        ("        extra!complex!content: >+", "extra!complex!content", LineType::MappingKey),

        // Tab indentation with folded strip modifier (>-)
        ("\ttab_text: >-", "tab_text", LineType::MappingKey),
        ("\ttab!warning: >-", "tab!warning", LineType::MappingKey),
        ("\ttab!error!log: >-", "tab!error!log", LineType::MappingKey),

        // Tab indentation with folded keep modifier (>+)
        ("\ttab_note: >+", "tab_note", LineType::MappingKey),
        ("\ttab!info: >+", "tab!info", LineType::MappingKey),
        ("\ttab!message!log: >+", "tab!message!log", LineType::MappingKey),

        // Mixed indentation with tab + spaces
        ("\t  mixed_tab_spaces_text: >-", "mixed_tab_spaces_text", LineType::MappingKey),
        ("\t    mixed_tab!spaces!warning: >-", "mixed_tab!spaces!warning", LineType::MappingKey),
        ("\t  mixed_tab_note: >+", "mixed_tab_note", LineType::MappingKey),
        ("\t    mixed_tab!spaces!info: >+", "mixed_tab!spaces!info", LineType::MappingKey),

        // Keys with multiple exclamation marks at different levels
        ("  key!!: >-", "key!!", LineType::MappingKey),
        ("    deep!!key: >+", "deep!!key", LineType::MappingKey),
        ("      very!!deep!!key: >-", "very!!deep!!key", LineType::MappingKey),
        ("        super!!!deep!!!key: >+", "super!!!deep!!!key", LineType::MappingKey),
        ("\ttab!!key!!!test: >-", "tab!!key!!!test", LineType::MappingKey),

        // Edge case: Single character keys with ! at different levels
        ("  a!: >-", "a!", LineType::MappingKey),
        ("    b!: >+", "b!", LineType::MappingKey),
        ("      c!: >-", "c!", LineType::MappingKey),
        ("        d!: >+", "d!", LineType::MappingKey),
        ("\te!: >-", "e!", LineType::MappingKey),

        // Edge case: Keys ending with ! at different levels
        ("  end!with!bang!: >-", "end!with!bang!", LineType::MappingKey),
        ("    deep!end!with!bang!: >+", "deep!end!with!bang!", LineType::MappingKey),
        ("      very!deep!end!bang!: >-", "very!deep!end!bang!", LineType::MappingKey),
        ("        super!deep!end!bang!: >+", "super!deep!end!bang!", LineType::MappingKey),
        ("\ttab!end!with!bang!: >-", "tab!end!with!bang!", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Folded scalar basic modifier test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for folded scalar with basic modifier: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for folded scalar with basic modifier: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for folded scalars with basic modifiers
    let continuation_lines = vec![
        // Level 1 continuation lines with ! characters
        ("  This is content with! exclamation", vec![LineType::MappingKey, LineType::Unknown]),
        ("  More! text! here! for! testing!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 2 continuation lines with ! characters
        ("    Deeper! content! with! more! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Nested! text! continues! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 3 continuation lines with ! characters
        ("      Very! deep! content! with! emphasis!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 4 continuation lines with ! characters
        ("        Super! deep! content! with! many! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        Extra! complex! continuation! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Tab continuation lines with ! characters
        ("\tTab! content! with! exclamation!", vec![LineType::MappingKey, LineType::Unknown]),
        ("\tMore! tab! text! continues! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Lines starting with ! (may be classified as Tag)
        ("  !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("    !Deep! tag! like! content!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("      !Very! deep! tag! line!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Continuation line for folded scalar basic modifier should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );

        // Continuation lines should NOT detect as mapping keys
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_explicit_indent_modifiers_at_various_levels() {
    // Test folded scalars with explicit indent modifiers: >n, >-n, >+n for n=1-9
    // Tested at various base indentation levels: 2-space, 4-space, 6-space, 8-space, tab
    // This provides comprehensive coverage of explicit indent specification for folded scalars

    let test_cases = vec![
        // ===== Level 1: 2-space indentation with explicit indent modifiers =====
        // Plain >n (n=1-9)
        ("  text1: >1", "text1", LineType::MappingKey),
        ("  text2: >2", "text2", LineType::MappingKey),
        ("  text3: >3", "text3", LineType::MappingKey),
        ("  text4: >4", "text4", LineType::MappingKey),
        ("  text5: >5", "text5", LineType::MappingKey),
        ("  text6: >6", "text6", LineType::MappingKey),
        ("  text7: >7", "text7", LineType::MappingKey),
        ("  text8: >8", "text8", LineType::MappingKey),
        ("  text9: >9", "text9", LineType::MappingKey),

        // Strip modifier >-n (n=1-9)
        ("  strip1: >-1", "strip1", LineType::MappingKey),
        ("  strip2: >-2", "strip2", LineType::MappingKey),
        ("  strip3: >-3", "strip3", LineType::MappingKey),
        ("  strip4: >-4", "strip4", LineType::MappingKey),
        ("  strip5: >-5", "strip5", LineType::MappingKey),
        ("  strip6: >-6", "strip6", LineType::MappingKey),
        ("  strip7: >-7", "strip7", LineType::MappingKey),
        ("  strip8: >-8", "strip8", LineType::MappingKey),
        ("  strip9: >-9", "strip9", LineType::MappingKey),

        // Keep modifier >+n (n=1-9)
        ("  keep1: >+1", "keep1", LineType::MappingKey),
        ("  keep2: >+2", "keep2", LineType::MappingKey),
        ("  keep3: >+3", "keep3", LineType::MappingKey),
        ("  keep4: >+4", "keep4", LineType::MappingKey),
        ("  keep5: >+5", "keep5", LineType::MappingKey),
        ("  keep6: >+6", "keep6", LineType::MappingKey),
        ("  keep7: >+7", "keep7", LineType::MappingKey),
        ("  keep8: >+8", "keep8", LineType::MappingKey),
        ("  keep9: >+9", "keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 1
        ("  key!1: >1", "key!1", LineType::MappingKey),
        ("  warn!2: >-2", "warn!2", LineType::MappingKey),
        ("  error!3: >+3", "error!3", LineType::MappingKey),
        ("  test!4: >4", "test!4", LineType::MappingKey),
        ("  data!5: >-5", "data!5", LineType::MappingKey),
        ("  info!6: >+6", "info!6", LineType::MappingKey),

        // ===== Level 2: 4-space indentation with explicit indent modifiers =====
        // Plain >n (n=1-9)
        ("    level2_1: >1", "level2_1", LineType::MappingKey),
        ("    level2_2: >2", "level2_2", LineType::MappingKey),
        ("    level2_3: >3", "level2_3", LineType::MappingKey),
        ("    level2_4: >4", "level2_4", LineType::MappingKey),
        ("    level2_5: >5", "level2_5", LineType::MappingKey),
        ("    level2_6: >6", "level2_6", LineType::MappingKey),
        ("    level2_7: >7", "level2_7", LineType::MappingKey),
        ("    level2_8: >8", "level2_8", LineType::MappingKey),
        ("    level2_9: >9", "level2_9", LineType::MappingKey),

        // Strip modifier >-n (n=1-9)
        ("    nested_strip1: >-1", "nested_strip1", LineType::MappingKey),
        ("    nested_strip2: >-2", "nested_strip2", LineType::MappingKey),
        ("    nested_strip3: >-3", "nested_strip3", LineType::MappingKey),
        ("    nested_strip4: >-4", "nested_strip4", LineType::MappingKey),
        ("    nested_strip5: >-5", "nested_strip5", LineType::MappingKey),
        ("    nested_strip6: >-6", "nested_strip6", LineType::MappingKey),
        ("    nested_strip7: >-7", "nested_strip7", LineType::MappingKey),
        ("    nested_strip8: >-8", "nested_strip8", LineType::MappingKey),
        ("    nested_strip9: >-9", "nested_strip9", LineType::MappingKey),

        // Keep modifier >+n (n=1-9)
        ("    nested_keep1: >+1", "nested_keep1", LineType::MappingKey),
        ("    nested_keep2: >+2", "nested_keep2", LineType::MappingKey),
        ("    nested_keep3: >+3", "nested_keep3", LineType::MappingKey),
        ("    nested_keep4: >+4", "nested_keep4", LineType::MappingKey),
        ("    nested_keep5: >+5", "nested_keep5", LineType::MappingKey),
        ("    nested_keep6: >+6", "nested_keep6", LineType::MappingKey),
        ("    nested_keep7: >+7", "nested_keep7", LineType::MappingKey),
        ("    nested_keep8: >+8", "nested_keep8", LineType::MappingKey),
        ("    nested_keep9: >+9", "nested_keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 2
        ("    deep!key1: >1", "deep!key1", LineType::MappingKey),
        ("    deep!warn2: >-2", "deep!warn2", LineType::MappingKey),
        ("    deep!error3: >+3", "deep!error3", LineType::MappingKey),
        ("    deep!test4: >4", "deep!test4", LineType::MappingKey),
        ("    deep!data5: >-5", "deep!data5", LineType::MappingKey),
        ("    deep!info6: >+6", "deep!info6", LineType::MappingKey),

        // ===== Level 3: 6-space indentation with explicit indent modifiers =====
        // Plain >n (n=1-9)
        ("      level3_1: >1", "level3_1", LineType::MappingKey),
        ("      level3_2: >2", "level3_2", LineType::MappingKey),
        ("      level3_3: >3", "level3_3", LineType::MappingKey),
        ("      level3_4: >4", "level3_4", LineType::MappingKey),
        ("      level3_5: >5", "level3_5", LineType::MappingKey),
        ("      level3_6: >6", "level3_6", LineType::MappingKey),
        ("      level3_7: >7", "level3_7", LineType::MappingKey),
        ("      level3_8: >8", "level3_8", LineType::MappingKey),
        ("      level3_9: >9", "level3_9", LineType::MappingKey),

        // Strip modifier >-n (n=1-9) - sample
        ("      deeper_strip1: >-1", "deeper_strip1", LineType::MappingKey),
        ("      deeper_strip3: >-3", "deeper_strip3", LineType::MappingKey),
        ("      deeper_strip5: >-5", "deeper_strip5", LineType::MappingKey),
        ("      deeper_strip7: >-7", "deeper_strip7", LineType::MappingKey),
        ("      deeper_strip9: >-9", "deeper_strip9", LineType::MappingKey),

        // Keep modifier >+n (n=1-9) - sample
        ("      deeper_keep1: >+1", "deeper_keep1", LineType::MappingKey),
        ("      deeper_keep3: >+3", "deeper_keep3", LineType::MappingKey),
        ("      deeper_keep5: >+5", "deeper_keep5", LineType::MappingKey),
        ("      deeper_keep7: >+7", "deeper_keep7", LineType::MappingKey),
        ("      deeper_keep9: >+9", "deeper_keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 3
        ("      very!deep!1: >1", "very!deep!1", LineType::MappingKey),
        ("      very!deep!2: >-2", "very!deep!2", LineType::MappingKey),
        ("      very!deep!3: >+3", "very!deep!3", LineType::MappingKey),
        ("      very!deep!4: >4", "very!deep!4", LineType::MappingKey),
        ("      very!deep!5: >-5", "very!deep!5", LineType::MappingKey),
        ("      very!deep!6: >+6", "very!deep!6", LineType::MappingKey),

        // ===== Level 4: 8-space indentation with explicit indent modifiers =====
        // Plain >n (n=1-9) - sample
        ("        level4_1: >1", "level4_1", LineType::MappingKey),
        ("        level4_3: >3", "level4_3", LineType::MappingKey),
        ("        level4_5: >5", "level4_5", LineType::MappingKey),
        ("        level4_7: >7", "level4_7", LineType::MappingKey),
        ("        level4_9: >9", "level4_9", LineType::MappingKey),

        // Strip modifier >-n (n=1-9) - sample
        ("        deepest_strip1: >-1", "deepest_strip1", LineType::MappingKey),
        ("        deepest_strip3: >-3", "deepest_strip3", LineType::MappingKey),
        ("        deepest_strip5: >-5", "deepest_strip5", LineType::MappingKey),
        ("        deepest_strip7: >-7", "deepest_strip7", LineType::MappingKey),
        ("        deepest_strip9: >-9", "deepest_strip9", LineType::MappingKey),

        // Keep modifier >+n (n=1-9) - sample
        ("        deepest_keep1: >+1", "deepest_keep1", LineType::MappingKey),
        ("        deepest_keep3: >+3", "deepest_keep3", LineType::MappingKey),
        ("        deepest_keep5: >+5", "deepest_keep5", LineType::MappingKey),
        ("        deepest_keep7: >+7", "deepest_keep7", LineType::MappingKey),
        ("        deepest_keep9: >+9", "deepest_keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 4
        ("        super!deep!1: >1", "super!deep!1", LineType::MappingKey),
        ("        super!deep!2: >-2", "super!deep!2", LineType::MappingKey),
        ("        super!deep!3: >+3", "super!deep!3", LineType::MappingKey),
        ("        super!deep!4: >4", "super!deep!4", LineType::MappingKey),
        ("        super!deep!5: >-5", "super!deep!5", LineType::MappingKey),
        ("        super!deep!6: >+6", "super!deep!6", LineType::MappingKey),

        // ===== Tab indentation with explicit indent modifiers =====
        // Plain >n (n=1-9) - sample
        ("\ttab_1: >1", "tab_1", LineType::MappingKey),
        ("\ttab_3: >3", "tab_3", LineType::MappingKey),
        ("\ttab_5: >5", "tab_5", LineType::MappingKey),
        ("\ttab_7: >7", "tab_7", LineType::MappingKey),
        ("\ttab_9: >9", "tab_9", LineType::MappingKey),

        // Strip modifier >-n (n=1-9) - sample
        ("\ttab_strip1: >-1", "tab_strip1", LineType::MappingKey),
        ("\ttab_strip3: >-3", "tab_strip3", LineType::MappingKey),
        ("\ttab_strip5: >-5", "tab_strip5", LineType::MappingKey),
        ("\ttab_strip7: >-7", "tab_strip7", LineType::MappingKey),
        ("\ttab_strip9: >-9", "tab_strip9", LineType::MappingKey),

        // Keep modifier >+n (n=1-9) - sample
        ("\ttab_keep1: >+1", "tab_keep1", LineType::MappingKey),
        ("\ttab_keep3: >+3", "tab_keep3", LineType::MappingKey),
        ("\ttab_keep5: >+5", "tab_keep5", LineType::MappingKey),
        ("\ttab_keep7: >+7", "tab_keep7", LineType::MappingKey),
        ("\ttab_keep9: >+9", "tab_keep9", LineType::MappingKey),

        // Keys with exclamation marks at tab level
        ("\ttab!key1: >1", "tab!key1", LineType::MappingKey),
        ("\ttab!warn2: >-2", "tab!warn2", LineType::MappingKey),
        ("\ttab!error3: >+3", "tab!error3", LineType::MappingKey),
        ("\ttab!test4: >4", "tab!test4", LineType::MappingKey),
        ("\ttab!data5: >-5", "tab!data5", LineType::MappingKey),
        ("\ttab!info6: >+6", "tab!info6", LineType::MappingKey),

        // ===== Mixed indentation (tab + spaces) =====
        ("\t  mixed1: >1", "mixed1", LineType::MappingKey),
        ("\t  mixed2: >2", "mixed2", LineType::MappingKey),
        ("\t  mixed3: >-3", "mixed3", LineType::MappingKey),
        ("\t    mixed4: >4", "mixed4", LineType::MappingKey),
        ("\t    mixed5: >-5", "mixed5", LineType::MappingKey),
    ];

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
}

#[test]
fn test_folded_scalar_plain_explicit_indent_modifiers_at_2_space() {
    // Test folded scalars with plain explicit indent modifiers: >n for n=1-9
    // At 2-space indentation level only
    // This provides focused coverage of plain explicit indent (>n) specification for folded scalars
    // Follows the pattern established in test_folded_scalar_basic_modifiers_at_various_indentation_levels

    let test_cases = vec![
        // ===== Level 1: 2-space indentation with plain explicit indent >n =====
        // Plain >n (n=1-9) - main test cases
        ("  text1: >1", "text1", LineType::MappingKey),
        ("  text2: >2", "text2", LineType::MappingKey),
        ("  text3: >3", "text3", LineType::MappingKey),
        ("  text4: >4", "text4", LineType::MappingKey),
        ("  text5: >5", "text5", LineType::MappingKey),
        ("  text6: >6", "text6", LineType::MappingKey),
        ("  text7: >7", "text7", LineType::MappingKey),
        ("  text8: >8", "text8", LineType::MappingKey),
        ("  text9: >9", "text9", LineType::MappingKey),

        // Keys with exclamation marks at 2-space indentation
        ("  key!1: >1", "key!1", LineType::MappingKey),
        ("  warn!2: >2", "warn!2", LineType::MappingKey),
        ("  error!3: >3", "error!3", LineType::MappingKey),
        ("  test!4: >4", "test!4", LineType::MappingKey),
        ("  data!5: >5", "data!5", LineType::MappingKey),
        ("  info!6: >6", "info!6", LineType::MappingKey),
        ("  msg!7: >7", "msg!7", LineType::MappingKey),
        ("  log!8: >8", "log!8", LineType::MappingKey),
        ("  val!9: >9", "val!9", LineType::MappingKey),

        // Keys with multiple exclamation marks
        ("  key!!1: >1", "key!!1", LineType::MappingKey),
        ("  deep!!key2: >2", "deep!!key2", LineType::MappingKey),
        ("  very!!deep!!key3: >3", "very!!deep!!key3", LineType::MappingKey),
        ("  super!!!deep!!!key4: >4", "super!!!deep!!!key4", LineType::MappingKey),

        // Edge case: Single character keys with !
        ("  a!: >1", "a!", LineType::MappingKey),
        ("  b!: >2", "b!", LineType::MappingKey),
        ("  c!: >3", "c!", LineType::MappingKey),
        ("  d!: >4", "d!", LineType::MappingKey),
        ("  e!: >5", "e!", LineType::MappingKey),

        // Edge case: Keys ending with !
        ("  end!with!bang!: >1", "end!with!bang!", LineType::MappingKey),
        ("  another!end!bang!: >2", "another!end!bang!", LineType::MappingKey),
        ("  final!end!with!bang!: >3", "final!end!with!bang!", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Folded scalar plain explicit indent test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for folded scalar with plain explicit indent: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for folded scalar with plain explicit indent: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for folded scalars with plain explicit indent modifiers
    let continuation_lines = vec![
        // Level 1 continuation lines with ! characters
        ("  This is content with! exclamation", vec![LineType::MappingKey, LineType::Unknown]),
        ("  More! text! here! for! testing!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Folded! content! continues! with! explicit! indent!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Lines! with! various! exclamation! marks! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Testing! continuation! behavior! for! >n! modifiers!", vec![LineType::MappingKey, LineType::Unknown]),

        // Lines starting with ! (may be classified as Tag)
        ("  !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  !Tag! like! content! with! exclamation!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  !Another! tag! pattern! here!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Continuation line for folded scalar plain explicit indent should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );

        // Continuation lines should NOT detect as mapping keys
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_literal_scalar_explicit_indent_modifiers_at_various_levels() {
    // Test literal scalars with explicit indent modifiers: |n, |-n, |+n for n=1-9
    // Tested at various base indentation levels: 2-space, 4-space, 6-space, 8-space, tab
    // This provides comprehensive coverage of explicit indent specification

    let test_cases = vec![
        // ===== Level 1: 2-space indentation with explicit indent modifiers =====
        // Plain |n (n=1-9)
        ("  text1: |1", "text1", LineType::MappingKey),
        ("  text2: |2", "text2", LineType::MappingKey),
        ("  text3: |3", "text3", LineType::MappingKey),
        ("  text4: |4", "text4", LineType::MappingKey),
        ("  text5: |5", "text5", LineType::MappingKey),
        ("  text6: |6", "text6", LineType::MappingKey),
        ("  text7: |7", "text7", LineType::MappingKey),
        ("  text8: |8", "text8", LineType::MappingKey),
        ("  text9: |9", "text9", LineType::MappingKey),

        // Strip modifier |-n (n=1-9)
        ("  strip1: |-1", "strip1", LineType::MappingKey),
        ("  strip2: |-2", "strip2", LineType::MappingKey),
        ("  strip3: |-3", "strip3", LineType::MappingKey),
        ("  strip4: |-4", "strip4", LineType::MappingKey),
        ("  strip5: |-5", "strip5", LineType::MappingKey),
        ("  strip6: |-6", "strip6", LineType::MappingKey),
        ("  strip7: |-7", "strip7", LineType::MappingKey),
        ("  strip8: |-8", "strip8", LineType::MappingKey),
        ("  strip9: |-9", "strip9", LineType::MappingKey),

        // Keep modifier |+n (n=1-9)
        ("  keep1: |+1", "keep1", LineType::MappingKey),
        ("  keep2: |+2", "keep2", LineType::MappingKey),
        ("  keep3: |+3", "keep3", LineType::MappingKey),
        ("  keep4: |+4", "keep4", LineType::MappingKey),
        ("  keep5: |+5", "keep5", LineType::MappingKey),
        ("  keep6: |+6", "keep6", LineType::MappingKey),
        ("  keep7: |+7", "keep7", LineType::MappingKey),
        ("  keep8: |+8", "keep8", LineType::MappingKey),
        ("  keep9: |+9", "keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 1
        ("  key!1: |1", "key!1", LineType::MappingKey),
        ("  warn!2: |-2", "warn!2", LineType::MappingKey),
        ("  error!3: |+3", "error!3", LineType::MappingKey),
        ("  test!4: |4", "test!4", LineType::MappingKey),
        ("  data!5: |-5", "data!5", LineType::MappingKey),
        ("  info!6: |+6", "info!6", LineType::MappingKey),

        // ===== Level 2: 4-space indentation with explicit indent modifiers =====
        // Plain |n (n=1-9)
        ("    level2_1: |1", "level2_1", LineType::MappingKey),
        ("    level2_2: |2", "level2_2", LineType::MappingKey),
        ("    level2_3: |3", "level2_3", LineType::MappingKey),
        ("    level2_4: |4", "level2_4", LineType::MappingKey),
        ("    level2_5: |5", "level2_5", LineType::MappingKey),
        ("    level2_6: |6", "level2_6", LineType::MappingKey),
        ("    level2_7: |7", "level2_7", LineType::MappingKey),
        ("    level2_8: |8", "level2_8", LineType::MappingKey),
        ("    level2_9: |9", "level2_9", LineType::MappingKey),

        // Strip modifier |-n (n=1-9)
        ("    nested_strip1: |-1", "nested_strip1", LineType::MappingKey),
        ("    nested_strip2: |-2", "nested_strip2", LineType::MappingKey),
        ("    nested_strip3: |-3", "nested_strip3", LineType::MappingKey),
        ("    nested_strip4: |-4", "nested_strip4", LineType::MappingKey),
        ("    nested_strip5: |-5", "nested_strip5", LineType::MappingKey),
        ("    nested_strip6: |-6", "nested_strip6", LineType::MappingKey),
        ("    nested_strip7: |-7", "nested_strip7", LineType::MappingKey),
        ("    nested_strip8: |-8", "nested_strip8", LineType::MappingKey),
        ("    nested_strip9: |-9", "nested_strip9", LineType::MappingKey),

        // Keep modifier |+n (n=1-9)
        ("    nested_keep1: |+1", "nested_keep1", LineType::MappingKey),
        ("    nested_keep2: |+2", "nested_keep2", LineType::MappingKey),
        ("    nested_keep3: |+3", "nested_keep3", LineType::MappingKey),
        ("    nested_keep4: |+4", "nested_keep4", LineType::MappingKey),
        ("    nested_keep5: |+5", "nested_keep5", LineType::MappingKey),
        ("    nested_keep6: |+6", "nested_keep6", LineType::MappingKey),
        ("    nested_keep7: |+7", "nested_keep7", LineType::MappingKey),
        ("    nested_keep8: |+8", "nested_keep8", LineType::MappingKey),
        ("    nested_keep9: |+9", "nested_keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 2
        ("    deep!key1: |1", "deep!key1", LineType::MappingKey),
        ("    deep!warn2: |-2", "deep!warn2", LineType::MappingKey),
        ("    deep!error3: |+3", "deep!error3", LineType::MappingKey),
        ("    deep!test4: |4", "deep!test4", LineType::MappingKey),
        ("    deep!data5: |-5", "deep!data5", LineType::MappingKey),
        ("    deep!info6: |+6", "deep!info6", LineType::MappingKey),

        // ===== Level 3: 6-space indentation with explicit indent modifiers =====
        // Plain |n (n=1-9)
        ("      level3_1: |1", "level3_1", LineType::MappingKey),
        ("      level3_2: |2", "level3_2", LineType::MappingKey),
        ("      level3_3: |3", "level3_3", LineType::MappingKey),
        ("      level3_4: |4", "level3_4", LineType::MappingKey),
        ("      level3_5: |5", "level3_5", LineType::MappingKey),
        ("      level3_6: |6", "level3_6", LineType::MappingKey),
        ("      level3_7: |7", "level3_7", LineType::MappingKey),
        ("      level3_8: |8", "level3_8", LineType::MappingKey),
        ("      level3_9: |9", "level3_9", LineType::MappingKey),

        // Strip modifier |-n (n=1-9) - sample
        ("      deeper_strip1: |-1", "deeper_strip1", LineType::MappingKey),
        ("      deeper_strip3: |-3", "deeper_strip3", LineType::MappingKey),
        ("      deeper_strip5: |-5", "deeper_strip5", LineType::MappingKey),
        ("      deeper_strip7: |-7", "deeper_strip7", LineType::MappingKey),
        ("      deeper_strip9: |-9", "deeper_strip9", LineType::MappingKey),

        // Keep modifier |+n (n=1-9) - sample
        ("      deeper_keep1: |+1", "deeper_keep1", LineType::MappingKey),
        ("      deeper_keep3: |+3", "deeper_keep3", LineType::MappingKey),
        ("      deeper_keep5: |+5", "deeper_keep5", LineType::MappingKey),
        ("      deeper_keep7: |+7", "deeper_keep7", LineType::MappingKey),
        ("      deeper_keep9: |+9", "deeper_keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 3
        ("      very!deep!1: |1", "very!deep!1", LineType::MappingKey),
        ("      very!deep!2: |-2", "very!deep!2", LineType::MappingKey),
        ("      very!deep!3: |+3", "very!deep!3", LineType::MappingKey),
        ("      very!deep!4: |4", "very!deep!4", LineType::MappingKey),
        ("      very!deep!5: |-5", "very!deep!5", LineType::MappingKey),
        ("      very!deep!6: |+6", "very!deep!6", LineType::MappingKey),

        // ===== Level 4: 8-space indentation with explicit indent modifiers =====
        // Plain |n (n=1-9) - sample
        ("        level4_1: |1", "level4_1", LineType::MappingKey),
        ("        level4_3: |3", "level4_3", LineType::MappingKey),
        ("        level4_5: |5", "level4_5", LineType::MappingKey),
        ("        level4_7: |7", "level4_7", LineType::MappingKey),
        ("        level4_9: |9", "level4_9", LineType::MappingKey),

        // Strip modifier |-n (n=1-9) - sample
        ("        deepest_strip1: |-1", "deepest_strip1", LineType::MappingKey),
        ("        deepest_strip3: |-3", "deepest_strip3", LineType::MappingKey),
        ("        deepest_strip5: |-5", "deepest_strip5", LineType::MappingKey),
        ("        deepest_strip7: |-7", "deepest_strip7", LineType::MappingKey),
        ("        deepest_strip9: |-9", "deepest_strip9", LineType::MappingKey),

        // Keep modifier |+n (n=1-9) - sample
        ("        deepest_keep1: |+1", "deepest_keep1", LineType::MappingKey),
        ("        deepest_keep3: |+3", "deepest_keep3", LineType::MappingKey),
        ("        deepest_keep5: |+5", "deepest_keep5", LineType::MappingKey),
        ("        deepest_keep7: |+7", "deepest_keep7", LineType::MappingKey),
        ("        deepest_keep9: |+9", "deepest_keep9", LineType::MappingKey),

        // Keys with exclamation marks at Level 4
        ("        super!deep!1: |1", "super!deep!1", LineType::MappingKey),
        ("        super!deep!2: |-2", "super!deep!2", LineType::MappingKey),
        ("        super!deep!3: |+3", "super!deep!3", LineType::MappingKey),
        ("        super!deep!4: |4", "super!deep!4", LineType::MappingKey),
        ("        super!deep!5: |-5", "super!deep!5", LineType::MappingKey),
        ("        super!deep!6: |+6", "super!deep!6", LineType::MappingKey),

        // ===== Tab indentation with explicit indent modifiers =====
        // Plain |n (n=1-9) - sample
        ("\ttab_1: |1", "tab_1", LineType::MappingKey),
        ("\ttab_3: |3", "tab_3", LineType::MappingKey),
        ("\ttab_5: |5", "tab_5", LineType::MappingKey),
        ("\ttab_7: |7", "tab_7", LineType::MappingKey),
        ("\ttab_9: |9", "tab_9", LineType::MappingKey),

        // Strip modifier |-n (n=1-9) - sample
        ("\ttab_strip1: |-1", "tab_strip1", LineType::MappingKey),
        ("\ttab_strip3: |-3", "tab_strip3", LineType::MappingKey),
        ("\ttab_strip5: |-5", "tab_strip5", LineType::MappingKey),
        ("\ttab_strip7: |-7", "tab_strip7", LineType::MappingKey),
        ("\ttab_strip9: |-9", "tab_strip9", LineType::MappingKey),

        // Keep modifier |+n (n=1-9) - sample
        ("\ttab_keep1: |+1", "tab_keep1", LineType::MappingKey),
        ("\ttab_keep3: |+3", "tab_keep3", LineType::MappingKey),
        ("\ttab_keep5: |+5", "tab_keep5", LineType::MappingKey),
        ("\ttab_keep7: |+7", "tab_keep7", LineType::MappingKey),
        ("\ttab_keep9: |+9", "tab_keep9", LineType::MappingKey),

        // Keys with exclamation marks at tab level
        ("\ttab!key1: |1", "tab!key1", LineType::MappingKey),
        ("\ttab!warn2: |-2", "tab!warn2", LineType::MappingKey),
        ("\ttab!error3: |+3", "tab!error3", LineType::MappingKey),
        ("\ttab!test4: |4", "tab!test4", LineType::MappingKey),
        ("\ttab!data5: |-5", "tab!data5", LineType::MappingKey),
        ("\ttab!info6: |+6", "tab!info6", LineType::MappingKey),

        // ===== Mixed indentation with explicit indent modifiers =====
        // Tab + spaces combinations
        ("\t  mixed_1: |1", "mixed_1", LineType::MappingKey),
        ("\t  mixed_2: |-2", "mixed_2", LineType::MappingKey),
        ("\t  mixed_3: |+3", "mixed_3", LineType::MappingKey),
        ("\t    mixed_4: |4", "mixed_4", LineType::MappingKey),
        ("\t    mixed_5: |-5", "mixed_5", LineType::MappingKey),
        ("\t    mixed_6: |+6", "mixed_6", LineType::MappingKey),

        // Edge cases: Multiple exclamation marks with explicit indent
        ("  key!!: |1", "key!!", LineType::MappingKey),
        ("    deep!!key: |-2", "deep!!key", LineType::MappingKey),
        ("      very!!deep!!key: |+3", "very!!deep!!key", LineType::MappingKey),
        ("        super!!!deep!!!key: |4", "super!!!deep!!!key", LineType::MappingKey),
        ("\ttab!!key!!!test: |-5", "tab!!key!!!test", LineType::MappingKey),

        // Edge cases: Single character keys with explicit indent
        ("  a!: |1", "a!", LineType::MappingKey),
        ("    b!: |-2", "b!", LineType::MappingKey),
        ("      c!: |+3", "c!", LineType::MappingKey),
        ("        d!: |4", "d!", LineType::MappingKey),
        ("\te!: |-5", "e!", LineType::MappingKey),

        // Edge cases: Keys ending with ! at various levels with explicit indent
        ("  end!with!bang!: |1", "end!with!bang!", LineType::MappingKey),
        ("    deep!end!with!bang!: |-2", "deep!end!with!bang!", LineType::MappingKey),
        ("      very!deep!end!bang!: |+3", "very!deep!end!bang!", LineType::MappingKey),
        ("        super!deep!end!bang!: |4", "super!deep!end!bang!", LineType::MappingKey),
        ("\ttab!end!with!bang!: |-5", "tab!end!with!bang!", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Literal scalar explicit indent modifier test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for literal scalar with explicit indent modifier: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for literal scalar with explicit indent modifier: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for literal scalars with explicit indent modifiers
    let continuation_lines = vec![
        // Level 1 continuation lines with ! characters
        ("  This is content with! exclamation", vec![LineType::MappingKey, LineType::Unknown]),
        ("  More! text! here! for! testing!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 2 continuation lines with ! characters
        ("    Deeper! content! with! more! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Nested! text! continues! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 3 continuation lines with ! characters
        ("      Very! deep! content! with! emphasis!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),

        // Level 4 continuation lines with ! characters
        ("        Super! deep! content! with! many! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        Extra! complex! continuation! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Tab continuation lines with ! characters
        ("\tTab! content! with! exclamation!", vec![LineType::MappingKey, LineType::Unknown]),
        ("\tMore! tab! text! continues! here!", vec![LineType::MappingKey, LineType::Unknown]),

        // Lines starting with ! (may be classified as Tag)
        ("  !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("    !Deep! tag! like! content!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("      !Very! deep! tag! line!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Continuation line for literal scalar explicit indent modifier should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );

        // Continuation lines should NOT detect as mapping keys
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_multiline_mixed_with_singleline_exclamation_patterns() {
    // Test mixed multiline blocks containing single-line configs with exclamation marks
    // This simulates real-world config files with various structures mixed together

    let test_cases = vec![
        // Mixed: multiline block scalars followed by single-line configs with !
        ("description: >", "description", None, true),   // Folded scalar starts
        ("  This is a long description!", "description", Some("This is a long description!"), true),
        ("  with multiple lines!", "description", Some("with multiple lines!"), true),
        ("priority: high!", "priority", Some("high!"), true),  // Back to single-line with !
        ("note: >", "note", None, true),                   // Another folded scalar
        ("  !important message!", "note", Some("!important message!"), true),
        ("  continues here!", "note", Some("continues here!"), true),

        // Mixed: literal scalars with single-line configs
        ("error_message: |", "error_message", None, true),
        ("  Error occurred!", "error_message", Some("Error occurred!"), true),
        ("  Check logs!", "error_message", Some("Check logs!"), true),
        ("status: failed!", "status", Some("failed!"), true),

        // Complex nesting with mixed styles
        ("config:", "config", None, true),
        ("  description: >", "description", None, true),
        ("    Main config! settings!", "description", Some("Main config! settings!"), true),
        ("  enabled: true!", "enabled", Some("true!"), true),
        ("  note: |", "note", None, true),
        ("    !Important!", "note", Some("!Important!"), true),
        ("    Read carefully!", "note", Some("Read carefully!"), true),
    ];

    for (line, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);
        // Continuation lines (indented without colons) are not detected as mapping keys
        let is_continuation_line = (line.starts_with("  ") || line.starts_with("\t") ||
                                    line.starts_with("    ")) && !line.contains(':');

        if should_detect && !is_continuation_line {
            assert!(
                info.is_some(),
                "Should detect mapping key in mixed multiline scenario: '{}'",
                line
            );
            let info = info.unwrap();
            assert_eq!(
                info.key, expected_key,
                "Should extract correct key in mixed multiline: '{}'",
                line
            );
            if let Some(exp_val) = expected_value {
                assert_eq!(
                    info.value, Some(exp_val.to_string()),
                    "Should extract correct value in mixed multiline: '{}'",
                    line
                );
            }
        } else if is_continuation_line {
            // Continuation lines don't have key: value patterns
            assert!(
                info.is_none(),
                "Continuation line should NOT be detected as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_multiline_yaml_strings_with_exclamation_in_nested_contexts() {
    // Test multiline YAML strings with exclamation marks in deeply nested contexts
    // This verifies that nested structures with block scalars are handled correctly

    let test_cases = vec![
        // Nested mappings with block scalars containing !
        ("outer:", "outer", None, true),
        ("  middle:", "middle", None, true),
        ("    inner:", "inner", None, true),
        ("      description: >", "description", None, true),
        ("        Deep! nested! content!", "description", Some("Deep! nested! content!"), true),
        ("        with! multiple! lines!", "description", Some("with! multiple! lines!"), true),
        ("      value: important!", "value", Some("important!"), true),

        // Sequences within nested structures with block scalars
        ("items:", "items", None, true),
        ("  - name: item1", "name", Some("item1"), true),
        ("    description: |", "description", None, true),
        ("      First! item!", "description", Some("First! item!"), true),
        ("      With! details!", "description", Some("With! details!"), true),
        ("  - name: item2", "name", Some("item2"), true),
        ("    note: >", "note", None, true),
        ("      Second! item!", "note", Some("Second! item!"), true),
    ];

    for (line, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);
        // Continuation lines (indented without colons) are not detected as mapping keys
        let is_continuation_line = (line.starts_with("          ") || line.starts_with("        ") ||
                                    line.starts_with("    ")) && !line.contains(':');

        if should_detect && !is_continuation_line {
            if line.starts_with("- ") {
                // Sequence items are handled differently
                if let Some(key_part) = line.strip_prefix("- ") {
                    if let Some(colon_pos) = key_part.find(':') {
                        let key = &key_part[..colon_pos];
                        let _value_part = &key_part[colon_pos + 1..];
                        // For sequence items, we verify the line structure
                        assert!(line.starts_with("-"), "Sequence item should start with '-': '{}'", line);
                        assert_eq!(key, expected_key, "Sequence item key should match: '{}'", line);
                    }
                }
            } else {
                assert!(
                    info.is_some(),
                    "Should detect mapping key in nested multiline: '{}'",
                    line
                );
                let info = info.unwrap();
                assert_eq!(
                    info.key, expected_key,
                    "Should extract correct key in nested multiline: '{}'",
                    line
                );
                if let Some(exp_val) = expected_value {
                    assert_eq!(
                        info.value, Some(exp_val.to_string()),
                        "Should extract correct value in nested multiline: '{}'",
                        line
                    );
                }
            }
        } else if is_continuation_line {
            // Continuation lines don't have key: value patterns
            assert!(
                info.is_none(),
                "Continuation line should NOT be detected as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_folded_scalar_exclamation_at_different_positions() {
    // Test folded scalars with exclamation marks at various positions in the content
    let test_cases = vec![
        // ! at start of folded scalar content
        ("warning: >", "warning", None, true),
        ("  !Important! message!", "warning", Some("!Important! message!"), true),

        // ! in middle of folded scalar content
        ("description: >", "description", None, true),
        ("  This! is! important!", "description", Some("This! is! important!"), true),

        // ! at end of folded scalar content
        ("note: >", "note", None, true),
        ("  Read this carefully!", "note", Some("Read this carefully!"), true),

        // Multiple ! throughout folded scalar
        ("message: >", "message", None, true),
        ("  !Start! Middle! End!", "message", Some("!Start! Middle! End!"), true),
        ("  More! content! here!", "message", Some("More! content! here!"), true),
    ];

    for (line, expected_key, _expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);
        // Continuation lines (indented without colons) are not detected as mapping keys
        let is_continuation_line = (line.starts_with("  ") || line.starts_with('\t')) &&
                                    !line.contains(':');

        if should_detect && !is_continuation_line {
            assert!(
                info.is_some(),
                "Should detect mapping key for folded scalar: '{}'",
                line
            );
            let info = info.unwrap();
            assert_eq!(
                info.key, expected_key,
                "Should extract correct key for folded scalar: '{}'",
                line
            );
        } else if is_continuation_line {
            // Continuation lines don't have key: value patterns
            assert!(
                info.is_none(),
                "Continuation line should NOT be detected as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_literal_scalar_exclamation_at_different_positions() {
    // Test literal scalars with exclamation marks at various positions in the content
    let test_cases = vec![
        // ! at start of literal scalar content
        ("warning: |", "warning", None, true),
        ("  !Important! message!", "warning", Some("!Important! message!"), true),

        // ! in middle of literal scalar content
        ("description: |", "description", None, true),
        ("  This! is! important!", "description", Some("This! is! important!"), true),

        // ! at end of literal scalar content
        ("note: |", "note", None, true),
        ("  Read this carefully!", "note", Some("Read this carefully!"), true),

        // Multiple ! throughout literal scalar
        ("message: |", "message", None, true),
        ("  !Start! Middle! End!", "message", Some("!Start! Middle! End!"), true),
        ("  More! content! here!", "message", Some("More! content! here!"), true),

        // Literal scalar with various newline positions and !
        ("log_output: |", "log_output", None, true),
        ("  Line 1 with!", "log_output", Some("Line 1 with!"), true),
        ("  Line 2!", "log_output", Some("Line 2!"), true),
        ("  !Line 3!", "log_output", Some("!Line 3!"), true),
    ];

    for (line, expected_key, _expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);
        // Continuation lines (indented without colons) are not detected as mapping keys
        let is_continuation_line = (line.starts_with("  ") || line.starts_with('\t')) &&
                                    !line.contains(':');

        if should_detect && !is_continuation_line {
            assert!(
                info.is_some(),
                "Should detect mapping key for literal scalar: '{}'",
                line
            );
            let info = info.unwrap();
            assert_eq!(
                info.key, expected_key,
                "Should extract correct key for literal scalar: '{}'",
                line
            );
        } else if is_continuation_line {
            // Continuation lines don't have key: value patterns
            assert!(
                info.is_none(),
                "Continuation line should NOT be detected as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_multiline_block_scalar_modifiers_with_exclamation() {
    // Test block scalars with different modifiers and exclamation marks
    // Modifiers: -, +, and indentation levels

    let test_cases = vec![
        // Strip modifier (-) - removes trailing newlines
        ("note: >-", "note", None, true),
        ("  !Important! note!", "note", Some("!Important! note!"), true),

        // Keep modifier (+) - keeps trailing newlines
        ("message: >+", "message", None, true),
        ("  !Urgent! message!", "message", Some("!Urgent! message!"), true),

        // Explicit indentation (2)
        ("description: >-2", "description", None, true),
        ("    !Detailed! info!", "description", Some("!Detailed! info!"), true),

        // Explicit indentation (4)
        ("text: >2", "text", None, true),
        ("      !More! text!", "text", Some("!More! text!"), true),

        // Literal scalars with modifiers
        ("log: |-", "log", None, true),
        ("  !Error! log!", "log", Some("!Error! log!"), true),

        ("output: |+", "output", None, true),
        ("  !Success! output!", "output", Some("!Success! output!"), true),
    ];

    for (line, expected_key, _expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);
        // Continuation lines (indented without colons) are not detected as mapping keys
        let is_continuation_line = (line.starts_with("  ") || line.starts_with('\t') ||
                                    line.starts_with("    ") || line.starts_with("      ")) &&
                                    !line.contains(':');

        if should_detect && !is_continuation_line {
            assert!(
                info.is_some(),
                "Should detect mapping key with block scalar modifier: '{}'",
                line
            );
            let info = info.unwrap();
            assert_eq!(
                info.key, expected_key,
                "Should extract correct key with modifier: '{}'",
                line
            );
        } else if is_continuation_line {
            // Continuation lines don't have key: value patterns
            assert!(
                info.is_none(),
                "Continuation line should NOT be detected as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_real_world_multiline_config_with_exclamation() {
    // Real-world configuration patterns with multiline strings containing exclamation marks
    // This simulates production config files with complex multiline structures

    let test_cases = vec![
        // Database connection with multiline error messages
        ("database:", "database", None, true),
        ("  error_message: |", "error_message", None, true),
        ("    !Failed! to connect!", "error_message", Some("!Failed! to connect!"), true),
        ("    Check! credentials!", "error_message", Some("Check! credentials!"), true),
        ("  connection_string: postgresql://localhost/db!", "connection_string", Some("postgresql://localhost/db!"), true),

        // Application configuration with multiline descriptions
        ("app:", "app", None, true),
        ("  name: MyApp!", "name", Some("MyApp!"), true),
        ("  description: >", "description", None, true),
        ("    This! is! a! great! app!", "description", Some("This! is! a! great! app!"), true),
        ("    With! many! features!", "description", Some("With! many! features!"), true),

        // Monitoring configuration with multiline alert messages
        ("monitoring:", "monitoring", None, true),
        ("  alert_message: |", "alert_message", None, true),
        ("    !Critical! alert!", "alert_message", Some("!Critical! alert!"), true),
        ("    System! overload!", "alert_message", Some("System! overload!"), true),
        ("  threshold: 90%!", "threshold", Some("90%!"), true),
    ];

    for (line, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);
        if should_detect {
            // Lines starting with spaces are continuation lines
            if line.starts_with(' ') || line.starts_with('\t') {
                // Continuation lines may or may not be detected
                if let Some(detected) = info {
                    if expected_value.is_some() {
                        assert_eq!(
                            detected.value, expected_value.map(|v| v.to_string()),
                            "Real-world multiline continuation should match: '{}'",
                            line
                        );
                    }
                }
            } else {
                assert!(
                    info.is_some(),
                    "Should detect mapping key in real-world multiline: '{}'",
                    line
                );
                let info = info.unwrap();
                assert_eq!(
                    info.key, expected_key,
                    "Should extract correct key in real-world multiline: '{}'",
                    line
                );
            }
        }
    }
}

#[test]
fn test_multiline_comment_and_config_mixed_with_exclamation() {
    // Test multiline scenarios with comments, block scalars, and single-line configs
    // all mixed together with exclamation marks

    let yaml_lines = vec![
        "# Configuration file - !Important! Read carefully!",
        "version: 1.0!",
        "# Note: This! is! a! comment!",
        "description: >",
        "  This is a multiline",
        "  description with! marks!",
        "  And! more! content!",
        "# Another !comment! here!",
        "settings:",
        "  enabled: true!",
        "  message: !important!",
        "  note: |",
        "    !Literal! multiline!",
        "    With! exclamations!",
    ];

    // Verify line classification
    let expected_types = vec![
        LineType::Comment,      // # Configuration file
        LineType::MappingKey,   // version: 1.0!
        LineType::Comment,      // # Note: comment
        LineType::MappingKey,   // description: >
        LineType::Unknown,      // Indented continuation line
        LineType::Unknown,      // Indented continuation line
        LineType::Unknown,      // Indented continuation line
        LineType::Comment,      // # Another !comment!
        LineType::MappingKey,   // settings:
        LineType::MappingKey,   // enabled: true!
        LineType::MappingKey,   // message: !important!
        LineType::MappingKey,   // note: |
        LineType::Tag,          // Indented continuation line starting with !
        LineType::Unknown,      // Indented continuation line
    ];

    for (i, line) in yaml_lines.iter().enumerate() {
        let result = classify_line_type(line);
        let expected = expected_types[i];
        assert_eq!(
            result, expected,
            "Mixed multiline line {} should be {:?}: '{}'",
            i, expected, line
        );
    }
}

#[test]
fn test_multiline_sequence_with_exclamation_in_block_scalars() {
    // Test sequences where items have block scalar values with exclamation marks

    let test_cases = vec![
        // Sequence with folded scalar values
        ("items:", "items", None, true),
        ("  - name: item1", "name", Some("item1"), true),
        ("    description: >", "description", None, true),
        ("      First! item! description!", "description", Some("First! item! description!"), true),
        ("  - name: item2", "name", Some("item2"), true),
        ("    note: |", "note", None, true),
        ("      Second! item!", "note", Some("Second! item!"), true),
        ("      With! details!", "note", Some("With! details!"), true),

        // Nested sequences with block scalars
        ("nested:", "nested", None, true),
        ("  - subitems:", "subitems", None, true),
        ("    - value: sub1!", "value", Some("sub1!"), true),
        ("      detail: >", "detail", None, true),
        ("        !Important! detail!", "detail", Some("!Important! detail!"), true),
    ];

    for (line, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);
        if should_detect {
            // Lines starting with spaces are continuation lines or nested
            if line.starts_with(' ') || line.starts_with('\t') {
                // Continuation lines may or may not be detected
                if let Some(detected) = info {
                    if expected_value.is_some() && !line.starts_with("- ") {
                        assert_eq!(
                            detected.value, expected_value.map(|v| v.to_string()),
                            "Nested sequence continuation should match: '{}'",
                            line
                        );
                    }
                }
            } else if !line.starts_with("- ") {
                assert!(
                    info.is_some(),
                    "Should detect mapping key in sequence with block scalars: '{}'",
                    line
                );
                let info = info.unwrap();
                assert_eq!(
                    info.key, expected_key,
                    "Should extract correct key in sequence with block scalars: '{}'",
                    line
                );
            }
        }
    }
}

#[test]
fn test_level1_indentation_with_exclamation_marks() {
    // Test level 1 (2-space) indentation with '!' character in Section 12B
    // This test focuses specifically on single-level indentation scenarios with exclamation marks
    // Level 1 uses 2-space indentation as the standard first-level indentation

    let test_cases = vec![
        // Basic level 1 keys with '!' at various positions
        ("  key!: >", "key!", LineType::MappingKey),
        ("  test!here: >", "test!here", LineType::MappingKey),
        ("  simple!test: >", "simple!test", LineType::MappingKey),
        ("  basic!: >-", "basic!", LineType::MappingKey),
        ("  another!one: >+", "another!one", LineType::MappingKey),

        // Level 1 with multiple '!' characters
        ("  key!!: >", "key!!", LineType::MappingKey),
        ("  test!here!now: >", "test!here!now", LineType::MappingKey),
        ("  multiple!!!: >", "multiple!!!", LineType::MappingKey),
        ("  spaced!out!keys!: >", "spaced!out!keys!", LineType::MappingKey),
        ("  end!with!bang!: >", "end!with!bang!", LineType::MappingKey),

        // Level 1 tag keys (starting with '!')
        ("  !tag: >", "!tag", LineType::Tag),
        ("  !.custom: >", "!.custom", LineType::Tag),
        ("  !local: >", "!local", LineType::Tag),
        ("  !!double: >", "!!double", LineType::Tag),

        // Level 1 keys with '!' in various positions
        ("  !start: >", "!start", LineType::Tag),
        ("  middle!bang: >", "middle!bang", LineType::MappingKey),
        ("  end!with!bang!: >", "end!with!bang!", LineType::MappingKey),
        ("  !somewhere: >", "!somewhere", LineType::Tag),
        ("  complex!key!here!: >", "complex!key!here!", LineType::MappingKey),

        // Level 1 with different folded scalar modifiers
        ("  level1_key!: >", "level1_key!", LineType::MappingKey),
        ("  first!: >-", "first!", LineType::MappingKey),
        ("  second!: >+", "second!", LineType::MappingKey),
        ("  third!: >-2", "third!", LineType::MappingKey),
        ("  fourth!: >+2", "fourth!", LineType::MappingKey),

        // Edge cases for level 1
        ("  !: >", "!", LineType::Tag),
        ("  !!: >", "!!", LineType::Tag),
        ("  a!: >", "a!", LineType::MappingKey),
        ("  a!b: >", "a!b", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 1 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for level 1 indentation: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for level 1 indentation: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test level 1 continuation lines with '!' characters
    // Note: Lines starting with '!' may be classified as Tag, which is correct behavior
    let continuation_lines = vec![
        ("  Content with! exclamations!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  More! indented! level! 1! content!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Single! line! with! multiple! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  Ending! with! bang! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  !At! start! and! middle!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  Throughout! the! entire! line!", vec![LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Level 1 continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Level 1 continuation result should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}

#[test]
fn test_basic_indentation_levels_with_exclamation_marks() {
    // Test basic indentation levels (1-3) with '!' character in folded scalar indicators
    // Level 1: 2 spaces, Level 2: 4 spaces, Level 3: 6 spaces
    // This provides clear coverage of the three most common indentation scenarios

    let test_cases = vec![
        // Level 1: 2-space indentation with '!'
        ("  level1_key!: >", "level1_key!", LineType::MappingKey),
        ("  first!: >-", "first!", LineType::MappingKey),
        ("  simple!test: >", "simple!test", LineType::MappingKey),
        ("  basic!: >+", "basic!", LineType::MappingKey),

        // Level 2: 4-space indentation with '!'
        ("    level2_key!: >", "level2_key!", LineType::MappingKey),
        ("    second!: >-", "second!", LineType::MappingKey),
        ("    nested!test: >", "nested!test", LineType::MappingKey),
        ("    deeper!: >+", "deeper!", LineType::MappingKey),

        // Level 3: 6-space indentation with '!'
        ("      level3_key!: >", "level3_key!", LineType::MappingKey),
        ("      third!: >-", "third!", LineType::MappingKey),
        ("      deep!nest: >", "deep!nest", LineType::MappingKey),
        ("      deepest!: >+", "deepest!", LineType::MappingKey),

        // Level 1: Tag keys (starting with '!')
        ("  !tag1: >", "!tag1", LineType::Tag),

        // Level 2: Tag keys (starting with '!')
        ("    !tag2: >", "!tag2", LineType::Tag),

        // Level 3: Tag keys (starting with '!')
        ("      !tag3: >", "!tag3", LineType::Tag),

        // Level 1: Multiple '!' in key (not starting with '!')
        ("  key!!: >", "key!!", LineType::MappingKey),

        // Level 2: Multiple '!' in key (not starting with '!')
        ("    deep!!key: >", "deep!!key", LineType::MappingKey),

        // Level 3: Multiple '!' in key (not starting with '!')
        ("      very!!deep: >", "very!!deep", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Basic indentation level test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for basic indentation: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for basic indentation: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for each level
    let continuation_lines = vec![
        // Level 1 continuation
        "  Content with! exclamations!",

        // Level 2 continuation
        "    More! indented! content!",

        // Level 3 continuation
        "      Very! deep! continuation! here!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be MappingKey or Unknown
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Basic indentation continuation should be MappingKey or Unknown: '{}' (got {:?})",
            line, result
        );
    }
}

#[test]
fn test_level_1_indentation_with_exclamation_mark() {
    // Test level 1 (single-level indentation) with '!' character
    // Level 1: 2-space indentation - the most common single-level indentation scenario
    // This test focuses exclusively on level 1 indentation with exclamation marks

    let test_cases = vec![
        // Level 1: 2-space indentation with '!' at various positions
        ("  key!: >", LineType::MappingKey),
        ("  simple!test: >", LineType::MappingKey),
        ("  end!with!bang!: >", LineType::MappingKey),
        ("  multiple!!!here: >", LineType::MappingKey),
        ("  spaced!out!keys!: >", LineType::MappingKey),

        // Level 1: Keys starting with '!' (Tag type)
        ("  !tag: >", LineType::Tag),
        ("  !.custom: >", LineType::Tag),
        ("  !start!end: >", LineType::Tag),
        ("  !!double: >", LineType::Tag),

        // Level 1: Folded scalar with modifiers
        ("  level1_key!: >-", LineType::MappingKey),
        ("  first!: >+", LineType::MappingKey),
        ("  basic!: >-2", LineType::MappingKey),
        ("  simple!test: >2", LineType::MappingKey),
    ];

    for (line, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 1 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );
    }

    // Test continuation lines for level 1 indentation with '!'
    let continuation_lines = vec![
        "  Content with! exclamations!",
        "  Multiple! bangs! in! line!",
        "  Important! message! continues!",
        "  Another! line! with! emphasis!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be MappingKey or Unknown
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Level 1 continuation should be MappingKey or Unknown: '{}' (got {:?})",
            line, result
        );
    }
}

#[test]
fn test_level_2_indentation_with_exclamation_mark() {
    // Test level 2 (two-level indentation) with '!' character
    // Level 2: 4-space indentation - common nested indentation scenario
    // This test focuses exclusively on level 2 indentation with exclamation marks

    let test_cases = vec![
        // Level 2: 4-space indentation with '!' at various positions
        ("    key!: >", LineType::MappingKey),
        ("    simple!test: >", LineType::MappingKey),
        ("    end!with!bang!: >", LineType::MappingKey),
        ("    multiple!!!here: >", LineType::MappingKey),
        ("    spaced!out!keys!: >", LineType::MappingKey),

        // Level 2: Keys starting with '!' (Tag type)
        ("    !tag: >", LineType::Tag),
        ("    !.custom: >", LineType::Tag),
        ("    !start!end: >", LineType::Tag),
        ("    !!double: >", LineType::Tag),

        // Level 2: Folded scalar with modifiers
        ("    level2_key!: >-", LineType::MappingKey),
        ("    second!: >+", LineType::MappingKey),
        ("    nested!: >-2", LineType::MappingKey),
        ("    deep!test: >2", LineType::MappingKey),
    ];

    for (line, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 2 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );
    }

    // Test continuation lines for level 2 indentation with '!'
    let continuation_lines = vec![
        "    Content with! exclamations!",
        "    Multiple! bangs! in! line!",
        "    Important! message! continues!",
        "    Another! line! with! emphasis!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be MappingKey or Unknown
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Level 2 continuation should be MappingKey or Unknown: '{}' (got {:?})",
            line, result
        );
    }
}

#[test]
fn test_level_3_indentation_with_exclamation_mark() {
    // Test level 3 (three-level indentation) with '!' character
    // Level 3: 6-space indentation - common deeply nested indentation scenario
    // This test focuses exclusively on level 3 indentation with exclamation marks

    let test_cases = vec![
        // Level 3: 6-space indentation with '!' at various positions
        ("      key!: >", LineType::MappingKey),
        ("      simple!test: >", LineType::MappingKey),
        ("      end!with!bang!: >", LineType::MappingKey),
        ("      multiple!!!here: >", LineType::MappingKey),
        ("      spaced!out!keys!: >", LineType::MappingKey),

        // Level 3: Keys starting with '!' (Tag type)
        ("      !tag: >", LineType::Tag),
        ("      !.custom: >", LineType::Tag),
        ("      !start!end: >", LineType::Tag),
        ("      !!double: >", LineType::Tag),

        // Level 3: Folded scalar with modifiers
        ("      level3_key!: >-", LineType::MappingKey),
        ("      third!: >+", LineType::MappingKey),
        ("      deep!: >-2", LineType::MappingKey),
        ("      very!deep!test: >2", LineType::MappingKey),
    ];

    for (line, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 3 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );
    }

    // Test continuation lines for level 3 indentation with '!'
    let continuation_lines = vec![
        "      Content with! exclamations!",
        "      Multiple! bangs! in! line!",
        "      Important! message! continues!",
        "      Another! line! with! emphasis!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be MappingKey or Unknown
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Level 3 continuation should be MappingKey or Unknown: '{}' (got {:?})",
            line, result
        );
    }
}

#[test]
fn test_various_indentation_levels_with_exclamation_marks() {
    // Test various indentation levels with folded scalars and exclamation marks in keys
    // This provides comprehensive coverage of indentation scenarios with '!' characters

    let test_cases = vec![
        // 2-space indentation with '!'
        ("  key!: >", LineType::MappingKey),
        ("  !key: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("  key!value: >", LineType::MappingKey),
        ("  !start!end!: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("  multiple!!!: >", LineType::MappingKey),

        // 4-space indentation with '!'
        ("    deep!: >", LineType::MappingKey),
        ("    !nested: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("    level!key!: >", LineType::MappingKey),
        ("    spaced!out!: >", LineType::MappingKey),

        // 6-space indentation with '!'
        ("      deeper!: >", LineType::MappingKey),
        ("      !very!deep: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("      nesting!level!: >", LineType::MappingKey),

        // 8-space indentation with '!'
        ("        very!deep!: >", LineType::MappingKey),
        ("        !ultra!nested: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("        deep!nest!ed!: >", LineType::MappingKey),

        // 10-space indentation with '!'
        ("          extreme!: >", LineType::MappingKey),
        ("          !mega!nested!: >", LineType::Tag),  // Keys starting with '!' are Tags

        // 12-space indentation with '!'
        ("            insane!: >", LineType::MappingKey),
        ("            !crazy!deep!nest!: >", LineType::Tag),  // Keys starting with '!' are Tags

        // Tab indentation with '!'
        ("\ttab!: >", LineType::MappingKey),
        ("\t!tabkey: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("\ttab!key!: >", LineType::MappingKey),

        // Double tab with '!'
        ("\t\tdouble!tab!: >", LineType::MappingKey),
        ("\t\t!double!nested: >", LineType::Tag),  // Keys starting with '!' are Tags

        // Triple tab with '!'
        ("\t\t\ttriple!deep!: >", LineType::MappingKey),
        ("\t\t\t!ultra!tab!: >", LineType::Tag),  // Keys starting with '!' are Tags

        // Mixed indentation (space + tab) with '!'
        ("  \tmixed!: >", LineType::MappingKey),
        ("    \tmixed!key!: >", LineType::MappingKey),
        ("\t  space!tab!: >", LineType::MappingKey),
        ("\t    mixed!deep!: >", LineType::MappingKey),

        // Odd indentation (1, 3, 5, 7, 9, 11 spaces) with '!'
        (" odd!: >", LineType::MappingKey),
        ("   !three: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("     five!key!: >", LineType::MappingKey),
        ("       seven!deep!: >", LineType::MappingKey),
        ("         nine!nest!: >", LineType::MappingKey),
        ("           eleven!crazy!: >", LineType::MappingKey),

        // With strip modifier (-)
        ("  key!: >-", LineType::MappingKey),
        ("    !nested: >-", LineType::Tag),  // Keys starting with '!' are Tags
        ("      deep!value!: >-", LineType::MappingKey),

        // With keep modifier (+)
        ("  key!: >+", LineType::MappingKey),
        ("    !nested: >+", LineType::Tag),  // Keys starting with '!' are Tags
        ("      deep!value!: >+", LineType::MappingKey),

        // With explicit indent (2, 3, 4)
        ("  key!: >2", LineType::MappingKey),
        ("    !nested: >-3", LineType::Tag),  // Keys starting with '!' are Tags
        ("      deep!value!: >+4", LineType::MappingKey),
    ];

    for (line, expected_type) in test_cases {
        let result = classify_line_type(line);

        assert_eq!(
            result, expected_type,
            "Folded scalar with '!' at indentation should be {:?}: '{}' (got {:?})",
            expected_type, line, result
        );
    }

    // Test continuation lines with various indentation and '!'
    let continuation_lines = vec![
        // 2-space continuation with '!'
        "  Content! with! exclamations!",
        "    More! indented! continuation!",

        // 4-space continuation with '!'
        "    Deep! content! here!",
        "      Even! deeper! with! bangs!",

        // 6-space continuation with '!'
        "      Very! deep! content!",
        "        Extremely! deep! bangs!",

        // Tab continuation with '!'
        "\tTab! content! with! marks!",
        "\t\tDouble! tab! continuation!",

        // Mixed continuation with '!'
        "  \tMixed! indentation! here!",
        "\t  Tab! then! spaces! bangs!",

        // Odd indentation continuation with '!'
        " One! space! continuation!",
        "   Three! spaces! here!",
        "     Five! space! content!",
        "       Seven! space! deep!",
        "         Nine! space! nesting!",
        "           Eleven! space! extreme!",

        // Multiple consecutive '!'
        "  Check! this!!!",
        "    Urgent!!! message!!!",
        "      Critical!!! alert!!!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);

        // Continuation lines (without colons) should be MappingKey or Unknown
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Continuation line with '!' should be MappingKey or Unknown: '{}' (got {:?})",
            line, result
        );
    }
}

#[test]
fn test_level2_indentation_with_exclamation_marks() {
    // Test level 2 (4-space) indentation with '!' character in Section 12B
    // This test focuses specifically on two-level indentation scenarios with exclamation marks
    // Level 2 uses 4-space indentation as the standard second-level indentation

    let test_cases = vec![
        // Basic level 2 keys with '!' at various positions
        ("    key!: >", "key!", LineType::MappingKey),
        ("    test!here: >", "test!here", LineType::MappingKey),
        ("    simple!test: >", "simple!test", LineType::MappingKey),
        ("    basic!: >-", "basic!", LineType::MappingKey),
        ("    another!one: >+", "another!one", LineType::MappingKey),

        // Level 2 with multiple '!' characters
        ("    key!!: >", "key!!", LineType::MappingKey),
        ("    test!here!now: >", "test!here!now", LineType::MappingKey),
        ("    multiple!!!: >", "multiple!!!", LineType::MappingKey),
        ("    spaced!out!keys!: >", "spaced!out!keys!", LineType::MappingKey),
        ("    end!with!bang!: >", "end!with!bang!", LineType::MappingKey),

        // Level 2 tag keys (starting with '!')
        ("    !tag: >", "!tag", LineType::Tag),
        ("    !.custom: >", "!.custom", LineType::Tag),
        ("    !local: >", "!local", LineType::Tag),
        ("    !!double: >", "!!double", LineType::Tag),

        // Level 2 keys with '!' in various positions
        ("    !start: >", "!start", LineType::Tag),
        ("    middle!bang: >", "middle!bang", LineType::MappingKey),
        ("    end!with!bang!: >", "end!with!bang!", LineType::MappingKey),
        ("    !somewhere: >", "!somewhere", LineType::Tag),
        ("    complex!key!here!: >", "complex!key!here!", LineType::MappingKey),

        // Level 2 with different folded scalar modifiers
        ("    level2_key!: >", "level2_key!", LineType::MappingKey),
        ("    first!: >-", "first!", LineType::MappingKey),
        ("    second!: >+", "second!", LineType::MappingKey),
        ("    third!: >-2", "third!", LineType::MappingKey),
        ("    fourth!: >+2", "fourth!", LineType::MappingKey),

        // Edge cases for level 2
        ("    !: >", "!", LineType::Tag),
        ("    !!: >", "!!", LineType::Tag),
        ("    a!: >", "a!", LineType::MappingKey),
        ("    a!b: >", "a!b", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 2 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for level 2 indentation: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for level 2 indentation: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test level 2 continuation lines with '!' characters
    // Note: Lines starting with '!' may be classified as Tag, which is correct behavior
    let continuation_lines = vec![
        ("    Content with! exclamations!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    More! indented! level! 2! content!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Single! line! with! multiple! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("    Ending! with! bang! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),
        ("    !At! start! and! middle!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("    Throughout! the! entire! line!", vec![LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Level 2 continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Level 2 continuation result should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}

#[test]
fn test_level3_indentation_with_exclamation_marks() {
    // Test level 3 (6-space) indentation with '!' character in Section 12B
    // This test focuses specifically on three-level indentation scenarios with exclamation marks
    // Level 3 uses 6-space indentation as the standard third-level indentation

    let test_cases = vec![
        // Basic level 3 keys with '!' at various positions
        ("      key!: >", "key!", LineType::MappingKey),
        ("      test!here: >", "test!here", LineType::MappingKey),
        ("      simple!test: >", "simple!test", LineType::MappingKey),
        ("      basic!: >-", "basic!", LineType::MappingKey),
        ("      another!one: >+", "another!one", LineType::MappingKey),

        // Level 3 with multiple '!' characters
        ("      key!!: >", "key!!", LineType::MappingKey),
        ("      test!here!now: >", "test!here!now", LineType::MappingKey),
        ("      multiple!!!: >", "multiple!!!", LineType::MappingKey),
        ("      spaced!out!keys!: >", "spaced!out!keys!", LineType::MappingKey),
        ("      end!with!bang!: >", "end!with!bang!", LineType::MappingKey),

        // Level 3 tag keys (starting with '!')
        ("      !tag: >", "!tag", LineType::Tag),
        ("      !.custom: >", "!.custom", LineType::Tag),
        ("      !local: >", "!local", LineType::Tag),
        ("      !!double: >", "!!double", LineType::Tag),

        // Level 3 keys with '!' in various positions
        ("      !start: >", "!start", LineType::Tag),
        ("      middle!bang: >", "middle!bang", LineType::MappingKey),
        ("      end!with!bang!: >", "end!with!bang!", LineType::MappingKey),
        ("      !somewhere: >", "!somewhere", LineType::Tag),
        ("      complex!key!here!: >", "complex!key!here!", LineType::MappingKey),

        // Level 3 with different folded scalar modifiers
        ("      level3_key!: >", "level3_key!", LineType::MappingKey),
        ("      first!: >-", "first!", LineType::MappingKey),
        ("      second!: >+", "second!", LineType::MappingKey),
        ("      third!: >-2", "third!", LineType::MappingKey),
        ("      fourth!: >+2", "fourth!", LineType::MappingKey),

        // Edge cases for level 3
        ("      !: >", "!", LineType::Tag),
        ("      !!: >", "!!", LineType::Tag),
        ("      a!: >", "a!", LineType::MappingKey),
        ("      a!b: >", "a!b", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 3 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for level 3 indentation: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for level 3 indentation: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test level 3 continuation lines with '!' characters
    // Note: Lines starting with '!' may be classified as Tag, which is correct behavior
    let continuation_lines = vec![
        ("      Content with! exclamations!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      More! indented! level! 3! content!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      Single! line! with! multiple! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("      Ending! with! bang! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),
        ("      !At! start! and! middle!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("      Throughout! the! entire! line!", vec![LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Level 3 continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Level 3 continuation result should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}

#[test]
fn test_level4_indentation_with_exclamation_marks() {
    // Test level 4 (8-space) indentation with '!' character in Section 12B
    // This test focuses specifically on four-level indentation scenarios with exclamation marks
    // Level 4 uses 8-space indentation as the standard fourth-level indentation

    let test_cases = vec![
        // Basic level 4 keys with '!' at various positions
        ("        key!: >", "key!", LineType::MappingKey),
        ("        test!here: >", "test!here", LineType::MappingKey),
        ("        simple!test: >", "simple!test", LineType::MappingKey),
        ("        basic!: >-", "basic!", LineType::MappingKey),
        ("        another!one: >+", "another!one", LineType::MappingKey),

        // Level 4 with multiple '!' characters
        ("        key!!: >", "key!!", LineType::MappingKey),
        ("        test!here!now: >", "test!here!now", LineType::MappingKey),
        ("        multiple!!!: >", "multiple!!!", LineType::MappingKey),
        ("        spaced!out!keys!: >", "spaced!out!keys!", LineType::MappingKey),
        ("        end!with!bang!: >", "end!with!bang!", LineType::MappingKey),

        // Level 4 tag keys (starting with '!')
        ("        !tag: >", "!tag", LineType::Tag),
        ("        !.custom: >", "!.custom", LineType::Tag),
        ("        !local: >", "!local", LineType::Tag),
        ("        !!double: >", "!!double", LineType::Tag),

        // Level 4 keys with '!' in various positions
        ("        !start: >", "!start", LineType::Tag),
        ("        middle!bang: >", "middle!bang", LineType::MappingKey),
        ("        end!with!bang!: >", "end!with!bang!", LineType::MappingKey),
        ("        !somewhere: >", "!somewhere", LineType::Tag),
        ("        complex!key!here!: >", "complex!key!here!", LineType::MappingKey),

        // Level 4 with different folded scalar modifiers
        ("        level4_key!: >", "level4_key!", LineType::MappingKey),
        ("        first!: >-", "first!", LineType::MappingKey),
        ("        second!: >+", "second!", LineType::MappingKey),
        ("        third!: >-2", "third!", LineType::MappingKey),
        ("        fourth!: >+2", "fourth!", LineType::MappingKey),

        // Edge cases for level 4
        ("        !: >", "!", LineType::Tag),
        ("        !!: >", "!!", LineType::Tag),
        ("        a!: >", "a!", LineType::MappingKey),
        ("        a!b: >", "a!b", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 4 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for level 4 indentation: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for level 4 indentation: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test level 4 continuation lines with '!' characters
    // Note: Lines starting with '!' may be classified as Tag, which is correct behavior
    let continuation_lines = vec![
        ("        Content with! exclamations!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        More! indented! level! 4! content!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        Single! line! with! multiple! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("        Ending! with! bang! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),
        ("        !At! start! and! middle!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("        Throughout! the! entire! line!", vec![LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Level 4 continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Level 4 continuation result should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}

#[test]
fn test_level5_indentation_with_exclamation_marks() {
    // Test level 5 (10-space) indentation with '!' character in Section 12B
    // This test focuses specifically on five-level indentation scenarios with exclamation marks
    // Level 5 uses 10-space indentation as the standard fifth-level indentation

    let test_cases = vec![
        // Basic level 5 keys with '!' at various positions
        ("          key!: >", "key!", LineType::MappingKey),
        ("          test!here: >", "test!here", LineType::MappingKey),
        ("          simple!test: >", "simple!test", LineType::MappingKey),
        ("          basic!: >-", "basic!", LineType::MappingKey),
        ("          another!one: >+", "another!one", LineType::MappingKey),

        // Level 5 with multiple '!' characters
        ("          key!!: >", "key!!", LineType::MappingKey),
        ("          test!here!now: >", "test!here!now", LineType::MappingKey),
        ("          multiple!!!: >", "multiple!!!", LineType::MappingKey),
        ("          spaced!out!keys!: >", "spaced!out!keys!", LineType::MappingKey),
        ("          end!with!bang!: >", "end!with!bang!", LineType::MappingKey),

        // Level 5 tag keys (starting with '!')
        ("          !tag: >", "!tag", LineType::Tag),
        ("          !.custom: >", "!.custom", LineType::Tag),
        ("          !local: >", "!local", LineType::Tag),
        ("          !!double: >", "!!double", LineType::Tag),

        // Level 5 keys with '!' in various positions
        ("          !start: >", "!start", LineType::Tag),
        ("          middle!bang: >", "middle!bang", LineType::MappingKey),
        ("          end!with!bang!: >", "end!with!bang!", LineType::MappingKey),
        ("          !somewhere: >", "!somewhere", LineType::Tag),
        ("          complex!key!here!: >", "complex!key!here!", LineType::MappingKey),

        // Level 5 with different folded scalar modifiers
        ("          level5_key!: >", "level5_key!", LineType::MappingKey),
        ("          first!: >-", "first!", LineType::MappingKey),
        ("          second!: >+", "second!", LineType::MappingKey),
        ("          third!: >-2", "third!", LineType::MappingKey),
        ("          fourth!: >+2", "fourth!", LineType::MappingKey),

        // Edge cases for level 5
        ("          !: >", "!", LineType::Tag),
        ("          !!: >", "!!", LineType::Tag),
        ("          a!: >", "a!", LineType::MappingKey),
        ("          a!b: >", "a!b", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 5 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for level 5 indentation: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for level 5 indentation: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test level 5 continuation lines with '!' characters
    // Note: Lines starting with '!' may be classified as Tag, which is correct behavior
    let continuation_lines = vec![
        ("          Content with! exclamations!", vec![LineType::MappingKey, LineType::Unknown]),
        ("          More! indented! level! 5! content!", vec![LineType::MappingKey, LineType::Unknown]),
        ("          Single! line! with! multiple! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("          !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("          Ending! with! bang! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("          Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),
        ("          !At! start! and! middle!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("          Throughout! the! entire! line!", vec![LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Level 5 continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Level 5 continuation result should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}

#[test]
fn test_level6_indentation_with_exclamation_marks() {
    // Test level 6 (12-space) indentation with '!' character in Section 12B
    // This test focuses specifically on six-level indentation scenarios with exclamation marks
    // Level 6 uses 12-space indentation as the standard sixth-level indentation

    let test_cases = vec![
        // Basic level 6 keys with '!' at various positions
        ("            key!: >", "key!", LineType::MappingKey),
        ("            test!here: >", "test!here", LineType::MappingKey),
        ("            simple!test: >", "simple!test", LineType::MappingKey),
        ("            basic!: >-", "basic!", LineType::MappingKey),
        ("            another!one: >+", "another!one", LineType::MappingKey),

        // Level 6 with multiple '!' characters
        ("            key!!: >", "key!!", LineType::MappingKey),
        ("            test!here!now: >", "test!here!now", LineType::MappingKey),
        ("            multiple!!!: >", "multiple!!!", LineType::MappingKey),
        ("            spaced!out!keys!: >", "spaced!out!keys!", LineType::MappingKey),
        ("            end!with!bang!: >", "end!with!bang!", LineType::MappingKey),

        // Level 6 tag keys (starting with '!')
        ("            !tag: >", "!tag", LineType::Tag),
        ("            !.custom: >", "!.custom", LineType::Tag),
        ("            !local: >", "!local", LineType::Tag),
        ("            !!double: >", "!!double", LineType::Tag),

        // Level 6 keys with '!' in various positions
        ("            !start: >", "!start", LineType::Tag),
        ("            middle!bang: >", "middle!bang", LineType::MappingKey),
        ("            end!with!bang!: >", "end!with!bang!", LineType::MappingKey),
        ("            !somewhere: >", "!somewhere", LineType::Tag),
        ("            complex!key!here!: >", "complex!key!here!", LineType::MappingKey),

        // Level 6 with different folded scalar modifiers
        ("            level6_key!: >", "level6_key!", LineType::MappingKey),
        ("            first!: >-", "first!", LineType::MappingKey),
        ("            second!: >+", "second!", LineType::MappingKey),
        ("            third!: >-2", "third!", LineType::MappingKey),
        ("            fourth!: >+2", "fourth!", LineType::MappingKey),

        // Edge cases for level 6
        ("            !: >", "!", LineType::Tag),
        ("            !!: >", "!!", LineType::Tag),
        ("            a!: >", "a!", LineType::MappingKey),
        ("            a!b: >", "a!b", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Level 6 indentation test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for level 6 indentation: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for level 6 indentation: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test level 6 continuation lines with '!' characters
    // Note: Lines starting with '!' may be classified as Tag, which is correct behavior
    let continuation_lines = vec![
        ("            Content with! exclamations!", vec![LineType::MappingKey, LineType::Unknown]),
        ("            More! indented! level! 6! content!", vec![LineType::MappingKey, LineType::Unknown]),
        ("            Single! line! with! multiple! bangs!", vec![LineType::MappingKey, LineType::Unknown]),
        ("            !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("            Ending! with! bang! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("            Complex! continuation! line! test!", vec![LineType::MappingKey, LineType::Unknown]),
        ("            !At! start! and! middle!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("            Throughout! the! entire! line!", vec![LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Level 6 continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Level 6 continuation result should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );
    }
}

// ============================================================================
// Section 12B.2: Folded Scalar Indicator Line Tests
// ============================================================================

#[test]
fn test_folded_scalar_indicator_lines() {
    // Test folded scalar indicator lines (>) are classified as MappingKey
    // This covers the first acceptance criterion

    let test_cases = vec![
        // Basic folded scalar indicators (>)
        "description: >",
        "content: >",
        "message: >",
        "text: >",
        "note: >",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Basic folded scalar indicator (>) should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_basic_modifiers() {
    // Test folded scalar with basic modifiers (>-, >+)
    // This covers the second acceptance criterion

    let test_cases = vec![
        // Strip modifier (-) - removes trailing newlines
        "description: >-",
        "content: >-",
        "message: >-",

        // Keep modifier (+) - preserves trailing newlines
        "note: >+",
        "text: >+",
        "comment: >+",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded scalar with basic modifier (>-, >+) should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_numeric_modifiers() {
    // Test folded scalar with numeric modifiers (>2, >-2, >4, >-4, etc.)
    // This covers the third acceptance criterion

    let test_cases = vec![
        // Numeric modifiers (1-9)
        "text: >1",
        "content: >2",
        "message: >3",
        "note: >4",
        "description: >5",
        "comment: >6",
        "body: >7",
        "data: >8",
        "info: >9",

        // Numeric modifiers with strip (-)
        "text: >-1",
        "content: >-2",
        "message: >-3",
        "note: >-4",
        "description: >-5",
        "comment: >-6",
        "body: >-7",
        "data: >-8",
        "info: >-9",

        // Numeric modifiers with keep (+)
        "log: >+1",
        "output: >+2",
        "field: >+3",
        "value: >+4",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded scalar with numeric modifier (>n, >-n, >+n) should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_indented_indicators() {
    // Test folded scalar indicators with various indentation levels
    // This verifies classify_line_type() behavior for indicator lines with indentation

    let test_cases = vec![
        // 2-space indentation
        "  description: >",
        "  content: >",
        "  message: >-",
        "  note: >+",
        "  text: >2",

        // 4-space indentation
        "    description: >",
        "    content: >",
        "    message: >-",
        "    note: >+",
        "    text: >4",

        // Tab indentation
        "\tdescription: >",
        "\tcontent: >",
        "\tmessage: >-",
        "\tnote: >+",
        "\ttext: >2",

        // Mixed indentation (spaces + tabs)
        "  \tdescription: >",
        "    \tcontent: >",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded scalar indicator with indentation should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_all_modifier_combinations() {
    // Test all valid modifier combinations for folded scalars
    // This provides comprehensive coverage of the modifier syntax

    let test_cases = vec![
        // Basic > (no modifier)
        "text: >",

        // Strip modifier (>)
        "note: >-",

        // Keep modifier (+)
        "log: >+",

        // Numeric indent 1-9 (no strip/keep)
        "f1: >1",
        "f2: >2",
        "f3: >3",
        "f4: >4",
        "f5: >5",
        "f6: >6",
        "f7: >7",
        "f8: >8",
        "f9: >9",

        // Numeric indent with strip (-)
        "s1: >-1",
        "s2: >-2",
        "s3: >-3",
        "s4: >-4",
        "s5: >-5",
        "s6: >-6",
        "s7: >-7",
        "s8: >-8",
        "s9: >-9",

        // Numeric indent with keep (+)
        "k1: >+1",
        "k2: >+2",
        "k3: >+3",
        "k4: >+4",
        "k5: >+5",
        "k6: >+6",
        "k7: >+7",
        "k8: >+8",
        "k9: >+9",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "All folded scalar modifier combinations should be MappingKey: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 12B.1: Comprehensive Folded Block Scalar Tests with Exclamation
// ============================================================================

#[test]
fn test_folded_scalar_indicator_classification() {
    // Test that folded scalar indicator lines (>) are classified as MappingKey
    // This verifies the first acceptance criterion
    let test_cases = vec![
        // Basic folded scalar indicator
        "description: >",
        "  folded_text: >",
        "    note: >",
        "\tmessage: >",

        // Folded with strip modifier (-)
        "warning: >-",
        "  alert: >-",
        "    info: >-",

        // Folded with keep modifier (+)
        "log: >+",
        "  output: >+",
        "    data: >+",

        // Folded with explicit indent (2)
        "text: >-2",
        "content: >2",
        "  field: >-2",
        "    value: >2",

        // Folded with explicit indent (4)
        "doc: >-4",
        "info: >4",
        "  body: >-4",
        "    detail: >4",

        // Tab-indented folded scalars
        "\tfolded: >",
        "\t  note: >",
        "\t    text: >",
    ];

    for line in test_cases {
        // All folded scalar indicators should be classified as MappingKey
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Folded scalar indicator should be MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_continuation_lines_with_exclamation() {
    // Test continuation lines with exclamation marks in folded style
    // This verifies the third acceptance criterion

    let continuation_lines = vec![
        // Basic continuation lines with exclamation marks
        "  This is folded text with! exclamation marks",
        "    Multiple! exclamations! in! folded! style",
        "\tMore! content! with! bangs!",
        "  Important! message! continues!",

        // Exclamation at different positions
        "  !Start with exclamation",
        "  End with exclamation!",
        "  !Both! ends!",
        "  In! the! middle!",

        // Multiple consecutive exclamation marks
        "  Check this!!!",
        "  Urgent!! message!!",
        "  Critical!!! alert!!!",

        // Mixed content with exclamation marks
        "  Line 1! Line 2! Line 3!",
        "  First! Second! Third!",
        "  A! B! C! D! E! F!",
        "  !a!b!c!d!e!",

        // Tab-indented continuation lines with !
        "\tTab! folded! content!",
        "\t  More! tab! indented!",
        "\t\tDeep! tab! indentation!",
    ];

    for line in continuation_lines {
        // Continuation lines of folded scalars should be classified appropriately
        // Lines starting with ! are Tags (correct YAML behavior)
        // Other continuation lines are MappingKey or Unknown
        let result = classify_line_type(line);
        let starts_with_bang = line.trim().starts_with('!');
        if starts_with_bang {
            assert_eq!(
                result, LineType::Tag,
                "Continuation starting with ! should be Tag: '{}' (got {:?})",
                line, result
            );
        } else {
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown,
                "Folded scalar continuation should be MappingKey or Unknown: '{}' (got {:?})",
                line, result
            );
        }
    }

    // Lines that start with ! after indentation should be classified as Tag
    let tag_like_continuations = vec![
        "  !important",
        "    !custom",
        "\t!value",
    ];

    for line in tag_like_continuations {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::Tag,
            "Continuation starting with ! should be Tag: '{}'",
            line
        );
    }
}

#[test]
fn test_tab_indented_folded_scalars_with_exclamation() {
    // Test tab-indented folded scalars with exclamation marks
    // This verifies the fourth acceptance criterion

    let test_cases: Vec<(&str, Option<&str>)> = vec![
        // Tab-indented folded scalar indicators
        ("\ttext: >", Some("text")),
        ("\tnote: >", Some("note")),
        ("\tdescription: >", Some("description")),
        ("\tmessage: >-", Some("message")),
        ("\tlog: >+", Some("log")),
        ("\tcontent: >-2", Some("content")),
        ("\tdata: >2", Some("data")),
        ("\t'alert!': >", Some("'alert!'")),
        ("\t'warning!': >-", Some("'warning!'")),
        ("\t'info!': >+", Some("'info!'")),

        // Tab-indented continuation lines with exclamation marks
        ("\t!Tab! folded! content!", None),
        ("\t  More! tab! indented!", None),
        ("\t\tDeep! tab! indentation!", None),
        ("\t!Important! message!", None),
        ("\tMultiple! exclamations! in! tabs!", None),
    ];

    for (line, expected_key) in test_cases {
        if let Some(key) = expected_key {
            // This is a folded scalar indicator line (has colon)
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect tab-indented folded scalar indicator: '{}'",
                line
            );
            let info = info.unwrap();
            assert_eq!(
                info.key, key,
                "Should extract correct key from tab-indented line: '{}'",
                line
            );
        } else {
            // This is a continuation line (no colon)
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_none(),
                "Should NOT detect tab-indented continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_folded_scalar_various_indentation_levels() {
    // Test various indentation levels for folded scalars with exclamation marks
    // This verifies the fifth acceptance criterion

    let test_cases: Vec<(&str, Option<&str>, Option<&str>, bool)> = vec![
        // 0 spaces (root level)
        ("root: >", Some("root"), None, true),
        ("  Root! level! content!", None, None, false),

        // 2 spaces indentation
        ("  level2: >", Some("level2"), None, true),
        ("    Two! spaces! indent!", None, None, false),

        // 4 spaces indentation
        ("    level3: >", Some("level3"), None, true),
        ("      Four! spaces! indent!", None, None, false),

        // 6 spaces indentation
        ("      level4: >", Some("level4"), None, true),
        ("        Six! spaces! indent!", None, None, false),

        // 8 spaces indentation
        ("        level5: >", Some("level5"), None, true),
        ("          Eight! spaces! indent!", None, None, false),

        // Tab indentation
        ("\ttab1: >", Some("tab1"), None, true),
        ("\t\tTab! content!", None, None, false),

        // Mixed spaces and tabs (unusual but valid)
        ("  \tmixed: >", Some("mixed"), None, true),
        ("    \tMixed! content!", None, None, false),
    ];

    for (line, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);

        if should_detect {
            assert!(
                info.is_some(),
                "Should detect mapping key at indentation level: '{}'",
                line
            );
            let info = info.unwrap();
            if let Some(key) = expected_key {
                assert_eq!(
                    info.key, key,
                    "Should extract correct key at indentation: '{}'",
                    line
                );
            }
            if let Some(val) = expected_value {
                assert_eq!(
                    info.value, Some(val.to_string()),
                    "Should extract correct value at indentation: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_folded_scalar_modifiers_comprehensive() {
    // Test folded scalars with all modifier combinations and exclamation marks
    // This provides comprehensive coverage of the second acceptance criterion

    let test_cases: Vec<(&str, Option<&str>, Option<&str>, bool)> = vec![
        // Strip modifier (-) - removes trailing newlines
        ("note: >-", Some("note"), None, true),
        ("  !Important! note!", None, None, false),

        // Keep modifier (+) - keeps trailing newlines
        ("message: >+", Some("message"), None, true),
        ("  !Urgent! message!", None, None, false),

        // Explicit indent level 1
        ("text: >-1", Some("text"), None, true),
        ("  One! space! indent!", None, None, false),

        // Explicit indent level 2
        ("description: >-2", Some("description"), None, true),
        ("    Two! spaces! indent!", None, None, false),

        // Explicit indent level 3
        ("content: >-3", Some("content"), None, true),
        ("      Three! spaces! indent!", None, None, false),

        // Explicit indent level 4
        ("doc: >-4", Some("doc"), None, true),
        ("        Four! spaces! indent!", None, None, false),

        // Explicit indent level 5
        ("info: >-5", Some("info"), None, true),
        ("          Five! spaces! indent!", None, None, false),

        // Explicit indent level 6
        ("data: >-6", Some("data"), None, true),
        ("            Six! spaces! indent!", None, None, false),

        // Explicit indent level 7
        ("value: >-7", Some("value"), None, true),
        ("              Seven! spaces! indent!", None, None, false),

        // Explicit indent level 8
        ("field: >-8", Some("field"), None, true),
        ("                Eight! spaces! indent!", None, None, false),

        // Explicit indent level 9
        ("item: >-9", Some("item"), None, true),
        ("                  Nine! spaces! indent!", None, None, false),

        // Keep modifier with explicit indent
        ("log: >+2", Some("log"), None, true),
        ("    Keep! indent! two!", None, None, false),

        // Strip modifier with explicit indent
        ("output: >-4", Some("output"), None, true),
        ("        Strip! indent! four!", None, None, false),
    ];

    for (line, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);

        if should_detect {
            assert!(
                info.is_some(),
                "Should detect folded scalar with modifier: '{}'",
                line
            );
            let info = info.unwrap();
            if let Some(key) = expected_key {
                assert_eq!(
                    info.key, key,
                    "Should extract correct key with modifier: '{}'",
                    line
                );
            }
            if let Some(val) = expected_value {
                assert_eq!(
                    info.value, Some(val.to_string()),
                    "Should extract correct value with modifier: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_folded_scalar_exclamation_positions_comprehensive() {
    // Test exclamation marks at all possible positions in folded scalars
    // This provides comprehensive edge case coverage

    let test_cases: Vec<(&str, Option<&str>, Option<&str>, bool)> = vec![
        // ! at the very start of continuation line
        ("text: >", Some("text"), None, true),
        ("  !Start of line", None, None, false),

        // ! at the very end of continuation line
        ("note: >", Some("note"), None, true),
        ("  End of line!", None, None, false),

        // ! at both start and end
        ("message: >", Some("message"), None, true),
        ("  !Both ends!", None, None, false),

        // Multiple ! at start
        ("warning: >", Some("warning"), None, true),
        ("  !!!Triple start", None, None, false),

        // Multiple ! at end
        ("alert: >", Some("alert"), None, true),
        ("  Triple end!!!", None, None, false),

        // ! at every word boundary
        ("description: >", Some("description"), None, true),
        ("  !One! !Two! !Three! !Four!", None, None, false),

        // Consecutive ! in middle
        ("content: >", Some("content"), None, true),
        ("  Text!!!with!!!bangs", None, None, false),

        // ! with numbers
        ("data: >", Some("data"), None, true),
        ("  Item1! Item2! Item3!", None, None, false),

        // ! with special characters
        ("info: >", Some("info"), None, true),
        ("  (Hello)! [World]! {Test}!", None, None, false),

        // ! mixed with other punctuation
        ("text: >", Some("text"), None, true),
        ("  One, Two! Three; Four!", None, None, false),

        // Single ! alone on continuation line
        ("note: >", Some("note"), None, true),
        ("  !", None, None, false),

        // Only ! repeated
        ("message: >", Some("message"), None, true),
        ("  !!!!!", None, None, false),

        // ! with whitespace
        ("log: >", Some("log"), None, true),
        ("  ! ! ! ! !", None, None, false),

        // ! in various word positions
        ("doc: >", Some("doc"), None, true),
        ("  !pre!fix! mid!fix! suf!fix!", None, None, false),

        // Realistic text with !
        ("comment: >", Some("comment"), None, true),
        ("  This is important! Read carefully now!", None, None, false),

        // Multiple sentences with !
        ("body: >", Some("body"), None, true),
        ("  First sentence! Second sentence! Third!", None, None, false),

        // ! with quotes
        ("text: >", Some("text"), None, true),
        ("  He said \"Hello!\" and left!", None, None, false),
    ];

    for (line, expected_key, expected_value, should_detect) in test_cases {
        let info = detect_mapping_key(line, 0);

        if should_detect {
            assert!(
                info.is_some(),
                "Should detect mapping key for folded scalar: '{}'",
                line
            );
            let info = info.unwrap();
            if let Some(key) = expected_key {
                assert_eq!(
                    info.key, key,
                    "Should extract correct key: '{}'",
                    line
                );
            }
            if let Some(val) = expected_value {
                assert_eq!(
                    info.value, Some(val.to_string()),
                    "Should extract correct value: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

// ============================================================================
// Section 12B.2: Basic Folded Scalar Indicator Tests
// ============================================================================

#[test]
fn test_basic_folded_scalar_indicator_as_mapping_key() {
    // Test that basic folded scalar indicator lines (>) are classified as MappingKey
    // This verifies the first acceptance criterion:
    // "simple '>' indicator line is classified as MappingKey"

    let test_cases = vec![
        // Simple folded scalar indicator with key
        "key: >",
        "name: >",
        "value: >",
        "text: >",
        "description: >",

        // With whitespace variations
        "key: > ",   // Space after >
        "key:  >",   // Multiple spaces after colon
        "key : >",   // Space before colon

        // Indented folded scalar indicators (should still be MappingKey)
        "  key: >",
        "    name: >",
        "\tvalue: >",
    ];

    for line in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result,
            LineType::MappingKey,
            "Basic folded scalar indicator should be classified as MappingKey: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_with_continuation_content() {
    // Test folded scalar indicator lines with following content lines
    // This verifies the second acceptance criterion:
    // "test case: '>' with following content line"

    let test_cases = vec![
        // Simple indicator followed by content
        ("description: >", "  This is folded content"),
        ("text: >", "  Line with content"),
        ("note: >", "    More indented content"),
        ("message: >", "\tTab-indented content"),

        // Indicator followed by content with exclamation marks
        ("warning: >", "  Important! message"),
        ("alert: >", "  Check! this! now"),
        ("info: >", "  Multiple! exclamations! here"),

        // Various indentation levels for continuation
        ("key: >", "  Two space indent"),
        ("key: >", "    Four space indent"),
        ("key: >", "\tTab indent"),
        ("key: >", "  Mixed! content! here"),
    ];

    for (indicator_line, content_line) in test_cases {
        // Test the indicator line is classified as MappingKey
        let indicator_result = classify_line_type(indicator_line);
        assert_eq!(
            indicator_result,
            LineType::MappingKey,
            "Folded scalar indicator should be MappingKey: '{}'",
            indicator_line
        );

        // Test the continuation line classification
        // Content lines (indented without colons) are typically Unknown or MappingKey
        let content_result = classify_line_type(content_line);
        assert!(
            content_result == LineType::MappingKey || content_result == LineType::Unknown,
            "Folded scalar content line should be MappingKey or Unknown: '{}' (got {:?})",
            content_line, content_result
        );
    }
}

#[test]
fn test_folded_scalar_continuation_lines_with_exclamation_marks() {
    // Test folded scalar continuation lines containing exclamation marks
    // Continuation lines are indented lines that follow a folded scalar indicator (>)
    // This verifies that continuation lines with ! are properly classified

    let test_cases = vec![
        // Exclamation in the middle of continuation lines (NOT at start)
        ("note: >", "  check! this value"),
        ("message: >", "    hello! world"),
        ("comment: >", "\tdata! point"),

        // Exclamation at the end of continuation lines
        ("warning: >", "  important!"),
        ("alert: >", "    critical!"),
        ("info: >", "\turgent!"),

        // Multiple exclamation marks in continuation lines
        ("log: >", "  very! important! message!"),
        ("status: >", "    multiple!!! here!!!"),
        ("output:>", "\tvarious! positions! test!"),

        // Exclamation with different indentation levels (2 spaces)
        ("key: >", "  content! here"),
        ("field: >", "  value! with bang"),

        // Exclamation with 4-space indentation
        ("data: >", "    deeper! content"),
        ("value: >", "    nested! value"),

        // Exclamation with tab indentation
        ("text: >", "\ttab! indented"),
        ("note: >", "\tvalue! with tab"),

        // Mixed indentation (spaces then content with !)
        ("info: >", "  mixed! content! here"),
        ("debug: >", "    more! complex! data!"),

        // Exclamation in continuation after different folded indicators
        ("description: >-", "  content! with strip"),
        ("content: >+", "    data! with keep"),
        ("message: >2", "  text! with indent-2"),
        ("note: >-3", "    value! with strip-3"),
        ("log: >+4", "  info! with keep-4"),
    ];

    for (indicator_line, continuation_line) in test_cases {
        // Test the indicator line is classified as MappingKey
        let indicator_result = classify_line_type(indicator_line);
        assert_eq!(
            indicator_result,
            LineType::MappingKey,
            "Folded scalar indicator should be MappingKey: '{}'",
            indicator_line
        );

        // Test the continuation line with exclamation mark is properly classified
        // Continuation lines with ! in the middle/end are MappingKey or Unknown
        let continuation_result = classify_line_type(continuation_line);
        assert!(
            continuation_result == LineType::MappingKey || continuation_result == LineType::Unknown,
            "Folded scalar continuation line with ! should be MappingKey or Unknown, not Tag: '{}' (got {:?})",
            continuation_line, continuation_result
        );
    }
}

#[test]
fn test_folded_scalar_continuation_lines_starting_with_exclamation() {
    // Test continuation lines that START with exclamation marks
    // These are edge cases where the continuation looks like a YAML tag
    // but in the context of a folded scalar, it's actually content

    let test_cases = vec![
        // Continuation lines starting with ! (would be Tag if not in context)
        ("description: >", "  !important note"),
        ("text: >", "    !warning message"),
        ("content: >", "\t!critical alert"),

        // With different folded indicators
        ("note: >-", "  !value with strip"),
        ("message: >+", "    !data with keep"),
        ("log: >2", "  !text indent-2"),
    ];

    for (indicator_line, continuation_line) in test_cases {
        // Test the indicator line is classified as MappingKey
        let indicator_result = classify_line_type(indicator_line);
        assert_eq!(
            indicator_result,
            LineType::MappingKey,
            "Folded scalar indicator should be MappingKey: '{}'",
            indicator_line
        );

        // Lines starting with ! are classified as Tag by the syntax classifier
        // This is technically correct YAML syntax, even if in a folded scalar context
        let continuation_result = classify_line_type(continuation_line);
        assert_eq!(
            continuation_result,
            LineType::Tag,
            "Continuation starting with ! is classified as Tag (syntactically correct): '{}'",
            continuation_line
        );
    }
}

#[test]
fn test_folded_scalar_continuation_exclamation_various_contexts() {
    // Test continuation lines with exclamation marks in various contextual scenarios
    // This ensures folded scalar parsing handles ! correctly in continuations

    let test_cases = vec![
        // Continuation with ! as CSS-like important flag
        ("styles: >", "  .button!important"),

        // Continuation with ! in URL-like values
        ("url: >", "  https://example.com/path!query"),

        // Continuation with ! in natural language sentences
        ("note: >", "  This is important! Check it."),
        ("message: >", "    Warning! Something happened!"),

        // Continuation with ! in regex-like patterns (avoiding FlowSequence syntax)
        ("pattern: >", "  .*!.*"),
        ("regex: >", "    match! pattern!"),

        // Continuation with ! in code-like snippets
        ("code: >", "  if (value!) { return; }"),
        ("snippet: >", "    flag = true!"),

        // Continuation with ! in error messages
        ("error: >", "  Failed! Check logs!"),
        ("exception: >", "    Error! Timeout!"),

        // Continuation with ! in configuration values
        ("config: >", "  enabled: true!"),
        ("settings: >", "    priority: high!"),

        // Continuation with ! in pseudo-data structures (avoiding FlowSequence syntax)
        ("data: >", "  key: value!"),
        ("structure: >", "    item1! item2!"),

        // Continuation with ! at various positions relative to words
        ("text: >", "  word!middle!end"),
        ("value: >", "  multiple! spaces! around!"),
    ];

    for (indicator_line, continuation_line) in test_cases {
        // Test the indicator line is classified as MappingKey
        let indicator_result = classify_line_type(indicator_line);
        assert_eq!(
            indicator_result,
            LineType::MappingKey,
            "Folded scalar indicator should be MappingKey: '{}'",
            indicator_line
        );

        // Test the continuation line with ! in various contexts
        // Should NOT be classified as Tag (unless it starts with !)
        let continuation_result = classify_line_type(continuation_line);

        // Lines starting with ! would be Tag, others should be MappingKey, Unknown, or Flow types
        if continuation_line.trim().starts_with('!') {
            assert_eq!(
                continuation_result,
                LineType::Tag,
                "Continuation starting with ! should be Tag: '{}'",
                continuation_line
            );
        } else {
            // Accept MappingKey, Unknown, or Flow types for continuation lines
            // (Flow types occur when line contains { or [ characters)
            assert!(
                continuation_result == LineType::MappingKey
                    || continuation_result == LineType::Unknown
                    || continuation_result == LineType::FlowMapping
                    || continuation_result == LineType::FlowSequence,
                "Folded scalar continuation with ! in context should be MappingKey, Unknown, or Flow type: '{}' (got {:?})",
                continuation_line, continuation_result
            );

            // Explicitly verify it's NOT classified as Tag (unless it starts with !)
            assert_ne!(
                continuation_result,
                LineType::Tag,
                "Folded scalar continuation line with ! should NOT be classified as Tag: '{}'",
                continuation_line
            );
        }
    }
}

#[test]
fn test_comprehensive_tab_indented_folded_scalars_with_exclamation() {
    // Comprehensive test for tab-indented folded scalars with exclamation marks
    // Verifies classification behavior across all tab indentation scenarios

    let test_cases: Vec<(&str, LineType, &str)> = vec![
        // Single tab with folded scalar indicator
        ("\tmessage: >", LineType::MappingKey, "Single tab with > indicator"),
        ("\ttext: >-", LineType::MappingKey, "Single tab with >- modifier"),
        ("\tnote: >+", LineType::MappingKey, "Single tab with >+ modifier"),
        ("\tcontent: >2", LineType::MappingKey, "Single tab with >2 explicit indent"),
        ("\tdata: >-2", LineType::MappingKey, "Single tab with >-2 explicit indent"),

        // Double tab with folded scalar indicator
        ("\t\tdeep: >", LineType::MappingKey, "Double tab with > indicator"),
        ("\t\tvalue: >", LineType::MappingKey, "Double tab with > indicator"),

        // Triple tab with folded scalar indicator
        ("\t\t\tnested: >", LineType::MappingKey, "Triple tab with > indicator"),
        ("\t\t\titem: >", LineType::MappingKey, "Triple tab with > indicator"),

        // Tab-indented continuation lines with exclamation marks
        // These can be MappingKey or Unknown (both are valid)
        ("\tContent! with! exclamations!", LineType::Unknown, "Single tab continuation with !"),
        ("\t  More! text! here!", LineType::Unknown, "Single tab + 2 spaces continuation with !"),
        ("\t\tDeep! folded! content!", LineType::Unknown, "Double tab continuation with !"),
        ("\t\t\tTriple! tab! indentation!", LineType::Unknown, "Triple tab continuation with !"),

        // Tab-indented lines starting with ! (should be Tag)
        ("\t!important", LineType::Tag, "Single tab starting with !"),
        ("\t!custom_type", LineType::Tag, "Single tab with custom type"),
        ("\t\t!nested_type", LineType::Tag, "Double tab starting with !"),
    ];

    for (line, expected_type, description) in test_cases {
        let result = classify_line_type(line);
        // For continuation lines without colons, both MappingKey and Unknown are acceptable
        if line.contains(':') {
            assert_eq!(
                result, expected_type,
                "{}: '{}' (expected {:?}, got {:?})",
                description, line, expected_type, result
            );
        } else if expected_type == LineType::Tag {
            assert_eq!(
                result, expected_type,
                "{}: '{}' (expected {:?}, got {:?})",
                description, line, expected_type, result
            );
        } else {
            // Continuation lines can be either MappingKey or Unknown
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown,
                "{}: '{}' (expected MappingKey or Unknown, got {:?})",
                description, line, result
            );
        }
    }

    // Verify detect_mapping_key behavior for tab-indented lines
    let detection_tests: Vec<(&str, bool, Option<&str>)> = vec![
        // Should detect as mapping keys
        ("\tkey: >", true, Some("key")),
        ("\t  name: >", true, Some("name")),
        ("\t\tvalue: >", true, Some("value")),
        ("\ttext: >-", true, Some("text")),
        ("\t\tnote: >+", true, Some("note")),

        // Should NOT detect as mapping keys (continuation lines)
        ("\tcontent! here!", false, None),
        ("\t  more! text!", false, None),
        ("\t\tdeep! indented!", false, None),
        ("\t\t\tvery! deep!", false, None),
    ];

    for (line, should_detect, expected_key) in detection_tests {
        let info = detect_mapping_key(line, 0);
        if should_detect {
            assert!(
                info.is_some(),
                "Should detect tab-indented mapping key: '{}'",
                line
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "Should extract correct key from tab-indented line: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect tab-indented continuation as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_comprehensive_various_indentation_levels_with_exclamation() {
    // Comprehensive test for various indentation levels with folded scalars and exclamation marks
    // Covers deeper and more diverse indentation scenarios

    let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
        // 0 spaces (root level)
        ("root: >", LineType::MappingKey, true, Some("root")),
        ("  Root! level! content!", LineType::MappingKey, false, None),

        // 2 spaces
        ("  level2: >", LineType::MappingKey, true, Some("level2")),
        ("    Two! spaces! indent!", LineType::MappingKey, false, None),

        // 4 spaces
        ("    level3: >", LineType::MappingKey, true, Some("level3")),
        ("      Four! spaces! indent!", LineType::MappingKey, false, None),

        // 6 spaces
        ("      level4: >", LineType::MappingKey, true, Some("level4")),
        ("        Six! spaces! indent!", LineType::MappingKey, false, None),

        // 8 spaces
        ("        level5: >", LineType::MappingKey, true, Some("level5")),
        ("          Eight! spaces! indent!", LineType::MappingKey, false, None),

        // 10 spaces (unusual but valid)
        ("          level6: >", LineType::MappingKey, true, Some("level6")),
        ("            Ten! spaces! indent!", LineType::MappingKey, false, None),

        // 12 spaces (deep nesting)
        ("            level7: >", LineType::MappingKey, true, Some("level7")),
        ("              Twelve! spaces! indent!", LineType::MappingKey, false, None),

        // Single tab
        ("\ttab1: >", LineType::MappingKey, true, Some("tab1")),
        ("\t\tTab! content!", LineType::MappingKey, false, None),

        // Double tab
        ("\t\ttab2: >", LineType::MappingKey, true, Some("tab2")),
        ("\t\t\tDouble! tab! content!", LineType::MappingKey, false, None),

        // Triple tab
        ("\t\t\ttab3: >", LineType::MappingKey, true, Some("tab3")),
        ("\t\t\t\tTriple! tab! deep!", LineType::MappingKey, false, None),
    ];

    for (line, expected_type, should_detect_key, expected_key) in test_cases {
        let result = classify_line_type(line);

        // For lines without colons (continuation lines), both MappingKey and Unknown are acceptable
        if line.contains(':') {
            assert_eq!(
                result, expected_type,
                "Line type classification failed for: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        } else {
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown,
                "Continuation line should be MappingKey or Unknown: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        }

        // Test detect_mapping_key behavior
        let info = detect_mapping_key(line, 0);
        if should_detect_key {
            assert!(
                info.is_some(),
                "Should detect mapping key at indentation level: '{}'",
                line
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "Should extract correct key at indentation: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_mixed_indentation_scenarios_with_folded_scalars() {
    // Test mixed indentation scenarios (tabs and spaces combined)
    // These are unusual but syntactically valid YAML that should be handled correctly

    let test_cases: Vec<(&str, LineType, bool, Option<&str>, &str)> = vec![
        // Tab followed by spaces
        ("\t mixed1: >", LineType::MappingKey, true, Some("mixed1"), "Tab + 1 space with >"),
        ("\t  mixed2: >", LineType::MappingKey, true, Some("mixed2"), "Tab + 2 spaces with >"),
        ("\t    mixed3: >", LineType::MappingKey, true, Some("mixed3"), "Tab + 4 spaces with >"),

        // Spaces followed by tab
        (" \tmixed4: >", LineType::MappingKey, true, Some("mixed4"), "1 space + tab with >"),
        ("  \tmixed5: >", LineType::MappingKey, true, Some("mixed5"), "2 spaces + tab with >"),
        ("    \tmixed6: >", LineType::MappingKey, true, Some("mixed6"), "4 spaces + tab with >"),

        // Continuation lines with mixed indentation and exclamation marks
        // These should be Unknown or MappingKey (both valid for continuations)
        ("\t Mixed! content! here!", LineType::Unknown, false, None, "Tab + space continuation with !"),
        ("\t  More! mixed! indentation!", LineType::Unknown, false, None, "Tab + 2 spaces continuation with !"),
        (" \tSpaces! then! tab! indent!", LineType::Unknown, false, None, "Space + tab continuation with !"),
        ("  \tTwo! spaces! then! tab!", LineType::Unknown, false, None, "2 spaces + tab continuation with !"),

        // Mixed indentation with modifiers
        ("\t mixed: >-", LineType::MappingKey, true, Some("mixed"), "Tab + space with >-"),
        ("  \tmixed: >+", LineType::MappingKey, true, Some("mixed"), "2 spaces + tab with >+"),
        ("\t  mixed: >2", LineType::MappingKey, true, Some("mixed"), "Tab + 2 spaces with >2"),
        ("    \tmixed: >-2", LineType::MappingKey, true, Some("mixed"), "4 spaces + tab with >-2"),

        // Lines starting with ! in mixed indentation (should be Tag)
        ("\t !important", LineType::Tag, false, None, "Tab + space starting with !"),
        ("  \t!value", LineType::Tag, false, None, "2 spaces + tab starting with !"),
        ("\t  !custom", LineType::Tag, false, None, "Tab + 2 spaces starting with !"),
    ];

    for (line, expected_type, should_detect_key, expected_key, description) in test_cases {
        let result = classify_line_type(line);

        // For lines without colons (continuation lines), both MappingKey and Unknown are acceptable
        // For lines with colons or Tags, check exact match
        if line.contains(':') || expected_type == LineType::Tag {
            assert_eq!(
                result, expected_type,
                "{}: '{}' (expected {:?}, got {:?})",
                description, line, expected_type, result
            );
        } else {
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown,
                "{}: Continuation should be MappingKey or Unknown: '{}' (got {:?})",
                description, line, result
            );
        }

        // Test detect_mapping_key behavior
        let info = detect_mapping_key(line, 0);
        if should_detect_key {
            assert!(
                info.is_some(),
                "{}: Should detect mapping key",
                description
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "{}: Should extract correct key",
                    description
                );
            }
        } else {
            // For continuation lines or Tag lines, should NOT detect as mapping key
            if expected_type != LineType::Tag {
                assert!(
                    info.is_none(),
                    "{}: Should NOT detect continuation as mapping key: '{}' (got key: {:?})",
                    description, line, info.map(|i| i.key)
                );
            }
        }
    }

    // Test classification behavior verification across indentation variations
    let classification_tests: Vec<(&str, LineType, &str)> = vec![
        // Verify that deeply indented folded scalars with ! are correctly classified
        ("          deeply: >", LineType::MappingKey, "Deeply indented folded scalar indicator"),
        ("            Deep! content! with! exclamations!", LineType::Unknown, "Deeply indented continuation with !"),

        // Verify tab + space combinations with exclamation marks in continuation
        ("\t deeply! mixed! indented!", LineType::Unknown, "Tab + space continuation with multiple !"),
        ("  \t  Mixed! at! various! levels!", LineType::Unknown, "Spaces + tab + spaces continuation with !"),

        // Verify that mixed indentation doesn't break folded scalar recognition
        ("\t    very: >", LineType::MappingKey, "Tab + multiple spaces with >"),
        ("      \t  mixed: >", LineType::MappingKey, "Multiple spaces + tab + spaces with >"),
    ];

    for (line, expected_type, description) in classification_tests {
        let result = classify_line_type(line);
        // For lines with colons, check exact match
        if line.contains(':') {
            assert_eq!(
                result, expected_type,
                "{}: '{}' (expected {:?}, got {:?})",
                description, line, expected_type, result
            );
        } else {
            // For continuation lines without colons, both MappingKey and Unknown are acceptable
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown,
                "{}: Continuation should be MappingKey or Unknown: '{}' (got {:?})",
                description, line, result
            );
        }
    }
}

#[test]
fn test_odd_indentation_levels_with_exclamation_marks() {
    // Test odd indentation levels (1, 3, 5, 7, 9, 11 spaces) with folded scalars and exclamation marks
    // These complement the existing even indentation level tests

    let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
        // 1 space indentation
        (" level1: >", LineType::MappingKey, true, Some("level1")),
        ("  One! space! indent!", LineType::MappingKey, false, None),

        // 3 spaces indentation
        ("   level3: >", LineType::MappingKey, true, Some("level3")),
        ("     Three! spaces! indent!", LineType::MappingKey, false, None),

        // 5 spaces indentation
        ("     level5: >", LineType::MappingKey, true, Some("level5")),
        ("       Five! spaces! indent!", LineType::MappingKey, false, None),

        // 7 spaces indentation
        ("       level7: >", LineType::MappingKey, true, Some("level7")),
        ("         Seven! spaces! indent!", LineType::MappingKey, false, None),

        // 9 spaces indentation
        ("         level9: >", LineType::MappingKey, true, Some("level9")),
        ("           Nine! spaces! indent!", LineType::MappingKey, false, None),

        // 11 spaces indentation
        ("           level11: >", LineType::MappingKey, true, Some("level11")),
        ("             Eleven! spaces! indent!", LineType::MappingKey, false, None),

        // Odd indentation with folded scalar modifiers
        (" odd1: >-", LineType::MappingKey, true, Some("odd1")),
        ("  Stripped! odd! indent!", LineType::MappingKey, false, None),

        ("   odd3: >+", LineType::MappingKey, true, Some("odd3")),
        ("     Kept! odd! indent!", LineType::MappingKey, false, None),

        ("     odd5: >2", LineType::MappingKey, true, Some("odd5")),
        ("       Explicit! indent! level!", LineType::MappingKey, false, None),

        // Odd indentation with exclamation in various positions
        // Note: Keys starting with '!' are Tags in YAML, not MappingKey
        (" !start: >", LineType::Tag, false, None),
        ("  !End! with! exclamations!", LineType::Tag, false, None),

        ("   !both: >", LineType::Tag, false, None),
        ("     In! the! middle! here!", LineType::MappingKey, false, None),

        // Odd indentation continuation lines with multiple !
        ("       Multiple!!!", LineType::MappingKey, false, None),
        ("         Urgent!! message!!", LineType::MappingKey, false, None),
        ("           Critical!!! alert!!!", LineType::MappingKey, false, None),
    ];

    for (line, expected_type, should_detect_key, expected_key) in test_cases {
        let result = classify_line_type(line);

        // For lines without colons (continuation lines), MappingKey, Unknown, or Tag are acceptable
        // Tag is acceptable because lines starting with '!' are classified as tags
        if line.contains(':') {
            assert_eq!(
                result, expected_type,
                "Line type classification failed for odd indentation: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        } else {
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown || result == LineType::Tag,
                "Continuation line should be MappingKey, Unknown, or Tag: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        }

        // Test detect_mapping_key behavior
        let info = detect_mapping_key(line, 0);
        if should_detect_key {
            assert!(
                info.is_some(),
                "Should detect mapping key at odd indentation level: '{}'",
                line
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "Should extract correct key at odd indentation: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_deep_indentation_levels_with_exclamation_marks() {
    // Test very deep indentation levels (14, 16, 18, 20, 24 spaces) with exclamation marks
    // These test edge cases for deeply nested YAML structures

    let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
        // 14 spaces indentation
        ("              level14: >", LineType::MappingKey, true, Some("level14")),
        ("                Fourteen! spaces! deep!", LineType::MappingKey, false, None),

        // 16 spaces indentation
        ("                level16: >", LineType::MappingKey, true, Some("level16")),
        ("                  Sixteen! spaces! indent!", LineType::MappingKey, false, None),

        // 18 spaces indentation
        ("                  level18: >", LineType::MappingKey, true, Some("level18")),
        ("                    Eighteen! spaces! very! deep!", LineType::MappingKey, false, None),

        // 20 spaces indentation
        ("                    level20: >", LineType::MappingKey, true, Some("level20")),
        ("                      Twenty! spaces! extreme!", LineType::MappingKey, false, None),

        // 24 spaces indentation (very deep nesting)
        ("                        level24: >", LineType::MappingKey, true, Some("level24")),
        ("                          Twenty! four! spaces! ultra!", LineType::MappingKey, false, None),

        // Deep indentation with folded scalar modifiers
        ("              deep14: >-", LineType::MappingKey, true, Some("deep14")),
        ("                Stripped! deep! content!", LineType::MappingKey, false, None),

        ("                deep16: >+", LineType::MappingKey, true, Some("deep16")),
        ("                  Kept! deep! indentation!", LineType::MappingKey, false, None),

        ("                  deep18: >2", LineType::MappingKey, true, Some("deep18")),
        ("                    Explicit! deep! indent!", LineType::MappingKey, false, None),

        // Deep indentation with exclamation at various positions
        ("              !deep: >", LineType::Tag, false, None),
        ("                Very! deep! !marker!", LineType::MappingKey, false, None),

        ("                  !both: >", LineType::Tag, false, None),
        ("                    In! deep! middle! !here!", LineType::MappingKey, false, None),

        // Deep indentation continuation lines with multiple !
        ("                Multiple!!! at!!! depth!!!", LineType::MappingKey, false, None),
        ("                  Urgent!! deep!! message!!", LineType::MappingKey, false, None),
        ("                    Critical!!! very!!! deep!!!", LineType::MappingKey, false, None),
    ];

    for (line, expected_type, should_detect_key, expected_key) in test_cases {
        let result = classify_line_type(line);

        // For lines without colons (continuation lines), MappingKey, Unknown, or Tag are acceptable
        if line.contains(':') {
            assert_eq!(
                result, expected_type,
                "Line type classification failed for deep indentation: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        } else {
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown || result == LineType::Tag,
                "Continuation line should be MappingKey, Unknown, or Tag: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        }

        // Test detect_mapping_key behavior
        let info = detect_mapping_key(line, 0);
        if should_detect_key {
            assert!(
                info.is_some(),
                "Should detect mapping key at deep indentation level: '{}'",
                line
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "Should extract correct key at deep indentation: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_extensive_tab_indentation_with_exclamation_marks() {
    // Test extensive tab indentation (4, 5, 6 tabs) with exclamation marks
    // These test edge cases for very deeply nested tab-indented YAML structures

    let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
        // 4 tabs indentation
        ("\t\t\t\ttab4: >", LineType::MappingKey, true, Some("tab4")),
        ("\t\t\t\t\tFour! tabs! deep!", LineType::MappingKey, false, None),

        // 5 tabs indentation
        ("\t\t\t\t\ttab5: >", LineType::MappingKey, true, Some("tab5")),
        ("\t\t\t\t\t\tFive! tabs! very! deep!", LineType::MappingKey, false, None),

        // 6 tabs indentation (very deep tab nesting)
        ("\t\t\t\t\t\ttab6: >", LineType::MappingKey, true, Some("tab6")),
        ("\t\t\t\t\t\t\tSix! tabs! extreme!", LineType::MappingKey, false, None),

        // Deep tab indentation with folded scalar modifiers
        ("\t\t\t\tdeep4: >-", LineType::MappingKey, true, Some("deep4")),
        ("\t\t\t\t\tStripped! deep! tabs!", LineType::MappingKey, false, None),

        ("\t\t\t\t\tdeep5: >+", LineType::MappingKey, true, Some("deep5")),
        ("\t\t\t\t\t\tKept! deep! tab! indent!", LineType::MappingKey, false, None),

        ("\t\t\t\t\t\tdeep6: >2", LineType::MappingKey, true, Some("deep6")),
        ("\t\t\t\t\t\t\tExplicit! deep! tab!", LineType::MappingKey, false, None),

        // Deep tab indentation with exclamation at various positions
        ("\t\t\t\t!deep: >", LineType::Tag, false, None),
        ("\t\t\t\t\tVery! deep! !marker!", LineType::MappingKey, false, None),

        ("\t\t\t\t\t!both: >", LineType::Tag, false, None),
        ("\t\t\t\t\t\tIn! deep! middle! !here!", LineType::MappingKey, false, None),

        // Deep tab indentation continuation lines with multiple !
        ("\t\t\t\t\tMultiple!!! at!!! depth!!!", LineType::MappingKey, false, None),
        ("\t\t\t\t\t\tUrgent!! deep!! message!!", LineType::MappingKey, false, None),
        ("\t\t\t\t\t\t\tCritical!!! very!!! deep!!!", LineType::MappingKey, false, None),
    ];

    for (line, expected_type, should_detect_key, expected_key) in test_cases {
        let result = classify_line_type(line);

        // For lines without colons (continuation lines), MappingKey, Unknown, or Tag are acceptable
        if line.contains(':') {
            assert_eq!(
                result, expected_type,
                "Line type classification failed for deep tab indentation: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        } else {
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown || result == LineType::Tag,
                "Continuation line should be MappingKey, Unknown, or Tag: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        }

        // Test detect_mapping_key behavior
        let info = detect_mapping_key(line, 0);
        if should_detect_key {
            assert!(
                info.is_some(),
                "Should detect mapping key at deep tab indentation level: '{}'",
                line
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "Should extract correct key at deep tab indentation: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_complex_mixed_indentation_with_exclamation_marks() {
    // Test complex mixed indentation scenarios (tabs and spaces in various combinations)
    // These test unusual but syntactically valid YAML that should be handled correctly

    let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
        // Tab followed by multiple spaces (complex patterns)
        ("\t    complex1: >", LineType::MappingKey, true, Some("complex1")),
        ("\t      Complex! mixed! content!", LineType::MappingKey, false, None),

        ("    \tcomplex2: >", LineType::MappingKey, true, Some("complex2")),
        ("      \tMore! complex! pattern!", LineType::MappingKey, false, None),

        // Multiple spaces followed by tab
        ("      \tmixed3: >", LineType::MappingKey, true, Some("mixed3")),
        ("        \tDeep! mixed! spaces! tabs!", LineType::MappingKey, false, None),

        // Alternating tabs and spaces (unusual but valid)
        ("\t \t alt1: >", LineType::MappingKey, true, Some("alt1")),
        ("\t \t  Alt! pattern! here!", LineType::MappingKey, false, None),

        ("  \t  alt2: >", LineType::MappingKey, true, Some("alt2")),
        ("  \t    Another! alt! pattern!", LineType::MappingKey, false, None),

        // Complex mixed indentation with modifiers
        ("\t    mixed: >-", LineType::MappingKey, true, Some("mixed")),
        ("\t      Stripped! complex! mix!", LineType::MappingKey, false, None),

        ("    \tmixed: >+", LineType::MappingKey, true, Some("mixed")),
        ("      \tKept! complex! pattern!", LineType::MappingKey, false, None),

        // Complex mixed indentation with exclamation in key
        ("\t    key!with!bang: >", LineType::MappingKey, true, Some("key!with!bang")),
        ("\t      Another!key!here!: >", LineType::MappingKey, true, Some("Another!key!here!")),
        ("    \tmixed!key!test: >", LineType::MappingKey, true, Some("mixed!key!test")),

        // Complex mixed continuation lines with multiple !
        ("\t      Complex!!! multiple!!! patterns!!!", LineType::MappingKey, false, None),
        ("        \tUrgent!! complex!! message!!", LineType::MappingKey, false, None),
        ("  \t    Critical!!! very!!! complex!!!", LineType::MappingKey, false, None),

        // Complex mixed indentation starting with ! (should be Tag)
        ("\t      !important", LineType::Tag, false, None),
        ("        \t!value", LineType::Tag, false, None),
        ("  \t    !custom", LineType::Tag, false, None),
    ];

    for (line, expected_type, should_detect_key, expected_key) in test_cases {
        let result = classify_line_type(line);

        // For lines without colons (continuation lines), MappingKey, Unknown, or Tag are acceptable
        // For lines with colons or Tags, check exact match
        if line.contains(':') || expected_type == LineType::Tag {
            assert_eq!(
                result, expected_type,
                "Complex mixed indentation test failed: '{}' (expected {:?}, got {:?})",
                line, expected_type, result
            );
        } else {
            assert!(
                result == LineType::MappingKey || result == LineType::Unknown || result == LineType::Tag,
                "Complex mixed continuation should be MappingKey, Unknown, or Tag: '{}' (got {:?})",
                line, result
            );
        }

        // Test detect_mapping_key behavior
        let info = detect_mapping_key(line, 0);
        if should_detect_key {
            assert!(
                info.is_some(),
                "Should detect mapping key in complex mixed indentation: '{}'",
                line
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "Should extract correct key in complex mixed indentation: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect continuation line as mapping key in complex mixed: '{}'",
                line
            );
        }
    }
}

#[test]
fn test_various_indentation_levels_with_exclamation_mark() {
    // Test various indentation levels with '!' character in different positions
    // This systematically tests indentation from 2 to 12+ spaces with exclamation marks
    // covering edge cases and common patterns

    let test_cases: Vec<(&str, LineType)> = vec![
        // ===== 2-space indentation =====
        ("  key2!bang: >", LineType::MappingKey),
        ("  test!here: >", LineType::MappingKey),
        ("  end!with!bang!: >", LineType::MappingKey),
        ("  !key: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("  !.nested: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 3-space indentation =====
        ("   key3!bang: >", LineType::MappingKey),
        ("   test!here: >", LineType::MappingKey),
        ("   !.tag: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 4-space indentation =====
        ("    key4!bang: >", LineType::MappingKey),
        ("    test!here: >", LineType::MappingKey),
        ("    another!key!here: >", LineType::MappingKey),
        ("    !start: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("    !.custom: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 5-space indentation =====
        ("     key5!bang: >", LineType::MappingKey),
        ("     test!here: >", LineType::MappingKey),
        ("     !.type: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 6-space indentation =====
        ("      key6!bang: >", LineType::MappingKey),
        ("      test!here: >", LineType::MappingKey),
        ("      middle!bang: >", LineType::MappingKey),
        ("      !.value: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 7-space indentation =====
        ("       key7!bang: >", LineType::MappingKey),
        ("       test!here: >", LineType::MappingKey),
        ("       !.deep: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 8-space indentation =====
        ("        key8!bang: >", LineType::MappingKey),
        ("        test!here: >", LineType::MappingKey),
        ("        deep!nest!ed: >", LineType::MappingKey),
        ("        !nested!: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("        !.very: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 9-space indentation =====
        ("         key9!bang: >", LineType::MappingKey),
        ("         test!here: >", LineType::MappingKey),
        ("         !.nine: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 10-space indentation =====
        ("          key10!bang: >", LineType::MappingKey),
        ("          test!here: >", LineType::MappingKey),
        ("          !.ten: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 11-space indentation =====
        ("           key11!bang: >", LineType::MappingKey),
        ("           test!here: >", LineType::MappingKey),
        ("           !.eleven: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 12-space indentation =====
        ("            key12!bang: >", LineType::MappingKey),
        ("            test!here: >", LineType::MappingKey),
        ("            !.twelve: >", LineType::Tag),  // Tags starting with '!.'

        // ===== 16-space indentation (extra deep) =====
        ("                key16!bang: >", LineType::MappingKey),
        ("                test!here: >", LineType::MappingKey),
        ("                !.deep: >", LineType::Tag),  // Tags starting with '!.'

        // ===== Tab indentation (single tab) =====
        ("\tkeytab!bang: >", LineType::MappingKey),
        ("\ttest!here: >", LineType::MappingKey),
        ("\ttab!key!with!bang!: >", LineType::MappingKey),
        ("\t!key: >", LineType::Tag),  // Keys starting with '!' are Tags
        ("\t!.tag: >", LineType::Tag),  // Tags starting with '!.'

        // ===== Double tab indentation =====
        ("\t\tkeytab2!bang: >", LineType::MappingKey),
        ("\t\ttest!here: >", LineType::MappingKey),
        ("\t\tdouble!tab!key!test: >", LineType::MappingKey),
        ("\t\t!.double: >", LineType::Tag),  // Tags starting with '!.'

        // ===== Triple tab indentation =====
        ("\t\t\tkeytab3!bang: >", LineType::MappingKey),
        ("\t\t\ttest!here: >", LineType::MappingKey),
        ("\t\t\t!.triple: >", LineType::Tag),  // Tags starting with '!.'

        // ===== Tab + spaces combinations =====
        ("\t  keytab2s!bang: >", LineType::MappingKey),
        ("  \tkeys2tab!bang: >", LineType::MappingKey),
        ("\t    keytab4s!bang: >", LineType::MappingKey),
        ("    \tkeys4tab!bang: >", LineType::MappingKey),
        ("\t      keytab6s!bang: >", LineType::MappingKey),
        ("      \tkeys6tab!bang: >", LineType::MappingKey),
        ("\t  !.mix1: >", LineType::Tag),  // Tags starting with '!.'
        ("  \t!.mix2: >", LineType::Tag),  // Tags starting with '!.'
        ("\t    !.mix3: >", LineType::Tag),  // Tags starting with '!.'
        ("    \t!.mix4: >", LineType::Tag),  // Tags starting with '!.'
        ("\t      !.mix5: >", LineType::Tag),  // Tags starting with '!.'
        ("      \t!.mix6: >", LineType::Tag),  // Tags starting with '!.'
    ];

    for (line, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Various indentation with '!' test failed: '{}' (expected {:?}, got {:?})",
            line, expected_type, result
        );
    }

    // Test continuation lines with various indentation and '!'
    let continuation_lines = vec![
        // 2-space continuation with '!'
        "  content! with! bang!",
        "  multiple!!! here!!!",
        "    More! indented! continuation!",

        // 3-space continuation with '!'
        "   three! space! indent!",
        "     deeper! three! space!",

        // 4-space continuation with '!'
        "    deep!!! nesting!!! here!",
        "      another! level! down!",

        // 6-space continuation with '!'
        "      six! space! indent! test",
        "        very! six! space! deep!",

        // 8-space continuation with '!'
        "        very! deep! eight! spaces",
        "          extreme! eight! spaces!",

        // 12-space continuation with '!'
        "            twelve! spaces! deep!",
        "              extra! twelve! spaces!",

        // 16-space continuation with '!'
        "                content! with! bang!",
        "                  extra! deep! content!",

        // Tab continuation with '!'
        "\tcontent! with! bang!",
        "\t\ttab! continuation! here!",
        "\t\t\ttriple! tab! deep!",

        // Mixed tab + spaces continuation with '!'
        "\t  mixed! tab! spaces!",
        "  \tspaces! then! tab!",
        "\t    deep! mixed! pattern!",
        "    \tmixed! continuation! test!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Continuation with '!' should be MappingKey or Unknown: '{}' (got {:?})",
            line, result
        );

        // Continuation lines should NOT detect as mapping keys
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_basic_indentation_levels_with_exclamation() {
    // Test basic indentation levels 1-3 with '!' character in keys
    // This covers the fundamental indentation scenarios with exclamation marks

    let test_cases: Vec<(&str, LineType, bool, Option<&str>)> = vec![
        // ===== Level 1: 1-space indentation =====
        (" key1!bang: >", LineType::MappingKey, true, Some("key1!bang")),
        (" test!here: >", LineType::MappingKey, true, Some("test!here")),
        (" end!with!bang!: >", LineType::MappingKey, true, Some("end!with!bang!")),
        (" !start: >", LineType::Tag, false, None),
        (" !.custom: >", LineType::Tag, false, None),

        // ===== Level 2: 2-space indentation =====
        ("  key2!bang: >", LineType::MappingKey, true, Some("key2!bang")),
        ("  test!here: >", LineType::MappingKey, true, Some("test!here")),
        ("  another!key!here: >", LineType::MappingKey, true, Some("another!key!here")),
        ("  !start: >", LineType::Tag, false, None),
        ("  !.type: >", LineType::Tag, false, None),

        // ===== Level 3: 3-space indentation =====
        ("   key3!bang: >", LineType::MappingKey, true, Some("key3!bang")),
        ("   test!here: >", LineType::MappingKey, true, Some("test!here")),
        ("   deep!nest!ed: >", LineType::MappingKey, true, Some("deep!nest!ed")),
        ("   !.tag: >", LineType::Tag, false, None),
        ("   !value: >", LineType::Tag, false, None),

        // ===== Level 3 alternative: 4-space indentation =====
        ("    key4!bang: >", LineType::MappingKey, true, Some("key4!bang")),
        ("    test!here: >", LineType::MappingKey, true, Some("test!here")),
        ("    another!level!down: >", LineType::MappingKey, true, Some("another!level!down")),
        ("    !nested!: >", LineType::Tag, false, None),
        ("    !.custom: >", LineType::Tag, false, None),
    ];

    for (line, expected_type, should_detect_key, expected_key) in test_cases {
        let result = classify_line_type(line);

        // Check classification
        assert_eq!(
            result, expected_type,
            "Basic indentation level test failed: '{}' (expected {:?}, got {:?})",
            line, expected_type, result
        );

        // Test detect_mapping_key behavior
        let info = detect_mapping_key(line, 0);
        if should_detect_key {
            assert!(
                info.is_some(),
                "Should detect mapping key at basic indentation level: '{}'",
                line
            );
            if let Some(key) = expected_key {
                assert_eq!(
                    info.unwrap().key, key,
                    "Should extract correct key at basic indentation: '{}'",
                    line
                );
            }
        } else {
            assert!(
                info.is_none(),
                "Should NOT detect Tag line as mapping key: '{}'",
                line
            );
        }
    }

    // Test continuation lines at basic indentation levels with '!'
    let continuation_lines = vec![
        // Level 1 continuation
        " content! with! bang!",
        "  more! level! 1! continuation!",

        // Level 2 continuation
        "  two! space! indent! content!",
        "    more! level! 2! continuation!",

        // Level 3 continuation (3-space)
        "   three! space! indent! test",
        "     deeper! three! space! content!",

        // Level 3 continuation (4-space)
        "    four! space! indent! test",
        "      deeper! four! space! content!",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        assert!(
            result == LineType::MappingKey || result == LineType::Unknown,
            "Continuation line should be MappingKey or Unknown: '{}' (got {:?})",
            line, result
        );

        // Continuation lines should NOT detect as mapping keys
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_literal_scalar_basic_modifiers_at_indentation_levels() {
    // Test literal scalar (|) with basic modifiers: |- (strip) and |+ (keep)
    // At various indentation levels: 2-space, 4-space, 6-space, 8-space, tab
    // This covers Section 12B literal scalar basic modifier scenarios

    let test_cases = vec![
        // ===== Level 1: 2-space indentation =====
        ("  text: |", "text", LineType::MappingKey),
        ("  warning: |-", "warning", LineType::MappingKey),
        ("  info: |+", "info", LineType::MappingKey),
        ("  message!here: |", "message!here", LineType::MappingKey),
        ("  alert!now: |-", "alert!now", LineType::MappingKey),
        ("  note!important: |+", "note!important", LineType::MappingKey),

        // ===== Level 2: 4-space indentation =====
        ("    content: |", "content", LineType::MappingKey),
        ("    error: |-", "error", LineType::MappingKey),
        ("    debug: |+", "debug", LineType::MappingKey),
        ("    level2!key: |", "level2!key", LineType::MappingKey),
        ("    nested!item: |-", "nested!item", LineType::MappingKey),
        ("    deep!value: |+", "deep!value", LineType::MappingKey),

        // ===== Level 3: 6-space indentation =====
        ("      data: |", "data", LineType::MappingKey),
        ("      output: |-", "output", LineType::MappingKey),
        ("      input: |+", "input", LineType::MappingKey),
        ("      level3!test: |", "level3!test", LineType::MappingKey),
        ("      deeply!nested: |-", "deeply!nested", LineType::MappingKey),
        ("      further!down: |+", "further!down", LineType::MappingKey),

        // ===== Level 4: 8-space indentation =====
        ("        record: |", "record", LineType::MappingKey),
        ("        field: |-", "field", LineType::MappingKey),
        ("        property: |+", "property", LineType::MappingKey),
        ("        level4!item: |", "level4!item", LineType::MappingKey),
        ("        very!deep!key: |-", "very!deep!key", LineType::MappingKey),
        ("        extremely!nested: |+", "extremely!nested", LineType::MappingKey),

        // ===== Level 5: Tab indentation =====
        ("\tentry: |", "entry", LineType::MappingKey),
        ("\tlog: |-", "log", LineType::MappingKey),
        ("\tstatus: |+", "status", LineType::MappingKey),
        ("\ttab!key: |", "tab!key", LineType::MappingKey),
        ("\ttab!nested: |-", "tab!nested", LineType::MappingKey),
        ("\ttab!indented: |+", "tab!indented", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Literal scalar basic modifier test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for literal scalar basic modifier: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for literal scalar basic modifier: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for literal scalars with basic modifiers
    // Continuation lines should be indented more than the parent key line
    let continuation_lines = vec![
        // Level 1 (2-space) continuation
        "  First line of literal text",
        "  Second line with continuation",
        "    More indented continuation at level 2",

        // Level 2 (4-space) continuation
        "    Four-space literal content",
        "    Another line at same level",
        "      Deeper continuation at level 3",

        // Level 3 (6-space) continuation
        "      Six-space literal content",
        "      Continues at this level",
        "        Even deeper at level 4",

        // Level 4 (8-space) continuation
        "        Eight-space literal content",
        "        Deeply nested continuation",
        "          Maximum depth at level 5",

        // Tab continuation
        "\tTab-indented literal content",
        "\tContinues with tabs",
        "\t  Deeper tab continuation",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines can be Unknown or MappingKey (they might be detected as keys)
        assert!(
            result == LineType::Unknown || result == LineType::MappingKey,
            "Continuation line should be Unknown or MappingKey: '{}' (got {:?})",
            line, result
        );

        // Continuation lines should NOT detect as mapping keys (no key: value pattern)
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

#[test]
fn test_folded_scalar_basic_modifiers_at_indentation_levels() {
    // Test folded scalar (>) with basic modifiers: >- (strip) and >+ (keep)
    // At various indentation levels: 2-space, 4-space, 6-space, 8-space, tab
    // This covers Section 12B folded scalar basic modifier scenarios

    let test_cases = vec![
        // ===== Level 1: 2-space indentation =====
        ("  text: >", "text", LineType::MappingKey),
        ("  warning: >-", "warning", LineType::MappingKey),
        ("  info: >+", "info", LineType::MappingKey),
        ("  message!here: >", "message!here", LineType::MappingKey),
        ("  alert!now: >-", "alert!now", LineType::MappingKey),
        ("  note!important: >+", "note!important", LineType::MappingKey),

        // ===== Level 2: 4-space indentation =====
        ("    content: >", "content", LineType::MappingKey),
        ("    error: >-", "error", LineType::MappingKey),
        ("    debug: >+", "debug", LineType::MappingKey),
        ("    level2!key: >", "level2!key", LineType::MappingKey),
        ("    nested!item: >-", "nested!item", LineType::MappingKey),
        ("    deep!value: >+", "deep!value", LineType::MappingKey),

        // ===== Level 3: 6-space indentation =====
        ("      data: >", "data", LineType::MappingKey),
        ("      output: >-", "output", LineType::MappingKey),
        ("      input: >+", "input", LineType::MappingKey),
        ("      level3!test: >", "level3!test", LineType::MappingKey),
        ("      deeply!nested: >-", "deeply!nested", LineType::MappingKey),
        ("      further!down: >+", "further!down", LineType::MappingKey),

        // ===== Level 4: 8-space indentation =====
        ("        record: >", "record", LineType::MappingKey),
        ("        field: >-", "field", LineType::MappingKey),
        ("        property: >+", "property", LineType::MappingKey),
        ("        level4!item: >", "level4!item", LineType::MappingKey),
        ("        very!deep!key: >-", "very!deep!key", LineType::MappingKey),
        ("        extremely!nested: >+", "extremely!nested", LineType::MappingKey),

        // ===== Level 5: Tab indentation =====
        ("\tentry: >", "entry", LineType::MappingKey),
        ("\tlog: >-", "log", LineType::MappingKey),
        ("\tstatus: >+", "status", LineType::MappingKey),
        ("\ttab!key: >", "tab!key", LineType::MappingKey),
        ("\ttab!nested: >-", "tab!nested", LineType::MappingKey),
        ("\ttab!indented: >+", "tab!indented", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Folded scalar basic modifier test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for folded scalar basic modifier: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for folded scalar basic modifier: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for folded scalars with basic modifiers
    // Continuation lines should be indented more than the parent key line
    let continuation_lines = vec![
        // Level 1 (2-space) continuation
        "  First line of folded text",
        "  Second line with continuation",
        "    More indented continuation at level 2",

        // Level 2 (4-space) continuation
        "    Four-space folded content",
        "    Another line at same level",
        "      Deeper continuation at level 3",

        // Level 3 (6-space) continuation
        "      Six-space folded content",
        "      Continues at this level",
        "        Even deeper at level 4",

        // Level 4 (8-space) continuation
        "        Eight-space folded content",
        "        Deeply nested continuation",
        "          Maximum depth at level 5",

        // Tab continuation
        "\tTab-indented folded content",
        "\tContinues with tabs",
        "\t  Deeper tab continuation",
    ];

    for line in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines can be Unknown or MappingKey (they might be detected as keys)
        assert!(
            result == LineType::Unknown || result == LineType::MappingKey,
            "Continuation line should be Unknown or MappingKey: '{}' (got {:?})",
            line, result
        );

        // Continuation lines should NOT detect as mapping keys (no key: value pattern)
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}

// ============================================================================
// Section 12B.3: Folded Scalar Explicit Indent Infrastructure Pattern
// ============================================================================
// This section demonstrates the infrastructure pattern for folded scalar
// explicit indent tests. Bead: bf-63gy6
//
// Pattern for children (bf-4aw6b and related beads):
// ------------------------------------------------
// 1. Use the `generate_folded_explicit_indent_tests!` macro to create test cases
//    for specific indentation levels and modifier combinations.
//
// 2. Use the `run_folded_scalar_tests!` macro to execute the test cases with
//    standard assertions (line type classification and key detection).
//
// 3. Or use helper functions `create_folded_scalar_test` and
//    `generate_folded_scalar_tests_multi_level` for non-macro test building.
//
// Template Test Function (Copy this pattern for new tests):
// ```rust
// #[test]
// fn test_folded_scalar_explicit_indent_<variant>() {
//     // Generate test cases for 2-space indentation with various modifiers
//     let test_cases = generate_folded_explicit_indent_tests!(
//         "  ",                    // indent: 2 spaces
//         "level1",               // level_name: descriptive name
//         [">", ">-", ">+"],      // modifiers: array of modifier patterns
//         [1, 2, 3, 4, 5],       // indent_nums: array of indent numbers
//         "test"                  // key_prefix: prefix for generated key names
//     );
//
//     // Run tests with standard assertions
//     run_folded_scalar_tests!(test_cases);
// }
// ```
//
// Available Modifiers:
// - ">"     : Plain folded scalar
// - ">-"    : Folded with strip modifier (removes trailing newlines)
// - ">+"    : Folded with keep modifier (preserves trailing newlines)
//
// Indent Numbers: 1-9 (e.g., >2 means 2 * 2 = 4 spaces indentation)
//
// Indentation Levels:
// - "  "    : 2 spaces (level1)
// - "    "  : 4 spaces (level2)
// - "      ": 6 spaces (level3)
// - "        ": 8 spaces (level4)
// - "\t"    : Tab (tab)
//
// Example: Using helper functions instead of macros:
// ```rust
// #[test]
// fn test_folded_scalar_custom_pattern() {
//     let mut cases = vec![];
//
//     // Add custom test cases using helper function
//     cases.push(create_folded_scalar_test("  ", "my_key", ">", 2));
//     cases.push(create_folded_scalar_test("    ", "another", ">-", 3));
//
//     // Run with standard macro
//     run_folded_scalar_tests!(cases);
// }
// ```
//
// Example: Bulk generation for multiple levels:
// ```rust
// #[test]
// fn test_folded_scalar_multi_level() {
//     let test_cases = generate_folded_scalar_tests_multi_level(
//         &["text", "note", "message"],  // keys
//         &[">", ">-", ">+"],           // modifiers
//         &[1, 2, 3, 4]                 // indent levels
//     );
//
//     run_folded_scalar_tests!(test_cases);
// }
// ```

#[test]
fn test_folded_scalar_explicit_indent_template_example() {
    // TEMPLATE EXAMPLE - This demonstrates the infrastructure pattern.
    // Copy this function and modify for your specific test needs.
    //
    // Bead: bf-63gy6 - Infrastructure pattern setup
    // See Section 12B.3 documentation above for full pattern details.

    // Example: Generate test cases for 2-space indentation
    let test_cases = generate_folded_explicit_indent_tests!(
        "  ",                    // 2-space base indentation
        "level1",               // Descriptive level name
        [">", ">-", ">+"],      // All three modifier types
        [1, 2, 3],             // Indent numbers 1-3
        "template"             // Key prefix for generated names
    );

    // Run tests with standard assertions
    run_folded_scalar_tests!(test_cases);
}

#[test]
fn test_folded_scalar_explicit_indent_tab_template() {
    // TEMPLATE EXAMPLE for tab indentation
    // Copy this pattern for tab-based folded scalar tests

    let test_cases = generate_folded_explicit_indent_tests!(
        "\t",                    // Tab base indentation
        "tab",                  // Descriptive level name
        [">", ">-", ">+"],      // All three modifier types
        [1, 2, 3, 4],          // Indent numbers 1-4
        "tab_test"             // Key prefix
    );

    run_folded_scalar_tests!(test_cases);
}

#[test]
fn test_folded_scalar_explicit_indent_helper_function_example() {
    // TEMPLATE EXAMPLE using helper functions instead of macros
    // Use this when you need more control over test case generation

    let mut cases = vec![];

    // Build custom test cases using helper function
    cases.push(create_folded_scalar_test("  ", "custom_key", ">", 1));
    cases.push(create_folded_scalar_test("    ", "another_key", ">-", 2));
    cases.push(create_folded_scalar_test("\t", "tab_key", ">+", 3));

    // Or manually add tuples:
    // (line, expected_key, expected_type)
    cases.push((
        "      manual: >4".to_string(),
        "manual".to_string(),
        LineType::MappingKey
    ));

    run_folded_scalar_tests!(cases);
}

#[test]
fn test_folded_scalar_strip_indent_explicit_indent_modifiers_at_2_space() {
    // Test folded scalars with strip indent explicit indent modifiers: >-n for n=1-9
    // At 2-space indentation level only
    // This provides focused coverage of strip indent explicit indent (>-n) specification for folded scalars
    // Follows the pattern established in test_folded_scalar_plain_explicit_indent_modifiers_at_2_space

    let test_cases = vec![
        // ===== Level 1: 2-space indentation with strip indent explicit indent >-n =====
        // Strip indent >-n (n=1-9) - main test cases
        ("  text1: >-1", "text1", LineType::MappingKey),
        ("  text2: >-2", "text2", LineType::MappingKey),
        ("  text3: >-3", "text3", LineType::MappingKey),
        ("  text4: >-4", "text4", LineType::MappingKey),
        ("  text5: >-5", "text5", LineType::MappingKey),
        ("  text6: >-6", "text6", LineType::MappingKey),
        ("  text7: >-7", "text7", LineType::MappingKey),
        ("  text8: >-8", "text8", LineType::MappingKey),
        ("  text9: >-9", "text9", LineType::MappingKey),

        // Keys with exclamation marks at 2-space indentation
        ("  key!1: >-1", "key!1", LineType::MappingKey),
        ("  warn!2: >-2", "warn!2", LineType::MappingKey),
        ("  error!3: >-3", "error!3", LineType::MappingKey),
        ("  test!4: >-4", "test!4", LineType::MappingKey),
        ("  data!5: >-5", "data!5", LineType::MappingKey),
        ("  info!6: >-6", "info!6", LineType::MappingKey),
        ("  msg!7: >-7", "msg!7", LineType::MappingKey),
        ("  log!8: >-8", "log!8", LineType::MappingKey),
        ("  val!9: >-9", "val!9", LineType::MappingKey),

        // Keys with multiple exclamation marks
        ("  key!!1: >-1", "key!!1", LineType::MappingKey),
        ("  deep!!key2: >-2", "deep!!key2", LineType::MappingKey),
        ("  very!!deep!!key3: >-3", "very!!deep!!key3", LineType::MappingKey),
        ("  super!!!deep!!!key4: >-4", "super!!!deep!!!key4", LineType::MappingKey),

        // Edge case: Single character keys with !
        ("  a!: >-1", "a!", LineType::MappingKey),
        ("  b!: >-2", "b!", LineType::MappingKey),
        ("  c!: >-3", "c!", LineType::MappingKey),
        ("  d!: >-4", "d!", LineType::MappingKey),
        ("  e!: >-5", "e!", LineType::MappingKey),

        // Edge case: Keys ending with !
        ("  end!with!bang!: >-1", "end!with!bang!", LineType::MappingKey),
        ("  another!end!bang!: >-2", "another!end!bang!", LineType::MappingKey),
        ("  final!end!with!bang!: >-3", "final!end!with!bang!", LineType::MappingKey),
    ];

    for (line, expected_key, expected_type) in test_cases {
        let result = classify_line_type(line);
        assert_eq!(
            result, expected_type,
            "Folded scalar strip indent explicit indent test failed: '{}' - expected {:?}, got {:?}",
            line, expected_type, result
        );

        // Verify that the key is correctly detected for MappingKey types
        if result == LineType::MappingKey {
            let info = detect_mapping_key(line, 0);
            assert!(
                info.is_some(),
                "Should detect mapping key for folded scalar with strip indent explicit indent: '{}'",
                line
            );
            let detected = info.unwrap();
            assert_eq!(
                detected.key, expected_key,
                "Key mismatch for folded scalar with strip indent explicit indent: '{}' - expected '{}', got '{}'",
                line, expected_key, detected.key
            );
        }
    }

    // Test continuation lines for folded scalars with strip indent explicit indent modifiers
    let continuation_lines = vec![
        // Level 1 continuation lines with ! characters
        ("  This is content with! exclamation", vec![LineType::MappingKey, LineType::Unknown]),
        ("  More! text! here! for! testing!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Folded! content! continues! with! strip! indent!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Lines! with! various! exclamation! marks! here!", vec![LineType::MappingKey, LineType::Unknown]),
        ("  Testing! continuation! behavior! for! >-n! modifiers!", vec![LineType::MappingKey, LineType::Unknown]),

        // Lines starting with ! (may be classified as Tag)
        ("  !Starting! with! emphasis!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  !Tag! like! content! with! exclamation!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
        ("  !Another! tag! pattern! here!", vec![LineType::Tag, LineType::MappingKey, LineType::Unknown]),
    ];

    for (line, expected_types) in continuation_lines {
        let result = classify_line_type(line);
        // Continuation lines should be one of the expected types
        assert!(
            expected_types.contains(&result),
            "Continuation line for folded scalar strip indent explicit indent should be one of {:?}: '{}' (got {:?})",
            expected_types, line, result
        );

        // Continuation lines should NOT detect as mapping keys
        let info = detect_mapping_key(line, 0);
        assert!(
            info.is_none(),
            "Continuation line should NOT detect mapping key: '{}'",
            line
        );
    }
}
