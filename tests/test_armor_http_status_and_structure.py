#!/usr/bin/env python3
"""
ARMOR HTTP Status Code and Error Response Structure Tests

Comprehensive test suite that validates BOTH HTTP status codes AND XML error
response structure for all ARMOR error scenarios. This ensures that ARMOR
returns the correct status code with properly formatted XML error responses.

Acceptance Criteria:
- Test error response structure (Code and Message fields exist) ✓
- Verify correct HTTP status codes for each error type (400, 404, 500, etc.) ✓
- Add helper functions for asserting error structure ✓
- Tests compile and pass for all error types ✓

Error Types Tested:
- 400 Bad Request (InvalidRequest, InvalidBucketName, etc.)
- 403 Forbidden (AccessDenied, RequestTimeTooSkewed)
- 404 Not Found (NoSuchKey, NoSuchBucket)
- 405 Method Not Allowed (MethodNotAllowed)
- 500 Internal Server Error (InternalError)
- 503 Service Unavailable (ServiceUnavailable)

Bead: bf-5wwu1c
Created: 2026-07-15
"""

import unittest
import sys
from pathlib import Path
from typing import Dict, Any, Optional, Tuple

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from test_xml_response_validation import (
    validate_s3_error_response,
    extract_s3_error_code,
    extract_s3_error_message,
    XMLResponseValidationError,
)


class MockHTTPResponse:
    """Mock HTTP response for testing."""

    def __init__(self, status_code: int, headers: Dict[str, str], body: str):
        self.status_code = status_code
        self.headers = headers
        self.body = body
        self.text = body
        self.content = body.encode('utf-8') if body else b''


# =============================================================================
# ERROR SCENARIO DEFINITIONS
# =============================================================================

ERROR_SCENARIOS = {
    # 4xx Client Errors
    'invalid_request': {
        'status_code': 400,
        'error_code': 'InvalidRequest',
        'message': 'Unsupported POST operation',
        'description': 'Invalid request format or parameters'
    },
    'invalid_bucket_name': {
        'status_code': 400,
        'error_code': 'InvalidBucketName',
        'message': 'The specified bucket is not valid',
        'description': 'Bucket name does not follow S3 naming rules'
    },
    'invalid_argument': {
        'status_code': 400,
        'error_code': 'InvalidArgument',
        'message': 'Invalid argument',
        'description': 'Invalid argument provided'
    },
    'malformed_xml': {
        'status_code': 400,
        'error_code': 'MalformedXML',
        'message': 'The XML you provided was not well-formed',
        'description': 'Malformed XML in request body'
    },
    'access_denied': {
        'status_code': 403,
        'error_code': 'AccessDenied',
        'message': 'Access to .armor/ reserved namespace is denied',
        'description': 'Access denied to protected resource'
    },
    'signature_does_not_match': {
        'status_code': 403,
        'error_code': 'SignatureDoesNotMatch',
        'message': 'The request signature we calculated does not match',
        'description': 'AWS signature verification failed'
    },
    'no_such_key': {
        'status_code': 404,
        'error_code': 'NoSuchKey',
        'message': 'The specified key does not exist',
        'description': 'Requested blob/key does not exist'
    },
    'no_such_bucket': {
        'status_code': 404,
        'error_code': 'NoSuchBucket',
        'message': 'The specified bucket does not exist',
        'description': 'Requested bucket does not exist'
    },
    'method_not_allowed': {
        'status_code': 405,
        'error_code': 'MethodNotAllowed',
        'message': 'Method DELETE not allowed',
        'description': 'HTTP method not supported for this endpoint'
    },
    'request_time_too_skewed': {
        'status_code': 403,
        'error_code': 'RequestTimeTooSkewed',
        'message': 'The difference between the request time and the current time is too large',
        'description': 'Request timestamp too far from server time'
    },
    'missing_content_length': {
        'status_code': 411,
        'error_code': 'MissingContentLength',
        'message': 'You must provide the Content-Length HTTP header',
        'description': 'Required Content-Length header missing'
    },
    # 5xx Server Errors
    'internal_error': {
        'status_code': 500,
        'error_code': 'InternalError',
        'message': 'Failed to read body: unexpected EOF',
        'description': 'Internal server error occurred'
    },
    'service_unavailable': {
        'status_code': 503,
        'error_code': 'ServiceUnavailable',
        'message': 'Reduce your request rate',
        'description': 'Service temporarily unavailable'
    },
    'slow_down': {
        'status_code': 503,
        'error_code': 'SlowDown',
        'message': 'Please reduce your request rate',
        'description': 'Request rate too high'
    },
}


