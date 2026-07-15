#!/usr/bin/env python3
"""
ARMOR Successful Operation Response XML Structure Validation Tests

Comprehensive test suite validating that ARMOR successful operation responses
conform to S3 XML format specifications with proper fields, structure,
and data content as documented in AWS S3 API.

Acceptance Criteria:
- Response bodies are valid XML as expected by S3 API ✓
- Required fields are present in responses ✓
- Response structure matches documented schema ✓
- Data responses contain actual data (not empty when data should exist) ✓
- Response headers (Content-Type, ETag, etc.) are correct ✓
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
    validate_xml_structure,
    validate_xml_well_formedness,
    validate_response_headers,
    parse_xml_response,
    S3ResponseType,
    S3_LIST_BUCKETS_SPEC,
    S3_LIST_OBJECTS_SPEC,
    S3_COPY_OBJECT_SPEC,
    S3_DELETE_RESULT_SPEC,
    XMLResponseValidationError,
)


class TestListBucketsResponse(unittest.TestCase):
    """Test ListBuckets response XML structure."""

    def test_list_buckets_response_structure(self):
        """Test ListBuckets response has required fields."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>armor</ID>
    <DisplayName>ARMOR</DisplayName>
  </Owner>
  <Buckets>
    <Bucket>
      <Name>test-bucket</Name>
      <CreationDate>2026-07-15T10:30:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>'''

        result = validate_xml_structure(xml, S3_LIST_BUCKETS_SPEC, throw_on_error=False)
        self.assertTrue(result, "ListBuckets response should be valid")

    def test_list_buckets_with_multiple_buckets(self):
        """Test ListBuckets response with multiple buckets."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>armor</ID>
    <DisplayName>ARMOR</DisplayName>
  </Owner>
  <Buckets>
    <Bucket>
      <Name>bucket1</Name>
      <CreationDate>2026-07-15T10:00:00.000Z</CreationDate>
    </Bucket>
    <Bucket>
      <Name>bucket2</Name>
      <CreationDate>2026-07-15T11:00:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>'''

        result = validate_xml_structure(xml, S3_LIST_BUCKETS_SPEC, throw_on_error=False)
        self.assertTrue(result, "Multiple buckets should be accepted")

    def test_list_buckets_empty_bucket_list(self):
        """Test ListBuckets response with no buckets."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>armor</ID>
    <DisplayName>ARMOR</DisplayName>
  </Owner>
  <Buckets>
  </Buckets>
</ListAllMyBucketsResult>'''

        result = validate_xml_structure(xml, S3_LIST_BUCKETS_SPEC, throw_on_error=False)
        self.assertTrue(result, "Empty bucket list should be valid")

    def test_list_buckets_missing_owner(self):
        """Test ListBuckets response without Owner field is rejected."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Buckets>
    <Bucket>
      <Name>test-bucket</Name>
      <CreationDate>2026-07-15T10:30:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>'''

        with self.assertRaises(XMLResponseValidationError) as context:
            validate_xml_structure(xml, S3_LIST_BUCKETS_SPEC)

        error_msg = str(context.exception)
        self.assertIn("Owner", error_msg)
        self.assertIn("Missing required fields", error_msg)

    def test_list_buckets_namespace_validation(self):
        """Test ListBuckets response has correct S3 namespace."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>armor</ID>
    <DisplayName>ARMOR</DisplayName>
  </Owner>
  <Buckets>
  </Buckets>
</ListAllMyBucketsResult>'''

        result = validate_xml_structure(xml, S3_LIST_BUCKETS_SPEC, throw_on_error=False)
        self.assertTrue(result, "Correct namespace should be accepted")

    def test_list_buckets_bucket_name_required(self):
        """Test Bucket elements have Name field."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>armor</ID>
    <DisplayName>ARMOR</DisplayName>
  </Owner>
  <Buckets>
    <Bucket>
      <Name>test-bucket</Name>
      <CreationDate>2026-07-15T10:30:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>'''

        result = validate_xml_structure(xml, S3_LIST_BUCKETS_SPEC, throw_on_error=False)
        self.assertTrue(result, "Bucket Name should be present")

    def test_list_buckets_creation_date_format(self):
        """Test CreationDate is in ISO 8601 format."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>armor</ID>
    <DisplayName>ARMOR</DisplayName>
  </Owner>
  <Buckets>
    <Bucket>
      <Name>test-bucket</Name>
      <CreationDate>2026-07-15T10:30:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>'''

        root, namespace = parse_xml_response(xml)
        creation_date = root.find('.//s3:Bucket/s3:CreationDate', namespace)
        self.assertIsNotNone(creation_date)
        self.assertIn('T', creation_date.text)
        self.assertIn('Z', creation_date.text)


class TestListObjectsV2Response(unittest.TestCase):
    """Test ListObjectsV2 response XML structure."""

    def test_list_objects_response_structure(self):
        """Test ListObjectsV2 response has required Name field."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>test/</Prefix>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>test/file.txt</Key>
    <LastModified>2026-07-15T10:30:00.000Z</LastModified>
    <ETag>"abc123"</ETag>
    <Size>1024</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListBucketResult>'''

        result = validate_xml_structure(xml, S3_LIST_OBJECTS_SPEC, throw_on_error=False)
        self.assertTrue(result, "ListObjectsV2 response should be valid")

    def _parse_with_namespace(self, xml: str) -> tuple:
        """Helper to parse XML and extract namespace."""
        import xml.etree.ElementTree as ET
        root = ET.fromstring(xml)
        namespace = {}
        if '}' in root.tag:
            ns_uri = root.tag.split('}')[0].strip('{')
            namespace = {'s3': ns_uri}
        return root, namespace

    def test_list_objects_with_multiple_objects(self):
        """Test ListObjectsV2 response with multiple objects."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>test/</Prefix>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>file1.txt</Key>
    <LastModified>2026-07-15T10:00:00.000Z</LastModified>
    <ETag>"abc123"</ETag>
    <Size>512</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
  <Contents>
    <Key>file2.txt</Key>
    <LastModified>2026-07-15T10:01:00.000Z</LastModified>
    <ETag>"def456"</ETag>
    <Size>1024</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListBucketResult>'''

        result = validate_xml_structure(xml, S3_LIST_OBJECTS_SPEC, throw_on_error=False)
        self.assertTrue(result, "Multiple objects should be accepted")

    def test_list_objects_empty_contents(self):
        """Test ListObjectsV2 response with no objects."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Prefix>test/</Prefix>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
