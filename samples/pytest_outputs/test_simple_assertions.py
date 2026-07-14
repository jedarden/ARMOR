"""
Simple assertion failures - basic pytest output examples
"""
import pytest


def test_simple_equality_fail():
    """Simple equality assertion failure"""
    a = 1
    b = 2
    assert a == b, f"Expected {a} to equal {b}"


def test_simple_truthy_fail():
    """Simple truthy assertion failure"""
    value = False
    assert value, "Expected value to be truthy"


def test_simple_in_fail():
    """Simple 'in' assertion failure"""
    item = "orange"
    collection = ["apple", "banana", "grape"]
    assert item in collection, f"{item} not found in {collection}"


def test_greater_than_fail():
    """Simple comparison failure"""
    score = 85
    threshold = 90
    assert score >= threshold, f"Score {score} below threshold {threshold}"


def test_type_fail():
    """Type check failure"""
    value = "123"
    assert isinstance(value, int), f"Expected int, got {type(value).__name__}"
