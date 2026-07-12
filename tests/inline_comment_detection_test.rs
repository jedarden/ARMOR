//! Inline YAML Comment Detection Tests
//!
//! These tests verify the detection of inline YAML comments (comments after values).
//! This is different from comment stripping - these tests verify that we can:
//! 1. Detect whether a line contains an inline comment
//! 2. Extract the comment text from a line
//! 3. Extract the content before the comment
//! 4. Handle edge cases (comments in quotes, URLs with hashes, etc.)
//!
//! Bead: bf-1vqdk
//! Acceptance Criteria:
//! - Test verifies comments after values are detected
//! - Test verifies values before comments are preserved correctly
//! - Test verifies edge cases (comments in quotes, etc.)
//! - All new tests pass

use armor::parsers::yaml::{
    strip_inline_comment, classify_line_type, is_comment_line, LineType
};

/// Detect if a line contains an inline comment
///
/// This helper function checks if a line has an inline comment (hash preceded by whitespace)
/// that is not inside quotes or part of a URL. Importantly, it distinguishes between
/// full-line comments (which start with #) and inline comments (which have content before the #).
fn has_inline_comment(line: &str) -> bool {
    // First check if this is a full-line comment (starts with # after trimming)
    let trimmed = line.trim();
    if trimmed.starts_with('#') {
        return false; // This is a full-line comment, not an inline comment
    }

    let stripped = strip_inline_comment(line);
    // If stripping changes the line and there was content before the comment, it's an inline comment
    if stripped.len() < line.len() {
        // Check that there's actual content (not just whitespace) before the comment
        let content_before_comment = stripped.trim();
        // If there's content before the comment, it's an inline comment
        !content_before_comment.is_empty()
    } else {
        false
    }
}

/// Extract the inline comment text from a line
///
/// This helper function extracts just the comment text (after the #) from a line.
/// Returns None if there is no inline comment (e.g., full-line comment or no comment at all).
fn extract_inline_comment(line: &str) -> Option<String> {
    // First check if this is a full-line comment (starts with # after trimming)
    let trimmed = line.trim();
    if trimmed.starts_with('#') {
        return None; // This is a full-line comment, not an inline comment
    }

    let mut in_single_quote = false;
    let mut in_double_quote = false;
    let mut escaped = false;
    let mut prev_char: Option<char> = None;
    let mut chars = line.chars().peekable();

    while let Some(ch) = chars.next() {
        if escaped {
            prev_char = Some(ch);
            escaped = false;
            continue;
        }

        match ch {
            '\\' => {
                prev_char = Some(ch);
                escaped = true;
            }
            '\'' if !in_double_quote => {
                in_single_quote = !in_single_quote;
                prev_char = Some(ch);
            }
            '"' if !in_single_quote => {
                in_double_quote = !in_double_quote;
                prev_char = Some(ch);
            }
            '#' if !in_single_quote && !in_double_quote => {
                // Check if this # is preceded by whitespace (YAML comment rule)
                let is_comment = prev_char.map_or(true, |c| c.is_whitespace());

                if is_comment {
                    // Found comment start - extract the rest
                    let comment_text: String = chars.collect();
                    return Some(comment_text.trim().to_string());
                } else {
                    // # is part of the value (like in URL)
                    prev_char = Some(ch);
                }
            }
            _ => {
                prev_char = Some(ch);
            }
        }
    }

    None
}

/// Extract the content before an inline comment
///
/// This helper function extracts just the content before an inline comment.
fn extract_content_before_comment(line: &str) -> String {
    strip_inline_comment(line)
}

// Tests for inline comment detection

