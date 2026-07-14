# Pytest Output Samples

This directory contains a comprehensive collection of pytest output examples covering different failure scenarios. Each test file demonstrates specific types of test failures with captured pytest output.

## Overview

This collection includes **5 test files** covering **35+ test scenarios** with various failure modes, from simple assertions to complex parameterized tests and fixture failures.

## Test Files and Output Examples

### 1. Simple Assertions (`test_simple_assertions.py` → `simple_assertions_output.txt`)

**Failure Types Covered:**
- Equality assertion failures
- Truthy/falsy value checks
- Membership testing (`in` operator)
- Comparison operators (greater than, less than)
- Type checking with `isinstance()`

**Test Count:** 5 tests (all fail)

**Example Output:**
```
E   AssertionError: Expected 1 to equal 2
E   assert 1 == 2
```

### 2. Collection Comparisons (`test_collection_comparisons.py` → `collection_comparisons_output.txt`)

**Failure Types Covered:**
- List equality with element differences
- Dictionary value comparisons
- Missing keys in dictionaries
- Nested dictionary structure comparisons
- Set membership differences
- Tuple ordering issues

**Test Count:** 6 tests (all fail)

**Example Output:**
```
E   assert [1, 2, 3, 4, 6] == [1, 2, 3, 4, 5]
E     At index 4 diff: 6 != 5
```

### 3. Multiline Strings (`test_multiline_strings.py` → `multiline_strings_output.txt`)

**Failure Types Covered:**
- Multiline text differences with context diffs
- Code block comparisons
- Long paragraph text differences
- JSON-like structure comparisons
- Line-by-line diff visualization

**Test Count:** 4 tests (all fail)

**Example Output:**
```
E   AssertionError: assert 'Line 1: Firs...5: Fifth line' == 'Line 1: Firs...5: Fifth line'
E     
E       Line 1: First line
E     - Line 2: Second line
E     + Line 2: Second line changed
E     ?                    ++++++++
```

### 4. Exceptions and Edge Cases (`test_exceptions_and_edge_cases.py` → `exceptions_edge_cases_output.txt`)

**Failure Types Covered:**
- Expected exceptions not raised
- Wrong exception types raised
- Exception message regex mismatches
- Floating-point comparison precision issues
- None value comparison failures
- Attribute errors
- Name errors for undefined variables

**Test Count:** 8 tests (all fail)

**Example Output:**
```
E   Failed: DID NOT RAISE <class 'ValueError'>
```

### 5. Parameterized Tests and Fixtures (`test_parameterized_and_fixtures.py` → `parameterized_fixtures_output.txt`)

**Failure Types Covered:**
- Parameterized test failures with multiple cases
- Fixture-based dependency failures
- Setup/teardown fixture errors
- Custom object comparison failures
- Mixed pass/fail scenarios in parameterized tests
- ERROR vs FAIL distinction

**Test Count:** 13 tests (8 failed, 4 passed, 1 error)

**Example Output:**
```
==================================== ERRORS ====================================
____________________ ERROR at setup of test_with_bad_setup _____________________
E   RuntimeError: Setup failed!
```

## Running the Tests

To regenerate the pytest outputs:

```bash
# Run all tests
pytest test_*.py -v

# Run specific test file
pytest test_simple_assertions.py -v

# Capture output to file
pytest test_simple_assertions.py -v > output_simple_assertions.txt
```

## Usage

These sample outputs are useful for:

1. **Parser Development**: Testing pytest output parsers and formatters
2. **Documentation**: Demonstrating how pytest displays different failure types
3. **CI/CD Integration**: Understanding test result formats
4. **Tool Development**: Building tools that analyze test results
5. **Learning**: Understanding how pytest reports different failure scenarios

## Failure Scenario Categories

| Category | Test File | Output File |
|----------|-----------|-------------|
| Basic Assertions | `test_simple_assertions.py` | `simple_assertions_output.txt` |
| Data Structure Comparisons | `test_collection_comparisons.py` | `collection_comparisons_output.txt` |
| Text/Code Diffs | `test_multiline_strings.py` | `multiline_strings_output.txt` |
| Exception Handling | `test_exceptions_and_edge_cases.py` | `exceptions_edge_cases_output.txt` |
| Advanced pytest Features | `test_parameterized_and_fixtures.py` | `parameterized_fixtures_output.txt` |

## Summary Statistics

- **Total Test Files**: 5
- **Total Test Cases**: 36
- **Failure Scenarios Covered**: 15+ distinct types
- **Output Format**: pytest 8.3.3 with verbose output

## Notes

- All tests are designed to fail (except a few in parameterized tests) to demonstrate failure outputs
- Tests use pytest 8.3.3 on Python 3.12.8
- Outputs include full pytest session information with platform details
- Each output file shows the complete test run with all failures documented