//! Test to verify error messages for negative to unsigned conversions
use armor::parsers::yaml::ParseError;

#[test]
fn test_int8_to_uint8_error_message() {
    let error = ParseError::type_mismatch("value", "uint8", "int8_negative");

    assert!(error.is_type_mismatch());
    let error_str = format!("{}", error.kind);
    assert!(error_str.contains("uint8"));
    assert!(error_str.contains("int8"));
}

#[test]
fn test_int16_to_uint16_error_message() {
    let error = ParseError::type_mismatch("value", "uint16", "int16_negative");

    assert!(error.is_type_mismatch());
    let error_str = format!("{}", error.kind);
    assert!(error_str.contains("uint16"));
    assert!(error_str.contains("int16"));
}

#[test]
fn test_int32_to_uint32_error_message() {
    let error = ParseError::type_mismatch("value", "uint32", "int32_negative");

    assert!(error.is_type_mismatch());
    let error_str = format!("{}", error.kind);
    assert!(error_str.contains("uint32"));
    assert!(error_str.contains("int32"));
}

#[test]
fn test_int64_to_uint64_error_message() {
    let error = ParseError::type_mismatch("value", "uint64", "int64_negative");

    assert!(error.is_type_mismatch());
    let error_str = format!("{}", error.kind);
    assert!(error_str.contains("uint64"));
    assert!(error_str.contains("int64"));
}

#[test]
fn test_signed_to_unsigned_error_message() {
    let error = ParseError::type_mismatch("port", "unsigned", "signed_negative");

    assert!(error.is_type_mismatch());
    let error_str = format!("{}", error.kind);
    assert!(error_str.contains("unsigned"));
    assert!(error_str.contains("negative"));
}
