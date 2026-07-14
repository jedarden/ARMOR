#!/usr/bin/env python3
"""
Example Test Suite Using Extensible Test Tables

This module demonstrates how to use ARMOR's extensible test table structure
to create comprehensive, maintainable test suites for different error types.

It shows real-world examples of:
- Using pre-configured test tables
- Creating custom test tables
- Running test tables with different executors
- Filtering and organizing test cases

Bead: bf-1a04kj
Created: 2026-07-14
"""

import unittest
from typing import Dict, Any
from tests.test_tables import (
    ErrorTestTable,
    TestCase,
    TestResult,
    TestExecutionResult,
    create_auth_test_table,
    create_validation_test_table,
    create_not_found_test_table,
    create_rate_limit_test_table,
    create_server_error_test_table,
    run_test_case,
    run_test_table,
    get_table,
    list_tables
)


# =============================================================================
# MOCK EXECUTORS FOR TESTING
# =============================================================================

class MockAPIExecutor:
    """
    Mock API executor for testing authentication error responses.

    Simulates an API that checks for API keys and returns appropriate errors.
    """

    def __init__(self):
        self.valid_keys = {"valid-key-123", "admin-key-456", "user-key"}
        self.expired_keys = {"expired-key-123"}

    def __call__(self, input_data: Dict[str, Any]) -> Dict[str, Any]:
        """Execute a test case against the mock API."""
        endpoint = input_data.get("endpoint", "")
        headers = input_data.get("headers", {})

        # Check for missing API key
        if "X-API-Key" not in headers:
            return {
                "status": 401,
                "error": "unauthorized",
                "message": "API key required"
            }

        api_key = headers["X-API-Key"]

        # Check for expired API key
        if api_key in self.expired_keys:
            return {
                "status": 403,
                "error": "forbidden",
                "message": "API key expired"
            }

        # Check for invalid API key
        if api_key not in self.valid_keys:
            return {
                "status": 403,
                "error": "forbidden",
                "message": "Invalid API key"
            }

        # Check admin endpoint permissions
        if "/admin" in endpoint and api_key != "admin-key-456":
            return {
                "status": 403,
                "error": "forbidden",
                "message": "Insufficient permissions"
            }

        # Valid request
        return {
            "status": 200,
            "error": None,
            "message": "OK",
            "fields": {"request_id": "req-123"}
        }


class MockValidationExecutor:
    """
    Mock API executor for testing validation error responses.

    Simulates an API that validates input data and returns validation errors.
    """

    def __call__(self, input_data: Dict[str, Any]) -> Dict[str, Any]:
        """Execute a test case against the validation API."""
        endpoint = input_data.get("endpoint", "")
        method = input_data.get("method", "GET")
        body = input_data.get("body", {})

        if method == "POST" and endpoint == "/api/users":
            # Check required fields
            if "email" not in body:
                return {
                    "status": 422,
                    "error": "validation_error",
                    "message": "required field",
                    "fields": {"missing_field": "email"}
                }

            # Check email format
            email = body.get("email", "")
            if "@" not in email or "." not in email:
                return {
                    "status": 422,
                    "error": "validation_error",
                    "message": "Invalid email format"
                }

            # Check age range
            age = body.get("age")
            if age is not None and (age < 0 or age > 120):
                return {
                    "status": 422,
                    "error": "validation_error",
                    "message": "age must be between 0 and 120"
                }

            # Check name length
            name = body.get("name", "")
            if len(name) > 100:
                return {
                    "status": 422,
                    "error": "validation_error",
                    "message": "name too long"
                }

            # Valid request
            return {
                "status": 201,
                "error": None,
                "message": "User created"
            }

        return {"status": 200, "error": None, "message": "OK"}


class MockNotFoundExecutor:
    """
    Mock API executor for testing 404 Not Found responses.

    Simulates an API that returns 404 for non-existent resources.
    """

    def __init__(self):
        self.existing_resources = {
            "/api/users/1": True,
            "/api/users/2": True,
            "/api/posts/1": False,  # Deleted
        }

    def __call__(self, input_data: Dict[str, Any]) -> Dict[str, Any]:
        """Execute a test case against the not found API."""
        endpoint = input_data.get("endpoint", "")

        # Check if resource exists
        if endpoint in self.existing_resources:
            exists = self.existing_resources[endpoint]
            if not exists:
                return {
                    "status": 404,
                    "error": "not_found",
                    "message": "Resource has been deleted"
                }
            return {
                "status": 200,
                "error": None,
                "message": "OK"
            }

        # Resource not found
        if endpoint.startswith("/api/users/"):
            return {
                "status": 404,
                "error": "not_found",
                "message": "User not found"
            }

        if endpoint.startswith("/api/"):
            return {
                "status": 404,
                "error": "not_found",
                "message": "Resource not found"
            }

        return {"status": 200, "error": None, "message": "OK"}


