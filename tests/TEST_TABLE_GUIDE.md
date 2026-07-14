# Extensible Test Table Structure Guide

This guide explains how to use and extend ARMOR's table-driven testing framework for different error types.

## Overview

Test tables provide a structured, declarative way to define test cases. Instead of writing individual test functions, you define test cases as data in a table format. This makes tests:

- **DRY**: Define patterns once, reuse across test cases
- **Maintainable**: Clear separation between test data and logic
- **Extensible**: Easy to add new test cases by adding table rows
- **Documented**: Test tables serve as living documentation

## Core Concepts

### TestCase

A `TestCase` represents a single test with inputs, expected outputs, and metadata:

```python
from tests.test_tables import TestCase

test_case = TestCase(
    id="AUTH-001",                          # Unique identifier
    description="Missing API key",          # What this tests
    input_data={"endpoint": "/api/users"}, # Test inputs
    expected_status=401,                   # Expected HTTP status
    expected_error="unauthorized",          # Expected error type
    expected_message="API key required",    # Expected message (or substring)
    tags=["smoke", "auth"],                 # Categorization tags
    enabled=True                            # Whether to run this test
)
```

### ErrorTestTable

An `ErrorTestTable` groups related test cases together:

```python
from tests.test_tables import ErrorTestTable

table = ErrorTestTable(
    name="authentication_errors",
    description="Test cases for authentication errors",
    tags=["auth", "security"],
    test_cases=[test_case1, test_case2, ...]
)
```

## Quick Start Examples

### Example 1: Using Pre-configured Tables

ARMOR includes pre-configured test tables for common error types:

```python
from tests.test_tables import create_auth_test_table, run_test_table

# Get a pre-configured authentication error table
auth_table = create_auth_test_table()

# Define an executor function that simulates API requests
def mock_executor(input_data):
    # Simulate API behavior
    if "X-API-Key" not in input_data.get("headers", {}):
        return {
            "status": 401,
            "error": "unauthorized",
            "message": "API key required"
        }
    return {"status": 200, "error": None, "message": "OK"}

# Run all tests in the table
results = run_test_table(auth_table, mock_executor)

print(f"Passed: {results.passed_count}")
print(f"Failed: {results.failed_count}")
print(f"Pass Rate: {results.pass_rate:.1f}%")
```

### Example 2: Creating a Simple Custom Table

```python
from tests.test_tables import ErrorTestTable, TestCase

# Define test cases for a new error type
custom_table = ErrorTestTable(
    name="payment_errors",
    description="Test cases for payment processing errors",
    tags=["payment", "critical"],
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
            description="Payment gateway timeout",
            input_data={"card_number": "4242424242424242", "amount": 100, "simulate_timeout": True},
            expected_status=504,
            expected_error="gateway_timeout",
            expected_message="Payment gateway timeout"
        ),
        TestCase(
            id="PAY-003",
            description="Successful payment",
            input_data={"card_number": "4242424242424242", "amount": 100},
            expected_status=200,
            expected_error=None
        )
    ]
)
```

## Creating New Test Tables for Different Error Types

### Pattern 1: Authentication/Authorization Errors

Test API authentication and authorization scenarios:

```python
from tests.test_tables import ErrorTestTable, TestCase

auth_table = ErrorTestTable(
    name="authentication_errors",
    description="Test cases for authentication and authorization",
    tags=["auth", "security"],
    test_cases=[
        # Missing credentials
        TestCase(
            id="AUTH-001",
            description="Request without API key",
            input_data={"endpoint": "/api/users", "headers": {}},
            expected_status=401,
            expected_error="unauthorized",
            expected_message="API key required"
        ),

        # Invalid credentials
        TestCase(
            id="AUTH-002",
            description="Invalid API key",
            input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "invalid"}},
            expected_status=403,
            expected_error="forbidden",
            expected_message="Invalid API key"
        ),

        # Expired credentials
        TestCase(
            id="AUTH-003",
            description="Expired API key",
            input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "expired-123"}},
            expected_status=403,
            expected_error="forbidden",
            expected_message="API key expired"
        ),

        # Insufficient permissions
        TestCase(
            id="AUTH-004",
            description="Regular user accessing admin endpoint",
            input_data={"endpoint": "/api/admin", "headers": {"X-API-Key": "user-key"}},
            expected_status=403,
            expected_error="forbidden",
            expected_message="Insufficient permissions"
        ),

        # Valid access
        TestCase(
            id="AUTH-005",
            description="Valid API key accepted",
            input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "valid-key"}},
            expected_status=200,
            expected_error=None,
            tags=["happy-path"]
        )
    ]
)
```

