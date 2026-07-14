#!/usr/bin/env python3
"""
Test Suite for Error Scenario Fixtures

This module validates that all HTTP error scenario fixtures are properly
structured and meet the acceptance criteria.

Acceptance Criteria:
- 404 Not Found fixture has path/message ✓
- 405 Method Not Allowed fixture has allowed methods ✓
- 415 Unsupported Media Type fixture exists ✓
- 500 Internal Server Error fixture exists ✓
- Fixtures are easily loadable and configurable ✓
- All fixtures have proper status codes ✓
- All fixtures have required error fields ✓

Bead: bf-7d2vgf
Created: 2026-07-14
"""

import unittest
import json
from typing import Dict, Any, List

# Import the fixtures
from tests.fixtures.error_scenarios import (
    ErrorResponseFixture,
    not_found_fixture,
    not_found_simple,
    not_found_with_suggestions,
    method_not_allowed_fixture,
    method_not_allowed_read_only,
    method_not_allowed_write_only,
    unsupported_media_type_fixture,
    unsupported_media_type_json_only,
    unsupported_media_type_missing,
    internal_server_error_fixture,
    internal_server_error_database,
    internal_server_error_external_service,
    create_error_response,
    create_error_batch,
    get_fixture,
    list_fixtures,
    COMMON_ERROR_FIXTURES,
    CLIENT_ERROR_FIXTURES,
    SERVER_ERROR_FIXTURES,
    ALL_ERROR_FIXTURES
)

# Import validation helpers
from tests.test_helpers import validate_http_status
from tests.test_error_response_validation import (
    validate_standard_error_response,
    validate_error_response
)


class TestErrorResponseFixture(unittest.TestCase):
    """Test the ErrorResponseFixture dataclass."""

    def test_fixture_creation(self):
        """Test that fixtures can be created with all fields."""
        fixture = ErrorResponseFixture(
            status_code=404,
            headers={'Content-Type': 'application/json'},
            body={'error': 'not_found', 'message': 'Not found'}
        )
        self.assertEqual(fixture.status_code, 404)
        self.assertEqual(fixture.headers['Content-Type'], 'application/json')
        self.assertEqual(fixture.body['error'], 'not_found')

    def test_to_tuple(self):
        """Test conversion to tuple format."""
        fixture = ErrorResponseFixture(
            status_code=404,
            headers={'Content-Type': 'application/json'},
            body={'error': 'not_found'}
        )
        tup = fixture.to_tuple()
        self.assertEqual(tup[0], 404)
        self.assertEqual(tup[1], {'Content-Type': 'application/json'})
        self.assertEqual(tup[2], {'error': 'not_found'})

    def test_to_response_dict(self):
        """Test conversion to response dictionary."""
        fixture = ErrorResponseFixture(
            status_code=404,
            headers={'Content-Type': 'application/json'},
            body={'error': 'not_found'}
        )
        resp = fixture.to_response_dict()
        self.assertEqual(resp['status_code'], 404)
        self.assertEqual(resp['headers'], {'Content-Type': 'application/json'})
        self.assertEqual(resp['body'], {'error': 'not_found'})

    def test_to_json_body(self):
        """Test conversion to JSON string."""
        fixture = ErrorResponseFixture(
            status_code=404,
            body={'error': 'not_found', 'message': 'Test'}
        )
        json_str = fixture.to_json_body()
        parsed = json.loads(json_str)
        self.assertEqual(parsed['error'], 'not_found')
        self.assertEqual(parsed['message'], 'Test')

    def test_with_path(self):
        """Test adding path to fixture."""
        fixture = ErrorResponseFixture(
            status_code=404,
            body={'error': 'not_found', 'message': 'Test'}
        )
        updated = fixture.with_path('/api/test')
        self.assertIn('details', updated.body)
        self.assertEqual(updated.body['details']['path'], '/api/test')
        # Original should be unchanged
        self.assertNotIn('details', fixture.body)

    def test_with_request_id(self):
        """Test adding request ID to fixture."""
        fixture = ErrorResponseFixture(
            status_code=500,
            body={'error': 'internal', 'message': 'Error'}
        )
        updated = fixture.with_request_id('REQ-12345')
        self.assertEqual(updated.body['request_id'], 'REQ-12345')
        # Original should be unchanged
        self.assertNotIn('request_id', fixture.body)


