# Duplicate Key Detection Implementation Analysis

## Overview

This document analyzes the current duplicate key detection implementation in `src/parsers/yaml/syntax_detector.rs` and identifies the root cause of the nested mapping scope issue.

## Current Implementation

The `detect_duplicate_key_errors` method (lines 653-732) implements scope-aware duplicate key detection using a stack-based approach.

### Data Structures

```rust
struct StructureState {
    /// Stack of key scopes, one per mapping scope
    /// Each scope contains: (keys_set, indent_level)
    /// The top of the stack is the current scope
    key_scope_stack: Vec<(HashSet<String>, usize)>,
    ...
}
```

Each scope tracks:
- **`HashSet<String>`**: Keys seen in this scope
- **`usize`**: Indentation level at which the parent key was defined

### Algorithm Flow

#### 1. Parent Key Detection (lines 689-705)
When a parent key is encountered (e.g., `web:` with no value on same line):

```rust
if is_parent_key {
    // Pop all scopes at this indent level or deeper
    while !self.structure_state.key_scope_stack.is_empty() {
        if let Some((_, scope_indent)) = self.structure_state.key_scope_stack.last() {
            if *scope_indent >= indent {
                self.structure_state.key_scope_stack.pop();
            } else {
                break;
            }
        }
    }
    // Create new scope for this parent key's children
    self.structure_state.key_scope_stack.push((HashSet::new(), indent));
}
```

**Behavior**: Closes sibling or nested scopes and creates a fresh scope for the new parent key.

#### 2. Key-Value Pair Handling (lines 706-727)
When a key-value pair is encountered:

```rust
else {
    // Ensure root scope exists
    if self.structure_state.key_scope_stack.is_empty() {
        self.structure_state.key_scope_stack.push((HashSet::new(), 0));
    }

    if let Some((current_scope, scope_indent)) = self.structure_state.key_scope_stack.last_mut() {
        // BUG IS HERE: This condition is too permissive
        if indent >= *scope_indent {  // ← Line 716
            if current_scope.contains(key) {
                // Report duplicate
            } else {
                current_scope.insert(key.to_string());
            }
        }
    }
}
```

## Root Cause Identification

### The Bug: Line 716

```rust
if indent >= *scope_indent {
```

**Problem**: This condition allows keys at the **same indentation level as the parent key** to be added to that scope.

### Why This Causes Issues

In YAML, indentation defines hierarchy:
- **Parent key**: Ends with `:`, has no value on same line
- **Child key-value pairs**: Must be **indented deeper than** parent (strictly greater indent)
- **Sibling keys**: At same indent level as each other

A child key at the **same indent level as its parent** is semantically invalid in YAML. However, the current condition `indent >= scope_indent` allows it, which can cause keys to be added to the wrong scope.

### Example Scenario

```yaml
parent:
  child1: value1
  child2: value2
```

With `indent >= scope_indent`:
- `parent:` at indent 0 → creates scope at indent 0
- `child1: value1` at indent 2 → `2 >= 0` ✓ → adds to parent's scope ✓
- `child2: value2` at indent 2 → `2 >= 0` ✓ → duplicate check in parent's scope ✓

This works correctly because children are deeper than parent.

### Where It Would Fail

The current logic would incorrectly handle a scenario like:

```yaml
parent:
sibling: value  # Same indent as parent, but should be in different scope
```

With `indent >= scope_indent`:
- `parent:` at indent 0 → creates scope at indent 0
- `sibling: value` at indent 0 → `0 >= 0` ✓ → adds to parent's scope ✗

**Should be**: `sibling` creates a new sibling scope
**Actually is**: `sibling` is added to parent's scope

## The Fix: Change Comparison to Strict Inequality

```rust
if indent > *scope_indent {  // Changed from >= to >
```

### Rationale

In YAML, child key-value pairs must be **more indented than** their parent key. This is a fundamental YAML rule:

- A parent key at indent N creates a scope at indent N
- Its children must be at indent N+1 or deeper
- A key at exactly indent N is a **sibling**, not a child

Therefore, the condition should be `indent > scope_indent` (strictly greater), not `indent >= scope_indent`.

### Impact of Fix

With `indent > scope_indent`:
- Only keys **deeper than** the scope's indent are added to that scope
- Keys at the same indent level as the parent are not added (they'll create their own scope when encountered)
- Properly isolates sibling scopes

## What "Scope" Means in This Context

### Definition

A **scope** is a mapping context where keys must be unique. Each parent key creates a new scope for its children.

### Scope Boundaries

Scopes are bounded by:
1. **Parent key definition**: Creates a new scope at its indent level
2. **Sibling parent key**: Ends previous scope, creates new scope
3. **Indentation decrease**: Pops scopes deeper than current indent

### Scope Hierarchy

```
services:                 ← Scope A (indent 0)
  web:                    ← Scope B (indent 2, child of A)
    host: localhost       ← Adds to scope B (indent 4 > 2)
    port: 8000            ← Adds to scope B (indent 4 > 2)
  database:               ← Scope C (indent 2, sibling of B)
    host: remotehost      ← Adds to scope C (indent 4 > 2)
    port: 5432            ← Adds to scope C (indent 4 > 2)
```

Key `host` appears in both scopes B and C — **not a duplicate** because they're different scopes.

## Verification

The fix is validated by existing tests:

1. **`test_same_key_in_different_scopes`**: Verifies that `host` in `web:` and `host` in `database:` are NOT duplicates (different scopes)
2. **`test_duplicate_key_nested_mapping`**: Verifies that `port: 8080` and `port: 8081` in the SAME scope ARE duplicates
3. **`test_complex_nested_structure_no_duplicates`**: Verifies deeply nested scopes are properly isolated

## Summary

- **Current implementation**: Uses `indent >= scope_indent` to check if a key belongs to a scope
- **Root cause**: This allows keys at same indent as parent to be added, which violates YAML scoping rules
- **Fix**: Change to `indent > scope_indent` (strict inequality)
- **Impact**: Properly isolates sibling scopes, preventing false duplicates and catching true duplicates
