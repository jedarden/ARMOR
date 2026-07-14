"""
Exception-related failures - unexpected exceptions and wrong exceptions
"""

def test_unexpected_exception():
    """Test that raises an unexpected exception"""
    def divide():
        return 10 / 0  # ZeroDivisionError
    divide()


def test_wrong_exception_type():
    """Test expecting one exception but getting another"""
    import pytest

    def raise_value_error():
        raise ValueError("Wrong error type")

    # Expect TypeError but get ValueError
    with pytest.raises(TypeError):
        raise_value_error()


def test_exception_message_mismatch():
    """Test with correct exception type but wrong message"""
    import pytest

    def raise_error():
        raise ValueError("Actual error message")

    # Message doesn't match
    with pytest.raises(ValueError, match="Expected pattern"):
        raise_error()


def test_exception_not_raised():
    """Test that should raise but doesn't (pytest.raises fails)"""
    import pytest

    def normal_function():
        return 42

    with pytest.raises(ValueError):
        normal_function()


def test_exception_in_fixture():
    """Exception raised in test setup/fixture"""
    import pytest

    @pytest.fixture
    def broken_setup():
        raise RuntimeError("Setup failed")

    def test_with_broken_fixture(broken_setup):
        # This never runs because fixture fails
        assert True


def test_assert_raises_context():
    """Using assertRaises incorrectly"""
    def my_function():
        return 42

    # assertRaises requires exception to be raised
    my_function()  # Should be inside assertRaises context
