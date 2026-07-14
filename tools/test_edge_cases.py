#!/usr/bin/env python3
"""
Test edge cases from pytest_patterns.md research.

This script tests the documented edge cases to ensure the parser handles them correctly.
"""

import sys
sys.path.insert(0, '/home/coding/ARMOR/tools')
from parse_pytest_output import PytestOutputParser
import json


def test_truncated_output():
    """Edge Case 1: Truncated output in Format 2."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_dict_equality ______________________________

    def test_dict_equality():
        expected = {"name": "Alice", "age": 30, "city": "NYC"}
        actual = {"name": "Bob", "age": 25, "city": "LA"}
>       assert actual == expected, f"Dictionaries don't match"
E       AssertionError: Dictionaries don't match
E       assert {'age': 25, '...ame': 'Bob'} == {'age': 30, '...ame': 'Alice'}
E
E         Differing items:
E         {'city': 'LA'} != {'city': 'NYC'}
E         {'age': 25} != {'age': 30}
E         {'name': 'Bob'} != {'name': 'Alice'}
E
E         Full diff:...
E
E         ...Full output truncated (14 lines hidden), use '-vv' to show

tools/test_pytest_flags_minimal.py:24: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert f.test_name == "test_dict_equality"
    assert f.error_message == "Dictionaries don't match"
    assert len(f.differing_items) == 3
    print("✅ Truncated output handled")


def test_ellipsis_values():
    """Edge Case 2: Ellipsis in values."""
    parser = PytestOutputParser()

    sample = """/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:44: AssertionError: assert '\\n    This i...ontent.\\n    ' == '\\n    This i... lines.\\n    '/"""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert '...' in f.error_message
    print("✅ Ellipsis in values handled")


def test_multiline_escapes():
    """Edge Case 3: Multiline string escapes."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_multiline _____________________________
tools/test_pytest_flags_minimal.py:44: in test_multiline
    assert actual_content == expected_content
E   AssertionError: Multiline content mismatch
E   assert '\\n    This i...ontent.\\n    ' == '\\n    This i... lines.\\n    '
E
E     -     lines.
E     +     content."""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert f.error_message == "Multiline content mismatch"
    assert 'lines.' in f.expected
    assert 'content.' in f.actual
    print("✅ Multiline string escapes handled")


def test_path_variations():
    """Edge Case 4: Path format variations."""
    parser = PytestOutputParser()

    relative_path = """tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'"""
    absolute_path = """/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'"""

    failures_rel = parser.parse(relative_path)
    failures_abs = parser.parse(absolute_path)

    assert len(failures_rel) == 1
    assert len(failures_abs) == 1
    assert failures_rel[0].test_file == "tools/test_pytest_flags_minimal.py"
    assert failures_abs[0].test_file == "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py"
    print("✅ Path format variations handled")


def test_floating_point_precision():
    """Edge Case 5: Floating-point precision."""
    parser = PytestOutputParser()

    sample = """/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:89: AssertionError: Floats don't match: 0.30000000000000004 != 0.3"""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert "0.30000000000000004" in f.error_message
    assert "0.3" in f.error_message
    print("✅ Floating-point precision handled")


def test_boolean_logic():
    """Edge Case 6: Boolean logic."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_boolean _____________________________
tools/test_pytest_flags_minimal.py:96: in test_boolean
>   assert (True and False)
E   assert (True and False)

tools/test_pytest_flags_minimal.py:96: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert f.assertion_line == "E   assert (True and False)"
    print("✅ Boolean logic handled")


def test_set_operations():
    """Edge Case 7: Set operations."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_set _____________________________
tools/test_pytest_flags_minimal.py:64: in test_set
>   assert set([1, 2, 3]) == set([1, 2, 3, 4, 5])
E   AssertionError: Sets should be equal
E   assert set([1, 2, 3]) == set([1, 2, 3, 4, 5])
E
E     Extra items in the left set: 4, 5
E     Extra items in the right set: """

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert f.assertion_type == "equality"
    print("✅ Set operations handled")


def test_type_checks():
    """Edge Case 8: Type checks with where clause."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_type _____________________________
tools/test_pytest_flags_minimal.py:108: in test_type
>   assert isinstance('123', int), f"Value should be int, got {type('123').__name__}"
E   AssertionError: Value should be int, got str
E   assert isinstance('123', int)
E
E   +  where False = isinstance('123', int)

tools/test_pytest_flags_minimal.py:108: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert f.assertion_type == "type_check"
    assert f.where_clause is not None
    print("✅ Type checks with where clause handled")


def test_range_diffs():
    """Edge Case 9: Range differences."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_range _____________________________
tools/test_pytest_flags_minimal.py:122: in test_range
>   assert range(5) == range(7)
E   AssertionError: Ranges should be equal
E   assert range(0, 5) == range(0, 7)
E
E   At index 5 diff: 5 != 6

tools/test_pytest_flags_minimal.py:122: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert f.index_diff == 5
    assert f.actual == "5"
    assert f.expected == "6"
    print("✅ Range differences handled")


def test_index_diffs():
    """Edge Case 10: Index diffs in list comparisons."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_list _____________________________
tools/test_pytest_flags_minimal.py:31: in test_list
>   assert list1 == list2, "Lists should be equal"
E   AssertionError: Lists should be equal
E   assert [1, 2, 3, 4, 5] == [1, 2, 10, 4, 5]
E
E     At index 2 diff: 3 != 10
E
E     Full diff:
E       [
E           1,
E           2,
E     -     10,
E     ?     ^^
E     +     3,
E     ?     ^
E           4,
E           5,
E       ]"""

    failures = parser.parse(sample)
    assert len(failures) == 1
    f = failures[0]
    assert f.index_diff == 2
    assert f.actual == "3"
    assert f.expected == "10"
    print("✅ Index diffs handled")


def main():
    """Run all edge case tests."""
    print("=" * 80)
    print("TESTING EDGE CASES FROM RESEARCH")
    print("=" * 80)

    tests = [
        ("Truncated Output", test_truncated_output),
        ("Ellipsis in Values", test_ellipsis_values),
        ("Multiline Escapes", test_multiline_escapes),
        ("Path Variations", test_path_variations),
        ("Floating-Point Precision", test_floating_point_precision),
        ("Boolean Logic", test_boolean_logic),
        ("Set Operations", test_set_operations),
        ("Type Checks", test_type_checks),
        ("Range Diffs", test_range_diffs),
        ("Index Diffs", test_index_diffs),
    ]

    passed = 0
    failed = 0

    for name, test_func in tests:
        try:
            print(f"\nTesting: {name}")
            print("-" * 80)
            test_func()
            passed += 1
        except Exception as e:
            print(f"❌ FAILED: {e}")
            failed += 1

    print("\n" + "=" * 80)
    print(f"EDGE CASE TESTING COMPLETE: {passed}/{len(tests)} passed")
    if failed > 0:
        print(f"⚠️  {failed} test(s) failed")
    print("=" * 80)

    return failed == 0


if __name__ == '__main__':
    success = main()
    sys.exit(0 if success else 1)
