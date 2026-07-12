//! YAML Comment Detection in Plain Multi-Line Scalars Tests
//!
//! These tests verify YAML comment detection behavior within plain multi-line
//! scalars (values without explicit style indicators like `|` or `>`).
//!
//! ## YAML Specification for Comments in Plain Scalars
//!
//! In plain scalars, the `#` character has context-dependent behavior:
//! - When preceded by whitespace (or at line start): `#` starts a comment
//! - When NOT preceded by whitespace: `#` is preserved as content (e.g., URLs, anchors)
//!
//! This differs from literal/ folded block scalars where `#` is ALWAYS preserved
//! as content and never starts a comment.
//!
//! Bead: bf-4ukhl
//! Acceptance Criteria:
//! - Test passes for comments in plain multi-line scalars
//! - Tests reflect actual parser behavior for plain scalars
//! - Verify `#` in plain scalars starts comments (unlike block scalars)
//! - Verify plain scalar content is correctly parsed

use armor::parsers::yaml::{
    classify_line_type, is_comment_line, strip_inline_comment, LineType
};

// ============================================================================
// Plain Multi-Line Scalar Basics
// ============================================================================

#[test]
fn test_plain_scalar_single_line_classification() {
    // Single-line plain scalars should be classified as mapping keys
    let test_cases = vec![
        "text: value",
        "description: plain text",
        "content: data",
        "summary: information",
    ];

    for line in test_cases {
        // Should be classified as mapping key, not comment
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));
    }
}

#[test]
fn test_plain_scalar_multiline_continuation() {
    // Plain scalars can span multiple lines through indentation
    // Continuation lines should not be classified as comments
    let test_cases = vec![
        "  continuation line",
        "    more indented continuation",
        "  even more text",
        "    deeply indented line",
    ];

    for line in test_cases {
        // Indented continuation lines are unknown type (not comments)
        // They don't start with # and are not empty
        assert!(!is_comment_line(line));
    }
}

// ============================================================================
// Hash Characters in Plain Scalars - Unlike Block Scalars
// ============================================================================

#[test]
fn test_hash_in_plain_scalar_starts_comment() {
    // Unlike literal/folded blocks, plain scalars treat # as comment start
    let test_cases = vec![
        "# This IS a comment in plain scalar context",
        "  # Indented comment line",
        "    # Also a comment",
        "# Another full-line comment",
    ];

    for line in test_cases {
        // Lines starting with # ARE comments in plain scalar context
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert!(is_comment_line(line));
    }
}

#[test]
fn test_hash_symbol_in_plain_scalar_value() {
    // Hash symbols in plain scalar values preceded by space start comments
    let test_cases = vec![
        // Hash NOT preceded by space - preserved
        ("url:http://example.com#anchor", "url:http://example.com#anchor"),
        ("value#hash", "value#hash"),
        ("text#more#here", "text#more#here"),
        // Hash preceded by whitespace - treated as comment start
        ("text with # hash", "text with "),
        ("code is #FFFFFF", "code is "),
        ("value # then comment", "value "),
    ];

    for (line, expected) in test_cases {
        // These lines don't start with #, so they're not comment lines
        assert!(!is_comment_line(line));

        // The hash behavior depends on preceding character
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected,
                   "Hash handling should match YAML spec: line={}, got={}", line, stripped);
    }
}

#[test]
fn test_multiple_hashes_in_plain_scalar() {
    // Multiple hash symbols in a plain scalar line
    // The behavior depends on whether each hash is preceded by whitespace
    let test_cases = vec![
        // First hash preceded by space → comment starts there
        ("Line with # multiple # hash # symbols", "Line with "),
        // Hashes not preceded by space → preserved
        ("Config:key=value#hash#more#here", "Config:key=value#hash#more#here"),
        // Mixed: some preceded by space, some not
        ("value#part # comment", "value#part "),
        ("url#anchor1 # comment # more", "url#anchor1 "),
    ];

    for (line, expected) in test_cases {
        // Lines don't start with #, so they're not comment lines
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected,
                   "Hash handling should match YAML spec: line={}, got={}", line, stripped);
    }
}

// ============================================================================
// Plain Scalar with Real Comments
// ============================================================================

#[test]
fn test_plain_scalar_followed_by_real_comment() {
    // A plain scalar value followed by an actual comment
    let yaml_lines = vec![
        "text: value",                // Plain scalar
        "# This is a real comment",   // Actual comment
        "more: data",                 // Another plain scalar
        "  # Indented comment",       // Indented comment
    ];

    // The plain scalar lines
    assert_eq!(classify_line_type(yaml_lines[0]), LineType::MappingKey);
    assert!(!is_comment_line(yaml_lines[0]));

    // The actual comment lines
    assert_eq!(classify_line_type(yaml_lines[1]), LineType::Comment);
    assert!(is_comment_line(yaml_lines[1]));
    assert_eq!(classify_line_type(yaml_lines[3]), LineType::Comment);
    assert!(is_comment_line(yaml_lines[3]));

    // Another plain scalar
    assert_eq!(classify_line_type(yaml_lines[2]), LineType::MappingKey);
    assert!(!is_comment_line(yaml_lines[2]));
}

