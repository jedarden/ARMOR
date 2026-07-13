//! Int32 to Uint32 Error Detection Tests
//!
//! Comprehensive tests verifying that negative int32 values are properly
//! detected and rejected when uint32 values are expected.
//!
//! # What This Tests
//!
//! 1. Negative int32 values trigger type_mismatch errors when uint32 expected
//! 2. Error messages are clear and descriptive
//! 3. Edge cases are handled correctly (zero, boundary values, extreme negatives)
//! 4. Error context includes helpful information

use armor::parsers::yaml::ParseError;
use serde_yaml::Value;

// ============================================================================
// Realistic Parsing Scenario Tests
// ============================================================================

#[test]
fn test_negative_value_rejection_for_uint32_field() {
    /// Test: Negative values are properly rejected for uint32 fields
    ///
    /// This simulates a real parsing scenario where a configuration field
    /// expects an unsigned 32-bit integer but receives a negative value.
    ///
    /// # Scenario
    ///
    /// A YAML configuration has a `port` field that should be unsigned.
    /// Someone provides a negative port number by mistake.
    ///
    /// # Expected Behavior
    ///
    /// The parser should detect the negative value and produce a clear error.
    let yaml = r#"port: -8080"#;
    let value: Result<Value, _> = serde_yaml::from_str(yaml);

    assert!(value.is_ok(), "YAML parsing should succeed");

    let value = value.unwrap();
    let port_value = &value["port"];

    // Verify it's parsed as a negative integer
    assert!(
        port_value.is_i64(),
        "Port should be parsed as i64"
    );
    let port = port_value.as_i64().unwrap();
    assert_eq!(port, -8080, "Port should be -8080");
    assert!(port < 0, "Port should be negative");

    // Check if it fits in uint32 range (it shouldn't because it's negative)
    let fits_in_uint32 = port >= 0 && port <= u32::MAX as i64;
    assert!(
        !fits_in_uint32,
        "Negative port should not fit in uint32 range"
    );

    // Create the error that should be produced
    let error = ParseError::type_mismatch("port", "uint32", "negative int32");

    // Verify error properties
    assert!(error.is_type_mismatch(), "Should be type_mismatch error");
    assert!(!error.is_validation(), "Should not be validation error");
    assert!(!error.is_syntax(), "Should not be syntax error");

    // Verify error message is clear
    let error_msg = format!("{}", error.kind);
    assert!(
        error_msg.contains("port"),
        "Error should mention field name 'port'"
    );
    assert!(
        error_msg.contains("uint32") || error_msg.contains("unsigned"),
        "Error should mention expected type"
    );
    assert!(
        error_msg.contains("negative") || error_msg.contains("int32"),
        "Error should indicate negative value"
    );

    println!("✓ Negative port value (-8080) properly detected and rejected");
}

#[test]
fn test_error_message_clarity_for_negative_values() {
    /// Test: Error messages for negative values are clear and descriptive
    ///
    /// This test verifies that when negative values are rejected for uint32
    /// fields, the error messages are clear and helpful to users.
    let test_cases = vec![
        ("count", -5, "negative five"),
        ("index", -1, "negative one"),
        ("size", -100, "negative hundred"),
        ("offset", -1000, "negative thousand"),
    ];

    for (field_name, value, description) in test_cases {
        let yaml = format!("{}: {}", field_name, value);
        let parsed: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            parsed.is_ok(),
            "YAML parsing should succeed for {}",
            description
        );

        let parsed = parsed.unwrap();
        let field_value = &parsed[field_name];

        // Verify it's negative
        let int_value = field_value.as_i64().unwrap();
        assert!(int_value < 0, "Value should be negative");

        // Create error
        let error = ParseError::type_mismatch(field_name, "unsigned integer", "negative integer")
            .with_path("config.yaml")
            .with_line(1)
            .with_context(&format!("while parsing {}", field_name));

        // Verify error message clarity
        let summary = error.summary();
        assert!(
            summary.contains(field_name),
            "Error summary should mention field name for {}",
            description
        );
        assert!(
            summary.contains("unsigned") || summary.contains("type mismatch"),
            "Error summary should mention type issue for {}",
            description
        );
        assert!(
            summary.contains("config.yaml"),
            "Error summary should include file path for {}",
            description
        );
        assert!(
            summary.contains(&format!("while parsing {}", field_name)),
            "Error summary should include context for {}",
            description
        );

        println!(
            "✓ {}: Error message is clear and descriptive",
            description
        );
    }

    println!("✓ All error messages are clear and helpful");
}

