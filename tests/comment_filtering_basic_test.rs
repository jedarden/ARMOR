//! Basic YAML comment filtering tests
//!
//! These tests verify basic YAML comment filtering patterns including:
//! - Full-line comments (lines starting with #)
//! - Inline comments (# followed by text after content)
//! - Empty/whitespace-only lines
//!
//! Bead: bf-4i5yj
//! Acceptance Criteria:
//! - Test function for full-line comment removal
//! - Test function for inline comment filtering
//! - Test function for empty line handling
//! - All tests pass

use armor::parsers::yaml::{
    classify_line_type, strip_inline_comment, is_comment_line, calculate_indentation, LineType
};

// Full-line comment tests

#[test]
fn test_full_line_comment_detection() {
    // Basic full-line comment
    assert_eq!(classify_line_type("# This is a comment"), LineType::Comment);

    // Comment with leading whitespace
    assert_eq!(classify_line_type("  # indented comment"), LineType::Comment);

    // Tab-indented comment
    assert_eq!(classify_line_type("\t# tab comment"), LineType::Comment);

    // Comment with just hash
    assert_eq!(classify_line_type("#"), LineType::Comment);

    // Comment with special characters
    assert_eq!(classify_line_type("# TODO: fix this bug"), LineType::Comment);
    assert_eq!(classify_line_type("# NOTE: important information"), LineType::Comment);
    assert_eq!(classify_line_type("# FIXME: broken code"), LineType::Comment);
}

#[test]
fn test_full_line_comment_with_various_indentation() {
    // No indentation
    assert_eq!(classify_line_type("# comment"), LineType::Comment);

    // Space indentation
    assert_eq!(classify_line_type(" # comment"), LineType::Comment);
    assert_eq!(classify_line_type("  # comment"), LineType::Comment);
    assert_eq!(classify_line_type("    # comment"), LineType::Comment);
    assert_eq!(classify_line_type("      # comment"), LineType::Comment);
    assert_eq!(classify_line_type("        # comment"), LineType::Comment);

    // Tab indentation
    assert_eq!(classify_line_type("\t# comment"), LineType::Comment);
    assert_eq!(classify_line_type("\t\t# comment"), LineType::Comment);

    // Mixed whitespace
    assert_eq!(classify_line_type(" \t# comment"), LineType::Comment);
    assert_eq!(classify_line_type("\t # comment"), LineType::Comment);
    assert_eq!(classify_line_type("  \t  # comment"), LineType::Comment);
}

#[test]
fn test_full_line_comment_not_content_lines() {
    // Lines with inline comments are NOT full-line comments
    assert_ne!(classify_line_type("key: value # comment"), LineType::Comment);
    assert_ne!(classify_line_type("key: value#comment"), LineType::Comment);
    assert_ne!(classify_line_type("- item # comment"), LineType::Comment);
    assert_ne!(classify_line_type("  key: value # inline"), LineType::Comment);
}

#[test]
fn test_full_line_comment_helper_function() {
    // Test the is_comment_line helper function
    assert!(is_comment_line("# comment"));
    assert!(is_comment_line("  # comment"));
    assert!(is_comment_line("\t# comment"));
    assert!(is_comment_line("#"));
    assert!(is_comment_line("  #"));

    // These should NOT be full-line comments
    assert!(!is_comment_line("key: value"));
    assert!(!is_comment_line("key: value # comment"));
    assert!(!is_comment_line(""));
    assert!(!is_comment_line("   "));
}

// Inline comment tests

#[test]
fn test_inline_comment_basic_removal() {
    // Basic inline comment with space before #
    assert_eq!(strip_inline_comment("key: value # comment"), "key: value ");

    // Comment at end of line
    assert_eq!(strip_inline_comment("name: John # this is name"), "name: John ");

    // Multiple spaces before comment
    assert_eq!(strip_inline_comment("key: value    # comment"), "key: value    ");

    // Tab before comment
    assert_eq!(strip_inline_comment("key: value\t# comment"), "key: value\t");
}

#[test]
fn test_inline_comment_hash_without_whitespace_is_part_of_value() {
    // Hash without preceding whitespace is part of the value
    assert_eq!(strip_inline_comment("key: value#comment"), "key: value#comment");
    assert_eq!(strip_inline_comment("key: value#hash#in#value"), "key: value#hash#in#value");

    // But hash preceded by whitespace starts comment
    assert_eq!(strip_inline_comment("key: value # comment"), "key: value ");
}

#[test]
fn test_inline_comment_preserves_urls_with_hashes() {
    // Hash in URL should be preserved (not treated as comment)
    assert_eq!(strip_inline_comment("url: http://example.com#anchor"), "url: http://example.com#anchor");
    assert_eq!(strip_inline_comment("url: https://example.com/path#section"), "url: https://example.com/path#section");
    assert_eq!(strip_inline_comment("link: ftp://files.example.com#dir"), "link: ftp://files.example.com#dir");

    // URL with hash followed by actual comment
    assert_eq!(strip_inline_comment("url: http://example.com#anchor # this is comment"), "url: http://example.com#anchor ");
}

