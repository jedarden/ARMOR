//! YAML Comment Detection in Multi-Line Quoted Scalars Tests
//!
//! These tests verify YAML comment detection behavior within multi-line
//! quoted scalars (both single-quoted '...' and double-quoted "...").
//! In quoted scalars, ALL content including `#` characters is preserved
//! as literal text and should NOT be treated as comments.
//!
//! Bead: bf-2tqyk
//! Acceptance Criteria:
//! - Test passes for comments in multi-line quoted scalars
//! - Test reflects actual parser behavior for quoted scalars
//! - Verify `#` in quoted strings is content, not comment
//! - Verify quoted scalar content is preserved correctly

use armor::parsers::yaml::{
    classify_line_type, is_comment_line, strip_inline_comment, LineType
};

// ============================================================================
// Double-Quoted Multi-Line Scalar Basics
// ============================================================================

#[test]
fn test_double_quoted_scalar_marker_classification() {
    // Double-quoted scalars can span multiple lines by including newlines
    let test_cases = vec![
        "text: \"line1\"",                // Single line
        "description: \"line1",
        "content: \"multi",
        "summary: \"line1",
    ];

    for line in test_cases {
        // Should be classified as mapping key, not comment
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));
    }
}

#[test]
fn test_double_quoted_scalar_with_hash_inside() {
    // Hash symbols inside double quotes are ALWAYS preserved as content
    let test_cases = vec![
        "color: \"#FFFFFF\"",
        "url: \"http://example.com#anchor\"",
        "text: \"value # with # hash\"",
        "config: \"#this # is # content\"",
    ];

    for line in test_cases {
        // Should NOT be a comment line
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        // Hash should be preserved when inside quotes
        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "Hash should be preserved in: {}", line);
    }
}

#[test]
fn test_double_quoted_scalar_with_inline_comment() {
    // Real inline comment after the quoted scalar
    let test_cases = vec![
        ("color: \"#FFFFFF\" # this is a comment", "color: \"#FFFFFF\" "),
        ("url: \"http://example.com#anchor\" # API URL", "url: \"http://example.com#anchor\" "),
        ("text: \"value # hash\" # documentation", "text: \"value # hash\" "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);
    }
}

#[test]
fn test_double_quoted_multiline_with_newlines() {
    // Double-quoted scalars with actual newlines inside
    let test_cases = vec![
        "text: \"line1",
        "description: \"First paragraph",
        "content: \"Line with # hash",
    ];

    for line in test_cases {
        // Should be classified as mapping key
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));

        // Hash symbols inside quotes should be preserved
        let stripped = strip_inline_comment(line);
        if line.contains('#') {
            assert!(stripped.contains('#'), "Hash in quoted multiline should be preserved in: {}", line);
        }
    }
}

// ============================================================================
// Single-Quoted Multi-Line Scalar Basics
// ============================================================================

#[test]
fn test_single_quoted_scalar_marker_classification() {
    // Single-quoted scalars can also span multiple lines
    let test_cases = vec![
        "text: 'line1'",                // Single line
        "description: 'line1",
        "content: 'multi",
        "summary: 'line1",
    ];

    for line in test_cases {
        // Should be classified as mapping key, not comment
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));
    }
}

#[test]
fn test_single_quoted_scalar_with_hash_inside() {
    // Hash symbols inside single quotes are ALWAYS preserved as content
    let test_cases = vec![
        "color: '#FFFFFF'",
        "url: 'http://example.com#anchor'",
        "text: 'value # with # hash'",
        "config: '#this # is # content'",
    ];

    for line in test_cases {
        // Should NOT be a comment line
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        // Hash should be preserved when inside quotes
        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "Hash should be preserved in: {}", line);
    }
}

#[test]
fn test_single_quoted_scalar_with_inline_comment() {
    // Real inline comment after the quoted scalar
    let test_cases = vec![
        ("color: '#FFFFFF' # this is a comment", "color: '#FFFFFF' "),
        ("url: 'http://example.com#anchor' # API URL", "url: 'http://example.com#anchor' "),
        ("text: 'value # hash' # documentation", "text: 'value # hash' "),
    ];

    for (line, expected_stripped) in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);
    }
}

#[test]
fn test_single_quoted_multiline_with_newlines() {
    // Single-quoted scalars with actual newlines inside
    let test_cases = vec![
        "text: 'line1",
        "description: 'First paragraph",
        "content: 'Line with # hash",
    ];

    for line in test_cases {
        // Should be classified as mapping key
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));

        // Hash symbols inside quotes should be preserved
        let stripped = strip_inline_comment(line);
        if line.contains('#') {
            assert!(stripped.contains('#'), "Hash in quoted multiline should be preserved in: {}", line);
        }
    }
}

// ============================================================================
// Complex Multi-Line Quoted Scalar Scenarios
// ============================================================================

#[test]
fn test_quoted_scalar_preserves_all_content() {
    // Quoted scalars should preserve all formatting and special characters
    let content_lines = vec![
        "text: \"#!/bin/bash\"",                     // Shell script with shebang
        "code: \"echo 'value#test'\"",               // Shell echo with hash
        "url: \"https://example.com#anchor\"",      // URL with anchor
        "pattern: \"regex#pattern#here\"",           // Pattern with hashes
    ];

    for line in content_lines {
        // In quoted scalars, all content is preserved
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "Hash should be preserved in: {}", line);
    }
}

