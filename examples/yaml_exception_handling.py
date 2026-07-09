#!/usr/bin/env python3
"""
Example: YAML Exception Handling

This example demonstrates how to use the custom YAML exception classes
for comprehensive error handling in YAML parsing operations.

Usage:
    python3 yaml_exception_handling.py
"""

import tempfile
import os
from pathlib import Path

# Add parent directory to path for imports
import sys
sys.path.insert(0, str(Path(__file__).parent.parent))

from internal.yamlutil import (
    YAMLFileReader,
    read_yaml_file,
    YAMLParserError,
    YAMLFileNotFoundError,
    YAMLSyntaxError,
    YAMLStructureError,
    YAMLEmptyFileError
)


def example_1_basic_usage():
    """Example 1: Basic usage with result pattern."""
    print("=" * 70)
    print("Example 1: Basic Usage with Result Pattern")
    print("=" * 70)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
        f.write("""
name: example-config
version: 1.0
settings:
  debug: true
  log_level: info
""")
        temp_path = f.name

    try:
        result = read_yaml_file(temp_path)

        if result.success:
            print(f"✓ Successfully loaded YAML from {temp_path}")
            print(f"  Data: {result.data}")
        else:
            print(f"✗ Failed to load YAML")
            for error in result.errors:
                print(f"  Error: {error}")
    finally:
        os.remove(temp_path)


def example_2_exception_pattern():
    """Example 2: Using exception pattern with raise_if_error."""
    print("\n" + "=" * 70)
    print("Example 2: Exception Pattern with raise_if_error")
    print("=" * 70)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
        f.write("""
config:
  host: localhost
  port: 8080
""")
        temp_path = f.name

    try:
        result = read_yaml_file(temp_path)
        result.raise_if_error()  # Raises exception if failed

        print(f"✓ Successfully loaded config")
        print(f"  Host: {result.data['config']['host']}")
        print(f"  Port: {result.data['config']['port']}")
    finally:
        os.remove(temp_path)


def example_3_file_not_found():
    """Example 3: Handling file not found errors."""
    print("\n" + "=" * 70)
    print("Example 3: Handling File Not Found Errors")
    print("=" * 70)

    result = read_yaml_file("/nonexistent/config.yaml")

    if not result.success:
        # Convert to exception
        exc = result.to_exception()

        if isinstance(exc, YAMLFileNotFoundError):
            print(f"✗ File not found: {exc.filepath}")
            print(f"  Message: {exc.message}")
            print("  → Please check the file path and try again")
        elif isinstance(exc, YAMLParserError):
            print(f"✗ YAML parsing error: {exc}")


def example_4_syntax_errors():
    """Example 4: Handling syntax errors with line/column info."""
    print("\n" + "=" * 70)
    print("Example 4: Handling Syntax Errors")
    print("=" * 70)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
        # Write YAML with syntax error (bad indentation)
        f.write("""
service:
  name: web
  port: 8080
    bad_indentation: true
""")
        temp_path = f.name

    try:
        result = read_yaml_file(temp_path)

        if not result.success:
            exc = result.to_exception()

            if isinstance(exc, YAMLSyntaxError):
                print(f"✗ Syntax error detected:")
                print(f"  File: {exc.filepath}")
                print(f"  Line: {exc.line}, Column: {exc.column}")
                print(f"  Message: {exc.message}")
                if exc.suggestion:
                    print(f"  Suggestion: {exc.suggestion}")
                if exc.context:
                    print(f"  Context:")
                    for line in exc.context.split('\n'):
                        print(f"    {line}")
    finally:
        os.remove(temp_path)


def example_5_empty_file():
    """Example 5: Handling empty file errors."""
    print("\n" + "=" * 70)
    print("Example 5: Handling Empty File Errors")
    print("=" * 70)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
        # Write nothing (empty file)
        temp_path = f.name

    try:
        result = read_yaml_file(temp_path)

        if not result.success:
            exc = result.to_exception()

            if isinstance(exc, YAMLEmptyFileError):
                print(f"✗ Empty file detected:")
                print(f"  File: {exc.filepath}")
                print(f"  Message: {exc.message}")
                print("  → Please add YAML content to the file")
    finally:
        os.remove(temp_path)


def example_6_catch_base_exception():
    """Example 6: Catching all YAML errors as base exception."""
    print("\n" + "=" * 70)
    print("Example 6: Catching All YAML Errors")
    print("=" * 70)

    error_cases = [
        ("/nonexistent/file.yaml", "File not found"),
        ("syntax_error.yaml", "Syntax error"),
        ("empty.yaml", "Empty file"),
    ]

    for filepath, description in error_cases:
        try:
            if filepath == "syntax_error.yaml":
                # Create file with syntax error
                with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
                    f.write("key:\n  value\n    bad: indent\n")
                    filepath = f.name
                    temp_to_remove = filepath
            elif filepath == "empty.yaml":
                # Create empty file
                with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
                    filepath = f.name
                    temp_to_remove = filepath
            else:
                temp_to_remove = None

            result = read_yaml_file(filepath)
            if not result.success:
                raise result.to_exception()

        except YAMLParserError as e:
            print(f"✗ {description}: {e.message}")
            if temp_to_remove:
                os.remove(temp_to_remove)


