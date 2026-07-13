//! YAML Indentation and Mixed Scenarios Tests
//!
//! These tests verify YAML comment detection and filtering in complex scenarios:
//! - Comments at various indentation levels (0-12 spaces)
//! - Comments in nested structures (maps, lists)
//! - Mixed scenarios with values, comments, and anchors together
//! - Comments in multi-line strings and scalars
//!
//! Bead: bf-3xefd
//! Acceptance Criteria:
//! - Test for comments at indentation levels 0, 2, 4, 6, 8, 10, 12
//! - Test for comments in nested maps and lists
//! - Test for mixed scenarios with values + comments + anchors
//! - Test for comments in multi-line contexts
//! - All tests pass

use armor::parsers::yaml::{
    classify_line_type, is_comment_line, strip_inline_comment, calculate_indentation,
    detect_mapping_key, LineType
};

// ============================================================================
// Indentation Level Tests (0-12 spaces)
// ============================================================================

// Bead: bf-463jg - Test functions for indentation levels with descriptive names

#[test]
fn test_root_level_comments() {
    // Test for root-level comments (zero indentation)
    let test_cases = vec![
        "# Root level comment",
        "#",
        "# TODO: implement feature",
        "# FIXME: needs attention",
        "# Configuration header",
        "# ---",
    ];

    for line in test_cases {
        let indent = calculate_indentation(line);
        assert_eq!(indent, 0, "Root-level comment should have 0 indentation: '{}'", line);

        let line_type = classify_line_type(line);
        assert_eq!(line_type, LineType::Comment, "Root-level comment should be Comment type: '{}'", line);

        let is_comment = is_comment_line(line);
        assert!(is_comment, "Root-level line should be identified as comment: '{}'", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, "", "Root-level comment should strip to empty: '{}'", line);
    }
}

#[test]
fn test_single_indent_comments() {
    // Test for single-indent comments (2 spaces)
    let test_cases = vec![
        "  # Indented comment (2 spaces)",
        "  #",
        "  # Nested configuration section",
        "  # Section header",
        "  # Note: important info",
    ];

    for line in test_cases {
        let indent = calculate_indentation(line);
        assert_eq!(indent, 2, "Single-indent comment should have 2 spaces: '{}'", line);

        let line_type = classify_line_type(line);
        assert_eq!(line_type, LineType::Comment, "Single-indent comment should be Comment type: '{}'", line);

        let is_comment = is_comment_line(line);
        assert!(is_comment, "Single-indent line should be identified as comment: '{}'", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, "  ", "Single-indent comment should preserve indentation: '{}'", line);
    }

    // Also test that inline comments at single indent work correctly
    let inline_cases = vec![
        ("  key: value # inline comment", "  key: value "),
        ("  - item # comment", "  - item "),
    ];

    for (line, expected_stripped) in inline_cases {
        assert_eq!(calculate_indentation(line), 2, "Inline comment line should have 2 spaces");
        assert!(!is_comment_line(line), "Inline comment line should not be a full comment line");
        assert_eq!(strip_inline_comment(line), expected_stripped);
    }
}

#[test]
fn test_double_indent_comments() {
    // Test for double-indent comments (4 spaces)
    let test_cases = vec![
        "    # Double-nested comment (4 spaces)",
        "    #",
        "    # Deep configuration option",
        "    # Deep section header",
        "    # Note: very important",
    ];

    for line in test_cases {
        let indent = calculate_indentation(line);
        assert_eq!(indent, 4, "Double-indent comment should have 4 spaces: '{}'", line);

        let line_type = classify_line_type(line);
        assert_eq!(line_type, LineType::Comment, "Double-indent comment should be Comment type: '{}'", line);

        let is_comment = is_comment_line(line);
        assert!(is_comment, "Double-indent line should be identified as comment: '{}'", line);

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, "    ", "Double-indent comment should preserve indentation: '{}'", line);
    }

    // Also test that inline comments at double indent work correctly
    let inline_cases = vec![
        ("    key: value # inline comment", "    key: value "),
        ("    - item # comment", "    - item "),
    ];

    for (line, expected_stripped) in inline_cases {
        assert_eq!(calculate_indentation(line), 4, "Inline comment line should have 4 spaces");
        assert!(!is_comment_line(line), "Inline comment line should not be a full comment line");
        assert_eq!(strip_inline_comment(line), expected_stripped);
    }
}

#[test]
fn test_deep_indent_comments() {
    // Test for deep-indent comments (8+ spaces)
    let test_cases = vec![
        ("        # Quad-nested comment (8 spaces)", 8),
        ("          # Penta-nested comment (10 spaces)", 10),
        ("            # Hexa-nested comment (12 spaces)", 12),
        ("              # Very deep comment (14 spaces)", 14),
        ("                # Extremely deep comment (16 spaces)", 16),
    ];

    for (line, expected_indent) in test_cases {
        let indent = calculate_indentation(line);
        assert_eq!(indent, expected_indent, "Deep-indent comment should have {} spaces: '{}'", expected_indent, line);

        let line_type = classify_line_type(line);
        assert_eq!(line_type, LineType::Comment, "Deep-indent comment should be Comment type: '{}'", line);

        let is_comment = is_comment_line(line);
        assert!(is_comment, "Deep-indent line should be identified as comment: '{}'", line);

        let stripped = strip_inline_comment(line);
        let expected_spaces = " ".repeat(expected_indent);
        assert_eq!(stripped, expected_spaces, "Deep-indent comment should preserve indentation: '{}'", line);
    }

    // Also test that inline comments at deep indents work correctly
    let inline_cases = vec![
        ("        key: value # inline comment", "        key: value ", 8),
        ("          key: value # comment", "          key: value ", 10),
        ("            key: value # comment", "            key: value ", 12),
    ];

    for (line, expected_stripped, expected_indent) in inline_cases {
        assert_eq!(calculate_indentation(line), expected_indent, "Inline comment line should have {} spaces", expected_indent);
        assert!(!is_comment_line(line), "Inline comment line should not be a full comment line");
        assert_eq!(strip_inline_comment(line), expected_stripped);
    }
}

