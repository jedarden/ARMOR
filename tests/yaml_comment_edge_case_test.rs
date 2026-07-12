//! YAML Comment Edge Case Tests
//!
//! These tests verify edge case scenarios in YAML comment filtering:
//! - Empty lines around comments
//! - Consecutive comment lines
//! - Comments with special characters (!@#$ etc.)
//! - Boundary conditions (start/end of document)
//!
//! Bead: bf-12vgr
//! Acceptance Criteria:
//! - Test for empty lines around comments
//! - Test for consecutive comment lines handling
//! - Test for comments with special characters (!@#$ etc.)
//! - Test for boundary condition handling
//! - All tests pass

use armor::parsers::yaml::{
    classify_line_type, strip_inline_comment, is_comment_line, LineType
};

// Empty Lines Around Comments Tests

#[test]
fn test_empty_line_before_comment() {
    // Empty line preceding a comment
    let lines = vec![
        "",                     // line 1: empty
        "# comment after empty", // line 2: comment
    ];

    // Verify empty line classification
    assert_eq!(classify_line_type(lines[0]), LineType::Blank);
    assert!(!is_comment_line(lines[0]));

    // Verify comment line classification
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);
    assert!(is_comment_line(lines[1]));

    // Verify stripping works correctly
    assert_eq!(strip_inline_comment(lines[0]), "");
    assert_eq!(strip_inline_comment(lines[1]), "");
}

#[test]
fn test_empty_line_after_comment() {
    // Empty line following a comment
    let lines = vec![
        "# comment before empty", // line 1: comment
        "",                       // line 2: empty
    ];

    // Verify comment line classification
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert!(is_comment_line(lines[0]));

    // Verify empty line classification
    assert_eq!(classify_line_type(lines[1]), LineType::Blank);
    assert!(!is_comment_line(lines[1]));

    // Verify stripping works correctly
    assert_eq!(strip_inline_comment(lines[0]), "");
    assert_eq!(strip_inline_comment(lines[1]), "");
}

#[test]
fn test_multiple_empty_lines_around_comment() {
    // Multiple empty lines around a comment
    let lines = vec![
        "",                     // line 1: empty
        "",                     // line 2: empty
        "# comment",            // line 3: comment
        "",                     // line 4: empty
        "",                     // line 5: empty
    ];

    // Verify all empty lines are classified as Blank
    assert_eq!(classify_line_type(lines[0]), LineType::Blank);
    assert_eq!(classify_line_type(lines[1]), LineType::Blank);
    assert_eq!(classify_line_type(lines[3]), LineType::Blank);
    assert_eq!(classify_line_type(lines[4]), LineType::Blank);

    // Verify comment is classified correctly
    assert_eq!(classify_line_type(lines[2]), LineType::Comment);
    assert!(is_comment_line(lines[2]));

    // None of the empty lines should be detected as comments
    assert!(!is_comment_line(lines[0]));
    assert!(!is_comment_line(lines[1]));
    assert!(!is_comment_line(lines[3]));
    assert!(!is_comment_line(lines[4]));
}

#[test]
fn test_whitespace_only_line_around_comment() {
    // Whitespace-only lines around comments
    let lines = vec![
        "   ",                  // line 1: whitespace only
        "# comment",            // line 2: comment
        "  \t  ",              // line 3: mixed whitespace
    ];

    // Verify whitespace-only lines are classified as Blank
    assert_eq!(classify_line_type(lines[0]), LineType::Blank);
    assert_eq!(classify_line_type(lines[2]), LineType::Blank);

    // Verify comment is classified correctly
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);

    // Whitespace-only lines should not be comments
    assert!(!is_comment_line(lines[0]));
    assert!(!is_comment_line(lines[2]));
}

#[test]
fn test_indented_comment_with_empty_lines() {
    // Indented comment with empty lines
    let lines = vec![
        "",                     // line 1: empty
        "  # indented comment", // line 2: indented comment
        "",                     // line 3: empty
    ];

    // Verify empty line classification
    assert_eq!(classify_line_type(lines[0]), LineType::Blank);
    assert_eq!(classify_line_type(lines[2]), LineType::Blank);

    // Verify indented comment classification
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);
    assert!(is_comment_line(lines[1]));

    // Verify indented comment stripping preserves leading whitespace
    assert_eq!(strip_inline_comment(lines[1]), "  ");
}

// Consecutive Comment Lines Tests

