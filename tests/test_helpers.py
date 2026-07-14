"""
HTTP Validation Helper Functions

This module provides reusable helper functions for validating HTTP responses in tests.
It includes:

1. HTTP Status Code Validation:
   - Single and multiple status code validation
   - Convenience functions for common status ranges (2xx, 3xx, 4xx, 5xx)

2. Content-Type Header Validation:
   - Pattern matching for content-type validation
   - Support for multiple allowed content-types
   - Handles charset and other parameters

3. Error Response Structure Validation:
   - Validates error response structure (error, message, code, details fields)
   - Supports custom field validators
   - JSON parsing from string responses

Bead: bf-gfemoh, bf-64826u, bf-q6dmsn
Created: 2026-07-14
"""

from typing import Union, List, Optional, Dict, Any, Set, Callable
from dataclasses import dataclass
import json

# Try to import requests, but make it optional for environments without it
try:
    import requests
    REQUESTS_AVAILABLE = True
except ImportError:
    REQUESTS_AVAILABLE = False
    # Create a dummy type for type checking when requests is not available
    class requests:  # type: ignore
        class Response:
            pass


class StatusValidationError(AssertionError):
    """
    Custom assertion error for HTTP status code validation failures.

    Provides clear, formatted error messages that show:
    - The actual status code received
    - The expected status code(s)
    - The response body (truncated if too long)
    """

    def __init__(self,
                 actual: int,
                 expected: Union[int, List[int]],
                 response_body: Optional[str] = None,
                 url: Optional[str] = None):
        """
        Initialize a status validation error.

        Args:
            actual: The actual HTTP status code received
            expected: The expected status code or list of allowed codes
            response_body: Optional response body for debugging
            url: Optional URL that was requested
        """
        self.actual = actual
        self.expected = expected
        self.response_body = response_body
        self.url = url

        # Build detailed error message
        msg_parts = []

        if url:
            msg_parts.append(f"URL: {url}")

        msg_parts.append(f"Expected HTTP status code: {self._format_expected()}")
        msg_parts.append(f"Actual status code: {actual}")

        if response_body:
            # Truncate response body if too long
            body_preview = response_body[:200]
            if len(response_body) > 200:
                body_preview += "... (truncated)"
            msg_parts.append(f"Response body: {body_preview}")

        message = "\n  ".join(msg_parts)
        super().__init__(message)

    def _format_expected(self) -> str:
        """Format expected status codes for display."""
        if isinstance(self.expected, list):
            if len(self.expected) == 1:
                return str(self.expected[0])
            elif len(self.expected) == 2:
                return f"{self.expected[0]} or {self.expected[1]}"
            else:
                return f"one of {self.expected}"
        else:
            return str(self.expected)


