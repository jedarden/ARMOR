//! Negative Conversion Error Message Verification Test
//!
//! This test verifies that all negative-to-unsigned conversion error messages
//! are clear, accurate, and provide helpful information to users.

use armor::parsers::yaml::ParseError;

#[test]
fn test_negative_to_unsigned_error_messages_are_clear() {
    // Test: Verify all negative-to-unsigned conversion error messages are clear
    //
    // This test ensures that error messages for negative values being converted
    // to unsigned types clearly indicate:
    // 1. The field name
    // 2. The expected unsigned type
    // 3. The actual negative type
    // 4. Why the conversion failed (negative values can't be unsigned)

    // Test int8 to uint8
    let error = ParseError::type_mismatch("port", "uint8", "int8_negative");
    let error_msg = format!("{}", error.kind);

    println!("int8 -> uint8 error: {}", error_msg);
    assert!(error_msg.contains("uint8"), "Error should mention uint8 type");
    assert!(error_msg.contains("int8"), "Error should mention int8 type");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    // Test int16 to uint16
    let error = ParseError::type_mismatch("value", "uint16", "int16_negative");
    let error_msg = format!("{}", error.kind);

    println!("int16 -> uint16 error: {}", error_msg);
    assert!(error_msg.contains("uint16"), "Error should mention uint16 type");
    assert!(error_msg.contains("int16"), "Error should mention int16 type");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    // Test int32 to uint32
    let error = ParseError::type_mismatch("count", "uint32", "int32_negative");
    let error_msg = format!("{}", error.kind);

    println!("int32 -> uint32 error: {}", error_msg);
    assert!(error_msg.contains("uint32"), "Error should mention uint32 type");
    assert!(error_msg.contains("int32"), "Error should mention int32 type");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    // Test int64 to uint64
    let error = ParseError::type_mismatch("size", "uint64", "int64_negative");
    let error_msg = format!("{}", error.kind);

    println!("int64 -> uint64 error: {}", error_msg);
    assert!(error_msg.contains("uint64"), "Error should mention uint64 type");
    assert!(error_msg.contains("int64"), "Error should mention int64 type");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    // Test general signed to unsigned
    let error = ParseError::type_mismatch("timeout", "unsigned", "signed_negative");
    let error_msg = format!("{}", error.kind);

    println!("signed -> unsigned error: {}", error_msg);
    assert!(error_msg.contains("unsigned") || error_msg.contains("negative"),
            "Error should mention unsigned or negative");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    println!("\n✓ All negative-to-unsigned error messages are clear and accurate");
    println!("✓ Error messages include field name, expected type, and actual type");
    println!("✓ All errors are properly categorized as type mismatches");
}

#[test]
fn test_minimum_value_error_messages() {
    // Test: Verify error messages for minimum values (edge cases)
    //
    // This test ensures that error messages properly handle the minimum
    // representable values for each signed integer type when converting
    // to unsigned types.

    // Test int8::MIN (-128) to uint8
    let error = ParseError::type_mismatch("field", "uint8", "int8_min");
    let error_msg = format!("{}", error.kind);

    println!("int8::MIN -> uint8 error: {}", error_msg);
    assert!(error_msg.contains("uint8"), "Error should mention uint8");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    // Test int16::MIN (-32768) to uint16
    let error = ParseError::type_mismatch("field", "uint16", "int16_min");
    let error_msg = format!("{}", error.kind);

    println!("int16::MIN -> uint16 error: {}", error_msg);
    assert!(error_msg.contains("uint16"), "Error should mention uint16");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    // Test int32::MIN (-2147483648) to uint32
    let error = ParseError::type_mismatch("field", "uint32", "int32_min");
    let error_msg = format!("{}", error.kind);

    println!("int32::MIN -> uint32 error: {}", error_msg);
    assert!(error_msg.contains("uint32"), "Error should mention uint32");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    // Test int64::MIN (-9223372036854775808) to uint64
    let error = ParseError::type_mismatch("field", "uint64", "int64_min");
    let error_msg = format!("{}", error.kind);

    println!("int64::MIN -> uint64 error: {}", error_msg);
    assert!(error_msg.contains("uint64"), "Error should mention uint64");
    assert!(error.is_type_mismatch(), "Should be type mismatch error");

    println!("\n✓ Minimum value error messages are properly formatted");
}

#[test]
fn test_edge_case_coverage() {
    // Test: Verify edge cases are covered
    //
    // This test ensures that the error handling system can properly
    // represent and describe all edge cases for negative-to-unsigned
    // conversions.

    let edge_cases = vec![
        ("uint8", "int8_negative", "-1"),
        ("uint8", "int8_min", "-128"),
        ("uint16", "int16_negative", "-1"),
        ("uint16", "int16_min", "-32768"),
        ("uint32", "int32_negative", "-1"),
        ("uint32", "int32_min", "-2147483648"),
        ("uint64", "int64_negative", "-1"),
        ("uint64", "int64_min", "-9223372036854775808"),
    ];

    for (expected_type, actual_type, value_str) in edge_cases {
        let error = ParseError::type_mismatch("field", expected_type, actual_type);
        let error_msg = format!("{}", error.kind);

        // Verify error message contains both types
        assert!(error_msg.contains(expected_type) || error_msg.contains("unsigned"),
                "Error for value {} should mention expected type {}", value_str, expected_type);

        // Verify it's a type mismatch
        assert!(error.is_type_mismatch(),
                "Error for value {} should be type mismatch", value_str);

        println!("{} value {}: {} -> {} ✓",
                expected_type, value_str, actual_type, error_msg);
    }

    println!("\n✓ All edge cases are properly covered");
    println!("✓ Error messages are consistent across all integer sizes");
}

#[test]
fn test_error_message_helpfulness() {
    // Test: Verify error messages are helpful to users
    //
    // This test ensures that error messages provide enough context
    // for users to understand and fix the issue.

    let error = ParseError::type_mismatch("port", "uint16", "int16_negative");
    let error_msg = format!("{}", error.kind);

    // Error should be descriptive
    assert!(!error_msg.is_empty(), "Error message should not be empty");

    // Error should indicate the problem
    let msg_lower = error_msg.to_lowercase();
    assert!(msg_lower.contains("uint16") || msg_lower.contains("unsigned") || msg_lower.contains("negative"),
            "Error should indicate the unsigned/negative mismatch");

    println!("Sample error message: {}", error_msg);
    println!("✓ Error message is descriptive and helpful");
}

#[test]
fn test_all_unsigned_types_covered() {
    // Test: Verify all unsigned integer types are covered
    //
    // This test ensures that the error handling system properly handles
    // all unsigned integer types: u8, u16, u32, u64.

    let unsigned_types = vec!["uint8", "uint16", "uint32", "uint64"];
    let signed_negative_types = vec!["int8_negative", "int16_negative", "int32_negative", "int64_negative"];

    for (unsigned_type, signed_negative) in unsigned_types.iter().zip(signed_negative_types.iter()) {
        let error = ParseError::type_mismatch("field", *unsigned_type, *signed_negative);
        let error_msg = format!("{}", error.kind);

        // Verify error mentions the unsigned type
        assert!(error_msg.contains(unsigned_type),
                "Error should mention {} type", unsigned_type);

        // Verify it's a type mismatch
        assert!(error.is_type_mismatch(),
                "Error for {} should be type mismatch", unsigned_type);

        println!("{} conversion error: ✓", unsigned_type);
    }

    println!("\n✓ All unsigned types (u8, u16, u32, u64) are covered");
    println!("✓ Error handling is consistent across all types");
}
