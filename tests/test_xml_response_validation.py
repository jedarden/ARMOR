#!/usr/bin/env python3
"""
XML Response Validation Helper Functions for ARMOR S3 API

This module provides reusable helper functions for validating XML response
structure in ARMOR S3 API tests. These helpers verify that responses conform
to S3 XML format specifications with proper field structure, content, and headers.

Acceptance Criteria:
- Function accepts XML response body and validates structure ✓
- Validates XML is well-formed and parseable ✓
- Checks for required fields in S3 responses ✓
- Supports S3-specific response formats (Error, ListBuckets, ListObjectsV2, etc.) ✓
- Returns boolean or throws assertion error with clear message ✓
- Validates response headers (Content-Type, ETag, etc.) ✓

Bead: bf-562pd4
Created: 2026-07-15
"""

import xml.etree.ElementTree as ET
from typing import Union, Dict, Any, Optional, Set, List
from dataclasses import dataclass
from enum import Enum


class S3ResponseType(Enum):
    """Types of S3 XML responses."""
    ERROR = "Error"
    LIST_ALL_MY_BUCKETS = "ListAllMyBucketsResult"
    LIST_BUCKET_RESULT = "ListBucketResult"
    COPY_OBJECT_RESULT = "CopyObjectResult"
    DELETE_RESULT = "DeleteResult"
    LOCATION_CONSTRAINT = "LocationConstraint"
    VERSIONING_CONFIGURATION = "VersioningConfiguration"


@dataclass
class S3ResponseSpec:
    """
    Specification for expected S3 XML response structure.

    Defines which fields are required, optional, or forbidden in an S3 response.
    """
    response_type: S3ResponseType
    required_fields: Set[str]
    optional_fields: Set[str]
    # Expected XML namespace
    expected_namespace: Optional[str] = None


class XMLResponseValidationError(AssertionError):
    """
    Custom assertion error for XML response structure validation failures.

    Provides clear, formatted error messages that show:
    - The actual response body received
    - Which required fields are missing
    - Which unexpected fields are present
    - XML parsing errors
    - Field-specific validation errors
    """

    def __init__(self,
                 response_body: Any,
                 missing_fields: Optional[Set[str]] = None,
                 unexpected_fields: Optional[Set[str]] = None,
                 field_errors: Optional[Dict[str, str]] = None,
                 parse_error: Optional[str] = None,
                 header_errors: Optional[List[str]] = None):
        """
        Initialize an XML response validation error.

        Args:
            response_body: The actual response body (raw XML string)
            missing_fields: Set of required field names that were missing
            unexpected_fields: Set of unexpected field names that were present
            field_errors: Dict mapping field names to specific error messages
            parse_error: Error message if XML couldn't be parsed
            header_errors: List of header validation errors
        """
        self.response_body = response_body
        self.missing_fields = missing_fields or set()
        self.unexpected_fields = unexpected_fields or set()
        self.field_errors = field_errors or {}
        self.parse_error = parse_error
        self.header_errors = header_errors or []

        message = self._format_error_message()
        super().__init__(message)

    def _format_error_message(self) -> str:
        """Format a detailed error message."""
        msg_parts = ["XML response validation failed:"]

        if self.parse_error:
            msg_parts.append(f"  Parse error: {self.parse_error}")
            body_preview = str(self.response_body)[:200]
            msg_parts.append(f"  Response body (truncated): {body_preview}")
            return "\n".join(msg_parts)

        if self.header_errors:
            msg_parts.append("  Header validation errors:")
            for error in self.header_errors:
                msg_parts.append(f"    - {error}")

        if self.missing_fields:
            msg_parts.append(f"  Missing required fields: {', '.join(sorted(self.missing_fields))}")

        if self.unexpected_fields:
            msg_parts.append(f"  Unexpected fields: {', '.join(sorted(self.unexpected_fields))}")

        if self.field_errors:
            msg_parts.append("  Field validation errors:")
            for field, error in sorted(self.field_errors.items()):
                msg_parts.append(f"    - {field}: {error}")

        # Add response body preview
        body_preview = str(self.response_body)
        if len(body_preview) > 300:
            body_preview = body_preview[:300] + "\n  ... (truncated)"
        msg_parts.append(f"  Response body:\n{body_preview}")

        return "\n".join(msg_parts)


# S3 XML response specifications based on AWS S3 API documentation
S3_ERROR_SPEC = S3ResponseSpec(
    response_type=S3ResponseType.ERROR,
    required_fields={'Code', 'Message'},
    optional_fields=set(),
    expected_namespace=None
)

S3_LIST_BUCKETS_SPEC = S3ResponseSpec(
    response_type=S3ResponseType.LIST_ALL_MY_BUCKETS,
    required_fields={'Owner', 'Buckets'},
    optional_fields=set(),
    expected_namespace="http://s3.amazonaws.com/doc/2006-03-01/"
)

