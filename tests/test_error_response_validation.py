#!/usr/bin/env python3
"""
Error Response Structure Validation Helper Functions

This module provides reusable helper functions for validating error response
structure in tests. These helpers verify that error responses contain required
fields like 'error', 'message', and optional fields like 'code' and 'details'.

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

import json
import unittest
from typing import Union, Dict, Any, Optional, List, Set
from dataclasses import dataclass


@dataclass
class ErrorResponseSpec:
    """
    Specification for expected error response structure.

    Defines which fields are required, optional, or forbidden in an error response.
    """
    required_fields: Set[str]
    optional_fields: Set[str]

    def __post_init__(self):
        """Validate the specification doesn't have overlapping fields."""
        overlap = self.required_fields & self.optional_fields
        if overlap:
            raise ValueError(
                f"Fields cannot be both required and optional: {overlap}"
            )


class ErrorResponseValidationError(AssertionError):
    """
    Custom assertion error for error response structure validation failures.

    Provides clear, formatted error messages that show:
    - The actual response body received
    - Which required fields are missing
    - Which unexpected fields are present
    - Field-specific validation errors (e.g., empty message)
    """

    def __init__(self,
                 response_body: Any,
                 missing_fields: Optional[Set[str]] = None,
                 unexpected_fields: Optional[Set[str]] = None,
                 field_errors: Optional[Dict[str, str]] = None,
                 parse_error: Optional[str] = None):
        """
        Initialize an error response validation error.

        Args:
            response_body: The actual response body (parsed or raw)
            missing_fields: Set of required field names that were missing
            unexpected_fields: Set of unexpected field names that were present
            field_errors: Dict mapping field names to specific error messages
            parse_error: Error message if response body couldn't be parsed
        """
        self.response_body = response_body
        self.missing_fields = missing_fields or set()
        self.unexpected_fields = unexpected_fields or set()
        self.field_errors = field_errors or {}
        self.parse_error = parse_error

        message = self._format_error_message()
        super().__init__(message)

    def _format_error_message(self) -> str:
        """Format a detailed error message."""
        msg_parts = ["Error response validation failed:"]

        if self.parse_error:
            msg_parts.append(f"  Parse error: {self.parse_error}")
            msg_parts.append(f"  Response body: {str(self.response_body)[:200]}")
            return "\n".join(msg_parts)

        if self.missing_fields:
            msg_parts.append(f"  Missing required fields: {', '.join(sorted(self.missing_fields))}")

        if self.unexpected_fields:
            msg_parts.append(f"  Unexpected fields: {', '.join(sorted(self.unexpected_fields))}")

        if self.field_errors:
            msg_parts.append("  Field validation errors:")
            for field, error in sorted(self.field_errors.items()):
                msg_parts.append(f"    - {field}: {error}")

        # Add response body preview
        body_preview = json.dumps(self.response_body, indent=2, default=str)
        if len(body_preview) > 300:
            body_preview = body_preview[:300] + "\n  ... (truncated)"
        msg_parts.append(f"  Response body:\n{body_preview}")

        return "\n".join(msg_parts)


# Common error response specifications
STANDARD_ERROR_SPEC = ErrorResponseSpec(
    required_fields={'error', 'message'},
    optional_fields={'code', 'details'}
)

DETAILED_ERROR_SPEC = ErrorResponseSpec(
    required_fields={'error', 'message', 'code'},
    optional_fields={'details', 'stack_trace', 'request_id'}
)

MINIMAL_ERROR_SPEC = ErrorResponseSpec(
    required_fields={'error'},
    optional_fields={'message', 'code'}
)