#[test]
fn test_plain_scalar_with_inline_comment() {
    // Plain scalar with inline comment on same line
    let test_cases = vec![
        ("text: value # This is a comment", "text: value "),
        ("description: plain # inline comment here", "description: plain "),
        ("content: data # end-of-line comment", "content: data "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);
    }
}

// ============================================================================
// Multi-Line Plain Scalar Scenarios
// ============================================================================

#[test]
fn test_multiline_plain_scalar_with_comment_lines() {
    // Multi-line plain scalar with comment lines interspersed
    let yaml_lines = vec![
        "description: This is a plain scalar",
        "  that spans multiple lines",
        "# This is a comment, NOT part of scalar",
        "  and continues here",
        "  # Another comment line",
        "  final line of scalar",
    ];

    // First line - mapping key with start of plain scalar
    assert_eq!(classify_line_type(yaml_lines[0]), LineType::MappingKey);
    assert!(!is_comment_line(yaml_lines[0]));

    // Continuation lines - not comments
    assert!(!is_comment_line(yaml_lines[1]));
    assert!(!is_comment_line(yaml_lines[3]));
    assert!(!is_comment_line(yaml_lines[5]));

    // Comment lines - ARE comments in plain scalar context
    assert!(is_comment_line(yaml_lines[2]));
    assert!(is_comment_line(yaml_lines[4]));
}

#[test]
fn test_multiline_plain_scalar_with_hash_in_content() {
    // Multi-line plain scalar with hash symbols in content
    let yaml_lines = vec![
        "config: value with#hash",  // Hash not preceded by space
        "  url:http://example.com#anchor",  // URL with anchor
        "  code is # comment here", // Hash preceded by space - comment start
        "  more data",
    ];

    // All lines are not comment lines (none start with #)
    for line in &yaml_lines {
        assert!(!is_comment_line(line));
    }

    // Verify inline comment stripping
    assert_eq!(strip_inline_comment(yaml_lines[0]), "config: value with#hash");
    assert_eq!(strip_inline_comment(yaml_lines[1]), "  url:http://example.com#anchor");
    // The `# comment` part gets stripped
    let line2_stripped = strip_inline_comment(yaml_lines[2]);
    assert!(line2_stripped.contains("code is"));
    assert!(!line2_stripped.contains("comment"));
}

// ============================================================================
// Complex Plain Scalar Scenarios
// ============================================================================

#[test]
fn test_plain_scalar_preserves_some_content() {
    // Plain scalar preserves content up to first ` #` pattern
    let content_lines = vec![
        "Regular text without hash",
        "Text with #hash in middle",  // Preserved - no space before #
        "URL: http://example.com#anchor",  // Preserved
        "Value # with comment",  // Comment part removed
    ];

    for line in content_lines {
        // None of these start with #, so they're not comment lines
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        let has_space_hash = line.contains(" #");

        if has_space_hash {
            // Content before ` #` preserved, after removed
            assert!(!stripped.contains("#") ||
                   stripped.split(" #").next().unwrap_or("").contains("#"),
                   "Only hashes before ` #` should be preserved");
        }
    }
}

#[test]
fn test_plain_scalar_with_mixed_content() {
    // Plain scalar containing various types of content with hash symbols
    let test_cases = vec![
        "Regular text without hash",
        "value#hash",  // Preserved - no space before #
        "Text with # hash in middle",  // Comment starts at ` #`
        "https://url.com#anchor with hash",  // Preserved - no space before #
        "End line with value#hash",  // Preserved
        "Last # comment here",  // Comment starts at ` #`
    ];

    for line in test_cases {
        let stripped = strip_inline_comment(line);
        let hash_count = line.matches('#').count();
        let stripped_hash_count = stripped.matches('#').count();

        // Count preserved vs removed hashes
        let has_space_hash = line.contains(" #");

        if has_space_hash {
            // All hashes from the first ` #` onwards are stripped
            let before_space_hash = line.split(" #").next().unwrap_or(line);
            let expected_hash_count = before_space_hash.matches('#').count();
            assert_eq!(stripped_hash_count, expected_hash_count,
                       "Hash count mismatch for line: {:?}, stripped: {:?}", line, stripped);
        } else {
            // No ` #` pattern, so all hashes should be preserved
            assert_eq!(stripped_hash_count, hash_count,
                       "All hashes should be preserved for line: {:?}", line);
        }
    }
}

