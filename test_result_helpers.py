#!/usr/bin/env python3
"""
Test Result helper methods against acceptance criteria.

Acceptance Criteria:
- is_success() returns True for SUCCESS status
- is_error() returns True for ERROR status
- get_error() returns error message or None
- get_data() returns data or default value
- All methods handle edge cases properly
"""

import sys
from pathlib import Path

# Add internal directory to path
sys.path.insert(0, str(Path(__file__).parent / 'internal'))

from yamlutil.result_types import Result, Status


def test_is_success():
    """Test is_success() returns True for SUCCESS status."""
    success_result = Result.success({"key": "value"})
    error_result = Result.error("Test error")

    assert success_result.is_success() is True, "is_success() should return True for SUCCESS"
    assert error_result.is_success() is False, "is_success() should return False for ERROR"
    print("✓ is_success() returns True for SUCCESS status")


def test_is_error():
    """Test is_error() returns True for ERROR status."""
    success_result = Result.success({"key": "value"})
    error_result = Result.error("Test error")

    assert success_result.is_error() is False, "is_error() should return False for SUCCESS"
    assert error_result.is_error() is True, "is_error() should return True for ERROR"
    print("✓ is_error() returns True for ERROR status")


def test_get_error():
    """Test get_error() returns error message or None."""
    success_result = Result.success({"key": "value"})
    error_result = Result.error("Test error message")

    assert success_result.get_error() is None, "get_error() should return None for SUCCESS"
    assert error_result.get_error() == "Test error message", "get_error() should return error message"
    print("✓ get_error() returns error message or None")


def test_get_data_with_default():
    """Test get_data() returns data or default value."""
    success_result = Result.success({"key": "value"})
    error_result = Result.error("Test error")

    # Test getting data from success result
    assert success_result.get_data() == {"key": "value"}, "get_data() should return data on success"

    # Test getting default from error result
    assert error_result.get_data() is None, "get_data() with no default should return None on error"
    assert error_result.get_data({}) == {}, "get_data() with default should return default on error"
    assert error_result.get_data_or([]) == [], "get_data_or() should return default on error"

    # Test with custom default
    default_value = {"default": "data"}
    assert error_result.get_data(default_value) == default_value, "get_data() should return provided default"

    print("✓ get_data() returns data or default value")


def test_edge_cases():
    """Test edge cases."""
    # Test with None data
    result = Result.success(None)
    assert result.is_success(), "Result with None data should still be success"
    assert result.get_data() is None, "get_data() should return None when data is None"

    # Test with empty string data
    result = Result.success("")
    assert result.is_success(), "Result with empty string should be success"
    assert result.get_data() == "", "get_data() should return empty string"

    # Test with empty list data
    result = Result.success([])
    assert result.is_success(), "Result with empty list should be success"
    assert result.get_data() == [], "get_data() should return empty list"

    # Test with empty dict data
    result = Result.success({})
    assert result.is_success(), "Result with empty dict should be success"
    assert result.get_data() == {}, "get_data() should return empty dict"

    # Test get_data_or with success result
    result = Result.success({"actual": "data"})
    assert result.get_data_or({"default": "value"}) == {"actual": "data"}, \
        "get_data_or() should return actual data on success, not default"

    print("✓ All methods handle edge cases properly")


def test_unwrap():
    """Test unwrap() method for completeness."""
    success_result = Result.success({"key": "value"})
    error_result = Result.error("Test error")

    assert success_result.unwrap() == {"key": "value"}, "unwrap() should return data on success"

    try:
        error_result.unwrap()
        assert False, "unwrap() should raise ValueError on error"
    except ValueError as e:
        assert str(e) == "Test error", "unwrap() should raise error with message"
        print("✓ unwrap() raises ValueError on error as expected")


if __name__ == '__main__':
    print("\n" + "="*60)
    print("RESULT HELPER METHODS VERIFICATION")
    print("="*60 + "\n")

    tests = [
        ("is_success() method", test_is_success),
        ("is_error() method", test_is_error),
        ("get_error() method", test_get_error),
        ("get_data() with default", test_get_data_with_default),
        ("Edge cases handling", test_edge_cases),
        ("unwrap() method", test_unwrap),
    ]

    all_passed = True
    for name, test_func in tests:
        try:
            print(f"Testing: {name}")
            test_func()
            print()
        except AssertionError as e:
            print(f"✗ FAILED: {e}\n")
            all_passed = False
        except Exception as e:
            print(f"✗ CRASHED: {e}\n")
            all_passed = False

    print("="*60)
    if all_passed:
        print("✓ ALL ACCEPTANCE CRITERIA VERIFIED!")
        sys.exit(0)
    else:
        print("✗ SOME TESTS FAILED")
        sys.exit(1)
