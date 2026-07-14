#!/usr/bin/env python3
"""
Test Cases for HTTP Status Code and Content-Type Validation Helper Functions

Comprehensive test suite for the HTTP validation helpers.
Tests cover:

1. HTTP Status Code Validation:
   - Single status code validation
   - Multiple allowed status codes
   - Both requests.Response and tuple response formats
   - Valid and invalid status codes
   - Throw and no-throw modes
   - Edge cases and error conditions

2. Content-Type Header Validation:
   - Pattern matching for content-types
   - Multiple allowed content-types
   - Various content-type formats and parameters
   - JSON, XML, HTML, text content-types
   - Both requests.Response and tuple response formats
   - Throw and no-throw modes

Acceptance Criteria:
- Function accepts a response object and expected status/content-type(s) ✓
- Supports both single values and arrays of allowed values ✓
- Returns boolean or throws assertion error with clear message ✓
- Includes test cases demonstrating various scenarios ✓
- Function is exported from test utils module ✓

Bead: bf-gfemoh, bf-q6dmsn
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
    validate_server_error,
    validate_content_type,
    ContentTypeValidationError,
    validate_json_content_type,
    validate_xml_content_type,
    validate_html_content_type,
    validate_text_content_type,
    validate_cors_headers,
    CORSValidationError,
    validate_cors_allow_origin,
    validate_cors_wildcard,
    validate_cors_specific_origin,
    validate_cors_credentials,
)


class MockResponse:
    """
    Mock HTTP response object for testing.

    Simulates requests.Response behavior without making actual HTTP calls.
    """

    def __init__(self, status_code: int, text: str = "", url: str = "http://example.com",
                 content_type: str = None):
        self.status_code = status_code
        self.text = text
        self.url = url
        self.headers = {}
        if content_type:
            self.headers['Content-Type'] = content_type


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


# =============================================================================
# CONTENT-TYPE VALIDATION TESTS
# =============================================================================

class TestContentTypeValidationBasic(unittest.TestCase):
    """Test basic content-type validation functionality."""

    def test_valid_content_type_exact_match(self):
        """Test validation passes with exact content-type match."""
        response = MockResponse(status_code=200, content_type='application/json')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should accept exact content-type match")

    def test_valid_content_type_pattern_match(self):
        """Test validation passes with pattern matching (ignoring charset)."""
        response = MockResponse(status_code=200,
                                content_type='application/json; charset=utf-8')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should match content-type pattern ignoring parameters")

    def test_valid_content_type_with_charset(self):
        """Test validation handles charset parameter correctly."""
        response = MockResponse(status_code=200,
                                content_type='text/html; charset=utf-8')

        result = validate_content_type(response, 'text/html', throw_on_error=False)

        self.assertTrue(result, "Should handle charset parameter")

    def test_valid_content_type_case_insensitive(self):
        """Test validation is case-insensitive."""
        response = MockResponse(status_code=200,
                                content_type='Application/JSON; Charset=UTF-8')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should handle case-insensitive matching")

    def test_invalid_content_type_returns_false(self):
        """Test validation failure returns False when not throwing."""
        response = MockResponse(status_code=200, content_type='text/html')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertFalse(result, "Should reject non-matching content-type")

    def test_invalid_content_type_throws_error(self):
        """Test validation failure throws ContentTypeValidationError by default."""
        response = MockResponse(status_code=200, content_type='text/html')

        with self.assertRaises(ContentTypeValidationError) as context:
            validate_content_type(response, 'application/json')

        error = context.exception
        self.assertIn('text/html', str(error))
        self.assertIn('application/json', str(error))

    def test_missing_content_type(self):
        """Test validation handles missing Content-Type header."""
        response = MockResponse(status_code=200)  # No content-type set

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertFalse(result, "Should reject missing content-type")


class TestContentTypeValidationMultipleTypes(unittest.TestCase):
    """Test validation with multiple allowed content-types."""

    def test_valid_type_in_list(self):
        """Test validation passes when content-type is in allowed list."""
        response = MockResponse(status_code=200, content_type='application/xml')

        result = validate_content_type(response,
                                       ['application/json', 'application/xml'],
                                       throw_on_error=False)

        self.assertTrue(result, "Should accept content-type in allowed list")

    def test_invalid_type_not_in_list(self):
        """Test validation fails when content-type is not in allowed list."""
        response = MockResponse(status_code=200, content_type='text/html')

        result = validate_content_type(response,
                                       ['application/json', 'application/xml'],
                                       throw_on_error=False)

        self.assertFalse(result, "Should reject content-type not in allowed list")

    def test_multiple_types_error_message(self):
        """Test error message properly formats multiple allowed types."""
        response = MockResponse(status_code=200, content_type='text/html')

        with self.assertRaises(ContentTypeValidationError) as context:
            validate_content_type(response, ['application/json', 'application/xml'])

        error_message = str(context.exception)
        self.assertIn('application/json', error_message)
        self.assertIn('application/xml', error_message)

    def test_pattern_match_with_multiple_types(self):
        """Test pattern matching works with multiple allowed types."""
        response = MockResponse(status_code=200,
                                content_type='application/json; charset=utf-8')

        result = validate_content_type(response,
                                       ['application/json', 'application/xml'],
                                       throw_on_error=False)

        self.assertTrue(result, "Should pattern match against multiple types")


class TestContentTypeValidationTupleResponse(unittest.TestCase):
    """Test content-type validation with tuple response format."""

    def test_tuple_response_with_headers_dict(self):
        """Test validation with tuple response (status, headers dict)."""
        response = (200, {'Content-Type': 'application/json'}, '{}')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should handle tuple with headers dict")

    def test_tuple_response_with_headers_dict_lowercase(self):
        """Test validation with lowercase header key in tuple."""
        response = (200, {'content-type': 'application/json'}, '{}')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should handle lowercase content-type in headers")

    def test_tuple_response_pattern_match(self):
        """Test pattern matching works with tuple response."""
        response = (200,
                   {'Content-Type': 'application/json; charset=utf-8'},
                   '{}')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should pattern match with tuple response")

    def test_tuple_response_missing_content_type(self):
        """Test tuple response without content-type."""
        response = (200, {}, '{}')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertFalse(result, "Should reject tuple without content-type")


class TestContentTypeValidationConvenienceFunctions(unittest.TestCase):
    """Test convenience functions for common content-types."""

    def test_validate_json_content_type_standard(self):
        """Test validate_json_content_type accepts standard JSON."""
        response = MockResponse(status_code=200, content_type='application/json')

        result = validate_json_content_type(response, throw_on_error=False)

        self.assertTrue(result, "Should accept application/json")

    def test_validate_json_content_type_with_charset(self):
        """Test validate_json_content_type accepts JSON with charset."""
        response = MockResponse(status_code=200,
                                content_type='application/json; charset=utf-8')

        result = validate_json_content_type(response, throw_on_error=False)

        self.assertTrue(result, "Should accept application/json with charset")

    def test_validate_json_content_type_variants(self):
        """Test validate_json_content_type accepts JSON variants."""
        json_variants = [
            'application/json',
            'text/json',
            'application/vnd.api+json',
            'application/problem+json'
        ]

        for ct in json_variants:
            response = MockResponse(status_code=200, content_type=ct)
            result = validate_json_content_type(response, throw_on_error=False)
            self.assertTrue(result, f"Should accept JSON variant: {ct}")

    def test_validate_json_content_type_rejects_non_json(self):
        """Test validate_json_content_type rejects non-JSON types."""
        response = MockResponse(status_code=200, content_type='text/html')

        result = validate_json_content_type(response, throw_on_error=False)

        self.assertFalse(result, "Should reject non-JSON content-type")

    def test_validate_xml_content_type(self):
        """Test validate_xml_content_type accepts XML variants."""
        xml_variants = [
            'application/xml',
            'text/xml',
            'application/rss+xml',
            'application/atom+xml'
        ]

        for ct in xml_variants:
            response = MockResponse(status_code=200, content_type=ct)
            result = validate_xml_content_type(response, throw_on_error=False)
            self.assertTrue(result, f"Should accept XML variant: {ct}")

    def test_validate_xml_content_type_rejects_non_xml(self):
        """Test validate_xml_content_type rejects non-XML types."""
        response = MockResponse(status_code=200, content_type='application/json')

        result = validate_xml_content_type(response, throw_on_error=False)

        self.assertFalse(result, "Should reject non-XML content-type")

    def test_validate_html_content_type(self):
        """Test validate_html_content_type accepts HTML variants."""
        html_variants = [
            'text/html',
            'application/xhtml+xml'
        ]

        for ct in html_variants:
            response = MockResponse(status_code=200, content_type=ct)
            result = validate_html_content_type(response, throw_on_error=False)
            self.assertTrue(result, f"Should accept HTML variant: {ct}")

    def test_validate_html_content_type_rejects_non_html(self):
        """Test validate_html_content_type rejects non-HTML types."""
        response = MockResponse(status_code=200, content_type='application/json')

        result = validate_html_content_type(response, throw_on_error=False)

        self.assertFalse(result, "Should reject non-HTML content-type")

    def test_validate_text_content_type(self):
        """Test validate_text_content_type accepts text/plain."""
        response = MockResponse(status_code=200, content_type='text/plain')

        result = validate_text_content_type(response, throw_on_error=False)

        self.assertTrue(result, "Should accept text/plain")

    def test_validate_text_content_type_with_charset(self):
        """Test validate_text_content_type accepts text/plain with charset."""
        response = MockResponse(status_code=200,
                                content_type='text/plain; charset=utf-8')

        result = validate_text_content_type(response, throw_on_error=False)

        self.assertTrue(result, "Should accept text/plain with charset")


class TestContentTypeValidationEdgeCases(unittest.TestCase):
    """Test edge cases and error conditions."""

    def test_empty_content_type(self):
        """Test validation handles empty content-type string."""
        response = MockResponse(status_code=200, content_type='')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertFalse(result, "Should reject empty content-type")

    def test_whitespace_handling(self):
        """Test validation handles whitespace correctly."""
        response = MockResponse(status_code=200,
                                content_type='  application/json; charset=utf-8  ')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should handle whitespace in content-type")

    def test_multiple_parameters(self):
        """Test validation handles multiple parameters."""
        response = MockResponse(status_code=200,
                                content_type='application/json; charset=utf-8; version=1')

        result = validate_content_type(response, 'application/json', throw_on_error=False)

        self.assertTrue(result, "Should handle multiple parameters")

    def test_invalid_response_type(self):
        """Test validation raises TypeError for invalid response type."""
        invalid_response = "not a response"

        with self.assertRaises(TypeError) as context:
            validate_content_type(invalid_response, 'application/json')

        self.assertIn("Response must be", str(context.exception))

    def test_invalid_expected_type_type(self):
        """Test validation raises TypeError for invalid expected_type type."""
        response = MockResponse(status_code=200, content_type='application/json')

        with self.assertRaises(TypeError) as context:
            validate_content_type(response, 123)  # Number instead of string

        self.assertIn("must be str or List[str]", str(context.exception))

    def test_invalid_type_in_list(self):
        """Test validation raises TypeError when list contains non-string."""
        response = MockResponse(status_code=200, content_type='application/json')

        with self.assertRaises(TypeError) as context:
            validate_content_type(response, ['application/json', 123])

        self.assertIn("must be strings", str(context.exception))

    def test_error_message_includes_url(self):
        """Test error message includes URL when available."""
        response = MockResponse(
            status_code=200,
            content_type='text/html',
            url='http://api.example.com/data'
        )

        with self.assertRaises(ContentTypeValidationError) as context:
            validate_content_type(response, 'application/json')

        error_message = str(context.exception)
        self.assertIn('http://api.example.com/data', error_message)


class TestContentTypeValidationRealWorldScenarios(unittest.TestCase):
    """Test common real-world content-type validation scenarios."""

    def test_api_returns_json(self):
        """Test validating JSON API response."""
        response = MockResponse(
            status_code=200,
            content_type='application/json; charset=utf-8',
            url='http://api.example.com/users'
        )

        validate_json_content_type(response)

    def test_api_error_response(self):
        """Test validating error response has correct content-type."""
        response = MockResponse(
            status_code=404,
            content_type='application/problem+json',
            url='http://api.example.com/users/123'
        )

        validate_json_content_type(response)

    def test_web_page_returns_html(self):
        """Test validating HTML page response."""
        response = MockResponse(
            status_code=200,
            content_type='text/html; charset=utf-8',
            url='http://example.com/page'
        )

        validate_html_content_type(response)

    def test_xml_feed(self):
        """Test validating XML feed response."""
        response = MockResponse(
            status_code=200,
            content_type='application/rss+xml',
            url='http://example.com/feed'
        )

        validate_xml_content_type(response)

    def test_flexible_json_acceptance(self):
        """Test accepting multiple JSON content-type variants."""
        response = MockResponse(
            status_code=200,
            content_type='application/vnd.api+json'
        )

        # Should accept any JSON variant
        validate_json_content_type(response)

    def test_api_content_negotiation(self):
        """Test handling multiple acceptable content-types."""
        response = MockResponse(
            status_code=200,
            content_type='application/xml'
        )

        # API might return either JSON or XML
        validate_content_type(response, ['application/json', 'application/xml'])

    def test_missing_content_type_error_message(self):
        """Test error message for missing content-type is helpful."""
        response = MockResponse(
            status_code=200,
            url='http://api.example.com/data'
        )

        with self.assertRaises(ContentTypeValidationError) as context:
            validate_json_content_type(response)

        error_message = str(context.exception)
        self.assertIn('(missing)', error_message)


class TestContentTypeValidationErrorQuality(unittest.TestCase):
    """Test that error messages provide helpful debugging information."""

    def test_error_message_structure(self):
        """Test error messages have proper structure."""
        response = MockResponse(
            status_code=200,
            content_type='text/html',
            url='http://api.example.com/data'
        )

        with self.assertRaises(ContentTypeValidationError) as context:
            validate_content_type(response, 'application/json')

        error_msg = str(context.exception)

        # Should contain key information
        self.assertIn('URL:', error_msg)
        self.assertIn('Expected Content-Type:', error_msg)
        self.assertIn('Actual Content-Type:', error_msg)

    def test_error_without_url_still_useful(self):
        """Test error message is useful even without URL."""
        response = (200, {}, '{}')

        with self.assertRaises(ContentTypeValidationError) as context:
            validate_content_type(response, 'application/json')

        error_msg = str(context.exception)
        self.assertIn('(missing)', error_msg)
        self.assertIn('application/json', error_msg)


# =============================================================================
# CORS HEADER VALIDATION TESTS
# =============================================================================

class TestCORSValidationBasic(unittest.TestCase):
    """Test basic CORS header validation functionality."""

    def test_validate_cors_allow_origin_present(self):
        """Test validation passes when Access-Control-Allow-Origin is present."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        result = validate_cors_headers(response, throw_on_error=False)

        self.assertTrue(result, "Should accept response with CORS headers")

    def test_validate_cors_allow_origin_missing(self):
        """Test validation fails when Access-Control-Allow-Origin is missing."""
        response = MockResponse(status_code=200, content_type='application/json')
        # No CORS headers set

        result = validate_cors_headers(response, throw_on_error=False)

        self.assertFalse(result, "Should reject response without CORS headers")

    def test_validate_cors_allow_origin_missing_throws(self):
        """Test validation throws error when CORS header is missing and throw_on_error=True."""
        response = MockResponse(status_code=200, content_type='application/json')

        with self.assertRaises(CORSValidationError) as context:
            validate_cors_headers(response, throw_on_error=True)

        error_message = str(context.exception)
        self.assertIn("Missing required CORS header", error_message)
        self.assertIn("Access-Control-Allow-Origin", error_message)

    def test_validate_cors_not_required_when_disabled(self):
        """Test validation passes when require_allow_origin=False."""
        response = MockResponse(status_code=200, content_type='application/json')
        # No CORS headers set

        result = validate_cors_headers(response, require_allow_origin=False, throw_on_error=False)

        self.assertTrue(result, "Should pass when CORS not required")


