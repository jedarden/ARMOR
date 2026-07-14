#!/usr/bin/env python3
"""
Comprehensive Edge Case Tests for Pytest Output Parser

This script tests all edge cases documented in pytest_patterns.md
plus additional edge cases found in real pytest outputs.

Based on: bf-5x5xz1 (pytest_patterns.md research)
Author: ARMOR Project (bf-3dctxn)
Date: 2026-07-13
"""

import sys
sys.path.insert(0, '/home/coding/ARMOR/tools')
from parse_pytest_output import PytestOutputParser
import json


def test_format1_truncated_output():
    """Edge Case 1: Format 2 with truncated output (using -v without -vv)."""
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
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.test_name == "test_dict_equality"
    assert f.error_message == "Dictionaries don't match"
    assert len(f.differing_items) == 3, f"Expected 3 differing items, got {len(f.differing_items)}"
    print("✅ Format 1: Truncated output handled correctly")


def test_ellipsis_in_values():
    """Edge Case 2: Ellipsis in truncated values (Format 2 and 3)."""
    parser = PytestOutputParser()

    # Format 3 line format with ellipsis
    sample = """/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:44: AssertionError: assert '\\n    This i...ontent.\\n    ' == '\\n    This i... lines.\\n    '/"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert '...' in f.error_message, f"Expected '...' in error message: {f.error_message}"
    assert f.test_file == "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py"
    assert f.line_number == 44
    print("✅ Ellipsis in values handled correctly")


def test_multiline_string_escapes():
    """Edge Case 3: Multiline string escapes (\\n, \\t in string diffs)."""
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
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.error_message == "Multiline content mismatch"
    assert f.expected is not None and 'lines.' in f.expected, f"Expected 'lines.' in expected value: {f.expected}"
    assert f.actual is not None and 'content.' in f.actual, f"Expected 'content.' in actual value: {f.actual}"
    print("✅ Multiline string escapes handled correctly")


def test_path_format_variations():
    """Edge Case 4: Path format variations (relative vs absolute)."""
    parser = PytestOutputParser()

    relative_path = """tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'"""
    absolute_path = """/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:17: AssertionError: Expected 'world', got 'hello'"""

    failures_rel = parser.parse(relative_path)
    failures_abs = parser.parse(absolute_path)

    assert len(failures_rel) == 1, f"Expected 1 failure for relative path, got {len(failures_rel)}"
    assert len(failures_abs) == 1, f"Expected 1 failure for absolute path, got {len(failures_abs)}"
    assert failures_rel[0].test_file == "tools/test_pytest_flags_minimal.py"
    assert failures_abs[0].test_file == "/home/coding/ARMOR/tools/test_pytest_flags_minimal.py"
    assert failures_rel[0].line_number == 17
    assert failures_abs[0].line_number == 17
    print("✅ Path format variations handled correctly")


def test_whitespace_variations():
    """Edge Case 5: Whitespace variations (E   prefix vs no prefix)."""
    parser = PytestOutputParser()

    # Format 1/2 with E   prefix
    format_with_e = """_____________________________ test_simple _____________________________
tools/test.py:17: in test_simple
    assert greeting == "world"
E   AssertionError: Expected 'world', got 'hello'
E   assert 'hello' == 'world'
E
E     - world
E     + hello

tools/test.py:17: AssertionError"""

    # Format 3 without E prefix
    format_without_e = """tools/test.py:17: AssertionError: Expected 'world', got 'hello'"""

    failures_with_e = parser.parse(format_with_e)
    failures_without_e = parser.parse(format_without_e)

    assert len(failures_with_e) == 1, f"Expected 1 failure with E prefix, got {len(failures_with_e)}"
    assert len(failures_without_e) == 1, f"Expected 1 failure without E prefix, got {len(failures_without_e)}"
    assert failures_with_e[0].error_message == "Expected 'world', got 'hello'"
    assert failures_without_e[0].error_message == "Expected 'world', got 'hello'"
    print("✅ Whitespace variations handled correctly")


def test_floating_point_precision():
    """Edge Case 6: Floating-point precision issues."""
    parser = PytestOutputParser()

    sample = """/home/coding/ARMOR/tools/test_pytest_flags_minimal.py:89: AssertionError: Floats don't match: 0.30000000000000004 != 0.3"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert "0.30000000000000004" in f.error_message, f"Expected floating point value in message: {f.error_message}"
    assert "0.3" in f.error_message, f"Expected expected value in message: {f.error_message}"
    print("✅ Floating-point precision handled correctly")


def test_boolean_logic():
    """Edge Case 7: Boolean logic expressions without explicit expected/actual."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_boolean _____________________________
tools/test_pytest_flags_minimal.py:96: in test_boolean
>   assert (True and False)
E   assert (True and False)