class Test404NotFoundFixtures(unittest.TestCase):
    """Test 404 Not Found error fixtures."""

    def test_not_found_fixture_basic(self):
        """Test basic 404 fixture creation."""
        fixture = not_found_fixture(path="/api/users/123")
        self.assertEqual(fixture.status_code, 404)
        self.assertEqual(fixture.body['error'], 'not_found')
        self.assertIn('/api/users/123', fixture.body['message'])
        self.assertEqual(fixture.body['code'], 404)
        self.assertIn('path', fixture.body['details'])
        self.assertEqual(fixture.body['details']['path'], '/api/users/123')

    def test_not_found_fixture_custom_message(self):
        """Test 404 with custom message."""
        fixture = not_found_fixture(
            path="/api/items/456",
            message="Custom not found message"
        )
        self.assertEqual(fixture.body['message'], 'Custom not found message')

    def test_not_found_fixture_custom_error_type(self):
        """Test 404 with custom error type."""
        fixture = not_found_fixture(
            path="/api/test",
            error_type="resource_missing"
        )
        self.assertEqual(fixture.body['error'], 'resource_missing')

    def test_not_found_simple(self):
        """Test minimal 404 fixture."""
        fixture = not_found_simple(path="/api/test")
        self.assertEqual(fixture.status_code, 404)
        self.assertEqual(fixture.body['error'], 'not_found')
        # Should have only required fields
        self.assertIn('message', fixture.body)
        self.assertNotIn('details', fixture.body)

    def test_not_found_with_suggestions(self):
        """Test 404 with suggested paths."""
        fixture = not_found_with_suggestions(
            path="/api/users/999",
            suggestions=["/api/users", "/api/users/me"]
        )
        self.assertEqual(fixture.status_code, 404)
        self.assertIn('suggested_paths', fixture.body['details'])
        self.assertEqual(fixture.body['details']['suggested_paths'], ["/api/users", "/api/users/me"])

    def test_not_found_default_suggestions(self):
        """Test 404 with default suggestions."""
        fixture = not_found_with_suggestions(path="/api/test")
        self.assertIn('suggested_paths', fixture.body['details'])
        self.assertIsInstance(fixture.body['details']['suggested_paths'], list)
        self.assertTrue(len(fixture.body['details']['suggested_paths']) > 0)


class Test405MethodNotAllowedFixtures(unittest.TestCase):
    """Test 405 Method Not Allowed error fixtures."""

    def test_method_not_allowed_basic(self):
        """Test basic 405 fixture."""
        fixture = method_not_allowed_fixture(
            path="/api/users",
            method="DELETE",
            allowed_methods=["GET", "POST"]
        )
        self.assertEqual(fixture.status_code, 405)
        self.assertEqual(fixture.body['error'], 'method_not_allowed')
        self.assertIn('Allow', fixture.headers)
        self.assertEqual(fixture.headers['Allow'], 'GET, POST')
        self.assertIn('allowed_methods', fixture.body['details'])
        self.assertEqual(fixture.body['details']['method'], 'DELETE')

    def test_method_not_allowed_default_methods(self):
        """Test 405 with default allowed methods."""
        fixture = method_not_allowed_fixture(
            path="/api/test",
            method="PATCH"
        )
        self.assertIn('allowed_methods', fixture.body['details'])
        self.assertIn('PATCH', fixture.body['details']['method'])

    def test_method_not_allowed_read_only(self):
        """Test 405 for read-only endpoint."""
        fixture = method_not_allowed_read_only(path="/api/stats", method="POST")
        self.assertEqual(fixture.status_code, 405)
        self.assertIn('GET', fixture.headers['Allow'])
        self.assertIn('read-only', fixture.body['message'].lower())

    def test_method_not_allowed_write_only(self):
        """Test 405 for write-only endpoint."""
        fixture = method_not_allowed_write_only(path="/api/deleted", method="GET")
        self.assertEqual(fixture.status_code, 405)
        self.assertNotIn('GET', fixture.headers['Allow'])
        self.assertIn('POST', fixture.headers['Allow'])
        self.assertIn('write-only', fixture.body['message'].lower())


