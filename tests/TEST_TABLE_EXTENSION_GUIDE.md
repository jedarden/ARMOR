# Test Table Extension Guide

## Overview

This guide provides comprehensive instructions for extending the ARMOR test table structure to support new error types and scenarios. The test table framework is designed to be easily extensible while maintaining consistency across your test suite.

## Table Structure Architecture

### Core Components

The test table framework consists of four main components:

1. **TestCase** - Individual test case definition with inputs, expected outputs, and metadata
2. **ErrorTestTable** - Collection of related test cases with shared metadata
3. **TestExecutionResult** - Result of executing a single test case
4. **TestTableResult** - Aggregated results from running a complete test table

### Design Principles

- **Immutability**: Test cases use immutable builder patterns to prevent state mutations
- **Composability**: Tables can be merged, filtered, and combined
- **Extensibility**: New error types can be added without modifying core framework
- **Type Safety**: Strong typing with dataclasses and enums
- **Separation of Concerns**: Test data separate from execution logic

## Creating Custom Test Tables

### Step 1: Define Your Test Table Structure

Start by defining what category of errors you want to test. Common categories include:

- **Authentication/Authorization errors** - Login failures, permission issues
- **Validation errors** - Input validation, format checking
- **Resource errors** - Missing resources, conflicts
- **Rate limiting errors** - Throttling, quota exceeded
- **Server errors** - Internal failures, upstream issues
- **Network errors** - Timeouts, connection issues

### Step 2: Create Test Cases

For each error scenario, create a `TestCase` with:

- **id**: Unique identifier (e.g., "AUTH-001", "VAL-001")
- **description**: Clear, human-readable description
- **input_data**: Test input parameters
- **expected_status**: HTTP status code (or None for non-HTTP tests)
- **expected_error**: Error type identifier (or None for success cases)
- **expected_message**: Expected error message (or substring)
- **expected_fields**: Optional dict of additional response fields to validate
- **tags**: List of tags for categorization and filtering
- **enabled**: Whether the test is currently enabled

### Step 3: Create the Test Table

Combine related test cases into an `ErrorTestTable`:

```python
from tests.test_tables import TestCase, ErrorTestTable

def create_my_custom_table() -> ErrorTestTable:
    """Create a test table for my custom error scenarios."""
    
    return ErrorTestTable(
        name="my_custom_errors",
        description="Test cases for custom error responses",
        tags=["custom", "integration"],
        test_cases=[
            TestCase(
                id="CUSTOM-001",
                description="First custom error scenario",
                input_data={"endpoint": "/api/custom", "param": "value"},
                expected_status=400,
                expected_error="custom_error",
                expected_message="Custom error message",
                tags=["smoke"]
            ),
            # Add more test cases...
        ]
    )
```

### Step 4: Register Your Table

Add your table to the global registry:

```python
from tests.test_tables import ALL_ERROR_TABLES

# Register your custom table
ALL_ERROR_TABLES['my_custom'] = create_my_custom_table()
```

## Pattern Library

### Pattern 1: HTTP Status Testing

For testing HTTP status codes:

```python
TestCase(
    id="HTTP-001",
    description="Test 404 response",
    input_data={"endpoint": "/api/users/999"},
    expected_status=404,
    expected_error="not_found",
    expected_message="Resource not found"
)
```

### Pattern 2: Field Validation

For testing specific response fields:

```python
TestCase(
    id="FIELD-001",
    description="Test response includes retry information",
    input_data={"endpoint": "/api/data"},
    expected_status=429,
    expected_error="rate_limited",
    expected_fields={
        "retry_after": 60,
        "limit": 100,
        "remaining": 0
    }
)
```

### Pattern 3: Authentication Matrix

For testing multiple authentication scenarios:

```python
auth_test_cases = [
    TestCase(
        id=f"AUTH-{i:03d}",
        description=f"Auth scenario {i}",
        input_data={"endpoint": "/api/secure", "auth_type": auth_type},
        expected_status=status,
        expected_error=error_type,
        tags=["auth", scenario_type]
    )
    for i, (auth_type, status, error_type, scenario_type) in enumerate([
        ("none", 401, "unauthorized", "missing"),
        ("invalid", 403, "forbidden", "invalid"),
        ("expired", 401, "unauthorized", "expired"),
        ("valid", 200, None, "success")
    ], 1)
]
```

### Pattern 4: Validation Matrix

For testing input validation:

