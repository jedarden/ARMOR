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
// Section 12: Integration - Detect Mapping Key with False Positives
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
// Section 13: Complex Real-World Scenarios
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

// ============================================================================
// Section 14: Error Code-like Strings in Values
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