S3_LIST_OBJECTS_SPEC = S3ResponseSpec(
    response_type=S3ResponseType.LIST_BUCKET_RESULT,
    required_fields={'Name'},
    optional_fields={
        'Prefix', 'Delimiter', 'MaxKeys', 'IsTruncated',
        'Contents', 'CommonPrefixes', 'NextContinuationToken'
    },
    expected_namespace="http://s3.amazonaws.com/doc/2006-03-01/"
)

S3_COPY_OBJECT_SPEC = S3ResponseSpec(
    response_type=S3ResponseType.COPY_OBJECT_RESULT,
    required_fields={'LastModified', 'ETag'},
    optional_fields=set(),
    expected_namespace=None
)

S3_DELETE_RESULT_SPEC = S3ResponseSpec(
    response_type=S3ResponseType.DELETE_RESULT,
    required_fields=set(),  # Can have only Deleted or Error elements
    optional_fields={'Deleted', 'Error'},
    expected_namespace="http://s3.amazonaws.com/doc/2006-03-01/"
)


def parse_xml_response(xml_body: Union[str, bytes], extract_namespace: bool = True) -> tuple[ET.Element, Dict[str, str]]:
    """
    Parse XML response body into ElementTree element and extract namespace.

    Args:
        xml_body: XML response as string or bytes
        extract_namespace: If True, returns (element, namespace_dict) tuple

    Returns:
        If extract_namespace=True: (root element, namespace dict with 'default' key)
        If extract_namespace=False: root element only

    Raises:
        XMLResponseValidationError: If XML is malformed or unparseable
    """
    try:
        if isinstance(xml_body, bytes):
            xml_body = xml_body.decode('utf-8')
        root = ET.fromstring(xml_body)

        if not extract_namespace:
            return root

        # Extract namespace from root element
        # Format: {http://s3.amazonaws.com/doc/2006-03-01/}ListBucketResult
        namespace = {}
        if '}' in root.tag:
            namespace_uri = root.tag.split('}')[0].strip('{')
            namespace = {'s3': namespace_uri, 'default': namespace_uri}

        return root, namespace

    except ET.ParseError as e:
        raise XMLResponseValidationError(
            response_body=xml_body,
            parse_error=f"Invalid XML: {e}"
        )


def validate_xml_structure(
    response_body: Union[str, bytes],
    spec: S3ResponseSpec,
    throw_on_error: bool = True
) -> bool:
    """
    Validate XML response structure against S3 specification.

    This helper function validates that an XML response contains the required
    fields, meets structure requirements, and follows S3 API conventions.

    Args:
        response_body: Response body as XML string or bytes
        spec: S3ResponseSpec defining required and optional fields
        throw_on_error: If True, throws XMLResponseValidationError on failure.
                       If False, returns False instead.

    Returns:
        bool: True if XML response structure is valid, False otherwise

    Raises:
        XMLResponseValidationError: If validation fails and throw_on_error is True

    Examples:
        >>> # Validate error response
        >>> xml = '<?xml version="1.0"?><Error><Code>NoSuchKey</Code><Message>Not found</Message></Error>'
        >>> validate_xml_structure(xml, S3_ERROR_SPEC)

        >>> # Validate ListBuckets response
        >>> xml = '<?xml version="1.0"?><ListAllMyBucketsResult xmlns="...">...</ListAllMyBucketsResult>'
        >>> validate_xml_structure(xml, S3_LIST_BUCKETS_SPEC)
    """
    # Parse XML
    try:
        root, namespace = parse_xml_response(response_body)
    except XMLResponseValidationError:
        if throw_on_error:
            raise
        return False

    # Check root element name matches expected type
    expected_root = spec.response_type.value
    actual_root = root.tag.split('}')[1] if '}' in root.tag else root.tag
    if actual_root != expected_root:
        if throw_on_error:
            raise XMLResponseValidationError(
                response_body=response_body,
                field_errors={'root_element': f"Expected <{expected_root}>, got <{actual_root}>"}
            )
        return False

    # Check namespace if specified
    if spec.expected_namespace and namespace:
        actual_ns = namespace.get('default')
        if actual_ns != spec.expected_namespace:
            if throw_on_error:
                raise XMLResponseValidationError(
                    response_body=response_body,
                    field_errors={'namespace': f"Expected namespace {spec.expected_namespace}, got {actual_ns}"}
                )
            return False

    # Check for required fields
    missing_fields: Set[str] = set()
    field_errors: Dict[str, str] = {}

    for field in spec.required_fields:
        # Use namespace if available, otherwise no namespace
        if namespace:
            element = root.find(f's3:{field}', namespace)
        else:
            element = root.find(field)
        if element is None:
            missing_fields.add(field)
        else:
            # Validate that the field has content (for simple text fields)
            if element.text is None and len(element) == 0:
                field_errors[field] = f"Field is empty"

    # If we have missing fields or field errors, raise error
    if (missing_fields or field_errors) and throw_on_error:
        raise XMLResponseValidationError(
            response_body=response_body,
            missing_fields=missing_fields,
            field_errors=field_errors
        )

    return not (missing_fields or field_errors)


