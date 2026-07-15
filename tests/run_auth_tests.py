#!/usr/bin/env python3
"""
Standalone Authentication Error Test Runner

This script runs authentication error tests without requiring pytest.
It demonstrates the authentication error test cases working end-to-end.

Usage:
    python3 tests/run_auth_tests.py

Bead: bf-346he3
Created: 2026-07-15
"""

import sys
import json
from typing import Dict, Any, List

# Add project root to path
sys.path.insert(0, '/home/coding/ARMOR')

from tests.test_helpers import (
    validate_http_status,
    validate_json_content_type,
)

from tests.test_error_response_validation import (
    validate_standard_error_response,
)

from tests.fixtures.error_scenarios import (
    unauthorized_fixture,
    forbidden_fixture,
    invalid_token_fixture,
    expired_token_fixture,
    wrong_password_fixture,
    missing_api_key_fixture,
    invalid_api_key_fixture,
)

from tests.test_tables import (
    create_auth_test_table,
)


# =============================================================================
# TEST RESULT TRACKING
# =============================================================================

class TestResults:
    """Track test results."""

    def __init__(self):
        self.total = 0
        self.passed = 0
        self.failed = 0
        self.errors = []

    def add_pass(self, test_name: str):
        """Record a passed test."""
        self.total += 1
        self.passed += 1
        print(f"  ✓ {test_name}")

    def add_fail(self, test_name: str, error: str):
        """Record a failed test."""
        self.total += 1
        self.failed += 1
        print(f"  ✗ {test_name}")
        print(f"    Error: {error}")
        self.errors.append((test_name, error))

    def print_summary(self):
        """Print test summary."""
        print()
        print("=" * 60)
        print(f"Test Results: {self.passed}/{self.total} passed")
        if self.failed > 0:
            print(f"Failed tests: {self.failed}")
            for name, error in self.errors:
                print(f"  - {name}: {error}")
        print("=" * 60)
        return self.failed == 0


results = TestResults()


# =============================================================================
# TEST FUNCTIONS
# =============================================================================

def test_missing_api_key():
    """Test 401 response when API key is missing."""
    try:
        fixture = missing_api_key_fixture(path="/api/users")
        response = fixture.to_tuple()

        validate_http_status(response, 401)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert fixture.body['error'] == 'unauthorized'
        assert 'required' in fixture.body['message'].lower()

        results.add_pass("Missing API Key")
    except Exception as e:
        results.add_fail("Missing API Key", str(e))


def test_missing_bearer_token():
    """Test 401 response when Bearer token is missing."""
    try:
        fixture = unauthorized_fixture(path="/api/users", auth_type="Bearer")
        response = fixture.to_tuple()

        validate_http_status(response, 401)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert 'WWW-Authenticate' in fixture.headers
        assert 'Bearer' in fixture.headers['WWW-Authenticate']

        results.add_pass("Missing Bearer Token")
    except Exception as e:
        results.add_fail("Missing Bearer Token", str(e))


def test_missing_basic_auth():
    """Test 401 response when Basic auth is missing."""
    try:
        fixture = unauthorized_fixture(path="/api/auth/login", auth_type="Basic")
        response = fixture.to_tuple()

        validate_http_status(response, 401)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert 'WWW-Authenticate' in fixture.headers
        assert 'Basic' in fixture.headers['WWW-Authenticate']

        results.add_pass("Missing Basic Auth")
    except Exception as e:
        results.add_fail("Missing Basic Auth", str(e))


def test_invalid_api_key():
    """Test 403 response when API key is invalid."""
    try:
        fixture = invalid_api_key_fixture(path="/api/users", api_key="invalid-key-123")
        response = fixture.to_tuple()

        validate_http_status(response, 403)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert fixture.body['error'] == 'forbidden'
        assert 'invalid' in fixture.body['message'].lower()
        assert 'key_preview' in fixture.body['details']

        results.add_pass("Invalid API Key")
    except Exception as e:
        results.add_fail("Invalid API Key", str(e))


