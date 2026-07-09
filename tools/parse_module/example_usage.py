#!/usr/bin/env python3
"""
Example usage of the YAML Parser utility module.

Demonstrates basic usage patterns and error handling.
"""

from pathlib import Path
import sys

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent))

from yaml_parser import YAMLParser, ParseResult


def example_basic_usage():
    """Example: Basic YAML string parsing."""
    print("=" * 50)
    print("Example 1: Basic YAML String Parsing")
    print("=" * 50)

    yaml_content = """
database:
  host: localhost
  port: 5432
  name: mydb
  credentials:
    user: admin
    password: secret
"""

    parser = YAMLParser()
    result = parser.parse_string(yaml_content)

    if result.is_success():
        print("✓ Successfully parsed YAML:")
        print(f"  Database: {result.data['database']['name']}")
        print(f"  Host: {result.data['database']['host']}")
        print(f"  Port: {result.data['database']['port']}")
    else:
        print(f"✗ Error: {result.error}")


def example_file_parsing():
    """Example: Parsing a YAML file."""
    print("\n" + "=" * 50)
    print("Example 2: YAML File Parsing")
    print("=" * 50)

    # Create a temporary YAML file
    yaml_content = """
app:
  name: MyApp
  version: 1.0.0
  debug: true

features:
  - authentication
  - logging
  - monitoring
"""

    import tempfile
    with tempfile.NamedTemporaryFile(mode='w', suffix='.yaml', delete=False) as f:
        f.write(yaml_content)
        temp_path = f.name

    try:
        parser = YAMLParser()
        result = parser.parse_file(temp_path)

        if result.is_success():
            print("✓ Successfully parsed YAML file:")
            print(f"  App: {result.data['app']['name']}")
            print(f"  Version: {result.data['app']['version']}")
            print(f"  Features: {', '.join(result.data['features'])}")
        else:
            print(f"✗ Error: {result.error}")
    finally:
        import os
        os.unlink(temp_path)


def example_error_handling():
    """Example: Error handling for invalid YAML."""
    print("\n" + "=" * 50)
    print("Example 3: Error Handling")
    print("=" * 50)

    # Example 1: Invalid YAML syntax
    invalid_yaml = """
key: value
  bad_indentation: here
    worse: structure
"""

    parser = YAMLParser()
    result = parser.parse_string(invalid_yaml)

    if result.is_error():
        print("✓ Caught invalid YAML syntax:")
        print(f"  Error: {result.error}")

    # Example 2: File not found
    print("\nTesting file not found:")
    result = parser.parse_file('/nonexistent/path/config.yaml')

    if result.is_error():
        print("✓ Caught missing file:")
        print(f"  Error: {result.error}")

    # Example 3: Empty content
    print("\nTesting empty content:")
    result = parser.parse_string("")

    if result.is_error():
        print("✓ Caught empty content:")
        print(f"  Error: {result.error}")


def example_result_inspection():
    """Example: Inspecting ParseResult objects."""
    print("\n" + "=" * 50)
    print("Example 4: Result Object Inspection")
    print("=" * 50)

    yaml_content = "test: value"
    parser = YAMLParser()
    result = parser.parse_string(yaml_content)

    print(f"Status: {result.status}")
    print(f"Is Success: {result.is_success()}")
    print(f"Is Error: {result.is_error()}")
    print(f"Data: {result.data}")
    print(f"Error: {result.error}")


def example_multiple_configs():
    """Example: Parsing multiple configuration files."""
    print("\n" + "=" * 50)
    print("Example 5: Multiple Configuration Files")
    print("=" * 50)

    configs = [
        ("Database config", """
host: localhost
port: 5432
name: users_db
"""),
        ("Cache config", """
host: localhost
port: 6379
db: 0
"""),
        ("API config", """
base_url: https://api.example.com
timeout: 30
retry: 3
""")
    ]

    parser = YAMLParser()
    results = []

    for config_name, yaml_content in configs:
        result = parser.parse_string(yaml_content)
        results.append((config_name, result))

    print("Parsing results:")
    for config_name, result in results:
        if result.is_success():
            print(f"✓ {config_name}: Loaded successfully")
        else:
            print(f"✗ {config_name}: {result.error}")


def main():
    """Run all examples."""
    print("\nYAML Parser Utility Module - Examples\n")

    example_basic_usage()
    example_file_parsing()
    example_error_handling()
    example_result_inspection()
    example_multiple_configs()

    print("\n" + "=" * 50)
    print("Examples completed!")
    print("=" * 50 + "\n")


if __name__ == '__main__':
    main()
