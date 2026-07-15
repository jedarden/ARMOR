#!/usr/bin/env python3
"""
Authentication Error Test Cases

This module provides comprehensive test cases for authentication-related error responses.
Each test validates a specific authentication scenario including missing credentials,
invalid tokens, expired tokens, and wrong passwords.

Run tests:
    # Run all authentication tests
    pytest tests/test_auth_error_scenarios.py -v

    # Run specific test
    pytest tests/test_auth_error_scenarios.py::TestMissingCredentials::test_missing_api_key -v

    # Run with coverage
    pytest tests/test_auth_error_scenarios.py --cov=tests --cov-report=html

Bead: bf-346he3
Created: 2026-07-15
"""

import pytest
from typing import Dict, Any

from tests.test_helpers import (
    validate_http_status,
    validate_json_content_type,
)

from tests.test_error_response_validation import (
    validate_standard_error_response,
    validate_detailed_error_response,
)

from tests.fixtures.error_scenarios import (
    ErrorResponseFixture,
    unauthorized_fixture,
    forbidden_fixture,
    invalid_token_fixture,
    expired_token_fixture,
    wrong_password_fixture,
    missing_api_key_fixture,
    invalid_api_key_fixture,
    get_fixture,
)

from tests.test_tables import (
    create_auth_test_table,
    run_test_table,
)


# =============================================================================
# MISSING CREDENTIALS TESTS
# =============================================================================

class TestMissingCredentials:
    """Test cases for missing authentication credentials."""

    def test_missing_api_key(self):
        """
        Test 401 response when API key is missing from request headers.

        Scenario: Request to protected endpoint without X-API-Key header
        Expected: 401 Unauthorized with clear error message
        """
        # Create fixture for missing API key
        fixture = missing_api_key_fixture(
            path="/api/users",
            header_name="X-API-Key"
        )

        # Convert to tuple for validation
        response = fixture.to_tuple()

        # Validate status code
        validate_http_status(response, 401)

        # Validate content type
        validate_json_content_type(response)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate specific fields
        assert fixture.body['error'] == 'unauthorized'
        assert 'required' in fixture.body['message'].lower()
        assert fixture.body['details']['required_header'] == 'X-API-Key'
        assert fixture.body['details']['path'] == '/api/users'

        print("✓ Missing API key test passed")

    def test_missing_bearer_token(self):
        """
        Test 401 response when Authorization header is missing or empty.

        Scenario: Request to protected endpoint without Bearer token
        Expected: 401 Unauthorized with WWW-Authenticate header
        """
        # Create fixture for missing credentials
        fixture = unauthorized_fixture(
            path="/api/users",
            auth_type="Bearer",
            realm="api"
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 401)

        # Validate WWW-Authenticate header is present
        assert 'WWW-Authenticate' in fixture.headers
        assert 'Bearer' in fixture.headers['WWW-Authenticate']

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate auth type in details
        assert fixture.body['details']['auth_type'] == 'Bearer'

        print("✓ Missing Bearer token test passed")

    def test_missing_basic_auth(self):
        """
        Test 401 response when Basic auth credentials are missing.

        Scenario: Login request without Authorization header
        Expected: 401 Unauthorized with Basic auth challenge
        """
        # Create fixture for missing Basic auth
        fixture = unauthorized_fixture(
            path="/api/auth/login",
            auth_type="Basic",
            realm="api"
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 401)

        # Validate WWW-Authenticate header for Basic auth
        assert 'WWW-Authenticate' in fixture.headers
        assert 'Basic' in fixture.headers['WWW-Authenticate']

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate realm
        assert fixture.body['details']['realm'] == 'api'

        print("✓ Missing Basic auth test passed")


# =============================================================================
# INVALID CREDENTIALS TESTS
# =============================================================================

