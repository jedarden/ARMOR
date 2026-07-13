# Duplicate Key Detection Analysis - bf-1csnvk

**Date:** 2026-07-13
**Component:** YAML Syntax Detector
**File:** `src/parsers/yaml/syntax_detector.rs`

---

## Executive Summary

The duplicate key detection logic in `syntax_detector.rs` had **two fundamental flaws** that produced false positives:

1. **Global duplicate detection** - Flagged ANY key appearing more than once in the entire document, regardless of nesting context
2. **No flow-context awareness** - Incorrectly parsed flow-style YAML (e.g., `{name: value}`) and extracted partial keys like `{name`

Both issues have since been fixed in commit `b1599939` (see `final_test_report.md` for details).

---

## Current Implementation

### Structure: How Keys Are Tracked

The `StructureState` struct (lines 268-277) tracks duplicate keys:

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

**Key insight:** The detector uses a **single flat `HashSet`** (`current_keys`) to track keys, clearing it when indentation decreases to exit nested contexts.

### Duplicate Detection Logic

The `detect_duplicate_key_errors()` function (lines 649-708) implements same-level duplicate detection:

**Flow control:**
1. Skip when inside flow-style contexts (`[]` or `{}`) - lines 653-655
2. Track indentation changes and clear keys when exiting nested contexts - lines 660-665
3. Extract keys from key-value pairs (e.g., `key: value` → `key`) - line 671
4. Check `current_keys` set and report duplicates - lines 698-707

**Scope management (lines 660-668):**
```rust
// Clear keys when exiting a nested context
if indent < self.structure_state.prev_indent {
    self.structure_state.current_keys.clear();
}
self.structure_state.prev_indent = indent;
```

This approach correctly handles same-level duplicate detection:
- Keys are added to `current_keys` when encountered
- When indentation decreases, the set is cleared (exiting nested scope)
- Duplicates within the same scope are detected via the `HashSet`

---

## Root Causes of False Positives

### Issue 1: Global Duplicate Detection (REMOVED)

**Location:** `syntax_detector.rs` lines 736-748 (function `finalize_structure_checks()`)

**What was wrong:**
The detector had a **global duplicate check** that reported ANY key appearing more than once in the entire YAML document:

```rust
for (key, line_nums) in &self.structure_state.all_keys {
    if line_nums.len() > 1 {
        // Reported ANY key appearing >1 time globally
        errors.push(ValidationError::new(
            format!("key_{}", key),
            format!("duplicate key '{}' appears {} times in document", key, line_nums.len())
        ).with_line(line_nums[0]));
    }
}
```

**Why this is incorrect:**
In YAML, the same key name is **valid** in different nested contexts:

```yaml
server:
  host: localhost    # server.host
  port: 8080
database:
  host: db.example   # database.host - NOT a duplicate!
  port: 5432
```

The global check would incorrectly flag `host` and `port` as duplicates, even though they belong to different parent mappings (`server` vs `database`).

**Impact:**
- Caused 2 of 3 test failures:
  - `test_valid_complete_yaml`
  - `test_complex_nested_structure`

**Fix:**
Global duplicate detection was **removed entirely** in commit `b1599939`. The same-level detection (lines 698-707) is sufficient and correct.

---

### Issue 2: No Flow-Context Awareness (FIXED)

**Location:** `syntax_detector.rs` lines 649-708 (function `detect_duplicate_key_errors()`)

**What was wrong:**
The detector did not check if it was parsing inside flow-style YAML syntax (within `{}` or `[]`). When it encountered flow-style mappings, it incorrectly extracted partial keys:

```yaml
items: [
  {name: item1, tags: [a, b]},  # Detector extracted "{name" as a key
  {name: item2, tags: [c, d]}   # Flagged as duplicate of "{name"
]
```

**Root cause:**
The key extraction logic (line 671) did this:
```rust
if let Some(colon_pos) = trimmed.find(':') {
    let key_part = &trimmed[..colon_pos];  // Everything before ':'
    // ...
    let key = key_part.trim();  // "{name" in flow-style context
```

When parsing `{name: item1}`, the `colon_pos` finds the first `:` inside the braces, and `key_part` becomes `{name`, which is then treated as a duplicate key.

**Impact:**
- Caused 1 of 3 test failures:
  - `test_complex_delimiter_balance`

**Fix:**
Added **flow-context tracking** to `DelimiterState`:

```rust
struct DelimiterState {
    // ... existing fields
    /// Whether we're inside flow-style context (within [] or {})
    in_flow_context: bool,
}
```

Updated `detect_duplicate_key_errors()` to skip processing when in flow context:
```rust
fn detect_duplicate_key_errors(&mut self, line: &str, line_num: usize, errors: &mut Vec<ValidationError>) {
    // Skip duplicate key detection when inside flow-style contexts
    if self.delimiter_state.in_flow_context {
        return;
    }
    // ... rest of logic
}
```

The `in_flow_context` flag is set to `true` when encountering `{` or `[`, and reset to `false` when all flow delimiters are closed.

---

## Code Locations Reference

