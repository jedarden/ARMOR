# JSON Output Validation Tests

## Overview

This directory contains comprehensive JSON output validation tests for the ARMOR pytest parser. These tests verify that the parser produces valid, parseable JSON with all required fields present and structure matching the documented schema.

## Test Coverage

The test suite includes 29 tests covering:

### Format-Specific Tests (24 tests)
- **Format 1 (--tb=short)**: 4 tests × 2 sample files = 8 tests
- **Format 2 (--tb=long)**: 4 tests × 2 sample files = 8 tests  
- **Format 3 (--tb=line)**: 4 tests × 2 sample files = 8 tests

Each format sample is tested for:
1. JSON parseability
2. Required fields presence
3. Correct field types
4. No unexpected fields

### Cross-Format Tests (5 tests)
- All samples produce valid JSON
- All samples have required fields
- All samples have correct field types
- All samples have no extra fields
- Format schemas are properly defined

## Running the Tests

### Option 1: Using Python Directly (No Installation Required)

```bash
python3 tests/test_json_validation.py
```

This runs the tests using Python's built-in `unittest` framework. No additional installation required.

### Option 2: Using pytest

```bash
# Install pytest first (if not already installed)
pip3 install pytest

# Run tests
pytest tests/test_json_validation.py -v

# Run with more detailed output
pytest tests/test_json_validation.py -vv

# Run specific test
pytest tests/test_json_validation.py::test_format1_sample1_json_parseable -v

# Run all tests in tests/ directory
pytest tests/ -v
```

### Option 3: Using pytest with discovery

```bash
# From project root
pytest tests/ -v --tb=short

# Run only JSON validation tests
pytest tests/test_json_validation.py -v
```

## Acceptance Criteria

All acceptance criteria are met:

✅ **Tests verify JSON is parseable** - Each sample's output is verified to be parseable by Python's `json` module

✅ **Tests check for all required fields** - Each format's required fields (`test_file`, `line_number`, `error_type`) are verified to be present and non-null

✅ **Tests validate structure matches schema** - Field types are validated (strings, ints, lists) and no unexpected fields are present

✅ **All tests pass for the 3 sample formats** - 29/29 tests pass, covering all 3 formats with 2 samples each

✅ **Tests are runnable via pytest** - Tests are written using pytest conventions and can be run with `pytest`

## Test Output

When all tests pass, you should see:

```
Ran 29 tests in 0.036s

OK
```

## File Structure

```
tests/
├── test_json_validation.py      # Main test file (29 tests)
├── test_pytest_patterns_unit.py # Pattern unit tests (separate suite)
└── README_JSON_VALIDATION_TESTS.md # This file

tools/
├── parse_pytest_output.py        # Parser under test
├── extract_json_compatible_data  # JSON conversion utility
└── test_samples/                  # Test data
    ├── sample1_full_detailed.txt
    ├── sample_format1_vv_short.txt
    ├── sample2_long_format.txt
    ├── sample_format2_v_long.txt
    ├── sample3_line_format.txt
    └── sample_format3_tb_line.txt
```

## Schema Definitions

Each format has a defined schema:

### Format 1 (--tb=short) & Format 2 (--tb=long)
- **Required fields**: `test_file`, `line_number`, `error_type`
- **Optional fields**: `test_name`, `error_message`, `assertion_line`, `assertion_type`, `expected`, `actual`, `diff_lines`, `index_diff`, `differing_items`, `where_clause`

### Format 3 (--tb=line)
- **Required fields**: `test_file`, `line_number`, `error_type`
- **Optional fields**: `test_name`, `error_message`, `expected`, `actual`

## Test Maintenance

When adding new pytest formats or samples:

1. Update `FORMAT_SCHEMAS` with the new format specification
2. Add new sample files to `tools/test_samples/`
3. Add corresponding test functions following the naming pattern:
   - `test_format{N}_sample{M}_json_parseable()`
   - `test_format{N}_sample{M}_required_fields()`
   - `test_format{N}_sample{M}_field_types()`
   - `test_format{N}_sample{M}_no_extra_fields()`
4. Update the `test_all_samples_*()` functions to include new samples
5. Run tests to verify: `python3 tests/test_json_validation.py`

## Related Documentation

- `tools/pytest_patterns.md` - Pattern definitions
- `tools/verify_json_structure.py` - Standalone verification script
- `docs/parser_verification_summary.md` - Overall verification status