def test_invalid_token():
    """Test 401 response when Bearer token is invalid."""
    try:
        fixture = invalid_token_fixture(path="/api/users", token="invalid-token-abc")
        response = fixture.to_tuple()

        validate_http_status(response, 401)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert fixture.body['error'] == 'unauthorized'
        assert 'invalid' in fixture.body['message'].lower()
        assert fixture.body['details']['auth_type'] == 'Bearer'

        results.add_pass("Invalid Token")
    except Exception as e:
        results.add_fail("Invalid Token", str(e))


def test_expired_token():
    """Test 401 response when Bearer token has expired."""
    try:
        fixture = expired_token_fixture(
            path="/api/users",
            token_expired_at="2026-07-14T00:00:00Z"
        )
        response = fixture.to_tuple()

        validate_http_status(response, 401)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert fixture.body['error'] == 'unauthorized'
        assert 'expired' in fixture.body['message'].lower()
        assert 'expired_at' in fixture.body['details']

        results.add_pass("Expired Token")
    except Exception as e:
        results.add_fail("Expired Token", str(e))


def test_wrong_password():
    """Test 401 response when password is incorrect."""
    try:
        fixture = wrong_password_fixture(
            path="/api/auth/login",
            username="user@example.com",
            attempts_remaining=2
        )
        response = fixture.to_tuple()

        validate_http_status(response, 401)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert fixture.body['error'] == 'unauthorized'
        assert 'attempts_remaining' in fixture.body['details']
        assert fixture.body['details']['attempts_remaining'] == 2

        results.add_pass("Wrong Password")
    except Exception as e:
        results.add_fail("Wrong Password", str(e))


def test_wrong_password_lockout_warning():
    """Test 401 response with account lockout warning."""
    try:
        fixture = wrong_password_fixture(
            path="/api/auth/login",
            username="admin@example.com",
            attempts_remaining=1
        )
        response = fixture.to_tuple()

        validate_http_status(response, 401)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert fixture.body['details']['attempts_remaining'] == 1

        results.add_pass("Wrong Password - Lockout Warning")
    except Exception as e:
        results.add_fail("Wrong Password - Lockout Warning", str(e))


def test_insufficient_permissions():
    """Test 403 response when user lacks admin permissions."""
    try:
        fixture = forbidden_fixture(
            path="/api/admin/users",
            permission="admin:write",
            provided_permission="user:read"
        )
        response = fixture.to_tuple()

        validate_http_status(response, 403)
        validate_json_content_type(response)
        validate_standard_error_response(fixture.body)

        assert fixture.body['error'] == 'forbidden'
        assert 'access denied' in fixture.body['message'].lower() or 'permission' in fixture.body['message'].lower()
        assert fixture.body['details']['required_permission'] == 'admin:write'

        results.add_pass("Insufficient Permissions")
    except Exception as e:
        results.add_fail("Insufficient Permissions", str(e))


def test_auth_table_structure():
    """Test that the authentication test table is properly structured."""
    try:
        auth_table = create_auth_test_table()

        assert auth_table.name == "authentication_errors"
        assert len(auth_table.test_cases) > 0

        test_ids = [tc.id for tc in auth_table.test_cases]

        # Check for required test cases
        required_tests = [
            "AUTH-001",  # Missing API key
            "AUTH-002",  # Missing Bearer token
            "AUTH-004",  # Invalid API key
            "AUTH-005",  # Invalid token
            "AUTH-007",  # Expired token
            "AUTH-009",  # Wrong password
            "AUTH-011",  # Insufficient permissions
        ]

        for test_id in required_tests:
            assert test_id in test_ids, f"Missing test case: {test_id}"

        results.add_pass("Auth Table Structure")
    except Exception as e:
        results.add_fail("Auth Table Structure", str(e))


def test_auth_table_missing_credentials_cases():
    """Test all missing credentials test cases."""
    try:
        auth_table = create_auth_test_table()
        missing_creds_cases = auth_table.get_test_cases_by_tag("missing-credentials")

        assert len(missing_creds_cases) >= 3

        for case in missing_creds_cases:
            assert case.expected_status == 401
            assert case.expected_error == "unauthorized"
            assert case.enabled

        results.add_pass(f"Missing Credentials Cases ({len(missing_creds_cases)} tests)")
    except Exception as e:
        results.add_fail("Missing Credentials Cases", str(e))