#[test]
fn test_quoted_scalar_with_escaped_quotes() {
    // Quoted scalars with escaped quotes inside (double quotes only)
    let test_cases = vec![
        "text: \"Quote with \\\" inside\"",
        "json: \"{\\\"key\\\": \\\"value#hash\\\"}\"",
        "path: \"C:\\\\path#to\\\\file\"",
    ];

    for line in test_cases {
        // Should be mapping keys
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        // Hash should be preserved
        if line.contains('#') {
            assert!(stripped.contains('#'), "Hash should be preserved in: {}", line);
        }
    }
}

#[test]
fn test_quoted_scalar_with_mixed_quotes() {
    // Single quotes inside double quotes, and vice versa
    let test_cases = vec![
        "text: \"Single 'quotes' inside double\"",
        "text: 'Double \"quotes\" inside single'",
        "mixed1: \"Value with 'single' # hash\"",
        "mixed2: 'Value with \"double\" # hash'",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        if line.contains('#') {
            assert!(stripped.contains('#'), "Hash should be preserved in: {}", line);
        }
    }
}

#[test]
fn test_double_vs_single_quoted_identical_handling() {
    // Both double and single quotes handle comments identically
    let pairs = vec![
        ("text: \"#hash\"", "text: '#hash'"),
        ("value: \"multi", "value: 'multi"),
    ];

    for (double_quoted, single_quoted) in pairs {
        // Both should be classified the same way
        assert_eq!(classify_line_type(double_quoted), classify_line_type(single_quoted));
        assert_eq!(is_comment_line(double_quoted), is_comment_line(single_quoted));

        // Both should preserve hash
        let stripped_double = strip_inline_comment(double_quoted);
        let stripped_single = strip_inline_comment(single_quoted);
        assert_eq!(stripped_double.contains('#'), stripped_single.contains('#'));
    }
}

// ============================================================================
// Edge Cases
// ============================================================================

#[test]
fn test_quoted_scalar_empty() {
    // Empty quoted scalars
    let test_cases = vec![
        "empty: \"\"",
        "empty: ''",
    ];

    for line in test_cases {
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));
    }
}

#[test]
fn test_quoted_scalar_only_whitespace() {
    // Quoted scalars containing only whitespace
    let test_cases = vec![
        "whitespace: \"   \"",
        "whitespace: '   '",
        "newlines: \"",
        "newlines: '",
    ];

    for line in test_cases {
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));
    }
}

#[test]
fn test_quoted_scalar_with_comment_like_content() {
    // Content that looks like comments but is inside quotes
    let test_cases = vec![
        "comment: \"# This is not a comment\"",
        "todo: '# TODO: fix this'",
        "note: '# FIXME: broken code'",
        "warning: '# WARNING: dangerous'",
    ];

    for line in test_cases {
        // All are quoted scalars, NOT comments
        assert!(!is_comment_line(line), "Should not be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        // The hash should be preserved
        assert!(stripped.contains('#'), "Hash should be preserved in: {}", line);
    }
}

#[test]
fn test_quoted_scalar_url_anchors() {
    // URLs with anchors inside quoted scalars
    let test_cases = vec![
        "api: \"https://api.example.com/v1#endpoint\"",
        "docs: \"http://localhost:8080/docs#readme\"",
        "guide: \"https://docs.example.com/guide#getting-started\"",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "URL anchor should be preserved in: {}", line);
    }
}

#[test]
fn test_quoted_scalar_color_codes() {
    // Hex color codes inside quoted scalars
    let test_cases = vec![
        "background: \"#1a2b3c\"",
        "foreground: \"#000000\"",
        "accent: \"#ff6600\"",
        "transparent: \"#00000000\"",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('#'), "Color code hash should be preserved in: {}", line);
    }
}

// ============================================================================
// Integration Tests - Complete YAML Documents
// ============================================================================

#[test]
fn test_complete_yaml_with_multiline_quoted_scalars() {
    // Complete YAML document with multi-line quoted scalars
    let yaml = r##"# Configuration file
app:
  name: "My Application"
  description: "A multi-line
  description that spans
  multiple lines"
  theme: "#1a2b3c"  # Dark theme color
  url: "https://example.com#anchor"

  single_quoted: 'Another multi-line
  scalar with single quotes'

  inline_comment: "Text with # hash" # This is a real comment

  config:
    enabled: true
    debug: "false"
"##;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify the key lines are classified correctly
    assert!(is_comment_line(lines[0])); // # Configuration file

    assert!(!is_comment_line(lines[2])); // name: "My Application"
    assert_eq!(classify_line_type(lines[2]), LineType::MappingKey);

    assert!(!is_comment_line(lines[3])); // description: "A multi-line...
    assert_eq!(classify_line_type(lines[3]), LineType::MappingKey);

    assert!(!is_comment_line(lines[6])); // theme: "#1a2b3c"  # Dark theme color
    // Hash in quotes should be preserved
    let stripped = strip_inline_comment(lines[6]);
    assert!(stripped.contains('#'));

    assert!(!is_comment_line(lines[7])); // url: "https://example.com#anchor"
    // Hash in quotes should be preserved
    let stripped = strip_inline_comment(lines[7]);
    assert!(stripped.contains('#'));

    assert!(!is_comment_line(lines[12])); // inline_comment: "Text with # hash" # This is a real comment
    // Hash in quotes should be preserved (but not the trailing comment)
    let stripped = strip_inline_comment(lines[12]);
    assert!(stripped.contains('#'));

    assert!(!is_comment_line(lines[15])); // enabled: true
}

