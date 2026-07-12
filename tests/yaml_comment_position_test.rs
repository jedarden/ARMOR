//! YAML Comments at Various Line Positions Tests
//!
//! These tests verify YAML comment detection and filtering when comments
//! appear at different positions within lines.
//!
//! Bead: bf-5muf6
//! Acceptance Criteria:
//! - Test verifies start-of-line comment detection
//! - Test verifies middle-of-line comment detection
//! - Test verifies end-of-line comment detection
//! - Test handles multiple # symbols correctly (only first # preceded by whitespace starts comment)
//! - All new tests pass

use armor::parsers::yaml::{
    classify_line_type, strip_inline_comment, is_comment_line, LineType
};

// Comments at Start of Line Tests

#[test]
fn test_comment_at_start_of_line_basic() {
    // Comment at the very start of line (no leading whitespace)
    assert_eq!(classify_line_type("# comment"), LineType::Comment);
    assert!(is_comment_line("# comment"));
    assert_eq!(strip_inline_comment("# comment"), "");
}

#[test]
fn test_comment_at_start_of_line_various_content() {
    // Comments at start with various content patterns
    assert_eq!(classify_line_type("# TODO: fix this"), LineType::Comment);
    assert_eq!(classify_line_type("# FIXME: broken code"), LineType::Comment);
    assert_eq!(classify_line_type("# NOTE: important"), LineType::Comment);
    assert_eq!(classify_line_type("# WARNING: dangerous"), LineType::Comment);
    assert_eq!(classify_line_type("# INFO: reference"), LineType::Comment);

    // All should be identified as comment lines
    assert!(is_comment_line("# TODO: fix this"));
    assert!(is_comment_line("# FIXME: broken code"));
    assert!(is_comment_line("# NOTE: important"));

    // All should strip to empty string
    assert_eq!(strip_inline_comment("# TODO: fix this"), "");
    assert_eq!(strip_inline_comment("# FIXME: broken code"), "");
    assert_eq!(strip_inline_comment("# NOTE: important"), "");
}

#[test]
fn test_comment_at_start_of_line_with_colons() {
    // Comments at start with colons (common in TODO/FIXME comments)
    assert_eq!(classify_line_type("# TODO: implement feature X"), LineType::Comment);
    assert_eq!(classify_line_type("# FIXME: bug in module Y"), LineType::Comment);
    assert_eq!(classify_line_type("# NOTE: this is important: very important"), LineType::Comment);
    assert_eq!(classify_line_type("# REVIEW: check: this, that, and the other"), LineType::Comment);

    // All should be comment lines, NOT mapping keys
    assert!(is_comment_line("# TODO: implement feature X"));
    assert!(is_comment_line("# FIXME: bug in module Y"));
    assert!(is_comment_line("# NOTE: this is important: very important"));

    // Should strip completely
    assert_eq!(strip_inline_comment("# TODO: implement feature X"), "");
    assert_eq!(strip_inline_comment("# FIXME: bug in module Y"), "");
}

#[test]
fn test_comment_at_start_after_leading_whitespace() {
    // Comment at start after whitespace indentation
    assert_eq!(classify_line_type("  # comment"), LineType::Comment);
    assert_eq!(classify_line_type("    # indented comment"), LineType::Comment);
    assert_eq!(classify_line_type("\t# tab comment"), LineType::Comment);
    assert_eq!(classify_line_type("  \t  # mixed whitespace comment"), LineType::Comment);

    // All should be identified as comment lines
    assert!(is_comment_line("  # comment"));
    assert!(is_comment_line("    # indented comment"));
    assert!(is_comment_line("\t# tab comment"));

    // Should preserve leading whitespace when stripping
    assert_eq!(strip_inline_comment("  # comment"), "  ");
    assert_eq!(strip_inline_comment("    # indented comment"), "    ");
    assert_eq!(strip_inline_comment("\t# tab comment"), "\t");
}

// Comments at End of Line Tests

#[test]
fn test_comment_at_end_of_line_basic() {
    // Comment at end of line with content before it
    assert_eq!(strip_inline_comment("key: value # comment"), "key: value ");
    assert_eq!(strip_inline_comment("name: John # inline comment"), "name: John ");
    assert_eq!(strip_inline_comment("- item # end comment"), "- item ");

    // These should NOT be full comment lines
    assert!(!is_comment_line("key: value # comment"));
    assert!(!is_comment_line("name: John # inline comment"));
    assert!(!is_comment_line("- item # end comment"));
}

