#!/usr/bin/env python3
"""
Extensible Test Table Structure for ARMOR Tests

This module provides a reusable, table-driven testing framework for ARMOR test cases.
Test tables allow you to define test cases in a structured, declarative way that can
be easily extended for different error types and scenarios.

Structure:
- TestCase: Individual test case with inputs, expected outputs, and metadata
- ErrorTestTable: Collection of test cases for a specific error type or scenario
- Test execution functions: Run tests from tables with proper validation

Benefits:
- DRY: Define test patterns once, reuse across multiple test cases
- Extensible: Easy to add new test cases by adding rows to the table
- Maintainable: Clear separation between test data and test logic
- Documented: Test tables serve as living documentation of expected behavior

Usage Example:
    >>> # Define a test table for authentication errors
    >>> auth_table = ErrorTestTable(
    ...     name="Authentication Errors",
    ...     description="Test cases for authentication-related error responses",
    ...     test_cases=[
    ...         TestCase(
    ...             id="AUTH-001",
    ...             description="Missing API key",
    ...             input_data={"endpoint": "/api/users", "headers": {}},
    ...             expected_status=401,
    ...             expected_error="unauthorized",
    ...             expected_message="API key required"
    ...         ),
    ...         TestCase(
    ...             id="AUTH-002",
    ...             description="Invalid API key",
    ...             input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "invalid"}},
    ...             expected_status=403,
    ...             expected_error="forbidden",
    ...             expected_message="Invalid API key"
    ...         )
    ...     ]
    ... )
    >>>
    >>> # Run all tests in the table
    >>> results = run_test_table(auth_table)
    >>> print(f"Passed: {results.passed_count}, Failed: {results.failed_count}")

Bead: bf-1a04kj
Created: 2026-07-14
"""

from dataclasses import dataclass, field
from typing import Dict, Any, Optional, List, Callable, Tuple, Union
from enum import Enum
import json


class TestResult(Enum):
    """Result of a single test case execution."""
    PASSED = "passed"
    FAILED = "failed"
    SKIPPED = "skipped"
    ERROR = "error"


@dataclass
class TestCase:
    """
    A single test case definition.

    Represents one row in a test table with all the information needed
    to execute the test and validate the result.

    Attributes:
        id: Unique identifier for this test case (e.g., "AUTH-001", "VAL-001")
        description: Human-readable description of what this test validates
        input_data: Input parameters for the test (endpoint, headers, body, etc.)
        expected_status: Expected HTTP status code (None if not testing HTTP)
        expected_error: Expected error type/identifier (e.g., "not_found", "unauthorized")
        expected_message: Expected error message (or substring of message)
        expected_fields: Optional dict of additional fields to validate in response
        tags: Optional list of tags for categorization (e.g., ["smoke", "auth", "critical"])
        enabled: Whether this test case is enabled (disabled tests are skipped)
        setup_callback: Optional callback function to run before test execution
        teardown_callback: Optional callback function to run after test execution
        custom_validator: Optional custom validation function for this test case
        metadata: Optional dict for additional test metadata
    """

    id: str
    description: str
    input_data: Dict[str, Any]
    expected_status: Optional[int] = None
    expected_error: Optional[str] = None
    expected_message: Optional[str] = None
    expected_fields: Optional[Dict[str, Any]] = None
    tags: List[str] = field(default_factory=list)
    enabled: bool = True
    setup_callback: Optional[Callable] = None
    teardown_callback: Optional[Callable] = None
    custom_validator: Optional[Callable[[Any], Tuple[bool, str]]] = None
    metadata: Dict[str, Any] = field(default_factory=dict)

    def __post_init__(self):
        """Validate test case after initialization."""
        if not self.id:
            raise ValueError("Test case must have an id")
        if not self.description:
            raise ValueError("Test case must have a description")
        if self.input_data is None:
            raise ValueError("Test case must have input_data")

    @property
    def is_http_test(self) -> bool:
        """Check if this is an HTTP status test."""
        return self.expected_status is not None

    @property
    def is_error_test(self) -> bool:
        """Check if this tests error responses."""
        return self.expected_error is not None

    def with_tags(self, *tags: str) -> 'TestCase':
        """Return a new test case with additional tags."""
        new_tags = list(set(self.tags + list(tags)))
        return TestCase(
            id=self.id,
            description=self.description,
            input_data=self.input_data,
            expected_status=self.expected_status,
            expected_error=self.expected_error,
            expected_message=self.expected_message,
            expected_fields=self.expected_fields,
            tags=new_tags,
            enabled=self.enabled,
            setup_callback=self.setup_callback,
            teardown_callback=self.teardown_callback,
            custom_validator=self.custom_validator,
            metadata=self.metadata.copy()
        )

    def with_metadata(self, **metadata) -> 'TestCase':
        """Return a new test case with additional metadata."""
        new_metadata = {**self.metadata, **metadata}
        return TestCase(
            id=self.id,
            description=self.description,
            input_data=self.input_data,
            expected_status=self.expected_status,
            expected_error=self.expected_error,
            expected_message=self.expected_message,
            expected_fields=self.expected_fields,
            tags=self.tags.copy(),
            enabled=self.enabled,
            setup_callback=self.setup_callback,
            teardown_callback=self.teardown_callback,
            custom_validator=self.custom_validator,
            metadata=new_metadata
        )


