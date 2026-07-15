"""
ARMOR Test Infrastructure Package

This package provides a comprehensive test infrastructure for validating ARMOR's HTTP
error responses and API behaviors. It includes reusable helpers, pre-configured test
fixtures, and structured test tables that make writing robust tests faster and more
consistent.

## Package Architecture

The test infrastructure is organized into three main layers:

### 1. Validation Helpers (test_helpers.py)

Low-level validation functions for HTTP responses:
- **Status Validation**: validate_http_status(), validate_success(), validate_client_error()
- **Content-Type Validation**: validate_content_type(), validate_json_content_type()
- **CORS Header Validation**: validate_cors_headers(), validate_cors_allow_origin()
- **HTTP Request Helper**: make_http_request() for making test requests

### 2. Error Response Fixtures (fixtures/error_scenarios.py)

Pre-configured error response fixtures for common HTTP error scenarios:
- **Client Errors (4xx)**: 400, 403, 404, 405, 415, 422, 429
- **Server Errors (5xx)**: 500, 502, 503, 504
- **Generic Builders**: create_error_response(), create_error_batch()

### 3. Test Tables (test_tables.py)

Structured test case management for data-driven testing:
- **Pre-configured Tables**: Common error scenarios, auth tests, validation tests
- **Table Builders**: create_auth_test_table(), create_not_found_test_table()
- **Execution Helpers**: run_test_case(), run_test_table()

## Quick Start

### Validating HTTP Status Codes

    from tests import validate_http_status, validate_client_error

    # Single status code
    validate_http_status(response, 200)

    # Multiple acceptable codes
    validate_http_status(response, [200, 201, 204])

    # Check for client error
    validate_client_error(response)

### Using Error Fixtures

    from tests import not_found_fixture, get_fixture

    # Create a 404 fixture
    fixture = not_found_fixture(path="/api/users/123")
    assert fixture.status_code == 404

    # Get pre-configured fixture
    fixture = get_fixture('404_not_found')

### Making Test Requests

    from tests import make_http_request

    response = make_http_request(
        base_url='http://localhost:8080',
        path='/api/health',
        method='GET'
    )

    if response['success']:
        print(f"Status: {response['status_code']}")

## Testing Patterns

### Pattern 1: Fixture-Based Testing

Use pre-configured fixtures for standard error scenarios:

    def test_404_not_found():
        fixture = not_found_fixture(path="/api/users/123")
        validate_http_status(fixture.to_tuple(), 404)
        validate_json_content_type(fixture.to_tuple())
        validate_standard_error_response(fixture.body)

### Pattern 2: Table-Based Testing

Use test tables for data-driven scenarios:

    table = create_not_found_test_table([
        TestCase(path="/api/users/999", expected_code="not_found"),
        TestCase(path="/api/items/abc", expected_code="not_found"),
    ])
    results = run_test_table(table)
    assert all(result.passed for result in results)

### Pattern 3: Helper-Based Validation

Use validation helpers for custom test logic:

    response = make_http_request('http://localhost:8080', '/api/error')
    validate_http_status(response['status_code'], 418)
    validate_content_type(response['headers'], 'application/json')

## Adding New Test Cases

When adding new error test cases, follow these steps:

1. **Choose Your Approach**:
   - Use existing fixtures for standard errors (404, 500, etc.)
   - Create custom fixtures with create_error_response() for new scenarios
   - Use test tables for multiple similar scenarios

2. **Write the Test**:
    def test_my_new_scenario():
        fixture = create_error_response(
            status=418,
            error_type="im_a_teapot",
            message="I'm a teapot"
        )
        validate_http_status(fixture.to_tuple(), 418)
        validate_standard_error_response(fixture.body)

3. **Run and Verify**:
    pytest tests/test_my_new_scenario.py -v

## Key Design Decisions

### Fixture Immutability

Error fixtures use immutable builder patterns to prevent accidental state mutations
in parallel tests:

    fixture = not_found_fixture(path="/api/users")
    with_path = fixture.with_path("/api/items")  # Returns new fixture

### Tuple-Based Response Format

Validation functions accept both requests.Response objects and simple tuples:

    # With requests.Response
    validate_http_status(response, 200)

    # With tuple (status, headers, body)
    validate_http_status((200, {'Content-Type': 'application/json'}, '{}'), 200)

### Pattern-Matching Content Types

Content-Type validation uses pattern matching to handle charset parameters:

    # Both match 'application/json'
    validate_content_type(response, 'application/json')
    validate_content_type(response, 'application/json; charset=utf-8')

## Module Organization

tests/
├── __init__.py                         # This file - package exports
├── README.md                           # Infrastructure overview
├── TEST_PATTERNS.md                   # Guide for adding new test cases
├── example_test.py                     # Runnable usage examples
├── test_helpers.py                     # Low-level validation helpers
├── test_error_response_validation.py  # Error response structure validation
├── test_tables.py                      # Test table management
└── fixtures/
    ├── __init__.py                     # Fixture exports
    ├── error_scenarios.py              # Error response fixtures
    └── README.md                       # Fixture documentation

## Exported API

### HTTP Status Validation
- validate_http_status() - Validate HTTP status codes
- validate_http_status_codes() - Validate multiple responses
- validate_success() - Check for 2xx status
- validate_client_error() - Check for 4xx status
- validate_server_error() - Check for 5xx status
- StatusValidationError - Exception for status validation failures

### Content-Type Validation
- validate_content_type() - Validate Content-Type headers
- validate_json_content_type() - Validate JSON content type
- validate_xml_content_type() - Validate XML content type
- validate_html_content_type() - Validate HTML content type
- validate_text_content_type() - Validate plain text content type
- ContentTypeValidationError - Exception for content-type validation failures

### CORS Header Validation
- validate_cors_headers() - Validate CORS headers
- validate_cors_allow_origin() - Check for CORS allow-origin header
- validate_cors_wildcard() - Validate wildcard CORS origin
- validate_cors_specific_origin() - Validate specific CORS origin
- validate_cors_credentials() - Validate CORS credentials header
- CORSValidationError - Exception for CORS validation failures

### HTTP Request Helper
- make_http_request() - Make HTTP requests with error handling
- HTTPRequestError - Exception for HTTP request failures

### Error Response Validation
- validate_error_response() - Validate error response structure
- validate_standard_error_response() - Validate standard error format
- validate_detailed_error_response() - Validate detailed error format
- validate_http_error_response() - Validate HTTP error responses
- ErrorResponseValidationError - Exception for error response validation failures

### Test Tables
- TestCase - Test case dataclass
- TestResult - Single test execution result
- TestTableResult - Test table execution results
- run_test_case() - Execute a single test case
- run_test_table() - Execute a test table
- create_simple_test_table() - Create a simple test table
- create_auth_test_table() - Create authentication test table
- create_not_found_test_table() - Create 404 test table
- create_validation_test_table() - Create validation test table
- create_rate_limit_test_table() - Create rate limit test table

## Related Documentation

- tests/README.md - Infrastructure overview and quick start
- tests/TEST_PATTERNS.md - Detailed guide for adding new test cases
- tests/example_test.py - Runnable usage examples
- tests/fixtures/README.md - Error fixtures documentation
- tests/TEST_TABLE_GUIDE.md - Test tables usage guide

## Contributing

When contributing to the test infrastructure:

1. **Add Examples**: New functions should include usage examples in docstrings
2. **Update Tests**: Add tests for new validation helpers
3. **Document Decisions**: Explain non-obvious design decisions in comments
4. **Follow Patterns**: Use existing patterns for consistency

Bead: bf-qxhx20
Created: 2026-07-15
"""

