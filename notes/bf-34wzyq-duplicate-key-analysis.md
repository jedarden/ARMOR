# Duplicate Key Detection Implementation Analysis

## Task: Analyze current duplicate key detection implementation

Date: 2026-07-13

## Overview

This document analyzes the current duplicate key detection logic in `syntax_detector.rs` to understand how keys are tracked and why nested mappings are not properly scoped.

## Current Implementation

### Data Structures

The duplicate key detection uses a stack-based scope tracking approach defined in `StructureState`:

```rust
struct StructureState {
    /// Stack of nested structures (mapping, sequence, etc.)
    context_stack: Vec<StructureContext>,
    
    /// Stack of key scopes, one per mapping scope
    /// Each scope contains: (keys_set, indent_level)
    /// The top of the stack is the current scope
    key_scope_stack: Vec<(HashSet<String>, usize)>,
    
    /// Previous line's indentation level
    prev_indent: usize,
    
    /// Previous line's parent key status
    prev_was_parent_key: bool,
}
```

### Scope Management Logic

The `detect_duplicate_key_errors` function (lines 653-758) manages scope transitions:

1. **Sequence items (`- `)** (lines 665-688):
   - Pop all scopes at current indent level or deeper
   - Create a new scope for this sequence item's children
   - Handles sibling sequence items correctly

2. **Parent keys** (keys with no value on same line) (lines 715-731):
   - Pop all scopes at current indent level or deeper
   - Create a new scope for this parent key's children
   - This ensures each nested mapping gets its own scope

3. **Key-value pairs** (lines 732-753):
   - Add key to the current scope (top of stack)
   - Check for duplicates within that scope only
   - Root scope is created if it doesn't exist

### How Scope Transitions Work

The scope popping logic (lines 667-681 and 717-729):

```rust
while !self.structure_state.key_scope_stack.is_empty() {
    if let Some((_, scope_indent)) = self.structure_state.key_scope_stack.last() {
        if *scope_indent >= indent {
            // Pop scopes at same or deeper level (siblings or nested)
            self.structure_state.key_scope_stack.pop();
        } else {
            // Keep parent scopes
            break;
        }
    } else {
        break;
    }
}
```

This logic:
- **Pops scopes** when indent decreases (moving out of a nested block)
- **Keeps parent scopes** when at the same or deeper indent
- Uses `>=` comparison to handle both siblings and depth changes

## What "Scope" Means in This Context

In YAML duplicate key detection, **scope** refers to the mapping context where keys must be unique:

### Same Scope (should detect duplicates):
```yaml
services:
  web:
    host: localhost    # 'web' scope
    host: 127.0.0.1   # DUPLICATE in 'web' scope
```

### Different Scopes (should NOT detect duplicates):
```yaml
services:
  web:
    host: localhost    # 'web' scope
  database:
    host: remotehost  # 'database' scope (different!)
```

## Test Analysis

### Test Case 1: Same key in different scopes (test_same_key_in_different_scopes)

```yaml
services:
  web:
    host: localhost
  database:
    host: remotehost
```

**Expected**: No duplicate key errors (different scopes)
**Actual**: ✓ PASS (correctly identified as different scopes)

**Trace**:
1. `services:` (indent 0) → Push scope `(HashSet, 0)`
2. `  web:` (indent 2) → Push scope `(HashSet, 2)`
3. `    host: localhost` (indent 4) → Add "host" to scope at indent 2
4. `  database:` (indent 2) → Pop scope at indent 2, push new scope `(HashSet, 2)`
5. `    host: remotehost` (indent 4) → Add "host" to NEW scope at indent 2

Result: Each "host" is in a different scope (different web vs database scopes)

### Test Case 2: Duplicate in nested mapping (test_duplicate_key_nested_mapping)

```yaml
config:
  web:
    port: 8080
    port: 8081
```

**Expected**: Duplicate key error for 'port'
**Actual**: ✓ PASS (correctly detected)