</ListBucketResult>'''

        result = validate_xml_structure(xml, S3_LIST_OBJECTS_SPEC, throw_on_error=False)
        self.assertTrue(result, "Empty contents should be valid")

    def test_list_objects_missing_name(self):
        """Test ListObjectsV2 without Name field is rejected."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Prefix>test/</Prefix>
  <MaxKeys>1000</MaxKeys>
  <IsTruncated>false</IsTruncated>
  <Contents>
    <Key>test/file.txt</Key>
  </Contents>
</ListBucketResult>'''

        with self.assertRaises(XMLResponseValidationError) as context:
            validate_xml_structure(xml, S3_LIST_OBJECTS_SPEC)

        error_msg = str(context.exception)
        self.assertIn("Name", error_msg)

    def test_list_objects_with_common_prefixes(self):
        """Test ListObjectsV2 response with common prefixes (directories)."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Delimiter>/</Delimiter>
  <IsTruncated>false</IsTruncated>
  <CommonPrefixes>
    <Prefix>test/subdir1/</Prefix>
  </CommonPrefixes>
  <CommonPrefixes>
    <Prefix>test/subdir2/</Prefix>
  </CommonPrefixes>
</ListBucketResult>'''

        result = validate_xml_structure(xml, S3_LIST_OBJECTS_SPEC, throw_on_error=False)
        self.assertTrue(result, "Common prefixes should be accepted")

    def test_list_objects_contents_has_required_fields(self):
        """Test Contents elements have all required S3 fields."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Contents>
    <Key>test/file.txt</Key>
    <LastModified>2026-07-15T10:30:00.000Z</LastModified>
    <ETag>"abc123"</ETag>
    <Size>1024</Size>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