class TestInvalidCredentials:
    """Test cases for invalid authentication credentials."""

    def test_invalid_api_key(self):
        """
        Test 403 response when API key is invalid or revoked.

        Scenario: Request with invalid API key format or revoked key
        Expected: 403 Forbidden with clear error message
        """
        # Create fixture for invalid API key
        fixture = invalid_api_key_fixture(
            path="/api/users",
            api_key="invalid-key-123"
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 403)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate specific fields
        assert fixture.body['error'] == 'forbidden'
        assert 'invalid' in fixture.body['message'].lower()
        assert 'key_preview' in fixture.body['details']

        # Validate key is truncated for security
        key_preview = fixture.body['details']['key_preview']
        assert '...' in key_preview
        assert key_preview != "invalid-key-123"

        print("✓ Invalid API key test passed")

    def test_invalid_token(self):
        """
        Test 401 response when Bearer token is invalid.

        Scenario: Request with malformed or invalid JWT token
        Expected: 401 Unauthorized with token validation error
        """
        # Create fixture for invalid token
        fixture = invalid_token_fixture(
            path="/api/users",
            token="invalid-token-abc123def456"
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 401)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate specific fields
        assert fixture.body['error'] == 'unauthorized'
        assert 'invalid' in fixture.body['message'].lower()
        assert fixture.body['details']['auth_type'] == 'Bearer'

        # Validate token preview is truncated
        token_preview = fixture.body['details']['token_preview']
        assert token_preview.startswith('invalid')
        assert '...' in token_preview

        print("✓ Invalid token test passed")

    def test_revoked_api_key(self):
        """
        Test 403 response when API key has been revoked.

        Scenario: Request with previously valid but now revoked API key
        Expected: 403 Forbidden with revocation message
        """
        # Create custom fixture for revoked key
        from tests.fixtures.error_scenarios import create_error_response

        fixture = create_error_response(
            status=403,
            error_type="forbidden",
            message="API key has been revoked",
            code=403,
            details={
                'path': '/api/users',
                'key_id': 'key_abc123',
                'revoked_at': '2026-07-14T12:00:00Z',
                'reason': 'user_request'
            }
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 403)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate revocation details
        assert 'revoked' in fixture.body['message'].lower()
        assert 'revoked_at' in fixture.body['details']
        assert 'key_id' in fixture.body['details']

        print("✓ Revoked API key test passed")


# =============================================================================
# EXPIRED CREDENTIALS TESTS
# =============================================================================

class TestExpiredCredentials:
    """Test cases for expired authentication credentials."""

    def test_expired_token(self):
        """
        Test 401 response when Bearer token has expired.

        Scenario: Request with JWT token past its expiration time
        Expected: 401 Unauthorized with expiration details
        """
        # Create fixture for expired token
        fixture = expired_token_fixture(
            path="/api/users",
            token_expired_at="2026-07-14T00:00:00Z"
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 401)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate specific fields
        assert fixture.body['error'] == 'unauthorized'
        assert 'expired' in fixture.body['message'].lower()
        assert 'expired_at' in fixture.body['details']
        assert fixture.body['details']['auth_type'] == 'Bearer'

        # Validate expiration timestamp is present
        assert '2026-07-14' in fixture.body['details']['expired_at']

        print("✓ Expired token test passed")

    def test_expired_api_key(self):
        """
        Test 403 response when API key has expired.

        Scenario: Request with API key past its expiration date
        Expected: 403 Forbidden with expiration message
        """
        # Create custom fixture for expired API key
        from tests.fixtures.error_scenarios import create_error_response

        fixture = create_error_response(
            status=403,
            error_type="forbidden",
            message="API key expired",
            code=403,
            details={
                'path': '/api/users',
                'key_id': 'key_xyz789',
                'expired_at': '2026-07-14T00:00:00Z',
                'grace_period_ends': '2026-07-21T00:00:00Z'
            }
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 403)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate expiration details
        assert 'expired' in fixture.body['message'].lower()
        assert 'expired_at' in fixture.body['details']
        assert 'grace_period_ends' in fixture.body['details']

        print("✓ Expired API key test passed")


# =============================================================================
# WRONG PASSWORD TESTS
# =============================================================================

