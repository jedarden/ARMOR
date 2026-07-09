"""
Comprehensive unit tests for Result structure (bf-3prra).

This module provides complete coverage of all Result functionality including:
- Result creation with SUCCESS and ERROR statuses
- All helper methods (is_success, get_error, get_data, get_data_or)
- Advanced methods (map, and_then)
- Status enum methods (from_bool, as_bool)
- Edge cases and complex scenarios
"""

from typing import Dict, Any, List, Optional
from internal.yamlutil import Result, Status


class TestResultCreation:
    """Test Result creation scenarios."""

    def test_success_with_dict(self):
        """Verify Result creation with dict data."""
        result = Result.success({"key": "value", "number": 42})
        assert result.is_success()
        assert result.data == {"key": "value", "number": 42}
        assert result.error is None
        assert result.status == Status.SUCCESS

    def test_success_with_list(self):
        """Verify Result creation with list data."""
        result = Result.success([1, 2, 3, 4, 5])
        assert result.is_success()
        assert result.data == [1, 2, 3, 4, 5]
        assert result.error is None

    def test_success_with_string(self):
        """Verify Result creation with string data."""
        result = Result.success("test string")
        assert result.is_success()
        assert result.data == "test string"
        assert result.error is None

    def test_success_with_number(self):
        """Verify Result creation with numeric data."""
        result = Result.success(42)
        assert result.is_success()
        assert result.data == 42
        assert result.error is None

    def test_success_with_boolean(self):
        """Verify Result creation with boolean data."""
        result = Result.success(True)
        assert result.is_success()
        assert result.data is True
        assert result.error is None

    def test_success_with_none(self):
        """Verify Result creation with None data (edge case)."""
        result = Result.success(None)
        assert result.is_success()
        assert result.data is None
        assert result.error is None

    def test_success_with_empty_dict(self):
        """Verify Result creation with empty dict."""
        result = Result.success({})
        assert result.is_success()
        assert result.data == {}
        assert isinstance(result.data, dict)

    def test_success_with_empty_list(self):
        """Verify Result creation with empty list."""
        result = Result.success([])
        assert result.is_success()
        assert result.data == []
        assert isinstance(result.data, list)

    def test_success_with_empty_string(self):
        """Verify Result creation with empty string."""
        result = Result.success("")
        assert result.is_success()
        assert result.data == ""
        assert isinstance(result.data, str)

    def test_success_with_zero(self):
        """Verify Result creation with zero value."""
        result = Result.success(0)
        assert result.is_success()
        assert result.data == 0

    def test_success_with_false(self):
        """Verify Result creation with False value."""
        result = Result.success(False)
        assert result.is_success()
        assert result.data is False
        assert bool(result) is True  # Result itself is truthy

    def test_error_with_message(self):
        """Verify Result creation with error message."""
        result = Result.error("File not found")
        assert result.is_error()
        assert result.error == "File not found"
        assert result.data is None
        assert result.status == Status.ERROR

    def test_error_with_empty_message(self):
        """Verify Result creation with empty error message (edge case)."""
        result = Result.error("")
        assert result.is_error()
        assert result.error == ""
        assert result.data is None

    def test_error_with_long_message(self):
        """Verify Result creation with long error message."""
        long_msg = "Error: " + "A" * 100 + " (end)"
        result = Result.error(long_msg)
        assert result.is_error()
        assert result.error == long_msg
        assert len(result.error) > 100

    def test_error_with_special_chars(self):
        """Verify Result creation with special characters in error."""
        result = Result.error("Error: \n\t\"quoted\" and 'single'")
        assert result.is_error()
        assert "\n" in result.error
        assert "\t" in result.error
        assert '"' in result.error
        assert "'" in result.error

    def test_error_with_multiline_message(self):
        """Verify Result creation with multiline error message."""
        msg = """Multi-line error:
Line 1
Line 2
Line 3"""
        result = Result.error(msg)
        assert result.is_error()
        assert "\n" in result.error
        assert "Line 1" in result.error


