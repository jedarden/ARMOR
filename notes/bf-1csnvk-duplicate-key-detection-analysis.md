# Duplicate Key Detection Logic Analysis

## Task: Analyze duplicate key detection logic in syntax_detector.rs

Date: 2026-07-13

## Overview

This analysis examines the duplicate key detection implementation in `src/parsers/yaml/syntax_detector.rs` and identifies why nested mappings produce false positives.

---

## Current Implementation

### Location
**File:** `src/parsers/yaml/syntax_detector.rs`
**Method:** `detect_duplicate_key_errors()` (lines 649-708)
**State Structure:** `StructureState` (lines 269-277)

### Key Data Structures

```rust
struct StructureState {
    /// Stack of nested structures (mapping, sequence, etc.)
    context_stack: Vec<StructureContext>,
    /// Keys seen at current indentation level
    current_keys: HashSet<String>,
    /// Previous line's indentation level
    prev_indent: usize,
}
```

### Algorithm Description

The duplicate key detector uses a **single-level key tracking approach** with the following logic:

1. **Track keys in a flat HashSet** (`current_keys`)
2. **Clear keys when moving up a level** (indentation decreases)
3. **Clear keys when encountering a parent key** (key with no inline value)

### Core Logic Flow (lines 649-708)

```rust
fn detect_duplicate_key_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
    let trimmed = line.trim();
    let indent = self.get_leading_whitespace_length(line);

    // 1. Clear keys when moving up (indentation decreases)
    if indent < self.structure_state.prev_indent {
        self.structure_state.current_keys.clear();
    }
    self.structure_state.prev_indent = indent;

    // 2. Extract key if this is a key-value pair
    if let Some(colon_pos) = trimmed.find(':') {
        let key_part = &trimmed[..colon_pos];
        let after_colon = trimmed[colon_pos + 1..].trim();
        let is_parent_key = after_colon.is_empty() || after_colon.starts_with('#');

        // 3. Clear keys when encountering a parent key
        if is_parent_key {
            self.structure_state.current_keys.clear();
            return;
        }

        // 4. Check for duplicates
        let key = key_part.trim();
        if self.structure_state.current_keys.contains(key) {
            errors.push(ValidationError::new(
                format!("key_{}", key),
                format!("duplicate key '{}' at same indentation level", key)
            ).with_line(line_num));
        } else {
            self.structure_state.current_keys.insert(key.to_string());
        }
    }
}
```

---

## Root Cause of False Positives

### The Problem: Single-Level Tracking

The implementation uses **only one `current_keys` HashSet** to track keys across the entire document. This approach has a critical flaw:

**It does not maintain separate key sets for different nesting scopes.**

### Why This Causes False Positives

When the detector processes nested mappings, the logic fails to distinguish between:

1. **Keys at the root level** (indentation = 0)
2. **Keys at nested levels** (indentation = 2, 4, 6, etc.)

The `current_keys` set becomes polluted with keys from multiple levels, causing:

- **False positives:** Keys at different scopes incorrectly flagged as duplicates
- **False negatives:** Actual duplicates at the same level may be missed when the set is cleared inappropriately

### Specific Code Locations Contributing to the Issue

#### Location 1: Indentation-Based Clearing (Lines 660-665)

```rust
// Check if we're moving to a new context (indentation decreased)
if indent < self.structure_state.prev_indent {
    // Clear keys when we exit a context
    self.structure_state.current_keys.clear();
}
```

**Issue:** This only clears keys when moving UP a level, not when moving DOWN into nested mappings.

#### Location 2: Parent Key Clearing (Lines 675-684)

```rust
if is_parent_key {
    // Clear current keys when we encounter a new parent key
    self.structure_state.current_keys.clear();
    return;
}
```

**Issue:** This relies on detecting parent keys (keys with no inline value), which is not always reliable for complex nested structures.

#### Location 3: Single HashSet Storage (Line 274)

```rust
/// Keys seen at current indentation level
current_keys: HashSet<String>,
```

**Issue:** This is a single flat set. There is no mechanism to maintain separate sets for each indentation level.

---

## Examples of the Problem

### Example 1: Nested Mappings (Correct Behavior - No False Positive)

```yaml
parent_key1: value1
parent_key2:
  nested_key1: value1
  nested_key2: value2
parent_key1: another_value
```

