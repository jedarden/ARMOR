# Failing Integration Tests Documentation

## Summary

This document catalogs all failing integration tests with specific assertion errors and patterns.

**Total failing tests**: 3  
**Test file**: `tests/comprehensive_scope_tracking_test.rs`  
**Module**: `armor::parsers.yaml.scope`

---

## Test Failures

### 1. test_enter_scope_creates_new_scope

**Location**: `tests/comprehensive_scope_tracking_test.rs:187:5`

**Assertion Error**:
```
assertion `left == right` failed
  left: 2
 right: 1
```

**Expected**: After calling `stack.enter_scope()`, the stack depth should increase by 1 (from 0 to 1)  
**Actual**: Stack depth is 2

**Root Cause**: The `enter_scope()` implementation auto-creates a root scope if the stack is empty:

```rust
// From src/parsers/yaml/scope.rs:748-751
pub fn enter_scope(&mut self, indent_level: usize, line: usize, parent_key: Option<String>) {
    // Auto-create root scope if stack is empty
    if self.scopes.is_empty() {
        self.scopes.push(Scope::new(0, 0, None));
    }
```

**Pattern**: Off-by-one error due to implicit scope creation

**Test Code**:
```rust
// Line 181-190
#[test]
fn test_enter_scope_creates_new_scope() {
    let mut stack = ScopeStack::new(2);
    let initial_depth = stack.depth();  // Returns 0

    stack.enter_scope(2, 1, Some("parent".to_string()));

    assert_eq!(stack.depth(), initial_depth + 1);  // Expects 0 + 1 = 1, gets 2
    assert_eq!(stack.current_indent(), 2);
    assert_eq!(stack.get_scope_path(), "parent");
}
```

---

### 2. test_scope_at_zero_indent

**Location**: `tests/comprehensive_scope_tracking_test.rs:833:5`

**Assertion Error**:
```
assertion failed: scope.is_some()
```

**Expected**: There should be a scope at indent level 0  
**Actual**: No scope exists at level 0 (returns `None`)

**Root Cause**: The `ScopeStack::new()` constructor initializes with an empty vector, so there's no root scope until one is explicitly created or auto-created via `enter_scope()`:

```rust
// From src/parsers/yaml/scope.rs:687-695
pub fn new(base_indent: usize) -> Self {
    Self {
        scopes: Vec::new(), // Empty stack - initialized with no scopes
        base_indent,
        sequence_item_counter: 0,
        indent_transitions: Vec::new(),
        last_indent: 0,
    }
}
```

**Pattern**: Missing initialization of root scope

**Test Code**:
```rust
// Line 829-835
#[test]
fn test_scope_at_zero_indent() {
    let stack = ScopeStack::new(2);
    let scope = stack.get_scope_at_level(0);
    assert!(scope.is_some());  // Fails - no scope at level 0
    assert_eq!(scope.unwrap().indent_level, 0);
}
```

---

### 3. test_scope_stack_reset_clears_all_scopes

**Location**: `tests/comprehensive_scope_tracking_test.rs:63:5`

**Assertion Error**:
```
assertion `left == right` failed
  left: 0
 right: 1
```

**Expected**: After calling `stack.reset()`, the stack should have depth 1 (presumably a root scope)  
**Actual**: Stack depth is 0 (all scopes cleared)

**Root Cause**: The `reset()` implementation clears all scopes without leaving a root scope:

```rust
// From src/parsers/yaml/scope.rs:1135-1142
pub fn reset(&mut self) {
    self.scopes.clear();
    self.clear_indent_transitions();
}
```

**Pattern**: Inconsistent expectations about reset behavior

**Test Code**:
```rust
// Line 51-65
#[test]
fn test_scope_stack_reset_clears_all_scopes() {
    let mut stack = ScopeStack::new(2);
    stack.enter_scope(2, 1, Some("first".to_string()));
    stack.enter_scope(4, 2, Some("second".to_string()));
    stack.add_key("key1", 3).unwrap();

    assert_eq!(stack.depth(), 3);  // Passes - has 3 scopes (root + 2 entered)
    assert!(stack.contains_key("key1"));

    stack.reset();

    assert_eq!(stack.depth(), 1);  // Fails - depth is 0, expects 1
    assert!(!stack.contains_key("key1"));
}
```

---

## Error Patterns

### 1. Off-by-One Errors
The most common pattern is off-by-one errors due to implicit scope creation. The `enter_scope()` method automatically creates a root scope when called on an empty stack, which affects depth calculations.

### 2. Initialization Inconsistency
Tests expect a root scope to exist at level 0 immediately after construction, but the implementation starts with an empty stack and only creates scopes on demand.

### 3. Reset Behavior Ambiguity
The `reset()` method clears all scopes completely, but tests expect it to leave a root scope (depth = 1).

---

## Implementation Analysis

### ScopeStack Lifecycle

1. **Initial State**: Empty stack (`scopes: Vec::new()`)
2. **After enter_scope()**: Auto-creates root scope if empty, then pushes new scope
3. **After reset()**: Returns to initial empty state

### Test Expectations vs Implementation

| Test | Expects | Implementation | Gap |
|------|---------|----------------|-----|
| test_enter_scope_creates_new_scope | Depth increases by 1 | Depth increases by 2 (root + new) | Implicit root creation |
| test_scope_at_zero_indent | Scope at level 0 exists | No scopes until enter_scope() | Missing initialization |
| test_scope_stack_reset_clears_all_scopes | Reset leaves depth 1 | Reset leaves depth 0 | Different reset semantics |

---

## Recommendations

1. **Decide on initialization model**: Either auto-create root scope in `new()` or keep current lazy initialization
2. **Make enter_scope() behavior consistent**: If auto-creating root scope, document it clearly or make it explicit
3. **Clarify reset() semantics**: Should it leave a root scope or truly empty the stack?
4. **Add tests for both models**: If changing behavior, ensure both explicit and implicit initialization patterns are tested

---

## Test Execution Summary

```
Total tests run: 351 + 5 + 19 + 65 + 351 + 5 + 19 + 65 = 880
Passing: 877
Failing: 3
Error rate: 0.34%

All failures are in: tests/comprehensive_scope_tracking_test.rs
```

---

**Generated**: 2026-07-13  
**Bead ID**: bf-2ey0v2  
**Task**: Document all failing integration tests