class TestStatusEnum:
    """Test Status enum functionality."""

    def test_status_success_value(self):
        """Verify Status.SUCCESS value."""
        assert Status.SUCCESS.value == "success"
        assert Status.SUCCESS.name == "SUCCESS"

    def test_status_error_value(self):
        """Verify Status.ERROR value."""
        assert Status.ERROR.value == "error"
        assert Status.ERROR.name == "ERROR"

    def test_status_is_success_on_success(self):
        """Verify Status.is_success() returns True for SUCCESS."""
        status = Status.SUCCESS
        assert status.is_success() is True
        assert status.is_error() is False

    def test_status_is_error_on_error(self):
        """Verify Status.is_error() returns True for ERROR."""
        status = Status.ERROR
        assert status.is_error() is True
        assert status.is_success() is False

    def test_status_from_bool_true(self):
        """Verify Status.from_bool(True) returns SUCCESS."""
        status = Status.from_bool(True)
        assert status == Status.SUCCESS
        assert status.is_success() is True

    def test_status_from_bool_false(self):
        """Verify Status.from_bool(False) returns ERROR."""
        status = Status.from_bool(False)
        assert status == Status.ERROR
        assert status.is_error() is True

    def test_status_as_bool_on_success(self):
        """Verify Status.as_bool() returns True for SUCCESS."""
        status = Status.SUCCESS
        assert status.as_bool() is True

    def test_status_as_bool_on_error(self):
        """Verify Status.as_bool() returns False for ERROR."""
        status = Status.ERROR
        assert status.as_bool() is False

    def test_status_roundtrip(self):
        """Verify Status roundtrip: bool -> Status -> bool."""
        # True -> SUCCESS -> True
        status1 = Status.from_bool(True)
        assert status1.as_bool() is True

        # False -> ERROR -> False
        status2 = Status.from_bool(False)
        assert status2.as_bool() is False


class TestIsSuccessMethod:
    """Test is_success() method behavior."""

    def test_is_success_on_success_result(self):
        """Verify is_success() returns True for SUCCESS status."""
        result = Result.success({"data": "value"})
        assert result.is_success() is True
        assert result.is_error() is False

    def test_is_success_on_error_result(self):
        """Verify is_success() returns False for ERROR status."""
        result = Result.error("Test error")
        assert result.is_success() is False
        assert result.is_error() is True

    def test_is_success_with_none_data(self):
        """Verify is_success() works with None data."""
        result = Result.success(None)
        assert result.is_success() is True
        assert result.data is None

    def test_is_success_with_false_data(self):
        """Verify is_success() works with False data."""
        result = Result.success(False)
        assert result.is_success() is True
        assert result.data is False

    def test_is_success_with_zero_data(self):
        """Verify is_success() works with zero data."""
        result = Result.success(0)
        assert result.is_success() is True
        assert result.data == 0


class TestGetErrorMethod:
    """Test get_error() method behavior."""

    def test_get_error_on_success(self):
        """Verify get_error() returns None on success."""
        result = Result.success({"data": "value"})
        assert result.get_error() is None

    def test_get_error_on_success_with_none_data(self):
        """Verify get_error() returns None even with None data."""
        result = Result.success(None)
        assert result.get_error() is None

    def test_get_error_on_error(self):
        """Verify get_error() returns error message on error."""
        result = Result.error("File not found")
        assert result.get_error() == "File not found"

    def test_get_error_with_empty_message(self):
        """Verify get_error() returns empty string when set."""
        result = Result.error("")
        assert result.get_error() == ""

    def test_get_error_with_special_chars(self):
        """Verify get_error() preserves special characters."""
        msg = "Error: \n\t\"test\""
        result = Result.error(msg)
        assert result.get_error() == msg
        assert "\n" in result.get_error()
        assert "\t" in result.get_error()


