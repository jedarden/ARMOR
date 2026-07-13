//! Test suite for malformed or improperly formatted error messages
//!
//! This test suite verifies that the error handling system gracefully handles:
//! - Empty or null field values in error messages
//! - Invalid character sequences
//! - Incomplete or truncated message patterns
//! - Messages that don't match expected formatting patterns
//! - Unicode edge cases and encoding issues

use armor::parsers::yaml::{ParseError, ValidationError};

// ============================================================================
// Section 1: Empty and Null Field Values
// ============================================================================

#[test]
fn test_parse_error_with_empty_path() {
    // Tests ParseError handling when path is an empty string
    //
    // Expected behavior: Should not panic, should display <unknown> location
    let error = ParseError::syntax("invalid YAML")
        .with_path("");

    let location = error.location_string();
    assert!(location == "<unknown>" || location == "",
        "Empty path should result in <unknown> or empty location, got: '{}'", location);

    // Should still be able to format the error
    let display = format!("{}", error);
    assert!(!display.is_empty(), "Display should not be empty for empty path");
}

#[test]
fn test_parse_error_with_empty_context() {
    // Tests ParseError handling when context is empty
    //
    // Expected behavior: Should format correctly without crashing
    let error = ParseError::syntax("test error")
        .with_path("config.yaml")
        .with_context("");

    let summary = error.summary();
    assert!(summary.contains("test error"), "Should contain error message");
    // Empty context should not break the summary
    assert!(summary.contains("config.yaml"), "Should contain path");
}

#[test]
fn test_parse_error_with_empty_message() {
    // Tests ParseError with empty error message
    //
    // Expected behavior: Should handle gracefully with default error type
    let error = ParseError::syntax("");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Empty message should still produce display output");
}

#[test]
fn test_validation_error_with_empty_path() {
    // Tests ValidationError with empty field path
    //
    // Expected behavior: Should format correctly despite empty path
    let error = ValidationError::new("", "port must be between 1 and 65535");

    let display = format!("{}", error);
    assert!(display.contains("port must be between 1 and 65535"),
        "Should contain message even with empty path");
}

#[test]
fn test_validation_error_with_empty_message() {
    // Tests ValidationError with empty error message
    //
    // Expected behavior: Should format with empty message field
    let error = ValidationError::new("server.port", "");

    let display = format!("{}", error);
    assert!(display.contains("server.port") || display.contains("validation error"),
        "Should contain path or error type even with empty message");
}

// ============================================================================
// Section 2: Invalid Character Sequences
// ============================================================================

#[test]
fn test_parse_error_with_null_bytes() {
    // Tests ParseError handling with null bytes in context
    //
    // Expected behavior: Should handle gracefully without crashing
    let error = ParseError::syntax("error\x00with\x00nulls")
        .with_path("test.yaml");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle null bytes gracefully");
}

#[test]
fn test_parse_error_with_control_characters() {
    // Tests ParseError with various control characters
    //
    // Expected behavior: Should format without crashing
    let test_cases = vec![
        "error\nwith\nnewlines",
        "error\twith\ttabs",
        "error\rcarriage\rreturn",
        "error\x1bescape",
    ];

    for msg in test_cases {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle control characters in: {:?}", msg);
    }
}

#[test]
fn test_parse_error_with_unicode_edge_cases() {
    // Tests ParseError with Unicode edge cases
    //
    // Expected behavior: Should handle Unicode correctly
    let test_cases = vec![
        "error with émojis 🎉",
        "error with 反转字符",
        "error with אַרַבעִת",
        "error with ₭uᵲ₳ic",
        "error with 🚀🌟✨",
    ];

    for msg in test_cases {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle Unicode in: {}", msg);
    }
}

#[test]
fn test_validation_error_with_special_characters() {
    // Tests ValidationError with special characters in path and message
    //
    // Expected behavior: Should handle special characters gracefully
    let error = ValidationError::new("field.with.dots", "error: with $pecial chars!");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle special characters");
}