# =============================================================================
# UNIT TESTS FOR TEST TABLE STRUCTURE
# =============================================================================

class TestTestTableStructure(unittest.TestCase):
    """Unit tests for the test table data structures."""

    def test_test_case_creation(self):
        """Test that test cases can be created with all fields."""
        test_case = TestCase(
            id="TEST-001",
            description="Test case creation",
            input_data={"endpoint": "/api/test"},
            expected_status=200,
            expected_error=None,
            expected_message="OK",
            tags=["unit"],
            enabled=True
        )

        self.assertEqual(test_case.id, "TEST-001")
        self.assertEqual(test_case.description, "Test case creation")
        self.assertTrue(test_case.enabled)
        self.assertIn("unit", test_case.tags)
        self.assertTrue(test_case.is_http_test)

    def test_test_case_validation(self):
        """Test that test cases validate required fields."""
        with self.assertRaises(ValueError):
            TestCase(id="", description="Test", input_data={})

        with self.assertRaises(ValueError):
            TestCase(id="TEST", description="", input_data={})

        with self.assertRaises(ValueError):
            TestCase(id="TEST", description="Test", input_data=None)

    def test_test_case_with_tags(self):
        """Test adding tags to test cases."""
        test_case = TestCase(
            id="TEST-001",
            description="Test",
            input_data={},
            tags=["tag1"]
        )

        new_case = test_case.with_tags("tag2", "tag3")

        self.assertIn("tag1", new_case.tags)
        self.assertIn("tag2", new_case.tags)
        self.assertIn("tag3", new_case.tags)

    def test_test_case_with_metadata(self):
        """Test adding metadata to test cases."""
        test_case = TestCase(
            id="TEST-001",
            description="Test",
            input_data={},
            metadata={"jira": "TEST-123"}
        )

        new_case = test_case.with_metadata(priority="high", component="api")

        self.assertEqual(new_case.metadata["jira"], "TEST-123")
        self.assertEqual(new_case.metadata["priority"], "high")
        self.assertEqual(new_case.metadata["component"], "api")

    def test_error_test_table_creation(self):
        """Test that test tables can be created."""
        table = ErrorTestTable(
            name="test_table",
            description="Test table for unit tests",
            test_cases=[
                TestCase(
                    id="TEST-001",
                    description="Test case 1",
                    input_data={}
                )
            ]
        )

        self.assertEqual(table.name, "test_table")
        self.assertEqual(len(table.test_cases), 1)
        self.assertEqual(table.test_cases[0].id, "TEST-001")

    def test_error_test_table_unique_ids(self):
        """Test that test tables enforce unique IDs."""
        with self.assertRaises(ValueError):
            ErrorTestTable(
                name="test_table",
                description="Test table",
                test_cases=[
                    TestCase(id="DUP", description="Test 1", input_data={}),
                    TestCase(id="DUP", description="Test 2", input_data={})
                ]
            )

    def test_add_test_case(self):
        """Test adding test cases to a table."""
        table = ErrorTestTable(
            name="test_table",
            description="Test table",
            test_cases=[]
        )

        test_case = TestCase(
            id="TEST-001",
            description="New test",
            input_data={}
        )

        table.add_test_case(test_case)

        self.assertEqual(len(table.test_cases), 1)
        self.assertEqual(table.test_cases[0].id, "TEST-001")

    def test_add_duplicate_test_case_id(self):
        """Test that duplicate test case IDs are rejected."""
        table = ErrorTestTable(
            name="test_table",
            description="Test table",
            test_cases=[
                TestCase(id="TEST-001", description="Test 1", input_data={})
            ]
        )

        with self.assertRaises(ValueError):
            table.add_test_case(
                TestCase(id="TEST-001", description="Test 2", input_data={})
            )

    def test_get_test_case_by_id(self):
        """Test retrieving test cases by ID."""
        table = ErrorTestTable(
            name="test_table",
            description="Test table",
            test_cases=[
                TestCase(id="TEST-001", description="Test 1", input_data={}),
                TestCase(id="TEST-002", description="Test 2", input_data={})
            ]
        )

        test_case = table.get_test_case("TEST-001")
        self.assertIsNotNone(test_case)
        self.assertEqual(test_case.id, "TEST-001")

        test_case = table.get_test_case("TEST-999")
        self.assertIsNone(test_case)

    def test_filter_by_tags(self):
        """Test filtering test cases by tags."""
        table = ErrorTestTable(
            name="test_table",
            description="Test table",
            test_cases=[
                TestCase(id="TEST-001", description="Test 1", input_data={}, tags=["smoke", "fast"]),
                TestCase(id="TEST-002", description="Test 2", input_data={}, tags=["regression"]),
                TestCase(id="TEST-003", description="Test 3", input_data={}, tags=["smoke", "slow"])
            ]
        )

        smoke_tests = table.get_test_cases_by_tag("smoke")
        self.assertEqual(len(smoke_tests), 2)

        regression_tests = table.get_test_cases_by_tag("regression")
        self.assertEqual(len(regression_tests), 1)

    def test_get_enabled_test_cases(self):
        """Test getting only enabled test cases."""
        table = ErrorTestTable(
            name="test_table",
            description="Test table",
            test_cases=[
                TestCase(id="TEST-001", description="Test 1", input_data={}, enabled=True),
                TestCase(id="TEST-002", description="Test 2", input_data={}, enabled=False),
                TestCase(id="TEST-003", description="Test 3", input_data={}, enabled=True)
            ]
        )

        enabled = table.get_enabled_test_cases()
        self.assertEqual(len(enabled), 2)

    def test_table_filter_by_tags(self):
        """Test creating filtered table views."""
        table = ErrorTestTable(
            name="test_table",
            description="Test table",
            test_cases=[
                TestCase(id="TEST-001", description="Test 1", input_data={}, tags=["smoke", "auth"]),
                TestCase(id="TEST-002", description="Test 2", input_data={}, tags=["regression"]),
                TestCase(id="TEST-003", description="Test 3", input_data={}, tags=["smoke", "validation"])
            ]
        )

        filtered = table.filter_by_tags("smoke")
        self.assertEqual(len(filtered.test_cases), 2)

        filtered = table.filter_by_tags("smoke", "auth")
        self.assertEqual(len(filtered.test_cases), 1)

    def test_merge_tables(self):
        """Test merging two test tables."""
        table1 = ErrorTestTable(
            name="table1",
            description="Table 1",
            test_cases=[
                TestCase(id="TEST-001", description="Test 1", input_data={})
            ]
        )

        table2 = ErrorTestTable(
            name="table2",
            description="Table 2",
            test_cases=[
                TestCase(id="TEST-002", description="Test 2", input_data={})
            ]
        )

        table1.merge(table2)

        self.assertEqual(len(table1.test_cases), 2)
        self.assertIn("TEST-001", {tc.id for tc in table1.test_cases})
        self.assertIn("TEST-002", {tc.id for tc in table1.test_cases})

    def test_merge_tables_with_duplicates(self):
        """Test that merging tables with duplicate IDs fails."""
        table1 = ErrorTestTable(
            name="table1",
            description="Table 1",
            test_cases=[
                TestCase(id="TEST-001", description="Test 1", input_data={})
            ]
        )

        table2 = ErrorTestTable(
            name="table2",
            description="Table 2",
            test_cases=[
                TestCase(id="TEST-001", description="Test 2", input_data={})
            ]
        )

        with self.assertRaises(ValueError):
            table1.merge(table2)