class TestGetDataMethod:
    """Test get_data() method behavior."""

    def test_get_data_on_success(self):
        """Verify get_data() returns data on success."""
        result = Result.success({"key": "value"})
        assert result.get_data() == {"key": "value"}

    def test_get_data_on_success_with_none(self):
        """Verify get_data() returns None when data is None."""
        result = Result.success(None)
        assert result.get_data() is None

    def test_get_data_on_success_with_empty_string(self):
        """Verify get_data() returns empty string."""
        result = Result.success("")
        assert result.get_data() == ""
        assert isinstance(result.get_data(), str)

    def test_get_data_on_success_with_false(self):
        """Verify get_data() returns False value."""
        result = Result.success(False)
        assert result.get_data() is False

    def test_get_data_on_success_with_zero(self):
        """Verify get_data() returns zero value."""
        result = Result.success(0)
        assert result.get_data() == 0

    def test_get_data_on_error_no_default(self):
        """Verify get_data() returns None on error without default."""
        result = Result.error("Test error")
        assert result.get_data() is None

    def test_get_data_on_error_with_none_default(self):
        """Verify get_data() with default=None on error."""
        result = Result.error("Test error")
        assert result.get_data(None) is None

    def test_get_data_on_error_with_dict_default(self):
        """Verify get_data() returns dict default on error."""
        result = Result.error("Test error")
        default = {"default": "value"}
        assert result.get_data(default) == default

    def test_get_data_on_error_with_list_default(self):
        """Verify get_data() returns list default on error."""
        result = Result.error("Test error")
        default = [1, 2, 3]
        assert result.get_data(default) == default

    def test_get_data_on_error_with_string_default(self):
        """Verify get_data() returns string default on error."""
        result = Result.error("Test error")
        assert result.get_data("default") == "default"

    def test_get_data_on_error_with_number_default(self):
        """Verify get_data() returns numeric default on error."""
        result = Result.error("Test error")
        assert result.get_data(42) == 42

    def test_get_data_on_error_with_zero_default(self):
        """Verify get_data() returns zero as default."""
        result = Result.error("Test error")
        assert result.get_data(0) == 0

    def test_get_data_on_error_with_false_default(self):
        """Verify get_data() returns False as default."""
        result = Result.error("Test error")
        assert result.get_data(False) is False


class TestGetDataOrMethod:
    """Test get_data_or() method behavior."""

    def test_get_data_or_on_success(self):
        """Verify get_data_or() returns data on success (ignores default)."""
        result = Result.success({"actual": "data"})
        assert result.get_data_or({"default": "value"}) == {"actual": "data"}

    def test_get_data_or_on_error(self):
        """Verify get_data_or() returns default on error."""
        result = Result.error("Test error")
        assert result.get_data_or({"default": "value"}) == {"default": "value"}

    def test_get_data_or_with_none_default(self):
        """Verify get_data_or() works with None default."""
        result = Result.error("Test error")
        assert result.get_data_or(None) is None

    def test_get_data_or_on_success_with_none_data(self):
        """Verify get_data_or() returns None data on success."""
        result = Result.success(None)
        assert result.get_data_or("default") is None

    def test_get_data_or_on_success_with_false_data(self):
        """Verify get_data_or() returns False data on success."""
        result = Result.success(False)
        assert result.get_data_or(True) is False

    def test_get_data_or_on_success_with_zero_data(self):
        """Verify get_data_or() returns zero data on success."""
        result = Result.success(0)
        assert result.get_data_or(100) == 0


class TestMapMethod:
    """Test map() method behavior."""

    def test_map_on_success(self):
        """Verify map() applies function on success."""
        result = Result.success([1, 2, 3, 4, 5])
        mapped = result.map(len)
        assert mapped.is_success()
        assert mapped.data == 5

    def test_map_on_success_with_transform(self):
        """Verify map() transforms data."""
        result = Result.success({"key": "value"})
        mapped = result.map(lambda d: d.get("key", ""))
        assert mapped.is_success()
        assert mapped.data == "value"

    def test_map_on_success_with_dict_transform(self):
        """Verify map() can transform dict to list."""
        result = Result.success({"a": 1, "b": 2})
        mapped = result.map(lambda d: list(d.keys()))
        assert mapped.is_success()
        assert mapped.data == ["a", "b"]

    def test_map_on_error(self):
        """Verify map() returns error result unchanged."""
        result = Result.error("Test error")
        mapped = result.map(lambda x: len(x))
        assert mapped.is_error()
        assert mapped.error == "Test error"
        assert mapped is result  # Same instance

    def test_map_with_exception(self):
        """Verify map() catches exceptions and returns error."""
        result = Result.success([1, 2, 3])
        mapped = result.map(lambda x: 1 / 0)  # Division by zero
        assert mapped.is_error()
        assert "division by zero" in mapped.error

    def test_map_with_type_error(self):
        """Verify map() catches TypeError."""
        result = Result.success("not a number")
        mapped = result.map(lambda x: x + 1)  # Can't add int to str
        assert mapped.is_error()
        assert "Map function failed" in mapped.error

    def test_map_chain(self):
        """Verify map() can be chained."""
        result = Result.success([1, 2, 3, 4, 5])
        chained = result.map(len).map(lambda x: x * 2)
        assert chained.is_success()
        assert chained.data == 10  # len([1,2,3,4,5]) = 5, *2 = 10

    def test_map_on_success_with_none(self):
        """Verify map() works with None data."""
        result = Result.success(None)
        mapped = result.map(lambda x: "transformed" if x is None else x)
        assert mapped.is_success()
        assert mapped.data == "transformed"


