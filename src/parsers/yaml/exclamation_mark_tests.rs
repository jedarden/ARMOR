//! Comprehensive tests for exclamation mark classification in YAML line parser
//!
//! This test module verifies that exclamation marks (!) are correctly classified
//! in different contexts within YAML files.

use crate::parsers::yaml::line_parser::{classify_line_type, detect_mapping_key, LineType};

#[test]
fn test_exclamation_mark_in_full_comment_classified_as_comment() {
    // Exclamation marks in full-line comments should be classified as Comment, not Tag
    assert_eq!(classify_line_type("# This is a comment!"), LineType::Comment,
        "Comments with ! should be classified as Comment");

    assert_eq!(classify_line_type("# TODO: Fix this bug!"), LineType::Comment,
        "TODO comments with ! should be classified as Comment");

    assert_eq!(classify_line_type("# !important"), LineType::Comment,
        "Comments starting with ! after # should be classified as Comment");

    assert_eq!(classify_line_type("# Note: This is !critical"), LineType::Comment,
        "Comments with ! in middle should be classified as Comment");

    assert_eq!(classify_line_type("  # Indented comment!"), LineType::Comment,
        "Indented comments with ! should be classified as Comment");
}

#[test]
fn test_exclamation_mark_at_end_of_value_not_tag() {
    // Exclamation marks at the end of values should not trigger Tag classification
    // These should be classified as MappingKey since they contain colons

    assert_eq!(classify_line_type("key: value!"), LineType::MappingKey,
        "Value ending with ! should be MappingKey, not Tag");

    assert_eq!(classify_line_type("priority: high!"), LineType::MappingKey,
        "Priority value with ! should be MappingKey");

    assert_eq!(classify_line_type("status: active!"), LineType::MappingKey,
        "Status value with ! should be MappingKey");

    assert_eq!(classify_line_type("  nested: value!"), LineType::MappingKey,
        "Nested value ending with ! should be MappingKey");
}

#[test]
fn test_exclamation_mark_in_quoted_strings() {
    // Exclamation marks in quoted strings should be handled correctly
    // The line should be classified based on the overall structure, not the ! inside quotes

    assert_eq!(classify_line_type("key: \"value!\""), LineType::MappingKey,
        "Quoted value with ! should be MappingKey");

    assert_eq!(classify_line_type("key: 'value!'"), LineType::MappingKey,
        "Single-quoted value with ! should be MappingKey");

    assert_eq!(classify_line_type("message: \"Hello! World!\""), LineType::MappingKey,
        "Quoted string with multiple ! should be MappingKey");

    assert_eq!(classify_line_type("text: '!!!'"), LineType::MappingKey,
        "Quoted string with only ! should be MappingKey");

    assert_eq!(classify_line_type("url: \"http://example.com#!anchor\""), LineType::MappingKey,
        "Quoted URL with ! should be MappingKey");
}

#[test]
fn test_exclamation_mark_at_line_start_is_tag() {
    // Lines starting with ! should be classified as Tag
    // This is the correct YAML tag syntax

    assert_eq!(classify_line_type("!tag"), LineType::Tag,
        "Line starting with ! should be Tag");

    assert_eq!(classify_line_type("!my_tag"), LineType::Tag,
        "Tag with underscore should be Tag");

    assert_eq!(classify_line_type("!yaml.org/types:str"), LineType::Tag,
        "Full URI tag should be Tag");

    assert_eq!(classify_line_type("  !indented_tag"), LineType::Tag,
        "Indented tag should be Tag");

    assert_eq!(classify_line_type("!"), LineType::Tag,
        "Lone ! should be Tag");
}

#[test]
fn test_exclamation_mark_in_sequence_items() {
    // Exclamation marks in sequence items should be handled correctly

    assert_eq!(classify_line_type("- item!"), LineType::SequenceItem,
        "Sequence item with ! should be SequenceItem");

    assert_eq!(classify_line_type("- key: value!"), LineType::SequenceItem,
        "Sequence item mapping with value ending in ! should be SequenceItem");

    assert_eq!(classify_line_type("  - nested!"), LineType::SequenceItem,
        "Indented sequence item with ! should be SequenceItem");
}

#[test]
fn test_exclamation_mark_inline_comments() {
    // Inline comments with ! should have the ! preserved in the value part

    // key: value! # inline comment - should detect the key correctly
    let info = detect_mapping_key("key: value! # inline comment", 0);
    assert!(info.is_some(), "Should detect key with ! in value and inline comment");
    let info = info.unwrap();
    assert_eq!(info.key, "key");
    assert_eq!(info.value, Some("value!".to_string()));

    // key: !value # comment - should handle ! at start of value
    let info = detect_mapping_key("priority: !high # comment", 0);
    assert!(info.is_some(), "Should detect key with ! starting value and inline comment");
    let info = info.unwrap();
    assert_eq!(info.key, "priority");
    assert_eq!(info.value, Some("!high".to_string()));
}

