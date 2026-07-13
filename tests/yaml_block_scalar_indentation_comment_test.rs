//! YAML Comment Detection Near Block Scalars with Various Indentation Levels
//!
//! These tests verify YAML comment detection behavior near block scalars
//! (literal `|` and folded `>`) at various indentation levels.
//!
//! Key aspects tested:
//! - Comments at different indentation levels relative to block scalars
//! - Block scalar content lines that start with `#` at various indentations
//! - How indentation affects comment vs content classification
//! - Edge cases with mixed indentation patterns
//!
//! Bead: bf-791kn
//! Acceptance Criteria:
//! - Test passes for comments near block scalars with various indentation
//! - Test reflects actual parser behavior for indentation contexts
//! - Verify correct classification of comments at different indent levels

use armor::parsers::yaml::{
    classify_line_type, is_comment_line, strip_inline_comment, LineType
};

// ============================================================================
// Literal Block Scalars with Indentation Variations
// ============================================================================

#[test]
fn test_literal_block_scalar_comment_at_base_indent() {
    // Comment at base indentation (level with block scalar marker)
    let test_cases = vec![
        "text: |",
        "# This is a real comment at base indent",
        "  content line",
        "# Another comment at base indent",
    ];

    let lines: Vec<&str> = test_cases;

    // Block scalar marker is a mapping key
    assert_eq!(classify_line_type(lines[0]), LineType::MappingKey);
    assert!(!is_comment_line(lines[0]));

    // Lines starting with # at base indent are comments
    assert!(is_comment_line(lines[1]));
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);

    // Indented content is not a comment
    assert!(!is_comment_line(lines[2]));

    // Another base-level comment
    assert!(is_comment_line(lines[3]));
    assert_eq!(classify_line_type(lines[3]), LineType::Comment);
}

#[test]
fn test_literal_block_scalar_with_indented_content_lines() {
    // Content lines at various indentation levels within literal block
    let test_cases = vec![
        // 0-space indent (not actually valid YAML, but testing parser)
        "# Content at 0 indent",
        // 2-space indent
        "  # Content at 2-space indent",
        // 4-space indent
        "    # Content at 4-space indent",
        // 6-space indent
        "      # Content at 6-space indent",
        // 8-space indent
        "        # Content at 8-space indent",
        // Tab indent
        "\t# Content with tab indent",
    ];

    for line in test_cases {
        // All lines starting with # are classified as comments
        // by the line parser (regardless of indentation level)
        assert!(is_comment_line(line),
               "Line should be classified as comment: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
    }
}

#[test]
fn test_literal_block_scalar_content_with_hash_not_at_line_start() {
    // Content lines where hash appears in middle (not at line start)
    // at various indentation levels
    let test_cases = vec![
        // No leading indent, hash in middle
        ("text#value", "text#value"),
        // 2-space indent, hash in middle
        ("  text#value", "  text#value"),
        // 4-space indent, hash in middle
        ("    text#value", "    text#value"),
        // 8-space indent, hash in middle
        ("        text#value", "        text#value"),
        // Mixed tabs and spaces
        ("\t  text#value", "\t  text#value"),
    ];

    for (line, expected) in test_cases {
        // These don't start with #, so they're not comment lines
        assert!(!is_comment_line(line));

        // Hash should be preserved
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected,
                   "Hash not preceded by space should be preserved: {:?}", line);
    }
}

#[test]
fn test_literal_block_scalar_content_with_space_before_hash() {
    // Content lines where hash is preceded by space (triggers comment)
    // at various indentation levels
    let test_cases = vec![
        // No indent
        ("text # comment", "text "),
        // 2-space indent
        ("  text # comment", "  text "),
        // 4-space indent
        ("    text # comment", "    text "),
        // 8-space indent
        ("        text # comment", "        text "),
        // Tab indent
        ("\ttext # comment", "\ttext "),
    ];

    for (line, expected) in test_cases {
        // These are not comment lines (don't start with #)
        assert!(!is_comment_line(line));

        // Space before # triggers comment stripping
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected,
                   "Space before hash should trigger comment stripping: {:?}", line);
    }
}

// ============================================================================
// Folded Block Scalars with Indentation Variations
// ============================================================================