class TestAndThenMethod:
    """Test and_then() method behavior."""

    def test_and_then_on_success(self):
        """Verify and_then() chains successful operations."""
        result = Result.success({"key": "value"})

        def validate(data):
            if "key" in data:
                return Result.success(data)
            return Result.error("Missing 'key'")

        chained = result.and_then(validate)
        assert chained.is_success()
        assert chained.data == {"key": "value"}

    def test_and_then_on_error(self):
        """Verify and_then() returns error unchanged."""
        result = Result.error("Initial error")

        def validate(data):
            return Result.success(data)

        chained = result.and_then(validate)
        assert chained.is_error()
        assert chained.error == "Initial error"
        assert chained is result  # Same instance

    def test_and_then_with_error_result(self):
        """Verify and_then() propagates error from chained function."""
        result = Result.success({"missing": "key"})

        def validate(data):
            if "key" not in data:
                return Result.error("Missing 'key'")
            return Result.success(data)

        chained = result.and_then(validate)
        assert chained.is_error()
        assert chained.error == "Missing 'key'"

    def test_and_then_chain_multiple(self):
        """Verify and_then() can chain multiple operations."""
        result = Result.success({"data": "value"})

        def step1(data):
            return Result.success(list(data.keys()))

        def step2(data):
            return Result.success(len(data))

        def step3(data):
            return Result.success(data * 2)

        chained = result.and_then(step1).and_then(step2).and_then(step3)
        assert chained.is_success()
        assert chained.data == 2  # ["data"] -> 1 -> 2

    def test_and_then_with_transform(self):
        """Verify and_then() can transform data types."""
        result = Result.success({"a": 1, "b": 2})

        def to_list(data):
            return Result.success(list(data.items()))

        chained = result.and_then(to_list)
        assert chained.is_success()
        assert isinstance(chained.data, list)
        assert ("a", 1) in chained.data


class TestBooleanConversion:
    """Test __bool__() method behavior."""

    def test_bool_on_success(self):
        """Verify bool(result) returns True for success."""
        result = Result.success({"data": "value"})
        assert bool(result) is True
        assert result.is_success() is True

    def test_bool_on_error(self):
        """Verify bool(result) returns False for error."""
        result = Result.error("Test error")
        assert bool(result) is False
        assert result.is_error() is True

    def test_bool_on_success_with_false_data(self):
        """Verify bool(result) is True even with False data."""
        result = Result.success(False)
        assert bool(result) is True
        assert result.data is False

    def test_bool_on_success_with_zero_data(self):
        """Verify bool(result) is True even with zero data."""
        result = Result.success(0)
        assert bool(result) is True
        assert result.data == 0

    def test_bool_on_success_with_none_data(self):
        """Verify bool(result) is True even with None data."""
        result = Result.success(None)
        assert bool(result) is True
        assert result.data is None

    def test_bool_on_success_with_empty_string(self):
        """Verify bool(result) is True even with empty string."""
        result = Result.success("")
        assert bool(result) is True
        assert result.data == ""

    def test_bool_on_success_with_empty_list(self):
        """Verify bool(result) is True even with empty list."""
        result = Result.success([])
        assert bool(result) is True
        assert result.data == []

    def test_bool_on_success_with_empty_dict(self):
        """Verify bool(result) is True even with empty dict."""
        result = Result.success({})
        assert bool(result) is True
        assert result.data == {}


