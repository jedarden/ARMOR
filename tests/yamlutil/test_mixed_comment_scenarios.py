#!/usr/bin/env python3
"""
Tests for YAML comment detection in mixed scenarios.

This module tests comment detection behavior when comments appear alongside:
- Regular YAML values
- YAML anchors (&) and aliases (*)
- All three elements together (values, comments, anchors)
- Edge cases of mixed content

These tests reflect the actual parser behavior and ensure that comments
are properly filtered in complex real-world YAML documents.
"""

import sys
from pathlib import Path

# Add project root to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent))

from internal.yamlutil.parser import YAMLCoreParser


def run_test(test_name, test_func):
    """Run a single test and report results."""
    try:
        test_func()
        print(f"✓ {test_name}")
        return True
    except AssertionError as e:
        print(f"✗ {test_name}: {e}")
        return False
    except Exception as e:
        print(f"✗ {test_name}: Unexpected error: {e}")
        return False


def test_comment_with_anchor_definition():
    """Test comments alongside anchor definitions."""
    parser = YAMLCoreParser()
    yaml_content = """# Default configuration anchor
defaults: &defaults
  timeout: 30  # Connection timeout in seconds
  retry: 3

# Another anchor
production: &prod
  env: production

service:
  <<: *defaults
  name: web
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['service']['timeout'] == 30, "Anchor merge should work"
    assert result.data['service']['retry'] == 3, "Anchor values should be preserved"
    assert result.data['production']['env'] == 'production', "Production anchor should work"


def test_comment_with_alias_usage():
    """Test comments alongside alias usage."""
    parser = YAMLCoreParser()
    yaml_content = """# Base configuration
base_config: &base
  timeout: 30

# Service configuration using alias
service1:
  <<: *base  # Merge base config
  name: service1

service2:
  <<: *base  # Also merge base config
  name: service2
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['service1']['timeout'] == 30, "Alias should expand correctly"
    assert result.data['service2']['timeout'] == 30, "Alias should expand correctly"
    assert 'comment' not in str(result.data).lower(), "Comments should be filtered"


def test_comment_in_complex_mixed_document():
    """Test comments in a complex document with values, comments, and anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Global defaults
defaults: &defaults
  timeout: 30
  retry: 3

# Database configuration
database:
  host: localhost
  port: 5432

# Services section
services:
  # Web service configuration
  web:
    <<: *defaults  # Inherit defaults
    port: 8080

  # API service configuration
  api:
    <<: *defaults  # Inherit defaults
    port: 8081

# Monitoring section
monitoring:
  enabled: true  # Enable monitoring
  interval: 60
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['services']['web']['timeout'] == 30, "Anchor merge should work in nested structure"
    assert result.data['services']['api']['retry'] == 3, "Anchor should merge correctly"
    assert result.data['monitoring']['enabled'] is True, "Boolean value should be preserved"
    assert result.data['monitoring']['interval'] == 60, "Integer value should be preserved"
    assert 'comment' not in str(result.data).lower(), "All comments should be filtered"


def test_comment_list_with_anchors():
    """Test comments in lists that use anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Item template
item_template: &item
  enabled: true
  timeout: 30

# List of items
items:
  # First item
  - <<: *item
    name: item1

  # Second item
  - <<: *item
    name: item2

  # Third item with override
  - <<: *item
    name: item3
    timeout: 60  # Override timeout
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['items'][0]['enabled'] is True, "First item should have enabled=true"
    assert result.data['items'][1]['name'] == 'item2', "Second item name should be preserved"
    assert result.data['items'][2]['timeout'] == 60, "Third item timeout override should work"


def test_comment_nested_mapping_with_anchors():
    """Test comments in nested mappings with anchor references."""
    parser = YAMLCoreParser()
    yaml_content = """# Server defaults
server_defaults: &server
  host: localhost
  port: 8080

# Environment configurations
environments:
  # Development environment
  development:
    server:
      <<: *server
      debug: true  # Enable debug mode

  # Production environment
  production:
    server:
      <<: *server
      debug: false  # Disable debug mode
      port: 80  # Production port
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['environments']['development']['server']['debug'] is True
    assert result.data['environments']['production']['server']['debug'] is False
    assert result.data['environments']['production']['server']['port'] == 80
    assert 'comment' not in str(result.data).lower()


def test_comment_multiple_anchors_and_aliases():
    """Test comments with multiple anchors and aliases in one document."""
    parser = YAMLCoreParser()
    yaml_content = """# First anchor definition
