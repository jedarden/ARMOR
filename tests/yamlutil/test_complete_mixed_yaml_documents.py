#!/usr/bin/env python3
"""
Tests for complete YAML documents with mixed comment scenarios.

This module tests comment filtering behavior in complete, realistic YAML documents:
- Complete document filtering from start to end
- Comments in complex nested structures
- Document boundary comments (headers and footers)

These tests verify that comments are properly filtered throughout entire
documents, including at document boundaries and within deeply nested structures.

Bead: bf-12nez
Acceptance Criteria:
- Test function for complete document filtering
- Test function for nested structure comments
- Test function for document boundary comments
- All tests pass
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


# ============================================================================
# Complete Document Filtering Tests
# ============================================================================

def test_complete_document_with_all_comment_types():
    """Test filtering of all comment types in a complete realistic document."""
    parser = YAMLCoreParser()
    yaml_content = """# ============================================================
# Application Configuration File
# ============================================================
# This file contains the complete configuration for the application
# including database, cache, server, and monitoring settings.

# Database Configuration Section
database:
  # Primary database connection settings
  primary:
    host: db1.example.com  # Database server hostname
    port: 5432  # PostgreSQL default port
    name: production_db  # Database name
    pool_size: 20  # Connection pool size
    timeout: 30  # Query timeout in seconds

  # Replica database connection settings
  replica:
    host: db2.example.com  # Replica server hostname
    port: 5432  # Same port as primary
    name: production_db  # Same database name
    pool_size: 10  # Smaller pool for replica
    readonly: true  # Read-only access

# Cache Configuration Section
cache:
  # Redis configuration
  redis:
    host: redis.example.com  # Redis server hostname
    port: 6379  # Redis default port
    db: 0  # Redis database number
    ttl: 3600  # Default TTL in seconds (1 hour)

  # Memcached configuration
  memcached:
    host: memcached.example.com  # Memcached server hostname
    port: 11211  # Memcached default port
    ttl: 300  # Default TTL in seconds (5 minutes)

# Server Configuration Section
server:
  host: 0.0.0.0  # Listen on all interfaces
  port: 8080  # Application port
  workers: 8  # Number of worker processes
  timeout: 60  # Request timeout in seconds

  # SSL/TLS Configuration
  ssl:
    enabled: true  # Enable SSL
    cert: /path/to/cert.pem  # SSL certificate path
    key: /path/to/key.pem  # SSL private key path

# Monitoring Configuration Section
monitoring:
  # Metrics collection
  metrics:
    enabled: true  # Enable metrics collection
    port: 9090  # Metrics endpoint port
    path: /metrics  # Metrics endpoint path

  # Health checks
  health:
    enabled: true  # Enable health checks
    interval: 30  # Health check interval in seconds
    timeout: 5  # Health check timeout

# Logging Configuration Section
logging:
  level: info  # Log level (debug, info, warn, error)
  format: json  # Log format (json, text)
  output: /var/log/app.log  # Log file path

# ============================================================
# End of Configuration File
# ============================================================
# Last updated: 2026-07-13
# Version: 1.0.0
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Complete document parsing should succeed"

    # Verify all data was extracted correctly
    assert result.data['database']['primary']['host'] == 'db1.example.com'
    assert result.data['database']['primary']['port'] == 5432
    assert result.data['database']['replica']['readonly'] is True
    assert result.data['cache']['redis']['ttl'] == 3600
    assert result.data['cache']['memcached']['port'] == 11211
    assert result.data['server']['workers'] == 8
    assert result.data['server']['ssl']['enabled'] is True
    assert result.data['monitoring']['metrics']['enabled'] is True
    assert result.data['monitoring']['health']['interval'] == 30
    assert result.data['logging']['level'] == 'info'

    # Verify no comments leaked into data
    data_str = str(result.data).lower()
    assert 'application configuration file' not in data_str
    assert 'database configuration section' not in data_str
    assert 'primary database connection' not in data_str
    assert 'end of configuration file' not in data_str
    assert 'last updated' not in data_str


