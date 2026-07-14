"""
Collection comparison failures - showing detailed diffs
"""
import pytest


def test_list_equality_fail():
    """List comparison with missing/extra elements"""
    expected = [1, 2, 3, 4, 5]
    actual = [1, 2, 3, 4, 6]
    assert actual == expected, f"Lists don't match"


def test_dict_equality_fail():
    """Dictionary comparison with different values"""
    expected = {"name": "Alice", "age": 30, "city": "NYC"}
    actual = {"name": "Alice", "age": 31, "city": "NYC"}
    assert actual == expected, "Dictionaries don't match"


def test_dict_missing_key_fail():
    """Dictionary comparison with missing keys"""
    expected = {"name": "Bob", "email": "bob@example.com", "role": "admin"}
    actual = {"name": "Bob", "email": "bob@example.com"}
    assert actual == expected, "Dictionaries don't match"


def test_nested_dict_fail():
    """Nested dictionary comparison failure"""
    expected = {
        "user": {"id": 1, "name": "Charlie"},
        "settings": {"theme": "dark", "notifications": True}
    }
    actual = {
        "user": {"id": 1, "name": "Charles"},
        "settings": {"theme": "dark", "notifications": True}
    }
    assert actual == expected


def test_set_comparison_fail():
    """Set comparison failure"""
    expected = {"apple", "banana", "cherry", "date"}
    actual = {"apple", "banana", "cherry", "elderberry"}
    assert actual == expected


def test_tuple_order_fail():
    """Tuple comparison - order matters"""
    expected = (1, 2, 3, 4)
    actual = (1, 3, 2, 4)
    assert actual == expected
