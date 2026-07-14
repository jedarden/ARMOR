"""
Parametrize and fixture failures - demonstrating parameterized test outputs
"""
import pytest


@pytest.mark.parametrize("input,expected", [
    (1, 2),
    (2, 4),
    (3, 7),  # This will fail - expected 6
    (4, 8),
])
def test_parametrized_multiply(input, expected):
    """Demonstrates parametrized test failure"""
    assert input * 2 == expected, f"Failed for input={input}"


@pytest.fixture
def fragile_fixture():
    """A fixture that fails during setup"""
    raise RuntimeError("Fixture setup failed")
    yield {"value": 42}


def test_using_fixture(fragile_fixture):
    """This test will fail due to fixture error"""
    assert fragile_fixture["value"] == 42


@pytest.fixture
def fixture_with_cleanup_error():
    """Fixture that fails during teardown"""
    data = {"value": 100}
    yield data
    raise RuntimeError("Cleanup failed!")


def test_with_cleanup_error(fixture_with_cleanup_error):
    """Test passes but fixture teardown fails"""
    assert fixture_with_cleanup_error["value"] == 100


@pytest.mark.parametrize("x,y,expected", [
    ("a", "b", "ab"),
    ("hello", "world", "helloworld"),
    ("test", "ing", "wrong"),  # This will fail
])
def test_parametrized_strings(x, y, expected):
    """Demonstrates string concatenation failures"""
    result = x + y
    assert result == expected