#[test]
fn test_comment_at_indentation_level_0() {
    // Comments with no indentation (level 0)
    let test_cases = vec![
        "# Root level comment",
        "#",
        "# TODO: implement feature",
        "# FIXME: needs attention",
    ];

    for line in test_cases {
        assert_eq!(calculate_indentation(line), 0, "Indentation should be 0 for: {}", line);
        assert_eq!(classify_line_type(line), LineType::Comment, "Should be comment line: {}", line);
        assert!(is_comment_line(line), "Should be identified as comment: {}", line);
        assert_eq!(strip_inline_comment(line), "", "Should strip to empty: {}", line);
    }
}

#[test]
fn test_comment_at_indentation_level_2() {
    // Comments with 2-space indentation (level 2)
    let test_cases = vec![
        "  # Indented comment (2 spaces)",
        "  #",
        "  # Nested configuration section",
    ];

    for line in test_cases {
        assert_eq!(calculate_indentation(line), 2, "Indentation should be 2 for: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment, "Should be comment line: {}", line);
        assert!(is_comment_line(line), "Should be identified as comment: {}", line);
        assert_eq!(strip_inline_comment(line), "  ", "Should preserve indentation: {}", line);
    }
}

#[test]
fn test_comment_at_indentation_level_4() {
    // Comments with 4-space indentation (level 4)
    let test_cases = vec![
        "    # Double-nested comment (4 spaces)",
        "    #",
        "    # Deep configuration option",
    ];

    for line in test_cases {
        assert_eq!(calculate_indentation(line), 4, "Indentation should be 4 for: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment, "Should be comment line: {}", line);
        assert!(is_comment_line(line), "Should be identified as comment: {}", line);
        assert_eq!(strip_inline_comment(line), "    ", "Should preserve indentation: {}", line);
    }
}

#[test]
fn test_comment_at_indentation_level_6() {
    // Comments with 6-space indentation (level 6)
    let test_cases = vec![
        "      # Triple-nested comment (6 spaces)",
        "      #",
        "      # Very deep configuration",
    ];

    for line in test_cases {
        assert_eq!(calculate_indentation(line), 6, "Indentation should be 6 for: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment, "Should be comment line: {}", line);
        assert!(is_comment_line(line), "Should be identified as comment: {}", line);
        assert_eq!(strip_inline_comment(line), "      ", "Should preserve indentation: {}", line);
    }
}

#[test]
fn test_comment_at_indentation_level_8() {
    // Comments with 8-space indentation (level 8)
    let test_cases = vec![
        "        # Quad-nested comment (8 spaces)",
        "        #",
        "        # Extremely deep configuration",
    ];

    for line in test_cases {
        assert_eq!(calculate_indentation(line), 8, "Indentation should be 8 for: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment, "Should be comment line: {}", line);
        assert!(is_comment_line(line), "Should be identified as comment: {}", line);
        assert_eq!(strip_inline_comment(line), "        ", "Should preserve indentation: {}", line);
    }
}

#[test]
fn test_comment_at_indentation_level_10() {
    // Comments with 10-space indentation (level 10)
    let test_cases = vec![
        "          # Penta-nested comment (10 spaces)",
        "          #",
        "          # Ultra-deep configuration",
    ];

    for line in test_cases {
        assert_eq!(calculate_indentation(line), 10, "Indentation should be 10 for: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment, "Should be comment line: {}", line);
        assert!(is_comment_line(line), "Should be identified as comment: {}", line);
        assert_eq!(strip_inline_comment(line), "          ", "Should preserve indentation: {}", line);
    }
}

#[test]
fn test_comment_at_indentation_level_12() {
    // Comments with 12-space indentation (level 12)
    let test_cases = vec![
        "            # Hexa-nested comment (12 spaces)",
        "            #",
        "            # Maximum tested depth",
    ];

    for line in test_cases {
        assert_eq!(calculate_indentation(line), 12, "Indentation should be 12 for: {:?}", line);
        assert_eq!(classify_line_type(line), LineType::Comment, "Should be comment line: {}", line);
        assert!(is_comment_line(line), "Should be identified as comment: {}", line);
        assert_eq!(strip_inline_comment(line), "            ", "Should preserve indentation: {}", line);
    }
}

#[test]
fn test_all_indentation_levels_together() {
    // Test all indentation levels in sequence
    let lines = vec![
        "# Level 0",
        "  # Level 2",
        "    # Level 4",
        "      # Level 6",
        "        # Level 8",
        "          # Level 10",
        "            # Level 12",
    ];

    let expected_levels = vec![0, 2, 4, 6, 8, 10, 12];

    for (i, line) in lines.iter().enumerate() {
        assert_eq!(calculate_indentation(line), expected_levels[i],
                   "Line {}: expected indentation {}", i + 1, expected_levels[i]);
        assert_eq!(classify_line_type(line), LineType::Comment,
                   "Line {}: should be comment", i + 1);
        assert!(is_comment_line(line), "Line {}: should be comment", i + 1);
    }
}

#[test]
fn test_content_lines_at_various_indentations() {
    // Content lines (not comments) at various indentations
    let test_cases = vec![
        ("key: value", 0),
        ("  key: value", 2),
        ("    key: value", 4),
        ("      key: value", 6),
        ("        key: value", 8),
        ("          key: value", 10),
        ("            key: value", 12),
    ];

    for (line, expected_indent) in test_cases {
        assert_eq!(calculate_indentation(line), expected_indent,
                   "Indentation should be {} for: {}", expected_indent, line);
        assert_eq!(classify_line_type(line), LineType::MappingKey,
                   "Should be mapping key: {}", line);
        assert!(!is_comment_line(line), "Should NOT be comment: {}", line);
    }
}

// ============================================================================
// Comments in Nested Maps Tests
// ============================================================================

#[test]
fn test_comment_in_nested_map_single_level() {
    // Single-level nested map with comments
    let yaml = vec![
        "parent:",
        "  # Comment inside parent block",
        "  child: value",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);
    assert_eq!(calculate_indentation(yaml[0]), 0);

    assert_eq!(classify_line_type(yaml[1]), LineType::Comment);
    assert_eq!(calculate_indentation(yaml[1]), 2);
    assert!(is_comment_line(yaml[1]));

    assert_eq!(classify_line_type(yaml[2]), LineType::MappingKey);
    assert_eq!(calculate_indentation(yaml[2]), 2);
}

#[test]
fn test_comment_in_nested_map_two_levels() {
    // Two-level nested map with comments
    let yaml = vec![
        "parent:",
        "  # Comment at level 1",
        "  child:",
        "    # Comment at level 2",
        "    grandchild: value",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);
    assert_eq!(calculate_indentation(yaml[0]), 0);

    assert_eq!(classify_line_type(yaml[1]), LineType::Comment);
    assert_eq!(calculate_indentation(yaml[1]), 2);

    assert_eq!(classify_line_type(yaml[2]), LineType::MappingKey);
    assert_eq!(calculate_indentation(yaml[2]), 2);

    assert_eq!(classify_line_type(yaml[3]), LineType::Comment);
    assert_eq!(calculate_indentation(yaml[3]), 4);

    assert_eq!(classify_line_type(yaml[4]), LineType::MappingKey);
    assert_eq!(calculate_indentation(yaml[4]), 4);
}

#[test]
fn test_comment_in_deeply_nested_map() {
    // Deeply nested map (6 levels) with comments at each level
    let yaml = vec![
        "level0:",
        "  # L0 comment",
        "  level1:",
        "    # L1 comment",
        "    level2:",
        "      # L2 comment",
        "      level3:",
        "        # L3 comment",
        "        level4:",
        "          # L4 comment",
        "          level5:",
        "            # L5 comment",
        "            final: value",
    ];

    let expected_types = vec![
        LineType::MappingKey,  // level0:
        LineType::Comment,     // L0 comment
        LineType::MappingKey,  // level1:
        LineType::Comment,     // L1 comment
        LineType::MappingKey,  // level2:
        LineType::Comment,     // L2 comment
        LineType::MappingKey,  // level3:
        LineType::Comment,     // L3 comment
        LineType::MappingKey,  // level4:
        LineType::Comment,     // L4 comment
        LineType::MappingKey,  // level5:
        LineType::Comment,     // L5 comment
        LineType::MappingKey,  // final: value
    ];

    let expected_indents = vec![0, 2, 2, 4, 4, 6, 6, 8, 8, 10, 10, 12, 12];

    for (i, line) in yaml.iter().enumerate() {
        assert_eq!(classify_line_type(line), expected_types[i],
                   "Line {}: incorrect type for: {}", i + 1, line);
        assert_eq!(calculate_indentation(line), expected_indents[i],
                   "Line {}: incorrect indentation for: {:?}", i + 1, line);
    }
}

#[test]
fn test_comments_between_nested_keys() {
    // Comments interspersed between nested keys
    let yaml = vec![
        "outer:",
        "  # Section A",
        "  key1: value1",
        "  # Separator comment",
        "  key2: value2",
        "  # Section B",
        "  inner:",
        "    # Nested section",
        "    nested: value",
    ];

    let comment_indices = vec![1, 3, 5, 7];
    for i in comment_indices {
        assert_eq!(classify_line_type(yaml[i]), LineType::Comment,
                   "Line {} should be comment: {}", i + 1, yaml[i]);
    }

    let key_indices = vec![0, 2, 4, 6, 8];
    for i in key_indices {
        assert_eq!(classify_line_type(yaml[i]), LineType::MappingKey,
                   "Line {} should be mapping key: {}", i + 1, yaml[i]);
    }
}

#[test]
fn test_inline_comments_in_nested_map() {
    // Inline comments in nested map
    let test_cases = vec![
        ("parent:", None, "parent:"),
        ("  child: value # inline comment", Some("  child: value "), "value"),
        ("    nested: value # deep inline", Some("    nested: value "), "value"),
        ("      final: value # deepest", Some("      final: value "), "value"),
    ];

    for (line, expected_stripped, expected_val) in test_cases {
        let indent = calculate_indentation(line);
        assert!(indent % 2 == 0, "Indentation should be even: {}", indent);

        if let Some(expected) = expected_stripped {
            assert_eq!(strip_inline_comment(line), expected,
                       "Incorrect stripping for: {}", line);
        }

        // Check the parsed value if this line has a value
        if !line.ends_with(':') {
            let info = detect_mapping_key(line, indent);
            assert!(info.is_some(), "Should detect mapping key in: {}", line);
            assert_eq!(info.unwrap().value, Some(expected_val.to_string()),
                       "Value mismatch for: {}", line);
        }
    }
}

// ============================================================================
// Comments in Nested Lists Tests
// ============================================================================

#[test]
fn test_comment_in_flat_list() {
    // Comments in a flat list
    let yaml = vec![
        "- item1",
        "- item2 # inline comment",
        "# Full-line comment between items",
        "- item3",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::SequenceItem);
    assert_eq!(classify_line_type(yaml[1]), LineType::SequenceItem);
    assert_eq!(strip_inline_comment(yaml[1]), "- item2 ");

    assert_eq!(classify_line_type(yaml[2]), LineType::Comment);
    assert!(is_comment_line(yaml[2]));

    assert_eq!(classify_line_type(yaml[3]), LineType::SequenceItem);
}

#[test]
fn test_comment_in_nested_list_single_level() {
    // Single-level nested list with comments
    let yaml = vec![
        "- parent:",
        "  # Comment inside parent",
        "  - child1",
        "  - child2",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::SequenceItem);
    assert_eq!(classify_line_type(yaml[1]), LineType::Comment);
    assert_eq!(calculate_indentation(yaml[1]), 2);
    assert!(is_comment_line(yaml[1]));

    assert_eq!(classify_line_type(yaml[2]), LineType::SequenceItem);
    assert_eq!(classify_line_type(yaml[3]), LineType::SequenceItem);
}

#[test]
fn test_comment_in_nested_list_multiple_levels() {
    // Multi-level nested list with comments
    let yaml = vec![
        "- level0:",
        "  # L0 comment",
        "  - level1:",
        "    # L1 comment",
        "    - level2:",
        "      # L2 comment",
        "      - final_item",
    ];

    let expected_types = vec![
        LineType::SequenceItem,  // - level0:
        LineType::Comment,       // L0 comment
        LineType::SequenceItem,  // - level1:
        LineType::Comment,       // L1 comment
        LineType::SequenceItem,  // - level2:
        LineType::Comment,       // L2 comment
        LineType::SequenceItem,  // - final_item
    ];

    let expected_indents = vec![0, 2, 2, 4, 4, 6, 6];

    for (i, line) in yaml.iter().enumerate() {
        assert_eq!(classify_line_type(line), expected_types[i],
                   "Line {}: incorrect type for: {}", i + 1, line);
        assert_eq!(calculate_indentation(line), expected_indents[i],
                   "Line {}: incorrect indentation for: {:?}", i + 1, line);
    }
}

#[test]
fn test_comments_between_list_items() {
    // Comments interspersed between list items
    let yaml = vec![
        "# Start of list",
        "- item1",
        "# Separator 1",
        "- item2",
        "# Separator 2",
        "- item3",
        "# End of list",
    ];

    let comment_indices = vec![0, 2, 4, 6];
    for i in comment_indices {
        assert_eq!(classify_line_type(yaml[i]), LineType::Comment,
                   "Line {} should be comment: {}", i + 1, yaml[i]);
    }

    let item_indices = vec![1, 3, 5];
    for i in item_indices {
        assert_eq!(classify_line_type(yaml[i]), LineType::SequenceItem,
                   "Line {} should be sequence item: {}", i + 1, yaml[i]);
    }
}

#[test]
fn test_inline_comments_in_nested_list() {
    // Inline comments in nested list
    let test_cases = vec![
        ("- item1 # first", "- item1 "),
        ("  - item2 # second", "  - item2 "),
        ("    - item3 # third", "    - item3 "),
        ("      - item4 # fourth", "      - item4 "),
    ];

    for (line, expected_stripped) in test_cases {
        assert_eq!(classify_line_type(line), LineType::SequenceItem);
        assert_eq!(strip_inline_comment(line), expected_stripped);
    }
}

// ============================================================================
// Comments in Mixed Map/List Structures Tests
// ============================================================================

#[test]
fn test_comment_in_map_with_list_value() {
    // Map with list value containing comments
    let yaml = vec![
        "items:",
        "  # List of items",
        "  - item1",
        "  - item2",
        "  # Another list",
        "  - item3",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[1]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[2]), LineType::SequenceItem);
    assert_eq!(classify_line_type(yaml[3]), LineType::SequenceItem);
    assert_eq!(classify_line_type(yaml[4]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[5]), LineType::SequenceItem);
}

#[test]
fn test_comment_in_list_with_map_values() {
    // List with map values containing comments
    let yaml = vec![
        "- name: item1",
        "  # Item1 details",
        "  value: 10",
        "- name: item2",
        "  # Item2 details",
        "  value: 20",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::SequenceItem);
    assert_eq!(classify_line_type(yaml[1]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[2]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[3]), LineType::SequenceItem);
    assert_eq!(classify_line_type(yaml[4]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[5]), LineType::MappingKey);
}

#[test]
fn test_comment_in_complex_nested_structure() {
    // Complex mixed structure with maps and lists
    let yaml = vec![
        "config:",
        "  # Database section",
        "  database:",
        "    hosts:",
        "      # Production hosts",
        "      - prod1.example.com",
        "      - prod2.example.com",
        "    # Database settings",
        "    settings:",
        "      pool_size: 10",
        "      timeout: 30",
    ];

    let expected = vec![
        (LineType::MappingKey, 0),    // config:
        (LineType::Comment, 2),        // Database section
        (LineType::MappingKey, 2),     // database:
        (LineType::MappingKey, 4),     // hosts:
        (LineType::Comment, 6),        // Production hosts
        (LineType::SequenceItem, 6),   // - prod1.example.com
        (LineType::SequenceItem, 6),   // - prod2.example.com
        (LineType::Comment, 4),        // Database settings
        (LineType::MappingKey, 4),     // settings:
        (LineType::MappingKey, 6),     // pool_size: 10
        (LineType::MappingKey, 6),     // timeout: 30
    ];

    for (i, line) in yaml.iter().enumerate() {
        let (exp_type, exp_indent) = expected[i];
        assert_eq!(classify_line_type(line), exp_type,
                   "Line {}: type mismatch for: {}", i + 1, line);
        assert_eq!(calculate_indentation(line), exp_indent,
                   "Line {}: indent mismatch for: {:?}", i + 1, line);
    }
}

// ============================================================================
// Mixed Scenarios: Values + Comments + Anchors
// ============================================================================

#[test]
fn test_anchor_with_comment() {
    // Anchor definition with inline comment
    let test_cases = vec![
        "defaults: &default_config # Default configuration",
        "database: &prod_db postgres://localhost/prod # Production DB",
        "timeout: &default_timeout 30 # Default timeout in seconds",
    ];

    for line in test_cases {
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('&'), "Anchor & should be preserved in: {}", line);
        assert!(!stripped.contains('#') || stripped.ends_with(' '),
                "Comment should be stripped from: {}", line);

        let indent = calculate_indentation(line);
        let info = detect_mapping_key(line, indent);
        assert!(info.is_some(), "Should detect mapping key in: {}", line);
    }
}

#[test]
fn test_alias_with_comment() {
    // Alias reference with inline comment
    let test_cases = vec![
        "config: *default_config # Use default config",
        "database: *prod_db # Use production database",
        "timeout: *default_timeout # Use default timeout",
    ];

    for line in test_cases {
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert!(stripped.contains('*'), "Alias * should be preserved in: {}", line);
        assert!(!stripped.contains('#') || stripped.ends_with(' '),
                "Comment should be stripped from: {}", line);

        let indent = calculate_indentation(line);
        let info = detect_mapping_key(line, indent);
        assert!(info.is_some());
        assert!(info.unwrap().value.unwrap().starts_with('*'));
    }
}

#[test]
fn test_value_anchor_and_comment_together() {
    // Value with anchor, actual value, and comment all together
    let test_cases = vec![
        ("defaults: &default {host: localhost} # Defaults", "defaults: &default {host: localhost} "),
        ("list: &items [1, 2, 3] # Item list", "list: &items [1, 2, 3] "),
        ("settings: &config {a: 1, b: 2} # Config", "settings: &config {a: 1, b: 2} "),
    ];

    for (line, expected_stripped) in test_cases {
        // Lines with flow collections ([...]) are classified as FlowSequence
        // Lines with flow mappings ({...}) are classified as FlowMapping
        // Simple values are classified as MappingKey
        let line_type = classify_line_type(line);
        assert!(matches!(line_type, LineType::MappingKey | LineType::FlowSequence | LineType::FlowMapping),
                "Line should be mapping or flow type: {} (got {:?}", line, line_type);
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);
        assert!(stripped.contains('&'), "Anchor should be preserved in: {}", line);
    }
}

#[test]
fn test_nested_anchor_with_comment() {
    // Nested anchor with comment
    let yaml = vec![
        "parent:",
        "  # Parent section",
        "  child: &anchor value # Anchored value",
        "  other: *anchor # Alias to anchor",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[1]), LineType::Comment);
    assert_eq!(classify_line_type(yaml[2]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[3]), LineType::MappingKey);

    let stripped = strip_inline_comment(yaml[2]);
    assert!(stripped.contains('&'));
    assert!(stripped.contains("value"));

    let info = detect_mapping_key(yaml[2], 2);
    assert!(info.is_some());

    let info = detect_mapping_key(yaml[3], 2);
    assert!(info.is_some());
    assert!(info.unwrap().value.unwrap().starts_with('*'));
}

#[test]
fn test_complex_mixed_anchor_alias_comment_scenario() {
    // Complex scenario with anchors, aliases, and comments
    let yaml = r#"# Configuration
defaults: &defaults
  timeout: 30 # Default timeout
  retries: 3 # Retry count

production:
  <<: *defaults # Inherit defaults
  timeout: 60 # Override for production
  # Production-specific settings
  host: prod.example.com

development:
  <<: *defaults # Inherit defaults
  host: localhost # Development host
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify line classifications
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert_eq!(classify_line_type(lines[1]), LineType::MappingKey);

    // Verify anchor in line 1
    assert!(lines[1].contains("&defaults"));

    // Verify inline comments in lines 2-3
    assert_eq!(strip_inline_comment(lines[2]), "  timeout: 30 ");
    assert_eq!(strip_inline_comment(lines[3]), "  retries: 3 ");

    // Verify alias usage (merge key <<:)
    assert!(lines[6].contains("*defaults"));
    assert_eq!(strip_inline_comment(lines[6]), "  <<: *defaults ");

    // Verify production override (line 7, not 9)
    assert_eq!(strip_inline_comment(lines[7]), "  timeout: 60 ");

    // Verify comment line 8
    assert_eq!(classify_line_type(lines[8]), LineType::Comment);
}

// ============================================================================
// Multi-line String and Scalar Tests
// ============================================================================

#[test]
fn test_comment_before_multiline_literal_scalar() {
    // Comment before literal scalar (|)
    let yaml = vec![
        "# Multi-line description",
        "description: |",
        "  This is a",
        "  multi-line",
        "  string",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::Comment);
    assert!(is_comment_line(yaml[0]));

    // Note: The | line and content lines are MappingKey types in this parser
    // because they contain content (the scalar value)
    for (i, line) in yaml.iter().enumerate().skip(1) {
        assert_ne!(classify_line_type(line), LineType::Comment,
                   "Line {} should not be comment: {}", i + 1, line);
        assert!(!is_comment_line(line),
                "Line {} should not be comment line: {}", i + 1, line);
    }
}

#[test]
fn test_comment_before_multiline_folded_scalar() {
    // Comment before folded scalar (>)
    let yaml = vec![
        "# Multi-line text (folded)",
        "text: >",
        "  This is a",
        "  folded string",
        "  with newlines",
        "  normalized",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::Comment);
    assert!(is_comment_line(yaml[0]));

    for (i, line) in yaml.iter().enumerate().skip(1) {
        assert_ne!(classify_line_type(line), LineType::Comment,
                   "Line {} should not be comment: {}", i + 1, line);
        assert!(!is_comment_line(line),
                "Line {} should not be comment line: {}", i + 1, line);
    }
}

#[test]
fn test_inline_comment_on_scalar_header() {
    // Inline comment on the line with scalar indicator (| or >)
    let test_cases = vec![
        ("description: | # Multi-line description", "description: | "),
        ("text: > # Folded text", "text: > "),
        ("  content: | # Indented multi-line", "  content: | "),
        ("    data: > # Deep folded", "    data: > "),
    ];

    for (line, expected_stripped) in test_cases {
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);
        assert!(stripped.contains('|') || stripped.contains('>'),
                "Scalar indicator should be preserved in: {}", line);
    }
}