@dataclass
class TestExecutionResult:
    """
    Result of executing a single test case.

    Contains the outcome, actual vs expected values, and any error messages.
    """
    test_case: TestCase
    result: TestResult
    actual_status: Optional[int] = None
    actual_error: Optional[str] = None
    actual_message: Optional[str] = None
    actual_fields: Optional[Dict[str, Any]] = None
    error_message: Optional[str] = None
    execution_time_ms: Optional[float] = None

    @property
    def passed(self) -> bool:
        """Check if the test passed."""
        return self.result == TestResult.PASSED

    @property
    def failed(self) -> bool:
        """Check if the test failed."""
        return self.result == TestResult.FAILED

    @property
    def skipped(self) -> bool:
        """Check if the test was skipped."""
        return self.result == TestResult.SKIPPED

    def to_dict(self) -> Dict[str, Any]:
        """Convert result to dictionary for serialization."""
        return {
            'test_id': self.test_case.id,
            'description': self.test_case.description,
            'result': self.result.value,
            'expected_status': self.test_case.expected_status,
            'actual_status': self.actual_status,
            'expected_error': self.test_case.expected_error,
            'actual_error': self.actual_error,
            'expected_message': self.test_case.expected_message,
            'actual_message': self.actual_message,
            'error_message': self.error_message,
            'execution_time_ms': self.execution_time_ms
        }


@dataclass
class TestTableResult:
    """
    Aggregated results from running a test table.

    Contains summary statistics and individual test case results.
    """
    table_name: str
    total_count: int
    passed_count: int
    failed_count: int
    skipped_count: int
    error_count: int
    results: List[TestExecutionResult] = field(default_factory=list)
    execution_time_ms: Optional[float] = None

    @property
    def all_passed(self) -> bool:
        """Check if all tests passed."""
        return self.failed_count == 0 and self.error_count == 0

    @property
    def pass_rate(self) -> float:
        """Calculate pass rate as a percentage."""
        if self.total_count == 0:
            return 0.0
        return (self.passed_count / self.total_count) * 100

    def to_dict(self) -> Dict[str, Any]:
        """Convert results to dictionary for serialization."""
        return {
            'table_name': self.table_name,
            'total': self.total_count,
            'passed': self.passed_count,
            'failed': self.failed_count,
            'skipped': self.skipped_count,
            'error': self.error_count,
            'pass_rate': self.pass_rate,
            'execution_time_ms': self.execution_time_ms,
            'results': [r.to_dict() for r in self.results]
        }