class TestStringRepresentation:
    """Test __str__() and __repr__() methods."""

    def test_str_on_success(self):
        """Verify __str__() shows SUCCESS and data type."""
        result = Result.success({"key": "value"})
        str_repr = str(result)
        assert "SUCCESS" in str_repr
        assert "dict" in str_repr

    def test_str_on_success_with_list(self):
        """Verify __str__() shows list type."""
        result = Result.success([1, 2, 3])
        str_repr = str(result)
        assert "SUCCESS" in str_repr
        assert "list" in str_repr

    def test_str_on_success_with_string(self):
        """Verify __str__() shows str type."""
        result = Result.success("test")
        str_repr = str(result)
        assert "SUCCESS" in str_repr
        assert "str" in str_repr

    def test_str_on_success_with_none(self):
        """Verify __str__() shows None type."""
        result = Result.success(None)
        str_repr = str(result)
        assert "SUCCESS" in str_repr
        assert "None" in str_repr

    def test_str_on_error(self):
        """Verify __str__() shows ERROR and message."""
        result = Result.error("File not found")
        str_repr = str(result)
        assert "ERROR" in str_repr
        assert "File not found" in str_repr

    def test_str_on_error_with_empty_message(self):
        """Verify __str__() shows empty error message."""
        result = Result.error("")
        str_repr = str(result)
        assert "ERROR" in str_repr
        assert "error=" in str_repr

    def test_repr_on_success(self):
        """Verify __repr__() shows all fields."""
        result = Result.success({"key": "value"})
        repr_str = repr(result)
        assert "Result(status=" in repr_str
        assert "Status.SUCCESS" in repr_str
        assert "'key': 'value'" in repr_str
        assert "error=None" in repr_str

    def test_repr_on_error(self):
        """Verify __repr__() shows error details."""
        result = Result.error("Test error")
        repr_str = repr(result)
        assert "Result(status=" in repr_str
        assert "Status.ERROR" in repr_str
        assert "data=None" in repr_str
        assert "error='Test error'" in repr_str

    def test_repr_on_success_with_none(self):
        """Verify __repr__() shows None data."""
        result = Result.success(None)
        repr_str = repr(result)
        assert "data=None" in repr_str
        assert "error=None" in repr_str


class TestGenericTypes:
    """Test generic type behavior."""

    def test_result_with_dict_type(self):
        """Verify Result[Dict[str, Any]] works correctly."""
        result: Result[Dict[str, Any]] = Result.success({"key": "value"})
        assert isinstance(result.data, dict)
        assert result.data["key"] == "value"

    def test_result_with_list_type(self):
        """Verify Result[List[int]] works correctly."""
        result: Result[List[int]] = Result.success([1, 2, 3])
        assert isinstance(result.data, list)
        assert all(isinstance(x, int) for x in result.data)

    def test_result_with_string_type(self):
        """Verify Result[str] works correctly."""
        result: Result[str] = Result.success("test string")
        assert isinstance(result.data, str)
        assert result.data == "test string"

    def test_result_with_optional_type(self):
        """Verify Result[Optional[str]] works correctly."""
        result: Result[Optional[str]] = Result.success(None)
        assert result.data is None

    def test_dict_result_type_alias(self):
        """Verify DictResult type alias works."""
        from internal.yamlutil.result_types import DictResult
        result: DictResult = Result.success({"key": "value"})
        assert isinstance(result.data, dict)

    def test_list_result_type_alias(self):
        """Verify ListResult type alias works."""
        from internal.yamlutil.result_types import ListResult
        result: ListResult = Result.success([1, 2, 3])
        assert isinstance(result.data, list)

    def test_str_result_type_alias(self):
        """Verify StrResult type alias works."""
        from internal.yamlutil.result_types import StrResult
        result: StrResult = Result.success("test")
        assert isinstance(result.data, str)

    def test_bool_result_type_alias(self):
        """Verify BoolResult type alias works."""
        from internal.yamlutil.result_types import BoolResult
        result: BoolResult = Result.success(True)
        assert isinstance(result.data, bool)

    def test_int_result_type_alias(self):
        """Verify IntResult type alias works."""
        from internal.yamlutil.result_types import IntResult
        result: IntResult = Result.success(42)
        assert isinstance(result.data, int)


