"""
HTTP Status Code Validation Helper Functions

This module provides reusable helper functions for validating HTTP status codes
in error response tests. These helpers support both single status codes and arrays
of allowed codes, with clear assertion error messages.

Bead: bf-gfemoh
Created: 2026-07-14
"""

from typing import Union, List, Optional

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