#[test]
fn test_inline_comment_preserves_quoted_hashes() {
    // Hash in quoted string should be preserved
    assert_eq!(strip_inline_comment("key: \"value with # hash\""), "key: \"value with # hash\"");
    assert_eq!(strip_inline_comment("key: 'value with # hash'"), "key: 'value with # hash'");

    // Double quotes
    assert_eq!(strip_inline_comment("key: \"value #1\" # comment"), "key: \"value #1\" ");

    // Single quotes
    assert_eq!(strip_inline_comment("key: 'value #2' # comment"), "key: 'value #2' ");

    // Mixed quotes
    assert_eq!(strip_inline_comment("key: \"double 'inner' # hash\" # comment"), "key: \"double 'inner' # hash\" ");
    assert_eq!(strip_inline_comment("key: 'single \"inner\" # hash' # comment"), "key: 'single \"inner\" # hash' ");
}

#[test]
fn test_inline_comment_with_indented_lines() {
    // Indented lines with inline comments
    assert_eq!(strip_inline_comment("  key: value # comment"), "  key: value ");
    assert_eq!(strip_inline_comment("    nested: value # inline"), "    nested: value ");
    assert_eq!(strip_inline_comment("\tkey: value # comment"), "\tkey: value ");
    assert_eq!(strip_inline_comment("  \t key: value # comment"), "  \t key: value ");
}

#[test]
fn test_inline_comment_no_comment_present() {
    // Lines without comments should remain unchanged
    assert_eq!(strip_inline_comment("key: value"), "key: value");
    assert_eq!(strip_inline_comment("  key: value  "), "  key: value  ");
    assert_eq!(strip_inline_comment("name: John Doe"), "name: John Doe");
    assert_eq!(strip_inline_comment("- item in list"), "- item in list");
    assert_eq!(strip_inline_comment("just plain text"), "just plain text");
}

#[test]
fn test_inline_comment_complex_real_world_examples() {
    // Database connection string with hash
    let line = "database: \"postgresql://localhost:5432/db#schema\" # production database";
    assert_eq!(strip_inline_comment(line), "database: \"postgresql://localhost:5432/db#schema\" ");

    // API endpoint with hash and comment
    let line = "api: \"https://api.example.com/v1#endpoint\" # production API";
    assert_eq!(strip_inline_comment(line), "api: \"https://api.example.com/v1#endpoint\" ");

    // Configuration with comment
    let line = "timeout: 30 # seconds before timeout";
    assert_eq!(strip_inline_comment(line), "timeout: 30 ");
}

// Empty line tests

#[test]
fn test_empty_line_detection() {
    // Empty line (no characters)
    assert_eq!(classify_line_type(""), LineType::Blank);

    // Whitespace-only lines
    assert_eq!(classify_line_type("   "), LineType::Blank);
    assert_eq!(classify_line_type("\t\t"), LineType::Blank);
    assert_eq!(classify_line_type(" \t "), LineType::Blank);
    assert_eq!(classify_line_type("  \t  "), LineType::Blank);
    assert_eq!(classify_line_type("\t \t"), LineType::Blank);

    // Various amounts of whitespace
    assert_eq!(classify_line_type(" "), LineType::Blank);
    assert_eq!(classify_line_type("  "), LineType::Blank);
    assert_eq!(classify_line_type("    "), LineType::Blank);
    assert_eq!(classify_line_type("\t"), LineType::Blank);
    assert_eq!(classify_line_type("\t\t\t"), LineType::Blank);
}

#[test]
fn test_empty_line_indentation_calculation() {
    // Empty line has zero indentation
    assert_eq!(calculate_indentation(""), 0);

    // Whitespace-only lines count their indentation
    assert_eq!(calculate_indentation("   "), 3);
    assert_eq!(calculate_indentation("\t\t"), 2);
    assert_eq!(calculate_indentation(" \t "), 3);
    assert_eq!(calculate_indentation("  \t  "), 5);
    assert_eq!(calculate_indentation("\t \t"), 3);
}

#[test]
fn test_empty_line_vs_content_line() {
    // Empty lines are classified as Blank
    assert_eq!(classify_line_type(""), LineType::Blank);
    assert_eq!(classify_line_type("   "), LineType::Blank);

    // Content lines are NOT Blank
    assert_ne!(classify_line_type("key: value"), LineType::Blank);
    assert_ne!(classify_line_type("# comment"), LineType::Blank);
    assert_ne!(classify_line_type("- item"), LineType::Blank);
}

#[test]
fn test_empty_line_not_confused_with_other_types() {
    // Empty lines should not be classified as comments
    assert_ne!(classify_line_type(""), LineType::Comment);
    assert_ne!(classify_line_type("   "), LineType::Comment);

    // Empty lines should not be classified as content
    assert_ne!(classify_line_type(""), LineType::MappingKey);
    assert_ne!(classify_line_type("   "), LineType::SequenceItem);

    // But whitespace with # is a comment, not blank
    assert_eq!(classify_line_type(" #"), LineType::Comment);
    assert_eq!(classify_line_type("  # comment"), LineType::Comment);
}

