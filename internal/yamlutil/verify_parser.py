#!/usr/bin/env python3
"""
Verification script for Core YAML Parser implementation.

Tests all acceptance criteria:
- Function accepts YAML string/file input
- Catches and processes YAML exceptions properly
- Returns error details when parsing fails
- Basic unit tests for error cases pass
- Ready for result structure integration
"""

import sys
from pathlib import Path

# Add parent directory to path
sys.path.insert(0, str(Path(__file__).parent.parent))

from yamlutil.parser import YAMLCoreParser, SafeLoadResult, safe_load_yaml
from yamlutil.error_types import YAMLErrorCategory, YAMLErrorSeverity


def print_test_header(test_name):
    """Print a test header."""
    print(f"\n{'='*60}")
    print(f"TEST: {test_name}")
    print('='*60)


def print_result(result):
    """Print test result."""
    status = "✓ PASS" if result else "✗ FAIL"
    print(f"{status}")


def test_parser_initialization():
    """Test 1: Parser initializes correctly."""
    print_test_header("Parser Initialization")

    try:
        parser = YAMLCoreParser()
        assert parser.yaml is not None, "PyYAML not imported"
        assert parser.ScannerError is not None, "ScannerError not available"
        assert parser.ParserError is not None, "ParserError not available"
        print("Parser initialized successfully")
        print(f"  - PyYAML: {parser.yaml}")
        print(f"  - ScannerError: {parser.ScannerError}")
        print(f"  - ParserError: {parser.ParserError}")
        print_result(True)
        return True
    except Exception as e:
        print(f"Failed to initialize parser: {e}")
        print_result(False)
        return False


def test_accepts_string_input():
    """Test 2: Function accepts YAML string input."""
    print_test_header("Accepts String Input")

    try:
        parser = YAMLCoreParser()
        yaml_content = """
key: value
number: 42
nested:
  item: test
"""
        result = parser.safe_load(yaml_content)

        assert result.is_success(), f"Parse failed: {result.error}"
        assert result.data['key'] == 'value', "Key value incorrect"
        assert result.data['number'] == 42, "Number value incorrect"

        print("Successfully parsed YAML string:")
        print(f"  - key: {result.data['key']}")
        print(f"  - number: {result.data['number']}")
        print(f"  - nested.item: {result.data['nested']['item']}")
        print_result(True)
        return True
    except Exception as e:
        print(f"String input test failed: {e}")
        print_result(False)
        return False


def test_catches_yaml_exceptions():
    """Test 3: Catches and processes YAML exceptions properly."""
    print_test_header("Catches YAML Exceptions")

    try:
        parser = YAMLCoreParser()

        # Test invalid YAML that triggers ScannerError
        invalid_yaml = """
key: value
  bad_indentation: here
    worse: indentation
"""
        result = parser.safe_load(invalid_yaml)

        assert result.is_error(), "Expected parsing to fail"
        assert result.error is not None, "Expected error detail"
        assert result.raw_exception is not None, "Expected raw exception"

        print("Successfully caught YAML exception:")
        print(f"  - Category: {result.error.category.value}")
        print(f"  - Severity: {result.error.severity.value}")
        print(f"  - Message: {result.error.message}")
        if result.error.line:
            print(f"  - Line: {result.error.line}")
        if result.error.suggestion:
            print(f"  - Suggestion: {result.error.suggestion}")
        print_result(True)
        return True
    except Exception as e:
        print(f"Exception handling test failed: {e}")
        import traceback
        traceback.print_exc()
        print_result(False)
        return False


def test_returns_error_details():
    """Test 4: Returns structured error details when parsing fails."""
    print_test_header("Returns Structured Error Details")

    try:
        parser = YAMLCoreParser()

        # Test empty content
        result = parser.safe_load("")
        assert result.is_error(), "Expected empty content to fail"
        assert result.error is not None, "Expected error detail"
        assert result.error.category == YAMLErrorCategory.DOCUMENT, \
            f"Expected DOCUMENT category, got {result.error.category}"
        print("Empty content error:")
        print(f"  - Category: {result.error.category.value}")
        print(f"  - Message: {result.error.message}")

        # Test invalid structure
        result2 = parser.safe_load("invalid: yaml: syntax:")
        assert result2.is_error(), "Expected invalid syntax to fail"
        assert result2.error is not None, "Expected error detail"
        print("\nInvalid syntax error:")
        print(f"  - Category: {result2.error.category.value}")
        print(f"  - Severity: {result2.error.severity.value}")

        print("\n✓ All error cases return structured details")
        print_result(True)
        return True
    except Exception as e:
        print(f"Error details test failed: {e}")
        import traceback
        traceback.print_exc()
        print_result(False)
        return False


