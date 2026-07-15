# ARMOR Test Table Extension System - Summary

## Overview

The ARMOR test table extension system provides a comprehensive, extensible framework for creating table-driven tests for any error type or scenario. This document provides a high-level overview of the extension system and guides you to the appropriate documentation for your needs.

## What Are Test Tables?

Test tables are structured, data-driven test definitions that separate test data from test logic. They provide:

- **Consistency**: Standardized test structure across your codebase
- **Maintainability**: Easy to update test data without changing test code
- **Reusability**: Define test patterns once, reuse across multiple scenarios
- **Documentation**: Test tables serve as living documentation of expected behavior
- **Extensibility**: Simple to add new error types and test scenarios

## Core Architecture

### Components

```
┌─────────────────────────────────────────────────────────────┐
│                     Test Table System                         │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌──────────────┐  ┌─────────────────────────────────┐   │
│  │   TestCase   │  │      ErrorTestTable              │   │
│  │              │  │                                 │   │
│  │  • id        │  │  • name                        │   │
│  │  • description│  │  • description                 │   │
│  │  • input_data│  │  • test_cases[]                │   │
│  │  • expected_*│  │  • tags[]                      │   │
│  │  • tags[]    │  │  • setup/teardown              │   │
│  └──────────────┘  └─────────────────────────────────┘   │
│           │                       │                          │
│           │                       │                          │
│           ▼                       ▼                          │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Execution Layer                           │  │
│  │                                                         │  │
│  │  • run_test_case()                                    │  │
│  │  • run_test_table()                                   │  │
│  │  • Custom executor functions                          │  │
│  └──────────────────────────────────────────────────────┘  │
│           │                                                      │
│           ▼                                                      │
│  ┌──────────────────────────────────────────────────────┐  │
│  │              Results Layer                             │  │
│  │                                                         │  │
│  │  • TestExecutionResult                                 │  │
│  │  • TestTableResult                                     │  │
│  │  • Pass/fail/skip tracking                             │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Data Flow

```
Test Definition → Test Table → Executor → Validation → Results
     (TestCase)   (Collection)   (Function)   (Logic)    (Report)
```

## Quick Start

### 1. Choose Your Starting Point

**New to test tables?** Start here:
- [TEST_TABLE_QUICKSTART.md](TEST_TABLE_QUICKSTART.md) - Copy-paste examples and common patterns

**Creating custom error types?** Read this:
- [TEST_TABLE_EXTENSION_GUIDE.md](TEST_TABLE_EXTENSION_GUIDE.md) - Comprehensive extension guide

**Looking for working examples?** Check these:
- [test_table_extensions_examples.py](test_table_extensions_examples.py) - Complete, runnable examples

**Understanding the system?** Review these:
- [TEST_TABLE_GUIDE.md](TEST_TABLE_GUIDE.md) - Detailed usage guide
- [test_tables.py](test_tables.py) - Core implementation

### 2. Basic Template

```python
from tests.test_tables import TestCase, ErrorTestTable, run_test_table

# Define your test table
def create_my_table() -> ErrorTestTable:
    return ErrorTestTable(
        name="my_errors",
        description="My error scenarios",
        test_cases=[
            TestCase(
                id="MY-001",
                description="My test case",
                input_data={"endpoint": "/api/test"},
                expected_status=400,
                expected_error="validation_error",
                expected_message="Invalid input"
            )
        ]
    )

# Define your executor
def my_executor(input_data):
    response = make_request(input_data)
    return {
        'status': response.status_code,
        'error': response.json().get('error'),
        'message': response.json().get('message')
    }