#[test]
fn test_folded_block_scalar_comment_at_base_indent() {
    // Comment at base indentation (level with block scalar marker)
    let test_cases = vec![
        "text: >",
        "# This is a real comment at base indent",
        "  content line",
        "# Another comment at base indent",
    ];

    let lines: Vec<&str> = test_cases;

    // Block scalar marker is a mapping key
    assert_eq!(classify_line_type(lines[0]), LineType::MappingKey);
    assert!(!is_comment_line(lines[0]));

    // Lines starting with # at base indent are comments
    assert!(is_comment_line(lines[1]));
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);

    // Indented content is not a comment
    assert!(!is_comment_line(lines[2]));

    // Another base-level comment
    assert!(is_comment_line(lines[3]));
    assert_eq!(classify_line_type(lines[3]), LineType::Comment);
}

#[test]
fn test_folded_block_scalar_with_indented_content_lines() {
    // Content lines at various indentation levels within folded block
    let test_cases = vec![
        "  # Content at 2-space indent",
        "    # Content at 4-space indent",
        "      # Content at 6-space indent",
        "        # Content at 8-space indent",
        "\t# Content with tab indent",
        "  \t# Content with mixed space-tab indent",
    ];

    for line in test_cases {
        // All lines starting with # are classified as comments
        // by the line parser regardless of indentation level
        assert!(is_comment_line(line),
               "Line should be classified as comment: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment);
    }
}

#[test]
fn test_folded_block_scalar_preserves_nested_indentation() {
    // Folded blocks preserve nested indentation for content
    // Lines with greater indentation are preserved as line breaks
    let content_lines = vec![
        "  Base level content",
        "    Double-indented content (preserves newline)",
        "      Triple-indented content",
        "  Back to base level",
    ];

    for line in content_lines {
        // None of these are comment lines (don't start with #)
        assert!(!is_comment_line(line),
               "Content line should not be comment: {:?}", line);
    }
}

// ============================================================================
// Block Scalar Markers with Indentation
// ============================================================================

#[test]
fn test_block_scalar_markers_indented_in_mapping() {
    // Block scalar markers at various indentation levels in mappings
    let test_cases = vec![
        "text: |",
        "  text: |",
        "    text: |",
        "      text: |",
        "text: >",
        "  text: >",
        "    text: >",
        "      text: >",
    ];

    for line in test_cases {
        // All block scalar markers are mapping keys
        assert_eq!(classify_line_type(line), LineType::MappingKey,
                   "Block scalar marker should be mapping key: {:?}", line);
        assert!(!is_comment_line(line),
               "Block scalar marker should not be comment: {:?}", line);
    }
}

#[test]
fn test_block_scalar_markers_with_inline_comments() {
    // Block scalar markers with inline comments at various indent levels
    let test_cases = vec![
        ("text: | # comment", "text: | "),
        ("  text: | # comment", "  text: | "),
        ("    text: | # comment", "    text: | "),
        ("text: > # comment", "text: > "),
        ("  text: > # comment", "  text: > "),
        ("    text: > # comment", "    text: > "),
    ];

    for (line, expected) in test_cases {
        // These are not comment lines (mapping keys with inline comments)
        assert!(!is_comment_line(line));

        // Inline comment should be stripped
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected,
                   "Inline comment should be stripped: {:?}", line);
    }
}

// ============================================================================
// Comments Nested in Structures with Block Scalars
// ============================================================================

#[test]
fn test_nested_mapping_with_block_scalar_and_comments() {
    // Comments at various levels in nested structures with block scalars
    let yaml = r#"top:
  # Comment at level 1
  block1: |
    # Looks like comment but is content in literal block
    content line
  # Another comment at level 1
  block2: |
    content
  # Comment at level 1 after second block
  nested:
    # Comment at level 2
    block3: |
      # Content in nested literal block
      more content
    # Comment at level 2 after nested block
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify key classifications
    assert!(!is_comment_line(lines[0]));     // top:
    assert!(is_comment_line(lines[1]));      // # Comment at level 1
    assert!(!is_comment_line(lines[2]));     // block1: |
    assert!(is_comment_line(lines[3]));      // # Looks like comment...
    assert!(!is_comment_line(lines[4]));     // content line
    assert!(is_comment_line(lines[5]));      // # Another comment at level 1
    assert!(!is_comment_line(lines[6]));     // block2: |
    assert!(!is_comment_line(lines[7]));     // content
    assert!(is_comment_line(lines[8]));      // # Comment at level 1 after second block
    assert!(!is_comment_line(lines[9]));     // nested:
    assert!(is_comment_line(lines[10]));     // # Comment at level 2
    assert!(!is_comment_line(lines[11]));    // block3: |
    assert!(is_comment_line(lines[12]));     // # Content in nested...
    assert!(!is_comment_line(lines[13]));    // more content
    assert!(is_comment_line(lines[14]));     // # Comment at level 2...
}