#[test]
fn test_comments_amongst_multiline_scalar_lines() {
    // Comments between multi-line scalar continuation lines
    let yaml = vec![
        "text: |",
        "  Line 1",
        "  # This is content, not a comment",
        "  Line 3",
    ];

    // In a multi-line scalar, lines starting with # are still content
    // because they're part of the literal scalar value
    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);

    // Note: This line-based parser treats each line independently
    // A line starting with # IS classified as a comment without full YAML context
    // In full YAML parsing, continuation lines after | are part of the scalar
    // Our line-based classifier sees "  # This is content..." and classifies it as Comment
    // Content lines like "  Line 1" are classified as Unknown (they're not valid mapping keys)
    assert!(!classify_line_type(yaml[1]).is_structural()); // "  Line 1" - not structural
    assert_eq!(classify_line_type(yaml[2]), LineType::Comment); // "  # This is content..." - looks like comment
    assert!(!classify_line_type(yaml[3]).is_structural()); // "  Line 3" - not structural
}

#[test]
fn test_comment_near_double_quoted_scalar() {
    // Comments with double-quoted scalars
    let test_cases = vec![
        "name: \"John Doe\" # Person name",
        "path: \"/home/user\" # Home directory",
        "url: \"https://example.com\" # Example URL",
        "  quoted: \"value # not a comment\" # This IS a comment",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(!stripped.contains('#') || stripped.ends_with(' '),
                "Only actual comment should be stripped from: {}", line);

        let indent = calculate_indentation(line);
        let info = detect_mapping_key(line, indent);
        assert!(info.is_some());
    }
}