class Test415UnsupportedMediaTypeFixtures(unittest.TestCase):
    """Test 415 Unsupported Media Type error fixtures."""

    def test_unsupported_media_type_basic(self):
        """Test basic 415 fixture."""
        fixture = unsupported_media_type_fixture(
            path="/api/users",
            content_type="application/xml",
            supported_types=["application/json"]
        )
        self.assertEqual(fixture.status_code, 415)
        self.assertEqual(fixture.body['error'], 'unsupported_media_type')
        self.assertEqual(fixture.body['details']['provided_type'], 'application/xml')
        self.assertEqual(fixture.body['details']['supported_types'], ['application/json'])

    def test_unsupported_media_type_json_only(self):
        """Test 415 for JSON-only endpoint."""
        fixture = unsupported_media_type_json_only(
            path="/api/users",
            provided_type="text/plain"
        )
        self.assertEqual(fixture.status_code, 415)
        self.assertEqual(fixture.body['details']['supported_types'], ['application/json'])
        self.assertIn('JSON', fixture.body['message'])

    def test_unsupported_media_type_missing(self):
        """Test 415 for missing Content-Type."""
        fixture = unsupported_media_type_missing(path="/api/upload")
        self.assertEqual(fixture.status_code, 415)
        self.assertIsNone(fixture.body['details']['provided_type'])
        self.assertIn('Content-Type', fixture.body['message'])


class Test500InternalServerErrorFixtures(unittest.TestCase):
    """Test 500 Internal Server Error fixtures."""

    def test_internal_server_error_basic(self):
        """Test basic 500 fixture."""
        fixture = internal_server_error_fixture(path="/api/users")
        self.assertEqual(fixture.status_code, 500)
        self.assertEqual(fixture.body['error'], 'internal_server_error')
        self.assertEqual(fixture.body['code'], 500)
        self.assertIn('error_id', fixture.body['details'])
        self.assertIn('X-Error-ID', fixture.headers)

    def test_internal_server_error_custom_message(self):
        """Test 500 with custom message."""
        fixture = internal_server_error_fixture(
            path="/api/test",
            message="Custom error message"
        )
        self.assertEqual(fixture.body['message'], 'Custom error message')

    def test_internal_server_error_custom_error_id(self):
        """Test 500 with custom error ID."""
        fixture = internal_server_error_fixture(
            path="/api/test",
            error_id="ERR-CUSTOM-123"
        )
        self.assertEqual(fixture.body['details']['error_id'], 'ERR-CUSTOM-123')
        self.assertEqual(fixture.headers['X-Error-ID'], 'ERR-CUSTOM-123')

    def test_internal_server_error_with_stack_trace(self):
        """Test 500 with stack trace included."""
        fixture = internal_server_error_fixture(
            path="/api/test",
            include_stack_trace=True
        )
        self.assertIn('stack_trace', fixture.body['details'])
        self.assertIsInstance(fixture.body['details']['stack_trace'], list)

    def test_internal_server_error_database(self):
        """Test 500 for database error."""
        fixture = internal_server_error_database(
            path="/api/users",
            db_error="Connection timeout"
        )
        self.assertEqual(fixture.status_code, 500)
        self.assertIn('Database', fixture.body['message'])

    def test_internal_server_error_external_service(self):
        """Test 500 for external service failure."""
        fixture = internal_server_error_external_service(
            path="/api/payments",
            service_name="Stripe API"
        )
        self.assertEqual(fixture.status_code, 500)
        self.assertIn('Stripe API', fixture.body['message'])