#[test]
fn test_sequence_with_block_scalars_and_comments() {
    // Comments in sequences containing block scalars
    let yaml = r#"items:
  # Comment before first item
  - name: item1
    description: |
      # Content in block
      text
  # Comment between items
  - name: item2
    description: >
      # Folded content
      text
  # Comment after items
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify structure
    assert!(!is_comment_line(lines[0]));     // items:
    assert!(is_comment_line(lines[1]));      // # Comment before first item
    assert!(!is_comment_line(lines[2]));     // - name: item1
    assert!(!is_comment_line(lines[3]));     // description: |
    assert!(is_comment_line(lines[4]));      // # Content in block
    assert!(!is_comment_line(lines[5]));     // text
    assert!(is_comment_line(lines[6]));      // # Comment between items
    assert!(!is_comment_line(lines[7]));     // - name: item2
    assert!(!is_comment_line(lines[8]));     // description: >
    assert!(is_comment_line(lines[9]));      // # Folded content
    assert!(!is_comment_line(lines[10]));    // text
    assert!(is_comment_line(lines[11]));    // # Comment after items
}

// ============================================================================
// Edge Cases: Indentation Ambiguity
// ============================================================================

#[test]
fn test_block_scalar_with_ambiguous_indentation() {
    // Cases where indentation makes it unclear if # is comment or content
    let test_cases = vec![
        // Same indent as marker - definitely a comment
        ("text: |", "# Comment at marker level"),
        // Less indent than content but more than marker - ambiguous
        ("  text: |", " # Less indented than content"),
        // Exactly at content indent level
        ("  text: |", "  # At content indent level"),
        // More indented than typical content
        ("  text: |", "    # Extra indented"),
    ];

    for (marker, content_line) in test_cases {
        // Marker is always a mapping key
        assert_eq!(classify_line_type(marker), LineType::MappingKey);
        assert!(!is_comment_line(marker));

        // Lines starting with # are classified as comments
        // by line parser (regardless of indent context)
        assert!(is_comment_line(content_line));
        assert_eq!(classify_line_type(content_line), LineType::Comment);
    }
}

#[test]
fn test_block_scalar_with_inconsistent_indentation() {
    // Block scalars with inconsistent indentation in content
    let content_variations = vec![
        // Mix of 2-space and 4-space indent
        vec!["  two spaces", "    four spaces", "  back to two"],
        // Mix of tabs and spaces
        vec!["\ttab", "  space", "\t  mixed"],
        // Inconsistent indentation that looks like comments
        vec!["  # two-space hash", "    # four-space hash", "      # six-space hash"],
    ];

    for content_group in content_variations {
        for line in content_group {
            let trimmed = line.trim();
            if trimmed.starts_with('#') {
                // Lines starting with # are comments to line parser
                assert!(is_comment_line(line));
            } else {
                // Other content lines are not comments
                assert!(!is_comment_line(line));
            }
        }
    }
}

#[test]
fn test_block_scalar_blank_lines_at_various_indents() {
    // Blank lines at various indentation levels
    let blank_lines = vec![
        "",           // Truly blank
        "  ",         // 2 spaces
        "    ",       // 4 spaces
        "\t",         // Tab
        "  \t  ",     // Mixed
    ];

    for line in blank_lines {
        // Blank/whitespace-only lines are not comment lines
        assert!(!is_comment_line(line),
               "Blank line should not be comment: {:?}", line);
    }
}

// ============================================================================
// Integration: Complex Multi-Level YAML
// ============================================================================

