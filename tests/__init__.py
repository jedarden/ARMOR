"""
ARMOR Test Suite Package

This package contains test utilities and test cases for the ARMOR project.
"""

from .test_helpers import (
    validate_http_status,
    validate_http_status_codes,
    StatusValidationError,
    validate_content_type,
    validate_json_content_type,
    validate_xml_content_type,
    ContentTypeValidationError,
)

from .test_error_response_validation import (
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
)

__all__ = [
    # HTTP status validation
    'validate_http_status',
    'validate_http_status_codes',
    'StatusValidationError',
    # Content-Type validation
    'validate_content_type',
    'validate_json_content_type',
    'validate_xml_content_type',
    'ContentTypeValidationError',
    # Error response structure validation
    'validate_error_response',
    'validate_error_field_only',
    'validate_standard_error_response',
    'validate_detailed_error_response',
    'validate_http_error_response',
    'validate_error_responses',
    'validate_with_standard_validators',
    'validate_with_detailed_validators',
    'ErrorResponseSpec',
    'ErrorResponseValidationError',
    'STANDARD_ERROR_SPEC',
    'DETAILED_ERROR_SPEC',
    'MINIMAL_ERROR_SPEC',
]