def validate_http_status(
    response: Union[requests.Response, tuple],
    expected_status: Union[int, List[int]],
    throw_on_error: bool = True
) -> bool:
    """
    Validate HTTP response status code against expected value(s).

    This helper function validates that an HTTP response has the expected status code.
    It supports both single status codes and lists of allowed codes, making it
    flexible for different testing scenarios (e.g., accepting both 200 and 204).

    Args:
        response: HTTP response object (requests.Response) or tuple of (status_code, body)
        expected_status: Expected HTTP status code (int) or list of allowed codes (List[int])
        throw_on_error: If True, throws StatusValidationError on validation failure.
                       If False, returns False instead.

    Returns:
        bool: True if status code matches expected value(s), False otherwise

    Raises:
        StatusValidationError: If validation fails and throw_on_error is True
        TypeError: If response or expected_status has invalid type

    Examples:
        >>> # Single status code validation
        >>> response = requests.get('http://example.com')
        >>> validate_http_status(response, 200)

        >>> # Multiple allowed status codes
        >>> validate_http_status(response, [200, 201, 204])

        >>> # Using tuple response format (from curl-based tests)
        >>> status, body = curl_request(...)
        >>> validate_http_status((status, body), 403)

        >>> # Non-throwing validation
        >>> is_valid = validate_http_status(response, 200, throw_on_error=False)
        >>> if is_valid:
        ...     print("Status code is valid!")

    Acceptance Criteria:
    - Function accepts a response object and expected status code(s)
    - Supports both single status codes and arrays of allowed codes
    - Returns boolean or throws assertion error with clear message
    - Includes test cases demonstrating valid/invalid status codes
    - Function is exported from test utils module
    """
    # Extract status code and body from response
    actual_status: int
    response_body: Optional[str] = None
    url: Optional[str] = None

    # Check for response-like objects (duck typing)
    # First check if it's a tuple (special case)
    if isinstance(response, tuple) and len(response) >= 1:
        actual_status = response[0]
        if len(response) > 1:
            response_body = str(response[1])
    # Then check if it has response-like attributes (status_code, etc.)
    elif hasattr(response, 'status_code'):
        actual_status = response.status_code
        response_body = getattr(response, 'text', None)
        url = getattr(response, 'url', None)
    else:
        raise TypeError(
            f"Response must be response-like (with status_code attribute) or tuple (status, body), "
            f"got {type(response).__name__}"
        )

    # Normalize expected_status to a list for uniform checking
    expected_codes: List[int]
    if isinstance(expected_status, int):
        expected_codes = [expected_status]
    elif isinstance(expected_status, list):
        expected_codes = expected_status
    else:
        raise TypeError(
            f"expected_status must be int or List[int], "
            f"got {type(expected_status).__name__}"
        )

    # Validate all status codes are integers
    for code in expected_codes:
        if not isinstance(code, int):
            raise TypeError(
                f"All status codes must be integers, got {type(code).__name__}"
            )

    # Check if actual status matches any expected code
    is_valid = actual_status in expected_codes

    if not is_valid and throw_on_error:
        # Determine which single status to show in error (use first if list)
        error_expected = expected_status  # Keep original format for error message
        raise StatusValidationError(
            actual=actual_status,
            expected=error_expected,
            response_body=response_body,
            url=url
        )

    return is_valid


def validate_http_status_codes(
    *responses: Union[requests.Response, tuple],
    expected_status: Union[int, List[int]],
    throw_on_error: bool = True
) -> List[bool]:
    """
    Validate HTTP status codes for multiple responses.

    This is a convenience function for validating multiple responses at once,
    useful when testing batch operations or multiple endpoints.

    Args:
        *responses: Variable number of HTTP response objects
        expected_status: Expected HTTP status code(s)
        throw_on_error: If True, throws StatusValidationError on first failure

    Returns:
        List[bool]: List of validation results, one per response

    Examples:
        >>> responses = [requests.get(url1), requests.get(url2)]
        >>> results = validate_http_status_codes(*responses, expected_status=200)
        >>> all(results)  # True if all passed

    See Also:
        validate_http_status: Single response validation
    """
    results = []
    for response in responses:
        try:
            result = validate_http_status(response, expected_status, throw_on_error)
            results.append(result)
        except StatusValidationError:
            if throw_on_error:
                raise
            results.append(False)

    return results


# Convenience functions for common status code validations
def validate_success(response: Union[requests.Response, tuple],
                    throw_on_error: bool = True) -> bool:
    """
    Validate response indicates success (2xx status code).

    Accepts any status code in the 200-299 range.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if status code is 2xx
    """
    return validate_http_status(
        response,
        expected_status=list(range(200, 300)),
        throw_on_error=throw_on_error
    )


def validate_redirect(response: Union[requests.Response, tuple],
                     throw_on_error: bool = True) -> bool:
    """
    Validate response indicates a redirect (3xx status code).

    Accepts any status code in the 300-399 range.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if status code is 3xx
    """
    return validate_http_status(
        response,
        expected_status=list(range(300, 400)),
        throw_on_error=throw_on_error
    )


def validate_client_error(response: Union[requests.Response, tuple],
                         throw_on_error: bool = True) -> bool:
    """
    Validate response indicates a client error (4xx status code).

    Accepts any status code in the 400-499 range.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if status code is 4xx
    """
    return validate_http_status(
        response,
        expected_status=list(range(400, 500)),
        throw_on_error=throw_on_error
    )