@dataclass
class ErrorTestTable:
    """
    A collection of test cases for a specific error type or scenario.

    Test tables organize related test cases together and provide metadata
    about what category of errors they test (authentication, validation, etc.).

    Attributes:
        name: Unique name for this test table (e.g., "auth_errors", "validation_errors")
        description: Human-readable description of what this table tests
        test_cases: List of test case definitions
        tags: Optional list of tags for categorization
        setup_callback: Optional callback to run before the entire table
        teardown_callback: Optional callback to run after the entire table
        metadata: Optional dict for additional table metadata
    """

    name: str
    description: str
    test_cases: List[TestCase] = field(default_factory=list)
    tags: List[str] = field(default_factory=list)
    setup_callback: Optional[Callable] = None
    teardown_callback: Optional[Callable] = None
    metadata: Dict[str, Any] = field(default_factory=dict)

    def __post_init__(self):
        """Validate test table after initialization."""
        if not self.name:
            raise ValueError("Test table must have a name")
        if not self.description:
            raise ValueError("Test table must have a description")

        # Ensure test case IDs are unique within the table
        ids = [tc.id for tc in self.test_cases]
        if len(ids) != len(set(ids)):
            duplicates = [id for id in ids if ids.count(id) > 1]
            raise ValueError(f"Duplicate test case IDs found: {set(duplicates)}")

        # Initialize default test_cases if None
        if self.test_cases is None:
            self.test_cases = []

    def add_test_case(self, test_case: TestCase) -> 'ErrorTestTable':
        """Add a test case to this table."""
        # Check for duplicate ID
        existing_ids = {tc.id for tc in self.test_cases}
        if test_case.id in existing_ids:
            raise ValueError(f"Test case ID '{test_case.id}' already exists in table")

        self.test_cases.append(test_case)
        return self

    def get_test_case(self, test_id: str) -> Optional[TestCase]:
        """Get a test case by ID."""
        for tc in self.test_cases:
            if tc.id == test_id:
                return tc
        return None

    def get_test_cases_by_tag(self, tag: str) -> List[TestCase]:
        """Get all test cases with a specific tag."""
        return [tc for tc in self.test_cases if tag in tc.tags]

    def get_enabled_test_cases(self) -> List[TestCase]:
        """Get only enabled test cases."""
        return [tc for tc in self.test_cases if tc.enabled]

    def filter_by_tags(self, *tags: str) -> 'ErrorTestTable':
        """Create a new table with only test cases having all specified tags."""
        filtered_cases = [
            tc for tc in self.test_cases
            if all(tag in tc.tags for tag in tags)
        ]
        return ErrorTestTable(
            name=f"{self.name}_filtered",
            description=f"Filtered view of {self.name}",
            test_cases=filtered_cases,
            tags=self.tags.copy(),
            metadata=self.metadata.copy()
        )

    def merge(self, other: 'ErrorTestTable') -> 'ErrorTestTable':
        """Merge another test table into this one."""
        # Check for duplicate IDs
        self_ids = {tc.id for tc in self.test_cases}
        other_ids = {tc.id for tc in other.test_cases}
        overlaps = self_ids & other_ids
        if overlaps:
            raise ValueError(f"Cannot merge tables with overlapping test case IDs: {overlaps}")

        self.test_cases.extend(other.test_cases)
        return self


# =============================================================================
# TEST EXECUTION FUNCTIONS
# =============================================================================