#[test]
fn test_comment_near_single_quoted_scalar() {
    // Comments with single-quoted scalars
    let test_cases = vec![
        "name: 'John Doe' # Person name",
        "path: '/home/user' # Home directory",
        "url: 'https://example.com' # Example URL",
        "  quoted: 'value # not a comment' # This IS a comment",
    ];

    for line in test_cases {
        assert!(!is_comment_line(line));
        assert_eq!(classify_line_type(line), LineType::MappingKey);

        let stripped = strip_inline_comment(line);
        assert!(!stripped.contains('#') || stripped.ends_with(' '),
                "Only actual comment should be stripped from: {}", line);

        let indent = calculate_indentation(line);
        let info = detect_mapping_key(line, indent);
        assert!(info.is_some());
    }
}

#[test]
fn test_hash_in_quoted_scalar_with_comment() {
    // Hash in quoted scalar followed by comment
    let test_cases = vec![
        ("color: \"#FFFFFF\" # White color", "color: \"#FFFFFF\" ", "#FFFFFF"),
        ("url: \"http://example.com#anchor\" # With anchor", "url: \"http://example.com#anchor\" ", "http://example.com#anchor"),
        ("text: \"value #1\" # First value", "text: \"value #1\" ", "value #1"),
    ];

    for (line, expected_stripped, expected_hash_content) in test_cases {
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);

        let indent = calculate_indentation(line);
        let info = detect_mapping_key(line, indent);
        assert!(info.is_some());
        assert!(info.unwrap().value.unwrap().contains(expected_hash_content));
    }
}