from .test_helpers import (
    validate_http_status,
    validate_http_status_codes,
    StatusValidationError,
    validate_content_type,
    validate_json_content_type,
    validate_xml_content_type,
    ContentTypeValidationError,
    validate_cors_headers,
    CORSValidationError,
    validate_cors_allow_origin,
    validate_cors_wildcard,
    validate_cors_specific_origin,
    validate_cors_credentials,
    HTTPRequestError,
    make_http_request,
)

from .test_error_response_validation import (
    validate_error_response,
    validate_error_field_only,
    validate_standard_error_response,
    validate_detailed_error_response,
    validate_http_error_response,
    validate_error_responses,
    validate_with_standard_validators,
    validate_with_detailed_validators,
    ErrorResponseSpec,
    ErrorResponseValidationError,
    STANDARD_ERROR_SPEC,
    DETAILED_ERROR_SPEC,
    MINIMAL_ERROR_SPEC,
)

from .test_tables import (
    TestCase,
    TestResult,
    TestExecutionResult,
    TestTableResult,
    ErrorTestTable,
    run_test_case,
    run_test_table,
    create_simple_test_table,
    create_auth_test_table,
    create_validation_test_table,
    create_not_found_test_table,
    create_rate_limit_test_table,
    create_server_error_test_table,
    get_table,
    list_tables,
    COMMON_ERROR_TABLES,
    ALL_ERROR_TABLES,
)

__all__ = [
    # HTTP status validation
    'validate_http_status',
    'validate_http_status_codes',
    'StatusValidationError',
    # Content-Type validation
    'validate_content_type',
    'validate_json_content_type',
    'validate_xml_content_type',
    'ContentTypeValidationError',
    # CORS header validation
    'validate_cors_headers',
    'CORSValidationError',
    'validate_cors_allow_origin',
    'validate_cors_wildcard',
    'validate_cors_specific_origin',
    'validate_cors_credentials',
    # HTTP request helper
    'HTTPRequestError',
    'make_http_request',
    # Error response structure validation
    'validate_error_response',
    'validate_error_field_only',
    'validate_standard_error_response',
    'validate_detailed_error_response',
    'validate_http_error_response',
    'validate_error_responses',
    'validate_with_standard_validators',
    'validate_with_detailed_validators',
    'ErrorResponseSpec',
    'ErrorResponseValidationError',
    'STANDARD_ERROR_SPEC',
    'DETAILED_ERROR_SPEC',
    'MINIMAL_ERROR_SPEC',
    # Test tables
    'TestCase',
    'TestResult',
    'TestExecutionResult',
    'TestTableResult',
    'ErrorTestTable',
    'run_test_case',
    'run_test_table',
    'create_simple_test_table',
    'create_auth_test_table',
    'create_validation_test_table',
    'create_not_found_test_table',
    'create_rate_limit_test_table',
    'create_server_error_test_table',
    'get_table',
    'list_tables',
    'COMMON_ERROR_TABLES',
    'ALL_ERROR_TABLES',
]
