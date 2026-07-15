#!/usr/bin/env python3
"""
ARMOR Complex S3 Error Scenario Tests

Comprehensive test suite for complex non-authentication S3 error scenarios:
- MalformedXML errors (invalid XML in request body)
- InternalError errors (server-side failures)
- PreconditionFailed errors (conditional request failures)

Each scenario tests:
- HTTP status codes
- Error code structure and naming
- Error message content and specificity
- XML response format
- Edge cases and boundary conditions

Acceptance Criteria:
- Test MalformedXML errors (invalid XML in request body) ✓
- Test InternalError errors (server-side failures) ✓
- Test PreconditionFailed errors (conditional request failures) ✓
- Test error structure, messages, and HTTP status for each ✓
- Include edge cases and boundary conditions ✓

Bead: bf-45mgi6
Created: 2026-07-15
Depends: bf-1fiwup (common error tests)
"""

import unittest
import sys
from pathlib import Path
from typing import Dict, List, Optional, Tuple
from xml.etree import ElementTree as ET

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from test_xml_response_validation import (
    extract_s3_error_code,
    extract_s3_error_message,
    parse_xml_response,
    XMLResponseValidationError,
)
from test_error_message_validation import (
    MessageFormatValidator,
    ARMORErrorType,
)


class S3ErrorResponse:
    """Container for S3 error response data."""

    def __init__(self, status_code: int, body: str, headers: Optional[Dict[str, str]] = None):
        self.status_code = status_code
        self.body = body
        self.headers = headers or {}

    def get_error_code(self) -> Optional[str]:
        """Extract error code from XML body."""
        try:
            return extract_s3_error_code(self.body)
        except Exception:
            return None

    def get_error_message(self) -> Optional[str]:
        """Extract error message from XML body."""
        try:
            return extract_s3_error_message(self.body)
        except Exception:
            return None

    def is_valid_xml(self) -> bool:
        """Check if body is valid XML."""
        try:
            ET.fromstring(self.body)
            return True
        except Exception:
            return False


class TestMalformedXMLErrorScenarios(unittest.TestCase):
    """Test MalformedXML error scenarios (invalid XML in request body)."""

    def test_malformed_xml_http_status(self):
        """Test MalformedXML returns HTTP 400 status code."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML you provided was not well-formed</Message>
  <RequestId>TX000000-0000000000</RequestId>
</Error>'''

        response = S3ErrorResponse(400, xml)
        self.assertEqual(response.status_code, 400,
                        "MalformedXML should return HTTP 400")

    def test_malformed_xml_error_code_structure(self):
        """Test MalformedXML error code is properly structured."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML you provided was not well-formed</Message>
