"""
Shared fixtures for pytest samples
"""
import pytest


@pytest.fixture
def setup_database():
    """Fixture that simulates database setup"""
    print("\nSetting up database connection...")
    db_conn = {"connected": True, "host": "localhost"}
    yield db_conn
    print("\nClosing database connection...")


@pytest.fixture
def sample_data():
    """Fixture that provides sample test data"""
    return {
        "users": [
            {"id": 1, "name": "Alice"},
            {"id": 2, "name": "Bob"}
        ],
        "count": 2
    }


@pytest.fixture(scope="module")
def module_resource():
    """Module-scoped fixture that will fail"""
    print("\nModule setup starting...")
    if True:  # Simulate a failure condition
        raise RuntimeError("Module resource initialization failed")
    yield {"resource": "value"}


@pytest.fixture
def failing_teardown():
    """Fixture that fails during teardown"""
    data = {"temp": "data"}
    yield data
    print("\nTeardown error occurred!")
    raise Exception("Cleanup cannot complete")


def pytest_configure(config):
    """Pytest hook - demonstrates configuration-time failure"""
    # This runs at test collection time
    pass
