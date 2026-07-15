# Test Table Extension Guide

## Overview

This guide provides comprehensive documentation for extending ARMOR's test table structure to add new error type tests. Test tables provide a reusable, maintainable way to organize and execute data-driven tests across different error scenarios.

**Key Benefits:**
- **DRY Principle**: Define validation logic once, reuse across many test cases
- **Maintainability**: Clear separation between test data and test logic
- **Extensibility**: Easy to add new test cases by extending tables
- **Documentation**: Test tables serve as living documentation of expected behavior
- **Organization**: Group related test cases together with tags and metadata

---

## Architecture Overview

### Core Components

```
┌─────────────────────────────────────────────────────────────┐
│                     ErrorTestTable                           │
│  - name: "authentication_errors"                            │
│  - description: "Test cases for auth error responses"        │
│  - tags: ["auth", "security", "critical"]                    │
│  - test_cases: [TestCase, TestCase, ...]                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       TestCase                                │
│  - id: "AUTH-001"                                            │
│  - description: "Missing API key"                            │
│  - input_data: {"endpoint": "/api/users", ...}              │
│  - expected_status: 401                                      │
│  - expected_error: "unauthorized"                           │
│  - expected_message: "API key required"                     │
│  - expected_fields: {...}                                    │
│  - tags: ["smoke", "missing-credentials"]                   │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Test Execution                            │
│  run_test_table(table, executor) → TestTableResult          │
│  - Runs all test cases with provided executor                │
│  - Returns aggregated results with pass/fail statistics     │
└─────────────────────────────────────────────────────────────┘
```

---

## Required Fields for Test Cases

Every `TestCase` must include the following fields:

### Essential Fields (Required)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `id` | `str` | Unique identifier for the test case | `"AUTH-001"`, `"VAL-001"` |
| `description` | `str` | Human-readable description of what the test validates | `"Missing API key in request headers"` |
| `input_data` | `Dict[str, Any]` | Input parameters for the test (endpoint, headers, body, etc.) | `{"endpoint": "/api/users", "headers": {}}` |

### Expected Output Fields (At least one required)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `expected_status` | `int` (optional) | Expected HTTP status code | `401`, `422`, `404` |
| `expected_error` | `str` (optional) | Expected error type/identifier | `"unauthorized"`, `"validation_error"` |
| `expected_message` | `str` (optional) | Expected error message (or substring) | `"API key required"` |
| `expected_fields` | `Dict[str, Any]` (optional) | Additional fields to validate in response | `{"attempts_remaining": 2}` |

### Optional Metadata Fields

| Field | Type | Description | Default |
|-------|------|-------------|---------|
| `tags` | `List[str]` | Tags for categorization and filtering | `[]` |
| `enabled` | `bool` | Whether this test case is enabled | `True` |
| `setup_callback` | `Callable` | Function to run before test execution | `None` |
| `teardown_callback` | `Callable` | Function to run after test execution | `None` |
| `custom_validator` | `Callable` | Custom validation function for this test case | `None` |
| `metadata` | `Dict[str, Any]` | Additional test metadata | `{}` |

---

## Step-by-Step Guide: Adding New Error Type Tables

### Step 1: Identify Your Error Category

Before creating a new test table, identify which category of errors you're testing:

**Common Error Categories:**
- **Authentication/Authorization**: Missing credentials, invalid tokens, expired keys
- **Validation**: Missing fields, invalid formats, out-of-range values, type mismatches
- **Not Found**: Non-existent resources, deleted items
- **Rate Limiting**: Request throttling, retry-after headers
- **Server Errors**: Internal errors, upstream failures, timeouts
- **Method Restrictions**: Unsupported methods, forbidden operations
- **Content-Type Issues**: Unsupported media types, encoding errors

**Example Decision Tree:**
```
Does the error relate to credentials/permissions?
  └─ YES → Authentication/Authorization table
  └─ NO → Does it relate to input validation?
      └─ YES → Validation table
      └─ NO → Does it relate to missing resources?
          └─ YES → Not Found table
          └─ NO → Consider creating a new category
```