# Run your tests
table = create_my_table()
results = run_test_table(table, my_executor)
print(f"Passed: {results.passed_count}/{results.total_count}")
```

## Extension Categories

### 1. Protocol-Specific Errors

Test errors specific to protocols like S3, WebSocket, gRPC:

- **S3 Errors**: NoSuchBucket, NoSuchKey, BucketAlreadyExists
- **HTTP Errors**: 4xx client errors, 5xx server errors
- **WebSocket Errors**: Connection failures, frame errors
- **gRPC Errors**: Status codes, metadata issues

**Example**: `create_s3_error_test_table()` in [test_table_extensions_examples.py](test_table_extensions_examples.py)

### 2. Infrastructure Errors

Test infrastructure-related errors:

- **Database Errors**: Connection issues, constraint violations, deadlocks
- **Network Errors**: Timeouts, DNS failures, connection resets
- **Filesystem Errors**: Permission issues, disk full, file not found
- **Cache Errors**: Misses, expiration, invalidation

**Examples**: 
- `create_database_error_test_table()`
- `create_network_error_test_table()`
- `create_filesystem_error_test_table()`

### 3. Business Logic Errors

Test domain-specific business rules:

- **Validation Errors**: Input validation, format checking, range validation
- **State Errors**: Invalid state transitions, concurrent modification
- **Constraint Errors**: Resource limits, quota exceeded, rule violations
- **Dependency Errors**: Missing dependencies, unsatisfied prerequisites

**Example**: `create_business_logic_error_test_table()`

### 4. Rate Limiting Errors

Test throttling and rate limiting:

- **Token Bucket**: Burst capacity, gradual recovery
- **Sliding Window**: Time-based limits
- **Fixed Window**: Per-period limits
- **Per-User/IP**: Individual rate limits

**Example**: `create_rate_limiting_test_table()`

## Available Pre-Configured Tables

The system includes these pre-configured tables:

| Table | Description | Error Types |
|-------|-------------|-------------|
| `authentication` | Authentication/authorization errors | 401, 403 |
| `validation` | Input validation errors | 422 |
| `not_found` | Resource not found errors | 404 |
| `rate_limit` | Rate limiting errors | 429 |
| `server_error` | Server error scenarios | 500, 502, 503, 504 |

## Extension Examples

The [test_table_extensions_examples.py](test_table_extensions_examples.py) file provides complete, working examples:

### 1. S3 Error Table (50+ test cases)
- Bucket operations (create, delete, list)
- Object operations (get, put, delete)
- ACL and permission errors
- Multi-part upload errors
- Lifecycle configuration errors

### 2. Database Error Table (20+ test cases)
- Connection errors (timeout, refused, unavailable)
- Constraint violations (unique, foreign key, null)
- Transaction errors (deadlock, lock timeout)
- Query syntax errors
- Result set errors

### 3. Network Error Table (15+ test cases)
- Connection timeout
- Connection refused
- DNS resolution failure
- SSL/TLS certificate errors
- Network unreachable
- Connection reset

### 4. Filesystem Error Table (15+ test cases)
- File/directory not found
- Permission denied
- Disk full
- File too large
- Invalid path
- Is directory error

### 5. Rate Limiting Table (20+ test cases)
- Token bucket limits
- Sliding window limits
- Fixed window limits
- Per-user limits
- Per-IP limits
- Burst capacity
- Gradual recovery

### 6. Business Logic Table (15+ test cases)
- Insufficient funds
- Invalid state transitions
- Duplicate resources
- Business rule violations
- Concurrent modification
- Dependency not satisfied

## Common Patterns

### Pattern 1: Error Type Matrix

```python
def create_error_matrix():
    errors = [
        ("missing", 401, "unauthorized"),
        ("invalid", 401, "unauthorized"),
        ("expired", 401, "unauthorized"),
        ("forbidden", 403, "forbidden"),
    ]
    return [
        TestCase(
            id=f"AUTH-{err_type.upper()}-{i:03d}",
            description=f"{err_type} credentials",
            input_data={"auth_type": err_type},
            expected_status=status,
            expected_error=error
        )
        for i, (err_type, status, error) in enumerate(errors, 1)
    ]
```

### Pattern 2: Field Validation Matrix

```python
def create_validation_matrix():
    fields = ["email", "name", "age"]
    return [
        TestCase(
            id=f"VAL-{field.upper()}-001",
            description=f"Missing {field} field",
            input_data={"body": {}},
            expected_status=422,
            expected_error="validation_error",
            expected_fields={"missing_field": field}
        )
        for field in fields
    ]
```

### Pattern 3: Custom Validation

```python
def custom_validator(actual):
    if 'custom_field' not in actual:
        return False, "Missing custom_field"
    return True, None

