#!/usr/bin/env python3
"""
Example Usage of Base Error Test Structure

This file demonstrates how to use and extend the base error test structure
for testing specific error types. It provides concrete examples that you can
adapt for your own testing needs.

Usage:
    python3 -m tests.error_tests.example_usage

Bead: bf-2zqplr
Created: 2026-07-15
"""

from tests.error_tests import (
    TestPattern,
    ErrorTestSuite,
    ErrorCategory,
    create_base_pattern,
    create_error_suite
)


# Example 1: Creating a simple test pattern
def example_simple_pattern():
    """Create and execute a simple test pattern."""
    print("\n=== Example 1: Simple Pattern ===\n")

    # Define a simple pattern
    pattern = TestPattern(
        name="basic_404",
        description="Test basic 404 Not Found error",
        category=ErrorCategory.CLIENT_ERROR,
        setup=lambda: {"status": 404, "message": "Not Found"},
        execute=lambda data: (data["status"], data["message"]),
        validate=lambda result: (result[0] == 404, "Status should be 404")
    )

    # Execute the pattern
    result = pattern.execute_pattern()

    print(f"Pattern: {result.pattern.name}")
    print(f"Status: {result.status.value}")
    print(f"Passed: {result.passed}")
    print(f"Execution Time: {result.execution_time_ms:.2f}ms")


# Example 2: Creating a test suite
def example_test_suite():
    """Create and execute a test suite with multiple patterns."""
    print("\n=== Example 2: Test Suite ===\n")

    # Create patterns
    pattern1 = TestPattern(
        name="404_test",
        description="Test 404 error",
        category=ErrorCategory.CLIENT_ERROR,
        setup=lambda: {"status": 404},
        validate=lambda data: (data.get("status") == 404, "Should be 404")
    )

    pattern2 = TestPattern(
        name="500_test",
        description="Test 500 error",
        category=ErrorCategory.SERVER_ERROR,
        setup=lambda: {"status": 500},
        validate=lambda data: (data.get("status") == 500, "Should be 500")
    )

    # Create suite and add patterns
    suite = ErrorTestSuite(
        name="HTTP Error Tests",
        description="Test various HTTP error responses"
    )
    suite.add_pattern(pattern1)
    suite.add_pattern(pattern2)

    # Execute all patterns
    results = suite.execute_all()

    # Print summary
    results.print_summary()


# Example 3: Using helper functions
def example_helper_functions():
    """Use helper functions to create patterns and suites."""
    print("\n=== Example 3: Helper Functions ===\n")

    # Create pattern using helper
    pattern = create_base_pattern(
        name="auth_test",
        description="Test authentication error",
        category=ErrorCategory.AUTHENTICATION,
        setup=lambda: {"error": "unauthorized"},
        validate=lambda data: (data.get("error") == "unauthorized", "Should be unauthorized")
    )

    # Create suite using helper
    suite = create_error_suite(
        name="Auth Tests",
        description="Authentication error tests",
        patterns=[pattern]
    )

    # Execute
    results = suite.execute_all()
    print(f"Suite: {results.suite.name}")
    print(f"Total: {results.total_count}, Passed: {results.passed_count}")


# Example 4: Pattern with tags and metadata
def example_pattern_metadata():
    """Create patterns with tags and metadata."""
    print("\n=== Example 4: Tags and Metadata ===\n")

    # Create pattern with tags and metadata
    pattern = TestPattern(
        name="validation_error",
        description="Test validation error response",
        category=ErrorCategory.VALIDATION,
        tags=["smoke", "critical", "validation"],
        metadata={
            "priority": "high",
            "owner": "auth-team",
            "ticket": "AUTH-123"
        },
        setup=lambda: {"error": "validation_failed"},
        validate=lambda data: (True, "Always passes")
    )

    print(f"Pattern: {pattern.name}")
    print(f"Tags: {', '.join(pattern.tags)}")
    print(f"Metadata: {pattern.metadata}")

    # Add additional tags
    pattern_with_tags = pattern.with_tags("regression")
    print(f"Extended Tags: {', '.join(pattern_with_tags.tags)}")


