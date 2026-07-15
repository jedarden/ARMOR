# Test Table Extension Quickstart Guide

## Overview

This quickstart guide will help you quickly create and extend test tables for your ARMOR testing needs. Each section provides copy-paste examples for common scenarios.

## Quick Reference

### Create a Simple Test Table

```python
from tests.test_tables import TestCase, ErrorTestTable

def create_my_table() -> ErrorTestTable:
    return ErrorTestTable(
        name="my_errors",
        description="My custom error scenarios",
        tags=["custom", "api"],
        test_cases=[
            TestCase(
                id="CUSTOM-001",
                description="My first test case",
                input_data={"endpoint": "/api/test"},
                expected_status=400,
                expected_error="validation_error",
                expected_message="Invalid input"
            )
        ]
    )
```

### Run Your Test Table

```python
from tests.test_tables import run_test_table

def my_executor(input_data):
    """Execute test and return actual results."""
    response = make_request(input_data)
    return {
        'status': response.status_code,
        'error': response.json().get('error'),
        'message': response.json().get('message')
    }

table = create_my_table()
results = run_test_table(table, my_executor)
print(f"Passed: {results.passed_count}/{results.total_count}")
```

## Common Error Type Templates

### 1. Authentication Errors

```python
def create_auth_table() -> ErrorTestTable:
    return ErrorTestTable(
        name="auth_errors",
        description="Authentication error scenarios",
        tags=["auth", "security"],
        test_cases=[
            TestCase(
                id="AUTH-001",
                description="Missing credentials",
                input_data={"endpoint": "/api/secure"},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="Authentication required"
            ),
            TestCase(
                id="AUTH-002",
                description="Invalid credentials",
                input_data={"endpoint": "/api/secure", "credentials": "invalid"},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="Invalid credentials"
            )
        ]
    )
```

### 2. Validation Errors

```python
def create_validation_table() -> ErrorTestTable:
    return ErrorTestTable(
        name="validation_errors",
        description="Validation error scenarios",
        tags=["validation", "input"],
        test_cases=[
            TestCase(
                id="VAL-001",
                description="Missing required field",
                input_data={"endpoint": "/api/users", "body": {}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="required field",
                expected_fields={"missing_field": "email"}
            ),
            TestCase(
                id="VAL-002",
                description="Invalid email format",
                input_data={"endpoint": "/api/users", "body": {"email": "invalid"}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="Invalid email format"
            )
        ]
    )
```

### 3. Resource Not Found Errors

```python
def create_not_found_table() -> ErrorTestTable:
    return ErrorTestTable(
        name="not_found_errors",
        description="404 Not Found scenarios",
        tags=["404", "not_found"],
        test_cases=[
            TestCase(
                id="NF-001",
                description="Non-existent resource",
                input_data={"endpoint": "/api/users/999"},
                expected_status=404,
                expected_error="not_found",
                expected_message="Resource not found"
            )
        ]
    )
```

### 4. Rate Limiting Errors

```python
def create_rate_limit_table() -> ErrorTestTable:
    return ErrorTestTable(
        name="rate_limit_errors",
        description="Rate limiting scenarios",
        tags=["rate_limit", "throttling"],
        test_cases=[
            TestCase(
                id="RL-001",
                description="Request rate exceeded",
                input_data={"endpoint": "/api/data", "request_count": 101},
                expected_status=429,
                expected_error="rate_limited",
                expected_message="Too many requests",
                expected_fields={"retry_after": 60}
            )
        ]
    )
```

### 5. Server Errors

```python
def create_server_error_table() -> ErrorTestTable:
    return ErrorTestTable(
        name="server_errors",
        description="Server error scenarios",
        tags=["5xx", "server_error"],
        test_cases=[
            TestCase(
                id="SE-001",
                description="Internal server error",
                input_data={"endpoint": "/api/crash"},
                expected_status=500,
                expected_error="internal_server_error",
                expected_message="Internal server error"
            ),
            TestCase(
                id="SE-002",
                description="Service unavailable",
                input_data={"endpoint": "/api/data"},
                expected_status=503,
                expected_error="service_unavailable",
                expected_message="Service temporarily unavailable"
            )
        ]
    )
```

## Advanced Patterns

### Pattern 1: Programmatic Test Generation

```python
def generate_validation_tests(fields: List[str]) -> List[TestCase]:
    """Generate validation tests for multiple fields."""
    test_cases = []
    for field in fields:
        test_cases.append(TestCase(
            id=f"VAL-{field.upper()}-001",
            description=f"Missing {field} field",
            input_data={"endpoint": "/api/users", "body": {}},
            expected_status=422,
            expected_error="validation_error",
            expected_message=f"{field} is required",
            expected_fields={"missing_field": field}
        ))
    return test_cases

# Use it
validation_table = ErrorTestTable(
    name="validation_errors",
    description="Generated validation tests",
    test_cases=generate_validation_tests(["email", "name", "age"])
)
```

### Pattern 2: Matrix Testing