def example_7_usage_patterns():
    """Example 7: Common usage patterns."""
    print("\n" + "=" * 70)
    print("Example 7: Common Usage Patterns")
    print("=" * 70)

    # Pattern 1: Result check
    print("\nPattern 1: Result Check")
    result = read_yaml_file("/nonexistent/file.yaml")
    if result.success:
        data = result.data
    else:
        print(f"  ✗ Failed: {result.errors[0].message}")

    # Pattern 2: Exception with raise_if_error
    print("\nPattern 2: Exception with raise_if_error")
    try:
        result = read_yaml_file("/nonexistent/file.yaml")
        result.raise_if_error()
        data = result.data
    except YAMLParserError as e:
        print(f"  ✗ Caught: {e.message}")

    # Pattern 3: Specific exception handling
    print("\nPattern 3: Specific Exception Handling")
    try:
        result = read_yaml_file("/nonexistent/file.yaml")
        if not result.success:
            raise result.to_exception()
    except YAMLFileNotFoundError as e:
        print(f"  ✗ File not found: {e.filepath}")
    except YAMLSyntaxError as e:
        print(f"  ✗ Syntax error at line {e.line}")
    except YAMLParserError as e:
        print(f"  ✗ General YAML error: {e.message}")


def example_8_error_context():
    """Example 8: Using error context for debugging."""
    print("\n" + "=" * 70)
    print("Example 8: Error Context for Debugging")
    print("=" * 70)

    with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.yaml') as f:
        # Write YAML with unclosed quote
        f.write('key: "unclosed string\n')
        temp_path = f.name

    try:
        result = read_yaml_file(temp_path)

        if not result.success:
            print(f"✗ YAML parsing failed:")
            for error in result.errors:
                print(f"\n  Category: {error.category.value}")
                print(f"  Severity: {error.severity.value}")
                if error.line:
                    print(f"  Location: Line {error.line}, Column {error.column}")
                print(f"  Message: {error.message}")
                if error.context:
                    print(f"  Context:")
                    for line in error.context.split('\n'):
                        print(f"    {line}")
                if error.suggestion:
                    print(f"  Suggestion: {error.suggestion}")
    finally:
        os.remove(temp_path)


def example_9_multiple_files():
    """Example 9: Handling errors from multiple files."""
    print("\n" + "=" * 70)
    print("Example 9: Multiple File Validation")
    print("=" * 70)

    with tempfile.TemporaryDirectory() as tmpdir:
        # Create multiple YAML files
        files = {
            'valid.yaml': """
name: valid-config
value: 123
""",
            'invalid.yaml': """
key:
  bad_indent: value
    too_far: true
""",
            'empty.yaml': ""  # Empty file
        }

        file_paths = []
        for filename, content in files.items():
            path = os.path.join(tmpdir, filename)
            with open(path, 'w') as f:
                f.write(content)
            file_paths.append(path)

        reader = YAMLFileReader()
        results = reader.read_multiple_files(file_paths)

        valid_count = sum(1 for r in results if r.success)
        invalid_count = len(results) - valid_count

        print(f"\nProcessed {len(results)} files:")
        print(f"  ✓ Valid: {valid_count}")
        print(f"  ✗ Invalid: {invalid_count}")

        for result in results:
            filename = os.path.basename(result.filepath)
            if result.success:
                print(f"\n  ✓ {filename}: OK")
            else:
                print(f"\n  ✗ {filename}: FAILED")
                exc = result.to_exception()
                if isinstance(exc, YAMLSyntaxError):
                    print(f"    Syntax error at line {exc.line}")
                elif isinstance(exc, YAMLEmptyFileError):
                    print(f"    Empty file")
                elif isinstance(exc, YAMLParserError):
                    print(f"    {exc.message}")


def main():
    """Run all examples."""
    examples = [
        example_1_basic_usage,
        example_2_exception_pattern,
        example_3_file_not_found,
        example_4_syntax_errors,
        example_5_empty_file,
        example_6_catch_base_exception,
        example_7_usage_patterns,
        example_8_error_context,
        example_9_multiple_files,
    ]

    for example in examples:
        try:
            example()
        except Exception as e:
            print(f"\n✗ Example failed with unexpected error: {e}")

    print("\n" + "=" * 70)
    print("All examples completed!")
    print("=" * 70)


if __name__ == '__main__':
    main()
