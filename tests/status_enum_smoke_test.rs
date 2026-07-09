//! Smoke test for Status enum
//!
//! This test verifies that the Status enum is properly defined and functional.

use armor::parsers::yaml::Status;

#[test]
fn test_status_enum_exists() {
    // Test SUCCESS variant
    let success = Status::SUCCESS;
    assert!(success.is_success());
    assert!(!success.is_error());

    // Test ERROR variant
    let error = Status::ERROR;
    assert!(error.is_error());
    assert!(!error.is_success());
}

#[test]
fn test_status_from_bool() {
    // Test from_bool conversion
    let success = Status::from_bool(true);
    assert_eq!(success, Status::SUCCESS);

    let error = Status::from_bool(false);
    assert_eq!(error, Status::ERROR);
}

#[test]
fn test_status_as_bool() {
    // Test as_bool conversion
    assert_eq!(Status::SUCCESS.as_bool(), true);
    assert_eq!(Status::ERROR.as_bool(), false);
}

#[test]
fn test_status_display() {
    // Test Display trait
    assert_eq!(format!("{}", Status::SUCCESS), "SUCCESS");
    assert_eq!(format!("{}", Status::ERROR), "ERROR");
}

#[test]
fn test_status_equality() {
    // Test PartialEq
    assert_eq!(Status::SUCCESS, Status::SUCCESS);
    assert_eq!(Status::ERROR, Status::ERROR);
    assert_ne!(Status::SUCCESS, Status::ERROR);
}
