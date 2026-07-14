# Pytest Flags Verification - 2026-07-13

## Purpose
Re-verify the optimal pytest flag combination `-vv --tb=short` on single failing tests.

## Tests Run

### 1. Dictionary Equality (`test_dict_equality`)
```bash
nix-shell -p python312Packages.pytest --run "pytest test_pytest_flags_minimal.py::test_dict_equality -vv --tb=short"
```

**Verification Results:**
- ✅ Line number: `test_pytest_flags_minimal.py:39: in test_dict_equality`
- ✅ Expected vs actual: Both dicts displayed in full
- ✅ Common items shown: `{'name': 'test'}`
- ✅ Differing items listed separately
- ✅ Character-level diffs: `^^^` vs `^^^^` for boolean values

### 2. List Comparison (`test_list_comparison`)
```bash
nix-shell -p python312Packages.pytest --run "pytest test_pytest_flags_minimal.py::test_list_comparison -vv --tb=short"
```

**Verification Results:**
- ✅ Line number: `test_pytest_flags_minimal.py:46: in test_list_comparison`
- ✅ Index information: `At index 2 diff: 0 != 3`
- ✅ Visual indicators: `? ^` shows exact element positions
- ✅ Multiple differences detected: indices 2 and 4

### 3. Nested Structure (`test_nested_structure`)
```bash
nix-shell -p python312Packages.pytest --run "pytest test_pytest_flags_minimal.py::test_nested_structure -vv --tb=short"
```

**Verification Results:**
- ✅ Line number: `test_pytest_flags_minimal.py:122: in test_nested_structure`
- ✅ Hierarchical diffs: Shows path to nested differences
- ✅ Character-level precision even for nested strings: `^^^^^^^` vs `^  ^^^^^^^`
- ✅ Clear visualization of "Springfield" → "Shelbyville" change

## Acceptance Criteria Verification

✅ **Test output captured showing line numbers, expected, and actual values**
- All tests show exact file:line location
- Expected and actual values displayed in full
- Diffs provide detailed comparisons

✅ **Verification that flags work as expected**
- `-vv --tb=short` produces comprehensive, readable output
- Assertion rewriting (default in pytest 8.3.3) provides excellent diffs
- No missing information in output

✅ **Alternative flags tested if needed**
- Not needed - `-vv --tb=short` works excellently for all test patterns

## Conclusion

The pytest flag combination **`-vv --tb=short`** provides optimal output for debugging failing tests:
- Shows exact location (file:line)
- Displays expected vs actual values clearly
- Provides detailed diffs with visual indicators
- Handles all data types (strings, numbers, dicts, lists, nested structures)
- Maintains readability without overwhelming verbosity

## Recommended Execution Pattern

For single failing tests:
```bash
nix-shell -p python312Packages.pytest --run "pytest <test_file>::<test_name> -vv --tb=short"
```

For all tests in a file:
```bash
nix-shell -p python312Packages.pytest --run "pytest <test_file> -vv --tb=short"
```

---
*Verification completed: 2026-07-13*
*Bead: bf-1wxppv*