### Step 2: Define Your Test Table Structure

Create a function that returns an `ErrorTestTable` with your test cases:

```python
from tests.test_tables import (
    ErrorTestTable,
    TestCase,
    create_simple_test_table,
)

def create_my_error_type_table() -> ErrorTestTable:
    """
    Create a test table for your error type.

    Returns:
        ErrorTestTable: Configured test table for your error scenarios
    """
    return ErrorTestTable(
        name="my_error_type",
        description="Test cases for my error type responses",
        tags=["my-error-category", "4xx", "custom"],
        test_cases=[
            # Test cases will be added here
        ]
    )
```

### Step 3: Add Test Cases to Your Table

Add test cases following these patterns:

#### Pattern 1: Simple Error Test Case

```python
TestCase(
    id="MYERR-001",
    description="Clear description of what triggers the error",
    input_data={
        "endpoint": "/api/endpoint",
        "method": "POST",
        "body": {"key": "value"}
    },
    expected_status=422,
    expected_error="my_error_type",
    expected_message="Expected error message",
    tags=["smoke"]
)
```

#### Pattern 2: Test Case with Field Validation

```python
TestCase(
    id="MYERR-002",
    description="Error with specific field details",
    input_data={
        "endpoint": "/api/endpoint",
        "field": "email",
        "value": "invalid"
    },
    expected_status=422,
    expected_error="validation_error",
    expected_message="Invalid email format",
    expected_fields={
        "field": "email",
        "provided_value": "invalid",
        "constraint": "must contain @"
    },
    tags=["validation", "email"]
)
```

#### Pattern 3: Happy Path Test Case

```python
TestCase(
    id="MYERR-003",
    description="Valid request accepted",
    input_data={
        "endpoint": "/api/endpoint",
        "method": "POST",
        "body": {"valid": "data"}
    },
    expected_status=201,
    expected_error=None,  # No error expected
    tags=["happy-path", "smoke"]
)
```

### Step 4: Create an Executor Function

The executor function simulates or makes actual HTTP requests and returns the response data:

```python
from typing import Dict, Any

def my_error_type_executor(input_data: Dict[str, Any]) -> Dict[str, Any]:
    """
    Executor for my error type tests.

    In production, this would make actual HTTP requests.
    For testing, it can simulate responses based on input.

    Args:
        input_data: Test case input parameters

    Returns:
        Dict containing:
            - status: HTTP status code
            - error: Error type (or None if no error)
            - message: Error message (or success message)
            - fields: Additional fields from response
    """
    # Extract input data
    endpoint = input_data.get("endpoint", "")
    method = input_data.get("method", "GET")
    body = input_data.get("body", {})

    # Simulate logic based on input
    # In production: make actual HTTP request here
    if method == "POST" and "invalid" in body.get("email", ""):
        return {
            "status": 422,
            "error": "validation_error",
            "message": "Invalid email format",
            "fields": {
                "field": "email",
                "provided_value": body.get("email"),
                "constraint": "must contain @"
            }
        }

    # Default: valid request
    return {
        "status": 201,
        "error": None,
        "message": "Resource created successfully"
    }
```

### Step 5: Run Your Test Table

Use the `run_test_table()` function to execute your test cases:

```python
from tests.test_tables import run_test_table

# Create the test table
table = create_my_error_type_table()

# Run all test cases
results = run_test_table(
    table=table,
    executor=my_error_type_executor,
    continue_on_failure=True  # Continue running tests after failures
)

# Check results
print(f"Total: {results.total_count}")
print(f"Passed: {results.passed_count}")
print(f"Failed: {results.failed_count}")
print(f"Pass Rate: {results.pass_rate:.1f}%")

assert results.all_passed, f"Some tests failed: {results.failed_count} failures"
```

---

## Complete Example: Authentication Error Table

Here's a complete example from the existing authentication error implementation:

### 1. Table Definition