def test_complete_document_with_anchors_and_comments():
    """Test complete document with anchors, aliases, and comments mixed throughout."""
    parser = YAMLCoreParser()
    yaml_content = """# ============================================================
# Multi-Environment Configuration
# ============================================================

# Default configuration template
defaults: &default_config
  timeout: 30  # Default timeout value
  retries: 3  # Default retry count
  backoff: 2  # Exponential backoff multiplier

# Development environment configuration
development:
  <<: *default_config  # Inherit defaults
  environment: development  # Environment name
  debug: true  # Enable debug mode
  verbose: true  # Enable verbose logging
  timeout: 60  # Longer timeout for development
  # Development-specific database
  database:
    host: localhost  # Local database
    port: 5432  # Default PostgreSQL port
    name: dev_db  # Development database name

# Production environment configuration
production:
  <<: *default_config  # Inherit defaults
  environment: production  # Environment name
  debug: false  # Disable debug mode
  verbose: false  # Disable verbose logging
  timeout: 15  # Shorter timeout for production
  # Production-specific database
  database:
    host: db.production.example.com  # Production database
    port: 5432  # Default PostgreSQL port
    name: prod_db  # Production database name
    ssl: true  # Enable SSL for database connections

# Staging environment configuration
staging:
  <<: *default_config  # Inherit defaults
  environment: staging  # Environment name
  debug: false  # Disable debug mode
  verbose: true  # Keep verbose logging for debugging
  timeout: 30  # Keep default timeout
  # Staging-specific database
  database:
    host: db.staging.example.com  # Staging database
    port: 5432  # Default PostgreSQL port
    name: staging_db  # Staging database name
    ssl: true  # Enable SSL for database connections

# ============================================================
# End of Multi-Environment Configuration
# ============================================================
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Complete document with anchors should parse successfully"

    # Verify all environments inherited defaults correctly
    assert result.data['development']['retries'] == 3
    assert result.data['development']['timeout'] == 60  # Override
    assert result.data['development']['debug'] is True
    assert result.data['development']['database']['host'] == 'localhost'

    assert result.data['production']['retries'] == 3
    assert result.data['production']['timeout'] == 15  # Override
    assert result.data['production']['debug'] is False
    assert result.data['production']['database']['ssl'] is True

    assert result.data['staging']['retries'] == 3
    assert result.data['staging']['timeout'] == 30  # Default
    assert result.data['staging']['debug'] is False
    assert result.data['staging']['verbose'] is True
    assert result.data['staging']['database']['ssl'] is True

    # Verify no comment text in data
    data_str = str(result.data).lower()
    assert 'multi-environment configuration' not in data_str
    assert 'default configuration template' not in data_str
    assert 'development-specific' not in data_str
    assert 'end of multi-environment' not in data_str


def test_complete_document_with_multiline_values_and_comments():
    """Test complete document with multiline strings, block scalars, and comments."""
    parser = YAMLCoreParser()
    yaml_content = """# ============================================================
# Documentation and Help Text Configuration
# ============================================================

# Application metadata
metadata:
  name: MyApp  # Application name
  version: 1.0.0  # Semantic version
  author: Development Team  # Author information

  # Long description using literal block scalar
  description: |
    This application is a comprehensive solution for managing
    complex data workflows with support for multiple data sources,
    transformation pipelines, and output formats.

    Key features:
    - Multi-source data ingestion
    - Real-time data processing
    - Flexible output formats
    - Comprehensive monitoring

  # Short tagline using folded block scalar
  tagline: >
    The ultimate data management solution
    for modern enterprise needs.

# Help documentation
help:
  # Introduction text
  introduction: |
    Welcome to MyApp! This help guide will walk you through
    the core concepts and features of the application.

  # Getting started guide
  getting_started: |
    To get started with MyApp, follow these steps:

    1. Install the application
    2. Configure your data sources
    3. Create your first pipeline
    4. Run and monitor your pipeline

  # FAQ section
  faq:
    - question: How do I install MyApp?
      answer: |
        Install MyApp using your package manager:
        pip install myapp

    - question: How do I configure data sources?
      answer: |
        Edit the configuration file and add your
        data source credentials in the sources section.

    - question: How do I monitor pipelines?
      answer: |
        Use the monitoring dashboard or API endpoints
        to check pipeline status and metrics.

# Error messages
errors:
  # Connection error message
  connection_failed: |
    Failed to connect to the server. Please check:
    - Server hostname and port
    - Network connectivity
    - Firewall settings
    - Authentication credentials

  # Authentication error message
  auth_failed: >
    Authentication failed. Please verify your credentials
    and ensure you have the necessary permissions.