```python
def create_auth_matrix() -> List[TestCase]:
    """Create authentication test matrix."""
    scenarios = [
        ("none", 401, "unauthorized", "missing"),
        ("invalid", 401, "unauthorized", "invalid"),
        ("expired", 401, "unauthorized", "expired"),
        ("valid", 200, None, "success")
    ]
    
    test_cases = []
    for i, (auth_type, status, error, desc) in enumerate(scenarios, 1):
        test_cases.append(TestCase(
            id=f"AUTH-MATRIX-{i:03d}",
            description=f"Auth {desc}: {auth_type}",
            input_data={"endpoint": "/api/secure", "auth_type": auth_type},
            expected_status=status,
            expected_error=error,
            tags=["auth", "matrix"]
        ))
    return test_cases
```

### Pattern 3: Custom Validation

```python
def custom_validator(actual: Dict[str, Any]) -> tuple[bool, str]:
    """Custom validation logic."""
    if 'custom_field' not in actual:
        return False, "Missing custom_field"
    if not isinstance(actual['custom_field'], dict):
        return False, "custom_field must be dict"
    return True, None

TestCase(
    id="CUSTOM-001",
    description="Test with custom validation",
    input_data={"endpoint": "/api/custom"},
    expected_status=200,
    custom_validator=custom_validator
)
```

## Working with Tags

### Filter Tests by Tag

```python
# Get only smoke tests
smoke_tests = table.get_test_cases_by_tag("smoke")

# Get only critical tests
critical_tests = table.get_test_cases_by_tag("critical")

# Get tests with multiple tags
filtered = table.filter_by_tags("smoke", "api")
```

### Run Filtered Tests

```python
# Run only smoke tests
smoke_table = ErrorTestTable(
    name=f"{table.name}_smoke",
    description="Smoke tests only",
    test_cases=table.get_test_cases_by_tag("smoke")
)
results = run_test_table(smoke_table, executor)
```

## Combining Tables

### Merge Tables

```python
combined_table = auth_table.merge(validation_table)
results = run_test_table(combined_table, executor)
```

### Create Suite

```python
def create_test_suite() -> Dict[str, ErrorTestTable]:
    """Create a comprehensive test suite."""
    return {
        'authentication': create_auth_table(),
        'validation': create_validation_table(),
        'not_found': create_not_found_table(),
        'rate_limit': create_rate_limit_table(),
        'server_error': create_server_error_table()
    }

# Run all tests
suite = create_test_suite()
all_results = {}
for name, table in suite.items():
    results = run_test_table(table, executor)
    all_results[name] = results

# Print summary
for name, results in all_results.items():
    print(f"{name}: {results.passed_count}/{results.total_count} passed")
```

## Setup and Teardown

### Test-Level Setup/Teardown

```python
def setup_test(input_data):
    """Setup before test."""
    resource_id = create_test_resource()
    input_data['resource_id'] = resource_id

def teardown_test(input_data):
    """Cleanup after test."""
    if 'resource_id' in input_data:
        delete_test_resource(input_data['resource_id'])

TestCase(
    id="SETUP-001",
    description="Test with setup and teardown",
    input_data={"endpoint": "/api/resources"},
    expected_status=200,
    setup_callback=setup_test,
    teardown_callback=teardown_test
)
```

### Table-Level Setup/Teardown

```python
def table_setup():
    """Setup before entire table."""
    initialize_test_database()

def table_teardown():
    """Cleanup after entire table."""
    cleanup_test_database()

table = ErrorTestTable(
    name="my_table",
    description="My test table",
    setup_callback=table_setup,
    teardown_callback=table_teardown,
    test_cases=[...]
)
```

## Working with Results

### Check Results

```python
results = run_test_table(table, executor)

# Check if all passed
if results.all_passed:
    print("All tests passed!")

# Get pass rate
print(f"Pass rate: {results.pass_rate:.1f}%")

# Get individual results
for result in results.results:
    if result.failed:
        print(f"Test {result.test_case.id} failed: {result.error_message}")
```

### Export Results

```python
# Convert to dictionary
results_dict = results.to_dict()

# Save to file
import json
with open('test_results.json', 'w') as f:
    json.dump(results_dict, f, indent=2)
```

## Best Practices Quick Reference

### DO ✓

- Use consistent ID prefixes: `AUTH-001`, `VAL-001`, `NF-001`
- Write clear, descriptive descriptions
- Include tags for categorization
- Add success cases (happy paths)
- Make tests independent
- Use custom validators for complex validation

### DON'T ✗

- Don't duplicate test IDs
- Don't make tests depend on each other
- Don't use vague descriptions
- Don't skip happy path tests
- Don't hardcode test data in test logic

## Common Executor Patterns

### HTTP Request Executor