```python
def create_auth_test_table() -> ErrorTestTable:
    """Create authentication error test table."""
    return ErrorTestTable(
        name="authentication_errors",
        description="Test cases for authentication and authorization error responses",
        tags=["auth", "security", "critical"],
        test_cases=[
            # Missing credentials
            TestCase(
                id="AUTH-001",
                description="Missing API key in request headers",
                input_data={"endpoint": "/api/users", "headers": {}},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="API key required",
                tags=["smoke", "missing-credentials"]
            ),

            # Invalid credentials
            TestCase(
                id="AUTH-004",
                description="Invalid API key format",
                input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "invalid-format"}},
                expected_status=403,
                expected_error="forbidden",
                expected_message="Invalid API key",
                tags=["invalid-credentials", "api-key"]
            ),

            # Wrong password
            TestCase(
                id="AUTH-009",
                description="Wrong password for basic authentication",
                input_data={"endpoint": "/api/auth/login", "username": "user@example.com", "password": "wrongpassword"},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="Invalid username or password",
                expected_fields={"attempts_remaining": 2},
                tags=["wrong-password", "basic-auth"]
            ),

            # Happy path
            TestCase(
                id="AUTH-013",
                description="Valid API key accepted",
                input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "valid-key-123"}},
                expected_status=200,
                expected_error=None,
                tags=["happy-path", "smoke"]
            )
        ]
    )
```

### 2. Executor Function

```python
def auth_test_executor(input_data: Dict[str, Any]) -> Dict[str, Any]:
    """Executor for authentication tests."""
    headers = input_data.get('headers', {})

    # Missing credentials
    if 'X-API-Key' not in headers and 'Authorization' not in headers:
        return {
            'status': 401,
            'error': 'unauthorized',
            'message': 'API key required'
        }

    # Valid credentials
    api_key = headers.get('X-API-Key', '')
    if 'valid' in api_key:
        return {
            'status': 200,
            'error': None,
            'message': 'Success'
        }

    # Invalid credentials
    return {
        'status': 403,
        'error': 'forbidden',
        'message': 'Invalid API key'
    }
```

### 3. Running the Tests

```python
# Create and run the table
auth_table = create_auth_test_table()
results = run_test_table(auth_table, auth_test_executor)

# Print results
print(f"Passed: {results.passed_count}/{results.total_count}")
print(f"Pass rate: {results.pass_rate:.1f}%")

# Show failures
for result in results.results:
    if result.failed:
        print(f"✗ {result.test_case.id}: {result.error_message}")
```

---

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

## Complete Example: Validation Error Table

Here's a complete example from the validation error implementation:

### 1. Table Definition

```python
def create_validation_test_table() -> ErrorTestTable:
    """Create validation error test table."""
    return ErrorTestTable(
        name="validation_errors",
        description="Test cases for input validation error responses",
        tags=["validation", "input", "4xx"],
        test_cases=[
            # Missing required field
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

            # Invalid format
            TestCase(
                id="VAL-002",
                description="Invalid email format",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"email": "not-an-email"}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="Invalid email format",
                tags=["regression"]
            ),

            # Out of range
            TestCase(
                id="VAL-003",
                description="Numeric field out of range",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"age": 150}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="age must be between",
                tags=["regression"]
            ),

            # Happy path
            TestCase(
                id="VAL-004",
                description="Valid request accepted",
                input_data={"endpoint": "/api/users", "method": "POST", "body": {"name": "Jane", "email": "jane@example.com", "age": 25}},
                expected_status=201,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )
```

### 2. Running with Pytest

```python
import pytest

class TestValidationErrors:
    """Test validation error scenarios."""

    def test_all_validation_errors(self):
        """Run all validation error test cases."""
        table = create_validation_test_table()
        results = run_test_table(table, validation_test_executor)

        # Print summary
        print(f"\n{'='*60}")
        print(f"Validation Error Test Results")
        print(f"{'='*60}")
        print(f"Total: {results.total_count}")
        print(f"Passed: {results.passed_count}")
        print(f"Failed: {results.failed_count}")
        print(f"Pass Rate: {results.pass_rate:.1f}%")

        assert results.all_passed, f"Some tests failed: {results.failed_count} failures"

    def test_missing_required_field(self):
        """Test specific missing field scenario."""
        table = create_validation_test_table()
        test_case = table.get_test_case("VAL-001")

        result = run_test_case(test_case, validation_test_executor)

        assert result.passed, f"Test failed: {result.error_message}"
        print(f"✓ {test_case.id}: {test_case.description}")
```

