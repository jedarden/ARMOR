# Parsed and Categorized Test Results Analysis
**Generated:** 2026-07-13  
**Source:** Integration Test Execution Log - Bead bf-48ig0c  
**Total Test Suites:** 2 (Rust + Python)

## Executive Summary
- **Total Tests Executed:** 751 tests (377 Rust + 374 Python)
- **Overall Pass Rate:** 546/751 (72.7%)
- **Overall Fail Rate:** 205/751 (27.3%)
- **Rust Pass Rate:** 363/377 (96.3%)
- **Python Pass Rate:** 183/374 (48.9%)
- **Overall Exit Code:** 1 (both suites had failures)

---

## SECTION 1: RUST INTEGRATION TESTS

### Framework: Rust Test Framework (cargo test)
### Total Rust Tests: 377 (351 unit + 26 integration)

#### Rust Test Results Summary
- **Passed:** 363 tests (96.3%)
- **Failed:** 14 tests (3.7%)
- **Ignored:** 0 tests
- **Exit Code:** 1

#### Test Suite Breakdown

##### 1. Rust Unit Tests
**Status:** ✅ ALL PASSED
- **Total:** 351 tests
- **Passed:** 351 (100%)
- **Failed:** 0

##### 2. Rust Integration Test Suites

| Test Suite | Passed | Failed | Total | Status |
|-------------|--------|--------|-------|--------|
| acceptance_criteria_verification_test | 5 | 0 | 5 | ✅ PASSED |
| comment_filtering_basic_test | 19 | 0 | 19 | ✅ PASSED |
| error_code_validation_test | 15 | 0 | 15 | ✅ PASSED |
| error_message_format_examples_test | 21 | 0 | 21 | ✅ PASSED |
| error_messages_test | 5 | 0 | 5 | ✅ PASSED |
| **exit_to_scope_edge_cases_test** | **12** | **14** | **26** | ❌ **FAILED** |

### Rust Failure Analysis

#### exit_to_scope_edge_cases_test (14/26 failures)

**Failed Tests (14):**
1. ❌ test_exit_to_scope_cleanup_multiple_nested_levels
2. ❌ test_exit_to_scope_cleanup_with_indent_gaps
3. ❌ test_exit_to_scope_complex_nesting_with_gaps
4. ❌ test_exit_to_scope_from_stack_with_only_root
5. ❌ test_exit_to_scope_multiple_times_in_sequence
6. ❌ test_exit_to_scope_partial_depth
7. ❌ test_exit_to_scope_preserves_root_scope_even_in_edge_cases
8. ❌ test_exit_to_scope_rapid_exits_no_stale_state
9. ❌ test_exit_to_scope_rapid_successive_exits
10. ❌ test_exit_to_scope_to_nonexistent_level_between_existing_scopes
11. ❌ test_exit_to_scope_to_root
12. ❌ test_exit_to_scope_when_target_has_no_scope_but_parent_exists
13. ❌ test_exit_to_scope_with_parent_at_target
14. ❌ test_exit_to_scope_without_parent_at_target

**Passed Tests (12):**
- ✅ test_exit_to_scope_allows_clean_reentry
- ✅ test_exit_to_scope_does_not_exit_to_deeper_level
- ✅ test_exit_to_scope_from_root_to_root_is_idempotent
- ✅ test_exit_to_scope_from_sequence_context
- ✅ test_exit_to_scope_handles_indent_not_multiple_of_base
- ✅ test_exit_to_scope_clears_large_scope_data
- ✅ test_exit_to_scope_resets_sequence_context_flags
- ✅ test_exit_to_scope_sequence_item_id_preservation_in_parent
- ✅ test_exit_to_scope_sibling_transition
- ✅ test_exit_to_scope_state_cleanup
- ✅ test_exit_to_scope_when_target_is_same_as_current_indent
- ✅ test_exit_to_scope_with_flow_style_preservation

#### Common Failure Pattern: Scope Stack Management

**Assertion Failure Types:**
- **Scope Count Mismatch:** 10 failures (expected vs actual scope levels)
  - Expected 1, got 0: 5 failures
  - Expected 2, got 1: 4 failures
  - Expected 3, got 2: 2 failures
  
- **Root Scope Preservation:** 1 failure
  - `assertion failed: stack.get_scope_at_level(0).is_some()`

**Root Cause:** The scope stack cleanup logic in `exit_to_scope` function is not correctly managing scope levels during complex exit scenarios, particularly:
- Multiple nested levels
- Indentation gaps
- Partial depth exits
- Rapid successive exits

---

## SECTION 2: PYTHON INTEGRATION TESTS

### Framework: pytest 8.3.3
### Total Python Tests: 374

#### Python Test Results Summary
- **Passed:** 183 tests (48.9%)
- **Failed:** 191 tests (51.1%)
- **Exit Code:** 1
- **Execution Time:** 2.62 seconds