def test_auth_table_invalid_credentials_cases():
    """Test all invalid credentials test cases."""
    try:
        auth_table = create_auth_test_table()
        invalid_creds_cases = auth_table.get_test_cases_by_tag("invalid-credentials")

        assert len(invalid_creds_cases) >= 3

        for case in invalid_creds_cases:
            assert case.expected_status in (401, 403)
            assert case.expected_error in ("unauthorized", "forbidden")
            assert case.enabled

        results.add_pass(f"Invalid Credentials Cases ({len(invalid_creds_cases)} tests)")
    except Exception as e:
        results.add_fail("Invalid Credentials Cases", str(e))


def test_auth_table_expired_credentials_cases():
    """Test all expired credentials test cases."""
    try:
        auth_table = create_auth_test_table()
        expired_creds_cases = auth_table.get_test_cases_by_tag("expired-credentials")

        assert len(expired_creds_cases) >= 2

        for case in expired_creds_cases:
            assert case.expected_status in (401, 403)
            assert 'expired' in case.expected_message.lower()
            assert case.enabled

        results.add_pass(f"Expired Credentials Cases ({len(expired_creds_cases)} tests)")
    except Exception as e:
        results.add_fail("Expired Credentials Cases", str(e))


def test_auth_table_wrong_password_cases():
    """Test all wrong password test cases."""
    try:
        auth_table = create_auth_test_table()
        wrong_password_cases = auth_table.get_test_cases_by_tag("wrong-password")

        assert len(wrong_password_cases) >= 2

        for case in wrong_password_cases:
            assert case.expected_status == 401
            assert case.expected_error == "unauthorized"
            assert case.enabled

        results.add_pass(f"Wrong Password Cases ({len(wrong_password_cases)} tests)")
    except Exception as e:
        results.add_fail("Wrong Password Cases", str(e))


def test_auth_table_rbac_cases():
    """Test all RBAC/permissions test cases."""
    try:
        auth_table = create_auth_test_table()
        rbac_cases = auth_table.get_test_cases_by_tag("rbac")

        assert len(rbac_cases) >= 2

        for case in rbac_cases:
            assert case.expected_status == 403
            assert case.expected_error == "forbidden"
            assert case.enabled

        results.add_pass(f"RBAC Cases ({len(rbac_cases)} tests)")
    except Exception as e:
        results.add_fail("RBAC Cases", str(e))


# =============================================================================
# MAIN TEST RUNNER
# =============================================================================

def main():
    """Run all authentication error tests."""
    print()
    print("=" * 60)
    print("Authentication Error Test Suite")
    print("=" * 60)
    print()

    # Run individual fixture tests
    print("Missing Credentials Tests:")
    test_missing_api_key()
    test_missing_bearer_token()
    test_missing_basic_auth()

    print()
    print("Invalid Credentials Tests:")
    test_invalid_api_key()
    test_invalid_token()

    print()
    print("Expired Credentials Tests:")
    test_expired_token()

    print()
    print("Wrong Password Tests:")
    test_wrong_password()
    test_wrong_password_lockout_warning()

    print()
    print("Insufficient Permissions Tests:")
    test_insufficient_permissions()

    print()
    print("Test Table Structure Tests:")
    test_auth_table_structure()

    print()
    print("Test Table Coverage Tests:")
    test_auth_table_missing_credentials_cases()
    test_auth_table_invalid_credentials_cases()
    test_auth_table_expired_credentials_cases()
    test_auth_table_wrong_password_cases()
    test_auth_table_rbac_cases()

    # Print summary and exit
    all_passed = results.print_summary()

    if all_passed:
        print()
        print("✓ All authentication error tests passed!")
        return 0
    else:
        print()
        print("✗ Some tests failed. Please review the errors above.")
        return 1


if __name__ == '__main__':
    sys.exit(main())
