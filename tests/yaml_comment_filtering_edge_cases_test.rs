//! YAML Comment Filtering Edge Cases and False Positive Prevention Tests
//!
//! These tests verify YAML comment filtering behavior for edge cases and
//! prevention of false positives when distinguishing comments from content.
//!
//! Bead: bf-13c81
//! Acceptance Criteria:
//! - Tests for false positives (hashes in values, URLs with anchors)
//! - Tests for edge cases in comment detection
//! - Tests for special characters and patterns
//! - All tests pass

use armor::parsers::yaml::{
    classify_line_type, is_comment_line, strip_inline_comment, LineType
};

// ============================================================================
// False Positive Prevention - Hashes in Values
// ============================================================================

#[test]
fn test_hash_in_color_values_unquoted() {
    // Color hex values WITHOUT space before # are preserved
    // Color hex values WITH space before # are treated as comments (YAML spec)
    let test_cases = vec![
        // No space before # - hash is part of value
        ("color:#FFFFFF", "color:#FFFFFF"),
        ("color:#000000", "color:#000000"),
        ("background:#ff0000", "background:#ff0000"),
        // Space before # - hash starts a comment
        ("color: #FFFFFF", "color: "),
        ("background: #ff0000", "background: "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Color value handling: got {}", stripped);
    }
}

#[test]
fn test_hash_in_urls_with_anchors() {
    // URL anchors (the # in http://example.com#section) should be preserved
    let test_cases = vec![
        ("url: http://example.com#section", "url: http://example.com#section"),
        ("link: https://api.example.com/v1#endpoint", "link: https://api.example.com/v1#endpoint"),
        ("doc: http://localhost:8080/docs#intro", "doc: http://localhost:8080/docs#intro"),
        ("href: https://example.com/path#fragment", "href: https://example.com/path#fragment"),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "URL with anchor should not be comment line: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey, "URL with anchor should be mapping key: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "URL anchor should be preserved: got {}", stripped);
    }
}

#[test]
fn test_hash_in_urls_with_comment_after() {
    // URL anchors preserved, real comment stripped
    let test_cases = vec![
        ("url: http://example.com#section # this is a comment", "url: http://example.com#section "),
        ("link: https://api.com#endpoint # API link", "link: https://api.com#endpoint "),
        ("home: http://site.com#top # homepage", "home: http://site.com#top "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "URL anchor preserved, comment stripped: got {}", stripped);
        assert!(stripped.contains('#'), "URL anchor hash should be preserved in: {}", stripped);
    }
}

#[test]
fn test_hash_in_css_selectors() {
    // CSS ID selectors WITHOUT space before # are preserved
    // WITH space before #, they're treated as comments (YAML spec)
    let test_cases = vec![
        // No space before # - hash is part of value
        ("selector:#main-content", "selector:#main-content"),
        ("element:#container", "element:#container"),
        ("id:submit-button", "id:submit-button"),
        // Space before # - hash starts a comment
        ("selector: #main-content", "selector: "),
        ("element: #container", "element: "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "CSS selector handling: got {}", stripped);
    }
}