**Trace**:
1. `config:` (indent 0) → Push scope `(HashSet, 0)`
2. `  web:` (indent 2) → Push scope `(HashSet, 2)`
3. `    port: 8080` (indent 4) → Add "port" to scope at indent 2
4. `    port: 8081` (indent 4) → Check scope at indent 2 → DUPLICATE FOUND

Result: Correctly detected duplicate within same scope

### Test Case 3: Complex nested structure (test_complex_nested_structure)

```yaml
servers:
  production:
    host: prod.example.com
    port: 443
    ssl: true
  staging:
    host: staging.example.com
    port: 443
    ssl: true
```

**Expected**: No duplicate key errors (each server has its own scope)
**Actual**: ✓ PASS

## Root Cause Analysis

### CURRENT IMPLEMENTATION IS WORKING CORRECTLY

After thorough analysis and testing, **the current implementation correctly handles nested mappings**. The scope tracking logic properly:

1. ✅ Creates new scopes for each nested mapping (parent keys)
2. ✅ Maintains separate key sets for sibling mappings
3. ✅ Detects duplicates within the same scope
4. ✅ Does NOT report false positives for keys in different scopes
5. ✅ Handles sequence items with their own scopes

### What IS Working

- **Scope creation**: New scopes are created for each parent key and sequence item
- **Scope popping**: Scopes are correctly popped when moving to sibling items
- **Duplicate detection**: Keys are only checked against their current scope
- **Nesting**: Deep nesting is properly handled by the stack-based approach

## Potential Issues (Edge Cases)

### 1. Root Scope Assumption

Lines 735-737 create a root scope if none exists:
```rust
if self.structure_state.key_scope_stack.is_empty() {
    self.structure_state.key_scope_stack.push((HashSet::new(), 0));
}
```

This could be problematic if the first line is a key-value pair (not a parent key).

### 2. Flow Style Context

Line 657-659 skips duplicate detection inside flow contexts:
```rust
if self.delimiter_state.in_flow_context {
    return;
}
```

This is correct but means flow-style mappings like `{key: value, key: value2}` won't be checked.

### 3. Quoted Keys

Line 710 skips quoted keys:
```rust
if key.starts_with('\'') || key.starts_with('"') || key.contains('#') {
    self.structure_state.prev_indent = indent;
    return;
}
```

This means quoted keys with the same name won't be detected as duplicates.

## Conclusion

**The current implementation is fundamentally sound and correctly handles nested mapping scopes.** The scope tracking logic uses a proper stack-based approach that:

1. Maintains separate key sets for each mapping scope
2. Correctly transitions between scopes based on indentation
3. Detects duplicates only within the appropriate scope
4. Does not produce false positives for keys in different scopes

### What "Scope" Means

**Scope** in this context refers to a single mapping context where keys must be unique. In YAML:
- Each parent key creates a new scope for its children
- Each sequence item creates a new scope for its children
- Sibling mappings have separate scopes (e.g., `web:` vs `database:`)
- Duplicate keys are only errors within the same scope

### No Root Cause Found

The implementation correctly handles all tested cases of nested mappings. There is no fundamental issue with scope awareness that would cause incorrect behavior.

## Recommendations

While the current implementation is working, potential improvements:

1. **Add more edge case tests** for unusual YAML structures
2. **Consider handling flow-style duplicates** if needed
3. **Better handling of quoted keys** to detect duplicates even when quoted
4. **Documentation improvements** to explain the scope model more clearly

## Test Results Summary

All duplicate key detection tests pass:
- ✅ test_duplicate_key_in_same_scope
- ✅ test_same_key_in_different_scopes  
- ✅ test_duplicate_key_nested_mapping
- ✅ test_complex_nested_structure_no_duplicates
- ✅ test_mixed_valid_and_invalid_duplicates
- ✅ test_deeply_nested_scopes
- ✅ test_flow_style_not_flagged