#[test]
fn test_error_with_invalid_utf8_sequences() {
    // Tests error handling with invalid UTF-8 sequences
    //
    // Expected behavior: Should not panic, should handle or sanitize
    let invalid_bytes = vec![0xFF, 0xFE, 0xFD];
    let result = String::from_utf8(invalid_bytes);

    assert!(result.is_err(), "Should detect invalid UTF-8");

    // Test that ParseError handles this gracefully via From impl
    let utf8_err = result.unwrap_err();
    let parse_error: ParseError = utf8_err.into();

    assert!(parse_error.kind.to_string().contains("UTF-8") || parse_error.context.contains("UTF-8"),
        "Should convert UTF-8 error to ParseError");
}

// ============================================================================
// Section 3: Incomplete or Truncated Message Patterns
// ============================================================================

#[test]
fn test_parse_error_with_incomplete_location() {
    // Tests ParseError with partial location information
    //
    // Expected behavior: Should format with available location only
    let error = ParseError::syntax("test error")
        .with_line(5);
    // No column, no path

    let location = error.location_string();
    assert!(location.contains("5"), "Should contain line number");
    assert!(!location.contains("column"), "Should not reference missing column");
}

#[test]
fn test_parse_error_with_only_column() {
    // Tests ParseError with only column information
    //
    // Expected behavior: Should show column only
    let error = ParseError::syntax("test error")
        .with_column(10);

    let location = error.location_string();
    assert!(location.contains("10") || location.contains("col"),
        "Should contain column information");
}

#[test]
fn test_parse_error_with_zero_values() {
    // Tests ParseError with zero values for line/column
    //
    // Expected behavior: Zero values should be handled gracefully
    let error = ParseError::syntax("test error")
        .with_location(0, 0);

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle zero values");
}

#[test]
fn test_validation_error_with_incomplete_line_info() {
    // Tests ValidationError with incomplete line information
    //
    // Expected behavior: Should format with available information
    let error = ValidationError::new("field", "message")
        .with_line(0);

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle zero line number");
}

// ============================================================================
// Section 4: Messages That Don't Match Expected Patterns
// ============================================================================

#[test]
fn test_parse_error_with_malformed_type_mismatch() {
    // Tests type mismatch error with unusual field names
    //
    // Expected behavior: Should handle unusual field names
    let unusual_fields = vec![
        "",                    // empty
        "...",                 // dots
        "field..name",         // double dots
        "field.[0].name",     // brackets
        "field with spaces",  // spaces
        "field\nwith\nnewlines", // newlines
    ];

    for field in unusual_fields {
        let error = ParseError::type_mismatch(field, "string", "integer");
        let display = format!("{}", error.kind);
        assert!(!display.is_empty(), "Should handle unusual field: {:?}", field);
    }
}

#[test]
fn test_validation_error_with_malformed_paths() {
    // Tests ValidationError with malformed field paths
    //
    // Expected behavior: Should handle gracefully
    let malformed_paths = vec![
        "field.",
        ".field",
        "field..name",
        "field...name",
        "field.[0]",
        "[0].field",
        "field[0]",
        "field.",
    ];

    for path in malformed_paths {
        let error = ValidationError::new(path, "validation failed");
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle malformed path: {}", path);
    }
}

#[test]
fn test_error_with_extremely_long_messages() {
    // Tests error handling with extremely long messages
    //
    // Expected behavior: Should handle without truncation or crash
    let long_message = "error ".repeat(10000);
    let error = ParseError::syntax(&long_message);

    let display = format!("{}", error);
    assert!(display.len() > 10000, "Should preserve long message");
}

#[test]
fn test_error_with_extremely_long_paths() {
    // Tests error handling with extremely long paths
    //
    // Expected behavior: Should handle long paths
    let long_path = "a".repeat(10000);
    let error = ValidationError::new(&long_path, "validation failed");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle extremely long path");
}

// ============================================================================
// Section 5: Edge Cases and Boundary Conditions
// ============================================================================

#[test]
fn test_parse_error_with_large_line_column_numbers() {
    // Tests error handling with very large line/column numbers
    //
    // Expected behavior: Should handle large numbers without overflow
    let error = ParseError::syntax("test error")
        .with_location(999999, 999999);

    let display = format!("{}", error);
    assert!(display.contains("999999"), "Should handle large line/column numbers");
}