timeouts: &timeouts
  connect: 30
  read: 60

# Second anchor definition
retries: &retries
  max: 3
  backoff: 2

# Service using both anchors
service:
  <<: *timeouts  # Merge timeouts
  <<: *retries   # Merge retries
  name: myservice
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['service']['connect'] == 30, "Timeout values should merge"
    assert result.data['service']['max'] == 3, "Retry values should merge"
    assert result.data['service']['name'] == 'myservice', "Direct value should be preserved"


def test_comment_anchor_in_flow_style():
    """Test comments with anchors in flow-style collections."""
    parser = YAMLCoreParser()
    yaml_content = """# Anchor in flow sequence
seq: &default_seq [1, 2, 3]  # Default sequence

# Use the anchor
my_seq: *default_seq

# Anchor in flow mapping
map: &default_map {a: 1, b: 2}  # Default mapping

# Use the anchor
my_map: *default_map
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['my_seq'] == [1, 2, 3], "Flow sequence anchor should work"
    assert result.data['my_map'] == {'a': 1, 'b': 2}, "Flow mapping anchor should work"
    assert 'comment' not in str(result.data).lower()


def test_comment_multiline_with_anchors():
    """Test multiline strings and comments with anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Text template
text_template: &text
  description: |
    This is a multiline
    description that
    spans multiple lines

# Use the template
item1:
  <<: *text  # Inherit description
  name: item1

item2:
  <<: *text  # Also inherit description
  name: item2
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert 'multiline' in result.data['item1']['description'], "Multiline string should be preserved"
    assert 'spans multiple lines' in result.data['item2']['description'], "Multiline anchor should work"
    assert 'comment' not in str(result.data).lower()


def test_comment_edge_case_comment_between_anchor_and_alias():
    """Test edge case: comment between anchor definition and alias usage."""
    parser = YAMLCoreParser()
    yaml_content = """# Anchor definition
base: &base
  value: 100

# Comment between anchor and alias
# This comment should not affect the anchor resolution

target: *base  # Use the anchor
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['target']['value'] == 100, "Anchor should resolve despite intervening comments"


def test_comment_edge_case_comment_on_anchor_line():
    """Test edge case: comment on same line as anchor definition."""
    parser = YAMLCoreParser()
    yaml_content = """defaults: &defaults  # Default configuration values
  timeout: 30
  retry: 3

service:
  <<: *defaults  # Inherit from defaults
  name: web
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['service']['timeout'] == 30, "Anchor with inline comment should work"
    assert result.data['service']['retry'] == 3, "All anchor values should be preserved"
    assert 'comment' not in str(result.data['service']['timeout']).lower()


def test_comment_edge_case_comment_on_alias_line():
    """Test edge case: comment on same line as alias usage."""
    parser = YAMLCoreParser()
    yaml_content = """base: &base
  value: test

target:
  config: *base  # Reference to base configuration
  name: mytarget
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['target']['config']['value'] == 'test', "Alias with inline comment should work"
    assert 'comment' not in str(result.data['target']['config']).lower()


def test_comment_mixed_boolean_and_anchors():
    """Test comments with boolean values and anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Boolean configuration
bool_config: &bool_defaults
  enabled: true   # Feature enabled
  debug: false    # Debug disabled

# Use in different contexts
service:
  <<: *bool_defaults
  name: service1

worker:
  <<: *bool_defaults
  name: worker1
  debug: true  # Override debug setting
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['service']['enabled'] is True, "Boolean anchor value should preserve"
    assert result.data['service']['debug'] is False, "Boolean anchor value should preserve"
    assert result.data['worker']['debug'] is True, "Boolean override should work"
    assert 'comment' not in str(result.data).lower()


def test_comment_null_values_with_anchors():
    """Test comments with null/None values and anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Template with null values
template: &template
  optional1: null  # Can be empty
  optional2: ~     # Also null

# Use template
instance:
  <<: *template
  required: value
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['instance']['optional1'] is None, "Null value from anchor should be None"
    assert result.data['instance']['optional2'] is None, "Tilde null should be None"
    assert result.data['instance']['required'] == 'value', "Regular value should be preserved"
    assert 'comment' not in str(result.data).lower()


def test_comment_numeric_values_with_anchors():
    """Test comments with numeric values and anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Numeric defaults
numeric_defaults: &numeric_defaults
  integer: 42       # The answer
  float: 3.14       # Pi approximation
  negative: -10     # Negative number
  large: 100000     # Large number

