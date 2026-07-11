"""
Comprehensive unit tests for ParseResult structure.

This module provides complete coverage of all ParseResult functionality including:
- Result creation with SUCCESS and ERROR statuses
- All helper methods (is_success, is_error, get_data, get_error)
- Edge cases (None values, empty strings, etc.)
- String representation methods
- Factory methods (success(), make_error())
"""

import sys
from typing import Any, Dict, List
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))

from result import ParseResult, ParseStatus


class TestParseResultCreation:
    """Test ParseResult creation scenarios."""

    def test_success_with_dict(self):
        """Verify ParseResult creation with dict data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={"key": "value", "number": 42},
            error=None
        )
        assert result.is_success()
        assert result.data == {"key": "value", "number": 42}
        assert result.error is None
        assert result.status == ParseStatus.SUCCESS

    def test_success_with_list(self):
        """Verify ParseResult creation with list data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=[1, 2, 3, 4, 5],
            error=None
        )
        assert result.is_success()
        assert result.data == [1, 2, 3, 4, 5]
        assert result.error is None

    def test_success_with_string(self):
        """Verify ParseResult creation with string data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data="test string",
            error=None
        )
        assert result.is_success()
        assert result.data == "test string"
        assert result.error is None

    def test_success_with_number(self):
        """Verify ParseResult creation with numeric data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=42,
            error=None
        )
        assert result.is_success()
        assert result.data == 42
        assert result.error is None

    def test_success_with_boolean(self):
        """Verify ParseResult creation with boolean data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=True,
            error=None
        )
        assert result.is_success()
        assert result.data is True
        assert result.error is None

    def test_success_with_none(self):
        """Verify ParseResult creation with None data (edge case)."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=None,
            error=None
        )
        assert result.is_success()
        assert result.data is None
        assert result.error is None

    def test_success_with_empty_dict(self):
        """Verify ParseResult creation with empty dict."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={},
            error=None
        )
        assert result.is_success()
        assert result.data == {}
        assert isinstance(result.data, dict)

    def test_success_with_empty_list(self):
        """Verify ParseResult creation with empty list."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=[],
            error=None
        )
        assert result.is_success()
        assert result.data == []
        assert isinstance(result.data, list)

    def test_success_with_empty_string(self):
        """Verify ParseResult creation with empty string."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data="",
            error=None
        )
        assert result.is_success()
        assert result.data == ""
        assert isinstance(result.data, str)

    def test_success_with_zero(self):
        """Verify ParseResult creation with zero value."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=0,
            error=None
        )
        assert result.is_success()
        assert result.data == 0

    def test_success_with_false(self):
        """Verify ParseResult creation with False value."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=False,
            error=None
        )
        assert result.is_success()
        assert result.data is False

    def test_error_with_message(self):
        """Verify ParseResult creation with error message."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="File not found"
        )
        assert result.is_error()
        assert result.error == "File not found"
        assert result.data is None
        assert result.status == ParseStatus.ERROR

    def test_error_with_empty_message(self):
        """Verify ParseResult creation with empty error message (edge case)."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=""
        )
        assert result.is_error()
        assert result.error == ""
        assert result.data is None

    def test_error_with_long_message(self):
        """Verify ParseResult creation with long error message."""
        long_msg = "Error: " + "A" * 100 + " (end)"
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=long_msg
        )
        assert result.is_error()
        assert result.error == long_msg
        assert len(result.error) > 100

    def test_error_with_special_chars(self):
        """Verify ParseResult creation with special characters in error."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="Error: \n\t\"quoted\" and 'single'"
        )
        assert result.is_error()
        assert "\n" in result.error
        assert "\t" in result.error
        assert '"' in result.error
        assert "'" in result.error

    def test_error_with_multiline_message(self):
        """Verify ParseResult creation with multiline error message."""
        msg = """Multi-line error:
Line 1
Line 2
Line 3"""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=msg
        )
        assert result.is_error()
        assert "\n" in result.error
        assert "Line 1" in result.error