class TestCORSOriginValidation(unittest.TestCase):
    """Test CORS origin value validation."""

    def test_validate_wildcard_origin(self):
        """Test validation accepts wildcard origin."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        result = validate_cors_headers(response, expected_origin='*', throw_on_error=False)

        self.assertTrue(result, "Should accept wildcard origin")

    def test_validate_wildcard_when_allowed(self):
        """Test validation accepts wildcard when allow_wildcard=True."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        result = validate_cors_headers(response, expected_origin='https://example.com',
                                       allow_wildcard=True, throw_on_error=False)

        self.assertTrue(result, "Should accept wildcard when allowed")

    def test_validate_wildcard_when_not_allowed(self):
        """Test validation rejects wildcard when allow_wildcard=False."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        result = validate_cors_headers(response, expected_origin='https://example.com',
                                       allow_wildcard=False, throw_on_error=False)

        self.assertFalse(result, "Should reject wildcard when not allowed")

    def test_validate_wildcard_when_not_allowed_throws(self):
        """Test validation throws when wildcard not allowed."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        with self.assertRaises(CORSValidationError) as context:
            validate_cors_headers(response, expected_origin='https://example.com',
                                   allow_wildcard=False, throw_on_error=True)

        error_message = str(context.exception)
        self.assertIn("Wildcard origin", error_message)
        self.assertIn("not allowed", error_message)

    def test_validate_specific_origin_match(self):
        """Test validation accepts exact origin match."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'https://example.com'

        result = validate_cors_headers(response, expected_origin='https://example.com',
                                       throw_on_error=False)

        self.assertTrue(result, "Should accept exact origin match")

    def test_validate_specific_origin_mismatch(self):
        """Test validation rejects origin mismatch."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'https://other.com'

        result = validate_cors_headers(response, expected_origin='https://example.com',
                                       throw_on_error=False)

        self.assertFalse(result, "Should reject origin mismatch")

    def test_validate_specific_origin_mismatch_throws(self):
        """Test validation throws on origin mismatch."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'https://other.com'

        with self.assertRaises(CORSValidationError) as context:
            validate_cors_headers(response, expected_origin='https://example.com',
                                   throw_on_error=True)

        error_message = str(context.exception)
        self.assertIn("Origin mismatch", error_message)
        self.assertIn("https://example.com", error_message)
        self.assertIn("https://other.com", error_message)


class TestCORSValidationTupleResponse(unittest.TestCase):
    """Test CORS validation with tuple response format."""

    def test_tuple_response_with_cors_headers(self):
        """Test validation with tuple response including CORS headers."""
        response = (200, {'Access-Control-Allow-Origin': '*'}, '{}')

        result = validate_cors_headers(response, throw_on_error=False)

        self.assertTrue(result, "Should handle tuple with CORS headers")

    def test_tuple_response_lowercase_headers(self):
        """Test validation handles lowercase header keys in tuple."""
        response = (200, {'access-control-allow-origin': '*'}, '{}')

        result = validate_cors_headers(response, throw_on_error=False)

        self.assertTrue(result, "Should handle lowercase header keys")

    def test_tuple_response_with_specific_origin(self):
        """Test validation with specific origin in tuple response."""
        response = (200, {'Access-Control-Allow-Origin': 'https://example.com'}, '{}')

        result = validate_cors_headers(response, expected_origin='https://example.com',
                                       throw_on_error=False)

        self.assertTrue(result, "Should validate specific origin in tuple")

    def test_tuple_response_without_cors_headers(self):
        """Test tuple response without CORS headers fails validation."""
        response = (200, {}, '{}')

        result = validate_cors_headers(response, throw_on_error=False)

        self.assertFalse(result, "Should reject tuple without CORS headers")


class TestCORSValidationConvenienceFunctions(unittest.TestCase):
    """Test convenience functions for common CORS scenarios."""

    def test_validate_cors_allow_origin_present(self):
        """Test validate_cors_allow_origin accepts header present."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        result = validate_cors_allow_origin(response, throw_on_error=False)

        self.assertTrue(result, "Should accept when header is present")

    def test_validate_cors_allow_origin_missing(self):
        """Test validate_cors_allow_origin rejects missing header."""
        response = MockResponse(status_code=200, content_type='application/json')

        result = validate_cors_allow_origin(response, throw_on_error=False)

        self.assertFalse(result, "Should reject when header is missing")

    def test_validate_cors_wildcard_accepts_wildcard(self):
        """Test validate_cors_wildcard accepts wildcard origin."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        result = validate_cors_wildcard(response, throw_on_error=False)

        self.assertTrue(result, "Should accept wildcard origin")

    def test_validate_cors_wildcard_rejects_specific(self):
        """Test validate_cors_wildcard rejects specific origin."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'https://example.com'

        result = validate_cors_wildcard(response, throw_on_error=False)

        self.assertFalse(result, "Should reject specific origin when expecting wildcard")

    def test_validate_cors_specific_origin_match(self):
        """Test validate_cors_specific_origin accepts matching origin."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'https://example.com'

        result = validate_cors_specific_origin(response, 'https://example.com',
                                                throw_on_error=False)

        self.assertTrue(result, "Should accept matching specific origin")

    def test_validate_cors_specific_origin_rejects_wildcard(self):
        """Test validate_cors_specific_origin rejects wildcard."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = '*'

        result = validate_cors_specific_origin(response, 'https://example.com',
                                                throw_on_error=False)

        self.assertFalse(result, "Should reject wildcard when expecting specific origin")

    def test_validate_cors_credentials_present(self):
        """Test validate_cors_credentials accepts header present."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Credentials'] = 'true'

        result = validate_cors_credentials(response, throw_on_error=False)

        self.assertTrue(result, "Should accept when credentials header is present")

    def test_validate_cors_credentials_missing(self):
        """Test validate_cors_credentials rejects missing header."""
        response = MockResponse(status_code=200, content_type='application/json')

        result = validate_cors_credentials(response, throw_on_error=False)

        self.assertFalse(result, "Should reject when credentials header is missing")


class TestCORSValidationEdgeCases(unittest.TestCase):
    """Test edge cases and error conditions."""

    def test_empty_origin_value(self):
        """Test validation handles empty origin value."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = ''

        result = validate_cors_headers(response, expected_origin='https://example.com',
                                       throw_on_error=False)

        self.assertFalse(result, "Should reject empty origin value")

    def test_null_origin(self):
        """Test validation handles null origin."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'null'

        result = validate_cors_headers(response, expected_origin='null',
                                       throw_on_error=False)

        self.assertTrue(result, "Should accept null origin when expected")

    def test_multiple_cors_headers(self):
        """Test validation with multiple CORS headers."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'https://example.com'
        response.headers['Access-Control-Allow-Methods'] = 'GET, POST, PUT, DELETE'
        response.headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization'

        cors_headers = validate_cors_headers.__wrapped__.__globals__['_extract_cors_headers'](response) if hasattr(validate_cors_headers, '__wrapped__') else {}
        # We'll just test the main function works
        result = validate_cors_headers(response, expected_origin='https://example.com',
                                       throw_on_error=False)

        self.assertTrue(result, "Should handle multiple CORS headers")

    def test_error_message_includes_url(self):
        """Test error message includes URL when available."""
        response = MockResponse(
            status_code=200,
            content_type='application/json',
            url='http://api.example.com/data'
        )

        with self.assertRaises(CORSValidationError) as context:
            validate_cors_headers(response, throw_on_error=True)

        error_message = str(context.exception)
        self.assertIn('http://api.example.com/data', error_message)

    def test_error_message_includes_actual_headers(self):
        """Test error message includes actual CORS headers."""
        response = MockResponse(status_code=200, content_type='application/json')
        response.headers['Access-Control-Allow-Origin'] = 'https://other.com'

        with self.assertRaises(CORSValidationError) as context:
            validate_cors_headers(response, expected_origin='https://example.com',
                                   throw_on_error=True)

        error_message = str(context.exception)
        self.assertIn('Actual CORS headers', error_message)
        self.assertIn('https://other.com', error_message)


