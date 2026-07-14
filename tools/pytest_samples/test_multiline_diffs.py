"""
Multiline diff failures - demonstrating context and diff output
"""
import pytest


def test_multiline_string_diff():
    """Demonstrates multiline string diff"""
    expected = """Line one
Line two
Line three
Line four
Line five"""
    actual = """Line one
Line two
Line THREE
Line four
Line five"""
    assert actual == expected


def test_code_diff():
    """Demonstrates code/programming text diff"""
    expected_code = """
def calculate(x, y):
    result = x + y
    return result * 2
"""
    actual_code = """
def calculate(x, y):
    result = x + y
    return result * 3
"""
    assert actual_code == expected_code


def test_long_list_diff():
    """Demonstrates diff in a long list"""
    expected = list(range(20))
    actual = list(range(20))
    actual[10] = 999  # Change one element
    assert actual == expected


def test_json_like_diff():
    """Demonstrates JSON-like structure diff"""
    expected = """{
    "status": "success",
    "data": {
        "items": [1, 2, 3],
        "count": 3
    }
}"""
    actual = """{
    "status": "success",
    "data": {
        "items": [1, 2, 4],
        "count": 3
    }
}"""
    assert actual == expected
