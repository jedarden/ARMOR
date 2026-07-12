#!/usr/bin/env python3
"""
Test suite for mixed scenario YAML comment detection.
Tests comments alongside values, anchors, and aliases.
"""

import sys
from pathlib import Path

# Add project root to path
sys.path.insert(0, str(Path(__file__).parent))

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


# ============================================================================
# Tests: Comments with Regular Values
# ============================================================================

def test_comments_with_string_values():
    """Test comments mixed with various string value types."""
    parser = YAMLCoreParser()
    yaml_content = """# Configuration header
name: test_config  # Configuration name
description: "A test config with values"  # Description
version: 1.0  # Version number
# Footer comments
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['name'] == 'test_config', "String value with comment should parse"
    assert result.data['description'] == 'A test config with values', "Quoted string with comment should parse"
    assert result.data['version'] == 1.0, "Numeric value with comment should parse"
    assert 'comment' not in str(result.data), "Comment text should not appear in data"


def test_comments_with_numeric_values():
    """Test comments mixed with numeric values (int, float, scientific notation)."""
    parser = YAMLCoreParser()
    yaml_content = """# Numeric configuration
integer: 42  # The answer
float_val: 3.14159  # Pi approximation
scientific: 1.5e-10  # Very small number
negative: -100  # Negative number
# End of numbers
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['integer'] == 42, "Integer with comment should parse"
    assert result.data['float_val'] == 3.14159, "Float with comment should parse"
    assert result.data['scientific'] == 1.5e-10, "Scientific notation with comment should parse"
    assert result.data['negative'] == -100, "Negative number with comment should parse"


def test_comments_with_boolean_values():
    """Test comments mixed with boolean values."""
    parser = YAMLCoreParser()
    yaml_content = """# Boolean flags
enabled: true  # Feature enabled
debug_mode: false  # Debugging disabled
production: True  # Capitalized boolean
# End of flags
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['enabled'] is True, "Boolean true with comment should parse"
    assert result.data['debug_mode'] is False, "Boolean false with comment should parse"
    assert result.data['production'] is True, "Capitalized boolean should parse"


def test_comments_with_null_values():
    """Test comments mixed with null/empty values."""
    parser = YAMLCoreParser()
    yaml_content = """# Null values
empty_field: null  # Explicitly null
unset_field: ~  # Tilde notation
blank_field:  # Empty after colon
# End of nulls
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['empty_field'] is None, "null with comment should parse"
    assert result.data['unset_field'] is None, "Tilde with comment should parse"
    assert result.data['blank_field'] is None, "Empty value with comment should parse"


def test_comments_with_list_values():
    """Test comments mixed with list/array values."""
    parser = YAMLCoreParser()
    yaml_content = """# List configuration
items:  # List of items
  - first  # First item
  - second  # Second item
  - third  # Third item
numbers: [1, 2, 3]  # Inline list
# End of lists
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['items'] == ['first', 'second', 'third'], "List with comments should parse"
    assert result.data['numbers'] == [1, 2, 3], "Inline list with comment should parse"


def test_comments_with_dict_values():
    """Test comments mixed with nested dictionary values."""
    parser = YAMLCoreParser()
    yaml_content = """# Nested configuration
server:  # Server settings
  host: localhost  # Server host
  port: 8080  # Server port
database:  # Database settings
  name: mydb  # Database name
  # End of database
# End of config
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['server']['host'] == 'localhost', "Nested dict with comments should parse"
    assert result.data['server']['port'] == 8080, "Nested port with comment should parse"
    assert result.data['database']['name'] == 'mydb', "Nested database with comments should parse"


# ============================================================================
# Tests: Comments with Anchors and Aliases
# ============================================================================