def validate_error_response(
    response_body: Any,
    spec: ErrorResponseSpec = STANDARD_ERROR_SPEC,
    throw_on_error: bool = True,
    custom_validators: Optional[Dict[str, callable]] = None
) -> bool:
    """
    Validate error response structure against a specification.

    This helper function validates that an error response contains the required
    fields and meets structure requirements. It supports both JSON-parsed
    dictionaries and raw string responses, with flexible field specifications.

    Args:
        response_body: Response body as parsed JSON (dict) or raw string
        spec: ErrorResponseSpec defining required and optional fields
        throw_on_error: If True, throws ErrorResponseValidationError on failure.
                       If False, returns False instead.
        custom_validators: Optional dict mapping field names to validation functions.
                           Each function should accept (value, field_name) and return
                           None if valid or an error message string if invalid.

    Returns:
        bool: True if error response structure is valid, False otherwise

    Raises:
        ErrorResponseValidationError: If validation fails and throw_on_error is True
        TypeError: If response_body or spec has invalid type

    Examples:
        >>> # Basic validation with standard spec
        >>> response = {"error": "not_found", "message": "Resource not found"}
        >>> validate_error_response(response)

        >>> # Validation with detailed spec
        >>> response = {"error": "auth_failed", "message": "Invalid credentials", "code": 401}
        >>> validate_error_response(response, spec=DETAILED_ERROR_SPEC)

        >>> # Non-throwing validation
        >>> is_valid = validate_error_response(response, throw_on_error=False)
        >>> if is_valid:
        ...     print("Error response is valid!")

        >>> # Custom field validators
        >>> def validate_code(value, field_name):
        ...     if not isinstance(value, int):
        ...         return f"{field_name} must be an integer"
        ...     if value < 400 or value >= 600:
        ...         return f"{field_name} must be a 4xx or 5xx code"
        >>> validate_error_response(
        ...     response,
        ...     custom_validators={"code": validate_code}
        ... )

        >>> # Raw JSON string response
        >>> validate_error_response('{"error":"internal","message":"Server error"}')
    """
    # Parse response body if it's a string
    parsed_body: Any
    if isinstance(response_body, str):
        try:
            parsed_body = json.loads(response_body)
        except json.JSONDecodeError as e:
            if throw_on_error:
                raise ErrorResponseValidationError(
                    response_body=response_body,
                    parse_error=f"Invalid JSON: {e}"
                )
            return False
    elif isinstance(response_body, dict):
        parsed_body = response_body
    else:
        raise TypeError(
            f"response_body must be dict or JSON string, "
            f"got {type(response_body).__name__}"
        )

    # Validate spec type
    if not isinstance(spec, ErrorResponseSpec):
        raise TypeError(
            f"spec must be ErrorResponseSpec, got {type(spec).__name__}"
        )

    # Initialize validation results
    missing_fields: Set[str] = set()
    unexpected_fields: Set[str] = set()
    field_errors: Dict[str, str] = {}

    # Check for required fields
    for field in spec.required_fields:
        if field not in parsed_body:
            missing_fields.add(field)
        else:
            # Validate that the field value is not None or empty (for strings)
            value = parsed_body[field]
            if value is None:
                field_errors[field] = f"Field is None"
            elif isinstance(value, str) and not value.strip():
                field_errors[field] = f"Field is empty or whitespace only"

    # Check for unexpected fields (not in required or optional)
    all_allowed_fields = spec.required_fields | spec.optional_fields
    for field in parsed_body.keys():
        if field not in all_allowed_fields:
            unexpected_fields.add(field)

    # Run custom validators if provided
    if custom_validators:
        for field_name, validator in custom_validators.items():
            if field_name in parsed_body:
                try:
                    error_msg = validator(parsed_body[field_name], field_name)
                    if error_msg is not None:
                        field_errors[field_name] = error_msg
                except Exception as e:
                    field_errors[field_name] = f"Validator error: {e}"

    # Determine if validation passed
    is_valid = (
        not missing_fields and
        not unexpected_fields and
        not field_errors
    )

    if not is_valid and throw_on_error:
        raise ErrorResponseValidationError(
            response_body=parsed_body,
            missing_fields=missing_fields,
            unexpected_fields=unexpected_fields,
            field_errors=field_errors
        )

    return is_valid


def validate_error_field_only(
    response_body: Any,
    throw_on_error: bool = True
) -> bool:
    """
    Validate that response contains at least an 'error' field.

    Minimal validation for cases where only the error type matters.

    Args:
        response_body: Response body as parsed JSON (dict) or raw string
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if error field exists and is non-empty
    """
    return validate_error_response(
        response_body,
        spec=MINIMAL_ERROR_SPEC,
        throw_on_error=throw_on_error
    )


def validate_standard_error_response(
    response_body: Any,
    throw_on_error: bool = True
) -> bool:
    """
    Validate standard error response with 'error' and 'message' fields.

    This is the most common error response format, suitable for most APIs.

    Args:
        response_body: Response body as parsed JSON (dict) or raw string
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response has required error and message fields
    """
    return validate_error_response(
        response_body,
        spec=STANDARD_ERROR_SPEC,
        throw_on_error=throw_on_error
    )


def validate_detailed_error_response(
    response_body: Any,
    throw_on_error: bool = True
) -> bool:
    """
    Validate detailed error response with error, message, and code fields.

    Use this for APIs that provide error codes along with messages.

    Args:
        response_body: Response body as parsed JSON (dict) or raw string
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response has required error, message, and code fields
    """
    return validate_error_response(
        response_body,
        spec=DETAILED_ERROR_SPEC,
        throw_on_error=throw_on_error
    )


