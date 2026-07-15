"""
Test Fixtures Package for ARMOR

This package provides reusable test fixtures for common HTTP error scenarios
and other testing utilities.

Modules:
    error_scenarios: HTTP error response fixtures (404, 405, 415, 500, etc.)

Usage:
    >>> from tests.fixtures.error_scenarios import (
    ...     not_found_fixture,
    ...     method_not_allowed_fixture,
    ...     unsupported_media_type_fixture,
    ...     internal_server_error_fixture,
    ...     create_error_response
    ... )
    >>>
    >>> # Create a 404 error for a specific path
    >>> error_404 = not_found_fixture(path="/api/users/123")
    >>> assert error_404.status_code == 404

Bead: bf-7d2vgf
Created: 2026-07-14
"""

from tests.fixtures.error_scenarios import (  # noqa: F401
    ErrorResponseFixture,
    not_found_fixture,
    not_found_simple,
    not_found_with_suggestions,
    method_not_allowed_fixture,
    method_not_allowed_read_only,
    method_not_allowed_write_only,
    unsupported_media_type_fixture,
    unsupported_media_type_json_only,
    unsupported_media_type_missing,
    internal_server_error_fixture,
    internal_server_error_database,
    internal_server_error_external_service,
    unauthorized_fixture,
    forbidden_fixture,
    invalid_token_fixture,
    expired_token_fixture,
    wrong_password_fixture,
    missing_api_key_fixture,
    invalid_api_key_fixture,
    create_error_response,
    create_error_batch,
    get_fixture,
    list_fixtures,
    COMMON_ERROR_FIXTURES,
    CLIENT_ERROR_FIXTURES,
    SERVER_ERROR_FIXTURES,
    ALL_ERROR_FIXTURES
)

__all__ = [
    'ErrorResponseFixture',
    'not_found_fixture',
    'not_found_simple',
    'not_found_with_suggestions',
    'method_not_allowed_fixture',
    'method_not_allowed_read_only',
    'method_not_allowed_write_only',
    'unsupported_media_type_fixture',
    'unsupported_media_type_json_only',
    'unsupported_media_type_missing',
    'internal_server_error_fixture',
    'internal_server_error_database',
    'internal_server_error_external_service',
    'unauthorized_fixture',
    'forbidden_fixture',
    'invalid_token_fixture',
    'expired_token_fixture',
    'wrong_password_fixture',
    'missing_api_key_fixture',
    'invalid_api_key_fixture',
    'create_error_response',
    'create_error_batch',
    'get_fixture',
    'list_fixtures',
    'COMMON_ERROR_FIXTURES',
    'CLIENT_ERROR_FIXTURES',
    'SERVER_ERROR_FIXTURES',
    'ALL_ERROR_FIXTURES'
]