tools/test_pytest_flags_minimal.py:96: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.assertion_line is not None, "Expected assertion line to be captured"
    assert "True and False" in f.assertion_line, f"Expected boolean expression in assertion: {f.assertion_line}"
    assert f.assertion_type == "boolean", f"Expected assertion_type 'boolean', got {f.assertion_type}"
    print("✅ Boolean logic handled correctly")


def test_keyerror():
    """Edge Case 8: KeyError exceptions (different from AssertionError)."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_key_error_dict ______________________________

    def test_key_error_dict():
        data = {"a": 1, "b": 2}
>       value = data["c"]  # KeyError
E       KeyError: 'c'

test_edge_cases.py:23: KeyError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.error_type == "KeyError", f"Expected error_type 'KeyError', got {f.error_type}"
    assert f.line_number == 23, f"Expected line_number 23, got {f.line_number}"
    assert "KeyError" in f.error_message, f"Expected 'KeyError' in error message: {f.error_message}"
    print("✅ KeyError handled correctly")


def test_attributeerror():
    """Edge Case 9: AttributeError exceptions (different from AssertionError)."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_attribute_error _____________________________

    def test_attribute_error():
        obj = type("Obj", (), {})()
>       assert obj.missing_attr == "value"  # AttributeError
E       AttributeError: 'Obj' object has no attribute 'missing_attr'

test_edge_cases.py:29: AttributeError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.error_type == "AttributeError", f"Expected error_type 'AttributeError', got {f.error_type}"
    assert f.line_number == 29, f"Expected line_number 29, got {f.line_number}"
    assert "AttributeError" in f.error_message, f"Expected 'AttributeError' in error message: {f.error_message}"
    print("✅ AttributeError handled correctly")


def test_none_assertion():
    """Edge Case 10: None assertions (e.g., from failed regex matches)."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_regex_match _____________________________
tools/test_edge_cases.py:62: in test_regex_match
>   assert re.match(pattern, text)
E       assert None
E        +  where None = <function match at 0x7f4a3b398360>('^admin_\\d+$', 'user_123')

tools/test_edge_cases.py:62: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.actual == "None", f"Expected actual value 'None', got {f.actual}"
    assert f.where_clause is not None, "Expected where_clause to be captured"
    print("✅ None assertion handled correctly")


def test_comparison_operators():
    """Edge Case 11: Comparison operators (>=, >, <, <=) beyond equality."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_greater_than _____________________________
tools/test_edge_cases.py:35: in test_greater_than
>   assert score >= 90
E       AssertionError: Score 75 is below passing threshold
E       assert 75 >= 90

tools/test_edge_cases.py:35: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.assertion_line is not None, "Expected assertion line to be captured"
    assert "75 >= 90" in f.assertion_line, f"Expected comparison in assertion: {f.assertion_line}"
    print("✅ Comparison operators handled correctly")


def test_set_operations():
    """Edge Case 12: Set operations with extra items."""
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
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.assertion_type == "equality", f"Expected assertion_type 'equality', got {f.assertion_type}"
    print("✅ Set operations handled correctly")


def test_type_checks_with_where():
    """Edge Case 13: Type checks with where clause."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_type _____________________________
tools/test_pytest_flags_minimal.py:108: in test_type
>   assert isinstance('123', int)
E       AssertionError: Value should be int, got str
E       assert isinstance('123', int)
E
E       +  where False = isinstance('123', int)

tools/test_pytest_flags_minimal.py:108: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.assertion_type == "type_check", f"Expected assertion_type 'type_check', got {f.assertion_type}"
    assert f.where_clause is not None, "Expected where_clause to be captured"
    assert "isinstance" in f.where_clause, f"Expected 'isinstance' in where clause: {f.where_clause}"
    print("✅ Type checks with where clause handled correctly")


def test_range_diffs():
    """Edge Case 14: Range differences with index diffs."""
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
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.index_diff == 5, f"Expected index_diff 5, got {f.index_diff}"
    assert f.actual == "5", f"Expected actual '5', got {f.actual}"
    assert f.expected == "6", f"Expected expected '6', got {f.expected}"
    print("✅ Range differences handled correctly")


def test_index_diffs():
    """Edge Case 15: Index diffs in list comparisons."""
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
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.index_diff == 2, f"Expected index_diff 2, got {f.index_diff}"
    assert f.actual == "3", f"Expected actual '3', got {f.actual}"
    assert f.expected == "10", f"Expected expected '10', got {f.expected}"
    print("✅ Index diffs handled correctly")


def test_dict_diffs():
    """Edge Case 16: Dictionary differences with key-value pairs."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_dict _____________________________
