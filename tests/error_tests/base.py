#!/usr/bin/env python3
"""
Base Error Test Module for ARMOR

This module provides the foundational test structure and table definitions
for error response testing. It defines extensible patterns for testing
specific error types and serves as the base for all error test modules.

Architecture:
    TestPattern: Base pattern definition for test cases
    ErrorTestSuite: Container for organizing multiple test tables
    PatternExecutor: Executes test patterns with validation

Usage:
    from tests.error_tests.base import TestPattern, ErrorTestSuite

    # Define a test pattern
    pattern = TestPattern(
        name="not_found",
        description="Test 404 Not Found errors",
        setup=lambda: not_found_fixture(path="/api/test"),
        execute=lambda fixture: validate_http_status(fixture.to_tuple(), 404),
        validate=lambda result: result.status_code == 404
    )

    # Create a test suite
    suite = ErrorTestSuite(name="Error Tests")
    suite.add_pattern(pattern)

Extension:
    To extend for specific error types:
    1. Create a new module in tests/error_tests/ (e.g., auth_tests.py)
    2. Import TestPattern and ErrorTestSuite
    3. Define patterns specific to your error type
    4. Add patterns to a suite

Bead: bf-2zqplr
Created: 2026-07-15
"""

from dataclasses import dataclass, field
from typing import Dict, Any, Optional, List, Callable, Tuple, Union
from enum import Enum
import time


class PatternStatus(Enum):
    """Status of a test pattern execution."""
    PENDING = "pending"
    RUNNING = "running"
    PASSED = "passed"
    FAILED = "failed"
    SKIPPED = "skipped"
    ERROR = "error"


@dataclass
class TestPattern:
    """
    Base pattern definition for error test cases.

    A TestPattern defines the structure and execution logic for a single
    test scenario. It provides a template for running tests with consistent
    setup, execution, and validation phases.

    Attributes:
        name: Unique identifier for this pattern (e.g., "not_found", "unauthorized")
        description: Human-readable description of what this pattern tests
        category: Category of error (e.g., "client_error", "server_error", "auth")
        setup: Optional setup function that prepares test fixtures or data
        execute: Function that executes the test and returns a result
        validate: Function that validates the execution result
        cleanup: Optional cleanup function to run after execution
        enabled: Whether this pattern is enabled (disabled patterns are skipped)
        timeout_ms: Optional timeout for pattern execution in milliseconds
        tags: Optional list of tags for categorization
        metadata: Optional dict for additional pattern metadata

    Example:
        >>> pattern = TestPattern(
        ...     name="not_found",
        ...     description="Test 404 Not Found errors",
        ...     category="client_error",
        ...     setup=lambda: not_found_fixture(path="/api/test"),
        ...     execute=lambda fixture: validate_http_status(fixture.to_tuple(), 404),
        ...     validate=lambda result: result.status_code == 404
        ... )
    """

    name: str
    description: str
    category: str
    setup: Optional[Callable] = None
    execute: Optional[Callable] = None
    validate: Optional[Callable[[Any], Tuple[bool, str]]] = None
    cleanup: Optional[Callable] = None
    enabled: bool = True
    timeout_ms: Optional[int] = None
    tags: List[str] = field(default_factory=list)
    metadata: Dict[str, Any] = field(default_factory=dict)

    def __post_init__(self):
        """Validate test pattern after initialization."""
        if not self.name:
            raise ValueError("Test pattern must have a name")
        if not self.description:
            raise ValueError("Test pattern must have a description")
        if not self.category:
            raise ValueError("Test pattern must have a category")

    def with_tags(self, *tags: str) -> 'TestPattern':
        """Return a new pattern with additional tags."""
        new_tags = list(set(self.tags + list(tags)))
        return TestPattern(
            name=self.name,
            description=self.description,
            category=self.category,
            setup=self.setup,
            execute=self.execute,
            validate=self.validate,
            cleanup=self.cleanup,
            enabled=self.enabled,
            timeout_ms=self.timeout_ms,
            tags=new_tags,
            metadata=self.metadata.copy()
        )

    def with_metadata(self, **metadata) -> 'TestPattern':
        """Return a new pattern with additional metadata."""
        new_metadata = {**self.metadata, **metadata}
        return TestPattern(
            name=self.name,
            description=self.description,
            category=self.category,
            setup=self.setup,
            execute=self.execute,
            validate=self.validate,
            cleanup=self.cleanup,
            enabled=self.enabled,
            timeout_ms=self.timeout_ms,
            tags=self.tags.copy(),
            metadata=new_metadata
        )

    def execute_pattern(self) -> 'PatternResult':
        """Execute this test pattern and return the result."""
        start_time = time.time()
        status = PatternStatus.RUNNING
        error_message = None
        result_data = None

        try:
            # Skip if disabled
            if not self.enabled:
                return PatternResult(
                    pattern=self,
                    status=PatternStatus.SKIPPED,
                    execution_time_ms=0,
                    error_message="Pattern is disabled"
                )

            # Setup phase
            setup_result = None
            if self.setup:
                setup_result = self.setup()

            # Execution phase
            if self.execute:
                result_data = self.execute(setup_result) if setup_result is not None else self.execute()

            # Validation phase
            if self.validate and result_data is not None:
                is_valid, validation_msg = self.validate(result_data)
                if not is_valid:
                    status = PatternStatus.FAILED
                    error_message = f"Validation failed: {validation_msg}"
                else:
                    status = PatternStatus.PASSED
            else:
                # No validation, consider passed if no exception was raised
                status = PatternStatus.PASSED

            # Cleanup phase
            if self.cleanup:
                self.cleanup()

        except Exception as e:
            status = PatternStatus.ERROR
            error_message = str(e)

        finally:
            execution_time = (time.time() - start_time) * 1000  # Convert to ms

        return PatternResult(
            pattern=self,
            status=status,
            result_data=result_data,
            error_message=error_message,
            execution_time_ms=execution_time
        )