---

## Test Case Naming Conventions

Use consistent naming conventions for test case IDs:

### By Error Type
- **Authentication**: `AUTH-001`, `AUTH-002`, ...
- **Validation**: `VAL-001`, `VAL-002`, ...
- **Not Found**: `NF-001`, `NF-002`, ...
- **Rate Limiting**: `RL-001`, `RL-002`, ...
- **Server Error**: `SE-001`, `SE-002`, ...

### By Category (More Granular)
- **Missing Fields**: `VAL-MISS-001`, `VAL-MISS-002`, ...
- **Invalid Format**: `VAL-FMT-001`, `VAL-FMT-002`, ...
- **Out of Range**: `VAL-RANGE-001`, `VAL-RANGE-002`, ...
- **Type Mismatch**: `VAL-TYPE-001`, `VAL-TYPE-002`, ...
- **Happy Path**: `VAL-HAPPY-001`, `VAL-HAPPY-002`, ...

---

## Tagging Strategy

Use tags to categorize and filter test cases:

### Common Tags

| Tag | Usage | Examples |
|-----|-------|----------|
| `smoke` | Critical tests that should always pass | Missing credentials, basic validation |
| `regression` | Tests for previously fixed bugs | Edge cases, historical issues |
| `happy-path` | Valid requests that should succeed | Valid authentication, valid input |
| `critical` | Security or stability critical tests | Authentication, authorization, rate limiting |
| `integration` | Tests requiring external services | Database, upstream API calls |

### Category-Specific Tags

**Authentication:**
- `missing-credentials`, `invalid-credentials`, `expired-credentials`
- `wrong-password`, `rbac`, `permissions`

**Validation:**
- `missing-field`, `invalid-format`, `out-of-range`, `type-mismatch`
- `email`, `date`, `url`, `phone`, `uuid`

**HTTP Methods:**
- `get`, `post`, `put`, `delete`, `patch`

### Filtering by Tags

```python
# Get all smoke tests
smoke_tests = table.get_test_cases_by_tag("smoke")

# Filter table by multiple tags
filtered_table = table.filter_by_tags("smoke", "auth")

# Get only critical tests
critical_tests = [tc for tc in table.test_cases if "critical" in tc.tags]
```

---

## Best Practices

### 1. Test Case Design

**DO:**
- Make test cases independent and idempotent
- Use descriptive test case names and IDs
- Include both error and happy path scenarios
- Test edge cases and boundary conditions
- Use tags for categorization

**DON'T:**
- Create test cases that depend on each other
- Use vague descriptions like "test error"
- Over-complicate test logic in the table
- Skip happy path testing

### 2. Input Data Structure

**DO:**
- Use consistent field names across test cases
- Include all relevant input parameters
- Use realistic test data values
- Document non-standard fields

**DON'T:**
- Overload input_data with unnecessary fields
- Use magic numbers or unclear values
- Mix different endpoint patterns in the same table

### 3. Expected Output

**DO:**
- Be specific about expected values
- Include expected error messages
- Validate important response fields
- Test both success and failure paths

**DON'T:**
- Leave expected_error as None for error tests
- Use overly broad message matching
- Ignore important validation fields

### 4. Table Organization

**DO:**
- Group related test cases together
- Use clear table names and descriptions
- Add comprehensive tags
- Document non-obvious test scenarios

**DON'T:**
- Create massive tables with 100+ test cases
- Mix unrelated error types in one table
- Use generic table names like "errors"

### 5. Executor Functions

**DO:**
- Keep executors simple and focused
- Handle all expected error scenarios
- Return consistent response structure
- Include helpful error messages

**DON'T:**
- Put complex business logic in executors
- Mix different response formats
- Return incomplete response data

---

## Common Pitfalls

