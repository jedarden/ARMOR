#!/usr/bin/env python3
"""
HTTP Error Scenario Test Fixtures

This module provides reusable test fixtures for common HTTP error scenarios
in the ARMOR test suite. Each fixture includes status code, headers, and body
data that can be easily loaded and customized for different endpoints.

Fixtures include:
- 404 Not Found errors with configurable paths/messages
- 405 Method Not Allowed errors with allowed methods
- 415 Unsupported Media Type errors
- 500 Internal Server Error errors
- Generic error builders for custom scenarios

Usage Examples:
    >>> from tests.fixtures.error_scenarios import (
    ...     not_found_fixture,
    ...     method_not_allowed_fixture,
    ...     unsupported_media_type_fixture,
    ...     internal_server_error_fixture
    ... )
    >>>
    >>> # Use predefined fixture
    >>> response = not_found_fixture(path="/api/users/123")
    >>> assert response['status'] == 404
    >>>
    >>> # Create custom error response
    >>> from tests.fixtures.error_scenarios import create_error_response
    >>> custom_error = create_error_response(
    ...     status=418,
    ...     error_type="im_a_teapot",
    ...     message="I'm a teapot"
    ... )

Bead: bf-7d2vgf
Created: 2026-07-14
"""

from typing import Dict, Any, Optional, List, Union
from dataclasses import dataclass, field


@dataclass
class ErrorResponseFixture:
    """
    A complete HTTP error response fixture.

    Contains all components needed to simulate an HTTP error response:
    - status_code: HTTP status code
    - headers: Dictionary of HTTP headers
    - body: Response body as dict (can be serialized to JSON)
    """
    status_code: int
    headers: Dict[str, str] = field(default_factory=dict)
    body: Dict[str, Any] = field(default_factory=dict)

    def to_tuple(self) -> tuple:
        """
        Convert to tuple format (status_code, headers, body).

        Returns:
            tuple: (status_code, headers, body_dict)
        """
        return (self.status_code, self.headers, self.body)

    def to_response_dict(self) -> Dict[str, Any]:
        """
        Convert to dictionary format for response-like objects.

        Returns:
            dict: Dictionary with status_code, headers, and body keys
        """
        return {
            'status_code': self.status_code,
            'headers': self.headers,
            'body': self.body
        }

    def to_json_body(self) -> str:
        """
        Convert body to JSON string.

        Returns:
            str: JSON string representation of the body
        """
        import json
        return json.dumps(self.body)

    def with_path(self, path: str) -> 'ErrorResponseFixture':
        """
        Add or update path information in the error response.

        Args:
            path: The path that caused the error

        Returns:
            ErrorResponseFixture: New fixture with updated path
        """
        new_body = self.body.copy()
        if 'details' not in new_body:
            new_body['details'] = {}
        new_body['details']['path'] = path
        return ErrorResponseFixture(
            status_code=self.status_code,
            headers=self.headers.copy(),
            body=new_body
        )

    def with_request_id(self, request_id: str) -> 'ErrorResponseFixture':
        """
        Add or update request ID in the error response.

        Args:
            request_id: Unique request identifier

        Returns:
            ErrorResponseFixture: New fixture with updated request_id
        """
        new_body = self.body.copy()
        new_body['request_id'] = request_id
        return ErrorResponseFixture(
            status_code=self.status_code,
            headers=self.headers.copy(),
            body=new_body
        )


# =============================================================================
# 404 NOT FOUND ERROR FIXTURES
# =============================================================================