### Pattern 2: Validation Errors

Test input validation scenarios:

```python
validation_table = ErrorTestTable(
    name="validation_errors",
    description="Test cases for input validation",
    tags=["validation", "4xx"],
    test_cases=[
        # Missing required fields
        TestCase(
            id="VAL-001",
            description="Missing required email field",
            input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "John"}},
            expected_status=422,
            expected_error="validation_error",
            expected_message="required field",
            expected_fields={"missing_field": "email"}
        ),

        # Invalid format
        TestCase(
            id="VAL-002",
            description="Invalid email format",
            input_data={"endpoint": "/api/users", "method": "POST", "body": {"email": "not-an-email"}},
            expected_status=422,
            expected_error="validation_error",
            expected_message="Invalid email format"
        ),

        # Out of range
        TestCase(
            id="VAL-003",
            description="Age value exceeds maximum",
            input_data={"endpoint": "/api/users", "method": "POST", "body": {"age": 150}},
            expected_status=422,
            expected_error="validation_error",
            expected_message="age must be between"
        ),

        # String too long
        TestCase(
            id="VAL-004",
            description="Name exceeds max length",
            input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "x" * 300}},
            expected_status=422,
            expected_error="validation_error",
            expected_message="name too long"
        ),

        # Valid input
        TestCase(
            id="VAL-005",
            description="Valid user creation",
            input_data={
                "endpoint": "/api/users",
                "method": "POST",
                "body": {"name": "Jane", "email": "jane@example.com", "age": 25}
            },
            expected_status=201,
            expected_error=None,
            tags=["happy-path"]
        )
    ]
)
```

### Pattern 3: Resource Not Found Errors

Test 404 scenarios for missing resources:

```python
not_found_table = ErrorTestTable(
    name="not_found_errors",
    description="Test cases for 404 Not Found errors",
    tags=["404", "not_found"],
    test_cases=[
        # Non-existent resource ID
        TestCase(
            id="NF-001",
            description="Non-existent user ID",
            input_data={"endpoint": "/api/users/999999"},
            expected_status=404,
            expected_error="not_found",
            expected_message="User not found"
        ),

        # Invalid path
        TestCase(
            id="NF-002",
            description="Non-existent endpoint path",
            input_data={"endpoint": "/api/nonexistent"},
            expected_status=404,
            expected_error="not_found",
            expected_message="Resource not found"
        ),

        # Deleted resource
        TestCase(
            id="NF-003",
            description="Accessing deleted resource",
            input_data={"endpoint": "/api/posts/1", "context": "deleted"},
            expected_status=404,
            expected_error="not_found",
            expected_message="Resource has been deleted"
        ),

        # Valid resource
        TestCase(
            id="NF-004",
            description="Existing resource is accessible",
            input_data={"endpoint": "/api/users/1"},
            expected_status=200,
            expected_error=None,
            tags=["happy-path"]
        )
    ]
)
```

### Pattern 4: Rate Limiting Errors

Test API rate limiting behavior:

```python
rate_limit_table = ErrorTestTable(
    name="rate_limit_errors",
    description="Test cases for rate limiting (429) errors",
    tags=["rate_limit", "throttling"],
    test_cases=[
        # Rate limit exceeded
        TestCase(
            id="RL-001",
            description="Request rate exceeds limit",
            input_data={"endpoint": "/api/search", "request_count": 101, "window": "1m"},
            expected_status=429,
            expected_error="rate_limited",
            expected_message="Too many requests",
            expected_fields={"retry_after": 60}
        ),

        # Retry-After header present
        TestCase(
            id="RL-002",
            description="Rate limit includes Retry-After header",
            input_data={"endpoint": "/api/data", "request_count": 1001},
            expected_status=429,
            expected_error="rate_limited",
            expected_message="rate limit",
            expected_fields={"headers": {"Retry-After": "30"}}
        ),

        # Within limit
        TestCase(
            id="RL-003",
            description="Requests within rate limit accepted",
            input_data={"endpoint": "/api/data", "request_count": 50, "window": "1m"},
            expected_status=200,
            expected_error=None,
            tags=["happy-path"]
        )
    ]
)
```

### Pattern 5: Server Errors (5xx)

Test server-side error scenarios:

