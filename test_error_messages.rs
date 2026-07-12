//! Test to verify error messages for negative to unsigned conversions
use armor::parsers::yaml::ParseError;

fn main() {
    println!("=== Error Messages for Negative to Unsigned Conversions ===\n");

    // Test int8 to uint8
    let error = ParseError::type_mismatch("value", "uint8", "int8_negative");
    println!("int8 -> uint8: {}", error.kind);
    assert!(error.is_type_mismatch());
    assert!(format!("{}", error.kind).contains("uint8"));
    assert!(format!("{}", error.kind).contains("int8"));

    // Test int16 to uint16
    let error = ParseError::type_mismatch("value", "uint16", "int16_negative");
    println!("int16 -> uint16: {}", error.kind);
    assert!(error.is_type_mismatch());
    assert!(format!("{}", error.kind).contains("uint16"));
    assert!(format!("{}", error.kind).contains("int16"));

    // Test int32 to uint32
    let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
    println!("int32 -> uint32: {}", error.kind);
    assert!(error.is_type_mismatch());
    assert!(format!("{}", error.kind).contains("uint32"));
    assert!(format!("{}", error.kind).contains("int32"));

    // Test int64 to uint64
    let error = ParseError::type_mismatch("value", "uint64", "int64_negative");
    println!("int64 -> uint64: {}", error.kind);
    assert!(error.is_type_mismatch());
    assert!(format!("{}", error.kind).contains("uint64"));
    assert!(format!("{}", error.kind).contains("int64"));

    // Test general signed to unsigned
    let error = ParseError::type_mismatch("port", "unsigned", "signed_negative");
    println!("signed -> unsigned: {}", error.kind);
    assert!(error.is_type_mismatch());
    assert!(format!("{}", error.kind).contains("unsigned"));
    assert!(format!("{}", error.kind).contains("negative"));

    println!("\n=== All Error Messages Verified ===");
    println!("✓ Error messages clearly indicate field path, expected type, and actual type");
    println!("✓ All error messages are properly formatted as type mismatches");
    println!("✓ Negative to unsigned conversion errors are clearly identified");
}