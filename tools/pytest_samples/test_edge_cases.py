"""
Edge cases - various assertion types and special conditions
"""
import pytest


def test_approximate_float_failure():
    """Demonstrates floating point comparison failure"""
    result = 0.1 + 0.2
    expected = 0.3
    assert result == expected, f"Float comparison failed: {result} != {expected}"


def test_in_operator_failure():
    """Demonstrates 'in' assertion failure"""
    items = ["apple", "banana", "cherry"]
    assert "orange" in items, "Item not found in list"


def test_key_error_dict():
    """Demonstrates missing key failure"""
    data = {"a": 1, "b": 2}
    value = data["c"]  # KeyError


def test_attribute_error():
    """Demonstrates attribute error failure"""
    obj = type("Obj", (), {})()
    assert obj.missing_attr == "value"  # AttributeError


def test_greater_than_failure():
    """Demonstrates comparison operator failure"""
    score = 75
    assert score >= 90, f"Score {score} is below passing threshold"


def test_custom_message_assertion():
    """Demonstrates assertion with custom detailed message"""
    user = {
        "name": "John",
        "age": 15,
        "email": "john@example.com"
    }
    assert user["age"] >= 18, (
        f"User {user['name']} is not old enough. "
        f"Age: {user['age']}, Required: 18"
    )


def test_empty_container():
    """Demonstrates empty container assertion failure"""
    items = []
    assert len(items) > 0, "Container should not be empty"


def test_regex_match_failure():
    """Demonstrates regex pattern matching failure"""
    import re
    text = "user_123"
    pattern = r"^admin_\d+$"
    assert re.match(pattern, text), f"Text '{text}' doesn't match pattern '{pattern}'"