def not_found_fixture(
    path: str = "/api/resource/123",
    message: Optional[str] = None,
    error_type: str = "not_found"
) -> ErrorResponseFixture:
    """
    Create a 404 Not Found error response fixture.

    Args:
        path: The resource path that was not found
        message: Optional custom error message (defaults to standard message)
        error_type: The error type identifier (default: "not_found")

    Returns:
        ErrorResponseFixture: Configured 404 error response

    Examples:
        >>> fixture = not_found_fixture(path="/api/users/123")
        >>> assert fixture.status_code == 404
        >>> assert fixture.body['error'] == 'not_found'
        >>> assert 'path' in fixture.body['details']

        >>> # Custom message
        >>> fixture = not_found_fixture(
        ...     path="/api/products/456",
        ...     message="Product does not exist"
        ... )
    """
    if message is None:
        message = f"The requested resource '{path}' was not found"

    return ErrorResponseFixture(
        status_code=404,
        headers={
            'Content-Type': 'application/json',
            'X-Error-Type': 'not_found'
        },
        body={
            'error': error_type,
            'message': message,
            'code': 404,
            'details': {
                'path': path,
                'timestamp': '2026-07-14T00:00:00Z'
            }
        }
    )


def not_found_simple(path: str = "/api/resource") -> ErrorResponseFixture:
    """
    Minimal 404 error with only required fields.

    Args:
        path: The resource path that was not found

    Returns:
        ErrorResponseFixture: Minimal 404 error response
    """
    return ErrorResponseFixture(
        status_code=404,
        headers={'Content-Type': 'application/json'},
        body={
            'error': 'not_found',
            'message': f'Resource not found: {path}'
        }
    )


def not_found_with_suggestions(
    path: str = "/api/users/123",
    suggestions: Optional[List[str]] = None
) -> ErrorResponseFixture:
    """
    404 error with suggested alternative paths.

    Args:
        path: The resource path that was not found
        suggestions: List of suggested alternative paths

    Returns:
        ErrorResponseFixture: 404 error with suggestions
    """
    if suggestions is None:
        suggestions = ["/api/users", "/api/users/me", "/api/users/active"]

    return ErrorResponseFixture(
        status_code=404,
        headers={
            'Content-Type': 'application/json',
            'X-Error-Type': 'not_found'
        },
        body={
            'error': 'not_found',
            'message': f"Resource not found: {path}",
            'code': 404,
            'details': {
                'path': path,
                'suggested_paths': suggestions,
                'timestamp': '2026-07-14T00:00:00Z'
            }
        }
    )


# =============================================================================
# 405 METHOD NOT ALLOWED ERROR FIXTURES
# =============================================================================

def method_not_allowed_fixture(
    path: str = "/api/users",
    method: str = "DELETE",
    allowed_methods: Optional[List[str]] = None,
    message: Optional[str] = None
) -> ErrorResponseFixture:
    """
    Create a 405 Method Not Allowed error response fixture.

    Args:
        path: The resource path
        method: The HTTP method that was not allowed
        allowed_methods: List of allowed HTTP methods for this path
        message: Optional custom error message

    Returns:
        ErrorResponseFixture: Configured 405 error response

    Examples:
        >>> fixture = method_not_allowed_fixture(
        ...     path="/api/users",
        ...     method="DELETE",
        ...     allowed_methods=["GET", "POST", "PUT"]
        ... )
        >>> assert fixture.status_code == 405
        >>> assert "Allow" in fixture.headers
        >>> assert "DELETE" not in fixture.headers["Allow"]
    """
    if allowed_methods is None:
        allowed_methods = ["GET", "POST", "PUT", "PATCH"]

    if message is None:
        message = f"Method '{method}' not allowed for path '{path}'"

    return ErrorResponseFixture(
        status_code=405,
        headers={
            'Content-Type': 'application/json',
            'Allow': ', '.join(allowed_methods),
            'X-Error-Type': 'method_not_allowed'
        },
        body={
            'error': 'method_not_allowed',
            'message': message,
            'code': 405,
            'details': {
                'path': path,
                'method': method,
                'allowed_methods': allowed_methods
            }
        }
    )


def method_not_allowed_read_only(
    path: str = "/api/users",
    method: str = "POST"
) -> ErrorResponseFixture:
    """
    405 error for read-only endpoints (GET, HEAD, OPTIONS only).

    Args:
        path: The resource path
        method: The method that was attempted (e.g., POST, PUT, DELETE)

    Returns:
        ErrorResponseFixture: 405 error for read-only endpoint
    """
    return method_not_allowed_fixture(
        path=path,
        method=method,
        allowed_methods=["GET", "HEAD", "OPTIONS"],
        message=f"Path '{path}' is read-only. Method '{method}' is not allowed."
    )