# =============================================================================
# INTEGRATION TESTS WITH MOCK EXECUTORS
# =============================================================================

class TestAuthenticationTestTable(unittest.TestCase):
    """Integration tests for authentication error test table."""

    def setUp(self):
        """Set up test fixtures."""
        self.table = create_auth_test_table()
        self.executor = MockAPIExecutor()

    def test_table_structure(self):
        """Test that authentication table is properly structured."""
        self.assertEqual(self.table.name, "authentication_errors")
        self.assertTrue(len(self.table.test_cases) > 0)

        # Check for unique IDs
        ids = [tc.id for tc in self.table.test_cases]
        self.assertEqual(len(ids), len(set(ids)))

    def test_run_all_authentication_tests(self):
        """Test running all authentication tests."""
        results = run_test_table(self.table, self.executor)

        self.assertEqual(results.total_count, len(self.table.test_cases))
        self.assertGreater(results.passed_count, 0)
        self.assertIsNotNone(results.execution_time_ms)

    def test_authentication_missing_api_key(self):
        """Test authentication failure with missing API key."""
        test_case = self.table.get_test_case("AUTH-001")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 401)
        self.assertEqual(result.actual_error, "unauthorized")

    def test_authentication_invalid_api_key(self):
        """Test authentication failure with invalid API key."""
        test_case = self.table.get_test_case("AUTH-002")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 403)
        self.assertEqual(result.actual_error, "forbidden")

    def test_authentication_expired_api_key(self):
        """Test authentication failure with expired API key."""
        test_case = self.table.get_test_case("AUTH-003")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 403)
        self.assertIn("expired", result.actual_message.lower())

    def test_authentication_insufficient_permissions(self):
        """Test authentication failure with insufficient permissions."""
        test_case = self.table.get_test_case("AUTH-004")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 403)
        self.assertIn("permission", result.actual_message.lower())

    def test_authentication_valid_key(self):
        """Test successful authentication with valid API key."""
        test_case = self.table.get_test_case("AUTH-005")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 200)
        self.assertIsNone(result.actual_error)

    def test_filter_by_smoke_tag(self):
        """Test filtering tests by smoke tag."""
        smoke_tests = self.table.get_test_cases_by_tag("smoke")
        self.assertGreater(len(smoke_tests), 0)

        # Create filtered table
        smoke_table = ErrorTestTable(
            name="auth_smoke",
            description="Smoke tests for authentication",
            test_cases=smoke_tests
        )

        results = run_test_table(smoke_table, self.executor)
        self.assertTrue(results.all_passed)


