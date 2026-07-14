"""
Exception and error failures - demonstrating error handling outputs
"""
import pytest


def test_raises_wrong_exception():
    """Expected to raise ValueError, but raises TypeError instead"""
    with pytest.raises(ValueError):
        raise TypeError("Wrong exception type")


def test_no_exception_raised():
    """Expected an exception but none was raised"""
    with pytest.raises(ValueError):
        x = 5 + 5  # No exception here


def test_unexpected_exception():
    """Exception was not expected at all"""
    result = 10 / 0  # ZeroDivisionError


def test_exception_message_mismatch():
    """Exception type matches but message doesn't"""
    with pytest.raises(ValueError, match="expected message"):
        raise ValueError("different message")


def test_exception_in_setup():
    """Demonstrates failure during test setup"""
    data = None
    if data is None:
        raise RuntimeError("Failed to setup test data")
    assert data is not None


def test_index_error():
    """Demonstrates index error failure"""
    items = [1, 2, 3]
    return items[10]  # IndexError
