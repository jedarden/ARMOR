#!/usr/bin/env python3
"""
Minimal pytest test file demonstrating various failure patterns
for machine-parseable output validation.

This file contains intentionally failing tests that produce different
assertion output patterns to verify parseability.

Author: ARMOR Project (bf-1rd15q)
Created: 2026-07-13
"""


def test_simple_equality():
    """Simple equality assertion failure."""
    greeting = "hello"
    assert greeting == "world", f"Expected 'world', got '{greeting}'"


def test_dict_equality():
    """Dictionary equality assertion failure."""
    expected = {"name": "Alice", "age": 30, "city": "NYC"}
    actual = {"name": "Bob", "age": 25, "city": "LA"}
    assert actual == expected, f"Dictionaries don't match"


def test_list_comparison():
    """List comparison with index diff."""
    list1 = [1, 2, 3, 4, 5]
    list2 = [1, 2, 10, 4, 5]
    assert list1 == list2, "Lists should be equal"


def test_multiline_string():
    """Multiline string comparison."""
    expected_text = """
    This is the expected text.
    It has multiple lines.
    """
    actual_text = """
    This is the actual text.
    It has different content.
    """
    assert actual_text == expected_text


def test_numeric_comparison():
    """Numeric comparison failure."""
    x = 5
    y = 10
    assert x == y, f"Numbers don't match: {x} != {y}"


def test_in_operator():
    """Test 'in' operator failure."""
    items = ["apple", "banana", "cherry"]
    assert "orange" in items, "orange should be in the list"


def test_long_sequence():
    """Long sequence comparison with index diff."""
    seq1 = [0, 1, 2, 3, 4, 10, 6, 7, 8, 9]
    seq2 = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
    assert seq1 == seq2, "Sequences should match"


def test_nested_structure():
    """Nested data structure comparison."""
    data1 = {
        "users": [
            {"name": "Alice", "scores": [100, 95, 88]},
            {"name": "Bob", "scores": [92, 87, 91]}
        ]
    }
    data2 = {
        "users": [
            {"name": "Alice", "scores": [100, 95, 88]},
            {"name": "Bob", "scores": [92, 99, 91]}  # Different score
        ]
    }
    assert data1 == data2, "Nested structures should match"


def test_approximate_numbers():
    """Approximate number comparison."""
    import math
    result = 0.1 + 0.2  # Floating point imprecision
    expected = 0.3
    assert result == expected, f"Floats don't match: {result} != {expected}"


def test_boolean_logic():
    """Boolean logic assertion."""
    x = True
    y = False
    assert x and y, "Both x and y should be True"


def test_string_operations():
    """String operation failure."""
    text = "Hello, World!"
    assert text.upper() == "HELLO", "Uppercase conversion failed"


def test_type_checking():
    """Type checking assertion."""
    value = "123"
    assert isinstance(value, int), f"Value should be int, got {type(value).__name__}"


def test_set_operations():
    """Set comparison failure."""
    set1 = {1, 2, 3, 4}
    set2 = {1, 2, 3, 5}
    assert set1 == set2, "Sets should be equal"


def test_range_comparison():
    """Range comparison failure."""
    r1 = range(0, 10)
    r2 = range(0, 11)
    assert r1 == r2, "Ranges should be equal"


def test_tuple_comparison():
    """Tuple comparison failure."""
    t1 = (1, 2, 3)
    t2 = (1, 2, 4)
    assert t1 == t2, "Tuples should be equal"


if __name__ == "__main__":
    # Allow running directly for testing
    import pytest
    pytest.main([__file__, "-vv", "--tb=short"])