### Pitfall 1: Duplicate Test Case IDs

```python
# WRONG: Duplicate IDs
test_cases = [
    TestCase(id="AUTH-001", ...),
    TestCase(id="AUTH-001", ...),  # Duplicate!
]

# RIGHT: Unique IDs
test_cases = [
    TestCase(id="AUTH-001", ...),
    TestCase(id="AUTH-002", ...),
]
```

### Pitfall 2: Missing Required Fields

```python
# WRONG: Missing expected_status for error test
TestCase(
    id="AUTH-001",
    description="Missing API key",
    input_data={"endpoint": "/api/users"},
    # Missing expected_status!
)

# RIGHT: Include expected_status
TestCase(
    id="AUTH-001",
    description="Missing API key",
    input_data={"endpoint": "/api/users"},
    expected_status=401,
    expected_error="unauthorized",
)
```

### Pitfall 3: Vague Test Descriptions

```python
# WRONG: Vague description
TestCase(
    id="VAL-001",
    description="test error",  # Too vague!
    ...
)

# RIGHT: Clear description
TestCase(
    id="VAL-001",
    description="Missing required email field in user creation",
    ...
)
```

### Pitfall 4: Not Testing Happy Paths

```python
# WRONG: Only error cases
test_cases = [
    TestCase(id="VAL-001", expected_status=422, ...),
    TestCase(id="VAL-002", expected_status=422, ...),
    # No happy path!
]

# RIGHT: Include happy path
test_cases = [
    TestCase(id="VAL-001", expected_status=422, ...),
    TestCase(id="VAL-002", expected_status=422, ...),
    TestCase(id="VAL-003", expected_status=201, expected_error=None, ...),  # Happy path
]
```

### Pitfall 5: Over-Complicated Custom Validators

```python
# WRONG: Complex custom validator
def custom_validator(actual):
    # 50 lines of complex validation logic
    ...

# RIGHT: Keep it simple or use expected_fields
TestCase(
    id="AUTH-001",
    ...
    expected_fields={"attempts_remaining": 2},  # Use built-in field validation
)
```

---

## Advanced Usage

### Custom Validators

For complex validation logic, use custom validators:

```python
def validate_response_structure(actual: Dict[str, Any]) -> Tuple[bool, str]:
    """Custom validator for response structure."""
    if "timestamp" not in actual:
        return False, "Response missing timestamp field"

    if "request_id" not in actual:
        return False, "Response missing request_id field"

    return True, ""

TestCase(
    id="ADV-001",
    description="Complex validation scenario",
    input_data={"endpoint": "/api/complex"},
    expected_status=200,
    custom_validator=validate_response_structure,
    tags=["advanced"]
)
```

### Setup and Teardown Callbacks

Use callbacks for test-specific setup/teardown:

```python
def setup_test_data(input_data: Dict[str, Any]):
    """Create test data before test runs."""
    # Create test user, database records, etc.
    print(f"Setting up test data for {input_data['endpoint']}")

def teardown_test_data(input_data: Dict[str, Any]):
    """Clean up test data after test runs."""
    # Delete test records, clear cache, etc.
    print(f"Tearing down test data for {input_data['endpoint']}")

TestCase(
    id="ADV-002",
    description="Test with setup/teardown",
    input_data={"endpoint": "/api/users", "method": "POST"},
    expected_status=201,
    setup_callback=setup_test_data,
    teardown_callback=teardown_test_data,
    tags=["integration"]
)
```

### Table Composition

Merge multiple tables into one:

```python
# Create specialized tables
missing_fields_table = create_missing_fields_table()
invalid_format_table = create_invalid_format_table()

# Merge into comprehensive table
comprehensive_table = missing_fields_table.merge(invalid_format_table)
```

### Filtering and Views

Create filtered views of tables:

```python
# Get only smoke tests
smoke_table = comprehensive_table.filter_by_tags("smoke")

# Get only regression tests for email validation
email_regression_table = comprehensive_table.filter_by_tags("regression", "email")

# Get only enabled test cases
enabled_tests = table.get_enabled_test_cases()
```