# Use numeric template
config:
  <<: *numeric_defaults
  name: test
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['config']['integer'] == 42, "Integer anchor value should preserve"
    assert result.data['config']['float'] == 3.14, "Float anchor value should preserve"
    assert result.data['config']['negative'] == -10, "Negative number should preserve"
    assert result.data['config']['large'] == 100000, "Large number should preserve"
    assert 'comment' not in str(result.data).lower()


def test_comment_quoted_strings_with_anchors():
    """Test comments with quoted strings and anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# String defaults
string_defaults: &strings
  single: 'value with # hash'  # Single quoted
  double: "another # value"    # Double quoted

# Use string template
data:
  <<: *strings
  plain: plainvalue  # This is the value
  another: test # Hash after value (with space)
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '#' in result.data['data']['single'], "Hash in single quotes should be preserved"
    assert '#' in result.data['data']['double'], "Hash in double quotes should be preserved"
    assert result.data['data']['plain'] == 'plainvalue', "Plain value should be preserved"
    assert result.data['data']['another'] == 'test', "Hash after unquoted value (with space) creates a comment"
    assert 'comment' not in str(result.data).lower()


def test_comment_deeply_nested_with_anchors():
    """Test comments in deeply nested structures with anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Deep nesting template
deep_template: &deep
  level1:
    level2:
      level3:  # Third level
        value: deep_value
        another: nested_value

# Use template
structure:
  <<: *deep
  name: my_structure
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['structure']['level1']['level2']['level3']['value'] == 'deep_value'
    assert result.data['structure']['level1']['level2']['level3']['another'] == 'nested_value'
    assert result.data['structure']['name'] == 'my_structure'
    assert 'comment' not in str(result.data).lower()


def test_comment_empty_document_sections_with_anchors():
    """Test empty document sections with comments and anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Anchor definition
anchor: &anchor
  value: test

# Empty section below

# Another section
target: *anchor

# More empty space
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['target']['value'] == 'test', "Anchor should resolve across empty sections"


# ============================================================================
# Multi-line Context Tests
# ============================================================================

def test_multiline_literal_block_preserves_hash_symbols():
    """Test that literal block scalars (|) preserve # symbols as content."""
    parser = YAMLCoreParser()
    yaml_content = """description: |
  This is a literal block
  # This looks like a comment but is part of the string
  Another line
  # Another fake comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '# This looks like a comment' in result.data['description'], \
        "Literal block should preserve # symbols as content"
    assert '# Another fake comment' in result.data['description'], \
        "Multiple # symbols should be preserved in literal block"
    assert 'This is a literal block' in result.data['description'], \
        "Regular text should be preserved"


def test_multiline_folded_block_preserves_hash_symbols():
    """Test that folded block scalars (>) preserve # symbols as content."""
    parser = YAMLCoreParser()
    yaml_content = """description: >
  This is a folded block
  # This looks like a comment but is part of the string
  Another line
  # Another fake comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '# This looks like a comment' in result.data['description'], \
        "Folded block should preserve # symbols as content"
    assert '# Another fake comment' in result.data['description'], \
        "Multiple # symbols should be preserved in folded block"
    assert 'This is a folded block' in result.data['description'], \
        "Regular text should be preserved"


def test_multiline_real_comment_after_literal_block():
    """Test that real comments after literal blocks are filtered."""
    parser = YAMLCoreParser()
    yaml_content = """description: |
  First line
  Second line
# This is a real comment after block scalar
value: test  # Inline comment after value
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['description'] == 'First line\nSecond line\n', \
        "Literal block should not include trailing comment"
    assert result.data['value'] == 'test', \
        "Inline comment after value should be filtered"
    assert 'This is a real comment' not in str(result.data), \
        "Comment after block scalar should not appear in data"