def method_not_allowed_write_only(
    path: str = "/api/deleted-items",
    method: str = "GET"
) -> ErrorResponseFixture:
    """
    405 error for write-only endpoints (POST, PUT, DELETE only).

    Args:
        path: The resource path
        method: The method that was attempted (e.g., GET)

    Returns:
        ErrorResponseFixture: 405 error for write-only endpoint
    """
    return method_not_allowed_fixture(
        path=path,
        method=method,
        allowed_methods=["POST", "PUT", "DELETE"],
        message=f"Path '{path}' is write-only. Method '{method}' is not allowed."
    )


# =============================================================================
# 415 UNSUPPORTED MEDIA TYPE ERROR FIXTURES
# =============================================================================

def unsupported_media_type_fixture(
    path: str = "/api/users",
    content_type: str = "application/xml",
    supported_types: Optional[List[str]] = None,
    message: Optional[str] = None
) -> ErrorResponseFixture:
    """
    Create a 415 Unsupported Media Type error response fixture.

    Args:
        path: The resource path
        content_type: The unsupported content type that was sent
        supported_types: List of supported content types
        message: Optional custom error message

    Returns:
        ErrorResponseFixture: Configured 415 error response

    Examples:
        >>> fixture = unsupported_media_type_fixture(
        ...     path="/api/users",
        ...     content_type="application/xml",
        ...     supported_types=["application/json"]
        ... )
        >>> assert fixture.status_code == 415
        >>> assert fixture.body['error'] == 'unsupported_media_type'
    """
    if supported_types is None:
        supported_types = ["application/json", "application/vnd.api+json"]

    if message is None:
        message = f"Unsupported media type '{content_type}'. Supported types: {', '.join(supported_types)}"

    return ErrorResponseFixture(
        status_code=415,
        headers={
            'Content-Type': 'application/json',
            'Accept-Content': ', '.join(supported_types),
            'X-Error-Type': 'unsupported_media_type'
        },
        body={
            'error': 'unsupported_media_type',
            'message': message,
            'code': 415,
            'details': {
                'path': path,
                'provided_type': content_type,
                'supported_types': supported_types
            }
        }
    )


def unsupported_media_type_json_only(
    path: str = "/api/users",
    provided_type: str = "text/plain"
) -> ErrorResponseFixture:
    """
    415 error for JSON-only endpoints.

    Args:
        path: The resource path
        provided_type: The content type that was provided

    Returns:
        ErrorResponseFixture: 415 error for JSON-only endpoint
    """
    return unsupported_media_type_fixture(
        path=path,
        content_type=provided_type,
        supported_types=["application/json"],
        message=f"Endpoint '{path}' only accepts JSON content"
    )


def unsupported_media_type_missing(
    path: str = "/api/users"
) -> ErrorResponseFixture:
    """
    415 error when Content-Type header is missing.

    Args:
        path: The resource path

    Returns:
        ErrorResponseFixture: 415 error for missing Content-Type
    """
    return ErrorResponseFixture(
        status_code=415,
        headers={
            'Content-Type': 'application/json',
            'X-Error-Type': 'unsupported_media_type'
        },
        body={
            'error': 'unsupported_media_type',
            'message': f"Content-Type header is required for '{path}'",
            'code': 415,
            'details': {
                'path': path,
                'provided_type': None,
                'supported_types': ['application/json']
            }
        }
    )


# =============================================================================
# 500 INTERNAL SERVER ERROR FIXTURES
# =============================================================================