class TestCORSValidationRealWorldScenarios(unittest.TestCase):
    """Test common real-world CORS validation scenarios."""

    def test_public_api_with_wildcard(self):
        """Test validating public API that allows all origins."""
        response = MockResponse(
            status_code=200,
            content_type='application/json',
            url='http://api.example.com/public/data'
        )
        response.headers['Access-Control-Allow-Origin'] = '*'

        validate_cors_wildcard(response)

    def test_private_api_with_specific_origin(self):
        """Test validating private API with specific origin."""
        response = MockResponse(
            status_code=200,
            content_type='application/json',
            url='http://api.example.com/protected/data'
        )
        response.headers['Access-Control-Allow-Origin'] = 'https://myapp.com'

        validate_cors_specific_origin(response, 'https://myapp.com')

    def test_authenticated_api_with_credentials(self):
        """Test validating API that requires credentials."""
        response = MockResponse(
            status_code=200,
            content_type='application/json',
            url='http://api.example.com/user/profile'
        )
        response.headers['Access-Control-Allow-Origin'] = 'https://myapp.com'
        response.headers['Access-Control-Allow-Credentials'] = 'true'

        validate_cors_specific_origin(response, 'https://myapp.com')
        validate_cors_credentials(response)

    def test_mixed_mode_api(self):
        """Test validating API that accepts wildcard or specific origins."""
        response = MockResponse(
            status_code=200,
            content_type='application/json',
            url='http://api.example.com/data'
        )
        response.headers['Access-Control-Allow-Origin'] = '*'

        # Should accept wildcard when allowed
        validate_cors_headers(response, expected_origin='https://example.com',
                              allow_wildcard=True)

    def test_error_response_with_cors(self):
        """Test validating error responses still have proper CORS."""
        response = MockResponse(
            status_code=404,
            content_type='application/json',
            url='http://api.example.com/users/123'
        )
        response.headers['Access-Control-Allow-Origin'] = 'https://myapp.com'

        validate_cors_specific_origin(response, 'https://myapp.com')

    def test_preflight_request_headers(self):
        """Test validating preflight request CORS headers."""
        response = MockResponse(
            status_code=200,
            content_type='application/json',
            url='http://api.example.com/data'
        )
        response.headers['Access-Control-Allow-Origin'] = 'https://myapp.com'
        response.headers['Access-Control-Allow-Methods'] = 'GET, POST, PUT, DELETE, OPTIONS'
        response.headers['Access-Control-Allow-Headers'] = 'Content-Type, Authorization'
        response.headers['Access-Control-Max-Age'] = '86400'

        # Should still validate the origin correctly
        validate_cors_specific_origin(response, 'https://myapp.com')