def test_comments_with_anchors_and_aliases():
    """Test comments alongside YAML anchors (&) and aliases (*)."""
    parser = YAMLCoreParser()
    yaml_content = """# Anchor definitions
defaults: &default  # Default configuration
  timeout: 30  # Timeout value
  retries: 3  # Retry count
# Alias usage
production:  # Production config
  <<: *default  # Inherit defaults
  host: prod.example.com  # Production host
development:  # Development config
  <<: *default  # Inherit defaults
  host: dev.example.com  # Development host
# End of configs
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['defaults']['timeout'] == 30, "Anchor with comment should define"
    assert result.data['production']['timeout'] == 30, "Alias should inherit anchor value"
    assert result.data['production']['host'] == 'prod.example.com', "Alias override should work"
    assert result.data['development']['timeout'] == 30, "Second alias should inherit anchor value"
    assert result.data['development']['host'] == 'dev.example.com', "Second alias override should work"


def test_comments_with_multiple_anchors():
    """Test comments with multiple anchor definitions and aliases."""
    parser = YAMLCoreParser()
    yaml_content = """# Base configurations
base_config: &base  # Base anchor
  enabled: true  # Enabled by default
  level: info  # Log level
# Extended configuration
extended_config: &extended  # Extended anchor
  <<: *base  # Inherit from base
  verbose: false  # Non-verbose
# Usage
config1:  # First usage
  <<: *extended  # Use extended
  name: config1  # Config name
config2:  # Second usage
  <<: *extended  # Use extended
  name: config2  # Another name
# End of configs
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['base_config']['enabled'] is True, "Base anchor should define value"
    assert result.data['extended_config']['enabled'] is True, "Extended should inherit from base"
    assert result.data['extended_config']['verbose'] is False, "Extended should have its own field"
    assert result.data['config1']['enabled'] is True, "Config1 should inherit through chain"
    assert result.data['config1']['verbose'] is False, "Config1 should inherit extended field"
    assert result.data['config2']['enabled'] is True, "Config2 should inherit through chain"


def test_comments_with_anchor_in_list():
    """Test comments with anchors used in list contexts."""
    parser = YAMLCoreParser()
    yaml_content = """# Anchor in list
item_template: &item  # Item template
  name: default  # Default name
  count: 0  # Default count
items:  # List of items
  - <<: *item  # First item using template
    name: item1  # Override name
  - <<: *item  # Second item using template
    name: item2  # Override name
  - <<: *item  # Third item using template
    name: item3  # Override name
# End of items
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    assert result.data['item_template']['name'] == 'default', "Template should be defined"
    assert len(result.data['items']) == 3, "List should have 3 items"
    assert result.data['items'][0]['name'] == 'item1', "First item should have override name"
    assert result.data['items'][0]['count'] == 0, "First item should inherit count"
    assert result.data['items'][1]['name'] == 'item2', "Second item should have override name"
    assert result.data['items'][2]['name'] == 'item3', "Third item should have override name"


def test_comments_with_complex_anchor_merge():
    """Test comments with complex anchor merging scenarios."""
    parser = YAMLCoreParser()
    yaml_content = """# Complex merge scenario
defaults: &defaults  # Defaults anchor
  timeout: 30  # Default timeout
  retries: 3  # Default retries
overrides: &overrides  # Overrides anchor
  timeout: 60  # Override timeout
  debug: true  # Debug flag
# Merge order matters
config:  # Final config
  <<: [*defaults, *overrides]  # Merge both anchors
  name: final  # Config name
# End of config
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Parsing should succeed"
    # In YAML merge, earlier anchors take precedence (defaults comes first)
    assert result.data['config']['timeout'] == 30, "First anchor in list wins in merge"
    assert result.data['config']['retries'] == 3, "First anchor value should be preserved"
    assert result.data['config']['debug'] is True, "Second anchor field should be added"
    assert result.data['config']['name'] == 'final', "Own field should be present"


# ============================================================================
# Tests: Comments with All Three Elements (Values, Comments, Anchors)
# ============================================================================