| Component | Location | Purpose |
|-----------|----------|---------|
| `StructureState` struct | Lines 268-277 | State tracking for structure analysis |
| `current_keys` field | Line 274 | Keys seen at current indentation level |
| `DelimiterState` struct | Lines 244-259 | State tracking for delimiter analysis |
| `in_flow_context` field | Line 258 | Track if inside `[]` or `{}` |
| `detect_duplicate_key_errors()` | Lines 649-708 | Main duplicate key detection logic |
| Flow-context check | Lines 653-655 | Skip detection in flow-style YAML |
| Indentation scope management | Lines 660-668 | Clear keys when exiting nested contexts |
| Parent key detection | Lines 672-684 | Identify keys with nested content |
| Same-level duplicate check | Lines 698-707 | Check `current_keys` set for duplicates |

---

## How the Logic Works Now (Post-Fix)

### Correct Behavior Example

```yaml
top:
  level1: value1
  level2: value2
other:
  level1: value3  # NOT a duplicate (different parent context)
```

**Execution trace:**

1. Line 1 (`top:`): Parent key, clears `current_keys`, returns early
2. Line 2 (`  level1: value1`): Adds `"level1"` to `current_keys`
3. Line 3 (`  level2: value2`): Adds `"level2"` to `current_keys`
4. Line 4 (`other:`): Indent decreases (0 < 2), clears `current_keys`. Parent key, returns early.
5. Line 5 (`  level1: value3`): Adds `"level1"` to `current_keys` (not a duplicate - set was cleared)

**Result:** No false positive. The duplicate detector correctly understands that `level1` under `other` is distinct from `level1` under `top`.

### Flow-Style Handling Example

```yaml
items: [
  {name: item1, value: 100},
  {name: item2, value: 200}
]
```

**Execution trace:**

1. Line 1 (`items:`): Parent key, clears `current_keys`
2. Line 2 (`  [`): Sets `in_flow_context = true`
3. Line 3 (`  {name: item1, value: 100},`): Skips duplicate detection (`in_flow_context` is true)
4. Line 4 (`  {name: item2, value: 200}`): Skips duplicate detection
5. Line 5 (`]`): Resets `in_flow_context = false`

**Result:** No false positive. The detector correctly skips flow-style content.

---

## Design Decisions and Trade-offs

### Why Use a Single HashSet Instead of a Stack?

The detector uses a **single `current_keys` set** cleared on indentation decrease rather than a **stack of sets** per nesting level.

**Advantages:**
- Simpler implementation (no stack management)
- Sufficient for YAML's indentation-based scoping
- Lower memory overhead

**Disadvantages:**
- Cannot track keys in sibling contexts simultaneously (not needed for YAML)
- Assumes indentation always strictly increases/decreases (valid for well-formed YAML)

**Why this works:**
YAML's scope is defined by indentation. When you exit a nested structure (indentation decreases), all keys from that scope become irrelevant. A simple clear operation is sufficient - no need to maintain a hierarchy of key sets.

### Why Skip Flow-Style Contexts Entirely?

Flow-style YAML (`{key: value}` and `[item1, item2]`) uses inline syntax where comma-separated values and colons inside braces/brackets don't follow the same rules as block-style YAML.

**Reasoning:**
- Flow-style mappings are self-contained - they don't mix with block-style scoping
- Parsing flow-style requires a full tokenizer (complex)
- The detector's line-by-line approach can't reliably track flow-style nesting
- Skipping flow-style entirely avoids false positives without losing value

**Trade-off:**
Duplicate keys within flow-style mappings (e.g., `{a: 1, a: 2}`) are NOT detected. This is acceptable because:
1. Flow-style is typically used for simple, short structures
2. A full YAML parser would catch these errors during validation
3. The detector focuses on block-style YAML where indentation-based scoping applies

---

## Testing Context

The duplicate key detection is validated by the `syntax_detector_tests` test suite:

**Test location:** `src/parsers/yaml/syntax_detector_tests.rs`

**Relevant tests:**
- `test_detect_duplicate_keys_same_level` - Validates true duplicate detection
- `test_detect_nested_duplicate_keys` - Validates nested context handling
- `test_flow_style_with_braces` - Regression test for flow-style false positives
- `test_flow_style_with_brackets` - Regression test for flow-style arrays

**Test results (as of 2026-07-13):**
- **57/57 tests passing** (100% pass rate)
- All duplicate key detection tests passing
- No false positives on nested contexts
- No false positives on flow-style YAML

---

## Summary

### Current State
✅ **Fixed** - Both root causes addressed in commit `b1599939`

### What Was Changed
1. **Removed:** Global duplicate key detection (`finalize_structure_checks()`)
2. **Added:** Flow-context tracking (`in_flow_context` flag)
3. **Result:** 100% test pass rate, no false positives

### Key Takeaways
- Same-level duplicate detection (within a single mapping) works correctly
- Keys in different nested contexts are **not** duplicates (e.g., `server.host` vs `database.host`)
- Flow-style YAML requires special handling to avoid false positives
- The detector's indentation-based scoping is simpler but effective for block-style YAML

---

**Analysis completed:** 2026-07-13
**Component status:** ✅ Fully functional, no outstanding issues
