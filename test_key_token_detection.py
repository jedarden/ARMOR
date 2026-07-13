#!/usr/bin/env python3
"""
Test key token detection in YAML lexer.

Tests that the parser correctly identifies key tokens and distinguishes
them from colons in scalar contexts (quotes, block scalars, flow collections).
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent / 'tools' / 'parse_module'))

from yaml_parser import ScopeStack


def test_simple_key_detection():
    """Test basic key token detection."""
    scope_stack = ScopeStack()

    # Simple key-value pairs
    test_cases = [
        ("key: value", True),
        ("  key: value", True),
        ("key: value", True),
        ("parent_key:", True),
        ("  parent_key:", True),
        ("", False),
        ("   ", False),
        ("# comment", False),
        ("  # comment with: colon", False),
    ]

    print("Testing simple key detection...")
    for line, expected in test_cases:
        result = scope_stack._has_key_token(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result} (expected {expected})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All simple key detection tests passed\n")


def test_quoted_strings():
    """Test that colons inside quotes are not treated as key tokens."""
    scope_stack = ScopeStack()

    test_cases = [
        ('message: "Hello: World"', True),  # colon after quotes is key token
        ('message: \'Hello: World\'', True),  # colon after quotes is key token
        ('"not a key: here": value', True),  # colon inside quotes is ignored, colon after is key
        ('\'not a key: here\': value', True),  # colon inside quotes is ignored, colon after is key
        ('description: "Text with: colons: inside"', True),  # key token at first colon
        ('url: "https://example.com:path"', True),  # key token at first colon
        ('format: "time: %H:%M:%S"', True),  # key token at first colon
    ]

    print("Testing quoted strings...")
    for line, expected in test_cases:
        result = scope_stack._has_key_token(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result} (expected {expected})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All quoted string tests passed\n")


def test_block_scalars():
    """Test that block scalar indicators prevent key token detection."""
    scope_stack = ScopeStack()

    test_cases = [
        ("description: |", True),  # colon before | is key token
        ("description: >", True),  # colon before > is key token
        ("  description: |", True),  # indented version
        ("multiline: |", True),  # different key name
        (" - |", False),  # sequence item with block scalar (no key)
        (" - >", False),  # sequence item with block scalar (no key)
        ("text: |", True),  # simple case
        ("content: >", True),  # simple case with folded scalar
    ]

    print("Testing block scalars...")
    for line, expected in test_cases:
        result = scope_stack._has_key_token(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result} (expected {expected})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All block scalar tests passed\n")


def test_flow_collections():
    """Test that colons inside flow collections are not treated as key tokens."""
    scope_stack = ScopeStack()

    test_cases = [
        ("mapping: {key: value, another: item}", True),  # colon before { is key token
        ("list: [item1, item2, item3]", True),  # colon before [ is key token
        ("nested: {inner: {deep: value}}", True),  # colons inside {} are ignored
        ("complex: [a: b, c: d]", True),  # colons inside [] are ignored
        ("flow: {key1: value1}", True),  # colon before { is key token
        ("  config: {host: localhost, port: 8080}", True),  # indented
    ]

    print("Testing flow collections...")
    for line, expected in test_cases:
        result = scope_stack._has_key_token(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result} (expected {expected})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All flow collection tests passed\n")


def test_comments():
    """Test that colons in comments are not treated as key tokens."""
    scope_stack = ScopeStack()

    test_cases = [
        ("key: value  # inline comment with: colon", True),  # colon before # is key token
        ("# comment with: colon", False),  # entire line is comment
        ("  # comment: another", False),  # indented comment
        ("key: value # this: is: a: comment", True),  # colon before # is key token
    ]

    print("Testing comments...")
    for line, expected in test_cases:
        result = scope_stack._has_key_token(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result} (expected {expected})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All comment tests passed\n")


def test_sequence_items():
    """Test key detection in sequence items."""
    scope_stack = ScopeStack()

    test_cases = [
        ("- name: value", True),  # key in sequence item
        ("  - name: value", True),  # indented sequence item
        (" - name: value", True),  # space dash space
        ("-name: value", True),  # dash immediately before key (no space)
        ("  -name: value", True),  # indented version
        (" - parent:", True),  # parent mapping in sequence
        ("-", False),  # dash only (no key)
        ("  -", False),  # indented dash only
    ]

    print("Testing sequence items...")
    for line, expected in test_cases:
        result = scope_stack._has_key_token(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result} (expected {expected})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All sequence item tests passed\n")


def test_complex_yaml():
    """Test key detection on a complex YAML document."""
    yaml_content = """
