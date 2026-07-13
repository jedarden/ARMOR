# Test Failure Analysis for YAML Syntax Detector

> **Analysis Date:** 2026-07-13
> **Bead Context:** Step 2 of bf-3o3g6l, building on baseline capture from bf-67kemy
> **Test Results Source:** test_results.txt

---

## Executive Summary

**Total Tests Run:** 54
**Passed:** 51 (94.4%)
**Failed:** 3 (5.6%)
**Failure Type:** All assertion failures (panic on `errors.is_empty()` check)

### Failure Categories
| Category | Count | Tests |
|----------|-------|-------|
| **False Positive: Global duplicate detection** | 2 | test_valid_complete_yaml, test_complex_nested_structure |
| **False Positive: Flow-style parsing** | 1 | test_complex_delimiter_balance |

---

## Failing Tests

### 1. test_complex_delimiter_balance
**Location:** `src/parsers/yaml/syntax_detector_tests.rs:495`
**Test Module:** `parsers::yaml::syntax_detector_tests::delimiter_tests`

**Input YAML:**
```yaml
items: [
  {name: item1, tags: [a, b]},
  {name: item2, tags: [c, d]}
]
```

**Failure Details:**
```
thread '...test_complex_delimiter_balance' panicked at src/parsers/yaml/syntax_detector_tests.rs:495:9:
assertion failed: errors.is_empty()

DEBUG - Found 2 errors in complex delimiter YAML:
  0 path=key_{name line=Some(3) msg=duplicate key '{name' at same indentation level
  1 path=key_{name line=Some(2) msg=duplicate key '{name' appears 2 times in document
```

**Root Cause:**
The duplicate key detector is incorrectly parsing flow-style YAML. The `{name:` pattern inside flow-style mappings is being treated as a key when it's actually part of the flow syntax. The detector sees:
- Line 2: `{name: item1, tags: [a, b]}` - extracts `{name` as a key
- Line 3: `{name: item2, tags: [c, d]}` - extracts `{name` as a key again
- Reports this as a duplicate

**Failure Category:** Flow-style YAML parsing bug / False positive

**Estimated Fix Complexity:** Medium
- Requires detecting when inside flow-style context (brackets/braces)
- Need to skip duplicate key detection within flow-style mappings
- May need to track delimiter state before checking for keys

---

### 2. test_valid_complete_yaml
**Location:** `src/parsers/yaml/syntax_detector_tests.rs:641`
**Test Module:** `parsers::yaml::syntax_detector_tests::integration_tests`

**Input YAML:**
```yaml
# Configuration file
server:
  host: localhost
  port: 8080
database:
  host: db.example.com
  port: 5432
  name: mydb
features:
  - authentication
  - logging
  - caching
env:
  production: true
  debug: false
```

**Failure Details:**
```
thread '...test_valid_complete_yaml' panicked at src/parsers/yaml/syntax_detector_tests.rs:641:9:
assertion failed: errors.is_empty()

DEBUG - Found 2 errors in valid YAML:
  0 path=key_host line=Some(4) msg=duplicate key 'host' appears 2 times in document
  1 path=key_port line=Some(5) msg=duplicate key 'port' appears 2 times in document
```

**Root Cause:**
The global duplicate key detection is fundamentally flawed. Keys with the same name appearing in different nested contexts (e.g., `server.host` and `database.host`) are being reported as duplicates.

In this test:
- Line 4: `host: localhost` (under `server:`)
- Line 6: `host: db.example.com` (under `database:`)

These are NOT duplicates because they're in different parent contexts. The detector should only report duplicates within the same mapping level.

**Failure Category:** Logic error in duplicate key detection scope

**Estimated Fix Complexity:** Low
- The global duplicate detection (lines 736-748 in syntax_detector.rs) should be removed entirely
- Keys in different nested contexts are valid YAML and should not be flagged

---

### 3. test_complex_nested_structure
**Location:** `src/parsers/yaml/syntax_detector_tests.rs:701`
**Test Module:** `parsers::yaml::syntax_detector_tests::integration_tests`

**Input YAML:**
```yaml
services:
  web:
    host: localhost
    port: 8080
    ssl:
      enabled: true
      cert: /path/to/cert.pem
  database:
    host: db.example.com
    port: 5432
    credentials:
      username: admin
      password: secret
deployment:
  environments:
    - name: dev
      url: dev.example.com
    - name: prod
      url: prod.example.com
```

**Failure Details:**
```
thread '...test_complex_nested_structure' panicked at src/parsers/yaml/syntax_detector_tests.rs:701:9:
assertion failed: errors.is_empty()
```

**Root Cause:**
Same as test_valid_complete_yaml - global duplicate key detection incorrectly flagging `host` and `port` keys that appear in different nested contexts (`services.web` and `services.database`).

**Failure Category:** Logic error in duplicate key detection scope

**Estimated Fix Complexity:** Low
- Same fix as test_valid_complete_yaml

---

## Common Root Causes