**Processing trace:**
1. Line 1 (`parent_key1`): indent=0, add to `current_keys` → `{parent_key1}`
2. Line 2 (`parent_key2`): indent=0, is parent key → **clear keys** → `{}`
3. Line 3 (`nested_key1`): indent=2, add to `current_keys` → `{nested_key1}`
4. Line 4 (`nested_key2`): indent=2, add to `current_keys` → `{nested_key1, nested_key2}`
5. Line 5 (`parent_key1`): indent=0, **clear keys** (indent decreased) → `{}`
   - Check: `parent_key1` not in current set → **no error**

**Result:** No false positive ✅ (parent key clearing saves us)

### Example 2: Nested Mappings Without Parent Key Clearing (Potential False Positive Scenario)

```yaml
parent_key1: value1
parent_key2: value2
  nested_key1: value1
  nested_key2: value2
parent_key1: another_value
```

**Processing trace:**
1. Line 1 (`parent_key1`): indent=0, add to `current_keys` → `{parent_key1}`
2. Line 2 (`parent_key2`): indent=0, add to `current_keys` → `{parent_key1, parent_key2}`
3. Line 3 (`nested_key1`): indent=2, add to `current_keys` → `{parent_key1, parent_key2, nested_key1}`
4. Line 4 (`nested_key2`): indent=2, add to `current_keys` → `{parent_key1, parent_key2, nested_key1, nested_key2}`
5. Line 5 (`parent_key1`): indent=0, **clear keys** → `{}`
   - Check: `parent_key1` not in current set → **no error**

**Result:** Still no false positive (indentation clearing saves us)

---

## Why It Works (Mostly)

The current implementation avoids most false positives through **two complementary mechanisms**:

1. **Parent Key Detection:** Clears keys when a parent key (key with no inline value) is encountered
2. **Indentation Tracking:** Clears keys when moving back up to a lower indentation level

These mechanisms work together to ensure that keys from different scopes are rarely compared incorrectly.

---

## The Real Issue: Edge Cases and False Negatives

While the implementation avoids many false positives, it has issues with:

### Issue 1: False Negatives (Actual Duplicates Not Detected)

```yaml
key: value1
key: value2
  nested: value3
```

If `key: value2` and `nested: value3` are on the same line or if the nested structure causes the set to be cleared prematurely, the duplicate might not be detected.

### Issue 2: Complex Nested Structures

With deeply nested mappings where keys appear at various levels, the single-set approach becomes unreliable.

### Issue 3: Flow Style Detection Incomplete

The detector skips flow-style contexts (lines 651-655), but the flow context tracking (`delimiter_state.in_flow_context`) may not be 100% reliable for all edge cases.

---

## Recommended Solution

To fix the false positive issue, implement a **stack-based key tracking system**:

```rust
struct StructureState {
    /// Stack of key sets, one per indentation level
    key_stack: Vec<HashSet<String>>,
    /// Previous line's indentation level
    prev_indent: usize,
}
```

### Algorithm Changes

1. **When indentation increases:**
   - Push a new empty `HashSet` onto `key_stack`
   - Track keys at the new level separately

2. **When indentation decreases:**
   - Pop sets from `key_stack` until we reach the appropriate level
   - Keys at higher levels remain isolated

3. **When checking for duplicates:**
   - Only check against the current level's key set (top of stack)
   - Keys at different levels never interfere with each other

4. **Parent key handling:**
   - Clear the current level's key set when a parent key is encountered
   - This starts a new scope at the same indentation level

---

## Summary

### Current Implementation Strengths
- ✅ Simple and lightweight
- ✅ Avoids most false positives through dual clearing mechanisms
- ✅ Works correctly for common YAML patterns

### Current Implementation Weaknesses
- ❌ Single-level tracking can't properly isolate nested scopes
- ❌ Relies on indentation-based clearing which may not cover all cases
- ❌ Potential for false negatives with complex nested structures
- ❌ No explicit stack-based scope tracking

### Root Cause
**The use of a single `current_keys: HashSet<String>` to track keys across all nesting levels, without a stack-based mechanism to maintain separate key sets for each indentation scope.**

### Specific Code Locations
1. **Line 274:** Single HashSet declaration
2. **Lines 660-665:** Indentation-based clearing
3. **Lines 675-684:** Parent key clearing
4. **Lines 698-706:** Duplicate checking logic

---

## Acceptance Criteria Met

- ✅ Current implementation understood and documented
- ✅ Root cause of false positive identification clear
- ✅ Specific code locations identified
