# Syntax Detector Test Failures Analysis

**Test Command**: `cargo test --lib syntax_detector_tests`
**Test Run Date**: 2026-07-13
**Total Tests**: 54 tests
**Failed Tests**: 3 tests
**Passed Tests**: 51 tests

## Executive Summary

Three syntax_detector tests are failing due to two distinct bugs in the duplicate key detection logic:
1. **Bug #1**: Opening braces `{` in flow-style mappings are incorrectly included in key names
2. **Bug #2**: Global duplicate detection doesn't account for nested contexts (keys in different parent scopes are incorrectly flagged as duplicates)

## Test Results Summary

| Test Category | Total | Passed | Failed |
|---------------|-------|--------|--------|
| Indentation Tests | 14 | 14 | 0 |
| Delimiter Tests | 18 | 17 | 1 |
| Structure Tests | 6 | 6 | 0 |
| Integration Tests | 3 | 0 | 3 |
| Regression Tests | 8 | 8 | 0 |
| Performance Tests | 2 | 2 | 0 |

## Detailed Failure Analysis

### Failure 1: test_complex_delimiter_balance

**Test Location**: `src/parsers/yaml/syntax_detector_tests.rs:480`
**Test Type**: Delimiter balance in nested flow-style structures

**YAML Input**:
```yaml
items: [
  {name: item1, tags: [a, b]},
  {name: item2, tags: [c, d]}
]
```

**Error Messages**:
```
0 path=key_{name line=Some(3) msg=duplicate key '{name' at same indentation level
1 path=key_{name line=Some(2) msg=duplicate key '{name' appears 2 times in document
```

**Root Cause**: 
The key extraction logic in `detect_duplicate_key_errors()` (line 659) incorrectly includes the opening brace `{` as part of the key name. When parsing the line `{name: item1, tags: [a, b]}`, the code extracts everything before the colon as the key, resulting in `{name` instead of `name`.

**Code Location**: `src/parsers/yaml/syntax_detector.rs:659`
```rust
if let Some(colon_pos) = trimmed.find(':') {
    let key_part = &trimmed[..colon_pos];
    // key_part is "{name" instead of "name"
```

**Failure Type**: Logic error (incorrect key extraction in flow-style contexts)

---

### Failure 2: test_valid_complete_yaml

**Test Location**: `src/parsers/yaml/syntax_detector_tests.rs:602`
**Test Type**: Complete valid YAML configuration

**YAML Input**:
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

**Error Messages**:
```
0 path=key_host line=Some(4) msg=duplicate key 'host' appears 2 times in document
1 path=key_port line=Some(5) msg=duplicate key 'port' appears 2 times in document
```

**Root Cause**: 
The global duplicate key detection logic in `finalize_structure_checks()` (line 736-748) doesn't account for nested contexts. Keys `host` and `port` appear in different parent scopes (`server` vs `database`), but the detector treats them as document-level duplicates.

**Code Location**: `src/parsers/yaml/syntax_detector.rs:736-748`
```rust
fn finalize_structure_checks(&mut self, errors: &mut Vec<ValidationError>) {
    // Check for global duplicate keys
    for (key, line_nums) in &self.structure_state.all_keys {
        if line_nums.len() > 1 {
            let is_same_level = line_nums.windows(2).all(|w| w[0] == w[1]);
            if !is_same_level {
                errors.push(ValidationError::new(
                    format!("key_{}", key),
                    format!("duplicate key '{}' appears {} times in document", key, line_nums.len())
                ).with_line(line_nums[0]));
            }
        }
    }
}
```

**Problem**: The logic incorrectly treats all keys with the same name as duplicates, regardless of their nested context. `server.host` and `database.host` should be considered distinct keys.

**Failure Type**: Logic error (global duplicate detection doesn't respect nested contexts)

---

### Failure 3: test_complex_nested_structure

**Test Location**: `src/parsers/yaml/syntax_detector_tests.rs:666`
**Test Type**: Complex nested configuration

**YAML Input**:
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

**Error Messages**:
```
0 path=key_host line=Some(4) msg=duplicate key 'host' appears 2 times in document
1 path=key_port line=Some(5) msg=duplicate key 'port' appears 2 times in document
2 path=key_url line=Some(18) msg=duplicate key 'url' appears 2 times in document
```

**Root Cause**: 
Same as Failure 2 - the global duplicate detection logic doesn't respect nested contexts. Keys `host`, `port`, and `url` appear in different parent scopes:
- `host` and `port` appear under both `services.web` and `services.database`
- `url` appears under both `deployment.environments[0]` and `deployment.environments[1]`

**Failure Type**: Logic error (global duplicate detection doesn't respect nested contexts)

---

## Error Categorization

### By Error Type

| Error Type | Count | Tests Affected |
|-------------|-------|----------------|
| Logic Error - Key Extraction | 1 | test_complex_delimiter_balance |
| Logic Error - Global Duplicate Detection | 3 | test_valid_complete_yaml, test_complex_nested_structure |
| **Total** | **4** | **3 tests** |

### By Assertion Type

| Assertion Type | Count | Tests Affected |
|----------------|-------|----------------|
| `assert!(errors.is_empty())` | 3 | All 3 failing tests |
| **Total** | **3** | **3 tests** |

## Root Causes Summary

### Bug #1: Flow-Style Key Extraction Bug

**Symptom**: Opening braces `{` included in key names
**Impact**: test_complex_delimiter_balance
**File**: `src/parsers/yaml/syntax_detector.rs`
**Function**: `detect_duplicate_key_errors()`
**Line**: ~659

The key extraction logic needs to skip leading delimiters (braces, brackets) before extracting the key name.

### Bug #2: Global Duplicate Detection Doesn't Respect Context

**Symptom**: Keys in different nested scopes flagged as duplicates
**Impact**: test_valid_complete_yaml, test_complex_nested_structure  
**File**: `src/parsers/yaml/syntax_detector.rs`
**Function**: `finalize_structure_checks()`
**Line**: ~736-748

The global duplicate detection logic should track keys as hierarchical paths (e.g., `server.host` and `database.host` as distinct keys) rather than just flat key names.

## Test Coverage Notes

- **51/54 tests passing (94.4% pass rate)**
- All indentation tests pass (14/14)
- Most delimiter tests pass (17/18)  
- All structure tests pass (6/6)
- All regression tests pass (8/8)
- All performance tests pass (2/2)
- Integration tests have the most failures (0/3)

## Recommended Fixes

### Fix for Bug #1: Flow-Style Key Extraction

In `detect_duplicate_key_errors()`, trim leading delimiters before extracting keys:
```rust
let key = key_part.trim();
let key = key.trim_start_matches(|c| c == '{' || c == '[' || c == '(');
```

### Fix for Bug #2: Context-Aware Duplicate Detection

1. Track hierarchical key paths instead of flat keys
2. Only flag duplicates within the same parent context
3. Alternatively, disable global duplicate detection and only detect duplicates at the same indentation level

## Additional Observations

1. **Same-level duplicate detection works correctly**: The detector correctly identifies `key: value1\nkey: value2` as a duplicate
2. **Context clearing on parent keys works**: When encountering a parent key (e.g., `server:`), the detector clears the current keys
3. **Indentation-based context switching works**: The detector clears keys when indentation decreases

The core issue is that the `all_keys` HashMap stores flat key names without context, while the `current_keys` HashSet (for same-level detection) properly respects context by clearing on scope changes.