#[test]
fn test_two_consecutive_comment_lines() {
    // Two consecutive comment lines
    let lines = vec![
        "# first comment",     // line 1: comment
        "# second comment",    // line 2: comment
    ];

    // Both should be classified as comments
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);

    // Both should be detected as comment lines
    assert!(is_comment_line(lines[0]));
    assert!(is_comment_line(lines[1]));

    // Both should strip to empty
    assert_eq!(strip_inline_comment(lines[0]), "");
    assert_eq!(strip_inline_comment(lines[1]), "");
}

#[test]
fn test_multiple_consecutive_comment_lines() {
    // Multiple consecutive comment lines
    let lines = vec![
        "# comment 1",         // line 1: comment
        "# comment 2",         // line 2: comment
        "# comment 3",         // line 3: comment
        "# comment 4",         // line 4: comment
        "# comment 5",         // line 5: comment
    ];

    // All should be classified as comments
    for line in &lines {
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert!(is_comment_line(line));
        assert_eq!(strip_inline_comment(line), "");
    }
}

#[test]
fn test_indented_consecutive_comment_lines() {
    // Consecutive indented comment lines
    let lines = vec![
        "  # first indented",   // line 1: indented comment
        "  # second indented",  // line 2: indented comment
        "  # third indented",   // line 3: indented comment
    ];

    // All should be classified as comments
    for line in &lines {
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert!(is_comment_line(line));
        // All should preserve leading whitespace when stripped
        assert_eq!(strip_inline_comment(line), "  ");
    }
}

#[test]
fn test_varying_indentation_consecutive_comments() {
    // Consecutive comments with varying indentation
    let lines = vec![
        "# root level",         // line 1: 0 indent
        "  # level 1",         // line 2: 2 spaces
        "    # level 2",       // line 3: 4 spaces
        "  # back to level 1", // line 4: 2 spaces
    ];

    // All should be classified as comments
    for line in &lines {
        assert_eq!(classify_line_type(line), LineType::Comment);
        assert!(is_comment_line(line));
    }

    // Verify indentation is preserved correctly
    assert_eq!(strip_inline_comment(lines[0]), "");
    assert_eq!(strip_inline_comment(lines[1]), "  ");
    assert_eq!(strip_inline_comment(lines[2]), "    ");
    assert_eq!(strip_inline_comment(lines[3]), "  ");
}

#[test]
fn test_comment_block_with_content_lines() {
    // Comment block surrounding content lines
    let lines = vec![
        "# start comment block",   // line 1: comment
        "# describes next section", // line 2: comment
        "",                          // line 3: empty
        "key: value",                // line 4: content
        "",                          // line 5: empty
        "# end comment block",       // line 6: comment
    ];

    // Verify classifications
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);
    assert_eq!(classify_line_type(lines[2]), LineType::Blank);
    assert_eq!(classify_line_type(lines[3]), LineType::MappingKey);
    assert_eq!(classify_line_type(lines[4]), LineType::Blank);
    assert_eq!(classify_line_type(lines[5]), LineType::Comment);

    // Verify comment detection
    assert!(is_comment_line(lines[0]));
    assert!(is_comment_line(lines[1]));
    assert!(!is_comment_line(lines[2]));
    assert!(!is_comment_line(lines[3]));
    assert!(!is_comment_line(lines[4]));
    assert!(is_comment_line(lines[5]));
}

// Special Characters in Comments Tests

#[test]
fn test_comment_with_all_special_characters() {
    // Comment containing all common special characters
    let special_chars = "!@#$%^&*()_+-=[]{}|;':\",./<>?`~";
    let line = format!("# comment with all special chars: {}", special_chars);

    // Should be classified as comment
    assert_eq!(classify_line_type(&line), LineType::Comment);
    assert!(is_comment_line(&line));

    // Should strip to empty
    assert_eq!(strip_inline_comment(&line), "");
}

