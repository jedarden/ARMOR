"""
Test Result dataclass implementation verification.

This module verifies that the Result dataclass meets all acceptance criteria
from bead bf-dy2ix.
"""

from typing import Dict, Any
from internal.yamlutil import Result, Status


class TestResultDataclass:
    """Test Result dataclass implementation."""

    def test_result_structure_exists(self):
        """Verify Result structure exists with all three fields."""
        result = Result.success({"key": "value"})

        # Check all three fields exist
        assert hasattr(result, 'status')
        assert hasattr(result, 'data')
        assert hasattr(result, 'error')

    def test_status_field_uses_enum(self):
        """Verify status field uses Status enum."""
        result = Result.success({"key": "value"})
        assert isinstance(result.status, Status)
        assert result.status == Status.SUCCESS

        error_result = Result.error("Test error")
        assert isinstance(error_result.status, Status)
        assert error_result.status == Status.ERROR

    def test_data_field_generic_typed(self):
        """Verify data field is generic/typed for parsed content."""
        # Test with dict
        dict_result: Result[Dict[str, Any]] = Result.success({"key": "value"})
        assert isinstance(dict_result.data, dict)
        assert dict_result.data == {"key": "value"}

        # Test with list
        list_result: Result[list] = Result.success([1, 2, 3])
        assert isinstance(list_result.data, list)
        assert list_result.data == [1, 2, 3]

        # Test with string
        str_result: Result[str] = Result.success("test")
        assert isinstance(str_result.data, str)
        assert str_result.data == "test"

    def test_error_field_typed(self):
        """Verify error field is typed for error messages."""
        result = Result.error("File not found")
        assert isinstance(result.error, str)
        assert result.error == "File not found"
        assert result.data is None

    def test_can_be_imported(self):
        """Verify Result can be imported from yamlutil module."""
        # This test passes if the import at the top of the file works
        from internal.yamlutil import Result as ResultImported
        from internal.yamlutil.result_types import Result as ResultDirect

        # Both imports should work
        assert ResultImported is ResultDirect

    def test_result_success_factory(self):
        """Verify Result.success() factory method."""
        result = Result.success({"test": "data"})
        assert result.is_success()
        assert result.data == {"test": "data"}
        assert result.error is None

    def test_result_error_factory(self):
        """Verify Result.error() factory method."""
        result = Result.error("Something went wrong")
        assert result.is_error()
        assert result.data is None
        assert result.error == "Something went wrong"

    def test_status_enum_values(self):
        """Verify Status enum has expected values."""
        assert Status.SUCCESS.value == "success"
        assert Status.ERROR.value == "error"

    def test_result_helper_methods(self):
        """Verify Result helper methods work correctly."""
        # is_success()
        success_result = Result.success({"data": "value"})
        assert success_result.is_success() is True
        assert success_result.is_error() is False

        # is_error()
        error_result = Result.error("Test error")
        assert error_result.is_error() is True
        assert error_result.is_success() is False

        # get_data()
        data = success_result.get_data()
        assert data == {"data": "value"}

        # get_error()
        error = error_result.get_error()
        assert error == "Test error"

    def test_result_boolean_conversion(self):
        """Verify Result can be used in boolean contexts."""
        success_result = Result.success({"data": "value"})
        assert bool(success_result) is True

        error_result = Result.error("Test error")
        assert bool(error_result) is False

    def test_result_string_representation(self):
        """Verify Result string representations."""
        # Success case - shows status and data type
        success_result = Result.success({"data": "value"})
        str_repr = str(success_result)
        assert "success" in str_repr  # lowercase from status.value
        assert "dict" in str_repr  # Shows data type, not literal "data"

        # Error case - shows status and error message
        error_result = Result.error("Test error")
        str_repr = str(error_result)
        assert "error" in str_repr  # lowercase from status.value
        assert "Test error" in str_repr


if __name__ == "__main__":
    # Run tests
    test = TestResultDataclass()

    print("Running Result dataclass verification tests...")

    tests = [
        ("Result structure exists with all three fields", test.test_result_structure_exists),
        ("Status field uses Status enum", test.test_status_field_uses_enum),
        ("Data field is generic/typed", test.test_data_field_generic_typed),
        ("Error field is typed for messages", test.test_error_field_typed),
        ("Can be imported from yamlutil", test.test_can_be_imported),
        ("Result.success() factory method", test.test_result_success_factory),
        ("Result.error() factory method", test.test_result_error_factory),
        ("Status enum values", test.test_status_enum_values),
        ("Result helper methods", test.test_result_helper_methods),
        ("Result boolean conversion", test.test_result_boolean_conversion),
        ("Result string representation", test.test_result_string_representation),
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
        print("\n✅ All acceptance criteria met for bead bf-dy2ix:")
        print("  ✓ Result structure exists with all three fields")
        print("  ✓ status field uses Status enum")
        print("  ✓ data field is generic/typed for parsed YAML content")
        print("  ✓ error field is typed for error messages")
        print("  ✓ Can be imported and instantiated")