class TestGenericErrorBuilders(unittest.TestCase):
    """Test generic error response builders."""

    def test_create_error_response_basic(self):
        """Test basic custom error creation."""
        fixture = create_error_response(
            status=418,
            error_type="im_a_teapot",
            message="I'm a teapot",
            code=418
        )
        self.assertEqual(fixture.status_code, 418)
        self.assertEqual(fixture.body['error'], 'im_a_teapot')
        self.assertEqual(fixture.body['code'], 418)

    def test_create_error_response_with_details(self):
        """Test error creation with details."""
        fixture = create_error_response(
            status=403,
            error_type="forbidden",
            message="Access denied",
            details={'permission': 'admin', 'required': True}
        )
        self.assertEqual(fixture.status_code, 403)
        self.assertIn('details', fixture.body)
        self.assertEqual(fixture.body['details']['permission'], 'admin')

    def test_create_error_response_default_code(self):
        """Test that code defaults to status if not provided."""
        fixture = create_error_response(
            status=404,
            error_type="not_found",
            message="Not found"
        )
        self.assertEqual(fixture.body['code'], 404)

    def test_create_error_batch(self):
        """Test creating multiple error responses."""
        errors = create_error_batch([
            {'status': 404, 'error_type': 'not_found', 'message': 'Not found', 'path': '/api/1'},
            {'status': 403, 'error_type': 'forbidden', 'message': 'Forbidden'},
            {'status': 500, 'error_type': 'internal', 'message': 'Server error'}
        ])
        self.assertEqual(len(errors), 3)
        self.assertEqual(errors[0].status_code, 404)
        self.assertEqual(errors[1].status_code, 403)
        self.assertEqual(errors[2].status_code, 500)

    def test_create_error_batch_with_method(self):
        """Test batch creation with method in details."""
        errors = create_error_batch([
            {'status': 405, 'error_type': 'method_not_allowed', 'message': 'Wrong method', 'path': '/api/test', 'method': 'POST'}
        ])
        self.assertEqual(errors[0].body['details']['method'], 'POST')


class TestFixtureCollections(unittest.TestCase):
    """Test pre-configured fixture collections."""

    def test_common_error_fixtures(self):
        """Test COMMON_ERROR_FIXTURES collection."""
        self.assertIn('not_found', COMMON_ERROR_FIXTURES)
        self.assertIn('method_not_allowed', COMMON_ERROR_FIXTURES)
        self.assertIn('unsupported_media_type', COMMON_ERROR_FIXTURES)
        self.assertIn('internal_server_error', COMMON_ERROR_FIXTURES)

    def test_client_error_fixtures(self):
        """Test CLIENT_ERROR_FIXTURES collection."""
        self.assertIn('400_bad_request', CLIENT_ERROR_FIXTURES)
        self.assertIn('404_not_found', CLIENT_ERROR_FIXTURES)
        self.assertIn('415_unsupported_media_type', CLIENT_ERROR_FIXTURES)
        # All should be 4xx codes
        for name, fixture in CLIENT_ERROR_FIXTURES.items():
            self.assertGreaterEqual(fixture.status_code, 400)
            self.assertLess(fixture.status_code, 500)

    def test_server_error_fixtures(self):
        """Test SERVER_ERROR_FIXTURES collection."""
        self.assertIn('500_internal_server_error', SERVER_ERROR_FIXTURES)
        self.assertIn('502_bad_gateway', SERVER_ERROR_FIXTURES)
        # All should be 5xx codes
        for name, fixture in SERVER_ERROR_FIXTURES.items():
            self.assertGreaterEqual(fixture.status_code, 500)
            self.assertLess(fixture.status_code, 600)

    def test_all_error_fixtures(self):
        """Test ALL_ERROR_FIXTURES collection."""
        # Should contain both client and server errors
        self.assertGreater(len(ALL_ERROR_FIXTURES), len(CLIENT_ERROR_FIXTURES))
        self.assertIn('400_bad_request', ALL_ERROR_FIXTURES)
        self.assertIn('500_internal_server_error', ALL_ERROR_FIXTURES)

    def test_get_fixture(self):
        """Test retrieving fixtures by name."""
        # Test with fixture name from ALL_ERROR_FIXTURES collection
        fixture = get_fixture('404_not_found')
        self.assertIsNotNone(fixture)
        self.assertEqual(fixture.status_code, 404)

        fixture = get_fixture('500_internal_server_error')
        self.assertIsNotNone(fixture)
        self.assertEqual(fixture.status_code, 500)

        # Test with non-existent fixture
        fixture = get_fixture('non_existent')
        self.assertIsNone(fixture)

    def test_list_fixtures(self):
        """Test listing all available fixture names."""
        names = list_fixtures()
        self.assertIsInstance(names, list)
        self.assertIn('404_not_found', names)
        self.assertIn('500_internal_server_error', names)
        # Should be sorted
        self.assertEqual(names, sorted(names))


