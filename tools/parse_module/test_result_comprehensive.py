#!/usr/bin/env python3
"""
Comprehensive unit tests for ParseResult structure.

Tests cover:
- Result creation with SUCCESS and ERROR status
- is_success() and is_error() methods
- get_error() and get_data() methods
- Edge cases (None values, empty strings, complex data types, etc.)

Run with pytest: pytest test_result_comprehensive.py
Run with unittest: python3 -m unittest test_result_comprehensive -v
Run standalone: python3 test_result_comprehensive.py
"""

import unittest
from result import ParseResult, ParseStatus


class TestResultCreation(unittest.TestCase):
    """Tests for ParseResult creation with different statuses."""

    def test_success_result_creation(self):
        """Test creating a success result."""
        result = ParseResult(status=ParseStatus.SUCCESS, data={'key': 'value'})
        self.assertEqual(result.status, ParseStatus.SUCCESS)
        self.assertEqual(result.data, {'key': 'value'})
        self.assertIsNone(result.error)

    def test_error_result_creation(self):
        """Test creating an error result."""
        result = ParseResult(status=ParseStatus.ERROR, error='Test error')
        self.assertEqual(result.status, ParseStatus.ERROR)
        self.assertIsNone(result.data)
        self.assertEqual(result.error, 'Test error')


class TestStatusMethods(unittest.TestCase):
    """Tests for is_success() and is_error() methods."""

    def test_is_success_on_success_result(self):
        """Test is_success returns True for successful results."""
        result = ParseResult.success({'data': 'value'})
        self.assertTrue(result.is_success())
        self.assertFalse(result.is_error())

    def test_is_error_on_error_result(self):
        """Test is_error returns True for failed results."""
        result = ParseResult.make_error('Error message')
        self.assertTrue(result.is_error())
        self.assertFalse(result.is_success())

    def test_status_methods_on_directly_created_results(self):
        """Test status methods on directly created results."""
        success_result = ParseResult(status=ParseStatus.SUCCESS, data={})
        error_result = ParseResult(status=ParseStatus.ERROR, error='Failed')

        self.assertTrue(success_result.is_success())
        self.assertFalse(success_result.is_error())
        self.assertFalse(error_result.is_success())
        self.assertTrue(error_result.is_error())


class TestDataAccess(unittest.TestCase):
    """Tests for get_data() and get_error() methods."""

    def test_get_data_returns_data_on_success(self):
        """Test get_data returns data when successful."""
        result = ParseResult.success({'key': 'value'})
        data = result.get_data()
        self.assertEqual(data, {'key': 'value'})

    def test_get_data_raises_runtime_error_on_failure(self):
        """Test get_data raises RuntimeError when result is an error."""
        result = ParseResult.make_error('Test error')
        with self.assertRaises(RuntimeError) as context:
            result.get_data()
        self.assertIn('Test error', str(context.exception))

    def test_get_error_returns_error_message(self):
        """Test get_error returns error message."""
        error_result = ParseResult.make_error('Test error message')
        self.assertEqual(error_result.get_error(), 'Test error message')

    def test_get_error_returns_none_on_success(self):
        """Test get_error returns None for successful results."""
        success_result = ParseResult.success({'data': 'value'})
        self.assertIsNone(success_result.get_error())


class TestFactoryMethods(unittest.TestCase):
    """Tests for success() and make_error() factory methods."""

    def test_success_factory_method(self):
        """Test ParseResult.success() factory method."""
        result = ParseResult.success({'key': 'value', 'number': 42})
        self.assertEqual(result.status, ParseStatus.SUCCESS)
        self.assertEqual(result.data, {'key': 'value', 'number': 42})
        self.assertIsNone(result.error)
        self.assertTrue(result.is_success())

    def test_make_error_factory_method(self):
        """Test ParseResult.make_error() factory method."""
        result = ParseResult.make_error('File not found: /path/to/file.yaml')
        self.assertEqual(result.status, ParseStatus.ERROR)
        self.assertIsNone(result.data)
        self.assertEqual(result.error, 'File not found: /path/to/file.yaml')
        self.assertTrue(result.is_error())