class TestCORSValidationErrorQuality(unittest.TestCase):
    """Test that error messages provide helpful debugging information."""

    def test_error_message_structure(self):
        """Test error messages have proper structure."""
        response = MockResponse(
            status_code=200,
            content_type='application/json',
            url='http://api.example.com/data'
        )
        response.headers['Access-Control-Allow-Origin'] = 'https://other.com'

        with self.assertRaises(CORSValidationError) as context:
            validate_cors_headers(response, expected_origin='https://example.com',
                                   throw_on_error=True)

        error_msg = str(context.exception)

        # Should contain key information
        self.assertIn('Origin mismatch', error_msg)
        self.assertIn('https://example.com', error_msg)
        self.assertIn('https://other.com', error_msg)

    def test_error_without_url_still_useful(self):
        """Test error message is useful even without URL."""
        response = (200, {}, '{}')

        with self.assertRaises(CORSValidationError) as context:
            validate_cors_headers(response, throw_on_error=True)

        error_msg = str(context.exception)
        self.assertIn('Missing required CORS header', error_msg)
        self.assertIn('Access-Control-Allow-Origin', error_msg)


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("HTTP VALIDATION HELPER TEST SUITE")
    print("Bead: bf-gfemoh, bf-q6dmsn, bf-2rj8cd")
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
        print("  ✓ HTTP Status Code Validation:")
        print("    - Single status code validation (requests.Response and tuple)")
        print("    - Multiple allowed status codes")
        print("    - Throw and no-throw modes")
        print("    - Convenience functions (2xx, 3xx, 4xx, 5xx)")
        print("    - Multiple response validation")
        print("    - Edge cases and type validation")
        print("    - Real-world scenarios")
        print("    - Error message quality")
        print()
        print("  ✓ Content-Type Header Validation:")
        print("    - Pattern matching (ignores charset and parameters)")
        print("    - Multiple allowed content-types")
        print("    - Case-insensitive matching")
        print("    - Tuple and requests.Response formats")
        print("    - Convenience functions (JSON, XML, HTML, text)")
        print("    - Edge cases and error handling")
        print("    - Real-world scenarios")
        print("    - Error message quality")
        print()
        print("  ✓ CORS Header Validation:")
        print("    - Access-Control-Allow-Origin presence validation")
        print("    - Wildcard vs specific origin validation")
        print("    - Exact origin matching")
        print("    - Access-Control-Allow-Credentials validation")
        print("    - Tuple and requests.Response formats")
        print("    - Convenience functions (allow_origin, wildcard, specific_origin, credentials)")
        print("    - Edge cases (empty origin, null origin, multiple headers)")
        print("    - Real-world scenarios (public API, private API, authenticated API)")
        print("    - Error message quality")
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
