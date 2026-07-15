#!/usr/bin/env python3
"""
Runnable Authentication Error Test Cases

This script provides a standalone test runner for authentication error scenarios.
It uses the test table structure and creates actual test fixtures to validate
each authentication error case.

Run tests:
    python3 tests/runnable_auth_error_tests.py

    Run specific test category:
    python3 tests/runnable_auth_error_tests.py --category missing-credentials
    python3 tests/runnable_auth_error_tests.py --category invalid-credentials
    python3 tests/runnable_auth_error_tests.py --category expired-credentials
    python3 tests/runnable_auth_error_tests.py --category wrong-password

Bead: bf-346he3
Created: 2026-07-15
"""

import sys
import json
from typing import Dict, Any, List, Tuple
from dataclasses import dataclass

# Add the tests directory to the path
sys.path.insert(0, '/home/coding/ARMOR')

from tests.fixtures.error_scenarios import (
    unauthorized_fixture,
    forbidden_fixture,
    invalid_token_fixture,
    expired_token_fixture,
    wrong_password_fixture,
    missing_api_key_fixture,
    invalid_api_key_fixture,
)
from tests.test_helpers import (
    validate_http_status,
    validate_json_content_type,
)


# =============================================================================
# TEST RESULT STRUCTURES
# =============================================================================

@dataclass
class TestResult:
    """Result of a single test execution."""
    test_id: str
    description: str
    passed: bool
    expected_status: int
    actual_status: int
    expected_error: str
    actual_error: str
    expected_message: str
    actual_message: str
    execution_time_ms: float
    error_details: str = ""


@dataclass
class TestSuiteResult:
    """Result of running a complete test suite."""
    total_count: int
    passed_count: int
    failed_count: int
    skipped_count: int
    results: List[TestResult]
    execution_time_ms: float


# =============================================================================
# AUTHENTICATION ERROR TEST TABLE
# =============================================================================

@dataclass
class AuthTestCase:
    """A single authentication test case."""
    id: str
    description: str
    category: str
    input_data: Dict[str, Any]
    expected_status: int
    expected_error: str
    expected_message: str
    expected_fields: Dict[str, Any] = None

    def __post_init__(self):
        if self.expected_fields is None:
            self.expected_fields = {}


