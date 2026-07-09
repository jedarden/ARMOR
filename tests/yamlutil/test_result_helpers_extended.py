"""
Extended tests for Result helper methods (bf-50670).

This module tests the convenience methods added to the Result class.
"""

from typing import Dict, Any
from internal.yamlutil import Result, Status


class TestResultHelpersExtended:
    """Test extended Result helper methods."""

    def test_get_data_or_on_success(self):
        """Verify get_data_or returns data on success."""
        result = Result.success({"key": "value"})
        data = result.get_data_or({})
        assert data == {"key": "value"}

    def test_get_data_or_on_error(self):
        """Verify get_data_or returns default on error."""
        result = Result.error("Failed")
        data = result.get_data_or({})
        assert data == {}

    def test_get_data_or_with_none_default(self):
        """Verify get_data_or works with None as default."""
        result = Result.error("Failed")
        data = result.get_data_or(None)
        assert data is None

    def test_get_data_on_success_with_none_data(self):
        """Verify get_data handles None data on success (edge case)."""
        # This is valid - success(None) means operation succeeded but returned None
        result = Result.success(None)
        data = result.get_data()
        assert data is None  # Should return None, not raise

    def test_get_data_on_success_with_data(self):
        """Verify get_data returns data on success."""
        result = Result.success({"key": "value"})
        data = result.get_data()
        assert data == {"key": "value"}

    def test_get_data_on_error_returns_none(self):
        """Verify get_data returns None on error when no default provided."""
        result = Result.error("Test error")
        data = result.get_data()
        assert data is None

    def test_get_data_on_error_with_default(self):
        """Verify get_data returns default value on error."""
        result = Result.error("Test error")
        data = result.get_data({})
        assert data == {}
        assert isinstance(data, dict)

    def test_get_data_on_error_with_custom_default(self):
        """Verify get_data returns custom default value on error."""
        result = Result.error("Test error")
        data = result.get_data({"default": "value"})
        assert data == {"default": "value"}

    def test_get_error_on_success(self):
        """Verify get_error returns None on success."""
        result = Result.success({"key": "value"})
        error = result.get_error()
        assert error is None

    def test_get_error_on_error(self):
        """Verify get_error returns error message on error."""
        result = Result.error("File not found")
        error = result.get_error()
        assert error == "File not found"

    def test_bool_on_success(self):
        """Verify __bool__ returns True on success."""
        result = Result.success({"data": "value"})
        assert bool(result) is True
        assert result.is_success() is True

    def test_bool_on_error(self):
        """Verify __bool__ returns False on error."""
        result = Result.error("Test error")
        assert bool(result) is False
        assert result.is_error() is True

    def test_string_representation_success(self):
        """Verify __str__ includes SUCCESS and data type."""
        result = Result.success({"key": "value"})
        str_repr = str(result)
        assert "SUCCESS" in str_repr
        assert "data" in str_repr
        assert "dict" in str_repr

    def test_string_representation_error(self):
        """Verify __str__ includes ERROR and error message."""
        result = Result.error("Test error")
        str_repr = str(result)
        assert "ERROR" in str_repr
        assert "Test error" in str_repr


if __name__ == "__main__":
    # Run tests
    test = TestResultHelpersExtended()

    print("Running extended Result helper tests...")

    tests = [
        ("get_data_or on success returns data", test.test_get_data_or_on_success),
        ("get_data_or on error returns default", test.test_get_data_or_on_error),
        ("get_data_or with None default", test.test_get_data_or_with_none_default),
        ("get_data handles None data on success", test.test_get_data_on_success_with_none_data),
        ("get_data returns data on success", test.test_get_data_on_success_with_data),
        ("get_data returns None on error without default", test.test_get_data_on_error_returns_none),
        ("get_data returns default on error with default", test.test_get_data_on_error_with_default),
        ("get_data returns custom default on error", test.test_get_data_on_error_with_custom_default),
        ("get_error returns None on success", test.test_get_error_on_success),
        ("get_error returns message on error", test.test_get_error_on_error),
        ("__bool__ returns True on success", test.test_bool_on_success),
        ("__bool__ returns False on error", test.test_bool_on_error),
        ("__str__ includes SUCCESS and data type", test.test_string_representation_success),
        ("__str__ includes ERROR and message", test.test_string_representation_error),
    ]

    passed = 0
    failed = 0

    for name, test_func in tests:
        try:
            test_func()
            print(f"✓ {name}")
            passed += 1
        except Exception as e:
            print(f"✗ {name}: {e}")
            failed += 1

    print(f"\n{'='*60}")
    print(f"Results: {passed} passed, {failed} failed")
    print(f"{'='*60}")

    if failed == 0:
        print("\n✅ All acceptance criteria met for bead bf-50670:")
        print("  ✓ is_success() method exists and works correctly")
        print("  ✓ get_error() method returns error message on ERROR status, None otherwise")
        print("  ✓ get_data() method returns data on SUCCESS, or default value on ERROR")
        print("  ✓ Methods handle edge cases (data=None on success, defaults on error)")
        print("\n✅ All acceptance criteria met for bead bf-l3evl:")
        print("  ✓ is_success() returns True for SUCCESS status")
        print("  ✓ is_error() returns True for ERROR status")
        print("  ✓ get_error() returns error message or None")
        print("  ✓ get_data() returns data or default value (with optional default)")
        print("  ✓ All methods handle edge cases properly")
