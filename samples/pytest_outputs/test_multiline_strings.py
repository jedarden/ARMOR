"""
Multiline string comparison failures - showing context diffs
"""
import pytest


def test_multiline_string_fail():
    """Multiline string comparison - typical output format diff"""
    expected = """Line 1: First line
Line 2: Second line
Line 3: Third line
Line 4: Fourth line
Line 5: Fifth line"""

    actual = """Line 1: First line
Line 2: Second line changed
Line 3: Third line
Line 4: Fourth line
Line 5: Fifth line"""

    assert actual == expected


def test_code_block_diff_fail():
    """Code/text block comparison failure"""
    expected = """
def hello_world():
    print("Hello, World!")
    return True

def goodbye():
    print("Goodbye!")
    return False
"""

    actual = """
def hello_world():
    print("Hello, World!")
    return True

def goodbye():
    print("Goodbye, World!")
    return False
"""

    assert actual == expected


def test_long_text_diff_fail():
    """Long text paragraph comparison"""
    expected = """
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud
exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.

Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident.
"""

    actual = """
Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor
incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud
exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.

Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu
fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa.
"""

    assert actual == expected


def test_json_like_diff_fail():
    """JSON-like structure comparison"""
    expected = """{
    "status": "success",
    "data": {
        "id": 123,
        "items": ["a", "b", "c"]
    }
}"""

    actual = """{
    "status": "success",
    "data": {
        "id": 123,
        "items": ["a", "b", "c", "d"]
    }
}"""

    assert actual == expected