# Example 5: Filtering by tags
def example_tag_filtering():
    """Execute patterns filtered by tags."""
    print("\n=== Example 5: Tag Filtering ===\n")

    # Create patterns with different tags
    pattern1 = TestPattern(
        name="smoke_test",
        description="Smoke test",
        category=ErrorCategory.CLIENT_ERROR,
        tags=["smoke"],
        setup=lambda: {"status": "ok"},
        validate=lambda data: (True, "Passes")
    )

    pattern2 = TestPattern(
        name="regression_test",
        description="Regression test",
        category=ErrorCategory.CLIENT_ERROR,
        tags=["regression"],
        setup=lambda: {"status": "ok"},
        validate=lambda data: (True, "Passes")
    )

    pattern3 = TestPattern(
        name="both_test",
        description="Both smoke and regression",
        category=ErrorCategory.CLIENT_ERROR,
        tags=["smoke", "regression"],
        setup=lambda: {"status": "ok"},
        validate=lambda data: (True, "Passes")
    )

    # Create suite
    suite = ErrorTestSuite(
        name="Tagged Tests",
        description="Tests with various tags"
    )
    suite.add_pattern(pattern1)
    suite.add_pattern(pattern2)
    suite.add_pattern(pattern3)

    # Execute only smoke tests
    results = suite.execute_all(filter_tags=["smoke"])
    print(f"Smoke tests only: {results.total_count} executed")

    # Execute only regression tests
    results = suite.execute_all(filter_tags=["regression"])
    print(f"Regression tests only: {results.total_count} executed")


# Example 6: Pattern that fails validation
def example_failed_validation():
    """Create a pattern that fails validation."""
    print("\n=== Example 6: Failed Validation ===\n")

    pattern = TestPattern(
        name="failing_test",
        description="Test that fails validation",
        category=ErrorCategory.VALIDATION,
        setup=lambda: {"status": 400},
        validate=lambda data: (data.get("status") == 404, "Expected 404 but got 400")
    )

    result = pattern.execute_pattern()
    print(f"Pattern: {result.pattern.name}")
    print(f"Status: {result.status.value}")
    print(f"Passed: {result.passed}")
    print(f"Error: {result.error_message}")


# Example 7: Suite with setup and teardown
def example_suite_setup_teardown():
    """Create a suite with setup and teardown callbacks."""
    print("\n=== Example 7: Suite Setup/Teardown ===\n")

    setup_called = []
    teardown_called = []

    def suite_setup():
        setup_called.append(True)
        print("Suite setup called")

    def suite_teardown():
        teardown_called.append(True)
        print("Suite teardown called")

    pattern = TestPattern(
        name="test_with_cleanup",
        description="Test with suite cleanup",
        category=ErrorCategory.CLIENT_ERROR,
        setup=lambda: {"status": "ok"},
        validate=lambda data: (True, "Passes")
    )

    suite = ErrorTestSuite(
        name="Cleanup Test Suite",
        description="Suite with setup/teardown",
        patterns=[pattern],
        setup_callback=suite_setup,
        teardown_callback=suite_teardown
    )

    results = suite.execute_all()
    print(f"Setup called: {len(setup_called) > 0}")
    print(f"Teardown called: {len(teardown_called) > 0}")


# Example 8: Disabled pattern
def example_disabled_pattern():
    """Create and execute a disabled pattern."""
    print("\n=== Example 8: Disabled Pattern ===\n")

    pattern = TestPattern(
        name="disabled_test",
        description="This test is disabled",
        category=ErrorCategory.CLIENT_ERROR,
        enabled=False,
        setup=lambda: {"status": "ok"},
        validate=lambda data: (True, "Passes")
    )

    result = pattern.execute_pattern()
    print(f"Pattern: {result.pattern.name}")
    print(f"Status: {result.status.value}")
    print(f"Skipped: {result.skipped}")