#[test]
fn test_parse_error_builder_pattern_chaining() {
    // Tests that builder pattern works even with malformed intermediate states
    //
    // Expected behavior: Each step should produce valid error
    let error1 = ParseError::syntax("test");
    assert!(!format!("{}", error1).is_empty());

    let error2 = error1.with_path("");
    assert!(!format!("{}", error2).is_empty());

    let error3 = error2.with_line(0);
    assert!(!format!("{}", error3).is_empty());

    let error4 = error3.with_context("");
    assert!(!format!("{}", error4).is_empty());
}

#[test]
fn test_validation_error_with_whitespace_only() {
    // Tests error with whitespace-only strings
    //
    // Expected behavior: Should handle whitespace-only fields
    let error = ValidationError::new("   ", "   ");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle whitespace-only fields");
}

#[test]
fn test_parse_error_with_newline_in_message() {
    // Tests error message with embedded newlines
    //
    // Expected behavior: Should preserve newlines in context
    let error = ParseError::syntax("line 1\nline 2\nline 3");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle multi-line messages");
}

#[test]
fn test_parse_error_detailed_report_with_empty_snippet() {
    // Tests detailed_report with empty snippet
    //
    // Expected behavior: Should format without snippet section
    let error = ParseError::syntax("test error")
        .with_path("config.yaml")
        .with_line(5)
        .with_snippet("");

    let report = error.detailed_report();
    assert!(!report.is_empty(), "Report should not be empty");
    assert!(report.contains("test error"), "Should contain error message");
}

#[test]
fn test_parse_error_summary_with_all_empty_fields() {
    // Tests summary with minimal information
    //
    // Expected behavior: Should produce valid summary with <unknown> location
    let error = ParseError::syntax("test");

    let summary = error.summary();
    assert!(!summary.is_empty(), "Summary should not be empty");
    assert!(summary.contains("test") || summary.contains("syntax"),
        "Should contain error information");
}

// ============================================================================
// Section 6: Special Characters Only (No Alphanumeric)
// ============================================================================

#[test]
fn test_parse_error_with_symbols_only() {
    // Tests ParseError with messages containing only symbols (no alphanumeric)
    //
    // Expected behavior: Should handle symbol-only messages gracefully
    let symbol_only_messages = vec![
        "!@#$%^&*()",
        "+++=---",
        "***///",
        ":::::",
        "|||||",
        "#####",
        "~~~~~",
        "+++++",
        "-----",
        "======",
        "*****",
        "%%%%%",
        "&&&&&",
        "|||||",
        "#####",
        ":::::",
        ";;;;;",
        "<<<<<",
        ">>>>>",
        "?????",
    ];

    for msg in symbol_only_messages {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle symbol-only message: '{}'", msg);
    }
}

#[test]
fn test_parse_error_with_punctuation_only() {
    // Tests ParseError with messages containing only punctuation
    //
    // Expected behavior: Should handle punctuation-only messages
    let punctuation_only = vec![
        "...",
        ",,,",
        ";;;",
        ":::",
        "!!!",
        "???",
        "???...",
        "!!!???",
        "...",
        "---",
        "___",
        "'''",
        "\"\"\"",
        "```",
        "~~~",
    ];

    for msg in punctuation_only {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle punctuation-only message: '{}'", msg);
    }
}

#[test]
fn test_parse_error_with_brackets_only() {
    // Tests ParseError with messages containing only brackets/braces
    //
    // Expected behavior: Should handle bracket-only messages
    let bracket_only = vec![
        "[]{}()",
        "{{{{",
        "}}}}",
        "((((",
        "))))",
        "[[[[",
        "]]]]",
        "<{{{",
        ">}}}",
        "()[]{}",
        "<<>>",
    ];

    for msg in bracket_only {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle bracket-only message: '{}'", msg);
    }
}