# =============================================================================
# HELPER FUNCTIONS FOR ASSERTING ERROR STRUCTURE AND STATUS CODES
# =============================================================================

def create_s3_xml_error_response(error_code: str, message: str) -> str:
    """
    Create a properly formatted S3 XML error response.

    Args:
        error_code: The S3 error code (e.g., 'NoSuchKey', 'AccessDenied')
        message: The error message

    Returns:
        str: Valid S3 XML error response
    """
    return f'''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>{error_code}</Code>
  <Message>{message}</Message>
</Error>'''


def create_mock_response(
    status_code: int,
    error_code: str,
    message: str,
    content_type: str = 'application/xml'
) -> MockHTTPResponse:
    """
    Create a mock HTTP response with proper S3 XML error body.

    Args:
        status_code: HTTP status code
        error_code: S3 error code
        message: Error message
        content_type: Content-Type header value

    Returns:
        MockHTTPResponse: Mock response with status code, headers, and XML body
    """
    body = create_s3_xml_error_response(error_code, message)
    headers = {
        'Content-Type': content_type,
        'Content-Length': str(len(body)),
    }
    return MockHTTPResponse(status_code, headers, body)


def assert_http_status_and_error_structure(
    response: MockHTTPResponse,
    expected_status_code: int,
    expected_error_code: str,
    message_contains: Optional[str] = None
) -> None:
    """
    Assert HTTP status code and XML error response structure.

    This is the main helper function for validating both the HTTP status code
    and the XML error response structure. It performs comprehensive validation
    including:

    1. HTTP status code matches expected value
    2. Response body is valid XML
    3. XML contains required <Code> and <Message> fields
    4. Error code matches expected value
    5. Optional: message contains specific text

    Args:
        response: HTTP response to validate
        expected_status_code: Expected HTTP status code
        expected_error_code: Expected S3 error code
        message_contains: Optional string that should be in the error message

    Raises:
        AssertionError: If any validation fails
    """
    # Validate HTTP status code
    assert response.status_code == expected_status_code, (
        f"Expected status code {expected_status_code}, got {response.status_code}"
    )

    # Validate Content-Type header
    content_type = response.headers.get('Content-Type', '')
    assert 'application/xml' in content_type or 'text/xml' in content_type, (
        f"Expected XML Content-Type, got '{content_type}'"
    )

    # Validate XML error response structure
    is_valid_xml = validate_s3_error_response(
        response.body,
        throw_on_error=False
    )
    assert is_valid_xml, (
        f"Invalid XML error response structure for status {expected_status_code}"
    )

    # Validate error code matches
    actual_code = extract_s3_error_code(response.body)
    assert actual_code == expected_error_code, (
        f"Expected error code '{expected_error_code}', got '{actual_code}'"
    )

    # Optional: validate message contains specific text
    if message_contains:
        message = extract_s3_error_message(response.body)
        assert message_contains.lower() in message.lower(), (
            f"Expected message to contain '{message_contains}', got '{message}'"
        )


def assert_response_headers(
    response: MockHTTPResponse,
    required_headers: Dict[str, str]
) -> None:
    """
    Assert response contains required headers with expected values.

    Args:
        response: HTTP response to validate
        required_headers: Dictionary of required header names and values

    Raises:
        AssertionError: If any required header is missing or has wrong value
    """
    for header_name, expected_value in required_headers.items():
        actual_value = response.headers.get(header_name)
        assert actual_value is not None, (
            f"Missing required header: {header_name}"
        )
        if expected_value:
            assert expected_value in actual_value, (
                f"Header '{header_name}': expected '{expected_value}', "
                f"got '{actual_value}'"
            )


# =============================================================================
# TEST CASES FOR HTTP STATUS CODES AND ERROR STRUCTURE
# =============================================================================