#[test]
fn test_exclamation_mark_edge_cases() {
    // Various edge cases with exclamation marks

    // Empty line with just ! (edge case)
    assert_eq!(classify_line_type("!"), LineType::Tag,
        "Lone ! should be Tag");

    // Multiple exclamation marks at start
    assert_eq!(classify_line_type("!!"), LineType::Tag,
        "Double !! should be Tag (YAML tag prefix)");

    assert_eq!(classify_line_type("!!!tag"), LineType::Tag,
        "Triple !!! should be Tag (YAML local tag prefix)");

    // ! in the middle of a value
    assert_eq!(classify_line_type("key: value!more"), LineType::MappingKey,
        "! in middle of value should be MappingKey");

    // ! before colon (this would be unusual but should not crash)
    assert_eq!(classify_line_type("key!: value"), LineType::MappingKey,
        "! before colon should be MappingKey");
}

#[test]
fn test_exclamation_mark_in_parent_keys() {
    // Parent keys (keys without values) with ! should be handled correctly

    let info = detect_mapping_key("section!:", 0);
    assert!(info.is_some(), "Parent key ending with ! should be detected");
    let info = info.unwrap();
    assert_eq!(info.key, "section!");
    assert!(info.is_parent_key);

    let info = detect_mapping_key("nested!:", 0);
    assert!(info.is_some(), "Nested parent key with ! should be detected");
    let info = info.unwrap();
    assert_eq!(info.key, "nested!");
    assert!(info.is_parent_key);
}

#[test]
fn test_exclamation_mark_in_document_markers_and_specials() {
    // Exclamation marks should not interfere with other YAML constructs

    assert_eq!(classify_line_type("---"), LineType::DocumentStart,
        "Document start marker should not be affected by context with !");

    assert_eq!(classify_line_type("..."), LineType::DocumentEnd,
        "Document end marker should be DocumentEnd");

    assert_eq!(classify_line_type("%YAML 1.2"), LineType::Directive,
        "YAML directive should be Directive");

    assert_eq!(classify_line_type("&anchor"), LineType::Anchor,
        "Anchor should be Anchor");

    assert_eq!(classify_line_type("*alias"), LineType::Alias,
        "Alias should be Alias");

    assert_eq!(classify_line_type("|"), LineType::LiteralBlockScalar,
        "Literal block scalar should be LiteralBlockScalar");

    assert_eq!(classify_line_type(">"), LineType::FoldedBlockScalar,
        "Folded block scalar should be FoldedBlockScalar");
}

#[test]
fn test_exclamation_mark_comprehensive_real_world_examples() {
    // Test real-world YAML examples with exclamation marks

    // Configuration file with emphasis
    assert_eq!(classify_line_type("production: true!"), LineType::MappingKey,
        "Production flag with ! should be MappingKey");

    // Comments with urgency markers
    assert_eq!(classify_line_type("# FIXME: This needs attention!"), LineType::Comment,
        "FIXME comment with ! should be Comment");

    // Tag usage in YAML schemas
    assert_eq!(classify_line_type("!type definition"), LineType::Tag,
        "Type tag should be Tag");

    // Values with emoticons/emphasis
    assert_eq!(classify_line_type("message: Hello!!!"), LineType::MappingKey,
        "Message with trailing !!! should be MappingKey");

    // URL-like values
    assert_eq!(classify_line_type("link: http://example.com!"), LineType::MappingKey,
        "URL with ! should be MappingKey");

    // Complex nested structure
    assert_eq!(classify_line_type("  config: value!"), LineType::MappingKey,
        "Nested config with ! should be MappingKey");
}

#[test]
fn test_exclamation_mark_classification_order_matters() {
    // Verify that classification happens in the correct order
    // Comments should be checked before tags to avoid misclassification

    // This should be Comment, not Tag (because # is checked first)
    assert_eq!(classify_line_type("# !tag"), LineType::Comment,
        "! after # should be Comment, not Tag (order matters)");

    // This should be Tag (no # prefix)
    assert_eq!(classify_line_type("!tag"), LineType::Tag,
        "! at start should be Tag");

    // This should be MappingKey (contains :)
    assert_eq!(classify_line_type("key: !value"), LineType::MappingKey,
        "! in value position after : should be MappingKey");
}

#[test]
fn test_exclamation_mark_with_various_indentation_levels() {
    // Exclamation marks with different indentation levels

    for spaces in 0..10 {
        let indent = " ".repeat(spaces);

        // Comment with !
        let comment_line = format!("{}# comment!", indent);
        assert_eq!(classify_line_type(&comment_line), LineType::Comment,
            "Comment with ! at {} spaces should be Comment", spaces);

        // Value ending with !
        let value_line = format!("{}key: value!", indent);
        assert_eq!(classify_line_type(&value_line), LineType::MappingKey,
            "Value with ! at {} spaces should be MappingKey", spaces);

        // Tag at various indents
        let tag_line = format!("{}!tag", indent);
        assert_eq!(classify_line_type(&tag_line), LineType::Tag,
            "Tag at {} spaces should be Tag", spaces);
    }
}