#[test]
fn test_parse_error_with_mixed_special_chars_only() {
    // Tests ParseError with various combinations of special characters (no alphanumeric)
    //
    // Expected behavior: Should handle mixed special character messages
    let mixed_special = vec![
        "!@#$%^&*()_+-=[]{}|;':\",./<>?",
        "~`!@#$%^&*()_+-=[]{}\\|;':\",./<>?",
        "^&*()_+-=",
        "!@#$%",
        "^&*()",
        "_+-=",
        "[]\\|;':\",./<>?",
        "~`",
        "!!!@@@###",
        "^^^&&&***(((",
        "___---===",
        "```~~~",
    ];

    for msg in mixed_special {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle mixed special chars: '{}'", msg);
    }
}

#[test]
fn test_validation_error_with_special_char_only_path() {
    // Tests ValidationError with special character-only field paths
    //
    // Expected behavior: Should handle special character-only paths
    let special_paths = vec![
        "!",
        "@",
        "#",
        "$",
        "%",
        "^",
        "&",
        "*",
        "...",
        ":::",
        "++",
        "--",
        "==",
        "(((",
        ")))",
        "[[",
        "]]",
        "{{",
        "}}",
    ];

    for path in special_paths {
        let error = ValidationError::new(path, "validation failed");
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle special-char-only path: '{}'", path);
    }
}

