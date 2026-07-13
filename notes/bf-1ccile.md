# Sequence Scope Verification (Bead: bf-1ccile)

## Summary

Created comprehensive tests for sequence scope handling in YAML parsing. The tests successfully verify that the low-level scope tracking mechanism works correctly.

## Test Coverage

Created `tests/sequence_scope_verification_test.rs` with 32 test cases covering:

### 1. Sequence Scope Entry Tracking
- Sequence scope entry sets `in_sequence_context` flag correctly
- Sequence scope entry clears previous keys for new items
- Sequence scope entry increments item IDs
- Sequence scope entry creates isolated scopes

### 2. Sequence Scope Exit Behavior  
- Sequence scope exit updates scope state correctly
- Sequence scope exit clears sequence keys
- Sequence scope exit to different indents

### 3. Sequences Nested in Mappings
- Sequence nested in mapping scope isolation
- Sequence nested in mapping parent context preservation
- Multiple sequences in same mapping
- Deeply nested sequence in mapping

### 4. Mappings Nested in Sequences
- Mapping nested in sequence scope
- Mapping nested in sequence isolation between items
- Multiple mappings in same sequence item
- Deeply nested mapping in sequence

### 5. Complex Combinations
- Sequence → mapping → sequence → pattern
- Alternating sequence and mapping
- Sibling sequences with nested mappings
- Complex real-world structure simulation

### 6. Integration with Parser
- Parser sequence nested in mapping
- Parser mapping nested in sequence
- Parser complex nested structure
- Parser detects duplicate within sequence item
- Parser sequence at root level
- Parser mixed root sequences and mappings

### 7. Edge Cases
- Sequence with empty items
- Sequence with single key
- Sequence entry after deep nesting
- Sequence exit from deeply nested
- Multiple consecutive sequence entries at same indent
- Sequence entry preserves parent scopes

## Test Results

**31 out of 32 tests passing**

The scope tracking mechanism works correctly when tested in isolation (direct ScopeStack API usage). However, one parser integration test fails, revealing a bug in the parser implementation.

## Bug Discovered

The parser has an issue with sequence item key extraction. When parsing sequence items like:

```yaml
items:
  - id: 1
  - id: 2
```

The `extract_key_context()` function extracts the key as "- id" (including the dash marker), not just "id". This causes all sequence items to have the same key ("- id"), which is then detected as a duplicate even though they're in different sequence scopes.

### Root Cause

In `src/parsers/yaml/scope.rs`, the `extract_key_context()` function (lines 1003-1033) extracts the entire text before the colon as the key, including the sequence dash marker "- ".

For input "- id: 1":
- `colon_pos` = 4 (position of ":")
- `key_part` = "- id" (includes the dash)
- `key` = "- id" (trimmed but still includes dash)

### Expected Behavior

Sequence items should have their keys extracted WITHOUT the dash marker, so:
- "- id: 1" should extract key as "id" not "- id"
- "- name: First" should extract key as "name" not "- name"

### Files Created

- `tests/sequence_scope_verification_test.rs` - Comprehensive test suite (651 lines)

### Test Execution

```bash
cargo test --test sequence_scope_verification_test
```

## Recommendations

1. **Fix the key extraction**: Modify `extract_key_context()` to strip the sequence marker "- " from the key before returning it.

2. **Alternative approach**: Create a separate `extract_sequence_item_key()` function specifically for sequence items that handles the dash marker correctly.

3. **Parser integration**: Ensure the parser calls the correct key extraction function based on whether the line is a sequence item or not.

## Verification

The low-level scope tracking tests all pass, demonstrating that:
- Sequence scope entry is tracked correctly ✓
- Sequence scope exit maintains proper scope state ✓  
- Nested sequence/mapping combinations work ✓
- Scope isolation between sequence items works ✓

The bug is isolated to the parser integration layer, not the scope tracking core.
