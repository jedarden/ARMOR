//! Test suite for type-like strings that aren't actual types
//!
//! This test suite verifies that the YAML parser correctly handles strings
//! that look like types or tags but shouldn't be classified as such:
//! - Strings starting with `!` that aren't YAML tags (comments, values, etc.)
//! - False positive scenarios in tag/type detection
//! - Strings that resemble type names in error messages
//! - Edge cases where classification might be incorrect

use armor::parsers::yaml::{classify_line_type, detect_mapping_key, LineType};

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

// ============================================================================
// Section 9: Special YAML Tag Patterns vs False Positives
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
fn test_invalid_tag_patterns() {
    // Patterns that look like tags but aren't valid YAML
    let test_cases = vec![
        "!123",              // Tag with only numbers (unusual)
        "!$",               // Tag with special char
        "!@tag",            // Tag starting with @ (invalid)
        "! space",          // Tag with space immediately after
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

// ============================================================================
// Section 10: Whitespace and Exclamation Combinations
// ============================================================================

#[test]
fn test_whitespace_before_exclamation() {
    // Various whitespace patterns before !
    let test_cases = vec![
        "key : value!",     // Space before colon
        "key: value !",     // Space before !
        "key:  value!",     // Multiple spaces
        "key:\tvalue!",     // Tab separator
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

// ============================================================================
// Section 11: Integration - Detect Mapping Key with False Positives
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
