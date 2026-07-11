//! Unit tests for ParseError variants and builder methods
//!
//! This test suite covers:
//! - Each ParseErrorKind variant
//! - Builder method edge cases
//! - Trait implementations (Clone, PartialEq)
//! - Constructor methods

use armor::parsers::yaml::{ParseError, ParseErrorKind};

// ===========================================================================
// Constructor Tests
// ===========================================================================

#[test]
fn test_new_creates_default_error() {
    let error = ParseError::new(ParseErrorKind::UnexpectedEof);
    assert_eq!(error.kind, ParseErrorKind::UnexpectedEof);
    assert_eq!(error.line, None);
    assert_eq!(error.column, None);
    assert_eq!(error.path, None);
    assert_eq!(error.snippet, None);
    assert_eq!(error.context, "");
}

#[test]
fn test_syntax_constructor() {
    let error = ParseError::syntax("invalid YAML syntax");
    assert!(matches!(error.kind, ParseErrorKind::Syntax(_)));
    if let ParseErrorKind::Syntax(msg) = error.kind {
        assert_eq!(msg, "invalid YAML syntax");
    }
}

#[test]
fn test_io_constructor() {
    let error = ParseError::io("file not found");
    assert!(matches!(error.kind, ParseErrorKind::Io(_)));
    if let ParseErrorKind::Io(msg) = error.kind {
        assert_eq!(msg, "file not found");
    }
}

#[test]
fn test_validation_constructor() {
    let error = ParseError::validation("value out of range");
    assert!(matches!(error.kind, ParseErrorKind::Validation(_)));
    if let ParseErrorKind::Validation(msg) = error.kind {
        assert_eq!(msg, "value out of range");
    }
}

#[test]
fn test_type_mismatch_constructor() {
    let error = ParseError::type_mismatch("port", "integer", "string");
    assert!(matches!(error.kind, ParseErrorKind::TypeMismatch { .. }));
    if let ParseErrorKind::TypeMismatch { field, expected, actual } = error.kind {
        assert_eq!(field, "port");
        assert_eq!(expected, "integer");
        assert_eq!(actual, "string");
    }
}

// ===========================================================================
// Builder Method Tests
// ===========================================================================

#[test]
fn test_with_line() {
    let error = ParseError::syntax("test").with_line(42);
    assert_eq!(error.line, Some(42));
    assert_eq!(error.column, None);
}

#[test]
fn test_with_column() {
    let error = ParseError::syntax("test").with_column(15);
    assert_eq!(error.column, Some(15));
    assert_eq!(error.line, None);
}

#[test]
fn test_with_path_string() {
    let error = ParseError::syntax("test").with_path("config.yaml");
    assert_eq!(error.path, Some("config.yaml".to_string()));
}

#[test]
fn test_with_path_str() {
    let error = ParseError::syntax("test").with_path("/etc/app/config.yaml");
    assert_eq!(error.path, Some("/etc/app/config.yaml".to_string()));
}

#[test]
fn test_with_snippet() {
    let snippet = "key: value\n  invalid: true";
    let error = ParseError::syntax("test").with_snippet(snippet);
    assert_eq!(error.snippet, Some(snippet.to_string()));
}

#[test]
fn test_with_context() {
    let error = ParseError::syntax("test").with_context("while parsing service config");
    assert_eq!(error.context, "while parsing service config");
}

#[test]
fn test_with_location() {
    let error = ParseError::syntax("test").with_location(10, 20);
    assert_eq!(error.line, Some(10));
    assert_eq!(error.column, Some(20));
}

#[test]
fn test_builder_chain() {
    let error = ParseError::syntax("test")
        .with_path("config.yaml")
        .with_location(5, 10)
        .with_context("in service section")
        .with_snippet("service:\n  port: abc");

    assert_eq!(error.path, Some("config.yaml".to_string()));
    assert_eq!(error.line, Some(5));
    assert_eq!(error.column, Some(10));
    assert_eq!(error.context, "in service section");
    assert_eq!(error.snippet, Some("service:\n  port: abc".to_string()));
}

// ===========================================================================
// Edge Case Tests
// ===========================================================================