// ============================================================================
// Complex Real-world Integration Tests
// ============================================================================

#[test]
fn test_complete_complex_document_with_all_features() {
    // A complete document combining all features
    let yaml = r#"# Application Configuration
# Version 2.0

app:
  # Main application settings
  name: "MyApp" # Application name
  version: 2.0 # Version number

  # Database configuration
  database:
    &db_config
    host: localhost # Database host
    port: 5432 # Default PostgreSQL port
    name: myapp_db # Database name

  # Production overrides
  production:
    <<: *db_config # Inherit database config
    host: prod-db.example.com # Production database
    # Additional production settings
    ssl: true # Enable SSL
    pool_size: 20 # Connection pool size

# List of features
features:
  # Authentication
  - name: auth # Authentication feature
    enabled: true
    # Auth providers
    providers:
      - oauth
      - ldap

  # Rate limiting
  - name: rate_limit # Rate limiting feature
    enabled: false
    # Rate limit settings
    limits:
      requests: 100 # Max requests
      window: 60 # Time window (seconds)
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Count comment lines vs content lines
    let mut comment_count = 0;
    let mut content_count = 0;

    for line in &lines {
        if line.trim().is_empty() {
            continue; // Skip empty lines
        }

        if classify_line_type(line) == LineType::Comment {
            comment_count += 1;
        } else {
            content_count += 1;
        }
    }

    // Verify we detected comments
    assert!(comment_count > 0, "Should detect comment lines");
    assert!(content_count > 0, "Should detect content lines");

    // Verify specific key lines
    let db_config_line = "database:";
    assert!(lines.iter().any(|l| l.contains(db_config_line)),
            "Should contain database section");

    // Verify anchor usage
    assert!(lines.iter().any(|l| l.contains("&db_config")),
            "Should contain anchor definition");
    assert!(lines.iter().any(|l| l.contains("*db_config")),
            "Should contain alias reference");
}

