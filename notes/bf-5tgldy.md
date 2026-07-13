# Core Scope Tracking Test Results

## Test Run Summary

Executed 5 core scope tracking integration tests on 2026-07-13.

## Results by Test File

### 1. comprehensive_scope_tracking_test.rs
**Status: FAILED** - 55 passed; 10 failed

Failed tests:
- test_depth_of_new_stack - Expected depth 1, got 0
- test_deep_nesting_stack - Expected depth 21, got 20
- test_exit_to_nonexistent_scope_uses_closest_parent - Expected 2 scopes, got 1
- test_exit_to_root_leaves_one_scope - Expected 1 scope, got 0
- test_exit_to_scope_removes_deeper_scopes - Expected 4 scopes, got 3
- test_get_scope_at_level - Expected scope at level 0, found None
- test_many_sibling_scopes - Expected 2 scopes, got 1
- test_scope_at_zero_indent - Expected scope at zero indent, found None
- test_scope_stack_display - Expected "depth=3" in display
- test_scope_stack_reset_clears_all_scopes - Expected 3 scopes, got 2

### 2. scope_stack_test.rs
**Status: PASSED** - 6 passed; 0 failed

All tests passed successfully.

### 3. scope_stack_verification_test.rs
**Status: PASSED** - 25 passed; 0 failed

All tests passed successfully.

### 4. scope_tracking_comprehensive_test.rs
**Status: FAILED** - 63 passed; 10 failed

Failed tests:
- test_depth_tracking - Expected depth 1, got 0
- test_enter_multiple_scopes - Expected 4 scopes, got 3
- test_enter_scope_creates_new_level - Expected 1 scope, got 0
- test_enter_scope_with_no_parent_key - Expected 2 scopes, got 1
- test_exit_multiple_levels - Expected 1 scope, got 0
- test_exit_to_parent_scope - Expected 3 scopes, got 2
- test_get_scope_at_level - Expected scope at level 0, found None
- test_scope_stack_initial_state - Expected depth 1, got 0
- test_scope_stack_reset - Expected 2 scopes, got 1
- test_very_deep_nesting - Expected depth 21, got 20

### 5. sequence_scope_verification_test.rs
**Status: FAILED** - 27 passed; 5 failed

Failed tests:
- test_deeply_nested_sequence_in_mapping - Expected 4 scopes, got 3
- test_deeply_nested_mapping_in_sequence - Expected 5 scopes, got 4
- test_parser_no_false_duplicates_in_sequences_simple - False duplicate detection in sequence items
- test_sequence_entry_preserves_parent_scopes - Expected 4 scopes, got 3
- test_sequence_mapping_sequence_pattern - Expected 5 scopes, got 4

## Pattern Analysis

**Common Issue: Depth tracking off by -1**

All failures show a consistent pattern where the actual depth/scope count is **1 less than expected**. This suggests:

1. The root scope is not being properly initialized or counted
2. The `push_scope` or scope entry mechanism is not incrementing depth correctly
3. The scope stack may be starting empty (depth=0) when it should start with a root scope (depth=1)

## Summary

- **Total tests run**: 191 tests (65+6+25+73+32)
- **Passed**: 176 tests
- **Failed**: 25 tests
- **Pass rate**: 92.1%

## Root Cause

The scope tracking system appears to have a fundamental issue with depth initialization. The stack starts at depth 0 when tests expect it to start at depth 1 (with a root scope implicitly present). This affects:
- Initial scope depth reporting
- Scope count after operations
- Nested scope tracking
- Sequence scope depth calculations

The fix will likely require adjusting the scope stack initialization to either:
1. Initialize with a root scope at depth 1, OR
2. Update test expectations to match the current depth-0-based indexing