#[test]
fn test_comment_at_end_of_line_with_spacing() {
    // Various spacing patterns before comment
    assert_eq!(strip_inline_comment("key: value#comment"), "key: value#comment");
    assert_eq!(strip_inline_comment("key: value #comment"), "key: value ");
    assert_eq!(strip_inline_comment("key: value  #comment"), "key: value  ");
    assert_eq!(strip_inline_comment("key: value   #comment"), "key: value   ");
    assert_eq!(strip_inline_comment("key: value\t#comment"), "key: value\t");

    // Only whitespace-preceded # starts comment
    assert!(!is_comment_line("key: value#comment"));
    assert!(!is_comment_line("key: value #comment"));
}

#[test]
fn test_comment_at_end_of_line_complex_values() {
    // Comments at end of complex values
    assert_eq!(strip_inline_comment("url: http://example.com # anchor"), "url: http://example.com ");
    assert_eq!(strip_inline_comment("path: /some/path # directory"), "path: /some/path ");
    assert_eq!(strip_inline_comment("count: 42 # number"), "count: 42 ");
    assert_eq!(strip_inline_comment("flag: true # boolean"), "flag: true ");

    // URLs with hashes preserved
    assert_eq!(strip_inline_comment("url: http://example.com#section # comment"), "url: http://example.com#section ");
    assert_eq!(strip_inline_comment("link: https://api.example.com/v1#endpoint # API"), "link: https://api.example.com/v1#endpoint ");
}

// Comments in Middle of Line Tests

#[test]
fn test_comment_in_middle_of_line_with_trailing_content() {
    // Comments that appear mid-line with content both before and after
    // Note: In YAML, once a # starts a comment, everything after is ignored
    // So "middle" comments effectively become "end" comments in the result

    // These demonstrate that once a comment starts, everything after is removed
    assert_eq!(strip_inline_comment("key: value # comment with trailing text"), "key: value ");
    assert_eq!(strip_inline_comment("item: one # comment more stuff here"), "item: one ");
    assert_eq!(strip_inline_comment("field: value # note: important info"), "field: value ");

    // The trailing content after # is always removed
    assert_eq!(strip_inline_comment("key: value # TODO: fix this later"), "key: value ");
    assert_eq!(strip_inline_comment("key: value # FIXME: broken"), "key: value ");
}

#[test]
fn test_comment_in_middle_separated_by_whitespace() {
    // Verify that only the first whitespace-preceded # starts the comment
    assert_eq!(strip_inline_comment("key: value # comment # more comment"), "key: value ");
    assert_eq!(strip_inline_comment("text: hello # world # foo # bar"), "text: hello ");
    assert_eq!(strip_inline_comment("data: value # first # second # third"), "data: value ");

    // Everything from first # to end is removed
    assert_eq!(strip_inline_comment("key: value # comment with:colons # and:more"), "key: value ");
}

// Multiple # Symbol Tests

#[test]
fn test_multiple_hash_symbols_at_different_positions() {
    // Multiple # symbols - only first whitespace-preceded one starts comment

    // Hash in value (no whitespace before) - part of value
    assert_eq!(strip_inline_comment("key: value#hash#in#value"), "key: value#hash#in#value");
    assert_eq!(strip_inline_comment("text: hello#world#foo#bar"), "text: hello#world#foo#bar");

    // First whitespace-preceded # starts comment
    assert_eq!(strip_inline_comment("key: value # comment # with # multiple"), "key: value ");
    assert_eq!(strip_inline_comment("text: hello#world # comment # more"), "text: hello#world ");
}

#[test]
fn test_multiple_hash_symbols_mixed_positions() {
    // Complex scenarios with # at various positions
    assert_eq!(strip_inline_comment("key: value#not#comment # is#comment"), "key: value#not#comment ");
    assert_eq!(strip_inline_comment("url: http://example.com#anchor # has # hash"), "url: http://example.com#anchor ");
    assert_eq!(strip_inline_comment("quoted: \"value#hash\" # comment # second"), "quoted: \"value#hash\" ");

    // Multiple consecutive # symbols
    assert_eq!(strip_inline_comment("key: value ## double hash comment"), "key: value ");
    assert_eq!(strip_inline_comment("key: value ### triple hash comment"), "key: value ");
    assert_eq!(strip_inline_comment("key: value #### quad hash comment"), "key: value ");
}

#[test]
fn test_hash_without_preceding_whitespace() {
    // Hash without whitespace before is part of value
    assert_eq!(strip_inline_comment("key: value#comment"), "key: value#comment");
    assert_eq!(strip_inline_comment("key: value#not#a#comment"), "key: value#not#a#comment");
    assert_eq!(strip_inline_comment("key: value###"), "key: value###");
    assert_eq!(strip_inline_comment("key: value#"), "key: value#");

    // Even at "end" of line, without whitespace it's part of value
    assert_eq!(strip_inline_comment("key: value#"), "key: value#");
    assert_eq!(strip_inline_comment("key: value#end#of#line"), "key: value#end#of#line");
}