```python
validation_cases = [
    TestCase(
        id=f"VAL-{field[:3].upper()}-{i:03d}",
        description=f"Test {field} validation: {scenario}",
        input_data={
            "endpoint": "/api/users",
            "method": "POST",
            "body": {field: invalid_value}
        },
        expected_status=422,
        expected_error="validation_error",
        expected_message=expected_message,
        tags=["validation", field]
    )
    for field, scenarios in validation_matrix.items()
    for i, (invalid_value, expected_message, scenario) in enumerate(scenarios, 1)
]
```

### Pattern 5: Conditional Testing

For tests with setup/teardown:

```python
def setup_test_resource(input_data):
    """Create a test resource before the test."""
    resource_id = create_test_resource(input_data)
    input_data['resource_id'] = resource_id

def teardown_test_resource(input_data):
    """Clean up test resource after the test."""
    if 'resource_id' in input_data:
        delete_test_resource(input_data['resource_id'])

TestCase(
    id="COND-001",
    description="Test with setup and teardown",
    input_data={"endpoint": "/api/resources"},
    expected_status=200,
    setup_callback=setup_test_resource,
    teardown_callback=teardown_test_resource,
    tags=["integration", "cleanup"]
)
```

### Pattern 6: Custom Validation

For complex validation logic:

```python
def custom_response_validator(actual_response):
    """Custom validation for complex response structure."""
    # Check custom conditions
    if 'custom_field' not in actual_response:
        return False, "Missing custom_field"
    
    if not isinstance(actual_response['custom_field'], dict):
        return False, "custom_field must be a dict"
    
    # Check nested structure
    nested = actual_response['custom_field']
    if 'timestamp' not in nested:
        return False, "Missing timestamp in custom_field"
    
    return True, None

TestCase(
    id="CUSTOM-001",
    description="Test with custom validation",
    input_data={"endpoint": "/api/custom"},
    expected_status=200,
    custom_validator=custom_response_validator,
    tags=["custom-validation"]
)
```

## Error Type Templates

### Authentication/Authorization Errors

```python
def create_auth_error_table() -> ErrorTestTable:
    """Template for authentication error testing."""
    return ErrorTestTable(
        name="auth_errors",
        description="Authentication and authorization error scenarios",
        tags=["auth", "security", "critical"],
        test_cases=[
            # Missing credentials
            TestCase(
                id="AUTH-MISS-001",
                description="Missing authentication credentials",
                input_data={"endpoint": "/api/secure"},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="Authentication required",
                tags=["smoke", "missing-credentials"]
            ),
            # Invalid credentials
            TestCase(
                id="AUTH-INV-001",
                description="Invalid authentication credentials",
                input_data={"endpoint": "/api/secure", "credentials": "invalid"},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="Invalid credentials",
                tags=["invalid-credentials"]
            ),
            # Expired credentials
            TestCase(
                id="AUTH-EXP-001",
                description="Expired authentication token",
                input_data={"endpoint": "/api/secure", "token_age": "expired"},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="Token expired",
                tags=["expired-credentials"]
            ),
            # Insufficient permissions
            TestCase(
                id="AUTH-PERM-001",
                description="Valid credentials, insufficient permissions",
                input_data={"endpoint": "/api/admin", "role": "user"},
                expected_status=403,
                expected_error="forbidden",
                expected_message="Insufficient permissions",
                tags=["rbac", "permissions"]
            ),
            # Success case
            TestCase(
                id="AUTH-SUCCESS-001",
                description="Valid authentication",
                input_data={"endpoint": "/api/secure", "credentials": "valid"},
                expected_status=200,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )
```

### Validation Errors

```python
def create_validation_error_table() -> ErrorTestTable:
    """Template for validation error testing."""
    return ErrorTestTable(
        name="validation_errors",
        description="Input validation error scenarios",
        tags=["validation", "input", "4xx"],
        test_cases=[
            # Missing required fields
            TestCase(
                id="VAL-REQ-001",
                description="Missing required field",
                input_data={"endpoint": "/api/users", "body": {}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="required field",
                expected_fields={"missing_fields": ["email"]},
                tags=["smoke", "required-fields"]
            ),
            # Invalid format
            TestCase(
                id="VAL-FMT-001",
                description="Invalid email format",
                input_data={"endpoint": "/api/users", "body": {"email": "invalid"}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="Invalid email format",
                tags=["format-validation"]
            ),
            # Out of range
            TestCase(
                id="VAL-RANGE-001",
                description="Value out of valid range",
                input_data={"endpoint": "/api/users", "body": {"age": 150}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="must be between",
                expected_fields={"field": "age", "min": 0, "max": 120},
                tags=["range-validation"]
            ),
            # Too long
            TestCase(
                id="VAL-LEN-001",
                description="String exceeds maximum length",
                input_data={"endpoint": "/api/users", "body": {"name": "x" * 300}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="too long",
                expected_fields={"field": "name", "max_length": 100},
                tags=["length-validation"]
            ),
            # Success case
            TestCase(
                id="VAL-SUCCESS-001",
                description="Valid input accepted",
                input_data={"endpoint": "/api/users", "body": {"email": "user@example.com"}},
                expected_status=201,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )
```

