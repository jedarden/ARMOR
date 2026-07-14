"""
Multiline diffs - complex data structure comparisons
"""

def test_multiline_string_diff():
    """Multiline string comparison failure"""
    expected = """line one
line two
line three
line four"""

    actual = """line one
line TWO
line three
line five"""

    assert actual == expected


def test_list_comparison():
    """List comparison with multiple differences"""
    expected = [1, 2, 3, 4, 5]
    actual = [1, 2, 99, 4, 5]
    assert actual == expected


def test_dict_comparison():
    """Dictionary comparison with nested differences"""
    expected = {
        "name": "Alice",
        "age": 30,
        "city": "Boston",
        "active": True
    }
    actual = {
        "name": "Alice",
        "age": 31,
        "city": "New York",
        "active": True,
        "email": "alice@example.com"
    }
    assert actual == expected


def test_nested_structure_diff():
    """Nested dictionary/list structure comparison"""
    expected = {
        "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
        ],
        "count": 2
    }
    actual = {
        "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Robert"}
        ],
        "count": 2
    }
    assert actual == expected


def test_long_list_diff():
    """Long list with multiple differences scattered throughout"""
    expected = list(range(0, 100, 10))  # [0, 10, 20, 30, 40, 50, 60, 70, 80, 90]
    actual = [0, 10, 25, 30, 40, 55, 60, 70, 85, 90]
    assert actual == expected