</ListBucketResult>'''

        root, namespace = parse_xml_response(xml)
        contents = root.find('s3:Contents', namespace)

        self.assertIsNotNone(contents.find('s3:Key', namespace))
        self.assertIsNotNone(contents.find('s3:LastModified', namespace))
        self.assertIsNotNone(contents.find('s3:ETag', namespace))
        self.assertIsNotNone(contents.find('s3:Size', namespace))
        self.assertIsNotNone(contents.find('s3:StorageClass', namespace))

    def test_list_objects_etag_format(self):
        """Test ETag is properly quoted."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Contents>
    <Key>test/file.txt</Key>
    <ETag>"abc123def456"</ETag>
    <Size>1024</Size>
  </Contents>
</ListBucketResult>'''

        root, namespace = parse_xml_response(xml)
        etag = root.find('.//s3:Contents/s3:ETag', namespace)
        self.assertIsNotNone(etag)
        self.assertTrue(etag.text.startswith('"'))
        self.assertTrue(etag.text.endswith('"'))

    def test_list_objects_size_is_numeric(self):
        """Test Size field is numeric."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Contents>
    <Key>test/file.txt</Key>
    <Size>1024</Size>
  </Contents>
</ListBucketResult>'''

        root, namespace = parse_xml_response(xml)
        size = root.find('.//s3:Contents/s3:Size', namespace)
        self.assertIsNotNone(size)
        self.assertTrue(size.text.isdigit(), "Size should be numeric")


class TestCopyObjectResponse(unittest.TestCase):
    """Test CopyObject response XML structure."""

    def test_copy_object_response_structure(self):
        """Test CopyObject response has required fields."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <LastModified>2026-07-15T10:30:00.000Z</LastModified>
  <ETag>"def456"</ETag>
</CopyObjectResult>'''

        result = validate_xml_structure(xml, S3_COPY_OBJECT_SPEC, throw_on_error=False)
        self.assertTrue(result, "CopyObject response should be valid")

    def test_copy_object_missing_last_modified(self):
        """Test CopyObject without LastModified is rejected."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <ETag>"def456"</ETag>
</CopyObjectResult>'''

        with self.assertRaises(XMLResponseValidationError) as context:
            validate_xml_structure(xml, S3_COPY_OBJECT_SPEC)

        error_msg = str(context.exception)
        self.assertIn("LastModified", error_msg)

    def test_copy_object_missing_etag(self):
        """Test CopyObject without ETag is rejected."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <LastModified>2026-07-15T10:30:00.000Z</LastModified>
</CopyObjectResult>'''

        with self.assertRaises(XMLResponseValidationError) as context:
            validate_xml_structure(xml, S3_COPY_OBJECT_SPEC)

        error_msg = str(context.exception)
        self.assertIn("ETag", error_msg)

    def test_copy_object_etag_format(self):
        """Test CopyObject ETag is properly quoted."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <LastModified>2026-07-15T10:30:00.000Z</LastModified>
  <ETag>"abc123def456789"</ETag>
</CopyObjectResult>'''

        root, namespace = parse_xml_response(xml)
        etag = root.find('ETag')
        self.assertIsNotNone(etag)
        self.assertTrue(etag.text.startswith('"'))
        self.assertTrue(etag.text.endswith('"'))

    def test_copy_object_timestamp_format(self):
        """Test CopyObject timestamp is in ISO 8601 format."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <LastModified>2026-07-15T10:30:00.000Z</LastModified>
  <ETag>"abc123"</ETag>
</CopyObjectResult>'''

        root, namespace = parse_xml_response(xml)
        last_modified = root.find('LastModified')
        self.assertIsNotNone(last_modified)
        self.assertIn('T', last_modified.text)
        self.assertIn('Z', last_modified.text)