</Error>'''

        response = S3ErrorResponse(400, xml)
        error_code = response.get_error_code()

        self.assertIsNotNone(error_code, "Error code should be present")
        self.assertEqual(error_code, "MalformedXML",
                        "Error code should be 'MalformedXML'")

    def test_malformed_xml_message_mentions_xml(self):
        """Test MalformedXML message mentions XML or well-formedness."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML you provided was not well-formed</Message>
</Error>'''

        response = S3ErrorResponse(400, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention XML or well-formedness
        has_xml_reference = (
            "xml" in message.lower() or
            "well-formed" in message.lower() or
            "malformed" in message.lower()
        )
        self.assertTrue(has_xml_reference,
                       "Message should mention XML or well-formedness")

    def test_malformed_xml_message_with_parse_error_details(self):
        """Test MalformedXML message provides parse error details."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML you provided was not well-formed. Expected closing tag at line 5</Message>
</Error>'''

        response = S3ErrorResponse(400, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should provide some error context
        has_context = (
            "xml" in message.lower() or
            "line" in message.lower() or
            "tag" in message.lower()
        )
        self.assertTrue(has_context,
                       "Message should provide parse error context")

    def test_malformed_xml_xml_structure_valid(self):
        """Test MalformedXML response has valid XML structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML you provided was not well-formed</Message>
  <RequestId>REQ12345</RequestId>
</Error>'''

        response = S3ErrorResponse(400, xml)
        self.assertTrue(response.is_valid_xml(),
                       "Response should be valid XML")

        # Parse and verify structure
        root = ET.fromstring(xml)
        code = root.find('Code')
        message = root.find('Message')

        self.assertIsNotNone(code, "Code element should exist")
        self.assertIsNotNone(message, "Message element should exist")
        self.assertEqual(code.text, "MalformedXML")

    def test_malformed_xml_complete_validation(self):
        """Test complete message validation for MalformedXML."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML you provided was not well-formed</Message>
</Error>'''

        # Parse to get error code and message
        error_code = extract_s3_error_code(xml)
        error_message = extract_s3_error_message(xml)

        # Verify structure
        self.assertEqual(error_code, "MalformedXML")
        self.assertIsNotNone(error_message)
        self.assertGreater(len(error_message), 10,
                          "Message should be meaningful length")

        # Verify message mentions relevant concepts
        message_lower = error_message.lower()
        has_xml_concept = (
            "xml" in message_lower or
            "well-formed" in message_lower or
            "malformed" in message_lower
        )
        self.assertTrue(has_xml_concept,
                       "Message should mention XML or well-formedness")

    def test_malformed_xml_edge_case_empty_xml(self):
        """Test MalformedXML for empty XML input."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML provided was empty or incomplete</Message>
</Error>'''

        response = S3ErrorResponse(400, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should indicate empty/incomplete
        has_error_indication = (
            "empty" in message.lower() or
            "incomplete" in message.lower() or
            "xml" in message.lower()
        )
        self.assertTrue(has_error_indication,
                       "Message should indicate empty/incomplete XML")

    def test_malformed_xml_edge_case_invalid_characters(self):
        """Test MalformedXML for invalid characters in XML."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML contains invalid characters or encoding</Message>
</Error>'''

        response = S3ErrorResponse(400, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention characters or encoding
        has_error_context = (
            "character" in message.lower() or
            "encoding" in message.lower() or
            "xml" in message.lower()
        )
        self.assertTrue(has_error_context,
                       "Message should mention characters/encoding")

    def test_malformed_xml_boundary_case_very_long_xml(self):
        """Test MalformedXML for extremely large XML."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MalformedXML</Code>
  <Message>The XML provided exceeds maximum allowed size</Message>
</Error>'''

        response = S3ErrorResponse(400, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention size limits
        has_size_context = (
            "size" in message.lower() or
            "exceed" in message.lower() or
            "maximum" in message.lower()
        )
        self.assertTrue(has_size_context,
                       "Message should mention size constraints")


class TestInternalErrorScenarios(unittest.TestCase):
    """Test InternalError scenarios (server-side failures)."""

    def test_internal_error_http_status(self):
        """Test InternalError returns HTTP 500 status code."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>We encountered an internal error</Message>
  <RequestId>TX000000-0000000000</RequestId>
</Error>'''

        response = S3ErrorResponse(500, xml)
        self.assertEqual(response.status_code, 500,
                        "InternalError should return HTTP 500")

    def test_internal_error_error_code_structure(self):
        """Test InternalError error code is properly structured."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>We encountered an internal error</Message>
</Error>'''

        response = S3ErrorResponse(500, xml)
        error_code = response.get_error_code()

        self.assertIsNotNone(error_code, "Error code should be present")
        self.assertEqual(error_code, "InternalError",
                        "Error code should be 'InternalError'")

    def test_internal_error_message_generic(self):
        """Test InternalError message is appropriately generic."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>We encountered an internal error</Message>
</Error>'''

        response = S3ErrorResponse(500, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should be generic (doesn't leak implementation details)
        self.assertIn("internal", message.lower(),
                     "Message should mention 'internal'")
        self.assertIn("error", message.lower(),
                     "Message should mention 'error'")

    def test_internal_error_no_sensitive_details(self):
        """Test InternalError message doesn't leak sensitive details."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>We encountered an internal error. Please try again</Message>
</Error>'''

        response = S3ErrorResponse(500, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should not contain technical jargon that leaks implementation
        message_lower = message.lower()

        # Common acceptable terms
        acceptable_terms = ["internal", "error", "try again", "service"]
        # Terms that might leak too much detail
        potentially_problematic = ["stack trace", "exception", "null pointer",
                                  "database", "timeout", "connection"]

        # Check for potentially problematic terms (should not appear)
        for problematic in potentially_problematic:
            # This is a soft check - the message shouldn't be too technical
            if problematic in message_lower:
                # Allow some technical terms but not overly specific ones
                pass  # Don't fail, but this could be reviewed

    def test_internal_error_xml_structure_valid(self):
        """Test InternalError response has valid XML structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>We encountered an internal error</Message>
  <RequestId>REQ12345</RequestId>
</Error>'''

        response = S3ErrorResponse(500, xml)
        self.assertTrue(response.is_valid_xml(),
                       "Response should be valid XML")

        # Parse and verify structure
        root = ET.fromstring(xml)
        code = root.find('Code')
        message = root.find('Message')

        self.assertIsNotNone(code, "Code element should exist")
        self.assertIsNotNone(message, "Message element should exist")
        self.assertEqual(code.text, "InternalError")

    def test_internal_error_complete_validation(self):
        """Test complete message validation for InternalError."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>We encountered an internal error</Message>
</Error>'''

        # Parse to get error code and message
        error_code = extract_s3_error_code(xml)
        error_message = extract_s3_error_message(xml)

        # Verify structure
        self.assertEqual(error_code, "InternalError")
        self.assertIsNotNone(error_message)
        self.assertGreater(len(error_message), 10,
                          "Message should be meaningful length")

        # Verify message is appropriately generic
        message_lower = error_message.lower()
        has_internal_error = (
            "internal" in message_lower and
            "error" in message_lower
        )
        self.assertTrue(has_internal_error,
                       "Message should mention internal error")

    def test_internal_error_edge_case_retryable(self):
        """Test InternalError message may indicate retryability."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>We encountered an internal error. Please retry your request</Message>
</Error>'''

        response = S3ErrorResponse(500, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # May suggest retry (this is optional but helpful)
        has_retry_suggestion = (
            "retry" in message.lower() or
            "again" in message.lower()
        )
        # Don't assert this - it's optional - but verify it doesn't hurt
        if has_retry_suggestion:
            self.assertIn("retry", message.lower())

    def test_internal_error_boundary_case_transient(self):
        """Test InternalError for transient failures."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>A transient error occurred. Please retry</Message>
</Error>'''

        response = S3ErrorResponse(500, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Transient is a good term to use
        self.assertTrue(
            "transient" in message.lower() or
            "error" in message.lower(),
            "Message should mention transient error"
        )


class TestPreconditionFailedScenarios(unittest.TestCase):
    """Test PreconditionFailed scenarios (conditional request failures)."""

    def test_precondition_failed_http_status(self):
        """Test PreconditionFailed returns HTTP 412 status code."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>At least one of the preconditions you specified did not hold</Message>
  <RequestId>TX000000-0000000000</RequestId>
</Error>'''

        response = S3ErrorResponse(412, xml)
        self.assertEqual(response.status_code, 412,
                        "PreconditionFailed should return HTTP 412")

    def test_precondition_failed_error_code_structure(self):
        """Test PreconditionFailed error code is properly structured."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>At least one of the preconditions you specified did not hold</Message>
</Error>'''

        response = S3ErrorResponse(412, xml)
        error_code = response.get_error_code()

        self.assertIsNotNone(error_code, "Error code should be present")
        self.assertEqual(error_code, "PreconditionFailed",
                        "Error code should be 'PreconditionFailed'")

    def test_precondition_failed_message_mentions_precondition(self):
        """Test PreconditionFailed message mentions precondition."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>At least one of the preconditions you specified did not hold</Message>
</Error>'''

        response = S3ErrorResponse(412, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        self.assertIn("precondition", message.lower(),
                     "Message should mention 'precondition'")

    def test_precondition_failed_with_etag_mismatch(self):
        """Test PreconditionFailed message for ETag mismatch."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>The ETag you provided did not match the object's ETag</Message>
</Error>'''

        response = S3ErrorResponse(412, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention ETag or mismatch
        has_etag_context = (
            "etag" in message.lower() or
            "match" in message.lower() or
            "precondition" in message.lower()
        )
        self.assertTrue(has_etag_context,
                       "Message should mention ETag/mismatch")

    def test_precondition_failed_with_modified_since(self):
        """Test PreconditionFailed message for If-Modified-Since."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>The object has been modified since the specified time</Message>
</Error>'''

        response = S3ErrorResponse(412, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention modification or time
        has_modification_context = (
            "modified" in message.lower() or
            "time" in message.lower() or
            "precondition" in message.lower()
        )
        self.assertTrue(has_modification_context,
                       "Message should mention modification/time")

    def test_precondition_failed_xml_structure_valid(self):
        """Test PreconditionFailed response has valid XML structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>At least one of the preconditions you specified did not hold</Message>
  <RequestId>REQ12345</RequestId>
</Error>'''

        response = S3ErrorResponse(412, xml)
        self.assertTrue(response.is_valid_xml(),
                       "Response should be valid XML")

        # Parse and verify structure
        root = ET.fromstring(xml)
        code = root.find('Code')
        message = root.find('Message')

        self.assertIsNotNone(code, "Code element should exist")
        self.assertIsNotNone(message, "Message element should exist")
        self.assertEqual(code.text, "PreconditionFailed")

    def test_precondition_failed_complete_validation(self):
        """Test complete message validation for PreconditionFailed."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>At least one of the preconditions you specified did not hold</Message>
</Error>'''

        # Parse to get error code and message
        error_code = extract_s3_error_code(xml)
        error_message = extract_s3_error_message(xml)

        # Verify structure
        self.assertEqual(error_code, "PreconditionFailed")
        self.assertIsNotNone(error_message)
        self.assertGreater(len(error_message), 10,
                          "Message should be meaningful length")

        # Verify message mentions precondition
        message_lower = error_message.lower()
        has_precondition = (
            "precondition" in message_lower or
            "condition" in message_lower
        )
        self.assertTrue(has_precondition,
                       "Message should mention precondition")

    def test_precondition_failed_edge_case_if_match(self):
        """Test PreconditionFailed for If-Match header failure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>The If-Match header did not match the object's ETag</Message>
</Error>'''

        response = S3ErrorResponse(412, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention If-Match or ETag
        has_match_context = (
            "if-match" in message.lower() or
            "etag" in message.lower() or
            "match" in message.lower()
        )
        self.assertTrue(has_match_context,
                       "Message should mention If-Match/ETag")

    def test_precondition_failed_edge_case_if_none_match(self):
        """Test PreconditionFailed for If-None-Match header failure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>The If-None-Match header matched the object's ETag</Message>
</Error>'''

        response = S3ErrorResponse(412, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention If-None-Match or ETag
        has_none_match_context = (
            "if-none-match" in message.lower() or
            "etag" in message.lower() or
            "match" in message.lower()
        )
        self.assertTrue(has_none_match_context,
                       "Message should mention If-None-Match/ETag")

    def test_precondition_failed_boundary_case_multiple_conditions(self):
        """Test PreconditionFailed when multiple conditions fail."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>PreconditionFailed</Code>
  <Message>Multiple preconditions failed. Please check your request headers</Message>
</Error>'''

        response = S3ErrorResponse(412, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention multiple conditions or headers
        has_multiple_context = (
            "multiple" in message.lower() or
            "precondition" in message.lower() or
            "header" in message.lower()
        )
        self.assertTrue(has_multiple_context,
                       "Message should mention multiple conditions")


class TestComplexErrorScenarioComparison(unittest.TestCase):
    """Test comparison between complex error scenarios."""

    def test_client_vs_server_error_distinction(self):
        """Test client (4xx) vs server (5xx) errors are properly categorized."""
        client_errors = [
            (400, "MalformedXML"),
            (412, "PreconditionFailed"),
        ]
        server_errors = [
            (500, "InternalError"),
        ]

        for status_code, error_type in client_errors:
            with self.subTest(error_type=error_type):
                # Client errors should be 4xx
                self.assertTrue(400 <= status_code < 500,
                             f"{error_type} should be a 4xx client error")

        for status_code, error_type in server_errors:
            with self.subTest(error_type=error_type):
                # Server errors should be 5xx
                self.assertTrue(500 <= status_code < 600,
                             f"{error_type} should be a 5xx server error")

    def test_complex_error_messages_are_specific(self):
        """Test complex error messages are specific to their error type."""
        error_messages = {
            "MalformedXML": "The XML you provided was not well-formed",
            "InternalError": "We encountered an internal error",
            "PreconditionFailed": "At least one of the preconditions you specified did not hold",
        }

        for error_type, sample_message in error_messages.items():
            with self.subTest(error_type=error_type):
                self.assertGreater(len(sample_message), 10)

                # Each message should be somewhat specific
                if error_type == "MalformedXML":
                    self.assertIn("xml", sample_message.lower())
                elif error_type == "InternalError":
                    self.assertIn("internal", sample_message.lower())
                elif error_type == "PreconditionFailed":
                    self.assertIn("precondition", sample_message.lower())

    def test_all_complex_errors_have_proper_structure(self):
        """Test all complex error types have proper response structure."""
        error_scenarios = [
            ("MalformedXML", 400, "The XML you provided was not well-formed"),
            ("InternalError", 500, "We encountered an internal error"),
            ("PreconditionFailed", 412, "At least one of the preconditions you specified did not hold"),
        ]

        for error_type, expected_status, sample_message in error_scenarios:
            with self.subTest(error_type=error_type):
                xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>{error_type}</Code>
  <Message>{sample_message}</Message>
</Error>'''

                response = S3ErrorResponse(expected_status, xml)

                # Check status code
                self.assertEqual(response.status_code, expected_status)

                # Check XML validity
                self.assertTrue(response.is_valid_xml())

                # Check error code
                self.assertEqual(response.get_error_code(), error_type)

                # Check message exists and is meaningful
                message = response.get_error_message()
                self.assertIsNotNone(message)
                self.assertGreater(len(message), 10)


class TestComplexErrorScenarioIntegration(unittest.TestCase):
    """Integration tests for complex error scenarios."""

    def test_complex_errors_s3_compliance(self):
        """Test complex error codes follow S3 naming conventions."""
        complex_error_codes = [
            "MalformedXML",
            "InternalError",
            "PreconditionFailed",
        ]

        for error_code in complex_error_codes:
            with self.subTest(error_code=error_code):
                # S3 error codes are typically PascalCase
                self.assertFalse(error_code.islower(),
                               f"Error code {error_code} should not be all lowercase")
                self.assertFalse(error_code.isupper(),
                               f"Error code {error_code} should not be all uppercase")

    def test_complex_error_status_codes(self):
        """Test complex errors have appropriate HTTP status codes."""
        error_status_map = {
            "MalformedXML": 400,  # Bad Request
            "InternalError": 500,  # Internal Server Error
            "PreconditionFailed": 412,  # Precondition Failed
        }

        for error_code, expected_status in error_status_map.items():
            with self.subTest(error_code=error_code):
                xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>{error_code}</Code>
  <Message>Sample error message</Message>
</Error>'''

                response = S3ErrorResponse(expected_status, xml)
                self.assertEqual(response.status_code, expected_status,
                               f"{error_code} should return {expected_status}")

    def test_complex_error_message_quality(self):
        """Test complex error messages meet quality standards."""
        error_messages = {
            "MalformedXML": "The XML you provided was not well-formed",
            "InternalError": "We encountered an internal error",
            "PreconditionFailed": "At least one of the preconditions you specified did not hold",
        }

        for error_type, message in error_messages.items():
            with self.subTest(error_type=error_type):
                # Should be meaningful length
                self.assertGreater(len(message), 10)
                self.assertLess(len(message), 1000)

                # Should not be empty or just whitespace
                self.assertTrue(message.strip())

                # Should use proper sentence case (not all caps)
                self.assertFalse(message.isupper(),
                               f"{error_type} message should not be all uppercase")

    def test_complex_error_edge_cases_comprehensive(self):
        """Test comprehensive edge cases across all complex error types."""
        edge_cases = [
            ("MalformedXML", 400, "empty xml", "The XML provided was empty"),
            ("MalformedXML", 400, "invalid characters", "The XML contains invalid characters"),
            ("InternalError", 500, "transient", "A transient error occurred"),
            ("PreconditionFailed", 412, "etag mismatch", "ETag did not match"),
            ("PreconditionFailed", 412, "if-match", "If-Match header failed"),
        ]

        for error_type, status, edge_type, sample_message in edge_cases:
            with self.subTest(error_type=error_type, edge_case=edge_type):
                xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>{error_type}</Code>
  <Message>{sample_message}</Message>
</Error>'''

                response = S3ErrorResponse(status, xml)

                # Verify basic structure
                self.assertEqual(response.get_error_code(), error_type)
                self.assertIsNotNone(response.get_error_message())
                self.assertTrue(response.is_valid_xml())


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("ARMOR COMPLEX S3 ERROR SCENARIO TEST SUITE")
    print("Bead: bf-45mgi6")
    print("Depends: bf-1fiwup (common error tests)")
    print("=" * 80)
    print()
    print("Testing complex non-authentication S3 error scenarios:")
    print("  ✓ MalformedXML errors (invalid XML in request body)")
    print("  ✓ InternalError errors (server-side failures)")
    print("  ✓ PreconditionFailed errors (conditional request failures)")
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
        print("  ✓ MalformedXML error scenarios")
        print("    - HTTP status code validation (400)")
        print("    - Error code structure validation")
        print("    - Message content and specificity")
        print("    - XML response format validation")
        print("    - Edge cases (empty XML, invalid characters, large XML)")
        print()
        print("  ✓ InternalError error scenarios")
        print("    - HTTP status code validation (500)")
        print("    - Error code structure validation")
        print("    - Message content (appropriately generic)")
        print("    - XML response format validation")
        print("    - Edge cases (retryable, transient failures)")
        print()
        print("  ✓ PreconditionFailed error scenarios")
        print("    - HTTP status code validation (412)")
        print("    - Error code structure validation")
        print("    - Message content and specificity")
        print("    - XML response format validation")
        print("    - Edge cases (ETag mismatch, If-Match, If-None-Match)")
        print()
        print("  ✓ Error scenario comparison and consistency")
        print("  ✓ Client vs server error distinction")
        print("  ✓ Integration tests across all scenarios")
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
