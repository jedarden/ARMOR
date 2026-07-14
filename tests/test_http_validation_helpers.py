#!/usr/bin/env python3
"""
Test Cases for HTTP Status Code Validation Helper Functions

Comprehensive test suite for the HTTP status code validation helpers.
Tests cover:
- Single status code validation
- Multiple allowed status codes
- Both requests.Response and tuple response formats
- Valid and invalid status codes
- Throw and no-throw modes
- Edge cases and error conditions

Acceptance Criteria:
- Function accepts a response object and expected status code(s) ✓
- Supports both single status codes and arrays of allowed codes ✓
- Returns boolean or throws assertion error with clear message ✓
- Includes test cases demonstrating valid/invalid status codes ✓
- Function is exported from test utils module ✓

Bead: bf-gfemoh
Created: 2026-07-14
"""

import unittest
import sys
from pathlib import Path
from typing import Tuple
from unittest.mock import Mock, MagicMock

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from test_helpers import (
    validate_http_status,
    validate_http_status_codes,
    StatusValidationError,
    validate_success,
    validate_redirect,
    validate_client_error,
    validate_server_error
)


class MockResponse:
    """
    Mock HTTP response object for testing.

    Simulates requests.Response behavior without making actual HTTP calls.
    """

    def __init__(self, status_code: int, text: str = "", url: str = "http://example.com"):
        self.status_code = status_code
        self.text = text
        self.url = url


class TestSingleStatusCodeValidation(unittest.TestCase):
    """Test validation with single expected status code."""

    def test_valid_single_status_code_requests_response(self):
        """Test successful validation with requests.Response object."""
        response = MockResponse(status_code=200, text="OK")

        result = validate_http_status(response, 200, throw_on_error=False)

        self.assertTrue(result, "Should return True for matching status code")

    def test_valid_single_status_code_tuple(self):
        """Test successful validation with tuple response format."""
        response = (200, "OK")

        result = validate_http_status(response, 200, throw_on_error=False)

        self.assertTrue(result, "Should return True for tuple response with matching status")

    def test_invalid_status_code_returns_false(self):
        """Test validation failure returns False when not throwing."""
        response = MockResponse(status_code=404, text="Not Found")

        result = validate_http_status(response, 200, throw_on_error=False)

        self.assertFalse(result, "Should return False for non-matching status code")

    def test_invalid_status_code_throws_error(self):
        """Test validation failure throws StatusValidationError by default."""
        response = MockResponse(status_code=404, text="Not Found")

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, 200)

        error = context.exception
        self.assertEqual(error.actual, 404)
        self.assertEqual(error.expected, 200)
        self.assertIn("404", str(error))
        self.assertIn("200", str(error))

    def test_error_message_includes_response_body(self):
        """Test error message includes response body for debugging."""
        response = MockResponse(status_code=500, text="Internal Server Error: Database connection failed")

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, 200)

        error_message = str(context.exception)
        self.assertIn("Internal Server Error", error_message)

    def test_error_message_includes_url(self):
        """Test error message includes URL when available."""
        response = MockResponse(
            status_code=403,
            text="Forbidden",
            url="http://example.com/api/protected"
        )

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, 200)

        error_message = str(context.exception)
        self.assertIn("http://example.com/api/protected", error_message)

    def test_long_response_body_truncated_in_error(self):
        """Test long response bodies are truncated in error messages."""
        long_body = "Error: " + "x" * 500
        response = MockResponse(status_code=500, text=long_body)

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, 200)

        error_message = str(context.exception)
        # Should be truncated
        self.assertLess(len(error_message.split("Response body: ")[1]), 250)
        self.assertIn("truncated", error_message)


