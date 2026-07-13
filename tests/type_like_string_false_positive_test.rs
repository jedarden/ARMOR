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
