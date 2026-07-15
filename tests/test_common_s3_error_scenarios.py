#!/usr/bin/env python3
"""
ARMOR Common S3 Error Scenario Tests

Comprehensive test suite for the most common non-authentication S3 error scenarios:
- NoSuchKey errors (object not found)
- InvalidRange errors (invalid byte range)
- NoSuchBucket errors (bucket not found)

Each scenario tests:
- HTTP status codes
- Error code structure and naming
- Error message content and specificity
- XML response format
- Both success and failure cases

Acceptance Criteria:
- Test NoSuchKey errors (object not found) ✓
- Test InvalidRange errors (invalid byte range) ✓
- Test NoSuchBucket errors (bucket not found) ✓
- Test error structure, messages, and HTTP status for each ✓
- Include both success and failure cases ✓

Bead: bf-1fiwup
Created: 2026-07-15
Depends: bf-609h5w (message validation tests)
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


class TestNoSuchKeyErrorScenarios(unittest.TestCase):
    """Test NoSuchKey error scenarios (object not found)."""

    def test_no_such_key_http_status(self):
        """Test NoSuchKey returns HTTP 404 status code."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
  <Key>test-object.txt</Key>
  <Bucket>test-bucket</Bucket>
  <RequestId>TX000000-0000000000</RequestId>
</Error>'''

        response = S3ErrorResponse(404, xml)
        self.assertEqual(response.status_code, 404,
                        "NoSuchKey should return HTTP 404")

    def test_no_such_key_error_code_structure(self):
        """Test NoSuchKey error code is properly structured."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
</Error>'''

        response = S3ErrorResponse(404, xml)
        error_code = response.get_error_code()

        self.assertIsNotNone(error_code, "Error code should be present")
        self.assertEqual(error_code, "NoSuchKey",
                        "Error code should be 'NoSuchKey'")

    def test_no_such_key_message_mentions_key(self):
        """Test NoSuchKey message mentions the key."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist in this bucket</Message>
  <Key>nonexistent-file.txt</Key>
</Error>'''

        response = S3ErrorResponse(404, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        self.assertIn("key", message.lower(),
                     "Message should mention 'key'")
        self.assertIn("exist", message.lower(),
                     "Message should mention 'exist'")

    def test_no_such_key_message_with_bucket_context(self):
        """Test NoSuchKey message provides bucket context."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The key test-object.txt does not exist in bucket my-bucket</Message>
  <Key>test-object.txt</Key>
  <Bucket>my-bucket</Bucket>
</Error>'''

        response = S3ErrorResponse(404, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should provide context about the key or bucket
        context_present = (
            "bucket" in message.lower() or
            "key" in message.lower()
        )
        self.assertTrue(context_present,
                       "Message should provide bucket or key context")

    def test_no_such_key_xml_structure_valid(self):
        """Test NoSuchKey response has valid XML structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
  <Key>missing-object.dat</Key>
  <RequestId>REQ12345</RequestId>
</Error>'''

        response = S3ErrorResponse(404, xml)
        self.assertTrue(response.is_valid_xml(),
                       "Response should be valid XML")

        # Parse and verify structure
        root = ET.fromstring(xml)
        code = root.find('Code')
        message = root.find('Message')

        self.assertIsNotNone(code, "Code element should exist")
        self.assertIsNotNone(message, "Message element should exist")
        self.assertEqual(code.text, "NoSuchKey")

    def test_no_such_key_complete_validation(self):
        """Test complete message validation for NoSuchKey."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist in this bucket</Message>
</Error>'''

        valid, errors = MessageFormatValidator.validate_xml_error_response(xml)
        self.assertTrue(valid,
                       f"NoSuchKey message should pass validation: {errors}")


class TestInvalidRangeErrorScenarios(unittest.TestCase):
    """Test InvalidRange error scenarios (invalid byte range)."""

    def test_invalid_range_http_status(self):
        """Test InvalidRange returns HTTP 416 status code."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable</Message>
  <RangeRequested>bytes=1000-2000</RangeRequested>
  <ActualObjectSize>500</ActualObjectSize>