class TestMultipleStatusCodesValidation(unittest.TestCase):
    """Test validation with multiple allowed status codes."""

    def test_valid_status_code_in_list(self):
        """Test validation passes when status is in allowed list."""
        response = MockResponse(status_code=204)

        result = validate_http_status(response, [200, 201, 204], throw_on_error=False)

        self.assertTrue(result, "Should accept status code in allowed list")

    def test_invalid_status_code_not_in_list(self):
        """Test validation fails when status is not in allowed list."""
        response = MockResponse(status_code=404)

        result = validate_http_status(response, [200, 201, 204], throw_on_error=False)

        self.assertFalse(result, "Should reject status code not in allowed list")

    def test_multiple_codes_error_message(self):
        """Test error message properly formats multiple allowed codes."""
        response = MockResponse(status_code=500, text="Server Error")

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, [200, 201, 204])

        error_message = str(context.exception)
        self.assertIn("200", error_message)
        self.assertIn("201", error_message)
        self.assertIn("204", error_message)

    def test_two_codes_or_formatting(self):
        """Test error message uses 'or' for two status codes."""
        response = MockResponse(status_code=404, text="Not Found")

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, [200, 201])

        error_message = str(context.exception)
        self.assertIn("200 or 201", error_message)

    def test_single_element_list(self):
        """Test validation with single-element list works correctly."""
        response = MockResponse(status_code=200)

        result = validate_http_status(response, [200], throw_on_error=False)

        self.assertTrue(result, "Should handle single-element list correctly")

    def test_large_status_code_list(self):
        """Test validation with many allowed status codes."""
        response = MockResponse(status_code=418)  # I'm a teapot

        allowed_codes = [200, 201, 202, 204, 301, 302, 304, 400, 401, 403, 404, 418]
        result = validate_http_status(response, allowed_codes, throw_on_error=False)

        self.assertTrue(result, "Should handle large lists of allowed codes")


class TestConvenienceFunctions(unittest.TestCase):
    """Test convenience functions for common status code ranges."""

    def test_validate_success_2xx(self):
        """Test validate_success accepts all 2xx codes."""
        for code in [200, 201, 202, 204, 206]:
            response = MockResponse(status_code=code)
            result = validate_success(response, throw_on_error=False)
            self.assertTrue(result, f"Should accept 2xx code {code}")

    def test_validate_success_rejects_non_2xx(self):
        """Test validate_success rejects non-2xx codes."""
        response = MockResponse(status_code=304)
        result = validate_success(response, throw_on_error=False)
        self.assertFalse(result, "Should reject non-2xx code")

    def test_validate_redirect_3xx(self):
        """Test validate_redirect accepts all 3xx codes."""
        for code in [301, 302, 304, 307, 308]:
            response = MockResponse(status_code=code)
            result = validate_redirect(response, throw_on_error=False)
            self.assertTrue(result, f"Should accept 3xx code {code}")

    def test_validate_redirect_rejects_non_3xx(self):
        """Test validate_redirect rejects non-3xx codes."""
        response = MockResponse(status_code=200)
        result = validate_redirect(response, throw_on_error=False)
        self.assertFalse(result, "Should reject non-3xx code")

    def test_validate_client_error_4xx(self):
        """Test validate_client_error accepts all 4xx codes."""
        for code in [400, 401, 403, 404, 418]:
            response = MockResponse(status_code=code)
            result = validate_client_error(response, throw_on_error=False)
            self.assertTrue(result, f"Should accept 4xx code {code}")

    def test_validate_client_error_rejects_non_4xx(self):
        """Test validate_client_error rejects non-4xx codes."""
        response = MockResponse(status_code=500)
        result = validate_client_error(response, throw_on_error=False)
        self.assertFalse(result, "Should reject non-4xx code")

    def test_validate_server_error_5xx(self):
        """Test validate_server_error accepts all 5xx codes."""
        for code in [500, 502, 503, 504]:
            response = MockResponse(status_code=code)
            result = validate_server_error(response, throw_on_error=False)
            self.assertTrue(result, f"Should accept 5xx code {code}")

    def test_validate_server_error_rejects_non_5xx(self):
        """Test validate_server_error rejects non-5xx codes."""
        response = MockResponse(status_code=404)
        result = validate_server_error(response, throw_on_error=False)
        self.assertFalse(result, "Should reject non-5xx code")