@dataclass
class PatternResult:
    """
    Result of executing a test pattern.

    Contains the outcome, execution time, and any error messages from
    running a test pattern.

    Attributes:
        pattern: The test pattern that was executed
        status: The status of the execution (passed, failed, error, skipped)
        result_data: Optional data returned from the execution phase
        error_message: Optional error message if execution failed
        execution_time_ms: Time taken to execute the pattern in milliseconds
    """

    pattern: TestPattern
    status: PatternStatus
    result_data: Optional[Any] = None
    error_message: Optional[str] = None
    execution_time_ms: Optional[float] = None

    @property
    def passed(self) -> bool:
        """Check if the pattern passed."""
        return self.status == PatternStatus.PASSED

    @property
    def failed(self) -> bool:
        """Check if the pattern failed."""
        return self.status == PatternStatus.FAILED

    @property
    def skipped(self) -> bool:
        """Check if the pattern was skipped."""
        return self.status == PatternStatus.SKIPPED

    @property
    def error(self) -> bool:
        """Check if the pattern had an error."""
        return self.status == PatternStatus.ERROR

    def to_dict(self) -> Dict[str, Any]:
        """Convert result to dictionary for serialization."""
        return {
            'pattern_name': self.pattern.name,
            'category': self.pattern.category,
            'status': self.status.value,
            'passed': self.passed,
            'failed': self.failed,
            'error': self.error,
            'error_message': self.error_message,
            'execution_time_ms': self.execution_time_ms
        }


