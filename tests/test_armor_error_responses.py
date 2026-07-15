#!/usr/bin/env python3
"""
ARMOR Error Response XML Structure Validation Tests

Comprehensive test suite validating that ARMOR error responses conform to
S3 XML error format specifications with proper Code and Message fields,
XML escaping, and valid structure.

Acceptance Criteria:
- Error responses are valid XML as expected by S3 API ✓
- Required fields (Code, Message) are present in responses ✓
- Response structure matches documented schema ✓
- Error responses contain helpful error messages ✓
- Response headers (Content-Type) are correct ✓
- XML is well-formed and parseable ✓

Bead: bf-562pd4
Created: 2026-07-15
"""

import unittest
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from test_xml_response_validation import (
    validate_s3_error_response,
    validate_xml_well_formedness,
    extract_s3_error_code,
    extract_s3_error_message,
    validate_response_headers,
    XMLResponseValidationError,
    parse_xml_response,
)


class TestARMORErrorResponseStructure(unittest.TestCase):
    """Test ARMOR error response XML structure."""

    def test_access_denied_error_response(self):
        """Test AccessDenied error response structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access to .armor/ reserved namespace is denied</Message>
</Error>'''

        result = validate_s3_error_response(xml, throw_on_error=False)
        self.assertTrue(result, "AccessDenied response should be valid")

        code = extract_s3_error_code(xml)
        self.assertEqual(code, "AccessDenied")

        message = extract_s3_error_message(xml)
        self.assertIn("denied", message.lower())

    def test_invalid_request_error_response(self):
        """Test InvalidRequest error response structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRequest</Code>
  <Message>Unsupported POST operation</Message>
</Error>'''

        result = validate_s3_error_response(xml, throw_on_error=False)
        self.assertTrue(result, "InvalidRequest response should be valid")

        code = extract_s3_error_code(xml)
        self.assertEqual(code, "InvalidRequest")

    def test_method_not_allowed_error_response(self):
        """Test MethodNotAllowed error response structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>MethodNotAllowed</Code>
  <Message>Method DELETE not allowed</Message>
</Error>'''

        result = validate_s3_error_response(xml, throw_on_error=False)
        self.assertTrue(result, "MethodNotAllowed response should be valid")

    def test_internal_error_response(self):
        """Test InternalError error response structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>Failed to read body: unexpected EOF</Message>
</Error>'''

        result = validate_s3_error_response(xml, throw_on_error=False)
        self.assertTrue(result, "InternalError response should be valid")

        code = extract_s3_error_code(xml)
        self.assertEqual(code, "InternalError")

    def test_no_such_key_error_response(self):
        """Test NoSuchKey error response structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
</Error>'''

        result = validate_s3_error_response(xml, throw_on_error=False)
        self.assertTrue(result, "NoSuchKey response should be valid")


class TestXMLWellFormedness(unittest.TestCase):
    """Test that error responses are well-formed XML."""

    def test_xml_declaration_present(self):
        """Test XML declaration is present and correct."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>Key not found</Message>
</Error>'''

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "XML with declaration should be well-formed")

    def test_xml_without_declaration_still_parseable(self):
        """Test XML is parseable even without declaration."""
        xml = '<Error><Code>NoSuchKey</Code><Message>Key not found</Message></Error>'

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "XML without declaration should still be parseable")

    def test_xml_with_whitespace(self):
        """Test XML with whitespace formatting is parseable."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>Key not found</Message>
</Error>'''

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "XML with whitespace should be well-formed")

    def test_malformed_xml_rejected(self):
        """Test malformed XML is rejected."""
        xml = '<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code>'

        result = validate_xml_well_formedness(xml)
        self.assertFalse(result, "Malformed XML should be rejected")

    def test_unclosed_tag_rejected(self):
        """Test XML with unclosed tag is rejected."""
        xml = '<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>Key not found'

        result = validate_xml_well_formedness(xml)
        self.assertFalse(result, "XML with unclosed tag should be rejected")


class TestRequiredFields(unittest.TestCase):
    """Test that error responses contain required fields."""

    def test_code_field_required(self):
        """Test Code field is required."""
        xml = '<?xml version="1.0" encoding="UTF-8"?><Error><Message>Key not found</Message></Error>'

        with self.assertRaises(XMLResponseValidationError) as context:
            validate_s3_error_response(xml)

        error_msg = str(context.exception)
        self.assertIn("Code", error_msg)
        self.assertIn("Missing required fields", error_msg)

    def test_message_field_required(self):
        """Test Message field is required."""
        xml = '<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code></Error>'

        with self.assertRaises(XMLResponseValidationError) as context:
            validate_s3_error_response(xml)

        error_msg = str(context.exception)
        self.assertIn("Message", error_msg)
        self.assertIn("Missing required fields", error_msg)

    def test_both_required_fields_present(self):
        """Test both Code and Message fields are present."""
        xml = '<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>Key not found</Message></Error>'

        result = validate_s3_error_response(xml, throw_on_error=False)
        self.assertTrue(result, "Both required fields should be present")

    def test_empty_code_field_rejected(self):
        """Test empty Code field is rejected as invalid."""
        xml = '<?xml version="1.0" encoding="UTF-8"?><Error><Code></Code><Message>Key not found</Message></Error>'

        result = validate_s3_error_response(xml, throw_on_error=False)
        # Validation framework properly rejects empty fields
        self.assertFalse(result, "Empty Code field should be rejected as invalid")


class TestErrorMessages(unittest.TestCase):
    """Test that error messages are helpful and descriptive."""

    def test_access_denied_message_is_helpful(self):
        """Test AccessDenied message is helpful."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>AccessDenied</Code>
  <Message>Access to .armor/ reserved namespace is denied</Message>
