"""
Parameterized tests and fixture failures
"""
import pytest


# Parameterized test data
test_data = [
    ("case1", 10, 5, 15),
    ("case2", 20, 30, 50),  # This will fail - expected 50 but gets 50
    ("case3", 5, 5, 12),    # This will fail - expected 12 but gets 10
    ("case4", -5, 5, 0),
    ("case5", 100, 200, 300),
]


@pytest.mark.parametrize("name,a,b,expected", test_data)
def test_addition_cases(name, a, b, expected):
    """Parameterized addition test with some failures"""
    # Intentionally broken to cause failures
    result = a + b + 1  # Bug: adds extra 1
    assert result == expected, f"{name}: {a} + {b} = {result}, expected {expected}"


@pytest.fixture
def broken_database():
    """Fixture that simulates a broken database connection"""
    class MockDB:
        def __init__(self):
            self.connected = False

        def query(self, sql):
            raise ConnectionError("Database not connected")

    return MockDB()


def test_database_query(broken_database):
    """Test that should fail due to broken database"""
    result = broken_database.query("SELECT * FROM users")
    assert result is not None


@pytest.fixture
def setup_teardown_failure():
    """Fixture with setup failure"""
    print("\nSetting up...")
    # Simulate setup failure
    raise RuntimeError("Setup failed!")
    yield {"data": "test"}
    print("Tearing down...")


def test_with_bad_setup(setup_teardown_failure):
    """Test that will fail during fixture setup"""
    assert setup_teardown_failure["data"] == "test"


# Complex object comparison test
class Person:
    def __init__(self, name, age, email):
        self.name = name
        self.age = age
        self.email = email

    def __repr__(self):
        return f"Person(name={self.name!r}, age={self.age}, email={self.email!r})"


def test_complex_object_equality_fail():
    """Custom object comparison failure"""
    expected = Person("Alice", 30, "alice@example.com")
    actual = Person("Alice", 31, "alice@example.com")
    # This will fail because Person doesn't implement __eq__
    assert actual == expected, f"Objects don't match: {actual!r} != {expected!r}"


@pytest.mark.parametrize("input,expected", [
    ("hello", 5),
    ("world", 5),
    ("test", 4),
    ("longer string", 13),  # Wrong - should be 12
    ("emoji 😊", 7),        # Wrong - should be 7 including emoji
])
def test_string_length(input, expected):
    """Parameterized string length test"""
    # Intentionally count bytes instead of characters for failure
    result = len(input.encode('utf-8'))
    assert result == expected, f"Length of {input!r}: got {result}, expected {expected}"