def run_test_case(
    test_case: TestCase,
    executor: Callable[[Dict[str, Any]], Dict[str, Any]]
) -> TestExecutionResult:
    """
    Execute a single test case.

    Args:
        test_case: The test case to execute
        executor: Function that takes input_data and returns actual results
                 Should return dict with optional keys: status, error, message, fields

    Returns:
        TestExecutionResult: The execution result with pass/fail status
    """
    import time

    # Check if test is enabled
    if not test_case.enabled:
        return TestExecutionResult(
            test_case=test_case,
            result=TestResult.SKIPPED
        )

    start_time = time.time()

    try:
        # Run setup callback if provided
        if test_case.setup_callback:
            test_case.setup_callback(test_case.input_data)

        # Execute the test
        actual = executor(test_case.input_data)

        # Extract actual values
        actual_status = actual.get('status')
        actual_error = actual.get('error')
        actual_message = actual.get('message')
        actual_fields = actual.get('fields')

        # Validate expected status
        if test_case.expected_status is not None:
            if actual_status != test_case.expected_status:
                return TestExecutionResult(
                    test_case=test_case,
                    result=TestResult.FAILED,
                    actual_status=actual_status,
                    actual_error=actual_error,
                    actual_message=actual_message,
                    actual_fields=actual_fields,
                    error_message=f"Expected status {test_case.expected_status}, got {actual_status}"
                )

        # Validate expected error
        if test_case.expected_error is not None:
            if actual_error != test_case.expected_error:
                return TestExecutionResult(
                    test_case=test_case,
                    result=TestResult.FAILED,
                    actual_status=actual_status,
                    actual_error=actual_error,
                    actual_message=actual_message,
                    actual_fields=actual_fields,
                    error_message=f"Expected error '{test_case.expected_error}', got '{actual_error}'"
                )

        # Validate expected message
        if test_case.expected_message is not None:
            if actual_message is None or test_case.expected_message not in actual_message:
                return TestExecutionResult(
                    test_case=test_case,
                    result=TestResult.FAILED,
                    actual_status=actual_status,
                    actual_error=actual_error,
                    actual_message=actual_message,
                    actual_fields=actual_fields,
                    error_message=f"Expected message containing '{test_case.expected_message}', got '{actual_message}'"
                )

        # Validate expected fields
        if test_case.expected_fields is not None:
            if actual_fields is None:
                return TestExecutionResult(
                    test_case=test_case,
                    result=TestResult.FAILED,
                    actual_status=actual_status,
                    actual_error=actual_error,
                    actual_message=actual_message,
                    actual_fields=actual_fields,
                    error_message=f"Expected fields {test_case.expected_fields}, but no fields returned"
                )
            for key, expected_value in test_case.expected_fields.items():
                if key not in actual_fields:
                    return TestExecutionResult(
                        test_case=test_case,
                        result=TestResult.FAILED,
                        actual_status=actual_status,
                        actual_error=actual_error,
                        actual_message=actual_message,
                        actual_fields=actual_fields,
                        error_message=f"Missing expected field: {key}"
                    )
                if actual_fields[key] != expected_value:
                    return TestExecutionResult(
                        test_case=test_case,
                        result=TestResult.FAILED,
                        actual_status=actual_status,
                        actual_error=actual_error,
                        actual_message=actual_message,
                        actual_fields=actual_fields,
                        error_message=f"Field '{key}' expected {expected_value}, got {actual_fields[key]}"
                    )

        # Run custom validator if provided
        if test_case.custom_validator:
            passed, error_msg = test_case.custom_validator(actual)
            if not passed:
                return TestExecutionResult(
                    test_case=test_case,
                    result=TestResult.FAILED,
                    actual_status=actual_status,
                    actual_error=actual_error,
                    actual_message=actual_message,
                    actual_fields=actual_fields,
                    error_message=error_msg or "Custom validation failed"
                )

        # Test passed
        execution_time = (time.time() - start_time) * 1000
        return TestExecutionResult(
            test_case=test_case,
            result=TestResult.PASSED,
            actual_status=actual_status,
            actual_error=actual_error,
            actual_message=actual_message,
            actual_fields=actual_fields,
            execution_time_ms=execution_time
        )

    except Exception as e:
        # Test error
        execution_time = (time.time() - start_time) * 1000
        return TestExecutionResult(
            test_case=test_case,
            result=TestResult.ERROR,
            error_message=f"Test execution error: {str(e)}",
            execution_time_ms=execution_time
        )

    finally:
        # Run teardown callback if provided
        if test_case.teardown_callback:
            try:
                test_case.teardown_callback(test_case.input_data)
            except Exception as e:
                # Don't fail the test on teardown error, just log it
                pass


def run_test_table(
    table: ErrorTestTable,
    executor: Callable[[Dict[str, Any]], Dict[str, Any]],
    continue_on_failure: bool = True
) -> TestTableResult:
    """
    Run all test cases in a test table.

    Args:
        table: The test table to run
        executor: Function that executes test cases and returns results
        continue_on_failure: If True, continues running tests after failures

    Returns:
        TestTableResult: Aggregated results from all test cases
    """
    import time

    start_time = time.time()
    results = []

    # Run table setup callback
    if table.setup_callback:
        try:
            table.setup_callback()
        except Exception as e:
            # If table setup fails, return error result
            return TestTableResult(
                table_name=table.name,
                total_count=len(table.test_cases),
                passed_count=0,
                failed_count=0,
                skipped_count=len(table.test_cases),
                error_count=len(table.test_cases),
                execution_time_ms=(time.time() - start_time) * 1000
            )

    # Run each test case
    for test_case in table.test_cases:
        result = run_test_case(test_case, executor)
        results.append(result)

        # Stop on first failure if configured
        if not continue_on_failure and result.result in (TestResult.FAILED, TestResult.ERROR):
            # Mark remaining tests as skipped
            for remaining_case in table.test_cases[len(results):]:
                results.append(TestExecutionResult(
                    test_case=remaining_case,
                    result=TestResult.SKIPPED
                ))
            break

    # Run table teardown callback
    if table.teardown_callback:
        try:
            table.teardown_callback()
        except Exception as e:
            pass  # Don't fail the table run on teardown error

    # Calculate summary statistics
    passed_count = sum(1 for r in results if r.result == TestResult.PASSED)
    failed_count = sum(1 for r in results if r.result == TestResult.FAILED)
    skipped_count = sum(1 for r in results if r.result == TestResult.SKIPPED)
    error_count = sum(1 for r in results if r.result == TestResult.ERROR)

    return TestTableResult(
        table_name=table.name,
        total_count=len(results),
        passed_count=passed_count,
        failed_count=failed_count,
        skipped_count=skipped_count,
        error_count=error_count,
        results=results,
        execution_time_ms=(time.time() - start_time) * 1000
    )


