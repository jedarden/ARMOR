#!/usr/bin/env python3
"""Basic test for YAML parser implementation."""

from internal.yamlutil.parser import YAMLCoreParser, SafeLoadResult, safe_load_yaml

# Test basic parsing
print("=== Test 1: Basic YAML parsing ===")
parser = YAMLCoreParser()
result = parser.safe_load('key: value')
print(f"Success: {result.success}")
print(f"Data: {result.data}")
print()

# Test error case
print("=== Test 2: Invalid YAML (indentation error) ===")
result2 = parser.safe_load('key:\n  bad_indent: value\n    too_far: true')
print(f"Success: {result2.success}")
print(f"Error category: {result2.error.category if result2.error else 'N/A'}")
print(f"Error message: {result2.error.message if result2.error else 'N/A'}")
print(f"Line: {result2.error.line if result2.error else 'N/A'}")
print(f"Suggestion: {result2.error.suggestion if result2.error else 'N/A'}")
print()

# Test convenience function
print("=== Test 3: Convenience function ===")
result3 = safe_load_yaml('test: data\nnumber: 42')
print(f"Success: {result3.success}")
print(f"Data: {result3.data}")
print()

# Test empty content
print("=== Test 4: Empty content ===")
result4 = parser.safe_load('')
print(f"Success: {result4.success}")
print(f"Error: {result4.error.message if result4.error else 'N/A'}")

print("\nAll basic tests passed!")
