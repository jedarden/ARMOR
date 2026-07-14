"""
Simple assertion failures - basic pytest output examples
"""
import pytest


def test_simple_assertion_failure():
    """Demonstrates a basic assertion failure"""
    x = 5
    y = 10
    assert x == y, f"Expected {x} to equal {y}"


def test_boolean_assertion():
    """Demonstrates boolean assertion failure"""
    result = False
    assert result, "Expected result to be True"


def test_none_assertion():
    """Demonstrates None assertion failure"""
    value = None
    assert value is not None, "Value should not be None"


def test_type_assertion():
    """Demonstrates isinstance assertion failure"""
    value = "123"
    assert isinstance(value, int), f"Expected int, got {type(value).__name__}"