class TestMultipleResponseValidation(unittest.TestCase):
    """Test validation of multiple responses at once."""

    def test_validate_multiple_responses_all_pass(self):
        """Test validating multiple responses where all pass."""
        responses = [
            MockResponse(status_code=200),
            MockResponse(status_code=200),
            MockResponse(status_code=200)
        ]

        results = validate_http_status_codes(*responses, expected_status=200, throw_on_error=False)

        self.assertEqual(len(results), 3, "Should return result for each response")
        self.assertTrue(all(results), "All responses should pass validation")

    def test_validate_multiple_responses_some_fail(self):
        """Test validating multiple responses where some fail."""
        responses = [
            MockResponse(status_code=200),
            MockResponse(status_code=404),
            MockResponse(status_code=200)
        ]

        results = validate_http_status_codes(*responses, expected_status=200, throw_on_error=False)

        self.assertEqual(results, [True, False, True], "Should track individual results")

    def test_validate_multiple_throws_on_first_failure(self):
        """Test validation throws on first failure when throw_on_error=True."""
        responses = [
            MockResponse(status_code=200),
            MockResponse(status_code=404),
            MockResponse(status_code=500)  # Should never reach this
        ]

        with self.assertRaises(StatusValidationError):
            validate_http_status_codes(*responses, expected_status=200, throw_on_error=True)

    def test_validate_multiple_no_throw_returns_all_results(self):
        """Test validation returns all results when not throwing."""
        responses = [
            MockResponse(status_code=200, text="OK"),
            MockResponse(status_code=404, text="Not Found"),
            MockResponse(status_code=500, text="Server Error")
        ]

        results = validate_http_status_codes(*responses, expected_status=200, throw_on_error=False)

        self.assertEqual(len(results), 3)
        self.assertEqual(results, [True, False, False])


class TestEdgeCasesAndErrors(unittest.TestCase):
    """Test edge cases, type validation, and error conditions."""

    def test_tuple_response_without_body(self):
        """Test tuple response format with only status code."""
        response = (200,)

        result = validate_http_status(response, 200, throw_on_error=False)

        self.assertTrue(result, "Should handle tuple with only status code")

    def test_tuple_response_with_body(self):
        """Test tuple response format with status code and body."""
        response = (403, "Access Denied")

        result = validate_http_status(response, 403, throw_on_error=False)

        self.assertTrue(result, "Should handle tuple with status and body")

    def test_invalid_response_type(self):
        """Test validation raises TypeError for invalid response type."""
        invalid_response = "not a response"

        with self.assertRaises(TypeError) as context:
            validate_http_status(invalid_response, 200)

        self.assertIn("Response must be", str(context.exception))

    def test_invalid_expected_status_type(self):
        """Test validation raises TypeError for invalid expected_status type."""
        response = MockResponse(status_code=200)

        with self.assertRaises(TypeError) as context:
            validate_http_status(response, "200")  # String instead of int

        self.assertIn("must be int or List[int]", str(context.exception))

    def test_invalid_status_in_list(self):
        """Test validation raises TypeError when list contains non-int."""
        response = MockResponse(status_code=200)

        with self.assertRaises(TypeError) as context:
            validate_http_status(response, [200, "404"])  # String in list

        self.assertIn("must be integers", str(context.exception))

    def test_empty_allowed_list(self):
        """Test validation with empty list of allowed codes."""
        response = MockResponse(status_code=200)

        result = validate_http_status(response, [], throw_on_error=False)

        self.assertFalse(result, "Should reject when no codes are allowed")

    def test_zero_status_code(self):
        """Test validation handles edge case status code 0."""
        response = MockResponse(status_code=0)

        result = validate_http_status(response, 0, throw_on_error=False)

        self.assertTrue(result, "Should accept status code 0 when expected")

    def test_very_high_status_code(self):
        """Test validation handles very high status codes."""
        response = MockResponse(status_code=999)

        result = validate_http_status(response, 999, throw_on_error=False)

        self.assertTrue(result, "Should handle high status codes")