# =============================================================================
# CONVENIENCE FUNCTIONS FOR CREATING TEST TABLES
# =============================================================================

def create_simple_test_table(
    name: str,
    description: str,
    test_data: List[Tuple[str, str, Dict[str, Any], int, str, str]]
) -> ErrorTestTable:
    """
    Create a test table from a simple list format.

    Args:
        name: Table name
        description: Table description
        test_data: List of tuples in format:
                   (id, description, input_data, expected_status, expected_error, expected_message)

    Returns:
        ErrorTestTable: Configured test table
    """
    test_cases = []
    for i, (test_id, desc, inputs, status, error, message) in enumerate(test_data, 1):
        test_cases.append(TestCase(
            id=test_id,
            description=desc,
            input_data=inputs,
            expected_status=status,
            expected_error=error,
            expected_message=message
        ))

    return ErrorTestTable(
        name=name,
        description=description,
        test_cases=test_cases
    )


def create_auth_test_table() -> ErrorTestTable:
    """
    Create a test table for authentication error scenarios.

    Returns:
        ErrorTestTable: Pre-configured authentication error test table
    """
    return ErrorTestTable(
        name="authentication_errors",
        description="Test cases for authentication and authorization error responses",
        tags=["auth", "security", "critical"],
        test_cases=[
            TestCase(
                id="AUTH-001",
                description="Missing API key in request",
                input_data={"endpoint": "/api/users", "headers": {}},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="API key required",
                tags=["smoke", "regression"]
            ),
            TestCase(
                id="AUTH-002",
                description="Invalid API key format",
                input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "invalid-format"}},
                expected_status=403,
                expected_error="forbidden",
                expected_message="Invalid API key",
                tags=["regression"]
            ),
            TestCase(
                id="AUTH-003",
                description="Expired API key",
                input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "expired-key-123"}},
                expected_status=403,
                expected_error="forbidden",
                expected_message="API key expired",
                tags=["regression"]
            ),
            TestCase(
                id="AUTH-004",
                description="Insufficient permissions for admin endpoint",
                input_data={"endpoint": "/api/admin/users", "headers": {"X-API-Key": "user-key"}},
                expected_status=403,
                expected_error="forbidden",
                expected_message="Insufficient permissions",
                tags=["admin", "rbac"]
            ),
            TestCase(
                id="AUTH-005",
                description="Valid API key accepted",
                input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "valid-key-123"}},
                expected_status=200,
                expected_error=None,
                tags=["smoke", "happy-path"]
            )
        ]
    )


def create_validation_test_table() -> ErrorTestTable:
    """
    Create a test table for validation error scenarios.

    Returns:
        ErrorTestTable: Pre-configured validation error test table
    """
    return ErrorTestTable(
        name="validation_errors",
        description="Test cases for input validation error responses",
        tags=["validation", "input", "4xx"],
        test_cases=[
            TestCase(
                id="VAL-001",
                description="Missing required field in POST body",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "John"}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="required field",
                expected_fields={"missing_field": "email"},
                tags=["smoke"]
            ),
            TestCase(
                id="VAL-002",
                description="Invalid email format",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"email": "not-an-email"}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="Invalid email format",
                tags=["regression"]
            ),
            TestCase(
                id="VAL-003",
                description="Numeric field out of range",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"age": 150}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="age must be between",
                tags=["regression"]
            ),
            TestCase(
                id="VAL-004",
                description="String field exceeds max length",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "x" * 300}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="name too long",
                tags=["regression"]
            ),
            TestCase(
                id="VAL-005",
                description="Valid request accepted",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "Jane", "email": "jane@example.com", "age": 25}},
                expected_status=201,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )


