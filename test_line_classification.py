#!/usr/bin/env python3
"""
Test line classification in YAML lexer.

Tests that the parser correctly categorizes lines as:
- KEY_BEARING: lines that have a key token
- INDENT_ONLY: lines with only indentation (no key token)
- EMPTY: empty lines or whitespace-only lines
"""

import sys
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent / 'tools' / 'parse_module'))

from yaml_parser import ScopeStack, LineClassification


def test_empty_lines():
    """Test that empty lines are classified correctly."""
    scope_stack = ScopeStack()

    test_cases = [
        ("", LineClassification.EMPTY),
        ("   ", LineClassification.EMPTY),
        ("\t", LineClassification.EMPTY),
        ("    \t  ", LineClassification.EMPTY),
    ]

    print("Testing empty line classification...")
    for line, expected in test_cases:
        result = scope_stack._classify_line_type(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{repr(line)}': {result.value} (expected {expected.value})")
        assert result == expected, f"Failed for line: {repr(line)}"

    print("  ✓ All empty line tests passed\n")


def test_comment_lines():
    """Test that comment lines are classified as indent-only (no key token)."""
    scope_stack = ScopeStack()

    test_cases = [
        ("# comment", LineClassification.INDENT_ONLY),
        ("  # comment", LineClassification.INDENT_ONLY),
        ("# comment with: colon", LineClassification.INDENT_ONLY),
        ("    # multi-level indent comment", LineClassification.INDENT_ONLY),
    ]

    print("Testing comment line classification...")
    for line, expected in test_cases:
        result = scope_stack._classify_line_type(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result.value} (expected {expected.value})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All comment line tests passed\n")


def test_key_bearing_lines():
    """Test that lines with key tokens are classified correctly."""
    scope_stack = ScopeStack()

    test_cases = [
        # Simple key-value pairs
        ("key: value", LineClassification.KEY_BEARING),
        ("  key: value", LineClassification.KEY_BEARING),
        ("    key: value", LineClassification.KEY_BEARING),

        # Parent mappings (keys with nested content)
        ("parent:", LineClassification.KEY_BEARING),
        ("  parent:", LineClassification.KEY_BEARING),
        ("    parent:", LineClassification.KEY_BEARING),

        # Keys in sequences
        ("- name: value", LineClassification.KEY_BEARING),
        ("  - name: value", LineClassification.KEY_BEARING),
        ("- parent:", LineClassification.KEY_BEARING),

        # Keys with complex values
        ("key: {nested: value}", LineClassification.KEY_BEARING),
        ("list: [item1, item2]", LineClassification.KEY_BEARING),
        ("quoted: 'value'", LineClassification.KEY_BEARING),
        ('double: "value"', LineClassification.KEY_BEARING),

        # Keys with inline comments
        ("key: value  # comment", LineClassification.KEY_BEARING),

        # Keys with colons in quoted strings (first colon is key token)
        ('url: "https://example.com:8080"', LineClassification.KEY_BEARING),
        ('time: "12:30:45"', LineClassification.KEY_BEARING),

        # Block scalars (colon before | or > is key token)
        ("text: |", LineClassification.KEY_BEARING),
        ("folded: >", LineClassification.KEY_BEARING),
    ]

    print("Testing key-bearing line classification...")
    for line, expected in test_cases:
        result = scope_stack._classify_line_type(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result.value} (expected {expected.value})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All key-bearing line tests passed\n")


def test_indent_only_lines():
    """Test that lines without key tokens are classified as indent-only."""
    scope_stack = ScopeStack()

    test_cases = [
        # Sequence items without keys
        ("-", LineClassification.INDENT_ONLY),
        ("  -", LineClassification.INDENT_ONLY),
        ("    -", LineClassification.INDENT_ONLY),

        # Block scalar lines (continuation lines)
        ("  continuation text", LineClassification.INDENT_ONLY),
        ("    more continuation", LineClassification.INDENT_ONLY),

        # Lines that look like they have keys but don't
        (": value", LineClassification.INDENT_ONLY),  # empty key
        ("  : value", LineClassification.INDENT_ONLY),

        # Invalid key characters
        ("{invalid}: value", LineClassification.INDENT_ONLY),
        ("[invalid]: value", LineClassification.INDENT_ONLY),
    ]

    print("Testing indent-only line classification...")
    for line, expected in test_cases:
        result = scope_stack._classify_line_type(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result.value} (expected {expected.value})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All indent-only line tests passed\n")


def test_document_markers():
    """Test that YAML document markers are classified correctly."""
    scope_stack = ScopeStack()

    test_cases = [
        ("---", LineClassification.INDENT_ONLY),
        ("  ---", LineClassification.INDENT_ONLY),
        ("...", LineClassification.INDENT_ONLY),
        ("  ...", LineClassification.INDENT_ONLY),
    ]

    print("Testing document marker classification...")
    for line, expected in test_cases:
        result = scope_stack._classify_line_type(line)
        status = "✓" if result == expected else "✗"
        print(f"  {status} '{line}': {result.value} (expected {expected.value})")
        assert result == expected, f"Failed for line: {line}"

    print("  ✓ All document marker tests passed\n")


def test_complex_yaml_structure():
    """Test classification on a complex YAML document with nested keys."""
    yaml_content = """# Configuration file
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

features:
  - authentication
  - authorization: enabled
  - rate_limiting

description: |
  This is a multiline
  description block.

messages:
  - greeting: "Hello: World"
  - error: 'Error: not found'
"""

    scope_stack = ScopeStack()

    expected_classifications = {
        1: LineClassification.INDENT_ONLY,  # # comment
        2: LineClassification.KEY_BEARING,  # server:
        3: LineClassification.KEY_BEARING,  # host: localhost
        4: LineClassification.KEY_BEARING,  # port: 8080
        5: LineClassification.KEY_BEARING,  # ssl:
        6: LineClassification.KEY_BEARING,  # enabled: true
        7: LineClassification.KEY_BEARING,  # cert_path: ...
        8: LineClassification.EMPTY,        # blank line
        9: LineClassification.KEY_BEARING,  # database:
        10: LineClassification.KEY_BEARING, # host: ...
        11: LineClassification.KEY_BEARING, # port: 5432
        12: LineClassification.KEY_BEARING, # name: armor_db
        13: LineClassification.EMPTY,       # blank line
        14: LineClassification.KEY_BEARING, # features:
        15: LineClassification.INDENT_ONLY, # - authentication (no key)
        16: LineClassification.KEY_BEARING, # - authorization: enabled
        17: LineClassification.INDENT_ONLY, # - rate_limiting (no key)
        18: LineClassification.EMPTY,       # blank line
        19: LineClassification.KEY_BEARING, # description: |
        20: LineClassification.INDENT_ONLY, # continuation text
        21: LineClassification.INDENT_ONLY, # continuation text
        22: LineClassification.EMPTY,       # blank line
        23: LineClassification.KEY_BEARING, # messages:
        24: LineClassification.KEY_BEARING, # - greeting: "Hello: World"
        25: LineClassification.KEY_BEARING, # - error: 'Error: not found'
    }

    print("Testing complex YAML structure classification...")
    lines = yaml_content.split('\n')

    all_correct = True
    for line_num, raw_line in enumerate(lines, start=1):
        result = scope_stack._classify_line_type(raw_line)
        expected = expected_classifications.get(line_num)

        if expected is not None:
            status = "✓" if result == expected else "✗"
            line_preview = raw_line[:40].ljust(40)
            print(f"  {status} Line {line_num:2d}: {result.value:15s} {line_preview}")

            if result != expected:
                print(f"      ERROR: Expected {expected.value}, got {result.value}")
                all_correct = False

    assert all_correct, "Some lines were classified incorrectly"
    print("  ✓ Complex YAML structure classification test passed\n")


def test_indent_transition_classification():
    """Test that indent transitions record line classifications correctly."""
    yaml_content = """parent:
  child: value
"""

    scope_stack = ScopeStack()

    print("Testing indent transition with line classification...")

    # Parse the content to generate transitions
    lines = yaml_content.split('\n')
    for line_num, raw_line in enumerate(lines, start=1):
        trimmed = raw_line.strip()
        if not trimmed:
            continue

        indent = len(raw_line) - len(raw_line.lstrip())
        has_key = scope_stack._has_key_token(raw_line)
        scope_stack.record_indent_transition(line_num, indent, has_key, raw_line)

    # Get transitions and verify classifications
    transitions = scope_stack.get_indent_transitions()

    # Line 1: parent: (KEY_BEARING, indent 0 -> 0, SAME_LEVEL - but not recorded since no change)
    # Line 2:   child: value (KEY_BEARING, indent 0 -> 2, ENTER_SCOPE)

    print(f"  Found {len(transitions)} transitions:")
    for trans in transitions:
        print(f"    Line {trans.line_number}: {trans.line_classification.value} "
              f"(indent {trans.from_indent} -> {trans.to_indent}, "
              f"transition: {trans.transition_type.value})")

    # Verify we have the transition to child scope
    assert len(transitions) == 1, f"Expected 1 transition, got {len(transitions)}"
    assert transitions[0].line_classification == LineClassification.KEY_BEARING
    assert transitions[0].transition_type.name == "ENTER_SCOPE"
    assert transitions[0].from_indent == 0
    assert transitions[0].to_indent == 2

    print("  ✓ Indent transition classification test passed\n")


def run_all_tests():
    """Run all line classification tests."""
    print("\n" + "=" * 70)
    print("Line Classification Tests for YAML Lexer")
    print("=" * 70 + "\n")

    try:
        test_empty_lines()
        test_comment_lines()
        test_key_bearing_lines()
        test_indent_only_lines()
        test_document_markers()
        test_complex_yaml_structure()
        test_indent_transition_classification()

        print("=" * 70)
        print("✓ All line classification tests passed!")
        print("=" * 70)
        print("\nAcceptance Criteria Verified:")
        print("  ✓ Parser correctly categorizes key-bearing lines")
        print("  ✓ Parser correctly categorizes indent-only lines (no key token)")
        print("  ✓ Classification works for complex YAML structures with nested keys")
        print("  ✓ Edge cases handled (comments, empty lines, document markers)")

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