---

## Integration with Pytest

Create pytest-compatible test classes:

```python
import pytest

class TestMyErrorType:
    """Pytest test class for my error type."""

    def test_all_scenarios(self):
        """Run all test cases in the table."""
        table = create_my_error_type_table()
        results = run_test_table(table, my_executor)

        # Print summary
        print(f"\n{'='*60}")
        print(f"My Error Type Test Results")
        print(f"{'='*60}")
        print(f"Total: {results.total_count}")
        print(f"Passed: {results.passed_count}")
        print(f"Failed: {results.failed_count}")
        print(f"Pass Rate: {results.pass_rate:.1f}%")

        # Assert all passed
        assert results.all_passed

    def test_specific_scenario(self):
        """Test a specific scenario."""
        table = create_my_error_type_table()
        test_case = table.get_test_case("MYERR-001")

        result = run_test_case(test_case, my_executor)

        assert result.passed, f"Test failed: {result.error_message}"

    def test_table_structure(self):
        """Verify the table has proper structure."""
        table = create_my_error_type_table()

        # Check metadata
        assert table.name == "my_error_type"
        assert len(table.test_cases) > 0

        # Check test cases
        for tc in table.test_cases:
            assert tc.id
            assert tc.description
            assert tc.input_data
            assert tc.expected_status is not None

    @pytest.mark.smoke
    def test_smoke_tests(self):
        """Run only smoke tests."""
        table = create_my_error_type_table()
        smoke_table = table.filter_by_tags("smoke")

        results = run_test_table(smoke_table, my_executor)

        assert results.all_passed, "Smoke tests should always pass"
```

Run with pytest:
```bash
# Run all tests
pytest tests/test_my_error_type.py -v

# Run only smoke tests
pytest tests/test_my_error_type.py -m smoke -v

# Run with coverage
pytest tests/test_my_error_type.py --cov=tests --cov-report=html
```

---

## Running Test Tables

### From Python Code

```python
from tests.test_tables import run_test_table, create_auth_test_table

# Create and run table
table = create_auth_test_table()
results = run_test_table(table, auth_executor)

# Check results
if results.all_passed:
    print(f"✓ All {results.total_count} tests passed")
else:
    print(f"✗ {results.failed_count} tests failed")

    # Show failures
    for result in results.results:
        if result.failed:
            print(f"  {result.test_case.id}: {result.error_message}")
```

### From Command Line (Pytest)

```bash
# Run all test tables
pytest tests/test_*.py -v

# Run specific test file
pytest tests/test_auth_error_scenarios.py -v

# Run specific test
pytest tests/test_auth_error_scenarios.py::TestMissingCredentials::test_missing_api_key -v

# Run with coverage
pytest tests/ --cov=tests --cov-report=html

# Run only smoke tests
pytest tests/ -m smoke -v
```

---

## Summary Checklist

When creating a new test table, ensure you:

- [ ] Identified the correct error category
- [ ] Defined table with clear name, description, and tags
- [ ] Created test cases with unique IDs
- [ ] Included all required fields (id, description, input_data)
- [ ] Set appropriate expected outputs (status, error, message)
- [ ] Added both error and happy path test cases
- [ ] Used descriptive test case descriptions
- [ ] Tagged test cases for categorization
- [ ] Created executor function
- [ ] Tested the table with `run_test_table()`
- [ ] Added pytest test class if needed
- [ ] Documented non-obvious scenarios

---

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

---

## Related Documentation

- [README.md](README.md) - Test infrastructure overview
- [TEST_PATTERNS.md](TEST_PATTERNS.md) - Detailed testing patterns
- [fixtures/README.md](fixtures/README.md) - Error fixtures documentation
- [test_tables.py](test_tables.py) - Test table implementation
- [internal/validate/TYPES_DOCUMENTATION.md](../internal/validate/TYPES_DOCUMENTATION.md) - Error type documentation

---

## Bead Tracking

This documentation is part of bead `bf-6cpnyg` - "Add documentation for extending test tables".

Created: 2026-07-15
Last Updated: 2026-07-15
