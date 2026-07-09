#!/usr/bin/env python3
"""
Verification script for YAML parser module structure.
Tests the module structure and API without requiring PyYAML.
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

def test_imports():
    """Test that all expected symbols can be imported."""
    print("Testing imports...")

    try:
        from result import ParseResult, ParseStatus
        print("✓ result module: ParseResult, ParseStatus imported successfully")
    except ImportError as e:
        print(f"✗ Failed to import from result module: {e}")
        return False

    try:
        # Test YAMLParser class (will fail at runtime if PyYAML is missing)
        from yaml_parser import YAMLParser
        print("✓ yaml_parser module: YAMLParser imported successfully")
    except ImportError as e:
        print(f"✗ Failed to import from yaml_parser module: {e}")
        return False

    try:
        from parse_module import YAMLParser, ParseResult, ParseStatus
        print("✓ parse_module package: All expected symbols exported")
    except ImportError as e:
        print(f"✗ Failed to import from parse_module package: {e}")
        return False

    return True


def test_result_structure():
    """Test ParseResult structure without parsing."""
    print("\nTesting ParseResult structure...")

    from result import ParseResult, ParseStatus

    # Test success result
    success = ParseResult.success({'test': 'data'})
    assert success.status == ParseStatus.SUCCESS
    assert success.data == {'test': 'data'}
    assert success.error is None
    assert success.is_success()
    assert not success.is_error()
    print("✓ ParseResult.success() creates correct structure")

    # Test error result
    error = ParseResult.error("Test error message")
    assert error.status == ParseStatus.ERROR
    assert error.data is None
    assert error.error == "Test error message"
    assert not error.is_success()
    assert error.is_error()
    print("✓ ParseResult.error() creates correct structure")

    # Test helper methods
    assert success.get_data() == {'test': 'data'}
    try:
        error.get_data()
        print("✗ error.get_data() should have raised RuntimeError")
        return False
    except RuntimeError:
        print("✓ error.get_data() raises RuntimeError correctly")

    assert error.get_error() == "Test error message"
    print("✓ get_error() returns correct error message")

    # Test string representation
    str_success = str(success)
    str_error = str(error)
    assert 'success' in str_success.lower()
    assert 'error' in str_error.lower()
    print("✓ String representations work correctly")

    return True


def test_api_contract():
    """Test that YAMLParser has the required API."""
    print("\nTesting YAMLParser API contract...")

    from yaml_parser import YAMLParser

    # Check class exists and has required methods
    assert hasattr(YAMLParser, '__init__')
    assert hasattr(YAMLParser, 'parse_string')
    assert hasattr(YAMLParser, 'parse_file')
    print("✓ YAMLParser has all required methods")

    # Check method signatures (basic)
    import inspect

    parse_string_sig = inspect.signature(YAMLParser.parse_string)
    assert 'yaml_content' in parse_string_sig.parameters
    print("✓ parse_string has 'yaml_content' parameter")

    parse_file_sig = inspect.signature(YAMLParser.parse_file)
    assert 'filepath' in parse_file_sig.parameters
    print("✓ parse_file has 'filepath' parameter")

    return True


def test_error_handling():
    """Test error handling without actual parsing."""
    print("\nTesting error handling infrastructure...")

    from result import ParseResult, ParseStatus

    # Test various error scenarios
    test_cases = [
        ("File not found", "File not found: /path/to/file.yaml"),
        ("Permission denied", "Permission denied: /protected/file.yaml"),
        ("Empty YAML", "Empty YAML content"),
        ("Invalid syntax", "YAML syntax error: ..."),
    ]

    for description, error_msg in test_cases:
        result = ParseResult.error(error_msg)
        assert result.status == ParseStatus.ERROR
        assert result.error == error_msg
        assert not result.is_success()
        assert result.is_error()
    print(f"✓ All {len(test_cases)} error cases handled correctly")

    return True


def test_file_structure():
    """Test that all required files exist."""
    print("\nTesting file structure...")

    required_files = [
        'yaml_parser.py',
        'result.py',
        '__init__.py',
        'tests/test_yaml_parser.py',
        'example_usage.py',
        'README.md'
    ]

    base_path = Path(__file__).parent
    for filename in required_files:
        filepath = base_path / filename
        if filepath.exists():
            print(f"✓ {filename} exists")
        else:
            print(f"✗ {filename} missing")
            return False

    return True


def main():
    """Run all verification tests."""
    print("=" * 60)
    print("YAML Parser Module - Structure Verification")
    print("=" * 60)

    all_passed = True

    all_passed &= test_file_structure()
    all_passed &= test_imports()
    all_passed &= test_result_structure()
    all_passed &= test_api_contract()
    all_passed &= test_error_handling()

    print("\n" + "=" * 60)
    if all_passed:
        print("✓ All verification tests PASSED")
        print("=" * 60)
        return 0
    else:
        print("✗ Some verification tests FAILED")
        print("=" * 60)
        return 1


if __name__ == '__main__':
    sys.exit(main())