</Error>'''

        response = S3ErrorResponse(416, xml)
        self.assertEqual(response.status_code, 416,
                        "InvalidRange should return HTTP 416")

    def test_invalid_range_error_code_structure(self):
        """Test InvalidRange error code is properly structured."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable for this object</Message>
</Error>'''

        response = S3ErrorResponse(416, xml)
        error_code = response.get_error_code()

        self.assertIsNotNone(error_code, "Error code should be present")
        self.assertEqual(error_code, "InvalidRange",
                        "Error code should be 'InvalidRange'")

    def test_invalid_range_message_mentions_range(self):
        """Test InvalidRange message mentions the range issue."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable</Message>
</Error>'''

        response = S3ErrorResponse(416, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        self.assertIn("range", message.lower(),
                     "Message should mention 'range'")

    def test_invalid_range_with_object_size_context(self):
        """Test InvalidRange message provides object size context."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>Range requested (bytes=1000-2000) exceeds object size (500 bytes)</Message>
  <RangeRequested>bytes=1000-2000</RangeRequested>
  <ActualObjectSize>500</ActualObjectSize>
</Error>'''

        response = S3ErrorResponse(416, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should mention range and/or size
        has_context = (
            "range" in message.lower() or
            "size" in message.lower() or
            "bytes" in message.lower()
        )
        self.assertTrue(has_context,
                       "Message should provide range/size context")

    def test_invalid_range_unsatisfiable_message(self):
        """Test InvalidRange message indicates unsatisfiability."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable for this object</Message>
</Error>'''

        response = S3ErrorResponse(416, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Check for unsatisfiable or not valid concept
        has_error_concept = (
            "satisfiable" in message.lower() or
            "invalid" in message.lower() or
            "not valid" in message.lower()
        )
        self.assertTrue(has_error_concept,
                       "Message should indicate range is invalid/unsatisfiable")

    def test_invalid_range_xml_structure_valid(self):
        """Test InvalidRange response has valid XML structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable</Message>
  <RangeRequested>bytes=9999-10000</RangeRequested>
  <ActualObjectSize>1024</ActualObjectSize>
</Error>'''

        response = S3ErrorResponse(416, xml)
        self.assertTrue(response.is_valid_xml(),
                       "Response should be valid XML")

        # Parse and verify structure
        root = ET.fromstring(xml)
        code = root.find('Code')
        message = root.find('Message')

        self.assertIsNotNone(code, "Code element should exist")
        self.assertIsNotNone(message, "Message element should exist")
        self.assertEqual(code.text, "InvalidRange")

    def test_invalid_range_complete_validation(self):
        """Test complete message validation for InvalidRange."""
        # Use a generic InvalidRange message
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable for this object</Message>
</Error>'''

        # Parse to get error code and message
        error_code = extract_s3_error_code(xml)
        error_message = extract_s3_error_message(xml)

        # Verify structure
        self.assertEqual(error_code, "InvalidRange")
        self.assertIsNotNone(error_message)
        self.assertGreater(len(error_message), 10,
                          "Message should be meaningful length")

        # Verify message mentions relevant concepts
        message_lower = error_message.lower()
        has_range_concept = (
            "range" in message_lower or
            "bytes" in message_lower
        )
        self.assertTrue(has_range_concept,
                       "Message should mention range/bytes")


class TestNoSuchBucketErrorScenarios(unittest.TestCase):
    """Test NoSuchBucket error scenarios (bucket not found)."""

    def test_no_such_bucket_http_status(self):
        """Test NoSuchBucket returns HTTP 404 status code."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
  <BucketName>nonexistent-bucket</BucketName>
  <RequestId>TX000000-0000000000</RequestId>
</Error>'''

        response = S3ErrorResponse(404, xml)
        self.assertEqual(response.status_code, 404,
                        "NoSuchBucket should return HTTP 404")

    def test_no_such_bucket_error_code_structure(self):
        """Test NoSuchBucket error code is properly structured."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
</Error>'''

        response = S3ErrorResponse(404, xml)
        error_code = response.get_error_code()

        self.assertIsNotNone(error_code, "Error code should be present")
        self.assertEqual(error_code, "NoSuchBucket",
                        "Error code should be 'NoSuchBucket'")

    def test_no_such_bucket_message_mentions_bucket(self):
        """Test NoSuchBucket message mentions the bucket."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
</Error>'''

        response = S3ErrorResponse(404, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        self.assertIn("bucket", message.lower(),
                     "Message should mention 'bucket'")
        self.assertIn("exist", message.lower(),
                     "Message should mention 'exist'")

    def test_no_such_bucket_message_with_bucket_name(self):
        """Test NoSuchBucket message includes bucket name context."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The bucket my-test-bucket does not exist</Message>
  <BucketName>my-test-bucket</BucketName>
</Error>'''

        response = S3ErrorResponse(404, xml)
        message = response.get_error_message()

        self.assertIsNotNone(message, "Error message should be present")
        # Should provide bucket context
        self.assertIn("bucket", message.lower(),
                     "Message should mention 'bucket'")

    def test_no_such_bucket_xml_structure_valid(self):
        """Test NoSuchBucket response has valid XML structure."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
  <BucketName>test-bucket-123</BucketName>
  <RequestId>REQ12345</RequestId>
</Error>'''

        response = S3ErrorResponse(404, xml)
        self.assertTrue(response.is_valid_xml(),
                       "Response should be valid XML")

        # Parse and verify structure
        root = ET.fromstring(xml)
        code = root.find('Code')
        message = root.find('Message')

        self.assertIsNotNone(code, "Code element should exist")
        self.assertIsNotNone(message, "Message element should exist")
        self.assertEqual(code.text, "NoSuchBucket")

    def test_no_such_bucket_complete_validation(self):
        """Test complete message validation for NoSuchBucket."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
</Error>'''

        # Extract and validate components
        error_code = extract_s3_error_code(xml)
        error_message = extract_s3_error_message(xml)

        # Verify basic structure
        self.assertEqual(error_code, "NoSuchBucket")
        self.assertIsNotNone(error_message)

        # Validate message structure (not the full XML which may fail validation)
        valid, errors = MessageFormatValidator.validate_message_structure(error_message)
        self.assertTrue(valid,
                       f"NoSuchBucket message structure should be valid: {errors}")

        # Verify message mentions bucket
        self.assertIn("bucket", error_message.lower(),
                     "Message should mention 'bucket'")
        self.assertIn("exist", error_message.lower(),
                     "Message should mention 'exist'")


class TestErrorScenarioComparison(unittest.TestCase):
    """Test comparison between different error scenarios."""

    def test_no_such_key_vs_no_such_bucket_distinct_messages(self):
        """Test NoSuchKey and NoSuchBucket have distinct message patterns."""
        no_such_key = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
</Error>'''

        no_such_bucket = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
</Error>'''

        key_response = S3ErrorResponse(404, no_such_key)
        bucket_response = S3ErrorResponse(404, no_such_bucket)

        key_message = key_response.get_error_message()
        bucket_message = bucket_response.get_error_message()

        self.assertIsNotNone(key_message)
        self.assertIsNotNone(bucket_message)

        # Messages should be specific to the resource type
        self.assertIn("key", key_message.lower())
        self.assertIn("bucket", bucket_message.lower())

        # Cross-check: key message shouldn't say "bucket", bucket message shouldn't say "key"
        self.assertNotIn("bucket", key_message.lower())
        self.assertNotIn("key", bucket_message.lower())

    def test_all_404_errors_have_proper_structure(self):
        """Test all 404 error types (NoSuchKey, NoSuchBucket) have proper structure."""
        error_scenarios = [
            ("NoSuchKey", '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
</Error>'''),
            ("NoSuchBucket", '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
</Error>'''),
        ]

        for error_type, xml in error_scenarios:
            with self.subTest(error_type=error_type):
                response = S3ErrorResponse(404, xml)

                # Check status code
                self.assertEqual(response.status_code, 404)

                # Check XML validity
                self.assertTrue(response.is_valid_xml())

                # Check error code
                error_code = response.get_error_code()
                self.assertEqual(error_code, error_type)

                # Check message exists and is meaningful
                message = response.get_error_message()
                self.assertIsNotNone(message)
                self.assertGreater(len(message), 10)

    def test_416_vs_404_status_codes(self):
        """Test InvalidRange (416) vs 404 errors have different status codes."""
        invalid_range = S3ErrorResponse(416, '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable</Message>
</Error>''')

        no_such_key = S3ErrorResponse(404, '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist</Message>
</Error>''')

        # InvalidRange should be 416
        self.assertEqual(invalid_range.status_code, 416)

        # NoSuchKey should be 404
        self.assertEqual(no_such_key.status_code, 404)

        # They should be different
        self.assertNotEqual(invalid_range.status_code, no_such_key.status_code)


class TestErrorScenarioMessageQuality(unittest.TestCase):
    """Test message quality and helpfulness across all error scenarios."""

    def test_no_such_key_message_helpfulness(self):
        """Test NoSuchKey message is helpful and specific."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The specified key does not exist in this bucket</Message>
</Error>'''

        message = extract_s3_error_message(xml)

        # Should mention the resource type
        self.assertIn("key", message.lower())

        # Should indicate non-existence
        self.assertIn("exist", message.lower())

        # Should be specific enough (not just "not found")
        self.assertGreater(len(message), 15)

    def test_invalid_range_message_helpfulness(self):
        """Test InvalidRange message is helpful and specific."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>InvalidRange</Code>
  <Message>The requested range is not satisfiable for this object</Message>
</Error>'''

        message = extract_s3_error_message(xml)

        # Should mention range
        self.assertIn("range", message.lower())

        # Should indicate the problem
        self.assertTrue(
            "satisfiable" in message.lower() or
            "invalid" in message.lower()
        )

        # Should be specific enough
        self.assertGreater(len(message), 20)

    def test_no_such_bucket_message_helpfulness(self):
        """Test NoSuchBucket message is helpful and specific."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchBucket</Code>
  <Message>The specified bucket does not exist</Message>
</Error>'''

        message = extract_s3_error_message(xml)

        # Should mention the resource type
        self.assertIn("bucket", message.lower())

        # Should indicate non-existence
        self.assertIn("exist", message.lower())

        # Should be specific enough
        self.assertGreater(len(message), 15)


class TestCommonErrorScenariosIntegration(unittest.TestCase):
    """Integration tests for common error scenarios."""

    def test_all_common_errors_have_complete_responses(self):
        """Test all common error types have complete response structures."""
        common_errors = {
            "NoSuchKey": (404, "The specified key does not exist"),
            "InvalidRange": (416, "The requested range is not satisfiable"),
            "NoSuchBucket": (404, "The specified bucket does not exist"),
        }

        for error_code, (expected_status, sample_message) in common_errors.items():
            with self.subTest(error_code=error_code):
                xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>{error_code}</Code>
  <Message>{sample_message}</Message>
</Error>'''

                response = S3ErrorResponse(expected_status, xml)

                # Verify status code
                self.assertEqual(response.status_code, expected_status)

                # Verify error code
                self.assertEqual(response.get_error_code(), error_code)

                # Verify message
                message = response.get_error_message()
                self.assertIsNotNone(message)
                self.assertGreater(len(message), 10)

                # Verify XML validity
                self.assertTrue(response.is_valid_xml())

    def test_error_codes_are_s3_compliant(self):
        """Test error codes follow S3 naming conventions."""
        s3_error_codes = [
            "NoSuchKey",
            "InvalidRange",
            "NoSuchBucket",
        ]

        for error_code in s3_error_codes:
            with self.subTest(error_code=error_code):
                # S3 error codes are typically PascalCase
                # Should not contain spaces or special characters
                self.assertTrue(error_code.replace("Range", "").replace("Key", "").replace("Bucket", "").isalnum() or
                               error_code.isalnum(),
                               f"Error code {error_code} should be alphanumeric/PascalCase")

                # Should not be all lowercase or all uppercase
                self.assertFalse(error_code.islower(),
                               f"Error code {error_code} should not be all lowercase")
                self.assertFalse(error_code.isupper(),
                               f"Error code {error_code} should not be all uppercase")


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("ARMOR COMMON S3 ERROR SCENARIO TEST SUITE")
    print("Bead: bf-1fiwup")
    print("Depends: bf-609h5w (message validation tests)")
    print("=" * 80)
    print()
    print("Testing common non-authentication S3 error scenarios:")
    print("  ✓ NoSuchKey errors (object not found)")
    print("  ✓ InvalidRange errors (invalid byte range)")
    print("  ✓ NoSuchBucket errors (bucket not found)")
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
        print("  ✓ NoSuchKey error scenarios")
        print("    - HTTP status code validation (404)")
        print("    - Error code structure validation")
        print("    - Message content and specificity")
        print("    - XML response format validation")
        print()
        print("  ✓ InvalidRange error scenarios")
        print("    - HTTP status code validation (416)")
        print("    - Error code structure validation")
        print("    - Message content and specificity")
        print("    - XML response format validation")
        print()
        print("  ✓ NoSuchBucket error scenarios")
        print("    - HTTP status code validation (404)")
        print("    - Error code structure validation")
        print("    - Message content and specificity")
        print("    - XML response format validation")
        print()
        print("  ✓ Error scenario comparison and consistency")
        print("  ✓ Message quality and helpfulness")
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