#### Test Suite Breakdown

| Test Suite | Status | Tests | Note |
|------------|--------|-------|------|
| test_inventory_reader.py | ✅ PASSED | 25 | All 25 tests passed |
| yamlutil/test_broken_samples.py | ❌ FAILED | 30 | Error handling for invalid YAML |
| yamlutil/test_complete_mixed_yaml_documents.py | ❌ FAILED | 10 | Document boundaries |
| yamlutil/test_exceptions.py | ⚠️ MIXED | 32 | 16 PASSED, 16 FAILED |
| yamlutil/test_explicit_indent.py | ❌ FAILED | 20 | Folded scalar indentation |
| yamlutil/test_indentation_comment_filtering.py | ❌ FAILED | 16 | Comment filtering |
| yamlutil/test_mixed_comment_scenarios.py | ❌ FAILED | 33 | Complex comment scenarios |
| yamlutil/test_parser.py | ⚠️ MIXED | 32 | 6 PASSED, 26 FAILED |
| yamlutil/test_reader.py | ⚠️ MIXED | 31 | 1 PASSED, 30 FAILED |
| yamlutil/test_result_comprehensive.py | ✅ PASSED | 2 | Result dataclass |
| yamlutil/test_result_helpers.py | ✅ PASSED | 2 | Helper methods |
| yamlutil/test_result_helpers_extended.py | ✅ PASSED | 2 | Extended helpers |
| yamlutil/test_validator.py | ⚠️ MIXED | 24 | 2 PASSED, 22 FAILED |
| yamlutil/validate_yaml_functional.py | ✅ PASSED | 130 | Functional validation |
| yamlutil/verify_implementation.py | ✅ PASSED | 111 | Implementation verification |

### Python Failure Categories

#### Category 1: Complete Failures (All tests failed)
1. **test_broken_samples.py** (0/30 passed)
   - Error handling for invalid YAML samples

2. **test_complete_mixed_yaml_documents.py** (0/10 passed)
   - Document boundary handling

3. **test_explicit_indent.py** (0/20 passed)
   - Folded scalar indentation detection

4. **test_indentation_comment_filtering.py** (0/16 passed)
   - Comment filtering with indentation

5. **test_mixed_comment_scenarios.py** (0/33 passed)
   - Complex comment scenario handling

#### Category 2: Majority Failures
1. **test_parser.py** (6/32 passed, 81.3% failure rate)
   - YAML parser functionality

2. **test_reader.py** (1/31 passed, 96.8% failure rate)
   - File reading and parsing

3. **test_validator.py** (2/24 passed, 91.7% failure rate)
   - YAML syntax validation

4. **test_exceptions.py** (16/32 passed, 50% failure rate)
   - Exception handling

#### Category 3: Complete Passes
1. **test_inventory_reader.py** (25/25 passed)
   - Debug file inventory reading

2. **test_result_comprehensive.py** (2/2 passed)
   - Result dataclass tests

3. **test_result_helpers.py** (2/2 passed)
   - Helper method tests

4. **test_result_helpers_extended.py** (2/2 passed)
   - Extended helper tests

5. **validate_yaml_functional.py** (130/130 passed)
   - Functional validation

6. **verify_implementation.py** (111/111 passed)
   - Implementation verification

### Python Failure Analysis

**Major Failure Areas:**
1. **YAML Parsing Core** (test_parser, test_reader): 93.5% failure rate
   - Parser initialization
   - Basic functionality
   - File reading integration

2. **Comment Handling** (test_indentation_comment_filtering, test_mixed_comment_scenarios): 100% failure rate
   - Comment detection in various indentation contexts
   - Complex comment scenarios
   - Comment filtering logic

3. **Error Detection** (test_broken_samples, test_validator): 95.8% failure rate
   - Malformed YAML detection
   - Error categorization
   - Error reporting

4. **Document Structure** (test_complete_mixed_yaml_documents, test_explicit_indent): 100% failure rate
   - Document boundaries
   - Folded scalar indentation
   - Multi-document handling

**Successful Areas:**
- **Result data structures** (100% pass rate)
- **Functional validation** (130/130 passed)
- **Implementation verification** (111/111 passed)
- **Inventory reading** (25/25 passed)

---

## OVERALL TEST CATEGORIZATION

### By Test Framework

| Framework | Total | Passed | Failed | Pass Rate |
|-----------|-------|--------|--------|-----------|
| Rust Cargo Test | 377 | 363 | 14 | 96.3% |
| Python pytest | 374 | 183 | 191 | 48.9% |
| **TOTAL** | **751** | **546** | **205** | **72.7%** |

### By Test Suite Category