# Define authentication error test cases
AUTH_TEST_TABLE: List[AuthTestCase] = [
    # =============================================================================
    # MISSING CREDENTIALS TESTS
    # =============================================================================
    AuthTestCase(
        id="AUTH-001",
        description="Missing API key in request headers",
        category="missing-credentials",
        input_data={
            "endpoint": "/api/users",
            "headers": {},
            "fixture_type": "missing_api_key"
        },
        expected_status=401,
        expected_error="unauthorized",
        expected_message="API key required",
    ),
    AuthTestCase(
        id="AUTH-002",
        description="Missing Bearer token in Authorization header",
        category="missing-credentials",
        input_data={
            "endpoint": "/api/users",
            "headers": {"Authorization": ""},
            "fixture_type": "unauthorized_bearer"
        },
        expected_status=401,
        expected_error="unauthorized",
        expected_message="Authentication required",
    ),
    AuthTestCase(
        id="AUTH-003",
        description="Missing Basic auth credentials",
        category="missing-credentials",
        input_data={
            "endpoint": "/api/auth/login",
            "headers": {},
            "fixture_type": "unauthorized_basic"
        },
        expected_status=401,
        expected_error="unauthorized",
        expected_message="Authentication required",
    ),

    # =============================================================================
    # INVALID CREDENTIALS TESTS
    # =============================================================================
    AuthTestCase(
        id="AUTH-004",
        description="Invalid API key format",
        category="invalid-credentials",
        input_data={
            "endpoint": "/api/users",
            "headers": {"X-API-Key": "invalid-format-123"},
            "fixture_type": "invalid_api_key"
        },
        expected_status=403,
        expected_error="forbidden",
        expected_message="Invalid API key",
    ),
    AuthTestCase(
        id="AUTH-005",
        description="Invalid Bearer token format",
        category="invalid-credentials",
        input_data={
            "endpoint": "/api/users",
            "headers": {"Authorization": "Bearer invalid-token-abc123def456"},
            "fixture_type": "invalid_token"
        },
        expected_status=401,
        expected_error="unauthorized",
        expected_message="Invalid authentication token",
    ),
    AuthTestCase(
        id="AUTH-006",
        description="Revoked API key",
        category="invalid-credentials",
        input_data={
            "endpoint": "/api/users",
            "headers": {"X-API-Key": "revoked-key-456"},
            "fixture_type": "invalid_api_key"
        },
        expected_status=403,
        expected_error="forbidden",
        expected_message="API key has been revoked",
    ),

    # =============================================================================
    # EXPIRED CREDENTIALS TESTS
    # =============================================================================
    AuthTestCase(
        id="AUTH-007",
        description="Expired Bearer token",
        category="expired-credentials",
        input_data={
            "endpoint": "/api/users",
            "headers": {"Authorization": "Bearer expired-token-789"},
            "token_expired_at": "2026-07-14T00:00:00Z",
            "fixture_type": "expired_token"
        },
        expected_status=401,
        expected_error="unauthorized",
        expected_message="token expired",
    ),
    AuthTestCase(
        id="AUTH-008",
        description="Expired API key",
        category="expired-credentials",
        input_data={
            "endpoint": "/api/users",
            "headers": {"X-API-Key": "expired-key-abc"},
            "key_expired_at": "2026-07-14T00:00:00Z",
            "fixture_type": "expired_api_key"
        },
        expected_status=403,
        expected_error="forbidden",
        expected_message="API key expired",
    ),

    # =============================================================================
    # WRONG PASSWORD TESTS
    # =============================================================================
    AuthTestCase(
        id="AUTH-009",
        description="Wrong password for basic authentication",
        category="wrong-password",
        input_data={
            "endpoint": "/api/auth/login",
            "username": "user@example.com",
            "password": "wrongpassword",
            "fixture_type": "wrong_password"
        },
        expected_status=401,
        expected_error="unauthorized",
        expected_message="Invalid username or password",
        expected_fields={"attempts_remaining": 2}
    ),
    AuthTestCase(
        id="AUTH-010",
        description="Wrong password with account lockout warning",
        category="wrong-password",
        input_data={
            "endpoint": "/api/auth/login",
            "username": "admin@example.com",
            "password": "wrongpass",
            "attempts_used": 4,
            "max_attempts": 5,
            "fixture_type": "wrong_password_lockout"
        },
        expected_status=401,
        expected_error="unauthorized",
        expected_message="Invalid username or password",
        expected_fields={"attempts_remaining": 1}
    ),
]


# =============================================================================
# FIXTURE CREATION HELPERS
# =============================================================================