# ============================================================
# End of Documentation Configuration
# ============================================================
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Complete document with multiline values should parse successfully"

    # Verify metadata
    assert result.data['metadata']['name'] == 'MyApp'
    assert result.data['metadata']['version'] == '1.0.0'

    # Verify multiline descriptions are preserved
    assert 'Key features:' in result.data['metadata']['description']
    assert 'Real-time data processing' in result.data['metadata']['description']
    assert 'modern enterprise needs' in result.data['metadata']['tagline']

    # Verify help documentation
    assert 'Welcome to MyApp!' in result.data['help']['introduction']
    assert 'Install the application' in result.data['help']['getting_started']

    # Verify FAQ
    assert len(result.data['help']['faq']) == 3
    assert result.data['help']['faq'][0]['question'] == 'How do I install MyApp?'
    assert 'pip install myapp' in result.data['help']['faq'][0]['answer']

    # Verify error messages
    assert 'Firewall settings' in result.data['errors']['connection_failed']
    assert 'Authentication failed' in result.data['errors']['auth_failed']

    # Verify comments were filtered
    data_str = str(result.data).lower()
    assert 'documentation and help text' not in data_str
    assert 'application metadata' not in data_str
    assert 'long description using' not in data_str
    assert 'end of documentation' not in data_str


# ============================================================================
# Nested Structure Comments Tests
# ============================================================================

def test_deeply_nested_structure_with_comments():
    """Test comments in deeply nested structures across multiple levels."""
    parser = YAMLCoreParser()
    yaml_content = """# Level 1 comment
level1:
  # Level 1a - nested structure
  level1a:
    # Level 2 comment
    level2:
      # Level 2a comment
      level2a:
        # Level 3 comment
        level3:
          # Level 3a comment
          level3a:
            # Level 4 comment
            value1: deep1  # Deep value 1
            value2: deep2  # Deep value 2
          # End of level 3a
        # End of level 3
      # End of level 2a
    # End of level 2
  # Level 1b - another nested structure
  level1b:
    # Alternative path
    level2b:
      # Level 2b comment
      level3b:
        # Level 3b comment
        value3: deep3  # Deep value 3
        value4: deep4  # Deep value 4
# End of level 1
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Deeply nested structure should parse successfully"

    # Verify deep values are accessible
    assert result.data['level1']['level1a']['level2']['level2a']['level3']['level3a']['value1'] == 'deep1'
    assert result.data['level1']['level1a']['level2']['level2a']['level3']['level3a']['value2'] == 'deep2'
    assert result.data['level1']['level1b']['level2b']['level3b']['value3'] == 'deep3'
    assert result.data['level1']['level1b']['level2b']['level3b']['value4'] == 'deep4'

    # Verify comments were filtered
    data_str = str(result.data).lower()
    assert 'level 1 comment' not in data_str
    assert 'level 2 comment' not in data_str
    assert 'level 3 comment' not in data_str
    assert 'level 4 comment' not in data_str
    assert 'end of level' not in data_str


def test_nested_sequences_with_comments():
    """Test comments in nested sequences and complex data structures."""
    parser = YAMLCoreParser()
    yaml_content = """# Top-level sequence configuration
items:
  # First item group
  - name: item1  # First item name
    config:  # Item 1 configuration
      enabled: true  # Enabled
      timeout: 30  # Timeout value
    nested:  # Nested data
      - subitem1  # Sub-item 1
      - subitem2  # Sub-item 2
      # End of sub-items

  # Second item group
  - name: item2  # Second item name
    config:  # Item 2 configuration
      enabled: false  # Disabled
      timeout: 60  # Longer timeout
    nested:  # Nested data
      - subitem3  # Sub-item 3
      - subitem4  # Sub-item 4

  # Third item group with deeper nesting
  - name: item3  # Third item name
    config:  # Item 3 configuration
      enabled: true  # Enabled
      advanced:  # Advanced settings
        deep:  # Deep settings
          - deep1  # Deep value 1
          - deep2  # Deep value 2
          # End of deep values