// Integration tests combining all three patterns

#[test]
fn test_comment_filtering_integration_complete_yaml_document() {
    let yaml_lines = vec![
        "",                                              // line 1: empty
        "# Configuration file",                           // line 2: full-line comment
        "database: postgres",                            // line 3: content
        "",                                              // line 4: empty
        "  # Database settings",                          // line 5: full-line comment (indented)
        "  host: localhost",                              // line 6: content (indented)
        "  port: 5432 # default PostgreSQL port",        // line 7: content with inline comment
        "",                                              // line 8: empty
        "# TODO: add SSL settings",                      // line 9: full-line comment
        "ssl: false # disabled for now",                // line 10: content with inline comment
        "",                                              // line 11: empty
    ];

    // Test line classification
    assert_eq!(classify_line_type(yaml_lines[0]), LineType::Blank);
    assert_eq!(classify_line_type(yaml_lines[1]), LineType::Comment);
    assert_eq!(classify_line_type(yaml_lines[2]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml_lines[3]), LineType::Blank);
    assert_eq!(classify_line_type(yaml_lines[4]), LineType::Comment);
    assert_eq!(classify_line_type(yaml_lines[5]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml_lines[6]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml_lines[7]), LineType::Blank);
    assert_eq!(classify_line_type(yaml_lines[8]), LineType::Comment);
    assert_eq!(classify_line_type(yaml_lines[9]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml_lines[10]), LineType::Blank);

    // Test inline comment stripping
    assert_eq!(strip_inline_comment(yaml_lines[6]), "  port: 5432 ");
    assert_eq!(strip_inline_comment(yaml_lines[9]), "ssl: false ");
}

#[test]
fn test_comment_filtering_edge_cases_hash_variations() {
    // Hash at start of line (full comment)
    assert_eq!(classify_line_type("#"), LineType::Comment);
    assert_eq!(strip_inline_comment("#"), "");

    // Hash with just whitespace after
    assert_eq!(classify_line_type("# "), LineType::Comment);
    assert_eq!(strip_inline_comment("# "), "");

    // Multiple hashes
    assert_eq!(classify_line_type("## comment"), LineType::Comment);
    assert_eq!(strip_inline_comment("## comment"), "");

    // Hash in the middle (inline comment)
    assert_eq!(classify_line_type("key: value # comment"), LineType::MappingKey);
    assert_eq!(strip_inline_comment("key: value # comment"), "key: value ");

    // Hash at end without space (part of value)
    assert_eq!(classify_line_type("key: value#"), LineType::MappingKey);
    assert_eq!(strip_inline_comment("key: value#"), "key: value#");
}

#[test]
fn test_comment_filtering_preserves_structure_and_content() {
    // Test that comment filtering preserves important YAML structure

    // Sequence with comments
    let yaml = vec![
        "- item1 # first item",
        "- item2",
        "  # nested comment",
        "  - subitem",
        "- item3 # last item",
    ];

    // All lines should be classified as sequence items except the comment
    assert_eq!(classify_line_type(yaml[0]), LineType::SequenceItem);
    assert_eq!(strip_inline_comment(yaml[0]), "- item1 ");

    assert_eq!(classify_line_type(yaml[1]), LineType::SequenceItem);

    assert_eq!(classify_line_type(yaml[2]), LineType::Comment);

    assert_eq!(classify_line_type(yaml[3]), LineType::SequenceItem);

    assert_eq!(classify_line_type(yaml[4]), LineType::SequenceItem);
    assert_eq!(strip_inline_comment(yaml[4]), "- item3 ");
}

#[test]
fn test_comment_filtering_with_nested_structures() {
    // Nested mapping with various comment patterns
    let yaml = vec![
        "parent:",                          // parent key
        "  # child section",                // full-line comment
        "  child1: value1",                 // nested key
        "  child2: value2 # with comment",  // nested key with inline comment
        "  ",                               // whitespace-only line
        "  nested:",                        // nested parent key
        "    deeply: value # deep comment", // deeply nested with comment
    ];

    // Test classifications
    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[1]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[2]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[3]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[4]), LineType::Blank);
    assert_eq!(classify_line_type(yaml[5]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[6]), LineType::MappingKey);

    // Test inline comment stripping
    assert_eq!(strip_inline_comment(yaml[3]), "  child2: value2 ");
    assert_eq!(strip_inline_comment(yaml[6]), "    deeply: value ");

    // Test indentation calculations
    assert_eq!(calculate_indentation(yaml[0]), 0);
    assert_eq!(calculate_indentation(yaml[1]), 2);
    assert_eq!(calculate_indentation(yaml[2]), 2);
    assert_eq!(calculate_indentation(yaml[3]), 2);
    assert_eq!(calculate_indentation(yaml[4]), 2);
    assert_eq!(calculate_indentation(yaml[5]), 2);
    assert_eq!(calculate_indentation(yaml[6]), 4);
}