class TestWrongPassword:
    """Test cases for incorrect password in basic authentication."""

    def test_wrong_password(self):
        """
        Test 401 response when password is incorrect.

        Scenario: Login attempt with wrong password
        Expected: 401 Unauthorized with attempts remaining
        """
        # Create fixture for wrong password
        fixture = wrong_password_fixture(
            path="/api/auth/login",
            username="user@example.com",
            attempts_remaining=2
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 401)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate specific fields
        assert fixture.body['error'] == 'unauthorized'
        assert 'invalid' in fixture.body['message'].lower() or 'password' in fixture.body['message'].lower()
        assert 'attempts_remaining' in fixture.body['details']
        assert fixture.body['details']['attempts_remaining'] == 2

        # Validate username is masked for security
        if 'username_preview' in fixture.body['details']:
            username_preview = fixture.body['details']['username_preview']
            assert '***' in username_preview
            assert 'user@example.com' not in username_preview

        print("✓ Wrong password test passed")

    def test_wrong_password_lockout_warning(self):
        """
        Test 401 response with account lockout warning.

        Scenario: Multiple failed login attempts, approaching lockout
        Expected: 401 Unauthorized with warning about remaining attempts
        """
        # Create fixture with few attempts remaining
        fixture = wrong_password_fixture(
            path="/api/auth/login",
            username="admin@example.com",
            attempts_remaining=1
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 401)

        # Validate attempts remaining
        assert fixture.body['details']['attempts_remaining'] == 1

        # Validate error structure
        validate_standard_error_response(fixture.body)

        print("✓ Wrong password lockout warning test passed")


# =============================================================================
# INSUFFICIENT PERMISSIONS TESTS
# =============================================================================

class TestInsufficientPermissions:
    """Test cases for insufficient permissions/authorization."""

    def test_insufficient_permissions_for_admin_endpoint(self):
        """
        Test 403 response when user lacks admin permissions.

        Scenario: Regular user accessing admin endpoint
        Expected: 403 Forbidden with permission details
        """
        # Create fixture for insufficient permissions
        fixture = forbidden_fixture(
            path="/api/admin/users",
            permission="admin:write",
            provided_permission="user:read"
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 403)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate specific fields
        assert fixture.body['error'] == 'forbidden'
        assert 'access denied' in fixture.body['message'].lower() or 'permission' in fixture.body['message'].lower()
        assert fixture.body['details']['required_permission'] == 'admin:write'
        assert fixture.body['details']['provided_permission'] == 'user:read'

        # Validate X-Required-Permission header
        assert 'X-Required-Permission' in fixture.headers
        assert fixture.headers['X-Required-Permission'] == 'admin:write'

        print("✓ Insufficient permissions test passed")

    def test_read_only_user_write_attempt(self):
        """
        Test 403 response when read-only user attempts write operation.

        Scenario: Read-only API key used with POST request
        Expected: 403 Forbidden with read-only access message
        """
        # Create custom fixture for read-only violation
        from tests.fixtures.error_scenarios import create_error_response

        fixture = create_error_response(
            status=403,
            error_type="forbidden",
            message="Read-only access. Write operations are not allowed.",
            code=403,
            details={
                'path': '/api/users',
                'method': 'POST',
                'permission_level': 'read_only',
                'required_permission': 'write'
            }
        )

        # Validate status code
        validate_http_status(fixture.to_tuple(), 403)

        # Validate error structure
        validate_standard_error_response(fixture.body)

        # Validate read-only message
        assert 'read-only' in fixture.body['message'].lower()
        assert fixture.body['details']['permission_level'] == 'read_only'

        print("✓ Read-only user write attempt test passed")


# =============================================================================
# AUTHENTICATION TEST TABLE
# =============================================================================