# End of items
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Nested sequences with comments should parse successfully"

    # Verify structure
    assert len(result.data['items']) == 3
    assert result.data['items'][0]['name'] == 'item1'
    assert result.data['items'][0]['config']['enabled'] is True
    assert result.data['items'][0]['config']['timeout'] == 30
    assert result.data['items'][0]['nested'] == ['subitem1', 'subitem2']

    assert result.data['items'][1]['name'] == 'item2'
    assert result.data['items'][1]['config']['enabled'] is False

    assert result.data['items'][2]['name'] == 'item3'
    assert result.data['items'][2]['config']['advanced']['deep'] == ['deep1', 'deep2']

    # Verify comments filtered
    data_str = str(result.data).lower()
    assert 'first item group' not in data_str
    assert 'second item group' not in data_str
    assert 'end of items' not in data_str


def test_nested_mappings_with_anchors_and_comments():
    """Test complex nested mappings with anchors and comments throughout."""
    parser = YAMLCoreParser()
    yaml_content = """# Base configuration template
base_config: &config_template
  # Basic settings
  basic:
    timeout: 30  # Default timeout
    retries: 3  # Retry count
  # Advanced settings
  advanced:
    debug: false  # Debug disabled
    verbose: false  # Verbose disabled

# First service configuration
service1:
  # Use base template
  <<: *config_template
  name: service1  # Service name
  # Override basic settings
  basic:
    timeout: 60  # Override timeout
    retries: 5  # Override retries
  # Additional nested config
  custom:
    # Custom nested level 1
    level1:
      # Custom nested level 2
      level2:
        value: custom1  # Custom value

# Second service configuration
service2:
  # Use base template
  <<: *config_template
  name: service2  # Service name
  # Override advanced settings
  advanced:
    debug: true  # Enable debug
    verbose: true  # Enable verbose
  # Additional nested config
  custom:
    # Custom nested level 1
    level1:
      # Custom nested level 2
      level2:
        value: custom2  # Custom value
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Nested mappings with anchors should parse successfully"

    # Verify service1 inherited and overrode correctly
    assert result.data['service1']['name'] == 'service1'
    assert result.data['service1']['basic']['timeout'] == 60  # Overridden
    assert result.data['service1']['basic']['retries'] == 5  # Overridden
    assert result.data['service1']['advanced']['debug'] is False  # Inherited
    assert result.data['service1']['custom']['level1']['level2']['value'] == 'custom1'

    # Verify service2 inherited and overrode correctly
    assert result.data['service2']['name'] == 'service2'
    assert result.data['service2']['basic']['timeout'] == 30  # Inherited
    assert result.data['service2']['basic']['retries'] == 3  # Inherited
    assert result.data['service2']['advanced']['debug'] is True  # Overridden
    assert result.data['service2']['advanced']['verbose'] is True  # Overridden
    assert result.data['service2']['custom']['level1']['level2']['value'] == 'custom2'

    # Verify comments filtered
    data_str = str(result.data).lower()
    assert 'base configuration template' not in data_str
    assert 'basic settings' not in data_str
    assert 'advanced settings' not in data_str


# ============================================================================
# Document Boundary Comments Tests
# ============================================================================

def test_document_header_comments():
    """Test comments at document header (before any content)."""
    parser = YAMLCoreParser()
    yaml_content = """# ============================================================
# Configuration File for MyApp v1.0
# ============================================================
# Author: Development Team
# Created: 2026-07-13
# Last Modified: 2026-07-13
# Description: This file contains application configuration
# ============================================================

# Database configuration
database:
  host: localhost
  port: 5432
  name: mydb

# Server configuration
server:
  port: 8080
  host: 0.0.0.0
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Document with header comments should parse successfully"

    # Verify data is correct
    assert result.data['database']['host'] == 'localhost'
    assert result.data['database']['port'] == 5432
    assert result.data['server']['port'] == 8080

    # Verify header comments filtered
    data_str = str(result.data).lower()
    assert 'configuration file for myapp' not in data_str
    assert 'author: development team' not in data_str
    assert 'created: 2026-07-13' not in data_str
    assert 'description: this file contains' not in data_str