def test_multiline_real_comment_before_literal_block():
    """Test that real comments before literal blocks are filtered."""
    parser = YAMLCoreParser()
    yaml_content = """# Comment before literal block
text1: |
  Line 1
  Line 2
# Comment after literal block
text2: >
  Line 3
  Line 4
# Comment after folded block
final: value
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['text1'] == 'Line 1\nLine 2\n', \
        "Literal block content should be preserved"
    assert result.data['text2'] == 'Line 3 Line 4\n', \
        "Folded block content should be preserved"
    assert result.data['final'] == 'value', \
        "Regular value should be preserved"
    assert 'Comment before literal block' not in str(result.data), \
        "Comment before block should be filtered"
    assert 'Comment after literal block' not in str(result.data), \
        "Comment after block should be filtered"


def test_multiline_indented_literal_block_with_hash_symbols():
    """Test that indented literal blocks preserve # symbols."""
    parser = YAMLCoreParser()
    yaml_content = """nested:
  # Comment before block scalar
  description: |
    Indented literal
    # Not a comment in string
  another: test
  # Comment after block scalar
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '# Not a comment in string' in result.data['nested']['description'], \
        "Indented literal block should preserve # symbols as content"
    assert result.data['nested']['another'] == 'test', \
        "Regular value should be preserved"
    assert 'Comment before block scalar' not in str(result.data), \
        "Comment before indented block should be filtered"
    assert 'Comment after block scalar' not in str(result.data), \
        "Comment after indented block should be filtered"


def test_multiline_indented_folded_block_with_hash_symbols():
    """Test that indented folded blocks preserve # symbols."""
    parser = YAMLCoreParser()
    yaml_content = """nested:
  # Comment before folded block
  description: >
    Indented folded
    # Not a comment in string
  another: test
  # Comment after folded block
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '# Not a comment in string' in result.data['nested']['description'], \
        "Indented folded block should preserve # symbols as content"
    assert result.data['nested']['another'] == 'test', \
        "Regular value should be preserved"


def test_multiline_literal_vs_plain_scalar_with_hash():
    """Test contrast between literal blocks and plain scalars with #."""
    parser = YAMLCoreParser()
    yaml_content = """# Plain scalar - # starts comment
plain: value # This is a comment, not part of value

# Literal block - # preserved as text
literal: |
  value # This is part of the value
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['plain'] == 'value', \
        "Plain scalar should not include comment text after #"
    assert '# This is part of the value' in result.data['literal'], \
        "Literal block should include # as text content"
    assert 'This is a comment' not in result.data['plain'], \
        "Comment after plain scalar should be filtered"


def test_multiline_folded_vs_plain_scalar_with_hash():
    """Test contrast between folded blocks and plain scalars with #."""
    parser = YAMLCoreParser()
    yaml_content = """# Plain scalar - # starts comment
plain: value # This is a comment, not part of value

# Folded block - # preserved as text
folded: >
  value # This is part of the value
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['plain'] == 'value', \
        "Plain scalar should not include comment text after #"
    assert '# This is part of the value' in result.data['folded'], \
        "Folded block should include # as text content"


def test_multiline_mixed_block_scalars_with_comments():
    """Test multiple block scalars with mixed content and real comments."""
    parser = YAMLCoreParser()
    yaml_content = """# Header comment
config:
  # Literal block with # symbols
  text1: |
    Line 1
    # Hash symbol 1
    Line 2
    # Hash symbol 2

  # Folded block with # symbols
  text2: >
    Folded line 1
    # Hash symbol 3
    Folded line 2
    # Hash symbol 4

  # Regular value
  regular: value  # Inline comment

# Footer comment
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '# Hash symbol 1' in result.data['config']['text1'], \
        "Literal block should preserve # symbols"
    assert '# Hash symbol 2' in result.data['config']['text1'], \
        "Literal block should preserve all # symbols"
    assert '# Hash symbol 3' in result.data['config']['text2'], \
        "Folded block should preserve # symbols"
    assert '# Hash symbol 4' in result.data['config']['text2'], \
        "Folded block should preserve all # symbols"
    assert result.data['config']['regular'] == 'value', \
        "Inline comment after value should be filtered"
    assert 'Header comment' not in str(result.data), \
        "Header comment should be filtered"
    assert 'Footer comment' not in str(result.data), \
        "Footer comment should be filtered"