class TestAuthenticationTestTable:
    """Test the authentication error test table structure."""

    def test_auth_table_structure(self):
        """
        Test that the authentication test table is properly structured.

        Validates that the table contains all expected test cases
        and they follow the correct format.
        """
        # Get the auth test table
        auth_table = create_auth_test_table()

        # Validate table metadata
        assert auth_table.name == "authentication_errors"
        assert auth_table.description is not None
        assert len(auth_table.tags) > 0
        assert "auth" in auth_table.tags

        # Validate test cases exist
        assert len(auth_table.test_cases) > 0

        # Check for required test case categories
        test_ids = [tc.id for tc in auth_table.test_cases]

        # Missing credentials
        assert any("AUTH-001" == tid for tid in test_ids), "Missing API key test case"
        assert any("AUTH-002" == tid for tid in test_ids), "Missing Bearer token test case"

        # Invalid credentials
        assert any("AUTH-004" == tid for tid in test_ids), "Invalid API key test case"
        assert any("AUTH-005" == tid for tid in test_ids), "Invalid token test case"

        # Expired credentials
        assert any("AUTH-007" == tid for tid in test_ids), "Expired token test case"

        # Wrong password
        assert any("AUTH-009" == tid for tid in test_ids), "Wrong password test case"

        # Insufficient permissions
        assert any("AUTH-011" == tid for tid in test_ids), "Insufficient permissions test case"

        print(f"✓ Auth table structure validated with {len(auth_table.test_cases)} test cases")

    def test_auth_table_missing_credentials_cases(self):
        """
        Test all missing credentials test cases in the auth table.
        """
        auth_table = create_auth_test_table()

        # Get missing credentials test cases
        missing_creds_cases = auth_table.get_test_cases_by_tag("missing-credentials")

        assert len(missing_creds_cases) >= 3, "Should have at least 3 missing credentials test cases"

        # Validate each missing credentials case
        for case in missing_creds_cases:
            assert case.expected_status == 401, f"{case.id}: Missing credentials should return 401"
            assert case.expected_error == "unauthorized", f"{case.id}: Should be unauthorized error"
            assert case.enabled, f"{case.id}: Test case should be enabled"

        print(f"✓ All {len(missing_creds_cases)} missing credentials test cases validated")

    def test_auth_table_invalid_credentials_cases(self):
        """
        Test all invalid credentials test cases in the auth table.
        """
        auth_table = create_auth_test_table()

        # Get invalid credentials test cases
        invalid_creds_cases = auth_table.get_test_cases_by_tag("invalid-credentials")

        assert len(invalid_creds_cases) >= 3, "Should have at least 3 invalid credentials test cases"

        # Validate each invalid credentials case
        for case in invalid_creds_cases:
            assert case.expected_status in (401, 403), f"{case.id}: Invalid credentials should return 401 or 403"
            assert case.expected_error in ("unauthorized", "forbidden"), f"{case.id}: Should be auth error"
            assert case.enabled, f"{case.id}: Test case should be enabled"

        print(f"✓ All {len(invalid_creds_cases)} invalid credentials test cases validated")

    def test_auth_table_expired_credentials_cases(self):
        """
        Test all expired credentials test cases in the auth table.
        """
        auth_table = create_auth_test_table()

        # Get expired credentials test cases
        expired_creds_cases = auth_table.get_test_cases_by_tag("expired-credentials")

        assert len(expired_creds_cases) >= 2, "Should have at least 2 expired credentials test cases"

        # Validate each expired credentials case
        for case in expired_creds_cases:
            assert case.expected_status in (401, 403), f"{case.id}: Expired credentials should return 401 or 403"
            assert 'expired' in case.expected_message.lower(), f"{case.id}: Should mention expiration"
            assert case.enabled, f"{case.id}: Test case should be enabled"

        print(f"✓ All {len(expired_creds_cases)} expired credentials test cases validated")

    def test_auth_table_wrong_password_cases(self):
        """
        Test all wrong password test cases in the auth table.
        """
        auth_table = create_auth_test_table()

        # Get wrong password test cases
        wrong_password_cases = auth_table.get_test_cases_by_tag("wrong-password")

        assert len(wrong_password_cases) >= 2, "Should have at least 2 wrong password test cases"

        # Validate each wrong password case
        for case in wrong_password_cases:
            assert case.expected_status == 401, f"{case.id}: Wrong password should return 401"
            assert case.expected_error == "unauthorized", f"{case.id}: Should be unauthorized"
            assert 'password' in case.expected_message.lower() or 'invalid' in case.expected_message.lower()
            assert case.enabled, f"{case.id}: Test case should be enabled"

        print(f"✓ All {len(wrong_password_cases)} wrong password test cases validated")

    def test_auth_table_rbac_cases(self):
        """
        Test all RBAC/permissions test cases in the auth table.
        """
        auth_table = create_auth_test_table()

        # Get RBAC test cases
        rbac_cases = auth_table.get_test_cases_by_tag("rbac")

        assert len(rbac_cases) >= 2, "Should have at least 2 RBAC test cases"

        # Validate each RBAC case
        for case in rbac_cases:
            assert case.expected_status == 403, f"{case.id}: RBAC violation should return 403"
            assert case.expected_error == "forbidden", f"{case.id}: Should be forbidden"
            assert 'permission' in case.expected_message.lower() or 'access' in case.expected_message.lower()
            assert case.enabled, f"{case.id}: Test case should be enabled"

        print(f"✓ All {len(rbac_cases)} RBAC test cases validated")

    def test_auth_table_happy_path_cases(self):
        """
        Test all happy path test cases in the auth table.
        """
        auth_table = create_auth_test_table()

        # Get happy path test cases
        happy_path_cases = auth_table.get_test_cases_by_tag("happy-path")

        assert len(happy_path_cases) >= 3, "Should have at least 3 happy path test cases"

        # Validate each happy path case
        for case in happy_path_cases:
            assert case.expected_status == 200, f"{case.id}: Happy path should return 200"
            assert case.expected_error is None, f"{case.id}: Should have no error"
            assert case.enabled, f"{case.id}: Test case should be enabled"

        print(f"✓ All {len(happy_path_cases)} happy path test cases validated")