#[test]
fn test_complex_yaml_with_block_scalars_at_multiple_levels() {
    // Realistic complex YAML with block scalars at multiple indentation levels
    let yaml = r#"config:
  # Top-level comment

  # Documentation block
  documentation: |
    # This is documentation content
    # Multiple lines of docs
    Including code examples

  # Settings block
  settings: >
    Configuration settings
    # With various indentations
    And notes

  nested:
    # Nested level comment

    # Nested documentation
    nested_doc: |
      # Deep content
      # More deep content
      value = key#hash

    # Another nested comment
    value: test
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Track comment classifications throughout
    let mut comment_count = 0;
    let mut content_count = 0;

    for (i, line) in lines.iter().enumerate() {
        if line.trim().is_empty() {
            // Blank lines are not comments
            assert!(!is_comment_line(line), "Blank line at {} should not be comment", i);
            continue;
        }

        let is_com = is_comment_line(line);

        if line.trim().starts_with('#') {
            // Lines starting with # should be comments
            assert!(is_com, "Line {} should be comment: {:?}", i, line);
            comment_count += 1;
        } else if line.ends_with('|') || line.ends_with('>') {
            // Block scalar markers are not comments
            assert!(!is_com, "Block marker at {} should not be comment: {:?}", i, line);
            content_count += 1;
        } else if line.contains(':') {
            // Mapping keys are not comments
            assert!(!is_com, "Mapping key at {} should not be comment: {:?}", i, line);
            content_count += 1;
        } else {
            // Other content (like indented block content)
            // May or may not be comment depending on if it starts with #
            if is_com {
                comment_count += 1;
            } else {
                content_count += 1;
            }
        }
    }

    // We should have found both comments and content
    assert!(comment_count > 0, "Should have found comments");
    assert!(content_count > 0, "Should have found content");
}

// ============================================================================
// Behavior Documentation
// ============================================================================

#[test]
fn test_document_block_scalar_indentation_behavior() {
    // Document how indentation affects comment detection near block scalars

    // Fact 1: Block scalar markers are mapping keys regardless of indent
    let markers = vec![
        "text: |", "  text: |", "    text: |",
        "text: >", "  text: >", "    text: >",
    ];
    for marker in markers {
        assert_eq!(classify_line_type(marker), LineType::MappingKey);
        assert!(!is_comment_line(marker));
    }

    // Fact 2: Lines starting with # are comments regardless of indent
    let hash_lines = vec![
        "# comment", "  # comment", "    # comment", "\t# comment",
    ];
    for line in hash_lines {
        assert!(is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::Comment);
    }

    // Fact 3: Inline comments work at any indent level
    let inline_cases = vec![
        ("text: | # c", "text: | "),
        ("  text: | # c", "  text: | "),
        ("    text: | # c", "    text: | "),
    ];
    for (line, expected) in inline_cases {
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected);
    }

    // Fact 4: Hash preceded by space triggers comment stripping at any indent
    let space_hash_cases = vec![
        ("text # c", "text "),
        ("  text # c", "  text "),
        ("    text # c", "    text "),
    ];
    for (line, expected) in space_hash_cases {
        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected);
    }
}

#[test]
fn test_literal_vs_folded_identical_indentation_handling() {
    // Document that | and > handle indentation identically for comments

    let test_indents = vec!["", "  ", "    ", "\t", "  \t"];

    for indent in test_indents {
        // Markers
        let literal = format!("{}text: |", indent);
        let folded = format!("{}text: >", indent);

        assert_eq!(classify_line_type(&literal), classify_line_type(&folded));
        assert_eq!(is_comment_line(&literal), is_comment_line(&folded));

        // Content starting with #
        let literal_content = format!("{}# content", indent);
        let folded_content = format!("{}# content", indent);

        assert_eq!(is_comment_line(&literal_content), is_comment_line(&folded_content));

        // Inline comments
        let literal_inline = format!("{}text: | # c", indent);
        let folded_inline = format!("{}text: > # c", indent);

        assert_eq!(
            strip_inline_comment(&literal_inline),
            strip_inline_comment(&folded_inline).replace('>', "|")
        );
    }
}

// ============================================================================
// Strip Modifier (>-) Indentation Validation
// ============================================================================

