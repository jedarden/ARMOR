"""
Edge cases and complex assertion failures
"""

def test_none_vs_false():
    """None vs False distinction"""
    value = None
    assert value is False, "None is not False"


def test_empty_vs_none():
    """Empty string vs None"""
    name = ""
    assert name is None, "Empty string is not None"


def test_falsy_values():
    """Various falsy value comparisons"""
    assert 0 == False, "0 is not equal to False in all contexts"
    assert [] == None, "Empty list is not None"


def test_object_equality_vs_identity():
    """Same value, different objects"""
    class Counter:
        def __init__(self, value):
            self.value = value
        def __eq__(self, other):
            return self.value == other.value

    a = Counter(5)
    b = Counter(5)
    assert a is b, "Objects are equal in value but not identical"


def test_custom_repr_failure():
    """Custom object with __repr__ causing confusing output"""
    class WeirdObject:
        def __init__(self, x):
            self.x = x
        def __repr__(self):
            return f"<WeirdObject: {self.x}>"

    obj = WeirdObject(42)
    assert obj.x == 100, f"WeirdObject value mismatch"


def test_recursion_limit():
    """Deep recursion causing assertion error"""
    def create_nested(depth):
        if depth == 0:
            return "end"
        return {"nested": create_nested(depth - 1)}

    shallow = create_nested(2)
    deep = create_nested(5)
    assert shallow == deep, "Nested structures differ"


def test_unicode_string():
    """Unicode string comparison issues"""
    expected = "café"
    actual = "café"  # Same visual, different bytes
    assert expected == actual


def test_bytes_vs_str():
    """Bytes vs string type confusion"""
    data = b"hello"
    assert data == "hello", "Bytes not equal to string"