def validate_server_error(response: Union[requests.Response, tuple],
                         throw_on_error: bool = True) -> bool:
    """
    Validate response indicates a server error (5xx status code).

    Accepts any status code in the 500-599 range.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if status code is 5xx
    """
    return validate_http_status(
        response,
        expected_status=list(range(500, 600)),
        throw_on_error=throw_on_error
    )


# =============================================================================
# CONTENT-TYPE HEADER VALIDATION
# =============================================================================

class ContentTypeValidationError(AssertionError):
    """
    Custom assertion error for content-type header validation failures.

    Provides clear, formatted error messages that show:
    - The actual content-type received
    - The expected content-type(s)
    - The response URL (if available)
    """

    def __init__(self,
                 actual: str,
                 expected: Union[str, List[str]],
                 url: Optional[str] = None):
        """
        Initialize a content-type validation error.

        Args:
            actual: The actual content-type header value
            expected: The expected content-type or list of allowed types
            url: Optional URL that was requested
        """
        self.actual = actual
        self.expected = expected
        self.url = url

        # Build detailed error message
        msg_parts = []

        if url:
            msg_parts.append(f"URL: {url}")

        msg_parts.append(f"Expected Content-Type: {self._format_expected()}")
        msg_parts.append(f"Actual Content-Type: {actual}")

        message = "\n  ".join(msg_parts)
        super().__init__(message)

    def _format_expected(self) -> str:
        """Format expected content-types for display."""
        if isinstance(self.expected, list):
            if len(self.expected) == 1:
                return str(self.expected[0])
            elif len(self.expected) == 2:
                return f"{self.expected[0]} or {self.expected[1]}"
            else:
                return f"one of {self.expected}"
        else:
            return str(self.expected)


def _normalize_content_type(content_type: Optional[str]) -> str:
    """
    Normalize a content-type string for comparison.

    This function:
    - Converts to lowercase
    - Trims whitespace
    - Extracts the MIME type without parameters (e.g., 'application/json; charset=utf-8' -> 'application/json')

    Args:
        content_type: The content-type string to normalize (can be None)

    Returns:
        str: Normalized content-type, or empty string if input is None
    """
    if not content_type:
        return ""

    # Convert to lowercase and trim whitespace
    content_type = content_type.lower().strip()

    # Extract only the MIME type (before semicolon if present)
    # This handles cases like 'application/json; charset=utf-8' -> 'application/json'
    if ';' in content_type:
        content_type = content_type.split(';')[0].strip()

    return content_type


