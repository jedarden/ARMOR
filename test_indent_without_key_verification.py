#!/usr/bin/env python3
"""Comprehensive verification that indent changes are detected without key tokens."""

import sys
sys.path.insert(0, "tools/parse_module")
from yaml_parser import YAMLParser, ScopeStack, LineClassification

print("=" * 80)
print("COMPREHENSIVE VERIFICATION: Indent Detection Without Key Tokens")
print("=" * 80)
print()

# Test 1: Indent changes are detected regardless of key presence
print("TEST 1: Indent changes are detected regardless of key presence")
print("-" * 80)

yaml1 = """
root:
  key1: value1
    indent_only_continuation
    another_continuation
  key2: value2
"""

parser1 = YAMLParser()
result1 = parser1.parse_with_scope_tracking(yaml1)
transitions1 = parser1.get_scope_stack().get_indent_transitions()

print(f"YAML with indent-only continuation lines:")
for trans in transitions1:
    has_key_str = "WITH KEY" if trans.has_key else "WITHOUT KEY"
    print(f"  Line {trans.line_number}: {trans.from_indent} -> {trans.to_indent} ({has_key_str})")

indent_only_transitions = [t for t in transitions1 if not t.has_key]
if len(indent_only_transitions) > 0:
    print(f"✓ PASS: {len(indent_only_transitions)} indent-only transitions detected")
else:
    print("✗ FAIL: No indent-only transitions detected")
print()

# Test 2: Parser can distinguish key-bearing lines from indent-only lines
print("TEST 2: Parser can distinguish key-bearing lines from indent-only lines")
print("-" * 80)

yaml2 = """
parent:
  child_key: value
    indent_only_line
  another_key: value
"""

parser2 = YAMLParser()
result2 = parser2.parse_with_scope_tracking(yaml2)
transitions2 = parser2.get_scope_stack().get_indent_transitions()

key_bearing = [t for t in transitions2 if t.line_classification == LineClassification.KEY_BEARING]
indent_only = [t for t in transitions2 if t.line_classification == LineClassification.INDENT_ONLY]
empty = [t for t in transitions2 if t.line_classification == LineClassification.EMPTY]

print(f"Line classification results:")
print(f"  Key-bearing lines: {len(key_bearing)}")
print(f"  Indent-only lines: {len(indent_only)}")
print(f"  Empty lines: {len(empty)}")

if len(key_bearing) > 0 and len(indent_only) > 0:
    print("✓ PASS: Both key-bearing and indent-only lines classified")
else:
    print("✗ FAIL: Line classification not working correctly")
print()

# Test 3: Detection logic doesn't interfere with existing key parsing
print("TEST 3: Detection logic doesn't interfere with existing key parsing")
print("-" * 80)

yaml3 = """
service:
  name: my-service
  config:
    enabled: true
    timeout: 30
  description: |
    This is a multiline
    description block
  ports:
    - 8080
    - 9090
"""

parser3 = YAMLParser()
result3 = parser3.parse_with_scope_tracking(yaml3)

if result3.is_success():
    print("✓ PASS: YAML parsing succeeded (indent detection didn't break key parsing)")
else:
    print(f"✗ FAIL: YAML parsing failed: {result3.error}")

transitions3 = parser3.get_scope_stack().get_indent_transitions()
key_bearing_trans = [t for t in transitions3 if t.line_classification == LineClassification.KEY_BEARING]
print(f"  Key-bearing transitions detected: {len(key_bearing_trans)}")

# Verify the actual data structure is correct
data = result3.data
if data and "service" in data:
    print("✓ PASS: Data structure parsed correctly")
    print(f"  Service name: {data['service'].get('name', 'N/A')}")
else:
    print("✗ FAIL: Data structure not parsed correctly")
print()

# Test 4: Complex scenario with mixed indent types
print("TEST 4: Complex scenario with mixed indent types")
print("-" * 80)

yaml4 = """
level1:
  level2_key: value
    continuation1
    continuation2
  level2_sibling: another
  level2_with_multiline: |
    line 1
    line 2
      nested line
"""

parser4 = YAMLParser()
result4 = parser4.parse_with_scope_tracking(yaml4)
transitions4 = parser4.get_scope_stack().get_indent_transitions()

print(f"Complex YAML transitions:")
for trans in transitions4:
    has_key_str = "KEY" if trans.has_key else "NO KEY"
    print(f"  Line {trans.line_number}: {trans.from_indent}->{trans.to_indent} ({has_key_str}) - {trans.line_classification.value}")

# Count by type
with_key = [t for t in transitions4 if t.has_key]
without_key = [t for t in transitions4 if not t.has_key]
enters = [t for t in transitions4 if t.is_enter_scope()]
exits = [t for t in transitions4 if t.is_exit_scope()]

print()
print(f"Summary:")
print(f"  Transitions with keys: {len(with_key)}")
print(f"  Transitions without keys: {len(without_key)}")
print(f"  Enter-scope transitions: {len(enters)}")
print(f"  Exit-scope transitions: {len(exits)}")

if len(with_key) > 0 and len(without_key) > 0:
    print("✓ PASS: Mixed indent scenario handled correctly")
else:
    print("✗ FAIL: Mixed indent scenario not handled correctly")
print()

print("=" * 80)
print("FINAL VERDICT")
print("=" * 80)

all_pass = (
    len(indent_only_transitions) > 0 and
    len(key_bearing) > 0 and len(indent_only) > 0 and
    result3.is_success() and
    len(with_key) > 0 and len(without_key) > 0
)

if all_pass:
    print("✓ ALL ACCEPTANCE CRITERIA MET")
    print()
    print("The parser already implements:")
    print("  1. Indent change detection on whitespace changes (regardless of keys)")
    print("  2. Line classification (key-bearing vs indent-only)")
    print("  3. Indent level transition tracking in parser state")
    print("  4. Non-interference with existing key parsing")
else:
    print("✗ SOME ACCEPTANCE CRITERIA NOT MET")
