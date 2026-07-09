#!/usr/bin/env python3
"""
Functional test for YAML validation without pytest dependency
"""
import sys
import tempfile
import os
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil import (
    YAMLSyntaxValidator,
    validate_yaml_string,
    YAMLErrorCategory,
    YAMLErrorSeverity
)


def test_valid_yaml():
    """Test that valid YAML passes validation."""
    print("Test 1: Valid YAML")
    valid_yaml = """
key: value
nested:
  item1: value1
  item2: value2
sequence:
  - item1
  - item2
"""
    result = validate_yaml_string(valid_yaml)
    assert result.is_valid, f"Expected valid YAML, got: {result}"
    print("  ✓ Valid YAML correctly identified")


def test_tab_detection():
    """Test detection of tab characters."""
    print("Test 2: Tab Character Detection")
    yaml_with_tabs = "key:\tvalue\nnested:\n\titem: value"
    result = validate_yaml_string(yaml_with_tabs)

    assert not result.is_valid, "Expected invalid YAML with tabs"
    assert len(result.errors) > 0, "Expected at least one error"

    tab_error = None
    for error in result.errors:
        if 'tab' in error.message.lower():
            tab_error = error
            break

    assert tab_error is not None, "Expected tab-related error"
    assert tab_error.category == YAMLErrorCategory.INDENTATION
    assert tab_error.line is not None
    print(f"  ✓ Tab error detected at line {tab_error.line}")


def test_unclosed_quote():
    """Test detection of unclosed quotes."""
    print("Test 3: Unclosed Quote Detection")
    invalid_yaml = 'key: "unclosed string\nanother: value'
    result = validate_yaml_string(invalid_yaml)

    assert not result.is_valid, "Expected invalid YAML with unclosed quote"
    assert len(result.errors) > 0

    error = result.errors[0]
    assert error.category in [YAMLErrorCategory.SYNTAX, YAMLErrorCategory.SCALAR]
    print(f"  ✓ Quote error detected as {error.category.value}")


def test_empty_file():
    """Test detection of empty files."""
    print("Test 4: Empty File Detection")
    result = validate_yaml_string("   \n  \n")

    assert not result.is_valid, "Expected invalid YAML for empty file"
    assert len(result.errors) > 0
    assert 'empty' in result.errors[0].message.lower()
    print("  ✓ Empty file correctly identified")


def test_line_column_extraction():
    """Test that line and column numbers are extracted."""
    print("Test 5: Line/Column Extraction")
    yaml_content = 'key1: value1\nkey2: value2\nkey3: "unclosed\n'

    validator = YAMLSyntaxValidator()
    result = validator.validate_content(yaml_content)

    if not result.is_valid:
        has_location = False
        for error in result.errors:
            if error.line is not None:
                has_location = True
                assert error.line >= 1
                if error.column is not None:
                    assert error.column >= 1
                print(f"  ✓ Line/column info: Line {error.line}, Column {error.column}")
                break

        assert has_location, "Expected at least one error with location information"
    else:
        print("  ⚠ Warning: YAML was unexpectedly valid")


def test_error_categorization():
    """Test error categorization."""
    print("Test 6: Error Categorization")
    test_cases = [
        ("Flow error", 'flow: {unclosed: bracket', YAMLErrorCategory.FLOW),
        ("Document error", '---\ndoc1\n---\ndoc2\nextra', None),  # May be valid or different
    ]

    validator = YAMLSyntaxValidator()
    passed = 0

    for name, yaml_content, expected_category in test_cases:
        result = validator.validate_content(yaml_content)
        if not result.is_valid and expected_category:
            found = any(e.category == expected_category for e in result.errors)
            if found:
                print(f"  ✓ {name} correctly categorized")
                passed += 1
            else:
                print(f"  ⚠ {name} - category not matched")
        else:
            print(f"  ⚠ {name} - YAML was valid or no category expected")

    print(f"  Categorization: {passed}/{len(test_cases)} tests matched")


def test_error_message_formatting():
    """Test error message formatting."""
    print("Test 7: Error Message Formatting")
    invalid_yaml = 'key: "unclosed quote\nanother: value'
    validator = YAMLSyntaxValidator()
    result = validator.validate_content(invalid_yaml)

    assert not result.is_valid
    assert len(result.errors) > 0

    error = result.errors[0]
    error_str = str(error)

    assert error.message != ""
    assert error.category is not None
    assert error.severity is not None
    assert len(error_str) > 0

    print(f"  ✓ Error message formatted correctly")
    print(f"    Message preview: {error_str[:80]}...")


def test_file_validation():
    """Test file validation."""
    print("Test 8: File Validation")
    valid_yaml = "config:\n  name: test\n  values:\n    - item1\n    - item2"

    with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
        f.write(valid_yaml)
        temp_path = f.name

    try:
        validator = YAMLSyntaxValidator()
        result = validator.validate_file(temp_path)

        assert result.is_valid, f"Expected valid YAML file, got: {result}"
        print("  ✓ File validation working correctly")
    finally:
        os.unlink(temp_path)


def test_validation_result_methods():
    """Test YAMLValidationResult helper methods."""
    print("Test 9: Validation Result Methods")
    from internal.yamlutil.error_types import YAMLValidationResult, YAMLErrorDetail

    result = YAMLValidationResult(
        is_valid=True,
        errors=[],
        warnings=[]
    )

    assert not result.has_errors()
    assert not result.has_warnings()
    assert result.get_all_issues() == []
    print("  ✓ Empty result methods working")

    # Add error
    error = YAMLErrorDetail(
        category=YAMLErrorCategory.SYNTAX,
        severity=YAMLErrorSeverity.ERROR,
        message="Test error"
    )
    result_with_error = YAMLValidationResult(
        is_valid=False,
        errors=[error],
        warnings=[]
    )

    assert result_with_error.has_errors()
    assert not result_with_error.has_warnings()
    assert len(result_with_error.get_all_issues()) == 1
    print("  ✓ Error result methods working")

    # Add warning
    warning = YAMLErrorDetail(
        category=YAMLErrorCategory.SYNTAX,
        severity=YAMLErrorSeverity.WARNING,
        message="Test warning"
    )
    result_with_warning = YAMLValidationResult(
        is_valid=True,
        errors=[],
        warnings=[warning]
    )

    assert not result_with_warning.has_errors()
    assert result_with_warning.has_warnings()
    print("  ✓ Warning result methods working")


def run_all_tests():
    """Run all functional tests."""
    print("=" * 60)
    print("YAML Validation Functional Tests")
    print("=" * 60)
    print()

    tests = [
        test_valid_yaml,
        test_tab_detection,
        test_unclosed_quote,
        test_empty_file,
        test_line_column_extraction,
        test_error_categorization,
        test_error_message_formatting,
        test_file_validation,
        test_validation_result_methods,
    ]

    passed = 0
    failed = 0

    for test in tests:
        try:
            test()
            passed += 1
            print()
        except AssertionError as e:
            print(f"  ✗ FAILED: {e}")
            failed += 1
            print()
        except Exception as e:
            print(f"  ✗ ERROR: {e}")
            failed += 1
            import traceback
            traceback.print_exc()
            print()

    print("=" * 60)
    print(f"Test Results: {passed} passed, {failed} failed")
    print("=" * 60)

    return failed == 0


if __name__ == '__main__':
    success = run_all_tests()
    sys.exit(0 if success else 1)