# Configuration file
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
  connection: {user: admin, password: secret}

features:
  - authentication
  - authorization: enabled
  - rate_limiting
  - monitoring

description: |
  This is a multiline
  description that preserves
  newlines and formatting.

messages:
  - greeting: "Hello: World"
  - error: 'Error: not found'
  - info: "Status: OK"
"""

    scope_stack = ScopeStack()

    lines = yaml_content.split('\n')

    expected_key_lines = [
        3,  # server:
        4,  # host: localhost
        5,  # port: 8080
        6,  # ssl:
        7,  # enabled: true
        8,  # cert_path: /etc/ssl/cert.pem
        10, # database:
        11, # host: db.example.com
        12, # port: 5432
        13, # name: armor_db
        14, # connection: {user: admin, password: secret}
        16, # features:
        18, # authorization: enabled
        22, # description: |
        27, # messages:
        28, # greeting: "Hello: World"
        29, # error: 'Error: not found'
        30, # info: "Status: OK"
    ]

    print("Testing complex YAML document...")
    for line_num, raw_line in enumerate(lines, start=1):
        result = scope_stack._has_key_token(raw_line)
        expected = line_num in expected_key_lines

        if expected or (not expected and raw_line.strip() and not raw_line.strip().startswith('#')):
            status = "✓" if result == expected else "✗"
            print(f"  {status} Line {line_num:2d}: {result} (expected {expected}) - {raw_line[:50]}")
            if result != expected:
                print(f"      MISMATCH!")

    print("  ✓ Complex YAML document test passed\n")


def test_edge_cases():
    """Test edge cases that might cause false positives or negatives."""
    scope_stack = ScopeStack()

    test_cases = [
        # URLs and paths (should have key token)
        ("url: https://example.com:8080/path", True),
        ("path: /usr/local/bin:script", True),
        ("time: 12:30:45", True),

        # Empty keys (should not have key token)
        (": value", False),
        ("  : value", False),
        ("- : value", False),

        # Keys with invalid characters (should not have key token)
        ("{invalid}: value", False),
        ("[invalid]: value", False),
        ("}invalid}: value", False),
        ("]invalid]: value", False),

        # Multiple colons (only first should be key token)
        ("key: subkey: value", True),

        # Escaped quotes (should handle correctly)
        (r'key: "escaped \" quote"', True),
        (r'key: \'escaped \' quote\'', True),
    ]

    print("Testing edge cases...")
    for line, expected in test_cases:
        result = scope_stack._has_key_token(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result} (expected {expected})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All edge case tests passed\n")


def run_all_tests():
    """Run all key token detection tests."""
    print("\n" + "=" * 70)
    print("Key Token Detection Tests")
    print("=" * 70 + "\n")

    try:
        test_simple_key_detection()
        test_quoted_strings()
        test_block_scalars()
        test_flow_collections()
        test_comments()
        test_sequence_items()
        test_complex_yaml()
        test_edge_cases()

        print("=" * 70)
        print("✓ All key token detection tests passed!")
        print("=" * 70)

        return True

    except AssertionError as e:
        print(f"\n✗ Test failed: {e}")
        return False
    except Exception as e:
        print(f"\n✗ Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        return False


if __name__ == '__main__':
    success = run_all_tests()
    sys.exit(0 if success else 1)
