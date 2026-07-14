"""
Exception handling and edge case failures
"""
import pytest


def test_exception_not_raised():
    """Expected exception not raised"""
    with pytest.raises(ValueError):
        # This should raise ValueError but doesn't
        result = 10 / 2


def test_wrong_exception_type():
    """Different exception raised than expected"""
    with pytest.raises(ValueError):
        # This raises ZeroDivisionError, not ValueError
        result = 10 / 0


def test_exception_message_mismatch():
    """Exception raised but message doesn't match"""
    with pytest.raises(ValueError, match="specific error message"):
        raise ValueError("different error message")


def test_float_comparison_fail():
    """Floating point comparison failure"""
    expected = 0.1 + 0.2  # = 0.30000000000000004
    actual = 0.3
    assert actual == expected


def test_near_equal_float_fail():
    """Near equal assertion that still fails"""
    import math
    expected = math.pi
    actual = 3.14159
    assert actual == expected, "Pi values don't match"


def test_none_comparison_fail():
    """None check failure"""
    value = 0
    assert value is None, "Expected None"


def test_attribute_error_fail():
    """Attribute error in test"""
    obj = {"key": "value"}
    # This will raise AttributeError: 'dict' object has no attribute 'missing_attr'
    assert obj.missing_attr == "expected"


def test_unbound_variable_fail():
    """Unbound local variable reference"""
    try:
        x = undefined_variable
    except NameError:
        assert False, "Variable not defined"