class TestValidationTestTable(unittest.TestCase):
    """Integration tests for validation error test table."""

    def setUp(self):
        """Set up test fixtures."""
        self.table = create_validation_test_table()
        self.executor = MockValidationExecutor()

    def test_table_structure(self):
        """Test that validation table is properly structured."""
        self.assertEqual(self.table.name, "validation_errors")
        self.assertTrue(len(self.table.test_cases) > 0)

    def test_run_all_validation_tests(self):
        """Test running all validation tests."""
        results = run_test_table(self.table, self.executor)

        self.assertEqual(results.total_count, len(self.table.test_cases))
        self.assertGreater(results.passed_count, 0)

    def test_validation_missing_required_field(self):
        """Test validation failure for missing required field."""
        test_case = self.table.get_test_case("VAL-001")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 422)
        self.assertEqual(result.actual_error, "validation_error")

    def test_validation_invalid_email_format(self):
        """Test validation failure for invalid email format."""
        test_case = self.table.get_test_case("VAL-002")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 422)
        self.assertIn("invalid", result.actual_message.lower())


class TestNotFoundTestTable(unittest.TestCase):
    """Integration tests for not found error test table."""

    def setUp(self):
        """Set up test fixtures."""
        self.table = create_not_found_test_table()
        self.executor = MockNotFoundExecutor()

    def test_table_structure(self):
        """Test that not found table is properly structured."""
        self.assertEqual(self.table.name, "not_found_errors")
        self.assertTrue(len(self.table.test_cases) > 0)

    def test_run_all_not_found_tests(self):
        """Test running all not found tests."""
        results = run_test_table(self.table, self.executor)

        self.assertEqual(results.total_count, len(self.table.test_cases))
        self.assertGreater(results.passed_count, 0)

    def test_not_found_non_existent_user(self):
        """Test 404 for non-existent user ID."""
        test_case = self.table.get_test_case("NF-001")
        self.assertIsNotNone(test_case)

        result = run_test_case(test_case, self.executor)

        self.assertEqual(result.result, TestResult.PASSED)
        self.assertEqual(result.actual_status, 404)
        self.assertEqual(result.actual_error, "not_found")


# =============================================================================
# CUSTOM TEST TABLE EXAMPLES
# =============================================================================