```python
import requests

def http_executor(input_data):
    """Execute HTTP request and return results."""
    endpoint = input_data['endpoint']
    method = input_data.get('method', 'GET')
    headers = input_data.get('headers', {})
    body = input_data.get('body', None)
    
    response = requests.request(
        method=method,
        url=f"http://localhost:8080{endpoint}",
        headers=headers,
        json=body
    )
    
    return {
        'status': response.status_code,
        'error': response.json().get('error') if response.headers.get('content-type') == 'application/json' else None,
        'message': response.json().get('message') if response.headers.get('content-type') == 'application/json' else response.text,
        'fields': response.json().get('details', {}) if response.headers.get('content-type') == 'application/json' else {}
    }
```

### S3 Operation Executor

```python
import boto3

def s3_executor(input_data):
    """Execute S3 operation and return results."""
    operation = input_data['operation']
    bucket = input_data.get('bucket')
    key = input_data.get('key')
    
    s3 = boto3.client('s3')
    
    try:
        if operation == 'GetObject':
            response = s3.get_object(Bucket=bucket, Key=key)
            return {
                'status': 200,
                'error': None,
                'message': 'OK',
                'fields': {
                    'content_type': response['ContentType'],
                    'content_length': response['ContentLength']
                }
            }
        elif operation == 'PutObject':
            # ... handle other operations
            pass
    except s3.exceptions.NoSuchBucket:
        return {
            'status': 404,
            'error': 'NoSuchBucket',
            'message': 'The specified bucket does not exist',
            'fields': {'s3_error_code': 'NoSuchBucket'}
        }
    except Exception as e:
        return {
            'status': 500,
            'error': 'internal_error',
            'message': str(e),
            'fields': {}
        }
```

## Troubleshooting

### Issue: Import Errors

```bash
# Error: ImportError: No module named 'tests'
# Solution: Run from project root
cd /home/coding/ARMOR
python your_test_script.py
```

### Issue: Duplicate Test IDs

```python
# Error: Duplicate test case IDs found: {'AUTH-001'}
# Solution: Ensure all IDs are unique
test_cases = [
    TestCase(id="AUTH-001", ...),  # OK
    TestCase(id="AUTH-002", ...),  # OK
    # TestCase(id="AUTH-001", ...),  # ERROR - duplicate
]
```

### Issue: Validation Failures

```python
# Error: Expected status 404, got 200
# Solution: Check your expected values match actual API behavior
# Use the actual API response to verify expected values
```

## Complete Example

```python
#!/usr/bin/env python3
"""
Complete example of creating and running a custom test table.
"""

from tests.test_tables import TestCase, ErrorTestTable, run_test_table
import requests

def create_my_api_table() -> ErrorTestTable:
    """Create test table for my API."""
    return ErrorTestTable(
        name="my_api_errors",
        description="My API error scenarios",
        tags=["api", "integration"],
        test_cases=[
            TestCase(
                id="MYAPI-001",
                description="Missing API key",
                input_data={"endpoint": "/api/users"},
                expected_status=401,
                expected_error="unauthorized",
                expected_message="API key required",
                tags=["smoke", "auth"]
            ),
            TestCase(
                id="MYAPI-002",
                description="Invalid email format",
                input_data={"endpoint": "/api/users", "body": {"email": "invalid"}},
                expected_status=422,
                expected_error="validation_error",
                expected_message="Invalid email format",
                tags=["validation"]
            ),
            TestCase(
                id="MYAPI-003",
                description="Valid request",
                input_data={"endpoint": "/api/users", "headers": {"X-API-Key": "valid"}},
                expected_status=200,
                expected_error=None,
                tags=["happy-path"]
            )
        ]
    )

def my_api_executor(input_data):
    """Executor for my API tests."""
    endpoint = input_data['endpoint']
    headers = input_data.get('headers', {})
    body = input_data.get('body')
    
    response = requests.get(
        f"http://localhost:8080{endpoint}",
        headers=headers,
        json=body
    )
    
    return {
        'status': response.status_code,
        'error': response.json().get('error') if response.headers.get('content-type', '').startswith('application/json') else None,
        'message': response.json().get('message') if response.headers.get('content-type', '').startswith('application/json') else response.text,
        'fields': response.json().get('details', {}) if response.headers.get('content-type', '').startswith('application/json') else {}
    }

def main():
    """Run the test table."""
    table = create_my_api_table()
    results = run_test_table(table, my_api_executor)
    
    print(f"Results: {results.passed_count}/{results.total_count} passed")
    print(f"Pass rate: {results.pass_rate:.1f}%")
    
    if not results.all_passed:
        print("\nFailed tests:")
        for result in results.results:
            if result.failed:
                print(f"  - {result.test_case.id}: {result.error_message}")
    
    return 0 if results.all_passed else 1

if __name__ == '__main__':
    import sys
    sys.exit(main())
```

## Resources

- **TEST_TABLE_EXTENSION_GUIDE.md** - Comprehensive extension guide
- **test_table_extensions_examples.py** - Complete working examples
- **test_tables.py** - Core test table implementation
- **TEST_TABLE_GUIDE.md** - Detailed usage guide

Bead: bf-1a04kj
Created: 2026-07-15