tools/test_pytest_flags_minimal.py:24: in test_dict
>   assert actual == expected, "Dictionaries don't match"
E   AssertionError: Dictionaries don't match
E   assert {'name': 'Bob', 'age': 25} == {'name': 'Alice', 'age': 30}
E
E     Differing items:
E     {'name': 'Bob'} != {'name': 'Alice'}
E     {'age': 25} != {'age': 30}"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert len(f.differing_items) >= 2, f"Expected at least 2 differing items, got {len(f.differing_items)}"
    print("✅ Dictionary differences handled correctly")


def test_complex_where_clauses():
    """Edge Case 17: Complex where clauses with function references."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_regex _____________________________
tools/test_edge_cases.py:62: in test_regex
>   assert re.match(pattern, text), f"Text '{text}' doesn't match pattern '{pattern}'"
E   AssertionError: Text 'user_123' doesn't match pattern '^admin_\\d+$'
E   assert None
E    +  where None = <function match at 0x7f4a3b398360>('^admin_\\d+$', 'user_123')
E    +    where <function match at 0x7f4a3b398360> = <module 're' from '/nix/store/.../lib/python3.12/re/__init__.py'>.match

tools/test_edge_cases.py:62: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.where_clause is not None, "Expected where_clause to be captured"
    assert len(f.where_clause) > 10, f"Expected substantial where clause, got: {f.where_clause}"
    print("✅ Complex where clauses handled correctly")


def test_custom_messages():
    """Edge Case 18: Custom detailed assertion messages."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_custom_message _____________________________
tools/test_edge_cases.py:45: in test_custom_message
>   assert user["age"] >= 18, (
        f"User {user['name']} is not old enough. "
        f"Age: {user['age']}, Required: 18"
    )
E   AssertionError: User John is not old enough. Age: 15, Required: 18
E   assert 15 >= 18

tools/test_edge_cases.py:45: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert "User John" in f.error_message, f"Expected user name in error message: {f.error_message}"
    assert "Age: 15" in f.error_message, f"Expected age in error message: {f.error_message}"
    print("✅ Custom messages handled correctly")


def test_empty_containers():
    """Edge Case 19: Empty container assertions with where clause."""
    parser = PytestOutputParser()

    sample = """_____________________________ test_empty _____________________________
tools/test_edge_cases.py:54: in test_empty
>   assert len(items) > 0, "Container should not be empty"
E   AssertionError: Container should not be empty
E   assert 0 > 0
E    +  where 0 = len([])

tools/test_edge_cases.py:54: AssertionError"""

    failures = parser.parse(sample)
    assert len(failures) == 1, f"Expected 1 failure, got {len(failures)}"
    f = failures[0]
    assert f.where_clause is not None, "Expected where_clause to be captured"
    assert "len([])" in f.where_clause, f"Expected 'len([])' in where clause: {f.where_clause}"
    print("✅ Empty containers handled correctly")


def main():
    """Run all comprehensive edge case tests."""
    print("=" * 80)
    print("COMPREHENSIVE EDGE CASE TESTING")
    print("Testing all edge cases from pytest_patterns.md research")
    print("=" * 80)

    tests = [
        # Edge cases from pytest_patterns.md
        ("Format 1: Truncated Output", test_format1_truncated_output),
        ("Ellipsis in Values", test_ellipsis_in_values),
        ("Multiline String Escapes", test_multiline_string_escapes),
        ("Path Format Variations", test_path_format_variations),
        ("Whitespace Variations", test_whitespace_variations),
        ("Floating-Point Precision", test_floating_point_precision),
        ("Boolean Logic", test_boolean_logic),
        ("Set Operations", test_set_operations),
        ("Type Checks with Where", test_type_checks_with_where),
        ("Range Diffs", test_range_diffs),
        ("Index Diffs", test_index_diffs),
        ("Dictionary Diffs", test_dict_diffs),

        # Additional edge cases from real outputs
        ("KeyError", test_keyerror),
        ("AttributeError", test_attributeerror),
        ("None Assertion", test_none_assertion),
        ("Comparison Operators", test_comparison_operators),
        ("Complex Where Clauses", test_complex_where_clauses),
        ("Custom Messages", test_custom_messages),
        ("Empty Containers", test_empty_containers),
    ]

    passed = 0
    failed = 0
    failures_list = []

    for name, test_func in tests:
        try:
            print(f"\nTesting: {name}")
            print("-" * 80)
            test_func()
            passed += 1
        except Exception as e:
            print(f"❌ FAILED: {e}")
            failed += 1
            failures_list.append((name, str(e)))

    print("\n" + "=" * 80)
    print(f"COMPREHENSIVE EDGE CASE TESTING COMPLETE: {passed}/{len(tests)} passed")
    if failed > 0:
        print(f"⚠️  {failed} test(s) failed:")
        for name, error in failures_list:
            print(f"  - {name}: {error}")
    print("=" * 80)

    return failed == 0


if __name__ == '__main__':
    success = main()
    sys.exit(0 if success else 1)