#[test]
fn test_hash_in_html_entity_references() {
    // HTML entity number references WITHOUT space before # are preserved
    // WITH space before #, they're treated as comments (YAML spec)
    let test_cases = vec![
        // No space before # - hash is part of value
        ("entity:#38", "entity:#38"),  // &#38; is &
        ("code:#169", "code:#169"),   // &#169; is ©
        // Space before # - hash starts a comment
        ("entity: #38", "entity: "),
        ("code: #169", "code: "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Entity reference handling: got {}", stripped);
    }
}

#[test]
fn test_hash_in_numeric_values() {
    // Numeric values with # (special notation) should be preserved
    let test_cases = vec![
        ("note: C# major", "note: C# major"),
        ("key: F# minor", "key: F# minor"),
        ("language: C#", "language: C#"),
        ("version: R#2", "version: R#2"),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Numeric with # should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Numeric with # should be preserved: got {}", stripped);
    }
}

#[test]
fn test_hash_in_programming_identifiers() {
    // Programming directives (C preprocessor, etc.) - WITHOUT space before # are preserved
    // WITH space before #, they're treated as comments (YAML spec)
    let test_cases = vec![
        // No space before # - hash is part of value
        ("directive:#ifdef", "directive:#ifdef"),
        ("preprocessor:#include", "preprocessor:#include"),
        ("directive:#define", "directive:#define"),
        // Space before # - hash starts a comment
        ("directive: #ifdef", "directive: "),
        ("preprocessor: #include", "preprocessor: "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Directive handling: got {}", stripped);
    }
}

#[test]
fn test_hash_immediately_following_value() {
    // Hash immediately after value (no space) is part of value
    let test_cases = vec![
        ("tag: value#tag", "tag: value#tag"),
        ("id: item#123", "id: item#123"),
        ("ref: section#1", "ref: section#1"),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Value with # should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Value with # should be preserved: got {}", stripped);
    }
}

// ============================================================================
// False Positive Prevention - Special Contexts
// ============================================================================

#[test]
fn test_hash_in_quoted_strings_double_quotes() {
    // Hashes inside double-quoted strings are always preserved
    let test_cases = vec![
        ("text: \"hello#world\"", "text: \"hello#world\""),
        ("message: \"value # with hash\"", "message: \"value # with hash\""),
        ("quoted: \"#hashtag\"", "quoted: \"#hashtag\""),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Quoted value should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Hash in quotes should be preserved: got {}", stripped);
    }
}

#[test]
fn test_hash_in_quoted_strings_single_quotes() {
    // Hashes inside single-quoted strings are always preserved
    let test_cases = vec![
        ("text: 'hello#world'", "text: 'hello#world'"),
        ("message: 'value # with hash'", "message: 'value # with hash'"),
        ("quoted: '#hashtag'", "quoted: '#hashtag'"),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Quoted value should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Hash in quotes should be preserved: got {}", stripped);
    }
}

#[test]
fn test_hash_in_quoted_string_with_comment_after() {
    // Hash in quoted string is preserved, comment after is stripped
    let test_cases = vec![
        ("color: \"#FFFFFF\" # white", "color: \"#FFFFFF\" "),
        ("text: \"value#hash\" # comment", "text: \"value#hash\" "),
        ("msg: 'hello#world' # inline", "msg: 'hello#world' "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Hash in quotes preserved, comment stripped: got {}", stripped);
    }
}

#[test]
fn test_escaped_hash_in_strings() {
    // Escaped hash characters should be preserved
    let test_cases = vec![
        ("text: \"value\\#notcomment\"", "text: \"value\\#notcomment\""),
        ("path: \"path\\#to\\#file\"", "path: \"path\\#to\\#file\""),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Escaped hash should not create comment: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Escaped hash should be preserved: got {}", stripped);
    }
}

#[test]
fn test_hash_in_flow_sequences() {
    // Hashes in flow sequences [value#hash] should be preserved
    let test_cases = vec![
        ("tags: [item#1, item#2]", "tags: [item#1, item#2]"),
        ("list: [http://a.com#1, http://b.com#2]", "list: [http://a.com#1, http://b.com#2]"),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Flow sequence should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Hash in flow sequence should be preserved: got {}", stripped);
    }
}

#[test]
fn test_hash_in_flow_mappings() {
    // Hashes in flow mappings - behavior depends on preceding character
    let test_cases = vec![
        // Hash without preceding space - preserved
        ("config: {url: http://example.com#anchor}", "config: {url: http://example.com#anchor}"),
        ("data: {color:#FFFFFF}", "data: {color:#FFFFFF}"),
        // Hash with preceding space - treated as comment
        ("data: {color: #FFFFFF}", "data: {color: "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Flow mapping should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Hash in flow mapping: got {}", stripped);
    }
}

// ============================================================================
// Edge Cases - Comment Detection
// ============================================================================

#[test]
fn test_empty_comment() {
    // Comment with nothing after #
    assert!(is_comment_line("#"));
    assert_eq!(classify_line_type("#"), LineType::Comment);
    assert_eq!(strip_inline_comment("#"), "");
}

#[test]
fn test_comment_with_only_whitespace_after_hash() {
    // Comment with only whitespace after #
    let test_cases = vec![
        ("# ", "# "),
        ("#   ", "#   "),
        ("#\t", "#\t"),
        ("#  \t ", "#  \t "),
    ];

    for (line, _expected) in test_cases {
        assert!(is_comment_line(line), "Should be comment line: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert_eq!(strip_inline_comment(line), "", "Empty comment should strip to empty: {:?}", line);
    }
}

#[test]
fn test_multiple_consecutive_hashes() {
    // Multiple consecutive # symbols at line start
    let test_cases = vec![
        ("##", "##"),
        ("###", "###"),
        ("####", "####"),
        ("#####", "#####"),
    ];

    for (line, _expected) in test_cases {
        assert!(is_comment_line(line), "Multiple hashes should be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert_eq!(strip_inline_comment(line), "", "Should strip to empty: {}", line);
    }
}

#[test]
fn test_multiple_hashes_with_text() {
    // Multiple hashes with text after (markdown-style headers)
    let test_cases = vec![
        ("## Section", "## Section"),
        ("### Subsection", "### Subsection"),
        ("#### Header", "#### Header"),
    ];

    for (line, _expected) in test_cases {
        assert!(is_comment_line(line), "Markdown header should be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert_eq!(strip_inline_comment(line), "", "Should strip to empty: {}", line);
    }
}

#[test]
fn test_comment_like_pattern_in_value() {
    // Patterns that look like comments but are part of values
    let test_cases = vec![
        // Value contains "# text" pattern but isn't a comment
        ("text: This is # not a comment", "text: This is "),
        ("note: TODO # fix this", "note: TODO "),
        // Even without space before #, if followed by space it becomes comment
        ("value: something#comment", "value: something#comment"),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Pattern handling: got {}", stripped);
    }
}

#[test]
fn test_colon_hash_pattern() {
    // Colon followed by hash (: #) - could be key: #value or key: # comment
    let test_cases = vec![
        // # after space is comment
        ("key: #comment", "key: "),
        ("key:   # multiple spaces", "key:   "),
        // # without space is part of value
        ("key:#value", "key:#value"),
        ("key: #value", "key: "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Colon-hash pattern: got {}", stripped);
    }
}

#[test]
fn test_comment_at_end_of_complex_value() {
    // Comments after complex values
    let test_cases = vec![
        // After URL
        ("url: https://example.com/path?query=value # comment", "url: https://example.com/path?query=value "),
        // After timestamp
        ("time: 2024-01-01T12:30:45Z # timestamp", "time: 2024-01-01T12:30:45Z "),
        // After number
        ("count: 42 # number", "count: 42 "),
        // After boolean
        ("flag: true # boolean", "flag: true "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Comment after complex value: got {}", stripped);
    }
}

// ============================================================================
// Edge Cases - Special Characters
// ============================================================================

#[test]
fn test_comment_with_special_characters() {
    // Comments containing various special characters
    let test_cases = vec![
        "# @#$%^&*()",
        "# <tag>{{value}}</tag>",
        "# `backtick` and *asterisk*",
        "# [link](url)",
        "# $100.00",
    ];

    for line in test_cases {
        assert!(is_comment_line(line), "Should be comment line: {}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert_eq!(strip_inline_comment(line), "", "Should strip to empty: {}", line);
    }
}

#[test]
fn test_unicode_in_comments() {
    // Comments with Unicode characters
    let test_cases = vec![
        "# Unicode: ñ, é, 中文, 日本語",
        "# Emojis: 🎉, 🚀, ✨",
        "# RTL: مرحبا",
        "# Symbols: ©, ®, ™",
    ];

    for line in test_cases {
        assert!(is_comment_line(line), "Unicode comment should be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert_eq!(strip_inline_comment(line), "", "Unicode comment should strip: {}", line);
    }
}

#[test]
fn test_comment_with_quotes() {
    // Comments containing quotes
    let test_cases = vec![
        "# 'single quotes'",
        "# \"double quotes\"",
        "# `backticks`",
        "# 'mix \"of\" quotes'",
    ];

    for line in test_cases {
        assert!(is_comment_line(line), "Comment with quotes should be comment: {}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert_eq!(strip_inline_comment(line), "", "Comment with quotes should strip: {}", line);
    }
}

#[test]
fn test_inline_comment_with_special_chars() {
    // Inline comments with special characters
    let test_cases = vec![
        ("key: value # @#$%", "key: value "),
        ("text: hello # <tag>", "text: hello "),
        ("data: test # ©®™", "data: test "),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Special chars in comment: got {}", stripped);
    }
}

// ============================================================================
// Edge Cases - Whitespace and Indentation
// ============================================================================

#[test]
fn test_comment_with_tabs() {
    // Comments with tab characters
    let test_cases = vec![
        "\t# tab comment",
        "  \t# mixed tab-space",
        "#\ttab after hash",
    ];

    for line in test_cases {
        assert!(is_comment_line(line), "Tab comment should be comment: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);

        let stripped = strip_inline_comment(line);
        // Should preserve leading whitespace
        let leading_ws = line.chars().take_while(|c| c.is_whitespace()).collect::<String>();
        assert_eq!(stripped, leading_ws, "Tab comment should preserve leading ws: {:?}", line);
    }
}

#[test]
fn test_inline_comment_with_tab_before_hash() {
    // Tab before hash (not space) - does it start comment?
    // YAML spec says whitespace, which includes tabs
    let test_cases = vec![
        ("key: value\t# comment", "key: value\t"),
        ("item: one\t# inline", "item: one\t"),
    ];

    for (line, expected) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected, "Tab before hash should start comment: got {}", stripped);
    }
}

#[test]
fn test_comment_at_various_indentations() {
    // Comments at different indentation levels
    let test_cases = vec![
        ("# level 0", "", 0),
        ("  # level 2", "  ", 2),
        ("    # level 4", "    ", 4),
        ("      # level 6", "      ", 6),
        ("        # level 8", "        ", 8),
        ("\t# tab indent", "\t", 1),
    ];

    for (line, expected_stripped, _indent) in test_cases {
        assert!(is_comment_line(line), "Should be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped, "Indentation preserved: got {:?}", stripped);
    }
}

// ============================================================================
// Complex Integration Tests
// ============================================================================

#[test]
fn test_complete_document_with_hashes_and_comments() {
    // A realistic YAML document with URLs, colors, and comments
    // NOTE: In YAML, color: #336699 is treated as a comment (space before #)
    // To preserve color values, use quotes or no space: color:#336699 or color: "#336699"
    let yaml = "# Application Configuration\n\
app:\n\
  name: MyApp\n\
  # UI Settings\n\
  theme:\n\
    primary: \"#336699\" # Primary color\n\
    secondary: \"#6699CC\" # Secondary color\n\
  # API Configuration\n\
  api:\n\
    base_url: https://api.example.com/v1 # API base\n\
    endpoints:\n\
      - https://api.example.com/v1#users # Users endpoint\n\
      - https://api.example.com/v1#posts # Posts endpoint\n\
  # Feature flags\n\
  features:\n\
    - name: auth\n\
      enabled: true # Enable authentication\n\
    - name: rate_limit\n\
      enabled: false # Disabled for now\n";

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify specific lines
    // Line 0: # Application Configuration - comment
    assert!(is_comment_line(lines[0]));
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);

    // Line 3: # UI Settings - comment
    assert!(is_comment_line(lines[3]));

    // Line 5: primary: "#336699" # Primary color - quoted color + comment
    assert!(!is_comment_line(lines[5]));
    let stripped = strip_inline_comment(lines[5]);
    assert!(stripped.contains("#336699"), "Color hex in quotes should be preserved in: {}", stripped);
    assert!(!stripped.ends_with('#'), "Comment should be stripped from: {}", stripped);

    // Line 9: base_url: https://api.example.com/v1 # API base - URL + comment
    assert!(!is_comment_line(lines[9]));
    let stripped = strip_inline_comment(lines[9]);
    assert!(stripped.contains("https://"), "URL should be preserved in: {}", stripped);
    assert!(stripped.contains("api.example.com"), "URL domain should be preserved in: {}", stripped);

    // Line 11: - https://api.example.com/v1#users # Users endpoint - URL anchor + comment
    assert!(!is_comment_line(lines[11]));
    let stripped = strip_inline_comment(lines[11]);
    assert!(stripped.contains("#users"), "URL anchor should be preserved in: {}", stripped);
    assert!(stripped.contains("https://"), "URL protocol should be preserved in: {}", stripped);
}

#[test]
fn test_false_positive_prevention_comprehensive() {
    // Comprehensive test for all hash contexts that should NOT be treated as comments
    let test_cases = vec![
        // Color values (no space before #)
        ("color:#FFFFFF", "#FFFFFF"),
        ("background:#000", "#000"),

        // URL anchors (no space before # in URL)
        ("url: http://example.com#section", "#section"),
        ("link: https://api.com/v1#endpoint", "#endpoint"),

        // Programming identifiers (no space before #)
        ("lang:C#", "C#"),
        ("note:C# major", "C#"),
        ("directive:#ifdef", "#ifdef"),

        // CSS selectors (no space before #)
        ("id:#main", "#main"),
        ("selector:#content", "#content"),

        // Quoted strings (hashes inside quotes always preserved)
        ("text: \"value#hash\"", "#hash"),
        ("msg: 'data#tag'", "data#tag"),

        // Numeric references (no space before #)
        ("entity:#38", "#38"),
        ("code:#169", "#169"),
    ];

    for (line, hash_part) in test_cases {
        assert!(!is_comment_line(line), "Should not be comment line: {}", line);

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains(hash_part),
                "Hash should be preserved in {}: got {}", line, stripped);
    }
}

#[test]
fn test_edge_case_boundary_conditions() {
    // Test boundary conditions and edge cases
    let test_cases = vec![
        // Empty/minimal cases
        ("#", "", true, "empty comment"),
        ("# ", "", true, "comment with space"),

        // Single char after hash
        ("#a", "", true, "single char comment"),
        ("# a", "", true, "space and single char"),

        // Hash at boundary
        ("key: value#", "key: value#", false, "hash at end, no comment"),
        ("key: value# ", "key: value# ", false, "hash and space, no comment text"),
        ("key: value #", "key: value ", false, "space hash at end"),

        // Multiple boundary transitions
        ("key: value1#notcomment # comment", "key: value1#notcomment ", false, "hash then comment"),
        ("key: http://a.com#1 #comment", "key: http://a.com#1 ", false, "URL anchor then comment"),
    ];

    for (line, expected_stripped, is_comment, description) in test_cases {
        if is_comment {
            assert!(is_comment_line(line), "{}: should be comment", description);
        } else {
            assert!(!is_comment_line(line), "{}: should not be comment", description);
        }

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped, "{}: got {}", description, stripped);
    }
}