#[test]
fn test_detect_inline_comment_basic_scalar_value() {
    // Basic scalar value with inline comment
    let line = "key: value # this is a comment";
    assert!(has_inline_comment(line), "Should detect inline comment in scalar value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("this is a comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: value ");
}

#[test]
fn test_detect_inline_comment_numeric_value() {
    // Numeric value with inline comment
    let line = "timeout: 30 # seconds before timeout";
    assert!(has_inline_comment(line), "Should detect inline comment with numeric value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("seconds before timeout".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "timeout: 30 ");
}

#[test]
fn test_detect_inline_comment_boolean_value() {
    // Boolean value with inline comment
    let line = "enabled: true # feature is enabled";
    assert!(has_inline_comment(line), "Should detect inline comment with boolean value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("feature is enabled".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "enabled: true ");
}

#[test]
fn test_detect_inline_comment_string_value() {
    // String value with inline comment
    let line = "name: John Doe # user name";
    assert!(has_inline_comment(line), "Should detect inline comment with string value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("user name".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "name: John Doe ");
}

#[test]
fn test_detect_inline_comment_quoted_string_value() {
    // Quoted string value with inline comment
    let line = "message: \"Hello World\" # greeting message";
    assert!(has_inline_comment(line), "Should detect inline comment with quoted string value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("greeting message".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "message: \"Hello World\" ");
}

#[test]
fn test_detect_inline_comment_list_item_basic() {
    // List item with inline comment
    let line = "- item1 # first item";
    assert!(has_inline_comment(line), "Should detect inline comment in list item");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("first item".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "- item1 ");
}

#[test]
fn test_detect_inline_comment_list_item_with_value() {
    // List item with complex value and inline comment
    let line = "- name: Alice # user name";
    assert!(has_inline_comment(line), "Should detect inline comment in list item with value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("user name".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "- name: Alice ");
}

#[test]
fn test_detect_inline_comment_list_item_nested() {
    // Nested list item with inline comment
    let line = "  - subitem # nested item";
    assert!(has_inline_comment(line), "Should detect inline comment in nested list item");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("nested item".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "  - subitem ");
}

#[test]
fn test_detect_inline_comment_list_item_multiple() {
    // Multiple list items with inline comments
    let lines = vec![
        "- item1 # first",
        "- item2 # second",
        "- item3 # third",
    ];

    for line in lines {
        assert!(has_inline_comment(line), "Should detect inline comment in: {}", line);
    }
}

#[test]
fn test_detect_inline_comment_preserves_quoted_hashes() {
    // Hash in quoted string should not be detected as comment
    let line = "key: \"value with # hash\" # this is a comment";
    assert!(has_inline_comment(line), "Should detect inline comment after quoted string");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("this is a comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: \"value with # hash\" ");
}

#[test]
fn test_detect_inline_comment_preserves_single_quoted_hashes() {
    // Hash in single-quoted string should not be detected as comment
    let line = "key: 'value with # hash' # this is a comment";
    assert!(has_inline_comment(line), "Should detect inline comment after single-quoted string");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("this is a comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: 'value with # hash' ");
}

#[test]
fn test_detect_inline_comment_preserves_url_hashes() {
    // Hash in URL should not be detected as comment
    let line = "url: http://example.com#anchor # this is a comment";
    assert!(has_inline_comment(line), "Should detect inline comment after URL");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("this is a comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "url: http://example.com#anchor ");
}

#[test]
fn test_detect_inline_comment_hash_without_whitespace() {
    // Hash without preceding whitespace is part of value, not comment
    let line = "key: value#notacomment";
    assert!(!has_inline_comment(line), "Should not detect comment when # has no preceding whitespace");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, None, "Should not extract comment when # is part of value");

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: value#notacomment");
}

#[test]
fn test_detect_inline_comment_hash_without_whitespace_multiple() {
    // Multiple hashes without preceding whitespace are part of value
    let line = "key: value#hash#in#value";
    assert!(!has_inline_comment(line), "Should not detect comment when multiple # have no preceding whitespace");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, None);

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: value#hash#in#value");
}

#[test]
fn test_detect_inline_comment_no_comment_present() {
    // Lines without inline comments
    let lines = vec![
        "key: value",
        "name: John Doe",
        "- item",
        "  nested: value",
        "just plain text",
    ];

    for line in lines {
        assert!(!has_inline_comment(line), "Should not detect comment in: {}", line);
        assert_eq!(extract_inline_comment(line), None, "Should not extract comment from: {}", line);
        assert_eq!(extract_content_before_comment(line), line, "Content should equal original line: {}", line);
    }
}

#[test]
fn test_detect_inline_comment_with_various_indentation() {
    // Inline comments at various indentation levels
    let lines = vec![
        ("key: value # comment", 0),
        ("  key: value # comment", 2),
        ("    key: value # comment", 4),
        ("      key: value # comment", 6),
        ("\tkey: value # comment", 1), // Tab counts as 1
    ];

    for (line, expected_indent) in lines {
        assert!(has_inline_comment(line), "Should detect inline comment at indent {}: {}", expected_indent, line);
        let comment = extract_inline_comment(line);
        assert_eq!(comment, Some("comment".to_string()));
    }
}

#[test]
fn test_detect_inline_comment_complex_real_world_examples() {
    // Real-world YAML configuration examples
    let test_cases = vec![
        (
            "database: \"postgresql://localhost:5432/db#schema\" # production database",
            Some("production database"),
            "database: \"postgresql://localhost:5432/db#schema\" "
        ),
        (
            "api: \"https://api.example.com/v1#endpoint\" # production API",
            Some("production API"),
            "api: \"https://api.example.com/v1#endpoint\" "
        ),
        (
            "timeout: 30 # seconds before timeout",
            Some("seconds before timeout"),
            "timeout: 30 "
        ),
        (
            "retry: 3 # number of retry attempts",
            Some("number of retry attempts"),
            "retry: 3 "
        ),
    ];

    for (line, expected_comment, expected_content) in test_cases {
        assert!(has_inline_comment(line), "Should detect inline comment in: {}", line);
        let comment = extract_inline_comment(line);
        assert_eq!(comment, expected_comment.map(|s| s.to_string()));
        let content = extract_content_before_comment(line);
        assert_eq!(content, expected_content);
    }
}

#[test]
fn test_detect_inline_comment_mixed_quotes_and_hashes() {
    // Complex cases with mixed quotes and hash characters
    let test_cases = vec![
        (
            "key: \"double 'inner' # hash\" # comment",
            Some("comment"),
            "key: \"double 'inner' # hash\" "
        ),
        (
            "key: 'single \"inner\" # hash' # comment",
            Some("comment"),
            "key: 'single \"inner\" # hash' "
        ),
        (
            "key: \"value #1\" # comment",
            Some("comment"),
            "key: \"value #1\" "
        ),
        (
            "key: 'value #2' # comment",
            Some("comment"),
            "key: 'value #2' "
        ),
    ];

    for (line, expected_comment, expected_content) in test_cases {
        assert!(has_inline_comment(line), "Should detect inline comment in: {}", line);
        let comment = extract_inline_comment(line);
        assert_eq!(comment, expected_comment.map(|s| s.to_string()));
        let content = extract_content_before_comment(line);
        assert_eq!(content, expected_content);
    }
}

#[test]
fn test_detect_inline_comment_comment_text_extraction() {
    // Test extraction of various comment text patterns
    let test_cases = vec![
        ("key: value # comment", "comment"),
        ("key: value # multi word comment", "multi word comment"),
        ("key: value # comment-with-dashes", "comment-with-dashes"),
        ("key: value # comment_with_underscores", "comment_with_underscores"),
        ("key: value # 123 numbers", "123 numbers"),
        ("key: value # !@#$ special chars", "!@#$ special chars"),
    ];

    for (line, expected_comment_text) in test_cases {
        let comment = extract_inline_comment(line);
        assert_eq!(comment, Some(expected_comment_text.to_string()));
    }
}

#[test]
fn test_detect_inline_comment_empty_comment_text() {
    // Comment with just whitespace after #
    let line = "key: value # ";
    assert!(has_inline_comment(line), "Should detect inline comment even with empty text");

    let comment = extract_inline_comment(line);
    // Comment text is empty (just whitespace trimmed)
    assert_eq!(comment, Some("".to_string()));
}

#[test]
fn test_detect_inline_comment_multiple_hashes_in_comment() {
    // Comment text itself contains hash characters
    let line = "key: value # this is # not # multiple comments";
    assert!(has_inline_comment(line), "Should detect inline comment");

    let comment = extract_inline_comment(line);
    // All text after first # is the comment
    assert_eq!(comment, Some("this is # not # multiple comments".to_string()));
}

#[test]
fn test_detect_inline_comment_no_false_positives() {
    // Lines that should NOT be detected as having inline comments
    let lines = vec![
        "key: value#notacomment", // No whitespace before #
        "key: value#hash#in#value", // Multiple # without whitespace
        "url: http://example.com#anchor", // Hash in URL
        "time: 12:30:00", // Time format
        "key: \"value # hash\"", // Hash in quotes (no trailing comment)
        "key: 'value # hash'", // Hash in single quotes (no trailing comment)
        "# full line comment", // Full-line comment, not inline
        "  # indented comment", // Indented full-line comment
    ];

    for line in lines {
        assert!(!has_inline_comment(line), "Should not detect inline comment in: {}", line);
    }
}

#[test]
fn test_detect_inline_comment_classification_vs_full_line_comment() {
    // Verify that inline comments are different from full-line comments
    let inline_line = "key: value # inline comment";
    let full_comment_line = "# this is a full-line comment";

    // Inline line should be classified as content, not comment
    assert_eq!(classify_line_type(inline_line), LineType::MappingKey);
    assert!(!is_comment_line(inline_line));

    // Full comment line should be classified as comment
    assert_eq!(classify_line_type(full_comment_line), LineType::Comment);
    assert!(is_comment_line(full_comment_line));

    // But inline line should still be detected as having an inline comment
    assert!(has_inline_comment(inline_line));
    assert!(!has_inline_comment(full_comment_line));
}

#[test]
fn test_detect_inline_comment_preserves_leading_whitespace() {
    // Leading whitespace should be preserved in content extraction
    let lines = vec![
        ("  key: value # comment", "  key: value "),
        ("    key: value # comment", "    key: value "),
        ("\tkey: value # comment", "\tkey: value "),
        ("  \t key: value # comment", "  \t key: value "),
    ];

    for (line, expected_content) in lines {
        let content = extract_content_before_comment(line);
        assert_eq!(content, expected_content, "Leading whitespace should be preserved in: {}", line);
    }
}

#[test]
fn test_detect_inline_comment_trailing_whitespace_preservation() {
    // Trailing whitespace before comment should be preserved
    let line = "key: value   # comment";
    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: value   ", "Trailing whitespace before # should be preserved");
}

#[test]
fn test_detect_inline_comment_tab_before_hash() {
    // Tab before hash should start comment
    let line = "key: value\t# comment";
    assert!(has_inline_comment(line), "Tab before # should start comment");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: value\t");
}

#[test]
fn test_detect_inline_comment_complex_nested_structure() {
    // Test inline comments in complex nested YAML structure
    let yaml_lines = vec![
        "parent: # parent key",
        "  child1: value1 # first child",
        "  child2: value2 # second child",
        "  nested: # nested section",
        "    deeply: value # deep value",
        "  - list item # list comment",
        "  - another item # another comment",
    ];

    // All lines with inline comments should be detected
    let lines_with_comments = vec![
        "parent: # parent key",
        "  child1: value1 # first child",
        "  child2: value2 # second child",
        "  nested: # nested section",
        "    deeply: value # deep value",
        "  - list item # list comment",
        "  - another item # another comment",
    ];

    for line in lines_with_comments {
        assert!(has_inline_comment(line), "Should detect inline comment in: {}", line);
    }
}

#[test]
fn test_detect_inline_comment_edge_case_just_hash() {
    // Edge case: line with just "#"
    let line = "#";
    // This is a full-line comment, not an inline comment
    assert!(!has_inline_comment(line), "Single # is full-line comment, not inline");
    assert_eq!(extract_inline_comment(line), None);
}

#[test]
fn test_detect_inline_comment_edge_case_hash_with_space() {
    // Edge case: line with "# "
    let line = "# ";
    // This is a full-line comment, not an inline comment
    assert!(!has_inline_comment(line), "# followed by space is full-line comment, not inline");
    assert_eq!(extract_inline_comment(line), None);
}

#[test]
fn test_detect_inline_comment_edge_case_value_space_hash() {
    // Edge case: value followed by space and hash
    let line = "key: value #";
    assert!(has_inline_comment(line), "Should detect inline comment even with no comment text");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("".to_string()));
}

#[test]
fn test_detect_inline_comment_sequence_item_with_nested_mapping() {
    // Sequence item with nested mapping and inline comment
    let line = "- key: value # nested mapping in sequence";
    assert!(has_inline_comment(line), "Should detect inline comment in complex sequence item");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("nested mapping in sequence".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "- key: value ");
}

#[test]
fn test_detect_inline_comment_flow_style_mapping() {
    // Flow style mapping with inline comment
    let line = "config: {key: value} # flow style mapping";
    assert!(has_inline_comment(line), "Should detect inline comment after flow style mapping");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("flow style mapping".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "config: {key: value} ");
}

#[test]
fn test_detect_inline_comment_flow_style_sequence() {
    // Flow style sequence with inline comment
    let line = "items: [one, two, three] # flow style sequence";
    assert!(has_inline_comment(line), "Should detect inline comment after flow style sequence");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("flow style sequence".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "items: [one, two, three] ");
}

#[test]
fn test_detect_inline_comment_ipv6_address() {
    // IPv6 address with inline comment
    let line = "address: 2001:db8::1 # IPv6 address";
    assert!(has_inline_comment(line), "Should detect inline comment after IPv6 address");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("IPv6 address".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "address: 2001:db8::1 ");
}

#[test]
fn test_detect_inline_comment_escaped_quotes_in_value() {
    // Value with escaped quotes and inline comment
    let line = "key: \"value with \\\" escaped quote\" # comment";
    assert!(has_inline_comment(line), "Should detect inline comment with escaped quotes");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: \"value with \\\" escaped quote\" ");
}

#[test]
fn test_detect_inline_comment_multiple_spaces_before_hash() {
    // Multiple spaces before hash should still start comment
    let line = "key: value     # comment";
    assert!(has_inline_comment(line), "Multiple spaces before # should start comment");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: value     ");
}

#[test]
fn test_detect_inline_comment_special_characters_in_comment() {
    // Comments with special characters
    let test_cases = vec![
        ("key: value # TODO: fix this bug", "TODO: fix this bug"),
        ("key: value # FIXME: broken code", "FIXME: broken code"),
        ("key: value # NOTE: important info", "NOTE: important info"),
        ("key: value # @mention user", "@mention user"),
        ("key: value # https://example.com", "https://example.com"),
    ];

    for (line, expected_comment) in test_cases {
        assert!(has_inline_comment(line), "Should detect inline comment with special chars: {}", line);
        let comment = extract_inline_comment(line);
        assert_eq!(comment, Some(expected_comment.to_string()));
    }
}

#[test]
fn test_detect_inline_comment_unicode_values() {
    // Unicode values with inline comments
    let test_cases = vec![
        ("name: José # Spanish name", "Spanish name"),
        ("city: München # German city", "German city"),
        ("greeting:こんにちは # Japanese greeting", "Japanese greeting"),
        ("emoji: 👍 # thumbs up", "thumbs up"),
    ];

    for (line, expected_comment) in test_cases {
        assert!(has_inline_comment(line), "Should detect inline comment with Unicode: {}", line);
        let comment = extract_inline_comment(line);
        assert_eq!(comment, Some(expected_comment.to_string()));
    }
}

#[test]
fn test_detect_inline_comment_empty_value() {
    // Empty value with inline comment
    let line = "key: # empty value with comment";
    assert!(has_inline_comment(line), "Should detect inline comment with empty value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("empty value with comment".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: ");
}

#[test]
fn test_detect_inline_comment_null_value() {
    // Null value with inline comment
    let line = "key: null # null value";
    assert!(has_inline_comment(line), "Should detect inline comment with null value");

    let comment = extract_inline_comment(line);
    assert_eq!(comment, Some("null value".to_string()));

    let content = extract_content_before_comment(line);
    assert_eq!(content, "key: null ");
}

#[test]
fn test_detect_inline_comment_integration_complete_document() {
    // Test inline comment detection in a complete YAML document
    let yaml = r#"
# Configuration file
database: postgres # main database
  host: localhost # database host
  port: 5432 # default PostgreSQL port
  name: mydb # database name

# Server settings
server:
  host: localhost # server host
  port: 8080 # server port

# List of users
users:
  - name: Alice # admin user
    role: admin # admin role
  - name: Bob # regular user
    role: user # regular role
"#;

    let lines_with_inline_comments = vec![
        "database: postgres # main database",
        "  host: localhost # database host",
        "  port: 5432 # default PostgreSQL port",
        "  name: mydb # database name",
        "  host: localhost # server host",
        "  port: 8080 # server port",
        "  - name: Alice # admin user",
        "    role: admin # admin role",
        "  - name: Bob # regular user",
        "    role: user # regular role",
    ];

    for expected_line in lines_with_inline_comments {
        let found = yaml.lines().any(|line| line == expected_line);
        assert!(found, "Expected line not found in YAML: {}", expected_line);

        if found {
            assert!(has_inline_comment(expected_line),
                   "Should detect inline comment in: {}", expected_line);
        }
    }
}