// ============================================================================
// Plain vs Block Scalar Comparison
// ============================================================================

#[test]
fn test_plain_vs_literal_block_classification() {
    // Plain scalars and literal blocks classify differently
    let test_cases = vec![
        ("text: value", "text: |"),           // Plain vs literal marker
        ("more: data", "more: |-"),           // Plain vs literal with modifier
    ];

    for (plain, literal) in test_cases {
        // Both should be classified as mapping keys
        assert_eq!(classify_line_type(plain), LineType::MappingKey);
        assert_eq!(classify_line_type(literal), LineType::MappingKey);
        assert_eq!(is_comment_line(plain), is_comment_line(literal));
    }

    // But content lines are classified differently
    let plain_content = "  # This is a comment in plain scalar context";
    let literal_content = "  # This is content in literal block";

    // Plain scalar: lines starting with # ARE comments
    assert!(is_comment_line(plain_content));

    // Literal block: lines starting with # are also classified as comments
    // by the line parser (since it doesn't track block context)
    // This is expected - line-level analysis doesn't maintain block state
    assert!(is_comment_line(literal_content));
}

#[test]
fn test_plain_vs_folded_block_classification() {
    // Plain scalars and folded blocks classify similarly for mapping keys
    let test_cases = vec![
        ("text: value", "text: >"),           // Plain vs folded marker
        ("more: data", "more: >+"),           // Plain vs folded with modifier
    ];

    for (plain, folded) in test_cases {
        // Both should be classified as mapping keys
        assert_eq!(classify_line_type(plain), LineType::MappingKey);
        assert_eq!(classify_line_type(folded), LineType::MappingKey);
        assert_eq!(is_comment_line(plain), is_comment_line(folded));
    }

    // Content lines behave the same at line level
    let plain_content = "  # This is a comment";
    let folded_content = "  # This is also a comment at line level";

    // Both classified as comments by line parser
    assert!(is_comment_line(plain_content));
    assert!(is_comment_line(folded_content));
}

// ============================================================================
// Integration Tests - Complete YAML Documents
// ============================================================================

#[test]
fn test_complete_yaml_with_plain_scalar_and_comments() {
    // Complete YAML document showing plain scalars with various content
    let yaml = r#"#!/usr/bin/env script
# This is a real comment at document level

summary: This is a plain scalar
  that spans multiple lines
  # This is a comment, not content
  And continues after comment

description: plain text here # Inline comment
  More continuation text
  # Another comment line
  Final continuation

# Real comment after plain scalar
config: value
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify the key lines are classified correctly
    // Line 0: Shebang-like
    assert!(is_comment_line(lines[0]));

    // Line 1: Real comment
    assert!(is_comment_line(lines[1]));

    // Line 3: Plain scalar mapping key
    assert_eq!(classify_line_type(lines[3]), LineType::MappingKey);
    assert!(!is_comment_line(lines[3]));

    // Line 4: Continuation - not a comment
    assert!(!is_comment_line(lines[4]));

    // Line 5: Comment line - IS a comment in plain scalar context
    assert!(is_comment_line(lines[5]));

    // Line 6: Continuation after comment - not a comment
    assert!(!is_comment_line(lines[6]));

    // Line 8: Description with inline comment
    assert!(!is_comment_line(lines[8]));

    // Line 13: Real comment after plain scalar
    assert!(is_comment_line(lines[13]));

    // Line 14: Normal mapping
    assert!(!is_comment_line(lines[14]));
}

#[test]
fn test_plain_scalar_documentation_example() {
    // Realistic example: configuration with plain scalars
    let yaml = r#"api:
  endpoint: /api/v1
  # Configuration comment
  description: This is a plain scalar description
    that spans multiple lines
    # Comment within plain scalar
    and continues here
  summary: Plain scalar summary
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify structure
    assert!(!is_comment_line(lines[1]));     // endpoint: /api/v1
    assert!(is_comment_line(lines[2]));      // # Configuration comment
    assert!(!is_comment_line(lines[3]));     // description: This is...

    // Lines inside plain scalar
    assert!(!is_comment_line(lines[4]));     // Continuation
    assert!(is_comment_line(lines[5]));       // Comment line
    assert!(!is_comment_line(lines[6]));     // Continuation

    // After plain scalar
    assert!(!is_comment_line(lines[7]));     // summary: line
}