#[test]
fn test_hash_immediately_after_colon() {
    // Hash immediately after colon (no space) is part of value
    assert_eq!(strip_inline_comment("key:#value"), "key:#value");
    assert_eq!(strip_inline_comment("key:#value#more#hashes"), "key:#value#more#hashes");

    // But with space before #, it's a comment
    assert_eq!(strip_inline_comment("key: #value"), "key: ");
}

// Edge Cases and Special Scenarios

#[test]
fn test_comment_with_special_characters() {
    // Comments containing various special characters
    assert_eq!(strip_inline_comment("key: value # !@#$%^&*()"), "key: value ");
    assert_eq!(strip_inline_comment("key: value # [brackets], {braces}, (parens)"), "key: value ");
    assert_eq!(strip_inline_comment("key: value # <tags>, `backticks`, 'quotes\""), "key: value ");
}

#[test]
fn test_comment_with_urls_in_comment_text() {
    // URLs in the comment portion itself (these are removed)
    assert_eq!(strip_inline_comment("key: value # see http://example.com for details"), "key: value ");
    assert_eq!(strip_inline_comment("key: value # ref https://api.example.com/v1"), "key: value ");

    // But URLs in the value portion are preserved
    assert_eq!(strip_inline_comment("url: http://example.com#anchor # comment with http://other.com"), "url: http://example.com#anchor ");
}

#[test]
fn test_comment_empty_after_hash() {
    // Comment with nothing or just whitespace after #
    // Once a comment starts with #, everything after is removed
    assert_eq!(strip_inline_comment("key: value #"), "key: value ");
    assert_eq!(strip_inline_comment("key: value # "), "key: value ");
    assert_eq!(strip_inline_comment("key: value #  "), "key: value ");
    assert_eq!(strip_inline_comment("key: value #   "), "key: value ");
    assert_eq!(strip_inline_comment("key: value #\t"), "key: value ");
}

#[test]
fn test_comment_at_different_indentations() {
    // Comments at various indentation levels
    assert_eq!(strip_inline_comment("key: value # comment"), "key: value ");
    assert_eq!(strip_inline_comment("  key: value # comment"), "  key: value ");
    assert_eq!(strip_inline_comment("    key: value # comment"), "    key: value ");
    assert_eq!(strip_inline_comment("\tkey: value # comment"), "\tkey: value ");
    assert_eq!(strip_inline_comment("  \t key: value # comment"), "  \t key: value ");
}

#[test]
fn test_full_line_comment_with_multiple_hashes() {
    // Full-line comments with multiple #
    assert_eq!(classify_line_type("## double hash comment"), LineType::Comment);
    assert_eq!(classify_line_type("### triple hash comment"), LineType::Comment);
    assert_eq!(classify_line_type("#### quad hash comment"), LineType::Comment);

    // All should strip completely
    assert_eq!(strip_inline_comment("## double hash comment"), "");
    assert_eq!(strip_inline_comment("### triple hash comment"), "");
    assert_eq!(strip_inline_comment("#### quad hash comment"), "");
}

#[test]
fn test_classification_of_lines_with_hash_at_different_positions() {
    // Classification based on first non-whitespace character

    // Hash at start (after trim) = comment
    assert_eq!(classify_line_type("# comment"), LineType::Comment);
    assert_eq!(classify_line_type("  # comment"), LineType::Comment);
    assert_eq!(classify_line_type("\t# comment"), LineType::Comment);

    // Content before hash = not a comment line
    assert_eq!(classify_line_type("key: value # comment"), LineType::MappingKey);
    assert_eq!(classify_line_type("- item # comment"), LineType::SequenceItem);

    // Hash without preceding whitespace = still not a comment line
    assert_eq!(classify_line_type("key: value#comment"), LineType::MappingKey);
}

// Integration Tests

