#!/usr/bin/env python3
"""
Minimal failing test cases for pytest flag evaluation.

This test file is designed to fail with clear assertion differences,
providing a controlled testbed for evaluating different pytest flags.

Expected Failure Patterns:
- test_simple_equality: Shows basic string/number diff
- test_dict_equality: Shows dictionary structure diff
- test_list_comparison: Shows list content diff
- test_multiline_string: Shows multiline text diff
- test_numeric_comparison: Shows numeric comparison diff
- test_in_operator: Shows membership test diff
- test_long_sequence: Shows diff of longer sequences
"""

def test_simple_equality():
    """Basic equality assertion - simple diff."""
    expected = "hello world"
    actual = "hello there"
    assert actual == expected, f"Expected '{expected}' but got '{actual}'"


def test_dict_equality():
    """Dictionary equality - shows key/value diffs."""
    expected = {
        "name": "test",
        "count": 42,
        "active": True,
        "tags": ["a", "b", "c"]
    }
    actual = {
        "name": "test",
        "count": 43,  # Wrong number
        "active": False,  # Wrong boolean
        "tags": ["a", "b", "d"]  # Wrong list element
    }
    assert actual == expected


def test_list_comparison():
    """List equality - shows element-wise diffs."""
    expected = [1, 2, 3, 4, 5]
    actual = [1, 2, 0, 4, 6]  # Wrong elements at indices 2 and 4
    assert actual == expected


def test_multiline_string():
    """Multiline string comparison - shows line-by-line diff."""
    expected = """line one
line two
line three
line four"""

    actual = """line one
LINE TWO  # different case
line three
line five"""  # different content

    assert actual == expected


def test_numeric_comparison():
    """Numeric comparison - shows which operators fail."""
    assert 5 > 10, "5 should be greater than 10"
    assert 3.14 >= 3.14159, "3.14 should be >= 3.14159"
    assert -1 < -5, "-1 should be less than -5"


def test_in_operator():
    """Membership test - shows what's missing from collections."""
    expected_items = {"apple", "banana", "cherry", "date"}
    actual_items = {"apple", "banana", "elderberry"}  # Missing 'cherry' and 'date', has 'elderberry' instead

    # Check if expected items are in actual
    for item in expected_items:
        assert item in actual_items, f"'{item}' should be in the collection"


def test_long_sequence():
    """Longer sequence comparison - shows how diffs scale."""
    expected = list(range(20))  # [0, 1, 2, ..., 19]
    actual = list(range(20))
    actual[5] = 99  # Change one element
    actual[15] = -1  # Change another element
    assert actual == expected


def test_nested_structure():
    """Nested data structure - shows hierarchical diff."""
    expected = {
        "user": {
            "name": "Alice",
            "age": 30,
            "address": {
                "city": "Springfield",
                "zip": "12345"
            }
        },
        "metadata": {
            "created": "2024-01-01",
            "version": 1
        }
    }

    actual = {
        "user": {
            "name": "Alice",
            "age": 31,  # Wrong age
            "address": {
                "city": "Shelbyville",  # Wrong city
                "zip": "12345"
            }
        },
        "metadata": {
            "created": "2024-01-02",  # Wrong date
            "version": 1
        }
    }

    assert actual == expected


def test_approximate_numbers():
    """Floating point comparison - shows precision diffs."""
    import math

    # This should fail and show the small difference
    assert math.pi == 3.141592653589793, "Pi should equal truncated value"

    # This should fail showing relative vs absolute difference
    assert 0.1 + 0.2 == 0.3, "Floating point arithmetic"


def test_boolean_logic():
    """Boolean logic failures."""
    x = True
    y = False

    # These will fail and show boolean evaluation
    assert x and y, "True AND False should be True"
    assert x or y is False, "True OR False should be True with strict False check"
    assert not x, "NOT True should be True"


def test_string_operations():
    """String operation failures."""
    text = "Hello World"

    # These will fail showing string comparisons
    assert text.lower() == "hello world", "lowercase conversion"
    assert text.upper() == "HELLO WORLD!", "uppercase conversion - wrong"  # Should fail
    assert text.replace("World", "Universe") == "Hello Universe", "string replacement"


if __name__ == "__main__":
    # Run with verbose output to see all failures
    import pytest
    import sys

    sys.exit(pytest.main([__file__, "-v", "--tb=short"]))