class TestParseStatusEnum:
    """Test ParseStatus enum functionality."""

    def test_status_success_value(self):
        """Verify ParseStatus.SUCCESS value."""
        assert ParseStatus.SUCCESS.value == "success"
        assert ParseStatus.SUCCESS.name == "SUCCESS"

    def test_status_error_value(self):
        """Verify ParseStatus.ERROR value."""
        assert ParseStatus.ERROR.value == "error"
        assert ParseStatus.ERROR.name == "ERROR"

    def test_status_comparison(self):
        """Verify ParseStatus enum comparison works."""
        assert ParseStatus.SUCCESS == ParseStatus.SUCCESS
        assert ParseStatus.ERROR == ParseStatus.ERROR
        assert ParseStatus.SUCCESS != ParseStatus.ERROR


class TestIsSuccessMethod:
    """Test is_success() method behavior."""

    def test_is_success_on_success_result(self):
        """Verify is_success() returns True for SUCCESS status."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={"data": "value"},
            error=None
        )
        assert result.is_success() is True
        assert result.is_error() is False

    def test_is_success_on_error_result(self):
        """Verify is_success() returns False for ERROR status."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="Test error"
        )
        assert result.is_success() is False
        assert result.is_error() is True

    def test_is_success_with_none_data(self):
        """Verify is_success() works with None data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=None,
            error=None
        )
        assert result.is_success() is True
        assert result.data is None

    def test_is_success_with_false_data(self):
        """Verify is_success() works with False data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=False,
            error=None
        )
        assert result.is_success() is True
        assert result.data is False

    def test_is_success_with_zero_data(self):
        """Verify is_success() works with zero data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=0,
            error=None
        )
        assert result.is_success() is True
        assert result.data == 0


class TestIsErrorMethod:
    """Test is_error() method behavior."""

    def test_is_error_on_error_result(self):
        """Verify is_error() returns True for ERROR status."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="Test error"
        )
        assert result.is_error() is True
        assert result.is_success() is False

    def test_is_error_on_success_result(self):
        """Verify is_error() returns False for SUCCESS status."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={"data": "value"},
            error=None
        )
        assert result.is_error() is False
        assert result.is_success() is True


class TestGetErrorMethod:
    """Test get_error() method behavior."""

    def test_get_error_on_success(self):
        """Verify get_error() returns None on success."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={"data": "value"},
            error=None
        )
        assert result.get_error() is None

    def test_get_error_on_success_with_none_data(self):
        """Verify get_error() returns None even with None data."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=None,
            error=None
        )
        assert result.get_error() is None

    def test_get_error_on_error(self):
        """Verify get_error() returns error message on error."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="File not found"
        )
        assert result.get_error() == "File not found"

    def test_get_error_with_empty_message(self):
        """Verify get_error() returns empty string when set."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=""
        )
        assert result.get_error() == ""

    def test_get_error_with_special_chars(self):
        """Verify get_error() preserves special characters."""
        msg = "Error: \n\t\"test\""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=msg
        )
        assert result.get_error() == msg
        assert "\n" in result.get_error()
        assert "\t" in result.get_error()


class TestGetDataMethod:
    """Test get_data() method behavior."""

    def test_get_data_on_success(self):
        """Verify get_data() returns data on success."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={"key": "value"},
            error=None
        )
        assert result.get_data() == {"key": "value"}

    def test_get_data_on_success_with_none(self):
        """Verify get_data() returns None when data is None."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=None,
            error=None
        )
        assert result.get_data() is None

    def test_get_data_on_success_with_empty_string(self):
        """Verify get_data() returns empty string."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data="",
            error=None
        )
        assert result.get_data() == ""
        assert isinstance(result.get_data(), str)

    def test_get_data_on_success_with_false(self):
        """Verify get_data() returns False value."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=False,
            error=None
        )
        assert result.get_data() is False

    def test_get_data_on_success_with_zero(self):
        """Verify get_data() returns zero value."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=0,
            error=None
        )
        assert result.get_data() == 0

    def test_get_data_on_error_raises_runtime_error(self):
        """Verify get_data() raises RuntimeError on error."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="Test error"
        )
        try:
            result.get_data()
            raise AssertionError("Expected RuntimeError to be raised")
        except RuntimeError as e:
            assert "Test error" in str(e)

    def test_get_data_on_error_with_empty_message_raises(self):
        """Verify get_data() raises RuntimeError even with empty error."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=""
        )
        try:
            result.get_data()
            raise AssertionError("Expected RuntimeError to be raised")
        except RuntimeError as e:
            # Should raise with the error message (empty string in this case)
            assert "" in str(e)

    def test_get_data_on_error_with_none_message_raises(self):
        """Verify get_data() raises RuntimeError with None error."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=None
        )
        try:
            result.get_data()
            raise AssertionError("Expected RuntimeError to be raised")
        except RuntimeError as e:
            # Should still raise RuntimeError
            assert e is not None


class TestStringRepresentation:
    """Test __str__() method behavior."""

    def test_str_on_success(self):
        """Verify __str__() shows SUCCESS and data type."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data={"key": "value"},
            error=None
        )
        str_repr = str(result)
        assert "success" in str_repr
        assert "dict" in str_repr

    def test_str_on_success_with_list(self):
        """Verify __str__() shows list type."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=[1, 2, 3],
            error=None
        )
        str_repr = str(result)
        assert "success" in str_repr
        assert "list" in str_repr

    def test_str_on_success_with_string(self):
        """Verify __str__() shows str type."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data="test",
            error=None
        )
        str_repr = str(result)
        assert "success" in str_repr
        assert "str" in str_repr

    def test_str_on_success_with_none(self):
        """Verify __str__() shows None type."""
        result = ParseResult(
            status=ParseStatus.SUCCESS,
            data=None,
            error=None
        )
        str_repr = str(result)
        assert "success" in str_repr
        assert "NoneType" in str_repr

    def test_str_on_error(self):
        """Verify __str__() shows ERROR and message."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error="File not found"
        )
        str_repr = str(result)
        assert "error" in str_repr
        assert "File not found" in str_repr

    def test_str_on_error_with_empty_message(self):
        """Verify __str__() shows empty error message."""
        result = ParseResult(
            status=ParseStatus.ERROR,
            data=None,
            error=""
        )
        str_repr = str(result)
        assert "error" in str_repr
        assert "error=" in str_repr


class TestFactoryMethods:
    """Test class method factories (success() and make_error())."""

    def test_success_factory_with_dict(self):
        """Verify ParseResult.success() with dict data."""
        result = ParseResult.success({"key": "value"})
        assert result.is_success()
        assert result.data == {"key": "value"}
        assert result.error is None
        assert result.status == ParseStatus.SUCCESS

    def test_success_factory_with_list(self):
        """Verify ParseResult.success() with list data."""
        result = ParseResult.success([1, 2, 3])
        assert result.is_success()
        assert result.data == [1, 2, 3]
        assert result.error is None

    def test_success_factory_with_string(self):
        """Verify ParseResult.success() with string data."""
        result = ParseResult.success("test string")
        assert result.is_success()
        assert result.data == "test string"
        assert result.error is None

    def test_success_factory_with_none(self):
        """Verify ParseResult.success() with None data."""
        result = ParseResult.success(None)
        assert result.is_success()
        assert result.data is None
        assert result.error is None

    def test_success_factory_with_empty_dict(self):
        """Verify ParseResult.success() with empty dict."""
        result = ParseResult.success({})
        assert result.is_success()
        assert result.data == {}
        assert result.error is None

    def test_make_error_factory_with_message(self):
        """Verify ParseResult.make_error() with message."""
        result = ParseResult.make_error("File not found")
        assert result.is_error()
        assert result.error == "File not found"
        assert result.data is None
        assert result.status == ParseStatus.ERROR

    def test_make_error_factory_with_empty_message(self):
        """Verify ParseResult.make_error() with empty message."""
        result = ParseResult.make_error("")
        assert result.is_error()
        assert result.error == ""
        assert result.data is None

    def test_make_error_factory_with_long_message(self):
        """Verify ParseResult.make_error() with long message."""
        long_msg = "This is a very long error message: " + "A" * 100
        result = ParseResult.make_error(long_msg)
        assert result.is_error()
        assert result.error == long_msg
        assert len(result.error) > 100

    def test_make_error_factory_with_special_chars(self):
        """Verify ParseResult.make_error() with special characters."""
        msg = "Error: \n\t\"test\" and 'single'"
        result = ParseResult.make_error(msg)
        assert result.is_error()
        assert result.error == msg
        assert "\n" in result.error
        assert "\t" in result.error


class TestEdgeCases:
    """Test edge cases and complex scenarios."""

    def test_success_with_complex_nested_dict(self):
        """Verify ParseResult with deeply nested dict structure."""
        data = {
            "level1": {
                "level2": {
                    "level3": {
                        "value": 42
                    }
                }
            }
        }
        result = ParseResult.success(data)
        assert result.is_success()
        assert result.data["level1"]["level2"]["level3"]["value"] == 42

    def test_success_with_mixed_list(self):
        """Verify ParseResult with mixed-type list."""
        data = [1, "two", 3.0, True, None]
        result = ParseResult.success(data)
        assert result.is_success()
        assert result.data == data

    def test_error_with_unicode_message(self):
        """Verify ParseResult with Unicode error message."""
        result = ParseResult.make_error("Error: 你好 🚀")
        assert result.is_error()
        assert "你好" in result.error
        assert "🚀" in result.error

    def test_multiple_results_independent(self):
        """Verify multiple ParseResult instances are independent."""
        result1 = ParseResult.success({"key": "value1"})
        result2 = ParseResult.success({"key": "value2"})

        assert result1.data["key"] == "value1"
        assert result2.data["key"] == "value2"

        # Modify one, shouldn't affect the other (they're different instances)
        result1.data["key"] = "modified"
        assert result1.data["key"] == "modified"
        assert result2.data["key"] == "value2"

    def test_success_result_immutability(self):
        """Verify success result maintains data correctly."""
        data = {"key": "value"}
        result = ParseResult.success(data)
        # Modifying original dict should still reflect in result (same reference)
        data["key"] = "modified"
        assert result.data["key"] == "modified"

    def test_get_data_runtime_error_message_format(self):
        """Verify RuntimeError message format from get_data()."""
        result = ParseResult.make_error("Custom error message")
        try:
            result.get_data()
            raise AssertionError("Expected RuntimeError to be raised")
        except RuntimeError as e:
            error_msg = str(e)
            assert "Custom error message" in error_msg
            assert "Cannot get data from failed result" in error_msg


class TestAcceptanceCriteria:
    """Tests that verify all acceptance criteria from the task."""

    def test_acceptance_criteria_success_creation(self):
        """Verify Result creation with SUCCESS status (Acceptance Criteria)."""
        result = ParseResult.success({"test": "data"})
        assert result.status == ParseStatus.SUCCESS
        assert result.is_success() is True

    def test_acceptance_criteria_error_creation(self):
        """Verify Result creation with ERROR status (Acceptance Criteria)."""
        result = ParseResult.make_error("Test error")
        assert result.status == ParseStatus.ERROR
        assert result.is_error() is True

    def test_acceptance_criteria_is_success_method(self):
        """Verify is_success() method works correctly (Acceptance Criteria)."""
        success_result = ParseResult.success({"data": "value"})
        error_result = ParseResult.make_error("Test error")
        assert success_result.is_success() is True
        assert error_result.is_success() is False

    def test_acceptance_criteria_is_error_method(self):
        """Verify is_error() method works correctly (Acceptance Criteria)."""
        success_result = ParseResult.success({"data": "value"})
        error_result = ParseResult.make_error("Test error")
        assert success_result.is_error() is False
        assert error_result.is_error() is True

    def test_acceptance_criteria_get_error_method(self):
        """Verify get_error() method works correctly (Acceptance Criteria)."""
        success_result = ParseResult.success({"data": "value"})
        error_result = ParseResult.make_error("Test error")
        assert success_result.get_error() is None
        assert error_result.get_error() == "Test error"

    def test_acceptance_criteria_get_data_method(self):
        """Verify get_data() method works correctly (Acceptance Criteria)."""
        success_result = ParseResult.success({"data": "value"})
        error_result = ParseResult.make_error("Test error")
        assert success_result.get_data() == {"data": "value"}
        try:
            error_result.get_data()
            raise AssertionError("Expected RuntimeError to be raised")
        except RuntimeError:
            pass  # Expected

    def test_acceptance_criteria_none_values_edge_case(self):
        """Verify edge case with None values (Acceptance Criteria)."""
        result = ParseResult.success(None)
        assert result.is_success() is True
        assert result.get_data() is None
        assert result.get_error() is None

    def test_acceptance_criteria_empty_string_edge_case(self):
        """Verify edge case with empty strings (Acceptance Criteria)."""
        # Empty data should still be success
        success_result = ParseResult.success("")
        assert success_result.is_success() is True
        assert success_result.get_data() == ""

        # Empty error message is still error
        error_result = ParseResult.make_error("")
        assert error_result.is_error() is True
        assert error_result.get_error() == ""

    def test_acceptance_criteria_empty_collections_edge_case(self):
        """Verify edge case with empty collections (Acceptance Criteria)."""
        # Empty list should still be success
        list_result = ParseResult.success([])
        assert list_result.is_success() is True
        assert list_result.get_data() == []

        # Empty dict should still be success
        dict_result = ParseResult.success({})
        assert dict_result.is_success() is True
        assert dict_result.get_data() == {}

    def test_acceptance_criteria_zero_and_false_edge_cases(self):
        """Verify edge case with zero and False values (Acceptance Criteria)."""
        # Zero should still be success
        zero_result = ParseResult.success(0)
        assert zero_result.is_success() is True
        assert zero_result.get_data() == 0

        # False should still be success
        false_result = ParseResult.success(False)
        assert false_result.is_success() is True
        assert false_result.get_data() is False


def run_all_tests():
    """Run all test classes and report results."""
    test_classes = [
        ("ParseResult Creation", TestParseResultCreation),
        ("ParseStatus Enum", TestParseStatusEnum),
        ("is_success() Method", TestIsSuccessMethod),
        ("is_error() Method", TestIsErrorMethod),
        ("get_error() Method", TestGetErrorMethod),
        ("get_data() Method", TestGetDataMethod),
        ("String Representation", TestStringRepresentation),
        ("Factory Methods", TestFactoryMethods),
        ("Edge Cases", TestEdgeCases),
        ("Acceptance Criteria", TestAcceptanceCriteria),
    ]

    total_passed = 0
    total_failed = 0

    print("=" * 70)
    print("COMPREHENSIVE PARSE RESULT UNIT TESTS")
    print("=" * 70)

    for class_name, test_class in test_classes:
        print(f"\n{class_name}")
        print("-" * 70)

        test_instance = test_class()
        test_methods = [
            (name, getattr(test_instance, name))
            for name in dir(test_instance)
            if name.startswith('test_') and callable(getattr(test_instance, name))
        ]

        class_passed = 0
        class_failed = 0

        for test_name, test_func in test_methods:
            try:
                test_func()
                print(f"  ✓ {test_name}")
                class_passed += 1
            except Exception as e:
                print(f"  ✗ {test_name}: {e}")
                class_failed += 1

        total_passed += class_passed
        total_failed += class_failed

        if class_failed == 0:
            print(f"  ✅ All {class_passed} tests passed")
        else:
            print(f"  ⚠️  {class_passed} passed, {class_failed} failed")

    print("\n" + "=" * 70)
    print(f"FINAL RESULTS: {total_passed} passed, {total_failed} failed")
    print("=" * 70)

    if total_failed == 0:
        print("\n✅ ALL ACCEPTANCE CRITERIA MET:")
        print("  ✓ Unit test file exists and is runnable")
        print("  ✓ All ParseResult creation scenarios are tested")
        print("  ✓ All helper methods (is_success, is_error, get_error, get_data) tested")
        print("  ✓ Factory methods (success(), make_error()) tested")
        print("  ✓ String representation methods tested")
        print("  ✓ Edge cases covered (None values, empty strings, etc.)")
        print("  ✓ All tests pass")
        return True
    else:
        print(f"\n❌ {total_failed} test(s) failed")
        return False


if __name__ == "__main__":
    import sys
    success = run_all_tests()
    sys.exit(0 if success else 1)