#[test]
fn test_yaml_comment_positions_complete_document() {
    // Test various comment positions in a complete YAML document
    let lines = vec![
        "# Start comment",                              // line 1: start comment
        "database: postgres",                           // line 2: content
        "  # Nested comment",                          // line 3: start comment (indented)
        "  host: localhost",                           // line 4: content
        "  port: 5432 # default port",                 // line 5: end comment
        "  ssl: false # disabled for now",            // line 6: end comment
        "  # TODO: add SSL settings",                 // line 7: start comment (indented)
        "api: http://api.example.com/v1#endpoint # API URL", // line 8: hash in value + end comment
        "# End comment",                               // line 9: start comment
    ];

    // Test classifications
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert_eq!(classify_line_type(lines[1]), LineType::MappingKey);
    assert_eq!(classify_line_type(lines[2]), LineType::Comment);
    assert_eq!(classify_line_type(lines[3]), LineType::MappingKey);
    assert_eq!(classify_line_type(lines[4]), LineType::MappingKey);
    assert_eq!(classify_line_type(lines[5]), LineType::MappingKey);
    assert_eq!(classify_line_type(lines[6]), LineType::Comment);
    assert_eq!(classify_line_type(lines[7]), LineType::MappingKey);
    assert_eq!(classify_line_type(lines[8]), LineType::Comment);

    // Test comment detection helper
    assert!(is_comment_line(lines[0]));
    assert!(!is_comment_line(lines[1]));
    assert!(is_comment_line(lines[2]));
    assert!(!is_comment_line(lines[3]));
    assert!(!is_comment_line(lines[4]));
    assert!(!is_comment_line(lines[5]));
    assert!(is_comment_line(lines[6]));
    assert!(!is_comment_line(lines[7]));
    assert!(is_comment_line(lines[8]));

    // Test inline comment stripping
    assert_eq!(strip_inline_comment(lines[0]), "");
    assert_eq!(strip_inline_comment(lines[1]), "database: postgres");
    assert_eq!(strip_inline_comment(lines[2]), "  ");
    assert_eq!(strip_inline_comment(lines[3]), "  host: localhost");
    assert_eq!(strip_inline_comment(lines[4]), "  port: 5432 ");
    assert_eq!(strip_inline_comment(lines[5]), "  ssl: false ");
    assert_eq!(strip_inline_comment(lines[6]), "  ");
    assert_eq!(strip_inline_comment(lines[7]), "api: http://api.example.com/v1#endpoint ");
    assert_eq!(strip_inline_comment(lines[8]), "");
}

#[test]
fn test_multiple_hashes_complex_scenarios() {
    // Test complex scenarios with multiple # symbols

    // Hash in URL followed by comment
    let line1 = "url: http://example.com#anchor # this is a comment";
    assert_eq!(strip_inline_comment(line1), "url: http://example.com#anchor ");

    // Multiple hashes in value, then comment
    let line2 = "key: value#with#multiple#hashes # comment starts here";
    assert_eq!(strip_inline_comment(line2), "key: value#with#multiple#hashes ");

    // Hashes in quoted strings preserved
    let line3 = "key: \"value#with#hashes\" # comment # more # comments";
    assert_eq!(strip_inline_comment(line3), "key: \"value#with#hashes\" ");

    // Mixed: quoted hash, value hash, then comment
    let line4 = "key: \"quoted#hash\" and value#hash # final comment";
    assert_eq!(strip_inline_comment(line4), "key: \"quoted#hash\" and value#hash ");
}

#[test]
fn test_comment_positions_preserve_structure() {
    // Verify that comment filtering preserves YAML structure

    // Mapping with comments at various positions
    let yaml = vec![
        "# Document header",                            // start comment
        "parent:",                                      // parent key
        "  # Section comment",                         // indented start comment
        "  child1: value1",                            // nested key
        "  child2: value2 # inline comment",          // nested key with end comment
        "  child3: value3#hash",                       // nested key with hash in value
        "  child4: \"value#with#hashes\"",             // nested key with quoted hashes
        "  # TODO: add more children",                // indented start comment
        "  # FIXME: child5 is broken",                 // indented start comment with colon
    ];

    // Verify all classifications are correct
    assert_eq!(classify_line_type(yaml[0]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[1]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[2]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[3]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[4]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[5]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[6]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[7]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[8]), LineType::Comment);

    // Verify inline comment stripping preserves structure
    assert_eq!(strip_inline_comment(yaml[0]), "");
    assert_eq!(strip_inline_comment(yaml[1]), "parent:");
    assert_eq!(strip_inline_comment(yaml[2]), "  ");
    assert_eq!(strip_inline_comment(yaml[3]), "  child1: value1");
    assert_eq!(strip_inline_comment(yaml[4]), "  child2: value2 ");
    assert_eq!(strip_inline_comment(yaml[5]), "  child3: value3#hash");
    assert_eq!(strip_inline_comment(yaml[6]), "  child4: \"value#with#hashes\"");
    assert_eq!(strip_inline_comment(yaml[7]), "  ");
    assert_eq!(strip_inline_comment(yaml[8]), "  ");
}