#[test]
fn test_strip_modifier_classification() {
    // Test that strip modifier (>-) markers are correctly classified
    let strip_markers = vec![
        "text: >-",
        "  text: >-",
        "    text: >-",
        "      text: >-",
        "description: >-",
        "  description: >-",
        "content: >-",
        "  content: >-",
    ];

    for marker in strip_markers {
        // Strip modifier markers should be classified as mapping keys
        assert_eq!(
            classify_line_type(marker),
            LineType::MappingKey,
            "Strip marker should be mapping key: {:?}",
            marker
        );
        assert!(
            !is_comment_line(marker),
            "Strip marker should not be comment: {:?}",
            marker
        );
    }
}

#[test]
fn test_strip_modifier_indent_level_1() {
    // Test strip modifier with content at level 1 (2-space indent)
    let test_cases = vec![
        // Strip marker at base level
        "text: >-",
        // Content at 2-space indent (level 1)
        "  Line 1 indented at level 1",
        "  Line 2 indented at level 1",
        // Comment at base level
        "# This is a comment at base level",
        // More content
        "  Line 3 indented at level 1",
    ];

    // Verify strip marker classification
    assert_eq!(classify_line_type(test_cases[0]), LineType::MappingKey);
    assert!(!is_comment_line(test_cases[0]));

    // Verify content lines are not comments
    for (i, line) in test_cases.iter().enumerate().skip(1) {
        if line.trim().starts_with('#') {
            // Lines starting with # are comments
            assert!(
                is_comment_line(line),
                "Comment line should be classified as comment at index {}: {:?}",
                i,
                line
            );
        } else {
            // Content lines are not comments
            assert!(
                !is_comment_line(line),
                "Content line should not be comment at index {}: {:?}",
                i,
                line
            );
        }
    }
}

#[test]
fn test_strip_modifier_indent_level_2() {
    // Test strip modifier with content at level 2 (4-space indent)
    let test_cases = vec![
        // Strip marker at base level
        "text: >-",
        // Content at 4-space indent (level 2)
        "    Line 1 double-indented at level 2",
        "    Line 2 double-indented at level 2",
        // Comment at base level
        "# This is a comment at base level",
        // More content at level 2
        "    Line 3 double-indented at level 2",
    ];

    // Verify strip marker classification
    assert_eq!(classify_line_type(test_cases[0]), LineType::MappingKey);
    assert!(!is_comment_line(test_cases[0]));

    // Verify content lines are not comments
    for (i, line) in test_cases.iter().enumerate().skip(1) {
        if line.trim().starts_with('#') {
            // Lines starting with # are comments
            assert!(
                is_comment_line(line),
                "Comment line should be classified as comment at index {}: {:?}",
                i,
                line
            );
        } else {
            // Content lines are not comments
            assert!(
                !is_comment_line(line),
                "Content line should not be comment at index {}: {:?}",
                i,
                line
            );
        }
    }
}

#[test]
fn test_strip_modifier_indent_level_3() {
    // Test strip modifier with content at level 3 (6-space indent)
    let test_cases = vec![
        // Strip marker at base level
        "text: >-",
        // Content at 6-space indent (level 3)
        "      Line 1 triple-indented at level 3",
        "      Line 2 triple-indented at level 3",
        // Comment at base level
        "# This is a comment at base level",
        // More content at level 3
        "      Line 3 triple-indented at level 3",
    ];

    // Verify strip marker classification
    assert_eq!(classify_line_type(test_cases[0]), LineType::MappingKey);
    assert!(!is_comment_line(test_cases[0]));

    // Verify content lines are not comments
    for (i, line) in test_cases.iter().enumerate().skip(1) {
        if line.trim().starts_with('#') {
            // Lines starting with # are comments
            assert!(
                is_comment_line(line),
                "Comment line should be classified as comment at index {}: {:?}",
                i,
                line
            );
        } else {
            // Content lines are not comments
            assert!(
                !is_comment_line(line),
                "Content line should not be comment at index {}: {:?}",
                i,
                line
            );
        }
    }
}