def create_fixture_from_test_case(test_case: AuthTestCase):
    """Create an error response fixture from a test case definition."""
    input_data = test_case.input_data
    fixture_type = input_data.get("fixture_type", "")

    if fixture_type == "missing_api_key":
        return missing_api_key_fixture(
            path=input_data["endpoint"],
            header_name="X-API-Key"
        )
    elif fixture_type == "unauthorized_bearer":
        return unauthorized_fixture(
            path=input_data["endpoint"],
            auth_type="Bearer",
            realm="api"
        )
    elif fixture_type == "unauthorized_basic":
        return unauthorized_fixture(
            path=input_data["endpoint"],
            auth_type="Basic",
            realm="api"
        )
    elif fixture_type == "invalid_api_key":
        # Check if this is a revoked key case
        if "revoked" in input_data["headers"].get("X-API-Key", ""):
            # Create custom fixture for revoked API key
            from tests.fixtures.error_scenarios import create_error_response
            return create_error_response(
                status=403,
                error_type="forbidden",
                message="API key has been revoked",
                code=403,
                details={
                    "key_id": "key_abc123",
                    "revoked_at": "2026-07-14T12:00:00Z",
                    "reason": "user_request",
                    "path": input_data["endpoint"]
                }
            )
        else:
            return invalid_api_key_fixture(
                path=input_data["endpoint"],
                api_key=input_data["headers"].get("X-API-Key", "invalid-key")
            )
    elif fixture_type == "invalid_token":
        return invalid_token_fixture(
            path=input_data["endpoint"],
            token=input_data["headers"].get("Authorization", "").replace("Bearer ", "")
        )
    elif fixture_type == "expired_token":
        return expired_token_fixture(
            path=input_data["endpoint"],
            token_expired_at=input_data.get("token_expired_at", "2026-07-14T00:00:00Z")
        )
    elif fixture_type == "expired_api_key":
        # Create custom fixture for expired API key
        from tests.fixtures.error_scenarios import create_error_response
        return create_error_response(
            status=403,
            error_type="forbidden",
            message="API key expired",
            code=403,
            details={
                "key_expired_at": input_data.get("key_expired_at", "2026-07-14T00:00:00Z"),
                "key_id": "key_abc123",
                "path": input_data["endpoint"]
            }
        )
    elif fixture_type == "wrong_password":
        return wrong_password_fixture(
            path=input_data["endpoint"],
            username=input_data.get("username", "user@example.com"),
            attempts_remaining=test_case.expected_fields.get("attempts_remaining", 2)
        )
    elif fixture_type == "wrong_password_lockout":
        return wrong_password_fixture(
            path=input_data["endpoint"],
            username=input_data.get("username", "admin@example.com"),
            attempts_remaining=test_case.expected_fields.get("attempts_remaining", 1)
        )
    else:
        # Default to unauthorized fixture
        return unauthorized_fixture(path=input_data["endpoint"])


# =============================================================================
# TEST VALIDATION FUNCTIONS
# =============================================================================

def validate_test_case(test_case: AuthTestCase, fixture) -> TestResult:
    """
    Validate a single test case against its fixture.

    Returns a TestResult with the validation outcome.
    """
    import time
    start_time = time.time()

    try:
        # Get the response tuple
        response = fixture.to_tuple()
        actual_status = response[0]
        response_body = fixture.body

        # Extract actual error and message
        actual_error = response_body.get('error', 'unknown')
        actual_message = response_body.get('message', '')

        # Validate status code
        if actual_status != test_case.expected_status:
            return TestResult(
                test_id=test_case.id,
                description=test_case.description,
                passed=False,
                expected_status=test_case.expected_status,
                actual_status=actual_status,
                expected_error=test_case.expected_error,
                actual_error=actual_error,
                expected_message=test_case.expected_message,
                actual_message=actual_message,
                execution_time_ms=(time.time() - start_time) * 1000,
                error_details=f"Status code mismatch: expected {test_case.expected_status}, got {actual_status}"
            )

        # Validate error type
        if actual_error != test_case.expected_error:
            return TestResult(
                test_id=test_case.id,
                description=test_case.description,
                passed=False,
                expected_status=test_case.expected_status,
                actual_status=actual_status,
                expected_error=test_case.expected_error,
                actual_error=actual_error,
                expected_message=test_case.expected_message,
                actual_message=actual_message,
                execution_time_ms=(time.time() - start_time) * 1000,
                error_details=f"Error type mismatch: expected '{test_case.expected_error}', got '{actual_error}'"
            )

        # Validate message content (case-insensitive substring match)
        if test_case.expected_message and test_case.expected_message.lower() not in actual_message.lower():
            return TestResult(
                test_id=test_case.id,
                description=test_case.description,
                passed=False,
                expected_status=test_case.expected_status,
                actual_status=actual_status,
                expected_error=test_case.expected_error,
                actual_error=actual_error,
                expected_message=test_case.expected_message,
                actual_message=actual_message,
                execution_time_ms=(time.time() - start_time) * 1000,
                error_details=f"Message mismatch: expected '{test_case.expected_message}' in message, got '{actual_message}'"
            )

        # Validate expected fields if present
        if test_case.expected_fields:
            for field_name, expected_value in test_case.expected_fields.items():
                actual_value = response_body.get('details', {}).get(field_name)
                if actual_value != expected_value:
                    return TestResult(
                        test_id=test_case.id,
                        description=test_case.description,
                        passed=False,
                        expected_status=test_case.expected_status,
                        actual_status=actual_status,
                        expected_error=test_case.expected_error,
                        actual_error=actual_error,
                        expected_message=test_case.expected_message,
                        actual_message=actual_message,
                        execution_time_ms=(time.time() - start_time) * 1000,
                        error_details=f"Field mismatch: {field_name} expected {expected_value}, got {actual_value}"
                    )

        # All validations passed
        return TestResult(
            test_id=test_case.id,
            description=test_case.description,
            passed=True,
            expected_status=test_case.expected_status,
            actual_status=actual_status,
            expected_error=test_case.expected_error,
            actual_error=actual_error,
            expected_message=test_case.expected_message,
            actual_message=actual_message,
            execution_time_ms=(time.time() - start_time) * 1000,
            error_details=""
        )

    except Exception as e:
        return TestResult(
            test_id=test_case.id,
            description=test_case.description,
            passed=False,
            expected_status=test_case.expected_status,
            actual_status=-1,
            expected_error=test_case.expected_error,
            actual_error="exception",
            expected_message=test_case.expected_message,
            actual_message=str(e),
            execution_time_ms=(time.time() - start_time) * 1000,
            error_details=f"Exception during validation: {str(e)}"
        )


