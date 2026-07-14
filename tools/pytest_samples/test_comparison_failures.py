"""
Comparison failures - list, dict, and string comparisons
"""
import pytest


def test_list_comparison():
    """Demonstrates list comparison failure"""
    expected = [1, 2, 3, 4, 5]
    actual = [1, 2, 3, 0, 5]
    assert actual == expected, f"Lists don't match"


def test_dict_comparison():
    """Demonstrates dict comparison failure"""
    expected = {
        "name": "test",
        "value": 42,
        "active": True
    }
    actual = {
        "name": "test",
        "value": 43,
        "active": True
    }
    assert actual == expected, "Dictionaries don't match"


def test_string_comparison():
    """Demonstrates string comparison failure"""
    expected = "The quick brown fox"
    actual = "The quick brown cat"
    assert actual == expected, "Strings don't match"


def test_nested_structure_comparison():
    """Demonstrates nested structure comparison failure"""
    expected = {
        "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
        ]
    }
    actual = {
        "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Robert"}
        ]
    }
    assert actual == expected, "Nested structures don't match"