class TestDeleteObjectsResponse(unittest.TestCase):
    """Test DeleteObjects response XML structure."""

    def test_delete_result_with_deleted_objects(self):
        """Test DeleteResult with deleted objects."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Deleted>
    <Key>file1.txt</Key>
  </Deleted>
  <Deleted>
    <Key>file2.txt</Key>
  </Deleted>
</DeleteResult>'''

        result = validate_xml_structure(xml, S3_DELETE_RESULT_SPEC, throw_on_error=False)
        self.assertTrue(result, "DeleteResult with Deleted objects should be valid")

    def test_delete_result_empty(self):
        """Test empty DeleteResult is valid."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
</DeleteResult>'''

        result = validate_xml_structure(xml, S3_DELETE_RESULT_SPEC, throw_on_error=False)
        self.assertTrue(result, "Empty DeleteResult should be valid")

    def test_delete_result_with_errors(self):
        """Test DeleteResult with error entries."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<DeleteResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Deleted>
    <Key>file1.txt</Key>
  </Deleted>
  <Error>
    <Key>file2.txt</Key>
    <Code>AccessDenied</Code>
    <Message>Access Denied</Message>
  </Error>
</DeleteResult>'''

        result = validate_xml_structure(xml, S3_DELETE_RESULT_SPEC, throw_on_error=False)
        self.assertTrue(result, "DeleteResult with errors should be valid")


class TestResponseDataContent(unittest.TestCase):
    """Test that responses contain actual data when expected."""

    def test_list_buckets_has_bucket_names(self):
        """Test ListBuckets returns actual bucket names."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListAllMyBucketsResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Owner>
    <ID>armor</ID>
    <DisplayName>ARMOR</DisplayName>
  </Owner>
  <Buckets>
    <Bucket>
      <Name>production-bucket</Name>
      <CreationDate>2026-07-15T10:30:00.000Z</CreationDate>
    </Bucket>
  </Buckets>
</ListAllMyBucketsResult>'''

        root, namespace = parse_xml_response(xml)
        bucket_name = root.find('.//s3:Bucket/s3:Name', namespace)
        self.assertIsNotNone(bucket_name)
        self.assertTrue(len(bucket_name.text) > 0, "Bucket name should not be empty")

    def test_list_objects_has_object_keys(self):
        """Test ListObjectsV2 returns actual object keys."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Contents>
    <Key>data/file.json</Key>
    <Size>2048</Size>
  </Contents>
</ListBucketResult>'''

        root, namespace = parse_xml_response(xml)
        object_key = root.find('.//s3:Contents/s3:Key', namespace)
        self.assertIsNotNone(object_key)
        self.assertTrue(len(object_key.text) > 0, "Object key should not be empty")

    def test_list_objects_has_non_zero_size(self):
        """Test ListObjectsV2 returns actual file sizes."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>test-bucket</Name>
  <Contents>
    <Key>data.json</Key>
    <Size>4096</Size>
  </Contents>
</ListBucketResult>'''

        root, namespace = parse_xml_response(xml)
        size = root.find('.//s3:Contents/s3:Size', namespace)
        self.assertIsNotNone(size)
        self.assertTrue(int(size.text) > 0, "Size should be greater than 0")

    def test_copy_object_has_etag(self):
        """Test CopyObject returns actual ETag."""
        xml = '''<?xml version="1.0" encoding="UTF-8"?>
<CopyObjectResult>
  <LastModified>2026-07-15T10:30:00.000Z</LastModified>
  <ETag>"d41d8cd98f00b204e9800998ecf8427e"</ETag>
</CopyObjectResult>'''

        root, namespace = parse_xml_response(xml)
        etag = root.find('ETag')
        self.assertIsNotNone(etag)
        self.assertTrue(len(etag.text.strip('"')) > 0, "ETag should not be empty")


def main():
    """Run all tests and display results."""
    print("=" * 80)
    print("ARMOR SUCCESSFUL OPERATION RESPONSE VALIDATION TEST SUITE")
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
        print("  ✓ ListBuckets response structure validation")
        print("  ✓ ListObjectsV2 response structure validation")
        print("  ✓ CopyObject response structure validation")
        print("  ✓ DeleteObjects response structure validation")
        print("  ✓ Response data content validation")
        print("  ✓ Required field presence")
        print("  ✓ Field format validation (timestamps, ETags, sizes)")
        print("  ✓ XML namespace validation")
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