def test_comments_values_anchors_complete():
    """Test complex document with values, comments, and anchors all together."""
    parser = YAMLCoreParser()
    yaml_content = """# Server configuration file
# This file defines server settings

# Default server configuration
default_server: &server_config  # Anchor for server defaults
  host: localhost  # Server hostname
  port: 8080  # Server port
  ssl: false  # SSL disabled by default
  timeout: 30  # Connection timeout
  # End of defaults

# Production server
production:  # Production environment
  <<: *server_config  # Inherit default configuration
  host: prod.example.com  # Override host for production
  port: 443  # Use HTTPS port
  ssl: true  # Enable SSL in production
  # Production-specific settings
  workers: 8  # Number of worker processes
  # End of production config

# Development server
development:  # Development environment
  <<: *server_config  # Inherit default configuration
  host: dev.example.com  # Override host for development
  ssl: false  # Keep SSL disabled
  debug: true  # Enable debug mode
  # Development-specific settings
  workers: 2  # Fewer workers for development
  # End of development config

# Monitoring settings
monitoring:  # Monitoring configuration
  enabled: true  # Monitoring enabled
  interval: 60  # Check interval in seconds
  # No anchors here, just values
  endpoints:  # List of endpoints to monitor
    - /health  # Health check endpoint
    - /metrics  # Metrics endpoint
    - /status  # Status endpoint
  # End of monitoring config

# Global settings
settings:  # Global application settings
  log_level: info  # Logging level
  max_connections: 100  # Maximum concurrent connections
  # End of settings
# End of configuration file
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Complex document should parse successfully"

    # Verify default server anchor
    assert result.data['default_server']['host'] == 'localhost', "Default host should be set"
    assert result.data['default_server']['port'] == 8080, "Default port should be set"

    # Verify production inherits and overrides
    assert result.data['production']['host'] == 'prod.example.com', "Production should override host"
    assert result.data['production']['port'] == 443, "Production should override port"
    assert result.data['production']['ssl'] is True, "Production should enable SSL"
    assert result.data['production']['timeout'] == 30, "Production should inherit timeout"
    assert result.data['production']['workers'] == 8, "Production should have workers setting"

    # Verify development inherits and overrides
    assert result.data['development']['host'] == 'dev.example.com', "Development should override host"
    assert result.data['development']['ssl'] is False, "Development should keep SSL disabled"
    assert result.data['development']['debug'] is True, "Development should enable debug"
    assert result.data['development']['timeout'] == 30, "Development should inherit timeout"
    assert result.data['development']['workers'] == 2, "Development should have fewer workers"

    # Verify monitoring (no anchors)
    assert result.data['monitoring']['enabled'] is True, "Monitoring should be enabled"
    assert result.data['monitoring']['interval'] == 60, "Monitoring interval should be set"
    assert result.data['monitoring']['endpoints'] == ['/health', '/metrics', '/status'], "Monitoring endpoints should be list"

    # Verify global settings
    assert result.data['settings']['log_level'] == 'info', "Log level should be info"
    assert result.data['settings']['max_connections'] == 100, "Max connections should be 100"


def test_nested_structure_with_all_elements():
    """Test deeply nested structure with values, comments, and anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Application configuration
database: &db_config  # Database anchor
  primary:  # Primary database
    host: db1.example.com  # Primary host
    port: 5432  # Database port
    # Connection settings
    pool_size: 10  # Connection pool size
    timeout: 5  # Query timeout
  # Secondary database with same structure
  secondary:  # Secondary database
    host: db2.example.com  # Secondary host
    port: 5432  # Same port
    pool_size: 5  # Smaller pool for secondary
    timeout: 5  # Same timeout
# Cache configuration using similar pattern
cache:  # Cache settings
  redis:  # Redis configuration
    host: redis.example.com  # Redis host
    port: 6379  # Redis port
    # Redis settings
    max_memory: 1gb  # Maximum memory usage
    ttl: 3600  # Default TTL in seconds
  # Memcached configuration
  memcached:  # Memcached configuration
    host: memcached.example.com  # Memcached host
    port: 11211  # Memcached port
    ttl: 300  # Shorter TTL for memcached