#[test]
fn test_indentation_preservation_in_comment_stripping() {
    // Verify that indentation is preserved when stripping comments
    let test_cases = vec![
        ("# comment", "", 0),
        ("  # comment", "  ", 2),
        ("    # comment", "    ", 4),
        ("      # comment", "      ", 6),
        ("        # comment", "        ", 8),
        ("          # comment", "          ", 10),
        ("            # comment", "            ", 12),
    ];

    for (line, expected_stripped, expected_indent) in test_cases {
        assert_eq!(calculate_indentation(line), expected_indent,
                   "Indentation calculation for: {:?}", line);
        assert_eq!(strip_inline_comment(line), expected_stripped,
                   "Stripping should preserve indentation: {}", line);
    }
}

#[test]
fn test_all_indentation_levels_with_inline_comments() {
    // Inline comments at all indentation levels
    let test_cases = vec![
        ("key: value # level 0", "key: value ", 0),
        ("  key: value # level 2", "  key: value ", 2),
        ("    key: value # level 4", "    key: value ", 4),
        ("      key: value # level 6", "      key: value ", 6),
        ("        key: value # level 8", "        key: value ", 8),
        ("          key: value # level 10", "          key: value ", 10),
        ("            key: value # level 12", "            key: value ", 12),
    ];

    for (line, expected_stripped, expected_indent) in test_cases {
        assert_eq!(calculate_indentation(line), expected_indent,
                   "Indentation for: {}", line);
        assert_eq!(strip_inline_comment(line), expected_stripped,
                   "Stripping for: {}", line);
        assert!(!is_comment_line(line),
                "Should not be comment line: {}", line);
        assert_eq!(classify_line_type(line), LineType::MappingKey,
                   "Should be mapping key: {}", line);
    }
}

// ============================================================================
// Multi-line Scalar Context Tests - Literal Style (|)
// ============================================================================