### Resource Errors

```python
def create_resource_error_table() -> ErrorTestTable:
    """Template for resource error testing."""
    return ErrorTestTable(
        name="resource_errors",
        description="Resource-related error scenarios",
        tags=["resource", "4xx"],
        test_cases=[
            # Not found
            TestCase(
                id="RES-NF-001",
                description="Resource not found",
                input_data={"endpoint": "/api/users/999"},
                expected_status=404,
                expected_error="not_found",
                expected_message="Resource not found",
                tags=["smoke", "not-found"]
            ),
            # Conflict
            TestCase(
                id="RES-CONFLICT-001",
                description="Resource conflict (duplicate)",
                input_data={"endpoint": "/api/users", "body": {"email": "existing@example.com"}},
                expected_status=409,
                expected_error="conflict",
                expected_message="already exists",
                tags=["conflict"]
            ),
            # Locked
            TestCase(
                id="RES-LOCK-001",
                description="Resource is locked",
                input_data={"endpoint": "/api/users/1", "action": "delete"},
                expected_status=423,
                expected_error="locked",
                expected_message="Resource is locked",
                tags=["locked"]
            ),
            # Gone
            TestCase(
                id="RES-GONE-001",
                description="Resource previously deleted",
                input_data={"endpoint": "/api/users/1", "context": "deleted"},
                expected_status=410,
                expected_error="gone",
                expected_message="Resource has been deleted",
                tags=["gone"]
            ),
            # Success case
            TestCase(
                id="RES-SUCCESS-001",
                description="Valid resource access",
                input_data={"endpoint": "/api/users/1"},
                expected_status=200,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )
```

## Best Practices

### 1. Consistent Naming

Use consistent ID prefixes and naming:

```python
# Good - consistent prefixes
"AUTH-001", "AUTH-002", "AUTH-003"
"VAL-REQ-001", "VAL-FMT-001", "VAL-RANGE-001"

# Bad - inconsistent prefixes
"auth-1", "AUTH_02", "Auth003"
```

### 2. Comprehensive Descriptions

Write clear, descriptive test descriptions:

```python
# Good - clear description
description="Missing API key in request headers returns 401 Unauthorized"

# Bad - vague description
description="Test error"
```

### 3. Tag Strategy

Use tags effectively for test organization:

```python
# Good - descriptive tags
tags=["smoke", "auth", "critical", "api-key"]

# Bad - non-descriptive tags
tags=["test1", "test2", "test3"]
```

### 4. Test Independence

Ensure each test case is independent:

```python
# Good - each test has its own data
TestCase(id="T-001", input_data={"user_id": 1, "data": "test1"})
TestCase(id="T-002", input_data={"user_id": 2, "data": "test2"})

# Bad - tests depend on shared state
TestCase(id="T-001", input_data={"use_global_state": True})
TestCase(id="T-002", input_data={"assume_T_001_ran": True})
```

### 5. Include Happy Paths

Always include success cases:

```python
# Good - includes both error and success
test_cases=[
    TestCase(..., expected_status=400, expected_error="validation_error"),  # Error case
    TestCase(..., expected_status=200, expected_error=None)  # Success case
]

# Bad - only error cases
test_cases=[
    TestCase(..., expected_status=400, expected_error="validation_error"),
]
```

## Running Custom Test Tables

### Basic Execution

```python
from tests.test_tables import run_test_table

# Define your executor function
def my_executor(input_data):
    """Execute test and return actual results."""
    # Make request, get response
    response = make_request(input_data)
    
    return {
        'status': response.status_code,
        'error': response.json().get('error'),
        'message': response.json().get('message'),
        'fields': response.json().get('details', {})
    }

# Run your custom table
table = create_my_custom_table()
results = run_test_table(table, my_executor)

# Check results
print(f"Passed: {results.passed_count}/{results.total_count}")
print(f"Pass rate: {results.pass_rate:.1f}%")
```

### Filtered Execution