#[test]
fn test_plain_scalar_with_configuration_examples() {
    // Configuration file with plain scalars containing various data
    // Note: This test demonstrates that lines starting with # are comments
    // even when they appear to be continuation lines
    let yaml = r#"database:
  host: localhost
  # Connection settings
  url: postgresql://user:pass@host:port/database#schema
  # This comment is not part of URL
  ssl: true
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify key lines
    assert!(!is_comment_line(lines[1]));     // host: localhost
    assert!(is_comment_line(lines[2]));      // # Connection settings
    assert!(!is_comment_line(lines[3]));     // url: postgresql://...

    // Line 4 starts with # after indentation - it's a comment
    assert!(is_comment_line(lines[4]));       // Comment line
    // Even though it appears after the url line, it's NOT a continuation

    // After the comment, back to mapping keys
    assert!(!is_comment_line(lines[5]));     // ssl: true
}

// ============================================================================
// Edge Cases
// ============================================================================

#[test]
fn test_plain_scalar_empty_continuation() {
    // Plain scalar with empty continuation lines
    let yaml_lines = vec![
        "text: value",
        "",      // Empty line
        "  ",    // Whitespace line
        "more: data",
    ];

    assert!(!is_comment_line(yaml_lines[0]));
    assert!(!is_comment_line(yaml_lines[1])); // Empty line not a comment
    assert!(!is_comment_line(yaml_lines[2])); // Whitespace not a comment
    assert!(!is_comment_line(yaml_lines[3]));
}

#[test]
fn test_plain_scalar_with_nested_indentation() {
    // Plain scalar can have content with varying indentation
    let content_lines = vec![
        "  function() {",
        "    if (condition) {",
        "      # This is a comment at higher indent",
        "      value = 'test'",
        "    }",
        "  }",
    ];

    for line in content_lines {
        let trimmed = line.trim();
        if trimmed.starts_with('#') {
            // Lines starting with # are comments
            assert!(is_comment_line(line));
        } else {
            // Other lines are not
            assert!(!is_comment_line(line));
        }
    }
}

#[test]
fn test_plain_scalar_with_special_characters() {
    // Plain scalar with various special characters
    let test_cases = vec![
        "value: key=value#hash",
        "config: url#anchor",
        "text: data@server#tag",
        "mixed: value # comment",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        if line.contains(" #") {
            // Comment portion should be stripped
            assert!(!stripped.contains("comment"));
        } else {
            // All hashes preserved
            assert_eq!(stripped.matches('#').count(), line.matches('#').count());
        }
    }
}

// ============================================================================
// Behavior Documentation Tests
// ============================================================================

#[test]
fn test_document_plain_scalar_comment_behavior() {
    // This test documents the current behavior of comment detection
    // in plain multi-line scalars and serves as executable documentation

    // Key behavior 1: Plain scalar mapping key line
    let mapping = "content: value";
    assert_eq!(classify_line_type(mapping), LineType::MappingKey);
    assert!(!is_comment_line(mapping));

    // Key behavior 2: Lines starting with # ARE comments in plain scalar context
    let comment_line = "  # This is a comment";
    assert_eq!(classify_line_type(comment_line), LineType::Comment);
    assert!(is_comment_line(comment_line));

    // Key behavior 3: Inline comments work correctly
    let mapping_with_comment = "content: value # This is a real comment";
    assert!(!is_comment_line(mapping_with_comment));
    let stripped = strip_inline_comment(mapping_with_comment);
    assert_eq!(stripped, "content: value ");

    // Key behavior 4: Hash symbols preceded by whitespace trigger comment stripping
    let content_with_hash = "Text with # hash symbol";
    assert!(!is_comment_line(content_with_hash));
    let stripped = strip_inline_comment(content_with_hash);
    assert_eq!(stripped, "Text with ");
    assert!(!stripped.contains('#'));

    // Key behavior 5: Hash symbols NOT preceded by space are preserved
    let content_with_url = "url: http://example.com#anchor";
    let stripped = strip_inline_comment(content_with_url);
    assert!(stripped.contains("#anchor"));
}

#[test]
fn test_plain_scalar_vs_block_scalars_comment_handling() {
    // Document that plain scalars differ from block scalars in comment handling

    // Plain scalar: # starts a comment
    let plain = "text: value # comment";
    let plain_stripped = strip_inline_comment(plain);
    assert_eq!(plain_stripped, "text: value ");

    // Literal block: # is preserved as content (at line level, looks like comment)
    let literal_content = "  # This looks like comment but is content in literal block";
    // At line level, this IS classified as a comment
    assert!(is_comment_line(literal_content));
    // But in a full parser tracking literal block state, this would be content

    // Folded block: # is preserved as content (at line level, looks like comment)
    let folded_content = "  # This looks like comment but is content in folded block";
    // At line level, this IS classified as a comment
    assert!(is_comment_line(folded_content));
    // But in a full parser tracking folded block state, this would be content

    // The key difference: plain scalars treat # as comment only when preceded by whitespace
    // Block scalars preserve # as content regardless of context
}