class TestEdgeCases:
    """Test edge cases and complex scenarios."""

    def test_success_with_complex_nested_dict(self):
        """Verify Result with deeply nested dict structure."""
        data = {
            "level1": {
                "level2": {
                    "level3": {
                        "value": 42
                    }
                }
            }
        }
        result = Result.success(data)
        assert result.is_success()
        assert result.data["level1"]["level2"]["level3"]["value"] == 42

    def test_success_with_mixed_list(self):
        """Verify Result with mixed-type list."""
        data = [1, "two", 3.0, True, None]
        result = Result.success(data)
        assert result.is_success()
        assert result.data == data

    def test_error_with_unicode_message(self):
        """Verify Result with Unicode error message."""
        result = Result.error("Error: 你好 🚀")
        assert result.is_error()
        assert "你好" in result.error
        assert "🚀" in result.error

    def test_map_with_lambda_capturing_vars(self):
        """Verify map() with lambda capturing outer variables."""
        multiplier = 3
        result = Result.success([1, 2, 3])
        mapped = result.map(lambda x: len(x) * multiplier)
        assert mapped.is_success()
        assert mapped.data == 9  # len([1,2,3]) = 3, *3 = 9

    def test_and_then_with_validation_chain(self):
        """Verify and_then() with complex validation chain."""
        result = Result.success({"name": "test", "age": 25})

        def validate_name(data):
            if not data.get("name"):
                return Result.error("Name is required")
            return Result.success(data)

        def validate_age(data):
            age = data.get("age")
            if not isinstance(age, int) or age < 0:
                return Result.error("Age must be positive integer")
            return Result.success(data)

        validated = result.and_then(validate_name).and_then(validate_age)
        assert validated.is_success()
        assert validated.data == result.data

    def test_and_then_with_validation_failure(self):
        """Verify and_then() stops at first validation failure."""
        result = Result.success({"age": -5})

        def validate_name(data):
            if not data.get("name"):
                return Result.error("Name is required")
            return Result.success(data)

        def validate_age(data):
            age = data.get("age")
            if not isinstance(age, int) or age < 0:
                return Result.error("Age must be positive integer")
            return Result.success(data)

        validated = result.and_then(validate_name).and_then(validate_age)
        assert validated.is_error()
        assert "Name is required" in validated.error

    def test_result_immutability_on_error(self):
        """Verify error results don't expose mutable data."""
        result = Result.error("Test error")
        # Accessing data on error should not expose mutable state
        assert result.data is None
        assert result.get_data({}) == {}
        assert result.get_data_or({}) == {}

    def test_multiple_results_independent(self):
        """Verify multiple Result instances are independent."""
        result1 = Result.success({"key": "value1"})
        result2 = Result.success({"key": "value2"})

        assert result1.data["key"] == "value1"
        assert result2.data["key"] == "value2"

        # Modify one, shouldn't affect the other (they're different instances)
        result1.data["key"] = "modified"
        assert result1.data["key"] == "modified"
        assert result2.data["key"] == "value2"


def run_all_tests():
    """Run all test classes and report results."""
    test_classes = [
        ("Result Creation", TestResultCreation),
        ("Status Enum", TestStatusEnum),
        ("is_success() Method", TestIsSuccessMethod),
        ("get_error() Method", TestGetErrorMethod),
        ("get_data() Method", TestGetDataMethod),
        ("get_data_or() Method", TestGetDataOrMethod),
        ("map() Method", TestMapMethod),
        ("and_then() Method", TestAndThenMethod),
        ("Boolean Conversion", TestBooleanConversion),
        ("String Representation", TestStringRepresentation),
        ("Generic Types", TestGenericTypes),
        ("Edge Cases", TestEdgeCases),
    ]

    total_passed = 0
    total_failed = 0

    print("=" * 70)
    print("COMPREHENSIVE RESULT UNIT TESTS (bf-3prra)")
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
        print("  ✓ All Result creation scenarios are tested")
        print("  ✓ All helper methods (is_success, get_error, get_data, get_data_or) tested")
        print("  ✓ Advanced methods (map, and_then) tested")
        print("  ✓ Status enum methods (from_bool, as_bool) tested")
        print("  ✓ Edge cases covered (None data, empty error messages, etc.)")
        print("  ✓ All tests pass")
        return True
    else:
        print(f"\n❌ {total_failed} test(s) failed")
        return False


if __name__ == "__main__":
    import sys
    success = run_all_tests()
    sys.exit(0 if success else 1)