@dataclass
class ErrorTestSuite:
    """
    Container for organizing multiple test patterns.

    An ErrorTestSuite groups related test patterns together and provides
    methods for executing them as a group. Suites can be organized by
    error type, category, or any other logical grouping.

    Attributes:
        name: Unique name for this test suite
        description: Human-readable description of what this suite tests
        patterns: List of test patterns in this suite
        tags: Optional list of tags for categorization
        setup_callback: Optional callback to run before the entire suite
        teardown_callback: Optional callback to run after the entire suite
        metadata: Optional dict for additional suite metadata

    Example:
        >>> suite = ErrorTestSuite(
        ...     name="Client Error Tests",
        ...     description="Tests for 4xx client errors"
        ... )
        >>> suite.add_pattern(not_found_pattern)
        >>> suite.add_pattern(unauthorized_pattern)
        >>> results = suite.execute_all()
    """

    name: str
    description: str
    patterns: List[TestPattern] = field(default_factory=list)
    tags: List[str] = field(default_factory=list)
    setup_callback: Optional[Callable] = None
    teardown_callback: Optional[Callable] = None
    metadata: Dict[str, Any] = field(default_factory=dict)

    def __post_init__(self):
        """Validate test suite after initialization."""
        if not self.name:
            raise ValueError("Test suite must have a name")
        if not self.description:
            raise ValueError("Test suite must have a description")

        # Ensure pattern names are unique within the suite
        names = [p.name for p in self.patterns]
        if len(names) != len(set(names)):
            duplicates = [name for name in names if names.count(name) > 1]
            raise ValueError(f"Duplicate pattern names found: {set(duplicates)}")

    def add_pattern(self, pattern: TestPattern) -> 'ErrorTestSuite':
        """
        Add a test pattern to this suite.

        Args:
            pattern: The test pattern to add

        Returns:
            Self for method chaining

        Raises:
            ValueError: If a pattern with the same name already exists
        """
        # Check for duplicate name
        if any(p.name == pattern.name for p in self.patterns):
            raise ValueError(f"Pattern '{pattern.name}' already exists in suite")

        self.patterns.append(pattern)
        return self

    def remove_pattern(self, pattern_name: str) -> 'ErrorTestSuite':
        """
        Remove a test pattern from this suite.

        Args:
            pattern_name: Name of the pattern to remove

        Returns:
            Self for method chaining
        """
        self.patterns = [p for p in self.patterns if p.name != pattern_name]
        return self

    def get_pattern(self, pattern_name: str) -> Optional[TestPattern]:
        """
        Get a test pattern by name.

        Args:
            pattern_name: Name of the pattern to get

        Returns:
            The pattern if found, None otherwise
        """
        for pattern in self.patterns:
            if pattern.name == pattern_name:
                return pattern
        return None

    def execute_all(self, filter_tags: Optional[List[str]] = None) -> 'SuiteResult':
        """
        Execute all enabled patterns in this suite.

        Args:
            filter_tags: Optional list of tags to filter patterns by

        Returns:
            SuiteResult containing aggregated results from all pattern executions
        """
        start_time = time.time()
        results = []

        # Run suite setup if provided
        if self.setup_callback:
            try:
                self.setup_callback()
            except Exception as e:
                return SuiteResult(
                    suite=self,
                    total_count=0,
                    passed_count=0,
                    failed_count=0,
                    skipped_count=0,
                    error_count=0,
                    results=[],
                    execution_time_ms=0,
                    setup_error=str(e)
                )

        # Filter patterns by tags if specified
        patterns_to_run = self.patterns
        if filter_tags:
            patterns_to_run = [
                p for p in self.patterns
                if any(tag in p.tags for tag in filter_tags)
            ]

        # Execute each pattern
        for pattern in patterns_to_run:
            result = pattern.execute_pattern()
            results.append(result)

        # Run suite teardown if provided
        if self.teardown_callback:
            try:
                self.teardown_callback()
            except Exception as e:
                # Log error but don't fail the entire suite
                print(f"Warning: Suite teardown failed: {e}")

        execution_time = (time.time() - start_time) * 1000  # Convert to ms

        # Aggregate results
        return SuiteResult(
            suite=self,
            total_count=len(results),
            passed_count=sum(1 for r in results if r.passed),
            failed_count=sum(1 for r in results if r.failed),
            skipped_count=sum(1 for r in results if r.skipped),
            error_count=sum(1 for r in results if r.error),
            results=results,
            execution_time_ms=execution_time
        )

    def get_patterns_by_category(self, category: str) -> List[TestPattern]:
        """
        Get all patterns in a specific category.

        Args:
            category: Category to filter by

        Returns:
            List of patterns in the specified category
        """
        return [p for p in self.patterns if p.category == category]

    def get_categories(self) -> List[str]:
        """
        Get all unique categories in this suite.

        Returns:
            List of unique category names
        """
        return list(set(p.category for p in self.patterns))