#[test]
fn test_strip_modifier_with_explicit_indent_levels() {
    // Test strip modifier with explicit indentation specifications
    // Format: >-N where N is the explicit indent level
    let explicit_indent_cases = vec![
        // Strip with explicit indent level 1
        ("text: >-1", " Line 1 at explicit level 1"),
        // Strip with explicit indent level 2
        ("text: >-2", "  Line 1 at explicit level 2"),
        // Strip with explicit indent level 3
        ("text: >-3", "   Line 1 at explicit level 3"),
    ];

    for (marker, content) in explicit_indent_cases {
        // Explicit strip marker should be classified as mapping key
        assert_eq!(
            classify_line_type(marker),
            LineType::MappingKey,
            "Explicit strip marker should be mapping key: {:?}",
            marker
        );
        assert!(
            !is_comment_line(marker),
            "Explicit strip marker should not be comment: {:?}",
            marker
        );

        // Content line should not be comment
        assert!(
            !is_comment_line(content),
            "Content line should not be comment: {:?}",
            content
        );
    }
}

#[test]
fn test_strip_modifier_preserves_indented_hash_content() {
    // Test that hash characters in strip-modified content are preserved
    // when they're part of the content (not preceded by space)
    let strip_content_cases = vec![
        // Level 1: 2-space indent
        "  URL:http://example.com#anchor",
        // Level 2: 4-space indent
        "    value#hash",
        // Level 3: 6-space indent
        "      config#key=value",
        // Hash not preceded by space should be preserved at all levels
    ];

    for line in strip_content_cases {
        // These content lines don't start with #, so they're not comment lines
        assert!(
            !is_comment_line(line),
            "Hash in content should not make it a comment: {:?}",
            line
        );

        // Hash should be preserved when stripping inline comments
        let stripped = strip_inline_comment(line);
        assert_eq!(
            stripped, line,
            "Hash not preceded by space should be preserved: {:?}",
            line
        );
    }
}

#[test]
fn test_strip_modifier_with_space_before_hash() {
    // Test that hash preceded by space triggers comment stripping
    // in strip-modified content at various indentation levels
    let strip_comment_cases = vec![
        // Level 1: 2-space indent
        ("  text # comment", "  text "),
        // Level 2: 4-space indent
        ("    value # comment", "    value "),
        // Level 3: 6-space indent
        ("      config # comment", "      config "),
    ];

    for (line, expected) in strip_comment_cases {
        // These are not comment lines (don't start with #)
        assert!(!is_comment_line(line));

        // Space before # triggers comment stripping
        let stripped = strip_inline_comment(line);
        assert_eq!(
            stripped, expected,
            "Space before hash should trigger comment stripping at level: {:?}",
            line
        );
    }
}

#[test]
fn test_strip_modifier_nested_indentation() {
    // Test strip modifier with nested indentation variations
    // Content with varying indentation levels within the same block
    let nested_cases = vec![
        // Base marker
        "text: >-",
        // Level 1: 2-space
        "  Base level content",
        // Level 2: 4-space (preserves line break in folded blocks)
        "    Double-indented content",
        // Level 3: 6-space
        "      Triple-indented content",
        // Back to level 1
        "  Back to base level",
    ];

    for line in nested_cases {
        // None of these are comment lines (don't start with #)
        assert!(
            !is_comment_line(line),
            "Content line should not be comment: {:?}",
            line
        );
    }
}

#[test]
fn test_strip_modifier_indented_marker_with_content() {
    // Test strip modifier when the marker itself is indented
    let test_cases = vec![
        // Marker at level 1 (2-space), content at level 1+1=level 2 (4-space)
        ("  text: >-", "    Line content"),
        // Marker at level 2 (4-space), content at level 2+1=level 3 (6-space)
        ("    text: >-", "      Line content"),
        // Marker at level 3 (6-space), content at level 3+1=level 4 (8-space)
        ("      text: >-", "        Line content"),
    ];

    for (marker, content) in test_cases {
        // Strip marker should be mapping key
        assert_eq!(
            classify_line_type(marker),
            LineType::MappingKey,
            "Strip marker should be mapping key: {:?}",
            marker
        );
        assert!(
            !is_comment_line(marker),
            "Strip marker should not be comment: {:?}",
            marker
        );

        // Content should not be comment
        assert!(
            !is_comment_line(content),
            "Content should not be comment: {:?}",
            content
        );
    }
}
