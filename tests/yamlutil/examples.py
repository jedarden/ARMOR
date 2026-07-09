#!/usr/bin/env python3
"""
Examples demonstrating YAML validation functionality
"""
import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil import (
    YAMLSyntaxValidator,
    validate_yaml_file,
    validate_yaml_string,
    YAMLErrorCategory,
    YAMLErrorSeverity
)


def example_basic_validation():
    """Example: Basic YAML validation"""
    print("=== Example 1: Basic YAML Validation ===\n")

    # Valid YAML
    valid_yaml = """
config:
  name: my-app
  version: 1.0.0
  features:
    - authentication
    - logging
    - metrics
"""

    result = validate_yaml_string(valid_yaml)
    print(f"Valid YAML: {result}")
    print()

    # Invalid YAML (indentation error)
    invalid_yaml = """
config:
  name: my-app
    bad_indent: true
"""

    result = validate_yaml_string(invalid_yaml)
    print(f"Invalid YAML: {result}")
    if not result.is_valid:
        for error in result.errors:
            print(f"\nError Details:")
            print(f"  {error}")
    print()


def example_error_categorization():
    """Example: Error categorization and detailed reporting"""
    print("=== Example 2: Error Categorization ===\n")

    # YAML with tab characters
    yaml_with_tabs = """
database:
\turl: localhost
\tport: 5432
"""

    validator = YAMLSyntaxValidator()
    result = validator.validate_content(yaml_with_tabs)

    if not result.is_valid:
        for error in result.errors:
            print(f"Category: {error.category.value}")
            print(f"Severity: {error.severity.value}")
            print(f"Location: Line {error.line}, Column {error.column}")
            print(f"Message: {error.message}")
            print(f"Suggestion: {error.suggestion}")
            print()


def example_file_validation():
    """Example: Validating YAML files"""
    print("=== Example 3: File Validation ===\n")

    # Create example YAML file
    import tempfile
    import os

    yaml_content = """
# Application configuration
app:
  name: armor
  environment: production

services:
  - name: api
    port: 8080
  - name: metrics
    port: 9090
"""

    with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
        f.write(yaml_content)
        temp_path = f.name

    try:
        result = validate_yaml_file(temp_path)
        print(f"File validation result: {result}")

        if result.is_valid:
            print("✓ Configuration file is valid!")
        else:
            print("✗ Configuration file has errors:")
            for error in result.errors:
                print(f"  {error}")
    finally:
        os.unlink(temp_path)

    print()


def example_multiple_files():
    """Example: Batch validation of multiple files"""
    print("=== Example 4: Batch Validation ===\n")

    validator = YAMLSyntaxValidator()

    # Create temporary test files
    import tempfile
    import os

    test_files = []

    # Valid config
    valid_config = """
key1: value1
key2:
  nested: value2
"""

    # Invalid config (syntax error)
    invalid_config = """
key1: value1
key2: [unclosed list
"""

    # Create temp files
    for i, content in enumerate([valid_config, invalid_config]):
        with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
            f.write(content)
            test_files.append(f.name)

    try:
        results = validator.validate_multiple_files(test_files)

        print(f"Validated {len(results)} files:")
        for i, result in enumerate(results):
            status = "✓" if result.is_valid else "✗"
            print(f"  {status} File {i+1}: {result}")

            if not result.is_valid:
                for error in result.errors:
                    print(f"      Error at line {error.line}: {error.message[:50]}...")

    finally:
        # Clean up temp files
        for filepath in test_files:
            os.unlink(filepath)

    print()


def example_error_types():
    """Example: Different error types and their handling"""
    print("=== Example 5: Error Type Examples ===\n")

    test_cases = [
        ("Valid YAML", """
config:
  setting: value
""", True),

        ("Tab Character Error", """
key:\tvalue
""", False),

        ("Unclosed Quote", """
message: "hello world
""", False),

        ("Unclosed Flow Mapping", """
mapping: {key: value
""", False),

        ("Invalid Indentation", """
parent:
    child: value
""", False),
    ]

    validator = YAMLSyntaxValidator()

    for name, yaml_content, should_be_valid in test_cases:
        print(f"Test: {name}")
        result = validator.validate_content(yaml_content)

        if result.is_valid == should_be_valid:
            print(f"  ✓ Correctly identified as {'valid' if result.is_valid else 'invalid'}")
        else:
            print(f"  ✗ Incorrect validation result")

        if not result.is_valid:
            for error in result.errors:
                print(f"    Type: {error.category.value}")
                print(f"    Line {error.line}: {error.message[:60]}")
        print()


def example_integration_with_parser():
    """Example: Integration with existing YAML parser"""
    print("=== Example 6: Integration Example ===\n")

    # Show how the validation can be used alongside existing parsers
    yaml_content = """
# Test configuration
debug:
  enabled: true
  level: verbose
"""

    # Use new validator for detailed error checking
    validator = YAMLSyntaxValidator()
    validation_result = validator.validate_content(yaml_content)

    print(f"Validation result: {validation_result}")

    # Show detailed error information structure
    print("\nValidation provides:")
    print("  • Line/column numbers:", validation_result.is_valid or "N/A (valid YAML)")
    print("  • Error categorization:", "N/A (valid YAML)" if validation_result.is_valid else validation_result.errors[0].category.value)
    print("  • Helpful suggestions:", "N/A (valid YAML)" if validation_result.is_valid else validation_result.errors[0].suggestion)
    print("  • Pre-validation checks (tabs, trailing whitespace)")
    print("  • Severity levels for issues")

    print()


if __name__ == '__main__':
    print("YAML Validation Examples\n")
    print("=" * 60)
    print()

    try:
        example_basic_validation()
        example_error_categorization()
        example_file_validation()
        example_multiple_files()
        example_error_types()
        example_integration_with_parser()

        print("=" * 60)
        print("\nAll examples completed successfully!")

    except Exception as e:
        print(f"\nError running examples: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)