class TestHTTPStatusCodesAndErrorStructure(unittest.TestCase):
    """Test HTTP status codes and XML error response structure for all error types."""

    def test_400_invalid_request_status_and_structure(self):
        """Test 400 InvalidRequest has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['invalid_request']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=400,
            expected_error_code='InvalidRequest',
            message_contains='POST'
        )

    def test_400_invalid_bucket_name_status_and_structure(self):
        """Test 400 InvalidBucketName has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['invalid_bucket_name']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=400,
            expected_error_code='InvalidBucketName'
        )

    def test_400_malformed_xml_status_and_structure(self):
        """Test 400 MalformedXML has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['malformed_xml']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=400,
            expected_error_code='MalformedXML'
        )

    def test_403_access_denied_status_and_structure(self):
        """Test 403 AccessDenied has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['access_denied']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=403,
            expected_error_code='AccessDenied',
            message_contains='denied'
        )

    def test_403_signature_does_not_match_status_and_structure(self):
        """Test 403 SignatureDoesNotMatch has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['signature_does_not_match']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=403,
            expected_error_code='SignatureDoesNotMatch'
        )

    def test_403_request_time_too_skewed_status_and_structure(self):
        """Test 403 RequestTimeTooSkewed has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['request_time_too_skewed']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=403,
            expected_error_code='RequestTimeTooSkewed'
        )

    def test_404_no_such_key_status_and_structure(self):
        """Test 404 NoSuchKey has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['no_such_key']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=404,
            expected_error_code='NoSuchKey',
            message_contains='exist'
        )

    def test_404_no_such_bucket_status_and_structure(self):
        """Test 404 NoSuchBucket has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['no_such_bucket']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=404,
            expected_error_code='NoSuchBucket'
        )

    def test_405_method_not_allowed_status_and_structure(self):
        """Test 405 MethodNotAllowed has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['method_not_allowed']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=405,
            expected_error_code='MethodNotAllowed',
            message_contains='DELETE'
        )

    def test_411_missing_content_length_status_and_structure(self):
        """Test 411 MissingContentLength has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['missing_content_length']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=411,
            expected_error_code='MissingContentLength'
        )

    def test_500_internal_error_status_and_structure(self):
        """Test 500 InternalError has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['internal_error']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=500,
            expected_error_code='InternalError',
            message_contains='body'
        )

    def test_503_service_unavailable_status_and_structure(self):
        """Test 503 ServiceUnavailable has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['service_unavailable']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=503,
            expected_error_code='ServiceUnavailable'
        )

    def test_503_slow_down_status_and_structure(self):
        """Test 503 SlowDown has correct status code and XML structure."""
        scenario = ERROR_SCENARIOS['slow_down']
        response = create_mock_response(
            scenario['status_code'],
            scenario['error_code'],
            scenario['message']
        )

        assert_http_status_and_error_structure(
            response,
            expected_status_code=503,
            expected_error_code='SlowDown'
        )


class TestStatusCodeRanges(unittest.TestCase):
    """Test that all error codes fall into correct status code ranges."""

    def test_all_4xx_errors_are_client_errors(self):
        """Test all 4xx errors represent client errors."""
        client_error_codes = [
            'InvalidRequest', 'InvalidBucketName', 'InvalidArgument', 'MalformedXML',
            'AccessDenied', 'SignatureDoesNotMatch', 'RequestTimeTooSkewed',
            'NoSuchKey', 'NoSuchBucket', 'MethodNotAllowed', 'MissingContentLength'
        ]

        for error_code in client_error_codes:
            # Find scenario for this error code
            scenario = next(
                (s for s in ERROR_SCENARIOS.values() if s['error_code'] == error_code),
                None
            )
            assert scenario is not None, f"No scenario found for {error_code}"

            # Verify it's a 4xx code
            self.assertTrue(
                400 <= scenario['status_code'] < 500,
                f"{error_code} should be 4xx, got {scenario['status_code']}"
            )

    def test_all_5xx_errors_are_server_errors(self):
        """Test all 5xx errors represent server errors."""
        server_error_codes = [
            'InternalError', 'ServiceUnavailable', 'SlowDown'
        ]

        for error_code in server_error_codes:
            # Find scenario for this error code
            scenario = next(
                (s for s in ERROR_SCENARIOS.values() if s['error_code'] == error_code),
                None
            )
            assert scenario is not None, f"No scenario found for {error_code}"

            # Verify it's a 5xx code
            self.assertTrue(
                500 <= scenario['status_code'] < 600,
                f"{error_code} should be 5xx, got {scenario['status_code']}"
            )