```python
server_error_table = ErrorTestTable(
    name="server_errors",
    description="Test cases for server error (5xx) responses",
    tags=["5xx", "server_error", "critical"],
    test_cases=[
        # Internal server error
        TestCase(
            id="SE-001",
            description="Internal server error",
            input_data={"endpoint": "/api/crash", "simulate": "internal_error"},
            expected_status=500,
            expected_error="internal_server_error",
            expected_message="Internal server error"
        ),

        # Database unavailable
        TestCase(
            id="SE-002",
            description="Database connection timeout",
            input_data={"endpoint": "/api/users", "simulate": "db_timeout"},
            expected_status=503,
            expected_error="service_unavailable",
            expected_message="Database timeout",
            tags=["database", "critical"]
        ),

        # External service down
        TestCase(
            id="SE-003",
            description="External service unavailable",
            input_data={"endpoint": "/api/external", "simulate": "upstream_down"},
            expected_status=502,
            expected_error="bad_gateway",
            expected_message="Upstream service unavailable",
            tags=["external"]
        ),

        # Gateway timeout
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
```

## Advanced Features

### Custom Validators

Add custom validation logic to a test case:

```python
def custom_validator(response):
    """Custom validator that checks response structure."""
    if "request_id" not in response.get("fields", {}):
        return False, "Response missing request_id field"
    return True, None

test_case = TestCase(
    id="CUSTOM-001",
    description="Test with custom validation",
    input_data={"endpoint": "/api/custom"},
    expected_status=200,
    custom_validator=custom_validator
)
```

### Setup and Teardown Callbacks

Define setup/teardown logic for test cases:

```python
def setup_test(input_data):
    """Setup function run before test execution."""
    print(f"Setting up test for endpoint: {input_data.get('endpoint')}")

def teardown_test(input_data):
    """Teardown function run after test execution."""
    print(f"Cleaning up after test for endpoint: {input_data.get('endpoint')}")

test_case = TestCase(
    id="SETUP-001",
    description="Test with setup and teardown",
    input_data={"endpoint": "/api/resource"},
    expected_status=200,
    setup_callback=setup_test,
    teardown_callback=teardown_test
)
```

### Filtering by Tags

Run specific subsets of tests based on tags:

```python
from tests.test_tables import create_auth_test_table

table = create_auth_test_table()

# Get only smoke tests
smoke_tests = table.get_test_cases_by_tag("smoke")

# Get only regression tests
regression_tests = table.get_test_cases_by_tag("regression")

# Get only admin tests
admin_tests = table.get_test_cases_by_tag("admin")

# Create filtered table with multiple tags
filtered_table = table.filter_by_tags("smoke", "auth")
```

### Disabling Test Cases

Temporarily disable test cases without removing them:

```python
test_case = TestCase(
    id="TEMP-001",
    description="Feature not yet implemented",
    input_data={"endpoint": "/api/new-feature"},
    expected_status=200,
    enabled=False  # This test will be skipped
)
```

## Running Tests

### Basic Execution

```python
from tests.test_tables import run_test_table

# Define executor function
def executor(input_data):
    # Your test execution logic here
    # Returns dict with: status, error, message, fields
    return {
        "status": 200,
        "error": None,
        "message": "OK",
        "fields": {"request_id": "abc123"}
    }

# Run tests
results = run_test_table(table, executor)

# Check results
if results.all_passed:
    print("All tests passed!")
else:
    print(f"Failed: {results.failed_count} tests")
    for result in results.results:
        if result.failed:
            print(f"  {result.test_case.id}: {result.error_message}")
```

### Stop on First Failure

```python
# Stop execution on first failure
results = run_test_table(table, executor, continue_on_failure=False)
```

### Table-level Setup/Teardown

```python
def table_setup():
    """Setup for entire test table."""
    print("Setting up test environment...")

def table_teardown():
    """Teardown for entire test table."""
    print("Cleaning up test environment...")

table = ErrorTestTable(
    name="my_table",
    description="Test table with setup/teardown",
    test_cases=[...],
    setup_callback=table_setup,
    teardown_callback=table_teardown
)
```

## Best Practices

### 1. Naming Conventions

- **Test IDs**: Use descriptive prefixes (e.g., "AUTH-001", "VAL-001")
- **Table names**: Use lowercase with underscores (e.g., "authentication_errors")
- **Tags**: Use lowercase, meaningful categories (e.g., "smoke", "regression", "critical")

### 2. Test Case Organization

- Group related test cases in the same table
- Use tags to categorize tests by type (smoke, regression, integration)
- Include both positive (happy path) and negative test cases

### 3. Input Data Structure

- Keep input_data consistent across test cases in a table
- Use clear, descriptive field names
- Include all necessary context for the test

