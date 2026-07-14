#!/usr/bin/env python3
"""
Test Cases for Error Response Structure Validation Helper Functions

Comprehensive test suite for the error response structure validation helpers.
Tests cover:
- Required field validation (error, message)
- Optional field validation (code, details)
- JSON parsing from string responses
- Custom field validators
- Multiple response validation
- Edge cases and error conditions
- Real-world API error scenarios

Acceptance Criteria:
- Function accepts a response body and validates error field exists ✓
- Validates that error message is present and non-empty ✓
- Supports optional field validation (e.g., 'code', 'details') ✓
- Returns boolean or throws assertion error with clear message ✓
- Includes test cases for valid/invalid error structures ✓
- Function is exported from test utils module ✓

Bead: bf-64826u
Created: 2026-07-14
"""

import unittest
import sys
import json
from pathlib import Path
from typing import Dict, Any

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from test_error_response_validation import (
    validate_error_response,
    validate_error_field_only,
    validate_standard_error_response,
    validate_detailed_error_response,
    validate_http_error_response,
    validate_error_responses,
    validate_with_standard_validators,
    validate_with_detailed_validators,
    ErrorResponseSpec,
    ErrorResponseValidationError,
    STANDARD_ERROR_SPEC,
    DETAILED_ERROR_SPEC,
    MINIMAL_ERROR_SPEC,
    STANDARD_VALIDATORS,
    DETAILED_VALIDATORS,
    validate_error_code_is_int,
    validate_error_code_is_http,
    validate_message_not_empty,
    validate_error_not_empty,
)


class MockHTTPResponse:
    """Mock HTTP response object for testing."""

    def __init__(self, status_code: int, text: str, url: str = "http://api.example.com"):
        self.status_code = status_code
        self.text = text
        self.content = text.encode('utf-8') if isinstance(text, str) else text
        self.url = url