def test_document_footer_comments():
    """Test comments at document footer (after all content)."""
    parser = YAMLCoreParser()
    yaml_content = """# Main configuration
database:
  host: localhost
  port: 5432

server:
  port: 8080

# ============================================================
# Configuration Notes
# ============================================================
# - Do not modify the database port
# - Server port can be changed if needed
# - All passwords should be stored in secrets manager
# - Contact admin@company.com for support
# ============================================================
# Version: 1.0.0
# Environment: production
# ============================================================
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Document with footer comments should parse successfully"

    # Verify data is correct
    assert result.data['database']['host'] == 'localhost'
    assert result.data['database']['port'] == 5432
    assert result.data['server']['port'] == 8080

    # Verify footer comments filtered
    data_str = str(result.data).lower()
    assert 'configuration notes' not in data_str
    assert 'do not modify' not in data_str
    assert 'all passwords should' not in data_str
    assert 'contact admin' not in data_str
    assert 'version: 1.0.0' not in data_str


def test_document_both_boundaries_with_content():
    """Test comments at both document boundaries with content in middle."""
    parser = YAMLCoreParser()
    yaml_content = """# ============================================================
# START OF CONFIGURATION FILE
# Application: MyApp
# Version: 2.0.0
# ============================================================

# Section 1: Database
database:
  host: db.example.com
  port: 5432
  name: production

# Section 2: Cache
cache:
  host: cache.example.com
  port: 6379

# Section 3: Server
server:
  port: 8080
  host: 0.0.0.0

# ============================================================
# END OF CONFIGURATION FILE
# ============================================================
# Configuration loaded successfully
# Ready for deployment
# ============================================================
"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Document with both boundary comments should parse successfully"

    # Verify all sections parsed correctly
    assert result.data['database']['host'] == 'db.example.com'
    assert result.data['cache']['port'] == 6379
    assert result.data['server']['host'] == '0.0.0.0'

    # Verify all boundary comments filtered
    data_str = str(result.data).lower()
    assert 'start of configuration' not in data_str
    assert 'application: myapp' not in data_str
    assert 'end of configuration' not in data_str
    assert 'configuration loaded' not in data_str
    assert 'ready for deployment' not in data_str


def test_document_with_empty_sections_and_boundaries():
    """Test document with empty sections between content and boundary comments."""
    parser = YAMLCoreParser()
    yaml_content = """# Header comment 1
# Header comment 2

# Another header comment

database:
  host: localhost
  port: 5432


# Separator comment


server:
  port: 8080


# Footer comment 1
# Footer comment 2

"""
    result = parser.safe_load(yaml_content)
    assert result.is_success(), "Document with empty sections should parse successfully"

    # Verify data is correct
    assert result.data['database']['host'] == 'localhost'
    assert result.data['server']['port'] == 8080

    # Verify boundary comments filtered
    data_str = str(result.data).lower()
    assert 'header comment' not in data_str
    assert 'another header comment' not in data_str
    assert 'separator comment' not in data_str
    assert 'footer comment' not in data_str


# ============================================================================
# Test Runner
# ============================================================================

def main():
    """Run all complete mixed YAML document tests."""
    print("Running Complete Mixed YAML Document Tests")
    print("=" * 70)

    tests = [
        # Complete document filtering tests
        ("Complete document with all comment types", test_complete_document_with_all_comment_types),
        ("Complete document with anchors and comments", test_complete_document_with_anchors_and_comments),
        ("Complete document with multiline values and comments", test_complete_document_with_multiline_values_and_comments),

        # Nested structure comments tests
        ("Deeply nested structure with comments", test_deeply_nested_structure_with_comments),
        ("Nested sequences with comments", test_nested_sequences_with_comments),
        ("Nested mappings with anchors and comments", test_nested_mappings_with_anchors_and_comments),

        # Document boundary comments tests
        ("Document header comments", test_document_header_comments),
        ("Document footer comments", test_document_footer_comments),
        ("Document both boundaries with content", test_document_both_boundaries_with_content),
        ("Document with empty sections and boundaries", test_document_with_empty_sections_and_boundaries),
    ]

    passed = 0
    failed = 0

    for test_name, test_func in tests:
        if run_test(test_name, test_func):
            passed += 1
        else:
            failed += 1

    print("=" * 70)
    print(f"Results: {passed} passed, {failed} failed")

    if failed > 0:
        print("\n❌ Some tests failed")
        sys.exit(1)

    print("\n✅ All complete mixed YAML document tests passed!")
    print("\nAcceptance criteria verified:")
    print("  ✓ Test function for complete document filtering")
    print("  ✓ Test function for nested structure comments")
    print("  ✓ Test function for document boundary comments")
    print("  ✓ All tests pass")
    sys.exit(0)


if __name__ == '__main__':
    main()