def test_multiline_deeply_nested_block_scalars_with_hash():
    """Test deeply nested block scalars with # symbols."""
    parser = YAMLCoreParser()
    yaml_content = """level1:
  level2:
    level3:
      # Comment before nested literal block
      description: |
        Deep nested literal
        # Not a comment here
        # Another not a comment
      # Comment after nested literal block
      value: test
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '# Not a comment here' in result.data['level1']['level2']['level3']['description'], \
        "Deeply nested literal block should preserve # symbols"
    assert '# Another not a comment' in result.data['level1']['level2']['level3']['description'], \
        "All # symbols in nested literal block should be preserved"
    assert result.data['level1']['level2']['level3']['value'] == 'test', \
        "Nested regular value should be preserved"


def test_multiline_block_scalar_with_anchors_and_hash():
    """Test block scalars with anchors that contain # symbols."""
    parser = YAMLCoreParser()
    yaml_content = """# Text template with # symbols
text_template: &text
  description: |
    This is a multiline
    description with # hash symbols
    that spans multiple lines

# Use the template
item1:
  <<: *text  # Inherit description
  name: item1

item2:
  <<: *text  # Also inherit description
  name: item2
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert '# hash symbols' in result.data['item1']['description'], \
        "Literal block with anchor should preserve # symbols"
    assert 'that spans multiple lines' in result.data['item2']['description'], \
        "Anchor with literal block should work correctly"


def main():
    """Run all mixed scenario tests."""
    print("Running YAML Mixed Scenario Comment Tests")
    print("=" * 60)

    tests = [
        ("Comment with anchor definition", test_comment_with_anchor_definition),
        ("Comment with alias usage", test_comment_with_alias_usage),
        ("Comment in complex mixed document", test_comment_in_complex_mixed_document),
        ("Comment list with anchors", test_comment_list_with_anchors),
        ("Comment nested mapping with anchors", test_comment_nested_mapping_with_anchors),
        ("Comment multiple anchors and aliases", test_comment_multiple_anchors_and_aliases),
        ("Comment anchor in flow style", test_comment_anchor_in_flow_style),
        ("Comment multiline with anchors", test_comment_multiline_with_anchors),
        ("Edge case: comment between anchor and alias", test_comment_edge_case_comment_between_anchor_and_alias),
        ("Edge case: comment on anchor line", test_comment_edge_case_comment_on_anchor_line),
        ("Edge case: comment on alias line", test_comment_edge_case_comment_on_alias_line),
        ("Comment mixed boolean and anchors", test_comment_mixed_boolean_and_anchors),
        ("Comment null values with anchors", test_comment_null_values_with_anchors),
        ("Comment numeric values with anchors", test_comment_numeric_values_with_anchors),
        ("Comment quoted strings with anchors", test_comment_quoted_strings_with_anchors),
        ("Comment deeply nested with anchors", test_comment_deeply_nested_with_anchors),
        ("Comment empty document sections with anchors", test_comment_empty_document_sections_with_anchors),
        # Multi-line context tests
        ("Multi-line: literal block preserves # symbols", test_multiline_literal_block_preserves_hash_symbols),
        ("Multi-line: folded block preserves # symbols", test_multiline_folded_block_preserves_hash_symbols),
        ("Multi-line: real comment after literal block", test_multiline_real_comment_after_literal_block),
        ("Multi-line: real comment before literal block", test_multiline_real_comment_before_literal_block),
        ("Multi-line: indented literal block with # symbols", test_multiline_indented_literal_block_with_hash_symbols),
        ("Multi-line: indented folded block with # symbols", test_multiline_indented_folded_block_with_hash_symbols),
        ("Multi-line: literal vs plain scalar with #", test_multiline_literal_vs_plain_scalar_with_hash),
        ("Multi-line: folded vs plain scalar with #", test_multiline_folded_vs_plain_scalar_with_hash),
        ("Multi-line: mixed block scalars with comments", test_multiline_mixed_block_scalars_with_comments),
        ("Multi-line: deeply nested block scalars with #", test_multiline_deeply_nested_block_scalars_with_hash),
        ("Multi-line: block scalar with anchors and #", test_multiline_block_scalar_with_anchors_and_hash),
    ]

    passed = 0
    failed = 0

    for test_name, test_func in tests:
        if run_test(test_name, test_func):
            passed += 1
        else:
            failed += 1

    print("=" * 60)
    print(f"Results: {passed} passed, {failed} failed")

    if failed > 0:
        print("\n❌ Some tests failed")
        sys.exit(1)

    print("\n✅ All mixed scenario comment tests passed!")
    print("\nAcceptance criteria verified:")
    print("  ✓ Comments mixed with regular values are properly handled")
    print("  ✓ Comments with anchors and aliases work correctly")
    print("  ✓ Complex mixed scenarios parse successfully")
    print("  ✓ All tests reflect actual parser behavior")
    print("\nMulti-line context criteria verified:")
    print("  ✓ Comments in literal style multi-line strings (|)")
    print("  ✓ Comments in folded style multi-line strings (>)")
    print("  ✓ Comments in multi-line scalars")
    print("  ✓ Comments near block scalars with various indentation")
    sys.exit(0)


if __name__ == '__main__':
    main()