class TestFixtureValidation(unittest.TestCase):
    """Test that fixtures are valid according to validation helpers."""

    def test_404_fixtures_are_valid(self):
        """Validate 404 fixtures with standard error validation."""
        fixtures = [
            not_found_fixture(),
            not_found_simple(),
            not_found_with_suggestions()
        ]
        for fixture in fixtures:
            self.assertTrue(validate_standard_error_response(fixture.body))

    def test_405_fixtures_are_valid(self):
        """Validate 405 fixtures."""
        fixtures = [
            method_not_allowed_fixture(),
            method_not_allowed_read_only(),
            method_not_allowed_write_only()
        ]
        for fixture in fixtures:
            self.assertTrue(validate_standard_error_response(fixture.body))

    def test_415_fixtures_are_valid(self):
        """Validate 415 fixtures."""
        fixtures = [
            unsupported_media_type_fixture(),
            unsupported_media_type_json_only(),
            unsupported_media_type_missing()
        ]
        for fixture in fixtures:
            self.assertTrue(validate_standard_error_response(fixture.body))

    def test_500_fixtures_are_valid(self):
        """Validate 500 fixtures."""
        fixtures = [
            internal_server_error_fixture(),
            internal_server_error_database(),
            internal_server_error_external_service()
        ]
        for fixture in fixtures:
            self.assertTrue(validate_standard_error_response(fixture.body))

    def test_all_fixtures_have_valid_status(self):
        """Test all collection fixtures have valid status codes."""
        for name, fixture in ALL_ERROR_FIXTURES.items():
            # Validate status code (should be 4xx or 5xx)
            self.assertGreaterEqual(fixture.status_code, 400)
            self.assertLess(fixture.status_code, 600)

    def test_fixtures_with_http_status_validator(self):
        """Test fixtures work with HTTP status validator."""
        fixture = not_found_fixture()
        validate_http_status(fixture.to_tuple(), expected_status=404)

        fixture = internal_server_error_fixture()
        validate_http_status(fixture.to_tuple(), expected_status=500)


class TestFixtureConfigurability(unittest.TestCase):
    """Test that fixtures are easily configurable for different endpoints."""

    def test_404_configurable_path(self):
        """Test 404 path configurability."""
        paths = ['/api/users/1', '/api/items/2', '/api/orders/3']
        for path in paths:
            fixture = not_found_fixture(path=path)
            self.assertIn(path, fixture.body['details']['path'])

    def test_405_configurable_methods(self):
        """Test 405 method configurability."""
        test_cases = [
            ('/api/users', 'DELETE', ['GET', 'POST']),
            ('/api/items', 'PATCH', ['GET', 'PUT']),
            ('/api/orders', 'POST', ['GET'])
        ]
        for path, method, allowed in test_cases:
            fixture = method_not_allowed_fixture(
                path=path,
                method=method,
                allowed_methods=allowed
            )
            self.assertEqual(fixture.body['details']['method'], method)
            self.assertEqual(fixture.body['details']['allowed_methods'], allowed)

    def test_415_configurable_types(self):
        """Test 415 content type configurability."""
        test_cases = [
            ('/api/upload', 'text/xml', ['application/json']),
            ('/api/data', 'application/xml', ['application/json', 'application/yaml']),
            ('/api/import', 'text/plain', ['application/json'])
        ]
        for path, content_type, supported in test_cases:
            fixture = unsupported_media_type_fixture(
                path=path,
                content_type=content_type,
                supported_types=supported
            )
            self.assertEqual(fixture.body['details']['provided_type'], content_type)
            self.assertEqual(fixture.body['details']['supported_types'], supported)

    def test_500_configurable_paths(self):
        """Test 500 path configurability."""
        paths = ['/api/users', '/api/items', '/api/orders']
        for path in paths:
            fixture = internal_server_error_fixture(path=path)
            self.assertIn(path, fixture.body['details']['path'])


if __name__ == '__main__':
    unittest.main()
