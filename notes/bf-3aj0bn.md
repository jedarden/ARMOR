# HTTP Request Helper Implementation (bf-3aj0bn)

## Overview
Implemented a helper function to make HTTP requests to the test server in the ARMOR test suite.

## Implementation Details

### Files Modified
1. `/home/coding/ARMOR/tests/test_helpers.py` - Added HTTP request helper function
2. `/home/coding/ARMOR/tests/__init__.py` - Exported new helper functions

### New Components

#### HTTPRequestError Exception
Custom exception for HTTP request failures with clear error messages including:
- HTTP method and URL
- Error type (connection, timeout, etc.)
- Underlying error message

#### make_http_request Function
Complete HTTP request helper with the following features:

**Supported HTTP Methods:**
- GET
- POST
- PUT
- DELETE
- PATCH
- HEAD
- OPTIONS

**Parameters:**
- `base_url` (str): Base URL of the test server
- `path` (str, optional): Path to append to base URL
- `method` (str, optional): HTTP method (default: "GET")
- `headers` (Dict[str, str], optional): Request headers
- `body` (Optional[Union[str, Dict, bytes]]): Request body
- `timeout` (int, optional): Request timeout in seconds (default: 30)
- `raise_on_error` (bool, optional): Raise exception on error (default: False)

**Response Structure:**
```python
{
    'success': bool,           # True if request succeeded
    'status_code': int,        # HTTP status code (0 on error)
    'headers': Dict[str, str], # Response headers
    'body': str,               # Response body
    'url': str,                # Full URL requested
    'method': str,             # HTTP method used
    'error': Optional[str]     # Error type if failed
}
```

**Error Handling:**
- Connection errors → Returns error info or raises HTTPRequestError
- Timeout errors → Returns error info or raises HTTPRequestError
- HTTP errors → Returns error info or raises HTTPRequestError
- Invalid HTTP method → Raises ValueError
- Missing requests library → Raises ValueError with installation instructions

**Automatic JSON Serialization:**
- Dict bodies are automatically serialized to JSON
- Content-Type header set automatically if not provided

## Acceptance Criteria Met
✅ Function makes HTTP requests to a test server
✅ Supports configurable HTTP methods (GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)
✅ Allows setting request headers and body
✅ Returns full response including status code, headers, and body
✅ Handles connection errors gracefully
✅ Includes basic usage examples in docstring comments

## Usage Examples

```python
from tests import make_http_request, HTTPRequestError

# Simple GET request
response = make_http_request('http://localhost:8080', '/api/health')
if response['success']:
    print(f"Status: {response['status_code']}")

# POST request with JSON body
response = make_http_request(
    'http://localhost:8080',
    '/api/users',
    method='POST',
    headers={'Content-Type': 'application/json'},
    body={'name': 'test', 'value': 123}  # Auto-serialized to JSON
)

# PUT request with custom headers
response = make_http_request(
    'http://localhost:8080',
    '/api/users/123',
    method='PUT',
    headers={'Authorization': 'Bearer token123'},
    body='{"name": "updated"}'
)

# DELETE request
response = make_http_request(
    'http://localhost:8080',
    '/api/users/123',
    method='DELETE'
)

# With exception handling
try:
    response = make_http_request(
        'http://localhost:8080',
        '/api/test',
        raise_on_error=True
    )
except HTTPRequestError as e:
    print(f"Request failed: {e}")
```

## Design Decisions

1. **Followed existing patterns**: Used the same optional `requests` import pattern as existing code in `test_helpers.py`
2. **Consistent return format**: Returns a dictionary similar to other test helpers for consistency
3. **Graceful error handling**: Returns error info by default but can raise exceptions if needed
4. **Automatic JSON handling**: Dict bodies are automatically serialized to JSON for convenience
5. **Comprehensive examples**: Docstring includes multiple usage examples for common scenarios

## Notes
- The function requires the `requests` library to be installed
- If `requests` is not available, a clear error message is provided
- The implementation follows the existing code style and patterns in the test suite
- All functions are properly exported from `tests/__init__.py`