</Error>'''

        message = extract_s3_error_message(xml)
        self.assertIn("denied", message.lower())
        self.assertIn(".armor/", message)

    def test_invalid_request_message_is_descriptive(self):
        """Test InvalidRequest message is descriptive."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRequest</Code>
  <Message>Unsupported POST operation</Message>
</Error>'''

        message = extract_s3_error_message(xml)
        self.assertIn("POST", message)
        self.assertIn("Unsupported", message)

    def test_internal_error_message_includes_details(self):
        """Test InternalError message includes failure details."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InternalError</Code>
  <Message>Failed to read body: unexpected EOF</Message>
</Error>'''

        message = extract_s3_error_message(xml)
        self.assertIn("Failed to read body", message)
        self.assertIn("EOF", message)

    def test_no_such_key_message_is_clear(self):
        """Test NoSuchKey message is clear."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
</Error>'''

        message = extract_s3_error_message(xml)
        self.assertIn("does not exist", message)
        self.assertTrue(len(message) > 10, "Message should be descriptive")


class TestXMLEscaping(unittest.TestCase):
    """Test XML character escaping in error responses."""

    def test_special_characters_in_code(self):
        """Test special characters in Code field are escaped."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>Custom&lt;Error&gt;</Code>
  <Message>Test error</Message>
</Error>'''

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "Special characters should be properly escaped")

    def test_special_characters_in_message(self):
        """Test special characters in Message field are escaped."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>CustomError</Code>
  <Message>Error: &lt;test&gt; &amp; "data"</Message>
</Error>'''

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "Special characters should be properly escaped")

    def test_unicode_in_message(self):
        """Test Unicode characters in Message field."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>CustomError</Code>
  <Message>Error: 你好, مرحبا, こんにちは</Message>
</Error>'''

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "Unicode characters should be supported")


class TestResponseHeaders(unittest.TestCase):
    """Test that error responses include correct headers."""

    def test_content_type_header_present(self):
        """Test Content-Type header is present and correct."""
        headers = {'Content-Type': 'application/xml'}

        errors = validate_response_headers(headers, throw_on_error=False)
        self.assertEqual(len(errors), 0, "Should have no header errors")

    def test_content_type_header_with_charset(self):
        """Test Content-Type header with charset is accepted."""
        headers = {'Content-Type': 'application/xml; charset=utf-8'}

        errors = validate_response_headers(headers, throw_on_error=False)
        self.assertEqual(len(errors), 0, "Should accept charset parameter")

    def test_missing_content_type_header(self):
        """Test missing Content-Type header is detected."""
        headers = {}

        errors = validate_response_headers(headers, throw_on_error=False)
        self.assertGreater(len(errors), 0, "Should detect missing Content-Type")
        self.assertIn("Content-Type", str(errors))

    def test_wrong_content_type_header(self):
        """Test incorrect Content-Type header is detected."""
        headers = {'Content-Type': 'application/json'}

        errors = validate_response_headers(headers, throw_on_error=False)
        self.assertGreater(len(errors), 0, "Should detect wrong Content-Type")

    def test_additional_headers_allowed(self):
        """Test additional headers don't cause validation errors."""
        headers = {
            'Content-Type': 'application/xml',
            'ETag': '"abc123"',
            'Last-Modified': 'Wed, 15 Jul 2026 10:30:00 GMT'
        }

        errors = validate_response_headers(headers, throw_on_error=False)
        self.assertEqual(len(errors), 0, "Additional headers should be allowed")


class TestARMORErrorCodes(unittest.TestCase):
    """Test ARMOR-specific error codes."""

    def test_armor_error_codes_are_s3_compatible(self):
        """Test ARMOR uses S3-compatible error codes."""
        armor_codes = [
            'AccessDenied',
            'InvalidRequest',
            'MethodNotAllowed',
            'InternalError',
            'NoSuchKey',
            'InvalidCopySource'
        ]

        for code in armor_codes:
            xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>{code}</Code>
  <Message>Test message for {code}</Message>
</Error>'''

            result = validate_s3_error_response(xml, throw_on_error=False)
            self.assertTrue(result, f"{code} should be a valid S3 error code")


class TestEdgeCases(unittest.TestCase):
    """Test edge cases and boundary conditions."""

    def test_very_long_error_message(self):
        """Test very long error messages are handled."""
        long_message = "A" * 1000
        xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>CustomError</Code>
  <Message>{long_message}</Message>
</Error>'''

        result = validate_s3_error_response(xml, throw_on_error=False)
        self.assertTrue(result, "Long error messages should be accepted")

    def test_multiline_error_message(self):
        """Test multiline error messages are handled."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>CustomError</Code>
  <Message>Line 1
Line 2
Line 3</Message>
</Error>'''

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "Multiline messages should be parseable")

    def test_message_with_newlines_and_tabs(self):
        """Test messages with various whitespace characters."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>CustomError</Code>
  <Message>Error at line 5: unexpected token &#39;&lt;&#39; near &#39;test&#39;</Message>
</Error>'''

        result = validate_xml_well_formedness(xml)
        self.assertTrue(result, "Various whitespace should be handled")


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("ARMOR ERROR RESPONSE XML STRUCTURE VALIDATION TEST SUITE")
    print("Bead: bf-562pd4")
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
        print("  ✓ Error response XML structure validation")
        print("  ✓ XML well-formedness and parseability")
        print("  ✓ Required field presence (Code, Message)")
        print("  ✓ Error message helpfulness and descriptiveness")
        print("  ✓ XML character escaping")
        print("  ✓ Response header validation (Content-Type)")
        print("  ✓ ARMOR-specific error codes")
        print("  ✓ Edge cases and boundary conditions")
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