TestCase(
    id="CUSTOM-001",
    description="Custom validation",
    input_data={"endpoint": "/api/custom"},
    expected_status=200,
    custom_validator=custom_validator
)
```

## Best Practices

### DO ✓

1. **Use consistent naming**: `AUTH-001`, `VAL-001`, `S3-001`
2. **Write clear descriptions**: "Missing API key returns 401"
3. **Include tags**: `["smoke", "auth", "critical"]`
4. **Add success cases**: Always test happy paths
5. **Make tests independent**: No shared state between tests
6. **Document custom validators**: Explain validation logic

### DON'T ✗

1. **Don't duplicate IDs**: Each test must have a unique ID
2. **Don't create dependencies**: Tests should run independently
3. **Don't use vague descriptions**: "Test error" is not helpful
4. **Don't skip success cases**: Always include happy paths
5. **Don't hardcode test data**: Use input_data parameter
6. **Don't mix concerns**: One table should test one category

## Documentation Map

### Getting Started

- **[TEST_TABLE_QUICKSTART.md](TEST_TABLE_QUICKSTART.md)** - Quick reference and copy-paste examples
- **[TEST_TABLE_GUIDE.md](TEST_TABLE_GUIDE.md)** - Comprehensive usage guide

### Creating Extensions

- **[TEST_TABLE_EXTENSION_GUIDE.md](TEST_TABLE_EXTENSION_GUIDE.md)** - Detailed extension guide
- **[test_table_extensions_examples.py](test_table_extensions_examples.py)** - Complete working examples

### Core Documentation

- **[README.md](README.md)** - Test infrastructure overview
- **[TEST_PATTERNS.md](TEST_PATTERNS.md)** - Testing patterns and best practices
- **[test_tables.py](test_tables.py)** - Core implementation reference

## Running Test Tables

### Command Line

```bash
# Run all test tables
python -m pytest tests/test_example_test_tables.py -v

# Run specific table
python -m pytest tests/test_example_test_tables.py::test_s3_errors -v

# Run with coverage
python -m pytest tests/test_example_test_tables.py --cov=tests --cov-report=html
```

### Programmatically

```python
from tests.test_tables import run_test_table

# Run a single table
table = get_table('authentication')
results = run_test_table(table, my_executor)

# Run multiple tables
for table_name in ['authentication', 'validation', 'not_found']:
    table = get_table(table_name)
    results = run_test_table(table, my_executor)
    print(f"{table_name}: {results.passed_count}/{results.total_count}")
```

## Troubleshooting

### Common Issues

1. **Import Errors**
   - **Problem**: `ImportError: No module named 'tests'`
   - **Solution**: Run from project root: `cd /home/coding/ARMOR`

2. **Duplicate Test IDs**
   - **Problem**: `Duplicate test case IDs found: {'AUTH-001'}`
   - **Solution**: Ensure all test IDs in a table are unique

3. **Validation Failures**
   - **Problem**: `Expected status 404, got 200`
   - **Solution**: Verify expected values match actual API behavior

4. **Executor Returns Wrong Format**
   - **Problem**: Test fails with "Missing field: status"
   - **Solution**: Ensure executor returns dict with `status`, `error`, `message` keys

## Contributing

When adding new test table extensions:

1. **Follow patterns**: Use existing examples as templates
2. **Add documentation**: Document custom error types and validators
3. **Include examples**: Provide runnable example for your extension
4. **Update guides**: Add your pattern to extension guide if reusable
5. **Test thoroughly**: Ensure all test cases pass

## Version History

- **2026-07-15**: Initial extensible test table structure (bf-1a04kj)
- **2026-07-15**: Added extension examples for 6 error categories
- **2026-07-15**: Created comprehensive documentation suite

## Bead Tracking

This extensible test table structure is part of bead **bf-1a04kj** - "Create extensible test table structure".

## Support

For questions or issues:

1. Check the [TEST_TABLE_EXTENSION_GUIDE.md](TEST_TABLE_EXTENSION_GUIDE.md) for detailed guidance
2. Review [test_table_extensions_examples.py](test_table_extensions_examples.py) for working examples
3. Consult [TEST_TABLE_QUICKSTART.md](TEST_TABLE_QUICKSTART.md) for common patterns
4. Examine [test_tables.py](test_tables.py) for implementation details

## Summary

The ARMOR test table extension system provides:

- **Flexibility**: Create test tables for any error type
- **Consistency**: Standardized structure across all tests
- **Maintainability**: Easy to update and extend
- **Documentation**: Living documentation of test scenarios
- **Examples**: Comprehensive working examples for common patterns

Whether you're testing HTTP errors, S3 operations, database constraints, or business logic, the test table system provides the structure and patterns you need to create robust, maintainable tests.

---

**Next Steps**:

1. Read [TEST_TABLE_QUICKSTART.md](TEST_TABLE_QUICKSTART.md) for immediate examples
2. Review [TEST_TABLE_EXTENSION_GUIDE.md](TEST_TABLE_EXTENSION_GUIDE.md) for detailed guidance
3. Explore [test_table_extensions_examples.py](test_table_extensions_examples.py) for complete implementations
4. Start creating your own test tables!

Bead: bf-1a04kj
Created: 2026-07-15