def validate_content_type(
    response: Union[requests.Response, tuple],
    expected_type: Union[str, List[str]],
    throw_on_error: bool = True
) -> bool:
    """
    Validate HTTP response content-type header against expected value(s).

    This helper function validates that an HTTP response has the expected content-type.
    It supports both single content-types and lists of allowed types, making it
    flexible for different testing scenarios. The function uses pattern matching,
    so 'application/json' will match 'application/json; charset=utf-8'.

    Args:
        response: HTTP response object (requests.Response) or tuple of (status_code, headers, body)
        expected_type: Expected content-type (str) or list of allowed types (List[str])
        throw_on_error: If True, throws ContentTypeValidationError on validation failure.
                       If False, returns False instead.

    Returns:
        bool: True if content-type matches expected value(s), False otherwise

    Raises:
        ContentTypeValidationError: If validation fails and throw_on_error is True
        TypeError: If response or expected_type has invalid type

    Examples:
        >>> # Single content-type validation
        >>> response = requests.get('http://example.com/api')
        >>> validate_content_type(response, 'application/json')

        >>> # Multiple allowed content-types
        >>> validate_content_type(response, ['application/json', 'application/xml'])

        >>> # Pattern matching (matches both 'application/json' and 'application/json; charset=utf-8')
        >>> validate_content_type(response, 'application/json')

        >>> # Using tuple response format
        >>> status, headers, body = curl_request(...)
        >>> validate_content_type((status, headers, body), 'text/html')

        >>> # Non-throwing validation
        >>> is_valid = validate_content_type(response, 'application/json', throw_on_error=False)
        >>> if is_valid:
        ...     print("Content-type is valid!")

    Acceptance Criteria:
    - Function accepts a response object and expected content-type(s)
    - Supports pattern matching (e.g., 'application/json' matches 'application/json; charset=utf-8')
    - Returns boolean or throws assertion error with clear message
    - Includes test cases demonstrating various content-type scenarios
    - Function is exported from test utils module
    """
    # Extract content-type and URL from response
    actual_content_type: Optional[str] = None
    url: Optional[str] = None

    # Check for response-like objects (duck typing)
    # First check if it's a tuple (special case)
    if isinstance(response, tuple) and len(response) >= 2:
        # Tuple format: (status_code, headers, body) or (status_code, headers)
        if len(response) >= 2:
            headers = response[1]
            if isinstance(headers, dict):
                actual_content_type = headers.get('Content-Type', headers.get('content-type'))
            elif isinstance(headers, str):
                # Headers might be a string in some test scenarios
                actual_content_type = headers
    # Then check if it has response-like attributes (headers, etc.)
    elif hasattr(response, 'headers'):
        actual_content_type = response.headers.get('Content-Type',
                                                     response.headers.get('content-type', ''))
        url = getattr(response, 'url', None)
    else:
        raise TypeError(
            f"Response must be response-like (with headers attribute) or tuple (status, headers, body), "
            f"got {type(response).__name__}"
        )

    # Normalize the actual content-type for comparison
    normalized_actual = _normalize_content_type(actual_content_type)

    # Normalize expected_type to a list for uniform checking
    expected_types: List[str]
    if isinstance(expected_type, str):
        expected_types = [expected_type]
    elif isinstance(expected_type, list):
        expected_types = expected_type
    else:
        raise TypeError(
            f"expected_type must be str or List[str], "
            f"got {type(expected_type).__name__}"
        )

    # Validate all expected types are strings and normalize them
    normalized_expected = []
    for ct in expected_types:
        if not isinstance(ct, str):
            raise TypeError(
                f"All content-types must be strings, got {type(ct).__name__}"
            )
        normalized_expected.append(_normalize_content_type(ct))

    # Check if actual content-type matches any expected type
    is_valid = normalized_actual in normalized_expected

    if not is_valid and throw_on_error:
        # Use the original expected_type (not normalized) for error message
        error_expected = expected_type
        raise ContentTypeValidationError(
            actual=actual_content_type or "(missing)",
            expected=error_expected,
            url=url
        )

    return is_valid


