#!/usr/bin/env python3
"""
Example Usage of YAML File Reader

This demonstrates the various ways to use the YAML file reader functionality
in ARMOR.
"""

import sys
import os
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))

from internal.yamlutil import (
    YAMLFileReader,
    read_yaml_file,
    read_yaml_file_simple
)


def example_basic_reading():
    """Example 1: Basic YAML file reading"""
    print("=" * 60)
    print("Example 1: Basic YAML File Reading")
    print("=" * 60)

    # Create a sample YAML file
    sample_yaml = """
server:
  host: localhost
  port: 8080
  ssl:
    enabled: true
    cert_path: /etc/ssl/cert.pem

database:
  host: db.example.com
  port: 5432
  name: armor_db
  user: armor_user

features:
  - authentication
  - authorization
  - rate_limiting
  - monitoring
"""

    with open('/tmp/sample_config.yaml', 'w') as f:
        f.write(sample_yaml)

    # Read the file
    result = read_yaml_file('/tmp/sample_config.yaml')

    if result.success:
        print(f"✓ Successfully loaded configuration from: {result.filepath}")
        print(f"  Server: {result.data['server']['host']}:{result.data['server']['port']}")
        print(f"  Database: {result.data['database']['host']}:{result.data['database']['port']}")
        print(f"  Features: {', '.join(result.data['features'])}")
    else:
        print(f"✗ Failed to load configuration:")
        for error in result.errors:
            print(f"  - {error}")

    # Cleanup
    os.remove('/tmp/sample_config.yaml')
    print()


def example_simple_interface():
    """Example 2: Using the simple interface"""
    print("=" * 60)
    print("Example 2: Simple Interface (returns data or None)")
    print("=" * 60)

    # Create a sample YAML file
    with open('/tmp/simple_config.yaml', 'w') as f:
        f.write("app_name: ARMOR\nversion: 1.0.0\n")

    # Simple reading - returns data directly
    data = read_yaml_file_simple('/tmp/simple_config.yaml')

    if data:
        print(f"✓ App: {data['app_name']}")
        print(f"  Version: {data['version']}")
    else:
        print("✗ Failed to read file")

    # Cleanup
    os.remove('/tmp/simple_config.yaml')
    print()


def example_error_handling():
    """Example 3: Error handling for various failure scenarios"""
    print("=" * 60)
    print("Example 3: Error Handling")
    print("=" * 60)

    reader = YAMLFileReader()

    # Test 1: Non-existent file
    print("\n1. Reading non-existent file:")
    result = reader.read_file('/nonexistent/file.yaml')
    if not result.success:
        print(f"  ✓ Correctly detected error: {result.errors[0].message}")

    # Test 2: Invalid YAML syntax
    print("\n2. Reading file with invalid YAML:")
    with open('/tmp/invalid.yaml', 'w') as f:
        f.write("key:\n  valid: value\n    bad_indent: true\n")

    result = reader.read_file('/tmp/invalid.yaml')
    if not result.success:
        print(f"  ✓ Correctly detected syntax error")
        print(f"    Line: {result.errors[0].line}")
        print(f"    Message: {result.errors[0].message}")
        print(f"    Suggestion: {result.errors[0].suggestion}")

    os.remove('/tmp/invalid.yaml')
    print()


def example_multi_document():
    """Example 4: Multi-document YAML files"""
    print("=" * 60)
    print("Example 4: Multi-Document YAML Files")
    print("=" * 60)

    multi_doc_yaml = """---
document: 1
name: First Document
---
document: 2
name: Second Document
---
document: 3
name: Third Document
"""

    with open('/tmp/multi_doc.yaml', 'w') as f:
        f.write(multi_doc_yaml)

    # Read as multi-document
    result = read_yaml_file('/tmp/multi_doc.yaml', multi_document=True)

    if result.success:
        print(f"✓ Loaded {len(result.data)} documents")
        for i, doc in enumerate(result.data):
            print(f"  Document {i+1}: {doc['name']}")
    else:
        print(f"✗ Failed to load multi-document file")

    # Cleanup
    os.remove('/tmp/multi_doc.yaml')
    print()


def example_batch_processing():
    """Example 5: Reading multiple files at once"""
    print("=" * 60)
    print("Example 5: Batch Processing Multiple Files")
    print("=" * 60)

    # Create multiple config files
    configs = [
        ('web.yaml', {'service': 'web', 'port': 8080}),
        ('api.yaml', {'service': 'api', 'port': 8081}),
        ('db.yaml', {'service': 'database', 'port': 5432}),
    ]

    for filename, config in configs:
        with open(f'/tmp/{filename}', 'w') as f:
            f.write(f"service: {config['service']}\nport: {config['port']}\n")

    # Read all files
    reader = YAMLFileReader()
    filepaths = [f'/tmp/{name}' for name, _ in configs]
    results = reader.read_multiple_files(filepaths)

    print(f"✓ Processed {len(results)} files")
    for result in results:
        if result.success:
            print(f"  - {Path(result.filepath).name}: {result.data['service']} on port {result.data['port']}")
        else:
            print(f"  - {Path(result.filepath).name}: FAILED")

    # Cleanup
    for filename, _ in configs:
        os.remove(f'/tmp/{filename}')
    print()


def example_result_methods():
    """Example 6: Using YAMLReadResult helper methods"""
    print("=" * 60)
    print("Example 6: YAMLReadResult Helper Methods")
    print("=" * 60)

    # Create a file with a minor issue (trailing whitespace)
    with open('/tmp/config_with_issue.yaml', 'w') as f:
        f.write("key: value  \nother: data\n")

    result = read_yaml_file('/tmp/valid.yaml', multi_document=False)

    # Note: current implementation may not detect trailing whitespace as warning
    # This demonstrates the API structure
    print(f"Result object methods:")
    print(f"  - has_errors(): {result.has_errors()}")
    print(f"  - has_warnings(): {result.has_warnings()}")

    if result.success:
        print(f"  - get_data(): Returns parsed data")
        try:
            data = result.get_data()
            print(f"    Data: {data}")
        except RuntimeError as e:
            print(f"    Error getting data: {e}")

    print()


def main():
    """Run all examples"""
    print("\n" + "=" * 60)
    print("YAML File Reader Examples for ARMOR")
    print("=" * 60 + "\n")

    try:
        example_basic_reading()
        example_simple_interface()
        example_error_handling()
        example_multi_document()
        example_batch_processing()
        example_result_methods()

        print("=" * 60)
        print("All examples completed successfully!")
        print("=" * 60)

    except Exception as e:
        print(f"\n✗ Error running examples: {e}")
        import traceback
        traceback.print_exc()


if __name__ == '__main__':
    main()