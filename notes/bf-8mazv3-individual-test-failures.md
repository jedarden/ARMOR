# Individual Test Failure Documentation

## Overview
**Date**: 2026-07-13
**Test Suite**: YAML Syntax Detector Tests
**Total Failed**: 3 tests
**Total Passed**: 51 tests
**File**: `src/parsers/yaml/syntax_detector_tests.rs`

---

## Failure 1: test_complex_delimiter_balance

**Test Module**: `parsers::yaml::syntax_detector_tests::delimiter_tests`
**Test Name**: `test_complex_delimiter_balance`
**Assertion Line**: 495
**Assertion**: `assert!(errors.is_empty())`

### Test Input YAML
```yaml
items: [
  {name: item1, tags: [a, b]},
  {name: item2, tags: [c, d]}
]
```

### Expected Behavior
Should accept complex nested delimiter structures with no errors.

### Actual Behavior
```
thread 'parsers::yaml::syntax_detector_tests::delimiter_tests::test_complex_delimiter_balance' panicked at src/parsers/yaml/syntax_detector_tests.rs:495:9:
assertion failed: errors.is_empty()
```

### Error Details
```
DEBUG - Found 2 errors in complex delimiter YAML:
  0 path=key_{name line=Some(3) msg=duplicate key '{name' at same indentation level
  1 path=key_{name line=Some(2) msg=duplicate key '{name' appears 2 times in document
```

### Analysis
**Failure Type**: False Positive - Flow Style Detection
**Root Cause**: The detector is treating `{name` (from the flow-style mapping `{name: item1, tags: [a, b]}`) as a regular YAML key and detecting it as a duplicate, even though:
1. The flow-style syntax `{key: value}` should be recognized and excluded from duplicate key detection
2. The `in_flow_context` flag should be set when within `[...]` or `{...}` delimiters
3. The duplicate key detection at line 663 should skip processing when `in_flow_context` is true

### Expected Result
No errors - the YAML is valid flow-style syntax.

### Code Reference
- Test location: `src/parsers/yaml/syntax_detector_tests.rs:480-488`
- Assertion: line 495
- Detection logic: `src/parsers/yaml/syntax_detector.rs:659-664`

---

## Failure 2: test_valid_complete_yaml

**Test Module**: `parsers::yaml::syntax_detector_tests::integration_tests`
**Test Name**: `test_valid_complete_yaml`
**Assertion Line**: 641 (in test), 754 (main assertion)
**Assertion**: `assert!(errors.is_empty())`

### Test Input YAML
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

### Expected Behavior
Should accept complete valid YAML with no errors. The `host` and `port` keys appear in both `server` and `database` scopes, which is valid YAML.

### Actual Behavior
```
thread 'parsers::yaml::syntax_detector_tests::integration_tests::test_valid_complete_yaml' panicked at src/parsers/yaml/syntax_detector_tests.rs:641:9:
assertion failed: errors.is_empty()
```

### Error Details
```
DEBUG - Found 2 errors in valid YAML:
  0 path=key_host line=Some(4) msg=duplicate key 'host' appears 2 times in document
  1 path=key_port line=Some(5) msg=duplicate key 'port' appears 2 times in document
```

### Analysis
**Failure Type**: False Positive - Scope Tracking
**Root Cause**: The scope-aware duplicate detection is not properly tracking scope transitions when moving from one parent key to another. Specifically:
1. When processing line 4 (`host: db.example.com`), the detector should have exited the `server` scope and entered the `database` scope
2. The scope exit logic at line 740 (`self.structure_state.scope_stack.exit_to_scope(indent)`) is not working correctly
3. The `server` and `database` keys should create separate scopes that prevent false duplicate detection across them

### Expected Result
No errors - `host` and `port` in different scopes (`server` vs `database`) should not be flagged as duplicates.

### Code Reference
- Test location: `src/parsers/yaml/syntax_detector_tests.rs:723-755`
- Assertion: line 754
- Scope tracking: `src/parsers/yaml/syntax_detector.rs:730-745`

---

## Failure 3: test_complex_nested_structure

