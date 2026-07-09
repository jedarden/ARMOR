#!/usr/bin/env python3
"""
Standalone unit tests for ParseResult structure (no pytest dependency).
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from result import ParseResult, ParseStatus


def test_success_result_creation():
    """Test creating a success result."""
    result = ParseResult(status=ParseStatus.SUCCESS, data={'key': 'value'})
    assert result.status == ParseStatus.SUCCESS
    assert result.data == {'key': 'value'}
    assert result.error is None
    assert result.is_success()
    assert not result.is_error()
    print("✓ test_success_result_creation passed")


def test_error_result_creation():
    """Test creating an error result."""
    result = ParseResult(status=ParseStatus.ERROR, error='Test error')
    assert result.status == ParseStatus.ERROR
    assert result.data is None
    assert result.error == 'Test error'
    assert not result.is_success()
    assert result.is_error()
    print("✓ test_error_result_creation passed")


def test_is_success_method():
    """Test is_success method."""
    success_result = ParseResult(status=ParseStatus.SUCCESS, data={})
    error_result = ParseResult(status=ParseStatus.ERROR, error='Failed')

    assert success_result.is_success() is True
    assert success_result.is_error() is False
    assert error_result.is_success() is False
    assert error_result.is_error() is True
    print("✓ test_is_success_method passed")


def test_is_error_method():
    """Test is_error method."""
    success_result = ParseResult.success({'data': 'value'})
    error_result = ParseResult.make_error('Error message')

    assert success_result.is_error() is False
    assert error_result.is_error() is True
    print("✓ test_is_error_method passed")


def test_get_data_success():
    """Test get_data returns data when successful."""
    result = ParseResult.success({'key': 'value'})
    data = result.get_data()
    assert data == {'key': 'value'}
    print("✓ test_get_data_success passed")


def test_get_data_failure():
    """Test get_data raises RuntimeError when failed."""
    result = ParseResult.make_error('Test error')
    try:
        result.get_data()
        print("✗ test_get_data_failure failed - should have raised RuntimeError")
        sys.exit(1)
    except RuntimeError as e:
        assert 'Test error' in str(e)
        print("✓ test_get_data_failure passed")


def test_get_error():
    """Test get_error returns error message."""
    error_result = ParseResult.make_error('Test error message')
    assert error_result.get_error() == 'Test error message'

    success_result = ParseResult.success({'data': 'value'})
    assert success_result.get_error() is None
    print("✓ test_get_error passed")


def test_success_factory_method():
    """Test ParseResult.success() factory method."""
    result = ParseResult.success({'key': 'value', 'number': 42})
    assert result.status == ParseStatus.SUCCESS
    assert result.data == {'key': 'value', 'number': 42}
    assert result.error is None
    assert result.is_success()
    print("✓ test_success_factory_method passed")


def test_make_error_factory_method():
    """Test ParseResult.make_error() factory method."""
    result = ParseResult.make_error('File not found: /path/to/file.yaml')
    assert result.status == ParseStatus.ERROR
    assert result.data is None
    assert result.error == 'File not found: /path/to/file.yaml'
    assert result.is_error()
    print("✓ test_make_error_factory_method passed")


def test_make_error_method():
    """Test ParseResult.make_error() method."""
    result = ParseResult.make_error('Test error via make_error')
    assert result.status == ParseStatus.ERROR
    assert result.data is None
    assert result.error == 'Test error via make_error'
    assert result.is_error()
    print("✓ test_make_error_method passed")


def test_string_representation():
    """Test string representation of results."""
    success_result = ParseResult.success({'data': 'value'})
    error_result = ParseResult.make_error('Test error')

    success_str = str(success_result)
    error_str = str(error_result)

    assert 'success' in success_str.lower()
    assert 'error' in error_str.lower()
    assert 'Test error' in error_str
    print("✓ test_string_representation passed")


def test_parse_status_enum():
    """Test ParseStatus enum values."""
    assert ParseStatus.SUCCESS.value == 'success'
    assert ParseStatus.ERROR.value == 'error'
    print("✓ test_parse_status_enum passed")


def test_all_fields_present():
    """Test that all required fields are present."""
    success_result = ParseResult.success({'test': 'data'})
    error_result = ParseResult.make_error('Test error')

    # Check success result fields
    assert hasattr(success_result, 'status')
    assert hasattr(success_result, 'data')
    assert hasattr(success_result, 'error')
    assert hasattr(success_result, 'is_success')
    assert hasattr(success_result, 'is_error')
    assert hasattr(success_result, 'get_data')
    assert hasattr(success_result, 'get_error')

    # Check error result fields
    assert hasattr(error_result, 'status')
    assert hasattr(error_result, 'data')
    assert hasattr(error_result, 'error')
    print("✓ test_all_fields_present passed")


def main():
    """Run all tests."""
    print("=" * 60)
    print("ParseResult Unit Tests")
    print("=" * 60)

    tests = [
        test_success_result_creation,
        test_error_result_creation,
        test_is_success_method,
        test_is_error_method,
        test_get_data_success,
        test_get_data_failure,
        test_get_error,
        test_success_factory_method,
        test_make_error_factory_method,
        test_make_error_method,
        test_string_representation,
        test_parse_status_enum,
        test_all_fields_present,
    ]

    failed = 0
    for test in tests:
        try:
            test()
        except Exception as e:
            print(f"✗ {test.__name__} failed: {e}")
            failed += 1

    print("=" * 60)
    if failed == 0:
        print(f"✓ All {len(tests)} tests PASSED")
        print("=" * 60)
        return 0
    else:
        print(f"✗ {failed} of {len(tests)} tests FAILED")
        print("=" * 60)
        return 1


if __name__ == '__main__':
    sys.exit(main())
