# Error Tests Module

## Overview

The Error Tests module provides the foundational structure for table-driven error testing in ARMOR. It defines extensible patterns for testing specific error types and serves as the base for all error test modules.

## Architecture

The module is organized into three core components:

### 1. **TestPattern**
A single test pattern definition that encapsulates the setup, execution, and validation phases of a test.

**Key Features:**
- `name`: Unique identifier for the pattern
- `description`: Human-readable description
- `category`: Error category (auth, validation, client error, etc.)
- `setup()`: Optional setup function for fixtures/data
- `execute()`: Optional execution function
- `validate()`: Validation function returning (success, message)
- `cleanup()`: Optional cleanup function
- `enabled`: Enable/disable the pattern
- `tags`: List of tags for filtering
- `metadata`: Additional custom metadata

### 2. **ErrorTestSuite**
A container for organizing multiple test patterns.

**Key Features:**
- `name`: Unique suite name
- `description`: Suite description
- `patterns`: List of TestPattern objects
- `add_pattern()`: Add a pattern to the suite
- `execute_all()`: Execute all enabled patterns
- `get_patterns_by_category()`: Filter patterns by category
- `get_categories()`: Get all unique categories
- `setup_callback()`: Optional suite-level setup
- `teardown_callback()`: Optional suite-level teardown

### 3. **Pattern Categories**
Predefined categories for common error types:

- `ErrorCategory.CLIENT_ERROR`: 4xx client errors
- `ErrorCategory.SERVER_ERROR`: 5xx server errors
- `ErrorCategory.AUTHENTICATION`: Authentication errors
- `ErrorCategory.AUTHORIZATION`: Authorization errors
- `ErrorCategory.VALIDATION`: Validation errors
- `ErrorCategory.RATE_LIMIT`: Rate limiting errors
- `ErrorCategory.NETWORK`: Network errors
- `ErrorCategory.DATABASE`: Database errors
- `ErrorCategory.FILESYSTEM`: Filesystem errors
- `ErrorCategory.BUSINESS_LOGIC`: Business logic errors
- `ErrorCategory.PROTOCOL`: Protocol-specific errors

## Quick Start

### Basic Pattern Creation

```python
from tests.error_tests import TestPattern, ErrorCategory

pattern = TestPattern(
    name="not_found",
    description="Test 404 Not Found error",
    category=ErrorCategory.CLIENT_ERROR,
    setup=lambda: not_found_fixture(path="/api/test"),
    execute=lambda fixture: validate_http_status(fixture.to_tuple(), 404),
    validate=lambda result: (result.status_code == 404, "Should be 404")
)
```

### Creating a Test Suite

```python
from tests.error_tests import ErrorTestSuite

suite = ErrorTestSuite(
    name="Client Error Tests",
    description="Tests for 4xx client errors"
)
suite.add_pattern(pattern1)
suite.add_pattern(pattern2)

results = suite.execute_all()
results.print_summary()
```

### Using Helper Functions

```python
from tests.error_tests import create_base_pattern, create_error_suite

pattern = create_base_pattern(
    name="auth_test",
    description="Test authentication error",
    category=ErrorCategory.AUTHENTICATION,
    setup=lambda: {"error": "unauthorized"},
    validate=lambda data: (data.get("error") == "unauthorized", "Should be unauthorized")
)

suite = create_error_suite(
    name="Auth Tests",
    description="Authentication error tests",
    patterns=[pattern]
)
```

## Advanced Usage

### Pattern with Tags and Metadata

```python
pattern = TestPattern(
    name="validation_error",
    description="Test validation error",
    category=ErrorCategory.VALIDATION,
    tags=["smoke", "critical", "validation"],
    metadata={
        "priority": "high",
        "owner": "auth-team",
        "ticket": "AUTH-123"
    }
)
```

### Filtering by Tags

```python
# Execute only smoke tests
results = suite.execute_all(filter_tags=["smoke"])
```

### Suite with Setup/Teardown

```python
def setup_suite():
    print("Setting up test suite")

def teardown_suite():
    print("Tearing down test suite")

suite = ErrorTestSuite(
    name="My Suite",
    description="Suite with setup/teardown",
    setup_callback=setup_suite,
    teardown_callback=teardown_suite
)
```

### Pattern Methods

```python
# Add tags to existing pattern
pattern_with_tags = pattern.with_tags("regression", "integration")

# Add metadata
pattern_with_meta = pattern.with_metadata(priority="high", owner="team")
```

## Extension Guide

### Creating a New Error Test Module

To create a new module for a specific error type (e.g., S3 errors):

1. **Create a new module file:**

```python
# tests/error_tests/s3_tests.py

from tests.error_tests.base import (
    TestPattern,
    ErrorTestSuite,
    ErrorCategory
)

def create_s3_not_found_pattern(bucket: str) -> TestPattern:
    """Create a pattern for S3 bucket not found error."""
    return TestPattern(
        name="s3_bucket_not_found",
        description=f"Test S3 bucket not found for {bucket}",
        category=ErrorCategory.PROTOCOL,
        tags=["s3", "not_found"],
        setup=lambda: s3_error_fixture(
            error_code="NoSuchBucket",
            bucket=bucket
        ),
        validate=lambda response: (
            response.get("error") == "NoSuchBucket",
            "Should return NoSuchBucket error"
        )
    )

def create_s3_test_suite() -> ErrorTestSuite:
    """Create a comprehensive S3 error test suite."""
    patterns = [
        create_s3_not_found_pattern("nonexistent-bucket"),
        create_s3_access_denied_pattern(),
        create_s3_invalid_key_pattern()
    ]

    return ErrorTestSuite(
        name="S3 Error Tests",
        description="S3 protocol-specific error tests",
        patterns=patterns,
        tags=["s3", "protocol"]
    )
```