def test_custom_payment_error_table():
    """
    Example: Creating a custom test table for payment processing errors.

    This demonstrates how to create a new test table for a specific domain
    (payment processing) following the established patterns.
    """
    payment_table = ErrorTestTable(
        name="payment_errors",
        description="Test cases for payment processing errors",
        tags=["payment", "critical", "external"],
        test_cases=[
            TestCase(
                id="PAY-001",
                description="Invalid credit card number",
                input_data={"card_number": "1234", "amount": 100},
                expected_status=422,
                expected_error="validation_error",
                expected_message="Invalid card number"
            ),
            TestCase(
                id="PAY-002",
                description="Expired credit card",
                input_data={"card_number": "4242424242424242", "expiry": "12/20"},
                expected_status=422,
                expected_error="validation_error",
                expected_message="card expired"
            ),
            TestCase(
                id="PAY-003",
                description="Payment gateway timeout",
                input_data={"card_number": "4242424242424242", "amount": 100, "simulate_timeout": True},
                expected_status=504,
                expected_error="gateway_timeout",
                expected_message="Payment gateway timeout"
            ),
            TestCase(
                id="PAY-004",
                description="Insufficient funds",
                input_data={"card_number": "4242424242424242", "amount": 999999},
                expected_status=422,
                expected_error="payment_failed",
                expected_message="Insufficient funds"
            ),
            TestCase(
                id="PAY-005",
                description="Successful payment",
                input_data={"card_number": "4242424242424242", "amount": 100},
                expected_status=200,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )

    # Mock executor for payment processing
    def payment_executor(input_data):
        card_number = input_data.get("card_number", "")
        amount = input_data.get("amount", 0)

        if input_data.get("simulate_timeout"):
            return {"status": 504, "error": "gateway_timeout", "message": "Payment gateway timeout"}

        if len(card_number) < 16:
            return {"status": 422, "error": "validation_error", "message": "Invalid card number"}

        if amount > 10000:
            return {"status": 422, "error": "payment_failed", "message": "Insufficient funds"}

        return {"status": 200, "error": None, "message": "Payment processed"}

    # Run the test table
    results = run_test_table(payment_table, payment_executor)

    # Verify results
    assert results.total_count == 5
    assert results.passed_count > 0
    print(f"Payment Error Tests: {results.passed_count}/{results.total_count} passed")


# =============================================================================
# PRE-CONFIGURED TABLE COLLECTION TESTS
# =============================================================================

def test_list_available_tables():
    """Test that all pre-configured tables are available."""
    tables = list_tables()

    expected_tables = ["authentication", "validation", "not_found", "rate_limit", "server_error"]
    for table_name in expected_tables:
        assert table_name in tables, f"Table '{table_name}' not found in available tables"


def test_get_table_by_name():
    """Test retrieving pre-configured tables by name."""
    auth_table = get_table("authentication")
    assert auth_table is not None
    assert auth_table.name == "authentication_errors"

    validation_table = get_table("validation")
    assert validation_table is not None
    assert validation_table.name == "validation_errors"

    non_existent = get_table("non_existent")
    assert non_existent is None


# =============================================================================
# ADDITIONAL UNIT TESTS FOR TABLE COLLECTIONS
# =============================================================================

class TestTableCollections(unittest.TestCase):
    """Unit tests for pre-configured table collections."""

    def test_list_available_tables(self):
        """Test that all pre-configured tables are available."""
        tables = list_tables()

        expected_tables = ["authentication", "validation", "not_found", "rate_limit", "server_error"]
        for table_name in expected_tables:
            self.assertIn(table_name, tables, f"Table '{table_name}' not found in available tables")

    def test_get_table_by_name(self):
        """Test retrieving pre-configured tables by name."""
        auth_table = get_table("authentication")
        self.assertIsNotNone(auth_table)
        self.assertEqual(auth_table.name, "authentication_errors")

        validation_table = get_table("validation")
        self.assertIsNotNone(validation_table)
        self.assertEqual(validation_table.name, "validation_errors")

        non_existent = get_table("non_existent")
        self.assertIsNone(non_existent)


    def test_all_preconfigured_tables_have_tests(self):
        """Test that all pre-configured tables have test cases."""
        tables = list_tables()
        for table_name in tables:
            table = get_table(table_name)
            self.assertIsNotNone(table)
            self.assertGreater(len(table.test_cases), 0, f"Table '{table_name}' has no test cases")
            self.assertGreater(len(table.get_enabled_test_cases()), 0, f"Table '{table_name}' has no enabled test cases")


if __name__ == "__main__":
    # Run tests when executed directly
    unittest.main()