def test_result_structure():
    """Test 5: Result structure integrates properly."""
    print_test_header("Result Structure Integration")

    try:
        # Test success result structure
        success_result = SafeLoadResult(success=True, data={'test': 'data'})
        assert success_result.is_success(), "is_success() should return True"
        assert not success_result.is_error(), "is_error() should return False"
        assert success_result.get_data() == {'test': 'data'}, "get_data() incorrect"

        print("Success result structure:")
        print(f"  - is_success(): {success_result.is_success()}")
        print(f"  - is_error(): {success_result.is_error()}")
        print(f"  - get_data(): {success_result.get_data()}")

        # Test error result structure
        from yamlutil.error_types import YAMLErrorDetail
        error = YAMLErrorDetail(
            category=YAMLErrorCategory.SYNTAX,
            severity=YAMLErrorSeverity.ERROR,
            message="Test error"
        )
        error_result = SafeLoadResult(success=False, error=error)
        assert error_result.is_error(), "is_error() should return True"
        assert error_result.get_error() == error, "get_error() incorrect"

        print("\nError result structure:")
        print(f"  - is_success(): {error_result.is_success()}")
        print(f"  - is_error(): {error_result.is_error()}")
        print(f"  - error.category: {error_result.get_error().category.value}")
        print(f"  - error.severity: {error_result.get_error().severity.value}")

        print("\n✓ Result structure integrates correctly")
        print_result(True)
        return True
    except Exception as e:
        print(f"Result structure test failed: {e}")
        import traceback
        traceback.print_exc()
        print_result(False)
        return False


def test_convenience_function():
    """Test 6: Convenience function works correctly."""
    print_test_header("Convenience Function")

    try:
        result = safe_load_yaml("key: value")
        assert result.is_success(), "Convenience function failed"
        assert result.data['key'] == 'value', "Incorrect data"

        print("Convenience function works:")
        print(f"  - safe_load_yaml('key: value')")
        print(f"  - Result: {result.data}")
        print_result(True)
        return True
    except Exception as e:
        print(f"Convenience function test failed: {e}")
        print_result(False)
        return False


def test_input_validation():
    """Test 7: Input validation."""
    print_test_header("Input Validation")

    try:
        parser = YAMLCoreParser()

        # Test None input
        result = parser.safe_load(None)
        assert result.is_error(), "None input should fail"
        print("✓ None input rejected")

        # Test non-string input
        result = parser.safe_load(12345)
        assert result.is_error(), "Non-string input should fail"
        print("✓ Non-string input rejected")

        # Test whitespace only
        result = parser.safe_load("   \n  \n  ")
        assert result.is_error(), "Whitespace-only should fail"
        print("✓ Whitespace-only input rejected")

        print("\n✓ All input validation working")
        print_result(True)
        return True
    except Exception as e:
        print(f"Input validation test failed: {e}")
        print_result(False)
        return False


def run_all_tests():
    """Run all verification tests."""
    print("\n" + "="*60)
    print("YAML CORE PARSER VERIFICATION")
    print("="*60)
    print("\nTesting acceptance criteria:")
    print("1. Function accepts YAML string input")
    print("2. Catches and processes YAML exceptions properly")
    print("3. Returns error details when parsing fails")
    print("4. Basic unit tests for error cases")
    print("5. Ready for result structure integration")

    tests = [
        ("Parser Initialization", test_parser_initialization),
        ("Accepts String Input", test_accepts_string_input),
        ("Catches YAML Exceptions", test_catches_yaml_exceptions),
        ("Returns Error Details", test_returns_error_details),
        ("Result Structure", test_result_structure),
        ("Convenience Function", test_convenience_function),
        ("Input Validation", test_input_validation),
    ]

    results = []
    for name, test_func in tests:
        try:
            passed = test_func()
            results.append((name, passed))
        except Exception as e:
            print(f"\n✗ Test '{name}' crashed: {e}")
            import traceback
            traceback.print_exc()
            results.append((name, False))

    # Summary
    print("\n" + "="*60)
    print("SUMMARY")
    print("="*60)

    passed_count = sum(1 for _, passed in results if passed)
    total_count = len(results)

    for name, passed in results:
        status = "✓ PASS" if passed else "✗ FAIL"
        print(f"{status}: {name}")

    print(f"\nTotal: {passed_count}/{total_count} tests passed")

    if passed_count == total_count:
        print("\n🎉 All acceptance criteria verified!")
        return 0
    else:
        print(f"\n⚠️  {total_count - passed_count} test(s) failed")
        return 1


if __name__ == '__main__':
    sys.exit(run_all_tests())