2. **Export from `__init__.py`:**

```python
# tests/error_tests/__init__.py

from .s3_tests import create_s3_not_found_pattern, create_s3_test_suite

__all__ += [
    'create_s3_not_found_pattern',
    'create_s3_test_suite'
]
```

3. **Use the new module:**

```python
from tests.error_tests import create_s3_test_suite

suite = create_s3_test_suite()
results = suite.execute_all()
```

## Best Practices

### 1. Pattern Naming
- Use lowercase with underscores: `not_found`, `unauthorized`
- Be descriptive but concise
- Include error type in name: `s3_bucket_not_found`, `validation_email_format`

### 2. Category Selection
- Use `ErrorCategory` constants for consistency
- Choose the most specific category applicable
- Create custom categories if needed (use strings)

### 3. Tag Usage
- Use tags for test selection and filtering
- Common tags: `smoke`, `critical`, `regression`, `integration`
- Include error-type tags: `auth`, `validation`, `s3`

### 4. Validation Functions
- Return `(success: bool, message: str)` tuple
- Provide clear, actionable error messages
- Validate both success and failure cases

### 5. Setup/Execute/Validate Separation
- **Setup**: Prepare fixtures, data, or environment
- **Execute**: Run the actual test logic
- **Validate**: Check results against expectations

### 6. Error Handling
- Patterns that raise exceptions during execution get `ERROR` status
- Patterns that fail validation get `FAILED` status
- Use the `error_message` field for debugging

## Results and Reporting

### Pattern Result

```python
result = pattern.execute_pattern()

print(f"Passed: {result.passed}")
print(f"Failed: {result.failed}")
print(f"Skipped: {result.skipped}")
print(f"Error: {result.error}")
print(f"Execution Time: {result.execution_time_ms}ms")
```

### Suite Result

```python
results = suite.execute_all()

print(f"Total: {results.total_count}")
print(f"Passed: {results.passed_count}")
print(f"Failed: {results.failed_count}")
print(f"Pass Rate: {results.pass_rate}%")
print(f"All Passed: {results.all_passed}")
```

### Human-Readable Summary

```python
results.print_summary()
```

Output:
```
============================================================
Test Suite: API Error Responses
============================================================
Description: Test common API error response structures
Total: 3
Passed: 3
Failed: 0
Skipped: 0
Errors: 0
Pass Rate: 100.0%
Execution Time: 0.01ms
============================================================
```

## Integration with ARMOR Test Infrastructure

This module integrates seamlessly with existing ARMOR test infrastructure:

```python
from tests.error_tests import TestPattern, ErrorTestSuite
from tests.fixtures.error_scenarios import not_found_fixture
from tests.test_helpers import validate_http_status
from tests.test_error_response_validation import validate_standard_error_response

# Create a pattern using ARMOR fixtures and validators
pattern = TestPattern(
    name="not_found",
    description="Test 404 Not Found",
    category=ErrorCategory.CLIENT_ERROR,
    setup=lambda: not_found_fixture(path="/api/test"),
    execute=lambda fixture: fixture.to_tuple(),
    validate=lambda response: (
        validate_http_status(response, 404) is None,
        "Should be valid 404 response"
    )
)
```

## Example Use Cases

### 1. Authentication Error Tests

```python
auth_patterns = [
    TestPattern(
        name="missing_api_key",
        description="Test missing API key",
        category=ErrorCategory.AUTHENTICATION,
        setup=lambda: auth_error_fixture(error="missing_key"),
        validate=lambda r: r["error"] == "missing_key"
    ),
    TestPattern(
        name="invalid_api_key",
        description="Test invalid API key",
        category=ErrorCategory.AUTHENTICATION,
        setup=lambda: auth_error_fixture(error="invalid_key"),
        validate=lambda r: r["error"] == "invalid_key"
    )
]

auth_suite = create_error_suite(
    name="Authentication Tests",
    description="API key authentication tests",
    patterns=auth_patterns
)
```

### 2. Validation Error Tests

```python
validation_patterns = [
    TestPattern(
        name="email_format",
        description="Test email format validation",
        category=ErrorCategory.VALIDATION,
        setup=lambda: validation_error_fixture(field="email"),
        validate=lambda r: r["field"] == "email"
    ),
    TestPattern(
        name="phone_format",
        description="Test phone format validation",
        category=ErrorCategory.VALIDATION,
        setup=lambda: validation_error_fixture(field="phone"),
        validate=lambda r: r["field"] == "phone"
    )
]
```

### 3. Rate Limit Tests

```python
rate_limit_patterns = [
    TestPattern(
        name="rate_limit_exceeded",
        description="Test rate limit exceeded",
        category=ErrorCategory.RATE_LIMIT,
        tags=["rate_limit", "critical"],
        setup=lambda: rate_limit_fixture(limit=100, exceeded=150),
        validate=lambda r: r["error"] == "rate_limit_exceeded"
    )
]
```

## See Also

- [tests/test_tables.py](../test_tables.py) - Core table-driven testing framework
- [tests/TEST_TABLE_EXTENSION_GUIDE.md](../TEST_TABLE_EXTENSION_GUIDE.md) - Comprehensive extension guide
- [tests/example_test.py](../example_test.py) - General test examples
- [tests/error_tests/example_usage.py](example_usage.py) - This module's usage examples

## Bead Tracking

This module is part of bead `bf-2zqplr` - "Create base test file and table structure for error tests".

Created: 2026-07-15