#[test]
fn test_empty_context() {
    let error = ParseError::syntax("test").with_context("");
    assert_eq!(error.context, "");
}

#[test]
fn test_empty_snippet() {
    let error = ParseError::syntax("test").with_snippet("");
    assert_eq!(error.snippet, Some("".to_string()));
}

#[test]
fn test_zero_line_and_column() {
    // Line 0 is unusual but should be accepted
    let error = ParseError::syntax("test").with_line(0).with_column(0);
    assert_eq!(error.line, Some(0));
    assert_eq!(error.column, Some(0));
}

#[test]
fn test_large_line_and_column() {
    let error = ParseError::syntax("test")
        .with_line(999999)
        .with_column(1000);
    assert_eq!(error.line, Some(999999));
    assert_eq!(error.column, Some(1000));
}

#[test]
fn test_multiline_snippet() {
    let snippet = "line 1\nline 2\nline 3";
    let error = ParseError::syntax("test").with_snippet(snippet);
    assert_eq!(error.snippet, Some(snippet.to_string()));
}

#[test]
fn test_snippet_with_special_characters() {
    let snippet = "key: \"value with \\n escape\"\n  tab:\tindented";
    let error = ParseError::syntax("test").with_snippet(snippet);
    assert_eq!(error.snippet, Some(snippet.to_string()));
}

#[test]
fn test_path_with_special_characters() {
    let paths = vec![
        "/path/with spaces/file.yaml",
        "/path/with-dash/file.yaml",
        "/path/with_underscore/file.yaml",
        "/path/with.dot/file.yaml",
        "C:\\Windows\\Path\\file.yaml",  // Windows path
    ];

    for path in paths {
        let error = ParseError::syntax("test").with_path(path);
        assert_eq!(error.path, Some(path.to_string()));
    }
}

// ===========================================================================
// Clone Trait Tests
// ===========================================================================

#[test]
fn test_clone_independence() {
    let original = ParseError::syntax("test")
        .with_path("config.yaml")
        .with_line(10)
        .with_column(5)
        .with_context("original context")
        .with_snippet("original snippet");

    let cloned = original.clone();

    // Verify they're equal
    assert_eq!(original, cloned);

    // Modify original
    let modified = original.with_context("modified context");

    // Clone should be unchanged
    assert_eq!(cloned.context, "original context");
    assert_eq!(modified.context, "modified context");
}

#[test]
fn test_clone_all_fields() {
    let original = ParseError::type_mismatch("port", "integer", "string")
        .with_path("config.yaml")
        .with_line(42)
        .with_column(10)
        .with_context("context")
        .with_snippet("snippet");

    let cloned = original.clone();

    assert_eq!(cloned.kind, original.kind);
    assert_eq!(cloned.line, original.line);
    assert_eq!(cloned.column, original.column);
    assert_eq!(cloned.path, original.path);
    assert_eq!(cloned.context, original.context);
    assert_eq!(cloned.snippet, original.snippet);
}

// ===========================================================================
// PartialEq Trait Tests
// ===========================================================================

#[test]
fn test_partial_equality_same_kind() {
    let error1 = ParseError::syntax("test");
    let error2 = ParseError::syntax("test");
    assert_eq!(error1, error2);
}

#[test]
fn test_partial_equality_different_kind() {
    let error1 = ParseError::syntax("test");
    let error2 = ParseError::io("test");
    assert_ne!(error1, error2);
}

#[test]
fn test_partial_equality_with_same_location() {
    let error1 = ParseError::syntax("test")
        .with_path("file.yaml")
        .with_line(10)
        .with_column(5);
    let error2 = ParseError::syntax("test")
        .with_path("file.yaml")
        .with_line(10)
        .with_column(5);
    assert_eq!(error1, error2);
}

#[test]
fn test_partial_equality_with_different_location() {
    let error1 = ParseError::syntax("test")
        .with_path("file.yaml")
        .with_line(10);
    let error2 = ParseError::syntax("test")
        .with_path("file.yaml")
        .with_line(20);
    assert_ne!(error1, error2);
}