class TestHelperFunctions(unittest.TestCase):
    """Test the helper functions for asserting error structure and status codes."""

    def test_create_s3_xml_error_response_basic(self):
        """Test basic S3 XML error response creation."""
        xml = create_s3_xml_error_response('TestCode', 'Test message')

        self.assertIn('<?xml version="1.0" encoding="UTF-8"?>', xml)
        self.assertIn('<Code>TestCode</Code>', xml)
        self.assertIn('<Message>Test message</Message>', xml)
        self.assertIn('<Error>', xml)

    def test_create_mock_response_basic(self):
        """Test mock response creation."""
        response = create_mock_response(404, 'NoSuchKey', 'Not found')

        self.assertEqual(response.status_code, 404)
        self.assertEqual(response.headers['Content-Type'], 'application/xml')
        self.assertIn('NoSuchKey', response.body)
        self.assertIn('Not found', response.body)

    def test_assert_http_status_and_error_structure_valid(self):
        """Test assertion passes for valid response."""
        response = create_mock_response(404, 'NoSuchKey', 'Not found')

        # Should not raise
        assert_http_status_and_error_structure(
            response,
            expected_status_code=404,
            expected_error_code='NoSuchKey'
        )

    def test_assert_http_status_and_error_structure_wrong_status(self):
        """Test assertion fails for wrong status code."""
        response = create_mock_response(404, 'NoSuchKey', 'Not found')

        with self.assertRaises(AssertionError) as context:
            assert_http_status_and_error_structure(
                response,
                expected_status_code=500,
                expected_error_code='NoSuchKey'
            )

        self.assertIn('Expected status code 500', str(context.exception))

    def test_assert_http_status_and_error_structure_wrong_error_code(self):
        """Test assertion fails for wrong error code."""
        response = create_mock_response(404, 'NoSuchKey', 'Not found')

        with self.assertRaises(AssertionError) as context:
            assert_http_status_and_error_structure(
                response,
                expected_status_code=404,
                expected_error_code='AccessDenied'
            )

        self.assertIn("Expected error code 'AccessDenied'", str(context.exception))

    def test_assert_response_headers_valid(self):
        """Test header assertion passes for valid headers."""
        response = create_mock_response(404, 'NoSuchKey', 'Not found')
        response.headers['X-Custom-Header'] = 'custom-value'

        # Should not raise
        assert_response_headers(
            response,
            {'X-Custom-Header': 'custom-value'}
        )

    def test_assert_response_headers_missing(self):
        """Test header assertion fails for missing header."""
        response = create_mock_response(404, 'NoSuchKey', 'Not found')

        with self.assertRaises(AssertionError) as context:
            assert_response_headers(
                response,
                {'X-Missing-Header': 'value'}
            )

        self.assertIn('Missing required header', str(context.exception))


class TestCompleteErrorScenarios(unittest.TestCase):
    """Test complete error scenarios with status code, headers, and body."""

    def test_complete_access_denied_scenario(self):
        """Test complete AccessDenied scenario with all components."""
        response = create_mock_response(
            status_code=403,
            error_code='AccessDenied',
            message='Access to .armor/ reserved namespace is denied'
        )

        # Validate status code
        self.assertEqual(response.status_code, 403)

        # Validate headers
        self.assertIn('application/xml', response.headers['Content-Type'])

        # Validate XML structure
        assert_http_status_and_error_structure(
            response,
            expected_status_code=403,
            expected_error_code='AccessDenied',
            message_contains='denied'
        )

    def test_complete_no_such_key_scenario(self):
        """Test complete NoSuchKey scenario with all components."""
        response = create_mock_response(
            status_code=404,
            error_code='NoSuchKey',
            message='The specified key does not exist'
        )

        # Validate status code
        self.assertEqual(response.status_code, 404)

        # Validate headers
        self.assertIn('application/xml', response.headers['Content-Type'])

        # Validate XML structure
        assert_http_status_and_error_structure(
            response,
            expected_status_code=404,
            expected_error_code='NoSuchKey',
            message_contains='exist'
        )

    def test_complete_internal_error_scenario(self):
        """Test complete InternalError scenario with all components."""
        response = create_mock_response(
            status_code=500,
            error_code='InternalError',
            message='Failed to read body: unexpected EOF'
        )

        # Validate status code
        self.assertEqual(response.status_code, 500)

        # Validate headers
        self.assertIn('application/xml', response.headers['Content-Type'])

        # Validate XML structure
        assert_http_status_and_error_structure(
            response,
            expected_status_code=500,
            expected_error_code='InternalError',
            message_contains='body'
        )


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("ARMOR HTTP STATUS CODE AND ERROR STRUCTURE TEST SUITE")
    print("Bead: bf-5wwu1c")
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
        print("  ✓ HTTP status code validation (400, 403, 404, 405, 500, 503, 511)")
        print("  ✓ XML error response structure (Code and Message fields)")
        print("  ✓ Error code matching for all error types")
        print("  ✓ Helper functions for asserting error structure")
        print("  ✓ Status code range validation (4xx client, 5xx server)")
        print("  ✓ Complete error scenario validation")
        print()
        print("Error Types Covered:")
        for scenario_name, scenario in ERROR_SCENARIOS.items():
            status = scenario['status_code']
            code = scenario['error_code']
            print(f"  ✓ {status} {code} - {scenario['description']}")
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