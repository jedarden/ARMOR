# Test Fixtures for ARMOR

This directory contains reusable test fixtures for the ARMOR test suite.

## Available Modules

### error_scenarios.py

HTTP error response fixtures for common scenarios. Provides pre-configured fixtures for:
- 404 Not Found
- 405 Method Not Allowed  
- 415 Unsupported Media Type
- 500 Internal Server Error

And custom builders for any HTTP error response.

## Quick Start

```python
from tests.fixtures.error_scenarios import (
    not_found_fixture,
    method_not_allowed_fixture,
    unsupported_media_type_fixture,
    internal_server_error_fixture
)

# Use a pre-configured fixture
error_404 = not_found_fixture(path="/api/users/123")
assert error_404.status_code == 404
assert error_404.body['error'] == 'not_found'

# Customize with allowed methods
error_405 = method_not_allowed_fixture(
    path="/api/users",
    method="DELETE",
    allowed_methods=["GET", "POST", "PUT"]
)

# Create custom error responses
from tests.fixtures.error_scenarios import create_error_response
custom_error = create_error_response(
    status=418,
    error_type="im_a_teapot",
    message="I'm a teapot"
)
```

## Fixture Examples

### 404 Not Found

```python
# Basic 404
fixture = not_found_fixture(path="/api/users/123")

# With custom message
fixture = not_found_fixture(
    path="/api/products/456",
    message="Product does not exist"
)

# Minimal 404 (only required fields)
fixture = not_found_simple(path="/api/resource")

# With suggested alternatives
fixture = not_found_with_suggestions(
    path="/api/users/999",
    suggestions=["/api/users", "/api/users/me"]
)
```

### 405 Method Not Allowed

```python
# Basic 405
fixture = method_not_allowed_fixture(
    path="/api/users",
    method="DELETE",
    allowed_methods=["GET", "POST", "PUT"]
)

# For read-only endpoints
fixture = method_not_allowed_read_only(
    path="/api/stats",
    method="POST"
)

# For write-only endpoints
fixture = method_not_allowed_write_only(
    path="/api/deleted",
    method="GET"
)
```

### 415 Unsupported Media Type

```python
# Basic 415
fixture = unsupported_media_type_fixture(
    path="/api/users",
    content_type="application/xml",
    supported_types=["application/json"]
)

# For JSON-only endpoints
fixture = unsupported_media_type_json_only(
    path="/api/users",
    provided_type="text/plain"
)

# For missing Content-Type header
fixture = unsupported_media_type_missing(path="/api/upload")
```

### 500 Internal Server Error

```python
# Basic 500
fixture = internal_server_error_fixture(
    path="/api/users",
    error_id="ERR-12345"
)

# For database errors
fixture = internal_server_error_database(
    path="/api/users",
    db_error="Connection timeout"
)

# For external service failures
fixture = internal_server_error_external_service(
    path="/api/payments",
    service_name="Stripe API"
)

# With stack trace (for debugging)
fixture = internal_server_error_fixture(
    path="/api/test",
    include_stack_trace=True
)
```

## Fixture Collections

Pre-configured fixture collections are available:

```python
from tests.fixtures.error_scenarios import (
    COMMON_ERROR_FIXTURES,
    CLIENT_ERROR_FIXTURES,
    SERVER_ERROR_FIXTURES,
    ALL_ERROR_FIXTURES
)

# Access a specific fixture
fixture_404 = COMMON_ERROR_FIXTURES['not_found']
fixture_500 = SERVER_ERROR_FIXTURES['500_internal_server_error']

# List all available fixtures
from tests.fixtures.error_scenarios import list_fixtures
names = list_fixtures()  # Returns sorted list of fixture names

# Get a fixture by name
from tests.fixtures.error_scenarios import get_fixture
fixture = get_fixture('500_internal_server_error')
```

## Fixture Methods

The `ErrorResponseFixture` class provides several utility methods:

### to_tuple()

Convert to tuple format: `(status_code, headers, body)`

```python
fixture = not_found_fixture()
status, headers, body = fixture.to_tuple()
assert status == 404
```

### to_response_dict()

Convert to dictionary format: `{'status_code': ..., 'headers': ..., 'body': ...}`

```python
fixture = not_found_fixture()
response = fixture.to_response_dict()
assert response['status_code'] == 404
```

### to_json_body()

Convert body to JSON string:

```python
fixture = not_found_fixture()
json_str = fixture.to_json_body()
import json
body = json.loads(json_str)
assert body['error'] == 'not_found'
```

### with_path(path)

Add or update path information:

```python
fixture = internal_server_error_fixture()
updated = fixture.with_path('/api/test')
assert updated.body['details']['path'] == '/api/test'
```

### with_request_id(request_id)

Add or update request ID:

```python
fixture = internal_server_error_fixture()
updated = fixture.with_request_id('REQ-12345')
assert updated.body['request_id'] == 'REQ-12345'
```

## Custom Error Responses

Use `create_error_response()` for custom HTTP errors:

```python
from tests.fixtures.error_scenarios import create_error_response

# Custom error with all fields
fixture = create_error_response(
    status=418,
    error_type="im_a_teapot",
    message="I'm a teapot",
    code=418,
    details={'RFC': '2324'},
    headers={'X-Custom-Header': 'value'}
)

# Minimal custom error
fixture = create_error_response(
    status=403,
    error_type="forbidden",
    message="Access denied"
)
```

## Batch Creation

Create multiple error responses at once:

```python
from tests.fixtures.error_scenarios import create_error_batch

errors = create_error_batch([
    {
        'status': 404,
        'error_type': 'not_found',
        'message': 'User not found',
        'path': '/api/users/1'
    },
    {
        'status': 403,
        'error_type': 'forbidden',
        'message': 'Access denied'
    },
    {
        'status': 500,
        'error_type': 'internal',
        'message': 'Internal error'
    }
])

assert len(errors) == 3
assert errors[0].status_code == 404
assert errors[1].status_code == 403
assert errors[2].status_code == 500
```

## Validation

All fixtures are compatible with ARMOR validation helpers:

```python
from tests.fixtures.error_scenarios import not_found_fixture
from tests.test_helpers import validate_http_status
from tests.test_error_response_validation import validate_standard_error_response

fixture = not_found_fixture()

# Validate status code
validate_http_status(fixture.to_tuple(), expected_status=404)

# Validate error structure
validate_standard_error_response(fixture.body)
```

## Running Tests

Test the fixtures themselves:

```bash
# Run with unittest
python3 -m unittest tests.test_error_scenario_fixtures -v

# Run with pytest (if available)
pytest tests/test_error_scenario_fixtures.py -v
```

## Contributing

When adding new error scenarios:

1. Add fixture functions to `error_scenarios.py`
2. Add comprehensive tests to `test_error_scenario_fixtures.py`
3. Update this README with examples
4. Ensure fixtures are compatible with validation helpers

## Bead Reference

- **Bead:** bf-7d2vgf
- **Created:** 2026-07-14
- **Acceptance Criteria:**
  - ✓ 404 Not Found test fixture with path/message
  - ✓ 405 Method Not Allowed test fixture with allowed methods
  - ✓ 415 Unsupported Media Type test fixture
  - ✓ 500 Internal Server Error test fixture
  - ✓ Fixtures are easily loadable and configurable for different endpoints