#[test]
fn test_partial_equality_context_does_not_affect_equality() {
    let error1 = ParseError::syntax("test").with_context("context 1");
    let error2 = ParseError::syntax("test").with_context("context 2");
    // Context is not part of PartialEq
    assert_eq!(error1, error2);
}

#[test]
fn test_partial_equality_snippet_does_not_affect_equality() {
    let error1 = ParseError::syntax("test").with_snippet("snippet 1");
    let error2 = ParseError::syntax("test").with_snippet("snippet 2");
    // Snippet is not part of PartialEq
    assert_eq!(error1, error2);
}

#[test]
fn test_partial_equality_type_mismatch() {
    let error1 = ParseError::type_mismatch("port", "integer", "string");
    let error2 = ParseError::type_mismatch("port", "integer", "string");
    assert_eq!(error1, error2);
}

#[test]
fn test_partial_equality_type_mismatch_different_field() {
    let error1 = ParseError::type_mismatch("port", "integer", "string");
    let error2 = ParseError::type_mismatch("host", "integer", "string");
    assert_ne!(error1, error2);
}

// ===========================================================================
// ParseErrorKind Variant Tests
// ===========================================================================

#[test]
fn test_error_kind_syntax() {
    let error = ParseError::new(ParseErrorKind::Syntax("invalid syntax".to_string()));
    assert!(error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_kind_io() {
    let error = ParseError::new(ParseErrorKind::Io("permission denied".to_string()));
    assert!(!error.is_syntax());
    assert!(error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_kind_validation() {
    let error = ParseError::new(ParseErrorKind::Validation("value out of range".to_string()));
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_kind_type_mismatch() {
    let error = ParseError::new(ParseErrorKind::TypeMismatch {
        field: "port".to_string(),
        expected: "integer".to_string(),
        actual: "string".to_string(),
    });
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(error.is_type_mismatch());
}

#[test]
fn test_error_kind_unexpected_eof() {
    let error = ParseError::new(ParseErrorKind::UnexpectedEof);
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_kind_invalid_utf8() {
    let error = ParseError::new(ParseErrorKind::InvalidUtf8);
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_kind_unknown_anchor() {
    let error = ParseError::new(ParseErrorKind::UnknownAnchor("ref".to_string()));
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_kind_duplicate_key() {
    let error = ParseError::new(ParseErrorKind::DuplicateKey("name".to_string()));
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

#[test]
fn test_error_kind_other() {
    let error = ParseError::new(ParseErrorKind::Other("custom error".to_string()));
    assert!(!error.is_syntax());
    assert!(!error.is_io());
    assert!(!error.is_validation());
    assert!(!error.is_type_mismatch());
}

// ===========================================================================
// ErrorKind Clone and PartialEq Tests
// ===========================================================================

#[test]
fn test_error_kind_clone() {
    let kind = ParseErrorKind::TypeMismatch {
        field: "port".to_string(),
        expected: "integer".to_string(),
        actual: "string".to_string(),
    };
    let cloned = kind.clone();
    assert_eq!(kind, cloned);
}

#[test]
fn test_error_kind_partial_equality_syntax() {
    let kind1 = ParseErrorKind::Syntax("error".to_string());
    let kind2 = ParseErrorKind::Syntax("error".to_string());
    let kind3 = ParseErrorKind::Syntax("different".to_string());

    assert_eq!(kind1, kind2);
    assert_ne!(kind1, kind3);
}

#[test]
fn test_error_kind_partial_equality_type_mismatch() {
    let kind1 = ParseErrorKind::TypeMismatch {
        field: "port".to_string(),
        expected: "integer".to_string(),
        actual: "string".to_string(),
    };
    let kind2 = ParseErrorKind::TypeMismatch {
        field: "port".to_string(),
        expected: "integer".to_string(),
        actual: "string".to_string(),
    };
    let kind3 = ParseErrorKind::TypeMismatch {
        field: "host".to_string(),
        expected: "integer".to_string(),
        actual: "string".to_string(),
    };

    assert_eq!(kind1, kind2);
    assert_ne!(kind1, kind3);
}