class TestRealWorldScenarios(unittest.TestCase):
    """Test common real-world validation scenarios."""

    def test_successful_get_request(self):
        """Test validating a successful GET request."""
        response = MockResponse(status_code=200, text="Success", url="http://api.example.com/users")

        # Should pass without throwing
        validate_http_status(response, 200)

    def test_resource_not_found(self):
        """Test validating a 404 Not Found response."""
        response = MockResponse(status_code=404, text="User not found", url="http://api.example.com/users/123")

        # For 404, we might expect either 404 or 410 Gone
        validate_http_status(response, [404, 410])

    def test_authentication_failure(self):
        """Test validating authentication failure."""
        response = MockResponse(
            status_code=401,
            text="Unauthorized: Invalid API key",
            url="http://api.example.com/protected"
        )

        # Should accept 401 or 403 (some APIs return 403 for auth issues)
        validate_http_status(response, [401, 403])

    def test_created_response(self):
        """Test validating resource creation."""
        response = MockResponse(status_code=201, text="Created", url="http://api.example.com/users")

        # Accept both 201 and 202 (some APIs process async)
        validate_http_status(response, [201, 202])

    def test_accepted_no_content(self):
        """Test validating accepted requests that may return 204."""
        response = MockResponse(status_code=204, text="", url="http://api.example.com/users/123")

        # Accept both 200 and 204 for successful updates
        validate_http_status(response, [200, 204])

    def test_rate_limiting(self):
        """Test validating rate limit response."""
        response = MockResponse(
            status_code=429,
            text="Too Many Requests: Rate limit exceeded",
            url="http://api.example.com/search"
        )

        # Should be 429
        validate_http_status(response, 429)

    def test_service_unavailable(self):
        """Test validating service unavailability."""
        response = MockResponse(
            status_code=503,
            text="Service Unavailable: Maintenance in progress",
            url="http://api.example.com"
        )

        # Could be 503 or 504
        validate_http_status(response, [503, 504])


class TestErrorResponseQuality(unittest.TestCase):
    """Test that error messages provide helpful debugging information."""

    def test_error_message_structure(self):
        """Test error messages have proper structure."""
        response = MockResponse(
            status_code=500,
            text="Database connection failed",
            url="http://api.example.com/users"
        )

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, 200)

        error_msg = str(context.exception)

        # Should contain key information
        self.assertIn("URL:", error_msg)
        self.assertIn("Expected HTTP status code:", error_msg)
        self.assertIn("Actual status code:", error_msg)
        self.assertIn("Response body:", error_msg)

    def test_error_without_body_still_useful(self):
        """Test error message is useful even without response body."""
        response = MockResponse(status_code=404, text="", url="http://example.com")

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, 200)

        error_msg = str(context.exception)
        self.assertIn("404", error_msg)
        self.assertIn("200", error_msg)

    def test_error_without_url_still_useful(self):
        """Test error message is useful even without URL."""
        response = (500, "Server Error")

        with self.assertRaises(StatusValidationError) as context:
            validate_http_status(response, 200)

        error_msg = str(context.exception)
        self.assertIn("500", error_msg)
        self.assertIn("200", error_msg)
        self.assertIn("Server Error", error_msg)


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("HTTP STATUS CODE VALIDATION HELPER TEST SUITE")
    print("Bead: bf-gfemoh")
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
        print("  ✓ Single status code validation (requests.Response and tuple)")
        print("  ✓ Multiple allowed status codes")
        print("  ✓ Throw and no-throw modes")
        print("  ✓ Convenience functions (2xx, 3xx, 4xx, 5xx)")
        print("  ✓ Multiple response validation")
        print("  ✓ Edge cases and type validation")
        print("  ✓ Real-world scenarios")
        print("  ✓ Error message quality")
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