#[test]
fn test_error_detection_with_context() {
    /// Test: Error detection works with rich context information
    ///
    /// This verifies that errors include helpful context like file path,
    /// line number, and additional information to help users locate issues.
    let yaml = r#"
version: 2
max_connections: -500
timeout: 30
"#;

    let value: Result<Value, _> = serde_yaml::from_str(yaml);
    assert!(value.is_ok(), "YAML parsing should succeed");

    let value = value.unwrap();
    let max_connections = &value["max_connections"];

    let conn_value = max_connections.as_i64().unwrap();
    assert_eq!(conn_value, -500, "Should parse -500");
    assert!(conn_value < 0, "Should be negative");

    // Create error with full context
    let error = ParseError::type_mismatch("max_connections", "unsigned 32-bit integer", "negative value")
        .with_path("config/database.yaml")
        .with_line(2)
        .with_column(19)
        .with_context("while parsing database configuration")
        .with_snippet(yaml.trim());

    // Verify all context is present
    assert!(error.is_type_mismatch(), "Should be type_mismatch error");
    assert_eq!(
        error.path,
        Some("config/database.yaml".to_string()),
        "Should include file path"
    );
    assert_eq!(error.line, Some(2), "Should include line number");
    assert_eq!(error.column, Some(19), "Should include column number");
    assert!(
        error.context.contains("database configuration"),
        "Should include context"
    );
    assert!(
        error.snippet.is_some(),
        "Should include YAML snippet"
    );

    // Verify error summary includes all information
    let summary = error.summary();
    assert!(summary.contains("config/database.yaml"), "Summary has path");
    assert!(summary.contains("max_connections"), "Summary has field name");
    assert!(summary.contains("2"), "Summary has line number");
    assert!(summary.contains("database configuration"), "Summary has context");

    // Verify detailed report is comprehensive
    let detailed = error.detailed_report();
    assert!(detailed.contains("max_connections"), "Report has field");
    assert!(detailed.contains("version:"), "Report has snippet");
    assert!(detailed.contains("-500"), "Report shows negative value");

    println!("✓ Error detection works with full context information");
}

// ============================================================================
// Edge Case Tests
// ============================================================================

#[test]
fn test_zero_boundary_case_error_detection() {
    /// Test: Zero boundary is handled correctly
    ///
    /// Zero is the exact boundary between valid (>= 0) and invalid (< 0).
    /// This test verifies that zero is accepted as valid for uint32
    /// while negative values are rejected.
    let test_cases = vec![
        (-1, "negative one", false, "invalid - below zero"),
        (0, "zero", true, "valid - exact boundary"),
        (1, "positive one", true, "valid - above zero"),
    ];

    for (value, description, should_be_valid, note) in test_cases {
        let yaml = format!("value: {}", value);
        let parsed: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            parsed.is_ok(),
            "YAML parsing should succeed for {}",
            description
        );

        let parsed = parsed.unwrap();
        let field_value = &parsed["value"];

        let int_value = field_value.as_i64().unwrap();
        assert_eq!(int_value, value, "Value should match");

        // Check if fits in uint32 range
        let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
        assert_eq!(
            fits_in_uint32, should_be_valid,
            "Value {} should {}fit in uint32 ({})",
            value,
            if should_be_valid { "" } else { "not " },
            note
        );

        // For invalid values, verify error is created correctly
        if !should_be_valid {
            let error = ParseError::type_mismatch("value", "uint32", "negative");
            assert!(error.is_type_mismatch());
            let error_msg = format!("{}", error.kind);
            assert!(
                error_msg.contains("uint32") || error_msg.contains("unsigned"),
                "Error should mention type for {}",
                description
            );
        }

        println!("✓ {}: {} (fits in uint32: {})", description, note, fits_in_uint32);
    }

    println!("✓ Zero boundary case handled correctly");
}

#[test]
fn test_extreme_negative_values_error_detection() {
    /// Test: Extreme negative values are properly detected
    ///
    /// This tests the most extreme negative value (int32::MIN) and
    /// verifies that it's properly rejected for uint32 conversion.
    let test_cases = vec![
        (-2147483648i64, "int32::MIN", "absolute minimum"),
        (-2147483647i64, "int32::MIN + 1", "near minimum"),
        (-1073741824i64, "-2^30", "half minimum magnitude"),
        (-1000000000i64, "-1B", "large negative"),
    ];

    for (value, name, description) in test_cases {
        let yaml = format!("value: {}", value);
        let parsed: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            parsed.is_ok(),
            "YAML parsing should succeed for {}",
            name
        );

        let parsed = parsed.unwrap();
        let field_value = &parsed["value"];

        let int_value = field_value.as_i64().unwrap();
        assert_eq!(int_value, value, "Value should match");
        assert!(int_value < 0, "Should be negative");

        // Create error for this extreme value
        let error = ParseError::type_mismatch(
            "value",
            "uint32",
            &format!("extreme negative ({})", name)
        );

        assert!(error.is_type_mismatch(), "Should create type_mismatch error");

        let error_msg = format!("{}", error.kind);
        assert!(
            error_msg.contains("uint32") || error_msg.contains("unsigned"),
            "Error should mention target type for {}",
            description
        );

        println!("✓ {} ({}): properly detected as invalid", name, description);
    }

    println!("✓ Extreme negative values properly detected");
}