# =============================================================================
# TEST RUNNER
# =============================================================================

def run_auth_tests(category: str = None) -> TestSuiteResult:
    """
    Run authentication error test cases.

    Args:
        category: Optional category filter (e.g., "missing-credentials", "invalid-credentials")

    Returns:
        TestSuiteResult with execution summary
    """
    import time
    start_time = time.time()

    # Filter test cases by category if specified
    test_cases = AUTH_TEST_TABLE
    if category:
        test_cases = [tc for tc in AUTH_TEST_TABLE if tc.category == category]

    results = []
    for test_case in test_cases:
        # Create fixture from test case
        fixture = create_fixture_from_test_case(test_case)

        # Validate the test case
        result = validate_test_case(test_case, fixture)
        results.append(result)

    # Calculate summary
    total_time = (time.time() - start_time) * 1000
    passed = sum(1 for r in results if r.passed)
    failed = sum(1 for r in results if not r.passed)
    skipped = 0  # No skip logic yet

    return TestSuiteResult(
        total_count=len(results),
        passed_count=passed,
        failed_count=failed,
        skipped_count=skipped,
        results=results,
        execution_time_ms=total_time
    )


# =============================================================================
# OUTPUT FORMATTING
# =============================================================================

def print_results(suite_result: TestSuiteResult):
    """Print formatted test results."""
    # ANSI color codes
    GREEN = '\033[92m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    CYAN = '\033[96m'
    RESET = '\033[0m'

    print(f"\n{CYAN}{'=' * 80}{RESET}")
    print(f"{CYAN}AUTHENTICATION ERROR TEST RESULTS{RESET}")
    print(f"{CYAN}{'=' * 80}{RESET}\n")

    # Summary
    print(f"{BLUE}Total Tests:{RESET} {suite_result.total_count}")
    print(f"{GREEN}Passed:{RESET} {suite_result.passed_count}")
    print(f"{RED}Failed:{RESET} {suite_result.failed_count}")
    print(f"{YELLOW}Skipped:{RESET} {suite_result.skipped_count}")
    print(f"{BLUE}Execution Time:{RESET} {suite_result.execution_time_ms:.2f}ms")
    print()

    # Results by category
    categories = {}
    for result in suite_result.results:
        # Find the test case to get its category
        test_case = next(tc for tc in AUTH_TEST_TABLE if tc.id == result.test_id)
        category = test_case.category
        if category not in categories:
            categories[category] = []
        categories[category].append(result)

    for category, results in categories.items():
        print(f"\n{BLUE}{category.replace('-', ' ').title()}{RESET}")
        print(f"{BLUE}{'-' * 40}{RESET}")

        for result in results:
            if result.passed:
                print(f"  {GREEN}✓{RESET} {result.test_id}: {result.description}")
            else:
                print(f"  {RED}✗{RESET} {result.test_id}: {result.description}")
                print(f"      {RED}Error:{RESET} {result.error_details}")
                if result.expected_status != result.actual_status:
                    print(f"      Expected status: {result.expected_status}, Got: {result.actual_status}")
                if result.expected_error != result.actual_error:
                    print(f"      Expected error: '{result.expected_error}', Got: '{result.actual_error}'")
                if result.expected_message not in result.actual_message.lower():
                    print(f"      Expected message: '{result.expected_message}' in '{result.actual_message}'")

    # Final summary line
    print(f"\n{CYAN}{'=' * 80}{RESET}")
    if suite_result.failed_count == 0:
        print(f"{GREEN}ALL TESTS PASSED{RESET}")
    else:
        print(f"{RED}{suite_result.failed_count} TEST(S) FAILED{RESET}")
    print(f"{CYAN}{'=' * 80}{RESET}\n")


def print_test_table():
    """Print the authentication error test table in a formatted way."""
    # ANSI color codes
    CYAN = '\033[96m'
    YELLOW = '\033[93m'
    GREEN = '\033[92m'
    RESET = '\033[0m'

    print(f"\n{CYAN}{'=' * 100}{RESET}")
    print(f"{CYAN}AUTHENTICATION ERROR TEST TABLE{RESET}")
    print(f"{CYAN}{'=' * 100}{RESET}\n")

    for test_case in AUTH_TEST_TABLE:
        print(f"{YELLOW}{test_case.id}{RESET} - {test_case.description}")
        print(f"  Category: {test_case.category}")
        print(f"  Input: {json.dumps(test_case.input_data, indent=4)}")
        print(f"  Expected: HTTP {test_case.expected_status} | Error: {test_case.expected_error} | Message: {test_case.expected_message}")
        if test_case.expected_fields:
            print(f"  Expected Fields: {json.dumps(test_case.expected_fields, indent=4)}")
        print()


# =============================================================================
# MAIN ENTRY POINT
# =============================================================================

def main():
    """Main entry point for the test runner."""
    # ANSI color codes
    GREEN = '\033[92m'
    RED = '\033[91m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    CYAN = '\033[96m'
    RESET = '\033[0m'

    import argparse

    parser = argparse.ArgumentParser(
        description="Run authentication error test cases"
    )
    parser.add_argument(
        '--category',
        choices=['missing-credentials', 'invalid-credentials', 'expired-credentials', 'wrong-password'],
        help='Run only tests from a specific category'
    )
    parser.add_argument(
        '--list',
        action='store_true',
        help='List all test cases in the table'
    )
    parser.add_argument(
        '--verbose',
        action='store_true',
        help='Show detailed test output'
    )

    args = parser.parse_args()

    # List test cases if requested
    if args.list:
        print_test_table()
        return 0

    # Run tests
    print(f"{CYAN}Running Authentication Error Tests...{RESET}\n")
    suite_result = run_auth_tests(category=args.category)

    # Print results
    print_results(suite_result)

    # Exit with appropriate code
    return 0 if suite_result.failed_count == 0 else 1


if __name__ == '__main__':
    sys.exit(main())