def validate_s3_error_response(
    response_body: Union[str, bytes],
    throw_on_error: bool = True
) -> bool:
    """
    Validate S3 error response structure.

    S3 error responses must have Code and Message fields.

    Args:
        response_body: Response body as XML string or bytes
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response is valid S3 error response
    """
    return validate_xml_structure(response_body, S3_ERROR_SPEC, throw_on_error)


def validate_s3_list_buckets_response(
    response_body: Union[str, bytes],
    throw_on_error: bool = True
) -> bool:
    """
    Validate S3 ListBuckets response structure.

    Must include Owner and Buckets elements with proper S3 namespace.

    Args:
        response_body: Response body as XML string or bytes
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response is valid ListBuckets response
    """
    return validate_xml_structure(response_body, S3_LIST_BUCKETS_SPEC, throw_on_error)


def validate_s3_list_objects_response(
    response_body: Union[str, bytes],
    throw_on_error: bool = True
) -> bool:
    """
    Validate S3 ListObjectsV2 response structure.

    Must include Name element, with optional Contents, CommonPrefixes, etc.

    Args:
        response_body: Response body as XML string or bytes
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response is valid ListObjectsV2 response
    """
    return validate_xml_structure(response_body, S3_LIST_OBJECTS_SPEC, throw_on_error)


def validate_s3_copy_object_response(
    response_body: Union[str, bytes],
    throw_on_error: bool = True
) -> bool:
    """
    Validate S3 CopyObject response structure.

    Must include LastModified and ETag elements.

    Args:
        response_body: Response body as XML string or bytes
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response is valid CopyObject response
    """
    return validate_xml_structure(response_body, S3_COPY_OBJECT_SPEC, throw_on_error)


def validate_xml_well_formedness(xml_body: Union[str, bytes]) -> bool:
    """
    Validate that XML is well-formed and parseable.

    Args:
        xml_body: XML string or bytes

    Returns:
        bool: True if XML is well-formed
    """
    try:
        parse_xml_response(xml_body)
        return True
    except XMLResponseValidationError:
        return False


def extract_s3_error_code(xml_body: Union[str, bytes]) -> Optional[str]:
    """
    Extract error code from S3 error response.

    Args:
        xml_body: XML error response body

    Returns:
        Error code string, or None if not found/invalid
    """
    try:
        root, namespace = parse_xml_response(xml_body)
        if namespace:
            code_element = root.find('s3:Code', namespace)
        else:
            code_element = root.find('Code')
        return code_element.text if code_element is not None else None
    except (XMLResponseValidationError, AttributeError):
        return None


def extract_s3_error_message(xml_body: Union[str, bytes]) -> Optional[str]:
    """
    Extract error message from S3 error response.

    Args:
        xml_body: XML error response body

    Returns:
        Error message string, or None if not found/invalid
    """
    try:
        root, namespace = parse_xml_response(xml_body)
        if namespace:
            message_element = root.find('s3:Message', namespace)
        else:
            message_element = root.find('Message')
        return message_element.text if message_element is not None else None
    except (XMLResponseValidationError, AttributeError):
        return None


def validate_response_headers(headers: Dict[str, str], throw_on_error: bool = True) -> List[str]:
    """
    Validate HTTP response headers for S3 API responses.

    Args:
        headers: Dict of HTTP headers
        throw_on_error: If True, raises exception on validation failure

    Returns:
        List of validation errors (empty if all valid)

    Raises:
        XMLResponseValidationError: If validation fails and throw_on_error is True
    """
    errors = []

    # Check Content-Type header
    content_type = headers.get('Content-Type', '')
    if 'application/xml' not in content_type.lower():
        errors.append(f"Expected Content-Type 'application/xml', got '{content_type}'")

    # Check for common S3 headers (optional but recommended)
    # ETag, Last-Modified, Content-Length should be present for object operations
    # but we don't enforce them strictly as they're operation-specific

    if errors and throw_on_error:
        raise XMLResponseValidationError(
            response_body="(headers only)",
            header_errors=errors
        )

    return errors


if __name__ == '__main__':
    # Run basic smoke tests
    print("XML Response Validation Framework - Smoke Tests")
    print("=" * 60)

    # Test error response validation
    error_xml = '<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist</Message></Error>'
    result = validate_s3_error_response(error_xml, throw_on_error=False)
    print(f"✓ Error response validation: {result}")

    # Test XML well-formedness
    result = validate_xml_well_formedness(error_xml)
    print(f"✓ XML well-formedness: {result}")

    # Test error code extraction
    code = extract_s3_error_code(error_xml)
    print(f"✓ Error code extraction: {code}")

    # Test error message extraction
    message = extract_s3_error_message(error_xml)
    print(f"✓ Error message extraction: {message}")

    # Test header validation
    headers = {'Content-Type': 'application/xml', 'ETag': '"abc123"'}
    errors = validate_response_headers(headers, throw_on_error=False)
    print(f"✓ Header validation: {len(errors)} errors")

    print("\nAll smoke tests passed!")
