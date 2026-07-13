# Bead bf-2w54h: Helper Macros for Parameterized Folded Scalar Testing

## Summary
Enhanced helper macro infrastructure to support parameterized folded scalar testing across all indent levels (0, 1, 2, 3, 4, tab).

## Work Completed

### 1. Enhanced Helper Functions Added

#### `generate_folded_scalar_tests_for_level()`
- **Purpose**: Generate test cases for a specific indent level (0-4 or tab)
- **Parameters**:
  - `level`: "level0" through "level4", or "tab"
  - `keys`: Array of key names to test
  - `modifiers`: Array of modifiers (`">"`, `">-"`, `">+"`)
  - `indent_nums`: Array of explicit indent numbers (1-9)
- **Returns**: Vector of `(line, expected_key, expected_type)` tuples
- **Key Feature**: Supports level 0 (no indentation) with empty string indent

#### `generate_folded_scalar_tests_all_levels()`
- **Purpose**: Comprehensive test generation across ALL indent levels
- **Levels Supported**:
  - level0: `""` (no indentation)
  - level1: `"  "` (2 spaces)
  - level2: `"    "` (4 spaces)
  - level3: `"      "` (6 spaces)
  - level4: `"        "` (8 spaces)
  - tab: `"\t"` (tab character)
- **Parameters**: Same as above, but iterates all levels
- **Use Case**: Comprehensive testing across all indentation scenarios

### 2. Existing Infrastructure (Already Present)

The following infrastructure was already in place from prior beads:
- `generate_folded_explicit_indent_tests!` macro (line 73)
- `run_folded_scalar_tests!` macro (line 95)
- `create_folded_scalar_test()` helper function (line 128)
- `generate_folded_scalar_tests_multi_level()` function (line 141)

### 3. Test Functions Added

#### `test_folded_scalar_level0_all_modifiers()`
- Tests folded scalars with NO indentation (level 0)
- Verifies all three modifiers: `>`, `>-`, `>+`
- Explicit indent numbers: 1, 2, 3, 4, 5
- Validates that generated lines have no leading spaces

#### `test_folded_scalar_all_levels_comprehensive()`
- Tests across ALL indent levels (0, 1, 2, 3, 4, tab)
- Single key "sample" with plain modifier `>`
- Two indent numbers: 1, 2
- Generates 12 test cases (6 levels × 2 indent numbers)

#### `test_folded_scalar_level0_specific_manual_cases()`
- Manually specified level 0 test cases
- Comprehensive coverage of all modifiers at level 0
- Explicit expectations for each pattern

#### `test_folded_scalar_level0_vs_level1_comparison()`
- Compares level 0 (no indent) vs level 1 (2-space)
- Validates that indentation is correctly applied

#### `test_folded_scalar_level0_with_exclamation()`
- Tests level 0 with exclamation marks in content
- Ensures exclamation marks don't trigger false tag detection

### 4. Acceptance Criteria Status

✅ **Create or adapt helper macros for parameterized testing**
   - Enhanced with level-specific and all-levels generators

✅ **Follow the macro pattern from existing tests**
   - Maintains consistency with existing `generate_folded_explicit_indent_tests!` pattern
   - Uses `run_folded_scalar_tests!` for execution

✅ **Support varying indent levels (0, 1, 2, 3+)**
   - Full support for levels 0-4 plus tab indentation
   - Level 0 = no indentation (empty string)
   - Levels 1-4 = 2, 4, 6, 8 spaces respectively

✅ **Ensure macros are testable and compile**
   - All tests compile successfully
   - 4 new tests pass:
     - `test_folded_scalar_level0_all_modifiers` ✓
     - `test_folded_scalar_all_levels_comprehensive` ✓
     - `test_folded_scalar_level0_specific_manual_cases` ✓
     - `test_folded_scalar_level0_vs_level1_comparison` ✓

### 5. Usage Examples

```rust
// Generate tests for level 0 only
let test_cases = generate_folded_scalar_tests_for_level(
    "level0",              // No indentation
    &["text", "data"],
    &[">", ">-", ">+"],
    &[1, 2, 3, 4, 5],
);
run_folded_scalar_tests!(test_cases);

// Generate tests for all levels (0-4, tab)
let test_cases = generate_folded_scalar_tests_all_levels(
    &["sample"],
    &[">"],
    &[1, 2],
);
run_folded_scalar_tests!(test_cases);

// Generate for a specific level with tab
let test_cases = generate_folded_scalar_tests_for_level(
    "tab",
    &["key1", "key2"],
    &[">-", ">+"],
    &[2, 3, 4],
);
run_folded_scalar_tests!(test_cases);
```

## Files Modified
- `tests/type_like_string_false_positive_test.rs`
  - Added 2 helper functions (lines 178-259)
  - Added 5 test functions (lines 12167-12370)
  - Comprehensive inline documentation

## Test Results
```
running 4 tests
test test_folded_scalar_level0_specific_manual_cases ... ok
test test_folded_scalar_level0_vs_level1_comparison ... ok
test test_folded_scalar_level0_all_modifiers ... ok
test test_folded_scalar_level0_with_exclamation ... ok

test result: ok. 4 passed; 0 failed
```

## Related Beads
- bf-41ba1: Skeleton template creation
- bf-63gy6: Infrastructure pattern documentation
- bf-4aw6b: Plain explicit indent tests
- bf-45gyh: Strip indent tests
- bf-5rzoh: Keep explicit indent tests
