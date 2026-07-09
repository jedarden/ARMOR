//! Tests for the OperationResult dataclass with status, data, and error fields

use armor::parsers::yaml::{OperationResult, Status};

#[test]
fn test_operation_result_has_all_three_fields() {
    // Test that OperationResult can be instantiated with all three values
    let result: OperationResult<String> = OperationResult::new(
        Status::SUCCESS,
        Some("test data".to_string()),
        None,
    );

    assert_eq!(result.status, Status::SUCCESS);
    assert_eq!(result.data, Some("test data".to_string()));
    assert_eq!(result.error, None);
}

#[test]
fn test_operation_result_fields_are_properly_typed() {
    // Test with different data types to ensure generic typing works
    let string_result: OperationResult<String> = OperationResult::success("hello".to_string());
    assert!(string_result.is_success());
    assert_eq!(string_result.get_data(), Some(&"hello".to_string()));

    let num_result: OperationResult<i32> = OperationResult::success(42);
    assert!(num_result.is_success());
    assert_eq!(num_result.get_data(), Some(&42));

    let vec_result: OperationResult<Vec<i32>> = OperationResult::success(vec![1, 2, 3]);
    assert!(vec_result.is_success());
    assert_eq!(vec_result.get_data(), Some(&vec![1, 2, 3]));
}

#[test]
fn test_operation_result_success_constructor() {
    let result: OperationResult<String> = OperationResult::success("data".to_string());

    assert_eq!(result.status, Status::SUCCESS);
    assert_eq!(result.data, Some("data".to_string()));
    assert_eq!(result.error, None);
    assert!(result.is_success());
    assert!(!result.is_error());
}

#[test]
fn test_operation_result_error_constructor() {
    let result: OperationResult<String> = OperationResult::error("Parse error".to_string());

    assert_eq!(result.status, Status::ERROR);
    assert_eq!(result.data, None);
    assert_eq!(result.error, Some("Parse error".to_string()));
    assert!(!result.is_success());
    assert!(result.is_error());
}

#[test]
fn test_operation_result_get_data() {
    let success_result: OperationResult<String> = OperationResult::success("test".to_string());
    assert_eq!(success_result.get_data(), Some(&"test".to_string()));

    let error_result: OperationResult<String> = OperationResult::error("failed".to_string());
    assert_eq!(error_result.get_data(), None);
}

#[test]
fn test_operation_result_get_error() {
    let error_result: OperationResult<String> = OperationResult::error("error message".to_string());
    assert_eq!(error_result.get_error(), Some("error message"));

    let success_result: OperationResult<String> = OperationResult::success("data".to_string());
    assert_eq!(success_result.get_error(), None);
}

#[test]
fn test_operation_result_with_explicit_new() {
    // Test creating OperationResult with all three fields explicitly set
    let result: OperationResult<i32> = OperationResult::new(
        Status::ERROR,
        None,
        Some("Invalid input".to_string()),
    );

    assert_eq!(result.status, Status::ERROR);
    assert_eq!(result.data, None);
    assert_eq!(result.error, Some("Invalid input".to_string()));
}
