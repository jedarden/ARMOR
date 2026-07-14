"""
Simple assertion failures - basic pytest output examples
"""
import pytest

def test_simple_assertion_equal():
    """Simple == assertion failure"""
    a = 1
    b = 2
    assert a == b, f"Expected {a} to equal {b}"


def test_simple_assertion_true():
    """Simple truth assertion failure"""
    value = False
    assert value, "Expected value to be True"


def test_simple_assertion_in():
    """Simple 'in' assertion failure"""
    assert "orange" in ["apple", "banana", "cherry"]


def test_simple_assertion_is():
    """Simple 'is' assertion failure (identity)"""
    a = [1, 2, 3]
    b = [1, 2, 3]
    assert a is b, "Lists are not the same object"


def test_simple_assertion_type():
    """Simple type assertion failure"""
    value = "42"
    assert isinstance(value, int), f"Expected int, got {type(value)}"


def test_simple_assertion_raises():
    """Simple exception assertion - expected but not raised"""
    def no_error():
        return 42
    # This should raise ValueError but doesn't
    with pytest.raises(ValueError):
        no_error()