class TestEdgeCases(unittest.TestCase):
    """Tests for edge cases and special values."""

    def test_success_with_none_data(self):
        """Test success result with None as data."""
        result = ParseResult.success(None)
        self.assertTrue(result.is_success())
        self.assertIsNone(result.data)
        self.assertIsNone(result.get_data())

    def test_error_with_empty_string(self):
        """Test error result with empty string as error message."""
        result = ParseResult.make_error('')
        self.assertTrue(result.is_error())
        self.assertEqual(result.error, '')
        self.assertEqual(result.get_error(), '')

    def test_success_with_empty_dict(self):
        """Test success result with empty dictionary."""
        result = ParseResult.success({})
        self.assertTrue(result.is_success())
        self.assertEqual(result.data, {})
        self.assertEqual(result.get_data(), {})

    def test_success_with_empty_list(self):
        """Test success result with empty list."""
        result = ParseResult.success([])
        self.assertTrue(result.is_success())
        self.assertEqual(result.data, [])
        self.assertEqual(result.get_data(), [])

    def test_success_with_empty_string(self):
        """Test success result with empty string."""
        result = ParseResult.success('')
        self.assertTrue(result.is_success())
        self.assertEqual(result.data, '')
        self.assertEqual(result.get_data(), '')

    def test_success_with_zero(self):
        """Test success result with zero as data."""
        result = ParseResult.success(0)
        self.assertTrue(result.is_success())
        self.assertEqual(result.data, 0)
        self.assertEqual(result.get_data(), 0)

    def test_success_with_false(self):
        """Test success result with False as data."""
        result = ParseResult.success(False)
        self.assertTrue(result.is_success())
        self.assertFalse(result.data)
        self.assertFalse(result.get_data())

    def test_success_with_nested_structures(self):
        """Test success result with nested data structures."""
        complex_data = {
            'level1': {
                'level2': {
                    'level3': ['a', 'b', 'c']
                }
            },
            'numbers': [1, 2, 3],
            'nested_list': [[1, 2], [3, 4]]
        }
        result = ParseResult.success(complex_data)
        self.assertTrue(result.is_success())
        self.assertEqual(result.get_data(), complex_data)

    def test_error_with_special_characters(self):
        """Test error result with special characters in error message."""
        special_chars = '!@#$%^&*()_+-=[]{}|;:\',.<>?/~`'
        result = ParseResult.make_error(f'Error: {special_chars}')
        self.assertTrue(result.is_error())
        self.assertEqual(result.error, f'Error: {special_chars}')

    def test_error_with_newlines(self):
        """Test error result with newlines in error message."""
        multiline_error = '''Line 1
Line 2
Line 3'''
        result = ParseResult.make_error(multiline_error)
        self.assertTrue(result.is_error())
        self.assertEqual(result.error, multiline_error)

    def test_error_with_unicode(self):
        """Test error result with unicode characters."""
        unicode_error = 'Error: 错误 🚀 💥 émojis'
        result = ParseResult.make_error(unicode_error)
        self.assertTrue(result.is_error())
        self.assertEqual(result.error, unicode_error)

    def test_error_with_very_long_message(self):
        """Test error result with very long error message."""
        long_message = 'Error: ' + 'A' * 10000
        result = ParseResult.make_error(long_message)
        self.assertTrue(result.is_error())
        self.assertEqual(len(result.error), 10007)
        self.assertTrue(result.error.startswith('Error: AAAA'))

    def test_success_with_large_data(self):
        """Test success result with large data structure."""
        large_data = {f'key_{i}': f'value_{i}' for i in range(1000)}
        result = ParseResult.success(large_data)
        self.assertTrue(result.is_success())
        self.assertEqual(len(result.get_data()), 1000)
        self.assertEqual(result.get_data()['key_500'], 'value_500')

    def test_success_with_tuple(self):
        """Test success result with tuple as data."""
        result = ParseResult.success((1, 2, 3))
        self.assertTrue(result.is_success())
        self.assertEqual(result.get_data(), (1, 2, 3))

    def test_success_with_set(self):
        """Test success result with set as data."""
        data_set = {1, 2, 3, 4, 5}
        result = ParseResult.success(data_set)
        self.assertTrue(result.is_success())
        self.assertEqual(result.get_data(), data_set)

    def test_success_with_custom_object(self):
        """Test success result with custom object as data."""
        class CustomObject:
            def __init__(self, value):
                self.value = value

        obj = CustomObject(42)
        result = ParseResult.success(obj)
        self.assertTrue(result.is_success())
        self.assertEqual(result.get_data().value, 42)

    def test_runtime_error_message_includes_original_error(self):
        """Test that RuntimeError from get_data includes original error."""
        original_error = 'YAML syntax error at line 5'
        result = ParseResult.make_error(original_error)
        with self.assertRaises(RuntimeError) as context:
            result.get_data()
        self.assertIn(original_error, str(context.exception))