def internal_server_error_fixture(
    path: str = "/api/users",
    message: Optional[str] = None,
    error_id: Optional[str] = None,
    include_stack_trace: bool = False
) -> ErrorResponseFixture:
    """
    Create a 500 Internal Server Error response fixture.

    Args:
        path: The resource path that caused the error
        message: Optional custom error message
        error_id: Unique error identifier for tracking
        include_stack_trace: Whether to include stack trace (for debugging)

    Returns:
        ErrorResponseFixture: Configured 500 error response

    Examples:
        >>> fixture = internal_server_error_fixture(
        ...     path="/api/users",
        ...     error_id="ERR-12345"
        ... )
        >>> assert fixture.status_code == 500
        >>> assert 'request_id' in fixture.body
    """
    import time
    import uuid

    if message is None:
        message = "An internal server error occurred"

    if error_id is None:
        error_id = f"ERR-{int(time.time())}-{uuid.uuid4().hex[:8].upper()}"

    body = {
        'error': 'internal_server_error',
        'message': message,
        'code': 500,
        'details': {
            'path': path,
            'error_id': error_id,
            'timestamp': '2026-07-14T00:00:00Z'
        }
    }

    if include_stack_trace:
        body['details']['stack_trace'] = [
            '  File "/app/api/handlers.py", line 45, in process_request',
            '    result = database.query(query)',
            '  File "/app/db/connection.py", line 123, in query',
            '    raise ConnectionError("Database timeout")'
        ]

    return ErrorResponseFixture(
        status_code=500,
        headers={
            'Content-Type': 'application/json',
            'X-Error-ID': error_id,
            'X-Error-Type': 'internal_server_error'
        },
        body=body
    )


def internal_server_error_database(
    path: str = "/api/users",
    db_error: str = "Database connection timeout"
) -> ErrorResponseFixture:
    """
    500 error for database-related failures.

    Args:
        path: The resource path
        db_error: Specific database error description

    Returns:
        ErrorResponseFixture: 500 error for database failure
    """
    return internal_server_error_fixture(
        path=path,
        message=f"Database error: {db_error}",
        include_stack_trace=False
    )


def internal_server_error_external_service(
    path: str = "/api/users",
    service_name: str = "Payment API"
) -> ErrorResponseFixture:
    """
    500 error for external service failures.

    Args:
        path: The resource path
        service_name: Name of the external service that failed

    Returns:
        ErrorResponseFixture: 500 error for external service failure
    """
    return internal_server_error_fixture(
        path=path,
        message=f"External service '{service_name}' is unavailable"
    )


# =============================================================================
# GENERIC ERROR BUILDERS
# =============================================================================

def create_error_response(
    status: int,
    error_type: str,
    message: str,
    code: Optional[int] = None,
    details: Optional[Dict[str, Any]] = None,
    headers: Optional[Dict[str, str]] = None
) -> ErrorResponseFixture:
    """
    Create a custom error response with any status code and fields.

    Args:
        status: HTTP status code
        error_type: Type identifier for the error
        message: Human-readable error message
        code: Optional error code (defaults to status if not provided)
        details: Optional additional details dictionary
        headers: Optional custom headers

    Returns:
        ErrorResponseFixture: Custom error response

    Examples:
        >>> # Custom 418 error
        >>> response = create_error_response(
        ...     status=418,
        ...     error_type="im_a_teapot",
        ...     message="I'm a teapot",
        ...     code=418
        ... )
        >>>
        >>> # Custom 403 error with specific details
        >>> response = create_error_response(
        ...     status=403,
        ...     error_type="forbidden",
        ...     message="Access denied",
        ...     details={'permission': 'admin', 'required': True}
        ... )
    """
    if code is None:
        code = status

    if headers is None:
        headers = {}

    if 'Content-Type' not in headers:
        headers['Content-Type'] = 'application/json'

    body = {
        'error': error_type,
        'message': message,
        'code': code
    }

    if details:
        body['details'] = details

    return ErrorResponseFixture(
        status_code=status,
        headers=headers,
        body=body
    )


