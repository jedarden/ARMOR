"""
Comparison failures - numeric and string comparison examples
"""

def test_numeric_comparison():
    """Numeric comparison failure with >="""
    score = 85
    assert score >= 90, f"Score {score} is below threshold 90"


def test_numeric_range():
    """Numeric range check failure"""
    value = 150
    assert 0 <= value <= 100, f"Value {value} out of range [0, 100]"


def test_string_comparison():
    """String comparison failure"""
    result = "failure"
    assert result == "success", f"Got '{result}' instead of 'success'"


def test_float_comparison():
    """Floating point comparison failure"""
    result = 0.1 + 0.2  # = 0.30000000000000004
    expected = 0.3
    assert result == expected, f"{result} != {expected}"


def test_length_comparison():
    """Length comparison failure"""
    items = [1, 2, 3]
    assert len(items) >= 5, f"Collection has only {len(items)} items"


def test_dictionary_key_comparison():
    """Dictionary key existence failure"""
    data = {"name": "Alice", "age": 30}
    assert "email" in data, "Email field missing from data"