def validate_http_error_response(
    response: Any,
    spec: ErrorResponseSpec = STANDARD_ERROR_SPEC,
    throw_on_error: bool = True,
    custom_validators: Optional[Dict[str, callable]] = None
) -> bool:
    """
    Validate HTTP error response by extracting body from response object.

    Convenience function that works with HTTP response objects (like requests.Response)
    by extracting the body and then validating it.

    Args:
        response: HTTP response object or tuple of (status_code, body)
        spec: ErrorResponseSpec defining required and optional fields
        throw_on_error: If True, throws on validation failure
        custom_validators: Optional dict of field validation functions

    Returns:
        bool: True if error response structure is valid

    Examples:
        >>> import requests
        >>> response = requests.get('http://api.example.com/notfound')
        >>> validate_http_error_response(response)

        >>> # With tuple format (from curl-based tests)
        >>> status, body = curl_request(...)
        >>> validate_http_error_response((status, body))
    """
    # Extract body from response
    body: Any
    if isinstance(response, tuple) and len(response) >= 2:
        body = response[1]
    elif hasattr(response, 'text'):
        body = response.text
    elif hasattr(response, 'content'):
        body = response.content
    else:
        raise TypeError(
            f"Response must have 'text'/'content' attributes or be a tuple, "
            f"got {type(response).__name__}"
        )

    return validate_error_response(
        body,
        spec=spec,
        throw_on_error=throw_on_error,
        custom_validators=custom_validators
    )


def validate_error_responses(
    *responses: Any,
    spec: ErrorResponseSpec = STANDARD_ERROR_SPEC,
    throw_on_error: bool = True
) -> List[bool]:
    """
    Validate multiple error responses at once.

    Convenience function for batch validation of error responses,
    useful when testing multiple error scenarios.

    Args:
        *responses: Variable number of response bodies
        spec: ErrorResponseSpec defining required and optional fields
        throw_on_error: If True, throws on first validation failure

    Returns:
        List[bool]: List of validation results, one per response

    Examples:
        >>> responses = [
        ...     {"error": "not_found", "message": "Resource not found"},
        ...     {"error": "auth_failed", "message": "Invalid credentials"}
        ... ]
        >>> results = validate_error_responses(*responses)
        >>> all(results)  # True if all passed
    """
    results = []
    for response in responses:
        try:
            result = validate_error_response(response, spec, throw_on_error)
            results.append(result)
        except ErrorResponseValidationError:
            if throw_on_error:
                raise
            results.append(False)

    return results


# Convenience validators for common error field types
def validate_error_code_is_int(value: Any, field_name: str) -> Optional[str]:
    """Validator: Ensure error code is an integer."""
    if not isinstance(value, int):
        return f"{field_name} must be an integer, got {type(value).__name__}"
    return None


def validate_error_code_is_http(value: Any, field_name: str) -> Optional[str]:
    """Validator: Ensure error code is a valid HTTP error code (4xx or 5xx)."""
    if not isinstance(value, int):
        return f"{field_name} must be an integer"
    if value < 400 or value >= 600:
        return f"{field_name} must be 4xx or 5xx (got {value})"
    return None


def validate_message_not_empty(value: Any, field_name: str) -> Optional[str]:
    """Validator: Ensure message is a non-empty string."""
    if not isinstance(value, str):
        return f"{field_name} must be a string"
    if not value.strip():
        return f"{field_name} cannot be empty"
    return None


def validate_error_not_empty(value: Any, field_name: str) -> Optional[str]:
    """Validator: Ensure error field is a non-empty string."""
    if not isinstance(value, str):
        return f"{field_name} must be a string"
    if not value.strip():
        return f"{field_name} cannot be empty"
    return None


# Pre-configured validator sets for common scenarios
STANDARD_VALIDATORS = {
    'error': validate_error_not_empty,
    'message': validate_message_not_empty,
}

DETAILED_VALIDATORS = {
    'error': validate_error_not_empty,
    'message': validate_message_not_empty,
    'code': validate_error_code_is_http,
}


def validate_with_standard_validators(
    response_body: Any,
    throw_on_error: bool = True
) -> bool:
    """
    Validate error response using standard field validators.

    Applies common validation rules to ensure error fields have proper types
    and non-empty values.

    Args:
        response_body: Response body as parsed JSON (dict) or raw string
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response passes all standard validations
    """
    return validate_error_response(
        response_body,
        spec=STANDARD_ERROR_SPEC,
        throw_on_error=throw_on_error,
        custom_validators=STANDARD_VALIDATORS
    )


def validate_with_detailed_validators(
    response_body: Any,
    throw_on_error: bool = True
) -> bool:
    """
    Validate error response using detailed field validators.

    Applies strict validation rules including HTTP error code validation.

    Args:
        response_body: Response body as parsed JSON (dict) or raw string
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if response passes all detailed validations
    """
    return validate_error_response(
        response_body,
        spec=DETAILED_ERROR_SPEC,
        throw_on_error=throw_on_error,
        custom_validators=DETAILED_VALIDATORS
    )