def create_error_batch(
    errors: List[Dict[str, Any]]
) -> List[ErrorResponseFixture]:
    """
    Create multiple error responses from a list of error specifications.

    Args:
        errors: List of error specification dictionaries, each containing:
                - status (int): HTTP status code
                - error_type (str): Error type identifier
                - message (str): Error message
                - Optional: code, details, headers, path, method

    Returns:
        List[ErrorResponseFixture]: List of configured error responses

    Examples:
        >>> errors = create_error_batch([
        ...     {'status': 404, 'error_type': 'not_found', 'message': 'User not found', 'path': '/api/users/1'},
        ...     {'status': 403, 'error_type': 'forbidden', 'message': 'Access denied'},
        ...     {'status': 500, 'error_type': 'server_error', 'message': 'Internal error'}
        ... ])
        >>> len(errors)
        3
    """
    fixtures = []
    for error_spec in errors:
        status = error_spec.get('status', 500)
        error_type = error_spec.get('error_type', 'error')
        message = error_spec.get('message', 'An error occurred')

        # Extract optional fields
        details = error_spec.get('details', {})
        headers = error_spec.get('headers', {})

        # Add path/method to details if provided
        if 'path' in error_spec:
            details['path'] = error_spec['path']
        if 'method' in error_spec:
            details['method'] = error_spec['method']

        fixture = create_error_response(
            status=status,
            error_type=error_type,
            message=message,
            code=error_spec.get('code'),
            details=details if details else None,
            headers=headers if headers else None
        )
        fixtures.append(fixture)

    return fixtures


# =============================================================================
# PRE-CONFIGURED FIXTURE COLLECTIONS
# =============================================================================

COMMON_ERROR_FIXTURES = {
    'not_found': not_found_fixture(),
    'not_found_simple': not_found_simple(),
    'method_not_allowed': method_not_allowed_fixture(),
    'unsupported_media_type': unsupported_media_type_fixture(),
    'internal_server_error': internal_server_error_fixture()
}

CLIENT_ERROR_FIXTURES = {
    '400_bad_request': create_error_response(400, 'bad_request', 'Invalid request'),
    '401_unauthorized': create_error_response(401, 'unauthorized', 'Authentication required'),
    '403_forbidden': create_error_response(403, 'forbidden', 'Access denied'),
    '404_not_found': not_found_fixture(),
    '405_method_not_allowed': method_not_allowed_fixture(),
    '409_conflict': create_error_response(409, 'conflict', 'Resource conflict'),
    '415_unsupported_media_type': unsupported_media_type_fixture(),
    '422_unprocessable_entity': create_error_response(422, 'unprocessable_entity', 'Invalid data'),
    '429_rate_limited': create_error_response(429, 'rate_limited', 'Too many requests')
}

SERVER_ERROR_FIXTURES = {
    '500_internal_server_error': internal_server_error_fixture(),
    '502_bad_gateway': create_error_response(502, 'bad_gateway', 'Upstream service unavailable'),
    '503_service_unavailable': create_error_response(503, 'service_unavailable', 'Service temporarily unavailable'),
    '504_gateway_timeout': create_error_response(504, 'gateway_timeout', 'Upstream service timeout')
}

ALL_ERROR_FIXTURES = {**CLIENT_ERROR_FIXTURES, **SERVER_ERROR_FIXTURES}


def get_fixture(name: str) -> Optional[ErrorResponseFixture]:
    """
    Retrieve a pre-configured fixture by name.

    Args:
        name: Fixture name (e.g., 'not_found', '404_not_found', '500_internal_server_error')

    Returns:
        ErrorResponseFixture or None: The fixture if found, None otherwise

    Examples:
        >>> fixture = get_fixture('not_found')
        >>> assert fixture.status_code == 404
        >>>
        >>> fixture = get_fixture('500_internal_server_error')
        >>> assert fixture.status_code == 500
    """
    return ALL_ERROR_FIXTURES.get(name)


def list_fixtures() -> List[str]:
    """
    List all available pre-configured fixture names.

    Returns:
        List[str]: Sorted list of fixture names
    """
    return sorted(ALL_ERROR_FIXTURES.keys())