# Example 9: Getting patterns by category
def example_category_filtering():
    """Filter patterns by category."""
    print("\n=== Example 9: Category Filtering ===\n")

    # Create patterns in different categories
    patterns = [
        TestPattern(
            name="auth_1",
            description="Auth test 1",
            category=ErrorCategory.AUTHENTICATION,
            setup=lambda: {},
            validate=lambda data: (True, "Passes")
        ),
        TestPattern(
            name="auth_2",
            description="Auth test 2",
            category=ErrorCategory.AUTHENTICATION,
            setup=lambda: {},
            validate=lambda data: (True, "Passes")
        ),
        TestPattern(
            name="validation_1",
            description="Validation test",
            category=ErrorCategory.VALIDATION,
            setup=lambda: {},
            validate=lambda data: (True, "Passes")
        )
    ]

    suite = ErrorTestSuite(
        name="Multi-Category Suite",
        description="Suite with multiple categories"
    )

    for pattern in patterns:
        suite.add_pattern(pattern)

    # Get all categories
    categories = suite.get_categories()
    print(f"Categories in suite: {', '.join(categories)}")

    # Get patterns by category
    auth_patterns = suite.get_patterns_by_category(ErrorCategory.AUTHENTICATION)
    print(f"Authentication patterns: {len(auth_patterns)}")


# Example 10: Complete realistic scenario
def example_realistic_scenario():
    """Complete example with realistic error testing."""
    print("\n=== Example 10: Realistic Scenario ===\n")

    # Simulate error fixtures
    def create_error_fixture(status, error_type, message):
        return {
            "status_code": status,
            "error": error_type,
            "message": message,
            "details": {"path": "/api/test"}
        }

    # Create test patterns for different error scenarios
    patterns = [
        TestPattern(
            name="not_found_error",
            description="Test 404 Not Found error response",
            category=ErrorCategory.CLIENT_ERROR,
            tags=["smoke", "critical"],
            setup=lambda: create_error_fixture(404, "not_found", "Resource not found"),
            execute=lambda fixture: fixture,
            validate=lambda fixture: (
                fixture["status_code"] == 404 and
                fixture["error"] == "not_found",
                "Should be 404 with not_found error"
            )
        ),
        TestPattern(
            name="unauthorized_error",
            description="Test 401 Unauthorized error response",
            category=ErrorCategory.AUTHENTICATION,
            tags=["smoke", "auth"],
            setup=lambda: create_error_fixture(401, "unauthorized", "Authentication required"),
            execute=lambda fixture: fixture,
            validate=lambda fixture: (
                fixture["status_code"] == 401 and
                fixture["error"] == "unauthorized",
                "Should be 401 with unauthorized error"
            )
        ),
        TestPattern(
            name="validation_error",
            description="Test 400 Bad Request validation error",
            category=ErrorCategory.VALIDATION,
            tags=["validation"],
            setup=lambda: create_error_fixture(400, "validation_error", "Invalid input"),
            execute=lambda fixture: fixture,
            validate=lambda fixture: (
                fixture["status_code"] == 400 and
                fixture["error"] == "validation_error",
                "Should be 400 with validation_error"
            )
        )
    ]

    # Create suite
    suite = ErrorTestSuite(
        name="API Error Responses",
        description="Test common API error response structures"
    )

    for pattern in patterns:
        suite.add_pattern(pattern)

    # Execute all patterns
    results = suite.execute_all()

    # Print detailed summary
    results.print_summary()

    # Check if all passed
    if results.all_passed:
        print("✓ All error response tests passed!")
    else:
        print("✗ Some tests failed")


def main():
    """Run all examples."""
    print("="*60)
    print("Base Error Test Structure - Usage Examples")
    print("="*60)

    examples = [
        example_simple_pattern,
        example_test_suite,
        example_helper_functions,
        example_pattern_metadata,
        example_tag_filtering,
        example_failed_validation,
        example_suite_setup_teardown,
        example_disabled_pattern,
        example_category_filtering,
        example_realistic_scenario,
    ]

    for example in examples:
        try:
            example()
        except Exception as e:
            print(f"Error in {example.__name__}: {e}")

    print("\n" + "="*60)
    print("All examples completed")
    print("="*60 + "\n")


if __name__ == "__main__":
    main()