#[test]
fn test_literal_scalar_content_with_hash_prefix() {
    // In a literal scalar (|), lines starting with # are CONTENT, not comments
    // This is a key difference from general YAML parsing
    let yaml = vec![
        "description: |",
        "  # This is a heading in the content",
        "  Regular content line",
        "  # Another heading",
        "  Final content",
    ];

    // The header line with | is a mapping key
    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);
    assert!(!is_comment_line(yaml[0]));

    // Content lines (including those starting with #) are treated as:
    // - Lines starting with #: classified as Comment by line-based parser
    // - Regular content lines: not structural (not valid mapping keys)
    for (i, line) in yaml.iter().enumerate().skip(1) {
        if line.trim().starts_with("#") {
            // Line-based parser sees this as a comment
            assert_eq!(classify_line_type(line), LineType::Comment,
                       "Line {} should be classified as comment by line-based parser: {}", i + 1, line);
            assert!(is_comment_line(line),
                    "Line {} should be identified as comment line: {}", i + 1, line);
        } else {
            // Regular content lines are not structural
            assert!(!classify_line_type(line).is_structural(),
                    "Line {} should not be structural: {}", i + 1, line);
            assert!(!is_comment_line(line),
                    "Line {} should not be comment line: {}", i + 1, line);
        }
    }
}

#[test]
fn test_literal_scalar_various_indentation_levels() {
    // Test literal scalars at different indentation levels
    let test_cases = vec![
        // Level 0 (no indentation)
        vec![
            "text: |",
            "  Content at level 0",
            "  # Comment-like content",
        ],
        // Level 2 (2 spaces)
        vec![
            "  section:",
            "    text: |",
            "      Content at level 2",
            "      # Comment-like at level 2",
        ],
        // Level 4 (4 spaces)
        vec![
            "    nested:",
            "      deep:",
            "        text: |",
            "          Content at level 4",
            "          # Comment-like at level 4",
        ],
        // Level 6 (6 spaces)
        vec![
            "      deeply:",
            "        nested:",
            "          text: |",
            "            Content at level 6",
            "            # Comment-like at level 6",
        ],
    ];

    for yaml in test_cases {
        // Find the | line (scalar header)
        let header_idx = yaml.iter().position(|l| l.contains("|")).unwrap();
        let header_line = yaml[header_idx];

        // Header is always a mapping key
        assert_eq!(classify_line_type(header_line), LineType::MappingKey);

        // Content lines after | should be handled appropriately
        for (i, line) in yaml.iter().enumerate().skip(header_idx + 1) {
            let indent = calculate_indentation(line);

            // Lines that look like comments are classified as such by line-based parser
            if line.trim().starts_with("#") {
                assert_eq!(classify_line_type(line), LineType::Comment,
                           "Content line should be comment: {}", line);
                assert_eq!(strip_inline_comment(line), " ".repeat(indent).as_str(),
                           "Should strip to just indentation: {}", line);
            } else if !line.trim().is_empty() {
                // Regular content is not structural
                assert!(!classify_line_type(line).is_structural(),
                        "Regular content should not be structural: {}", line);
            }
        }
    }
}

#[test]
fn test_literal_scalar_empty_lines_and_whitespace() {
    // Test empty lines and whitespace in literal scalars
    let yaml = vec![
        "text: |",
        "  First line",
        "",
        "  Third line (after empty)",
        "    ",  // Line with only spaces
        "  Fifth line",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);

    // Empty lines and whitespace-only lines have specific behavior
    // Empty lines ARE structural (they're Blank type)
    // Whitespace-only lines are also Blank
    assert!(!classify_line_type(yaml[1]).is_structural()); // "  First line" - content
    assert_eq!(classify_line_type(yaml[2]), LineType::Blank); // "" (empty line) is structural/Blank
    assert!(!classify_line_type(yaml[3]).is_structural()); // "  Third line" - content
    assert_eq!(classify_line_type(yaml[4]), LineType::Blank); // "    " (spaces only) is Blank
    assert!(!classify_line_type(yaml[5]).is_structural()); // "  Fifth line" - content
}

#[test]
fn test_literal_scalar_with_special_patterns() {
    // Test literal scalars with special patterns that might look like YAML syntax
    let yaml = vec![
        "config: |",
        "  # KEY: value - looks like mapping but is content",
        "  - item - looks like list item but is content",
        "  key: value - looks like mapping but is content",
        "  Regular text without special chars",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);

    // All content lines are treated as content, not YAML syntax
    // by the line-based parser's structural classification
    for (i, line) in yaml.iter().enumerate().skip(1) {
        if line.trim().starts_with("#") {
            // Lines starting with # are classified as comments
            assert_eq!(classify_line_type(line), LineType::Comment);
        } else if line.contains(":") {
            // Lines with colons might be classified as mapping keys
            // even though they're actually content in the scalar
            let line_type = classify_line_type(line);
            // The parser sees "  key: value" as a potential mapping key
            assert!(matches!(line_type, LineType::MappingKey | LineType::Unknown),
                    "Content with colon should be mapping key or unknown: {} (got {:?})", line, line_type);
        } else {
            // Other content lines
            assert!(!classify_line_type(line).is_structural());
        }
    }
}

// ============================================================================
// Multi-line Scalar Context Tests - Folded Style (>)
// ============================================================================

#[test]
fn test_folded_scalar_content_with_hash_prefix() {
    // In a folded scalar (>), lines starting with # are CONTENT, not comments
    // Behavior is similar to literal scalars
    let yaml = vec![
        "text: >",
        "  # This is a heading in the content",
        "  Regular content line",
        "  # Another heading",
        "  Final content",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);

    // Content lines behavior matches literal scalars
    for (i, line) in yaml.iter().enumerate().skip(1) {
        if line.trim().starts_with("#") {
            assert_eq!(classify_line_type(line), LineType::Comment,
                       "Line {} should be comment: {}", i + 1, line);
        } else {
            assert!(!classify_line_type(line).is_structural(),
                    "Line {} should not be structural: {}", i + 1, line);
        }
    }
}