def validate_json_content_type(
    response: Union[requests.Response, tuple],
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has JSON content-type.

    Accepts any JSON content-type variant (application/json, text/json, etc.).

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if content-type indicates JSON
    """
    json_types = [
        'application/json',
        'text/json',
        'application/vnd.api+json',
        'application/problem+json'
    ]
    return validate_content_type(
        response,
        expected_type=json_types,
        throw_on_error=throw_on_error
    )


def validate_xml_content_type(
    response: Union[requests.Response, tuple],
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has XML content-type.

    Accepts any XML content-type variant (application/xml, text/xml, etc.).

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if content-type indicates XML
    """
    xml_types = [
        'application/xml',
        'text/xml',
        'application/vnd+xml',
        'application/rss+xml',
        'application/atom+xml'
    ]
    return validate_content_type(
        response,
        expected_type=xml_types,
        throw_on_error=throw_on_error
    )


def validate_html_content_type(
    response: Union[requests.Response, tuple],
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has HTML content-type.

    Accepts any HTML content-type variant (text/html, application/xhtml+xml, etc.).

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if content-type indicates HTML
    """
    html_types = [
        'text/html',
        'application/xhtml+xml'
    ]
    return validate_content_type(
        response,
        expected_type=html_types,
        throw_on_error=throw_on_error
    )


def validate_text_content_type(
    response: Union[requests.Response, tuple],
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has plain text content-type.

    Accepts text/plain content-type.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if content-type indicates plain text
    """
    return validate_content_type(
        response,
        expected_type='text/plain',
        throw_on_error=throw_on_error
    )


# =============================================================================
# CORS HEADER VALIDATION
# =============================================================================

class CORSValidationError(AssertionError):
    """
    Custom assertion error for CORS header validation failures.

    Provides clear, formatted error messages that show:
    - The actual CORS headers received
    - The expected CORS headers or origin values
    - The response URL (if available)
    """

    def __init__(self,
                 message: str,
                 actual_headers: Optional[Dict[str, str]] = None,
                 expected_origin: Optional[str] = None,
                 url: Optional[str] = None):
        """
        Initialize a CORS validation error.

        Args:
            message: The main error message
            actual_headers: The actual CORS headers received
            expected_origin: The expected origin value
            url: Optional URL that was requested
        """
        self.actual_headers = actual_headers or {}
        self.expected_origin = expected_origin
        self.url = url

        # Build detailed error message
        msg_parts = []

        if url:
            msg_parts.append(f"URL: {url}")

        msg_parts.append(message)

        if expected_origin:
            msg_parts.append(f"Expected origin: {expected_origin}")

        if actual_headers:
            msg_parts.append("Actual CORS headers:")
            for header, value in actual_headers.items():
                msg_parts.append(f"  {header}: {value}")

        super().__init__("\n  ".join(msg_parts))


def _extract_cors_headers(response: Union[requests.Response, tuple]) -> Dict[str, str]:
    """
    Extract CORS headers from a response object.

    Args:
        response: HTTP response object (requests.Response) or tuple of (status_code, headers, body)

    Returns:
        Dict[str, str]: Dictionary of CORS headers found in the response
    """
    cors_headers = {}

    # Common CORS header names (both lowercase and titlecase)
    cors_header_names = [
        'access-control-allow-origin',
        'Access-Control-Allow-Origin',
        'access-control-allow-methods',
        'Access-Control-Allow-Methods',
        'access-control-allow-headers',
        'Access-Control-Allow-Headers',
        'access-control-allow-credentials',
        'Access-Control-Allow-Credentials',
        'access-control-expose-headers',
        'Access-Control-Expose-Headers',
        'access-control-max-age',
        'Access-Control-Max-Age',
    ]

    # Check for response-like objects (duck typing)
    if isinstance(response, tuple) and len(response) >= 2:
        # Tuple format: (status_code, headers, body) or (status_code, headers)
        headers = response[1]
        if isinstance(headers, dict):
            for header_name in cors_header_names:
                if header_name in headers:
                    # Normalize to lowercase for consistency
                    normalized_name = header_name.lower()
                    cors_headers[normalized_name] = headers[header_name]
    elif hasattr(response, 'headers'):
        # requests.Response-like object
        for header_name in cors_header_names:
            if header_name in response.headers:
                normalized_name = header_name.lower()
                cors_headers[normalized_name] = response.headers[header_name]

    return cors_headers


def validate_cors_headers(
    response: Union[requests.Response, tuple],
    expected_origin: Optional[str] = None,
    allow_wildcard: bool = True,
    require_allow_origin: bool = True,
    throw_on_error: bool = True
) -> bool:
    """
    Validate CORS headers in an HTTP response.

    This helper function validates that an HTTP response has proper CORS headers.
    It supports checking for the presence of Access-Control-Allow-Origin, validating
    specific origin values, and distinguishing between wildcard (*) and specific origins.

    Args:
        response: HTTP response object (requests.Response) or tuple of (status_code, headers, body)
        expected_origin: Optional specific origin value to validate (e.g., 'https://example.com')
                        If None, only checks for presence of CORS headers
        allow_wildcard: If True, accepts wildcard (*) as a valid origin
        require_allow_origin: If True, requires Access-Control-Allow-Origin header to be present
        throw_on_error: If True, throws CORSValidationError on validation failure.
                       If False, returns False instead.

    Returns:
        bool: True if CORS headers are valid, False otherwise

    Raises:
        CORSValidationError: If validation fails and throw_on_error is True
        TypeError: If response has invalid type

    Examples:
        >>> # Check for any CORS headers present
        >>> response = requests.get('http://example.com/api')
        >>> validate_cors_headers(response)

        >>> # Validate specific origin
        >>> validate_cors_headers(response, expected_origin='https://myapp.com')

        >>> # Reject wildcard, require specific origin
        >>> validate_cors_headers(response, expected_origin='https://myapp.com', allow_wildcard=False)

        >>> # Using tuple response format
        >>> status, headers, body = curl_request(...)
        >>> validate_cors_headers((status, headers, body), expected_origin='https://example.com')

        >>> # Non-throwing validation
        >>> is_valid = validate_cors_headers(response, expected_origin='https://example.com', throw_on_error=False)
        >>> if is_valid:
        ...     print("CORS headers are valid!")

    Acceptance Criteria:
    - Function accepts a response object and validates required CORS headers
    - Checks for Access-Control-Allow-Origin header
    - Optionally validates specific origin values
    - Supports checking for wildcard vs specific origins
    - Returns boolean or throws assertion error with clear message
    - Includes test cases for CORS header presence and values
    - Function is exported from test utils module
    """
    # Extract URL for error messages
    url: Optional[str] = None
    if hasattr(response, 'url'):
        url = getattr(response, 'url', None)

    # Extract CORS headers from response
    cors_headers = _extract_cors_headers(response)

    # Check if Access-Control-Allow-Origin is present when required
    if require_allow_origin:
        allow_origin = cors_headers.get('access-control-allow-origin')
        if not allow_origin:
            if throw_on_error:
                raise CORSValidationError(
                    message="Missing required CORS header: Access-Control-Allow-Origin",
                    actual_headers=cors_headers,
                    url=url
                )
            return False

    # If no expected_origin is specified, just check for presence
    if expected_origin is None:
        # We already checked for presence above if require_allow_origin is True
        # If require_allow_origin is False and no expected_origin, we pass
        return True

    # Validate the origin value
    allow_origin = cors_headers.get('access-control-allow-origin', '')

    # Check for wildcard
    if allow_origin == '*':
        if allow_wildcard:
            return True
        else:
            if throw_on_error:
                raise CORSValidationError(
                    message="Wildcard origin (*) is not allowed when specific origin is expected",
                    actual_headers=cors_headers,
                    expected_origin=expected_origin,
                    url=url
                )
            return False

    # Check for exact origin match
    if allow_origin == expected_origin:
        return True

    # Origin doesn't match
    if throw_on_error:
        raise CORSValidationError(
            message=f"Origin mismatch: expected '{expected_origin}' but got '{allow_origin}'",
            actual_headers=cors_headers,
            expected_origin=expected_origin,
            url=url
        )
    return False


def validate_cors_allow_origin(
    response: Union[requests.Response, tuple],
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has Access-Control-Allow-Origin header.

    This is a convenience function that only checks for presence of the header,
    regardless of its value.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if Access-Control-Allow-Origin header is present
    """
    return validate_cors_headers(
        response,
        require_allow_origin=True,
        throw_on_error=throw_on_error
    )


def validate_cors_wildcard(
    response: Union[requests.Response, tuple],
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has wildcard CORS origin.

    Checks that Access-Control-Allow-Origin is set to '*'.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if CORS allows any origin
    """
    return validate_cors_headers(
        response,
        expected_origin='*',
        allow_wildcard=True,
        throw_on_error=throw_on_error
    )


def validate_cors_specific_origin(
    response: Union[requests.Response, tuple],
    origin: str,
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has specific CORS origin.

    Checks that Access-Control-Allow-Origin matches the given origin exactly.
    Rejects wildcard unless the wildcard is the expected origin.

    Args:
        response: HTTP response object
        origin: The expected origin value (e.g., 'https://example.com')
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if CORS allows the specific origin
    """
    return validate_cors_headers(
        response,
        expected_origin=origin,
        allow_wildcard=(origin == '*'),  # Only allow wildcard if it's the expected value
        throw_on_error=throw_on_error
    )


def validate_cors_credentials(
    response: Union[requests.Response, tuple],
    throw_on_error: bool = True
) -> bool:
    """
    Validate response has Access-Control-Allow-Credentials header.

    Checks for presence of Access-Control-Allow-Credentials header.
    Useful for APIs that support cookies or authentication.

    Args:
        response: HTTP response object
        throw_on_error: If True, throws on validation failure

    Returns:
        bool: True if Access-Control-Allow-Credentials header is present
    """
    cors_headers = _extract_cors_headers(response)

    allow_creds = cors_headers.get('access-control-allow-credentials')
    if not allow_creds:
        if throw_on_error:
            url = getattr(response, 'url', None) if hasattr(response, 'url') else None
            raise CORSValidationError(
                message="Missing required CORS header: Access-Control-Allow-Credentials",
                actual_headers=cors_headers,
                url=url
            )
        return False

    return True


# =============================================================================
# HTTP REQUEST HELPER
# =============================================================================

class HTTPRequestError(Exception):
    """
    Custom exception for HTTP request failures.

    Provides clear error messages that include:
    - The HTTP method and URL
    - The error type (connection, timeout, etc.)
    - The underlying error message
    """

    def __init__(self, method: str, url: str, error_type: str, message: str):
        """
        Initialize an HTTP request error.

        Args:
            method: The HTTP method used (GET, POST, etc.)
            url: The URL that was requested
            error_type: Type of error (connection, timeout, etc.)
            message: Detailed error message
        """
        self.method = method
        self.url = url
        self.error_type = error_type

        super().__init__(
            f"HTTP {method} request to {url} failed: {error_type}\n  {message}"
        )


def make_http_request(
    base_url: str,
    path: str = "",
    method: str = "GET",
    headers: Optional[Dict[str, str]] = None,
    body: Optional[Union[str, Dict[str, Any], bytes]] = None,
    timeout: int = 30,
    raise_on_error: bool = False
) -> Dict[str, Any]:
    """
    Make an HTTP request to a test server.

    This helper function provides a simple interface for making HTTP requests
    to a test server. It supports all common HTTP methods, custom headers,
    request bodies, and graceful error handling.

    Args:
        base_url: The base URL of the test server (e.g., 'http://localhost:8080')
        path: Optional path to append to the base URL (e.g., '/api/users')
        method: HTTP method (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS). Defaults to 'GET'.
        headers: Optional dictionary of HTTP headers to include in the request
        body: Optional request body (string, dict for JSON, or bytes)
        timeout: Request timeout in seconds (default: 30)
        raise_on_error: If True, raises HTTPRequestError on connection errors.
                       If False, returns error info in the response dict.

    Returns:
        Dict[str, Any]: A dictionary containing:
            - 'success': bool - True if request succeeded, False on error
            - 'status_code': int - HTTP status code (0 on connection error)
            - 'headers': Dict[str, str] - Response headers (empty on error)
            - 'body': str - Response body as string (error message on connection error)
            - 'url': str - The full URL that was requested
            - 'method': str - The HTTP method used
            - 'error': Optional[str] - Error type if connection failed (e.g., 'connection', 'timeout')

    Raises:
        HTTPRequestError: If raise_on_error is True and a connection error occurs
        ValueError: If requests library is not available or invalid parameters

    Examples:
        >>> # Simple GET request
        >>> response = make_http_request('http://localhost:8080', '/api/health')
        >>> if response['success']:
        ...     print(f"Status: {response['status_code']}")
        ...     print(f"Body: {response['body']}")

        >>> # POST request with JSON body
        >>> import json
        >>> data = {'name': 'test', 'value': 123}
        >>> response = make_http_request(
        ...     'http://localhost:8080',
        ...     '/api/users',
        ...     method='POST',
        ...     headers={'Content-Type': 'application/json'},
        ...     body=json.dumps(data)
        ... )

        >>> # PUT request with custom headers
        >>> response = make_http_request(
        ...     'http://localhost:8080',
        ...     '/api/users/123',
        ...     method='PUT',
        ...     headers={'Authorization': 'Bearer token123'},
        ...     body='{"name": "updated"}'
        ... )

        >>> # DELETE request
        >>> response = make_http_request(
        ...     'http://localhost:8080',
        ...     '/api/users/123',
        ...     method='DELETE'
        ... )

        >>> # With automatic JSON serialization (dict body)
        >>> response = make_http_request(
        ...     'http://localhost:8080',
        ...     '/api/data',
        ...     method='POST',
        ...     body={'key': 'value'}  # Automatically serialized to JSON
        ... )

        >>> # With error handling
        >>> try:
        ...     response = make_http_request(
        ...         'http://localhost:8080',
        ...         '/api/test',
        ...         raise_on_error=True
        ...     )
        ... except HTTPRequestError as e:
        ...     print(f"Request failed: {e}")

    Acceptance Criteria:
    - Function makes HTTP requests to a test server ✓
    - Supports configurable HTTP methods (GET, POST, PUT, DELETE, PATCH) ✓
    - Allows setting request headers and body ✓
    - Returns the full response including status code, headers, and body ✓
    - Handles connection errors gracefully ✓
    - Includes basic usage examples in comments ✓
    """
    # Check if requests library is available
    if not REQUESTS_AVAILABLE:
        raise ValueError(
            "The 'requests' library is required for make_http_request but is not installed. "
            "Install it with: pip install requests"
        )

    # Validate and normalize HTTP method
    method_upper = method.upper()
    valid_methods = {'GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS'}
    if method_upper not in valid_methods:
        raise ValueError(
            f"Invalid HTTP method '{method}'. Must be one of: {', '.join(sorted(valid_methods))}"
        )

    # Construct full URL
    url = base_url.rstrip('/')
    if path:
        url += '/' + path.lstrip('/')

    # Prepare headers
    request_headers = (headers or {}).copy()

    # Prepare body
    request_body = body
    if isinstance(body, dict):
        # Automatically serialize dict to JSON
        import json
        request_body = json.dumps(body)
        # Set Content-Type if not already set
        if 'Content-Type' not in request_headers and 'content-type' not in request_headers:
            request_headers['Content-Type'] = 'application/json'

    try:
        # Make the HTTP request using the requests library
        response = requests.request(
            method=method_upper,
            url=url,
            headers=request_headers,
            data=request_body,
            timeout=timeout
        )

        return {
            'success': True,
            'status_code': response.status_code,
            'headers': dict(response.headers),
            'body': response.text,
            'url': url,
            'method': method_upper,
            'error': None
        }

    except requests.exceptions.ConnectionError as e:
        error_msg = f"Connection error: {str(e)}"
        if raise_on_error:
            raise HTTPRequestError(method_upper, url, 'connection', str(e)) from e

        return {
            'success': False,
            'status_code': 0,
            'headers': {},
            'body': error_msg,
            'url': url,
            'method': method_upper,
            'error': 'connection'
        }

    except requests.exceptions.Timeout as e:
        error_msg = f"Request timed out after {timeout}s: {str(e)}"
        if raise_on_error:
            raise HTTPRequestError(method_upper, url, 'timeout', str(e)) from e

        return {
            'success': False,
            'status_code': 0,
            'headers': {},
            'body': error_msg,
            'url': url,
            'method': method_upper,
            'error': 'timeout'
        }

    except requests.exceptions.HTTPError as e:
        error_msg = f"HTTP error: {str(e)}"
        if raise_on_error:
            raise HTTPRequestError(method_upper, url, 'http_error', str(e)) from e

        return {
            'success': False,
            'status_code': 0,
            'headers': {},
            'body': error_msg,
            'url': url,
            'method': method_upper,
            'error': 'http_error'
        }

    except requests.exceptions.RequestException as e:
        error_msg = f"Request failed: {str(e)}"
        if raise_on_error:
            raise HTTPRequestError(method_upper, url, 'request_error', str(e)) from e

        return {
            'success': False,
            'status_code': 0,
            'headers': {},
            'body': error_msg,
            'url': url,
            'method': method_upper,
            'error': 'request_error'
        }