```python
# Run only smoke tests
smoke_tests = table.get_test_cases_by_tag("smoke")
smoke_table = ErrorTestTable(
    name=f"{table.name}_smoke",
    description=f"Smoke tests from {table.name}",
    test_cases=smoke_tests
)
results = run_test_table(smoke_table, executor)

# Run only enabled tests
enabled_table = ErrorTestTable(
    name=f"{table.name}_enabled",
    description=f"Enabled tests from {table.name}",
    test_cases=table.get_enabled_test_cases()
)
results = run_test_table(enabled_table, executor)
```

## Advanced Patterns

### Dynamic Test Generation

Generate test cases programmatically:

```python
def generate_auth_tests(auth_methods: List[str]) -> List[TestCase]:
    """Generate authentication test cases for multiple methods."""
    test_cases = []
    
    for method in auth_methods:
        test_cases.extend([
            TestCase(
                id=f"AUTH-{method.upper()}-001",
                description=f"Missing {method} credentials",
                input_data={"auth_method": method, "credentials": None},
                expected_status=401,
                expected_error="unauthorized",
                tags=["auth", method, "missing-credentials"]
            ),
            TestCase(
                id=f"AUTH-{method.upper()}-002",
                description=f"Invalid {method} credentials",
                input_data={"auth_method": method, "credentials": "invalid"},
                expected_status=401,
                expected_error="unauthorized",
                tags=["auth", method, "invalid-credentials"]
            )
        ])
    
    return test_cases
```

### Table Composition

Combine multiple tables:

```python
# Create individual tables
auth_table = create_auth_error_table()
validation_table = create_validation_error_table()

# Merge tables
combined_table = auth_table.merge(validation_table)

# Run combined tests
results = run_test_table(combined_table, executor)
```

### Conditional Test Execution

Skip tests based on runtime conditions:

```python
def conditional_executor(input_data):
    """Executor that can skip tests based on conditions."""
    # Check if test should run
    if 'requires_feature' in input_data:
        if not feature_is_enabled(input_data['requires_feature']):
            return {
                'status': None,
                'error': None,
                'message': 'Feature not enabled',
                'skip': True
            }
    
    # Normal execution
    return execute_test(input_data)
```

## Troubleshooting

### Common Issues

**1. Duplicate Test Case IDs**

```python
# Error: Duplicate test case IDs found: {'AUTH-001'}
# Solution: Ensure all test IDs in a table are unique
test_cases = [
    TestCase(id="AUTH-001", ...),  # OK
    TestCase(id="AUTH-002", ...),  # OK
    # TestCase(id="AUTH-001", ...),  # ERROR - duplicate
]
```

**2. Validation Failures**

```python
# Error: Expected status 404, got 200
# Solution: Check your expected values match actual behavior
TestCase(
    id="TEST-001",
    description="Test description",
    input_data={"endpoint": "/api/users/999"},
    expected_status=404,  # Make sure this is what your API actually returns
    expected_error="not_found"
)
```

**3. Missing Expected Fields**

```python
# Error: Missing expected field: retry_after
# Solution: Include all fields you want to validate in expected_fields
expected_fields={
    "retry_after": 60,      # Required field
    "limit": 100,          # Required field
    "remaining": 0          # Required field
}
```

## Migration Guide

### Migrating from Existing Tests

If you have existing tests that you want to convert to test tables:

1. **Identify test patterns** - Look for repeated test structures
2. **Extract test data** - Separate test logic from test data
3. **Create test cases** - Convert each test to a TestCase
4. **Group into tables** - Combine related tests into ErrorTestTable
5. **Update execution** - Replace test loops with run_test_table()

### Example Migration

Before (traditional pytest):

```python
def test_auth_errors():
    """Test authentication errors."""
    # Missing credentials
    response = requests.get("/api/secure", headers={})
    assert response.status_code == 401
    
    # Invalid credentials
    response = requests.get("/api/secure", headers={"X-API-Key": "invalid"})
    assert response.status_code == 401
    
    # Expired credentials
    response = requests.get("/api/secure", headers={"X-API-Key": "expired"})
    assert response.status_code == 401
```

After (test table):

```python
def test_auth_errors():
    """Test authentication errors using test table."""
    table = create_auth_error_table()
    results = run_test_table(table, auth_executor)
    assert results.all_passed
```

## Resources

- **test_tables.py** - Core test table implementation
- **TEST_TABLE_GUIDE.md** - Comprehensive test table usage guide
- **TEST_PATTERNS.md** - Testing patterns and best practices
- **example_test.py** - Runnable examples

Bead: bf-1a04kj
Created: 2026-07-15