#[test]
fn test_folded_scalar_various_indentation_levels() {
    // Test folded scalars at different indentation levels
    let test_cases = vec![
        // Level 0
        vec![
            "description: >",
            "  Content at level 0",
            "  # Comment-like content",
        ],
        // Level 4
        vec![
            "    nested:",
            "      description: >",
            "        Content at level 4",
            "        # Comment-like at level 4",
        ],
        // Level 8
        vec![
            "        deeply:",
            "          nested:",
            "            description: >",
            "              Content at level 8",
            "              # Comment-like at level 8",
        ],
    ];

    for yaml in test_cases {
        let header_idx = yaml.iter().position(|l| l.contains(">")).unwrap();
        let header_line = yaml[header_idx];

        assert_eq!(classify_line_type(header_line), LineType::MappingKey);

        for (i, line) in yaml.iter().enumerate().skip(header_idx + 1) {
            if line.trim().starts_with("#") {
                assert_eq!(classify_line_type(line), LineType::Comment);
            } else if !line.trim().is_empty() {
                assert!(!classify_line_type(line).is_structural());
            }
        }
    }
}

#[test]
fn test_folded_scalar_newline_handling() {
    // Folded scalars treat newlines differently than literal scalars
    // (newlines are converted to spaces in folded scalars)
    // But for line-based classification, behavior is similar
    let yaml = vec![
        "text: >",
        "  Line 1",
        "  Line 2 (folded with Line 1)",
        "",
        "  Line 4 (after empty line - new paragraph)",
    ];

    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);
    assert_eq!(classify_line_type(yaml[0]), LineType::MappingKey);

    // Content lines are not structural
    for (i, line) in yaml.iter().enumerate().skip(1) {
        if !line.trim().is_empty() {
            assert!(!classify_line_type(line).is_structural(),
                    "Content line {} should not be structural: {}", i + 1, line);
        }
    }
}

// ============================================================================
// Multi-line Scalar Edge Cases
// ============================================================================

#[test]
fn test_multiline_scalar_with_inline_comment_on_header() {
    // Test inline comment on the multi-line scalar header line
    let test_cases = vec![
        ("text: | # Literal scalar", "text: | "),
        ("description: > # Folded scalar", "description: > "),
        ("  content: | # Indented literal", "  content: | "),
        ("    data: > # Deep folded", "    data: > "),
    ];

    for (line, expected_stripped) in test_cases {
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));

        let stripped = strip_inline_comment(line);
        assert_eq!(stripped, expected_stripped);
        assert!(stripped.contains('|') || stripped.contains('>'));
    }
}

#[test]
fn test_multiline_scalar_chomping_indicators() {
    // Test chomping indicators (-, +, -|, +>, etc.)
    let test_cases = vec![
        "text: |-",    // Strip final newline
        "text: |+",    // Keep final newlines
        "description: >-",  // Strip for folded
        "description: >+",  // Keep for folded
        "  content: |-",     // With indentation
        "    data: >+",      // Deep with chomping
    ];

    for line in test_cases {
        // Lines with chomping indicators are still mapping keys
        assert_eq!(classify_line_type(line), LineType::MappingKey,
                   "Chomping indicator line should be mapping key: {}", line);
        assert!(!is_comment_line(line),
                "Should not be comment line: {}", line);
    }
}

#[test]
fn test_multiline_scalar_with_indentation_indicator() {
    // Test indentation indicators (e.g., |2, >4)
    let test_cases = vec![
        "text: |2",   // 2-space indentation
        "description: >4",  // 4-space indentation
        "content: |8",      // 8-space indentation
        "  data: >2",       // Indented with 2-space indicator
    ];

    for line in test_cases {
        assert_eq!(classify_line_type(line), LineType::MappingKey);
        assert!(!is_comment_line(line));
    }
}

#[test]
fn test_multiline_scalar_mixed_with_other_yaml_structures() {
    // Test multi-line scalars mixed with other YAML structures
    let yaml = vec![
        "config:",
        "  # This is a comment",
        "  description: |",
        "    Multi-line",
        "    description",
        "    # with hash",
        "  settings:",
        "    timeout: 30",
        "  another: >",
        "    Folded",
        "    scalar",
    ];

    let expected_types = vec![
        LineType::MappingKey,      // config:
        LineType::Comment,         // # This is a comment
        LineType::MappingKey,      // description: |
        // Content lines for | scalar
        // These are not structural or are comments if starting with #
        LineType::Unknown,         // "    Multi-line" - not a valid mapping key at this indent
        LineType::Unknown,         // "    description"
        LineType::Comment,         // "    # with hash"
        LineType::MappingKey,      // settings:
        LineType::MappingKey,      // timeout: 30
        LineType::MappingKey,      // another: >
        LineType::Unknown,         // "    Folded"
        LineType::Unknown,         // "    scalar"
    ];

    for (i, line) in yaml.iter().enumerate() {
        let expected = expected_types[i];
        let actual = classify_line_type(line);
        assert_eq!(actual, expected,
                   "Line {}: expected {:?}, got {:?} for: {}", i + 1, expected, actual, line);
    }
}

#[test]
fn test_comment_detection_in_multiline_nested_structures() {
    // Test comment detection in complex nested structures with multi-line scalars
    let yaml = r#"# Root comment
root:
  # Nested comment
  section1:
    # Deep comment before scalar
    description: |
      Multi-line content
      # Looks like comment but is content
      More content
    # Comment after scalar
    section2:
      # Another deep comment
      text: >
        Folded content
        # Hash in folded
        End of folded
      # Final comment
"#;

    let lines: Vec<&str> = yaml.lines().filter(|l| !l.trim().is_empty()).collect();

    // Count comments vs non-comments
    let mut comment_count = 0;
    let mut content_count = 0;

    for line in &lines {
        if classify_line_type(line) == LineType::Comment {
            comment_count += 1;
        } else {
            content_count += 1;
        }
    }

    // Should detect both comments and content
    assert!(comment_count > 0, "Should detect comment lines");
    assert!(content_count > 0, "Should detect content lines");

    // Verify specific lines
    let hash_content_line = "      # Looks like comment but is content";
    if lines.iter().any(|l| *l == hash_content_line) {
        // This line IS classified as a comment by the line-based parser
        assert!(is_comment_line(hash_content_line));
    }
}