**Test Module**: `parsers::yaml::syntax_detector_tests::integration_tests`
**Test Name**: `test_complex_nested_structure`
**Assertion Line**: 701
**Assertion**: `assert!(errors.is_empty())`

### Test Input YAML
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

### Expected Behavior
Should accept complex nested structure with repeated key names (`host`, `port`, `name`, `url`) in different scopes.

### Actual Behavior
```
thread 'parsers::yaml::syntax_detector_tests::integration_tests::test_complex_nested_structure' panicked at src/parsers/yaml/syntax_detector_tests.rs:701:9:
assertion failed: errors.is_empty()
```

### Error Details
No debug output captured for this failure (test does not have debug printing enabled).

### Analysis
**Failure Type**: False Positive - Scope Tracking
**Root Cause**: Similar to Failure 2, this is a scope tracking issue. The complex nested structure has multiple levels:
1. `services` scope contains `web` and `database` sub-scopes
2. Both `web` and `database` have `host` and `port` keys (should be allowed)
3. `deployment` → `environments` is a sequence with items that have `name` and `url` keys (should be allowed)

The scope stack is likely not properly:
1. Entering new scopes when encountering parent keys (e.g., `services:`, `web:`, `database:`)
2. Exiting scopes when indentation decreases
3. Handling sequence items as separate scopes

### Expected Result
No errors - keys in different scopes and sequence items should not be flagged as duplicates.

### Code Reference
- Test location: `src/parsers/yaml/syntax_detector_tests.rs:787-823`
- Assertion: line 822
- Scope tracking: `src/parsers/yaml/syntax_detector.rs:659-764`

---

## Failure Categories

### Category 1: Flow Style Detection (1 failure)
- **test_complex_delimiter_balance**
- **Issue**: Detector not recognizing flow-style `{key: value}` syntax within `[...]` arrays
- **Impact**: False positives for duplicate keys in valid flow-style YAML

### Category 2: Scope Tracking (2 failures)
- **test_valid_complete_yaml**
- **test_complex_nested_structure**
- **Issue**: Scope stack not properly entering/exiting scopes for parent keys and sequence items
- **Impact**: False positives for duplicate keys across different scopes

---

## Cross-Reference to Raw Output

The raw test output is captured in:
- **File**: `/home/coding/ARMOR/test_results.txt`
- **Lines**: 60-67 (Failure 1), 74-80 (Failure 2), 69-70 (Failure 3)

Summary from raw output:
```
failures:
    parsers::yaml::syntax_detector_tests::delimiter_tests::test_complex_delimiter_balance
    parsers::yaml::syntax_detector_tests::integration_tests::test_complex_nested_structure
    parsers::yaml::syntax_detector_tests::integration_tests::test_valid_complete_yaml

test result: FAILED. 51 passed; 3 failed; 0 ignored; 0 measured; 195 filtered out
```

---

## Additional Notes

### Related Files
- Test file: `src/parsers/yaml/syntax_detector_tests.rs`
- Implementation: `src/parsers/yaml/syntax_detector.rs`
- Scope module: `src/parsers/yaml/scope.rs` (likely, based on imports)

### Investigation Recommendations

1. **Flow Context Tracking**: 
   - Verify `delimiter_state.in_flow_context` is set correctly when entering `[...]` or `{...}` 
   - Check that the flag is cleared when exiting flow context
   - Ensure nested flow contexts are tracked (e.g., `[{...}]`)

2. **Scope Stack Management**:
   - Add debug logging to trace scope enter/exit operations
   - Verify `scope_stack.enter_scope()` is called for parent keys
   - Verify `scope_stack.exit_to_scope()` correctly handles indentation decreases
   - Check sequence scope handling in `enter_sequence_scope()`

3. **Test Coverage**:
   - Add unit tests specifically for flow context detection
   - Add unit tests for scope transitions with varying indentation patterns
   - Add integration tests for complex nested structures with repeated keys

### Severity Assessment
- **Impact**: Medium - Tests fail but functionality may still work for simple YAML files
- **User Impact**: False positives will report errors in valid YAML files
- **Regression Risk**: Low - These are test failures, not production code changes

---

## Total Documented Failures: 3
- **Flow Style Detection**: 1
- **Scope Tracking**: 2