### 1. Global Duplicate Key Detection is Overly Aggressive

**Location:** `src/parsers/yaml/syntax_detector.rs:736-748`

The `finalize_structure_checks()` function implements global duplicate detection that reports ANY key appearing more than once in the document, regardless of context:

```rust
for (key, line_nums) in &self.structure_state.all_keys {
    if line_nums.len() > 1 {
        // Reports ANY key appearing >1 time globally
        errors.push(ValidationError::new(
            format!("key_{}", key),
            format!("duplicate key '{}' appears {} times in document", key, line_nums.len())
        ).with_line(line_nums[0]));
    }
}
```

**Problem:** This is fundamentally wrong for YAML. Keys like `host`, `port`, `name`, `id` commonly appear in different nested structures and are NOT duplicates.

**Correct Behavior:** Duplicate keys should only be detected within the same mapping level (same parent context). The same-level detection (lines 687-694) is correct; the global detection is incorrect.

### 2. Flow-Style YAML Not Recognized

**Location:** `src/parsers/yaml/syntax_detector.rs:643-702`

The `detect_duplicate_key_errors()` function does not check if it's inside flow-style syntax (within `{}` or `[]`). When parsing flow-style YAML like `{name: value}`, it incorrectly extracts `{name` as a key name.

**Problem:** The detector needs delimiter context awareness to skip parsing when inside flow-style mappings/sequences.

---

## Prioritized Fix List

### Priority 1: Remove Global Duplicate Detection (Low complexity, high impact)

**Files to modify:**
- `src/parsers/yaml/syntax_detector.rs`

**Changes:**
1. Remove lines 736-748 (`finalize_structure_checks()` function body, or entire function if only used for global duplicates)
2. Remove `all_keys` field from `StructureState` struct (line 298)
3. Remove all code that populates `all_keys` (lines 696-700)

**Impact:** Fixes 2 of 3 failing tests (test_valid_complete_yaml, test_complex_nested_structure)

**Estimated effort:** 15 minutes

**Risk:** Low - The global detection is producing false positives; removing it restores correct behavior.

---

### Priority 2: Fix Flow-Style YAML Parsing (Medium complexity, medium impact)

**Files to modify:**
- `src/parsers/yaml/syntax_detector.rs`

**Changes:**
1. Add flow-context tracking to `DelimiterState`:
   ```rust
   struct DelimiterState {
       bracket_stack: Vec<(char, usize)>,
       in_flow_context: bool,  // NEW: Track if we're inside [] or {}
       // ... rest of fields
   }
   ```

2. In `detect_delimiter_errors()`, set `in_flow_context = true` when encountering `{` or `[`, and `false` when all flow delimiters are closed

3. In `detect_duplicate_key_errors()`, skip processing if `in_flow_context` is true:
   ```rust
   if self.delimiter_state.in_flow_context {
       return;
   }
   ```

4. Alternatively, enhance the key extraction logic to ignore flow-style patterns like `{key:` or `[item`

**Impact:** Fixes 1 of 3 failing tests (test_complex_delimiter_balance)

**Estimated effort:** 30-45 minutes

**Risk:** Medium - Requires careful state management to correctly track nested flow contexts

---

### Priority 3: Add Regression Tests (Low complexity, preventive)

**Files to modify:**
- `src/parsers/yaml/syntax_detector_tests.rs`

**Changes:**
Add new tests to prevent future regressions:
1. Test for flow-style arrays with objects
2. Test for same key names in different nested contexts
3. Test for complex nested flow-structures

**Estimated effort:** 20 minutes

**Risk:** None - Tests only validate behavior

---

## Additional Findings

### Compiler Warnings

The test output shows 15 compiler warnings, including:
- 1 unused import: `ValidationError` (line 10)
- 12 unused variables (can be prefixed with `_`)
- 1 unused struct field: `check_duplicate_keys` (never read)
- 1 useless comparison: `config.port > 65535` (u16 can't exceed 65535)

**Note:** The unused `check_duplicate_keys` field is interesting - it suggests the feature was planned but the configuration option isn't wired into the detection logic.

---

## Recommended Fix Order

1. **First:** Priority 1 - Remove global duplicate detection (quick win, fixes 66% of failures)
2. **Second:** Priority 2 - Fix flow-style YAML parsing (fixes remaining 33% of failures)
3. **Third:** Priority 3 - Add regression tests (prevents future breakage)

---

## Test Results Summary

| Test Name | Status | Category | Fix Priority |
|----------|--------|----------|--------------|
| test_complex_delimiter_balance | ❌ FAILED | Flow-style parsing | 2 |
| test_complex_nested_structure | ❌ FAILED | Global duplicate detection | 1 |
| test_valid_complete_yaml | ❌ FAILED | Global duplicate detection | 1 |

**Total estimated fix time:** 65-80 minutes
**Tests fixed after Priority 1:** 2 of 3
**Tests fixed after Priority 2:** 3 of 3 (100%)
