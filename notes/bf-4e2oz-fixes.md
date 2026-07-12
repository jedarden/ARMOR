# Int64 Test Fix Mapping

## Specific Changes Required

### File: `/home/coding/ARMOR/tests/invalid_type_conversion_test.rs`
### Function: `test_negative_int64_to_uint64_conversions()` (lines 1669-1779)

---

## Change 1: Add Documentation Comment

**Location**: After line 1678 (after the test cases table)

**Add**:
```rust
    // NOTE: This test uses two separate test case vectors because:
    // 1. Values beyond i64::MIN (-9223372036854775809) cannot be represented as i64 integers
    // 2. YAML parses these as strings, requiring different validation logic
    // 3. To fully test int64 boundaries, we need both within-range and beyond-range cases
```

---

## Change 2: Standardize Error Type Labels

**Location**: Line 1757

**Current**:
```rust
let error = ParseError::type_mismatch("value", "uint64", "negative_string");
```

**Change to**:
```rust
let error = ParseError::type_mismatch("value", "uint64", "int64_negative");
```

**Reason**: Keep error type labels consistent - all negative int64 cases should use `"int64_negative"` regardless of whether the YAML parser represents them as strings.

---

## Change 3: Add Documentation to Second Loop

**Location**: Before line 1736 (before the second validation loop)

**Add**:
```rust
    // Test values beyond i64 range - these are parsed as strings
    // and require separate validation logic
```

---

## Alternative Approach: Simplify by Removing String Cases

If beyond-i64::MIN test cases are not essential, consider removing lines 1731-1778 entirely:

**Remove**:
- Lines 1731-1734: `beyond_i64_min_cases` vector
- Lines 1736-1778: Second validation loop

**Result**: Test would match the int32 pattern exactly

**Trade-off**: Loss of coverage for extreme boundary conditions beyond i64::MIN

---

## Summary of Changes

| Change # | Type | Location | Impact |
|----------|------|----------|--------|
| 1 | Documentation | After line 1678 | Adds clarity about dual-pattern |
| 2 | Code standardization | Line 1757 | Makes error labels consistent |
| 3 | Documentation | Before line 1736 | Clarifies second loop purpose |
| Alt | Code removal | Lines 1731-1778 | Simplifies to match int32 pattern |

---

## Recommended Action

**Apply Changes 1, 2, and 3** to improve clarity and consistency while maintaining comprehensive boundary testing.

**Only consider Alternative approach** if:
- Beyond-i64::MIN testing is not required
- Consistency with int32 pattern is more important than boundary coverage