| Category | Total | Passed | Failed | Pass Rate |
|----------|-------|--------|--------|-----------|
| Unit Tests | 351 | 351 | 0 | 100% |
| Integration Tests | 400 | 195 | 205 | 48.8% |
| **TOTAL** | **751** | **546** | **205** | **72.7%** |

### By Functionality

| Functionality | Total | Passed | Failed | Pass Rate |
|---------------|-------|--------|--------|-----------|
| Scope Management (Rust) | 26 | 12 | 14 | 46.2% |
| Comment Filtering (Rust) | 19 | 19 | 0 | 100% |
| Error Messages (Rust) | 26 | 26 | 0 | 100% |
| YAML Parsing (Python) | 93 | 9 | 84 | 9.7% |
| Comment Handling (Python) | 49 | 0 | 49 | 0% |
| Error Detection (Python) | 54 | 18 | 36 | 33.3% |
| Result Structures (Python) | 4 | 4 | 0 | 100% |
| Validation (Python) | 241 | 241 | 0 | 100% |

---

## COMMON FAILURE PATTERNS

### Pattern 1: Scope Stack Management (Rust)
**Affected Tests:** 14/26 failures in exit_to_scope_edge_cases_test
**Failure Type:** Scope count mismatches during complex exit scenarios
**Root Cause:** Incomplete cleanup logic in `exit_to_scope` function

### Pattern 2: YAML Parser Core (Python)
**Affected Tests:** 84/93 failures across test_parser, test_reader
**Failure Type:** Parser initialization and basic functionality failures
**Root Cause:** Implementation gaps in YAML parsing core

### Pattern 3: Comment Handling (Python)
**Affected Tests:** 49/49 failures across comment filtering tests
**Failure Type:** Complete failure of comment detection and filtering
**Root Cause:** Missing or broken comment handling implementation

### Pattern 4: Error Detection (Python)
**Affected Tests:** 36/54 failures across error detection tests
**Failure Type:** Inability to detect malformed YAML
**Root Cause:** Error detection logic not properly implemented

### Pattern 5: Document Structure (Python)
**Affected Tests:** 30/30 failures in document boundary tests
**Failure Type:** Document boundary and structure handling failures
**Root Cause:** Multi-document and folded scalar handling issues

---

## KEY FINDINGS

1. **Rust tests are mostly healthy** - 96.3% pass rate with only scope management edge cases failing
2. **Python tests have critical issues** - 51.1% failure rate, with complete failure in comment handling and document structure
3. **Scope management needs attention** - 14 failures in Rust exit_to_scope edge cases
4. **YAML parser core needs implementation** - 90% failure rate in Python parsing tests
5. **Comment handling is completely broken** - 100% failure rate across all Python comment tests
6. **Validation tests are healthy** - 241/241 validation tests passed in Python

---

## RECOMMENDATIONS

### High Priority (Critical)
1. **Fix Python YAML parser implementation** - Core parsing functionality has 90% failure rate
2. **Implement comment handling in Python** - 100% failure rate across all comment tests
3. **Fix Rust scope stack management** - 14/26 failures in exit_to_scope edge cases

### Medium Priority (Important)
1. **Complete Python error detection logic** - 67% failure rate in error detection tests
2. **Implement document structure handling** - 100% failure in document boundary tests
3. **Review Rust exit_to_scope cleanup logic** - Focus on nested level and indent gap scenarios

### Low Priority (Nice to have)
1. **Add more comprehensive integration tests** - Current tests have good coverage but gaps exist
2. **Improve test error messages** - Some failures lack detailed error context
3. **Consider adding performance benchmarks** - No performance regression testing

---

## TEST SUITE HEALTH ASSESSMENT

### Healthy Test Suites (100% pass rate)
- Rust Unit Tests (351/351)
- Rust Comment Filtering (19/19)
- Rust Error Messages (26/26)
- Python Inventory Reader (25/25)
- Python Result Structures (4/4)
- Python Validation (241/241)

### At-Risk Test Suites (50-95% pass rate)
- Rust Scope Management (12/26 - 46.2%)
- Python Exception Handling (16/32 - 50%)

### Critical Test Suites (<50% pass rate)
- Python YAML Parsing (9/93 - 9.7%)
- Python Comment Handling (0/49 - 0%)
- Python Error Detection (18/54 - 33.3%)
- Python Document Structure (0/30 - 0%)

---

## CONCLUSION

The integration test results reveal a **significant gap** between Rust and Python test implementations:

**Rust**: Healthy overall with only scope management edge cases needing attention (96.3% pass rate)

**Python**: Critical issues in core functionality (48.9% pass rate) with complete failures in comment handling and document structure processing

The test suite clearly indicates that **Python YAML parsing implementation needs immediate attention**, particularly in the areas of:
1. Core parser functionality
2. Comment detection and filtering
3. Document boundary handling
4. Error detection and reporting

The Rust implementation is in much better shape but requires **scope stack management refinement** for complex exit scenarios.