# End of configuration
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Nested structure should parse successfully"

    # Verify database structure
    assert result.data['database']['primary']['host'] == 'db1.example.com', "Primary DB host should be set"
    assert result.data['database']['primary']['pool_size'] == 10, "Primary pool size should be set"
    assert result.data['database']['secondary']['host'] == 'db2.example.com', "Secondary DB host should be set"
    assert result.data['database']['secondary']['pool_size'] == 5, "Secondary pool should be smaller"

    # Verify cache structure
    assert result.data['cache']['redis']['host'] == 'redis.example.com', "Redis host should be set"
    assert result.data['cache']['redis']['ttl'] == 3600, "Redis TTL should be set"
    assert result.data['cache']['memcached']['host'] == 'memcached.example.com', "Memcached host should be set"
    assert result.data['cache']['memcached']['ttl'] == 300, "Memcached TTL should be shorter"


# ============================================================================
# Tests: Edge Cases of Mixed Content
# ============================================================================

def test_comment_between_anchor_and_alias():
    """Test comments placed between anchor definition and alias usage."""
    parser = YAMLCoreParser()
    yaml_content = """# Anchor definition
template: &tmpl  # Template anchor
  value: default  # Default value
  # Comment between anchor and alias
  flag: true  # Boolean flag
# Use the template after comments
instance1:  # First instance
  <<: *tmpl  # Use template
  value: overridden  # Override value
# More comments
instance2:  # Second instance
  <<: *tmpl  # Use template again
  # End of instances
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Comments between anchor and alias should not interfere"
    assert result.data['template']['value'] == 'default', "Template should define value"
    assert result.data['instance1']['flag'] is True, "Instance1 should inherit flag"
    assert result.data['instance1']['value'] == 'overridden', "Instance1 should override value"
    assert result.data['instance2']['flag'] is True, "Instance2 should inherit flag"
    assert result.data['instance2']['value'] == 'default', "Instance2 should keep default value"


def test_comment_in_multiline_value_with_anchor():
    """Test comments with multiline values alongside anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Multiline with anchor
description_template: &desc  # Description template
  text: >  # Multiline text marker
    This is a multiline
    description that spans
    multiple lines
  # End of multiline
# Use the template
item1:  # First item
  <<: *desc  # Inherit description
  name: item1  # Item name
# End of items
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Multiline with anchor should parse"
    assert 'multiline' in result.data['description_template']['text'], "Multiline should be preserved"
    assert result.data['item1']['text'] == result.data['description_template']['text'], "Item should inherit multiline"
    assert result.data['item1']['name'] == 'item1', "Item should have its own name"


def test_comment_with_special_characters_and_anchors():
    """Test comments with special characters in values alongside anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Special characters with anchors
template: &tmpl  # Template with special chars
  path: /usr/local/bin  # Unix path
  regex: "^[a-z]+$"  # Regex pattern
  url: "https://example.com?param=value&other=123"  # URL with ampersands
  # End of template
# Use template
config:  # Configuration
  <<: *tmpl  # Inherit template
  path: /opt/bin  # Override path
  # End of config
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Special characters with anchor should parse"
    assert result.data['template']['regex'] == '^[a-z]+$', "Regex should be preserved"
    assert 'ampersands' not in result.data['template']['url'], "Comment should not be in URL"
    assert result.data['config']['path'] == '/opt/bin', "Config should override path"
    assert result.data['config']['regex'] == '^[a-z]+$', "Config should inherit regex"


def test_empty_document_with_comments():
    """Test document with only comments and anchors but no values."""
    parser = YAMLCoreParser()
    yaml_content = """# Just a comment
# Another comment
# &anchor: alone  # Anchor without value is invalid
# More comments
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Document with only comments should parse as None"
    assert result.data is None, "Empty document should result in None"