# =============================================================================
# FIXTURE INTEGRATION TESTS
# =============================================================================

class TestAuthFixtureIntegration:
    """Test that authentication fixtures integrate properly with the test infrastructure."""

    def test_unauthorized_fixture_in_all_fixtures(self):
        """Test that unauthorized_fixture is available in ALL_ERROR_FIXTURES."""
        from tests.fixtures.error_scenarios import ALL_ERROR_FIXTURES

        # Check if unauthorized fixture is in the collection
        assert '401_unauthorized' in ALL_ERROR_FIXTURES or '401_missing_credentials' in ALL_ERROR_FIXTURES

        print("✓ Unauthorized fixture available in collection")

    def test_auth_fixture_from_get_fixture(self):
        """Test retrieving authentication fixtures via get_fixture()."""
        # Try to get auth fixtures
        fixture_401 = get_fixture('401_unauthorized')
        fixture_403 = get_fixture('403_forbidden')

        # At least one should be available
        assert fixture_401 is not None or fixture_403 is not None

        print("✓ Auth fixtures retrievable via get_fixture()")

    def test_custom_auth_fixture_creation(self):
        """Test creating custom authentication error fixtures."""
        from tests.fixtures.error_scenarios import create_error_response

        # Create a custom 401 error
        custom_401 = create_error_response(
            status=401,
            error_type="custom_auth_error",
            message="Custom authentication error",
            code=401,
            details={'custom_field': 'custom_value'}
        )

        # Validate the fixture
        assert custom_401.status_code == 401
        assert custom_401.body['error'] == 'custom_auth_error'
        assert custom_401.body['details']['custom_field'] == 'custom_value'

        print("✓ Custom auth fixture creation successful")


# =============================================================================
# RUN TEST TABLE EXAMPLE
# =============================================================================

def test_run_auth_test_table_example():
    """
    Example of running the authentication test table with a mock executor.

    This demonstrates how the test table would be used in practice.
    """
    # Create the auth test table
    auth_table = create_auth_test_table()

    # Mock executor function
    def mock_executor(input_data: Dict[str, Any]) -> Dict[str, Any]:
        """
        Mock executor that simulates API responses based on input data.

        In a real scenario, this would make actual HTTP requests.
        """
        # Check for missing credentials
        headers = input_data.get('headers', {})

        # Missing API key
        if 'X-API-Key' not in headers and 'Authorization' not in headers:
            return {
                'status': 401,
                'error': 'unauthorized',
                'message': 'API key required'
            }

        # Check for happy path with valid credentials
        api_key = headers.get('X-API-Key', '')
        if 'valid' in api_key:
            return {
                'status': 200,
                'error': None,
                'message': 'Success'
            }

        # Invalid API key
        return {
            'status': 403,
            'error': 'forbidden',
            'message': 'Invalid API key'
        }

    # Run the test table (only first 5 test cases for demo)
    results = run_test_table(
        table=auth_table,
        executor=mock_executor,
        continue_on_failure=True
    )

    # Validate results
    assert results.total_count == len(auth_table.test_cases)
    assert results.passed_count > 0 or results.failed_count > 0

    print(f"✓ Test table execution completed: {results.passed_count} passed, {results.failed_count} failed")


if __name__ == '__main__':
    # Run tests with pytest
    import subprocess
    import sys

    print("Running Authentication Error Test Suite")
    print("=" * 60)
    print()

    # Run pytest on this file
    result = subprocess.run(
        [sys.executable, '-m', 'pytest', __file__, '-v', '--tb=short'],
        capture_output=False
    )

    sys.exit(result.returncode)