### 4. Expected Results

- Always specify expected_status for HTTP tests
- Use expected_message for substring matching (not exact match)
- Use expected_fields for additional response validation

### 5. Documentation

- Keep descriptions clear and specific
- Add tags for easy filtering
- Include metadata for traceability (e.g., JIRA ticket IDs)

## Real-World Example: Complete API Test Suite

```python
from tests.test_tables import ErrorTestTable, TestCase, run_test_table

# Create comprehensive test suite
api_test_suite = ErrorTestTable(
    name="api_comprehensive",
    description="Complete API test suite",
    tags=["comprehensive", "integration"],
    test_cases=[
        # Authentication tests
        TestCase(
            id="API-001",
            description="Valid authentication",
            input_data={"endpoint": "/api/data", "token": "valid-token"},
            expected_status=200,
            tags=["auth", "smoke"]
        ),
        TestCase(
            id="API-002",
            description="Invalid authentication",
            input_data={"endpoint": "/api/data", "token": "invalid"},
            expected_status=401,
            expected_error="unauthorized",
            tags=["auth"]
        ),

        # Validation tests
        TestCase(
            id="API-003",
            description="Valid input",
            input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "John", "email": "john@example.com"}},
            expected_status=201,
            tags=["validation", "smoke"]
        ),
        TestCase(
            id="API-004",
            description="Invalid email format",
            input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "John", "email": "invalid"}},
            expected_status=422,
            expected_error="validation_error",
            tags=["validation"]
        ),

        # Rate limiting tests
        TestCase(
            id="API-005",
            description="Within rate limit",
            input_data={"endpoint": "/api/search", "request_count": 10},
            expected_status=200,
            tags=["rate_limit", "smoke"]
        ),
        TestCase(
            id="API-006",
            description="Exceeds rate limit",
            input_data={"endpoint": "/api/search", "request_count": 1000},
            expected_status=429,
            expected_error="rate_limited",
            expected_fields={"retry_after": 60},
            tags=["rate_limit"]
        )
    ]
)

# Run the suite
def api_executor(input_data):
    # Your API testing logic here
    pass

results = run_test_table(api_test_suite, api_executor)

# Generate report
print(f"API Test Suite Results:")
print(f"  Total: {results.total_count}")
print(f"  Passed: {results.passed_count}")
print(f"  Failed: {results.failed_count}")
print(f"  Pass Rate: {results.pass_rate:.1f}%")
print(f"  Execution Time: {results.execution_time_ms:.0f}ms")
```

## Extending for New Error Types

When adding support for a new error type:

1. **Identify the error pattern**: What triggers this error? What should the response look like?

2. **Create test cases covering**:
   - Minimal triggering condition
   - Edge cases
   - Valid input (happy path)

3. **Add to a new or existing table**:
   ```python
   new_error_table = ErrorTestTable(
       name="new_error_type",
       description="Test cases for new error type",
       test_cases=[...]
   )
   ```

4. **Register in collection** (if reusable):
   ```python
   from tests.test_tables import ALL_ERROR_TABLES
   ALL_ERROR_TABLES['new_error_type'] = new_error_table
   ```

## Testing the Test Tables

Create test cases that validate your test tables:

```python
import unittest
from tests.test_tables import create_auth_test_table

class TestTestTables(unittest.TestCase):
    def test_auth_table_structure(self):
        """Test that auth table is properly structured."""
        table = create_auth_test_table()

        # Check table metadata
        self.assertEqual(table.name, "authentication_errors")
        self.assertTrue(len(table.test_cases) > 0)

        # Check test case IDs are unique
        ids = [tc.id for tc in table.test_cases]
        self.assertEqual(len(ids), len(set(ids)))

    def test_auth_table_coverage(self):
        """Test that auth table covers all scenarios."""
        table = create_auth_test_table()

        # Check for required test scenarios
        descriptions = [tc.description.lower() for tc in table.test_cases]
        self.assertIn("missing", descriptions)
        self.assertIn("invalid", descriptions)
```

## Summary

ARMOR's extensible test table structure provides:

- **Consistency**: All tests follow the same structure
- **Reusability**: Test patterns defined once, used everywhere
- **Maintainability**: Easy to add, modify, or remove test cases
- **Documentation**: Test tables document expected behavior
- **Flexibility**: Support for custom validation, setup/teardown, filtering

For questions or contributions, refer to the main test_tables module documentation.

---

**Bead**: bf-1a04kj
**Created**: 2026-07-14
**Updated**: 2026-07-14