@dataclass
class SuiteResult:
    """
    Aggregated results from executing a test suite.

    Contains summary statistics and individual pattern results.

    Attributes:
        suite: The test suite that was executed
        total_count: Total number of patterns executed
        passed_count: Number of patterns that passed
        failed_count: Number of patterns that failed
        skipped_count: Number of patterns that were skipped
        error_count: Number of patterns that had errors
        results: List of individual pattern results
        execution_time_ms: Total time taken to execute the suite
        setup_error: Optional error message from suite setup failure
    """

    suite: ErrorTestSuite
    total_count: int
    passed_count: int
    failed_count: int
    skipped_count: int
    error_count: int
    results: List[PatternResult] = field(default_factory=list)
    execution_time_ms: Optional[float] = None
    setup_error: Optional[str] = None

    @property
    def all_passed(self) -> bool:
        """Check if all patterns passed."""
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
            'suite_name': self.suite.name,
            'description': self.suite.description,
            'total': self.total_count,
            'passed': self.passed_count,
            'failed': self.failed_count,
            'skipped': self.skipped_count,
            'error': self.error_count,
            'pass_rate': self.pass_rate,
            'execution_time_ms': self.execution_time_ms,
            'setup_error': self.setup_error,
            'results': [r.to_dict() for r in self.results]
        }

    def print_summary(self):
        """Print a human-readable summary of the suite results."""
        print(f"\n{'='*60}")
        print(f"Test Suite: {self.suite.name}")
        print(f"{'='*60}")
        print(f"Description: {self.suite.description}")
        print(f"Total: {self.total_count}")
        print(f"Passed: {self.passed_count}")
        print(f"Failed: {self.failed_count}")
        print(f"Skipped: {self.skipped_count}")
        print(f"Errors: {self.error_count}")
        print(f"Pass Rate: {self.pass_rate:.1f}%")
        print(f"Execution Time: {self.execution_time_ms:.2f}ms")

        if self.setup_error:
            print(f"\nSetup Error: {self.setup_error}")

        # Print failed/error results
        failed_results = [r for r in self.results if r.failed or r.error]
        if failed_results:
            print(f"\nFailed/Error Patterns:")
            for result in failed_results:
                print(f"  - {result.pattern.name}: {result.error_message}")

        print(f"{'='*60}\n")


# Predefined categories for common error types
class ErrorCategory:
    """Predefined error categories for use in test patterns."""

    CLIENT_ERROR = "client_error"
    SERVER_ERROR = "server_error"
    AUTHENTICATION = "authentication"
    AUTHORIZATION = "authorization"
    VALIDATION = "validation"
    RATE_LIMIT = "rate_limit"
    NETWORK = "network"
    DATABASE = "database"
    FILESYSTEM = "filesystem"
    BUSINESS_LOGIC = "business_logic"
    PROTOCOL = "protocol"


def create_base_pattern(
    name: str,
    description: str,
    category: str,
    setup: Optional[Callable] = None,
    execute: Optional[Callable] = None,
    validate: Optional[Callable[[Any], Tuple[bool, str]]] = None,
    **kwargs
) -> TestPattern:
    """
    Helper function to create a base test pattern.

    This is a convenience function for creating test patterns with
    common parameters.

    Args:
        name: Unique identifier for the pattern
        description: Human-readable description
        category: Category of error
        setup: Optional setup function
        execute: Optional execution function
        validate: Optional validation function
        **kwargs: Additional pattern parameters

    Returns:
        A new TestPattern instance
    """
    return TestPattern(
        name=name,
        description=description,
        category=category,
        setup=setup,
        execute=execute,
        validate=validate,
        **kwargs
    )


def create_error_suite(
    name: str,
    description: str,
    patterns: Optional[List[TestPattern]] = None,
    **kwargs
) -> ErrorTestSuite:
    """
    Helper function to create an error test suite.

    This is a convenience function for creating test suites with
    common parameters.

    Args:
        name: Unique name for the suite
        description: Human-readable description
        patterns: Optional list of patterns to add
        **kwargs: Additional suite parameters

    Returns:
        A new ErrorTestSuite instance
    """
    suite = ErrorTestSuite(
        name=name,
        description=description,
        **kwargs
    )

    if patterns:
        for pattern in patterns:
            suite.add_pattern(pattern)

    return suite