#[test]
fn test_realistic_config_with_quoted_multiline() {
    // Realistic configuration file with multi-line quoted scalars
    let yaml = r##"api:
  endpoint: "https://api.example.com/v1"
  documentation: |
    This API provides access to user data.
    # Authentication header required
    Use: Authorization: Bearer <token>

  url_with_anchor: "https://docs.example.com/api#authentication"

  example_request: 'POST https://api.example.com/v1/users#list

  Headers:
    Authorization: Bearer <token>

  Response: JSON format'

  color_scheme: "#2c3e50"
  fallback_color: '#34495e'
"##;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify structure
    assert!(!is_comment_line(lines[1]));     // endpoint: "..."
    assert!(!is_comment_line(lines[2]));     // documentation: |
    assert!(!is_comment_line(lines[7]));     // url_with_anchor: "..."

    // Hash in quoted URL should be preserved
    let stripped = strip_inline_comment(lines[7]);
    assert!(stripped.contains('#'));

    assert!(!is_comment_line(lines[9]));     // example_request: '...'
    assert!(!is_comment_line(lines[16]));    // color_scheme: "#..."

    // Hash in color code should be preserved
    let stripped = strip_inline_comment(lines[16]);
    assert!(stripped.contains('#'));

    assert!(!is_comment_line(lines[17]));    // fallback_color: '#...'

    // Hash in color code should be preserved
    let stripped = strip_inline_comment(lines[17]);
    assert!(stripped.contains('#'));
}

// ============================================================================
// Behavior Documentation Tests
// ============================================================================

#[test]
fn test_document_quoted_scalar_comment_behavior() {
    // This test documents the current behavior of comment detection
    // in multi-line quoted scalars

    // Key behavior 1: Quoted scalars (both " and ') are mapping keys
    let double_quoted = "content: \"text\"";
    let single_quoted = "content: 'text'";
    assert_eq!(classify_line_type(double_quoted), LineType::MappingKey);
    assert_eq!(classify_line_type(single_quoted), LineType::MappingKey);
    assert!(!is_comment_line(double_quoted));
    assert!(!is_comment_line(single_quoted));

    // Key behavior 2: Hash inside quotes is ALWAYS preserved as content
    let double_with_hash = "text: \"value # hash\"";
    let single_with_hash = "text: 'value # hash'";

    let stripped_double = strip_inline_comment(double_with_hash);
    let stripped_single = strip_inline_comment(single_with_hash);

    assert!(stripped_double.contains('#'));
    assert!(stripped_single.contains('#'));

    // Key behavior 3: Inline comments after quotes work correctly
    let double_with_comment = "text: \"value\" # this is a comment";
    let single_with_comment = "text: 'value' # this is a comment";

    let stripped_double_comment = strip_inline_comment(double_with_comment);
    let stripped_single_comment = strip_inline_comment(single_with_comment);

    assert_eq!(stripped_double_comment, "text: \"value\" ");
    assert_eq!(stripped_single_comment, "text: 'value' ");

    // Key behavior 4: Both quote types handle identically
    assert_eq!(is_comment_line(double_with_hash), is_comment_line(single_with_hash));
    assert_eq!(classify_line_type(double_with_hash), classify_line_type(single_with_hash));
}

#[test]
fn test_quoted_vs_block_scalar_comment_handling() {
    // Document that quoted scalars differ from block scalars in newline handling
    // but handle comments similarly (hash inside = content, hash after = comment)

    // Block scalars
    let literal = "text: |";
    let folded = "text: >";

    // Quoted scalars
    let double_quoted = "text: \"multi\"";
    let single_quoted = "text: 'multi'";

    // All are classified as mapping keys
    assert_eq!(classify_line_type(literal), LineType::MappingKey);
    assert_eq!(classify_line_type(folded), LineType::MappingKey);
    assert_eq!(classify_line_type(double_quoted), LineType::MappingKey);
    assert_eq!(classify_line_type(single_quoted), LineType::MappingKey);

    // All preserve hash inside the scalar content
    let literal_with_hash = "text: |";
    let quoted_with_hash = "text: \"#hash\"";

    let stripped_literal = strip_inline_comment(literal_with_hash);
    let stripped_quoted = strip_inline_comment(quoted_with_hash);

    // For block scalars, the line parser doesn't track block context
    // For quoted scalars, hash inside quotes is preserved
    assert!(stripped_quoted.contains('#'));
}