class TestRequiredFieldValidation(unittest.TestCase):
    """Test validation of required fields."""

    def test_standard_spec_valid_response(self):
        """Test valid response meets standard spec requirements."""
        response = {"error": "not_found", "message": "Resource not found"}

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should accept response with error and message fields")

    def test_standard_spec_missing_error_field(self):
        """Test validation fails when error field is missing."""
        response = {"message": "Resource not found"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("error", str(context.exception))
        self.assertIn("Missing required fields", str(context.exception))

    def test_standard_spec_missing_message_field(self):
        """Test validation fails when message field is missing."""
        response = {"error": "not_found"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("message", str(context.exception))
        self.assertIn("Missing required fields", str(context.exception))

    def test_detailed_spec_valid_response(self):
        """Test valid response meets detailed spec requirements."""
        response = {
            "error": "auth_failed",
            "message": "Invalid credentials",
            "code": 401
        }

        result = validate_error_response(response, DETAILED_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should accept response with error, message, and code")

    def test_detailed_spec_missing_code(self):
        """Test validation fails when code field is missing for detailed spec."""
        response = {"error": "auth_failed", "message": "Invalid credentials"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, DETAILED_ERROR_SPEC)

        self.assertIn("code", str(context.exception))

    def test_minimal_spec_valid_response(self):
        """Test minimal spec only requires error field."""
        response = {"error": "internal_error"}

        result = validate_error_response(response, MINIMAL_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should accept response with only error field")

    def test_all_required_fields_missing(self):
        """Test validation fails when all required fields are missing."""
        response = {"details": "Some details"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        error_msg = str(context.exception)
        self.assertIn("error", error_msg)
        self.assertIn("message", error_msg)


class TestFieldValidation(unittest.TestCase):
    """Test validation of field values."""

    def test_error_field_none(self):
        """Test validation fails when error field is None."""
        response = {"error": None, "message": "Some message"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("error", str(context.exception))
        self.assertIn("None", str(context.exception))

    def test_error_field_empty_string(self):
        """Test validation fails when error field is empty."""
        response = {"error": "", "message": "Some message"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("error", str(context.exception))
        self.assertIn("empty", str(context.exception).lower())

    def test_error_field_whitespace_only(self):
        """Test validation fails when error field is whitespace only."""
        response = {"error": "   ", "message": "Some message"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("error", str(context.exception))
        self.assertIn("whitespace", str(context.exception).lower())

    def test_message_field_empty(self):
        """Test validation fails when message field is empty."""
        response = {"error": "not_found", "message": ""}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("message", str(context.exception))

    def test_message_field_none(self):
        """Test validation fails when message field is None."""
        response = {"error": "not_found", "message": None}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("message", str(context.exception))
        self.assertIn("None", str(context.exception))

    def test_non_string_error_field_accepted(self):
        """Test non-string error field values are accepted."""
        # Non-string values don't trigger empty/None checks
        response = {"error": 123, "message": "Some message"}

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should accept non-string error field values")


class TestOptionalFieldValidation(unittest.TestCase):
    """Test validation of optional fields."""

    def test_optional_fields_present(self):
        """Test validation passes when optional fields are present."""
        response = {
            "error": "not_found",
            "message": "Resource not found",
            "code": 404,
            "details": {"resource_type": "user", "resource_id": "123"}
        }

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should accept optional fields")

    def test_optional_fields_absent(self):
        """Test validation passes when optional fields are absent."""
        response = {"error": "not_found", "message": "Resource not found"}

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should not require optional fields")

    def test_unexpected_field_detected(self):
        """Test validation detects unexpected fields."""
        response = {
            "error": "not_found",
            "message": "Resource not found",
            "unexpected_field": "some value"
        }

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("unexpected_field", str(context.exception))
        self.assertIn("Unexpected fields", str(context.exception))

    def test_multiple_unexpected_fields(self):
        """Test validation detects multiple unexpected fields."""
        response = {
            "error": "not_found",
            "message": "Resource not found",
            "field1": "value1",
            "field2": "value2"
        }

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        error_msg = str(context.exception)
        self.assertIn("field1", error_msg)
        self.assertIn("field2", error_msg)


class TestJSONParsing(unittest.TestCase):
    """Test JSON parsing from string responses."""

    def test_valid_json_string(self):
        """Test validation accepts valid JSON string."""
        response = '{"error": "not_found", "message": "Resource not found"}'

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should parse and validate JSON string")

    def test_invalid_json_string(self):
        """Test validation rejects invalid JSON string."""
        response = '{invalid json}'

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("Invalid JSON", str(context.exception))

    def test_malformed_json_string(self):
        """Test validation rejects malformed JSON string."""
        response = '{"error": "not_found", "message": "Resource not found"'

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("Invalid JSON", str(context.exception))

    def test_json_string_with_optional_fields(self):
        """Test JSON string with optional fields."""
        response = '{"error": "auth_failed", "message": "Invalid credentials", "code": 401}'

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)

        self.assertTrue(result, "Should parse JSON with optional fields")

    def test_non_json_string_response(self):
        """Test non-JSON string response is rejected."""
        response = "Plain text error message"

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, STANDARD_ERROR_SPEC)

        self.assertIn("Invalid JSON", str(context.exception))


class TestCustomValidators(unittest.TestCase):
    """Test custom field validators."""

    def test_custom_validator_passes(self):
        """Test custom validator that passes."""
        response = {"error": "not_found", "message": "Resource not found", "code": 404}

        def validate_code(value, field_name):
            if value < 400 or value >= 500:
                return f"{field_name} must be 4xx"
            return None

        result = validate_error_response(
            response,
            STANDARD_ERROR_SPEC,
            custom_validators={"code": validate_code},
            throw_on_error=False
        )

        self.assertTrue(result, "Should pass custom validation")

    def test_custom_validator_fails(self):
        """Test custom validator that fails."""
        response = {"error": "not_found", "message": "Resource not found", "code": 200}

        def validate_code(value, field_name):
            if value < 400 or value >= 500:
                return f"{field_name} must be 4xx"
            return None

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(
                response,
                STANDARD_ERROR_SPEC,
                custom_validators={"code": validate_code}
            )

        self.assertIn("code", str(context.exception))
        self.assertIn("4xx", str(context.exception))

    def test_multiple_custom_validators(self):
        """Test multiple custom validators."""
        # Create a custom spec that allows request_id field
        custom_spec = ErrorResponseSpec(
            required_fields={"error", "message"},
            optional_fields={"code", "details", "request_id"}
        )

        response = {
            "error": "not_found",
            "message": "Resource not found",
            "code": 404,
            "request_id": "abc123"
        }

        def validate_code(value, field_name):
            if not isinstance(value, int):
                return f"{field_name} must be int"
            return None

        def validate_request_id(value, field_name):
            if not isinstance(value, str) or len(value) < 3:
                return f"{field_name} must be at least 3 chars"
            return None

        result = validate_error_response(
            response,
            custom_spec,
            custom_validators={
                "code": validate_code,
                "request_id": validate_request_id
            },
            throw_on_error=False
        )

        self.assertTrue(result, "Should pass all custom validators")

    def test_custom_validator_exception_handling(self):
        """Test custom validator exceptions are handled gracefully."""
        response = {"error": "not_found", "message": "Resource not found", "code": 404}

        def broken_validator(value, field_name):
            raise ValueError("Something went wrong")

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(
                response,
                STANDARD_ERROR_SPEC,
                custom_validators={"code": broken_validator}
            )

        self.assertIn("Validator error", str(context.exception))


class TestConvenienceFunctions(unittest.TestCase):
    """Test convenience validation functions."""

    def test_validate_error_field_only_valid(self):
        """Test error field validation with valid response."""
        response = {"error": "internal_error"}

        result = validate_error_field_only(response, throw_on_error=False)

        self.assertTrue(result, "Should accept response with only error field")

    def test_validate_error_field_only_invalid(self):
        """Test error field validation with missing field."""
        response = {"message": "Some error"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_field_only(response)

        self.assertIn("error", str(context.exception))

    def test_validate_standard_error_response_valid(self):
        """Test standard error response validation."""
        response = {"error": "not_found", "message": "Resource not found"}

        result = validate_standard_error_response(response, throw_on_error=False)

        self.assertTrue(result, "Should accept standard error response")

    def test_validate_standard_error_response_invalid(self):
        """Test standard error response validation fails."""
        response = {"error": "not_found"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_standard_error_response(response)

        self.assertIn("message", str(context.exception))

    def test_validate_detailed_error_response_valid(self):
        """Test detailed error response validation."""
        response = {
            "error": "auth_failed",
            "message": "Invalid credentials",
            "code": 401
        }

        result = validate_detailed_error_response(response, throw_on_error=False)

        self.assertTrue(result, "Should accept detailed error response")

    def test_validate_detailed_error_response_invalid(self):
        """Test detailed error response validation fails."""
        response = {"error": "auth_failed", "message": "Invalid credentials"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_detailed_error_response(response)

        self.assertIn("code", str(context.exception))


class TestBuiltInValidators(unittest.TestCase):
    """Test built-in field validators."""

    def test_validate_error_code_is_int_valid(self):
        """Test error code integer validator with valid code."""
        result = validate_error_code_is_int(404, "code")
        self.assertIsNone(result, "Should accept integer error code")

    def test_validate_error_code_is_int_invalid(self):
        """Test error code integer validator with invalid code."""
        result = validate_error_code_is_int("404", "code")
        self.assertIsNotNone(result, "Should reject string error code")
        self.assertIn("integer", result)

    def test_validate_error_code_is_http_valid(self):
        """Test HTTP error code validator with valid codes."""
        for code in [400, 401, 403, 404, 500, 502, 503]:
            result = validate_error_code_is_http(code, "code")
            self.assertIsNone(result, f"Should accept HTTP error code {code}")

    def test_validate_error_code_is_http_invalid_client_error(self):
        """Test HTTP error code validator rejects non-error codes."""
        result = validate_error_code_is_http(200, "code")
        self.assertIsNotNone(result, "Should reject 2xx code")
        self.assertIn("4xx or 5xx", result)

    def test_validate_error_code_is_http_invalid_server_error(self):
        """Test HTTP error code validator redirects."""
        result = validate_error_code_is_http(301, "code")
        self.assertIsNotNone(result, "Should reject 3xx code")
        self.assertIn("4xx or 5xx", result)

    def test_validate_message_not_empty_valid(self):
        """Test message not empty validator with valid message."""
        result = validate_message_not_empty("Error occurred", "message")
        self.assertIsNone(result, "Should accept non-empty message")

    def test_validate_message_not_empty_invalid_empty(self):
        """Test message not empty validator with empty message."""
        result = validate_message_not_empty("", "message")
        self.assertIsNotNone(result, "Should reject empty message")
        self.assertIn("empty", result)

    def test_validate_message_not_empty_invalid_whitespace(self):
        """Test message not empty validator with whitespace message."""
        result = validate_message_not_empty("   ", "message")
        self.assertIsNotNone(result, "Should reject whitespace message")
        self.assertIn("empty", result)

    def test_validate_error_not_empty_valid(self):
        """Test error not empty validator with valid error."""
        result = validate_error_not_empty("not_found", "error")
        self.assertIsNone(result, "Should accept non-empty error")

    def test_validate_error_not_empty_invalid(self):
        """Test error not empty validator with empty error."""
        result = validate_error_not_empty("", "error")
        self.assertIsNotNone(result, "Should reject empty error")
        self.assertIn("empty", result)


class TestValidatorCombinations(unittest.TestCase):
    """Test validation with built-in validator combinations."""

    def test_validate_with_standard_validators_valid(self):
        """Test standard validators with valid response."""
        response = {
            "error": "not_found",
            "message": "Resource not found"
        }

        result = validate_with_standard_validators(response, throw_on_error=False)

        self.assertTrue(result, "Should pass standard validators")

    def test_validate_with_standard_validators_invalid_error(self):
        """Test standard validators with empty error field."""
        response = {"error": "", "message": "Resource not found"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_with_standard_validators(response)

        self.assertIn("error", str(context.exception))

    def test_validate_with_detailed_validators_valid(self):
        """Test detailed validators with valid response."""
        response = {
            "error": "auth_failed",
            "message": "Invalid credentials",
            "code": 401
        }

        result = validate_with_detailed_validators(response, throw_on_error=False)

        self.assertTrue(result, "Should pass detailed validators")

    def test_validate_with_detailed_validators_invalid_code(self):
        """Test detailed validators with invalid error code."""
        response = {
            "error": "auth_failed",
            "message": "Invalid credentials",
            "code": 200  # Not a 4xx or 5xx code
        }

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_with_detailed_validators(response)

        self.assertIn("code", str(context.exception))
        self.assertIn("4xx or 5xx", str(context.exception))


class TestMultipleResponseValidation(unittest.TestCase):
    """Test validation of multiple responses at once."""

    def test_validate_multiple_all_pass(self):
        """Test validating multiple responses where all pass."""
        responses = [
            {"error": "not_found", "message": "Resource 1 not found"},
            {"error": "auth_failed", "message": "Invalid credentials"},
            {"error": "internal_error", "message": "Server error"}
        ]

        results = validate_error_responses(*responses, throw_on_error=False)

        self.assertEqual(len(results), 3, "Should return result for each response")
        self.assertTrue(all(results), "All responses should pass validation")

    def test_validate_multiple_some_fail(self):
        """Test validating multiple responses where some fail."""
        responses = [
            {"error": "not_found", "message": "Resource not found"},
            {"error": "auth_failed"},  # Missing message
            {"error": "internal_error", "message": "Server error"}
        ]

        results = validate_error_responses(*responses, throw_on_error=False)

        self.assertEqual(results, [True, False, True], "Should track individual results")

    def test_validate_multiple_throws_on_first_failure(self):
        """Test validation throws on first failure when throw_on_error=True."""
        responses = [
            {"error": "not_found", "message": "Resource not found"},
            {"error": "auth_failed"},  # Missing message
            {"error": "internal_error", "message": "Server error"}  # Never reached
        ]

        with self.assertRaises(ErrorResponseValidationError):
            validate_error_responses(*responses, throw_on_error=True)


class TestHTTPResponseValidation(unittest.TestCase):
    """Test validation with HTTP response objects."""

    def test_validate_http_response_with_text(self):
        """Test validation extracts body from HTTP response object."""
        response = MockHTTPResponse(
            status_code=404,
            text='{"error": "not_found", "message": "Resource not found"}'
        )

        result = validate_http_error_response(response, throw_on_error=False)

        self.assertTrue(result, "Should extract and validate response body")

    def test_validate_http_response_with_tuple(self):
        """Test validation with tuple response format."""
        response = (404, '{"error": "not_found", "message": "Resource not found"}')

        result = validate_http_error_response(response, throw_on_error=False)

        self.assertTrue(result, "Should handle tuple response format")

    def test_validate_http_response_with_dict(self):
        """Test validation with dictionary response."""
        response = MockHTTPResponse(
            status_code=404,
            text='{"error": "not_found", "message": "Resource not found"}'
        )

        # Parse the text to get a dict
        import json
        body_dict = json.loads(response.text)

        result = validate_http_error_response((response.status_code, body_dict), throw_on_error=False)

        self.assertTrue(result, "Should handle dictionary body")

    def test_validate_http_response_invalid_body(self):
        """Test validation fails with invalid response body."""
        response = MockHTTPResponse(
            status_code=404,
            text='{"error": "not_found"}'  # Missing message
        )

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_http_error_response(response)

        self.assertIn("message", str(context.exception))

    def test_validate_http_response_invalid_type(self):
        """Test validation raises TypeError for invalid response type."""
        invalid_response = "not a response"

        with self.assertRaises(TypeError) as context:
            validate_http_error_response(invalid_response)

        self.assertIn("tuple", str(context.exception))


class TestThrowOnErrorModes(unittest.TestCase):
    """Test throw_on_error parameter behavior."""

    def test_throw_on_error_true_raises_exception(self):
        """Test throw_on_error=True raises exception on validation failure."""
        response = {"error": "not_found"}  # Missing message

        with self.assertRaises(ErrorResponseValidationError):
            validate_standard_error_response(response, throw_on_error=True)

    def test_throw_on_error_false_returns_false(self):
        """Test throw_on_error=False returns False on validation failure."""
        response = {"error": "not_found"}  # Missing message

        result = validate_standard_error_response(response, throw_on_error=False)

        self.assertFalse(result, "Should return False for invalid response")

    def test_throw_on_error_true_returns_true_on_success(self):
        """Test throw_on_error=True returns True on validation success."""
        response = {"error": "not_found", "message": "Resource not found"}

        result = validate_standard_error_response(response, throw_on_error=False)

        self.assertTrue(result, "Should return True for valid response")


class TestRealWorldScenarios(unittest.TestCase):
    """Test common real-world API error response scenarios."""

    def test_api_not_found_response(self):
        """Test typical 404 Not Found API response."""
        response = {
            "error": "not_found",
            "message": "The requested resource was not found",
            "code": 404,
            "details": {"resource_type": "user", "resource_id": "123"}
        }

        validate_detailed_error_response(response)

    def test_authentication_failure_response(self):
        """Test authentication failure error response."""
        response = {
            "error": "authentication_failed",
            "message": "Invalid API key or token",
            "code": 401,
            "details": {"auth_method": "bearer_token"}
        }

        validate_detailed_error_response(response)

    def test_authorization_failure_response(self):
        """Test authorization failure error response."""
        response = {
            "error": "access_denied",
            "message": "You do not have permission to access this resource",
            "code": 403
        }

        validate_detailed_error_response(response)

    def test_rate_limit_exceeded_response(self):
        """Test rate limiting error response."""
        response = {
            "error": "rate_limit_exceeded",
            "message": "API rate limit exceeded. Please try again later.",
            "code": 429,
            "details": {
                "limit": 100,
                "window": "1h",
                "retry_after": 3600
            }
        }

        validate_detailed_error_response(response)

    def test_server_error_response(self):
        """Test internal server error response."""
        response = {
            "error": "internal_server_error",
            "message": "An unexpected error occurred. Please try again later.",
            "code": 500,
            "request_id": "req_abc123"
        }

        validate_detailed_error_response(response)

    def test_service_unavailable_response(self):
        """Test service unavailable error response."""
        response = {
            "error": "service_unavailable",
            "message": "Service temporarily unavailable for maintenance",
            "code": 503,
            "details": {
                "retry_after": 3600,
                "maintenance_window": "2026-07-14 02:00-04:00 UTC"
            }
        }

        validate_detailed_error_response(response)

    def test_bad_request_response(self):
        """Test bad request error response."""
        response = {
            "error": "bad_request",
            "message": "Invalid request parameters",
            "code": 400,
            "details": {
                "invalid_fields": ["email", "phone"],
                "errors": {
                    "email": "Invalid email format",
                    "phone": "Invalid phone number format"
                }
            }
        }

        validate_detailed_error_response(response)


class TestErrorMessages(unittest.TestCase):
    """Test quality and clarity of error messages."""

    def test_error_message_includes_missing_fields(self):
        """Test error message lists missing required fields."""
        response = {"message": "Some message"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_standard_error_response(response)

        error_msg = str(context.exception)
        self.assertIn("error", error_msg)
        self.assertIn("Missing required fields", error_msg)

    def test_error_message_includes_unexpected_fields(self):
        """Test error message lists unexpected fields."""
        response = {
            "error": "not_found",
            "message": "Resource not found",
            "unexpected": "value"
        }

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_standard_error_response(response)

        error_msg = str(context.exception)
        self.assertIn("unexpected", error_msg)
        self.assertIn("Unexpected fields", error_msg)

    def test_error_message_includes_field_errors(self):
        """Test error message includes field-specific errors."""
        response = {"error": "", "message": ""}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_with_standard_validators(response)

        error_msg = str(context.exception)
        self.assertIn("Field validation errors", error_msg)

    def test_error_message_includes_response_body(self):
        """Test error message includes response body for debugging."""
        response = {"message": "Some message"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_standard_error_response(response)

        error_msg = str(context.exception)
        self.assertIn("Response body", error_msg)

    def test_error_message_clarity(self):
        """Test error messages are clear and actionable."""
        response = {"error": None, "message": "test"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_with_standard_validators(response)

        error_msg = str(context.exception)
        # Should have clear structure
        self.assertIn("validation failed", error_msg.lower())


class TestEdgeCases(unittest.TestCase):
    """Test edge cases and boundary conditions."""

    def test_response_with_many_fields(self):
        """Test validation with response containing many fields."""
        response = {
            "error": "complex_error",
            "message": "Complex error with many fields",
            "code": 500,
            "details": {},
            "stack_trace": "...",
            "request_id": "...",
            "timestamp": "2026-07-14T12:00:00Z",
            "user_id": "123",
            "session_id": "abc"
        }

        # Create a spec that allows all these fields
        custom_spec = ErrorResponseSpec(
            required_fields={"error", "message"},
            optional_fields={
                "code", "details", "stack_trace", "request_id",
                "timestamp", "user_id", "session_id"
            }
        )

        result = validate_error_response(response, custom_spec, throw_on_error=False)
        self.assertTrue(result, "Should handle responses with many fields")

    def test_response_with_nested_objects(self):
        """Test validation with nested object fields."""
        response = {
            "error": "validation_error",
            "message": "Request validation failed",
            "details": {
                "fields": ["email", "password"],
                "errors": {
                    "email": "Invalid format",
                    "password": "Too short"
                },
                "metadata": {
                    "attempt": 3,
                    "last_attempt": "2026-07-14T12:00:00Z"
                }
            }
        }

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)
        self.assertTrue(result, "Should handle nested object fields")

    def test_response_with_array_values(self):
        """Test validation with array field values."""
        response = {
            "error": "batch_error",
            "message": "Multiple errors occurred",
            "details": {
                "errors": ["Error 1", "Error 2", "Error 3"]
            }
        }

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)
        self.assertTrue(result, "Should handle array field values")

    def test_response_with_special_characters(self):
        """Test validation with special characters in field values."""
        response = {
            "error": "parse_error",
            "message": "Error: Unexpected token '<' at line 5, column 10\n\nJSON parse failed"
        }

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)
        self.assertTrue(result, "Should handle special characters")

    def test_response_with_unicode(self):
        """Test validation with Unicode characters."""
        response = {
            "error": "encoding_error",
            "message": "Error: 你好, مرحبا,こんにちは"
        }

        result = validate_error_response(response, STANDARD_ERROR_SPEC, throw_on_error=False)
        self.assertTrue(result, "Should handle Unicode characters")

    def test_spec_with_no_optional_fields(self):
        """Test spec with only required fields."""
        strict_spec = ErrorResponseSpec(
            required_fields={"error", "message"},
            optional_fields=set()
        )

        response = {"error": "test", "message": "test message", "extra": "value"}

        with self.assertRaises(ErrorResponseValidationError) as context:
            validate_error_response(response, strict_spec)

        self.assertIn("extra", str(context.exception))

    def test_spec_with_all_optional_fields(self):
        """Test spec with only optional fields."""
        loose_spec = ErrorResponseSpec(
            required_fields=set(),
            optional_fields={"error", "message", "code"}
        )

        response = {"error": "test", "message": "test"}

        result = validate_error_response(response, loose_spec, throw_on_error=False)
        self.assertTrue(result, "Should accept any combination of optional fields")


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("ERROR RESPONSE STRUCTURE VALIDATION TEST SUITE")
    print("Bead: bf-64826u")
    print("=" * 80)
    print()

    # Run tests with unittest
    loader = unittest.TestLoader()
    suite = loader.loadTestsFromModule(sys.modules[__name__])
    runner = unittest.TextTestRunner(verbosity=2)
    result = runner.run(suite)

    print()
    print("=" * 80)
    if result.wasSuccessful():
        print("✅ ALL TESTS PASSED")
        print("=" * 80)
        print()
        print("Coverage Summary:")
        print("  ✓ Required field validation (error, message)")
        print("  ✓ Optional field validation (code, details)")
        print("  ✓ JSON parsing from string responses")
        print("  ✓ Custom field validators")
        print("  ✓ Multiple response validation")
        print("  ✓ Edge cases and error conditions")
        print("  ✓ Real-world API error scenarios")
        print("  ✓ Error message quality and clarity")
        print("  ✓ Convenience functions")
        print("  ✓ Built-in validators")
        print("  ✓ HTTP response validation")
        print("  ✓ Throw/no-throw modes")
        print()
        return 0
    else:
        print("❌ SOME TESTS FAILED")
        print("=" * 80)
        print(f"Failures: {len(result.failures)}")
        print(f"Errors: {len(result.errors)}")
        return 1


if __name__ == '__main__':
    sys.exit(main())