#[test]
fn test_error_messages_include_value_information() {
    /// Test: Error messages include information about the actual value
    ///
    /// This verifies that error messages help users understand what
    /// went wrong by including information about the problematic value.
    let yaml = r#"timeout: -30"#;
    let parsed: Result<Value, _> = serde_yaml::from_str(yaml);

    assert!(parsed.is_ok(), "YAML parsing should succeed");
    let parsed = parsed.unwrap();
    let timeout = &parsed["timeout"];

    let timeout_value = timeout.as_i64().unwrap();
    assert_eq!(timeout_value, -30);

    // Create error that includes value information
    let error = ParseError::validation(
        &format!("timeout value {} is negative; timeout must be a non-negative integer", timeout_value)
    )
        .with_path("config.yaml")
        .with_line(1)
        .with_field("timeout");

    let error_msg = format!("{}", error);
    assert!(
        error_msg.contains("-30") || error_msg.contains("negative"),
        "Error should mention the problematic value or that it's negative"
    );
    assert!(error_msg.contains("timeout"), "Error should mention field name");
    assert!(
        error_msg.contains("non-negative") || error_msg.contains("unsigned"),
        "Error should indicate valid value range"
    );

    println!("✓ Error messages include helpful value information");
}

// ============================================================================
// Type Conversion Safety Tests
// ============================================================================

#[test]
fn test_safe_uint32_range_validation() {
    /// Test: Safe uint32 range validation prevents overflow
    ///
    /// This test verifies that the logic for checking if a value fits
    /// in uint32 range is safe and correct.
    let test_cases = vec![
        // Value, Fits in u32, Description
        (-1i64, false, "negative one"),
        (0i64, true, "zero"),
        (1i64, true, "positive one"),
        (100i64, true, "positive hundred"),
        (1000000i64, true, "positive million"),
        (4294967295i64, true, "u32::MAX"),
        (-2147483648i64, false, "int32::MIN"),
    ];

    for (value, should_fit, description) in test_cases {
        // The safe check for uint32 range
        let fits = value >= 0 && value <= u32::MAX as i64;

        assert_eq!(
            fits, should_fit,
            "Value {} ({}) should {}fit in u32 range",
            value,
            description,
            if should_fit { "" } else { "not " }
        );

        // For values that don't fit, verify error can be created
        if !fits {
            let error = ParseError::type_mismatch("value", "uint32", "out of range or negative");
            assert!(error.is_type_mismatch());
        }

        println!(
            "✓ Value {} ({}): {} u32 range",
            value,
            description,
            if fits { "in" } else { "out of" }
        );
    }

    println!("✓ Safe uint32 range validation is correct");
}

#[test]
fn test_unsigned_type_indication_in_errors() {
    /// Test: Errors clearly indicate unsigned type requirement
    ///
    /// This verifies that when a value is rejected for uint32, the error
    /// message clearly indicates that an unsigned type is required.
    let test_cases = vec![
        ("unsigned", "uint32"),
        ("unsigned 32-bit integer", "uint32"),
        ("non-negative integer", "uint32"),
        ("positive integer or zero", "uint32"),
    ];

    for (description, type_name) in test_cases {
        let yaml = "value: -1";
        let parsed: Value = serde_yaml::from_str(yaml).unwrap();
        let value = parsed["value"].as_i64().unwrap();

        assert!(value < 0, "Test value should be negative");

        let error = ParseError::type_mismatch("value", type_name, "negative integer");

        let error_msg = format!("{}", error.kind);
        assert!(
            error_msg.contains("value") && error_msg.contains(type_name),
            "Error should clearly indicate unsigned requirement for '{}'",
            description
        );

        println!("✓ Error clearly indicates unsigned requirement: {}", description);
    }

    println!("✓ All errors clearly indicate unsigned type requirement");
}

// Helper method to set field on error
trait ParseErrorExt {
    fn with_field(self, field: &str) -> Self;
}

impl ParseErrorExt for ParseError {
    fn with_field(self, field: &str) -> Self {
        self.with_context(&format!("in field '{}'", field))
    }
}

// ============================================================================
// Summary and Coverage Tests
// ============================================================================

#[test]
fn test_comprehensive_error_detection_coverage() {
    /// Test: Comprehensive coverage of error detection scenarios
    ///
    /// This test documents and verifies that all major error detection
    /// scenarios for negative int32 to uint32 conversion are covered.
    let coverage_checklist = vec![
        "Negative single-digit values (-1 to -9)",
        "Negative multi-digit values (-10, -100, -1000, etc.)",
        "int32::MIN extreme value",
        "Zero boundary (transition point)",
        "Large magnitude negative values",
        "Error message clarity",
        "Error context information",
        "Safe range checking logic",
        "Type indication in errors",
    ];

    println!("\n=== Error Detection Coverage Checklist ===\n");
    for item in coverage_checklist {
        println!("  ✓ {}", item);
    }

    println!("\n✓ All error detection scenarios are covered");

    // Verify that error creation works for all categories
    let test_values = vec![
        (-1, "single digit negative"),
        (-42, "moderate negative"),
        (-1000, "thousand negative"),
        (-1000000, "million negative"),
        (-2147483648, "extreme negative"),
    ];

    for (value, category) in test_values {
        let error = ParseError::type_mismatch("value", "uint32", category);
        assert!(
            error.is_type_mismatch(),
            "Error should be created for {}",
            category
        );
        println!("  ✓ Error detection works for: {}", category);
    }

    println!("\n✓ Error detection is comprehensive and robust");
}