class TestStringRepresentation(unittest.TestCase):
    """Tests for __str__ method."""

    def test_string_representation_success(self):
        """Test string representation of success result."""
        result = ParseResult.success({'data': 'value'})
        result_str = str(result)
        self.assertIn('success', result_str.lower())
        self.assertIn('dict', result_str)

    def test_string_representation_error(self):
        """Test string representation of error result."""
        result = ParseResult.make_error('Test error')
        result_str = str(result)
        self.assertIn('error', result_str.lower())
        self.assertIn('Test error', result_str)

    def test_string_representation_with_list_data(self):
        """Test string representation with list data."""
        result = ParseResult.success([1, 2, 3])
        result_str = str(result)
        self.assertIn('success', result_str.lower())
        self.assertIn('list', result_str)

    def test_string_representation_with_none_data(self):
        """Test string representation with None data."""
        result = ParseResult.success(None)
        result_str = str(result)
        self.assertIn('success', result_str.lower())
        self.assertIn('NoneType', result_str)


class TestParseStatusEnum(unittest.TestCase):
    """Tests for ParseStatus enum."""

    def test_parse_status_enum_values(self):
        """Test ParseStatus enum values."""
        self.assertEqual(ParseStatus.SUCCESS.value, 'success')
        self.assertEqual(ParseStatus.ERROR.value, 'error')

    def test_parse_status_enum_comparison(self):
        """Test ParseStatus enum comparison."""
        result = ParseResult.success({})
        self.assertIs(result.status, ParseStatus.SUCCESS)
        self.assertIsNot(result.status, ParseStatus.ERROR)


class TestAllFieldsPresent(unittest.TestCase):
    """Tests that all required fields and methods are present."""

    def test_all_success_result_fields_present(self):
        """Test that all required fields are present on success result."""
        result = ParseResult.success({'test': 'data'})
        self.assertTrue(hasattr(result, 'status'))
        self.assertTrue(hasattr(result, 'data'))
        self.assertTrue(hasattr(result, 'error'))
        self.assertTrue(hasattr(result, 'is_success'))
        self.assertTrue(hasattr(result, 'is_error'))
        self.assertTrue(hasattr(result, 'get_data'))
        self.assertTrue(hasattr(result, 'get_error'))

    def test_all_error_result_fields_present(self):
        """Test that all required fields are present on error result."""
        result = ParseResult.make_error('Test error')
        self.assertTrue(hasattr(result, 'status'))
        self.assertTrue(hasattr(result, 'data'))
        self.assertTrue(hasattr(result, 'error'))
        self.assertTrue(hasattr(result, 'is_success'))
        self.assertTrue(hasattr(result, 'is_error'))
        self.assertTrue(hasattr(result, 'get_data'))
        self.assertTrue(hasattr(result, 'get_error'))


if __name__ == '__main__':
    # Allow running with Python directly
    unittest.main()