def test_comment_at_end_of_value_before_anchor():
    """Test comment placement at end of value line before anchor."""
    parser = YAMLCoreParser()
    yaml_content = """# Value then anchor
defaults: &defaults  # Anchor definition
  timeout: 30  # Timeout value
  retries: 3  # Retry count
config:  # Config section
  <<: *defaults  # Use anchor
  name: test  # Config name
  # End of config
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Comment at end of value before anchor should parse"
    assert result.data['defaults']['timeout'] == 30, "Default timeout should be set"
    assert result.data['config']['timeout'] == 30, "Config should inherit timeout"


def test_comment_in_sequence_with_anchors():
    """Test comments within sequences that use anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Sequence with anchors
item: &item_template  # Item template
  id: 0  # Default ID
  active: true  # Active by default
items:  # List of items
  - <<: *item_template  # First item
    id: 1  # Set ID
    name: first  # Add name
  - <<: *item_template  # Second item
    id: 2  # Set ID
    name: second  # Add name
  # Comment in sequence
  - <<: *item_template  # Third item
    id: 3  # Set ID
    name: third  # Add name
# End of items
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Sequence with anchors and comments should parse"
    assert len(result.data['items']) == 3, "Sequence should have 3 items"
    assert result.data['items'][0]['id'] == 1, "First item should have ID 1"
    assert result.data['items'][0]['active'] is True, "First item should inherit active"
    assert result.data['items'][1]['id'] == 2, "Second item should have ID 2"
    assert result.data['items'][2]['id'] == 3, "Third item should have ID 3"


def test_comment_with_flow_style_and_anchors():
    """Test comments with flow-style syntax alongside anchors."""
    parser = YAMLCoreParser()
    yaml_content = """# Flow style with anchors
defaults: &defaults  # Default values
  values: [1, 2, 3]  # Flow array
  mapping: {key: value}  # Flow mapping
config:  # Config section
  <<: *defaults  # Inherit defaults
  values: [4, 5, 6]  # Override array
  # End of config
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Flow style with anchor should parse"
    assert result.data['defaults']['values'] == [1, 2, 3], "Flow array should parse"
    assert result.data['defaults']['mapping'] == {'key': 'value'}, "Flow mapping should parse"
    assert result.data['config']['values'] == [4, 5, 6], "Config should override flow array"
    assert result.data['config']['mapping'] == {'key': 'value'}, "Config should inherit mapping"


# ============================================================================
# Test Runner
# ============================================================================

def main():
    """Run all mixed scenario YAML comment tests."""
    print("Running Mixed Scenario YAML Comment Tests")
    print("=" * 60)

    tests = [
        # Comments with values
        ("Comments with string values", test_comments_with_string_values),
        ("Comments with numeric values", test_comments_with_numeric_values),
        ("Comments with boolean values", test_comments_with_boolean_values),
        ("Comments with null values", test_comments_with_null_values),
        ("Comments with list values", test_comments_with_list_values),
        ("Comments with dict values", test_comments_with_dict_values),

        # Comments with anchors and aliases
        ("Comments with anchors and aliases", test_comments_with_anchors_and_aliases),
        ("Comments with multiple anchors", test_comments_with_multiple_anchors),
        ("Comments with anchor in list", test_comments_with_anchor_in_list),
        ("Comments with complex anchor merge", test_comments_with_complex_anchor_merge),

        # All three elements together
        ("Complete document with all elements", test_comments_values_anchors_complete),
        ("Nested structure with all elements", test_nested_structure_with_all_elements),

        # Edge cases
        ("Comment between anchor and alias", test_comment_between_anchor_and_alias),
        ("Comment in multiline with anchor", test_comment_in_multiline_value_with_anchor),
        ("Comment with special chars and anchors", test_comment_with_special_characters_and_anchors),
        ("Empty document with comments", test_empty_document_with_comments),
        ("Comment at end of value before anchor", test_comment_at_end_of_value_before_anchor),
        ("Comment in sequence with anchors", test_comment_in_sequence_with_anchors),
        ("Comment with flow style and anchors", test_comment_with_flow_style_and_anchors),
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
        sys.exit(1)

    print("\n✓ All mixed scenario YAML comment tests passed!")
    sys.exit(0)


if __name__ == '__main__':
    main()