def create_not_found_test_table() -> ErrorTestTable:
    """
    Create a test table for 404 Not Found error scenarios.

    Returns:
        ErrorTestTable: Pre-configured not found error test table
    """
    return ErrorTestTable(
        name="not_found_errors",
        description="Test cases for 404 Not Found error responses",
        tags=["404", "not_found", "client_error"],
        test_cases=[
            TestCase(
                id="NF-001",
                description="Non-existent user ID",
                input_data={"endpoint": "/api/users/999999"},
                expected_status=404,
                expected_error="not_found",
                expected_message="User not found",
                tags=["smoke"]
            ),
            TestCase(
                id="NF-002",
                description="Non-existent resource path",
                input_data={"endpoint": "/api/nonexistent"},
                expected_status=404,
                expected_error="not_found",
                expected_message="Resource not found",
                tags=["regression"]
            ),
            TestCase(
                id="NF-003",
                description="Deleted resource",
                input_data={"endpoint": "/api/posts/1", "context": "previously_deleted"},
                expected_status=404,
                expected_error="not_found",
                expected_message="Resource has been deleted",
                tags=["edge-case"]
            ),
            TestCase(
                id="NF-004",
                description="Valid resource exists",
                input_data={"endpoint": "/api/users/1"},
                expected_status=200,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )


def create_rate_limit_test_table() -> ErrorTestTable:
    """
    Create a test table for rate limiting error scenarios.

    Returns:
        ErrorTestTable: Pre-configured rate limit error test table
    """
    return ErrorTestTable(
        name="rate_limit_errors",
        description="Test cases for rate limiting (429) error responses",
        tags=["rate_limit", "429", "throttling"],
        test_cases=[
            TestCase(
                id="RL-001",
                description="Request rate exceeds limit",
                input_data={"endpoint": "/api/search", "request_count": 101, "window": "1m"},
                expected_status=429,
                expected_error="rate_limited",
                expected_message="Too many requests",
                expected_fields={"retry_after": 60},
                tags=["smoke"]
            ),
            TestCase(
                id="RL-002",
                description="Rate limit response includes Retry-After header",
                input_data={"endpoint": "/api/data", "request_count": 1001},
                expected_status=429,
                expected_error="rate_limited",
                expected_message="rate limit",
                expected_fields={"headers": {"Retry-After": "30"}},
                tags=["header-validation"]
            ),
            TestCase(
                id="RL-003",
                description="Within rate limit accepted",
                input_data={"endpoint": "/api/data", "request_count": 50, "window": "1m"},
                expected_status=200,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )


def create_server_error_test_table() -> ErrorTestTable:
    """
    Create a test table for server error (5xx) scenarios.

    Returns:
        ErrorTestTable: Pre-configured server error test table
    """
    return ErrorTestTable(
        name="server_errors",
        description="Test cases for server error (5xx) responses",
        tags=["5xx", "server_error", "critical"],
        test_cases=[
            TestCase(
                id="SE-001",
                description="Internal server error",
                input_data={"endpoint": "/api/crash", "simulate": "internal_error"},
                expected_status=500,
                expected_error="internal_server_error",
                expected_message="Internal server error",
                tags=["smoke"]
            ),
            TestCase(
                id="SE-002",
                description="Database connection timeout",
                input_data={"endpoint": "/api/users", "simulate": "db_timeout"},
                expected_status=503,
                expected_error="service_unavailable",
                expected_message="Database timeout",
                tags=["database", "critical"]
            ),
            TestCase(
                id="SE-003",
                description="External service unavailable",
                input_data={"endpoint": "/api/external", "simulate": "upstream_down"},
                expected_status=502,
                expected_error="bad_gateway",
                expected_message="Upstream service unavailable",
                tags=["external", "integration"]
            ),
            TestCase(
                id="SE-004",
                description="Gateway timeout",
                input_data={"endpoint": "/api/slow", "simulate": "timeout"},
                expected_status=504,
                expected_error="gateway_timeout",
                expected_message="Gateway timeout",
                tags=["timeout"]
            )
        ]
    )


# =============================================================================
# PRE-CONFIGURED TABLE COLLECTIONS
# =============================================================================

COMMON_ERROR_TABLES = {
    'authentication': create_auth_test_table(),
    'validation': create_validation_test_table(),
    'not_found': create_not_found_test_table(),
    'rate_limit': create_rate_limit_test_table(),
    'server_error': create_server_error_test_table()
}

ALL_ERROR_TABLES = COMMON_ERROR_TABLES.copy()


def get_table(name: str) -> Optional[ErrorTestTable]:
    """
    Get a pre-configured test table by name.

    Args:
        name: Table name (e.g., 'authentication', 'validation', 'not_found')

    Returns:
        ErrorTestTable or None: The table if found, None otherwise
    """
    return ALL_ERROR_TABLES.get(name)


def list_tables() -> List[str]:
    """
    List all available pre-configured test table names.

    Returns:
        List[str]: Sorted list of table names
    """
    return sorted(ALL_ERROR_TABLES.keys())