#[test]
fn test_validation_error_with_special_char_only_message() {
    // Tests ValidationError with special character-only error messages
    //
    // Expected behavior: Should handle special character-only messages
    let special_messages = vec![
        "!!!",
        "???",
        "...",
        "!!!",
        "@@@",
        "###",
        "$$$",
        "^^^",
        "&&&",
        "***",
        "+++",
        "---",
        "===",
        ":::",
        ";;;",
        "|||",
        "~~~",
        "```",
    ];

    for msg in special_messages {
        let error = ValidationError::new("field", msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle special-char-only message: '{}'", msg);
    }
}

#[test]
fn test_parse_error_with_whitespace_variations() {
    // Tests ParseError with various whitespace-only patterns
    //
    // Expected behavior: Should handle different whitespace patterns
    let whitespace_variations = vec![
        "     ",      // spaces only
        "\t\t\t",     // tabs only
        "\n\n\n",     // newlines only
        " \t \t ",    // mixed spaces and tabs
        " \n \n ",    // mixed spaces and newlines
        "\t\n\t\n",   // mixed tabs and newlines
        " \t\n ",     // all three
        "\r\r\r",     // carriage returns
        " \r \r ",    // spaces and CR
        "\r\n\r\n",   // CRLF sequences
    ];

    for msg in whitespace_variations {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle whitespace variation: {:?}", msg);
    }
}

#[test]
fn test_parse_error_with_single_special_chars() {
    // Tests ParseError with single special character messages
    //
    // Expected behavior: Should handle single special character messages
    let single_chars = vec![
        "!", "@", "#", "$", "%", "^", "&", "*", "(", ")",
        "-", "_", "=", "+", "[", "]", "{", "}", "|", "\\",
        ";", ":", "\"", "'", "<", ">", ",", ".", "?", "/",
        "~", "`",
    ];

    for msg in single_chars {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle single special char: '{}'", msg);
    }
}

#[test]
fn test_parse_error_with_repeated_special_chars() {
    // Tests ParseError with repeated special character patterns
    //
    // Expected behavior: Should handle repeated special characters
    let repeated_patterns = vec![
        "!!!!!!!!",        // 8 exclamation marks
        "@@@@@@@@",        // 8 at signs
        "########",        // 8 hashes
        "$$$$$$$$",        // 8 dollar signs
        "%%%%%%%%",        // 8 percent signs
        "^^^^^^^^",        // 8 carets
        "&&&&&&&&",        // 8 ampersands
        "********",        // 8 asterisks
        "::::::::",        // 8 colons
        ";;;;;;;;",        // 8 semicolons
        "????????",        // 8 question marks
        "~~~~~~~~",        // 8 tildes
        "````````",        // 8 backticks
    ];

    for msg in repeated_patterns {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle repeated special chars: '{}'", msg);
    }
}

#[test]
fn test_parse_error_context_with_special_chars_only() {
    // Tests ParseError with special character-only context
    //
    // Expected behavior: Should handle special character-only context
    let error = ParseError::syntax("syntax error")
        .with_context("!@#$%^&*()");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle special-char-only context");
}

#[test]
fn test_parse_error_path_with_special_chars_only() {
    // Tests ParseError with special character-only paths
    //
    // Expected behavior: Should handle special character-only paths
    let special_paths = vec![
        "!@#$",
        "^&*()",
        "_+-=",
        "[]{}",
        "|\\:",
        ";'",
        "<>",
        "?/",
        "~`",
    ];

    for path in special_paths {
        let error = ParseError::syntax("error").with_path(path);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle special-char-only path: '{}'", path);
    }
}

#[test]
fn test_validation_error_with_both_special_char_fields() {
    // Tests ValidationError with both path and message as special chars only
    //
    // Expected behavior: Should handle both fields being special characters
    let error = ValidationError::new("!!!", "@@@");

    let display = format!("{}", error);
    assert!(!display.is_empty(), "Should handle both fields as special chars only");
}

#[test]
fn test_error_kind_with_special_char_messages() {
    // Tests various ParseErrorKind with special character-only messages
    //
    // Expected behavior: All error kinds should handle special char messages
    use armor::parsers::yaml::ParseErrorKind;

    let test_cases = vec![
        ParseErrorKind::Syntax("!!!".to_string()),
        ParseErrorKind::Io("@@@".to_string()),
        ParseErrorKind::Validation("###".to_string()),
        ParseErrorKind::UnknownAnchor("$$$".to_string()),
        ParseErrorKind::DuplicateKey("%%%".to_string()),
        ParseErrorKind::Other("^^^".to_string()),
    ];

    for kind in test_cases {
        let display = format!("{}", kind);
        assert!(!display.is_empty(), "Error kind should handle special chars: {:?}", kind);
    }
}

#[test]
fn test_parse_error_with_escaped_special_chars() {
    // Tests ParseError with escaped special character sequences
    //
    // Expected behavior: Should handle escaped sequences properly
    let escaped_sequences = vec![
        "\\n\\t\\r",      // escaped whitespace
        "\\\\////",       // mixed slashes
        "\\\"\\'\\`",      // escaped quotes
        "\\[\\]\\{\\}",   // escaped brackets
        "\\<\\>",         // escaped angle brackets
    ];

    for msg in escaped_sequences {
        let error = ParseError::syntax(msg);
        let display = format!("{}", error);
        assert!(!display.is_empty(), "Should handle escaped sequences: '{}'", msg);
    }
}

// ============================================================================
// Section 7: Error Type Detection Malformations
// ============================================================================

#[test]
fn test_error_kind_display_edge_cases() {
    // Tests Display implementation for all error kinds with edge cases
    //
    // Expected behavior: All error kinds should display without panicking
    use armor::parsers::yaml::ParseErrorKind;

    let test_cases = vec![
        ParseErrorKind::Syntax("".to_string()),
        ParseErrorKind::Io("".to_string()),
        ParseErrorKind::Validation("".to_string()),
        ParseErrorKind::TypeMismatch {
            field: "".to_string(),
            expected: "".to_string(),
            actual: "".to_string(),
        },
        ParseErrorKind::UnknownAnchor("".to_string()),
        ParseErrorKind::DuplicateKey("".to_string()),
        ParseErrorKind::Other("".to_string()),
        ParseErrorKind::UnexpectedEof,
        ParseErrorKind::InvalidUtf8,
    ];

    for kind in test_cases {
        let display = format!("{}", kind);
        assert!(!display.is_empty(), "Error kind {:?} should produce display output", kind);
    }
}

#[test]
fn test_error_from_standard_errors() {
    // Tests conversion from standard library errors
    //
    // Expected behavior: Should convert and display properly
    use std::io;

    let io_err = io::Error::new(io::ErrorKind::NotFound, "");
    let parse_err: ParseError = io_err.into();
    assert!(!format!("{}", parse_err).is_empty());
}

#[test]
fn test_parse_error_format_structured_edge_cases() {
    // Tests format_structured with edge cases
    //
    // Expected behavior: Should produce valid structured output
    let error = ParseError::syntax("")
        .with_path("")
        .with_location(0, 0);

    let structured = error.format_structured();
    assert!(!structured.is_empty(), "Structured format should not be empty");
    assert!(structured.contains("ParseError"), "Should contain type name");
}