#[test]
fn test_comment_with_exclamation_mark() {
    // Comments with exclamation marks
    assert_eq!(classify_line_type("# TODO! important"), LineType::Comment);
    assert_eq!(classify_line_type("# WARNING! danger"), LineType::Comment);
    assert_eq!(classify_line_type("# Note! read this"), LineType::Comment);
    assert_eq!(classify_line_type("# !!!!!"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# TODO! important"));
    assert!(is_comment_line("# WARNING! danger"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# TODO! important"), "");
    assert_eq!(strip_inline_comment("# WARNING! danger"), "");
}

#[test]
fn test_comment_with_at_sign() {
    // Comments with @ signs
    assert_eq!(classify_line_type("# @username mention"), LineType::Comment);
    assert_eq!(classify_line_type("# @@double at"), LineType::Comment);
    assert_eq!(classify_line_type("# @@@@ multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# @username mention"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# @username mention"), "");
}

#[test]
fn test_comment_with_hash_sign() {
    // Comments with # signs (note: only first # preceded by whitespace starts comment)
    assert_eq!(classify_line_type("# comment with # hash"), LineType::Comment);
    assert_eq!(classify_line_type("# ### multiple hashes"), LineType::Comment);
    assert_eq!(classify_line_type("# ###### even more"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# comment with # hash"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# comment with # hash"), "");
    assert_eq!(strip_inline_comment("# ### multiple hashes"), "");
}

#[test]
fn test_comment_with_dollar_sign() {
    // Comments with $ signs
    assert_eq!(classify_line_type("# cost: $100"), LineType::Comment);
    assert_eq!(classify_line_type("# $$ double dollar"), LineType::Comment);
    assert_eq!(classify_line_type("# $$$$ many dollars"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# cost: $100"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# cost: $100"), "");
}

#[test]
fn test_comment_with_percent_sign() {
    // Comments with % signs
    assert_eq!(classify_line_type("# progress: 50%"), LineType::Comment);
    assert_eq!(classify_line_type("# %% double percent"), LineType::Comment);
    assert_eq!(classify_line_type("# %%%% multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# progress: 50%"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# progress: 50%"), "");
}

#[test]
fn test_comment_with_caret() {
    // Comments with ^ signs
    assert_eq!(classify_line_type("# press Ctrl^C"), LineType::Comment);
    assert_eq!(classify_line_type("# ^^ double caret"), LineType::Comment);
    assert_eq!(classify_line_type("# ^^^^^ multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# press Ctrl^C"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# press Ctrl^C"), "");
}

#[test]
fn test_comment_with_ampersand() {
    // Comments with & signs
    assert_eq!(classify_line_type("# this & that"), LineType::Comment);
    assert_eq!(classify_line_type("# && logical AND"), LineType::Comment);
    assert_eq!(classify_line_type("# &&&& multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# this & that"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# this & that"), "");
}

#[test]
fn test_comment_with_asterisk() {
    // Comments with * signs
    assert_eq!(classify_line_type("# * wildcard"), LineType::Comment);
    assert_eq!(classify_line_type("# ** bold"), LineType::Comment);
    assert_eq!(classify_line_type("# **** multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# * wildcard"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# * wildcard"), "");
}

#[test]
fn test_comment_with_parentheses() {
    // Comments with parentheses
    assert_eq!(classify_line_type("# (parenthesized) text"), LineType::Comment);
    assert_eq!(classify_line_type("# ((nested) parentheses)"), LineType::Comment);
    assert_eq!(classify_line_type("# (((multiple))) levels"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# (parenthesized) text"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# (parenthesized) text"), "");
}

#[test]
fn test_comment_with_brackets() {
    // Comments with square brackets
    assert_eq!(classify_line_type("# [array] index"), LineType::Comment);
    assert_eq!(classify_line_type("# [[nested]] brackets"), LineType::Comment);
    assert_eq!(classify_line_type("# [[[multiple]]] levels"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# [array] index"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# [array] index"), "");
}

#[test]
fn test_comment_with_braces() {
    // Comments with curly braces
    assert_eq!(classify_line_type("# {key: value} object"), LineType::Comment);
    assert_eq!(classify_line_type("# {{nested}} braces"), LineType::Comment);
    assert_eq!(classify_line_type("# {{{multiple}}} levels"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# {key: value} object"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# {key: value} object"), "");
}

#[test]
fn test_comment_with_pipe() {
    // Comments with pipe characters
    assert_eq!(classify_line_type("# a | b pipe"), LineType::Comment);
    assert_eq!(classify_line_type("# || logical OR"), LineType::Comment);
    assert_eq!(classify_line_type("# |||| multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# a | b pipe"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# a | b pipe"), "");
}

#[test]
fn test_comment_with_backslash() {
    // Comments with backslash characters
    assert_eq!(classify_line_type("# path\\to\\file"), LineType::Comment);
    assert_eq!(classify_line_type("# \\\\ double backslash"), LineType::Comment);
    assert_eq!(classify_line_type("# \\\\\\\\ multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# path\\to\\file"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# path\\to\\file"), "");
}

#[test]
fn test_comment_with_colon() {
    // Comments with colons (common in TODO/FIXME)
    assert_eq!(classify_line_type("# TODO: implement this"), LineType::Comment);
    assert_eq!(classify_line_type("# FIXME: bug description"), LineType::Comment);
    assert_eq!(classify_line_type("# NOTE: important info"), LineType::Comment);
    assert_eq!(classify_line_type("# :::: multiple colons"), LineType::Comment);

    // All should be comment lines (despite containing colons)
    assert!(is_comment_line("# TODO: implement this"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# TODO: implement this"), "");
}

#[test]
fn test_comment_with_semicolon() {
    // Comments with semicolons
    assert_eq!(classify_line_type("# line1; line2"), LineType::Comment);
    assert_eq!(classify_line_type("# ;; double semicolon"), LineType::Comment);
    assert_eq!(classify_line_type("# ;;;; multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# line1; line2"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# line1; line2"), "");
}

#[test]
fn test_comment_with_quotes() {
    // Comments with various quote characters
    assert_eq!(classify_line_type("# \"double quotes\""), LineType::Comment);
    assert_eq!(classify_line_type("# 'single quotes'"), LineType::Comment);
    assert_eq!(classify_line_type("# `backticks`"), LineType::Comment);
    assert_eq!(classify_line_type("# \"'mixed\"'`quotes`"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# \"double quotes\""));
    assert!(is_comment_line("# 'single quotes'"));
    assert!(is_comment_line("# `backticks`"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# \"double quotes\""), "");
    assert_eq!(strip_inline_comment("# 'single quotes'"), "");
    assert_eq!(strip_inline_comment("# `backticks`"), "");
}

#[test]
fn test_comment_with_angle_brackets() {
    // Comments with angle brackets
    assert_eq!(classify_line_type("# <html> tag"), LineType::Comment);
    assert_eq!(classify_line_type("# <<nested>> brackets"), LineType::Comment);
    assert_eq!(classify_line_type("# <<<multiple>>> levels"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# <html> tag"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# <html> tag"), "");
}

#[test]
fn test_comment_with_slash() {
    // Comments with forward slashes
    assert_eq!(classify_line_type("# path/to/file"), LineType::Comment);
    assert_eq!(classify_line_type("# // double slash"), LineType::Comment);
    assert_eq!(classify_line_type("# //// multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# path/to/file"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# path/to/file"), "");
}

#[test]
fn test_comment_with_question_mark() {
    // Comments with question marks
    assert_eq!(classify_line_type("# what is this?"), LineType::Comment);
    assert_eq!(classify_line_type("# ?? confused"), LineType::Comment);
    assert_eq!(classify_line_type("# ????? very confused"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# what is this?"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# what is this?"), "");
}

#[test]
fn test_comment_with_tilde() {
    // Comments with tilde characters
    assert_eq!(classify_line_type("# ~ home directory"), LineType::Comment);
    assert_eq!(classify_line_type("# ~~ double tilde"), LineType::Comment);
    assert_eq!(classify_line_type("# ~~~~ multiple"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# ~ home directory"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# ~ home directory"), "");
}

#[test]
fn test_comment_with_grave_accent() {
    // Comments with grave accent (backtick)
    assert_eq!(classify_line_type("# `code` snippet"), LineType::Comment);
    assert_eq!(classify_line_type("# ```code block```"), LineType::Comment);
    assert_eq!(classify_line_type("# ````multiple````"), LineType::Comment);

    // All should be comment lines
    assert!(is_comment_line("# `code` snippet"));

    // All should strip to empty
    assert_eq!(strip_inline_comment("# `code` snippet"), "");
}

// Boundary Conditions Tests

#[test]
fn test_comment_at_document_start() {
    // Comment as the first line of the document
    let lines = vec![
        "# first line is comment", // line 1: comment at document start
        "key: value",              // line 2: content
    ];

    // Verify comment at start
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert!(is_comment_line(lines[0]));
    assert_eq!(strip_inline_comment(lines[0]), "");

    // Verify subsequent content
    assert_eq!(classify_line_type(lines[1]), LineType::MappingKey);
    assert!(!is_comment_line(lines[1]));
}

#[test]
fn test_comment_at_document_end() {
    // Comment as the last line of the document
    let lines = vec![
        "key: value",              // line 1: content
        "# last line is comment",  // line 2: comment at document end
    ];

    // Verify content first
    assert_eq!(classify_line_type(lines[0]), LineType::MappingKey);
    assert!(!is_comment_line(lines[0]));

    // Verify comment at end
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);
    assert!(is_comment_line(lines[1]));
    assert_eq!(strip_inline_comment(lines[1]), "");
}

#[test]
fn test_document_start_and_end_with_comments() {
    // Comments at both start and end of document
    let lines = vec![
        "# start comment",    // line 1: comment at start
        "key: value",         // line 2: content
        "# end comment",      // line 3: comment at end
    ];

    // Verify comment at start
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert!(is_comment_line(lines[0]));

    // Verify content in middle
    assert_eq!(classify_line_type(lines[1]), LineType::MappingKey);
    assert!(!is_comment_line(lines[1]));

    // Verify comment at end
    assert_eq!(classify_line_type(lines[2]), LineType::Comment);
    assert!(is_comment_line(lines[2]));
}

#[test]
fn test_comment_at_line_boundary() {
    // Test behavior at line boundaries
    let single_char_comments = vec![
        "#",
        " #",
        "  #",
        "\t#",
    ];

    for comment in single_char_comments {
        assert_eq!(classify_line_type(comment), LineType::Comment);
        assert!(is_comment_line(comment));
        // Should preserve leading whitespace when stripped
        let leading_whitespace = comment.chars().take_while(|c| c.is_whitespace()).collect::<String>();
        assert_eq!(strip_inline_comment(comment), leading_whitespace);
    }
}

#[test]
fn test_empty_document() {
    // Completely empty document
    let empty = "";

    assert_eq!(classify_line_type(empty), LineType::Blank);
    assert!(!is_comment_line(empty));
    assert_eq!(strip_inline_comment(empty), "");
}

#[test]
fn test_whitespace_only_document() {
    // Document with only whitespace
    let whitespace_only = vec![
        "   ",
        "\t\t",
        "  \t  ",
    ];

    for ws in whitespace_only {
        assert_eq!(classify_line_type(ws), LineType::Blank);
        assert!(!is_comment_line(ws));
        assert_eq!(strip_inline_comment(ws), ws);
    }
}

#[test]
fn test_comment_only_document() {
    // Document with only comments
    let comment_only = vec![
        "# comment 1",
        "# comment 2",
        "# comment 3",
    ];

    for comment in comment_only {
        assert_eq!(classify_line_type(comment), LineType::Comment);
        assert!(is_comment_line(comment));
        assert_eq!(strip_inline_comment(comment), "");
    }
}

#[test]
fn test_comment_immediately_followed_by_content() {
    // Comment immediately followed by content line (no empty line between)
    let lines = vec![
        "# comment",           // line 1: comment
        "key: value",          // line 2: content immediately after
    ];

    // Verify comment classification
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert!(is_comment_line(lines[0]));

    // Verify content immediately after comment
    assert_eq!(classify_line_type(lines[1]), LineType::MappingKey);
    assert!(!is_comment_line(lines[1]));
}

#[test]
fn test_content_immediately_followed_by_comment() {
    // Content immediately followed by comment line (no empty line between)
    let lines = vec![
        "key: value",          // line 1: content
        "# comment",           // line 2: comment immediately after
    ];

    // Verify content classification
    assert_eq!(classify_line_type(lines[0]), LineType::MappingKey);
    assert!(!is_comment_line(lines[0]));

    // Verify comment immediately after content
    assert_eq!(classify_line_type(lines[1]), LineType::Comment);
    assert!(is_comment_line(lines[1]));
}

#[test]
fn test_document_start_marker_with_comments() {
    // Document start marker with surrounding comments
    let lines = vec![
        "# before document start", // line 1: comment
        "---",                      // line 2: document start
        "# after document start",  // line 3: comment
    ];

    // Verify comment before marker
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert!(is_comment_line(lines[0]));

    // Verify document start marker
    assert_eq!(classify_line_type(lines[1]), LineType::DocumentStart);
    assert!(!is_comment_line(lines[1]));

    // Verify comment after marker
    assert_eq!(classify_line_type(lines[2]), LineType::Comment);
    assert!(is_comment_line(lines[2]));
}

#[test]
fn test_document_end_marker_with_comments() {
    // Document end marker with surrounding comments
    let lines = vec![
        "# before document end",   // line 1: comment
        "...",                      // line 2: document end
        "# after document end",    // line 3: comment
    ];

    // Verify comment before marker
    assert_eq!(classify_line_type(lines[0]), LineType::Comment);
    assert!(is_comment_line(lines[0]));

    // Verify document end marker
    assert_eq!(classify_line_type(lines[1]), LineType::DocumentEnd);
    assert!(!is_comment_line(lines[1]));

    // Verify comment after marker
    assert_eq!(classify_line_type(lines[2]), LineType::Comment);
    assert!(is_comment_line(lines[2]));
}

// Integration Tests for Edge Cases

#[test]
fn test_realistic_config_file_with_comments() {
    // Realistic configuration file with various comment patterns
    let yaml = r#"# Database Configuration
# This section defines database connection settings

database:
  host: localhost     # database host
  port: 5432         # default PostgreSQL port
  name: mydb         # database name

# API Configuration
api:
  base_url: https://api.example.com/v1#endpoint  # production API
  timeout: 30          # timeout in seconds

# TODO: Add SSL settings
# FIXME: Update for production deployment
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Verify comment lines at various positions
    assert!(is_comment_line(lines[0])); // # Database Configuration
    assert!(is_comment_line(lines[1])); // # This section defines...
    assert_eq!(classify_line_type(lines[2]), LineType::Blank); // empty line
    assert!(!is_comment_line(lines[3])); // database:
    assert!(!is_comment_line(lines[4])); //   host: localhost # comment
    assert!(!is_comment_line(lines[6])); //   name: mydb
    assert_eq!(classify_line_type(lines[7]), LineType::Blank); // empty line
    assert!(is_comment_line(lines[8])); // # API Configuration
    assert!(!is_comment_line(lines[9])); // api:
    assert!(!is_comment_line(lines[10])); //   base_url: ...
    assert!(!is_comment_line(lines[11])); //   timeout: 30
    assert_eq!(classify_line_type(lines[12]), LineType::Blank); // empty line
    assert!(is_comment_line(lines[13])); // # TODO: Add SSL settings
    assert!(is_comment_line(lines[14])); // # FIXME: Update for...
}

#[test]
fn test_comment_edge_cases_complete_document() {
    // Complete document testing all edge cases
    let yaml = r#"# Start of document
# With multiple consecutive comments

# Section header
key1: value1

# Comment block
# Line 1
# Line 2

key2: value2 # inline comment

# !@#$%^&*() special chars

# TODO: implement feature
# FIXME: fix bug
# NOTE: important

# End of document
"#;

    let lines: Vec<&str> = yaml.lines().collect();

    // Count comment lines
    let comment_count = lines.iter()
        .filter(|line| is_comment_line(*line))
        .count();

    // Should have 11 comment lines
    assert_eq!(comment_count, 11);

    // Verify specific lines
    assert!(is_comment_line(lines[0])); // # Start of document
    assert!(is_comment_line(lines[1])); // # With multiple...
    assert_eq!(classify_line_type(lines[2]), LineType::Blank); // empty
    assert!(is_comment_line(lines[3])); // # Section header
    assert!(!is_comment_line(lines[4])); // key1: value1
    assert_eq!(classify_line_type(lines[5]), LineType::Blank); // empty
    assert!(is_comment_line(lines[6])); // # Comment block
    assert!(is_comment_line(lines[7])); // # Line 1
    assert!(is_comment_line(lines[8])); // # Line 2
    assert_eq!(classify_line_type(lines[9]), LineType::Blank); // empty
    assert!(!is_comment_line(lines[10])); // key2: value2 # inline
    assert_eq!(classify_line_type(lines[11]), LineType::Blank); // empty
    assert!(is_comment_line(lines[12])); // # !@#$%^&*()
    assert_eq!(classify_line_type(lines[13]), LineType::Blank); // empty
    assert!(is_comment_line(lines[14])); // # TODO:...
    assert!(is_comment_line(lines[15])); // # FIXME:...
    assert!(is_comment_line(lines[16])); // # NOTE:...
    assert_eq!(classify_line_type(lines[17]), LineType::Blank); // empty
    assert!(is_comment_line(lines[18])); // # End of document
}
